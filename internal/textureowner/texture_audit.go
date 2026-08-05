package textureowner

import (
	"context"
	"log"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

func (h *Handler) recordTextureAudit(ctx context.Context, action, ownerID, computerID, trajectoryID, documentID, revisionID, commandID, commandDigest string, lifecycleVersion int64) {
	if h == nil || h.Core == nil {
		return
	}
	if err := h.Core.RecordTextureAudit(ctx, agentcore.TextureAuditEntry{
		Action: action, OwnerID: ownerID, ComputerID: computerID,
		TrajectoryID: trajectoryID, DocumentID: documentID, RevisionID: revisionID,
		CommandID: commandID, CommandDigest: commandDigest, LifecycleVersion: lifecycleVersion,
	}); err != nil {
		log.Printf("texture audit: append %s for document %s: %v", action, documentID, err)
	}
}

func textureAuditDigest(values ...string) string {
	return computerevent.DigestBytes([]byte(strings.Join(values, "\x00")))
}
