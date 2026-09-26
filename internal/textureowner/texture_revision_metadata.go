package textureowner

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/wirepublish"
)

// buildAppagentRevisionMetadata constructs the metadata JSON for an
// appagent-authored revision, carrying forward durable context keys
// from the parent revision so they remain available on the next revise.
func (rt *Handler) buildAppagentRevisionMetadata(_ context.Context, rec *types.RunRecord, doc types.Document, ownerID string, _ *store.AgentMutation, _ int64) json.RawMessage {
	meta := map[string]any{
		"source":  "texture_cell",
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
	promptOnlyInitialModelPrior := promptOnlyInitialModelPriorTextureRevision(rec, meta)
	if wirepublish.IsWireArticleRevisionRun(rec) && !promptOnlyInitialModelPrior {
		meta["artifact_kind"] = "article_revision"
		meta["revision_role"] = textureRevisionRoleCanonical
		meta["texture_version_stage"] = "article_revision"
	}
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
	join := selfDevelopmentJoinFromSourceEntities(decodeAvailableTextureSourceEntities(rec.Metadata))
	mergeSelfDevelopmentJoinIntoMetadata(meta, join)
	// Available source entities are run-time prompt context, not durable revision metadata.
	// Keep them out of the persisted revision so they do not leak into the next run's parent
	// revision projection and are recomputed from the actual revision's source_entities.
	// Joinable self-development identities stay in dedicated durable keys, never in prose.
	delete(meta, textureAvailableSourceEntitiesKey)

	data, err := json.Marshal(meta)
	if err != nil {
		return json.RawMessage(`{"source":"texture_cell","loop_id":"` + rec.RunID + `"}`)
	}
	return data
}

func promptOnlyInitialModelPriorTextureRevision(rec *types.RunRecord, meta map[string]any) bool {
	if rec == nil || agentProfileForRun(rec) != agentprofile.Texture {
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

