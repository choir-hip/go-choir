package actorruntime

import (
	"context"
	"errors"
	"testing"
	"time"
)

// Failure modes (docs/problems/layered-release-spa-underivable-2026-10-09.md,
// "Residual 1 measured"): a deterministic refusal is retried ten times and
// delays every boot by ~17 s; a transient error that clears is given up on
// too early; a cancelled boot keeps retrying.

func TestSurfaceBootstrapStopsOnRepeatedVerdict(t *testing.T) {
	calls := 0
	err := ensureSurfaceWithRetry(context.Background(), 10, time.Millisecond, func(context.Context) error {
		calls++
		return errors.New("self-development checkpoint: served SPA is underivable")
	})
	if err == nil {
		t.Fatal("deterministic refusal reported success")
	}
	if calls != 2 {
		t.Fatalf("attempts = %d, want 2 (stop when the verdict repeats)", calls)
	}
}

func TestSurfaceBootstrapRetriesChangingErrorsUntilSuccess(t *testing.T) {
	calls := 0
	err := ensureSurfaceWithRetry(context.Background(), 10, time.Millisecond, func(context.Context) error {
		calls++
		switch calls {
		case 1:
			return errors.New("updater: current pointer missing")
		case 2:
			return errors.New("updater: release manifest incomplete")
		default:
			return nil
		}
	})
	if err != nil || calls != 3 {
		t.Fatalf("err=%v calls=%d, want success on the third attempt", err, calls)
	}
}

func TestSurfaceBootstrapStopsWhenBootIsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	err := ensureSurfaceWithRetry(ctx, 10, time.Hour, func(context.Context) error {
		calls++
		cancel()
		return errors.New("transient")
	})
	if !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatalf("err=%v calls=%d, want context.Canceled after one attempt", err, calls)
	}
}
