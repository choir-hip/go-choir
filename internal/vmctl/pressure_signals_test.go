package vmctl

import (
	"reflect"
	"testing"
	"time"
)

// Reclaim answers memory and disk pressure only
// (docs/archive/reports/resource-efficiency-memory-assessment-2026-10-13.md, R3).
// Ways this could fail, written before the code: a CPU or IO stall spike
// hibernates an idle computer, which relieves neither and crash-boots it
// on wake; a memory shortage stops being answered; a decision cannot be
// attributed to the signal that fired it; the idle sweep's primary
// keepalive still drops on a stall spike.

func signalRegistry(t *testing.T, sample HostPressureSample) *OwnershipRegistry {
	t.Helper()
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.setGuestBusyProbeForTest(func(*VMOwnership) bool { return false })
	reg.SetPressureReclaimConfig(PressureReclaimConfig{
		Mode: PressureReclaimModeActive,
		// These pin how one sample is classified; the sustained window over
		// samples is pinned in pressure_sustained_test.go.
		Signal:                    PressureReclaimSignalInstant,
		MinIdle:                   10 * time.Minute,
		MinMemoryAvailableBytes:   1024,
		MinMemoryAvailablePercent: 10,
		MaxMemorySomeAvg10:        1,
		MaxCPUSomeAvg10:           90,
		MaxIOSomeAvg10:            5,
	})
	reg.setPressureSamplerForTest(func(PressureReclaimConfig) HostPressureSample { return sample })
	if _, err := reg.ResolveOrAssign("idle-user"); err != nil {
		t.Fatalf("resolve idle-user: %v", err)
	}
	reg.mu.Lock()
	reg.ownerships[ownershipKey("idle-user", PrimaryDesktopID)].LastActiveAt = time.Now().Add(-2 * time.Hour)
	reg.mu.Unlock()
	return reg
}

func ampleMemory() HostPressureSample {
	return HostPressureSample{
		SampledAt:              "2026-10-13T21:00:10Z",
		MemoryTotalBytes:       32 << 30,
		MemoryAvailableBytes:   20 << 30,
		MemoryAvailablePercent: 62,
	}
}

func TestStallSpikeAloneIsReportedButNeverReclaims(t *testing.T) {
	for name, sample := range map[string]HostPressureSample{
		"io":  func() HostPressureSample { s := ampleMemory(); s.IOSomeAvg10 = 40; return s }(),
		"cpu": func() HostPressureSample { s := ampleMemory(); s.CPUSomeAvg10 = 99; return s }(),
	} {
		reg := signalRegistry(t, sample)
		plan := reg.PressureReclaimPlan()
		if !plan.Pressure.Pressure {
			t.Fatalf("%s: the stall is not reported as pressure", name)
		}
		if plan.Pressure.ReclaimPressure {
			t.Fatalf("%s: a stall alone counts as reclaim pressure", name)
		}
		if plan.Decision != "observe" || plan.Reason != "cpu or io stall reported; reclaim answers memory and disk pressure only" {
			t.Fatalf("%s: decision = %s (%s), want observe naming the stall", name, plan.Decision, plan.Reason)
		}
		if !reflect.DeepEqual(plan.Pressure.Signals, []string{name}) {
			t.Fatalf("%s: signals = %v", name, plan.Pressure.Signals)
		}
		if got := reg.pressureReclaimActionCandidates(); len(got) != 0 {
			t.Fatalf("%s: %d reclaim candidates on a stall spike", name, len(got))
		}
	}
}

func TestMemoryShortageStillReclaimsAndNamesItsSignal(t *testing.T) {
	sample := ampleMemory()
	sample.MemoryAvailableBytes = 512
	sample.MemoryAvailablePercent = 2
	sample.IOSomeAvg10 = 40
	reg := signalRegistry(t, sample)
	plan := reg.PressureReclaimPlan()
	if plan.Decision != "reclaim" || !plan.Pressure.ReclaimPressure {
		t.Fatalf("decision = %s reclaim_pressure = %v, want reclaim", plan.Decision, plan.Pressure.ReclaimPressure)
	}
	if !reflect.DeepEqual(plan.Pressure.Signals, []string{"memory", "io"}) {
		t.Fatalf("signals = %v, want [memory io]", plan.Pressure.Signals)
	}
	if got := reg.pressureReclaimActionCandidates(); len(got) != 1 {
		t.Fatalf("candidates = %d, want 1", len(got))
	}
}
