package textureowner

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type textureCoagentUpdatePacket struct {
	PacketType        string                      `json:"packet_type"`
	DeliveryPhase     string                      `json:"delivery_phase"`
	TargetAgentID     string                      `json:"target_agent_id,omitempty"`
	ChannelID         string                      `json:"channel_id,omitempty"`
	Updates           []types.CoagentSourcePacket `json:"updates"`
	SourceEntities    []types.SourceEntity        `json:"source_entities,omitempty"`
	SourceRejections  []coagentSourceRejection    `json:"source_rejections,omitempty"`
	SourceInstruction string                      `json:"source_instruction,omitempty"`
}

type textureOwnerInstructionPacket struct {
	PacketType    string                            `json:"packet_type"`
	DeliveryPhase string                            `json:"delivery_phase"`
	DocumentID    string                            `json:"document_id"`
	TrajectoryID  string                            `json:"trajectory_id"`
	TargetAgentID string                            `json:"target_agent_id"`
	Instructions  []types.LifecycleOwnerInstruction `json:"instructions"`
}

const (
	textureOwnerInstructionIDsMetadata = "texture_owner_instruction_ids"
	textureOwnerRequestIDsMetadata     = "texture_owner_request_ids"
)

func (h *Handler) coagentUpdateTurnInjector(rec *types.RunRecord) toolregistry.InjectUserTurnsFunc {
	if h == nil || h.Store == nil || rec == nil || agentProfileForRun(rec) != "texture" {
		return nil
	}
	ownerID := strings.TrimSpace(rec.OwnerID)
	targetAgentID := strings.TrimSpace(rec.AgentID)
	if targetAgentID == "" {
		targetAgentID = currentTextureAgentID(firstNonEmpty(metadataStringValue(rec.Metadata, "doc_id"), rec.ChannelID))
	}
	if ownerID == "" || targetAgentID == "" {
		return nil
	}
	computerID := strings.TrimSpace(rec.SandboxID)
	subject, subjectErr := h.Store.GetAgentByScope(context.Background(), ownerID, computerID, targetAgentID)
	durableLifecycle := subjectErr == nil && subject.LifecycleVersion > 0
	lifecycleRequired := metadataStringValue(rec.Metadata, "lifecycle_work_item_id") != ""
	return func(finalCheckpoint bool) ([]json.RawMessage, error) {
		if lifecycleRequired && !durableLifecycle {
			return nil, fmt.Errorf("load scoped lifecycle Texture subject: %w", subjectErr)
		}
		var updates []types.CoagentSourcePacket
		var err error
		if durableLifecycle {
			updates, err = h.Store.ListPendingLifecycleUpdates(context.Background(), ownerID, computerID, targetAgentID, 100)
		} else {
			updates, err = h.Store.ListCoagentMailboxBacklog(context.Background(), ownerID, targetAgentID, 100)
			if err == nil {
				legacy := updates[:0]
				for _, update := range updates {
					if update.LifecycleVersion <= 0 {
						legacy = append(legacy, update)
					}
				}
				updates = legacy
			}
		}
		if err != nil {
			return nil, err
		}
		phase := "mid_activation"
		if finalCheckpoint {
			phase = "final_checkpoint"
		}
		messages := make([]json.RawMessage, 0, 2)
		if durableLifecycle {
			if rec.Metadata == nil {
				rec.Metadata = make(map[string]any)
			}
			rec.Metadata[textureOwnerInstructionIDsMetadata] = []string{}
			rec.Metadata[textureOwnerRequestIDsMetadata] = []string{}
			docID := strings.TrimSpace(firstNonEmpty(metadataStringValue(rec.Metadata, "doc_id"), rec.ChannelID))
			trajectoryID := strings.TrimSpace(metadataStringValue(rec.Metadata, "trajectory_id"))
			instructions, loadErr := h.Store.ListPendingLifecycleOwnerInstructions(context.Background(), ownerID, computerID, trajectoryID, targetAgentID, 100)
			if loadErr != nil {
				return nil, loadErr
			}
			if len(instructions) > 0 {
				snapshot, snapshotErr := h.Store.GetLifecycleSnapshot(context.Background(), ownerID, computerID, trajectoryID)
				if snapshotErr != nil || snapshot.Document.DocID != docID || snapshot.Document.TrajectoryID != trajectoryID {
					return nil, fmt.Errorf("owner instruction lifecycle scope is unavailable")
				}
				openWork := make(map[string]bool)
				for _, work := range snapshot.WorkItems {
					if work.Status == types.WorkItemOpen && work.OwnerID == ownerID && work.ComputerID == computerID && work.TrajectoryID == trajectoryID && work.AssignedAgentID == targetAgentID {
						openWork[work.WorkItemID] = true
					}
				}
				instructionIDs, requestIDs := make([]string, 0, len(instructions)), make([]string, 0, len(instructions))
				for _, instruction := range instructions {
					if instruction.Schema != types.LifecycleOwnerInstructionSchemaV1 || instruction.DocumentID != docID || instruction.TrajectoryID != trajectoryID || instruction.TargetAgentID != targetAgentID || !openWork[instruction.TargetWorkItemID] {
						return nil, fmt.Errorf("owner instruction %q fails exact run/work binding", instruction.InstructionID)
					}
					instructionIDs = append(instructionIDs, instruction.InstructionID)
					requestIDs = append(requestIDs, instruction.RequestID)
				}
				rec.Metadata[textureOwnerInstructionIDsMetadata] = instructionIDs
				rec.Metadata[textureOwnerRequestIDsMetadata] = requestIDs
				payload, marshalErr := json.Marshal(textureOwnerInstructionPacket{PacketType: "owner_instruction", DeliveryPhase: phase, DocumentID: docID, TrajectoryID: trajectoryID, TargetAgentID: targetAgentID, Instructions: instructions})
				if marshalErr != nil {
					return nil, marshalErr
				}
				message, marshalErr := json.Marshal(map[string]any{"role": "user", "content": []map[string]string{{"type": "text", "text": "Choir authenticated owner instruction packet.\n\n" + string(payload)}}})
				if marshalErr != nil {
					return nil, marshalErr
				}
				messages = append(messages, message)
			}
		}
		if len(updates) > 0 {
			entities, rejections := h.evidenceSourceEntitiesAndRejectionsFromWorkerUpdates(context.Background(), ownerID, updates)
			mergeTextureSourceEntitiesIntoRunMetadata(rec, entities)
			mergeCoagentSourceRejectionsIntoRunMetadata(rec, rejections)
			packet := textureCoagentUpdatePacket{PacketType: "coagent_update", DeliveryPhase: phase, TargetAgentID: targetAgentID, Updates: updates, SourceEntities: entities, SourceRejections: rejections}
			if len(entities) > 0 {
				packet.SourceInstruction = "When writing Texture content from these updates, preserve sources as Texture source entities/transclusion refs using the listed source_entities entity_id values. Do not write ordinary URL links, markdown web links, source inventories, or Source: lines as substitutes for a listed source entity."
			}
			payload, marshalErr := json.Marshal(packet)
			if marshalErr != nil {
				return nil, fmt.Errorf("marshal Texture coagent update packet: %w", marshalErr)
			}
			message, marshalErr := json.Marshal(map[string]any{"role": "user", "content": []map[string]string{{"type": "text", "text": "Choir coagent update packet.\n\n" + string(payload)}}})
			if marshalErr != nil {
				return nil, marshalErr
			}
			messages = append(messages, message)
		}
		return messages, nil
	}
}

func agentMutationComputerID(rec *types.RunRecord) string {
	if rec == nil || strings.TrimSpace(metadataStringValue(rec.Metadata, "lifecycle_work_item_id")) == "" {
		return ""
	}
	return strings.TrimSpace(rec.SandboxID)
}

func (h *Handler) createAgentMutationForRun(ctx context.Context, rec *types.RunRecord) {
	if h == nil || h.Store == nil || rec == nil {
		return
	}
	docID := firstNonEmpty(metadataStringValue(rec.Metadata, "doc_id"), rec.ChannelID)
	if docID == "" {
		return
	}
	_ = h.Store.CreateAgentMutation(ctx, store.AgentMutation{
		DocID: docID, RunID: rec.RunID, OwnerID: rec.OwnerID, ComputerID: agentMutationComputerID(rec), State: "pending",
		ScheduledMessageSeq: int64(metadataIntValue(rec.Metadata, "scheduled_message_seq")), CreatedAt: time.Now().UTC(),
	})
}
