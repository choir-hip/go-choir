package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
)

// The host no longer rebuilds checkpoints. Projection bases, the host replay
// workers that built them and the base-backed verdicts are gone with the data
// system cutover (docs/design/data-system-build-sheet-2026-10-30.md, decision
// 7): a checkpoint is proven where it is used, by restore, which installs the
// newest snapshot at or below its head, replays the tape after it and refuses
// a witness mismatch. This pass settles each published checkpoint so none
// stays pending forever: unverified, naming why the host cannot judge it.
const (
	noHostRebuildDetail = "the host keeps no projection bases and does not rebuild checkpoints; restore reconstructs the checkpoint's head from a snapshot and the tape and refuses a witness mismatch"
)

// verdictStore is the slice of the control store the checkpoint verdict pass
// uses; the real store satisfies it.
type verdictStore interface {
	PendingCheckpointVerdicts(ctx context.Context) ([]platform.CheckpointVerdictRecord, error)
	ResolveCheckpointVerdict(ctx context.Context, computerID, checkpointDigest string, verdict selfdevprotocol.CheckpointVerdict) (bool, error)
	StoredCheckpoint(ctx context.Context, computerID, checkpointDigest string) (selfdevprotocol.Checkpoint, error)
}

// reconcileCheckpointVerdicts settles every pending checkpoint unverified. A
// checkpoint whose witness is not version 3 (a Dolt-era witness) says so with
// witness_version; every other says no_base. One checkpoint's error does not
// stop the others; every error is returned.
func reconcileCheckpointVerdicts(ctx context.Context, store verdictStore) error {
	pending, err := store.PendingCheckpointVerdicts(ctx)
	if err != nil {
		return fmt.Errorf("list pending checkpoints: %w", err)
	}
	var errs []error
	for _, rec := range pending {
		if err := ctx.Err(); err != nil {
			return errors.Join(append(errs, err)...)
		}
		verdict := selfdevprotocol.CheckpointVerdict{
			Status: selfdevprotocol.CheckpointVerdictUnverified,
			Reason: selfdevprotocol.VerdictReasonNoBase,
			Detail: noHostRebuildDetail,
		}
		checkpoint, err := store.StoredCheckpoint(ctx, rec.ComputerID, rec.CheckpointDigest)
		if err != nil {
			errs = append(errs, fmt.Errorf("checkpoint %s: read checkpoint: %w", rec.CheckpointDigest, err))
			continue
		}
		if verr := selfdevprotocol.RefuseWitnessVersion(checkpoint.Request.VMLocalContentWitness.WitnessVersion); verr != nil {
			verdict.Reason = selfdevprotocol.VerdictReasonWitnessVersion
			verdict.Detail = verr.Error()
		}
		if _, err := store.ResolveCheckpointVerdict(ctx, rec.ComputerID, rec.CheckpointDigest, verdict); err != nil {
			errs = append(errs, fmt.Errorf("checkpoint %s: %w", rec.CheckpointDigest, err))
		}
	}
	return errors.Join(errs...)
}

// projectionWorkStore is what one minute's pass uses.
type projectionWorkStore interface {
	verdictStore
}

// reconcileWork runs the minute's pass and reports a problem for the worker's
// heartbeat.
func reconcileWork(ctx context.Context, store projectionWorkStore) string {
	var problems []string
	if err := reconcileCheckpointVerdicts(ctx, store); err != nil {
		problems = append(problems, "verdicts: "+err.Error())
		log.Printf("checkpoint verdicts: %v", err)
	}
	return strings.Join(problems, " ")
}
