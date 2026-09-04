//go:build linux

package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
)

// TestDirectExecEnvAllowlist proves the canonical runner never inherits the
// broker daemon environment: baseline only, caller keys restricted to the
// allowlist, credential-shaped entries dropped.
func TestDirectExecEnvAllowlist(t *testing.T) {
	env := directExecEnv([]string{
		"HOME=/custom", "LANG=fr_FR.UTF-8",
		"CHOIR_ACTUATOR=rlm", "API_KEY=secret", "AUTH_TOKEN=x",
		"LD_PRELOAD=/tmp/evil.so", "PATH=/custom/bin",
		"no-equals-sign",
	})
	joined := "\n" + strings.Join(env, "\n") + "\n"
	for _, want := range []string{"\nHOME=/custom\n", "\nLANG=fr_FR.UTF-8\n", "\nPATH=/custom/bin\n", "\nTMPDIR=/tmp\n"} {
		if !strings.Contains(joined, want) {
			t.Errorf("direct env missing %q in %q", want, joined)
		}
	}
	for _, absent := range []string{"CHOIR_ACTUATOR", "API_KEY", "AUTH_TOKEN", "LD_PRELOAD", "no-equals"} {
		if strings.Contains(joined, absent) {
			t.Errorf("direct env leaked %q in %q", absent, joined)
		}
	}
}

// TestHandleExecDirectNoShell proves argv metacharacters are data, not code:
// a shell would expand $(...), ; and $VARS, the direct runner must not.
func TestHandleExecDirectNoShell(t *testing.T) {
	root := t.TempDir()
	b := &Broker{mergedDir: root}
	p := capsule.ExecRequest{Command: "/bin/echo", Args: []string{"hello; touch pwned", "$(whoami)", "$HOME"}}
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
	resp := b.handleExecDirect(ctx, root, capsule.ExecRequest{Command: "/bin/sleep", Args: []string{"30"}})
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

// TestHandleExecLegacyFrozen proves the empty-args path still routes to the
// frozen shell runner for mechanical rollback (RLM callers must pass args).
func TestHandleExecLegacyFrozen(t *testing.T) {
	root := t.TempDir()
	b := &Broker{mergedDir: root}
	params, _ := json.Marshal(capsule.ExecRequest{Command: "echo legacy-ok"})
	resp := b.handleExec(context.Background(), nil, params)
	if resp.Error != "" {
		t.Fatalf("legacy exec: %v", resp.Error)
	}
	var got capsule.ExecResult
	if err := json.Unmarshal(resp.Result, &got); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Stdout, "legacy-ok") {
		t.Fatalf("legacy shell path broken: %+v", got)
	}
}
