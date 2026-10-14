//go:build linux

package vmbackend

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestRealBinaryConformance runs the conformance suite live against real VMM
// binaries. Opt-in: it needs Linux, /dev/kvm, a guest kernel, and (for
// networking) root and a pre-created tap.
//
//	CHOIR_VMBACKEND_REAL=1
//	CHOIR_VMBACKEND_KERNEL=/var/lib/go-choir/guest/vmlinux      (required)
//	CHOIR_VMBACKEND_INITRD=/var/lib/go-choir/guest/initrd       (optional)
//	CHOIR_VMBACKEND_STORE_DISK=/var/lib/go-choir/guest/storedisk.erofs (optional)
//	CHOIR_VMBACKEND_FIRECRACKER=/run/current-system/sw/bin/firecracker
//	CHOIR_VMBACKEND_CLOUD_HYPERVISOR=/nix/store/...-cloud-hypervisor-52.0/bin/cloud-hypervisor
//	CHOIR_VMBACKEND_TAP=vm-test0  (optional: an existing, up tap)
//
// The guest does not need to come up: the suite checks that the VMM accepts
// its arguments (alive after 2 s), is found by the /proc scan, reports
// metrics, and stops. The full guest boot is the staging test plan.
func TestRealBinaryConformance(t *testing.T) {
	if os.Getenv("CHOIR_VMBACKEND_REAL") != "1" {
		t.Skip("set CHOIR_VMBACKEND_REAL=1 on a KVM host to run real VMM binaries")
	}
	if _, err := os.Stat("/dev/kvm"); err != nil {
		t.Fatalf("/dev/kvm: %v", err)
	}
	kernel := os.Getenv("CHOIR_VMBACKEND_KERNEL")
	if kernel == "" {
		t.Fatal("CHOIR_VMBACKEND_KERNEL is required")
	}
	tapName := os.Getenv("CHOIR_VMBACKEND_TAP")

	type real struct {
		env     string
		backend Backend
	}
	ran := 0
	for _, r := range []real{
		{"CHOIR_VMBACKEND_FIRECRACKER", FirecrackerBackend{}},
		{"CHOIR_VMBACKEND_CLOUD_HYPERVISOR", CloudHypervisorBackend{}},
	} {
		bin := os.Getenv(r.env)
		if bin == "" {
			continue
		}
		if _, err := exec.LookPath(bin); err != nil {
			t.Fatalf("%s=%s: %v", r.env, bin, err)
		}
		ran++
		t.Run(string(r.backend.Name()), func(t *testing.T) {
			version, err := r.backend.Version(context.Background(), bin)
			if err != nil || version == "" {
				t.Fatalf("version: %q %v", version, err)
			}
			t.Logf("%s: %s", r.backend.Name(), version)
			// One tap handle per backend run: a single-queue tap can be
			// attached once, and the suite plans (never starts) a second VM.
			var tapFD *os.File
			if tapName != "" && r.backend.Caps().TapFD {
				fd, err := OpenTap(tapName)
				if err != nil {
					t.Fatalf("open tap %s: %v", tapName, err)
				}
				t.Cleanup(func() { fd.Close() })
				tapFD = fd
			}
			runConformance(t, conformanceEnv{
				backend:  r.backend,
				live:     true,
				aliveFor: 2 * time.Second,
				spec: func(t *testing.T, dir, vmID string) MachineSpec {
					return realSpec(t, bin, kernel, tapName, tapFD, dir, vmID)
				},
			})
		})
	}
	if ran == 0 {
		t.Fatal("set CHOIR_VMBACKEND_FIRECRACKER and/or CHOIR_VMBACKEND_CLOUD_HYPERVISOR")
	}
}

func realSpec(t *testing.T, bin, kernel, tapName string, tapFD *os.File, dir, vmID string) MachineSpec {
	t.Helper()
	stateDir := filepath.Join(dir, vmID)
	if err := os.MkdirAll(filepath.Join(stateDir, "api"), 0o700); err != nil {
		t.Fatal(err)
	}
	data := filepath.Join(stateDir, "data.img")
	f, err := os.Create(data)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(64 << 20); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	spec := MachineSpec{
		VMID:     vmID,
		StateDir: stateDir,
		Binary:   bin,
		Kernel:   kernel,
		Initrd:   os.Getenv("CHOIR_VMBACKEND_INITRD"),
		Cmdline:  strings.Join([]string{"console=ttyS0,115200", "reboot=k", "panic=1", "vm_id=" + vmID, "epoch=1"}, " "),
		VCPUs:    1,
		MemMiB:   256,
	}
	if store := os.Getenv("CHOIR_VMBACKEND_STORE_DISK"); store != "" {
		spec.Drives = append(spec.Drives, Drive{ID: "store", Path: store, ReadOnly: true})
	}
	spec.Drives = append(spec.Drives, Drive{ID: "data", Path: data})
	sock := filepath.Join(stateDir, "api", "vmm.sock")
	if len(sock) <= 107 {
		spec.APISocketPath = sock
	}
	if tapName != "" {
		spec.Net = &NetIface{ID: "eth0", TapName: tapName, MAC: "AA:FC:00:00:00:F0", TapFD: tapFD}
	}
	return spec
}
