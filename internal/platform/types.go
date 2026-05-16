package platform

import (
	"encoding/json"
	"time"
)

type PublishVTextRequest struct {
	OwnerID          string          `json:"owner_id"`
	SourceDocID      string          `json:"source_doc_id"`
	SourceRevisionID string          `json:"source_revision_id"`
	Title            string          `json:"title"`
	Content          string          `json:"content"`
	Citations        json.RawMessage `json:"citations,omitempty"`
	Slug             string          `json:"slug,omitempty"`
	SourceTraceID    string          `json:"source_trace_id,omitempty"`
	RequestedBy      string          `json:"requested_by,omitempty"`
}

type PublishVTextResponse struct {
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
}

type PublicationRoute struct {
	Path  string `json:"path"`
	State string `json:"state"`
}

type PublicationSummary struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Slug  string `json:"slug"`
	State string `json:"state"`
}

type PublicationVersionSummary struct {
	ID                 string    `json:"id"`
	ContentHash        string    `json:"content_hash"`
	SourceRevisionHash string    `json:"source_revision_hash"`
	ProjectionHash     string    `json:"projection_hash"`
	PublishedAt        time.Time `json:"published_at"`
}

type PublicationArtifact struct {
	ManifestID  string        `json:"manifest_id"`
	MediaType   string        `json:"media_type"`
	Content     string        `json:"content"`
	RenderModel []RenderBlock `json:"render_model"`
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
	Route       PublicationRoute              `json:"route"`
	Publication PublicationSummary            `json:"publication"`
	Version     PublicationVersionSummary     `json:"version"`
	Artifact    PublicationArtifact           `json:"artifact"`
	Retrieval   RetrievalBundle               `json:"retrieval"`
	Citations   []CitationEdge                `json:"citations"`
	Proposals   PublicationProposalCapability `json:"proposals"`
	Provenance  PublicationProvenanceSummary  `json:"provenance"`
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
