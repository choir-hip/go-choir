package agentcore

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func engineeringFateRequest(assignment types.EngineeringAssignment, disposition types.EngineeringCapsuleDisposition, intentRef, ackRef string) types.SetEngineeringCapsuleDispositionRequest {
	req := types.SetEngineeringCapsuleDispositionRequest{
		CommandID: fmt.Sprintf("co-super-capsule:%s:%d:%s", assignment.AssignmentID, assignment.Binding.Attempt, disposition),
		OwnerID:   assignment.Binding.OwnerID, ComputerID: assignment.Binding.ComputerID,
		AssignmentID: assignment.AssignmentID, Attempt: assignment.Binding.Attempt,
		ExpectedLifecycleVersion: assignment.LifecycleVersion, Disposition: disposition,
		IntentRef: intentRef, AckRef: ackRef,
	}
	if assignment.GrantPolicyAttestation != nil || len(assignment.CapsuleFateHistory) > 0 {
		req.FateStep = &types.EngineeringCapsuleFateStep{}
	}
	req.CommandDigest, _ = store.ComputeSetEngineeringCapsuleDispositionDigest(req)
	return req
}

func engineeringFateAckRequest(assignment types.EngineeringAssignment, disposition types.EngineeringCapsuleDisposition, intentRef, ackRef, sourceDigest, finalDigest, occurredAt string, capsuleAbsent bool) (types.SetEngineeringCapsuleDispositionRequest, error) {
	req := engineeringFateRequest(assignment, disposition, intentRef, ackRef)
	if req.FateStep == nil {
		return req, nil
	}
	req.FateStep.SourceSubjectDigest = strings.TrimSpace(sourceDigest)
	req.FateStep.FinalSubjectDigest = strings.TrimSpace(finalDigest)
	req.FateStep.CapsuleAbsent = capsuleAbsent
	occurredAt = strings.TrimSpace(occurredAt)
	if occurredAt != "" {
		parsed, err := time.Parse(time.RFC3339Nano, occurredAt)
		if err != nil {
			return types.SetEngineeringCapsuleDispositionRequest{}, fmt.Errorf("assignment command receipt occurred_at is invalid")
		}
		req.FateStep.OccurredAt = parsed.UTC()
	}
	req.CommandDigest, _ = store.ComputeSetEngineeringCapsuleDispositionDigest(req)
	return req, nil
}

func (rt *Runtime) assignedCapsule() assignmentCapsuleRuntime {
	if rt == nil {
		return nil
	}
	if rt.assignmentRuntime != nil {
		return rt.assignmentRuntime
	}
	if rt.capsuleExecutor == nil {
		return nil
	}
	return rt.capsuleExecutor
}

func assignedEngineeringRun(rec *types.RunRecord) bool {
	if rec == nil {
		return false
	}
	profile := agentProfileForRun(rec)
	return profile == agentprofile.Engineering &&
		metadataStringValue(rec.Metadata, "assignment_id") != ""
}

func (rt *Runtime) assignedEngineeringCapsuleUsable(assignment types.EngineeringAssignment) bool {
	exec := rt.assignedCapsule()
	if exec == nil || strings.TrimSpace(assignment.BoundRunID) == "" || strings.TrimSpace(assignment.Binding.CapsuleID) == "" {
		return false
	}
	handle, handleErr := exec.AssignmentHandle(assignment.BoundRunID, assignment.Binding.CapsuleID)
	diagnostics, inspectErr := exec.InspectCapsuleRaw(assignment.Binding.CapsuleID)
	return handleErr == nil && strings.TrimSpace(handle) != "" && inspectErr == nil &&
		diagnostics != nil && diagnostics.ID == assignment.Binding.CapsuleID && diagnostics.State == capsule.StateActive
}

func (rt *Runtime) revokeAssignedCapsule(ctx context.Context, assignment types.EngineeringAssignment, reason string) (types.EngineeringAssignment, error) {
	exec := rt.assignedCapsule()
	if exec == nil {
		return assignment, fmt.Errorf("assigned capsule executor unavailable")
	}
	intentRef := assignment.CapsuleIntentRef
	if assignment.CapsuleDisposition != types.EngineeringCapsuleRevokeRequested && assignment.CapsuleDisposition != types.EngineeringCapsuleRevoked {
		intentRef = "capsule-revoke-intent:" + objectgraph.SHA256([]byte(strings.Join([]string{
			assignment.AssignmentID, fmt.Sprint(assignment.Binding.Attempt), assignment.BoundRunID, assignment.Binding.CapsuleID, reason,
		}, "\x00")))
		requested, err := rt.store.SetEngineeringCapsuleDisposition(ctx, engineeringFateRequest(assignment, types.EngineeringCapsuleRevokeRequested, intentRef, ""))
		if err != nil {
			return assignment, err
		}
		assignment = requested.Assignment
	}
	if assignment.CapsuleDisposition == types.EngineeringCapsuleRevoked {
		return assignment, nil
	}
	ackRunID := assignment.BoundRunID
	if ackRunID == "" {
		ackRunID = "unbound:" + assignment.AssignmentID
	} else if handle, err := exec.AssignmentHandle(assignment.BoundRunID, assignment.Binding.CapsuleID); err == nil {
		if err := exec.RevokeCapability(assignment.BoundRunID, handle); err != nil {
			return assignment, fmt.Errorf("revoke assignment capability: %w", err)
		}
	}
	if exec.HasCapsule(assignment.Binding.CapsuleID) {
		if err := exec.ForceDestroy(ctx, assignment.Binding.CapsuleID); err != nil {
			return assignment, fmt.Errorf("destroy assignment capsule after durable revoke intent: %w", err)
		}
	}
	if exec.HasCapsule(assignment.Binding.CapsuleID) {
		return assignment, fmt.Errorf("assignment capsule continued after executor acknowledgement")
	}
	if err := exec.CleanupOrphanedCapsule(ctx, assignment.Binding.CapsuleID); err != nil {
		return assignment, fmt.Errorf("clean exact orphaned capsule residue before acknowledgement: %w", err)
	}
	revocationReceipt, receiptErr := exec.PersistRevocationReceipt(ackRunID, assignment.Binding.CapabilityDigest, assignment.Binding.CapsuleID, intentRef)
	if receiptErr != nil || !revocationReceipt.CapsuleAbsent || revocationReceipt.AgentRunID != ackRunID ||
		revocationReceipt.CapsuleID != assignment.Binding.CapsuleID || revocationReceipt.IntentRef != intentRef ||
		revocationReceipt.AssignmentCapabilityDigest != assignment.Binding.CapabilityDigest {
		return assignment, fmt.Errorf("persist exact structured capsule revoke acknowledgement: %w", receiptErr)
	}
	ackRef := revocationReceipt.ReceiptRef
	fateAck, fateAckErr := engineeringFateAckRequest(assignment, types.EngineeringCapsuleRevoked, intentRef, ackRef, "", "", revocationReceipt.OccurredAt, revocationReceipt.CapsuleAbsent)
	if fateAckErr != nil {
		return assignment, fmt.Errorf("invalid revoke receipt occurred_at: %w", fateAckErr)
	}
	acked, err := rt.store.SetEngineeringCapsuleDisposition(ctx, fateAck)
	if err != nil {
		return assignment, err
	}
	return acked.Assignment, nil
}

func (rt *Runtime) cancelAssignedEngineering(ctx context.Context, parent types.RunRecord, assignmentID string, attempt uint64, reason string) (types.EngineeringAssignmentCommandResult, error) {
	assignment, err := rt.store.GetEngineeringAssignment(ctx, parent.OwnerID, parent.ComputerID, strings.TrimSpace(assignmentID), attempt)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	if assignment.Binding.ParentRunID != parent.RunID || assignment.Binding.ParentAgentID != parent.AgentID {
		return types.EngineeringAssignmentCommandResult{}, fmt.Errorf("assignment cancellation requires the exact recorded parent")
	}
	if parent.RunID != "" && parent.AgentID != persistentManagementAgentID(parent.OwnerID) {
		return types.EngineeringAssignmentCommandResult{}, fmt.Errorf("assignment cancellation requires exact persistent Management parent")
	}
	cancel := types.CancelEngineeringAssignmentRequest{
		CommandID: fmt.Sprintf("co-super-cancel:%s:%d", assignment.AssignmentID, assignment.Binding.Attempt),
		OwnerID:   assignment.Binding.OwnerID, ComputerID: assignment.Binding.ComputerID,
		AssignmentID: assignment.AssignmentID, Attempt: assignment.Binding.Attempt,
		ExpectedLifecycleVersion: assignment.LifecycleVersion, Reason: strings.TrimSpace(reason),
	}
	if cancel.Reason == "" {
		cancel.Reason = "persistent Management cancelled assignment"
	}
	cancel.CommandDigest, _ = store.ComputeCancelEngineeringAssignmentDigest(cancel)
	if assignment.Disposition.Terminal() {
		// Exact cancellation replay/conflict is receipt-authoritative even after
		// terminal projection; never bypass it with an in-memory early return.
		replayed, replayErr := rt.store.CancelEngineeringAssignment(ctx, cancel)
		if replayErr != nil {
			return types.EngineeringAssignmentCommandResult{}, replayErr
		}
		return replayed, nil
	}
	if assignment.BoundRunID != "" {
		assignment, err = rt.revokeAssignedCapsule(ctx, assignment, reason)
		if err != nil {
			return types.EngineeringAssignmentCommandResult{}, err
		}
	}
	cancel.ExpectedLifecycleVersion = assignment.LifecycleVersion
	cancel.CommandDigest, _ = store.ComputeCancelEngineeringAssignmentDigest(cancel)
	cancelled, err := rt.store.CancelEngineeringAssignment(ctx, cancel)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	return cancelled, nil
}

func (rt *Runtime) persistSystemEngineeringCancellation(ctx context.Context, assignment types.EngineeringAssignment, reason string) (types.EngineeringAssignmentCommandResult, error) {
	for attempt := 0; attempt < 4; attempt++ {
		current, err := rt.store.GetEngineeringAssignment(ctx, assignment.Binding.OwnerID, assignment.Binding.ComputerID, assignment.AssignmentID, assignment.Binding.Attempt)
		if err != nil {
			return types.EngineeringAssignmentCommandResult{}, err
		}
		if current.Disposition.Terminal() {
			return types.EngineeringAssignmentCommandResult{Assignment: current, Replay: true}, nil
		}
		if current.CapsuleDisposition != types.EngineeringCapsuleRevoked {
			return types.EngineeringAssignmentCommandResult{}, fmt.Errorf("system assignment cancellation requires durable revoke acknowledgement")
		}
		cancel := types.CancelEngineeringAssignmentRequest{CommandID: fmt.Sprintf("co-super-system-cancel:%s:%d", current.AssignmentID, current.Binding.Attempt),
			OwnerID: current.Binding.OwnerID, ComputerID: current.Binding.ComputerID, AssignmentID: current.AssignmentID, Attempt: current.Binding.Attempt,
			ExpectedLifecycleVersion: current.LifecycleVersion, Reason: strings.TrimSpace(reason)}
		if cancel.Reason == "" {
			cancel.Reason = "system cancelled assignment fate"
		}
		cancel.CommandDigest, _ = store.ComputeCancelEngineeringAssignmentDigest(cancel)
		result, cancelErr := rt.store.CancelEngineeringAssignment(ctx, cancel)
		if cancelErr == nil {
			if !result.Replay && result.Update != nil {
				rt.wakeUpdatedCoagent(ctx, *result.Update)
			}
			return result, nil
		}
		if !errors.Is(cancelErr, store.ErrEngineeringAssignmentInvalid) && !errors.Is(cancelErr, store.ErrConcurrentStateChange) {
			return types.EngineeringAssignmentCommandResult{}, cancelErr
		}
	}
	return types.EngineeringAssignmentCommandResult{}, store.ErrConcurrentStateChange
}

func (rt *Runtime) prepareEngineeringTrajectoryCancellation(ctx context.Context, ownerID, computerID, trajectoryID, reason string) ([]types.EngineeringAssignment, error) {
	if rt == nil || rt.store == nil || rt.assignedCapsule() == nil || strings.TrimSpace(computerID) == "" {
		return nil, nil
	}
	assignments, err := rt.store.ListEngineeringAssignments(ctx, ownerID, computerID, trajectoryID)
	if err != nil {
		return nil, err
	}
	prepared := make([]types.EngineeringAssignment, 0, len(assignments))
	for _, assignment := range assignments {
		// Terminal assignment outcome does not by itself prove executor fate;
		// cancellation closes every non-revoked capsule before trajectory fate.
		if assignment.CapsuleDisposition != types.EngineeringCapsuleRevoked {
			assignment, err = rt.revokeAssignedCapsule(ctx, assignment, reason)
			if err != nil {
				return prepared, err
			}
		}
		prepared = append(prepared, assignment)
	}
	return prepared, nil
}

func (rt *Runtime) finishEngineeringTrajectoryCancellation(ctx context.Context, ownerID, computerID, trajectoryID, reason string) error {
	if rt == nil || rt.store == nil || strings.TrimSpace(computerID) == "" {
		return nil
	}
	assignments, err := rt.store.ListEngineeringAssignments(ctx, ownerID, computerID, trajectoryID)
	if err != nil {
		return err
	}
	for _, assignment := range assignments {
		if assignment.Disposition.Terminal() {
			continue
		}
		if assignment.CapsuleDisposition != types.EngineeringCapsuleRevoked {
			return fmt.Errorf("assignment %s cancellation missing durable executor revoke acknowledgement", assignment.AssignmentID)
		}
		if _, err := rt.persistSystemEngineeringCancellation(ctx, assignment, reason); err != nil {
			return err
		}
	}
	return nil
}

func (rt *Runtime) cancelBoundEngineeringRun(ctx context.Context, rec types.RunRecord, reason string) (bool, error) {
	assignmentID := metadataStringValue(rec.Metadata, "assignment_id")
	attempt := uint64(metadataIntValue(rec.Metadata, "assignment_attempt"))
	if assignmentID == "" || attempt == 0 {
		return false, nil
	}
	assignment, err := rt.store.GetEngineeringAssignment(ctx, rec.OwnerID, rec.ComputerID, assignmentID, attempt)
	if err != nil {
		return true, err
	}
	if assignment.BoundRunID != rec.RunID || assignment.Binding.AssignedAgentID != rec.AgentID {
		return true, fmt.Errorf("cancel run assignment binding mismatch")
	}
	if assignment.Disposition.Terminal() {
		if assignment.CapsuleDisposition != types.EngineeringCapsuleRevoked {
			_, err = rt.revokeAssignedCapsule(ctx, assignment, reason)
		}
		return true, err
	}
	assignment, err = rt.revokeAssignedCapsule(ctx, assignment, reason)
	if err != nil {
		return true, err
	}
	_, err = rt.persistSystemEngineeringCancellation(ctx, assignment, reason)
	return true, err
}

// ReconcileEngineeringAssignmentsForTrajectory closes restart gaps without a
// poller. It is called from existing actor/runtime reconstruction for the exact
// lifecycle trajectory: an absent executor capsule is first recorded as a
// durable revoke intent, then acknowledged absent, then cancelled. No wake or
// attempt reopen follows.
func (rt *Runtime) ReconcileEngineeringAssignmentsForTrajectory(ctx context.Context, ownerID, computerID, trajectoryID string) error {
	if rt == nil || rt.store == nil || rt.assignedCapsule() == nil {
		return nil
	}
	exec := rt.assignedCapsule()
	assignments, err := rt.store.ListEngineeringAssignments(ctx, ownerID, computerID, trajectoryID)
	if err != nil {
		return err
	}
	trajectory, trajectoryErr := rt.store.GetLifecycleTrajectory(ctx, ownerID, computerID, trajectoryID)
	if trajectoryErr != nil {
		return trajectoryErr
	}
	if trajectory.Status == types.TrajectoryLive {
		if intent, intentErr := rt.store.GetLifecycleCancellationIntent(ctx, ownerID, computerID, trajectoryID); intentErr == nil {
			result, resumeErr := rt.cancelTrajectoryAuthorityCommand(ctx, ownerID, trajectoryID, intent.CommandID, intent.Reason, intent.RequestedLifecycleVersion, intent.ExpectedHeadRevisionID)
			if resumeErr != nil {
				return resumeErr
			}
			if result.Trajectory.Status == types.TrajectoryCancelled {
				if finishErr := rt.finishEngineeringTrajectoryCancellation(ctx, ownerID, computerID, trajectoryID, intent.Reason); finishErr != nil {
					return finishErr
				}
				return nil
			}
		} else if !errors.Is(intentErr, store.ErrNotFound) {
			return intentErr
		}
	}
	for _, assignment := range assignments {
		if assignment.Disposition.Terminal() {
			// Unbound means the capsule was never bound (spawn failed before
			// bind), so there is nothing to revoke. Attempting to revoke it
			// fails the SetEngineeringCapsuleDisposition guard (BoundRunID empty
			// on a non-open assignment) and aborts the whole trajectory.
			if assignment.CapsuleDisposition != types.EngineeringCapsuleRevoked && assignment.CapsuleDisposition != types.EngineeringCapsuleUnbound {
				if _, fateErr := rt.revokeAssignedCapsule(ctx, assignment, "restart terminal assignment fate reconciliation"); fateErr != nil {
					return fateErr
				}
			}
			continue
		}
		if trajectory.Status != types.TrajectoryLive {
			if assignment.CapsuleDisposition != types.EngineeringCapsuleRevoked {
				assignment, err = rt.revokeAssignedCapsule(ctx, assignment, "restart terminal trajectory fate reconciliation")
				if err != nil {
					return err
				}
			}
			if _, cancelErr := rt.persistSystemEngineeringCancellation(ctx, assignment, "restart completed terminal trajectory assignment fate"); cancelErr != nil {
				return cancelErr
			}
			continue
		}
		if assignment.BoundRunID == "" {
			intent := "capsule-revoke-intent:" + objectgraph.SHA256([]byte(assignment.AssignmentID+"\x00restart-pre-bind"))
			if assignment.CapsuleDisposition == types.EngineeringCapsuleUnbound {
				requested, fateErr := rt.store.SetEngineeringCapsuleDisposition(ctx, engineeringFateRequest(assignment, types.EngineeringCapsuleRevokeRequested, intent, ""))
				if fateErr != nil {
					return fateErr
				}
				assignment = requested.Assignment
			}
			if assignment.CapsuleDisposition == types.EngineeringCapsuleRevokeRequested {
				if assignment.CapsuleIntentRef != intent {
					return store.ErrEngineeringAssignmentCommandConflict
				}
				if exec.HasCapsule(assignment.Binding.CapsuleID) {
					if destroyErr := exec.ForceDestroy(ctx, assignment.Binding.CapsuleID); destroyErr != nil {
						return destroyErr
					}
				}
				if exec.HasCapsule(assignment.Binding.CapsuleID) {
					return fmt.Errorf("restart pre-bind capsule remained after revoke effect")
				}
				ackRunID := "unbound:" + assignment.AssignmentID
				receipt, receiptErr := exec.PersistRevocationReceipt(ackRunID, assignment.Binding.CapabilityDigest, assignment.Binding.CapsuleID, intent)
				if receiptErr != nil {
					return receiptErr
				}
				fateAck, fateAckErr := engineeringFateAckRequest(assignment, types.EngineeringCapsuleRevoked, intent, receipt.ReceiptRef, "", "", receipt.OccurredAt, receipt.CapsuleAbsent)
				if fateAckErr != nil {
					return fmt.Errorf("invalid revoke receipt occurred_at: %w", fateAckErr)
				}
				acked, fateErr := rt.store.SetEngineeringCapsuleDisposition(ctx, fateAck)
				if fateErr != nil {
					return fateErr
				}
				assignment = acked.Assignment
			}
			if assignment.CapsuleDisposition != types.EngineeringCapsuleRevoked {
				return fmt.Errorf("restart open assignment has ambiguous capsule fate %s", assignment.CapsuleDisposition)
			}
			cancel := types.CancelEngineeringAssignmentRequest{
				CommandID: fmt.Sprintf("co-super-restart-open-cancel:%s:%d", assignment.AssignmentID, assignment.Binding.Attempt),
				OwnerID:   ownerID, ComputerID: computerID, AssignmentID: assignment.AssignmentID, Attempt: assignment.Binding.Attempt,
				ExpectedLifecycleVersion: assignment.LifecycleVersion, Reason: "restart acknowledged absent pre-bind assignment capsule",
			}
			cancel.CommandDigest, _ = store.ComputeCancelEngineeringAssignmentDigest(cancel)
			cancelled, cancelErr := rt.store.CancelEngineeringAssignment(ctx, cancel)
			if cancelErr != nil {
				return cancelErr
			}
			if !cancelled.Replay && cancelled.Update != nil {
				rt.wakeUpdatedCoagent(ctx, *cancelled.Update)
			}
			continue
		}
		// A Complete reduce that failed mid-terminal-saga strands the
		// assignment bound with a frozen capsule and a staged pending
		// proposal: the worker cannot run another cell (the capsule is
		// physically frozen) and the restart-cancel branch below would
		// destroy the work evidence. The reducer authors the fate from the
		// staged proposal instead — same intent identity, so the store
		// replays instead of minting a second report.
		if assignedEngineeringFatePending(assignment) {
			if resumeErr := rt.resumeStrandedFrozenAssignmentCommit(ctx, assignment); resumeErr != nil {
				log.Printf("runtime: assignment %s stranded frozen proposal resume: %v", assignment.AssignmentID, resumeErr)
			}
			continue
		}
		usable := rt.assignedEngineeringCapsuleUsable(assignment)
		if usable {
			continue
		}
		assignment, err = rt.revokeAssignedCapsule(ctx, assignment, "restart executor binding reconciliation")
		if err != nil {
			return err
		}
		cancel := types.CancelEngineeringAssignmentRequest{
			CommandID: fmt.Sprintf("co-super-restart-cancel:%s:%d", assignment.AssignmentID, assignment.Binding.Attempt),
			OwnerID:   ownerID, ComputerID: computerID, AssignmentID: assignment.AssignmentID, Attempt: assignment.Binding.Attempt,
			ExpectedLifecycleVersion: assignment.LifecycleVersion, Reason: "restart revoked absent assignment capsule",
		}
		cancel.CommandDigest, _ = store.ComputeCancelEngineeringAssignmentDigest(cancel)
		cancelled, cancelErr := rt.store.CancelEngineeringAssignment(ctx, cancel)
		if cancelErr != nil {
			return cancelErr
		}
		if !cancelled.Replay && cancelled.Update != nil {
			rt.wakeUpdatedCoagent(ctx, *cancelled.Update)
		}
	}
	return nil
}

// reconcileEngineeringAssignmentCapsulesAfterRestart closes restart gaps for every
// durable Engineering assignment in the computer, independent of the run-state
// metadata index that assignments bound before it was introduced may lack. It
// delegates to the per-trajectory reconciler so revoke, cancel, and run
// terminalization share one authority path.
func (rt *Runtime) reconcileEngineeringAssignmentCapsulesAfterRestart(ctx context.Context) {
	if rt == nil || rt.store == nil || rt.assignedCapsule() == nil {
		return
	}
	computerID := strings.TrimSpace(rt.TextureComputerID())
	if computerID == "" {
		return
	}
	assignments, err := rt.store.ListEngineeringAssignmentsForComputer(ctx, computerID)
	if err != nil {
		log.Printf("runtime: boot Engineering assignment capsule sweep: %v", err)
		return
	}
	seen := make(map[string]struct{})
	for _, assignment := range assignments {
		if assignment.Disposition.Terminal() {
			continue
		}
		key := assignment.Binding.OwnerID + "\x00" + assignment.Binding.TrajectoryID
		if _, done := seen[key]; done {
			continue
		}
		seen[key] = struct{}{}
		if err := rt.ReconcileEngineeringAssignmentsForTrajectory(ctx, assignment.Binding.OwnerID, assignment.Binding.ComputerID, assignment.Binding.TrajectoryID); err != nil {
			log.Printf("runtime: boot Engineering assignment capsule sweep trajectory %s: %v", assignment.Binding.TrajectoryID, err)
		}
	}
}

func (rt *Runtime) recordAssignedEngineeringReport(ctx context.Context, rec *types.RunRecord, toolCallID string, report types.EngineeringAssignmentReport) (types.EngineeringAssignmentCommandResult, error) {
	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		result, err := rt.recordAssignedEngineeringReportOnce(ctx, rec, toolCallID, report)
		if err == nil {
			return result, nil
		}
		lastErr = err
		if !errors.Is(err, store.ErrEngineeringAssignmentInvalid) && !errors.Is(err, store.ErrConcurrentStateChange) {
			return types.EngineeringAssignmentCommandResult{}, err
		}
		// A cancellation/report race may invalidate the live freeze CAS or
		// advance assignment fate after late authority was read. Reload the
		// exact immutable binding and retry only after cancellation/revocation
		// has won. The next attempt is forced through the evidence-only path.
		assignmentID := metadataStringValue(rec.Metadata, "assignment_id")
		assignmentAttempt := uint64(metadataIntValue(rec.Metadata, "assignment_attempt"))
		current, loadErr := rt.store.GetEngineeringAssignment(ctx, rec.OwnerID, rec.ComputerID, assignmentID, assignmentAttempt)
		if loadErr != nil {
			return types.EngineeringAssignmentCommandResult{}, loadErr
		}
		_, intentErr := rt.store.GetLifecycleCancellationIntent(ctx, rec.OwnerID, rec.ComputerID, current.Binding.TrajectoryID)
		cancellationWon := intentErr == nil
		if intentErr != nil && !errors.Is(intentErr, store.ErrNotFound) {
			return types.EngineeringAssignmentCommandResult{}, intentErr
		}
		if !cancellationWon && !current.Disposition.Terminal() && current.CapsuleDisposition != types.EngineeringCapsuleRevokeRequested && current.CapsuleDisposition != types.EngineeringCapsuleRevoked {
			return types.EngineeringAssignmentCommandResult{}, err
		}
	}
	return types.EngineeringAssignmentCommandResult{}, fmt.Errorf("retain assignment report after cancellation race: %w", lastErr)
}

func (rt *Runtime) recordAssignedEngineeringReportOnce(ctx context.Context, rec *types.RunRecord, toolCallID string, report types.EngineeringAssignmentReport) (types.EngineeringAssignmentCommandResult, error) {
	if rec == nil || rt.capsuleExecutor == nil || strings.TrimSpace(toolCallID) == "" {
		return types.EngineeringAssignmentCommandResult{}, fmt.Errorf("assigned Engineering report authority unavailable")
	}
	assignmentID := metadataStringValue(rec.Metadata, "assignment_id")
	attempt := uint64(metadataIntValue(rec.Metadata, "assignment_attempt"))
	assignment, err := rt.store.GetEngineeringAssignment(ctx, rec.OwnerID, rec.ComputerID, assignmentID, attempt)
	if err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	if assignment.BoundRunID != rec.RunID || assignment.Binding.AssignedAgentID != rec.AgentID {
		return types.EngineeringAssignmentCommandResult{}, fmt.Errorf("report run is not the exact bound assignment")
	}
	// v1 terminal identity (settlement gate item 3): the proposition digest
	// covers the submitted claims with the pinned pre-execution belief.
	// Provider-call identifiers, summary prose, and transport metadata never
	// enter identity. The canonical observed input is the binding subject
	// digest pinned server-side; post-reservation ack facts overlay derivation
	// and never rewrite the digest. Cancellation-wins is a reducer disposition
	// recorded by the store; the submitted packet is never rewritten here.
	report.ObservedSubjectDigest = assignment.Binding.SubjectDigest
	terminal := report.Result != types.EngineeringResultPartial
	propositionDigest := ""
	if terminal {
		var propErr error
		propositionDigest, propErr = store.ComputeTerminalPropositionDigest(
			assignment.Binding.SubjectDigest,
			report.Result, report.Verdict,
			report.Commands, report.Outputs, report.EvidenceRefs)
		if propErr != nil {
			return types.EngineeringAssignmentCommandResult{}, propErr
		}
		report.ReportID = store.TerminalReportID(assignment.Binding.OwnerID, assignment.Binding.ComputerID, assignmentID, attempt, propositionDigest)
		report.PropositionDigest = propositionDigest
	} else {
		report.ReportID = "report:" + objectgraph.SHA256([]byte(strings.Join([]string{
			"choir:co-super-report:v1", assignment.Binding.OwnerID, assignment.Binding.ComputerID, rec.RunID, assignmentID, fmt.Sprint(attempt), strings.TrimSpace(toolCallID),
		}, "\x00")))
		report.PropositionDigest = ""
	}
	cancellationIntended := false
	if _, intentErr := rt.store.GetLifecycleCancellationIntent(ctx, rec.OwnerID, rec.ComputerID, assignment.Binding.TrajectoryID); intentErr == nil {
		cancellationIntended = true
	} else if !errors.Is(intentErr, store.ErrNotFound) {
		return types.EngineeringAssignmentCommandResult{}, intentErr
	}
	// The saga's own mid-flight dispositions are not late fate: a staged
	// pending proposal whose proposition digest matches this report is the
	// same terminal commit resuming after a strand, and must re-enter the
	// freeze/revoke path instead of degrading to late evidence. This mirrors
	// the store's pendingMatches exclusion in RecordEngineeringAssignmentReport.
	pendingMatches := assignment.PendingProposal != nil && assignment.PendingProposal.PropositionDigest == propositionDigest &&
		!cancellationIntended && !assignment.Disposition.Terminal()
	lateFate := (cancellationIntended || assignment.Disposition.Terminal() || assignment.CapsuleDisposition == types.EngineeringCapsuleRevokeRequested || assignment.CapsuleDisposition == types.EngineeringCapsuleRevoked) && !pendingMatches
	storedReport, reportErr := rt.store.GetEngineeringAssignmentReport(ctx, assignment.Binding.OwnerID, assignment.Binding.ComputerID, report.ReportID)
	reportExists := reportErr == nil
	if reportErr != nil && !errors.Is(reportErr, store.ErrNotFound) {
		return types.EngineeringAssignmentCommandResult{}, reportErr
	}
	if reportExists {
		// The derived ReportID already binds the digest; a stored row with a
		// different digest is a defensive conflict (legacy rows without the
		// field recompute best-effort from the pinned belief and can only
		// fail closed here).
		storedDigest := strings.TrimSpace(storedReport.PropositionDigest)
		if storedDigest == "" {
			var recomputeErr error
			storedDigest, recomputeErr = store.ComputeTerminalPropositionDigest(
				assignment.Binding.SubjectDigest,
				storedReport.Result, storedReport.Verdict,
				storedReport.Commands, storedReport.Outputs, storedReport.EvidenceRefs)
			if recomputeErr != nil {
				return types.EngineeringAssignmentCommandResult{}, store.ErrEngineeringAssignmentCommandConflict
			}
		}
		if storedDigest != propositionDigest {
			return types.EngineeringAssignmentCommandResult{}, store.ErrEngineeringAssignmentCommandConflict
		}
		report = storedReport
	}
	if terminal && !reportExists {
		// Slot gate before any physical effect: a same-digest occupant replays
		// its receipt with no new effects; a non-late terminal occupant with a
		// different digest conflicts a non-late submission, while late evidence
		// never competes for terminal truth.
		matchID, matchCommandID, conflictID, scanErr := rt.store.SlotTerminalReport(ctx, assignment, propositionDigest)
		if scanErr != nil {
			return types.EngineeringAssignmentCommandResult{}, scanErr
		}
		if matchID != "" {
			return rt.store.ReplayRecordedEngineeringAssignmentReport(ctx, assignment.Binding.OwnerID, assignment.Binding.ComputerID,
				assignment.AssignmentID, assignment.Binding.Attempt, matchID, matchCommandID)
		}
		if conflictID != "" && !lateFate {
			return types.EngineeringAssignmentCommandResult{}, store.ErrEngineeringAssignmentCommandConflict
		}
	}
	if !terminal && reportExists {
		return rt.store.ReplayRecordedEngineeringAssignmentReport(ctx, assignment.Binding.OwnerID, assignment.Binding.ComputerID,
			assignment.AssignmentID, assignment.Binding.Attempt, report.ReportID, "co-super-report:"+assignment.AssignmentID+":"+report.ReportID)
	}

	// Cancellation/revocation wins the lifecycle race. A provider tool call
	// already in flight may still commit its authenticated report identity, but
	// Store derives it as late evidence only: no packet, wake, projection, or
	// capsule effect can reopen/revise the cancelled assignment.
	if lateFate {
		if reportExists {
			if assignment.CapsuleDisposition != types.EngineeringCapsuleRevoked {
				assignment, err = rt.revokeAssignedCapsule(ctx, assignment, "terminal assignment report recorded")
				if err != nil {
					return types.EngineeringAssignmentCommandResult{}, err
				}
			}
			return rt.store.ReplayRecordedEngineeringAssignmentReport(ctx, assignment.Binding.OwnerID, assignment.Binding.ComputerID,
				assignment.AssignmentID, assignment.Binding.Attempt, report.ReportID, "co-super-report:"+assignment.AssignmentID+":"+report.ReportID)
		}
		report, err = rt.bindLateAssignmentExecutionReceipts(assignment, report)
		if err != nil {
			return types.EngineeringAssignmentCommandResult{}, err
		}
		return rt.commitAssignedEngineeringReport(ctx, assignment, report)
	}
	if !terminal {
		if assignment.CapsuleDisposition != types.EngineeringCapsuleActive || assignment.Disposition.Terminal() {
			return types.EngineeringAssignmentCommandResult{}, fmt.Errorf("partial report requires active nonterminal assignment")
		}
		return rt.commitAssignedEngineeringReport(ctx, assignment, report)
	}

	intent := "capsule-freeze-intent:" + propositionDigest
	switch assignment.CapsuleDisposition {
	case types.EngineeringCapsuleActive:
		if reportExists {
			return types.EngineeringAssignmentCommandResult{}, fmt.Errorf("stored terminal report cannot precede freeze intent")
		}
		proposal := &types.EngineeringPendingProposal{
			PropositionDigest: propositionDigest,
			Report:            report,
			FreezeIntentRef:   intent,
			CreatedAt:         time.Now().UTC(),
		}
		freezeReq := engineeringFateRequest(assignment, types.EngineeringCapsuleFreezeRequested, intent, "")
		freezeReq.PendingProposal = proposal
		freezeReq.CommandDigest, _ = store.ComputeSetEngineeringCapsuleDispositionDigest(freezeReq)
		requested, err := rt.store.SetEngineeringCapsuleDisposition(ctx, freezeReq)
		if err != nil {
			return types.EngineeringAssignmentCommandResult{}, err
		}
		assignment = requested.Assignment
	case types.EngineeringCapsuleFreezeRequested:
		if assignment.CapsuleIntentRef != intent {
			return types.EngineeringAssignmentCommandResult{}, store.ErrEngineeringAssignmentCommandConflict
		}
	case types.EngineeringCapsuleFrozen, types.EngineeringCapsuleRevokeRequested, types.EngineeringCapsuleRevoked:
		if assignment.CapsuleIntentRef != intent && !reportExists &&
			assignment.CapsuleIntentRef != assignedEngineeringTerminalRevokeIntent(assignment) {
			return types.EngineeringAssignmentCommandResult{}, store.ErrEngineeringAssignmentCommandConflict
		}
	default:
		return types.EngineeringAssignmentCommandResult{}, fmt.Errorf("terminal report cannot resume capsule disposition %s", assignment.CapsuleDisposition)
	}

	if assignment.CapsuleDisposition == types.EngineeringCapsuleFreezeRequested {
		freezeFailure := func(err error) (types.EngineeringAssignmentCommandResult, error) {
			rt.armAssignedEngineeringFateWatchdog(assignment)
			return types.EngineeringAssignmentCommandResult{}, err
		}
		handle, err := rt.capsuleExecutor.AssignmentHandle(rec.RunID, assignment.Binding.CapsuleID)
		if err != nil {
			return freezeFailure(fmt.Errorf("resolve exact assignment capability after freeze intent: %w", err))
		}
		_, err = rt.capsuleExecutor.ExtractGranted(ctx, rec.RunID, handle)
		if err != nil {
			return freezeFailure(fmt.Errorf("freeze assignment after durable intent: %w", err))
		}
		diagnostics, err := rt.capsuleExecutor.InspectCapsuleRaw(assignment.Binding.CapsuleID)
		if err != nil || diagnostics.ID != assignment.Binding.CapsuleID || diagnostics.State != capsule.StateFrozen {
			return freezeFailure(fmt.Errorf("executor did not acknowledge exact frozen assignment capsule: %w", err))
		}
		frozenDigest, err := rt.capsuleExecutor.ResolveGrantedWorktreeDigest(ctx, rec.RunID, handle)
		if err != nil || !types.ValidSHA256Digest(frozenDigest) {
			return freezeFailure(fmt.Errorf("frozen assignment digest unavailable: %w", err))
		}
		freezeReceipt, receiptErr := rt.capsuleExecutor.PersistGrantedFreezeReceipt(ctx, rec.RunID, handle)
		if receiptErr != nil || freezeReceipt.CapsuleID != assignment.Binding.CapsuleID || "sha256:"+strings.TrimPrefix(freezeReceipt.FinalSubjectDigest, "sha256:") != frozenDigest || "sha256:"+strings.TrimPrefix(freezeReceipt.SourceSubjectDigest, "sha256:") != assignment.Binding.SubjectDigest {
			return freezeFailure(fmt.Errorf("durable typed executor freeze receipt unavailable: %w", receiptErr))
		}
		ack := freezeReceipt.ReceiptRef
		fateAck, fateAckErr := engineeringFateAckRequest(assignment, types.EngineeringCapsuleFrozen, intent, ack, "sha256:"+strings.TrimPrefix(freezeReceipt.SourceSubjectDigest, "sha256:"), "sha256:"+strings.TrimPrefix(freezeReceipt.FinalSubjectDigest, "sha256:"), freezeReceipt.OccurredAt, false)
		if fateAckErr != nil {
			return freezeFailure(fmt.Errorf("invalid freeze receipt occurred_at: %w", fateAckErr))
		}
		if assignment.PendingProposal != nil {
			fateAck.PendingProposal = assignment.PendingProposal
			fateAck.CommandDigest, _ = store.ComputeSetEngineeringCapsuleDispositionDigest(fateAck)
		}
		frozen, err := rt.store.SetEngineeringCapsuleDisposition(ctx, fateAck)
		if err != nil {
			return freezeFailure(err)
		}
		assignment = frozen.Assignment
		if frozenDigest != assignment.Binding.SubjectDigest {
			report.ObservedSubjectDigest = frozenDigest
			report.Mutations = []types.EngineeringRecordedMutation{{
				MutationID: "assignment-overlay:" + objectgraph.SHA256([]byte(ack)), Kind: "assignment_overlay",
				BeforeDigest: assignment.Binding.SubjectDigest, AfterDigest: frozenDigest,
				EvidenceRef: "capsule-diff:" + objectgraph.SHA256([]byte(ack)), SubjectBytesChanged: true,
			}}
		}
	}
	if (assignment.CapsuleDisposition == types.EngineeringCapsuleFrozen || assignment.CapsuleDisposition == types.EngineeringCapsuleRevokeRequested) && !reportExists {
		// A revoke_requested strand is the saga's own mid-flight state: the
		// executor capsule is still physically frozen until the revoke
		// effects run, so the receipt binding re-drives here.
		handle, resolveErr := rt.capsuleExecutor.AssignmentHandle(rec.RunID, assignment.Binding.CapsuleID)
		if resolveErr != nil {
			return types.EngineeringAssignmentCommandResult{}, fmt.Errorf("resolve frozen assignment capability: %w", resolveErr)
		}
		frozenDigest, digestErr := rt.capsuleExecutor.ResolveGrantedWorktreeDigest(ctx, rec.RunID, handle)
		if digestErr != nil || !types.ValidSHA256Digest(frozenDigest) {
			return types.EngineeringAssignmentCommandResult{}, fmt.Errorf("already-frozen assignment digest unavailable: %w", digestErr)
		}
		report, resolveErr = rt.bindFrozenAssignmentExecutionReceipts(ctx, assignment, handle, report)
		if resolveErr != nil {
			return types.EngineeringAssignmentCommandResult{}, resolveErr
		}
		if frozenDigest != assignment.Binding.SubjectDigest && len(report.Mutations) == 0 {
			report.ObservedSubjectDigest = frozenDigest
			report.Mutations = []types.EngineeringRecordedMutation{{MutationID: "assignment-subject:" + objectgraph.SHA256([]byte(assignment.CapsuleAckRef)), Kind: "workspace_platform_complete_tree", BeforeDigest: assignment.Binding.SubjectDigest, AfterDigest: frozenDigest, EvidenceRef: "capsule-diff:" + objectgraph.SHA256([]byte(assignment.CapsuleAckRef)), SubjectBytesChanged: true}}
		}
		if frozenDigest != assignment.Binding.SubjectDigest {
			candidate, candidateErr := rt.capsuleExecutor.PersistGrantedCandidate(ctx, rec.RunID, handle)
			if candidateErr != nil || "sha256:"+strings.TrimPrefix(candidate.SubjectDigest, "sha256:") != frozenDigest {
				return types.EngineeringAssignmentCommandResult{}, fmt.Errorf("persist reconstructable content-addressed candidate: %w", candidateErr)
			}
			report.CandidateArtifactRef = candidate.ArtifactRef
		}
		// Persist the receipt-bound report into the staged proposal so a
		// post-revoke resume (watchdog, sweep, restart reconcile) commits the
		// same evidence without re-resolving executor state that no longer
		// exists. The proposal digest is unchanged — receipts are derived
		// evidence, not submitted claims.
		if assignment.PendingProposal != nil && assignment.PendingProposal.PropositionDigest == propositionDigest {
			proposalReport := report
			proposalReport.ObservedSubjectDigest = assignment.PendingProposal.Report.ObservedSubjectDigest
			assignment.PendingProposal.Report = proposalReport
		}
	}
	// A revoked strand still owes the receipt binding: the live saga binds
	// granted receipts while the capsule is frozen, but a resume after revoke
	// finds the capability and frozen state gone. The raw execution receipts
	// are durable artifacts — resolve them and carry the refs forward so the
	// commit sees the same evidence the freeze certified.
	if assignment.CapsuleDisposition == types.EngineeringCapsuleRevoked && !reportExists && len(report.ExecutorReceiptRefs) != len(report.Commands) {
		report, err = rt.bindLateAssignmentExecutionReceipts(assignment, report)
		if err != nil {
			return types.EngineeringAssignmentCommandResult{}, err
		}
	}

	var result types.EngineeringAssignmentCommandResult
	if reportExists {
		if assignment.CapsuleDisposition != types.EngineeringCapsuleRevoked {
			assignment, err = rt.revokeAssignedCapsule(ctx, assignment, "terminal assignment report recorded")
			if err != nil {
				return types.EngineeringAssignmentCommandResult{}, err
			}
		}
		commandID := "co-super-report:" + assignment.AssignmentID + ":" + report.ReportID
		result, err = rt.store.ReplayRecordedEngineeringAssignmentReport(ctx, assignment.Binding.OwnerID, assignment.Binding.ComputerID, assignment.AssignmentID, assignment.Binding.Attempt, report.ReportID, commandID)
	} else {
		// Revocation must precede terminal commitment! No terminal state is visible until revoke ack.
		if assignment.CapsuleDisposition != types.EngineeringCapsuleRevoked {
			assignment, err = rt.revokeAssignedCapsule(ctx, assignment, "terminal assignment report recorded")
			if err != nil {
				return types.EngineeringAssignmentCommandResult{}, err
			}
		}
		if assignment.CapsuleDisposition != types.EngineeringCapsuleRevoked {
			return types.EngineeringAssignmentCommandResult{}, fmt.Errorf("terminal report requires revoked executor acknowledgement")
		}
		result, err = rt.commitAssignedEngineeringReport(ctx, assignment, report)
	}
	if err != nil {
		// Name the exact strand state on every failed terminal attempt: the
		// saga's partial dispositions and the failing step were invisible in
		// the journal for assignments 121ae9fe and e8592727, which blocked
		// diagnosis for hours.
		log.Printf("runtime: assignment %s terminal report attempt failed: disposition=%s capsule=%s version=%d err=%v",
			assignment.AssignmentID, assignment.Disposition, assignment.CapsuleDisposition, assignment.LifecycleVersion, err)
		// A failed terminal attempt strands the assignment on whichever
		// pending fate disposition the saga already committed. Arm the fate
		// watchdog so the continuation re-drives without depending on the
		// worker's next cell (a frozen capsule refuses every operation) or
		// on a restart.
		rt.armAssignedEngineeringFateWatchdog(assignment)
		return types.EngineeringAssignmentCommandResult{}, err
	}
	return result, nil
}

// assignedEngineeringTerminalRevokeIntent recomputes the revoke intent the
// terminal saga itself mints in revokeAssignedCapsule for the fixed reason
// "terminal assignment report recorded". A stranded revoke_requested strand
// carries exactly this intent, which lets the same terminal report re-enter
// the saga instead of conflicting with the saga's own mid-flight state.
func assignedEngineeringTerminalRevokeIntent(assignment types.EngineeringAssignment) string {
	return "capsule-revoke-intent:" + objectgraph.SHA256([]byte(strings.Join([]string{
		assignment.AssignmentID, fmt.Sprint(assignment.Binding.Attempt), assignment.BoundRunID, assignment.Binding.CapsuleID,
		"terminal assignment report recorded",
	}, "\x00")))
}

// assignedEngineeringFatePending reports whether one assignment still owes a
// continuation after a committed fate disposition: a nonterminal bound
// assignment whose capsule moved past active with a staged pending proposal
// has a terminal saga mid-flight that nobody else will finish.
func assignedEngineeringFatePending(assignment types.EngineeringAssignment) bool {
	if assignment.Disposition != types.EngineeringAssignmentBound || assignment.PendingProposal == nil {
		return false
	}
	switch assignment.CapsuleDisposition {
	case types.EngineeringCapsuleFreezeRequested, types.EngineeringCapsuleFrozen, types.EngineeringCapsuleRevokeRequested, types.EngineeringCapsuleRevoked:
		return true
	default:
		return false
	}
}

// armAssignedEngineeringFateWatchdog is the fate-transition watchdog: after a
// committed disposition leaves pending fate work, a delayed re-drive confirms
// the strand predicate and finishes the saga. If the in-flight commit landed
// first the re-read resolves terminal or active and the watchdog no-ops; a
// process death before firing is covered by boot reconciliation. The
// kernel-scheduled bound deadline separately protects an active assignment.
func (rt *Runtime) armAssignedEngineeringFateWatchdog(assignment types.EngineeringAssignment) {
	if rt == nil || !assignedEngineeringFatePending(assignment) {
		return
	}
	assignmentID, attempt := assignment.AssignmentID, assignment.Binding.Attempt
	ownerID, computerID := assignment.Binding.OwnerID, assignment.Binding.ComputerID
	deadline := time.Now().UTC().Add(assignedEngineeringFateWatchdogDelay)
	content, err := encodeAssignedEngineeringFateDeadline(assignmentID, attempt)
	if err != nil {
		log.Printf("runtime: encode assignment %s fate deadline: %v", assignmentID, err)
		return
	}
	// The durable not_before wake is the continuation authority under kernel
	// mode; the dispatcher fires HandleAssignedEngineeringFateDeadline at the
	// deadline. No process-local timer remains.
	rt.scheduleContinuation(context.Background(), ownerID, computerID, assignment.Binding.ParentAgentID,
		assignedEngineeringFateDeadlineUpdateKind, content, assignment.Binding.TrajectoryID, "", deadline)
}

// resumeStrandedFateAssignmentIfPending re-reads the assignment and finishes
// the stranded terminal saga when the strand predicate still holds.
func (rt *Runtime) resumeStrandedFateAssignmentIfPending(ctx context.Context, ownerID, computerID, assignmentID string, attempt uint64) error {
	assignment, err := rt.store.GetEngineeringAssignment(ctx, ownerID, computerID, assignmentID, attempt)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil
		}
		return err
	}
	if !assignedEngineeringFatePending(assignment) {
		return nil
	}
	if resumeErr := rt.resumeStrandedFrozenAssignmentCommit(ctx, assignment); resumeErr != nil {
		return resumeErr
	}
	log.Printf("runtime: assignment %s fate watchdog re-drove stranded %s proposal", assignmentID, assignment.CapsuleDisposition)
	return nil
}

func (rt *Runtime) bindLateAssignmentExecutionReceipts(assignment types.EngineeringAssignment, report types.EngineeringAssignmentReport) (types.EngineeringAssignmentReport, error) {
	refs := make([]string, 0, len(report.Commands))
	for _, command := range report.Commands {
		refs = append(refs, command.ExecutionRef)
	}
	if len(refs) == 0 {
		return report, nil
	}
	if !types.ValidSHA256Digest(assignment.Binding.ExecutionHandleDigest) {
		return report, fmt.Errorf("late assignment raw execution evidence requires exact stored execution handle digest")
	}
	resolver := rt.assignmentReceiptResolver
	if resolver == nil {
		resolver = rt.capsuleExecutor
	}
	if resolver == nil {
		return report, fmt.Errorf("late assignment raw execution evidence resolver unavailable")
	}
	receipts, err := resolver.ResolveExecutionReceipts(refs)
	if err != nil || len(receipts) != len(refs) {
		return report, fmt.Errorf("late assignment raw execution evidence unavailable: %w", err)
	}
	seen := map[string]bool{}
	for i, receipt := range receipts {
		// SourceTreeDigest is the tree before this command ran — only the
		// first command's equals the binding subject; later commands' source
		// trees are prior commands' results. The freeze already certified the
		// final subject; here we authenticate run/capsule/handle/command
		// binding, not per-command source equality.
		if receipt.ReceiptRef != refs[i] || receipt.AgentRunID != assignment.BoundRunID || receipt.CapsuleID != assignment.Binding.CapsuleID ||
			"sha256:"+receipt.CapabilityHandleDigest != assignment.Binding.ExecutionHandleDigest ||
			objectgraph.SHA256([]byte(receipt.Command)) != report.Commands[i].CommandDigest || seen[receipt.ReceiptRef] {
			return report, fmt.Errorf("late assignment raw execution evidence does not authenticate exact receipt/run/handle/capsule/source")
		}
		seen[receipt.ReceiptRef] = true
		// Raw receipts deliberately are not granted/frozen/final-subject
		// certification. They remain exact evidence refs on the late report.
		report.ExecutorReceiptRefs = append(report.ExecutorReceiptRefs, receipt.ReceiptRef)
	}
	return report, nil
}

// resumeStrandedFrozenAssignmentCommit finishes the terminal saga for one
// assignment whose Complete reduce failed after the capsule froze: disposition
// is still bound, the capsule is frozen, and the staged pending proposal holds
// the report the worker authored. The worker cannot run another cell (a frozen
// capsule refuses every operation at acquireOp), so the reducer authors the
// fate from the stored proposal through the exact same report path a live
// worker would drive — same intent identity, idempotent on replay.
func (rt *Runtime) resumeStrandedFrozenAssignmentCommit(ctx context.Context, assignment types.EngineeringAssignment) error {
	proposal := assignment.PendingProposal
	if proposal == nil || strings.TrimSpace(assignment.BoundRunID) == "" {
		return fmt.Errorf("stranded assignment %s has no resumable proposal", assignment.AssignmentID)
	}
	rec := &types.RunRecord{
		RunID: assignment.BoundRunID, OwnerID: assignment.Binding.OwnerID, ComputerID: assignment.Binding.ComputerID,
		AgentID: assignment.Binding.AssignedAgentID, ChannelID: assignment.Binding.AssignedAgentID,
		Metadata: map[string]any{
			"assignment_id": assignment.AssignmentID, "assignment_attempt": fmt.Sprint(assignment.Binding.Attempt),
			"assigned_work_item_id": assignment.Binding.AssignedWorkItemID,
		},
	}
	_, err := rt.recordAssignedEngineeringReport(ctx, rec, "resume:"+assignment.AssignmentID+":"+fmt.Sprint(assignment.Binding.Attempt), proposal.Report)
	return err
}

func engineeringExecutionAttestationFromReceipt(assignment types.EngineeringAssignment, reportID string, command types.EngineeringRecordedCommand, receipt capsule.ExecutionReceipt) (types.EngineeringExecutionAttestation, error) {
	if receipt.AgentRunID != assignment.BoundRunID || receipt.CapsuleID != assignment.Binding.CapsuleID || objectgraph.SHA256([]byte(receipt.Command)) != command.CommandDigest ||
		"sha256:"+strings.TrimPrefix(receipt.SourceTreeDigest, "sha256:") != assignment.Binding.SubjectDigest || strings.TrimSpace(receipt.GrantedReceiptRef) == "" {
		return types.EngineeringExecutionAttestation{}, fmt.Errorf("assignment granted receipt scope is invalid")
	}
	occurredAt, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(receipt.OccurredAt))
	if err != nil {
		return types.EngineeringExecutionAttestation{}, fmt.Errorf("assignment command receipt occurred_at is invalid")
	}
	return types.EngineeringExecutionAttestation{
		GrantedReceiptRef: receipt.GrantedReceiptRef, CommandID: command.CommandID, CommandDigest: command.CommandDigest,
		ExitCode: receipt.ExitCode, StdoutDigest: "sha256:" + strings.TrimPrefix(receipt.StdoutDigest, "sha256:"), StderrDigest: "sha256:" + strings.TrimPrefix(receipt.StderrDigest, "sha256:"),
		SourceSubjectDigest: "sha256:" + strings.TrimPrefix(receipt.SourceTreeDigest, "sha256:"), FinalSubjectDigest: "sha256:" + strings.TrimPrefix(receipt.WorktreeDigest, "sha256:"), WorktreeDigest: "sha256:" + strings.TrimPrefix(receipt.WorktreeDigest, "sha256:"),
		Granted: true, Frozen: true, OccurredAt: occurredAt.UTC(), ReportID: reportID,
	}, nil
}

func (rt *Runtime) bindFrozenAssignmentExecutionReceipts(ctx context.Context, assignment types.EngineeringAssignment, handle string, report types.EngineeringAssignmentReport) (types.EngineeringAssignmentReport, error) {
	refs := make([]string, 0, len(report.Commands))
	for _, command := range report.Commands {
		refs = append(refs, command.ExecutionRef)
	}
	if len(refs) == 0 {
		return report, nil
	}
	receipts, err := rt.capsuleExecutor.ResolveGrantedExecutionReceipts(ctx, assignment.BoundRunID, handle, refs)
	if err != nil || len(receipts) != len(refs) {
		return report, fmt.Errorf("assignment command evidence unavailable after durable freeze: %w", err)
	}
	seen := map[string]bool{}
	for i, receipt := range receipts {
		if receipt.CapsuleID != assignment.Binding.CapsuleID || objectgraph.SHA256([]byte(receipt.Command)) != report.Commands[i].CommandDigest || strings.TrimSpace(receipt.GrantedReceiptRef) == "" || seen[receipt.GrantedReceiptRef] {
			return report, fmt.Errorf("assignment command evidence does not bind unique exact final subject")
		}
		seen[receipt.GrantedReceiptRef] = true
		if report.Commands[i].ExitCode != receipt.ExitCode {
			return report, fmt.Errorf("assignment command exit code changed after freeze: %d vs %d", report.Commands[i].ExitCode, receipt.ExitCode)
		}
		report.ExecutorReceiptRefs = append(report.ExecutorReceiptRefs, receipt.GrantedReceiptRef)
		if assignment.GrantPolicyAttestation != nil {
			attestation, buildErr := engineeringExecutionAttestationFromReceipt(assignment, report.ReportID, report.Commands[i], receipt)
			if buildErr != nil {
				return report, buildErr
			}
			report.ExecutionAttestations = append(report.ExecutionAttestations, attestation)
		}
	}
	return report, nil
}

func (rt *Runtime) commitAssignedEngineeringReport(ctx context.Context, assignment types.EngineeringAssignment, report types.EngineeringAssignmentReport) (types.EngineeringAssignmentCommandResult, error) {
	if report.Result == types.EngineeringResultPartial {
		refs := make([]string, 0, len(report.Commands))
		for _, command := range report.Commands {
			refs = append(refs, command.ExecutionRef)
		}
		if len(refs) > 0 {
			receipts, err := rt.capsuleExecutor.ResolveExecutionReceipts(refs)
			if err != nil || len(receipts) != len(refs) {
				return types.EngineeringAssignmentCommandResult{}, fmt.Errorf("partial assignment command evidence unavailable: %w", err)
			}
			for i, receipt := range receipts {
				if receipt.CapsuleID != assignment.Binding.CapsuleID || objectgraph.SHA256([]byte(receipt.Command)) != report.Commands[i].CommandDigest {
					return types.EngineeringAssignmentCommandResult{}, fmt.Errorf("partial assignment command evidence binding mismatch")
				}
			}
		}
	} else if len(report.Commands) != len(report.ExecutorReceiptRefs) {
		return types.EngineeringAssignmentCommandResult{}, fmt.Errorf("terminal assignment requires one durable granted executor receipt per command")
	}
	req := types.RecordEngineeringAssignmentReportRequest{
		CommandID: "co-super-report:" + assignment.AssignmentID + ":" + report.ReportID,
		OwnerID:   assignment.Binding.OwnerID, ComputerID: assignment.Binding.ComputerID,
		AssignmentID: assignment.AssignmentID, Attempt: assignment.Binding.Attempt,
		ExpectedLifecycleVersion: assignment.LifecycleVersion, Report: report,
		ExecutionAttestations: append([]types.EngineeringExecutionAttestation(nil), report.ExecutionAttestations...),
	}
	req.CommandDigest, _ = store.ComputeRecordEngineeringAssignmentReportDigest(req)
	return rt.store.RecordEngineeringAssignmentReport(ctx, req)
}
