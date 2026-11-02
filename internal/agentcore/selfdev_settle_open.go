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
// (docs/problems/selfdev-stale-open-operations-block-engineering-2026-11-02.md).
// Any operation in selfdev.OpenStates holds every engineering admission on its
// computer, so an operation with no terminal path is a computer-wide hold.
// Two settle it to `failed`, both through settleSelfDevelopmentOperationFailed:
//   - the materializer, for a rollback (degraded or rollback_pending) whose
//     pinned release is gone or fails verification (a fact, not a decision);
//   - the owner's `abandon` act, for an open operation that installed nothing,
//     a degraded one, or a materializing/rollback_pending one whose updater
//     journal shows no applied or mid-flight apply (a decision, with the
//     owner's reason).
// Both put the settlement on the tape first, as a recovery_recorded event, so
// the state row is never the only record.

const (
	selfdevSettlementMediaType = "application/vnd.choir.selfdev-operation-settlement+json"
	selfdevSettlementSchema    = "choir.selfdev.operation-settlement.v1"

	selfdevSettleActPinnedUnavailable = "pinned_release_unavailable"
	selfdevSettleActPinnedCorrupt     = "pinned_release_failed_verification"
	selfdevSettleActAbandon           = "abandon"

	// selfdevPinnedReleaseUnavailableCause is the terminal error of a rollback
	// whose pinned release is gone.
	selfdevPinnedReleaseUnavailableCause = "pinned release unavailable; the computer stays on its current release"
)

// errSelfdevSettleConflict marks a settle refused because the operation's
// state or the updater's work changed under it: a state conflict (HTTP 409).
var errSelfdevSettleConflict = errors.New("self-development settle conflict")

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

type selfdevSettleSpec struct {
	Act, Actor, Authority, Reason, Key, TerminalError string
}

func selfdevSettleIdempotency(act, computerID, operationID, key string) string {
	return "selfdev-settle-" + act + "-" + computerevent.DigestBytes([]byte(computerID+"\x00"+operationID+"\x00"+key))
}

// selfdevApplyJournalKey is the updater journal key of the operation's apply
// (or, for a rollback, its rollback apply).
func selfdevApplyJournalKey(operation selfdev.Operation) string {
	if operation.State == selfdev.StateRollbackPending || (operation.State == selfdev.StateDegraded && strings.TrimSpace(operation.RouteReceipt) != "") {
		return "selfdev-rollback-apply-" + operation.DecisionEvent
	}
	return "selfdev-apply-" + operation.DecisionEvent
}

// refuseSettleIfApplyLive refuses (a conflict) when the updater journal shows
// the operation's apply mid-flight or applied: a release may be installed or
// about to be, and the materializer, not a settle, records that. A missing
// journal (nothing started) and a terminal non-applied outcome pass.
func (rt *Runtime) refuseSettleIfApplyLive(operation selfdev.Operation) error {
	if strings.TrimSpace(operation.DecisionEvent) == "" || rt.selfdevUpdaterRoot == "" {
		return nil
	}
	journaled, found, err := updater.ReadJournalOutcome(rt.selfdevUpdaterRoot, selfdevApplyJournalKey(operation))
	if err != nil {
		return fmt.Errorf("updater journal unreadable: %w", err)
	}
	switch {
	case !found:
		return nil
	case !journaled.Terminal:
		return fmt.Errorf("%w: operation %s has an apply mid-flight with the updater", errSelfdevSettleConflict, operation.OperationID)
	case journaled.Result.Outcome == "applied":
		return fmt.Errorf("%w: operation %s has an applied release; the materializer records it", errSelfdevSettleConflict, operation.OperationID)
	}
	return nil
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

// settleSelfDevelopmentOperationFailed moves an open operation to failed,
// recording the settlement on the tape first. It never leaves the head's
// pending transition bound to a failed operation:
//   - if the pending transition is this operation's decision, it is discharged
//     with the same materialization_failed every failed materialization uses,
//     which also returns desired to effective, and the operation moves in the
//     same call (RestoredPriorEffective is honest there because the journal
//     guard above shows no applied release for the decision: the prior
//     effective release is the one still running);
//   - otherwise the row moves, and the head is read again afterwards: a
//     decision that landed between the caller's checks and the transition is
//     discharged then, so the head cannot stay wedged on a failed operation.
func (rt *Runtime) settleSelfDevelopmentOperationFailed(ctx context.Context, operation selfdev.Operation, spec selfdevSettleSpec) (selfdev.Operation, error) {
	if operation.State == selfdev.StateMaterializing || operation.State == selfdev.StateRollbackPending {
		if err := rt.refuseSettleIfApplyLive(operation); err != nil {
			return selfdev.Operation{}, err
		}
	}
	payload, err := computerevent.CanonicalJSON(selfdevOperationSettlement{
		Schema: selfdevSettlementSchema, OperationID: operation.OperationID, FromState: operation.State, ToState: selfdev.StateFailed,
		Act: spec.Act, Actor: spec.Actor, Reason: spec.Reason, IdempotencyKey: spec.Key,
	})
	if err != nil {
		return selfdev.Operation{}, err
	}
	idempotency := selfdevSettleIdempotency(spec.Act, operation.ComputerID, operation.OperationID, spec.Key)
	if _, found, lookupErr := rt.store.EventByIdempotency(ctx, operation.ComputerID, idempotency); lookupErr != nil {
		return selfdev.Operation{}, lookupErr
	} else if !found {
		eventID, eventErr := computerevent.NewEventID()
		if eventErr != nil {
			return selfdev.Operation{}, eventErr
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
		if _, _, eventErr = rt.eventAppender.AppendNewPayload(ctx, event, computerevent.TransitionInput{}, payload, selfdevSettlementMediaType, "owner"); eventErr != nil {
			return selfdev.Operation{}, eventErr
		}
	}
	head, err := rt.store.Head(ctx, operation.ComputerID)
	if err != nil || head == nil {
		return selfdev.Operation{}, fmt.Errorf("settle: event head unavailable")
	}
	owned, err := rt.pendingDecisionOf(ctx, operation, head.PendingTransitionRef)
	if err != nil {
		return selfdev.Operation{}, err
	}
	if owned != "" {
		decided := operation
		decided.DecisionEvent = owned
		if err := rt.refuseSettleIfApplyLive(decided); err != nil {
			return selfdev.Operation{}, err
		}
		if err := rt.recordMaterializationFailed(ctx, decided, updater.ApplyResult{Outcome: spec.Act}, errors.New(spec.TerminalError), operation.State); err != nil {
			return selfdev.Operation{}, settleConflict(err)
		}
		return rt.selfdevOperations.Get(ctx, operation.ComputerID, operation.OperationID)
	}
	settled, err := rt.selfdevOperations.Transition(ctx, operation.ComputerID, operation.OperationID, operation.State, selfdev.StateFailed, func(next *selfdev.Operation) error {
		next.DesiredHead, next.EffectiveHead = head.DesiredEventHead, head.EffectiveEventHead
		next.TerminalError = spec.TerminalError
		return nil
	})
	if err != nil {
		return selfdev.Operation{}, settleConflict(err)
	}
	// A decision may have landed between the checks above and the transition
	// (the decision path appends its event, then moves the row; this move won
	// the row). Its pending transition now belongs to a failed operation.
	after, err := rt.store.Head(ctx, operation.ComputerID)
	if err != nil || after == nil {
		return settled, fmt.Errorf("settle: event head unavailable after the transition")
	}
	if late, lateErr := rt.pendingDecisionOf(ctx, settled, after.PendingTransitionRef); lateErr != nil {
		return settled, lateErr
	} else if late != "" {
		decided := operation
		decided.DecisionEvent = late
		if _, err := rt.appendMaterializationFailedEvent(ctx, decided, updater.ApplyResult{Outcome: spec.Act}); err != nil {
			return settled, err
		}
	}
	return settled, nil
}

func settleConflict(err error) error {
	if errors.Is(err, selfdev.ErrConflict) || errors.Is(err, selfdev.ErrInvalidTransition) {
		return fmt.Errorf("%w: %v", errSelfdevSettleConflict, err)
	}
	return err
}

// settleRollbackPinnedReleaseUnusable closes a rollback (degraded or
// rollback_pending) whose pinned release is gone or fails verification: the
// re-drive can never succeed, and the computer keeps the release it runs.
// The tape tells the two causes apart by the settlement's act.
func (rt *Runtime) settleRollbackPinnedReleaseUnusable(ctx context.Context, operation selfdev.Operation, readErr error) error {
	spec := selfdevSettleSpec{
		Act: selfdevSettleActPinnedUnavailable, Actor: "guest-core:choir-updater", Authority: "guest-core:choir-updater",
		Key: "pinned-release-unavailable", TerminalError: selfdevPinnedReleaseUnavailableCause,
	}
	if errors.Is(readErr, updater.ErrPinnedReleaseCorrupt) {
		spec.Act, spec.Key = selfdevSettleActPinnedCorrupt, "pinned-release-failed-verification"
		spec.TerminalError = strings.TrimPrefix(readErr.Error(), "updater: ")
	}
	spec.Reason = spec.TerminalError
	log.Printf("selfdev materializer: operation %s is %s and its pinned release is unusable (%s); settled failed", operation.OperationID, operation.State, spec.Act)
	_, err := rt.settleSelfDevelopmentOperationFailed(ctx, operation, spec)
	return err
}

// selfDevelopmentAbandonRequest is the owner's abandon act.
type selfDevelopmentAbandonRequest struct {
	OperationID    string `json:"operation_id"`
	Reason         string `json:"reason"`
	IdempotencyKey string `json:"idempotency_key"`
}

// abandonableSelfDevelopmentState reports the open states an owner can
// abandon: every one that installed nothing, degraded (applied with an
// unsettled tail the materializer can no longer finish), and the two in-flight
// states, which abandon settles only when the updater journal shows nothing
// applied or mid-flight (refuseSettleIfApplyLive).
func abandonableSelfDevelopmentState(state string) bool {
	switch state {
	case selfdev.StateRequested, selfdev.StateExecuting, selfdev.StateFrozen, selfdev.StateVerified,
		selfdev.StateAwaitingApproval, selfdev.StateAccepted, selfdev.StateDegraded,
		selfdev.StateMaterializing, selfdev.StateRollbackPending:
		return true
	}
	return false
}

func selfDevelopmentAbandonTerminalError(reason string) string {
	return "abandoned by the owner: " + reason
}

// abandonSelfDevelopmentOperation is the owner's act: close an open operation
// to failed with the owner's reason, recorded on the tape as an owner
// decision. It answers an HTTP status like decideSelfDevelopment.
func (rt *Runtime) abandonSelfDevelopmentOperation(ctx context.Context, ownerID, computerID string, request selfDevelopmentAbandonRequest) (selfdev.Operation, int, error) {
	request.OperationID, request.Reason, request.IdempotencyKey = strings.TrimSpace(request.OperationID), strings.TrimSpace(request.Reason), strings.TrimSpace(request.IdempotencyKey)
	if request.OperationID == "" || request.Reason == "" || request.IdempotencyKey == "" {
		return selfdev.Operation{}, http.StatusBadRequest, errors.New("operation_id, reason and idempotency_key are required")
	}
	if rt.selfdevOperations == nil || rt.eventAppender == nil || rt.store == nil {
		return selfdev.Operation{}, http.StatusServiceUnavailable, errors.New("abandon authority unavailable")
	}
	operation, err := rt.selfdevOperations.Get(ctx, computerID, request.OperationID)
	if err != nil {
		return selfdev.Operation{}, http.StatusNotFound, errors.New("self-development operation not found")
	}
	terminalError := selfDevelopmentAbandonTerminalError(request.Reason)
	idempotency := selfdevSettleIdempotency(selfdevSettleActAbandon, computerID, request.OperationID, request.IdempotencyKey)
	if _, spent, lookupErr := rt.store.EventByIdempotency(ctx, computerID, idempotency); lookupErr != nil {
		return selfdev.Operation{}, http.StatusInternalServerError, errors.New("abandon event lookup failed")
	} else if spent && operation.State == selfdev.StateFailed {
		if operation.TerminalError == terminalError {
			return operation, http.StatusOK, nil
		}
		return selfdev.Operation{}, http.StatusConflict, errors.New("this idempotency key is already spent on an abandon with a different reason")
	}
	if !abandonableSelfDevelopmentState(operation.State) {
		return selfdev.Operation{}, http.StatusConflict, fmt.Errorf("self-development operation %s is %s, not open; there is nothing to abandon", operation.OperationID, operation.State)
	}
	if operation.State == selfdev.StateMaterializing || operation.State == selfdev.StateRollbackPending {
		// In flight is decided by the updater's journal, not the row, and a
		// materializer pass holds this mutex across its whole apply: if it
		// is busy, an apply may be mid-flight, so refuse rather than race it.
		// A pass that only retries a failed read is short, so wait a moment
		// for it; one inside a long apply outlasts the wait and is refused.
		locked := false
		for deadline := time.Now().Add(3 * time.Second); !locked && time.Now().Before(deadline) && ctx.Err() == nil; {
			if locked = rt.selfdevMaterializeMu.TryLock(); !locked {
				time.Sleep(20 * time.Millisecond)
			}
		}
		if !locked {
			return selfdev.Operation{}, http.StatusConflict, fmt.Errorf("self-development operation %s is %s and the materializer is working; try again when it is idle", operation.OperationID, operation.State)
		}
		defer rt.selfdevMaterializeMu.Unlock()
		// Re-read under the lock: the pass that just finished may have settled it.
		if operation, err = rt.selfdevOperations.Get(ctx, computerID, request.OperationID); err != nil {
			return selfdev.Operation{}, http.StatusNotFound, errors.New("self-development operation not found")
		}
		if !abandonableSelfDevelopmentState(operation.State) {
			return selfdev.Operation{}, http.StatusConflict, fmt.Errorf("self-development operation %s is %s, not open; there is nothing to abandon", operation.OperationID, operation.State)
		}
	}
	if operation.State == selfdev.StateAwaitingApproval {
		// A decision already on the tape is settling; abandoning would fight it.
		if _, decided, decisionErr := rt.store.FinalizedDecisionForOperation(ctx, computerID, operation.OperationID, operation.TrajectoryID, operation.CapsuleID); decisionErr != nil {
			return selfdev.Operation{}, http.StatusInternalServerError, errors.New("decision lookup failed")
		} else if decided {
			return selfdev.Operation{}, http.StatusConflict, errors.New("a decision is already recorded for this operation; let it settle")
		}
	}
	settled, err := rt.settleSelfDevelopmentOperationFailed(ctx, operation, selfdevSettleSpec{
		Act: selfdevSettleActAbandon, Actor: "owner:" + ownerID, Authority: selfDevelopmentDecisionAuthority(ownerID, ""),
		Reason: request.Reason, Key: request.IdempotencyKey, TerminalError: terminalError,
	})
	if err != nil {
		if errors.Is(err, errSelfdevSettleConflict) {
			return selfdev.Operation{}, http.StatusConflict, err
		}
		return selfdev.Operation{}, http.StatusInternalServerError, err
	}
	rt.notifyTextureOfOwnerAct(ctx, ownerID, settled, types.OwnerActAbandon, request.Reason)
	return settled, http.StatusOK, nil
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
