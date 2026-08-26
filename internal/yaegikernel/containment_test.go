package yaegikernel

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestContainmentInfiniteLoopTimeout(t *testing.T) {
	evaluator := NewEvaluator(NewDefaultSafeAllowlist(), nil)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	src := `
package main

func main() {
	for {
		// Infinite loop
	}
}
`
	start := time.Now()
	_, err := evaluator.Eval(ctx, src)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected infinite loop to time out and return error")
	}
	if !strings.Contains(err.Error(), "timed out") && !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Fatalf("expected timeout error, got %v", err)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("timeout took too long: %v", elapsed)
	}
}

func TestContainmentPanicRecovery(t *testing.T) {
	evaluator := NewEvaluator(NewDefaultSafeAllowlist(), nil)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	src := `
package main

func main() {
	panic("intentional model panic")
}
`
	_, err := evaluator.Eval(ctx, src)
	if err == nil {
		t.Fatal("expected panic to return error")
	}
	if !strings.Contains(err.Error(), "panic") || !strings.Contains(err.Error(), "intentional model panic") {
		t.Fatalf("expected panic message in error, got %v", err)
	}
}

func TestSidecarRunnerInProcess(t *testing.T) {
	cfg := SidecarConfig{
		Timeout:         2 * time.Second,
		AllowedPackages: []string{"fmt", "strings"},
	}
	runner := NewSidecarRunner(cfg)

	src := `
package main

import "fmt"
import "strings"

func main() {
	fmt.Print(strings.ToLower("YAEGI-RUNNER-OK"))
}
`
	res, err := runner.RunInProcess(context.Background(), src)
	if err != nil {
		t.Fatalf("RunInProcess failed: %v", err)
	}
	if strings.TrimSpace(res.Stdout) != "yaegi-runner-ok" {
		t.Fatalf("unexpected stdout: %q", res.Stdout)
	}
}
