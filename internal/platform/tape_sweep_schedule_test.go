package platform

import (
	"bytes"
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// Failure list for station S7, "the collector reaps by itself". Each test pins
// one of them before the scheduler exists:
//
//	F1 a destroyed computer is never reaped unless a person runs the sweep
//	F2 a capped pass leaves the rest until the next person
//	F3 the schedule would hardcode the fork-lineage acknowledgement
//	F4 two sweeps could run at once (hand job and schedule, or two schedules)
//	F5 a restart mid-pass could delete twice or lose the rest
//	F6 the schedule could bypass the mode, the archive or a protection
//	F7 a pass leaves no line for the hourly status script
//	F8 the cadence resets on every deploy, so a daily pass may never run

func (f *tapeFixture) scheduleOpts(ack bool) {
	f.svc.tapeSweep.mu.Lock()
	f.svc.tapeSweep.schedForkAck = ack
	f.svc.tapeSweep.mu.Unlock()
}

func captureScheduleLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	var mu sync.Mutex
	log.SetOutput(writerFunc(func(p []byte) (int, error) { mu.Lock(); defer mu.Unlock(); return buf.Write(p) }))
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return &buf
}

type writerFunc func([]byte) (int, error)

func (w writerFunc) Write(p []byte) (int, error) { return w(p) }

// F1, F6: the pass reaps a destroyed computer with the existing proof and
// leaves every protected computer alone.
func TestScheduledTapeSweepReapsDestroyedAndLeavesProtected(t *testing.T) {
	f := newTapeFixture(t)
	f.scheduleOpts(true)
	dead := f.chain("dead-1", 2, tapeAged)
	f.reg.listed["listed-1"] = true
	listed := f.chain("listed-1", 1, tapeAged)
	f.reg.owned["owned-1"] = true
	owned := f.chain("owned-1", 1, tapeAged)
	routed := f.chain("routed-1", 1, tapeAged)
	f.insertClosure("code-r", platformTestDigest('9'), "routed-1")
	recent := f.chain("recent-1", 1, 2*24*time.Hour)
	paper := f.chain("computer-1355b83bced1613eb048ee28d66688a8", 1, tapeAged) // the hard-protected paper
	res := f.svc.RunScheduledTapeSweep(f.ctx)
	if res.Outcome != TapeSweepPassDone || res.Deleted != 8 || res.ComputersDeleted != 1 {
		t.Fatalf("pass = %+v", res)
	}
	f.allExist(dead, false)
	for _, c := range []*tapeChain{listed, owned, routed, recent, paper} {
		f.allExist(c, true)
	}
	// The default archive stays on: the scheduled pass never uses no_archive.
	if len(res.Archives) != 1 {
		t.Fatalf("archives = %v, want one", res.Archives)
	}
	if _, err := os.Stat(res.Archives[0]); err != nil {
		t.Fatalf("archive missing: %v", err)
	}
	// A second pass finds nothing to do.
	again := f.svc.RunScheduledTapeSweep(f.ctx)
	if again.Outcome != TapeSweepPassNothing || again.Deleted != 0 {
		t.Fatalf("second pass = %+v", again)
	}
}

// F3: without the operator acknowledgement the pass reports the computers it
// leaves, and never applies.
func TestScheduledTapeSweepLeavesUnrecordedLineageAndReportsIt(t *testing.T) {
	f := newTapeFixture(t)
	dead := f.chain("dead-1", 2, tapeAged)
	res := f.svc.RunScheduledTapeSweep(f.ctx)
	if res.Outcome != TapeSweepPassBlocked || res.BlockedByLineage != 1 || res.Listed != 0 || res.Deleted != 0 || res.Rounds != 1 {
		t.Fatalf("pass = %+v", res)
	}
	f.allExist(dead, true)
	if _, ok, _ := f.store.TapeTombstone(f.ctx, "dead-1"); ok {
		t.Fatal("a computer the pass left was tombstoned")
	}
}

// F2: a capped pass repeats dry run and apply until the destroyed set is gone.
func TestScheduledTapeSweepRepeatsWhileCapped(t *testing.T) {
	f := newTapeFixture(t)
	f.scheduleOpts(true)
	f.svc.tapeSweep.schedMaxDeletes = 5
	dead := f.chain("dead-1", 3, tapeAged) // 12 files
	res := f.svc.RunScheduledTapeSweep(f.ctx)
	if res.Outcome != TapeSweepPassDone || res.Deleted != 12 || res.Rounds != 3 || res.ComputersDeleted != 1 {
		t.Fatalf("pass = %+v", res)
	}
	f.allExist(dead, false)
}

// F2: a pass that deletes nothing while capped does not loop for ever.
func TestScheduledTapeSweepStopsWithoutProgress(t *testing.T) {
	f := newTapeFixture(t)
	f.scheduleOpts(true)
	f.svc.tapeSweep.schedMaxBytes = 1 // every file is larger: nothing fits
	dead := f.chain("dead-1", 1, tapeAged)
	res := f.svc.RunScheduledTapeSweep(f.ctx)
	if res.Outcome != TapeSweepPassStalled || res.Deleted != 0 || res.Rounds != 1 {
		t.Fatalf("pass = %+v", res)
	}
	f.allExist(dead, true)
}

// F6: the schedule follows the existing mode. dry-run applies nothing; off
// does not even classify.
func TestScheduledTapeSweepFollowsMode(t *testing.T) {
	f := newTapeFixture(t)
	f.scheduleOpts(true)
	dead := f.chain("dead-1", 1, tapeAged)
	f.svc.SetTapeSweepPolicy(TapeSweepPolicy{Mode: ArtifactGCModeDryRun})
	res := f.svc.RunScheduledTapeSweep(f.ctx)
	if res.Outcome != TapeSweepPassDryRunOnly || res.Listed != 1 || res.Deleted != 0 {
		t.Fatalf("dry-run pass = %+v", res)
	}
	f.allExist(dead, true)
	f.svc.SetTapeSweepPolicy(TapeSweepPolicy{Mode: ArtifactGCModeOff})
	res = f.svc.RunScheduledTapeSweep(f.ctx)
	if res.Outcome != TapeSweepPassOff || res.Rounds != 0 {
		t.Fatalf("off pass = %+v", res)
	}
	f.allExist(dead, true)
}

// F6: a refusal of the destroyed proof (registry failure) is reported, and
// nothing is deleted.
func TestScheduledTapeSweepReportsRegistryRefusal(t *testing.T) {
	f := newTapeFixture(t)
	f.scheduleOpts(true)
	dead := f.chain("dead-1", 1, tapeAged)
	f.reg.listErr = context.DeadlineExceeded
	res := f.svc.RunScheduledTapeSweep(f.ctx)
	if res.Outcome != TapeSweepPassRefused || res.Refused == "" || res.Deleted != 0 {
		t.Fatalf("pass = %+v", res)
	}
	f.allExist(dead, true)
}

// F4: a run in progress (a hand job) makes the pass wait; two passes never
// overlap.
func TestScheduledTapeSweepNeverOverlapsAnotherSweep(t *testing.T) {
	f := newTapeFixture(t)
	f.scheduleOpts(true)
	dead := f.chain("dead-1", 1, tapeAged)
	f.svc.tapeSweep.mu.Lock()
	f.svc.tapeSweep.running = true // a hand-started job
	f.svc.tapeSweep.mu.Unlock()
	res := f.svc.RunScheduledTapeSweep(f.ctx)
	if res.Outcome != TapeSweepPassBusy || res.Deleted != 0 {
		t.Fatalf("pass beside a hand job = %+v", res)
	}
	f.allExist(dead, true)
	f.svc.tapeSweep.mu.Lock()
	f.svc.tapeSweep.running = false
	f.svc.tapeSweep.mu.Unlock()

	// Two schedules: the second returns at once while the first holds the pass.
	entered, release := make(chan struct{}), make(chan struct{})
	f.svc.tapeSweep.afterReferences = func() {
		select {
		case <-entered:
		default:
			close(entered)
		}
		<-release
	}
	done := make(chan TapeSweepPassResult, 1)
	go func() { done <- f.svc.RunScheduledTapeSweep(f.ctx) }()
	<-entered
	second := f.svc.RunScheduledTapeSweep(f.ctx)
	if second.Outcome != TapeSweepPassBusy {
		t.Fatalf("second schedule = %+v", second)
	}
	close(release)
	first := <-done
	if first.Outcome != TapeSweepPassDone || first.Deleted != 4 {
		t.Fatalf("first schedule = %+v", first)
	}
}

// F5: a restart between a capped apply and the rest. The new process holds no
// memory of the first; its pass finishes the set and nothing is deleted twice.
func TestScheduledTapeSweepSurvivesARestart(t *testing.T) {
	f := newTapeFixture(t)
	f.scheduleOpts(true)
	f.svc.tapeSweep.schedMaxDeletes = 5
	f.svc.tapeSweep.schedMaxRounds = 1 // the process dies after round one
	dead := f.chain("dead-1", 3, tapeAged)
	first := f.svc.RunScheduledTapeSweep(f.ctx)
	if first.Outcome != TapeSweepPassRoundLimit || first.Deleted != 5 {
		t.Fatalf("first = %+v", first)
	}
	svc2 := NewService(f.store, f.root, "")
	svc2.SetTapeSweepPolicy(TapeSweepPolicy{Mode: ArtifactGCModeActive})
	svc2.SetComputerRegistry(f.reg)
	svc2.tapeSweep.schedForkAck = true
	svc2.tapeSweep.schedMaxDeletes = 5
	second := svc2.RunScheduledTapeSweep(f.ctx)
	if second.Outcome != TapeSweepPassDone || second.Deleted != 7 || second.Errors != 0 {
		t.Fatalf("second = %+v", second)
	}
	f.allExist(dead, false)
	tomb, _, _ := f.store.TapeTombstone(f.ctx, "dead-1")
	if tomb.Status != tapeTombstoneDeleted || tomb.FilesDeleted != 12 {
		t.Fatalf("tombstone = %+v", tomb)
	}
}

// F7: one log line per pass, with the counts the status script reads.
func TestScheduledTapeSweepLogsOneLinePerPass(t *testing.T) {
	f := newTapeFixture(t)
	f.scheduleOpts(true)
	f.chain("dead-1", 1, tapeAged)
	buf := captureScheduleLog(t)
	f.svc.RunScheduledTapeSweep(f.ctx)
	var lines []string
	for _, l := range strings.Split(buf.String(), "\n") {
		if strings.Contains(l, "tape sweep schedule:") {
			lines = append(lines, l)
		}
	}
	if len(lines) != 1 {
		t.Fatalf("%d schedule lines in %q", len(lines), buf.String())
	}
	for _, want := range []string{"outcome=done", "rounds=1", "listed=1", "blocked_by_fork_lineage=0", "deleted=4", "bytes=", "deferred_by_cap=0", "errors=0", "computers_deleted=1", "mode=active"} {
		if !strings.Contains(lines[0], want) {
			t.Fatalf("line %q lacks %q", lines[0], want)
		}
	}
}

// F8: the next pass is a day after the last finished pass, kept across
// restarts; with no record, or a stale one, it is soon after boot.
func TestTapeSweepScheduleDelaySurvivesRestarts(t *testing.T) {
	f := newTapeFixture(t)
	now := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	if d := f.svc.tapeSweepScheduleDelay(now); d != TapeSweepScheduleInitialDelay {
		t.Fatalf("no record: %v", d)
	}
	f.svc.markTapeSweepPass(now.Add(-time.Hour))
	if d := f.svc.tapeSweepScheduleDelay(now); d != TapeSweepScheduleInterval-time.Hour {
		t.Fatalf("an hour after a pass: %v", d)
	}
	f.svc.markTapeSweepPass(now.Add(-30 * time.Hour))
	if d := f.svc.tapeSweepScheduleDelay(now); d != TapeSweepScheduleInitialDelay {
		t.Fatalf("overdue: %v", d)
	}
	if err := os.WriteFile(filepath.Join(f.svc.tapeSweepLogDir(), tapeSweepScheduleStateName), []byte("{"), 0o640); err != nil {
		t.Fatal(err)
	}
	if d := f.svc.tapeSweepScheduleDelay(now); d != TapeSweepScheduleInitialDelay {
		t.Fatalf("corrupt record: %v", d)
	}
}

// F8: busy and error passes are retried sooner than a day and are not marked.
func TestTapeSweepSchedulerRetriesBusyAndErrorPasses(t *testing.T) {
	f := newTapeFixture(t)
	now := time.Now()
	for _, o := range []string{TapeSweepPassBusy, TapeSweepPassError} {
		if d := f.svc.afterTapeSweepPass(TapeSweepPassResult{Outcome: o}, now); d != TapeSweepScheduleRetry {
			t.Fatalf("%s: retry in %v", o, d)
		}
	}
	if _, err := os.Stat(f.svc.tapeSweepScheduleStatePath()); err == nil {
		t.Fatal("a busy or failed pass was marked as finished")
	}
	if d := f.svc.afterTapeSweepPass(TapeSweepPassResult{Outcome: TapeSweepPassDone}, now); d != TapeSweepScheduleInterval {
		t.Fatalf("done: next in %v", d)
	}
	if _, err := os.Stat(f.svc.tapeSweepScheduleStatePath()); err != nil {
		t.Fatalf("a finished pass was not marked: %v", err)
	}
	// A record from the future (a clock step) never waits longer than a day.
	f.svc.markTapeSweepPass(now.Add(72 * time.Hour))
	if d := f.svc.tapeSweepScheduleDelay(now); d != TapeSweepScheduleInterval {
		t.Fatalf("future record: %v", d)
	}
}

// B1: an apply with errors is never "done", whatever the cap says; the pass is
// retried at the retry interval, not after a day.
func TestScheduledTapeSweepApplyErrorsAreNotDone(t *testing.T) {
	f := newTapeFixture(t)
	f.scheduleOpts(true)
	blocker := filepath.Join(filepath.Dir(f.root), "not-a-dir")
	if err := os.WriteFile(blocker, []byte("x"), 0o640); err != nil {
		t.Fatal(err)
	}
	f.svc.SetTapeSweepPolicy(TapeSweepPolicy{Mode: ArtifactGCModeActive, ArchiveDir: blocker}) // the archive cannot be written
	dead := f.chain("dead-1", 1, tapeAged)
	res := f.svc.RunScheduledTapeSweep(f.ctx)
	if res.Outcome != TapeSweepPassError || res.Errors == 0 || res.Deleted != 0 || res.Err == "" {
		t.Fatalf("pass = %+v", res)
	}
	f.allExist(dead, true)
	if d := f.svc.afterTapeSweepPass(res, time.Now()); d != TapeSweepScheduleRetry {
		t.Fatalf("next pass in %v, want the retry interval", d)
	}
}

// B3: listed computers that apply refused (here: revived between the dry run
// and the apply, the same path an absent acknowledgement takes) are not "done".
func TestScheduledTapeSweepSkippedApplyHasItsOwnOutcome(t *testing.T) {
	f := newTapeFixture(t)
	f.scheduleOpts(true)
	f.svc.tapeSweep.schedAfterDry = func() { f.reg.owned["dead-1"] = true }
	dead := f.chain("dead-1", 1, tapeAged)
	res := f.svc.RunScheduledTapeSweep(f.ctx)
	if res.Outcome != TapeSweepPassSkipped || res.Listed != 1 || res.Deleted != 0 || res.ComputersSkipped != 1 {
		t.Fatalf("pass = %+v", res)
	}
	f.allExist(dead, true)
}

// B2: the tombstone's counts are written per batch, so an apply that is
// killed mid-computer leaves a tombstone that says what was deleted.
func TestTapeSweepApplyPersistsTombstoneCountsPerBatch(t *testing.T) {
	f := newTapeFixture(t)
	dead := f.chain("dead-1", 3, tapeAged) // 12 files
	dry := f.dryRun(TapeSweepRequest{AcceptUnrecordedForkLineage: true, Sweeps: []string{TapeSweepDestroyed}})
	seen, calls := map[int]int64{}, 0
	f.svc.tapeSweep.beforeDelete = func(tapeSweepItem) {
		calls++
		tomb, _, _ := f.store.TapeTombstone(f.ctx, "dead-1")
		seen[calls] = tomb.FilesDeleted
	}
	if _, err := f.apply(dry, TapeSweepRequest{AcceptUnrecordedForkLineage: true, BatchSize: 4}); err != nil {
		t.Fatal(err)
	}
	// Before the first item of batch 2 (call 5) and batch 3 (call 9) the
	// tombstone already counts the batches before it.
	if seen[1] != 0 || seen[5] != 4 || seen[9] != 8 {
		t.Fatalf("tombstone files_deleted seen before items: %v", seen)
	}
	f.allExist(dead, false)
	tomb, _, _ := f.store.TapeTombstone(f.ctx, "dead-1")
	if tomb.FilesDeleted != 12 {
		t.Fatalf("final tombstone = %+v", tomb)
	}
}

// B2: an archive left without its sidecar (a killed apply) is removed at the
// next pass start; verified archives and fresh ones are kept.
func TestScheduledTapeSweepRemovesSidecarlessArchives(t *testing.T) {
	f := newTapeFixture(t)
	dir := filepath.Join(f.root, tapeSweepArchiveDirName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	mk := func(name string, age time.Duration) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		when := time.Now().Add(-age)
		_ = os.Chtimes(p, when, when)
		return p
	}
	stale := mk("dead-1-ts-20261114T000000Z-aaaaaaaa.tar.zst", 3*time.Hour)
	staleTmp := mk("dead-1-ts-20261114T000000Z-bbbbbbbb.tar.zst.tmp", 3*time.Hour)
	whole := mk("dead-2-ts-20261114T000000Z-cccccccc.tar.zst", 3*time.Hour)
	mk("dead-2-ts-20261114T000000Z-cccccccc.tar.zst.json", 3*time.Hour)
	fresh := mk("dead-3-ts-20261114T000000Z-dddddddd.tar.zst", 0)
	f.svc.RunScheduledTapeSweep(f.ctx)
	for _, gone := range []string{stale, staleTmp} {
		if _, err := os.Stat(gone); err == nil {
			t.Fatalf("%s was kept", gone)
		}
	}
	for _, kept := range []string{whole, fresh} {
		if _, err := os.Stat(kept); err != nil {
			t.Fatalf("%s was removed", kept)
		}
	}
}

// F2: a capped round does not tombstone or archive a computer it cannot
// delete from.
func TestTapeSweepCapReachedSkipsLaterComputersArchive(t *testing.T) {
	f := newTapeFixture(t)
	f.scheduleOpts(true)
	f.svc.tapeSweep.schedMaxDeletes = 12
	f.svc.tapeSweep.schedMaxRounds = 1
	f.chain("dead-1", 3, tapeAged) // 12 files: exactly the cap
	dead2 := f.chain("dead-2", 3, tapeAged)
	res := f.svc.RunScheduledTapeSweep(f.ctx)
	if res.Outcome != TapeSweepPassRoundLimit || res.Deleted != 12 || len(res.Archives) != 1 {
		t.Fatalf("pass = %+v", res)
	}
	f.allExist(dead2, true)
	if _, ok, _ := f.store.TapeTombstone(f.ctx, "dead-2"); ok {
		t.Fatal("dead-2 was tombstoned with the cap already spent")
	}
}

func TestScheduledTapeSweepNilService(t *testing.T) {
	var s *Service
	if res := s.RunScheduledTapeSweep(context.Background()); res.Outcome != TapeSweepPassError {
		t.Fatalf("pass = %+v", res)
	}
}

// B4: the production entry runs a pass after the first delay, again after the
// next, logs each, and stops on cancel (Wait returns).
func TestTapeSweepSchedulerStartRunsPassesAndStops(t *testing.T) {
	f := newTapeFixture(t)
	buf := captureScheduleLog(t)
	passes := make(chan TapeSweepPassResult, 16)
	sch := &TapeSweepScheduler{
		service:   f.svc,
		first:     func() time.Duration { return 20 * time.Millisecond },
		next:      func(TapeSweepPassResult) time.Duration { return 20 * time.Millisecond },
		afterPass: func(r TapeSweepPassResult) { passes <- r },
	}
	ctx, cancel := context.WithCancel(context.Background())
	sch.Start(ctx)
	for i := 0; i < 2; i++ {
		select {
		case r := <-passes:
			if r.Outcome != TapeSweepPassNothing {
				t.Fatalf("pass %d = %+v", i, r)
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("pass %d never ran", i)
		}
	}
	cancel()
	wctx, wcancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer wcancel()
	if err := sch.Wait(wctx); err != nil {
		t.Fatalf("wait: %v", err)
	}
	n := strings.Count(buf.String(), "tape sweep schedule:")
	time.Sleep(100 * time.Millisecond)
	if m := strings.Count(buf.String(), "tape sweep schedule:"); m != n || n < 2 {
		t.Fatalf("schedule lines before/after cancel: %d, %d", n, m)
	}
}

// B4: a scheduler in mode off does not start at all.
func TestTapeSweepSchedulerOffModeDoesNotStart(t *testing.T) {
	f := newTapeFixture(t)
	f.svc.SetTapeSweepPolicy(TapeSweepPolicy{Mode: ArtifactGCModeOff})
	sch := &TapeSweepScheduler{service: f.svc, first: func() time.Duration { return time.Millisecond }, next: func(TapeSweepPassResult) time.Duration { return time.Millisecond }, afterPass: func(TapeSweepPassResult) { t.Error("a pass ran in mode off") }}
	sch.Start(context.Background())
	if err := sch.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
}
