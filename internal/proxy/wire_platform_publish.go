package proxy

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/wirepublish"
)

type wirePlatformPublishRequest struct {
	DocID         string `json:"doc_id"`
	RevisionID    string `json:"revision_id"`
	RunID         string `json:"run_id,omitempty"`
	RequestIntent string `json:"request_intent,omitempty"`
}

// HandleInternalWirePlatformPublish is the host-mediated choke point for autonomous
// Universal Wire publication. Platform sandboxes call this route; proxy re-reads
// the revision from the platform sandbox and forwards to platformd.
func (h *Handler) HandleInternalWirePlatformPublish(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	if r.Header.Get("X-Internal-Caller") != "true" {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "internal caller required"})
		return
	}
	platformOwner := wirepublish.PlatformOwnerID()
	userID := strings.TrimSpace(r.Header.Get("X-Authenticated-User"))
	if userID != platformOwner {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "wire publish requires platform owner"})
		return
	}

	var req wirePlatformPublishRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request body"})
		return
	}
	req.DocID = strings.TrimSpace(req.DocID)
	req.RevisionID = strings.TrimSpace(req.RevisionID)
	if req.DocID == "" || req.RevisionID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "doc_id and revision_id are required"})
		return
	}

	desktopID := requestDesktopID(r)
	sandboxURL, err := h.resolveSandboxURL(r.Context(), platformOwner, desktopID)
	if err != nil {
		log.Printf("proxy: wire publish failed to resolve sandbox for %s: %v", platformOwner, err)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to resolve platform sandbox"})
		return
	}

	var doc sandboxVTextDocument
	if err := h.fetchSandboxJSON(r, sandboxURL, "/api/vtext/documents/"+url.PathEscape(req.DocID), platformOwner, &doc); err != nil {
		log.Printf("proxy: wire publish fetch document: %v", err)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to load wire document"})
		return
	}
	if doc.OwnerID != platformOwner || doc.DocID != req.DocID {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "document does not belong to platform owner"})
		return
	}

	var rev sandboxVTextRevision
	if err := h.fetchSandboxJSON(r, sandboxURL, "/api/vtext/revisions/"+url.PathEscape(req.RevisionID), platformOwner, &rev); err != nil {
		log.Printf("proxy: wire publish fetch revision: %v", err)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to load wire revision"})
		return
	}
	if rev.OwnerID != platformOwner || rev.DocID != req.DocID || rev.RevisionID != req.RevisionID {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "revision does not belong to wire document"})
		return
	}

	rec := &types.RunRecord{
		OwnerID: platformOwner,
		RunID:   strings.TrimSpace(req.RunID),
		Metadata: map[string]any{
			"request_intent": strings.TrimSpace(req.RequestIntent),
		},
	}
	docType := types.Document{
		DocID:   doc.DocID,
		OwnerID: doc.OwnerID,
		Title:   doc.Title,
	}
	revType := types.Revision{
		RevisionID: rev.RevisionID,
		DocID:      rev.DocID,
		OwnerID:    rev.OwnerID,
		Content:    rev.Content,
		Citations:  rev.Citations,
		Metadata:   rev.Metadata,
	}
	if !wirepublish.EligibleForAutonomousPublish(docType, revType, rec, platformOwner) {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "revision is not eligible for autonomous wire publish"})
		return
	}

	enrichedMetadata, err := h.enrichVTextPublicationMetadata(r, sandboxURL, platformOwner, rev.Metadata)
	if err != nil {
		log.Printf("proxy: wire publish enrich metadata: %v", err)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to prepare publication source metadata"})
		return
	}

	wireReq := wirepublish.BuildAutonomousPublishRequest(docType, revType, rec, enrichedMetadata)
	platformReq := platform.PublishVTextRequest{
		OwnerID:          wireReq.OwnerID,
		SourceDocID:      wireReq.SourceDocID,
		SourceRevisionID: wireReq.SourceRevisionID,
		Title:            wireReq.Title,
		Content:          wireReq.Content,
		Citations:        wireReq.Citations,
		Metadata:         wireReq.Metadata,
		Slug:             wireReq.Slug,
		AccessPolicy:     wireReq.AccessPolicy,
		ExportPolicy:     wireReq.ExportPolicy,
		SourceTraceID:    wireReq.SourceTraceID,
		RequestedBy:      wireReq.RequestedBy,
	}
	platformResp, status, err := h.postPlatformPublication(r, platformReq)
	if err != nil {
		log.Printf("proxy: wire publish post platformd: %v", err)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to publish wire article"})
		return
	}
	if status < 200 || status >= 300 {
		writeJSON(w, status, platformResp)
		return
	}
	writeJSON(w, status, platformResp)
}
