package vmctl

import (
	"sort"
	"testing"
)

// Computer tiers (owner ruling 2026-10-13,
// docs/archive/problems/live-paper-computer-reclaimed-no-computer-tiers-2026-10-13.md).
// Ways this could fail, written before the code: a downgraded account keeps
// a persisted always-on class; the platform computer is reclaimed or idle
// hibernated; a hibernated platform computer is never woken; a paid
// account's computer is woken while held; the old env list keeps working.

const paperComputerID = "computer-1355b83bced1613eb048ee28d66688a8"

func tierConfig() WarmnessPolicyConfig {
	return WarmnessPolicyConfig{
		PaidUserIDs:         map[string]bool{"paid-user": true},
		PlatformComputerIDs: map[string]bool{paperComputerID: true},
	}
}

func TestPlatformComputerIsCriticalProtectedAboveEveryUserTier(t *testing.T) {
	cfg := tierConfig()
	paper := &VMOwnership{UserID: "paper-owner", ComputerID: paperComputerID, DesktopID: PrimaryDesktopID, Kind: VMKindInteractive}
	if got := warmnessClassForOwnership(paper, cfg); got != WarmnessClassCriticalProtected {
		t.Fatalf("paper computer class = %s, want %s", got, WarmnessClassCriticalProtected)
	}
	paid := &VMOwnership{UserID: "paid-user", ComputerID: "computer-paid", DesktopID: PrimaryDesktopID}
	if got := warmnessClassForOwnership(paid, cfg); got != WarmnessClassPremiumAlwaysOn {
		t.Fatalf("paid computer class = %s, want %s", got, WarmnessClassPremiumAlwaysOn)
	}
	if warmnessPriority(WarmnessClassCriticalProtected) <= warmnessPriority(WarmnessClassPremiumAlwaysOn) {
		t.Fatal("the platform tier must rank above the paid tier")
	}
	reasons := protectedReclaimReasons(paper, PressureReclaimConfig{}, cfg, 0, nil)
	if len(reasons) == 0 || reasons[0] != "critical_protected" {
		t.Fatalf("paper computer reclaim protection = %v", reasons)
	}
}

func TestDowngradedAccountLosesAlwaysOnDespiteAPersistedClass(t *testing.T) {
	cfg := tierConfig()
	// Stamped premium when the account was paid; the account is free now.
	own := &VMOwnership{UserID: "former-paid", ComputerID: "computer-x", DesktopID: PrimaryDesktopID, WarmnessClass: WarmnessClassPremiumAlwaysOn}
	if got := warmnessClassForOwnership(own, cfg); got != WarmnessClassPrimary {
		t.Fatalf("downgraded account class = %s, want %s", got, WarmnessClassPrimary)
	}
	// A computer-level class persisted on the ownership is still honoured.
	platform := &VMOwnership{UserID: "u", ComputerID: "computer-y", DesktopID: PrimaryDesktopID, WarmnessClass: WarmnessClassCriticalProtected}
	if got := warmnessClassForOwnership(platform, cfg); got != WarmnessClassCriticalProtected {
		t.Fatalf("persisted platform class = %s", got)
	}
}

func TestAlwaysOnWarmTargetsArePaidAndPlatformComputersOnly(t *testing.T) {
	reg, _ := hostCapacityRegistry(t, newProcessControlVMManager())
	reg.SetWarmnessPolicyConfig(tierConfig())
	addOwnership(reg, &VMOwnership{VMID: "vm-paper", UserID: "paper-owner", ComputerID: paperComputerID, DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateHibernated})
	addOwnership(reg, &VMOwnership{VMID: "vm-paid", UserID: "paid-user", ComputerID: "computer-paid", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateStopped})
	addOwnership(reg, &VMOwnership{VMID: "vm-free", UserID: "free-user", ComputerID: "computer-free", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateHibernated})
	addOwnership(reg, &VMOwnership{VMID: "vm-paid-active", UserID: "paid-user", ComputerID: "computer-paid-2", DesktopID: "second", Kind: VMKindInteractive, State: VMStateActive})
	held := &VMOwnership{VMID: "vm-paid-held", UserID: "paid-user", ComputerID: "computer-paid-3", DesktopID: "third", Kind: VMKindInteractive, State: VMStateStopped}
	held.HoldStatus = &MaintenanceHold{Reason: "maintenance", HeldBy: "test"}
	addOwnership(reg, held)

	var got []string
	for _, target := range reg.alwaysOnWarmTargets() {
		got = append(got, target.vmID)
	}
	sort.Strings(got)
	if len(got) != 2 || got[0] != "vm-paid" || got[1] != "vm-paper" {
		t.Fatalf("warm targets = %v, want [vm-paid vm-paper]", got)
	}
}

func TestNewOwnershipPersistsNoAccountDerivedClass(t *testing.T) {
	reg, _ := hostCapacityRegistry(t, newProcessControlVMManager())
	reg.SetWarmnessPolicyConfig(tierConfig())
	if _, err := reg.ResolveOrAssign("paid-user"); err != nil {
		t.Fatalf("resolve paid-user: %v", err)
	}
	reg.mu.RLock()
	defer reg.mu.RUnlock()
	for _, own := range reg.ownerships {
		if own.UserID == "paid-user" && own.WarmnessClass == WarmnessClassPremiumAlwaysOn {
			t.Fatal("the paid tier was persisted on the ownership; a downgrade would not take effect")
		}
	}
}
