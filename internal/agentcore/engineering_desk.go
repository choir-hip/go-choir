package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// engineeringDeskAgentID derives the desk agent bound to one engineering
// document. The desk agent never runs; it is the durable parent authority and
// mailbox the document-channel occurrences target.
func engineeringDeskAgentID(docID string) string {
	return agentprofile.Engineering + ":" + strings.TrimSpace(docID)
}

// engineeringRevisionObjective derives the assignment objective from the
// admitting revision: the owner_prompt metadata carries the directive; the
// revision content is the fallback when the directive was committed without a
// prompt (e.g. the document's initial revision).
func engineeringRevisionObjective(revision types.Revision) string {
	var metadata map[string]any
	if len(revision.Metadata) > 0 {
		_ = json.Unmarshal(revision.Metadata, &metadata)
	}
	if prompt, ok := metadata["owner_prompt"].(string); ok && strings.TrimSpace(prompt) != "" {
		return strings.TrimSpace(prompt)
	}
	return strings.TrimSpace(revision.Content)
}

// restartCancelledAssignmentReason is the terminal disposition reason the
// restart reconciliation writes when it revokes an absent capsule and cancels
// the bound assignment (engineering_assignment_fate.go). A deliberate cancel
// (owner/management) never carries it, so the restart strand is recoverable
// by re-cast while an intentional cancel stays dead.
const restartCancelledAssignmentReason = "restart revoked absent assignment capsule"

// engineeringMaxRecastAttempts bounds restart recasts: a deterministic failure
// (broken subject, unresolvable objective) must not loop attempts forever.
// Attempt 1 is the original open; recasts mint attempts 2..N. Past the cap the
// bound self-development operation fails instead of recasting again.
const engineeringMaxRecastAttempts = 3

// latestCancelledForRestartRecast scans every recorded attempt of the cast
// identity and reports whether the newest terminal attempt was cancelled by a
// guest restart — the only class that may re-execute under a bumped attempt.
// Returns the latest attempt row and true when a recast is admissible.
func latestCancelledForRestartRecast(assignments []types.EngineeringAssignment, assignmentID string) (types.EngineeringAssignment, bool) {
	latest, found := latestAssignmentAttempt(assignments, assignmentID)
	if !found || !latest.Disposition.Terminal() {
		return types.EngineeringAssignment{}, false
	}
	return latest, latest.Disposition == types.EngineeringAssignmentCancelled &&
		strings.TrimSpace(latest.DispositionReason) == restartCancelledAssignmentReason
}

// latestAssignmentAttempt returns the highest-attempt recorded row for one
// assignment identity; found=false when the identity has never been opened.
func latestAssignmentAttempt(assignments []types.EngineeringAssignment, assignmentID string) (types.EngineeringAssignment, bool) {
	var latest types.EngineeringAssignment
	found := false
	for _, a := range assignments {
		if a.AssignmentID == assignmentID && (!found || a.Binding.Attempt > latest.Binding.Attempt) {
			latest = a
			found = true
		}
	}
	return latest, found
}

// restartRecastReportRef resolves the prior attempt's cancel report ID — the
// receipt the supersede tuple must name. ReportRefs carry canonical object
// IDs, so the report is fetched by canonical ID, not by ReportID suffix.
func (rt *Runtime) restartRecastReportRef(ctx context.Context, prior types.EngineeringAssignment) string {
	for _, ref := range prior.ReportRefs {
		report, reportErr := rt.store.GetEngineeringAssignmentReportByCanonicalID(ctx, strings.TrimSpace(ref))
		if reportErr != nil {
			continue
		}
		if report.Late && report.Result == types.EngineeringResultFailed {
			return report.ReportID
		}
	}
	return ""
}

// ReconcileEngineeringRevisionCast opens the assignment one specific
// owner-authored revision admits. The occurrence consumer calls this with the
// revision the occurrence names; the boot scan and commit dispatch call
// ReconcileEngineeringDesk, which reconciles the head.
func (rt *Runtime) ReconcileEngineeringRevisionCast(ctx context.Context, ownerID, docID, revisionID string) (*types.EngineeringAssignment, error) {
	ownerID, docID, revisionID = strings.TrimSpace(ownerID), strings.TrimSpace(docID), strings.TrimSpace(revisionID)
	computerID := strings.TrimSpace(rt.TextureComputerID())
	if ownerID == "" || docID == "" || revisionID == "" || computerID == "" {
		return nil, fmt.Errorf("engineering desk reconcile: owner, document, revision, and computer identity are required")
	}
	doc, err := rt.store.GetLifecycleDocument(ctx, ownerID, computerID, docID)
	if err != nil {
		return nil, fmt.Errorf("engineering desk reconcile: %w", err)
	}
	revision, err := rt.store.GetLifecycleRevision(ctx, ownerID, computerID, revisionID)
	if err != nil {
		return nil, fmt.Errorf("engineering desk reconcile: %w", err)
	}
	snapshot, err := rt.store.GetLifecycleSnapshot(ctx, ownerID, computerID, doc.TrajectoryID)
	if err != nil {
		return nil, fmt.Errorf("engineering desk reconcile: %w", err)
	}
	return rt.reconcileEngineeringCast(ctx, doc, snapshot, revision)
}

// ReconcileEngineeringDesk drives the document-channel cast for one
// engineering-bound lifecycle document. It is idempotent: the deterministic
// assignment identity replays a committed open, and an open-but-unbound
// assignment resumes its spawn/bind saga. Called from the revision-commit
// dispatch, the create path, and the boot scan.
//
// Two duties, in order:
//  1. Pending cast: the document head is an owner-authored revision with no
//     assignment yet — open the implementation assignment it admits.
//  2. Verification chaining: a completed implementation assignment minted a
//     candidate and the bound self-development operation is frozen — open the
//     verification assignment host-side.
func (rt *Runtime) ReconcileEngineeringDesk(ctx context.Context, ownerID, docID string) (*types.EngineeringAssignment, error) {
	if rt == nil || rt.store == nil {
		return nil, fmt.Errorf("engineering desk reconcile: store authority unavailable")
	}
	ownerID, docID = strings.TrimSpace(ownerID), strings.TrimSpace(docID)
	computerID := strings.TrimSpace(rt.TextureComputerID())
	if ownerID == "" || docID == "" || computerID == "" {
		return nil, fmt.Errorf("engineering desk reconcile: owner, document, and computer identity are required")
	}
	doc, err := rt.store.GetLifecycleDocument(ctx, ownerID, computerID, docID)
	if err != nil {
		return nil, fmt.Errorf("engineering desk reconcile: %w", err)
	}
	if strings.TrimSpace(doc.TrajectoryID) == "" {
		return nil, fmt.Errorf("engineering desk reconcile: document is not lifecycle-bound")
	}
	snapshot, err := rt.store.GetLifecycleSnapshot(ctx, ownerID, computerID, doc.TrajectoryID)
	if err != nil {
		return nil, fmt.Errorf("engineering desk reconcile: %w", err)
	}
	if snapshot.Trajectory.Status != types.TrajectoryLive {
		return nil, nil
	}
	head := snapshot.HeadRevision
	if head.RevisionID == "" || head.RevisionID != snapshot.Document.CurrentRevisionID {
		return nil, nil
	}
	if head.AuthorKind != types.AuthorUser {
		// An agent-authored head (the desk's own landed revision) ends cast
		// admission — only an owner revision opens new work — but it does NOT
		// discharge the verification obligation: a completed implementation
		// whose bound operation is frozen still needs its verification
		// assignment. Run the verification-only pass over every attempt so a
		// desk-authored head cannot wedge the operation at frozen.
		return rt.reconcileVerificationOnly(ctx, doc, snapshot)
	}
	return rt.reconcileEngineeringCast(ctx, doc, snapshot, head)
}

// ReconcileEngineeringDeskForTrajectory is the trajectory-keyed variant used
// by the assignment-completion path: the completing assignment names its
// trajectory, and the snapshot resolves the bound document.
func (rt *Runtime) ReconcileEngineeringDeskForTrajectory(ctx context.Context, ownerID, trajectoryID string) (*types.EngineeringAssignment, error) {
	if rt == nil || rt.store == nil {
		return nil, fmt.Errorf("engineering desk reconcile: store authority unavailable")
	}
	ownerID, trajectoryID = strings.TrimSpace(ownerID), strings.TrimSpace(trajectoryID)
	computerID := strings.TrimSpace(rt.TextureComputerID())
	if ownerID == "" || trajectoryID == "" || computerID == "" {
		return nil, fmt.Errorf("engineering desk reconcile: owner, trajectory, and computer identity are required")
	}
	snapshot, err := rt.store.GetLifecycleSnapshot(ctx, ownerID, computerID, trajectoryID)
	if err != nil {
		return nil, fmt.Errorf("engineering desk reconcile: %w", err)
	}
	docID := strings.TrimSpace(snapshot.Document.DocID)
	if docID == "" {
		return nil, nil // not a document-bound trajectory
	}
	return rt.ReconcileEngineeringDesk(ctx, ownerID, docID)
}

// reconcileEngineeringCast opens the implementation assignment one revision
// admits, then chains verification when the cast completed and a bound
// self-development operation is frozen.
func (rt *Runtime) reconcileEngineeringCast(ctx context.Context, doc types.Document, snapshot types.LifecycleSnapshot, revision types.Revision) (*types.EngineeringAssignment, error) {
	ownerID, computerID, trajectoryID, docID := doc.OwnerID, doc.ComputerID, doc.TrajectoryID, doc.DocID
	deskAgentID := engineeringDeskAgentID(docID)
	deskBound := false
	for _, agent := range snapshot.Agents {
		if agent.AgentID == deskAgentID && agent.Profile == agentprofile.Engineering && agent.LifecycleVersion > 0 {
			deskBound = true
			break
		}
	}
	if !deskBound {
		return nil, fmt.Errorf("engineering desk reconcile: document is not bound to the engineering desk")
	}
	if revision.RevisionID == "" || revision.DocID != docID || revision.TrajectoryID != trajectoryID || revision.AuthorKind != types.AuthorUser {
		return nil, fmt.Errorf("engineering desk reconcile: revision is not an owner-authored revision on the bound document")
	}
	assignmentID := deterministicDocumentAssignmentIdentity(ownerID, computerID, trajectoryID, revision.RevisionID, types.EngineeringAssignmentImplementation, "")
	attempts, listErr := rt.store.ListEngineeringAssignments(ctx, ownerID, computerID, trajectoryID)
	if listErr != nil {
		return nil, fmt.Errorf("engineering desk reconcile: %w", listErr)
	}
	latest, existingFound := latestAssignmentAttempt(attempts, assignmentID)
	// Restart-recast: the newest attempt was cancelled because a guest restart
	// revoked its capsule — system fate, not an owner/management decision. The
	// document's cast intent is still live (the revision is still the head, the
	// desk work item is still open), so the reconcile re-opens the cast at the
	// next attempt with a retry_after_block supersede tuple. Deliberate cancels
	// carry a different reason and remain terminal.
	if latestCancelled, restartCancelled := latestCancelledForRestartRecast(attempts, assignmentID); restartCancelled &&
		revision.RevisionID == snapshot.Document.CurrentRevisionID {
		latest = latestCancelled
		reportRef := rt.restartRecastReportRef(ctx, latest)
		if reportRef == "" {
			return nil, fmt.Errorf("engineering desk reconcile: restart-cancelled attempt %d has no cancel report receipt", latest.Binding.Attempt)
		}
		objective := engineeringRevisionObjective(revision)
		if objective == "" {
			return nil, fmt.Errorf("engineering desk reconcile: admitting revision carries no objective")
		}
	nextAttempt := latest.Binding.Attempt + 1
	if nextAttempt > engineeringMaxRecastAttempts {
		// Terminal, not transient: the bound operation is durably marked
		// failed below; returning an error would re-deliver this reconcile
		// occurrence forever (actor retry -> re-cast attempt -> same
		// exhausted branch), a live-lock that starves every other desk
		// reconcile on the guest. Return clean so the occurrence is
		// incorporated and the loop stops.
		rt.failBoundSelfdevOperation(ctx, doc.ComputerID, doc.TrajectoryID,
			fmt.Sprintf("restart recast attempts exhausted at %d", latest.Binding.Attempt))
		return nil, nil
	}
		deltaDigest := objectgraph.SHA256([]byte(strings.Join([]string{
			"retry_after_block", assignmentID,
			fmt.Sprint(latest.Binding.Attempt), fmt.Sprint(nextAttempt), restartCancelledAssignmentReason,
		}, "\x00")))
		started, openErr := rt.startAssignedEngineeringForDocument(ctx, doc, revision, OpenDocumentAssignmentRequest{
			Objective: objective, Kind: types.EngineeringAssignmentImplementation, RevisionID: revision.RevisionID,
			Attempt: nextAttempt,
			Supersedes: &types.EngineeringSupersedeTuple{
				SupersedesAssignmentID: assignmentID, SupersedesAttempt: latest.Binding.Attempt,
				PriorReceiptRef: reportRef, SupersedeKind: types.EngineeringSupersedeRetryAfterBlock,
				ReasonEnum: "restart_passivation", DeltaDigest: deltaDigest,
			},
		})
		if openErr != nil {
			// A durable-invalid supersede (prior-attempt object gone, malformed
			// tuple, receipt mismatch) can never succeed on retry — the missing
			// durable object is not transient. Re-delivering the reconcile
			// occurrence re-runs this same branch forever and starves every
			// other desk reconcile (same live-lock class as the exhausted
			// branch above). Incorporate: fail the bound op and return clean.
			if errors.Is(openErr, store.ErrEngineeringAssignmentInvalid) {
				rt.failBoundSelfdevOperation(ctx, doc.ComputerID, doc.TrajectoryID,
					fmt.Sprintf("restart recast durably invalid: %v", openErr))
				return nil, nil
			}
			return nil, fmt.Errorf("engineering desk reconcile: restart recast: %w", openErr)
		}
		return &started.Assignment, nil
	}
	// A terminal non-completed implementation that was NOT restart-cancelled
	// (owner cancel, deadline expiry, or a bound-run death whose trajectory
	// reconcile already cancelled it) has no recast path — the bound
	// self-development operation would otherwise sit in executing forever.
	// The desk reconcile is the op-coupling authority: it closes the op here.
	if existingFound && latest.Disposition.Terminal() && latest.Disposition != types.EngineeringAssignmentCompleted {
		rt.failBoundSelfdevOperation(ctx, doc.ComputerID, doc.TrajectoryID,
			"implementation assignment terminated "+string(latest.Disposition)+": "+strings.TrimSpace(latest.DispositionReason))
		return &latest, nil
	}
	if !existingFound || (latest.Disposition != types.EngineeringAssignmentBound && !latest.Disposition.Terminal()) {
		objective := engineeringRevisionObjective(revision)
		if objective == "" {
			return nil, fmt.Errorf("engineering desk reconcile: admitting revision carries no objective")
		}
		started, openErr := rt.startAssignedEngineeringForDocument(ctx, doc, revision, OpenDocumentAssignmentRequest{
			Objective: objective, Kind: types.EngineeringAssignmentImplementation, RevisionID: revision.RevisionID,
			Attempt: latest.Binding.Attempt,
		})
		if openErr != nil {
			return nil, fmt.Errorf("engineering desk reconcile: open cast: %w", openErr)
		}
		return &started.Assignment, nil
	}
	if latest.Disposition == types.EngineeringAssignmentCompleted {
		if verification, verr := rt.reconcileEngineeringVerification(ctx, doc, snapshot, latest, attempts); verr != nil {
			return nil, verr
		} else if verification != nil {
			return verification, nil
		}
	}
	return &latest, nil
}

// reconcileEngineeringVerification opens the verification assignment for a
// completed implementation when the bound self-development operation is
// frozen. Host-side: no model turn mediates it.
func (rt *Runtime) reconcileEngineeringVerification(ctx context.Context, doc types.Document, snapshot types.LifecycleSnapshot, implementation types.EngineeringAssignment, attempts []types.EngineeringAssignment) (*types.EngineeringAssignment, error) {
	if rt.selfdevOperations == nil {
		return nil, nil
	}
	operation, err := rt.selfdevOperations.GetByTrajectory(ctx, doc.ComputerID, doc.TrajectoryID)
	if err != nil {
		return nil, nil // no self-development operation bound to this trajectory
	}
	if operation.BundleDigest == "" ||
		(operation.State != selfdev.StateFrozen && operation.State != selfdev.StateVerified && operation.State != selfdev.StateAwaitingApproval) {
		return nil, nil
	}
	// A verified operation carries the finalized bundle digest + verifier ref
	// on the store row; a crash between frozen->verified and
	// verified->awaiting_approval only needs the terminal transition replayed.
	if operation.State == selfdev.StateVerified {
		if _, transitionErr := rt.selfdevOperations.Transition(ctx, doc.ComputerID, operation.OperationID,
			selfdev.StateVerified, selfdev.StateAwaitingApproval, nil); transitionErr != nil {
			return nil, fmt.Errorf("engineering desk reconcile: resume verified selfdev operation: %w", transitionErr)
		}
		return nil, nil
	}
	candidateID := ""
	for _, ref := range implementation.ReportRefs {
		// ReportRefs carry canonical object IDs (obj:choir.co_super_report:…key-…),
		// not ReportIDs. Resolve by canonical ID — the same loader the
		// restart-recast supersede path uses — so a completed implementation's
		// candidate is found instead of silently skipped.
		report, reportErr := rt.store.GetEngineeringAssignmentReportByCanonicalID(ctx, ref)
		if reportErr != nil {
			continue
		}
		if report.AssignmentID != implementation.AssignmentID {
			continue
		}
		if report.CandidateID != "" {
			candidateID = report.CandidateID
		}
	}
	if candidateID == "" {
		// A completed implementation that carried no candidate identity cannot
		// satisfy a frozen operation's verification obligation — the digest
		// namespace has nothing to admit. Fail rather than return nil forever.
		// Frozen only: an awaiting_approval op already consumed a prior
		// verification and must not be re-failed by a stale report scan.
		if operation.State == selfdev.StateFrozen {
			rt.failBoundSelfdevOperation(ctx, doc.ComputerID, doc.TrajectoryID,
				"implementation completed without a candidate artifact for verification")
		}
		return nil, nil
	}
	verificationID := deterministicDocumentAssignmentIdentity(implementation.Binding.OwnerID, implementation.Binding.ComputerID,
		doc.TrajectoryID, implementation.Binding.ParentControlID, types.EngineeringAssignmentVerification, candidateID)
	vLatest, vFound := latestAssignmentAttempt(attempts, verificationID)
	revision, revErr := rt.store.GetLifecycleRevision(ctx, implementation.Binding.OwnerID, implementation.Binding.ComputerID, implementation.Binding.ParentControlID)
	if revErr != nil {
		return nil, fmt.Errorf("engineering desk reconcile: load admitting revision: %w", revErr)
	}
	// Restart-recast mirrors the implementation branch: a restart-cancelled
	// verification must not strand a frozen operation. Frozen-only: an
	// awaiting-approval operation already consumed a completed verification —
	// a terminal newest attempt is an older row and must not reopen work.
	if operation.State == selfdev.StateFrozen && vFound && vLatest.Disposition == types.EngineeringAssignmentCancelled &&
		strings.TrimSpace(vLatest.DispositionReason) == restartCancelledAssignmentReason {
		reportRef := rt.restartRecastReportRef(ctx, vLatest)
		if reportRef == "" {
			return nil, fmt.Errorf("engineering desk reconcile: restart-cancelled verification attempt %d has no cancel report receipt", vLatest.Binding.Attempt)
		}
		nextAttempt := vLatest.Binding.Attempt + 1
		if nextAttempt > engineeringMaxRecastAttempts {
			// Terminal (see implementation branch): the operation is durably
			// failed; an error return re-delivers this reconcile forever.
			rt.failBoundSelfdevOperation(ctx, doc.ComputerID, doc.TrajectoryID,
				fmt.Sprintf("verification restart recast attempts exhausted at %d", vLatest.Binding.Attempt))
			return nil, nil
		}
		deltaDigest := objectgraph.SHA256([]byte(strings.Join([]string{
			"retry_after_block", verificationID,
			fmt.Sprint(vLatest.Binding.Attempt), fmt.Sprint(nextAttempt), restartCancelledAssignmentReason,
		}, "\x00")))
		objective := "Verify the frozen self-development bundle for operation " + operation.OperationID +
			" against the implementation assignment's candidate artifact."
		started, openErr := rt.startAssignedEngineeringForDocument(ctx, doc, revision, OpenDocumentAssignmentRequest{
			Objective: objective, Kind: types.EngineeringAssignmentVerification, CandidateID: candidateID,
			RevisionID: implementation.Binding.ParentControlID, Attempt: nextAttempt,
			Supersedes: &types.EngineeringSupersedeTuple{
				SupersedesAssignmentID: verificationID, SupersedesAttempt: vLatest.Binding.Attempt,
				PriorReceiptRef: reportRef, SupersedeKind: types.EngineeringSupersedeRetryAfterBlock,
				ReasonEnum: "restart_passivation", DeltaDigest: deltaDigest,
			},
		})
		if openErr != nil {
			return nil, fmt.Errorf("engineering desk reconcile: verification recast: %w", openErr)
		}
		return &started.Assignment, nil
	}
	if vFound && vLatest.Disposition == types.EngineeringAssignmentBound {
		return &vLatest, nil
	}
	// A verification that finished while the operation stayed frozen (e.g. a
	// restart-uncancelled failure, or a completed verification whose
	// record_self_development_verification cell never landed) leaves the op
	// stranded — no further verification can be admitted under the same
	// frozen digest. Fail the operation rather than retry forever. Frozen
	// only: awaiting_approval already consumed a terminal verification.
	if vFound && vLatest.Disposition.Terminal() {
		if operation.State != selfdev.StateFrozen {
			return &vLatest, nil
		}
		reason := "verification assignment terminated while operation stayed frozen"
		if vLatest.Disposition == types.EngineeringAssignmentCompleted {
			reason = "verification completed but operation never advanced past frozen"
		}
		rt.failBoundSelfdevOperation(ctx, doc.ComputerID, doc.TrajectoryID, reason)
		return nil, fmt.Errorf("engineering desk reconcile: %s (verification attempt %d: %s)",
			reason, vLatest.Binding.Attempt, vLatest.Disposition)
	}
	// Only a frozen operation admits a fresh verification: verified already
	// returned early, and awaiting_approval has consumed its verification.
	if operation.State != selfdev.StateFrozen {
		return nil, nil
	}
	objective := "Verify the frozen self-development bundle for operation " + operation.OperationID +
		" against the implementation assignment's candidate artifact."
	started, openErr := rt.startAssignedEngineeringForDocument(ctx, doc, revision, OpenDocumentAssignmentRequest{
		Objective: objective, Kind: types.EngineeringAssignmentVerification, CandidateID: candidateID,
		RevisionID: implementation.Binding.ParentControlID, Attempt: vLatest.Binding.Attempt,
	})
	if openErr != nil {
		if errors.Is(openErr, capsule.ErrSubjectArtifactUnavailable) {
			rt.failBoundSelfdevOperation(ctx, doc.ComputerID, doc.TrajectoryID,
				"candidate artifact lost before verification opened: "+strings.TrimSpace(openErr.Error()))
		}
		return nil, fmt.Errorf("engineering desk reconcile: open verification: %w", openErr)
	}
	return &started.Assignment, nil
}

// reconcileVerificationOnly is the agent-headed counterpart of the cast
// reconcile's second duty: with no owner head to admit new work, it scans the
// trajectory's assignments for the newest completed implementation attempt
// and runs the frozen-operation verification chain on it. Deterministic: the
// highest-attempt completed implementation is the one whose candidate the
// verification must name.
func (rt *Runtime) reconcileVerificationOnly(ctx context.Context, doc types.Document, snapshot types.LifecycleSnapshot) (*types.EngineeringAssignment, error) {
	attempts, listErr := rt.store.ListEngineeringAssignments(ctx, doc.OwnerID, doc.ComputerID, doc.TrajectoryID)
	if listErr != nil {
		return nil, fmt.Errorf("engineering desk reconcile: %w", listErr)
	}
	var implementation *types.EngineeringAssignment
	for i := range attempts {
		a := &attempts[i]
		if a.Binding.Kind != types.EngineeringAssignmentImplementation || a.Disposition != types.EngineeringAssignmentCompleted {
			continue
		}
		if implementation == nil || a.Binding.Attempt > implementation.Binding.Attempt {
			implementation = a
		}
	}
	if implementation == nil {
		return nil, nil
	}
	return rt.reconcileEngineeringVerification(ctx, doc, snapshot, *implementation, attempts)
}
