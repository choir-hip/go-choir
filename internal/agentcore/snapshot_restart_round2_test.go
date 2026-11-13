package agentcore

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// Review round 2 on lane/restart-cancels-capture. Failure list, written before
// the fixes:
//
//  G1. a planned boot wipes the whole attempt count: a producer closed at the
//      limit reopens, and two real crashes are forgotten;
//  M1. the host's withdrawal (DELETE) removes a marker the guest wrote for
//      its own platform_update, self-development apply or restore;
//  M2. a withdrawal waiting behind a mark ignores its own request context;
//  T1. a paused capsule is not thawed when the marker write fails after the
//      ack (the 500 branch);
//  T2. a paused capsule is not thawed when the unwind wait refuses.

// G1: a planned boot refunds at most one attempt, and never from a producer
// the limit has closed.
func TestAPlannedBootRefundsAtMostOneSnapshotAttemptAndNeverReopensAClosedProducer(t *testing.T) {
	for _, tc := range []struct {
		name    string
		planned bool
		spent   int
		want    int
	}{
		{"planned, closed at the limit", true, snapshotAttemptLimit, snapshotAttemptLimit},
		{"planned, two spent", true, 2, 1},
		{"planned, one spent", true, 1, 0},
		{"planned, none spent", true, 0, 0},
		{"crash, closed at the limit", false, snapshotAttemptLimit, snapshotAttemptLimit},
		{"crash, two spent", false, 2, 2},
		{"crash, one spent", false, 1, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rt, _ := testRuntime(t)
			rt.cfg.ComputerID = "computer-attempt-guard"
			for i := 0; i < tc.spent; i++ {
				if err := spendGuestAttempt(rt.cfg.StorePath, snapshotAttemptKey, rt.cfg.ComputerID, snapshotAttemptLimit); err != nil {
					t.Fatal(err)
				}
			}
			rt.bootRestartPlanned = tc.planned
			rt.settleSnapshotAttemptAtBoot()
			n, err := peekGuestAttempts(rt.cfg.StorePath, snapshotAttemptKey, rt.cfg.ComputerID)
			if err != nil || n != tc.want {
				t.Fatalf("attempt count = %d (%v), want %d", n, err, tc.want)
			}
		})
	}
}

func withdraw(t *testing.T, rt *Runtime) (int, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodDelete, "/internal/runtime/planned-restart", nil)
	req.RemoteAddr = "192.0.2.1:4000"
	rec := httptest.NewRecorder()
	(&APIHandler{rt: rt}).HandleInternalPlannedRestart(rec, req)
	return rec.Code, rec.Body.String()
}

// M1: the withdrawal removes the host's own marker only.
func TestWithdrawalRemovesOnlyTheHostsPlatformRefreshMarker(t *testing.T) {
	for _, tc := range []struct {
		reason string
		kept   bool
	}{
		{PlannedRestartPlatformRefresh, false},
		{"platform_update", true},
		{PlannedRestartSelfDevelopmentApply, true},
		{PlannedRestartSelfDevelopmentRestore, true},
	} {
		t.Run(tc.reason, func(t *testing.T) {
			rt, _ := testRuntime(t)
			t.Cleanup(func() { _ = ClearPlannedRestartMarker(rt.cfg.StorePath) })
			if err := WritePlannedRestartMarker(rt.cfg.StorePath, PlannedRestart{Reason: tc.reason, Target: "t", RequestedAt: time.Now().UTC(), WorkStateEpoch: CurrentWorkStateEpoch}); err != nil {
				t.Fatal(err)
			}
			code, body := withdraw(t, rt)
			if code != http.StatusOK {
				t.Fatalf("withdrawal: %d %s", code, body)
			}
			_, err := os.Stat(PlannedRestartMarkerPath(rt.cfg.StorePath))
			if tc.kept && (err != nil || !strings.Contains(body, "kept")) {
				t.Fatalf("a %q marker was removed by the host's withdrawal (stat %v, body %s)", tc.reason, err, body)
			}
			if !tc.kept && !os.IsNotExist(err) {
				t.Fatalf("the host's own marker was left (stat %v)", err)
			}
		})
	}
}

// M2: a withdrawal stuck behind a mark gives up with its own context.
func TestWithdrawalWaitingBehindAMarkRespectsItsOwnContext(t *testing.T) {
	setUnwindBound(t, 3*time.Second)
	rt, _ := testRuntime(t)
	t.Cleanup(func() { _ = ClearPlannedRestartMarker(rt.cfg.StorePath) })
	h := newProducerHarness(t)
	capture := h.stubbornCapture(false)
	done := h.startRun(&rt.snapshots)
	waitEntered(t, capture)
	postFinished := make(chan struct{})
	go func() {
		defer close(postFinished)
		postPlannedRestartURL(t, &APIHandler{rt: rt}, "/internal/runtime/planned-restart?allow_busy=1", `{"target":"761d94c5"}`)
	}()
	deadline := time.Now().Add(5 * time.Second)
	for {
		rt.snapshots.mu.Lock()
		waiting := rt.snapshots.cur != nil && rt.snapshots.cur.cancelled
		rt.snapshots.mu.Unlock()
		if waiting {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the mark never started waiting")
		}
		time.Sleep(5 * time.Millisecond)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	req := httptest.NewRequest(http.MethodDelete, "/internal/runtime/planned-restart", nil).WithContext(ctx)
	req.RemoteAddr = "192.0.2.1:4000"
	rec := httptest.NewRecorder()
	start := time.Now()
	(&APIHandler{rt: rt}).HandleInternalPlannedRestart(rec, req)
	if took := time.Since(start); took > time.Second || rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("the withdrawal took %s and answered %d, want a prompt 503 when its own context ends", took, rec.Code)
	}
	close(capture.release)
	waitResult(t, done)
	// The mark finishes before the test returns, so it cannot write into a
	// directory the test has already cleaned up.
	select {
	case <-postFinished:
	case <-time.After(10 * time.Second):
		t.Fatal("the mark never finished")
	}
}

// T1: the marker write fails after the ack (the marker path is a directory).
func TestAMarkerWriteFailureAfterTheAckThawsThePausedCapsule(t *testing.T) {
	f := newPauseFixture(t)
	if code, body := f.mark(t, "?allow_busy=1"); code != http.StatusConflict {
		t.Fatalf("phase 1: %d %s", code, body)
	}
	f.rt.waitCapsulePauses()
	if err := os.MkdirAll(PlannedRestartMarkerPath(f.rt.cfg.StorePath), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(PlannedRestartMarkerPath(f.rt.cfg.StorePath)) })
	code, body := f.mark(t, "?allow_busy=1")
	if code != http.StatusInternalServerError {
		t.Fatalf("phase 2 over an unwritable marker: %d %s, want 500", code, body)
	}
	pauseEventually(t, "the capsule to thaw", func() bool { return !f.fake.IsPaused(f.capsuleID) })
}

// T2: the unwind wait refuses after the ack; the capsule this request paused
// is thawed.
func TestSnapshotNotUnwoundThawsTheCapsuleTheRequestPaused(t *testing.T) {
	setUnwindBound(t, 200*time.Millisecond)
	f := newPauseFixture(t)
	h := newProducerHarness(t)
	capture := h.stubbornCapture(false)
	done := h.startRun(&f.rt.snapshots)
	waitEntered(t, capture)
	if code, body := f.mark(t, "?allow_busy=1"); code != http.StatusConflict || !strings.Contains(body, `"reason":"pausing"`) {
		t.Fatalf("phase 1: %d %s", code, body)
	}
	f.rt.waitCapsulePauses()
	if !f.fake.IsPaused(f.capsuleID) {
		t.Fatal("the pause did not commit")
	}
	code, body := f.mark(t, "?allow_busy=1")
	if code != http.StatusConflict || !strings.Contains(body, "snapshot_not_unwound") {
		t.Fatalf("phase 2: %d %s, want 409 snapshot_not_unwound", code, body)
	}
	if f.markerExists() {
		t.Fatal("a refused mark wrote a marker")
	}
	pauseEventually(t, "the capsule to thaw", func() bool { return !f.fake.IsPaused(f.capsuleID) })
	close(capture.release)
	waitResult(t, done)
}
