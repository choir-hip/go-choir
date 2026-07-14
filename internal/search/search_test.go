package search

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewGatewayClientFromEnvRequiresURLAndToken(t *testing.T) {
	t.Setenv("RUNTIME_GATEWAY_URL", "")
	t.Setenv("PROXY_VMCTL_URL", "")
	t.Setenv("RUNTIME_GATEWAY_TOKEN", "")
	if got := NewGatewayClientFromEnv(); got != nil {
		t.Fatalf("client without URL or token = %#v, want nil", got)
	}

	t.Setenv("RUNTIME_GATEWAY_URL", "https://gateway.example")
	if got := NewGatewayClientFromEnv(); got != nil {
		t.Fatalf("client without token = %#v, want nil", got)
	}

	t.Setenv("RUNTIME_GATEWAY_URL", "")
	t.Setenv("RUNTIME_GATEWAY_TOKEN", "token")
	if got := NewGatewayClientFromEnv(); got != nil {
		t.Fatalf("client without URL = %#v, want nil", got)
	}
}

func TestNewGatewayClientFromEnvPreservesURLPrecedenceAndTimeout(t *testing.T) {
	t.Setenv("RUNTIME_GATEWAY_URL", "  https://runtime-gateway.example  ")
	t.Setenv("PROXY_VMCTL_URL", "https://proxy-gateway.example")
	t.Setenv("RUNTIME_GATEWAY_TOKEN", "  secret-token  ")

	client, ok := NewGatewayClientFromEnv().(*gatewaySearchClient)
	if !ok {
		t.Fatalf("client type = %T, want *gatewaySearchClient", NewGatewayClientFromEnv())
	}
	if client.baseURL != "https://runtime-gateway.example" {
		t.Fatalf("baseURL = %q, want runtime gateway", client.baseURL)
	}
	if client.token != "secret-token" {
		t.Fatalf("token = %q, want trimmed token", client.token)
	}
	if client.httpClient == nil || client.httpClient.Timeout != 30*time.Second {
		t.Fatalf("http client = %#v, want 30s timeout", client.httpClient)
	}

	t.Setenv("RUNTIME_GATEWAY_URL", "")
	client, ok = NewGatewayClientFromEnv().(*gatewaySearchClient)
	if !ok {
		t.Fatalf("fallback client type = %T, want *gatewaySearchClient", NewGatewayClientFromEnv())
	}
	if client.baseURL != "https://proxy-gateway.example" {
		t.Fatalf("fallback baseURL = %q, want proxy gateway", client.baseURL)
	}
}

func TestGatewaySearchClientPreservesRequestAndCompleteResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/provider/v1/search" {
			t.Errorf("path = %q, want /provider/v1/search", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer gateway-token" {
			t.Errorf("Authorization = %q, want bearer token", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", got)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if payload["query"] != "agentic systems" {
			t.Errorf("query = %#v, want agentic systems", payload["query"])
		}
		if payload["max_results"] != float64(40) {
			t.Errorf("max_results = %#v, want 40", payload["max_results"])
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"query":     "agentic systems",
			"provider":  "gateway-merge",
			"providers": []string{"tavily", "brave"},
			"attempts": []map[string]any{{
				"provider": "tavily", "status": "success", "latency_ms": 17, "results": 1,
			}},
			"results": []map[string]any{{
				"title": "Result", "url": "https://example.test/result", "snippet": "Evidence", "provider": "tavily", "score": 0.75,
			}},
			"merged_count": 1,
			"waves":        2,
			"degraded":     true,
			"provider_health": map[string]any{
				"brave": map[string]any{"state": "cooling_down"},
			},
			"outage": false,
			"code":   "partial_results",
			"error":  "brave unavailable",
		})
	}))
	defer server.Close()

	client := &gatewaySearchClient{baseURL: server.URL, token: "gateway-token", httpClient: server.Client()}
	resp, err := client.Search(context.Background(), "agentic systems", 40)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if resp.Query != "agentic systems" || resp.Provider != "gateway-merge" {
		t.Fatalf("identity = query %q provider %q", resp.Query, resp.Provider)
	}
	if len(resp.Providers) != 2 || resp.Providers[0] != "tavily" || resp.Providers[1] != "brave" {
		t.Fatalf("providers = %#v", resp.Providers)
	}
	if len(resp.Attempts) != 1 || resp.Attempts[0]["status"] != "success" {
		t.Fatalf("attempts = %#v", resp.Attempts)
	}
	if len(resp.Results) != 1 || resp.Results[0]["url"] != "https://example.test/result" {
		t.Fatalf("results = %#v", resp.Results)
	}
	if resp.MergedCount != 1 || resp.Waves != 2 || !resp.Degraded || resp.Outage {
		t.Fatalf("aggregate fields = %+v", resp)
	}
	if resp.Code != "partial_results" || resp.Error != "brave unavailable" {
		t.Fatalf("status fields = code %q error %q", resp.Code, resp.Error)
	}
	health, ok := resp.ProviderHealth["brave"].(map[string]any)
	if !ok || health["state"] != "cooling_down" {
		t.Fatalf("provider health = %#v", resp.ProviderHealth)
	}
}

func TestGatewaySearchClientOmitsNonPositiveMaxResults(t *testing.T) {
	for _, maxResults := range []int{0, -1} {
		t.Run(strings.ReplaceAll(time.Duration(maxResults).String(), "-", "negative-"), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var payload map[string]any
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Fatalf("decode request: %v", err)
				}
				if _, ok := payload["max_results"]; ok {
					t.Fatalf("payload = %#v, want max_results omitted", payload)
				}
				_, _ = io.WriteString(w, `{"query":"q","provider":"gateway","results":[]}`)
			}))
			defer server.Close()
			client := &gatewaySearchClient{baseURL: server.URL, token: "token", httpClient: server.Client()}
			if _, err := client.Search(context.Background(), "q", maxResults); err != nil {
				t.Fatalf("Search() error = %v", err)
			}
		})
	}
}

func TestGatewaySearchClientPreservesStructuredOutage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, `{
			"error":"  search_outage  ",
			"query":"  ",
			"provider_health":{
				"tavily":{"state":"cooling_down","cooldown_until":"2026-06-18T05:00:00Z","strike_count":2,"last_failure_class":"rate_limited","last_error_summary":"429 too many requests"},
				"brave":{"state":"active","strike_count":0}
			},
			"attempts":[
				{"provider":"tavily","endpoint":"https://api.tavily.test","status":"cooling_down","latency_ms":0,"results":0,"error":"cooldown active"},
				{"provider":"brave","status":"failed","latency_ms":15,"results":0}
			]
		}`)
	}))
	defer server.Close()

	client := &gatewaySearchClient{baseURL: server.URL, token: "token", httpClient: server.Client()}
	resp, err := client.Search(context.Background(), "  fallback query  ", 5)
	if err != nil {
		t.Fatalf("Search() error = %v, want structured outage", err)
	}
	if !resp.Outage || !resp.Degraded || resp.Query != "fallback query" {
		t.Fatalf("outage identity = %+v", resp)
	}
	if resp.Code != "search_outage" || resp.Error != "search_outage" {
		t.Fatalf("outage code/error = %q/%q", resp.Code, resp.Error)
	}
	if resp.Results == nil || len(resp.Results) != 0 {
		t.Fatalf("results = %#v, want empty non-nil", resp.Results)
	}
	if len(resp.Attempts) != 2 || resp.Attempts[0]["endpoint"] != "https://api.tavily.test" || resp.Attempts[0]["error"] != "cooldown active" {
		t.Fatalf("attempts = %#v", resp.Attempts)
	}
	health, ok := resp.ProviderHealth["tavily"].(map[string]any)
	if !ok || health["state"] != "cooling_down" || health["strike_count"] != 2 || health["last_failure_class"] != "rate_limited" {
		t.Fatalf("tavily health = %#v", resp.ProviderHealth["tavily"])
	}
	brave, ok := resp.ProviderHealth["brave"].(map[string]any)
	if !ok || len(brave) != 2 || brave["state"] != "active" {
		t.Fatalf("brave health = %#v", resp.ProviderHealth["brave"])
	}
}

func TestGatewaySearchClientPreservesHTTPAndDecodeErrors(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		want       string
	}{
		{name: "gateway error", statusCode: http.StatusUnauthorized, body: `{"error":"credential rejected"}`, want: "gateway search: credential rejected"},
		{name: "generic status", statusCode: http.StatusTeapot, body: "not json", want: "gateway search: status 418 I'm a teapot"},
		{name: "malformed success", statusCode: http.StatusOK, body: "{", want: "gateway search: decode response:"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = io.WriteString(w, tt.body)
			}))
			defer server.Close()
			client := &gatewaySearchClient{baseURL: server.URL, token: "token", httpClient: server.Client()}
			_, err := client.Search(context.Background(), "q", 1)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Search() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestGatewaySearchClientPreservesRequestTransportAndReadErrors(t *testing.T) {
	t.Run("request", func(t *testing.T) {
		client := &gatewaySearchClient{baseURL: ":", token: "token", httpClient: http.DefaultClient}
		_, err := client.Search(context.Background(), "q", 1)
		if err == nil || !strings.Contains(err.Error(), "gateway search: create request:") {
			t.Fatalf("Search() error = %v, want request creation error", err)
		}
	})

	t.Run("transport", func(t *testing.T) {
		client := &gatewaySearchClient{
			baseURL: "https://gateway.example",
			token:   "token",
			httpClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return nil, errors.New("dial failed")
			})},
		}
		_, err := client.Search(context.Background(), "q", 1)
		if err == nil || !strings.Contains(err.Error(), "gateway search: http call:") || !strings.Contains(err.Error(), "dial failed") {
			t.Fatalf("Search() error = %v, want transport error", err)
		}
	})

	t.Run("read", func(t *testing.T) {
		client := &gatewaySearchClient{
			baseURL: "https://gateway.example",
			token:   "token",
			httpClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: errorReadCloser{}}, nil
			})},
		}
		_, err := client.Search(context.Background(), "q", 1)
		if err == nil || !strings.Contains(err.Error(), "gateway search: read response: read failed") {
			t.Fatalf("Search() error = %v, want body read error", err)
		}
	})
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type errorReadCloser struct{}

func (errorReadCloser) Read([]byte) (int, error) { return 0, errors.New("read failed") }
func (errorReadCloser) Close() error             { return nil }
