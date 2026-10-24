package vmmanager

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
)

// VMM jailer slice 1 tests. The failure list these cover is the "Failure
// modes" section of docs/problems/firecracker-runs-as-root-without-jailer-
// 2026-10-14.md; each test names the items it pins. They prove argument
// building, jail layout, uid allocation and snapshot path handling only;
// confinement itself is proven on Node B (scripts/vmm_confinement_verify.py).

type chownCall struct {
	Path     string
	UID, GID int
}

// chownRecorder stands in for chown(2) in tests that cannot change owners:
// it records each call and keeps the owner per inode, so a rename or a hard
// link carries the owner exactly as the kernel would.
type chownRecorder struct {
	mu    sync.Mutex
	calls []chownCall
	owner map[string]int
}

func inodeKey(path string) (string, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return "", false
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "", false
	}
	return fmt.Sprintf("%d:%d", st.Dev, st.Ino), true
}

func (r *chownRecorder) chown(path string, uid, gid int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, chownCall{Path: path, UID: uid, GID: gid})
	key, ok := inodeKey(path)
	if !ok {
		return fmt.Errorf("chown %s: no such file", path)
	}
	if r.owner == nil {
		r.owner = map[string]int{}
	}
	r.owner[key] = uid
	return nil
}

func (r *chownRecorder) ownerOf(path string) (int, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key, ok := inodeKey(path)
	if !ok {
		return 0, false
	}
	uid, ok := r.owner[key]
	return uid, ok
}

func (r *chownRecorder) chownedTo(path string, uid int) bool {
	got, ok := r.ownerOf(path)
	return ok && got == uid
}

// withVMSlice gives mgr a fake cgroup2 root holding the VM slice systemd
// would have created and a parent inside it: every jailed launch checks the
// slice (confinementReasonVMSliceMissing).
func withVMSlice(t *testing.T, mgr *Manager) {
	t.Helper()
	mgr.cgroupRoot = t.TempDir()
	mgr.cfg.Jailer.CgroupParent = "system.slice/system-choirvms.slice/vms"
	if err := os.MkdirAll(filepath.Join(mgr.cgroupRoot, "system.slice", "system-choirvms.slice"), 0o755); err != nil {
		t.Fatal(err)
	}
}

// jailedManager returns a manager with the jailer enabled and the host
// marked prepared; chown and file-owner lookups go through the recorder.
func jailedManager(t *testing.T, stateDir string) (*Manager, *chownRecorder) {
	t.Helper()
	cfg := DefaultManagerConfig()
	cfg.StateDir = stateDir
	cfg.FirecrackerBinPath = "/nix/store/abc-firecracker-1.15.1/bin/firecracker"
	cfg.Jailer = JailerConfig{
		Enabled:      true,
		BinPath:      "/nix/store/abc-firecracker-1.15.1/bin/jailer",
		UIDBase:      DefaultJailerUIDBase,
		UIDCount:     DefaultJailerUIDCount,
		CgroupParent: "system.slice/system-choirvms.slice/vms",
		PidsMax:      DefaultJailerPidsMax,
		CPUWeight:    DefaultJailerCPUWeight,
		NoFile:       DefaultJailerNoFile,
	}
	mgr := NewManager(cfg)
	withVMSlice(t, mgr)
	// The host floor fails a start whose rules do not install (SEC-03):
	// tap networking runs against the fakes, never the host.
	mgr.hostNet = newFakeHostNet(t, newFakeIPTables(t), newFakeIP(nil))
	mgr.jailerHostErr = nil
	rec := &chownRecorder{}
	mgr.chownFn = rec.chown
	mgr.fileOwnerFn = func(path string) (int, bool) {
		if uid, ok := rec.ownerOf(path); ok {
			return uid, true
		}
		return defaultFileOwner(path)
	}
	return mgr, rec
}

func hostFirecrackerConfigFixture() map[string]interface{} {
	return map[string]interface{}{
		"boot-source": map[string]interface{}{
			"kernel_image_path": "/var/lib/go-choir/guest/vmlinux",
			"initrd_path":       "/var/lib/go-choir/guest/initrd",
			"boot_args":         "console=ttyS0 vm_id=vm-a",
		},
		"drives": []map[string]interface{}{
			{"drive_id": "store", "path_on_host": "/var/lib/go-choir/guest/storedisk.erofs", "is_root_device": false, "is_read_only": true},
			{"drive_id": "data", "path_on_host": "/var/lib/go-choir/vm-state/vm-a/data.img", "is_root_device": false, "is_read_only": false},
			{"drive_id": "credential", "path_on_host": "/var/lib/go-choir/vm-state/vm-a/credential.img", "is_root_device": false, "is_read_only": false},
		},
		"machine-config":     map[string]interface{}{"vcpu_count": 4, "mem_size_mib": 8192},
		"network-interfaces": []map[string]interface{}{{"iface_id": "eth0", "guest_mac": "AA:FC:00:00:00:01", "host_dev_name": "vm-abc"}},
		"balloon":            map[string]interface{}{"amount_mib": 0, "deflate_on_oom": true, "free_page_reporting": true},
	}
}

// Failure items 2, 3, 16: every host path becomes a jail-relative path; the
// writable drives are marked for hard links, the rest for copies; nothing
// else in the config moves and the input is not mutated.
func TestRewriteFirecrackerConfigForJail(t *testing.T) {
	host := hostFirecrackerConfigFixture()
	before, _ := json.Marshal(host)
	jail, files, err := rewriteFirecrackerConfigForJail(host)
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	after, _ := json.Marshal(host)
	if string(before) != string(after) {
		t.Fatal("rewrite mutated the host config")
	}
	boot := jail["boot-source"].(map[string]interface{})
	if boot["kernel_image_path"] != "/vmlinux" || boot["initrd_path"] != "/initrd" || boot["boot_args"] != "console=ttyS0 vm_id=vm-a" {
		t.Fatalf("boot-source = %v", boot)
	}
	drives := jail["drives"].([]interface{})
	wantDrives := map[string]string{"store": "/store.img", "data": "/data.img", "credential": "/credential.img"}
	for _, raw := range drives {
		d := raw.(map[string]interface{})
		if d["path_on_host"] != wantDrives[d["drive_id"].(string)] {
			t.Fatalf("drive %v path = %v", d["drive_id"], d["path_on_host"])
		}
	}
	want := []jailFile{
		{HostPath: "/var/lib/go-choir/guest/vmlinux", Name: "vmlinux"},
		{HostPath: "/var/lib/go-choir/guest/initrd", Name: "initrd"},
		{HostPath: "/var/lib/go-choir/guest/storedisk.erofs", Name: "store.img"},
		{HostPath: "/var/lib/go-choir/vm-state/vm-a/data.img", Name: "data.img", Writable: true},
		{HostPath: "/var/lib/go-choir/vm-state/vm-a/credential.img", Name: "credential.img", Writable: true},
	}
	if !reflect.DeepEqual(files, want) {
		t.Fatalf("files = %+v\nwant %+v", files, want)
	}
	for _, key := range []string{"machine-config", "network-interfaces", "balloon"} {
		a, _ := json.Marshal(host[key])
		b, _ := json.Marshal(jail[key])
		if string(a) != string(b) {
			t.Fatalf("%s changed: %s -> %s", key, a, b)
		}
	}
	// The hash input is deterministic: the same host config always yields the
	// same jail config bytes (the snapshot launch-config hash depends on it).
	again, _, _ := rewriteFirecrackerConfigForJail(hostFirecrackerConfigFixture())
	h1, _ := launchConfigHash(jail)
	h2, _ := launchConfigHash(again)
	if h1 != h2 {
		t.Fatal("jail config hash is not deterministic")
	}

	bad := hostFirecrackerConfigFixture()
	bad["drives"] = []map[string]interface{}{{"drive_id": "../etc", "path_on_host": "/x", "is_read_only": true}}
	if _, _, err := rewriteFirecrackerConfigForJail(bad); err == nil {
		t.Fatal("an unsafe drive id must be refused")
	}
}

// Failure items 9, 10: the jailer command line, Firecracker's own arguments
// after "--" without a second --id.
func TestJailerLaunchArgs(t *testing.T) {
	mgr, _ := jailedManager(t, "/var/lib/go-choir/vm-state")
	got := mgr.jailerArgs("vm-a", 900123, jailFirecrackerLaunchArgs(true))
	want := "--id vm-a --exec-file /nix/store/abc-firecracker-1.15.1/bin/firecracker --uid 900123 --gid 900123 " +
		"--chroot-base-dir /var/lib/go-choir/vm-state/.jail --new-pid-ns --cgroup-version 2 " +
		"--parent-cgroup system.slice/system-choirvms.slice/vms --cgroup pids.max=128 --cgroup cpu.weight=100 " +
		"--resource-limit no-file=2048 -- --api-sock /run/fc.sock --enable-pci --config-file /fc-config.json"
	if strings.Join(got, " ") != want {
		t.Fatalf("jailer args =\n%s\nwant\n%s", strings.Join(got, " "), want)
	}
	if got := strings.Join(jailFirecrackerLaunchArgs(false), " "); got != "--no-api --enable-pci --config-file /fc-config.json" {
		t.Fatalf("kill-mode jail args = %s", got)
	}
	if got := strings.Join(jailFirecrackerRestoreArgs(), " "); got != "--api-sock /run/fc.sock --enable-pci" {
		t.Fatalf("restore jail args = %s", got)
	}
	for _, args := range [][]string{jailFirecrackerLaunchArgs(true), jailFirecrackerRestoreArgs()} {
		for _, a := range args {
			if a == "--id" {
				t.Fatalf("firecracker args under the jailer must not repeat --id: %v", args)
			}
		}
	}

	// Cgroup properties are optional (an unset parent omits --parent-cgroup).
	mgr.cfg.Jailer.CgroupParent = ""
	mgr.cfg.Jailer.PidsMax = 0
	mgr.cfg.Jailer.CPUWeight = 0
	got = mgr.jailerArgs("vm-a", 900123, []string{"--version"})
	if s := strings.Join(got, " "); strings.Contains(s, "--parent-cgroup") || strings.Contains(s, "--cgroup ") || !strings.Contains(s, "--cgroup-version 2") {
		t.Fatalf("args without cgroup properties = %s", s)
	}
}

// Failure item 9: the jailer process names firecracker in --exec-file and
// carries --id, but only a process whose argv[0] is firecracker is the VM.
func TestCmdlineMatcherIgnoresJailerProcess(t *testing.T) {
	mgr, _ := jailedManager(t, "/var/lib/go-choir/vm-state")
	jailer := append([]string{mgr.cfg.Jailer.BinPath}, mgr.jailerArgs("vm-a", 900001, jailFirecrackerLaunchArgs(true))...)
	if firecrackerCmdlineBytesMatchVM([]byte(strings.Join(jailer, "\x00")), "vm-a") {
		t.Fatal("the jailer process must not count as the VM's Firecracker")
	}
	if got := firecrackerVMIDFromCmdline([]byte(strings.Join(jailer, "\x00"))); got != "" {
		t.Fatalf("jailer process mapped to VM %q", got)
	}
	// What the jailer execs: /firecracker --id <vm> --start-time-us ... <args>.
	jailed := []string{"/firecracker", "--id", "vm-a", "--start-time-us", "1", "--start-time-cpu-us", "2", "--parent-cpu-time-us", "3", "--api-sock", "/run/fc.sock", "--enable-pci", "--config-file", "/fc-config.json"}
	if !firecrackerCmdlineBytesMatchVM([]byte(strings.Join(jailed, "\x00")), "vm-a") {
		t.Fatal("the jailed Firecracker must identify its VM")
	}
	if got := firecrackerVMIDFromCmdline([]byte(strings.Join(jailed, "\x00"))); got != "vm-a" {
		t.Fatalf("jailed Firecracker mapped to %q", got)
	}
	direct := []string{"/nix/store/x-firecracker/bin/firecracker", "--api-sock", "/s", "--enable-pci", "--id", "vm-a", "--config-file", "/c"}
	if !firecrackerCmdlineBytesMatchVM([]byte(strings.Join(direct, "\x00")), "vm-a") {
		t.Fatal("a direct Firecracker must still identify its VM")
	}
}

// Failure item 4: uids are per VM, stable across a manager restart, never
// shared, and refused when out of range or exhausted.
func TestAllocateVMMUID(t *testing.T) {
	stateDir := t.TempDir()
	mgr, _ := jailedManager(t, stateDir)
	for _, vm := range []string{"vm-a", "vm-b", "vm-c"} {
		if err := os.MkdirAll(filepath.Join(stateDir, vm), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	a, err := mgr.allocateVMMUID("vm-a")
	if err != nil {
		t.Fatalf("allocate a: %v", err)
	}
	if a < DefaultJailerUIDBase || a >= DefaultJailerUIDBase+DefaultJailerUIDCount {
		t.Fatalf("uid %d outside range", a)
	}
	again, err := mgr.allocateVMMUID("vm-a")
	if err != nil || again != a {
		t.Fatalf("second allocation = %d, %v; want %d", again, err, a)
	}
	// A restarted manager reads the same durable record.
	restarted, _ := jailedManager(t, stateDir)
	if got, ok := restarted.readVMMUID("vm-a"); !ok || got != a {
		t.Fatalf("after restart uid = %d %v; want %d", got, ok, a)
	}
	data, err := os.ReadFile(filepath.Join(stateDir, "vm-a", vmmUIDFileName))
	if err != nil || strings.TrimSpace(string(data)) != strconv.Itoa(a) {
		t.Fatalf("uid record = %q, %v", data, err)
	}

	// Collision: vm-b's preferred uid is already held by vm-c.
	preferred := DefaultJailerUIDBase + int(fnv32("vm-b")%uint32(DefaultJailerUIDCount))
	if err := os.WriteFile(filepath.Join(stateDir, "vm-c", vmmUIDFileName), []byte(strconv.Itoa(preferred)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := mgr.allocateVMMUID("vm-b")
	if err != nil {
		t.Fatalf("allocate b: %v", err)
	}
	if b == preferred || b == a {
		t.Fatalf("vm-b got a held uid %d (preferred %d, a %d)", b, preferred, a)
	}

	// A corrupt or out-of-range record is never trusted.
	if err := os.WriteFile(filepath.Join(stateDir, "vm-c", vmmUIDFileName), []byte("42\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := mgr.readVMMUID("vm-c"); ok {
		t.Fatal("an out-of-range uid record must be refused")
	}
	if _, err := mgr.allocateVMMUID("vm-c"); err == nil {
		t.Fatal("allocation over a corrupt record must fail closed, not overwrite it")
	}

	// Exhaustion.
	small, _ := jailedManager(t, t.TempDir())
	small.cfg.Jailer.UIDCount = 1
	for _, vm := range []string{"vm-x", "vm-y"} {
		_ = os.MkdirAll(filepath.Join(small.cfg.StateDir, vm), 0o750)
	}
	if _, err := small.allocateVMMUID("vm-x"); err != nil {
		t.Fatalf("first allocation: %v", err)
	}
	if _, err := small.allocateVMMUID("vm-y"); err == nil {
		t.Fatal("an exhausted range must refuse allocation")
	}
}

// Failure items 1, 2, 3, 7: the jail is rebuilt each launch; writable disks
// are hard links owned by the VMM uid; boot artifacts are copies the VMM
// cannot change; parents are verified.
func TestPrepareJailLayout(t *testing.T) {
	stateDir := shortStateDir(t)
	mgr, rec := jailedManager(t, stateDir)
	vmDir := filepath.Join(stateDir, "vm-a")
	guest := filepath.Join(stateDir, "guest")
	for _, d := range []string{vmDir, guest} {
		if err := os.MkdirAll(d, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	write := func(path, body string, mode os.FileMode) {
		if err := os.WriteFile(path, []byte(body), mode); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(guest, "vmlinux"), "kernel", 0o444)
	write(filepath.Join(guest, "storedisk.erofs"), "store", 0o444)
	write(filepath.Join(vmDir, "data.img"), "data", 0o600)
	files := []jailFile{
		{HostPath: filepath.Join(guest, "vmlinux"), Name: "vmlinux"},
		{HostPath: filepath.Join(guest, "storedisk.erofs"), Name: "store.img"},
		{HostPath: filepath.Join(vmDir, "data.img"), Name: "data.img", Writable: true},
	}
	// Leftovers from a previous launch that the jailer would trip over.
	root := mgr.jailRoot("vm-a")
	if err := os.MkdirAll(filepath.Join(root, "dev", "net"), 0o700); err != nil {
		t.Fatal(err)
	}
	write(filepath.Join(root, "firecracker.pid"), "123", 0o644)
	write(filepath.Join(root, "firecracker"), strings.Repeat("old-binary", 100), 0o755)

	notes, err := mgr.prepareJail("vm-a", 900007, files, []byte(`{"cfg":1}`))
	if err != nil {
		t.Fatalf("prepareJail: %v", err)
	}
	for _, leftover := range []string{"firecracker.pid", "firecracker", "dev"} {
		if _, err := os.Lstat(filepath.Join(root, leftover)); !os.IsNotExist(err) {
			t.Fatalf("leftover %s survived the rebuild: %v", leftover, err)
		}
	}
	dataHost, _ := os.Stat(filepath.Join(vmDir, "data.img"))
	dataJail, err := os.Stat(filepath.Join(root, "data.img"))
	if err != nil || !os.SameFile(dataHost, dataJail) {
		t.Fatalf("data.img must be a hard link of the host file: %v", err)
	}
	if !rec.chownedTo(filepath.Join(root, "data.img"), 900007) {
		t.Fatalf("data.img not chowned to the VMM uid: %+v", rec.calls)
	}
	if dataJail.Mode().Perm() != 0o600 {
		t.Fatalf("data.img mode = %v", dataJail.Mode().Perm())
	}
	for _, name := range []string{"vmlinux", "store.img"} {
		host, _ := os.Stat(filepath.Join(guest, map[string]string{"vmlinux": "vmlinux", "store.img": "storedisk.erofs"}[name]))
		jail, err := os.Stat(filepath.Join(root, name))
		if err != nil {
			t.Fatalf("%s missing: %v", name, err)
		}
		if os.SameFile(host, jail) {
			t.Fatalf("%s must not share the source inode", name)
		}
		if jail.Mode().Perm() != 0o444 {
			t.Fatalf("%s mode = %v, want 0444", name, jail.Mode().Perm())
		}
		if _, chowned := rec.ownerOf(filepath.Join(root, name)); chowned {
			t.Fatalf("%s must stay owned by vmctl", name)
		}
	}
	if _, chowned := rec.ownerOf(filepath.Join(guest, "vmlinux")); chowned {
		t.Fatal("a source artifact was chowned")
	}
	if cfg, err := os.ReadFile(filepath.Join(root, "fc-config.json")); err != nil || string(cfg) != `{"cfg":1}` {
		t.Fatalf("fc-config.json = %q, %v", cfg, err)
	}
	if len(notes) != len(files) {
		t.Fatalf("placement notes = %v", notes)
	}
	for _, dir := range []string{mgr.jailChrootBase(), filepath.Join(mgr.jailChrootBase(), "firecracker"), mgr.jailDir("vm-a")} {
		info, err := os.Lstat(dir)
		if err != nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
			t.Fatalf("%s = %v, %v; want a 0700 directory", dir, info, err)
		}
	}

	// A symlinked parent is refused (CVE-2026-1386 class).
	evil := shortStateDir(t)
	mgr2, _ := jailedManager(t, evil)
	if err := os.Symlink(t.TempDir(), mgr2.jailChrootBase()); err != nil {
		t.Fatal(err)
	}
	if _, err := mgr2.prepareJail("vm-a", 900007, nil, nil); err == nil {
		t.Fatal("a symlinked jail base must be refused")
	}
	// A writable source outside the jail's filesystem is never copied.
	if _, err := mgr.prepareJail("vm-a", 900007, []jailFile{{HostPath: filepath.Join(vmDir, "missing.img"), Name: "data.img", Writable: true}}, nil); err == nil {
		t.Fatal("a writable drive that cannot be hard-linked must fail the launch")
	}
}

// Failure items 5, 25, 26: the mode decision and its reasons.
func TestDecideConfinement(t *testing.T) {
	mgr, _ := jailedManager(t, t.TempDir())
	cfg := VMConfig{VMID: "vm-0123abcd", ComputerID: "computer-1"}
	if mode, reason := mgr.decideConfinement(cfg); mode != ConfinementJailer || reason != "" {
		t.Fatalf("on = %s %s", mode, reason)
	}
	mgr.cfg.Jailer.Only = []string{"computer-2", "vm-other"}
	if mode, reason := mgr.decideConfinement(cfg); mode != ConfinementDirect || reason != confinementReasonNotSelected {
		t.Fatalf("not selected = %s %s", mode, reason)
	}
	mgr.cfg.Jailer.Only = []string{"computer-1"}
	if mode, _ := mgr.decideConfinement(cfg); mode != ConfinementJailer {
		t.Fatal("a computer id in the allowlist must select the jailer")
	}
	mgr.cfg.Jailer.Only = []string{"vm-0123abcd"}
	if mode, _ := mgr.decideConfinement(cfg); mode != ConfinementJailer {
		t.Fatal("a VM id in the allowlist must select the jailer")
	}
	mgr.jailerHostErr = errors.New("iptables missing")
	if mode, reason := mgr.decideConfinement(cfg); mode != ConfinementDirect || !strings.HasPrefix(reason, confinementReasonHostUnprepared) {
		t.Fatalf("unprepared = %s %s", mode, reason)
	}
	mgr.jailerHostErr = nil
	for _, bad := range []string{"vm_a", "vm.a", strings.Repeat("a", 65), ""} {
		mgr.cfg.Jailer.Only = nil
		if mode, reason := mgr.decideConfinement(VMConfig{VMID: bad}); mode != ConfinementDirect || reason != confinementReasonInvalidID {
			t.Fatalf("id %q = %s %s", bad, mode, reason)
		}
	}
	mgr.cfg.Jailer.Enabled = false
	if mode, reason := mgr.decideConfinement(cfg); mode != ConfinementDirect || reason != confinementReasonOff {
		t.Fatalf("off = %s %s", mode, reason)
	}
}

func TestJailerConfigFromEnv(t *testing.T) {
	t.Setenv("VM_FIRECRACKER_BIN", "/nix/store/x-firecracker-1.15.1/bin/firecracker")
	t.Setenv("VM_STATE_DIR", "/var/lib/go-choir/vm-state")
	cfg := LoadConfigFromEnv()
	if cfg.Jailer.Enabled || cfg.Jailer.SpeculationDirect {
		t.Fatalf("default must be off: %+v", cfg.Jailer)
	}
	if cfg.Jailer.BinPath != "/nix/store/x-firecracker-1.15.1/bin/jailer" || cfg.Jailer.UIDBase != 900000 || cfg.Jailer.UIDCount != 4096 {
		t.Fatalf("defaults = %+v", cfg.Jailer)
	}
	t.Setenv("VMCTL_JAILER", "on")
	t.Setenv("VMCTL_JAILER_ONLY", " computer-1 , vm-a ,,")
	t.Setenv("VMCTL_JAILER_CGROUP_PARENT", "/system.slice/go-choir-vmctl.service/vms/")
	t.Setenv("VMCTL_JAILER_PIDS_MAX", "64")
	t.Setenv("VMCTL_VMM_SPECULATION_BARRIERS", "on")
	cfg = LoadConfigFromEnv()
	if !cfg.Jailer.Enabled || !reflect.DeepEqual(cfg.Jailer.Only, []string{"computer-1", "vm-a"}) || cfg.Jailer.PidsMax != 64 || !cfg.Jailer.SpeculationDirect {
		t.Fatalf("env config = %+v", cfg.Jailer)
	}
	if cfg.Jailer.CgroupParent != "system.slice/go-choir-vmctl.service/vms" {
		t.Fatalf("cgroup parent must be relative and clean: %q", cfg.Jailer.CgroupParent)
	}
	t.Setenv("VMCTL_JAILER", "off")
	if LoadConfigFromEnv().Jailer.Enabled {
		t.Fatal("off must disable")
	}
}

// Failure item 11: vmctl reads its own cgroup v2 path. The VMs' parent is
// never derived from it (docs/problems/vmctl-restart-fails-while-a-jailed-
// vm-runs-2026-10-24.md, "Design: the VM slice"); see vm_slice_test.go.
func TestParseOwnCgroupV2Path(t *testing.T) {
	own := parseCgroupV2Path("0::/system.slice/go-choir-vmctl.service/supervisor\n")
	if own != "system.slice/go-choir-vmctl.service/supervisor" {
		t.Fatalf("own = %q", own)
	}
	if got := parseCgroupV2Path("12:pids:/x\n1:name=systemd:/y\n"); got != "" {
		t.Fatalf("v1-only cgroup file parsed as %q", got)
	}
}

// Failure items 14, 15, 16: a jailed hibernate writes inside the jail, the
// files move to the host snapshot dir owned by the VMM uid, metadata records
// the confinement, and the restore loads jail-relative paths from a rebuilt
// jail under the same uid.
func TestJailedSnapshotRoundTrip(t *testing.T) {
	f := newJailedSnapshotFixture(t)
	if err := f.mgr.HibernateVM(f.vmID); err != nil {
		t.Fatalf("HibernateVM: %v", err)
	}
	f.fc.mu.Lock()
	var createBody map[string]any
	for _, c := range f.fc.calls {
		if c.Path == "/snapshot/create" {
			createBody = c.Body
		}
	}
	f.fc.mu.Unlock()
	if createBody["snapshot_path"] != "/snapshot.partial/vmstate" || createBody["mem_file_path"] != "/snapshot.partial/mem" {
		t.Fatalf("create paths must be jail-relative: %v", createBody)
	}
	dir := f.mgr.snapshotDir(f.vmID)
	meta, err := readSnapshotMeta(dir)
	if err != nil {
		t.Fatalf("committed snapshot: %v (timeline %+v)", err, readHibernateTimeline(t, f.mgr, f.vmID).Snapshot)
	}
	if meta.Confinement != ConfinementJailer || meta.VMMUID != f.uid {
		t.Fatalf("meta confinement = %q uid %d", meta.Confinement, meta.VMMUID)
	}
	for _, name := range []string{snapshotStateName, snapshotMemName} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("%s = %v, %v", name, info, err)
		}
	}
	if _, err := os.Stat(filepath.Join(f.mgr.jailRoot(f.vmID), snapshotPartialDirName)); !os.IsNotExist(err) {
		t.Fatalf("the jail's capture dir must be empty after commit: %v", err)
	}

	restoreAPI := &fakeFirecrackerAPI{root: f.mgr.jailRoot(f.vmID)}
	var restoreArgs []string
	f.jailedRestoreStarter(t, restoreAPI, &restoreArgs)
	inst, err := f.mgr.ResumeVMFromSnapshot(f.vmID, f.expect, nil)
	if err != nil {
		t.Fatalf("ResumeVMFromSnapshot: %v", err)
	}
	if inst.State != StateRunning {
		t.Fatalf("state = %s", inst.State)
	}
	joined := strings.Join(restoreArgs, " ")
	for _, want := range []string{"--exec-file " + f.mgr.cfg.FirecrackerBinPath, fmt.Sprintf("--uid %d --gid %d", f.uid, f.uid), "--new-pid-ns", "-- --api-sock /run/fc.sock --enable-pci"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("restore args %q lack %q", joined, want)
		}
	}
	if restoreAPI.loadState != "/snapshot/vmstate" || restoreAPI.loadMem != "/snapshot/mem" {
		t.Fatalf("load paths = %s %s", restoreAPI.loadState, restoreAPI.loadMem)
	}
	if !restoreAPI.loadFilesPresent {
		t.Fatal("the snapshot files were not inside the jail at load time")
	}
	for _, name := range []string{"vmlinux", "initrd", "store.img", "data.img", "credential.img"} {
		if _, err := os.Stat(filepath.Join(f.mgr.jailRoot(f.vmID), name)); err != nil {
			t.Fatalf("restore jail lacks %s: %v", name, err)
		}
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("host snapshot files must be removed after resume: %v", err)
	}
	if _, err := os.Stat(filepath.Join(f.mgr.jailRoot(f.vmID), snapshotDirName)); !os.IsNotExist(err) {
		t.Fatalf("jail snapshot links must be removed after resume (they pin the memory file): %v", err)
	}
	tl, _ := f.mgr.BootTimelineForVM(f.vmID)
	if tl == nil || tl.Confinement == nil || tl.Confinement.Mode != ConfinementJailer || tl.Confinement.UID != f.uid {
		t.Fatalf("resume timeline confinement = %+v", tl)
	}
}

// Failure item 15: no mixed-mode resume. A snapshot taken outside the jail
// is refused once the computer is selected for the jailer, and the reverse.
func TestSnapshotRefusedAcrossConfinementModes(t *testing.T) {
	t.Run("direct snapshot, jailer now", func(t *testing.T) {
		f := newSnapshotFixture(t)
		f.serveLaunchAPI(t)
		if err := f.mgr.HibernateVM(f.vmID); err != nil {
			t.Fatal(err)
		}
		meta, err := readSnapshotMeta(f.mgr.snapshotDir(f.vmID))
		if err != nil || meta.Confinement != ConfinementDirect {
			t.Fatalf("direct meta = %+v, %v", meta, err)
		}
		enableJailer(t, f.mgr)
		started := false
		f.mgr.firecrackerStarter = func(string, []string) error { started = true; return nil }
		_, err = f.mgr.ResumeVMFromSnapshot(f.vmID, f.expect, nil)
		var fb *SnapshotFallbackError
		if !errors.As(err, &fb) || fb.Reason != snapshotReasonConfinement {
			t.Fatalf("resume = %v, want %s", err, snapshotReasonConfinement)
		}
		if started {
			t.Fatal("a refused snapshot must never start Firecracker")
		}
	})
	t.Run("jailed snapshot, jailer now off", func(t *testing.T) {
		f := newJailedSnapshotFixture(t)
		if err := f.mgr.HibernateVM(f.vmID); err != nil {
			t.Fatal(err)
		}
		f.mgr.cfg.Jailer.Enabled = false
		_, err := f.mgr.ResumeVMFromSnapshot(f.vmID, f.expect, nil)
		var fb *SnapshotFallbackError
		if !errors.As(err, &fb) || fb.Reason != snapshotReasonConfinement {
			t.Fatalf("resume = %v, want %s", err, snapshotReasonConfinement)
		}
	})
	t.Run("jailed snapshot, uid changed", func(t *testing.T) {
		f := newJailedSnapshotFixture(t)
		if err := f.mgr.HibernateVM(f.vmID); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(f.mgr.vmStateDir(f.vmID), vmmUIDFileName), []byte(strconv.Itoa(f.uid+1)+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := f.mgr.ResumeVMFromSnapshot(f.vmID, f.expect, nil)
		var fb *SnapshotFallbackError
		if !errors.As(err, &fb) || fb.Reason != snapshotReasonConfinement {
			t.Fatalf("resume = %v, want %s", err, snapshotReasonConfinement)
		}
	})
}

// Failure item 23: a direct launch after a jailed one gives the disks back to
// root and removes the stale jail.
func TestReclaimDirectOwnership(t *testing.T) {
	stateDir := shortStateDir(t)
	mgr, rec := jailedManager(t, stateDir)
	vmDir := filepath.Join(stateDir, "vm-a")
	if err := os.MkdirAll(vmDir, 0o750); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"data.img", "credential.img"} {
		if err := os.WriteFile(filepath.Join(vmDir, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
		_ = rec.chown(filepath.Join(vmDir, name), 900005, 900005)
	}
	if err := os.MkdirAll(mgr.jailRoot("vm-a"), 0o700); err != nil {
		t.Fatal(err)
	}
	rec.calls = nil
	mgr.reclaimDirectOwnership("vm-a")
	for _, name := range []string{"data.img", "credential.img"} {
		if !rec.chownedTo(filepath.Join(vmDir, name), 0) {
			t.Fatalf("%s not returned to root: %+v", name, rec.calls)
		}
	}
	if _, err := os.Stat(mgr.jailDir("vm-a")); !os.IsNotExist(err) {
		t.Fatalf("stale jail must be removed: %v", err)
	}
	// Files never owned by a VMM uid are left alone.
	rec.calls = nil
	mgr.reclaimDirectOwnership("vm-a")
	if len(rec.calls) != 0 {
		t.Fatalf("second reclaim chowned again: %+v", rec.calls)
	}
}

// Failure item 6: a jailed VMM gets a tap its uid owns; an ownerless tap left
// from a direct launch is recreated, and the tap firewall is installed as for
// any tap.
func TestJailedTapIsOwnedByTheVMMUID(t *testing.T) {
	ipt := newFakeIPTables(t)
	ip := newFakeIP(nil)
	m := NewManager(ManagerConfig{StateDir: t.TempDir(), HostBasePort: 9000})
	m.hostNet = newFakeHostNet(t, ipt, ip)
	vmID := "vm-tap-owner"
	tap := tapNameForVMID(vmID)
	m.prepareVMNetwork(vmID, 9302, 8085, -1)
	ip.mu.Lock()
	ip.calls = nil
	ip.mu.Unlock()
	m.prepareVMNetwork(vmID, 9302, 8085, 900042)
	ip.mu.Lock()
	calls := make([]string, 0, len(ip.calls))
	for _, c := range ip.calls {
		calls = append(calls, strings.Join(c, " "))
	}
	ip.mu.Unlock()
	deleted, created := -1, -1
	for i, c := range calls {
		if c == "link del "+tap {
			deleted = i
		}
		if c == "tuntap add dev "+tap+" mode tap user 900042 group 900042" {
			created = i
		}
	}
	if deleted < 0 || created < 0 || deleted > created {
		t.Fatalf("an ownerless tap must be deleted, then created owned by the VMM uid; calls=%v", calls)
	}
	if !ip.has(tap) || !strings.Contains(ipt.dump(), tap) {
		t.Fatalf("jailed tap %s lacks its link or firewall rules:\n%s", tap, ipt.dump())
	}
}

// Failure item 24: VMM uids hold no IP. The OUTPUT DROP is installed first
// in the chain, once, and the tap sweep never takes it for a tap's rule.
func TestVMMNoIPRule(t *testing.T) {
	ipt := newFakeIPTables(t)
	m := NewManager(ManagerConfig{StateDir: t.TempDir(), HostBasePort: 9000, Jailer: defaultJailerConfig()})
	m.hostNet = newFakeHostNet(t, ipt, newFakeIP(nil))
	for i := 0; i < 2; i++ {
		if err := m.ensureVMMNoIPRules(); err != nil {
			t.Fatalf("ensureVMMNoIPRules: %v", err)
		}
	}
	dump := ipt.dump()
	if n := strings.Count(dump, vmmNoIPComment); n != 1 {
		t.Fatalf("no-IP rule present %d times:\n%s", n, dump)
	}
	if !strings.Contains(dump, "--uid-owner 900000-904095") {
		t.Fatalf("no-IP rule does not cover the VMM range:\n%s", dump)
	}
	if got := ruleOwnerTap(m.vmmNoIPRuleSpec()); got != "" {
		t.Fatalf("the tap sweep would own the no-IP rule as tap %q", got)
	}
}

// --- jailed snapshot fixture ----------------------------------------------

type jailedSnapshotFixture struct {
	*snapshotFixture
	uid int
}

func enableJailer(t *testing.T, mgr *Manager) *chownRecorder {
	t.Helper()
	mgr.cfg.Jailer = JailerConfig{
		Enabled:  true,
		BinPath:  "/nonexistent/jailer",
		UIDBase:  DefaultJailerUIDBase,
		UIDCount: DefaultJailerUIDCount,
	}
	withVMSlice(t, mgr)
	mgr.jailerHostErr = nil
	rec := &chownRecorder{}
	mgr.chownFn = rec.chown
	mgr.fileOwnerFn = func(path string) (int, bool) {
		if uid, ok := rec.ownerOf(path); ok {
			return uid, true
		}
		return defaultFileOwner(path)
	}
	return rec
}

// newJailedSnapshotFixture is newSnapshotFixture with the VM launched under
// the jailer: its launch config has jail paths and its API socket and
// capture files live in the jail.
func newJailedSnapshotFixture(t *testing.T) *jailedSnapshotFixture {
	t.Helper()
	f := newSnapshotFixture(t)
	f.vmID = "vm-s3"
	enableJailer(t, f.mgr)
	uid, err := f.mgr.allocateVMMUID(f.vmID)
	if err != nil {
		t.Fatalf("allocate uid: %v", err)
	}
	hostPort, _ := f.mgr.hostPortFromHostURL(f.guestS.URL)
	jailCfg, files, err := rewriteFirecrackerConfigForJail(f.mgr.buildFirecrackerConfig(f.cfg, hostPort))
	if err != nil {
		t.Fatal(err)
	}
	launch, _ := json.Marshal(jailCfg)
	if err := os.WriteFile(filepath.Join(f.mgr.vmStateDir(f.vmID), "fc-config.json"), launch, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := f.mgr.prepareJail(f.vmID, uid, files, launch); err != nil {
		t.Fatalf("prepareJail: %v", err)
	}
	f.inst.confinement = &ConfinementReceipt{Mode: ConfinementJailer, UID: uid, GID: uid, JailRoot: f.mgr.jailRoot(f.vmID)}
	f.fc.root = f.mgr.jailRoot(f.vmID)
	serveUnix(t, f.mgr.jailAPISocketHostPath(f.vmID), f.fc)
	return &jailedSnapshotFixture{snapshotFixture: f, uid: uid}
}

// jailedRestoreStarter fakes the jailer: it serves the restore API on the
// jail socket named after "--".
func (f *jailedSnapshotFixture) jailedRestoreStarter(t *testing.T, api *fakeFirecrackerAPI, gotArgs *[]string) {
	f.mgr.firecrackerStarter = func(vmID string, args []string) error {
		*gotArgs = append([]string(nil), args...)
		sep := -1
		for i, a := range args {
			if a == "--" {
				sep = i
				break
			}
		}
		if sep < 0 {
			return errors.New("jailer args without --")
		}
		sock := ""
		fcArgs := args[sep+1:]
		for i, a := range fcArgs {
			if a == "--api-sock" && i+1 < len(fcArgs) {
				sock = fcArgs[i+1]
			}
		}
		if sock == "" {
			return errors.New("no --api-sock")
		}
		serveUnix(t, filepath.Join(f.mgr.jailRoot(vmID), sock), api)
		return nil
	}
}

// RES-05 (docs/problems/jailer-copies-store-image-per-vm-2026-10-21.md):
// jailed VMs read each read-only artifact through one shared inode, so the
// host page cache holds one copy of the store however many VMs run.
func TestPrepareJailSharesReadOnlyArtifacts(t *testing.T) {
	stateDir := shortStateDir(t)
	mgr, rec := jailedManager(t, stateDir)
	guest := filepath.Join(stateDir, "guest")
	if err := os.MkdirAll(guest, 0o750); err != nil {
		t.Fatal(err)
	}
	store := filepath.Join(guest, "storedisk.erofs")
	if err := os.WriteFile(store, []byte("store-v1"), 0o444); err != nil {
		t.Fatal(err)
	}
	place := func(vmID string) (string, []string) {
		t.Helper()
		vmDir := filepath.Join(stateDir, vmID)
		if err := os.MkdirAll(vmDir, 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(vmDir, "data.img"), []byte("data"), 0o600); err != nil {
			t.Fatal(err)
		}
		notes, err := mgr.prepareJail(vmID, 900007, []jailFile{
			{HostPath: store, Name: "store.img"},
			{HostPath: filepath.Join(vmDir, "data.img"), Name: "data.img", Writable: true},
		}, nil)
		if err != nil {
			t.Fatalf("prepareJail %s: %v", vmID, err)
		}
		return filepath.Join(mgr.jailRoot(vmID), "store.img"), notes
	}
	aPath, aNotes := place("vm-a")
	bPath, _ := place("vm-b")
	a, _ := os.Stat(aPath)
	b, _ := os.Stat(bPath)
	src, _ := os.Stat(store)
	if !os.SameFile(a, b) {
		t.Fatal("two jailed VMs must read the store image through one inode")
	}
	if os.SameFile(a, src) {
		t.Fatal("the jail must never link the source artifact itself")
	}
	if a.Mode().Perm() != 0o444 {
		t.Fatalf("shared store mode = %v, want 0444", a.Mode().Perm())
	}
	if _, chowned := rec.ownerOf(aPath); chowned {
		t.Fatal("the shared store image must stay owned by vmctl")
	}
	if st, ok := a.Sys().(*syscall.Stat_t); !ok || int(st.Uid) != os.Geteuid() {
		t.Fatalf("shared store owner = %+v, want vmctl", a.Sys())
	}
	if !strings.Contains(strings.Join(aNotes, " "), "store.img=shared") {
		t.Fatalf("receipt notes %v must say the store is shared", aNotes)
	}
	if got, _ := os.ReadFile(aPath); string(got) != "store-v1" {
		t.Fatalf("shared store bytes = %q", got)
	}
	// A relaunch of vm-a keeps the same shared inode.
	again, _ := place("vm-a")
	ag, _ := os.Stat(again)
	if !os.SameFile(ag, b) {
		t.Fatal("a relaunch must reuse the shared inode")
	}

	// A deploy replaces the source: new launches get a new inode, running
	// jails keep theirs, and the old copy goes once no jail holds it.
	if err := os.Remove(store); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store, []byte("store-v2-longer"), 0o444); err != nil {
		t.Fatal(err)
	}
	cPath, _ := place("vm-c")
	c, _ := os.Stat(cPath)
	if os.SameFile(c, b) {
		t.Fatal("a replaced source must get a new shared inode")
	}
	if got, _ := os.ReadFile(bPath); string(got) != "store-v1" {
		t.Fatalf("a running jail's store changed under it: %q", got)
	}
	countShared := func() int {
		entries, err := os.ReadDir(mgr.jailSharedDir())
		if err != nil {
			t.Fatal(err)
		}
		return len(entries)
	}
	if n := countShared(); n != 2 {
		t.Fatalf("shared entries = %d, want the old copy (held by vm-a, vm-b) and the new", n)
	}
	place("vm-a")
	place("vm-b")
	if n := countShared(); n != 1 {
		t.Fatalf("shared entries = %d, want 1 once no jail holds the old copy", n)
	}

	// A shared file that is not vmctl's read-only copy is rebuilt, never used.
	entries, _ := os.ReadDir(mgr.jailSharedDir())
	shared := filepath.Join(mgr.jailSharedDir(), entries[0].Name())
	if err := os.Chmod(shared, 0o666); err != nil {
		t.Fatal(err)
	}
	dPath, _ := place("vm-d")
	d, _ := os.Stat(dPath)
	if d.Mode().Perm() != 0o444 {
		t.Fatalf("a writable shared file was linked into a jail (mode %v)", d.Mode().Perm())
	}
	if base, err := os.Lstat(mgr.jailSharedDir()); err != nil || !base.IsDir() || base.Mode().Perm() != 0o700 {
		t.Fatalf("shared dir = %v, %v; want a 0700 directory", base, err)
	}
}
