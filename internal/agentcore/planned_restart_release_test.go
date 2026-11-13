package agentcore

import (
	"errors"
	"testing"
	"time"
)

// The owner's two rules at the function level (new symbols; the behavioural
// tests are in planned_restart_marker_rules_test.go).

// Rule 1: clear is reason-scoped, and "removed" is never confused with "kept".
func TestClearIfReasonSaysRemovedOrKeptNeverBoth(t *testing.T) {
	rt, _ := testRuntime(t)
	path := rt.cfg.StorePath
	t.Cleanup(func() { _ = ClearPlannedRestartMarker(path) })

	cleared, kept, err := clearPlannedRestartMarkerIfReason(path, PlannedRestartPlatformRefresh)
	if err != nil || cleared || kept != "" {
		t.Fatalf("no marker: cleared=%v kept=%q err=%v, want nothing to report", cleared, kept, err)
	}
	if err := WritePlannedRestartMarker(path, PlannedRestart{Reason: "platform_update", Target: "t"}); err != nil {
		t.Fatal(err)
	}
	cleared, kept, err = clearPlannedRestartMarkerIfReason(path, PlannedRestartPlatformRefresh)
	if err != nil || cleared || kept != "platform_update" {
		t.Fatalf("a guest marker: cleared=%v kept=%q err=%v, want kept platform_update", cleared, kept, err)
	}
	cleared, kept, err = clearPlannedRestartMarkerIfReason(path, "platform_update")
	if err != nil || !cleared || kept != "" {
		t.Fatalf("its own marker: cleared=%v kept=%q err=%v, want cleared", cleared, kept, err)
	}
}

// Rule 1: a host write never lands over a guest marker.
func TestHostWriteRefusesToOverwriteAGuestMarker(t *testing.T) {
	rt, _ := testRuntime(t)
	path := rt.cfg.StorePath
	t.Cleanup(func() { _ = ClearPlannedRestartMarker(path) })
	if err := WritePlannedRestartMarker(path, PlannedRestart{Reason: PlannedRestartSelfDevelopmentApply, Target: "op"}); err != nil {
		t.Fatal(err)
	}
	err := WritePlannedRestartMarker(path, PlannedRestart{Reason: PlannedRestartPlatformRefresh, Target: "host"})
	if !errors.Is(err, errGuestRestartPending) {
		t.Fatalf("host write over a guest marker = %v, want errGuestRestartPending", err)
	}
	// The other way is allowed: the guest's restart is the one that happens,
	// and a planned boot follows either way.
	if err := WritePlannedRestartMarker(path, PlannedRestart{Reason: PlannedRestartPlatformRefresh, Target: "host"}); err == nil {
		t.Fatal("the host write should still be refused")
	}
	if _, _, err := clearPlannedRestartMarkerIfReason(path, PlannedRestartSelfDevelopmentApply); err != nil {
		t.Fatal(err)
	}
	if err := WritePlannedRestartMarker(path, PlannedRestart{Reason: PlannedRestartPlatformRefresh, Target: "host"}); err != nil {
		t.Fatal(err)
	}
	if err := WritePlannedRestartMarker(path, PlannedRestart{Reason: "platform_update", Target: "guest"}); err != nil {
		t.Fatalf("a guest write over the host's marker should land (it cannot produce a crash boot): %v", err)
	}
}

// Rule 2: with a marker on disk the one release function releases nothing;
// without one it releases exactly what it was asked.
func TestReleaseRestartProtectionsReleasesNothingWhileAMarkerIsOnDisk(t *testing.T) {
	f := newPauseFixture(t)
	path := f.rt.cfg.StorePath
	if code, body := f.mark(t, "?allow_busy=1"); code != 409 {
		t.Fatalf("phase 1: %d %s", code, body)
	}
	f.rt.waitCapsulePauses()
	f.rt.noteRefreshAsked("761d94c5", time.Now())
	f.rt.snapshots.blockUntil = time.Now().Add(time.Hour)
	all := relBlock | relRefreshHold | relTelemetry | relThaw

	if err := WritePlannedRestartMarker(path, PlannedRestart{Reason: PlannedRestartPlatformRefresh, Target: "t"}); err != nil {
		t.Fatal(err)
	}
	if f.rt.releaseRestartProtections(path, all, false) {
		t.Fatal("released while a marker is on disk")
	}
	if !f.rt.refreshHoldsNewWork(time.Now()) || !time.Now().Before(f.rt.snapshots.blockUntil) || !f.fake.IsPaused(f.capsuleID) {
		t.Fatal("a protection was released under a marker")
	}

	if _, _, err := clearPlannedRestartMarkerIfReason(path, PlannedRestartPlatformRefresh); err != nil {
		t.Fatal(err)
	}
	if !f.rt.releaseRestartProtections(path, relBlock|relRefreshHold, false) {
		t.Fatal("nothing released with no marker")
	}
	if f.rt.refreshHoldsNewWork(time.Now()) || time.Now().Before(f.rt.snapshots.blockUntil) {
		t.Fatal("the block or the hold was not released")
	}
	if !f.fake.IsPaused(f.capsuleID) {
		t.Fatal("a protection that was not asked for was released")
	}
	f.rt.releaseRestartProtections(path, relThaw, false)
	pauseEventually(t, "the capsule to thaw", func() bool { return !f.fake.IsPaused(f.capsuleID) })
}
