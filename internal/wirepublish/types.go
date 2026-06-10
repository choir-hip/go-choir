package wirepublish

import "encoding/json"

// PublishVTextRequest is the platformd internal publish payload shape used by
// autonomous Wire publication. Kept in wirepublish so sandbox runtime does not
// import internal/platform.
type PublishVTextRequest struct {
	OwnerID          string          `json:"owner_id"`
	SourceDocID      string          `json:"source_doc_id"`
	SourceRevisionID string          `json:"source_revision_id"`
	Title            string          `json:"title"`
	Content          string          `json:"content"`
	Citations        json.RawMessage `json:"citations,omitempty"`
	Metadata         json.RawMessage `json:"metadata,omitempty"`
	Slug             string          `json:"slug,omitempty"`
	AccessPolicy     json.RawMessage `json:"access_policy,omitempty"`
	ExportPolicy     json.RawMessage `json:"export_policy,omitempty"`
	SourceTraceID    string          `json:"source_trace_id,omitempty"`
	RequestedBy      string          `json:"requested_by,omitempty"`
}

// PublishVTextResponse is the platformd publish response subset persisted on
// wire article revisions for staging evidence.
type PublishVTextResponse struct {
	PublicationID        string `json:"publication_id"`
	PublicationVersionID string `json:"publication_version_id"`
	RoutePath            string `json:"route_path"`
	SourceRevisionHash   string `json:"source_revision_hash"`
}
