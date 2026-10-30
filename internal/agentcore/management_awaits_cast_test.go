package agentcore

import (
	"context"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// docs/problems/engineering-completion-path-cluster-2026-10-20.md:
// management completed its turn seconds after casting, so the engineering
// result was delivered to a dead run and nothing woke management. The fix
// parks the casting turn while its own cast is open, so the result resumes
// it through the park-resume path (a producer report never mints a
// management run; terminal events never select backlog).
//
// Failure modes pinned:
//   - a turn with an open cast it authored completes instead of parking;
//   - a turn parks on a cast another run authored, or on a terminal cast
//     (a leaked passivated run per finished assignment);
//   - a turn that cast nothing changes behavior;
//   - a result that already arrived is not taken in the same turn;
//   - a turn with nothing outstanding stops completing normally.
func TestManagementParksOnlyWhileItsOwnCastIsOpen(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	seed, err := store.SeedEngineeringAssignmentAuthority(s, "owner-awaits", rt.TextureComputerID(), 1)
	if err != nil {
		t.Fatal(err)
	}
	caster := &types.RunRecord{
		RunID: seed.ParentRunID, OwnerID: seed.OwnerID, ComputerID: seed.ComputerID,
		AgentID: persistentManagementAgentID(seed.OwnerID), AgentProfile: "management", AgentRole: "management",
		Metadata: map[string]any{"request_source": "lifecycle_texture_control", "assignment_trajectory_id": seed.TrajectoryID},
	}
	other := *caster
	other.RunID = "run-management-other-turn"

	if authored, open, err := rt.managementRunCasts(ctx, caster); err != nil || authored || open {
		t.Fatalf("before any cast: authored=%v open=%v err=%v", authored, open, err)
	}

	open := store.SeedEngineeringOpenRequest(seed, 0, "opaque-awaits")
	assignmentID := open.AssignmentID
	if _, err := s.OpenEngineeringAssignment(ctx, open); err != nil {
		t.Fatalf("open assignment: %v", err)
	}

	if authored, open, err := rt.managementRunCasts(ctx, caster); err != nil || !authored || !open {
		t.Fatalf("casting turn with an open cast: authored=%v open=%v err=%v", authored, open, err)
	}
	if authored, open, err := rt.managementRunCasts(ctx, &other); err != nil || authored || open {
		t.Fatalf("a turn that did not cast: authored=%v open=%v err=%v", authored, open, err)
	}
	texture := *caster
	texture.AgentID, texture.AgentProfile, texture.AgentRole = "texture:doc", "texture", "texture"
	if authored, open, err := rt.managementRunCasts(ctx, &texture); err != nil || authored || open {
		t.Fatalf("a non-management run: authored=%v open=%v err=%v", authored, open, err)
	}

	current, err := s.GetEngineeringAssignment(ctx, seed.OwnerID, seed.ComputerID, assignmentID, 1)
	if err != nil {
		t.Fatal(err)
	}
	cancel := types.CancelEngineeringAssignmentRequest{
		CommandID: "command-cancel-" + assignmentID, OwnerID: seed.OwnerID, ComputerID: seed.ComputerID,
		AssignmentID: assignmentID, Attempt: 1, ExpectedLifecycleVersion: current.LifecycleVersion, Reason: "test terminal",
	}
	if cancel.CommandDigest, err = store.ComputeCancelEngineeringAssignmentDigest(cancel); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CancelEngineeringAssignment(ctx, cancel); err != nil {
		t.Fatalf("cancel assignment: %v", err)
	}
	if authored, open, err := rt.managementRunCasts(ctx, caster); err != nil || !authored || open {
		t.Fatalf("after the cast is terminal: authored=%v open=%v err=%v", authored, open, err)
	}
}

func TestManagementParkDecision(t *testing.T) {
	cont := toolregistry.ToolLoopParkResult{Continue: true, Reason: "update_coagent_signal"}
	park := toolregistry.ToolLoopParkResult{Passivate: true, Reason: managementAwaitingCastPassivation}
	done := toolregistry.ToolLoopParkResult{}
	cases := []struct {
		ready, authored, open bool
		want                  toolregistry.ToolLoopParkResult
	}{
		// The turn cast and the result already arrived: take it now, or it
		// is delivered to a run that is about to complete.
		{ready: true, authored: true, open: true, want: cont},
		{ready: true, authored: true, open: false, want: cont},
		// The cast is still open: park until its result resumes the turn.
		{ready: false, authored: true, open: true, want: park},
		// The cast finished and its result was taken: complete.
		{ready: false, authored: true, open: false, want: done},
		// A turn that cast nothing keeps today's behavior.
		{ready: true, authored: false, open: false, want: done},
		{ready: false, authored: false, open: false, want: done},
	}
	for _, c := range cases {
		if got := managementParkDecision(c.ready, c.authored, c.open); got != c.want {
			t.Errorf("ready=%v authored=%v open=%v: got %+v, want %+v", c.ready, c.authored, c.open, got, c.want)
		}
	}
}
