package agentcore

import (
	"context"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func seedReclaimAssignmentFrom(t *testing.T, s *store.Store, fixture store.EngineeringAssignmentSeed, ownerID, computerID, assignmentID, capsuleID string) types.EngineeringAssignment {
	t.Helper()
	ctx := context.Background()
	capability := "opaque-reclaim-" + assignmentID
	binding := types.EngineeringAssignmentBinding{
		OwnerID: fixture.OwnerID, ComputerID: fixture.ComputerID, TrajectoryID: fixture.TrajectoryID,
		ParentAgentID: fixture.ParentAgentID, ParentRunID: fixture.ParentRunID,
		ParentDecisionID: fixture.ParentDecisionID, ParentControlID: fixture.ParentControlID,
		ParentWorkItemID: fixture.ParentWorkID, AssignedWorkItemID: fixture.AssignedWorkIDs[len(assignmentID)%len(fixture.AssignedWorkIDs)],
		AssignedAgentID: fixture.AssignedAgentIDs[len(assignmentID)%len(fixture.AssignedAgentIDs)], Kind: types.EngineeringAssignmentImplementation, Attempt: 1,
		ScopeDigest:           objectgraph.SHA256([]byte("scope:" + assignmentID)),
		RequestDigest:         objectgraph.SHA256([]byte("request:" + assignmentID)),
		CapabilityDigest:      store.DigestEngineeringOpaqueCapability(capability),
		ExecutionHandleDigest: objectgraph.SHA256([]byte(capability)),
		SubjectDigest:         objectgraph.SHA256([]byte("subject:" + assignmentID)),
		SourceArtifactRef:     "capsule-source-git:commit:" + objectgraph.SHA256([]byte("subject:"+assignmentID)),
		Writable:              true, CapsuleID: capsuleID,
		NetworkMode:    types.EngineeringCapsuleNetworkForbidden,
		FilesystemMode: types.EngineeringCapsuleFilesystemAssignmentLocalWritableOverlay,
	}
	open := types.OpenEngineeringAssignmentRequest{
		CommandID: "open-" + assignmentID, AssignmentID: assignmentID, Binding: binding,
		AssignedAgent: types.AgentRecord{AgentID: binding.AssignedAgentID},
		AssignedWork:  types.WorkItemRecord{WorkItemID: binding.AssignedWorkItemID, AssignedAgentID: binding.AssignedAgentID, Objective: "test"},
	}
	open.CommandDigest, _ = store.ComputeOpenEngineeringAssignmentDigest(open)
	opened, err := s.OpenEngineeringAssignment(ctx, open)
	if err != nil {
		t.Fatal(err)
	}
	return opened.Assignment
}

func TestReclaimSkipsCurrentAndAlreadyRevoked(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	ownerID, computerID := "owner-reclaim-skip", rt.TextureComputerID()

	fixture, err := store.SeedEngineeringAssignmentAuthority(s, ownerID, computerID, 2)
	if err != nil {
		t.Fatal(err)
	}
	parent := types.RunRecord{
		RunID: fixture.ParentRunID, OwnerID: ownerID, ComputerID: computerID,
		AgentID: fixture.ParentAgentID, AgentProfile: agentprofile.Management, AgentRole: agentprofile.Management,
		State: types.RunRunning, Metadata: map[string]any{},
	}

	current := seedReclaimAssignmentFrom(t, s, fixture, ownerID, computerID, "assignment-current", "capsule-current")
	stale := seedReclaimAssignmentFrom(t, s, fixture, ownerID, computerID, "assignment-stale2", "capsule-stale2")

	// Reclaim targeting the current assignment ID must not touch it. Both
	// assignments are unbound (never spawned), so reclaim skips them entirely.
	if err := rt.reclaimSupersededAssignmentCapsules(ctx, parent, current.AssignmentID); err != nil {
		t.Fatalf("reclaim: %v", err)
	}

	currentAfter, err := s.GetEngineeringAssignment(ctx, ownerID, computerID, current.AssignmentID, current.Binding.Attempt)
	if err != nil {
		t.Fatal(err)
	}
	if currentAfter.Disposition.Terminal() {
		t.Fatal("current assignment must not be reclaimed")
	}

	staleAfter, err := s.GetEngineeringAssignment(ctx, ownerID, computerID, stale.AssignmentID, stale.Binding.Attempt)
	if err != nil {
		t.Fatal(err)
	}
	if staleAfter.Disposition.Terminal() {
		t.Fatalf("unbound stale assignment should not be terminal, got %s", staleAfter.Disposition)
	}
}
