package runtime

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/sourcecontract"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// evidenceContentID extracts an owner-scoped content item id from an evidence
// record's metadata (content_id) so the citation/quote validator can retrieve the
// stored body to verify excerpts against. Returns "" when no content id is
// declared.
func evidenceContentID(rec types.EvidenceRecord) string {
	if len(rec.Metadata) == 0 {
		return ""
	}
	var meta map[string]any
	if err := json.Unmarshal(rec.Metadata, &meta); err != nil {
		return ""
	}
	for _, key := range []string{"content_id", "content_item_id"} {
		if id := strings.TrimSpace(metadataString(meta, key)); id != "" {
			return id
		}
	}
	return ""
}

// evidenceRecordToSourceEntity turns a typed researcher evidence record into a
// collated source entity. When the evidence references a retrievable content item
// and carries an excerpt, the entity gets a text_quote selector (the excerpt),
// which the deterministic citation/quote validator checks against the stored body
// at write time. Evidence without a retrievable body becomes a whole_resource
// reference (cited resolution still validated, quote not). Returns a zero entity
// (EntityID == "") when there is nothing addressable to cite.
func evidenceRecordToSourceEntity(rec types.EvidenceRecord) textureSourceEntity {
	quote := strings.TrimSpace(rec.Content)
	contentID := evidenceContentID(rec)
	sourceURI := strings.TrimSpace(rec.SourceURI)

	var entity textureSourceEntity
	switch {
	case contentID != "":
		entity.EntityID = stableSourceEntityID("content_item", contentID)
		entity.Kind = "content_item"
		entity.Target = textureSourceEntityTarget{TargetKind: "content_item", ContentID: contentID}
		if isHTTPURL(sourceURI) {
			entity.Target.URL = sourceURI
			entity.Target.CanonicalURL = sourceURI
		}
		if quote != "" {
			entity.Selectors = []textureSourceEntitySelector{{SelectorKind: "text_quote", TextQuote: quote}}
		} else {
			entity.Selectors = []textureSourceEntitySelector{{SelectorKind: "whole_resource"}}
		}
	case isHTTPURL(sourceURI):
		entity.EntityID = stableSourceEntityID("content_item", sourceURI)
		entity.Kind = "content_item"
		entity.Target = textureSourceEntityTarget{TargetKind: "content_item", URL: sourceURI, CanonicalURL: sourceURI}
		entity.Selectors = []textureSourceEntitySelector{{SelectorKind: "whole_resource"}}
	default:
		return textureSourceEntity{}
	}

	entity.Label = firstNonEmpty(strings.TrimSpace(rec.Title), contentID, sourceURI, "Researcher source")
	entity.Display = textureSourceEntityDisplay{
		InlineMode:       "collapsed_citation",
		ExpandedMode:     "source_card",
		OpenSurface:      sourcecontract.OpenSurfaceSource,
		DefaultCollapsed: true,
	}
	entity.Evidence = textureSourceEntityEvidence{State: "available", ResearchState: "represented"}
	entity.Provenance = textureSourceEntityProvenance{
		CreatedBy:           "researcher",
		RightsScope:         "private_user_source",
		UntrustedSourceText: true,
	}
	return entity
}

func sourceEntityFromWorkerUpdateRef(ctx context.Context, rt *Runtime, ownerID, ref string) textureSourceEntity {
	key, value := splitTypedWorkerUpdateRef(ref)
	if key == "" || value == "" {
		return textureSourceEntity{}
	}
	switch key {
	case "source_service_item":
		if !textureRawSourceServiceItemIDRE.MatchString(value) || textureRawSourceServiceItemIDRE.FindString(value) != value {
			return textureSourceEntity{}
		}
		return sourceServiceItemRefToSourceEntity(value, ref)
	case "content_id", "content_item":
		if rt == nil || rt.store == nil {
			return textureSourceEntity{}
		}
		item, err := rt.store.GetContentItem(ctx, ownerID, value)
		if err != nil {
			return textureSourceEntity{}
		}
		return contentItemRefToSourceEntity(item)
	case "evidence", "evidence_id":
		if rt == nil || rt.store == nil {
			return textureSourceEntity{}
		}
		rec, err := rt.store.GetEvidence(ctx, value, ownerID)
		if err != nil {
			return textureSourceEntity{}
		}
		return evidenceRecordToSourceEntity(rec)
	default:
		return textureSourceEntity{}
	}
}

func (rt *Runtime) evidenceSourceEntitiesFromWorkerUpdates(ctx context.Context, ownerID string, updates []types.WorkerUpdateRecord) []textureSourceEntity {
	if len(updates) == 0 {
		return nil
	}
	seenEvidence := map[string]bool{}
	entities := []textureSourceEntity{}
	seenEntity := map[string]bool{}
	for _, update := range updates {
		if ownerID != "" && strings.TrimSpace(update.OwnerID) != strings.TrimSpace(ownerID) {
			continue
		}
		for _, evidenceID := range update.EvidenceIDs {
			evidenceID = strings.TrimSpace(evidenceID)
			if evidenceID == "" || seenEvidence[evidenceID] {
				continue
			}
			seenEvidence[evidenceID] = true
			if rt == nil || rt.store == nil {
				continue
			}
			rec, err := rt.store.GetEvidence(ctx, evidenceID, ownerID)
			if err != nil {
				continue
			}
			entity := evidenceRecordToSourceEntity(rec)
			key := sourceEntityKey(entity)
			if entity.EntityID == "" || key == "" || seenEntity[key] {
				continue
			}
			seenEntity[key] = true
			entities = append(entities, entity)
		}
		for _, ref := range update.Refs {
			entity := sourceEntityFromWorkerUpdateRef(ctx, rt, ownerID, ref)
			key := sourceEntityKey(entity)
			if entity.EntityID == "" || key == "" || seenEntity[key] {
				continue
			}
			seenEntity[key] = true
			entities = append(entities, entity)
		}
	}
	return entities
}

func (rt *Runtime) evidenceSourceEntitiesFromWorkerUpdateIDs(ctx context.Context, ownerID, targetAgentID string, updateIDs []string, limit int) []textureSourceEntity {
	if rt == nil || rt.store == nil || len(updateIDs) == 0 {
		return nil
	}
	ownerID = strings.TrimSpace(ownerID)
	targetAgentID = strings.TrimSpace(targetAgentID)
	if ownerID == "" || targetAgentID == "" {
		return nil
	}
	if limit <= 0 || limit > len(updateIDs) {
		limit = len(updateIDs)
	}
	updates := make([]types.WorkerUpdateRecord, 0, limit)
	seen := map[string]bool{}
	for _, updateID := range updateIDs {
		updateID = strings.TrimSpace(updateID)
		if updateID == "" || seen[updateID] {
			continue
		}
		seen[updateID] = true
		update, err := rt.store.GetWorkerUpdate(ctx, ownerID, updateID)
		if err != nil {
			continue
		}
		if strings.TrimSpace(update.TargetAgentID) != targetAgentID {
			continue
		}
		updates = append(updates, update)
		if len(updates) >= limit {
			break
		}
	}
	return rt.evidenceSourceEntitiesFromWorkerUpdates(ctx, ownerID, updates)
}

func mergeTextureSourceEntitiesIntoMetadata(metadata map[string]any, incoming []textureSourceEntity) bool {
	if metadata == nil || len(incoming) == 0 {
		return false
	}
	existing := decodeTextureSourceEntities(metadata["source_entities"])
	merged, changed := mergeTextureSourceEntities(existing, incoming)
	if len(merged) > 0 {
		metadata["source_entities"] = merged
	}
	return changed
}

func mergeTextureSourceEntitiesIntoRunMetadata(rec *types.RunRecord, incoming []textureSourceEntity) bool {
	if rec == nil || len(incoming) == 0 {
		return false
	}
	if rec.Metadata == nil {
		rec.Metadata = map[string]any{}
	}
	return mergeTextureSourceEntitiesIntoMetadata(rec.Metadata, incoming)
}

func splitTypedWorkerUpdateRef(ref string) (string, string) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", ""
	}
	for _, sep := range []string{":", "="} {
		if before, after, ok := strings.Cut(ref, sep); ok {
			key := normalizeWorkerUpdateRefKey(before)
			value := strings.TrimSpace(after)
			if key == "" || value == "" || strings.ContainsAny(value, " \t\r\n") {
				return "", ""
			}
			return key, value
		}
	}
	return "", ""
}

func normalizeWorkerUpdateRefKey(key string) string {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "source_service_item", "source_item", "item_id":
		return "source_service_item"
	case "content_id", "content_item", "content_item_id":
		return "content_id"
	case "evidence", "evidence_id":
		return "evidence"
	default:
		return ""
	}
}

func isHTTPURL(value string) bool {
	value = strings.TrimSpace(value)
	return strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://")
}

// evidenceSourceEntitiesFromPendingUpdates collates source entities from the
// typed evidence records attached to the pending update_coagent records addressed
// to a Texture coagent. This is the typed replacement for the deleted regex
// researcher-prose scraping: sources (and their text_quote excerpts) come from
// structured researcher evidence, not from parsing message text.
func (rt *Runtime) evidenceSourceEntitiesFromPendingUpdates(ctx context.Context, ownerID, textureAgentID string, limit int) []textureSourceEntity {
	if rt == nil || rt.store == nil {
		return nil
	}
	textureAgentID = strings.TrimSpace(textureAgentID)
	ownerID = strings.TrimSpace(ownerID)
	if textureAgentID == "" || ownerID == "" {
		return nil
	}
	updates, err := rt.store.ListCoagentMailboxBacklog(ctx, ownerID, textureAgentID, limit)
	if err != nil || len(updates) == 0 {
		return nil
	}
	return rt.evidenceSourceEntitiesFromWorkerUpdates(ctx, ownerID, updates)
}
