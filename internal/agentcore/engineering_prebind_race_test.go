package agentcore

import (
	"context"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
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
	assignmentID := "assignment-prebind"
	capability := "opaque-prebind"
	open := types.OpenEngineeringAssignmentRequest{
		CommandID: "command-open-" + assignmentID + "-1", AssignmentID: assignmentID,
		Binding: types.EngineeringAssignmentBinding{
			OwnerID: seed.OwnerID, ComputerID: seed.ComputerID, TrajectoryID: seed.TrajectoryID,
			ParentAgentID: seed.ParentAgentID, ParentRunID: seed.ParentRunID,
			ParentDecisionID: seed.ParentDecisionID, ParentControlID: seed.ParentControlID,
			ParentWorkItemID: seed.ParentWorkID, AssignedWorkItemID: seed.AssignedWorkIDs[0], AssignedAgentID: seed.AssignedAgentIDs[0],
			Kind: types.EngineeringAssignmentImplementation, Attempt: 1,
			ScopeDigest: objectgraph.SHA256([]byte("scope:" + assignmentID)), RequestDigest: objectgraph.SHA256([]byte("request:" + assignmentID)),
			CapabilityDigest: store.DigestEngineeringOpaqueCapability(capability), ExecutionHandleDigest: objectgraph.SHA256([]byte(capability)),
			SubjectDigest:     objectgraph.SHA256([]byte("subject:" + assignmentID)),
			SourceArtifactRef: "capsule-source-git:commit:" + objectgraph.SHA256([]byte("subject:"+assignmentID)),
			Writable:          true, CapsuleID: "capsule-prebind",
			NetworkMode:    types.EngineeringCapsuleNetworkForbidden,
			FilesystemMode: types.EngineeringCapsuleFilesystemAssignmentLocalWritableOverlay,
		},
		AssignedAgent: types.AgentRecord{AgentID: seed.AssignedAgentIDs[0]},
		AssignedWork:  types.WorkItemRecord{WorkItemID: seed.AssignedWorkIDs[0], AssignedAgentID: seed.AssignedAgentIDs[0], Objective: "build the game"},
	}
	open.CommandDigest, err = store.ComputeOpenEngineeringAssignmentDigest(open)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.OpenEngineeringAssignment(ctx, open); err != nil {
		t.Fatal(err)
	}
	wake := func() types.EngineeringAssignment {
		t.Helper()
		if err := rt.ReconcileLifecycleWorkAssignment(ctx, open.Binding.OwnerID, open.Binding.ComputerID, open.Binding.AssignedAgentID, open.Binding.TrajectoryID, open.Binding.AssignedWorkItemID); err != nil {
			t.Fatalf("work wake: %v", err)
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
