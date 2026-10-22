package capsule

import "testing"

// The seccomp leg of the isolation self-test (SEC-13 on the deployed
// kernel; failure modes S1-S6 in
// docs/problems/fp2-step4-deployed-oom-not-drivable-desk-refuses-runaway-2026-10-22.md).

func goodSeccompReceipt() SeccompSelfTestReceipt {
	rec := SeccompSelfTestReceipt{Kind: "capsule_seccomp_selftest", Kernel: "6.1.155"}
	for i, name := range SeccompSelfTestFilterNames {
		h := string(rune('a'+i)) + "0f3"
		rec.Filters = append(rec.Filters, SeccompSelfTestFilter{
			Name: name, BPFSHA256: h, ExpectedSHA256: h, Instructions: 120, Loaded: true, Seccomp: "2",
			Probes: map[string]string{
				"clone3":          "ENOSYS: function not implemented",
				"clone_newuser":   "EPERM: operation not permitted",
				"unshare_newuser": "EPERM: operation not permitted",
				"fork_exec":       "",
				"os_exec":         "",
			},
		})
	}
	return rec
}

func TestJudgeSeccompSelfTestAcceptsTheFilterOnTheKernel(t *testing.T) {
	if f := JudgeSeccompSelfTest(goodSeccompReceipt()); len(f) != 0 {
		t.Fatalf("good receipt refused: %v", f)
	}
}

func TestJudgeSeccompSelfTestRefusesEveryShortfall(t *testing.T) {
	cases := map[string]func(*SeccompSelfTestReceipt){
		"no kernel":         func(r *SeccompSelfTestReceipt) { r.Kernel = "" },
		"filter missing":    func(r *SeccompSelfTestReceipt) { r.Filters = r.Filters[1:] },
		"child error":       func(r *SeccompSelfTestReceipt) { r.Filters[0].Error = "exit status 2" },
		"not loaded":        func(r *SeccompSelfTestReceipt) { r.Filters[0].Loaded = false },
		"load error":        func(r *SeccompSelfTestReceipt) { r.Filters[0].LoadError = "EINVAL" },
		"hash differs":      func(r *SeccompSelfTestReceipt) { r.Filters[0].BPFSHA256 = "ffff" },
		"hash empty":        func(r *SeccompSelfTestReceipt) { r.Filters[0].BPFSHA256 = ""; r.Filters[0].ExpectedSHA256 = "" },
		"seccomp off":       func(r *SeccompSelfTestReceipt) { r.Filters[0].Seccomp = "0" },
		"clone3 reached":    func(r *SeccompSelfTestReceipt) { r.Filters[1].Probes["clone3"] = "EINVAL: invalid argument" },
		"clone3 allowed":    func(r *SeccompSelfTestReceipt) { r.Filters[1].Probes["clone3"] = "" },
		"newuser reached":   func(r *SeccompSelfTestReceipt) { r.Filters[2].Probes["clone_newuser"] = "EINVAL: invalid argument" },
		"unshare reached":   func(r *SeccompSelfTestReceipt) { r.Filters[3].Probes["unshare_newuser"] = "EINVAL: invalid argument" },
		"fork broken":       func(r *SeccompSelfTestReceipt) { r.Filters[4].Probes["fork_exec"] = "EPERM: operation not permitted" },
		"os/exec broken":    func(r *SeccompSelfTestReceipt) { r.Filters[5].Probes["os_exec"] = "ENOSYS: function not implemented" },
		"probe did not run": func(r *SeccompSelfTestReceipt) { delete(r.Filters[0].Probes, "os_exec") },
		"duplicate filter":  func(r *SeccompSelfTestReceipt) { r.Filters[1] = r.Filters[0] },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			r := goodSeccompReceipt()
			mutate(&r)
			if f := JudgeSeccompSelfTest(r); len(f) == 0 {
				t.Fatalf("%s: judged ok", name)
			}
		})
	}
}
