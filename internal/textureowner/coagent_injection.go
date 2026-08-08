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
		if err != nil || len(updates) == 0 {
			return nil, err
		}
		entities, rejections := h.evidenceSourceEntitiesAndRejectionsFromWorkerUpdates(context.Background(), ownerID, updates)
		mergeTextureSourceEntitiesIntoRunMetadata(rec, entities)
		mergeCoagentSourceRejectionsIntoRunMetadata(rec, rejections)
		phase := "mid_activation"
		if finalCheckpoint {
			phase = "final_checkpoint"
		}
		packet := textureCoagentUpdatePacket{
			PacketType: "coagent_update", DeliveryPhase: phase, TargetAgentID: targetAgentID,
			Updates: updates, SourceEntities: entities, SourceRejections: rejections,
		}
		if len(entities) > 0 {
			packet.SourceInstruction = "When writing Texture content from these updates, preserve sources as Texture source entities/transclusion refs using the listed source_entities entity_id values. Do not write ordinary URL links, markdown web links, source inventories, or Source: lines as substitutes for a listed source entity."
		}
		payload, err := json.Marshal(packet)
		if err != nil {
			return nil, fmt.Errorf("marshal Texture coagent update packet: %w", err)
		}
		message, err := json.Marshal(map[string]any{"role": "user", "content": []map[string]string{{"type": "text", "text": "Choir coagent update packet.\n\n" + string(payload)}}})
		if err != nil {
			return nil, err
		}
		return []json.RawMessage{message}, nil
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
