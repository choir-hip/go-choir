package agentcore

import (
	"context"
	"encoding/json"
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

func terminalAssignedCoSuperReportCall(call types.ToolCall) bool {
	if call.Name != "record_assignment_result" || strings.TrimSpace(call.ID) == "" {
		return false
	}
	var input struct {
		Result types.CoSuperAssignmentResultKind `json:"result"`
	}
	if err := json.Unmarshal(call.Arguments, &input); err != nil {
		return false
	}
	return input.Result == types.CoSuperResultCompleted || input.Result == types.CoSuperResultFailed || input.Result == types.CoSuperResultBlocked
}

func coSuperFateRequest(assignment types.CoSuperAssignment, disposition types.CoSuperCapsuleDisposition, intentRef, ackRef string) types.SetCoSuperCapsuleDispositionRequest {
	req := types.SetCoSuperCapsuleDispositionRequest{
		CommandID: fmt.Sprintf("co-super-capsule:%s:%d:%s", assignment.AssignmentID, assignment.Binding.Attempt, disposition),
		OwnerID:   assignment.Binding.OwnerID, ComputerID: assignment.Binding.ComputerID,
		AssignmentID: assignment.AssignmentID, Attempt: assignment.Binding.Attempt,
		ExpectedLifecycleVersion: assignment.LifecycleVersion, Disposition: disposition,
		IntentRef: intentRef, AckRef: ackRef,
	}
	if assignment.GrantPolicyAttestation != nil || len(assignment.CapsuleFateHistory) > 0 {
		req.FateStep = &types.CoSuperCapsuleFateStep{}
	}
	req.CommandDigest, _ = store.ComputeSetCoSuperCapsuleDispositionDigest(req)
	return req
}

func coSuperFateAckRequest(assignment types.CoSuperAssignment, disposition types.CoSuperCapsuleDisposition, intentRef, ackRef, sourceDigest, finalDigest, occurredAt string, capsuleAbsent bool) (types.SetCoSuperCapsuleDispositionRequest, error) {
	req := coSuperFateRequest(assignment, disposition, intentRef, ackRef)
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
			return types.SetCoSuperCapsuleDispositionRequest{}, fmt.Errorf("assignment command receipt occurred_at is invalid")
		}
		req.FateStep.OccurredAt = parsed.UTC()
	}
	req.CommandDigest, _ = store.ComputeSetCoSuperCapsuleDispositionDigest(req)
	return req, nil
}

func (rt *Runtime) assignedCapsule() assignmentCapsuleRuntime {
	if rt == nil {
		return nil
	}
	if rt.assignmentRuntime != nil {
		return rt.assignmentRuntime
	}
	return rt.capsuleExecutor
}

func assignedCoSuperRun(rec *types.RunRecord) bool {
	return rec != nil &&
		agentprofile.Canonical(agentProfileForRun(rec)) == agentprofile.CoSuper &&
		metadataStringValue(rec.Metadata, "assignment_id") != ""
}

func (rt *Runtime) assignedCoSuperCapsuleUsable(assignment types.CoSuperAssignment) bool {
	exec := rt.assignedCapsule()
	if exec == nil || strings.TrimSpace(assignment.BoundRunID) == "" || strings.TrimSpace(assignment.Binding.CapsuleID) == "" {
		return false
	}
	handle, handleErr := exec.AssignmentHandle(assignment.BoundRunID, assignment.Binding.CapsuleID)
	diagnostics, inspectErr := exec.InspectCapsuleRaw(assignment.Binding.CapsuleID)
	return handleErr == nil && strings.TrimSpace(handle) != "" && inspectErr == nil &&
		diagnostics != nil && diagnostics.ID == assignment.Binding.CapsuleID && diagnostics.State == capsule.StateActive
}

func (rt *Runtime) revokeAssignedCapsule(ctx context.Context, assignment types.CoSuperAssignment, reason string) (types.CoSuperAssignment, error) {
	exec := rt.assignedCapsule()
	if exec == nil {
		return assignment, fmt.Errorf("assigned capsule executor unavailable")
	}
	intentRef := assignment.CapsuleIntentRef
	if assignment.CapsuleDisposition != types.CoSuperCapsuleRevokeRequested && assignment.CapsuleDisposition != types.CoSuperCapsuleRevoked {
		intentRef = "capsule-revoke-intent:" + objectgraph.SHA256([]byte(strings.Join([]string{
			assignment.AssignmentID, fmt.Sprint(assignment.Binding.Attempt), assignment.BoundRunID, assignment.Binding.CapsuleID, reason,
		}, "\x00")))
		requested, err := rt.store.SetCoSuperCapsuleDisposition(ctx, coSuperFateRequest(assignment, types.CoSuperCapsuleRevokeRequested, intentRef, ""))
		if err != nil {
			return assignment, err
		}
		assignment = requested.Assignment
	}
	if assignment.CapsuleDisposition == types.CoSuperCapsuleRevoked {
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
	fateAck, fateAckErr := coSuperFateAckRequest(assignment, types.CoSuperCapsuleRevoked, intentRef, ackRef, "", "", revocationReceipt.OccurredAt, revocationReceipt.CapsuleAbsent)
	if fateAckErr != nil {
		return assignment, fmt.Errorf("invalid revoke receipt occurred_at: %w", fateAckErr)
	}
	acked, err := rt.store.SetCoSuperCapsuleDisposition(ctx, fateAck)
	if err != nil {
		return assignment, err
	}
	return acked.Assignment, nil
}

func (rt *Runtime) cancelAssignedCoSuper(ctx context.Context, parent types.RunRecord, assignmentID string, attempt uint64, reason string) (types.CoSuperAssignmentCommandResult, error) {
	assignment, err := rt.store.GetCoSuperAssignment(ctx, parent.OwnerID, parent.ComputerID, strings.TrimSpace(assignmentID), attempt)
	if err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	if assignment.Binding.ParentRunID != parent.RunID || assignment.Binding.ParentAgentID != parent.AgentID ||
		parent.AgentID != persistentSuperAgentID(parent.OwnerID) {
		return types.CoSuperAssignmentCommandResult{}, fmt.Errorf("assignment cancellation requires exact persistent Super parent")
	}
	cancel := types.CancelCoSuperAssignmentRequest{
		CommandID: fmt.Sprintf("co-super-cancel:%s:%d", assignment.AssignmentID, assignment.Binding.Attempt),
		OwnerID:   assignment.Binding.OwnerID, ComputerID: assignment.Binding.ComputerID,
		AssignmentID: assignment.AssignmentID, Attempt: assignment.Binding.Attempt,
		ExpectedLifecycleVersion: assignment.LifecycleVersion, Reason: strings.TrimSpace(reason),
	}
	if cancel.Reason == "" {
		cancel.Reason = "persistent Super cancelled assignment"
	}
	cancel.CommandDigest, _ = store.ComputeCancelCoSuperAssignmentDigest(cancel)
	if assignment.Disposition.Terminal() {
		// Exact cancellation replay/conflict is receipt-authoritative even after
		// terminal projection; never bypass it with an in-memory early return.
		replayed, replayErr := rt.store.CancelCoSuperAssignment(ctx, cancel)
		if replayErr != nil {
			return types.CoSuperAssignmentCommandResult{}, replayErr
		}
		return replayed, nil
	}
	if assignment.BoundRunID != "" {
		assignment, err = rt.revokeAssignedCapsule(ctx, assignment, reason)
		if err != nil {
			return types.CoSuperAssignmentCommandResult{}, err
		}
	}
	cancel.ExpectedLifecycleVersion = assignment.LifecycleVersion
	cancel.CommandDigest, _ = store.ComputeCancelCoSuperAssignmentDigest(cancel)
	cancelled, err := rt.store.CancelCoSuperAssignment(ctx, cancel)
	if err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	return cancelled, nil
}

func (rt *Runtime) persistSystemCoSuperCancellation(ctx context.Context, assignment types.CoSuperAssignment, reason string) (types.CoSuperAssignmentCommandResult, error) {
	for attempt := 0; attempt < 4; attempt++ {
		current, err := rt.store.GetCoSuperAssignment(ctx, assignment.Binding.OwnerID, assignment.Binding.ComputerID, assignment.AssignmentID, assignment.Binding.Attempt)
		if err != nil {
			return types.CoSuperAssignmentCommandResult{}, err
		}
		if current.Disposition.Terminal() {
			return types.CoSuperAssignmentCommandResult{Assignment: current, Replay: true}, nil
		}
		if current.CapsuleDisposition != types.CoSuperCapsuleRevoked {
			return types.CoSuperAssignmentCommandResult{}, fmt.Errorf("system assignment cancellation requires durable revoke acknowledgement")
		}
		cancel := types.CancelCoSuperAssignmentRequest{CommandID: fmt.Sprintf("co-super-system-cancel:%s:%d", current.AssignmentID, current.Binding.Attempt),
			OwnerID: current.Binding.OwnerID, ComputerID: current.Binding.ComputerID, AssignmentID: current.AssignmentID, Attempt: current.Binding.Attempt,
			ExpectedLifecycleVersion: current.LifecycleVersion, Reason: strings.TrimSpace(reason)}
		if cancel.Reason == "" {
			cancel.Reason = "system cancelled assignment fate"
		}
		cancel.CommandDigest, _ = store.ComputeCancelCoSuperAssignmentDigest(cancel)
		result, cancelErr := rt.store.CancelCoSuperAssignment(ctx, cancel)
		if cancelErr == nil {
			if !result.Replay && result.Update != nil {
				rt.wakeUpdatedCoagent(ctx, *result.Update)
			}
			return result, nil
		}
		if !errors.Is(cancelErr, store.ErrCoSuperAssignmentInvalid) && !errors.Is(cancelErr, store.ErrConcurrentStateChange) {
			return types.CoSuperAssignmentCommandResult{}, cancelErr
		}
	}
	return types.CoSuperAssignmentCommandResult{}, store.ErrConcurrentStateChange
}

func (rt *Runtime) prepareCoSuperTrajectoryCancellation(ctx context.Context, ownerID, computerID, trajectoryID, reason string) ([]types.CoSuperAssignment, error) {
	if rt == nil || rt.store == nil || rt.assignedCapsule() == nil || strings.TrimSpace(computerID) == "" {
		return nil, nil
	}
	assignments, err := rt.store.ListCoSuperAssignments(ctx, ownerID, computerID, trajectoryID)
	if err != nil {
		return nil, err
	}
	prepared := make([]types.CoSuperAssignment, 0, len(assignments))
	for _, assignment := range assignments {
		// Terminal assignment outcome does not by itself prove executor fate;
		// cancellation closes every non-revoked capsule before trajectory fate.
		if assignment.CapsuleDisposition != types.CoSuperCapsuleRevoked {
			assignment, err = rt.revokeAssignedCapsule(ctx, assignment, reason)
			if err != nil {
				return prepared, err
			}
		}
		prepared = append(prepared, assignment)
	}
	return prepared, nil
}

func (rt *Runtime) finishCoSuperTrajectoryCancellation(ctx context.Context, ownerID, computerID, trajectoryID, reason string) error {
	if rt == nil || rt.store == nil || strings.TrimSpace(computerID) == "" {
		return nil
	}
	assignments, err := rt.store.ListCoSuperAssignments(ctx, ownerID, computerID, trajectoryID)
	if err != nil {
		return err
	}
	for _, assignment := range assignments {
		if assignment.Disposition.Terminal() {
			continue
		}
		if assignment.CapsuleDisposition != types.CoSuperCapsuleRevoked {
			return fmt.Errorf("assignment %s cancellation missing durable executor revoke acknowledgement", assignment.AssignmentID)
		}
		if _, err := rt.persistSystemCoSuperCancellation(ctx, assignment, reason); err != nil {
			return err
		}
	}
	return nil
}

func (rt *Runtime) cancelBoundCoSuperRun(ctx context.Context, rec types.RunRecord, reason string) (bool, error) {
	assignmentID := metadataStringValue(rec.Metadata, "assignment_id")
	attempt := uint64(metadataIntValue(rec.Metadata, "assignment_attempt"))
	if assignmentID == "" || attempt == 0 {
		return false, nil
	}
	assignment, err := rt.store.GetCoSuperAssignment(ctx, rec.OwnerID, rec.ComputerID, assignmentID, attempt)
	if err != nil {
		return true, err
	}
	if assignment.BoundRunID != rec.RunID || assignment.Binding.AssignedAgentID != rec.AgentID {
		return true, fmt.Errorf("cancel run assignment binding mismatch")
	}
	if assignment.Disposition.Terminal() {
		if assignment.CapsuleDisposition != types.CoSuperCapsuleRevoked {
			_, err = rt.revokeAssignedCapsule(ctx, assignment, reason)
		}
		return true, err
	}
	assignment, err = rt.revokeAssignedCapsule(ctx, assignment, reason)
	if err != nil {
		return true, err
	}
	_, err = rt.persistSystemCoSuperCancellation(ctx, assignment, reason)
	return true, err
}

// ReconcileCoSuperAssignmentsForTrajectory closes restart gaps without a
// poller. It is called from existing actor/runtime reconstruction for the exact
// lifecycle trajectory: an absent executor capsule is first recorded as a
// durable revoke intent, then acknowledged absent, then cancelled. No wake or
// attempt reopen follows.
func (rt *Runtime) ReconcileCoSuperAssignmentsForTrajectory(ctx context.Context, ownerID, computerID, trajectoryID string) error {
	if rt == nil || rt.store == nil || rt.assignedCapsule() == nil {
		return nil
	}
	exec := rt.assignedCapsule()
	assignments, err := rt.store.ListCoSuperAssignments(ctx, ownerID, computerID, trajectoryID)
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
				if finishErr := rt.finishCoSuperTrajectoryCancellation(ctx, ownerID, computerID, trajectoryID, intent.Reason); finishErr != nil {
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
			// fails the SetCoSuperCapsuleDisposition guard (BoundRunID empty
			// on a non-open assignment) and aborts the whole trajectory.
			if assignment.CapsuleDisposition != types.CoSuperCapsuleRevoked && assignment.CapsuleDisposition != types.CoSuperCapsuleUnbound {
				if _, fateErr := rt.revokeAssignedCapsule(ctx, assignment, "restart terminal assignment fate reconciliation"); fateErr != nil {
					return fateErr
				}
			}
			continue
		}
		if trajectory.Status != types.TrajectoryLive {
			if assignment.CapsuleDisposition != types.CoSuperCapsuleRevoked {
				assignment, err = rt.revokeAssignedCapsule(ctx, assignment, "restart terminal trajectory fate reconciliation")
				if err != nil {
					return err
				}
			}
			if _, cancelErr := rt.persistSystemCoSuperCancellation(ctx, assignment, "restart completed terminal trajectory assignment fate"); cancelErr != nil {
				return cancelErr
			}
			continue
		}
		if assignment.BoundRunID == "" {
			intent := "capsule-revoke-intent:" + objectgraph.SHA256([]byte(assignment.AssignmentID+"\x00restart-pre-bind"))
			if assignment.CapsuleDisposition == types.CoSuperCapsuleUnbound {
				requested, fateErr := rt.store.SetCoSuperCapsuleDisposition(ctx, coSuperFateRequest(assignment, types.CoSuperCapsuleRevokeRequested, intent, ""))
				if fateErr != nil {
					return fateErr
				}
				assignment = requested.Assignment
			}
			if assignment.CapsuleDisposition == types.CoSuperCapsuleRevokeRequested {
				if assignment.CapsuleIntentRef != intent {
					return store.ErrCoSuperAssignmentCommandConflict
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
				fateAck, fateAckErr := coSuperFateAckRequest(assignment, types.CoSuperCapsuleRevoked, intent, receipt.ReceiptRef, "", "", receipt.OccurredAt, receipt.CapsuleAbsent)
				if fateAckErr != nil {
					return fmt.Errorf("invalid revoke receipt occurred_at: %w", fateAckErr)
				}
				acked, fateErr := rt.store.SetCoSuperCapsuleDisposition(ctx, fateAck)
				if fateErr != nil {
					return fateErr
				}
				assignment = acked.Assignment
			}
			if assignment.CapsuleDisposition != types.CoSuperCapsuleRevoked {
				return fmt.Errorf("restart open assignment has ambiguous capsule fate %s", assignment.CapsuleDisposition)
			}
			cancel := types.CancelCoSuperAssignmentRequest{
				CommandID: fmt.Sprintf("co-super-restart-open-cancel:%s:%d", assignment.AssignmentID, assignment.Binding.Attempt),
				OwnerID:   ownerID, ComputerID: computerID, AssignmentID: assignment.AssignmentID, Attempt: assignment.Binding.Attempt,
				ExpectedLifecycleVersion: assignment.LifecycleVersion, Reason: "restart acknowledged absent pre-bind assignment capsule",
			}
			cancel.CommandDigest, _ = store.ComputeCancelCoSuperAssignmentDigest(cancel)
			cancelled, cancelErr := rt.store.CancelCoSuperAssignment(ctx, cancel)
			if cancelErr != nil {
				return cancelErr
			}
			if !cancelled.Replay && cancelled.Update != nil {
				rt.wakeUpdatedCoagent(ctx, *cancelled.Update)
			}
			continue
		}
		usable := rt.assignedCoSuperCapsuleUsable(assignment)
		if usable {
			continue
		}
		assignment, err = rt.revokeAssignedCapsule(ctx, assignment, "restart executor binding reconciliation")
		if err != nil {
			return err
		}
		cancel := types.CancelCoSuperAssignmentRequest{
			CommandID: fmt.Sprintf("co-super-restart-cancel:%s:%d", assignment.AssignmentID, assignment.Binding.Attempt),
			OwnerID:   ownerID, ComputerID: computerID, AssignmentID: assignment.AssignmentID, Attempt: assignment.Binding.Attempt,
			ExpectedLifecycleVersion: assignment.LifecycleVersion, Reason: "restart revoked absent assignment capsule",
		}
		cancel.CommandDigest, _ = store.ComputeCancelCoSuperAssignmentDigest(cancel)
		cancelled, cancelErr := rt.store.CancelCoSuperAssignment(ctx, cancel)
		if cancelErr != nil {
			return cancelErr
		}
		if !cancelled.Replay && cancelled.Update != nil {
			rt.wakeUpdatedCoagent(ctx, *cancelled.Update)
		}
	}
	return nil
}

// reconcileCoSuperAssignmentCapsulesAfterRestart closes restart gaps for every
// durable CoSuper assignment in the computer, independent of the run-state
// metadata index that assignments bound before it was introduced may lack. It
// delegates to the per-trajectory reconciler so revoke, cancel, and run
// terminalization share one authority path.
func (rt *Runtime) reconcileCoSuperAssignmentCapsulesAfterRestart(ctx context.Context) {
	if rt == nil || rt.store == nil || rt.assignedCapsule() == nil {
		return
	}
	computerID := strings.TrimSpace(rt.TextureComputerID())
	if computerID == "" {
		return
	}
	assignments, err := rt.store.ListCoSuperAssignmentsForComputer(ctx, computerID)
	if err != nil {
		log.Printf("runtime: boot CoSuper assignment capsule sweep: %v", err)
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
		if err := rt.ReconcileCoSuperAssignmentsForTrajectory(ctx, assignment.Binding.OwnerID, assignment.Binding.ComputerID, assignment.Binding.TrajectoryID); err != nil {
			log.Printf("runtime: boot CoSuper assignment capsule sweep trajectory %s: %v", assignment.Binding.TrajectoryID, err)
		}
	}
}

func (rt *Runtime) recordAssignedCoSuperReport(ctx context.Context, rec *types.RunRecord, toolCallID string, report types.CoSuperAssignmentReport) (types.CoSuperAssignmentCommandResult, error) {
	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		result, err := rt.recordAssignedCoSuperReportOnce(ctx, rec, toolCallID, report)
		if err == nil {
			return result, nil
		}
		lastErr = err
		if !errors.Is(err, store.ErrCoSuperAssignmentInvalid) && !errors.Is(err, store.ErrConcurrentStateChange) {
			return types.CoSuperAssignmentCommandResult{}, err
		}
		// A cancellation/report race may invalidate the live freeze CAS or
		// advance assignment fate after late authority was read. Reload the
		// exact immutable binding and retry only after cancellation/revocation
		// has won. The next attempt is forced through the evidence-only path.
		assignmentID := metadataStringValue(rec.Metadata, "assignment_id")
		assignmentAttempt := uint64(metadataIntValue(rec.Metadata, "assignment_attempt"))
		current, loadErr := rt.store.GetCoSuperAssignment(ctx, rec.OwnerID, rec.ComputerID, assignmentID, assignmentAttempt)
		if loadErr != nil {
			return types.CoSuperAssignmentCommandResult{}, loadErr
		}
		_, intentErr := rt.store.GetLifecycleCancellationIntent(ctx, rec.OwnerID, rec.ComputerID, current.Binding.TrajectoryID)
		cancellationWon := intentErr == nil
		if intentErr != nil && !errors.Is(intentErr, store.ErrNotFound) {
			return types.CoSuperAssignmentCommandResult{}, intentErr
		}
		if !cancellationWon && !current.Disposition.Terminal() && current.CapsuleDisposition != types.CoSuperCapsuleRevokeRequested && current.CapsuleDisposition != types.CoSuperCapsuleRevoked {
			return types.CoSuperAssignmentCommandResult{}, err
		}
	}
	return types.CoSuperAssignmentCommandResult{}, fmt.Errorf("retain assignment report after cancellation race: %w", lastErr)
}

func (rt *Runtime) recordAssignedCoSuperReportOnce(ctx context.Context, rec *types.RunRecord, toolCallID string, report types.CoSuperAssignmentReport) (types.CoSuperAssignmentCommandResult, error) {
	if rec == nil || rt.capsuleExecutor == nil || strings.TrimSpace(toolCallID) == "" {
		return types.CoSuperAssignmentCommandResult{}, fmt.Errorf("assigned CoSuper report authority unavailable")
	}
	assignmentID := metadataStringValue(rec.Metadata, "assignment_id")
	attempt := uint64(metadataIntValue(rec.Metadata, "assignment_attempt"))
	assignment, err := rt.store.GetCoSuperAssignment(ctx, rec.OwnerID, rec.ComputerID, assignmentID, attempt)
	if err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	if assignment.BoundRunID != rec.RunID || assignment.Binding.AssignedAgentID != rec.AgentID {
		return types.CoSuperAssignmentCommandResult{}, fmt.Errorf("report run is not the exact bound assignment")
	}
	report.ReportID = "report:" + objectgraph.SHA256([]byte(strings.Join([]string{
		"choir:co-super-report:v1", rec.OwnerID, rec.ComputerID, rec.RunID, assignmentID, fmt.Sprint(attempt), strings.TrimSpace(toolCallID),
	}, "\x00")))
	terminal := report.Result != types.CoSuperResultPartial
	cancellationIntended := false
	if _, intentErr := rt.store.GetLifecycleCancellationIntent(ctx, rec.OwnerID, rec.ComputerID, assignment.Binding.TrajectoryID); intentErr == nil {
		cancellationIntended = true
	} else if !errors.Is(intentErr, store.ErrNotFound) {
		return types.CoSuperAssignmentCommandResult{}, intentErr
	}
	lateFate := cancellationIntended || assignment.Disposition.Terminal() || assignment.CapsuleDisposition == types.CoSuperCapsuleRevokeRequested || assignment.CapsuleDisposition == types.CoSuperCapsuleRevoked
	// Cancellation wins: a racing verification result is evidence-only and can
	// never retain or derive Pass semantics.
	if lateFate && report.Verdict == types.CoSuperVerdictPass {
		report.Verdict = types.CoSuperVerdictAbstain
	}
	report.Mutations = nil
	report.ObservedSubjectDigest = assignment.Binding.SubjectDigest
	report.CertifiesOriginalSubject = false
	fingerprintParts := []string{string(report.Result), string(report.Verdict), report.ReportID, strings.TrimSpace(report.Summary)}
	fingerprintParts = append(fingerprintParts, report.EvidenceRefs...)
	for _, command := range report.Commands {
		fingerprintParts = append(fingerprintParts, command.ExecutionRef, command.CommandDigest)
	}
	terminalFingerprint := objectgraph.SHA256([]byte(strings.Join(fingerprintParts, "\x00")))

	storedReport, reportErr := rt.store.GetCoSuperAssignmentReport(ctx, rec.OwnerID, rec.ComputerID, report.ReportID)
	reportExists := reportErr == nil
	if reportErr != nil && !errors.Is(reportErr, store.ErrNotFound) {
		return types.CoSuperAssignmentCommandResult{}, reportErr
	}
	if reportExists {
		// The stored runtime-derived report is the only replay authority. A
		// changed result/verdict/evidence conflicts before any fate effect.
		storedParts := []string{string(storedReport.Result), string(storedReport.Verdict), storedReport.ReportID, strings.TrimSpace(storedReport.Summary)}
		storedParts = append(storedParts, storedReport.EvidenceRefs...)
		for _, command := range storedReport.Commands {
			storedParts = append(storedParts, command.ExecutionRef, command.CommandDigest)
		}
		if objectgraph.SHA256([]byte(strings.Join(storedParts, "\x00"))) != terminalFingerprint {
			return types.CoSuperAssignmentCommandResult{}, store.ErrCoSuperAssignmentCommandConflict
		}
		report = storedReport
	}
	if !terminal && reportExists {
		return rt.store.ReplayRecordedCoSuperAssignmentReport(ctx, assignment.Binding.OwnerID, assignment.Binding.ComputerID,
			assignment.AssignmentID, assignment.Binding.Attempt, report.ReportID, "co-super-report:"+assignment.AssignmentID+":"+report.ReportID)
	}

	// Cancellation/revocation wins the lifecycle race. A provider tool call
	// already in flight may still commit its authenticated report identity, but
	// Store derives it as late evidence only: no packet, wake, projection, or
	// capsule effect can reopen/revise the cancelled assignment.
	if lateFate {
		if reportExists {
			return rt.store.ReplayRecordedCoSuperAssignmentReport(ctx, assignment.Binding.OwnerID, assignment.Binding.ComputerID,
				assignment.AssignmentID, assignment.Binding.Attempt, report.ReportID, "co-super-report:"+assignment.AssignmentID+":"+report.ReportID)
		}
		report, err = rt.bindLateAssignmentExecutionReceipts(assignment, report)
		if err != nil {
			return types.CoSuperAssignmentCommandResult{}, err
		}
		return rt.commitAssignedCoSuperReport(ctx, assignment, report)
	}
	if !terminal {
		if assignment.CapsuleDisposition != types.CoSuperCapsuleActive || assignment.Disposition.Terminal() {
			return types.CoSuperAssignmentCommandResult{}, fmt.Errorf("partial report requires active nonterminal assignment")
		}
		return rt.commitAssignedCoSuperReport(ctx, assignment, report)
	}

	intent := "capsule-freeze-intent:" + terminalFingerprint
	switch assignment.CapsuleDisposition {
	case types.CoSuperCapsuleActive:
		if reportExists {
			return types.CoSuperAssignmentCommandResult{}, fmt.Errorf("stored terminal report cannot precede freeze intent")
		}
		requested, err := rt.store.SetCoSuperCapsuleDisposition(ctx, coSuperFateRequest(assignment, types.CoSuperCapsuleFreezeRequested, intent, ""))
		if err != nil {
			return types.CoSuperAssignmentCommandResult{}, err
		}
		assignment = requested.Assignment
	case types.CoSuperCapsuleFreezeRequested:
		if assignment.CapsuleIntentRef != intent {
			return types.CoSuperAssignmentCommandResult{}, store.ErrCoSuperAssignmentCommandConflict
		}
	case types.CoSuperCapsuleFrozen, types.CoSuperCapsuleRevokeRequested, types.CoSuperCapsuleRevoked:
		if assignment.CapsuleIntentRef != intent && !reportExists {
			return types.CoSuperAssignmentCommandResult{}, store.ErrCoSuperAssignmentCommandConflict
		}
	default:
		return types.CoSuperAssignmentCommandResult{}, fmt.Errorf("terminal report cannot resume capsule disposition %s", assignment.CapsuleDisposition)
	}

	if assignment.CapsuleDisposition == types.CoSuperCapsuleFreezeRequested {
		handle, err := rt.capsuleExecutor.AssignmentHandle(rec.RunID, assignment.Binding.CapsuleID)
		if err != nil {
			return types.CoSuperAssignmentCommandResult{}, fmt.Errorf("resolve exact assignment capability after freeze intent: %w", err)
		}
		_, err = rt.capsuleExecutor.ExtractGranted(ctx, rec.RunID, handle)
		if err != nil {
			return types.CoSuperAssignmentCommandResult{}, fmt.Errorf("freeze assignment after durable intent: %w", err)
		}
		diagnostics, err := rt.capsuleExecutor.InspectCapsuleRaw(assignment.Binding.CapsuleID)
		if err != nil || diagnostics.ID != assignment.Binding.CapsuleID || diagnostics.State != capsule.StateFrozen {
			return types.CoSuperAssignmentCommandResult{}, fmt.Errorf("executor did not acknowledge exact frozen assignment capsule: %w", err)
		}
		frozenDigest, err := rt.capsuleExecutor.ResolveGrantedWorktreeDigest(ctx, rec.RunID, handle)
		if err != nil || !types.ValidSHA256Digest(frozenDigest) {
			return types.CoSuperAssignmentCommandResult{}, fmt.Errorf("frozen assignment digest unavailable: %w", err)
		}
		freezeReceipt, receiptErr := rt.capsuleExecutor.PersistGrantedFreezeReceipt(ctx, rec.RunID, handle)
		if receiptErr != nil || freezeReceipt.CapsuleID != assignment.Binding.CapsuleID || "sha256:"+strings.TrimPrefix(freezeReceipt.FinalSubjectDigest, "sha256:") != frozenDigest || "sha256:"+strings.TrimPrefix(freezeReceipt.SourceSubjectDigest, "sha256:") != assignment.Binding.SubjectDigest {
			return types.CoSuperAssignmentCommandResult{}, fmt.Errorf("durable typed executor freeze receipt unavailable: %w", receiptErr)
		}
		ack := freezeReceipt.ReceiptRef
		fateAck, fateAckErr := coSuperFateAckRequest(assignment, types.CoSuperCapsuleFrozen, intent, ack, "sha256:"+strings.TrimPrefix(freezeReceipt.SourceSubjectDigest, "sha256:"), "sha256:"+strings.TrimPrefix(freezeReceipt.FinalSubjectDigest, "sha256:"), freezeReceipt.OccurredAt, false)
		if fateAckErr != nil {
			return types.CoSuperAssignmentCommandResult{}, fmt.Errorf("invalid freeze receipt occurred_at: %w", fateAckErr)
		}
		frozen, err := rt.store.SetCoSuperCapsuleDisposition(ctx, fateAck)
		if err != nil {
			return types.CoSuperAssignmentCommandResult{}, err
		}
		assignment = frozen.Assignment
		if frozenDigest != assignment.Binding.SubjectDigest {
			report.ObservedSubjectDigest = frozenDigest
			report.Mutations = []types.CoSuperRecordedMutation{{
				MutationID: "assignment-overlay:" + objectgraph.SHA256([]byte(ack)), Kind: "assignment_overlay",
				BeforeDigest: assignment.Binding.SubjectDigest, AfterDigest: frozenDigest,
				EvidenceRef: "capsule-diff:" + objectgraph.SHA256([]byte(ack)), SubjectBytesChanged: true,
			}}
		}
	}

	if assignment.CapsuleDisposition == types.CoSuperCapsuleFrozen && !reportExists {
		handle, resolveErr := rt.capsuleExecutor.AssignmentHandle(rec.RunID, assignment.Binding.CapsuleID)
		if resolveErr != nil {
			return types.CoSuperAssignmentCommandResult{}, fmt.Errorf("resolve frozen assignment capability: %w", resolveErr)
		}
		frozenDigest, digestErr := rt.capsuleExecutor.ResolveGrantedWorktreeDigest(ctx, rec.RunID, handle)
		if digestErr != nil || !types.ValidSHA256Digest(frozenDigest) {
			return types.CoSuperAssignmentCommandResult{}, fmt.Errorf("already-frozen assignment digest unavailable: %w", digestErr)
		}
		report, resolveErr = rt.bindFrozenAssignmentExecutionReceipts(ctx, assignment, handle, report)
		if resolveErr != nil {
			return types.CoSuperAssignmentCommandResult{}, resolveErr
		}
		if frozenDigest != assignment.Binding.SubjectDigest && len(report.Mutations) == 0 {
			report.ObservedSubjectDigest = frozenDigest
			report.Mutations = []types.CoSuperRecordedMutation{{MutationID: "assignment-subject:" + objectgraph.SHA256([]byte(assignment.CapsuleAckRef)), Kind: "workspace_platform_complete_tree", BeforeDigest: assignment.Binding.SubjectDigest, AfterDigest: frozenDigest, EvidenceRef: "capsule-diff:" + objectgraph.SHA256([]byte(assignment.CapsuleAckRef)), SubjectBytesChanged: true}}
		}
		if frozenDigest != assignment.Binding.SubjectDigest {
			candidate, candidateErr := rt.capsuleExecutor.PersistGrantedCandidate(ctx, rec.RunID, handle)
			if candidateErr != nil || "sha256:"+strings.TrimPrefix(candidate.SubjectDigest, "sha256:") != frozenDigest {
				return types.CoSuperAssignmentCommandResult{}, fmt.Errorf("persist reconstructable content-addressed candidate: %w", candidateErr)
			}
			report.CandidateArtifactRef = candidate.ArtifactRef
		}
	}

	var result types.CoSuperAssignmentCommandResult
	if reportExists {
		commandID := "co-super-report:" + assignment.AssignmentID + ":" + report.ReportID
		result, err = rt.store.ReplayRecordedCoSuperAssignmentReport(ctx, assignment.Binding.OwnerID, assignment.Binding.ComputerID, assignment.AssignmentID, assignment.Binding.Attempt, report.ReportID, commandID)
	} else {
		if assignment.CapsuleDisposition != types.CoSuperCapsuleFrozen {
			return types.CoSuperAssignmentCommandResult{}, fmt.Errorf("terminal report requires frozen executor acknowledgement")
		}
		result, err = rt.commitAssignedCoSuperReport(ctx, assignment, report)
	}
	if err != nil {
		return result, err
	}
	current := result.Assignment
	if current.CapsuleDisposition != types.CoSuperCapsuleRevoked {
		current, err = rt.revokeAssignedCapsule(ctx, current, "terminal assignment report recorded")
		if err != nil {
			return result, err
		}
	}
	result.Assignment = current
	return result, nil
}

func (rt *Runtime) bindLateAssignmentExecutionReceipts(assignment types.CoSuperAssignment, report types.CoSuperAssignmentReport) (types.CoSuperAssignmentReport, error) {
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
		if receipt.ReceiptRef != refs[i] || receipt.AgentRunID != assignment.BoundRunID || receipt.CapsuleID != assignment.Binding.CapsuleID ||
			"sha256:"+receipt.CapabilityHandleDigest != assignment.Binding.ExecutionHandleDigest ||
			"sha256:"+strings.TrimPrefix(receipt.SourceTreeDigest, "sha256:") != assignment.Binding.SubjectDigest ||
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

func coSuperExecutionAttestationFromReceipt(assignment types.CoSuperAssignment, reportID string, command types.CoSuperRecordedCommand, receipt capsule.ExecutionReceipt) (types.CoSuperExecutionAttestation, error) {
	if receipt.AgentRunID != assignment.BoundRunID || receipt.CapsuleID != assignment.Binding.CapsuleID || objectgraph.SHA256([]byte(receipt.Command)) != command.CommandDigest ||
		"sha256:"+strings.TrimPrefix(receipt.SourceTreeDigest, "sha256:") != assignment.Binding.SubjectDigest || strings.TrimSpace(receipt.GrantedReceiptRef) == "" {
		return types.CoSuperExecutionAttestation{}, fmt.Errorf("assignment granted receipt scope is invalid")
	}
	occurredAt, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(receipt.OccurredAt))
	if err != nil {
		return types.CoSuperExecutionAttestation{}, fmt.Errorf("assignment command receipt occurred_at is invalid")
	}
	return types.CoSuperExecutionAttestation{
		GrantedReceiptRef: receipt.GrantedReceiptRef, CommandID: command.CommandID, CommandDigest: command.CommandDigest,
		ExitCode: receipt.ExitCode, StdoutDigest: "sha256:" + strings.TrimPrefix(receipt.StdoutDigest, "sha256:"), StderrDigest: "sha256:" + strings.TrimPrefix(receipt.StderrDigest, "sha256:"),
		SourceSubjectDigest: "sha256:" + strings.TrimPrefix(receipt.SourceTreeDigest, "sha256:"), FinalSubjectDigest: "sha256:" + strings.TrimPrefix(receipt.WorktreeDigest, "sha256:"), WorktreeDigest: "sha256:" + strings.TrimPrefix(receipt.WorktreeDigest, "sha256:"),
		Granted: true, Frozen: true, OccurredAt: occurredAt.UTC(), ReportID: reportID,
	}, nil
}

func (rt *Runtime) bindFrozenAssignmentExecutionReceipts(ctx context.Context, assignment types.CoSuperAssignment, handle string, report types.CoSuperAssignmentReport) (types.CoSuperAssignmentReport, error) {
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
		report.Commands[i].ExitCode = receipt.ExitCode
		report.ExecutorReceiptRefs = append(report.ExecutorReceiptRefs, receipt.GrantedReceiptRef)
		if assignment.GrantPolicyAttestation != nil {
			attestation, buildErr := coSuperExecutionAttestationFromReceipt(assignment, report.ReportID, report.Commands[i], receipt)
			if buildErr != nil {
				return report, buildErr
			}
			report.ExecutionAttestations = append(report.ExecutionAttestations, attestation)
		}
	}
	return report, nil
}

func (rt *Runtime) commitAssignedCoSuperReport(ctx context.Context, assignment types.CoSuperAssignment, report types.CoSuperAssignmentReport) (types.CoSuperAssignmentCommandResult, error) {
	if report.Result == types.CoSuperResultPartial {
		refs := make([]string, 0, len(report.Commands))
		for _, command := range report.Commands {
			refs = append(refs, command.ExecutionRef)
		}
		if len(refs) > 0 {
			receipts, err := rt.capsuleExecutor.ResolveExecutionReceipts(refs)
			if err != nil || len(receipts) != len(refs) {
				return types.CoSuperAssignmentCommandResult{}, fmt.Errorf("partial assignment command evidence unavailable: %w", err)
			}
			for i, receipt := range receipts {
				if receipt.CapsuleID != assignment.Binding.CapsuleID || objectgraph.SHA256([]byte(receipt.Command)) != report.Commands[i].CommandDigest {
					return types.CoSuperAssignmentCommandResult{}, fmt.Errorf("partial assignment command evidence binding mismatch")
				}
			}
		}
	} else if len(report.Commands) != len(report.ExecutorReceiptRefs) {
		return types.CoSuperAssignmentCommandResult{}, fmt.Errorf("terminal assignment requires one durable granted executor receipt per command")
	}
	req := types.RecordCoSuperAssignmentReportRequest{
		CommandID: "co-super-report:" + assignment.AssignmentID + ":" + report.ReportID,
		OwnerID:   assignment.Binding.OwnerID, ComputerID: assignment.Binding.ComputerID,
		AssignmentID: assignment.AssignmentID, Attempt: assignment.Binding.Attempt,
		ExpectedLifecycleVersion: assignment.LifecycleVersion, Report: report,
		ExecutionAttestations: append([]types.CoSuperExecutionAttestation(nil), report.ExecutionAttestations...),
	}
	req.CommandDigest, _ = store.ComputeRecordCoSuperAssignmentReportDigest(req)
	return rt.store.RecordCoSuperAssignmentReport(ctx, req)
}
