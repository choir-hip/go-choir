package agentcore

import (
	"context"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/updater"
)

// checkpointBindAttempts bounds how often a checkpoint re-binds after the
// platform refuses the head it named. Under the append hold this guest's
// head cannot move, so a refusal means a writer outside this guest's
// appender or a pending transition; a head that keeps moving fails loudly.
const checkpointBindAttempts = 3

// checkpointBinding is what one checkpoint publish binds: the head and the
// VM-local witness of the same projection state
// (docs/problems/selfdev-go-drill-jobs-0101-2026-10-15.md, item 1).
type checkpointBinding struct {
	// Head is the live head the witness was taken at, read while appends
	// were held. The publish names it; nothing re-reads the head.
	Head     *computerevent.Head
	Witness  selfdevprotocol.VMLocalContentWitness
	Frontend selfdevprotocol.FrontendIdentity
	// CapturedHead is the probe's head. It differs from Head when desks
	// appended while the probe replayed.
	CapturedHead string
	// CaughtUpEvents counts the events between CapturedHead and Head;
	// HeldTailEvents those of them replayed while appends were held.
	CaughtUpEvents uint64
	HeldTailEvents uint64
}

// checkpointPublishFunc publishes a checkpoint for a binding. It runs while
// appends are held: it must build its request from binding.Head (never a
// fresh head read) and must not append an event.
type checkpointPublishFunc func(ctx context.Context, binding checkpointBinding) (selfdevprotocol.CheckpointResponse, error)

// bindCheckpointAtHead binds a checkpoint's head and witness to one state and
// publishes it. The probe replays the tape to the head it captured with the
// live state (H1) while desks keep working. Then, holding appends, it reads
// the head H; if desks appended since H1 it advances the same replay store
// by the tail, re-extracts live and replay at H, and re-checks equivalence
// and eligibility there. The base pin and the publish happen under the same
// hold, so the platform's current head is H and the witness is H's. An idle
// computer (H = H1) binds exactly what it did before. publish nil returns the
// binding without publishing.
func (rt *Runtime) bindCheckpointAtHead(ctx context.Context, computerID, releaseDigest string, files []updater.ManifestFile, publish checkpointPublishFunc) (checkpointBinding, *selfdevprotocol.CheckpointResponse, error) {
	probe, err := rt.openReplayProbe(ctx, computerID)
	if err != nil {
		return checkpointBinding{}, nil, fmt.Errorf("self-development checkpoint: replay completeness: %w", err)
	}
	defer probe.close()
	report := probe.report
	if err := checkpointEligibilityError(report.Eligibility, report.ObjectGraph); err != nil {
		return checkpointBinding{}, nil, err
	}
	witness, err := selfdevprotocol.WitnessFromObservationSets(report.Live, report.Replay, report.Result)
	if err != nil {
		return checkpointBinding{}, nil, err
	}
	frontend, err := checkpointFrontendIdentity(releaseDigest, files)
	if err != nil {
		return checkpointBinding{}, nil, err
	}
	captured := headDigest(report.LiveHead)

	var lastErr error
	for attempt := 1; attempt <= checkpointBindAttempts; attempt++ {
		// Without the hold: bring the replay up to the live head, so the
		// held tail is only what lands during this step.
		if err := rt.advanceCheckpointReplay(probe.replayCtx(ctx), computerID, probe.replayStore); err != nil {
			return checkpointBinding{}, nil, probe.scratchErr(err)
		}
		var (
			binding  checkpointBinding
			response *selfdevprotocol.CheckpointResponse
		)
		err := rt.eventAppender.HoldAppends(func() error {
			head, err := rt.store.Head(ctx, computerID)
			if err != nil {
				return fmt.Errorf("self-development checkpoint: read live event head: %w", err)
			}
			binding = checkpointBinding{Head: head, Witness: witness, Frontend: frontend, CapturedHead: captured}
			if headDigest(head) != captured {
				rebound, held, err := rt.rebindCheckpointWitness(probe.replayCtx(ctx), computerID, probe.replayStore, head)
				if err != nil {
					return probe.scratchErr(err)
				}
				binding.Witness = rebound
				binding.HeldTailEvents = held
				if report.LiveHead != nil && head.Sequence > report.LiveHead.Sequence {
					binding.CaughtUpEvents = head.Sequence - report.LiveHead.Sequence
				}
			}
			if err := rt.pinCheckpointBase(ctx, computerID, report.BaseBlobSHA256, headDigest(head)); err != nil {
				return fmt.Errorf("self-development checkpoint: pin replay base: %w", err)
			}
			if publish == nil {
				return nil
			}
			published, err := publish(ctx, binding)
			if err != nil {
				return err
			}
			response = &published
			return nil
		})
		if err == nil {
			return binding, response, nil
		}
		if !checkpointHeadRefused(err) {
			return checkpointBinding{}, nil, err
		}
		lastErr = err
	}
	return checkpointBinding{}, nil, fmt.Errorf("self-development checkpoint: the platform refused the bound head on all %d attempts (its head moved outside this computer's appends): %w", checkpointBindAttempts, lastErr)
}

// advanceCheckpointReplay replays the tail from the replay store's head to
// the live head, without holding appends. It never compares: the held
// section compares at the head it binds.
func (rt *Runtime) advanceCheckpointReplay(ctx context.Context, computerID string, replayStore *choirstore.Store) error {
	live, err := rt.store.Head(ctx, computerID)
	if err != nil || live == nil {
		// The held section reads the head again and refuses there.
		return nil
	}
	replayed, err := replayStore.Head(ctx, computerID)
	if err != nil {
		return fmt.Errorf("self-development checkpoint: read replay event head: %w", err)
	}
	if headDigest(replayed) == live.CanonicalEventHead {
		return nil
	}
	if err := rt.eventAppender.ReconstructThroughTarget(ctx, replayStore, live.CanonicalEventHead); err != nil {
		return fmt.Errorf("self-development checkpoint: replay tail to sequence %d: %w", live.Sequence, err)
	}
	return nil
}

// rebindCheckpointWitness derives the witness at head. The caller holds
// appends, so the live store is head's state. The replay store advances by
// the tail only; replayInto uses its own dry-run appender, so the hold's
// lock is never taken twice.
func (rt *Runtime) rebindCheckpointWitness(ctx context.Context, computerID string, replayStore *choirstore.Store, head *computerevent.Head) (selfdevprotocol.VMLocalContentWitness, uint64, error) {
	var empty selfdevprotocol.VMLocalContentWitness
	if head == nil || !computerevent.IsSHA256(head.CanonicalEventHead) {
		return empty, 0, fmt.Errorf("self-development checkpoint: live event head unavailable")
	}
	version := replayCompletenessVersion(rt, head)
	live, err := replayDoltStateExtractor(rt.store.TexturePath()).Extract(ctx, computerversion.ExtractRequest{Name: "live-dolt", Version: version})
	if err != nil {
		return empty, 0, fmt.Errorf("self-development checkpoint: extract live Dolt state at sequence %d: %w", head.Sequence, err)
	}
	before, err := replayStore.Head(ctx, computerID)
	if err != nil {
		return empty, 0, fmt.Errorf("self-development checkpoint: read replay event head: %w", err)
	}
	if err := rt.eventAppender.ReconstructThroughTarget(ctx, replayStore, head.CanonicalEventHead); err != nil {
		return empty, 0, fmt.Errorf("self-development checkpoint: replay tail to sequence %d: %w", head.Sequence, err)
	}
	replayHead, err := replayStore.Head(ctx, computerID)
	if err != nil || headDigest(replayHead) != head.CanonicalEventHead {
		return empty, 0, fmt.Errorf("self-development checkpoint: replay did not reach the live head at sequence %d", head.Sequence)
	}
	replay, err := replayDoltStateExtractor(replayStore.TexturePath()).Extract(ctx, computerversion.ExtractRequest{Name: "event-replay-projection", Version: version})
	if err != nil {
		return empty, 0, fmt.Errorf("self-development checkpoint: extract replay Dolt state at sequence %d: %w", head.Sequence, err)
	}
	result := (computerversion.EquivalenceChecker{}).CheckObservationSets(
		filterReplayHeadObservation(live), filterReplayHeadObservation(replay),
	)
	var objectGraph *ReplayObjectGraphComparison
	if !result.Equivalent() {
		liveObjects, liveErr := rt.store.ListObjectGraphFingerprints(ctx)
		replayObjects, replayErr := replayStore.ListObjectGraphFingerprints(ctx)
		if liveErr == nil && replayErr == nil {
			objectGraph = compareReplayObjectGraph(liveObjects, replayObjects)
		}
	}
	if err := checkpointEligibilityError(replayEligibility(head, replayHead, live, replay, result), objectGraph); err != nil {
		return empty, 0, fmt.Errorf("%w (at bound sequence %d)", err, head.Sequence)
	}
	witness, err := selfdevprotocol.WitnessFromObservationSets(live, replay, result)
	if err != nil {
		return empty, 0, err
	}
	var held uint64
	if before != nil && replayHead.Sequence > before.Sequence {
		held = replayHead.Sequence - before.Sequence
	}
	return witness, held, nil
}

func checkpointEligibilityError(eligibility ReplayEligibility, og *ReplayObjectGraphComparison) error {
	err := eligibility.Error()
	if err == nil {
		return nil
	}
	if og != nil {
		return fmt.Errorf("self-development checkpoint: %w (og_objects live_only=%d replay_only=%d changed=%d by_kind=%v)", err, og.LiveOnlyCount, og.ReplayOnlyCount, og.ChangedCount, og.ByObjectKind)
	}
	return fmt.Errorf("self-development checkpoint: %w", err)
}

func checkpointFrontendIdentity(releaseDigest string, files []updater.ManifestFile) (selfdevprotocol.FrontendIdentity, error) {
	releaseFiles := make([]selfdevprotocol.ReleaseFile, 0, len(files))
	for _, file := range files {
		releaseFiles = append(releaseFiles, selfdevprotocol.ReleaseFile{Path: file.Path, SHA256: file.SHA256})
	}
	return selfdevprotocol.FrontendIdentityFromReleaseFiles(releaseDigest, releaseFiles)
}

// checkpointHeadRefused reports a platform refusal of the checkpoint's head:
// the authority's current head is not the request's (its FOR UPDATE
// equality), or the head-scoped idempotency key conflicts.
func checkpointHeadRefused(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "does not match request") || strings.Contains(message, "head CAS conflict")
}

func headDigest(head *computerevent.Head) string {
	if head == nil {
		return ""
	}
	return head.CanonicalEventHead
}
