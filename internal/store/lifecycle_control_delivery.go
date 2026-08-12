package store

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
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
	if err := normalizeLifecycleControlActivationRefresh(req.ActivationRefresh); err != nil {
		return "", err
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

func normalizeLifecycleControlActivationRefresh(refresh *types.LifecycleControlActivationRefresh) error {
	if refresh == nil {
		return nil
	}
	refresh.Prompt = strings.TrimSpace(refresh.Prompt)
	refresh.LogicalActivationKey = strings.TrimSpace(refresh.LogicalActivationKey)
	refresh.FailedAttemptKey = strings.TrimSpace(refresh.FailedAttemptKey)
	refresh.BuildCommit = strings.TrimSpace(refresh.BuildCommit)
	if refresh.Prompt == "" || refresh.LogicalActivationKey == "" || refresh.FailedAttemptKey == "" || refresh.BuildCommit == "" || len(refresh.Versions) == 0 || len(refresh.WorkItemIDs) == 0 {
		return ErrLifecycleInvalidTransition
	}
	seenWork := map[string]bool{}
	for i := range refresh.WorkItemIDs {
		refresh.WorkItemIDs[i] = strings.TrimSpace(refresh.WorkItemIDs[i])
		if refresh.WorkItemIDs[i] == "" || seenWork[refresh.WorkItemIDs[i]] {
			return ErrLifecycleInvalidTransition
		}
		seenWork[refresh.WorkItemIDs[i]] = true
	}
	seenUpdate := map[string]bool{}
	for i := range refresh.Versions {
		version := &refresh.Versions[i]
		version.UpdateID = strings.TrimSpace(version.UpdateID)
		version.TargetWorkItemID = strings.TrimSpace(version.TargetWorkItemID)
		if version.UpdateID == "" || version.TargetWorkItemID == "" || version.ControlLifecycleVersion <= 0 || version.WorkLifecycleVersion <= 0 || seenUpdate[version.UpdateID] || !seenWork[version.TargetWorkItemID] {
			return ErrLifecycleInvalidTransition
		}
		seenUpdate[version.UpdateID] = true
	}
	return nil
}

// LifecycleControlActivationReplay is the canonical pre-dispatch replay view
// for one runtime-derived lifecycle-control fingerprint.
type LifecycleControlActivationReplay struct {
	Active        *types.RunRecord
	DurablyFailed *types.RunRecord
}

// ResolveLifecycleControlActivation returns the unique active logical
// activation and exact durably failed attempt in one lifecycle scope. It never
// consults legacy run projections. Duplicate fingerprint owners are ambiguous
// and fail closed.
func (s *Store) ResolveLifecycleControlActivation(ctx context.Context, ownerID, computerID, trajectoryID, agentID, logicalKey, failedAttemptKey string, versions []types.LifecycleControlActivationVersion) (LifecycleControlActivationReplay, error) {
	ownerID, computerID, err := normalizeLifecycleScope(ownerID, computerID)
	if err != nil {
		return LifecycleControlActivationReplay{}, err
	}
	trajectoryID, agentID = strings.TrimSpace(trajectoryID), strings.TrimSpace(agentID)
	logicalKey, failedAttemptKey = strings.TrimSpace(logicalKey), strings.TrimSpace(failedAttemptKey)
	if trajectoryID == "" || agentID == "" || logicalKey == "" || failedAttemptKey == "" || len(versions) == 0 {
		return LifecycleControlActivationReplay{}, ErrLifecycleInvalidTransition
	}
	seenUpdates := make(map[string]bool, len(versions))
	for _, version := range versions {
		version.UpdateID, version.TargetWorkItemID = strings.TrimSpace(version.UpdateID), strings.TrimSpace(version.TargetWorkItemID)
		if version.UpdateID == "" || version.TargetWorkItemID == "" || version.ControlLifecycleVersion <= 0 || version.WorkLifecycleVersion <= 0 || seenUpdates[version.UpdateID] {
			return LifecycleControlActivationReplay{}, ErrLifecycleInvalidTransition
		}
		seenUpdates[version.UpdateID] = true
		work, workErr := s.GetLifecycleWorkItem(ctx, ownerID, computerID, version.TargetWorkItemID)
		if workErr != nil {
			return LifecycleControlActivationReplay{}, workErr
		}
		if work.OwnerID != ownerID || work.ComputerID != computerID || work.TrajectoryID != trajectoryID || work.AssignedAgentID != agentID || work.LifecycleVersion != version.WorkLifecycleVersion || work.Status != types.WorkItemOpen {
			return LifecycleControlActivationReplay{}, ErrConcurrentStateChange
		}
	}
	agent, err := s.GetAgentByScope(ctx, ownerID, computerID, agentID)
	if err != nil {
		return LifecycleControlActivationReplay{}, err
	}
	if agent.OwnerID != ownerID || agent.ComputerID != computerID || agent.AgentID != agentID || agentprofile.Canonical(agent.Profile) != agentprofile.Researcher || agentprofile.Canonical(agent.Role) != agentprofile.Researcher || agent.LifecycleVersion <= 0 {
		return LifecycleControlActivationReplay{}, ErrLifecycleInvalidTransition
	}
	result := LifecycleControlActivationReplay{}
	if activeRunID := strings.TrimSpace(agent.ActiveRunID); activeRunID != "" {
		active, activeErr := s.GetLifecycleRun(ctx, ownerID, computerID, activeRunID)
		if activeErr != nil {
			return LifecycleControlActivationReplay{}, activeErr
		}
		if active.OwnerID != ownerID || active.ComputerID != computerID || active.TrajectoryID != trajectoryID || active.AgentID != agentID || !lifecycleRunOwnsActivation(active.State) {
			return LifecycleControlActivationReplay{}, ErrLifecycleInvalidTransition
		}
		if metadataStringValueStore(active.Metadata, "request_source") == "lifecycle_texture_control" && metadataStringValueStore(active.Metadata, "lifecycle_logical_activation_key") == logicalKey {
			result.Active = &active
		}
	}
	failed, err := s.resolveLifecycleControlActivationFailure(ctx, ownerID, computerID, trajectoryID, agentID, logicalKey, failedAttemptKey, versions)
	if err != nil {
		return LifecycleControlActivationReplay{}, err
	}
	result.DurablyFailed = failed
	return result, nil
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
		if item.UpdateID == "" || item.ProducerAgentID == "" || item.ProducerUpdateID == "" || item.TargetWorkItemID == "" || item.ExpectedControlLifecycleVersion <= 0 || item.ExpectedWorkLifecycleVersion <= 0 {
			return types.LifecycleResult{}, ErrLifecycleInvalidTransition
		}
	}
	if err := normalizeLifecycleControlActivationRefresh(req.ActivationRefresh); err != nil {
		return types.LifecycleResult{}, err
	}
	if req.ActivationRefresh != nil && len(req.ActivationRefresh.Versions) != len(req.Controls) {
		return types.LifecycleResult{}, ErrLifecycleInvalidTransition
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
	agentCanonicalID, err := scopedAgentCanonicalID(ownerID, computerID, req.TargetAgentID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	agentObj, err := s.lifecycleGraph().GetObject(ctx, agentCanonicalID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	agent, err := decodeLifecycleObject[types.AgentRecord](agentObj)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	profile := agentprofile.Canonical(agent.Profile)
	if agent.OwnerID != ownerID || agent.ComputerID != computerID || agentprofile.Canonical(agent.Role) != profile || (profile != agentprofile.Researcher && !(profile == agentprofile.Super && agent.AgentID == agentprofile.Super+":"+ownerID && agent.LifecycleVersion == 0)) {
		return types.LifecycleResult{}, ErrLifecycleInvalidTransition
	}
	var runObj objectgraph.Object
	var run types.RunRecord
	if profile == agentprofile.Super {
		runObj, err = s.getRunObjectByOwnerOG(ctx, ownerID, req.TargetRunID)
		if err == nil {
			err = ogDecode(runObj, &run)
		}
	} else {
		runObj, run, err = s.textureTurnRunObject(ctx, ownerID, computerID, req.TargetRunID)
	}
	if err != nil {
		return types.LifecycleResult{}, err
	}
	if run.RunID != req.TargetRunID || run.OwnerID != ownerID || run.ComputerID != computerID || run.AgentID != req.TargetAgentID || agentprofile.Canonical(run.AgentProfile) != profile || !run.State.Active() {
		return types.LifecycleResult{}, ErrLifecycleInvalidTransition
	}
	if profile == agentprofile.Super {
		if strings.TrimSpace(run.TrajectoryID) != "" || metadataStringValueStore(run.Metadata, "assignment_trajectory_id") != req.TrajectoryID {
			return types.LifecycleResult{}, ErrLifecycleInvalidTransition
		}
	} else if run.TrajectoryID != req.TrajectoryID {
		return types.LifecycleResult{}, ErrLifecycleInvalidTransition
	}

	now := time.Now().UTC()
	seq := trajectory.ReducerSeq
	conditions := []objectgraph.ObjectCondition{{CanonicalID: trajectoryObj.CanonicalID, Exists: true, ExpectedContentHash: trajectoryObj.ContentHash}, {CanonicalID: agentObj.CanonicalID, Exists: true, ExpectedContentHash: agentObj.ContentHash}, {CanonicalID: runObj.CanonicalID, Exists: true, ExpectedContentHash: runObj.ContentHash}}
	seenConditions := make(map[string]bool, len(req.Controls)*3+len(conditions))
	for _, condition := range conditions {
		seenConditions[condition.CanonicalID] = true
	}
	appendCondition := func(condition objectgraph.ObjectCondition) {
		if !seenConditions[condition.CanonicalID] {
			conditions = append(conditions, condition)
			seenConditions[condition.CanonicalID] = true
		}
	}
	objects := make([]objectgraph.Object, 0, len(req.Controls)*2+2)
	events := make([]types.LifecycleEvent, 0, len(req.Controls))
	eventObjs := make([]objectgraph.Object, 0, len(req.Controls))
	controls := make([]types.CoagentSourcePacket, 0, len(req.Controls))
	controlBindings := make([]map[string]string, 0, len(req.Controls))
	controlWorkIDs := make([]string, 0, len(req.Controls))
	seenControlWork := map[string]bool{}
	refreshVersions := map[string]types.LifecycleControlActivationVersion{}
	if req.ActivationRefresh != nil {
		for _, version := range req.ActivationRefresh.Versions {
			refreshVersions[version.UpdateID] = version
		}
	}
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
		if update.LifecycleVersion != item.ExpectedControlLifecycleVersion {
			return types.LifecycleResult{}, ErrConcurrentStateChange
		}
		workObj, work, err := s.lifecycleWorkObject(ctx, ownerID, computerID, item.TargetWorkItemID)
		if err != nil {
			return types.LifecycleResult{}, err
		}
		if work.Status != types.WorkItemOpen || work.TrajectoryID != req.TrajectoryID || work.AssignedAgentID != req.TargetAgentID || work.OwnerID != ownerID || work.ComputerID != computerID {
			return types.LifecycleResult{}, ErrLifecycleInvalidTransition
		}
		if work.LifecycleVersion != item.ExpectedWorkLifecycleVersion {
			return types.LifecycleResult{}, ErrConcurrentStateChange
		}
		if req.ActivationRefresh != nil {
			version, ok := refreshVersions[item.UpdateID]
			if !ok || version.TargetWorkItemID != item.TargetWorkItemID || version.ControlLifecycleVersion != item.ExpectedControlLifecycleVersion || version.WorkLifecycleVersion != item.ExpectedWorkLifecycleVersion {
				return types.LifecycleResult{}, ErrLifecycleInvalidTransition
			}
		}
		appendCondition(objectgraph.ObjectCondition{CanonicalID: updateObj.CanonicalID, Exists: true, ExpectedContentHash: updateObj.ContentHash})
		appendCondition(objectgraph.ObjectCondition{CanonicalID: workObj.CanonicalID, Exists: true, ExpectedContentHash: workObj.ContentHash})
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
		appendCondition(objectgraph.ObjectCondition{CanonicalID: eventObj.CanonicalID})
		objects = append(objects, eventObj)
		eventObjs = append(eventObjs, eventObj)
		controls = append(controls, update)
		controlBindings = append(controlBindings, map[string]string{"trajectory_id": req.TrajectoryID, "update_id": update.UpdateID, "target_work_item_id": update.TargetWorkItemID, "producer_agent_id": update.AgentID})
		if !seenControlWork[update.TargetWorkItemID] {
			seenControlWork[update.TargetWorkItemID] = true
			controlWorkIDs = append(controlWorkIDs, update.TargetWorkItemID)
		}
	}
	if req.ActivationRefresh != nil {
		if profile != agentprofile.Researcher || len(controlWorkIDs) != len(req.ActivationRefresh.WorkItemIDs) {
			return types.LifecycleResult{}, ErrLifecycleInvalidTransition
		}
		for index := range controlWorkIDs {
			if controlWorkIDs[index] != req.ActivationRefresh.WorkItemIDs[index] {
				return types.LifecycleResult{}, ErrLifecycleInvalidTransition
			}
		}
		run.Prompt = req.ActivationRefresh.Prompt
		if run.Metadata == nil {
			run.Metadata = map[string]any{}
		}
		run.Metadata["lifecycle_logical_activation_key"] = req.ActivationRefresh.LogicalActivationKey
		run.Metadata["lifecycle_failed_attempt_key"] = req.ActivationRefresh.FailedAttemptKey
		run.Metadata["lifecycle_activation_build_commit"] = req.ActivationRefresh.BuildCommit
		run.Metadata["lifecycle_activation_versions"] = req.ActivationRefresh.Versions
		run.Metadata["work_item_ids"] = append([]string(nil), req.ActivationRefresh.WorkItemIDs...)
		run.Metadata["request_source"] = "lifecycle_texture_control"
		run.Metadata["trajectory_id"] = req.TrajectoryID
	}
	if run.Metadata == nil {
		run.Metadata = map[string]any{}
	}
	run.Metadata["assignment_trajectory_id"] = req.TrajectoryID
	// A resident exact run can receive more controls after its first bind. Keep
	// every historical control join in order; replacing this slice would make
	// older CoSuper assignment/report authority disappear.
	mergedBindings := make([]map[string]string, 0, len(controlBindings))
	seenBindings := map[string]bool{}
	appendBinding := func(binding map[string]string) {
		key := strings.TrimSpace(binding["trajectory_id"]) + "\x00" + strings.TrimSpace(binding["update_id"])
		if key == "\x00" || seenBindings[key] {
			return
		}
		seenBindings[key] = true
		mergedBindings = append(mergedBindings, binding)
	}
	switch existing := run.Metadata["lifecycle_control_bindings"].(type) {
	case []any:
		for _, raw := range existing {
			entry, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			appendBinding(map[string]string{
				"trajectory_id":       strings.TrimSpace(fmt.Sprint(entry["trajectory_id"])),
				"update_id":           strings.TrimSpace(fmt.Sprint(entry["update_id"])),
				"target_work_item_id": strings.TrimSpace(fmt.Sprint(entry["target_work_item_id"])),
				"producer_agent_id":   strings.TrimSpace(fmt.Sprint(entry["producer_agent_id"])),
			})
		}
	case []map[string]string:
		for _, entry := range existing {
			appendBinding(entry)
		}
	case []map[string]any:
		for _, entry := range existing {
			appendBinding(map[string]string{
				"trajectory_id":       strings.TrimSpace(fmt.Sprint(entry["trajectory_id"])),
				"update_id":           strings.TrimSpace(fmt.Sprint(entry["update_id"])),
				"target_work_item_id": strings.TrimSpace(fmt.Sprint(entry["target_work_item_id"])),
				"producer_agent_id":   strings.TrimSpace(fmt.Sprint(entry["producer_agent_id"])),
			})
		}
	}
	for _, binding := range controlBindings {
		appendBinding(binding)
	}
	run.Metadata["lifecycle_control_bindings"] = mergedBindings
	run.UpdatedAt = now
	runBody, err := json.Marshal(run)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	runUpdated := runObj
	runUpdated.Body, runUpdated.ContentHash, runUpdated.UpdatedAt = runBody, objectgraph.SHA256(runBody), now
	objects = append(objects, runUpdated)
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
	appendCondition(objectgraph.ObjectCondition{CanonicalID: receiptObj.CanonicalID})
	objects = append(objects, receiptObj)
	result := types.LifecycleResult{Receipt: receipt, Trajectory: trajectory, Agent: &agent, Events: events, Controls: controls}
	return s.commitLifecycleTransition(ctx, ownerID, computerID, req.CommandID, req.CommandDigest, conditions, objects, result)
}

// LifecycleDeliveredPacketPage is one exact-run delivery page. NextCursor is
// the immutable packet MessageSeq. ReducerSeq is not a valid delivery cursor:
// it advances again when the packet is incorporated.
type LifecycleDeliveredPacketPage struct {
	Packets    []types.CoagentSourcePacket
	NextCursor int64
	HasMore    bool
}

// ListLifecycleControlsDeliveredToRun returns one compatibility page of the
// typed envelopes bound to one exact durable run. Callers that must observe the
// complete run use ListLifecycleControlsDeliveredToRunPage until HasMore is
// false; they must never assume the first 100 occurrences are the whole run.
func (s *Store) ListLifecycleControlsDeliveredToRun(ctx context.Context, ownerID, computerID, trajectoryID, targetAgentID, targetRunID string, limit int) ([]types.CoagentSourcePacket, error) {
	page, err := s.ListLifecycleControlsDeliveredToRunPage(ctx, ownerID, computerID, trajectoryID, targetAgentID, targetRunID, 0, limit)
	return page.Packets, err
}

// ListLifecycleControlsDeliveredToRunPage returns downward controls plus
// authenticated CoSuper reports already delivered to one exact run. It
// validates the complete exact-run set before returning a page, never consults
// the legacy mailbox, and resumes strictly after an immutable occurrence cursor.
func (s *Store) ListLifecycleControlsDeliveredToRunPage(ctx context.Context, ownerID, computerID, trajectoryID, targetAgentID, targetRunID string, after int64, limit int) (LifecycleDeliveredPacketPage, error) {
	ownerID, computerID, err := normalizeLifecycleScope(ownerID, computerID)
	if err != nil {
		return LifecycleDeliveredPacketPage{}, err
	}
	trajectoryID = strings.TrimSpace(trajectoryID)
	targetAgentID = strings.TrimSpace(targetAgentID)
	targetRunID = strings.TrimSpace(targetRunID)
	if trajectoryID == "" || targetAgentID == "" || targetRunID == "" {
		return LifecycleDeliveredPacketPage{}, ErrLifecycleInvalidTransition
	}
	if limit <= 0 {
		limit = 100
	}

	agent, err := s.GetAgentByScope(ctx, ownerID, computerID, targetAgentID)
	if err != nil {
		return LifecycleDeliveredPacketPage{}, err
	}
	profile := agentprofile.Canonical(agent.Profile)
	if agent.OwnerID != ownerID || agent.ComputerID != computerID || agentprofile.Canonical(agent.Role) != profile ||
		(profile != agentprofile.Researcher && !(profile == agentprofile.Super && targetAgentID == agentprofile.Super+":"+ownerID && agent.LifecycleVersion == 0)) {
		return LifecycleDeliveredPacketPage{}, ErrLifecycleInvalidTransition
	}
	var run types.RunRecord
	if profile == agentprofile.Super {
		run, err = s.GetRunByOwner(ctx, ownerID, targetRunID)
	} else {
		run, err = s.GetLifecycleRun(ctx, ownerID, computerID, targetRunID)
	}
	if err != nil {
		return LifecycleDeliveredPacketPage{}, err
	}
	if run.RunID != targetRunID || run.OwnerID != ownerID || run.ComputerID != computerID || run.AgentID != targetAgentID || agentprofile.Canonical(run.AgentProfile) != profile {
		return LifecycleDeliveredPacketPage{}, ErrLifecycleInvalidTransition
	}
	if profile == agentprofile.Super {
		if strings.TrimSpace(run.TrajectoryID) != "" || metadataStringValueStore(run.Metadata, "assignment_trajectory_id") != trajectoryID {
			return LifecycleDeliveredPacketPage{}, ErrLifecycleInvalidTransition
		}
	} else if strings.TrimSpace(run.TrajectoryID) != trajectoryID {
		return LifecycleDeliveredPacketPage{}, ErrLifecycleInvalidTransition
	}

	graph := s.ogReadStore
	if graph == nil {
		graph = s.ogStore
	}
	if graph == nil {
		return LifecycleDeliveredPacketPage{}, fmt.Errorf("lifecycle delivered controls: object graph not initialized")
	}
	objects, err := graph.ReadObjectSnapshot(ctx, ownerID, computerID)
	if err != nil {
		return LifecycleDeliveredPacketPage{}, err
	}
	updates := make([]types.CoagentSourcePacket, 0)
	for _, obj := range objects {
		if obj.ObjectKind != ogKindWorkerUpdate {
			continue
		}
		update, decodeErr := decodeLifecycleObject[types.CoagentSourcePacket](obj)
		if decodeErr != nil {
			return LifecycleDeliveredPacketPage{}, decodeErr
		}
		if update.DeliveredToRunID != targetRunID {
			continue
		}
		if update.OwnerID != ownerID || update.ComputerID != computerID || update.TrajectoryID != trajectoryID || update.TargetAgentID != targetAgentID ||
			(update.Disposition != types.UpdatePending && update.Disposition != types.UpdateIncorporated) || update.DeliveredAt == nil || update.Packet.SchemaVersion != types.CoagentSourcePacketSchemaV1 ||
			strings.TrimSpace(update.Packet.Kind) == "" || strings.TrimSpace(update.Content) == "" {
			return LifecycleDeliveredPacketPage{}, ErrLifecycleInvalidTransition
		}
		payloadDigest, digestErr := ComputeLifecycleUpdatePayloadDigest(update.Packet, update.Content)
		if digestErr != nil || payloadDigest != update.PayloadDigest {
			return LifecycleDeliveredPacketPage{}, ErrLifecycleInvalidTransition
		}
		producerWorkID, targetWorkID, bindingErr := ResolveLifecyclePacketWorkBindings(update)
		if bindingErr != nil {
			return LifecycleDeliveredPacketPage{}, bindingErr
		}
		switch update.Direction {
		case types.LifecyclePacketDirectionControl:
			if targetWorkID == "" || !lifecycleRunBindsWork(run, targetWorkID) {
				return LifecycleDeliveredPacketPage{}, ErrLifecycleInvalidTransition
			}
			work, workErr := s.GetLifecycleWorkItem(ctx, ownerID, computerID, targetWorkID)
			if workErr != nil {
				return LifecycleDeliveredPacketPage{}, workErr
			}
			if work.OwnerID != ownerID || work.ComputerID != computerID || work.TrajectoryID != trajectoryID || work.AssignedAgentID != targetAgentID ||
				(update.Disposition == types.UpdatePending && work.Status != types.WorkItemOpen) ||
				(update.Disposition == types.UpdateIncorporated && work.Status != types.WorkItemOpen && !workItemTerminal(work.Status)) {
				return LifecycleDeliveredPacketPage{}, ErrLifecycleInvalidTransition
			}
		case types.LifecyclePacketDirectionProducerReport:
			if profile != agentprofile.Super || producerWorkID == "" || targetWorkID == "" || strings.TrimSpace(update.ControlBindingID) == "" ||
				!lifecycleRunBindsWork(run, targetWorkID) || !persistentSuperControlBinding(run.Metadata, trajectoryID, targetWorkID, update.ControlBindingID) {
				return LifecycleDeliveredPacketPage{}, ErrLifecycleInvalidTransition
			}
			producerRun, runErr := s.GetLifecycleRun(ctx, ownerID, computerID, update.SourceRunID)
			if runErr != nil {
				return LifecycleDeliveredPacketPage{}, runErr
			}
			producerWork, workErr := s.GetLifecycleWorkItem(ctx, ownerID, computerID, producerWorkID)
			if workErr != nil {
				return LifecycleDeliveredPacketPage{}, workErr
			}
			if producerRun.RunID != update.SourceRunID || producerRun.AgentID != update.AgentID || producerRun.TrajectoryID != trajectoryID ||
				producerRun.AgentProfile != agentprofile.CoSuper || !producerRun.State.Valid() || !lifecycleRunBindsWork(producerRun, producerWorkID) ||
				producerWork.TrajectoryID != trajectoryID || producerWork.AssignedAgentID != update.AgentID || producerWork.AuthorityProfile != agentprofile.CoSuper {
				return LifecycleDeliveredPacketPage{}, ErrLifecycleInvalidTransition
			}
		default:
			return LifecycleDeliveredPacketPage{}, ErrLifecycleInvalidTransition
		}
		updates = append(updates, update)
	}
	sort.Slice(updates, func(i, j int) bool {
		if updates[i].MessageSeq != updates[j].MessageSeq {
			return updates[i].MessageSeq < updates[j].MessageSeq
		}
		return updates[i].UpdateID < updates[j].UpdateID
	})
	if after < 0 {
		return LifecycleDeliveredPacketPage{}, ErrLifecycleInvalidTransition
	}
	first := sort.Search(len(updates), func(i int) bool { return updates[i].MessageSeq > after })
	remaining := updates[first:]
	page := LifecycleDeliveredPacketPage{Packets: make([]types.CoagentSourcePacket, 0, min(limit, len(remaining))), NextCursor: after}
	if len(remaining) > limit {
		page.HasMore = true
		remaining = remaining[:limit]
	}
	page.Packets = append(page.Packets, remaining...)
	if len(page.Packets) > 0 {
		page.NextCursor = page.Packets[len(page.Packets)-1].MessageSeq
	}
	return page, nil
}

// ListHistoricalLifecycleControlsDeliveredToRun returns every downward control
// durably delivered to one exact old persistent-Super run after its trajectory
// became terminal. It is evidence authentication only: terminal/passivated run
// and terminal work identities are required, and no delivery or lifecycle state
// is changed.
func (s *Store) ListHistoricalLifecycleControlsDeliveredToRun(ctx context.Context, ownerID, computerID, trajectoryID, targetAgentID, targetRunID string) ([]types.CoagentSourcePacket, error) {
	ownerID, computerID, err := normalizeLifecycleScope(ownerID, computerID)
	if err != nil {
		return nil, err
	}
	trajectoryID, targetAgentID, targetRunID = strings.TrimSpace(trajectoryID), strings.TrimSpace(targetAgentID), strings.TrimSpace(targetRunID)
	if trajectoryID == "" || targetAgentID != agentprofile.Super+":"+ownerID || targetRunID == "" {
		return nil, ErrLifecycleInvalidTransition
	}
	trajectory, err := s.GetLifecycleTrajectory(ctx, ownerID, computerID, trajectoryID)
	if err != nil {
		return nil, err
	}
	if trajectory.Status == types.TrajectoryLive {
		return nil, ErrLifecycleInvalidTransition
	}
	agent, err := s.GetAgentByScope(ctx, ownerID, computerID, targetAgentID)
	if err != nil {
		return nil, err
	}
	if agent.OwnerID != ownerID || agent.ComputerID != computerID || agent.Profile != agentprofile.Super ||
		agent.Role != agentprofile.Super || agent.LifecycleVersion != 0 {
		return nil, ErrLifecycleInvalidTransition
	}
	run, err := s.GetRunByOwner(ctx, ownerID, targetRunID)
	if err != nil {
		return nil, err
	}
	if run.RunID != targetRunID || run.OwnerID != ownerID || run.ComputerID != computerID || run.AgentID != targetAgentID ||
		run.AgentProfile != agentprofile.Super || run.AgentRole != agentprofile.Super || run.TrajectoryID != "" ||
		metadataStringValueStore(run.Metadata, "assignment_trajectory_id") != trajectoryID ||
		!persistentSuperHistoricalReportRunStateAllowed(run.State) {
		return nil, ErrLifecycleInvalidTransition
	}
	graph := s.ogReadStore
	if graph == nil {
		graph = s.ogStore
	}
	if graph == nil {
		return nil, fmt.Errorf("historical lifecycle controls: object graph not initialized")
	}
	objects, err := graph.ReadObjectSnapshot(ctx, ownerID, computerID)
	if err != nil {
		return nil, err
	}
	controls := make([]types.CoagentSourcePacket, 0)
	for _, obj := range objects {
		if obj.ObjectKind != ogKindWorkerUpdate {
			continue
		}
		control, decodeErr := decodeLifecycleObject[types.CoagentSourcePacket](obj)
		if decodeErr != nil {
			return nil, decodeErr
		}
		if control.Direction != types.LifecyclePacketDirectionControl || control.DeliveredToRunID != targetRunID {
			continue
		}
		if control.OwnerID != ownerID || control.ComputerID != computerID || control.TrajectoryID != trajectoryID ||
			control.TargetAgentID != targetAgentID || control.DeliveredAt == nil || control.TargetWorkItemID == "" ||
			control.Packet.SchemaVersion != types.CoagentSourcePacketSchemaV1 || strings.TrimSpace(control.Packet.Kind) == "" ||
			strings.TrimSpace(control.Content) == "" || !lifecycleRunBindsWork(run, control.TargetWorkItemID) ||
			!persistentSuperControlBinding(run.Metadata, trajectoryID, control.TargetWorkItemID, control.UpdateID) {
			return nil, ErrLifecycleInvalidTransition
		}
		payloadDigest, digestErr := ComputeLifecycleUpdatePayloadDigest(control.Packet, control.Content)
		if digestErr != nil || payloadDigest != control.PayloadDigest {
			return nil, ErrLifecycleInvalidTransition
		}
		work, workErr := s.GetLifecycleWorkItem(ctx, ownerID, computerID, control.TargetWorkItemID)
		if workErr != nil {
			return nil, workErr
		}
		if work.OwnerID != ownerID || work.ComputerID != computerID || work.TrajectoryID != trajectoryID ||
			work.AssignedAgentID != targetAgentID || work.AuthorityProfile != agentprofile.Super || !workItemTerminal(work.Status) {
			return nil, ErrLifecycleInvalidTransition
		}
		controls = append(controls, control)
	}
	sort.Slice(controls, func(i, j int) bool {
		if controls[i].ReducerSeq != controls[j].ReducerSeq {
			return controls[i].ReducerSeq < controls[j].ReducerSeq
		}
		return controls[i].UpdateID < controls[j].UpdateID
	})
	if len(controls) == 0 {
		return nil, ErrNotFound
	}
	return controls, nil
}
