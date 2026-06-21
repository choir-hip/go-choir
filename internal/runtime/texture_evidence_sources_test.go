package runtime

import (
	"context"
	"encoding/json"
	"strings"
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

func TestTextureCoagentSourceRefsSurviveInjectionAndDelivery(t *testing.T) {
	t.Parallel()
	rt, _ := testAPISetup(t)
	s := rt.Store()
	ctx := context.Background()
	now := time.Now().UTC()
	ownerID := "user-native-sources"
	docID := "doc-native-sources"
	targetAgentID := currentTextureAgentID(docID)

	doc := types.Document{
		DocID:   docID,
		OwnerID: ownerID,
		Title:   "Native sources",
	}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}
	parent := types.Revision{
		RevisionID: "rev-native-sources-v0",
		DocID:      docID,
		OwnerID:    ownerID,
		AuthorKind: types.AuthorUser,
		Content:    "Write the sourced update.",
		Citations:  json.RawMessage("[]"),
		Metadata:   json.RawMessage("{}"),
		CreatedAt:  now,
	}
	if err := s.CreateRevision(ctx, parent); err != nil {
		t.Fatalf("CreateRevision: %v", err)
	}
	doc.CurrentRevisionID = parent.RevisionID

	update := types.WorkerUpdateRecord{
		UpdateID:      "update-native-source-refs",
		OwnerID:       ownerID,
		AgentID:       "researcher:native-sources",
		TargetAgentID: targetAgentID,
		ChannelID:     docID,
		Role:          AgentProfileResearcher,
		Kind:          "findings",
		Summary:       "native source refs ready",
		Findings:      []string{"The source-backed finding is ready."},
		Refs:          []string{"source_service_item:srcitem_native_panel"},
		Content:       "Use the source-backed finding.",
		CreatedAt:     now,
	}
	message := types.ChannelMessage{
		ChannelID:   update.ChannelID,
		FromAgentID: update.AgentID,
		ToAgentID:   update.TargetAgentID,
		Role:        update.Role,
		Content:     update.Content,
		Timestamp:   now,
	}
	stored, _, err := s.DispatchWorkerUpdate(ctx, update, &message)
	if err != nil {
		t.Fatalf("DispatchWorkerUpdate: %v", err)
	}

	rec := &types.RunRecord{
		RunID:        "run-native-source-refs",
		OwnerID:      ownerID,
		AgentID:      targetAgentID,
		AgentProfile: AgentProfileTexture,
		AgentRole:    AgentProfileTexture,
		ChannelID:    docID,
		Metadata: map[string]any{
			"type":           textureAgentRevisionTaskType,
			"request_source": "update_coagent",
			"doc_id":         docID,
		},
	}
	inject := rt.coagentUpdateTurnInjector(rec)
	if inject == nil {
		t.Fatal("Texture coagent update injector is nil")
	}
	msgs, err := inject(false)
	if err != nil {
		t.Fatalf("inject coagent update: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("injected messages = %d, want 1", len(msgs))
	}
	entityID := stableSourceEntityID("source_service_item", "srcitem_native_panel")
	if !messageTextContains(t, msgs[0], `"source_entities"`) ||
		!messageTextContains(t, msgs[0], entityID) ||
		!messageTextContains(t, msgs[0], "[label](source:ENTITY_ID)") {
		t.Fatalf("coagent packet missing native source entity fields: %s", string(msgs[0]))
	}
	if !hasSourceEntity(decodeTextureSourceEntities(rec.Metadata["source_entities"]), "source_service_item", "srcitem_native_panel", "") {
		t.Fatalf("run metadata missing injected source_entities: %#v", rec.Metadata["source_entities"])
	}

	if err := s.MarkWorkerUpdatesDelivered(ctx, ownerID, targetAgentID, []string{stored.UpdateID}, rec.RunID); err != nil {
		t.Fatalf("MarkWorkerUpdatesDelivered: %v", err)
	}
	backlog, err := s.ListCoagentMailboxBacklog(ctx, ownerID, targetAgentID, 10)
	if err != nil {
		t.Fatalf("ListCoagentMailboxBacklog: %v", err)
	}
	if len(backlog) != 0 {
		t.Fatalf("backlog after cursor advance = %+v, want none", backlog)
	}

	result := rt.buildAppagentRevisionMetadata(ctx, rec, doc, ownerID, nil, stored.MessageSeq)
	meta := decodeRevisionMetadata(result)
	if !hasSourceEntity(decodeTextureSourceEntities(meta["source_entities"]), "source_service_item", "srcitem_native_panel", "") {
		t.Fatalf("revision metadata missing delivered source entity: %#v", meta["source_entities"])
	}
}

func messageTextContains(t *testing.T, raw json.RawMessage, needle string) bool {
	t.Helper()
	var msg map[string]any
	if err := json.Unmarshal(raw, &msg); err != nil {
		t.Fatalf("decode message: %v", err)
	}
	content, _ := msg["content"].([]any)
	if len(content) != 1 {
		t.Fatalf("content blocks = %+v", content)
	}
	block, _ := content[0].(map[string]any)
	text, _ := block["text"].(string)
	return strings.Contains(text, needle)
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
