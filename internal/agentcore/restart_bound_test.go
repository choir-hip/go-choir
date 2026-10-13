package agentcore

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// Restart L1a-2 (docs/restart-subsystem-design-2026-10-13.md §9, findings 5-7).
// Ways the bound can fail, written before the code:
//   - an apply that crashes the guest re-runs on every boot, because the guest
//     writes its own planned marker before each attempt and the next boot
//     reads planned and re-applies;
//   - the attempt count lives in memory, so every boot starts it again;
//   - a refused attempt still writes a marker, so the refusal boot reads as
//     planned;
//   - one update's attempts spend another update's allowance;
//   - after a crash, a pre-boot work wake is re-driven under a fresh
//     occurrence whose CreatedAt is after boot, slipping past the handler's
//     crash gate;
//   - a planned boot loses the wakes it is allowed to resume.

func TestGuestApplyAttemptsAreBoundedAcrossBoots(t *testing.T) {
	store := filepath.Join(t.TempDir(), "state")
	boot := func() *Runtime {
		rt := &Runtime{}
		rt.cfg.StorePath = store
		return rt
	}
	for i := 1; i <= guestApplyAttemptLimit; i++ {
		done, err := boot().markPlannedRestart("platform_update", "u-1")
		if err != nil {
			t.Fatalf("attempt %d refused: %v", i, err)
		}
		done(nil)
	}
	if err := ClearPlannedRestartMarker(store); err != nil {
		t.Fatal(err)
	}
	if _, err := boot().markPlannedRestart("platform_update", "u-1"); !errors.Is(err, ErrGuestApplyAttemptsSpent) {
		t.Fatalf("attempt %d err = %v; want ErrGuestApplyAttemptsSpent", guestApplyAttemptLimit+1, err)
	}
	if _, err := os.Stat(PlannedRestartMarkerPath(store)); !os.IsNotExist(err) {
		t.Fatalf("a refused attempt wrote a planned marker: %v", err)
	}
	if done, err := boot().markPlannedRestart("platform_update", "u-2"); err != nil {
		t.Fatalf("another update was refused: %v", err)
	} else {
		done(nil)
	}
	if done, err := boot().markPlannedRestart(PlannedRestartSelfDevelopmentApply, "u-1"); err != nil {
		t.Fatalf("another reason with the same target was refused: %v", err)
	} else {
		done(nil)
	}
}

func TestCrashBootInterruptsPreBootWorkWakes(t *testing.T) {
	for _, planned := range []bool{false, true} {
		rt, s := testRuntime(t)
		ctx := context.Background()
		suffix := "crash"
		if planned {
			suffix = "planned"
		}
		target := queueOneActorWake(t, s, "boot-"+suffix)
		wakes, err := s.ListUnprojectedActorWakes(ctx)
		if err != nil || len(wakes) == 0 {
			t.Fatalf("queued wake missing: %v", err)
		}
		if !RestartWorkKind(wakes[0].Kind) {
			t.Fatalf("fixture wake kind %q is not work", wakes[0].Kind)
		}
		rt.bootedAt = time.Now().UTC().Add(time.Second)
		rt.bootRestartPlanned = planned
		var calls atomic.Int32
		rt.SetDispatchActor(func(_ context.Context, _, _, to, _, _, _, _ string) error {
			if to == target {
				calls.Add(1)
			}
			return nil
		})
		rt.sweepActorWakeOutbox(ctx)
		left, err := s.ListUnprojectedActorWakes(ctx)
		if err != nil {
			t.Fatal(err)
		}
		for _, wake := range left {
			if wake.TargetAgentID == target {
				t.Fatalf("planned=%v: pre-boot wake still in the drain", planned)
			}
		}
		if planned && calls.Load() != 1 {
			t.Fatalf("planned boot dispatched %d pre-boot wakes, want 1", calls.Load())
		}
		if !planned && calls.Load() != 0 {
			t.Fatalf("crash boot re-drove a pre-boot work wake (%d dispatches)", calls.Load())
		}
	}
}
