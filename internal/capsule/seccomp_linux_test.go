//go:build linux

package capsule

import (
	"errors"
	"os"
	"os/exec"
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
