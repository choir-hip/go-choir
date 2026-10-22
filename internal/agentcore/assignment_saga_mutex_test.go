package agentcore

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Review round 4b, DeepSeek 3 / Sonnet F6 (FP4 round 4 P2 at its source):
// the worker's terminal saga and the fate wake's resume of the staged
// proposal both drive one assignment's report. While the live saga holds
// the assignment, the resume defers to it instead of racing it (the loser
// used to fail early, at the capsule capability or the binding, and its
// cell heard an error for a report that landed). Ways it can fail: the
// resume still runs; it returns nil, consuming the wake (round 4c); it
// touches the assignment; once the saga is free it no longer re-drives.
func TestFateWakeResumeDefersToTheLiveTerminalSaga(t *testing.T) {
	f := newAdmissionFixture(t, 1)
	f.open(0)
	bound := f.bind(0)
	ctx := context.Background()
	freeze := types.SetEngineeringCapsuleDispositionRequest{
		CommandID: "cmd-saga-freeze", OwnerID: bound.Binding.OwnerID, ComputerID: bound.Binding.ComputerID,
		AssignmentID: bound.AssignmentID, Attempt: bound.Binding.Attempt, ExpectedLifecycleVersion: bound.LifecycleVersion,
		Disposition: types.EngineeringCapsuleFreezeRequested, IntentRef: "capsule-freeze-intent:saga",
		PendingProposal: &types.EngineeringPendingProposal{
			PropositionDigest: objectgraph.SHA256([]byte("saga")), Report: types.EngineeringAssignmentReport{ReportID: "report-saga"},
			FreezeIntentRef: "capsule-freeze-intent:saga", CreatedAt: time.Now().UTC(),
		},
	}
	freeze.CommandDigest, _ = store.ComputeSetEngineeringCapsuleDispositionDigest(freeze)
	staged, err := f.s.SetEngineeringCapsuleDisposition(ctx, freeze)
	if err != nil {
		t.Fatal(err)
	}
	if !assignedEngineeringFatePending(staged.Assignment) {
		t.Fatal("fixture: the assignment is not fate-pending")
	}

	saga := f.rt.assignmentSagaMutex(bound.AssignmentID, bound.Binding.Attempt)
	saga.Lock() // the worker's live terminal saga
	err = f.rt.resumeStrandedFateAssignmentIfPending(ctx, bound.Binding.OwnerID, bound.Binding.ComputerID, bound.AssignmentID, bound.Binding.Attempt)
	after := f.get(0)
	saga.Unlock()
	// The held saga is reported, not swallowed: a nil here consumed the
	// fate wake (review round 4c); actorruntime defers on this error.
	if !errors.Is(err, errAssignmentSagaHeld) {
		t.Fatalf("the resume under a held saga returned %v, want errAssignmentSagaHeld", err)
	}
	if after.LifecycleVersion != staged.Assignment.LifecycleVersion || after.CapsuleDisposition != staged.Assignment.CapsuleDisposition {
		t.Fatalf("the resume touched the assignment while the live saga held it: v%d %s -> v%d %s",
			staged.Assignment.LifecycleVersion, staged.Assignment.CapsuleDisposition, after.LifecycleVersion, after.CapsuleDisposition)
	}
	// Free, the resume re-drives as before (here it fails at the absent
	// capsule, which shows it ran).
	if err := f.rt.resumeStrandedFateAssignmentIfPending(ctx, bound.Binding.OwnerID, bound.Binding.ComputerID, bound.AssignmentID, bound.Binding.Attempt); err == nil {
		t.Fatal("with the saga free the resume did not run")
	}
}
