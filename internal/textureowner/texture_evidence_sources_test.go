package textureowner

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/sourcecontract"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestEvidenceRecordToSourceEntity_ContentIDYieldsWholeResourceByDefault(t *testing.T) {
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
		entity.Selectors[0].SelectorKind != "whole_resource" ||
		entity.Selectors[0].TextQuote != "" {
		t.Fatalf("expected whole_resource selector, got %#v", entity.Selectors)
	}
	if entity.Label != "Rate-cut odds" || entity.Target.CanonicalURL != "https://example.test/markets/rates" {
		t.Fatalf("unexpected label/url: %#v", entity)
	}
}

func TestEvidenceRecordToSourceEntity_ContentIDExplicitTextQuoteYieldsTextQuote(t *testing.T) {
	rec := types.EvidenceRecord{
		EvidenceID: "ev-quote",
		Kind:       "source_excerpt",
		Title:      "Rate-cut odds",
		SourceURI:  "https://example.test/markets/rates",
		Content:    "researcher note: revenue rose sharply",
		Metadata:   json.RawMessage(`{"content_id":"content-rates","text_quote":"revenue surged 40% in Q3"}`),
	}
	entity := evidenceRecordToSourceEntity(rec)
	if len(entity.Selectors) != 1 ||
		entity.Selectors[0].SelectorKind != "text_quote" ||
		entity.Selectors[0].TextQuote != "revenue surged 40% in Q3" {
		t.Fatalf("expected explicit text_quote selector, got %#v", entity.Selectors)
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
	if entity.Kind != "web_url" || entity.Target.TargetKind != "web_url" {
		t.Fatalf("URL-only evidence should stay a web_url source, got %#v", entity)
	}
	if len(entity.Selectors) != 1 || entity.Selectors[0].SelectorKind != "whole_resource" {
		t.Fatalf("expected whole_resource selector, got %#v", entity.Selectors)
	}
}

func TestCoagentPacketHTTPSourceStaysURLBackedNotSyntheticContentItem(t *testing.T) {
	update := types.CoagentSourcePacket{
		OwnerID: "owner-url-source",
		AgentID: "research:url-source",
		Role:    agentprofile.Research,
		Packet: types.CoagentSourcePacketPayload{
			Sources: []types.CoagentPacketSource{{
				SourceID: "src-http",
				Kind:     "web_url",
				Target: types.CoagentPacketSourceTarget{
					URI:   "https://example.test/newsroom",
					Title: "Example newsroom",
				},
			}},
		},
	}
	entity := sourceEntityFromCoagentPacketSource(context.Background(), nil, update.OwnerID, update.Packet.Sources[0], update)
	if entity.EntityID == "" {
		t.Fatal("expected URL source entity")
	}
	if entity.Kind != "web_url" || entity.Target.TargetKind != "web_url" {
		t.Fatalf("HTTP packet source should stay URL-backed, got %#v", entity)
	}
	if entity.Target.ContentID != "" || entity.Target.ItemID != "" {
		t.Fatalf("HTTP packet source should not invent retrievable content ids: %#v", entity.Target)
	}
	if entity.Target.CanonicalURL != "https://example.test/newsroom" {
		t.Fatalf("canonical URL not preserved: %#v", entity.Target)
	}
}

func TestCoagentPacketHTTPSourcePreservesSourceTextForTransclusion(t *testing.T) {
	update := types.CoagentSourcePacket{
		OwnerID: "owner-url-source-text",
		AgentID: "research:url-source-text",
		Role:    agentprofile.Research,
		Packet: types.CoagentSourcePacketPayload{
			Sources: []types.CoagentPacketSource{{
				SourceID: "src-whitehouse",
				Kind:     "web_url",
				Target: types.CoagentPacketSourceTarget{
					URI:       "https://example.test/ai-policy",
					Title:     "AI policy source",
					MediaType: "text/html",
				},
				Excerpt: "The order directs agencies to accelerate secure AI adoption.",
				ReaderSnapshot: &types.CoagentPacketSourceReaderSnapshot{
					TextContent:       "The order directs agencies to accelerate secure AI adoption.\n\nIt also assigns responsibilities across federal agencies.",
					SnapshotKind:      "cleaned_reader_markdown",
					MediaType:         "text/markdown",
					OriginalMediaType: "text/html",
					AccessScope:       "private_user_source",
				},
			}},
		},
	}
	entity := sourceEntityFromCoagentPacketSource(context.Background(), nil, update.OwnerID, update.Packet.Sources[0], update)
	if entity.EntityID == "" {
		t.Fatal("expected URL source entity")
	}
	if entity.Kind != "web_url" || entity.Target.TargetKind != "web_url" || entity.Target.ContentID != "" {
		t.Fatalf("source text must not turn URL source into synthetic content item: %#v", entity)
	}
	if len(entity.Selectors) == 0 || entity.Selectors[0].SelectorKind != sourcecontract.SelectorKindTextQuote || !strings.Contains(entity.Selectors[0].TextQuote, "accelerate secure AI adoption") {
		t.Fatalf("expected bounded excerpt text_quote selector first, got %#v", entity.Selectors)
	}
	if entity.ReaderSnapshot == nil || !strings.Contains(metadataString(entity.ReaderSnapshot, "text_content"), "assigns responsibilities") {
		t.Fatalf("reader snapshot text not preserved: %#v", entity.ReaderSnapshot)
	}
	if metadataString(entity.ReaderSnapshotStatus, "state") != sourcecontract.ReaderArtifactStateReady {
		t.Fatalf("reader snapshot status = %#v", entity.ReaderSnapshotStatus)
	}
	if !entity.Evidence.ReaderSnapshot || entity.Evidence.SourceRepresentationID != sourcecontract.ReaderArtifactStateReady {
		t.Fatalf("evidence reader snapshot not recorded: %#v", entity.Evidence)
	}

	structured := structuredSourceEntityFromRuntimeSource(entity)
	if structured.ReaderSnapshot == nil || !strings.Contains(metadataString(structured.ReaderSnapshot, "text_content"), "assigns responsibilities") {
		t.Fatalf("structured reader snapshot not preserved: %#v", structured.ReaderSnapshot)
	}
	if structured.Selectors[0].Kind != sourcecontract.SelectorKindTextQuote {
		t.Fatalf("structured selector = %#v", structured.Selectors)
	}
}

func TestCoagentPacketContentIDSourceHydratesImportedText(t *testing.T) {
	t.Parallel()
	_, handler := testAPISetup(t)
	s := handler.Store
	ctx := context.Background()
	now := time.Now().UTC()
	ownerID := "owner-content-id-source-text"
	contentID := "content-whitehouse-eo"

	if err := s.CreateContentItem(ctx, types.ContentItem{
		ContentID:    contentID,
		OwnerID:      ownerID,
		SourceType:   "imported_url",
		MediaType:    "text/html",
		Title:        "White House AI security executive order",
		SourceURL:    "https://www.whitehouse.gov/presidential-actions/2026/06/promoting-advanced-artificial-intelligence-innovation-and-security/",
		CanonicalURL: "https://www.whitehouse.gov/presidential-actions/2026/06/promoting-advanced-artificial-intelligence-innovation-and-security/",
		TextContent:  "PROMOTING ADVANCED ARTIFICIAL INTELLIGENCE INNOVATION AND SECURITY\n\nExecutive Order 14409 directs agencies to accelerate secure AI adoption.",
		ContentHash:  "hash-whitehouse-eo",
		CreatedAt:    now,
		UpdatedAt:    now,
	}); err != nil {
		t.Fatalf("CreateContentItem: %v", err)
	}

	update := types.CoagentSourcePacket{
		OwnerID: ownerID,
		AgentID: "research:content-id-source-text",
		Role:    agentprofile.Research,
		Packet: types.CoagentSourcePacketPayload{
			Sources: []types.CoagentPacketSource{{
				SourceID: "src-content-id",
				Kind:     "content_item",
				Target: types.CoagentPacketSourceTarget{
					URI:   "content_id:" + contentID,
					Title: "content_id:" + contentID,
				},
			}},
		},
	}

	entity := sourceEntityFromCoagentPacketSource(ctx, handler, ownerID, update.Packet.Sources[0], update)
	if entity.EntityID == "" {
		t.Fatal("expected content-id source entity")
	}
	if entity.Label != "White House AI security executive order" {
		t.Fatalf("typed ref title leaked into label: %#v", entity)
	}
	if entity.Target.TargetKind != "content_item" || entity.Target.ContentID != contentID {
		t.Fatalf("expected hydrated content_item target, got %#v", entity.Target)
	}
	if entity.ReaderSnapshot == nil || !strings.Contains(metadataString(entity.ReaderSnapshot, "text_content"), "Executive Order 14409") {
		t.Fatalf("content item text was not preserved as reader snapshot: %#v", entity.ReaderSnapshot)
	}
	if metadataString(entity.ReaderSnapshotStatus, "state") != sourcecontract.ReaderArtifactStateReady {
		t.Fatalf("reader snapshot status = %#v", entity.ReaderSnapshotStatus)
	}
	if !entity.Evidence.ReaderSnapshot || entity.Evidence.BodyKind != "reader_snapshot" {
		t.Fatalf("reader snapshot evidence not recorded: %#v", entity.Evidence)
	}
}

func TestEvidenceRecordToSourceEntity_NoAddressableTargetSkipped(t *testing.T) {
	rec := types.EvidenceRecord{EvidenceID: "ev-3", Kind: "note", Content: "ungrounded thought"}
	if entity := evidenceRecordToSourceEntity(rec); entity.EntityID != "" {
		t.Fatalf("expected zero entity for unaddressable evidence, got %#v", entity)
	}
}

func TestEvidenceSummaryEntityAllowsNativeCitationWithoutQuoteMatch(t *testing.T) {
	rec := types.EvidenceRecord{
		EvidenceID: "ev-summary",
		Title:      "OpenAI docs",
		Content:    "Research synthesis: OpenAI API docs identify GPT-5.5 as public.",
		Metadata:   json.RawMessage(`{"content_id":"content-openai-docs"}`),
	}
	entity := evidenceRecordToSourceEntity(rec)
	if len(entity.Selectors) != 1 || entity.Selectors[0].SelectorKind != "whole_resource" {
		t.Fatalf("summary evidence should cite as whole_resource, got %#v", entity.Selectors)
	}
}

func TestEvidenceRecordToSourceEntity_CommandOutputIsAddressable(t *testing.T) {
	rec := types.EvidenceRecord{
		EvidenceID: "ev-command",
		OwnerID:    "user-command",
		AgentID:    "management:verify",
		Kind:       "command_output",
		Title:      "Focused runtime tests",
		SourceURI:  "command_output:cmd-runtime-source-handoff",
		Content:    "PASS",
	}
	entity := evidenceRecordToSourceEntity(rec)
	if entity.EntityID == "" || entity.Kind != "command_output" {
		t.Fatalf("unexpected command output entity %#v", entity)
	}
	if entity.Target.TargetKind != "command_output" || entity.Target.PublicRecordID != "cmd-runtime-source-handoff" {
		t.Fatalf("unexpected command target %#v", entity.Target)
	}
	if entity.Display.OpenSurface != "source_window" || entity.Provenance.CreatedBy != "management:verify" {
		t.Fatalf("unexpected command display/provenance %#v", entity)
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

func sourceEntityByContentID(entities []textureSourceEntity, contentID string) textureSourceEntity {
	for _, entity := range entities {
		if entity.Target.ContentID == contentID {
			return entity
		}
	}
	return textureSourceEntity{}
}

func assertExecutionEntity(t *testing.T, entities []textureSourceEntity, kind, identity, labelNeedle string) {
	t.Helper()
	for _, entity := range entities {
		if entity.Kind != kind || entity.Target.TargetKind != kind {
			continue
		}
		targetIdentity := firstNonEmpty(entity.Target.PublicRecordID, entity.Target.FilePath, entity.Target.ContentID, entity.Target.ItemID)
		if targetIdentity != identity {
			continue
		}
		if labelNeedle != "" && !strings.Contains(entity.Label, labelNeedle) {
			continue
		}
		return
	}
	t.Fatalf("missing execution source kind=%s identity=%s label~%q in %#v", kind, identity, labelNeedle, entities)
}


func TestSelfDevelopmentJoinIgnoresProseAndPersistsInRevisionMetadata(t *testing.T) {
	t.Parallel()
	_, handler := testAPISetup(t)
	ctx := context.Background()
	now := time.Now().UTC()
	ownerID := "user-joinable-metadata"
	docID := "doc-joinable-metadata"
	operationID := "operation-meta-1"
	bundleDigest := "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
	doc := types.Document{DocID: docID, OwnerID: ownerID, Title: "Joinable identities"}
	if err := handler.Store.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}
	parent := types.Revision{
		RevisionID: "rev-joinable-parent", DocID: docID, OwnerID: ownerID,
		AuthorKind: types.AuthorUser, Content: "Human prose without machine identifiers.",
		Citations: json.RawMessage("[]"), CreatedAt: now, Metadata: json.RawMessage("{}"),
	}
	if err := handler.Store.CreateRevision(ctx, parent); err != nil {
		t.Fatalf("CreateRevision: %v", err)
	}
	doc.CurrentRevisionID = parent.RevisionID
	entities := []textureSourceEntity{
		executionEvidenceSourceEntity("operation", operationID, operationID, "management"),
		executionEvidenceSourceEntity("capsule_bundle", bundleDigest, bundleDigest, "management"),
		executionEvidenceSourceEntity("receipt", "receipt-meta-1", "receipt-meta-1", "management"),
		executionEvidenceSourceEntity("event_head", "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee", "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee", "management"),
	}
	rec := &types.RunRecord{
		RunID: "run-joinable-metadata", OwnerID: ownerID, AgentID: currentTextureAgentID(docID),
		AgentProfile: agentprofile.Texture, AgentRole: agentprofile.Texture, ChannelID: docID,
		Metadata: map[string]any{
			"type": textureAgentRevisionTaskType, "request_source": "update_coagent", "doc_id": docID,
		},
	}
	mergeTextureSourceEntitiesIntoRunMetadata(rec, entities)
	result := handler.buildAppagentRevisionMetadata(ctx, rec, doc, ownerID, nil, 0)
	meta := decodeRevisionMetadata(result)
	if meta[textureJoinOperationID] != operationID || meta[textureJoinBundleDigest] != bundleDigest {
		t.Fatalf("joinable metadata missing: %#v", meta)
	}
	if meta[textureJoinReceiptID] != "receipt-meta-1" || meta[textureJoinEventHead] != "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee" {
		t.Fatalf("joinable receipt/head missing: %#v", meta)
	}
	if _, ok := meta[textureAvailableSourceEntitiesKey]; ok {
		t.Fatalf("available source entities leaked into revision metadata: %#v", meta[textureAvailableSourceEntitiesKey])
	}
	if strings.Contains(parent.Content, operationID) || strings.Contains(parent.Content, bundleDigest) {
		t.Fatal("prose unexpectedly contained machine identifiers")
	}
}


