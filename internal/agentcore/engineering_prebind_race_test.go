package agentcore

import (
	"context"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Minesweeper demo rerun 2026-10-10: every assignment management cast was
// cancelled in the second it opened — "restart acknowledged absent pre-bind
// assignment capsule" — on a computer that never restarted. Opening an
// assignment mints its work item's lifecycle_work_assigned wake and its
// deferred delegated_assignment_spawn_deadline wake in one commit; the work
// wake runs the restart reconciler, which reaped the open, still-unbound
// assignment before the spawn wake could spawn its capsule. Failure modes
// pinned: the work wake cancels an assignment this boot opened and still owes
// a spawn; a pre-bind assignment opened before this boot (a real restart
// strand) is no longer closed.
func TestWorkWakeLeavesThisBootsPreBindAssignmentToItsSpawn(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	rt.assignmentRuntime = absentAssignmentCapsule{}
	seed, err := store.SeedEngineeringAssignmentAuthority(s, "owner-prebind", rt.TextureComputerID(), 1)
	if err != nil {
		t.Fatal(err)
	}
	open := store.SeedEngineeringOpenRequest(seed, 0, "opaque-prebind")
	assignmentID := open.AssignmentID
	if _, err := s.OpenEngineeringAssignment(ctx, open); err != nil {
		t.Fatal(err)
	}
	// A records assignment has no work item (C1e D1): the reconcile the
	// trajectory's wakes run is the one that must leave this boot's
	// pre-bind window to its spawn.
	wake := func() types.EngineeringAssignment {
		t.Helper()
		if err := rt.ReconcileEngineeringAssignmentsForTrajectory(ctx, open.Binding.OwnerID, open.Binding.ComputerID, open.Binding.TrajectoryID); err != nil {
			t.Fatalf("reconcile: %v", err)
		}
		assignment, err := s.GetEngineeringAssignment(ctx, open.Binding.OwnerID, open.Binding.ComputerID, assignmentID, 1)
		if err != nil {
			t.Fatal(err)
		}
		return assignment
	}
	if assignment := wake(); assignment.Disposition.Terminal() {
		t.Fatalf("work wake reaped this boot's pre-bind assignment: %s (%s)", assignment.Disposition, assignment.DispositionReason)
	}
	// A restart: the same row is now a pre-boot strand and must close.
	rt.bootedAt = time.Now().UTC().Add(time.Second)
	if assignment := wake(); !assignment.Disposition.Terminal() {
		t.Fatalf("pre-boot pre-bind assignment survived a restart reconcile: %s", assignment.Disposition)
	}
}
