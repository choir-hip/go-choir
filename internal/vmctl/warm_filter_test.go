package vmctl

import (
	"context"
	"sort"
	"testing"
	"time"
)

// The always-on sweep runs every two minutes. Ways it could loop or starve,
// written before the bound:
//   - a wedged computer is stopped, warmed, wedges again: a loop every ~7 min;
//   - a computer whose recovery failed is warmed into the same failure;
//   - the bound is too tight and a transient wedge strands an always-on
//     computer that nobody opens (the paper computer), against the
//     2026-10-10 ruling that the owner never has to boot a wedged computer;
//   - the bound is too wide and a host act (hibernation, vmctl restart)
//     keeps an always-on computer down.
func warmTargetIDs(reg *OwnershipRegistry) []string {
	var got []string
	for _, target := range reg.alwaysOnWarmTargets() {
		got = append(got, target.vmID)
	}
	sort.Strings(got)
	return got
}

func TestAlwaysOnSweepWarmsAFailedComputerOnceThenHolds(t *testing.T) {
	reg, _ := hostCapacityRegistry(t, newProcessControlVMManager())
	now := time.Date(2026, 10, 14, 1, 0, 0, 0, time.UTC)
	reg.clock = func() time.Time { return now }
	reg.SetWarmnessPolicyConfig(tierConfig())
	addOwnership(reg, &VMOwnership{VMID: "vm-paper", UserID: "paper-owner", ComputerID: paperComputerID, DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateStopped, StoppedBy: stoppedByWedged})

	if got := warmTargetIDs(reg); len(got) != 1 {
		t.Fatalf("first failure: warm targets = %v, want the one automatic retry", got)
	}
	// The sweep records the retry before it boots.
	reg.WarmAlwaysOnDesktops(t.Context(), func(_ context.Context, _, _ string) error { return nil })
	reg.mu.Lock()
	own := reg.vmByID["vm-paper"]
	if own == nil || own.FailureWarmedAt == "" {
		reg.mu.Unlock()
		t.Fatal("the automatic retry was not recorded")
	}
	// It wedges again.
	own.State, own.StoppedBy = VMStateStopped, stoppedByWedged
	reg.mu.Unlock()

	now = now.Add(2 * time.Hour)
	if got := warmTargetIDs(reg); len(got) != 0 {
		t.Fatalf("second failure within the window: warm targets = %v, want none", got)
	}
	now = now.Add(failureWarmWindow)
	if got := warmTargetIDs(reg); len(got) != 1 {
		t.Fatalf("after the window: warm targets = %v, want one retry", got)
	}
}

func TestAlwaysOnSweepStillWarmsComputersStoppedByAHostAct(t *testing.T) {
	reg, _ := hostCapacityRegistry(t, newProcessControlVMManager())
	cfg := tierConfig()
	cfg.PaidUserIDs["paid-user-2"] = true
	reg.SetWarmnessPolicyConfig(cfg)
	recent := time.Now().UTC().Format(time.RFC3339Nano)
	// A failure retry spent recently does not block a later host-act stop.
	addOwnership(reg, &VMOwnership{VMID: "vm-paid-restart", UserID: "paid-user", ComputerID: "computer-paid", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateStopped, StoppedBy: "vmctl-restart", FailureWarmedAt: recent})
	addOwnership(reg, &VMOwnership{VMID: "vm-paid-hibernated", UserID: "paid-user-2", ComputerID: "computer-paid-2", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateHibernated})
	addOwnership(reg, &VMOwnership{VMID: "vm-paper-failed", UserID: "paper-owner", ComputerID: paperComputerID, DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateStopped, StoppedBy: stoppedByRecoveryFailed, FailureWarmedAt: recent})

	got := warmTargetIDs(reg)
	if len(got) != 2 || got[0] != "vm-paid-hibernated" || got[1] != "vm-paid-restart" {
		t.Fatalf("warm targets = %v, want [vm-paid-hibernated vm-paid-restart]", got)
	}
}
