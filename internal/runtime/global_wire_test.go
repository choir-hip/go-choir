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

func TestHandleGlobalWireStoriesSeedsDurableStoryGraphAndVTexts(t *testing.T) {
	_, handler := testAPISetup(t)

	w := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/global-wire/stories", "", "user-global-wire")
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/global-wire/stories status = %d body=%s", w.Code, w.Body.String())
	}
	var resp globalWireStoriesResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode stories response: %v", err)
	}
	if resp.Source != "durable-storygraph" {
		t.Fatalf("source = %q, want durable-storygraph", resp.Source)
	}
	if len(resp.Stories) != 3 {
		t.Fatalf("stories length = %d, want 3", len(resp.Stories))
	}
	story := resp.Stories[0]
	if story.StoryVTextDoc == "" {
		t.Fatalf("story has no linked VText doc: %+v", story)
	}
	if story.ProjectionVTextDocs["claim-audit-style"] == "" {
		t.Fatalf("story has no claim-audit projection VText doc: %+v", story.ProjectionVTextDocs)
	}
	if len(story.Manifest.Lead) == 0 || len(story.Manifest.Supporting) == 0 || len(story.Manifest.Contrary) == 0 || len(story.Manifest.Context) == 0 {
		t.Fatalf("story manifest is missing required evidence tiers: %+v", story.Manifest)
	}
	if story.Manifest.Lead[0].ContentID == "" {
		t.Fatalf("lead source has no backing content item: %+v", story.Manifest.Lead[0])
	}
	if len(resp.StyleSources) != 3 {
		t.Fatalf("style_sources length = %d, want 3", len(resp.StyleSources))
	}
	if resp.StyleSources[0].DocID == "" {
		t.Fatalf("style source has no citeable VText doc: %+v", resp.StyleSources[0])
	}

	docW := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/vtext/documents/"+story.StoryVTextDoc, "", "user-global-wire")
	if docW.Code != http.StatusOK {
		t.Fatalf("get linked story VText status = %d body=%s", docW.Code, docW.Body.String())
	}
	projectionW := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/vtext/documents/"+story.ProjectionVTextDocs["claim-audit-style"], "", "user-global-wire")
	if projectionW.Code != http.StatusOK {
		t.Fatalf("get linked projection VText status = %d body=%s", projectionW.Code, projectionW.Body.String())
	}
	sourceW := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/content/items/"+story.Manifest.Lead[0].ContentID, "", "user-global-wire")
	if sourceW.Code != http.StatusOK {
		t.Fatalf("get linked source content item status = %d body=%s", sourceW.Code, sourceW.Body.String())
	}
}

func TestHandleGlobalWireStoriesIndexesSourceNetworkVTextHeads(t *testing.T) {
	_, handler := testAPISetup(t)
	ctx := context.Background()
	now := time.Now().UTC()
	doc := types.Document{
		DocID:     "doc-source-maxx-live",
		OwnerID:   "global-wire-platform",
		Title:     "Madrid dispatch.vtext",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := handler.rt.Store().CreateDocument(ctx, doc); err != nil {
		t.Fatalf("create platform source maxx doc: %v", err)
	}
	meta, _ := json.Marshal(map[string]any{
		"source":                   "edit_vtext",
		"source_maxx_cycle_id":     "cycle-live",
		"source_maxx_request_id":   "reconciler-live",
		"source_maxx_request_kind": "reconciler",
		"selected_style_sources":   []map[string]any{{"title": "Style.vtext: Global Wire"}},
		"selected_style_rationale": "Global Wire style fits a fast sourced dispatch.",
		"source_item_ids":          []string{"srcitem_live_1", "srcitem_live_2"},
	})
	rev := types.Revision{
		RevisionID:  "rev-source-maxx-live",
		DocID:       doc.DocID,
		OwnerID:     doc.OwnerID,
		AuthorKind:  types.AuthorAppAgent,
		AuthorLabel: "vtext:doc-source-maxx-live",
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

	w := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/global-wire/stories", "", "user-global-wire")
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/global-wire/stories status = %d body=%s", w.Code, w.Body.String())
	}
	var resp globalWireStoriesResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode stories response: %v", err)
	}
	if resp.Source != "durable-storygraph+source-network-vtexts" {
		t.Fatalf("source = %q, want source network vtext index", resp.Source)
	}
	if len(resp.Stories) < 4 {
		t.Fatalf("stories length = %d, want source maxx story plus seeded stories", len(resp.Stories))
	}
	story := resp.Stories[0]
	if story.ID != "source-network-vtext-"+doc.DocID ||
		story.OwnerID != "global-wire-platform" ||
		story.StoryVTextDoc != doc.DocID ||
		story.VTextContent == "" {
		t.Fatalf("first story is not the indexed source-network VText: %+v", story)
	}
	if story.Headline != "Madrid dispatch" || !strings.Contains(story.Projections["wire-style"], "MADRID -- Pope Leo XIV") {
		t.Fatalf("indexed source-network story did not expose article head: %+v", story)
	}
	if len(story.Manifest.Lead) != 0 || len(story.Manifest.Context) != 1 ||
		story.Manifest.Context[0].ID != "source-network-cycle:cycle-live" ||
		!strings.Contains(story.Manifest.Context[0].Standing, "2 source handles retained in revision provenance") {
		t.Fatalf("indexed source-network story should expose bounded cycle provenance, got %+v", story.Manifest)
	}
	claimText := strings.Join(story.Claims, "\n")
	if strings.Contains(claimText, "Style.vtext: Global Wire") ||
		!strings.Contains(claimText, "Source and style provenance are carried by the VText revision metadata and citations") {
		t.Fatalf("indexed source-network story claims did not preserve provenance/body separation: %+v", story.Claims)
	}
}

func TestHandleGlobalWireStoriesUsesVisibleSourceEntitiesForSourceNetworkManifest(t *testing.T) {
	_, handler := testAPISetup(t)
	ctx := context.Background()
	now := time.Now().UTC()
	doc := types.Document{
		DocID:     "doc-source-network-scoped-sources",
		OwnerID:   "global-wire-platform",
		Title:     "Scoped sources.vtext",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := handler.rt.Store().CreateDocument(ctx, doc); err != nil {
		t.Fatalf("create platform source-network doc: %v", err)
	}
	meta, _ := json.Marshal(map[string]any{
		"source":                   "edit_vtext",
		"source_maxx_cycle_id":     "cycle-scoped",
		"source_maxx_request_id":   "reconciler-scoped",
		"source_maxx_request_kind": "reconciler",
		"selected_style_sources":   []map[string]any{{"title": "Style.vtext: Global Wire"}},
		"source_item_ids":          []string{"srcitem_cycle_1", "srcitem_cycle_2", "srcitem_cycle_3", "srcitem_cycle_4"},
		"source_entities": []map[string]any{
			{
				"entity_id": "src_cited_one",
				"kind":      "source_service_item",
				"label":     "Regional wire bulletin",
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
			"Selection rationale: Global Wire style.",
		}, "\n"),
		Citations: json.RawMessage("[]"),
		Metadata:  meta,
		CreatedAt: now,
	}
	if err := handler.rt.Store().CreateRevision(ctx, rev); err != nil {
		t.Fatalf("create platform source-network revision: %v", err)
	}

	w := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/global-wire/stories", "", "user-global-wire")
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/global-wire/stories status = %d body=%s", w.Code, w.Body.String())
	}
	var resp globalWireStoriesResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode stories response: %v", err)
	}
	story := resp.Stories[0]
	if story.ID != "source-network-vtext-"+doc.DocID {
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
}

func TestHandleGlobalWireStyleSourcesComposeAndReplace(t *testing.T) {
	_, handler := testAPISetup(t)

	composeBody := `{"story_id":"story-supply-resilience","action":"compose","base_style_ids":["wire-style","claim-audit-style"],"title":"Style.vtext: Wire Audit Hybrid","label":"Hybrid","summary":"Hybrid style preserving wire speed and claim-audit uncertainty."}`
	composeW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/global-wire/style-sources", composeBody, "user-style")
	if composeW.Code != http.StatusCreated {
		t.Fatalf("compose style status = %d body=%s", composeW.Code, composeW.Body.String())
	}
	var composeResp globalWireStyleSourceResponse
	if err := json.NewDecoder(composeW.Body).Decode(&composeResp); err != nil {
		t.Fatalf("decode compose style: %v", err)
	}
	if composeResp.Style.ID == "" ||
		composeResp.Style.DocID != composeResp.Document.DocID ||
		composeResp.Revision.AuthorKind != types.AuthorAppAgent ||
		composeResp.Projection.StyleID != composeResp.Style.ID ||
		composeResp.Projection.StyleDocID != composeResp.Style.DocID {
		t.Fatalf("compose response missing style/projection lineage: %+v", composeResp)
	}
	if !strings.Contains(composeResp.Revision.Content, "Parent Style.vtext Sources") ||
		!strings.Contains(composeResp.Projection.Text, "source:gw-src-") ||
		strings.Contains(composeResp.Projection.Text, "StoryGraph id:") ||
		strings.Contains(composeResp.Projection.Text, "Projection relation:") ||
		strings.Contains(composeResp.Projection.Text, "## Projection") ||
		strings.Contains(composeResp.Projection.Text, "## Evidence Invariant") {
		t.Fatalf("compose VText/projection content missing provenance: rev=%q projection=%q", composeResp.Revision.Content, composeResp.Projection.Text)
	}
	projectionDoc, err := handler.rt.Store().GetDocument(context.Background(), composeResp.Projection.StoryDocID, "user-style")
	if err != nil {
		t.Fatalf("get compose projection doc: %v", err)
	}
	projectionRev, err := handler.rt.Store().GetRevision(context.Background(), projectionDoc.CurrentRevisionID, "user-style")
	if err != nil {
		t.Fatalf("get compose projection revision: %v", err)
	}
	projectionMeta := decodeRevisionMetadata(projectionRev.Metadata)
	if projectionMeta["artifact_kind"] != "article_revision" || projectionMeta["article_version"] != true {
		t.Fatalf("compose projection metadata did not mark article revision: %#v", projectionMeta)
	}
	if len(decodeVTextSourceEntities(projectionMeta["source_entities"])) == 0 {
		t.Fatalf("compose projection metadata missing source entities: %#v", projectionMeta)
	}
	var citations []types.Citation
	if err := json.Unmarshal(composeResp.Revision.Citations, &citations); err != nil {
		t.Fatalf("decode compose citations: %v", err)
	}
	if len(citations) < 3 {
		t.Fatalf("compose style citations too sparse: %+v", citations)
	}

	storiesW := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/global-wire/stories", "", "user-style")
	if storiesW.Code != http.StatusOK {
		t.Fatalf("stories after compose status = %d body=%s", storiesW.Code, storiesW.Body.String())
	}
	var storiesResp globalWireStoriesResponse
	if err := json.NewDecoder(storiesW.Body).Decode(&storiesResp); err != nil {
		t.Fatalf("decode stories after compose: %v", err)
	}
	composedStory := storiesResp.Stories[0]
	if findGlobalWireStyleSource(composedStory.StyleSources, composeResp.Style.ID).ID == "" ||
		composedStory.ProjectionVTextDocs[composeResp.Style.ID] == "" ||
		strings.Contains(composedStory.Projections[composeResp.Style.ID], "Composed Style.vtext projection") ||
		strings.Contains(composedStory.Projections[composeResp.Style.ID], "Evidence manifest unchanged") {
		t.Fatalf("composed style not visible in StoryGraph response: %+v", composedStory)
	}

	replaceBody := `{"story_id":"story-supply-resilience","action":"replace","base_style_ids":["` + composeResp.Style.ID + `"],"replace_style_id":"` + composeResp.Style.ID + `","title":"Style.vtext: Replacement Hybrid","label":"Replace","summary":"Replacement style source with explicit provenance."}`
	replaceW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/global-wire/style-sources", replaceBody, "user-style")
	if replaceW.Code != http.StatusCreated {
		t.Fatalf("replace style status = %d body=%s", replaceW.Code, replaceW.Body.String())
	}
	var replaceResp globalWireStyleSourceResponse
	if err := json.NewDecoder(replaceW.Body).Decode(&replaceResp); err != nil {
		t.Fatalf("decode replace style: %v", err)
	}
	if replaceResp.Style.ID == composeResp.Style.ID ||
		findGlobalWireStyleSource(replaceResp.Story.StyleSources, replaceResp.Style.ID).ID == "" ||
		findGlobalWireStyleSource(replaceResp.Story.StyleSources, composeResp.Style.ID).ID != "" ||
		replaceResp.Projection.StyleID != replaceResp.Style.ID {
		t.Fatalf("replace style did not swap selectable source/projection: %+v", replaceResp)
	}
}

func TestHandleGlobalWireRequiresAuth(t *testing.T) {
	_, handler := testAPISetup(t)

	w := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/global-wire/stories", "", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unauth story status = %d body=%s", w.Code, w.Body.String())
	}
	w = registeredRuntimeRequest(t, handler, http.MethodPost, "/api/global-wire/contributions", `{"story_id":"story-supply-resilience","kind":"source"}`, "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unauth contribution status = %d body=%s", w.Code, w.Body.String())
	}
}

func TestHandleGlobalWireContributionsAreOwnerScoped(t *testing.T) {
	_, handler := testAPISetup(t)

	body := `{"story_id":"story-supply-resilience","kind":"source","headline":"Port backlog recedes","text":"Add carrier PDF before reconciliation."}`
	w := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/global-wire/contributions", body, "user-alpha")
	if w.Code != http.StatusCreated {
		t.Fatalf("create contribution status = %d body=%s", w.Code, w.Body.String())
	}
	var created map[string]any
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatalf("decode contribution: %v", err)
	}
	if created["research_state"] != "pending-researcher-review" {
		t.Fatalf("research_state = %v", created["research_state"])
	}
	if created["source_content_id"] == "" {
		t.Fatalf("source_content_id is empty in created contribution: %+v", created)
	}

	alpha := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/global-wire/contributions?story_id=story-supply-resilience", "", "user-alpha")
	if alpha.Code != http.StatusOK {
		t.Fatalf("list alpha contributions status = %d body=%s", alpha.Code, alpha.Body.String())
	}
	var alphaResp globalWireContributionListResponse
	if err := json.NewDecoder(alpha.Body).Decode(&alphaResp); err != nil {
		t.Fatalf("decode alpha contributions: %v", err)
	}
	if len(alphaResp.Contributions) != 1 {
		t.Fatalf("alpha contribution count = %d, want 1", len(alphaResp.Contributions))
	}
	if alphaResp.Contributions[0].SourceContentID == "" {
		t.Fatalf("persisted source_content_id is empty: %+v", alphaResp.Contributions[0])
	}
	sourceW := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/content/items/"+alphaResp.Contributions[0].SourceContentID, "", "user-alpha")
	if sourceW.Code != http.StatusOK {
		t.Fatalf("get contribution source item status = %d body=%s", sourceW.Code, sourceW.Body.String())
	}

	beta := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/global-wire/contributions?story_id=story-supply-resilience", "", "user-beta")
	if beta.Code != http.StatusOK {
		t.Fatalf("list beta contributions status = %d body=%s", beta.Code, beta.Body.String())
	}
	var betaResp globalWireContributionListResponse
	if err := json.NewDecoder(beta.Body).Decode(&betaResp); err != nil {
		t.Fatalf("decode beta contributions: %v", err)
	}
	if len(betaResp.Contributions) != 0 {
		t.Fatalf("beta contribution count = %d, want 0", len(betaResp.Contributions))
	}
}

func TestHandleGlobalWireContributionCanReferenceExistingContentItem(t *testing.T) {
	_, handler := testAPISetup(t)
	now := time.Now().UTC()
	item := types.ContentItem{
		ContentID:   "existing-global-wire-source",
		OwnerID:     "user-alpha",
		SourceType:  "text",
		MediaType:   "text/markdown",
		AppHint:     "global-wire",
		Title:       "Existing source",
		TextContent: "Existing imported source text.",
		Metadata:    []byte(`{"schema":"test.source"}`),
		Provenance:  []byte(`{"created_from":"test"}`),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := handler.rt.Store().CreateContentItem(context.Background(), item); err != nil {
		t.Fatalf("CreateContentItem: %v", err)
	}

	body := `{"story_id":"story-supply-resilience","kind":"source","headline":"Port backlog recedes","text":"Use the imported source.","source_content_id":"existing-global-wire-source"}`
	w := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/global-wire/contributions", body, "user-alpha")
	if w.Code != http.StatusCreated {
		t.Fatalf("create contribution with existing source status = %d body=%s", w.Code, w.Body.String())
	}
	var created types.GlobalWireContribution
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatalf("decode contribution: %v", err)
	}
	if created.SourceContentID != item.ContentID {
		t.Fatalf("source_content_id = %q, want %q", created.SourceContentID, item.ContentID)
	}

	beta := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/global-wire/contributions", body, "user-beta")
	if beta.Code != http.StatusBadRequest {
		t.Fatalf("cross-owner source contribution status = %d body=%s", beta.Code, beta.Body.String())
	}
}

func TestHandleGlobalWireSourceSearchImportsAndQueuesSourceServiceEvidence(t *testing.T) {
	sourceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/source-service/search" {
			t.Fatalf("unexpected source service path %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("q"); got != "port congestion" {
			t.Fatalf("query = %q, want port congestion", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(sourceapi.SearchResponse{
			Query:    "port congestion",
			Provider: sourceapi.ProviderName,
			Metadata: sourceapi.Metadata{TargetKind: sourceapi.TargetKind},
			Results: []sourceapi.ItemResult{{
				Rank:          1,
				TargetKind:    sourceapi.TargetKind,
				ItemID:        "srcitem_port_congestion",
				SourceID:      "rss:ports",
				SourceType:    "rss",
				FetchID:       "fetch-port-1",
				Title:         "Port congestion eases",
				Body:          "Terminal dwell times fell after additional rail slots opened.",
				URL:           "https://example.test/ports",
				CanonicalURL:  "https://example.test/ports",
				ContentHash:   "hash-port-congestion",
				EvidenceLevel: "source-service-ledger",
			}},
		})
	}))
	defer sourceServer.Close()
	t.Setenv("SOURCE_SERVICE_BASE_URL", sourceServer.URL)
	t.Setenv("SOURCE_SERVICE_URL", "")
	t.Setenv("SOURCECYCLED_API_URL", "")

	_, handler := testAPISetup(t)
	body := `{"query":"port congestion","max_results":2,"story_id":"story-supply-resilience","queue_top_result":true}`
	w := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/global-wire/source-search", body, "user-alpha")
	if w.Code != http.StatusOK {
		t.Fatalf("source search status = %d body=%s", w.Code, w.Body.String())
	}
	var resp globalWireSourceSearchResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode source search: %v", err)
	}
	if resp.Status != "ok" || len(resp.ContentItems) != 1 {
		t.Fatalf("unexpected source search response: %+v", resp)
	}
	item := resp.ContentItems[0]
	if item.SourceType != "source_service_item" || item.AppHint != "global-wire" || item.ContentHash != "hash-port-congestion" {
		t.Fatalf("unexpected imported item: %+v", item)
	}
	if resp.Contribution == nil || resp.Contribution.SourceContentID != item.ContentID {
		t.Fatalf("queued contribution missing source content: %+v", resp.Contribution)
	}
	if resp.Contribution.ResearchState != "pending-researcher-review" {
		t.Fatalf("research_state = %q", resp.Contribution.ResearchState)
	}
	sourceW := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/content/items/"+item.ContentID, "", "user-alpha")
	if sourceW.Code != http.StatusOK {
		t.Fatalf("get source-service content item status = %d body=%s", sourceW.Code, sourceW.Body.String())
	}
	var stored types.ContentItem
	if err := json.NewDecoder(sourceW.Body).Decode(&stored); err != nil {
		t.Fatalf("decode stored content item: %v", err)
	}
	var metadata map[string]any
	if err := json.Unmarshal(stored.Metadata, &metadata); err != nil {
		t.Fatalf("decode metadata: %v", err)
	}
	if metadata["schema"] != "choir.global_wire_source_service_item.v1" || metadata["source_item_id"] != "srcitem_port_congestion" {
		t.Fatalf("unexpected source-service metadata: %+v", metadata)
	}
}

func TestHandleGlobalWireSourceSearchReportsUnconfiguredSourceService(t *testing.T) {
	t.Setenv("SOURCE_SERVICE_BASE_URL", "")
	t.Setenv("SOURCE_SERVICE_URL", "")
	t.Setenv("SOURCECYCLED_API_URL", "")

	_, handler := testAPISetup(t)
	w := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/global-wire/source-search", `{"query":"rates"}`, "user-alpha")
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("unconfigured source search status = %d body=%s", w.Code, w.Body.String())
	}
	var resp globalWireSourceSearchResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode unconfigured source search: %v", err)
	}
	if resp.Status != "unavailable" || resp.Source != "source-service" {
		t.Fatalf("unexpected unconfigured response: %+v", resp)
	}
}

func TestHandleGlobalWireSourceMaxxStatusReportsAggregateHandoffs(t *testing.T) {
	rt, handler := testAPISetup(t)
	ctx := context.Background()
	now := time.Now().UTC()
	processorRun := types.RunRecord{
		RunID:        "run_processor_source_maxx",
		AgentID:      "processor:processor-global_firehose-global-gdelt",
		ChannelID:    "channel_source_maxx",
		AgentProfile: AgentProfileProcessor,
		AgentRole:    AgentProfileProcessor,
		OwnerID:      "global-wire-platform",
		SandboxID:    "sandbox-test",
		State:        types.RunPending,
		Prompt:       "Process SourceMaxx GDELT sources.",
		CreatedAt:    now,
		UpdatedAt:    now,
		Metadata: map[string]any{
			runMetadataAgentProfile: AgentProfileProcessor,
			runMetadataAgentRole:    AgentProfileProcessor,
			runMetadataTrajectoryID: "trajectory-source-maxx",
			runMetadataProcessorKey: "processor:global_firehose:global:gdelt",
			"request_source":        "sourcecycled",
		},
	}
	if err := rt.Store().CreateRun(ctx, processorRun); err != nil {
		t.Fatalf("CreateRun processor: %v", err)
	}
	reconcilerRun := types.RunRecord{
		RunID:        "run_reconciler_source_maxx",
		AgentID:      "reconciler:story-corpus",
		ChannelID:    "channel_source_maxx",
		AgentProfile: AgentProfileReconciler,
		AgentRole:    AgentProfileReconciler,
		OwnerID:      "global-wire-platform",
		SandboxID:    "sandbox-test",
		State:        types.RunRunning,
		Prompt:       "Reconcile story corpus.",
		CreatedAt:    now,
		UpdatedAt:    now,
		Metadata: map[string]any{
			runMetadataAgentProfile:    AgentProfileReconciler,
			runMetadataAgentRole:       AgentProfileReconciler,
			runMetadataTrajectoryID:    "trajectory-source-maxx",
			runMetadataReconcilerScope: "story-corpus",
			"request_source":           "sourcecycled",
		},
	}
	if err := rt.Store().CreateRun(ctx, reconcilerRun); err != nil {
		t.Fatalf("CreateRun reconciler: %v", err)
	}
	childRun := types.RunRecord{
		RunID:        "run_processor_researcher_child",
		AgentID:      "researcher:source-maxx-child",
		ChannelID:    "channel_source_maxx",
		ParentRunID:  processorRun.RunID,
		AgentProfile: AgentProfileResearcher,
		AgentRole:    AgentProfileResearcher,
		OwnerID:      "global-wire-platform",
		SandboxID:    "sandbox-test",
		State:        types.RunCompleted,
		Prompt:       "Research one source cluster.",
		CreatedAt:    now,
		UpdatedAt:    now,
		FinishedAt:   &now,
		Metadata: map[string]any{
			runMetadataAgentProfile: AgentProfileResearcher,
			runMetadataAgentRole:    AgentProfileResearcher,
			runMetadataTrajectoryID: "trajectory-source-maxx",
		},
	}
	if err := rt.Store().CreateRun(ctx, childRun); err != nil {
		t.Fatalf("CreateRun child researcher: %v", err)
	}
	update := types.WorkerUpdateRecord{
		UpdateID:      "update_processor_source_maxx",
		OwnerID:       processorRun.OwnerID,
		AgentID:       processorRun.AgentID,
		TargetAgentID: processorRun.AgentID,
		ChannelID:     processorRun.ChannelID,
		TrajectoryID:  "trajectory-source-maxx",
		Role:          AgentProfileProcessor,
		Kind:          "status",
		Summary:       "Processor consumed source firehose and requested research.",
		Content:       "Processor consumed source firehose and requested research.",
		CreatedAt:     now,
	}
	message := &types.ChannelMessage{
		ChannelID:    processorRun.ChannelID,
		From:         processorRun.AgentID,
		FromAgentID:  processorRun.AgentID,
		FromRunID:    processorRun.RunID,
		ToAgentID:    processorRun.AgentID,
		TrajectoryID: "trajectory-source-maxx",
		Role:         AgentProfileProcessor,
		Content:      update.Content,
		Timestamp:    now,
	}
	delivery := types.InboxDelivery{
		DeliveryID:   "delivery_processor_source_maxx",
		OwnerID:      processorRun.OwnerID,
		ToAgentID:    processorRun.AgentID,
		FromAgentID:  processorRun.AgentID,
		FromRunID:    processorRun.RunID,
		ChannelID:    processorRun.ChannelID,
		Role:         AgentProfileProcessor,
		Content:      update.Content,
		TrajectoryID: "trajectory-source-maxx",
		CreatedAt:    now,
	}
	if _, _, err := rt.Store().DispatchWorkerUpdate(ctx, update, message, delivery); err != nil {
		t.Fatalf("DispatchWorkerUpdate: %v", err)
	}

	sourceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/source-service/sourcemaxx/latest" {
			t.Fatalf("unexpected source service path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(sourceapi.SourceMaxxResponse{
			Provider: sourceapi.ProviderName,
			Cycle: sourceapi.CycleSummary{
				CycleID:    "cycle_source_maxx",
				StartedAt:  "2026-06-07T13:50:27Z",
				EndedAt:    "2026-06-07T13:50:27Z",
				Status:     "completed",
				ItemCount:  710,
				FetchCount: 14,
			},
			ProcessorRequests: []sourceapi.ProcessorRequest{
				{
					RequestID:    "processor_1",
					ProcessorKey: "processor:global_firehose:global:gdelt",
					Status:       "submitted",
					RuntimeRunID: processorRun.RunID,
					SourceCount:  50,
				},
				{
					RequestID:    "processor_2",
					ProcessorKey: "processor:conflict:global:telegram",
					Status:       "queued",
					RuntimeRunID: "run_processor_missing_from_request_runtime",
					SourceCount:  38,
				},
			},
			ReconcilerRequests: []sourceapi.ReconcilerRequest{{
				RequestID:    "reconciler_1",
				Scope:        "story-corpus",
				Status:       "submitted",
				RuntimeRunID: reconcilerRun.RunID,
			}},
			Metadata: sourceapi.SourceMaxxMetadata{
				Topology:      "source-items -> processor-handoffs -> corpus-reconciler-handoff",
				AuthorityRule: "source and version provenance stay in source items and VText",
			},
		})
	}))
	defer sourceServer.Close()
	t.Setenv("SOURCE_SERVICE_BASE_URL", sourceServer.URL)
	t.Setenv("SOURCE_SERVICE_URL", "")
	t.Setenv("SOURCECYCLED_API_URL", "")

	w := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/global-wire/source-status", "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("source status = %d body=%s", w.Code, w.Body.String())
	}
	var resp globalWireSourceMaxxStatusResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode sourcemaxx status: %v", err)
	}
	if resp.Status != "ok" || resp.ItemCount != 710 || resp.FetchCount != 14 {
		t.Fatalf("unexpected status response: %+v", resp)
	}
	if resp.ProcessorRequestCount != 2 || resp.ReconcilerRequestCount != 1 {
		t.Fatalf("unexpected handoff counts: %+v", resp)
	}
	if resp.ProcessorStatusCounts["submitted"] != 1 || resp.ProcessorStatusCounts["queued"] != 1 {
		t.Fatalf("unexpected processor status counts: %+v", resp.ProcessorStatusCounts)
	}
	if resp.ReconcilerStatusCounts["submitted"] != 1 {
		t.Fatalf("unexpected reconciler status counts: %+v", resp.ReconcilerStatusCounts)
	}
	if resp.ProcessorRuntimeRunCount != 2 || resp.ReconcilerRuntimeRunCount != 1 {
		t.Fatalf("unexpected runtime run counts: %+v", resp)
	}
	if resp.ProcessorResolvedRunCount != 1 || resp.ProcessorUnresolvedRunCount != 1 {
		t.Fatalf("unexpected processor runtime resolution counts: %+v", resp)
	}
	if resp.ReconcilerResolvedRunCount != 1 || resp.ReconcilerUnresolvedRunCount != 0 {
		t.Fatalf("unexpected reconciler runtime resolution counts: %+v", resp)
	}
	if resp.ProcessorRunStateCounts[string(types.RunPending)] != 1 || resp.ReconcilerRunStateCounts[string(types.RunRunning)] != 1 {
		t.Fatalf("unexpected runtime run states: processor=%+v reconciler=%+v", resp.ProcessorRunStateCounts, resp.ReconcilerRunStateCounts)
	}
	if resp.ProcessorUpdateCount != 1 {
		t.Fatalf("processor update count = %d, want 1", resp.ProcessorUpdateCount)
	}
	if resp.ProcessorChildProfileCounts[AgentProfileResearcher] != 1 {
		t.Fatalf("processor child profile counts = %+v, want researcher child", resp.ProcessorChildProfileCounts)
	}
	if len(resp.ProcessorKeys) != 2 || resp.ProcessorKeys[0] != "processor:global_firehose:global:gdelt" {
		t.Fatalf("unexpected processor keys: %+v", resp.ProcessorKeys)
	}
	if resp.AuthorityRule == "" || !resp.SourceServiceInternalOnly {
		t.Fatalf("missing provenance/internal boundary metadata: %+v", resp)
	}

	compat := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/global-wire/sourcemaxx-status", "", "")
	if compat.Code != http.StatusOK {
		t.Fatalf("legacy sourcemaxx status alias = %d body=%s", compat.Code, compat.Body.String())
	}
}

func TestHandleGlobalWireSourceMaxxStatusResolvesRemoteRuntimeEvidence(t *testing.T) {
	_, handler := testAPISetup(t)
	processorRunID := "run_processor_remote_source_maxx"
	reconcilerRunID := "run_reconciler_remote_source_maxx"
	runtimeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Internal-Caller") != "true" {
			t.Fatalf("missing internal caller header")
		}
		if r.URL.Query().Get("owner_id") != "global-wire-platform" {
			t.Fatalf("owner_id = %q", r.URL.Query().Get("owner_id"))
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/internal/runtime/runs/" + processorRunID:
			_ = json.NewEncoder(w).Encode(runStatusResponse{
				RunID:        processorRunID,
				AgentID:      "processor:remote",
				AgentProfile: AgentProfileProcessor,
				AgentRole:    AgentProfileProcessor,
				OwnerID:      "global-wire-platform",
				State:        types.RunCompleted,
			})
		case "/internal/runtime/runs/" + processorRunID + "/events":
			_ = json.NewEncoder(w).Encode(eventListResponse{Events: []types.EventRecord{
				{Kind: types.EventToolInvoked, Payload: json.RawMessage(`{"tool":"submit_coagent_update"}`)},
				{Kind: types.EventToolResult, Payload: json.RawMessage(`{"tool":"spawn_agent","output":"{\"profile\":\"researcher\",\"loop_id\":\"child_researcher\"}"}`)},
				{Kind: types.EventToolResult, Payload: json.RawMessage(`{"tool":"spawn_agent","output":"{\"profile\":\"vtext\",\"loop_id\":\"child_vtext\"}"}`)},
			}})
		case "/internal/runtime/runs/" + reconcilerRunID:
			_ = json.NewEncoder(w).Encode(runStatusResponse{
				RunID:        reconcilerRunID,
				AgentID:      "reconciler:story-corpus",
				AgentProfile: AgentProfileReconciler,
				AgentRole:    AgentProfileReconciler,
				OwnerID:      "global-wire-platform",
				State:        types.RunRunning,
			})
		case "/internal/runtime/runs/" + reconcilerRunID + "/events":
			_ = json.NewEncoder(w).Encode(eventListResponse{Events: []types.EventRecord{
				{Kind: types.EventToolInvoked, Payload: json.RawMessage(`{"tool":"submit_coagent_update"}`)},
				{Kind: types.EventToolResult, Payload: json.RawMessage(`{"tool":"spawn_agent","output":"{\"profile\":\"researcher\",\"loop_id\":\"child_researcher_2\"}"}`)},
			}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer runtimeServer.Close()

	sourceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/source-service/sourcemaxx/latest" {
			t.Fatalf("unexpected source service path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(sourceapi.SourceMaxxResponse{
			Provider: sourceapi.ProviderName,
			Cycle: sourceapi.CycleSummary{
				CycleID:    "cycle_remote_runtime",
				StartedAt:  "2026-06-07T15:35:44Z",
				EndedAt:    "2026-06-07T15:35:45Z",
				Status:     "completed",
				ItemCount:  502,
				FetchCount: 14,
			},
			ProcessorRequests: []sourceapi.ProcessorRequest{{
				RequestID:    "processor_remote",
				ProcessorKey: "processor:global_firehose:global:gdelt",
				Status:       "submitted",
				RuntimeRunID: processorRunID,
			}},
			ReconcilerRequests: []sourceapi.ReconcilerRequest{{
				RequestID:    "reconciler_remote",
				Scope:        "story-corpus",
				Status:       "submitted",
				RuntimeRunID: reconcilerRunID,
			}},
		})
	}))
	defer sourceServer.Close()

	t.Setenv("SOURCE_SERVICE_BASE_URL", sourceServer.URL)
	t.Setenv("SOURCE_SERVICE_URL", "")
	t.Setenv("SOURCECYCLED_API_URL", "")
	t.Setenv("SOURCE_SERVICE_RUNTIME_BASE_URL", runtimeServer.URL)
	t.Setenv("SOURCE_SERVICE_RUNTIME_OWNER_ID", "global-wire-platform")
	t.Setenv("SOURCECYCLED_RUNTIME_BASE_URL", "")
	t.Setenv("SOURCECYCLED_RUNTIME_OWNER_ID", "")

	w := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/global-wire/sourcemaxx-status", "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("sourcemaxx status = %d body=%s", w.Code, w.Body.String())
	}
	var resp globalWireSourceMaxxStatusResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode sourcemaxx status: %v", err)
	}
	if resp.ProcessorRuntimeRunCount != 1 || resp.ProcessorResolvedRunCount != 1 || resp.ProcessorUnresolvedRunCount != 0 {
		t.Fatalf("processor runtime resolution = %+v", resp)
	}
	if resp.ReconcilerRuntimeRunCount != 1 || resp.ReconcilerResolvedRunCount != 1 || resp.ReconcilerUnresolvedRunCount != 0 {
		t.Fatalf("reconciler runtime resolution = %+v", resp)
	}
	if resp.ProcessorRunStateCounts[string(types.RunCompleted)] != 1 || resp.ReconcilerRunStateCounts[string(types.RunRunning)] != 1 {
		t.Fatalf("run state counts: processor=%+v reconciler=%+v", resp.ProcessorRunStateCounts, resp.ReconcilerRunStateCounts)
	}
	if resp.ProcessorUpdateCount != 1 || resp.ReconcilerUpdateCount != 1 {
		t.Fatalf("update counts: processor=%d reconciler=%d", resp.ProcessorUpdateCount, resp.ReconcilerUpdateCount)
	}
	if resp.ProcessorChildProfileCounts[AgentProfileResearcher] != 1 || resp.ProcessorChildProfileCounts[AgentProfileVText] != 1 {
		t.Fatalf("processor child counts = %+v", resp.ProcessorChildProfileCounts)
	}
	if resp.ReconcilerChildProfileCounts[AgentProfileResearcher] != 1 {
		t.Fatalf("reconciler child counts = %+v", resp.ReconcilerChildProfileCounts)
	}
}

func TestHandleGlobalWireSourceMaxxStatusReportsUnconfiguredSourceService(t *testing.T) {
	t.Setenv("SOURCE_SERVICE_BASE_URL", "")
	t.Setenv("SOURCE_SERVICE_URL", "")
	t.Setenv("SOURCECYCLED_API_URL", "")

	_, handler := testAPISetup(t)
	w := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/global-wire/sourcemaxx-status", "", "")
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("unconfigured sourcemaxx status = %d body=%s", w.Code, w.Body.String())
	}
	var resp globalWireSourceMaxxStatusResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode unconfigured sourcemaxx status: %v", err)
	}
	if resp.Status != "unavailable" || resp.Source != "source-service" || !resp.SourceServiceInternalOnly {
		t.Fatalf("unexpected unconfigured response: %+v", resp)
	}
}

func TestHandleGlobalWireSourceRefreshCreatesCandidateWithoutMutatingStoryGraph(t *testing.T) {
	sourceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/source-service/search" {
			t.Fatalf("unexpected source service path %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("q"); got != "port congestion refresh" {
			t.Fatalf("query = %q, want port congestion refresh", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(sourceapi.SearchResponse{
			Query:    "port congestion refresh",
			Provider: sourceapi.ProviderName,
			Metadata: sourceapi.Metadata{TargetKind: sourceapi.TargetKind},
			Results: []sourceapi.ItemResult{{
				Rank:          1,
				TargetKind:    sourceapi.TargetKind,
				ItemID:        "srcitem_port_refresh",
				SourceID:      "rss:ports",
				SourceType:    "rss",
				FetchID:       "fetch-port-refresh",
				Title:         "Rail slots improve at port complex",
				Body:          "A new operations bulletin says additional rail slots reduced terminal dwell.",
				URL:           "https://example.test/ports-refresh",
				CanonicalURL:  "https://example.test/ports-refresh",
				ContentHash:   "hash-port-refresh",
				EvidenceLevel: "source-service-ledger",
			}},
		})
	}))
	defer sourceServer.Close()
	t.Setenv("SOURCE_SERVICE_BASE_URL", sourceServer.URL)
	t.Setenv("SOURCE_SERVICE_URL", "")
	t.Setenv("SOURCECYCLED_API_URL", "")

	_, handler := testAPISetup(t)
	storiesBeforeW := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/global-wire/stories", "", "user-alpha")
	if storiesBeforeW.Code != http.StatusOK {
		t.Fatalf("stories before status = %d body=%s", storiesBeforeW.Code, storiesBeforeW.Body.String())
	}
	var storiesBefore globalWireStoriesResponse
	if err := json.NewDecoder(storiesBeforeW.Body).Decode(&storiesBefore); err != nil {
		t.Fatalf("decode stories before: %v", err)
	}
	beforeManifest := storiesBefore.Stories[0].Manifest

	body := `{"story_id":"story-supply-resilience","query":"port congestion refresh","max_results":2}`
	w := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/global-wire/source-refresh", body, "user-alpha")
	if w.Code != http.StatusCreated {
		t.Fatalf("source refresh status = %d body=%s", w.Code, w.Body.String())
	}
	var resp globalWireSourceRefreshResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode source refresh: %v", err)
	}
	if resp.Status != "candidate-review" || resp.ContentItem == nil || resp.Contribution == nil || resp.Decision == nil || resp.Candidate == nil {
		t.Fatalf("unexpected source refresh response: %+v", resp)
	}
	if resp.ClaimRecord == nil || resp.SourceReviewSignal == nil || resp.ResearchTask == nil || resp.ExtractionArtifact == nil {
		t.Fatalf("source refresh did not create structured claim/research state: %+v", resp)
	}
	if resp.RefreshRun.SourceContentID != resp.ContentItem.ContentID ||
		resp.RefreshRun.ContributionID != resp.Contribution.ID ||
		resp.RefreshRun.DecisionID != resp.Decision.ID ||
		resp.RefreshRun.CandidateID != resp.Candidate.ID {
		t.Fatalf("refresh run lineage mismatch: %+v", resp.RefreshRun)
	}
	if resp.RefreshRun.UpdateClassification != "claim-changed" ||
		resp.RefreshRun.StoryGraphAction != "claim-review" ||
		resp.RefreshRun.ProjectionAction != "projection-review-required" {
		t.Fatalf("refresh run classification missing: %+v", resp.RefreshRun)
	}
	if resp.Contribution.ResearchState != "accepted-for-graph-review" ||
		resp.Decision.Decision != "accepted" ||
		resp.Candidate.Status != "candidate-review" ||
		resp.Candidate.SourceContentID != resp.ContentItem.ContentID {
		t.Fatalf("refresh artifacts not candidate-ready: contribution=%+v decision=%+v candidate=%+v", resp.Contribution, resp.Decision, resp.Candidate)
	}
	if resp.Candidate.CandidateKind != "claim-changed" ||
		resp.Candidate.EdgeKind != "update-relation" ||
		resp.Candidate.ProjectionAction != "projection-review-required" {
		t.Fatalf("refresh candidate did not inherit classification: %+v", resp.Candidate)
	}
	if resp.ClaimRecord.RefreshID != resp.RefreshRun.ID ||
		resp.ClaimRecord.SourceContentID != resp.ContentItem.ContentID ||
		resp.ClaimRecord.ContributionID != resp.Contribution.ID ||
		resp.ClaimRecord.DecisionID != resp.Decision.ID ||
		resp.ClaimRecord.CandidateID != resp.Candidate.ID ||
		resp.ClaimRecord.ClaimKind != "claim-change" ||
		resp.ClaimRecord.UncertaintyState != "material-change-unverified" ||
		resp.ClaimRecord.DisputeState != "needs-comparison" ||
		resp.ClaimRecord.Status != "research-review-required" {
		t.Fatalf("claim record missing non-oracle refresh lineage: %+v", resp.ClaimRecord)
	}
	if resp.ResearchTask.ClaimID != resp.ClaimRecord.ID ||
		resp.ResearchTask.RefreshID != resp.RefreshRun.ID ||
		resp.ResearchTask.CandidateID != resp.Candidate.ID ||
		resp.ResearchTask.TaskKind != "claim-change-review" ||
		resp.ResearchTask.Status != "open" ||
		resp.ResearchTask.Priority != "high" ||
		!strings.Contains(resp.ResearchTask.Prompt, "Do not treat the source as an oracle") {
		t.Fatalf("research task missing review contract: %+v", resp.ResearchTask)
	}
	if resp.SourceReviewSignal.ClaimID != resp.ClaimRecord.ID ||
		resp.SourceReviewSignal.RefreshID != resp.RefreshRun.ID ||
		resp.SourceReviewSignal.SourceContentID != resp.ContentItem.ContentID ||
		resp.SourceReviewSignal.CandidateID != resp.Candidate.ID ||
		resp.SourceReviewSignal.SignalKind != "claim-change" ||
		resp.SourceReviewSignal.UpdateClassification != "claim-changed" ||
		resp.SourceReviewSignal.OverlapState != "claim-overlap-review-required" ||
		resp.SourceReviewSignal.ContradictionState != "no-contradiction-claimed" ||
		resp.SourceReviewSignal.ProjectionAction != "projection-review-required" ||
		!slices.Contains(resp.SourceReviewSignal.EvidenceRefs, "claim:"+resp.ClaimRecord.ID) ||
		!strings.Contains(resp.SourceReviewSignal.Rationale, "non-oracle review input") {
		t.Fatalf("source review signal missing normalization contract: %+v", resp.SourceReviewSignal)
	}
	if resp.ExtractionArtifact.ClaimID != resp.ClaimRecord.ID ||
		resp.ExtractionArtifact.RefreshID != resp.RefreshRun.ID ||
		resp.ExtractionArtifact.SourceContentID != resp.ContentItem.ContentID ||
		resp.ExtractionArtifact.CandidateID != resp.Candidate.ID ||
		resp.ExtractionArtifact.Status != "provisional-review" ||
		len(resp.ExtractionArtifact.Entities) == 0 ||
		len(resp.ExtractionArtifact.Events) == 0 ||
		len(resp.ExtractionArtifact.Timeline) == 0 ||
		!strings.Contains(resp.ExtractionArtifact.Rationale, "does not create or replace StoryGraph nodes") {
		t.Fatalf("extraction artifact missing source-neighborhood overlay contract: %+v", resp.ExtractionArtifact)
	}

	storiesAfterW := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/global-wire/stories", "", "user-alpha")
	if storiesAfterW.Code != http.StatusOK {
		t.Fatalf("stories after status = %d body=%s", storiesAfterW.Code, storiesAfterW.Body.String())
	}
	var storiesAfter globalWireStoriesResponse
	if err := json.NewDecoder(storiesAfterW.Body).Decode(&storiesAfter); err != nil {
		t.Fatalf("decode stories after: %v", err)
	}
	afterManifest := storiesAfter.Stories[0].Manifest
	if len(afterManifest.Lead) != len(beforeManifest.Lead) || len(afterManifest.Supporting) != len(beforeManifest.Supporting) ||
		len(afterManifest.Contrary) != len(beforeManifest.Contrary) || len(afterManifest.Context) != len(beforeManifest.Context) {
		t.Fatalf("StoryGraph manifest mutated during source refresh: before=%+v after=%+v", beforeManifest, afterManifest)
	}

	listW := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/global-wire/reconciliation?story_id=story-supply-resilience", "", "user-alpha")
	if listW.Code != http.StatusOK {
		t.Fatalf("list reconciliation status = %d body=%s", listW.Code, listW.Body.String())
	}
	var listResp globalWireReconciliationResponse
	if err := json.NewDecoder(listW.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode reconciliation list: %v", err)
	}
	if len(listResp.Refreshes) != 1 || listResp.Refreshes[0].CandidateID != resp.Candidate.ID {
		t.Fatalf("refresh run missing from reconciliation list: %+v", listResp.Refreshes)
	}
	if len(listResp.ClaimRecords) != 1 || listResp.ClaimRecords[0].ID != resp.ClaimRecord.ID {
		t.Fatalf("claim record missing from reconciliation list: %+v", listResp.ClaimRecords)
	}
	if len(listResp.SourceReviewSignals) != 1 ||
		listResp.SourceReviewSignals[0].ID != resp.SourceReviewSignal.ID ||
		listResp.SourceReviewSignals[0].OverlapState != "claim-overlap-review-required" {
		t.Fatalf("source review signal missing from reconciliation list: %+v", listResp.SourceReviewSignals)
	}
	if len(listResp.ResearchTasks) != 1 || listResp.ResearchTasks[0].ClaimID != resp.ClaimRecord.ID {
		t.Fatalf("research task missing from reconciliation list: %+v", listResp.ResearchTasks)
	}
	if len(listResp.ExtractionArtifacts) != 1 ||
		listResp.ExtractionArtifacts[0].ClaimID != resp.ClaimRecord.ID ||
		listResp.ExtractionArtifacts[0].SourceContentID != resp.ContentItem.ContentID {
		t.Fatalf("extraction artifact missing from reconciliation list: %+v", listResp.ExtractionArtifacts)
	}
	if len(listResp.ResearchEvidence) != 0 {
		t.Fatalf("research evidence should not exist before task lifecycle transition: %+v", listResp.ResearchEvidence)
	}
	if listResp.Refreshes[0].UpdateClassification != "claim-changed" {
		t.Fatalf("refresh classification missing from reconciliation list: %+v", listResp.Refreshes[0])
	}

	taskBody := fmt.Sprintf(`{"task_id":%q,"action":"complete","evidence_summary":"Research completed against source-service item; reconciliation can consider the evidence, but the platform StoryGraph remains unchanged.","evidence_level":"reconciliation-level"}`, resp.ResearchTask.ID)
	taskW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/global-wire/research-tasks", taskBody, "user-alpha")
	if taskW.Code != http.StatusCreated {
		t.Fatalf("complete research task status = %d body=%s", taskW.Code, taskW.Body.String())
	}
	var taskResp globalWireResearchTaskLifecycleResponse
	if err := json.NewDecoder(taskW.Body).Decode(&taskResp); err != nil {
		t.Fatalf("decode research task lifecycle response: %v", err)
	}
	if taskResp.Task.ID != resp.ResearchTask.ID ||
		taskResp.Task.Status != "completed" ||
		taskResp.Evidence.TaskID != resp.ResearchTask.ID ||
		taskResp.Evidence.ClaimID != resp.ClaimRecord.ID ||
		taskResp.Evidence.SourceContentID != resp.ContentItem.ContentID ||
		taskResp.Evidence.Status != "completed" ||
		taskResp.Evidence.EvidenceLevel != "reconciliation-level" ||
		!strings.Contains(taskResp.Evidence.Summary, "platform StoryGraph remains unchanged") {
		t.Fatalf("research task lifecycle response missing reconciliation evidence: %+v", taskResp)
	}

	taskListW := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/global-wire/reconciliation?story_id=story-supply-resilience", "", "user-alpha")
	if taskListW.Code != http.StatusOK {
		t.Fatalf("list reconciliation after research status = %d body=%s", taskListW.Code, taskListW.Body.String())
	}
	var taskListResp globalWireReconciliationResponse
	if err := json.NewDecoder(taskListW.Body).Decode(&taskListResp); err != nil {
		t.Fatalf("decode reconciliation list after research: %v", err)
	}
	if len(taskListResp.ResearchTasks) != 1 ||
		taskListResp.ResearchTasks[0].Status != "completed" ||
		len(taskListResp.ResearchEvidence) != 1 ||
		taskListResp.ResearchEvidence[0].TaskID != resp.ResearchTask.ID {
		t.Fatalf("completed research evidence missing from reconciliation list: %+v", taskListResp)
	}

	handoffBody := fmt.Sprintf(`{"evidence_id":%q,"decision":"accept","note":"Accept completed research evidence into platform review without mutating the story."}`, taskResp.Evidence.ID)
	handoffW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/global-wire/research-evidence", handoffBody, "user-alpha")
	if handoffW.Code != http.StatusCreated {
		t.Fatalf("research evidence handoff status = %d body=%s", handoffW.Code, handoffW.Body.String())
	}
	var handoffResp globalWireResearchEvidenceDecisionResponse
	if err := json.NewDecoder(handoffW.Body).Decode(&handoffResp); err != nil {
		t.Fatalf("decode research evidence handoff response: %v", err)
	}
	if handoffResp.Decision.EvidenceID != taskResp.Evidence.ID ||
		handoffResp.Decision.TaskID != resp.ResearchTask.ID ||
		handoffResp.Decision.CandidateID != resp.Candidate.ID ||
		handoffResp.Decision.Decision != "accepted-for-review" ||
		handoffResp.Decision.ResultState != "ready-for-platform-review" ||
		handoffResp.Candidate == nil ||
		handoffResp.Candidate.Status != "research-evidence-accepted" {
		t.Fatalf("research evidence handoff missing candidate review state: %+v", handoffResp)
	}

	handoffListW := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/global-wire/reconciliation?story_id=story-supply-resilience", "", "user-alpha")
	if handoffListW.Code != http.StatusOK {
		t.Fatalf("list reconciliation after handoff status = %d body=%s", handoffListW.Code, handoffListW.Body.String())
	}
	var handoffListResp globalWireReconciliationResponse
	if err := json.NewDecoder(handoffListW.Body).Decode(&handoffListResp); err != nil {
		t.Fatalf("decode reconciliation list after handoff: %v", err)
	}
	if len(handoffListResp.ResearchDecisions) != 1 ||
		handoffListResp.ResearchDecisions[0].EvidenceID != taskResp.Evidence.ID ||
		handoffListResp.ResearchDecisions[0].ResultState != "ready-for-platform-review" ||
		len(handoffListResp.Candidates) != 1 ||
		handoffListResp.Candidates[0].Status != "research-evidence-accepted" {
		t.Fatalf("research evidence handoff missing from reconciliation list: %+v", handoffListResp)
	}

	publicationBody := fmt.Sprintf(`{"research_decision_id":%q}`, handoffResp.Decision.ID)
	publicationW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/global-wire/publication-updates", publicationBody, "user-alpha")
	if publicationW.Code != http.StatusCreated {
		t.Fatalf("publication update package status = %d body=%s", publicationW.Code, publicationW.Body.String())
	}
	var publicationResp globalWirePublicationUpdateResponse
	if err := json.NewDecoder(publicationW.Body).Decode(&publicationResp); err != nil {
		t.Fatalf("decode publication update response: %v", err)
	}
	if publicationResp.Update.ResearchDecisionID != handoffResp.Decision.ID ||
		publicationResp.Update.EvidenceID != taskResp.Evidence.ID ||
		publicationResp.Update.CandidateID != resp.Candidate.ID ||
		publicationResp.Update.SourceContentID != resp.ContentItem.ContentID ||
		publicationResp.Update.Status != "packaged-for-publication-review" ||
		!strings.Contains(publicationResp.Update.Summary, "does not publish or mutate") ||
		len(publicationResp.Update.ExtractionIDs) != 1 ||
		publicationResp.Update.ExtractionIDs[0] != resp.ExtractionArtifact.ID ||
		len(publicationResp.Update.RollbackRefs) < 4 ||
		publicationResp.Candidate == nil ||
		publicationResp.SourceItem == nil {
		t.Fatalf("publication update missing review package lineage: %+v", publicationResp)
	}

	artifactBody := fmt.Sprintf(`{"update_id":%q,"channel":"newsletter"}`, publicationResp.Update.ID)
	artifactW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/global-wire/publication-artifacts", artifactBody, "user-alpha")
	if artifactW.Code != http.StatusCreated {
		t.Fatalf("publication artifact status = %d body=%s", artifactW.Code, artifactW.Body.String())
	}
	var artifactResp globalWirePublicationArtifactResponse
	if err := json.NewDecoder(artifactW.Body).Decode(&artifactResp); err != nil {
		t.Fatalf("decode publication artifact response: %v", err)
	}
	if artifactResp.Artifact.UpdateID != publicationResp.Update.ID ||
		artifactResp.Artifact.StoryID != publicationResp.Update.StoryID ||
		artifactResp.Artifact.StoryVTextDocID == "" ||
		artifactResp.Artifact.SourceContentID != resp.ContentItem.ContentID ||
		artifactResp.Artifact.Channel != "newsletter" ||
		artifactResp.Artifact.Status != "publication-review-ready" ||
		len(artifactResp.Artifact.ExtractionIDs) != len(publicationResp.Update.ExtractionIDs) ||
		len(artifactResp.Artifact.CitationRefs) < 5 ||
		!strings.Contains(artifactResp.Artifact.Body, "not public publication") ||
		!strings.Contains(artifactResp.Artifact.Body, "does not mutate the platform story") ||
		artifactResp.SourceItem == nil {
		t.Fatalf("publication artifact missing citeable lineage: %+v", artifactResp)
	}

	feedW := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/global-wire/publication-feed?story_id=story-supply-resilience&channel=newsletter", "", "user-alpha")
	if feedW.Code != http.StatusOK {
		t.Fatalf("publication feed status = %d body=%s", feedW.Code, feedW.Body.String())
	}
	var feedResp globalWirePublicationFeedResponse
	if err := json.NewDecoder(feedW.Body).Decode(&feedResp); err != nil {
		t.Fatalf("decode publication feed response: %v", err)
	}
	if feedResp.Status != "ready" ||
		feedResp.Channel != "newsletter" ||
		len(feedResp.FeedItems) != 1 ||
		feedResp.FeedItems[0].Artifact.ID != artifactResp.Artifact.ID ||
		feedResp.FeedItems[0].Story.ID != "story-supply-resilience" ||
		feedResp.FeedItems[0].SourceItem == nil ||
		feedResp.FeedItems[0].CitationCount < 5 ||
		feedResp.FeedItems[0].RollbackCount < 5 ||
		feedResp.FeedItems[0].Status != "publication-review-ready" {
		t.Fatalf("publication artifact missing from feed: %+v", feedResp)
	}

	reviewBody := fmt.Sprintf(`{"artifact_id":%q,"decision":"approve","note":"owner approved for publication review proof"}`, artifactResp.Artifact.ID)
	reviewW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/global-wire/publication-artifact-reviews", reviewBody, "user-alpha")
	if reviewW.Code != http.StatusCreated {
		t.Fatalf("publication artifact review status = %d body=%s", reviewW.Code, reviewW.Body.String())
	}
	var reviewResp globalWirePublicationArtifactReviewResponse
	if err := json.NewDecoder(reviewW.Body).Decode(&reviewResp); err != nil {
		t.Fatalf("decode publication artifact review response: %v", err)
	}
	if reviewResp.Artifact.ID != artifactResp.Artifact.ID ||
		reviewResp.Artifact.Status != "publication-approved" ||
		reviewResp.Status != "publication-approved" {
		t.Fatalf("publication artifact review did not approve artifact: %+v", reviewResp)
	}
	approvedFeedW := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/global-wire/publication-feed?story_id=story-supply-resilience&channel=newsletter", "", "user-alpha")
	if approvedFeedW.Code != http.StatusOK {
		t.Fatalf("approved publication feed status = %d body=%s", approvedFeedW.Code, approvedFeedW.Body.String())
	}
	var approvedFeedResp globalWirePublicationFeedResponse
	if err := json.NewDecoder(approvedFeedW.Body).Decode(&approvedFeedResp); err != nil {
		t.Fatalf("decode approved publication feed response: %v", err)
	}
	if len(approvedFeedResp.FeedItems) != 1 || approvedFeedResp.FeedItems[0].Status != "publication-approved" {
		t.Fatalf("approved publication artifact missing from feed: %+v", approvedFeedResp)
	}

	deliveryBody := fmt.Sprintf(`{"artifact_id":%q,"channel":"newsletter"}`, artifactResp.Artifact.ID)
	deliveryW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/global-wire/publication-deliveries", deliveryBody, "user-alpha")
	if deliveryW.Code != http.StatusCreated {
		t.Fatalf("publication delivery status = %d body=%s", deliveryW.Code, deliveryW.Body.String())
	}
	var deliveryResp globalWirePublicationDeliveryResponse
	if err := json.NewDecoder(deliveryW.Body).Decode(&deliveryResp); err != nil {
		t.Fatalf("decode publication delivery response: %v", err)
	}
	if deliveryResp.Delivery.ArtifactID != artifactResp.Artifact.ID ||
		deliveryResp.Delivery.StoryID != "story-supply-resilience" ||
		deliveryResp.Delivery.Channel != "newsletter" ||
		deliveryResp.Delivery.Status != "delivery-ready" ||
		deliveryResp.Delivery.DeliveryRef == "" ||
		deliveryResp.Delivery.CitationCount < 5 ||
		deliveryResp.Delivery.RollbackCount < 5 ||
		!slices.Contains(deliveryResp.Delivery.RollbackRefs, "publication_artifact:"+artifactResp.Artifact.ID) ||
		deliveryResp.Artifact.Status != "publication-approved" ||
		deliveryResp.Story.ID != "story-supply-resilience" {
		t.Fatalf("publication delivery missing approved artifact lineage: %+v", deliveryResp)
	}
	deliveryListW := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/global-wire/publication-deliveries?story_id=story-supply-resilience", "", "user-alpha")
	if deliveryListW.Code != http.StatusOK {
		t.Fatalf("publication delivery list status = %d body=%s", deliveryListW.Code, deliveryListW.Body.String())
	}
	var deliveryListResp struct {
		PublicationDeliveries []types.GlobalWirePublicationDelivery `json:"publication_deliveries"`
	}
	if err := json.NewDecoder(deliveryListW.Body).Decode(&deliveryListResp); err != nil {
		t.Fatalf("decode publication delivery list response: %v", err)
	}
	if len(deliveryListResp.PublicationDeliveries) != 1 ||
		deliveryListResp.PublicationDeliveries[0].ArtifactID != artifactResp.Artifact.ID {
		t.Fatalf("publication delivery missing from list: %+v", deliveryListResp)
	}
	deliveryDetailW := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/global-wire/publication-deliveries/"+deliveryResp.Delivery.ID, "", "user-alpha")
	if deliveryDetailW.Code != http.StatusOK {
		t.Fatalf("publication delivery detail status = %d body=%s", deliveryDetailW.Code, deliveryDetailW.Body.String())
	}
	var deliveryDetailResp globalWirePublicationDeliveryDetailResponse
	if err := json.NewDecoder(deliveryDetailW.Body).Decode(&deliveryDetailResp); err != nil {
		t.Fatalf("decode publication delivery detail response: %v", err)
	}
	if deliveryDetailResp.Delivery.ID != deliveryResp.Delivery.ID ||
		deliveryDetailResp.Artifact.ID != artifactResp.Artifact.ID ||
		deliveryDetailResp.Artifact.Body == "" ||
		deliveryDetailResp.Story.ID != "story-supply-resilience" ||
		deliveryDetailResp.SourceItem == nil ||
		len(deliveryDetailResp.Delivery.CitationRefs) < 5 ||
		len(deliveryDetailResp.Delivery.RollbackRefs) < 5 {
		t.Fatalf("publication delivery detail missing provenance: %+v", deliveryDetailResp)
	}

	scriptBody := fmt.Sprintf(`{"artifact_id":%q}`, artifactResp.Artifact.ID)
	scriptW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/global-wire/autoradio-scripts", scriptBody, "user-alpha")
	if scriptW.Code != http.StatusCreated {
		t.Fatalf("autoradio script status = %d body=%s", scriptW.Code, scriptW.Body.String())
	}
	var scriptResp globalWireAutoradioScriptResponse
	if err := json.NewDecoder(scriptW.Body).Decode(&scriptResp); err != nil {
		t.Fatalf("decode autoradio script response: %v", err)
	}
	if scriptResp.Script.ArtifactID != artifactResp.Artifact.ID ||
		scriptResp.Script.StoryID != "story-supply-resilience" ||
		scriptResp.Script.Status != "script-ready" ||
		!strings.Contains(scriptResp.Script.ScriptBody, artifactResp.Artifact.Body) ||
		scriptResp.Script.CitationCount < 5 ||
		scriptResp.Script.RollbackCount < 5 ||
		!slices.Contains(scriptResp.Script.RollbackRefs, "publication_artifact:"+artifactResp.Artifact.ID) ||
		scriptResp.Artifact.Status != "publication-approved" ||
		scriptResp.Story.ID != "story-supply-resilience" ||
		scriptResp.SourceItem == nil {
		t.Fatalf("autoradio script missing artifact provenance: %+v", scriptResp)
	}
	scriptListW := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/global-wire/autoradio-scripts?story_id=story-supply-resilience", "", "user-alpha")
	if scriptListW.Code != http.StatusOK {
		t.Fatalf("autoradio script list status = %d body=%s", scriptListW.Code, scriptListW.Body.String())
	}
	var scriptListResp struct {
		AutoradioScripts []types.GlobalWireAutoradioScript `json:"autoradio_scripts"`
	}
	if err := json.NewDecoder(scriptListW.Body).Decode(&scriptListResp); err != nil {
		t.Fatalf("decode autoradio script list response: %v", err)
	}
	if len(scriptListResp.AutoradioScripts) != 1 ||
		scriptListResp.AutoradioScripts[0].ArtifactID != artifactResp.Artifact.ID {
		t.Fatalf("autoradio script missing from list: %+v", scriptListResp)
	}

	episodeBody := fmt.Sprintf(`{"script_id":%q}`, scriptResp.Script.ID)
	episodeW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/global-wire/autoradio-episodes", episodeBody, "user-alpha")
	if episodeW.Code != http.StatusCreated {
		t.Fatalf("autoradio episode status = %d body=%s", episodeW.Code, episodeW.Body.String())
	}
	var episodeResp globalWireAutoradioEpisodeResponse
	if err := json.NewDecoder(episodeW.Body).Decode(&episodeResp); err != nil {
		t.Fatalf("decode autoradio episode response: %v", err)
	}
	if episodeResp.Episode.ScriptID != scriptResp.Script.ID ||
		episodeResp.Episode.ArtifactID != artifactResp.Artifact.ID ||
		episodeResp.Episode.StoryID != "story-supply-resilience" ||
		episodeResp.Episode.Status != "episode-ready" ||
		episodeResp.Episode.PlaybackMode != "browser-speech" ||
		!strings.Contains(episodeResp.Episode.Transcript, scriptResp.Script.ScriptBody) ||
		episodeResp.Episode.DurationSeconds <= 0 ||
		!slices.Contains(episodeResp.Episode.RollbackRefs, "autoradio_script:"+scriptResp.Script.ID) ||
		episodeResp.Script.ID != scriptResp.Script.ID ||
		episodeResp.Artifact.Status != "publication-approved" ||
		episodeResp.Story.ID != "story-supply-resilience" ||
		episodeResp.SourceItem == nil {
		t.Fatalf("autoradio episode missing playback provenance: %+v", episodeResp)
	}
	episodeListW := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/global-wire/autoradio-episodes?story_id=story-supply-resilience", "", "user-alpha")
	if episodeListW.Code != http.StatusOK {
		t.Fatalf("autoradio episode list status = %d body=%s", episodeListW.Code, episodeListW.Body.String())
	}
	var episodeListResp struct {
		AutoradioEpisodes []types.GlobalWireAutoradioEpisode `json:"autoradio_episodes"`
	}
	if err := json.NewDecoder(episodeListW.Body).Decode(&episodeListResp); err != nil {
		t.Fatalf("decode autoradio episode list response: %v", err)
	}
	if len(episodeListResp.AutoradioEpisodes) != 1 ||
		episodeListResp.AutoradioEpisodes[0].ScriptID != scriptResp.Script.ID {
		t.Fatalf("autoradio episode missing from list: %+v", episodeListResp)
	}

	exportBody := fmt.Sprintf(`{"delivery_id":%q,"format":"md"}`, deliveryResp.Delivery.ID)
	exportW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/global-wire/publication-delivery-exports", exportBody, "user-alpha")
	if exportW.Code != http.StatusCreated {
		t.Fatalf("delivery export status = %d body=%s", exportW.Code, exportW.Body.String())
	}
	var exportResp globalWirePublicationDeliveryExportResponse
	if err := json.NewDecoder(exportW.Body).Decode(&exportResp); err != nil {
		t.Fatalf("decode delivery export response: %v", err)
	}
	if exportResp.Export.DeliveryID != deliveryResp.Delivery.ID ||
		exportResp.Export.ArtifactID != artifactResp.Artifact.ID ||
		exportResp.Export.ScriptID != scriptResp.Script.ID ||
		exportResp.Export.Status != "export-ready" ||
		exportResp.Export.Format != "md" ||
		!strings.Contains(exportResp.Export.ExportBody, artifactResp.Artifact.Body) ||
		!strings.Contains(exportResp.Export.ExportBody, scriptResp.Script.ScriptBody) ||
		exportResp.Export.CitationCount < 5 ||
		exportResp.Export.RollbackCount < 5 ||
		!slices.Contains(exportResp.Export.RollbackRefs, "publication_delivery:"+deliveryResp.Delivery.ID) ||
		!slices.Contains(exportResp.Export.RollbackRefs, "autoradio_script:"+scriptResp.Script.ID) ||
		exportResp.Script == nil ||
		exportResp.SourceItem == nil {
		t.Fatalf("delivery export missing publication/script provenance: %+v", exportResp)
	}
	exportListW := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/global-wire/publication-delivery-exports?story_id=story-supply-resilience", "", "user-alpha")
	if exportListW.Code != http.StatusOK {
		t.Fatalf("delivery export list status = %d body=%s", exportListW.Code, exportListW.Body.String())
	}
	var exportListResp struct {
		DeliveryExports []types.GlobalWirePublicationDeliveryExport `json:"delivery_exports"`
	}
	if err := json.NewDecoder(exportListW.Body).Decode(&exportListResp); err != nil {
		t.Fatalf("decode delivery export list response: %v", err)
	}
	if len(exportListResp.DeliveryExports) != 1 ||
		exportListResp.DeliveryExports[0].DeliveryID != deliveryResp.Delivery.ID {
		t.Fatalf("delivery export missing from list: %+v", exportListResp)
	}

	linkBody := fmt.Sprintf(`{"export_id":%q}`, exportResp.Export.ID)
	linkW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/global-wire/publication-public-links", linkBody, "user-alpha")
	if linkW.Code != http.StatusCreated {
		t.Fatalf("public link status = %d body=%s", linkW.Code, linkW.Body.String())
	}
	var linkResp globalWirePublicationPublicLinkResponse
	if err := json.NewDecoder(linkW.Body).Decode(&linkResp); err != nil {
		t.Fatalf("decode public link response: %v", err)
	}
	if linkResp.PublicLink.ExportID != exportResp.Export.ID ||
		linkResp.PublicLink.Status != "public-unlisted" ||
		linkResp.PublicLink.RoutePath == "" ||
		linkResp.PublicLink.FeedPath == "" ||
		linkResp.PublicLink.Token == "" ||
		!strings.Contains(linkResp.PublicLink.ExportBody, artifactResp.Artifact.Body) ||
		!slices.Contains(linkResp.PublicLink.RollbackRefs, "delivery_export:"+exportResp.Export.ID) {
		t.Fatalf("public link missing export provenance: %+v", linkResp)
	}
	publicLinkW := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/global-wire/publication-public-links/"+linkResp.PublicLink.Token, "", "")
	if publicLinkW.Code != http.StatusOK {
		t.Fatalf("public link detail status = %d body=%s", publicLinkW.Code, publicLinkW.Body.String())
	}
	var publicLinkResp globalWirePublicationPublicLinkResponse
	if err := json.NewDecoder(publicLinkW.Body).Decode(&publicLinkResp); err != nil {
		t.Fatalf("decode public link detail response: %v", err)
	}
	if publicLinkResp.PublicLink.ID != linkResp.PublicLink.ID ||
		publicLinkResp.PublicLink.OwnerID != "" ||
		publicLinkResp.PublicLink.ExportID != exportResp.Export.ID ||
		!strings.Contains(publicLinkResp.PublicLink.ExportBody, artifactResp.Artifact.Body) {
		t.Fatalf("public link detail leaked or lost fields: %+v", publicLinkResp)
	}
	rssW := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/global-wire/publication-public-links/"+linkResp.PublicLink.Token+"/rss", "", "")
	if rssW.Code != http.StatusOK {
		t.Fatalf("public link rss status = %d body=%s", rssW.Code, rssW.Body.String())
	}
	if contentType := rssW.Header().Get("Content-Type"); !strings.Contains(contentType, "application/rss+xml") {
		t.Fatalf("public link rss content-type = %q", contentType)
	}
	rssBody := rssW.Body.String()
	if !strings.Contains(rssBody, "<rss") ||
		!strings.Contains(rssBody, linkResp.PublicLink.Title) ||
		!strings.Contains(rssBody, "Global Wire publication artifact for") ||
		!strings.Contains(rssBody, artifactResp.Artifact.ID) ||
		!strings.Contains(rssBody, "Citation refs:") ||
		!strings.Contains(rssBody, "Rollback refs:") {
		t.Fatalf("public link rss missing publication/provenance: %s", rssBody)
	}
	subscriberBody := `{"email":"global-wire-subscriber@example.com","label":"Staging subscriber"}`
	subscriberW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/global-wire/newsletter-subscribers", subscriberBody, "user-alpha")
	if subscriberW.Code != http.StatusCreated {
		t.Fatalf("newsletter subscriber status = %d body=%s", subscriberW.Code, subscriberW.Body.String())
	}
	var subscriberResp globalWireNewsletterSubscriberResponse
	if err := json.NewDecoder(subscriberW.Body).Decode(&subscriberResp); err != nil {
		t.Fatalf("decode newsletter subscriber response: %v", err)
	}
	if subscriberResp.Subscriber.Status != "active" || subscriberResp.Subscriber.Email != "global-wire-subscriber@example.com" {
		t.Fatalf("newsletter subscriber missing active email: %+v", subscriberResp)
	}
	issueBody := fmt.Sprintf(`{"public_link_ids":[%q],"story_id":"story-supply-resilience"}`, linkResp.PublicLink.ID)
	issueW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/global-wire/newsletter-issues", issueBody, "user-alpha")
	if issueW.Code != http.StatusCreated {
		t.Fatalf("newsletter issue status = %d body=%s", issueW.Code, issueW.Body.String())
	}
	var issueResp globalWireNewsletterIssueResponse
	if err := json.NewDecoder(issueW.Body).Decode(&issueResp); err != nil {
		t.Fatalf("decode newsletter issue response: %v", err)
	}
	if issueResp.Issue.Status != "issue-ready" ||
		issueResp.Issue.SubscriberCount != 1 ||
		!slices.Contains(issueResp.Issue.PublicLinkIDs, linkResp.PublicLink.ID) ||
		!slices.Contains(issueResp.Issue.RollbackRefs, "public_link:"+linkResp.PublicLink.ID) ||
		len(issueResp.Deliveries) != 1 ||
		issueResp.Deliveries[0].Status != "delivery-ready" ||
		issueResp.Deliveries[0].SubscriberID != subscriberResp.Subscriber.ID ||
		len(issueResp.Receipts) != 1 ||
		issueResp.Receipts[0].IssueID != issueResp.Issue.ID ||
		issueResp.Receipts[0].DeliveryID != issueResp.Deliveries[0].ID ||
		issueResp.Receipts[0].Provider != "choir-dry-run-mailer" ||
		issueResp.Receipts[0].ProviderMode != "dry-run" ||
		issueResp.Receipts[0].Status != "provider-dry-run-recorded" ||
		!strings.HasPrefix(issueResp.Receipts[0].MessageID, "dryrun-") ||
		!slices.Contains(issueResp.Receipts[0].EventRefs, "newsletter_issue:"+issueResp.Issue.ID) ||
		!slices.Contains(issueResp.Receipts[0].RollbackRefs, "newsletter_delivery:"+issueResp.Deliveries[0].ID) {
		t.Fatalf("newsletter issue/delivery missing provenance: %+v", issueResp)
	}

	publicationListW := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/global-wire/reconciliation?story_id=story-supply-resilience", "", "user-alpha")
	if publicationListW.Code != http.StatusOK {
		t.Fatalf("list reconciliation after publication package status = %d body=%s", publicationListW.Code, publicationListW.Body.String())
	}
	var finalReconciliation globalWireReconciliationResponse
	if err := json.NewDecoder(publicationListW.Body).Decode(&finalReconciliation); err != nil {
		t.Fatalf("decode reconciliation after newsletter issue: %v", err)
	}
	if len(finalReconciliation.NewsletterIssues) == 0 || len(finalReconciliation.NewsletterDeliveries) == 0 || len(finalReconciliation.NewsletterReceipts) == 0 {
		t.Fatalf("reconciliation missing newsletter ledger: %+v", finalReconciliation)
	}
	publicationListResp := finalReconciliation
	if len(publicationListResp.PublicationUpdates) != 1 ||
		publicationListResp.PublicationUpdates[0].ResearchDecisionID != handoffResp.Decision.ID ||
		len(publicationListResp.PublicationUpdates[0].ExtractionIDs) != 1 ||
		len(publicationListResp.PublicationUpdates[0].RollbackRefs) < 4 ||
		len(publicationListResp.PublicationArtifacts) != 1 ||
		publicationListResp.PublicationArtifacts[0].UpdateID != publicationResp.Update.ID ||
		len(publicationListResp.PublicationArtifacts[0].CitationRefs) < 5 ||
		len(publicationListResp.PublicationDeliveries) != 1 ||
		publicationListResp.PublicationDeliveries[0].ArtifactID != artifactResp.Artifact.ID ||
		len(publicationListResp.AutoradioScripts) != 1 ||
		publicationListResp.AutoradioScripts[0].ArtifactID != artifactResp.Artifact.ID ||
		len(publicationListResp.AutoradioEpisodes) != 1 ||
		publicationListResp.AutoradioEpisodes[0].ScriptID != scriptResp.Script.ID ||
		len(publicationListResp.DeliveryExports) != 1 ||
		publicationListResp.DeliveryExports[0].DeliveryID != deliveryResp.Delivery.ID ||
		len(publicationListResp.PublicLinks) != 1 ||
		publicationListResp.PublicLinks[0].ExportID != exportResp.Export.ID {
		t.Fatalf("publication artifacts missing from reconciliation list: updates=%+v artifacts=%+v deliveries=%+v scripts=%+v episodes=%+v exports=%+v links=%+v", publicationListResp.PublicationUpdates, publicationListResp.PublicationArtifacts, publicationListResp.PublicationDeliveries, publicationListResp.AutoradioScripts, publicationListResp.AutoradioEpisodes, publicationListResp.DeliveryExports, publicationListResp.PublicLinks)
	}
	if len(finalReconciliation.SourceDossiers) == 0 ||
		!slices.Contains(finalReconciliation.SourceDossiers[0].PublicationRefs.AutoradioEpisodeIDs, episodeResp.Episode.ID) {
		t.Fatalf("source dossier missing autoradio episode ref: %+v", finalReconciliation.SourceDossiers)
	}
	if !slices.Contains(finalReconciliation.SourceDossiers[0].PublicationRefs.NewsletterReceiptIDs, issueResp.Receipts[0].ID) ||
		!slices.Contains(finalReconciliation.SourceDossiers[0].PublicationRefs.RollbackRefs, "newsletter_delivery:"+issueResp.Deliveries[0].ID) {
		t.Fatalf("source dossier missing newsletter provider receipt refs: %+v", finalReconciliation.SourceDossiers[0].PublicationRefs)
	}
	if len(finalReconciliation.SourceDossiers[0].SourceReviewSignals) != 1 ||
		finalReconciliation.SourceDossiers[0].SourceReviewSignals[0].ID != resp.SourceReviewSignal.ID ||
		!slices.Contains(finalReconciliation.SourceDossiers[0].ClaimDossiers[0].SourceReviewSignalIDs, resp.SourceReviewSignal.ID) {
		t.Fatalf("source dossier missing review signal: %+v", finalReconciliation.SourceDossiers[0])
	}

	storiesTaskAfterW := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/global-wire/stories", "", "user-alpha")
	if storiesTaskAfterW.Code != http.StatusOK {
		t.Fatalf("stories after task status = %d body=%s", storiesTaskAfterW.Code, storiesTaskAfterW.Body.String())
	}
	var storiesTaskAfter globalWireStoriesResponse
	if err := json.NewDecoder(storiesTaskAfterW.Body).Decode(&storiesTaskAfter); err != nil {
		t.Fatalf("decode stories after task: %v", err)
	}
	taskAfterManifest := storiesTaskAfter.Stories[0].Manifest
	if len(taskAfterManifest.Lead) != len(beforeManifest.Lead) || len(taskAfterManifest.Supporting) != len(beforeManifest.Supporting) ||
		len(taskAfterManifest.Contrary) != len(beforeManifest.Contrary) || len(taskAfterManifest.Context) != len(beforeManifest.Context) {
		t.Fatalf("StoryGraph manifest mutated during research task lifecycle: before=%+v after=%+v", beforeManifest, taskAfterManifest)
	}
}

func TestHandleGlobalWireFetchCycleCreatesRegistryAndRefreshEvidence(t *testing.T) {
	sourceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/source-service/search" {
			t.Fatalf("unexpected source service path %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("q"); !strings.Contains(got, "Port backlog recedes") {
			t.Fatalf("query = %q, want story headline query", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(sourceapi.SearchResponse{
			Query:    r.URL.Query().Get("q"),
			Provider: sourceapi.ProviderName,
			Metadata: sourceapi.Metadata{TargetKind: sourceapi.TargetKind},
			Results: []sourceapi.ItemResult{{
				Rank:          1,
				TargetKind:    sourceapi.TargetKind,
				ItemID:        "srcitem_fetch_cycle_port",
				SourceID:      "rss:ports",
				SourceType:    "rss",
				FetchID:       "fetch-cycle-port-1",
				Title:         "Port rail dwell reduced after added slots",
				Body:          "A source registry fetch cycle found that additional rail slots reduced dwell and updated the claim basis.",
				URL:           "https://example.test/fetch-cycle-port",
				CanonicalURL:  "https://example.test/fetch-cycle-port",
				ContentHash:   "hash-fetch-cycle-port",
				EvidenceLevel: "source-service-ledger",
			}},
		})
	}))
	defer sourceServer.Close()
	t.Setenv("SOURCE_SERVICE_BASE_URL", sourceServer.URL)
	t.Setenv("SOURCE_SERVICE_URL", "")
	t.Setenv("SOURCECYCLED_API_URL", "")

	_, handler := testAPISetup(t)
	body := `{"story_ids":["story-supply-resilience"],"max_stories":1,"max_results":1,"trigger":"test-scheduled-cycle","scheduler_mode":true,"cadence_seconds":1800}`
	w := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/global-wire/fetch-cycles", body, "user-cycle")
	if w.Code != http.StatusCreated {
		t.Fatalf("fetch cycle status = %d body=%s", w.Code, w.Body.String())
	}
	var resp globalWireFetchCycleResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode fetch cycle: %v", err)
	}
	if resp.FetchCycle.ID == "" ||
		resp.FetchCycle.Trigger != "test-scheduled-cycle" ||
		resp.FetchCycle.Status != "completed" ||
		resp.SchedulerRun == nil ||
		resp.SchedulerRun.FetchCycleID != resp.FetchCycle.ID ||
		resp.SchedulerRun.Status != "scheduled-cycle-recorded" ||
		len(resp.RegistryEntries) != 1 ||
		len(resp.RefreshRuns) != 1 ||
		len(resp.ContentItems) != 1 {
		t.Fatalf("fetch cycle missing registry/source evidence: %+v", resp)
	}
	if resp.RegistryEntries[0].LastCycleID != resp.FetchCycle.ID ||
		resp.RegistryEntries[0].LastScheduledRunID != resp.SchedulerRun.ID ||
		resp.RegistryEntries[0].CadenceSeconds != 1800 ||
		resp.RegistryEntries[0].SourceStandingPolicy == "" ||
		!strings.Contains(resp.RegistryEntries[0].SourceStandingRationale, "not automatic StoryGraph mutations") ||
		resp.FetchCycle.RegistryEntryIDs[0] != resp.RegistryEntries[0].ID ||
		resp.FetchCycle.RefreshRunIDs[0] != resp.RefreshRuns[0].ID ||
		resp.FetchCycle.SourceContentIDs[0] != resp.ContentItems[0].ContentID {
		t.Fatalf("fetch cycle lineage mismatch: cycle=%+v registry=%+v refresh=%+v item=%+v", resp.FetchCycle, resp.RegistryEntries[0], resp.RefreshRuns[0], resp.ContentItems[0])
	}
	if resp.RefreshRuns[0].UpdateClassification != "claim-changed" ||
		len(resp.Candidates) != 1 ||
		len(resp.ClaimRecords) != 1 ||
		len(resp.ResearchTasks) != 1 ||
		len(resp.ExtractionArtifacts) != 1 ||
		resp.ClaimRecords[0].RefreshID != resp.RefreshRuns[0].ID ||
		resp.ResearchTasks[0].ClaimID != resp.ClaimRecords[0].ID ||
		resp.ExtractionArtifacts[0].ClaimID != resp.ClaimRecords[0].ID {
		t.Fatalf("fetch cycle did not reuse source-refresh classification artifacts: %+v", resp)
	}

	listW := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/global-wire/fetch-cycles?story_id=story-supply-resilience", "", "user-cycle")
	if listW.Code != http.StatusOK {
		t.Fatalf("list fetch cycles status = %d body=%s", listW.Code, listW.Body.String())
	}
	var listResp globalWireFetchCycleResponse
	if err := json.NewDecoder(listW.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode fetch cycle list: %v", err)
	}
	if len(listResp.RegistryEntries) != 1 ||
		len(listResp.RecentCycles) != 1 ||
		listResp.RecentCycles[0].ID != resp.FetchCycle.ID ||
		len(listResp.SchedulerRuns) != 1 ||
		listResp.SchedulerRuns[0].FetchCycleID != resp.FetchCycle.ID {
		t.Fatalf("fetch cycle not listed durably: %+v", listResp)
	}
}

func TestHandleGlobalWireSourceRefreshClassifiesNoVisibleChangeWithoutCandidate(t *testing.T) {
	sourceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/source-service/search" {
			t.Fatalf("unexpected source service path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(sourceapi.SearchResponse{
			Query:    "unchanged port refresh",
			Provider: sourceapi.ProviderName,
			Metadata: sourceapi.Metadata{TargetKind: sourceapi.TargetKind},
			Results: []sourceapi.ItemResult{{
				Rank:          1,
				TargetKind:    sourceapi.TargetKind,
				ItemID:        "srcitem_port_unchanged",
				SourceID:      "rss:ports",
				SourceType:    "rss",
				FetchID:       "fetch-port-unchanged",
				Title:         "Port unchanged update",
				Body:          "No visible change from the existing port source neighborhood.",
				URL:           "https://example.test/ports-unchanged",
				CanonicalURL:  "https://example.test/ports-unchanged",
				ContentHash:   "hash-port-unchanged",
				EvidenceLevel: "source-service-ledger",
			}},
		})
	}))
	defer sourceServer.Close()
	t.Setenv("SOURCE_SERVICE_BASE_URL", sourceServer.URL)
	t.Setenv("SOURCE_SERVICE_URL", "")
	t.Setenv("SOURCECYCLED_API_URL", "")

	_, handler := testAPISetup(t)
	body := `{"story_id":"story-supply-resilience","query":"unchanged port refresh","max_results":1}`
	w := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/global-wire/source-refresh", body, "user-alpha")
	if w.Code != http.StatusOK {
		t.Fatalf("source refresh status = %d body=%s", w.Code, w.Body.String())
	}
	var resp globalWireSourceRefreshResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode source refresh: %v", err)
	}
	if resp.Status != "no-visible-change" ||
		resp.RefreshRun.UpdateClassification != "no-visible-change" ||
		resp.RefreshRun.StoryGraphAction != "no-storygraph-change" ||
		resp.RefreshRun.ProjectionAction != "no-projection-change-yet" {
		t.Fatalf("unexpected no-visible-change refresh: %+v", resp)
	}
	if resp.ContentItem == nil || resp.Contribution != nil || resp.Decision != nil || resp.Candidate != nil {
		t.Fatalf("no-visible-change should import source without review artifacts: %+v", resp)
	}

	listW := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/global-wire/reconciliation?story_id=story-supply-resilience", "", "user-alpha")
	if listW.Code != http.StatusOK {
		t.Fatalf("list reconciliation status = %d body=%s", listW.Code, listW.Body.String())
	}
	var listResp globalWireReconciliationResponse
	if err := json.NewDecoder(listW.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode reconciliation list: %v", err)
	}
	if len(listResp.Refreshes) != 1 || listResp.Refreshes[0].CandidateID != "" ||
		listResp.Refreshes[0].UpdateClassification != "no-visible-change" {
		t.Fatalf("no-visible-change refresh run missing or created candidate: %+v", listResp.Refreshes)
	}
	if len(listResp.Candidates) != 0 || len(listResp.Contributions) != 0 || len(listResp.Decisions) != 0 {
		t.Fatalf("no-visible-change should not create review queue artifacts: contributions=%+v decisions=%+v candidates=%+v", listResp.Contributions, listResp.Decisions, listResp.Candidates)
	}
}

func TestHandleGlobalWirePromotesClassifiedRefreshIntoStoryGraphAndPlatformVText(t *testing.T) {
	sourceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/source-service/search" {
			t.Fatalf("unexpected source service path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(sourceapi.SearchResponse{
			Query:    "urgent port prominence",
			Provider: sourceapi.ProviderName,
			Metadata: sourceapi.Metadata{TargetKind: sourceapi.TargetKind},
			Results: []sourceapi.ItemResult{{
				Rank:          1,
				TargetKind:    sourceapi.TargetKind,
				ItemID:        "srcitem_port_prominence",
				SourceID:      "rss:ports",
				SourceType:    "rss",
				FetchID:       "fetch-port-prominence",
				Title:         "Urgent major port disruption moves to front page",
				Body:          "Breaking emergency port update may change front page prominence and lead evidence.",
				URL:           "https://example.test/ports-prominence",
				CanonicalURL:  "https://example.test/ports-prominence",
				ContentHash:   "hash-port-prominence",
				EvidenceLevel: "source-service-ledger",
			}},
		})
	}))
	defer sourceServer.Close()
	t.Setenv("SOURCE_SERVICE_BASE_URL", sourceServer.URL)
	t.Setenv("SOURCE_SERVICE_URL", "")
	t.Setenv("SOURCECYCLED_API_URL", "")

	_, handler := testAPISetup(t)
	storiesBeforeW := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/global-wire/stories", "", "user-alpha")
	if storiesBeforeW.Code != http.StatusOK {
		t.Fatalf("stories before status = %d body=%s", storiesBeforeW.Code, storiesBeforeW.Body.String())
	}
	var storiesBefore globalWireStoriesResponse
	if err := json.NewDecoder(storiesBeforeW.Body).Decode(&storiesBefore); err != nil {
		t.Fatalf("decode stories before: %v", err)
	}
	beforeStory := storiesBefore.Stories[0]
	beforeDoc, err := handler.rt.Store().GetDocument(context.Background(), beforeStory.StoryVTextDoc, "user-alpha")
	if err != nil {
		t.Fatalf("get platform story doc before: %v", err)
	}

	refreshBody := `{"story_id":"story-supply-resilience","query":"urgent port prominence","max_results":1}`
	refreshW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/global-wire/source-refresh", refreshBody, "user-alpha")
	if refreshW.Code != http.StatusCreated {
		t.Fatalf("source refresh status = %d body=%s", refreshW.Code, refreshW.Body.String())
	}
	var refreshResp globalWireSourceRefreshResponse
	if err := json.NewDecoder(refreshW.Body).Decode(&refreshResp); err != nil {
		t.Fatalf("decode source refresh: %v", err)
	}
	if refreshResp.RefreshRun.UpdateClassification != "front-page-prominence-changed" ||
		refreshResp.Candidate == nil ||
		refreshResp.Candidate.CandidateKind != "front-page-prominence-changed" ||
		refreshResp.Candidate.SourceTier != "lead" {
		t.Fatalf("refresh did not create prominence candidate: %+v", refreshResp)
	}

	promoteBody := `{"candidate_id":"` + refreshResp.Candidate.ID + `","decision":"promoted","note":"Platform review accepts prominence update."}`
	promoteW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/global-wire/graph-candidates", promoteBody, "user-alpha")
	if promoteW.Code != http.StatusCreated {
		t.Fatalf("promote graph candidate status = %d body=%s", promoteW.Code, promoteW.Body.String())
	}
	var promoteResp globalWireGraphCandidateReviewResponse
	if err := json.NewDecoder(promoteW.Body).Decode(&promoteResp); err != nil {
		t.Fatalf("decode graph promotion: %v", err)
	}
	if promoteResp.Story.ChangeState != "front-page prominence changed" ||
		promoteResp.Story.Prominence <= beforeStory.Prominence ||
		promoteResp.Story.Manifest.Lead[len(promoteResp.Story.Manifest.Lead)-1].ContentID != refreshResp.ContentItem.ContentID {
		t.Fatalf("promoted story did not apply prominence semantics: before=%+v after=%+v", beforeStory, promoteResp.Story)
	}
	if !strings.Contains(promoteResp.Promotion.AppliedChange, "created PlatformStory VText revision") {
		t.Fatalf("promotion did not record platform story revision: %+v", promoteResp.Promotion)
	}
	afterDoc, err := handler.rt.Store().GetDocument(context.Background(), beforeStory.StoryVTextDoc, "user-alpha")
	if err != nil {
		t.Fatalf("get platform story doc after: %v", err)
	}
	if afterDoc.CurrentRevisionID == beforeDoc.CurrentRevisionID {
		t.Fatalf("platform story current revision did not change: before=%q after=%q", beforeDoc.CurrentRevisionID, afterDoc.CurrentRevisionID)
	}
	afterRev, err := handler.rt.Store().GetRevision(context.Background(), afterDoc.CurrentRevisionID, "user-alpha")
	if err != nil {
		t.Fatalf("get platform story revision after: %v", err)
	}
	if afterRev.AuthorKind != types.AuthorAppAgent ||
		afterRev.ParentRevisionID != beforeDoc.CurrentRevisionID ||
		!strings.Contains(afterRev.Content, "reviewed source context") ||
		!strings.Contains(afterRev.Content, "source:gw-src-") ||
		strings.Contains(afterRev.Content, "source_content_id") ||
		strings.Contains(afterRev.Content, "User-owned forks, edits, and contributions remain separate") {
		t.Fatalf("platform story revision did not preserve article-body/source-ref contract: %+v", afterRev)
	}
	afterMeta := decodeRevisionMetadata(afterRev.Metadata)
	if afterMeta["created_from"] != "global_wire_graph_candidate_promotion" ||
		afterMeta["storygraph_id"] != "story-supply-resilience" ||
		afterMeta["candidate_id"] != refreshResp.Candidate.ID ||
		afterMeta["source_content_id"] != refreshResp.ContentItem.ContentID ||
		afterMeta["candidate_kind"] != "front-page-prominence-changed" {
		t.Fatalf("platform story revision missing structured promotion provenance: %#v", afterMeta)
	}
	sourceEntities := decodeVTextSourceEntities(afterMeta["source_entities"])
	foundPromotedSource := false
	for _, entity := range sourceEntities {
		if entity.Target.ContentID == refreshResp.ContentItem.ContentID {
			foundPromotedSource = true
			break
		}
	}
	if !foundPromotedSource {
		t.Fatalf("platform story revision source_entities missing promoted source %q: %#v", refreshResp.ContentItem.ContentID, sourceEntities)
	}
}

func TestHandleGlobalWireReconciliationRecordsDecisionWithoutMutatingStoryGraph(t *testing.T) {
	_, handler := testAPISetup(t)

	storiesBeforeW := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/global-wire/stories", "", "user-alpha")
	if storiesBeforeW.Code != http.StatusOK {
		t.Fatalf("stories before status = %d body=%s", storiesBeforeW.Code, storiesBeforeW.Body.String())
	}
	var storiesBefore globalWireStoriesResponse
	if err := json.NewDecoder(storiesBeforeW.Body).Decode(&storiesBefore); err != nil {
		t.Fatalf("decode stories before: %v", err)
	}
	beforeManifest := storiesBefore.Stories[0].Manifest

	contributionBody := `{"story_id":"story-supply-resilience","kind":"source","headline":"Port backlog recedes","text":"Reviewer source text for reconciliation."}`
	contributionW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/global-wire/contributions", contributionBody, "user-alpha")
	if contributionW.Code != http.StatusCreated {
		t.Fatalf("create contribution status = %d body=%s", contributionW.Code, contributionW.Body.String())
	}
	var contribution types.GlobalWireContribution
	if err := json.NewDecoder(contributionW.Body).Decode(&contribution); err != nil {
		t.Fatalf("decode contribution: %v", err)
	}
	if contribution.SourceContentID == "" {
		t.Fatalf("contribution source_content_id is empty: %+v", contribution)
	}

	listW := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/global-wire/reconciliation?story_id=story-supply-resilience", "", "user-alpha")
	if listW.Code != http.StatusOK {
		t.Fatalf("list reconciliation status = %d body=%s", listW.Code, listW.Body.String())
	}
	var listResp globalWireReconciliationResponse
	if err := json.NewDecoder(listW.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode reconciliation list: %v", err)
	}
	if len(listResp.Contributions) != 1 {
		t.Fatalf("reconciliation contribution count = %d, want 1", len(listResp.Contributions))
	}
	if listResp.SourceItems[contribution.SourceContentID].ContentID != contribution.SourceContentID {
		t.Fatalf("reconciliation source item missing: %+v", listResp.SourceItems)
	}

	decisionBody := `{"contribution_id":"` + contribution.ID + `","decision":"accepted","note":"Evidence is relevant; send to graph review."}`
	decisionW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/global-wire/reconciliation", decisionBody, "user-alpha")
	if decisionW.Code != http.StatusCreated {
		t.Fatalf("create reconciliation decision status = %d body=%s", decisionW.Code, decisionW.Body.String())
	}
	var decisionResp globalWireReconciliationCreateResponse
	if err := json.NewDecoder(decisionW.Body).Decode(&decisionResp); err != nil {
		t.Fatalf("decode reconciliation decision: %v", err)
	}
	if decisionResp.Decision.Decision != "accepted" || decisionResp.Decision.SourceContentID != contribution.SourceContentID {
		t.Fatalf("unexpected reconciliation decision: %+v", decisionResp.Decision)
	}
	if decisionResp.Contribution.ResearchState != "accepted-for-graph-review" {
		t.Fatalf("contribution research_state = %q", decisionResp.Contribution.ResearchState)
	}
	if decisionResp.SourceItem == nil || decisionResp.SourceItem.ContentID != contribution.SourceContentID {
		t.Fatalf("decision source item missing: %+v", decisionResp.SourceItem)
	}
	if decisionResp.Candidate == nil {
		t.Fatalf("accepted decision did not create graph update candidate")
	}
	if decisionResp.Candidate.SourceContentID != contribution.SourceContentID ||
		decisionResp.Candidate.SourceTier != "supporting" ||
		decisionResp.Candidate.EdgeKind != "shared-source-neighborhood" ||
		decisionResp.Candidate.Status != "candidate-review" {
		t.Fatalf("unexpected graph update candidate: %+v", decisionResp.Candidate)
	}

	candidateListW := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/global-wire/reconciliation?story_id=story-supply-resilience", "", "user-alpha")
	if candidateListW.Code != http.StatusOK {
		t.Fatalf("list candidates status = %d body=%s", candidateListW.Code, candidateListW.Body.String())
	}
	var candidateList globalWireReconciliationResponse
	if err := json.NewDecoder(candidateListW.Body).Decode(&candidateList); err != nil {
		t.Fatalf("decode candidate list: %v", err)
	}
	if len(candidateList.Candidates) != 1 {
		t.Fatalf("candidate count = %d, want 1; response=%+v", len(candidateList.Candidates), candidateList)
	}
	if candidateList.Candidates[0].DecisionID != decisionResp.Decision.ID ||
		candidateList.Candidates[0].ContributionID != contribution.ID {
		t.Fatalf("candidate lineage missing: %+v", candidateList.Candidates[0])
	}

	storiesAfterW := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/global-wire/stories", "", "user-alpha")
	if storiesAfterW.Code != http.StatusOK {
		t.Fatalf("stories after status = %d body=%s", storiesAfterW.Code, storiesAfterW.Body.String())
	}
	var storiesAfter globalWireStoriesResponse
	if err := json.NewDecoder(storiesAfterW.Body).Decode(&storiesAfter); err != nil {
		t.Fatalf("decode stories after: %v", err)
	}
	afterManifest := storiesAfter.Stories[0].Manifest
	if len(afterManifest.Lead) != len(beforeManifest.Lead) || len(afterManifest.Supporting) != len(beforeManifest.Supporting) ||
		len(afterManifest.Contrary) != len(beforeManifest.Contrary) || len(afterManifest.Context) != len(beforeManifest.Context) {
		t.Fatalf("StoryGraph manifest mutated: before=%+v after=%+v", beforeManifest, afterManifest)
	}

	promoteBody := `{"candidate_id":"` + decisionResp.Candidate.ID + `","decision":"promoted","note":"Platform review accepts bounded source-manifest update."}`
	promoteW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/global-wire/graph-candidates", promoteBody, "user-alpha")
	if promoteW.Code != http.StatusCreated {
		t.Fatalf("promote graph candidate status = %d body=%s", promoteW.Code, promoteW.Body.String())
	}
	var promoteResp globalWireGraphCandidateReviewResponse
	if err := json.NewDecoder(promoteW.Body).Decode(&promoteResp); err != nil {
		t.Fatalf("decode graph promotion: %v", err)
	}
	if promoteResp.Candidate.Status != "promoted-to-storygraph" ||
		promoteResp.Promotion.Decision != "promoted" ||
		promoteResp.Promotion.SourceContentID != contribution.SourceContentID {
		t.Fatalf("unexpected graph promotion response: %+v", promoteResp)
	}
	if len(promoteResp.ProjectionReviews) != len(promoteResp.Story.StyleSources) {
		t.Fatalf("projection review count = %d, want %d: %+v", len(promoteResp.ProjectionReviews), len(promoteResp.Story.StyleSources), promoteResp.ProjectionReviews)
	}
	if len(promoteResp.ProjectionReviews) == 0 ||
		promoteResp.ProjectionReviews[0].CandidateID != decisionResp.Candidate.ID ||
		promoteResp.ProjectionReviews[0].PromotionID != promoteResp.Promotion.ID ||
		promoteResp.ProjectionReviews[0].Status != "projection-review-required" {
		t.Fatalf("projection review lineage missing: %+v", promoteResp.ProjectionReviews)
	}
	if len(promoteResp.Story.Manifest.Supporting) != len(beforeManifest.Supporting)+1 {
		t.Fatalf("promoted story supporting count = %d, want %d", len(promoteResp.Story.Manifest.Supporting), len(beforeManifest.Supporting)+1)
	}
	promotedSource := promoteResp.Story.Manifest.Supporting[len(promoteResp.Story.Manifest.Supporting)-1]
	if promotedSource.ContentID != contribution.SourceContentID || promotedSource.Role != "supporting" {
		t.Fatalf("promoted source lineage missing: %+v", promotedSource)
	}

	promotedListW := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/global-wire/reconciliation?story_id=story-supply-resilience", "", "user-alpha")
	if promotedListW.Code != http.StatusOK {
		t.Fatalf("list promoted reconciliation status = %d body=%s", promotedListW.Code, promotedListW.Body.String())
	}
	var promotedList globalWireReconciliationResponse
	if err := json.NewDecoder(promotedListW.Body).Decode(&promotedList); err != nil {
		t.Fatalf("decode promoted list: %v", err)
	}
	if len(promotedList.Promotions) != 1 || promotedList.Promotions[0].CandidateID != decisionResp.Candidate.ID {
		t.Fatalf("promotion decision missing from reconciliation list: %+v", promotedList.Promotions)
	}
	if len(promotedList.ProjectionReviews) != len(promoteResp.ProjectionReviews) {
		t.Fatalf("projection reviews missing from reconciliation list: %+v", promotedList.ProjectionReviews)
	}

	draftBody := `{"review_id":"` + promoteResp.ProjectionReviews[0].ID + `"}`
	draftW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/global-wire/projection-reviews", draftBody, "user-alpha")
	if draftW.Code != http.StatusCreated {
		t.Fatalf("create projection draft status = %d body=%s", draftW.Code, draftW.Body.String())
	}
	var draftResp globalWireProjectionReviewDraftResponse
	if err := json.NewDecoder(draftW.Body).Decode(&draftResp); err != nil {
		t.Fatalf("decode projection draft: %v", err)
	}
	if draftResp.Review.Status != "draft-created" ||
		draftResp.Review.DraftStoryDocID != draftResp.Document.DocID ||
		draftResp.Document.CurrentRevisionID != draftResp.Revision.RevisionID {
		t.Fatalf("projection draft response missing lineage: %+v", draftResp)
	}
	if draftResp.Revision.AuthorKind != types.AuthorAppAgent ||
		!strings.Contains(draftResp.Revision.Content, "source:gw-src-") ||
		strings.Contains(draftResp.Revision.Content, "Draft state:") ||
		strings.Contains(draftResp.Revision.Content, "StoryGraph id:") ||
		strings.Contains(draftResp.Revision.Content, "Projection review id:") ||
		strings.Contains(draftResp.Revision.Content, "Promoted source content id:") ||
		strings.Contains(draftResp.Revision.Content, "## Draft Revision Notes") {
		t.Fatalf("projection draft VText content/authorship invalid: %+v", draftResp.Revision)
	}
	draftMeta := decodeRevisionMetadata(draftResp.Revision.Metadata)
	if draftMeta["artifact_kind"] != "article_revision_draft" || draftMeta["article_version"] != true {
		t.Fatalf("projection draft metadata did not mark article draft: %#v", draftMeta)
	}
	if len(decodeVTextSourceEntities(draftMeta["source_entities"])) == 0 {
		t.Fatalf("projection draft metadata missing source entities: %#v", draftMeta)
	}
	var draftCitations []types.Citation
	if err := json.Unmarshal(draftResp.Revision.Citations, &draftCitations); err != nil {
		t.Fatalf("decode projection draft citations: %v", err)
	}
	if len(draftCitations) < 5 {
		t.Fatalf("projection draft citations too sparse: %+v", draftCitations)
	}

	draftedListW := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/global-wire/reconciliation?story_id=story-supply-resilience", "", "user-alpha")
	if draftedListW.Code != http.StatusOK {
		t.Fatalf("list drafted reconciliation status = %d body=%s", draftedListW.Code, draftedListW.Body.String())
	}
	var draftedList globalWireReconciliationResponse
	if err := json.NewDecoder(draftedListW.Body).Decode(&draftedList); err != nil {
		t.Fatalf("decode drafted list: %v", err)
	}
	var draftedReview types.GlobalWireProjectionReview
	for _, rec := range draftedList.ProjectionReviews {
		if rec.ID == draftResp.Review.ID {
			draftedReview = rec
			break
		}
	}
	if draftedReview.Status != "draft-created" || draftedReview.DraftStoryDocID == "" {
		t.Fatalf("drafted projection review missing from reconciliation list: %+v", draftedList.ProjectionReviews)
	}

	approveBody := `{"review_id":"` + draftResp.Review.ID + `","action":"approve"}`
	approveW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/global-wire/projection-reviews", approveBody, "user-alpha")
	if approveW.Code != http.StatusOK {
		t.Fatalf("approve projection draft status = %d body=%s", approveW.Code, approveW.Body.String())
	}
	var approveResp globalWireProjectionReviewDraftResponse
	if err := json.NewDecoder(approveW.Body).Decode(&approveResp); err != nil {
		t.Fatalf("decode projection approval: %v", err)
	}
	if approveResp.Review.Status != "approved" ||
		approveResp.Review.ApprovedStoryDocID != approveResp.Document.DocID ||
		approveResp.Review.ApprovedRevisionID != approveResp.Revision.RevisionID ||
		approveResp.Projection.StoryDocID != approveResp.Document.DocID ||
		approveResp.Projection.Text != approveResp.Revision.Content {
		t.Fatalf("projection approval response missing lineage: %+v", approveResp)
	}
	if approveResp.Revision.AuthorKind != types.AuthorAppAgent ||
		approveResp.Revision.ParentRevisionID == "" ||
		strings.Contains(approveResp.Revision.Content, "Review status: approved") ||
		strings.Contains(approveResp.Revision.Content, "Projection Review Approval") ||
		strings.Contains(approveResp.Revision.Content, "user-owned forks remain separate") {
		t.Fatalf("projection approval revision invalid: %+v", approveResp.Revision)
	}
	approveMeta := decodeRevisionMetadata(approveResp.Revision.Metadata)
	if approveMeta["artifact_kind"] != "article_revision" || approveMeta["article_version"] != true {
		t.Fatalf("projection approval metadata did not mark article revision: %#v", approveMeta)
	}
	if len(decodeVTextSourceEntities(approveMeta["source_entities"])) == 0 {
		t.Fatalf("projection approval metadata missing source entities: %#v", approveMeta)
	}
	approvedDocW := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/vtext/documents/"+approveResp.Document.DocID, "", "user-alpha")
	if approvedDocW.Code != http.StatusOK {
		t.Fatalf("get approved projection VText status = %d body=%s", approvedDocW.Code, approvedDocW.Body.String())
	}
	approvedStoriesW := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/global-wire/stories", "", "user-alpha")
	if approvedStoriesW.Code != http.StatusOK {
		t.Fatalf("stories after projection approval status = %d body=%s", approvedStoriesW.Code, approvedStoriesW.Body.String())
	}
	var approvedStories globalWireStoriesResponse
	if err := json.NewDecoder(approvedStoriesW.Body).Decode(&approvedStories); err != nil {
		t.Fatalf("decode approved stories: %v", err)
	}
	approvedStory := approvedStories.Stories[0]
	if approvedStory.ProjectionVTextDocs[draftResp.Review.StyleID] != approveResp.Document.DocID ||
		strings.Contains(approvedStory.Projections[draftResp.Review.StyleID], "Review status: approved") ||
		strings.Contains(approvedStory.Projections[draftResp.Review.StyleID], "Projection Review Approval") {
		t.Fatalf("approved projection relation not visible in StoryGraph response: docs=%+v projection=%q", approvedStory.ProjectionVTextDocs, approvedStory.Projections[draftResp.Review.StyleID])
	}
}
