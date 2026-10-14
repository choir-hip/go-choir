package vmbackend

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func chSpec(t *testing.T, tap *os.File) MachineSpec {
	return MachineSpec{
		VMID:          "vm-ch-0001",
		StateDir:      "/state/vm-ch-0001",
		Binary:        "/nix/store/x-cloud-hypervisor-52.0/bin/cloud-hypervisor",
		Kernel:        "/g/vmlinux",
		Initrd:        "/g/initrd",
		Cmdline:       "root=fstab console=ttyS0,115200 vm_id=vm-ch-0001 epoch=3",
		VCPUs:         4,
		MemMiB:        8192,
		APISocketPath: "/state/vm-ch-0001/api/ch.sock",
		Drives: []Drive{
			{ID: "store", Path: "/g/store.erofs", ReadOnly: true},
			{ID: "data", Path: "/state/vm-ch-0001/data.img"},
			{ID: "credential", Path: "/state/vm-ch-0001/credential.img"},
		},
		Net:     &NetIface{ID: "eth0", TapName: "vm-abc", MAC: "AA:FC:00:00:00:29", TapFD: tap},
		Balloon: &Balloon{DeflateOnOOM: true, FreePageReporting: true, StatsPollingIntervalS: 5},
	}
}

func devNull(t *testing.T) *os.File {
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

func TestCloudHypervisorArgs(t *testing.T) {
	tap := devNull(t)
	plan, err := CloudHypervisorBackend{}.PlanLaunch(chSpec(t, tap))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"--api-socket", "path=/state/vm-ch-0001/api/ch.sock",
		"--kernel", "/g/vmlinux",
		"--initramfs", "/g/initrd",
		"--cmdline", "root=fstab console=ttyS0,115200 vm_id=vm-ch-0001 epoch=3",
		"--cpus", "boot=4",
		"--memory", "size=8192M",
		"--disk",
		"path=/g/store.erofs,readonly=on,image_type=raw,id=store",
		"path=/state/vm-ch-0001/data.img,image_type=raw,id=data",
		"path=/state/vm-ch-0001/credential.img,image_type=raw,id=credential",
		"--net", "fd=3,mac=AA:FC:00:00:00:29,id=eth0",
		"--balloon", "size=0M,deflate_on_oom=on,free_page_reporting=on",
		"--serial", "tty", "--console", "off",
		"--seccomp", "true",
		"--event-monitor", "path=/state/vm-ch-0001/ch-events.json",
	}
	if strings.Join(plan.Args, "\n") != strings.Join(want, "\n") {
		t.Fatalf("args:\n%s\nwant:\n%s", strings.Join(plan.Args, "\n"), strings.Join(want, "\n"))
	}
	if plan.Binary != "/nix/store/x-cloud-hypervisor-52.0/bin/cloud-hypervisor" {
		t.Fatalf("binary %q", plan.Binary)
	}
	if len(plan.ExtraFiles) != 1 || plan.ExtraFiles[0] != tap {
		t.Fatalf("the tap must be the child's fd 3, got %v", plan.ExtraFiles)
	}
	if len(plan.WriteFiles) != 0 {
		t.Fatalf("cloud-hypervisor takes no config file, got %+v", plan.WriteFiles)
	}
	for _, p := range plan.HostPaths {
		if p.Path == "/dev/net/tun" {
			t.Fatal("a tap passed by fd must not need /dev/net/tun")
		}
	}
}

// Security floor of the slice: seccomp on, every disk raw, no tap by name.
func TestCloudHypervisorSecurityFloor(t *testing.T) {
	for _, landlock := range []bool{false, true} {
		args, err := CloudHypervisorBackend{Landlock: landlock}.CloudHypervisorArgs(chSpec(t, devNull(t)))
		if err != nil {
			t.Fatal(err)
		}
		joined := strings.Join(args, " ")
		if !strings.Contains(joined, "--seccomp true") {
			t.Fatal("seccomp must be on")
		}
		if strings.Contains(joined, "tap=") {
			t.Fatal("tap must arrive by fd, never by name")
		}
		inDisks := false
		for _, a := range args {
			if a == "--disk" {
				inDisks = true
				continue
			}
			if strings.HasPrefix(a, "--") {
				inDisks = false
			}
			if inDisks && !strings.Contains(a, "image_type=raw") {
				t.Fatalf("disk %q without image_type=raw", a)
			}
		}
		if landlock != strings.Contains(joined, "--landlock ") {
			t.Fatalf("landlock=%v but args %q", landlock, joined)
		}
	}
}

func TestCloudHypervisorLandlockRules(t *testing.T) {
	args, err := CloudHypervisorBackend{Landlock: true}.CloudHypervisorArgs(chSpec(t, devNull(t)))
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	for _, rule := range []string{
		"path=/g/store.erofs,access=r",
		"path=/state/vm-ch-0001/data.img,access=rw",
		"path=/g/vmlinux,access=r",
		"path=/dev/kvm,access=rw",
		"path=/state/vm-ch-0001/api,access=rw",
	} {
		if !strings.Contains(joined, rule) {
			t.Errorf("missing landlock rule %s in %s", rule, joined)
		}
	}
}

func TestCloudHypervisorArgsRefusals(t *testing.T) {
	ch := CloudHypervisorBackend{}
	cases := map[string]func(*MachineSpec){
		"tap without fd":     func(s *MachineSpec) { s.Net.TapFD = nil },
		"no mac":             func(s *MachineSpec) { s.Net.MAC = "" },
		"comma in disk path": func(s *MachineSpec) { s.Drives[1].Path = "/state/a,b/data.img" },
		"equals in disk id":  func(s *MachineSpec) { s.Drives[1].ID = "da=ta" },
		"comma in socket":    func(s *MachineSpec) { s.APISocketPath = "/s,x/ch.sock" },
		"equals in state":    func(s *MachineSpec) { s.StateDir = "/state/a=b" },
	}
	for name, mutate := range cases {
		s := chSpec(t, devNull(t))
		s.Drives = append([]Drive(nil), s.Drives...)
		net := *s.Net
		s.Net = &net
		mutate(&s)
		if _, err := ch.PlanLaunch(s); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestCloudHypervisorOptionalParts(t *testing.T) {
	s := chSpec(t, nil)
	s.Net = nil
	s.Balloon = nil
	s.Initrd = ""
	s.APISocketPath = ""
	plan, err := CloudHypervisorBackend{}.PlanLaunch(s)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(plan.Args, " ")
	for _, absent := range []string{"--net", "--balloon", "--initramfs", "--api-socket"} {
		if strings.Contains(joined, absent) {
			t.Errorf("%s present: %s", absent, joined)
		}
	}
	if len(plan.ExtraFiles) != 0 {
		t.Fatal("no net, no fds")
	}
	if plan.Binary != "/nix/store/x-cloud-hypervisor-52.0/bin/cloud-hypervisor" {
		t.Fatal(plan.Binary)
	}
	s.Binary = ""
	plan, _ = CloudHypervisorBackend{}.PlanLaunch(s)
	if plan.Binary != "cloud-hypervisor" {
		t.Fatalf("default binary %q", plan.Binary)
	}
}

func TestCloudHypervisorVMIDFromCmdline(t *testing.T) {
	ch := CloudHypervisorBackend{}
	for raw, want := range map[string]string{
		"/nix/store/x/bin/cloud-hypervisor\x00--kernel\x00/k\x00--cmdline\x00root=fstab vm_id=vm-ch-0001 epoch=3": "vm-ch-0001",
		"cloud-hypervisor\x00--cmdline=console=ttyS0 vm_id=vm-b":                                                  "vm-b",
	} {
		if id, ok := ch.VMIDFromCmdline(SplitCmdline([]byte(raw))); !ok || id != want {
			t.Fatalf("%q -> %q,%v want %q", raw, id, ok, want)
		}
	}
	for _, raw := range []string{
		"cloud-hypervisor\x00--cmdline\x00root=fstab epoch=3", // no vm_id
		"/bin/sh\x00-c\x00--cmdline vm_id=vm-x",               // not the VMM
		"firecracker\x00--id\x00vm-ch-0001",                   // the other backend
		"cloud-hypervisor\x00--kernel\x00/k",                  // no cmdline at all
		"cloud-hypervisor\x00--cmdline\x00vm_id=",             // empty id
	} {
		if id, ok := ch.VMIDFromCmdline(SplitCmdline([]byte(raw))); ok {
			t.Fatalf("%q identified as %q", raw, id)
		}
	}
	// Prefix ids must not collide: vm_id=vm-ch-0001-extra is a different VM.
	id, _ := ch.VMIDFromCmdline([]string{"cloud-hypervisor", "--cmdline", "vm_id=vm-ch-0001-extra"})
	if id != "vm-ch-0001-extra" {
		t.Fatalf("got %q", id)
	}
}

func TestCloudHypervisorIsColdBootOnly(t *testing.T) {
	var b Backend = CloudHypervisorBackend{}
	if b.Caps().Snapshot {
		t.Fatal("this slice must not report Snapshot")
	}
	if _, ok := b.(Snapshotter); ok {
		t.Fatal("cloud-hypervisor must not implement Snapshotter in this slice")
	}
	if _, ok := b.(MemoryController); ok {
		t.Fatal("cloud-hypervisor must not implement MemoryController in this slice")
	}
}

func TestCloudHypervisorPingAndCounters(t *testing.T) {
	sock, calls := fakeAPIServer(t, func(w http.ResponseWriter, call apiCall) {
		switch call.Path {
		case "/api/v1/vmm.ping":
			_, _ = w.Write([]byte(`{"build_version":"v52.0","version":"52.0","pid":1}`))
		case "/api/v1/vm.counters":
			_, _ = w.Write([]byte(`{"_disk1":{"read_bytes":4096,"write_ops":2},"_net3":{"rx_bytes":10}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	h := Handle{Backend: CloudHypervisor, VMID: "vm-ch", APISocket: sock}
	if err := (CloudHypervisorBackend{}).Ping(context.Background(), h); err != nil {
		t.Fatal(err)
	}
	var counters map[string]map[string]uint64
	if err := newUnixAPI("ch", sock).do(context.Background(), CloudHypervisor, http.MethodGet, "/api/v1/vm.counters", nil, &counters); err != nil {
		t.Fatal(err)
	}
	flat := flattenCounters(counters)
	if flat["_disk1.read_bytes"] != 4096 || flat["_disk1.write_ops"] != 2 || flat["_net3.rx_bytes"] != 10 || len(flat) != 3 {
		t.Fatalf("flattened %+v", flat)
	}
	if got := calls(); len(got) != 2 || got[0].Method != "GET" {
		t.Fatalf("calls %+v", got)
	}
	if err := (CloudHypervisorBackend{}).Ping(context.Background(), Handle{}); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("ping without socket: %v", err)
	}
}

func TestProcParsers(t *testing.T) {
	rss, err := parseStatusRSS([]byte("Name:\tcloud-hypervisor\nVmPeak:\t 1 kB\nVmRSS:\t  204800 kB\nThreads:\t5\n"))
	if err != nil || rss != 204800*1024 {
		t.Fatalf("rss %d %v", rss, err)
	}
	if _, err := parseStatusRSS([]byte("Name:\tzombie\nState:\tZ\n")); err == nil {
		t.Fatal("zombie without VmRSS must be an error")
	}
	// comm with spaces and parens; utime=250 ticks, stime=50 ticks.
	stat := "4242 (cloud hyper)visor) S 1 4242 4242 0 -1 4194560 100 0 0 0 250 50 0 0 20 0 5 0 100 0 0"
	u, s, err := parseStatCPU([]byte(stat))
	if err != nil || u.Seconds() != 2.5 || s.Seconds() != 0.5 {
		t.Fatalf("cpu %v %v %v", u, s, err)
	}
	if _, _, err := parseStatCPU([]byte("garbage")); err == nil {
		t.Fatal("malformed stat accepted")
	}

	root := t.TempDir()
	dir := filepath.Join(root, "77")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(dir, "status"), []byte("VmRSS:\t 8 kB\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "stat"), []byte(stat), 0o644)
	m, err := procMetricsAt(root, 77)
	if err != nil || m.PID != 77 || m.RSSBytes != 8192 {
		t.Fatalf("metrics %+v %v", m, err)
	}
	if _, err := procMetricsAt(root, 78); err == nil {
		t.Fatal("missing pid must be an error")
	}
}

func TestCloudHypervisorEvents(t *testing.T) {
	// The v52 event monitor writes pretty-printed objects back to back.
	stream := `{
  "timestamp": {"secs": 0, "nanos": 1200},
  "source": "vm",
  "event": "booting",
  "properties": null
}{
  "timestamp": {"secs": 3, "nanos": 5},
  "source": "vm",
  "event": "booted",
  "properties": null
}{
  "timestamp": {"secs": 90, "nanos": 7},
  "source": "vm",
  "event": "rebooting",
  "properties": null
}{
  "timestamp": {"secs": 90, "nanos": 9},
  "sour`
	evs, err := DecodeCloudHypervisorEvents(strings.NewReader(stream))
	if err != nil {
		t.Fatal(err)
	}
	if len(evs) != 3 || evs[2].Event != "rebooting" || evs[1].Timestamp.Secs != 3 {
		t.Fatalf("events %+v", evs)
	}
	ends := []bool{}
	for _, ev := range evs {
		ends = append(ends, ev.EndsRealization())
	}
	if ends[0] || ends[1] || !ends[2] {
		t.Fatalf("EndsRealization %v, want only the reboot", ends)
	}
	if (CloudHypervisorEvent{Source: "vmm", Event: "shutdown"}).EndsRealization() {
		t.Fatal("vmm shutdown is a process exit, already observed")
	}
	if _, err := DecodeCloudHypervisorEvents(strings.NewReader("not json")); err == nil {
		t.Fatal("garbage accepted")
	}
}
