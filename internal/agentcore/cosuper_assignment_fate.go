package agentcore

import (
	"context"
	"errors"
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

func (rt *Runtime) cancelAssignedCoSuper(ctx context.Context, parent types.RunRecord, assignmentID string, attempt uint64, reason string) (types.CoSuperAssignmentCommandResult, error) {
	assignment, err := rt.store.GetCoSuperAssignment(ctx, parent.OwnerID, parent.SandboxID, strings.TrimSpace(assignmentID), attempt)
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
				if rt.capsuleExecutor.HasCapsule(assignment.Binding.CapsuleID) {
					if destroyErr := rt.capsuleExecutor.ForceDestroy(ctx, assignment.Binding.CapsuleID); destroyErr != nil {
						return destroyErr
					}
				}
				if rt.capsuleExecutor.HasCapsule(assignment.Binding.CapsuleID) {
					return fmt.Errorf("restart pre-bind capsule remained after revoke effect")
				}
				ack := "capsule-revoke-ack:" + objectgraph.SHA256([]byte(intent+"\x00absent"))
				acked, fateErr := rt.store.SetCoSuperCapsuleDisposition(ctx, coSuperFateRequest(assignment, types.CoSuperCapsuleRevoked, intent, ack))
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

func (rt *Runtime) recordAssignedCoSuperReport(ctx context.Context, rec *types.RunRecord, toolCallID string, report types.CoSuperAssignmentReport) (types.CoSuperAssignmentCommandResult, error) {
	if rec == nil || rt.capsuleExecutor == nil || strings.TrimSpace(toolCallID) == "" {
		return types.CoSuperAssignmentCommandResult{}, fmt.Errorf("assigned CoSuper report authority unavailable")
	}
	assignmentID := metadataStringValue(rec.Metadata, "assignment_id")
	attempt := uint64(metadataIntValue(rec.Metadata, "assignment_attempt"))
	assignment, err := rt.store.GetCoSuperAssignment(ctx, rec.OwnerID, rec.SandboxID, assignmentID, attempt)
	if err != nil {
		return types.CoSuperAssignmentCommandResult{}, err
	}
	if assignment.BoundRunID != rec.RunID || assignment.Binding.AssignedAgentID != rec.AgentID {
		return types.CoSuperAssignmentCommandResult{}, fmt.Errorf("report run is not the exact bound assignment")
	}
	report.ReportID = "report:" + objectgraph.SHA256([]byte(strings.Join([]string{
		"choir:co-super-report:v1", rec.OwnerID, rec.SandboxID, rec.RunID, assignmentID, fmt.Sprint(attempt), strings.TrimSpace(toolCallID),
	}, "\x00")))
	terminal := report.Result != types.CoSuperResultPartial
	report.Mutations = nil
	report.ObservedSubjectDigest = assignment.Binding.SubjectDigest
	report.CertifiesOriginalSubject = false
	fingerprintParts := []string{string(report.Result), string(report.Verdict), report.ReportID, strings.TrimSpace(report.Summary)}
	fingerprintParts = append(fingerprintParts, report.EvidenceRefs...)
	for _, command := range report.Commands {
		fingerprintParts = append(fingerprintParts, command.ExecutionRef, command.CommandDigest)
	}
	terminalFingerprint := objectgraph.SHA256([]byte(strings.Join(fingerprintParts, "\x00")))

	storedReport, reportErr := rt.store.GetCoSuperAssignmentReport(ctx, rec.OwnerID, rec.SandboxID, report.ReportID)
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
	if assignment.Disposition.Terminal() || assignment.CapsuleDisposition == types.CoSuperCapsuleRevokeRequested || assignment.CapsuleDisposition == types.CoSuperCapsuleRevoked {
		if reportExists {
			return rt.store.ReplayRecordedCoSuperAssignmentReport(ctx, assignment.Binding.OwnerID, assignment.Binding.ComputerID,
				assignment.AssignmentID, assignment.Binding.Attempt, report.ReportID, "co-super-report:"+assignment.AssignmentID+":"+report.ReportID)
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
		frozen, err := rt.store.SetCoSuperCapsuleDisposition(ctx, coSuperFateRequest(assignment, types.CoSuperCapsuleFrozen, intent, ack))
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
		report.ExecutorReceiptRefs = append(report.ExecutorReceiptRefs, receipt.GrantedReceiptRef)
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
	}
	req.CommandDigest, _ = store.ComputeRecordCoSuperAssignmentReportDigest(req)
	return rt.store.RecordCoSuperAssignmentReport(ctx, req)
}
