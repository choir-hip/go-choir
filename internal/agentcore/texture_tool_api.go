package agentcore

import (
	"context"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/coagentpacket"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// ReconcilePersistentManagementActor starts or wakes the concrete persistent-super
// lifecycle after Texture has durably dispatched a privileged request.
func (rt *Runtime) ReconcilePersistentManagementActor(ctx context.Context, ownerID, agentID string) (*types.RunRecord, error) {
	return rt.reconcilePersistentManagementActor(ctx, ownerID, agentID)
}

// EmitChannelMessageEvent publishes a newly-created durable channel message.
func (rt *Runtime) EmitChannelMessageEvent(ctx context.Context, message types.ChannelMessage, ownerID string) {
	rt.emitChannelMessageEvent(ctx, message, ownerID)
}

// WakeUpdatedCoagent wakes the concrete coagent lifecycle for a newly-created
// source packet.
func (rt *Runtime) WakeUpdatedCoagent(ctx context.Context, update types.CoagentSourcePacket) {
	rt.wakeUpdatedCoagent(ctx, update)
}

// PrepareTextureControlPacket applies the same normalization and validation as
// update_coagent without exposing that generic mutation to Texture. The caller
// still derives direction, command, control, update, target-agent, and work
// identities from authenticated runtime state.
func PrepareTextureControlPacket(packet types.CoagentSourcePacketPayload) (types.CoagentSourcePacketPayload, error) {
	packet.SchemaVersion = types.CoagentSourcePacketSchemaV1
	packet = coagentpacket.Normalize(packet)
	if err := coagentpacket.Validate(packet); err != nil {
		return types.CoagentSourcePacketPayload{}, err
	}
	return packet, nil
}

// BuildTextureLifecycleControlContent derives the only readable control body
// from the validated typed packet and runtime-resolved target binding.
func BuildTextureLifecycleControlContent(packet types.CoagentSourcePacketPayload, targetAgentID, targetWorkItemID string) string {
	update := types.CoagentSourcePacket{Packet: packet, Role: agentprofile.Texture}
	content := buildWorkerUpdateMessage(update)
	return content + "\n\nRuntime target binding:\n- target_agent_id: " + strings.TrimSpace(targetAgentID) +
		"\n- target_work_item_id: " + strings.TrimSpace(targetWorkItemID)
}
