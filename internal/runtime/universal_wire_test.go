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
	"github.com/yusefmosiah/go-choir/internal/sourcecontract"
	"github.com/yusefmosiah/go-choir/internal/sources"
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
	if resp.Diagnostics == nil {
		t.Fatal("diagnostics = nil, want empty-feed diagnostics")
	}
	if resp.Diagnostics.Status != "empty" || len(resp.Diagnostics.Substrates) != 3 {
		t.Fatalf("diagnostics = %+v, want empty status with three substrate entries", resp.Diagnostics)
	}
	textureDiag := universalWireDiagnosticForSubstrate(resp.Diagnostics, "texture_edition")
	if textureDiag.State != "missing" || textureDiag.StoryCount != 0 || !strings.Contains(textureDiag.Reason, "Texture edition alias") {
		t.Fatalf("texture diagnostic = %+v, want missing edition alias", textureDiag)
	}
	graphDiag := universalWireDiagnosticForSubstrate(resp.Diagnostics, "web_capture_graph")
	if graphDiag.State != "empty" || graphDiag.CandidateCount != 0 || graphDiag.StoryCount != 0 || !strings.Contains(graphDiag.Reason, "No non-tombstoned") {
		t.Fatalf("graph diagnostic = %+v, want empty non-tombstoned graph captures", graphDiag)
	}
	provenanceDiag := universalWireDiagnosticForSubstrate(resp.Diagnostics, "source_provenance")
	if provenanceDiag.State != "not_applicable" || strings.Contains(strings.ToLower(provenanceDiag.Reason), "/api/agent") || strings.Contains(strings.ToLower(provenanceDiag.Reason), "/internal/") {
		t.Fatalf("source provenance diagnostic = %+v, want safe unavailable provenance reason", provenanceDiag)
	}
}

func TestHandleInternalSourcecycledWebCapturesExposeGraphCapturesAsDiagnostics(t *testing.T) {
	_, handler := testAPISetup(t)
	now := time.Date(2026, 6, 26, 18, 40, 0, 0, time.UTC)
	item := sources.Item{
		ID:           "srcitem-runtime-wire-1",
		SourceID:     "rss:test_wire",
		SourceType:   sources.SourceTypeRSS,
		FetchID:      "fetch-runtime-wire-1",
		OriginalID:   "https://example.com/runtime-wire",
		Title:        "Runtime-projected sourcecycled story",
		Body:         "Runtime endpoint should project this sourcecycled item into the platform objectgraph.",
		URL:          "https://example.com/runtime-wire",
		CanonicalURL: "https://example.com/runtime-wire",
		FetchedAt:    now,
		ContentHash:  sources.ContentHash("Runtime-projected sourcecycled story", "Runtime endpoint should project this sourcecycled item into the platform objectgraph.", "https://example.com/runtime-wire", "https://example.com/runtime-wire"),
	}
	body, err := json.Marshal(internalSourcecycledWebCapturesRequest{
		OwnerID: universalWirePlatformOwnerID(),
		Items:   []sources.Item{item},
		Now:     now.Format(time.RFC3339Nano),
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/internal/runtime/objectgraph/web-captures", strings.NewReader(string(body)))
	req.Header.Set("X-Internal-Caller", "true")
	w := httptest.NewRecorder()
	handler.HandleInternalSourcecycledWebCaptures(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("POST /internal/runtime/objectgraph/web-captures status = %d body=%s", w.Code, w.Body.String())
	}
	var projection internalSourcecycledWebCapturesResponse
	if err := json.NewDecoder(w.Body).Decode(&projection); err != nil {
		t.Fatalf("decode projection response: %v", err)
	}
	if projection.CaptureCount != 1 || projection.SourceEntityCount != 1 || projection.CapturedFromEdges != 1 {
		t.Fatalf("projection response = %+v, want one capture/source/edge", projection)
	}
	if projection.SynthesisStatus != "skipped" || projection.SynthesisSourceCount != 1 {
		t.Fatalf("projection synthesis = %+v, want skipped one-source cluster", projection)
	}

	storiesW := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/universal-wire/stories", "", "reader-1")
	if storiesW.Code != http.StatusOK {
		t.Fatalf("GET /api/universal-wire/stories status = %d body=%s", storiesW.Code, storiesW.Body.String())
	}
	var stories universalWireStoriesResponse
	if err := json.NewDecoder(storiesW.Body).Decode(&stories); err != nil {
		t.Fatalf("decode stories: %v", err)
	}
	if stories.Source != "universal-wire-texture-index" || len(stories.Stories) != 0 {
		t.Fatalf("stories source/count = %q/%d, want empty Texture article feed", stories.Source, len(stories.Stories))
	}
	if stories.Diagnostics == nil {
		t.Fatal("diagnostics = nil, want graph capture diagnostic")
	}
	graphDiag := universalWireDiagnosticForSubstrate(stories.Diagnostics, "web_capture_graph")
	if graphDiag.State != "diagnostic_only" ||
		graphDiag.CandidateCount != 1 ||
		graphDiag.StoryCount != 1 ||
		!strings.Contains(graphDiag.Reason, "does not publish raw capture projections") {
		t.Fatalf("graph diagnostic = %+v, want diagnostic-only graph capture", graphDiag)
	}
}

func countStrings(values []string, needle string) int {
	count := 0
	for _, value := range values {
		if value == needle {
			count++
		}
	}
	return count
}

func universalWireDiagnosticForSubstrate(diag *universalWireFeedDiagnostics, substrate string) universalWireFeedSubstrateDiagnostic {
	if diag == nil {
		return universalWireFeedSubstrateDiagnostic{}
	}
	for _, item := range diag.Substrates {
		if item.Substrate == substrate {
			return item
		}
	}
	return universalWireFeedSubstrateDiagnostic{}
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
		metaMap["corpusd_route_path"] = "/pub/texture/madrid-dispatch"
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

func TestCorpusdReadBaseURLPreservesSiblingDerivationAndDirectCorpusd(t *testing.T) {
	for _, key := range []string{
		"RUNTIME_CORPUSD_URL",
		"PROXY_CORPUSD_URL",
		"RUNTIME_VMCTL_URL",
		"PROXY_VMCTL_URL",
		"RUNTIME_GATEWAY_URL",
		"RUNTIME_MAILD_URL",
	} {
		t.Setenv(key, "")
	}

	t.Setenv("RUNTIME_CORPUSD_URL", "http://10.203.154.1:8082")
	if got := corpusdReadBaseURL(); got != "http://10.203.154.1:8086" {
		t.Fatalf("sibling runtime corpusd URL = %q, want derived :8086", got)
	}

	t.Setenv("RUNTIME_CORPUSD_URL", "http://127.0.0.1:8086")
	if got := corpusdReadBaseURL(); got != "http://127.0.0.1:8086" {
		t.Fatalf("direct runtime corpusd URL = %q, want direct :8086", got)
	}

	t.Setenv("RUNTIME_CORPUSD_URL", "")
	t.Setenv("RUNTIME_VMCTL_URL", "http://10.203.154.1:8083")
	if got := corpusdReadBaseURL(); got != "http://10.203.154.1:8086" {
		t.Fatalf("vmctl URL = %q, want derived :8086", got)
	}
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
	if resp.Diagnostics != nil {
		t.Fatalf("diagnostics = %+v, want omitted diagnostics for non-empty Texture edition response", resp.Diagnostics)
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

func TestHandleUniversalWireStoriesDoesNotPublishGraphBackedWebCapturesAsArticles(t *testing.T) {
	_, handler := testAPISetup(t)
	now := time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC)
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
	if resp.Source != "universal-wire-texture-index" {
		t.Fatalf("source = %q, want Texture index source until synthesis articles exist", resp.Source)
	}
	if resp.Edition != nil {
		t.Fatalf("edition = %+v, want no Texture edition", resp.Edition)
	}
	if resp.Diagnostics == nil {
		t.Fatal("diagnostics = nil, want diagnostic-only graph capture response")
	}
	if len(resp.Stories) != 0 {
		t.Fatalf("stories length = %d, want no raw capture projection articles: %+v", len(resp.Stories), resp.Stories)
	}
	graphDiag := universalWireDiagnosticForSubstrate(resp.Diagnostics, "web_capture_graph")
	if graphDiag.State != "diagnostic_only" ||
		graphDiag.CandidateCount != 1 ||
		graphDiag.StoryCount != 1 ||
		!strings.Contains(graphDiag.Reason, "Texture synthesis has not published an edition yet") {
		t.Fatalf("graph diagnostic = %+v, want diagnostic-only graph captures", graphDiag)
	}

	captureStories, captureDiagnostic, err := handler.universalWireWebCaptureStories(context.Background(), 12)
	if err != nil {
		t.Fatalf("read graph capture helper: %v", err)
	}
	if captureDiagnostic.State != "available" || captureDiagnostic.StoryCount != 1 {
		t.Fatalf("capture helper diagnostic = %+v, want available substrate stories", captureDiagnostic)
	}
	if len(captureStories) != 1 {
		t.Fatalf("capture helper stories length = %d, want graph-backed captures: %+v", len(captureStories), captureStories)
	}
	story := captureStories[0]
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
	lead := story.Manifest.Lead[0]
	if lead.ObjectKind != string(objectgraph.WebCaptureObjectKind) ||
		lead.CanonicalID != newer.CanonicalID ||
		lead.ContentHash != newer.ContentHash ||
		lead.SourceKind != sourcecontract.SourceKindWebSource ||
		lead.TargetKind != "web_url" ||
		lead.OpenSurface != sourcecontract.OpenSurfaceSource ||
		lead.LiveOpenSurface != sourcecontract.OpenSurfaceWebLens ||
		lead.ReaderArtifactState != sourcecontract.ReaderArtifactStateReady ||
		lead.ReaderSnapshot == nil ||
		!strings.Contains(lead.ReaderSnapshot.TextContent, "Emergency crews reopened the rail corridor") ||
		lead.ReaderSnapshot.SnapshotKind != "cleaned_reader_markdown" ||
		lead.ReaderSnapshot.MediaType != "text/markdown" ||
		lead.ReaderSnapshot.SourceURL != "https://example.test/rail" {
		t.Fatalf("graph-backed capture manifest did not carry source-open graph identity: %+v", lead)
	}
	storyJSON, err := json.Marshal(story)
	if err != nil {
		t.Fatalf("marshal graph-backed capture story: %v", err)
	}
	for _, want := range []string{
		`"object_kind":"choir.web_capture"`,
		`"canonical_id":"` + newer.CanonicalID + `"`,
		`"content_hash":"` + newer.ContentHash + `"`,
		`"open_surface":"source"`,
		`"live_open_surface":"web_lens"`,
		`"reader_snapshot"`,
		`"text_content":"PARIS -- Emergency crews reopened the rail corridor`,
	} {
		if !strings.Contains(string(storyJSON), want) {
			t.Fatalf("graph-backed capture story JSON missing %s: %s", want, string(storyJSON))
		}
	}
	if strings.Contains(string(storyJSON), `"source_ref"`) || strings.Contains(string(storyJSON), `"story_texture_doc_id"`) {
		t.Fatalf("graph-backed capture story JSON should not claim Texture source_ref/publication fields: %s", string(storyJSON))
	}
	claims := strings.Join(story.Claims, "\n")
	if !strings.Contains(claims, "choir.web_capture") ||
		!strings.Contains(claims, "not a Texture article publication") {
		t.Fatalf("graph-backed capture claims did not bound the projection: %+v", story.Claims)
	}
}

func TestHandleUniversalWireStoriesDiagnosticsForFilteredGraphCaptureCandidates(t *testing.T) {
	_, handler := testAPISetup(t)
	now := time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC)
	seedUniversalWireWebCaptureFixture(t, handler,
		"Empty capture body",
		"https://example.test/empty-capture",
		"",
		now)

	w := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/universal-wire/stories", "", "user-universal-wire")
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/universal-wire/stories status = %d body=%s", w.Code, w.Body.String())
	}
	var resp universalWireStoriesResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode stories response: %v", err)
	}
	if len(resp.Stories) != 0 {
		t.Fatalf("stories length = %d, want filtered empty response: %+v", len(resp.Stories), resp.Stories)
	}
	if resp.Diagnostics == nil {
		t.Fatal("diagnostics = nil, want filtered graph diagnostic")
	}
	graphDiag := universalWireDiagnosticForSubstrate(resp.Diagnostics, "web_capture_graph")
	if graphDiag.State != "filtered" || graphDiag.CandidateCount != 1 || graphDiag.StoryCount != 0 || graphDiag.FilteredCount != 1 {
		t.Fatalf("graph diagnostic = %+v, want one filtered graph candidate", graphDiag)
	}
	if strings.Contains(strings.ToLower(graphDiag.Reason), "sqlite") ||
		strings.Contains(graphDiag.Reason, "/") {
		t.Fatalf("graph diagnostic leaked internal detail: %+v", graphDiag)
	}
}

func TestHandleUniversalWireStoriesCarriesCapturedFromSourceEntityContext(t *testing.T) {
	_, handler := testAPISetup(t)
	ctx := context.Background()
	now := time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC)
	capture := seedUniversalWireWebCaptureFixture(t, handler,
		"River watch lifted",
		"https://example.test/river-watch",
		"GENEVA -- Emergency officials lifted the river watch after gauges fell below the alert threshold.",
		now)
	graph := handler.rt.ObjectGraph()
	sourceEntity, err := graph.CreateObject(ctx, objectgraph.CreateObjectRequest{
		Kind:       "choir.source_entity",
		OwnerID:    universalWirePlatformOwnerID(),
		ComputerID: "computer-universal-wire-platform",
		Body:       []byte("Emergency officials lifted the river watch after gauges fell below the alert threshold."),
		Metadata: map[string]any{
			"schema_version": "choir.source_entity.v1",
			"source_kind":    sourcecontract.SourceKindSourceServiceItem,
			"target": map[string]any{
				"target_kind":   sourcecontract.SourceKindSourceServiceItem,
				"item_id":       "srcitem-river-watch",
				"source_id":     "rss:city-alerts",
				"fetch_id":      "fetch-river-watch",
				"url":           "https://example.test/river-watch",
				"canonical_url": "https://example.test/river-watch",
			},
			"display": map[string]any{
				"title": "City alerts river watch bulletin",
				"url":   "https://example.test/river-watch",
			},
			"evidence": map[string]any{
				"state":                 sourcecontract.EvidenceStateAvailable,
				"reader_snapshot":       true,
				"default_open_surface":  sourcecontract.OpenSurfaceSource,
				"explicit_live_surface": sourcecontract.OpenSurfaceWebLens,
			},
			"provenance": map[string]any{
				"created_by": "test",
			},
		},
		Now: now,
	})
	if err != nil {
		t.Fatalf("create source entity: %v", err)
	}
	if _, err := graph.PutEdge(ctx, capture.CanonicalID, sourceEntity.CanonicalID, "captured_from", map[string]any{"relation": "sourcecycled_source_item"}); err != nil {
		t.Fatalf("put captured_from edge: %v", err)
	}

	captureStories, captureDiagnostic, err := handler.universalWireWebCaptureStories(ctx, 12)
	if err != nil {
		t.Fatalf("read graph capture helper: %v", err)
	}
	if captureDiagnostic.State != "available" || len(captureStories) != 1 {
		t.Fatalf("capture helper diagnostic/stories = %+v/%d, want one available capture story", captureDiagnostic, len(captureStories))
	}
	story := captureStories[0]
	if len(story.Manifest.Lead) != 1 || story.Manifest.Lead[0].CanonicalID != capture.CanonicalID {
		t.Fatalf("lead manifest should remain the web capture object: %+v", story.Manifest.Lead)
	}
	if len(story.Manifest.Context) != 1 {
		t.Fatalf("context manifest len = %d, want captured_from source entity: %+v", len(story.Manifest.Context), story.Manifest.Context)
	}
	contextItem := story.Manifest.Context[0]
	if contextItem.ID != "srcitem-river-watch" ||
		contextItem.ContentID != "srcitem-river-watch" ||
		contextItem.Title != "City alerts river watch bulletin" ||
		contextItem.SourceID != "rss:city-alerts" ||
		contextItem.FetchID != "fetch-river-watch" ||
		contextItem.CanonicalURL != "https://example.test/river-watch" ||
		contextItem.SourceKind != sourcecontract.SourceKindSourceServiceItem ||
		contextItem.TargetKind != sourcecontract.SourceKindSourceServiceItem ||
		contextItem.ObjectKind != "choir.source_entity" ||
		contextItem.CanonicalID != sourceEntity.CanonicalID ||
		contextItem.ContentHash != sourceEntity.ContentHash ||
		contextItem.OpenSurface != sourcecontract.OpenSurfaceSource ||
		contextItem.LiveOpenSurface != sourcecontract.OpenSurfaceWebLens ||
		contextItem.ReaderArtifactState != sourcecontract.ReaderArtifactStateReady ||
		contextItem.ReaderSnapshot == nil ||
		!strings.Contains(contextItem.ReaderSnapshot.TextContent, "river watch after gauges fell") {
		t.Fatalf("captured_from source entity context = %+v", contextItem)
	}
	storyJSON, err := json.Marshal(story)
	if err != nil {
		t.Fatalf("marshal graph-backed story: %v", err)
	}
	if !strings.Contains(string(storyJSON), `"object_kind":"choir.source_entity"`) ||
		!strings.Contains(string(storyJSON), `"canonical_id":"`+sourceEntity.CanonicalID+`"`) {
		t.Fatalf("story JSON missing graph source entity provenance: %s", string(storyJSON))
	}
	if strings.Contains(string(storyJSON), `"source_ref"`) || strings.Contains(string(storyJSON), `"story_texture_doc_id"`) {
		t.Fatalf("graph provenance story should not claim Texture source_ref/publication fields: %s", string(storyJSON))
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
		"corpusd_route_path":           "/pub/texture/scoped-sources",
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

func TestSynthesizeUniversalWireSourceClusterDispatchesProcessorRun(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	now := time.Date(2026, 6, 27, 14, 0, 0, 0, time.UTC)

	_, _, editionRef, err := rt.synthesizeUniversalWireSourceClusterTextureArticle(ctx, universalWireSynthesisClusterRequest{
		ClusterID: "cluster-dispatch-test",
		Sources: []universalWireSynthesisSource{
			{
				ItemID:       "srcitem-dispatch-pt",
				SourceID:     "rss:pt-transport",
				FetchID:      "fetch-dispatch-pt",
				Title:        "Corredor ferroviario reabre parcialmente apos enchentes",
				URL:          "https://example.test/pt/rail",
				CanonicalURL: "https://example.test/pt/rail",
				Language:     "pt",
				Body:         "Equipes de emergencia informaram que o corredor ferroviario reabriu parcialmente depois das enchentes.",
				FetchedAt:    now.Add(-20 * time.Minute),
			},
			{
				ItemID:       "srcitem-dispatch-es",
				SourceID:     "rss:es-commuters",
				FetchID:      "fetch-dispatch-es",
				Title:        "Autoridades advierten demoras para pasajeros regionales",
				URL:          "https://example.test/es/commuters",
				CanonicalURL: "https://example.test/es/commuters",
				Language:     "es",
				Body:         "Las autoridades regionales pidieron a los pasajeros prever demoras mientras continuaban las revisiones de seguridad.",
				FetchedAt:    now.Add(-15 * time.Minute),
			},
		},
		Now: now,
	})
	if err != nil {
		t.Fatalf("synthesize dispatch: %v", err)
	}
	if !strings.HasPrefix(editionRef, universalWireProcessorDispatchRunID+":") {
		t.Fatalf("edition ref = %q, want %s:<run_id> dispatch sentinel", editionRef, universalWireProcessorDispatchRunID)
	}
	runID := strings.TrimPrefix(editionRef, universalWireProcessorDispatchRunID+":")
	if runID == "" {
		t.Fatalf("dispatch sentinel missing run id: %q", editionRef)
	}

	rec, err := s.GetRun(ctx, runID)
	if err != nil {
		t.Fatalf("load dispatched processor run %s: %v", runID, err)
	}
	if canonicalAgentProfile(rec.AgentProfile) != AgentProfileProcessor {
		t.Fatalf("dispatched run profile = %q, want %s", rec.AgentProfile, AgentProfileProcessor)
	}
	processorKey := metadataString(rec.Metadata, runMetadataProcessorKey)
	if processorKey == "" || !strings.HasPrefix(processorKey, "processor:") {
		t.Fatalf("dispatched run processor_key = %q, want processor:... key", processorKey)
	}
	if metadataString(rec.Metadata, "universal_wire_story_cluster_id") != "cluster-dispatch-test" {
		t.Fatalf("dispatched run cluster metadata missing: %+v", rec.Metadata)
	}
	if metadataString(rec.Metadata, "ingestion_handoff_request_kind") != "synthesis_cluster" {
		t.Fatalf("dispatched run request_kind missing: %+v", rec.Metadata)
	}
	dispatchedSourceItemIDs := metadataStringSlice(rec.Metadata["source_item_ids"])
	if !slices.Contains(dispatchedSourceItemIDs, "srcitem-dispatch-pt") ||
		!slices.Contains(dispatchedSourceItemIDs, "srcitem-dispatch-es") {
		t.Fatalf("dispatched run source_item_ids = %+v, want both cluster sources", dispatchedSourceItemIDs)
	}

	// createRunWithMetadata must have opened a per-source-item decision work
	// item for each dispatched source, keyed by the standard source-item
	// fingerprint.
	for _, sourceItemID := range []string{"srcitem-dispatch-pt", "srcitem-dispatch-es"} {
		item, found, err := s.FindWorkItemByFingerprint(ctx, rec.OwnerID, rec.TrajectoryID, wireProcessorSourceItemDecisionWorkItemFingerprint(rec.TrajectoryID, sourceItemID))
		if err != nil {
			t.Fatalf("find source-item work item for %s: %v", sourceItemID, err)
		}
		if !found {
			t.Fatalf("source-item work item not created for %s (trajectory %s)", sourceItemID, rec.TrajectoryID)
		}
		if item.AuthorityProfile != AgentProfileProcessor {
			t.Fatalf("source-item work item authority = %q, want %s", item.AuthorityProfile, AgentProfileProcessor)
		}
	}
}
