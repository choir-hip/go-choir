package proxy

import (
	"testing"
	"time"
)

// docs/archive/problems/tracer-paper-blind-to-test-publications-2026-10-13.md: the
// publish stages counted every 201 and every "published" as an error
// (errors: 49 with by_status {http_201: 49}). Failure modes pinned: a 2xx
// other than 200 counts as an error; the publish path's "published" counts
// as an error; a 4xx, 5xx or named failure stops counting.
func TestLifecycleErrorsCountOnlyFailures(t *testing.T) {
	r := newLifecycleRecorder()
	for _, status := range []string{"ok", "connected", "published", lifecycleHTTPStatus(200), lifecycleHTTPStatus(201), lifecycleHTTPStatus(204)} {
		r.record("stage", status, time.Millisecond)
	}
	for _, status := range []string{lifecycleHTTPStatus(400), lifecycleHTTPStatus(502), "forbidden", "platform_error"} {
		r.record("stage", status, time.Millisecond)
	}
	if got := r.stages["stage"].Errors; got != 4 {
		t.Fatalf("errors = %d, want 4 (only the failures)", got)
	}
}
