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
	finalizeDone := make(chan error, 1)
	go func() {
		finalizeDone <- s.FinalizeBatch(ctx, computerID, digest, receipt, &batch)
	}()
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
	if min := uint64(hold / 2); stats.FinalizeCommitWaitMaxNs < min || stats.FinalizeCommitWaitNanos < min {
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
	if s.CheckpointStats().Failures < 1 {
		t.Fatal("the timed-out closing commit was not counted as a failure")
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

// A background commit already in flight (holding the commit lock, and
// ignoring its context, as DOLT_COMMIT may) does not hold Close past the
// close cap: Close skips the closing commit and counts the failure.
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

	held := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseCommit := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(releaseCommit)
	var calls sync.Mutex
	first := true
	s.checkpointExecHook = func(context.Context) error {
		calls.Lock()
		isFirst := first
		first = false
		calls.Unlock()
		if isFirst {
			close(held)
			select {
			case <-release:
			case <-time.After(10 * time.Second):
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
	s.markDoltHistoryDirty()
	failuresBefore := s.CheckpointStats().Failures
	started := time.Now()
	closed := make(chan error, 1)
	go func() { closed <- s.Close() }()
	select {
	case <-closed:
	case <-time.After(3 * time.Second):
		releaseCommit()
		<-closed
		t.Fatalf("Close waited %s behind the background commit with a 100ms close cap", time.Since(started))
	}
	if s.CheckpointStats().Failures <= failuresBefore {
		t.Fatal("the skipped closing commit was not counted as a failure")
	}
	releaseCommit()
	select {
	case <-background:
	case <-time.After(10 * time.Second):
		t.Fatal("the background commit did not finish after release")
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
