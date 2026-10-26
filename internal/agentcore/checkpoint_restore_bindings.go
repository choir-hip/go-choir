package agentcore

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/projectionbase"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
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
}

// checkpointPublishFunc publishes a checkpoint for a binding. It runs while
// appends are held: it must build its request from binding.Head (never a
// fresh head read) and must not append an event.
type checkpointPublishFunc func(ctx context.Context, binding checkpointBinding) (selfdevprotocol.CheckpointResponse, error)

// bindCheckpointAtHead binds a checkpoint's head and witness to one state and
// publishes it. Holding appends, it reads the head H, extracts the live
// witness at H, pins the advertised projection base under H and publishes, so
// the platform's current head is H and the witness is H's. It installs no
// base, replays no tape and writes no scratch or Dolt row: whether the
// witnessed state is derivable from the tape is proven where a checkpoint is
// used, by restore, which reconstructs to H and refuses a witness mismatch
// (docs/design/apply-checkpoint-without-replay-2026-10-26.md, 4.1). The
// publish stays inside the hold: the authority accepts a checkpoint only at
// its current head (checkpoints.go Publish), and a head read before the hold
// is not the head after it on a computer that is working. publish nil
// returns the binding without publishing.
func (rt *Runtime) bindCheckpointAtHead(ctx context.Context, computerID, releaseDigest string, files []updater.ManifestFile, publish checkpointPublishFunc) (checkpointBinding, *selfdevprotocol.CheckpointResponse, error) {
	if rt == nil || rt.store == nil || rt.eventAppender == nil {
		return checkpointBinding{}, nil, fmt.Errorf("%w: event projection authority is not configured", ErrReplayCompletenessUnavailable)
	}
	computerID = strings.TrimSpace(computerID)
	if computerID == "" || computerID != strings.TrimSpace(rt.cfg.ComputerID) {
		return checkpointBinding{}, nil, fmt.Errorf("%w: computer binding does not match runtime", ErrReplayCompletenessUnavailable)
	}
	if err := ctx.Err(); err != nil {
		return checkpointBinding{}, nil, err
	}
	frontend, err := checkpointFrontendIdentity(releaseDigest, files)
	if err != nil {
		return checkpointBinding{}, nil, err
	}

	var lastErr error
	for attempt := 1; attempt <= checkpointBindAttempts; attempt++ {
		var (
			binding  checkpointBinding
			response *selfdevprotocol.CheckpointResponse
		)
		err := rt.eventAppender.HoldAppends(func() error {
			head, err := rt.store.Head(ctx, computerID)
			if err != nil {
				return fmt.Errorf("self-development checkpoint: read live event head: %w", err)
			}
			witness, err := rt.liveCheckpointWitness(ctx, head)
			if err != nil {
				return err
			}
			binding = checkpointBinding{Head: head, Witness: witness, Frontend: frontend}
			if err := rt.pinAdvertisedBase(ctx, computerID, head); err != nil {
				return fmt.Errorf("self-development checkpoint: pin advertised base: %w", err)
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

// liveCheckpointWitness is the VM-local content witness of the live store at
// head. The caller holds appends, so the live store is head's state.
func (rt *Runtime) liveCheckpointWitness(ctx context.Context, head *computerevent.Head) (selfdevprotocol.VMLocalContentWitness, error) {
	var empty selfdevprotocol.VMLocalContentWitness
	if head == nil || !computerevent.IsSHA256(head.CanonicalEventHead) {
		return empty, fmt.Errorf("self-development checkpoint: live event head unavailable")
	}
	live, err := replayDoltStateExtractor(rt.store.TexturePath()).Extract(ctx, computerversion.ExtractRequest{Name: "live-dolt", Version: replayCompletenessVersion(rt, head)})
	if err != nil {
		return empty, fmt.Errorf("self-development checkpoint: extract live Dolt state at sequence %d: %w", head.Sequence, err)
	}
	return selfdevprotocol.WitnessFromObservationSet(live)
}

// pinAdvertisedBase retains the platform's advertised projection base under
// the checkpoint's head, so a restore to the checkpoint starts from a snapshot
// at or below it even after the advertised base moves past it. A computer the
// platform never published a base for has nothing to pin (restore replays its
// bounded chain from genesis); a source that cannot answer is not that case
// and refuses, so no checkpoint is published without the pin it needs.
func (rt *Runtime) pinAdvertisedBase(ctx context.Context, computerID string, head *computerevent.Head) error {
	if head == nil || head.Sequence == 0 {
		return nil
	}
	src, err := rt.resolveRestoreBaseSource()
	if err != nil {
		return err
	}
	sequence, baseRef, err := src.Watermark(ctx, computerID)
	if err != nil {
		if errors.Is(err, projectionbase.ErrBaseRefused) {
			return nil
		}
		return err
	}
	if sequence == 0 || strings.TrimSpace(baseRef) == "" {
		return nil
	}
	return rt.pinCheckpointBase(ctx, computerID, baseRef, head.CanonicalEventHead)
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
