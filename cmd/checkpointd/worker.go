package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/keyescrow"
	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/projectionbase"
)

type checkpointReplaySource struct {
	*projectionbase.DiskEventSource
	cas      *platform.ComputerEventCAS
	events   *platform.EventArtifactService
	verifier computerevent.EventHeadReceiptVerifier
}

func (s checkpointReplaySource) Head(ctx context.Context, id string) (*computerevent.Head, error) {
	return s.cas.Head(ctx, id)
}

func (s checkpointReplaySource) EventsPage(ctx context.Context, id string, after uint64, limit int) ([]computerevent.DurableEvent, error) {
	return s.events.EventsPage(ctx, id, after, limit)
}

func (s checkpointReplaySource) EventsAfter(ctx context.Context, id string, after uint64) ([]computerevent.DurableEvent, error) {
	var all []computerevent.DurableEvent
	for {
		page, err := s.EventsPage(ctx, id, after, computerevent.EventReplayMaxPageSize)
		if err != nil {
			return nil, err
		}
		if len(page) == 0 {
			return all, nil
		}
		all = append(all, page...)
		after = page[len(page)-1].Request.Event.Sequence
	}
}

func (s checkpointReplaySource) VerifyEventHeadReceipt(ctx context.Context, r computerevent.Receipt, q computerevent.CASRequest) error {
	return s.verifier.VerifyEventHeadReceipt(ctx, r, q)
}

func (s checkpointReplaySource) CompareAndSwap(context.Context, computerevent.CASRequest) (computerevent.Receipt, error) {
	return computerevent.Receipt{}, fmt.Errorf("checkpoint worker cannot append semantic events")
}

func (s checkpointReplaySource) PinEvent(context.Context, string, []byte, string) (computerevent.PinResult, error) {
	return computerevent.PinResult{}, fmt.Errorf("checkpoint worker cannot pin semantic events")
}

func runCheckpointWorker(ctx context.Context, svc *platform.Service, store *platform.Store, escrow keyescrow.PrivateKey, scratchRoot string, memoryLimit int64) (*platform.ProjectionJob, error) {
	if svc == nil || store == nil {
		return nil, fmt.Errorf("checkpoint worker requires control store and service")
	}
	// A reconcile error (one computer that cannot be enqueued) is recorded on
	// the worker heartbeat and does not stop already-queued work.
	// Judge published checkpoints and queue their rebuilds before checkpointd's
	// own cadence jobs (reconcileProjectionWork). Best effort: a problem is
	// recorded on the heartbeat and never stops already-queued work.
	loadBase := func(computerID, baseRef string) (projectionbase.Descriptor, error) {
		d, _, err := projectionbase.LoadPublishedBase(svc.ArtifactsRoot(), computerID, baseRef)
		return d, err
	}
	reconcileProblem := reconcileProjectionWork(ctx, store, loadBase)
	if err := store.CheckpointHeartbeat(ctx, reconcileProblem); err != nil {
		return nil, err
	}
	// Orphan scratch sweep at start: best effort, never fails the run
	// (docs/problems/platform-artifact-retention-2026-10-22.md, F1).
	sweepScratch := func() {
		if ctx.Err() != nil {
			return
		}
		res, err := sweepOrphanScratch(scratchSweepConfig{
			Root:   scratchRoot,
			LogDir: filepath.Join(svc.ArtifactsRoot(), "artifact-gc-log"),
			MinAge: scratchSweepMinAge,
			Now:    time.Now(),
			Jobs: func() (map[string]scratchJobState, error) {
				rows, err := store.ListProjectionJobs(ctx)
				if err != nil {
					return nil, err
				}
				out := make(map[string]scratchJobState, len(rows))
				for _, r := range rows {
					out[r.ComputerID] = scratchJobState{Generation: r.Generation, Status: r.Status, GenesisRepair: r.GenesisRepair}
				}
				return out, nil
			},
		})
		if err != nil {
			log.Printf("checkpoint scratch sweep: %v", err)
		}
		if len(res.Removed) > 0 || len(res.Kept) > 0 {
			log.Printf("checkpoint scratch sweep removed=%d bytes=%d kept_genesis_repair=%d", len(res.Removed), res.BytesRemoved, len(res.Kept))
		}
	}
	sweepScratch()
	j, err := store.ClaimProjectionJob(ctx)
	if err != nil || j == nil {
		return j, err
	}
	ctx, cancel := checkpointJobContext(ctx)
	defer cancel()
	stop := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		tick := time.NewTicker(45 * time.Second)
		defer tick.Stop()
		lastSweep := time.Now()
		for {
			select {
			case <-stop:
				return
			case <-ctx.Done():
				return
			case <-tick.C:
				if err := store.RenewProjectionJob(ctx, *j); err != nil {
					cancel()
					return
				}
				problem := reconcileProjectionWork(ctx, store, loadBase)
				if err := store.CheckpointHeartbeat(ctx, problem); err != nil {
					cancel()
					return
				}
				if time.Since(lastSweep) >= scratchSweepInterval {
					lastSweep = time.Now()
					sweepScratch()
				}
			}
		}
	}()
	defer func() {
		close(stop)
		<-stopped
	}()
	fail := func(status string, problem error) (*platform.ProjectionJob, error) {
		status = projectionFailStatus(status, problem)
		// Backstop for causes a %v wrap lost: a job whose own context is gone
		// was stopped, not refused.
		if status == "blocked" && ctx.Err() != nil {
			status = "failed"
		}
		finishCtx, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()
		// A seed refused deterministically is counted; twice quarantines it
		// and the next enqueue falls back to an older base or to none.
		if countsAgainstSeedBase(status, j.SeedBaseRef, problem) {
			if quarantined, qerr := store.RecordBaseRefusal(finishCtx, j.ComputerID, j.SeedBaseRef, problem.Error()); qerr != nil {
				log.Printf("checkpoint base refusal computer=%s: %v", j.ComputerID, qerr)
			} else if quarantined {
				log.Printf("checkpoint base quarantined computer=%s base=%s", j.ComputerID, j.SeedBaseRef)
			}
		}
		if status == "failed" && j.Failures < 1 {
			status = "retry"
		}
		if err := store.FinishProjectionJob(finishCtx, *j, status, problem.Error(), j.WatermarkSequence, ""); err != nil {
			return j, fmt.Errorf("%w; persist failure: %v", problem, err)
		}
		j.Status = status
		j.Error = problem.Error()
		j.Failures++
		_ = store.CheckpointHeartbeat(finishCtx, problem.Error())
		return j, problem
	}
	if j.SeedBaseRef == "" && !j.GenesisRepair && j.TargetSequence > projectionbase.MaxRecoveryTailEvents {
		return fail("blocked", fmt.Errorf("projection base missing beyond bootstrap bound; explicit genesis repair required"))
	}
	wrapped, digest, err := store.GetKeyEscrow(ctx, j.ComputerID, keyescrow.ProtectorCustodian)
	if err != nil {
		// Only a missing escrow is a deterministic refusal; a store error or a
		// cancelled context reading it is transient.
		status := "failed"
		if errors.Is(err, platform.ErrKeyEscrowNotFound) {
			status = "blocked"
		}
		return fail(status, fmt.Errorf("checkpoint escrow unavailable: %w", err))
	}
	var record keyescrow.WrappedKey
	if err := json.Unmarshal(wrapped, &record); err != nil {
		return fail("blocked", fmt.Errorf("checkpoint escrow record invalid"))
	}
	if digest != j.KeyDigest || record.KeyDigest != digest {
		return fail("blocked", fmt.Errorf("checkpoint escrow changed since job admission"))
	}
	if err := store.AuditCheckpointKeyUse(ctx, *j, digest); err != nil {
		return fail("failed", err)
	}
	key, err := keyescrow.OpenDEK(escrow, &record, j.ComputerID)
	if err != nil {
		return fail("blocked", fmt.Errorf("checkpoint escrow unwrap refused: %w", err))
	}
	defer clear(key)
	cas, events, _, err := svc.ComputerEventRuntime()
	if err != nil {
		return fail("failed", err)
	}
	source := checkpointReplaySource{
		DiskEventSource: projectionbase.NewDiskEventSource(svc.ArtifactsRoot(), j.ComputerID, j.TargetHead),
		cas:             cas,
		events:          events,
		verifier:        computerevent.EventHeadReceiptVerifier{Keys: svc.ControlKeyResolver()},
	}
	scratch := filepath.Join(scratchRoot, fmt.Sprintf("%s-%d", j.ComputerID, j.Generation))
	if removed := pruneSupersededScratch(scratchRoot, j.ComputerID, j.Generation); len(removed) > 0 {
		log.Printf("checkpoint scratch pruned computer=%s superseded=%d", j.ComputerID, len(removed))
	}
	if err := prepareScratchDir(scratch, j.GenesisRepair); err != nil {
		return fail("failed", err)
	}
	cfg := projectionbase.Config{
		ComputerID:     j.ComputerID,
		TargetHead:     j.TargetHead,
		TargetSequence: j.TargetSequence,
		ArtifactsRoot:  svc.ArtifactsRoot(),
		ScratchDir:     scratch,
		KeyMaterial:    key,
		MemoryLimitRSS: memoryLimit,
		SeedPolicy:     projectionbase.SeedPinned,
		SeedBaseRef:    j.SeedBaseRef,
		SeedRequired:   true,
	}
	if j.SeedBaseRef == "" {
		cfg.SeedPolicy = projectionbase.SeedNone
		cfg.SeedRequired = false
		if !j.GenesisRepair {
			cfg.MaxTailEvents = projectionbase.MaxRecoveryTailEvents
		}
	}
	rebuilder, err := projectionbase.NewRebuilder(cfg)
	if err != nil {
		return fail("blocked", err)
	}
	started := time.Now()
	result, err := rebuilder.Run(ctx, source)
	if err != nil {
		return fail(projectionRunStatus(err), err)
	}
	d := result.Descriptor
	if d.ComputerID != j.ComputerID || d.Sequence != j.TargetSequence || d.CanonicalHead != j.TargetHead {
		return fail("blocked", fmt.Errorf("checkpoint result does not match frozen target"))
	}
	if err := svc.PublishCheckpointResult(ctx, *j, d.Sequence, d.BlobSHA256, j.SeedBaseRef); err != nil {
		return fail("failed", err)
	}
	wm, ref, err := store.ReplayWatermark(ctx, j.ComputerID)
	if err != nil || uint64(wm) < d.Sequence || (uint64(wm) == d.Sequence && ref != d.BlobSHA256) {
		return fail("failed", fmt.Errorf("checkpoint advertisement readback mismatch: %v", err))
	}
	if err := store.FinishProjectionJob(ctx, *j, "succeeded", "", uint64(wm), ref); err != nil {
		return j, err
	}
	j.Status = "succeeded"
	j.WatermarkSequence = uint64(wm)
	j.BaseRef = ref
	log.Printf("checkpoint published computer=%s target=%d bytes=%d duration=%s", j.ComputerID, d.Sequence, d.BlobSizeBytes, time.Since(started))
	// The base is at exactly this head: judge the checkpoints bound to it. A
	// failure leaves them pending for the next pass; the job has succeeded.
	if err := resolveVerdictsAtBase(ctx, store, d); err != nil {
		log.Printf("checkpoint verdicts computer=%s: %v", j.ComputerID, err)
	}
	if err := removeScratchDir(scratch); err != nil {
		log.Printf("checkpoint scratch cleanup computer=%s: %v", j.ComputerID, err)
	}
	return j, nil
}

// transientProjectionCause reports whether err carries a cancelled or expired
// context. That is the unit stopping or the job's deadline, not a verdict on
// the base, so it must never be recorded as a deterministic refusal even when
// the read-back path wrapped it in ErrBaseRefused
// (docs/problems/projection-job-blocked-by-a-cancel-2026-10-26.md).
func transientProjectionCause(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

// projectionRunStatus classifies a rebuilder.Run error: `blocked` only for a
// deterministic refusal, `failed` (retried after backoff) for everything else,
// including a cancel or deadline wrapped inside a refusal.
func projectionRunStatus(err error) string {
	if errors.Is(err, projectionbase.ErrBaseRefused) && !transientProjectionCause(err) {
		return "blocked"
	}
	return "failed"
}

// projectionFailStatus demotes a call site's `blocked` to `failed` when its
// error carries a transient cause; every other status passes through.
func projectionFailStatus(status string, err error) string {
	if status == "blocked" && transientProjectionCause(err) {
		return "failed"
	}
	return status
}

// checkpointJobDeadline bounds one job below the service's TimeoutStartSec of
// 10 minutes (nix/node-b.nix), leaving time to record the outcome: without it
// one large job runs until systemd kills the unit, records nothing, and starves
// every job behind it. Expiry is a transient cause (`failed`, backed off).
const checkpointJobDeadline = 8 * time.Minute

func checkpointJobContext(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, checkpointJobDeadline)
}

// countsAgainstSeedBase reports whether a job outcome is a deterministic
// refusal of its seed base's own bytes (ErrSeedRefused), the only kind that
// counts toward quarantine; target, tail, scratch and IO causes do not.
func countsAgainstSeedBase(status, seedBaseRef string, err error) bool {
	return status == "blocked" && seedBaseRef != "" && errors.Is(err, projectionbase.ErrSeedRefused) && !transientProjectionCause(err)
}
