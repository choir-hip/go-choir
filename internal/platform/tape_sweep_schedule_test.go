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
	res := f.svc.RunScheduledTapeSweep(f.ctx)
	if res.Outcome != TapeSweepPassDone || res.Deleted != 8 || res.ComputersDeleted != 1 {
		t.Fatalf("pass = %+v", res)
	}
	f.allExist(dead, false)
	for _, c := range []*tapeChain{listed, owned, routed, recent} {
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
	if d := f.svc.afterTapeSweepPass(TapeSweepPassResult{Outcome: TapeSweepPassDone}, now); d != TapeSweepScheduleInterval {
		t.Fatalf("done: next in %v", d)
	}
}
