package agentcore

import (
	"bytes"
	"context"
	"log"
	"reflect"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Failure list (docs/problems/captures-left-on-closed-tasks-2026-11-24.md: in
// seven days the paper left 476 captures on a task that closed without
// deciding them; in 461 a management run followed and did not delegate them):
// R1 a task whose run failed, was passivated, hit its deadline or completed
//    without closing leaves its undecided captures to a model's choice;
// R2 the re-queued task loses the records the first was given, or leases a
//    capture the first already decided;
// R3 a capture that fails every time is re-queued for ever;
// R4 a capture closed after its attempts has no row saying so, or can be
//    delegated again;
// R5 a task that finished (done) and left captures is re-queued by the host,
//    overriding a decision management's task made;
// R6 a cancel, or a crash restart (which resumes nothing), re-queues;
// R7 nothing says how many captures a closure left, re-queued and closed.

func captureIndex(t *testing.T, f *delegationFixture, id string) types.CaptureDisposition {
	t.Helper()
	d, _, err := f.s.CaptureDispositionOf(objectgraph.WithCustodianReader(context.Background(), "choir.capture_disposition"), f.owner, f.computer, id)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// successor returns the one open task, other than those in before, that
// leases the capture.
func successor(t *testing.T, f *delegationFixture, capture string, before ...string) *types.ManagementTask {
	t.Helper()
	old := map[string]bool{}
	for _, id := range before {
		old[id] = true
	}
	var found *types.ManagementTask
	for _, task := range f.tasks("") {
		if old[task.TaskID] || task.State == types.ManagementTaskClosed {
			continue
		}
		for _, c := range task.LeasedCaptures {
			if c == capture {
				if found != nil {
					t.Fatalf("capture %s is leased by two open tasks: %s and %s", capture, found.TaskID, task.TaskID)
				}
				task := task
				found = &task
			}
		}
	}
	return found
}

// TestAHostVisibleClosureRequeuesItsCaptures (R1, R2).
func TestAHostVisibleClosureRequeuesItsCaptures(t *testing.T) {
	for _, reason := range []string{"run_failed", types.TaskClosureRunPassivated, types.TaskClosureDeadline, "run_completed"} {
		t.Run(reason, func(t *testing.T) {
			f, queued, gate := queueFixture(t, "owner-requeue-"+strings.ReplaceAll(reason, "_", "-"), 2)
			defer close(gate)
			ctx := context.Background()
			first := queued[1]
			capture := first.LeasedCaptures[0]
			f.rt.closeManagementTask(ctx, first, reason, "", "the test ended it", "")
			if got := f.task(first.TaskID); got.State != types.ManagementTaskClosed {
				t.Fatalf("fixture: task is %s, want closed", got.State)
			}
			next := successor(t, f, capture, first.TaskID)
			if next == nil {
				t.Fatalf("after a %s closure no open task leases %s: it is left to a model's choice", reason, capture)
			}
			if next.State != types.ManagementTaskQueued {
				t.Fatalf("the re-queued task is %s, want queued", next.State)
			}
			if !reflect.DeepEqual(next.DelegatedRecordIDs, first.DelegatedRecordIDs) {
				t.Fatalf("the re-queued task was given records %v, want the first task's %v", next.DelegatedRecordIDs, first.DelegatedRecordIDs)
			}
			if !reflect.DeepEqual(next.LeasedCaptures, []string{capture}) {
				t.Fatalf("the re-queued task leases %v, want only %s", next.LeasedCaptures, capture)
			}
			ix := captureIndex(t, f, capture)
			if ix.LeftCount != 1 || ix.Fate != "" || ix.LeaseTask != next.TaskID {
				t.Fatalf("index after one re-queue: left_count=%d fate=%q lease=%s, want 1, none, %s", ix.LeftCount, ix.Fate, ix.LeaseTask, next.TaskID)
			}
		})
	}
}

// TestACaptureIsClosedAfterItsAttempts (R3, R4).
func TestACaptureIsClosedAfterItsAttempts(t *testing.T) {
	f, queued, gate := queueFixture(t, "owner-requeue-bound", 2)
	defer close(gate)
	ctx := context.Background()
	task := queued[1]
	capture := task.LeasedCaptures[0]
	seen := []string{}
	for attempt := 1; attempt <= types.CaptureMaxAttempts; attempt++ {
		f.rt.closeManagementTask(ctx, task, "run_failed", "", "the test failed it", "")
		seen = append(seen, task.TaskID)
		next := successor(t, f, capture, seen...)
		if attempt < types.CaptureMaxAttempts {
			if next == nil {
				t.Fatalf("attempt %d of %d: the capture was not re-queued", attempt, types.CaptureMaxAttempts)
			}
			task = *next
			continue
		}
		if next != nil {
			t.Fatalf("after %d failed attempts the capture was queued again (task %s)", attempt, next.TaskID)
		}
	}
	ix := captureIndex(t, f, capture)
	if ix.Fate != types.CaptureFateLeftAfterAttempts || ix.LeftCount != types.CaptureMaxAttempts || ix.FateReason != "run_failed" {
		t.Fatalf("index after the last attempt: fate=%q left_count=%d reason=%q, want %q, %d, run_failed", ix.Fate, ix.LeftCount, ix.FateReason, types.CaptureFateLeftAfterAttempts, types.CaptureMaxAttempts)
	}
	if free, err := f.rt.captureDelegable(ctx, f.owner, f.computer, capture); err != nil || free {
		t.Fatalf("a capture closed after its attempts is delegable (%v, %v)", free, err)
	}
}

// TestOtherClosuresDoNotRequeue (R5, R6): a finished task's leftovers are
// management's; a cancel and a crash restart start nothing.
func TestOtherClosuresDoNotRequeue(t *testing.T) {
	for _, reason := range []string{types.TaskClosureDone, types.TaskClosureCancelled, "run_cancelled", types.ClosureReasonInterruptedByRestart} {
		t.Run(reason, func(t *testing.T) {
			f, queued, gate := queueFixture(t, "owner-norequeue-"+strings.ReplaceAll(reason, "_", "-"), 2)
			defer close(gate)
			ctx := context.Background()
			first := queued[1]
			capture := first.LeasedCaptures[0]
			f.rt.closeManagementTask(ctx, first, reason, "", "", "")
			if got := f.task(first.TaskID); got.State != types.ManagementTaskClosed {
				t.Fatalf("fixture: task is %s, want closed", got.State)
			}
			if next := successor(t, f, capture, first.TaskID); next != nil {
				t.Fatalf("a %s closure re-queued its capture (task %s)", reason, next.TaskID)
			}
			if ix := captureIndex(t, f, capture); ix.LeftCount != 0 || ix.Fate == types.CaptureFateLeftAfterAttempts {
				t.Fatalf("a %s closure counted an attempt: left_count=%d fate=%q", reason, ix.LeftCount, ix.Fate)
			}
		})
	}
}

// TestAClosureSaysWhatBecameOfItsCaptures (R7).
func TestAClosureSaysWhatBecameOfItsCaptures(t *testing.T) {
	f, queued, gate := queueFixture(t, "owner-requeue-log", 2)
	defer close(gate)
	var buf bytes.Buffer
	old := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(old)
	f.rt.closeManagementTask(context.Background(), queued[1], "run_failed", "", "the test failed it", "")
	log.SetOutput(old)
	want := "management task captures: task=" + queued[1].TaskID + " reason=run_failed leased=1 decided=0 requeued=1 closed_after_attempts=0 to_management=0 unaccounted=0"
	if !strings.Contains(buf.String(), want) {
		t.Fatalf("no line %q in:\n%s", want, buf.String())
	}
}
