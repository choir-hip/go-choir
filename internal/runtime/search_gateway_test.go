package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGatewaySearchClientParsesSearchOutage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/provider/v1/search" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": "search_outage",
			"code":  "search_outage",
			"query": "ai infrastructure june 2026",
			"provider_health": map[string]any{
				"tavily": map[string]any{
					"state":              "cooling_down",
					"cooldown_until":       "2026-06-18T05:00:00Z",
					"strike_count":         1,
					"last_failure_class":   "rate_limited",
					"last_error_summary":   "429 too many requests",
				},
				"brave": map[string]any{
					"state": "active",
				},
			},
			"attempts": []map[string]any{
				{
					"provider":   "tavily",
					"status":     "cooling_down",
					"latency_ms": 0,
					"results":    0,
					"error":      "provider cooling down until 2026-06-18T05:00:00Z",
				},
				{
					"provider":   "brave",
					"status":     "rate_limited",
					"latency_ms": 42,
					"results":    0,
					"error":      "429 too many requests",
				},
			},
		})
	}))
	defer server.Close()

	client := &gatewaySearchClient{
		baseURL:    server.URL,
		token:      "sandbox-token",
		httpClient: server.Client(),
	}
	resp, err := client.Search(context.Background(), "ai infrastructure june 2026", 5)
	if err != nil {
		t.Fatalf("Search() error = %v, want structured outage response", err)
	}
	if resp == nil || !resp.Outage {
		t.Fatalf("resp = %+v, want outage=true", resp)
	}
	if resp.Code != "search_outage" {
		t.Fatalf("code = %q, want search_outage", resp.Code)
	}
	if len(resp.Results) != 0 {
		t.Fatalf("results = %d, want 0", len(resp.Results))
	}
	health, ok := resp.ProviderHealth["tavily"].(map[string]any)
	if !ok || health["state"] != "cooling_down" {
		t.Fatalf("tavily health = %#v, want cooling_down", resp.ProviderHealth["tavily"])
	}
	if len(resp.Attempts) != 2 {
		t.Fatalf("attempts = %d, want 2", len(resp.Attempts))
	}
}
