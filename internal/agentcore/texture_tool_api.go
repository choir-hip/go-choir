package agentcore

import (
	"context"

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
