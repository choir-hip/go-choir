//go:build linux

package capsule

import (
	"slices"
	"testing"

	"github.com/elastic/go-seccomp-bpf/arch"
)

// docs/problems/engineering-completion-path-cluster-2026-10-20.md failure
// 20: the filters allowed fchmodat but not fchmodat2, so GNU cp, whose libc
// sets permissions through fchmodat2, failed every release copy with EPERM.
// Failure modes: an allowed call's newer variant (name+"2", as the kernel
// names its replacements) is refused while the older one is allowed; the
// fix allows one sibling and the next kernel addition repeats the failure.
func TestSeccompAllowsTheNewerVariantOfEveryAllowedCall(t *testing.T) {
	filters := map[string][]string{
		"workload":        capsuleAllowedSyscalls,
		"broker":          append(append([]string(nil), capsuleAllowedSyscalls...), brokerExtraSyscalls...),
		"delegate":        append(append([]string(nil), capsuleAllowedSyscalls...), delegateExtraSyscalls...),
		"broker-delegate": append(append(append([]string(nil), capsuleAllowedSyscalls...), brokerExtraSyscalls...), delegateExtraSyscalls...),
	}
	for _, table := range []*arch.Info{arch.X86_64, arch.AARCH64} {
		for name, allowed := range filters {
			for _, call := range allowed {
				newer := call + "2"
				if _, known := table.SyscallNames[newer]; known && !slices.Contains(allowed, newer) {
					t.Errorf("%s filter allows %s but not %s (%s)", name, call, newer, table.Name)
				}
			}
		}
	}
	if !slices.Contains(capsuleAllowedSyscalls, "fchmodat2") {
		t.Error("workload filter does not allow fchmodat2")
	}
}
