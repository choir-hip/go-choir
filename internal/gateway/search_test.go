package gateway

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// --- Mock Search Provider for Testing ---

type mockSearchProvider struct {
	name        string
	available   bool
	searchFunc  func(ctx context.Context, query string, maxResults int) ([]SearchResult, error)
	searchCount int
}

func (m *mockSearchProvider) Name() string      { return m.name }
func (m *mockSearchProvider) IsAvailable() bool { return m.available }
func (m *mockSearchProvider) Search(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
	m.searchCount++
	if m.searchFunc != nil {
		return m.searchFunc(ctx, query, maxResults)
	}
	return nil, errors.New("mock: no search func")
}

func TestParseSearXNGResults(t *testing.T) {
	raw := []byte(`{
		"query": "test query",
		"number_of_results": 2,
		"results": [
			{
				"url": "https://example.com/one",
				"title": "Example One",
				"content": "First result content",
				"publishedDate": "2025-01-15T10:30:00",
				"engines": ["google", "bing"],
				"score": 8.5
			},
			{
				"url": "https://example.com/two",
				"title": "Example Two",
				"content": "Second result content",
				"publishedDate": null,
				"engines": ["duckduckgo"],
				"score": 3.2
			}
		]
	}`)

	results, err := parseSearXNGResults(raw, 10)
	if err != nil {
		t.Fatalf("parseSearXNGResults: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("results = %d, want 2", len(results))
	}
	if results[0].Title != "Example One" || results[0].URL != "https://example.com/one" {
		t.Fatalf("result[0] = %+v, want parsed title/url", results[0])
	}
	if results[0].Snippet != "First result content" {
		t.Fatalf("snippet = %q, want content", results[0].Snippet)
	}
	if results[0].Score != 8.5 {
		t.Fatalf("score = %v, want 8.5", results[0].Score)
	}
	if results[1].Title != "Example Two" {
		t.Fatalf("result[1] title = %q, want Example Two", results[1].Title)
	}
}

func TestParseSearXNGResults_EmptyURL(t *testing.T) {
	raw := []byte(`{
		"results": [
			{"url": "", "title": "No URL", "content": "skip"},
			{"url": "https://valid.com", "title": "Valid", "content": "keep"}
		]
	}`)

	results, err := parseSearXNGResults(raw, 10)
	if err != nil {
		t.Fatalf("parseSearXNGResults: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results = %d, want 1 (empty URL skipped)", len(results))
	}
	if results[0].URL != "https://valid.com" {
		t.Fatalf("url = %q, want https://valid.com", results[0].URL)
	}
}

func TestParseParallelResults(t *testing.T) {
	raw := []byte(`{
		"search_id": "search_test",
		"results": [
			{
				"url": "https://example.com/one",
				"title": "Example One",
				"publish_date": "2026-05-25",
				"excerpts": ["First excerpt.", "Second excerpt."]
			}
		],
		"session_id": "session_test"
	}`)

	results, err := parseParallelResults(raw)
	if err != nil {
		t.Fatalf("parseParallelResults: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results = %d, want 1", len(results))
	}
	if results[0].Title != "Example One" || results[0].URL != "https://example.com/one" {
		t.Fatalf("result = %+v, want parsed title/url", results[0])
	}
	if !strings.Contains(results[0].Snippet, "First excerpt.") || !strings.Contains(results[0].Snippet, "Second excerpt.") {
		t.Fatalf("snippet = %q, want joined excerpts", results[0].Snippet)
	}
	if results[0].PublishedAt != "2026-05-25" {
		t.Fatalf("published_at = %q, want date", results[0].PublishedAt)
	}
}

func TestSearXNGProvider_Search(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Errorf("path = %q, want /search", r.URL.Path)
		}
		q := r.URL.Query().Get("q")
		if q != "test query" {
			t.Errorf("q = %q, want 'test query'", q)
		}
		if r.URL.Query().Get("format") != "json" {
			t.Errorf("format = %q, want json", r.URL.Query().Get("format"))
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"query":"test query","results":[{"url":"https://result.com","title":"Result","content":"snippet","score":5.0}]}`)
	}))
	defer srv.Close()

	t.Setenv("SEARXNG_URL", srv.URL)
	p := &SearXNGProvider{}
	if !p.IsAvailable() {
		t.Fatal("expected available")
	}

	results, err := p.Search(context.Background(), "test query", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results = %d, want 1", len(results))
	}
	if results[0].URL != "https://result.com" || results[0].Title != "Result" {
		t.Fatalf("result = %+v", results[0])
	}
	if results[0].Snippet != "snippet" {
		t.Fatalf("snippet = %q, want 'snippet'", results[0].Snippet)
	}
}

func TestSearXNGProvider_SearchError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprint(w, `{"error": "upstream unavailable"}`)
	}))
	defer srv.Close()

	t.Setenv("SEARXNG_URL", srv.URL)
	p := &SearXNGProvider{}

	_, err := p.Search(context.Background(), "test", 10)
	if err == nil {
		t.Fatal("expected error on 503")
	}
}

func TestHandleSearch_MissingAuth(t *testing.T) {
	registry := NewIdentityRegistry(time.Hour)
	h := NewHandlerWithRateLimit(registry, nil, nil)

	body := `{"query": "test"}`
	req := httptest.NewRequest(http.MethodPost, "/provider/v1/search", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.HandleSearch(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestHandleSearch_InvalidBody(t *testing.T) {
	registry := NewIdentityRegistry(time.Hour)
	h := NewHandlerWithRateLimit(registry, nil, nil)

	// Issue a valid credential
	cred, err := registry.IssueCredential("test-autoputer")
	if err != nil {
		t.Fatalf("failed to issue credential: %v", err)
	}

	body := `invalid json`
	req := httptest.NewRequest(http.MethodPost, "/provider/v1/search", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+cred.RawToken)
	w := httptest.NewRecorder()

	h.HandleSearch(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleSearch_EmptyQuery(t *testing.T) {
	registry := NewIdentityRegistry(time.Hour)
	h := NewHandlerWithRateLimit(registry, nil, nil)

	// Issue a valid credential
	cred, err := registry.IssueCredential("test-autoputer")
	if err != nil {
		t.Fatalf("failed to issue credential: %v", err)
	}

	body := `{"query": ""}`
	req := httptest.NewRequest(http.MethodPost, "/provider/v1/search", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+cred.RawToken)
	w := httptest.NewRecorder()

	h.HandleSearch(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleSearch_NoProvidersConfigured(t *testing.T) {
	registry := NewIdentityRegistry(time.Hour)
	h := NewHandlerWithRateLimit(registry, nil, nil)
	h.searchClient = testSearchClient(nil)

	// Issue a valid credential
	cred, err := registry.IssueCredential("test-autoputer")
	if err != nil {
		t.Fatalf("failed to issue credential: %v", err)
	}

	body := `{"query": "test"}`
	req := httptest.NewRequest(http.MethodPost, "/provider/v1/search", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+cred.RawToken)
	w := httptest.NewRecorder()

	h.HandleSearch(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestHandleSearch_DeniesExternalPeerWithValidToken(t *testing.T) {
	registry := NewIdentityRegistry(time.Hour)

	mock := &mockSearchProvider{
		name:      "test",
		available: true,
		searchFunc: func(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
			return []SearchResult{{Title: "Result 1", URL: "http://example.com/1", Snippet: "Snippet 1"}}, nil
		},
	}

	h := &Handler{
		registry:     registry,
		searchClient: testSearchClient([]SearchProvider{mock}),
	}

	cred, err := registry.IssueCredential("test-autoputer")
	if err != nil {
		t.Fatalf("failed to issue credential: %v", err)
	}

	body := `{"query": "test", "max_results": 5}`
	req := httptest.NewRequest(http.MethodPost, "/provider/v1/search", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+cred.RawToken)
	req.RemoteAddr = "8.8.8.8:12345"
	w := httptest.NewRecorder()

	h.HandleSearch(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

// --- Provider Integration Tests (requires env vars, skipped by default) ---

func TestTavilyProvider_Integration(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") == "" {
		t.Skip("skipping live provider integration (set RUN_INTEGRATION_TESTS=1)")
	}
	apiKey := os.Getenv("TAVILY_API_KEY")
	if apiKey == "" {
		t.Skip("TAVILY_API_KEY not set, skipping integration test")
	}

	provider := &TavilyProvider{}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results, err := provider.Search(ctx, "golang programming", 3)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(results) == 0 {
		t.Error("expected at least one result")
	}

	for _, r := range results {
		if r.Title == "" {
			t.Error("expected non-empty title")
		}
		if r.URL == "" {
			t.Error("expected non-empty URL")
		}
	}
}

func TestBraveProvider_Integration(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") == "" {
		t.Skip("skipping live provider integration (set RUN_INTEGRATION_TESTS=1)")
	}
	apiKey := os.Getenv("BRAVE_API_KEY")
	if apiKey == "" {
		t.Skip("BRAVE_API_KEY not set, skipping integration test")
	}

	provider := &BraveProvider{}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results, err := provider.Search(ctx, "golang programming", 3)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(results) == 0 {
		t.Error("expected at least one result")
	}

	for _, r := range results {
		if r.Title == "" {
			t.Error("expected non-empty title")
		}
		if r.URL == "" {
			t.Error("expected non-empty URL")
		}
	}
}

func TestExaProvider_Integration(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") == "" {
		t.Skip("skipping live provider integration (set RUN_INTEGRATION_TESTS=1)")
	}
	apiKey := os.Getenv("EXA_API_KEY")
	if apiKey == "" {
		t.Skip("EXA_API_KEY not set, skipping integration test")
	}

	provider := &ExaProvider{}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results, err := provider.Search(ctx, "golang programming", 3)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(results) == 0 {
		t.Error("expected at least one result")
	}

	for _, r := range results {
		if r.Title == "" {
			t.Error("expected non-empty title")
		}
		if r.URL == "" {
			t.Error("expected non-empty URL")
		}
	}
}

func TestSerperProvider_Integration(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") == "" {
		t.Skip("skipping live provider integration (set RUN_INTEGRATION_TESTS=1)")
	}
	apiKey := os.Getenv("SERPER_API_KEY")
	if apiKey == "" {
		t.Skip("SERPER_API_KEY not set, skipping integration test")
	}

	provider := &SerperProvider{}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results, err := provider.Search(ctx, "golang programming", 3)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(results) == 0 {
		t.Error("expected at least one result")
	}

	for _, r := range results {
		if r.Title == "" {
			t.Error("expected non-empty title")
		}
		if r.URL == "" {
			t.Error("expected non-empty URL")
		}
	}
}

func TestParseSerpAPIResults(t *testing.T) {
	results, err := parseSerpAPIResults([]byte(`{
		"organic_results": [
			{"title": "AI infra", "link": "https://example.com/ai", "snippet": "June 2026 launch", "date": "Jun 16, 2026"}
		]
	}`))
	if err != nil {
		t.Fatalf("parseSerpAPIResults: %v", err)
	}
	if len(results) != 1 || results[0].URL != "https://example.com/ai" {
		t.Fatalf("results = %#v, want one parsed organic hit", results)
	}
	if results[0].PublishedAt != "Jun 16, 2026" {
		t.Fatalf("published_at = %q", results[0].PublishedAt)
	}

	_, err = parseSerpAPIResults([]byte(`{"error":"Invalid API key."}`))
	if err == nil {
		t.Fatal("expected error for API error envelope")
	}
}

func TestSerpAPIProvider_Integration(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") == "" {
		t.Skip("skipping live provider integration (set RUN_INTEGRATION_TESTS=1)")
	}
	apiKey := os.Getenv("SERPAPI_API_KEY")
	if apiKey == "" {
		t.Skip("SERPAPI_API_KEY not set, skipping integration test")
	}

	provider := &SerpAPIProvider{}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results, err := provider.Search(ctx, "golang programming", 3)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(results) == 0 {
		t.Error("expected at least one result")
	}
	for _, r := range results {
		if r.Title == "" {
			t.Error("expected non-empty title")
		}
		if r.URL == "" {
			t.Error("expected non-empty URL")
		}
	}
}

func TestParallelProvider_Integration(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") == "" {
		t.Skip("skipping live provider integration (set RUN_INTEGRATION_TESTS=1)")
	}
	apiKey := os.Getenv("PARALLEL_API_KEY")
	if apiKey == "" {
		t.Skip("PARALLEL_API_KEY not set, skipping integration test")
	}

	provider := &ParallelProvider{}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results, err := provider.Search(ctx, "golang programming", 3)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(results) == 0 {
		t.Error("expected at least one result")
	}
	for _, r := range results {
		if r.Title == "" {
			t.Error("expected non-empty title")
		}
		if r.URL == "" {
			t.Error("expected non-empty URL")
		}
	}
}
