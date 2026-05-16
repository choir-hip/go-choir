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

type PublishedPage struct {
	PublicationID        string
	PublicationVersionID string
	Title                string
	Content              string
	ContentHash          string
	SourceRevisionHash   string
	PublishedAt          time.Time
}

type publishedRouteRecord struct {
	PublicationID        string
	PublicationVersionID string
	Title                string
	ContentHash          string
	SourceRevisionHash   string
	StorageRef           string
	PublishedAt          time.Time
}
