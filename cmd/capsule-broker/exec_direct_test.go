//go:build linux

package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// TestDirectExecEnvAllowlist proves the canonical runner never inherits the
// broker daemon environment: baseline only, caller keys restricted to the
// allowlist, credential-shaped entries dropped.
func TestDirectExecEnvAllowlist(t *testing.T) {
	env := directExecEnv([]string{
		"HOME=/custom", "LANG=fr_FR.UTF-8",
		"API_KEY=secret", "AUTH_TOKEN=x",
		"LD_PRELOAD=/tmp/evil.so", "PATH=/custom/bin",
		"no-equals-sign",
	})
	joined := "\n" + strings.Join(env, "\n") + "\n"
	for _, want := range []string{"\nHOME=/custom\n", "\nLANG=fr_FR.UTF-8\n", "\nPATH=/custom/bin\n", "\nTMPDIR=/tmp\n"} {
		if !strings.Contains(joined, want) {
			t.Errorf("direct env missing %q in %q", want, joined)
		}
	}
	for _, absent := range []string{"API_KEY", "AUTH_TOKEN", "LD_PRELOAD", "no-equals"} {
		if strings.Contains(joined, absent) {
			t.Errorf("direct env leaked %q in %q", absent, joined)
		}
	}
}

// pathBin resolves a binary from PATH; NixOS has no /bin/echo or /bin/sleep.
func pathBin(t *testing.T, name string) string {
	t.Helper()
	p, err := exec.LookPath(name)
	if err != nil {
		t.Skipf("%s not on PATH: %v", name, err)
	}
	return p
}

// TestHandleExecDirectNoShell proves argv metacharacters are data, not code:
// a shell would expand $(...), ; and $VARS, the direct runner must not.
func TestHandleExecDirectNoShell(t *testing.T) {
	root := t.TempDir()
	b := &Broker{mergedDir: root}
	p := capsule.ExecRequest{Command: pathBin(t, "echo"), Args: []string{"hello; touch pwned", "$(whoami)", "$HOME"}}
	resp := b.handleExecDirect(context.Background(), root, p)
	if resp.Error != "" {
		t.Fatalf("direct exec: %v", resp.Error)
	}
	var got capsule.ExecResult
	if err := json.Unmarshal(resp.Result, &got); err != nil {
		t.Fatal(err)
	}
	if got.ExitCode != 0 || !strings.Contains(got.Stdout, "hello; touch pwned") {
		t.Fatalf("unexpected result: %+v", got)
	}
	if _, err := os.Stat(filepath.Join(root, "pwned")); !os.IsNotExist(err) {
		t.Fatal("shell metacharacters executed: pwned file created")
	}
}

// TestHandleExecDirectEnvIsolated proves ambient broker credentials never
// reach the child: the child observes only the allowlist baseline.
func TestHandleExecDirectEnvIsolated(t *testing.T) {
	t.Setenv("CHOIR_SECRET_TOKEN", "must-not-leak")
	root := t.TempDir()
	probe := filepath.Join(root, "printenv.sh")
	if err := os.WriteFile(probe, []byte("#!/bin/sh\nenv\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	b := &Broker{mergedDir: root}
	p := capsule.ExecRequest{Command: probe, Args: []string{"probe-arg"}}
	resp := b.handleExecDirect(context.Background(), root, p)
	if resp.Error != "" {
		t.Fatalf("direct exec env: %v", resp.Error)
	}
	var got capsule.ExecResult
	if err := json.Unmarshal(resp.Result, &got); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got.Stdout, "CHOIR_SECRET_TOKEN=must-not-leak") {
		t.Fatalf("ambient credential leaked into direct child:\n%s", got.Stdout)
	}
	if !strings.Contains(got.Stdout, "HOME=/tmp") {
		t.Fatalf("allowlist baseline missing from direct child:\n%s", got.Stdout)
	}
}

// TestHandleExecDirectTimeoutReapsGroup proves a timed-out command's process
// group is SIGKILLed and reaped within the 500ms grace, not orphaned.
func TestHandleExecDirectTimeoutReapsGroup(t *testing.T) {
	root := t.TempDir()
	b := &Broker{mergedDir: root}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	resp := b.handleExecDirect(ctx, root, capsule.ExecRequest{Command: pathBin(t, "sleep"), Args: []string{"30"}})
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond+execKillReapGrace+2*time.Second {
		t.Fatalf("reap exceeded grace: %v", elapsed)
	}
	var got capsule.ExecResult
	if err := json.Unmarshal(resp.Result, &got); err != nil {
		t.Fatal(err)
	}
	if got.ExitCode != -1 {
		t.Fatalf("timed-out exec exit = %d, want -1", got.ExitCode)
	}
}

// TestHandleExecRejectsMissingArgs preserves the direct-argv-only boundary.
func TestHandleExecRejectsMissingArgs(t *testing.T) {
	b := &Broker{mergedDir: t.TempDir()}
	params, _ := json.Marshal(capsule.ExecRequest{Command: "echo rejected"})
	resp := b.handleExec(context.Background(), nil, params)
	if resp.Error != "exec_args_required: direct args vector is required" {
		t.Fatalf("missing args response = %+v", resp)
	}
}

func toolchainLookup(env map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		value, ok := env[key]
		return value, ok
	}
}

var guestToolchain = map[string]string{
	yaegikernel.BuildToolchainPathEnv: "/nix/store/aaa-gcc-wrapper/bin:/nix/store/bbb-binutils-wrapper/bin",
	"CGO_CFLAGS":                      "-I/nix/store/ccc-icu4c-dev/include",
	"CGO_CXXFLAGS":                    "-I/nix/store/ccc-icu4c-dev/include",
	"CGO_LDFLAGS":                     "-L/nix/store/ddd-icu4c/lib",
	"PKG_CONFIG_PATH":                 "/nix/store/ccc-icu4c-dev/lib/pkgconfig",
	"RUNTIME_GATEWAY_TOKEN_FILE":      "/run/choir-bootstrap/gateway-token",
}

// Direct exec gets the guest's C toolchain and ICU cgo flags when the broker
// carries them, appended after the baseline PATH (busybox/bash resolution is
// unchanged), and nothing else from the broker's environment
// (docs/archive/problems/selfdev-drill-capsule-cannot-build-autoputer-2026-10-14.md, B1-B4).
func TestDirectExecEnvCarriesBuildToolchain(t *testing.T) {
	env := directExecEnvFrom(toolchainLookup(guestToolchain), nil)
	joined := "\n" + strings.Join(env, "\n") + "\n"
	for _, want := range []string{
		"\nPATH=/run/current-system/sw/bin:/bin:/usr/bin:/nix/store/aaa-gcc-wrapper/bin:/nix/store/bbb-binutils-wrapper/bin\n",
		"\nCGO_ENABLED=1\n", "\nCGO_CFLAGS=-I/nix/store/ccc-icu4c-dev/include\n", "\nCGO_LDFLAGS=-L/nix/store/ddd-icu4c/lib\n",
		"\nPKG_CONFIG_PATH=/nix/store/ccc-icu4c-dev/lib/pkgconfig\n",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("direct env missing %q in %q", strings.TrimSpace(want), env)
		}
	}
	for _, absent := range []string{"GATEWAY", yaegikernel.BuildToolchainPathEnv} {
		if strings.Contains(joined, absent) {
			t.Errorf("direct env leaked %q: %q", absent, env)
		}
	}
	if plain := directExecEnvFrom(toolchainLookup(nil), nil); strings.Contains(strings.Join(plain, "\n"), "CGO_") {
		t.Fatalf("a broker without the toolchain gave one: %q", plain)
	}
}

// The capsule's Go bounds reach direct exec and jobs (both build their
// environment here) as GOMAXPROCS and GOMEMLIMIT, and the session worker as
// the carry, with or without the C toolchain
// (docs/problems/capsule-release-build-runs-at-the-2gib-ceiling-2026-10-22.md, G1).
func TestDirectExecAndWorkerEnvCarryGoBounds(t *testing.T) {
	broker := map[string]string{yaegikernel.CapsuleGoMaxProcsEnv: "1", yaegikernel.CapsuleGoMemLimitEnv: "1342177280"}
	direct := "\n" + strings.Join(directExecEnvFrom(toolchainLookup(broker), nil), "\n") + "\n"
	for _, want := range []string{"\nGOMAXPROCS=1\n", "\nGOMEMLIMIT=1342177280\n"} {
		if !strings.Contains(direct, want) {
			t.Errorf("direct env missing %q in %q", strings.TrimSpace(want), direct)
		}
	}
	if strings.Contains(direct, yaegikernel.CapsuleGoMaxProcsEnv) {
		t.Errorf("direct env exposes the carry: %q", direct)
	}
	worker := "\n" + strings.Join(sessionWorkerEnv(true, toolchainLookup(broker)), "\n") + "\n"
	for _, want := range []string{"\n" + yaegikernel.CapsuleGoMaxProcsEnv + "=1\n", "\n" + yaegikernel.CapsuleGoMemLimitEnv + "=1342177280\n"} {
		if !strings.Contains(worker, want) {
			t.Errorf("worker env missing %q in %q", strings.TrimSpace(want), worker)
		}
	}
}

// A job (the long build) gets the bounds from the broker's own environment,
// as GOMAXPROCS and GOMEMLIMIT, never under the carry names. The job env
// legitimately carries other CHOIR_ names (the frontend deps, and visual
// capture on a granted computer); the broker's environment is set here so
// the test does not depend on the one it runs in.
func TestJobEnvCarriesGoBounds(t *testing.T) {
	t.Setenv(yaegikernel.CapsuleGoMaxProcsEnv, "1")
	t.Setenv(yaegikernel.CapsuleGoMemLimitEnv, "1342177280")
	t.Setenv(yaegikernel.BuildToolchainPathEnv, "/nix/store/aaa-gcc-wrapper/bin")
	t.Setenv(yaegikernel.FrontendDepsEnv, "/nix/store/eee-frontend-deps")
	env := "\n" + strings.Join(newJobManager("/run/capsule/broker", true).Env, "\n") + "\n"
	for _, want := range []string{"\nGOMAXPROCS=1\n", "\nGOMEMLIMIT=1342177280\n", "\nGOFLAGS=-mod=mod\n",
		"\n" + yaegikernel.FrontendDepsEnv + "=/nix/store/eee-frontend-deps\n"} {
		if !strings.Contains(env, want) {
			t.Errorf("job env missing %q in %q", strings.TrimSpace(want), env)
		}
	}
	for _, carry := range []string{yaegikernel.CapsuleGoMaxProcsEnv, yaegikernel.CapsuleGoMemLimitEnv} {
		if strings.Contains(env, "\n"+carry+"=") {
			t.Errorf("job env exposes the carry %s: %q", carry, env)
		}
	}
}

// The documented override for a job: `env NAME=value` in its argv. Run with
// the job manager's own environment, the command sees the override for
// GOMAXPROCS and the default for GOMEMLIMIT.
func TestJobArgvEnvOverridesGoBounds(t *testing.T) {
	t.Setenv(yaegikernel.CapsuleGoMaxProcsEnv, "1")
	t.Setenv(yaegikernel.CapsuleGoMemLimitEnv, "1342177280")
	cmd := exec.Command("env", "GOMAXPROCS=4", "sh", "-c", "echo $GOMAXPROCS:$GOMEMLIMIT")
	cmd.Env = newJobManager("/run/capsule/broker", false).Env
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := strings.TrimSpace(string(out)); got != "4:1342177280" {
		t.Fatalf("job saw %q, want 4:1342177280", got)
	}
}

// The session worker is spawned with the toolchain carry, so its exec action
// (cellExecEnv) can hand it to every choir.Exec a cell runs.
func TestSessionWorkerEnvCarriesBuildToolchain(t *testing.T) {
	env := sessionWorkerEnv(false, toolchainLookup(guestToolchain))
	joined := "\n" + strings.Join(env, "\n") + "\n"
	for _, want := range []string{
		"\nPATH=/run/current-system/sw/bin:/bin:/usr/bin\n", "\nTMPDIR=/tmp\n",
		"\n" + yaegikernel.BuildToolchainPathEnv + "=/nix/store/aaa-gcc-wrapper/bin:/nix/store/bbb-binutils-wrapper/bin\n",
		"\nCGO_CFLAGS=-I/nix/store/ccc-icu4c-dev/include\n", "\nPKG_CONFIG_PATH=/nix/store/ccc-icu4c-dev/lib/pkgconfig\n",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("worker env missing %q in %q", strings.TrimSpace(want), env)
		}
	}
	if strings.Contains(joined, "GATEWAY") || strings.Contains(joined, "HTTP_PROXY") {
		t.Fatalf("worker env leaked: %q", env)
	}
	if inet := sessionWorkerEnv(true, toolchainLookup(nil)); !strings.Contains(strings.Join(inet, "\n"), "HTTP_PROXY=") {
		t.Fatalf("L2 worker lost its egress env: %q", inet)
	}
}
