package sourceapi

import "time"

const (
	ProviderName = "source_service_api"
	TargetKind   = "source_service_item"
)

type HealthResponse struct {
	Status     string    `json:"status"`
	ItemCount  int       `json:"item_count"`
	FetchCount int       `json:"fetch_count"`
	CheckedAt  time.Time `json:"checked_at"`
}

type SearchResponse struct {
	Query    string       `json:"query"`
	Provider string       `json:"provider"`
	Results  []ItemResult `json:"results"`
	Metadata Metadata     `json:"metadata,omitempty"`
}

type ResolveItemResponse struct {
	Provider string     `json:"provider"`
	Item     ItemResult `json:"item"`
}

type Metadata struct {
	TargetKind string `json:"target_kind,omitempty"`
}

type ItemResult struct {
	Rank            int      `json:"rank,omitempty"`
	TargetKind      string   `json:"target_kind"`
	ItemID          string   `json:"item_id"`
	SourceID        string   `json:"source_id"`
	SourceType      string   `json:"source_type,omitempty"`
	FetchID         string   `json:"fetch_id,omitempty"`
	OriginalID      string   `json:"original_id,omitempty"`
	Title           string   `json:"title,omitempty"`
	Body            string   `json:"body,omitempty"`
	URL             string   `json:"url,omitempty"`
	CanonicalURL    string   `json:"canonical_url,omitempty"`
	PublishedAt     string   `json:"published_at,omitempty"`
	FetchedAt       string   `json:"fetched_at,omitempty"`
	Verticals       []string `json:"verticals,omitempty"`
	Language        string   `json:"language,omitempty"`
	Region          string   `json:"region,omitempty"`
	ContentHash     string   `json:"content_hash,omitempty"`
	EvidenceLevel   string   `json:"evidence_level,omitempty"`
	VintagePolicy   string   `json:"vintage_policy,omitempty"`
	LookaheadStatus string   `json:"lookahead_status,omitempty"`
	ReleaseDate     string   `json:"release_date,omitempty"`
}
