package runtime

import (
	"encoding/json"
	"net/http"
	"testing"
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
	if len(story.Manifest.Lead) == 0 || len(story.Manifest.Supporting) == 0 || len(story.Manifest.Contrary) == 0 || len(story.Manifest.Context) == 0 {
		t.Fatalf("story manifest is missing required evidence tiers: %+v", story.Manifest)
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
