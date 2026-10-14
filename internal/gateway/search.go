// Package gateway implements web search across SearXNG (self-hosted, free),
// Tavily, Brave, Parallel, Exa, Serper, SerpAPI, Keenable and ChatGPT/Codex
// hosted web search.
//
// A search starts the free providers, adds paid ones cheapest first only
// when the free answers fall short (budget routing; CHOIR_SEARCH_ROUTING=fanout
// starts every eligible provider at once), and streams each provider's
// results as that provider answers (search_plane.go, searchplane/router.go).
package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/gateway/searchplane"
)

// SearchResult represents a single search result item.
type SearchResult struct {
	// Title is the page title.
	Title string `json:"title"`

	// URL is the result URL.
	URL string `json:"url"`

	// Snippet is a text excerpt or description.
	Snippet string `json:"snippet"`

	// PublishedAt is the optional publication date (ISO 8601 format).
	PublishedAt string `json:"published_at,omitempty"`

	// Score is the optional relevance score.
	Score float64 `json:"score,omitempty"`

	// Provider identifies the search backend that returned this result.
	Provider string `json:"provider,omitempty"`
}

// SearchRequest is the incoming search request payload.
type SearchRequest struct {
	// Query is the search query string (required).
	Query string `json:"query"`

	// MaxResults is the maximum number of results each provider returns
	// (default 40, max 50).
	MaxResults int `json:"max_results,omitempty"`

	// Depth "deep" asks for more resources (an important story): under
	// budget routing the cheapest paid provider starts with the free ones
	// and the deep thresholds apply. Optional; the guest does not send it
	// yet.
	Depth string `json:"depth,omitempty"`
}

// SearchDepthDeep is SearchRequest.Depth for a deep search.
const SearchDepthDeep = "deep"

// SearchProvider is the interface for search API implementations.
type SearchProvider interface {
	// Name returns the provider identifier (e.g., "tavily", "brave").
	Name() string

	// Search executes a search query and returns normalized results.
	// Returns an error if the search fails or the API key is invalid.
	Search(ctx context.Context, query string, maxResults int) ([]SearchResult, error)

	// IsAvailable returns true if the provider has credentials configured.
	IsAvailable() bool
}

// SearchClient streams searches across the available providers.
type SearchClient struct {
	providers   []SearchProvider
	healthStore searchplane.HealthStore
	planeConfig searchplane.Config
	plane       *searchplane.Router
	// cache answers repeated searches and joins identical ones in flight;
	// nil when CacheTTL is 0 (the uncached path).
	cache       *searchplane.Cache
	cacheShared bool
	// seen counts, per computer, result URLs already returned to it, for
	// the per-search log line (measurement only).
	seen *searchURLSeen
	// streamMargin is how long a search stream stays open past the provider
	// request timeout; zero means searchStreamMargin.
	streamMargin time.Duration
}

// NewSearchClient creates a SearchClient with all available providers.
func NewSearchClient() *SearchClient {
	providers := []SearchProvider{
		&SearXNGProvider{},
		&TavilyProvider{},
		&BraveProvider{},
		&ParallelProvider{},
		&ExaProvider{},
		&SerperProvider{},
		&SerpAPIProvider{},
		&KeenableProvider{},
		NewChatGPTSearchProviderFromEnv(),
	}

	// Filter to only available providers.
	var available []SearchProvider
	for _, p := range providers {
		if p.IsAvailable() {
			available = append(available, p)
		}
	}

	return &SearchClient{
		providers: available,
	}
}

// AvailableProviders returns the names of configured search providers.
func (c *SearchClient) AvailableProviders() []string {
	names := make([]string, len(c.providers))
	for i, p := range c.providers {
		names[i] = p.Name()
	}
	return names
}

func normalizeSearchResultURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	parsed.Fragment = ""
	return strings.TrimRight(parsed.String(), "/")
}

func searchProviderEndpoint(provider string) string {
	switch strings.TrimSpace(provider) {
	case "searxng":
		return os.Getenv("SEARXNG_URL")
	case "tavily":
		return "https://api.tavily.com/search"
	case "brave":
		return "https://api.search.brave.com/res/v1/web/search"
	case "parallel":
		return "https://api.parallel.ai/v1/search"
	case "exa":
		return "https://api.exa.ai/search"
	case "serper":
		return "https://google.serper.dev/search"
	case "serpapi":
		return "https://serpapi.com/search.json"
	case "keenable":
		return keenableSearchURL
	case "chatgpt":
		if endpoint := strings.TrimSpace(os.Getenv("CHATGPT_SEARCH_BASE_URL")); endpoint != "" {
			return endpoint
		}
		if endpoint := strings.TrimSpace(os.Getenv("CHATGPT_BASE_URL")); endpoint != "" {
			return endpoint
		}
		return defaultChatGPTSearchURL
	default:
		return ""
	}
}

// --- SearXNG Provider ---

// SearXNGProvider implements search using a self-hosted SearXNG instance.
// SearXNG is a free meta-search engine that aggregates results from Google,
// Bing, DuckDuckGo, and 70+ other engines. It requires no API key — only a
// SEARXNG_URL env var pointing to the instance (e.g. http://localhost:8888).
// The instance must have JSON format enabled in settings.yml.
type SearXNGProvider struct {
	httpClient *http.Client
}

func (p *SearXNGProvider) Name() string { return "searxng" }

func (p *SearXNGProvider) IsAvailable() bool {
	return strings.TrimSpace(os.Getenv("SEARXNG_URL")) != ""
}

func (p *SearXNGProvider) http() *http.Client {
	if p.httpClient != nil {
		return p.httpClient
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func (p *SearXNGProvider) Search(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("SEARXNG_URL")), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("SEARXNG_URL not set")
	}
	if maxResults <= 0 {
		maxResults = 10
	}

	u := fmt.Sprintf("%s/search?q=%s&format=json&pageno=1", baseURL, url.QueryEscape(query))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := p.http().Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("status %s: %s", resp.Status, truncateError(bodyBytes))
	}

	return parseSearXNGResults(bodyBytes, maxResults)
}

func parseSearXNGResults(data []byte, maxResults int) ([]SearchResult, error) {
	var result struct {
		Results []struct {
			URL           string   `json:"url"`
			Title         string   `json:"title"`
			Content       string   `json:"content"`
			PublishedDate string   `json:"publishedDate"`
			Engines       []string `json:"engines"`
			Score         float64  `json:"score"`
		} `json:"results"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	results := make([]SearchResult, 0, len(result.Results))
	for _, r := range result.Results {
		if r.URL == "" {
			continue
		}
		results = append(results, SearchResult{
			Title:       r.Title,
			URL:         r.URL,
			Snippet:     r.Content,
			PublishedAt: r.PublishedDate,
			Score:       r.Score,
		})
		if len(results) >= maxResults {
			break
		}
	}
	return results, nil
}

// --- Keenable Provider ---

const keenableSearchURL = "https://api.keenable.ai/v1/search"

// KeenableProvider implements search using Keenable's Search API
// (https://docs.keenable.ai/api-reference/search). "realtime" mode keeps it
// on the fast path beside the other providers.
type KeenableProvider struct {
	httpClient *http.Client
}

func (p *KeenableProvider) Name() string { return "keenable" }

func (p *KeenableProvider) IsAvailable() bool {
	return os.Getenv("KEENABLE_API_KEY") != ""
}

func (p *KeenableProvider) http() *http.Client {
	if p.httpClient != nil {
		return p.httpClient
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func (p *KeenableProvider) Search(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
	apiKey := os.Getenv("KEENABLE_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("KEENABLE_API_KEY not set")
	}
	if maxResults <= 0 {
		maxResults = 10
	}
	if maxResults > 50 {
		maxResults = 50
	}
	bodyBytes, err := json.Marshal(map[string]any{"query": query, "max_results": maxResults, "mode": "realtime"})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, keenableSearchURL, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", apiKey)

	resp, err := p.http().Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	bodyBytes, err = io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("status %s: %s", resp.Status, truncateError(bodyBytes))
	}
	var result struct {
		Results []struct {
			Title       string `json:"title"`
			URL         string `json:"url"`
			Description string `json:"description"`
			Snippet     string `json:"snippet"`
			PublishedAt string `json:"published_at"`
		} `json:"results"`
	}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	results := make([]SearchResult, 0, len(result.Results))
	for _, r := range result.Results {
		if r.URL == "" {
			continue
		}
		snippet := r.Snippet
		if snippet == "" {
			snippet = r.Description
		}
		results = append(results, SearchResult{Title: r.Title, URL: r.URL, Snippet: snippet, PublishedAt: r.PublishedAt})
	}
	return results, nil
}

// --- Parallel Provider ---

// ParallelProvider implements search using Parallel's Search API. It uses only
// the Search endpoint; generic paid extraction remains outside the ordinary
// fast path.
type ParallelProvider struct {
	httpClient *http.Client
}

func (p *ParallelProvider) Name() string { return "parallel" }

func (p *ParallelProvider) IsAvailable() bool {
	return os.Getenv("PARALLEL_API_KEY") != ""
}

func (p *ParallelProvider) http() *http.Client {
	if p.httpClient != nil {
		return p.httpClient
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func (p *ParallelProvider) Search(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
	apiKey := os.Getenv("PARALLEL_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("PARALLEL_API_KEY not set")
	}
	if maxResults <= 0 {
		maxResults = 10
	}
	body := map[string]any{
		"objective":       query,
		"search_queries":  []string{query},
		"mode":            "basic",
		"max_chars_total": maxResults * 900,
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.parallel.ai/v1/search", strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)

	resp, err := p.http().Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	bodyBytes, err = io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("status %s: %s", resp.Status, truncateError(bodyBytes))
	}
	return parseParallelResults(bodyBytes)
}

func parseParallelResults(data []byte) ([]SearchResult, error) {
	var result struct {
		Results []struct {
			Title       string   `json:"title"`
			URL         string   `json:"url"`
			PublishDate string   `json:"publish_date"`
			Excerpts    []string `json:"excerpts"`
		} `json:"results"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	results := make([]SearchResult, 0, len(result.Results))
	for _, r := range result.Results {
		if r.URL == "" {
			continue
		}
		results = append(results, SearchResult{
			Title:       r.Title,
			URL:         r.URL,
			Snippet:     strings.Join(r.Excerpts, "\n\n"),
			PublishedAt: r.PublishDate,
		})
	}
	return results, nil
}

// --- Tavily Provider ---

// TavilyProvider implements search using the Tavily API.
type TavilyProvider struct {
	httpClient *http.Client
}

func (p *TavilyProvider) Name() string { return "tavily" }

func (p *TavilyProvider) IsAvailable() bool {
	return os.Getenv("TAVILY_API_KEY") != ""
}

func (p *TavilyProvider) http() *http.Client {
	if p.httpClient != nil {
		return p.httpClient
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func (p *TavilyProvider) Search(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
	apiKey := os.Getenv("TAVILY_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("TAVILY_API_KEY not set")
	}

	body := map[string]any{
		"query":               query,
		"max_results":         maxResults,
		"search_depth":        "basic",
		"include_answer":      false,
		"include_raw_content": false,
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.tavily.com/search", strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := p.http().Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	bodyBytes, err = io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("status %s: %s", resp.Status, truncateError(bodyBytes))
	}

	return parseTavilyResults(bodyBytes)
}

func parseTavilyResults(data []byte) ([]SearchResult, error) {
	var result struct {
		Results []struct {
			Title         string  `json:"title"`
			URL           string  `json:"url"`
			Content       string  `json:"content"`
			PublishedDate string  `json:"published_date"`
			Score         float64 `json:"score"`
		} `json:"results"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	results := make([]SearchResult, 0, len(result.Results))
	for _, r := range result.Results {
		if r.URL == "" {
			continue
		}
		results = append(results, SearchResult{
			Title:       r.Title,
			URL:         r.URL,
			Snippet:     r.Content,
			PublishedAt: r.PublishedDate,
			Score:       r.Score,
		})
	}

	return results, nil
}

// --- Brave Provider ---

type BraveProvider struct {
	httpClient *http.Client
}

func (p *BraveProvider) Name() string { return "brave" }

func (p *BraveProvider) IsAvailable() bool {
	return os.Getenv("BRAVE_API_KEY") != ""
}

func (p *BraveProvider) http() *http.Client {
	if p.httpClient != nil {
		return p.httpClient
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func (p *BraveProvider) Search(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
	apiKey := os.Getenv("BRAVE_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("BRAVE_API_KEY not set")
	}

	u := braveSearchURL(query, maxResults)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Subscription-Token", apiKey)

	resp, err := p.http().Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("status %s: %s", resp.Status, truncateError(bodyBytes))
	}

	return parseBraveResults(bodyBytes)
}

// braveMaxCount is Brave's per-request limit; a larger count is a 422 that
// strikes Brave for a week (search-plane-cooldown-on-caller-cancel-2026-10-09).
const braveMaxCount = 20

func braveSearchURL(query string, maxResults int) string {
	if maxResults <= 0 || maxResults > braveMaxCount {
		maxResults = braveMaxCount
	}
	return fmt.Sprintf("https://api.search.brave.com/res/v1/web/search?q=%s&count=%d", url.QueryEscape(query), maxResults)
}

func parseBraveResults(data []byte) ([]SearchResult, error) {
	var result struct {
		Web struct {
			Results []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
				Age         string `json:"age"`
			} `json:"results"`
		} `json:"web"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	results := make([]SearchResult, 0, len(result.Web.Results))
	for _, r := range result.Web.Results {
		if r.URL == "" {
			continue
		}
		results = append(results, SearchResult{
			Title:       r.Title,
			URL:         r.URL,
			Snippet:     r.Description,
			PublishedAt: r.Age,
		})
	}

	return results, nil
}

// --- Exa Provider ---

type ExaProvider struct {
	httpClient *http.Client
}

func (p *ExaProvider) Name() string { return "exa" }

func (p *ExaProvider) IsAvailable() bool {
	return os.Getenv("EXA_API_KEY") != ""
}

func (p *ExaProvider) http() *http.Client {
	if p.httpClient != nil {
		return p.httpClient
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func (p *ExaProvider) Search(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
	apiKey := os.Getenv("EXA_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("EXA_API_KEY not set")
	}

	body := map[string]any{
		"query":      query,
		"numResults": maxResults,
		"type":       "auto",
		"contents": map[string]any{
			"text": true,
		},
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.exa.ai/search", strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)

	resp, err := p.http().Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	bodyBytes, err = io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("status %s: %s", resp.Status, truncateError(bodyBytes))
	}

	return parseExaResults(bodyBytes)
}

func parseExaResults(data []byte) ([]SearchResult, error) {
	var result struct {
		Results []struct {
			Title         string   `json:"title"`
			URL           string   `json:"url"`
			Text          string   `json:"text"`
			PublishedDate string   `json:"publishedDate"`
			Score         float64  `json:"score"`
			Highlights    []string `json:"highlights"`
		} `json:"results"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	results := make([]SearchResult, 0, len(result.Results))
	for _, r := range result.Results {
		if r.URL == "" {
			continue
		}

		// Use highlights if available, otherwise text.
		snippet := r.Text
		if len(r.Highlights) > 0 {
			snippet = strings.Join(r.Highlights, " ")
		}

		results = append(results, SearchResult{
			Title:       r.Title,
			URL:         r.URL,
			Snippet:     snippet,
			PublishedAt: r.PublishedDate,
			Score:       r.Score,
		})
	}

	return results, nil
}

// --- Serper Provider ---

type SerperProvider struct {
	httpClient *http.Client
}

func (p *SerperProvider) Name() string { return "serper" }

func (p *SerperProvider) IsAvailable() bool {
	return os.Getenv("SERPER_API_KEY") != ""
}

func (p *SerperProvider) http() *http.Client {
	if p.httpClient != nil {
		return p.httpClient
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func (p *SerperProvider) Search(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
	apiKey := os.Getenv("SERPER_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("SERPER_API_KEY not set")
	}

	body := map[string]any{
		"q":   query,
		"num": maxResults,
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://google.serper.dev/search", strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-KEY", apiKey)

	resp, err := p.http().Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	bodyBytes, err = io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("status %s: %s", resp.Status, truncateError(bodyBytes))
	}

	return parseSerperResults(bodyBytes)
}

func parseSerperResults(data []byte) ([]SearchResult, error) {
	var result struct {
		Organic []struct {
			Title   string `json:"title"`
			Link    string `json:"link"`
			Snippet string `json:"snippet"`
			Date    string `json:"date"`
		} `json:"organic"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	results := make([]SearchResult, 0, len(result.Organic))
	for _, r := range result.Organic {
		if r.Link == "" {
			continue
		}
		results = append(results, SearchResult{
			Title:       r.Title,
			URL:         r.Link,
			Snippet:     r.Snippet,
			PublishedAt: r.Date,
		})
	}

	return results, nil
}

// --- SerpAPI Provider ---

type SerpAPIProvider struct {
	httpClient *http.Client
}

func (p *SerpAPIProvider) Name() string { return "serpapi" }

func (p *SerpAPIProvider) IsAvailable() bool {
	return os.Getenv("SERPAPI_API_KEY") != ""
}

func (p *SerpAPIProvider) http() *http.Client {
	if p.httpClient != nil {
		return p.httpClient
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func (p *SerpAPIProvider) Search(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
	apiKey := os.Getenv("SERPAPI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("SERPAPI_API_KEY not set")
	}
	if maxResults <= 0 {
		maxResults = 10
	}

	u, err := url.Parse("https://serpapi.com/search.json")
	if err != nil {
		return nil, fmt.Errorf("parse endpoint: %w", err)
	}
	q := u.Query()
	q.Set("engine", "google")
	q.Set("q", query)
	q.Set("num", strconv.Itoa(maxResults))
	q.Set("api_key", apiKey)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := p.http().Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("status %s: %s", resp.Status, truncateError(bodyBytes))
	}

	return parseSerpAPIResults(bodyBytes)
}

func parseSerpAPIResults(data []byte) ([]SearchResult, error) {
	var envelope struct {
		Error   string `json:"error"`
		Organic []struct {
			Title   string `json:"title"`
			Link    string `json:"link"`
			Snippet string `json:"snippet"`
			Date    string `json:"date"`
		} `json:"organic_results"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	if strings.TrimSpace(envelope.Error) != "" {
		return nil, fmt.Errorf("status error: %s", envelope.Error)
	}

	results := make([]SearchResult, 0, len(envelope.Organic))
	for _, r := range envelope.Organic {
		if r.Link == "" {
			continue
		}
		results = append(results, SearchResult{
			Title:       r.Title,
			URL:         r.Link,
			Snippet:     r.Snippet,
			PublishedAt: r.Date,
		})
	}
	return results, nil
}

// --- Helpers ---

// truncateError limits error message length to avoid leaking large responses.
func truncateError(data []byte) string {
	s := string(data)
	if len(s) > 200 {
		return s[:200] + "..."
	}
	return s
}
