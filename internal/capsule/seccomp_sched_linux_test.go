//go:build linux

package capsule

import (
	"os"
	"os/exec"
	"slices"
	"testing"
	"unsafe"

	"golang.org/x/sys/unix"
)

// docs/problems/engineering-completion-path-cluster-2026-10-20.md failure 5:
// every Node process in a capsule printed "pthread_getschedparam failed: 1"
// because sched_getparam and sched_getscheduler were refused with EPERM. The
// two reads are allowed in every filter built from the shared list; the
// setters are not, in any of them.
func TestSeccompAllowsSchedulingReadsAndNeverTheSetters(t *testing.T) {
	filters := map[string][]string{
		"workload":        capsuleAllowedSyscalls,
		"broker":          append(append([]string(nil), capsuleAllowedSyscalls...), brokerExtraSyscalls...),
		"delegate":        append(append([]string(nil), capsuleAllowedSyscalls...), delegateExtraSyscalls...),
		"broker-delegate": append(append(append([]string(nil), capsuleAllowedSyscalls...), brokerExtraSyscalls...), delegateExtraSyscalls...),
	}
	for name, allowed := range filters {
		for _, want := range []string{"sched_getparam", "sched_getscheduler"} {
			if !slices.Contains(allowed, want) {
				t.Errorf("%s filter does not allow %s", name, want)
			}
		}
		for _, setter := range []string{"sched_setscheduler", "sched_setparam", "sched_setattr", "sched_getattr", "setpriority", "sched_rr_get_interval"} {
			if slices.Contains(allowed, setter) {
				t.Errorf("%s filter allows %s", name, setter)
			}
		}
	}
}

// With the workload filter loaded, the two reads succeed and a setter is
// still refused with EPERM. Runs in a helper process because a filter cannot
// be unloaded.
func TestWorkloadSeccompAdmitsSchedulingReadsAndRefusesTheSetter(t *testing.T) {
	if os.Getenv(seccompHelperEnv) == "sched" {
		if err := LoadWorkloadFilter(); err != nil {
			t.Fatalf("load workload filter: %v", err)
		}
		var param struct{ priority int32 }
		if _, _, errno := unix.Syscall(unix.SYS_SCHED_GETPARAM, 0, uintptr(unsafe.Pointer(&param)), 0); errno != 0 {
			t.Fatalf("sched_getparam: %v", errno)
		}
		if _, _, errno := unix.Syscall(unix.SYS_SCHED_GETSCHEDULER, 0, 0, 0); errno != 0 {
			t.Fatalf("sched_getscheduler: %v", errno)
		}
		// SCHED_OTHER (0) at priority 0 is what the process already has: the
		// call would succeed unfiltered, so only the filter can refuse it.
		if _, _, errno := unix.Syscall(unix.SYS_SCHED_SETSCHEDULER, 0, 0, uintptr(unsafe.Pointer(&param))); errno != unix.EPERM {
			t.Fatalf("sched_setscheduler = %v, want EPERM from the filter", errno)
		}
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestWorkloadSeccompAdmitsSchedulingReadsAndRefusesTheSetter$")
	cmd.Env = append(os.Environ(), seccompHelperEnv+"=sched")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("seccomp helper: %v\n%s", err, output)
	}
}
