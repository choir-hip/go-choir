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

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/sourceapi"
	"github.com/yusefmosiah/go-choir/internal/texturedoc"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func runtimeTestTextureBodyDoc(t *testing.T, docID, revisionID, content string) json.RawMessage {
	t.Helper()
	doc := plainStructuredTextureToolDoc(docID, revisionID, content)
	bodyDoc, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal test body_doc: %v", err)
	}
	return bodyDoc
}

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
		Title:     "Madrid dispatch.texture",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := handler.rt.Store().CreateDocument(ctx, doc); err != nil {
		t.Fatalf("create platform source maxx doc: %v", err)
	}
	metaMap := map[string]any{
		"source":                         "edit_texture",
		"revision_role":                  textureRevisionRoleCanonical,
		"ingestion_handoff_cycle_id":     "cycle-live",
		"ingestion_handoff_request_id":   "reconciler-live",
		"ingestion_handoff_request_kind": "reconciler",
		"selected_style_sources":         []map[string]any{{"title": "Style.texture: Universal Wire"}},
		"selected_style_rationale":       "Universal Wire style fits a fast sourced dispatch.",
		"source_item_ids":                []string{"srcitem_live_1", "srcitem_live_2"},
	}
	if published {
		metaMap["platformd_route_path"] = "/pub/texture/madrid-dispatch"
	}
	meta, _ := json.Marshal(metaMap)
	content := strings.Join([]string{
		"# Madrid dispatch",
		"",
		"MADRID -- Pope Leo XIV addressed a packed crowd while city officials adjusted transport and security plans around the visit.",
		"",
		"The article keeps the sourcing narrow: official crowd-control notices, local transit updates, and source-network context remain separate from commentary.",
	}, "\n")
	rev := types.Revision{
		RevisionID:  "rev-" + docID,
		DocID:       doc.DocID,
		OwnerID:     doc.OwnerID,
		AuthorKind:  types.AuthorAppAgent,
		AuthorLabel: "texture:" + doc.DocID,
		Content:     content,
		BodyDoc:     runtimeTestTextureBodyDoc(t, doc.DocID, "rev-"+docID, content),
		Citations:   json.RawMessage("[]"),
		Metadata:    meta,
		CreatedAt:   now,
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
		Title:     "Wire.texture",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := handler.rt.Store().CreateDocument(ctx, doc); err != nil {
		t.Fatalf("create Universal Wire edition doc: %v", err)
	}
	lines := []string{"# Wire", "", "Universal Wire edition."}
	for _, docID := range includedDocIDs {
		lines = append(lines, "", fmt.Sprintf("- [Article](texture:%s)", docID))
	}
	content := strings.Join(lines, "\n")
	if err := handler.rt.Store().CreateRevision(ctx, types.Revision{
		RevisionID:  "rev-universal-wire-edition",
		DocID:       doc.DocID,
		OwnerID:     doc.OwnerID,
		AuthorKind:  types.AuthorAppAgent,
		AuthorLabel: "texture:" + doc.DocID,
		Content:     content,
		BodyDoc:     runtimeTestTextureBodyDoc(t, doc.DocID, "rev-universal-wire-edition", content),
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

func seedUniversalWireWebCaptureFixture(t *testing.T, handler *APIHandler, title, url, body string, fetchedAt time.Time) objectgraph.Object {
	t.Helper()
	graph := handler.rt.ObjectGraph()
	if graph == nil {
		t.Fatal("runtime objectgraph service is unavailable")
	}
	capture, err := graph.CreateWebCapture(context.Background(), objectgraph.CreateWebCaptureRequest{
		OwnerID:             universalWirePlatformOwnerID(),
		ComputerID:          "computer-universal-wire-platform",
		URL:                 url,
		CanonicalURL:        url,
		Title:               title,
		FetchedAt:           fetchedAt,
		ContentBlobID:       "blob-html-" + strings.ToLower(strings.ReplaceAll(title, " ", "-")),
		ExtractedTextBlobID: "blob-text-" + strings.ToLower(strings.ReplaceAll(title, " ", "-")),
		ExtractedText:       []byte(body),
		Now:                 fetchedAt,
	})
	if err != nil {
		t.Fatalf("create web capture fixture: %v", err)
	}
	return capture
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
	seedUniversalWireWebCaptureFixture(t, handler, "Capture fallback should not win", "https://example.test/fallback", "A graph capture exists, but the edition Texture story should remain primary.", time.Now().UTC().Add(-time.Hour))

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
		!strings.Contains(string(storyJSON), `"projection_texture_docs"`) {
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
		!strings.Contains(story.Manifest.Context[0].Standing, "2 source ids retained in revision provenance") {
		t.Fatalf("indexed source-network story should expose bounded cycle provenance, got %+v", story.Manifest)
	}
	claimText := strings.Join(story.Claims, "\n")
	if strings.Contains(claimText, "Style.texture: Universal Wire") ||
		strings.Contains(claimText, "Style.texture Source") ||
		strings.Contains(claimText, "Style.texture Source") ||
		!strings.Contains(claimText, "Source and style provenance are carried by the Texture revision metadata and citations") {
		t.Fatalf("indexed source-network story claims did not preserve provenance/body separation: %+v", story.Claims)
	}
}

func TestHandleUniversalWireStoriesFallsBackToGraphBackedWebCaptures(t *testing.T) {
	_, handler := testAPISetup(t)
	now := time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC)
	older := seedUniversalWireWebCaptureFixture(t, handler,
		"Regional harbor notice",
		"https://example.test/harbor",
		"PORTO -- Harbor pilots reopened the inner channel after overnight inspections.\n\nOfficials said the next update will follow the afternoon tide window.",
		now.Add(-2*time.Hour))
	newer := seedUniversalWireWebCaptureFixture(t, handler,
		"Rail corridor reopens",
		"https://example.test/rail",
		"PARIS -- Emergency crews reopened the rail corridor after flooding, with regional authorities saying inspections will continue through the afternoon.",
		now)

	w := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/universal-wire/stories", "", "user-universal-wire")
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/universal-wire/stories status = %d body=%s", w.Code, w.Body.String())
	}
	var resp universalWireStoriesResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode stories response: %v", err)
	}
	if resp.Source != "universal-wire-web-capture-graph" {
		t.Fatalf("source = %q, want graph-backed web capture source", resp.Source)
	}
	if resp.Edition != nil {
		t.Fatalf("edition = %+v, want no Texture edition for graph fallback", resp.Edition)
	}
	if len(resp.Stories) != 2 {
		t.Fatalf("stories length = %d, want graph-backed captures: %+v", len(resp.Stories), resp.Stories)
	}
	story := resp.Stories[0]
	if story.ID != "web-capture-"+newer.CanonicalID ||
		story.OwnerID != universalWirePlatformOwnerID() ||
		story.Headline != "Rail corridor reopens" ||
		story.StoryTextureDoc != "" ||
		story.PlatformRoutePath != "" ||
		story.SourceState != "objectgraph-web-capture" {
		t.Fatalf("first story is not the newest graph-backed capture projection: %+v", story)
	}
	if !strings.Contains(story.Dek, "Emergency crews reopened") ||
		!strings.Contains(story.Projections["wire-style"], "regional authorities") {
		t.Fatalf("graph-backed capture text was not projected into the Wire card: %+v", story)
	}
	if len(story.Manifest.Lead) != 1 ||
		story.Manifest.Lead[0].ID != newer.CanonicalID ||
		story.Manifest.Lead[0].CanonicalURL != "https://example.test/rail" ||
		story.Manifest.Lead[0].Standing != "graph-backed web capture" {
		t.Fatalf("graph-backed capture manifest = %+v, want durable capture identity and canonical URL", story.Manifest)
	}
	if resp.Stories[1].ID != "web-capture-"+older.CanonicalID {
		t.Fatalf("second story id = %q, want older capture %s", resp.Stories[1].ID, older.CanonicalID)
	}
	claims := strings.Join(story.Claims, "\n")
	if !strings.Contains(claims, "choir.web_capture") ||
		!strings.Contains(claims, "not a Texture article publication") {
		t.Fatalf("graph-backed capture claims did not bound the projection: %+v", story.Claims)
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
		Title:     "Scoped sources.texture",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := handler.rt.Store().CreateDocument(ctx, doc); err != nil {
		t.Fatalf("create platform source-network doc: %v", err)
	}
	sourceEntities := []texturedoc.SourceEntity{
		{
			SourceEntityID: "src_cited_one",
			Target:         texturedoc.SourceTarget{Kind: "source_service_item", ID: "srcitem_cited_one"},
			Selectors:      []texturedoc.SourceSelector{{Kind: "whole_resource"}},
			Display:        texturedoc.SourceDisplay{Mode: "numbered_ref", Title: "Source Service item srcitem_cited_one"},
			Evidence:       texturedoc.SourceEvidence{State: "available", OpenSurface: "source"},
			Provenance:     texturedoc.SourceEntityProvenance{CreatedBy: "test"},
		},
		{
			SourceEntityID: "src_cited_two",
			Target:         texturedoc.SourceTarget{Kind: "content_item", ID: "content-cited-two"},
			Selectors:      []texturedoc.SourceSelector{{Kind: "whole_resource"}},
			Display:        texturedoc.SourceDisplay{Mode: "numbered_ref", Title: "Local emergency notice"},
			Evidence:       texturedoc.SourceEvidence{State: "available", OpenSurface: "source"},
			Provenance:     texturedoc.SourceEntityProvenance{CreatedBy: "test"},
		},
		{
			SourceEntityID: "src_uncited",
			Target:         texturedoc.SourceTarget{Kind: "source_service_item", ID: "srcitem_uncited"},
			Selectors:      []texturedoc.SourceSelector{{Kind: "whole_resource"}},
			Display:        texturedoc.SourceDisplay{Mode: "numbered_ref", Title: "Uncited cycle context"},
			Evidence:       texturedoc.SourceEvidence{State: "available", OpenSurface: "source"},
			Provenance:     texturedoc.SourceEntityProvenance{CreatedBy: "test"},
		},
	}
	bodyDoc, _ := json.Marshal(texturedoc.StructuredTextureDoc{
		Schema: texturedoc.SchemaV1,
		Doc: texturedoc.Node{
			Type:  "doc",
			Attrs: map[string]any{"id": "doc-source-network-scoped-sources-root"},
			Content: []texturedoc.Node{
				{Type: "heading", Attrs: map[string]any{"id": "h-scoped-sources", "level": 1}, Content: []texturedoc.Node{{Type: "text", Text: "Scoped sources"}}},
				{Type: "paragraph", Attrs: map[string]any{"id": "p-scoped-meta"}, Content: []texturedoc.Node{{Type: "text", Text: "Published: Date TBD | Source: internal handoff"}}},
				{Type: "paragraph", Attrs: map[string]any{"id": "p-scoped-lead"}, Content: []texturedoc.Node{
					{Type: "text", Text: "PARIS -- Emergency crews reopened the rail corridor after overnight flooding, with regional authorities saying inspections will continue through the afternoon "},
					{Type: "source_ref", Attrs: map[string]any{"id": "ref-cited-one", "source_entity_id": "src_cited_one", "display_mode": "numbered_ref"}},
					{Type: "text", Text: "."},
				}},
				{Type: "paragraph", Attrs: map[string]any{"id": "p-scoped-delay"}, Content: []texturedoc.Node{
					{Type: "text", Text: "Local notices still warn commuters to expect rolling delays while crews clear debris from the lowest platforms "},
					{Type: "source_ref", Attrs: map[string]any{"id": "ref-cited-two", "source_entity_id": "src_cited_two", "display_mode": "numbered_ref"}},
					{Type: "text", Text: "."},
				}},
				{Type: "heading", Attrs: map[string]any{"id": "h-source-handles", "level": 2}, Content: []texturedoc.Node{{Type: "text", Text: "Source Handles"}}},
				{Type: "paragraph", Attrs: map[string]any{"id": "p-source-handles"}, Content: []texturedoc.Node{
					{Type: "text", Text: "Uncited cycle context "},
					{Type: "source_ref", Attrs: map[string]any{"id": "ref-uncited", "source_entity_id": "src_uncited", "display_mode": "numbered_ref"}},
				}},
				{Type: "heading", Attrs: map[string]any{"id": "h-style-source", "level": 2}, Content: []texturedoc.Node{{Type: "text", Text: "Style.texture Source"}}},
				{Type: "paragraph", Attrs: map[string]any{"id": "p-style-source"}, Content: []texturedoc.Node{{Type: "text", Text: "Selection rationale: Universal Wire style."}}},
				{Type: "heading", Attrs: map[string]any{"id": "h-style-source-legacy", "level": 2}, Content: []texturedoc.Node{{Type: "text", Text: "Style.texture Source"}}},
				{Type: "paragraph", Attrs: map[string]any{"id": "p-style-source-legacy"}, Content: []texturedoc.Node{{Type: "text", Text: "Legacy selection rationale that should still be stripped."}}},
			},
		},
	})
	sourceEntitiesRaw, _ := json.Marshal(sourceEntities)
	meta, _ := json.Marshal(map[string]any{
		"source":                         "edit_texture",
		"revision_role":                  textureRevisionRoleCanonical,
		"ingestion_handoff_cycle_id":     "cycle-scoped",
		"ingestion_handoff_request_id":   "reconciler-scoped",
		"ingestion_handoff_request_kind": "reconciler",
		"selected_style_sources":         []map[string]any{{"title": "Style.texture: Universal Wire"}},
		"platformd_route_path":           "/pub/texture/scoped-sources",
		"source_item_ids":                []string{"srcitem_cycle_1", "srcitem_cycle_2", "srcitem_cycle_3", "srcitem_cycle_4"},
	})
	rev := types.Revision{
		RevisionID:     "rev-source-network-scoped-sources",
		DocID:          doc.DocID,
		OwnerID:        doc.OwnerID,
		AuthorKind:     types.AuthorAppAgent,
		AuthorLabel:    "texture:doc-source-network-scoped-sources",
		BodyDoc:        bodyDoc,
		SourceEntities: sourceEntitiesRaw,
		Citations:      json.RawMessage("[]"),
		Metadata:       meta,
		CreatedAt:      now,
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

func TestNormalizeWireArticleRevisionForReadDoesNotMintSourceLinks(t *testing.T) {
	itemID := "srcitem_el_nino"
	entityID := stableSourceEntityID("source_service_item", itemID)
	content := "Forecasters warned Source Service item " + itemID + " that El Niño odds rose."
	meta, _ := json.Marshal(map[string]any{
		"source":                     "patch_texture",
		"ingestion_handoff_cycle_id": "cycle-el-nino",
		"source_entities": []map[string]any{{
			"entity_id": entityID,
			"kind":      "source_service_item",
			"label":     "WMO El Niño bulletin",
			"target":    map[string]any{"target_kind": "source_service_item", "item_id": itemID},
		}},
	})
	rev := types.Revision{
		RevisionID: "rev-wire-legacy-source-prose",
		OwnerID:    "universal-wire-platform",
		Content:    content,
		Metadata:   meta,
	}
	normalized := normalizeWireArticleRevisionForRead(rev)
	if normalized.Content != content {
		t.Fatalf("normalized content = %q, want unchanged %q", normalized.Content, content)
	}
	if strings.Contains(normalized.Content, "](source:") || strings.Contains(normalized.Content, "[source:") {
		t.Fatalf("normalized content minted source syntax: %q", normalized.Content)
	}
	if string(normalized.Metadata) != string(meta) {
		t.Fatalf("metadata changed:\n got %s\nwant %s", normalized.Metadata, meta)
	}
}
