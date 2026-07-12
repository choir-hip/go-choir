package platform

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestListUniversalWireStoriesUsesPublishedObjectsNewestFirst(t *testing.T) {
	store, root := openTestPlatformStore(t)
	service := NewService(store, filepath.Join(root, "artifacts"), "")
	ctx := context.Background()
	older, err := service.PublishTexture(ctx, PublishTextureRequest{
		OwnerID:          "universal-wire-platform",
		SourceDocID:      "doc-old",
		SourceRevisionID: "rev-old",
		Title:            "Older.texture",
		Content:          "# Older story\n\nOlder published paragraph.",
		RequestedBy:      "wire-agent",
	})
	if err != nil {
		t.Fatalf("publish older: %v", err)
	}
	newer, err := service.PublishTexture(ctx, PublishTextureRequest{
		OwnerID:          "universal-wire-platform",
		SourceDocID:      "doc-new",
		SourceRevisionID: "rev-new",
		Title:            "Newer.texture",
		Content:          "# Newest story\n\nNewest published paragraph.",
		RequestedBy:      "wire-agent",
	})
	if err != nil {
		t.Fatalf("publish newer: %v", err)
	}
	if platformTableCount(t, store, "platform_texture_documents") != 0 || platformTableCount(t, store, "platform_texture_revisions") != 0 {
		t.Fatal("story fixture unexpectedly used platform Texture mirror tables")
	}

	got, err := service.ListUniversalWireStories(ctx)
	if err != nil {
		t.Fatalf("list stories: %v", err)
	}
	if len(got.Stories) != 2 {
		t.Fatalf("story count = %d, want 2", len(got.Stories))
	}
	if got.Stories[0].StoryTextureDoc != "doc-new" || got.Stories[1].StoryTextureDoc != "doc-old" {
		t.Fatalf("story order = [%s %s], want newest first", got.Stories[0].StoryTextureDoc, got.Stories[1].StoryTextureDoc)
	}
	if got.Stories[0].Headline != "Newest story" || got.Stories[0].Dek != "Newest published paragraph." {
		t.Fatalf("newest story shape = %+v", got.Stories[0])
	}
	if got.Stories[0].PlatformRoutePath != newer.RoutePath || got.Stories[1].PlatformRoutePath != older.RoutePath {
		t.Fatalf("story routes = [%s %s], want canonical publication receipts", got.Stories[0].PlatformRoutePath, got.Stories[1].PlatformRoutePath)
	}
	if got.Source != "corpusd-publications" || got.StyleSources == nil || got.Stories[0].Related == nil || got.Stories[0].Claims == nil {
		t.Fatalf("response contract contains nil collection: %+v", got)
	}
}

func TestInternalUniversalWireStoriesRouteRequiresInternalCaller(t *testing.T) {
	store, root := openTestPlatformStore(t)
	handler := NewHandler(NewService(store, filepath.Join(root, "artifacts"), ""))
	denied := httptest.NewRecorder()
	handler.HandleInternalUniversalWireStories(denied, httptest.NewRequest(http.MethodGet, "/internal/platform/universal-wire/stories", nil))
	if denied.Code != http.StatusForbidden {
		t.Fatalf("denied status = %d, want %d", denied.Code, http.StatusForbidden)
	}
	req := httptest.NewRequest(http.MethodGet, "/internal/platform/universal-wire/stories", nil)
	req.Header.Set("X-Internal-Caller", "true")
	w := httptest.NewRecorder()
	handler.HandleInternalUniversalWireStories(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	var got UniversalWireStoriesResponse
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Stories == nil || got.StyleSources == nil || got.Source != "corpusd-publications" {
		t.Fatalf("empty response shape = %+v", got)
	}
}
