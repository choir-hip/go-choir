package types

import (
	"encoding/json"
	"time"
)

// ContentItem is the shared substrate record for uploaded, linked, extracted,
// and media content. Large binary bytes stay in the per-user filesystem; this
// row stores identity, routing, extracted text, and provenance.
type ContentItem struct {
	ContentID    string          `json:"content_id"`
	OwnerID      string          `json:"owner_id"`
	SourceType   string          `json:"source_type"`
	MediaType    string          `json:"media_type"`
	AppHint      string          `json:"app_hint"`
	Title        string          `json:"title,omitempty"`
	SourceURL    string          `json:"source_url,omitempty"`
	CanonicalURL string          `json:"canonical_url,omitempty"`
	FilePath     string          `json:"file_path,omitempty"`
	TextContent  string          `json:"text_content,omitempty"`
	ContentHash  string          `json:"content_hash,omitempty"`
	Metadata     json.RawMessage `json:"metadata,omitempty"`
	Provenance   json.RawMessage `json:"provenance,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}
