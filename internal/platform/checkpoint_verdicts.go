package platform

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
)

// ErrNoVerificationBase: no base at or below a checkpoint's head exists to
// start the host's rebuild from, and genesis is beyond the replay bound.
var ErrNoVerificationBase = errors.New("no base at or below the checkpoint head to rebuild from")

// CheckpointVerdictRecord is the host's judgment of one published checkpoint
// (docs/design/apply-checkpoint-without-replay-2026-10-26.md, slice 2).
type CheckpointVerdictRecord struct {
	ComputerID       string                            `json:"computer_id"`
	CheckpointDigest string                            `json:"checkpoint_digest"`
	AcceptedHead     string                            `json:"accepted_head"`
	CreatedAt        time.Time                         `json:"created_at"`
	Verdict          selfdevprotocol.CheckpointVerdict `json:"verdict"`
}

func (s *Store) CheckpointVerdict(ctx context.Context, computerID, checkpointDigest string) (CheckpointVerdictRecord, error) {
	return CheckpointVerdictRecord{}, fmt.Errorf("not implemented")
}

func (s *Store) PendingCheckpointVerdicts(ctx context.Context) ([]CheckpointVerdictRecord, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *Store) ResolveCheckpointVerdict(ctx context.Context, computerID, checkpointDigest string, verdict selfdevprotocol.CheckpointVerdict) (bool, error) {
	return false, fmt.Errorf("not implemented")
}

func (s *Store) StoredCheckpoint(ctx context.Context, computerID, checkpointDigest string) (selfdevprotocol.Checkpoint, error) {
	return selfdevprotocol.Checkpoint{}, fmt.Errorf("not implemented")
}

func (s *Store) ProjectionBaseAtSequence(ctx context.Context, computerID string, sequence uint64) (string, bool, error) {
	return "", false, fmt.Errorf("not implemented")
}

func (s *Store) EnqueueCheckpointVerificationJob(ctx context.Context, record CheckpointVerdictRecord) (ProjectionJob, error) {
	return ProjectionJob{}, fmt.Errorf("not implemented")
}

func (h *Handler) HandleCheckpointVerdict(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusNotImplemented, apiError{Error: "not implemented"})
}
