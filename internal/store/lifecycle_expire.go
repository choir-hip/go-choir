package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Stale-packet expiry: the SA1 wake-outbox storm fix.
//
// A pending worker-update packet whose obligation can never deliver (settled
// target work item, terminated trajectory, missing directive record) must
// reach a terminal disposition rather than re-arm a wake on every boot. This
// command is that terminal fate: a recorded act (command receipt + typed
// update_expired event + CAS'd packet) so the discharge is evidence, not a
// silent mutation. The deterministic wake row for the packet is marked
// projected in the same commit when supplied, so the discharged obligation
// leaves the outbox drain atomically.

// ComputeExpireStaleLifecyclePacketDigest digests the canonical request.
func ComputeExpireStaleLifecyclePacketDigest(req types.ExpireStaleLifecyclePacketRequest) (string, error) {
	req.OwnerID, req.ComputerID, req.CommandDigest = "", "", ""
	return lifecycleDigest(req)
}

// ExpireStaleLifecyclePacket terminalizes one stale pending packet. It is a
// CAS command, not a policy check: the caller proves staleness against
// canonical state, then this command verifies the packet is still the exact
// pending version the caller saw before discharging it. A packet that
// re-pends, binds, or consumes between proof and command fails the CAS and
// returns ErrLifecycleCommandConflict.
func (s *Store) ExpireStaleLifecyclePacket(ctx context.Context, req types.ExpireStaleLifecyclePacketRequest) (types.LifecycleResult, error) {
	ownerID, computerID, err := normalizeLifecycleScope(req.OwnerID, req.ComputerID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	req.OwnerID, req.ComputerID = ownerID, computerID
	req.CommandID = strings.TrimSpace(req.CommandID)
	updateID := strings.TrimSpace(req.UpdateID)
	if req.CommandID == "" || updateID == "" || strings.TrimSpace(req.TargetAgentID) == "" || strings.TrimSpace(req.StaleReason) == "" {
		return types.LifecycleResult{}, fmt.Errorf("expire stale lifecycle packet: command, update, target agent, and stale reason are required")
	}
	computedDigest, digestErr := ComputeExpireStaleLifecyclePacketDigest(req)
	if err := requireLifecycleDigest(req.CommandDigest, computedDigest, digestErr); err != nil {
		return types.LifecycleResult{}, err
	}
	if replay, found, replayErr := s.replayLifecycleCommand(ctx, ownerID, computerID, req.CommandID, req.CommandDigest); found || replayErr != nil {
		return replay, replayErr
	}
	if s.ogStore == nil {
		return types.LifecycleResult{}, fmt.Errorf("expire stale lifecycle packet: object graph not initialized")
	}
	trajectoryID := strings.TrimSpace(req.TrajectoryID)
	key := trajectoryID + "\x00" + strings.TrimSpace(req.TargetAgentID) + "\x00" + strings.TrimSpace(req.ProducerAgentID) + "\x00" + strings.TrimSpace(req.ProducerUpdateID)
	updateObj, update, err := s.textureTurnUpdateObject(ctx, ownerID, computerID, key)
	if err != nil {
		return types.LifecycleResult{}, fmt.Errorf("expire stale lifecycle packet %s: %w", updateID, err)
	}
	if update.UpdateID != updateID ||
		update.Disposition != types.UpdatePending ||
		update.LifecycleVersion != req.ExpectedLifecycleVersion ||
		strings.TrimSpace(update.DeliveredToRunID) != strings.TrimSpace(req.ExpectedRunID) ||
		update.TargetAgentID != strings.TrimSpace(req.TargetAgentID) ||
		strings.TrimSpace(update.TrajectoryID) != trajectoryID {
		return types.LifecycleResult{}, ErrLifecycleCommandConflict
	}

	now := time.Now().UTC()
	// Trajectory-bound packets sequence their event on the trajectory's own
	// reducer stream; computer-scoped directives have no trajectory authority,
	// so the packet's existing ReducerSeq is the only consistent seq.
	seq := update.ReducerSeq + 1
	var trajectoryObj objectgraph.Object
	var trajectory types.TrajectoryRecord
	trajectoryBound := trajectoryID != ""
	if trajectoryBound {
		trajectoryObj, trajectory, err = s.lifecycleTrajectoryObject(ctx, ownerID, computerID, trajectoryID)
		if err != nil {
			return types.LifecycleResult{}, fmt.Errorf("expire stale lifecycle packet trajectory %s: %w", trajectoryID, err)
		}
		seq = trajectory.ReducerSeq + 1
	}

	var conditions []objectgraph.ObjectCondition
	var objects []objectgraph.Object
	var eventObjs []objectgraph.Object
	var events []types.LifecycleEvent
	if trajectoryBound {
		conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: trajectoryObj.CanonicalID, Exists: true, ExpectedContentHash: trajectoryObj.ContentHash})
	}

	update.Disposition = types.UpdateCancelled
	update.DispositionReason = "stale_obligation_expired"
	update.DispositionRef = req.CommandID
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

	event := types.LifecycleEvent{
		EventID:        req.CommandID + ":1",
		OwnerID:        ownerID,
		ComputerID:     computerID,
		TrajectoryID:   trajectoryID,
		WorkItemID:     expireWorkItemID(update),
		UpdateID:       update.UpdateID,
		AgentID:        update.TargetAgentID,
		Kind:           types.LifecycleUpdateExpired,
		ReducerVersion: types.LifecycleReducerVersion,
		ReducerSeq:     seq,
		CommandID:      req.CommandID,
		CommandDigest:  req.CommandDigest,
		Reason:         strings.TrimSpace(req.StaleReason),
		CreatedAt:      now,
	}
	eventObj, err := lifecycleObject(ogKindLifecycleEvent, ownerID, computerID, event.EventID, event,
		lifecycleMetadata("event_id", event.EventID, computerID, trajectoryID, seq), now, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: eventObj.CanonicalID})
	objects = append(objects, eventObj)
	eventObjs = append(eventObjs, eventObj)
	events = append(events, event)

	// Mark the packet's deterministic wake row projected in the same commit so
	// the discharged obligation cannot drain once more after the write.
	if wakeID := strings.TrimSpace(req.WakeCanonicalID); wakeID != "" {
		wakeObj, wakeErr := s.ogStore.GetObject(ctx, wakeID)
		switch {
		case wakeErr == nil && wakeObj.ObjectKind == ogKindActorWakeOutbox:
			var wakeMeta map[string]any
			if json.Unmarshal(wakeObj.Metadata, &wakeMeta) == nil {
				if projected, _ := wakeMeta["projected"].(bool); !projected {
					wakeMeta["projected"] = true
					rawMeta, normErr := objectgraph.NormalizeMetadata(wakeMeta)
					if normErr == nil {
						marked := wakeObj
						marked.Metadata = rawMeta
						marked.UpdatedAt = now
						marked.ContentHash = objectgraph.ContentHash(marked.ObjectKind, marked.Body, marked.Metadata)
						conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: wakeObj.CanonicalID, Exists: true, ExpectedContentHash: wakeObj.ContentHash})
						objects = append(objects, marked)
					}
				}
			}
		case wakeErr != nil && wakeErr != objectgraph.ErrNotFound:
			return types.LifecycleResult{}, fmt.Errorf("expire stale lifecycle packet %s: read wake row: %w", updateID, wakeErr)
		}
	}

	if trajectoryBound {
		trajectory.ReducerSeq = seq
		trajectory.LifecycleVersion++
		trajectory.UpdatedAt = now
		updatedTrajectoryObj, trajErr := lifecycleObject(ogKindTrajectory, ownerID, computerID, trajectory.TrajectoryID, trajectory, lifecycleMetadata("trajectory_id", trajectory.TrajectoryID, computerID, trajectory.TrajectoryID, seq), trajectoryObj.CreatedAt, now)
		if trajErr != nil {
			return types.LifecycleResult{}, trajErr
		}
		objects = append(objects, updatedTrajectoryObj)
	}

	receipt, receiptObj, err := s.lifecycleTransitionReceipt(now, ownerID, computerID, trajectoryID, req.CommandID, req.CommandDigest, types.LifecycleExpireStalePacket, seq, eventObjs)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: receiptObj.CanonicalID})
	objects = append(objects, receiptObj)

	result := types.LifecycleResult{Receipt: receipt, Update: &update, Events: events}
	return s.commitLifecycleTransition(ctx, ownerID, computerID, req.CommandID, req.CommandDigest, conditions, objects, result)
}

// expireWorkItemID picks the packet's most specific work identity for the
// expiry event: target binding first, then producer alias, then legacy.
func expireWorkItemID(update types.CoagentSourcePacket) string {
	for _, id := range []string{update.TargetWorkItemID, update.ProducerWorkItemID, update.WorkItemID} {
		if trimmed := strings.TrimSpace(id); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
