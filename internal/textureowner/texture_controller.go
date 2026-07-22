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
	doc, err := rt.Store.GetDocument(ctx, docID, ownerID)
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
	doc, err := rt.Store.GetDocument(ctx, docID, ownerID)
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
	if _, found, err := rt.Core.TextureActiveRunByAgent(ctx, ownerID, textureAgentID); err != nil {
		return nil, fmt.Errorf("check resident Texture loop: %w", err)
	} else if found {
		return nil, nil
	}
	updates, err := rt.Store.ListPendingLifecycleUpdates(ctx, ownerID, doc.ComputerID, textureAgentID, 100)
	if err != nil {
		return nil, fmt.Errorf("list pending lifecycle Texture updates: %w", err)
	}
	if len(updates) == 0 {
		return nil, nil
	}
	if mutation, err := rt.Store.GetPendingAgentMutationByDoc(ctx, docID, ownerID); err == nil && mutation != nil {
		rt.scheduleTextureWorkerWake(ownerID, docID, "")
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("check pending doc mutation: %w", err)
	}
	var scheduledSeq int64
	for _, update := range updates {
		if update.MessageSeq > scheduledSeq {
			scheduledSeq = update.MessageSeq
		}
	}
	if rec, reactivated, err := rt.reactivatePassivatedTextureRun(ctx, doc, textureAgentID, scheduledSeq); err != nil {
		return nil, err
	} else if reactivated {
		return rec, nil
	}
	rec, err := rt.submitTextureAgentRevisionRun(ctx, doc, ownerID, textureAgentRevisionRequest{
		Intent: "integrate_execution_findings",
	}, scheduledSeq)
	if err != nil {
		return nil, fmt.Errorf("start reconciled Texture revision: %w", err)
	}
	return rec, nil
}

func (rt *Handler) reactivatePassivatedTextureRun(ctx context.Context, doc types.Document, textureAgentID string, scheduledSeq int64) (*types.RunRecord, bool, error) {
	if rt == nil || rt.Store == nil || scheduledSeq <= 0 {
		return nil, false, nil
	}
	ownerID := strings.TrimSpace(doc.OwnerID)
	docID := strings.TrimSpace(doc.DocID)
	textureAgentID = strings.TrimSpace(textureAgentID)
	if ownerID == "" || docID == "" || textureAgentID == "" {
		return nil, false, nil
	}
	rec, err := rt.Store.GetLatestPassivatedRunByAgent(ctx, ownerID, textureAgentID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("lookup passivated Texture run: %w", err)
	}
	if !isTextureAgentRevisionTaskType(metadataStringValue(rec.Metadata, "type")) ||
		strings.TrimSpace(metadataStringValue(rec.Metadata, "doc_id")) != docID {
		return nil, false, nil
	}
	if mutation, err := rt.Store.GetAgentMutationByRun(ctx, rec.RunID); err != nil {
		return nil, false, fmt.Errorf("lookup passivated Texture mutation: %w", err)
	} else if mutation != nil && mutation.State == "completed" {
		return nil, false, nil
	}
	if currentRevisionID := strings.TrimSpace(metadataStringValue(rec.Metadata, "current_revision_id")); currentRevisionID != "" && currentRevisionID != strings.TrimSpace(doc.CurrentRevisionID) {
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
	if err := rt.Store.ReactivateAgentMutation(ctx, rec.RunID, scheduledSeq); err != nil && !errors.Is(err, store.ErrMutationAlreadyCompleted) {
		return nil, false, err
	}
	if err := rt.Store.UpdateRun(ctx, rec); err != nil {
		return nil, false, fmt.Errorf("reactivate passivated Texture run: %w", err)
	}
	rt.Core.ActivateRun(&rec)
	return &rec, true, nil
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
			ok, err := rt.isEligibleWorkerMessage(ctx, channelID, message, cache)
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

func (rt *Handler) isEligibleWorkerMessage(ctx context.Context, docID string, message types.ChannelMessage, cache map[string]bool) (bool, error) {
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
	run, err := rt.Store.GetRun(ctx, runID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			cache[runID] = false
			return false, nil
		}
		return false, err
	}
	switch agentProfileForRun(&run) {
	case agentprofile.Researcher, agentprofile.Super, agentprofile.CoSuper:
		cache[runID] = true
		return true, nil
	default:
		cache[runID] = false
		return false, nil
	}
}
