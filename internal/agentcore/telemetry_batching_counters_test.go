package agentcore

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The tape write counters are part of the observability JSON, under their wire
// names (storage solutions 2, S5).
func TestObservabilityServesTheTapeWriteCounters(t *testing.T) {
	rt := telemetryRuntime(t, "computer-tape-counters-obs")
	queueTelemetry(t, rt, 2)
	req := httptest.NewRequest(http.MethodGet, "/api/runtime/observability", nil)
	req.Header.Set("X-Authenticated-User", "owner-telemetry")
	rec := httptest.NewRecorder()
	(&APIHandler{rt: rt}).HandleGuestObservability(rec, req)
	var obs struct {
		TelemetryBatching map[string]json.RawMessage `json:"telemetry_batching"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &obs); err != nil || obs.TelemetryBatching == nil {
		t.Fatalf("observability = %d %s", rec.Code, rec.Body.String())
	}
	for _, key := range []string{
		"tape_bytes", "tape_events_by_family", "tape_bytes_by_family", "tape_ops_by_object_kind",
		"tape_op_bytes_by_object_kind", "run_memory_ops_by_kind", "run_memory_bytes_by_kind", "lifecycle_command_forms", "corpusd_calls",
		"corpusd_calls_per_tape_event", "appender_depth_at_enter", "appender_inflight", "appender_call_mean_ms",
	} {
		if _, ok := obs.TelemetryBatching[key]; !ok {
			t.Errorf("telemetry_batching lacks %q", key)
		}
	}
}

// The per-kind rewrite counters ride in the same block: object_rewrites maps a
// kind to ops, bytes, mean bytes per op, distinct objects, writes per object
// and the estimated flag.
func TestObservabilityServesTheObjectRewriteCounters(t *testing.T) {
	rt := telemetryRuntime(t, "computer-rewrite-counters-obs")
	queueTelemetry(t, rt, 2) // each event is one object op of kind choir.event
	const kind = "choir.event"
	wantOps := rt.store.TelemetryStats().TapeOpsByObjectKind[kind]
	wantBytes := rt.store.TelemetryStats().TapeOpBytesByObjectKind[kind]
	if wantOps < 2 || wantBytes <= 0 {
		t.Fatalf("setup: the tape counted %d %s ops and %d bytes, want at least 2 and some bytes", wantOps, kind, wantBytes)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/runtime/observability", nil)
	req.Header.Set("X-Authenticated-User", "owner-telemetry")
	rec := httptest.NewRecorder()
	(&APIHandler{rt: rt}).HandleGuestObservability(rec, req)
	var obs struct {
		TelemetryBatching struct {
			ObjectRewrites map[string]map[string]json.RawMessage `json:"object_rewrites"`
		} `json:"telemetry_batching"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &obs); err != nil {
		t.Fatalf("observability = %d %s", rec.Code, rec.Body.String())
	}
	served := obs.TelemetryBatching.ObjectRewrites
	if len(served) == 0 {
		t.Fatalf("object_rewrites is empty or absent: %s", rec.Body.String())
	}
	fields, ok := served[kind]
	if !ok {
		t.Fatalf("object_rewrites lacks %s: %s", kind, rec.Body.String())
	}
	for _, key := range []string{"ops", "bytes", "mean_bytes_per_op", "distinct_objects", "writes_per_object", "estimated"} {
		if _, ok := fields[key]; !ok {
			t.Errorf("object_rewrites[%s] lacks %q", kind, key)
		}
	}
	var ops, bytes int64
	if err := json.Unmarshal(fields["ops"], &ops); err != nil || ops != wantOps {
		t.Errorf("served ops = %s, want %d", fields["ops"], wantOps)
	}
	if err := json.Unmarshal(fields["bytes"], &bytes); err != nil || bytes != wantBytes {
		t.Errorf("served bytes = %s, want %d", fields["bytes"], wantBytes)
	}
}
