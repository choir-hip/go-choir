package agentcore

import (
	"context"
	"fmt"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func seedReclaimAssignmentFrom(t *testing.T, s *store.Store, fixture store.EngineeringAssignmentSeed, index int) types.EngineeringAssignment {
	t.Helper()
	open := store.SeedEngineeringOpenRequest(fixture, index, fmt.Sprintf("opaque-reclaim-%d", index))
	opened, err := s.OpenEngineeringAssignment(context.Background(), open)
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

	current := seedReclaimAssignmentFrom(t, s, fixture, 0)
	stale := seedReclaimAssignmentFrom(t, s, fixture, 1)

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
