package store

import (
	"context"
	"database/sql"
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
	_, err := s.textureHandle().ExecContext(ctx,
		`INSERT INTO podcast_subscriptions (
			subscription_id, owner_id, feed_url, content_id, title, author,
			artwork_url, last_fetched_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			content_id = VALUES(content_id),
			title = VALUES(title),
			author = VALUES(author),
			artwork_url = VALUES(artwork_url),
			last_fetched_at = VALUES(last_fetched_at),
			updated_at = VALUES(updated_at)`,
		rec.SubscriptionID,
		rec.OwnerID,
		rec.FeedURL,
		strings.TrimSpace(rec.ContentID),
		strings.TrimSpace(rec.Title),
		strings.TrimSpace(rec.Author),
		strings.TrimSpace(rec.ArtworkURL),
		formatNullableTime(rec.LastFetchedAt),
		rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
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
	rows, err := s.textureHandle().QueryContext(ctx,
		`SELECT subscription_id, owner_id, feed_url, content_id, title, author,
		        artwork_url, last_fetched_at, created_at, updated_at
		   FROM podcast_subscriptions
		  WHERE owner_id = ?
		  ORDER BY updated_at DESC
		  LIMIT ?`,
		ownerID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query podcast subscriptions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := []types.PodcastSubscription{}
	for rows.Next() {
		rec, err := scanPodcastSubscription(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate podcast subscriptions: %w", err)
	}
	return out, nil
}

func scanPodcastSubscription(row interface{ Scan(...any) error }) (types.PodcastSubscription, error) {
	var (
		rec           types.PodcastSubscription
		lastFetchedAt sql.NullString
		createdAt     string
		updatedAt     string
	)
	if err := row.Scan(
		&rec.SubscriptionID,
		&rec.OwnerID,
		&rec.FeedURL,
		&rec.ContentID,
		&rec.Title,
		&rec.Author,
		&rec.ArtworkURL,
		&lastFetchedAt,
		&createdAt,
		&updatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return types.PodcastSubscription{}, ErrNotFound
		}
		return types.PodcastSubscription{}, fmt.Errorf("scan podcast subscription: %w", err)
	}
	if lastFetchedAt.Valid && strings.TrimSpace(lastFetchedAt.String) != "" {
		rec.LastFetchedAt = parsePodcastStoreTime(lastFetchedAt.String)
	}
	rec.CreatedAt = parsePodcastStoreTime(createdAt)
	rec.UpdatedAt = parsePodcastStoreTime(updatedAt)
	return rec, nil
}

func formatNullableTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func parsePodcastStoreTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}
	}
	return parsed
}
