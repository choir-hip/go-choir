package store

import (
	"context"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// The settled observer fires once, with the trajectory, on a real settle (the
// runtime closes the trajectory's undecided self-development candidates from
// it), and not while work is still open.
func TestTrajectorySettledObserverFiresOnRealSettle(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	var got []string
	s.SetTrajectorySettledObserver(func(ownerID, computerID, trajectoryID string) {
		got = append(got, ownerID+"/"+computerID+"/"+trajectoryID)
	})
	start := lifecycleStartFixture()
	if _, err := s.StartLifecycle(ctx, start); err != nil {
		t.Fatal(err)
	}
	run := lifecycleRunFixture(start, "run-observer", types.RunPending)
	if err := s.CreateRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	settleInitialLifecycleWork(t, s, start, "command-work-observer")
	if len(got) != 0 {
		t.Fatalf("observer fired before the run ended: %v", got)
	}
	run.State, run.UpdatedAt = types.RunPassivated, time.Now().UTC()
	if err := s.UpdateRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != start.OwnerID+"/"+start.ComputerID+"/"+start.TrajectoryID {
		t.Fatalf("observer calls = %v, want one for the settled trajectory", got)
	}
}
