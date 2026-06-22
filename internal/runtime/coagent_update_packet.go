package runtime

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/types"
)

const (
	coagentPacketTypeUpdate     = "coagent_update"
	coagentPacketDeliveryMid    = "mid_activation"
	coagentPacketDeliveryFinal  = "final_checkpoint"
	coagentPacketDeliveryCold   = "cold_activation"
	coagentPacketDeliveryThread = "activation_mailbox_turn"
)

type coagentUpdatePacket struct {
	PacketType        string                    `json:"packet_type"`
	DeliveryPhase     string                    `json:"delivery_phase"`
	TargetAgentID     string                    `json:"target_agent_id,omitempty"`
	ChannelID         string                    `json:"channel_id,omitempty"`
	TrajectoryID      string                    `json:"trajectory_id,omitempty"`
	Updates           []coagentUpdatePacketItem `json:"updates"`
	SourceEntities    []textureSourceEntity     `json:"source_entities,omitempty"`
	SourceInstruction string                    `json:"source_instruction,omitempty"`
	Instruction       string                    `json:"instruction,omitempty"`
}

type coagentUpdatePacketItem struct {
	UpdateID        string                           `json:"update_id"`
	FromAgentID     string                           `json:"from_agent_id,omitempty"`
	FromRole        string                           `json:"from_role,omitempty"`
	ChannelID       string                           `json:"channel_id,omitempty"`
	MessageSeq      int64                            `json:"message_seq,omitempty"`
	Packet          types.CoagentSourcePacketPayload `json:"packet"`
	HumanProjection string                           `json:"human_projection"`
}

func buildCoagentUpdateUserMessages(updates []types.CoagentSourcePacket, deliveryPhase string, targetAgentID string, sourceEntities []textureSourceEntity) ([]json.RawMessage, []string, error) {
	if len(updates) == 0 {
		return nil, nil, nil
	}
	packet := coagentUpdatePacket{
		PacketType:        coagentPacketTypeUpdate,
		DeliveryPhase:     deliveryPhase,
		TargetAgentID:     strings.TrimSpace(targetAgentID),
		SourceEntities:    sourceEntities,
		SourceInstruction: coagentUpdateSourceInstruction(sourceEntities),
		Instruction:       coagentUpdateInstruction(deliveryPhase),
		Updates:           make([]coagentUpdatePacketItem, 0, len(updates)),
	}
	updateIDs := make([]string, 0, len(updates))
	for _, update := range updates {
		id := strings.TrimSpace(update.UpdateID)
		if id != "" {
			updateIDs = append(updateIDs, id)
		}
		if packet.ChannelID == "" {
			packet.ChannelID = strings.TrimSpace(update.ChannelID)
		}
		if packet.TrajectoryID == "" {
			packet.TrajectoryID = strings.TrimSpace(update.TrajectoryID)
		}
		packet.Updates = append(packet.Updates, coagentUpdatePacketItem{
			UpdateID:        id,
			FromAgentID:     strings.TrimSpace(update.AgentID),
			FromRole:        strings.TrimSpace(update.Role),
			ChannelID:       strings.TrimSpace(update.ChannelID),
			MessageSeq:      update.MessageSeq,
			Packet:          normalizeCoagentSourcePacketPayload(update.Packet),
			HumanProjection: strings.TrimSpace(update.Content),
		})
	}
	packetJSON, err := json.Marshal(packet)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal coagent update packet: %w", err)
	}
	text := strings.TrimSpace(fmt.Sprintf("%s\n\n%s", coagentUpdatePacketPreamble(deliveryPhase), string(packetJSON)))
	msg, err := json.Marshal(map[string]any{
		"role": "user",
		"content": []map[string]string{{
			"type": "text",
			"text": text,
		}},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("marshal coagent update user message: %w", err)
	}
	return []json.RawMessage{msg}, updateIDs, nil
}

func coagentUpdateSourceInstruction(sourceEntities []textureSourceEntity) string {
	if len(sourceEntities) == 0 {
		return ""
	}
	return "When writing Texture content from these updates, preserve sources as Texture source entities/transclusion refs using the listed source_entities entity_id values. Do not write ordinary URL links, markdown web links, source inventories, or Source: lines as substitutes for a listed source entity."
}

func coagentUpdatePacketPreamble(deliveryPhase string) string {
	switch deliveryPhase {
	case coagentPacketDeliveryFinal:
		return "Choir coagent update packet (final checkpoint before ending this activation)."
	case coagentPacketDeliveryThread:
		return "Choir coagent update packet (activation mailbox turn)."
	case coagentPacketDeliveryCold:
		return "Choir coagent update packet (cold activation backlog)."
	default:
		return "Choir coagent update packet (mid-activation delivery)."
	}
}

func coagentUpdateInstruction(deliveryPhase string) string {
	switch deliveryPhase {
	case coagentPacketDeliveryFinal:
		return "New update_coagent records arrived before this activation finished. Process them before ending the turn."
	case coagentPacketDeliveryThread:
		return "Pending update_coagent records are appended as the first mailbox turn for this activation. Process them before continuing."
	case coagentPacketDeliveryCold:
		return "Pending update_coagent records are being delivered at activation start. Incorporate them before continuing."
	default:
		return "New update_coagent records arrived while this activation was running. Treat this packet as the next user turn."
	}
}
