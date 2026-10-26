package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"sort"

	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/projectionbase"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
)

// A checkpoint's rebuild job that was refused, or has failed this many times
// at the checkpoint's head, settles the checkpoint unverified rather than
// leaving it pending behind a job that will not run again soon.
const verificationFailureLimit = 3

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
// that has none. Oldest first: each rebuild seeds the next, so a backlog is
// a chain of cheap incremental builds, and the computer's one job row is
// never churned by a second request in the same pass. One computer's error
// does not stop the others; every error is returned.
func reconcileCheckpointVerdicts(ctx context.Context, store verdictStore, loadBase func(computerID, baseRef string) (projectionbase.Descriptor, error)) error {
	pending, err := store.PendingCheckpointVerdicts(ctx)
	if err != nil {
		return fmt.Errorf("list pending checkpoints: %w", err)
	}
	sort.SliceStable(pending, func(i, j int) bool { return pending[i].Verdict.AcceptedSequence < pending[j].Verdict.AcceptedSequence })
	var errs []error
	handled := map[string]bool{}
	for _, rec := range pending {
		if err := ctx.Err(); err != nil {
			return errors.Join(append(errs, err)...)
		}
		if handled[rec.ComputerID] {
			continue
		}
		judged, err := judgeFromHeldBase(ctx, store, loadBase, rec)
		if err != nil {
			errs = append(errs, fmt.Errorf("checkpoint %s: %w", rec.CheckpointDigest, err))
			handled[rec.ComputerID] = true
			continue
		}
		if judged {
			continue
		}
		settled, err := settleRefusedRebuild(ctx, store, rec)
		if err != nil {
			errs = append(errs, fmt.Errorf("checkpoint %s: %w", rec.CheckpointDigest, err))
			handled[rec.ComputerID] = true
			continue
		}
		if settled {
			continue
		}
		handled[rec.ComputerID] = true
		if _, err := store.EnqueueCheckpointVerificationJob(ctx, rec); err != nil {
			if errors.Is(err, platform.ErrNoVerificationBase) {
				if _, rerr := store.ResolveCheckpointVerdict(ctx, rec.ComputerID, rec.CheckpointDigest, selfdevprotocol.CheckpointVerdict{
					Status: selfdevprotocol.CheckpointVerdictUnverified, Reason: selfdevprotocol.VerdictReasonNoBase, Detail: err.Error(),
				}); rerr != nil {
					errs = append(errs, fmt.Errorf("checkpoint %s: %w", rec.CheckpointDigest, rerr))
				}
				handled[rec.ComputerID] = false
				continue
			}
			errs = append(errs, fmt.Errorf("checkpoint %s: enqueue rebuild: %w", rec.CheckpointDigest, err))
		}
	}
	return errors.Join(errs...)
}

// judgeFromHeldBase judges rec against a base the platform already holds at
// exactly its sequence. It reports false when there is no such base (or its
// bytes are gone), which asks for a rebuild instead.
func judgeFromHeldBase(ctx context.Context, store verdictStore, loadBase func(string, string) (projectionbase.Descriptor, error), rec platform.CheckpointVerdictRecord) (bool, error) {
	ref, ok, err := store.ProjectionBaseAtSequence(ctx, rec.ComputerID, rec.Verdict.AcceptedSequence)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	d, err := loadBase(rec.ComputerID, ref)
	if err != nil {
		log.Printf("checkpoint verdict computer=%s base=%s unreadable, rebuilding: %v", rec.ComputerID, ref, err)
		return false, nil
	}
	return true, judgeAgainstBase(ctx, store, rec, d)
}

// judgeAgainstBase compares the checkpoint's witness with the host's rebuild
// d at the checkpoint's head, pins d under the checkpoint so a restore to it
// starts from d with no tail, and records the verdict. The pin comes first: a
// verdict is only recorded once the base it rests on is retained.
func judgeAgainstBase(ctx context.Context, store verdictStore, rec platform.CheckpointVerdictRecord, d projectionbase.Descriptor) error {
	if d.CanonicalHead != rec.AcceptedHead || d.Sequence != rec.Verdict.AcceptedSequence {
		_, err := store.ResolveCheckpointVerdict(ctx, rec.ComputerID, rec.CheckpointDigest, selfdevprotocol.CheckpointVerdict{
			Status: selfdevprotocol.CheckpointVerdictUnverified, Reason: selfdevprotocol.VerdictReasonBaseHeadMismatch, BaseRef: d.BlobSHA256,
			Detail: fmt.Sprintf("base %s is at sequence %d head %s; the checkpoint is at %d head %s", d.BlobSHA256, d.Sequence, d.CanonicalHead, rec.Verdict.AcceptedSequence, rec.AcceptedHead),
		})
		return err
	}
	checkpoint, err := store.StoredCheckpoint(ctx, rec.ComputerID, rec.CheckpointDigest)
	if err != nil {
		return fmt.Errorf("read checkpoint: %w", err)
	}
	if err := store.PinProjectionBase(ctx, rec.ComputerID, d.BlobSHA256, projectionbase.CheckpointPinReference(rec.AcceptedHead)); err != nil {
		return fmt.Errorf("pin base %s: %w", d.BlobSHA256, err)
	}
	verdict := selfdevprotocol.JudgeCheckpointWitness(checkpoint.Request.DepositMode, checkpoint.Request.VMLocalContentWitness, d.VMLocalContentWitness)
	verdict.BaseRef = d.BlobSHA256
	_, err = store.ResolveCheckpointVerdict(ctx, rec.ComputerID, rec.CheckpointDigest, verdict)
	return err
}

// settleRefusedRebuild settles rec unverified when the computer's job is at
// rec's own head and was refused (blocked) or has failed too often. A job at
// another head, or still running, says nothing about rec.
func settleRefusedRebuild(ctx context.Context, store verdictStore, rec platform.CheckpointVerdictRecord) (bool, error) {
	job, err := store.ProjectionJob(ctx, rec.ComputerID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if job.TargetSequence != rec.Verdict.AcceptedSequence || job.TargetHead != rec.AcceptedHead {
		return false, nil
	}
	refused := job.Status == "blocked" || (job.Status == "failed" && job.Failures >= verificationFailureLimit)
	if !refused {
		return false, nil
	}
	_, err = store.ResolveCheckpointVerdict(ctx, rec.ComputerID, rec.CheckpointDigest, selfdevprotocol.CheckpointVerdict{
		Status: selfdevprotocol.CheckpointVerdictUnverified, Reason: selfdevprotocol.VerdictReasonJobBlocked,
		Detail: fmt.Sprintf("rebuild job %s after %d failures: %s", job.Status, job.Failures, job.Error),
	})
	return err == nil, err
}

// resolveVerdictsAtBase judges the pending checkpoints of d's computer whose
// head is exactly d's, and pins d under each judged checkpoint's reference so
// a restore to it starts from d with no tail. A cadence base that happens to
// land on a checkpoint's head resolves it too.
func resolveVerdictsAtBase(ctx context.Context, store verdictStore, d projectionbase.Descriptor) error {
	pending, err := store.PendingCheckpointVerdicts(ctx)
	if err != nil {
		return fmt.Errorf("list pending checkpoints: %w", err)
	}
	var errs []error
	for _, rec := range pending {
		if rec.ComputerID != d.ComputerID || rec.Verdict.AcceptedSequence != d.Sequence || rec.AcceptedHead != d.CanonicalHead {
			continue
		}
		if err := judgeAgainstBase(ctx, store, rec, d); err != nil {
			errs = append(errs, fmt.Errorf("checkpoint %s: %w", rec.CheckpointDigest, err))
		}
	}
	return errors.Join(errs...)
}
