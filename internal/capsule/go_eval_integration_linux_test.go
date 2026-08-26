//go:build linux

package capsule

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestCapsuleGoEvalEndToEnd is opt-in (needs root, cgroup v2, overlayfs,
// namespaces, Landlock, seccomp, and the immutable broker) — same harness as
// TestExecutorInheritedBrokerListenerEndToEnd, but it drives the go_eval verb
// through a real spawned capsule to prove the sealed-CoSuper Go authoring path
// end to end (the Definition's focused-product-path activation evidence).
func TestCapsuleGoEvalEndToEnd(t *testing.T) {
	if os.Getenv("CHOIR_CAPSULE_INTEGRATION") != "1" {
		t.Skip("set CHOIR_CAPSULE_INTEGRATION=1 on the designated Linux harness")
	}
	if os.Geteuid() != 0 {
		t.Fatal("capsule integration requires root")
	}
	brokerPath := filepath.Clean(os.Getenv("CHOIR_CAPSULE_BROKER"))
	if info, err := os.Stat(brokerPath); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("immutable capsule broker unavailable: %s", brokerPath)
	}

	sourceDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(sourceDir, "frontend"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "frontend", "index.html"), []byte("<html>computer</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-q"}, {"config", "user.email", "capsule-go-eval@choir.invalid"},
		{"config", "user.name", "Capsule Go Eval"}, {"add", "frontend/index.html"},
		{"commit", "-q", "-m", "go-eval integration source"},
	} {
		command := exec.Command("git", args...)
		command.Dir = sourceDir
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("freeze integration source: %v: %s", err, output)
		}
	}

	lowerDir := t.TempDir()
	for _, path := range []string{"dev/pts", "proc"} {
		if err := os.MkdirAll(filepath.Join(lowerDir, path), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	stateDir, err := os.MkdirTemp("/tmp", "choir-capsule-go-eval-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(stateDir) })

	executor := NewExecutorWithSource(stateDir, lowerDir, sourceDir, brokerPath, 512<<20)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	capsuleID := "g1-goeval-" + strconv.Itoa(os.Getpid())
	caps, err := executor.Spawn(ctx, SpawnSpec{
		CapsuleID: capsuleID, OwnerRunID: "g1-goeval-integration",
		MemoryMax: 256 << 20, CpuQuota: 50000, CpuPeriod: 100000, PidsMax: 128,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = executor.ForceDestroy(context.Background(), capsuleID) })
	if caps.State != StateActive || caps.listener == nil || caps.broker == nil {
		t.Fatalf("capsule broker did not become active: %+v", caps)
	}

	capability, err := executor.MintCapability("g1-goeval-run", RoleCoSuper, capsuleID, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	socketPath := filepath.Join(caps.MergedDir, "run", "capsule", "broker.sock")
	client := NewBrokerClient(socketPath, executor.publicKey)
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("connect broker: %v", err)
	}
	defer client.Close()

	// Evaluate Go that produces deterministic output; assert it is actually
	// interpreted in-capsule (single-broker authoring path), not short-circuited.
	res, err := client.GoEval(ctx, capability, GoEvalRequest{
		Source: `package main; import "fmt"; func main(){ fmt.Print("go-eval-ok") }`,
	})
	if err != nil {
		t.Fatalf("go_eval through real capsule failed: %v", err)
	}
	if !strings.Contains(res.Stdout, "go-eval-ok") {
		t.Fatalf("go_eval did not evaluate in-capsule; stdout=%q stderr=%q err=%q", res.Stdout, res.Stderr, res.Error)
	}

	// Refuse a banned package (server-side allowlist) inside the same capsule.
	_, err = client.GoEval(ctx, capability, GoEvalRequest{
		Source: `package main; import "os/exec"; func main(){}`,
	})
	if err == nil {
		t.Fatal("go_eval accepted os/exec import through the real capsule; want refusal")
	}
	if !strings.Contains(err.Error(), "refused") && !strings.Contains(err.Error(), "os/exec") {
		t.Fatalf("go_eval returned a non-refusal error for the banned import: %v", err)
	}
}
