//go:build comprehensive

package runtime

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
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

func TestPodcastRefreshSeedsExistingRSSContent(t *testing.T) {
	t.Parallel()
	feed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(`<?xml version="1.0"?>
<rss><channel>
  <title>Existing Feed</title>
  <item><title>Episode One</title><enclosure url="https://example.com/episode-one.mp3" type="audio/mpeg"/></item>
</channel></rss>`))
	}))
	defer feed.Close()

	_, handler := testAPISetup(t)
	createBody := `{
		"source_type":"url",
		"media_type":"application/rss+xml",
		"app_hint":"podcast",
		"title":"Existing Feed",
		"source_url":` + strconv.Quote(feed.URL+"/feed.rss") + `,
		"text_content":"<rss><channel><title>Existing Feed</title></channel></rss>"
	}`
	createW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/content/items", createBody, "user-podcast")
	if createW.Code != http.StatusCreated {
		t.Fatalf("create content status = %d body=%s", createW.Code, createW.Body.String())
	}

	refreshW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/podcast/subscriptions/refresh?limit=10", `{}`, "user-podcast")
	if refreshW.Code != http.StatusOK {
		t.Fatalf("refresh status = %d body=%s", refreshW.Code, refreshW.Body.String())
	}
	var refreshed podcastSubscriptionsResponse
	if err := json.Unmarshal(refreshW.Body.Bytes(), &refreshed); err != nil {
		t.Fatalf("decode refresh: %v", err)
	}
	if len(refreshed.Subscriptions) != 1 || refreshed.Subscriptions[0].FeedURL != feed.URL+"/feed.rss" {
		t.Fatalf("refreshed subscriptions = %+v", refreshed.Subscriptions)
	}
}

func TestPodcastSubscriptionsPersistAndListContent(t *testing.T) {
	allowPrivateSourceFetchForTest(t)
	feed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(`<?xml version="1.0"?>
<rss><channel>
  <title>Mission Gradient Radio</title>
  <item><title>Episode One</title><enclosure url="https://example.com/episode-one.mp3" type="audio/mpeg"/></item>
</channel></rss>`))
	}))
	defer feed.Close()

	_, handler := testAPISetup(t)
	body := `{"feed_url":` + strconv.Quote(feed.URL+"/feed.rss") + `,"title":"Mission Gradient Radio"}`
	createW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/podcast/subscriptions", body, "user-podcast")
	if createW.Code != http.StatusCreated {
		t.Fatalf("subscribe status = %d body=%s", createW.Code, createW.Body.String())
	}
	var created podcastSubscriptionResponse
	if err := json.Unmarshal(createW.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode subscribe: %v", err)
	}
	if created.Subscription.ContentID == "" || created.Subscription.ContentItem == nil {
		t.Fatalf("subscription missing imported content: %+v", created.Subscription)
	}
	if created.Subscription.ContentItem.AppHint != "podcast" {
		t.Fatalf("content app hint = %q", created.Subscription.ContentItem.AppHint)
	}

	listW := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/podcast/subscriptions?limit=10", "", "user-podcast")
	if listW.Code != http.StatusOK {
		t.Fatalf("list status = %d body=%s", listW.Code, listW.Body.String())
	}
	var listed podcastSubscriptionsResponse
	if err := json.Unmarshal(listW.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(listed.Subscriptions) != 1 || listed.Subscriptions[0].ContentItem == nil {
		t.Fatalf("listed subscriptions = %+v", listed.Subscriptions)
	}
}
