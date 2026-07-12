package runtime

import (
	"context"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// ChannelMessage is the store-backed channel message type. The old in-memory
// AgentChannel/ChannelManager (channels.go) is deleted — the actor mailbox
// replaces it. Channel messages are now purely store-backed: AppendChannelMessage
// persists, ListChannelMessages reads. This alias keeps call sites working.
type ChannelMessage = types.ChannelMessage

// ChannelPost posts a broadcast message to the channel log in the store and
// emits a channel.message event. Broadcast posts are the audit/trace surface;
// addressed wake/delivery is owned by update_coagent (actor messages).
func (rt *Runtime) ChannelPost(ctx context.Context, channelID, from, role, content string) (uint64, error) {
	return rt.ChannelCast(ctx, channelID, "", "", from, role, content)
}

// ChannelCast posts an addressed message to the store channel log and emits
// the corresponding event. Addressed wake/delivery is owned by update_coagent
// (actor messages); channel messages remain the audit/replay surface.
func (rt *Runtime) ChannelCast(ctx context.Context, channelID, toAgentID, toRunID, from, role, content string) (uint64, error) {
	trajectoryID := ""
	if runRec := toolregistry.ExecutionContextFrom(ctx).RunRecord; runRec != nil && runRec.Metadata != nil {
		if id, _ := runRec.Metadata[runMetadataTrajectoryID].(string); strings.TrimSpace(id) != "" {
			trajectoryID = strings.TrimSpace(id)
		}
	}
	message := ChannelMessage{
		ChannelID:    channelID,
		FromAgentID:  toolregistry.ExecutionContextFrom(ctx).AgentID,
		FromRunID:    toolregistry.ExecutionContextFrom(ctx).RunID,
		ToAgentID:    strings.TrimSpace(toAgentID),
		ToRunID:      strings.TrimSpace(toRunID),
		TrajectoryID: trajectoryID,
		From:         from,
		Role:         role,
		Content:      content,
		Timestamp:    time.Now().UTC(),
	}
	ownerID := toolregistry.ExecutionContextFrom(ctx).OwnerID
	if ownerID == "" && message.FromRunID != "" {
		if rec, err := rt.store.GetRun(context.Background(), message.FromRunID); err == nil {
			ownerID = rec.OwnerID
		}
	}
	if err := rt.store.AppendChannelMessage(ctx, &message, ownerID); err != nil {
		return 0, err
	}
	rt.emitChannelMessageEvent(ctx, message, ownerID)
	return uint64(message.Seq), nil
}

// ChannelRead reads messages from the store for the given channel ID since
// the provided cursor (seq) position. Returns the messages and the new cursor.
func (rt *Runtime) ChannelRead(channelID string, cursor uint64) ([]ChannelMessage, uint64, error) {
	ownerID := ""
	ctx := context.Background()
	messages, err := rt.store.ListChannelMessages(ctx, ownerID, channelID, int64(cursor), 500)
	if err != nil {
		return nil, cursor, err
	}
	var newCursor uint64 = cursor
	for _, m := range messages {
		if uint64(m.Seq) > newCursor {
			newCursor = uint64(m.Seq)
		}
	}
	return messages, newCursor, nil
}
