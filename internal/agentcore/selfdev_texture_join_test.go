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
	"github.com/yusefmosiah/go-choir/internal/sourcecontract"
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


func TestPersistentSuperReconcileMintsTextureRewakeAfterTerminalSelfDevelopmentSuper(t *testing.T) {
	ctx := context.Background()
	runtime, productStore := testRuntime(t)
	ownerID := "owner-selfdev-rewake"
	computerID := "computer-selfdev-rewake"
	runtime.cfg.ComputerID = computerID
	operation := selfdev.Operation{
		OperationID:       "selfdev-op-rewake-test",
		ComputerID:        computerID,
		PromptArtifactRef: "artifact:sha256:" + strings.Repeat("c", 64),
	}
	originalPrompt := "Author classic solitaire game engine"
	if err := runtime.startSelfDevelopmentPersistentSuper(ctx, operation, ownerID, originalPrompt); err != nil {
		t.Fatal(err)
	}

	superAgentID := persistentSuperAgentID(ownerID)
	firstSuper, err := productStore.GetLatestRunByAgent(ctx, ownerID, superAgentID)
	if err != nil || !firstSuper.State.Active() {
		t.Fatalf("first Super active state: %+v err=%v", firstSuper, err)
	}
	if metadataStringValue(firstSuper.Metadata, "self_development_operation_id") != operation.OperationID {
		t.Fatalf("first Super missing operation_id: %+v", firstSuper.Metadata)
	}

	// Simulate first Super failure (e.g. 200 iterations or CoSuper cancel).
	_ = runtime.CancelRun(ctx, firstSuper.RunID, ownerID)
	finished := time.Now().UTC()
	firstSuper.State = types.RunFailed
	firstSuper.Error = "tool loop: exceeded 200 iterations without end_turn"
	firstSuper.UpdatedAt = finished
	firstSuper.FinishedAt = &finished
	if err := productStore.UpdateRun(ctx, firstSuper); err != nil {
		t.Fatal(err)
	}
	if err := runtime.unbindSelfDevelopmentSuper(ctx, &firstSuper); err != nil {
		t.Fatal(err)
	}

	// Terminal event wakes Texture (queues owner instruction on Texture trajectory), never mints Super directly.
	rewakeErr := runtime.maybeRewakeSelfDevelopmentTextureAfterTerminalSuper(ctx, ownerID)
	if rewakeErr != nil {
		t.Fatalf("rewake Texture error: %v", rewakeErr)
	}
	// Before Texture turn commits a new execution_request, reconcile mints ZERO Super:
	noSuper, err := runtime.reconcilePersistentSuperActor(ctx, ownerID, superAgentID)
	if err != nil {
		t.Fatal(err)
	}
	if noSuper != nil {
		t.Fatalf("expected nil Super before Texture turn commits execution_request, got: %+v", noSuper)
	}

	// Texture turn commits a NEW typed execution_request in response to the instruction:
	docID, _, textureWorkID, trajectoryID, superWorkID, _ := selfDevelopmentTextureJoinIDs(ownerID, computerID, operation.OperationID)
	textureAgentID := agentprofile.Texture + ":" + docID
	snapshot, err := productStore.GetLifecycleSnapshot(ctx, ownerID, computerID, trajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	textureAgent, err := productStore.GetAgentByScope(ctx, ownerID, computerID, textureAgentID)
	if err != nil {
		t.Fatal(err)
	}
	packet, err := PrepareTextureControlPacket(types.CoagentSourcePacketPayload{
		SchemaVersion: types.CoagentSourcePacketSchemaV1,
		Kind:          "execution_request",
		Summary:       "Continue self-development operation",
		Sources: []types.CoagentPacketSource{{
			SourceID: "src-operation",
			Kind:     sourcecontract.SourceKindCapsuleBundle,
			Target:   types.CoagentPacketSourceTarget{URI: "operation:" + operation.OperationID},
		}},
		Actions: []types.CoagentPacketAction{{
			Type:      "run_command",
			Objective: originalPrompt,
			Safety: types.CoagentPacketActionSafety{
				MutationClass: "green",
				Network:       "forbidden",
				FileMutation:  "forbidden",
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	content := BuildTextureLifecycleControlContent(packet, superAgentID, superWorkID)
	payloadDigest, err := store.ComputeLifecycleUpdatePayloadDigest(packet, content)
	if err != nil {
		t.Fatal(err)
	}
	expectedInstructions, err := productStore.ListPendingLifecycleOwnerInstructionsForHead(ctx, ownerID, computerID, trajectoryID, textureAgentID, snapshot.HeadRevision.RevisionID)
	if err != nil {
		t.Fatal(err)
	}
	var ownerInstructions []types.TextureTurnOwnerInstruction
	for _, inst := range expectedInstructions {
		ownerInstructions = append(ownerInstructions, types.TextureTurnOwnerInstruction{
			InstructionID: inst.InstructionID,
			RequestID:     inst.RequestID,
		})
	}
	turn := types.ApplyTextureTurnRequest{
		OwnerID:                        ownerID,
		ComputerID:                     computerID,
		CommandID:                      "turn:selfdev-texture-rewake:" + operation.OperationID,
		DocumentID:                     docID,
		TrajectoryID:                   trajectoryID,
		CallerAgentID:                  textureAgentID,
		CallerRunID:                    runtime.selfDevelopmentCallerRunID(ownerID, computerID, trajectoryID),
		ExpectedLifecycleVersion:       snapshot.Trajectory.LifecycleVersion,
		ExpectedCallerLifecycleVersion: textureAgent.LifecycleVersion,
		ExpectedHeadRevisionID:         snapshot.HeadRevision.RevisionID,
		CallerWorkItemID:               textureWorkID,
		CallerWorkDisposition:          types.WorkItemOpen,
		Outcome:                        types.TextureTurnWait,
		Reason:                         "continue after terminal Super",
		Controls: []types.TextureTurnControl{{
			ControlID:        "control-rewake-" + operation.OperationID,
			TargetAgentID:    superAgentID,
			TargetWorkItemID: superWorkID,
			Packet:           packet,
			Content:          content,
			PayloadDigest:    payloadDigest,
		}},
		OwnerInstructions: ownerInstructions,
	}
	turn.CommandDigest, err = store.ComputeApplyTextureTurnDigest(turn)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := productStore.ApplyTextureTurn(ctx, turn); err != nil {
		t.Fatal(err)
	}

	// Now the live trigger wakes Super and mints exactly one replacement Super:
	rewokeSuper, err := runtime.reconcilePersistentSuperActor(ctx, ownerID, superAgentID)
	if err != nil {
		t.Fatalf("reconcile after terminal Super: %v", err)
	}
	if rewokeSuper == nil {
		t.Fatal("expected replacement Super from Texture rewake, got nil")
	}
	if rewokeSuper.RunID == firstSuper.RunID {
		t.Fatalf("expected new Super run, got same run %s", firstSuper.RunID)
	}
	if metadataStringValue(rewokeSuper.Metadata, "request_source") != "lifecycle_texture_control" {
		t.Fatalf("rewoke Super request_source=%q", metadataStringValue(rewokeSuper.Metadata, "request_source"))
	}
	if metadataStringValue(rewokeSuper.Metadata, "self_development_operation_id") != operation.OperationID {
		t.Fatalf("rewoke Super missing operation_id: %+v", rewokeSuper.Metadata)
	}
	if len(metadataStringSlice(rewokeSuper.Metadata[runMetadataProducerReportIDs])) > 0 {
		t.Fatalf("rewoke Super should not carry producer_report_ids: %+v", rewokeSuper.Metadata)
	}

	// Calling reconcile again while the rewoke Super is active returns the resident run immediately.
	resident, err := runtime.reconcilePersistentSuperActor(ctx, ownerID, superAgentID)
	if err != nil {
		t.Fatal(err)
	}
	if resident.RunID != rewokeSuper.RunID {
		t.Fatalf("resident run %s != rewoke run %s", resident.RunID, rewokeSuper.RunID)
	}
}
