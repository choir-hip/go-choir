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

type SourceMaxxResponse struct {
	Provider           string              `json:"provider"`
	Cycle              CycleSummary        `json:"cycle"`
	ProcessorRequests  []ProcessorRequest  `json:"processor_requests"`
	ReconcilerRequests []ReconcilerRequest `json:"reconciler_requests"`
	Metadata           SourceMaxxMetadata  `json:"metadata,omitempty"`
}

type CycleSummary struct {
	CycleID    string `json:"cycle_id"`
	StartedAt  string `json:"started_at,omitempty"`
	EndedAt    string `json:"ended_at,omitempty"`
	Status     string `json:"status"`
	ItemCount  int    `json:"item_count"`
	FetchCount int    `json:"fetch_count"`
	Error      string `json:"error,omitempty"`
}

type ProcessorRequest struct {
	RequestID     string   `json:"request_id"`
	CycleID       string   `json:"cycle_id"`
	ProcessorKey  string   `json:"processor_key"`
	Status        string   `json:"status"`
	RuntimeRunID  string   `json:"runtime_run_id,omitempty"`
	SourceItemIDs []string `json:"source_item_ids"`
	SourceCount   int      `json:"source_count"`
	SourceTypes   []string `json:"source_types,omitempty"`
	Verticals     []string `json:"verticals,omitempty"`
	Regions       []string `json:"regions,omitempty"`
	ContinuityRef string   `json:"continuity_ref,omitempty"`
	Prompt        string   `json:"prompt,omitempty"`
	CreatedAt     string   `json:"created_at,omitempty"`
	UpdatedAt     string   `json:"updated_at,omitempty"`
}

type ReconcilerRequest struct {
	RequestID           string   `json:"request_id"`
	CycleID             string   `json:"cycle_id"`
	Status              string   `json:"status"`
	RuntimeRunID        string   `json:"runtime_run_id,omitempty"`
	Scope               string   `json:"scope"`
	SourceItemIDs       []string `json:"source_item_ids"`
	ProcessorRequestIDs []string `json:"processor_request_ids"`
	Prompt              string   `json:"prompt,omitempty"`
	CreatedAt           string   `json:"created_at,omitempty"`
	UpdatedAt           string   `json:"updated_at,omitempty"`
}

type SourceMaxxMetadata struct {
	Topology      string `json:"topology,omitempty"`
	AuthorityRule string `json:"authority_rule,omitempty"`
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
