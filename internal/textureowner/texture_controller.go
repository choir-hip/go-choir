package textureowner

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// scheduleTextureWorkerWake sends an actor message to the Texture agent for
// the given doc. The actor mailbox replaces the old debounce timer system —
// the handler processes the message when the actor activates, and the tool
// loop's park-resume handles coalescing naturally.
func (rt *Handler) scheduleTextureWorkerWake(ownerID, docID, _ string) {
	ownerID = strings.TrimSpace(ownerID)
	docID = strings.TrimSpace(docID)
	if ownerID == "" || docID == "" {
		return
	}
	textureAgentID := currentTextureAgentID(docID)
	if rt.Core == nil || !rt.Core.DispatchActorActive() {
		return
	}
	if err := rt.Core.DispatchActor(context.Background(), ownerID, rt.Core.TextureSandboxID(), textureAgentID, "coagent_result", "", "", ""); err != nil {
		log.Printf("runtime: schedule texture wake for doc %s: %v", docID, err)
	}
}

// Start reconciles durable Texture documents after the generic core has
// recovered interrupted activations and before the actor mailbox boot sweep.
func (rt *Handler) Start(ctx context.Context) {
	subjects, err := rt.Store.ListLifecycleSubjects(ctx, rt.Core.TextureSandboxID())
	if err != nil {
		log.Printf("runtime: reconcile lifecycle Texture subjects: %v", err)
		return
	}
	for _, subject := range subjects {
		if agentprofile.Canonical(subject.Profile) != agentprofile.Texture {
			continue
		}
		if _, err := rt.ReconcileActorWake(ctx, subject.OwnerID, subject.ComputerID, subject.AgentID); err != nil {
			log.Printf("runtime: reconcile subject %s/%s/%s: %v", subject.OwnerID, subject.ComputerID, subject.AgentID, err)
		}
	}
}

// ReconcileActorWake resolves a Texture actor from canonical document state,
// persists its durable identity when first seen, and reconciles its mailbox.
// This path does not depend on a pre-existing generic agents row.
func (rt *Handler) ReconcileActorWake(ctx context.Context, ownerID, computerID, agentID string) (*types.RunRecord, error) {
	ownerID, computerID, agentID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID), strings.TrimSpace(agentID)
	if rt == nil || rt.Store == nil || rt.Core == nil || ownerID == "" || computerID == "" || agentID == "" {
		return nil, fmt.Errorf("resolve Texture actor wake: incomplete scoped owner state")
	}
	docID := docIDFromTextureAgentID(agentID)
	if docID == "" {
		return nil, fmt.Errorf("resolve Texture actor wake: invalid Texture agent id")
	}
	doc, err := rt.getTextureDocument(ctx, ownerID, docID)
	if err != nil {
		return nil, fmt.Errorf("resolve Texture actor wake: document not found: %w", err)
	}
	if doc.ComputerID != computerID || strings.TrimSpace(doc.TrajectoryID) == "" {
		return nil, fmt.Errorf("resolve Texture actor wake: document lifecycle binding conflict")
	}
	if _, err := rt.Store.GetAgentByScope(ctx, ownerID, computerID, agentID); err != nil {
		return nil, fmt.Errorf("resolve Texture actor wake: durable subject unavailable: %w", err)
	}
	return rt.ReconcileAgentWake(ctx, ownerID, doc.DocID)
}

// ValidateActivationAuthority proves that an initial Texture dispatch is bound
// to the canonical document head and a pending scoped mutation.
func (rt *Handler) ValidateActivationAuthority(ctx context.Context, ownerID, computerID, agentID, runID string) error {
	ownerID, computerID, agentID, runID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID), strings.TrimSpace(agentID), strings.TrimSpace(runID)
	docID := docIDFromTextureAgentID(agentID)
	if rt == nil || rt.Store == nil || ownerID == "" || computerID == "" || docID == "" || runID == "" {
		return fmt.Errorf("validate Texture activation: incomplete scoped authority")
	}
	doc, err := rt.getTextureDocument(ctx, ownerID, docID)
	if err != nil {
		return fmt.Errorf("validate Texture activation document: %w", err)
	}
	if strings.TrimSpace(doc.ComputerID) != computerID || strings.TrimSpace(doc.CurrentRevisionID) == "" {
		return fmt.Errorf("validate Texture activation: document authority mismatch")
	}
	revision, err := rt.getTextureRevision(ctx, ownerID, doc.CurrentRevisionID)
	if err != nil {
		return fmt.Errorf("validate Texture activation revision: %w", err)
	}
	if !textureRevisionMatchesDocument(revision, doc, ownerID) {
		return fmt.Errorf("validate Texture activation: revision authority mismatch")
	}
	run, err := rt.Store.GetRunByOwner(ctx, ownerID, runID)
	if err != nil {
		return fmt.Errorf("validate Texture activation run: %w", err)
	}
	if strings.TrimSpace(run.OwnerID) != ownerID ||
		strings.TrimSpace(run.SandboxID) != computerID ||
		strings.TrimSpace(run.AgentID) != agentID ||
		!isTextureAgentRevisionTaskType(metadataStringValue(run.Metadata, "type")) ||
		strings.TrimSpace(metadataStringValue(run.Metadata, "doc_id")) != docID ||
		strings.TrimSpace(metadataStringValue(run.Metadata, "current_revision_id")) != strings.TrimSpace(doc.CurrentRevisionID) {
		return fmt.Errorf("validate Texture activation: run authority mismatch")
	}
	if metadataStringValue(run.Metadata, "request_source") == "update_coagent" {
		trajectoryID := strings.TrimSpace(doc.TrajectoryID)
		if trajectoryID == "" ||
			metadataStringValue(run.Metadata, "trajectory_id") != trajectoryID ||
			metadataStringValue(run.Metadata, "lifecycle_work_item_id") == "" {
			return fmt.Errorf("validate Texture activation: supervision scope mismatch")
		}
		deliveryIDs := metadataStringSlice(run.Metadata["supervision_delivery_ids"])
		if len(deliveryIDs) == 0 {
			return fmt.Errorf("validate Texture activation: supervision delivery binding missing")
		}
		var pending []types.CoagentSourcePacket
		var err error
		switch metadataStringValue(run.Metadata, "supervision_delivery_authority") {
		case "canonical":
			pending, err = rt.Core.ListPendingCanonicalSupervisionUpdates(ctx, ownerID, computerID, agentID, trajectoryID, 0)
		case "compatibility":
			pending, err = rt.Core.ListPendingSupervisionCompatibilityUpdates(ctx, ownerID, computerID, agentID, trajectoryID, 0)
		default:
			return fmt.Errorf("validate Texture activation: supervision delivery authority missing")
		}
		if err != nil {
			return fmt.Errorf("validate Texture activation delivery: %w", err)
		}
		pendingIDs := make(map[string]bool, len(pending))
		for _, update := range pending {
			pendingIDs[strings.TrimSpace(update.UpdateID)] = true
		}
		for _, deliveryID := range deliveryIDs {
			if !pendingIDs[strings.TrimSpace(deliveryID)] {
				return fmt.Errorf("validate Texture activation: supervision delivery %q is no longer pending", deliveryID)
			}
		}
		return nil
	}
	mutation, err := rt.Store.GetAgentMutationByRun(ctx, ownerID, computerID, runID)
	if err != nil {
		return fmt.Errorf("validate Texture activation mutation: %w", err)
	}
	if mutation == nil ||
		strings.TrimSpace(mutation.DocID) != docID ||
		strings.TrimSpace(mutation.RunID) != runID ||
		strings.TrimSpace(mutation.OwnerID) != ownerID ||
		strings.TrimSpace(mutation.ComputerID) != computerID ||
		(strings.TrimSpace(mutation.RevisionID) != "" &&
			strings.TrimSpace(mutation.RevisionID) != strings.TrimSpace(doc.CurrentRevisionID)) ||
		mutation.State != "pending" {
		return fmt.Errorf("validate Texture activation: mutation authority mismatch")
	}
	return nil
}

// ReconcileAgentWake starts or reuses a Texture activation when pending
// update_coagent records are addressed to texture:<docID>. Delivery uses the
// same typed coagent update packets as other actors; integrate intent only
// selects the Texture revision run shape.
func (rt *Handler) ReconcileAgentWake(ctx context.Context, ownerID, docID string) (*types.RunRecord, error) {
	ownerID = strings.TrimSpace(ownerID)
	docID = strings.TrimSpace(docID)
	if ownerID == "" || docID == "" {
		return nil, nil
	}
	textureAgentID := currentTextureAgentID(docID)
	doc, err := rt.getTextureDocument(ctx, ownerID, docID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("load doc for texture wake: %w", err)
	}
	if strings.TrimSpace(doc.ComputerID) == "" || strings.TrimSpace(doc.TrajectoryID) == "" {
		return nil, fmt.Errorf("texture wake requires durable lifecycle document binding")
	}
	if _, err := rt.Store.GetAgentByScope(ctx, ownerID, doc.ComputerID, textureAgentID); err != nil {
		return nil, fmt.Errorf("load durable Texture subject: %w", err)
	}
	active, found, err := rt.Core.TextureActiveRunByAgent(ctx, ownerID, doc.ComputerID, textureAgentID)
	if err != nil {
		return nil, fmt.Errorf("check resident Texture loop: %w", err)
	}
	if found {
		if authorityErr := rt.ValidateActivationAuthority(ctx, ownerID, doc.ComputerID, textureAgentID, active.RunID); authorityErr == nil {
			return nil, nil
		}
		if err := rt.refuseLegacyTextureWriter("replace Texture lifecycle activation"); err != nil {
			return nil, err
		}
	}
	canonical, err := rt.Core.ListPendingCanonicalSupervisionUpdates(ctx, ownerID, doc.ComputerID, textureAgentID, doc.TrajectoryID, 0)
	if err != nil {
		return nil, fmt.Errorf("list canonical Texture updates: %w", err)
	}
	updates := canonical
	if len(updates) == 0 {
		// Existing pre-cutover lifecycle mailboxes remain a read-only
		// compatibility floor only after canonical authority proves emptiness.
		updates, err = rt.Core.ListPendingSupervisionCompatibilityUpdates(ctx, ownerID, doc.ComputerID, textureAgentID, doc.TrajectoryID, 0)
		if err != nil {
			return nil, fmt.Errorf("list compatibility Texture updates: %w", err)
		}
	}
	var scheduledSeq int64
	for _, update := range updates {
		if update.ReducerSeq > scheduledSeq {
			scheduledSeq = update.ReducerSeq
		}
		if update.MessageSeq > scheduledSeq {
			scheduledSeq = update.MessageSeq
		}
	}
	if rec, reactivated, err := rt.reactivatePassivatedTextureRun(ctx, doc, textureAgentID, scheduledSeq); err != nil {
		return nil, err
	} else if reactivated {
		return rec, nil
	}
	if len(updates) == 0 {
		return nil, nil
	}
	pendingCleanupCtx := context.WithoutCancel(ctx)
	for {
		mutation, mutationErr := rt.Store.GetPendingAgentMutationByDoc(pendingCleanupCtx, ownerID, doc.ComputerID, docID)
		if mutationErr != nil {
			return nil, fmt.Errorf("check pending doc mutation: %w", mutationErr)
		}
		if mutation == nil {
			break
		}
		if staleErr := rt.Store.MarkAgentMutationStale(pendingCleanupCtx, ownerID, doc.ComputerID, mutation.RunID); staleErr != nil {
			return nil, fmt.Errorf("stale unbound pending Texture mutation: %w", staleErr)
		}
	}
	deliveryIDs := make([]string, 0, len(updates))
	for _, update := range updates {
		if id := strings.TrimSpace(update.UpdateID); id != "" {
			deliveryIDs = append(deliveryIDs, id)
		}
	}
	deliveryAuthority := "compatibility"
	if len(canonical) > 0 {
		deliveryAuthority = "canonical"
	}
	rec, err := rt.submitTextureAgentRevisionRun(ctx, doc, ownerID, textureAgentRevisionRequest{
		Intent: "integrate_execution_findings",
		Provenance: map[string]any{
			"supervision_delivery_authority": deliveryAuthority,
			"supervision_delivery_ids":       deliveryIDs,
		},
	}, scheduledSeq)
	if err != nil {
		return nil, fmt.Errorf("start reconciled Texture revision: %w", err)
	}
	return rec, nil
}

func (rt *Handler) reactivatePassivatedTextureRun(ctx context.Context, doc types.Document, textureAgentID string, scheduledSeq int64) (*types.RunRecord, bool, error) {
	if rt == nil || rt.Store == nil {
		return nil, false, nil
	}
	ownerID := strings.TrimSpace(doc.OwnerID)
	docID := strings.TrimSpace(doc.DocID)
	textureAgentID = strings.TrimSpace(textureAgentID)
	if ownerID == "" || docID == "" || textureAgentID == "" {
		return nil, false, nil
	}
	runs, err := rt.Store.ListLifecycleRunsByChannel(ctx, ownerID, doc.ComputerID, docID, 0)
	if err != nil {
		return nil, false, fmt.Errorf("list passivated Texture runs: %w", err)
	}
	var rec *types.RunRecord
	for i := range runs {
		candidate := &runs[i]
		if candidate.State != types.RunPassivated ||
			strings.TrimSpace(candidate.AgentID) != textureAgentID ||
			!isTextureAgentRevisionTaskType(metadataStringValue(candidate.Metadata, "type")) ||
			strings.TrimSpace(metadataStringValue(candidate.Metadata, "doc_id")) != docID {
			continue
		}
		mutation, mutationErr := rt.Store.GetAgentMutationByRun(ctx, ownerID, doc.ComputerID, candidate.RunID)
		if mutationErr != nil {
			return nil, false, fmt.Errorf("lookup passivated Texture mutation: %w", mutationErr)
		}
		if mutation == nil ||
			strings.TrimSpace(mutation.DocID) != docID ||
			strings.TrimSpace(mutation.RunID) != strings.TrimSpace(candidate.RunID) ||
			strings.TrimSpace(mutation.OwnerID) != ownerID ||
			strings.TrimSpace(mutation.ComputerID) != strings.TrimSpace(doc.ComputerID) {
			continue
		}
		documentRevisionID := strings.TrimSpace(doc.CurrentRevisionID)
		runRevisionID := strings.TrimSpace(metadataStringValue(candidate.Metadata, "current_revision_id"))
		mutationRevisionID := strings.TrimSpace(mutation.RevisionID)
		if documentRevisionID == "" {
			continue
		}
		if mutationRevisionID != "" {
			if mutationRevisionID != documentRevisionID {
				continue
			}
		} else if runRevisionID != documentRevisionID {
			continue
		}
		if mutation.State == "pending" {
			if staleErr := rt.Store.MarkAgentMutationStale(ctx, ownerID, doc.ComputerID, candidate.RunID); staleErr != nil {
				return nil, false, fmt.Errorf("repair passivated Texture mutation authority: %w", staleErr)
			}
			mutation, mutationErr = rt.Store.GetAgentMutationByRun(ctx, ownerID, doc.ComputerID, candidate.RunID)
			if mutationErr != nil {
				return nil, false, fmt.Errorf("reload repaired Texture mutation authority: %w", mutationErr)
			}
		}
		if mutation == nil || (mutation.State != "stale_activation" && (mutation.State != "sleeping" || scheduledSeq <= 0)) {
			continue
		}
		rec = candidate
		break
	}
	if rec == nil {
		return nil, false, nil
	}
	rec.Metadata = cloneMetadata(rec.Metadata)
	rec.Metadata["request_source"] = "update_coagent"
	rec.Metadata["request_intent"] = "integrate_execution_findings"
	rec.Metadata["scheduled_message_seq"] = scheduledSeq
	rec.Metadata["actor_reactivate_existing_memory"] = true
	rec.Metadata["actor_reactivated_from_passivated"] = true
	rec.Metadata["actor_resume_source_loop_id"] = rec.RunID
	rec.Metadata["current_revision_id"] = strings.TrimSpace(doc.CurrentRevisionID)
	if spend, ok, err := rt.Core.LatestTextureActorToolLoopBudgetSpend(ctx, ownerID, textureAgentID); err != nil {
		return nil, false, fmt.Errorf("load passivated Texture budget spend: %w", err)
	} else if ok {
		rec.Metadata["actor_budget_spent_provider_calls"] = spend.ProviderCalls
		rec.Metadata["actor_budget_spent_input_tokens"] = spend.InputTokens
		rec.Metadata["actor_budget_spent_output_tokens"] = spend.OutputTokens
		if spend.SourceRunID != "" {
			rec.Metadata["actor_resume_source_loop_id"] = spend.SourceRunID
		}
	}
	rec.State = types.RunPending
	rec.Error = ""
	rec.Result = ""
	rec.FinishedAt = nil
	rec.UpdatedAt = time.Now().UTC()
	if err := rt.refuseLegacyTextureWriter("reactivate Texture lifecycle run"); err != nil {
		return nil, false, err
	}
	rt.Core.ActivateRun(rec)
	return rec, true, nil
}

func (rt *Handler) latestEligibleWorkerMessage(ctx context.Context, ownerID, channelID string, afterSeq int64) (types.ChannelMessage, bool, error) {
	const batchSize = 200
	cache := make(map[string]bool)
	cursor := afterSeq
	var latest types.ChannelMessage
	found := false
	for {
		messages, err := rt.Store.ListChannelMessages(ctx, ownerID, channelID, cursor, batchSize)
		if err != nil {
			return types.ChannelMessage{}, false, err
		}
		if len(messages) == 0 {
			break
		}
		for _, message := range messages {
			if message.Seq > cursor {
				cursor = message.Seq
			}
			ok, err := rt.isEligibleWorkerMessage(ctx, ownerID, channelID, message, cache)
			if err != nil {
				return types.ChannelMessage{}, false, err
			}
			if !ok {
				continue
			}
			latest = message
			found = true
		}
		if len(messages) < batchSize {
			break
		}
	}
	return latest, found, nil
}

func (rt *Handler) isEligibleWorkerMessage(ctx context.Context, ownerID, docID string, message types.ChannelMessage, cache map[string]bool) (bool, error) {
	if strings.TrimSpace(message.ToAgentID) != "texture:"+strings.TrimSpace(docID) {
		return false, nil
	}
	runID := strings.TrimSpace(message.FromRunID)
	if runID == "" {
		return false, nil
	}
	if cached, ok := cache[runID]; ok {
		return cached, nil
	}
	run, err := rt.Core.GetRun(ctx, runID, ownerID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			cache[runID] = false
			return false, nil
		}
		return false, err
	}
	switch agentProfileForRun(run) {
	case agentprofile.Researcher, agentprofile.Super, agentprofile.CoSuper:
		cache[runID] = true
		return true, nil
	default:
		cache[runID] = false
		return false, nil
	}
}
