//go:build linux

package capsule

import (
	"errors"
	"os"
	"os/exec"
	"testing"

	"golang.org/x/sys/unix"
)

// SEC-13: clone3 answers ENOSYS and clone refuses CLONE_NEWUSER, in every
// capsule filter (workload, broker, delegate and their L2 variants).
//
// clone3 takes its flags in a struct seccomp cannot read, so allowing it
// admits any CLONE_NEW* flag unconditionally; ENOSYS makes libc fall back to
// clone, whose flags the filter reads. A nested user namespace hands a
// workload a full capability set inside it, the first step of most kernel
// exploits.
//
// Ways this could fail, written first: the filter admits clone3 (EINVAL, not
// ENOSYS, for an empty struct); the filter admits clone with CLONE_NEWUSER
// (the kernel answers EINVAL for this invalid combination, the filter EPERM);
// the change breaks what children legitimately do: Go's os/exec (clone with
// vfork flags and CLONE_NEWPID in the broker), pthread_create's fallback.

const cloneHelperEnv = "CHOIR_SECCOMP_CLONE_HELPER"

type filterLoader struct {
	name string
	load func() error
}

var cloneFilters = []filterLoader{
	{"workload", LoadWorkloadFilter},
	{"workload-inet", LoadWorkloadInetFilter},
	{"broker", LoadBrokerFilter},
	{"broker-inet", LoadBrokerInetFilter},
	{"delegate", LoadDelegateFilter},
	{"broker-delegate", LoadBrokerDelegateFilter},
}

func TestCapsuleSeccompCloneRules(t *testing.T) {
	if name := os.Getenv(cloneHelperEnv); name != "" {
		for _, f := range cloneFilters {
			if f.name != name {
				continue
			}
			if err := f.load(); err != nil {
				t.Fatalf("load %s filter: %v", name, err)
			}
			// clone3 with a null struct: allowed it is EINVAL/EFAULT from the
			// kernel; refused it is ENOSYS.
			if _, _, errno := unix.Syscall(unix.SYS_CLONE3, 0, 0, 0); errno != unix.ENOSYS {
				t.Fatalf("%s: clone3 = %v, want ENOSYS", name, errno)
			}
			// clone with CLONE_NEWUSER and CLONE_THREAD is invalid to the
			// kernel (EINVAL) and would never create a process; the filter
			// refuses it first.
			if _, _, errno := unix.Syscall6(unix.SYS_CLONE, unix.CLONE_NEWUSER|unix.CLONE_THREAD, 0, 0, 0, 0, 0); errno != unix.EPERM {
				t.Fatalf("%s: clone(CLONE_NEWUSER) = %v, want EPERM", name, errno)
			}
			// What children legitimately do still works: Go's os/exec
			// (clone with vfork flags) starts a process.
			cmd := exec.Command(os.Args[0], "-test.run=^$")
			cmd.Env = []string{"HOME=/tmp", "TMPDIR=/tmp", "PATH=/bin:/usr/bin"}
			if err := cmd.Run(); err != nil {
				t.Fatalf("%s: a child process cannot start: %v", name, err)
			}
			return
		}
		t.Fatalf("unknown filter %q", name)
	}
	for _, f := range cloneFilters {
		f := f
		t.Run(f.name, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestCapsuleSeccompCloneRules$")
			cmd.Env = append(os.Environ(), cloneHelperEnv+"="+f.name)
			if out, err := cmd.CombinedOutput(); err != nil {
				var exitErr *exec.ExitError
				if errors.As(err, &exitErr) {
					t.Fatalf("%s helper failed: %v\n%s", f.name, err, out)
				}
				t.Fatalf("%s helper: %v", f.name, err)
			}
		})
	}
}

// The filters keep clone with CLONE_NEWPID: the broker starts the delegate
// stage in a pid namespace (cmd/capsule-broker/main.go).
func TestCapsuleSeccompKeepsCloneNewPID(t *testing.T) {
	if os.Getenv(cloneHelperEnv) == "newpid" {
		if err := LoadBrokerFilter(); err != nil {
			t.Fatal(err)
		}
		// CLONE_NEWPID with CLONE_THREAD is invalid (EINVAL): the filter did
		// not refuse it first, which is what this checks.
		if _, _, errno := unix.Syscall6(unix.SYS_CLONE, unix.CLONE_NEWPID|unix.CLONE_THREAD, 0, 0, 0, 0, 0); errno == unix.EPERM || errno == unix.ENOSYS {
			t.Fatalf("broker filter refuses clone(CLONE_NEWPID): %v", errno)
		}
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestCapsuleSeccompKeepsCloneNewPID$")
	cmd.Env = append(os.Environ(), cloneHelperEnv+"=newpid")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("helper: %v\n%s", err, out)
	}
}
