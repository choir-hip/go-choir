package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/sourcecontract"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func selfDevelopmentOperationIDFromPacketSources(sources []types.CoagentPacketSource) string {
	for _, source := range sources {
		key, value := splitTypedWorkerUpdateRef(source.Target.URI)
		if key == "operation" && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func selfDevelopmentTextureJoinIDs(ownerID, computerID, operationID string) (docID, revisionID, textureWorkID, trajectoryID, superWorkID, controlID string) {
	key := strings.Join([]string{"choir:texture:self-development", ownerID, computerID, operationID}, ":")
	docID = uuid.NewSHA1(uuid.NameSpaceOID, []byte(key+":document")).String()
	revisionID = uuid.NewSHA1(uuid.NameSpaceOID, []byte(key+":revision:v0")).String()
	textureWorkID = uuid.NewSHA1(uuid.NameSpaceOID, []byte(key+":work:texture")).String()
	trajectoryID = uuid.NewSHA1(uuid.NameSpaceOID, []byte(key+":trajectory")).String()
	superWorkID = uuid.NewSHA1(uuid.NameSpaceOID, []byte(key+":work:super")).String()
	controlID = uuid.NewSHA1(uuid.NameSpaceOID, []byte(key+":control:opener")).String()
	return docID, revisionID, textureWorkID, trajectoryID, superWorkID, controlID
}

func (rt *Runtime) startSelfDevelopmentPersistentSuper(ctx context.Context, operation selfdev.Operation, ownerID, prompt string) error {
	if rt == nil || rt.store == nil {
		return fmt.Errorf("start self-development run: persistent Super authority unavailable")
	}
	ownerID = strings.TrimSpace(ownerID)
	computerID := strings.TrimSpace(rt.TextureComputerID())
	if ownerID == "" || computerID == "" {
		return fmt.Errorf("start self-development run: owner and computer identity are required")
	}
	if strings.TrimSpace(operation.ComputerID) != computerID {
		return fmt.Errorf("start self-development run: operation computer does not match this runtime")
	}
	if _, err := rt.EnsurePersistentSuperAgent(ctx, ownerID); err != nil {
		return fmt.Errorf("start self-development run: %w", err)
	}
	if err := rt.ensureSelfDevelopmentTextureJoin(ctx, operation, ownerID, prompt); err != nil {
		return err
	}
	rec, err := rt.reconcilePersistentSuperActor(ctx, ownerID, persistentSuperAgentID(ownerID))
	if err != nil {
		return fmt.Errorf("start self-development run: %w", err)
	}
	if rec == nil {
		return fmt.Errorf("start self-development run: Texture control did not wake persistent Super")
	}
	if err := rt.bindSelfDevelopmentOperationToPersistentSuper(ctx, rec, operation); err != nil {
		return err
	}
	if strings.TrimSpace(metadataStringValue(rec.Metadata, "assignment_trajectory_id")) == "" {
		return fmt.Errorf("start self-development run: persistent Super still lacks Texture trajectory binding")
	}
	return nil
}

func (rt *Runtime) bindSelfDevelopmentOperationToPersistentSuper(ctx context.Context, rec *types.RunRecord, operation selfdev.Operation) error {
	if rec == nil {
		return fmt.Errorf("start self-development run: persistent Super unavailable")
	}
	rec.Metadata = cloneMetadata(rec.Metadata)
	if rec.Metadata == nil {
		rec.Metadata = map[string]any{}
	}
	rec.RequestedByRunID = ""
	delete(rec.Metadata, "requested_by_run_id")
	rec.Metadata["self_development_operation_id"] = operation.OperationID
	rec.Metadata["self_development_computer_id"] = operation.ComputerID
	if strings.TrimSpace(operation.PromptArtifactRef) != "" {
		rec.Metadata["self_development_prompt_artifact_ref"] = operation.PromptArtifactRef
	}
	if err := rt.store.UpdateRun(ctx, *rec); err != nil {
		return fmt.Errorf("bind self-development operation to persistent Super: %w", err)
	}
	return nil
}

func (rt *Runtime) ensureSelfDevelopmentTextureJoin(ctx context.Context, operation selfdev.Operation, ownerID, prompt string) error {
	computerID := strings.TrimSpace(rt.TextureComputerID())
	docID, revisionID, textureWorkID, trajectoryID, superWorkID, openerControlID := selfDevelopmentTextureJoinIDs(ownerID, computerID, operation.OperationID)
	textureAgentID := agentprofile.Texture + ":" + docID
	superAgentID := persistentSuperAgentID(ownerID)
	now := time.Now().UTC()
	joinMeta, _ := json.Marshal(map[string]any{"self_development_operation_id": operation.OperationID})
	start := types.StartLifecycleRequest{
		OwnerID: ownerID, ComputerID: computerID, CommandID: "start:selfdev-texture:" + operation.OperationID,
		TrajectoryID: trajectoryID, Kind: types.TrajectoryKindDocument,
		SubjectRefs:    map[string]string{"artifact": "texture://documents/" + docID, "doc_id": docID},
		SettlementRule: types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{"artifact"}},
		InitialWork:    types.WorkItemRecord{WorkItemID: textureWorkID, Objective: "Supervise self-development on this computer.", AssignedAgentID: textureAgentID, AuthorityProfile: agentprofile.Texture},
		InitialDocument: types.Document{
			DocID: docID, OwnerID: ownerID, ComputerID: computerID, TrajectoryID: trajectoryID,
			Title: "Self-development supervision", CreatedAt: now, UpdatedAt: now,
		},
		InitialRevision: types.Revision{
			RevisionID: revisionID, DocID: docID, OwnerID: ownerID, ComputerID: computerID, TrajectoryID: trajectoryID,
			AuthorKind: types.AuthorUser, AuthorLabel: ownerID,
			Content: "Supervise self-development on this computer.", Metadata: joinMeta, CreatedAt: now,
		},
		Agent: types.AgentRecord{
			AgentID: textureAgentID, OwnerID: ownerID, ComputerID: computerID,
			Profile: agentprofile.Texture, Role: agentprofile.Texture, ChannelID: docID, CreatedAt: now, UpdatedAt: now,
		},
	}
	start.StartRequestDigest, _ = store.ComputeStartLifecycleRequestDigest(start)
	if _, err := rt.store.StartLifecycle(ctx, start); err != nil {
		return fmt.Errorf("start self-development Texture lifecycle: %w", err)
	}
	snapshot, err := rt.store.GetLifecycleSnapshot(ctx, ownerID, computerID, trajectoryID)
	if err != nil {
		return fmt.Errorf("load self-development Texture lifecycle: %w", err)
	}
	caller, err := rt.ensureSelfDevelopmentTextureCaller(ctx, ownerID, computerID, trajectoryID, textureAgentID, textureWorkID, docID)
	if err != nil {
		return err
	}
	pending, err := rt.store.ListAllPendingLifecycleUpdates(ctx, ownerID, computerID, superAgentID)
	if err != nil {
		return fmt.Errorf("load pending Super controls: %w", err)
	}
	for _, update := range pending {
		if update.Direction == types.LifecyclePacketDirectionControl &&
			selfDevelopmentOperationIDFromPacketSources(update.Packet.Sources) == operation.OperationID &&
			persistentSuperExecutablePacket(update) {
			return nil
		}
	}
	controlID := openerControlID
	commandID := "turn:selfdev-texture:" + operation.OperationID
	var openWork *types.WorkItemRecord
	targetWorkID := superWorkID
	if existing := selfDevelopmentOpenSuperWork(snapshot, superAgentID); existing != nil {
		targetWorkID = existing.WorkItemID
		controlID = uuid.NewSHA1(uuid.NameSpaceOID, []byte(strings.Join([]string{
			"choir:texture:self-development", ownerID, computerID, operation.OperationID, "continue", existing.WorkItemID, fmt.Sprint(existing.LastReducerSeq),
		}, ":"))).String()
		commandID = "turn:selfdev-texture-continue:" + operation.OperationID + ":" + fmt.Sprint(existing.LastReducerSeq)
	} else {
		openWork = &types.WorkItemRecord{
			WorkItemID: superWorkID, Objective: strings.TrimSpace(prompt),
			AuthorityProfile: agentprofile.Super, Status: types.WorkItemOpen, AssignedAgentID: superAgentID,
		}
	}
	packet, err := PrepareTextureControlPacket(types.CoagentSourcePacketPayload{
		SchemaVersion: types.CoagentSourcePacketSchemaV1,
		Kind:          "execution_request",
		Summary:       "Author, freeze, and propose the bound self-development operation.",
		Sources: []types.CoagentPacketSource{{
			SourceID: "src-operation",
			Kind:     sourcecontract.SourceKindCapsuleBundle,
			Target:   types.CoagentPacketSourceTarget{URI: "operation:" + operation.OperationID},
		}},
		Actions: []types.CoagentPacketAction{{
			Type:      "run_command",
			Objective: strings.TrimSpace(prompt),
			Safety: types.CoagentPacketActionSafety{
				MutationClass: "green",
				Network:       "forbidden",
				FileMutation:  "forbidden",
			},
		}},
	})
	if err != nil {
		return fmt.Errorf("prepare self-development Super execution_request: %w", err)
	}
	content := BuildTextureLifecycleControlContent(packet, superAgentID, targetWorkID)
	payloadDigest, err := store.ComputeLifecycleUpdatePayloadDigest(packet, content)
	if err != nil {
		return fmt.Errorf("digest self-development Super control: %w", err)
	}
	textureAgent, err := rt.store.GetAgentByScope(ctx, ownerID, computerID, textureAgentID)
	if err != nil {
		return fmt.Errorf("load self-development Texture agent: %w", err)
	}
	snapshot, err = rt.store.GetLifecycleSnapshot(ctx, ownerID, computerID, trajectoryID)
	if err != nil {
		return fmt.Errorf("reload self-development Texture lifecycle: %w", err)
	}
	turn := types.ApplyTextureTurnRequest{
		OwnerID: ownerID, ComputerID: computerID, CommandID: commandID, DocumentID: docID, TrajectoryID: trajectoryID,
		CallerAgentID: textureAgentID, CallerRunID: caller.RunID,
		ExpectedLifecycleVersion: snapshot.Trajectory.LifecycleVersion, ExpectedCallerLifecycleVersion: textureAgent.LifecycleVersion,
		ExpectedHeadRevisionID: snapshot.HeadRevision.RevisionID, CallerWorkItemID: textureWorkID,
		CallerWorkDisposition: types.WorkItemOpen, Outcome: types.TextureTurnWait, Reason: "wait after self-development Super control",
		Controls: []types.TextureTurnControl{{
			ControlID: controlID, TargetAgentID: superAgentID, TargetWorkItemID: targetWorkID,
			OpenWork: openWork, Packet: packet, Content: content, PayloadDigest: payloadDigest,
		}},
	}
	turn.CommandDigest, err = store.ComputeApplyTextureTurnDigest(turn)
	if err != nil {
		return fmt.Errorf("digest self-development Texture turn: %w", err)
	}
	if _, err := rt.store.ApplyTextureTurn(ctx, turn); err != nil {
		return fmt.Errorf("apply self-development Texture Super opener: %w", err)
	}
	return nil
}

func selfDevelopmentOpenSuperWork(snapshot types.LifecycleSnapshot, superAgentID string) *types.WorkItemRecord {
	var found *types.WorkItemRecord
	for i := range snapshot.WorkItems {
		work := snapshot.WorkItems[i]
		if work.Status == types.WorkItemOpen && work.AssignedAgentID == superAgentID && work.AuthorityProfile == agentprofile.Super {
			if found != nil {
				return found
			}
			copy := work
			found = &copy
		}
	}
	return found
}

func (rt *Runtime) ensureSelfDevelopmentTextureCaller(ctx context.Context, ownerID, computerID, trajectoryID, textureAgentID, textureWorkID, docID string) (types.RunRecord, error) {
	if rec, ok, err := rt.TextureActiveRunByAgent(ctx, ownerID, computerID, textureAgentID); err != nil {
		return types.RunRecord{}, fmt.Errorf("load self-development Texture caller: %w", err)
	} else if ok {
		return rec, nil
	}
	now := time.Now().UTC()
	runID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(strings.Join([]string{
		"choir:texture:self-development", ownerID, computerID, trajectoryID, "texture-run",
	}, ":"))).String()
	caller := types.RunRecord{
		RunID: runID, OwnerID: ownerID, ComputerID: computerID, AgentID: textureAgentID,
		AgentProfile: agentprofile.Texture, AgentRole: agentprofile.Texture, ChannelID: docID,
		TrajectoryID: trajectoryID, State: types.RunRunning,
		Prompt: "Supervise self-development on this computer.",
		Metadata: map[string]any{
			"lifecycle_work_item_id": textureWorkID,
			"work_item_ids":          []string{textureWorkID},
			runMetadataAgentProfile:  agentprofile.Texture,
			runMetadataAgentRole:     agentprofile.Texture,
		},
		CreatedAt: now, UpdatedAt: now,
	}
	project := types.ReplaceLifecycleActivationRequest{
		OwnerID: ownerID, ComputerID: computerID, CommandID: "project:selfdev-texture:" + runID,
		TrajectoryID: trajectoryID, AgentID: textureAgentID, Run: caller,
	}
	project.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(project)
	if _, err := rt.store.ReplaceLifecycleActivation(ctx, project); err != nil {
		return types.RunRecord{}, fmt.Errorf("project self-development Texture caller: %w", err)
	}
	loaded, err := rt.store.GetLifecycleRun(ctx, ownerID, computerID, runID)
	if err != nil {
		return types.RunRecord{}, fmt.Errorf("reload self-development Texture caller: %w", err)
	}
	return loaded, nil
}
