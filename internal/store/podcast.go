package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// UpsertPodcastSubscription stores a user's durable subscription to a podcast
// feed and links it to the latest imported RSS ContentItem when available.
func (s *Store) UpsertPodcastSubscription(ctx context.Context, rec types.PodcastSubscription) (types.PodcastSubscription, error) {
	rec.OwnerID = strings.TrimSpace(rec.OwnerID)
	rec.FeedURL = strings.TrimSpace(rec.FeedURL)
	if rec.OwnerID == "" || rec.FeedURL == "" {
		return types.PodcastSubscription{}, fmt.Errorf("owner_id and feed_url are required")
	}
	now := time.Now().UTC()
	if rec.SubscriptionID == "" {
		rec.SubscriptionID = "podsub-" + mediaIdentityHash(rec.OwnerID + ":" + rec.FeedURL)[:24]
	}
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = now
	}
	if rec.UpdatedAt.IsZero() {
		rec.UpdatedAt = now
	}
	if err := s.CreatePodcastSubscriptionOG(ctx, rec); err != nil {
		return types.PodcastSubscription{}, fmt.Errorf("upsert podcast subscription: %w", err)
	}
	return rec, nil
}

// ListPodcastSubscriptions returns a user's podcast library newest first.
func (s *Store) ListPodcastSubscriptions(ctx context.Context, ownerID string, limit int) ([]types.PodcastSubscription, error) {
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return nil, fmt.Errorf("owner_id is required")
	}
	if limit <= 0 {
		limit = 50
	}
	return s.ListPodcastSubscriptionsByOwnerOG(ctx, ownerID, limit)
}
