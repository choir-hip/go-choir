package searchplane

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"
)

// Router starts searches across its providers with durable health.
//
// Under RoutingBudget a search starts the free providers, holds the paid
// ones in reserve and adds them one at a time, cheapest first, only while
// the search is short of distinct results
// (docs/archive/reports/search-efficiency-and-reuse-2026-10-14.md §4.1). Under
// RoutingFanout every eligible provider starts at once. Every call is
// counted per provider per UTC day in the health store, and a provider at
// its daily cap is skipped as capped.
type Router struct {
	providers []Provider
	store     HealthStore
	counter   CallCounter
	config    Config
	pace      *pacer
	now       func() time.Time

	// monthly memoizes each monthly-quota provider's pace for its day
	// (monthly.go): the period's earlier days do not change.
	monthlyMu sync.Mutex
	monthly   map[string]MonthlyPace
}

// NewRouter creates a search router.
func NewRouter(providers []Provider, store HealthStore, config Config) *Router {
	if store == nil {
		store = NewMemoryHealthStore()
	}
	if config.RequestTimeout <= 0 {
		config.RequestTimeout = 30 * time.Second
	}
	if config.PaceWait <= 0 {
		config.PaceWait = DefaultPaceWait
	}
	if config.Routing != RoutingBudget {
		config.Routing = RoutingFanout
	}
	if config.Tiers == nil {
		config.Tiers = ParseProviderTiers(DefaultProviderTiers)
	}
	if config.MinResults <= 0 {
		config.MinResults = DefaultMinResults
	}
	if config.DeepMinResults <= 0 {
		config.DeepMinResults = DefaultDeepMinResults
	}
	if config.MaxPaid < 0 {
		config.MaxPaid = 0
	} else if config.MaxPaid == 0 {
		config.MaxPaid = DefaultMaxPaid
	}
	if config.DeepMaxPaid <= 0 {
		config.DeepMaxPaid = DefaultDeepMaxPaid
	}
	if config.EscalateAfter <= 0 {
		config.EscalateAfter = DefaultEscalateAfter
	}
	config.EscalateWindow = EscalationWindow(config)
	counter, ok := store.(CallCounter)
	if !ok {
		counter = newMemoryCallCounter()
	}
	return &Router{providers: providers, store: store, counter: counter, config: config, pace: &pacer{next: map[string]time.Time{}}, now: time.Now, monthly: map[string]MonthlyPace{}}
}

// HealthStore exposes the underlying store for ops endpoints.
func (r *Router) HealthStore() HealthStore { return r.store }

// RequestTimeout is the bound on each provider call.
func (r *Router) RequestTimeout() time.Duration { return r.config.RequestTimeout }

// Routing is the router's mode, RoutingBudget or RoutingFanout.
func (r *Router) Routing() string { return r.config.Routing }

// EscalationWindow is how long after a search began a paid provider may
// still join it (zero under fan-out).
func (r *Router) EscalationWindow() time.Duration { return r.config.EscalateWindow }

// TierOf is a provider's tier; a provider not configured is paid at
// UnknownProviderCost.
func (r *Router) TierOf(provider string) ProviderTier {
	if t, ok := r.config.Tiers[provider]; ok {
		return t
	}
	return ProviderTier{Tier: TierPaid, Cost: UnknownProviderCost}
}

// capOf is a provider's daily cap, or NoCap: the configured DailyCaps
// entry and, for a provider with a monthly quota, the day's pace cap,
// whichever is lower.
func (r *Router) capOf(provider string, now time.Time) int {
	limit := NoCap
	if n, ok := r.config.DailyCaps[provider]; ok {
		limit = n
	}
	if pace, ok := r.monthlyPace(provider, now); ok && (limit < 0 || pace.Cap < limit) {
		limit = pace.Cap
	}
	return limit
}

// MonthlyPaceDayStart is how much of a monthly-quota provider's day cap
// is open at UTC midnight; the rest accrues evenly through the day.
const MonthlyPaceDayStart = time.Hour

// limitAt is the call limit a provider is held to at now, and when that
// limit next grows. For a monthly-quota provider the day's cap accrues
// through the day (an hour's share at midnight, all of it by the end), so
// a morning burst cannot leave the rest of the day without the provider
// (docs/archive/problems/search-throughput-keenable-underused-2026-10-15.md). A
// plain daily cap is open all day.
func (r *Router) limitAt(provider string, now time.Time) (limit int, until time.Time) {
	limit = r.capOf(provider, now)
	until = nextUTCDay(now)
	if limit <= 0 {
		return limit, until
	}
	if _, ok := r.monthlyPace(provider, now); !ok {
		return limit, until
	}
	const daySec = int64(24 * 60 * 60)
	midnight := nextUTCDay(now).AddDate(0, 0, -1)
	elapsed := int64((now.Sub(midnight) + MonthlyPaceDayStart) / time.Second)
	if elapsed >= daySec {
		return limit, until
	}
	accrued := int(int64(limit) * elapsed / daySec)
	// The limit next grows when the accrual reaches accrued+1.
	nextSec := (int64(accrued+1)*daySec + int64(limit) - 1) / int64(limit)
	return accrued, midnight.Add(time.Duration(nextSec)*time.Second - MonthlyPaceDayStart)
}

// monthlyPace is the provider's pace for now's UTC day, computed once a
// day from the durable daily counts (so a restart recomputes the same
// pace). ok is false for a provider without a monthly quota. A counting
// error falls back to the even pace (target / period days) and is not
// memoized.
func (r *Router) monthlyPace(provider string, now time.Time) (MonthlyPace, bool) {
	q, ok := r.config.MonthlyQuotas[provider]
	if !ok || q.Allowance <= 0 {
		return MonthlyPace{}, false
	}
	day := DayOf(now)
	r.monthlyMu.Lock()
	defer r.monthlyMu.Unlock()
	if p, ok := r.monthly[provider]; ok && p.Day == day {
		return p, true
	}
	p, err := q.pace(now, func(d string) (int, error) {
		calls, err := r.counter.CallsOn(d)
		return calls[provider], err
	})
	if err != nil {
		start, end := q.Period(now)
		periodDays := int(end.Sub(start).Hours()/24 + 0.5)
		if periodDays < 1 {
			periodDays = 1
		}
		p.Cap = p.Target / periodDays
		log.Printf("searchplane: %s monthly pace: count error, even pace %d today: %v", provider, p.Cap, err)
		return p, true
	}
	r.monthly[provider] = p
	log.Printf("searchplane: %s monthly pace day=%s period=%s..%s used=%d target=%d days_left=%d day_cap=%d",
		provider, p.Day, p.PeriodStart, p.PeriodEnd, p.Used, p.Target, p.DaysLeft, p.Cap)
	return p, true
}

// DailyCount is one provider's calls on one UTC day.
type DailyCount struct {
	Provider string
	Day      string
	Calls    int
	// Cap is the daily cap, or NoCap.
	Cap  int
	Tier ProviderTier
	// Monthly is the day's monthly pace, for a provider with a monthly
	// quota.
	Monthly *MonthlyPace
}

// DailyCounts returns today's (UTC) count, cap and tier for every
// registered provider, in registration order.
func (r *Router) DailyCounts() ([]DailyCount, error) {
	now := r.now()
	day := DayOf(now)
	calls, err := r.counter.CallsOn(day)
	if err != nil {
		return nil, err
	}
	out := make([]DailyCount, 0, len(r.providers))
	for _, p := range r.providers {
		name := p.Name()
		count := DailyCount{Provider: name, Day: day, Calls: calls[name], Cap: r.capOf(name, now), Tier: r.TierOf(name)}
		if pace, ok := r.monthlyPace(name, now); ok {
			count.Monthly = &pace
		}
		out = append(out, count)
	}
	return out, nil
}

// Request is one search.
type Request struct {
	Query      string
	MaxResults int
	// Deep marks a search the caller wants more resources for: the
	// cheapest paid provider starts with the free ones, and the deep
	// thresholds (DeepMinResults, DeepMaxPaid) apply.
	Deep bool
}

// Stream is one search under way.
type Stream struct {
	Query string
	// Providers are the providers started when the search started, in
	// registration order (plus any paid provider started at once for a
	// deep search or because no free one could run). A provider escalated
	// later is in Route.Started, not here.
	Providers []string
	// Cooling are the providers the search skipped at its start: cooling
	// down, disabled, paced or capped. They are not failures of this search.
	Cooling []Cooling
	// Arrivals carries one Arrival per provider started (Route.Started),
	// in the order the calls complete, and is closed after the last.
	Arrivals <-chan Arrival
	// Served is how the cache answered (ServedMiss, ServedHit, ...); empty
	// when the stream did not pass through a Cache.
	Served string
	// CachedAt is when a hit's answer was first received.
	CachedAt time.Time
	// Route is the search's routing record, shared with every joiner of
	// the same search; nil for a cache hit (no provider was started).
	Route *Route
}

// Cooling is a provider a search skipped.
type Cooling struct {
	Provider string
	State    ProviderState
	Until    time.Time
	// Reason is why: the provider's last failure class (quota_limited,
	// rate_limited, auth_error, ...), "paced" or "capped". Never an error
	// summary.
	Reason string
}

// Arrival is one provider's completed call.
type Arrival struct {
	Provider  string
	Endpoint  string
	Status    string
	LatencyMs int64
	Results   []Result
	Error     string
}

// Answered reports whether the provider returned an answer, possibly empty.
func (a Arrival) Answered() bool {
	return a.Status == string(OutcomeSuccess) || a.Status == string(OutcomeSuccessEmpty)
}

// Escalation reasons: why a paid provider joined a search.
const (
	EscalateDeep   = "deep"    // the caller marked the search deep
	EscalateNoFree = "no_free" // no free provider could start
	EscalateFailed = "failed"  // every started call ended without an answer
	EscalateShort  = "short"   // every started call answered; too few distinct URLs
	EscalateSlow   = "slow"    // the step's soft deadline passed while short
)

// Escalation is one paid provider joining a search.
type Escalation struct {
	Provider string
	Reason   string
	// Unique is the distinct URLs the search had when it escalated.
	Unique int
}

// Route records how one search was routed. It is written by the search's
// coordinator and read by the handler and every joiner; safe for
// concurrent use.
type Route struct {
	mode string
	// reserve is the paid providers held when the search started.
	reserve []string

	mu          sync.Mutex
	started     []string
	paid        int
	escalations []Escalation
	skipped     []Cooling
	today       map[string]DailyCount
	todayOrder  []string
	done        bool
}

// RouteSummary is a copy of a Route.
type RouteSummary struct {
	Mode string
	// Reserve is the paid providers held at the start (not started then).
	Reserve []string
	// Started is every provider started, escalated ones included: the N
	// of "M of N".
	Started     []string
	PaidCalls   int
	Escalations []Escalation
	// Skipped are providers skipped after the start (at escalation).
	Skipped []Cooling
	// Today is the day's count after this search, for each provider it
	// called or skipped as capped, in the order touched.
	Today []DailyCount
	Done  bool
}

// Summary returns a copy of the route so far.
func (rt *Route) Summary() RouteSummary {
	if rt == nil {
		return RouteSummary{}
	}
	rt.mu.Lock()
	defer rt.mu.Unlock()
	out := RouteSummary{
		Mode:        rt.mode,
		Reserve:     append([]string{}, rt.reserve...),
		Started:     append([]string{}, rt.started...),
		PaidCalls:   rt.paid,
		Escalations: append([]Escalation{}, rt.escalations...),
		Skipped:     append([]Cooling{}, rt.skipped...),
		Done:        rt.done,
	}
	for _, name := range rt.todayOrder {
		out.Today = append(out.Today, rt.today[name])
	}
	return out
}

func (rt *Route) noteStarted(name string, paid bool) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.started = append(rt.started, name)
	if paid {
		rt.paid++
	}
}

func (rt *Route) noteCount(c DailyCount) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.today == nil {
		rt.today = map[string]DailyCount{}
	}
	if _, ok := rt.today[c.Provider]; !ok {
		rt.todayOrder = append(rt.todayOrder, c.Provider)
	}
	rt.today[c.Provider] = c
}

func (rt *Route) noteEscalation(e Escalation) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.escalations = append(rt.escalations, e)
}

func (rt *Route) noteSkipped(c Cooling) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.skipped = append(rt.skipped, c)
}

func (rt *Route) finish() {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.done = true
}

// pacer hands out call slots per provider, at most one per MinInterval.
type pacer struct {
	mu   sync.Mutex
	next map[string]time.Time
}

// reserve returns the provider's next slot and books it, or ok=false (and
// books nothing) when that slot is more than wait away.
func (p *pacer) reserve(provider string, interval, wait time.Duration, now time.Time) (slot time.Time, ok bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	slot = now
	if next := p.next[provider]; next.After(now) {
		slot = next
	}
	if slot.Sub(now) > wait {
		return slot, false
	}
	p.next[provider] = slot.Add(interval)
	return slot, true
}

// launch is one provider call admitted to a search.
type launch struct {
	provider Provider
	slot     time.Time
	paid     bool
	// at is the start time recorded with the call's outcome (sameEpisode).
	at time.Time
}

// admit paces and counts one call to p. It returns the call, or the reason
// the search skips p (paced, capped). A counting error lets a free provider
// through and skips a paid one: an unknown count never spends.
func (r *Router) admit(p Provider, now time.Time, route *Route) (launch, *Cooling) {
	name := p.Name()
	tier := r.TierOf(name)
	slot := now
	if interval := r.config.MinInterval[name]; interval > 0 {
		var ok bool
		slot, ok = r.pace.reserve(name, interval, r.config.PaceWait, now)
		if !ok {
			return launch{}, &Cooling{Provider: name, State: StatePaced, Until: slot, Reason: string(StatePaced)}
		}
	}
	day := DayOf(now)
	limit, until := r.limitAt(name, now)
	calls, ok, err := r.counter.ReserveCall(name, day, limit)
	if err != nil {
		log.Printf("searchplane: count call for %s: %v", name, err)
		if tier.Tier != TierFree {
			return launch{}, &Cooling{Provider: name, State: StateCapped, Reason: "count_error"}
		}
		ok = true
	}
	route.noteCount(DailyCount{Provider: name, Day: day, Calls: calls, Cap: r.capOf(name, now), Tier: tier})
	if !ok {
		return launch{}, &Cooling{Provider: name, State: StateCapped, Until: until, Reason: string(StateCapped)}
	}
	return launch{provider: p, slot: slot, paid: tier.Tier != TierFree, at: now}, nil
}

func nextUTCDay(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d+1, 0, 0, 0, 0, time.UTC)
}

// Start is StartRequest for an ordinary search.
func (r *Router) Start(ctx context.Context, query string, maxResults int) (*Stream, error) {
	return r.StartRequest(ctx, Request{Query: query, MaxResults: maxResults})
}

// StartRequest starts a search and returns before any provider answers.
// Each call is bounded by the request timeout and by ctx; a call cut short
// by ctx is the caller going away, never a provider strike. A paced
// provider's call first waits for its slot, at most PaceWait.
func (r *Router) StartRequest(ctx context.Context, req Request) (*Stream, error) {
	if len(r.providers) == 0 {
		return nil, fmt.Errorf("no search providers available")
	}
	now := r.now()
	health, err := r.store.Snapshot()
	if err != nil {
		return nil, err
	}
	budget := r.config.Routing == RoutingBudget
	route := &Route{mode: r.config.Routing}
	stream := &Stream{Query: req.Query, Route: route}
	todays, err := r.counter.CallsOn(DayOf(now))
	if err != nil {
		todays = map[string]int{}
	}
	var launches []launch
	var reserve []Provider
	for _, p := range r.providers {
		name := p.Name()
		rec := health[name]
		if rec.Provider == "" {
			rec.Provider = name
			rec.State = StateActive
		}
		if !IsEligible(rec, now) {
			skipped := Cooling{Provider: name, State: rec.State, Reason: rec.LastFailureClass}
			if rec.CooldownUntil != nil {
				skipped.Until = *rec.CooldownUntil
			}
			stream.Cooling = append(stream.Cooling, skipped)
			continue
		}
		if budget && r.TierOf(name).Tier != TierFree {
			if limit, until := r.limitAt(name, now); limit >= 0 && todays[name] >= limit {
				route.noteCount(DailyCount{Provider: name, Day: DayOf(now), Calls: todays[name], Cap: r.capOf(name, now), Tier: r.TierOf(name)})
				stream.Cooling = append(stream.Cooling, Cooling{Provider: name, State: StateCapped, Until: until, Reason: string(StateCapped)})
				continue
			}
			reserve = append(reserve, p)
			continue
		}
		l, skip := r.admit(p, now, route)
		if skip != nil {
			stream.Cooling = append(stream.Cooling, *skip)
			continue
		}
		launches = append(launches, l)
	}
	// Cheapest first: paid by cost, constrained last; ties keep
	// registration order.
	sort.SliceStable(reserve, func(i, j int) bool {
		a, b := r.TierOf(reserve[i].Name()), r.TierOf(reserve[j].Name())
		if a.rank() != b.rank() {
			return a.rank() < b.rank()
		}
		return a.Cost < b.Cost
	})
	for _, p := range reserve {
		route.reserve = append(route.reserve, p.Name())
	}

	co := &coordinator{
		r:          r,
		ctx:        ctx,
		req:        req,
		began:      now,
		route:      route,
		reserve:    reserve,
		in:         make(chan Arrival, len(r.providers)),
		out:        make(chan Arrival, len(r.providers)),
		seen:       map[string]bool{},
		step:       map[string]bool{},
		minResults: r.config.MinResults,
		maxPaid:    r.config.MaxPaid,
	}
	if req.Deep {
		co.minResults = r.config.DeepMinResults
		co.maxPaid = r.config.DeepMaxPaid
	}
	if req.MaxResults > 0 && req.MaxResults < co.minResults {
		co.minResults = req.MaxResults
	}
	for _, l := range launches {
		co.start(l)
	}
	if budget {
		switch {
		case req.Deep:
			co.escalate(EscalateDeep, false)
		case len(launches) == 0:
			co.escalate(EscalateNoFree, false)
		}
	}
	// The start line names everything started so far, and every provider
	// skipped so far; Route.skipped keeps only the skips after the start.
	route.mu.Lock()
	stream.Providers = append([]string{}, route.started...)
	stream.Cooling = append(stream.Cooling, route.skipped...)
	route.skipped = nil
	route.mu.Unlock()
	stream.Arrivals = co.out
	go co.run()
	return stream, nil
}

// coordinator runs one search: it forwards every arrival, and under
// budget routing decides after each step whether the next paid provider
// joins. Only its goroutine touches its fields after StartRequest returns.
type coordinator struct {
	r       *Router
	ctx     context.Context
	req     Request
	began   time.Time
	route   *Route
	reserve []Provider

	in  chan Arrival
	out chan Arrival

	outstanding int
	answered    int // started calls that answered
	arrived     int
	seen        map[string]bool // distinct result URLs
	step        map[string]bool // calls of the current step still out
	paid        int
	minResults  int
	maxPaid     int
	stepTimer   *time.Timer
}

func (co *coordinator) start(l launch) {
	co.outstanding++
	co.step[l.provider.Name()] = true
	co.route.noteStarted(l.provider.Name(), l.paid)
	go func() {
		co.in <- co.r.call(co.ctx, l.provider, co.req.Query, co.req.MaxResults, l.at, l.slot)
	}()
}

// short reports whether the search has fewer distinct URLs than it wants.
func (co *coordinator) short() bool { return len(co.seen) < co.minResults }

// escalate starts the next paid provider that can run, cheapest first,
// skipping (and recording) any benched, paced or capped since the search
// began. It reports whether one started. A new step's soft deadline
// covers only the provider it starts; otherwise (a deep search's first
// paid provider) it joins the free providers' step.
func (co *coordinator) escalate(reason string, newStep bool) bool {
	if co.r.config.Routing != RoutingBudget || co.ctx.Err() != nil {
		return false
	}
	if co.paid >= co.maxPaid || co.r.now().Sub(co.began) > co.r.config.EscalateWindow {
		return false
	}
	for len(co.reserve) > 0 {
		p := co.reserve[0]
		co.reserve = co.reserve[1:]
		now := co.r.now()
		rec, err := co.r.store.Get(p.Name())
		if err == nil && !IsEligible(rec, now) {
			skipped := Cooling{Provider: p.Name(), State: rec.State, Reason: rec.LastFailureClass}
			if rec.CooldownUntil != nil {
				skipped.Until = *rec.CooldownUntil
			}
			co.route.noteSkipped(skipped)
			continue
		}
		l, skip := co.r.admit(p, now, co.route)
		if skip != nil {
			co.route.noteSkipped(*skip)
			continue
		}
		co.paid++
		co.route.noteEscalation(Escalation{Provider: p.Name(), Reason: reason, Unique: len(co.seen)})
		if newStep {
			co.step = map[string]bool{}
		}
		co.start(l)
		co.resetTimer()
		return true
	}
	return false
}

func (co *coordinator) resetTimer() {
	if co.stepTimer == nil {
		co.stepTimer = time.NewTimer(co.r.config.EscalateAfter)
		return
	}
	co.stepTimer.Stop()
	co.stepTimer.Reset(co.r.config.EscalateAfter)
}

func (co *coordinator) run() {
	defer close(co.out)
	defer co.route.finish()
	budget := co.r.config.Routing == RoutingBudget
	if budget {
		co.resetTimer()
		defer co.stepTimer.Stop()
	}
	var timerC <-chan time.Time
	done := co.ctx.Done()
	for co.outstanding > 0 {
		if co.stepTimer != nil {
			timerC = co.stepTimer.C
		}
		select {
		case a := <-co.in:
			co.outstanding--
			co.arrived++
			if a.Answered() {
				co.answered++
			}
			for _, hit := range a.Results {
				if key := resultKey(hit.URL); key != "" {
					co.seen[key] = true
				}
			}
			delete(co.step, a.Provider)
			co.out <- a
			if budget && len(co.step) == 0 && co.short() {
				reason := EscalateShort
				if co.answered == 0 {
					reason = EscalateFailed
				}
				co.escalate(reason, true)
			}
		case <-timerC:
			if budget && len(co.step) > 0 && co.short() {
				co.escalate(EscalateSlow, true)
			}
		case <-done:
			// The caller left: no further escalation; the calls in flight
			// end on the same context and still arrive.
			done = nil
		}
	}
}

// resultKey is a result URL as counted for distinctness: no fragment, no
// trailing slash.
func resultKey(raw string) string {
	for i := 0; i < len(raw); i++ {
		if raw[i] == '#' {
			raw = raw[:i]
			break
		}
	}
	for len(raw) > 0 && raw[len(raw)-1] == '/' {
		raw = raw[:len(raw)-1]
	}
	return raw
}

func (r *Router) call(ctx context.Context, provider Provider, query string, maxResults int, now, slot time.Time) Arrival {
	name := provider.Name()
	if wait := slot.Sub(r.now()); wait > 0 {
		timer := time.NewTimer(wait)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			// The caller left while the call waited for its slot: nothing
			// was sent, and nothing is charged to the provider.
			return Arrival{Provider: name, Endpoint: r.endpoint(name), Status: AttemptStatus(OutcomeCallerGone), Error: ctx.Err().Error()}
		}
	}
	callCtx, cancel := context.WithTimeout(ctx, r.config.RequestTimeout)
	defer cancel()
	started := time.Now()
	hits, err := provider.Search(callCtx, query, maxResults)
	class := ClassifyCall(err, len(hits))
	// The caller going away (cancel, or its own deadline before the request
	// timeout) is not the provider failing: one cancelled search must not
	// cool the whole plane down.
	callerGone := err != nil && ctx.Err() != nil
	if callerGone {
		class = OutcomeCallerGone
	}
	arrival := Arrival{
		Provider:  name,
		Endpoint:  r.endpoint(name),
		Status:    AttemptStatus(class),
		LatencyMs: time.Since(started).Milliseconds(),
	}
	if err != nil {
		arrival.Error = truncateSummary(err.Error())
		if !callerGone {
			// The journal's skip reason says only the class ("exa=error");
			// the failing stage and HTTP status say why. Never the key, never
			// the body (FailureFields).
			status, stage := FailureFields(err)
			log.Printf("searchplane: %s call failed class=%s http_status=%s stage=%s latency_ms=%d",
				name, class, status, stage, arrival.LatencyMs)
		}
	} else {
		for i := range hits {
			hits[i].Provider = name
		}
		arrival.Results = hits
	}
	if !callerGone {
		// At is when the search started (or, for an escalated call, when it
		// joined): a failure from a call that was in flight when another
		// call benched the provider is the same episode and adds no strike
		// (sameEpisode).
		_, _ = r.store.RecordOutcome(Outcome{
			Provider: name,
			Class:    class,
			Results:  len(hits),
			Error:    arrival.Error,
			At:       now,
		})
	}
	return arrival
}

func (r *Router) endpoint(provider string) string {
	if r.config.EndpointFor != nil {
		return r.config.EndpointFor(provider)
	}
	return ""
}
