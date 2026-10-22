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

			// A replay of the same report writes nothing new to the closed
			// operation. The outer replay path that must also close it is
			// TestReplayedReportClosesTheOperationAfterACrashBeforeTheClose.
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

// Review bunch 15, F1/F4: a crash between the report's commit and the
// close leaves a terminal assignment with a stored report and an executing
// operation. The re-delivered report takes the outer replay path
// (recordAssignedEngineeringReportOnce returns the stored receipt early),
// which must still close the operation: nothing else re-drives a terminal
// assignment on a planned boot.
func TestReplayedReportClosesTheOperationAfterACrashBeforeTheClose(t *testing.T) {
	for _, result := range []types.EngineeringAssignmentResultKind{types.EngineeringResultCompleted, types.EngineeringResultFailed} {
		t.Run(string(result), func(t *testing.T) {
			ctx := context.Background()
			rt, s, _ := c1eRuntime(t)
			operations, err := selfdev.NewStore(s, s)
			if err != nil {
				t.Fatal(err)
			}
			rt.selfdevOperations = operations
			seed, err := store.SeedEngineeringAssignmentAuthority(s, "owner-replay-"+string(result), rt.TextureComputerID(), 1)
			if err != nil {
				t.Fatal(err)
			}
			assignment, run := c1eOpenBound(t, s, seed, 0)
			operationID := seedAssignmentOperation(t, rt, assignment, selfdev.StateExecuting, "freeze: refused")

			// The report as the outer path derives it: pinned subject,
			// proposition digest, slot report id.
			report := terminalAssignmentReport(assignment, result)
			report.ObservedSubjectDigest = assignment.Binding.SubjectDigest
			report.CandidateArtifactRef = ""
			report.Mutations[0].AfterDigest = assignment.Binding.SubjectDigest
			report.Mutations[0].SubjectBytesChanged = false
			report.PropositionDigest, err = store.ComputeTerminalPropositionDigest(assignment.Binding.SubjectDigest, report.Result, report.Verdict, report.Commands, report.Outputs, report.EvidenceRefs)
			if err != nil {
				t.Fatal(err)
			}
			report.ReportID = store.TerminalReportID(assignment.Binding.OwnerID, assignment.Binding.ComputerID, assignment.AssignmentID, assignment.Binding.Attempt, report.PropositionDigest)

			// The crash window: the store commits the terminal report; the
			// close never runs.
			req := types.RecordEngineeringAssignmentReportRequest{
				CommandID: "engineering-report:" + assignment.AssignmentID + ":" + report.ReportID,
				OwnerID:   assignment.Binding.OwnerID, ComputerID: assignment.Binding.ComputerID,
				AssignmentID: assignment.AssignmentID, Attempt: assignment.Binding.Attempt,
				ExpectedLifecycleVersion: assignment.LifecycleVersion, Report: report,
			}
			req.CommandDigest, _ = store.ComputeRecordEngineeringAssignmentReportDigest(req)
			committed, err := s.RecordEngineeringAssignmentReport(ctx, req)
			if err != nil || !committed.Assignment.Disposition.Terminal() {
				t.Fatalf("store report = %+v, %v", committed.Assignment.Disposition, err)
			}
			if before, _ := operations.Get(ctx, assignment.Binding.ComputerID, operationID); before.State != selfdev.StateExecuting {
				t.Fatalf("setup: operation = %s before the replay", before.State)
			}

			if _, err := rt.recordAssignedEngineeringReport(ctx, &run, "tool-call:replay", report); err != nil {
				t.Fatalf("replayed report: %v", err)
			}
			operation, err := operations.Get(ctx, assignment.Binding.ComputerID, operationID)
			if err != nil {
				t.Fatal(err)
			}
			if operation.State != selfdev.StateFailed || !strings.Contains(operation.TerminalError, "before its candidate froze") {
				t.Fatalf("operation after the replayed report = %s %q, want failed", operation.State, operation.TerminalError)
			}
			if got := rt.activeSelfdevOperations(ctx); got != 0 {
				t.Fatalf("busy probe after the replay = %d, want 0", got)
			}
		})
	}
}
