package vmmanager

import (
	"strings"
	"testing"
)

// Failure list (W3): the DAMON_RECLAIM boot arguments, VM_GUEST_RECLAIM_ONLY
// and its min-age variable are gone. The experiment measured negative (ledger
// 2026-10-22: no host memory returned in the idle window) and the owner ruled
// it deleted; the active mechanism is the cache return in internal/vmctl.
//  1. A host that still carries the old variables (selecting every computer)
//     must launch a guest whose boot arguments hold no damon_reclaim
//     parameter, for a formerly selected computer and an unselected one, on
//     the microvm branch real hosts take (StoreDiskPath and kernel params set)
//     and on the legacy branch.
//  2. The variables change nothing else: with them set, every computer's boot
//     arguments are byte-identical to those with them unset, which pins that
//     only the two formerly named computers' launch hashes moved (they lose
//     the six arguments); the golden launch hash (TestLaunchConfigGolden) pins
//     the rest.
//  3. The arguments are not empty: the guest still gets its network and
//     service URLs.
func TestNoDamonBootArgsWhateverTheEnvironment(t *testing.T) {
	build := func(t *testing.T, withOldEnv bool) *Manager {
		if withOldEnv {
			t.Setenv("VM_GUEST_RECLAIM_ONLY", "*")
			t.Setenv("VM_GUEST_RECLAIM_MIN_AGE_S", "120")
		}
		cfg := LoadConfigFromEnv()
		cfg.StateDir = t.TempDir()
		cfg.KernelImagePath = "/opt/go-choir/guest/vmlinux"
		cfg.RootfsPath = "/opt/go-choir/guest/rootfs.ext4"
		return NewManager(cfg)
	}
	selected := balloonTestVMConfig()
	selected.WarmnessClass = "test"
	other := balloonTestVMConfig()
	other.VMID = "vm-other"
	for name, branch := range map[string]func(VMConfig) VMConfig{
		"microvm": func(c VMConfig) VMConfig {
			c.StoreDiskPath = "/var/lib/go-choir/store.img"
			c.KernelParams = "root=fstab init=/nix/store/x-init regInfo=/nix/store/x-reg"
			return c
		},
		"legacy": func(c VMConfig) VMConfig { return c },
	} {
		t.Run(name, func(t *testing.T) {
			clean := build(t, false)
			old := build(t, true)
			for _, vm := range []VMConfig{branch(selected), branch(other)} {
				want := bootArgsOf(t, clean.buildFirecrackerConfig(vm, 9001))
				got := bootArgsOf(t, old.buildFirecrackerConfig(vm, 9001))
				if strings.Contains(got, "damon_reclaim") {
					t.Fatalf("boot args carry damon_reclaim: %s", got)
				}
				if got != want {
					t.Fatalf("the retired variables changed %s's boot args:\n got %s\nwant %s", vm.VMID, got, want)
				}
				for _, p := range []string{"ip=", "choir.gateway_url="} {
					if !strings.Contains(got, p) {
						t.Fatalf("boot args lack %s: %s", p, got)
					}
				}
			}
		})
	}
}
