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
// is a computer-wide hold. One rule settles every one, under
// selfdevMaterializeMu: hold the apply key (updater.HoldApplyKey) and act on
// its verdict. A running apply is refused; a terminal journal is the
// authority; a dead journal is arbitrated on the updater's `current` pointer:
// the prior release settles failed, the target settles forward, neither
// settles failed with the effective release named unknown. Every verdict but a
// running apply (or an older daemon's unproven lock) is terminal.

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

	// selfdevArbitratedOutcome is the updater-result outcome recorded for an
	// apply whose dead journal was arbitrated on the current pointer.
	selfdevArbitratedOutcome = "arbitrated_on_current"
)

var (
	// errSelfdevSettleConflict marks a settle refused because the operation's
	// state or the updater's work changed under it: a state conflict (HTTP 409).
	errSelfdevSettleConflict = errors.New("self-development settle conflict")
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

// holdSelfDevelopmentApply holds the operation's apply key and answers its
// verdict (updater.HoldApplyKey). An operation with no decision has no apply.
// The caller releases the hold.
func (rt *Runtime) holdSelfDevelopmentApply(operation selfdev.Operation) (*updater.ApplyKeyHold, error) {
	if strings.TrimSpace(operation.DecisionEvent) == "" || strings.TrimSpace(rt.selfdevUpdaterRoot) == "" {
		return updater.NoApplyKey(), nil
	}
	hold, err := updater.HoldApplyKey(rt.selfdevUpdaterRoot, selfdevApplyJournalKey(operation))
	if err != nil {
		return nil, fmt.Errorf("updater journal unreadable: %w", err)
	}
	return hold, nil
}

// refuseVerdict answers the conflict for a verdict no settle may act on.
func refuseVerdict(operation selfdev.Operation, hold *updater.ApplyKeyHold) error {
	switch hold.Verdict {
	case updater.VerdictInFlight:
		return fmt.Errorf("%w: operation %s has an apply in flight with the updater; it settles when the updater finishes", errSelfdevSettleConflict, operation.OperationID)
	case updater.VerdictLockUnproven:
		return fmt.Errorf("%w: operation %s: the updater daemon predates the per-key lock, so a running apply cannot be ruled out; it settles once the base image carries the lock, or from a terminal journal", errSelfdevSettleConflict, operation.OperationID)
	}
	return nil
}

// unknownEffectiveNote is added to the cause of an operation settled failed
// whose effective release is unknown.
func unknownEffectiveNote(hold *updater.ApplyKeyHold) string {
	return "; the effective release is unknown: the updater journal was interrupted and " + hold.Cause
}

// failSettledRow moves an operation failed under the held key: the row first,
// under its expected-state CAS (a lost CAS changes nothing, not even the
// journal), then the fence of a dead journal, then, with the key released,
// the discharge of a pending transition that is this operation's decision.
func (rt *Runtime) failSettledRow(ctx context.Context, operation selfdev.Operation, hold *updater.ApplyKeyHold, result updater.ApplyResult, cause, expectedState string) error {
	recoveryDigest := ""
	if result.RecoveryReceipt != nil {
		recoveryBytes, err := result.RecoveryReceipt.CanonicalBytes()
		if err != nil {
			return err
		}
		recoveryDigest = computerevent.DigestBytes(recoveryBytes)
	}
	head, err := rt.store.Head(ctx, operation.ComputerID)
	if err != nil || head == nil {
		return fmt.Errorf("materializer: recovery projection unavailable")
	}
	owned, err := rt.pendingDecisionOf(ctx, operation, head.PendingTransitionRef)
	if err != nil {
		return err
	}
	settled, err := rt.selfdevOperations.Transition(ctx, operation.ComputerID, operation.OperationID, expectedState, selfdev.StateFailed, func(next *selfdev.Operation) error {
		next.MaterializationReceipt = recoveryDigest
		// The discharge below returns desired to effective.
		next.DesiredHead, next.EffectiveHead = head.DesiredEventHead, head.EffectiveEventHead
		if owned != "" {
			next.DesiredHead = head.EffectiveEventHead
		}
		next.TerminalError = cause
		return nil
	})
	if err != nil {
		return err
	}
	if settled.DecisionEvent == "" {
		settled.DecisionEvent = operation.DecisionEvent
	}
	dischargeErr := rt.dischargeSettledPending(ctx, settled, result, hold)
	if err := hold.Fence("self-development operation " + operation.OperationID + " settled failed: " + cause); err != nil {
		log.Printf("selfdev settle: operation %s failed; its apply key could not be fenced: %v", operation.OperationID, err)
	}
	hold.Release()
	return dischargeErr
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
// the decision of an operation whose row already left its open state, by the
// same rule: the decision's apply key is held and its verdict decides the
// event. Nothing installed (or unknown) discharges with materialization_failed
// (the reducer's only failed discharge; an unknown verdict's payload says
// unknown); an applied journal discharges forward with its signed receipts,
// and a dead journal whose target is current with an arbitrated applied
// event. A running apply leaves it for a later pass.
// held is the caller's hold when it already holds the operation's own key
// (nil otherwise); the verdict and result are then the caller's.
func (rt *Runtime) dischargeSettledPending(ctx context.Context, operation selfdev.Operation, result updater.ApplyResult, held *updater.ApplyKeyHold) error {
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
	if held != nil && held.Verdict != "" && owned == operation.DecisionEvent && operation.DecisionEvent != "" {
		// The caller holds this key and has chosen the result.
		_, err := rt.appendMaterializationFailedEvent(ctx, decided, result)
		return err
	}
	hold, err := rt.holdSelfDevelopmentApply(decided)
	if err != nil {
		return err
	}
	defer hold.Release()
	if err := refuseVerdict(decided, hold); err != nil {
		return err
	}
	rollback := selfdevOperationIsRollback(decided)
	switch hold.Verdict {
	case updater.VerdictApplied:
		return rt.recordSettlementApplied(ctx, decided, hold.Outcome.Result, rollback)
	case updater.VerdictInstalled:
		if err := rt.appendArbitratedAppliedEvent(ctx, decided, hold, rollback); err != nil {
			return err
		}
	case updater.VerdictFailed:
		result = hold.Outcome.Result
		fallthrough
	default:
		if hold.Verdict == updater.VerdictUnknown {
			result = updater.ApplyResult{Outcome: "unknown_effective", ReleaseDigest: hold.Target, PriorReleaseDigest: hold.Prior}
		}
		if _, err := rt.appendMaterializationFailedEvent(ctx, decided, result); err != nil {
			return err
		}
	}
	return hold.Fence("the decision of settled operation " + operation.OperationID + " was discharged")
}

// appendArbitratedAppliedEvent records an apply whose dead journal's target
// is the current release: the applied event, with the arbitration as its
// result (there are no signed receipts: the updater never finished).
func (rt *Runtime) appendArbitratedAppliedEvent(ctx context.Context, operation selfdev.Operation, hold *updater.ApplyKeyHold, rollback bool) error {
	kind := computerevent.EventMaterializationApplied
	if rollback {
		kind = computerevent.EventRollbackApplied
	}
	return rt.appendMaterializationAppliedEvent(ctx, operation, updater.ApplyResult{
		Outcome: selfdevArbitratedOutcome, ReleaseDigest: hold.Target, PriorReleaseDigest: hold.Prior,
	}, kind)
}

// settleSelfDevelopmentOperation settles an open operation by the one rule.
// The caller holds selfdevMaterializeMu, so no materializer pass moves the row
// under it; a decide can still, and only an awaiting_approval row, which has
// no apply key.
//   - apply in flight (or an older daemon's unproven lock): refused;
//   - journaled applied, or dead with its target current: settled forward
//     with spec.ForwardNote (settleSelfDevelopmentOperationForward);
//   - anything else: settled failed (failSettledRow), with the journal's
//     result when it has one, and the unknown effective release named when
//     `current` is neither the prior nor the target.
//
// The settlement goes on the tape as a recovery_recorded event after the row
// moved, so it never claims a settle that did not land.
func (rt *Runtime) settleSelfDevelopmentOperation(ctx context.Context, operation selfdev.Operation, spec selfdevSettleSpec) (selfdev.Operation, error) {
	hold, err := rt.holdSelfDevelopmentApply(operation)
	if err != nil {
		return selfdev.Operation{}, err
	}
	defer hold.Release()
	if err := refuseVerdict(operation, hold); err != nil {
		return selfdev.Operation{}, err
	}
	var settled selfdev.Operation
	switch hold.Verdict {
	case updater.VerdictApplied, updater.VerdictInstalled:
		note := spec.ForwardNote
		if hold.Verdict == updater.VerdictInstalled {
			note += " (recorded from the updater's current pointer: " + hold.Cause + ")"
		}
		settled, err = rt.settleSelfDevelopmentOperationForward(ctx, operation, hold, note)
		if err != nil {
			return selfdev.Operation{}, settleConflict(err)
		}
	default:
		result, cause := updater.ApplyResult{Outcome: spec.Act}, spec.TerminalError
		switch hold.Verdict {
		case updater.VerdictFailed:
			result = hold.Outcome.Result
		case updater.VerdictUnknown:
			result = updater.ApplyResult{Outcome: "unknown_effective", ReleaseDigest: hold.Target, PriorReleaseDigest: hold.Prior}
			cause += unknownEffectiveNote(hold)
		}
		if err := rt.failSettledRow(ctx, operation, hold, result, cause, operation.State); err != nil {
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

// settleSelfDevelopmentOperationForward records an operation whose apply is
// installed: a terminal applied journal (its signed receipts) or a dead one
// whose target is the current release (arbitrated). The operation is applied
// (or rolled back), never failed. The applied event is recorded when the
// pending transition is still this operation's decision (it discharges it);
// then the row moves forward, then a dead journal is fenced. The event comes
// before the row here, as in recordMaterializationApplied: under
// selfdevMaterializeMu nothing else moves a materializing, rollback_pending or
// degraded row. The tail (checkpoint and route promotion) is not run: note
// says so on the row.
func (rt *Runtime) settleSelfDevelopmentOperationForward(ctx context.Context, operation selfdev.Operation, hold *updater.ApplyKeyHold, note string) (selfdev.Operation, error) {
	switch operation.State {
	case selfdev.StateMaterializing, selfdev.StateRollbackPending, selfdev.StateDegraded:
	default:
		return selfdev.Operation{}, fmt.Errorf("%w: the release is installed but the operation is %s", errSelfDevSettlementStateMismatch, operation.State)
	}
	rollback := selfdevOperationIsRollback(operation)
	head, err := rt.store.Head(ctx, operation.ComputerID)
	if err != nil || head == nil {
		return selfdev.Operation{}, fmt.Errorf("settle: event head unavailable")
	}
	receipt, release := "", hold.Target
	if hold.Verdict == updater.VerdictApplied {
		if err := rt.recordSettlementApplied(ctx, operation, hold.Outcome.Result, rollback); err != nil {
			return selfdev.Operation{}, err
		}
		receiptBytes, err := hold.Outcome.Result.MaterializationReceipt.CanonicalBytes()
		if err != nil {
			return selfdev.Operation{}, err
		}
		receipt, release = computerevent.DigestBytes(receiptBytes), hold.Outcome.Result.ReleaseDigest
	} else if head.PendingTransitionRef != "" && head.PendingTransitionRef == operation.DecisionEvent {
		if err := rt.appendArbitratedAppliedEvent(ctx, operation, hold, rollback); err != nil {
			return selfdev.Operation{}, err
		}
	}
	if head, err = rt.store.Head(ctx, operation.ComputerID); err != nil || head == nil {
		return selfdev.Operation{}, fmt.Errorf("settle: event head unavailable")
	}
	next := selfdev.StateApplied
	if rollback {
		next = selfdev.StateRolledBack
	}
	settled, err := rt.selfdevOperations.Transition(ctx, operation.ComputerID, operation.OperationID, operation.State, next, func(row *selfdev.Operation) error {
		row.DesiredHead, row.EffectiveHead = head.DesiredEventHead, head.EffectiveEventHead
		if receipt != "" {
			row.MaterializationReceipt = receipt
		}
		if release != "" {
			row.ReleaseDigest = release
		}
		row.TerminalError = note
		return nil
	})
	if err != nil {
		return selfdev.Operation{}, err
	}
	if err := hold.Fence("self-development operation " + operation.OperationID + " settled forward"); err != nil {
		log.Printf("selfdev settle: operation %s settled forward; its apply key could not be fenced: %v", operation.OperationID, err)
	}
	return settled, nil
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
	defer rt.selfdevMaterializeMu.Unlock()
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
			if err := rt.dischargeSettledPending(ctx, operation, updater.ApplyResult{Outcome: spec.Act}, nil); err != nil {
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
	if err := rt.dischargeSettledPending(ctx, owner, updater.ApplyResult{Outcome: "settled"}, nil); err != nil {
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
