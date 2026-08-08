package textureowner

import (
	"context"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

const (
	textureOwnerInstructionIDsMetadata = "texture_owner_instruction_ids"
	textureOwnerRequestIDsMetadata     = "texture_owner_request_ids"
)

func (h *Handler) coagentUpdateTurnInjector(rec *types.RunRecord) toolregistry.InjectUserTurnsFunc {
	if h == nil || h.Core == nil {
		return nil
	}
	return h.Core.CoagentUpdateTurnInjector(rec)
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
