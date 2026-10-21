package store

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// The guest's Dolt checkpoint off the write path (ARCH-13,
// docs/problems/guest-checkpoint-on-the-write-path-2026-10-21.md). Ways it
// can fail, written before the code:
//   - a mark made while a commit is in flight is lost, so the store reports
//     clean while rows sit uncommitted;
//   - a mark waits on a commit in flight, so writers still stall;
//   - the background commit races a finalize transaction;
//   - a failed background commit is never retried;
//   - a quiet computer never commits;
//   - Close leaves a timer that commits on a closed store, or drops the
//     pending commit;
//   - turning the mode off leaves a timer running, or does not return to
//     counted inline commits;
//   - count mode drifts from today's cadence;
//   - the counters cannot tell write-path time from background time.

// finalizeTestEvents appends n events after the head of computerID.
func finalizeTestEvents(t *testing.T, s *Store, computerID string, n int) {
	t.Helper()
	ctx := context.Background()
	for i := 0; i < n; i++ {
		head, err := s.Head(ctx, computerID)
		if err != nil || head == nil {
			t.Fatalf("head before event %d: %v", i, err)
		}
		eventID, err := computerevent.NewEventID()
		if err != nil {
			t.Fatal(err)
		}
		event, digest := prepareProjectionEvent(t, s, *head, eventID, "cadence-"+eventID)
		batch := testBatchCheckpointBatch(t, computerID, event.EventID, digest)
		if err := s.FinalizeBatch(ctx, computerID, digest, signedReceipt(t, computerID, digest, event.Sequence), &batch); err != nil {
			t.Fatalf("finalize event %d: %v", i, err)
		}
	}
}

func historyCommits(t *testing.T, s *Store, computerID string) int {
	t.Helper()
	var commits int
	if err := s.db.QueryRowContext(context.Background(), `SELECT COUNT(DISTINCT commit_hash) FROM dolt_history_computer_event_index WHERE computer_id = ?`, computerID).Scan(&commits); err != nil {
		t.Fatalf("count history commits: %v", err)
	}
	return commits
}

func waitForCheckpoint(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// Count mode is today's cadence, now measured: one inline commit per
// `every` finalized events, its time charged to the write path.
func TestCheckpointCountModeCountsInlineCommits(t *testing.T) {
	s := openProjectStore(t)
	s.doltCheckpointEvery = 3
	const computerID = "computer-cadence-count"
	_, _ = prepareGenesis(t, s, computerID, "genesis-cadence-count")
	finalizeTestEvents(t, s, computerID, 5)
	stats := s.CheckpointStats()
	if stats.Mode != CheckpointModeCount {
		t.Fatalf("mode = %q, want %q (the default)", stats.Mode, CheckpointModeCount)
	}
	if stats.InlineCommits != 2 || historyCommits(t, s, computerID) != 2 {
		t.Fatalf("inline commits = %d, history commits = %d, want 2 and 2", stats.InlineCommits, historyCommits(t, s, computerID))
	}
	if stats.InlineNanos == 0 || stats.InlineMaxNanos == 0 || stats.InlineMaxNanos > stats.InlineNanos {
		t.Fatalf("write-path time not recorded: %+v", stats)
	}
	if stats.BackgroundCommits != 0 {
		t.Fatalf("background commits in count mode: %+v", stats)
	}
	if stats.EventsSinceCommit != 0 {
		t.Fatalf("events since commit = %d after the 6th event committed, want 0", stats.EventsSinceCommit)
	}
}

// Debounced mode never commits on the write path; the background committer
// commits within the window, so a quiet computer still commits.
func TestCheckpointDebouncedCommitsInBackground(t *testing.T) {
	s := openProjectStore(t)
	s.doltCheckpointEvery = 1
	s.ConfigureCheckpoint(CheckpointConfig{Debounced: true, Debounce: 50 * time.Millisecond, MaxOps: 1 << 30})
	const computerID = "computer-cadence-debounced"
	_, _ = prepareGenesis(t, s, computerID, "genesis-cadence-debounced")
	finalizeTestEvents(t, s, computerID, 4)
	if got := s.CheckpointStats(); got.InlineCommits != 0 || got.InlineNanos != 0 {
		t.Fatalf("debounced mode committed on the write path: %+v", got)
	}
	waitForCheckpoint(t, "a background commit of every event", func() bool {
		st := s.CheckpointStats()
		return st.BackgroundCommits >= 1 && st.EventsSinceCommit == 0
	})
	if historyCommits(t, s, computerID) < 1 {
		t.Fatal("the background commit did not reach Dolt history")
	}
	stats := s.CheckpointStats()
	if stats.Mode != CheckpointModeDebounced || stats.DebounceMS != 50 || stats.BackgroundNanos == 0 || stats.LastCommitAt == "" {
		t.Fatalf("stats after a background commit: %+v", stats)
	}
	if stats.EventsSinceCommit != 0 || stats.OpsSinceCommit != 0 {
		t.Fatalf("since-commit counters not reset: %+v", stats)
	}
}

// Past the ops bound the committer is kicked at once, not at the window.
func TestCheckpointDebouncedOpsBoundKicksCommit(t *testing.T) {
	s := openProjectStore(t)
	s.ConfigureCheckpoint(CheckpointConfig{Debounced: true, Debounce: time.Hour, MaxOps: 2})
	const computerID = "computer-cadence-opsbound"
	_, _ = prepareGenesis(t, s, computerID, "genesis-cadence-opsbound")
	finalizeTestEvents(t, s, computerID, 3)
	waitForCheckpoint(t, "the ops bound to kick a commit", func() bool { return s.CheckpointStats().BackgroundCommits >= 1 })
	if s.CheckpointStats().InlineCommits != 0 {
		t.Fatal("the ops bound committed on the write path")
	}
}

// A mark never waits on a commit in flight.
func TestCheckpointMarkDoesNotWaitOnCommit(t *testing.T) {
	s := openProjectStore(t)
	s.doltCommitMu.Lock()
	done := make(chan struct{})
	go func() {
		s.markDoltHistoryDirty()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		s.doltCommitMu.Unlock()
		t.Fatal("markDoltHistoryDirty blocked behind a commit in flight")
	}
	s.doltCommitMu.Unlock()
}

// A mark made after a commit read the dirty state keeps the store dirty, so
// the next commit takes it.
func TestCheckpointMarkDuringCommitStaysDirty(t *testing.T) {
	ctx := context.Background()
	s := openProjectStore(t)
	const computerID = "computer-cadence-gen"
	_, _ = prepareGenesis(t, s, computerID, "genesis-cadence-gen")
	s.markDoltHistoryDirty()
	marked := false
	s.checkpointExecHook = func() error {
		if !marked {
			marked = true
			s.markDoltHistoryDirty()
		}
		return nil
	}
	if err := s.commitDoltCheckpoint(ctx, "first"); err != nil {
		t.Fatal(err)
	}
	if !s.doltHistoryIsDirty() {
		t.Fatal("a mark made during the commit was cleared by it")
	}
	s.checkpointExecHook = nil
	if err := s.commitDoltCheckpoint(ctx, "second"); err != nil {
		t.Fatal(err)
	}
	if s.doltHistoryIsDirty() {
		t.Fatal("the second commit left the store dirty")
	}
}

// A failed background commit is retried on the window, and counted.
func TestCheckpointBackgroundFailureRetries(t *testing.T) {
	s := openProjectStore(t)
	var mu sync.Mutex
	fail := 1
	s.checkpointExecHook = func() error {
		mu.Lock()
		defer mu.Unlock()
		if fail > 0 {
			fail--
			return errors.New("injected commit failure")
		}
		return nil
	}
	s.ConfigureCheckpoint(CheckpointConfig{Debounced: true, Debounce: 20 * time.Millisecond, MaxOps: 1 << 30})
	const computerID = "computer-cadence-retry"
	_, _ = prepareGenesis(t, s, computerID, "genesis-cadence-retry")
	finalizeTestEvents(t, s, computerID, 1)
	waitForCheckpoint(t, "a retried background commit", func() bool { return s.CheckpointStats().BackgroundCommits >= 1 })
	stats := s.CheckpointStats()
	if stats.Failures < 1 || stats.LastError == "" {
		t.Fatalf("the failure was not counted: %+v", stats)
	}
}

// Writers on several computers while the background committer runs every
// millisecond: no error, every row finalized (run with -race).
func TestCheckpointBackgroundCommitWithConcurrentWriters(t *testing.T) {
	s := openProjectStore(t)
	s.ConfigureCheckpoint(CheckpointConfig{Debounced: true, Debounce: time.Millisecond, MaxOps: 3})
	const writers, events = 3, 8
	for w := 0; w < writers; w++ {
		_, _ = prepareGenesis(t, s, fmt.Sprintf("computer-cadence-race-%d", w), fmt.Sprintf("genesis-cadence-race-%d", w))
	}
	var wg sync.WaitGroup
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			finalizeTestEvents(t, s, fmt.Sprintf("computer-cadence-race-%d", w), events)
		}(w)
	}
	wg.Wait()
	waitForCheckpoint(t, "a background commit", func() bool { return s.CheckpointStats().BackgroundCommits >= 1 })
	assertCount(t, s, `SELECT COUNT(*) FROM computer_event_index WHERE computer_id LIKE 'computer-cadence-race-%' AND status = 'finalized'`, writers*(events+1))
	if stats := s.CheckpointStats(); stats.Failures != 0 || stats.InlineCommits != 0 {
		t.Fatalf("stats after concurrent writers: %+v", stats)
	}
}

// Turning debounced mode off stops the timer and returns to counted inline
// commits.
func TestCheckpointSwitchOffReturnsToCount(t *testing.T) {
	s := openProjectStore(t)
	s.doltCheckpointEvery = 2
	s.ConfigureCheckpoint(CheckpointConfig{Debounced: true, Debounce: time.Hour, MaxOps: 1 << 30})
	const computerID = "computer-cadence-switch"
	_, _ = prepareGenesis(t, s, computerID, "genesis-cadence-switch")
	s.ConfigureCheckpoint(CheckpointConfig{})
	if s.checkpointTimerArmed() {
		t.Fatal("turning debounced mode off left its timer armed")
	}
	finalizeTestEvents(t, s, computerID, 3)
	if stats := s.CheckpointStats(); stats.Mode != CheckpointModeCount || stats.InlineCommits < 1 {
		t.Fatalf("after turning it off: %+v", stats)
	}
}

// Close stops the timer and commits what is pending once.
func TestCheckpointCloseCommitsPending(t *testing.T) {
	path := testStorePath(t)
	cleanupTestStorePath(path)
	t.Cleanup(func() { cleanupTestStorePath(path) })
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s.ConfigureCheckpoint(CheckpointConfig{Debounced: true, Debounce: time.Hour, MaxOps: 1 << 30})
	const computerID = "computer-cadence-close"
	_, _ = prepareGenesis(t, s, computerID, "genesis-cadence-close")
	finalizeTestEvents(t, s, computerID, 2)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if s.checkpointTimerArmed() {
		t.Fatal("Close left the debounce timer armed")
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if historyCommits(t, reopened, computerID) < 1 {
		t.Fatal("Close did not commit the pending working set")
	}
}

func TestCheckpointConfigFromEnv(t *testing.T) {
	t.Setenv("CHOIR_GUEST_CHECKPOINT", "")
	if cfg := CheckpointConfigFromEnv(); cfg.Debounced {
		t.Fatal("debounced with no env: the default must be count mode")
	}
	t.Setenv("CHOIR_GUEST_CHECKPOINT", "debounced")
	t.Setenv("CHOIR_GUEST_CHECKPOINT_DEBOUNCE", "90s")
	t.Setenv("CHOIR_GUEST_CHECKPOINT_MAX_OPS", "500")
	cfg := CheckpointConfigFromEnv()
	if !cfg.Debounced || cfg.Debounce != 90*time.Second || cfg.MaxOps != 500 {
		t.Fatalf("env config = %+v", cfg)
	}
	t.Setenv("CHOIR_GUEST_CHECKPOINT_DEBOUNCE", "nonsense")
	t.Setenv("CHOIR_GUEST_CHECKPOINT_MAX_OPS", "-1")
	cfg = CheckpointConfigFromEnv()
	if cfg.Debounce != 0 || cfg.MaxOps != 0 {
		t.Fatalf("invalid env values not ignored: %+v", cfg)
	}
	s := openProjectStore(t)
	s.ConfigureCheckpoint(CheckpointConfig{Debounced: true})
	if stats := s.CheckpointStats(); stats.DebounceMS != defaultCheckpointDebounce.Milliseconds() || stats.MaxOps != defaultCheckpointMaxOps {
		t.Fatalf("zero values did not take the defaults: %+v", stats)
	}
}
