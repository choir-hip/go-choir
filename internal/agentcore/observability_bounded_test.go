package agentcore

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// docs/problems/engineering-completion-path-cluster-2026-10-20.md failure 9:
// a read that waits on a held lock must not hold the route. The bounded read
// gives up at its budget and says so; a read that returns in time is kept.
func TestBoundedObservabilityReadGivesUpOnABlockedRead(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	start := time.Now()
	_, ok := boundedObservabilityRead(50*time.Millisecond, func() int { <-release; return 1 })
	if ok || time.Since(start) > 2*time.Second {
		t.Fatalf("blocked read: ok=%v after %s", ok, time.Since(start))
	}
	v, ok := boundedObservabilityRead(2*time.Second, func() int { return 7 })
	if !ok || v != 7 {
		t.Fatalf("fast read = %d, %v", v, ok)
	}
}

// The owner can ask the route for the runtime's goroutine stacks, so a hang
// names the call it is stopped in without SSH. Opt-in, bounded, marked when
// cut.
func TestObservabilityGoroutineDumpIsOptInAndBounded(t *testing.T) {
	rt := telemetryRuntime(t, "computer-goroutine-dump")
	read := func(query string) map[string]any {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/api/runtime/observability"+query, nil)
		req.Header.Set("X-Authenticated-User", "owner-telemetry")
		rec := httptest.NewRecorder()
		(&APIHandler{rt: rt}).HandleGuestObservability(rec, req)
		var obs map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &obs); err != nil || rec.Code != http.StatusOK {
			t.Fatalf("observability%s = %d %s", query, rec.Code, rec.Body.String())
		}
		return obs
	}
	if _, ok := read("")["goroutines"]; ok {
		t.Fatal("goroutines present without ?goroutines=1")
	}
	dump, _ := read("?goroutines=1")["goroutines"].(string)
	if !strings.Contains(dump, "goroutine ") || !strings.Contains(dump, "TestObservabilityGoroutineDumpIsOptInAndBounded") {
		t.Fatalf("goroutine dump does not show this test's stack: %.300s", dump)
	}
	if cut := boundGoroutineDump(strings.Repeat("x", 100), 10); !strings.HasPrefix(cut, strings.Repeat("x", 10)) ||
		!strings.Contains(cut, "truncated: 10 of 100 bytes") {
		t.Fatalf("bounded dump = %q", cut)
	}
}
