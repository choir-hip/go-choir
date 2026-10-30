package agentcore

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/store"
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

// The memory breakdown is opt-in (it walks the store and the nix store) and
// splits the page cache by this computer's store and the shared nix store.
// Failure modes: the default request paying for the walk; the store tree
// missing from the split; the store path's parent walked instead of the
// store.
func TestObservabilityMemoryDetailIsOptInAndSplitsTheStore(t *testing.T) {
	rt := telemetryRuntime(t, "computer-memory-detail")
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
	if _, ok := read("")["memory_detail"]; ok {
		t.Fatal("memory_detail present without ?memory=detail")
	}
	detail, _ := read("?memory=detail")["memory_detail"].(map[string]any)
	if detail == nil {
		t.Fatal("memory_detail missing with ?memory=detail")
	}
	splits, _ := detail["page_cache"].([]any)
	labels := map[string]string{}
	storeBytes := float64(0)
	for _, s := range splits {
		m, _ := s.(map[string]any)
		label, _ := m["label"].(string)
		root, _ := m["root"].(string)
		labels[label] = root
		if label == "store" {
			storeBytes, _ = m["bytes"].(float64)
		}
	}
	// StorePath names the store's directory; the SQLite state file is the store.
	workspace := store.StateDBPath(rt.cfg.StorePath)
	if labels["store"] != workspace {
		t.Fatalf("store entry root=%q, want the state file %q", labels["store"], workspace)
	}
	// Bytes are counted only where the residency probe runs (Linux).
	if runtime.GOOS == "linux" && storeBytes <= 0 {
		t.Fatalf("store entry bytes=%v, want bytes > 0", storeBytes)
	}
	if labels["store"] != workspace || labels["nix_store"] != "/nix/store" {
		t.Fatalf("page_cache trees = %v, want store at %q and the nix store", labels, workspace)
	}
}
