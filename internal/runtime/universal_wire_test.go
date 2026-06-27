package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/sourceapi"
	"github.com/yusefmosiah/go-choir/internal/sourcecontract"
	"github.com/yusefmosiah/go-choir/internal/sourcegraph"
	"github.com/yusefmosiah/go-choir/internal/sources"
	"github.com/yusefmosiah/go-choir/internal/texturedoc"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/wirepublish"
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

func TestHandleInternalSourcecycledWebCapturesTriggersTextureSynthesisAndUpdatesCluster(t *testing.T) {
	_, handler := testAPISetup(t)
	now := time.Date(2026, 6, 26, 22, 5, 0, 0, time.UTC)
	wantClusterID := universalWireLiveSourcecycledClusterID + "-transport-delay-flood-inspection"
	firstBatch := []sources.Item{
		universalWireSourcecycledTestItem("srcitem-live-pt", "rss:pt-transport", "fetch-live-pt", "Corredor ferroviario reabre parcialmente", "https://example.com/pt/rail", "pt", "Equipes de emergencia informaram que o corredor ferroviario reabriu parcialmente depois das enchentes, com inspecoes ainda em andamento.", now.Add(-18*time.Minute)),
		universalWireSourcecycledTestItem("srcitem-live-es", "rss:es-commuters", "fetch-live-es", "Autoridades advierten demoras regionales", "https://example.com/es/commuters", "es", "Las autoridades pidieron a los pasajeros prever demoras mientras continuaban las revisiones de seguridad en estaciones afectadas.", now.Add(-12*time.Minute)),
	}
	firstProjection := postInternalSourcecycledWebCapturesForTest(t, handler, firstBatch, now)
	if firstProjection.SynthesisStatus != "ok" ||
		firstProjection.SynthesisDocID == "" ||
		firstProjection.SynthesisRevisionID == "" ||
		firstProjection.SynthesisClusterID != wantClusterID ||
		firstProjection.SynthesisClusterObjectID == "" ||
		firstProjection.SynthesisSourceCount != 2 ||
		firstProjection.SynthesisClusterCount != 1 ||
		firstProjection.SynthesisEditionRef == "" {
		t.Fatalf("first projection synthesis = %+v, want two-source Texture synthesis", firstProjection)
	}

	firstStories := getUniversalWireStoriesForTest(t, handler)
	if firstStories.Source != "universal-wire-edition-texture" ||
		firstStories.Diagnostics != nil ||
		len(firstStories.Stories) != 1 {
		t.Fatalf("first stories = %+v, want non-empty edition Texture story", firstStories)
	}
	firstStory := firstStories.Stories[0]
	if firstStory.StoryTextureDoc != firstProjection.SynthesisDocID ||
		firstStory.SourceState != "universal-wire-edition-texture" ||
		strings.Contains(firstStory.SourceState, "objectgraph-web-capture") ||
		!strings.Contains(firstStory.TextureContent, "disrupted rail corridor") ||
		!strings.Contains(firstStory.TextureContent, "[1]") ||
		!strings.Contains(firstStory.TextureContent, "[2]") {
		t.Fatalf("first story is not the synthesized Texture article: %+v", firstStory)
	}
	assertUniversalWireStoryAvoidsHelperCopyForTest(t, firstStory)
	assertUniversalWireStoryTextureReadableForTest(t, handler, firstStory, firstProjection.SynthesisRevisionID)
	if len(firstStory.Manifest.Lead) != 2 {
		t.Fatalf("manifest lead len = %d, want two source_ref-cited source items: %+v", len(firstStory.Manifest.Lead), firstStory.Manifest)
	}
	if firstStory.Manifest.Lead[0].OpenSurface != sourcecontract.OpenSurfaceSource ||
		firstStory.Manifest.Lead[0].ReaderArtifactState != sourcecontract.ReaderArtifactStateReady ||
		firstStory.Manifest.Lead[0].ReaderSnapshot == nil ||
		!strings.Contains(firstStory.Manifest.Lead[0].ReaderSnapshot.TextContent, "corredor ferroviario") {
		t.Fatalf("first manifest lead lacks Source Viewer reader provenance: %+v", firstStory.Manifest.Lead[0])
	}
	captureStories, captureDiagnostic, err := handler.universalWireWebCaptureStories(context.Background(), 12)
	if err != nil {
		t.Fatalf("read graph capture helper: %v", err)
	}
	if captureDiagnostic.State != "available" ||
		captureDiagnostic.StoryCount != 2 ||
		len(captureStories) != 2 ||
		captureStories[0].SourceState != "objectgraph-web-capture" ||
		captureStories[0].StoryTextureDoc != "" ||
		captureStories[0].SemanticStory != nil {
		t.Fatalf("raw graph captures should remain diagnostic substrate, got diagnostic=%+v stories=%+v", captureDiagnostic, captureStories)
	}
	firstRev, err := handler.rt.Store().GetRevision(context.Background(), firstProjection.SynthesisRevisionID, universalWirePlatformOwnerID())
	if err != nil {
		t.Fatalf("load first synthesis revision: %v", err)
	}
	if !strings.Contains(string(firstRev.BodyDoc), `"source_ref"`) {
		t.Fatalf("first synthesis body_doc missing native source_ref citations: %s", string(firstRev.BodyDoc))
	}
	var firstRevMeta map[string]any
	if err := json.Unmarshal(firstRev.Metadata, &firstRevMeta); err != nil {
		t.Fatalf("decode first synthesis revision metadata: %v", err)
	}
	if metadataString(firstRevMeta, "universal_wire_story_cluster_id") != wantClusterID ||
		metadataString(firstRevMeta, "universal_wire_story_cluster_object_id") != firstProjection.SynthesisClusterObjectID {
		t.Fatalf("first synthesis revision cluster metadata = %#v, want %s/%s", firstRevMeta, wantClusterID, firstProjection.SynthesisClusterObjectID)
	}
	firstCluster, err := handler.rt.ObjectGraph().GetObject(context.Background(), firstProjection.SynthesisClusterObjectID)
	if err != nil {
		t.Fatalf("load first Universal Wire story cluster object: %v", err)
	}
	var firstClusterMeta map[string]any
	if err := json.Unmarshal(firstCluster.Metadata, &firstClusterMeta); err != nil {
		t.Fatalf("decode first story cluster metadata: %v", err)
	}
	if firstCluster.ObjectKind != objectgraph.UniversalWireStoryClusterObjectKind ||
		metadataString(firstClusterMeta, "schema_version") != objectgraph.UniversalWireStoryClusterSchemaVersion ||
		metadataString(firstClusterMeta, "cluster_id") != wantClusterID ||
		metadataString(firstClusterMeta, "world_model_kind") != "universal_wire_semantic_story" ||
		metadataString(firstClusterMeta, "semantic_story_id") == "" ||
		metadataString(firstClusterMeta, "article_doc_id") != firstProjection.SynthesisDocID ||
		metadataString(firstClusterMeta, "article_revision_id") != firstProjection.SynthesisRevisionID ||
		int(firstClusterMeta["source_count"].(float64)) != 2 {
		t.Fatalf("first story cluster = %+v metadata=%#v, want durable two-source cluster for article", firstCluster, firstClusterMeta)
	}
	var firstSemanticState universalWireSemanticStoryState
	if err := json.Unmarshal(firstCluster.Body, &firstSemanticState); err != nil {
		t.Fatalf("decode first semantic story state: %v body=%s", err, string(firstCluster.Body))
	}
	if firstSemanticState.WorldModelKind != "universal_wire_semantic_story" ||
		firstSemanticState.StoryID != metadataString(firstClusterMeta, "semantic_story_id") ||
		firstSemanticState.LatestChange.ChangeType != "story_created" ||
		firstSemanticState.LatestChange.PreviousSourceCount != 0 ||
		firstSemanticState.LatestChange.CurrentSourceCount != 2 ||
		len(firstSemanticState.LatestChange.AddedSourceItemIDs) != 2 ||
		!slices.Contains(firstSemanticState.TopicConcepts, "transport") ||
		!slices.Contains(firstSemanticState.SignalConcepts, "inspection") {
		t.Fatalf("first semantic story state = %+v metadata=%#v, want durable created world-model state", firstSemanticState, firstClusterMeta)
	}
	assertWireStorySemanticStateForTest(t, firstStory, firstSemanticState)
	firstStoriesJSON, err := json.Marshal(firstStories)
	if err != nil {
		t.Fatalf("marshal first stories: %v", err)
	}
	if !strings.Contains(string(firstStoriesJSON), `"semantic_story"`) ||
		strings.Contains(string(firstStoriesJSON), "universal_wire_semantic_story_id") ||
		strings.Contains(string(firstStoriesJSON), "World-model") {
		t.Fatalf("first product story response = %s, want structured semantic evidence without raw metadata/prose leaks", string(firstStoriesJSON))
	}
	if strings.Contains(firstRev.Content, firstSemanticState.StoryID) ||
		strings.Contains(firstRev.Content, "World-model") ||
		!strings.Contains(firstRev.Content, "disrupted rail corridor") ||
		!strings.Contains(firstRev.Content, "Later reporting should update this account") {
		t.Fatalf("first synthesis article content = %q, want article-like semantic-state update without internal ids", firstRev.Content)
	}
	firstClusterEdges, err := handler.rt.ObjectGraph().ListEdges(context.Background(), objectgraph.EdgeFilter{
		FromID: firstCluster.CanonicalID,
		Kind:   "contains",
	})
	if err != nil {
		t.Fatalf("list first story cluster source edges: %v", err)
	}
	if len(firstClusterEdges) != 2 {
		t.Fatalf("first story cluster edges len = %d, want two source captures: %#v", len(firstClusterEdges), firstClusterEdges)
	}

	secondBatch := []sources.Item{
		universalWireSourcecycledTestItem("srcitem-live-fr", "rss:fr-rail", "fetch-live-fr", "La reprise reste partielle sur le corridor ferroviaire", "https://example.com/fr/rail", "fr", "Les exploitants ferroviaires ont confirme que la reprise restait partielle et que de nouvelles inspections etaient prevues avant le soir.", now.Add(8*time.Minute)),
	}
	secondProjection := postInternalSourcecycledWebCapturesForTest(t, handler, secondBatch, now.Add(10*time.Minute))
	if secondProjection.SynthesisStatus != "ok" ||
		secondProjection.SynthesisDocID != firstProjection.SynthesisDocID ||
		secondProjection.SynthesisRevisionID == firstProjection.SynthesisRevisionID ||
		secondProjection.SynthesisSourceCount != 3 ||
		secondProjection.SynthesisClusterID != wantClusterID ||
		secondProjection.SynthesisClusterCount != 1 {
		t.Fatalf("second projection synthesis = %+v, want same article revised with three sources", secondProjection)
	}
	updatedDoc, err := handler.rt.Store().GetDocument(context.Background(), firstProjection.SynthesisDocID, universalWirePlatformOwnerID())
	if err != nil {
		t.Fatalf("load updated synthesis doc: %v", err)
	}
	if updatedDoc.CurrentRevisionID != secondProjection.SynthesisRevisionID {
		t.Fatalf("updated doc current revision = %q, want %q", updatedDoc.CurrentRevisionID, secondProjection.SynthesisRevisionID)
	}
	secondCluster, err := handler.rt.ObjectGraph().GetObject(context.Background(), firstProjection.SynthesisClusterObjectID)
	if err != nil {
		t.Fatalf("load updated Universal Wire story cluster object: %v", err)
	}
	if secondCluster.CanonicalID != firstCluster.CanonicalID || secondCluster.ContentHash == firstCluster.ContentHash {
		t.Fatalf("updated story cluster identity/hash = %s/%s, first = %s/%s; want same object identity with revised state", secondCluster.CanonicalID, secondCluster.ContentHash, firstCluster.CanonicalID, firstCluster.ContentHash)
	}
	var secondClusterMeta map[string]any
	if err := json.Unmarshal(secondCluster.Metadata, &secondClusterMeta); err != nil {
		t.Fatalf("decode updated story cluster metadata: %v", err)
	}
	if metadataString(secondClusterMeta, "article_doc_id") != firstProjection.SynthesisDocID ||
		metadataString(secondClusterMeta, "article_revision_id") != secondProjection.SynthesisRevisionID ||
		metadataString(secondClusterMeta, "semantic_story_id") != firstSemanticState.StoryID ||
		int(secondClusterMeta["source_count"].(float64)) != 3 {
		t.Fatalf("updated story cluster metadata = %#v, want same article updated to three sources", secondClusterMeta)
	}
	var secondSemanticState universalWireSemanticStoryState
	if err := json.Unmarshal(secondCluster.Body, &secondSemanticState); err != nil {
		t.Fatalf("decode updated semantic story state: %v body=%s", err, string(secondCluster.Body))
	}
	if secondSemanticState.StoryID != firstSemanticState.StoryID ||
		secondSemanticState.LatestChange.ChangeType != "source_added" ||
		secondSemanticState.LatestChange.PreviousSourceCount != 2 ||
		secondSemanticState.LatestChange.CurrentSourceCount != 3 ||
		!slices.Contains(secondSemanticState.LatestChange.AddedSourceItemIDs, "srcitem-live-fr") {
		t.Fatalf("updated semantic story state = %+v first=%+v, want typed later-source update on same identity", secondSemanticState, firstSemanticState)
	}
	secondRev, err := handler.rt.Store().GetRevision(context.Background(), secondProjection.SynthesisRevisionID, universalWirePlatformOwnerID())
	if err != nil {
		t.Fatalf("load second synthesis revision: %v", err)
	}
	var secondRevMeta map[string]any
	if err := json.Unmarshal(secondRev.Metadata, &secondRevMeta); err != nil {
		t.Fatalf("decode second synthesis revision metadata: %v", err)
	}
	if metadataString(secondRevMeta, "universal_wire_semantic_story_id") != firstSemanticState.StoryID ||
		metadataString(secondRevMeta, "universal_wire_semantic_change_type") != "source_added" ||
		strings.Contains(secondRev.Content, secondSemanticState.StoryID) ||
		strings.Contains(secondRev.Content, "World-model") ||
		!strings.Contains(secondRev.Content, "should be revised here while later reporting still fits the same event") {
		t.Fatalf("second revision meta/content = %#v / %q, want Texture revision from semantic metadata and article-like public copy", secondRevMeta, secondRev.Content)
	}
	secondSourceItemIDs := metadataStringSliceValue(secondRevMeta, "source_item_ids")
	for _, wantSourceItemID := range []string{"srcitem-live-pt", "srcitem-live-es", "srcitem-live-fr"} {
		if !slices.Contains(secondSourceItemIDs, wantSourceItemID) ||
			!strings.Contains(string(secondRev.SourceEntities), wantSourceItemID) {
			t.Fatalf("second revision source carry-forward missing %q: metadata=%v source_entities=%s", wantSourceItemID, secondSourceItemIDs, string(secondRev.SourceEntities))
		}
	}
	if strings.Count(string(secondRev.BodyDoc), `"source_ref"`) != 3 {
		t.Fatalf("second revision body_doc = %s, want native source_ref citations for prior and later sources", string(secondRev.BodyDoc))
	}
	secondClusterEdges, err := handler.rt.ObjectGraph().ListEdges(context.Background(), objectgraph.EdgeFilter{
		FromID: firstCluster.CanonicalID,
		Kind:   "contains",
	})
	if err != nil {
		t.Fatalf("list updated story cluster source edges: %v", err)
	}
	if len(secondClusterEdges) != 3 {
		t.Fatalf("updated story cluster edges len = %d, want three source captures: %#v", len(secondClusterEdges), secondClusterEdges)
	}
	secondStories := getUniversalWireStoriesForTest(t, handler)
	if len(secondStories.Stories) != 1 ||
		secondStories.Stories[0].StoryTextureDoc != firstProjection.SynthesisDocID ||
		len(secondStories.Stories[0].Manifest.Lead) != 3 ||
		!slices.Contains(secondStories.Edition.IncludedDocIDs, firstProjection.SynthesisDocID) ||
		countStrings(secondStories.Edition.IncludedDocIDs, firstProjection.SynthesisDocID) != 1 {
		t.Fatalf("updated stories = %+v, want one revised article and one edition transclusion", secondStories)
	}
	assertWireStorySemanticStateForTest(t, secondStories.Stories[0], secondSemanticState)
	if secondStories.Stories[0].SemanticStory.StoryID != firstStory.SemanticStory.StoryID ||
		secondStories.Stories[0].SemanticStory.ChangeType != "source_added" ||
		secondStories.Stories[0].SemanticStory.PreviousSourceCount != 2 ||
		secondStories.Stories[0].SemanticStory.CurrentSourceCount != 3 {
		t.Fatalf("updated product story semantic evidence = %+v, want same story id with typed source_added change", secondStories.Stories[0].SemanticStory)
	}
	secondStoriesJSON, err := json.Marshal(secondStories)
	if err != nil {
		t.Fatalf("marshal second stories: %v", err)
	}
	if !strings.Contains(string(secondStoriesJSON), firstStory.SemanticStory.StoryID) ||
		strings.Contains(secondStories.Stories[0].TextureContent, firstStory.SemanticStory.StoryID) ||
		strings.Contains(string(secondStoriesJSON), "universal_wire_semantic_story_id") {
		t.Fatalf("updated product story response = %s, content=%q; want API-observable story id without reader-copy leak", string(secondStoriesJSON), secondStories.Stories[0].TextureContent)
	}
	assertUniversalWireStoryTextureReadableForTest(t, handler, secondStories.Stories[0], secondProjection.SynthesisRevisionID)
}

func TestHandleInternalSourcecycledWebCapturesSplitsUnrelatedStoryClusters(t *testing.T) {
	_, handler := testAPISetup(t)
	ctx := context.Background()
	now := time.Date(2026, 6, 27, 3, 15, 0, 0, time.UTC)
	batch := []sources.Item{
		universalWireSourcecycledTestItem("srcitem-split-rail-1", "rss:rail", "fetch-rail-1", "Rail corridor reopens after inspections", "https://example.com/rail/reopen", "en", "Emergency crews reopened the rail corridor after flood inspections, and commuters returned to central stations.", now.Add(-35*time.Minute)),
		universalWireSourcecycledTestItem("srcitem-split-rail-2", "rss:commuters", "fetch-rail-2", "Commuters return to railway stations", "https://example.com/rail/commuters", "en", "Transit officials said passengers should expect slower railway service while station inspections continue.", now.Add(-31*time.Minute)),
		universalWireSourcecycledTestItem("srcitem-split-strike-1", "rss:transit-labor", "fetch-strike-1", "Transit strike begins after overnight talks fail", "https://example.com/transit/strike", "en", "Commuters waited for buses after a transit strike began when overnight labor talks failed.", now.Add(-25*time.Minute)),
		universalWireSourcecycledTestItem("srcitem-split-strike-2", "rss:bus-labor", "fetch-strike-2", "Bus drivers extend walkout through afternoon", "https://example.com/transit/walkout", "en", "Bus drivers extended the walkout, and city officials warned passengers to expect reduced service.", now.Add(-22*time.Minute)),
	}
	projection := postInternalSourcecycledWebCapturesForTest(t, handler, batch, now)
	if projection.SynthesisStatus != "ok" ||
		projection.SynthesisClusterCount != 2 ||
		projection.SynthesisDocID == "" ||
		projection.SynthesisRevisionID == "" ||
		projection.SynthesisClusterID == "" {
		t.Fatalf("projection synthesis = %+v, want two deterministic story clusters materialized", projection)
	}

	stories := getUniversalWireStoriesForTest(t, handler)
	if stories.Source != "universal-wire-edition-texture" ||
		stories.Diagnostics != nil ||
		stories.Edition == nil ||
		len(stories.Stories) != 2 ||
		len(stories.Edition.IncludedDocIDs) != 2 {
		t.Fatalf("stories = %+v, want two platform-owned synthesis articles in the Wire edition", stories)
	}
	docIDs := map[string]bool{}
	semanticStoryIDs := map[string]bool{}
	for _, story := range stories.Stories {
		if story.StoryTextureDoc == "" ||
			story.SourceState != "universal-wire-edition-texture" ||
			strings.Contains(story.SourceState, "objectgraph-web-capture") ||
			len(story.Manifest.Lead) != 2 {
			t.Fatalf("story = %+v, want synthesized Texture article with two source-ref leads", story)
		}
		if story.SemanticStory == nil ||
			story.SemanticStory.StoryID == "" ||
			story.SemanticStory.ChangeType != "story_created" ||
			story.SemanticStory.CurrentSourceCount != 2 ||
			story.SemanticStory.SourceCount != 2 {
			t.Fatalf("story semantic evidence = %+v, want product-observable created semantic story", story.SemanticStory)
		}
		semanticStoryIDs[story.SemanticStory.StoryID] = true
		assertUniversalWireStoryAvoidsHelperCopyForTest(t, story)
		docIDs[story.StoryTextureDoc] = true
		assertUniversalWireStoryTextureReadableForTest(t, handler, story, "")
		for _, lead := range story.Manifest.Lead {
			if lead.OpenSurface != sourcecontract.OpenSurfaceSource ||
				lead.ReaderArtifactState != sourcecontract.ReaderArtifactStateReady ||
				lead.ReaderSnapshot == nil ||
				lead.ReaderSnapshot.TextContent == "" {
				t.Fatalf("story lead lacks Source Viewer reader provenance: %+v", lead)
			}
		}
		doc, err := handler.rt.Store().GetDocument(ctx, story.StoryTextureDoc, universalWirePlatformOwnerID())
		if err != nil {
			t.Fatalf("load story doc %s: %v", story.StoryTextureDoc, err)
		}
		rev, err := handler.rt.Store().GetRevision(ctx, doc.CurrentRevisionID, universalWirePlatformOwnerID())
		if err != nil {
			t.Fatalf("load story revision %s: %v", doc.CurrentRevisionID, err)
		}
		if !strings.Contains(string(rev.BodyDoc), `"source_ref"`) {
			t.Fatalf("revision body_doc missing native source_ref citations: %s", string(rev.BodyDoc))
		}
		var entities []texturedoc.SourceEntity
		if err := json.Unmarshal(rev.SourceEntities, &entities); err != nil {
			t.Fatalf("decode source_entities: %v", err)
		}
		if len(entities) != 2 {
			t.Fatalf("source_entities len = %d, want two cited sources: %#v", len(entities), entities)
		}
	}
	for _, docID := range stories.Edition.IncludedDocIDs {
		if !docIDs[docID] || countStrings(stories.Edition.IncludedDocIDs, docID) != 1 {
			t.Fatalf("edition included docs = %+v, want each synthesized doc exactly once", stories.Edition.IncludedDocIDs)
		}
	}

	clusterObjects, err := handler.rt.ObjectGraph().ListObjects(ctx, objectgraph.ListFilter{
		Kind:    objectgraph.UniversalWireStoryClusterObjectKind,
		OwnerID: universalWirePlatformOwnerID(),
		Limit:   12,
	})
	if err != nil {
		t.Fatalf("list story clusters: %v", err)
	}
	if len(clusterObjects) != 2 {
		t.Fatalf("cluster object count = %d, want two durable story clusters: %+v", len(clusterObjects), clusterObjects)
	}
	clusterIDs := map[string]bool{}
	clusterDocIDs := map[string]bool{}
	for _, cluster := range clusterObjects {
		var meta map[string]any
		if err := json.Unmarshal(cluster.Metadata, &meta); err != nil {
			t.Fatalf("decode cluster metadata: %v", err)
		}
		clusterID := metadataString(meta, "cluster_id")
		clusterIDs[clusterID] = true
		clusterDocIDs[metadataString(meta, "article_doc_id")] = true
		if metadataString(meta, "schema_version") != objectgraph.UniversalWireStoryClusterSchemaVersion ||
			int(meta["source_count"].(float64)) != 2 ||
			metadataString(meta, "article_revision_id") == "" ||
			metadataString(meta, "wire_edition_ref") == "" {
			t.Fatalf("cluster metadata = %#v, want two-source article cluster", meta)
		}
		edges, err := handler.rt.ObjectGraph().ListEdges(ctx, objectgraph.EdgeFilter{
			FromID: cluster.CanonicalID,
			Kind:   "contains",
		})
		if err != nil {
			t.Fatalf("list cluster edges: %v", err)
		}
		if len(edges) != 2 {
			t.Fatalf("cluster %s edges len = %d, want two source captures: %#v", clusterID, len(edges), edges)
		}
	}
	if !clusterIDs[universalWireLiveSourcecycledClusterID+"-transport-delay-flood-inspection"] ||
		!clusterIDs[universalWireLiveSourcecycledClusterID+"-transport-strike"] ||
		len(clusterDocIDs) != 2 ||
		len(semanticStoryIDs) != 2 {
		t.Fatalf("clusters = %#v docIDs=%#v semanticIDs=%#v, want separate transport rail-reopening and transport-strike articles", clusterIDs, clusterDocIDs, semanticStoryIDs)
	}

	captureStories, captureDiagnostic, err := handler.universalWireWebCaptureStories(ctx, 12)
	if err != nil {
		t.Fatalf("read raw graph capture helper: %v", err)
	}
	if captureDiagnostic.State != "available" ||
		captureDiagnostic.StoryCount != 4 ||
		len(captureStories) != 4 {
		t.Fatalf("raw graph captures diagnostic = %+v stories=%+v, want diagnostic-only capture substrate", captureDiagnostic, captureStories)
	}
	for _, story := range captureStories {
		if story.SourceState != "objectgraph-web-capture" || story.StoryTextureDoc != "" || story.SemanticStory != nil {
			t.Fatalf("raw graph capture helper story = %+v, want diagnostic-only non-article projection", story)
		}
	}
}

func TestHandleInternalSourcecycledWebCapturesKeepsDeployedShapedArrivalsSeparated(t *testing.T) {
	_, handler := testAPISetup(t)
	ctx := context.Background()
	now := time.Date(2026, 6, 27, 8, 0, 0, 0, time.UTC)
	firstBatch := []sources.Item{
		universalWireSourcecycledTestItem("srcitem-noisy-rail-1", "rss:rail-a", "fetch-noisy-rail-1", "Rail corridor reopens after flood inspections", "https://example.com/noisy/rail-1", "en", "Emergency officials said the rail corridor reopened after flood inspections, while commuters returned to central stations. This source also mentions unrelated butcher video commentary and ai criticism as background noise.", now.Add(-50*time.Minute)),
		universalWireSourcecycledTestItem("srcitem-noisy-rail-2", "rss:rail-b", "fetch-noisy-rail-2", "Commuters warned about rail delays after inspections", "https://example.com/noisy/rail-2", "en", "Transit officials warned passengers about delays while railway inspection crews finished checks along the same corridor.", now.Add(-48*time.Minute)),
		universalWireSourcecycledTestItem("srcitem-noisy-harbor-1", "rss:harbor-a", "fetch-noisy-harbor-1", "Harbor pilots reopen cargo channel", "https://example.com/noisy/harbor-1", "en", "Port pilots reopened the cargo channel after soundings, and vessels waited for tide clearance outside the harbor.", now.Add(-44*time.Minute)),
		universalWireSourcecycledTestItem("srcitem-noisy-harbor-2", "rss:harbor-b", "fetch-noisy-harbor-2", "Cargo vessels face delays at harbor channel", "https://example.com/noisy/harbor-2", "en", "Maritime officials said cargo vessels faced delay while pilots checked channel soundings near the port.", now.Add(-42*time.Minute)),
		universalWireSourcecycledTestItem("srcitem-noisy-health-1", "rss:health-a", "fetch-noisy-health-1", "Clinic vaccine inspection delays patients", "https://example.com/noisy/health-1", "en", "Hospital patients waited after clinic vaccine inspections delayed appointments during a regional health review.", now.Add(-38*time.Minute)),
		universalWireSourcecycledTestItem("srcitem-noisy-ai", "rss:ai", "fetch-noisy-ai", "Cory Doctorow criticizes AI industry arguments", "https://example.com/noisy/ai", "en", "The essay criticized ai firms and copyright claims, with no relation to transport, harbor access, hospital inspections, or vaccine logistics.", now.Add(-35*time.Minute)),
		universalWireSourcecycledTestItem("srcitem-noisy-energy", "rss:energy", "fetch-noisy-energy", "Grid substation crews review blackout plan", "https://example.com/noisy/energy", "en", "Energy officials reviewed power grid blackout plans after a substation alarm, but reported no transport or harbor delays.", now.Add(-33*time.Minute)),
		universalWireSourcecycledTestItem("srcitem-noisy-school", "rss:school", "fetch-noisy-school", "School board delays budget vote", "https://example.com/noisy/school", "en", "A school board delayed its budget vote after members requested more public comment.", now.Add(-31*time.Minute)),
		universalWireSourcecycledTestItem("srcitem-noisy-farm", "rss:farm", "fetch-noisy-farm", "Farm market adjusts weekend hours", "https://example.com/noisy/farm", "en", "A farm market adjusted weekend hours after vendors changed delivery windows.", now.Add(-29*time.Minute)),
		universalWireSourcecycledTestItem("srcitem-noisy-culture", "rss:culture", "fetch-noisy-culture", "Museum opens a restored theater exhibit", "https://example.com/noisy/culture", "en", "The museum opened a restored theater exhibit with local archival material.", now.Add(-27*time.Minute)),
	}
	firstProjection := postInternalSourcecycledWebCapturesForTest(t, handler, firstBatch, now)
	if firstProjection.SynthesisStatus != "ok" || firstProjection.SynthesisClusterCount != 2 {
		t.Fatalf("first projection synthesis = %+v, want two coherent story clusters from noisy batch", firstProjection)
	}
	firstStories := getUniversalWireStoriesForTest(t, handler)
	if firstStories.Source != "universal-wire-edition-texture" ||
		firstStories.Diagnostics != nil ||
		firstStories.Edition == nil ||
		len(firstStories.Stories) != 2 ||
		len(firstStories.Edition.IncludedDocIDs) != 2 {
		t.Fatalf("first stories = %+v, want two synthesized articles and no mega-article", firstStories)
	}
	railStory := universalWireStoryWithSourceTitleForTest(firstStories.Stories, "Rail corridor reopens")
	harborStory := universalWireStoryWithSourceTitleForTest(firstStories.Stories, "Harbor pilots reopen")
	if railStory == nil || harborStory == nil || railStory.StoryTextureDoc == harborStory.StoryTextureDoc {
		t.Fatalf("stories = %+v, want separate rail and harbor articles", firstStories.Stories)
	}
	if railStory.SemanticStory == nil ||
		railStory.SemanticStory.CurrentSourceCount != 2 ||
		!slices.Contains(railStory.SemanticStory.TopicConcepts, "transport") ||
		!slices.Contains(railStory.SemanticStory.SignalConcepts, "inspection") ||
		slices.Contains(railStory.SemanticStory.SignalConcepts, "doctorow") ||
		len(railStory.SemanticStory.SignalConcepts) > universalWireSemanticSignatureMaxSignals {
		t.Fatalf("rail semantic story = %+v, want capped story concepts without noisy raw tokens", railStory.SemanticStory)
	}
	if harborStory.SemanticStory == nil ||
		harborStory.SemanticStory.CurrentSourceCount != 2 ||
		!slices.Contains(harborStory.SemanticStory.TopicConcepts, "harbor") ||
		!slices.Contains(harborStory.SemanticStory.SignalConcepts, "harbor-access") {
		t.Fatalf("harbor semantic story = %+v, want separate harbor identity", harborStory.SemanticStory)
	}
	railDocID := railStory.StoryTextureDoc
	railStoryID := railStory.SemanticStory.StoryID
	harborDocID := harborStory.StoryTextureDoc
	harborStoryID := harborStory.SemanticStory.StoryID
	assertUniversalWireStoryTextureReadableForTest(t, handler, *railStory, "")
	assertUniversalWireStoryTextureReadableForTest(t, handler, *harborStory, "")

	secondProjection := postInternalSourcecycledWebCapturesForTest(t, handler, []sources.Item{
		universalWireSourcecycledTestItem("srcitem-noisy-rail-3", "rss:rail-c", "fetch-noisy-rail-3", "Rail inspections extend after evening flood checks", "https://example.com/noisy/rail-3", "en", "Railway crews extended evening flood inspections along the corridor, keeping some commuter delays in place.", now.Add(5*time.Minute)),
	}, now.Add(6*time.Minute))
	if secondProjection.SynthesisStatus != "ok" ||
		secondProjection.SynthesisClusterCount != 1 ||
		secondProjection.SynthesisSourceCount != 3 {
		t.Fatalf("second projection = %+v, want matching rail arrival to update only the rail article", secondProjection)
	}
	secondStories := getUniversalWireStoriesForTest(t, handler)
	if len(secondStories.Stories) != 2 ||
		countStrings(secondStories.Edition.IncludedDocIDs, railDocID) != 1 ||
		countStrings(secondStories.Edition.IncludedDocIDs, harborDocID) != 1 {
		t.Fatalf("second stories = %+v, want same two edition articles with no duplicate rail transclusion", secondStories)
	}
	updatedRail := universalWireStoryWithSourceTitleForTest(secondStories.Stories, "Rail inspections extend")
	unchangedHarbor := universalWireStoryWithDocForTest(secondStories.Stories, harborDocID)
	if updatedRail == nil ||
		updatedRail.StoryTextureDoc != railDocID ||
		updatedRail.SemanticStory == nil ||
		updatedRail.SemanticStory.StoryID != railStoryID ||
		updatedRail.SemanticStory.ChangeType != "source_added" ||
		updatedRail.SemanticStory.PreviousSourceCount != 2 ||
		updatedRail.SemanticStory.CurrentSourceCount != 3 {
		t.Fatalf("updated rail story = %+v, want same semantic story/article revised to three sources", updatedRail)
	}
	if unchangedHarbor == nil ||
		unchangedHarbor.SemanticStory == nil ||
		unchangedHarbor.SemanticStory.StoryID != harborStoryID ||
		unchangedHarbor.SemanticStory.ChangeType != "story_created" ||
		unchangedHarbor.SemanticStory.CurrentSourceCount != 2 {
		t.Fatalf("harbor story after rail update = %+v, want separate story not absorbed by rail update", unchangedHarbor)
	}
	updatedRailRev, err := handler.rt.Store().GetRevision(ctx, secondProjection.SynthesisRevisionID, universalWirePlatformOwnerID())
	if err != nil {
		t.Fatalf("load updated rail revision: %v", err)
	}
	if strings.Count(string(updatedRailRev.BodyDoc), `"source_ref"`) != 3 ||
		!strings.Contains(string(updatedRailRev.SourceEntities), "srcitem-noisy-rail-1") ||
		!strings.Contains(string(updatedRailRev.SourceEntities), "srcitem-noisy-rail-3") {
		t.Fatalf("updated rail revision body/source_entities = %s / %s, want prior and new native source refs", string(updatedRailRev.BodyDoc), string(updatedRailRev.SourceEntities))
	}

	thirdProjection := postInternalSourcecycledWebCapturesForTest(t, handler, []sources.Item{
		universalWireSourcecycledTestItem("srcitem-noisy-health-2", "rss:health-b", "fetch-noisy-health-2", "Hospital vaccine inspections delay clinic appointments", "https://example.com/noisy/health-2", "en", "Hospital clinic managers said vaccine inspections delayed patient appointments while health officials reviewed storage records.", now.Add(12*time.Minute)),
	}, now.Add(13*time.Minute))
	if thirdProjection.SynthesisStatus != "ok" ||
		thirdProjection.SynthesisClusterCount != 1 ||
		thirdProjection.SynthesisSourceCount != 2 ||
		thirdProjection.SynthesisDocID == railDocID ||
		thirdProjection.SynthesisDocID == harborDocID {
		t.Fatalf("third projection = %+v, want unrelated health arrival to create a separate article", thirdProjection)
	}
	thirdStories := getUniversalWireStoriesForTest(t, handler)
	healthStory := universalWireStoryWithSourceTitleForTest(thirdStories.Stories, "Hospital vaccine inspections")
	if len(thirdStories.Stories) != 3 ||
		healthStory == nil ||
		healthStory.SemanticStory == nil ||
		healthStory.SemanticStory.StoryID == railStoryID ||
		healthStory.SemanticStory.StoryID == harborStoryID ||
		healthStory.SemanticStory.ChangeType != "story_created" ||
		healthStory.SemanticStory.CurrentSourceCount != 2 ||
		len(thirdStories.Edition.IncludedDocIDs) != 3 {
		t.Fatalf("third stories = %+v health=%+v, want separate health article alongside rail and harbor", thirdStories, healthStory)
	}
	assertUniversalWireStoryTextureReadableForTest(t, handler, *healthStory, thirdProjection.SynthesisRevisionID)
}

func TestHandleUniversalWireStoriesMaterializesExistingSourcecycledGraphCaptures(t *testing.T) {
	_, handler := testAPISetup(t)
	ctx := context.Background()
	now := time.Date(2026, 6, 26, 22, 32, 0, 0, time.UTC)
	var publishedMu sync.Mutex
	publishedDocs := map[string]types.Document{}
	publishedRevs := map[string]types.Revision{}
	publishCount := 0
	syncCount := 0
	platformd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Internal-Caller") != "true" {
			t.Fatalf("platformd internal header = %q, want true", r.Header.Get("X-Internal-Caller"))
		}
		publishedMu.Lock()
		defer publishedMu.Unlock()
		switch {
		case r.URL.Path == "/internal/platform/publications/texture":
			publishCount++
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(wirepublish.PublishTextureResponse{
				PublicationID:        "pub-direct",
				PublicationVersionID: "pubv-direct",
				RoutePath:            "wire/direct",
			})
		case r.URL.Path == "/internal/platform/texture/sync":
			var req struct {
				DocID     string `json:"doc_id"`
				OwnerID   string `json:"owner_id"`
				Title     string `json:"title"`
				Revisions []struct {
					RevisionID     string          `json:"revision_id"`
					Content        string          `json:"content"`
					BodyDoc        json.RawMessage `json:"body_doc"`
					SourceEntities json.RawMessage `json:"source_entities"`
				} `json:"revisions"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode platformd sync: %v", err)
			}
			if req.DocID == "" || req.OwnerID != universalWirePlatformOwnerID() || len(req.Revisions) == 0 {
				t.Fatalf("platformd sync request = %+v, want platform document with revisions", req)
			}
			if !strings.Contains(string(req.Revisions[0].BodyDoc), `"source_ref"`) {
				t.Fatalf("platformd sync body_doc missing native source refs: %s", req.Revisions[0].BodyDoc)
			}
			if !strings.Contains(string(req.Revisions[0].SourceEntities), `"source_entity_id"`) {
				t.Fatalf("platformd sync source_entities missing: %s", req.Revisions[0].SourceEntities)
			}
			syncCount++
			publishedDocs[req.DocID] = types.Document{DocID: req.DocID, OwnerID: req.OwnerID, Title: req.Title}
			for _, rev := range req.Revisions {
				publishedRevs[rev.RevisionID] = types.Revision{RevisionID: rev.RevisionID, DocID: req.DocID, OwnerID: req.OwnerID, Content: rev.Content, BodyDoc: rev.BodyDoc, SourceEntities: rev.SourceEntities}
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{"doc_id": req.DocID, "revision_count": len(req.Revisions)})
		case strings.HasPrefix(r.URL.Path, "/internal/platform/texture/documents/"):
			docID := strings.TrimPrefix(r.URL.Path, "/internal/platform/texture/documents/")
			if _, ok := publishedDocs[docID]; !ok {
				http.NotFound(w, r)
				return
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{"doc_id": docID})
		case strings.HasPrefix(r.URL.Path, "/internal/platform/texture/revisions/"):
			revisionID := strings.TrimPrefix(r.URL.Path, "/internal/platform/texture/revisions/")
			if _, ok := publishedRevs[revisionID]; !ok {
				http.NotFound(w, r)
				return
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{"revision_id": revisionID})
		default:
			http.NotFound(w, r)
		}
	}))
	defer platformd.Close()
	t.Setenv("RUNTIME_PLATFORMD_URL", platformd.URL)
	handler.rt.cfg.PlatformdURL = platformd.URL
	items := []sources.Item{
		universalWireSourcecycledTestItem("srcitem-backfill-pt", "rss:pt-wire", "fetch-backfill-pt", "Chuvas interrompem corredor logistico", "https://example.com/pt/logistics", "pt", "Relatorios locais disseram que as chuvas interromperam um corredor logistico e atrasaram entregas regionais.", now.Add(-25*time.Minute)),
		universalWireSourcecycledTestItem("srcitem-backfill-en", "rss:en-wire", "fetch-backfill-en", "Regional logistics delays follow heavy rain", "https://example.com/en/logistics", "en", "Transport agencies reported regional delays after heavy rain damaged inspection points along the logistics corridor.", now.Add(-18*time.Minute)),
	}
	projection, err := sourcegraph.WriteWebCaptureGraphObjects(ctx, handler.rt.ObjectGraph(), items, sourcegraph.WebCaptureGraphProjectionConfig{
		OwnerID:    universalWirePlatformOwnerID(),
		ComputerID: "computer-universal-wire-platform",
		Now:        now,
	})
	if err != nil {
		t.Fatalf("seed existing sourcecycled graph captures: %v", err)
	}
	if len(projection.Captures) != 2 || len(projection.SourceEntities) != 2 || projection.EdgeCount != 2 {
		t.Fatalf("projection = %+v, want two existing sourcecycled captures with source edges", projection)
	}
	if _, err := handler.rt.Store().GetDocumentAlias(ctx, universalWirePlatformOwnerID(), universalWireEditionSourcePath); err == nil {
		t.Fatal("Wire edition alias exists before read-time materialization")
	}

	firstStories := getUniversalWireStoriesForTest(t, handler)
	if firstStories.Source != "universal-wire-edition-texture" ||
		firstStories.Diagnostics != nil ||
		firstStories.Edition == nil ||
		len(firstStories.Stories) != 1 {
		t.Fatalf("first stories = %+v, want read-time materialized Texture edition article", firstStories)
	}
	firstStory := firstStories.Stories[0]
	if firstStory.StoryTextureDoc == "" ||
		firstStory.SourceState != "universal-wire-edition-texture" ||
		strings.Contains(firstStory.SourceState, "objectgraph-web-capture") ||
		!strings.Contains(firstStory.TextureContent, "disrupted rail corridor") ||
		len(firstStory.Manifest.Lead) != 2 {
		t.Fatalf("first story = %+v, want synthesized Texture article with two source_ref leads", firstStory)
	}
	assertUniversalWireStoryAvoidsHelperCopyForTest(t, firstStory)
	assertUniversalWireStoryTextureReadableForTest(t, handler, firstStory, "")
	if firstStory.Manifest.Lead[0].OpenSurface != sourcecontract.OpenSurfaceSource ||
		firstStory.Manifest.Lead[0].ReaderArtifactState != sourcecontract.ReaderArtifactStateReady ||
		firstStory.Manifest.Lead[0].ReaderSnapshot == nil {
		t.Fatalf("first story lead lacks source-viewer reader provenance: %+v", firstStory.Manifest.Lead[0])
	}
	if publishCount != 1 {
		t.Fatalf("publish count = %d, want one platform publish before advertising story", publishCount)
	}
	if syncCount != 1 {
		t.Fatalf("sync count = %d, want one platform texture sync before advertising story", syncCount)
	}
	firstDoc, err := handler.rt.Store().GetDocument(ctx, firstStory.StoryTextureDoc, universalWirePlatformOwnerID())
	if err != nil {
		t.Fatalf("load materialized doc: %v", err)
	}
	if !slices.Contains(firstStories.Edition.IncludedDocIDs, firstStory.StoryTextureDoc) {
		t.Fatalf("edition = %+v, want materialized story doc transcluded", firstStories.Edition)
	}

	secondStories := getUniversalWireStoriesForTest(t, handler)
	secondDoc, err := handler.rt.Store().GetDocument(ctx, firstStory.StoryTextureDoc, universalWirePlatformOwnerID())
	if err != nil {
		t.Fatalf("reload materialized doc: %v", err)
	}
	if secondDoc.CurrentRevisionID != firstDoc.CurrentRevisionID ||
		secondStories.Edition.RevisionID != firstStories.Edition.RevisionID ||
		len(secondStories.Stories) != 1 ||
		secondStories.Stories[0].StoryTextureDoc != firstStory.StoryTextureDoc ||
		countStrings(secondStories.Edition.IncludedDocIDs, firstStory.StoryTextureDoc) != 1 {
		t.Fatalf("second stories/doc = %+v / %+v, want idempotent read after edition exists", secondStories, secondDoc)
	}
}

func TestPlatformdReadBaseURLPreservesSiblingDerivationAndDirectPlatformd(t *testing.T) {
	for _, key := range []string{
		"RUNTIME_PLATFORMD_URL",
		"PROXY_PLATFORMD_URL",
		"RUNTIME_VMCTL_URL",
		"PROXY_VMCTL_URL",
		"RUNTIME_GATEWAY_URL",
		"RUNTIME_MAILD_URL",
	} {
		t.Setenv(key, "")
	}

	t.Setenv("RUNTIME_PLATFORMD_URL", "http://10.203.154.1:8082")
	if got := platformdReadBaseURL(); got != "http://10.203.154.1:8086" {
		t.Fatalf("sibling runtime platformd URL = %q, want derived :8086", got)
	}

	t.Setenv("RUNTIME_PLATFORMD_URL", "http://127.0.0.1:8086")
	if got := platformdReadBaseURL(); got != "http://127.0.0.1:8086" {
		t.Fatalf("direct runtime platformd URL = %q, want direct :8086", got)
	}

	t.Setenv("RUNTIME_PLATFORMD_URL", "")
	t.Setenv("RUNTIME_VMCTL_URL", "http://10.203.154.1:8083")
	if got := platformdReadBaseURL(); got != "http://10.203.154.1:8086" {
		t.Fatalf("vmctl URL = %q, want derived :8086", got)
	}
}

func TestUniversalWireSynthesisSanitizesHelperCopyAndReadsStoryTexture(t *testing.T) {
	for _, tc := range []struct {
		name     string
		headline func([]universalWireSynthesisSource) string
		summary  func([]universalWireSynthesisSource) string
		tension  string
	}{
		{
			name: "legacy universal wire meta copy",
			headline: func(sources []universalWireSynthesisSource) string {
				return "Universal Wire live synthesis: " + sources[0].Title
			},
			summary: func([]universalWireSynthesisSource) string {
				return "Universal Wire selected 2 graph-backed source captures from the live sourcecycled feed and published one English synthesis article instead of exposing raw capture cards."
			},
			tension: "Later relevant source arrivals should revise this same live synthesis article until semantic story clustering can split independent events.",
		},
		{
			name: "deployed scaffold copy",
			headline: func(sources []universalWireSynthesisSource) string {
				return "Multiple reports converge on " + sources[0].Title
			},
			summary: func(sources []universalWireSynthesisSource) string {
				return fmt.Sprintf("2 incoming reports point to the same developing story. %s provides the lead signal, while %s adds a second angle for readers.", sources[0].Title, sources[1].Title)
			},
			tension: "A second source in the cluster adds a separate angle rather than repeating the same capture, so the reports read as one developing article instead of two isolated updates.",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, handler := testAPISetup(t)
			ctx := context.Background()
			now := time.Date(2026, 6, 26, 23, 5, 0, 0, time.UTC)
			sourcesForSynthesis := []universalWireSynthesisSource{
				{
					ItemID:       "srcitem-legacy-pt",
					SourceID:     "rss:pt-wire",
					FetchID:      "fetch-legacy-pt",
					Title:        "Telegram Post from Metropoles Telegram",
					URL:          "https://example.com/pt/telegram",
					CanonicalURL: "https://example.com/pt/telegram",
					Language:     "pt",
					Body:         "Autoridades locais relataram novas medidas enquanto equipes acompanhavam os efeitos regionais.",
					FetchedAt:    now.Add(-20 * time.Minute),
				},
				{
					ItemID:       "srcitem-legacy-en",
					SourceID:     "rss:en-wire",
					FetchID:      "fetch-legacy-en",
					Title:        "Regional officials describe the same developing update",
					URL:          "https://example.com/en/update",
					CanonicalURL: "https://example.com/en/update",
					Language:     "en",
					Body:         "Officials described the same developing update and said additional details would follow later in the day.",
					FetchedAt:    now.Add(-16 * time.Minute),
				},
			}
			legacyDoc, legacyRev, _, err := handler.rt.synthesizeUniversalWireSourceClusterTextureArticle(ctx, universalWireSynthesisClusterRequest{
				ClusterID: universalWireLiveSourcecycledClusterID,
				Headline:  tc.headline(sourcesForSynthesis),
				Summary:   tc.summary(sourcesForSynthesis),
				Tension:   tc.tension,
				Sources:   sourcesForSynthesis,
				Now:       now,
			})
			if err != nil {
				t.Fatalf("seed legacy synthesis article: %v", err)
			}
			stories := getUniversalWireStoriesForTest(t, handler)
			if stories.Source != "universal-wire-edition-texture" ||
				stories.Diagnostics != nil ||
				stories.Edition == nil ||
				len(stories.Stories) != 1 {
				t.Fatalf("stories = %+v, want repaired edition Texture story", stories)
			}
			story := stories.Stories[0]
			if story.StoryTextureDoc != legacyDoc.DocID ||
				strings.Contains(story.Headline, "Universal Wire live synthesis") ||
				strings.Contains(story.TextureContent, "Universal Wire selected") ||
				strings.Contains(story.TextureContent, "graph-backed source captures") ||
				!strings.Contains(story.TextureContent, "available reporting") {
				t.Fatalf("story was not synthesized as article-facing copy: %+v", story)
			}
			assertUniversalWireStoryAvoidsHelperCopyForTest(t, story)
			docResp, revsResp := assertUniversalWireStoryTextureReadableForTest(t, handler, story, "")
			if strings.Contains(docResp.Title, "Universal Wire live synthesis") ||
				strings.Contains(docResp.Title, "Multiple reports converge") {
				t.Fatalf("readable Texture document title was not sanitized: %+v", docResp)
			}
			if len(revsResp.Revisions) == 0 || revsResp.Revisions[0].RevisionID != docResp.CurrentRevisionID || legacyRev.RevisionID != docResp.CurrentRevisionID {
				t.Fatalf("revision list did not expose sanitized current revision: doc=%+v revisions=%+v", docResp, revsResp.Revisions)
			}
		})
	}
}

func TestHandleUniversalWireStoriesBackfillsSemanticStoryForLegacySynthesisRevision(t *testing.T) {
	_, handler := testAPISetup(t)
	doc := seedLegacyUniversalWireSynthesisTextureFixture(t, handler, "doc-legacy-wire-semantic")
	seedUniversalWireEditionFixture(t, handler, doc.DocID)

	resp := getUniversalWireStoriesForTest(t, handler)
	if resp.Source != "universal-wire-edition-texture" ||
		resp.Diagnostics != nil ||
		resp.Edition == nil ||
		len(resp.Stories) != 1 {
		t.Fatalf("stories = %+v, want one edition Texture story", resp)
	}
	story := resp.Stories[0]
	if story.StoryTextureDoc != doc.DocID || story.SemanticStory == nil {
		t.Fatalf("story = %+v, want semantic evidence for legacy Wire synthesis article", story)
	}
	if story.SemanticStory.SchemaVersion != "choir.universal_wire_story_cluster.semantic.legacy.v1" ||
		story.SemanticStory.WorldModelKind != "universal_wire_semantic_story" ||
		story.SemanticStory.StoryID == "" ||
		story.SemanticStory.ChangeType != "legacy_revision_projection" ||
		story.SemanticStory.CurrentSourceCount != 2 ||
		story.SemanticStory.SourceCount != 2 ||
		!slices.Contains(story.SemanticStory.SemanticSignature, "sourcecycled-live-legacy") ||
		!slices.Contains(story.SemanticStory.SemanticSignature, "srcitem-legacy-wire-a") ||
		!slices.Contains(story.SemanticStory.SemanticSignature, "srcitem-legacy-wire-b") {
		t.Fatalf("semantic evidence = %+v, want legacy projection from durable synthesis metadata", story.SemanticStory)
	}
	assertUniversalWireStoryAvoidsHelperCopyForTest(t, story)
	readerCopy := strings.Join([]string{story.Headline, story.Dek, story.TextureContent}, "\n")
	if strings.Contains(readerCopy, story.SemanticStory.StoryID) ||
		strings.Contains(readerCopy, "universal_wire_semantic_story_id") ||
		strings.Contains(readerCopy, "World-model") {
		t.Fatalf("reader-facing story copy leaked semantic metadata: story=%+v", story)
	}
	encoded, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal stories response: %v", err)
	}
	if !strings.Contains(string(encoded), `"semantic_story"`) ||
		strings.Contains(string(encoded), "universal_wire_semantic_story_id") {
		t.Fatalf("encoded product response = %s, want semantic_story without internal revision metadata key", string(encoded))
	}
}

func universalWireSourcecycledTestItem(id, sourceID, fetchID, title, url, language, body string, fetchedAt time.Time) sources.Item {
	return sources.Item{
		ID:           id,
		SourceID:     sourceID,
		SourceType:   sources.SourceTypeRSS,
		FetchID:      fetchID,
		OriginalID:   url,
		Title:        title,
		Body:         body,
		URL:          url,
		CanonicalURL: url,
		Language:     language,
		FetchedAt:    fetchedAt,
		ContentHash:  sources.ContentHash(title, body, url, url),
	}
}

func postInternalSourcecycledWebCapturesForTest(t *testing.T, handler *APIHandler, items []sources.Item, now time.Time) internalSourcecycledWebCapturesResponse {
	t.Helper()
	body, err := json.Marshal(internalSourcecycledWebCapturesRequest{
		OwnerID: universalWirePlatformOwnerID(),
		Items:   items,
		Now:     now.Format(time.RFC3339Nano),
	})
	if err != nil {
		t.Fatalf("marshal sourcecycled request: %v", err)
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
		t.Fatalf("decode sourcecycled projection response: %v", err)
	}
	return projection
}

func getUniversalWireStoriesForTest(t *testing.T, handler *APIHandler) universalWireStoriesResponse {
	t.Helper()
	w := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/universal-wire/stories", "", "reader-1")
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/universal-wire/stories status = %d body=%s", w.Code, w.Body.String())
	}
	var stories universalWireStoriesResponse
	if err := json.NewDecoder(w.Body).Decode(&stories); err != nil {
		t.Fatalf("decode Universal Wire stories: %v", err)
	}
	return stories
}

func assertUniversalWireStoryAvoidsHelperCopyForTest(t *testing.T, story types.WireStory) {
	t.Helper()
	text := strings.Join([]string{story.Headline, story.Dek, story.TextureContent}, "\n")
	for _, forbidden := range []string{
		"Universal Wire live synthesis",
		"Universal Wire selected",
		"graph-backed source captures",
		"incoming reports point to the same developing story",
		"source cluster",
		"reports read as one developing article",
		"gives the clearest current account",
		"second sourced angle",
		"The second account narrows what readers can trust now",
		"Multiple reports converge",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("story contains helper/meta copy %q: %+v", forbidden, story)
		}
	}
}

func assertWireStorySemanticStateForTest(t *testing.T, story types.WireStory, want universalWireSemanticStoryState) {
	t.Helper()
	if story.SemanticStory == nil {
		t.Fatalf("story semantic evidence = nil for story %+v", story)
	}
	got := story.SemanticStory
	if got.SchemaVersion != want.SchemaVersion ||
		got.WorldModelKind != want.WorldModelKind ||
		got.StoryID != want.StoryID ||
		got.ChangeType != want.LatestChange.ChangeType ||
		got.PreviousSourceCount != want.LatestChange.PreviousSourceCount ||
		got.CurrentSourceCount != want.LatestChange.CurrentSourceCount ||
		got.SourceCount != want.SourceCount ||
		got.ChangedAt != want.LatestChange.ChangedAt ||
		!slices.Equal(got.SemanticSignature, want.SemanticSignature) ||
		!slices.Equal(got.TopicConcepts, want.TopicConcepts) ||
		!slices.Equal(got.SignalConcepts, want.SignalConcepts) {
		t.Fatalf("story semantic evidence = %+v, want cluster semantic state %+v", got, want)
	}
	readerCopy := strings.Join([]string{story.Headline, story.Dek, story.TextureContent}, "\n")
	if strings.Contains(readerCopy, got.StoryID) ||
		strings.Contains(readerCopy, "universal_wire_semantic_story_id") ||
		strings.Contains(readerCopy, "World-model") {
		t.Fatalf("reader-facing story copy leaked semantic metadata id/state: story=%+v semantic=%+v", story, got)
	}
}

func assertUniversalWireStoryTextureReadableForTest(t *testing.T, handler *APIHandler, story types.WireStory, wantRevisionID string) (textureDocumentResponse, textureListRevisionsResponse) {
	t.Helper()
	if strings.TrimSpace(story.StoryTextureDoc) == "" {
		t.Fatalf("story_texture_doc_id is empty for story %+v", story)
	}
	docPath := "/api/texture/documents/" + story.StoryTextureDoc + "?read_owner=universal-wire-platform"
	docW := registeredRuntimeRequest(t, handler, http.MethodGet, docPath, "", "reader-1")
	if docW.Code != http.StatusOK {
		t.Fatalf("GET returned Wire story Texture document status = %d body=%s story=%+v", docW.Code, docW.Body.String(), story)
	}
	var docResp textureDocumentResponse
	if err := json.NewDecoder(docW.Body).Decode(&docResp); err != nil {
		t.Fatalf("decode readable Wire Texture document: %v", err)
	}
	if docResp.DocID != story.StoryTextureDoc ||
		docResp.OwnerID != universalWirePlatformOwnerID() ||
		strings.TrimSpace(docResp.CurrentRevisionID) == "" {
		t.Fatalf("document response = %+v, want platform story document %s", docResp, story.StoryTextureDoc)
	}
	if wantRevisionID != "" && docResp.CurrentRevisionID != wantRevisionID {
		t.Fatalf("document current revision = %q, want %q", docResp.CurrentRevisionID, wantRevisionID)
	}

	revsPath := "/api/texture/documents/" + story.StoryTextureDoc + "/revisions?read_owner=universal-wire-platform"
	revsW := registeredRuntimeRequest(t, handler, http.MethodGet, revsPath, "", "reader-1")
	if revsW.Code != http.StatusOK {
		t.Fatalf("GET returned Wire story Texture revisions status = %d body=%s story=%+v", revsW.Code, revsW.Body.String(), story)
	}
	var revsResp textureListRevisionsResponse
	if err := json.NewDecoder(revsW.Body).Decode(&revsResp); err != nil {
		t.Fatalf("decode readable Wire Texture revisions: %v", err)
	}
	if len(revsResp.Revisions) == 0 ||
		revsResp.Revisions[0].DocID != story.StoryTextureDoc ||
		revsResp.Revisions[0].OwnerID != universalWirePlatformOwnerID() {
		t.Fatalf("revision response = %+v, want platform story document revisions", revsResp)
	}
	if wantRevisionID != "" && revsResp.Revisions[0].RevisionID != wantRevisionID {
		t.Fatalf("first revision = %q, want current %q", revsResp.Revisions[0].RevisionID, wantRevisionID)
	}
	return docResp, revsResp
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

func universalWireStoryWithSourceTitleForTest(stories []types.WireStory, titlePart string) *types.WireStory {
	for i := range stories {
		for _, source := range stories[i].Manifest.Lead {
			if strings.Contains(source.Title, titlePart) {
				return &stories[i]
			}
		}
	}
	return nil
}

func universalWireStoryWithDocForTest(stories []types.WireStory, docID string) *types.WireStory {
	for i := range stories {
		if stories[i].StoryTextureDoc == docID {
			return &stories[i]
		}
	}
	return nil
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

func seedLegacyUniversalWireSynthesisTextureFixture(t *testing.T, handler *APIHandler, docID string) types.Document {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()
	doc := types.Document{
		DocID:     docID,
		OwnerID:   universalWirePlatformOwnerID(),
		Title:     "Legacy Wire synthesis.texture",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := handler.rt.Store().CreateDocument(ctx, doc); err != nil {
		t.Fatalf("create legacy Wire synthesis doc: %v", err)
	}
	meta, _ := json.Marshal(map[string]any{
		"source":                            "edit_texture",
		"revision_role":                     textureRevisionRoleCanonical,
		"artifact_kind":                     "article_revision",
		"texture_version_stage":             "article_revision",
		"source_network_cycle_id":           "sourcecycled-live-legacy",
		"ingestion_handoff_cycle_id":        "sourcecycled-live-legacy",
		"ingestion_handoff_request_id":      "universal-wire-synthesis-cluster:sourcecycled-live-legacy",
		"ingestion_handoff_request_kind":    "synthesis_cluster",
		"universal_wire_synthesis":          true,
		"universal_wire_story_cluster_id":   "sourcecycled-live-legacy",
		"universal_wire_article_alias_path": "universal-wire/articles/sourcecycled-live-legacy.texture",
		"synthesis_source_count":            2,
		"source_item_ids":                   []string{"srcitem-legacy-wire-a", "srcitem-legacy-wire-b"},
		"selected_style_sources":            []map[string]any{{"title": "Style.texture: Universal Wire"}},
		"selected_style_rationale":          "Legacy deployed Wire article created before semantic metadata projection.",
		"platformd_route_path":              "/pub/texture/universal-wire/sourcecycled-live-legacy",
	})
	content := strings.Join([]string{
		"# Legacy Wire synthesis",
		"",
		"Legacy Wire synthesis describes a developing story that remains open to revision as more details arrive.",
		"",
		"Further reporting should revise this article if the timeline, affected people, or official account changes.",
	}, "\n")
	rev := types.Revision{
		RevisionID:  "rev-" + doc.DocID,
		DocID:       doc.DocID,
		OwnerID:     doc.OwnerID,
		AuthorKind:  types.AuthorAppAgent,
		AuthorLabel: "texture:" + doc.DocID,
		Content:     content,
		BodyDoc:     runtimeTestTextureBodyDoc(t, doc.DocID, "rev-"+doc.DocID, content),
		Citations:   json.RawMessage("[]"),
		Metadata:    meta,
		CreatedAt:   now,
	}
	if err := handler.rt.Store().CreateRevision(ctx, rev); err != nil {
		t.Fatalf("create legacy Wire synthesis revision: %v", err)
	}
	return doc
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

func TestUniversalWireSynthesisClusterCreatesTextureArticleAndEdition(t *testing.T) {
	_, handler := testAPISetup(t)
	ctx := context.Background()
	now := time.Date(2026, 6, 26, 21, 10, 0, 0, time.UTC)
	seedUniversalWireWebCaptureFixture(t, handler,
		"Raw capture should remain diagnostic",
		"https://example.test/raw-capture",
		"This raw capture exists only to prove the synthesized Texture article wins the public route.",
		now.Add(-time.Hour))

	doc, rev, editionRef, err := handler.rt.synthesizeUniversalWireSourceClusterTextureArticle(ctx, universalWireSynthesisClusterRequest{
		ClusterID: "cluster-rail-flooding-20260626",
		Headline:  "Flooding disruption around the rail corridor becomes a regional transport story",
		Summary:   "Two multilingual reports describe the same developing transport disruption from different angles, so readers get one combined article rather than separate raw updates.",
		Tension:   "The next update should revise this article if later source arrivals change the reopening timeline.",
		Sources: []universalWireSynthesisSource{
			{
				ItemID:       "srcitem-portuguese-rail",
				SourceID:     "rss:pt-transport",
				FetchID:      "fetch-pt-rail",
				Title:        "Corredor ferroviario reabre parcialmente apos enchentes",
				URL:          "https://example.test/pt/rail",
				CanonicalURL: "https://example.test/pt/rail",
				Language:     "pt",
				Body:         "Equipes de emergencia informaram que o corredor ferroviario reabriu parcialmente depois das enchentes, com inspecoes ainda em andamento.",
				FetchedAt:    now.Add(-20 * time.Minute),
			},
			{
				ItemID:       "srcitem-spanish-commuters",
				SourceID:     "rss:es-commuters",
				FetchID:      "fetch-es-commuters",
				Title:        "Autoridades advierten demoras para pasajeros regionales",
				URL:          "https://example.test/es/commuters",
				CanonicalURL: "https://example.test/es/commuters",
				Language:     "es",
				Body:         "Las autoridades regionales pidieron a los pasajeros prever demoras mientras continuaban las revisiones de seguridad en las estaciones afectadas.",
				FetchedAt:    now.Add(-15 * time.Minute),
			},
		},
		Now: now,
	})
	if err != nil {
		t.Fatalf("synthesize Universal Wire cluster: %v", err)
	}
	if doc.OwnerID != universalWirePlatformOwnerID() || doc.CurrentRevisionID != rev.RevisionID || editionRef == "" {
		t.Fatalf("synthesis doc/revision/edition = %+v/%+v/%q, want platform article linked into edition", doc, rev, editionRef)
	}
	if !strings.Contains(rev.Content, "[1]") || !strings.Contains(rev.Content, "[2]") ||
		strings.Contains(rev.Content, "source:") ||
		strings.Contains(rev.Content, "Equipes de emergencia informaram") {
		t.Fatalf("synthesis revision content did not project native source_refs without copying source body: %q", rev.Content)
	}
	var structured []texturedoc.SourceEntity
	if err := json.Unmarshal(rev.SourceEntities, &structured); err != nil {
		t.Fatalf("decode synthesis source_entities: %v", err)
	}
	if len(structured) != 2 {
		t.Fatalf("source_entities len = %d, want two cited source entities: %#v", len(structured), structured)
	}
	for _, entity := range structured {
		if entity.Evidence.OpenSurface != sourcecontract.OpenSurfaceSource ||
			entity.Evidence.ReaderArtifactState != sourcecontract.ReaderArtifactStateReady ||
			metadataString(entity.ReaderSnapshot, "text_content") == "" ||
			metadataString(entity.Target.Metadata, "source_id") == "" ||
			metadataString(entity.Target.Metadata, "fetch_id") == "" {
			t.Fatalf("structured source entity missing Source Viewer/reader context: %#v", entity)
		}
	}
	var bodyDoc texturedoc.StructuredTextureDoc
	if err := json.Unmarshal(rev.BodyDoc, &bodyDoc); err != nil {
		t.Fatalf("decode synthesis body_doc: %v", err)
	}
	visible := wireArticleVisibleStructuredSourceEntities(rev)
	if len(visible) != 2 {
		t.Fatalf("visible structured sources = %#v, want two source_ref-cited entities", visible)
	}
	if bodyDoc.Doc.Type != "doc" || !strings.Contains(string(rev.BodyDoc), `"source_ref"`) {
		t.Fatalf("synthesis body_doc missing native source_ref nodes: %s", string(rev.BodyDoc))
	}

	w := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/universal-wire/stories", "", "user-universal-wire")
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/universal-wire/stories status = %d body=%s", w.Code, w.Body.String())
	}
	var resp universalWireStoriesResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode stories response: %v", err)
	}
	if resp.Source != "universal-wire-edition-texture" || resp.Diagnostics != nil {
		t.Fatalf("response source/diagnostics = %q/%+v, want non-empty edition Texture route", resp.Source, resp.Diagnostics)
	}
	if resp.Edition == nil || !slices.Contains(resp.Edition.IncludedDocIDs, doc.DocID) {
		t.Fatalf("edition = %+v, want synthesized article included", resp.Edition)
	}
	if len(resp.Stories) != 1 {
		t.Fatalf("stories len = %d, want one synthesized article: %+v", len(resp.Stories), resp.Stories)
	}
	story := resp.Stories[0]
	if story.SourceState != "universal-wire-edition-texture" ||
		story.StoryTextureDoc != doc.DocID ||
		story.ID != "source-network-texture-"+doc.DocID ||
		strings.Contains(story.SourceState, "objectgraph-web-capture") {
		t.Fatalf("story is not the synthesized Texture article: %+v", story)
	}
	if !strings.Contains(story.TextureContent, "same developing transport disruption") ||
		strings.Contains(story.TextureContent, "Universal Wire publishes") ||
		!strings.Contains(story.TextureContent, "[1]") ||
		!strings.Contains(story.TextureContent, "[2]") {
		t.Fatalf("story texture content did not carry synthesized cited prose: %q", story.TextureContent)
	}
	assertUniversalWireStoryAvoidsHelperCopyForTest(t, story)
	if len(story.Manifest.Lead) != 2 {
		t.Fatalf("manifest lead len = %d, want two cited source handles: %+v", len(story.Manifest.Lead), story.Manifest)
	}
	firstLead := story.Manifest.Lead[0]
	if firstLead.ID != "srcitem-portuguese-rail" ||
		firstLead.SourceID != "rss:pt-transport" ||
		firstLead.FetchID != "fetch-pt-rail" ||
		firstLead.CanonicalURL != "https://example.test/pt/rail" ||
		firstLead.OpenSurface != sourcecontract.OpenSurfaceSource ||
		firstLead.ReaderArtifactState != sourcecontract.ReaderArtifactStateReady ||
		firstLead.ReaderSnapshot == nil ||
		!strings.Contains(firstLead.ReaderSnapshot.TextContent, "corredor ferroviario") {
		t.Fatalf("manifest did not carry source-open reader context for cited source: %+v", firstLead)
	}
	storyJSON, err := json.Marshal(story)
	if err != nil {
		t.Fatalf("marshal story: %v", err)
	}
	if strings.Contains(string(storyJSON), `"source_state":"objectgraph-web-capture"`) ||
		!strings.Contains(string(storyJSON), `"story_texture_doc_id":"`+doc.DocID+`"`) ||
		!strings.Contains(string(storyJSON), `"reader_snapshot"`) {
		t.Fatalf("story JSON did not expose Texture article with reader-backed sources: %s", string(storyJSON))
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

func TestHandleUniversalWireStoriesMaterializesLegacyGraphCapturesWithoutSourceEdges(t *testing.T) {
	_, handler := testAPISetup(t)
	now := time.Date(2026, 6, 27, 1, 18, 0, 0, time.UTC)
	seedUniversalWireWebCaptureFixture(t, handler,
		"Rail corridor inspection update",
		"https://example.test/rail-inspection",
		"PARIS -- Transit officials said rail corridor inspections would continue while commuters returned to central stations.",
		now.Add(-2*time.Hour))
	seedUniversalWireWebCaptureFixture(t, handler,
		"Rail corridor reopens",
		"https://example.test/rail",
		"PARIS -- Emergency crews reopened the rail corridor after flooding, with regional authorities saying inspections will continue through the afternoon.",
		now)

	resp := getUniversalWireStoriesForTest(t, handler)
	if resp.Source != "universal-wire-edition-texture" ||
		resp.Diagnostics != nil ||
		resp.Edition == nil ||
		len(resp.Stories) != 1 {
		t.Fatalf("stories = %+v, want read-time materialized Texture article from legacy graph captures", resp)
	}
	story := resp.Stories[0]
	if story.StoryTextureDoc == "" ||
		story.SourceState != "universal-wire-edition-texture" ||
		strings.Contains(story.SourceState, "objectgraph-web-capture") ||
		strings.Contains(story.TextureContent, "Universal Wire selected") ||
		!strings.Contains(story.TextureContent, "available reporting") ||
		len(story.Manifest.Lead) != 2 {
		t.Fatalf("story = %+v, want synthesized Texture article with two graph-capture cited sources", story)
	}
	assertUniversalWireStoryAvoidsHelperCopyForTest(t, story)
	assertUniversalWireStoryTextureReadableForTest(t, handler, story, "")
	for _, lead := range story.Manifest.Lead {
		if lead.OpenSurface != sourcecontract.OpenSurfaceSource ||
			lead.ReaderArtifactState != sourcecontract.ReaderArtifactStateReady ||
			lead.ReaderSnapshot == nil ||
			lead.ReaderSnapshot.TextContent == "" ||
			lead.SourceKind != "source_service_item" {
			t.Fatalf("lead lacks source-viewer reader provenance: %+v", lead)
		}
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
