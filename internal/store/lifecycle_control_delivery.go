package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func ComputeBindLifecycleControlDeliveryDigest(req types.BindLifecycleControlDeliveryRequest) (string, error) {
	req.OwnerID, req.ComputerID, req.CommandDigest = "", "", ""
	req.CommandID, req.TrajectoryID = strings.TrimSpace(req.CommandID), strings.TrimSpace(req.TrajectoryID)
	req.TargetAgentID, req.TargetRunID = strings.TrimSpace(req.TargetAgentID), strings.TrimSpace(req.TargetRunID)
	req.ExpectedLifecycleVersion = 0
	for i := range req.Controls {
		req.Controls[i].UpdateID = strings.TrimSpace(req.Controls[i].UpdateID)
		req.Controls[i].ProducerAgentID = strings.TrimSpace(req.Controls[i].ProducerAgentID)
		req.Controls[i].ProducerUpdateID = strings.TrimSpace(req.Controls[i].ProducerUpdateID)
		req.Controls[i].TargetWorkItemID = strings.TrimSpace(req.Controls[i].TargetWorkItemID)
	}
	return lifecycleDigest(req)
}

func lifecycleRunBindsWork(run types.RunRecord, workID string) bool {
	if strings.TrimSpace(metadataStringValueStore(run.Metadata, "lifecycle_work_item_id")) == workID {
		return true
	}
	value := run.Metadata["work_item_ids"]
	switch ids := value.(type) {
	case []string:
		for _, id := range ids {
			if strings.TrimSpace(id) == workID {
				return true
			}
		}
	case []any:
		for _, raw := range ids {
			if id, ok := raw.(string); ok && strings.TrimSpace(id) == workID {
				return true
			}
		}
	}
	return false
}

func metadataStringValueStore(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	value, _ := metadata[key].(string)
	return strings.TrimSpace(value)
}

// BindLifecycleControlDelivery atomically binds ordered committed controls to
// one exact durable actor run. It is the delivery cursor authority; legacy
// mailbox delivery tables never participate.
func (s *Store) BindLifecycleControlDelivery(ctx context.Context, req types.BindLifecycleControlDeliveryRequest) (types.LifecycleResult, error) {
	ownerID, computerID, err := normalizeLifecycleScope(req.OwnerID, req.ComputerID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	req.OwnerID, req.ComputerID = ownerID, computerID
	req.CommandID, req.CommandDigest = strings.TrimSpace(req.CommandID), strings.TrimSpace(req.CommandDigest)
	req.TrajectoryID, req.TargetAgentID, req.TargetRunID = strings.TrimSpace(req.TrajectoryID), strings.TrimSpace(req.TargetAgentID), strings.TrimSpace(req.TargetRunID)
	if err := validateLifecycleCommand(req.CommandID, req.CommandDigest, req.TrajectoryID); err != nil {
		return types.LifecycleResult{}, err
	}
	if req.TargetAgentID == "" || req.TargetRunID == "" || req.ExpectedLifecycleVersion <= 0 || len(req.Controls) == 0 {
		return types.LifecycleResult{}, ErrLifecycleInvalidTransition
	}
	for i := range req.Controls {
		item := &req.Controls[i]
		item.UpdateID, item.ProducerAgentID, item.ProducerUpdateID, item.TargetWorkItemID = strings.TrimSpace(item.UpdateID), strings.TrimSpace(item.ProducerAgentID), strings.TrimSpace(item.ProducerUpdateID), strings.TrimSpace(item.TargetWorkItemID)
		if item.UpdateID == "" || item.ProducerAgentID == "" || item.ProducerUpdateID == "" || item.TargetWorkItemID == "" {
			return types.LifecycleResult{}, ErrLifecycleInvalidTransition
		}
	}
	computed, digestErr := ComputeBindLifecycleControlDeliveryDigest(req)
	if err := requireLifecycleDigest(req.CommandDigest, computed, digestErr); err != nil {
		return types.LifecycleResult{}, err
	}

	s.trajectoryMu.Lock()
	defer s.trajectoryMu.Unlock()
	if replay, found, replayErr := s.replayLifecycleCommand(ctx, ownerID, computerID, req.CommandID, req.CommandDigest); found || replayErr != nil {
		return replay, replayErr
	}
	trajectoryObj, trajectory, err := s.lifecycleTrajectoryObject(ctx, ownerID, computerID, req.TrajectoryID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	if trajectory.Status != types.TrajectoryLive || trajectory.LifecycleVersion != req.ExpectedLifecycleVersion {
		return types.LifecycleResult{}, ErrConcurrentStateChange
	}
	agentObj, agent, err := s.textureTurnAgentObject(ctx, ownerID, computerID, req.TargetAgentID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	profile := agentprofile.Canonical(agent.Profile)
	if agent.OwnerID != ownerID || agent.ComputerID != computerID || agentprofile.Canonical(agent.Role) != profile || (profile != agentprofile.Researcher && !(profile == agentprofile.Super && agent.AgentID == agentprofile.Super+":"+ownerID && agent.LifecycleVersion == 0)) {
		return types.LifecycleResult{}, ErrLifecycleInvalidTransition
	}
	runObj, run, err := s.textureTurnRunObject(ctx, ownerID, computerID, req.TargetRunID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	if run.RunID != req.TargetRunID || run.OwnerID != ownerID || run.SandboxID != computerID || run.AgentID != req.TargetAgentID || run.TrajectoryID != req.TrajectoryID || agentprofile.Canonical(run.AgentProfile) != profile || !run.State.Active() {
		return types.LifecycleResult{}, ErrLifecycleInvalidTransition
	}

	now := time.Now().UTC()
	seq := trajectory.ReducerSeq
	conditions := []objectgraph.ObjectCondition{{CanonicalID: trajectoryObj.CanonicalID, Exists: true, ExpectedContentHash: trajectoryObj.ContentHash}, {CanonicalID: agentObj.CanonicalID, Exists: true, ExpectedContentHash: agentObj.ContentHash}, {CanonicalID: runObj.CanonicalID, Exists: true, ExpectedContentHash: runObj.ContentHash}}
	objects := make([]objectgraph.Object, 0, len(req.Controls)*2+2)
	events := make([]types.LifecycleEvent, 0, len(req.Controls))
	eventObjs := make([]objectgraph.Object, 0, len(req.Controls))
	controls := make([]types.CoagentSourcePacket, 0, len(req.Controls))
	seen := map[string]bool{}
	for _, item := range req.Controls {
		if seen[item.UpdateID] || !lifecycleRunBindsWork(run, item.TargetWorkItemID) {
			return types.LifecycleResult{}, ErrLifecycleInvalidTransition
		}
		seen[item.UpdateID] = true
		key := req.TrajectoryID + "\x00" + req.TargetAgentID + "\x00" + item.ProducerAgentID + "\x00" + item.ProducerUpdateID
		updateObj, update, err := s.textureTurnUpdateObject(ctx, ownerID, computerID, key)
		if err != nil {
			return types.LifecycleResult{}, err
		}
		if update.UpdateID != item.UpdateID || update.Direction != types.LifecyclePacketDirectionControl || update.TargetAgentID != req.TargetAgentID || update.TargetWorkItemID != item.TargetWorkItemID || update.TrajectoryID != req.TrajectoryID || update.Disposition != types.UpdatePending || update.DeliveredAt != nil || strings.TrimSpace(update.DeliveredToRunID) != "" {
			return types.LifecycleResult{}, ErrLifecycleCommandConflict
		}
		workObj, work, err := s.lifecycleWorkObject(ctx, ownerID, computerID, item.TargetWorkItemID)
		if err != nil {
			return types.LifecycleResult{}, err
		}
		if work.Status != types.WorkItemOpen || work.TrajectoryID != req.TrajectoryID || work.AssignedAgentID != req.TargetAgentID || work.OwnerID != ownerID || work.ComputerID != computerID {
			return types.LifecycleResult{}, ErrLifecycleInvalidTransition
		}
		conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: updateObj.CanonicalID, Exists: true, ExpectedContentHash: updateObj.ContentHash}, objectgraph.ObjectCondition{CanonicalID: workObj.CanonicalID, Exists: true, ExpectedContentHash: workObj.ContentHash})
		seq++
		update.DeliveredToRunID, update.DeliveredAt = req.TargetRunID, &now
		update.LifecycleVersion++
		update.ReducerSeq = seq
		updatedObj, err := lifecycleObject(ogKindWorkerUpdate, ownerID, computerID, key, update, lifecycleMetadata("update_id", update.UpdateID, computerID, req.TrajectoryID, seq), updateObj.CreatedAt, now)
		if err != nil {
			return types.LifecycleResult{}, err
		}
		objects = append(objects, updatedObj)
		event := types.LifecycleEvent{EventID: req.CommandID + ":" + fmt.Sprintf("%d", len(events)+1), OwnerID: ownerID, ComputerID: computerID, TrajectoryID: req.TrajectoryID, WorkItemID: item.TargetWorkItemID, UpdateID: item.UpdateID, Kind: types.LifecycleControlDelivered, ReducerVersion: types.LifecycleReducerVersion, ReducerSeq: seq, CommandID: req.CommandID, CommandDigest: req.CommandDigest, CreatedAt: now}
		events = append(events, event)
		eventObj, err := lifecycleObject(ogKindLifecycleEvent, ownerID, computerID, event.EventID, event, lifecycleMetadata("event_id", event.EventID, computerID, req.TrajectoryID, seq), now, now)
		if err != nil {
			return types.LifecycleResult{}, err
		}
		conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: eventObj.CanonicalID})
		objects = append(objects, eventObj)
		eventObjs = append(eventObjs, eventObj)
		controls = append(controls, update)
	}
	trajectory.ReducerSeq, trajectory.LifecycleVersion, trajectory.UpdatedAt = seq, trajectory.LifecycleVersion+1, now
	trajectoryUpdated, err := lifecycleObject(ogKindTrajectory, ownerID, computerID, req.TrajectoryID, trajectory, lifecycleMetadata("trajectory_id", req.TrajectoryID, computerID, req.TrajectoryID, seq), trajectoryObj.CreatedAt, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	objects = append(objects, trajectoryUpdated)
	receipt, receiptObj, err := s.lifecycleTransitionReceipt(now, ownerID, computerID, req.TrajectoryID, req.CommandID, req.CommandDigest, types.LifecycleBindControlDelivery, seq, eventObjs)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: receiptObj.CanonicalID})
	objects = append(objects, receiptObj)
	result := types.LifecycleResult{Receipt: receipt, Trajectory: trajectory, Agent: &agent, Events: events, Controls: controls}
	return s.commitLifecycleTransition(ctx, ownerID, computerID, req.CommandID, req.CommandDigest, conditions, objects, result)
}
