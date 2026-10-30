package capsule

import (
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// The executor's broker launch carries the guest's build toolchain to the
// broker (hop 1 of B1 in
// docs/archive/problems/selfdev-drill-capsule-cannot-build-autoputer-2026-10-14.md)
// and keeps the PATH rule the broker's sh resolution depends on; nothing
// else from the autoputer's environment (credentials, gateway token files)
// crosses.
func TestBrokerProcessEnvCarriesBuildToolchain(t *testing.T) {
	autoputer := map[string]string{
		"PATH":                            "/nix/store/x-busybox/bin:/nix/store/y-gcc/bin",
		yaegikernel.BuildToolchainPathEnv: "/nix/store/y-gcc/bin:/nix/store/z-binutils/bin",
		"CGO_CFLAGS":                      "-I/nix/store/icu-dev/include",
		"CGO_CXXFLAGS":                    "-I/nix/store/icu-dev/include",
		"CGO_LDFLAGS":                     "-L/nix/store/icu/lib",
		"PKG_CONFIG_PATH":                 "/nix/store/icu-dev/lib/pkgconfig",
		"RUNTIME_GATEWAY_TOKEN_FILE":      "/run/choir-bootstrap/gateway-token",
		"CHOIR_PRIVACY_KEY_FILE":          "/mnt/persistent/choir-credentials/privacy-key",
	}
	lookup := func(key string) (string, bool) { v, ok := autoputer[key]; return v, ok }
	env := BrokerProcessEnv(lookup)
	joined := "\n" + strings.Join(env, "\n") + "\n"
	for _, want := range []string{
		"\nPATH=/run/current-system/sw/bin:/nix/store/x-busybox/bin:/nix/store/y-gcc/bin:/bin:/usr/bin\n",
		"\nHOME=/root\n", "\nTMPDIR=/tmp\n",
		"\n" + yaegikernel.BuildToolchainPathEnv + "=/nix/store/y-gcc/bin:/nix/store/z-binutils/bin\n",
		"\nCGO_CFLAGS=-I/nix/store/icu-dev/include\n", "\nCGO_LDFLAGS=-L/nix/store/icu/lib\n",
		"\nPKG_CONFIG_PATH=/nix/store/icu-dev/lib/pkgconfig\n",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("broker env missing %q in %q", strings.TrimSpace(want), env)
		}
	}
	for _, absent := range []string{"GATEWAY", "PRIVACY", "CGO_ENABLED"} {
		if strings.Contains(joined, absent) {
			t.Errorf("broker env leaked %q: %q", absent, env)
		}
	}
	plain := BrokerProcessEnv(func(string) (string, bool) { return "", false })
	if strings.Join(plain, ",") != "PATH=/run/current-system/sw/bin:/bin:/usr/bin:/bin:/usr/bin,HOME=/root,TMPDIR=/tmp" {
		t.Fatalf("without an autoputer PATH: %q", plain)
	}
}

// G8 (internal/yaegikernel/go_bounds_test.go): the broker launch never takes
// the Go bounds from the autoputer's own environment; only the executor
// sets them, from the capsule (capsuleBrokerGoBoundsEnv).
func TestBrokerProcessEnvIgnoresAmbientGoBounds(t *testing.T) {
	autoputer := map[string]string{
		yaegikernel.BuildToolchainPathEnv: "/nix/store/y-gcc/bin",
		yaegikernel.CapsuleGoMaxProcsEnv:  "64",
		yaegikernel.CapsuleGoMemLimitEnv:  "1073741824",
	}
	env := BrokerProcessEnv(func(key string) (string, bool) { v, ok := autoputer[key]; return v, ok })
	if joined := strings.Join(env, "\n"); strings.Contains(joined, "CHOIR_CAPSULE_GO") {
		t.Fatalf("broker env took ambient bounds: %q", env)
	}
}
