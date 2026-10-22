package capsule

import (
	"fmt"
	"strings"
)

// The seccomp leg of the isolation self-test (SEC-13 on the deployed
// kernel, POST /api/runtime/isolation-selftest?leg=seccomp). The daemon
// re-execs its own binary as `<bin> capsule-seccomp-selftest <filter>`, once
// per capsule filter. The child loads the filter the capsule's constructor
// builds (seccomp.go), then tries clone3, a clone and an unshare into a new
// user namespace, and two controls (fork+exec, os/exec) that must still work.
//
// It proves the filters on the deployed kernel, not that a live capsule
// loads them, and nothing about the session worker's bounds (RES-03). Ways
// it could mislead, S1-S6:
// docs/problems/fp2-step4-deployed-oom-not-drivable-desk-refuses-runaway-2026-10-22.md.

// SeccompSelfTestArg is the child mode's first argument.
const SeccompSelfTestArg = "capsule-seccomp-selftest"

// SeccompSelfTestFilterNames are the capsule filters the leg loads, each
// built by the constructor the capsule calls (seccomp_selftest_linux.go).
var SeccompSelfTestFilterNames = []string{"workload", "workload-inet", "broker", "broker-inet", "delegate", "broker-delegate"}

// seccompSelfTestWant is each probe's expected result: an errno name prefix,
// or "" for success.
var seccompSelfTestWant = map[string]string{
	"clone3":          "ENOSYS",
	"clone_newuser":   "EPERM",
	"unshare_newuser": "EPERM",
	"fork_exec":       "",
	"os_exec":         "",
}

// SeccompSelfTestFilter is one child's report.
type SeccompSelfTestFilter struct {
	Name string `json:"name"`
	// BPFSHA256 is the hash of the assembled program the child loaded;
	// ExpectedSHA256 is the daemon's, from the same constructor.
	BPFSHA256      string `json:"bpf_sha256"`
	ExpectedSHA256 string `json:"expected_sha256"`
	Instructions   int    `json:"instructions"`
	Loaded         bool   `json:"loaded"`
	LoadError      string `json:"load_error,omitempty"`
	// Seccomp is the child's /proc/self/status Seccomp field after the load.
	Seccomp string `json:"seccomp"`
	// Probes maps a probe to "" (succeeded) or "ERRNO: message".
	Probes map[string]string `json:"probes,omitempty"`
	Error  string            `json:"error,omitempty"`
}

// SeccompSelfTestReceipt is the leg's receipt, judged.
type SeccompSelfTestReceipt struct {
	Kind      string                  `json:"kind"`
	StartedAt string                  `json:"started_at"`
	Kernel    string                  `json:"kernel"`
	Filters   []SeccompSelfTestFilter `json:"filters"`
	OK        bool                    `json:"ok"`
	Failures  []string                `json:"failures,omitempty"`
}

// JudgeSeccompSelfTest returns every way the receipt falls short; empty
// means every capsule filter refused clone3 and a new user namespace on this
// kernel and still let a child start.
func JudgeSeccompSelfTest(rec SeccompSelfTestReceipt) []string {
	var fails []string
	fail := func(format string, args ...any) { fails = append(fails, fmt.Sprintf(format, args...)) }
	if strings.TrimSpace(rec.Kernel) == "" {
		fail("kernel release unread")
	}
	byName := map[string]SeccompSelfTestFilter{}
	for _, f := range rec.Filters {
		if _, dup := byName[f.Name]; dup {
			fail("filter %s reported twice", f.Name)
		}
		byName[f.Name] = f
	}
	for _, name := range SeccompSelfTestFilterNames {
		f, ok := byName[name]
		p := name + ": "
		switch {
		case !ok:
			fail("%sdid not run", p)
			continue
		case f.Error != "":
			fail("%s%s", p, f.Error)
			continue
		case !f.Loaded || f.LoadError != "":
			fail("%sfilter not loaded: %s", p, f.LoadError)
			continue
		}
		if f.BPFSHA256 == "" || f.BPFSHA256 != f.ExpectedSHA256 {
			fail("%sloaded program %q is not the capsule's %q", p, f.BPFSHA256, f.ExpectedSHA256)
		}
		if f.Seccomp != "2" {
			fail("%sSeccomp=%q after the load, want 2", p, f.Seccomp)
		}
		for probe, want := range seccompSelfTestWant {
			got, ran := f.Probes[probe]
			switch {
			case !ran:
				fail("%s%s did not run", p, probe)
			case want == "" && got != "":
				fail("%s%s = %q, want it to work under the filter", p, probe, got)
			case want != "" && !strings.HasPrefix(got, want+":"):
				fail("%s%s = %q, want %s from the filter", p, probe, got, want)
			}
		}
	}
	return fails
}
