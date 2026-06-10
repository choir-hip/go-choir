package store

import "testing"

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
