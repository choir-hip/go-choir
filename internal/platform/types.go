package platform

import (
	"encoding/json"
	"time"
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
	// History is the full source revision chain (oldest first) for the
	// published document. A Texture is its versioned history, not just the head
	// revision, so publish carries every revision's body, citations,
	// system-attributed provenance, and per-revision hash. The head revision is
	// still surfaced prominently via Content/SourceRevisionID; History is the
	// durable spine persisted alongside it.
	History []PublishTextureRevision `json:"history,omitempty"`
}

// PublishTextureRevision is one revision in the published version-history chain.
// It mirrors the runtime-attributed revision record (never model-authored) and
// carries the per-revision hash so the published artifact remains independently
// verifiable end to end.
type PublishTextureRevision struct {
	RevisionID       string          `json:"revision_id"`
	ParentRevisionID string          `json:"parent_revision_id,omitempty"`
	VersionNumber    int             `json:"version_number,omitempty"`
	AuthorKind       string          `json:"author_kind,omitempty"`
	AuthorLabel      string          `json:"author_label,omitempty"`
	Content          string          `json:"content"`
	BodyDoc          json.RawMessage `json:"body_doc,omitempty"`
	SourceEntities   json.RawMessage `json:"source_entities,omitempty"`
	Citations        json.RawMessage `json:"citations,omitempty"`
	Metadata         json.RawMessage `json:"metadata,omitempty"`
	Provenance       json.RawMessage `json:"provenance,omitempty"`
	RevisionHash     string          `json:"revision_hash,omitempty"`
	CreatedAt        string          `json:"created_at,omitempty"`
}

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
	VersionHistoryHash   string   `json:"version_history_hash,omitempty"`
	VersionCount         int      `json:"version_count,omitempty"`
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
	ID                 string          `json:"id"`
	SourceEntityID     string          `json:"source_entity_id"`
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
	VersionHistory *PublicationVersionHistory    `json:"version_history,omitempty"`
}

// PublicationVersionHistory is the published, self-contained version chain for a
// Texture: the full ordered list of revisions with per-revision provenance and
// the tamper-evident hash chain. It is read back from the persisted artifact
// manifest, not recomputed, so a reader/verifier can replay and check the chain.
type PublicationVersionHistory struct {
	Schema string `json:"schema"`
	// RevisionCount is the number of revisions in the chain.
	RevisionCount int    `json:"revision_count"`
	ChainHeadHash string `json:"chain_head_hash,omitempty"`
	ManifestHash  string `json:"manifest_hash,omitempty"`
	// SigningSchema identifies the per-revision attestation payload shape
	// (choir.platform.revision_attestation.v0). Present only when the chain is
	// platform-signed.
	SigningSchema string `json:"signing_schema,omitempty"`
	// SigningPublicKey is the platform Ed25519 public key (base64) that
	// attests every revision in this chain. A verifier checks each entry's
	// Signature against this key over the attestation of entry.RevisionHash.
	SigningPublicKey string `json:"signing_public_key,omitempty"`
	// SigningKeyID is a short content-addressed id of SigningPublicKey so
	// signatures remain identifiable across a future key rotation.
	SigningKeyID string                           `json:"signing_key_id,omitempty"`
	Revisions    []PublicationVersionHistoryEntry `json:"revisions"`
}

// PublicationVersionHistoryEntry is one revision within a published version
// history. Content/citations/metadata/provenance are carried verbatim from the
// source revision so the published artifact is self-contained.
type PublicationVersionHistoryEntry struct {
	RevisionID       string          `json:"revision_id"`
	ParentRevisionID string          `json:"parent_revision_id,omitempty"`
	VersionNumber    int             `json:"version_number,omitempty"`
	AuthorKind       string          `json:"author_kind,omitempty"`
	AuthorLabel      string          `json:"author_label,omitempty"`
	Content          string          `json:"content"`
	BodyDoc          json.RawMessage `json:"body_doc,omitempty"`
	SourceEntities   json.RawMessage `json:"source_entities,omitempty"`
	ContentHash      string          `json:"content_hash"`
	Citations        json.RawMessage `json:"citations,omitempty"`
	Metadata         json.RawMessage `json:"metadata,omitempty"`
	Provenance       json.RawMessage `json:"provenance,omitempty"`
	RevisionHash     string          `json:"revision_hash,omitempty"`
	CreatedAt        string          `json:"created_at,omitempty"`
	// Signature is the base64 Ed25519 platform signature over the canonical
	// attestation of RevisionHash. Present only for platform-signed revisions.
	Signature string `json:"signature,omitempty"`
	// SigningKeyID identifies which platform key produced Signature.
	SigningKeyID string `json:"signing_key_id,omitempty"`
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
