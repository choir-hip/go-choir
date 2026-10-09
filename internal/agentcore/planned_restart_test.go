package agentcore

import (
	"os"
	"path/filepath"
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

func TestMarkPlannedRestartClearsAfterFailedApply(t *testing.T) {
	store := filepath.Join(t.TempDir(), "state")
	rt := &Runtime{}
	rt.cfg.StorePath = store
	done := rt.markPlannedRestart("self_development_apply", "op-1")
	if _, err := os.Stat(PlannedRestartMarkerPath(store)); err != nil {
		t.Fatalf("marker not written before apply: %v", err)
	}
	done(os.ErrClosed)
	if _, err := os.Stat(PlannedRestartMarkerPath(store)); !os.IsNotExist(err) {
		t.Fatalf("failed apply left a marker: %v", err)
	}
	done = rt.markPlannedRestart("self_development_apply", "op-2")
	done(nil)
	if _, err := os.Stat(PlannedRestartMarkerPath(store)); err != nil {
		t.Fatalf("successful apply lost its marker: %v", err)
	}
}
