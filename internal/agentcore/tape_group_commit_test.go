package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// tape.group_commit at the runtime seam
// (docs/design/tape-group-commit-2026-10-31.md section 5). Ways it can fail:
//   - the owner's setting does not reach the store, or off does not return
//     writes to the direct path at once;
//   - the stored setting is not read at boot;
//   - the default is on for every computer before the ring reaches it;
//   - the counters cannot be read on the product observability surface.
func TestTapeGroupCommitSettingBootAndAPI(t *testing.T) {
	if store.KnownComputerSettings[store.SettingTapeGroupCommit] {
		t.Fatal("tape.group_commit defaults on: it rings from d@e by stored setting")
	}
	const computerID = "computer-group-commit-setting"
	t.Setenv("CHOIR_OWNER_ID", "owner-group")
	rt := telemetryRuntime(t, computerID)
	rt.configureTapeGroupCommit(context.Background())
	if rt.store.TapeGroupCommitOn() {
		t.Fatal("on at boot with no stored setting")
	}
	handler := &APIHandler{rt: rt}
	post := func(value bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/computer-settings", strings.NewReader(fmt.Sprintf(`{"key":"tape.group_commit","value":%v}`, value)))
		req.Header.Set("X-Authenticated-User", "owner-group")
		rec := httptest.NewRecorder()
		handler.HandleComputerSettings(rec, req)
		return rec
	}
	if rec := post(true); rec.Code != http.StatusOK {
		t.Fatalf("set on = %d %s", rec.Code, rec.Body)
	}
	if !rt.store.TapeGroupCommitOn() {
		t.Fatal("the owner's setting did not reach the store")
	}
	queueTelemetry(t, rt, 1) // a write under group commit still lands

	// A restart reads the stored setting.
	rt.store.SetTapeGroupCommit(false)
	rt.configureTapeGroupCommit(context.Background())
	if !rt.store.TapeGroupCommitOn() {
		t.Fatal("the stored setting was not applied at boot")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/runtime/observability", nil)
	req.Header.Set("X-Authenticated-User", "owner-group")
	rec := httptest.NewRecorder()
	handler.HandleGuestObservability(rec, req)
	var obs struct {
		TelemetryBatching *store.TelemetryStats `json:"telemetry_batching"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &obs); err != nil || obs.TelemetryBatching == nil {
		t.Fatalf("observability = %d %s", rec.Code, rec.Body.String())
	}
	if !obs.TelemetryBatching.GroupCommit.Enabled || obs.TelemetryBatching.GroupCommit.Members == 0 {
		t.Fatalf("group_commit on the observability surface = %+v", obs.TelemetryBatching.GroupCommit)
	}

	if rec := post(false); rec.Code != http.StatusOK {
		t.Fatalf("set off = %d %s", rec.Code, rec.Body)
	}
	if rt.store.TapeGroupCommitOn() {
		t.Fatal("off did not reach the store")
	}
}

// The run-memory window at the runtime seam (design section 9):
//   - a crash-closed run does not say its memory may be missing;
//   - a planned boot whose drain failed resumes a run on a memory log that
//     may have lost its tail;
//   - a run's opening messages are not one write.
func TestRunMemoryWindowRuntimeRules(t *testing.T) {
	var on map[string]string
	if err := json.Unmarshal(crashClosePayload(true, true), &on); err != nil || on["telemetry"] != runMemoryMissingWindowNote || !strings.Contains(on["telemetry"], "run memory") {
		t.Fatalf("crash-close payload with the window on = %v (%v)", on, err)
	}

	rt := telemetryRuntime(t, "computer-window-boot")
	// The window state is the marker's, stamped at drain time; the boot-time
	// setting does not decide (repair B1).
	closedAtDrain, openAtDrain := false, true
	rt.bootRestart, rt.bootRestartPlanned = PlannedRestart{Reason: "platform update", TelemetryDrainFailed: true, RunMemoryWindowOpen: &closedAtDrain}, true
	rt.store.SetTapeGroupCommit(true)
	if !rt.bootResumesWork() {
		t.Fatal("with the window closed at drain time, a failed telemetry drain must still resume (trace loss only)")
	}
	rt.store.SetTapeGroupCommit(false)
	rt.bootRestart.RunMemoryWindowOpen = &openAtDrain
	if rt.bootResumesWork() {
		t.Fatal("a planned boot whose drain failed with the window open resumed work on a possibly partial memory log")
	}
	rt.store.SetTapeGroupCommit(true)
	rt.bootRestart.TelemetryDrainFailed = false
	if !rt.bootResumesWork() {
		t.Fatal("a drained planned boot did not resume")
	}
	rt.bootRestartPlanned = false
	if rt.bootResumesWork() {
		t.Fatal("a crash boot resumed work")
	}

	rec := &types.RunRecord{RunID: "run-opening", OwnerID: "owner-telemetry", AgentID: "agent-opening"}
	m := newRunMemoryManager(rt.store, rec, provideriface.Config{}, nil)
	before := rt.store.TelemetryStats().TapeEvents
	msgs := []json.RawMessage{json.RawMessage(`{"role":"system","content":"a"}`), json.RawMessage(`{"role":"user","content":"b"}`), json.RawMessage(`{"role":"user","content":"c"}`)}
	if err := m.appendMessages(context.Background(), msgs); err != nil {
		t.Fatal(err)
	}
	if n := rt.store.TelemetryStats().TapeEvents - before; n != 1 {
		t.Fatalf("three opening messages took %d tape events, want 1", n)
	}
	entries, err := rt.store.ListRunMemoryEntries(context.Background(), "owner-telemetry", "run-opening")
	if err != nil || len(entries) != 3 || entries[2].Seq != 3 {
		t.Fatalf("opening entries = %+v (%v)", entries, err)
	}
}
