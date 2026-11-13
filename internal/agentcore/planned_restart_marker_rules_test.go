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
)

// Review round 4: the owner enforces what a marker means, not only who may
// touch the file. Failure list, written first.
//
// Rule 1, the marker belongs to whoever wrote it:
//
//  K1. the host's POST overwrites a marker the guest wrote for itself
//      (restore restart sleeping 2 s, ApplyPlatformUpdate) and answers 200;
//  K2. the guest's own cleanup (clearRestoreRestartMarker, the func
//      markPlannedRestart returns) deletes whatever marker is on disk, the
//      host's included, so the kill lands as a crash boot;
//  K3. a marker without a reason is written, and the withdrawal answers
//      "cleared" for one it left on disk.
//
// Rule 2, nothing that protects a coming kill is released while a marker is on
// disk:
//
//  K4. a caller that goes away while the unwind wait runs leaves the refresh
//      hold set although nothing was written (no marker);
//  K5. the caller-gone refusal thaws a capsule under an earlier request's
//      marker (the owner test asserts it).

func writeRawMarker(t *testing.T, rt *Runtime, m PlannedRestart) {
	t.Helper()
	m.WorkStateEpoch = CurrentWorkStateEpoch
	if m.RequestedAt.IsZero() {
		m.RequestedAt = time.Now().UTC()
	}
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(PlannedRestartMarkerPath(rt.cfg.StorePath), raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

func markerReason(t *testing.T, rt *Runtime) string {
	t.Helper()
	raw, err := os.ReadFile(PlannedRestartMarkerPath(rt.cfg.StorePath))
	if err != nil {
		return ""
	}
	var m PlannedRestart
	_ = json.Unmarshal(raw, &m)
	return m.Reason
}

// K1: Gemini scenarios 1 and 2. A guest restart is already coming; the host's
// mark is refused, not written over it.
func TestHostMarkIsRefusedWhileAGuestMarkerIsOnDisk(t *testing.T) {
	for _, reason := range []string{PlannedRestartSelfDevelopmentRestore, "platform_update", PlannedRestartSelfDevelopmentApply} {
		t.Run(reason, func(t *testing.T) {
			rt, _ := testRuntime(t)
			t.Cleanup(func() { _ = ClearPlannedRestartMarker(rt.cfg.StorePath) })
			writeRawMarker(t, rt, PlannedRestart{Reason: reason, Target: "guest"})
			rt.runningMu.Lock()
			rt.running["loop-busy"] = func() {}
			rt.runningMu.Unlock()
			code, body := postPlannedRestartURL(t, &APIHandler{rt: rt}, "/internal/runtime/planned-restart?allow_busy=1", `{"target":"761d94c5"}`)
			if code != http.StatusConflict || !strings.Contains(body, "guest_restart_pending") || !strings.Contains(body, `"holds_new_work":true`) {
				t.Fatalf("host mark over a %q marker: %d %s, want 409 guest_restart_pending holding new work", reason, code, body)
			}
			if got := markerReason(t, rt); got != reason {
				t.Fatalf("the guest's marker was overwritten: reason now %q, want %q", got, reason)
			}
		})
	}
}

// K2: the guest's cleanup clears its own reason only.
func TestGuestCleanupClearsOnlyItsOwnMarker(t *testing.T) {
	rt, _ := testRuntime(t)
	t.Cleanup(func() { _ = ClearPlannedRestartMarker(rt.cfg.StorePath) })

	// The restore restart's cleanup, over the host's marker.
	writeRawMarker(t, rt, PlannedRestart{Reason: PlannedRestartPlatformRefresh, Target: "host"})
	rt.clearRestoreRestartMarker()
	if got := markerReason(t, rt); got != PlannedRestartPlatformRefresh {
		t.Fatalf("the restore cleanup removed or changed the host's marker: reason %q", got)
	}

	// The func markPlannedRestart returns, after the host's marker replaced its own.
	restartPlanned, err := rt.markPlannedRestart("platform_update", "update-1")
	if err != nil {
		t.Fatal(err)
	}
	writeRawMarker(t, rt, PlannedRestart{Reason: PlannedRestartPlatformRefresh, Target: "host"})
	restartPlanned(context.Canceled)
	if got := markerReason(t, rt); got != PlannedRestartPlatformRefresh {
		t.Fatalf("an apply's cleanup removed the host's marker: reason %q", got)
	}

	// And it does clear its own.
	restartPlanned, err = rt.markPlannedRestart("platform_update", "update-2")
	if err != nil {
		t.Fatal(err)
	}
	restartPlanned(context.Canceled)
	if got := markerReason(t, rt); got != "" {
		t.Fatalf("an apply's cleanup left its own marker: reason %q", got)
	}
}

// K3: no marker without a reason; the withdrawal never calls a marker it left
// on disk "cleared".
func TestAMarkerNeedsAReasonAndAnUnnamedOneIsNeverReportedCleared(t *testing.T) {
	rt, _ := testRuntime(t)
	t.Cleanup(func() { _ = ClearPlannedRestartMarker(rt.cfg.StorePath) })
	if err := WritePlannedRestartMarker(rt.cfg.StorePath, PlannedRestart{Target: "t", RequestedAt: time.Now().UTC()}); err == nil {
		t.Fatal("a marker without a reason was written")
	}
	writeRawMarker(t, rt, PlannedRestart{Target: "legacy"}) // from before the rule
	req := httptest.NewRequest(http.MethodDelete, "/internal/runtime/planned-restart", nil)
	req.RemoteAddr = "192.0.2.1:4000"
	rec := httptest.NewRecorder()
	(&APIHandler{rt: rt}).HandleInternalPlannedRestart(rec, req)
	if _, err := os.Stat(PlannedRestartMarkerPath(rt.cfg.StorePath)); err != nil {
		t.Fatalf("the withdrawal removed a marker that is not the host's: %v", err)
	}
	if strings.Contains(rec.Body.String(), `"cleared"`) {
		t.Fatalf("the withdrawal said %s about a marker it left on disk", rec.Body.String())
	}
}

// K4: the caller went away during the unwind wait; nothing was written, so no
// hold stays behind.
func TestCallerGoneDuringTheUnwindWaitReleasesTheRefreshHold(t *testing.T) {
	setUnwindBound(t, 3*time.Second)
	rt, _ := testRuntime(t)
	t.Cleanup(func() { _ = ClearPlannedRestartMarker(rt.cfg.StorePath) })
	h := newProducerHarness(t)
	capture := h.stubbornCapture(false)
	done := h.startRun(&rt.snapshots)
	waitEntered(t, capture)

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	req := httptest.NewRequest(http.MethodPost, "/internal/runtime/planned-restart?allow_busy=1", strings.NewReader(`{"target":"761d94c5"}`)).WithContext(ctx)
	req.RemoteAddr = "192.0.2.1:4000"
	rec := httptest.NewRecorder()
	(&APIHandler{rt: rt}).HandleInternalPlannedRestart(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("%d %s, want 409", rec.Code, rec.Body.String())
	}
	if rt.refreshHoldsNewWork(time.Now()) {
		t.Fatal("the refresh hold outlived a mark whose caller went away and wrote nothing")
	}
	close(capture.release)
	waitResult(t, done)
}
