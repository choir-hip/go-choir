package agentcore

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Telemetry batching at the runtime seam (docs/reports/telemetry-batching-
// design-2026-10-20.md, plan items P1-F14, F16, F17). Ways it can fail,
// written before the code:
//   - a planned restart writes its marker while telemetry is still queued, so
//     the one resume a planned restart allows starts on a tape with a gap;
//   - a failed or timed-out drain blocks the restart, or is silent;
//   - the sealer's synchronous mode outlives an apply that never restarted;
//   - the crash-close or resume record does not say the trace may be missing,
//     or the flag-off record changes;
//   - the owner's setting does not reach the store, or turning it off leaves
//     records queued;
//   - the counters cannot be read without ssh.

// telemetryTestPinner adds the non-private payload pin the projection-batch
// path needs to the package's rollback pinner.
type telemetryTestPinner struct{ rollbackTestPinner }

func (p telemetryTestPinner) PinNonPrivatePayload(_ context.Context, computerID string, payload []byte, mediaType, privacyClass, pinIntentCommitment string) (computerevent.PinResult, error) {
	digest := computerevent.DigestBytes(payload)
	receipt, err := computerevent.NewSignedReceipt("PinReceipt", "corpusd", map[string]any{
		"computer_id": computerID, "artifact_digest": digest, "media_type": mediaType,
		"privacy_class": privacyClass, "pin_intent_commitment": pinIntentCommitment,
	}, []computerevent.SigningKey{p.key}, time.Date(2026, 8, 16, 18, 2, 0, 0, time.UTC))
	return computerevent.PinResult{ArtifactDigest: digest, Receipt: receipt}, err
}

// telemetryRuntime is a runtime over a tape-bound store with a genesis, able
// to append projection batches.
func telemetryRuntime(t *testing.T, computerID string) *Runtime {
	t.Helper()
	rt := chainBootstrapRuntime(t, computerID)
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	key := computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "platform-control", KeyID: "test"}, PrivateKey: privateKey}
	appender, err := computerevent.NewComputerEventAppender(computerID, telemetryTestPinner{rollbackTestPinner{key}}, rt.store, rollbackTestCAS{key: key, projection: rt.store}, rollbackTestReceiptVerifier{})
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
	return rt
}

func queueTelemetry(t *testing.T, rt *Runtime, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		rec := &types.EventRecord{
			EventID: fmt.Sprintf("q-%d", i), RunID: "run-q", OwnerID: "owner-telemetry",
			Timestamp: time.Now().UTC(), Kind: types.EventToolResult,
		}
		if err := rt.store.AppendEvent(context.Background(), rec); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPlannedRestartDrainsBeforeMarker(t *testing.T) {
	rt := telemetryRuntime(t, "computer-telemetry-planned")
	if err := rt.store.ConfigureTelemetryBatching(store.TelemetryBatchConfig{Enabled: true, Window: 2 * time.Second}); err != nil {
		t.Fatal(err)
	}
	queueTelemetry(t, rt, 3)
	if depth := rt.store.TelemetryStats().Batching.QueueDepth; depth != 3 {
		t.Fatalf("queue depth = %d, want 3 pending records", depth)
	}
	done, err := rt.markPlannedRestart("platform update", "u-1")
	if err != nil {
		t.Fatal(err)
	}
	// The marker is on disk only after the queue was sealed.
	if _, err := os.Stat(PlannedRestartMarkerPath(rt.cfg.StorePath)); err != nil {
		t.Fatalf("marker not written: %v", err)
	}
	stats := rt.store.TelemetryStats().Batching
	if stats.QueueDepth != 0 || stats.BatchesSealed != 1 || stats.OpsSealed != 3 {
		t.Fatalf("after markPlannedRestart: depth %d, batches %d, ops %d; want the 3 records sealed in one batch", stats.QueueDepth, stats.BatchesSealed, stats.OpsSealed)
	}
	if !stats.Synchronous || rt.store.TelemetryBatchingEnabled() {
		t.Fatalf("synchronous mode = %v, enabled = %v: new telemetry must take the acknowledged path while the restart is pending", stats.Synchronous, rt.store.TelemetryBatchingEnabled())
	}
	marker, planned := ConsumePlannedRestartMarker(rt.cfg.StorePath, time.Now())
	if !planned || marker.TelemetryDrainFailed {
		t.Fatalf("marker after a clean drain: planned=%v %+v", planned, marker)
	}
	// The apply returned and this process is still alive: the restart did not
	// happen, so records queue again.
	done(nil)
	if !rt.store.TelemetryBatchingEnabled() {
		t.Fatal("synchronous mode outlived an apply that returned")
	}
}

// A drain that fails or times out never blocks the restart: the marker is
// still written, the failure is recorded on it, and the resume record says the
// trace may have a gap.
func TestDrainFailureStillWritesMarkerAndFlagsResume(t *testing.T) {
	rt := telemetryRuntime(t, "computer-telemetry-drainfail")
	if err := rt.store.ConfigureTelemetryBatching(store.TelemetryBatchConfig{Enabled: true, Window: 2 * time.Second}); err != nil {
		t.Fatal(err)
	}
	queueTelemetry(t, rt, 2)
	rt.telemetryDrainDeadline = time.Nanosecond
	if _, err := rt.markPlannedRestart("platform update", "u-2"); err != nil {
		t.Fatal(err)
	}
	marker, planned := ConsumePlannedRestartMarker(rt.cfg.StorePath, time.Now())
	if !planned {
		t.Fatal("a failed drain stopped the planned-restart marker from being written")
	}
	if !marker.TelemetryDrainFailed {
		t.Fatalf("marker %+v does not record the failed drain", marker)
	}
	payload := passivatedOnRestartPayload(marker)
	if !strings.Contains(string(payload), telemetryMissingWindowNote) {
		t.Fatalf("resume record %s does not carry the missing-window sentence", payload)
	}
	// The markerful boot after a clean drain is unchanged.
	if got := string(passivatedOnRestartPayload(PlannedRestart{})); got != `{"recovery":"passivated_on_restart"}` {
		t.Fatalf("clean resume payload changed: %s", got)
	}
}

// P1-F16: the crash boot says what was lost when batching is on, and its
// record is byte-for-byte what it was when batching is off.
func TestCrashCloseRecordNamesTheMissingWindow(t *testing.T) {
	if got := string(crashClosePayload(false)); got != `{"fate":"interrupted_by_restart"}` {
		t.Fatalf("flag-off crash-close payload changed: %s", got)
	}
	on := crashClosePayload(true)
	var decoded map[string]string
	if err := json.Unmarshal(on, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["fate"] != "interrupted_by_restart" || decoded["telemetry"] != telemetryMissingWindowNote {
		t.Fatalf("crash-close payload with batching on = %s", on)
	}
}

// P1-F14 / flag source: the owner's computer setting turns batching on and off
// through the product API; off drains what is queued.
func TestComputerSettingTurnsBatchingOnAndOff(t *testing.T) {
	const computerID = "computer-telemetry-setting"
	rt := telemetryRuntime(t, computerID)
	handler := &APIHandler{rt: rt}
	post := func(value bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/computer-settings", strings.NewReader(fmt.Sprintf(`{"key":"telemetry.batch","value":%v}`, value)))
		req.Header.Set("X-Authenticated-User", "owner-telemetry")
		rec := httptest.NewRecorder()
		handler.HandleComputerSettings(rec, req)
		return rec
	}
	if rt.store.TelemetryBatchingEnabled() || rt.store.TelemetryBatchingConfigured() {
		t.Fatal("batching is on with no flag and no setting: the default must be off")
	}
	if rec := post(true); rec.Code != http.StatusOK {
		t.Fatalf("set on = %d %s", rec.Code, rec.Body)
	}
	if !rt.store.TelemetryBatchingEnabled() {
		t.Fatal("the owner's setting did not reach the store")
	}
	queueTelemetry(t, rt, 2)
	if rt.store.TelemetryStats().Batching.QueueDepth != 2 {
		t.Fatal("telemetry was not queued after the setting turned batching on")
	}
	if rec := post(false); rec.Code != http.StatusOK {
		t.Fatalf("set off = %d %s", rec.Code, rec.Body)
	}
	stats := rt.store.TelemetryStats().Batching
	if rt.store.TelemetryBatchingEnabled() || stats.QueueDepth != 0 || stats.OpsSealed != 2 {
		t.Fatalf("after turning it off: enabled=%v depth=%d sealed=%d; want off, drained, 2 ops sealed", rt.store.TelemetryBatchingEnabled(), stats.QueueDepth, stats.OpsSealed)
	}
}

// At boot the persisted setting (or the env flag) configures the store before
// the first event of the process, so the sequence reseed gap applies.
func TestBootConfiguresBatchingFromSettingAndEnv(t *testing.T) {
	const computerID = "computer-telemetry-boot"
	t.Setenv("CHOIR_OWNER_ID", "owner-telemetry")
	rt := telemetryRuntime(t, computerID)
	rt.configureTelemetryBatching(context.Background())
	if rt.store.TelemetryBatchingConfigured() {
		t.Fatal("batching configured with neither the env flag nor the setting")
	}
	if err := rt.store.PutComputerSetting(context.Background(), store.ComputerSetting{OwnerID: "owner-telemetry", ComputerID: computerID, Key: store.SettingTelemetryBatch, Value: true}); err != nil {
		t.Fatal(err)
	}
	rt.configureTelemetryBatching(context.Background())
	if !rt.store.TelemetryBatchingEnabled() {
		t.Fatal("the persisted setting did not enable batching at boot")
	}

	rt2 := telemetryRuntime(t, "computer-telemetry-boot-env")
	t.Setenv("CHOIR_TELEMETRY_BATCH", "on")
	t.Setenv("CHOIR_TELEMETRY_BATCH_WINDOW_MS", "500")
	rt2.configureTelemetryBatching(context.Background())
	stats := rt2.store.TelemetryStats().Batching
	if !stats.Enabled || stats.WindowMS != 500 {
		t.Fatalf("env flag: enabled=%v window=%dms, want on at 500ms", stats.Enabled, stats.WindowMS)
	}
}

// P1-F17: the counters are readable on the product observability surface.
func TestObservabilityReportsTelemetryBatching(t *testing.T) {
	rt := telemetryRuntime(t, "computer-telemetry-obs")
	if err := rt.store.ConfigureTelemetryBatching(store.TelemetryBatchConfig{Enabled: true, Window: time.Second}); err != nil {
		t.Fatal(err)
	}
	queueTelemetry(t, rt, 4)
	if err := rt.store.DrainTelemetry(context.Background()); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/runtime/observability", nil)
	req.Header.Set("X-Authenticated-User", "owner-telemetry")
	rec := httptest.NewRecorder()
	(&APIHandler{rt: rt}).HandleGuestObservability(rec, req)
	var obs struct {
		TelemetryBatching *store.TelemetryStats `json:"telemetry_batching"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &obs); err != nil || obs.TelemetryBatching == nil {
		t.Fatalf("observability = %d %s", rec.Code, rec.Body.String())
	}
	b := obs.TelemetryBatching
	if !b.Batching.Enabled || b.Batching.BatchesSealed != 1 || b.Batching.OpsSealed != 4 || b.Batching.MeanOpsPerBatch != 4 {
		t.Fatalf("telemetry_batching.batching = %+v", b.Batching)
	}
	// 4 tool.result plus the boot watermark (an acknowledged runtime.health).
	if b.EventsAppendedByKind["tool.result"] != 4 || b.EventsAppendedBatchable != 4 || b.EventsAppendedAcknowledged != 1 || b.BatchableShare != 0.8 {
		t.Fatalf("telemetry_batching phase 0 counters = %+v", b)
	}
	if b.TapeEventsByAuthority["authority:vm-local-projection-telemetry"] != 1 {
		t.Fatalf("tape_events_by_authority = %v", b.TapeEventsByAuthority)
	}
}

// ARCH-13 (docs/problems/guest-checkpoint-on-the-write-path-2026-10-21.md):
// the owner's setting store.debounced_checkpoint moves the guest's Dolt
// checkpoint off the write path, and turning it off is the rollback; the
// counters are readable on the observability surface in either mode.
func TestComputerSettingTurnsDebouncedCheckpointOnAndOff(t *testing.T) {
	const computerID = "computer-checkpoint-setting"
	rt := telemetryRuntime(t, computerID)
	handler := &APIHandler{rt: rt}
	post := func(value bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/computer-settings", strings.NewReader(fmt.Sprintf(`{"key":"store.debounced_checkpoint","value":%v}`, value)))
		req.Header.Set("X-Authenticated-User", "owner-telemetry")
		rec := httptest.NewRecorder()
		handler.HandleComputerSettings(rec, req)
		return rec
	}
	if rt.store.CheckpointStats().Mode != store.CheckpointModeCount {
		t.Fatal("debounced checkpoint is on with no flag and no setting: the default must be count mode")
	}
	if rec := post(true); rec.Code != http.StatusOK {
		t.Fatalf("set on = %d %s", rec.Code, rec.Body)
	}
	if rt.store.CheckpointStats().Mode != store.CheckpointModeDebounced {
		t.Fatal("the owner's setting did not reach the store")
	}
	if rec := post(false); rec.Code != http.StatusOK {
		t.Fatalf("set off = %d %s", rec.Code, rec.Body)
	}
	if rt.store.CheckpointStats().Mode != store.CheckpointModeCount {
		t.Fatal("turning the setting off did not return to count mode")
	}
}

func TestBootConfiguresDebouncedCheckpointFromSettingAndEnv(t *testing.T) {
	const computerID = "computer-checkpoint-boot"
	t.Setenv("CHOIR_OWNER_ID", "owner-telemetry")
	t.Setenv("CHOIR_GUEST_CHECKPOINT", "")
	rt := telemetryRuntime(t, computerID)
	rt.configureGuestCheckpoint(context.Background())
	if rt.store.CheckpointStats().Mode != store.CheckpointModeCount {
		t.Fatal("debounced with neither the env flag nor the setting")
	}
	if err := rt.store.PutComputerSetting(context.Background(), store.ComputerSetting{OwnerID: "owner-telemetry", ComputerID: computerID, Key: store.SettingDebouncedCheckpoint, Value: true}); err != nil {
		t.Fatal(err)
	}
	rt.configureGuestCheckpoint(context.Background())
	if rt.store.CheckpointStats().Mode != store.CheckpointModeDebounced {
		t.Fatal("the persisted setting did not turn debounced mode on at boot")
	}

	rt2 := telemetryRuntime(t, "computer-checkpoint-boot-env")
	t.Setenv("CHOIR_GUEST_CHECKPOINT", "debounced")
	t.Setenv("CHOIR_GUEST_CHECKPOINT_DEBOUNCE", "30s")
	rt2.configureGuestCheckpoint(context.Background())
	if stats := rt2.store.CheckpointStats(); stats.Mode != store.CheckpointModeDebounced || stats.DebounceMS != 30000 {
		t.Fatalf("env flag: %+v, want debounced at 30s", stats)
	}
}

func TestObservabilityReportsGuestCheckpoint(t *testing.T) {
	rt := telemetryRuntime(t, "computer-checkpoint-obs")
	req := httptest.NewRequest(http.MethodGet, "/api/runtime/observability", nil)
	req.Header.Set("X-Authenticated-User", "owner-telemetry")
	rec := httptest.NewRecorder()
	(&APIHandler{rt: rt}).HandleGuestObservability(rec, req)
	var obs struct {
		GuestCheckpoint *store.CheckpointStats `json:"guest_checkpoint"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &obs); err != nil || obs.GuestCheckpoint == nil {
		t.Fatalf("observability = %d %s", rec.Code, rec.Body.String())
	}
	if obs.GuestCheckpoint.Mode != store.CheckpointModeCount {
		t.Fatalf("guest_checkpoint = %+v", obs.GuestCheckpoint)
	}
}
