package agentcore

import (
	"context"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func coSuperFateRequest(assignment types.CoSuperAssignment, disposition types.CoSuperCapsuleDisposition, intentRef, ackRef string) types.SetCoSuperCapsuleDispositionRequest {
	req := types.SetCoSuperCapsuleDispositionRequest{
		CommandID: fmt.Sprintf("co-super-capsule:%s:%d:%s", assignment.AssignmentID, assignment.Binding.Attempt, disposition),
		OwnerID:   assignment.Binding.OwnerID, ComputerID: assignment.Binding.ComputerID,
		AssignmentID: assignment.AssignmentID, Attempt: assignment.Binding.Attempt,
		ExpectedLifecycleVersion: assignment.LifecycleVersion, Disposition: disposition,
		IntentRef: intentRef, AckRef: ackRef,
	}
	req.CommandDigest, _ = store.ComputeSetCoSuperCapsuleDispositionDigest(req)
	return req
}

func (rt *Runtime) revokeAssignedCapsule(ctx context.Context, assignment types.CoSuperAssignment, reason string) (types.CoSuperAssignment, error) {
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
	if handle, err := rt.capsuleExecutor.AssignmentHandle(assignment.BoundRunID, assignment.Binding.CapsuleID); err == nil {
		if err := rt.capsuleExecutor.RevokeCapability(assignment.BoundRunID, handle); err != nil {
			return assignment, fmt.Errorf("revoke assignment capability: %w", err)
		}
	}
	if rt.capsuleExecutor.HasCapsule(assignment.Binding.CapsuleID) {
		if err := rt.capsuleExecutor.ForceDestroy(ctx, assignment.Binding.CapsuleID); err != nil {
			return assignment, fmt.Errorf("destroy assignment capsule after durable revoke intent: %w", err)
		}
	}
	if rt.capsuleExecutor.HasCapsule(assignment.Binding.CapsuleID) {
		return assignment, fmt.Errorf("assignment capsule continued after executor acknowledgement")
	}
	ackRef := "capsule-revoke-ack:" + objectgraph.SHA256([]byte(intentRef+"\x00absent"))
	acked, err := rt.store.SetCoSuperCapsuleDisposition(ctx, coSuperFateRequest(assignment, types.CoSuperCapsuleRevoked, intentRef, ackRef))
	if err != nil {
		return assignment, err
	}
	return acked.Assignment, nil
}

func (rt *Runtime) cancelAssignedCoSuper(ctx context.Context, parent types.RunRecord, assignmentID string, attempt uint64, reason string) (types.CoSuperAssignment, error) {
	assignment, err := rt.store.GetCoSuperAssignment(ctx, parent.OwnerID, parent.SandboxID, strings.TrimSpace(assignmentID), attempt)
	if err != nil {
		return types.CoSuperAssignment{}, err
	}
	if assignment.Binding.ParentRunID != parent.RunID || assignment.Binding.ParentAgentID != parent.AgentID ||
		parent.AgentID != persistentSuperAgentID(parent.OwnerID) {
		return types.CoSuperAssignment{}, fmt.Errorf("assignment cancellation requires exact persistent Super parent")
	}
	if assignment.Disposition.Terminal() {
		return assignment, nil
	}
	if assignment.BoundRunID != "" {
		assignment, err = rt.revokeAssignedCapsule(ctx, assignment, reason)
		if err != nil {
			return assignment, err
		}
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
	cancelled, err := rt.store.CancelCoSuperAssignment(ctx, cancel)
	if err != nil {
		return assignment, err
	}
	if assignment.BoundRunID != "" {
		_ = rt.terminalizeRun(context.Background(), assignment.BoundRunID, assignment.Binding.OwnerID, cancel.Reason)
	}
	return cancelled.Assignment, nil
}

// ReconcileCoSuperAssignmentsForTrajectory closes restart gaps without a
// poller. It is called from existing actor/runtime reconstruction for the exact
// lifecycle trajectory: an absent executor capsule is first recorded as a
// durable revoke intent, then acknowledged absent, then cancelled. No wake or
// attempt reopen follows.
func (rt *Runtime) ReconcileCoSuperAssignmentsForTrajectory(ctx context.Context, ownerID, computerID, trajectoryID string) error {
	if rt == nil || rt.store == nil || rt.capsuleExecutor == nil {
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
		if assignment.BoundRunID == "" {
			cancel := types.CancelCoSuperAssignmentRequest{
				CommandID: fmt.Sprintf("co-super-restart-open-cancel:%s:%d", assignment.AssignmentID, assignment.Binding.Attempt),
				OwnerID:   ownerID, ComputerID: computerID, AssignmentID: assignment.AssignmentID, Attempt: assignment.Binding.Attempt,
				ExpectedLifecycleVersion: assignment.LifecycleVersion, Reason: "restart found durable assignment open without executor bind",
			}
			cancel.CommandDigest, _ = store.ComputeCancelCoSuperAssignmentDigest(cancel)
			if _, err := rt.store.CancelCoSuperAssignment(ctx, cancel); err != nil {
				return err
			}
			continue
		}
		handle, handleErr := rt.capsuleExecutor.AssignmentHandle(assignment.BoundRunID, assignment.Binding.CapsuleID)
		diagnostics, inspectErr := rt.capsuleExecutor.InspectCapsuleRaw(assignment.Binding.CapsuleID)
		usable := handleErr == nil && strings.TrimSpace(handle) != "" && inspectErr == nil &&
			diagnostics.ID == assignment.Binding.CapsuleID && diagnostics.State == capsule.StateActive
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
		if _, err := rt.store.CancelCoSuperAssignment(ctx, cancel); err != nil {
			return err
		}
	}
	return nil
}

func (rt *Runtime) recordAssignedCoSuperReport(ctx context.Context, rec *types.RunRecord, report types.CoSuperAssignmentReport) (types.CoSuperAssignmentCommandResult, error) {
	if rec == nil || rt.capsuleExecutor == nil {
		return types.CoSuperAssignmentCommandResult{}, fmt.Errorf("assigned CoSuper report authority unavailable")
	}
	assignmentID := metadataStringValue(rec.Metadata, "assignment_id")
	attempt := uint64(metadataIntValue(rec.Metadata, "assignment_attempt"))
	assignment, err := rt.store.GetCoSuperAssignment(ctx, rec.OwnerID, rec.SandboxID, assignmentID, attempt)
	if err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	if assignment.BoundRunID != rec.RunID || assignment.Binding.AssignedAgentID != rec.AgentID || assignment.CapsuleDisposition != types.CoSuperCapsuleActive {
		return types.CoSuperAssignmentCommandResult{}, fmt.Errorf("report run is not the exact active bound assignment")
	}
	terminal := report.Result != types.CoSuperResultPartial
	report.Mutations = nil // never accept model-authored filesystem evidence
	report.ObservedSubjectDigest = assignment.Binding.SubjectDigest
	if !terminal {
		// A partial report is typed intermediate evidence only. It cannot freeze,
		// certify, mint a candidate, or narrate unobserved mutations.
		report.CertifiesOriginalSubject = false
	} else {
		intent := "capsule-freeze-intent:" + objectgraph.SHA256([]byte(assignment.AssignmentID+"\x00"+report.ReportID))
		requested, err := rt.store.SetCoSuperCapsuleDisposition(ctx, coSuperFateRequest(assignment, types.CoSuperCapsuleFreezeRequested, intent, ""))
		if err != nil {
			return types.CoSuperAssignmentCommandResult{}, err
		}
		assignment = requested.Assignment
		handle, err := rt.capsuleExecutor.AssignmentHandle(rec.RunID, assignment.Binding.CapsuleID)
		if err != nil {
			return types.CoSuperAssignmentCommandResult{}, fmt.Errorf("resolve exact assignment capability after freeze intent: %w", err)
		}
		changes, err := rt.capsuleExecutor.ExtractGranted(ctx, rec.RunID, handle)
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
		ack := "capsule-freeze-ack:" + objectgraph.SHA256([]byte(strings.Join([]string{intent, assignment.Binding.CapsuleID, frozenDigest, fmt.Sprint(len(changes))}, "\x00")))
		frozen, err := rt.store.SetCoSuperCapsuleDisposition(ctx, coSuperFateRequest(assignment, types.CoSuperCapsuleFrozen, intent, ack))
		if err != nil {
			return types.CoSuperAssignmentCommandResult{}, err
		}
		assignment = frozen.Assignment
		if len(changes) > 0 {
			report.ObservedSubjectDigest = frozenDigest
			report.Mutations = []types.CoSuperRecordedMutation{{
				MutationID: "assignment-overlay:" + objectgraph.SHA256([]byte(ack)), Kind: "assignment_overlay",
				BeforeDigest: assignment.Binding.SubjectDigest, AfterDigest: frozenDigest,
				EvidenceRef: "capsule-diff:" + objectgraph.SHA256([]byte(ack)), SubjectBytesChanged: true,
			}}
		}
	}
	refs := make([]string, 0, len(report.Commands))
	for _, command := range report.Commands {
		refs = append(refs, command.ExecutionRef)
	}
	if len(refs) > 0 {
		receipts, err := rt.capsuleExecutor.ResolveExecutionReceipts(refs)
		if err != nil || len(receipts) != len(refs) {
			return types.CoSuperAssignmentCommandResult{}, fmt.Errorf("assignment command evidence unavailable: %w", err)
		}
		for i, receipt := range receipts {
			if receipt.CapsuleID != assignment.Binding.CapsuleID || objectgraph.SHA256([]byte(receipt.Command)) != report.Commands[i].CommandDigest {
				return types.CoSuperAssignmentCommandResult{}, fmt.Errorf("assignment command evidence binding mismatch")
			}
		}
	}
	req := types.RecordCoSuperAssignmentReportRequest{
		CommandID: "co-super-report:" + assignmentID + ":" + strings.TrimSpace(report.ReportID),
		OwnerID:   rec.OwnerID, ComputerID: rec.SandboxID, AssignmentID: assignmentID, Attempt: attempt,
		ExpectedLifecycleVersion: assignment.LifecycleVersion, Report: report,
	}
	req.CommandDigest, _ = store.ComputeRecordCoSuperAssignmentReportDigest(req)
	result, err := rt.store.RecordCoSuperAssignmentReport(ctx, req)
	if err != nil {
		return result, err
	}
	if terminal {
		if _, err := rt.revokeAssignedCapsule(ctx, result.Assignment, "terminal assignment report recorded"); err != nil {
			return result, err
		}
	}
	return result, nil
}
