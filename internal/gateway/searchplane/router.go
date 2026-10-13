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
}

// NewRouter creates a search router.
func NewRouter(providers []Provider, store HealthStore, config Config) *Router {
	if store == nil {
		store = NewMemoryHealthStore()
	}
	if config.RequestTimeout <= 0 {
		config.RequestTimeout = 30 * time.Second
	}
	return &Router{providers: providers, store: store, config: config}
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
	// Cooling are the providers the search skipped because they were
	// cooling down or disabled. They are not failures of this search.
	Cooling []Cooling
	// Arrivals carries one Arrival per provider in Providers, in the order
	// the calls complete, and is closed after the last.
	Arrivals <-chan Arrival
}

// Cooling is a provider a search skipped.
type Cooling struct {
	Provider string
	State    ProviderState
	Until    time.Time
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

// Start calls every eligible provider at once and returns before any of
// them answers. Each call is bounded by the request timeout and by ctx; a
// call cut short by ctx is the caller going away, never a provider strike.
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
	var eligible []Provider
	for _, p := range r.providers {
		rec := health[p.Name()]
		if rec.Provider == "" {
			rec.Provider = p.Name()
			rec.State = StateActive
		}
		if IsEligible(rec, now) {
			eligible = append(eligible, p)
			stream.Providers = append(stream.Providers, p.Name())
			continue
		}
		skipped := Cooling{Provider: p.Name(), State: rec.State}
		if rec.CooldownUntil != nil {
			skipped.Until = *rec.CooldownUntil
		}
		stream.Cooling = append(stream.Cooling, skipped)
	}
	arrivals := make(chan Arrival, len(eligible))
	stream.Arrivals = arrivals
	var wg sync.WaitGroup
	for _, provider := range eligible {
		wg.Add(1)
		go func(provider Provider) {
			defer wg.Done()
			arrivals <- r.call(ctx, provider, query, maxResults, now)
		}(provider)
	}
	go func() {
		wg.Wait()
		close(arrivals)
	}()
	return stream, nil
}

func (r *Router) call(ctx context.Context, provider Provider, query string, maxResults int, now time.Time) Arrival {
	name := provider.Name()
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
