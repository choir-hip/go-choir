package agentcore

// POST /api/computers/<id>/self-development/operations/<X>/revert
// (docs/design/selfdev-apply-and-revert-2026-11-06.md, 3.5, R3). An owner act
// (the proxy's gate admits it only for the owner's session or an owner-acts
// key). The guest assembles the rollback bindings itself from its current
// head, so a stale tap cannot fail on bindings a browser read earlier.
//
// Reverting X restores the state before X. X's prior effective head is the
// one its accept event expected. If that head belongs to an applied or
// rolled-back operation with full evidence, the revert is the existing
// rollback to it. Otherwise (a genesis or platform-update checkpoint, or an
// earlier base restore) the booted base release is restored with no
// self-development overlay (selfdev_base_restore.go).
//
// Idempotency key: revert:<X>:<latest landed operation>. A double tap is one
// act; a revert after later applies is a new act.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/routeledger"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
)

func (h *APIHandler) revertSelfDevelopmentOperation(w http.ResponseWriter, r *http.Request, ownerID, computerID, operationID string) {
	rt := h.rt
	if rt.eventAppender == nil || rt.store == nil || rt.selfdevRoute == nil || rt.selfdevOperations == nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "revert authority unavailable"})
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<16))
	if err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid revert request"})
		return
	}
	if trimmed := strings.TrimSpace(string(body)); trimmed != "" {
		decoder := json.NewDecoder(strings.NewReader(trimmed))
		decoder.DisallowUnknownFields()
		var empty struct{}
		if err := decoder.Decode(&empty); err != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "a revert request takes no fields"})
			return
		}
	}
	ctx := r.Context()
	operationID = strings.TrimSpace(operationID)
	reverted, err := rt.selfdevOperations.Get(ctx, computerID, operationID)
	if err != nil {
		if selfdevOperationMissing(err) {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "self-development operation not found"})
			return
		}
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "revert lookup failed"})
		return
	}
	latest, err := h.latestLandedOperation(ctx, computerID)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "revert lookup failed"})
		return
	}
	since := "none"
	if latest != nil {
		since = latest.OperationID
	}
	key := "revert:" + operationID + ":" + since
	// A double request with the same key is one act.
	if existing, found, err := rt.selfdevOperations.GetByIdempotency(ctx, computerID, key); err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "revert lookup failed"})
		return
	} else if found {
		if existing.State != selfdev.StateRollbackPending && existing.State != selfdev.StateRolledBack {
			writeAPIJSON(w, http.StatusConflict, apiError{Error: "revert idempotency binding changed"})
			return
		}
		writeAPIJSON(w, http.StatusOK, existing)
		return
	}
	if reverted.State != selfdev.StateApplied || strings.HasPrefix(reverted.OperationID, "rollback-") || strings.HasPrefix(reverted.OperationID, "genesis-") {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "operation " + operationID + " is " + reverted.State + ": only an applied self-development operation is reverted"})
		return
	}
	accepted, found, err := rt.store.EventByDigest(ctx, computerID, reverted.DecisionEvent)
	if err != nil || !found || !computerevent.IsSHA256(accepted.ExpectedEffectiveEventHead) || !computerevent.IsSHA256(accepted.ExpectedEffectiveStateCommitment) {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "the state before operation " + operationID + " is not recorded on the tape"})
		return
	}
	priorHead, priorCommitment := accepted.ExpectedEffectiveEventHead, accepted.ExpectedEffectiveStateCommitment
	// The latest landed operation already restored this state and nothing has
	// landed since: a second tap after the revert settled.
	if latest != nil && latest.State == selfdev.StateRolledBack && latest.BaseHead == priorHead {
		writeAPIJSON(w, http.StatusOK, *latest)
		return
	}
	head, err := rt.store.Head(ctx, computerID)
	if err != nil || head == nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "event head unavailable"})
		return
	}
	if head.PendingTransitionRef != "" {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "a transition is pending on the event head; wait for it to settle before reverting"})
		return
	}
	if head.EffectiveEventHead == priorHead {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "the computer is already in the state before operation " + operationID})
		return
	}
	var generation uint64
	if slotID, slotErr := routeledger.RouteSlotID(rt.selfdevRouteOwnerID, rt.selfdevRouteDesktopID); slotErr == nil {
		if route, routeErr := rt.selfdevRoute.ResolveComputerVersionRoute(ctx, slotID); routeErr == nil {
			generation = route.Slot.Generation
		}
	}
	prior, priorErr := rt.selfdevOperations.GetByEffectiveHead(ctx, computerID, priorHead)
	if priorErr != nil && !selfdevOperationMissing(priorErr) {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "revert lookup failed"})
		return
	}
	if priorErr == nil && revertTargetHasEvidence(prior) {
		if generation == 0 {
			writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "rollback route authority unavailable"})
			return
		}
		h.openSelfDevelopmentRollback(w, ctx, ownerID, computerID, selfDevelopmentRollbackRequest{
			ExpectedDesiredHead: head.DesiredEventHead, CurrentAppliedHead: head.EffectiveEventHead,
			ToAppliedHead: prior.EffectiveHead, PriorMaterialization: prior.MaterializationReceipt,
			PriorCheckpoint: prior.CheckpointRef, ExpectedRouteGeneration: generation, IdempotencyKey: key,
		})
		return
	}
	// The prior state is a checkpoint with no operation evidence.
	if priorErr != nil {
		priorEvent, ok, err := rt.store.EventByDigest(ctx, computerID, priorHead)
		if err != nil || !ok || (priorEvent.EventKind != computerevent.EventGenesisImported && priorEvent.EventKind != computerevent.EventMaterializationApplied) {
			writeAPIJSON(w, http.StatusConflict, apiError{Error: "the state before operation " + operationID + " is not a restorable checkpoint"})
			return
		}
	}
	h.openBaseRestore(w, ctx, ownerID, computerID, reverted, key, priorHead, priorCommitment, generation)
}

// selfdevOperationMissing: the store's single-row reads answer sql.ErrNoRows.
func selfdevOperationMissing(err error) bool {
	return errors.Is(err, sql.ErrNoRows) || errors.Is(err, selfdev.ErrOperationNotFound)
}

// revertTargetHasEvidence reports whether an operation can be the target of
// the existing rollback: an applied or rolled-back operation that carries its
// verifier, release, checkpoint and route evidence. A genesis baseline and a
// base restore carry none.
func revertTargetHasEvidence(op selfdev.Operation) bool {
	return !strings.HasPrefix(op.OperationID, "genesis-") && !selfdev.IsBaseRestore(op) &&
		(op.State == selfdev.StateApplied || op.State == selfdev.StateRolledBack) &&
		computerevent.IsSHA256(op.EffectiveHead) && computerevent.IsSHA256(op.BundleDigest) && computerevent.IsSHA256(op.ReleaseDigest) &&
		op.CodeRef != "" && op.ArtifactProgramRef != "" && len(op.VerifierRefs) > 0 && computerevent.IsSHA256(op.VerifierRefs[0]) &&
		op.MaterializationReceipt != "" && op.CheckpointRef != "" && op.RouteReceipt != ""
}

// latestLandedOperation is the newest-created operation whose head landed or
// whose attempt ended (the CLI's anchor for a rollback intent).
func (h *APIHandler) latestLandedOperation(ctx context.Context, computerID string) (*selfdev.Operation, error) {
	landed, err := h.rt.selfdevOperations.ListByStates(ctx, computerID, selfdev.StateApplied, selfdev.StateDegraded, selfdev.StateFailed, selfdev.StateRolledBack)
	if err != nil {
		return nil, err
	}
	var best *selfdev.Operation
	var bestAt time.Time
	for index := range landed {
		at, parseErr := time.Parse(time.RFC3339Nano, landed[index].CreatedAt)
		if parseErr != nil {
			continue
		}
		if best == nil || at.After(bestAt) || (at.Equal(bestAt) && landed[index].OperationID > best.OperationID) {
			best, bestAt = &landed[index], at
		}
	}
	return best, nil
}

// openBaseRestore opens the revert whose target is the booted base release.
func (h *APIHandler) openBaseRestore(w http.ResponseWriter, ctx context.Context, ownerID, computerID string, reverted selfdev.Operation, key, priorHead, priorCommitment string, generation uint64) {
	rt := h.rt
	baselineRoot := rt.packageMigrationBaselineRoot()
	if baselineRoot == "" {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "the booted base release is unavailable"})
		return
	}
	target, err := rt.servingBaselineManifest(ctx, computerID, baselineRoot)
	if err != nil || !computerevent.IsSHA256(target.ContentDigest) {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "the booted base release is unavailable"})
		return
	}
	commitmentBytes, err := computerevent.CanonicalJSON(map[string]string{
		"kind": "base_restore", "reverted_operation": reverted.OperationID, "target_event_head": priorHead,
		"base_release": target.ContentDigest, "idempotency_key": key,
	})
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "invalid revert request"})
		return
	}
	commitment := computerevent.DigestBytes(commitmentBytes)
	eventIdempotency := "selfdev-rollback-requested-" + key
	existingEvent, eventFound, err := rt.store.EventByIdempotency(ctx, computerID, eventIdempotency)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "revert event lookup failed"})
		return
	}
	if eventFound {
		if existingEvent.EventKind != computerevent.EventRollbackRequested || existingEvent.DecisionRef != priorHead {
			writeAPIJSON(w, http.StatusConflict, apiError{Error: "revert event binding changed"})
			return
		}
	} else {
		event, err := rollbackRequestedEvent(computerID, ownerID, eventIdempotency, commitment, reverted.TrajectoryID, reverted.CapsuleID, target.ContentDigest, priorHead)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "revert event identity unavailable"})
			return
		}
		if _, err := rt.eventAppender.AppendNew(ctx, event, computerevent.TransitionInput{TargetStateCommitment: priorCommitment}, nil); err != nil {
			writeAPIJSON(w, http.StatusConflict, apiError{Error: err.Error()})
			return
		}
	}
	next, err := rt.store.Head(ctx, computerID)
	if err != nil || next == nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "revert projection unavailable"})
		return
	}
	operation, err := rt.selfdevOperations.StartBaseRestore(ctx, selfdev.BaseRestoreStartRequest{
		ComputerID: computerID, IdempotencyKey: key, RequestCommitment: commitment,
		RollbackEvent: next.CanonicalEventHead, DecisionActor: ownerID, CurrentDesired: next.DesiredEventHead,
		CurrentEffective: next.EffectiveEventHead, PriorEffectiveHead: priorHead, RevertedOperation: reverted.OperationID,
		BaseReleaseDigest: target.ContentDigest,
		CodeRef:           target.CodeRef, ArtifactProgramRef: target.ArtifactProgramRef, RouteGeneration: generation,
	})
	if err != nil {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: err.Error()})
		return
	}
	rt.triggerSelfDevelopmentReconcile()
	writeAPIJSON(w, http.StatusCreated, operation)
}
