package guestmem

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// D6 S1, the guest's cache return act
// (docs/design/density-d6-page-cache-return-2026-10-26.md section 4.1;
// docs/problems/density-station-d-failure-modes-2026-10-26.md D6).
//
// Ways it can fail, written first. All with a fake /proc, a fake cgroup root
// and an injectable writer; no test writes a real cgroup file.
//   - M21: the target counts shmem, dirty or writeback pages, so the act
//     spins into EAGAIN or does writeback I/O;
//   - M3: the act reclaims anonymous memory or swaps (a swap device is
//     present), or drops swappiness=0 so a kernel could pick anonymous pages;
//   - M4: a write that gets EAGAIN parks the caller, or the budget is only
//     advisory and a further step starts after it ran out;
//   - M25: the act writes to a cgroup other than the configured scope, or
//     falls back silently to another one when the scope's file is absent;
//   - M1 (the guest half): a busy signal between steps does not stop the act;
//   - two acts run at once;
//   - dry_run writes;
//   - the act runs at normal priority (it must run quietly).

const mib = int64(1 << 20)

// reclaimFixture is a fake guest: /proc and /sys/fs/cgroup under a temp dir,
// with the daemon in system.slice/daemon.service.
type reclaimFixture struct {
	t      *testing.T
	proc   string
	cgroup string
	daemon string // the daemon's cgroup dir

	mu     sync.Mutex
	writes []fakeWrite
}

type fakeWrite struct{ path, value string }

func (f *reclaimFixture) writeFile(path, content string) {
	f.t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

// memoryStat renders a cgroup memory.stat with the fields the act reads.
func memoryStat(file, inactive, active, shmem, dirty, writeback int64) string {
	return fmt.Sprintf("anon 1048576\nfile %d\nshmem %d\nfile_dirty %d\nfile_writeback %d\ninactive_file %d\nactive_file %d\nslab 4096\n",
		file*mib, shmem*mib, dirty*mib, writeback*mib, inactive*mib, active*mib)
}

func newReclaimFixture(t *testing.T) *reclaimFixture {
	t.Helper()
	root := t.TempDir()
	f := &reclaimFixture{t: t, proc: filepath.Join(root, "proc"), cgroup: filepath.Join(root, "cgroup")}
	f.daemon = filepath.Join(f.cgroup, "system.slice", "daemon.service")
	f.writeFile(filepath.Join(f.proc, "self", "cgroup"), "0::/system.slice/daemon.service\n")
	f.writeFile(filepath.Join(f.proc, "swaps"), "Filename\t\t\t\tType\t\tSize\t\tUsed\t\tPriority\n")
	f.writeFile(filepath.Join(f.proc, "meminfo"), "MemTotal: 8388608 kB\nMemFree: 4194304 kB\nMemAvailable: 6291456 kB\nAnonPages: 524288 kB\nShmem: 1024 kB\n")
	f.writeFile(filepath.Join(f.proc, "vmstat"), "pgsteal_proactive 10\nworkingset_refault_file 20\npgmajfault 30\npgpgout 40\n")
	f.writeFile(filepath.Join(f.proc, "sys", "vm", "drop_caches"), "")
	// file 2000, inactive 1500, active 500, shmem 300, dirty 100, writeback 50.
	f.setScopeStat(f.daemon, memoryStat(2000, 1500, 500, 300, 100, 50))
	f.writeFile(filepath.Join(f.cgroup, "memory.stat"), memoryStat(3000, 2500, 500, 0, 0, 0))
	f.writeFile(filepath.Join(f.cgroup, "memory.reclaim"), "")
	return f
}

func (f *reclaimFixture) setScopeStat(dir, stat string) {
	f.writeFile(filepath.Join(dir, "memory.stat"), stat)
	f.writeFile(filepath.Join(dir, "memory.reclaim"), "")
}

// reclaimer returns a Reclaimer on the fixture whose writer records every
// write and lowers the written cgroup's "file" by the written bytes, as a
// real reclaim would.
func (f *reclaimFixture) reclaimer() *Reclaimer {
	return &Reclaimer{
		ProcRoot:   f.proc,
		CgroupRoot: f.cgroup,
		Write: func(path, value string) error {
			f.mu.Lock()
			f.writes = append(f.writes, fakeWrite{path, value})
			f.mu.Unlock()
			return nil
		},
		Quiet: func() {},
	}
}

func (f *reclaimFixture) writeCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.writes)
}

func (f *reclaimFixture) writesTo(path string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, w := range f.writes {
		if w.path == path {
			out = append(out, w.value)
		}
	}
	return out
}

func idleRequest() ReclaimRequest { return ReclaimRequest{Mode: ModeIdle, Scope: ScopeDaemon} }

// M21: the formulas of section 4.1, per mode, with shmem, dirty and
// writeback all non-zero.
func TestReclaimTargetFormulaPerMode(t *testing.T) {
	// file 2000, inactive 1500, active 500, shmem 300, dirty 100, writeback 50.
	stat := ParseMemoryStat(memoryStat(2000, 1500, 500, 300, 100, 50))
	// cold = min(1500, 2000-300) - 100 - 50 = 1350.
	if got, want := ReclaimTarget(ModeIdle, stat), (1350-256)*mib; got != want {
		t.Fatalf("idle target = %d MiB, want %d", got/mib, want/mib)
	}
	if got, want := ReclaimTarget(ModeHibernate, stat), 1350*mib; got != want {
		t.Fatalf("hibernate target = %d MiB, want %d", got/mib, want/mib)
	}
	// busy_cold = min(1 GiB, cold - max(512, active_file)) = min(1024, 838).
	if got, want := ReclaimTarget(ModeBusyCold, stat), (1350-512)*mib; got != want {
		t.Fatalf("busy_cold target = %d MiB, want %d", got/mib, want/mib)
	}
	// The 1 GiB cap.
	big := ParseMemoryStat(memoryStat(9000, 8000, 500, 0, 0, 0))
	if got := ReclaimTarget(ModeBusyCold, big); got != 1024*mib {
		t.Fatalf("busy_cold cap = %d MiB, want 1024", got/mib)
	}
}

// M21: inactive_file larger than file - shmem takes the clamp.
func TestReclaimTargetClampsInactiveFileToFileMinusShmem(t *testing.T) {
	// inactive 1900 > file 2000 - shmem 300 = 1700; cold = 1700 - 150.
	stat := ParseMemoryStat(memoryStat(2000, 1900, 100, 300, 100, 50))
	if got, want := ReclaimTarget(ModeHibernate, stat), (1700-150)*mib; got != want {
		t.Fatalf("clamped hibernate target = %d MiB, want %d", got/mib, want/mib)
	}
	// shmem larger than file must not wrap an unsigned subtraction.
	odd := ParseMemoryStat(memoryStat(100, 90, 10, 400, 0, 0))
	if got := ReclaimTarget(ModeHibernate, odd); got != 0 {
		t.Fatalf("shmem > file target = %d, want 0", got)
	}
	// Dirty and writeback larger than the cold set must not go negative.
	dirty := ParseMemoryStat(memoryStat(300, 100, 100, 0, 200, 100))
	if got := ReclaimTarget(ModeHibernate, dirty); got != 0 {
		t.Fatalf("dirty > cold target = %d, want 0", got)
	}
}

// M21: a target under 128 MiB is nothing_to_return with zero writes.
func TestReclaimUnderTheFloorWritesNothing(t *testing.T) {
	f := newReclaimFixture(t)
	// cold = 300 - 0 = 300; idle target = 300 - 256 = 44 MiB < 128.
	f.setScopeStat(f.daemon, memoryStat(400, 300, 100, 0, 0, 0))
	got, err := f.reclaimer().Reclaim(context.Background(), idleRequest())
	if err != nil {
		t.Fatal(err)
	}
	if got.Outcome != OutcomeNothingToReturn || f.writeCount() != 0 {
		t.Fatalf("outcome %q with %d writes, want nothing_to_return and none", got.Outcome, f.writeCount())
	}
	// Exactly at the floor still acts: target 128 MiB (cold 384).
	f.setScopeStat(f.daemon, memoryStat(500, 384, 100, 0, 0, 0))
	got, err = f.reclaimer().Reclaim(context.Background(), idleRequest())
	if err != nil {
		t.Fatal(err)
	}
	if got.TargetBytes != 128*mib || f.writeCount() != 2 {
		t.Fatalf("at the floor: target %d MiB, %d writes, want 128 MiB and 2 steps of 64", got.TargetBytes/mib, f.writeCount())
	}
}

// Steps: 64 MiB per write, the last one the remainder, every write carrying
// swappiness=0 (M3).
func TestReclaimWritesSixtyFourMiBStepsWithSwappinessZero(t *testing.T) {
	f := newReclaimFixture(t)
	got, err := f.reclaimer().Reclaim(context.Background(), idleRequest())
	if err != nil {
		t.Fatal(err)
	}
	reclaimFile := filepath.Join(f.daemon, "memory.reclaim")
	values := f.writesTo(reclaimFile)
	// idle target = (1350-256) = 1094 MiB = 17 full steps + 6 MiB.
	if len(values) != 18 || got.Steps != 18 {
		t.Fatalf("%d writes, %d steps, want 18", len(values), got.Steps)
	}
	if values[0] != fmt.Sprintf("%d swappiness=0", 64*mib) {
		t.Fatalf("first write %q", values[0])
	}
	if want := fmt.Sprintf("%d swappiness=0", 6*mib); values[17] != want {
		t.Fatalf("last write %q, want %q", values[17], want)
	}
	if got.Outcome != OutcomeReturned || !got.SwappinessKey || got.Cgroup != f.daemon {
		t.Fatalf("report = %+v", got)
	}
	if got.TargetBytes != 1094*mib || got.Mode != ModeIdle || got.Scope != ScopeDaemon {
		t.Fatalf("report target/mode/scope = %+v", got)
	}
}

// M3: a swap device means no write at all.
func TestReclaimRefusesWhenASwapDeviceIsPresent(t *testing.T) {
	f := newReclaimFixture(t)
	f.writeFile(filepath.Join(f.proc, "swaps"),
		"Filename\t\t\t\tType\t\tSize\t\tUsed\t\tPriority\n/dev/vdb partition 1048572 0 -2\n")
	got, err := f.reclaimer().Reclaim(context.Background(), idleRequest())
	if err != nil {
		t.Fatal(err)
	}
	if got.Outcome != OutcomeSwapPresent || f.writeCount() != 0 {
		t.Fatalf("outcome %q with %d writes, want swap_present and none", got.Outcome, f.writeCount())
	}
	// An unreadable /proc/swaps is not "no swap": it fails closed.
	if err := os.Remove(filepath.Join(f.proc, "swaps")); err != nil {
		t.Fatal(err)
	}
	got, err = f.reclaimer().Reclaim(context.Background(), idleRequest())
	if err != nil {
		t.Fatal(err)
	}
	if got.Outcome != OutcomeSwapPresent || f.writeCount() != 0 {
		t.Fatalf("missing /proc/swaps: outcome %q with %d writes, want swap_present and none", got.Outcome, f.writeCount())
	}
}

// M3: an EINVAL on the first write (a kernel that rejects the swappiness
// key) makes the same step retry bare and every later write bare.
func TestReclaimFallsBackToBareBytesOnEINVAL(t *testing.T) {
	f := newReclaimFixture(t)
	r := f.reclaimer()
	var calls []string
	r.Write = func(path, value string) error {
		calls = append(calls, value)
		if strings.Contains(value, "swappiness") {
			return &os.PathError{Op: "write", Path: path, Err: syscall.EINVAL}
		}
		return nil
	}
	got, err := r.Reclaim(context.Background(), idleRequest())
	if err != nil {
		t.Fatal(err)
	}
	if got.SwappinessKey {
		t.Fatal("swappiness_key = true after EINVAL")
	}
	if got.Outcome != OutcomeReturned || got.Steps != 18 {
		t.Fatalf("outcome %q steps %d, want returned and 18", got.Outcome, got.Steps)
	}
	if len(calls) != 19 || !strings.Contains(calls[0], "swappiness=0") {
		t.Fatalf("calls = %d, first %q: want one rejected keyed write then 18 bare", len(calls), calls[0])
	}
	for i, c := range calls[1:] {
		if strings.Contains(c, "swappiness") {
			t.Fatalf("write %d after the EINVAL still keyed: %q", i+1, c)
		}
	}
}

// M4: an EAGAIN on step 2 ends the act as partial and the call returns.
func TestReclaimEAGAINEndsPartialAndReturns(t *testing.T) {
	f := newReclaimFixture(t)
	r := f.reclaimer()
	n := 0
	r.Write = func(path, value string) error {
		n++
		if n == 2 {
			return &os.PathError{Op: "write", Path: path, Err: syscall.EAGAIN}
		}
		return nil
	}
	done := make(chan ReclaimReport, 1)
	go func() {
		report, _ := r.Reclaim(context.Background(), idleRequest())
		done <- report
	}()
	select {
	case got := <-done:
		if got.Outcome != OutcomePartial || n != 2 || got.Steps != 1 {
			t.Fatalf("outcome %q after %d writes, %d completed steps, want partial after 2 writes and 1 completed step", got.Outcome, n, got.Steps)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the act did not return after EAGAIN")
	}
}

// M4: the budget is checked between steps: a writer that sleeps past it
// ends the act, and no further step is started.
func TestReclaimStartsNoStepAfterTheBudget(t *testing.T) {
	f := newReclaimFixture(t)
	r := f.reclaimer()
	r.Budget = 50 * time.Millisecond
	n := 0
	r.Write = func(string, string) error {
		n++
		time.Sleep(120 * time.Millisecond)
		return nil
	}
	got, err := r.Reclaim(context.Background(), idleRequest())
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 || got.Outcome != OutcomePartial {
		t.Fatalf("%d writes, outcome %q, want exactly one write and partial", n, got.Outcome)
	}
}

// M4: a cancelled context stops the act between steps.
func TestReclaimStopsWhenTheContextEnds(t *testing.T) {
	f := newReclaimFixture(t)
	r := f.reclaimer()
	ctx, cancel := context.WithCancel(context.Background())
	n := 0
	r.Write = func(string, string) error {
		n++
		cancel()
		return nil
	}
	got, err := r.Reclaim(ctx, idleRequest())
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 || got.Outcome != OutcomePartial {
		t.Fatalf("%d writes, outcome %q, want one and partial", n, got.Outcome)
	}
}

// M1 (guest half): the Stop hook is asked before every step; a stop after
// step 2 leaves exactly two writes and outcome stopped_busy.
func TestReclaimStopHookEndsTheActBetweenSteps(t *testing.T) {
	f := newReclaimFixture(t)
	r := f.reclaimer()
	n := 0
	r.Write = func(string, string) error { n++; return nil }
	stopAt := 2
	r.Stop = func() string {
		if n >= stopAt {
			return "run_started"
		}
		return ""
	}
	got, err := r.Reclaim(context.Background(), idleRequest())
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 || got.Outcome != OutcomeStoppedBusy || got.Steps != 2 {
		t.Fatalf("%d writes, outcome %q, steps %d, want 2, stopped_busy, 2", n, got.Outcome, got.Steps)
	}
	if got.Detail != "run_started" {
		t.Fatalf("detail %q, want the stop reason", got.Detail)
	}
}

// M25: scope daemon writes only to the cgroup /proc/self/cgroup names; scope
// root only to the root.
func TestReclaimWritesOnlyToTheConfiguredScope(t *testing.T) {
	f := newReclaimFixture(t)
	r := f.reclaimer()
	if _, err := r.Reclaim(context.Background(), idleRequest()); err != nil {
		t.Fatal(err)
	}
	daemonFile := filepath.Join(f.daemon, "memory.reclaim")
	if f.writeCount() == 0 || len(f.writesTo(daemonFile)) != f.writeCount() {
		t.Fatalf("daemon scope wrote %d times, %d to %s", f.writeCount(), len(f.writesTo(daemonFile)), daemonFile)
	}

	f2 := newReclaimFixture(t)
	r2 := f2.reclaimer()
	got, err := r2.Reclaim(context.Background(), ReclaimRequest{Mode: ModeIdle, Scope: ScopeRoot})
	if err != nil {
		t.Fatal(err)
	}
	rootFile := filepath.Join(f2.cgroup, "memory.reclaim")
	if f2.writeCount() == 0 || len(f2.writesTo(rootFile)) != f2.writeCount() || got.Cgroup != f2.cgroup {
		t.Fatalf("root scope wrote %d times, %d to %s; cgroup %q", f2.writeCount(), len(f2.writesTo(rootFile)), rootFile, got.Cgroup)
	}
}

// M25: the daemon in the root cgroup is not the daemon scope: no silent
// widening to the root.
func TestReclaimDaemonInTheRootCgroupIsUnsupportedNotRoot(t *testing.T) {
	f := newReclaimFixture(t)
	f.writeFile(filepath.Join(f.proc, "self", "cgroup"), "0::/\n")
	got, err := f.reclaimer().Reclaim(context.Background(), idleRequest())
	if err != nil {
		t.Fatal(err)
	}
	if got.Outcome != OutcomeUnsupported || f.writeCount() != 0 {
		t.Fatalf("outcome %q with %d writes, want unsupported and none", got.Outcome, f.writeCount())
	}
	// A cgroup v1 or hybrid line has no "0::" entry: also unsupported.
	f.writeFile(filepath.Join(f.proc, "self", "cgroup"), "12:memory:/foo\n")
	got, err = f.reclaimer().Reclaim(context.Background(), idleRequest())
	if err != nil {
		t.Fatal(err)
	}
	if got.Outcome != OutcomeUnsupported || f.writeCount() != 0 {
		t.Fatalf("no 0:: line: outcome %q with %d writes, want unsupported and none", got.Outcome, f.writeCount())
	}
}

// M25: a daemon cgroup path that climbs out of the cgroup root is refused.
func TestReclaimRefusesACgroupPathOutsideTheRoot(t *testing.T) {
	f := newReclaimFixture(t)
	f.writeFile(filepath.Join(f.proc, "self", "cgroup"), "0::/../../etc\n")
	got, err := f.reclaimer().Reclaim(context.Background(), idleRequest())
	if err != nil {
		t.Fatal(err)
	}
	if got.Outcome != OutcomeUnsupported || f.writeCount() != 0 {
		t.Fatalf("outcome %q with %d writes, want unsupported and none", got.Outcome, f.writeCount())
	}
}

// M25: a missing memory.reclaim in idle mode is unsupported, naming the
// path, and writes nothing (no drop_caches, no other cgroup).
func TestReclaimMissingFileIsUnsupportedInIdleMode(t *testing.T) {
	f := newReclaimFixture(t)
	reclaimFile := filepath.Join(f.daemon, "memory.reclaim")
	if err := os.Remove(reclaimFile); err != nil {
		t.Fatal(err)
	}
	got, err := f.reclaimer().Reclaim(context.Background(), idleRequest())
	if err != nil {
		t.Fatal(err)
	}
	if got.Outcome != OutcomeUnsupported || f.writeCount() != 0 {
		t.Fatalf("outcome %q with %d writes, want unsupported and none", got.Outcome, f.writeCount())
	}
	if !strings.Contains(got.Detail, reclaimFile) {
		t.Fatalf("detail %q does not name %s", got.Detail, reclaimFile)
	}
}

// M25: in hibernate mode the missing file falls back to exactly one
// drop_caches write.
func TestReclaimMissingFileFallsBackToDropCachesInHibernateMode(t *testing.T) {
	f := newReclaimFixture(t)
	if err := os.Remove(filepath.Join(f.daemon, "memory.reclaim")); err != nil {
		t.Fatal(err)
	}
	got, err := f.reclaimer().Reclaim(context.Background(), ReclaimRequest{Mode: ModeHibernate, Scope: ScopeDaemon})
	if err != nil {
		t.Fatal(err)
	}
	dropCaches := filepath.Join(f.proc, "sys", "vm", "drop_caches")
	if f.writeCount() != 1 || len(f.writesTo(dropCaches)) != 1 || f.writesTo(dropCaches)[0] != "1" {
		t.Fatalf("writes = %v, want one \"1\" to %s", f.writes, dropCaches)
	}
	if got.Method != MethodDropCaches {
		t.Fatalf("method %q, want drop_caches", got.Method)
	}
	// The fallback is visible in the report: the method and the detail that
	// names the file that was missing.
	if !strings.Contains(got.Detail, filepath.Join(f.daemon, "memory.reclaim")) {
		t.Fatalf("detail %q does not name the missing memory.reclaim", got.Detail)
	}
	// drop_caches is never the idle path, even when the cgroup file exists
	// and the target is too small: hibernate mode with the file present
	// uses memory.reclaim.
	f2 := newReclaimFixture(t)
	if _, err := f2.reclaimer().Reclaim(context.Background(), ReclaimRequest{Mode: ModeHibernate, Scope: ScopeDaemon}); err != nil {
		t.Fatal(err)
	}
	if len(f2.writesTo(filepath.Join(f2.proc, "sys", "vm", "drop_caches"))) != 0 {
		t.Fatal("hibernate mode used drop_caches although memory.reclaim exists")
	}
}

// dry_run sizes and reports and writes nothing, in every mode.
func TestReclaimDryRunWritesNothingAndReportsTheTarget(t *testing.T) {
	f := newReclaimFixture(t)
	for _, mode := range []string{ModeIdle, ModeHibernate} {
		got, err := f.reclaimer().Reclaim(context.Background(), ReclaimRequest{Mode: mode, Scope: ScopeDaemon, DryRun: true})
		if err != nil {
			t.Fatal(err)
		}
		if f.writeCount() != 0 {
			t.Fatalf("%s dry run wrote %d times", mode, f.writeCount())
		}
		if got.TargetBytes <= 0 || got.Outcome != OutcomeDryRun || !got.DryRun || got.Cgroup != f.daemon {
			t.Fatalf("%s dry run report = %+v", mode, got)
		}
		if got.Before.File == nil || *got.Before.File != 2000*mib {
			t.Fatalf("%s dry run before.file = %v", mode, got.Before.File)
		}
	}
	// A missing memory.reclaim is still reported in a dry run, naming the
	// path (R2 reads this).
	if err := os.Remove(filepath.Join(f.daemon, "memory.reclaim")); err != nil {
		t.Fatal(err)
	}
	got, err := f.reclaimer().Reclaim(context.Background(), ReclaimRequest{Mode: ModeIdle, Scope: ScopeDaemon, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if got.Outcome != OutcomeUnsupported || f.writeCount() != 0 {
		t.Fatalf("dry run without the file: %q, %d writes", got.Outcome, f.writeCount())
	}
}

// The report carries before and after blocks and reclaimed_bytes is the
// scope's memory.stat "file" before minus after, not the sum of the writes.
func TestReclaimReportsBeforeAfterAndReclaimedBytesFromMemoryStat(t *testing.T) {
	f := newReclaimFixture(t)
	r := f.reclaimer()
	statPath := filepath.Join(f.daemon, "memory.stat")
	r.Write = func(path, value string) error {
		// The kernel freed less than asked: after the act file is 1300 MiB.
		f.writeFile(statPath, memoryStat(1300, 800, 500, 300, 100, 50))
		f.writeFile(filepath.Join(f.proc, "vmstat"), "pgsteal_proactive 999\nworkingset_refault_file 21\npgmajfault 31\npgpgout 41\n")
		return nil
	}
	got, err := r.Reclaim(context.Background(), idleRequest())
	if err != nil {
		t.Fatal(err)
	}
	if got.ReclaimedBytes != 700*mib {
		t.Fatalf("reclaimed_bytes = %d MiB, want 700", got.ReclaimedBytes/mib)
	}
	if got.Before.MemAvailable == nil || *got.Before.MemAvailable != 6144*mib {
		t.Fatalf("before.mem_available = %v", got.Before.MemAvailable)
	}
	if got.Before.PgstealProactive == nil || *got.Before.PgstealProactive != 10 || got.After.PgstealProactive == nil || *got.After.PgstealProactive != 999 {
		t.Fatalf("pgsteal_proactive before %v after %v, want 10 and 999", got.Before.PgstealProactive, got.After.PgstealProactive)
	}
	if got.Before.WorkingsetRefaultFile == nil || *got.Before.WorkingsetRefaultFile != 20 ||
		got.Before.Pgmajfault == nil || *got.Before.Pgmajfault != 30 ||
		got.Before.Pgpgout == nil || *got.Before.Pgpgout != 40 {
		t.Fatalf("before vmstat = %+v", got.Before)
	}
	if got.After.File == nil || *got.After.File != 1300*mib || got.After.InactiveFile == nil || *got.After.InactiveFile != 800*mib {
		t.Fatalf("after stat = %+v", got.After)
	}
}

// Reporting facts come from the injected reader, before and after.
func TestReclaimReportsReportingNegotiatedAndReportableBlocks(t *testing.T) {
	f := newReclaimFixture(t)
	r := f.reclaimer()
	calls := 0
	r.Reporting = func() Reporting {
		calls++
		kib := int64(1000 * calls)
		yes := true
		return Reporting{Negotiated: &yes, ReportableKiB: &kib}
	}
	got, err := r.Reclaim(context.Background(), idleRequest())
	if err != nil {
		t.Fatal(err)
	}
	if got.ReportingNegotiated == nil || !*got.ReportingNegotiated {
		t.Fatalf("reporting_negotiated = %v", got.ReportingNegotiated)
	}
	if got.Before.ReportableKiB == nil || *got.Before.ReportableKiB != 1000 || got.After.ReportableKiB == nil || *got.After.ReportableKiB != 2000 {
		t.Fatalf("reportable_kib before %v after %v", got.Before.ReportableKiB, got.After.ReportableKiB)
	}
}

// One act at a time: a second call while one is writing answers
// ErrInProgress (run with -race).
func TestReclaimAllowsOneActAtATime(t *testing.T) {
	f := newReclaimFixture(t)
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
	first := make(chan error, 1)
	go func() {
		_, err := r.Reclaim(context.Background(), idleRequest())
		first <- err
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the first act never reached its first write")
	}
	if _, err := r.Reclaim(context.Background(), idleRequest()); err != ErrInProgress {
		t.Fatalf("second concurrent act: err = %v, want ErrInProgress", err)
	}
	close(release)
	select {
	case err := <-first:
		if err != nil {
			t.Fatalf("first act: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the first act did not finish")
	}
	// The lock is released: a later act runs.
	r.Write = func(string, string) error { return nil }
	if _, err := r.Reclaim(context.Background(), idleRequest()); err != nil {
		t.Fatalf("act after the first finished: %v", err)
	}
}

// Run quietly: a real act takes the quiet scope once, on a goroutine of its
// own; a refused or dry act does not take it.
func TestReclaimRunsQuietlyOnlyWhenItWrites(t *testing.T) {
	f := newReclaimFixture(t)
	r := f.reclaimer()
	entered := 0
	r.Quiet = func() { entered++ }
	if _, err := r.Reclaim(context.Background(), idleRequest()); err != nil {
		t.Fatal(err)
	}
	if entered != 1 {
		t.Fatalf("quiet entered %d times, want 1", entered)
	}
	if _, err := r.Reclaim(context.Background(), ReclaimRequest{Mode: ModeIdle, Scope: ScopeDaemon, DryRun: true}); err != nil {
		t.Fatal(err)
	}
	if entered != 1 {
		t.Fatalf("a dry run took the quiet scope (%d)", entered)
	}
}

// Unknown mode or scope is an error, not a default.
func TestReclaimRejectsUnknownModeAndScope(t *testing.T) {
	f := newReclaimFixture(t)
	for _, req := range []ReclaimRequest{{Mode: "", Scope: ScopeDaemon}, {Mode: "aggressive", Scope: ScopeDaemon}, {Mode: ModeIdle, Scope: "cgroup"}} {
		if _, err := f.reclaimer().Reclaim(context.Background(), req); err == nil {
			t.Fatalf("request %+v accepted", req)
		}
	}
	if f.writeCount() != 0 {
		t.Fatal("a rejected request wrote")
	}
}
