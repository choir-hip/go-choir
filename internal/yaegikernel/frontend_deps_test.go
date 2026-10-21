package yaegikernel

import (
	"strings"
	"testing"
)

// docs/design/delegate-one-round-build-2026-10-21.md: the guest names its
// Nix-built frontend dependencies in CHOIR_FRONTEND_DEPS, and the toolchain
// carry hands that path to capsule workloads, where the read-only store
// resolves it. Failure modes: the path is dropped at a hop; a value that is
// not an absolute store path (relative, outside the store, with "..", with
// a newline) is carried; a process without the guest's build path gains it.
func TestFrontendDepsCarriedOnlyAsAStorePath(t *testing.T) {
	const deps = "/nix/store/eee-go-choir-frontend-deps-0.1.0"
	env := guestEnv()
	env[FrontendDepsEnv] = deps
	carried := "\n" + strings.Join(BuildToolchainCarry(lookupFrom(env)), "\n") + "\n"
	if !strings.Contains(carried, "\n"+FrontendDepsEnv+"="+deps+"\n") {
		t.Fatalf("carry dropped %s: %q", FrontendDepsEnv, carried)
	}
	_, extra := WorkloadToolchainEnv("/bin", lookupFrom(env))
	if !strings.Contains("\n"+strings.Join(extra, "\n")+"\n", "\n"+FrontendDepsEnv+"="+deps+"\n") {
		t.Fatalf("workload env dropped %s: %q", FrontendDepsEnv, extra)
	}
	for _, bad := range []string{"relative/deps", "/etc/deps", "/nix/store/../etc", "/nix/store/x\nPATH=/evil", "/nix/store", " "} {
		env[FrontendDepsEnv] = bad
		if got := strings.Join(BuildToolchainCarry(lookupFrom(env)), "\n"); strings.Contains(got, FrontendDepsEnv) {
			t.Errorf("carry passed %q: %q", bad, got)
		}
		if _, extra := WorkloadToolchainEnv("/bin", lookupFrom(env)); strings.Contains(strings.Join(extra, "\n"), FrontendDepsEnv) {
			t.Errorf("workload env passed %q: %q", bad, extra)
		}
	}
	if got := BuildToolchainCarry(lookupFrom(map[string]string{FrontendDepsEnv: deps})); len(got) != 0 {
		t.Fatalf("a process without the build path gained %q", got)
	}
}
