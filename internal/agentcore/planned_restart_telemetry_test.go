package agentcore

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
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

// failingTelemetryPinner refuses every telemetry batch pin once fail is set,
// so a drain deterministically loses its records (no timing race).
type failingTelemetryPinner struct {
	telemetryTestPinner
	fail *atomic.Bool
}

func (p failingTelemetryPinner) PinNonPrivatePayload(ctx context.Context, computerID string, payload []byte, mediaType, privacyClass, pinIntentCommitment string) (computerevent.PinResult, error) {
	if p.fail.Load() {
		return computerevent.PinResult{}, errors.New("test: telemetry pin refused")
	}
	return p.telemetryTestPinner.PinNonPrivatePayload(ctx, computerID, payload, mediaType, privacyClass, pinIntentCommitment)
}

// failingDrainRuntime is batchingPlannedRestartRuntime whose telemetry seals
// fail while the returned flag is set.
func failingDrainRuntime(t *testing.T, computerID string) (*Runtime, *APIHandler, *atomic.Bool) {
	t.Helper()
	rt := chainBootstrapRuntime(t, computerID)
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	key := computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "platform-control", KeyID: "test"}, PrivateKey: privateKey}
	fail := &atomic.Bool{}
	pinner := failingTelemetryPinner{telemetryTestPinner{rollbackTestPinner{key}}, fail}
	appender, err := computerevent.NewComputerEventAppender(computerID, pinner, rt.store, rollbackTestCAS{key: key, projection: rt.store}, rollbackTestReceiptVerifier{})
	if err != nil {
		t.Fatal(err)
	}
	rt.eventAppender = appender
	if err := rt.store.BindProjectionTape(computerID, appender); err != nil {
		t.Fatal(err)
	}
	if _, err := MintProvisionedGenesis(context.Background(), rt.store, appender, computerID, "owner-telemetry"); err != nil {
		t.Fatal(err)
	}
	if err := rt.store.ConfigureTelemetryBatching(store.TelemetryBatchConfig{Enabled: true, Window: 2 * time.Second}); err != nil {
		t.Fatal(err)
	}
	return rt, &APIHandler{rt: rt}, fail
}

func synchronous(rt *Runtime) bool { return rt.store.TelemetryStats().Batching.Synchronous }

func setBusy(t *testing.T, rt *Runtime, loopID string) {
	t.Helper()
	rt.runningMu.Lock()
	if rt.running == nil {
		rt.running = map[string]context.CancelFunc{}
	}
	rt.running[loopID] = func() {}
	rt.runningMu.Unlock()
	t.Cleanup(func() {
		rt.runningMu.Lock()
		delete(rt.running, loopID)
		rt.runningMu.Unlock()
	})
}

// occupyMarkerPath puts a non-empty directory where the marker file goes, so
// both writing and removing the marker fail while something stays there.
func occupyMarkerPath(t *testing.T, rt *Runtime) {
	t.Helper()
	path := PlannedRestartMarkerPath(rt.cfg.StorePath)
	_ = os.Remove(path)
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+"/occupied", []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(path) })
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
	rt, handler, fail := failingDrainRuntime(t, "computer-host-restart-drainfail")
	queueTelemetry(t, rt, 2)
	fail.Store(true)
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

// Failure mode 3 (round 2, finding 2): a busy refusal never undoes the
// synchronous mode an earlier host request entered while that request's marker
// is durable: the host's kill may still follow, and the planned boot would
// read a clean marker over lost telemetry. Once no marker is on disk the
// computer is not restarting, and a refusal returns it to batching.
func TestHostPlannedRestartBusyRefusalKeepsHoldWhileMarkerDurable(t *testing.T) {
	rt, handler := batchingPlannedRestartRuntime(t, "computer-host-restart-busy")
	if code, body := postPlannedRestart(t, handler, "192.0.2.1:4000", `{"target":"761d94c5"}`); code != http.StatusOK {
		t.Fatalf("idle request: status=%d body=%s", code, body)
	}
	if !synchronous(rt) {
		t.Fatal("the host's marker request did not enter synchronous mode")
	}
	setBusy(t, rt, "loop-busy")
	if code, body := postPlannedRestart(t, handler, "192.0.2.1:4000", `{"target":"8a2b0c11"}`); code != http.StatusConflict {
		t.Fatalf("busy retry: status=%d body=%s, want 409", code, body)
	}
	if _, err := os.Stat(PlannedRestartMarkerPath(rt.cfg.StorePath)); err != nil {
		t.Fatalf("the refusal touched the earlier durable marker: %v", err)
	}
	if !synchronous(rt) {
		t.Fatal("a busy refusal reopened the queue while an earlier request's marker is durable")
	}
	// The marker is gone (here: an apply's cleanup removed it): nothing will
	// restart on it, so the next refusal returns the computer to batching.
	if err := ClearPlannedRestartMarker(rt.cfg.StorePath); err != nil {
		t.Fatal(err)
	}
	if code, body := postPlannedRestart(t, handler, "192.0.2.1:4000", `{"target":"8a2b0c11"}`); code != http.StatusConflict {
		t.Fatalf("busy retry: status=%d body=%s, want 409", code, body)
	}
	if !rt.store.TelemetryBatchingEnabled() {
		t.Fatal("a busy refusal with no durable marker left synchronous mode on")
	}
}

// Round 2, finding 1: a failed marker clear keeps synchronous mode on (fail
// closed): the marker may still be durable.
func TestHostPlannedRestartFailedClearKeepsHold(t *testing.T) {
	rt, handler := batchingPlannedRestartRuntime(t, "computer-host-restart-clearfail")
	if code, body := postPlannedRestart(t, handler, "192.0.2.1:4000", `{"target":"761d94c5"}`); code != http.StatusOK {
		t.Fatalf("status=%d body=%s", code, body)
	}
	occupyMarkerPath(t, rt)
	if code := deletePlannedRestart(t, handler); code != http.StatusInternalServerError {
		t.Fatalf("clear of an occupied marker path: status=%d, want 500", code)
	}
	if !synchronous(rt) {
		t.Fatal("a failed marker clear reopened the queue while the marker may be durable")
	}
}

// Round 2, finding 5: a marker write that fails while something is at the
// marker path keeps synchronous mode on; one that left nothing releases it.
func TestHostPlannedRestartFailedWriteKeepsHoldWhenMarkerPathOccupied(t *testing.T) {
	rt, handler := batchingPlannedRestartRuntime(t, "computer-host-restart-writefail")
	occupyMarkerPath(t, rt)
	if code, body := postPlannedRestart(t, handler, "192.0.2.1:4000", `{"target":"761d94c5"}`); code != http.StatusInternalServerError {
		t.Fatalf("write onto an occupied marker path: status=%d body=%s, want 500", code, body)
	}
	if !synchronous(rt) {
		t.Fatal("a failed marker write released synchronous mode although the marker path is occupied")
	}
}

// Round 2, finding 3: synchronous mode is held, not toggled. A guest apply's
// hold and the host's hold each keep it on until both are released.
func TestHostAndApplyHoldsAreIndependent(t *testing.T) {
	rt, handler := batchingPlannedRestartRuntime(t, "computer-host-restart-overlap")
	done, err := rt.markPlannedRestart("self-development apply", "u-overlap")
	if err != nil {
		t.Fatal(err)
	}
	if code, body := postPlannedRestartURL(t, handler, "/internal/runtime/planned-restart?allow_busy=1", `{"target":"761d94c5"}`); code != http.StatusOK {
		t.Fatalf("status=%d body=%s", code, body)
	}
	// The apply returns (its own cleanup clears the shared marker file) while
	// the host's kill is still coming: the host's hold keeps the queue closed.
	done(nil)
	if !synchronous(rt) {
		t.Fatal("the apply's undo released the host's hold")
	}
	// Host/host: a second host request (deploy refresh after store compaction)
	// is still one marker, so one clear releases it.
	if code, body := postPlannedRestartURL(t, handler, "/internal/runtime/planned-restart?allow_busy=1", `{"target":"8a2b0c11"}`); code != http.StatusOK {
		t.Fatalf("status=%d body=%s", code, body)
	}
	if code := deletePlannedRestart(t, handler); code != http.StatusOK {
		t.Fatalf("clear: status=%d", code)
	}
	if !rt.store.TelemetryBatchingEnabled() {
		t.Fatal("after every holder released, synchronous mode is still on")
	}
	// The reverse order: the host clears first, the apply still holds.
	done2, err := rt.markPlannedRestart("self-development apply", "u-overlap-2")
	if err != nil {
		t.Fatal(err)
	}
	if code, body := postPlannedRestartURL(t, handler, "/internal/runtime/planned-restart?allow_busy=1", `{"target":"9c3d"}`); code != http.StatusOK {
		t.Fatalf("status=%d body=%s", code, body)
	}
	if code := deletePlannedRestart(t, handler); code != http.StatusOK {
		t.Fatalf("clear: status=%d", code)
	}
	if !synchronous(rt) {
		t.Fatal("the host's clear released the apply's hold")
	}
	done2(nil)
	if !rt.store.TelemetryBatchingEnabled() {
		t.Fatal("synchronous mode outlived both holders")
	}
}

// Round 2, finding 6: an earlier failed drain is not forgotten when the host
// asks again before the restart; the second marker still names the gap.
func TestHostPlannedRestartRepeatKeepsDrainFailure(t *testing.T) {
	rt, handler, fail := failingDrainRuntime(t, "computer-host-restart-repeat")
	queueTelemetry(t, rt, 2)
	fail.Store(true)
	if code, body := postPlannedRestart(t, handler, "192.0.2.1:4000", `{"target":"761d94c5"}`); code != http.StatusOK {
		t.Fatalf("status=%d body=%s", code, body)
	}
	fail.Store(false)
	if code, body := postPlannedRestart(t, handler, "192.0.2.1:4000", `{"target":"8a2b0c11"}`); code != http.StatusOK {
		t.Fatalf("status=%d body=%s", code, body)
	}
	marker, planned := ConsumePlannedRestartMarker(rt.cfg.StorePath, time.Now())
	if !planned || !marker.TelemetryDrainFailed {
		t.Fatalf("second marker planned=%v %+v forgot the first drain's lost records", planned, marker)
	}
}

// Round 2, finding 4: a sealer left in place after the owner turned batching
// off does not make the host's request drain or enter synchronous mode.
func TestHostPlannedRestartBatchingTurnedOffIsUnchanged(t *testing.T) {
	rt, handler := batchingPlannedRestartRuntime(t, "computer-host-restart-turnedoff")
	if err := rt.store.ConfigureTelemetryBatching(store.TelemetryBatchConfig{Enabled: false}); err != nil {
		t.Fatal(err)
	}
	code, body := postPlannedRestart(t, handler, "192.0.2.1:4000", `{"target":"761d94c5"}`)
	if code != http.StatusOK || strings.Contains(body, "telemetry") {
		t.Fatalf("status=%d body=%s, want 200 and an unchanged response", code, body)
	}
	if synchronous(rt) {
		t.Fatal("the host's request entered synchronous mode with batching turned off")
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
