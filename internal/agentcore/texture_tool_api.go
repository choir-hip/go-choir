package agentcore

import (
	"context"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// TextureEmailDraftRequest is the concrete publication request accepted from
// Texture ownership. The runtime remains responsible for durable appagent
// dispatch and approval-state creation.
type TextureEmailDraftRequest struct {
	DocID             string   `json:"doc_id"`
	RevisionID        string   `json:"revision_id"`
	SourceContentHash string   `json:"source_content_hash"`
	FromAlias         string   `json:"from_alias,omitempty"`
	ToAddresses       []string `json:"to_addresses"`
	CCAddresses       []string `json:"cc_addresses,omitempty"`
	BCCAddresses      []string `json:"bcc_addresses,omitempty"`
	Subject           string   `json:"subject"`
	BodyText          string   `json:"body_text"`
	SourceRefs        []string `json:"source_refs,omitempty"`
	ApprovalMode      string   `json:"approval_mode,omitempty"`
}

// RecordTextureEmailDraftRequest creates the durable Email appagent handoff
// for a Texture-owned publication request.
func (rt *Runtime) RecordTextureEmailDraftRequest(ctx context.Context, parent *types.RunRecord, in TextureEmailDraftRequest) (map[string]any, error) {
	return rt.recordEmailDraftRequest(ctx, parent, requestEmailDraftArgs{
		DocID:             in.DocID,
		RevisionID:        in.RevisionID,
		SourceContentHash: in.SourceContentHash,
		FromAlias:         in.FromAlias,
		ToAddresses:       in.ToAddresses,
		CCAddresses:       in.CCAddresses,
		BCCAddresses:      in.BCCAddresses,
		Subject:           in.Subject,
		BodyText:          in.BodyText,
		SourceRefs:        in.SourceRefs,
		ApprovalMode:      in.ApprovalMode,
	})
}

// ReconcilePersistentSuperActor starts or wakes the concrete persistent-super
// lifecycle after Texture has durably dispatched a privileged request.
func (rt *Runtime) ReconcilePersistentSuperActor(ctx context.Context, ownerID, agentID string) (*types.RunRecord, error) {
	return rt.reconcilePersistentSuperActor(ctx, ownerID, agentID)
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
	packet = normalizeCoagentSourcePacketPayload(packet)
	if err := validateCoagentSourcePacketPayload(packet); err != nil {
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
