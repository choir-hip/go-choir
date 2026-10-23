//go:build linux

package vmmanager

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// onDiscardedThread runs fn on a locked OS thread that Go destroys after:
// per-thread capability and speculation state must never reach the rest of
// the test binary.
func onDiscardedThread(fn func()) {
	done := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		defer close(done)
		fn()
	}()
	<-done
}

// kernelReportsForce reports whether this kernel shows a forced speculation
// control in /proc for a thread that set it (x86 does; an arm64 VM was seen
// to accept the prctl and keep reporting "vulnerable").
func kernelReportsForce(which uintptr, key string) bool {
	shows := false
	onDiscardedThread(func() {
		if unix.Prctl(unix.PR_SET_SPECULATION_CTRL, which, unix.PR_SPEC_FORCE_DISABLE, 0, 0) != nil {
			return
		}
		data, _ := os.ReadFile("/proc/thread-self/status")
		shows = strings.Contains(statusField(string(data), key), "force")
	})
	return shows
}

// Linux-only tests of the launch thread (failure items 19, 20) and, gated by
// CHOIR_JAILER_IT=1, the real jailer (items 7, 8, 9, 10, 17, 18).

func statusField(status, key string) string {
	for _, line := range strings.Split(status, "\n") {
		if k, v, ok := strings.Cut(line, ":"); ok && k == key {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func childStatus(t *testing.T, start func(*exec.Cmd) error) string {
	t.Helper()
	var out bytes.Buffer
	cmd := exec.Command(catBin(t), "/proc/self/status")
	cmd.Stdout = &out
	if err := start(cmd); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("wait: %v", err)
	}
	return out.String()
}

// Item 20: the barriers and the bounding cut reach the child and never any
// other process vmctl starts afterwards.
func TestLaunchThreadPolicyReachesOnlyItsChild(t *testing.T) {
	plain := func(cmd *exec.Cmd) error { return cmd.Start() }
	baseline := childStatus(t, plain)
	keep := []int{capNetAdmin}
	var report LaunchThreadReport
	policied := childStatus(t, func(cmd *exec.Cmd) error {
		var err error
		report, err = startWithThreadPolicy(cmd, threadPolicy{keepCaps: keep, speculation: true})
		return err
	})
	if !report.Supported {
		t.Fatal("thread policy must be supported on linux")
	}
	t.Logf("report=%+v", report)
	forced := map[string]bool{}
	for _, r := range report.Speculation {
		name, result, _ := strings.Cut(r, "=")
		forced[name] = result == "force_disabled"
	}
	for _, c := range []struct {
		name, key string
		which     uintptr
	}{
		{"indirect_branch", "SpeculationIndirectBranch", unix.PR_SPEC_INDIRECT_BRANCH},
		{"store_bypass", "Speculation_Store_Bypass", unix.PR_SPEC_STORE_BYPASS},
	} {
		if !forced[c.name] {
			continue
		}
		if !kernelReportsForce(c.which, c.key) {
			t.Logf("%s: kernel accepts the control but does not report it in /proc; child shows %q", c.name, statusField(policied, c.key))
			continue
		}
		if !strings.Contains(statusField(policied, c.key), "force") {
			t.Fatalf("child %s = %q, want forced", c.key, statusField(policied, c.key))
		}
	}
	if statusField(policied, "NoNewPrivs") != "1" || !report.NoNewPrivs {
		t.Fatalf("child NoNewPrivs = %q (report %v)", statusField(policied, "NoNewPrivs"), report.NoNewPrivs)
	}
	if os.Geteuid() == 0 {
		bnd, _ := strconv.ParseUint(statusField(policied, "CapBnd"), 16, 64)
		if bnd != 1<<capNetAdmin {
			t.Fatalf("child CapBnd = %#x, want only net_admin", bnd)
		}
	} else if report.BoundingSkipped == "" {
		t.Fatal("a non-root vmctl must skip the bounding cut and say so")
	}
	// No leak: many later plain children look like the baseline.
	for i := 0; i < 32; i++ {
		later := childStatus(t, plain)
		for _, key := range []string{"SpeculationIndirectBranch", "Speculation_Store_Bypass", "CapBnd", "NoNewPrivs"} {
			if statusField(later, key) != statusField(baseline, key) {
				t.Fatalf("launch thread state leaked: %s = %q, baseline %q", key, statusField(later, key), statusField(baseline, key))
			}
		}
	}
}

// Item 19: a capability that cannot be dropped fails the launch unless it is
// tolerated. Run on a discarded thread: a successful drop must not reach the
// rest of the test binary.
func TestLaunchThreadBoundingDrop(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("needs root")
	}
	var err error
	var report LaunchThreadReport
	onDiscardedThread(func() {
		err = dropBoundingSet(&report, []int{capNetAdmin}, nil)
	})
	if err != nil && !strings.Contains(err.Error(), "bounding set") {
		t.Fatalf("unexpected error: %v", err)
	}
	if err == nil && (len(report.BoundingKept) != 1 || report.BoundingDropped == 0) {
		t.Fatalf("report = %+v", report)
	}
	// A later child, forked from whichever thread runs this goroutine now,
	// keeps the full set (/proc/self/status would show the main thread, which
	// Go may have used and then wedged as the discarded thread).
	later := childStatus(t, func(cmd *exec.Cmd) error { return cmd.Start() })
	if bnd, _ := strconv.ParseUint(statusField(later, "CapBnd"), 16, 64); bnd == 1<<capNetAdmin {
		t.Fatal("the drop leaked to another thread")
	}
}

// TestRealJailerLaunch runs the real jailer and Firecracker (pre-boot: the
// API answers, no guest is started). Gate: CHOIR_JAILER_IT=1, root, and
// CHOIR_JAILER_IT_FIRECRACKER=<firecracker binary> with the jailer beside it
// (or CHOIR_JAILER_IT_JAILER). CHOIR_JAILER_IT_JAILED_FIRECRACKER names the
// jailer's exec file when it is not the firecracker above (Node B: the static
// build in VM_JAILER_FIRECRACKER_BIN). CHOIR_JAILER_IT_CGROUP_PARENT names a
// cgroup v2 parent to place the VM leaf under.
func TestRealJailerLaunch(t *testing.T) {
	if os.Getenv("CHOIR_JAILER_IT") != "1" {
		t.Skip("set CHOIR_JAILER_IT=1 to run the real jailer")
	}
	if os.Geteuid() != 0 {
		t.Skip("the jailer needs root")
	}
	fc := os.Getenv("CHOIR_JAILER_IT_FIRECRACKER")
	if fc == "" {
		t.Skip("CHOIR_JAILER_IT_FIRECRACKER is not set")
	}
	jailer := os.Getenv("CHOIR_JAILER_IT_JAILER")
	if jailer == "" {
		jailer = filepath.Join(filepath.Dir(fc), "jailer")
	}
	stateDir := t.TempDir()
	cfg := DefaultManagerConfig()
	cfg.StateDir = stateDir
	cfg.FirecrackerBinPath = fc
	cfg.Jailer = defaultJailerConfig()
	cfg.Jailer.Enabled = true
	cfg.Jailer.BinPath = jailer
	cfg.Jailer.FirecrackerBinPath = os.Getenv("CHOIR_JAILER_IT_JAILED_FIRECRACKER")
	cfg.Jailer.CgroupParent = os.Getenv("CHOIR_JAILER_IT_CGROUP_PARENT")
	if cfg.Jailer.CgroupParent == "" {
		cfg.Jailer.PidsMax, cfg.Jailer.CPUWeight = 0, 0
	}
	mgr := NewManager(cfg)
	mgr.jailerHostErr = nil
	vmID := "vm-jailer-it"
	if err := os.MkdirAll(mgr.vmStateDir(vmID), 0o750); err != nil {
		t.Fatal(err)
	}
	uid, err := mgr.allocateVMMUID(vmID)
	if err != nil {
		t.Fatal(err)
	}
	// A leftover from a "previous launch" must not trip the jailer.
	if err := os.MkdirAll(mgr.jailRoot(vmID), 0o700); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(mgr.jailRoot(vmID), "firecracker.pid"), []byte("1"), 0o644)
	if _, err := mgr.prepareJail(vmID, uid, nil, nil); err != nil {
		t.Fatalf("prepareJail: %v", err)
	}
	receipt := &ConfinementReceipt{Mode: ConfinementJailer, UID: uid, GID: uid, JailRoot: mgr.jailRoot(vmID), Cgroup: mgr.cgroupLeafRel(vmID)}
	inst := &VMInstance{Config: VMConfig{VMID: vmID}, State: StatePending, done: make(chan struct{})}
	mgr.mu.Lock()
	mgr.vms[vmID] = inst
	err = mgr.startJailedFirecracker(vmID, receipt, []string{"--api-sock", jailAPISocket})
	mgr.mu.Unlock()
	if err != nil {
		t.Fatalf("startJailedFirecracker: %v", err)
	}
	pid := inst.PID
	t.Cleanup(func() {
		if p, err := os.FindProcess(pid); err == nil {
			_ = p.Kill()
		}
	})
	status, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/status")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("pid=%d receipt=%+v launch=%+v", pid, receipt, receipt.LaunchThread)
	want := strings.Repeat(strconv.Itoa(uid)+"\t", 3) + strconv.Itoa(uid)
	if got := statusField(string(status), "Uid"); got != want {
		t.Fatalf("Uid = %q, want %q", got, want)
	}
	for key, value := range map[string]string{"CapEff": "0000000000000000", "CapPrm": "0000000000000000", "CapInh": "0000000000000000", "NoNewPrivs": "1"} {
		if got := statusField(string(status), key); got != value {
			t.Fatalf("%s = %q, want %q", key, got, value)
		}
	}
	// Pre-boot, Firecracker has filtered its API thread; the VMM and vCPU
	// threads are filtered at boot (Node B's running VMs show Seccomp 2 on
	// the main thread).
	tasks, _ := filepath.Glob("/proc/" + strconv.Itoa(pid) + "/task/*/status")
	filtered := 0
	for _, task := range tasks {
		data, _ := os.ReadFile(task)
		if statusField(string(data), "Seccomp") == "2" {
			filtered++
		}
	}
	if filtered == 0 {
		t.Fatalf("no Firecracker thread runs under a seccomp filter (%d threads)", len(tasks))
	}
	// readlink(/proc/<pid>/root) reads "/" across mount namespaces after a
	// pivot_root; the inode behind it proves the chroot.
	procRoot, err := os.Stat("/proc/" + strconv.Itoa(pid) + "/root")
	if err != nil {
		t.Fatal(err)
	}
	jailRoot, _ := os.Stat(mgr.jailRoot(vmID))
	hostRoot, _ := os.Stat("/")
	if !os.SameFile(procRoot, jailRoot) || os.SameFile(procRoot, hostRoot) {
		t.Fatalf("VMM root is not its jail (same as jail=%v, same as host=%v)", os.SameFile(procRoot, jailRoot), os.SameFile(procRoot, hostRoot))
	}
	selfPID, _ := os.Readlink("/proc/self/ns/pid")
	if ns, _ := os.Readlink("/proc/" + strconv.Itoa(pid) + "/ns/pid"); ns == selfPID {
		t.Fatal("the VMM must have its own pid namespace")
	}
	environ, _ := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/environ")
	if len(bytes.Trim(environ, "\x00")) != 0 {
		t.Fatalf("the VMM environment must be empty, got %d bytes", len(environ))
	}
	if !firecrackerCmdlineMatchesVM(pid, vmID) {
		t.Fatal("the jailed Firecracker must identify its VM")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := newFCAPIClient(mgr.jailAPISocketHostPath(vmID)).WaitReady(ctx); err != nil {
		t.Fatalf("API in the jail: %v", err)
	}
	if rec := mgr.loadConfinement(vmID); rec == nil || rec.PID != pid || rec.UID != uid {
		t.Fatalf("persisted receipt = %+v", rec)
	}
	// CHOIR_JAILER_IT_VERIFY=<scripts/vmm_confinement_verify.py>: judge this
	// live jailed VMM with the staging verifier. Checks that need a booted
	// guest, the host firewall or x86 speculation reporting are not asserted.
	if script := os.Getenv("CHOIR_JAILER_IT_VERIFY"); script != "" {
		out, _ := exec.Command("python3", "-I", script, "--state-dir", stateDir, "--vm", vmID, "--expect", "jailer").Output()
		var receipt struct {
			Summary []struct {
				Mode   string   `json:"mode"`
				Failed []string `json:"failed"`
			} `json:"summary"`
		}
		if err := json.Unmarshal(out, &receipt); err != nil || len(receipt.Summary) != 1 {
			t.Fatalf("verifier output: %v\n%s", err, out)
		}
		t.Logf("verifier: mode=%s failed=%v", receipt.Summary[0].Mode, receipt.Summary[0].Failed)
		allowed := map[string]bool{"seccomp_filter": true, "all_threads_seccomp": true, "ssbd_forced": true, "ibpb_stibp_forced": true}
		if receipt.Summary[0].Mode != ConfinementJailer {
			t.Fatalf("verifier mode = %s", receipt.Summary[0].Mode)
		}
		for _, failed := range receipt.Summary[0].Failed {
			if !allowed[failed] {
				t.Fatalf("verifier check %s failed on a live jailed VMM", failed)
			}
		}
	}

	mgr.mu.Lock()
	inst.State = StateRunning
	mgr.killFirecrackerProcess(inst, "it-stop")
	inst.State = StateStopped
	mgr.mu.Unlock()
	select {
	case <-inst.done:
	case <-time.After(5 * time.Second):
		t.Fatal("the pidfd watcher did not see the jailed Firecracker exit")
	}
	if processExists(pid) {
		t.Fatal("jailed Firecracker survived the kill")
	}
	// A second launch over the same jail works (leftovers rebuilt).
	if _, err := mgr.prepareJail(vmID, uid, nil, nil); err != nil {
		t.Fatalf("second prepareJail: %v", err)
	}
}

// catBin resolves cat from PATH; NixOS has no /bin/cat.
func catBin(t *testing.T) string {
	t.Helper()
	p, err := exec.LookPath("cat")
	if err != nil {
		t.Skipf("cat not on PATH: %v", err)
	}
	return p
}
