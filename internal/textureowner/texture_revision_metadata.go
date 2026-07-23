package textureowner

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/wirepublish"
)

// buildAppagentRevisionMetadata constructs the metadata JSON for an
// appagent-authored revision, carrying forward durable context keys
// from the parent revision so they remain available on the next revise.
func (rt *Handler) buildAppagentRevisionMetadata(ctx context.Context, rec *types.RunRecord, doc types.Document, ownerID string, mutation *store.AgentMutation, consumedThroughSeq int64) json.RawMessage {
	meta := map[string]any{
		"source":  "patch_texture",
		"loop_id": rec.RunID,
	}

	// Carry forward durable keys from the parent revision metadata.
	if doc.CurrentRevisionID != "" {
		if parentRev, err := rt.getTextureRevision(context.Background(), ownerID, doc.CurrentRevisionID); err == nil {
			parentMeta := decodeRevisionMetadata(parentRev.Metadata)
			for _, key := range durableMetadataKeys {
				if val, ok := parentMeta[key]; ok && hasNonEmptyTextureMetadataValue(val) {
					meta[key] = val
				}
			}
			promoteCanonicalTextureSourcePath(meta, parentMeta)
		}
	}

	// Also carry forward from run metadata (the initial agent revision
	// request sets these directly).
	if rec.Metadata != nil {
		for _, key := range durableMetadataKeys {
			if val, ok := rec.Metadata[key]; ok && hasNonEmptyTextureMetadataValue(val) {
				// Run metadata takes precedence over parent revision.
				meta[key] = val
			}
		}
		if val, ok := canonicalTextureSourcePathMetadataValue(rec.Metadata); ok {
			meta[canonicalTextureSourcePathMetadataKey] = val
		}
		if requestedByRunID := metadataStringValue(rec.Metadata, "requested_by_run_id"); requestedByRunID != "" {
			meta["requested_by_run_id"] = requestedByRunID
		}
	}
	promptOnlyInitialModelPrior := promptOnlyInitialModelPriorTextureRevision(rec, meta, consumedThroughSeq)
	if wirepublish.IsWireArticleRevisionRun(rec) && !promptOnlyInitialModelPrior {
		meta["artifact_kind"] = "article_revision"
		meta["revision_role"] = textureRevisionRoleCanonical
		meta["texture_version_stage"] = "article_revision"
	}
	workerUpdateMeta := rt.workerUpdateRevisionMetadata(ctx, ownerID, doc.DocID, mutation, consumedThroughSeq)
	if promptOnlyInitialModelPrior {
		meta["grounding_status"] = "model_prior_interim"
		meta["revision_grounding"] = "model_prior"
		meta["texture_version_stage"] = "interim"
		meta["model_prior_interim"] = true
		if metadataStringValue(meta, "artifact_kind") == "article_revision" {
			meta["artifact_kind"] = "working_revision"
		}
		if metadataStringValue(meta, "revision_role") == textureRevisionRoleCanonical {
			meta["revision_role"] = textureRevisionRoleInput
		}
	}
	for key, value := range workerUpdateMeta {
		meta[key] = value
	}
	// Available source entities are run-time prompt context, not durable revision metadata.
	// Keep them out of the persisted revision so they do not leak into the next run's parent
	// revision projection and are recomputed from the actual revision's source_entities.
	delete(meta, textureAvailableSourceEntitiesKey)

	data, err := json.Marshal(meta)
	if err != nil {
		return json.RawMessage(`{"source":"patch_texture","loop_id":"` + rec.RunID + `"}`)
	}
	return data
}

func promptOnlyInitialModelPriorTextureRevision(rec *types.RunRecord, meta map[string]any, consumedThroughSeq int64) bool {
	if rec == nil || agentProfileForRun(rec) != agentprofile.Texture {
		return false
	}
	if consumedThroughSeq != 0 || metadataIntValue(rec.Metadata, "scheduled_message_seq") != 0 {
		return false
	}
	if metadataStringValue(rec.Metadata, "request_source") == "update_coagent" {
		return false
	}
	inputOrigin := firstNonEmpty(
		metadataStringValue(meta, "input_origin"),
		metadataStringValue(rec.Metadata, "input_origin"),
	)
	if inputOrigin == textureInputOriginUserPrompt {
		return true
	}
	if strings.TrimSpace(metadataStringValue(rec.Metadata, "request_intent")) == "initial_conductor_workflow" &&
		strings.TrimSpace(metadataStringValue(rec.Metadata, "seed_prompt")) != "" {
		return true
	}
	return false
}

func textureWorkerUpdateMetadataHasRole(value any, role string) bool {
	role = strings.TrimSpace(role)
	if role == "" {
		return false
	}
	switch updates := value.(type) {
	case []textureWorkerUpdateMetadata:
		for _, update := range updates {
			if strings.TrimSpace(update.Role) == role {
				return true
			}
		}
	case []any:
		for _, raw := range updates {
			item, _ := raw.(map[string]any)
			if strings.TrimSpace(fmt.Sprint(item["role"])) == role {
				return true
			}
		}
	}
	return false
}

type textureWorkerUpdateMetadata struct {
	ChannelID      string `json:"channel_id"`
	Seq            int64  `json:"seq"`
	FromAgentID    string `json:"from_agent_id,omitempty"`
	FromLoopID     string `json:"from_loop_id,omitempty"`
	Role           string `json:"role,omitempty"`
	ContentPreview string `json:"content_preview,omitempty"`
	Reason         string `json:"reason,omitempty"`
}

func (rt *Handler) workerUpdateRevisionMetadata(ctx context.Context, ownerID, docID string, mutation *store.AgentMutation, consumedThroughSeq int64) map[string]any {
	out := map[string]any{
		"worker_updates_policy":         "eligible_addressed_channel_messages",
		"worker_updates_checkpoint_seq": int64(0),
		"worker_updates_scheduled_seq":  int64(0),
		"worker_updates_consumed":       []textureWorkerUpdateMetadata{},
		"worker_updates_skipped":        []textureWorkerUpdateMetadata{},
		"worker_updates_pending":        []textureWorkerUpdateMetadata{},
	}
	if strings.TrimSpace(ownerID) == "" || strings.TrimSpace(docID) == "" {
		return out
	}

	scheduledSeq := int64(0)
	if mutation != nil {
		scheduledSeq = mutation.ScheduledMessageSeq
	}
	if consumedThroughSeq > scheduledSeq {
		scheduledSeq = consumedThroughSeq
	}
	out["worker_updates_scheduled_seq"] = scheduledSeq

	checkpointSeq := int64(0)
	checkpoint, err := rt.Store.GetTextureControllerCheckpoint(ctx, docID, ownerID)
	if err != nil {
		log.Printf("runtime: load texture worker update checkpoint for metadata: %v", err)
		return out
	}
	if checkpoint != nil {
		checkpointSeq = checkpoint.IntegratedMessageSeq
	}
	out["worker_updates_checkpoint_seq"] = checkpointSeq

	messageAfterSeq := checkpointSeq
	if scheduledSeq > 0 && checkpointSeq >= scheduledSeq {
		if previousSeq := rt.previousTextureWorkerMetadataSeq(ctx, ownerID, docID); previousSeq < scheduledSeq {
			messageAfterSeq = previousSeq
		}
	}

	messages, err := rt.Store.ListChannelMessages(ctx, ownerID, docID, messageAfterSeq, 500)
	if err != nil {
		log.Printf("runtime: load texture worker update messages for metadata: %v", err)
		return out
	}

	cache := make(map[string]bool)
	consumed := []textureWorkerUpdateMetadata{}
	skipped := []textureWorkerUpdateMetadata{}
	pending := []textureWorkerUpdateMetadata{}
	for _, message := range messages {
		if !textureAgentIDMatchesDoc(message.ToAgentID, docID) {
			continue
		}
		eligible, err := rt.isEligibleWorkerMessage(ctx, ownerID, docID, message, cache)
		if err != nil {
			log.Printf("runtime: classify texture worker update for metadata: %v", err)
			continue
		}
		if scheduledSeq > 0 && message.Seq <= scheduledSeq {
			if eligible {
				consumed = append(consumed, summarizeWorkerUpdateForMetadata(message, ""))
			} else {
				skipped = append(skipped, summarizeWorkerUpdateForMetadata(message, "ineligible_sender"))
			}
			continue
		}
		if eligible {
			pending = append(pending, summarizeWorkerUpdateForMetadata(message, "after_scheduled_checkpoint"))
		} else if scheduledSeq > 0 && message.Seq <= scheduledSeq {
			skipped = append(skipped, summarizeWorkerUpdateForMetadata(message, "ineligible_sender"))
		}
	}

	out["worker_updates_consumed"] = consumed
	out["worker_updates_skipped"] = skipped
	out["worker_updates_pending"] = pending
	return out
}

func (rt *Handler) textureWorkerUpdateCommitSeq(ctx context.Context, rec *types.RunRecord, docID string, mutation *store.AgentMutation) int64 {
	seq := int64(0)
	if mutation != nil {
		seq = mutation.ScheduledMessageSeq
	}
	if rt == nil || rt.Store == nil || rec == nil {
		return seq
	}
	if seq == 0 {
		seq = int64(metadataIntValue(rec.Metadata, "scheduled_message_seq"))
	}
	ownerID := strings.TrimSpace(rec.OwnerID)
	targetAgentID := currentTextureAgentID(docID)
	if ownerID == "" || strings.TrimSpace(targetAgentID) == "" {
		return seq
	}
	for _, updateID := range metadataStringSlice(rec.Metadata["worker_update_ids"]) {
		updateID = strings.TrimSpace(updateID)
		if updateID == "" {
			continue
		}
		update, err := rt.Store.GetWorkerUpdate(ctx, ownerID, updateID)
		if err != nil {
			log.Printf("runtime: load injected texture worker update %s for revision metadata: %v", updateID, err)
			continue
		}
		if strings.TrimSpace(update.TargetAgentID) != targetAgentID || strings.TrimSpace(update.ChannelID) != strings.TrimSpace(docID) {
			continue
		}
		if update.MessageSeq > seq {
			seq = update.MessageSeq
		}
	}
	return seq
}

func (rt *Handler) previousTextureWorkerMetadataSeq(ctx context.Context, ownerID, docID string) int64 {
	if rt == nil || rt.Store == nil {
		return 0
	}
	doc, err := rt.getTextureDocument(ctx, ownerID, docID)
	if err != nil || strings.TrimSpace(doc.CurrentRevisionID) == "" {
		return 0
	}
	rev, err := rt.getTextureRevision(ctx, ownerID, doc.CurrentRevisionID)
	if err != nil {
		return 0
	}
	meta := decodeRevisionMetadata(rev.Metadata)
	return maxTextureWorkerMetadataSeq(meta)
}

func maxTextureWorkerMetadataSeq(meta map[string]any) int64 {
	maxSeq := int64(metadataIntValue(meta, "worker_updates_scheduled_seq"))
	if checkpointSeq := int64(metadataIntValue(meta, "worker_updates_checkpoint_seq")); checkpointSeq > maxSeq {
		maxSeq = checkpointSeq
	}
	for _, key := range []string{"worker_updates_consumed", "worker_updates_skipped"} {
		for _, item := range metadataArray(meta[key]) {
			itemMeta, _ := item.(map[string]any)
			if seq := int64(metadataIntValue(itemMeta, "seq")); seq > maxSeq {
				maxSeq = seq
			}
		}
	}
	return maxSeq
}

func metadataArray(value any) []any {
	switch items := value.(type) {
	case []any:
		return items
	case []textureWorkerUpdateMetadata:
		out := make([]any, 0, len(items))
		for _, item := range items {
			out = append(out, map[string]any{
				"seq": item.Seq,
			})
		}
		return out
	default:
		return nil
	}
}

func summarizeWorkerUpdateForMetadata(message types.ChannelMessage, reason string) textureWorkerUpdateMetadata {
	return textureWorkerUpdateMetadata{
		ChannelID:      message.ChannelID,
		Seq:            message.Seq,
		FromAgentID:    strings.TrimSpace(message.FromAgentID),
		FromLoopID:     strings.TrimSpace(message.FromRunID),
		Role:           strings.TrimSpace(message.Role),
		ContentPreview: truncatePromptSnippet(message.Content, 240),
		Reason:         strings.TrimSpace(reason),
	}
}

func (rt *Handler) markTextureWorkerUpdatesDelivered(ctx context.Context, rec *types.RunRecord, docID string, maxSeq int64) error {
	if rt == nil || rt.Store == nil || rec == nil || maxSeq <= 0 {
		return nil
	}
	ownerID := strings.TrimSpace(rec.OwnerID)
	docID = strings.TrimSpace(docID)
	if ownerID == "" || docID == "" {
		return nil
	}
	targetAgentIDs := []string{currentTextureAgentID(docID)}
	updates := make([]types.CoagentSourcePacket, 0)
	seenUpdates := make(map[string]bool)
	targetByUpdateID := make(map[string]string)
	for _, targetAgentID := range targetAgentIDs {
		if targetAgentID == "" {
			continue
		}
		targetUpdates, err := rt.Store.ListCoagentMailboxBacklog(ctx, ownerID, targetAgentID, 500)
		if err != nil {
			return err
		}
		for _, update := range targetUpdates {
			if seenUpdates[update.UpdateID] {
				continue
			}
			seenUpdates[update.UpdateID] = true
			targetByUpdateID[update.UpdateID] = targetAgentID
			updates = append(updates, update)
		}
	}
	updateIDsByTarget := make(map[string][]string)
	for _, update := range updates {
		if strings.TrimSpace(update.ChannelID) == docID && update.MessageSeq > 0 && update.MessageSeq <= maxSeq {
			targetAgentID := targetByUpdateID[update.UpdateID]
			updateIDsByTarget[targetAgentID] = append(updateIDsByTarget[targetAgentID], update.UpdateID)
		}
	}
	for targetAgentID, updateIDs := range updateIDsByTarget {
		if len(updateIDs) == 0 {
			continue
		}
		if err := rt.Store.MarkWorkerUpdatesDelivered(ctx, ownerID, targetAgentID, updateIDs, rec.RunID); err != nil {
			return err
		}
	}
	if err := rt.advanceTextureControllerCheckpoint(ctx, ownerID, docID, maxSeq); err != nil {
		return err
	}
	return nil
}

func (rt *Handler) markTextureRevisionRunUpdatesDelivered(ctx context.Context, rec *types.RunRecord) error {
	if rt == nil || rt.Store == nil || rec == nil || agentProfileForRun(rec) != agentprofile.Texture {
		return nil
	}
	docID := firstNonEmpty(metadataStringValue(rec.Metadata, "doc_id"), rec.ChannelID)
	if strings.TrimSpace(docID) == "" {
		return nil
	}
	consumedThroughSeq := rt.textureWorkerUpdateCommitSeq(ctx, rec, docID, nil)
	if consumedThroughSeq <= 0 {
		return nil
	}
	return rt.markTextureWorkerUpdatesDelivered(ctx, rec, docID, consumedThroughSeq)
}

func (rt *Handler) advanceTextureControllerCheckpoint(ctx context.Context, ownerID, docID string, seq int64) error {
	if rt == nil || rt.Store == nil || seq <= 0 {
		return nil
	}
	ownerID = strings.TrimSpace(ownerID)
	docID = strings.TrimSpace(docID)
	if ownerID == "" || docID == "" {
		return nil
	}
	checkpoint, err := rt.Store.GetTextureControllerCheckpoint(ctx, docID, ownerID)
	if err != nil {
		return fmt.Errorf("load texture controller checkpoint: %w", err)
	}
	if checkpoint != nil && checkpoint.IntegratedMessageSeq >= seq {
		return nil
	}
	return rt.Store.UpsertTextureControllerCheckpoint(ctx, store.TextureControllerCheckpoint{
		DocID:                docID,
		OwnerID:              ownerID,
		IntegratedMessageSeq: seq,
		UpdatedAt:            time.Now().UTC(),
	})
}
