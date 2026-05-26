package gateway

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/yusefmosiah/go-choir/internal/gateway/searchplane"
)

// ProviderHealthSummary is exposed in search responses and ops APIs.
type ProviderHealthSummary struct {
	State            searchplane.ProviderState `json:"state"`
	CooldownUntil    string                    `json:"cooldown_until,omitempty"`
	StrikeCount      int                       `json:"strike_count"`
	LastFailureClass string                    `json:"last_failure_class,omitempty"`
	LastErrorSummary string                    `json:"last_error_summary,omitempty"`
}

// SearchOutageResponse is returned when every provider is unavailable or empty.
type SearchOutageResponse struct {
	Error          string                           `json:"error"`
	Code           string                           `json:"code"`
	Query          string                           `json:"query,omitempty"`
	ProviderHealth map[string]ProviderHealthSummary `json:"provider_health,omitempty"`
	Attempts       []SearchProviderAttempt          `json:"attempts,omitempty"`
}

type searchProviderAdapter struct {
	inner SearchProvider
}

func (a searchProviderAdapter) Name() string { return a.inner.Name() }

func (a searchProviderAdapter) Search(ctx context.Context, query string, maxResults int) ([]searchplane.Result, error) {
	hits, err := a.inner.Search(ctx, query, maxResults)
	if err != nil {
		return nil, err
	}
	out := make([]searchplane.Result, len(hits))
	for i, hit := range hits {
		out[i] = searchplane.Result{
			Title:       hit.Title,
			URL:         hit.URL,
			Snippet:     hit.Snippet,
			PublishedAt: hit.PublishedAt,
			Score:       hit.Score,
			Provider:    hit.Provider,
		}
	}
	return out, nil
}

func (c *SearchClient) ensurePlane() error {
	if c.plane != nil {
		return nil
	}
	cfg := c.planeConfig
	if cfg.ProvidersPerQuery == 0 && c.providersPerQuery > 0 {
		cfg.ProvidersPerQuery = c.providersPerQuery
	}
	if cfg.ProvidersPerQuery == 0 {
		cfg = searchplane.ConfigFromEnv()
		if c.providersPerQuery > 0 {
			cfg.ProvidersPerQuery = c.providersPerQuery
		}
	}
	if cfg.MinMergedResults <= 0 {
		cfg.MinMergedResults = 1
	}
	if cfg.MaxWaves <= 0 {
		cfg.MaxWaves = 2
	}
	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = 30 * time.Second
	}
	cfg.EndpointFor = searchProviderEndpoint
	adapters := make([]searchplane.Provider, len(c.providers))
	for i, provider := range c.providers {
		adapters[i] = searchProviderAdapter{inner: provider}
	}
	store := c.healthStore
	if store == nil {
		var err error
		store, err = searchplane.OpenDefaultHealthStore()
		if err != nil {
			return err
		}
	}
	c.plane = searchplane.NewRouter(adapters, store, cfg)
	return nil
}

func convertPlaneResult(query string, result *searchplane.SearchResult) *SearchResponse {
	resp := &SearchResponse{
		Results:        make([]SearchResult, len(result.Results)),
		Provider:       result.Primary,
		Providers:      result.Providers,
		Attempts:       make([]SearchProviderAttempt, len(result.Attempts)),
		Query:          query,
		MergedCount:    result.MergedCount,
		Waves:          result.Waves,
		Degraded:       result.Degraded,
		ProviderHealth: map[string]ProviderHealthSummary{},
	}
	for i, hit := range result.Results {
		resp.Results[i] = SearchResult{
			Title:       hit.Title,
			URL:         hit.URL,
			Snippet:     hit.Snippet,
			PublishedAt: hit.PublishedAt,
			Score:       hit.Score,
			Provider:    hit.Provider,
		}
	}
	for i, attempt := range result.Attempts {
		resp.Attempts[i] = SearchProviderAttempt{
			Provider:  attempt.Provider,
			Endpoint:  attempt.Endpoint,
			Status:    attempt.Status,
			LatencyMs: attempt.LatencyMs,
			Results:   attempt.Results,
			Error:     attempt.Error,
		}
	}
	for name, health := range result.ProviderHealth {
		summary := ProviderHealthSummary{
			State:            health.State,
			StrikeCount:      health.StrikeCount,
			LastFailureClass: health.LastFailureClass,
			LastErrorSummary: health.LastErrorSummary,
		}
		if health.CooldownUntil != nil {
			summary.CooldownUntil = health.CooldownUntil.UTC().Format("2006-01-02T15:04:05Z07:00")
		}
		resp.ProviderHealth[name] = summary
	}
	return resp
}

func searchOutageResponse(err error) (*SearchOutageResponse, bool) {
	var outage *searchplane.OutageError
	if !errors.As(err, &outage) {
		return nil, false
	}
	resp := &SearchOutageResponse{
		Error: "search_outage",
		Code:  outage.Code(),
		Query: outage.Query,
		ProviderHealth: map[string]ProviderHealthSummary{},
		Attempts:       make([]SearchProviderAttempt, len(outage.Attempts)),
	}
	for i, attempt := range outage.Attempts {
		resp.Attempts[i] = SearchProviderAttempt{
			Provider:  attempt.Provider,
			Endpoint:  attempt.Endpoint,
			Status:    attempt.Status,
			LatencyMs: attempt.LatencyMs,
			Results:   attempt.Results,
			Error:     attempt.Error,
		}
	}
	for name, health := range outage.Health {
		summary := ProviderHealthSummary{
			State:            health.State,
			StrikeCount:      health.StrikeCount,
			LastFailureClass: health.LastFailureClass,
			LastErrorSummary: health.LastErrorSummary,
		}
		if health.CooldownUntil != nil {
			summary.CooldownUntil = health.CooldownUntil.UTC().Format("2006-01-02T15:04:05Z07:00")
		}
		resp.ProviderHealth[name] = summary
	}
	return resp, true
}

// HealthStore returns the durable provider health store when initialized.
func (c *SearchClient) HealthStore() (searchplane.HealthStore, error) {
	if err := c.ensurePlane(); err != nil {
		return nil, err
	}
	return c.plane.HealthStore(), nil
}

// ResetProviderHealth clears strikes and cooldown for one provider.
func (c *SearchClient) ResetProviderHealth(provider string) error {
	store, err := c.HealthStore()
	if err != nil {
		return err
	}
	_, err = store.ResetProvider(provider)
	return err
}

func (c *SearchClient) searchViaPlane(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
	if err := c.ensurePlane(); err != nil {
		return nil, fmt.Errorf("search plane: %w", err)
	}
	maxResults := req.MaxResults
	if maxResults <= 0 {
		maxResults = 10
	}
	if maxResults > 50 {
		maxResults = 50
	}
	result, err := c.plane.Search(ctx, req.Query, maxResults)
	if err != nil {
		return nil, err
	}
	return convertPlaneResult(req.Query, result), nil
}
