package textureowner

import (
	"context"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// TestSubmitTextureAgentRevisionRunRacedActivation proves the lifecycle_work_assigned
// wake race is handled: when reconcileAgentWakeLocked's submit wins the interlock and
// commits agent.ActiveRunID before EnsureTextureHandoff's own submit runs, the loser
// must return the already-committed run instead of ErrLifecycleInvalidTransition (500).
//
// Without the fix the second submit fails at projectLifecycleRun's
// previous-active-run gate: lifecycleRunOwnsActivation(previousRun.State)=true
// because run-A is still pending → ErrLifecycleInvalidTransition → management-open 500.
func TestSubmitTextureAgentRevisionRunRacedActivation(t *testing.T) {
	core, handler := testAPISetup(t)
	_ = core // runtime needed for lifecycle wiring; store assertions go through handler.Store
	ctx := context.Background()
	ownerID, computerID := "user-alice", "autoputer-test"
	docID := "doc-race-1"
	agentID := currentTextureAgentID(docID)
	trajID := "traj-race-1"
	workID := "work-race-1"

	start := types.StartLifecycleRequest{
		OwnerID: ownerID, ComputerID: computerID,
		CommandID: "cmd-race-1", TrajectoryID: trajID,
		Kind:        types.TrajectoryKindTask,
		SubjectRefs: map[string]string{"artifact": "texture://documents/" + docID},
		SettlementRule: types.SettlementRule{
			Version:               types.LifecycleReducerVersion,
			RequireNoOpenWorkItems: true,
			RequiredSubjectRefs:   []string{"artifact"},
		},
		InitialWork: types.WorkItemRecord{
			WorkItemID: workID, Objective: "initial objective", AssignedAgentID: agentID,
			AuthorityProfile: agentprofile.Texture,
		},
		InitialDocument: types.Document{
			DocID: docID, OwnerID: ownerID, ComputerID: computerID,
			TrajectoryID: trajID, Title: "Race Doc",
		},
		InitialRevision: types.Revision{
			RevisionID: "rev-race-v0", AuthorKind: types.AuthorUser,
			AuthorLabel: ownerID, Content: "initial content",
		},
		Agent: types.AgentRecord{
			AgentID: agentID, OwnerID: ownerID, ComputerID: computerID,
			Profile: agentprofile.Texture, Role: agentprofile.Texture,
			ChannelID: docID,
		},
	}
	start.StartRequestDigest, _ = store.ComputeStartLifecycleRequestDigest(start)
	result, err := handler.Store.StartLifecycle(ctx, start)
	if err != nil {
		t.Fatalf("StartLifecycle: %v", err)
	}
	doc := *result.Document

	// Simulate the wake handler winning: commit run-A as the activation.
	winnerRunID := "run-winner-1"
	winnerRun := types.RunRecord{
		RunID: winnerRunID, AgentID: agentID, ChannelID: docID,
		TrajectoryID: trajID, AgentProfile: agentprofile.Texture,
		AgentRole: agentprofile.Texture, OwnerID: ownerID, ComputerID: computerID,
		State: types.RunPending, Prompt: "wake handler activation",
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
		Metadata: map[string]any{"lifecycle_work_item_id": workID},
	}
	actReq := types.ReplaceLifecycleActivationRequest{
		OwnerID: ownerID, ComputerID: computerID,
		CommandID: "activation:" + winnerRunID,
		TrajectoryID: trajID, AgentID: agentID,
		Run: winnerRun,
	}
	actReq.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(actReq)
	if _, err := handler.Store.ReplaceLifecycleActivation(ctx, actReq); err != nil {
		t.Fatalf("winner activation: %v", err)
	}

	// EnsureTextureHandoff path: submitTextureAgentRevisionRun must return run-A,
	// not fail with ErrLifecycleInvalidTransition.
	loserRun, err := handler.submitTextureAgentRevisionRun(ctx, doc, ownerID, textureAgentRevisionRequest{
		Intent: "initial_conductor_workflow",
		Prompt: "initial prompt",
	}, 0)
	if err != nil {
		t.Fatalf("loser submitTextureAgentRevisionRun returned error: %v", err)
	}
	if loserRun == nil || loserRun.RunID != winnerRunID {
		t.Fatalf("loser returned RunID=%q, want %q", loserRun.RunID, winnerRunID)
	}
}
