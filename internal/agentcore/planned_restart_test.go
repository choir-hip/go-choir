package agentcore

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// Failure modes pinned: a marker survives its consumption (a resume that
// crashes would resume again: a loop); a stale, corrupt or missing marker
// reads as planned; a failed apply leaves a marker behind.
func TestPlannedRestartMarkerIsConsumedOnce(t *testing.T) {
	store := filepath.Join(t.TempDir(), "state")
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	if err := WritePlannedRestartMarker(store, PlannedRestart{Reason: "platform_update", Target: "u-1", RequestedAt: now.Add(-time.Minute)}); err != nil {
		t.Fatal(err)
	}
	marker, planned := ConsumePlannedRestartMarker(store, now)
	if !planned || marker.Reason != "platform_update" || marker.Target != "u-1" {
		t.Fatalf("fresh marker: planned=%v marker=%+v", planned, marker)
	}
	if _, planned := ConsumePlannedRestartMarker(store, now); planned {
		t.Fatal("marker consumed twice: a crashing resume would loop")
	}
}

func TestPlannedRestartMarkerStaleOrCorruptIsACrash(t *testing.T) {
	store := filepath.Join(t.TempDir(), "state")
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	if _, planned := ConsumePlannedRestartMarker(store, now); planned {
		t.Fatal("missing marker read as planned")
	}
	if err := WritePlannedRestartMarker(store, PlannedRestart{Reason: "old", RequestedAt: now.Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if _, planned := ConsumePlannedRestartMarker(store, now); planned {
		t.Fatal("stale marker read as planned")
	}
	if _, err := os.Stat(PlannedRestartMarkerPath(store)); !os.IsNotExist(err) {
		t.Fatalf("stale marker not removed: %v", err)
	}
	if err := os.WriteFile(PlannedRestartMarkerPath(store), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, planned := ConsumePlannedRestartMarker(store, now); planned {
		t.Fatal("corrupt marker read as planned")
	}
	if err := WritePlannedRestartMarker(store, PlannedRestart{Reason: "future", RequestedAt: now.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if _, planned := ConsumePlannedRestartMarker(store, now); planned {
		t.Fatal("future-dated marker read as planned")
	}
}

// The updater answers only a caller that is still alive: either the apply
// restarted this guest already (a resume) or it never will. Either way the
// marker is stale once Apply returns (L1a-3a).
func TestMarkPlannedRestartClearsWhenApplyReturns(t *testing.T) {
	store := filepath.Join(t.TempDir(), "state")
	rt := &Runtime{}
	rt.cfg.StorePath = store
	done, _ := rt.markPlannedRestart("self_development_apply", "op-1")
	if _, err := os.Stat(PlannedRestartMarkerPath(store)); err != nil {
		t.Fatalf("marker not written before apply: %v", err)
	}
	done(os.ErrClosed)
	if _, err := os.Stat(PlannedRestartMarkerPath(store)); !os.IsNotExist(err) {
		t.Fatalf("failed apply left a marker: %v", err)
	}
	done, _ = rt.markPlannedRestart("self_development_apply", "op-2")
	done(nil)
	if _, err := os.Stat(PlannedRestartMarkerPath(store)); !os.IsNotExist(err) {
		t.Fatalf("an apply that returned success left a marker: %v", err)
	}
}

// D7 (Cut 1): a planned-restart marker resumes only within one work-state
// schema epoch. A marker without the field, or from another epoch, was
// written by a binary whose work state this one does not read, so its boot
// is a crash. Ways this could fail, written first: the writer omits the
// epoch, so every planned restart becomes a crash; a pre-cut marker resumes
// runs into the records-only schema.
func TestPlannedRestartMarkerResumesOnlyWithinItsWorkStateEpoch(t *testing.T) {
	dir := t.TempDir()
	store := filepath.Join(dir, "store.db")
	now := time.Now().UTC()
	if err := WritePlannedRestartMarker(store, PlannedRestart{Reason: "platform_update", RequestedAt: now}); err != nil {
		t.Fatal(err)
	}
	if marker, planned := ConsumePlannedRestartMarker(store, now); !planned || marker.WorkStateEpoch != CurrentWorkStateEpoch {
		t.Fatalf("a marker this binary wrote = %+v planned=%v, want planned with epoch %d", marker, planned, CurrentWorkStateEpoch)
	}
	for _, raw := range []string{
		`{"reason":"platform_update","requested_at":"` + now.Format(time.RFC3339Nano) + `"}`,
		`{"reason":"platform_update","requested_at":"` + now.Format(time.RFC3339Nano) + `","work_state_epoch":` + strconv.Itoa(CurrentWorkStateEpoch+1) + `}`,
	} {
		if err := os.WriteFile(PlannedRestartMarkerPath(store), []byte(raw), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, planned := ConsumePlannedRestartMarker(store, now); planned {
			t.Fatalf("a marker from another work-state epoch resumed: %s", raw)
		}
	}
}

// The marker's 15-minute age is taken at process start, not at consumption
// (docs/problems/always-busy-computer-never-updates-2026-10-15.md). The boot
// runs Dolt GC (journals up to 4 GiB), opens the store and exchanges
// credentials before it consumes the marker; a large store's slow boot must
// not turn a planned refresh into a crash. Ways this could fail, written
// first: the boot passes the consumption instant, so a 2-minute kill and boot
// plus a 14-minute GC reads as stale; a marker already stale when the process
// started reads as planned.
func TestPlannedRestartMarkerAgeIsTakenAtProcessStart(t *testing.T) {
	store := filepath.Join(t.TempDir(), "state")
	written := time.Date(2026, 10, 15, 3, 0, 0, 0, time.UTC)
	processStarted := written.Add(2 * time.Minute) // kill and VM boot
	if err := WritePlannedRestartMarker(store, PlannedRestart{Reason: PlannedRestartPlatformRefresh, Target: "c0ffee", RequestedAt: written}); err != nil {
		t.Fatal(err)
	}
	// Consumed after a 14-minute boot GC: 16 minutes after the write, but 2
	// minutes old when the process started.
	if marker, planned := ConsumePlannedRestartMarker(store, processStarted); !planned || marker.Target != "c0ffee" {
		t.Fatalf("slow boot of a fresh refresh: planned=%v marker=%+v, want planned", planned, marker)
	}

	if err := WritePlannedRestartMarker(store, PlannedRestart{Reason: PlannedRestartPlatformRefresh, Target: "c0ffee", RequestedAt: written}); err != nil {
		t.Fatal(err)
	}
	if _, planned := ConsumePlannedRestartMarker(store, written.Add(plannedRestartMaxAge+time.Minute)); planned {
		t.Fatal("a marker already stale at process start read as planned")
	}
}
