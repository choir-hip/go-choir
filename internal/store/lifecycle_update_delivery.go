package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Consume-at-commit delivery reconciliation. Dispatch claims pending producer
// reports by binding DeliveredToRunID to the desk run being armed; a run that
// terminates without a committed turn leaves a stranded binding the next
// reconcile releases and counts. Consumption itself happens inside
// ApplyTextureTurn (textureTurnPendingInbound), never here — this command only
// moves the delivery bookkeeping so at-least-once is provable and poison
// packets die against the attempt cap instead of respawning the desk forever.
//
// Design receipt: docs/problems/texture-desk-activation-contract-respawn-loop-2026-09-28.md
// — bind at dispatch, consume at commit; bounded redelivery; owner-head armed
// independently (it is not a packet and never appears in Items).

// ComputeReconcileUpdateDeliveryDigest digests the whole request — Items are
// part of command identity, so a replayed request replays the same batch.
func ComputeReconcileUpdateDeliveryDigest(req types.ReconcileUpdateDeliveryRequest) (string, error) {
	req.OwnerID, req.ComputerID, req.CommandDigest = "", "", ""
	return lifecycleDigest(req)
}

func (s *Store) ReconcileUpdateDelivery(ctx context.Context, req types.ReconcileUpdateDeliveryRequest) (types.LifecycleResult, error) {
	ownerID, computerID, err := normalizeLifecycleScope(req.OwnerID, req.ComputerID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	req.OwnerID, req.ComputerID = ownerID, computerID
	req.CommandID = strings.TrimSpace(req.CommandID)
	if req.CommandID == "" || strings.TrimSpace(req.TrajectoryID) == "" || strings.TrimSpace(req.TargetAgentID) == "" {
		return types.LifecycleResult{}, fmt.Errorf("reconcile update delivery: command, trajectory, and target agent are required")
	}
	computedDigest, digestErr := ComputeReconcileUpdateDeliveryDigest(req)
	if err := requireLifecycleDigest(req.CommandDigest, computedDigest, digestErr); err != nil {
		return types.LifecycleResult{}, err
	}
	if replay, found, replayErr := s.replayLifecycleCommand(ctx, ownerID, computerID, req.CommandID, req.CommandDigest); found || replayErr != nil {
		return replay, replayErr
	}
	if req.MaxAttempts <= 0 {
		req.MaxAttempts = 3
	}
	graph := s.ogStore
	if graph == nil {
		return types.LifecycleResult{}, fmt.Errorf("reconcile update delivery: object graph not initialized")
	}
	trajectoryID := strings.TrimSpace(req.TrajectoryID)
	trajectoryObj, trajectory, err := s.lifecycleTrajectoryObject(ctx, ownerID, computerID, trajectoryID)
	if err != nil {
		return types.LifecycleResult{}, fmt.Errorf("reconcile update delivery trajectory %s: %w", trajectoryID, err)
	}

	firstNonEmptyStore := func(a, b string) string {
		if strings.TrimSpace(a) != "" {
			return a
		}
		return b
	}
	now := time.Now().UTC()
	var conditions []objectgraph.ObjectCondition
	var objects []objectgraph.Object
	var events []types.LifecycleEvent
	var eventObjs []objectgraph.Object
	conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: trajectoryObj.CanonicalID, Exists: true, ExpectedContentHash: trajectoryObj.ContentHash})
	seq := trajectory.ReducerSeq
	eventSeq := 0
	emit := func(kind types.LifecycleEventKind, update types.CoagentSourcePacket, reason string) error {
		eventSeq++
		event := types.LifecycleEvent{
			EventID:        req.CommandID + ":" + fmt.Sprintf("%d", eventSeq),
			OwnerID:        ownerID,
			ComputerID:     computerID,
			TrajectoryID:   trajectoryID,
			WorkItemID:     firstNonEmptyStore(update.ProducerWorkItemID, update.TargetWorkItemID),
			UpdateID:       update.UpdateID,
			RunID:          strings.TrimSpace(update.DeliveredToRunID),
			AgentID:        update.TargetAgentID,
			Kind:           kind,
			ReducerVersion: types.LifecycleReducerVersion,
			ReducerSeq:     seq,
			CommandID:      req.CommandID,
			CommandDigest:  req.CommandDigest,
			Reason:         reason,
			CreatedAt:      now,
		}
		eventObj, err := lifecycleObject(ogKindLifecycleEvent, ownerID, computerID, event.EventID, event, lifecycleMetadata("event_id", event.EventID, computerID, trajectoryID, seq), now, now)
		if err != nil {
			return err
		}
		conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: eventObj.CanonicalID})
		objects = append(objects, eventObj)
		events = append(events, event)
		eventObjs = append(eventObjs, eventObj)
		return nil
	}

	for _, item := range req.Items {
		updateID := strings.TrimSpace(item.UpdateID)
		if updateID == "" {
			return types.LifecycleResult{}, fmt.Errorf("reconcile update delivery: item update_id is required")
		}
		key := trajectoryID + "\x00" + strings.TrimSpace(req.TargetAgentID) + "\x00" + strings.TrimSpace(item.ProducerAgentID) + "\x00" + strings.TrimSpace(item.ProducerUpdateID)
		updateObj, update, getErr := s.textureTurnUpdateObject(ctx, ownerID, computerID, key)
		if getErr != nil {
			return types.LifecycleResult{}, getErr
		}
		if update.UpdateID != updateID || (update.Direction != types.LifecyclePacketDirectionProducerReport && update.Direction != types.LifecyclePacketDirectionControl && update.Direction != types.LifecyclePacketDirectionDirective) ||
			update.TrajectoryID != trajectoryID || update.TargetAgentID != req.TargetAgentID ||
			update.Disposition != types.UpdatePending ||
			strings.TrimSpace(update.DeliveredToRunID) != strings.TrimSpace(item.ExpectedRunID) ||
			update.LifecycleVersion != item.ExpectedLifecycleVersion {
			return types.LifecycleResult{}, ErrLifecycleCommandConflict
		}
		seq++
		exhausted := item.Exhaust || update.DeliveryAttempts+1 > req.MaxAttempts
		if exhausted {
			update.Disposition = types.UpdateDelivered
			update.DispositionReason = "delivery_attempts_exhausted"
			update.DispositionRef = req.CommandID
			update.DeliveredToRunID = ""
			update.DeliveredAt = nil
		} else if strings.TrimSpace(req.TargetRunID) == "" {
			// Pure unbind (a dead run claim reclaimed): the packet returns to
			// the pending set, so the claim timestamp must clear too. Leaving
			// DeliveredAt set reads as still-claimed to the pending scan
			// (lifecycle.go:1955 filters DeliveredAt==nil) while the bound scan
			// sees an empty DeliveredToRunID — the packet falls between both and
			// is stranded forever. DeliveryAttempts++ is kept: a dead-claim
			// reclaim is the only bound on strand cycles, so it stays the budget.
			update.DeliveredToRunID = ""
			update.DeliveredAt = nil
			update.DeliveryAttempts++
		} else {
			update.DeliveredToRunID = strings.TrimSpace(req.TargetRunID)
			update.DeliveredAt = &now
			update.DeliveryAttempts++
		}
		update.LifecycleVersion++
		update.ReducerSeq = seq
		meta := lifecycleMetadata("update_id", update.UpdateID, computerID, trajectoryID, seq)
		meta["producer_update_id"] = update.ProducerUpdateID
		meta["target_agent_id"] = update.TargetAgentID
		updatedObj, buildErr := lifecycleObject(ogKindWorkerUpdate, ownerID, computerID, key, update, meta, updateObj.CreatedAt, now)
		if buildErr != nil {
			return types.LifecycleResult{}, buildErr
		}
		conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: updateObj.CanonicalID, Exists: true, ExpectedContentHash: updateObj.ContentHash})
		objects = append(objects, updatedObj)
		if exhausted {
			failKind := types.LifecycleTextureActivationFailed
			if update.Direction == types.LifecyclePacketDirectionControl || update.Direction == types.LifecyclePacketDirectionDirective {
				failKind = types.LifecycleControlActivationFailed
			}
			if err := emit(failKind, update, "delivery_attempts_exhausted"); err != nil {
				return types.LifecycleResult{}, err
			}
		} else if strings.TrimSpace(req.TargetRunID) == "" {
			// Unbind re-queues the packet to pending, not a bind — emit the
			// queue event so the audit trail records release-to-pending, never
			// a false bound_to_activation.
			requeueKind := types.LifecycleUpdateQueued
			if update.Direction == types.LifecyclePacketDirectionControl || update.Direction == types.LifecyclePacketDirectionDirective {
				requeueKind = types.LifecycleControlQueued
			}
			if err := emit(requeueKind, update, "released_to_pending"); err != nil {
				return types.LifecycleResult{}, err
			}
		} else {
			deliverKind := types.LifecycleUpdateDelivered
			if update.Direction == types.LifecyclePacketDirectionControl || update.Direction == types.LifecyclePacketDirectionDirective {
				deliverKind = types.LifecycleControlDelivered
			}
			if err := emit(deliverKind, update, "bound_to_activation"); err != nil {
				return types.LifecycleResult{}, err
			}
		}
	}

	// BreakerReason emits one durable activation_failed marker for the
	// consecutive-no-commit breaker; command replay makes it idempotent.
	if reason := strings.TrimSpace(req.BreakerReason); reason != "" {
		seq++
		if err := emit(types.LifecycleTextureActivationFailed, types.CoagentSourcePacket{}, reason); err != nil {
			return types.LifecycleResult{}, err
		}
	}

	trajectory.ReducerSeq = seq
	trajectory.LifecycleVersion++
	trajectory.UpdatedAt = now
	updatedTrajectoryObj, err := lifecycleObject(ogKindTrajectory, ownerID, computerID, trajectory.TrajectoryID, trajectory, lifecycleMetadata("trajectory_id", trajectory.TrajectoryID, computerID, trajectory.TrajectoryID, seq), trajectoryObj.CreatedAt, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	objects = append(objects, updatedTrajectoryObj)

	receipt, receiptObj, err := s.lifecycleTransitionReceipt(now, ownerID, computerID, trajectoryID, req.CommandID, req.CommandDigest, types.LifecycleReconcileUpdateDelivery, seq, eventObjs)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: receiptObj.CanonicalID})
	objects = append(objects, receiptObj)

	result := types.LifecycleResult{Receipt: receipt, Trajectory: trajectory, Events: events}
	return s.commitLifecycleTransition(ctx, ownerID, computerID, req.CommandID, req.CommandDigest, conditions, objects, result)
}
