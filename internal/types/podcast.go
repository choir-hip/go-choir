package types

import "time"

// PodcastSubscription is the durable library row for a podcast feed. The feed
// content itself remains a ContentItem so the Podcast app can reuse the shared
// content substrate while subscription state survives library refreshes.
type PodcastSubscription struct {
	SubscriptionID string       `json:"subscription_id"`
	OwnerID        string       `json:"owner_id"`
	FeedURL        string       `json:"feed_url"`
	ContentID      string       `json:"content_id,omitempty"`
	Title          string       `json:"title,omitempty"`
	Author         string       `json:"author,omitempty"`
	ArtworkURL     string       `json:"artwork_url,omitempty"`
	LastFetchedAt  time.Time    `json:"last_fetched_at,omitempty"`
	CreatedAt      time.Time    `json:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"`
	ContentItem    *ContentItem `json:"content_item,omitempty"`
}
