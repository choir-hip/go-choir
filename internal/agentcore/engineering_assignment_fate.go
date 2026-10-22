package agentcore

import (
	"context"
	"errors"
	"fmt"
	"log"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func engineeringFateRequest(assignment types.EngineeringAssignment, disposition types.EngineeringCapsuleDisposition, intentRef, ackRef string) types.SetEngineeringCapsuleDispositionRequest {
	req := types.SetEngineeringCapsuleDispositionRequest{
		CommandID: fmt.Sprintf("engineering-capsule:%s:%d:%s", assignment.AssignmentID, assignment.Binding.Attempt, disposition),
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

func (rt *Runtime) assignedEngineeringCapsuleUsable(ctx context.Context, assignment types.EngineeringAssignment) bool {
	exec := rt.assignedCapsule()
	if exec == nil || strings.TrimSpace(assignment.BoundRunID) == "" || strings.TrimSpace(assignment.Binding.CapsuleID) == "" {
		return false
	}
	// A bound run that is no longer executing (completed/failed/cancelled
	// without reporting, or parked blocked by budget/rate-limit) cannot drive
	// its capsule to a terminal fate. Capsule liveness alone is not proof of
	// usability — the run is the driver. Receipt:
	// docs/problems/engineering-run-death-leaves-bound-assignment-2026-09-29.md
	// RunBlocked is dead for assignment purposes: blocked runs are excluded
	// from residency and replaced, never resumed, so the capsule can never
	// reach terminal fate through it. Terminal or missing rows are dead.
	// Pending bound runs are mid-saga (reconcile holds the open mutex), so
	// they stay usable — but only within the activation grace: a pending run
	// older than the grace means bind committed but the activate wake never
	// landed (crash between Bind and rt.activate, or a lost dispatch), so the
	// driver is dead and the capsule must not read usable forever.
	if rt.store != nil {
		run, runErr := rt.store.GetLifecycleRun(ctx, assignment.Binding.OwnerID, assignment.Binding.ComputerID, assignment.BoundRunID)
		// C1e D7: a passivated run is parked, not dead. It keeps its
		// capsule and round on a live runtime and on a planned boot (where it
		// takes the planned restart's one resume); a crash boot terminalizes
		// it before this check is reached.
		if runErr != nil || (run.State != types.RunPending && run.State != types.RunRunning && run.State != types.RunPassivated) {
			return false
		}
		if run.State == types.RunPending && !run.CreatedAt.IsZero() &&
			time.Since(run.CreatedAt) > engineeringActivationGrace {
			return false
		}
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
		if err := traceErr(rt, assignment.BoundRunID, "revoke.capability", func() error { return exec.RevokeCapability(assignment.BoundRunID, handle) }); err != nil {
			return assignment, fmt.Errorf("revoke assignment capability: %w", err)
		}
	}
	if exec.HasCapsule(assignment.Binding.CapsuleID) {
		if err := traceErr(rt, assignment.BoundRunID, "revoke.destroy_capsule", func() error { return exec.ForceDestroy(ctx, assignment.Binding.CapsuleID) }); err != nil {
			return assignment, fmt.Errorf("destroy assignment capsule after durable revoke intent: %w", err)
		}
	}
	if exec.HasCapsule(assignment.Binding.CapsuleID) {
		return assignment, fmt.Errorf("assignment capsule continued after executor acknowledgement")
	}
	if err := traceErr(rt, assignment.BoundRunID, "revoke.cleanup_orphan", func() error { return exec.CleanupOrphanedCapsule(ctx, assignment.Binding.CapsuleID) }); err != nil {
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
	acked, err := traceCall(rt, assignment.BoundRunID, "revoke.ack_store", func() (types.EngineeringAssignmentCommandResult, error) {
		return rt.store.SetEngineeringCapsuleDisposition(ctx, fateAck)
	})
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
	// C1e D7: cancellation authority is the management desk that is the
	// engineering conversation's caster, not the exact casting run. Under D5
	// the canceller is a fresh management turn, never the run that cast.
	if assignment.Binding.ParentAgentID != parent.AgentID || parent.AgentID != persistentManagementAgentID(parent.OwnerID) {
		return types.EngineeringAssignmentCommandResult{}, fmt.Errorf("assignment cancellation requires the casting management desk")
	}
	cancel := types.CancelEngineeringAssignmentRequest{
		CommandID: fmt.Sprintf("engineering-cancel:%s:%d", assignment.AssignmentID, assignment.Binding.Attempt),
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
	if !cancelled.Replay {
		rt.failBoundSelfdevOperation(ctx, assignment.Binding.ComputerID, assignment.AssignmentID, cancel.Reason)
		rt.stopCancelledAssignmentRun(ctx, cancelled.Assignment, cancel.Reason)
		rt.settleEngineeringPrecommitsAtTerminal(ctx, cancelled.Assignment, cancel.Reason)
		rt.wakeNextQueuedEngineeringRequest(ctx, cancelled.Assignment)
	}
	return cancelled, nil
}

// stopCancelledAssignmentRun ends the bound engineering run of an assignment
// whose row a cancel just turned terminal (C1e D7). The store cancel
// projects the run's durable state terminal in the same transaction
// (projectEngineeringTerminalRun) but cannot reach the live loop, so the
// loop's in-memory cancel follows here. A run the projection did not reach
// is cancelled through the runtime's run cancel. Either way the run then
// ends with its row already terminal, and the run-end branch finds nothing
// to settle.
func (rt *Runtime) stopCancelledAssignmentRun(ctx context.Context, assignment types.EngineeringAssignment, reason string) {
	runID := strings.TrimSpace(assignment.BoundRunID)
	if rt == nil || rt.store == nil || runID == "" {
		return
	}
	run, err := rt.store.GetLifecycleRun(ctx, assignment.Binding.OwnerID, assignment.Binding.ComputerID, runID)
	if err == nil && !run.State.Terminal() {
		if err := rt.terminalizeRun(context.WithoutCancel(ctx), runID, assignment.Binding.OwnerID, reason); err != nil {
			log.Printf("runtime: stop cancelled assignment %s run %s: %v", assignment.AssignmentID, runID, err)
		}
		return
	}
	rt.runningMu.Lock()
	cancel := rt.running[runID]
	delete(rt.running, runID)
	rt.runningMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// persistSystemEngineeringCancellation turns an assignment whose capsule is
// revoked terminal. Unless silent, a records assignment's cancellation mints
// its final Report into the engineering round while that round is open (D3);
// a crash boot's cancellation is silent (D7), so nothing wakes.
func (rt *Runtime) persistSystemEngineeringCancellation(ctx context.Context, assignment types.EngineeringAssignment, reason string, silent bool) (types.EngineeringAssignmentCommandResult, error) {
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
		cancel := types.CancelEngineeringAssignmentRequest{CommandID: fmt.Sprintf("engineering-system-cancel:%s:%d", current.AssignmentID, current.Binding.Attempt),
			OwnerID: current.Binding.OwnerID, ComputerID: current.Binding.ComputerID, AssignmentID: current.AssignmentID, Attempt: current.Binding.Attempt,
			ExpectedLifecycleVersion: current.LifecycleVersion, Reason: strings.TrimSpace(reason), Silent: silent}
		if cancel.Reason == "" {
			cancel.Reason = "system cancelled assignment fate"
		}
		cancel.CommandDigest, _ = store.ComputeCancelEngineeringAssignmentDigest(cancel)
		result, cancelErr := rt.store.CancelEngineeringAssignment(ctx, cancel)
		if cancelErr == nil {
			if !result.Replay {
				// Every assignment-level cancellation funnels through this
				// choke point; the assignment's own self-development op
				// (resolved by its freeze key, D2) fails here. Best-effort:
				// a terminal or missing op converges.
				rt.failBoundSelfdevOperation(ctx, current.Binding.ComputerID, current.AssignmentID, cancel.Reason)
				rt.settleEngineeringPrecommitsAtTerminal(ctx, result.Assignment, cancel.Reason)
				if !silent {
					rt.wakeNextQueuedEngineeringRequest(ctx, result.Assignment)
				}
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
		// A terminal assignment still at capsule `unbound` never held one (a
		// capsule never returns to unbound, and Bound requires a capsule), so
		// there is nothing to revoke; requesting it refused the whole
		// trajectory cancel forever (docs/problems/texture-zombie-activations-
		// revising-forever-2026-10-09.md).
		if assignment.Disposition.Terminal() && assignment.CapsuleDisposition == types.EngineeringCapsuleUnbound {
			prepared = append(prepared, assignment)
			continue
		}
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
		if _, err := rt.persistSystemEngineeringCancellation(ctx, assignment, reason, false); err != nil {
			return err
		}
	}
	// A cancelled trajectory's self-development operations cannot reach
	// acceptance; this is the one path every trajectory cancellation funnels
	// through, so the trajectory-wide close lives here.
	rt.failTrajectorySelfdevOperations(ctx, computerID, trajectoryID, reason)
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
	_, err = rt.persistSystemEngineeringCancellation(ctx, assignment, reason, false)
	// persistSystemEngineeringCancellation closes the bound self-development
	// op at the choke point; a second fail call here would be duplicate.
	return true, err
}

// failBoundSelfdevOperation transitions an assignment's self-development
// operation (resolved by its freeze key, C1e D2) to `failed` when the
// assignment ends without acceptance. It loops over the lawful pre-terminal
// states because the run can die anywhere in
// executing->frozen->verified->awaiting_approval; an already-terminal op or
// an assignment that never froze is a no-op.
func (rt *Runtime) failBoundSelfdevOperation(ctx context.Context, computerID, assignmentID, reason string) {
	if rt == nil || rt.selfdevOperations == nil || strings.TrimSpace(computerID) == "" || strings.TrimSpace(assignmentID) == "" {
		return
	}
	operation, found, err := rt.selfdevOperations.GetByIdempotency(ctx, computerID, store.EngineeringFreezeIdempotencyKey(assignmentID))
	if err != nil {
		log.Printf("selfdev: fail the operation of assignment %s: read: %v", assignmentID, err)
		return
	}
	if !found {
		return
	}
	rt.failSelfdevOperation(ctx, operation, reason)
}

// failTrajectorySelfdevOperations fails every non-terminal operation on a
// trajectory that ended (cancelled or settled without acceptance). It is a
// trajectory-wide caller in D2's sense: it lists and acts on each.
func (rt *Runtime) failTrajectorySelfdevOperations(ctx context.Context, computerID, trajectoryID, reason string) {
	if rt == nil || rt.selfdevOperations == nil || strings.TrimSpace(computerID) == "" || strings.TrimSpace(trajectoryID) == "" {
		return
	}
	operations, err := rt.selfdevOperations.ListByTrajectory(ctx, computerID, trajectoryID)
	if err != nil {
		return
	}
	for _, operation := range operations {
		rt.failSelfdevOperation(ctx, operation, reason)
	}
}

// failSelfdevOperation closes one operation from whichever lawful
// pre-terminal state it is in now. It re-reads and retries a lost race,
// in-process (ErrConflict) or on the tape (ErrProjectionMismatch: a writer
// without this Store's write mutex, as in the rematerialize rebind window),
// with the same bound as closeEndedAssignmentSelfdevOperation, and logs a
// give-up. It is the one seam every cancellation close passes, so it also
// carries the close's deadline: a wedged tape append must not hold the
// Store's write mutex (and, on the reconcile paths,
// engineeringAssignmentOpenMu) forever.
func (rt *Runtime) failSelfdevOperation(ctx context.Context, operation selfdev.Operation, reason string) {
	ctx, cancel := selfdevCloseContext(ctx)
	defer cancel()
	trimmedReason := strings.TrimSpace(reason)
	closable := []string{selfdev.StateExecuting, selfdev.StateFrozen, selfdev.StateVerified, selfdev.StateAwaitingApproval, selfdev.StateRequested}
	for attempt := 0; attempt < selfdevCloseAttempts; attempt++ {
		current, err := rt.selfdevOperations.Get(ctx, operation.ComputerID, operation.OperationID)
		if err != nil {
			log.Printf("selfdev: fail %s: read: %v", operation.OperationID, err)
			return
		}
		if !slices.Contains(closable, current.State) {
			return
		}
		_, err = rt.selfdevOperations.Transition(ctx, current.ComputerID, current.OperationID, current.State, selfdev.StateFailed, func(next *selfdev.Operation) error {
			next.TerminalError = trimmedReason
			return nil
		})
		if err == nil {
			return
		}
		if !selfdevCloseRetryable(err) {
			log.Printf("selfdev: fail %s from %s: %v", current.OperationID, current.State, err)
			return
		}
		log.Printf("selfdev: fail %s from %s: state moved, re-reading: %v", current.OperationID, current.State, err)
	}
	log.Printf("selfdev: fail %s: gave up after repeated state conflicts", operation.OperationID)
}

// selfdevCloseAttempts bounds the re-reads of a close that keeps losing races.
const selfdevCloseAttempts = 3

// selfdevCloseTimeout bounds a post-commit close detached from its request,
// as settleRulePrecommits bounds settlement.
const selfdevCloseTimeout = 15 * time.Second

// selfdevCloseContext is a close's context: detached from the request's
// cancellation (the close runs after a commit) with its own deadline.
func selfdevCloseContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), selfdevCloseTimeout)
}

// selfdevCloseRetryable reports a lost race: in-process (ErrConflict) or a
// tape CAS lost to a writer without this Store's write mutex
// (ErrProjectionMismatch).
func selfdevCloseRetryable(err error) bool {
	return errors.Is(err, selfdev.ErrConflict) || errors.Is(err, computerevent.ErrProjectionMismatch)
}

// ReconcileEngineeringAssignmentsForTrajectory closes restart gaps without a
// poller. It is called from existing actor/runtime reconstruction for the exact
// lifecycle trajectory: an absent executor capsule is first recorded as a
// durable revoke intent, then acknowledged absent, then cancelled. No wake or
// attempt reopen follows.
func (rt *Runtime) ReconcileEngineeringAssignmentsForTrajectory(ctx context.Context, ownerID, computerID, trajectoryID string) error {
	return rt.reconcileEngineeringAssignmentsForTrajectory(ctx, ownerID, computerID, trajectoryID, false)
}

// engineeringInterruptedByRestart is the cancellation reason a crash boot
// gives every open assignment (C1e D7).
const engineeringInterruptedByRestart = "interrupted_by_restart"

// reconcileEngineeringAssignmentsForTrajectory is the reconcile body. On a
// crash boot (C1e D7) every open assignment turns terminal
// `interrupted_by_restart`, silently: its round was already closed by the
// boot's crash closure, nothing resumes, and the pending-proposal resume is
// skipped.
func (rt *Runtime) reconcileEngineeringAssignmentsForTrajectory(ctx context.Context, ownerID, computerID, trajectoryID string, crashBoot bool) error {
	if rt == nil || rt.store == nil || rt.assignedCapsule() == nil {
		return nil
	}
	// The same saga mutex guards the open/spawn/bind critical section: an
	// assignment committed by the durable open is legitimately Open+Unbound
	// while its saga spawns — without the lock this reconciler would reap a
	// live saga's pre-bind window as "stranded".
	rt.engineeringAssignmentOpenMu.Lock()
	defer rt.engineeringAssignmentOpenMu.Unlock()
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
			cancelled, cancelErr := rt.persistSystemEngineeringCancellation(ctx, assignment, "restart completed terminal trajectory assignment fate", crashBoot)
			if cancelErr != nil {
				return cancelErr
			}
			rt.stopCancelledAssignmentRun(ctx, cancelled.Assignment, "restart completed terminal trajectory assignment fate")
			continue
		}
		if crashBoot {
			if interruptErr := rt.interruptAssignmentAtCrashBoot(ctx, assignment); interruptErr != nil {
				return interruptErr
			}
			continue
		}
		if assignment.BoundRunID == "" {
			if assignment.CapsuleDisposition == types.EngineeringCapsuleUnbound && !assignment.CreatedAt.Before(rt.bootedAt) {
				// Opened by this boot: its delegated_assignment_spawn_deadline
				// wake, minted in the same commit, owns the pre-bind window.
				// Only an assignment opened before this boot is a restart strand.
				continue
			}
			if rt.plannedBootKeepsQueuedRequest(assignment, exec) {
				// A queued request (engineering_admission.go), not a spawn
				// strand: a planned boot keeps it for its one resume, which
				// the boot sweep arms.
				continue
			}
			if assignment.CapsuleDisposition == types.EngineeringCapsuleUnbound {
				intent := "capsule-revoke-intent:" + objectgraph.SHA256([]byte(assignment.AssignmentID+"\x00restart-pre-bind"))
				requested, fateErr := rt.store.SetEngineeringCapsuleDisposition(ctx, engineeringFateRequest(assignment, types.EngineeringCapsuleRevokeRequested, intent, ""))
				if fateErr != nil && (errors.Is(fateErr, store.ErrLifecycleCommandConflict) || errors.Is(fateErr, store.ErrEngineeringAssignmentInvalid)) {
					// A cleanup path may have recorded a different revoke intent
					// first; the recorded intent is the durable authority, so
					// reload and adopt it rather than conflicting forever.
					if current, loadErr := rt.store.GetEngineeringAssignment(ctx, ownerID, computerID, assignment.AssignmentID, assignment.Binding.Attempt); loadErr == nil {
						assignment = current
					}
				} else if fateErr != nil {
					return fateErr
				} else {
					assignment = requested.Assignment
				}
			}
			if assignment.CapsuleDisposition == types.EngineeringCapsuleRevokeRequested {
				intent := strings.TrimSpace(assignment.CapsuleIntentRef)
				if intent == "" {
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
				if fateErr != nil && (errors.Is(fateErr, store.ErrLifecycleCommandConflict) || errors.Is(fateErr, store.ErrEngineeringAssignmentInvalid)) {
					if current, loadErr := rt.store.GetEngineeringAssignment(ctx, ownerID, computerID, assignment.AssignmentID, assignment.Binding.Attempt); loadErr == nil && current.CapsuleDisposition == types.EngineeringCapsuleRevoked {
						assignment = current
					} else {
						return fateErr
					}
				} else if fateErr != nil {
					return fateErr
				} else {
					assignment = acked.Assignment
				}
			}
			if assignment.CapsuleDisposition != types.EngineeringCapsuleRevoked {
				return fmt.Errorf("restart open assignment has ambiguous capsule fate %s", assignment.CapsuleDisposition)
			}
			cancel := types.CancelEngineeringAssignmentRequest{
				CommandID: fmt.Sprintf("engineering-restart-open-cancel:%s:%d", assignment.AssignmentID, assignment.Binding.Attempt),
				OwnerID:   ownerID, ComputerID: computerID, AssignmentID: assignment.AssignmentID, Attempt: assignment.Binding.Attempt,
				ExpectedLifecycleVersion: assignment.LifecycleVersion, Reason: "restart acknowledged absent pre-bind assignment capsule",
			}
			cancel.CommandDigest, _ = store.ComputeCancelEngineeringAssignmentDigest(cancel)
			cancelled, cancelErr := rt.store.CancelEngineeringAssignment(ctx, cancel)
			if cancelErr != nil && (errors.Is(cancelErr, store.ErrLifecycleCommandConflict) || errors.Is(cancelErr, store.ErrEngineeringAssignmentInvalid)) {
				// Another reconciler may have cancelled or advanced the
				// assignment first; terminal/adopted state is convergence.
				if current, loadErr := rt.store.GetEngineeringAssignment(ctx, ownerID, computerID, assignment.AssignmentID, assignment.Binding.Attempt); loadErr == nil && current.Disposition.Terminal() {
					continue
				}
				return cancelErr
			}
			if cancelErr != nil {
				return cancelErr
			}
			if !cancelled.Replay {
				rt.failBoundSelfdevOperation(ctx, computerID, assignment.AssignmentID, cancel.Reason)
				rt.settleEngineeringPrecommitsAtTerminal(ctx, cancelled.Assignment, cancel.Reason)
				rt.wakeNextQueuedEngineeringRequest(ctx, cancelled.Assignment)
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
		usable := rt.assignedEngineeringCapsuleUsable(ctx, assignment)
		if usable {
			continue
		}
		assignment, err = rt.revokeAssignedCapsule(ctx, assignment, "restart executor binding reconciliation")
		if err != nil {
			return err
		}
		cancel := types.CancelEngineeringAssignmentRequest{
			CommandID: fmt.Sprintf("engineering-restart-cancel:%s:%d", assignment.AssignmentID, assignment.Binding.Attempt),
			OwnerID:   ownerID, ComputerID: computerID, AssignmentID: assignment.AssignmentID, Attempt: assignment.Binding.Attempt,
			ExpectedLifecycleVersion: assignment.LifecycleVersion, Reason: "restart revoked absent assignment capsule",
		}
		cancel.CommandDigest, _ = store.ComputeCancelEngineeringAssignmentDigest(cancel)
		cancelled, cancelErr := rt.store.CancelEngineeringAssignment(ctx, cancel)
		if cancelErr != nil && (errors.Is(cancelErr, store.ErrLifecycleCommandConflict) || errors.Is(cancelErr, store.ErrEngineeringAssignmentInvalid)) {
			if current, loadErr := rt.store.GetEngineeringAssignment(ctx, ownerID, computerID, assignment.AssignmentID, assignment.Binding.Attempt); loadErr == nil && current.Disposition.Terminal() {
				continue
			}
			return cancelErr
		}
		if cancelErr != nil {
			return cancelErr
		}
		if !cancelled.Replay {
			rt.failBoundSelfdevOperation(ctx, computerID, assignment.AssignmentID, cancel.Reason)
			rt.stopCancelledAssignmentRun(ctx, cancelled.Assignment, cancel.Reason)
			rt.settleEngineeringPrecommitsAtTerminal(ctx, cancelled.Assignment, cancel.Reason)
			rt.wakeNextQueuedEngineeringRequest(ctx, cancelled.Assignment)
		}
	}
	// Terminal trajectories close their bound self-development operation here:
	// the desk reconcile early-returns on non-live trajectories, so without
	// this edge a cancelled/settled trajectory strands the op at executing.
	if trajectory.Status != types.TrajectoryLive {
		rt.failTrajectorySelfdevOperations(ctx, computerID, trajectoryID,
			"trajectory "+string(trajectory.Status))
	}
	return nil
}

// interruptAssignmentAtCrashBoot revokes an open assignment's capsule (bound
// or not) and cancels it silently with reason `interrupted_by_restart`
// (C1e D7). The crash closure already closed its rounds, so the cancel mints
// no Report and wakes no one; a staged pending proposal is not resumed.
func (rt *Runtime) interruptAssignmentAtCrashBoot(ctx context.Context, assignment types.EngineeringAssignment) error {
	var err error
	if assignment.CapsuleDisposition != types.EngineeringCapsuleRevoked {
		assignment, err = rt.revokeAssignedCapsule(ctx, assignment, engineeringInterruptedByRestart)
		if err != nil {
			return err
		}
	}
	cancelled, err := rt.persistSystemEngineeringCancellation(ctx, assignment, engineeringInterruptedByRestart, true)
	if err != nil {
		return err
	}
	rt.stopCancelledAssignmentRun(ctx, cancelled.Assignment, engineeringInterruptedByRestart)
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
		_, planned := rt.BootWasPlannedRestart()
		if err := rt.reconcileEngineeringAssignmentsForTrajectory(ctx, assignment.Binding.OwnerID, assignment.Binding.ComputerID, assignment.Binding.TrajectoryID, !planned); err != nil {
			log.Printf("runtime: boot Engineering assignment capsule sweep trajectory %s: %v", assignment.Binding.TrajectoryID, err)
		}
	}
	if _, planned := rt.BootWasPlannedRestart(); planned {
		rt.resumeQueuedEngineeringRequestsAtPlannedBoot(ctx, computerID)
		rt.rearmBoundEngineeringDeadlinesAtPlannedBoot(ctx, computerID)
	}
}

// rearmBoundEngineeringDeadlinesAtPlannedBoot re-arms the bound deadline of
// every assignment a planned boot kept bound and active. The bind arms it
// after the bind commit, so a restart in between left the assignment with
// none (review round 2, R2). The wake's content is the bind-time one, so a
// surviving wake dedups. It arms a deadline, which can only end work; it
// resumes nothing.
func (rt *Runtime) rearmBoundEngineeringDeadlinesAtPlannedBoot(ctx context.Context, computerID string) {
	assignments, err := rt.store.ListEngineeringAssignmentsForComputer(ctx, computerID)
	if err != nil {
		log.Printf("runtime: planned boot: list bound engineering assignments: %v", err)
		return
	}
	for _, a := range assignments {
		if a.Disposition != types.EngineeringAssignmentBound || a.CapsuleDisposition != types.EngineeringCapsuleActive {
			continue
		}
		rt.armAssignedEngineeringDeadline(a)
	}
}

func (rt *Runtime) recordAssignedEngineeringReport(ctx context.Context, rec *types.RunRecord, toolCallID string, report types.EngineeringAssignmentReport) (types.EngineeringAssignmentCommandResult, error) {
	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		result, err := rt.recordAssignedEngineeringReportOnce(ctx, rec, toolCallID, report)
		if err == nil {
			// Every success, the early replay returns included: a crash or a
			// failed close after the report committed leaves a terminal
			// assignment that nothing else re-drives, and its re-delivered
			// report takes a replay return. One indexed read when there is
			// nothing to close.
			rt.closeEndedAssignmentSelfdevOperation(ctx, result.Assignment)
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
			assignment.AssignmentID, assignment.Binding.Attempt, report.ReportID, "engineering-report:"+assignment.AssignmentID+":"+report.ReportID)
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
				assignment.AssignmentID, assignment.Binding.Attempt, report.ReportID, "engineering-report:"+assignment.AssignmentID+":"+report.ReportID)
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
		requested, err := traceCall(rt, rec.RunID, "complete.freeze_intent_store", func() (types.EngineeringAssignmentCommandResult, error) {
			return rt.store.SetEngineeringCapsuleDisposition(ctx, freezeReq)
		})
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
		_, err = traceCall(rt, rec.RunID, "complete.freeze_capsule", func() ([]capsule.FileChange, error) {
			return rt.capsuleExecutor.ExtractGranted(ctx, rec.RunID, handle)
		})
		if err != nil {
			return freezeFailure(fmt.Errorf("freeze assignment after durable intent: %w", err))
		}
		diagnostics, err := rt.capsuleExecutor.InspectCapsuleRaw(assignment.Binding.CapsuleID)
		if err != nil || diagnostics.ID != assignment.Binding.CapsuleID || diagnostics.State != capsule.StateFrozen {
			return freezeFailure(fmt.Errorf("executor did not acknowledge exact frozen assignment capsule: %w", err))
		}
		frozenDigest, err := traceCall(rt, rec.RunID, "complete.digest_frozen_tree", func() (string, error) {
			return rt.capsuleExecutor.ResolveGrantedWorktreeDigest(ctx, rec.RunID, handle)
		})
		if err != nil || !types.ValidSHA256Digest(frozenDigest) {
			return freezeFailure(fmt.Errorf("frozen assignment digest unavailable: %w", err))
		}
		freezeReceipt, receiptErr := traceCall(rt, rec.RunID, "complete.freeze_receipt", func() (capsule.CapsuleFateReceipt, error) {
			return rt.capsuleExecutor.PersistGrantedFreezeReceipt(ctx, rec.RunID, handle)
		})
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
		frozen, err := traceCall(rt, rec.RunID, "complete.frozen_ack_store", func() (types.EngineeringAssignmentCommandResult, error) {
			return rt.store.SetEngineeringCapsuleDisposition(ctx, fateAck)
		})
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
		frozenDigest, digestErr := traceCall(rt, rec.RunID, "complete.digest_frozen_tree_again", func() (string, error) {
			return rt.capsuleExecutor.ResolveGrantedWorktreeDigest(ctx, rec.RunID, handle)
		})
		if digestErr != nil || !types.ValidSHA256Digest(frozenDigest) {
			return types.EngineeringAssignmentCommandResult{}, fmt.Errorf("already-frozen assignment digest unavailable: %w", digestErr)
		}
		report, resolveErr = traceCall(rt, rec.RunID, "complete.bind_receipts", func() (types.EngineeringAssignmentReport, error) {
			return rt.bindFrozenAssignmentExecutionReceipts(ctx, assignment, handle, report, frozenDigest)
		})
		if resolveErr != nil {
			return types.EngineeringAssignmentCommandResult{}, resolveErr
		}
		if frozenDigest != assignment.Binding.SubjectDigest && len(report.Mutations) == 0 {
			report.ObservedSubjectDigest = frozenDigest
			report.Mutations = []types.EngineeringRecordedMutation{{MutationID: "assignment-subject:" + objectgraph.SHA256([]byte(assignment.CapsuleAckRef)), Kind: "workspace_platform_complete_tree", BeforeDigest: assignment.Binding.SubjectDigest, AfterDigest: frozenDigest, EvidenceRef: "capsule-diff:" + objectgraph.SHA256([]byte(assignment.CapsuleAckRef)), SubjectBytesChanged: true}}
		}
		if frozenDigest != assignment.Binding.SubjectDigest {
			candidate, candidateErr := traceCall(rt, rec.RunID, "complete.export_candidate", func() (capsule.SourcePreflight, error) {
				return rt.capsuleExecutor.PersistGrantedCandidate(ctx, rec.RunID, handle)
			})
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
	// finds the capability and frozen state gone. The staged pending proposal
	// is the evidence-bearing resume source — it already carries the granted
	// executor refs and execution attestations minted while the capsule was
	// frozen. When it matches this report's proposition, commit it verbatim
	// (only the pinned observed-subject overlay survives in the authored
	// report). When no matching proposal exists, resolve the durable raw
	// execution receipts and carry their refs forward.
	if assignment.CapsuleDisposition == types.EngineeringCapsuleRevoked && !reportExists {
		if proposal := assignment.PendingProposal; proposal != nil && proposal.PropositionDigest == propositionDigest &&
			len(proposal.Report.Commands) == len(report.Commands) && proposal.Report.ReportID == report.ReportID &&
			len(proposal.Report.ExecutorReceiptRefs) == len(report.Commands) {
			report = proposal.Report
			// The staged report stores the authored observed digest; the
			// subject-change overlay is carried on Mutations. Restore the
			// frozen-derived observed digest so attestation validation sees
			// the same final subject the freeze certified.
			for _, mutation := range report.Mutations {
				if mutation.SubjectBytesChanged && strings.TrimSpace(mutation.AfterDigest) != "" {
					report.ObservedSubjectDigest = mutation.AfterDigest
				}
			}
		} else if len(report.ExecutorReceiptRefs) != len(report.Commands) {
			report, err = rt.bindLateAssignmentExecutionReceipts(assignment, report)
			if err != nil {
				return types.EngineeringAssignmentCommandResult{}, err
			}
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
		commandID := "engineering-report:" + assignment.AssignmentID + ":" + report.ReportID
		result, err = rt.store.ReplayRecordedEngineeringAssignmentReport(ctx, assignment.Binding.OwnerID, assignment.Binding.ComputerID, assignment.AssignmentID, assignment.Binding.Attempt, report.ReportID, commandID)
	} else {
		// Revocation must precede terminal commitment! No terminal state is visible until revoke ack.
		if assignment.CapsuleDisposition != types.EngineeringCapsuleRevoked {
			assignment, err = traceCall(rt, rec.RunID, "complete.revoke_capsule", func() (types.EngineeringAssignment, error) {
				return rt.revokeAssignedCapsule(ctx, assignment, "terminal assignment report recorded")
			})
			if err != nil {
				return types.EngineeringAssignmentCommandResult{}, err
			}
		}
		if assignment.CapsuleDisposition != types.EngineeringCapsuleRevoked {
			return types.EngineeringAssignmentCommandResult{}, fmt.Errorf("terminal report requires revoked executor acknowledgement")
		}
		result, err = traceCall(rt, rec.RunID, "complete.commit_report", func() (types.EngineeringAssignmentCommandResult, error) {
			return rt.commitAssignedEngineeringReport(ctx, assignment, report)
		})
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
// errAssignmentSagaHeld reports that the assignment's live terminal saga
// holds it; the fate-wake re-drive defers to that saga.
var errAssignmentSagaHeld = errors.New("the assignment's live terminal saga holds it")

// assignmentSagaMutex serializes the drivers of one assignment's terminal
// report in this process: the worker's Complete and the fate wake's resume
// of its staged proposal (FP4 round 4 P2; review round 4b). Both run in the
// host process (capsule cells reduce there, and the fate wake is handled
// there). The worker waits for it; the resume only tries it.
func (rt *Runtime) assignmentSagaMutex(assignmentID string, attempt uint64) *sync.Mutex {
	mu, _ := rt.assignmentSagas.LoadOrStore(assignmentID+"\x00"+strconv.FormatUint(attempt, 10), new(sync.Mutex))
	return mu.(*sync.Mutex)
}

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
		if errors.Is(resumeErr, errAssignmentSagaHeld) {
			// The live saga either commits the report or, on failure, arms
			// the fate watchdog, which re-drives once it is free.
			log.Printf("runtime: assignment %s fate re-drive deferred: %v", assignmentID, resumeErr)
			return nil
		}
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
		return report, executionEvidenceCountError("late assignment raw execution evidence unavailable", len(refs), len(receipts), err)
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
	saga := rt.assignmentSagaMutex(assignment.AssignmentID, assignment.Binding.Attempt)
	if !saga.TryLock() {
		return errAssignmentSagaHeld
	}
	defer saga.Unlock()
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

// assignmentSubjectBinding is what a terminal report's cited receipts must
// prove about the frozen subject. Only a report that claims failure (failed,
// blocked) is relieved of certifying the tree: it states nothing about the
// subject, so tying it to the subject made honest failure impossible after
// any receipt-less mutation
// (docs/problems/engineering-completion-path-cluster-2026-10-20.md). The
// relaxation is an allow-list: completed, partial, unknown and empty results
// keep the certified binding. The result is part of the proposition digest,
// so the model cannot change the mode of a recorded or staged proposal.
func assignmentSubjectBinding(result types.EngineeringAssignmentResultKind) capsule.SubjectBinding {
	switch result {
	case types.EngineeringResultFailed, types.EngineeringResultBlocked:
		return capsule.SubjectObserved
	default:
		return capsule.SubjectCertified
	}
}

func engineeringExecutionAttestationFromReceipt(assignment types.EngineeringAssignment, reportID string, command types.EngineeringRecordedCommand, receipt capsule.ExecutionReceipt, finalSubjectDigest string, binding capsule.SubjectBinding) (types.EngineeringExecutionAttestation, error) {
	if receipt.AgentRunID != assignment.BoundRunID || receipt.CapsuleID != assignment.Binding.CapsuleID || objectgraph.SHA256([]byte(receipt.Command)) != command.CommandDigest ||
		"sha256:"+strings.TrimPrefix(receipt.SourceTreeDigest, "sha256:") != assignment.Binding.SubjectDigest || strings.TrimSpace(receipt.GrantedReceiptRef) == "" {
		return types.EngineeringExecutionAttestation{}, fmt.Errorf("assignment granted receipt scope is invalid")
	}
	occurredAt, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(receipt.OccurredAt))
	if err != nil {
		return types.EngineeringExecutionAttestation{}, fmt.Errorf("assignment command receipt occurred_at is invalid")
	}
	// The attestation certifies the run/handle/capsule/command against the
	// frozen final subject — the granted receipt's FinalSubjectDigest, not the
	// per-command post-tree in receipt.WorktreeDigest. Intermediate commands
	// legitimately observe earlier tree states; per-command post-trees remain
	// on the raw execution receipts.
	attestation := types.EngineeringExecutionAttestation{
		GrantedReceiptRef: receipt.GrantedReceiptRef, CommandID: command.CommandID, CommandDigest: command.CommandDigest,
		ExitCode: receipt.ExitCode, StdoutDigest: "sha256:" + strings.TrimPrefix(receipt.StdoutDigest, "sha256:"), StderrDigest: "sha256:" + strings.TrimPrefix(receipt.StderrDigest, "sha256:"),
		SourceSubjectDigest: "sha256:" + strings.TrimPrefix(receipt.SourceTreeDigest, "sha256:"), FinalSubjectDigest: "sha256:" + strings.TrimPrefix(finalSubjectDigest, "sha256:"), WorktreeDigest: "sha256:" + strings.TrimPrefix(finalSubjectDigest, "sha256:"),
		Granted: true, Frozen: true, OccurredAt: occurredAt.UTC(), ReportID: reportID,
	}
	if binding == capsule.SubjectObserved {
		// No receipt certifies the frozen subject: record the tree this
		// command observed beside it, and say so.
		attestation.SubjectBinding = types.EngineeringSubjectBindingObserved
		attestation.WorktreeDigest = ""
		if observed := strings.TrimSpace(receipt.WorktreeDigest); observed != "" {
			attestation.WorktreeDigest = "sha256:" + strings.TrimPrefix(observed, "sha256:")
		}
	}
	return attestation, nil
}

func (rt *Runtime) bindFrozenAssignmentExecutionReceipts(ctx context.Context, assignment types.EngineeringAssignment, handle string, report types.EngineeringAssignmentReport, frozenDigest string) (types.EngineeringAssignmentReport, error) {
	refs := make([]string, 0, len(report.Commands))
	for _, command := range report.Commands {
		refs = append(refs, command.ExecutionRef)
	}
	if len(refs) == 0 {
		return report, nil
	}
	binding := assignmentSubjectBinding(report.Result)
	receipts, err := rt.capsuleExecutor.ResolveGrantedExecutionReceiptsFor(ctx, assignment.BoundRunID, handle, refs, binding)
	if err != nil || len(receipts) != len(refs) {
		return report, executionEvidenceCountError("assignment command evidence unavailable after durable freeze", len(refs), len(receipts), err)
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
			attestation, buildErr := engineeringExecutionAttestationFromReceipt(assignment, report.ReportID, report.Commands[i], receipt, frozenDigest, binding)
			if buildErr != nil {
				return report, buildErr
			}
			report.ExecutionAttestations = append(report.ExecutionAttestations, attestation)
		}
	}
	return report, nil
}

// executionEvidenceCountError names why cited execution evidence did not
// resolve one receipt per ref: the resolver's error, or the counts when it
// returned none (a nil %w printed "%!w(<nil>)" and named no cause).
func executionEvidenceCountError(what string, refs, receipts int, err error) error {
	if err != nil {
		return fmt.Errorf("%s: %w", what, err)
	}
	return fmt.Errorf("%s: %d cited refs resolved to %d receipts", what, refs, receipts)
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
				return types.EngineeringAssignmentCommandResult{}, executionEvidenceCountError("partial assignment command evidence unavailable", len(refs), len(receipts), err)
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
		CommandID: "engineering-report:" + assignment.AssignmentID + ":" + report.ReportID,
		OwnerID:   assignment.Binding.OwnerID, ComputerID: assignment.Binding.ComputerID,
		AssignmentID: assignment.AssignmentID, Attempt: assignment.Binding.Attempt,
		ExpectedLifecycleVersion: assignment.LifecycleVersion, Report: report,
		ExecutionAttestations: append([]types.EngineeringExecutionAttestation(nil), report.ExecutionAttestations...),
		OutcomeRefs:           rt.assignmentOutcomeRefs(ctx, assignment, report),
	}
	req.CommandDigest, _ = store.ComputeRecordEngineeringAssignmentReportDigest(req)
	result, err := rt.store.RecordEngineeringAssignmentReport(ctx, req)
	if err == nil && !result.Replay && result.Assignment.Disposition.Terminal() {
		rt.settleEngineeringPrecommitsAtTerminal(ctx, result.Assignment, string(report.Result)+": "+report.Summary)
		rt.wakeNextQueuedEngineeringRequest(ctx, result.Assignment)
	}
	if err == nil {
		// At the commit; recordAssignedEngineeringReport closes again on
		// every success, which covers its early replay returns.
		rt.closeEndedAssignmentSelfdevOperation(ctx, result.Assignment)
	}
	return result, err
}

// closeEndedAssignmentSelfdevOperation closes the self-development operation
// of an assignment its own report just turned terminal. A freeze refusal is
// retryable while the assignment lives, so it only stamps
// last_intent_error; once the assignment ends nothing can freeze, verify or
// decide the operation, and an executing one pinned the busy probe and every
// deploy refresh for 17 hours
// (docs/problems/selfdev-freeze-failure-leaves-operation-executing-2026-10-22.md).
// A completed assignment keeps a frozen candidate for management's Verify;
// a failed one cannot be verified, so its candidate closes too. Cancellation
// closes through failBoundSelfdevOperation. The close is a normal Transition,
// on the tape like every other, and a terminal operation is left alone. It
// runs after the report committed, so a cancelled request must not stop it;
// it keeps a deadline of its own, and every give-up is logged.
//
// Invariant, scoped to this close: it never closes a decided operation
// (awaiting_approval, accepted, materializing, applied, rollback_pending,
// degraded). Cancellation is different on purpose: failSelfdevOperation,
// for an assignment's or a trajectory's cancellation, also closes
// awaiting_approval.
func (rt *Runtime) closeEndedAssignmentSelfdevOperation(ctx context.Context, assignment types.EngineeringAssignment) {
	if rt == nil || rt.selfdevOperations == nil || !assignment.Disposition.Terminal() {
		return
	}
	ctx, cancel := selfdevCloseContext(ctx)
	defer cancel()
	closable := []string{selfdev.StateRequested, selfdev.StateExecuting}
	if assignment.Disposition != types.EngineeringAssignmentCompleted {
		closable = append(closable, selfdev.StateFrozen, selfdev.StateVerified)
	}
	computerID := assignment.Binding.ComputerID
	for attempt := 0; attempt < selfdevCloseAttempts; attempt++ {
		operation, found, err := rt.selfdevOperations.GetByIdempotency(ctx, computerID, store.EngineeringFreezeIdempotencyKey(assignment.AssignmentID))
		if err != nil {
			log.Printf("selfdev: close the operation of ended assignment %s: read: %v", assignment.AssignmentID, err)
			return
		}
		if !found || !slices.Contains(closable, operation.State) {
			return
		}
		reason := fmt.Sprintf("assignment %s before its candidate froze", assignment.Disposition)
		if operation.State == selfdev.StateFrozen || operation.State == selfdev.StateVerified {
			reason = fmt.Sprintf("assignment %s; its candidate cannot be verified", assignment.Disposition)
		}
		if last := strings.TrimSpace(operation.LastIntentError); last != "" {
			reason += "; last refusal: " + last
		}
		_, err = rt.selfdevOperations.Transition(ctx, computerID, operation.OperationID, operation.State, selfdev.StateFailed, func(next *selfdev.Operation) error {
			next.TerminalError = reason
			return nil
		})
		if err == nil {
			log.Printf("selfdev: %s closed from %s: %s", operation.OperationID, operation.State, reason)
			return
		}
		// A competing writer in this process loses as ErrConflict; one that
		// does not share the store's write mutex loses its tape CAS as
		// ErrProjectionMismatch. Both re-read and retry.
		if !selfdevCloseRetryable(err) {
			log.Printf("selfdev: close %s of ended assignment %s: %v", operation.OperationID, assignment.AssignmentID, err)
			return
		}
		log.Printf("selfdev: close %s of ended assignment %s: state moved, re-reading: %v", operation.OperationID, assignment.AssignmentID, err)
	}
	log.Printf("selfdev: close the operation of ended assignment %s: gave up after repeated state conflicts", assignment.AssignmentID)
}

// assignmentOutcomeRefs names, on a terminal report, the candidate the
// assignment froze: its own operation (C1e D2) and that operation's bundle
// digest, so management's Verify can name both from the final Report's
// evidence refs (D9).
func (rt *Runtime) assignmentOutcomeRefs(ctx context.Context, assignment types.EngineeringAssignment, report types.EngineeringAssignmentReport) []string {
	var refs []string
	if ref := strings.TrimSpace(report.CandidateArtifactRef); ref != "" {
		refs = append(refs, ref)
	}
	if report.Result == types.EngineeringResultPartial || rt == nil || rt.selfdevOperations == nil {
		return refs
	}
	operation, found, err := rt.selfdevOperations.GetByIdempotency(ctx, assignment.Binding.ComputerID, store.EngineeringFreezeIdempotencyKey(assignment.AssignmentID))
	if err != nil || !found {
		return refs
	}
	refs = append(refs, "selfdev-operation:"+operation.OperationID)
	if digest := strings.TrimSpace(operation.BundleDigest); digest != "" {
		refs = append(refs, "bundle:"+digest)
	}
	return refs
}
