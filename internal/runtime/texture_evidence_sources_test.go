package runtime

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestEvidenceRecordToSourceEntity_ContentIDYieldsTextQuote(t *testing.T) {
	rec := types.EvidenceRecord{
		EvidenceID: "ev-1",
		Kind:       "source_excerpt",
		Title:      "Rate-cut odds",
		SourceURI:  "https://example.test/markets/rates",
		Content:    "revenue surged 40% in Q3",
		Metadata:   json.RawMessage(`{"content_id":"content-rates"}`),
	}
	entity := evidenceRecordToSourceEntity(rec)
	if entity.EntityID == "" || entity.EntityID != stableSourceEntityID("content_item", "content-rates") {
		t.Fatalf("unexpected entity id %q", entity.EntityID)
	}
	if entity.Target.TargetKind != "content_item" || entity.Target.ContentID != "content-rates" {
		t.Fatalf("unexpected target %#v", entity.Target)
	}
	if len(entity.Selectors) != 1 ||
		entity.Selectors[0].SelectorKind != "text_quote" ||
		entity.Selectors[0].TextQuote != "revenue surged 40% in Q3" {
		t.Fatalf("expected text_quote selector, got %#v", entity.Selectors)
	}
	if entity.Label != "Rate-cut odds" || entity.Target.CanonicalURL != "https://example.test/markets/rates" {
		t.Fatalf("unexpected label/url: %#v", entity)
	}
}

func TestEvidenceRecordToSourceEntity_URLOnlyIsWholeResource(t *testing.T) {
	rec := types.EvidenceRecord{
		EvidenceID: "ev-2",
		Kind:       "web",
		SourceURI:  "https://example.test/a",
		Content:    "some excerpt",
	}
	entity := evidenceRecordToSourceEntity(rec)
	if entity.EntityID == "" || entity.Target.URL != "https://example.test/a" {
		t.Fatalf("unexpected entity %#v", entity)
	}
	if len(entity.Selectors) != 1 || entity.Selectors[0].SelectorKind != "whole_resource" {
		t.Fatalf("expected whole_resource selector, got %#v", entity.Selectors)
	}
}

func TestEvidenceRecordToSourceEntity_NoAddressableTargetSkipped(t *testing.T) {
	rec := types.EvidenceRecord{EvidenceID: "ev-3", Kind: "note", Content: "ungrounded thought"}
	if entity := evidenceRecordToSourceEntity(rec); entity.EntityID != "" {
		t.Fatalf("expected zero entity for unaddressable evidence, got %#v", entity)
	}
}

func TestEvidenceRecordToSourceEntity_ContentIDWithoutExcerptIsWholeResource(t *testing.T) {
	rec := types.EvidenceRecord{
		EvidenceID: "ev-4",
		Content:    "",
		Metadata:   json.RawMessage(`{"content_id":"content-x"}`),
	}
	entity := evidenceRecordToSourceEntity(rec)
	if len(entity.Selectors) != 1 || entity.Selectors[0].SelectorKind != "whole_resource" {
		t.Fatalf("expected whole_resource selector for empty excerpt, got %#v", entity.Selectors)
	}
}

// TestEvidenceDerivedEntityFeedsCitationValidator proves the previously-dormant
// quote-match branch is now driven by typed researcher evidence: an evidence
// excerpt becomes a text_quote selector, and a body citing that entity is gated
// on the excerpt appearing in the retrieved source body.
func TestEvidenceDerivedEntityFeedsCitationValidator(t *testing.T) {
	rec := types.EvidenceRecord{
		EvidenceID: "ev-5",
		Title:      "Audit source",
		Content:    "Cloud providers should preserve auditability",
		Metadata:   json.RawMessage(`{"content_id":"content-audit"}`),
	}
	entity := evidenceRecordToSourceEntity(rec)
	entities := []textureSourceEntity{entity}
	body := "As established, [audit](source:" + entity.EntityID + ")."

	good := map[string]string{entity.EntityID: "The report: Cloud providers should preserve auditability across regions."}
	if issues := validateTextureCitations(body, entities, good); len(issues) != 0 {
		t.Fatalf("expected grounded excerpt citation to pass, got %v", issues)
	}

	bad := map[string]string{entity.EntityID: "An unrelated paragraph with no matching excerpt."}
	if got := reasonsByID(validateTextureCitations(body, entities, bad))[entity.EntityID]; got != citationQuoteNotInSource {
		t.Fatalf("expected quote_not_in_source, got %q", got)
	}
}

func TestPendingUpdateRefsBecomeSourceEntities(t *testing.T) {
	t.Parallel()
	rt, _ := testAPISetup(t)
	s := rt.Store()
	ctx := context.Background()
	now := time.Now().UTC()
	ownerID := "user-refs"
	targetAgentID := "texture:doc-refs"

	if err := s.CreateContentItem(ctx, types.ContentItem{
		ContentID:    "content-cloud-audit",
		OwnerID:      ownerID,
		SourceType:   "extracted_url",
		MediaType:    "text/html",
		Title:        "Cloud Audit Brief",
		SourceURL:    "https://example.test/cloud-audit",
		CanonicalURL: "https://example.test/cloud-audit",
		TextContent:  "Cloud providers should preserve auditability.",
		ContentHash:  "hash-cloud-audit",
		CreatedAt:    now,
		UpdatedAt:    now,
	}); err != nil {
		t.Fatalf("CreateContentItem: %v", err)
	}
	if err := s.CreateEvidence(ctx, types.EvidenceRecord{
		EvidenceID: "ev-cloud-audit",
		OwnerID:    ownerID,
		AgentID:    "researcher:refs",
		Kind:       "source_excerpt",
		Title:      "Cloud evidence",
		SourceURI:  "https://example.test/evidence",
		Content:    "Audit trails are available",
		Metadata:   json.RawMessage(`{"content_id":"content-evidence-audit"}`),
		CreatedAt:  now,
	}); err != nil {
		t.Fatalf("CreateEvidence: %v", err)
	}

	update := types.WorkerUpdateRecord{
		UpdateID:      "update-refs-source-entities",
		OwnerID:       ownerID,
		AgentID:       "researcher:refs",
		TargetAgentID: targetAgentID,
		ChannelID:     "doc-refs",
		Role:          AgentProfileResearcher,
		Kind:          "findings",
		Summary:       "source refs ready",
		Findings:      []string{"Typed refs should be available to Texture."},
		Refs: []string{
			"source_service_item:srcitem_market_rules",
			"source_service_item=srcitem_policy_digest",
			"content_id:content-cloud-audit",
			"evidence_id:ev-cloud-audit",
			"free-form note mentioning srcitem_ignored in prose",
		},
		Content:   "source refs ready",
		CreatedAt: now,
	}
	message := types.ChannelMessage{
		ChannelID:   update.ChannelID,
		FromAgentID: update.AgentID,
		ToAgentID:   update.TargetAgentID,
		Role:        update.Role,
		Content:     update.Content,
		Timestamp:   now,
	}
	if _, _, err := s.DispatchWorkerUpdate(ctx, update, &message); err != nil {
		t.Fatalf("DispatchWorkerUpdate: %v", err)
	}

	entities := rt.evidenceSourceEntitiesFromPendingUpdates(ctx, ownerID, targetAgentID, 10)
	if !hasSourceEntity(entities, "source_service_item", "srcitem_market_rules", "") {
		t.Fatalf("missing source_service_item entity: %#v", entities)
	}
	if !hasSourceEntity(entities, "source_service_item", "srcitem_policy_digest", "") {
		t.Fatalf("missing equals-form source_service_item entity: %#v", entities)
	}
	if !hasSourceEntity(entities, "content_item", "", "content-cloud-audit") {
		t.Fatalf("missing content item entity: %#v", entities)
	}
	if !hasSourceEntity(entities, "content_item", "", "content-evidence-audit") {
		t.Fatalf("missing evidence ref entity: %#v", entities)
	}
	for _, entity := range entities {
		if entity.Target.ItemID == "srcitem_ignored" {
			t.Fatalf("free-form prose ref was scraped into source entity: %#v", entities)
		}
	}
}

func hasSourceEntity(entities []textureSourceEntity, kind, itemID, contentID string) bool {
	for _, entity := range entities {
		if kind != "" && entity.Kind != kind {
			continue
		}
		if itemID != "" && entity.Target.ItemID != itemID {
			continue
		}
		if contentID != "" && entity.Target.ContentID != contentID {
			continue
		}
		return true
	}
	return false
}
