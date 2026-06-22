package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/store"
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
	if resident, found, err := rt.residentRunByAgent(ctx, ownerID, agentID); err != nil {
		return nil, fmt.Errorf("check resident super run: %w", err)
	} else if found {
		return &resident, nil
	}
	if active, err := rt.store.GetLatestActiveRunByAgent(ctx, ownerID, agentID); err == nil {
		if active.State == types.RunBlocked {
			blockedActive = &active
		}
	} else if !errors.Is(err, store.ErrNotFound) {
		return nil, fmt.Errorf("check blocked super run: %w", err)
	}

	updates, err := rt.listAndSettlePersistentSuperBacklog(ctx, ownerID, agentID)
	if err != nil {
		return nil, err
	}
	updates = filterPersistentSuperExecutionUpdates(updates)
	if len(updates) == 0 {
		if blockedActive != nil {
			return blockedActive, nil
		}
		return nil, nil
	}

	first := updates[0]
	metadata := map[string]any{
		runMetadataAgentProfile: AgentProfileSuper,
		runMetadataAgentRole:    AgentProfileSuper,
		runMetadataAgentID:      agentID,
		"request_source":        "update_coagent",
		"requested_by_agent_id": first.AgentID,
		"requested_by_profile":  strings.TrimSpace(first.Role),
	}
	if first.ChannelID != "" {
		metadata[runMetadataChannelID] = first.ChannelID
	}
	if first.TrajectoryID != "" {
		metadata[runMetadataTrajectoryID] = first.TrajectoryID
	}
	updateIDs := make([]string, 0, len(updates))
	for _, update := range updates {
		if id := strings.TrimSpace(update.UpdateID); id != "" {
			updateIDs = append(updateIDs, id)
		}
	}
	if len(updateIDs) > 0 {
		metadata["worker_update_ids"] = updateIDs
	}
	if first.AgentID != "" {
		if requester, err := rt.store.GetLatestActiveRunByAgent(ctx, ownerID, first.AgentID); err == nil {
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
	rt.startRunAsync(rec)
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
	if isTextureAgentRevisionTaskType(metadataStringValue(rec.Metadata, "type")) {
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
	if agentProfileForRun(rec) != AgentProfileSuper {
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
		return rt.reconcileTextureAgentWake(ctx, ownerID, docIDFromTextureAgentID(agentID))
	}
	if resident, found, err := rt.residentRunByAgent(ctx, ownerID, agentID); err != nil {
		return nil, fmt.Errorf("check resident coagent run: %w", err)
	} else if found {
		return &resident, nil
	}
	agent, err := rt.store.GetAgent(ctx, agentID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("lookup coagent: %w", err)
	}
	updates, err := rt.store.ListCoagentMailboxBacklog(ctx, ownerID, agentID, 100)
	if err != nil {
		return nil, fmt.Errorf("list coagent pending updates: %w", err)
	}
	if len(updates) == 0 {
		return nil, nil
	}
	first := updates[0]
	profile := canonicalAgentProfile(firstNonEmpty(agent.Profile, first.Role))
	if profile == "" || profile == AgentProfileEmail || profile == AgentProfileConductor || profile == AgentProfileSuper {
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
	metadata := map[string]any{
		runMetadataAgentProfile: profile,
		runMetadataAgentRole:    role,
		runMetadataAgentID:      agentID,
		"request_source":        "update_coagent",
		"worker_update_ids":     updateIDs,
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
	rt.startRunAsync(rec)
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

func (rt *Runtime) coagentUpdateTurnInjector(rec *types.RunRecord) InjectUserTurnsFunc {
	return rt.coagentUpdateTurnInjectorWithInitialPhase(rec, "")
}

func (rt *Runtime) coagentUpdateTurnInjectorWithInitialPhase(rec *types.RunRecord, initialPhase string) InjectUserTurnsFunc {
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
		updates, err := rt.store.ListCoagentMailboxBacklog(context.Background(), ownerID, agentID, 100)
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
		var sourceEntities []textureSourceEntity
		if agentProfileForRun(rec) == AgentProfileTexture {
			sourceEntities = rt.evidenceSourceEntitiesFromWorkerUpdates(context.Background(), ownerID, fresh)
			mergeTextureSourceEntitiesIntoRunMetadata(rec, sourceEntities)
		}
		phase := coagentPacketDeliveryMid
		if finalCheckpoint {
			phase = coagentPacketDeliveryFinal
		} else if initialPhase != "" {
			phase = initialPhase
			initialPhase = ""
		}
		msgs, _, err := buildCoagentUpdateUserMessages(fresh, phase, agentID, sourceEntities)
		if err != nil {
			return nil, err
		}
		return msgs, nil
	}
}

func shouldAppendInitialCoagentMailboxTurns(rec *types.RunRecord) bool {
	if rec == nil || agentProfileForRun(rec) != AgentProfileTexture {
		return false
	}
	if metadataStringValue(rec.Metadata, "request_source") == "update_coagent" {
		return true
	}
	return len(coagentUpdateIDsForRun(rec)) > 0
}

func (rt *Runtime) coagentParkWaiter(rec *types.RunRecord) ToolLoopParkWaiterFunc {
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
	maxWait := time.Duration(metadataIntValue(rec.Metadata, "actor_park_idle_seconds")) * time.Second
	return func(ctx context.Context, state ToolLoopParkState) (ToolLoopParkResult, error) {
		ready := func() (bool, error) {
			updates, err := rt.store.ListCoagentMailboxBacklog(ctx, ownerID, agentID, 100)
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
		ok, err := rt.waitForAgentSignal(ctx, ownerID, agentID, maxWait, ready)
		if err != nil {
			return ToolLoopParkResult{}, err
		}
		if ok {
			return ToolLoopParkResult{Continue: true, Reason: "update_coagent_signal"}, nil
		}
		return ToolLoopParkResult{Continue: false, Passivate: agentProfileForRun(rec) == AgentProfileTexture, Reason: "idle_deadline"}, nil
	}
}

func runSupportsCoagentUpdateInjection(rec *types.RunRecord) bool {
	if rec == nil {
		return false
	}
	switch agentProfileForRun(rec) {
	case AgentProfileSuper, AgentProfileVSuper, AgentProfileCoSuper, AgentProfileResearcher, AgentProfileTexture:
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
	updates, err := rt.store.ListCoagentMailboxBacklog(ctx, ownerID, agentID, 100)
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
	msgs, _, err := buildCoagentUpdateUserMessages(fresh, coagentPacketDeliveryCold, agentID, nil)
	if err != nil {
		return messages, err
	}
	return append(msgs, messages...), nil
}

func shouldPrependInitialCoagentUpdates(rec *types.RunRecord) bool {
	if rec == nil {
		return false
	}
	if agentProfileForRun(rec) == AgentProfileTexture {
		return false
	}
	if metadataStringValue(rec.Metadata, "request_source") == "update_coagent" {
		return true
	}
	return len(coagentUpdateIDsForRun(rec)) > 0
}

func (rt *Runtime) wakeUpdatedCoagent(ctx context.Context, update types.CoagentSourcePacket) {
	if rt == nil || rt.store == nil {
		return
	}
	target := strings.TrimSpace(update.TargetAgentID)
	if target == "" {
		return
	}
	rt.notifyAgentSignal(update.OwnerID, target)
	if target == persistentSuperAgentID(update.OwnerID) {
		if _, err := rt.reconcilePersistentSuperActor(context.Background(), update.OwnerID, target); err != nil {
			log.Printf("runtime: wake persistent super for update %s: %v", update.UpdateID, err)
		}
		return
	}
	if _, err := rt.reconcileUpdatedCoagentActor(context.Background(), update.OwnerID, target); err != nil {
		log.Printf("runtime: wake coagent for update %s: %v", update.UpdateID, err)
	}
}
