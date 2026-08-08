package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

const runMetadataWorkerUpdatesInjected = "worker_updates_injected"

// reconcilePersistentSuperActor is the durable controller boundary for the
// user's privileged execution actor. update_coagent can append addressed work
// for the persistent super, but only this runtime controller starts or reuses
// the super execution loop that drains those durable updates.
func (rt *Runtime) reconcilePersistentSuperActor(ctx context.Context, ownerID, agentID string) (*types.RunRecord, error) {
	ownerID = strings.TrimSpace(ownerID)
	agentID = strings.TrimSpace(agentID)
	if ownerID == "" {
		return nil, fmt.Errorf("owner_id is required")
	}
	if agentID == "" {
		agentID = persistentSuperAgentID(ownerID)
	}
	var blockedActive *types.RunRecord
	if resident, found, err := rt.activeRunByAgent(ctx, ownerID, agentID); err != nil {
		return nil, fmt.Errorf("check resident super run: %w", err)
	} else if found {
		return &resident, nil
	}
	if active, err := rt.latestActiveRunByAgent(ctx, ownerID, agentID); err == nil {
		if active.State == types.RunBlocked {
			blockedActive = &active
		}
	} else if !errors.Is(err, store.ErrNotFound) {
		return nil, fmt.Errorf("check blocked super run: %w", err)
	}

	computerID := strings.TrimSpace(rt.TextureSandboxID())
	updates, err := rt.listPendingPersistentSuperLifecycleControls(ctx, ownerID, computerID, agentID, 100)
	updates = selectLifecycleControlActivation(updates, "", nil)
	lifecycleControls := len(updates) > 0
	if err != nil {
		return nil, err
	}
	if !lifecycleControls {
		updates, err = rt.listAndSettlePersistentSuperBacklog(ctx, ownerID, agentID)
		if err != nil {
			return nil, err
		}
		updates = filterPersistentSuperExecutionUpdates(updates)
	}
	if len(updates) == 0 {
		if blockedActive != nil {
			return blockedActive, nil
		}
		return nil, nil
	}

	first := updates[0]
	requestSource := "update_coagent"
	if lifecycleControls {
		requestSource = "lifecycle_texture_control"
	}
	metadata := map[string]any{
		runMetadataAgentProfile: agentprofile.Super,
		runMetadataAgentRole:    agentprofile.Super,
		runMetadataAgentID:      agentID,
		"request_source":        requestSource,
		"requested_by_agent_id": first.AgentID,
		"requested_by_profile":  strings.TrimSpace(first.Role),
	}
	if first.ChannelID != "" {
		metadata[runMetadataChannelID] = first.ChannelID
	}
	if first.TrajectoryID != "" {
		metadata[runMetadataTrajectoryID] = first.TrajectoryID
	}
	targetWorkItemID := firstNonEmpty(first.TargetWorkItemID, first.WorkItemID)
	if targetWorkItemID != "" {
		metadata["lifecycle_work_item_id"] = targetWorkItemID
		workIDs, seenWork := make([]string, 0, len(updates)), map[string]bool{}
		for _, update := range updates {
			if id := firstNonEmpty(update.TargetWorkItemID, update.WorkItemID); id != "" && !seenWork[id] {
				seenWork[id] = true
				workIDs = append(workIDs, id)
			}
		}
		metadata["work_item_ids"] = workIDs
	}
	updateIDs := make([]string, 0, len(updates))
	for _, update := range updates {
		if id := strings.TrimSpace(update.UpdateID); id != "" {
			updateIDs = append(updateIDs, id)
		}
	}
	if len(updateIDs) > 0 && !lifecycleControls {
		metadata["worker_update_ids"] = updateIDs
	}
	if first.AgentID != "" {
		if requester, err := rt.latestActiveRunByAgent(ctx, ownerID, first.AgentID); err == nil {
			metadata["requested_by_run_id"] = requester.RunID
			if metadataStringValue(requester.Metadata, "agent_profile") != "" && metadata["requested_by_profile"] == "" {
				metadata["requested_by_profile"] = metadataStringValue(requester.Metadata, "agent_profile")
			}
			if desktopID := metadataStringValue(requester.Metadata, runMetadataDesktopID); desktopID != "" {
				metadata[runMetadataDesktopID] = desktopID
			}
		}
	}

	rec, err := rt.createRunWithMetadata(ctx, "Process pending coagent update packets for privileged execution.", ownerID, metadata)
	if err != nil {
		return nil, err
	}
	if lifecycleControls {
		if _, err := rt.bindLifecycleControlsToRun(ctx, rec, updates); err != nil {
			return nil, err
		}
	}
	rt.activate(rec)
	return rec, nil
}

func (rt *Runtime) markPersistentSuperRunUpdatesDelivered(ctx context.Context, rec *types.RunRecord) error {
	if rec == nil || strings.TrimSpace(rec.OwnerID) == "" || strings.TrimSpace(rec.RunID) == "" {
		return nil
	}
	updateIDs := coagentUpdateIDsForRun(rec)
	if len(updateIDs) == 0 {
		return nil
	}
	if err := rt.store.MarkWorkerUpdatesDelivered(ctx, rec.OwnerID, rec.AgentID, updateIDs, rec.RunID); err != nil {
		return fmt.Errorf("mark persistent super updates delivered: %w", err)
	}
	return nil
}

func coagentUpdateIDsForRun(rec *types.RunRecord) []string {
	if rec == nil {
		return nil
	}
	if !runSupportsCoagentUpdateInjection(rec) {
		return nil
	}
	if metadataStringValue(rec.Metadata, "request_source") != "update_coagent" &&
		!metadataBoolValue(rec.Metadata, runMetadataWorkerUpdatesInjected) {
		return nil
	}
	return metadataStringSlice(rec.Metadata["worker_update_ids"])
}

func appendCoagentUpdateIDsForRun(rec *types.RunRecord, updateIDs []string) {
	if rec == nil || len(updateIDs) == 0 {
		return
	}
	if rec.Metadata == nil {
		rec.Metadata = map[string]any{}
	}
	seen := map[string]bool{}
	merged := make([]string, 0, len(updateIDs))
	for _, id := range metadataStringSlice(rec.Metadata["worker_update_ids"]) {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		merged = append(merged, id)
	}
	for _, id := range updateIDs {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		merged = append(merged, id)
	}
	rec.Metadata["worker_update_ids"] = merged
	rec.Metadata[runMetadataWorkerUpdatesInjected] = true
}

func (rt *Runtime) updateRunAndMarkSuccessfulCoagentActivationDelivered(ctx context.Context, rec *types.RunRecord) error {
	if rec == nil {
		return nil
	}
	updateIDs := coagentUpdateIDsForRun(rec)
	if runHasProfile(rec, agentprofile.Texture) || metadataStringValue(rec.Metadata, "request_source") == "lifecycle_texture_control" {
		if err := rt.store.UpdateRun(ctx, *rec); err != nil {
			return err
		}
		return rt.completeSuccessfulRunWorkItems(ctx, rec)
	}
	if len(updateIDs) == 0 || rec.State != types.RunCompleted {
		if err := rt.store.UpdateRun(ctx, *rec); err != nil {
			return err
		}
		return rt.completeSuccessfulRunWorkItems(ctx, rec)
	}
	if err := rt.store.UpdateRunAndMarkWorkerUpdatesDelivered(ctx, *rec, rec.OwnerID, updateIDs); err != nil {
		return err
	}
	return rt.completeSuccessfulRunWorkItems(ctx, rec)
}

func (rt *Runtime) completeSuccessfulRunWorkItems(ctx context.Context, rec *types.RunRecord) error {
	if rt == nil || rt.store == nil || rec == nil || rec.State != types.RunCompleted {
		return nil
	}
	ownerID := strings.TrimSpace(rec.OwnerID)
	if ownerID == "" {
		return nil
	}
	if strings.TrimSpace(rec.SandboxID) != "" && strings.TrimSpace(rec.TrajectoryID) != "" {
		if _, err := rt.store.GetLifecycleTrajectory(ctx, ownerID, rec.SandboxID, rec.TrajectoryID); err == nil {
			return nil
		} else if !errors.Is(err, store.ErrNotFound) {
			return err
		}
	}
	for _, workItemID := range metadataStringSlice(rec.Metadata["work_item_ids"]) {
		workItemID = strings.TrimSpace(workItemID)
		if workItemID == "" {
			continue
		}
		if _, err := rt.store.UpdateWorkItemStatus(ctx, ownerID, workItemID, types.WorkItemCompleted); err != nil {
			return fmt.Errorf("complete run work item %s: %w", workItemID, err)
		}
	}
	return nil
}

func (rt *Runtime) maybeContinuePersistentSuperInbox(ctx context.Context, rec *types.RunRecord) {
	if !isPersistentSuperInboxRun(rec) {
		return
	}
	if rec.State != types.RunCompleted {
		return
	}
	if err := rt.markPersistentSuperRunUpdatesDelivered(ctx, rec); err != nil {
		log.Printf("runtime: mark persistent super updates delivered after %s: %v", rec.RunID, err)
		return
	}
	if _, err := rt.reconcilePersistentSuperActor(ctx, rec.OwnerID, rec.AgentID); err != nil {
		log.Printf("runtime: continue persistent super inbox after %s: %v", rec.RunID, err)
	}
}

func isPersistentSuperInboxRun(rec *types.RunRecord) bool {
	if rec == nil {
		return false
	}
	if !isPersistentSuperAgentRun(rec) {
		return false
	}
	if metadataStringValue(rec.Metadata, "request_source") != "update_coagent" {
		return false
	}
	return true
}

func isPersistentSuperAgentRun(rec *types.RunRecord) bool {
	if rec == nil {
		return false
	}
	if agentProfileForRun(rec) != agentprofile.Super {
		return false
	}
	if strings.TrimSpace(rec.OwnerID) == "" || strings.TrimSpace(rec.AgentID) == "" {
		return false
	}
	return rec.AgentID == persistentSuperAgentID(rec.OwnerID)
}

func filterPersistentSuperExecutionUpdates(updates []types.CoagentSourcePacket) []types.CoagentSourcePacket {
	if len(updates) == 0 {
		return nil
	}
	out := make([]types.CoagentSourcePacket, 0, len(updates))
	for _, update := range updates {
		if persistentSuperExecutableUpdate(update) {
			out = append(out, update)
		}
	}
	return out
}

func (rt *Runtime) listAndSettlePersistentSuperBacklog(ctx context.Context, ownerID, agentID string) ([]types.CoagentSourcePacket, error) {
	const limit = 100
	for i := 0; i < 10; i++ {
		updates, err := rt.store.ListCoagentMailboxBacklog(ctx, ownerID, agentID, limit)
		if err != nil {
			return nil, fmt.Errorf("list super pending updates: %w", err)
		}
		settled, err := rt.settlePersistentSuperNonExecutionUpdates(ctx, ownerID, agentID, updates)
		if err != nil {
			return nil, fmt.Errorf("settle non-execution super updates: %w", err)
		}
		if !settled {
			return updates, nil
		}
	}
	return nil, fmt.Errorf("settle non-execution super updates: mailbox did not converge")
}

func (rt *Runtime) settlePersistentSuperNonExecutionUpdates(ctx context.Context, ownerID, agentID string, updates []types.CoagentSourcePacket) (bool, error) {
	var nonExecIDs []string
	for _, u := range updates {
		if u.DeliveredAt != nil || strings.TrimSpace(u.DeliveredToRunID) != "" {
			continue
		}
		if !persistentSuperExecutableUpdate(u) {
			if id := strings.TrimSpace(u.UpdateID); id != "" {
				nonExecIDs = append(nonExecIDs, id)
			}
		}
	}
	if len(nonExecIDs) == 0 {
		return false, nil
	}
	if err := rt.store.MarkWorkerUpdatesDelivered(ctx, ownerID, agentID, nonExecIDs, "settled_non_executable"); err != nil {
		return false, fmt.Errorf("mark non-execution updates settled: %w", err)
	}
	return true, nil
}

func lifecycleControlWorkIDsForRun(rec *types.RunRecord) map[string]bool {
	out := map[string]bool{}
	if rec == nil {
		return out
	}
	if id := strings.TrimSpace(metadataStringValue(rec.Metadata, "lifecycle_work_item_id")); id != "" {
		out[id] = true
	}
	for _, id := range metadataStringSlice(rec.Metadata["work_item_ids"]) {
		if id = strings.TrimSpace(id); id != "" {
			out[id] = true
		}
	}
	return out
}

func selectLifecycleControlActivation(updates []types.CoagentSourcePacket, trajectoryID string, workIDs map[string]bool) []types.CoagentSourcePacket {
	if len(updates) == 0 {
		return nil
	}
	trajectoryID = strings.TrimSpace(trajectoryID)
	if trajectoryID == "" {
		trajectoryID = strings.TrimSpace(updates[0].TrajectoryID)
	}
	out := make([]types.CoagentSourcePacket, 0, len(updates))
	for _, update := range updates {
		if strings.TrimSpace(update.TrajectoryID) != trajectoryID {
			continue
		}
		if len(workIDs) > 0 && !workIDs[strings.TrimSpace(update.TargetWorkItemID)] {
			continue
		}
		out = append(out, update)
	}
	return out
}

func (rt *Runtime) bindLifecycleControlsToRun(ctx context.Context, rec *types.RunRecord, updates []types.CoagentSourcePacket) (types.LifecycleResult, error) {
	if rt == nil || rt.store == nil || rec == nil || len(updates) == 0 {
		return types.LifecycleResult{}, nil
	}
	updates = selectLifecycleControlActivation(updates, rec.TrajectoryID, lifecycleControlWorkIDsForRun(rec))
	if len(updates) == 0 {
		return types.LifecycleResult{}, fmt.Errorf("bind lifecycle controls: no exact run/trajectory/work controls")
	}
	snapshot, err := rt.store.GetLifecycleSnapshot(ctx, rec.OwnerID, rec.SandboxID, rec.TrajectoryID)
	if err != nil {
		return types.LifecycleResult{}, fmt.Errorf("bind lifecycle controls snapshot: %w", err)
	}
	items := make([]types.BindLifecycleControlDeliveryItem, 0, len(updates))
	ids := make([]string, 0, len(updates))
	for _, update := range updates {
		items = append(items, types.BindLifecycleControlDeliveryItem{UpdateID: update.UpdateID, ProducerAgentID: update.AgentID, ProducerUpdateID: update.ProducerUpdateID, TargetWorkItemID: update.TargetWorkItemID})
		ids = append(ids, update.UpdateID)
	}
	commandID := "bind-control-delivery:" + rec.RunID + ":" + strings.Join(ids, ",")
	req := types.BindLifecycleControlDeliveryRequest{OwnerID: rec.OwnerID, ComputerID: rec.SandboxID, CommandID: commandID, TrajectoryID: rec.TrajectoryID, TargetAgentID: rec.AgentID, TargetRunID: rec.RunID, ExpectedLifecycleVersion: snapshot.Trajectory.LifecycleVersion, Controls: items}
	req.CommandDigest, err = store.ComputeBindLifecycleControlDeliveryDigest(req)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	return rt.store.BindLifecycleControlDelivery(ctx, req)
}

func (rt *Runtime) listPendingPersistentSuperLifecycleControls(ctx context.Context, ownerID, computerID, agentID string, limit int) ([]types.CoagentSourcePacket, error) {
	ownerID, computerID, agentID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID), strings.TrimSpace(agentID)
	if ownerID == "" || computerID == "" || agentID != persistentSuperAgentID(ownerID) {
		return nil, fmt.Errorf("list persistent-Super lifecycle controls: exact owner, computer, and persistent agent are required")
	}
	agent, err := rt.store.GetAgentByScope(ctx, ownerID, computerID, agentID)
	if err != nil {
		return nil, fmt.Errorf("load exact persistent Super: %w", err)
	}
	if agentprofile.Canonical(agent.Profile) != agentprofile.Super || agentprofile.Canonical(agent.Role) != agentprofile.Super || agent.LifecycleVersion != 0 || agent.OwnerID != ownerID || agent.ComputerID != computerID {
		return nil, fmt.Errorf("persistent Super lifecycle control target has invalid authority")
	}
	updates, err := rt.store.ListPendingLifecycleUpdates(ctx, ownerID, computerID, agentID, limit)
	if err != nil {
		return nil, fmt.Errorf("list persistent-Super lifecycle controls: %w", err)
	}
	return rt.validateTargetBoundLifecycleControls(ctx, ownerID, computerID, agentID, updates, true)
}

func (rt *Runtime) validateTargetBoundLifecycleControls(ctx context.Context, ownerID, computerID, agentID string, updates []types.CoagentSourcePacket, executionOnly bool) ([]types.CoagentSourcePacket, error) {
	out := make([]types.CoagentSourcePacket, 0, len(updates))
	for _, update := range updates {
		if update.Direction != types.LifecyclePacketDirectionControl || update.TargetAgentID != agentID || update.OwnerID != ownerID || update.ComputerID != computerID || update.Disposition != types.UpdatePending || strings.TrimSpace(update.TargetWorkItemID) == "" || strings.TrimSpace(update.TrajectoryID) == "" {
			return nil, fmt.Errorf("pending lifecycle control %q has ambiguous target binding", update.UpdateID)
		}
		snapshot, err := rt.store.GetLifecycleSnapshot(ctx, ownerID, computerID, update.TrajectoryID)
		if err != nil {
			return nil, fmt.Errorf("load lifecycle control trajectory %q: %w", update.UpdateID, err)
		}
		bound := false
		for _, work := range snapshot.WorkItems {
			if work.WorkItemID == update.TargetWorkItemID && work.Status == types.WorkItemOpen && work.AssignedAgentID == agentID && work.TrajectoryID == update.TrajectoryID && work.OwnerID == ownerID && work.ComputerID == computerID {
				bound = true
				break
			}
		}
		if !bound {
			return nil, fmt.Errorf("pending lifecycle control %q is not joined to exact open target work", update.UpdateID)
		}
		if executionOnly && !persistentSuperExecutableUpdate(update) {
			return nil, fmt.Errorf("persistent-Super lifecycle control %q is not an execution request", update.UpdateID)
		}
		out = append(out, update)
	}
	return out, nil
}

func persistentSuperExecutableUpdate(update types.CoagentSourcePacket) bool {
	if update.DeliveredAt != nil || strings.TrimSpace(update.DeliveredToRunID) != "" {
		return false
	}
	packet := normalizeCoagentSourcePacketPayload(update.Packet)
	if packet.Kind != "execution_request" {
		return false
	}
	return validateCoagentSourcePacketPayload(packet) == nil
}

func coagentUpdateDeliverableForRun(rec *types.RunRecord, update types.CoagentSourcePacket) bool {
	if isPersistentSuperAgentRun(rec) {
		return persistentSuperExecutableUpdate(update)
	}
	return true
}

func buildPersistentSuperUpdatePrompt(updates []types.CoagentSourcePacket) string {
	var b strings.Builder
	b.WriteString("Process the pending update_coagent records addressed to you as the user's persistent super actor.\n\n")
	b.WriteString("Each delivered packet is a validated packet.kind=execution_request with executable actions. When you have command output, diffs, tests, artifacts, questions, or blockers, report them back with update_coagent as packet.sources, claims, actions, questions, and notes.\n")
	for i, update := range updates {
		b.WriteString("\nUpdate ")
		b.WriteString(fmt.Sprintf("%d", i+1))
		if update.ChannelID != "" {
			b.WriteString(" channel=")
			b.WriteString(update.ChannelID)
		}
		if update.AgentID != "" {
			b.WriteString(" from=")
			b.WriteString(update.AgentID)
		}
		if kind := coagentPacketKind(update.Packet); kind != "" {
			b.WriteString(" kind=")
			b.WriteString(kind)
		}
		b.WriteString(":\n")
		b.WriteString(strings.TrimSpace(update.Content))
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func (rt *Runtime) reconcileUpdatedCoagentActor(ctx context.Context, ownerID, agentID string) (*types.RunRecord, error) {
	ownerID = strings.TrimSpace(ownerID)
	agentID = strings.TrimSpace(agentID)
	if ownerID == "" || agentID == "" {
		return nil, nil
	}
	if isTextureAgentID(agentID) {
		return nil, nil
	}
	if resident, found, err := rt.activeRunByAgent(ctx, ownerID, agentID); err != nil {
		return nil, fmt.Errorf("check resident coagent run: %w", err)
	} else if found {
		return &resident, nil
	}
	agent, err := rt.store.GetAgentByScope(ctx, ownerID, rt.TextureSandboxID(), agentID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("lookup coagent: %w", err)
	}
	profile := agentprofile.Canonical(firstNonEmpty(agent.Profile, agent.Role))
	lifecycleControls := false
	var updates []types.CoagentSourcePacket
	if profile == agentprofile.Researcher && agent.LifecycleVersion > 0 {
		updates, err = rt.store.ListPendingLifecycleUpdates(ctx, ownerID, rt.TextureSandboxID(), agentID, 100)
		if err == nil {
			updates, err = rt.validateTargetBoundLifecycleControls(ctx, ownerID, rt.TextureSandboxID(), agentID, updates, false)
		}
		updates = selectLifecycleControlActivation(updates, "", nil)
		lifecycleControls = len(updates) > 0
	}
	if err != nil {
		return nil, err
	}
	if !lifecycleControls {
		updates, err = rt.store.ListCoagentMailboxBacklog(ctx, ownerID, agentID, 100)
		if err != nil {
			return nil, fmt.Errorf("list coagent pending updates: %w", err)
		}
	}
	if len(updates) == 0 {
		return nil, nil
	}
	first := updates[0]
	if profile == "" || profile == agentprofile.Email || profile == agentprofile.Conductor || profile == agentprofile.Super {
		return nil, nil
	}
	role := strings.TrimSpace(firstNonEmpty(agent.Role, profile))
	channelID := strings.TrimSpace(firstNonEmpty(agent.ChannelID, first.ChannelID))
	updateIDs := make([]string, 0, len(updates))
	for _, update := range updates {
		if id := strings.TrimSpace(update.UpdateID); id != "" {
			updateIDs = append(updateIDs, id)
		}
	}
	requestSource := "update_coagent"
	if lifecycleControls {
		requestSource = "lifecycle_texture_control"
	}
	metadata := map[string]any{
		runMetadataAgentProfile: profile,
		runMetadataAgentRole:    role,
		runMetadataAgentID:      agentID,
		"request_source":        requestSource,
	}
	if !lifecycleControls {
		metadata["worker_update_ids"] = updateIDs
	}
	if channelID != "" {
		metadata[runMetadataChannelID] = channelID
	}
	if first.TrajectoryID != "" {
		metadata[runMetadataTrajectoryID] = first.TrajectoryID
	}
	workItems, err := rt.assignedOpenWorkItemsForAgentUpdateBacklog(ctx, ownerID, agentID, updates)
	if err != nil {
		return nil, err
	}
	if workItemIDs := workItemIDsForMetadata(workItems); len(workItemIDs) > 0 {
		metadata["work_item_ids"] = workItemIDs
	}
	prompt := "Continue assigned actor work. Process the coagent update packets in context."
	if workPrompt := buildAssignedWorkItemPrompt(workItems); workPrompt != "" {
		prompt = prompt + "\n\n" + workPrompt
	}
	rec, err := rt.createRunWithMetadata(ctx, prompt, ownerID, metadata)
	if err != nil {
		return nil, err
	}
	if lifecycleControls {
		if _, err := rt.bindLifecycleControlsToRun(ctx, rec, updates); err != nil {
			return nil, err
		}
	}
	rt.activate(rec)
	return rec, nil
}

func (rt *Runtime) assignedOpenWorkItemsForAgentUpdateBacklog(ctx context.Context, ownerID, agentID string, updates []types.CoagentSourcePacket) ([]types.WorkItemRecord, error) {
	seenTrajectories := map[string]bool{}
	var out []types.WorkItemRecord
	for _, update := range updates {
		trajectoryID := strings.TrimSpace(update.TrajectoryID)
		if trajectoryID == "" || seenTrajectories[trajectoryID] {
			continue
		}
		seenTrajectories[trajectoryID] = true
		items, err := rt.assignedOpenWorkItemsForAgentTrajectory(ctx, ownerID, agentID, trajectoryID)
		if err != nil {
			return nil, err
		}
		out = append(out, items...)
	}
	return out, nil
}

func (rt *Runtime) assignedOpenWorkItemsForAgentTrajectory(ctx context.Context, ownerID, agentID, trajectoryID string) ([]types.WorkItemRecord, error) {
	ownerID = strings.TrimSpace(ownerID)
	agentID = strings.TrimSpace(agentID)
	trajectoryID = strings.TrimSpace(trajectoryID)
	if rt == nil || rt.store == nil || ownerID == "" || agentID == "" || trajectoryID == "" {
		return nil, nil
	}
	items, err := rt.store.ListWorkItemsByTrajectory(ctx, ownerID, trajectoryID, true)
	if err != nil {
		return nil, fmt.Errorf("list assigned open work items for coagent wake: %w", err)
	}
	out := make([]types.WorkItemRecord, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.AssignedAgentID) == agentID {
			out = append(out, item)
		}
	}
	return out, nil
}

func workItemIDsForMetadata(workItems []types.WorkItemRecord) []string {
	ids := make([]string, 0, len(workItems))
	seen := map[string]bool{}
	for _, item := range workItems {
		id := strings.TrimSpace(item.WorkItemID)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	return ids
}

func buildCoagentBacklogPrompt(updates []types.CoagentSourcePacket, workItems []types.WorkItemRecord) string {
	updatePrompt := buildCoagentUpdatePrompt(updates)
	if len(workItems) == 0 {
		return updatePrompt
	}
	workPrompt := buildAssignedWorkItemPrompt(workItems)
	if updatePrompt == "" {
		return workPrompt
	}
	if workPrompt == "" {
		return updatePrompt
	}
	return updatePrompt + "\n\n" + workPrompt
}

func buildCoagentUpdatePrompt(updates []types.CoagentSourcePacket) string {
	var b strings.Builder
	b.WriteString("Process the pending update_coagent records addressed to you.\n")
	b.WriteString("Respond with the appropriate tool or final result for your role; report blockers with update_coagent when you cannot proceed.\n")
	for i, update := range updates {
		b.WriteString("\nUpdate ")
		b.WriteString(fmt.Sprintf("%d", i+1))
		if update.ChannelID != "" {
			b.WriteString(" channel=")
			b.WriteString(update.ChannelID)
		}
		if update.AgentID != "" {
			b.WriteString(" from=")
			b.WriteString(update.AgentID)
		}
		if kind := coagentPacketKind(update.Packet); kind != "" {
			b.WriteString(" kind=")
			b.WriteString(kind)
		}
		b.WriteString(":\n")
		b.WriteString(strings.TrimSpace(update.Content))
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func (rt *Runtime) coagentUpdateTurnInjector(rec *types.RunRecord) toolregistry.InjectUserTurnsFunc {
	return rt.coagentUpdateTurnInjectorWithInitialPhase(rec, "")
}

func (rt *Runtime) pendingCoagentUpdatesForRun(ctx context.Context, rec *types.RunRecord, ownerID, agentID string, limit int) ([]types.CoagentSourcePacket, error) {
	lifecycleRun := false
	if rec != nil && strings.TrimSpace(rec.OwnerID) != "" && strings.TrimSpace(rec.SandboxID) != "" && strings.TrimSpace(rec.RunID) != "" {
		if _, err := rt.store.GetLifecycleRun(ctx, rec.OwnerID, rec.SandboxID, rec.RunID); err == nil {
			lifecycleRun = true
		} else if !errors.Is(err, store.ErrNotFound) {
			return nil, fmt.Errorf("resolve lifecycle run authority: %w", err)
		}
	}
	computerID := strings.TrimSpace(rec.SandboxID)
	if isPersistentSuperAgentRun(rec) {
		if computerID != "" {
			controls, err := rt.listPendingPersistentSuperLifecycleControls(ctx, ownerID, computerID, agentID, limit)
			if err != nil {
				return nil, err
			}
			controls = selectLifecycleControlActivation(controls, rec.TrajectoryID, lifecycleControlWorkIDsForRun(rec))
			if len(controls) > 0 {
				return controls, nil
			}
		}
		return rt.store.ListCoagentMailboxBacklog(ctx, ownerID, agentID, limit)
	}
	if lifecycleRun {
		if computerID == "" {
			return nil, fmt.Errorf("list pending lifecycle updates: computer_id is required")
		}
		updates, err := rt.store.ListPendingLifecycleUpdates(ctx, ownerID, computerID, agentID, limit)
		if err != nil {
			return nil, err
		}
		if agentProfileForRun(rec) == agentprofile.Researcher {
			controls, validateErr := rt.validateTargetBoundLifecycleControls(ctx, ownerID, computerID, agentID, updates, false)
			if validateErr != nil {
				return nil, validateErr
			}
			return selectLifecycleControlActivation(controls, rec.TrajectoryID, lifecycleControlWorkIDsForRun(rec)), nil
		}
		return updates, nil
	}
	return rt.store.ListCoagentMailboxBacklog(ctx, ownerID, agentID, limit)
}

func (rt *Runtime) coagentUpdateTurnInjectorWithInitialPhase(rec *types.RunRecord, initialPhase string) toolregistry.InjectUserTurnsFunc {
	if rt == nil || rt.store == nil || rec == nil || !runSupportsCoagentUpdateInjection(rec) {
		return nil
	}
	// Texture participates in the same warm-injection path as the other durable
	// actors. A Texture activation may now incorporate an addressed packet, write
	// a canonical revision, then receive later packets and write deeper revisions
	// in the same logical actor run.
	ownerID := strings.TrimSpace(rec.OwnerID)
	agentID := strings.TrimSpace(rec.AgentID)
	if ownerID == "" || agentID == "" {
		return nil
	}
	initialPhase = strings.TrimSpace(initialPhase)
	seen := map[string]bool{}
	for _, id := range coagentUpdateIDsForRun(rec) {
		id = strings.TrimSpace(id)
		if id != "" {
			seen[id] = true
		}
	}
	return func(finalCheckpoint bool) ([]json.RawMessage, error) {
		updates, err := rt.pendingCoagentUpdatesForRun(context.Background(), rec, ownerID, agentID, 100)
		if err != nil {
			return nil, fmt.Errorf("list pending update_coagent turns: %w", err)
		}
		fresh := make([]types.CoagentSourcePacket, 0, len(updates))
		updateIDs := make([]string, 0, len(updates))
		for _, update := range updates {
			id := strings.TrimSpace(update.UpdateID)
			if id == "" || seen[id] {
				continue
			}
			if !coagentUpdateDeliverableForRun(rec, update) {
				continue
			}
			seen[id] = true
			fresh = append(fresh, update)
			updateIDs = append(updateIDs, id)
		}
		if len(fresh) == 0 {
			return nil, nil
		}
		appendCoagentUpdateIDsForRun(rec, updateIDs)
		phase := coagentPacketDeliveryMid
		if finalCheckpoint {
			phase = coagentPacketDeliveryFinal
		} else if initialPhase != "" {
			phase = initialPhase
			initialPhase = ""
		}
		projected, err := rt.projectTerminalOutcomeContent(context.Background(), fresh)
		if err != nil {
			return nil, err
		}
		msgs, _, err := buildCoagentUpdateUserMessages(projected, phase, agentID, nil, nil)
		if err != nil {
			return nil, err
		}
		return msgs, nil
	}
}

func shouldAppendInitialCoagentMailboxTurns(rec *types.RunRecord) bool {
	if rec == nil || agentProfileForRun(rec) != agentprofile.Texture {
		return false
	}
	requestSource := metadataStringValue(rec.Metadata, "request_source")
	if requestSource == "update_coagent" || requestSource == "lifecycle_texture_control" {
		return true
	}
	return len(coagentUpdateIDsForRun(rec)) > 0
}

func (rt *Runtime) coagentParkWaiter(rec *types.RunRecord) toolregistry.ToolLoopParkWaiterFunc {
	if rt == nil || rt.store == nil || rec == nil || !runSupportsCoagentUpdateInjection(rec) {
		return nil
	}
	if !metadataBoolValue(rec.Metadata, "actor_park_on_idle") {
		return nil
	}
	ownerID := strings.TrimSpace(rec.OwnerID)
	agentID := strings.TrimSpace(rec.AgentID)
	if ownerID == "" || agentID == "" {
		return nil
	}
	return func(ctx context.Context, state toolregistry.ToolLoopParkState) (toolregistry.ToolLoopParkResult, error) {
		ready := func() (bool, error) {
			updates, err := rt.pendingCoagentUpdatesForRun(ctx, rec, ownerID, agentID, 100)
			if err != nil {
				return false, fmt.Errorf("list pending update_coagent records for park wait: %w", err)
			}
			seen := map[string]bool{}
			for _, id := range coagentUpdateIDsForRun(rec) {
				id = strings.TrimSpace(id)
				if id != "" {
					seen[id] = true
				}
			}
			for _, update := range updates {
				id := strings.TrimSpace(update.UpdateID)
				if id != "" && !seen[id] {
					if !coagentUpdateDeliverableForRun(rec, update) {
						continue
					}
					return true, nil
				}
			}
			return false, nil
		}
		// Actor mode: do not block on a channel. If there are no
		// pending updates, passivate immediately. The actor will
		// re-activate when a new coagent update arrives via actor.Send,
		// and the handler will resume the tool loop from the park point.
		ok, err := ready()
		if err != nil {
			return toolregistry.ToolLoopParkResult{}, err
		}
		if ok {
			return toolregistry.ToolLoopParkResult{Continue: true, Reason: "update_coagent_signal"}, nil
		}
		return toolregistry.ToolLoopParkResult{Continue: false, Passivate: true, Reason: "idle_actor_passivate"}, nil
	}
}

func runSupportsCoagentUpdateInjection(rec *types.RunRecord) bool {
	if rec == nil {
		return false
	}
	switch agentProfileForRun(rec) {
	case agentprofile.Super, agentprofile.CoSuper, agentprofile.Researcher, agentprofile.Texture:
		return strings.TrimSpace(rec.AgentID) != ""
	default:
		return false
	}
}

func (rt *Runtime) prependInitialCoagentUpdatePackets(ctx context.Context, rec *types.RunRecord, messages []json.RawMessage) ([]json.RawMessage, error) {
	if rt == nil || rt.store == nil || rec == nil || !runSupportsCoagentUpdateInjection(rec) {
		return messages, nil
	}
	if !shouldPrependInitialCoagentUpdates(rec) {
		return messages, nil
	}
	ownerID := strings.TrimSpace(rec.OwnerID)
	agentID := strings.TrimSpace(rec.AgentID)
	if ownerID == "" || agentID == "" {
		return messages, nil
	}
	seen := map[string]bool{}
	for _, id := range coagentUpdateIDsForRun(rec) {
		id = strings.TrimSpace(id)
		if id != "" {
			seen[id] = true
		}
	}
	updates, err := rt.pendingCoagentUpdatesForRun(ctx, rec, ownerID, agentID, 100)
	if err != nil {
		return messages, fmt.Errorf("list pending coagent updates for cold delivery: %w", err)
	}
	fresh := make([]types.CoagentSourcePacket, 0, len(updates))
	updateIDs := make([]string, 0, len(updates))
	for _, update := range updates {
		id := strings.TrimSpace(update.UpdateID)
		if id == "" || seen[id] {
			continue
		}
		if !coagentUpdateDeliverableForRun(rec, update) {
			continue
		}
		seen[id] = true
		fresh = append(fresh, update)
		updateIDs = append(updateIDs, id)
	}
	if len(fresh) == 0 {
		return messages, nil
	}
	appendCoagentUpdateIDsForRun(rec, updateIDs)
	projected, err := rt.projectTerminalOutcomeContent(ctx, fresh)
	if err != nil {
		return messages, err
	}
	msgs, _, err := buildCoagentUpdateUserMessages(projected, coagentPacketDeliveryCold, agentID, nil, nil)
	if err != nil {
		return messages, err
	}
	return append(msgs, messages...), nil
}

func shouldPrependInitialCoagentUpdates(rec *types.RunRecord) bool {
	if rec == nil {
		return false
	}
	if agentProfileForRun(rec) == agentprofile.Texture {
		return false
	}
	requestSource := metadataStringValue(rec.Metadata, "request_source")
	if requestSource == "update_coagent" || requestSource == "lifecycle_texture_control" {
		return true
	}
	return len(coagentUpdateIDsForRun(rec)) > 0
}

// ReconcileCoagentWake is the actor-mode entry point for creating a new run
// when a coagent update arrives for an agent with no parked run. It is called
// by the actor handler (handleCoagentResult) when the actor's memory snapshot
// has no resume pointer. The reconcile logic creates a new run (if appropriate
// for the agent type — Texture, persistent super, or generic coagent) and
// calls rt.activate(rec), which sends an initial_dispatch actor message.
func (rt *Runtime) ReconcileCoagentWake(ctx context.Context, ownerID, agentID string) (*types.RunRecord, error) {
	ownerID = strings.TrimSpace(ownerID)
	agentID = strings.TrimSpace(agentID)
	if ownerID == "" || agentID == "" {
		return nil, nil
	}
	if agentID == persistentSuperAgentID(ownerID) {
		return rt.reconcilePersistentSuperActor(ctx, ownerID, agentID)
	}
	return rt.reconcileUpdatedCoagentActor(ctx, ownerID, agentID)
}

func (rt *Runtime) wakeUpdatedCoagent(ctx context.Context, update types.CoagentSourcePacket) {
	if rt == nil || rt.store == nil {
		return
	}
	target := strings.TrimSpace(update.TargetAgentID)
	if target == "" {
		return
	}
	if update.Direction == types.LifecyclePacketDirectionControl {
		if resident, found, err := rt.activeRunByAgent(ctx, update.OwnerID, target); err != nil {
			log.Printf("runtime: resolve resident lifecycle control target %s: %v", target, err)
			return
		} else if found {
			updates, listErr := rt.store.ListPendingLifecycleUpdates(ctx, update.OwnerID, update.ComputerID, target, 100)
			if listErr == nil {
				updates, listErr = rt.validateTargetBoundLifecycleControls(ctx, update.OwnerID, update.ComputerID, target, updates, agentProfileForRun(&resident) == agentprofile.Super)
			}
			updates = selectLifecycleControlActivation(updates, resident.TrajectoryID, lifecycleControlWorkIDsForRun(&resident))
			if listErr != nil || len(updates) == 0 {
				log.Printf("runtime: bind resident lifecycle controls target=%s: %v", target, listErr)
				return
			}
			if _, bindErr := rt.bindLifecycleControlsToRun(ctx, &resident, updates); bindErr != nil {
				log.Printf("runtime: bind resident lifecycle controls target=%s: %v", target, bindErr)
				return
			}
		}
	}
	// The coagent update is already in canonical lifecycle state. Send an actor
	// message to wake the target agent — the handler will resume the
	// parked run (or start a new one) and inject the update via
	// injectUserTurns. No channel signal, no reconcile-new-run.
	if rt.dispatchActor == nil {
		panic("runtime: wakeUpdatedCoagent called without dispatchActor set — actor runtime is required")
	}
	if err := rt.dispatchActor(context.Background(), update.OwnerID, firstNonEmpty(update.ComputerID, rt.TextureSandboxID()), target, "coagent_result", update.UpdateID, update.TrajectoryID, update.AgentID); err != nil {
		log.Printf("runtime: actor wake coagent for update %s: %v", update.UpdateID, err)
	}
}
