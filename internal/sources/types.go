package sources

import "time"

type SourceType string

const (
	SourceTypeRSS         SourceType = "rss"
	SourceTypeTelegram    SourceType = "telegram"
	SourceTypeGDELT       SourceType = "gdelt"
	SourceTypePolymarket  SourceType = "polymarket"
)

type Source struct {
	ID                  string     `json:"id"`
	Type                SourceType `json:"type"`
	Name                string     `json:"name"`
	URL                 string     `json:"url"`
	Verticals           []string   `json:"verticals"`
	PollIntervalSeconds int        `json:"poll_interval_seconds"`
	LastPolled          time.Time  `json:"last_polled,omitempty"`
	LastETag            string     `json:"last_etag,omitempty"`
	LastModified        string     `json:"last_modified,omitempty"`
	Status              string     `json:"status"`
}

type Item struct {
	ID         string    `json:"id"`
	SourceID   string    `json:"source_id"`
	OriginalID string    `json:"original_id"`
	Title      string    `json:"title"`
	Body       string    `json:"body"`
	URL        string    `json:"url"`
	Published  time.Time `json:"published"`
	FetchedAt  time.Time `json:"fetched_at"`
	Verticals  []string  `json:"verticals"`
	RawJSON    string    `json:"raw_json,omitempty"`
	Language   string    `json:"language,omitempty"`
}

type Registry struct {
	UserAgent string   `json:"user_agent"`
	Sources   []Source `json:"sources"`
}
