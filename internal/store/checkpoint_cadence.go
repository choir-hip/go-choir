package store

import (
	"context"
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
// dirty mark is a generation counter, so a write never waits on a commit.

const (
	CheckpointModeCount     = "count"
	CheckpointModeDebounced = "debounced"

	defaultCheckpointDebounce = time.Minute
	// defaultCheckpointMaxOps kicks a commit early on a busy computer, so
	// one commit never carries an unbounded working set.
	defaultCheckpointMaxOps uint64 = 10000
	checkpointCommitTimeout        = 2 * time.Minute
)

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
	// reads, discarded prepares, replay and Close.
	OtherCommits      uint64 `json:"other_commits"`
	Failures          uint64 `json:"failures"`
	LastError         string `json:"last_error,omitempty"`
	LastErrorAt       string `json:"last_error_at,omitempty"`
	EventsSinceCommit uint64 `json:"events_since_commit"`
	OpsSinceCommit    uint64 `json:"ops_since_commit"`
	LastCommitAt      string `json:"last_commit_at,omitempty"`
	LastCommitAgeMS   int64  `json:"last_commit_age_ms,omitempty"`
}

type checkpointSite int

const (
	checkpointSiteOther checkpointSite = iota
	checkpointSiteInline
	checkpointSiteBackground
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
	// does not queue more kicks behind it.
	inFlight bool

	// events and ops are finalized since boot; committedEvents and
	// committedOps are their values when the last successful commit read
	// the dirty generation.
	events, ops                   atomic.Uint64
	committedEvents, committedOps atomic.Uint64

	inlineCommits, inlineNanos, inlineMax             atomic.Uint64
	backgroundCommits, backgroundNanos, backgroundMax atomic.Uint64
	otherCommits, failures                            atomic.Uint64
	lastCommitAt                                      atomic.Int64 // unix nanos
	lastErr                                           string       // guarded by mu
	lastErrAt                                         time.Time    // guarded by mu
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
	c.inFlight = true
	c.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), checkpointCommitTimeout)
	defer cancel()
	err := s.commitDoltCheckpointAt(ctx, "guest snapshot (debounced)", checkpointSiteBackground)
	c.mu.Lock()
	c.inFlight = false
	c.mu.Unlock()
	if err != nil {
		log.Printf("runtime store: background checkpoint: %v", err)
	}
	if err != nil || s.doltHistoryIsDirty() {
		// Retry a failure on the window; a mark that arrived during the
		// commit found the timer armed and is taken here.
		s.armCheckpoint()
	}
}

func (s *Store) checkpointTimerArmed() bool {
	c := &s.checkpoint
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.timer != nil
}

// closeCheckpoint stops the timer and, in debounced mode, commits what is
// pending once. Count mode keeps its old Close (no commit).
func (s *Store) closeCheckpoint() {
	c := &s.checkpoint
	c.mu.Lock()
	debounced := c.debounced && !c.closed
	c.closed = true
	if c.timer != nil {
		c.timer.Stop()
		c.timer = nil
	}
	c.mu.Unlock()
	if !debounced {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), checkpointCommitTimeout)
	defer cancel()
	if err := s.commitDoltCheckpointAt(ctx, "guest snapshot at close", checkpointSiteOther); err != nil {
		log.Printf("runtime store: checkpoint at close: %v", err)
	}
}

func (c *checkpointCadence) recordCommit(site checkpointSite, elapsed time.Duration, events, ops uint64) {
	ns := uint64(elapsed.Nanoseconds())
	if ns == 0 {
		ns = 1
	}
	switch site {
	case checkpointSiteInline:
		c.inlineCommits.Add(1)
		c.inlineNanos.Add(ns)
		storeMax(&c.inlineMax, ns)
	case checkpointSiteBackground:
		c.backgroundCommits.Add(1)
		c.backgroundNanos.Add(ns)
		storeMax(&c.backgroundMax, ns)
	default:
		c.otherCommits.Add(1)
	}
	c.committedEvents.Store(events)
	c.committedOps.Store(ops)
	c.lastCommitAt.Store(time.Now().UnixNano())
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
