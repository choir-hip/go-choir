package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestPlanDoltGC_MilestoneCrossing(t *testing.T) {
	usage := doltGCDiskUsage{
		TotalBytes: 8 * gibBytes,
		UsedBytes:  2*gibBytes + 100,
		AvailBytes: 6*gibBytes - 100,
	}
	plan := planDoltGC(usage, 1, 1)
	if !plan.Run {
		t.Fatal("expected gc at 2 GiB milestone")
	}
	if plan.TargetMilestone != 2 {
		t.Fatalf("target milestone = %d, want 2", plan.TargetMilestone)
	}
	if plan.Warning {
		t.Fatal("did not expect warning below 7 GiB")
	}
}

func TestPlanDoltGC_SkipsUntilNextMilestone(t *testing.T) {
	usage := doltGCDiskUsage{
		TotalBytes: 8 * gibBytes,
		UsedBytes:  2*gibBytes + 100,
		AvailBytes: 6*gibBytes - 100,
	}
	plan := planDoltGC(usage, 2, 1)
	if plan.Run {
		t.Fatal("expected no gc when milestone unchanged")
	}
}

func TestPlanDoltGC_WarningAtSevenGiB(t *testing.T) {
	usage := doltGCDiskUsage{
		TotalBytes: 8 * gibBytes,
		UsedBytes:  7*gibBytes + 200,
		AvailBytes: gibBytes - 200,
	}
	plan := planDoltGC(usage, 6, 1)
	if !plan.Run || !plan.Warning {
		t.Fatalf("plan = %+v, want run+warning when crossing 7 GiB", plan)
	}
}

// Emergency GC must bypass the live-size guard: a huge store with almost no
// free space still attempts collection (ENOSPC is unrecoverable; OOM is not).
func TestMaybeRunDoltGCEmergencyBypassesSizeGuard(t *testing.T) {
	usage := doltGCDiskUsage{
		TotalBytes: 32 * gibBytes,
		UsedBytes:  20 * gibBytes,
		AvailBytes: 300 << 20,
	}
	defer stubDiskUsage(usage)()
	dir := t.TempDir()
	storePath := filepath.Join(dir, "ws.db")
	if err := os.MkdirAll(resolveTextureWorkspacePath(storePath), 0o755); err != nil {
		t.Fatal(err)
	}
	// GC on an empty workspace is a no-op success — proof the emergency path
	// ran rather than skipping on store size.
	if err := MaybeRunDoltGC(dir, storePath); err != nil {
		t.Fatalf("emergency gc: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, doltGCDispositionFileName))
	if err != nil {
		t.Fatalf("disposition missing: %v", err)
	}
	var got doltGCDisposition
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode disposition: %v", err)
	}
	if got.Outcome != "ran" {
		t.Fatalf("outcome = %q, want ran (emergency must bypass size guard)", got.Outcome)
	}
}

// The journal is collectible garbage: a large journal over a small live store
// must trigger GC, not trip the size guard (2026-09-11 feedback loop).
func TestMaybeRunDoltGCJournalTriggersAndBypassesGuard(t *testing.T) {
	usage := doltGCDiskUsage{
		TotalBytes: 32 * gibBytes,
		UsedBytes:  8 * gibBytes, // 6 GiB journal + 2 GiB live
		AvailBytes: 24 * gibBytes,
	}
	defer stubDiskUsage(usage)()
	dir := t.TempDir()
	storePath := filepath.Join(dir, "ws.db")
	nomsDir := filepath.Join(resolveTextureWorkspacePath(storePath), "texture", ".dolt", "noms")
	if err := os.MkdirAll(nomsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	journal := filepath.Join(nomsDir, doltJournalFileID)
	if err := os.WriteFile(journal, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(journal, 6*gibBytes); err != nil {
		t.Fatal(err)
	}
	if err := MaybeRunDoltGC(dir, storePath); err != nil {
		t.Fatalf("journal-triggered gc: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, doltGCDispositionFileName))
	if err != nil {
		t.Fatalf("disposition missing: %v", err)
	}
	var got doltGCDisposition
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode disposition: %v", err)
	}
	if got.Outcome == "skipped_size" {
		t.Fatalf("journal counted toward live-size guard: %+v", got)
	}
	if got.JournalGiB != 6 {
		t.Fatalf("journal_gib = %d, want 6", got.JournalGiB)
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

func TestMaybeRunDoltGCNoopWritesDisposition(t *testing.T) {
	defer stubDiskUsage(smallDiskUsage())()
	dir := t.TempDir()
	storePath := filepath.Join(dir, "ws.db")
	if err := os.MkdirAll(resolveTextureWorkspacePath(storePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := MaybeRunDoltGC(dir, storePath); err != nil {
		t.Fatalf("noop gc: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, doltGCDispositionFileName))
	if err != nil {
		t.Fatalf("disposition missing after noop: %v", err)
	}
	var got doltGCDisposition
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode disposition: %v", err)
	}
	if got.Outcome != "noop" {
		t.Fatalf("outcome = %q, want noop", got.Outcome)
	}
}

func TestMaybeRunDoltGCSkipWritesDisposition(t *testing.T) {
	big := smallDiskUsage()
	big.UsedBytes = 11 * gibBytes
	big.AvailBytes = 21 * gibBytes
	defer stubDiskUsage(big)()
	dir := t.TempDir()
	storePath := filepath.Join(dir, "ws.db")
	nomsDir := filepath.Join(resolveTextureWorkspacePath(storePath), "texture", ".dolt", "noms")
	if err := os.MkdirAll(nomsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	table := filepath.Join(nomsDir, "tablefile-live")
	if err := os.WriteFile(table, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(table, 11*gibBytes); err != nil {
		t.Fatal(err)
	}
	if err := MaybeRunDoltGC(dir, storePath); err != nil {
		t.Fatalf("skip gc: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, doltGCDispositionFileName))
	if err != nil {
		t.Fatalf("disposition missing after skip: %v", err)
	}
	var got doltGCDisposition
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode disposition: %v", err)
	}
	if got.Outcome != "skipped_size" || got.UsedGiB != 11 || got.ThresholdGiB != 5 {
		t.Fatalf("disposition = %+v, want skipped_size/11/5", got)
	}
}

// The size guard measures the Dolt store, not the disk. Failure modes pinned
// (docs/problems/guest-store-history-bloat-and-memory-shape-2026-10-09.md):
// updater releases and app files on the same disk counted as "live store"
// and suppressed GC on a compacted 1.7 GiB store; the journal counted as live;
// a large non-journal store not tripping the guard; a missing noms dir
// erroring instead of measuring zero.
func TestMaybeRunDoltGCGuardMeasuresStoreNotDisk(t *testing.T) {
	disk := doltGCDiskUsage{TotalBytes: 32 * gibBytes, UsedBytes: 12 * gibBytes, AvailBytes: 20 * gibBytes}
	defer stubDiskUsage(disk)()
	dir := t.TempDir()
	storePath := filepath.Join(dir, "ws.db")
	nomsDir := filepath.Join(resolveTextureWorkspacePath(storePath), "texture", ".dolt", "noms")
	if err := os.MkdirAll(nomsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sparse := func(name string, size int64) {
		t.Helper()
		path := filepath.Join(nomsDir, name)
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Truncate(path, size); err != nil {
			t.Fatal(err)
		}
	}
	sparse("tablefile-live", 1*gibBytes)

	if err := MaybeRunDoltGC(dir, storePath); err != nil {
		t.Fatalf("gc: %v", err)
	}
	got := readDisposition(t, dir)
	if got.Outcome == "skipped_size" || got.LiveGiB != 1 || got.UsedGiB != 12 {
		t.Fatalf("disposition = %+v, want not skipped, live 1 GiB, disk used 12 GiB", got)
	}

	sparse("tablefile-big", 6*gibBytes)
	if err := MaybeRunDoltGC(dir, storePath); err != nil {
		t.Fatalf("gc: %v", err)
	}
	if got := readDisposition(t, dir); got.Outcome != "skipped_size" || got.LiveGiB != 7 {
		t.Fatalf("disposition = %+v, want skipped_size at 7 GiB live", got)
	}
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
