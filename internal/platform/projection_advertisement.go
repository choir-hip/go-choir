package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
)

// Published sidecars are written only by the offline publisher after its blob
// install/head/witness readback. The request path checks that provenance's
// canonical binding; it does not synchronously rebuild a multi-GiB projection.
type projectionBaseDescriptorBinding struct {
	ComputerID    string `json:"computer_id"`
	Sequence      uint64 `json:"sequence"`
	CanonicalHead string `json:"canonical_head"`
	BlobSHA256    string `json:"blob_sha256"`
	BlobSizeBytes int64  `json:"blob_size_bytes"`
}

func (h *Handler) validateProjectionAdvertisement(ctx context.Context, input fileCASWatermarkRequest) error {
	raw, err := h.projectionBaseSidecar(input.BaseRef)
	if err != nil {
		return fmt.Errorf("published projection descriptor unavailable")
	}
	var d projectionBaseDescriptorBinding
	if err := json.Unmarshal(raw, &d); err != nil {
		return fmt.Errorf("invalid published projection descriptor: %w", err)
	}
	if d.ComputerID != input.ComputerID || d.Sequence != uint64(input.WatermarkSequence) || d.BlobSHA256 != input.BaseRef {
		return fmt.Errorf("projection descriptor request binding mismatch")
	}
	path, err := h.projectionBaseBlobPath(input.BaseRef)
	if err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() != d.BlobSizeBytes {
		return fmt.Errorf("published projection blob unavailable or size mismatch")
	}
	var head string
	if err := h.service.store.db.QueryRowContext(ctx, `SELECT event_digest FROM computer_event_append_receipts WHERE computer_id=? AND sequence=?`, input.ComputerID, input.WatermarkSequence).Scan(&head); err != nil {
		return fmt.Errorf("canonical projection sequence unavailable")
	}
	if head != d.CanonicalHead {
		return fmt.Errorf("projection base is not bound to canonical event at advertised sequence")
	}
	return nil
}
