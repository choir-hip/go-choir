package store

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
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
//   - the counters cannot tell write-path time from background time;
//   - a finalize that waits on a background commit for the store's one
//     connection is invisible to the counters;
//   - a wedged closing commit holds Close for the full commit timeout;
//   - a "nothing to commit" answer is counted as a commit;
//   - the mode report says debounced after Close;
//   - Close waits out a background commit already holding the commit lock;
//   - a background commit that passed the closed check runs after Close;
//   - a finalize is counted as waiting on a commit that did not hold the
//     connection.

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
	s.checkpointExecHook = func(context.Context) error {
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
	s.checkpointExecHook = func(context.Context) error {
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

// Close then Reopen on the same Store (the projection-base rebase path)
// keeps debounced mode working: finalize does not commit inline, the
// adopted workspace is committed in the background, and the mode report
// matches what finalize does.
func TestCheckpointReopenAfterCloseStaysDebounced(t *testing.T) {
	path := testStorePath(t)
	cleanupTestStorePath(path)
	t.Cleanup(func() { cleanupTestStorePath(path) })
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	s.doltCheckpointEvery = 1
	s.ConfigureCheckpoint(CheckpointConfig{Debounced: true, Debounce: 20 * time.Millisecond, MaxOps: 1 << 30})
	const computerID = "computer-cadence-reopen"
	_, _ = prepareGenesis(t, s, computerID, "genesis-cadence-reopen")
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Reopen(path); err != nil {
		t.Fatal(err)
	}
	if !s.checkpointDebounced() {
		t.Fatal("Reopen left the debounced cadence closed")
	}
	before := s.CheckpointStats().BackgroundCommits
	finalizeTestEvents(t, s, computerID, 2)
	if got := s.CheckpointStats().InlineCommits; got != 0 {
		t.Fatalf("finalize committed inline %d times after Reopen in debounced mode", got)
	}
	waitForCheckpoint(t, "a background commit after Reopen", func() bool {
		st := s.CheckpointStats()
		return st.BackgroundCommits > before && st.EventsSinceCommit == 0
	})
}

// A background commit holds the store's one connection (MaxOpenConns 1), so
// a finalize that starts during it waits at BeginTx while holding eventMu.
// Debounced mode bounds how often that happens; the counters must show it.
func TestCheckpointFinalizeWaitOnCommitIsCounted(t *testing.T) {
	ctx := context.Background()
	s := openProjectStore(t)
	const computerID = "computer-cadence-connwait"
	_, _ = prepareGenesis(t, s, computerID, "genesis-cadence-connwait")
	finalizeTestEvents(t, s, computerID, 1)
	if got := s.CheckpointStats(); got.FinalizeCommitWaits != 0 {
		t.Fatalf("a finalize with no commit in flight counted a wait: %+v", got)
	}

	head, err := s.Head(ctx, computerID)
	if err != nil || head == nil {
		t.Fatalf("head: %v", err)
	}
	eventID, err := computerevent.NewEventID()
	if err != nil {
		t.Fatal(err)
	}
	event, digest := prepareProjectionEvent(t, s, *head, eventID, "connwait-"+eventID)
	batch := testBatchCheckpointBatch(t, computerID, event.EventID, digest)
	receipt := signedReceipt(t, computerID, digest, event.Sequence)

	const hold = 150 * time.Millisecond
	held := make(chan struct{})
	release := make(chan struct{})
	s.checkpointExecHook = func(context.Context) error {
		// Runs while the commit holds the store's one connection: a
		// stand-in for a slow DOLT_COMMIT.
		close(held)
		<-release
		return nil
	}
	s.markDoltHistoryDirty()
	commitDone := make(chan error, 1)
	go func() {
		commitDone <- s.commitDoltCheckpointAt(ctx, "held", checkpointSiteBackground)
	}()
	select {
	case <-held:
	case <-time.After(10 * time.Second):
		t.Fatal("the commit never took the connection")
	}
	waitsBefore := s.db.Stats().WaitCount
	finalizeDone := make(chan error, 1)
	go func() {
		finalizeDone <- s.FinalizeBatch(ctx, computerID, digest, receipt, &batch)
	}()
	// The finalize is queued for the connection once the pool counts a
	// wait; then hold the commit for `hold` and release it.
	waitForCheckpoint(t, "the finalize to queue for the connection", func() bool {
		return s.db.Stats().WaitCount > waitsBefore
	})
	time.Sleep(hold)
	close(release)
	for name, ch := range map[string]chan error{"commit": commitDone, "finalize": finalizeDone} {
		select {
		case err := <-ch:
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("%s did not finish", name)
		}
	}
	s.checkpointExecHook = nil
	stats := s.CheckpointStats()
	if stats.FinalizeCommitWaits != 1 {
		t.Fatalf("finalize_commit_waits = %d, want 1: %+v", stats.FinalizeCommitWaits, stats)
	}
	if min := uint64(hold); stats.FinalizeCommitWaitMaxNs < min || stats.FinalizeCommitWaitNanos < min {
		t.Fatalf("the wait on the commit was not measured (want >= %d ns): %+v", min, stats)
	}
}

// A wedged closing commit holds Close for the close cap, not the
// background commit's timeout.
func TestCheckpointCloseCommitIsBoundedByCloseTimeout(t *testing.T) {
	if checkpointCloseTimeout >= checkpointCommitTimeout {
		t.Fatalf("close cap %s is not shorter than the commit timeout %s", checkpointCloseTimeout, checkpointCommitTimeout)
	}
	path := testStorePath(t)
	cleanupTestStorePath(path)
	t.Cleanup(func() { cleanupTestStorePath(path) })
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s.ConfigureCheckpoint(CheckpointConfig{Debounced: true, Debounce: time.Hour, MaxOps: 1 << 30})
	s.checkpoint.closeTimeout = 100 * time.Millisecond
	const computerID = "computer-cadence-closecap"
	_, _ = prepareGenesis(t, s, computerID, "genesis-cadence-closecap")
	finalizeTestEvents(t, s, computerID, 1)
	sawDeadline := make(chan bool, 1)
	s.checkpointExecHook = func(ctx context.Context) error {
		_, ok := ctx.Deadline()
		sawDeadline <- ok
		<-ctx.Done()
		return ctx.Err()
	}
	started := time.Now()
	closed := make(chan error, 1)
	go func() { closed <- s.Close() }()
	select {
	case <-closed:
	case <-time.After(10 * time.Second):
		t.Fatal("Close waited on the wedged closing commit past the close cap")
	}
	if elapsed := time.Since(started); elapsed > 5*time.Second {
		t.Fatalf("Close took %s with a 100ms close cap", elapsed)
	}
	if ok := <-sawDeadline; !ok {
		t.Fatal("the closing commit ran without a deadline")
	}
	if st := s.CheckpointStats(); st.ClosingCommitsSkipped != 1 || st.Failures != 0 || st.LastError != "" {
		t.Fatalf("a closing commit cut by the close deadline must count as skipped, not a failure: %+v", st)
	}
}

// "nothing to commit" is not a commit: it is counted apart and moves no
// site's commit count.
func TestCheckpointNoopCommitIsCountedApart(t *testing.T) {
	ctx := context.Background()
	s := openProjectStore(t)
	const computerID = "computer-cadence-noop"
	_, _ = prepareGenesis(t, s, computerID, "genesis-cadence-noop")
	s.markDoltHistoryDirty()
	if err := s.commitDoltCheckpoint(ctx, "real"); err != nil {
		t.Fatal(err)
	}
	before := s.CheckpointStats()
	s.markDoltHistoryDirty()
	if err := s.commitDoltCheckpoint(ctx, "nothing changed"); err != nil {
		t.Fatal(err)
	}
	after := s.CheckpointStats()
	if after.NoopCommits != before.NoopCommits+1 {
		t.Fatalf("noop_commits %d -> %d, want +1", before.NoopCommits, after.NoopCommits)
	}
	if after.OtherCommits != before.OtherCommits || after.InlineCommits != before.InlineCommits || after.BackgroundCommits != before.BackgroundCommits {
		t.Fatalf("a no-op commit moved a commit count: before %+v after %+v", before, after)
	}
	if s.doltHistoryIsDirty() {
		t.Fatal("a no-op commit left the store dirty")
	}
}

// After Close the mode report says closed, not the configured mode.
func TestCheckpointModeReportsClosedAfterClose(t *testing.T) {
	path := testStorePath(t)
	cleanupTestStorePath(path)
	t.Cleanup(func() { cleanupTestStorePath(path) })
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s.ConfigureCheckpoint(CheckpointConfig{Debounced: true, Debounce: time.Hour, MaxOps: 1 << 30})
	if got := s.CheckpointStats().Mode; got != CheckpointModeDebounced {
		t.Fatalf("mode before Close = %q", got)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if got := s.CheckpointStats().Mode; got != CheckpointModeClosed {
		t.Fatalf("mode after Close = %q, want %q", got, CheckpointModeClosed)
	}
}

// A background commit already in flight that ignores its cancellation (as
// DOLT_COMMIT may) does not hold Close past the close deadline, and Close
// does not tear the engine down under it: it fails closed with
// ErrCheckpointCommitLive, leaves the pools and the connector open, and
// Reopen refuses. Once the commit finishes, Close again closes the store.
// Neither the cancel nor the skip counts as a failure.
func TestCheckpointCloseIsBoundedBehindABackgroundCommit(t *testing.T) {
	path := testStorePath(t)
	cleanupTestStorePath(path)
	t.Cleanup(func() { cleanupTestStorePath(path) })
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s.ConfigureCheckpoint(CheckpointConfig{Debounced: true, Debounce: time.Hour, MaxOps: 1 << 30})
	s.checkpoint.closeTimeout = 100 * time.Millisecond
	const computerID = "computer-cadence-closebehind"
	_, _ = prepareGenesis(t, s, computerID, "genesis-cadence-closebehind")
	finalizeTestEvents(t, s, computerID, 1)

	held, releaseCommit, background := holdBackgroundCommit(t, s)
	<-held
	s.markDoltHistoryDirty()
	started := time.Now()
	closed := make(chan error, 1)
	go func() { closed <- s.Close() }()
	var closeErr error
	select {
	case closeErr = <-closed:
	case <-time.After(3 * time.Second):
		releaseCommit()
		<-closed
		t.Fatalf("Close waited %s behind the background commit with a 100ms close cap", time.Since(started))
	}
	if !errors.Is(closeErr, ErrCheckpointCommitLive) {
		t.Fatalf("Close with a live commit = %v, want ErrCheckpointCommitLive", closeErr)
	}
	pingCtx, cancelPing := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelPing()
	if err := s.readDB.PingContext(pingCtx); err != nil {
		t.Fatalf("Close closed the pools or the connector under a live commit: %v", err)
	}
	if err := s.Reopen(path); !errors.Is(err, ErrCheckpointCommitLive) {
		t.Fatalf("Reopen after a Close that left the store open = %v, want ErrCheckpointCommitLive", err)
	}
	if st := s.CheckpointStats(); st.ClosingCommitsSkipped < 1 || st.ClosesLeftOpen < 1 {
		t.Fatalf("the skipped closing commit and the open store were not counted: %+v", st)
	}

	releaseCommit()
	select {
	case <-background:
	case <-time.After(10 * time.Second):
		t.Fatal("the background commit did not finish after release")
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close after the commit finished: %v", err)
	}
	if err := s.readDB.PingContext(pingCtx); err == nil {
		t.Fatal("the second Close left the pools open")
	}
	st := s.CheckpointStats()
	if st.Failures != 0 || st.LastError != "" || st.CancelledAtClose < 1 {
		t.Fatalf("a commit cancelled by Close must count as cancelled, not a failure: %+v", st)
	}
}

// holdBackgroundCommit starts a background commit whose DOLT_COMMIT stand-in
// holds the connection and ignores its context until released.
func holdBackgroundCommit(t *testing.T, s *Store) (held <-chan struct{}, release func(), done <-chan struct{}) {
	t.Helper()
	heldCh := make(chan struct{})
	releaseCh := make(chan struct{})
	var once sync.Once
	release = func() { once.Do(func() { close(releaseCh) }) }
	t.Cleanup(release)
	var calls sync.Mutex
	first := true
	s.checkpointExecHook = func(context.Context) error {
		calls.Lock()
		isFirst := first
		first = false
		calls.Unlock()
		if isFirst {
			close(heldCh)
			select {
			case <-releaseCh:
			case <-time.After(20 * time.Second):
			}
		}
		return nil
	}
	doneCh := make(chan struct{})
	go func() {
		s.fireCheckpoint()
		close(doneCh)
	}()
	select {
	case <-heldCh:
	case <-time.After(10 * time.Second):
		t.Fatal("the background commit never started")
	}
	return heldCh, release, doneCh
}

func doltHead(t *testing.T, s *Store) string {
	t.Helper()
	var head string
	if err := s.db.QueryRowContext(context.Background(), `SELECT commit_hash FROM dolt_log LIMIT 1`).Scan(&head); err != nil {
		t.Fatalf("read the Dolt head: %v", err)
	}
	return head
}

// While a commit Close left running is held, the store stays open: a
// second open in this process is refused by Dolt's lock (the engine was
// not torn down). Once the commit is released, Close succeeds and its
// closing commit is HEAD. What this shows about the late commit is
// narrow: the stand-in returns before DOLT_COMMIT and its context is
// cancelled, so it never reaches Dolt; a real DOLT_COMMIT that ignores
// its context mid-write runs to its end and may advance HEAD.
func TestCheckpointNextOpenWhileCommitterHeld(t *testing.T) {
	path := testStorePath(t)
	cleanupTestStorePath(path)
	t.Cleanup(func() { cleanupTestStorePath(path) })
	s, preClose := openWithPendingCheckpoint(t, path, "computer-cadence-nextopen")

	_, releaseCommit, background := holdBackgroundCommit(t, s)
	if err := s.Close(); !errors.Is(err, ErrCheckpointCommitLive) {
		t.Fatalf("Close with a live commit = %v, want ErrCheckpointCommitLive", err)
	}
	if next, err := Open(path); err == nil {
		_ = next.Close()
		t.Fatal("a second open in this process succeeded: Close released the engine under a live commit")
	} else if !strings.Contains(err.Error(), "locked") {
		t.Fatalf("a second open in this process: %v, want Dolt's lock refusal", err)
	}

	releaseCommit()
	select {
	case <-background:
	case <-time.After(10 * time.Second):
		t.Fatal("the background commit did not finish after release")
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close after the commit finished: %v", err)
	}
	after, err := Open(path)
	if err != nil {
		t.Fatalf("open after the store closed: %v", err)
	}
	defer after.Close()
	if got := doltHead(t, after); got == preClose {
		t.Fatalf("the retried Close did not commit the pending rows: head is still the pre-close head %s", preClose)
	}
	if msg := doltHeadMessage(t, after); msg != "guest snapshot at close" {
		t.Fatalf("HEAD after the retried Close = %q, want the closing commit", msg)
	}
}

func doltHeadMessage(t *testing.T, s *Store) string {
	t.Helper()
	var msg string
	if err := s.db.QueryRowContext(context.Background(), `SELECT message FROM dolt_log LIMIT 1`).Scan(&msg); err != nil {
		t.Fatalf("read the Dolt head message: %v", err)
	}
	return msg
}

// openWithPendingCheckpoint opens a debounced store with a 100ms close cap,
// commits one event (the pre-close head) and finalizes another, uncommitted.
func openWithPendingCheckpoint(t *testing.T, path, computerID string) (*Store, string) {
	t.Helper()
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s.ConfigureCheckpoint(CheckpointConfig{Debounced: true, Debounce: time.Hour, MaxOps: 1 << 30})
	s.checkpoint.closeTimeout = 100 * time.Millisecond
	_, _ = prepareGenesis(t, s, computerID, "genesis-"+computerID)
	finalizeTestEvents(t, s, computerID, 1)
	s.markDoltHistoryDirty()
	if err := s.commitDoltCheckpoint(context.Background(), "pre-close"); err != nil {
		t.Fatal(err)
	}
	preClose := doltHead(t, s)
	finalizeTestEvents(t, s, computerID, 1)
	return s, preClose
}

const failedCloseChildEnv = "CHOIR_TEST_FAILED_CLOSE_CHILD"

// The restart after a Close that failed closed: a child process holds a
// background commit, Close fails closed, and the process exits with the
// commit still held (a planned restart's exit). The next process opens with
// no lock error and sees the pre-close head. This proves that process exit
// reclaims the store; the held stand-in never reached Dolt, so it says
// nothing about a DOLT_COMMIT already inside Dolt at the exit.
func TestCheckpointNextProcessOpenAfterFailedClose(t *testing.T) {
	if path := os.Getenv(failedCloseChildEnv); path != "" {
		s, preClose := openWithPendingCheckpoint(t, path, "computer-cadence-nextprocess")
		holdBackgroundCommit(t, s)
		if err := s.Close(); !errors.Is(err, ErrCheckpointCommitLive) {
			fmt.Printf("CHILD-ERROR close = %v\n", err)
			os.Exit(4)
		}
		fmt.Printf("PRECLOSE %s\n", preClose)
		os.Exit(3) // exit with the commit held; 0 is refused inside a test
	}
	path := testStorePath(t)
	cleanupTestStorePath(path)
	t.Cleanup(func() { cleanupTestStorePath(path) })
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCheckpointNextProcessOpenAfterFailedClose$", "-test.count=1")
	cmd.Env = append(os.Environ(), failedCloseChildEnv+"="+path)
	out, err := cmd.CombinedOutput()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 3 {
		t.Fatalf("child: %v\n%s", err, out)
	}
	preClose := ""
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		if rest, ok := strings.CutPrefix(sc.Text(), "PRECLOSE "); ok {
			preClose = strings.TrimSpace(rest)
		}
	}
	if preClose == "" {
		t.Fatalf("the child did not report its pre-close head:\n%s", out)
	}
	next, err := Open(path)
	if err != nil {
		t.Fatalf("the next process's open: %v", err)
	}
	defer next.Close()
	if got := doltHead(t, next); got != preClose {
		t.Fatalf("the next process's head = %s, want the pre-close head %s", got, preClose)
	}
}

// A planned close that cancels a background commit honoring its context
// commits once and counts the cancel apart from failures.
func TestCheckpointPlannedCloseCountsNoFailure(t *testing.T) {
	path := testStorePath(t)
	cleanupTestStorePath(path)
	t.Cleanup(func() { cleanupTestStorePath(path) })
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s.ConfigureCheckpoint(CheckpointConfig{Debounced: true, Debounce: time.Hour, MaxOps: 1 << 30})
	const computerID = "computer-cadence-plannedclose"
	_, _ = prepareGenesis(t, s, computerID, "genesis-cadence-plannedclose")
	finalizeTestEvents(t, s, computerID, 1)
	held := make(chan struct{})
	var calls sync.Mutex
	first := true
	s.checkpointExecHook = func(ctx context.Context) error {
		calls.Lock()
		isFirst := first
		first = false
		calls.Unlock()
		if !isFirst {
			return nil
		}
		close(held)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(20 * time.Second):
			return errors.New("never cancelled")
		}
	}
	background := make(chan struct{})
	go func() {
		s.fireCheckpoint()
		close(background)
	}()
	select {
	case <-held:
	case <-time.After(10 * time.Second):
		t.Fatal("the background commit never started")
	}
	if err := s.Close(); err != nil {
		t.Fatalf("planned Close: %v", err)
	}
	<-background
	st := s.CheckpointStats()
	if st.Failures != 0 || st.LastError != "" || st.CancelledAtClose != 1 || st.ClosingCommitsSkipped != 0 {
		t.Fatalf("a planned close must count one cancel and no failure: %+v", st)
	}
}

// A background commit that reaches the commit lock after Close (it passed
// the closed check before Close ran) is not attempted.
func TestCheckpointCommitAfterCloseIsNotAttempted(t *testing.T) {
	path := testStorePath(t)
	cleanupTestStorePath(path)
	t.Cleanup(func() { cleanupTestStorePath(path) })
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s.ConfigureCheckpoint(CheckpointConfig{Debounced: true, Debounce: time.Hour, MaxOps: 1 << 30})
	const computerID = "computer-cadence-afterclose"
	_, _ = prepareGenesis(t, s, computerID, "genesis-cadence-afterclose")
	finalizeTestEvents(t, s, computerID, 1)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	attempted := false
	s.checkpointExecHook = func(context.Context) error {
		attempted = true
		return nil
	}
	s.markDoltHistoryDirty()
	if err := s.commitDoltCheckpointAt(context.Background(), "late", checkpointSiteBackground); err != nil {
		t.Fatalf("a background commit after Close: %v", err)
	}
	if attempted {
		t.Fatal("a background commit ran after Close")
	}
}

// prepareOneFinalize prepares one event on computerID and returns its
// finalize, to run while the test holds a commit.
func prepareOneFinalize(t *testing.T, s *Store, computerID string) func() error {
	t.Helper()
	ctx := context.Background()
	head, err := s.Head(ctx, computerID)
	if err != nil || head == nil {
		t.Fatalf("head: %v", err)
	}
	eventID, err := computerevent.NewEventID()
	if err != nil {
		t.Fatal(err)
	}
	event, digest := prepareProjectionEvent(t, s, *head, eventID, "held-"+eventID)
	batch := testBatchCheckpointBatch(t, computerID, event.EventID, digest)
	receipt := signedReceipt(t, computerID, digest, event.Sequence)
	return func() error { return s.FinalizeBatch(ctx, computerID, digest, receipt, &batch) }
}

// holdNextCommit makes the next commit hold the connection, ignoring its
// context, until released.
func holdNextCommit(t *testing.T, s *Store) (held <-chan struct{}, release func()) {
	t.Helper()
	heldCh := make(chan struct{})
	releaseCh := make(chan struct{})
	var once sync.Once
	release = func() { once.Do(func() { close(releaseCh) }) }
	t.Cleanup(release)
	var mu sync.Mutex
	armed := true
	s.checkpointExecHook = func(context.Context) error {
		mu.Lock()
		hold := armed
		armed = false
		mu.Unlock()
		if hold {
			close(heldCh)
			select {
			case <-releaseCh:
			case <-time.After(20 * time.Second):
			}
		}
		return nil
	}
	return heldCh, release
}

func readDBAnswers(t *testing.T, s *Store) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.readDB.PingContext(ctx)
}

// A second Close while the first waits on a live commit does not close
// the pools: it waits for the first and fails closed too.
func TestCheckpointConcurrentCloseFailsClosed(t *testing.T) {
	path := testStorePath(t)
	cleanupTestStorePath(path)
	t.Cleanup(func() { cleanupTestStorePath(path) })
	s, _ := openWithPendingCheckpoint(t, path, "computer-cadence-concurrentclose")
	s.checkpoint.closeTimeout = 300 * time.Millisecond
	_, releaseCommit, background := holdBackgroundCommit(t, s)

	first := make(chan error, 1)
	go func() { first <- s.Close() }()
	waitForCheckpoint(t, "the first Close to start", s.checkpoint.isClosed)
	second := make(chan error, 1)
	go func() { second <- s.Close() }()
	for name, ch := range map[string]chan error{"first": first, "second": second} {
		select {
		case err := <-ch:
			if !errors.Is(err, ErrCheckpointCommitLive) {
				t.Errorf("%s Close with a live commit = %v, want ErrCheckpointCommitLive", name, err)
			}
		case <-time.After(10 * time.Second):
			releaseCommit()
			t.Fatalf("%s Close did not return", name)
		}
	}
	if err := readDBAnswers(t, s); err != nil {
		t.Fatalf("a Close closed the pools under a live commit: %v", err)
	}
	if got := s.CheckpointStats().ClosesLeftOpen; got != 2 {
		t.Fatalf("closes_left_open = %d, want 2 (one per failing Close)", got)
	}
	releaseCommit()
	select {
	case <-background:
	case <-time.After(10 * time.Second):
		t.Fatal("the background commit did not finish after release")
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close after the commit finished: %v", err)
	}
}

// After a Close that failed closed the store stays usable; a retried
// Close while an inline finalize commit runs fails closed too.
func TestCheckpointRetriedCloseUnderInlineCommitFailsClosed(t *testing.T) {
	path := testStorePath(t)
	cleanupTestStorePath(path)
	t.Cleanup(func() { cleanupTestStorePath(path) })
	const computerID = "computer-cadence-retriedclose"
	s, _ := openWithPendingCheckpoint(t, path, computerID)
	s.doltCheckpointEvery = 1
	_, releaseBackground, background := holdBackgroundCommit(t, s)
	if err := s.Close(); !errors.Is(err, ErrCheckpointCommitLive) {
		t.Fatalf("first Close = %v, want ErrCheckpointCommitLive", err)
	}
	releaseBackground()
	select {
	case <-background:
	case <-time.After(10 * time.Second):
		t.Fatal("the background commit did not finish after release")
	}

	// A failed Close restores the debounced cadence, so take the store to
	// count mode for an inline commit to retry Close under.
	s.ConfigureCheckpoint(CheckpointConfig{})
	finalize := prepareOneFinalize(t, s, computerID)
	held, releaseInline := holdNextCommit(t, s)
	finalized := make(chan error, 1)
	go func() { finalized <- finalize() }()
	select {
	case <-held:
	case <-time.After(10 * time.Second):
		t.Fatal("the inline commit never started")
	}
	if err := s.Close(); !errors.Is(err, ErrCheckpointCommitLive) {
		releaseInline()
		t.Fatalf("retried Close under an inline commit = %v, want ErrCheckpointCommitLive", err)
	}
	if err := readDBAnswers(t, s); err != nil {
		t.Fatalf("the retried Close closed the pools under the inline commit: %v", err)
	}
	releaseInline()
	select {
	case err := <-finalized:
		if err != nil {
			t.Fatalf("the inline commit failed: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the finalize did not finish after release")
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close after the inline commit finished: %v", err)
	}
}

// Count mode (the default) fences Close the same way: a live inline
// commit keeps the pools open and Close fails closed.
func TestCheckpointCountModeCloseFailsClosedUnderInlineCommit(t *testing.T) {
	path := testStorePath(t)
	cleanupTestStorePath(path)
	t.Cleanup(func() { cleanupTestStorePath(path) })
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s.doltCheckpointEvery = 1
	s.checkpoint.closeTimeout = 100 * time.Millisecond
	const computerID = "computer-cadence-countclose"
	_, _ = prepareGenesis(t, s, computerID, "genesis-cadence-countclose")
	finalize := prepareOneFinalize(t, s, computerID)
	held, releaseInline := holdNextCommit(t, s)
	finalized := make(chan error, 1)
	go func() { finalized <- finalize() }()
	select {
	case <-held:
	case <-time.After(10 * time.Second):
		t.Fatal("the inline commit never started")
	}
	if err := s.Close(); !errors.Is(err, ErrCheckpointCommitLive) {
		releaseInline()
		t.Fatalf("count-mode Close under an inline commit = %v, want ErrCheckpointCommitLive", err)
	}
	if err := readDBAnswers(t, s); err != nil {
		t.Fatalf("count-mode Close closed the pools under the inline commit: %v", err)
	}
	releaseInline()
	select {
	case err := <-finalized:
		if err != nil {
			t.Fatalf("the inline commit failed: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the finalize did not finish after release")
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close after the inline commit finished: %v", err)
	}
}

// One deadline bounds the whole Close: the close deadline plus one grace,
// not a grace per wait. The background commit ignores its cancel and
// returns inside the grace; the closing commit then has no time left and
// is skipped (counted once), and Close returns within the bound.
func TestCheckpointCloseBoundIsOneDeadline(t *testing.T) {
	path := testStorePath(t)
	cleanupTestStorePath(path)
	t.Cleanup(func() { cleanupTestStorePath(path) })
	s, _ := openWithPendingCheckpoint(t, path, "computer-cadence-onedeadline")
	const limit = 400 * time.Millisecond
	s.checkpoint.closeTimeout = limit
	var mu sync.Mutex
	call := 0
	var closeStarted time.Time
	held := make(chan struct{})
	s.checkpointExecHook = func(ctx context.Context) error {
		mu.Lock()
		call++
		n := call
		mu.Unlock()
		if n != 1 {
			return nil
		}
		close(held)
		for giveUp := time.Now().Add(20 * time.Second); time.Now().Before(giveUp); time.Sleep(5 * time.Millisecond) {
			mu.Lock()
			started := closeStarted
			mu.Unlock()
			if !started.IsZero() && time.Since(started) >= limit+150*time.Millisecond {
				return nil
			}
		}
		return nil
	}
	background := make(chan struct{})
	go func() {
		s.fireCheckpoint()
		close(background)
	}()
	select {
	case <-held:
	case <-time.After(10 * time.Second):
		t.Fatal("the background commit never started")
	}
	mu.Lock()
	closeStarted = time.Now()
	mu.Unlock()
	err := s.Close()
	elapsed := time.Since(closeStarted)
	if err != nil {
		t.Fatalf("Close after the background commit returned inside the grace: %v", err)
	}
	if bound := limit + checkpointCloseGrace + 75*time.Millisecond; elapsed > bound {
		t.Fatalf("Close took %s, past one close deadline plus one grace (%s)", elapsed, bound)
	}
	if st := s.CheckpointStats(); st.ClosingCommitsSkipped != 1 || st.Failures != 0 {
		t.Fatalf("the closing commit with no time left must count once as skipped: %+v", st)
	}
	select {
	case <-background:
	case <-time.After(10 * time.Second):
		t.Fatal("the background commit did not finish")
	}
}

// Reopen requires a closed store: on an open one it would orphan the open
// handles.
func TestCheckpointReopenRequiresAClosedStore(t *testing.T) {
	path := testStorePath(t)
	cleanupTestStorePath(path)
	t.Cleanup(func() { cleanupTestStorePath(path) })
	other := path + "-other"
	cleanupTestStorePath(other)
	t.Cleanup(func() { cleanupTestStorePath(other) })
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := s.Reopen(other); err == nil {
		t.Fatal("Reopen on an open store succeeded")
	}
	if err := readDBAnswers(t, s); err != nil {
		t.Fatalf("a refused Reopen disturbed the open store: %v", err)
	}
}

func commitLockFree(s *Store) bool {
	if s.doltCommitMu.TryLock() {
		s.doltCommitMu.Unlock()
		return true
	}
	return false
}

// A closing commit that overruns the deadline and later returns with its
// context error counts once as skipped, and the failing Close once as
// left open.
func TestCheckpointOverrunClosingCommitCountsOnce(t *testing.T) {
	path := testStorePath(t)
	cleanupTestStorePath(path)
	t.Cleanup(func() { cleanupTestStorePath(path) })
	s, _ := openWithPendingCheckpoint(t, path, "computer-cadence-overrun")
	s.markDoltHistoryDirty()
	held, release := holdNextCommit(t, s)
	closed := make(chan error, 1)
	go func() { closed <- s.Close() }()
	select {
	case <-held:
	case <-time.After(10 * time.Second):
		t.Fatal("the closing commit never started")
	}
	select {
	case err := <-closed:
		if !errors.Is(err, ErrCheckpointCommitLive) {
			t.Fatalf("Close with an overrunning closing commit = %v, want ErrCheckpointCommitLive", err)
		}
	case <-time.After(10 * time.Second):
		release()
		t.Fatal("Close did not return")
	}
	release()
	waitForCheckpoint(t, "the overrunning closing commit to return", func() bool { return commitLockFree(s) })
	st := s.CheckpointStats()
	if st.ClosingCommitsSkipped != 1 || st.ClosesLeftOpen != 1 || st.Failures != 0 {
		t.Fatalf("one overrunning closing commit must count once: %+v", st)
	}
}

// A Close that fails closed leaves the store as it was before the Close:
// the debounced cadence (not "closed", no inline commits) and the
// telemetry sealer accepting again.
func TestCheckpointFailedCloseRestoresCadenceAndTelemetry(t *testing.T) {
	path := testStorePath(t)
	cleanupTestStorePath(path)
	t.Cleanup(func() { cleanupTestStorePath(path) })
	const computerID = "computer-cadence-failedrestore"
	s, _ := openWithPendingCheckpoint(t, path, computerID)
	s.doltCheckpointEvery = 1
	s.telemetry.Store(&telemetrySealer{s: s, window: time.Second, enabled: true})
	if !s.TelemetryBatchingEnabled() {
		t.Fatal("the test sealer is not accepting")
	}
	_, releaseCommit, background := holdBackgroundCommit(t, s)
	if err := s.Close(); !errors.Is(err, ErrCheckpointCommitLive) {
		t.Fatalf("Close with a live commit = %v, want ErrCheckpointCommitLive", err)
	}
	if got := s.CheckpointStats().Mode; got != CheckpointModeDebounced {
		t.Fatalf("mode after a failed Close = %q, want %q", got, CheckpointModeDebounced)
	}
	if !s.TelemetryBatchingEnabled() {
		t.Fatal("a failed Close left the telemetry sealer stopped")
	}
	releaseCommit()
	select {
	case <-background:
	case <-time.After(10 * time.Second):
		t.Fatal("the background commit did not finish after release")
	}
	before := s.CheckpointStats().InlineCommits
	finalizeTestEvents(t, s, computerID, 2)
	if got := s.CheckpointStats().InlineCommits; got != before {
		t.Fatalf("finalize committed inline %d times after a failed Close in debounced mode", got-before)
	}
	if !s.checkpointTimerArmed() {
		t.Fatal("the debounce timer is not armed after a failed Close and new writes")
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close after the commit finished: %v", err)
	}
}

// runHeld runs fn in a goroutine and waits until pool reports a connection
// in use, so Close starts while fn holds it.
func runHeld(t *testing.T, pool interface{ Stats() sql.DBStats }, fn func() error) <-chan error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- fn() }()
	waitForCheckpoint(t, "the statement to hold the pool's connection", func() bool { return pool.Stats().InUse > 0 })
	return done
}

func waitDone(t *testing.T, what string, ch <-chan error) error {
	t.Helper()
	select {
	case err := <-ch:
		return err
	case <-time.After(30 * time.Second):
		t.Fatalf("%s did not finish", what)
		return nil
	}
}

func openForPinTest(t *testing.T) (*Store, string) {
	t.Helper()
	path := testStorePath(t)
	cleanupTestStorePath(path)
	t.Cleanup(func() { cleanupTestStorePath(path) })
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s.checkpoint.closeTimeout = 300 * time.Millisecond
	return s, path
}

// Sonnet's round-5 probe: a statement in flight on s.db that is not a
// commit holds the pool's one connection; Close fails closed instead of
// closing the engine under it, and the statement completes.
func TestCloseFailsClosedUnderAStatementInFlight(t *testing.T) {
	s, _ := openForPinTest(t)
	sleeping := runHeld(t, s.db, func() error {
		_, err := s.db.ExecContext(context.Background(), "SELECT SLEEP(2)")
		return err
	})
	if err := closeWithin(t, s, 5*time.Second); !errors.Is(err, ErrPoolStatementLive) {
		t.Fatalf("Close under a statement in flight = %v, want ErrPoolStatementLive", err)
	}
	if err := waitDone(t, "the statement", sleeping); err != nil {
		t.Fatalf("the statement in flight failed: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close after the statement finished: %v", err)
	}
}

// The same on the read pool.
func TestCloseFailsClosedUnderAReadInFlight(t *testing.T) {
	s, _ := openForPinTest(t)
	reading := runHeld(t, s.readDB, func() error {
		_, err := s.readDB.ExecContext(context.Background(), "SELECT SLEEP(2)")
		return err
	})
	if err := closeWithin(t, s, 5*time.Second); !errors.Is(err, ErrPoolStatementLive) {
		t.Fatalf("Close under a read in flight = %v, want ErrPoolStatementLive", err)
	}
	if err := waitDone(t, "the read", reading); err != nil {
		t.Fatalf("the read in flight failed: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close after the read finished: %v", err)
	}
}

// Sonnet's round-5 probe: an open write transaction holds the connection;
// Close fails closed, the transaction commits, and its rows are durable.
func TestCloseFailsClosedUnderAnOpenWriteTransaction(t *testing.T) {
	ctx := context.Background()
	s, path := openForPinTest(t)
	if _, err := s.db.ExecContext(ctx, "CREATE TABLE close_pin_probe (id INT PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 3; i++ {
		if _, err := tx.ExecContext(ctx, "INSERT INTO close_pin_probe VALUES (?)", i); err != nil {
			t.Fatal(err)
		}
	}
	if err := closeWithin(t, s, 5*time.Second); !errors.Is(err, ErrPoolStatementLive) {
		_ = tx.Rollback()
		t.Fatalf("Close under an open write transaction = %v, want ErrPoolStatementLive", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("the open transaction could not commit after the failed Close: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close after the transaction committed: %v", err)
	}
	after, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer after.Close()
	var rows int
	if err := after.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM close_pin_probe").Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 3 {
		t.Fatalf("rows after reopen = %d, want 3", rows)
	}
}

// Round-6 review F4: a Close that fails at the commit-lock fence (count
// mode, an inline commit holding the lock) restores the cadence and the
// telemetry sealer.
func TestCloseFailedAtTheFenceRestoresCadence(t *testing.T) {
	s, _ := openForPinTest(t)
	s.doltCheckpointEvery = 1
	s.telemetry.Store(&telemetrySealer{s: s, window: time.Second, enabled: true})
	const computerID = "computer-cadence-fencerestore"
	_, _ = prepareGenesis(t, s, computerID, "genesis-cadence-fencerestore")
	finalize := prepareOneFinalize(t, s, computerID)
	held, release := holdNextCommit(t, s)
	finalized := make(chan error, 1)
	go func() { finalized <- finalize() }()
	select {
	case <-held:
	case <-time.After(10 * time.Second):
		t.Fatal("the inline commit never started")
	}
	if err := s.Close(); !errors.Is(err, ErrCheckpointCommitLive) {
		release()
		t.Fatalf("Close at the fence = %v, want ErrCheckpointCommitLive", err)
	}
	if s.checkpoint.isClosed() || s.CheckpointStats().Mode != CheckpointModeCount {
		release()
		t.Fatalf("a Close failed at the fence left the cadence closed: %+v", s.CheckpointStats())
	}
	if !s.TelemetryBatchingEnabled() {
		release()
		t.Fatal("a Close failed at the fence left the telemetry sealer stopped")
	}
	release()
	if err := waitDone(t, "the finalize", finalized); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close after the inline commit finished: %v", err)
	}
}

// A Close that fails while pinning the pools (a statement in flight) in
// debounced mode restores the debounced cadence too.
func TestCloseFailedAtThePinRestoresCadence(t *testing.T) {
	s, _ := openForPinTest(t)
	s.ConfigureCheckpoint(CheckpointConfig{Debounced: true, Debounce: time.Hour, MaxOps: 1 << 30})
	sleeping := runHeld(t, s.db, func() error {
		_, err := s.db.ExecContext(context.Background(), "SELECT SLEEP(2)")
		return err
	})
	if err := closeWithin(t, s, 5*time.Second); !errors.Is(err, ErrPoolStatementLive) {
		t.Fatalf("Close under a statement in flight = %v, want ErrPoolStatementLive", err)
	}
	if got := s.CheckpointStats().Mode; got != CheckpointModeDebounced {
		t.Fatalf("mode after a Close failed at the pin = %q, want %q", got, CheckpointModeDebounced)
	}
	if err := waitDone(t, "the statement", sleeping); err != nil {
		t.Fatalf("the statement in flight failed: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close after the statement finished: %v", err)
	}
}

// runStoreChild runs testName in a child process of this test binary with
// env set, and returns its exit code and output. Dolt's crash on a fatal
// I/O error panics in a new goroutine, which no recover can catch, so a
// test that must show "no panic" runs the risky part in a child.
func runStoreChild(t *testing.T, testName string, env ...string) (int, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^"+testName+"$", "-test.count=1")
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		return 0, string(out)
	case errors.As(err, &exitErr):
		return exitErr.ExitCode(), string(out)
	default:
		t.Fatalf("child %s: %v\n%s", testName, err, out)
		return -1, ""
	}
}

const removedWorkspaceChildEnv = "CHOIR_TEST_REMOVED_WORKSPACE_CHILD"

// The mirror's arch13j panic (2026-10-22): the workspace was removed under
// an open engine, and closing the engine made Dolt crash the process on its
// journal flush. Close must return an error instead and leave the engine
// unclosed.
func TestCloseAfterTheWorkspaceIsRemovedReturnsAnError(t *testing.T) {
	if path := os.Getenv(removedWorkspaceChildEnv); path != "" {
		s, err := Open(path)
		if err != nil {
			fmt.Printf("CHILD-ERROR open = %v\n", err)
			os.Exit(4)
		}
		if err := os.RemoveAll(deriveTextureWorkspacePath(path)); err != nil {
			fmt.Printf("CHILD-ERROR remove = %v\n", err)
			os.Exit(4)
		}
		fmt.Printf("CLOSE-RESULT %v\n", s.Close())
		os.Exit(3)
	}
	path := testStorePath(t)
	cleanupTestStorePath(path)
	t.Cleanup(func() { cleanupTestStorePath(path) })
	code, out := runStoreChild(t, "TestCloseAfterTheWorkspaceIsRemovedReturnsAnError", removedWorkspaceChildEnv+"="+path)
	if code != 3 {
		t.Fatalf("the child did not return from Close (exit %d):\n%s", code, out)
	}
	if !strings.Contains(out, "CLOSE-RESULT ") || strings.Contains(out, "CLOSE-RESULT <nil>") {
		t.Fatalf("Close after the workspace was removed must return an error:\n%s", out)
	}
}

const openFailsChildEnv = "CHOIR_TEST_OPEN_FAILS_CHILD"

// Open that fails after the engine is up (the workspace removed right
// after it opens, as in the mirror) returns its error; its Close neither
// panics nor closes the engine over the missing files.
func TestOpenFailingAfterTheEngineIsUpReturnsAnError(t *testing.T) {
	if path := os.Getenv(openFailsChildEnv); path != "" {
		testHookAfterWorkspaceOpen = func(s *Store) error { return os.RemoveAll(s.texturePath) }
		s, err := Open(path)
		if err == nil {
			_ = s
			fmt.Printf("CHILD-ERROR open succeeded over a removed workspace\n")
			os.Exit(4)
		}
		fmt.Printf("OPEN-ERROR %v\n", err)
		os.Exit(3)
	}
	path := testStorePath(t)
	cleanupTestStorePath(path)
	t.Cleanup(func() { cleanupTestStorePath(path) })
	code, out := runStoreChild(t, "TestOpenFailingAfterTheEngineIsUpReturnsAnError", openFailsChildEnv+"="+path)
	if code != 3 || !strings.Contains(out, "OPEN-ERROR ") {
		t.Fatalf("Open over a workspace removed after the engine came up must return an error, not crash (exit %d):\n%s", code, out)
	}
}

// closeWithin runs Close and fails the test if it has not returned within
// d: an unbounded wait in Close fails fast instead of at go test's timeout.
func closeWithin(t *testing.T, s *Store, d time.Duration) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- s.Close() }()
	select {
	case err := <-done:
		return err
	case <-time.After(d):
		t.Fatalf("Close did not return within %s: an unbounded wait", d)
		return nil
	}
}

// closeOrderRecorder stands in for the Dolt connector and records, at the
// first engine close, how many pool connections were still open.
type closeOrderRecorder struct {
	doltConnector
	mu               sync.Mutex
	pools            func() []*sql.DB
	closes           int
	openAtFirstClose int
}

func (r *closeOrderRecorder) Close() error {
	r.mu.Lock()
	r.closes++
	if r.closes == 1 && r.pools != nil {
		for _, pool := range r.pools() {
			if pool != nil {
				r.openAtFirstClose += pool.Stats().OpenConnections
			}
		}
	}
	r.mu.Unlock()
	return r.doltConnector.Close()
}

func (r *closeOrderRecorder) snapshot() (closes, openAtFirstClose int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.closes, r.openAtFirstClose
}

// recordEngineCloses wraps every connector the next opens create.
func recordEngineCloses(t *testing.T) *closeOrderRecorder {
	t.Helper()
	rec := &closeOrderRecorder{}
	testHookWrapConnector = func(c doltConnector) doltConnector {
		rec.doltConnector = c
		return rec
	}
	t.Cleanup(func() { testHookWrapConnector = nil })
	return rec
}

func assertEngineClosedOnceLast(t *testing.T, s *Store, rec *closeOrderRecorder) {
	t.Helper()
	rec.mu.Lock()
	rec.pools = func() []*sql.DB { return []*sql.DB{s.textureDB, s.readDB, s.db} }
	rec.mu.Unlock()
	for _, pool := range []*sql.DB{s.textureDB, s.readDB, s.db} {
		if pool != nil {
			if err := pool.PingContext(context.Background()); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := closeWithin(t, s, 30*time.Second); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := closeWithin(t, s, 30*time.Second); err != nil {
		t.Fatalf("a second Close: %v", err)
	}
	closes, open := rec.snapshot()
	if closes != 1 {
		t.Fatalf("the engine was closed %d times, want exactly once", closes)
	}
	if open != 0 {
		t.Fatalf("%d pool connections were still open when the engine closed, want 0", open)
	}
}

// Sonnet r7 F1: the engine closes exactly once, last, after every pool
// connection (the pinned ones included) is closed. Kills the mutants that
// drop the connector wrapper, close the engine before releasing the pins,
// or never release the pins.
func TestCloseClosesTheEngineOnceAfterEveryConnection(t *testing.T) {
	rec := recordEngineCloses(t)
	path := testStorePath(t)
	cleanupTestStorePath(path)
	t.Cleanup(func() { cleanupTestStorePath(path) })
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	assertEngineClosedOnceLast(t, s, rec)
}

// The same for a texture-only store (OpenTextureWorkspace).
func TestTextureStoreClosesTheEngineOnceAfterEveryConnection(t *testing.T) {
	rec := recordEngineCloses(t)
	path := testStorePath(t)
	cleanupTestStorePath(path)
	t.Cleanup(func() { cleanupTestStorePath(path) })
	s, err := OpenTextureWorkspace(path)
	if err != nil {
		t.Fatal(err)
	}
	assertEngineClosedOnceLast(t, s, rec)
}

// A texture-only store pins its pool too: a statement in flight on it
// makes Close fail closed. Kills the mutant that drops the texture pin.
func TestTextureStoreCloseFailsClosedUnderAStatementInFlight(t *testing.T) {
	path := testStorePath(t)
	cleanupTestStorePath(path)
	t.Cleanup(func() { cleanupTestStorePath(path) })
	s, err := OpenTextureWorkspace(path)
	if err != nil {
		t.Fatal(err)
	}
	s.checkpoint.closeTimeout = 300 * time.Millisecond
	sleeping := runHeld(t, s.textureDB, func() error {
		_, err := s.textureDB.ExecContext(context.Background(), "SELECT SLEEP(2)")
		return err
	})
	if err := closeWithin(t, s, 5*time.Second); !errors.Is(err, ErrPoolStatementLive) {
		t.Fatalf("texture-store Close under a statement in flight = %v, want ErrPoolStatementLive", err)
	}
	if err := waitDone(t, "the statement", sleeping); err != nil {
		t.Fatalf("the statement in flight failed: %v", err)
	}
	if err := closeWithin(t, s, 30*time.Second); err != nil {
		t.Fatalf("Close after the statement finished: %v", err)
	}
}

// The pin shares the one hard deadline: when the closing commit has spent
// the close deadline waiting for a busy connection, the pin gets only the
// grace, so Close returns by the deadline plus the grace. Kills the mutant
// that gives the pin a fresh deadline (about double the worst case).
func TestClosePinSharesTheHardDeadline(t *testing.T) {
	path := testStorePath(t)
	cleanupTestStorePath(path)
	t.Cleanup(func() { cleanupTestStorePath(path) })
	s, _ := openWithPendingCheckpoint(t, path, "computer-cadence-pindeadline")
	const limit = 300 * time.Millisecond
	s.checkpoint.closeTimeout = limit
	sleeping := runHeld(t, s.db, func() error {
		_, err := s.db.ExecContext(context.Background(), "SELECT SLEEP(3)")
		return err
	})
	started := time.Now()
	err := closeWithin(t, s, 10*time.Second)
	elapsed := time.Since(started)
	if !errors.Is(err, ErrPoolStatementLive) {
		t.Fatalf("Close under a statement in flight = %v, want ErrPoolStatementLive", err)
	}
	if bound := limit + checkpointCloseGrace + 150*time.Millisecond; elapsed > bound {
		t.Fatalf("Close took %s, past one close deadline plus one grace (%s): the pin did not share the deadline", elapsed, bound)
	}
	if err := waitDone(t, "the statement", sleeping); err != nil {
		t.Fatalf("the statement in flight failed: %v", err)
	}
	if err := closeWithin(t, s, 30*time.Second); err != nil {
		t.Fatalf("Close after the statement finished: %v", err)
	}
}

// Sonnet r7 F3: a pin failure names a statement, not a commit, and both
// fail-closed errors are ErrCloseLeftOpen for callers.
func TestClosePinFailureIsNotACommitError(t *testing.T) {
	s, _ := openForPinTest(t)
	sleeping := runHeld(t, s.db, func() error {
		_, err := s.db.ExecContext(context.Background(), "SELECT SLEEP(2)")
		return err
	})
	err := closeWithin(t, s, 5*time.Second)
	if !errors.Is(err, ErrPoolStatementLive) || !errors.Is(err, ErrCloseLeftOpen) || errors.Is(err, ErrCheckpointCommitLive) {
		t.Fatalf("pin failure = %v, want ErrPoolStatementLive (an ErrCloseLeftOpen), not ErrCheckpointCommitLive", err)
	}
	if !errors.Is(ErrCheckpointCommitLive, ErrCloseLeftOpen) {
		t.Fatal("ErrCheckpointCommitLive is not an ErrCloseLeftOpen")
	}
	if err := waitDone(t, "the statement", sleeping); err != nil {
		t.Fatal(err)
	}
	if err := closeWithin(t, s, 30*time.Second); err != nil {
		t.Fatal(err)
	}
}

// DeepSeek r7 F4: a pin with the deadline already spent says so, rather
// than blaming a statement.
func TestPinPoolsWithASpentDeadlineNamesTheBudget(t *testing.T) {
	s := openProjectStore(t)
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	pinned, err := s.pinPools(ctx)
	if !errors.Is(err, errCloseBudgetSpent) || len(pinned) != 0 {
		t.Fatalf("pinPools with a spent deadline = %d pinned, %v; want errCloseBudgetSpent", len(pinned), err)
	}
}

// Sonnet r7 F4: an Open that fails after the engine is up closes its store
// and releases the workspace, even when a statement it started still holds
// a connection at its first Close: the next Open of the same path works,
// and Open's error is returned.
func TestOpenFailureReleasesTheWorkspace(t *testing.T) {
	path := testStorePath(t)
	cleanupTestStorePath(path)
	t.Cleanup(func() { cleanupTestStorePath(path) })
	injected := errors.New("injected open failure")
	var sleeping <-chan error
	testHookAfterWorkspaceOpen = func(s *Store) error {
		s.checkpoint.closeTimeout = 200 * time.Millisecond
		sleeping = runHeld(t, s.db, func() error {
			_, err := s.db.ExecContext(context.Background(), "SELECT SLEEP(1)")
			return err
		})
		return injected
	}
	t.Cleanup(func() { testHookAfterWorkspaceOpen = nil })
	if _, err := Open(path); !errors.Is(err, injected) {
		t.Fatalf("Open with an injected failure = %v, want it", err)
	}
	testHookAfterWorkspaceOpen = nil
	if sleeping != nil {
		_ = waitDone(t, "the statement", sleeping)
	}
	next, err := Open(path)
	if err != nil {
		t.Fatalf("the next Open after a failed Open: %v (the workspace was not released)", err)
	}
	if err := closeWithin(t, next, 30*time.Second); err != nil {
		t.Fatal(err)
	}
}
