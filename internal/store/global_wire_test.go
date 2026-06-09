package store

import (
	"context"
	"testing"
)

func TestGlobalWireStoriesDoNotSeedFakeFrontPage(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	ownerID := "global-wire-empty-user"

	stories, err := s.ListGlobalWireStories(ctx, ownerID)
	if err != nil {
		t.Fatalf("list global wire stories: %v", err)
	}
	if len(stories) != 0 {
		t.Fatalf("stories length = %d, want honest empty state", len(stories))
	}

	if _, err := s.GetGlobalWireStory(ctx, ownerID, "story-supply-resilience"); err != ErrNotFound {
		t.Fatalf("seed story lookup err = %v, want ErrNotFound", err)
	}
}
