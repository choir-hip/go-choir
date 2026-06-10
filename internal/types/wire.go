package types

import "time"

// WireStyleSource is a selectable Style.vtext artifact for article voice/structure.
type WireStyleSource struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Label      string `json:"label"`
	Summary    string `json:"summary"`
	SourcePath string `json:"sourcePath"`
	DocID      string `json:"doc_id,omitempty"`
}

// WireSourceItem is a source handle surfaced in a Wire story projection.
type WireSourceItem struct {
	ID           string `json:"id"`
	ContentID    string `json:"content_id,omitempty"`
	Title        string `json:"title"`
	Standing     string `json:"standing"`
	Role         string `json:"role"`
	SourceID     string `json:"source_id,omitempty"`
	FetchID      string `json:"fetch_id,omitempty"`
	CanonicalURL string `json:"canonical_url,omitempty"`
}

// WireSourceManifest groups source handles by editorial role for the Wire app.
type WireSourceManifest struct {
	Lead       []WireSourceItem `json:"lead"`
	Supporting []WireSourceItem `json:"supporting"`
	Contrary   []WireSourceItem `json:"contrary"`
	Context    []WireSourceItem `json:"context"`
}

// WireStory is the Wire app projection of a VText article head (edition index).
type WireStory struct {
	ID                  string           `json:"id"`
	OwnerID             string           `json:"owner_id,omitempty"`
	Headline            string           `json:"headline"`
	Dek                 string           `json:"dek"`
	Freshness           string           `json:"freshness"`
	Prominence          int              `json:"prominence"`
	Tension             string           `json:"tension"`
	ChangeState         string           `json:"changeState"`
	NodeTone            string           `json:"nodeTone"`
	Related             []string         `json:"related"`
	Manifest            WireSourceManifest `json:"manifest"`
	Claims              []string         `json:"claims"`
	Projections         map[string]string `json:"projections"`
	ProjectionVTextDocs map[string]string `json:"projection_vtext_docs,omitempty"`
	StyleSources        []WireStyleSource `json:"style_sources,omitempty"`
	StoryVTextDoc       string           `json:"story_vtext_doc_id,omitempty"`
	VTextContent        string           `json:"vtext_content,omitempty"`
	SourceState         string           `json:"source_state"`
	CreatedAt           time.Time        `json:"created_at,omitempty"`
	UpdatedAt           time.Time        `json:"updated_at,omitempty"`
}
