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
	ID           string `json:"id"`
	ContentID    string `json:"content_id,omitempty"`
	Title        string `json:"title"`
	Standing     string `json:"standing"`
	Role         string `json:"role"`
	CanonicalURL string `json:"canonical_url,omitempty"`
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
	ID                  string                   `json:"id"`
	OwnerID             string                   `json:"owner_id,omitempty"`
	Headline            string                   `json:"headline"`
	Dek                 string                   `json:"dek"`
	Freshness           string                   `json:"freshness"`
	Prominence          int                      `json:"prominence"`
	Tension             string                   `json:"tension"`
	ChangeState         string                   `json:"changeState"`
	NodeTone            string                   `json:"nodeTone"`
	Related             []string                 `json:"related"`
	Manifest            GlobalWireSourceManifest `json:"manifest"`
	Claims              []string                 `json:"claims"`
	Projections         map[string]string        `json:"projections"`
	ProjectionVTextDocs map[string]string        `json:"projection_vtext_docs,omitempty"`
	StyleSources        []GlobalWireStyleSource  `json:"style_sources,omitempty"`
	StoryVTextDoc       string                   `json:"story_vtext_doc_id,omitempty"`
	SourceState         string                   `json:"source_state"`
	CreatedAt           time.Time                `json:"created_at,omitempty"`
	UpdatedAt           time.Time                `json:"updated_at,omitempty"`
}

// GlobalWireStoryProjection is the durable relation:
// StoryGraph + Style.vtext + context -> Story VText.
type GlobalWireStoryProjection struct {
	ID          string    `json:"id"`
	OwnerID     string    `json:"owner_id,omitempty"`
	StoryID     string    `json:"story_id"`
	StyleID     string    `json:"style_id"`
	StyleDocID  string    `json:"style_doc_id,omitempty"`
	StoryDocID  string    `json:"story_vtext_doc_id"`
	ContextJSON string    `json:"context_json,omitempty"`
	Text        string    `json:"text"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
}

// GlobalWireContribution is a user-owned contribution queued for research and
// reconciliation. It never mutates the platform story directly.
type GlobalWireContribution struct {
	ID              string    `json:"id"`
	OwnerID         string    `json:"owner_id,omitempty"`
	StoryID         string    `json:"storyId"`
	Kind            string    `json:"kind"`
	Headline        string    `json:"headline"`
	Text            string    `json:"text"`
	SourceContentID string    `json:"source_content_id,omitempty"`
	UserVTextDocID  string    `json:"user_vtext_doc_id,omitempty"`
	ResearchState   string    `json:"research_state"`
	CreatedAt       time.Time `json:"created_at,omitempty"`
	UpdatedAt       time.Time `json:"updated_at,omitempty"`
}

// GlobalWireReconciliationDecision records a reviewer/researcher decision over
// a user-owned contribution. It is a decision artifact, not a platform story
// mutation.
type GlobalWireReconciliationDecision struct {
	ID              string    `json:"id"`
	OwnerID         string    `json:"owner_id,omitempty"`
	ContributionID  string    `json:"contribution_id"`
	StoryID         string    `json:"story_id"`
	Decision        string    `json:"decision"`
	Note            string    `json:"note"`
	SourceContentID string    `json:"source_content_id,omitempty"`
	CreatedAt       time.Time `json:"created_at,omitempty"`
}

// GlobalWireGraphUpdateCandidate is a non-mutating proposal produced from an
// accepted reconciliation decision. It bridges accepted evidence into
// StoryGraph source-neighborhood semantics without rewriting the platform story.
type GlobalWireGraphUpdateCandidate struct {
	ID               string    `json:"id"`
	OwnerID          string    `json:"owner_id,omitempty"`
	StoryID          string    `json:"story_id"`
	ContributionID   string    `json:"contribution_id"`
	DecisionID       string    `json:"decision_id"`
	SourceContentID  string    `json:"source_content_id,omitempty"`
	CandidateKind    string    `json:"candidate_kind"`
	Title            string    `json:"title"`
	Summary          string    `json:"summary"`
	SourceTier       string    `json:"source_tier"`
	EdgeKind         string    `json:"edge_kind"`
	ProjectionAction string    `json:"projection_action"`
	Status           string    `json:"status"`
	Rationale        string    `json:"rationale"`
	CreatedAt        time.Time `json:"created_at,omitempty"`
	UpdatedAt        time.Time `json:"updated_at,omitempty"`
}

// GlobalWireGraphPromotionDecision records the explicit platform review step
// over a graph-update candidate. Promotion may apply a bounded StoryGraph
// manifest change; rejection records review state only.
type GlobalWireGraphPromotionDecision struct {
	ID              string    `json:"id"`
	OwnerID         string    `json:"owner_id,omitempty"`
	CandidateID     string    `json:"candidate_id"`
	StoryID         string    `json:"story_id"`
	Decision        string    `json:"decision"`
	Note            string    `json:"note"`
	AppliedChange   string    `json:"applied_change"`
	SourceContentID string    `json:"source_content_id,omitempty"`
	CreatedAt       time.Time `json:"created_at,omitempty"`
}

// GlobalWireSourceRefreshRun records a bounded source-ingestion/classification
// pass for one StoryGraph node. It may create review artifacts, but never
// mutates the StoryGraph directly.
type GlobalWireSourceRefreshRun struct {
	ID              string    `json:"id"`
	OwnerID         string    `json:"owner_id,omitempty"`
	StoryID         string    `json:"story_id"`
	Query           string    `json:"query"`
	Status          string    `json:"status"`
	Provider        string    `json:"provider"`
	Message         string    `json:"message"`
	SourceContentID string    `json:"source_content_id,omitempty"`
	ContributionID  string    `json:"contribution_id,omitempty"`
	DecisionID      string    `json:"decision_id,omitempty"`
	CandidateID     string    `json:"candidate_id,omitempty"`
	CreatedAt       time.Time `json:"created_at,omitempty"`
	UpdatedAt       time.Time `json:"updated_at,omitempty"`
}

// GlobalWireProjectionReview records that a StoryGraph change may require a
// Style.vtext projection to be reviewed or revised.
type GlobalWireProjectionReview struct {
	ID               string    `json:"id"`
	OwnerID          string    `json:"owner_id,omitempty"`
	StoryID          string    `json:"story_id"`
	CandidateID      string    `json:"candidate_id"`
	PromotionID      string    `json:"promotion_id"`
	SourceContentID  string    `json:"source_content_id,omitempty"`
	StyleID          string    `json:"style_id"`
	StyleDocID       string    `json:"style_doc_id,omitempty"`
	StyleTitle       string    `json:"style_title"`
	ProjectionAction string    `json:"projection_action"`
	Status           string    `json:"status"`
	Rationale        string    `json:"rationale"`
	CreatedAt        time.Time `json:"created_at,omitempty"`
	UpdatedAt        time.Time `json:"updated_at,omitempty"`
}
