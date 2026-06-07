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
	ID                   string    `json:"id"`
	OwnerID              string    `json:"owner_id,omitempty"`
	StoryID              string    `json:"story_id"`
	Query                string    `json:"query"`
	Status               string    `json:"status"`
	Provider             string    `json:"provider"`
	Message              string    `json:"message"`
	UpdateClassification string    `json:"update_classification,omitempty"`
	StoryGraphAction     string    `json:"storygraph_action,omitempty"`
	ProjectionAction     string    `json:"projection_action,omitempty"`
	SourceContentID      string    `json:"source_content_id,omitempty"`
	ContributionID       string    `json:"contribution_id,omitempty"`
	DecisionID           string    `json:"decision_id,omitempty"`
	CandidateID          string    `json:"candidate_id,omitempty"`
	CreatedAt            time.Time `json:"created_at,omitempty"`
	UpdatedAt            time.Time `json:"updated_at,omitempty"`
}

// GlobalWireClaimRecord is a structured, provisional claim/dispute/evidence-gap
// artifact tied to refresh and reconciliation evidence. It is not an oracle
// verdict and does not mutate the platform StoryGraph by itself.
type GlobalWireClaimRecord struct {
	ID                   string    `json:"id"`
	OwnerID              string    `json:"owner_id,omitempty"`
	StoryID              string    `json:"story_id"`
	RefreshID            string    `json:"refresh_id,omitempty"`
	SourceContentID      string    `json:"source_content_id,omitempty"`
	ContributionID       string    `json:"contribution_id,omitempty"`
	DecisionID           string    `json:"decision_id,omitempty"`
	CandidateID          string    `json:"candidate_id,omitempty"`
	ClaimText            string    `json:"claim_text"`
	ClaimKind            string    `json:"claim_kind"`
	UncertaintyState     string    `json:"uncertainty_state"`
	DisputeState         string    `json:"dispute_state"`
	EvidenceGap          string    `json:"evidence_gap"`
	SourceStanding       string    `json:"source_standing"`
	UpdateClassification string    `json:"update_classification"`
	Status               string    `json:"status"`
	CreatedAt            time.Time `json:"created_at,omitempty"`
	UpdatedAt            time.Time `json:"updated_at,omitempty"`
}

// GlobalWireResearchTask is the reviewer-facing follow-up generated from a
// provisional claim/update classification.
type GlobalWireResearchTask struct {
	ID                   string    `json:"id"`
	OwnerID              string    `json:"owner_id,omitempty"`
	StoryID              string    `json:"story_id"`
	ClaimID              string    `json:"claim_id,omitempty"`
	RefreshID            string    `json:"refresh_id,omitempty"`
	SourceContentID      string    `json:"source_content_id,omitempty"`
	ContributionID       string    `json:"contribution_id,omitempty"`
	CandidateID          string    `json:"candidate_id,omitempty"`
	TaskKind             string    `json:"task_kind"`
	Prompt               string    `json:"prompt"`
	Status               string    `json:"status"`
	Priority             string    `json:"priority"`
	UpdateClassification string    `json:"update_classification"`
	CreatedAt            time.Time `json:"created_at,omitempty"`
	UpdatedAt            time.Time `json:"updated_at,omitempty"`
}

// GlobalWireExtractionArtifact is a provisional source/claim overlay with
// entities, events, and timeline points. It enriches review; it does not
// replace Story VText headline graph nodes.
type GlobalWireExtractionArtifact struct {
	ID              string    `json:"id"`
	OwnerID         string    `json:"owner_id,omitempty"`
	StoryID         string    `json:"story_id"`
	ClaimID         string    `json:"claim_id,omitempty"`
	RefreshID       string    `json:"refresh_id,omitempty"`
	SourceContentID string    `json:"source_content_id,omitempty"`
	CandidateID     string    `json:"candidate_id,omitempty"`
	Entities        []string  `json:"entities"`
	Events          []string  `json:"events"`
	Timeline        []string  `json:"timeline"`
	Uncertainty     string    `json:"uncertainty"`
	Rationale       string    `json:"rationale"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"created_at,omitempty"`
	UpdatedAt       time.Time `json:"updated_at,omitempty"`
}

// GlobalWireResearchTaskEvidence is the researcher-produced packet attached to
// a task lifecycle transition. It is reconciliation evidence, not a StoryGraph
// mutation.
type GlobalWireResearchTaskEvidence struct {
	ID              string    `json:"id"`
	OwnerID         string    `json:"owner_id,omitempty"`
	TaskID          string    `json:"task_id"`
	StoryID         string    `json:"story_id"`
	ClaimID         string    `json:"claim_id,omitempty"`
	SourceContentID string    `json:"source_content_id,omitempty"`
	Status          string    `json:"status"`
	EvidenceLevel   string    `json:"evidence_level"`
	Summary         string    `json:"summary"`
	ReviewerNote    string    `json:"reviewer_note,omitempty"`
	CreatedAt       time.Time `json:"created_at,omitempty"`
}

// GlobalWireResearchEvidenceDecision records a reviewer handoff decision over
// completed research evidence. It may update candidate review state, but it
// does not mutate platform StoryGraph stories.
type GlobalWireResearchEvidenceDecision struct {
	ID              string    `json:"id"`
	OwnerID         string    `json:"owner_id,omitempty"`
	EvidenceID      string    `json:"evidence_id"`
	TaskID          string    `json:"task_id"`
	StoryID         string    `json:"story_id"`
	ClaimID         string    `json:"claim_id,omitempty"`
	CandidateID     string    `json:"candidate_id,omitempty"`
	SourceContentID string    `json:"source_content_id,omitempty"`
	Decision        string    `json:"decision"`
	Note            string    `json:"note,omitempty"`
	ResultState     string    `json:"result_state"`
	CreatedAt       time.Time `json:"created_at,omitempty"`
}

// GlobalWirePublicationUpdate packages review-ready Global Wire evidence for
// publication/update-feed review. It is a queue artifact, not publication.
type GlobalWirePublicationUpdate struct {
	ID                  string    `json:"id"`
	OwnerID             string    `json:"owner_id,omitempty"`
	StoryID             string    `json:"story_id"`
	CandidateID         string    `json:"candidate_id,omitempty"`
	ResearchDecisionID  string    `json:"research_decision_id,omitempty"`
	EvidenceID          string    `json:"evidence_id,omitempty"`
	SourceContentID     string    `json:"source_content_id,omitempty"`
	ExtractionIDs       []string  `json:"extraction_ids"`
	ProjectionReviewIDs []string  `json:"projection_review_ids"`
	ProjectionStates    []string  `json:"projection_states"`
	RollbackRefs        []string  `json:"rollback_refs"`
	Status              string    `json:"status"`
	Summary             string    `json:"summary"`
	CreatedAt           time.Time `json:"created_at,omitempty"`
	UpdatedAt           time.Time `json:"updated_at,omitempty"`
}

// GlobalWirePublicationArtifact is a review-ready output artifact derived from
// a publication update package. It is citeable publication/feed material, not
// an automatic public publish or platform story mutation.
type GlobalWirePublicationArtifact struct {
	ID                  string    `json:"id"`
	OwnerID             string    `json:"owner_id,omitempty"`
	UpdateID            string    `json:"update_id"`
	StoryID             string    `json:"story_id"`
	CandidateID         string    `json:"candidate_id,omitempty"`
	StoryVTextDocID     string    `json:"story_vtext_doc_id,omitempty"`
	SourceContentID     string    `json:"source_content_id,omitempty"`
	Channel             string    `json:"channel"`
	Status              string    `json:"status"`
	Title               string    `json:"title"`
	Body                string    `json:"body"`
	StyleDocIDs         []string  `json:"style_doc_ids"`
	ProjectionReviewIDs []string  `json:"projection_review_ids"`
	ExtractionIDs       []string  `json:"extraction_ids"`
	SchedulerRunIDs     []string  `json:"scheduler_run_ids"`
	CitationRefs        []string  `json:"citation_refs"`
	RollbackRefs        []string  `json:"rollback_refs"`
	CreatedAt           time.Time `json:"created_at,omitempty"`
	UpdatedAt           time.Time `json:"updated_at,omitempty"`
}

// GlobalWirePublicationDelivery records an owner-approved artifact becoming
// available through a channel. It is delivery evidence, not a StoryGraph
// mutation or automatic public syndication.
type GlobalWirePublicationDelivery struct {
	ID            string    `json:"id"`
	OwnerID       string    `json:"owner_id,omitempty"`
	ArtifactID    string    `json:"artifact_id"`
	StoryID       string    `json:"story_id"`
	Channel       string    `json:"channel"`
	Status        string    `json:"status"`
	DeliveryRef   string    `json:"delivery_ref"`
	CitationCount int       `json:"citation_count"`
	RollbackCount int       `json:"rollback_count"`
	CitationRefs  []string  `json:"citation_refs"`
	RollbackRefs  []string  `json:"rollback_refs"`
	CreatedAt     time.Time `json:"created_at,omitempty"`
	UpdatedAt     time.Time `json:"updated_at,omitempty"`
}

// GlobalWireAutoradioScript is a durable text renderer over an approved
// publication artifact. Audio/playback may render this later; the script itself
// carries the citeable source and rollback provenance.
type GlobalWireAutoradioScript struct {
	ID              string    `json:"id"`
	OwnerID         string    `json:"owner_id,omitempty"`
	ArtifactID      string    `json:"artifact_id"`
	StoryID         string    `json:"story_id"`
	SourceContentID string    `json:"source_content_id,omitempty"`
	Status          string    `json:"status"`
	Title           string    `json:"title"`
	ScriptBody      string    `json:"script_body"`
	VoiceNotes      string    `json:"voice_notes"`
	CitationCount   int       `json:"citation_count"`
	RollbackCount   int       `json:"rollback_count"`
	CitationRefs    []string  `json:"citation_refs"`
	RollbackRefs    []string  `json:"rollback_refs"`
	CreatedAt       time.Time `json:"created_at,omitempty"`
	UpdatedAt       time.Time `json:"updated_at,omitempty"`
}

// GlobalWirePublicationDeliveryExport is a portable owner-scoped export over a
// delivered publication and optional Autoradio script. It is not a public route.
type GlobalWirePublicationDeliveryExport struct {
	ID              string    `json:"id"`
	OwnerID         string    `json:"owner_id,omitempty"`
	DeliveryID      string    `json:"delivery_id"`
	ArtifactID      string    `json:"artifact_id"`
	ScriptID        string    `json:"script_id,omitempty"`
	StoryID         string    `json:"story_id"`
	SourceContentID string    `json:"source_content_id,omitempty"`
	Format          string    `json:"format"`
	Status          string    `json:"status"`
	Title           string    `json:"title"`
	ExportBody      string    `json:"export_body"`
	CitationCount   int       `json:"citation_count"`
	RollbackCount   int       `json:"rollback_count"`
	CitationRefs    []string  `json:"citation_refs"`
	RollbackRefs    []string  `json:"rollback_refs"`
	CreatedAt       time.Time `json:"created_at,omitempty"`
	UpdatedAt       time.Time `json:"updated_at,omitempty"`
}

// GlobalWireSourceRegistryEntry records the owner-scoped source/query basis a
// fetch cycle should use for one StoryGraph neighborhood.
type GlobalWireSourceRegistryEntry struct {
	ID                      string    `json:"id"`
	OwnerID                 string    `json:"owner_id,omitempty"`
	StoryID                 string    `json:"story_id"`
	Query                   string    `json:"query"`
	SourceScope             string    `json:"source_scope"`
	Status                  string    `json:"status"`
	SourceStandingPolicy    string    `json:"source_standing_policy,omitempty"`
	SourceStandingRationale string    `json:"source_standing_rationale,omitempty"`
	CadenceSeconds          int       `json:"cadence_seconds,omitempty"`
	NextDueAt               time.Time `json:"next_due_at,omitempty"`
	LastCycleID             string    `json:"last_cycle_id,omitempty"`
	LastScheduledRunID      string    `json:"last_scheduled_run_id,omitempty"`
	CreatedAt               time.Time `json:"created_at,omitempty"`
	UpdatedAt               time.Time `json:"updated_at,omitempty"`
}

// GlobalWireFetchCycleRun records a bounded source-registry cycle. It is
// scheduler/fetch evidence, not a claim that a 24/7 worker exists.
type GlobalWireFetchCycleRun struct {
	ID               string    `json:"id"`
	OwnerID          string    `json:"owner_id,omitempty"`
	Trigger          string    `json:"trigger"`
	Status           string    `json:"status"`
	StoryIDs         []string  `json:"story_ids"`
	RegistryEntryIDs []string  `json:"registry_entry_ids"`
	RefreshRunIDs    []string  `json:"refresh_run_ids"`
	SourceContentIDs []string  `json:"source_content_ids"`
	Message          string    `json:"message"`
	CreatedAt        time.Time `json:"created_at,omitempty"`
	UpdatedAt        time.Time `json:"updated_at,omitempty"`
}

// GlobalWireSourceSchedulerRun records a scheduler-policy pass that selected
// StoryGraph headline neighborhoods for a fetch cycle. It is scheduler
// evidence, not a claim that platform stories were mutated.
type GlobalWireSourceSchedulerRun struct {
	ID               string    `json:"id"`
	OwnerID          string    `json:"owner_id,omitempty"`
	Trigger          string    `json:"trigger"`
	Status           string    `json:"status"`
	StoryIDs         []string  `json:"story_ids"`
	RegistryEntryIDs []string  `json:"registry_entry_ids"`
	FetchCycleID     string    `json:"fetch_cycle_id,omitempty"`
	StandingPolicies []string  `json:"standing_policies"`
	Message          string    `json:"message"`
	CreatedAt        time.Time `json:"created_at,omitempty"`
	UpdatedAt        time.Time `json:"updated_at,omitempty"`
}

// GlobalWireProjectionReview records that a StoryGraph change may require a
// Style.vtext projection to be reviewed or revised.
type GlobalWireProjectionReview struct {
	ID                 string    `json:"id"`
	OwnerID            string    `json:"owner_id,omitempty"`
	StoryID            string    `json:"story_id"`
	CandidateID        string    `json:"candidate_id"`
	PromotionID        string    `json:"promotion_id"`
	SourceContentID    string    `json:"source_content_id,omitempty"`
	StyleID            string    `json:"style_id"`
	StyleDocID         string    `json:"style_doc_id,omitempty"`
	StyleTitle         string    `json:"style_title"`
	ProjectionAction   string    `json:"projection_action"`
	Status             string    `json:"status"`
	Rationale          string    `json:"rationale"`
	DraftStoryDocID    string    `json:"draft_story_doc_id,omitempty"`
	ApprovedStoryDocID string    `json:"approved_story_doc_id,omitempty"`
	ApprovedRevisionID string    `json:"approved_revision_id,omitempty"`
	CreatedAt          time.Time `json:"created_at,omitempty"`
	UpdatedAt          time.Time `json:"updated_at,omitempty"`
}
