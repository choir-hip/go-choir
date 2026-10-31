package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Repair B1: the failed-drain crash-close follows the window state recorded in
// the marker at drain time, not the setting the new boot reads.
func TestPlannedBootGatesOnMarkerWindowNotBootSetting(t *testing.T) {
	rt := telemetryRuntime(t, "computer-marker-window")
	open, closed := true, false
	rt.store.SetTapeGroupCommit(false) // the new boot reads the setting off
	rt.bootRestartPlanned = true

	rt.bootRestart = PlannedRestart{Reason: "platform update", TelemetryDrainFailed: true, RunMemoryWindowOpen: &open}
	if rt.bootResumesWork() {
		t.Fatal("drain failed with the window open, setting off at boot: runs resumed on a possibly holed log")
	}
	if !rt.crashCloseNamesRunMemory() {
		t.Fatal("the crash-close note does not name run memory though the window was open at drain time")
	}
	rt.bootRestart = PlannedRestart{Reason: "platform update", TelemetryDrainFailed: true, RunMemoryWindowOpen: &closed}
	if !rt.bootResumesWork() {
		t.Fatal("drain failed with the window closed must still resume: trace loss only")
	}
	rt.store.SetTapeGroupCommit(true)
	if !rt.bootResumesWork() {
		t.Fatal("the boot-time setting must not decide: the window was closed at drain time")
	}
	rt.store.SetTapeGroupCommit(false)
	rt.bootRestart = PlannedRestart{Reason: "platform update", TelemetryDrainFailed: true} // old marker
	if rt.bootResumesWork() {
		t.Fatal("an old marker with a failed drain must close conservatively")
	}
	rt.bootRestart = PlannedRestart{Reason: "platform update"}
	if !rt.bootResumesWork() {
		t.Fatal("a drained planned boot did not resume")
	}
}

// Repair B1: the marker stamps the window state when written.
func TestPlannedRestartMarkerRecordsRunMemoryWindow(t *testing.T) {
	rt := telemetryRuntime(t, "computer-marker-stamp")
	rt.cfg.StorePath = filepath.Join(t.TempDir(), "store.db")
	rt.store.SetTapeGroupCommit(true)
	if _, err := rt.writeHostPlannedRestartMarker(rt.cfg.StorePath, PlannedRestart{Reason: "platform update"}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(PlannedRestartMarkerPath(rt.cfg.StorePath))
	if err != nil {
		t.Fatal(err)
	}
	var m PlannedRestart
	if err := json.Unmarshal(raw, &m); err != nil || m.RunMemoryWindowOpen == nil || !*m.RunMemoryWindowOpen {
		t.Fatalf("marker = %s (%v), want run_memory_window_open true", raw, err)
	}
}

// Repair B1 end to end: pre-boot runs close, they are not parked, when the
// marker says the window was open and the new boot's setting is off.
func TestFailedDrainWithOpenWindowClosesRunsAtBoot(t *testing.T) {
	t.Setenv("CHOIR_OWNER_ID", "owner-telemetry")
	rt := telemetryRuntime(t, "computer-boot-close")
	open := true
	rt.bus = events.NewEventBus()
	rt.store.SetTapeGroupCommit(false)
	rt.bootRestart, rt.bootRestartPlanned = PlannedRestart{Reason: "platform update", TelemetryDrainFailed: true, RunMemoryWindowOpen: &open}, true
	ctx := context.Background()
	now := time.Now().UTC().Add(-time.Hour)
	rec := types.RunRecord{RunID: "run-boot-close", AgentID: "research:boot-close", AgentProfile: agentprofile.Research,
		AgentRole: agentprofile.Research, OwnerID: "owner-telemetry", ComputerID: rt.cfg.ComputerID, State: types.RunRunning,
		CreatedAt: now, UpdatedAt: now}
	if err := rt.store.CreateRun(ctx, rec); err != nil {
		t.Fatal(err)
	}
	if rt.bootResumesWork() {
		rt.passivateInterruptedActivations(ctx)
	} else {
		rt.closePreBootRunsAfterCrash(ctx)
	}
	got, err := rt.store.GetRun(ctx, rec.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != types.RunCancelled || got.Error != runFateInterruptedByRestart {
		t.Fatalf("run after boot = %s / %q, want cancelled / %s", got.State, got.Error, runFateInterruptedByRestart)
	}

	// Not seeded: the next run of the same actor starts from the latest
	// completed or passivated run with memory (LatestActorRunMemoryEntries
	// filters on state in {completed, passivated}); the crash-closed run is
	// cancelled with fate interrupted_by_restart, so its entries are excluded.
	if _, err := rt.store.AppendRunMemoryEntry(ctx, types.RunMemoryEntry{EntryID: "closed-1", RunID: rec.RunID, OwnerID: rec.OwnerID,
		AgentID: rec.AgentID, Kind: types.RunMemoryEntryMessage, Role: "user", Message: json.RawMessage(`{"role":"user","content":"x"}`)}); err != nil {
		t.Fatal(err)
	}
	sourceRunID, entries, err := rt.store.LatestActorRunMemoryEntries(ctx, rec.OwnerID, rec.ComputerID, rec.AgentID, "run-next")
	if (err != nil && !errors.Is(err, store.ErrNotFound)) || sourceRunID != "" || len(entries) != 0 {
		t.Fatalf("next run seeded from the crash-closed run: source %q entries %d (%v)", sourceRunID, len(entries), err)
	}
}

// Repair note 2: a run whose run memory lost an entry after its last append
// (the damage mark is in memory only) must not complete as a clean run: the
// completion path asks runMemoryLossErr and fails the run, which puts the fate
// on the tape instead of letting the next run seed from a log with a hole.
func TestRunMemoryLossErrNamesADamagedRun(t *testing.T) {
	rt := telemetryRuntime(t, "computer-damaged-run")
	rec := &types.RunRecord{RunID: "run-damaged", OwnerID: "owner-telemetry"}
	if err := rt.runMemoryLossErr(rec); err != nil {
		t.Fatalf("a whole run reported lost: %v", err)
	}
	rt.store.MarkRunMemoryDamaged(rec.RunID)
	err := rt.runMemoryLossErr(rec)
	if err == nil || !strings.Contains(err.Error(), "run memory") {
		t.Fatalf("a damaged run reported %v, want a run-memory error", err)
	}
}
