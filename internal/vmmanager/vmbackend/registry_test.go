package vmbackend

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRegistryRejectsDuplicates(t *testing.T) {
	if _, err := NewRegistry(FirecrackerBackend{}, FirecrackerBackend{}); err == nil {
		t.Fatal("duplicate backend registered")
	}
	if _, err := NewRegistry(nil); err == nil {
		t.Fatal("nil backend registered")
	}
}

// One registry of live VMMs: a host running both backends sees both, and
// an unrelated process is no one's.
func TestRegistryLiveVMsAcrossBackends(t *testing.T) {
	reg, err := NewRegistry(FirecrackerBackend{}, CloudHypervisorBackend{})
	if err != nil {
		t.Fatal(err)
	}
	entries := []ProcCmdline{
		{PID: 42, Data: []byte("/nix/store/a/bin/firecracker\x00--no-api\x00--enable-pci\x00--id\x00vm-a\x00--config-file\x00/s/vm-a/fc-config.json\x00")},
		{PID: 43, Data: []byte("/nix/store/b/bin/cloud-hypervisor\x00--kernel\x00/k\x00--cmdline\x00root=fstab vm_id=vm-b epoch=1\x00")},
		{PID: 44, Data: []byte("/nix/store/b/bin/cloud-hypervisor\x00--cmdline\x00vm_id=vm-a\x00")}, // duplicate VMM for vm-a
		{PID: 45, Data: []byte("/bin/sleep\x0060\x00")},
		{PID: 0, Data: []byte("firecracker\x00--id\x00vm-zero\x00")},
	}
	got := reg.LiveVMs(entries)
	want := map[string][]LiveProcess{
		"vm-a": {{Backend: Firecracker, PID: 42}, {Backend: CloudHypervisor, PID: 44}},
		"vm-b": {{Backend: CloudHypervisor, PID: 43}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LiveVMs = %+v\nwant %+v", got, want)
	}
	if pids := reg.PIDsForVM(entries, "vm-a"); !reflect.DeepEqual(pids, []int{42, 44}) {
		t.Fatalf("PIDsForVM = %v", pids)
	}
	if pids := reg.PIDsForVM(entries, ""); pids != nil {
		t.Fatalf("empty id matched %v", pids)
	}
	if !reflect.DeepEqual(reg.Names(), []Name{Firecracker, CloudHypervisor}) {
		t.Fatal(reg.Names())
	}
}

// F3: a process two backends both claim is nobody's.
func TestRegistryRefusesAmbiguousIdentity(t *testing.T) {
	a := fakeBackend{name: "fake-a", caps: Caps{ColdBoot: true}}
	b := fakeBackend{name: "fake-b", caps: Caps{ColdBoot: true}}
	reg, err := NewRegistry(a, b)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, ok := reg.Identify([]string{"/bin/sh", "-c", "x", "fake-vmm", "--vm-id", "vm-x"}); ok {
		t.Fatal("ambiguous process identified")
	}
}

func TestScanProcAt(t *testing.T) {
	root := t.TempDir()
	write := func(pid, data string) {
		if err := os.MkdirAll(filepath.Join(root, pid), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, pid, "cmdline"), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("10", "firecracker\x00--id\x00vm-a\x00")
	write("11", "") // kernel thread
	write("self", "x")
	_ = os.WriteFile(filepath.Join(root, "uptime"), []byte("1"), 0o644)
	got := scanProcAt(root)
	if len(got) != 1 || got[0].PID != 10 {
		t.Fatalf("scan %+v", got)
	}
	if scanProcAt(filepath.Join(root, "missing")) != nil {
		t.Fatal("missing root must scan empty")
	}
}

func TestSplitCmdlineKeepsSpacedArgs(t *testing.T) {
	got := SplitCmdline([]byte("ch\x00--cmdline\x00a b c\x00"))
	if !reflect.DeepEqual(got, []string{"ch", "--cmdline", "a b c"}) {
		t.Fatalf("%q", got)
	}
	if got := SplitCmdline([]byte("firecracker --id vm")); !reflect.DeepEqual(got, []string{"firecracker", "--id", "vm"}) {
		t.Fatalf("%q", got)
	}
}
