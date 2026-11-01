package guestmem

import (
	"context"
	"path/filepath"
	"testing"
)

// D6 S6, ceiling mode. Ways it can fail, written first:
//   - a burst of anonymous memory over the ceiling is asked for (it must
//     yield nothing: the ceiling only ever takes cache);
//   - shmem, dirty or writeback pages, or the last keep of cache, are asked
//     for;
//   - a guest under its ceiling is written to;
//   - a ceiling under the floor squeezes a guest (refused, not clamped);
//   - one act asks for more than its cap.

func ceilingRequest(ceilingMiB int64) ReclaimRequest {
	return ReclaimRequest{Mode: ModeCeiling, Scope: ScopeRoot, CeilingBytes: ceilingMiB * mib}
}

func TestCeilingTarget(t *testing.T) {
	st := func(file, shmem, dirty, writeback int64) MemoryStat {
		return MemoryStat{File: file * mib, Shmem: shmem * mib, FileDirty: dirty * mib, FileWriteback: writeback * mib}
	}
	for _, tc := range []struct {
		name                 string
		stat                 MemoryStat
		used, ceiling, wantM int64
	}{
		{"over by 1 GiB of cache", st(3000, 0, 0, 0), 4096, 3072, 1024},
		{"under the ceiling", st(3000, 0, 0, 0), 2048, 3072, 0},
		{"over by anonymous memory: the keep stays", st(300, 0, 0, 0), 6000, 3072, 44},
		{"shmem, dirty and writeback excluded", st(2000, 500, 200, 100), 8000, 3072, 944},
		{"capped per act", st(8000, 0, 0, 0), 8192, 1024, 4096},
	} {
		if got := CeilingTarget(tc.stat, tc.used*mib, tc.ceiling*mib); got != tc.wantM*mib {
			t.Errorf("%s: target = %d MiB, want %d", tc.name, got/mib, tc.wantM)
		}
	}
}

func TestReclaimCeilingAsksForTheExcessFromRoot(t *testing.T) {
	f := newReclaimFixture(t)
	// The fixture guest: MemTotal 8 GiB, MemFree 4 GiB, so 4096 MiB used;
	// the root cgroup holds 3000 MiB of clean file pages.
	rep, err := f.reclaimer().Reclaim(context.Background(), ceilingRequest(3072))
	if err != nil {
		t.Fatal(err)
	}
	if rep.Outcome != OutcomeReturned || rep.TargetBytes != 1024*mib || rep.UsedBytes != 4096*mib || rep.CeilingBytes != 3072*mib {
		t.Fatalf("report = %+v", rep)
	}
	if n := len(f.writesTo(filepath.Join(f.cgroup, "memory.reclaim"))); n != 16 {
		t.Fatalf("root writes = %d, want 16 steps of 64 MiB", n)
	}
	if n := len(f.writesTo(filepath.Join(f.daemon, "memory.reclaim"))); n != 0 {
		t.Fatalf("daemon writes = %d, want 0", n)
	}
}

func TestReclaimCeilingUnderItWritesNothing(t *testing.T) {
	f := newReclaimFixture(t)
	rep, err := f.reclaimer().Reclaim(context.Background(), ceilingRequest(4096))
	if err != nil {
		t.Fatal(err)
	}
	if rep.Outcome != OutcomeNothingToReturn || f.writeCount() != 0 {
		t.Fatalf("outcome = %s, writes = %d", rep.Outcome, f.writeCount())
	}
}

func TestReclaimCeilingAnonymousBurstWritesNothing(t *testing.T) {
	f := newReclaimFixture(t)
	// 7 GiB used, almost all anonymous: the cache left is the keep.
	f.writeFile(filepath.Join(f.proc, "meminfo"), "MemTotal: 8388608 kB\nMemFree: 1048576 kB\nMemAvailable: 1200000 kB\nAnonPages: 6900000 kB\nShmem: 1024 kB\n")
	f.writeFile(filepath.Join(f.cgroup, "memory.stat"), memoryStat(300, 200, 100, 0, 0, 0))
	rep, err := f.reclaimer().Reclaim(context.Background(), ceilingRequest(3072))
	if err != nil {
		t.Fatal(err)
	}
	if rep.Outcome != OutcomeNothingToReturn || f.writeCount() != 0 {
		t.Fatalf("outcome = %s, writes = %d", rep.Outcome, f.writeCount())
	}
}

func TestReclaimCeilingUnderTheFloorIsRefused(t *testing.T) {
	f := newReclaimFixture(t)
	if _, err := f.reclaimer().Reclaim(context.Background(), ceilingRequest(512)); err == nil {
		t.Fatal("a 512 MiB ceiling was accepted")
	}
	if f.writeCount() != 0 {
		t.Fatalf("writes = %d", f.writeCount())
	}
}

// Compaction after a ceiling act. Ways it can fail, written first:
//   - the freed cache stays in blocks under free page reporting's order, so
//     the host keeps it (the paper, 2026-11-01: 4.9 GB free in the guest,
//     1.6 GB reportable, host RSS 6.8 GB);
//   - a compaction runs when nothing was reclaimed, on a dry run, or in
//     another mode (a cost with nothing to gain);
//   - a compaction that fails turns a good act into an error.

func TestReclaimCeilingCompactsAfterAnAct(t *testing.T) {
	f := newReclaimFixture(t)
	compact := filepath.Join(f.proc, "sys", "vm", "compact_memory")
	f.writeFile(compact, "")
	rep, err := f.reclaimer().Reclaim(context.Background(), ceilingRequest(3072))
	if err != nil {
		t.Fatal(err)
	}
	if got := f.writesTo(compact); len(got) != 1 || got[0] != "1" || !rep.Compacted {
		t.Fatalf("compaction writes = %+v, compacted = %v", got, rep.Compacted)
	}
}

func TestReclaimCeilingCompactsOnlyAfterWrites(t *testing.T) {
	for _, tc := range []struct {
		name string
		req  ReclaimRequest
	}{
		{"under the ceiling", ceilingRequest(4096)},
		{"dry run", func() ReclaimRequest { r := ceilingRequest(3072); r.DryRun = true; return r }()},
		{"idle mode", ReclaimRequest{Mode: ModeIdle, Scope: ScopeDaemon}},
	} {
		f := newReclaimFixture(t)
		compact := filepath.Join(f.proc, "sys", "vm", "compact_memory")
		f.writeFile(compact, "")
		rep, err := f.reclaimer().Reclaim(context.Background(), tc.req)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if n := len(f.writesTo(compact)); n != 0 || rep.Compacted {
			t.Fatalf("%s: compaction writes = %d, compacted = %v", tc.name, n, rep.Compacted)
		}
	}
}
