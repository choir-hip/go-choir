package doctool

import (
	"context"
	"testing"
)

// The isolation self-test (internal/agentcore, no build tag) uses these on
// every platform; this file has no build tag either, so a darwin build of
// the tests fails if one becomes Linux-only again (review B1).
var _ func(context.Context, string, []string) SelfTestReceipt = SelfTest

func TestJudgeProbeIsPortable(t *testing.T) {
	if f := JudgeProbe(ProbeReport{}, nil); len(f) == 0 {
		t.Fatal("an empty report passed the judge")
	}
}

// Review E4: a read probe is denied only by EACCES or EPERM; EISDIR (an
// unconfined read of a directory) or any other errno proves nothing.
func TestJudgeProbeReadDenialIsAccess(t *testing.T) {
	r := ProbeReport{UID: 61010, GID: 61010, NetNS: "net:[1]",
		Status: map[string]string{"NoNewPrivs": "1", "Seccomp": "2", "CapEff": "0000000000000000"},
		Probes: map[string]string{"read_input": ""}}
	for _, n := range []string{"write_input_dir", "write_tmp", "socket_inet", "socket_unix", "unshare_user", "unshare_net", "mount", "ptrace_parent"} {
		r.Probes[n] = "EPERM: x"
	}
	reads := []string{"/store", "/signers", "/etc/passwd"}
	r.Probes["read:/store"] = "EACCES: permission denied"
	r.Probes["read:/signers"] = "EPERM: operation not permitted"
	r.Probes["read:/etc/passwd"] = "EACCES: permission denied"
	if f := JudgeProbe(r, reads); len(f) != 0 {
		t.Fatalf("denied reads judged %v", f)
	}
	r.Probes["read:/store"] = "EISDIR: is a directory"
	r.Probes["read:/signers"] = "ELOOP: too many levels of symbolic links"
	if f := JudgeProbe(r, reads); len(f) != 2 {
		t.Fatalf("EISDIR and ELOOP judged %v, want 2 failures", f)
	}
}
