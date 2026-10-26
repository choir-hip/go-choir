package vmmanager

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// goldenLaunchManager is a manager with fixed, fake artifact paths: nothing
// here touches the host.
func goldenLaunchManager() (*Manager, VMConfig) {
	cfg := DefaultManagerConfig()
	cfg.StateDir = "/var/lib/go-choir/vm-state"
	cfg.KernelImagePath = "/var/lib/go-choir/guest/vmlinux"
	cfg.InitrdPath = "/var/lib/go-choir/guest/initrd"
	cfg.RootfsPath = "/var/lib/go-choir/guest/rootfs.ext4"
	cfg.StoreDiskPath = "/var/lib/go-choir/guest/storedisk.erofs"
	cfg.KernelParams = "root=fstab init=/nix/store/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-nixos-system/init regInfo=/nix/store/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb-reginfo/registration"
	mgr := NewManager(cfg)
	vmCfg := VMConfig{
		VMID: "vm-golden", ComputerID: "computer-golden", RealizationID: "real-1", OwnerID: "owner", DesktopID: "primary",
		Epoch: 3, GuestPort: 8085, MachineCPUCount: 2, MachineMemSizeMib: 512, GatewayToken: "computer-golden:secret",
	}
	mgr.applyLaunchDefaults(&vmCfg)
	return mgr, vmCfg
}

// D2 S1b test 4 (and S3 test 1): outside the switch window the Firecracker
// config and its launch hash are byte-identical to the ones recorded from
// the code before the window repair, so the repair voids no snapshot that
// that code would keep. The golden file was written by that code
// (UPDATE_GOLDEN=1 go test -run TestLaunchConfigGolden ./internal/vmmanager).
func TestLaunchConfigGolden(t *testing.T) {
	mgr, vmCfg := goldenLaunchManager()
	fc := mgr.buildFirecrackerConfig(vmCfg, 9001)
	raw, err := json.MarshalIndent(fc, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	hash, err := launchConfigHash(fc)
	if err != nil {
		t.Fatal(err)
	}
	got := append(raw, '\n')
	got = append(got, []byte("launch_config_sha256="+hash+"\n")...)
	path := filepath.Join("testdata", "launch_config_golden.txt")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(want) != string(got) {
		t.Fatalf("the launch config moved from the golden file %s.\nwant:\n%s\ngot:\n%s", path, want, got)
	}
}
