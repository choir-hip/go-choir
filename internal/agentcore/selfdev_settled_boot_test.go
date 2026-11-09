package agentcore

import (
	"context"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/selfdev"
)

// Failure modes of the boot catch-up: (1) a settled trajectory's frozen
// candidate stays open because the observer never ran; (2) a live
// trajectory's candidate is closed; (3) an awaiting_approval candidate (the
// owner's decision) is closed.
func TestBootClosesSettledTrajectoryCandidatesOnly(t *testing.T) {
	ctx := context.Background()
	f := newRebaseFallbackFixture(t, true)
	rt := f.rt
	operations, err := selfdev.NewStore(f.s, f.s)
	if err != nil {
		t.Fatal(err)
	}
	rt.selfdevOperations = operations
	now := time.Now().UTC()
	seed := func(id, state string) {
		t.Helper()
		if _, err := f.s.DB().Exec(`INSERT INTO self_development_operations (operation_id, computer_id, idempotency_key, request_commitment, trajectory_id, base_head, prompt_artifact_ref, verifier_refs_json, bundle_digest, desired_head, effective_head, state, last_intent_error, created_at, updated_at) VALUES (?, ?, ?, '', ?, '', '', '[]', '', '', '', ?, '', ?, ?)`,
			id, f.computer, "key-"+id, armTrajectory, state, now, now); err != nil {
			t.Fatal(err)
		}
	}
	seed("op-frozen-boot", selfdev.StateFrozen)
	seed("op-awaiting-boot", selfdev.StateAwaitingApproval)
	read := func(id string) selfdev.Operation {
		t.Helper()
		op, err := rt.selfdevOperations.Get(ctx, f.computer, id)
		if err != nil {
			t.Fatal(err)
		}
		return op
	}
	rt.closeSettledTrajectoryCandidatesAtBoot(ctx, f.computer)
	if op := read("op-frozen-boot"); op.State != selfdev.StateFrozen {
		t.Fatalf("a live trajectory's candidate = %s, want frozen", op.State)
	}
	settleArmTrajectory(t, f)
	rt.closeSettledTrajectoryCandidatesAtBoot(ctx, f.computer)
	if op := read("op-frozen-boot"); op.State != selfdev.StateFailed || op.TerminalError != selfdevSettledTrajectoryCause {
		t.Fatalf("a settled trajectory's frozen candidate = %s %q, want failed", op.State, op.TerminalError)
	}
	if op := read("op-awaiting-boot"); op.State != selfdev.StateAwaitingApproval {
		t.Fatalf("an awaiting_approval candidate = %s, want left to the owner", op.State)
	}
}

// A candidate that moved to its owner's decision between the list and the
// close is not failed: the close re-reads under a {frozen, verified} set.
func TestSettledCloseLeavesACandidateThatMovedToDecision(t *testing.T) {
	ctx := context.Background()
	rt, _, assignment := assignmentEndFixture(t, "owner-settled-race")
	id := seedAssignmentOperation(t, rt, assignment, selfdev.StateVerified, "")
	stale, err := rt.selfdevOperations.Get(ctx, assignment.Binding.ComputerID, id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rt.store.DB().Exec(`UPDATE self_development_operations SET state = ? WHERE operation_id = ?`, selfdev.StateAwaitingApproval, id); err != nil {
		t.Fatal(err)
	}
	rt.failSelfdevOperationFrom(ctx, stale, selfdevSettledTrajectoryCause, selfdev.StateFrozen, selfdev.StateVerified)
	if op, _ := rt.selfdevOperations.Get(ctx, assignment.Binding.ComputerID, id); op.State != selfdev.StateAwaitingApproval {
		t.Fatalf("a candidate that reached its decision = %s, want awaiting_approval", op.State)
	}
	// A verified one closes.
	if _, err := rt.store.DB().Exec(`UPDATE self_development_operations SET state = ? WHERE operation_id = ?`, selfdev.StateVerified, id); err != nil {
		t.Fatal(err)
	}
	rt.failSelfdevOperationFrom(ctx, stale, selfdevSettledTrajectoryCause, selfdev.StateFrozen, selfdev.StateVerified)
	if op, _ := rt.selfdevOperations.Get(ctx, assignment.Binding.ComputerID, id); op.State != selfdev.StateFailed {
		t.Fatalf("a verified candidate = %s, want failed", op.State)
	}
}
