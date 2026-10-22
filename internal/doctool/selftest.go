package doctool

import (
	"fmt"
	"os"
	"strings"
)

// The probe report, the self-test receipt and the judge are portable: the
// isolation self-test (internal/agentcore, every platform) carries the
// receipt; only running the probe is Linux (probe_linux.go).

// ProbeReport is what the probe tool saw from inside the sandbox.
type ProbeReport struct {
	UID    int               `json:"uid"`
	GID    int               `json:"gid"`
	Status map[string]string `json:"status"`
	NetNS  string            `json:"net_ns"`
	// Probes maps a probe name to "" when the operation succeeded, else
	// "ERRNO: message".
	Probes map[string]string `json:"probes"`
}

// SelfTestReceipt is the deployed proof of the document-tool sandbox: the
// probe tool's report from inside it on the guest, the warnings of the run,
// and what the judge found wrong.
type SelfTestReceipt struct {
	Mode Mode `json:"mode"`
	// Observe is observe's shadow counts since start (ShadowSnapshot).
	Observe  ShadowStats  `json:"observe"`
	Report   *ProbeReport `json:"report,omitempty"`
	Warnings []string     `json:"warnings,omitempty"`
	Failures []string     `json:"failures,omitempty"`
}

// JudgeProbe lists what is wrong with a probe report: the identity, the
// confinement status, the namespace, an allowed probe that failed, or a
// denied probe that succeeded or failed only with ENOENT (a read: probe
// must fail with EACCES or EPERM).
func JudgeProbe(r ProbeReport, reads []string) []string {
	var failures []string
	if r.UID == 0 || r.GID == 0 {
		failures = append(failures, fmt.Sprintf("tool ran as uid %d gid %d", r.UID, r.GID))
	}
	if r.Status["NoNewPrivs"] != "1" || r.Status["Seccomp"] != "2" || r.Status["CapEff"] != "0000000000000000" {
		failures = append(failures, fmt.Sprintf("tool status %v: want NoNewPrivs 1, Seccomp 2, CapEff 0", r.Status))
	}
	if own, _ := os.Readlink("/proc/self/ns/net"); r.NetNS == "" || r.NetNS == own {
		failures = append(failures, "tool shares the daemon's network namespace")
	}
	if r.Probes["read_input"] != "" {
		failures = append(failures, "tool could not read its input: "+r.Probes["read_input"])
	}
	denied := []string{"write_input_dir", "write_tmp", "socket_inet", "socket_unix", "unshare_user", "unshare_net", "mount", "ptrace_parent"}
	for _, p := range reads {
		denied = append(denied, "read:"+p)
	}
	for _, name := range denied {
		got, ok := r.Probes[name]
		switch {
		case !ok:
			failures = append(failures, "probe "+name+" missing")
		case got == "":
			failures = append(failures, "probe "+name+" succeeded inside the sandbox")
		case strings.HasPrefix(got, "ENOENT"):
			failures = append(failures, "probe "+name+" failed with ENOENT, which proves nothing")
		case strings.HasPrefix(name, "read:") && !strings.HasPrefix(got, "EACCES") && !strings.HasPrefix(got, "EPERM"):
			// Review E4: only an access denial is a denied read; EISDIR or
			// ELOOP would fail outside the sandbox too.
			failures = append(failures, "probe "+name+" failed with "+got+", not an access denial")
		}
	}
	return failures
}
