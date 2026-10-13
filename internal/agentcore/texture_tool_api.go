package agentcore

import (
	"context"

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
