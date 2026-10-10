//go:build linux

package capsule

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
	"testing"

	"golang.org/x/sys/unix"
)

const seccompHelperEnv = "CHOIR_SECCOMP_TEST_HELPER"

func TestWorkloadSeccompFilterLoadsAndDeniesINET(t *testing.T) {
	if os.Getenv(seccompHelperEnv) == "1" {
		if err := LoadWorkloadFilter(); err != nil {
			t.Fatalf("load workload filter: %v", err)
		}
		fd, err := unix.Socket(unix.AF_INET, unix.SOCK_STREAM, 0)
		if fd >= 0 {
			_ = unix.Close(fd)
		}
		if !errors.Is(err, unix.EPERM) {
			t.Fatalf("AF_INET socket error = %v, want EPERM", err)
		}
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestWorkloadSeccompFilterLoadsAndDeniesINET$")
	cmd.Env = append(os.Environ(), seccompHelperEnv+"=1")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("seccomp helper: %v\n%s", err, output)
	}
}

// Broker go_eval and exec start a child with SysProcAttr.Setpgid. The filter
// already allowed clone/execve; without setpgid, Start returns EPERM
// ("fork/exec ...: operation not permitted") before the worker runs.
func TestBrokerSeccompAllowsSetpgidChildStart(t *testing.T) {
	if os.Getenv(seccompHelperEnv) == "setpgid-exec" {
		if err := LoadBrokerFilter(); err != nil {
			t.Fatalf("load broker filter: %v", err)
		}
		cmd := exec.Command(os.Args[0], "-test.run=^$")
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		cmd.Env = []string{"HOME=/tmp", "TMPDIR=/tmp", "PATH=/bin:/usr/bin"}
		if err := cmd.Run(); err != nil {
			t.Fatalf("setpgid child start: %v", err)
		}
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestBrokerSeccompAllowsSetpgidChildStart$")
	cmd.Env = append(os.Environ(), seccompHelperEnv+"=setpgid-exec")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("seccomp helper: %v\n%s", err, output)
	}
}

// docs/problems/capsule-session-worker-dies-at-start-2026-10-09.md: the
// session worker inherits the broker's filter and must apply its own
// Landlock under it. Failure mode: the broker filter refuses the Landlock
// syscalls (EPERM, reported by go-landlock as "ABI v0"), so every worker
// dies before ready. The workload filter still refuses them to model code.
func landlockVersionErr() error {
	_, _, errno := unix.Syscall(unix.SYS_LANDLOCK_CREATE_RULESET, 0, 0, unix.LANDLOCK_CREATE_RULESET_VERSION)
	if errno != 0 {
		return errno
	}
	return nil
}

func TestBrokerSeccompAllowsLandlockForSessionWorker(t *testing.T) {
	switch os.Getenv(seccompHelperEnv) {
	case "landlock-broker":
		if err := LoadBrokerFilter(); err != nil {
			t.Fatalf("load broker filter: %v", err)
		}
		if err := landlockVersionErr(); errors.Is(err, unix.EPERM) {
			t.Fatalf("broker filter refuses landlock_create_ruleset: %v", err)
		}
		return
	case "worker-filter-under-broker":
		// The worker's third layer: stack the workload filter under the
		// inherited broker filter (seccomp() must not be refused).
		if err := LoadBrokerFilter(); err != nil {
			t.Fatalf("load broker filter: %v", err)
		}
		if err := LoadWorkloadFilter(); err != nil {
			t.Fatalf("workload filter cannot stack under the broker filter: %v", err)
		}
		if err := landlockVersionErr(); !errors.Is(err, unix.EPERM) {
			t.Fatalf("stacked workload filter allows landlock_create_ruleset: %v", err)
		}
		return
	case "landlock-workload":
		if err := LoadWorkloadFilter(); err != nil {
			t.Fatalf("load workload filter: %v", err)
		}
		if err := landlockVersionErr(); !errors.Is(err, unix.EPERM) {
			t.Fatalf("workload filter allows landlock_create_ruleset to model code: %v", err)
		}
		return
	}
	for _, mode := range []string{"landlock-broker", "worker-filter-under-broker", "landlock-workload"} {
		cmd := exec.Command(os.Args[0], "-test.run=^TestBrokerSeccompAllowsLandlockForSessionWorker$")
		cmd.Env = append(os.Environ(), seccompHelperEnv+"="+mode)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("seccomp helper %s: %v\n%s", mode, err, output)
		}
	}
}

// docs/design/engineering-network-grants-2026-10-10.md §4.0b: the L2 filter
// admits AF_INET/AF_INET6 so native tools reach the loopback egress
// forwarder. Failure modes: the variant still refuses inet (L2 is dead on
// arrival), or it also admits netlink or packet sockets (interface and route
// manipulation, raw frames).
func TestWorkloadInetSeccompFilterAdmitsOnlyInetAndUnix(t *testing.T) {
	if os.Getenv(seccompHelperEnv) == "inet" {
		if err := LoadWorkloadInetFilter(); err != nil {
			t.Fatalf("load L2 workload filter: %v", err)
		}
		for _, family := range []int{unix.AF_UNIX, unix.AF_INET, unix.AF_INET6} {
			fd, err := unix.Socket(family, unix.SOCK_STREAM, 0)
			if err != nil {
				t.Fatalf("family %d refused under the L2 filter: %v", family, err)
			}
			_ = unix.Close(fd)
		}
		for _, family := range []int{unix.AF_NETLINK, unix.AF_PACKET} {
			fd, err := unix.Socket(family, unix.SOCK_RAW, 0)
			if fd >= 0 {
				_ = unix.Close(fd)
			}
			if !errors.Is(err, unix.EPERM) {
				t.Fatalf("family %d error = %v, want EPERM", family, err)
			}
		}
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestWorkloadInetSeccompFilterAdmitsOnlyInetAndUnix$")
	cmd.Env = append(os.Environ(), seccompHelperEnv+"=inet")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("seccomp helper: %v\n%s", err, output)
	}
}
