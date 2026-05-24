//go:build comprehensive

package runtime

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPodcastSearchUsesConfiguredProvider(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("term") != "mission gradient" {
			t.Fatalf("term = %q", r.URL.Query().Get("term"))
		}
		writeAPIJSON(w, http.StatusOK, map[string]any{
			"results": []map[string]any{{
				"collectionName":   "Mission Gradient Radio",
				"artistName":       "Choir",
				"feedUrl":          "https://example.com/mission-gradient.rss",
				"artworkUrl100":    "https://example.com/art.png",
				"trackCount":       3,
				"primaryGenreName": "Technology",
			}},
		})
	}))
	defer provider.Close()
	t.Setenv("CHOIR_PODCAST_SEARCH_URL", provider.URL+"/search")

	_, handler := testAPISetup(t)
	w := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/podcast/search?q=mission%20gradient&limit=5", "", "user-podcast")
	if w.Code != http.StatusOK {
		t.Fatalf("search status = %d body=%s", w.Code, w.Body.String())
	}
	var resp podcastSearchResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ProviderStatus != "success" || len(resp.Results) != 1 {
		t.Fatalf("response = %+v", resp)
	}
	if resp.Results[0].FeedURL != "https://example.com/mission-gradient.rss" {
		t.Fatalf("feed url = %q", resp.Results[0].FeedURL)
	}
}

func TestPodcastSearchFallsBackToLibrary(t *testing.T) {
	t.Setenv("CHOIR_PODCAST_SEARCH_URL", "http://127.0.0.1:1/search")
	_, handler := testAPISetup(t)
	createBody := `{
		"source_type":"url",
		"media_type":"application/rss+xml",
		"app_hint":"podcast",
		"title":"Mission Gradient Radio",
		"source_url":"https://example.com/mission-gradient.rss",
		"text_content":"<rss><channel><title>Mission Gradient Radio</title></channel></rss>"
	}`
	createW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/content/items", createBody, "user-podcast")
	if createW.Code != http.StatusCreated {
		t.Fatalf("create content status = %d body=%s", createW.Code, createW.Body.String())
	}
	searchW := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/podcast/search?q=Mission&limit=5", "", "user-podcast")
	if searchW.Code != http.StatusOK {
		t.Fatalf("search status = %d body=%s", searchW.Code, searchW.Body.String())
	}
	var resp podcastSearchResponse
	if err := json.Unmarshal(searchW.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Results) != 1 || resp.Results[0].Source != "library" {
		t.Fatalf("fallback response = %+v", resp)
	}
}
