package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

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

	updates, err := rt.store.ListPendingWorkerUpdates(ctx, ownerID, agentID, 100)
	if err != nil {
		return nil, fmt.Errorf("list super pending updates: %w", err)
	}
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

	rec, err := rt.createRunWithMetadata(ctx, buildPersistentSuperUpdatePrompt(updates), ownerID, metadata)
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
	if len(updateIDs) == 0 || rec.State != types.RunCompleted {
		return rt.store.UpdateRun(ctx, *rec)
	}
	return rt.store.UpdateRunAndMarkWorkerUpdatesDelivered(ctx, *rec, rec.OwnerID, updateIDs)
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
	if agentProfileForRun(rec) != AgentProfileSuper {
		return false
	}
	if metadataStringValue(rec.Metadata, "request_source") != "update_coagent" {
		return false
	}
	if strings.TrimSpace(rec.OwnerID) == "" || strings.TrimSpace(rec.AgentID) == "" {
		return false
	}
	return rec.AgentID == persistentSuperAgentID(rec.OwnerID)
}

func buildPersistentSuperUpdatePrompt(updates []types.WorkerUpdateRecord) string {
	var b strings.Builder
	b.WriteString("Process the pending update_coagent records addressed to you as the user's persistent super actor.\n\n")
	b.WriteString("Use privileged tools only for the requested execution work. When you have artifacts, test results, references, questions, or proposals, report them back with update_coagent to the addressed vtext document.\n")
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
		if update.Kind != "" {
			b.WriteString(" kind=")
			b.WriteString(update.Kind)
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
	updates, err := rt.store.ListPendingWorkerUpdates(ctx, ownerID, agentID, 100)
	if err != nil {
		return nil, fmt.Errorf("list coagent pending updates: %w", err)
	}
	if len(updates) == 0 {
		return nil, nil
	}
	first := updates[0]
	profile := canonicalAgentProfile(firstNonEmpty(agent.Profile, first.Role))
	if profile == "" || profile == AgentProfileEmail || profile == AgentProfileConductor || profile == AgentProfileVText || profile == AgentProfileSuper {
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
	rec, err := rt.createRunWithMetadata(ctx, buildCoagentUpdatePrompt(updates), ownerID, metadata)
	if err != nil {
		return nil, err
	}
	rt.startRunAsync(rec)
	return rec, nil
}

func buildCoagentUpdatePrompt(updates []types.WorkerUpdateRecord) string {
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
		if update.Kind != "" {
			b.WriteString(" kind=")
			b.WriteString(update.Kind)
		}
		b.WriteString(":\n")
		b.WriteString(strings.TrimSpace(update.Content))
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func (rt *Runtime) coagentUpdateTurnInjector(rec *types.RunRecord) InjectUserTurnsFunc {
	if rt == nil || rt.store == nil || rec == nil || !runSupportsCoagentUpdateInjection(rec) {
		return nil
	}
	ownerID := strings.TrimSpace(rec.OwnerID)
	agentID := strings.TrimSpace(rec.AgentID)
	if ownerID == "" || agentID == "" {
		return nil
	}
	seen := map[string]bool{}
	for _, id := range coagentUpdateIDsForRun(rec) {
		id = strings.TrimSpace(id)
		if id != "" {
			seen[id] = true
		}
	}
	return func(finalCheckpoint bool) ([]json.RawMessage, error) {
		updates, err := rt.store.ListPendingWorkerUpdates(context.Background(), ownerID, agentID, 100)
		if err != nil {
			return nil, fmt.Errorf("list pending update_coagent turns: %w", err)
		}
		fresh := make([]types.WorkerUpdateRecord, 0, len(updates))
		updateIDs := make([]string, 0, len(updates))
		for _, update := range updates {
			id := strings.TrimSpace(update.UpdateID)
			if id == "" || seen[id] {
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
		content := buildCoagentInjectedUpdateTurn(fresh, finalCheckpoint)
		msg, _ := json.Marshal(map[string]any{
			"role": "user",
			"content": []map[string]string{{
				"type": "text",
				"text": content,
			}},
		})
		return []json.RawMessage{msg}, nil
	}
}

func runSupportsCoagentUpdateInjection(rec *types.RunRecord) bool {
	if rec == nil {
		return false
	}
	switch agentProfileForRun(rec) {
	case AgentProfileSuper, AgentProfileVSuper, AgentProfileCoSuper, AgentProfileResearcher:
		return strings.TrimSpace(rec.AgentID) != ""
	default:
		return false
	}
}

func buildCoagentInjectedUpdateTurn(updates []types.WorkerUpdateRecord, finalCheckpoint bool) string {
	var b strings.Builder
	if finalCheckpoint {
		b.WriteString("New update_coagent records arrived before this activation finished. Process them before ending the turn.\n")
	} else {
		b.WriteString("New update_coagent records arrived while this activation was running. Treat them as the next user turn.\n")
	}
	b.WriteString("Do not ignore these updates. If they change the work, incorporate them; if they are blocked, report the blocker with update_coagent.\n")
	for i, update := range updates {
		b.WriteString("\nUpdate ")
		b.WriteString(fmt.Sprintf("%d", i+1))
		if update.UpdateID != "" {
			b.WriteString(" id=")
			b.WriteString(update.UpdateID)
		}
		if update.ChannelID != "" {
			b.WriteString(" channel=")
			b.WriteString(update.ChannelID)
		}
		if update.AgentID != "" {
			b.WriteString(" from=")
			b.WriteString(update.AgentID)
		}
		if update.Kind != "" {
			b.WriteString(" kind=")
			b.WriteString(update.Kind)
		}
		b.WriteString(":\n")
		b.WriteString(strings.TrimSpace(update.Content))
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func (rt *Runtime) wakeUpdatedCoagent(ctx context.Context, update types.WorkerUpdateRecord) {
	if rt == nil || rt.store == nil {
		return
	}
	target := strings.TrimSpace(update.TargetAgentID)
	if target == "" {
		return
	}
	if target == persistentSuperAgentID(update.OwnerID) {
		if _, err := rt.reconcilePersistentSuperActor(context.Background(), update.OwnerID, target); err != nil {
			log.Printf("runtime: wake persistent super for update %s: %v", update.UpdateID, err)
		}
		return
	}
	if docID, ok := strings.CutPrefix(target, "vtext:"); ok {
		if err := rt.reconcileVTextWorkerState(context.Background(), update.OwnerID, docID); err != nil {
			log.Printf("runtime: wake vtext for update %s: %v", update.UpdateID, err)
		}
		return
	}
	if _, err := rt.reconcileUpdatedCoagentActor(context.Background(), update.OwnerID, target); err != nil {
		log.Printf("runtime: wake coagent for update %s: %v", update.UpdateID, err)
	}
}
