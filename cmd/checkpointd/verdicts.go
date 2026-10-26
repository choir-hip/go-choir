package main

import (
	"context"
	"fmt"

	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/projectionbase"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
)

// verdictStore is the slice of the control store the checkpoint verdict pass
// uses; the real store satisfies it.
type verdictStore interface {
	PendingCheckpointVerdicts(ctx context.Context) ([]platform.CheckpointVerdictRecord, error)
	ResolveCheckpointVerdict(ctx context.Context, computerID, checkpointDigest string, verdict selfdevprotocol.CheckpointVerdict) (bool, error)
	StoredCheckpoint(ctx context.Context, computerID, checkpointDigest string) (selfdevprotocol.Checkpoint, error)
	ProjectionBaseAtSequence(ctx context.Context, computerID string, sequence uint64) (string, bool, error)
	EnqueueCheckpointVerificationJob(ctx context.Context, record platform.CheckpointVerdictRecord) (platform.ProjectionJob, error)
	ProjectionJob(ctx context.Context, computerID string) (platform.ProjectionJob, error)
	PinProjectionBase(ctx context.Context, computerID, baseRef, reference string) error
}

// reconcileCheckpointVerdicts judges every pending checkpoint it can from a
// base the host already holds at the checkpoint's head, and asks for a
// rebuild (a projection job at that head) for the oldest one per computer
// that has none.
func reconcileCheckpointVerdicts(ctx context.Context, store verdictStore, loadBase func(computerID, baseRef string) (projectionbase.Descriptor, error)) error {
	return fmt.Errorf("not implemented")
}

// resolveVerdictsAtBase judges the pending checkpoints of d's computer whose
// head is exactly d's, and pins d under each judged checkpoint's reference so
// a restore to it starts from d with no tail.
func resolveVerdictsAtBase(ctx context.Context, store verdictStore, d projectionbase.Descriptor) error {
	return fmt.Errorf("not implemented")
}
