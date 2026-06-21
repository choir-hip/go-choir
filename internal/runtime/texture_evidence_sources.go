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
	seenEvidence := map[string]bool{}
	entities := []textureSourceEntity{}
	seenEntity := map[string]bool{}
	for _, update := range updates {
		for _, evidenceID := range update.EvidenceIDs {
			evidenceID = strings.TrimSpace(evidenceID)
			if evidenceID == "" || seenEvidence[evidenceID] {
				continue
			}
			seenEvidence[evidenceID] = true
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
	}
	return entities
}
