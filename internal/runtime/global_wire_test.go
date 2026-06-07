package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

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
