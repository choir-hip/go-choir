package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
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

// latestCancelledForRestartRecast scans every recorded attempt of the cast
// identity and reports whether the newest terminal attempt was cancelled by a
// guest restart — the only class that may re-execute under a bumped attempt.
// Returns the latest attempt row and true when a recast is admissible.
func latestCancelledForRestartRecast(assignments []types.EngineeringAssignment, assignmentID string) (types.EngineeringAssignment, bool) {
	var latest types.EngineeringAssignment
	found := false
	for _, a := range assignments {
		if a.AssignmentID != assignmentID {
			continue
		}
		if !found || a.Binding.Attempt > latest.Binding.Attempt {
			latest = a
			found = true
		}
	}
	if !found || !latest.Disposition.Terminal() {
		return types.EngineeringAssignment{}, false
	}
	return latest, latest.Disposition == types.EngineeringAssignmentCancelled &&
		strings.TrimSpace(latest.DispositionReason) == restartCancelledAssignmentReason
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
	if head.RevisionID == "" || head.RevisionID != snapshot.Document.CurrentRevisionID || head.AuthorKind != types.AuthorUser {
		return nil, nil
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
	var latest types.EngineeringAssignment
	existingFound := false
	for _, a := range attempts {
		if a.AssignmentID == assignmentID && (!existingFound || a.Binding.Attempt > latest.Binding.Attempt) {
			latest = a
			existingFound = true
		}
	}
	// Restart-recast: the newest attempt was cancelled because a guest restart
	// revoked its capsule — system fate, not an owner/management decision. The
	// document's cast intent is still live (the revision is still the head, the
	// desk work item is still open), so the reconcile re-opens the cast at the
	// next attempt with a retry_after_block supersede tuple. Deliberate cancels
	// carry a different reason and remain terminal.
	if latest, restartCancelled := latestCancelledForRestartRecast(attempts, assignmentID); restartCancelled &&
		revision.RevisionID == snapshot.Document.CurrentRevisionID {
		reportRef := rt.restartRecastReportRef(ctx, latest)
		if reportRef == "" {
			return nil, fmt.Errorf("engineering desk reconcile: restart-cancelled attempt %d has no cancel report receipt", latest.Binding.Attempt)
		}
		objective := engineeringRevisionObjective(revision)
		if objective == "" {
			return nil, fmt.Errorf("engineering desk reconcile: admitting revision carries no objective")
		}
		nextAttempt := latest.Binding.Attempt + 1
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
			return nil, fmt.Errorf("engineering desk reconcile: restart recast: %w", openErr)
		}
		return &started.Assignment, nil
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
		if verification, verr := rt.reconcileEngineeringVerification(ctx, doc, snapshot, latest); verr != nil {
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
func (rt *Runtime) reconcileEngineeringVerification(ctx context.Context, doc types.Document, snapshot types.LifecycleSnapshot, implementation types.EngineeringAssignment) (*types.EngineeringAssignment, error) {
	if rt.selfdevOperations == nil {
		return nil, nil
	}
	operation, err := rt.selfdevOperations.GetByTrajectory(ctx, doc.ComputerID, doc.TrajectoryID)
	if err != nil {
		return nil, nil // no self-development operation bound to this trajectory
	}
	if operation.BundleDigest == "" || (operation.State != selfdev.StateFrozen && operation.State != selfdev.StateAwaitingApproval) {
		return nil, nil
	}
	candidateID := ""
	for _, ref := range implementation.ReportRefs {
		_, _, suffix, parseErr := objectgraph.ParseCanonicalID(strings.TrimSpace(ref))
		if parseErr != nil {
			continue
		}
		report, reportErr := rt.store.GetEngineeringAssignmentReport(ctx, implementation.Binding.OwnerID, implementation.Binding.ComputerID, suffix)
		if reportErr != nil {
			continue
		}
		if report.CandidateID != "" {
			candidateID = report.CandidateID
		}
	}
	if candidateID == "" {
		return nil, nil
	}
	verificationID := deterministicDocumentAssignmentIdentity(implementation.Binding.OwnerID, implementation.Binding.ComputerID,
		doc.TrajectoryID, implementation.Binding.ParentControlID, types.EngineeringAssignmentVerification, candidateID)
	if existing, getErr := rt.store.GetEngineeringAssignment(ctx, implementation.Binding.OwnerID, implementation.Binding.ComputerID, verificationID, 1); getErr == nil {
		if existing.Disposition == types.EngineeringAssignmentBound || existing.Disposition.Terminal() {
			return &existing, nil
		}
	} else if !errors.Is(getErr, store.ErrNotFound) {
		return nil, fmt.Errorf("engineering desk reconcile: %w", getErr)
	}
	revision, revErr := rt.store.GetLifecycleRevision(ctx, implementation.Binding.OwnerID, implementation.Binding.ComputerID, implementation.Binding.ParentControlID)
	if revErr != nil {
		return nil, fmt.Errorf("engineering desk reconcile: load admitting revision: %w", revErr)
	}
	objective := "Verify the frozen self-development bundle for operation " + operation.OperationID +
		" against the implementation assignment's candidate artifact."
	started, openErr := rt.startAssignedEngineeringForDocument(ctx, doc, revision, OpenDocumentAssignmentRequest{
		Objective: objective, Kind: types.EngineeringAssignmentVerification, CandidateID: candidateID,
		RevisionID: implementation.Binding.ParentControlID,
	})
	if openErr != nil {
		return nil, fmt.Errorf("engineering desk reconcile: open verification: %w", openErr)
	}
	return &started.Assignment, nil
}
