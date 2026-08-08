package textureowner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func textureTurnRuntimeID(rec *types.RunRecord, toolCallID, kind string, ordinal int) (string, error) {
	toolCallID = strings.TrimSpace(toolCallID)
	if rec == nil || toolCallID == "" || strings.TrimSpace(rec.OwnerID) == "" || strings.TrimSpace(rec.SandboxID) == "" || strings.TrimSpace(rec.RunID) == "" {
		return "", fmt.Errorf("Texture lifecycle turn requires authenticated runtime tool_call_id and run scope")
	}
	seed := strings.Join([]string{rec.OwnerID, rec.SandboxID, rec.RunID, toolCallID, kind, fmt.Sprintf("%d", ordinal)}, "\x00")
	sum := sha256.Sum256([]byte(seed))
	raw := append([]byte(nil), sum[:16]...)
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	id, err := uuid.FromBytes(raw)
	if err != nil {
		return "", fmt.Errorf("derive Texture lifecycle %s identity: %w", kind, err)
	}
	return id.String(), nil
}

func textureTurnCallerAgent(snapshot types.LifecycleSnapshot, agentID string) (types.AgentRecord, error) {
	for _, agent := range snapshot.Agents {
		if strings.TrimSpace(agent.AgentID) == agentID {
			return agent, nil
		}
	}
	return types.AgentRecord{}, fmt.Errorf("Texture lifecycle caller %q is absent from trajectory snapshot", agentID)
}

func textureTurnPendingInbound(snapshot types.LifecycleSnapshot, callerAgentID string, decisions []textureUpdateDisposition, resultRef string) ([]types.TextureTurnInboundDisposition, error) {
	pending := make(map[string]types.CoagentSourcePacket)
	for _, update := range snapshot.Updates {
		if update.Disposition != types.UpdatePending || update.Direction != types.LifecyclePacketDirectionProducerReport || strings.TrimSpace(update.TargetAgentID) != callerAgentID {
			continue
		}
		pending[strings.TrimSpace(update.UpdateID)] = update
	}
	inbound := make([]types.TextureTurnInboundDisposition, 0, len(decisions))
	for _, decision := range decisions {
		update, ok := pending[strings.TrimSpace(decision.UpdateID)]
		if !ok {
			return nil, fmt.Errorf("Texture update disposition %q does not name a pending target-bound producer report", decision.UpdateID)
		}
		producerWorkID := strings.TrimSpace(update.ProducerWorkItemID)
		if producerWorkID == "" {
			return nil, fmt.Errorf("Texture update disposition %q lacks explicit producer work identity", decision.UpdateID)
		}
		workDisposition := update.WorkDisposition
		if workDisposition == "" {
			workDisposition = types.WorkItemOpen
		}
		disposition := types.UpdateDisposition(strings.TrimSpace(decision.Disposition))
		workResultRef := ""
		if workDisposition == types.WorkItemCompleted {
			workResultRef = resultRef
		}
		if disposition == types.UpdateRejected && workDisposition == types.WorkItemCompleted {
			workDisposition = types.WorkItemRefused
			workResultRef = ""
		}
		inbound = append(inbound, types.TextureTurnInboundDisposition{
			TargetAgentID: callerAgentID, ProducerAgentID: update.AgentID,
			ProducerUpdateID: update.ProducerUpdateID, UpdateID: update.UpdateID,
			Disposition: disposition, ProducerWorkItemID: producerWorkID,
			WorkDisposition: workDisposition, WorkResultRef: workResultRef,
			Reason: strings.TrimSpace(decision.Reason),
		})
	}
	return inbound, nil
}

func (h *Handler) textureTurnControls(ctx context.Context, rec *types.RunRecord, doc types.Document, snapshot types.LifecycleSnapshot, in editTextureArgs) ([]types.TextureTurnControl, error) {
	workByID := make(map[string]types.WorkItemRecord, len(snapshot.WorkItems))
	for _, work := range snapshot.WorkItems {
		workByID[strings.TrimSpace(work.WorkItemID)] = work
	}
	controls := make([]types.TextureTurnControl, 0, len(in.Controls))
	for i, raw := range in.Controls {
		packet, err := agentcore.PrepareTextureControlPacket(raw.Packet)
		if err != nil {
			return nil, fmt.Errorf("Texture controls[%d] packet: %w", i, err)
		}
		controlID, err := textureTurnRuntimeID(rec, in.ToolCallID, "control", i)
		if err != nil {
			return nil, err
		}
		targetAgentID, targetWorkItemID := "", strings.TrimSpace(raw.TargetWorkItemID)
		var openAgent *types.AgentRecord
		var openWork *types.WorkItemRecord
		if raw.OpenPersistentSuper {
			targetAgentID = agentprofile.Super + ":" + strings.TrimSpace(rec.OwnerID)
			targetWorkItemID, err = textureTurnRuntimeID(rec, in.ToolCallID, "persistent-super-work", i)
			if err != nil {
				return nil, err
			}
			if packet.Kind != "execution_request" || len(packet.Actions) == 0 {
				return nil, fmt.Errorf("Texture controls[%d] persistent-Super opener requires execution_request actions", i)
			}
			work := types.WorkItemRecord{
				WorkItemID: targetWorkItemID, Objective: strings.TrimSpace(raw.Objective),
				AuthorityProfile: agentprofile.Super, Status: types.WorkItemOpen,
				AssignedAgentID: targetAgentID,
			}
			openWork = &work
		} else if raw.OpenResearcher {
			agentIdentity, identityErr := textureTurnRuntimeID(rec, in.ToolCallID, "researcher-agent", i)
			if identityErr != nil {
				return nil, identityErr
			}
			targetAgentID = agentprofile.Researcher + ":" + agentIdentity
			targetWorkItemID, err = textureTurnRuntimeID(rec, in.ToolCallID, "researcher-work", i)
			if err != nil {
				return nil, err
			}
			agent := types.AgentRecord{AgentID: targetAgentID, Profile: agentprofile.Researcher, Role: agentprofile.Researcher, ChannelID: doc.DocID}
			work := types.WorkItemRecord{
				WorkItemID: targetWorkItemID, Objective: strings.TrimSpace(raw.Objective),
				AuthorityProfile: agentprofile.Researcher, Status: types.WorkItemOpen,
				AssignedAgentID: targetAgentID,
			}
			openAgent, openWork = &agent, &work
		} else {
			work, ok := workByID[targetWorkItemID]
			if !ok || work.Status != types.WorkItemOpen || strings.TrimSpace(work.TrajectoryID) != strings.TrimSpace(doc.TrajectoryID) {
				return nil, fmt.Errorf("Texture controls[%d] target work is not an open current-trajectory obligation", i)
			}
			targetAgentID = strings.TrimSpace(work.AssignedAgentID)
			if targetAgentID == "" {
				return nil, fmt.Errorf("Texture controls[%d] target work is unassigned", i)
			}
		}
		// Runtime lookup is an early fail-closed refusal for existing targets; a
		// Researcher opener proves absence and creates its runtime-derived agent in
		// the same ApplyTextureTurn CAS as work and first control.
		if openAgent == nil {
			if _, err := h.Store.GetAgentByScope(ctx, rec.OwnerID, doc.ComputerID, targetAgentID); err != nil {
				return nil, fmt.Errorf("Texture controls[%d] load exact target: %w", i, err)
			}
		}
		content := agentcore.BuildTextureLifecycleControlContent(packet, targetAgentID, targetWorkItemID)
		payloadDigest, err := store.ComputeLifecycleUpdatePayloadDigest(packet, content)
		if err != nil {
			return nil, fmt.Errorf("Texture controls[%d] payload digest: %w", i, err)
		}
		controls = append(controls, types.TextureTurnControl{
			ControlID: controlID, TargetAgentID: targetAgentID, TargetWorkItemID: targetWorkItemID,
			OpenAgent: openAgent, OpenWork: openWork, Packet: packet, Content: content, PayloadDigest: payloadDigest,
		})
	}
	return controls, nil
}

func (h *Handler) applyTextureLifecycleTurn(ctx context.Context, rec *types.RunRecord, doc types.Document, in editTextureArgs, outcome types.TextureTurnOutcome, revision types.Revision, graph store.TextureSourceGraphWriteSet, reason string) (types.LifecycleResult, error) {
	snapshot, err := h.Store.GetLifecycleSnapshot(ctx, rec.OwnerID, doc.ComputerID, doc.TrajectoryID)
	if err != nil {
		return types.LifecycleResult{}, fmt.Errorf("load Texture lifecycle turn snapshot: %w", err)
	}
	caller, err := textureTurnCallerAgent(snapshot, strings.TrimSpace(rec.AgentID))
	if err != nil {
		return types.LifecycleResult{}, err
	}
	resultRef := strings.TrimSpace(revision.RevisionID)
	if resultRef == "" {
		resultRef = strings.TrimSpace(doc.CurrentRevisionID)
	}
	inbound, err := textureTurnPendingInbound(snapshot, rec.AgentID, in.UpdateDispositions, resultRef)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	controls, err := h.textureTurnControls(ctx, rec, doc, snapshot, in)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	commandUUID, err := textureTurnRuntimeID(rec, in.ToolCallID, "command", 0)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	callerWorkItemID := strings.TrimSpace(metadataStringValue(rec.Metadata, "lifecycle_work_item_id"))
	callerWorkDisposition := types.WorkItemStatus(strings.TrimSpace(in.WorkDisposition))
	if callerWorkDisposition == "" {
		callerWorkDisposition = types.WorkItemOpen
	}
	instructionIDs := metadataStringSlice(rec.Metadata[textureOwnerInstructionIDsMetadata])
	requestIDs := metadataStringSlice(rec.Metadata[textureOwnerRequestIDsMetadata])
	if len(instructionIDs) != len(requestIDs) {
		return types.LifecycleResult{}, fmt.Errorf("authenticated owner instruction metadata is incomplete")
	}
	ownerInstructions := make([]types.TextureTurnOwnerInstruction, 0, len(instructionIDs))
	for i := range instructionIDs {
		if strings.TrimSpace(instructionIDs[i]) == "" || strings.TrimSpace(requestIDs[i]) == "" {
			return types.LifecycleResult{}, fmt.Errorf("authenticated owner instruction metadata is empty")
		}
		ownerInstructions = append(ownerInstructions, types.TextureTurnOwnerInstruction{InstructionID: instructionIDs[i], RequestID: requestIDs[i]})
	}
	req := types.ApplyTextureTurnRequest{
		OwnerID: rec.OwnerID, ComputerID: doc.ComputerID,
		CommandID: "texture-turn:" + commandUUID, DocumentID: doc.DocID, TrajectoryID: doc.TrajectoryID,
		CallerAgentID: rec.AgentID, CallerRunID: rec.RunID, OwnerInstructions: ownerInstructions,
		ExpectedLifecycleVersion:       snapshot.Trajectory.LifecycleVersion,
		ExpectedCallerLifecycleVersion: caller.LifecycleVersion,
		ExpectedHeadRevisionID:         doc.CurrentRevisionID,
		CallerWorkItemID:               callerWorkItemID,
		CallerWorkDisposition:          callerWorkDisposition,
		Outcome:                        outcome, Revision: revision, Reason: strings.TrimSpace(reason), Inbound: inbound, Controls: controls,
	}
	req.CommandDigest, err = store.ComputeApplyTextureTurnWithSourceGraphDigest(req, graph)
	if err != nil {
		return types.LifecycleResult{}, fmt.Errorf("digest Texture lifecycle turn: %w", err)
	}
	result, err := h.Store.ApplyTextureTurnWithSourceGraph(ctx, req, graph)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	// A successful replay is durable success but not a new commit. Only packets
	// first created by this commit are eligible for actor wake.
	if !result.Replay && h.wakeTextureControl != nil {
		seenTargets := map[string]bool{}
		for _, control := range result.Controls {
			key := control.TrajectoryID + "\x00" + control.TargetAgentID
			if seenTargets[key] {
				continue
			}
			seenTargets[key] = true
			h.wakeTextureControl(context.WithoutCancel(ctx), control)
		}
	}
	return result, nil
}

func textureTurnAuditDigest(result types.LifecycleResult) string {
	payload := strings.Join([]string{result.Receipt.CommandID, result.Receipt.CommandDigest, fmt.Sprintf("%d", result.Trajectory.LifecycleVersion)}, "\x00")
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

func textureDecisionTurnOutcome(kind string) types.TextureTurnOutcome {
	switch strings.TrimSpace(kind) {
	case "wait_for_evidence", "delegation_deferred":
		return types.TextureTurnWait
	case "blocker":
		return types.TextureTurnBlock
	default:
		return types.TextureTurnNoSemanticChange
	}
}

func (h *Handler) commitTextureNonRevisionTurn(ctx context.Context, rec *types.RunRecord, in recordTextureDecisionArgs, toolCallID string) (types.LifecycleResult, error) {
	if h == nil || h.Store == nil || rec == nil {
		return types.LifecycleResult{}, fmt.Errorf("Texture runtime store unavailable")
	}
	h.textureEditMu.Lock()
	defer h.textureEditMu.Unlock()
	computerID := strings.TrimSpace(rec.SandboxID)
	docID := strings.TrimSpace(in.DocID)
	if docID == "" {
		docID = strings.TrimSpace(firstNonEmpty(metadataStringValue(rec.Metadata, "doc_id"), rec.ChannelID))
	}
	if docID == "" || docID != strings.TrimSpace(metadataStringValue(rec.Metadata, "doc_id")) || rec.ChannelID != docID {
		return types.LifecycleResult{}, fmt.Errorf("record_texture_decision does not match authenticated Texture document")
	}
	mutation, err := h.Store.GetAgentMutationByRun(ctx, rec.OwnerID, computerID, rec.RunID)
	if err != nil || mutation == nil || mutation.State != "pending" || mutation.DocID != docID {
		if err != nil {
			return types.LifecycleResult{}, fmt.Errorf("load Texture mutation: %w", err)
		}
		return types.LifecycleResult{}, fmt.Errorf("Texture mutation is not pending for this document")
	}
	subject, err := h.Store.GetAgentByScope(ctx, rec.OwnerID, computerID, rec.AgentID)
	if err != nil || subject.LifecycleVersion <= 0 || rec.AgentID != currentTextureAgentID(docID) {
		if err != nil {
			return types.LifecycleResult{}, fmt.Errorf("load scoped lifecycle Texture subject: %w", err)
		}
		return types.LifecycleResult{}, fmt.Errorf("record_texture_decision requires exact lifecycle Texture caller")
	}
	doc, err := h.Store.GetLifecycleDocument(ctx, rec.OwnerID, computerID, docID)
	if err != nil {
		return types.LifecycleResult{}, fmt.Errorf("load lifecycle Texture document: %w", err)
	}
	if strings.TrimSpace(in.BaseRevisionID) == "" || strings.TrimSpace(in.BaseRevisionID) != doc.CurrentRevisionID {
		return types.LifecycleResult{}, fmt.Errorf("base_revision_id is required and must equal current revision %q", doc.CurrentRevisionID)
	}
	editIn := editTextureArgs{
		DocID: docID, BaseRevisionID: in.BaseRevisionID, UpdateDispositions: in.UpdateDispositions,
		Controls: in.Controls, WorkDisposition: string(types.WorkItemOpen), ToolCallID: strings.TrimSpace(toolCallID),
	}
	result, err := h.applyTextureLifecycleTurn(ctx, rec, doc, editIn, textureDecisionTurnOutcome(in.DecisionKind), types.Revision{}, store.TextureSourceGraphWriteSet{}, in.Reason)
	if err != nil {
		return types.LifecycleResult{}, fmt.Errorf("apply atomic Texture non-revision turn: %w", err)
	}
	// The lifecycle turn is the authority. Best-effort deferral prevents the
	// legacy revision-required finalizer from misclassifying this committed
	// no-revision outcome; it never writes the existing head as a fake revision.
	_ = h.Store.SleepAgentMutationAfterTextureTurn(context.WithoutCancel(ctx), rec.OwnerID, computerID, doc.TrajectoryID, rec.RunID)
	return result, nil
}
