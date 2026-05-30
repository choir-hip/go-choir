package sources

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/mmcdole/gofeed"
)

type RSSPoller struct {
	Client    *http.Client
	UserAgent string
}

func NewRSSPoller(userAgent string) *RSSPoller {
	return &RSSPoller{
		Client: &http.Client{
			Timeout: 30 * time.Second,
		},
		UserAgent: userAgent,
	}
}

func (p *RSSPoller) Poll(ctx context.Context, source *Source) ([]Item, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", source.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", p.UserAgent)
	if source.LastETag != "" {
		req.Header.Set("If-None-Match", source.LastETag)
	}
	if source.LastModified != "" {
		req.Header.Set("If-Modified-Since", source.LastModified)
	}

	resp, err := p.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		return nil, nil // No new content
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Update Source metadata for next poll
	source.LastETag = resp.Header.Get("ETag")
	source.LastModified = resp.Header.Get("Last-Modified")
	source.LastPolled = time.Now()

	fp := gofeed.NewParser()
	feed, err := fp.Parse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse feed: %w", err)
	}

	items := make([]Item, 0, len(feed.Items))
	for _, feedItem := range feed.Items {
		published := time.Now()
		if feedItem.PublishedParsed != nil {
			published = *feedItem.PublishedParsed
		}

		item := Item{
			ID:         fmt.Sprintf("rss:%s:%s", source.ID, feedItem.GUID),
			SourceID:   source.ID,
			OriginalID: feedItem.GUID,
			Title:      feedItem.Title,
			Body:       feedItem.Description,
			URL:        feedItem.Link,
			Published:  published,
			FetchedAt:  time.Now(),
			Verticals:  source.Verticals,
		}
		items = append(items, item)
	}

	return items, nil
}
