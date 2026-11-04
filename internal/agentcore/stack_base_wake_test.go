package agentcore

import (
	"context"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/sourceproj"
)

// A Cast refused at stack_base_moved is answered once, by the reconcile that
// moves the stack off stack_base_moved: the waiting Texture gets one note.
func TestStackBaseWakeWakesRefusedTextureOnceWhenTheBlockClears(t *testing.T) {
	f := newRebaseFixture(t)
	ctx := context.Background()
	seedOwnerActDocument(t, f.s, f.owner, f.computer, "doc-wait")
	result, err := f.rt.ReconcileSourceProjection(ctx, "cast")
	if err != nil || result.State != sourceproj.StateBaseMoved {
		t.Fatalf("setup: %v %+v", err, result)
	}
	w := stackBaseWaiter{OwnerID: f.owner, ComputerID: f.computer, DocID: "doc-wait", CastRecordID: "cast-1"}
	f.rt.noteStackBaseWaiter(w, result.Refusal())
	// Still moved: nothing is woken.
	if _, err := f.rt.ReconcileSourceProjection(ctx, "cast"); err != nil {
		t.Fatal(err)
	}
	noteID := "stack-base-cleared:doc-wait:cast-1"
	if rec, _ := f.s.GetCommitmentRecord(ctx, f.owner, f.computer, noteID); rec != nil {
		t.Fatalf("woken while the stack is still on a moved base: %+v", rec)
	}
	// The booted base returns to the stack's own: the block clears.
	setBootedBase(t, f.rt, f.base)
	after, err := f.rt.ReconcileSourceProjection(ctx, "apply")
	if err != nil || after.State == sourceproj.StateBaseMoved {
		t.Fatalf("the block did not clear: %v %+v", err, after)
	}
	rec, err := f.s.GetCommitmentRecord(ctx, f.owner, f.computer, noteID)
	if err != nil || rec == nil || rec.Inbox != "texture:doc-wait" {
		t.Fatalf("the waiting Texture was not woken: %v %+v", err, rec)
	}
	// Drained: a later move and clear wakes nobody again.
	if len(f.rt.readStackBaseWaiters()) != 0 {
		t.Fatalf("waiters not drained: %+v", f.rt.readStackBaseWaiters())
	}
	// A refusal that is not stack_base_moved registers nothing.
	f.rt.noteStackBaseWaiter(w, "source_projection_failed: x")
	if len(f.rt.readStackBaseWaiters()) != 0 {
		t.Fatal("a non-base-moved refusal registered a waiter")
	}
}

// The rebase apply restarts the computer: a planned boot off stack_base_moved
// wakes the persisted waiter once; a crash boot only clears it.
func TestStackBaseWakeBootPlannedWakesCrashClears(t *testing.T) {
	for _, planned := range []bool{true, false} {
		f := newRebaseFixture(t)
		ctx := context.Background()
		seedOwnerActDocument(t, f.s, f.owner, f.computer, "doc-wait")
		result, err := f.rt.ReconcileSourceProjection(ctx, "cast")
		if err != nil || result.State != sourceproj.StateBaseMoved {
			t.Fatalf("setup: %v %+v", err, result)
		}
		f.rt.noteStackBaseWaiter(stackBaseWaiter{OwnerID: f.owner, ComputerID: f.computer, DocID: "doc-wait", CastRecordID: "cast-1"}, result.Refusal())
		// The restart: a fresh process boots after the registration, with the
		// base back on the stack's own (the rebase applied).
		time.Sleep(5 * time.Millisecond)
		f.rt.bootedAt = time.Now().UTC()
		f.rt.bootRestartPlanned = planned
		f.rt.sourceProjectionLast = nil
		setBootedBase(t, f.rt, f.base)
		if _, err := f.rt.ReconcileSourceProjection(ctx, "boot"); err != nil {
			t.Fatal(err)
		}
		rec, _ := f.s.GetCommitmentRecord(ctx, f.owner, f.computer, "stack-base-cleared:doc-wait:cast-1")
		if planned && rec == nil {
			t.Fatal("a planned boot off stack_base_moved did not wake the waiter")
		}
		if !planned && rec != nil {
			t.Fatalf("a crash boot woke a waiter: %+v", rec)
		}
		if n := len(f.rt.readStackBaseWaiters()); n != 0 {
			t.Fatalf("planned=%v: %d waiters left", planned, n)
		}
	}
}
