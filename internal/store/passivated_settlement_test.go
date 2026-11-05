package store

import (
	"context"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// The work-bound run whose turn settled the last work item passivates
// (durable_transition_success); its passivation settles the trajectory as a
// terminal end would (d@e trajectory f62bea6c stayed live forever).
func TestPassivatedRunAfterLastWorkSettlesTrajectory(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	start := lifecycleStartFixture()
	if _, err := s.StartLifecycle(ctx, start); err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	run := lifecycleRunFixture(start, "run-passivated-settles", types.RunPending)
	if err := s.CreateRun(ctx, run); err != nil {
		t.Fatalf("create run: %v", err)
	}
	settleInitialLifecycleWork(t, s, start, "command-work-before-passivation")
	run.State = types.RunPassivated
	run.UpdatedAt = time.Now().UTC()
	if err := s.UpdateRun(ctx, run); err != nil {
		t.Fatalf("passivate run: %v", err)
	}
	snapshot, err := s.GetLifecycleSnapshot(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil || snapshot.Trajectory.Status != types.TrajectorySettled ||
		snapshot.Trajectory.TerminalArtifactHeadRef != start.InitialRevision.RevisionID {
		t.Fatalf("passivated final run did not settle: %+v, %v", snapshot.Trajectory, err)
	}
}

// A run that passivates while work is still open waits; nothing settles.
func TestPassivatedRunWithOpenWorkDoesNotSettle(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	start := lifecycleStartFixture()
	if _, err := s.StartLifecycle(ctx, start); err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	run := lifecycleRunFixture(start, "run-passivated-open", types.RunPending)
	if err := s.CreateRun(ctx, run); err != nil {
		t.Fatalf("create run: %v", err)
	}
	run.State = types.RunPassivated
	run.UpdatedAt = time.Now().UTC()
	if err := s.UpdateRun(ctx, run); err != nil {
		t.Fatalf("passivate run: %v", err)
	}
	snapshot, err := s.GetLifecycleSnapshot(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil || snapshot.Trajectory.Status != types.TrajectoryLive {
		t.Fatalf("a passivation with open work settled: %+v, %v", snapshot.Trajectory, err)
	}
}
