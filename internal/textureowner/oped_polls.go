package textureowner

import (
	"context"
	"strings"
)

// ensurePlatformPolls arms this computer's polls of its corpusd feeds
// (the notices for its owner) the first time it builds a document from a
// published version. Idempotent; a computer without a platform URL does
// nothing.
func (h *Handler) ensurePlatformPolls(ctx context.Context, ownerID string) {
	if h == nil || h.Core == nil || strings.TrimSpace(ownerID) == "" {
		return
	}
	h.Core.EnsurePlatformPolls(context.WithoutCancel(ctx), ownerID, h.Core.TextureComputerID())
}
