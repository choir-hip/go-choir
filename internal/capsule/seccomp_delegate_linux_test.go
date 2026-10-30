//go:build linux

package capsule

import (
	"errors"
	"os"
	"os/exec"
	"slices"
	"sort"
	"testing"

	"github.com/elastic/go-seccomp-bpf"
	"golang.org/x/sys/unix"
)

// docs/archive/problems/claude-delegate-bun-timer-seccomp-2026-10-15.md: Claude Code
// is a Bun binary. Failure modes, written before the change:
//
//	D1 the delegate filter refuses timerfd_create: uSockets' loop timer
//	   fails and Bun panics at start (us_create_timer returned null, exit
//	   134), the staging failure;
//	D2 the delegate filter refuses rt_sigsuspend: JavaScriptCore's GC
//	   suspend returns at once and the "parked" thread runs on while its
//	   stack is scanned (silent);
//	D3 the delegate filter admits the calls but the broker filter it
//	   inherits does not: the stack still refuses them;
//	D4 the cells' filter (session workers, jobs) gains the calls: the
//	   delegate fix widens every cell;
//	D5 a broker without a delegate gains the calls;
//	D6 the delegate profile gains more than the four calls (namespaces,
//	   Landlock, seccomp, netlink).

func allowedNames(f seccomp.Filter) map[string]bool {
	out := map[string]bool{}
	for _, g := range f.Policy.Syscalls {
		if g.Action != seccomp.ActionAllow {
			continue
		}
		for _, n := range g.Names {
			out[n] = true
		}
	}
	return out
}

func extraNames(wide, narrow seccomp.Filter) []string {
	n := allowedNames(narrow)
	var out []string
	for name := range allowedNames(wide) {
		if !n[name] {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

func socketFamilies(f seccomp.Filter) []uint64 {
	var out []uint64
	for _, g := range f.Policy.Syscalls {
		for _, c := range g.NamesWithCondtions {
			for _, a := range c.Conditions {
				out = append(out, a.Value)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

var bunEventLoopCalls = []string{"rt_sigsuspend", "timerfd_create", "timerfd_gettime", "timerfd_settime"}

func TestDelegateFilterAdmitsBunEventLoopCalls(t *testing.T) { // D1 D2 D3
	for name, f := range map[string]seccomp.Filter{"delegate": DelegateSeccompFilter(), "broker-delegate": BrokerDelegateSeccompFilter()} {
		allowed := allowedNames(f)
		for _, call := range bunEventLoopCalls {
			if !allowed[call] {
				t.Errorf("%s filter refuses %s", name, call)
			}
		}
	}
}

func TestDelegateProfileAddsExactlyTheBunCalls(t *testing.T) { // D6
	want := append([]string(nil), bunEventLoopCalls...)
	sort.Strings(want)
	for name, pair := range map[string][2]seccomp.Filter{
		"delegate vs workload L2":      {DelegateSeccompFilter(), WorkloadInetSeccompFilter()},
		"broker-delegate vs broker L2": {BrokerDelegateSeccompFilter(), BrokerInetSeccompFilter()},
	} {
		if got := extraNames(pair[0], pair[1]); !slices.Equal(got, want) {
			t.Errorf("%s: extra calls %v, want %v", name, got, want)
		}
		if missing := extraNames(pair[1], pair[0]); len(missing) != 0 {
			t.Errorf("%s: delegate profile lost %v", name, missing)
		}
		if got, want := socketFamilies(pair[0]), socketFamilies(pair[1]); !slices.Equal(got, want) {
			t.Errorf("%s: socket families %v, want %v", name, got, want)
		}
	}
}

func TestCellAndPlainBrokerFiltersDoNotGainDelegateCalls(t *testing.T) { // D4 D5
	for name, f := range map[string]seccomp.Filter{
		"workload": WorkloadSeccompFilter(), "workload L2": WorkloadInetSeccompFilter(),
		"broker": BrokerSeccompFilter(), "broker L2": BrokerInetSeccompFilter(),
	} {
		allowed := allowedNames(f)
		for _, call := range bunEventLoopCalls {
			if allowed[call] {
				t.Errorf("%s filter admits %s; only the delegate profile may", name, call)
			}
		}
	}
}

func timerfdErr() error {
	fd, err := unix.TimerfdCreate(unix.CLOCK_MONOTONIC, unix.TFD_CLOEXEC|unix.TFD_NONBLOCK)
	if err != nil {
		return err
	}
	defer unix.Close(fd)
	spec := unix.ItimerSpec{Value: unix.Timespec{Nsec: 16_000_000}}
	if err := unix.TimerfdSettime(fd, 0, &spec, nil); err != nil {
		return err
	}
	var cur unix.ItimerSpec
	return unix.TimerfdGettime(fd, &cur)
}

// The kernel half: a process under the delegate stack (the broker-delegate
// filter it inherits, then the delegate filter) creates, arms and reads a
// timerfd, as Bun's loop does at start; a cell in the same capsule (the
// broker-delegate filter, then the workload filter) still cannot.
func TestDelegateStackRunsATimerfdAndACellStackDoesNot(t *testing.T) {
	switch os.Getenv(seccompHelperEnv) {
	case "delegate-timerfd":
		if err := LoadBrokerDelegateFilter(); err != nil {
			t.Fatalf("load broker-delegate filter: %v", err)
		}
		if err := LoadDelegateFilter(); err != nil {
			t.Fatalf("delegate filter cannot stack under the broker-delegate filter: %v", err)
		}
		if err := timerfdErr(); err != nil {
			t.Fatalf("timerfd under the delegate stack: %v", err)
		}
		return
	case "cell-timerfd":
		if err := LoadBrokerDelegateFilter(); err != nil {
			t.Fatalf("load broker-delegate filter: %v", err)
		}
		if err := LoadWorkloadInetFilter(); err != nil {
			t.Fatalf("load workload L2 filter: %v", err)
		}
		if err := timerfdErr(); !errors.Is(err, unix.EPERM) {
			t.Fatalf("timerfd under a cell's stack = %v, want EPERM", err)
		}
		return
	case "delegate-under-plain-broker":
		// D3: without the broker-side half the delegate filter alone is
		// not enough.
		if err := LoadBrokerInetFilter(); err != nil {
			t.Fatalf("load broker L2 filter: %v", err)
		}
		if err := LoadDelegateFilter(); err != nil {
			t.Fatalf("load delegate filter: %v", err)
		}
		if err := timerfdErr(); !errors.Is(err, unix.EPERM) {
			t.Fatalf("timerfd under plain broker + delegate = %v, want EPERM", err)
		}
		return
	}
	for _, mode := range []string{"delegate-timerfd", "cell-timerfd", "delegate-under-plain-broker"} {
		cmd := exec.Command(os.Args[0], "-test.run=^TestDelegateStackRunsATimerfdAndACellStackDoesNot$")
		cmd.Env = append(os.Environ(), seccompHelperEnv+"="+mode)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("seccomp helper %s: %v\n%s", mode, err, output)
		}
	}
}
