package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/updater"
)

// Settling an open self-development operation that will not settle by itself
// (docs/problems/selfdev-stale-open-operations-block-engineering-2026-11-02.md,
// "Journal-first settlement"). Any operation in selfdev.OpenStates holds every
// engineering admission on its computer, so an operation with no terminal path
// is a computer-wide hold. Two acts settle one, both through
// settleSelfDevelopmentOperation, and both under selfdevMaterializeMu:
//   - the materializer, for a rollback whose pinned release (or the release
//     it installed) is gone or fails verification: a fact, not a decision;
//   - the owner's `abandon` act: a decision, with the owner's reason.
// The updater journal decides what a settle may do: an apply in flight is
// refused, one that installed nothing is fenced and settles failed, one
// journaled applied settles forward (applied / rolled_back), and one
// interrupted past its pointer swap is left to the updater's resume.

const (
	selfdevSettlementMediaType = "application/vnd.choir.selfdev-operation-settlement+json"
	selfdevSettlementSchema    = "choir.selfdev.operation-settlement.v1"

	selfdevSettleActPinnedUnavailable = "pinned_release_unavailable"
	selfdevSettleActPinnedCorrupt     = "pinned_release_failed_verification"
	selfdevSettleActAbandon           = "abandon"

	// selfdevPinnedReleaseUnavailableCause is the terminal error of a rollback
	// whose pinned release is gone before it installed anything.
	selfdevPinnedReleaseUnavailableCause = "pinned release unavailable; the computer stays on its current release"

	// selfdevSettledForwardTail is said of an operation settled forward without
	// its tail.
	selfdevSettledForwardTail = "the release it installed stays effective and is recorded on the tape; its checkpoint and route promotion are not"
)

var (
	// errSelfdevSettleConflict marks a settle refused because the operation's
	// state or the updater's work changed under it: a state conflict (HTTP 409).
	errSelfdevSettleConflict = errors.New("self-development settle conflict")
	// errSelfdevSettleInterrupted marks a settle refused because the apply's
	// journal is dead past its pointer swap: only the updater's resume can
	// say what is installed.
	errSelfdevSettleInterrupted = errors.New("the updater apply was interrupted after it moved the current release")
)

type selfdevOperationSettlement struct {
	Schema         string `json:"schema"`
	OperationID    string `json:"operation_id"`
	FromState      string `json:"from_state"`
	ToState        string `json:"to_state"`
	Act            string `json:"act"`
	Actor          string `json:"actor"`
	Reason         string `json:"reason"`
	IdempotencyKey string `json:"idempotency_key"`
}

// selfdevSettleSpec is one settle. TerminalError is the row's cause when it
// settles failed; ForwardNote when the journal shows the release installed
// and it settles forward.
type selfdevSettleSpec struct {
	Act, Actor, Authority, Reason, Key, TerminalError, ForwardNote string
}

func selfdevSettleIdempotency(act, computerID, operationID, key string) string {
	return "selfdev-settle-" + act + "-" + computerevent.DigestBytes([]byte(computerID+"\x00"+operationID+"\x00"+key))
}

// selfdevOperationIsRollback reports whether the operation is a rollback. A
// rollback opens rollback_pending under a "rollback-" id; a degraded one keeps
// the route receipt it copied from its target. The id survives the row
// leaving its open state.
func selfdevOperationIsRollback(operation selfdev.Operation) bool {
	return strings.HasPrefix(operation.OperationID, "rollback-") ||
		operation.State == selfdev.StateRollbackPending ||
		(operation.State == selfdev.StateDegraded && strings.TrimSpace(operation.RouteReceipt) != "")
}

// selfdevApplyJournalKey is the updater journal key of the operation's apply
// (or, for a rollback, its rollback apply).
func selfdevApplyJournalKey(operation selfdev.Operation) string {
	if selfdevOperationIsRollback(operation) {
		return "selfdev-rollback-apply-" + operation.DecisionEvent
	}
	return "selfdev-apply-" + operation.DecisionEvent
}

// fenceSelfDevelopmentApply asks the updater journal what the operation's
// apply did, fencing it when it installed nothing (updater.FenceApply). An
// operation with no decision has no apply to ask about.
func (rt *Runtime) fenceSelfDevelopmentApply(operation selfdev.Operation, reason string) (updater.FenceResult, error) {
	if strings.TrimSpace(operation.DecisionEvent) == "" || strings.TrimSpace(rt.selfdevUpdaterRoot) == "" {
		return updater.FenceResult{State: updater.FenceNothingInstalled}, nil
	}
	fence, err := updater.FenceApply(rt.selfdevUpdaterRoot, selfdevApplyJournalKey(operation),
		fmt.Sprintf("self-development operation %s settled before the updater installed anything: %s", operation.OperationID, reason))
	if err != nil {
		return updater.FenceResult{}, fmt.Errorf("updater journal unreadable: %w", err)
	}
	return fence, nil
}

// requireNothingInstalled refuses (a conflict) unless the updater journal
// proves the operation's apply installed nothing and is not running.
func (rt *Runtime) requireNothingInstalled(operation selfdev.Operation, reason string) (updater.FenceResult, error) {
	fence, err := rt.fenceSelfDevelopmentApply(operation, reason)
	if err != nil {
		return fence, err
	}
	switch fence.State {
	case updater.FenceNothingInstalled:
		return fence, nil
	case updater.FenceInFlight:
		return fence, fmt.Errorf("%w: operation %s has an apply in flight with the updater", errSelfdevSettleConflict, operation.OperationID)
	case updater.FenceApplied:
		return fence, fmt.Errorf("%w: operation %s has an applied release; it settles forward, never failed", errSelfdevSettleConflict, operation.OperationID)
	default:
		return fence, fmt.Errorf("%w: %w: operation %s", errSelfdevSettleConflict, errSelfdevSettleInterrupted, operation.OperationID)
	}
}

// pendingDecisionOf answers the head's pending transition ref when it is this
// operation's decision (the row's own, or a finalized decision on the tape the
// row has not caught up with), and "" when pending is empty or another's.
func (rt *Runtime) pendingDecisionOf(ctx context.Context, operation selfdev.Operation, pending string) (string, error) {
	if pending == "" {
		return "", nil
	}
	if operation.DecisionEvent != "" && operation.DecisionEvent == pending {
		return pending, nil
	}
	transition, found, err := rt.store.FinalizedDecisionForOperation(ctx, operation.ComputerID, operation.OperationID, operation.TrajectoryID, operation.CapsuleID)
	if err != nil {
		return "", err
	}
	if found && transition.Request.EventDigest == pending {
		return pending, nil
	}
	return "", nil
}

// dischargeSettledPending discharges the head's pending transition when it is
// the decision of an operation whose row has already left its open state, so
// pending is never left bound to a settled operation. It reads the head again
// (a decision may have landed after the row moved) and fences that decision's
// apply first: materialization_failed claims the prior effective release is
// the one running, which only the journal can prove.
func (rt *Runtime) dischargeSettledPending(ctx context.Context, operation selfdev.Operation, result updater.ApplyResult) error {
	head, err := rt.store.Head(ctx, operation.ComputerID)
	if err != nil || head == nil {
		return fmt.Errorf("settle: event head unavailable")
	}
	owned, err := rt.pendingDecisionOf(ctx, operation, head.PendingTransitionRef)
	if err != nil || owned == "" {
		return err
	}
	decided := operation
	decided.DecisionEvent = owned
	if _, err := rt.requireNothingInstalled(decided, "its operation settled"); err != nil {
		return err
	}
	_, err = rt.appendMaterializationFailedEvent(ctx, decided, result)
	return err
}

// settleSelfDevelopmentOperation settles an open operation by its updater
// journal. The caller holds selfdevMaterializeMu, so no materializer pass
// moves the row under it; a decide can still, and loses to (or beats) the
// row's expected-state CAS cleanly:
//   - apply in flight, or interrupted past its swap: refused (a conflict);
//   - journaled applied: settled forward with spec.ForwardNote
//     (settleSelfDevelopmentOperationForward);
//   - nothing installed (now fenced): the row moves to failed first, then the
//     pending transition is discharged if it is this operation's
//     (recordMaterializationFailed).
//
// The settlement goes on the tape as a recovery_recorded event after the row
// moved, so it never claims a settle that did not land.
func (rt *Runtime) settleSelfDevelopmentOperation(ctx context.Context, operation selfdev.Operation, spec selfdevSettleSpec) (selfdev.Operation, error) {
	fence, err := rt.fenceSelfDevelopmentApply(operation, spec.Reason)
	if err != nil {
		return selfdev.Operation{}, err
	}
	var settled selfdev.Operation
	switch fence.State {
	case updater.FenceInFlight:
		return selfdev.Operation{}, fmt.Errorf("%w: operation %s has an apply in flight with the updater; it settles when the updater finishes", errSelfdevSettleConflict, operation.OperationID)
	case updater.FenceInterrupted:
		return selfdev.Operation{}, fmt.Errorf("%w: %w: operation %s", errSelfdevSettleConflict, errSelfdevSettleInterrupted, operation.OperationID)
	case updater.FenceApplied:
		settled, err = rt.settleSelfDevelopmentOperationForward(ctx, operation, fence.Outcome.Result, spec.ForwardNote)
		if err != nil {
			return selfdev.Operation{}, settleConflict(err)
		}
	default:
		result := updater.ApplyResult{Outcome: spec.Act}
		if fence.Outcome.Terminal && !fence.Fenced {
			// A terminal non-applied outcome the updater wrote: the
			// discharge carries its result (and recovery receipt).
			result = fence.Outcome.Result
		}
		if err := rt.recordMaterializationFailed(ctx, operation, result, errors.New(spec.TerminalError), operation.State); err != nil {
			return selfdev.Operation{}, settleConflict(err)
		}
		if settled, err = rt.selfdevOperations.Get(ctx, operation.ComputerID, operation.OperationID); err != nil {
			return selfdev.Operation{}, err
		}
	}
	if err := rt.appendSettlementEvent(ctx, operation, operation.State, settled.State, spec); err != nil {
		return settled, err
	}
	return settled, nil
}

// settleSelfDevelopmentOperationForward records an operation whose updater
// journal is terminal applied: the release is installed, so the operation is
// applied (or rolled back), never failed. The applied event is recorded from
// the journal's signed receipts when it is not on the tape yet (it discharges
// the pending transition), and the row moves forward with the journal's
// release and receipt. The tail (checkpoint and route promotion) is not run:
// note says so on the row.
func (rt *Runtime) settleSelfDevelopmentOperationForward(ctx context.Context, operation selfdev.Operation, result updater.ApplyResult, note string) (selfdev.Operation, error) {
	switch operation.State {
	case selfdev.StateMaterializing, selfdev.StateRollbackPending, selfdev.StateDegraded:
	default:
		return selfdev.Operation{}, fmt.Errorf("%w: the journal is applied but the operation is %s", errSelfDevSettlementStateMismatch, operation.State)
	}
	rollback := selfdevOperationIsRollback(operation)
	if err := rt.recordSettlementApplied(ctx, operation, result, rollback); err != nil {
		return selfdev.Operation{}, err
	}
	receiptBytes, err := result.MaterializationReceipt.CanonicalBytes()
	if err != nil {
		return selfdev.Operation{}, err
	}
	head, err := rt.store.Head(ctx, operation.ComputerID)
	if err != nil || head == nil {
		return selfdev.Operation{}, fmt.Errorf("settle: event head unavailable")
	}
	next := selfdev.StateApplied
	if rollback {
		next = selfdev.StateRolledBack
	}
	return rt.selfdevOperations.Transition(ctx, operation.ComputerID, operation.OperationID, operation.State, next, func(row *selfdev.Operation) error {
		row.DesiredHead, row.EffectiveHead = head.DesiredEventHead, head.EffectiveEventHead
		row.MaterializationReceipt = computerevent.DigestBytes(receiptBytes)
		row.ReleaseDigest = result.ReleaseDigest
		row.TerminalError = note
		return nil
	})
}

// appendSettlementEvent puts the settlement on the tape, idempotently on its
// act and key.
func (rt *Runtime) appendSettlementEvent(ctx context.Context, operation selfdev.Operation, from, to string, spec selfdevSettleSpec) error {
	idempotency := selfdevSettleIdempotency(spec.Act, operation.ComputerID, operation.OperationID, spec.Key)
	if _, found, err := rt.store.EventByIdempotency(ctx, operation.ComputerID, idempotency); err != nil || found {
		return err
	}
	payload, err := computerevent.CanonicalJSON(selfdevOperationSettlement{
		Schema: selfdevSettlementSchema, OperationID: operation.OperationID, FromState: from, ToState: to,
		Act: spec.Act, Actor: spec.Actor, Reason: spec.Reason, IdempotencyKey: spec.Key,
	})
	if err != nil {
		return err
	}
	eventID, err := computerevent.NewEventID()
	if err != nil {
		return err
	}
	event := computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: operation.ComputerID,
		EventKind: computerevent.EventRecoveryRecorded, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
		IdempotencyKey: idempotency, TrajectoryID: operation.TrajectoryID, CapsuleID: operation.CapsuleID,
		ParentEventID: operation.OperationID,
		ActorProfile:  agentprofile.Management, AuthorityRef: spec.Authority, PrivacyClass: "owner",
		ProposedEffectRef: operation.BundleDigest, DecisionRef: operation.DecisionEvent,
		ReducerVersion: computerevent.ReducerVersionV1,
	}
	_, _, err = rt.eventAppender.AppendNewPayload(ctx, event, computerevent.TransitionInput{}, payload, selfdevSettlementMediaType, "owner")
	return err
}

func settleConflict(err error) error {
	if errors.Is(err, errSelfdevSettleConflict) {
		return err
	}
	if errors.Is(err, selfdev.ErrConflict) || errors.Is(err, selfdev.ErrInvalidTransition) ||
		errors.Is(err, computerevent.ErrInvalidTransition) || errors.Is(err, computerevent.ErrProjectionMismatch) ||
		errors.Is(err, computerevent.ErrPendingTransition) || errors.Is(err, errSelfDevSettlementStateMismatch) {
		return fmt.Errorf("%w: %w", errSelfdevSettleConflict, err)
	}
	return err
}

// settleRollbackPinnedReleaseUnusable settles a rollback (degraded or
// rollback_pending) whose pinned release, or the release it installed, is
// gone or fails verification: the re-drive can never succeed. The journal
// decides how: nothing installed settles failed and the computer keeps the
// release it runs; an applied rollback settles forward without its tail. The
// tape tells the two causes apart by the settlement's act. The caller is the
// materializer pass, which holds selfdevMaterializeMu.
func (rt *Runtime) settleRollbackPinnedReleaseUnusable(ctx context.Context, operation selfdev.Operation, readErr error) error {
	spec := selfdevSettleSpec{
		Act: selfdevSettleActPinnedUnavailable, Actor: "guest-core:choir-updater", Authority: "guest-core:choir-updater",
		Key: "pinned-release-unavailable", TerminalError: selfdevPinnedReleaseUnavailableCause,
	}
	cause := "release unavailable"
	if errors.Is(readErr, updater.ErrPinnedReleaseCorrupt) {
		spec.Act, spec.Key = selfdevSettleActPinnedCorrupt, "pinned-release-failed-verification"
		spec.TerminalError = strings.TrimPrefix(readErr.Error(), "updater: ")
		cause = spec.TerminalError
	}
	spec.Reason = spec.TerminalError
	spec.ForwardNote = "the rollback is installed (" + cause + "): " + selfdevSettledForwardTail
	settled, err := rt.settleSelfDevelopmentOperation(ctx, operation, spec)
	if err != nil {
		return err
	}
	log.Printf("selfdev materializer: operation %s was %s and a release it needs is unusable (%s); settled %s", operation.OperationID, operation.State, spec.Act, settled.State)
	return nil
}

// selfDevelopmentAbandonRequest is the owner's abandon act.
type selfDevelopmentAbandonRequest struct {
	OperationID    string `json:"operation_id"`
	Reason         string `json:"reason"`
	IdempotencyKey string `json:"idempotency_key"`
}

// abandonableSelfDevelopmentState reports the states an owner can abandon:
// every open state. What abandon does is the journal's call
// (settleSelfDevelopmentOperation).
func abandonableSelfDevelopmentState(state string) bool {
	for _, open := range selfdev.OpenStates {
		if state == open {
			return true
		}
	}
	return false
}

func selfDevelopmentAbandonTerminalError(reason string) string {
	return "abandoned by the owner: " + reason
}

func selfDevelopmentAbandonForwardNote(reason string) string {
	return selfDevelopmentAbandonTerminalError(reason) + "; " + selfdevSettledForwardTail
}

// selfdevAbandonWait is how long abandon waits for a materializer pass to
// finish before it answers 409.
const selfdevAbandonWait = 3 * time.Second

// abandonSelfDevelopmentOperation is the owner's act: close an open operation
// with the owner's reason, recorded on the tape as an owner decision. It
// answers an HTTP status like decideSelfDevelopment.
func (rt *Runtime) abandonSelfDevelopmentOperation(ctx context.Context, ownerID, computerID string, request selfDevelopmentAbandonRequest) (selfdev.Operation, int, error) {
	request.OperationID, request.Reason, request.IdempotencyKey = strings.TrimSpace(request.OperationID), strings.TrimSpace(request.Reason), strings.TrimSpace(request.IdempotencyKey)
	if request.OperationID == "" || request.Reason == "" || request.IdempotencyKey == "" {
		return selfdev.Operation{}, http.StatusBadRequest, errors.New("operation_id, reason and idempotency_key are required")
	}
	if rt.selfdevOperations == nil || rt.eventAppender == nil || rt.store == nil {
		return selfdev.Operation{}, http.StatusServiceUnavailable, errors.New("abandon authority unavailable")
	}
	spec := selfdevSettleSpec{
		Act: selfdevSettleActAbandon, Actor: "owner:" + ownerID, Authority: selfDevelopmentDecisionAuthority(ownerID, ""),
		Reason: request.Reason, Key: request.IdempotencyKey,
		TerminalError: selfDevelopmentAbandonTerminalError(request.Reason), ForwardNote: selfDevelopmentAbandonForwardNote(request.Reason),
	}
	if _, err := rt.selfdevOperations.Get(ctx, computerID, request.OperationID); err != nil {
		return selfdev.Operation{}, http.StatusNotFound, errors.New("self-development operation not found")
	}
	// Every settle runs under the materializer's mutex, so no pass moves the
	// row under it. A pass that only retries a failed read is short, so wait
	// a moment for it; one inside a long apply outlasts the wait and is
	// refused.
	locked := false
	for deadline := time.Now().Add(selfdevAbandonWait); !locked && time.Now().Before(deadline) && ctx.Err() == nil; {
		if locked = rt.selfdevMaterializeMu.TryLock(); !locked {
			time.Sleep(20 * time.Millisecond)
		}
	}
	if !locked {
		return selfdev.Operation{}, http.StatusConflict, fmt.Errorf("self-development operation %s: the materializer is working; try again when it is idle", request.OperationID)
	}
	readmitted := false
	defer func() {
		rt.selfdevMaterializeMu.Unlock()
		if readmitted {
			rt.triggerSelfDevelopmentReconcile()
		}
	}()
	operation, err := rt.selfdevOperations.Get(ctx, computerID, request.OperationID)
	if err != nil {
		return selfdev.Operation{}, http.StatusNotFound, errors.New("self-development operation not found")
	}
	if !abandonableSelfDevelopmentState(operation.State) {
		return rt.replayAbandon(ctx, operation, spec)
	}
	if operation.State == selfdev.StateAwaitingApproval {
		// A decision already on the tape is settling; abandoning would fight it.
		if _, decided, decisionErr := rt.store.FinalizedDecisionForOperation(ctx, computerID, operation.OperationID, operation.TrajectoryID, operation.CapsuleID); decisionErr != nil {
			return selfdev.Operation{}, http.StatusInternalServerError, errors.New("decision lookup failed")
		} else if decided {
			return selfdev.Operation{}, http.StatusConflict, errors.New("a decision is already recorded for this operation; the materializer settles it")
		}
	}
	settled, err := rt.settleSelfDevelopmentOperation(ctx, operation, spec)
	if errors.Is(err, errSelfdevSettleInterrupted) && rt.selfdevCrashInterrupted[operation.OperationID] {
		// A crash boot holds an apply interrupted past its swap: what is
		// installed is unknown until the updater's own resume finishes it.
		// The owner's act re-admits the operation for one materializer pass,
		// which calls the updater and records the outcome its journal reaches
		// (not a boot-time resume: AGENTS.md "Restarts End Work").
		delete(rt.selfdevCrashInterrupted, operation.OperationID)
		readmitted = true
		return operation, http.StatusAccepted, nil
	}
	if err != nil {
		if errors.Is(err, errSelfdevSettleConflict) {
			return selfdev.Operation{}, http.StatusConflict, err
		}
		return selfdev.Operation{}, http.StatusInternalServerError, err
	}
	rt.notifyTextureOfOwnerAct(ctx, ownerID, settled, types.OwnerActAbandon, request.Reason)
	return settled, http.StatusOK, nil
}

// replayAbandon answers an abandon of an operation that is no longer open. A
// replay of the abandon that settled it (same reason) finishes what that
// settle may not have: the settlement event and the discharge of a pending
// transition still bound to its decision. Anything else is a conflict.
func (rt *Runtime) replayAbandon(ctx context.Context, operation selfdev.Operation, spec selfdevSettleSpec) (selfdev.Operation, int, error) {
	if operation.TerminalError == spec.TerminalError || operation.TerminalError == spec.ForwardNote {
		if err := rt.appendSettlementEvent(ctx, operation, "", operation.State, spec); err != nil {
			return selfdev.Operation{}, http.StatusInternalServerError, err
		}
		if operation.State == selfdev.StateFailed {
			if err := rt.dischargeSettledPending(ctx, operation, updater.ApplyResult{Outcome: spec.Act}); err != nil {
				if errors.Is(settleConflict(err), errSelfdevSettleConflict) {
					return selfdev.Operation{}, http.StatusConflict, err
				}
				return selfdev.Operation{}, http.StatusInternalServerError, err
			}
		}
		return operation, http.StatusOK, nil
	}
	idempotency := selfdevSettleIdempotency(spec.Act, operation.ComputerID, operation.OperationID, spec.Key)
	if _, spent, err := rt.store.EventByIdempotency(ctx, operation.ComputerID, idempotency); err != nil {
		return selfdev.Operation{}, http.StatusInternalServerError, errors.New("abandon event lookup failed")
	} else if spent {
		return selfdev.Operation{}, http.StatusConflict, errors.New("this idempotency key is already spent on an abandon with a different reason")
	}
	return selfdev.Operation{}, http.StatusConflict, fmt.Errorf("self-development operation %s is %s, not open; there is nothing to abandon", operation.OperationID, operation.State)
}

// dischargeOrphanedPendingTransition is the reconciler's repair for a pending
// transition bound to the decision of an operation that is no longer open: a
// decide whose event landed after a settle moved the row, or a settle whose
// discharge failed after its row move. Without it every later decision and
// rollback is refused (the reducer's pending check). The caller holds
// selfdevMaterializeMu; open lists the open operations it read.
func (rt *Runtime) dischargeOrphanedPendingTransition(ctx context.Context, open []selfdev.Operation) {
	head, err := rt.store.Head(ctx, rt.selfdevComputerID)
	if err != nil || head == nil || head.PendingTransitionRef == "" {
		return
	}
	pending := head.PendingTransitionRef
	for _, operation := range open {
		if operation.DecisionEvent == pending {
			return
		}
	}
	event, found, err := rt.store.EventByDigest(ctx, rt.selfdevComputerID, pending)
	if err != nil || !found {
		return
	}
	var owner selfdev.Operation
	located := false
	if event.EventKind == computerevent.EventEffectAccepted && strings.TrimSpace(event.ParentEventID) != "" {
		if operation, getErr := rt.selfdevOperations.Get(ctx, rt.selfdevComputerID, event.ParentEventID); getErr == nil {
			owner, located = operation, true
		}
	}
	if !located && event.EventKind == computerevent.EventRollbackRequested {
		// A rollback's request event names no operation; the rollback row
		// carries the event as its decision.
		failed, listErr := rt.selfdevOperations.ListByStates(ctx, rt.selfdevComputerID, selfdev.StateFailed)
		if listErr != nil {
			return
		}
		for _, operation := range failed {
			if operation.DecisionEvent == pending {
				owner, located = operation, true
				break
			}
		}
	}
	if !located || owner.State != selfdev.StateFailed {
		return // an open operation's decision (a decide or rollback start still landing), or not ours
	}
	if err := rt.dischargeSettledPending(ctx, owner, updater.ApplyResult{Outcome: "settled"}); err != nil {
		log.Printf("selfdev materializer: pending transition %s belongs to failed operation %s and stays bound: %v", pending, owner.OperationID, err)
		return
	}
	log.Printf("selfdev materializer: discharged pending transition %s of failed operation %s", pending, owner.OperationID)
}

// abandonSelfDevelopmentOperation is the guest route's handler.
func (h *APIHandler) abandonSelfDevelopmentOperation(w http.ResponseWriter, r *http.Request, ownerID, computerID string) {
	var request selfDevelopmentAbandonRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid abandon request"})
		return
	}
	operation, status, err := h.rt.abandonSelfDevelopmentOperation(r.Context(), ownerID, computerID, request)
	if err != nil {
		writeAPIJSON(w, status, apiError{Error: err.Error()})
		return
	}
	writeAPIJSON(w, status, operation)
}
