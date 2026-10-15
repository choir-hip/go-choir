package store

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Failure modes pinned here, written before the code
// (docs/problems/guest-dolt-history-bound-2026-10-15.md):
//
//	FH1  History grows without bound between offline compactions.
//	FH2  Compaction changes projected state: the projection heads, the event
//	     index count, or any table's row count differ afterwards.
//	FH3  Compaction loses the working set written since the last checkpoint.
//	FH4  Concurrent writes interleave with the squash and the checks.
//	FH5  Online full GC leaves the store's write or read session unusable,
//	     or the tape cannot append after it.
//	FH6  Compaction starts without free space for its output.
//	FH7  Compaction re-runs every tick (no reclaim, or after a failure).
//	FH8  A second branch keeps history reachable.
//	FH9  A crash between the soft reset and the squash commit loses rows.
//	FH10 A compaction decision is invisible off the guest.
//	FH11 The kill switches are ignored.
//	FH12 A store-size gate turns compaction off for a large store.
//
// Added at the review gate, written before the code:
//
//	FH14 A compaction killed mid-run (planned refresh of the always-busy
//	     paper, OOM, VM kill) compacts again on the next boot instead of
//	     backing off.
//	FH15 Boot storm: compaction runs on the first tick after boot (before
//	     rt.Start, so boot writes wait out a full GC), or every enabled
//	     computer compacts at the same moment after a deploy.
//	FH16 Fleet exposure: compaction runs on a computer the host did not
//	     enable (default must be off; the journal GC stays fleet-wide).
//	FH17 A content change the light witness (heads, counts) cannot see
//	     passes: row contents must match a full content witness taken on
//	     a second session, and a mismatch fails closed, visibly, with the
//	     backoff.
//	FH18 The squash target is picked by date, so clock skew or equal
//	     timestamps pick the wrong commit; the root is the parentless
//	     commit, and more than one refuses.

func compactionInputs(availBytes, storeBytes, baselineBytes uint64) doltCompactionInputs {
	total := uint64(48 * gibBytes)
	return doltCompactionInputs{
		Usage:         doltGCDiskUsage{TotalBytes: total, UsedBytes: total - availBytes, AvailBytes: availBytes},
		StoreBytes:    storeBytes,
		BaselineBytes: baselineBytes,
		BudgetBytes:   defaultDoltHistoryBudgetMiB * mibBytes,
		Now:           time.Date(2026, 10, 15, 4, 0, 0, 0, time.UTC),
	}
}

// FH1: the paper's store (11 GiB, never compacted, 33 GiB free) compacts.
func TestPlanDoltCompactionRunsWhenHistoryExceedsBudget(t *testing.T) {
	plan := planDoltCompaction(compactionInputs(33*gibBytes, 11*gibBytes, 0))
	if !plan.Run || plan.BudgetBytes != 2*gibBytes || plan.HistoryBytes != 11*gibBytes {
		t.Fatalf("plan = %+v, want run with a 2 GiB budget", plan)
	}
}

func TestPlanDoltCompactionNoopUnderBudget(t *testing.T) {
	for _, in := range []doltCompactionInputs{
		compactionInputs(40*gibBytes, 1536*mibBytes, 0),       // small store, never compacted
		compactionInputs(40*gibBytes, 4*gibBytes, 3*gibBytes), // 1 GiB since the last compaction
	} {
		if plan := planDoltCompaction(in); plan.Run || plan.Outcome != "noop" {
			t.Fatalf("plan(%+v) = %+v, want noop", in, plan)
		}
	}
}

// FH12: cost follows the live set, so a large store with little history does
// not run, and a large store with much history does.
func TestPlanDoltCompactionHasNoStoreSizeGate(t *testing.T) {
	in := compactionInputs(60*gibBytes, 40*gibBytes, 39*gibBytes)
	in.Usage.TotalBytes = 120 * gibBytes
	if plan := planDoltCompaction(in); plan.Run {
		t.Fatalf("plan = %+v, want noop for 1 GiB of history", plan)
	}
	in.BaselineBytes = 30 * gibBytes
	if plan := planDoltCompaction(in); !plan.Run {
		t.Fatalf("plan = %+v, want run for 10 GiB of history on a 40 GiB store", plan)
	}
}

// The budget tightens with free space, floored so a full disk does not
// compact on every tick.
func TestPlanDoltCompactionBudgetFollowsHeadroom(t *testing.T) {
	if plan := planDoltCompaction(compactionInputs(4*gibBytes, 5*gibBytes, 3*gibBytes)); plan.BudgetBytes != gibBytes {
		t.Fatalf("budget = %d MiB at 4 GiB free, want 1024", plan.BudgetBytes/mibBytes)
	}
	if plan := planDoltCompaction(compactionInputs(gibBytes, 5*gibBytes, 3*gibBytes)); plan.BudgetBytes != doltCompactionBudgetFloorBytes {
		t.Fatalf("budget = %d MiB at 1 GiB free, want the floor", plan.BudgetBytes/mibBytes)
	}
}

// FH6: full GC writes its output before it frees anything.
func TestPlanDoltCompactionDefersWithoutOutputSpace(t *testing.T) {
	plan := planDoltCompaction(compactionInputs(6*gibBytes, 11*gibBytes, 0))
	if plan.Run || plan.Outcome != "deferred_space" || plan.RequiredAvailBytes <= 6*gibBytes {
		t.Fatalf("plan = %+v, want deferred_space", plan)
	}
}

// FH7: a failed compaction is not retried on the next tick.
func TestPlanDoltCompactionBacksOffAfterFailure(t *testing.T) {
	in := compactionInputs(33*gibBytes, 11*gibBytes, 0)
	in.LastFailureAt = in.Now.Add(-time.Hour)
	if plan := planDoltCompaction(in); plan.Run || plan.Outcome != "backoff" {
		t.Fatalf("plan = %+v, want backoff an hour after a failure", plan)
	}
	in.LastFailureAt = in.Now.Add(-doltCompactionFailureBackoff - time.Minute)
	if plan := planDoltCompaction(in); !plan.Run {
		t.Fatalf("plan = %+v, want run once the backoff passed", plan)
	}
}

// A store smaller than its recorded baseline (rebased or compacted offline)
// takes its current size as the new baseline.
func TestPlanDoltCompactionBaselineAboveStore(t *testing.T) {
	plan := planDoltCompaction(compactionInputs(40*gibBytes, 3*gibBytes, 5*gibBytes))
	if plan.Run || plan.BaselineBytes != 3*gibBytes || plan.HistoryBytes != 0 {
		t.Fatalf("plan = %+v, want noop with the baseline reset to the store", plan)
	}
}

// FH2, FH3, FH5: an online compaction on a store fed by the real tape path
// (Prepare + FinalizeBatch per event, a Dolt checkpoint every few events)
// drops history and keeps the projection exactly: the heads row and the event
// index count (the offline runbook's checks), every table's row count, the
// uncommitted working set, both sessions, and the ability to append.
func TestOnlineDoltCompactionPreservesTapeProjection(t *testing.T) {
	ctx := context.Background()
	s := openProjectStore(t)
	s.doltCheckpointEvery = 4
	computerID := "computer-compaction"
	bindCompactionTape(t, s, computerID)

	appendMemoryEntries(t, s, 0, 80)
	// Promote history to the old generation, as the paper's online GC does.
	if err := s.runOnlineDoltGC(ctx); err != nil {
		t.Fatalf("default gc: %v", err)
	}
	s.doltCheckpointEvery = 1000
	appendMemoryEntries(t, s, 80, 83) // working set past the last checkpoint

	headsBefore := runbookHeads(t, s, computerID)
	indexBefore := countRows(t, s, "computer_event_index")
	memoryBefore := countRows(t, s, "run_memory_entries")
	commitsBefore := countRows(t, s, "dolt_log")
	storeBefore := doltLiveStoreBytes(s.texturePath)
	if commitsBefore < 20 || indexBefore != 84 {
		t.Fatalf("fixture commits=%d index=%d, want >= 20 and 84", commitsBefore, indexBefore)
	}

	result, err := s.compactDoltHistoryOnline(ctx, "test")
	if err != nil {
		t.Fatalf("compact: %v", err)
	}
	if !result.Squashed || result.CommitsBefore != commitsBefore || result.CommitsAfter > 2 {
		t.Fatalf("result = %+v, want squashed to <= 2 commits from %d", result, commitsBefore)
	}
	if result.Before.HeadsDigest == "" || result.Before.HeadsDigest != result.After.HeadsDigest || result.Before.EventIndexCount != int64(indexBefore) {
		t.Fatalf("witness before=%+v after=%+v", result.Before, result.After)
	}
	if got := runbookHeads(t, s, computerID); got != headsBefore {
		t.Fatalf("projection head %q -> %q", headsBefore, got)
	}
	if got := countRows(t, s, "computer_event_index"); got != indexBefore {
		t.Fatalf("event index count %d -> %d", indexBefore, got)
	}
	if got := countRows(t, s, "run_memory_entries"); got != memoryBefore {
		t.Fatalf("run memory count %d -> %d", memoryBefore, got)
	}
	if storeAfter := doltLiveStoreBytes(s.texturePath); storeAfter >= storeBefore {
		t.Fatalf("store bytes %d -> %d, want history dropped", storeBefore, storeAfter)
	}
	var readCount int
	if err := s.readDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM computer_event_index").Scan(&readCount); err != nil || readCount != indexBefore {
		t.Fatalf("read session after compaction: count=%d err=%v", readCount, err)
	}

	// The tape keeps appending on top of the preserved head, and checkpoints.
	s.doltCheckpointEvery = 2
	appendMemoryEntries(t, s, 83, 90)
	if got := countRows(t, s, "computer_event_index"); got != indexBefore+7 {
		t.Fatalf("event index after appends = %d, want %d", got, indexBefore+7)
	}
	head, err := s.Head(ctx, computerID)
	if err != nil || head == nil || head.Sequence != uint64(indexBefore+7) {
		t.Fatalf("head after appends = %+v, %v", head, err)
	}

	// A second compaction squashes again and stays exact.
	again, err := s.compactDoltHistoryOnline(ctx, "test")
	if err != nil || again.Before.HeadsDigest != again.After.HeadsDigest || again.CommitsAfter > 2 {
		t.Fatalf("second compaction = %+v, %v", again, err)
	}
}

// FH8: a second branch keeps history reachable; compaction refuses and
// leaves history as it was.
func TestOnlineDoltCompactionRefusesExtraBranch(t *testing.T) {
	ctx := context.Background()
	s := openProjectStore(t)
	s.doltCheckpointEvery = 2
	bindCompactionTape(t, s, "computer-compaction-branch")
	appendMemoryEntries(t, s, 0, 10)
	if _, err := s.textureHandle().ExecContext(ctx, "CALL DOLT_BRANCH('keep-history')"); err != nil {
		t.Fatal(err)
	}
	before := countRows(t, s, "dolt_log")
	if _, err := s.compactDoltHistoryOnline(ctx, "test"); err == nil || !strings.Contains(err.Error(), "refused") {
		t.Fatalf("compaction with a second branch: err=%v, want refused", err)
	}
	if after := countRows(t, s, "dolt_log"); after != before {
		t.Fatalf("commits %d -> %d after a refused compaction", before, after)
	}
}

// FH9: the only non-atomic window is between the soft reset (HEAD moves to
// the root) and the squash commit. A crash there leaves every row in the
// persisted working set; the store reopens and the next compaction succeeds.
func TestDoltCompactionCrashAfterSoftResetKeepsRows(t *testing.T) {
	ctx := context.Background()
	path := testStorePath(t)
	cleanupTestStorePath(path)
	t.Cleanup(func() { cleanupTestStorePath(path) })
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s.doltCheckpointEvery = 3
	computerID := "computer-compaction-crash"
	bindCompactionTape(t, s, computerID)
	appendMemoryEntries(t, s, 0, 12)
	heads := runbookHeads(t, s, computerID)
	index := countRows(t, s, "computer_event_index")
	var root string
	rootConn, err := s.textureHandle().Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	root, err = doltRootCommit(ctx, rootConn)
	_ = rootConn.Close()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.textureHandle().ExecContext(ctx, "CALL DOLT_RESET('--soft', ?)", root); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil { // the crash: no squash commit
		t.Fatal(err)
	}

	s, err = Open(path)
	if err != nil {
		t.Fatalf("reopen after the crash window: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if got := runbookHeads(t, s, computerID); got != heads {
		t.Fatalf("head after crash window %q, want %q", got, heads)
	}
	if got := countRows(t, s, "computer_event_index"); got != index {
		t.Fatalf("event index after crash window = %d, want %d", got, index)
	}
	result, err := s.compactDoltHistoryOnline(ctx, "test")
	if err != nil || result.Before.HeadsDigest != result.After.HeadsDigest {
		t.Fatalf("compaction after the crash window = %+v, %v", result, err)
	}
}

// FH7, FH10, FH11: the decision record is written and readable for /health,
// a compaction resets the baseline so the next tick is a no-op, and both kill
// switches stop it.
func TestMaybeCompactDoltHistoryRecordsAndResetsBaseline(t *testing.T) {
	ctx := context.Background()
	s := openProjectStore(t)
	s.doltCheckpointEvery = 3
	const computerID = "computer-compaction-record"
	bindCompactionTape(t, s, computerID)
	enableCompactionFor(t, computerID)
	appendMemoryEntries(t, s, 0, 30)
	// The journal GC runs first on every tick and promotes history out of
	// the journal; only then is it counted.
	if err := s.runOnlineDoltGC(ctx); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	defer stubHistoryBudget(64 << 10)()
	defer stubDiskUsage(smallDiskUsage())()

	if err := s.MaybeCompactDoltHistory(ctx, dir, computerID); err != nil {
		t.Fatalf("compaction: %v", err)
	}
	got := readCompactionRecord(t, dir)
	if got.Outcome != "ran" || got.CommitsAfter > 2 || got.BaselineBytes == 0 || got.HeadsDigest == "" || got.EventIndexCount != 31 ||
		got.ContentRoot == "" || !got.WitnessMatch || got.LastFailureAt != "" || got.Degraded != "" {
		t.Fatalf("record = %+v, want ran with a baseline, both witnesses matched, no failure", got)
	}
	var served map[string]any
	if raw := ReadDoltCompactionRecord(dir); raw == nil || json.Unmarshal(raw, &served) != nil {
		t.Fatalf("record not readable for /health: %s", raw)
	}

	if err := s.MaybeCompactDoltHistory(ctx, dir, computerID); err != nil {
		t.Fatalf("second tick: %v", err)
	}
	if again := readCompactionRecord(t, dir); again.Outcome != "noop" || again.BaselineBytes != got.BaselineBytes {
		t.Fatalf("second tick record = %+v, want noop on the same baseline", again)
	}

	appendMemoryEntries(t, s, 30, 60)
	commits := countRows(t, s, "dolt_log")
	for _, env := range []string{"RUNTIME_DOLT_COMPACT_DISABLED", "RUNTIME_DOLT_GC_DISABLED"} {
		t.Run(env, func(t *testing.T) {
			t.Setenv(env, "1")
			if err := s.MaybeCompactDoltHistory(ctx, dir, computerID); err != nil {
				t.Fatal(err)
			}
			if rec := readCompactionRecord(t, dir); rec.Outcome != "skipped_disabled" {
				t.Fatalf("record = %+v, want skipped_disabled", rec)
			}
			if after := countRows(t, s, "dolt_log"); after != commits {
				t.Fatalf("commits %d -> %d while disabled", commits, after)
			}
		})
	}
}

// FH1: the periodic loop compacts, not only collects the journal (on a
// later tick: the first never compacts).
func TestStartPeriodicDoltGCCompactsHistory(t *testing.T) {
	s := openProjectStore(t)
	s.doltCheckpointEvery = 3
	const computerID = "computer-compaction-periodic"
	bindCompactionTape(t, s, computerID)
	enableCompactionFor(t, computerID)
	defer stubCompactionJitter(0)()
	appendMemoryEntries(t, s, 0, 24)
	if err := s.runOnlineDoltGC(context.Background()); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	defer stubHistoryBudget(64 << 10)()
	defer stubDiskUsage(smallDiskUsage())()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	StartPeriodicDoltGC(ctx, s, dir, computerID, 200*time.Millisecond)
	deadline := time.Now().Add(60 * time.Second)
	for {
		if raw := ReadDoltCompactionRecord(dir); raw != nil {
			var rec doltCompactionRecord
			if json.Unmarshal(raw, &rec) == nil && rec.LastSuccessAt != "" {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("periodic loop never compacted history: %s", ReadDoltCompactionRecord(dir))
		}
		time.Sleep(50 * time.Millisecond)
	}
	cancel()
}

// FH15: the first tick after boot never compacts, even when everything
// else says run; the computer is still starting.
func TestPeriodicDoltGCNeverCompactsOnFirstTick(t *testing.T) {
	s := openProjectStore(t)
	s.doltCheckpointEvery = 3
	const computerID = "computer-compaction-first-tick"
	bindCompactionTape(t, s, computerID)
	enableCompactionFor(t, computerID)
	defer stubCompactionJitter(0)()
	appendMemoryEntries(t, s, 0, 24)
	if err := s.runOnlineDoltGC(context.Background()); err != nil {
		t.Fatal(err)
	}
	commits := countRows(t, s, "dolt_log")
	dir := t.TempDir()
	defer stubHistoryBudget(64 << 10)()
	defer stubDiskUsage(smallDiskUsage())()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	StartPeriodicDoltGC(ctx, s, dir, computerID, time.Hour) // only the first tick fires
	deadline := time.Now().Add(30 * time.Second)
	var rec doltCompactionRecord
	for {
		if raw := ReadDoltCompactionRecord(dir); raw != nil && json.Unmarshal(raw, &rec) == nil && rec.Outcome != "" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("first tick wrote no compaction record")
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	if rec.Outcome != "boot_wait" {
		t.Fatalf("first tick record = %+v, want boot_wait", rec)
	}
	if after := countRows(t, s, "dolt_log"); after != commits {
		t.Fatalf("commits %d -> %d on the first tick", commits, after)
	}
}

// FH15: jitter spreads the first eligible compaction over 0-30 minutes, and
// a tick before it waits.
func TestDoltCompactionJitterDelaysFirstEligibleTick(t *testing.T) {
	for i := 0; i < 200; i++ {
		if j := defaultDoltCompactionStartJitter(); j < 0 || j >= doltCompactionMaxStartJitter {
			t.Fatalf("jitter %s outside [0, %s)", j, doltCompactionMaxStartJitter)
		}
	}
	s := openProjectStore(t)
	const computerID = "computer-compaction-jitter"
	bindCompactionTape(t, s, computerID)
	enableCompactionFor(t, computerID)
	dir := t.TempDir()
	defer stubHistoryBudget(1)()
	defer stubDiskUsage(smallDiskUsage())()
	now := time.Now()
	if err := s.maybeCompactDoltHistory(context.Background(), dir, doltCompactionTick{ComputerID: computerID, Now: now, EligibleAt: now.Add(10 * time.Minute)}); err != nil {
		t.Fatal(err)
	}
	if rec := readCompactionRecord(t, dir); rec.Outcome != "boot_wait" || !strings.Contains(rec.Detail, "eligible at") {
		t.Fatalf("record before the jittered start = %+v, want boot_wait", rec)
	}
}

// FH16: off unless the host lists this computer; the list names computers,
// so another computer's entry does not enable this one.
func TestDoltCompactionDisabledByDefault(t *testing.T) {
	ctx := context.Background()
	s := openProjectStore(t)
	s.doltCheckpointEvery = 3
	const computerID = "computer-compaction-default-off"
	bindCompactionTape(t, s, computerID)
	appendMemoryEntries(t, s, 0, 12)
	if err := s.runOnlineDoltGC(ctx); err != nil {
		t.Fatal(err)
	}
	commits := countRows(t, s, "dolt_log")
	dir := t.TempDir()
	defer stubHistoryBudget(1)()
	defer stubDiskUsage(smallDiskUsage())()
	for _, list := range []string{"", "computer-someone-else", " computer-a, computer-b "} {
		t.Setenv(doltCompactComputersEnv, list)
		if err := s.MaybeCompactDoltHistory(ctx, dir, computerID); err != nil {
			t.Fatal(err)
		}
		if rec := readCompactionRecord(t, dir); rec.Outcome != "not_enabled" {
			t.Fatalf("list %q: record = %+v, want not_enabled", list, rec)
		}
		if after := countRows(t, s, "dolt_log"); after != commits {
			t.Fatalf("list %q: commits %d -> %d while not enabled", list, commits, after)
		}
	}
	if err := s.MaybeCompactDoltHistory(ctx, dir, ""); err != nil {
		t.Fatal(err)
	}
	if rec := readCompactionRecord(t, dir); rec.Outcome != "not_enabled" {
		t.Fatalf("empty computer id: record = %+v, want not_enabled", rec)
	}
	t.Setenv(doltCompactComputersEnv, "computer-a,"+computerID)
	if err := s.MaybeCompactDoltHistory(ctx, dir, computerID); err != nil {
		t.Fatal(err)
	}
	if rec := readCompactionRecord(t, dir); rec.Outcome != "ran" {
		t.Fatalf("listed computer: record = %+v, want ran", rec)
	}
}

// FH14: the record says in_progress, with the failure time set, before the
// compaction starts. A guest killed mid-run therefore boots into the
// backoff instead of compacting again.
func TestDoltCompactionInterruptedRunBacksOffAfterRestart(t *testing.T) {
	ctx := context.Background()
	s := openProjectStore(t)
	s.doltCheckpointEvery = 3
	const computerID = "computer-compaction-killed"
	bindCompactionTape(t, s, computerID)
	enableCompactionFor(t, computerID)
	appendMemoryEntries(t, s, 0, 24)
	if err := s.runOnlineDoltGC(ctx); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	defer stubHistoryBudget(1)()
	defer stubDiskUsage(smallDiskUsage())()

	// The kill: snapshot the record after the full GC but before the run
	// finishes, then put that snapshot back as if the process died there.
	var midRun []byte
	defer stubCompactionAfterGCHook(func(context.Context, *sql.Conn) error {
		midRun, _ = os.ReadFile(filepath.Join(dir, doltCompactionRecordFileName))
		return nil
	})()
	if err := s.MaybeCompactDoltHistory(ctx, dir, computerID); err != nil {
		t.Fatal(err)
	}
	var during doltCompactionRecord
	if err := json.Unmarshal(midRun, &during); err != nil || during.Outcome != "in_progress" || during.LastFailureAt == "" {
		t.Fatalf("record during the run = %s (%v), want in_progress with last_failure_at", midRun, err)
	}
	if err := os.WriteFile(filepath.Join(dir, doltCompactionRecordFileName), midRun, 0o644); err != nil {
		t.Fatal(err)
	}

	appendMemoryEntries(t, s, 24, 36)
	if err := s.runOnlineDoltGC(ctx); err != nil {
		t.Fatal(err)
	}
	commits := countRows(t, s, "dolt_log")
	if err := s.MaybeCompactDoltHistory(ctx, dir, computerID); err != nil {
		t.Fatal(err)
	}
	rec := readCompactionRecord(t, dir)
	if rec.Outcome != "backoff" || !strings.Contains(rec.Detail, "interrupted") || rec.LastFailureAt != during.LastFailureAt {
		t.Fatalf("record after the restart = %+v, want backoff naming the interrupted run", rec)
	}
	if after := countRows(t, s, "dolt_log"); after != commits {
		t.Fatalf("commits %d -> %d during the backoff", commits, after)
	}
}

// FH17: a content change that keeps every count and head (the light
// witness) is caught by the full content witness on the read pool. The
// compaction fails closed: degraded on /health, the backoff set.
func TestDoltCompactionContentWitnessMismatchDegrades(t *testing.T) {
	ctx := context.Background()
	s := openProjectStore(t)
	s.doltCheckpointEvery = 3
	const computerID = "computer-compaction-mismatch"
	bindCompactionTape(t, s, computerID)
	enableCompactionFor(t, computerID)
	appendMemoryEntries(t, s, 0, 18)
	if err := s.runOnlineDoltGC(ctx); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	defer stubHistoryBudget(1)()
	defer stubDiskUsage(smallDiskUsage())()
	defer stubCompactionAfterGCHook(func(ctx context.Context, conn *sql.Conn) error {
		_, err := conn.ExecContext(ctx, "UPDATE run_memory_entries SET summary='changed under the compaction' WHERE entry_id='memory-0003'")
		return err
	})()

	err := s.MaybeCompactDoltHistory(ctx, dir, computerID)
	if err == nil || !strings.Contains(err.Error(), "content witness") {
		t.Fatalf("compaction with changed content: err=%v, want a content witness failure", err)
	}
	rec := readCompactionRecord(t, dir)
	if rec.Outcome != "degraded" || !strings.Contains(rec.Degraded, "run_memory_entries") || rec.LastFailureAt == "" {
		t.Fatalf("record = %+v, want degraded naming the table, with the backoff", rec)
	}
	if err := s.MaybeCompactDoltHistory(ctx, dir, computerID); err != nil {
		t.Fatal(err)
	}
	if again := readCompactionRecord(t, dir); again.Outcome != "backoff" || again.Degraded == "" {
		t.Fatalf("next tick = %+v, want backoff that keeps the degraded reason", again)
	}
}

// FH18: the squash target is the store's only parentless commit.
func TestDoltRootCommitIsParentless(t *testing.T) {
	ctx := context.Background()
	s := openProjectStore(t)
	s.doltCheckpointEvery = 2
	bindCompactionTape(t, s, "computer-compaction-root")
	appendMemoryEntries(t, s, 0, 8)
	conn, err := s.textureHandle().Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	root, err := doltRootCommit(ctx, conn)
	if err != nil {
		t.Fatal(err)
	}
	var parents int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM dolt_commit_ancestors WHERE commit_hash = ? AND parent_hash IS NOT NULL", root).Scan(&parents); err != nil || parents != 0 {
		t.Fatalf("root %s has %d parents (%v)", root, parents, err)
	}
	var message string
	if err := conn.QueryRowContext(ctx, "SELECT message FROM dolt_log WHERE commit_hash = ?", root).Scan(&message); err != nil || message == "" {
		t.Fatalf("root %s not in dolt_log: %v", root, err)
	}
}

func enableCompactionFor(t *testing.T, computerID string) {
	t.Helper()
	t.Setenv(doltCompactComputersEnv, computerID)
}

func stubCompactionJitter(d time.Duration) func() {
	previous := doltCompactionStartJitter
	doltCompactionStartJitter = func() time.Duration { return d }
	return func() { doltCompactionStartJitter = previous }
}

func stubCompactionAfterGCHook(hook func(context.Context, *sql.Conn) error) func() {
	previous := doltCompactionAfterGCHook
	doltCompactionAfterGCHook = hook
	return func() { doltCompactionAfterGCHook = previous }
}

func stubHistoryBudget(bytes uint64) func() {
	previous := historyBudgetForCompaction
	historyBudgetForCompaction = func() uint64 { return bytes }
	return func() { historyBudgetForCompaction = previous }
}

func TestDoltHistoryBudgetEnv(t *testing.T) {
	t.Setenv("RUNTIME_DOLT_HISTORY_BUDGET_MIB", "")
	if got := doltHistoryBudgetBytes(); got != 2*gibBytes {
		t.Fatalf("default budget = %d MiB, want 2048", got/mibBytes)
	}
	t.Setenv("RUNTIME_DOLT_HISTORY_BUDGET_MIB", "4096")
	if got := doltHistoryBudgetBytes(); got != 4*gibBytes {
		t.Fatalf("configured budget = %d MiB, want 4096", got/mibBytes)
	}
	t.Setenv("RUNTIME_DOLT_HISTORY_BUDGET_MIB", "bogus")
	if got := doltHistoryBudgetBytes(); got != 2*gibBytes {
		t.Fatalf("invalid budget = %d MiB, want the default", got/mibBytes)
	}
}

func bindCompactionTape(t *testing.T, s *Store, computerID string) {
	t.Helper()
	prepareGenesis(t, s, computerID, "genesis-"+computerID)
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer := computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "platform-control", KeyID: "test"}, PrivateKey: privateKey}
	appender, err := computerevent.NewComputerEventAppender(computerID, liveTapePinner{signer: signer}, s, liveTapeCAS{signer: signer, projection: s}, liveTapeVerifier{})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.BindProjectionTape(computerID, appender); err != nil {
		t.Fatal(err)
	}
}

// appendMemoryEntries appends one tape event (projection_batch_recorded)
// per run memory entry, each carrying a few KiB, like the paper's trace.
func appendMemoryEntries(t *testing.T, s *Store, from, to int) {
	t.Helper()
	ctx := context.Background()
	pad := strings.Repeat("model response text and tool result ", 120)
	for i := from; i < to; i++ {
		message, _ := json.Marshal(map[string]any{"role": "assistant", "content": fmt.Sprintf("%d %s", i, pad)})
		entry := types.RunMemoryEntry{
			EntryID: fmt.Sprintf("memory-%04d", i), RunID: fmt.Sprintf("run-%d", i/10), OwnerID: "owner-compaction",
			Kind: types.RunMemoryEntryMessage, Role: "assistant", Message: message,
			CreatedAt: time.Date(2026, 10, 15, 3, 0, i%60, 0, time.UTC),
		}
		if _, err := s.AppendRunMemoryEntry(ctx, entry); err != nil {
			t.Fatalf("append memory entry %d: %v", i, err)
		}
	}
}

// runbookHeads is the offline runbook's head check, verbatim, digested.
func runbookHeads(t *testing.T, s *Store, computerID string) string {
	t.Helper()
	var sequence uint64
	var head string
	if err := s.db.QueryRow("SELECT sequence, canonical_event_head FROM computer_event_projection_heads WHERE computer_id=?", computerID).Scan(&sequence, &head); err != nil {
		t.Fatalf("projection head: %v", err)
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d:%s", sequence, head)))
	return hex.EncodeToString(sum[:])
}

func countRows(t *testing.T, s *Store, table string) int {
	t.Helper()
	var n int
	if err := s.textureHandle().QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func readCompactionRecord(t *testing.T, dir string) doltCompactionRecord {
	t.Helper()
	raw := ReadDoltCompactionRecord(dir)
	if raw == nil {
		t.Fatal("compaction record missing")
	}
	var rec doltCompactionRecord
	if err := json.Unmarshal(raw, &rec); err != nil {
		t.Fatalf("decode compaction record: %v", err)
	}
	return rec
}
