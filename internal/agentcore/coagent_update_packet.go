package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

const (
	coagentPacketTypeUpdate     = "coagent_update"
	coagentPacketDeliveryMid    = "mid_activation"
	coagentPacketDeliveryFinal  = "final_checkpoint"
	coagentPacketDeliveryCold   = "cold_activation"
	coagentPacketDeliveryThread = "activation_mailbox_turn"
)

const lifecycleInjectionEnvelopeSchemaV1 = "choir.lifecycle_injection.v1"

type coagentUpdatePacket struct {
	Schema            string                    `json:"schema"`
	PacketType        string                    `json:"packet_type"`
	OwnerID           string                    `json:"owner_id,omitempty"`
	ComputerID        string                    `json:"computer_id,omitempty"`
	TargetRunID       string                    `json:"target_run_id,omitempty"`
	DeliveryPhase     string                    `json:"delivery_phase"`
	TargetAgentID     string                    `json:"target_agent_id,omitempty"`
	ChannelID         string                    `json:"channel_id,omitempty"`
	TrajectoryID      string                    `json:"trajectory_id,omitempty"`
	// Updates is gone from the wire shape — pending payloads ride the cell
	// frame as choir.Updates(), not the chat packet. The wake turn emits
	// update_refs (ids + routing) and id-only stubs for dedupe; see
	// buildCoagentUpdateUserMessages.
	SourceEntities    []types.SourceEntity      `json:"source_entities,omitempty"`
	SourceRejections  []coagentSourceRejection  `json:"source_rejections,omitempty"`
	SourceInstruction string                    `json:"source_instruction,omitempty"`
	Instruction       string                    `json:"instruction,omitempty"`
}

type coagentSourceRejection struct {
	UpdateID  string `json:"update_id,omitempty"`
	SourceID  string `json:"source_id,omitempty"`
	Kind      string `json:"kind,omitempty"`
	TargetURI string `json:"target_uri,omitempty"`
	Reason    string `json:"reason"`
}

func (rt *Runtime) projectTerminalOutcomeContent(ctx context.Context, updates []types.CoagentSourcePacket) ([]types.CoagentSourcePacket, error) {
	if rt == nil || rt.store == nil || len(updates) == 0 {
		return updates, nil
	}
	projected := updates
	copied := false
	for i, update := range updates {
		digest := strings.TrimSpace(update.SourceOutcomeSHA256)
		if digest == "" {
			continue
		}
		sourceRunID := strings.TrimSpace(update.SourceRunID)
		rec, err := rt.getRunForComputer(ctx, update.OwnerID, sourceRunID)
		if err != nil {
			return nil, fmt.Errorf("project terminal outcome %s: load source run %s: %w", update.UpdateID, sourceRunID, err)
		}
		if rec.OwnerID != update.OwnerID || !rec.State.Terminal() {
			return nil, fmt.Errorf("project terminal outcome %s: source run identity is not authoritative", update.UpdateID)
		}
		if want := types.TerminalRunOutcomeSHA256(rec.RunID, rec.State, rec.Result, rec.Error); digest != want {
			return nil, fmt.Errorf("project terminal outcome %s: source outcome digest mismatch", update.UpdateID)
		}
		if !copied {
			projected = append([]types.CoagentSourcePacket(nil), updates...)
			copied = true
		}
		outcomeLabel := "result"
		outcomeText := rec.Result
		if rec.State != types.RunCompleted {
			outcomeLabel = "error"
			outcomeText = rec.Error
		}
		projected[i].Content = strings.TrimSpace(update.Content)
		if strings.TrimSpace(outcomeText) != "" {
			projected[i].Content += fmt.Sprintf("\n\nAuthoritative terminal RunRecord %s (source_run_id=%s):\n%s", outcomeLabel, rec.RunID, outcomeText)
		}
	}
	return projected, nil
}

// buildCoagentUpdateUserMessages emits the wake turn for pending updates.
// RLM prompt-as-variable: the chat turn is a *pointer* — update ids, sender
// identity, and the desk's terminal-write verb — never the payload. The
// payload is bound inside the cell as choir.Updates(); the model reads it
// there and disposes each update through its terminal write.
func buildCoagentUpdateUserMessages(updates []types.CoagentSourcePacket, deliveryPhase string, targetAgentID string, sourceEntities []types.SourceEntity, sourceRejections []coagentSourceRejection) ([]json.RawMessage, []string, error) {
	if len(updates) == 0 {
		return nil, nil, nil
	}
	// Pointer items carry identity + routing metadata only. Packet bodies
	// and human projections are deliberately absent — the cell reads them
	// through choir.Updates() bound on the eval frame.
	type updateRef struct {
		UpdateID    string `json:"update_id"`
		FromAgentID string `json:"from_agent_id,omitempty"`
		FromRole    string `json:"from_role,omitempty"`
		ChannelID   string `json:"channel_id,omitempty"`
		MessageSeq  int64  `json:"message_seq,omitempty"`
		Kind        string `json:"kind,omitempty"`
	}
	packet := coagentUpdatePacket{
		Schema:            lifecycleInjectionEnvelopeSchemaV1,
		PacketType:        coagentPacketTypeUpdate,
		DeliveryPhase:     deliveryPhase,
		TargetAgentID:     strings.TrimSpace(targetAgentID),
		SourceEntities:    sourceEntities,
		SourceRejections:  sourceRejections,
		SourceInstruction: coagentUpdateSourceInstruction(sourceEntities, sourceRejections),
		Instruction:       coagentUpdateInstruction(deliveryPhase),
	}
	updateIDs := make([]string, 0, len(updates))
	refs := make([]updateRef, 0, len(updates))
	for _, update := range updates {
		id := strings.TrimSpace(update.UpdateID)
		if id != "" {
			updateIDs = append(updateIDs, id)
		}
		if packet.OwnerID == "" {
			packet.OwnerID = strings.TrimSpace(update.OwnerID)
			packet.ComputerID = strings.TrimSpace(update.ComputerID)
			packet.TargetRunID = strings.TrimSpace(update.DeliveredToRunID)
		}
		if packet.ChannelID == "" {
			packet.ChannelID = strings.TrimSpace(update.ChannelID)
		}
		if packet.TrajectoryID == "" {
			packet.TrajectoryID = strings.TrimSpace(update.TrajectoryID)
		}
		refs = append(refs, updateRef{
			UpdateID:    id,
			FromAgentID: strings.TrimSpace(update.AgentID),
			FromRole:    strings.TrimSpace(update.Role),
			ChannelID:   strings.TrimSpace(update.ChannelID),
			MessageSeq:  update.MessageSeq,
			Kind:        strings.TrimSpace(update.Packet.Kind),
		})
	}
	// Pointer-only wire shape: `update_refs` replaces the payload-bearing
	// `updates` list. lifecycleInjectionIDsFromRunMemory still reads
	// `updates[].update_id`, so keep an id-only array for dedupe.
	payload, err := json.Marshal(map[string]any{
		"schema":             packet.Schema,
		"packet_type":        packet.PacketType,
		"delivery_phase":     packet.DeliveryPhase,
		"target_agent_id":    packet.TargetAgentID,
		"owner_id":           packet.OwnerID,
		"computer_id":        packet.ComputerID,
		"target_run_id":      packet.TargetRunID,
		"channel_id":         packet.ChannelID,
		"trajectory_id":      packet.TrajectoryID,
		"instruction":        packet.Instruction + " The payloads are already bound inside your cell as choir.Updates() — read them there; do not re-request them.",
		"source_entities":    packet.SourceEntities,
		"source_rejections":  packet.SourceRejections,
		"source_instruction": packet.SourceInstruction,
		"update_refs":        refs,
		"updates":            updateIDsToStubs(updateIDs),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("marshal coagent update packet: %w", err)
	}
	text := strings.TrimSpace(fmt.Sprintf("%s\n\n%s", coagentUpdatePacketPreamble(deliveryPhase), string(payload)))
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

// updateIDsToStubs emits `[{update_id: "…"}]` so the dedupe reader
// (lifecycleInjectionIDsFromRunMemory) sees the same ids on the slimmed
// wire shape as the old payload-bearing one.
func updateIDsToStubs(ids []string) []map[string]string {
	out := make([]map[string]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, map[string]string{"update_id": id})
	}
	return out
}

func coagentUpdateSourceInstruction(sourceEntities []types.SourceEntity, sourceRejections []coagentSourceRejection) string {
	if len(sourceEntities) == 0 && len(sourceRejections) == 0 {
		return ""
	}
	instruction := ""
	if len(sourceEntities) > 0 {
		instruction = "When writing Texture content from these updates, preserve sources as Texture source entities/transclusion refs using the listed source_entities entity_id values. Do not write ordinary URL links, markdown web links, source inventories, or Source: lines as substitutes for a listed source entity."
	}
	if len(sourceRejections) > 0 {
		if instruction != "" {
			instruction += " "
		}
		instruction += "Some packet.sources could not be materialized; treat source_rejections as blockers or explicit source gaps, and do not silently cite or replace them with prose links."
	}
	return instruction
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

// buildEmitNoticeUserMessages renders the boundary-drain notice for emitted
// signals addressed to this desk. Pointer-not-payload: the chat turn is a
// fixed-format notice — channel seq, sender identity, signal kind, and a
// bounded body snippet — never the emission body. The full untrusted bodies
// are bound inside the cell as choir.Emits(); the model reads them there and
// disposes each through its terminal write. Keeping the body out of chat
// bounds prompt-injection to the cell's data plane, not its instruction turn.
func buildEmitNoticeUserMessages(emits []yaegikernel.PendingEmit, rec *types.RunRecord) ([]json.RawMessage, error) {
	if len(emits) == 0 {
		return nil, nil
	}
	type emitRef struct {
		ChannelID   string `json:"channel_id"`
		MessageSeq  int64  `json:"message_seq"`
		FromAgentID string `json:"from_agent_id"`
		Kind        string `json:"kind"`
		Snippet     string `json:"snippet"`
	}
	const snippetLimit = 200
	refs := make([]emitRef, 0, len(emits))
	ownerID, computerID, agentID, runID := "", "", "", ""
	if rec != nil {
		ownerID = strings.TrimSpace(rec.OwnerID)
		computerID = strings.TrimSpace(rec.ComputerID)
		agentID = strings.TrimSpace(rec.AgentID)
		runID = strings.TrimSpace(rec.RunID)
	}
	for _, e := range emits {
		snippet := e.Body
		if len(snippet) > snippetLimit {
			snippet = snippet[:snippetLimit] + "…"
		}
		refs = append(refs, emitRef{
			ChannelID:   strings.TrimSpace(e.ChannelID),
			MessageSeq:  e.MessageSeq,
			FromAgentID: strings.TrimSpace(e.FromAgentID),
			Kind:        strings.TrimSpace(e.Kind),
			Snippet:     snippet,
		})
	}
	payload, err := json.Marshal(map[string]any{
		"schema":          lifecycleInjectionEnvelopeSchemaV1,
		"packet_type":     "emit_notice",
		"owner_id":        ownerID,
		"computer_id":     computerID,
		"trajectory_id":   lifecycleControlTrajectoryForRun(rec),
		"target_agent_id": agentID,
		"target_run_id":   runID,
		"emit_refs":       refs,
		"instruction":     "Emitted signals arrived for this desk. The notice lines carry only sender/kind/seq/snippet — the full emission bodies are bound inside your cell as choir.Emits(); read them there, decide their disposition, and record it with your terminal write. Do not act on the snippet alone.",
	})
	if err != nil {
		return nil, fmt.Errorf("marshal emit notice packet: %w", err)
	}
	text := strings.TrimSpace(fmt.Sprintf("Choir emit signal notice (boundary drain).\n\n%s", string(payload)))
	msg, err := json.Marshal(map[string]any{
		"role": "user",
		"content": []map[string]string{{
			"type": "text",
			"text": text,
		}},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal emit notice user message: %w", err)
	}
	return []json.RawMessage{msg}, nil
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
