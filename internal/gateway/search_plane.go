package gateway

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/gateway/searchplane"
)

// searchStreamMargin is how long a search stream stays open past the
// provider request timeout, so every provider's own timeout lands on the
// stream before the stream's bound does.
const searchStreamMargin = 5 * time.Second

// ProviderHealthSummary is exposed by the search health ops API.
type ProviderHealthSummary struct {
	State            searchplane.ProviderState `json:"state"`
	CooldownUntil    string                    `json:"cooldown_until,omitempty"`
	StrikeCount      int                       `json:"strike_count"`
	LastFailureClass string                    `json:"last_failure_class,omitempty"`
	LastErrorSummary string                    `json:"last_error_summary,omitempty"`
}

// POST /provider/v1/search answers with NDJSON, one object per line: a
// SearchStreamStart, then one SearchStreamArrival per provider in the order
// the providers answer, then a SearchStreamDone.

// SearchStreamStart is a search stream's first line: the providers the
// search started (the N of "M of N answered") and the ones it skipped.
type SearchStreamStart struct {
	Type      string                  `json:"type"`
	Query     string                  `json:"query"`
	Providers []string                `json:"providers"`
	Cooling   []SearchCoolingProvider `json:"cooling"`
	// CachedAt is set when the answer is replayed from the gateway's result
	// cache: when it was first received. No provider was called. Additive.
	CachedAt string `json:"cached_at,omitempty"`
	// Routing is "budget" or "fanout"; empty for a cache hit. Additive.
	Routing string `json:"routing,omitempty"`
	// Reserve names the paid providers held for escalation: started only
	// if the free answers fall short. Not cooling, not in N. Additive.
	Reserve []string `json:"reserve,omitempty"`
}

// SearchEscalation is a paid provider that joined a search after its start
// line, and why (short, slow, failed, no_free, deep).
type SearchEscalation struct {
	Provider string `json:"provider"`
	Reason   string `json:"reason"`
}

// SearchCoolingProvider is a provider a search skipped because it was
// cooling down or disabled. It is not a failure of that search.
type SearchCoolingProvider struct {
	Provider string `json:"provider"`
	State    string `json:"state"`
	Until    string `json:"until,omitempty"`
	// Reason is the failure class that benched it (quota_limited,
	// rate_limited, ...) or "paced"; never an error summary.
	Reason string `json:"reason,omitempty"`
}

// SearchStreamArrival is one provider's completed call.
type SearchStreamArrival struct {
	Type      string         `json:"type"`
	Provider  string         `json:"provider"`
	Status    string         `json:"status"`
	LatencyMs int64          `json:"latency_ms"`
	Results   []SearchResult `json:"results"`
	Error     string         `json:"error,omitempty"`
}

// SearchStreamDone is a search stream's last line. Pending names the
// providers still out when the stream's bound ended it. Total counts every
// provider started, escalated ones included; Cooling every provider
// skipped, at the start or at escalation.
type SearchStreamDone struct {
	Type     string                  `json:"type"`
	Answered int                     `json:"answered"`
	Total    int                     `json:"total"`
	Pending  []string                `json:"pending,omitempty"`
	Cooling  []SearchCoolingProvider `json:"cooling"`
	// Escalated names every paid provider budget routing added, and why:
	// at the start (deep, no_free) or after it (short, slow, failed).
	// Additive.
	Escalated []SearchEscalation `json:"escalated,omitempty"`
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
	if cfg.RequestTimeout <= 0 {
		cfg = searchplane.ConfigFromEnv()
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
	paceWait := cfg.PaceWait
	if paceWait <= 0 {
		paceWait = searchplane.DefaultPaceWait
	}
	c.cache = searchplane.NewCache(searchplane.CacheConfig{
		TTL:      cfg.CacheTTL,
		FreshTTL: cfg.CacheFreshTTL,
		MaxBytes: cfg.CacheMaxBytes,
		// A shared search may start a paid provider up to the escalation
		// window after it began; that call needs its own timeout.
		Bound: c.plane.RequestTimeout() + paceWait + c.plane.EscalationWindow() + time.Second,
	})
	log.Printf("gateway: search routing=%s caps=[%s] monthly=[%s] rps=[%s]", c.plane.Routing(), searchplane.FormatDailyCaps(cfg.DailyCaps),
		searchplane.FormatMonthlyQuotas(cfg.MonthlyQuotas), searchplane.FormatProviderRPS(cfg.MinInterval))
	c.cacheShared = cfg.CacheShared
	c.seen = newSearchURLSeen(24*time.Hour, searchSeenPerComputer)
	return nil
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

// Stream starts the search on every eligible provider and returns at once;
// the stream's Arrivals carry each provider's answer as it lands.
func (c *SearchClient) Stream(ctx context.Context, req SearchRequest) (*searchplane.Stream, error) {
	return c.StreamFor(ctx, "", req)
}

// StreamFor is Stream for one computer: an identical search the computer
// made within the cache TTL is replayed, and one in flight is joined
// (stream.Served says which). With CacheShared the computer does not
// partition the cache.
func (c *SearchClient) StreamFor(ctx context.Context, computerID string, req SearchRequest) (*searchplane.Stream, error) {
	if req.Query == "" {
		return nil, fmt.Errorf("query is required")
	}
	if len(c.providers) == 0 {
		return nil, fmt.Errorf("no search providers available (set SEARXNG_URL, TAVILY_API_KEY, BRAVE_API_KEY, PARALLEL_API_KEY, EXA_API_KEY, SERPER_API_KEY, SERPAPI_API_KEY, KEENABLE_API_KEY, or CHATGPT_AUTH_PATH)")
	}
	if err := c.ensurePlane(); err != nil {
		return nil, fmt.Errorf("search plane: %w", err)
	}
	maxResults := req.MaxResults
	if maxResults <= 0 {
		// Agent retrieval, not human reading: each provider returns a broad
		// candidate set for grounding and reranking.
		maxResults = 40
	}
	if maxResults > 50 {
		maxResults = 50
	}
	scope := computerID
	if c.cacheShared {
		scope = ""
	}
	deep := strings.EqualFold(strings.TrimSpace(req.Depth), SearchDepthDeep)
	if deep {
		// A deep search is never answered by an ordinary one's results.
		scope += "\x00deep"
	}
	return c.cache.Start(ctx, scope, req.Query, maxResults, func(ctx context.Context) (*searchplane.Stream, error) {
		return c.plane.StartRequest(ctx, searchplane.Request{Query: req.Query, MaxResults: maxResults, Deep: deep})
	})
}

// DailyCounts returns today's (UTC) provider call counts with their caps.
func (c *SearchClient) DailyCounts() ([]searchplane.DailyCount, error) {
	if err := c.ensurePlane(); err != nil {
		return nil, err
	}
	return c.plane.DailyCounts()
}

// StreamBound is how long a search stream stays open: the provider request
// timeout, plus the escalation window under budget routing (a paid
// provider may join that late), plus a margin.
func (c *SearchClient) StreamBound() time.Duration {
	timeout := c.planeConfig.RequestTimeout
	window := searchplane.EscalationWindow(c.planeConfig)
	if c.plane != nil {
		timeout = c.plane.RequestTimeout()
		window = c.plane.EscalationWindow()
	}
	if timeout <= 0 {
		env := searchplane.ConfigFromEnv()
		timeout = env.RequestTimeout
		window = searchplane.EscalationWindow(env)
	}
	margin := c.streamMargin
	if margin <= 0 {
		margin = searchStreamMargin
	}
	return timeout + window + margin
}

func searchStreamStart(stream *searchplane.Stream) SearchStreamStart {
	start := SearchStreamStart{Type: "start", Query: stream.Query, Providers: append([]string{}, stream.Providers...), Cooling: searchCoolingProviders(stream.Cooling)}
	if stream.Served == searchplane.ServedHit && !stream.CachedAt.IsZero() {
		start.CachedAt = stream.CachedAt.UTC().Format(time.RFC3339)
	}
	if stream.Route != nil {
		route := stream.Route.Summary()
		start.Routing = route.Mode
		start.Reserve = route.Reserve
	}
	return start
}

// searchStarted is every provider a stream started: the route's record
// (escalated providers included) or, for a cache hit, its providers.
func searchStarted(stream *searchplane.Stream) []string {
	if stream.Route == nil {
		return append([]string{}, stream.Providers...)
	}
	started := stream.Route.Summary().Started
	// A joiner's stream names the start providers; the route has them all.
	seen := map[string]bool{}
	for _, name := range started {
		seen[name] = true
	}
	for _, name := range stream.Providers {
		if !seen[name] {
			started = append(started, name)
		}
	}
	return started
}

func searchEscalations(route searchplane.RouteSummary) []SearchEscalation {
	if len(route.Escalations) == 0 {
		return nil
	}
	out := make([]SearchEscalation, len(route.Escalations))
	for i, e := range route.Escalations {
		out[i] = SearchEscalation{Provider: e.Provider, Reason: e.Reason}
	}
	return out
}

// searchRouteSummary renders a search's routing for the gateway log line:
// routing mode, each escalation as reason(unique<min):provider, the paid
// calls this request caused (none for a cache hit or a join: the leader's
// line counts them), and today's count per provider touched
// ("serper=14/150", "keenable=812" without a cap).
func searchRouteSummary(stream *searchplane.Stream) string {
	if stream.Route == nil {
		return "routing=cache escalated=no paid_calls=0 today=[]"
	}
	route := stream.Route.Summary()
	escalated := "no"
	if len(route.Escalations) > 0 {
		parts := make([]string, len(route.Escalations))
		for i, e := range route.Escalations {
			parts[i] = fmt.Sprintf("%s(%d):%s", e.Reason, e.Unique, e.Provider)
		}
		escalated = strings.Join(parts, ",")
	}
	paid := route.PaidCalls
	if stream.Served == searchplane.ServedJoined {
		paid = 0
	}
	today := make([]string, 0, len(route.Today))
	for _, d := range route.Today {
		entry := fmt.Sprintf("%s=%d", d.Provider, d.Calls)
		if d.Cap >= 0 {
			entry += fmt.Sprintf("/%d", d.Cap)
		}
		today = append(today, entry)
	}
	return fmt.Sprintf("routing=%s escalated=%s paid_calls=%d today=[%s]", route.Mode, escalated, paid, strings.Join(today, " "))
}

func searchCoolingProviders(cooling []searchplane.Cooling) []SearchCoolingProvider {
	out := make([]SearchCoolingProvider, 0, len(cooling))
	for _, c := range cooling {
		entry := SearchCoolingProvider{Provider: c.Provider, State: string(c.State), Reason: c.Reason}
		if !c.Until.IsZero() {
			entry.Until = c.Until.UTC().Format(time.RFC3339)
		}
		out = append(out, entry)
	}
	return out
}

// searchSkippedSummary names each skipped provider with its reason for the
// gateway log ("tavily=quota_limited brave=paced"), so "7 cooling" says
// which are spent, which are throttled by us, and which are failing.
func searchSkippedSummary(cooling []searchplane.Cooling) string {
	parts := make([]string, 0, len(cooling))
	for _, c := range cooling {
		reason := c.Reason
		if reason == "" {
			reason = string(c.State)
		}
		parts = append(parts, c.Provider+"="+reason)
	}
	return strings.Join(parts, " ")
}

func searchStreamArrival(a searchplane.Arrival) SearchStreamArrival {
	line := SearchStreamArrival{
		Type:      "provider",
		Provider:  a.Provider,
		Status:    a.Status,
		LatencyMs: a.LatencyMs,
		Results:   make([]SearchResult, len(a.Results)),
		Error:     searchplane.RedactSummary(a.Error),
	}
	for i, hit := range a.Results {
		line.Results[i] = SearchResult{
			Title:       hit.Title,
			URL:         hit.URL,
			Snippet:     hit.Snippet,
			PublishedAt: hit.PublishedAt,
			Score:       hit.Score,
			Provider:    hit.Provider,
		}
	}
	return line
}
