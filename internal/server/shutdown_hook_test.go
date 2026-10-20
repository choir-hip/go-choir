package server

import (
	"context"
	"testing"
	"time"
)

// A shutdown hook runs after the HTTP server stops accepting and before
// Shutdown returns, with a deadline; a hook that never returns cannot hold the
// shutdown past it. Failure modes: the hook is skipped on SIGTERM (the open
// telemetry window is lost on a graceful stop); a stuck hook hangs shutdown.
func TestShutdownRunsHooksWithADeadline(t *testing.T) {
	s := NewServer("test-hook-service", "0")
	s.shutdownTimeout = 300 * time.Millisecond
	ran := make(chan time.Duration, 1)
	s.AddShutdownHook(func(ctx context.Context) {
		deadline, ok := ctx.Deadline()
		if !ok {
			ran <- -1
			return
		}
		ran <- time.Until(deadline)
	})
	s.AddShutdownHook(func(ctx context.Context) { <-ctx.Done() }) // a stuck hook
	started := time.Now()
	s.Shutdown()
	if time.Since(started) > 5*time.Second {
		t.Fatalf("Shutdown took %s with a stuck hook", time.Since(started))
	}
	select {
	case remaining := <-ran:
		if remaining <= 0 {
			t.Fatalf("hook ran without a deadline or after it (remaining %s)", remaining)
		}
	default:
		t.Fatal("shutdown hook did not run")
	}
}
