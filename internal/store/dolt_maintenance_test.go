package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Failure modes pinned here (docs/problems/guest-dolt-storage-growth-2026-10-15.md):
//
//	FM1 GC never runs between boots (the periodic loop's ctx was cancelled
//	    on return, so the paper's computer collected only when it rebooted).
//	FM2 A store-size gate suppresses journal GC for good once Dolt history
//	    (the old generation) passes 5 GiB.
//	FM3 GC waits until free space is too small for its output (the
//	    emergency GC at 0 MiB free could not complete).
//	FM4 A nearly full disk collects a near-empty journal on every tick.
//	FM5 Online GC invalidates the store's sessions or loses rows.
//	FM6 A failed or deferred GC is invisible off the guest.
//	FM7 Growth that GC cannot reclaim (history, live data) goes unreported.
//	FM8 The RUNTIME_DOLT_GC_DISABLED kill switch is ignored.

func planInputs(availBytes, journalBytes, liveBytes uint64) doltGCInputs {
	total := uint64(48 * gibBytes)
	return doltGCInputs{
		Usage:             doltGCDiskUsage{TotalBytes: total, UsedBytes: total - availBytes, AvailBytes: availBytes},
		JournalBytes:      journalBytes,
		LiveBytes:         liveBytes,
		JournalLimitBytes: gibBytes,
	}
}

// FM2: history size never gates a journal-triggered GC.
func TestPlanDoltGCIgnoresHistorySize(t *testing.T) {
	plan := planDoltGC(planInputs(20*gibBytes, gibBytes+1, 50*gibBytes))
	if !plan.Run || plan.Emergency {
		t.Fatalf("plan = %+v, want routine run with a 50 GiB old generation", plan)
	}
	if plan := planDoltGC(planInputs(20*gibBytes, 100<<20, 50*gibBytes)); plan.Run {
		t.Fatalf("plan = %+v, want noop below the 1 GiB journal threshold with ample space", plan)
	}
}

// FM3: the threshold tightens with free space, so GC runs while its output
// still fits; FM4: it never drops below the floor.
func TestPlanDoltGCThresholdFollowsHeadroom(t *testing.T) {
	cases := []struct {
		name      string
		avail     uint64
		journal   uint64
		run       bool
		threshold uint64
	}{
		{"ample space waits for 1 GiB", 40 * gibBytes, 600 << 20, false, gibBytes},
		{"2 GiB free collects 600 MiB", 2 * gibBytes, 600 << 20, true, 512 << 20},
		{"1 GiB free collects 300 MiB", gibBytes, 300 << 20, true, 256 << 20},
		{"600 MiB free waits for 150 MiB", 600 << 20, 32 << 20, false, 150 << 20},
		{"emergency at 512 MiB runs any journal", 512 << 20, 1 << 20, true, 128 << 20},
		{"emergency at 0 free", 0, 27 * gibBytes, true, doltGCMinJournalBytes},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plan := planDoltGC(planInputs(tc.avail, tc.journal, 6*gibBytes))
			if plan.Run != tc.run || plan.ThresholdBytes != tc.threshold {
				t.Fatalf("plan = %+v, want run=%v threshold=%d MiB", plan, tc.run, tc.threshold/mibBytes)
			}
		})
	}
}

func TestDoltGCJournalThresholdFloor(t *testing.T) {
	if got := doltGCJournalThreshold(gibBytes, 100<<20); got != doltGCMinJournalBytes {
		t.Fatalf("threshold at 100 MiB free = %d MiB, want the 64 MiB floor", got/mibBytes)
	}
}

func TestPlanDoltGCConfiguredLimit(t *testing.T) {
	in := planInputs(40*gibBytes, 3*gibBytes, 0)
	in.JournalLimitBytes = 4 * gibBytes
	if plan := planDoltGC(in); plan.Run {
		t.Fatalf("plan = %+v, want noop under a 4 GiB configured limit", plan)
	}
	in.JournalLimitBytes = 0 // unset falls back to 1 GiB
	if plan := planDoltGC(in); !plan.Run {
		t.Fatalf("plan = %+v, want run under the 1 GiB default", plan)
	}
}

// FM2 end to end: a large old generation plus a large journal runs the GC
// (the disposition says ran, not a size skip).
func TestMaybeRunDoltGCRunsWithLargeHistory(t *testing.T) {
	defer stubDiskUsage(doltGCDiskUsage{TotalBytes: 48 * gibBytes, UsedBytes: 20 * gibBytes, AvailBytes: 28 * gibBytes})()
	dir := t.TempDir()
	storePath := filepath.Join(dir, "ws.db")
	nomsDir := filepath.Join(resolveTextureWorkspacePath(storePath), "texture", ".dolt", "noms")
	sparseFile(t, filepath.Join(nomsDir, "oldgen", "history.darc"), 6*gibBytes)
	sparseFile(t, filepath.Join(nomsDir, doltJournalFileID), 2*gibBytes)

	if err := MaybeRunDoltGC(dir, storePath); err != nil {
		t.Fatalf("gc: %v", err)
	}
	got := readDisposition(t, dir)
	if got.Outcome != "ran" || got.Mode != "boot" || got.LiveGiB != 6 || got.JournalMiB != 2048 {
		t.Fatalf("disposition = %+v, want ran/boot with live 6 GiB and journal 2048 MiB", got)
	}
}

// FM10: boot GC runs before the HTTP listener, so a long one reads as
// refused connections and the wedge detector stops the computer (10-14
// 23:22-23:29). A journal past the boot cap is left to the online GC, which
// runs while the computer serves.
func TestMaybeRunDoltGCBootDefersHugeJournal(t *testing.T) {
	defer stubDiskUsage(doltGCDiskUsage{TotalBytes: 48 * gibBytes, UsedBytes: 36 * gibBytes, AvailBytes: 12 * gibBytes})()
	dir := t.TempDir()
	storePath := filepath.Join(dir, "ws.db")
	journal := filepath.Join(resolveTextureWorkspacePath(storePath), "texture", ".dolt", "noms", doltJournalFileID)
	sparseFile(t, journal, 27*gibBytes)
	if err := MaybeRunDoltGC(dir, storePath); err != nil {
		t.Fatalf("gc: %v", err)
	}
	if got := readDisposition(t, dir); got.Outcome != "deferred_online" || got.Mode != "boot" {
		t.Fatalf("disposition = %+v, want boot deferred_online for a 27 GiB journal", got)
	}
	sparseFile(t, journal, 3*gibBytes)
	if err := MaybeRunDoltGC(dir, storePath); err != nil {
		t.Fatalf("gc: %v", err)
	}
	if got := readDisposition(t, dir); got.Outcome != "ran" {
		t.Fatalf("disposition = %+v, want a 3 GiB journal collected at boot", got)
	}
}

// FM3: the emergency path still runs at near-zero free space.
func TestMaybeRunDoltGCEmergency(t *testing.T) {
	defer stubDiskUsage(doltGCDiskUsage{TotalBytes: 32 * gibBytes, UsedBytes: 32*gibBytes - (300 << 20), AvailBytes: 300 << 20})()
	dir := t.TempDir()
	storePath := filepath.Join(dir, "ws.db")
	if err := os.MkdirAll(resolveTextureWorkspacePath(storePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := MaybeRunDoltGC(dir, storePath); err != nil {
		t.Fatalf("emergency gc: %v", err)
	}
	if got := readDisposition(t, dir); got.Outcome != "ran" || !strings.HasPrefix(got.Detail, "emergency") {
		t.Fatalf("disposition = %+v, want an emergency run", got)
	}
}

// FM6: a noop is recorded too, and /health can read the record back.
func TestMaybeRunDoltGCNoopIsReadable(t *testing.T) {
	defer stubDiskUsage(smallDiskUsage())()
	dir := t.TempDir()
	storePath := filepath.Join(dir, "ws.db")
	if err := os.MkdirAll(resolveTextureWorkspacePath(storePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := MaybeRunDoltGC(dir, storePath); err != nil {
		t.Fatalf("noop gc: %v", err)
	}
	raw := ReadDoltGCDisposition(dir)
	if raw == nil {
		t.Fatal("ReadDoltGCDisposition returned nil after a noop")
	}
	var got doltGCDisposition
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Outcome != "noop" || got.ThresholdMiB != 1024 {
		t.Fatalf("disposition = %+v, want noop with the 1024 MiB threshold", got)
	}
	if ReadDoltGCDisposition(t.TempDir()) != nil {
		t.Fatal("want nil when no record exists")
	}
}

// FM8: the kill switch is honoured and recorded.
func TestMaybeRunDoltGCDisabled(t *testing.T) {
	t.Setenv("RUNTIME_DOLT_GC_DISABLED", "1")
	defer stubDiskUsage(doltGCDiskUsage{TotalBytes: 32 * gibBytes, AvailBytes: 0, UsedBytes: 32 * gibBytes})()
	dir := t.TempDir()
	if err := MaybeRunDoltGC(dir, filepath.Join(dir, "ws.db")); err != nil {
		t.Fatal(err)
	}
	if got := readDisposition(t, dir); got.Outcome != "skipped_disabled" {
		t.Fatalf("disposition = %+v, want skipped_disabled", got)
	}
}

// FM5 and FM7: online GC on an open store collects the journal, keeps both
// sessions usable and every row, and flags low space it could not fix.
func TestOnlineDoltGCKeepsStoreUsable(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	dir := t.TempDir()
	writeProbeRows(t, s, 0, 400)
	if err := s.commitDoltCheckpointForTest(ctx); err != nil {
		t.Fatalf("checkpoint: %v", err)
	}
	writeProbeRows(t, s, 400, 800) // uncommitted working set must survive too
	var headBefore string
	if err := s.textureHandle().QueryRowContext(ctx, "SELECT HASHOF('HEAD')").Scan(&headBefore); err != nil {
		t.Fatalf("head: %v", err)
	}
	journalBefore := doltJournalBytes(s.texturePath)
	if journalBefore == 0 {
		t.Fatal("expected a non-empty journal before gc")
	}

	// The test journal is a few MiB, under the 64 MiB floor, so stage the
	// emergency line to make this tick collect. Free space stays under 25% of
	// the disk after the GC, so the record says compaction_needed.
	defer stubDiskUsage(doltGCDiskUsage{TotalBytes: 48 * gibBytes, UsedBytes: 48*gibBytes - (400 << 20), AvailBytes: 400 << 20})()
	if err := s.MaybeRunOnlineDoltGC(ctx, dir); err != nil {
		t.Fatalf("online gc: %v", err)
	}
	got := readDisposition(t, dir)
	if got.Outcome != "ran" || got.Mode != "online" || !got.CompactionNeeded {
		t.Fatalf("disposition = %+v, want online ran with compaction_needed", got)
	}
	if after := doltJournalBytes(s.texturePath); after >= journalBefore {
		t.Fatalf("journal %d -> %d bytes, want it collected", journalBefore, after)
	}

	var headAfter string
	if err := s.textureHandle().QueryRowContext(ctx, "SELECT HASHOF('HEAD')").Scan(&headAfter); err != nil {
		t.Fatalf("write session after gc: %v", err)
	}
	if headAfter != headBefore {
		t.Fatalf("HEAD %s -> %s; gc must not move history", headBefore, headAfter)
	}
	assertProbeCount(t, s, 800)
	writeProbeRows(t, s, 800, 900)
	if err := s.commitDoltCheckpointForTest(ctx); err != nil {
		t.Fatalf("checkpoint after gc: %v", err)
	}
	assertProbeCount(t, s, 900)
}

// FM1: the periodic loop actually ticks on a live ctx and stops on cancel.
func TestStartPeriodicDoltGCTicks(t *testing.T) {
	s := openTestStore(t)
	writeProbeRows(t, s, 0, 50)
	dir := t.TempDir()
	defer stubDiskUsage(doltGCDiskUsage{TotalBytes: 32 * gibBytes, UsedBytes: 32*gibBytes - (100 << 20), AvailBytes: 100 << 20})()
	ctx, cancel := context.WithCancel(context.Background())
	done := StartPeriodicDoltGC(ctx, s, dir, 20*time.Millisecond)
	// Wait the loop out before the deferred stub restore and the store close:
	// a pass in flight at cancel still reads diskUsageForGC.
	stop := func() {
		cancel()
		select {
		case <-done:
		case <-time.After(30 * time.Second):
			t.Fatal("periodic gc loop did not exit after cancel")
		}
	}
	deadline := time.Now().Add(30 * time.Second)
	for {
		if raw := ReadDoltGCDisposition(dir); raw != nil {
			var got doltGCDisposition
			if err := json.Unmarshal(raw, &got); err == nil && got.Mode == "online" && got.Outcome == "ran" {
				break
			}
		}
		if time.Now().After(deadline) {
			stop()
			t.Fatal("periodic gc never ran an online collection")
		}
		time.Sleep(20 * time.Millisecond)
	}
	stop()
	assertProbeCount(t, s, 50)
}

func (s *Store) commitDoltCheckpointForTest(ctx context.Context) error {
	s.markDoltHistoryDirty()
	return s.commitDoltCheckpoint(ctx, "gc probe checkpoint")
}

func writeProbeRows(t *testing.T, s *Store, from, to int) {
	t.Helper()
	ctx := context.Background()
	if _, err := s.textureHandle().ExecContext(ctx, "CREATE TABLE IF NOT EXISTS gc_probe (id INT PRIMARY KEY, body LONGTEXT NOT NULL)"); err != nil {
		t.Fatalf("create probe table: %v", err)
	}
	pad := strings.Repeat("x", 2048)
	for i := from; i < to; i++ {
		// One transaction per row: per-mutation journal churn, like the paper's.
		if _, err := s.textureHandle().ExecContext(ctx, "INSERT INTO gc_probe (id, body) VALUES (?, ?)", i, fmt.Sprintf("%d-%s", i, pad)); err != nil {
			t.Fatalf("insert probe row %d: %v", i, err)
		}
	}
}

func assertProbeCount(t *testing.T, s *Store, want int) {
	t.Helper()
	ctx := context.Background()
	var write, read int
	if err := s.textureHandle().QueryRowContext(ctx, "SELECT COUNT(*) FROM gc_probe").Scan(&write); err != nil {
		t.Fatalf("count via write session: %v", err)
	}
	if err := s.readDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM gc_probe").Scan(&read); err != nil {
		t.Fatalf("count via read session: %v", err)
	}
	if write != want || read != want {
		t.Fatalf("probe rows write=%d read=%d, want %d", write, read, want)
	}
}

func sparseFile(t *testing.T, path string, size int64) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(path, size); err != nil {
		t.Fatal(err)
	}
}

func stubDiskUsage(usage doltGCDiskUsage) func() {
	previous := diskUsageForGC
	diskUsageForGC = func(string) (doltGCDiskUsage, error) { return usage, nil }
	return func() { diskUsageForGC = previous }
}

func smallDiskUsage() doltGCDiskUsage {
	return doltGCDiskUsage{TotalBytes: 32 * gibBytes, UsedBytes: 512 << 20, AvailBytes: 32*gibBytes - (512 << 20)}
}

func readDisposition(t *testing.T, dir string) doltGCDisposition {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, doltGCDispositionFileName))
	if err != nil {
		t.Fatalf("disposition missing: %v", err)
	}
	var got doltGCDisposition
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode disposition: %v", err)
	}
	return got
}
