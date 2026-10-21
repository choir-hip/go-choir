package platform

import (
	"encoding/json"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

type PublishTextureRequest struct {
	OwnerID          string          `json:"owner_id"`
	SourceDocID      string          `json:"source_doc_id"`
	SourceRevisionID string          `json:"source_revision_id"`
	Title            string          `json:"title"`
	Content          string          `json:"content"`
	BodyDoc          json.RawMessage `json:"body_doc,omitempty"`
	SourceEntities   json.RawMessage `json:"source_entities,omitempty"`
	Citations        json.RawMessage `json:"citations,omitempty"`
	Metadata         json.RawMessage `json:"metadata,omitempty"`
	Slug             string          `json:"slug,omitempty"`
	AccessPolicy     json.RawMessage `json:"access_policy,omitempty"`
	ExportPolicy     json.RawMessage `json:"export_policy,omitempty"`
	SourceTraceID    string          `json:"source_trace_id,omitempty"`
	RequestedBy      string          `json:"requested_by,omitempty"`
	// RevisionNumber is the source revision's number on its Texture's
	// revision chain, counted from 1 (types.RevisionNumber): the published
	// version is v<RevisionNumber>. Required.
	RevisionNumber int `json:"revision_number"`
	// RevisionHash is the source revision's chain hash
	// (types.ComputeStructuredRevisionHash), recorded in the manifest.
	RevisionHash string `json:"revision_hash,omitempty"`
	// PublishMode is how the publication came to be: PublishModeAuto from a
	// computer's story publish route, PublishModeOwner from the owner's
	// publish action. The proxy sets it from the route, never from a body.
	// The first version's mode is recorded on the publication and kept;
	// only auto publications are Autopaper stories. Empty means owner.
	PublishMode string `json:"publish_mode,omitempty"`
	// AuthorLabel is an opinion piece's byline, a short handle the proxy
	// derives from the signed-in session (never an email). It is required
	// for PublishModeOpinion and fixed by the publication's first version.
	AuthorLabel string `json:"author_label,omitempty"`
}

// Publish modes (PublishTextureRequest.PublishMode).
const (
	PublishModeAuto  = "auto"
	PublishModeOwner = "owner"
	// PublishModeOpinion is a signed-in user's labelled opinion piece, listed
	// in the Opinion section; it is never an Autopaper story.
	PublishModeOpinion = "opinion"
)

type PublishTextureResponse struct {
	PublicationID        string   `json:"publication_id"`
	ProposalID           string   `json:"proposal_id"`
	PublicationVersionID string   `json:"publication_version_id"`
	ArtifactManifestID   string   `json:"artifact_manifest_id"`
	ContentHash          string   `json:"content_hash"`
	SourceRevisionHash   string   `json:"source_revision_hash"`
	ProjectionHash       string   `json:"projection_hash"`
	RoutePath            string   `json:"route_path"`
	PublicURL            string   `json:"public_url,omitempty"`
	RetrievalSourceID    string   `json:"retrieval_source_id"`
	RetrievalSpanIDs     []string `json:"retrieval_span_ids"`
	CitationIDs          []string `json:"citation_ids"`
	ConsentID            string   `json:"consent_id"`
	ReviewID             string   `json:"review_id"`
	RollbackID           string   `json:"rollback_id"`
	State                string   `json:"state"`
	// Revision is the published version number, n in /v<n>.
	Revision int `json:"revision"`
	// RoutePath is the bare link, which serves the latest version;
	// PermalinkPath is this version's permanent link, RoutePath + "/v<n>".
	PermalinkPath        string `json:"permalink_path"`
	PermalinkURL         string `json:"permalink_url,omitempty"`
	ManifestHash         string `json:"manifest_hash"`
	PreviousManifestHash string `json:"previous_manifest_hash,omitempty"`
	SupersedesVersionID  string `json:"supersedes_version_id,omitempty"`
	// Replayed is true when the revision was already published: the
	// response names the existing version and nothing was written.
	Replayed bool `json:"replayed,omitempty"`
}

type PublicationRoute struct {
	// Path is the route as requested: the bare link or a /v<n> permalink.
	Path  string `json:"path"`
	State string `json:"state"`
	// Bare is the publication's link, which serves the latest version.
	Bare string `json:"bare"`
	// Permalink is the served version's permanent link, Bare + "/v<n>".
	Permalink string `json:"permalink"`
}

type PublicationSummary struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Slug  string `json:"slug"`
	State string `json:"state"`
	// Kind is "opinion" for an opinion piece, whose page shows the Opinion
	// label and AuthorLabel in place of the AI disclosure; empty otherwise.
	Kind        string `json:"kind,omitempty"`
	AuthorLabel string `json:"author_label,omitempty"`
}

type PublicationVersionSummary struct {
	ID                   string    `json:"id"`
	Revision             int       `json:"revision"`
	ContentHash          string    `json:"content_hash"`
	SourceRevisionHash   string    `json:"source_revision_hash"`
	ProjectionHash       string    `json:"projection_hash"`
	ManifestHash         string    `json:"manifest_hash"`
	PreviousManifestHash string    `json:"previous_manifest_hash,omitempty"`
	SupersedesVersionID  string    `json:"supersedes_version_id,omitempty"`
	PublishedAt          time.Time `json:"published_at"`
}

type PublicationArtifact struct {
	ManifestID     string          `json:"manifest_id"`
	MediaType      string          `json:"media_type"`
	Content        string          `json:"content"`
	BodyDoc        json.RawMessage `json:"body_doc,omitempty"`
	SourceEntities json.RawMessage `json:"source_entities,omitempty"`
	RenderModel    []RenderBlock   `json:"render_model"`
}

type PublicationSourceEntity struct {
	ID             string          `json:"id"`
	SourceEntityID string          `json:"source_entity_id"`
	Kind           string          `json:"kind"`
	TargetKind     string          `json:"target_kind"`
	TargetID       string          `json:"target_id,omitempty"`
	DisplayPolicy  string          `json:"display_policy"`
	OpenSurface    string          `json:"open_surface,omitempty"`
	Entity         json.RawMessage `json:"entity"`
}

type PublicationTransclusion struct {
	ID             string `json:"id"`
	SourceEntityID string `json:"source_entity_id"`
	// AnchorID is the source_ref node a Write citation or embed sits on;
	// Ref is the block it cites. A row with an AnchorID is one anchor of the
	// version; one without is a source entity's legacy transclusion.
	AnchorID           string          `json:"anchor_id,omitempty"`
	Ref                *types.Ref      `json:"ref,omitempty"`
	HostSelector       json.RawMessage `json:"host_selector"`
	SourceSelector     json.RawMessage `json:"source_selector"`
	RelationType       string          `json:"relation_type"`
	DefaultDisplayMode string          `json:"default_display_mode"`
	SnapshotText       string          `json:"snapshot_text,omitempty"`
	ContentHash        string          `json:"content_hash,omitempty"`
	AccessPolicy       json.RawMessage `json:"access_policy"`
	ExportPolicy       json.RawMessage `json:"export_policy"`
}

type PublicationPolicy struct {
	Access json.RawMessage `json:"access"`
	Export json.RawMessage `json:"export"`
}

type RenderBlock struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Text     string `json:"text"`
	Start    int    `json:"start"`
	End      int    `json:"end"`
	SpanID   string `json:"span_id,omitempty"`
	TextHash string `json:"text_hash,omitempty"`
}

type RetrievalSpan struct {
	ID              string          `json:"id"`
	SourceID        string          `json:"source_id"`
	SourceVersionID string          `json:"source_version_id"`
	SelectorKind    string          `json:"selector_kind"`
	Selector        json.RawMessage `json:"selector"`
	TextHash        string          `json:"text_hash"`
	ChunkHash       string          `json:"chunk_hash"`
	TokenCount      int64           `json:"token_count"`
	Snippet         string          `json:"snippet,omitempty"`
}

type RetrievalBundle struct {
	SourceID string          `json:"source_id"`
	Spans    []RetrievalSpan `json:"spans"`
}

type CitationEdge struct {
	ID           string          `json:"id"`
	FromKind     string          `json:"from_kind"`
	FromID       string          `json:"from_id"`
	FromSelector json.RawMessage `json:"from_selector"`
	ToKind       string          `json:"to_kind"`
	ToID         string          `json:"to_id"`
	ToSelector   json.RawMessage `json:"to_selector"`
	RelationType string          `json:"relation_type"`
	State        string          `json:"state"`
	ProposedBy   string          `json:"proposed_by,omitempty"`
	AcceptedBy   string          `json:"accepted_by,omitempty"`
	EvidenceRef  string          `json:"evidence_ref,omitempty"`
	Confidence   float64         `json:"confidence"`
}

type PublicationProposalCapability struct {
	CanSubmit           bool   `json:"can_submit"`
	SourcePublicationID string `json:"source_publication_id"`
}

type PublicationProvenanceSummary struct {
	ConsentIDs     []string `json:"consent_ids"`
	ReviewIDs      []string `json:"review_ids"`
	AttestationIDs []string `json:"attestation_ids"`
}

type PublicationBundle struct {
	Route          PublicationRoute              `json:"route"`
	Publication    PublicationSummary            `json:"publication"`
	Version        PublicationVersionSummary     `json:"version"`
	Artifact       PublicationArtifact           `json:"artifact"`
	Retrieval      RetrievalBundle               `json:"retrieval"`
	Citations      []CitationEdge                `json:"citations"`
	SourceEntities []PublicationSourceEntity     `json:"source_entities"`
	Transclusions  []PublicationTransclusion     `json:"transclusions"`
	Policy         PublicationPolicy             `json:"policy"`
	Proposals      PublicationProposalCapability `json:"proposals"`
	Provenance     PublicationProvenanceSummary  `json:"provenance"`
	// Payload is the version's immutable payload: fixed forever once
	// published. Nav and Checks are live and served beside it.
	Payload PublicationVersionPayload `json:"payload"`
	Nav     PublicationNav            `json:"nav"`
	Checks  PublicationCheckOverlay   `json:"checks"`
}

type PublicationExport struct {
	RoutePath            string          `json:"route_path"`
	PublicationID        string          `json:"publication_id"`
	PublicationVersionID string          `json:"publication_version_id"`
	Format               string          `json:"format"`
	MediaType            string          `json:"media_type"`
	Filename             string          `json:"filename"`
	Content              string          `json:"content"`
	ContentBase64        string          `json:"content_base64,omitempty"`
	ContentHash          string          `json:"content_hash"`
	Metadata             json.RawMessage `json:"metadata,omitempty"`
}

type RetrievalSearchResult struct {
	PublicationID        string `json:"publication_id"`
	PublicationVersionID string `json:"publication_version_id"`
	Title                string `json:"title"`
	RoutePath            string `json:"route_path"`
	SourceID             string `json:"source_id"`
	SpanID               string `json:"span_id"`
	ContentHash          string `json:"content_hash"`
	SourceRevisionHash   string `json:"source_revision_hash"`
	Snippet              string `json:"snippet"`
	Score                int    `json:"score"`
}

type RetrievalSearchResponse struct {
	Query   string                  `json:"query"`
	Results []RetrievalSearchResult `json:"results"`
}

type TransclusionRef struct {
	SourceKind           string          `json:"source_kind"`
	PublicationID        string          `json:"publication_id"`
	PublicationVersionID string          `json:"publication_version_id"`
	SpanID               string          `json:"span_id"`
	ContentHash          string          `json:"content_hash"`
	Selector             json.RawMessage `json:"selector,omitempty"`
	SnapshotText         string          `json:"snapshot_text"`
}

type SubmitPublicationProposalRequest struct {
	PublicationID        string            `json:"publication_id,omitempty"`
	PublicationVersionID string            `json:"publication_version_id,omitempty"`
	SubmitterID          string            `json:"submitter_id"`
	SubmitterDocID       string            `json:"submitter_doc_id"`
	SubmitterRevisionID  string            `json:"submitter_revision_id"`
	Title                string            `json:"title"`
	Content              string            `json:"content"`
	Transclusions        []TransclusionRef `json:"transclusions,omitempty"`
	Citations            json.RawMessage   `json:"citations,omitempty"`
	RequestedBy          string            `json:"requested_by,omitempty"`
}

type SubmitPublicationProposalResponse struct {
	ProposalID           string   `json:"proposal_id"`
	PublicationID        string   `json:"publication_id"`
	PublicationVersionID string   `json:"publication_version_id"`
	SourceOwnerID        string   `json:"source_owner_id"`
	SubmitterID          string   `json:"submitter_id"`
	ContentHash          string   `json:"content_hash"`
	ProposalRevisionHash string   `json:"proposal_revision_hash"`
	ArtifactManifestID   string   `json:"artifact_manifest_id"`
	TransclusionIDs      []string `json:"transclusion_ids"`
	CitationIDs          []string `json:"citation_ids"`
	DeliveryID           string   `json:"delivery_id"`
	DeliveryState        string   `json:"delivery_state"`
	State                string   `json:"state"`
}

type UpdateProposalDeliveryStateRequest struct {
	ProposalID    string `json:"proposal_id"`
	DeliveryID    string `json:"delivery_id"`
	DeliveryState string `json:"delivery_state"`
	DeliveryRef   string `json:"delivery_ref,omitempty"`
	RecordedBy    string `json:"recorded_by,omitempty"`
}

type UpdateProposalDeliveryStateResponse struct {
	ProposalID    string `json:"proposal_id"`
	DeliveryID    string `json:"delivery_id"`
	DeliveryState string `json:"delivery_state"`
}

type SyncTextureDocumentRequest struct {
	DocID     string                `json:"doc_id"`
	OwnerID   string                `json:"owner_id"`
	Title     string                `json:"title"`
	Revisions []SyncTextureRevision `json:"revisions"`
}

type SyncTextureRevision struct {
	RevisionID       string          `json:"revision_id"`
	ParentRevisionID string          `json:"parent_revision_id,omitempty"`
	AuthorKind       string          `json:"author_kind,omitempty"`
	AuthorLabel      string          `json:"author_label,omitempty"`
	Content          string          `json:"content"`
	BodyDoc          json.RawMessage `json:"body_doc,omitempty"`
	SourceEntities   json.RawMessage `json:"source_entities,omitempty"`
	Citations        json.RawMessage `json:"citations,omitempty"`
	Metadata         json.RawMessage `json:"metadata,omitempty"`
	CreatedAt        time.Time       `json:"created_at,omitempty"`
}

type SyncTextureDocumentResponse struct {
	DocID         string `json:"doc_id"`
	RevisionCount int    `json:"revision_count"`
}

type PlatformTextureDocument struct {
	DocID             string `json:"doc_id"`
	OwnerID           string `json:"owner_id"`
	Title             string `json:"title"`
	CurrentRevisionID string `json:"current_revision_id,omitempty"`
}

type PlatformTextureRevision struct {
	RevisionID       string          `json:"revision_id"`
	DocID            string          `json:"doc_id"`
	OwnerID          string          `json:"owner_id"`
	ParentRevisionID string          `json:"parent_revision_id,omitempty"`
	AuthorKind       string          `json:"author_kind,omitempty"`
	AuthorLabel      string          `json:"author_label,omitempty"`
	Content          string          `json:"content"`
	BodyDoc          json.RawMessage `json:"body_doc,omitempty"`
	SourceEntities   json.RawMessage `json:"source_entities,omitempty"`
	Citations        json.RawMessage `json:"citations"`
	Metadata         json.RawMessage `json:"metadata"`
	CreatedAt        time.Time       `json:"created_at"`
}

type PlatformTextureRevisionListResponse struct {
	Revisions []PlatformTextureRevision `json:"revisions"`
}
