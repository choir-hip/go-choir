package runtime

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/sourcecontract"
	"github.com/yusefmosiah/go-choir/internal/texturedoc"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/agentprofile"
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
		AgentID: "researcher:url-source",
		Role:    agentprofile.Researcher,
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
		AgentID: "researcher:url-source-text",
		Role:    agentprofile.Researcher,
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
	rt, _ := testAPISetup(t)
	s := rt.Store()
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
		AgentID: "researcher:content-id-source-text",
		Role:    agentprofile.Researcher,
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

	entity := sourceEntityFromCoagentPacketSource(ctx, rt, ownerID, update.Packet.Sources[0], update)
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

func TestEvidenceSummaryEntityAllowsNativeCitationWithoutQuoteMatch(t *testing.T) {
	rec := types.EvidenceRecord{
		EvidenceID: "ev-summary",
		Title:      "OpenAI docs",
		Content:    "Researcher synthesis: OpenAI API docs identify GPT-5.5 as public.",
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
		AgentID:    "super:verify",
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
	if entity.Display.OpenSurface != "source_window" || entity.Provenance.CreatedBy != "super:verify" {
		t.Fatalf("unexpected command display/provenance %#v", entity)
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

	sources := coagentSourcesFromTypedEvidenceRefs([]string{
		"source_service_item:srcitem_market_rules",
		"source_service_item=srcitem_policy_digest",
		"content_id:content-cloud-audit",
		"evidence_id:ev-cloud-audit",
		"free-form note mentioning srcitem_ignored in prose",
	})
	update := types.CoagentSourcePacket{
		UpdateID:      "update-refs-source-entities",
		OwnerID:       ownerID,
		AgentID:       "researcher:refs",
		TargetAgentID: targetAgentID,
		ChannelID:     "doc-refs",
		Role:          agentprofile.Researcher,
		Packet:        newCoagentPacket("evidence_update", "source refs ready", coagentClaimsFromTexts([]string{"Typed refs should be available to Texture."}, sources), sources, nil, nil, nil),
		Content:       "source refs ready",
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
	cloudAudit := sourceEntityByContentID(entities, "content-cloud-audit")
	if cloudAudit.Label != "Cloud Audit Brief" {
		t.Fatalf("content-id ref leaked into source label: %#v", cloudAudit)
	}
	if cloudAudit.ReaderSnapshot == nil || !strings.Contains(metadataString(cloudAudit.ReaderSnapshot, "text_content"), "preserve auditability") {
		t.Fatalf("content item text not preserved for pending update ref: %#v", cloudAudit.ReaderSnapshot)
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

func TestWorkerUpdateExecutionEvidenceBecomesSourceEntitiesWithoutProseScraping(t *testing.T) {
	t.Parallel()
	rt, _ := testAPISetup(t)
	ctx := context.Background()
	now := time.Now().UTC()
	ownerID := "user-execution-sources"

	sources := coagentSourcesFromTypedEvidenceRefs([]string{
		"command_output:cmd-runtime-tests",
		"diff_hunk:diff-texture-evidence",
		"app_change_package:acp-structured-texture",
		"./proof/screenshots/texture-sources.png",
		"benchmark_log:bench-runtime-shards",
	})
	sources = append(sources, coagentSourceFromURI("src-test-runtime-focused", "test_run", "test_run:runtime-focused-test", "go test ./internal/runtime -run TestWorkerUpdateExecutionEvidenceBecomesSourceEntitiesWithoutProseScraping -count=1: passed"))
	updates := []types.CoagentSourcePacket{{
		UpdateID:      "update-super-execution-sources",
		OwnerID:       ownerID,
		AgentID:       "super:execution",
		TargetAgentID: "texture:doc-execution",
		ChannelID:     "doc-execution",
		Role:          agentprofile.Super,
		Packet: newCoagentPacket("execution_result", "implementation and verification evidence ready",
			coagentClaimsFromTexts([]string{"Do not scrape command_output:prose-only or diff_hunk:prose-only from ordinary findings."}, sources),
			sources,
			nil,
			nil,
			nil),
		Content:   "verification evidence ready",
		CreatedAt: now,
	}}

	entities := rt.evidenceSourceEntitiesFromWorkerUpdates(ctx, ownerID, updates)
	assertExecutionEntity(t, entities, "command_output", "cmd-runtime-tests", "")
	assertExecutionEntity(t, entities, "diff_hunk", "diff-texture-evidence", "")
	assertExecutionEntity(t, entities, "app_change_package", "acp-structured-texture", "")
	assertExecutionEntity(t, entities, "file_artifact", "./proof/screenshots/texture-sources.png", "")
	assertExecutionEntity(t, entities, "benchmark_log", "bench-runtime-shards", "")
	foundTest := false
	for _, entity := range entities {
		if entity.Kind != "test_run" {
			continue
		}
		foundTest = strings.Contains(entity.Label, "go test ./internal/runtime")
		if foundTest {
			if entity.Display.OpenSurface != "source_window" || entity.Target.PublicRecordID == "" {
				t.Fatalf("test_run source did not retain source-window identity: %#v", entity)
			}
			break
		}
	}
	if !foundTest {
		t.Fatalf("missing test_run entity with command label: %#v", entities)
	}
	for _, entity := range entities {
		if entity.Target.PublicRecordID == "prose-only" {
			t.Fatalf("ordinary findings prose was scraped into source entity: %#v", entities)
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

	sources := coagentSourcesFromTypedEvidenceRefs([]string{"source_service_item:srcitem_native_panel"})
	update := types.CoagentSourcePacket{
		UpdateID:      "update-native-source-refs",
		OwnerID:       ownerID,
		AgentID:       "researcher:native-sources",
		TargetAgentID: targetAgentID,
		ChannelID:     docID,
		Role:          agentprofile.Researcher,
		Packet:        newCoagentPacket("evidence_update", "native source refs ready", coagentClaimsFromTexts([]string{"The source-backed finding is ready."}, sources), sources, nil, nil, nil),
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
		AgentProfile: agentprofile.Texture,
		AgentRole:    agentprofile.Texture,
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
		!messageTextContains(t, msgs[0], "Texture source entities/transclusion refs") ||
		!messageTextContains(t, msgs[0], "Do not write ordinary URL links") {
		t.Fatalf("coagent packet missing native source entity fields: %s", string(msgs[0]))
	}
	if !hasSourceEntity(decodeAvailableTextureSourceEntities(rec.Metadata), "source_service_item", "srcitem_native_panel", "") {
		t.Fatalf("run metadata missing injected available source entities: %#v", rec.Metadata[textureAvailableSourceEntitiesKey])
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
	if _, ok := meta["source_entities"]; ok {
		t.Fatalf("revision metadata retained legacy source_entities: %#v", meta["source_entities"])
	}
	if _, ok := meta[textureAvailableSourceEntitiesKey]; ok {
		t.Fatalf("revision metadata retained run source context: %#v", meta[textureAvailableSourceEntitiesKey])
	}
}

func TestTextureCoagentEvidenceSummarySourceCanPatchWithNativeCitation(t *testing.T) {
	t.Parallel()
	rt, _ := testAPISetup(t)
	if err := rt.InstallDefaultAgentTools(t.TempDir()); err != nil {
		t.Fatalf("InstallDefaultAgentTools: %v", err)
	}
	s := rt.Store()
	ctx := context.Background()
	now := time.Now().UTC()
	ownerID := "user-summary-source"
	docID := "doc-summary-source"
	targetAgentID := currentTextureAgentID(docID)
	contentID := "content-openai-docs"
	evidenceID := "ev-openai-summary"

	if err := s.CreateContentItem(ctx, types.ContentItem{
		ContentID:    contentID,
		OwnerID:      ownerID,
		SourceType:   "source_search",
		MediaType:    "text/html",
		Title:        "OpenAI GPT-5.5 docs",
		SourceURL:    "https://developers.openai.com/api/docs/models/gpt-5.5",
		CanonicalURL: "https://developers.openai.com/api/docs/models/gpt-5.5",
		TextContent:  "Model aliases include gpt-5.5 and gpt-5.5-2026-04-23.",
		ContentHash:  "hash-openai-docs",
		CreatedAt:    now,
		UpdatedAt:    now,
	}); err != nil {
		t.Fatalf("CreateContentItem: %v", err)
	}
	if err := s.CreateEvidence(ctx, types.EvidenceRecord{
		EvidenceID: evidenceID,
		OwnerID:    ownerID,
		AgentID:    "researcher:summary-source",
		Kind:       "source_excerpt",
		Title:      "OpenAI GPT-5.5 docs evidence",
		SourceURI:  "https://developers.openai.com/api/docs/models/gpt-5.5",
		Content:    "Researcher synthesis: OpenAI's API docs identify GPT-5.5 as the current public frontier model.",
		Metadata:   json.RawMessage(`{"content_id":"content-openai-docs"}`),
		CreatedAt:  now,
	}); err != nil {
		t.Fatalf("CreateEvidence: %v", err)
	}

	doc := types.Document{
		DocID:   docID,
		OwnerID: ownerID,
		Title:   "Summary source",
	}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}
	parent := types.Revision{
		RevisionID: "rev-summary-source-v0",
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

	sources := coagentSourcesFromTypedEvidenceRefs([]string{"evidence_id:" + evidenceID})
	update := types.CoagentSourcePacket{
		UpdateID:      "update-summary-source",
		OwnerID:       ownerID,
		AgentID:       "researcher:summary-source",
		TargetAgentID: targetAgentID,
		ChannelID:     docID,
		Role:          agentprofile.Researcher,
		Packet:        newCoagentPacket("evidence_update", "source evidence ready", coagentClaimsFromTexts([]string{"OpenAI GPT-5.5 public release evidence is ready."}, sources), sources, nil, nil, nil),
		Content:       "Use the OpenAI docs source evidence.",
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
	if _, _, err := s.DispatchWorkerUpdate(ctx, update, &message); err != nil {
		t.Fatalf("DispatchWorkerUpdate: %v", err)
	}

	rec := &types.RunRecord{
		RunID:        "run-summary-source",
		OwnerID:      ownerID,
		AgentID:      targetAgentID,
		AgentProfile: agentprofile.Texture,
		AgentRole:    agentprofile.Texture,
		ChannelID:    docID,
		CreatedAt:    now,
		UpdatedAt:    now,
		Metadata: map[string]any{
			"type":                textureAgentRevisionTaskType,
			"request_source":      "update_coagent",
			"doc_id":              docID,
			"current_revision_id": parent.RevisionID,
			runMetadataAgentID:    targetAgentID,
			runMetadataChannelID:  docID,
		},
	}
	inject := rt.coagentUpdateTurnInjector(rec)
	if inject == nil {
		t.Fatal("Texture coagent update injector is nil")
	}
	if _, err := inject(false); err != nil {
		t.Fatalf("inject coagent update: %v", err)
	}
	rt.createAgentMutationForRun(ctx, rec)
	sourceEntities := decodeAvailableTextureSourceEntities(rec.Metadata)
	entityID := stableSourceEntityID("content_item", contentID)
	if !hasSourceEntity(sourceEntities, "content_item", "", contentID) {
		t.Fatalf("run metadata missing evidence-derived content source: %#v", sourceEntities)
	}
	for _, entity := range sourceEntities {
		if entity.EntityID != entityID {
			continue
		}
		if len(entity.Selectors) != 1 || entity.Selectors[0].SelectorKind != "whole_resource" {
			t.Fatalf("summary evidence source should use whole_resource selector: %#v", entity)
		}
	}

	editArgs, err := json.Marshal(map[string]any{
		"doc_id":           docID,
		"base_revision_id": parent.RevisionID,
		"edits": []map[string]any{
			{
				"op":       "update_block_text",
				"block_id": "p-" + docID + "-" + parent.RevisionID + "-0",
				"text":     "OpenAI documentation supports the GPT-5.5 release.",
			},
			{
				"op":               "insert_source_ref",
				"block_id":         "p-" + docID + "-" + parent.RevisionID + "-0",
				"source_entity_id": entityID,
			},
		},
		"rationale": "Incorporate researcher source evidence with a native source citation.",
	})
	if err != nil {
		t.Fatalf("marshal texture edit args: %v", err)
	}
	if _, err := rt.ToolRegistryForProfile(agentprofile.Texture).Execute(toolregistry.WithExecutionContext(ctx, toolExecutionContextForRun(rec)), "patch_texture", editArgs); err != nil {
		t.Fatalf("patch_texture should accept whole_resource source citation: %v", err)
	}
	updated, err := s.GetDocument(ctx, docID, ownerID)
	if err != nil {
		t.Fatalf("GetDocument: %v", err)
	}
	rev, err := s.GetRevision(ctx, updated.CurrentRevisionID, ownerID)
	if err != nil {
		t.Fatalf("GetRevision: %v", err)
	}
	if !strings.Contains(rev.Content, "[1]") || strings.Contains(rev.Content, "](source:") {
		t.Fatalf("revision missing native source citation: %q", rev.Content)
	}
	meta := decodeRevisionMetadata(rev.Metadata)
	if _, ok := meta["source_entities"]; ok {
		t.Fatalf("revision metadata retained legacy source_entities: %#v", meta["source_entities"])
	}
	var structuredEntities []texturedoc.SourceEntity
	if err := json.Unmarshal(rev.SourceEntities, &structuredEntities); err != nil {
		t.Fatalf("unmarshal revision source_entities: %v", err)
	}
	if len(structuredEntities) != 1 || structuredEntities[0].SourceEntityID != entityID {
		t.Fatalf("revision source_entities = %#v, want %q", structuredEntities, entityID)
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
