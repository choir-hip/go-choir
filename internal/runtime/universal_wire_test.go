package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/sourceapi"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestHandleUniversalWireStoriesReturnsHonestEmptyState(t *testing.T) {
	_, handler := testAPISetup(t)

	w := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/universal-wire/stories", "", "user-universal-wire")
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/universal-wire/stories status = %d body=%s", w.Code, w.Body.String())
	}
	var resp universalWireStoriesResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode stories response: %v", err)
	}
	if resp.Source != "universal-wire-texture-index" {
		t.Fatalf("source = %q, want universal-wire-texture-index", resp.Source)
	}
	if len(resp.Stories) != 0 {
		t.Fatalf("stories length = %d, want no seeded stories", len(resp.Stories))
	}
	if len(resp.StyleSources) != 0 {
		t.Fatalf("style_sources length = %d, want no seeded style sources", len(resp.StyleSources))
	}
}

func seedPlatformSourceNetworkTextureFixtureWithPublishState(t *testing.T, handler *APIHandler, docID string, published bool) types.Document {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()
	doc := types.Document{
		DocID:     docID,
		OwnerID:   "universal-wire-platform",
		Title:     "Madrid dispatch.vtext",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := handler.rt.Store().CreateDocument(ctx, doc); err != nil {
		t.Fatalf("create platform source maxx doc: %v", err)
	}
	metaMap := map[string]any{
		"source":                         "edit_texture",
		"revision_role":                  vtextRevisionRoleCanonical,
		"ingestion_handoff_cycle_id":     "cycle-live",
		"ingestion_handoff_request_id":   "reconciler-live",
		"ingestion_handoff_request_kind": "reconciler",
		"selected_style_sources":         []map[string]any{{"title": "Style.vtext: Universal Wire"}},
		"selected_style_rationale":       "Universal Wire style fits a fast sourced dispatch.",
		"source_item_ids":                []string{"srcitem_live_1", "srcitem_live_2"},
	}
	if published {
		metaMap["platformd_route_path"] = "/pub/texture/madrid-dispatch"
	}
	meta, _ := json.Marshal(metaMap)
	rev := types.Revision{
		RevisionID:  "rev-" + docID,
		DocID:       doc.DocID,
		OwnerID:     doc.OwnerID,
		AuthorKind:  types.AuthorAppAgent,
		AuthorLabel: "vtext:" + doc.DocID,
		Content: strings.Join([]string{
			"# Madrid dispatch",
			"",
			"MADRID -- Pope Leo XIV addressed a packed crowd while city officials adjusted transport and security plans around the visit.",
			"",
			"The article keeps the sourcing narrow: official crowd-control notices, local transit updates, and source-network context remain separate from commentary.",
		}, "\n"),
		Citations: json.RawMessage("[]"),
		Metadata:  meta,
		CreatedAt: now,
	}
	if err := handler.rt.Store().CreateRevision(ctx, rev); err != nil {
		t.Fatalf("create platform source maxx revision: %v", err)
	}
	return doc
}

func seedPlatformSourceNetworkTextureFixture(t *testing.T, handler *APIHandler, docID string) types.Document {
	return seedPlatformSourceNetworkTextureFixtureWithPublishState(t, handler, docID, true)
}

func seedUniversalWireEditionFixture(t *testing.T, handler *APIHandler, includedDocIDs ...string) types.Document {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()
	doc := types.Document{
		DocID:     "doc-universal-wire-edition",
		OwnerID:   "universal-wire-platform",
		Title:     "Wire.vtext",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := handler.rt.Store().CreateDocument(ctx, doc); err != nil {
		t.Fatalf("create Universal Wire edition doc: %v", err)
	}
	lines := []string{"# Wire", "", "Universal Wire edition."}
	for _, docID := range includedDocIDs {
		lines = append(lines, "", fmt.Sprintf("- [Article](vtext:%s)", docID))
	}
	if err := handler.rt.Store().CreateRevision(ctx, types.Revision{
		RevisionID:  "rev-universal-wire-edition",
		DocID:       doc.DocID,
		OwnerID:     doc.OwnerID,
		AuthorKind:  types.AuthorAppAgent,
		AuthorLabel: "vtext:" + doc.DocID,
		Content:     strings.Join(lines, "\n"),
		Citations:   json.RawMessage("[]"),
		Metadata:    json.RawMessage(`{"source":"universal_wire_edition"}`),
		CreatedAt:   now,
	}); err != nil {
		t.Fatalf("create Universal Wire edition revision: %v", err)
	}
	if err := handler.rt.Store().UpsertDocumentAlias(ctx, doc.OwnerID, universalWireEditionSourcePath, doc.DocID, now); err != nil {
		t.Fatalf("upsert Universal Wire edition alias: %v", err)
	}
	return doc
}

func TestHandleUniversalWireStoriesDoesNotIndexUntranscludedPlatformTextures(t *testing.T) {
	_, handler := testAPISetup(t)
	doc := seedPlatformSourceNetworkTextureFixture(t, handler, "doc-source-network-live")

	w := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/universal-wire/stories", "", "user-universal-wire")
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/universal-wire/stories status = %d body=%s", w.Code, w.Body.String())
	}
	var resp universalWireStoriesResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode stories response: %v", err)
	}
	if resp.Source != "universal-wire-texture-index" {
		t.Fatalf("source = %q, want source network texture index", resp.Source)
	}
	if len(resp.Stories) != 0 {
		t.Fatalf("stories length = %d, want no untranscluded platform Textures: %+v", len(resp.Stories), resp.Stories)
	}
	if resp.Edition != nil {
		t.Fatalf("edition = %+v, want no edition without %s alias", resp.Edition, universalWireEditionSourcePath)
	}
	if doc.DocID == "" {
		t.Fatal("fixture doc id should not be empty")
	}
}

func TestHandleUniversalWireStoriesIndexesEditionTranscludedTextureHeads(t *testing.T) {
	_, handler := testAPISetup(t)
	doc := seedPlatformSourceNetworkTextureFixture(t, handler, "doc-source-network-live")
	edition := seedUniversalWireEditionFixture(t, handler, doc.DocID)

	w := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/universal-wire/stories", "", "user-universal-wire")
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/universal-wire/stories status = %d body=%s", w.Code, w.Body.String())
	}
	var resp universalWireStoriesResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode stories response: %v", err)
	}
	if resp.Source != "universal-wire-edition-texture" {
		t.Fatalf("source = %q, want edition Texture index", resp.Source)
	}
	if resp.Edition == nil || resp.Edition.DocID != edition.DocID || resp.Edition.SourcePath != universalWireEditionSourcePath {
		t.Fatalf("edition = %+v, want %s", resp.Edition, universalWireEditionSourcePath)
	}
	if !slices.Equal(resp.Edition.IncludedDocIDs, []string{doc.DocID}) {
		t.Fatalf("edition included docs = %+v, want %s", resp.Edition.IncludedDocIDs, doc.DocID)
	}
	if len(resp.Stories) != 1 {
		t.Fatalf("stories length = %d, want only edition-transcluded source Texture story", len(resp.Stories))
	}
	story := resp.Stories[0]
	if story.ID != "source-network-texture-"+doc.DocID ||
		story.OwnerID != "universal-wire-platform" ||
		story.StoryTextureDoc != doc.DocID ||
		story.TextureContent == "" {
		t.Fatalf("first story is not the indexed source-network Texture: %+v", story)
	}
	storyJSON, err := json.Marshal(story)
	if err != nil {
		t.Fatalf("marshal indexed story: %v", err)
	}
	if !strings.Contains(string(storyJSON), `"story_texture_doc_id"`) ||
		!strings.Contains(string(storyJSON), `"texture_content"`) ||
		!strings.Contains(string(storyJSON), `"projection_texture_docs"`) ||
		strings.Contains(string(storyJSON), "story_vtext_doc_id") ||
		strings.Contains(string(storyJSON), "vtext_content") ||
		strings.Contains(string(storyJSON), "projection_vtext_docs") {
		t.Fatalf("indexed story JSON did not expose Texture projection fields only: %s", string(storyJSON))
	}
	if story.Headline != "Madrid dispatch" || !strings.Contains(story.Projections["wire-style"], "MADRID -- Pope Leo XIV") {
		t.Fatalf("indexed source-network story did not expose article head: %+v", story)
	}
	if story.Freshness != "updated just now" {
		t.Fatalf("source-network story freshness = %q, want relative update time", story.Freshness)
	}
	if story.SourceState != "universal-wire-edition-texture" {
		t.Fatalf("source state = %q, want edition Texture state", story.SourceState)
	}
	if len(story.Manifest.Lead) != 0 || len(story.Manifest.Context) != 1 ||
		story.Manifest.Context[0].ID != "source-network-cycle:cycle-live" ||
		!strings.Contains(story.Manifest.Context[0].Standing, "2 source handles retained in revision provenance") {
		t.Fatalf("indexed source-network story should expose bounded cycle provenance, got %+v", story.Manifest)
	}
	claimText := strings.Join(story.Claims, "\n")
	if strings.Contains(claimText, "Style.vtext: Universal Wire") ||
		!strings.Contains(claimText, "Source and style provenance are carried by the Texture revision metadata and citations") {
		t.Fatalf("indexed source-network story claims did not preserve provenance/body separation: %+v", story.Claims)
	}
}
func TestHandleUniversalWireStoriesUsesVisibleSourceEntitiesForSourceNetworkManifest(t *testing.T) {
	_, handler := testAPISetup(t)
	ctx := context.Background()
	now := time.Now().UTC()
	sourceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		itemID := strings.TrimPrefix(r.URL.Path, "/internal/source-service/items/")
		switch itemID {
		case "srcitem_cited_one":
			_ = json.NewEncoder(w).Encode(sourceapi.ResolveItemResponse{
				Provider: sourceapi.ProviderName,
				Item: sourceapi.ItemResult{
					TargetKind:   sourceapi.TargetKind,
					ItemID:       itemID,
					SourceID:     "rss:regional-wire",
					FetchID:      "fetch-regional-wire",
					Title:        "Regional wire confirms rail corridor reopening",
					URL:          "https://example.test/regional-wire",
					CanonicalURL: "https://example.test/regional-wire",
					ContentHash:  "hash-regional-wire",
				},
			})
		case "srcitem_uncited":
			_ = json.NewEncoder(w).Encode(sourceapi.ResolveItemResponse{
				Provider: sourceapi.ProviderName,
				Item:     sourceapi.ItemResult{TargetKind: sourceapi.TargetKind, ItemID: itemID, Title: "Uncited cycle context"},
			})
		default:
			t.Fatalf("unexpected source service item path %s", r.URL.Path)
		}
	}))
	defer sourceServer.Close()
	t.Setenv("SOURCE_SERVICE_BASE_URL", sourceServer.URL)
	t.Setenv("SOURCE_SERVICE_URL", "")
	t.Setenv("SOURCECYCLED_API_URL", "")
	doc := types.Document{
		DocID:     "doc-source-network-scoped-sources",
		OwnerID:   "universal-wire-platform",
		Title:     "Scoped sources.vtext",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := handler.rt.Store().CreateDocument(ctx, doc); err != nil {
		t.Fatalf("create platform source-network doc: %v", err)
	}
	meta, _ := json.Marshal(map[string]any{
		"source":                         "edit_texture",
		"revision_role":                  vtextRevisionRoleCanonical,
		"ingestion_handoff_cycle_id":     "cycle-scoped",
		"ingestion_handoff_request_id":   "reconciler-scoped",
		"ingestion_handoff_request_kind": "reconciler",
		"selected_style_sources":         []map[string]any{{"title": "Style.vtext: Universal Wire"}},
		"platformd_route_path":           "/pub/texture/scoped-sources",
		"source_item_ids":                []string{"srcitem_cycle_1", "srcitem_cycle_2", "srcitem_cycle_3", "srcitem_cycle_4"},
		"source_entities": []map[string]any{
			{
				"entity_id": "src_cited_one",
				"kind":      "source_service_item",
				"label":     "Source Service item srcitem_cited_one",
				"target":    map[string]any{"target_kind": "source_service_item", "item_id": "srcitem_cited_one"},
			},
			{
				"entity_id": "src_cited_two",
				"kind":      "content_item",
				"label":     "Local emergency notice",
				"target":    map[string]any{"target_kind": "content_item", "content_id": "content-cited-two"},
			},
			{
				"entity_id": "src_uncited",
				"kind":      "source_service_item",
				"label":     "Uncited cycle context",
				"target":    map[string]any{"target_kind": "source_service_item", "item_id": "srcitem_uncited"},
			},
		},
	})
	rev := types.Revision{
		RevisionID:  "rev-source-network-scoped-sources",
		DocID:       doc.DocID,
		OwnerID:     doc.OwnerID,
		AuthorKind:  types.AuthorAppAgent,
		AuthorLabel: "vtext:doc-source-network-scoped-sources",
		Content: strings.Join([]string{
			"# Scoped sources",
			"",
			"**Published:** [Date TBD] | **Source:** internal handoff",
			"",
			"PARIS -- Emergency crews reopened the rail corridor after overnight flooding, with regional authorities saying inspections will continue through the afternoon [wire](source:src_cited_one).",
			"",
			"Local notices still warn commuters to expect rolling delays while crews clear debris from the lowest platforms [source:src_cited_two].",
			"",
			"## Source Handles",
			"",
			"- [Uncited cycle context](source:src_uncited)",
			"",
			"## Style.vtext Source",
			"",
			"Selection rationale: Universal Wire style.",
		}, "\n"),
		Citations: json.RawMessage("[]"),
		Metadata:  meta,
		CreatedAt: now,
	}
	if err := handler.rt.Store().CreateRevision(ctx, rev); err != nil {
		t.Fatalf("create platform source-network revision: %v", err)
	}
	seedUniversalWireEditionFixture(t, handler, doc.DocID)

	w := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/universal-wire/stories", "", "user-universal-wire")
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/universal-wire/stories status = %d body=%s", w.Code, w.Body.String())
	}
	var resp universalWireStoriesResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode stories response: %v", err)
	}
	if resp.Source != "universal-wire-edition-texture" || resp.Edition == nil {
		t.Fatalf("response did not use Universal Wire edition: source=%q edition=%+v", resp.Source, resp.Edition)
	}
	if len(resp.Stories) != 1 {
		t.Fatalf("stories length = %d, want edition-transcluded Texture story: %+v", len(resp.Stories), resp.Stories)
	}
	story := resp.Stories[0]
	if story.ID != "source-network-texture-"+doc.DocID {
		t.Fatalf("first story = %q, want scoped source-network doc", story.ID)
	}
	if strings.Contains(story.Dek, "Published:") || strings.Contains(story.Dek, "Source:") || !strings.Contains(story.Dek, "Emergency crews reopened") {
		t.Fatalf("dek leaked scaffolding or missed article prose: %q", story.Dek)
	}
	if len(story.Manifest.Lead) != 2 || len(story.Manifest.Context) != 0 {
		t.Fatalf("manifest should use only cited source entities, got %+v", story.Manifest)
	}
	if story.Manifest.Lead[0].ID != "srcitem_cited_one" || story.Manifest.Lead[1].ID != "content-cited-two" {
		t.Fatalf("manifest did not expose cited source entity ids: %+v", story.Manifest.Lead)
	}
	if story.Manifest.Lead[0].Title != "Regional wire confirms rail corridor reopening" ||
		story.Manifest.Lead[0].SourceID != "rss:regional-wire" ||
		story.Manifest.Lead[0].CanonicalURL != "https://example.test/regional-wire" {
		t.Fatalf("manifest did not resolve source-service item metadata: %+v", story.Manifest.Lead[0])
	}
}
func TestHandleUniversalWireStoriesSkipsTranscludedUnpublishedPlatformTextures(t *testing.T) {
	_, handler := testAPISetup(t)
	doc := seedPlatformSourceNetworkTextureFixtureWithPublishState(t, handler, "doc-source-network-unpublished", false)
	seedUniversalWireEditionFixture(t, handler, doc.DocID)

	w := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/universal-wire/stories", "", "user-universal-wire")
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/universal-wire/stories status = %d body=%s", w.Code, w.Body.String())
	}
	var resp universalWireStoriesResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Stories) != 0 {
		t.Fatalf("stories len = %d, want 0 for unpublished transcluded doc", len(resp.Stories))
	}
	if resp.Source != "universal-wire-edition-texture" {
		t.Fatalf("source = %q, want edition source", resp.Source)
	}
}

func TestHandleUniversalWireStoriesRequiresAuth(t *testing.T) {
	_, handler := testAPISetup(t)

	w := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/universal-wire/stories", "", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unauth story status = %d body=%s", w.Code, w.Body.String())
	}
}

func TestResolveUniversalWireTextureReadOwnerAllowsEditionTranscludedPlatformDoc(t *testing.T) {
	_, handler := testAPISetup(t)
	doc := seedPlatformSourceNetworkTextureFixture(t, handler, "doc-wire-read-through")
	seedUniversalWireEditionFixture(t, handler, doc.DocID)

	ctx := context.Background()
	owner, err := handler.resolveUniversalWireTextureReadOwner(ctx, "user-universal-wire", doc.DocID)
	if err != nil {
		t.Fatalf("resolveUniversalWireTextureReadOwner: %v", err)
	}
	if owner != "universal-wire-platform" {
		t.Fatalf("owner = %q, want universal-wire-platform", owner)
	}

	w := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/texture/documents/"+doc.DocID, "", "user-universal-wire")
	if w.Code != http.StatusOK {
		t.Fatalf("GET platform wire document status = %d body=%s", w.Code, w.Body.String())
	}
}

func TestNormalizeWireArticleSourceServiceProseRewritesBareLabels(t *testing.T) {
	itemID := "srcitem_el_nino"
	entityID := stableSourceEntityID("source_service_item", itemID)
	content := "Forecasters warned Source Service item " + itemID + " that El Niño odds rose."
	meta, _ := json.Marshal(map[string]any{
		"source_entities": []map[string]any{{
			"entity_id": entityID,
			"kind":      "source_service_item",
			"label":     "WMO El Niño bulletin",
			"target":    map[string]any{"target_kind": "source_service_item", "item_id": itemID},
		}},
	})
	rec := &types.RunRecord{
		OwnerID: "universal-wire-platform",
		Metadata: map[string]any{
			"request_intent":             "integrate_worker_findings",
			"type":                       "vtext_agent_revision",
			"ingestion_handoff_cycle_id": "cycle-el-nino",
		},
	}
	normalized, count, entities := normalizeWireArticleSourceServiceProse(content, meta, rec)
	if count != 1 {
		t.Fatalf("normalized count = %d, want 1", count)
	}
	want := "[WMO El Niño bulletin](source:" + entityID + ")"
	if !strings.Contains(normalized, want) {
		t.Fatalf("normalized content = %q, want %q", normalized, want)
	}
	if len(entities) != 1 || entities[0].EntityID != entityID {
		t.Fatalf("entities = %#v, want preserved entity %s", entities, entityID)
	}
}
