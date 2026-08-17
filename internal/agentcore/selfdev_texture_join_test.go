package agentcore

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestSelfDevelopmentTextureCallerReactivatesDeterministicRun(t *testing.T) {
	ctx := context.Background()
	runtime, productStore := testRuntime(t)
	ownerID := "owner"
	computerID := "computer-selfdev-caller-reactivate"
	runtime.cfg.ComputerID = computerID
	operation := selfdev.Operation{
		OperationID:       "selfdev-caller-reactivate",
		ComputerID:        computerID,
		PromptArtifactRef: "artifact:sha256:" + strings.Repeat("b", 64),
	}
	if err := runtime.startSelfDevelopmentPersistentSuper(ctx, operation, ownerID, "caller"); err != nil {
		t.Fatal(err)
	}
	docID, _, textureWorkID, trajectoryID, _, _ := selfDevelopmentTextureJoinIDs(ownerID, computerID, operation.OperationID)
	textureAgentID := agentprofile.Texture + ":" + docID
	deterministicRunID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(strings.Join([]string{
		"choir:texture:self-development", ownerID, computerID, trajectoryID, "texture-run",
	}, ":"))).String()

	caller, err := productStore.GetLifecycleRun(ctx, ownerID, computerID, deterministicRunID)
	if err != nil || !caller.State.Active() {
		t.Fatalf("deterministic caller after start: %+v err=%v", caller, err)
	}

	// Simulate boot passivation of the deterministic caller.
	passivated := caller
	passivated.State = types.RunPassivated
	passivated.UpdatedAt = time.Now().UTC()
	passivated.FinishedAt = nil
	passivateReq := types.ReplaceLifecycleActivationRequest{
		OwnerID: ownerID, ComputerID: computerID,
		CommandID:    "lifecycle-passivate-caller-test:" + deterministicRunID,
		TrajectoryID: trajectoryID, AgentID: textureAgentID, Run: passivated,
	}
	passivateReq.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(passivateReq)
	if _, err := productStore.ReplaceLifecycleActivation(ctx, passivateReq); err != nil {
		t.Fatal(err)
	}

	// A successor Texture activation then owns the agent slot.
	successor := types.RunRecord{
		RunID: "run-successor-texture", AgentID: textureAgentID, OwnerID: ownerID, ComputerID: computerID,
		AgentProfile: agentprofile.Texture, AgentRole: agentprofile.Texture, ChannelID: docID,
		TrajectoryID: trajectoryID, State: types.RunRunning,
		Prompt: "Supervise self-development on this computer.",
		Metadata: map[string]any{
			"lifecycle_work_item_id": textureWorkID,
			"work_item_ids":          []string{textureWorkID},
			runMetadataAgentProfile:  agentprofile.Texture,
			runMetadataAgentRole:     agentprofile.Texture,
		},
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	successorReq := types.ReplaceLifecycleActivationRequest{
		OwnerID: ownerID, ComputerID: computerID,
		CommandID: "project-successor-test", TrajectoryID: trajectoryID, AgentID: textureAgentID, Run: successor,
	}
	successorReq.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(successorReq)
	if _, err := productStore.ReplaceLifecycleActivation(ctx, successorReq); err != nil {
		t.Fatal(err)
	}

	// The caller must be reactivated (deterministic), not the successor.
	got, err := runtime.ensureSelfDevelopmentTextureCaller(ctx, ownerID, computerID, trajectoryID, textureAgentID, textureWorkID, docID)
	if err != nil {
		t.Fatal(err)
	}
	if got.RunID != deterministicRunID || !got.State.Active() {
		t.Fatalf("caller not reactivated: %+v want %s", got, deterministicRunID)
	}
	agent, err := productStore.GetAgentByScope(ctx, ownerID, computerID, textureAgentID)
	if err != nil || agent.ActiveRunID != deterministicRunID {
		t.Fatalf("agent active run after reactivation: %+v err=%v", agent, err)
	}
	successorStored, err := productStore.GetLifecycleRun(ctx, ownerID, computerID, "run-successor-texture")
	if err != nil || successorStored.State != types.RunPassivated {
		t.Fatalf("successor not released: %+v err=%v", successorStored, err)
	}
}
