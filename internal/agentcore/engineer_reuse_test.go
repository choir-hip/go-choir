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

// Engineer reuse, phase A (docs/design/engineer-reuse-2026-11-04.md). The
// ways it can fail, from the design's failure-mode list:
//  1. A rejected candidate cannot be followed by another Freeze of the same
//     assignment (the operation key is one per assignment).
//  2. A candidate that failed for another cause (cancel, apply) opens a new
//     revision, so the rework outlives the round's own end.
//  3. Revisions run past the bound.
//  4. A reader resolves an older revision, so Verify, the carry or the close
//     acts on the wrong candidate.
//  5. management verifies an open assignment that did not ask for review.
//  6. The review Report does not name its candidate, so Verify cannot.
//  7. The park misses a verdict that landed before it looked, or resumes
//     with no reason.

func reviewTestFrozen(t *testing.T, operations *selfdev.Store, computerID, trajectory, assignmentID string, revisionBundle string) selfdev.Operation {
	t.Helper()
	ctx := context.Background()
	authorized := func(context.Context) error { return nil }
	op, err := openFreezeCandidate(ctx, operations, authorized, computerID, trajectory, assignmentID, "make it")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	frozen, err := operations.Transition(ctx, computerID, op.OperationID, selfdev.StateExecuting, selfdev.StateFrozen, func(next *selfdev.Operation) error {
		next.BundleDigest = revisionBundle
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return frozen
}

func reviewTestFail(t *testing.T, operations *selfdev.Store, computerID string, op selfdev.Operation, terminalError string) selfdev.Operation {
	t.Helper()
	failed, err := operations.Transition(context.Background(), computerID, op.OperationID, selfdev.StateFrozen, selfdev.StateFailed, func(next *selfdev.Operation) error {
		next.TerminalError = terminalError
		if strings.HasPrefix(terminalError, selfDevelopmentJudgmentRejected) {
			next.VerifierRefs = []string{"sha256:" + strings.Repeat("9", 64)}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return failed
}

func TestEngineerReuseRevisionsFollowOnlyAJudgmentRejection(t *testing.T) {
	ctx := context.Background()
	_, operations, computerID := freezeCandidateTestStore(t)
	authorized := func(context.Context) error { return nil }
	const assignment = "assignment-reuse"
	first := reviewTestFrozen(t, operations, computerID, "trajectory-reuse", assignment, strings.Repeat("1", 64))
	if first.OperationID != store.EngineeringFreezeOperationID(computerID, assignment) || first.IdempotencyKey != store.EngineeringFreezeIdempotencyKey(assignment) {
		t.Fatalf("revision 1 changed identity: %+v", first)
	}
	// A frozen revision is returned, not followed.
	if again, err := openFreezeCandidate(ctx, operations, authorized, computerID, "trajectory-reuse", assignment, "make it"); err != nil || again.OperationID != first.OperationID {
		t.Fatalf("open while frozen = %+v, %v", again, err)
	}
	reviewTestFail(t, operations, computerID, first, selfDevelopmentJudgmentRejected+": the button is blank")
	second, err := openFreezeCandidate(ctx, operations, authorized, computerID, "trajectory-reuse", assignment, "make it")
	if err != nil {
		t.Fatalf("open after a rejection: %v", err)
	}
	if second.OperationID == first.OperationID || second.OperationID != store.EngineeringFreezeRevisionOperationID(computerID, assignment, 2) ||
		second.IdempotencyKey != store.EngineeringFreezeRevisionIdempotencyKey(assignment, 2) || second.State != selfdev.StateExecuting {
		t.Fatalf("revision 2 = %+v", second)
	}
	if got, rev, found, err := assignmentOperation(ctx, operations, computerID, assignment); err != nil || !found || rev != 2 || got.OperationID != second.OperationID {
		t.Fatalf("resolver = %+v rev %d found %v err %v; want revision 2", got, rev, found, err)
	}
	if id, ok := store.EngineeringFreezeAssignmentOf(second.IdempotencyKey); !ok || id != assignment {
		t.Fatalf("assignment of %q = %q %v", second.IdempotencyKey, id, ok)
	}
	if !store.IsEngineeringFreezeKeyOf(second.IdempotencyKey, assignment) || store.IsEngineeringFreezeKeyOf(second.IdempotencyKey, "assignment-other") {
		t.Fatal("IsEngineeringFreezeKeyOf misreads a revision key")
	}

	// Revision 2 failed for another cause: no revision 3.
	frozen2, err := operations.Transition(ctx, computerID, second.OperationID, selfdev.StateExecuting, selfdev.StateFrozen, func(next *selfdev.Operation) error {
		next.BundleDigest = strings.Repeat("2", 64)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	reviewTestFail(t, operations, computerID, frozen2, "assignment cancelled")
	if got, err := openFreezeCandidate(ctx, operations, authorized, computerID, "trajectory-reuse", assignment, "make it"); err != nil || got.OperationID != second.OperationID || got.State != selfdev.StateFailed {
		t.Fatalf("open after a non-judgment failure = %+v, %v; want revision 2, failed", got, err)
	}
}

func TestEngineerReuseRevisionsStopAtTheBound(t *testing.T) {
	ctx := context.Background()
	_, operations, computerID := freezeCandidateTestStore(t)
	authorized := func(context.Context) error { return nil }
	const assignment = "assignment-bound"
	var last selfdev.Operation
	for n := 1; n <= store.EngineeringFreezeMaxRevisions; n++ {
		op, err := openFreezeCandidate(ctx, operations, authorized, computerID, "trajectory-bound", assignment, "make it")
		if err != nil {
			t.Fatalf("revision %d: %v", n, err)
		}
		if op.OperationID != store.EngineeringFreezeRevisionOperationID(computerID, assignment, n) {
			t.Fatalf("revision %d id = %s", n, op.OperationID)
		}
		frozen, err := operations.Transition(ctx, computerID, op.OperationID, selfdev.StateExecuting, selfdev.StateFrozen, func(next *selfdev.Operation) error {
			next.BundleDigest = strings.Repeat(string(rune('a'+n)), 64)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		last = reviewTestFail(t, operations, computerID, frozen, selfDevelopmentJudgmentRejected+": no")
	}
	got, err := openFreezeCandidate(ctx, operations, authorized, computerID, "trajectory-bound", assignment, "make it")
	if err != nil || got.OperationID != last.OperationID || got.State != selfdev.StateFailed {
		t.Fatalf("open past the bound = %+v, %v; want the last revision, failed", got, err)
	}
	if freezeOpensNextRevision(last, store.EngineeringFreezeMaxRevisions) {
		t.Fatal("the bound's last revision opens another")
	}
}

// The close at an assignment's end, and the cancellation choke point, act
// on the current revision, never an older one.
func TestEngineerReuseClosersActOnTheCurrentRevision(t *testing.T) {
	ctx := context.Background()
	_, operations, computerID := freezeCandidateTestStore(t)
	const assignment = "assignment-close"
	first := reviewTestFrozen(t, operations, computerID, "trajectory-close", assignment, strings.Repeat("1", 64))
	reviewTestFail(t, operations, computerID, first, selfDevelopmentJudgmentRejected+": no")
	second := reviewTestFrozen(t, operations, computerID, "trajectory-close", assignment, strings.Repeat("2", 64))
	rt := &Runtime{selfdevOperations: operations}
	rt.failBoundSelfdevOperation(ctx, computerID, assignment, "cancelled")
	got, err := operations.Get(ctx, computerID, second.OperationID)
	if err != nil || got.State != selfdev.StateFailed || got.TerminalError != "cancelled" {
		t.Fatalf("current revision after cancel = %+v, %v", got, err)
	}
	if old, _ := operations.Get(ctx, computerID, first.OperationID); !strings.HasPrefix(old.TerminalError, selfDevelopmentJudgmentRejected) {
		t.Fatalf("the cancel rewrote revision 1: %+v", old)
	}
}

// A review Report names its candidate, and only the review Report the
// platform minted for exactly the current operation admits Verify on the
// open assignment.
func TestEngineerReuseReviewAdmitsVerifyOnlyForItsCandidate(t *testing.T) {
	rt, s, _ := c1eRuntime(t)
	ctx := context.Background()
	seed, err := store.SeedEngineeringAssignmentAuthority(s, "owner-reuse-review", rt.TextureComputerID(), 1)
	if err != nil {
		t.Fatal(err)
	}
	assignment, run := c1eOpenBound(t, s, seed, 0)
	operations, err := selfdev.NewStore(s, s)
	if err != nil {
		t.Fatal(err)
	}
	rt.selfdevOperations = operations
	op := selfdev.Operation{OperationID: "selfdev-review-1", State: selfdev.StateFrozen, BundleDigest: strings.Repeat("d", 64)}
	if rt.assignmentInReview(ctx, assignment, op) {
		t.Fatal("an assignment that asked for no review is in review")
	}
	// An engineer's own partial naming review: is not the platform's mark.
	own := types.EngineeringAssignmentReport{Result: types.EngineeringResultPartial, Verdict: types.EngineeringVerdictNone, Summary: "progress",
		EvidenceRefs: withoutReviewRefs([]string{"review:" + op.OperationID, "note:x"})}
	if len(own.EvidenceRefs) != 1 {
		t.Fatalf("withoutReviewRefs kept %v", own.EvidenceRefs)
	}
	if _, err := rt.recordAssignedEngineeringReport(ctx, &run, "tool-call:own", own); err != nil {
		t.Fatal(err)
	}
	if rt.assignmentInReview(ctx, assignment, op) {
		t.Fatal("an engineer's own progress Report admitted Verify")
	}
	review := types.EngineeringAssignmentReport{Result: types.EngineeringResultPartial, Verdict: types.EngineeringVerdictNone,
		Summary: reviewSummaryPrefix + " (candidate 1 of 6)", EvidenceRefs: []string{reviewEvidencePrefix + op.OperationID}}
	if _, err := rt.recordAssignedEngineeringReport(ctx, &run, reviewToolCallID(run.RunID, op.OperationID), review); err != nil {
		t.Fatalf("review Report: %v", err)
	}
	if !rt.assignmentInReview(ctx, assignment, op) {
		t.Fatal("the review Report did not admit Verify on its candidate")
	}
	other := op
	other.OperationID = "selfdev-review-2"
	if rt.assignmentInReview(ctx, assignment, other) {
		t.Fatal("a review Report admitted Verify on another candidate")
	}
	notFrozen := op
	notFrozen.State = selfdev.StateFailed
	if rt.assignmentInReview(ctx, assignment, notFrozen) {
		t.Fatal("a failed candidate is in review")
	}
	var reviewRec *types.CommitmentRecord
	for _, rec := range c1eFromEngineering(t, s, assignment) {
		if rec.Final {
			t.Fatalf("a review Report is final: %+v", rec)
		}
		if strings.HasPrefix(rec.Prediction.Hypothesis, reviewSummaryPrefix) {
			r := rec
			reviewRec = &r
		}
	}
	if reviewRec == nil || reviewOperationFromRefs(reviewRec.EvidenceRefs) != "" && reviewOperationFromRefs(reviewRec.EvidenceRefs) != op.OperationID {
		t.Fatalf("review record = %+v", reviewRec)
	}
	if got := c1eGet(t, s, assignment); got.Disposition.Terminal() {
		t.Fatalf("a review Report ended the assignment: %s", got.Disposition)
	}
}

// The park reads durable state before it waits, so a verdict that landed
// first resumes the turn at once with management's reason; a verdict that
// lands while it waits wakes it.
func TestEngineerReuseParkResumesWithTheRejection(t *testing.T) {
	for _, order := range []string{"verdict-first", "verdict-while-parked"} {
		t.Run(order, func(t *testing.T) {
			ctx := context.Background()
			_, operations, computerID := freezeCandidateTestStore(t)
			const assignment = "assignment-park"
			op := reviewTestFrozen(t, operations, computerID, "trajectory-park", assignment, strings.Repeat("3", 64))
			rt := &Runtime{selfdevOperations: operations}
			rec := &types.RunRecord{RunID: "run-park"}
			rt.reviews.Store(rec.RunID, &candidateReview{computerID: computerID, assignmentID: assignment, operationID: op.OperationID, revision: 1, at: time.Now()})
			reject := func() {
				reviewTestFail(t, operations, computerID, op, selfDevelopmentJudgmentRejected+": the chart has no axis labels")
				rt.signalReviewWake(assignment)
			}
			if order == "verdict-first" {
				reject()
			} else {
				go func() { time.Sleep(50 * time.Millisecond); reject() }()
			}
			wctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			got, ok, err := rt.engineeringReviewPark(wctx, rec, nil)
			if err != nil || !ok || !got.Continue || got.Terminal || got.Reason != reviewParkRejected {
				t.Fatalf("park = %+v ok %v err %v", got, ok, err)
			}
			if !strings.Contains(got.Instruction, "the chart has no axis labels") || !strings.Contains(got.Instruction, "Freeze again") || !strings.Contains(got.Instruction, "candidate 2") {
				t.Fatalf("resume instruction = %q", got.Instruction)
			}
			if _, still := rt.reviews.Load(rec.RunID); still {
				t.Fatal("the review outlived its park")
			}
			// No review kept: the park does not apply.
			if _, ok, _ := rt.engineeringReviewPark(wctx, rec, nil); ok {
				t.Fatal("a park with no review applied")
			}
		})
	}
}

// A candidate that ends for another cause resumes the turn with the fact;
// a cancelled run ends the park with its context error.
func TestEngineerReuseParkOtherEnds(t *testing.T) {
	ctx := context.Background()
	_, operations, computerID := freezeCandidateTestStore(t)
	const assignment = "assignment-park-end"
	op := reviewTestFrozen(t, operations, computerID, "trajectory-park-end", assignment, strings.Repeat("4", 64))
	rt := &Runtime{selfdevOperations: operations}
	rec := &types.RunRecord{RunID: "run-park-end"}
	rt.reviews.Store(rec.RunID, &candidateReview{computerID: computerID, assignmentID: assignment, operationID: op.OperationID, revision: 1})
	cctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, ok, err := rt.engineeringReviewPark(cctx, rec, nil); ok || err == nil {
		t.Fatalf("cancelled park = ok %v err %v", ok, err)
	}
	rt.reviews.Store(rec.RunID, &candidateReview{computerID: computerID, assignmentID: assignment, operationID: op.OperationID, revision: 1})
	reviewTestFail(t, operations, computerID, op, "assignment cancelled")
	got, ok, err := rt.engineeringReviewPark(ctx, rec, nil)
	if err != nil || !ok || got.Reason != reviewParkEnded || !strings.Contains(got.Instruction, "assignment cancelled") {
		t.Fatalf("park after another end = %+v ok %v err %v", got, ok, err)
	}
}

// A close reason that begins like a verdict is not one: no verification
// event was stamped, so no revision opens.
func TestEngineerReuseOnlyAVerdictIsARejection(t *testing.T) {
	op := selfdev.Operation{State: selfdev.StateFailed, TerminalError: selfDevelopmentJudgmentRejected + ": typed by hand"}
	if judgmentRejected(op) || freezeOpensNextRevision(op, 1) {
		t.Fatal("a close without a verification event counts as a rejection")
	}
	op.VerifierRefs = []string{"sha256:" + strings.Repeat("9", 64)}
	if !judgmentRejected(op) || !freezeOpensNextRevision(op, 1) {
		t.Fatal("a verdict's rejection does not open the next revision")
	}
}

// A candidate management passed while its engineer was parked stays for the
// owner when the run ends before its Complete commits; a frozen one closes.
func TestEngineerReuseCancelKeepsAPassedCandidate(t *testing.T) {
	ctx := context.Background()
	_, operations, computerID := freezeCandidateTestStore(t)
	const assignment = "assignment-passed"
	op := reviewTestFrozen(t, operations, computerID, "trajectory-passed", assignment, strings.Repeat("5", 64))
	verified, err := operations.Transition(ctx, computerID, op.OperationID, selfdev.StateFrozen, selfdev.StateVerified, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := operations.Transition(ctx, computerID, verified.OperationID, selfdev.StateVerified, selfdev.StateAwaitingApproval, nil); err != nil {
		t.Fatal(err)
	}
	rt := &Runtime{selfdevOperations: operations}
	rt.failBoundSelfdevOperation(ctx, computerID, assignment, "activation budget")
	if got, _ := operations.Get(ctx, computerID, op.OperationID); got.State != selfdev.StateAwaitingApproval {
		t.Fatalf("a passed candidate was closed by its run's end: %+v", got)
	}
}

// A passed candidate resumes nothing: the park commits the kept Complete,
// and when that commit cannot run, the turn resumes with the error and the
// candidate's next Complete takes the old order.
func TestEngineerReusePassCommitFailureResumesWithoutReDivert(t *testing.T) {
	ctx := context.Background()
	_, operations, computerID := freezeCandidateTestStore(t)
	const assignment = "assignment-pass-fail"
	op := reviewTestFrozen(t, operations, computerID, "trajectory-pass-fail", assignment, strings.Repeat("6", 64))
	if _, err := operations.Transition(ctx, computerID, op.OperationID, selfdev.StateFrozen, selfdev.StateVerified, nil); err != nil {
		t.Fatal(err)
	}
	rt := &Runtime{selfdevOperations: operations}
	rec := &types.RunRecord{RunID: "run-pass-fail"}
	rt.reviews.Store(rec.RunID, &candidateReview{computerID: computerID, assignmentID: assignment, operationID: op.OperationID, revision: 1})
	got, ok, err := rt.engineeringReviewPark(ctx, rec, nil)
	if err != nil || !ok || got.Terminal || !got.Continue || !strings.Contains(got.Instruction, "committing your Complete failed") {
		t.Fatalf("park on a pass whose commit fails = %+v ok %v err %v", got, ok, err)
	}
	if ended, _ := rt.reviewsEnded.Load(rec.RunID); ended != op.OperationID {
		t.Fatalf("reviewsEnded = %v; the next Complete would be diverted again", ended)
	}
}
