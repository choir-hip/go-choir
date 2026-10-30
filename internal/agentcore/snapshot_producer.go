package agentcore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
	"github.com/yusefmosiah/go-choir/internal/snapshot"
)

// The snapshot producer (docs/design/choir-data-system-frame-2026-10-30.md
// sections 5 and 6; build sheet decisions 7 and 12). A computer whose disk is
// lost is rebuilt from its newest snapshot plus a tape tail, so every computer
// takes snapshots: one as soon as boot has settled when it has none, then one
// per interval.
//
// Ways it could fail, written before the code:
//
//  1. the capture runs on the write connection or inside an append hold and
//     blocks every write on a 30 GB store for minutes (it streams the pages of
//     one read transaction on the read pool and holds nothing);
//  2. the first snapshot is judged by scanning the tape from genesis over HTTP
//     (the local computer_event_index answers it);
//  3. two snapshots run at once, or one runs during a boot, a maintenance
//     hold, a self-development apply or while an engineering capsule is live;
//  4. the stream writes a temporary copy of the store (it writes nothing: the
//     pages go from the read snapshot to sealed chunks), or a leftover copy
//     from the VACUUM INTO era is never removed (it is, at the start of an
//     attempt);
//  5. a long read transaction pins the WAL and the volume fills with WAL while
//     it runs (the producer refuses under a free-space floor and the stream
//     has a time budget, CHOIR_SNAPSHOT_MAX_DURATION, after which it aborts
//     and says why);
//  6. a snapshot that crashes the computer is retried at every boot (a crash
//     loop: the attempt guard closes it after snapshotAttemptLimit starts);
//  7. a refusal is read as success, or the producer waits forever on one
//     (every refusal is logged with its reason and retried at the next tick).
//
// There is no size cap: the stream needs no local room and the host stores
// only the chunks it does not already hold. The producer ships ON for every
// computer. A free-space or busy refusal is a correctness refusal, not an off
// switch; CHOIR_SNAPSHOT_DISABLE=1 is the emergency override only.
const (
	envSnapshotInterval    = "CHOIR_SNAPSHOT_INTERVAL"
	envSnapshotMaxDuration = "CHOIR_SNAPSHOT_MAX_DURATION"
	envSnapshotDisable     = "CHOIR_SNAPSHOT_DISABLE"

	snapshotDefaultInterval = 24 * time.Hour
	// The read transaction of the stream pins the WAL; past this the stream
	// aborts with a logged reason and the next tick tries again.
	snapshotDefaultMaxDuration = 30 * time.Minute
	// The stream writes no copy, but the WAL grows while it reads. Below this
	// much free space on the data volume the producer waits.
	snapshotFreeFloor = uint64(2) << 30

	snapshotInitialDelay = 3 * time.Minute
	snapshotTickEvery    = 10 * time.Minute

	snapshotAttemptKey   = "snapshot_producer"
	snapshotAttemptLimit = 3
	// snapshotLegacyTempDirName held the VACUUM INTO copy before the page
	// stream; an attempt removes what a crashed copy left there.
	snapshotLegacyTempDirName = "snapshot-tmp"
)

type snapshotOutcome string

const (
	snapshotTaken   snapshotOutcome = "taken"
	snapshotNotDue  snapshotOutcome = "not_due"
	snapshotRefused snapshotOutcome = "refused"
	snapshotFailed  snapshotOutcome = "failed"
	snapshotSkipped snapshotOutcome = "skipped"
)

// snapshotSettings are the producer's tunables, read from the environment.
type snapshotSettings struct {
	Interval    time.Duration
	MaxDuration time.Duration
	Disabled    bool
}

func snapshotSettingsFromEnv(getenv func(string) string) snapshotSettings {
	s := snapshotSettings{Interval: snapshotDefaultInterval, MaxDuration: snapshotDefaultMaxDuration}
	if raw := strings.TrimSpace(getenv(envSnapshotInterval)); raw != "" {
		if d, err := time.ParseDuration(raw); err == nil && d > 0 {
			s.Interval = d
		} else {
			log.Printf("runtime: %s=%q is not a positive duration; using %s", envSnapshotInterval, raw, snapshotDefaultInterval)
		}
	}
	if raw := strings.TrimSpace(getenv(envSnapshotMaxDuration)); raw != "" {
		if d, err := time.ParseDuration(raw); err == nil && d > 0 {
			s.MaxDuration = d
		} else {
			log.Printf("runtime: %s=%q is not a positive duration; using %s", envSnapshotMaxDuration, raw, snapshotDefaultMaxDuration)
		}
	}
	switch strings.ToLower(strings.TrimSpace(getenv(envSnapshotDisable))) {
	case "1", "true", "on", "yes":
		s.Disabled = true
	}
	return s
}

// snapshotDeps is what one producer tick reads and does. The runtime fills it
// from its store, appender and platform client; tests substitute seams.
type snapshotDeps struct {
	ComputerID string
	StorePath  string
	// DataDir is the data volume directory beside the store.
	DataDir string
	// Newest is the newest snapshot_recorded on the LOCAL event index.
	Newest func(ctx context.Context) (sequence uint64, at time.Time, found bool, err error)
	// HeadSequence is the store's event head sequence (false for an empty
	// store).
	HeadSequence func(ctx context.Context) (uint64, bool, error)
	// Busy returns a reason when work a snapshot must not run beside is live.
	Busy func(ctx context.Context) (string, error)
	// FreeBytes is the free space on the volume holding dir (the WAL grows
	// there while the stream's read transaction is open).
	FreeBytes func(dir string) (uint64, error)
	// Ready reports whether boot has settled and the runtime is not held.
	Ready func() bool
	// Take takes the snapshot.
	Take func(ctx context.Context, maxDuration time.Duration) (snapshot.Ref, error)
}

type snapshotProducer struct {
	tick sync.Mutex // one snapshot at a time

	mu       sync.Mutex
	stop     chan struct{}
	lastAt   time.Time
	lastSeq  uint64
	lastKnow bool
	lastWhy  string
}

// run is one evaluation: take a snapshot if one is due and nothing refuses.
func (p *snapshotProducer) run(ctx context.Context, d snapshotDeps, set snapshotSettings, now time.Time) (snapshotOutcome, string) {
	if set.Disabled {
		return snapshotSkipped, envSnapshotDisable + " is set"
	}
	if !p.tick.TryLock() {
		return snapshotSkipped, "a snapshot is already running"
	}
	defer p.tick.Unlock()
	if d.Ready != nil && !d.Ready() {
		return snapshotSkipped, "boot has not settled or the computer is held"
	}

	p.mu.Lock()
	known := p.lastKnow
	p.mu.Unlock()
	if !known {
		seq, at, found, err := d.Newest(ctx)
		if err != nil {
			return snapshotFailed, "read the local snapshot index: " + err.Error()
		}
		p.mu.Lock()
		p.lastKnow = true
		if found {
			p.lastSeq, p.lastAt = seq, at
		}
		p.mu.Unlock()
	}
	p.mu.Lock()
	lastSeq, lastAt := p.lastSeq, p.lastAt
	p.mu.Unlock()

	head, hasHead, err := d.HeadSequence(ctx)
	if err != nil {
		return snapshotFailed, "read the store head: " + err.Error()
	}
	if !hasHead {
		return snapshotNotDue, "the store holds no events"
	}
	reason := "first"
	if !lastAt.IsZero() {
		if head <= lastSeq+1 {
			return snapshotNotDue, "no event since the newest snapshot"
		}
		if now.Sub(lastAt) < set.Interval {
			return snapshotNotDue, ""
		}
		reason = "interval"
	}

	// Correctness refusals: logged by the caller, retried at the next tick.
	if why, err := d.Busy(ctx); err != nil {
		return snapshotRefused, "cannot tell whether work is live (fails closed): " + err.Error()
	} else if why != "" {
		return snapshotRefused, why
	}
	if err := os.MkdirAll(d.DataDir, 0o700); err != nil {
		return snapshotFailed, "create the data dir: " + err.Error()
	}
	free, err := d.FreeBytes(d.DataDir)
	if err != nil {
		return snapshotRefused, "cannot measure free space (fails closed): " + err.Error()
	}
	if free < snapshotFreeFloor {
		return snapshotRefused, fmt.Sprintf("free space %d bytes on the data volume is under the %d-byte floor the WAL needs while the stream reads", free, snapshotFreeFloor)
	}

	// A snapshot that crashes the computer is not retried forever.
	if d.StorePath != "" {
		if err := spendGuestAttempt(d.StorePath, snapshotAttemptKey, d.ComputerID, snapshotAttemptLimit); err != nil {
			return snapshotRefused, "closed after crashing the computer: " + err.Error()
		}
	}
	// The VACUUM INTO copy of an earlier version, if a crash left one.
	_ = os.RemoveAll(filepath.Join(d.DataDir, snapshotLegacyTempDirName))
	started := time.Now()
	ref, err := d.Take(ctx, set.MaxDuration)
	if d.StorePath != "" {
		_ = clearGuestAttempt(d.StorePath, snapshotAttemptKey, d.ComputerID)
	}
	if err != nil {
		if errors.Is(err, snapshot.ErrTimeBudget) {
			return snapshotFailed, fmt.Sprintf("aborted at the %s time budget (%s) with nothing recorded: %s", set.MaxDuration, envSnapshotMaxDuration, err.Error())
		}
		return snapshotFailed, err.Error()
	}
	p.mu.Lock()
	p.lastAt, p.lastSeq = now, ref.Record.StoreHeadSequence
	p.mu.Unlock()
	st := ref.Stats
	return snapshotTaken, fmt.Sprintf("%s snapshot at head %d: %d bytes (%d pages of %d) in %s, event %s; %d chunks, %d uploaded (%d bytes sealed), %d already held, %d repeated, %d probe errors; stream %s (%.0f pages/s), witness %s, read transaction %s",
		reason, ref.Record.StoreHeadSequence, ref.Record.PlaintextSize, st.Pages, st.PageSize, time.Since(started).Round(time.Millisecond), ref.EventID,
		st.Chunks, st.ChunksUploaded, st.BytesUploaded, st.ChunksHeld, st.ChunksRepeated, st.ProbeErrors,
		st.StreamTime.Round(time.Millisecond), pagesPerSecond(st), st.WitnessTime.Round(time.Millisecond), st.ReadTxTime.Round(time.Millisecond))
}

func pagesPerSecond(st snapshot.Stats) float64 {
	if st.StreamTime <= 0 {
		return 0
	}
	return float64(st.Pages) / st.StreamTime.Seconds()
}

// snapshotDepsForRuntime binds the producer to this runtime. It returns false
// when the runtime lacks what a snapshot needs (no store, appender, key or
// platform connection): a local or test runtime, not a production guest.
func (rt *Runtime) snapshotDepsForRuntime() (snapshotDeps, bool) {
	if rt == nil || rt.store == nil || rt.eventAppender == nil || rt.privateArtifactCipher == nil {
		return snapshotDeps{}, false
	}
	computerID := strings.TrimSpace(rt.cfg.ComputerID)
	client, ok := rt.eventPayloadReader.(*computerevent.HTTPClient)
	if computerID == "" || !ok || client == nil {
		return snapshotDeps{}, false
	}
	chunks := snapshot.HTTPChunks{BaseURL: client.BaseURL(), ComputerID: computerID, Capability: client.Capability()}
	store := rt.store
	return snapshotDeps{
		ComputerID: computerID,
		StorePath:  rt.cfg.StorePath,
		DataDir:    filepath.Dir(store.StatePath()),
		Newest: func(ctx context.Context) (uint64, time.Time, bool, error) {
			return store.NewestEventOfKind(ctx, computerID, computerevent.EventSnapshotRecorded)
		},
		HeadSequence: func(ctx context.Context) (uint64, bool, error) {
			head, err := store.Head(ctx, computerID)
			if err != nil || head == nil {
				return 0, false, err
			}
			return head.Sequence, true, nil
		},
		Busy: func(ctx context.Context) (string, error) {
			work, err := rt.plannedRestartBusy(ctx, true)
			if err != nil {
				return "", err
			}
			if work.SelfdevActive > 0 {
				return fmt.Sprintf("%d self-development operation(s) in flight", work.SelfdevActive), nil
			}
			if work.LiveCapsules > 0 {
				return fmt.Sprintf("%d engineering capsule(s) live", work.LiveCapsules), nil
			}
			return "", nil
		},
		FreeBytes: freeBytesOnVolume,
		Ready: func() bool {
			return !rt.bootInProgress.Load() && !rt.maintenanceHeld()
		},
		Take: func(ctx context.Context, maxDuration time.Duration) (snapshot.Ref, error) {
			return snapshot.TakeSnapshot(ctx, snapshot.Config{
				ComputerID: computerID,
				DB:         store.ReadDB(),
				// Both run inside the stream's one read transaction: the head
				// is its first statement, so head, pages and witness are one
				// state.
				Head: func(ctx context.Context, tx *sql.Tx) (*computerevent.Head, error) {
					return store.HeadTx(ctx, tx, computerID)
				},
				Witness: func(ctx context.Context, tx *sql.Tx) (selfdevprotocol.VMLocalContentWitness, error) {
					return LiveStoreWitnessTx(ctx, tx)
				},
				Cipher:      rt.privateArtifactCipher,
				Chunks:      chunks,
				Appender:    rt.eventAppender,
				MaxDuration: maxDuration,
			})
		},
	}, true
}

// snapshotProducerTick is one tick on the runtime, with its logging.
func (rt *Runtime) snapshotProducerTick(ctx context.Context, now time.Time) {
	deps, ok := rt.snapshotDepsForRuntime()
	if !ok {
		return
	}
	p := &rt.snapshots
	outcome, detail := p.run(ctx, deps, snapshotSettingsFromEnv(os.Getenv), now)
	switch outcome {
	case snapshotTaken:
		log.Printf("runtime: snapshot taken: %s", detail)
	case snapshotRefused, snapshotFailed:
		// A refusal repeats at every tick until it clears; say it when it
		// changes so the log stays readable.
		key := string(outcome) + ": " + detail
		p.mu.Lock()
		changed := p.lastWhy != key
		p.lastWhy = key
		p.mu.Unlock()
		if changed {
			log.Printf("runtime: snapshot %s (retry in %s): %s", outcome, snapshotTickEvery, detail)
		}
	default:
		p.mu.Lock()
		p.lastWhy = ""
		p.mu.Unlock()
	}
}

// startSnapshotProducer starts the standing intent: after the boot settles,
// and then at every tick, take a snapshot if one is due. Like the checkpoint
// cadence it binds nothing at boot: a restart resumes no work, the next tick
// is new work.
func (rt *Runtime) startSnapshotProducer(ctx context.Context) {
	if rt == nil {
		return
	}
	p := &rt.snapshots
	p.mu.Lock()
	if p.stop != nil {
		p.mu.Unlock()
		return
	}
	stop := make(chan struct{})
	p.stop = stop
	p.mu.Unlock()
	rt.wg.Add(1)
	go func() {
		defer rt.wg.Done()
		timer := time.NewTimer(snapshotInitialDelay)
		defer timer.Stop()
		select {
		case <-stop:
			return
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		ticker := time.NewTicker(snapshotTickEvery)
		defer ticker.Stop()
		for {
			rt.snapshotProducerTick(ctx, time.Now().UTC())
			select {
			case <-stop:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func (rt *Runtime) stopSnapshotProducer() {
	if rt == nil {
		return
	}
	p := &rt.snapshots
	p.mu.Lock()
	stop := p.stop
	p.stop = nil
	p.mu.Unlock()
	if stop != nil {
		close(stop)
	}
}
