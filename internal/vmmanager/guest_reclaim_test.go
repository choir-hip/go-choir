package vmmanager

import (
	"strings"
	"testing"
)

// Failure list (W3): the DAMON_RECLAIM boot arguments, VM_GUEST_RECLAIM_ONLY
// and its min-age variable are gone. The experiment measured negative (ledger
// 2026-10-22: no host memory returned in the idle window) and the owner ruled
// it deleted; the active mechanism is the cache return in internal/vmctl.
//  1. TestNoDamonBootArgsWhateverTheEnvironment: a host that still carries the
//     old variables (selecting every computer) must launch a guest whose boot
//     arguments hold no damon_reclaim parameter, for any computer.
func TestNoDamonBootArgsWhateverTheEnvironment(t *testing.T) {
	t.Setenv("VM_GUEST_RECLAIM_ONLY", "*")
	t.Setenv("VM_GUEST_RECLAIM_MIN_AGE_S", "120")
	cfg := LoadConfigFromEnv()
	cfg.StateDir = t.TempDir()
	cfg.KernelImagePath = "/opt/go-choir/guest/vmlinux"
	cfg.RootfsPath = "/opt/go-choir/guest/rootfs.ext4"
	mgr := NewManager(cfg)

	selected := balloonTestVMConfig()
	selected.WarmnessClass = "test"
	for _, vm := range []VMConfig{selected, balloonTestVMConfig()} {
		if args := bootArgsOf(t, mgr.buildFirecrackerConfig(vm, 9001)); strings.Contains(args, "damon_reclaim") {
			t.Fatalf("boot args carry damon_reclaim: %s", args)
		}
	}
}
