package agentcore

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// A failed freeze left its operation executing for 17 hours
// (docs/problems/selfdev-freeze-failure-leaves-operation-executing-2026-10-22.md):
// the busy probe counted it, vmctl deferred every refresh, and no owner
// decision could close it. A freeze refusal is retryable while the
// assignment lives (TestFreezeRefusalStampsOperationIntentError pins
// that), so the operation closes when its driver, the assignment, ends.
//
// Ways it can fail (written before the code):
//  1. An assignment reports terminal (completed or failed) before its
//     operation froze, and the operation stays executing: busy forever.
//     Only cancellation closed the operation before this change.
//  2. The close carries no visible fate: the terminal error must name the
//     assignment's end and the last intent error (the freeze refusal).
//  3. The close kills a frozen candidate of a completed assignment, which
//     waits for management's Verify.
//  4. A re-freeze of the same assignment revives the failed operation
//     instead of being refused.
//  5. The busy probe (selfdev_active_operations) still counts it.
//  6. The crash-boot close regresses
//     (TestCrashBootClosesPreDecisionSelfDevelopmentOperations pins it).

func assignmentEndFixture(t *testing.T, owner string) (*Runtime, *store.Store, types.EngineeringAssignment) {
	t.Helper()
	rt, s, _ := c1eRuntime(t)
	operations, err := selfdev.NewStore(s, s)
	if err != nil {
		t.Fatal(err)
	}
	rt.selfdevOperations = operations
	seed, err := store.SeedEngineeringAssignmentAuthority(s, owner, rt.TextureComputerID(), 1)
	if err != nil {
		t.Fatal(err)
	}
	assignment, _ := c1eOpenBound(t, s, seed, 0)
	return rt, s, assignment
}

func seedAssignmentOperation(t *testing.T, rt *Runtime, assignment types.EngineeringAssignment, state, intentError string) string {
	t.Helper()
	operationID := store.EngineeringFreezeOperationID(assignment.Binding.ComputerID, assignment.AssignmentID)
	now := time.Now().UTC()
	if _, err := rt.store.DB().Exec(`INSERT INTO self_development_operations (operation_id, computer_id, idempotency_key, request_commitment, trajectory_id, base_head, prompt_artifact_ref, verifier_refs_json, bundle_digest, desired_head, effective_head, state, last_intent_error, created_at, updated_at) VALUES (?, ?, ?, '', ?, '', '', '[]', ?, '', '', ?, ?, ?, ?)`,
		operationID, assignment.Binding.ComputerID, store.EngineeringFreezeIdempotencyKey(assignment.AssignmentID), assignment.Binding.TrajectoryID,
		map[bool]string{true: strings.Repeat("f", 64), false: ""}[state == selfdev.StateFrozen], state, intentError, now, now); err != nil {
		t.Fatal(err)
	}
	return operationID
}

func terminalAssignmentReport(assignment types.EngineeringAssignment, result types.EngineeringAssignmentResultKind) types.EngineeringAssignmentReport {
	after := objectgraph.SHA256([]byte("changed subject " + assignment.AssignmentID))
	return types.EngineeringAssignmentReport{
		Schema: types.EngineeringAssignmentSchemaV1, ReportID: "report-" + string(result) + "-" + assignment.AssignmentID,
		AssignmentID: assignment.AssignmentID, Attempt: assignment.Binding.Attempt, OwnerID: assignment.Binding.OwnerID,
		ComputerID: assignment.Binding.ComputerID, TrajectoryID: assignment.Binding.TrajectoryID,
		RunID: assignment.BoundRunID, AssignedAgentID: assignment.Binding.AssignedAgentID,
		Result: result, Verdict: types.EngineeringVerdictNone, ObservedSubjectDigest: after,
		CandidateArtifactRef: "capsule-subject:" + after,
		Mutations: []types.EngineeringRecordedMutation{{
			MutationID: "subject-mutation", Kind: "subject_bytes", BeforeDigest: assignment.Binding.SubjectDigest,
			AfterDigest: after, EvidenceRef: "receipt:mutation", SubjectBytesChanged: true,
		}},
		Summary: "the freeze was refused; reporting " + string(result), CreatedAt: time.Now().UTC(),
	}
}

func TestAssignmentEndClosesItsUnfrozenOperation(t *testing.T) {
	const refusal = "freeze: capsule release must contain executable bin/autoputer: the running binary was built from 219535ef"
	for _, result := range []types.EngineeringAssignmentResultKind{types.EngineeringResultCompleted, types.EngineeringResultFailed} {
		t.Run(string(result), func(t *testing.T) {
			ctx := context.Background()
			rt, s, assignment := assignmentEndFixture(t, "owner-end-"+string(result))
			operationID := seedAssignmentOperation(t, rt, assignment, selfdev.StateExecuting, refusal)
			if got := rt.activeSelfdevOperations(ctx); got != 1 {
				t.Fatalf("busy probe before the report = %d, want 1", got)
			}

			committed, err := rt.commitAssignedEngineeringReport(ctx, c1eGet(t, s, assignment), terminalAssignmentReport(assignment, result))
			if err != nil {
				t.Fatalf("terminal report: %v", err)
			}
			if !committed.Assignment.Disposition.Terminal() {
				t.Fatalf("assignment after the terminal report = %s", committed.Assignment.Disposition)
			}

			operation, err := rt.selfdevOperations.Get(ctx, assignment.Binding.ComputerID, operationID)
			if err != nil {
				t.Fatal(err)
			}
			if operation.State != selfdev.StateFailed {
				t.Fatalf("operation after its assignment ended = %s, want failed (item 1)", operation.State)
			}
			if !strings.Contains(operation.TerminalError, "before its candidate froze") || !strings.Contains(operation.TerminalError, refusal) {
				t.Fatalf("terminal error = %q; want the assignment's end and the freeze refusal (item 2)", operation.TerminalError)
			}
			if got := rt.activeSelfdevOperations(ctx); got != 0 {
				t.Fatalf("busy probe after the close = %d, want 0 (item 5)", got)
			}

			// Item 4: the same assignment's candidate is never revived.
			reopened, err := openFreezeCandidate(ctx, rt.selfdevOperations, func(context.Context) error { return nil },
				assignment.Binding.ComputerID, assignment.Binding.TrajectoryID, assignment.AssignmentID, "retry")
			if err != nil {
				t.Fatalf("reopen: %v", err)
			}
			if reopened.OperationID != operationID || reopened.State != selfdev.StateFailed {
				t.Fatalf("a re-freeze revived the failed operation: %+v", reopened)
			}

			// A replay of the same report converges and writes nothing new.
			if _, err := rt.commitAssignedEngineeringReport(ctx, c1eGet(t, s, assignment), terminalAssignmentReport(assignment, result)); err == nil {
				again, _ := rt.selfdevOperations.Get(ctx, assignment.Binding.ComputerID, operationID)
				if again.State != selfdev.StateFailed || again.TerminalError != operation.TerminalError {
					t.Fatalf("a replayed report changed the closed operation: %+v", again)
				}
			}
		})
	}
}

func TestCompletedAssignmentKeepsItsFrozenCandidate(t *testing.T) {
	ctx := context.Background()
	rt, s, assignment := assignmentEndFixture(t, "owner-end-frozen")
	operationID := seedAssignmentOperation(t, rt, assignment, selfdev.StateFrozen, "")
	if _, err := rt.commitAssignedEngineeringReport(ctx, c1eGet(t, s, assignment), terminalAssignmentReport(assignment, types.EngineeringResultCompleted)); err != nil {
		t.Fatalf("terminal report: %v", err)
	}
	operation, err := rt.selfdevOperations.Get(ctx, assignment.Binding.ComputerID, operationID)
	if err != nil {
		t.Fatal(err)
	}
	if operation.State != selfdev.StateFrozen {
		t.Fatalf("a completed assignment's frozen candidate = %s, want frozen for management's Verify (item 3)", operation.State)
	}
}

func TestFailedAssignmentClosesItsFrozenCandidate(t *testing.T) {
	ctx := context.Background()
	rt, s, assignment := assignmentEndFixture(t, "owner-end-failed-frozen")
	operationID := seedAssignmentOperation(t, rt, assignment, selfdev.StateFrozen, "")
	if _, err := rt.commitAssignedEngineeringReport(ctx, c1eGet(t, s, assignment), terminalAssignmentReport(assignment, types.EngineeringResultFailed)); err != nil {
		t.Fatalf("terminal report: %v", err)
	}
	operation, err := rt.selfdevOperations.Get(ctx, assignment.Binding.ComputerID, operationID)
	if err != nil {
		t.Fatal(err)
	}
	// Management verifies only a completed assignment's candidate, so a
	// failed assignment's frozen candidate can never advance.
	if operation.State != selfdev.StateFailed || !strings.Contains(operation.TerminalError, "assignment failed") {
		t.Fatalf("a failed assignment's frozen candidate = %s %q, want failed", operation.State, operation.TerminalError)
	}
}
