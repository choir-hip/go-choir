package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

const LifecycleControlActivationMissingRunWorkBindingFailure = "lifecycle control activation missing run-work binding"

func LifecycleControlActivationFailureCommandID(failedAttemptKey string) string {
	failedAttemptKey = strings.TrimSpace(failedAttemptKey)
	return "fail-control-activation:" + strings.TrimPrefix(failedAttemptKey, "sha256:")
}

const (
	lifecycleControlFailureCommandIDMetadata     = "lifecycle_control_activation_failure_command_id"
	lifecycleControlFailureCommandDigestMetadata = "lifecycle_control_activation_failure_command_digest"
)

func ComputeFailLifecycleControlActivationDigest(req types.FailLifecycleControlActivationRequest) (string, error) {
	req.OwnerID, req.ComputerID, req.CommandDigest = "", "", ""
	req.CommandID, req.TrajectoryID = strings.TrimSpace(req.CommandID), strings.TrimSpace(req.TrajectoryID)
	req.AgentID, req.RunID = strings.TrimSpace(req.AgentID), strings.TrimSpace(req.RunID)
	req.LogicalActivationKey, req.FailedAttemptKey = strings.TrimSpace(req.LogicalActivationKey), strings.TrimSpace(req.FailedAttemptKey)
	req.BindCommandID, req.BindCommandDigest, req.Failure = strings.TrimSpace(req.BindCommandID), strings.TrimSpace(req.BindCommandDigest), strings.TrimSpace(req.Failure)
	for i := range req.Controls {
		item := &req.Controls[i]
		item.UpdateID, item.ProducerAgentID = strings.TrimSpace(item.UpdateID), strings.TrimSpace(item.ProducerAgentID)
		item.ProducerUpdateID, item.TargetWorkItemID = strings.TrimSpace(item.ProducerUpdateID), strings.TrimSpace(item.TargetWorkItemID)
	}
	if err := normalizeLifecycleControlActivationRefresh(req.ActivationRefresh); err != nil {
		return "", err
	}
	return lifecycleDigest(req)
}

func lifecycleControlFailureMetadataCopy(metadata map[string]any) map[string]any {
	copy := make(map[string]any, len(metadata)+4)
	for key, value := range metadata {
		copy[key] = value
	}
	return copy
}

func lifecycleControlBindingsEmpty(metadata map[string]any) bool {
	if metadata == nil {
		return true
	}
	switch bindings := metadata["lifecycle_control_bindings"].(type) {
	case nil:
		return true
	case []any:
		return len(bindings) == 0
	case []map[string]string:
		return len(bindings) == 0
	case []map[string]any:
		return len(bindings) == 0
	default:
		return false
	}
}

// FailLifecycleControlActivation is the only durable handled-failure authority
// for a Researcher control activation. It accepts only the deterministic
// missing run/work binding that BindLifecycleControlDelivery would reject and
// atomically records the failed run, cleared agent activation, reducer advance,
// typed event, and replay receipt.
func (s *Store) FailLifecycleControlActivation(ctx context.Context, req types.FailLifecycleControlActivationRequest) (types.LifecycleResult, error) {
	ownerID, computerID, err := normalizeLifecycleScope(req.OwnerID, req.ComputerID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	req.OwnerID, req.ComputerID = ownerID, computerID
	req.CommandID, req.CommandDigest = strings.TrimSpace(req.CommandID), strings.TrimSpace(req.CommandDigest)
	req.TrajectoryID, req.AgentID, req.RunID = strings.TrimSpace(req.TrajectoryID), strings.TrimSpace(req.AgentID), strings.TrimSpace(req.RunID)
	req.LogicalActivationKey, req.FailedAttemptKey = strings.TrimSpace(req.LogicalActivationKey), strings.TrimSpace(req.FailedAttemptKey)
	req.BindCommandID, req.BindCommandDigest, req.Failure = strings.TrimSpace(req.BindCommandID), strings.TrimSpace(req.BindCommandDigest), strings.TrimSpace(req.Failure)
	if err := validateLifecycleCommand(req.CommandID, req.CommandDigest, req.TrajectoryID); err != nil {
		return types.LifecycleResult{}, err
	}
	if req.AgentID == "" || req.RunID == "" || req.ExpectedLifecycleVersion <= 0 || req.LogicalActivationKey == "" || req.FailedAttemptKey == "" || req.CommandID != LifecycleControlActivationFailureCommandID(req.FailedAttemptKey) || req.BindCommandID == "" || req.BindCommandDigest == "" || len(req.Controls) == 0 || req.Failure != LifecycleControlActivationMissingRunWorkBindingFailure {
		return types.LifecycleResult{}, ErrLifecycleInvalidTransition
	}
	for i := range req.Controls {
		item := &req.Controls[i]
		item.UpdateID, item.ProducerAgentID = strings.TrimSpace(item.UpdateID), strings.TrimSpace(item.ProducerAgentID)
		item.ProducerUpdateID, item.TargetWorkItemID = strings.TrimSpace(item.ProducerUpdateID), strings.TrimSpace(item.TargetWorkItemID)
		if item.UpdateID == "" || item.ProducerAgentID == "" || item.ProducerUpdateID == "" || item.TargetWorkItemID == "" || item.ExpectedControlLifecycleVersion <= 0 || item.ExpectedWorkLifecycleVersion <= 0 {
			return types.LifecycleResult{}, ErrLifecycleInvalidTransition
		}
	}
	computed, digestErr := ComputeFailLifecycleControlActivationDigest(req)
	if err := requireLifecycleDigest(req.CommandDigest, computed, digestErr); err != nil {
		return types.LifecycleResult{}, err
	}
	bindReq := types.BindLifecycleControlDeliveryRequest{
		OwnerID: ownerID, ComputerID: computerID, CommandID: req.BindCommandID, CommandDigest: req.BindCommandDigest,
		TrajectoryID: req.TrajectoryID, TargetAgentID: req.AgentID, TargetRunID: req.RunID,
		ExpectedLifecycleVersion: req.ExpectedLifecycleVersion, Controls: req.Controls, ActivationRefresh: req.ActivationRefresh,
	}
	computedBind, bindDigestErr := ComputeBindLifecycleControlDeliveryDigest(bindReq)
	if bindDigestErr != nil || computedBind != req.BindCommandDigest {
		return types.LifecycleResult{}, ErrLifecycleInvalidTransition
	}
	if req.ActivationRefresh == nil || strings.TrimSpace(req.ActivationRefresh.LogicalActivationKey) != req.LogicalActivationKey || strings.TrimSpace(req.ActivationRefresh.FailedAttemptKey) != req.FailedAttemptKey {
		return types.LifecycleResult{}, ErrLifecycleInvalidTransition
	}
	bindUpdateIDs := make([]string, 0, len(req.Controls))
	for _, item := range req.Controls {
		bindUpdateIDs = append(bindUpdateIDs, item.UpdateID)
	}
	if req.BindCommandID != "bind-control-delivery:"+req.RunID+":"+strings.Join(bindUpdateIDs, ",") {
		return types.LifecycleResult{}, ErrLifecycleInvalidTransition
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
	agentObj, err := s.lifecycleGetObject(ctx, ogKindAgent, ownerID, computerID, req.AgentID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	agent, err := decodeLifecycleObject[types.AgentRecord](agentObj)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	if agent.OwnerID != ownerID || agent.ComputerID != computerID || agent.AgentID != req.AgentID || agentprofile.Canonical(agent.Profile) != agentprofile.Researcher || agentprofile.Canonical(agent.Role) != agentprofile.Researcher || agent.LifecycleVersion <= 0 || strings.TrimSpace(agent.ActiveRunID) != req.RunID {
		return types.LifecycleResult{}, ErrLifecycleInvalidTransition
	}
	runObj, run, err := s.textureTurnRunObject(ctx, ownerID, computerID, req.RunID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	if run.OwnerID != ownerID || run.ComputerID != computerID || run.TrajectoryID != req.TrajectoryID || run.AgentID != req.AgentID || agentprofile.Canonical(run.AgentProfile) != agentprofile.Researcher || agentprofile.Canonical(run.AgentRole) != agentprofile.Researcher || !lifecycleRunOwnsActivation(run.State) || metadataStringValueStore(run.Metadata, "request_source") != "lifecycle_texture_control" || metadataStringValueStore(run.Metadata, "lifecycle_logical_activation_key") != req.LogicalActivationKey || metadataStringValueStore(run.Metadata, "lifecycle_failed_attempt_key") != req.FailedAttemptKey || !lifecycleControlBindingsEmpty(run.Metadata) {
		return types.LifecycleResult{}, ErrLifecycleInvalidTransition
	}
	if strings.TrimSpace(run.Prompt) != req.ActivationRefresh.Prompt || metadataStringValueStore(run.Metadata, "lifecycle_activation_build_commit") != req.ActivationRefresh.BuildCommit {
		return types.LifecycleResult{}, fmt.Errorf("lifecycle control failure activation refresh prompt/build mismatch: %w", ErrLifecycleInvalidTransition)
	}
	versionsBody, marshalErr := json.Marshal(run.Metadata["lifecycle_activation_versions"])
	if marshalErr != nil {
		return types.LifecycleResult{}, marshalErr
	}
	var runVersions []types.LifecycleControlActivationVersion
	if unmarshalErr := json.Unmarshal(versionsBody, &runVersions); unmarshalErr != nil || len(runVersions) != len(req.ActivationRefresh.Versions) {
		return types.LifecycleResult{}, fmt.Errorf("lifecycle control failure activation versions missing: %w", ErrLifecycleInvalidTransition)
	}
	for i := range runVersions {
		if runVersions[i] != req.ActivationRefresh.Versions[i] {
			return types.LifecycleResult{}, fmt.Errorf("lifecycle control failure activation version mismatch: %w", ErrLifecycleInvalidTransition)
		}
	}

	conditions := []objectgraph.ObjectCondition{
		{CanonicalID: trajectoryObj.CanonicalID, Exists: true, ExpectedContentHash: trajectoryObj.ContentHash},
		{CanonicalID: agentObj.CanonicalID, Exists: true, ExpectedContentHash: agentObj.ContentHash},
		{CanonicalID: runObj.CanonicalID, Exists: true, ExpectedContentHash: runObj.ContentHash},
	}
	seenConditions := make(map[string]bool, len(req.Controls)*2+len(conditions))
	for _, condition := range conditions {
		seenConditions[condition.CanonicalID] = true
	}
	appendCondition := func(condition objectgraph.ObjectCondition) {
		if !seenConditions[condition.CanonicalID] {
			conditions = append(conditions, condition)
			seenConditions[condition.CanonicalID] = true
		}
	}
	seenUpdates := make(map[string]bool, len(req.Controls))
	missingRunWorkBinding := false
	versions := make([]types.LifecycleControlActivationVersion, 0, len(req.Controls))
	for _, item := range req.Controls {
		if seenUpdates[item.UpdateID] {
			return types.LifecycleResult{}, ErrLifecycleInvalidTransition
		}
		seenUpdates[item.UpdateID] = true
		key := req.TrajectoryID + "\x00" + req.AgentID + "\x00" + item.ProducerAgentID + "\x00" + item.ProducerUpdateID
		updateObj, update, loadErr := s.textureTurnUpdateObject(ctx, ownerID, computerID, key)
		if loadErr != nil {
			return types.LifecycleResult{}, loadErr
		}
		if update.UpdateID != item.UpdateID || update.Direction != types.LifecyclePacketDirectionControl || update.OwnerID != ownerID || update.ComputerID != computerID || update.TrajectoryID != req.TrajectoryID || update.TargetAgentID != req.AgentID || update.TargetWorkItemID != item.TargetWorkItemID || update.LifecycleVersion != item.ExpectedControlLifecycleVersion || update.Disposition != types.UpdatePending || update.DeliveredAt != nil || strings.TrimSpace(update.DeliveredToRunID) != "" {
			return types.LifecycleResult{}, ErrConcurrentStateChange
		}
		workObj, work, loadErr := s.lifecycleWorkObject(ctx, ownerID, computerID, item.TargetWorkItemID)
		if loadErr != nil {
			return types.LifecycleResult{}, loadErr
		}
		if work.OwnerID != ownerID || work.ComputerID != computerID || work.WorkItemID != item.TargetWorkItemID || work.TrajectoryID != req.TrajectoryID || work.AssignedAgentID != req.AgentID || work.Status != types.WorkItemOpen || work.LifecycleVersion != item.ExpectedWorkLifecycleVersion {
			return types.LifecycleResult{}, ErrConcurrentStateChange
		}
		appendCondition(objectgraph.ObjectCondition{CanonicalID: updateObj.CanonicalID, Exists: true, ExpectedContentHash: updateObj.ContentHash})
		appendCondition(objectgraph.ObjectCondition{CanonicalID: workObj.CanonicalID, Exists: true, ExpectedContentHash: workObj.ContentHash})
		if !lifecycleRunBindsWork(run, item.TargetWorkItemID) {
			missingRunWorkBinding = true
		}
		versions = append(versions, types.LifecycleControlActivationVersion{UpdateID: item.UpdateID, TargetWorkItemID: item.TargetWorkItemID, ControlLifecycleVersion: item.ExpectedControlLifecycleVersion, WorkLifecycleVersion: item.ExpectedWorkLifecycleVersion})
	}
	if !missingRunWorkBinding {
		return types.LifecycleResult{}, ErrLifecycleInvalidTransition
	}

	now := time.Now().UTC()
	nextSeq := trajectory.ReducerSeq + 1
	run.Metadata = lifecycleControlFailureMetadataCopy(run.Metadata)
	run.Metadata["lifecycle_control_bind_failed"] = true
	run.Metadata["lifecycle_control_bind_failure"] = req.Failure
	run.Metadata[lifecycleControlFailureCommandIDMetadata] = req.CommandID
	run.Metadata[lifecycleControlFailureCommandDigestMetadata] = req.CommandDigest
	run.State, run.Error, run.Result = types.RunFailed, req.Failure, ""
	run.UpdatedAt, run.FinishedAt = now, &now
	runBody, err := json.Marshal(run)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	runMetadata := map[string]any{
		"run_id": run.RunID, "agent_id": run.AgentID, "channel_id": run.ChannelID,
		"requested_by_run_id": run.RequestedByRunID, "trajectory_id": run.TrajectoryID,
		"agent_profile": run.AgentProfile, "agent_role": run.AgentRole, "computer_id": run.ComputerID,
		"state": string(run.State), "created_at": run.CreatedAt.UTC().Format(time.RFC3339Nano), "updated_at": now.Format(time.RFC3339Nano),
	}
	runMetadataJSON, err := objectgraph.NormalizeMetadata(runMetadata)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	runUpdated := objectgraph.Object{CanonicalID: runObj.CanonicalID, ObjectKind: ogKindRun, OwnerID: ownerID, ComputerID: computerID, ContentHash: objectgraph.ContentHash(ogKindRun, runBody, runMetadataJSON), Body: runBody, Metadata: runMetadataJSON, CreatedAt: runObj.CreatedAt, UpdatedAt: now}

	agent.ActiveRunID, agent.UpdatedAt = "", now
	agent.LifecycleVersion++
	agent.LastReducerSeq = nextSeq
	agentUpdated, err := lifecycleObject(ogKindAgent, ownerID, computerID, agent.AgentID, agent, lifecycleMetadata("agent_id", agent.AgentID, computerID, req.TrajectoryID, nextSeq), agentObj.CreatedAt, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	trajectory.ReducerSeq, trajectory.LifecycleVersion, trajectory.UpdatedAt = nextSeq, trajectory.LifecycleVersion+1, now
	trajectoryUpdated, err := lifecycleObject(ogKindTrajectory, ownerID, computerID, req.TrajectoryID, trajectory, lifecycleMetadata("trajectory_id", req.TrajectoryID, computerID, req.TrajectoryID, nextSeq), trajectoryObj.CreatedAt, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	event := types.LifecycleEvent{EventID: req.CommandID + ":1", OwnerID: ownerID, ComputerID: computerID, TrajectoryID: req.TrajectoryID, RunID: req.RunID, AgentID: req.AgentID, Kind: types.LifecycleControlActivationFailed, ReducerVersion: types.LifecycleReducerVersion, ReducerSeq: nextSeq, CommandID: req.CommandID, CommandDigest: req.CommandDigest, LogicalActivationKey: req.LogicalActivationKey, FailedAttemptKey: req.FailedAttemptKey, BindCommandID: req.BindCommandID, BindCommandDigest: req.BindCommandDigest, ControlVersions: versions, Reason: req.Failure, CreatedAt: now}
	eventObj, err := lifecycleObject(ogKindLifecycleEvent, ownerID, computerID, event.EventID, event, lifecycleMetadata("event_id", event.EventID, computerID, req.TrajectoryID, nextSeq), now, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	receipt, receiptObj, err := s.lifecycleTransitionReceipt(now, ownerID, computerID, req.TrajectoryID, req.CommandID, req.CommandDigest, types.LifecycleFailControlActivation, nextSeq, []objectgraph.Object{eventObj})
	if err != nil {
		return types.LifecycleResult{}, err
	}
	appendCondition(objectgraph.ObjectCondition{CanonicalID: eventObj.CanonicalID})
	appendCondition(objectgraph.ObjectCondition{CanonicalID: receiptObj.CanonicalID})
	objects := []objectgraph.Object{runUpdated, agentUpdated, trajectoryUpdated, eventObj, receiptObj}
	return s.commitLifecycleTransition(ctx, ownerID, computerID, req.CommandID, req.CommandDigest, conditions, objects, types.LifecycleResult{Receipt: receipt, Trajectory: trajectory, Agent: &agent, Events: []types.LifecycleEvent{event}})
}

func isLifecycleControlActivationFailureEvidence(result types.LifecycleResult, run types.RunRecord, trajectoryID, agentID, logicalKey, failedAttemptKey string, versions []types.LifecycleControlActivationVersion) bool {
	if result.Receipt.Kind != types.LifecycleFailControlActivation || result.Receipt.CommandID != metadataStringValueStore(run.Metadata, lifecycleControlFailureCommandIDMetadata) || result.Receipt.CommandDigest != metadataStringValueStore(run.Metadata, lifecycleControlFailureCommandDigestMetadata) || result.Receipt.TrajectoryID != trajectoryID || len(result.Events) != 1 {
		return false
	}
	event := result.Events[0]
	if event.Kind != types.LifecycleControlActivationFailed || event.CommandID != result.Receipt.CommandID || event.CommandDigest != result.Receipt.CommandDigest || event.TrajectoryID != trajectoryID || event.RunID != run.RunID || event.AgentID != agentID || event.LogicalActivationKey != logicalKey || event.FailedAttemptKey != failedAttemptKey || event.Reason != LifecycleControlActivationMissingRunWorkBindingFailure || len(event.ControlVersions) != len(versions) {
		return false
	}
	for i := range versions {
		if event.ControlVersions[i] != versions[i] {
			return false
		}
	}
	return true
}

func (s *Store) resolveLifecycleControlActivationFailure(ctx context.Context, ownerID, computerID, trajectoryID, agentID, logicalKey, failedAttemptKey string, versions []types.LifecycleControlActivationVersion) (*types.RunRecord, error) {
	commandID := LifecycleControlActivationFailureCommandID(failedAttemptKey)
	commandObj, err := s.lifecycleGetObject(ctx, ogKindLifecycleCmd, ownerID, computerID, commandID)
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	receipt, err := decodeLifecycleObject[types.LifecycleCommandReceipt](commandObj)
	if err != nil {
		return nil, err
	}
	if receipt.CommandID != commandID || receipt.Kind != types.LifecycleFailControlActivation || receipt.OwnerID != ownerID || receipt.ComputerID != computerID || receipt.TrajectoryID != trajectoryID || receipt.StoredResult == nil || len(receipt.StoredResult.Events) != 1 {
		return nil, ErrLifecycleInvalidTransition
	}
	event := receipt.StoredResult.Events[0]
	run, err := s.GetLifecycleRun(ctx, ownerID, computerID, event.RunID)
	if err != nil {
		return nil, err
	}
	storedReceipt := receipt
	storedReceipt.StoredResult = nil
	result := types.LifecycleResult{Receipt: storedReceipt, Events: []types.LifecycleEvent{event}}
	if !isLifecycleControlActivationFailureEvidence(result, run, trajectoryID, agentID, logicalKey, failedAttemptKey, versions) {
		return nil, ErrLifecycleInvalidTransition
	}
	failed, _ := run.Metadata["lifecycle_control_bind_failed"].(bool)
	if run.State != types.RunFailed || !failed || metadataStringValueStore(run.Metadata, "lifecycle_failed_attempt_key") != failedAttemptKey {
		return nil, ErrLifecycleInvalidTransition
	}
	return &run, nil
}

func (s *Store) lifecycleControlActivationFailureEvidence(ctx context.Context, run types.RunRecord, trajectoryID, agentID, logicalKey, failedAttemptKey string, versions []types.LifecycleControlActivationVersion) (bool, error) {
	commandID := metadataStringValueStore(run.Metadata, lifecycleControlFailureCommandIDMetadata)
	digest := metadataStringValueStore(run.Metadata, lifecycleControlFailureCommandDigestMetadata)
	if commandID == "" || digest == "" {
		return false, nil
	}
	result, found, err := s.replayLifecycleCommand(ctx, run.OwnerID, run.ComputerID, commandID, digest)
	if err != nil || !found {
		return false, err
	}
	return isLifecycleControlActivationFailureEvidence(result, run, trajectoryID, agentID, logicalKey, failedAttemptKey, versions), nil
}
