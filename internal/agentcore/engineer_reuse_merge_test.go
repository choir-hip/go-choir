package agentcore

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Design A6: the retained-candidate fix and engineer reuse together. Ways
// it can fail:
//  1. A build that ends during rework retains revision 1 (failed) instead
//     of the revision it froze, and fails that revision.
//  2. The crash boot's orphan sweep misreads a revision key and leaves a
//     frozen revision open, pinning the one-open-operation rule.

// seedRejectedThenFrozen writes revision 1 failed by management's
// judgment and revision 2 frozen, and returns revision 2's id.
func seedRejectedThenFrozen(t *testing.T, rt *Runtime, assignment types.EngineeringAssignment) string {
	t.Helper()
	now := time.Now().UTC()
	insert := func(rev int, state, terminal, verifiers, bundle string) string {
		id := store.EngineeringFreezeRevisionOperationID(assignment.Binding.ComputerID, assignment.AssignmentID, rev)
		if _, err := rt.store.DB().Exec(`INSERT INTO self_development_operations (operation_id, computer_id, idempotency_key, request_commitment, trajectory_id, base_head, prompt_artifact_ref, verifier_refs_json, bundle_digest, desired_head, effective_head, state, last_intent_error, terminal_error, created_at, updated_at) VALUES (?, ?, ?, '', ?, '', '', ?, ?, '', '', ?, '', ?, ?, ?)`,
			id, assignment.Binding.ComputerID, store.EngineeringFreezeRevisionIdempotencyKey(assignment.AssignmentID, rev), assignment.Binding.TrajectoryID,
			verifiers, bundle, state, terminal, now, now); err != nil {
			t.Fatal(err)
		}
		return id
	}
	insert(1, selfdev.StateFailed, selfDevelopmentJudgmentRejected+": the chart is blank", `["sha256:`+strings.Repeat("9", 64)+`"]`, strings.Repeat("1", 64))
	return insert(2, selfdev.StateFrozen, "", "[]", strings.Repeat("2", 64))
}

func TestReuseMergeBuildEndRetainsTheCurrentRevision(t *testing.T) {
	ctx := withBuildBudgetEnd(context.Background())
	rt, s, assignment, run := retainedFixture(t, "owner-reuse-retain")
	rev2 := seedRejectedThenFrozen(t, rt, assignment)
	if _, err := rt.cancelBoundEngineeringRun(ctx, run, activationBudgetExceededReason); err != nil {
		t.Fatalf("budget cancel: %v", err)
	}
	if got := c1eGet(t, s, assignment); got.Disposition != types.EngineeringAssignmentCancelled || !strings.Contains(got.DispositionReason, rev2) {
		t.Fatalf("assignment = %s %q; want cancelled naming revision 2", got.Disposition, got.DispositionReason)
	}
	op := operationState(t, rt, assignment.Binding.ComputerID, rev2)
	if op.State != selfdev.StateFrozen {
		t.Fatalf("revision 2 after its build's budget = %s %q; want retained frozen", op.State, op.TerminalError)
	}
	if _, retained := rt.retainedFor(op, assignment.AssignmentID); !retained {
		t.Fatal("revision 2 is not the retained candidate")
	}
}

func TestReuseMergeCrashBootClosesARevisionOrphan(t *testing.T) {
	ctx := context.Background()
	rt, _, assignment, run := retainedFixture(t, "owner-reuse-orphan")
	rev2 := seedRejectedThenFrozen(t, rt, assignment)
	keep := rt.selfdevOperations
	rt.selfdevOperations = nil // the cancel's close does not land
	if _, err := rt.cancelBoundEngineeringRun(ctx, run, "persistent Management cancelled assignment"); err != nil {
		t.Fatal(err)
	}
	rt.selfdevOperations = keep
	rt.closeRetainedCandidatesAtCrashBoot(ctx)
	if op := operationState(t, rt, assignment.Binding.ComputerID, rev2); op.State != selfdev.StateFailed {
		t.Fatalf("revision 2 orphan after a crash boot = %s; want failed", op.State)
	}
}

// A rejection in review settles nothing; when the round ends on it, it is
// the final verdict: verify_pass, judge_accepts and owner_approves settle
// "no" (merge review, both seats).
func TestReuseMergeRejectionInReviewSettlesOnlyAtTheRoundsEnd(t *testing.T) {
	f := newRebaseCastFixture(t)
	ctx := context.Background()
	plan, err := f.plan(t, f.spec(nil), f.rebaseObjective())
	if err != nil {
		t.Fatal(err)
	}
	records := types.CommitInboxRecordsRequest{OwnerID: f.owner, ComputerID: f.computer, CommandID: "cell-records:" + plan.AssignmentID, Records: []types.CommitmentRecord{plan.Cast}}
	records.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(records)
	if _, err := f.rt.openDelegatedCastAssignment(ctx, plan, records); err != nil {
		t.Fatalf("open: %v", err)
	}
	if f.rt.selfdevOperations == nil {
		t.Fatal("fixture has no operation store")
	}
	assignment := mustAssignment(t, f, plan.AssignmentID)
	seedRejectedThenFrozen(t, f.rt, assignment)
	// Revision 2 ends too, so revision 2 is current and failed by another
	// cause; drop it to leave revision 1's rejection as the current one.
	if _, err := f.rt.store.DB().Exec(`DELETE FROM self_development_operations WHERE operation_id = ?`,
		store.EngineeringFreezeRevisionOperationID(assignment.Binding.ComputerID, assignment.AssignmentID, 2)); err != nil {
		t.Fatal(err)
	}
	verdictOf := func(kind types.PrecommitRuleKind) string {
		stated, err := f.s.ListRulePrecommitRecords(ctx, f.owner, f.computer, plan.AssignmentID)
		if err != nil {
			t.Fatal(err)
		}
		for _, rec := range stated {
			if rec.Precommit.Rule.Kind == kind {
				resolve, _ := f.s.GetCommitmentRecord(ctx, f.owner, f.computer, types.RuleResolveRecordID(rec.RecordID))
				if resolve == nil || resolve.Resolve == nil {
					return ""
				}
				return resolve.Resolve.Verdict
			}
		}
		t.Fatalf("no %s precommit", kind)
		return ""
	}
	for _, kind := range []types.PrecommitRuleKind{types.PrecommitRuleVerifyPass, types.PrecommitRuleJudgeAccepts} {
		if got := verdictOf(kind); got != "" {
			t.Fatalf("%s settled %q before the round ended", kind, got)
		}
	}
	// Completed: without the final-verdict branch the terminal pass would
	// leave verify_pass and judge_accepts open waiting for a Verify that
	// can never land on the failed revision (re-review, Devin).
	ended := assignment
	ended.Disposition = types.EngineeringAssignmentCompleted
	f.rt.settleEngineeringPrecommitsAtTerminal(ctx, ended, "completed without a new candidate")
	for _, kind := range []types.PrecommitRuleKind{types.PrecommitRuleVerifyPass, types.PrecommitRuleJudgeAccepts, types.PrecommitRuleOwnerApproves} {
		if got := verdictOf(kind); got != "no" {
			t.Fatalf("%s after the round ended on the rejection = %q, want no", kind, got)
		}
	}
}

// Design A6 rule 4: a late report on the cancelled assignment does not
// close its retained candidate.
func TestReuseMergeLateReportKeepsTheRetainedCandidate(t *testing.T) {
	ctx := withBuildBudgetEnd(context.Background())
	rt, s, assignment, run := retainedFixture(t, "owner-reuse-late")
	rev2 := seedRejectedThenFrozen(t, rt, assignment)
	if _, err := rt.cancelBoundEngineeringRun(ctx, run, activationBudgetExceededReason); err != nil {
		t.Fatal(err)
	}
	rt.closeEndedAssignmentSelfdevOperation(context.Background(), c1eGet(t, s, assignment))
	if op := operationState(t, rt, assignment.Binding.ComputerID, rev2); op.State != selfdev.StateFrozen {
		t.Fatalf("retained revision 2 after a late close = %s %q; want frozen", op.State, op.TerminalError)
	}
}

// The per-shot review flips management's pass to a fail inside Verify; the
// waiting engineer resumes as a rejection carrying the per-shot reasons,
// not as an ended candidate (coordinator, 2026-11-04).
func TestReuseMergeShotReviewFlipResumesTheEngineerWithItsReasons(t *testing.T) {
	ctx := context.Background()
	_, operations, computerID := freezeCandidateTestStore(t)
	const assignment = "assignment-flip"
	op := reviewTestFrozen(t, operations, computerID, "trajectory-flip", assignment, strings.Repeat("7", 64))
	rt := &Runtime{selfdevOperations: operations}
	rec := &types.RunRecord{RunID: "run-flip"}
	rt.reviews.Store(rec.RunID, &candidateReview{computerID: computerID, assignmentID: assignment, operationID: op.OperationID, revision: 1})
	flipped := selfDevelopmentJudgmentRejected + ": " + shotReviewMarker + " rejected 1 of 3 shots: 04-agenda.png Q3 (the week grid is cut off)"
	reviewTestFail(t, operations, computerID, op, flipped)
	got, ok, err := rt.engineeringReviewPark(ctx, rec, nil)
	if err != nil || !ok || got.Reason != reviewParkRejected || got.Terminal {
		t.Fatalf("park after a flipped pass = %+v ok %v err %v; want a rejection", got, ok, err)
	}
	if !strings.Contains(got.Instruction, "04-agenda.png Q3 (the week grid is cut off)") || !strings.Contains(got.Instruction, "Freeze again") {
		t.Fatalf("resume lacks the per-shot reasons: %q", got.Instruction)
	}
}
