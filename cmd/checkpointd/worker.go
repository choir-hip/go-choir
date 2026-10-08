package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
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
	if err := store.CheckpointHeartbeat(ctx, ""); err != nil {
		return nil, err
	}
	if err := store.ReconcileProjectionJobs(ctx); err != nil {
		return nil, err
	}
	j, err := store.ClaimProjectionJob(ctx)
	if err != nil || j == nil {
		return j, err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stop := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		tick := time.NewTicker(45 * time.Second)
		defer tick.Stop()
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
				if err := store.CheckpointHeartbeat(ctx, ""); err != nil {
					cancel()
					return
				}
				if err := store.ReconcileProjectionJobs(ctx); err != nil {
					log.Printf("checkpoint reconcile: %v", err)
				}
			}
		}
	}()
	defer func() {
		close(stop)
		<-stopped
	}()
	fail := func(status string, problem error) (*platform.ProjectionJob, error) {
		finishCtx, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()
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
		return fail("blocked", fmt.Errorf("checkpoint escrow unavailable: %w", err))
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
	if err := os.MkdirAll(scratch, 0700); err != nil {
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
		status := "failed"
		if errors.Is(err, projectionbase.ErrBaseRefused) {
			status = "blocked"
		}
		return fail(status, err)
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
	if err := os.RemoveAll(scratch); err != nil {
		log.Printf("checkpoint scratch cleanup computer=%s: %v", j.ComputerID, err)
	}
	return j, nil
}
