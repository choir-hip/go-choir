package yaegikernel

import (
	"strings"
	"testing"
)

// The C toolchain and ICU cgo environment reach capsule workloads
// (docs/archive/problems/selfdev-drill-capsule-cannot-build-autoputer-2026-10-14.md,
// option B). Ways it could fail:
//
//	B1. a hop drops a variable, so gcc or the ICU headers are missing again;
//	B2. a non-path value (a relative dir, "..", an empty entry) widens PATH;
//	B3. the toolchain is put ahead of the existing PATH, so `sh` or another
//	    tool resolves to a different file than before;
//	B4. a process without the guest's build path (a desk session outside a
//	    capsule) gains a toolchain or a CGO_ENABLED it never had.
func lookupFrom(env map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		value, ok := env[key]
		return value, ok
	}
}

func guestEnv() map[string]string {
	return map[string]string{
		BuildToolchainPathEnv:        "/nix/store/aaa-gcc-wrapper-15.2.0/bin:/nix/store/bbb-binutils-wrapper-2.44/bin",
		"CGO_CFLAGS":                 "-I/nix/store/ccc-icu4c-78.3-dev/include",
		"CGO_CXXFLAGS":               "-I/nix/store/ccc-icu4c-78.3-dev/include",
		"CGO_LDFLAGS":                "-L/nix/store/ddd-icu4c-78.3/lib",
		"PKG_CONFIG_PATH":            "/nix/store/ccc-icu4c-78.3-dev/lib/pkgconfig",
		"RUNTIME_GATEWAY_TOKEN_FILE": "/run/choir-bootstrap/gateway-token",
	}
}

// A cell's choir.Exec (the session worker's exec action) runs with the
// toolchain the worker was spawned with; the payload's credential filter is
// unchanged and a payload cannot drop the toolchain.
func TestCellExecEnvCarriesBuildToolchain(t *testing.T) {
	env := cellExecEnv(map[string]string{"GOFLAGS": "-p=1", "CHOIR_X": "drop", "API_KEY": "drop"}, lookupFrom(guestEnv()))
	joined := "\n" + strings.Join(env, "\n") + "\n"
	for _, want := range []string{
		"\nPATH=/run/current-system/sw/bin:/usr/bin:/bin:/nix/store/aaa-gcc-wrapper-15.2.0/bin:/nix/store/bbb-binutils-wrapper-2.44/bin\n",
		"\nCGO_ENABLED=1\n", "\nCGO_CFLAGS=-I/nix/store/ccc-icu4c-78.3-dev/include\n",
		"\nPKG_CONFIG_PATH=/nix/store/ccc-icu4c-78.3-dev/lib/pkgconfig\n", "\nGOFLAGS=-p=1\n", "\nHOME=/tmp\n",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("cell exec env missing %q in %q", strings.TrimSpace(want), env)
		}
	}
	for _, absent := range []string{"CHOIR_X", "API_KEY", BuildToolchainPathEnv, "GATEWAY"} {
		if strings.Contains(joined, absent) {
			t.Errorf("cell exec env leaked %q: %q", absent, env)
		}
	}
	plain := cellExecEnv(nil, lookupFrom(map[string]string{}))
	if strings.Join(plain, ",") != "PATH=/run/current-system/sw/bin:/usr/bin:/bin,HOME=/tmp,TMPDIR=/tmp" {
		t.Fatalf("outside a toolchain capsule: %q", plain)
	}
}

func TestBuildToolchainCarryPassesOnlyToolchainKeys(t *testing.T) {
	carried := BuildToolchainCarry(lookupFrom(guestEnv()))
	joined := "\n" + strings.Join(carried, "\n") + "\n"
	for _, key := range append([]string{BuildToolchainPathEnv}, BuildToolchainKeys...) {
		if !strings.Contains(joined, "\n"+key+"=") {
			t.Errorf("carry dropped %s: %q", key, carried)
		}
	}
	if strings.Contains(joined, "GATEWAY") || len(carried) != 1+len(BuildToolchainKeys) {
		t.Fatalf("carry passed a non-toolchain key: %q", carried)
	}
	if got := BuildToolchainCarry(lookupFrom(map[string]string{})); len(got) != 0 {
		t.Fatalf("carry invented a toolchain: %q", got)
	}
}

func TestWorkloadToolchainEnvAppendsTheBuildPath(t *testing.T) {
	const base = "/run/current-system/sw/bin:/bin:/usr/bin"
	path, extra := WorkloadToolchainEnv(base, lookupFrom(guestEnv()))
	want := base + ":/nix/store/aaa-gcc-wrapper-15.2.0/bin:/nix/store/bbb-binutils-wrapper-2.44/bin"
	if path != want {
		t.Fatalf("PATH = %q, want %q", path, want)
	}
	joined := "\n" + strings.Join(extra, "\n") + "\n"
	for _, want := range []string{
		"\nCGO_ENABLED=1\n",
		"\nCGO_CFLAGS=-I/nix/store/ccc-icu4c-78.3-dev/include\n",
		"\nCGO_CXXFLAGS=-I/nix/store/ccc-icu4c-78.3-dev/include\n",
		"\nCGO_LDFLAGS=-L/nix/store/ddd-icu4c-78.3/lib\n",
		"\nPKG_CONFIG_PATH=/nix/store/ccc-icu4c-78.3-dev/lib/pkgconfig\n",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("workload env missing %q in %q", strings.TrimSpace(want), extra)
		}
	}
	if strings.Contains(joined, BuildToolchainPathEnv) {
		t.Errorf("workload env exposes the carry variable: %q", extra)
	}

	// B4: no build path, no toolchain.
	path, extra = WorkloadToolchainEnv(base, lookupFrom(map[string]string{"CGO_CFLAGS": "-I/x"}))
	if path != base || len(extra) != 0 {
		t.Fatalf("without a build path: PATH = %q, extra = %q", path, extra)
	}

	// B2: only absolute, clean entries join PATH.
	env := guestEnv()
	env[BuildToolchainPathEnv] = "relative/bin::/nix/store/aaa-gcc/bin:/nix/../etc:/nix/store/bbb/bin/"
	path, _ = WorkloadToolchainEnv(base, lookupFrom(env))
	if path != base+":/nix/store/aaa-gcc/bin:/nix/store/bbb/bin" {
		t.Fatalf("PATH = %q", path)
	}
}
