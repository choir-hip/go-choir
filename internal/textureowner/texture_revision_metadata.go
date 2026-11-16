package textureowner

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
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
	if isWireArticleRevisionRun(rec) && !promptOnlyInitialModelPrior {
		meta["artifact_kind"] = "article_revision"
		meta["revision_role"] = textureRevisionRoleCanonical
		meta["texture_version_stage"] = "article_revision"
	}
	// The from-weights stamp is not decided here: it belongs to the document's
	// first agent revision when that revision cites nothing, which only the
	// commit knows (markModelPriorInterim). Inherited from the parent it
	// marked every later revision, v2 and after, as model prior.
	if metadataStringValue(meta, "texture_version_stage") == "interim" {
		delete(meta, "texture_version_stage")
	}
	// A story's seed role names the one empty revision its opening
	// committed; it is never carried forward. Inherited, it marked every
	// Texture revision of a story as the seed, so none published (B12).
	if metadataStringValue(meta, "revision_role") == types.StorySeedRole {
		delete(meta, "revision_role")
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

// isFirstAgentRevision reports whether base, the head a Write is committed
// against, is the owner's seed: the document's request itself, with no parent.
// The revision committed against it is the document's first from the desk, v1
// when the desk answers at once.
func isFirstAgentRevision(base types.Revision) bool {
	return base.AuthorKind == types.AuthorUser && strings.TrimSpace(base.ParentRevisionID) == ""
}

// markModelPriorInterim stamps a revision as the desk's answer from its own
// knowledge, uncited and provisional (owner, 2026-11-16: v1 is a fast
// response from the model's weights). It stamps only the first agent revision
// of a prompt document, and only when the revision cites no source (refs is
// the result document's source refs, not the quote checks, which skip a cite
// with no quote) and no message had reached the desk's inbox: a first Write
// that cites a capture is grounded, one written after research messaged is
// written from them, and every later revision is written from what arrived. The stamp is the fields the reader and the next turn
// read (grounding_status, revision_grounding, model_prior_interim,
// texture_version_stage).
func markModelPriorInterim(rec *types.RunRecord, meta map[string]any, base types.Revision, refs int, researchMailSeen bool) bool {
	if refs > 0 || researchMailSeen || !isFirstAgentRevision(base) || !promptOnlyInitialModelPriorTextureRevision(rec, meta) {
		return false
	}
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
	return true
}

func promptOnlyInitialModelPriorTextureRevision(rec *types.RunRecord, meta map[string]any) bool {
	if rec == nil || agentProfileForRun(rec) != deskmanifest.DocumentCustodianName() {
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

// isWireArticleRevisionRun reports whether a Texture run came from the
// retired wire article handoff (universal_wire_* intents, or a Texture agent
// revision carrying ingestion lineage). Such revisions keep the canonical
// article_revision stamp they always had.
func isWireArticleRevisionRun(rec *types.RunRecord) bool {
	if rec == nil {
		return false
	}
	intent := metadataStringValue(rec.Metadata, "request_intent")
	if strings.HasPrefix(intent, "universal_wire_") && strings.HasSuffix(intent, "_article_revision") {
		return true
	}
	if metadataStringValue(rec.Metadata, "type") != textureAgentRevisionTaskType {
		return false
	}
	return firstNonEmpty(
		metadataStringValue(rec.Metadata, "source_network_cycle_id"),
		metadataStringValue(rec.Metadata, "ingestion_handoff_cycle_id"),
	) != ""
}
