package agentcore

import (
	"context"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// R10-1 (docs/problems/terminal-operation-swallows-owner-direction-2026-10-26.md):
// a document whose every operation is terminal is not supervising anything, so
// the "Self-development supervision" block must not be attached, and an owner
// revision with new direction must not read as a forbidden re-run.

const supervisionBlockHeader = "Self-development supervision:"

func supervisionRun(computerID, trajectoryID, intent string) *types.RunRecord {
	return &types.RunRecord{
		RunID: "run-texture-supervision", AgentID: "texture:doc-supervision", AgentProfile: agentprofile.Texture,
		ComputerID: computerID, TrajectoryID: trajectoryID,
		Metadata: map[string]any{"request_intent": intent},
	}
}

func openSupervisionOperation(t *testing.T, operations *selfdev.Store, computerID, trajectoryID, assignmentID string) selfdev.Operation {
	t.Helper()
	op, err := openFreezeCandidate(context.Background(), operations, func(context.Context) error { return nil }, computerID, trajectoryID, assignmentID, "objective "+assignmentID)
	if err != nil {
		t.Fatal(err)
	}
	return op
}

func TestTextureSupervisionBlockAbsentWhenEveryOperationIsTerminal(t *testing.T) {
	ctx := context.Background()
	_, operations, computerID := freezeCandidateTestStore(t)
	op := openSupervisionOperation(t, operations, computerID, "trajectory-failed", "assignment-1")
	if _, err := operations.Transition(ctx, computerID, op.OperationID, op.State, selfdev.StateFailed, nil); err != nil {
		t.Fatal(err)
	}
	rt := &Runtime{selfdevOperations: operations}
	for _, intent := range []string{"apply_owner_revision", "integrate_execution_findings"} {
		if got := rt.runContextForRun(supervisionRun(computerID, "trajectory-failed", intent)); strings.Contains(got, supervisionBlockHeader) || strings.Contains(got, op.OperationID) {
			t.Fatalf("intent %s: a trajectory whose only operation failed was given the supervision block:\n%s", intent, got)
		}
	}
}

func TestTextureSupervisionBlockNamesTheLiveOperationAndItsState(t *testing.T) {
	ctx := context.Background()
	_, operations, computerID := freezeCandidateTestStore(t)
	// An earlier terminal round and a live one: the live one is supervised.
	earlier := openSupervisionOperation(t, operations, computerID, "trajectory-live", "assignment-1")
	if _, err := operations.Transition(ctx, computerID, earlier.OperationID, earlier.State, selfdev.StateFailed, nil); err != nil {
		t.Fatal(err)
	}
	live := openSupervisionOperation(t, operations, computerID, "trajectory-live", "assignment-2")
	rt := &Runtime{selfdevOperations: operations}

	got := rt.runContextForRun(supervisionRun(computerID, "trajectory-live", "integrate_execution_findings"))
	if !strings.Contains(got, supervisionBlockHeader) || !strings.Contains(got, live.OperationID) || strings.Contains(got, earlier.OperationID) {
		t.Fatalf("a live operation was not the one supervised:\n%s", got)
	}
	if !strings.Contains(got, "state "+live.State) {
		t.Fatalf("the block does not name the operation's state %q:\n%s", live.State, got)
	}
	if !strings.Contains(got, "new direction") || !strings.Contains(got, "new round") {
		t.Fatalf("the block does not say that new owner direction is a new round to Cast:\n%s", got)
	}

	// The state is read live, not frozen at selection time.
	if _, err := operations.Transition(ctx, computerID, live.OperationID, live.State, selfdev.StateFrozen, nil); err != nil {
		t.Fatal(err)
	}
	if got := rt.runContextForRun(supervisionRun(computerID, "trajectory-live", "apply_owner_revision")); !strings.Contains(got, "state "+selfdev.StateFrozen) {
		t.Fatalf("the block does not follow the operation to frozen:\n%s", got)
	}
}

func TestTextureSupervisionBlockForOwnerRevisionSaysNewDirectionIsANewRound(t *testing.T) {
	_, operations, computerID := freezeCandidateTestStore(t)
	openSupervisionOperation(t, operations, computerID, "trajectory-revision", "assignment-1")
	rt := &Runtime{selfdevOperations: operations}
	got := rt.runContextForRun(supervisionRun(computerID, "trajectory-revision", "apply_owner_revision"))
	for _, want := range []string{"owner revision", "new round", "Cast", "not a re-run"} {
		if !strings.Contains(got, want) {
			t.Fatalf("an apply_owner_revision run's block lacks %q:\n%s", want, got)
		}
	}
}

// pickTrajectoryOperation keeps its behavior for trajectory-wide callers that
// legitimately want the newest terminal operation (run acceptance reads the
// freeze evidence of a verify-fail trajectory); only the Texture run context
// uses the live-only selector.
func TestTrajectoryOperationSelectors(t *testing.T) {
	failed := selfdev.Operation{OperationID: "op-failed", State: selfdev.StateFailed}
	applied := selfdev.Operation{OperationID: "op-applied", State: selfdev.StateApplied}
	executing := selfdev.Operation{OperationID: "op-executing", State: selfdev.StateExecuting}
	awaiting := selfdev.Operation{OperationID: "op-awaiting", State: selfdev.StateAwaitingApproval}

	if _, ok := pickTrajectoryOperation(nil); ok {
		t.Fatal("pickTrajectoryOperation picked from an empty trajectory")
	}
	if op, ok := pickTrajectoryOperation([]selfdev.Operation{failed, applied}); !ok || op.OperationID != "op-applied" {
		t.Fatalf("pickTrajectoryOperation must keep falling back to the newest terminal operation, got %+v, %v", op, ok)
	}
	if op, ok := pickTrajectoryOperation([]selfdev.Operation{executing, failed}); !ok || op.OperationID != "op-executing" {
		t.Fatalf("pickTrajectoryOperation must still prefer the newest live operation, got %+v, %v", op, ok)
	}

	if _, ok := pickLiveTrajectoryOperation(nil); ok {
		t.Fatal("pickLiveTrajectoryOperation picked from an empty trajectory")
	}
	if op, ok := pickLiveTrajectoryOperation([]selfdev.Operation{failed, applied}); ok {
		t.Fatalf("pickLiveTrajectoryOperation picked a terminal operation: %+v", op)
	}
	if op, ok := pickLiveTrajectoryOperation([]selfdev.Operation{executing, awaiting, failed}); !ok || op.OperationID != "op-awaiting" {
		t.Fatalf("pickLiveTrajectoryOperation must name the newest live operation, got %+v, %v", op, ok)
	}
}
