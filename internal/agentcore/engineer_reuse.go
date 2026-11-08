package agentcore

import (
	"context"
	"errors"
	"fmt"
	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"log"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Engineer reuse, phase A (docs/design/engineer-reuse-2026-11-04.md): the
// judge runs while the engineer is still alive. The Complete that follows a
// Freeze becomes a review request (a progress Report that asks management
// for Verify now), and the engineer's turn parks on the verdict with its
// capsule live and its conversation in memory. A pass commits the
// engineer's own Complete; a fail resumes the turn with management's
// reason, and the engineer's next Freeze opens the assignment's next
// operation revision. Revisions 1 to 5 are reviewed in the loop; the sixth
// candidate takes the old order (Complete, then Verify), so its rejection
// ends the round with the judge's reason.
//
// The park holds nothing durable, like the job park: a crash ends it with
// the process, and the boot closes the run (the restart rule).

// reviewParkMax bounds the wait for management's verdict. When it passes,
// the park commits the engineer's Complete and the old order takes over.
var reviewParkMax = 20 * time.Minute

// reviewFixAllowance is the least build time a rejected candidate's
// engineer has left when it resumes. The verdict park suspends the
// activation's progress deadline (waiting on management is not the
// engineer's stall), and a rejection grants at least this much: deepseek
// Calendar r6 (2026-11-04) froze its second candidate at minute 60 and
// ended at the budget with no fix time.
var reviewFixAllowance = 20 * time.Minute

// reviewParkRecheck is the park's re-read of the operation between wakes.
// A Verify signals the wake; an owner act or a close on the operation does
// not, so the re-read bounds how long the park can miss one.
var reviewParkRecheck = 30 * time.Second

// reviewEvidencePrefix marks a review Report's evidence ref
// (review:<operation>). An engineer cannot stage one: commitCompleteIntent
// drops it from the Complete's own refs.
const reviewEvidencePrefix = "review:"

// reviewSummaryPrefix opens a review Report's summary; management's prompt
// names it.
const reviewSummaryPrefix = "Review requested"

// operationByKeyReader reads an operation by its idempotency key.
type operationByKeyReader interface {
	GetByIdempotency(ctx context.Context, computerID, idempotencyKey string) (selfdev.Operation, bool, error)
}

// assignmentOperation resolves an assignment's current operation: the
// highest revision that exists (revisions are opened in order, so the
// first missing one ends the series). revision is 0 when there is none.
func assignmentOperation(ctx context.Context, operations operationByKeyReader, computerID, assignmentID string) (selfdev.Operation, int, bool, error) {
	var current selfdev.Operation
	revision := 0
	for n := 1; n <= store.EngineeringFreezeMaxRevisions; n++ {
		op, found, err := operations.GetByIdempotency(ctx, computerID, store.EngineeringFreezeRevisionIdempotencyKey(assignmentID, n))
		if err != nil {
			return selfdev.Operation{}, 0, false, err
		}
		if !found {
			break
		}
		current, revision = op, n
	}
	return current, revision, revision > 0, nil
}

// judgmentRejected reports whether management's Verify failed the
// operation (recordSelfDevelopmentVerdict's terminal error).
func judgmentRejected(op selfdev.Operation) bool {
	// A verdict also stamps its verification event on the operation; no
	// cancellation close does, so a close reason that happens to begin with
	// the same words is not a rejection.
	return op.State == selfdev.StateFailed && strings.HasPrefix(op.TerminalError, selfDevelopmentJudgmentRejected) && len(op.VerifierRefs) > 0
}

// reviewPassed reports whether the operation's state is one a passed
// Verify leads to.
func reviewPassed(state string) bool {
	switch state {
	case selfdev.StateVerified, selfdev.StateAwaitingApproval, selfdev.StateAccepted, selfdev.StateMaterializing, selfdev.StateApplied:
		return true
	}
	return false
}

// freezeOpensNextRevision reports whether a Freeze on an assignment whose
// current operation is op (revision n) opens revision n+1: op was rejected
// by management's judgment and the bound leaves room. Any other terminal
// state keeps the old refusal.
func freezeOpensNextRevision(op selfdev.Operation, revision int) bool {
	return revision >= 1 && revision < store.EngineeringFreezeMaxRevisions && judgmentRejected(op)
}

// candidateReview is one candidate under review in the loop: the engineer's
// Complete, kept for the pass, and what the park needs to read the verdict.
type candidateReview struct {
	ownerID, computerID, assignmentID string
	attempt                           uint64
	operationID                       string
	revision                          int
	intentIdentity                    string
	report                            types.EngineeringAssignmentReport
	at                                time.Time
}

// reviewToolCallID is the identity of a review Report: one per operation,
// so a replayed cell replays the same Report and Verify can find it.
func reviewToolCallID(runID, operationID string) string {
	return "rlm-review:" + strings.TrimSpace(runID) + ":" + strings.TrimSpace(operationID)
}

// partialReportID is the report id of a partial (non-final) assignment
// report (recordAssignedEngineeringReportOnce).
func partialReportID(ownerID, computerID, runID, assignmentID string, attempt uint64, toolCallID string) string {
	return "report:" + objectgraph.SHA256([]byte(strings.Join([]string{
		"choir:co-super-report:v1", ownerID, computerID, runID, assignmentID, fmt.Sprint(attempt), strings.TrimSpace(toolCallID),
	}, "\x00")))
}

// reviewOperationFromRefs names the operation a review Report asks about.
func reviewOperationFromRefs(refs []string) string {
	for _, ref := range refs {
		if op, ok := strings.CutPrefix(strings.TrimSpace(ref), reviewEvidencePrefix); ok && strings.TrimSpace(op) != "" {
			return strings.TrimSpace(op)
		}
	}
	return ""
}

// withoutReviewRefs drops review refs from an engineer's own evidence refs:
// only the platform marks a Report as a review request.
func withoutReviewRefs(refs []string) []string {
	out := refs[:0:0]
	for _, ref := range refs {
		if !strings.HasPrefix(strings.TrimSpace(ref), reviewEvidencePrefix) {
			out = append(out, ref)
		}
	}
	return out
}

// reviewDivert decides whether a Complete becomes a review request: its
// result is completed, the assignment's current operation is frozen with a
// bundle, and that revision is below the bound. It returns the operation
// and its revision.
func (r *rlmCallReduction) reviewDivert(ctx context.Context, result types.EngineeringAssignmentResultKind) (selfdev.Operation, int, bool) {
	if result != types.EngineeringResultCompleted || r.rec == nil || r.toolCtx == nil || r.toolCtx.OperationStore == nil {
		return selfdev.Operation{}, 0, false
	}
	assignmentID := metadataStringValue(r.rec.Metadata, "assignment_id")
	if assignmentID == "" {
		return selfdev.Operation{}, 0, false
	}
	op, revision, found, err := assignmentOperation(ctx, r.toolCtx.OperationStore, r.toolCtx.ComputerID, assignmentID)
	if err != nil || !found {
		return selfdev.Operation{}, 0, false
	}
	if op.State != selfdev.StateFrozen || strings.TrimSpace(op.BundleDigest) == "" || revision >= store.EngineeringFreezeMaxRevisions {
		return selfdev.Operation{}, 0, false
	}
	// A candidate whose review timed out, or whose kept Complete failed to
	// commit, takes the old order: the engineer's Complete is not
	// diverted again.
	if done, ok := r.rt().reviewsEnded.Load(r.scope.RunID); ok && done.(string) == op.OperationID {
		return selfdev.Operation{}, 0, false
	}
	// A stranded terminal saga (a staged proposal, a capsule no longer
	// active) resumes through the normal path, not a review (review r1,
	// Devin 2).
	if rt := r.rt(); rt != nil && rt.store != nil {
		attempt := uint64(metadataIntValue(r.rec.Metadata, "assignment_attempt"))
		assignment, err := rt.store.GetEngineeringAssignment(ctx, r.scope.OwnerID, r.scope.ComputerID, assignmentID, attempt)
		if err != nil || assignment.PendingProposal != nil || assignment.CapsuleDisposition != types.EngineeringCapsuleActive || assignment.Disposition.Terminal() {
			return selfdev.Operation{}, 0, false
		}
	}
	return op, revision, true
}

// commitReviewRequest commits the engineer's Complete as a review request:
// a progress Report to management carrying the report's content, the
// candidate's refs and review:<operation>, and keeps the Complete for the
// verdict park. The assignment stays open and the capsule live.
func (r *rlmCallReduction) commitReviewRequest(ctx context.Context, op selfdev.Operation, revision int, intentIdentity string, report types.EngineeringAssignmentReport) error {
	rt := r.rt()
	assignmentID, attempt := metadataStringValue(r.rec.Metadata, "assignment_id"), uint64(metadataIntValue(r.rec.Metadata, "assignment_attempt"))
	review := report
	review.Result = types.EngineeringResultPartial
	review.Verdict = types.EngineeringVerdictNone
	review.Summary = fmt.Sprintf("%s (candidate %d of %d): operation %s, bundle %s. management, Verify it now; the engineer waits with its capsule live.\n\n%s",
		reviewSummaryPrefix, revision, store.EngineeringFreezeMaxRevisions, op.OperationID, op.BundleDigest, report.Summary)
	review.EvidenceRefs = append(append([]string(nil), report.EvidenceRefs...), reviewEvidencePrefix+op.OperationID)
	_, err := func() (types.EngineeringAssignmentCommandResult, error) {
		saga := rt.assignmentSagaMutex(assignmentID, attempt)
		saga.Lock()
		defer saga.Unlock()
		return rt.recordAssignedEngineeringReport(ctx, r.rec, reviewToolCallID(r.scope.RunID, op.OperationID), review)
	}()
	if err != nil {
		return fmt.Errorf("reduce: complete: request management's review of candidate %s: %w", op.OperationID, err)
	}
	rt.reviews.Store(r.scope.RunID, &candidateReview{
		ownerID: r.scope.OwnerID, computerID: r.scope.ComputerID, assignmentID: assignmentID, attempt: attempt,
		operationID: op.OperationID, revision: revision,
		intentIdentity: intentIdentity, report: report, at: time.Now().UTC(),
	})
	log.Printf("selfdev: assignment %s candidate %s (revision %d) is under management's review; run %s parks on the verdict", assignmentID, op.OperationID, revision, r.scope.RunID)
	return nil
}

// assignmentInReview reports whether management may Verify the open
// assignment's current operation now: the assignment is bound and not
// terminal, op is frozen, and the review Report for op exists.
func (rt *Runtime) assignmentInReview(ctx context.Context, assignment types.EngineeringAssignment, op selfdev.Operation) bool {
	// Awaiting approval too: a repeated pass on the open assignment while
	// the park commits the Complete replays instead of being refused; and a
	// rejected candidate, so a replayed pass the per-shot review failed
	// answers from the operation (commitVerifyIntent) instead of erroring.
	if rt == nil || rt.store == nil || assignment.Disposition.Terminal() || strings.TrimSpace(assignment.BoundRunID) == "" ||
		(op.State != selfdev.StateFrozen && op.State != selfdev.StateVerified && op.State != selfdev.StateAwaitingApproval && !judgmentRejected(op)) {
		return false
	}
	id := partialReportID(assignment.Binding.OwnerID, assignment.Binding.ComputerID, assignment.BoundRunID, assignment.AssignmentID,
		assignment.Binding.Attempt, reviewToolCallID(assignment.BoundRunID, op.OperationID))
	_, err := rt.store.GetEngineeringAssignmentReport(ctx, assignment.Binding.OwnerID, assignment.Binding.ComputerID, id)
	return err == nil
}

// reviewWake is the assignment's verdict wake (one pending signal).
func (rt *Runtime) reviewWake(assignmentID string) chan struct{} {
	ch, _ := rt.reviewWakes.LoadOrStore(assignmentID, make(chan struct{}, 1))
	return ch.(chan struct{})
}

func (rt *Runtime) signalReviewWake(assignmentID string) {
	if rt == nil || strings.TrimSpace(assignmentID) == "" {
		return
	}
	select {
	case rt.reviewWake(assignmentID) <- struct{}{}:
	default:
	}
}

// noteReviewVerdict runs after management's Verify on a candidate under
// review: it wakes the parked engineer, and on a fail tells the Texture the
// round serves that the engineer is fixing it.
func (rt *Runtime) noteReviewVerdict(ctx context.Context, assignment types.EngineeringAssignment, op selfdev.Operation, revision int, decision, reason string) {
	rt.signalReviewWake(assignment.AssignmentID)
	if decision != "fail" {
		return
	}
	textureInbox, ok := types.ConversationCounterpart(assignment.Binding.ParentConversationID, assignment.Binding.ParentAgentID)
	if !ok || !deskmanifest.IsDocumentInbox(textureInbox) {
		return
	}
	reason = strings.Join(strings.Fields(reason), " ")
	if reason == "" {
		reason = "no reason given"
	}
	body := fmt.Sprintf("rejected: %s; the engineer is fixing it (candidate %d of %d, operation %s).", reason, revision, store.EngineeringFreezeMaxRevisions, op.OperationID)
	source := types.CommitmentRecord{
		RecordID:    "review-rejected:" + op.OperationID,
		SenderInbox: assignment.Binding.AssignedAgentID, ComputerID: assignment.Binding.ComputerID,
		Prediction:   types.CommitmentPrediction{Hypothesis: body},
		Provenance:   types.CommitmentProvenance{CommittedAt: time.Now().UTC().Format(time.RFC3339Nano)},
		EvidenceRefs: []string{"assignment:" + assignment.AssignmentID, "selfdev-operation:" + op.OperationID},
	}
	relay, ok := types.NewEngineeringProgressRelay(source, textureInbox)
	if !ok {
		return
	}
	if !rt.computerSettingOn(context.WithoutCancel(ctx), assignment.Binding.OwnerID, assignment.Binding.ComputerID, store.SettingEngineeringProgressToTexture) {
		return
	}
	req := types.CommitInboxRecordsRequest{OwnerID: assignment.Binding.OwnerID, ComputerID: assignment.Binding.ComputerID,
		CommandID: "review-relay:" + relay.RecordID, Records: []types.CommitmentRecord{relay}}
	digest, err := store.ComputeCommitInboxRecordsDigest(req)
	if err == nil {
		req.CommandDigest = digest
		cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		_, err = rt.store.CommitInboxRecords(cctx, req)
		cancel()
	}
	if err != nil {
		log.Printf("selfdev: review rejection line for %s: %v", op.OperationID, err)
	}
}

// How a review park ended.
const (
	reviewParkPassed   = "candidate_review_passed"
	reviewParkRejected = "candidate_review_rejected"
	reviewParkEnded    = "candidate_review_ended"
	reviewParkTimedOut = "candidate_review_timed_out"
)

// engineeringReviewPark is the verdict park: after a cell that committed a
// review request, the turn waits with no model call for management's
// verdict on the candidate (A3 of the design).
func (rt *Runtime) engineeringReviewPark(ctx context.Context, rec *types.RunRecord, emit provideriface.EventEmitFunc) (toolregistry.ToolLoopCompletionGuardResult, bool, error) {
	none := toolregistry.ToolLoopCompletionGuardResult{}
	if rt == nil || rec == nil || rt.selfdevOperations == nil {
		return none, false, nil
	}
	v, ok := rt.reviews.Load(rec.RunID)
	if !ok {
		return none, false, nil
	}
	review := v.(*candidateReview)
	// Registered before the first read, so a verdict between the cell and
	// the park is not missed: the read sees it, or the wake does.
	wake := rt.reviewWake(review.assignmentID)
	emitJobPark(emit, "candidate_review_park", map[string]any{"operation_id": review.operationID, "revision": review.revision,
		"deadline": time.Now().Add(reviewParkMax).UTC().Format(time.RFC3339)})
	started := time.Now()
	deadline := activationDeadlineFrom(ctx)
	deadline.suspend(&types.WaitState{Kind: types.WaitKindVerifyOwed, Class: types.WaitClassPolicy,
		Subject: "assignment " + review.assignmentID, Resource: "management's verdict on candidate " + review.operationID,
		Since: started, Until: started.Add(reviewParkMax)})
	defer deadline.resume()
	timer := time.NewTimer(reviewParkMax)
	defer timer.Stop()
	recheck := time.NewTicker(reviewParkRecheck)
	defer recheck.Stop()
	end := func(reason string) {
		rt.reviews.Delete(rec.RunID)
		rt.reviewWakes.Delete(review.assignmentID)
		emitJobPark(emit, "candidate_review_park_ended", map[string]any{"operation_id": review.operationID, "revision": review.revision,
			"reason": reason, "waited_ms": time.Since(started).Milliseconds()})
	}
	for {
		op, err := rt.selfdevOperations.Get(ctx, review.computerID, review.operationID)
		if err != nil && ctx.Err() != nil {
			end("cancelled")
			return none, false, ctx.Err()
		}
		if err == nil {
			switch {
			case op.State == selfdev.StateFrozen || op.State == selfdev.StateRequested || op.State == selfdev.StateExecuting:
				// Still under review.
			case judgmentRejected(op):
				end(reviewParkRejected)
				deadline.resume()
				deadline.grantAtLeast(reviewFixAllowance)
				return toolregistry.ToolLoopCompletionGuardResult{Continue: true, Waited: true, Reason: reviewParkRejected,
					Instruction: withActivationBudget(ctx, reviewRejectedInstruction(review, op))}, true, nil
			case reviewPassed(op.State):
				end(reviewParkPassed)
				return rt.commitKeptComplete(ctx, rec, review, reviewParkPassed)
			default:
				// Failed for another cause, rejected by the owner, degraded,
				// rolled back: the candidate ended without this round's pass.
				end(reviewParkEnded)
				deadline.resume()
				deadline.grantAtLeast(reviewFixAllowance)
				return toolregistry.ToolLoopCompletionGuardResult{Continue: true, Waited: true, Reason: reviewParkEnded,
					Instruction: withActivationBudget(ctx, reviewEndedInstruction(op))}, true, nil
			}
		}
		select {
		case <-ctx.Done():
			end("cancelled")
			return none, false, ctx.Err()
		case <-timer.C:
			// One last read: a verdict that lands as the timer fires is the
			// judge's, never overridden by the timeout (review r1, DeepSeek N2).
			if op, err := rt.selfdevOperations.Get(ctx, review.computerID, review.operationID); err == nil &&
				op.State != selfdev.StateFrozen && op.State != selfdev.StateRequested && op.State != selfdev.StateExecuting {
				continue
			}
			end(reviewParkTimedOut)
			return rt.commitKeptComplete(ctx, rec, review, reviewParkTimedOut)
		case <-wake:
		case <-recheck.C:
		}
	}
}

// commitKeptComplete commits the engineer's own Complete, kept at the
// review request, through the terminal saga, and ends the loop. When the
// commit fails the turn resumes with the error, so the engineer can send
// the Complete again.
func (rt *Runtime) commitKeptComplete(ctx context.Context, rec *types.RunRecord, review *candidateReview, reason string) (toolregistry.ToolLoopCompletionGuardResult, bool, error) {
	// Committed as a cell's Complete is (commitTray): detached from the
	// activation's end, with a deadline of its own, so a budget that ends
	// now does not cut the terminal saga in half.
	if ctx.Err() != nil {
		return toolregistry.ToolLoopCompletionGuardResult{}, false, ctx.Err()
	}
	fateCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Minute)
	defer cancel()
	committed, err := func() (types.EngineeringAssignmentCommandResult, error) {
		saga := rt.assignmentSagaMutex(review.assignmentID, review.attempt)
		saga.Lock()
		defer saga.Unlock()
		return rt.recordAssignedEngineeringReport(fateCtx, rec, review.intentIdentity, review.report)
	}()
	if err != nil {
		rt.reviewsEnded.Store(rec.RunID, review.operationID)
		if ctx.Err() != nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
			return toolregistry.ToolLoopCompletionGuardResult{}, false, err
		}
		log.Printf("selfdev: commit the kept Complete of assignment %s after %s: %v", review.assignmentID, reason, err)
		activationDeadlineFrom(ctx).resume()
		activationDeadlineFrom(ctx).grantAtLeast(reviewFixAllowance)
		return toolregistry.ToolLoopCompletionGuardResult{Continue: true, Waited: true, Reason: reason,
			Instruction: withActivationBudget(ctx, fmt.Sprintf("Management's review of candidate %s ended (%s), and committing your Complete failed: %v. Send the same choir.Complete(result=\"completed\", ...) again in a cell of its own; if it fails again, end with choir.Complete(result=\"failed\", ...) naming this error.", review.operationID, reason, err))}, true, nil
	}
	if committed.Assignment.Disposition.Terminal() {
		rt.releaseAssignmentSaga(review.assignmentID, review.attempt)
	}
	log.Printf("selfdev: assignment %s: committed the kept Complete after %s (candidate %s)", review.assignmentID, reason, review.operationID)
	return toolregistry.ToolLoopCompletionGuardResult{Terminal: true, Reason: reason}, true, nil
}

// reviewRejectedInstruction resumes the engineer after management rejected
// its candidate.
func reviewRejectedInstruction(review *candidateReview, op selfdev.Operation) string {
	reason := strings.TrimSpace(strings.TrimPrefix(op.TerminalError, selfDevelopmentJudgmentRejected))
	reason = strings.TrimSpace(strings.TrimPrefix(reason, ":"))
	if reason == "" {
		reason = "management gave no reason"
	}
	next := review.revision + 1
	b := &strings.Builder{}
	fmt.Fprintf(b, "Management rejected your candidate %s (candidate %d of %d): %s\n", op.OperationID, review.revision, store.EngineeringFreezeMaxRevisions, reason)
	b.WriteString("Your capsule and your work are as you left them, and this assignment is still yours: no new round is cast. ")
	b.WriteString("Fix what the reason names, re-run the build and the checks Freeze cites (and the screenshots, if the reason is visual) after your last write. ")
	b.WriteString("If the reason is visual, re-shoot every screen it names, not only the one you changed, and look at each new shot yourself to confirm the named defect is gone and nothing new is clipped or covered before you Freeze: management judges the whole set again, and a candidate is a scarce turn, not a free retry. ")
	b.WriteString("Then ")
	fmt.Fprintf(b, "choir.Freeze again, which opens candidate %d, and choir.Complete(result=\"completed\", ...) as before.", next)
	if next >= store.EngineeringFreezeMaxRevisions {
		b.WriteString(" It is the last candidate: management judges it after the round ends, and a rejection ends the round with its reason.")
	}
	b.WriteString(" If the reason cannot be met, end with choir.Complete(result=\"failed\", ...) and say why.")
	return b.String()
}

// reviewEndedInstruction resumes the engineer when its candidate ended for a
// cause other than management's verdict.
func reviewEndedInstruction(op selfdev.Operation) string {
	cause := strings.TrimSpace(op.TerminalError)
	if cause == "" {
		cause = "no reason recorded"
	}
	what := fmt.Sprintf("Your candidate %s ended %s before management's verdict (%s).", op.OperationID, op.State, cause)
	if op.State == selfdev.StateRejected {
		what = fmt.Sprintf("Your candidate %s was rejected by the owner (%s).", op.OperationID, cause)
	}
	return what + " It cannot be judged or fixed in this round: end it with choir.Complete(result=\"failed\", ...) naming this cause."
}
