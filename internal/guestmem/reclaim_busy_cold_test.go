package guestmem

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// D6 S5, busy_cold: the always-busy computer's cache return act
// (docs/design/density-d6-page-cache-return-2026-10-26.md sections 4.1, 5.5,
// 9 S5). The guest half; the route's busy rule is in
// internal/agentcore/memory_cache_return_busy_cold_test.go.
//
// Ways it can fail, written first:
//   - the target formula is wrong at its edges: active_file larger than the
//     512 MiB keep is not the keep; dirty and writeback pages are reclaimed
//     (write I/O on a working computer); a target under 128 MiB writes; the
//     1 GiB cap is exceeded; a negative keep subtraction wraps;
//   - the refault guard is advisory: the act writes with no 60 s baseline,
//     with workingset_refault_file unreported, with a refault rate over the
//     limit in the 60 s before, or keeps writing when the rate over the act
//     and the 60 s before climbs past the limit mid-act;
//   - the guard reads a baseline older than its window (a stalled sampler)
//     as if it were the 60 s before;
//   - a counter that went backwards (a reset) reads as a zero rate;
//   - a swap device is present and busy_cold writes anyway;
//   - two acts run at once;
//   - the busy_cold budget (1 s) is not the one applied;
//   - the report does not carry the refault readings before and after, or
//     the limit, so the act cannot be proven from the guest's own log.

// busyColdWindow is a refault window whose clock is fixed at now, with a
// sample of base refaults at now-age.
func busyColdWindow(f *reclaimFixture, now time.Time, age time.Duration, base int64) *RefaultWindow {
	w := &RefaultWindow{ProcRoot: f.proc, Now: func() time.Time { return now }}
	w.Observe(now.Add(-age), base)
	return w
}

func (f *reclaimFixture) setRefaults(n int64) {
	f.writeFile(filepath.Join(f.proc, "vmstat"),
		fmt.Sprintf("pgsteal_proactive 10\nworkingset_refault_file %d\npgmajfault 30\npgpgout 40\n", n))
}

func busyColdRequest(w *RefaultWindow) ReclaimRequest {
	return ReclaimRequest{Mode: ModeBusyCold, Scope: ScopeDaemon, Refaults: w}
}

// The formula's edges: busy_cold = min(1 GiB, cold - max(512 MiB, active_file)).
func TestBusyColdTargetEdges(t *testing.T) {
	cases := []struct {
		name                                     string
		file, inactive, active, shmem, dirty, wb int64
		want                                     int64
	}{
		// active_file 2000 > 512: the keep is the active set. cold 3000.
		{"active_file is the keep when larger", 6000, 3000, 2000, 0, 0, 0, 1000},
		// active_file 100 < 512: the keep is 512. cold 1000.
		{"512 MiB is the keep when active is small", 2000, 1000, 100, 0, 0, 0, 488},
		// dirty 300 and writeback 200 come off the cold set first: cold 1000-500=500 < 512.
		{"dirty and writeback are never targeted", 2000, 1000, 100, 0, 300, 200, 0},
		// the cap.
		{"capped at 1 GiB", 9000, 8000, 300, 0, 0, 0, 1024},
		// keep larger than cold: zero, never negative.
		{"keep larger than cold is zero", 1000, 400, 900, 0, 0, 0, 0},
		// the paper on 2026-11-01: inactive 6080, active 27, file 6050 in the
		// daemon cgroup (shmem small): capped at 1 GiB per act.
		{"the paper's reading", 6050, 6080, 27, 4, 8, 0, 1024},
	}
	for _, c := range cases {
		st := ParseMemoryStat(memoryStat(c.file, c.inactive, c.active, c.shmem, c.dirty, c.wb))
		if got := ReclaimTarget(ModeBusyCold, st); got != c.want*mib {
			t.Errorf("%s: target = %d MiB, want %d", c.name, got/mib, c.want)
		}
	}
}

// Under 128 MiB is nothing_to_return with zero writes (the guard is not even
// asked to stop it: there is nothing to do).
func TestBusyColdUnderTheFloorWritesNothing(t *testing.T) {
	f := newReclaimFixture(t)
	// cold 600, keep 512: 88 MiB.
	f.setScopeStat(f.daemon, memoryStat(1000, 600, 100, 0, 0, 0))
	now := time.Now()
	got, err := f.reclaimer().Reclaim(context.Background(), busyColdRequest(busyColdWindow(f, now, 65*time.Second, 20)))
	if err != nil {
		t.Fatal(err)
	}
	if got.Outcome != OutcomeNothingToReturn || f.writeCount() != 0 {
		t.Fatalf("outcome %q with %d writes, want nothing_to_return and none", got.Outcome, f.writeCount())
	}
}

// The act runs with a quiet baseline: 64 MiB steps up to the target, the
// limit and both rates in the report.
func TestBusyColdRunsWithAQuietBaseline(t *testing.T) {
	f := newReclaimFixture(t)
	f.setScopeStat(f.daemon, memoryStat(6000, 5000, 100, 0, 0, 0)) // target 1024
	f.setRefaults(20 + 60)                                         // 1 refault/s over 60 s
	now := time.Now()
	got, err := f.reclaimer().Reclaim(context.Background(), busyColdRequest(busyColdWindow(f, now, 60*time.Second, 20)))
	if err != nil {
		t.Fatal(err)
	}
	if got.Outcome != OutcomeReturned || got.Steps != 16 || got.TargetBytes != 1024*mib {
		t.Fatalf("outcome %q steps %d target %d MiB, want returned, 16 steps, 1024 MiB", got.Outcome, got.Steps, got.TargetBytes/mib)
	}
	if got.RefaultLimitPerSec != RefaultLimitPerSecond {
		t.Fatalf("limit = %v, want %v", got.RefaultLimitPerSec, RefaultLimitPerSecond)
	}
	if got.RefaultRateBefore == nil || *got.RefaultRateBefore != 1 || got.RefaultRateAfter == nil {
		t.Fatalf("rates before %v after %v, want 1/s before and an after reading", got.RefaultRateBefore, got.RefaultRateAfter)
	}
	if got.Before.WorkingsetRefaultFile == nil || *got.Before.WorkingsetRefaultFile != 80 {
		t.Fatalf("before refaults = %v, want 80", got.Before.WorkingsetRefaultFile)
	}
}

// The guard stops the act before any write when the 60 s before were
// refaulting over the limit.
func TestBusyColdRefaultGuardRefusesAHotBaseline(t *testing.T) {
	f := newReclaimFixture(t)
	f.setScopeStat(f.daemon, memoryStat(6000, 5000, 100, 0, 0, 0))
	limit := int64(RefaultLimitPerSecond)
	f.setRefaults(20 + (limit+1)*60) // just over the limit for 60 s
	now := time.Now()
	got, err := f.reclaimer().Reclaim(context.Background(), busyColdRequest(busyColdWindow(f, now, 60*time.Second, 20)))
	if err != nil {
		t.Fatal(err)
	}
	if got.Outcome != OutcomeRefaultStop || f.writeCount() != 0 {
		t.Fatalf("outcome %q with %d writes, want refault_stop and none", got.Outcome, f.writeCount())
	}
	// At the limit exactly, it runs.
	f.setRefaults(20 + limit*60)
	got, _ = f.reclaimer().Reclaim(context.Background(), busyColdRequest(busyColdWindow(f, now, 60*time.Second, 20)))
	if got.Outcome != OutcomeReturned {
		t.Fatalf("at the limit: outcome %q, want returned", got.Outcome)
	}
}

// The guard is re-read between steps: refaults that climb mid-act stop it.
func TestBusyColdRefaultGuardStopsMidAct(t *testing.T) {
	f := newReclaimFixture(t)
	f.setScopeStat(f.daemon, memoryStat(6000, 5000, 100, 0, 0, 0))
	f.setRefaults(20)
	r := f.reclaimer()
	var mu sync.Mutex
	n := 0
	r.Write = func(path, value string) error {
		mu.Lock()
		n++
		if n == 3 {
			// The working set comes back: 10,000 refaults since the baseline.
			f.setRefaults(10020)
		}
		mu.Unlock()
		return nil
	}
	now := time.Now()
	got, err := r.Reclaim(context.Background(), busyColdRequest(busyColdWindow(f, now, 60*time.Second, 20)))
	if err != nil {
		t.Fatal(err)
	}
	if got.Outcome != OutcomeRefaultStop || got.Steps != 3 {
		t.Fatalf("outcome %q after %d steps, want refault_stop after 3", got.Outcome, got.Steps)
	}
	if got.RefaultRateAfter == nil || *got.RefaultRateAfter <= RefaultLimitPerSecond {
		t.Fatalf("after rate = %v, want over the limit", got.RefaultRateAfter)
	}
}

// Fail closed: no window, no baseline at least 60 s old, a baseline older
// than the window allows (stalled sampler), an unreported counter, or a
// counter that went backwards: refault_stop with zero writes.
func TestBusyColdRefaultGuardFailsClosed(t *testing.T) {
	now := time.Now()
	cases := map[string]func(f *reclaimFixture) ReclaimRequest{
		"no window": func(f *reclaimFixture) ReclaimRequest { return busyColdRequest(nil) },
		"baseline younger than 60 s": func(f *reclaimFixture) ReclaimRequest {
			return busyColdRequest(busyColdWindow(f, now, 30*time.Second, 20))
		},
		"baseline older than the window": func(f *reclaimFixture) ReclaimRequest {
			return busyColdRequest(busyColdWindow(f, now, 10*time.Minute, 20))
		},
		"counter unreported": func(f *reclaimFixture) ReclaimRequest {
			f.writeFile(filepath.Join(f.proc, "vmstat"), "pgsteal_proactive 10\n")
			return busyColdRequest(busyColdWindow(f, now, 60*time.Second, 20))
		},
		"counter went backwards": func(f *reclaimFixture) ReclaimRequest {
			f.setRefaults(5)
			return busyColdRequest(busyColdWindow(f, now, 60*time.Second, 20))
		},
	}
	for name, arrange := range cases {
		t.Run(name, func(t *testing.T) {
			f := newReclaimFixture(t)
			f.setScopeStat(f.daemon, memoryStat(6000, 5000, 100, 0, 0, 0))
			f.setRefaults(20)
			req := arrange(f)
			got, err := f.reclaimer().Reclaim(context.Background(), req)
			if err != nil {
				t.Fatal(err)
			}
			if got.Outcome != OutcomeRefaultStop || f.writeCount() != 0 {
				t.Fatalf("outcome %q with %d writes, want refault_stop and none", got.Outcome, f.writeCount())
			}
		})
	}
}

// The window picks the newest sample at least 60 s old, so a rate is over
// about the last minute, not since boot.
func TestRefaultWindowUsesTheNewestSampleOutsideTheLookback(t *testing.T) {
	now := time.Now()
	w := &RefaultWindow{Now: func() time.Time { return now }}
	w.Observe(now.Add(-110*time.Second), 0)
	w.Observe(now.Add(-70*time.Second), 1000)
	w.Observe(now.Add(-50*time.Second), 5000) // inside the lookback: not a base
	rate, ok := w.Rate(1700)
	if !ok || rate != 10 {
		t.Fatalf("rate = %v ok=%v, want 10/s over the 70 s since the -70 s sample", rate, ok)
	}
}

// The sampler reads the real counter from ProcRoot and keeps a bounded ring.
func TestRefaultWindowSampleReadsVmstatAndPrunes(t *testing.T) {
	f := newReclaimFixture(t)
	clock := time.Now()
	w := &RefaultWindow{ProcRoot: f.proc, Now: func() time.Time { return clock }}
	for i := 0; i < 100; i++ {
		f.setRefaults(int64(i))
		w.Sample()
		clock = clock.Add(RefaultSampleInterval)
	}
	if n := w.sampleCount(); n > int(refaultMaxBaseAge/RefaultSampleInterval)+2 {
		t.Fatalf("ring holds %d samples, want it pruned to the window", n)
	}
	// A vmstat without the counter adds nothing.
	f.writeFile(filepath.Join(f.proc, "vmstat"), "pgsteal_proactive 1\n")
	before := w.sampleCount()
	w.Sample()
	if w.sampleCount() != before {
		t.Fatal("a missing counter was sampled")
	}
}

// Swap present refuses busy_cold too.
func TestBusyColdRefusesWhenASwapDeviceIsPresent(t *testing.T) {
	f := newReclaimFixture(t)
	f.setScopeStat(f.daemon, memoryStat(6000, 5000, 100, 0, 0, 0))
	f.writeFile(filepath.Join(f.proc, "swaps"), "Filename Type Size Used Priority\n/dev/vdb partition 1048572 0 -2\n")
	got, err := f.reclaimer().Reclaim(context.Background(), busyColdRequest(busyColdWindow(f, time.Now(), 60*time.Second, 20)))
	if err != nil {
		t.Fatal(err)
	}
	if got.Outcome != OutcomeSwapPresent || f.writeCount() != 0 {
		t.Fatalf("outcome %q with %d writes, want swap_present and none", got.Outcome, f.writeCount())
	}
}

// One act at a time, busy_cold included.
func TestBusyColdIsOneActAtATime(t *testing.T) {
	f := newReclaimFixture(t)
	f.setScopeStat(f.daemon, memoryStat(6000, 5000, 100, 0, 0, 0))
	r := f.reclaimer()
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	r.Write = func(string, string) error {
		select {
		case entered <- struct{}{}:
		default:
		}
		select {
		case <-release:
		case <-time.After(5 * time.Second):
		}
		return nil
	}
	now := time.Now()
	done := make(chan struct{})
	go func() {
		_, _ = r.Reclaim(context.Background(), busyColdRequest(busyColdWindow(f, now, 60*time.Second, 20)))
		close(done)
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the first act never wrote")
	}
	if _, err := r.Reclaim(context.Background(), busyColdRequest(busyColdWindow(f, now, 60*time.Second, 20))); err != ErrInProgress {
		t.Fatalf("second act: err = %v, want ErrInProgress", err)
	}
	close(release)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the first act did not finish")
	}
}

// busy_cold's budget is 1 s: a writer slower than that gets one step only.
func TestBusyColdBudgetIsOneSecond(t *testing.T) {
	if got := (&Reclaimer{}).budget(ModeBusyCold); got != time.Second {
		t.Fatalf("busy_cold budget = %v, want 1s", got)
	}
}
