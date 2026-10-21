//go:build linux

package capfloor

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
)

// The test binary plays the daemon: started as root in a user namespace (so
// it holds every capability there, as the guest daemon does), it applies the
// floor in enforce mode at the top of "main", re-executes itself, and the
// re-executed image reports the bounding set of every one of its threads.
func TestMain(m *testing.M) {
	if os.Getenv("CAPFLOOR_TEST_DAEMON") == "1" {
		err := Apply(ModeFromEnv(os.Getenv))
		r := map[string]any{"error": "", "reexecs": os.Getenv("CAPFLOOR_TEST_REEXECS")}
		if err != nil {
			r["error"] = err.Error()
		}
		r["threads"] = threadCapBnds()
		_ = json.NewEncoder(os.Stdout).Encode(r)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func threadCapBnds() []string {
	entries, _ := os.ReadDir("/proc/self/task")
	var out []string
	for _, e := range entries {
		raw, err := os.ReadFile("/proc/self/task/" + e.Name() + "/status")
		if err != nil {
			continue
		}
		v, err := parseCapBnd(string(raw))
		if err == nil {
			out = append(out, Hex(v))
		}
	}
	return out
}

func runAsNamespaceRoot(t *testing.T, mode string) map[string]any {
	t.Helper()
	if out, err := exec.Command("unshare", "-Ur", "true").CombinedOutput(); err != nil {
		t.Skipf("unprivileged user namespaces unavailable: %v %s", err, out)
	}
	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin)
	cmd.Env = []string{"CAPFLOOR_TEST_DAEMON=1", ModeEnv + "=" + mode}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:  syscall.CLONE_NEWUSER,
		UidMappings: []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Getuid(), Size: 1}},
		GidMappings: []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Getgid(), Size: 1}},
		// Root in the namespace with a full effective set, as a root
		// service starts.
		AmbientCaps: nil,
	}
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("daemon stand-in: %v (%s)", err, out)
	}
	var r map[string]any
	if err := json.Unmarshal(out, &r); err != nil {
		t.Fatalf("output %q: %v", out, err)
	}
	return r
}

// K2, K3: enforce leaves every thread of the re-executed daemon with exactly
// the floor, after one re-exec.
func TestEnforceDropsOnEveryThread(t *testing.T) {
	r := runAsNamespaceRoot(t, "enforce")
	if r["error"] != "" {
		t.Fatalf("enforce: %v", r["error"])
	}
	threads, _ := r["threads"].([]any)
	if len(threads) < 2 {
		t.Fatalf("threads %v: want the Go runtime's several", threads)
	}
	want := Hex(keptMask())
	for _, th := range threads {
		if th != want {
			t.Fatalf("a thread's CapBnd is %v, want %s (all %v)", th, want, threads)
		}
	}
}

// Observe changes nothing.
func TestObserveDropsNothing(t *testing.T) {
	r := runAsNamespaceRoot(t, "observe")
	if r["error"] != "" {
		t.Fatalf("observe: %v", r["error"])
	}
	threads, _ := r["threads"].([]any)
	for _, th := range threads {
		if th == Hex(keptMask()) {
			t.Fatalf("observe dropped capabilities: %v", threads)
		}
	}
}

// K4: a marked start that still holds a capability outside the floor is
// refused, and never re-executes again.
func TestMarkedStartWithExtraCapabilitiesIsRefused(t *testing.T) {
	t.Setenv(appliedEnv, "1")
	err := Apply(ModeEnforce)
	own, _ := os.ReadFile("/proc/self/status")
	bnd, _ := parseCapBnd(string(own))
	if extra, _ := Plan(bnd); len(extra) == 0 {
		if err != nil {
			t.Fatalf("a marked start within the floor was refused: %v", err)
		}
		return
	}
	if err == nil || !strings.Contains(err.Error(), "still holds") {
		t.Fatalf("marked start with extra capabilities: %v", err)
	}
	if os.Getenv(appliedEnv) != "1" {
		t.Fatal("the marker changed")
	}
}

// Enforce without the privilege to drop (a non-root start) refuses rather
// than running with the full set.
func TestEnforceWithoutPrivilegeIsRefused(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("as root the drop can succeed")
	}
	own, _ := os.ReadFile("/proc/self/status")
	bnd, _ := parseCapBnd(string(own))
	if extra, _ := Plan(bnd); len(extra) == 0 {
		t.Skip("this process is already within the floor")
	}
	if err := Apply(ModeEnforce); err == nil {
		t.Fatal("enforce without CAP_SETPCAP succeeded")
	}
}
