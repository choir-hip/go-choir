package searchplane

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Router starts searches across its providers with durable health.
type Router struct {
	providers []Provider
	store     HealthStore
	config    Config
	pace      *pacer
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
	return &Router{providers: providers, store: store, config: config, pace: &pacer{next: map[string]time.Time{}}}
}

// HealthStore exposes the underlying store for ops endpoints.
func (r *Router) HealthStore() HealthStore { return r.store }

// RequestTimeout is the bound on each provider call.
func (r *Router) RequestTimeout() time.Duration { return r.config.RequestTimeout }

// Stream is one search under way across every eligible provider.
type Stream struct {
	Query string
	// Providers are the providers eligible when the search started, in
	// registration order; every one of them is called, and it is the N of
	// "M of N answered".
	Providers []string
	// Cooling are the providers the search skipped: cooling down, disabled,
	// or paced. They are not failures of this search.
	Cooling []Cooling
	// Arrivals carries one Arrival per provider in Providers, in the order
	// the calls complete, and is closed after the last.
	Arrivals <-chan Arrival
	// Served is how the cache answered (ServedMiss, ServedHit, ...); empty
	// when the stream did not pass through a Cache.
	Served string
	// CachedAt is when a hit's answer was first received.
	CachedAt time.Time
}

// Cooling is a provider a search skipped.
type Cooling struct {
	Provider string
	State    ProviderState
	Until    time.Time
	// Reason is why: the provider's last failure class (quota_limited,
	// rate_limited, auth_error, ...) or "paced". Never an error summary.
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

// Start calls every eligible provider at once and returns before any of
// them answers. Each call is bounded by the request timeout and by ctx; a
// call cut short by ctx is the caller going away, never a provider strike.
// A paced provider's call first waits for its slot, at most PaceWait.
func (r *Router) Start(ctx context.Context, query string, maxResults int) (*Stream, error) {
	if len(r.providers) == 0 {
		return nil, fmt.Errorf("no search providers available")
	}
	now := time.Now()
	health, err := r.store.Snapshot()
	if err != nil {
		return nil, err
	}
	stream := &Stream{Query: query}
	type start struct {
		provider Provider
		slot     time.Time
	}
	var eligible []start
	for _, p := range r.providers {
		rec := health[p.Name()]
		if rec.Provider == "" {
			rec.Provider = p.Name()
			rec.State = StateActive
		}
		if !IsEligible(rec, now) {
			skipped := Cooling{Provider: p.Name(), State: rec.State, Reason: rec.LastFailureClass}
			if rec.CooldownUntil != nil {
				skipped.Until = *rec.CooldownUntil
			}
			stream.Cooling = append(stream.Cooling, skipped)
			continue
		}
		slot := now
		if interval := r.config.MinInterval[p.Name()]; interval > 0 {
			var ok bool
			slot, ok = r.pace.reserve(p.Name(), interval, r.config.PaceWait, now)
			if !ok {
				stream.Cooling = append(stream.Cooling, Cooling{Provider: p.Name(), State: StatePaced, Until: slot, Reason: string(StatePaced)})
				continue
			}
		}
		eligible = append(eligible, start{provider: p, slot: slot})
		stream.Providers = append(stream.Providers, p.Name())
	}
	arrivals := make(chan Arrival, len(eligible))
	stream.Arrivals = arrivals
	var wg sync.WaitGroup
	for _, s := range eligible {
		wg.Add(1)
		go func(s start) {
			defer wg.Done()
			arrivals <- r.call(ctx, s.provider, query, maxResults, now, s.slot)
		}(s)
	}
	go func() {
		wg.Wait()
		close(arrivals)
	}()
	return stream, nil
}

func (r *Router) call(ctx context.Context, provider Provider, query string, maxResults int, now, slot time.Time) Arrival {
	name := provider.Name()
	if wait := time.Until(slot); wait > 0 {
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
	} else {
		for i := range hits {
			hits[i].Provider = name
		}
		arrival.Results = hits
	}
	if !callerGone {
		// At is when the search started: a failure from a call that was in
		// flight when another call benched the provider is the same episode
		// and adds no strike (sameEpisode).
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
