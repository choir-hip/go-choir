package agentcore

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/snapshot"
)

// Review round 1 on lane/restart-cancels-capture (two seats). Failure list,
// written before the fixes:
//
//  F1. a run that passed Ready before the hibernate fence closed starts its
//      capture after prepareHibernate saw nothing to cancel, and the VM is
//      paused mid-capture;
//  F2. the unwind bound plus the rest of the handler cannot fit the operator
//      restart's 10 s budget;
//  F4. a withdrawal (DELETE) lands while a mark is still waiting on the
//      capture, and the mark then writes its marker after the withdrawal;
//  F5. one request's failed wait lifts another request's hold;
//  F8. a real failure that lands in the cancel's window is reported as a
//      cancellation (no backoff);
//  F9. the busy refusal drops snapshot_in_flight for a computer that is
//      snapshotting.

// F1, deterministic: the fence closes while the run is between Ready and begin.
func TestACaptureDoesNotStartWhenTheFenceClosesBetweenReadyAndBegin(t *testing.T) {
	var closed atomic.Bool
	h := newProducerHarness(t)
	h.deps.Ready = func() bool { return !closed.Load() }
	h.deps.Busy = func(context.Context) (string, error) { closed.Store(true); return "", nil }
	var p snapshotProducer
	outcome, detail := h.run(&p, time.Now())
	if outcome != snapshotSkipped || h.takes != 0 {
		t.Fatalf("outcome=%s takes=%d detail=%q, want the run skipped: the fence closed before the capture began", outcome, h.takes, detail)
	}
}

// F1 with the real fence: prepareHibernate answers while a run sits between
// Ready and begin; the run must then not start a capture.
func TestPrepareHibernateWhileARunIsBetweenReadyAndBeginStartsNoCapture(t *testing.T) {
	rt, _ := hibernateTestRuntime(t)
	h := newProducerHarness(t)
	h.deps.Ready = rt.snapshotsMayStart
	inBusy, goOn := make(chan struct{}), make(chan struct{})
	h.deps.Busy = func(context.Context) (string, error) { close(inBusy); <-goOn; return "", nil }
	done := h.startRun(&rt.snapshots)
	<-inBusy

	code, body := postHibernate(t, &APIHandler{rt: rt}, "/internal/runtime/hibernate/prepare", "192.0.2.1:4000", `{}`)
	if code != http.StatusOK || body["status"] != "prepared" {
		t.Fatalf("prepare: %d %v", code, body)
	}
	close(goOn)
	r := waitResult(t, done)
	if r.outcome != snapshotSkipped || h.takes != 0 {
		t.Fatalf("outcome=%s takes=%d: a capture started after the VM was prepared for its pause", r.outcome, h.takes)
	}
}

// F2: the whole handler's worst case against the smallest caller budget.
// restartMarkTimeout is 10 s (internal/vmctl/owner_update.go) and agentcore
// cannot import vmctl; the arithmetic is in snapshotCancelUnwindBound's
// comment: the wait is the one cost this change adds, and it must leave most
// of 10 s to the drain, the acks and the fsyncs.
func TestTheUnwindBoundLeavesMostOfTheOperatorRestartBudget(t *testing.T) {
	const operatorRestartBudget = 10 * time.Second
	if snapshotCancelUnwindBound > operatorRestartBudget*3/10 {
		t.Fatalf("snapshotCancelUnwindBound = %s, want at most %s (30%% of the %s operator restart budget)", snapshotCancelUnwindBound, operatorRestartBudget*3/10, operatorRestartBudget)
	}
}

// F4: a withdrawal waits for a mark in flight, so it cannot land before that
// mark's marker.
func TestAWithdrawalDuringAPendingMarkClearsTheMarkerTheMarkWrites(t *testing.T) {
	setUnwindBound(t, 5*time.Second)
	rt, _ := testRuntime(t)
	t.Cleanup(func() { _ = ClearPlannedRestartMarker(rt.cfg.StorePath) })
	_ = ClearPlannedRestartMarker(rt.cfg.StorePath)
	h := newProducerHarness(t)
	capture := h.stubbornCapture(false)
	done := h.startRun(&rt.snapshots)
	waitEntered(t, capture)

	postDone := make(chan int, 1)
	go func() {
		code, _ := postPlannedRestartURL(t, &APIHandler{rt: rt}, "/internal/runtime/planned-restart?allow_busy=1", `{"target":"761d94c5"}`)
		postDone <- code
	}()
	// The mark is now waiting on the capture.
	deadline := time.Now().Add(5 * time.Second)
	for {
		rt.snapshots.mu.Lock()
		waiting := rt.snapshots.cur != nil && rt.snapshots.cur.cancelled
		rt.snapshots.mu.Unlock()
		if waiting {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the mark never started waiting on the capture")
		}
		time.Sleep(5 * time.Millisecond)
	}
	delDone := make(chan int, 1)
	go func() {
		req := httptest.NewRequest(http.MethodDelete, "/internal/runtime/planned-restart", nil)
		req.RemoteAddr = "192.0.2.1:4000"
		rec := httptest.NewRecorder()
		(&APIHandler{rt: rt}).HandleInternalPlannedRestart(rec, req)
		delDone <- rec.Code
	}()
	time.Sleep(100 * time.Millisecond)
	close(capture.release)
	if code := <-postDone; code != http.StatusOK {
		t.Fatalf("mark: %d", code)
	}
	if code := <-delDone; code != http.StatusOK {
		t.Fatalf("withdrawal: %d", code)
	}
	waitResult(t, done)
	if _, err := os.Stat(PlannedRestartMarkerPath(rt.cfg.StorePath)); !os.IsNotExist(err) {
		t.Fatalf("a marker outlived the withdrawal: %v", err)
	}
}

// F5: a request whose wait fails lifts only its own hold.
func TestAFailedWaitDoesNotLiftALaterRequestsHold(t *testing.T) {
	var p snapshotProducer
	h := newProducerHarness(t)
	capture := h.stubbornCapture(false)
	done := h.startRun(&p)
	waitEntered(t, capture)

	bDone := make(chan error, 1)
	aDone := make(chan error, 1)
	go func() {
		aDone <- p.cancelInFlight(context.Background(), errSnapshotCancelledForRestart, 400*time.Millisecond, time.Minute)
	}()
	time.Sleep(100 * time.Millisecond)
	go func() {
		bDone <- p.cancelInFlight(context.Background(), errSnapshotCancelledForRestart, 3*time.Second, time.Minute)
	}()
	if err := <-aDone; err == nil {
		t.Fatal("A's wait should have failed")
	}
	p.mu.Lock()
	held := time.Now().Before(p.blockUntil)
	p.mu.Unlock()
	if !held {
		t.Fatal("A's failed wait lifted B's hold")
	}
	close(capture.release)
	if err := <-bDone; err != nil {
		t.Fatalf("B: %v", err)
	}
	waitResult(t, done)
}

// F8: a genuine failure that lands after a cancel was requested is a failure.
func TestARealFailureInTheCancelWindowIsStillAFailure(t *testing.T) {
	var p snapshotProducer
	h := newProducerHarness(t)
	entered := make(chan struct{})
	h.deps.Take = func(ctx context.Context, _ time.Duration) (snapshot.Ref, error) {
		close(entered)
		<-ctx.Done()
		return snapshot.Ref{}, errors.New("snapshot: write chunk: disk full")
	}
	done := h.startRun(&p)
	<-entered
	if err := p.cancelInFlight(context.Background(), errSnapshotCancelledForRestart, 5*time.Second, 0); err != nil {
		t.Fatal(err)
	}
	r := waitResult(t, done)
	p.mu.Lock()
	failures := p.failures
	p.mu.Unlock()
	if r.outcome != snapshotFailed || failures != 1 {
		t.Fatalf("outcome=%s failures=%d detail=%q, want a failed attempt with its backoff", r.outcome, failures, r.detail)
	}
}

// F9: the busy refusal names a capture in flight, and only then.
func TestBusyRefusalReportsSnapshotInFlightTruthfully(t *testing.T) {
	rt, _ := testRuntime(t)
	t.Cleanup(func() { _ = ClearPlannedRestartMarker(rt.cfg.StorePath) })
	_ = ClearPlannedRestartMarker(rt.cfg.StorePath)
	rt.runningMu.Lock()
	rt.running["loop-busy"] = func() {}
	rt.runningMu.Unlock()
	handler := &APIHandler{rt: rt}

	code, body := postPlannedRestart(t, handler, "192.0.2.1:4000", `{"target":"761d94c5"}`)
	if code != http.StatusConflict || strings.Contains(body, "snapshot_in_flight") {
		t.Fatalf("no capture: %d %s, want 409 without snapshot_in_flight", code, body)
	}
	h := newProducerHarness(t)
	capture := h.parkOnContext()
	done := h.startRun(&rt.snapshots)
	waitEntered(t, capture)
	code, body = postPlannedRestart(t, handler, "192.0.2.1:4000", `{"target":"761d94c5"}`)
	if code != http.StatusConflict || !strings.Contains(body, `"snapshot_in_flight":true`) {
		t.Fatalf("capture in flight: %d %s, want 409 naming snapshot_in_flight", code, body)
	}
	close(capture.release)
	waitResult(t, done)
}
