package agentcore

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/sourceproj"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// stackBaseNotes counts the stack-base-cleared notes in doc's Texture inbox.
func stackBaseNotes(t *testing.T, f *rebaseFixture, docID string) int {
	t.Helper()
	n := 0
	for _, rec := range ownerActRecordsAny(t, f, docID) {
		if rec.SubjectRefs["stack_base_cleared"] != "" {
			n++
		}
	}
	return n
}

func ownerActRecordsAny(t *testing.T, f *rebaseFixture, docID string) []types.CommitmentRecord {
	t.Helper()
	recs, err := f.s.ListInboxRecords(context.Background(), f.owner, f.computer, currentTextureAgentID(docID), 0, 1<<40)
	if err != nil {
		t.Fatal(err)
	}
	return recs
}

func refuseAtBaseMoved(t *testing.T, f *rebaseFixture, docID, castID string) {
	t.Helper()
	result, err := f.rt.ReconcileSourceProjection(context.Background(), "cast")
	if err != nil || result.State != sourceproj.StateBaseMoved {
		t.Fatalf("setup: %v %+v", err, result)
	}
	f.rt.noteStackBaseWaiter(context.Background(), stackBaseWaiter{OwnerID: f.owner, ComputerID: f.computer, DocID: docID, CastRecordID: castID}, result.Refusal())
}

// Live: moved -> pending (refuses a cast) wakes no one and keeps the waiter;
// the clean step wakes it exactly once.
func TestStackBaseWakeWaitsForAStateThatAdmitsACast(t *testing.T) {
	f := newRebaseFixture(t)
	ctx := context.Background()
	seedOwnerActDocument(t, f.s, f.owner, f.computer, "doc-wait")
	refuseAtBaseMoved(t, f, "doc-wait", "cast-1")
	f.rt.wakeStackBaseWaiters(ctx, sourceproj.Result{State: sourceproj.StatePending})
	if n := stackBaseNotes(t, f, "doc-wait"); n != 0 || len(f.rt.readStackBaseWaiters()) != 1 {
		t.Fatalf("pending woke or dropped the waiter: notes=%d", n)
	}
	setBootedBase(t, f.rt, f.base)
	after, err := f.rt.ReconcileSourceProjection(ctx, "apply")
	if err != nil || after.Refusal() != "" {
		t.Fatalf("the block did not clear: %v %+v", err, after)
	}
	if n := stackBaseNotes(t, f, "doc-wait"); n != 1 {
		t.Fatalf("want one note at the clean step, got %d", n)
	}
	if len(f.rt.readStackBaseWaiters()) != 0 {
		t.Fatal("waiter not removed after its note committed")
	}
	if _, err := f.rt.ReconcileSourceProjection(ctx, "cast"); err != nil {
		t.Fatal(err)
	}
	if n := stackBaseNotes(t, f, "doc-wait"); n != 1 {
		t.Fatalf("a later reconcile woke again: %d", n)
	}
	// A non-base-moved refusal registers nothing.
	f.rt.noteStackBaseWaiter(ctx, stackBaseWaiter{OwnerID: f.owner, ComputerID: f.computer, DocID: "doc-wait", CastRecordID: "c2"}, "source_projection_failed: x")
	if len(f.rt.readStackBaseWaiters()) != 0 {
		t.Fatal("a non-base-moved refusal registered a waiter")
	}
}

// A boot pass runs on whichever reconcile comes first, whatever its reason:
// planned keeps the waiter for the next admitting reconcile, crash deletes it.
func TestStackBaseWakeBootPassRunsOnTheFirstReconcileOfAnyReason(t *testing.T) {
	for _, planned := range []bool{true, false} {
		f := newRebaseFixture(t)
		ctx := context.Background()
		seedOwnerActDocument(t, f.s, f.owner, f.computer, "doc-wait")
		refuseAtBaseMoved(t, f, "doc-wait", "cast-1")
		// The restart: a fresh process; its first reconcile is a "bind", and
		// the rebase has applied (booted base is the stack's own).
		time.Sleep(5 * time.Millisecond)
		f.rt.bootedAt = time.Now().UTC()
		f.rt.bootRestartPlanned = planned
		f.rt.sourceProjectionLast = nil
		f.rt.stackBaseBootOnce = sync.Once{}
		setBootedBase(t, f.rt, f.base)
		if _, err := f.rt.ReconcileSourceProjection(ctx, "bind"); err != nil {
			t.Fatal(err)
		}
		n := stackBaseNotes(t, f, "doc-wait")
		if planned && n != 1 {
			t.Fatalf("a planned boot did not wake the waiter: %d", n)
		}
		if !planned && n != 0 {
			t.Fatalf("a crash boot woke a waiter: %d", n)
		}
		if left := len(f.rt.readStackBaseWaiters()); left != 0 {
			t.Fatalf("planned=%v: %d waiters left", planned, left)
		}
	}
}

// The common case: the refused document's Texture ended its turn, so its work
// item is completed (settled). It is still woken.
func TestStackBaseWakeDeliversToASettledDocument(t *testing.T) {
	f := newRebaseFixture(t)
	ctx := context.Background()
	trajectoryID, agentID := seedOwnerActDocument(t, f.s, f.owner, f.computer, "doc-wait")
	snapshot, err := f.s.GetLifecycleSnapshot(ctx, f.owner, f.computer, trajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	for _, work := range snapshot.WorkItems {
		req := types.SettleLifecycleWorkRequest{OwnerID: f.owner, ComputerID: f.computer, CommandID: "settle:" + work.WorkItemID, TrajectoryID: trajectoryID, WorkItemID: work.WorkItemID, ActingAgentID: agentID, ResultRef: "result:done"}
		req.CommandDigest, _ = store.ComputeSettleLifecycleWorkDigest(req)
		if _, err := f.s.SettleLifecycleWork(ctx, req); err != nil {
			t.Fatal(err)
		}
	}
	if state, err := f.rt.rebaseTextureRunState(ctx, f.owner, f.computer, trajectoryID, agentID); err != nil || state != rebaseTextureSettled {
		t.Fatalf("setup: document should be settled: %v %v", state, err)
	}
	refuseAtBaseMoved(t, f, "doc-wait", "cast-1")
	setBootedBase(t, f.rt, f.base)
	if _, err := f.rt.ReconcileSourceProjection(ctx, "apply"); err != nil {
		t.Fatal(err)
	}
	if n := stackBaseNotes(t, f, "doc-wait"); n != 1 {
		t.Fatalf("a settled document was not woken: %d", n)
	}
}
