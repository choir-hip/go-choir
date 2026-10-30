package main

import (
	"context"
	"fmt"
)

// workerStore is the control-store surface one checkpointd pass uses.
type workerStore interface {
	projectionWorkStore
	CheckpointHeartbeat(ctx context.Context, problem string) error
}

// runCheckpointWorker is one pass of checkpointd: settle the published
// checkpoints and record the heartbeat. There are no projection jobs and no
// rebuilds; the host replay workers left with the projection bases.
func runCheckpointWorker(ctx context.Context, store workerStore) error {
	if store == nil {
		return fmt.Errorf("checkpoint worker requires the control store")
	}
	problem := reconcileWork(ctx, store)
	return store.CheckpointHeartbeat(ctx, problem)
}
