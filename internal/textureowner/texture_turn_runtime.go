package textureowner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func textureTurnRuntimeID(rec *types.RunRecord, toolCallID, kind string, ordinal int) (string, error) {
	toolCallID = strings.TrimSpace(toolCallID)
	if rec == nil || toolCallID == "" || strings.TrimSpace(rec.OwnerID) == "" || strings.TrimSpace(rec.ComputerID) == "" || strings.TrimSpace(rec.RunID) == "" {
		return "", fmt.Errorf("Texture lifecycle turn requires authenticated runtime tool_call_id and run scope")
	}
	seed := strings.Join([]string{rec.OwnerID, rec.ComputerID, rec.RunID, toolCallID, kind, fmt.Sprintf("%d", ordinal)}, "\x00")
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

// textureDispositionIneligibleReason names which eligibility rule a named
// update fails, so a refused disposition is diagnosable from the desk's
// own error and from the trace (M11 rerun 10).
func textureDispositionIneligibleReason(snapshot types.LifecycleSnapshot, callerAgentID, runID string, scheduledSeq int64, updateID string) string {
	updateID = strings.TrimSpace(updateID)
	for _, update := range snapshot.Updates {
		if strings.TrimSpace(update.UpdateID) != updateID {
			continue
		}
		switch {
		case update.Disposition != types.UpdatePending:
			return fmt.Sprintf("it has disposition %q, not pending", update.Disposition)
		case update.Direction != types.LifecyclePacketDirectionProducerReport:
			return fmt.Sprintf("it has direction %q, not producer_report", update.Direction)
		case strings.TrimSpace(update.TargetAgentID) != callerAgentID:
			return fmt.Sprintf("it targets %q, not this desk %q", update.TargetAgentID, callerAgentID)
		case scheduledSeq <= 0:
			return "this activation has no scheduled message seq"
		case update.MessageSeq > scheduledSeq:
			return fmt.Sprintf("its message seq %d is after this activation's scheduled seq %d", update.MessageSeq, scheduledSeq)
		}
		if bound := strings.TrimSpace(update.DeliveredToRunID); bound != "" && bound != strings.TrimSpace(runID) {
			return fmt.Sprintf("it is bound to run %q, not this run", bound)
		}
		return "it is eligible; the id was matched inexactly"
	}
	return "the snapshot has no update with this id"
}

// textureTurnPendingInbound converts the desk's explicit update_dispositions
// plus the consume-at-commit default into one ordered inbound set. Every
// pending producer report addressed to this caller whose MessageSeq is covered
// by the run's scheduled_seq (and which is unbound or bound to this run)
// terminalizes inside the committed turn: named packets get the desk's
// explicit disposition; un-named packets default to delivered — neutral
// receipt, never auto-incorporated (incorporated requires a work result ref).
// Iterate snapshot order so the synthesized inbound set is deterministic: the
// command digest covers req.Inbound, so identical cells produce identical
// requests (devin's digest-determinism point in the respawn-loop receipt).
func textureTurnPendingInbound(snapshot types.LifecycleSnapshot, rec *types.RunRecord, decisions []textureUpdateDisposition, resultRef string) ([]types.TextureTurnInboundDisposition, error) {
	callerAgentID := strings.TrimSpace(rec.AgentID)
	scheduledSeq := int64(metadataIntValue(rec.Metadata, "scheduled_message_seq"))
	pending := make(map[string]types.CoagentSourcePacket)
	eligibleOrder := make([]string, 0, len(snapshot.Updates))
	for _, update := range snapshot.Updates {
		if update.Disposition != types.UpdatePending || update.Direction != types.LifecyclePacketDirectionProducerReport || strings.TrimSpace(update.TargetAgentID) != callerAgentID {
			continue
		}
		// Consume boundary: packets dispatched to this activation (seq <=
		// scheduledSeq) and either unclaimed or claimed by this run. A packet
		// bound to another live run is left pending; reconcile re-binds
		// stranded claims on dead runs before dispatch.
		if scheduledSeq <= 0 || update.MessageSeq > scheduledSeq {
			continue
		}
		if bound := strings.TrimSpace(update.DeliveredToRunID); bound != "" && bound != strings.TrimSpace(rec.RunID) {
			continue
		}
		pending[strings.TrimSpace(update.UpdateID)] = update
		eligibleOrder = append(eligibleOrder, strings.TrimSpace(update.UpdateID))
	}
	inbound := make([]types.TextureTurnInboundDisposition, 0, len(eligibleOrder))
	consumed := make(map[string]bool, len(eligibleOrder))
	for _, decision := range decisions {
		update, ok := pending[strings.TrimSpace(decision.UpdateID)]
		if !ok {
			return nil, fmt.Errorf("Texture update disposition %q does not name a pending target-bound producer report eligible to this activation: %s", decision.UpdateID, textureDispositionIneligibleReason(snapshot, callerAgentID, rec.RunID, scheduledSeq, decision.UpdateID))
		}
		consumed[strings.TrimSpace(decision.UpdateID)] = true
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
	// Consume-at-commit default: covered packets the desk did not name
	// terminalize as delivered inside this same atomic commit.
	for _, updateID := range eligibleOrder {
		if consumed[updateID] {
			continue
		}
		update := pending[updateID]
		inbound = append(inbound, types.TextureTurnInboundDisposition{
			TargetAgentID: callerAgentID, ProducerAgentID: update.AgentID,
			ProducerUpdateID: update.ProducerUpdateID, UpdateID: update.UpdateID,
			Disposition:        types.UpdateDelivered,
			ProducerWorkItemID: strings.TrimSpace(update.ProducerWorkItemID),
			WorkDisposition:    types.WorkItemOpen,
		})
	}
	return inbound, nil
}

// textureResearchBudgetPerOwnerRequest bounds the research assignments
// Texture opens per owner request (or per document creation). Without it a
// research report wakes Texture, that turn opens more research, and the
// document never idles (texture-research-loop-never-idles-2026-10-09).
const textureResearchBudgetPerOwnerRequest = 2

// researchOpenedSinceOwnerInput counts research work opened since the latest
// owner input on the trajectory (creation or an owner revise), excluding
// own: the deterministic work ids of the turn being built, so replaying a
// committed turn computes the same controls.
func researchOpenedSinceOwnerInput(snapshot types.LifecycleSnapshot, own map[string]bool) int {
	var since time.Time
	for _, ev := range snapshot.Events {
		ownerInput := ev.Kind == types.LifecycleTrajectoryStarted ||
			(ev.Kind == types.LifecycleArtifactHeadAdvanced && strings.HasPrefix(ev.CommandID, "owner-revise:"))
		if ownerInput && ev.CreatedAt.After(since) {
			since = ev.CreatedAt
		}
	}
	n := 0
	for _, work := range snapshot.WorkItems {
		if work.AuthorityProfile == agentprofile.Research && !own[strings.TrimSpace(work.WorkItemID)] && !work.CreatedAt.Before(since) {
			n++
		}
	}
	return n
}

// textureTurnControls builds the turn's controls. A research opener past the
// per-request budget is dropped, not failed: the turn still commits, and the
// returned note goes into the committed reason.
func (h *Handler) textureTurnControls(ctx context.Context, rec *types.RunRecord, doc types.Document, snapshot types.LifecycleSnapshot, in editTextureArgs) ([]types.TextureTurnControl, string, error) {
	workByID := make(map[string]types.WorkItemRecord, len(snapshot.WorkItems))
	for _, work := range snapshot.WorkItems {
		workByID[strings.TrimSpace(work.WorkItemID)] = work
	}
	own := map[string]bool{}
	for i, raw := range in.Controls {
		if raw.OpenResearch {
			if id, err := textureTurnRuntimeID(rec, in.ToolCallID, "researcher-work", i); err == nil {
				own[id] = true
			}
		}
	}
	researchOpened := researchOpenedSinceOwnerInput(snapshot, own)
	dropped := 0
	controls := make([]types.TextureTurnControl, 0, len(in.Controls))
	for i, raw := range in.Controls {
		if raw.OpenResearch {
			if researchOpened >= textureResearchBudgetPerOwnerRequest {
				dropped++
				continue
			}
			researchOpened++
		}
		packet, err := agentcore.PrepareTextureControlPacket(raw.Packet)
		if err != nil {
			return nil, "", fmt.Errorf("Texture controls[%d] packet: %w", i, err)
		}
		controlID, err := textureTurnRuntimeID(rec, in.ToolCallID, "control", i)
		if err != nil {
			return nil, "", err
		}
		targetAgentID, targetWorkItemID := "", strings.TrimSpace(raw.TargetWorkItemID)
		var openAgent *types.AgentRecord
		var openWork *types.WorkItemRecord
		if raw.OpenPersistentManagement {
			targetAgentID = agentprofile.Management + ":" + strings.TrimSpace(rec.OwnerID)
			targetWorkItemID, err = textureTurnRuntimeID(rec, in.ToolCallID, "persistent-super-work", i)
			if err != nil {
				return nil, "", err
			}
			if packet.Kind != "execution_request" || len(packet.Actions) == 0 {
				return nil, "", fmt.Errorf("Texture controls[%d] persistent-Management opener requires execution_request actions", i)
			}
			work := types.WorkItemRecord{
				WorkItemID: targetWorkItemID, Objective: strings.TrimSpace(raw.Objective),
				AuthorityProfile: agentprofile.Management, Status: types.WorkItemOpen,
				AssignedAgentID: targetAgentID,
			}
			openWork = &work
		} else if raw.OpenResearch {
			agentIdentity, identityErr := textureTurnRuntimeID(rec, in.ToolCallID, "researcher-agent", i)
			if identityErr != nil {
				return nil, "", identityErr
			}
			targetAgentID = agentprofile.Research + ":" + agentIdentity
			targetWorkItemID, err = textureTurnRuntimeID(rec, in.ToolCallID, "researcher-work", i)
			if err != nil {
				return nil, "", err
			}
			agent := types.AgentRecord{AgentID: targetAgentID, Profile: agentprofile.Research, Role: agentprofile.Research, ChannelID: doc.DocID}
			work := types.WorkItemRecord{
				WorkItemID: targetWorkItemID, Objective: strings.TrimSpace(raw.Objective),
				AuthorityProfile: agentprofile.Research, Status: types.WorkItemOpen,
				AssignedAgentID: targetAgentID,
				CreatedByRunID:  rec.RunID,
				Details: map[string]any{
					"requested_by_profile":  agentprofile.Texture,
					"requested_by_agent_id": rec.AgentID,
					"requested_by_run_id":   rec.RunID,
				},
			}
			openAgent, openWork = &agent, &work
		} else {
			work, ok := workByID[targetWorkItemID]
			if !ok || work.Status != types.WorkItemOpen || strings.TrimSpace(work.TrajectoryID) != strings.TrimSpace(doc.TrajectoryID) {
				return nil, "", fmt.Errorf("Texture controls[%d] target work is not an open current-trajectory obligation", i)
			}
			targetAgentID = strings.TrimSpace(work.AssignedAgentID)
			if targetAgentID == "" {
				return nil, "", fmt.Errorf("Texture controls[%d] target work is unassigned", i)
			}
		}
		// Runtime lookup is an early fail-closed refusal for existing targets; a
		// Research opener proves absence and creates its runtime-derived agent in
		// the same ApplyTextureTurn CAS as work and first control.
		if openAgent == nil {
			if _, err := h.Store.GetAgentByScope(ctx, rec.OwnerID, doc.ComputerID, targetAgentID); err != nil {
				return nil, "", fmt.Errorf("Texture controls[%d] load exact target: %w", i, err)
			}
		}
		content := agentcore.BuildTextureLifecycleControlContent(packet, targetAgentID, targetWorkItemID)
		payloadDigest, err := store.ComputeLifecycleUpdatePayloadDigest(packet, content)
		if err != nil {
			return nil, "", fmt.Errorf("Texture controls[%d] payload digest: %w", i, err)
		}
		controls = append(controls, types.TextureTurnControl{
			ControlID: controlID, TargetAgentID: targetAgentID, TargetWorkItemID: targetWorkItemID,
			OpenAgent: openAgent, OpenWork: openWork, Packet: packet, Content: content, PayloadDigest: payloadDigest,
		})
	}
	note := ""
	if dropped > 0 {
		note = fmt.Sprintf("[runtime: research budget for this owner request is spent (%d of %d); %d research opener(s) not opened]", textureResearchBudgetPerOwnerRequest, textureResearchBudgetPerOwnerRequest, dropped)
		log.Printf("textureowner: Texture %s run %s: %s", rec.AgentID, rec.RunID, note)
	}
	return controls, note, nil
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
	inbound, err := textureTurnPendingInbound(snapshot, rec, in.UpdateDispositions, resultRef)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	controls, budgetNote, err := h.textureTurnControls(ctx, rec, doc, snapshot, in)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	if budgetNote != "" {
		reason = strings.TrimSpace(reason + " " + budgetNote)
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
	req := types.ApplyTextureTurnRequest{
		OwnerID: rec.OwnerID, ComputerID: doc.ComputerID,
		CommandID: "texture-turn:" + commandUUID, DocumentID: doc.DocID, TrajectoryID: doc.TrajectoryID,
		CallerAgentID: rec.AgentID, CallerRunID: rec.RunID,
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
	in.DecisionKind = normalizeTextureDecisionKind(in.DecisionKind)
	if !validTextureDecisionKind(in.DecisionKind) {
		return types.LifecycleResult{}, fmt.Errorf("decision_kind must be one of delegation_opened, delegation_skipped, delegation_deferred, wait_for_evidence, blocker, no_worker_needed")
	}
	if strings.TrimSpace(in.Reason) == "" {
		return types.LifecycleResult{}, fmt.Errorf("decision reason must not be empty")
	}
	if err := validateTextureControls("texture_cell", in.Controls); err != nil {
		return types.LifecycleResult{}, err
	}
	computerID := strings.TrimSpace(rec.ComputerID)
	docID := strings.TrimSpace(in.DocID)
	if docID == "" {
		docID = strings.TrimSpace(firstNonEmpty(metadataStringValue(rec.Metadata, "doc_id"), rec.ChannelID))
	}
	if docID == "" || docID != strings.TrimSpace(metadataStringValue(rec.Metadata, "doc_id")) || rec.ChannelID != docID {
		return types.LifecycleResult{}, fmt.Errorf("texture cell decision does not match authenticated Texture document")
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
		return types.LifecycleResult{}, fmt.Errorf("texture cell decision requires exact lifecycle Texture caller")
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
	return result, nil
}
