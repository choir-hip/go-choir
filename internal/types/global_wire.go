package types

import "time"

// GlobalWireStyleSource is a selectable Style.vtext artifact used to project a
// StoryGraph node without changing its evidence manifest.
type GlobalWireStyleSource struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Label      string `json:"label"`
	Summary    string `json:"summary"`
	SourcePath string `json:"sourcePath"`
	DocID      string `json:"doc_id,omitempty"`
}

// GlobalWireSourceItem is a source-neighborhood entry attached to a story node.
type GlobalWireSourceItem struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Standing string `json:"standing"`
	Role     string `json:"role"`
}

// GlobalWireSourceManifest keeps lead, supporting, contrary, and ambient context
// source tiers visible to every story projection.
type GlobalWireSourceManifest struct {
	Lead       []GlobalWireSourceItem `json:"lead"`
	Supporting []GlobalWireSourceItem `json:"supporting"`
	Contrary   []GlobalWireSourceItem `json:"contrary"`
	Context    []GlobalWireSourceItem `json:"context"`
}

// GlobalWireStory is the durable StoryGraph node shape consumed by the News app.
type GlobalWireStory struct {
	ID            string                   `json:"id"`
	OwnerID       string                   `json:"owner_id,omitempty"`
	Headline      string                   `json:"headline"`
	Dek           string                   `json:"dek"`
	Freshness     string                   `json:"freshness"`
	Prominence    int                      `json:"prominence"`
	Tension       string                   `json:"tension"`
	ChangeState   string                   `json:"changeState"`
	NodeTone      string                   `json:"nodeTone"`
	Related       []string                 `json:"related"`
	Manifest      GlobalWireSourceManifest `json:"manifest"`
	Claims        []string                 `json:"claims"`
	Projections   map[string]string        `json:"projections"`
	StyleSources  []GlobalWireStyleSource  `json:"style_sources,omitempty"`
	StoryVTextDoc string                   `json:"story_vtext_doc_id,omitempty"`
	SourceState   string                   `json:"source_state"`
	CreatedAt     time.Time                `json:"created_at,omitempty"`
	UpdatedAt     time.Time                `json:"updated_at,omitempty"`
}

// GlobalWireContribution is a user-owned contribution queued for research and
// reconciliation. It never mutates the platform story directly.
type GlobalWireContribution struct {
	ID             string    `json:"id"`
	OwnerID        string    `json:"owner_id,omitempty"`
	StoryID        string    `json:"storyId"`
	Kind           string    `json:"kind"`
	Headline       string    `json:"headline"`
	Text           string    `json:"text"`
	UserVTextDocID string    `json:"user_vtext_doc_id,omitempty"`
	ResearchState  string    `json:"research_state"`
	CreatedAt      time.Time `json:"created_at,omitempty"`
	UpdatedAt      time.Time `json:"updated_at,omitempty"`
}
