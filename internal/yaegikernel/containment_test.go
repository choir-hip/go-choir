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

// TestContainmentContextCancellationKillsBlockedProgram proves that cancelling the
// evaluation context terminates a program blocked on a channel (deadlock-like) or
// an unbounded wait — the resource experiment's cancellation case. A blocked
// program must not hang the evaluator past the context deadline.
func TestContainmentContextCancellationKillsBlockedProgram(t *testing.T) {
	evaluator := NewEvaluator(NewDefaultSafeAllowlist(), nil)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	src := `
package main

import (
	"fmt"
)

func main() {
	// Block forever on a channel read (deadlock scenario).
	ch := make(chan int)
	<-ch
	fmt.Println("never reached")
}
`
	start := time.Now()
	_, err := evaluator.Eval(ctx, src)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected blocked program to be cancelled, got nil")
	}
	if !strings.Contains(err.Error(), "timed out") && !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Fatalf("expected cancellation error, got %v", err)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("cancellation took too long: %v", elapsed)
	}
}

// TestContainmentDeadlockTimeout proves a program that blocks on a second
// goroutine reading a never-sent channel is bounded by the context deadline
// (goroutine-leak/deadlock containment).
func TestContainmentDeadlockTimeout(t *testing.T) {
	evaluator := NewEvaluator(NewDefaultSafeAllowlist(), nil)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	src := `
package main

func main() {
	done := make(chan bool)
	go func() { <-done }() // never signals
	<-done
}
`
	start := time.Now()
	_, err := evaluator.Eval(ctx, src)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected deadlock to time out, got nil")
	}
	if !strings.Contains(err.Error(), "timed out") && !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Fatalf("expected timeout error, got %v", err)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("deadlock timeout took too long: %v", elapsed)
	}
}
