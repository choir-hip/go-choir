package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"slices"
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
//   - the materializer, for a degraded rollback whose pinned release is gone
//     (a fact, not a decision);
//   - the owner's `abandon` act, for an open operation that installed nothing
//     or is degraded (a decision, with the owner's reason).
// Both put the settlement on the tape first, as a recovery_recorded event, so
// the state row is never the only record.

const (
	selfdevSettlementMediaType = "application/vnd.choir.selfdev-operation-settlement+json"
	selfdevSettlementSchema    = "choir.selfdev.operation-settlement.v1"

	selfdevSettleActPinnedUnavailable = "pinned_release_unavailable"
	selfdevSettleActAbandon           = "abandon"

	// selfdevPinnedReleaseUnavailableCause is the terminal error of a degraded
	// rollback whose pinned release is gone.
	selfdevPinnedReleaseUnavailableCause = "pinned release unavailable; the computer stays on its current release"
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

// settleSelfDevelopmentOperationFailed moves an open operation to failed with
// terminalError, recording the settlement on the tape first. When the
// operation's own decision is still the head's pending transition (an accepted
// operation, or a rollback that never applied), the pending transition is
// discharged the way every other failed materialization discharges it
// (recordMaterializationFailed), which also returns desired to effective.
// Otherwise the tape holds nothing pending for it and the row alone moves.
func (rt *Runtime) settleSelfDevelopmentOperationFailed(ctx context.Context, operation selfdev.Operation, act, actor, authority, reason, key, terminalError string) (selfdev.Operation, error) {
	payload, err := computerevent.CanonicalJSON(selfdevOperationSettlement{
		Schema: selfdevSettlementSchema, OperationID: operation.OperationID, FromState: operation.State, ToState: selfdev.StateFailed,
		Act: act, Actor: actor, Reason: reason, IdempotencyKey: key,
	})
	if err != nil {
		return selfdev.Operation{}, err
	}
	idempotency := "selfdev-settle-" + act + "-" + computerevent.DigestBytes([]byte(operation.ComputerID+"\x00"+operation.OperationID+"\x00"+key))
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
			ActorProfile:  agentprofile.Management, AuthorityRef: authority, PrivacyClass: "owner",
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
	if operation.DecisionEvent != "" && head.PendingTransitionRef == operation.DecisionEvent {
		if err := rt.recordMaterializationFailed(ctx, operation, updater.ApplyResult{Outcome: act}, errors.New(terminalError), operation.State); err != nil {
			return selfdev.Operation{}, err
		}
		return rt.selfdevOperations.Get(ctx, operation.ComputerID, operation.OperationID)
	}
	return rt.selfdevOperations.Transition(ctx, operation.ComputerID, operation.OperationID, operation.State, selfdev.StateFailed, func(next *selfdev.Operation) error {
		next.DesiredHead, next.EffectiveHead = head.DesiredEventHead, head.EffectiveEventHead
		next.TerminalError = terminalError
		return nil
	})
}

// settleDegradedRollbackPinnedReleaseUnavailable closes a degraded rollback
// whose pinned release can no longer be read: the re-drive can never succeed,
// and the computer already runs a release, so the rollback is over and the
// computer stays on it.
func (rt *Runtime) settleDegradedRollbackPinnedReleaseUnavailable(ctx context.Context, operation selfdev.Operation) error {
	log.Printf("selfdev materializer: operation %s is degraded and its pinned release is unavailable; settled failed", operation.OperationID)
	_, err := rt.settleSelfDevelopmentOperationFailed(ctx, operation, selfdevSettleActPinnedUnavailable, "guest-core:choir-updater", "guest-core:choir-updater",
		selfdevPinnedReleaseUnavailableCause, "pinned-release-unavailable", selfdevPinnedReleaseUnavailableCause)
	return err
}

// selfDevelopmentAbandonRequest is the owner's abandon act.
type selfDevelopmentAbandonRequest struct {
	OperationID    string `json:"operation_id"`
	Reason         string `json:"reason"`
	IdempotencyKey string `json:"idempotency_key"`
}

// abandonableSelfDevelopmentStates are the open states an owner can abandon:
// every one installed nothing, plus degraded (applied with an unsettled tail,
// which the materializer can no longer finish). materializing and
// rollback_pending are in flight with the updater and are never abandoned.
var abandonableSelfDevelopmentStates = []string{
	selfdev.StateRequested, selfdev.StateExecuting, selfdev.StateFrozen, selfdev.StateVerified,
	selfdev.StateAwaitingApproval, selfdev.StateAccepted, selfdev.StateDegraded,
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
	idempotency := "selfdev-settle-" + selfdevSettleActAbandon + "-" + computerevent.DigestBytes([]byte(computerID+"\x00"+request.OperationID+"\x00"+request.IdempotencyKey))
	if _, replay, lookupErr := rt.store.EventByIdempotency(ctx, computerID, idempotency); lookupErr != nil {
		return selfdev.Operation{}, http.StatusInternalServerError, errors.New("abandon event lookup failed")
	} else if replay && operation.State == selfdev.StateFailed && operation.TerminalError == terminalError {
		return operation, http.StatusOK, nil
	}
	switch operation.State {
	case selfdev.StateMaterializing, selfdev.StateRollbackPending:
		return selfdev.Operation{}, http.StatusConflict, fmt.Errorf("self-development operation %s is %s, in flight with the updater; it cannot be abandoned", operation.OperationID, operation.State)
	}
	if !slices.Contains(abandonableSelfDevelopmentStates, operation.State) {
		return selfdev.Operation{}, http.StatusConflict, fmt.Errorf("self-development operation %s is %s, not open; there is nothing to abandon", operation.OperationID, operation.State)
	}
	if operation.State == selfdev.StateAwaitingApproval {
		// A decision already on the tape is settling; abandoning would fight it.
		if _, decided, decisionErr := rt.store.FinalizedDecisionForOperation(ctx, computerID, operation.OperationID, operation.TrajectoryID, operation.CapsuleID); decisionErr != nil || decided {
			return selfdev.Operation{}, http.StatusConflict, errors.New("a decision is already recorded for this operation; let it settle")
		}
	}
	settled, err := rt.settleSelfDevelopmentOperationFailed(ctx, operation, selfdevSettleActAbandon, "owner:"+ownerID, selfDevelopmentDecisionAuthority(ownerID, ""),
		request.Reason, request.IdempotencyKey, terminalError)
	if err != nil {
		return selfdev.Operation{}, http.StatusConflict, err
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
