package agentcore

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// R7 slice S2: the deployed proof (design 9.1 step 4 (f), (g)) reads the trace
// log's counters through the product observability route, with no SQL and no
// SSH. Ways it can fail: the block is absent; a field the proof needs is
// missing; the block is served from a store that has no log directory yet
// (it must still say so, not omit the key).
func TestGuestObservabilityServesTheTraceLogBlock(t *testing.T) {
	rt, _ := testRuntime(t)
	req := httptest.NewRequest(http.MethodGet, "/api/runtime/observability", nil)
	req.Header.Set("X-Authenticated-User", "owner-trace-log-obs")
	response := httptest.NewRecorder()
	(&APIHandler{rt: rt}).HandleGuestObservability(response, req)
	if response.Code != http.StatusOK {
		t.Fatalf("observability = %d %s", response.Code, response.Body.String())
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	raw, ok := body["trace_log"]
	if !ok {
		t.Fatalf("observability has no trace_log block: %s", response.Body.String())
	}
	var block map[string]any
	if err := json.Unmarshal(raw, &block); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"open", "dir", "records", "bytes_by_kind", "fsyncs", "barrier_fsyncs", "barrier_fsync_p99_ms",
		"segments", "retained_bytes", "retention_floor", "dropped", "fallbacks", "poisoned"} {
		if _, present := block[key]; !present {
			t.Errorf("trace_log lacks %q: %s", key, raw)
		}
	}
}
