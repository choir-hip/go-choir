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
