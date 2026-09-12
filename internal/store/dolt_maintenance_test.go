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

func TestPlanDoltGC_EmergencyLowAvail(t *testing.T) {
	usage := doltGCDiskUsage{
		TotalBytes: 8 * gibBytes,
		UsedBytes:  8*gibBytes - (300 << 20),
		AvailBytes: 300 << 20,
	}
	plan := planDoltGC(usage, 7, 1)
	if !plan.Run {
		t.Fatal("expected emergency gc below 512 MiB free")
	}
	if plan.Reason == "" {
		t.Fatal("expected emergency reason")
	}
}

func TestWriteDoltGCDispositionRoundTrip(t *testing.T) {
	dir := t.TempDir()
	writeDoltGCDisposition(dir, doltGCDisposition{Outcome: "skipped_size", UsedGiB: 11, ThresholdGiB: 5, Detail: "x"})
	raw, err := os.ReadFile(filepath.Join(dir, doltGCDispositionFileName))
	if err != nil {
		t.Fatalf("read disposition: %v", err)
	}
	var got doltGCDisposition
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode disposition: %v", err)
	}
	if got.Outcome != "skipped_size" || got.UsedGiB != 11 || got.ThresholdGiB != 5 || got.At == "" {
		t.Fatalf("disposition = %+v", got)
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
	if err := os.MkdirAll(resolveTextureWorkspacePath(storePath), 0o755); err != nil {
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
