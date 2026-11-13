package agentcore

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/snapshot"
)

// The point of no return and the unwind bound
// (docs/problems/snapshot-loop-blocks-its-own-fix-2026-11-12.md). Ways that
// could fail, written before the code:
//
//  U1. a restart cancels a capture that has begun to register its root, so the
//      host holds a root the tape does not know;
//  U2. a restart waits for a capture that will not unwind for as long as the
//      capture would have run (the host's request times out at 20 s, and a
//      marker written after that is a restart nobody asked for);
//  U3. a refusal for a capture that did not unwind leaves the producer blocked
//      or the capture cancelled, so a restart that did not happen has already
//      cost the capture;
//  U4. the gate to the commit says yes after the cancel was requested;
//  U5. hibernate pauses a VM whose capture would not unwind.

func setUnwindBound(t *testing.T, d time.Duration) {
	t.Helper()
	prev := snapshotCancelUnwindBound
	snapshotCancelUnwindBound = d
	t.Cleanup(func() { snapshotCancelUnwindBound = prev })
}

// stubbornCapture ignores its context until released. When commit is true it
// passes the commit gate first, as TakeSnapshot does before it registers the
// root.
func (h *producerHarness) stubbornCapture(commit bool) *parkedCapture {
	c := &parkedCapture{entered: make(chan struct{}, 4), release: make(chan struct{})}
	h.deps.Take = func(ctx context.Context, _ time.Duration) (snapshot.Ref, error) {
		c.entered <- struct{}{}
		if commit {
			if err := snapshotCommitGate(ctx)(); err != nil {
				c.canceled.Store(true)
				return snapshot.Ref{}, err
			}
		}
		<-c.release
		if ctx.Err() != nil && !commit {
			c.canceled.Store(true)
			return snapshot.Ref{}, context.Cause(ctx)
		}
		return snapshot.Ref{EventID: "event-snapshot", Record: snapshot.Record{StoreHeadSequence: h.head, PlaintextSize: 1}}, nil
	}
	return c
}

// U1: past the gate the capture finishes (root, then event) and the restart
// proceeds once it has.
func TestRestartWaitsForACaptureThatIsRecordingItsRootInsteadOfCancellingIt(t *testing.T) {
	rt, _ := testRuntime(t)
	t.Cleanup(func() { _ = ClearPlannedRestartMarker(rt.cfg.StorePath) })
	_ = ClearPlannedRestartMarker(rt.cfg.StorePath)
	h := newProducerHarness(t)
	h.deps.StorePath = rt.cfg.StorePath
	capture := h.stubbornCapture(true)
	done := h.startRun(&rt.snapshots)
	waitEntered(t, capture)
	go func() { time.Sleep(300 * time.Millisecond); close(capture.release) }()

	code, body := postPlannedRestartURL(t, &APIHandler{rt: rt}, "/internal/runtime/planned-restart?allow_busy=1", `{"target":"761d94c5"}`)
	if code != http.StatusOK {
		t.Fatalf("restart during the commit: status=%d body=%s, want 200 after the commit finished", code, body)
	}
	if capture.canceled.Load() {
		t.Fatal("the commit was cancelled")
	}
	if r := waitResult(t, done); r.outcome != snapshotTaken {
		t.Fatalf("the committing capture ended %s %s, want taken", r.outcome, r.detail)
	}
}

// U1 and U2: a commit that does not finish inside the bound refuses the
// restart, cancels nothing and blocks nothing.
func TestRestartRefusesWhenTheCommitDoesNotFinishInTheBound(t *testing.T) {
	setUnwindBound(t, 300*time.Millisecond)
	rt, _ := testRuntime(t)
	t.Cleanup(func() { _ = ClearPlannedRestartMarker(rt.cfg.StorePath) })
	_ = ClearPlannedRestartMarker(rt.cfg.StorePath)
	h := newProducerHarness(t)
	capture := h.stubbornCapture(true)
	done := h.startRun(&rt.snapshots)
	waitEntered(t, capture)

	code, body := postPlannedRestartURL(t, &APIHandler{rt: rt}, "/internal/runtime/planned-restart?allow_busy=1", `{"target":"761d94c5"}`)
	if code != http.StatusConflict || !strings.Contains(body, "snapshot_not_unwound") {
		t.Fatalf("status=%d body=%s, want 409 naming snapshot_not_unwound", code, body)
	}
	if _, err := os.Stat(PlannedRestartMarkerPath(rt.cfg.StorePath)); !os.IsNotExist(err) {
		t.Fatalf("refusal wrote a marker: %v", err)
	}
	close(capture.release)
	if r := waitResult(t, done); r.outcome != snapshotTaken {
		t.Fatalf("the commit ended %s %s, want taken: a refused restart must not cut it", r.outcome, r.detail)
	}
	// U3: the refusal left the producer able to run.
	if outcome, _ := h.run(&rt.snapshots, time.Now()); outcome == snapshotSkipped {
		t.Fatal("a refused restart left the producer blocked")
	}
}

// U2 and U3: a capture that ignores its cancel past the bound refuses the
// restart with a reason that says so, and the producer is not left blocked.
func TestRestartRefusesWhenTheCaptureDoesNotUnwindInTheBound(t *testing.T) {
	setUnwindBound(t, 300*time.Millisecond)
	rt, _ := testRuntime(t)
	t.Cleanup(func() { _ = ClearPlannedRestartMarker(rt.cfg.StorePath) })
	_ = ClearPlannedRestartMarker(rt.cfg.StorePath)
	h := newProducerHarness(t)
	capture := h.stubbornCapture(false)
	done := h.startRun(&rt.snapshots)
	waitEntered(t, capture)

	start := time.Now()
	code, body := postPlannedRestartURL(t, &APIHandler{rt: rt}, "/internal/runtime/planned-restart?allow_busy=1", `{"target":"761d94c5"}`)
	if code != http.StatusConflict || !strings.Contains(body, "snapshot_not_unwound") || time.Since(start) > 5*time.Second {
		t.Fatalf("status=%d body=%s after %s, want a prompt 409 naming snapshot_not_unwound", code, body, time.Since(start))
	}
	if _, err := os.Stat(PlannedRestartMarkerPath(rt.cfg.StorePath)); !os.IsNotExist(err) {
		t.Fatalf("refusal wrote a marker: %v", err)
	}
	close(capture.release)
	waitResult(t, done)
	h.deps.Take = func(context.Context, time.Duration) (snapshot.Ref, error) {
		return snapshot.Ref{EventID: "e", Record: snapshot.Record{StoreHeadSequence: h.head, PlaintextSize: 1}}, nil
	}
	if outcome, detail := h.run(&rt.snapshots, time.Now()); outcome != snapshotTaken {
		t.Fatalf("after the refusal: %s %s, want the producer free to run", outcome, detail)
	}
}

// U4: once a cancel is requested the commit gate says no.
func TestTheCommitGateRefusesAfterACancelWasRequested(t *testing.T) {
	var p snapshotProducer
	gateSeen := make(chan func() error, 1)
	release := make(chan struct{})
	deps := newProducerHarness(t).deps
	deps.Take = func(ctx context.Context, _ time.Duration) (snapshot.Ref, error) {
		gateSeen <- snapshotCommitGate(ctx)
		<-ctx.Done()
		<-release
		return snapshot.Ref{}, context.Cause(ctx)
	}
	done := make(chan captureResult, 1)
	go func() {
		o, d := p.run(context.Background(), deps, snapshotSettings{Interval: time.Hour, MaxDuration: time.Hour}, time.Now())
		done <- captureResult{o, d}
	}()
	gate := <-gateSeen
	go func() { _ = p.cancelInFlight(context.Background(), errSnapshotCancelledForRestart, time.Second, 0) }()
	deadline := time.Now().Add(5 * time.Second)
	for gate() == nil {
		if time.Now().After(deadline) {
			t.Fatal("the gate still says yes after a cancel was requested")
		}
		time.Sleep(10 * time.Millisecond)
	}
	close(release)
	waitResult(t, done)
}

// U5: a hibernate whose capture does not unwind is refused, with admission
// open again.
func TestHibernatePrepareRefusesWhenTheCaptureDoesNotUnwind(t *testing.T) {
	setUnwindBound(t, 300*time.Millisecond)
	rt, _ := hibernateTestRuntime(t)
	h := newProducerHarness(t)
	capture := h.stubbornCapture(false)
	done := h.startRun(&rt.snapshots)
	waitEntered(t, capture)

	code, body := postHibernate(t, &APIHandler{rt: rt}, "/internal/runtime/hibernate/prepare", "192.0.2.1:4000", `{}`)
	if code != http.StatusConflict || body["outcome"] != "busy" || body["reason"] != "snapshot_not_unwound" {
		t.Fatalf("prepare with a stubborn capture: %d %v, want 409 busy snapshot_not_unwound", code, body)
	}
	if rt.hibernate.closed() {
		t.Fatal("a refused prepare left the fence closed")
	}
	close(capture.release)
	waitResult(t, done)
}

// R5: while the fence is closed no capture starts.
func TestNoCaptureStartsWhileTheHibernateFenceIsClosed(t *testing.T) {
	rt, _ := hibernateTestRuntime(t)
	if !rt.snapshotsMayStart() {
		t.Fatal("an open fence blocks captures")
	}
	if _, ok := rt.hibernate.close(time.Minute); !ok {
		t.Fatal("fence did not close")
	}
	if rt.snapshotsMayStart() {
		t.Fatal("a capture may start while the fence is closed")
	}
}
