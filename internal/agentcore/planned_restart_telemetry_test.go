package agentcore

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/store"
)

// The host's planned restart (vmctl's deploy refresh) and run-telemetry
// batching (docs/problems/host-planned-restart-skips-telemetry-drain-2026-10-22.md).
// vmctl asks the guest for a marker, then SIGKILLs the VM, so the guest's
// shutdown drain never runs: the marker request is the only place the queue
// can be sealed. Ways this can fail, written before the code (the record's
// failure modes 1-4, plus the flag-off pin):
//  1. the marker is durable while records are still queued, or telemetry
//     appended after the marker queues again and dies with the kill;
//  2. a failed or timed-out drain blocks the marker, or the marker does not
//     carry telemetry_drain_failed, so the passivation record hides the gap;
//  3. a busy refusal leaves synchronous mode on, so a computer that is not
//     restarting runs acknowledged until its next boot;
//  4. vmctl's marker clear (DELETE) after a failed refresh leaves
//     synchronous mode on, with the same result;
//  5. with batching off (every computer today) the request does anything it
//     did not do before: creates a sealer, drains, or changes the marker.

func batchingPlannedRestartRuntime(t *testing.T, computerID string) (*Runtime, *APIHandler) {
	t.Helper()
	rt := telemetryRuntime(t, computerID)
	if err := rt.store.ConfigureTelemetryBatching(store.TelemetryBatchConfig{Enabled: true, Window: 2 * time.Second}); err != nil {
		t.Fatal(err)
	}
	return rt, &APIHandler{rt: rt}
}

func deletePlannedRestart(t *testing.T, handler *APIHandler) int {
	t.Helper()
	req := httptest.NewRequest(http.MethodDelete, "/internal/runtime/planned-restart", nil)
	req.RemoteAddr = "192.0.2.1:4000"
	rec := httptest.NewRecorder()
	handler.HandleInternalPlannedRestart(rec, req)
	return rec.Code
}

// Failure mode 1.
func TestHostPlannedRestartDrainsTelemetryBeforeMarker(t *testing.T) {
	rt, handler := batchingPlannedRestartRuntime(t, "computer-host-restart-drain")
	queueTelemetry(t, rt, 3)
	if depth := rt.store.TelemetryStats().Batching.QueueDepth; depth != 3 {
		t.Fatalf("queue depth = %d, want 3 pending records", depth)
	}
	code, body := postPlannedRestartURL(t, handler, "/internal/runtime/planned-restart?allow_busy=1", `{"target":"761d94c5"}`)
	if code != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", code, body)
	}
	stats := rt.store.TelemetryStats().Batching
	if stats.QueueDepth != 0 || stats.OpsSealed != 3 {
		t.Fatalf("after the host's marker request: depth %d, ops sealed %d; want the 3 queued records sealed before the marker", stats.QueueDepth, stats.OpsSealed)
	}
	if !stats.Synchronous || rt.store.TelemetryBatchingEnabled() {
		t.Fatalf("synchronous=%v enabled=%v: telemetry after the marker must take the acknowledged path until the kill", stats.Synchronous, rt.store.TelemetryBatchingEnabled())
	}
	queueTelemetry(t, rt, 1)
	if depth := rt.store.TelemetryStats().Batching.QueueDepth; depth != 0 {
		t.Fatalf("telemetry appended after the marker was queued (depth %d); the kill would lose it", depth)
	}
	marker, planned := ConsumePlannedRestartMarker(rt.cfg.StorePath, time.Now())
	if !planned || marker.TelemetryDrainFailed {
		t.Fatalf("marker after a clean drain: planned=%v %+v", planned, marker)
	}
}

// Failure mode 2.
func TestHostPlannedRestartDrainFailureStillWritesMarkerAndNamesGap(t *testing.T) {
	rt, handler := batchingPlannedRestartRuntime(t, "computer-host-restart-drainfail")
	queueTelemetry(t, rt, 2)
	rt.telemetryDrainDeadline = time.Nanosecond
	code, body := postPlannedRestart(t, handler, "192.0.2.1:4000", `{"target":"761d94c5"}`)
	if code != http.StatusOK {
		t.Fatalf("a failed drain blocked the host's restart: status=%d body=%s", code, body)
	}
	if !strings.Contains(body, `"telemetry_drain_failed":true`) {
		t.Fatalf("response %s does not tell the host the drain failed", body)
	}
	marker, planned := ConsumePlannedRestartMarker(rt.cfg.StorePath, time.Now())
	if !planned || !marker.TelemetryDrainFailed {
		t.Fatalf("marker planned=%v %+v, want written and carrying telemetry_drain_failed", planned, marker)
	}
	if payload := passivatedOnRestartPayload(marker); !strings.Contains(string(payload), telemetryMissingWindowNote) {
		t.Fatalf("passivation record %s does not name the gap", payload)
	}
}

// Failure mode 3: a refusal undoes the synchronous mode the host's earlier
// request entered, so a computer that is not restarting goes back to batching.
func TestHostPlannedRestartBusyRefusalUndoesSynchronousMode(t *testing.T) {
	rt, handler := batchingPlannedRestartRuntime(t, "computer-host-restart-busy")
	if code, body := postPlannedRestart(t, handler, "192.0.2.1:4000", `{"target":"761d94c5"}`); code != http.StatusOK {
		t.Fatalf("idle request: status=%d body=%s", code, body)
	}
	if !rt.store.TelemetryStats().Batching.Synchronous {
		t.Fatal("the host's marker request did not enter synchronous mode")
	}
	// The refresh did not happen; by the host's retry a run is running.
	rt.runningMu.Lock()
	if rt.running == nil {
		rt.running = map[string]context.CancelFunc{}
	}
	rt.running["loop-busy"] = func() {}
	rt.runningMu.Unlock()
	t.Cleanup(func() {
		rt.runningMu.Lock()
		delete(rt.running, "loop-busy")
		rt.runningMu.Unlock()
	})
	if code, body := postPlannedRestart(t, handler, "192.0.2.1:4000", `{"target":"8a2b0c11"}`); code != http.StatusConflict {
		t.Fatalf("busy retry: status=%d body=%s, want 409", code, body)
	}
	if !rt.store.TelemetryBatchingEnabled() {
		t.Fatal("a busy refusal left synchronous mode on: the computer runs acknowledged until its next boot")
	}
}

// Failure mode 4.
func TestHostPlannedRestartMarkerClearUndoesSynchronousMode(t *testing.T) {
	rt, handler := batchingPlannedRestartRuntime(t, "computer-host-restart-clear")
	if code, body := postPlannedRestartURL(t, handler, "/internal/runtime/planned-restart?allow_busy=1", `{"target":"761d94c5"}`); code != http.StatusOK {
		t.Fatalf("status=%d body=%s", code, body)
	}
	if !rt.store.TelemetryStats().Batching.Synchronous {
		t.Fatal("the host's marker request did not enter synchronous mode")
	}
	if code := deletePlannedRestart(t, handler); code != http.StatusOK {
		t.Fatalf("clear: status=%d", code)
	}
	if _, err := os.Stat(PlannedRestartMarkerPath(rt.cfg.StorePath)); !os.IsNotExist(err) {
		t.Fatalf("clear left the marker: %v", err)
	}
	if !rt.store.TelemetryBatchingEnabled() {
		t.Fatal("marker clear after a failed refresh left synchronous mode on")
	}
	queueTelemetry(t, rt, 1)
	if depth := rt.store.TelemetryStats().Batching.QueueDepth; depth != 1 {
		t.Fatalf("after the clear telemetry is not queued again (depth %d)", depth)
	}
}

// Failure mode 5: batching is off on every computer today; the request must
// behave exactly as before.
func TestHostPlannedRestartFlagOffIsUnchanged(t *testing.T) {
	rt := telemetryRuntime(t, "computer-host-restart-flagoff")
	handler := &APIHandler{rt: rt}
	rt.telemetryDrainDeadline = time.Nanosecond // a drain attempt would fail and show on the marker
	code, body := postPlannedRestart(t, handler, "192.0.2.1:4000", `{"target":"761d94c5"}`)
	if code != http.StatusOK {
		t.Fatalf("status=%d body=%s", code, body)
	}
	if strings.Contains(body, "telemetry") {
		t.Fatalf("flag-off response changed: %s", body)
	}
	raw, err := os.ReadFile(PlannedRestartMarkerPath(rt.cfg.StorePath))
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	for key := range fields {
		switch key {
		case "reason", "target", "requested_at", "work_state_epoch":
		default:
			t.Fatalf("flag-off marker carries %q: %s", key, raw)
		}
	}
	if rt.store.TelemetryBatchingConfigured() || rt.store.TelemetryBatchingEnabled() {
		t.Fatal("the host's request created a sealer with the flag off")
	}
	if code := deletePlannedRestart(t, handler); code != http.StatusOK {
		t.Fatalf("clear: status=%d", code)
	}
	if rt.store.TelemetryBatchingConfigured() {
		t.Fatal("the marker clear created a sealer with the flag off")
	}
}
