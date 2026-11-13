package agentcore

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/snapshot"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// A planned restart cancels an in-flight snapshot capture and proceeds
// (docs/problems/snapshot-loop-blocks-its-own-fix-2026-11-12.md). A capture
// records nothing until it ends, so a restart loses only the time spent.
// Ways that could fail, written before the code:
//
//  R1. the restart is refused while a capture streams (the incident: 55
//      deferred refreshes), allow_busy or not;
//  R2. the capture is cancelled but the restart does not wait for it to
//      unwind, so the read transaction or the attempt guard outlives the
//      answer that lets the host kill the VM;
//  R3. a restart-cancelled capture counts as a failure: the backoff grows, or
//      the crash-attempt guard stays spent and three restarts in a row close
//      the producer for good;
//  R4. the producer starts a new capture between the cancel and the kill;
//  R5. hibernate pauses the VM with a capture streaming, which can then record
//      a root whose chunks were collected during the pause;
//  R6. a refused restart (self-development apply, live capsule) cancels the
//      capture anyway.

// parkedCapture makes the harness capture park until its context ends (or
// release closes), recording why it ended.
type parkedCapture struct {
	entered  chan struct{}
	release  chan struct{}
	cause    atomic.Value // string
	canceled atomic.Bool
}

func (h *producerHarness) parkOnContext() *parkedCapture {
	c := &parkedCapture{entered: make(chan struct{}, 4), release: make(chan struct{})}
	h.deps.Take = func(ctx context.Context, _ time.Duration) (snapshot.Ref, error) {
		c.entered <- struct{}{}
		select {
		case <-ctx.Done():
			cause := context.Cause(ctx)
			c.cause.Store(cause.Error())
			c.canceled.Store(true)
			return snapshot.Ref{}, fmt.Errorf("snapshot: read: %w", cause)
		case <-c.release:
			return snapshot.Ref{EventID: "event-snapshot", Record: snapshot.Record{StoreHeadSequence: h.head, PlaintextSize: 1}}, nil
		}
	}
	return c
}

func (c *parkedCapture) causeText() string {
	v, _ := c.cause.Load().(string)
	return v
}

type captureResult struct {
	outcome snapshotOutcome
	detail  string
}

func (h *producerHarness) startRun(p *snapshotProducer) chan captureResult {
	done := make(chan captureResult, 1)
	go func() {
		outcome, detail := h.run(p, time.Now())
		done <- captureResult{outcome, detail}
	}()
	return done
}

func waitEntered(t *testing.T, c *parkedCapture) {
	t.Helper()
	select {
	case <-c.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the capture never started")
	}
}

func waitResult(t *testing.T, done chan captureResult) captureResult {
	t.Helper()
	select {
	case r := <-done:
		return r
	case <-time.After(10 * time.Second):
		t.Fatal("the producer run never returned")
		return captureResult{}
	}
}

func TestPlannedRestartCancelsAStreamingCaptureAndProceeds(t *testing.T) {
	for _, tc := range []struct{ name, url string }{
		{"allow_busy", "/internal/runtime/planned-restart?allow_busy=1"},
		{"not allow_busy", "/internal/runtime/planned-restart"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rt, _ := testRuntime(t)
			t.Cleanup(func() { _ = ClearPlannedRestartMarker(rt.cfg.StorePath) })
			_ = ClearPlannedRestartMarker(rt.cfg.StorePath)
			h := newProducerHarness(t)
			h.deps.StorePath = rt.cfg.StorePath
			capture := h.parkOnContext()
			done := h.startRun(&rt.snapshots)
			waitEntered(t, capture)

			code, body := postPlannedRestartURL(t, &APIHandler{rt: rt}, tc.url, `{"target":"761d94c5"}`)
			if code != http.StatusOK || strings.Contains(body, "snapshot_in_flight") {
				t.Fatalf("restart during a capture: status=%d body=%s, want 200 and no snapshot refusal", code, body)
			}
			if _, err := os.Stat(PlannedRestartMarkerPath(rt.cfg.StorePath)); err != nil {
				t.Fatalf("200 without a durable marker: %v", err)
			}
			if !capture.canceled.Load() || !strings.Contains(capture.causeText(), "planned restart") {
				t.Fatalf("capture cancelled=%v cause=%q, want a cancel naming the planned restart", capture.canceled.Load(), capture.causeText())
			}
			// R2: the answer came after the capture unwound and its attempt
			// guard was settled, not before.
			if rt.snapshots.inFlight.Load() {
				t.Fatal("the restart was answered while the capture was still in flight")
			}
			if n, err := peekGuestAttempts(rt.cfg.StorePath, snapshotAttemptKey, h.deps.ComputerID); err != nil || n != 0 {
				t.Fatalf("attempt guard = %d (%v) when the restart was answered, want 0: three restarts would close the producer", n, err)
			}
			result := waitResult(t, done)
			// R3: not a failure; the backoff is untouched.
			if result.outcome == snapshotFailed || strings.Contains(result.detail, "failure") {
				t.Fatalf("a restart-cancelled capture was reported as a failed attempt: %s %q", result.outcome, result.detail)
			}
			rt.snapshots.mu.Lock()
			failures, retryAt := rt.snapshots.failures, rt.snapshots.retryAt
			rt.snapshots.mu.Unlock()
			if failures != 0 || !retryAt.IsZero() {
				t.Fatalf("backoff after a restart-cancel: failures=%d retryAt=%s", failures, retryAt)
			}
		})
	}
}

// R4: between the cancel and the kill no new capture starts; a withdrawn
// restart lets the producer take one at once.
func TestNoNewCaptureStartsAfterARestartCancelUntilTheRestartIsWithdrawn(t *testing.T) {
	rt, _ := testRuntime(t)
	t.Cleanup(func() { _ = ClearPlannedRestartMarker(rt.cfg.StorePath) })
	_ = ClearPlannedRestartMarker(rt.cfg.StorePath)
	h := newProducerHarness(t)
	h.deps.StorePath = rt.cfg.StorePath
	capture := h.parkOnContext()
	done := h.startRun(&rt.snapshots)
	waitEntered(t, capture)
	if code, body := postPlannedRestartURL(t, &APIHandler{rt: rt}, "/internal/runtime/planned-restart?allow_busy=1", `{"target":"761d94c5"}`); code != http.StatusOK {
		t.Fatalf("restart: %d %s", code, body)
	}
	waitResult(t, done)

	if outcome, _ := h.run(&rt.snapshots, time.Now()); outcome != snapshotSkipped {
		t.Fatalf("a capture started after the restart cancel: outcome %s", outcome)
	}
	req := httptest.NewRequest(http.MethodDelete, "/internal/runtime/planned-restart", nil)
	req.RemoteAddr = "192.0.2.1:4000"
	rec := httptest.NewRecorder()
	(&APIHandler{rt: rt}).HandleInternalPlannedRestart(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("withdraw: %d %s", rec.Code, rec.Body.String())
	}
	close(capture.release)
	if outcome, detail := h.run(&rt.snapshots, time.Now()); outcome != snapshotTaken {
		t.Fatalf("after the restart was withdrawn: %s %s, want a capture taken", outcome, detail)
	}
}

// R6: work a restart cannot survive still refuses, and the capture is not cut
// for a restart that does not happen.
func TestRefusedRestartDoesNotCancelTheCapture(t *testing.T) {
	ctx := context.Background()
	for _, kind := range []string{"selfdev_apply", "live_capsule"} {
		t.Run(kind, func(t *testing.T) {
			rt, s := testRuntime(t)
			t.Cleanup(func() { _ = ClearPlannedRestartMarker(rt.cfg.StorePath) })
			_ = ClearPlannedRestartMarker(rt.cfg.StorePath)
			switch kind {
			case "selfdev_apply":
				rt.cfg.ComputerID = "computer-restart-cancel-refused"
				operations, err := selfdev.NewStore(s, s)
				if err != nil {
					t.Fatal(err)
				}
				rt.selfdevOperations = operations
				now := time.Now().UTC()
				state := selfdev.StateExecuting
				if _, err := s.DB().ExecContext(ctx, `INSERT INTO self_development_operations (operation_id,computer_id,idempotency_key,request_commitment,trajectory_id,base_head,prompt_artifact_ref,verifier_refs_json,desired_head,effective_head,state,created_at,updated_at) VALUES (?,?,?,?,?,?,?,'[]',?,?,?, ?,?)`,
					"selfdev-"+state, rt.cfg.ComputerID, "key-"+state, strings.Repeat("d", 64), "trajectory-"+state, strings.Repeat("a", 64),
					"artifact:sha256:"+strings.Repeat("b", 64), strings.Repeat("c", 64), strings.Repeat("c", 64), state, now, now); err != nil {
					t.Fatal(err)
				}
			case "live_capsule":
				seed, err := store.SeedEngineeringAssignmentAuthority(s, "owner-live-capsule-cancel", rt.TextureComputerID(), 1)
				if err != nil {
					t.Fatal(err)
				}
				open := store.SeedEngineeringOpenRequest(seed, 0, "opaque-live-capsule-cancel")
				if _, err := s.OpenEngineeringAssignment(ctx, open); err != nil {
					t.Fatal(err)
				}
				if _, err := s.BindEngineeringAssignment(ctx, store.SeedEngineeringBindRequest(open, "run:"+seed.AssignmentIDs[0], "opaque-live-capsule-cancel")); err != nil {
					t.Fatal(err)
				}
			}
			h := newProducerHarness(t)
			capture := h.parkOnContext()
			done := h.startRun(&rt.snapshots)
			waitEntered(t, capture)
			code, body := postPlannedRestartURL(t, &APIHandler{rt: rt}, "/internal/runtime/planned-restart?allow_busy=1", `{"target":"761d94c5"}`)
			if code != http.StatusConflict {
				t.Fatalf("%s with a capture: status=%d body=%s, want 409", kind, code, body)
			}
			if _, err := os.Stat(PlannedRestartMarkerPath(rt.cfg.StorePath)); !os.IsNotExist(err) {
				t.Fatalf("refusal wrote a marker: %v", err)
			}
			time.Sleep(100 * time.Millisecond)
			if capture.canceled.Load() {
				t.Fatal("a refused restart cancelled the capture")
			}
			close(capture.release)
			if r := waitResult(t, done); r.outcome != snapshotTaken {
				t.Fatalf("capture after a refused restart: %s %s", r.outcome, r.detail)
			}
		})
	}
}

// R5: hibernate cancels a capture before the VM is paused.
func TestHibernatePrepareCancelsAStreamingCapture(t *testing.T) {
	rt, _ := hibernateTestRuntime(t)
	h := newProducerHarness(t)
	capture := h.parkOnContext()
	done := h.startRun(&rt.snapshots)
	waitEntered(t, capture)

	code, body := postHibernate(t, &APIHandler{rt: rt}, "/internal/runtime/hibernate/prepare", "192.0.2.1:4000", `{}`)
	if code != http.StatusOK || body["status"] != "prepared" {
		t.Fatalf("prepare during a capture: %d %v, want 200 prepared", code, body)
	}
	if !capture.canceled.Load() || !strings.Contains(capture.causeText(), "hibernate") {
		t.Fatalf("capture cancelled=%v cause=%q, want a cancel naming the hibernate", capture.canceled.Load(), capture.causeText())
	}
	if rt.snapshots.inFlight.Load() {
		t.Fatal("prepared while the capture was still in flight")
	}
	if r := waitResult(t, done); r.outcome == snapshotFailed {
		t.Fatalf("a hibernate-cancelled capture was reported as a failure: %s", r.detail)
	}
}
