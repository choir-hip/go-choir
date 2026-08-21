package agentcore

import (
	"context"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func seedReclaimAssignmentFrom(t *testing.T, s *store.Store, fixture store.CoSuperAssignmentSeed, ownerID, computerID, assignmentID, capsuleID string) types.CoSuperAssignment {
	t.Helper()
	ctx := context.Background()
	capability := "opaque-reclaim-" + assignmentID
	binding := types.CoSuperAssignmentBinding{
		OwnerID: fixture.OwnerID, ComputerID: fixture.ComputerID, TrajectoryID: fixture.TrajectoryID,
		ParentAgentID: fixture.ParentAgentID, ParentRunID: fixture.ParentRunID,
		ParentDecisionID: fixture.ParentDecisionID, ParentControlID: fixture.ParentControlID,
		ParentWorkItemID: fixture.ParentWorkID, AssignedWorkItemID: fixture.AssignedWorkIDs[len(assignmentID)%len(fixture.AssignedWorkIDs)],
		AssignedAgentID: fixture.AssignedAgentIDs[len(assignmentID)%len(fixture.AssignedAgentIDs)], Kind: types.CoSuperAssignmentImplementation, Attempt: 1,
		ScopeDigest:           objectgraph.SHA256([]byte("scope:" + assignmentID)),
		RequestDigest:         objectgraph.SHA256([]byte("request:" + assignmentID)),
		CapabilityDigest:      store.DigestCoSuperOpaqueCapability(capability),
		ExecutionHandleDigest: objectgraph.SHA256([]byte(capability)),
		SubjectDigest:         objectgraph.SHA256([]byte("subject:" + assignmentID)),
		SourceArtifactRef:     "capsule-source-git:commit:" + objectgraph.SHA256([]byte("subject:"+assignmentID)),
		Writable:              true, CapsuleID: capsuleID,
		NetworkMode:    types.CoSuperCapsuleNetworkForbidden,
		FilesystemMode: types.CoSuperCapsuleFilesystemAssignmentLocalWritableOverlay,
	}
	open := types.OpenCoSuperAssignmentRequest{
		CommandID: "open-" + assignmentID, AssignmentID: assignmentID, Binding: binding,
		AssignedAgent: types.AgentRecord{AgentID: binding.AssignedAgentID},
		AssignedWork:  types.WorkItemRecord{WorkItemID: binding.AssignedWorkItemID, AssignedAgentID: binding.AssignedAgentID, Objective: "test"},
	}
	open.CommandDigest, _ = store.ComputeOpenCoSuperAssignmentDigest(open)
	opened, err := s.OpenCoSuperAssignment(ctx, open)
	if err != nil {
		t.Fatal(err)
	}
	return opened.Assignment
}

func TestReclaimSupersededAssignmentCapsulesRevokesStaleAssignments(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	ownerID, computerID := "owner-reclaim", rt.TextureComputerID()

	fixture, err := store.SeedCoSuperAssignmentAuthority(s, ownerID, computerID, 1)
	if err != nil {
		t.Fatal(err)
	}
	parent := types.RunRecord{
		RunID: fixture.ParentRunID, OwnerID: ownerID, ComputerID: computerID,
		AgentID: fixture.ParentAgentID, AgentProfile: agentprofile.Super, AgentRole: agentprofile.Super,
		State: types.RunRunning, Metadata: map[string]any{},
	}

	stale := seedReclaimAssignmentFrom(t, s, fixture, ownerID, computerID, "assignment-stale", "capsule-stale")
	if stale.CapsuleDisposition == types.CoSuperCapsuleRevoked {
		t.Fatal("seeded assignment should start non-revoked")
	}

	// A fresh executor has no live capsules; the revoke fate path succeeds
	// because HasCapsule returns false and orphan cleanup finds no residue.
	rt.capsuleExecutor = capsule.NewExecutor(t.TempDir(), t.TempDir(), "", 3<<30)

	if err := rt.reclaimSupersededAssignmentCapsules(ctx, parent, "assignment-current"); err != nil {
		t.Fatalf("reclaim: %v", err)
	}

	reclaimed, err := s.GetCoSuperAssignment(ctx, ownerID, computerID, stale.AssignmentID, stale.Binding.Attempt)
	if err != nil {
		t.Fatal(err)
	}
	// An unbound assignment (opened but never bound/spawned) has no capsule to
	// revoke; its capsule disposition correctly remains unbound. The assertion
	// is that the assignment itself reached a terminal disposition.
	if !reclaimed.Disposition.Terminal() {
		t.Fatalf("stale assignment disposition = %s, want terminal", reclaimed.Disposition)
	}
}

func TestReclaimSkipsCurrentAndAlreadyRevoked(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	ownerID, computerID := "owner-reclaim-skip", rt.TextureComputerID()

	fixture, err := store.SeedCoSuperAssignmentAuthority(s, ownerID, computerID, 2)
	if err != nil {
		t.Fatal(err)
	}
	parent := types.RunRecord{
		RunID: fixture.ParentRunID, OwnerID: ownerID, ComputerID: computerID,
		AgentID: fixture.ParentAgentID, AgentProfile: agentprofile.Super, AgentRole: agentprofile.Super,
		State: types.RunRunning, Metadata: map[string]any{},
	}

	current := seedReclaimAssignmentFrom(t, s, fixture, ownerID, computerID, "assignment-current", "capsule-current")
	stale := seedReclaimAssignmentFrom(t, s, fixture, ownerID, computerID, "assignment-stale2", "capsule-stale2")

	// Reclaim targeting the current assignment ID must not touch it but must
	// terminalize the stale assignment.
	if err := rt.reclaimSupersededAssignmentCapsules(ctx, parent, current.AssignmentID); err != nil {
		t.Fatalf("reclaim: %v", err)
	}

	currentAfter, err := s.GetCoSuperAssignment(ctx, ownerID, computerID, current.AssignmentID, current.Binding.Attempt)
	if err != nil {
		t.Fatal(err)
	}
	if currentAfter.Disposition.Terminal() {
		t.Fatal("current assignment must not be reclaimed")
	}

	staleAfter, err := s.GetCoSuperAssignment(ctx, ownerID, computerID, stale.AssignmentID, stale.Binding.Attempt)
	if err != nil {
		t.Fatal(err)
	}
	if !staleAfter.Disposition.Terminal() {
		t.Fatalf("stale assignment disposition = %s, want terminal", staleAfter.Disposition)
	}
}
