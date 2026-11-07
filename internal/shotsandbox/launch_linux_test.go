//go:build linux

package shotsandbox

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// launchOrSkip runs a probe and skips when the host forbids the user
// namespace the sandbox needs (a hardened CI host), failing on anything else.
func launchOrSkip(t *testing.T, spec Spec) Result {
	t.Helper()
	requireSandboxHost(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if spec.WorkDir == "" {
		spec.WorkDir = t.TempDir()
	}
	res, err := Launch(ctx, os.Args[0], spec)
	if err != nil {
		if strings.Contains(err.Error(), "operation not permitted") || strings.Contains(err.Error(), "permission denied") || strings.Contains(err.Error(), "no space left") {
			t.Skipf("this host forbids the sandbox's user namespace: %v", err)
		}
		t.Fatalf("launch: %v", err)
	}
	return res
}

var (
	sandboxHostOnce sync.Once
	sandboxHostErr  string
)

// requireSandboxHost skips when this host lets the sandbox create its user
// namespace but not configure it: a hardened runner (Ubuntu's AppArmor
// userns restriction) leaves the namespace without CAP_NET_ADMIN, so the
// stage cannot bring its loopback up and fails closed. The guest, where
// shots run, is root and configures it; the probe keeps these tests running
// on every host that can.
func requireSandboxHost(t *testing.T) {
	t.Helper()
	sandboxHostOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		res, err := Launch(ctx, os.Args[0], Spec{Probe: "listen:127.0.0.1:0", WorkDir: t.TempDir()})
		switch {
		case err != nil && (strings.Contains(err.Error(), "operation not permitted") || strings.Contains(err.Error(), "permission denied") || strings.Contains(err.Error(), "no space left")):
			sandboxHostErr = err.Error()
		case err == nil && strings.Contains(res.Error, "could not confine itself") && strings.Contains(res.Error, "operation not permitted"):
			sandboxHostErr = res.Error
		}
	})
	if sandboxHostErr != "" {
		t.Skipf("this host forbids configuring the sandbox's namespaces: %s", sandboxHostErr)
	}
}

// nonLoopbackAddr is a host address outside the stage's namespace.
func nonLoopbackAddr(t *testing.T) string {
	t.Helper()
	addrs, _ := net.InterfaceAddrs()
	for _, a := range addrs {
		if ipn, ok := a.(*net.IPNet); ok && !ipn.IP.IsLoopback() && ipn.IP.To4() != nil {
			return ipn.IP.String()
		}
	}
	t.Skip("this host has no non-loopback IPv4 address")
	return ""
}

// FM3: from inside the sandbox, a dial to a host address that has a listener
// fails, the host's own loopback is not the sandbox's loopback, and the
// sandbox's own loopback works.
func TestSandboxCannotDialOutsideLoopback(t *testing.T) {
	l, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	port := l.Addr().(*net.TCPAddr).Port
	for _, target := range []string{net.JoinHostPort(nonLoopbackAddr(t), strconv.Itoa(port)), net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), "192.0.2.1:80"} {
		res := launchOrSkip(t, Spec{Probe: "dial:" + target})
		if !strings.HasPrefix(res.Probe, "refused") {
			t.Fatalf("dial %s from the sandbox = %q (error %q), want refused", target, res.Probe, res.Error)
		}
	}
	// Positive control: the sandbox does have a working loopback, so the
	// refusals above are the namespace's, not a broken stage.
	if res := launchOrSkip(t, Spec{Probe: "listen:127.0.0.1:0"}); !strings.HasPrefix(res.Probe, "listening") {
		t.Fatalf("the sandbox's own loopback = %q (error %q)", res.Probe, res.Error)
	}
}

// The workload seccomp filter is loaded: netlink and packet sockets are
// refused, inet and unix are not.
func TestSandboxSeccompRefusesNetlinkAndPacketSockets(t *testing.T) {
	for family, want := range map[string]string{"netlink": "refused", "packet": "refused", "inet": "opened", "unix": "opened"} {
		res := launchOrSkip(t, Spec{Probe: "socket:" + family})
		if !strings.HasPrefix(res.Probe, want) {
			t.Errorf("socket %s = %q (error %q), want %s", family, res.Probe, res.Error, want)
		}
	}
}

// A whole pass under the namespace and the filter: the stage starts the
// candidate (a child process), serves the frontend, proxies as the owner and
// resets the desktop per view, and the files it names are in OutDir.
func TestLaunchRunsAServerPairUnderTheSandbox(t *testing.T) {
	spec := stageSpec(t, "ok")
	// The sandbox's paths must be reachable by the id it runs as; the test
	// runs unprivileged, so that id is the caller's own.
	res := launchOrSkip(t, spec)
	if res.Error != "" || len(res.Files) != 2 {
		t.Fatalf("result = %+v", res)
	}
	got, err := os.ReadFile(filepath.Join(spec.OutDir, "home.png"))
	if err != nil || !strings.Contains(string(got), "user=owner") || !strings.Contains(string(got), "trust=1") {
		t.Fatalf("shot body = %q, %v", got, err)
	}
	resets, _ := os.ReadFile(filepath.Join(spec.WorkDir, "resets"))
	if strings.Count(string(resets), "reset") != 2 {
		t.Fatalf("resets = %q", resets)
	}
}

// FM9 through the sandbox: a candidate that dies on start is a Result with
// its stderr tail, not an error and not a hang.
func TestLaunchCandidateDeathIsAResultWithTail(t *testing.T) {
	res := launchOrSkip(t, stageSpec(t, "die"))
	if !strings.Contains(res.Error, "exited before port") || !strings.Contains(res.Tail, "store schema is newer") {
		t.Fatalf("result = %+v", res)
	}
}

// FM6: the launcher's context ends a stage that is waiting on a candidate
// that never serves; Launch returns and does not leave the group running.
func TestLaunchContextEndKillsTheStage(t *testing.T) {
	requireSandboxHost(t)
	spec := stageSpec(t, "silent")
	spec.ReadyTimeoutS = 600
	spec.WorkDir = t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	started := time.Now()
	_, err := Launch(ctx, os.Args[0], spec)
	if err == nil {
		t.Fatal("a stage waiting on a silent candidate returned a result")
	}
	if strings.Contains(err.Error(), "operation not permitted") || strings.Contains(err.Error(), "permission denied") {
		t.Skipf("this host forbids the sandbox's user namespace: %v", err)
	}
	if time.Since(started) > 30*time.Second {
		t.Fatalf("launch outlived its context by %s", time.Since(started))
	}
}
