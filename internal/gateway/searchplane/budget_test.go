package searchplane

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Budget routing (docs/archive/reports/search-efficiency-and-reuse-2026-10-14.md
// §4.1). Failure modes, numbered as there:
//  1. The free tier is sufficient and a paid provider is called anyway.
//  2. The free tier falls short and no paid provider is reached, including
//     when a free provider hangs.
//  3. Escalation starts every paid provider at once, not cheapest first, or
//     past MaxPaid.
//  4. A cap is overshot by concurrent searches, counts are lost on restart,
//     or the day rolls at local rather than UTC midnight.
//  5. A capped provider is a failure or strike, is in N, or has no reason.
//  6. A paid provider benched, paced or capped by the time it is reached is
//     called anyway, or stops escalation instead of yielding to the next.
//  7. A cache hit or join calls or counts a provider.
//  9. The fan-out switch does not start every eligible provider.
//  10. Escalation outlives its window.
//  11. Shared state races (run with -race).

// callLog records provider call starts and ends in order.
type callLog struct {
	mu     sync.Mutex
	events []string
}

func (l *callLog) add(e string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, e)
}

func (l *callLog) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return strings.Join(l.events, " ")
}

// logged answers n distinct results after delay and records its call.
func logged(l *callLog, name string, delay time.Duration, n int) *stubProvider {
	return &stubProvider{name: name, searchFn: func(ctx context.Context, q string, max int) ([]Result, error) {
		l.add("+" + name)
		defer l.add("-" + name)
		select {
		case <-time.After(delay):
			return manyResults(name, n), nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}}
}

func failing(l *callLog, name string) *stubProvider {
	return &stubProvider{name: name, searchFn: func(ctx context.Context, q string, max int) ([]Result, error) {
		l.add("+" + name)
		defer l.add("-" + name)
		return nil, errors.New("status 500: upstream")
	}}
}

func budgetConfig() Config {
	return Config{
		RequestTimeout: 5 * time.Second,
		Routing:        RoutingBudget,
		Tiers:          ParseProviderTiers(DefaultProviderTiers),
		MinResults:     10,
		MaxPaid:        3,
		EscalateAfter:  2 * time.Second,
		EscalateWindow: 5 * time.Second,
	}
}

func names(arrivals []Arrival) string {
	out := make([]string, len(arrivals))
	for i, a := range arrivals {
		out[i] = a.Provider
	}
	return strings.Join(out, ",")
}

func reasons(route RouteSummary) string {
	out := make([]string, len(route.Escalations))
	for i, e := range route.Escalations {
		out[i] = e.Reason + ":" + e.Provider
	}
	return strings.Join(out, ",")
}

func TestBudget_FreeSufficientMakesNoPaidCalls(t *testing.T) { // 1
	var l callLog
	tavily, serper := logged(&l, "tavily", 0, 5), logged(&l, "serper", 0, 5)
	router := NewRouter([]Provider{tavily, logged(&l, "keenable", 0, 12), serper}, NewMemoryHealthStore(), budgetConfig())
	stream, err := router.Start(context.Background(), "q", 40)
	if err != nil {
		t.Fatal(err)
	}
	got := drain(t, stream)
	if names(got) != "keenable" {
		t.Fatalf("arrivals = %s, want keenable only", names(got))
	}
	if tavily.searchCnt.Load()+serper.searchCnt.Load() != 0 {
		t.Fatalf("paid providers called: %s", l.String())
	}
	route := stream.Route.Summary()
	if fmt.Sprint(stream.Providers) != "[keenable]" || route.PaidCalls != 0 || len(route.Escalations) != 0 {
		t.Fatalf("providers=%v route=%+v", stream.Providers, route)
	}
	// Held in reserve, cheapest first; not cooling.
	if fmt.Sprint(route.Reserve) != "[serper tavily]" || len(stream.Cooling) != 0 {
		t.Fatalf("reserve=%v cooling=%v", route.Reserve, stream.Cooling)
	}
}

func TestBudget_ShortEscalatesOneAtATimeCheapestFirst(t *testing.T) { // 2, 3
	var l callLog
	providers := []Provider{
		logged(&l, "tavily", 20*time.Millisecond, 3),
		logged(&l, "serpapi", 20*time.Millisecond, 3),
		logged(&l, "keenable", 0, 3),
		logged(&l, "serper", 20*time.Millisecond, 3),
		logged(&l, "parallel", 20*time.Millisecond, 3),
	}
	router := NewRouter(providers, NewMemoryHealthStore(), budgetConfig())
	stream, err := router.Start(context.Background(), "q", 40)
	if err != nil {
		t.Fatal(err)
	}
	got := drain(t, stream)
	// 3 → serper 6 → parallel 9 → tavily 12 ≥ 10: serpapi never called.
	if names(got) != "keenable,serper,parallel,tavily" {
		t.Fatalf("arrivals = %s", names(got))
	}
	if want := "+keenable -keenable +serper -serper +parallel -parallel +tavily -tavily"; l.String() != want {
		t.Fatalf("calls = %q, want one paid provider at a time, cheapest first: %q", l.String(), want)
	}
	route := stream.Route.Summary()
	if reasons(route) != "short:serper,short:parallel,short:tavily" || route.PaidCalls != 3 {
		t.Fatalf("route = %+v", route)
	}
	if fmt.Sprint(route.Started) != "[keenable serper parallel tavily]" {
		t.Fatalf("started = %v", route.Started)
	}
}

func TestBudget_MaxPaidBoundsEscalation(t *testing.T) { // 3
	var l callLog
	cfg := budgetConfig()
	cfg.MaxPaid = 1
	router := NewRouter([]Provider{logged(&l, "keenable", 0, 1), logged(&l, "serper", 0, 1), logged(&l, "parallel", 0, 1)}, NewMemoryHealthStore(), cfg)
	stream, _ := router.Start(context.Background(), "q", 40)
	if got := names(drain(t, stream)); got != "keenable,serper" {
		t.Fatalf("arrivals = %s, want one paid provider", got)
	}
}

func TestBudget_MinResultsIsBoundedByMaxResults(t *testing.T) {
	var l callLog
	router := NewRouter([]Provider{logged(&l, "keenable", 0, 5), logged(&l, "serper", 0, 5)}, NewMemoryHealthStore(), budgetConfig())
	stream, _ := router.Start(context.Background(), "q", 5)
	if got := names(drain(t, stream)); got != "keenable" {
		t.Fatalf("arrivals = %s: five results answer a five-result search", got)
	}
}

func TestBudget_DuplicateURLsDoNotCount(t *testing.T) {
	var l callLog
	same := func(name string) *stubProvider {
		return &stubProvider{name: name, searchFn: func(ctx context.Context, q string, max int) ([]Result, error) {
			l.add("+" + name)
			out := manyResults("dup", 6)
			for i := range out {
				out[i].URL += "/#frag"
			}
			return out, nil
		}}
	}
	router := NewRouter([]Provider{logged(&l, "keenable", 0, 6), same("brave"), logged(&l, "serper", 0, 6)}, NewMemoryHealthStore(), func() Config {
		c := budgetConfig()
		c.Tiers["keenable"] = ProviderTier{Tier: TierFree}
		return c
	}())
	// keenable's six and brave's six are distinct sets: 12 ≥ 10.
	stream, _ := router.Start(context.Background(), "q", 40)
	if got := names(drain(t, stream)); strings.Contains(got, "serper") {
		t.Fatalf("arrivals = %s", got)
	}
	// Two free providers returning the same six URLs are six, not twelve.
	router = NewRouter([]Provider{same("keenable"), same("brave"), logged(&l, "serper", 0, 6)}, NewMemoryHealthStore(), budgetConfig())
	stream, _ = router.Start(context.Background(), "q", 40)
	drain(t, stream)
	if route := stream.Route.Summary(); reasons(route) != "short:serper" || route.Escalations[0].Unique != 6 {
		t.Fatalf("route = %+v, want escalation at 6 distinct URLs", route)
	}
}

func TestBudget_FailedFreeEscalates(t *testing.T) { // 2
	var l callLog
	router := NewRouter([]Provider{failing(&l, "keenable"), logged(&l, "serper", 0, 12)}, NewMemoryHealthStore(), budgetConfig())
	stream, _ := router.Start(context.Background(), "q", 40)
	if got := names(drain(t, stream)); got != "keenable,serper" {
		t.Fatalf("arrivals = %s", got)
	}
	if r := reasons(stream.Route.Summary()); r != "failed:serper" {
		t.Fatalf("reasons = %s", r)
	}
}

func TestBudget_NoFreeProviderStartsTheCheapestPaidAtOnce(t *testing.T) { // 2
	var l callLog
	store := NewMemoryHealthStore()
	if _, err := store.RecordOutcome(Outcome{Provider: "keenable", Class: OutcomeQuotaLimited, Error: "432 usage limit"}); err != nil {
		t.Fatal(err)
	}
	router := NewRouter([]Provider{logged(&l, "keenable", 0, 12), logged(&l, "parallel", 0, 12), logged(&l, "serper", 0, 12)}, store, budgetConfig())
	stream, _ := router.Start(context.Background(), "q", 40)
	if fmt.Sprint(stream.Providers) != "[serper]" {
		t.Fatalf("start providers = %v, want the cheapest paid provider named on the start line", stream.Providers)
	}
	if len(stream.Cooling) != 1 || stream.Cooling[0].Provider != "keenable" || stream.Cooling[0].Reason != string(OutcomeQuotaLimited) {
		t.Fatalf("cooling = %+v", stream.Cooling)
	}
	if got := names(drain(t, stream)); got != "serper" {
		t.Fatalf("arrivals = %s", got)
	}
	if r := reasons(stream.Route.Summary()); r != "no_free:serper" {
		t.Fatalf("reasons = %s", r)
	}
}

func TestBudget_SlowFreeEscalatesAtTheSoftDeadline(t *testing.T) { // 2
	var l callLog
	cfg := budgetConfig()
	cfg.EscalateAfter = 50 * time.Millisecond
	router := NewRouter([]Provider{logged(&l, "keenable", time.Second, 12), logged(&l, "serper", 0, 3)}, NewMemoryHealthStore(), cfg)
	stream, _ := router.Start(context.Background(), "q", 40)
	got := drain(t, stream)
	if names(got) != "serper,keenable" {
		t.Fatalf("arrivals = %s, want serper before the hanging free provider", names(got))
	}
	if r := reasons(stream.Route.Summary()); r != "slow:serper" {
		t.Fatalf("reasons = %s", r)
	}
}

func TestBudget_NoEscalationAfterTheWindow(t *testing.T) { // 10
	var l callLog
	cfg := budgetConfig()
	cfg.EscalateWindow = 30 * time.Millisecond
	router := NewRouter([]Provider{logged(&l, "keenable", 100*time.Millisecond, 1), logged(&l, "serper", 0, 12)}, NewMemoryHealthStore(), cfg)
	stream, _ := router.Start(context.Background(), "q", 40)
	if got := names(drain(t, stream)); got != "keenable" {
		t.Fatalf("arrivals = %s, want no paid provider after the window", got)
	}
}

func TestBudget_DeepStartsTheCheapestPaidWithTheFreeTier(t *testing.T) {
	var l callLog
	cfg := budgetConfig()
	cfg.DeepMinResults = 30
	router := NewRouter([]Provider{logged(&l, "keenable", 20*time.Millisecond, 40), logged(&l, "parallel", 0, 5), logged(&l, "serper", 0, 5)}, NewMemoryHealthStore(), cfg)
	stream, err := router.StartRequest(context.Background(), Request{Query: "q", MaxResults: 40, Deep: true})
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(stream.Providers) != "[keenable serper]" {
		t.Fatalf("start providers = %v", stream.Providers)
	}
	drain(t, stream)
	if r := reasons(stream.Route.Summary()); r != "deep:serper" {
		t.Fatalf("reasons = %s, want deep only (45 ≥ 30)", r)
	}
}

func TestBudget_CapSkipsWithReasonAndNoStrike(t *testing.T) { // 5
	var l callLog
	cfg := budgetConfig()
	cfg.DailyCaps = map[string]int{"serper": 1}
	store := NewMemoryHealthStore()
	serper := logged(&l, "serper", 0, 1)
	router := NewRouter([]Provider{logged(&l, "keenable", 0, 1), serper, logged(&l, "parallel", 0, 20)}, store, cfg)

	first, _ := router.Start(context.Background(), "a", 40)
	if got := names(drain(t, first)); got != "keenable,serper,parallel" {
		t.Fatalf("first arrivals = %s", got)
	}
	second, _ := router.Start(context.Background(), "b", 40)
	if fmt.Sprint(second.Providers) != "[keenable]" {
		t.Fatalf("a capped provider is in N: %v", second.Providers)
	}
	if len(second.Cooling) != 1 || second.Cooling[0].Provider != "serper" || second.Cooling[0].State != StateCapped || second.Cooling[0].Reason != "capped" {
		t.Fatalf("cooling = %+v, want serper capped", second.Cooling)
	}
	if got := names(drain(t, second)); got != "keenable,parallel" {
		t.Fatalf("second arrivals = %s, want parallel in place of capped serper", got)
	}
	if n := serper.searchCnt.Load(); n != 1 {
		t.Fatalf("serper called %d times, cap 1", n)
	}
	rec, _ := store.Get("serper")
	if rec.StrikeCount != 0 || rec.State != StateActive {
		t.Fatalf("capped provider recorded as a failure: %+v", rec)
	}
	today := second.Route.Summary().Today
	if len(today) == 0 || today[1].Provider != "serper" || today[1].Calls != 1 || today[1].Cap != 1 {
		t.Fatalf("today = %+v", today)
	}
}

// gatedFree answers 1 result once release is closed.
func gatedFree(name string, release <-chan struct{}) *stubProvider {
	return &stubProvider{name: name, searchFn: func(ctx context.Context, q string, max int) ([]Result, error) {
		<-release
		return manyResults(name, 1), nil
	}}
}

func TestBudget_ProviderCappedBenchedOrPacedAtEscalationYieldsToTheNext(t *testing.T) { // 6
	cases := map[string]struct {
		mutate func(r *Router, store *MemoryHealthStore)
		reason string
	}{
		"capped": {func(r *Router, store *MemoryHealthStore) {
			_, _, _ = store.ReserveCall("serper", DayOf(r.now()), NoCap)
		}, "capped"},
		"benched": {func(r *Router, store *MemoryHealthStore) {
			_, _ = store.RecordOutcome(Outcome{Provider: "serper", Class: OutcomeAuthError, Error: "401"})
		}, string(OutcomeAuthError)},
		"paced": {func(r *Router, store *MemoryHealthStore) {
			r.pace.reserve("serper", time.Hour, time.Hour, time.Now())
		}, "paced"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var l callLog
			cfg := budgetConfig()
			cfg.DailyCaps = map[string]int{"serper": 1}
			cfg.MinInterval = map[string]time.Duration{"serper": time.Hour}
			cfg.PaceWait = time.Millisecond
			store := NewMemoryHealthStore()
			release := make(chan struct{})
			serper := logged(&l, "serper", 0, 20)
			router := NewRouter([]Provider{gatedFree("keenable", release), serper, logged(&l, "parallel", 0, 20)}, store, cfg)
			stream, _ := router.Start(context.Background(), "q", 40)
			tc.mutate(router, store)
			close(release)
			if got := names(drain(t, stream)); got != "keenable,parallel" {
				t.Fatalf("arrivals = %s", got)
			}
			if serper.searchCnt.Load() != 0 {
				t.Fatal("serper called")
			}
			route := stream.Route.Summary()
			if len(route.Skipped) != 1 || route.Skipped[0].Provider != "serper" || route.Skipped[0].Reason != tc.reason {
				t.Fatalf("skipped = %+v, want serper=%s", route.Skipped, tc.reason)
			}
			if reasons(route) != "short:parallel" {
				t.Fatalf("reasons = %s", reasons(route))
			}
		})
	}
}

func TestBudget_ConcurrentSearchesNeverOvershootACap(t *testing.T) { // 4, 11
	for _, mode := range []string{RoutingBudget, RoutingFanout} {
		t.Run(mode, func(t *testing.T) {
			var l callLog
			cfg := budgetConfig()
			cfg.Routing = mode
			cfg.DailyCaps = map[string]int{"serper": 5, "parallel": 3}
			serper, parallel := logged(&l, "serper", time.Millisecond, 1), logged(&l, "parallel", time.Millisecond, 1)
			path := filepath.Join(t.TempDir(), "health.db")
			store, err := OpenFileHealthStore(path)
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			router := NewRouter([]Provider{logged(&l, "keenable", time.Millisecond, 1), serper, parallel}, store, cfg)
			var wg sync.WaitGroup
			for i := 0; i < 40; i++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					s, err := router.Start(context.Background(), fmt.Sprint("q", i), 40)
					if err != nil {
						t.Error(err)
						return
					}
					for range s.Arrivals {
					}
					_ = s.Route.Summary()
				}(i)
			}
			wg.Wait()
			if serper.searchCnt.Load() != 5 || parallel.searchCnt.Load() != 3 {
				t.Fatalf("serper=%d parallel=%d, want exactly the caps 5 and 3", serper.searchCnt.Load(), parallel.searchCnt.Load())
			}
			counts, _ := router.DailyCounts()
			byName := map[string]DailyCount{}
			for _, c := range counts {
				byName[c.Provider] = c
			}
			if byName["keenable"].Calls != 40 || byName["serper"].Calls != 5 || byName["parallel"].Calls != 3 || byName["keenable"].Cap != NoCap {
				t.Fatalf("counts = %+v", counts)
			}
		})
	}
}

func TestBudget_CountsSurviveRestartAndRollAtUTCMidnight(t *testing.T) { // 4
	path := filepath.Join(t.TempDir(), "search-health.db")
	la, _ := time.LoadLocation("America/Los_Angeles")
	// 16:30 in Los Angeles is 23:30 UTC on 10-14; 17:30 is 00:30 UTC on 10-15.
	evening := time.Date(2026, 10, 14, 16, 30, 0, 0, la)
	later := evening.Add(time.Hour)
	cfg := Config{RequestTimeout: time.Second, DailyCaps: map[string]int{"serper": 2}}

	store, err := OpenFileHealthStore(path)
	if err != nil {
		t.Fatal(err)
	}
	serper := &stubProvider{name: "serper", searchFn: func(ctx context.Context, q string, max int) ([]Result, error) { return manyResults("s", 1), nil }}
	router := NewRouter([]Provider{serper}, store, cfg)
	router.now = func() time.Time { return evening }
	for i := 0; i < 2; i++ {
		s, _ := router.Start(context.Background(), "q", 5)
		drain(t, s)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	// The gateway restarts: the counts are in the health DB.
	store, err = OpenFileHealthStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	router = NewRouter([]Provider{serper}, store, cfg)
	router.now = func() time.Time { return evening }
	s, _ := router.Start(context.Background(), "q", 5)
	if len(s.Providers) != 0 || len(s.Cooling) != 1 || s.Cooling[0].Reason != "capped" {
		t.Fatalf("after restart: providers=%v cooling=%+v, want serper capped", s.Providers, s.Cooling)
	}
	drain(t, s)
	if !s.Cooling[0].Until.Equal(time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("capped until %v, want UTC midnight", s.Cooling[0].Until)
	}

	// Past UTC midnight (still the 14th in Los Angeles) the day is new.
	router.now = func() time.Time { return later }
	s, _ = router.Start(context.Background(), "q", 5)
	if got := names(drain(t, s)); got != "serper" {
		t.Fatalf("after UTC midnight arrivals = %s", got)
	}
	if n := serper.searchCnt.Load(); n != 3 {
		t.Fatalf("serper calls = %d", n)
	}
	old, _ := store.CallsOn("2026-10-14")
	if old["serper"] != 2 {
		t.Fatalf("10-14 count = %v", old)
	}
}

func TestBudget_FanoutStartsEveryEligibleProvider(t *testing.T) { // 9
	var l callLog
	cfg := budgetConfig()
	cfg.Routing = RoutingFanout
	router := NewRouter([]Provider{logged(&l, "keenable", 0, 40), logged(&l, "serper", 0, 1), logged(&l, "chatgpt", 0, 1)}, NewMemoryHealthStore(), cfg)
	stream, _ := router.Start(context.Background(), "q", 40)
	if fmt.Sprint(stream.Providers) != "[keenable serper chatgpt]" {
		t.Fatalf("providers = %v", stream.Providers)
	}
	drain(t, stream)
	if route := stream.Route.Summary(); route.Mode != RoutingFanout || route.PaidCalls != 2 || len(route.Reserve) != 0 {
		t.Fatalf("route = %+v", route)
	}
}

func TestBudget_CacheHitAndJoinMakeNoProviderCalls(t *testing.T) { // 7
	var l callLog
	release := make(chan struct{})
	keenable := &stubProvider{name: "keenable", searchFn: func(ctx context.Context, q string, max int) ([]Result, error) {
		<-release
		return manyResults("k", 2), nil
	}}
	serper := logged(&l, "serper", 0, 12)
	router := NewRouter([]Provider{keenable, serper}, NewMemoryHealthStore(), budgetConfig())
	c := NewCache(CacheConfig{TTL: time.Minute, Bound: 10 * time.Second})
	start := func(ctx context.Context) (*Stream, error) { return router.Start(ctx, "q", 40) }

	leader, err := c.Start(context.Background(), "vm-a", "q", 40, start)
	if err != nil {
		t.Fatal(err)
	}
	joiner, err := c.Start(context.Background(), "vm-a", "q", 40, start)
	if err != nil {
		t.Fatal(err)
	}
	close(release)
	// The joiner receives the leader's escalated provider too.
	if got := names(drain(t, joiner)); got != "keenable,serper" {
		t.Fatalf("joiner arrivals = %s", got)
	}
	if got := names(drain(t, leader)); got != "keenable,serper" {
		t.Fatalf("leader arrivals = %s", got)
	}
	if joiner.Served != ServedJoined || joiner.Route != leader.Route {
		t.Fatalf("joiner served=%s shares route=%v", joiner.Served, joiner.Route == leader.Route)
	}
	c.WaitIdleForTest()

	hit, _ := c.Start(context.Background(), "vm-a", "q", 40, start)
	drain(t, hit)
	if hit.Served != ServedHit || hit.Route != nil {
		t.Fatalf("served=%s route=%v", hit.Served, hit.Route)
	}
	if keenable.searchCnt.Load() != 1 || serper.searchCnt.Load() != 1 {
		t.Fatalf("calls keenable=%d serper=%d, want one each for three searches", keenable.searchCnt.Load(), serper.searchCnt.Load())
	}
	counts, _ := router.DailyCounts()
	for _, cnt := range counts {
		if cnt.Calls != 1 {
			t.Fatalf("counts = %+v, want one call each", counts)
		}
	}
}

func TestBudget_ConcurrentMixedSearchesRace(t *testing.T) { // 11
	var short atomic.Bool
	keenable := &stubProvider{name: "keenable", searchFn: func(ctx context.Context, q string, max int) ([]Result, error) {
		if short.Load() {
			return manyResults(q, 2), nil
		}
		return manyResults(q, 20), nil
	}}
	paid := func(name string) *stubProvider {
		return &stubProvider{name: name, searchFn: func(ctx context.Context, q string, max int) ([]Result, error) {
			return manyResults(name+q, 4), nil
		}}
	}
	cfg := budgetConfig()
	cfg.EscalateAfter = 5 * time.Millisecond
	router := NewRouter([]Provider{keenable, paid("serper"), paid("parallel"), paid("tavily")}, NewMemoryHealthStore(), cfg)
	var wg sync.WaitGroup
	for i := 0; i < 60; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			short.Store(i%2 == 0)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			s, err := router.StartRequest(ctx, Request{Query: fmt.Sprint("q", i), MaxResults: 40, Deep: i%5 == 0})
			if err != nil {
				t.Error(err)
				return
			}
			if i%7 == 0 {
				cancel()
			}
			n := 0
			for range s.Arrivals {
				n++
				_ = s.Route.Summary()
			}
			if sum := s.Route.Summary(); n != len(sum.Started) || !sum.Done {
				t.Errorf("search %d: %d arrivals for %d started (done=%v)", i, n, len(sum.Started), sum.Done)
			}
		}(i)
	}
	wg.Wait()
}

func TestConfigFromEnv_BudgetDefaultsAndSwitches(t *testing.T) {
	cfg := ConfigFromEnv()
	if cfg.Routing != RoutingBudget || cfg.DailyCaps["tavily"] != 150 || cfg.DailyCaps["chatgpt"] != 50 {
		t.Fatalf("defaults: routing=%s caps=%v", cfg.Routing, cfg.DailyCaps)
	}
	if _, ok := cfg.DailyCaps["keenable"]; ok {
		t.Fatal("free provider capped by default")
	}
	if cfg.Tiers["brave"].Tier != TierFree || cfg.Tiers["chatgpt"].Tier != TierConstrained || cfg.Tiers["serper"].Cost != 1 {
		t.Fatalf("tiers = %v", cfg.Tiers)
	}
	if EscalationWindow(cfg) != DefaultEscalateWindow {
		t.Fatalf("window = %v", EscalationWindow(cfg))
	}

	t.Setenv("CHOIR_SEARCH_ROUTING", "fanout")
	t.Setenv("CHOIR_SEARCH_PROVIDER_DAILY_CAPS", "none")
	cfg = ConfigFromEnv()
	if cfg.Routing != RoutingFanout || len(cfg.DailyCaps) != 0 || EscalationWindow(cfg) != 0 {
		t.Fatalf("rollback switch: routing=%s caps=%v window=%v", cfg.Routing, cfg.DailyCaps, EscalationWindow(cfg))
	}

	t.Setenv("CHOIR_SEARCH_ROUTING", "")
	t.Setenv("CHOIR_SEARCH_PROVIDER_DAILY_CAPS", "tavily=500,exa=none,bogus=x")
	t.Setenv("CHOIR_SEARCH_PROVIDER_TIERS", "brave=paid:5,exa=free,weird=gold")
	cfg = ConfigFromEnv()
	if cfg.DailyCaps["tavily"] != 500 || cfg.DailyCaps["serper"] != 150 {
		t.Fatalf("caps merge: %v", cfg.DailyCaps)
	}
	if _, ok := cfg.DailyCaps["exa"]; ok {
		t.Fatalf("exa=none kept a cap: %v", cfg.DailyCaps)
	}
	if cfg.Tiers["brave"] != (ProviderTier{Tier: TierPaid, Cost: 5}) || cfg.Tiers["exa"].Tier != TierFree {
		t.Fatalf("tiers merge: %v", cfg.Tiers)
	}
	if _, ok := cfg.Tiers["weird"]; ok {
		t.Fatal("malformed tier kept")
	}
	r := NewRouter(nil, nil, cfg)
	if tier := r.TierOf("unknown"); tier.Tier != TierPaid || tier.Cost != UnknownProviderCost {
		t.Fatalf("unknown provider tier = %+v", tier)
	}
}
