package store

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// The guest's Dolt checkpoint cadence (ARCH-13,
// docs/problems/guest-checkpoint-on-the-write-path-2026-10-21.md). A Dolt
// commit makes the working set addressable by history and AS OF reads; the
// SQL transaction already made each write durable and the tape is the
// authority, so nothing in the write path needs the commit.
//
// Count mode (the default) commits inline in finalizeBatch every
// doltCheckpointEvery events, as before, and every writer waits behind it.
// Debounced mode (the owner setting store.debounced_checkpoint, or
// CHOIR_GUEST_CHECKPOINT=debounced) never commits in finalizeBatch: a mark
// arms a timer, an ops bound kicks it early, and the commit runs on the
// timer's goroutine outside eventMu and the appender mutex. Either way the
// dirty mark is a generation counter, so a mark never waits on a commit.
//
// A debounced commit still runs on the store's one connection
// (configureEmbeddedDoltDB pins MaxOpenConns to 1), so a finalize that
// starts during it waits at BeginTx while holding eventMu, and every other
// writer waits behind that. Debounced mode bounds how often that happens
// (once per window or ops bound); it does not remove it. The finalize
// commit-wait counters make the wait visible. Removing it needs a second
// connection or the store split (SA slice 5).

const (
	CheckpointModeCount     = "count"
	CheckpointModeDebounced = "debounced"
	// CheckpointModeClosed is reported between Close and Reopen.
	CheckpointModeClosed = "closed"

	defaultCheckpointDebounce = time.Minute
	// defaultCheckpointMaxOps kicks a commit early on a busy computer, so
	// one commit never carries an unbounded working set.
	defaultCheckpointMaxOps uint64 = 10000
	checkpointCommitTimeout        = 2 * time.Minute
	// checkpointCloseTimeout caps how long Close (planned restart,
	// projection-base rebase) waits for the background commit it cancels
	// and for the closing commit; checkpointCloseGrace is the extra wait,
	// after the deadline, for a commit that honors it to return.
	checkpointCloseTimeout = 10 * time.Second
	checkpointCloseGrace   = 250 * time.Millisecond
)

// ErrCloseLeftOpen is wrapped by every error of a Close that failed
// closed: the store is left open (pools, connector and engine), because
// closing them would tear the engine down under work still using it.
// Reopen refuses until a later Close succeeds. Callers treat every
// ErrCloseLeftOpen the same way.
var ErrCloseLeftOpen = errors.New("runtime store: close left the store open")

// ErrCheckpointCommitLive: a Dolt commit was still running (or held the
// commit lock) at the close deadline.
var ErrCheckpointCommitLive = fmt.Errorf("%w: a Dolt commit is still running", ErrCloseLeftOpen)

// ErrPoolStatementLive: a statement that is not a commit (a statement in
// flight, an open transaction) still held a pool's connection at the close
// deadline, so the pools could not be pinned.
var ErrPoolStatementLive = fmt.Errorf("%w: a statement holds a pool's connection", ErrCloseLeftOpen)

// errCloseBudgetSpent: the close deadline was gone before the pin tried
// any pool, so no statement is to blame.
var errCloseBudgetSpent = errors.New("the close deadline was spent before the pools could be pinned")

var errCommitLockTimeout = errors.New("commit lock not taken by the close deadline")

var errCancelledAtClose = errors.New("background commit cancelled by Close")

// CheckpointConfig is the checkpoint cadence's runtime configuration. Zero
// Debounce and MaxOps take the defaults.
type CheckpointConfig struct {
	Debounced bool
	Debounce  time.Duration
	MaxOps    uint64
}

// CheckpointConfigFromEnv reads CHOIR_GUEST_CHECKPOINT (count|debounced,
// default count), CHOIR_GUEST_CHECKPOINT_DEBOUNCE (a Go duration) and
// CHOIR_GUEST_CHECKPOINT_MAX_OPS. Invalid values are ignored.
func CheckpointConfigFromEnv() CheckpointConfig {
	cfg := CheckpointConfig{}
	if strings.ToLower(strings.TrimSpace(os.Getenv("CHOIR_GUEST_CHECKPOINT"))) == CheckpointModeDebounced {
		cfg.Debounced = true
	}
	if d, err := time.ParseDuration(strings.TrimSpace(os.Getenv("CHOIR_GUEST_CHECKPOINT_DEBOUNCE"))); err == nil && d > 0 {
		cfg.Debounce = d
	}
	if n, err := strconv.ParseUint(strings.TrimSpace(os.Getenv("CHOIR_GUEST_CHECKPOINT_MAX_OPS")), 10, 64); err == nil && n > 0 {
		cfg.MaxOps = n
	}
	return cfg
}

// CheckpointStats is the cadence's instrumentation, read on the
// observability surface. Inline time is time a writer waited on the write
// path; background time is not.
type CheckpointStats struct {
	Mode              string `json:"mode"`
	DebounceMS        int64  `json:"debounce_ms,omitempty"`
	MaxOps            uint64 `json:"max_ops,omitempty"`
	InlineCommits     uint64 `json:"inline_commits"`
	InlineNanos       uint64 `json:"inline_ns"`
	InlineMaxNanos    uint64 `json:"inline_max_ns"`
	BackgroundCommits uint64 `json:"background_commits"`
	BackgroundNanos   uint64 `json:"background_ns"`
	BackgroundMaxNs   uint64 `json:"background_max_ns"`
	// OtherCommits are the synchronous commits outside finalize: history
	// reads, discarded prepares, replay and Close. CancelledAtClose are
	// background commits Close cancelled; ClosingCommitsSkipped are
	// closing commits skipped or cut by the close deadline; ClosesLeftOpen
	// are Closes that failed closed (ErrCheckpointCommitLive). None of
	// them is a failure.
	OtherCommits uint64 `json:"other_commits"`
	// NoopCommits are commits Dolt answered "nothing to commit"; they are
	// in no commit count above, but their time is in their site's ns.
	NoopCommits           uint64 `json:"noop_commits"`
	CancelledAtClose      uint64 `json:"cancelled_at_close"`
	ClosingCommitsSkipped uint64 `json:"closing_commits_skipped"`
	ClosesLeftOpen        uint64 `json:"closes_left_open"`
	// EnginesLeftOpen counts engines a Close abandoned for process exit
	// because their workspace was gone from disk.
	EnginesLeftOpen uint64 `json:"engines_left_open"`
	// FinalizeCommitWaits counts finalizes whose BeginTx wait overlapped a
	// commit holding the store's one connection (the commit pins it, and
	// is counted in flight only while it does: exact for the connection
	// hold, not for the instants around it): a writer waiting on a
	// commit, inline or not. The ns are the whole BeginTx waits of those
	// finalizes; writers queued behind them on eventMu are not added.
	FinalizeCommitWaits     uint64 `json:"finalize_commit_waits"`
	FinalizeCommitWaitNanos uint64 `json:"finalize_commit_wait_ns"`
	FinalizeCommitWaitMaxNs uint64 `json:"finalize_commit_wait_max_ns"`
	Failures                uint64 `json:"failures"`
	LastError               string `json:"last_error,omitempty"`
	LastErrorAt             string `json:"last_error_at,omitempty"`
	EventsSinceCommit       uint64 `json:"events_since_commit"`
	OpsSinceCommit          uint64 `json:"ops_since_commit"`
	LastCommitAt            string `json:"last_commit_at,omitempty"`
	LastCommitAgeMS         int64  `json:"last_commit_age_ms,omitempty"`
}

type checkpointSite int

const (
	checkpointSiteOther checkpointSite = iota
	checkpointSiteInline
	checkpointSiteBackground
	// checkpointSiteClose is the closing commit; it counts as other.
	checkpointSiteClose
)

// checkpointCadence holds the mode, the timer and the counters. mu is a
// leaf: nothing that takes another store lock is called while holding it.
type checkpointCadence struct {
	mu        sync.Mutex
	debounced bool
	debounce  time.Duration
	maxOps    uint64
	timer     *time.Timer
	closed    bool
	// inFlight is set while a background commit runs, so the ops bound
	// does not queue more kicks behind it; cancelInFlight cancels it
	// (closeCheckpoint).
	inFlight       bool
	cancelInFlight context.CancelFunc
	// inFlightDone is closed when the background commit in flight returns.
	inFlightDone chan struct{}
	// leftOpenErr is the last Close's error while that Close failed closed.
	leftOpenErr error

	// events and ops are finalized since boot; committedEvents and
	// committedOps are their values when the last successful commit read
	// the dirty generation.
	events, ops                   atomic.Uint64
	committedEvents, committedOps atomic.Uint64

	inlineCommits, inlineNanos, inlineMax             atomic.Uint64
	backgroundCommits, backgroundNanos, backgroundMax atomic.Uint64
	otherCommits, noopCommits, failures               atomic.Uint64
	cancelledAtClose, closingSkipped, closesLeftOpen  atomic.Uint64
	enginesLeftOpen                                   atomic.Uint64
	// commitsStarted and commitsEnded bracket every DOLT_COMMIT while it
	// holds the pinned connection, so a finalize can tell whether its
	// BeginTx waited on one.
	commitsStarted, commitsEnded                           atomic.Uint64
	finalizeWaits, finalizeWaitNanos, finalizeWaitMaxNanos atomic.Uint64
	// closeTimeout overrides checkpointCloseTimeout (tests only).
	closeTimeout time.Duration
	lastCommitAt atomic.Int64 // unix nanos
	lastErr      string       // guarded by mu
	lastErrAt    time.Time    // guarded by mu
}

// ConfigureCheckpoint sets the cadence. Turning debounced mode off stops its
// timer; what is pending is committed by the next counted finalize.
func (s *Store) ConfigureCheckpoint(cfg CheckpointConfig) {
	if s == nil {
		return
	}
	c := &s.checkpoint
	c.mu.Lock()
	defer c.mu.Unlock()
	c.debounced = cfg.Debounced
	c.debounce = cfg.Debounce
	if c.debounce <= 0 {
		c.debounce = defaultCheckpointDebounce
	}
	c.maxOps = cfg.MaxOps
	if c.maxOps == 0 {
		c.maxOps = defaultCheckpointMaxOps
	}
	if !c.debounced && c.timer != nil {
		c.timer.Stop()
		c.timer = nil
	}
}

// checkpointDebounced reports whether finalize leaves the commit to the
// background committer.
func (s *Store) checkpointDebounced() bool {
	c := &s.checkpoint
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.debounced && !c.closed
}

// noteCheckpointFinalize counts one finalized event and its ops, and in
// debounced mode kicks the committer at once when the ops bound is reached.
func (s *Store) noteCheckpointFinalize(ops int) {
	c := &s.checkpoint
	c.events.Add(1)
	total := c.ops.Add(uint64(ops))
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.debounced || c.closed || c.inFlight {
		return
	}
	if total-c.committedOps.Load() >= c.maxOps {
		c.armLocked(s, 0)
	}
}

// armCheckpoint arms the debounce timer if debounced mode is on and no
// timer is armed. Called on every mark.
func (s *Store) armCheckpoint() {
	c := &s.checkpoint
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.debounced || c.closed || c.timer != nil {
		return
	}
	c.armLocked(s, c.debounce)
}

func (c *checkpointCadence) armLocked(s *Store, after time.Duration) {
	if c.timer != nil {
		c.timer.Reset(after)
		return
	}
	c.timer = time.AfterFunc(after, s.fireCheckpoint)
}

func (s *Store) fireCheckpoint() {
	c := &s.checkpoint
	c.mu.Lock()
	c.timer = nil
	if c.closed || !c.debounced || c.inFlight {
		c.mu.Unlock()
		return
	}
	parent, cancelCause := context.WithCancelCause(context.Background())
	ctx, cancel := context.WithTimeout(parent, checkpointCommitTimeout)
	defer cancel()
	defer cancelCause(nil)
	done := make(chan struct{})
	c.inFlight = true
	// Close cancels with a cause, so the commit's error counts as
	// cancelled at close, not as a failure, even after a failed Close
	// reopens the cadence.
	c.cancelInFlight = func() { cancelCause(errCancelledAtClose) }
	c.inFlightDone = done
	c.mu.Unlock()
	err := s.commitDoltCheckpointAt(ctx, "guest snapshot (debounced)", checkpointSiteBackground)
	c.mu.Lock()
	c.inFlight = false
	c.cancelInFlight = nil
	c.inFlightDone = nil
	c.mu.Unlock()
	close(done)
	if err != nil {
		log.Printf("runtime store: background checkpoint: %v", err)
	}
	if err != nil || s.doltHistoryIsDirty() {
		// Retry a failure on the window; a mark that arrived during the
		// commit found the timer armed and is taken here.
		s.armCheckpoint()
	}
}

func (c *checkpointCadence) isClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

func (s *Store) checkpointTimerArmed() bool {
	c := &s.checkpoint
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.timer != nil
}

// closeCheckpoint stops the timer, cancels a background commit in flight
// and waits for it, then, in debounced mode, commits what is pending once
// (on a retried Close too). The closing commit runs to deadline; every
// wait gives up at hard (the deadline plus one grace, for a commit that
// honors the deadline to return). If a commit is still running then, it
// returns an ErrCheckpointCommitLive and the caller must not close the
// pools: the deadline is a context, and a DOLT_COMMIT that does not check
// it mid-write runs on. Store.Close holds closeMu across this call.
func (s *Store) closeCheckpoint(deadline, hard time.Time) error {
	c := &s.checkpoint
	c.mu.Lock()
	debounced := c.debounced
	c.closed = true
	if c.timer != nil {
		c.timer.Stop()
		c.timer = nil
	}
	if c.cancelInFlight != nil {
		c.cancelInFlight()
	}
	committer := c.inFlightDone
	c.mu.Unlock()
	if committer != nil && !waitUntil(committer, hard) {
		// The closing commit, in debounced mode, is not attempted.
		if debounced {
			c.closingSkipped.Add(1)
		}
		return c.leaveOpen(fmt.Errorf("%w (the cancelled background commit has not returned)", ErrCheckpointCommitLive))
	}
	if !debounced {
		return nil
	}
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	finished := make(chan struct{})
	var err error
	go func() {
		defer close(finished)
		err = s.commitDoltCheckpointLocking(ctx, "guest snapshot at close", checkpointSiteClose, true)
	}()
	if !waitUntil(finished, hard) {
		// Counted (skipped, or committed) when it returns.
		return c.leaveOpen(fmt.Errorf("%w (the closing commit has not returned)", ErrCheckpointCommitLive))
	}
	if errors.Is(err, errCommitLockTimeout) {
		// Another commit holds the lock, so it is running on the engine.
		return c.leaveOpen(fmt.Errorf("%w (another commit holds the commit lock)", ErrCheckpointCommitLive))
	}
	if err != nil {
		log.Printf("runtime store: checkpoint at close: %v", err)
	}
	return nil
}

// waitUntil waits for done until the hard deadline.
func waitUntil(done <-chan struct{}, hard time.Time) bool {
	select {
	case <-done:
		return true
	default:
	}
	timer := time.NewTimer(time.Until(hard))
	defer timer.Stop()
	select {
	case <-done:
		return true
	case <-timer.C:
		return false
	}
}

func (c *checkpointCadence) closeLimit() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closeTimeout > 0 {
		return c.closeTimeout
	}
	return checkpointCloseTimeout
}

// leaveOpen records a Close that failed closed (once per failing Close)
// with err, an ErrCloseLeftOpen naming the cause, and returns it.
func (c *checkpointCadence) leaveOpen(err error) error {
	c.mu.Lock()
	c.leftOpenErr = err
	c.mu.Unlock()
	c.closesLeftOpen.Add(1)
	log.Printf("%v", err)
	return err
}

func (c *checkpointCadence) clearLeftOpen() {
	c.mu.Lock()
	c.leftOpenErr = nil
	c.mu.Unlock()
}

// leftOpen returns the last Close's error while that Close failed closed.
func (c *checkpointCadence) leftOpen() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.leftOpenErr
}

// recordCutAtClose counts a background commit cancelled by Close, or a
// closing commit cut or skipped by the close deadline. It reports false
// for other sites, whose errors stay failures.
func (c *checkpointCadence) recordCutAtClose(site checkpointSite) bool {
	switch site {
	case checkpointSiteBackground:
		c.cancelledAtClose.Add(1)
	case checkpointSiteClose:
		c.closingSkipped.Add(1)
	default:
		return false
	}
	return true
}

// reopenCheckpoint undoes closeCheckpoint for a Store that Reopen brings
// back (Close then Reopen on the same *Store, the projection-base rebase
// path): the configured mode holds again and the adopted workspace, which
// Reopen marks dirty, gets a debounced commit.
func (s *Store) reopenCheckpoint() {
	c := &s.checkpoint
	c.mu.Lock()
	c.closed = false
	c.mu.Unlock()
	s.armCheckpoint()
}

// recordCommit counts a commit at its site. A no-op ("nothing to commit")
// counts apart, keeps its time in its site's ns (a writer waited on it all
// the same) and leaves the last commit time alone.
func (c *checkpointCadence) recordCommit(site checkpointSite, elapsed time.Duration, events, ops uint64, noop bool) {
	ns := uint64(elapsed.Nanoseconds())
	if ns == 0 {
		ns = 1
	}
	count := uint64(1)
	if noop {
		count = 0
		c.noopCommits.Add(1)
	}
	switch site {
	case checkpointSiteInline:
		c.inlineCommits.Add(count)
		c.inlineNanos.Add(ns)
		storeMax(&c.inlineMax, ns)
	case checkpointSiteBackground:
		c.backgroundCommits.Add(count)
		c.backgroundNanos.Add(ns)
		storeMax(&c.backgroundMax, ns)
	default:
		c.otherCommits.Add(count)
	}
	c.committedEvents.Store(events)
	c.committedOps.Store(ops)
	if !noop {
		c.lastCommitAt.Store(time.Now().UnixNano())
	}
}

// finalizeConnWait is what a finalize reads before BeginTx.
type finalizeConnWait struct {
	at             time.Time
	started, ended uint64
}

func (c *checkpointCadence) startFinalizeConnWait() finalizeConnWait {
	// Ended before started: a commit that ends between the two loads
	// cannot look in flight when it was not.
	ended := c.commitsEnded.Load()
	return finalizeConnWait{at: time.Now(), started: c.commitsStarted.Load(), ended: ended}
}

// noteFinalizeConnWait counts the finalize's BeginTx wait if a commit was
// in flight when it began or ended while it waited.
func (c *checkpointCadence) noteFinalizeConnWait(w finalizeConnWait) {
	if w.started == w.ended && c.commitsEnded.Load() == w.ended {
		return
	}
	ns := uint64(time.Since(w.at).Nanoseconds())
	if ns == 0 {
		ns = 1
	}
	c.finalizeWaits.Add(1)
	c.finalizeWaitNanos.Add(ns)
	storeMax(&c.finalizeWaitMaxNanos, ns)
}

func (c *checkpointCadence) recordFailure(err error) {
	c.failures.Add(1)
	c.mu.Lock()
	c.lastErr = err.Error()
	c.lastErrAt = time.Now().UTC()
	c.mu.Unlock()
}

func storeMax(v *atomic.Uint64, n uint64) {
	for {
		old := v.Load()
		if n <= old || v.CompareAndSwap(old, n) {
			return
		}
	}
}

// CheckpointStats returns a snapshot of the cadence's counters.
func (s *Store) CheckpointStats() CheckpointStats {
	if s == nil {
		return CheckpointStats{Mode: CheckpointModeCount}
	}
	c := &s.checkpoint
	out := CheckpointStats{Mode: CheckpointModeCount}
	c.mu.Lock()
	if c.debounced {
		out.Mode = CheckpointModeDebounced
		out.DebounceMS = c.debounce.Milliseconds()
		out.MaxOps = c.maxOps
	}
	if c.closed {
		out.Mode = CheckpointModeClosed
	}
	out.LastError = c.lastErr
	if !c.lastErrAt.IsZero() {
		out.LastErrorAt = c.lastErrAt.Format(time.RFC3339Nano)
	}
	c.mu.Unlock()
	out.InlineCommits = c.inlineCommits.Load()
	out.InlineNanos = c.inlineNanos.Load()
	out.InlineMaxNanos = c.inlineMax.Load()
	out.BackgroundCommits = c.backgroundCommits.Load()
	out.BackgroundNanos = c.backgroundNanos.Load()
	out.BackgroundMaxNs = c.backgroundMax.Load()
	out.OtherCommits = c.otherCommits.Load()
	out.NoopCommits = c.noopCommits.Load()
	out.CancelledAtClose = c.cancelledAtClose.Load()
	out.ClosingCommitsSkipped = c.closingSkipped.Load()
	out.ClosesLeftOpen = c.closesLeftOpen.Load()
	out.EnginesLeftOpen = c.enginesLeftOpen.Load()
	out.FinalizeCommitWaits = c.finalizeWaits.Load()
	out.FinalizeCommitWaitNanos = c.finalizeWaitNanos.Load()
	out.FinalizeCommitWaitMaxNs = c.finalizeWaitMaxNanos.Load()
	out.Failures = c.failures.Load()
	out.EventsSinceCommit = subFloor(c.events.Load(), c.committedEvents.Load())
	out.OpsSinceCommit = subFloor(c.ops.Load(), c.committedOps.Load())
	if at := c.lastCommitAt.Load(); at > 0 {
		t := time.Unix(0, at).UTC()
		out.LastCommitAt = t.Format(time.RFC3339Nano)
		out.LastCommitAgeMS = time.Since(t).Milliseconds()
	}
	return out
}

func subFloor(a, b uint64) uint64 {
	if a < b {
		return 0
	}
	return a - b
}
