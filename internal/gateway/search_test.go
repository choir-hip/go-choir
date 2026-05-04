package gateway

import (
	"context"
	"encoding/json"
	"errors"
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

// --- SearchClient Tests ---

func TestSearchClient_NoProviders(t *testing.T) {
	client := &SearchClient{providers: []SearchProvider{}}
	req := SearchRequest{Query: "test", MaxResults: 5}

	_, err := client.Search(context.Background(), req)
	if err == nil {
		t.Fatal("expected error with no providers")
	}
	if !strings.Contains(err.Error(), "no search providers available") {
		t.Errorf("expected 'no search providers available' error, got: %v", err)
	}
}

func TestSearchClient_EmptyQuery(t *testing.T) {
	mock := &mockSearchProvider{
		name:      "mock",
		available: true,
		searchFunc: func(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
			return []SearchResult{{Title: "Test", URL: "http://example.com", Snippet: "test"}}, nil
		},
	}
	client := &SearchClient{providers: []SearchProvider{mock}}

	req := SearchRequest{Query: "", MaxResults: 5}
	_, err := client.Search(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for empty query")
	}
	if !strings.Contains(err.Error(), "query is required") {
		t.Errorf("expected 'query is required' error, got: %v", err)
	}
}

func TestSearchClient_Rotation(t *testing.T) {
	mock1 := &mockSearchProvider{
		name:      "mock1",
		available: true,
		searchFunc: func(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
			return []SearchResult{{Title: "Result1", URL: "http://example.com/1", Snippet: "result1"}}, nil
		},
	}
	mock2 := &mockSearchProvider{
		name:      "mock2",
		available: true,
		searchFunc: func(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
			return []SearchResult{{Title: "Result2", URL: "http://example.com/2", Snippet: "result2"}}, nil
		},
	}

	client := &SearchClient{providers: []SearchProvider{mock1, mock2}}

	// First request should go to mock1 (counter starts at 0, so start=0)
	resp1, err := client.Search(context.Background(), SearchRequest{Query: "test", MaxResults: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp1.Provider != "mock1" {
		t.Errorf("first request: expected provider mock1, got %s", resp1.Provider)
	}

	// Second request should go to mock2 (counter now 1, so start=1)
	resp2, err := client.Search(context.Background(), SearchRequest{Query: "test", MaxResults: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp2.Provider != "mock2" {
		t.Errorf("second request: expected provider mock2, got %s", resp2.Provider)
	}

	// Third request should wrap around to mock1 (counter now 2, so start=0)
	resp3, err := client.Search(context.Background(), SearchRequest{Query: "test", MaxResults: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp3.Provider != "mock1" {
		t.Errorf("third request: expected provider mock1, got %s", resp3.Provider)
	}
}

func TestSearchClient_DiversifiesAcrossProviders(t *testing.T) {
	mock1 := &mockSearchProvider{
		name:      "mock1",
		available: true,
		searchFunc: func(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
			return []SearchResult{{Title: "Result1", URL: "http://example.com/1", Snippet: "result1"}}, nil
		},
	}
	mock2 := &mockSearchProvider{
		name:      "mock2",
		available: true,
		searchFunc: func(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
			return []SearchResult{{Title: "Result2", URL: "http://example.com/2", Snippet: "result2"}}, nil
		},
	}
	mock3 := &mockSearchProvider{
		name:      "mock3",
		available: true,
		searchFunc: func(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
			return []SearchResult{{Title: "Result3", URL: "http://example.com/3", Snippet: "result3"}}, nil
		},
	}

	client := &SearchClient{providers: []SearchProvider{mock1, mock2, mock3}, providersPerQuery: 2}

	resp1, err := client.Search(context.Background(), SearchRequest{Query: "test", MaxResults: 10})
	if err != nil {
		t.Fatalf("first search: %v", err)
	}
	if got, want := strings.Join(resp1.Providers, ","), "mock1,mock2"; got != want {
		t.Fatalf("first providers = %q, want %q", got, want)
	}
	if len(resp1.Results) != 2 {
		t.Fatalf("first results = %d, want 2", len(resp1.Results))
	}
	if len(resp1.Attempts) != 2 {
		t.Fatalf("first attempts = %d, want 2", len(resp1.Attempts))
	}
	if resp1.Results[0].Provider != "mock1" || resp1.Results[1].Provider != "mock2" {
		t.Fatalf("result providers = %#v, want mock1/mock2", resp1.Results)
	}

	resp2, err := client.Search(context.Background(), SearchRequest{Query: "test", MaxResults: 10})
	if err != nil {
		t.Fatalf("second search: %v", err)
	}
	if got, want := strings.Join(resp2.Providers, ","), "mock2,mock3"; got != want {
		t.Fatalf("second providers = %q, want %q", got, want)
	}
}

func TestSearchClient_MergesMaxResultsAcrossProviderFanout(t *testing.T) {
	mock1 := &mockSearchProvider{
		name:      "mock1",
		available: true,
		searchFunc: func(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
			return []SearchResult{
				{Title: "Result 1A", URL: "http://example.com/1a", Snippet: "result1a"},
				{Title: "Result 1B", URL: "http://example.com/1b", Snippet: "result1b"},
				{Title: "Result 1C", URL: "http://example.com/1c", Snippet: "result1c"},
				{Title: "Result 1D", URL: "http://example.com/1d", Snippet: "result1d"},
			}, nil
		},
	}
	mock2 := &mockSearchProvider{
		name:      "mock2",
		available: true,
		searchFunc: func(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
			return []SearchResult{
				{Title: "Result 2A", URL: "http://example.com/2a", Snippet: "result2a"},
				{Title: "Result 2B", URL: "http://example.com/2b", Snippet: "result2b"},
				{Title: "Result 2C", URL: "http://example.com/2c", Snippet: "result2c"},
				{Title: "Result 2D", URL: "http://example.com/2d", Snippet: "result2d"},
			}, nil
		},
	}

	client := &SearchClient{providers: []SearchProvider{mock1, mock2}, providersPerQuery: 2}
	resp, err := client.Search(context.Background(), SearchRequest{Query: "test", MaxResults: 4})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if got, want := strings.Join(resp.Providers, ","), "mock1,mock2"; got != want {
		t.Fatalf("providers = %q, want %q", got, want)
	}
	if len(resp.Results) != 4 {
		t.Fatalf("results = %d, want 4", len(resp.Results))
	}
	gotProviders := []string{
		resp.Results[0].Provider,
		resp.Results[1].Provider,
		resp.Results[2].Provider,
		resp.Results[3].Provider,
	}
	if strings.Join(gotProviders, ",") != "mock1,mock2,mock1,mock2" {
		t.Fatalf("result providers = %v, want interleaved mock1/mock2", gotProviders)
	}
}

func TestSearchClient_Fallback(t *testing.T) {
	failProvider := &mockSearchProvider{
		name:      "fail",
		available: true,
		searchFunc: func(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
			return nil, errors.New("provider failed")
		},
	}
	successProvider := &mockSearchProvider{
		name:      "success",
		available: true,
		searchFunc: func(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
			return []SearchResult{{Title: "Success", URL: "http://example.com", Snippet: "success"}}, nil
		},
	}

	client := &SearchClient{providers: []SearchProvider{failProvider, successProvider}}

	resp, err := client.Search(context.Background(), SearchRequest{Query: "test", MaxResults: 5})
	if err != nil {
		t.Fatalf("expected success through fallback, got error: %v", err)
	}
	if resp.Provider != "success" {
		t.Errorf("expected provider 'success' after fallback, got %s", resp.Provider)
	}
	if len(resp.Attempts) != 2 {
		t.Fatalf("attempts = %d, want failed provider plus success provider", len(resp.Attempts))
	}
	if resp.Attempts[0].Provider != "fail" || resp.Attempts[0].Status != "error" {
		t.Fatalf("first attempt = %+v, want fail/error", resp.Attempts[0])
	}
	if resp.Attempts[1].Provider != "success" || resp.Attempts[1].Status != "success" {
		t.Fatalf("second attempt = %+v, want success/success", resp.Attempts[1])
	}
	if len(resp.Results) != 1 {
		t.Errorf("expected 1 result, got %d", len(resp.Results))
	}
}

func TestSearchClient_AllProvidersFail(t *testing.T) {
	fail1 := &mockSearchProvider{
		name:      "fail1",
		available: true,
		searchFunc: func(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
			return nil, errors.New("fail1 error")
		},
	}
	fail2 := &mockSearchProvider{
		name:      "fail2",
		available: true,
		searchFunc: func(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
			return nil, errors.New("fail2 error")
		},
	}

	client := &SearchClient{providers: []SearchProvider{fail1, fail2}}

	_, err := client.Search(context.Background(), SearchRequest{Query: "test", MaxResults: 5})
	if err == nil {
		t.Fatal("expected error when all providers fail")
	}
	if !strings.Contains(err.Error(), "all search providers failed") {
		t.Errorf("expected 'all search providers failed' error, got: %v", err)
	}
}

func TestSearchClient_MaxResultsClamping(t *testing.T) {
	mock := &mockSearchProvider{
		name:      "mock",
		available: true,
		searchFunc: func(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
			// Verify the maxResults is clamped
			if maxResults > 50 || maxResults < 1 {
				t.Errorf("maxResults should be clamped to [1,50], got %d", maxResults)
			}
			return []SearchResult{}, nil
		},
	}

	client := &SearchClient{providers: []SearchProvider{mock}}

	// Test zero (should default to 10)
	client.Search(context.Background(), SearchRequest{Query: "test", MaxResults: 0})

	// Test too large (should clamp to 50)
	client.Search(context.Background(), SearchRequest{Query: "test", MaxResults: 100})
}

func TestSearchClient_AvailableProviders(t *testing.T) {
	mock1 := &mockSearchProvider{name: "mock1", available: true}
	mock2 := &mockSearchProvider{name: "mock2", available: true}

	client := &SearchClient{providers: []SearchProvider{mock1, mock2}}

	names := client.AvailableProviders()
	if len(names) != 2 {
		t.Errorf("expected 2 providers, got %d", len(names))
	}
	if names[0] != "mock1" || names[1] != "mock2" {
		t.Errorf("expected [mock1, mock2], got %v", names)
	}
}

// --- Handler Tests ---

func TestHandleSearch_MethodNotAllowed(t *testing.T) {
	h := &Handler{searchClient: &SearchClient{}}
	req := httptest.NewRequest(http.MethodGet, "/provider/v1/search", nil)
	w := httptest.NewRecorder()

	h.HandleSearch(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestHandleSearch_MissingAuth(t *testing.T) {
	registry := NewIdentityRegistry(time.Hour)
	h := NewHandler(registry, nil)

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
	h := NewHandler(registry, nil)

	// Issue a valid credential
	cred, err := registry.IssueCredential("test-sandbox")
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
	h := NewHandler(registry, nil)

	// Issue a valid credential
	cred, err := registry.IssueCredential("test-sandbox")
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
	h := NewHandler(registry, nil)

	// Issue a valid credential
	cred, err := registry.IssueCredential("test-sandbox")
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

func TestHandleSearch_Success(t *testing.T) {
	registry := NewIdentityRegistry(time.Hour)

	// Create a search client with a mock provider
	mock := &mockSearchProvider{
		name:      "test",
		available: true,
		searchFunc: func(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
			return []SearchResult{
				{Title: "Result 1", URL: "http://example.com/1", Snippet: "Snippet 1"},
				{Title: "Result 2", URL: "http://example.com/2", Snippet: "Snippet 2"},
			}, nil
		},
	}

	h := &Handler{
		registry:     registry,
		searchClient: &SearchClient{providers: []SearchProvider{mock}},
	}

	// Issue a valid credential
	cred, err := registry.IssueCredential("test-sandbox")
	if err != nil {
		t.Fatalf("failed to issue credential: %v", err)
	}

	body := `{"query": "test", "max_results": 5}`
	req := httptest.NewRequest(http.MethodPost, "/provider/v1/search", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+cred.RawToken)
	w := httptest.NewRecorder()

	h.HandleSearch(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp SearchResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Provider != "test" {
		t.Errorf("expected provider 'test', got %s", resp.Provider)
	}
	if resp.Query != "test" {
		t.Errorf("expected query 'test', got %s", resp.Query)
	}
	if len(resp.Results) != 2 {
		t.Errorf("expected 2 results, got %d", len(resp.Results))
	}
	if resp.Results[0].Title != "Result 1" {
		t.Errorf("expected first result title 'Result 1', got %s", resp.Results[0].Title)
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
		searchClient: &SearchClient{providers: []SearchProvider{mock}},
	}

	cred, err := registry.IssueCredential("test-sandbox")
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

func TestNewSearchClient_FromEnv(t *testing.T) {
	// Save current env vars
	tavilyKey := os.Getenv("TAVILY_API_KEY")
	braveKey := os.Getenv("BRAVE_API_KEY")
	exaKey := os.Getenv("EXA_API_KEY")
	serperKey := os.Getenv("SERPER_API_KEY")

	// Clean up after test
	defer func() {
		os.Setenv("TAVILY_API_KEY", tavilyKey)
		os.Setenv("BRAVE_API_KEY", braveKey)
		os.Setenv("EXA_API_KEY", exaKey)
		os.Setenv("SERPER_API_KEY", serperKey)
	}()

	// Test with no keys set
	os.Unsetenv("TAVILY_API_KEY")
	os.Unsetenv("BRAVE_API_KEY")
	os.Unsetenv("EXA_API_KEY")
	os.Unsetenv("SERPER_API_KEY")

	client := NewSearchClient()
	providers := client.AvailableProviders()
	if len(providers) != 0 {
		t.Errorf("expected 0 providers with no env vars, got %d", len(providers))
	}

	// Test with one key set
	os.Setenv("TAVILY_API_KEY", "test-key")
	client = NewSearchClient()
	providers = client.AvailableProviders()
	if len(providers) != 1 || providers[0] != "tavily" {
		t.Errorf("expected [tavily], got %v", providers)
	}
}

// --- Response Format Tests ---

func TestSearchResponse_MarshalJSON(t *testing.T) {
	resp := SearchResponse{
		Provider: "test",
		Query:    "golang",
		Results: []SearchResult{
			{
				Title:       "Go Programming Language",
				URL:         "https://golang.org",
				Snippet:     "The Go programming language.",
				PublishedAt: "2024-01-01",
				Score:       0.95,
			},
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Verify the JSON structure
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded["provider"] != "test" {
		t.Errorf("expected provider 'test', got %v", decoded["provider"])
	}
	if decoded["query"] != "golang" {
		t.Errorf("expected query 'golang', got %v", decoded["query"])
	}

	results, ok := decoded["results"].([]any)
	if !ok || len(results) != 1 {
		t.Fatalf("expected 1 result, got %v", decoded["results"])
	}

	result := results[0].(map[string]any)
	if result["title"] != "Go Programming Language" {
		t.Errorf("expected title 'Go Programming Language', got %v", result["title"])
	}
}

func TestSearchRequest_UnmarshalJSON(t *testing.T) {
	jsonData := `{"query": "test query", "max_results": 15}`

	var req SearchRequest
	if err := json.Unmarshal([]byte(jsonData), &req); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if req.Query != "test query" {
		t.Errorf("expected query 'test query', got %s", req.Query)
	}
	if req.MaxResults != 15 {
		t.Errorf("expected max_results 15, got %d", req.MaxResults)
	}
}
