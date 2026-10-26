package platform

import (
	"context"
	"fmt"
	"net/http"

	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
)

// clampCheckpointListLimit bounds a listing request.
func clampCheckpointListLimit(limit int) int { return 0 }

// ListCheckpointSummaries lists a computer's checkpoints, newest first, with
// the host's verdict on each.
func (s *Store) ListCheckpointSummaries(ctx context.Context, computerID string, limit int) ([]selfdevprotocol.CheckpointSummary, error) {
	return nil, fmt.Errorf("not implemented")
}

// StoredCheckpointResponse returns a published checkpoint with its receipt.
func (s *Store) StoredCheckpointResponse(ctx context.Context, computerID, checkpointDigest string) (selfdevprotocol.CheckpointResponse, error) {
	return selfdevprotocol.CheckpointResponse{}, fmt.Errorf("not implemented")
}

// HandleCheckpointList serves a computer's checkpoint listing.
func (h *Handler) HandleCheckpointList(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusNotImplemented, apiError{Error: "not implemented"})
}
