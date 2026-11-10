package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
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

// tallyHarness is a tally with an injected clock and log sink; safe to read
// from the test while the timer goroutine runs.
type tallyHarness struct {
	tally *callTally
	ns    atomic.Int64
	mu    sync.Mutex
	lines []string
	tick  chan time.Time
}

func newTallyHarness(start time.Time) *tallyHarness {
	h := &tallyHarness{tick: make(chan time.Time)}
	h.ns.Store(start.UnixNano())
	h.tally = &callTally{
		now:   func() time.Time { return time.Unix(0, h.ns.Load()).UTC() },
		after: func(time.Duration) <-chan time.Time { return h.tick },
		logf: func(f string, a ...any) {
			h.mu.Lock()
			h.lines = append(h.lines, fmt.Sprintf(f, a...))
			h.mu.Unlock()
		},
	}
	return h
}

func (h *tallyHarness) advance(d time.Duration) { h.ns.Add(int64(d)) }

func (h *tallyHarness) logged() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.lines...)
}

func (h *tallyHarness) waitLines(t *testing.T, n int) []string {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if got := h.logged(); len(got) >= n {
			return got
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("wanted %d lines, got %v", n, h.logged())
	return nil
}

func (h *tallyHarness) fire(t *testing.T) {
	t.Helper()
	select {
	case h.tick <- time.Time{}:
	case <-time.After(3 * time.Second):
		t.Fatal("the tally timer was not waiting")
	}
}

func TestCallTallyLogsEachComputerAndProviderOnceAnHour(t *testing.T) {
	h := newTallyHarness(time.Date(2026, 11, 10, 14, 5, 0, 0, time.UTC))
	h.tally.Call("computer-a", "bedrock")
	h.tally.Call("computer-a", "bedrock")
	h.tally.Served("computer-a", "bedrock", "bedrock", 1500)
	h.tally.Call("computer-a", "search")
	h.tally.Call("computer-b", "bedrock")
	h.advance(30 * time.Minute)
	h.tally.Call("computer-b", "bedrock")
	if len(h.logged()) != 0 {
		t.Fatalf("a line was written inside the hour: %v", h.logged())
	}
	h.advance(time.Hour)
	h.tally.Call("computer-a", "bedrock")
	want := []string{
		"gateway: calls hour=2026-11-10T14:00Z computer=computer-a provider=bedrock calls=2 input_tokens=1500",
		"gateway: calls hour=2026-11-10T14:00Z computer=computer-a provider=search calls=1 input_tokens=0",
		"gateway: calls hour=2026-11-10T14:00Z computer=computer-b provider=bedrock calls=2 input_tokens=0",
	}
	if got := strings.Join(h.logged(), "\n"); got != strings.Join(want, "\n") {
		t.Fatalf("hour lines =\n%s\nwant\n%s", got, strings.Join(want, "\n"))
	}
}

func TestCallTallyCallAndTokensShareTheServingProvider(t *testing.T) {
	h := newTallyHarness(time.Date(2026, 11, 10, 14, 5, 0, 0, time.UTC))
	h.tally.Call("c", "primary")
	h.tally.Served("c", "primary", "fallback", 700) // a fallback served it
	h.tally.Call("c", "primary")                    // admitted, then failed: stays with primary
	h.advance(time.Hour)
	h.tally.Call("c", "primary") // closes the 14:00 hour
	want := []string{
		"gateway: calls hour=2026-11-10T14:00Z computer=c provider=fallback calls=1 input_tokens=700",
		"gateway: calls hour=2026-11-10T14:00Z computer=c provider=primary calls=1 input_tokens=0",
	}
	if got := strings.Join(h.logged(), "\n"); got != strings.Join(want, "\n") {
		t.Fatalf("lines =\n%s\nwant\n%s", got, strings.Join(want, "\n"))
	}
}

// A quiet hour is logged by the timer, with no further call; a call and the
// timer racing over the same closed hour log it once; stopping drops the
// partial hour.
func TestCallTallyTimerLogsAClosedHourOnceAndStopDropsTheOpenOne(t *testing.T) {
	h := newTallyHarness(time.Date(2026, 11, 10, 14, 5, 0, 0, time.UTC))
	stop := h.tally.start()
	h.tally.Call("c", "bedrock")
	h.fire(t) // still inside the hour: nothing closes
	h.advance(time.Hour)
	// The timer and a call both see the closed hour at once.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); h.tally.Call("c", "bedrock") }()
	h.fire(t)
	wg.Wait()
	got := h.waitLines(t, 1)
	if len(got) != 1 || !strings.Contains(got[0], "hour=2026-11-10T14:00Z computer=c provider=bedrock calls=1") {
		t.Fatalf("closed hour logged %v, want exactly one 14:00 line", got)
	}
	h.fire(t) // a second tick over the same hour is a no-op
	h.advance(time.Hour)
	h.fire(t) // the 15:00 hour, holding one call, closes by the timer alone
	got = h.waitLines(t, 2)
	if len(got) != 2 || !strings.Contains(got[1], "hour=2026-11-10T15:00Z") {
		t.Fatalf("idle hour not flushed by the timer: %v", got)
	}
	h.tally.Call("c", "bedrock") // open partial hour
	stop()
	stop() // idempotent
	if n := len(h.logged()); n != 2 {
		t.Fatalf("stop logged the partial hour: %v", h.logged())
	}
}
