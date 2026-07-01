package searchplane

import (
	"context"
	"fmt"
	"sort"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"
)

// SearchResult is the router output before gateway type conversion.
type SearchResult struct {
	Results        []Result
	Providers      []string
	Primary        string
	Attempts       []Attempt
	ProviderHealth map[string]ProviderHealth
	MergedCount    int
	Waves          int
	Degraded       bool
	Query          string
}

// Router executes parallel provider waves with durable health.
type Router struct {
	providers []Provider
	store     HealthStore
	policy    BackoffPolicy
	config    Config
	counter   atomic.Int64
}

// NewRouter creates a search router.
func NewRouter(providers []Provider, store HealthStore, config Config) *Router {
	if store == nil {
		store = NewMemoryHealthStore()
	}
	if config.ProvidersPerQuery < 1 {
		config.ProvidersPerQuery = 1
	}
	if config.MaxWaves < 1 {
		config.MaxWaves = 1
	}
	if config.RequestTimeout <= 0 {
		config.RequestTimeout = 30 * time.Second
	}
	return &Router{
		providers: providers,
		store:     store,
		policy:    BackoffPolicyFromEnv(),
		config:    config,
	}
}

// HealthStore exposes the underlying store for ops endpoints.
func (r *Router) HealthStore() HealthStore { return r.store }

// Search executes the provider plane algorithm.
func (r *Router) Search(ctx context.Context, query string, maxResults int) (*SearchResult, error) {
	if len(r.providers) == 0 {
		return nil, fmt.Errorf("no search providers available")
	}
	now := time.Now()
	health, err := r.store.Snapshot()
	if err != nil {
		return nil, err
	}
	eligible := r.eligibleProviders(health, now)
	if len(eligible) == 0 {
		return nil, r.outage(query, health, nil)
	}

	targetMin := r.targetMinMerged(maxResults)
	start := int(r.counter.Add(1)-1) % len(eligible)
	attempts := r.appendCoolingSkipAttempts(nil, health, now)
	used := map[string]struct{}{}
	var batches []providerBatch
	var successProviders []string
	waves := 0
	degraded := false

	for wave := 0; wave < r.config.MaxWaves; wave++ {
		waves++
		pick := r.pickProviders(eligible, start, used, r.waveFanout(len(eligible), len(used)))
		if len(pick) == 0 {
			break
		}
		waveAttempts, waveBatches, waveProviders, waveDegraded := r.executeWave(ctx, query, maxResults, pick, now)
		attempts = append(attempts, waveAttempts...)
		successProviders = appendUniqueStrings(successProviders, waveProviders...)
		for _, batch := range waveBatches {
			for _, result := range batch.results {
				batches = appendBatchResult(batches, batch.provider, result)
			}
		}
		if waveDegraded {
			degraded = true
		}
		for _, name := range pick {
			used[name] = struct{}{}
		}
		merged := mergeBatches(batches, maxResults)
		if len(merged) >= targetMin {
			break
		}
		health, err = r.store.Snapshot()
		if err != nil {
			return nil, err
		}
		eligible = r.eligibleProviders(health, now)
		remaining := 0
		for _, name := range eligible {
			if _, ok := used[name]; !ok {
				remaining++
			}
		}
		if remaining == 0 {
			break
		}
	}

	merged := mergeBatches(batches, maxResults)
	health, _ = r.store.Snapshot()
	if len(merged) == 0 {
		return nil, r.outage(query, health, attempts)
	}
	primary := ""
	if len(successProviders) > 0 {
		primary = successProviders[0]
	} else if len(merged) > 0 {
		primary = merged[0].Provider
	}
	sort.Slice(attempts, func(i, j int) bool {
		if attempts[i].Provider == attempts[j].Provider {
			return attempts[i].Status < attempts[j].Status
		}
		return attempts[i].Provider < attempts[j].Provider
	})
	return &SearchResult{
		Results:        merged,
		Providers:      successProviders,
		Primary:        primary,
		Attempts:       attempts,
		ProviderHealth: health,
		MergedCount:    len(merged),
		Waves:          waves,
		Degraded:       degraded,
		Query:          query,
	}, nil
}

func (r *Router) targetMinMerged(maxResults int) int {
	min := r.config.MinMergedResults
	if min <= 0 {
		min = 1
	}
	if maxResults < min {
		return maxResults
	}
	return min
}

func (r *Router) waveFanout(eligibleCount, usedCount int) int {
	remaining := eligibleCount - usedCount
	if remaining < 1 {
		return 0
	}
	k := r.config.ProvidersPerQuery
	if k > remaining {
		k = remaining
	}
	return k
}

func (r *Router) eligibleProviders(health map[string]ProviderHealth, now time.Time) []string {
	names := make([]string, 0, len(r.providers))
	for _, p := range r.providers {
		rec := health[p.Name()]
		if rec.Provider == "" {
			rec.Provider = p.Name()
			rec.State = StateActive
		}
		if IsEligible(rec, now) {
			names = append(names, p.Name())
		}
	}
	return names
}

func (r *Router) pickProviders(eligible []string, start int, used map[string]struct{}, count int) []string {
	if count <= 0 || len(eligible) == 0 {
		return nil
	}
	picked := make([]string, 0, count)
	for i := 0; i < len(eligible) && len(picked) < count; i++ {
		name := eligible[(start+i)%len(eligible)]
		if _, ok := used[name]; ok {
			continue
		}
		picked = append(picked, name)
	}
	return picked
}

type providerWaveBatch struct {
	provider string
	results  []Result
}

func (r *Router) executeWave(ctx context.Context, query string, maxResults int, providers []string, now time.Time) ([]Attempt, []providerWaveBatch, []string, bool) {
	type callResult struct {
		provider string
		attempt  Attempt
		results  []Result
		class    OutcomeClass
	}
	resultsCh := make([]callResult, len(providers))
	g, gctx := errgroup.WithContext(ctx)
	for i, name := range providers {
		i, name := i, name
		provider := r.providerByName(name)
		if provider == nil {
			continue
		}
		g.Go(func() error {
			callCtx, cancel := context.WithTimeout(gctx, r.config.RequestTimeout)
			defer cancel()
			started := time.Now()
			hits, err := provider.Search(callCtx, query, maxResults)
			class := ClassifyCall(err, len(hits))
			attempt := Attempt{
				Provider:  name,
				Endpoint:  r.endpoint(name),
				Status:    AttemptStatus(class),
				LatencyMs: time.Since(started).Milliseconds(),
				Results:   len(hits),
			}
			if err != nil {
				attempt.Error = truncateSummary(err.Error())
			}
			for j := range hits {
				hits[j].Provider = name
			}
			_, _ = r.store.RecordOutcome(Outcome{
				Provider: name,
				Class:    class,
				Results:  len(hits),
				Error:    attempt.Error,
				At:       now,
			})
			resultsCh[i] = callResult{provider: name, attempt: attempt, results: hits, class: class}
			return nil
		})
	}
	_ = g.Wait()

	attempts := make([]Attempt, 0, len(providers))
	batches := make([]providerWaveBatch, 0, len(providers))
	var success []string
	degraded := false
	for _, res := range resultsCh {
		if res.provider == "" {
			continue
		}
		attempts = append(attempts, res.attempt)
		if res.class == OutcomeSuccess && len(res.results) > 0 {
			success = append(success, res.provider)
			batches = append(batches, providerWaveBatch{provider: res.provider, results: res.results})
			continue
		}
		if res.class != OutcomeSkippedCoolingDown {
			degraded = true
		}
	}
	return attempts, batches, success, degraded
}

func (r *Router) providerByName(name string) Provider {
	for _, p := range r.providers {
		if p.Name() == name {
			return p
		}
	}
	return nil
}

func (r *Router) endpoint(provider string) string {
	if r.config.EndpointFor != nil {
		return r.config.EndpointFor(provider)
	}
	return ""
}

func (r *Router) outage(query string, health map[string]ProviderHealth, attempts []Attempt) error {
	return &OutageError{Query: query, Health: health, Attempts: attempts}
}

func appendUniqueStrings(list []string, values ...string) []string {
	for _, value := range values {
		found := false
		for _, existing := range list {
			if existing == value {
				found = true
				break
			}
		}
		if !found {
			list = append(list, value)
		}
	}
	return list
}

func (r *Router) appendCoolingSkipAttempts(attempts []Attempt, health map[string]ProviderHealth, now time.Time) []Attempt {
	for _, provider := range r.providers {
		name := provider.Name()
		rec := health[name]
		if rec.Provider == "" {
			rec.Provider = name
		}
		rec = normalizeExpiredLocked(rec, now)
		if rec.State != StateCoolingDown {
			continue
		}
		if rec.CooldownUntil != nil && rec.CooldownUntil.After(now) {
			until := rec.CooldownUntil.UTC().Format(time.RFC3339)
			attempts = append(attempts, Attempt{
				Provider: name,
				Endpoint: r.endpoint(name),
				Status:   AttemptStatus(OutcomeSkippedCoolingDown),
				Error:    "provider cooling down until " + until,
			})
		}
	}
	return attempts
}
