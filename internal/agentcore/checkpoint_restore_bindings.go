package agentcore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
	"github.com/yusefmosiah/go-choir/internal/updater"
)

// checkpointBindAttempts bounds how often a checkpoint re-binds after the
// platform refuses the head it named or the head moved under a scan. The last
// attempt pins appends for its whole bind, so a computer that keeps appending
// still converges; a head that moves under a pinned bind fails loudly.
const checkpointBindAttempts = 3

// errCheckpointHeadMoved reports that an unpinned bind's head is no longer the
// live head after its scan, so publishing it would be refused by the platform.
var errCheckpointHeadMoved = errors.New("self-development checkpoint: the live head moved during the witness scan")

// checkpointBinding is what one checkpoint publish binds: the head and the
// VM-local witness of the same projection state
// (docs/problems/selfdev-go-drill-jobs-0101-2026-10-15.md, item 1).
type checkpointBinding struct {
	// Head is the live head the witness was taken at: read as the first read
	// of the same read transaction that scanned the witness. The publish names
	// it; nothing re-reads the head.
	Head     *computerevent.Head
	Witness  selfdevprotocol.VMLocalContentWitness
	Frontend selfdevprotocol.FrontendIdentity
	// DepositMode is the live store's deposit mode at the snapshot, published
	// with the checkpoint so the host's verdict can name residual R1.
	DepositMode string
	// WitnessDuration is how long the witness scan took.
	WitnessDuration time.Duration
}

// checkpointPublishFunc publishes a checkpoint for a binding. It must build
// its request from binding.Head (never a fresh head read) and must not append
// an event. It runs outside the append hold unless the bind is pinned (the
// last attempt), so it may take as long as a network call.
type checkpointPublishFunc func(ctx context.Context, binding checkpointBinding) (selfdevprotocol.CheckpointResponse, error)

// bindCheckpointAtHead binds a checkpoint's head and witness to one state and
// publishes it: the head H and the live witness are read from one read
// snapshot (bindCheckpoint), so the witness is H's state and the checkpoint
// names H. It installs no snapshot, replays no tape and writes no
// scratch or store row: whether the
// witnessed state is derivable from the tape is proven where a checkpoint is
// used, by restore, which reconstructs to H and refuses a witness mismatch
// (docs/design/apply-checkpoint-without-replay-2026-10-26.md, 4.1). The
// authority accepts a checkpoint only at its current head (checkpoints.go
// Publish): a head that moved during the scan is refused there, and the bind
// retries. publish nil returns the binding without publishing.
func (rt *Runtime) bindCheckpointAtHead(ctx context.Context, computerID, releaseDigest string, files []updater.ManifestFile, publish checkpointPublishFunc) (checkpointBinding, *selfdevprotocol.CheckpointResponse, error) {
	return rt.bindCheckpoint(ctx, computerID, releaseDigest, files, publish)
}

// bindCheckpoint binds the head and the witness of one state and publishes.
// A restore to a checkpoint starts from the newest snapshot at or below its
// head (internal/snapshot) and replays the tape after it; the checkpoint pins
// nothing, because the snapshot is named on the tape.
//
// The witness scan reads a whole store (minutes on a large one) and the
// publish is a network call; appends wait for neither. An attempt holds
// appends only long enough to open a read transaction on the read pool and
// read the projection head as its first statement: with no append in flight
// that head names exactly the state the transaction's WAL snapshot holds
// (event index, projection rows and head commit together under the append
// mutex), and every later read of the transaction, the scan, is of that
// state while appends and writes proceed on the write connection. After the
// scan the bind checks the live head is still the bound one and publishes
// outside any hold. If the head moved, the platform would refuse it, so the
// next attempt pins appends for its whole bind (the old behaviour, now on the
// read pool); the last attempt is always pinned.
func (rt *Runtime) bindCheckpoint(ctx context.Context, computerID, releaseDigest string, files []updater.ManifestFile, publish checkpointPublishFunc) (checkpointBinding, *selfdevprotocol.CheckpointResponse, error) {
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
	pinned := false
	for attempt := 1; attempt <= checkpointBindAttempts; attempt++ {
		if attempt == checkpointBindAttempts {
			pinned = true
		}
		binding, response, err := rt.bindCheckpointOnce(ctx, computerID, frontend, publish, pinned)
		if err == nil {
			return binding, response, nil
		}
		switch {
		case errors.Is(err, errCheckpointHeadMoved):
			// A computer that moved its head under one scan will move it under
			// the next: pin the next attempt.
			pinned = true
		case checkpointHeadRefused(err):
		default:
			return checkpointBinding{}, nil, err
		}
		lastErr = err
	}
	return checkpointBinding{}, nil, fmt.Errorf("self-development checkpoint: the platform refused the bound head on all %d attempts (its head moved outside this computer's appends): %w", checkpointBindAttempts, lastErr)
}

// bindCheckpointOnce is one bind attempt. pinned holds appends for the whole
// attempt, publish included; unpinned holds them only to open the snapshot.
func (rt *Runtime) bindCheckpointOnce(ctx context.Context, computerID string, frontend selfdevprotocol.FrontendIdentity, publish checkpointPublishFunc, pinned bool) (checkpointBinding, *selfdevprotocol.CheckpointResponse, error) {
	var (
		binding  checkpointBinding
		response *selfdevprotocol.CheckpointResponse
	)
	run := func() error {
		head, depositMode, witness, duration, err := rt.liveCheckpointWitness(ctx, computerID, pinned)
		if err != nil {
			return err
		}
		binding = checkpointBinding{Head: head, Witness: witness, Frontend: frontend, DepositMode: depositMode, WitnessDuration: duration}
		if publish == nil {
			return nil
		}
		if !pinned {
			// Appends ran during the scan. If the head moved, the platform
			// refuses this one: skip the doomed call and re-bind.
			now, err := rt.store.Head(ctx, computerID)
			if err != nil {
				return fmt.Errorf("self-development checkpoint: re-read live event head: %w", err)
			}
			if now == nil || now.CanonicalEventHead != head.CanonicalEventHead {
				return errCheckpointHeadMoved
			}
		}
		published, err := publish(ctx, binding)
		if err != nil {
			return err
		}
		response = &published
		return nil
	}
	var err error
	if pinned {
		err = rt.eventAppender.HoldAppends(run)
	} else {
		err = run()
	}
	if err != nil {
		return checkpointBinding{}, nil, err
	}
	return binding, response, nil
}

// liveCheckpointWitness reads, in one read transaction on the store's read
// pool, the live head and then the v3 witness of the state that head names
// (LiveStoreWitness's extractor, byte for byte). The head read is the
// transaction's first statement and runs under the append hold unless the
// caller already holds appends (pinned); the scan after it holds nothing. The
// write connection is never used, so writes are not queued behind the scan.
func (rt *Runtime) liveCheckpointWitness(ctx context.Context, computerID string, pinned bool) (*computerevent.Head, string, selfdevprotocol.VMLocalContentWitness, time.Duration, error) {
	var empty selfdevprotocol.VMLocalContentWitness
	db := rt.store.ReadDB()
	if db == nil {
		return nil, "", empty, 0, fmt.Errorf("self-development checkpoint: live store is not open")
	}
	var (
		head        *computerevent.Head
		depositMode string
		scanStart   time.Time
	)
	extractor := replayStateExtractor(db)
	extractor.SnapshotHook = func(ctx context.Context, tx *sql.Tx) error {
		read := func() error {
			var err error
			if head, err = rt.store.HeadTx(ctx, tx, computerID); err != nil {
				return fmt.Errorf("self-development checkpoint: read live event head: %w", err)
			}
			if head == nil || !computerevent.IsSHA256(head.CanonicalEventHead) {
				return fmt.Errorf("self-development checkpoint: live event head unavailable")
			}
			if depositMode, err = depositModeOf(rt.store); err != nil {
				return fmt.Errorf("self-development checkpoint: %w", err)
			}
			return nil
		}
		var err error
		if pinned {
			err = read()
		} else {
			err = rt.eventAppender.HoldAppends(read)
		}
		if err != nil {
			return err
		}
		scanStart = time.Now()
		if rt.checkpointScanHook != nil {
			rt.checkpointScanHook(pinned)
		}
		return nil
	}
	witness, err := storeWitness(ctx, db, extractor)
	if err != nil {
		if head != nil {
			return nil, "", empty, 0, fmt.Errorf("self-development checkpoint: read live store witness at sequence %d: %w", head.Sequence, err)
		}
		return nil, "", empty, 0, err
	}
	return head, depositMode, witness, time.Since(scanStart), nil
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
