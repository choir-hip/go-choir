package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

const runMetadataWorkerUpdatesInjected = "worker_updates_injected"

type canonicalSupervisionDeliveryBody struct {
	MessageID          string  `json:"message_id"`
	FromActorID        string  `json:"from_actor_id"`
	ToRole             string  `json:"to_role"`
	ToActorID          *string `json:"to_actor_id"`
	ChannelID          string  `json:"channel_id"`
	PayloadArtifactRef string  `json:"payload_artifact_ref"`
	PacketID           string  `json:"packet_id"`
	ResearcherID       string  `json:"researcher_id"`
	PacketArtifactRef  string  `json:"packet_artifact_ref"`
	AssignmentID       string  `json:"assignment_id"`
	ResultID           string  `json:"result_id"`
	ResultArtifactRef  string  `json:"result_artifact_ref"`
	Outcome            string  `json:"outcome"`
}

// ListPendingCanonicalSupervisionUpdates exposes the one-tape delivery
// projection to the Texture lifecycle owner. It refuses when the canonical
// private-artifact read authority is unavailable rather than making an empty
// legacy mailbox look authoritative.
func (rt *Runtime) ListPendingCanonicalSupervisionUpdates(ctx context.Context, ownerID, computerID, targetAgentID, trajectoryID string, limit int) ([]types.CoagentSourcePacket, error) {
	if rt == nil || rt.store == nil || rt.eventAppender == nil || rt.privateArtifactCipher == nil {
		return nil, fmt.Errorf("%w: canonical supervision delivery authority unavailable", ErrSupervisionAuthorityRequired)
	}
	return rt.canonicalSupervisionUpdatesForAgent(ctx, ownerID, computerID, targetAgentID, trajectoryID, limit)
}

// ListPendingSupervisionCompatibilityUpdates reads pre-cutover lifecycle
// mailbox rows only after canonical private-artifact authority is available,
// then projects completed-run consumption without mutating the legacy rows.
func (rt *Runtime) ListPendingSupervisionCompatibilityUpdates(ctx context.Context, ownerID, computerID, targetAgentID, trajectoryID string, limit int) ([]types.CoagentSourcePacket, error) {
	if rt == nil || rt.store == nil || rt.eventAppender == nil || rt.privateArtifactCipher == nil {
		return nil, fmt.Errorf("%w: canonical supervision delivery authority unavailable", ErrSupervisionAuthorityRequired)
	}
	updates, err := rt.store.ListPendingLifecycleUpdatesForTrajectory(ctx, ownerID, computerID, targetAgentID, trajectoryID, 0)
	if err != nil {
		return nil, err
	}
	consumed, err := rt.consumedCanonicalSupervisionUpdateIDs(ctx, ownerID, computerID, targetAgentID)
	if err != nil {
		return nil, err
	}
	updates = filterConsumedCoagentUpdates(updates, consumed)
	if limit > 0 && len(updates) > limit {
		updates = updates[:limit]
	}
	return updates, nil
}

func (rt *Runtime) canonicalSupervisionUpdatesForAgent(ctx context.Context, ownerID, computerID, targetAgentID, trajectoryID string, limit int) ([]types.CoagentSourcePacket, error) {
	if rt == nil || rt.store == nil {
		return nil, fmt.Errorf("%w: canonical supervision delivery store unavailable", ErrSupervisionAuthorityRequired)
	}
	if rt.eventAppender == nil || rt.privateArtifactCipher == nil {
		pending, err := rt.hasPendingCanonicalSupervisionDelivery(ctx, ownerID, computerID, targetAgentID, trajectoryID)
		if err != nil {
			return nil, err
		}
		if pending {
			return nil, fmt.Errorf("%w: canonical supervision delivery artifact authority unavailable", ErrSupervisionAuthorityRequired)
		}
		return nil, nil
	}
	consumed, err := rt.consumedCanonicalSupervisionUpdateIDs(ctx, ownerID, computerID, targetAgentID)
	if err != nil {
		return nil, err
	}
	return rt.canonicalSupervisionUpdatesForAgentWithConsumed(ctx, ownerID, computerID, targetAgentID, trajectoryID, limit, consumed)
}

func (rt *Runtime) hasPendingCanonicalSupervisionDelivery(ctx context.Context, ownerID, computerID, targetAgentID, trajectoryID string) (bool, error) {
	ownerID, computerID, targetAgentID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID), strings.TrimSpace(targetAgentID)
	if ownerID == "" || computerID == "" || targetAgentID == "" {
		return false, nil
	}
	consumed, err := rt.consumedCanonicalSupervisionUpdateIDs(ctx, ownerID, computerID, targetAgentID)
	if err != nil {
		return false, err
	}
	var trajectories []types.TrajectoryRecord
	if trajectoryID = strings.TrimSpace(trajectoryID); trajectoryID != "" {
		trajectory, err := rt.store.GetLifecycleTrajectory(ctx, ownerID, computerID, trajectoryID)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return false, nil
			}
			return false, err
		}
		trajectories = []types.TrajectoryRecord{trajectory}
	} else {
		trajectories, err = rt.store.ListLifecycleTrajectoriesByOwner(ctx, ownerID, computerID, 0)
		if err != nil {
			return false, err
		}
	}
	for _, trajectory := range trajectories {
		deliveries, err := rt.store.ListSupervisionDeliveryEvents(ctx, ownerID, computerID, trajectory.TrajectoryID)
		if err != nil {
			return false, err
		}
		for _, delivery := range deliveries {
			if consumed[delivery.ID] {
				continue
			}
			addressed, err := rt.canonicalSupervisionDeliveryAddressed(ctx, trajectory, targetAgentID, delivery)
			if err != nil {
				return false, err
			}
			if addressed {
				return true, nil
			}
		}
	}
	return false, nil
}

func (rt *Runtime) canonicalSupervisionDeliveryAddressed(ctx context.Context, trajectory types.TrajectoryRecord, targetAgentID string, delivery store.SupervisionDeliveryEvent) (bool, error) {
	var body canonicalSupervisionDeliveryBody
	if err := json.Unmarshal(delivery.Body, &body); err != nil {
		return false, fmt.Errorf("decode canonical supervision delivery %s: %w", delivery.ID, err)
	}
	return rt.canonicalSupervisionDeliveryBodyAddressed(ctx, trajectory, targetAgentID, delivery.Kind, body)
}

func (rt *Runtime) canonicalSupervisionDeliveryBodyAddressed(ctx context.Context, trajectory types.TrajectoryRecord, targetAgentID, deliveryKind string, body canonicalSupervisionDeliveryBody) (bool, error) {
	switch deliveryKind {
	case "actor_message_recorded":
		if body.ToActorID != nil {
			return strings.TrimSpace(*body.ToActorID) == targetAgentID, nil
		}
		targetProfile := canonicalProfileForAgentID(targetAgentID)
		if targetProfile == "" {
			target, err := rt.store.GetAgentByScope(ctx, trajectory.OwnerID, trajectory.ComputerID, targetAgentID)
			if err != nil {
				if errors.Is(err, store.ErrNotFound) {
					return false, nil
				}
				return false, err
			}
			targetProfile = agentprofile.Canonical(firstNonEmpty(target.Profile, target.Role))
		}
		return agentprofile.Canonical(body.ToRole) == targetProfile, nil
	case "attempt_result":
		return targetAgentID == persistentSuperAgentID(trajectory.OwnerID), nil
	case "researcher_packet_recorded":
		return targetAgentID == textureAgentIDForTrajectory(trajectory), nil
	default:
		return false, nil
	}
}

func (rt *Runtime) canonicalSupervisionUpdatesForAgentWithConsumed(ctx context.Context, ownerID, computerID, targetAgentID, trajectoryID string, limit int, consumed map[string]bool) ([]types.CoagentSourcePacket, error) {
	if rt == nil || rt.store == nil || rt.eventAppender == nil || rt.privateArtifactCipher == nil {
		return nil, fmt.Errorf("%w: canonical supervision delivery authority unavailable", ErrSupervisionAuthorityRequired)
	}
	ownerID, computerID, targetAgentID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID), strings.TrimSpace(targetAgentID)
	if ownerID == "" || computerID == "" || targetAgentID == "" {
		return nil, fmt.Errorf("canonical supervision delivery requires owner, computer, and target actor")
	}
	var trajectories []types.TrajectoryRecord
	if trajectoryID = strings.TrimSpace(trajectoryID); trajectoryID != "" {
		trajectory, err := rt.store.GetLifecycleTrajectory(ctx, ownerID, computerID, trajectoryID)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return nil, nil
			}
			return nil, err
		}
		trajectories = []types.TrajectoryRecord{trajectory}
	} else {
		var err error
		trajectories, err = rt.store.ListLifecycleTrajectoriesByOwner(ctx, ownerID, computerID, 0)
		if err != nil {
			return nil, err
		}
	}
	updates := make([]types.CoagentSourcePacket, 0)
	for _, trajectory := range trajectories {
		deliveries, err := rt.store.ListSupervisionDeliveryEvents(ctx, ownerID, computerID, trajectory.TrajectoryID)
		if err != nil {
			return nil, err
		}
		for _, delivery := range deliveries {
			if consumed[delivery.ID] {
				continue
			}
			update, addressed, err := rt.canonicalSupervisionDeliveryUpdate(ctx, trajectory, targetAgentID, delivery)
			if err != nil {
				return nil, err
			}
			if addressed {
				updates = append(updates, update)
			}
		}
	}
	sort.Slice(updates, func(i, j int) bool {
		left := uint64(updates[i].ReducerSeq)
		right := uint64(updates[j].ReducerSeq)
		if left != right {
			return left < right
		}
		return updates[i].UpdateID < updates[j].UpdateID
	})
	if limit > 0 && len(updates) > limit {
		updates = updates[:limit]
	}
	return updates, nil
}

func (rt *Runtime) canonicalSupervisionDeliveryUpdate(ctx context.Context, trajectory types.TrajectoryRecord, targetAgentID string, delivery store.SupervisionDeliveryEvent) (types.CoagentSourcePacket, bool, error) {
	var body canonicalSupervisionDeliveryBody
	if err := json.Unmarshal(delivery.Body, &body); err != nil {
		return types.CoagentSourcePacket{}, false, fmt.Errorf("decode canonical supervision delivery %s: %w", delivery.ID, err)
	}
	addressed, err := rt.canonicalSupervisionDeliveryBodyAddressed(ctx, trajectory, targetAgentID, delivery.Kind, body)
	if err != nil || !addressed {
		return types.CoagentSourcePacket{}, addressed, err
	}
	sourceAgentID, sourceRole, channelID, artifactRef := "", "", trajectory.TrajectoryID, ""
	switch delivery.Kind {
	case "actor_message_recorded":
		sourceAgentID, channelID, artifactRef = strings.TrimSpace(body.FromActorID), strings.TrimSpace(body.ChannelID), strings.TrimSpace(body.PayloadArtifactRef)
	case "attempt_result":
		lineage, err := rt.store.GetSupervisionAssignmentLineage(ctx, trajectory.OwnerID, trajectory.ComputerID, trajectory.TrajectoryID, body.AssignmentID)
		if err != nil {
			return types.CoagentSourcePacket{}, false, err
		}
		sourceAgentID, sourceRole, artifactRef = lineage.AssignedActorID, agentprofile.CoSuper, strings.TrimSpace(body.ResultArtifactRef)
	case "researcher_packet_recorded":
		textureAgentID := textureAgentIDForTrajectory(trajectory)
		docID := docIDFromTextureAgentID(textureAgentID)
		sourceAgentID, sourceRole, channelID, artifactRef = strings.TrimSpace(body.ResearcherID), agentprofile.Researcher, docID, strings.TrimSpace(body.PacketArtifactRef)
	default:
		return types.CoagentSourcePacket{}, false, nil
	}
	if sourceRole == "" && sourceAgentID != "" {
		if source, err := rt.store.GetAgentByScope(ctx, trajectory.OwnerID, trajectory.ComputerID, sourceAgentID); err == nil {
			sourceRole = agentprofile.Canonical(source.Profile)
		}
	}
	bindingIDs := []string{delivery.ID + ":packet"}
	if legacy := strings.TrimSpace(delivery.CommandID); legacy != "" && legacy != delivery.ID {
		bindingIDs = append(bindingIDs, legacy+":packet")
	}
	var plaintext []byte
	var metadata computerevent.PrivateArtifactMetadata
	var loadErrors []error
	for _, bindingID := range bindingIDs {
		plaintext, metadata, err = rt.eventAppender.LoadPrivateSupervisionArtifact(ctx, artifactRef, bindingID, rt.privateArtifactCipher)
		if err == nil {
			break
		}
		loadErrors = append(loadErrors, fmt.Errorf("%s: %w", bindingID, err))
	}
	if err != nil {
		return types.CoagentSourcePacket{}, false, fmt.Errorf("load canonical supervision delivery %s: %w", delivery.ID, errors.Join(loadErrors...))
	}
	if metadata.MediaType != computerevent.SupervisionEvidenceMediaTypeV1 {
		return types.CoagentSourcePacket{}, false, fmt.Errorf("canonical supervision delivery %s has media type %q", delivery.ID, metadata.MediaType)
	}
	var packet types.CoagentSourcePacketPayload
	if err := json.Unmarshal(plaintext, &packet); err != nil {
		return types.CoagentSourcePacket{}, false, fmt.Errorf("decode canonical supervision packet %s: %w", delivery.ID, err)
	}
	packet = normalizeCoagentSourcePacketPayload(packet)
	if err := validateCoagentSourcePacketPayload(packet); err != nil {
		return types.CoagentSourcePacket{}, false, fmt.Errorf("validate canonical supervision packet %s: %w", delivery.ID, err)
	}
	workDisposition := types.WorkItemOpen
	if delivery.Kind == "attempt_result" {
		workDisposition = types.WorkItemCompleted
		if body.Outcome == "blocked" || body.Outcome == "failed" {
			workDisposition = types.WorkItemOpen
		}
	}
	update := types.CoagentSourcePacket{
		UpdateID: delivery.ID, OwnerID: trajectory.OwnerID, ComputerID: trajectory.ComputerID,
		AgentID: sourceAgentID, TargetAgentID: targetAgentID, ChannelID: channelID,
		TrajectoryID: trajectory.TrajectoryID, Role: sourceRole, Packet: packet,
		Content:   buildWorkerUpdateMessage(types.CoagentSourcePacket{Packet: packet}),
		CreatedAt: delivery.CreatedAt, WorkDisposition: workDisposition,
		LifecycleVersion: 1, ReducerSeq: int64(delivery.Sequence),
	}
	return update, true, nil
}

func canonicalProfileForAgentID(agentID string) string {
	switch {
	case strings.HasPrefix(strings.TrimSpace(agentID), agentprofile.Texture+":"):
		return agentprofile.Texture
	case strings.HasPrefix(strings.TrimSpace(agentID), agentprofile.Super+":") || strings.TrimSpace(agentID) == agentprofile.Super:
		return agentprofile.Super
	default:
		return ""
	}
}

func textureAgentIDForTrajectory(trajectory types.TrajectoryRecord) string {
	docID := strings.TrimSpace(trajectory.SubjectRefs["doc_id"])
	if docID == "" {
		docID = strings.TrimPrefix(strings.TrimSpace(trajectory.SubjectRefs["artifact"]), "texture://documents/")
	}
	if docID == "" {
		return ""
	}
	return currentTextureAgentID(docID)
}

func (rt *Runtime) consumedCanonicalSupervisionUpdateIDs(ctx context.Context, ownerID, computerID, targetAgentID string) (map[string]bool, error) {
	trajectories, err := rt.store.ListLifecycleTrajectoriesByOwner(ctx, ownerID, computerID, 0)
	if err != nil {
		return nil, fmt.Errorf("list canonical supervision acknowledgement trajectories: %w", err)
	}
	consumed := make(map[string]bool)
	for _, trajectory := range trajectories {
		events, err := rt.store.ListSupervisionDeliveryEvents(ctx, ownerID, computerID, trajectory.TrajectoryID)
		if err != nil {
			return nil, fmt.Errorf("list canonical supervision acknowledgements: %w", err)
		}
		for _, event := range events {
			if event.Kind != "actor_message_acknowledged" {
				continue
			}
			var body struct {
				MessageID     string `json:"message_id"`
				TargetActorID string `json:"target_actor_id"`
			}
			if err := json.Unmarshal(event.Body, &body); err != nil {
				return nil, fmt.Errorf("decode canonical supervision acknowledgement %s: %w", event.ID, err)
			}
			if strings.TrimSpace(body.TargetActorID) == strings.TrimSpace(targetAgentID) {
				consumed[strings.TrimSpace(body.MessageID)] = true
			}
		}
	}
	return consumed, nil
}

func (rt *Runtime) reconcilePendingCanonicalSupervisionActors(ctx context.Context) {
	if err := rt.sweepPendingCanonicalSupervisionActors(ctx); err != nil {
		log.Printf("runtime: recover canonical supervision deliveries: %v", err)
		rt.scheduleCanonicalSupervisionSweep(ctx)
	}
}

func (rt *Runtime) scheduleCanonicalSupervisionSweep(ctx context.Context) {
	if rt == nil || ctx == nil || ctx.Err() != nil || rt.textureWakeAfter == nil {
		return
	}
	rt.canonicalSweepMu.Lock()
	defer rt.canonicalSweepMu.Unlock()
	if rt.canonicalSweepTimer != nil {
		return
	}
	rt.canonicalSweepTimer = rt.textureWakeAfter(time.Second, func() {
		rt.canonicalSweepMu.Lock()
		rt.canonicalSweepTimer = nil
		rt.canonicalSweepMu.Unlock()
		if ctx.Err() == nil {
			rt.reconcilePendingCanonicalSupervisionActors(ctx)
		}
	})
}

func (rt *Runtime) sweepPendingCanonicalSupervisionActors(ctx context.Context) error {
	if rt == nil || rt.store == nil || rt.eventAppender == nil || rt.privateArtifactCipher == nil || rt.dispatchActor == nil {
		return fmt.Errorf("%w: canonical supervision recovery authority unavailable", ErrSupervisionAuthorityRequired)
	}
	agents, err := rt.store.ListComputerAgents(ctx, rt.TextureSandboxID())
	if err != nil {
		return fmt.Errorf("list canonical supervision delivery agents: %w", err)
	}
	type actorTarget struct {
		ownerID string
		agentID string
	}
	targets := make(map[string]actorTarget)
	owners := make(map[string]bool)
	for _, agent := range agents {
		ownerID, agentID := strings.TrimSpace(agent.OwnerID), strings.TrimSpace(agent.AgentID)
		if ownerID == "" || agentID == "" {
			continue
		}
		owners[ownerID] = true
		targets[ownerID+"\x00"+agentID] = actorTarget{ownerID: ownerID, agentID: agentID}
	}
	var recoveryErrors []error
	for ownerID := range owners {
		trajectories, err := rt.store.ListLifecycleTrajectoriesByOwner(ctx, ownerID, rt.TextureSandboxID(), 0)
		if err != nil {
			recoveryErrors = append(recoveryErrors, fmt.Errorf("list canonical supervision target trajectories for %s: %w", ownerID, err))
			continue
		}
		for _, trajectory := range trajectories {
			deliveries, err := rt.store.ListSupervisionDeliveryEvents(ctx, ownerID, rt.TextureSandboxID(), trajectory.TrajectoryID)
			if err != nil {
				recoveryErrors = append(recoveryErrors, fmt.Errorf("list canonical supervision targets for %s: %w", trajectory.TrajectoryID, err))
				continue
			}
			for _, delivery := range deliveries {
				if delivery.Kind != "actor_message_recorded" {
					continue
				}
				var body canonicalSupervisionDeliveryBody
				if err := json.Unmarshal(delivery.Body, &body); err != nil {
					recoveryErrors = append(recoveryErrors, fmt.Errorf("decode canonical supervision target %s: %w", delivery.ID, err))
					continue
				}
				if body.ToActorID == nil {
					continue
				}
				agentID := strings.TrimSpace(*body.ToActorID)
				if agentID != "" {
					targets[ownerID+"\x00"+agentID] = actorTarget{ownerID: ownerID, agentID: agentID}
				}
			}
		}
	}
	for ownerID := range owners {
		agentID := persistentSuperAgentID(ownerID)
		targets[ownerID+"\x00"+agentID] = actorTarget{ownerID: ownerID, agentID: agentID}
	}
	for _, target := range targets {
		updates, err := rt.canonicalSupervisionUpdatesForAgent(ctx, target.ownerID, rt.TextureSandboxID(), target.agentID, "", 0)
		if err != nil {
			recoveryErrors = append(recoveryErrors, fmt.Errorf("recover canonical supervision delivery for %s: %w", target.agentID, err))
			continue
		}
		if len(updates) == 0 {
			continue
		}
		if target.agentID == persistentSuperAgentID(target.ownerID) {
			if _, err := rt.ensurePersistentSuperAgent(ctx, target.ownerID); err != nil {
				recoveryErrors = append(recoveryErrors, fmt.Errorf("recover persistent Super identity for %s: %w", target.ownerID, err))
				continue
			}
		}
		dispatchedTrajectories := make(map[string]bool)
		for _, update := range updates {
			if dispatchedTrajectories[update.TrajectoryID] {
				continue
			}
			dispatchedTrajectories[update.TrajectoryID] = true
			if active, found, err := rt.activeRunByAgent(ctx, target.ownerID, target.agentID); err != nil {
				recoveryErrors = append(recoveryErrors, fmt.Errorf("load active canonical supervision run for %s: %w", target.agentID, err))
				continue
			} else if found &&
				metadataStringValue(active.Metadata, "request_source") == "update_coagent" &&
				trajectoryIDForRun(&active) == update.TrajectoryID {
				if err := rt.dispatchActor(ctx, active.OwnerID, active.SandboxID, active.AgentID, "initial_dispatch", active.RunID, update.TrajectoryID, ""); err != nil {
					recoveryErrors = append(recoveryErrors, fmt.Errorf("redispatch canonical supervision activation %s: %w", active.RunID, err))
				}
				continue
			}
			if err := rt.dispatchActor(ctx, update.OwnerID, update.ComputerID, update.TargetAgentID, "coagent_result", update.UpdateID, update.TrajectoryID, update.AgentID); err != nil {
				recoveryErrors = append(recoveryErrors, fmt.Errorf("dispatch recovered canonical supervision update %s: %w", update.UpdateID, err))
			}
		}
	}
	return errors.Join(recoveryErrors...)
}

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

	updates, err := rt.listAndSettlePersistentSuperBacklog(ctx, ownerID, agentID)
	if err != nil {
		return nil, err
	}
	updates = filterPersistentSuperRunnableUpdates(updates)
	if len(updates) > 0 {
		trajectoryID := updates[0].TrajectoryID
		sameTrajectory := updates[:0]
		for _, update := range updates {
			if update.TrajectoryID == trajectoryID {
				sameTrajectory = append(sameTrajectory, update)
			}
		}
		updates = sameTrajectory
	}
	if len(updates) == 0 {
		if blockedActive != nil {
			return blockedActive, nil
		}
		return nil, nil
	}

	first := updates[0]
	metadata := map[string]any{
		runMetadataAgentProfile: agentprofile.Super,
		runMetadataAgentRole:    agentprofile.Super,
		runMetadataAgentID:      agentID,
		"request_source":        "update_coagent",
		"requested_by_agent_id": first.AgentID,
		"requested_by_profile":  strings.TrimSpace(first.Role),
	}
	prompt := "Process pending coagent update packets for privileged execution."
	if first.LifecycleVersion > 0 {
		deliveryIDs := make([]string, 0, len(updates))
		for _, update := range updates {
			if id := strings.TrimSpace(update.UpdateID); id != "" {
				deliveryIDs = append(deliveryIDs, id)
			}
		}
		metadata["supervision_delivery_authority"] = "canonical"
		metadata["supervision_delivery_ids"] = deliveryIDs
	}
	if first.ChannelID != "" {
		metadata[runMetadataChannelID] = first.ChannelID
	}
	if first.TrajectoryID != "" {
		metadata[runMetadataTrajectoryID] = first.TrajectoryID
	}
	if first.WorkItemID != "" {
		metadata["lifecycle_work_item_id"] = first.WorkItemID
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

	rec, err := rt.createRunWithMetadata(ctx, prompt, ownerID, metadata)
	if err != nil {
		return nil, err
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
	if _, supervised, err := rt.supervisionSnapshotForRun(ctx, rec); err != nil {
		return err
	} else if supervised {
		return rt.store.UpdateRun(ctx, *rec)
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
	if _, supervised, err := rt.supervisionSnapshotForRun(ctx, rec); err != nil {
		return err
	} else if supervised {
		return rt.store.UpdateRun(ctx, *rec)
	}

	if err := rt.refuseLegacySupervisionWrite(ctx, rec.OwnerID, rec.SandboxID, trajectoryIDForRun(rec), "record run completion"); err != nil {
		return err
	}
	updateIDs := coagentUpdateIDsForRun(rec)
	if runHasProfile(rec, agentprofile.Texture) {
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
	if err := rt.refuseLegacySupervisionWrite(ctx, ownerID, rec.SandboxID, rec.TrajectoryID, "complete supervised work item"); err != nil {
		if errors.Is(err, ErrSupervisionAuthorityRequired) {
			return nil
		}
		return err
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

func (rt *Runtime) maybeContinueCoagentInbox(ctx context.Context, rec *types.RunRecord) {
	if rec == nil || rec.State != types.RunCompleted ||
		metadataStringValue(rec.Metadata, "request_source") != "update_coagent" {
		return
	}
	if isPersistentSuperInboxRun(rec) {
		if err := rt.markPersistentSuperRunUpdatesDelivered(ctx, rec); err != nil {
			log.Printf("runtime: mark persistent super updates delivered after %s: %v", rec.RunID, err)
			return
		}
		if _, err := rt.reconcilePersistentSuperActor(ctx, rec.OwnerID, rec.AgentID); err != nil {
			log.Printf("runtime: continue persistent super inbox after %s: %v", rec.RunID, err)
		}
		return
	}
	if _, supervised, err := rt.supervisionSnapshotForRun(ctx, rec); err != nil {
		log.Printf("runtime: resolve supervised inbox after %s: %v", rec.RunID, err)
	} else if supervised {
		if _, err := rt.reconcileUpdatedCoagentActor(ctx, rec.OwnerID, rec.AgentID); err != nil {
			log.Printf("runtime: continue supervised inbox after %s: %v", rec.RunID, err)
		}
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

func filterPersistentSuperRunnableUpdates(updates []types.CoagentSourcePacket) []types.CoagentSourcePacket {
	if len(updates) == 0 {
		return nil
	}
	out := make([]types.CoagentSourcePacket, 0, len(updates))
	for _, update := range updates {
		if persistentSuperRunnableUpdate(update) {
			out = append(out, update)
		}
	}
	return out
}

func (rt *Runtime) listAndSettlePersistentSuperBacklog(ctx context.Context, ownerID, agentID string) ([]types.CoagentSourcePacket, error) {
	const limit = 100
	canonical, err := rt.canonicalSupervisionUpdatesForAgent(ctx, ownerID, rt.TextureSandboxID(), agentID, "", 0)
	if err != nil {
		return nil, fmt.Errorf("list canonical super pending updates: %w", err)
	}
	canonical = filterPersistentSuperRunnableUpdates(canonical)
	if len(canonical) > 0 {
		return canonical, nil
	}
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
		if !persistentSuperRunnableUpdate(u) {
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

func persistentSuperRunnableUpdate(update types.CoagentSourcePacket) bool {
	if update.DeliveredAt != nil || strings.TrimSpace(update.DeliveredToRunID) != "" {
		return false
	}
	packet := normalizeCoagentSourcePacketPayload(update.Packet)
	if validateCoagentSourcePacketPayload(packet) != nil {
		return false
	}
	if packet.Kind == "execution_request" {
		return len(packet.Actions) > 0
	}
	return update.LifecycleVersion > 0
}

func coagentUpdateDeliverableForRun(rec *types.RunRecord, update types.CoagentSourcePacket) bool {
	if !isPersistentSuperAgentRun(rec) {
		return true
	}
	packet := normalizeCoagentSourcePacketPayload(update.Packet)
	if validateCoagentSourcePacketPayload(packet) != nil {
		return false
	}
	if packet.Kind == "execution_request" {
		return len(packet.Actions) > 0
	}
	return update.LifecycleVersion > 0
}

func buildPersistentSuperUpdatePrompt(updates []types.CoagentSourcePacket) string {
	var b strings.Builder
	b.WriteString("Process the pending canonical supervision records addressed to you as the user's persistent super actor.\n\n")
	b.WriteString("Each delivered packet is validated and tape-derived. Execute only packet.kind=execution_request actions; reconcile results, evidence, questions, blockers, and other typed supervision updates without inventing effect authority. Report dispositions through the supervision tools and update_coagent.\n")
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
	canonical, err := rt.canonicalSupervisionUpdatesForAgent(ctx, ownerID, rt.TextureSandboxID(), agentID, "", 0)
	if err != nil {
		return nil, fmt.Errorf("list canonical coagent updates: %w", err)
	}
	canonicalBacklog := len(canonical) > 0
	updates := canonical
	if canonicalBacklog {
		trajectoryID := canonical[0].TrajectoryID
		updates = updates[:0]
		for _, update := range canonical {
			if update.TrajectoryID == trajectoryID {
				updates = append(updates, update)
			}
		}
	} else {
		updates, err = rt.store.ListCoagentMailboxBacklog(ctx, ownerID, agentID, 100)
		if err != nil {
			return nil, fmt.Errorf("list coagent pending updates: %w", err)
		}
	}
	if len(updates) == 0 {
		return nil, nil
	}
	first := updates[0]
	profile := agentprofile.Canonical(firstNonEmpty(agent.Profile, first.Role))
	if profile == "" || profile == agentprofile.Email || profile == agentprofile.Conductor || profile == agentprofile.Super {
		return nil, nil
	}
	if canonicalBacklog && profile == agentprofile.CoSuper {
		return nil, fmt.Errorf("canonical CoSuper delivery requires a live supervised attempt; Super must open a retry attempt")
	}
	role := strings.TrimSpace(firstNonEmpty(agent.Role, profile))
	channelID := strings.TrimSpace(firstNonEmpty(agent.ChannelID, first.ChannelID))
	updateIDs := make([]string, 0, len(updates))
	if canonicalBacklog {
		for _, update := range updates {
			if id := strings.TrimSpace(update.UpdateID); id != "" {
				updateIDs = append(updateIDs, id)
			}
		}
	}
	metadata := map[string]any{
		runMetadataAgentProfile: profile,
		runMetadataAgentRole:    role,
		runMetadataAgentID:      agentID,
		"request_source":        "update_coagent",
	}
	if canonicalBacklog {
		metadata["supervision_delivery_authority"] = "canonical"
		metadata["supervision_delivery_ids"] = updateIDs
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

func filterConsumedCoagentUpdates(updates []types.CoagentSourcePacket, consumed map[string]bool) []types.CoagentSourcePacket {
	if len(updates) == 0 || len(consumed) == 0 {
		return updates
	}
	pending := updates[:0]
	for _, update := range updates {
		if !consumed[strings.TrimSpace(update.UpdateID)] {
			pending = append(pending, update)
		}
	}
	return pending
}

func mergePendingCoagentUpdates(groups [][]types.CoagentSourcePacket) []types.CoagentSourcePacket {
	seen := make(map[string]bool)
	var merged []types.CoagentSourcePacket
	for _, group := range groups {
		for _, update := range group {
			id := strings.TrimSpace(update.UpdateID)
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			merged = append(merged, update)
		}
	}
	sort.SliceStable(merged, func(i, j int) bool {
		if !merged[i].CreatedAt.Equal(merged[j].CreatedAt) {
			return merged[i].CreatedAt.Before(merged[j].CreatedAt)
		}
		return merged[i].UpdateID < merged[j].UpdateID
	})
	return merged
}

func (rt *Runtime) pendingCoagentUpdatesForRun(ctx context.Context, rec *types.RunRecord, ownerID, agentID string, limit int) ([]types.CoagentSourcePacket, error) {
	if rec == nil {
		return nil, nil
	}
	computerID := strings.TrimSpace(rec.SandboxID)
	if computerID == "" {
		return nil, fmt.Errorf("list pending coagent updates: computer_id is required")
	}
	if rt.eventAppender == nil || rt.privateArtifactCipher == nil {
		return nil, fmt.Errorf("%w: canonical supervision delivery authority unavailable", ErrSupervisionAuthorityRequired)
	}
	lifecycleRun := false
	if _, err := rt.store.GetLifecycleRun(ctx, rec.OwnerID, computerID, rec.RunID); err == nil {
		lifecycleRun = true
	} else if !errors.Is(err, store.ErrNotFound) {
		return nil, fmt.Errorf("resolve lifecycle run authority: %w", err)
	}
	consumed, err := rt.consumedCanonicalSupervisionUpdateIDs(ctx, ownerID, computerID, agentID)
	if err != nil {
		return nil, err
	}
	canonicalTrajectoryID := ""
	if lifecycleRun {
		canonicalTrajectoryID = trajectoryIDForRun(rec)
	}
	canonical, err := rt.canonicalSupervisionUpdatesForAgentWithConsumed(ctx, ownerID, computerID, agentID, canonicalTrajectoryID, 0, consumed)
	if err != nil {
		return nil, err
	}
	if len(canonical) > 0 {
		if limit > 0 && len(canonical) > limit {
			canonical = canonical[:limit]
		}
		if rec.Metadata == nil {
			rec.Metadata = map[string]any{}
		}
		rec.Metadata["supervision_delivery_authority"] = "canonical"
		deliveryIDs := make([]string, 0, len(canonical))
		for _, update := range canonical {
			if id := strings.TrimSpace(update.UpdateID); id != "" {
				deliveryIDs = append(deliveryIDs, id)
			}
		}
		rec.Metadata["supervision_delivery_ids"] = deliveryIDs
		return canonical, nil
	}
	if lifecycleRun {
		lifecycle, err := rt.store.ListPendingLifecycleUpdatesForTrajectory(ctx, ownerID, computerID, agentID, trajectoryIDForRun(rec), 0)
		if err != nil {
			return nil, err
		}
		mailbox, err := rt.store.ListCoagentMailboxBacklog(ctx, ownerID, agentID, 0)
		if err != nil {
			return nil, err
		}
		compatibility := mergePendingCoagentUpdates([][]types.CoagentSourcePacket{lifecycle, mailbox})
		compatibility = filterConsumedCoagentUpdates(compatibility, consumed)
		if limit > 0 && len(compatibility) > limit {
			compatibility = compatibility[:limit]
		}
		return compatibility, nil
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
			fresh = append(fresh, update)
			updateIDs = append(updateIDs, id)
			if len(fresh) >= 100 {
				break
			}
		}
		if len(fresh) == 0 {
			return nil, nil
		}
		if err := rt.persistPrivateTraceTaint(context.Background(), rec); err != nil {
			return nil, err
		}
		for _, id := range updateIDs {
			seen[id] = true
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
		if agentProfileForRun(rec) == agentprofile.Texture && rt.coagentUpdateEnvelopeBuilder != nil {
			return rt.coagentUpdateEnvelopeBuilder(context.Background(), rec, projected, phase)
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
	if metadataStringValue(rec.Metadata, "request_source") == "update_coagent" {
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
		if len(fresh) >= 100 {
			break
		}
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
	if metadataStringValue(rec.Metadata, "request_source") == "update_coagent" {
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
	// The coagent update is already in the store mailbox. Send an actor
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
