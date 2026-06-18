package runtime

import (
	"encoding/json"
	"testing"

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
