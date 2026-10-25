package textureowner

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/google/uuid"

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
		if !update.WithinTextureTurnBoundary(rec.RunID, scheduledSeq) {
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

// textureTurnArgs is what a Texture turn commits besides its outcome: the
// leftover update packets it rejects, the dispatch fold (records and the
// cursor advance), the caller work disposition, the per-cell identity and,
// for a revision, its citation pairs (listed to management by the turn).
type textureTurnArgs struct {
	UpdateDispositions []textureUpdateDisposition
	InboxRecords       []types.CommitmentRecord
	InboxCursor        *types.InboxCursorAdvance
	WorkDisposition    types.WorkItemStatus
	ToolCallID         string
	Citations          []types.CitationPair
}

// applyTextureLifecycleTurn commits one Texture turn through the atomic
// ApplyTextureTurn transaction: its outcome (a revision or none), the
// records it sends, the cursor advance and the caller's work disposition.
func (h *Handler) applyTextureLifecycleTurn(ctx context.Context, rec *types.RunRecord, doc types.Document, in textureTurnArgs, outcome types.TextureTurnOutcome, revision types.Revision, graph store.TextureSourceGraphWriteSet, reason string) (types.LifecycleResult, error) {
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
	commandUUID, err := textureTurnRuntimeID(rec, in.ToolCallID, "command", 0)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	callerWorkItemID := strings.TrimSpace(metadataStringValue(rec.Metadata, "lifecycle_work_item_id"))
	if callerWorkItemID != "" && metadataBoolValue(rec.Metadata, textureRecordsRouteMetadata) {
		// A records-route turn settles its work item only while it is open
		// (B5); an earlier turn may have completed it.
		open := false
		for _, work := range snapshot.WorkItems {
			if work.WorkItemID == callerWorkItemID && work.Status == types.WorkItemOpen {
				open = true
				break
			}
		}
		if !open {
			callerWorkItemID = ""
		}
	}
	callerWorkDisposition := in.WorkDisposition
	if callerWorkDisposition == "" || callerWorkItemID == "" {
		// A turn on records has no work item to settle (Cut 1).
		callerWorkDisposition = types.WorkItemOpen
	}
	// The Texture root's turns leave its standing item open (PD3 S1): the
	// item keeps the master's trajectory live, so a root that calls Done
	// does not settle it.
	_, isTextureRoot := h.textureRootFor(ctx, rec.OwnerID, doc.ComputerID, doc.DocID)
	if isTextureRoot {
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
		Outcome:                        outcome, Revision: revision, Reason: strings.TrimSpace(reason), Inbound: inbound,
		InboxRecords: in.InboxRecords, InboxCursor: in.InboxCursor, Citations: in.Citations,
	}
	req.CommandDigest, err = store.ComputeApplyTextureTurnWithSourceGraphDigest(req, graph)
	if err != nil {
		return types.LifecycleResult{}, fmt.Errorf("digest Texture lifecycle turn: %w", err)
	}
	result, err := h.Store.ApplyTextureTurnWithSourceGraph(ctx, req, graph)
	// After the transaction: a turn that changed another document wakes the
	// Texture root through one coalesced digest (PD3 S1). The root's own
	// turns never do.
	if err == nil && !result.Replay && !isTextureRoot &&
		(result.Revision != nil || len(in.InboxRecords) > 0 || len(in.UpdateDispositions) > 0 || callerWorkDisposition != types.WorkItemOpen) {
		h.NoteTextureRootChange(ctx, rec.OwnerID, doc.ComputerID, doc.DocID)
	}
	return result, err
}
