package vmctl

import (
	"path/filepath"
	"testing"
)

// A new account's computer has 4 GiB of memory (owner, 2026-11-24: "we may not
// need to cut computers from 4gb ram to 2gb since ram isnt the limit, disk
// is"). The size is recorded on
// the ownership when the computer is created, so a computer made before that
// keeps the interactive size it was made with (its snapshot depends on it),
// and the paper and a platform computer never take the small shape.

func newAccountFixture(t *testing.T, policy WarmnessPolicyConfig) (*OwnershipRegistry, *mockVMManager, string) {
	t.Helper()
	t.Setenv(interactiveVMMemSizeMibEnv, "8192")
	t.Setenv(interactiveVMCPUCountEnv, "4")
	path := filepath.Join(t.TempDir(), "ownerships.json")
	mgr := &mockVMManager{}
	reg := NewOwnershipRegistry("")
	reg.SetCorpusdURL(testComputerCredentialIssuerURL(t))
	reg.SetWarmnessPolicyConfig(policy)
	if err := reg.SetPersistencePath(path); err != nil {
		t.Fatal(err)
	}
	reg.SetVMManager(mgr)
	return reg, mgr, path
}

func TestNewAccountComputerGetsFourGiBAndKeepsItAcrossRestarts(t *testing.T) {
	reg, mgr, path := newAccountFixture(t, DefaultWarmnessPolicyConfig())
	own, err := reg.ResolveOrAssign("a-new-stranger")
	if err != nil {
		t.Fatal(err)
	}
	if len(mgr.boots) != 1 || mgr.boots[0].MachineMemSizeMib != 4096 {
		t.Fatalf("first boot memory = %+v, want one boot at 4096 MiB", mgr.boots)
	}
	if own.MemSizeMib != 4096 {
		t.Fatalf("recorded size = %d, want 4096", own.MemSizeMib)
	}
	// Every later boot of this computer is 4096 too, after a vmctl restart.
	again := NewOwnershipRegistry("")
	if err := again.SetPersistencePath(path); err != nil {
		t.Fatal(err)
	}
	again.mu.RLock()
	loaded := again.vmByID[own.VMID]
	again.mu.RUnlock()
	if loaded == nil {
		t.Fatal("ownership did not persist")
	}
	if _, mem := machineShapeForOwnership(loaded); mem != 4096 {
		t.Fatalf("size after restart = %d, want 4096", mem)
	}
	if got := vmManagerConfigForOwnership(loaded, "").MachineMemSizeMib; got != 4096 {
		t.Fatalf("boot config after restart = %d, want 4096", got)
	}
}

func TestComputerMadeBeforeTheSmallShapeKeepsItsSize(t *testing.T) {
	// An ownership written by an older vmctl has no recorded size: the
	// owner's and every existing computer's shape is what the host's
	// interactive setting says, as before.
	t.Setenv(interactiveVMMemSizeMibEnv, "8192")
	legacy := &VMOwnership{VMID: "vm-legacy", UserID: "owner", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive}
	if _, mem := machineShapeForOwnership(legacy); mem != 8192 {
		t.Fatalf("existing computer = %d MiB, want 8192", mem)
	}
	if got := vmManagerConfigForOwnership(legacy, "").MachineMemSizeMib; got != 8192 {
		t.Fatalf("existing computer boot config = %d, want 8192", got)
	}
	// A platform computer keeps its own size.
	platform := &VMOwnership{VMID: "vm-platform", UserID: "u", DesktopID: PrimaryDesktopID, WarmnessClass: WarmnessClassPublicPlatform}
	if _, mem := machineShapeForOwnership(platform); mem != platformVMMemSizeMib {
		t.Fatalf("platform computer = %d MiB, want %d", mem, platformVMMemSizeMib)
	}
}

func TestPaperComputerIsNeverGivenTheSmallShape(t *testing.T) {
	// If the paper's ownership were ever recreated, it must not come back at
	// 4 GiB: a computer the tiers name as a platform computer is exempt.
	paperUser := "paper-owner"
	paperComputer := stableComputerID(paperUser, PrimaryDesktopID, "")
	policy := DefaultWarmnessPolicyConfig()
	policy.PlatformComputerIDs = map[string]bool{paperComputer: true}
	reg, mgr, _ := newAccountFixture(t, policy)
	own, err := reg.ResolveOrAssign(paperUser)
	if err != nil {
		t.Fatal(err)
	}
	if own.MemSizeMib != 0 {
		t.Fatalf("paper recorded size = %d, want none", own.MemSizeMib)
	}
	if len(mgr.boots) != 1 || mgr.boots[0].MachineMemSizeMib != 8192 {
		t.Fatalf("paper boot memory = %+v, want 8192", mgr.boots)
	}
}

func TestNewAccountShapeNeverExceedsTheHostSetting(t *testing.T) {
	t.Setenv(interactiveVMMemSizeMibEnv, "1024")
	if got := newComputerMemSizeMib(WarmnessClassPrimary, false); got != 1024 {
		t.Fatalf("new account size with a 1024 host setting = %d, want 1024", got)
	}
	if got := newComputerMemSizeMib(WarmnessClassCriticalProtected, false); got != 0 {
		t.Fatalf("protected computer got a recorded size %d", got)
	}
}

// The smaller shape is for strangers. The owner's paid always-on computer, the
// operator proof computer and a test computer keep the interactive size when
// their ownership is created afresh (review of 2026-11-24, S3).
func TestOwnerAndProofComputersKeepTheirSize(t *testing.T) {
	cases := []struct {
		name  string
		user  string
		setup func(p *WarmnessPolicyConfig, userID, computerID string)
	}{
		{name: "paid owner", user: "owner-user", setup: func(p *WarmnessPolicyConfig, u, _ string) { p.PaidUserIDs[u] = true }},
		{name: "operator proof computer", user: "proof-user", setup: func(p *WarmnessPolicyConfig, _, c string) { p.OperatorProofComputerIDs = map[string]bool{c: true} }},
		{name: "test computer", user: "test-user", setup: func(p *WarmnessPolicyConfig, _, c string) { p.TestComputerIDs = map[string]bool{c: true} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			policy := DefaultWarmnessPolicyConfig()
			if tc.setup != nil {
				tc.setup(&policy, tc.user, stableComputerID(tc.user, PrimaryDesktopID, ""))
			}
			reg, mgr, _ := newAccountFixture(t, policy)
			own, err := reg.ResolveOrAssign(tc.user)
			if err != nil {
				t.Fatal(err)
			}
			if own.MemSizeMib != 0 || len(mgr.boots) != 1 || mgr.boots[0].MachineMemSizeMib != 8192 {
				t.Fatalf("recorded %d, boots %+v; want no recorded size and one 8192 boot", own.MemSizeMib, mgr.boots)
			}
		})
	}
}
