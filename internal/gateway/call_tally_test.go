package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/provider"
)

// docs/problems/gateway-inference-bucket-self-imposed-2026-11-10.md: a
// computer's own model and search calls are never refused by a gateway
// count. The limiter that remains (the Anthropic relay's) is passed in here
// at one request a minute to prove no other route draws on it.

func TestModelAndSearchCallsAreNotRefusedByACount(t *testing.T) {
	h, reg, _ := setupHandlerWithRateLimit(t, 1, time.Minute)
	h.searchClient = testSearchClient([]SearchProvider{&mockSearchProvider{
		name:      "search",
		available: true,
		searchFunc: func(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
			return []SearchResult{{Title: "R", URL: "http://example.com", Snippet: "s"}}, nil
		},
	}})
	cred, err := reg.IssueCredential("computer-busy")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(ProviderRequest{
		Messages: []provider.Message{{Role: "user", Content: []provider.Block{{Type: "text", Text: "Hi"}}}},
	})
	for i := 0; i < 150; i++ {
		req := httptest.NewRequest(http.MethodPost, "/provider/v1/inference", strings.NewReader(string(body)))
		req.Header.Set("Authorization", "Bearer "+cred.RawToken)
		w := httptest.NewRecorder()
		h.HandleInference(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("inference call %d: status %d, want 200: %s", i+1, w.Code, w.Body.String())
		}
		sreq := httptest.NewRequest(http.MethodPost, "/provider/v1/search", strings.NewReader(`{"query":"ai news"}`))
		sreq.Header.Set("Authorization", "Bearer "+cred.RawToken)
		sw := httptest.NewRecorder()
		h.HandleSearch(sw, sreq)
		if sw.Code != http.StatusOK {
			t.Fatalf("search call %d: status %d, want 200: %s", i+1, sw.Code, sw.Body.String())
		}
	}
}

func TestHealthNoLongerReportsAnInferenceRateLimit(t *testing.T) {
	h, _, _ := setupHandlerWithRateLimit(t, 60, time.Minute)
	w := httptest.NewRecorder()
	h.HandleHealth(w, httptest.NewRequest(http.MethodGet, "/health", nil))
	if strings.Contains(w.Body.String(), "rate_limit") {
		t.Fatalf("health still reports a rate limit: %s", w.Body.String())
	}
}

func TestCallTallyLogsEachComputerAndProviderOnceAnHour(t *testing.T) {
	now := time.Date(2026, 11, 10, 14, 5, 0, 0, time.UTC)
	var lines []string
	tally := &callTally{now: func() time.Time { return now }, logf: func(f string, a ...any) {
		lines = append(lines, fmt.Sprintf(f, a...))
	}}
	tally.Call("computer-a", "bedrock")
	tally.Call("computer-a", "bedrock")
	tally.InputTokens("computer-a", "bedrock", 1500)
	tally.Call("computer-a", "search")
	tally.Call("computer-b", "bedrock")
	now = now.Add(30 * time.Minute)
	tally.Call("computer-b", "bedrock")
	if len(lines) != 0 {
		t.Fatalf("a line was written inside the hour: %v", lines)
	}
	now = now.Add(time.Hour)
	tally.Call("computer-a", "bedrock")
	want := []string{
		"gateway: calls hour=2026-11-10T14:00Z computer=computer-a provider=bedrock calls=2 input_tokens=1500",
		"gateway: calls hour=2026-11-10T14:00Z computer=computer-a provider=search calls=1 input_tokens=0",
		"gateway: calls hour=2026-11-10T14:00Z computer=computer-b provider=bedrock calls=2 input_tokens=0",
	}
	if strings.Join(lines, "\n") != strings.Join(want, "\n") {
		t.Fatalf("hour lines =\n%s\nwant\n%s", strings.Join(lines, "\n"), strings.Join(want, "\n"))
	}
}
