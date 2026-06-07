package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
}
