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
	"sync/atomic"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/recoveryplan"
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

	// A snapshot bounds the replay a rebuild needs; it is due by tape
	// distance (snapshotDefaultTailEvents), and the interval is only a weekly
	// backstop for a quiet computer. Daily snapshots of the paper re-uploaded
	// about 20 GB each: a fixed page-range cut shares few chunks across a
	// day of writes (owner, 2026-10-31: "we need to reconsider how we do
	// compaction if our current plan has us rewriting gbs every day").
	snapshotDefaultInterval = 7 * 24 * time.Hour
	// Three quarters of the recovery bound: a tail reaches the bound only if
	// the snapshot due here fails for a third as long again (it retries every
	// tick). Not half: each of the paper's snapshots writes about 25 GB of new
	// chunks to the host, and at half it would take two a day.
	snapshotDefaultTailEvents = recoveryplan.MaxRecoveryTailEvents / 4 * 3
	// The read transaction pins the WAL; past this the attempt aborts with a
	// logged reason and the next attempt waits out its backoff. The witness
	// alone measured 22 to 35 minutes on the paper's 21 GiB store (2026-11-12),
	// so 30 minutes could never finish; 90 minutes holds the witness, the
	// 3.5 minute stream and the partitions. The cost is the WAL pin: it grew
	// to 3.7 GB in 30 minutes under load, so it may reach about 11 GB in 90;
	// the free-space floor and CheckpointAfterLongRead bound the damage. This
	// is a deadline, not a switch.
	snapshotDefaultMaxDuration = 90 * time.Minute
	// After a failed attempt the next one waits snapshotBackoffFloor, doubling
	// per consecutive failure up to snapshotBackoffCeiling; a success resets
	// it. Every attempt uploads most of a live store's chunks, which the host
	// keeps until the grace passes, so a failure must not be retried at the
	// tick (6 GiB an hour on the paper, 2026-11-12). At 6 events/s the
	// ceiling adds about 65,000 events to the tail, inside the margin between
	// the trigger and the recovery bound. A backoff after our own failure, not
	// a call limit.
	snapshotBackoffFloor   = 2 * snapshotTickEvery
	snapshotBackoffCeiling = 3 * time.Hour
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
	// snapshotBackoff: due, but waiting out the backoff after a failure.
	snapshotBackoff snapshotOutcome = "backoff"
	// snapshotCancelled: a planned restart or a hibernate cancelled the
	// capture. Not a failure: nothing was recorded, the backoff is untouched
	// and the crash-attempt guard is cleared, as for any attempt that returns.
	snapshotCancelled snapshotOutcome = "cancelled"
)

var (
	errSnapshotCancelledForRestart   = errors.New("snapshot capture cancelled for a planned restart")
	errSnapshotCancelledForHibernate = errors.New("snapshot capture cancelled for a hibernate")
)

// snapshotCancelUnwindBound is how long a planned restart or a hibernate waits
// for a cancelled capture to unwind (the read transaction rolled back, the
// WAL released, the upload workers stopped, the attempt guard cleared).
//
// The arithmetic. The host gives the planned-restart request its caller's
// budget: 20 s on the deploy refresh (httpGuestMarker's client) and 10 s on
// the operator and owner restart (restartMarkTimeout, owner_update.go; the
// same context cancels the request), and a timeout there is a markerless,
// crash-booting refresh for the operator's verb. After this wait the handler
// still drains queued telemetry (milliseconds; its own deadline is 10 s,
// telemetry_batching.go), acks paused capsules, writes and fsyncs the marker
// and syncs the filesystems. So this wait is the one cost this change adds,
// and it is set to 3 s: a real witness scan cancelled in 50 ms in review, and
// 3 s leaves 7 s of the 10 s budget for the rest, which is the normal case
// (milliseconds). It does NOT guarantee the 10 s budget when the telemetry
// drain runs to its own 10 s deadline; that was true before this change, and
// the host-side answer is in deploy_refresh.go (it clears a marker the guest
// wrote after the host gave up). A capture that does not end in the bound
// refuses the restart (snapshot_not_unwound); the host asks again. A
// deadline, not a switch. A var so tests can shorten it.
//
// The host's side of a give-up: it asks the guest to clear a marker it may
// have written late, for up to guestMarkClearTimeout (5 s). So on the
// operator's restart the longest the verb can take on a wedged guest is
// 3 s (probe) + 10 s (mark) + 5 s (clear) = 18 s, and on the deploy path
// 20 s + 5 s = 25 s. A named residual: the telemetry drain's own 10 s
// deadline, run after this wait, still exceeds the operator's 10 s budget
// when it is spent; that was true before this change and is not made
// better here. The guest does not write a marker for a caller that has gone
// (errPlannedRestartCallerGone) and a withdrawal waits behind a mark only as
// long as its own request does.
var snapshotCancelUnwindBound = 3 * time.Second

// snapshotRestartBlock is how long, after a restart cancelled the capture, the
// producer starts no new one: a capture started between the cancel and the
// host's kill would be cut for nothing and would leave its crash-attempt
// guard spent. The restart being withdrawn (DELETE) lifts it at once; the
// expiry covers a host that never answers.
const snapshotRestartBlock = 5 * time.Minute

// snapshotAttempt is one attempt's cancel handle, guarded by the producer's mu.
type snapshotAttempt struct {
	cancel     context.CancelCauseFunc
	cause      error
	cancelled  bool
	committing bool
	// done closes when the attempt has fully ended: the capture returned, the
	// in-flight flag is down and the attempt guard is settled.
	done chan struct{}
}

// snapshotNotUnwoundError is the refusal when a capture did not end inside the
// bound. committing says it was past its point of no return.
type snapshotNotUnwoundError struct {
	committing bool
	bound      time.Duration
}

func (e *snapshotNotUnwoundError) Error() string {
	if e.committing {
		return fmt.Sprintf("a snapshot is recording its root and did not finish within %s; it cannot be cancelled without leaving a root the tape does not know", e.bound)
	}
	return fmt.Sprintf("a snapshot capture was cancelled but did not unwind within %s", e.bound)
}

type snapshotCommitGateKey struct{}

// snapshotCommitGate is the capture's point of no return, carried on the
// context given to Take (snapshot.Config.BeforeCommit). A context without one
// (a test seam) always says yes.
func snapshotCommitGate(ctx context.Context) func() error {
	if gate, ok := ctx.Value(snapshotCommitGateKey{}).(func() error); ok {
		return gate
	}
	return func() error { return nil }
}

// snapshotSettings are the producer's tunables, read from the environment.
type snapshotSettings struct {
	Interval    time.Duration
	MaxDuration time.Duration
	Disabled    bool
	// TailEvents fires a snapshot before the interval once the tail since the
	// newest one reaches it, so a busy computer's tail stays inside the
	// recovery bound between snapshots. Zero: the interval alone.
	TailEvents uint64
}

func snapshotSettingsFromEnv(getenv func(string) string) snapshotSettings {
	s := snapshotSettings{Interval: snapshotDefaultInterval, MaxDuration: snapshotDefaultMaxDuration, TailEvents: snapshotDefaultTailEvents}
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
	// Now is the clock (nil: time.Now). The backoff counts from the time an
	// attempt ENDS, which a 90 minute attempt makes far later than `now`.
	Now func() time.Time
	// Take takes the snapshot.
	Take func(ctx context.Context, maxDuration time.Duration) (snapshot.Ref, error)
}

type snapshotProducer struct {
	tick sync.Mutex // one snapshot at a time
	// inFlight is true while a snapshot streams. A planned restart or a
	// hibernate cancels the capture (cancelInFlight) and waits for it to
	// unwind; nothing is recorded until the capture ends, so the cost is the
	// time spent. A restart asked for in the instant before a stream starts
	// still cuts it: that costs one attempt, never state.
	inFlight atomic.Bool

	// cur is the running attempt; blockUntil holds new attempts back after a
	// restart cancelled one (snapshotRestartBlock).
	cur        *snapshotAttempt
	blockUntil time.Time
	// blockGen names the hold that blockUntil belongs to, so a request whose
	// wait failed lifts only its own hold, not a later request's.
	blockGen uint64

	mu       sync.Mutex
	stop     chan struct{}
	lastAt   time.Time
	lastSeq  uint64
	lastKnow bool
	lastWhy  string

	// failures counts consecutive failed attempts; failedHead is the store
	// head the last one began at; retryAt is when the next may begin.
	failures   int
	failedHead uint64
	retryAt    time.Time
}

// snapshotBackoffWait is the wait after the nth consecutive failure (n >= 1).
func snapshotBackoffWait(n int) time.Duration {
	wait := snapshotBackoffFloor
	for i := 1; i < n && wait < snapshotBackoffCeiling; i++ {
		wait *= 2
	}
	return min(wait, snapshotBackoffCeiling)
}

// begin registers a new attempt unless a cancelled restart holds the
// producer back.
//
// mayStart is asked under p.mu, after the cheap checks in run. It makes the
// hibernate fence atomic with the start of a capture: hibernate.close sets the
// fence before prepareHibernate takes p.mu in cancelInFlight, so either begin
// ran first (cur is set and the cancel finds it) or begin runs second and sees
// the closed fence here. The Ready check at the top of run alone is not enough:
// the queries between it and begin take tens of milliseconds.
func (p *snapshotProducer) begin(ctx context.Context, mayStart func() bool) (*snapshotAttempt, context.Context, bool) {
	capCtx, cancel := context.WithCancelCause(ctx)
	cp := &snapshotAttempt{cancel: cancel, done: make(chan struct{})}
	p.mu.Lock()
	defer p.mu.Unlock()
	if time.Now().Before(p.blockUntil) || (mayStart != nil && !mayStart()) {
		cancel(nil)
		return nil, nil, false
	}
	p.cur = cp
	gate := func() error {
		p.mu.Lock()
		defer p.mu.Unlock()
		if cp.cancelled {
			return cp.cause
		}
		cp.committing = true
		return nil
	}
	return cp, context.WithValue(capCtx, snapshotCommitGateKey{}, gate), true
}

// end settles an attempt: it is no longer current and cancelInFlight's wait
// is released.
func (p *snapshotProducer) end(cp *snapshotAttempt) (cancelled bool, cause error) {
	cp.cancel(nil)
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cur == cp {
		p.cur = nil
	}
	close(cp.done)
	return cp.cancelled, cp.cause
}

// cancelInFlight cancels the running capture with cause and waits up to bound
// for it to end. A capture past its point of no return (it is registering its
// root and appending the event) is not cancelled: the wait is for it to
// finish. blockFor, when positive, holds new attempts back for that long
// (cleared again if the wait fails, since the caller then refuses). It
// returns nil when no capture is left running.
func (p *snapshotProducer) cancelInFlight(ctx context.Context, cause error, bound, blockFor time.Duration) error {
	p.mu.Lock()
	var gen uint64
	if blockFor > 0 {
		p.blockGen++
		gen = p.blockGen
		p.blockUntil = time.Now().Add(blockFor)
	}
	cp := p.cur
	if cp == nil {
		p.mu.Unlock()
		return nil
	}
	committing := cp.committing
	if !committing {
		cp.cancelled, cp.cause = true, cause
		cp.cancel(cause)
	}
	p.mu.Unlock()
	timer := time.NewTimer(bound)
	defer timer.Stop()
	select {
	case <-cp.done:
		return nil
	case <-timer.C:
	case <-ctx.Done():
	}
	if blockFor > 0 {
		p.mu.Lock()
		if p.blockGen == gen {
			p.blockUntil = time.Time{}
		}
		p.mu.Unlock()
	}
	return &snapshotNotUnwoundError{committing: committing, bound: bound}
}

// clearRestartBlock lets the producer start captures again: the restart that
// cancelled one was withdrawn or refused after the cancel.
func (p *snapshotProducer) clearRestartBlock() {
	p.mu.Lock()
	p.blockUntil = time.Time{}
	p.mu.Unlock()
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
		switch {
		case set.TailEvents > 0 && head-lastSeq >= set.TailEvents:
			reason = fmt.Sprintf("tail of %d events", head-lastSeq)
		case now.Sub(lastAt) < set.Interval:
			return snapshotNotDue, ""
		default:
			reason = "interval"
		}
	}

	// A failed attempt uploaded chunks the host keeps: wait before the next.
	p.mu.Lock()
	failures, failedHead, retryAt := p.failures, p.failedHead, p.retryAt
	p.mu.Unlock()
	if failures > 0 && now.Before(retryAt) {
		return snapshotBackoff, fmt.Sprintf("backing off after %d failed attempt(s), the last at head %d; the next attempt is not before %s", failures, failedHead, retryAt.UTC().Format(time.RFC3339))
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

	// A restart that cancelled a capture holds new ones back until the VM is
	// gone or the restart is withdrawn.
	cp, capCtx, ok := p.begin(ctx, d.Ready)
	if !ok {
		return snapshotSkipped, "a planned restart cancelled the last capture and is pending"
	}
	// A snapshot that crashes the computer is not retried forever.
	if d.StorePath != "" {
		if err := spendGuestAttempt(d.StorePath, snapshotAttemptKey, d.ComputerID, snapshotAttemptLimit); err != nil {
			_, _ = p.end(cp)
			return snapshotRefused, "closed after crashing the computer: " + err.Error()
		}
	}
	// The VACUUM INTO copy of an earlier version, if a crash left one.
	_ = os.RemoveAll(filepath.Join(d.DataDir, snapshotLegacyTempDirName))
	started := time.Now()
	p.inFlight.Store(true)
	ref, err := func() (snapshot.Ref, error) {
		defer p.inFlight.Store(false)
		return d.Take(capCtx, set.MaxDuration)
	}()
	// Returned, so it did not crash the guest: forget the counted start. A
	// capture cancelled for a restart returns too, so it never counts toward
	// snapshotAttemptLimit.
	if d.StorePath != "" {
		_ = clearGuestAttempt(d.StorePath, snapshotAttemptKey, d.ComputerID)
	}
	cancelled, cause := p.end(cp)
	// Only the cancel's own cause is a cancellation: a real failure that
	// lands in the same window is still a failure, with its backoff.
	if err != nil && cancelled && errors.Is(err, cause) {
		return snapshotCancelled, fmt.Sprintf("%v after %s with nothing recorded; not a failed attempt", context.Cause(capCtx), time.Since(started).Round(time.Second))
	}
	if err != nil {
		p.mu.Lock()
		p.failures++
		p.failedHead = head
		ended := time.Now()
		if d.Now != nil {
			ended = d.Now()
		}
		p.retryAt = ended.Add(snapshotBackoffWait(p.failures))
		n, next := p.failures, p.retryAt
		p.mu.Unlock()
		cause := err.Error()
		if errors.Is(err, snapshot.ErrTimeBudget) {
			cause = fmt.Sprintf("aborted at the %s time budget (%s) with nothing recorded: %s", set.MaxDuration, envSnapshotMaxDuration, cause)
		}
		return snapshotFailed, fmt.Sprintf("%s (attempt at head %d; failure %d; next attempt not before %s)", cause, head, n, next.UTC().Format(time.RFC3339))
	}
	p.mu.Lock()
	p.failures, p.failedHead, p.retryAt = 0, 0, time.Time{}
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
	dataDir := filepath.Dir(store.StatePath())
	return snapshotDeps{
		ComputerID: computerID,
		StorePath:  rt.cfg.StorePath,
		DataDir:    dataDir,
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
		Ready:     rt.snapshotsMayStart,
		Take: func(ctx context.Context, maxDuration time.Duration) (snapshot.Ref, error) {
			// The capture's read transaction pinned the WAL for its whole
			// stream; the bound backfills it as soon as the read ends.
			defer func() {
				cpCtx, stop := checkpointContext(ctx)
				defer stop()
				store.CheckpointAfterLongRead(cpCtx, "snapshot capture")
			}()
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
				// The WAL grows on the data volume while the read runs; the
				// check before the attempt is not enough for 90 minutes.
				FreeSpace: func() error {
					free, err := freeBytesOnVolume(dataDir)
					if err != nil {
						return fmt.Errorf("snapshot: cannot measure free space during the capture (fails closed): %w", err)
					}
					if free < snapshotFreeFloor {
						return fmt.Errorf("snapshot: free space %d bytes on the data volume fell under the %d-byte floor during the capture (the WAL is pinned)", free, snapshotFreeFloor)
					}
					return nil
				},
				// The point of no return: past it a cancel waits for the
				// root and the event, so the host and the tape agree.
				BeforeCommit: snapshotCommitGate(ctx),
				Partitions:   telemetryPartitionSource(store),
				Cipher:       rt.privateArtifactCipher,
				Chunks:       chunks,
				Appender:     rt.eventAppender,
				MaxDuration:  maxDuration,
			})
		},
	}, true
}

// snapshotCheckpointBound bounds the WAL checkpoint that follows a capture
// whose own context has ended.
const snapshotCheckpointBound = time.Second

// checkpointContext is the context for the checkpoint after a capture. A
// capture cancelled for a restart or a hibernate has a dead context, on which
// the checkpoint would fail at once and skip the backfill; it gets a detached
// context bounded by snapshotCheckpointBound instead (this runs inside the
// restart's unwind wait).
func checkpointContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx.Err() == nil {
		return ctx, func() {}
	}
	return context.WithTimeout(context.WithoutCancel(ctx), snapshotCheckpointBound)
}

// settleSnapshotAttemptAtBoot refunds at most ONE counted attempt on a planned
// boot, and none from a producer the limit has closed. A capture the host's
// kill cut (a platform_update or self-development apply, which do not go
// through the cancel) never returned, so its start was never cleared; a
// planned kill is not a crash. But the marker does not say whether a capture
// was in flight (that would need a marker format change, not made here), so
// the refund is the simple rule: a planned restart cannot wipe real crashes
// or reopen a closed producer, and a crashing capture cannot hide behind
// planned restarts for more than one attempt each. A crash boot keeps the
// count.
func (rt *Runtime) settleSnapshotAttemptAtBoot() {
	if rt == nil {
		return
	}
	if _, planned := rt.BootWasPlannedRestart(); !planned {
		return
	}
	if err := refundGuestAttempt(rt.cfg.StorePath, snapshotAttemptKey, strings.TrimSpace(rt.cfg.ComputerID), snapshotAttemptLimit); err != nil {
		log.Printf("runtime: snapshot attempt guard not refunded at a planned boot: %v", err)
	}
}

// snapshotsMayStart is false while boot is settling, under a maintenance hold
// and while the hibernate fence is closed (a capture across a hibernation can
// record a root whose chunks were collected during the pause).
func (rt *Runtime) snapshotsMayStart() bool {
	return !rt.bootInProgress.Load() && !rt.maintenanceHeld() && !rt.hibernate.closed()
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
	case snapshotCancelled:
		log.Printf("runtime: snapshot capture cancelled: %s", detail)
	case snapshotRefused, snapshotFailed, snapshotBackoff:
		// A refusal repeats at every tick until it clears; say it when it
		// changes so the log stays readable.
		key := string(outcome) + ": " + detail
		p.mu.Lock()
		changed := p.lastWhy != key
		p.lastWhy = key
		p.mu.Unlock()
		if changed {
			switch outcome {
			case snapshotFailed, snapshotBackoff:
				log.Printf("runtime: snapshot %s: %s", outcome, detail)
			default:
				log.Printf("runtime: snapshot %s (retry in %s): %s", outcome, snapshotTickEvery, detail)
			}
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
	rt.settleSnapshotAttemptAtBoot()
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
