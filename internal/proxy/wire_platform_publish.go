package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
	"github.com/yusefmosiah/go-choir/internal/wirepublish"
)

type wirePlatformPublishRequest struct {
	DocID          string          `json:"doc_id"`
	RevisionID     string          `json:"revision_id"`
	Title          string          `json:"title,omitempty"`
	Content        string          `json:"content,omitempty"`
	BodyDoc        json.RawMessage `json:"body_doc,omitempty"`
	SourceEntities json.RawMessage `json:"source_entities,omitempty"`
	Citations      json.RawMessage `json:"citations,omitempty"`
	Metadata       json.RawMessage `json:"metadata,omitempty"`
	RunID          string          `json:"run_id,omitempty"`
	RequestIntent  string          `json:"request_intent,omitempty"`
	RunMetadata    json.RawMessage `json:"run_metadata,omitempty"`
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

	desktopID := vmctl.UniversalWirePlatformDesktopID
	sandboxURL, err := h.resolveSandboxURL(r.Context(), platformOwner, desktopID)
	if err != nil {
		log.Printf("proxy: wire publish failed to resolve sandbox for %s: %v", platformOwner, err)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to resolve platform sandbox"})
		return
	}

	var doc sandboxTextureDocument
	var rev sandboxTextureRevision
	if strings.TrimSpace(req.Title) != "" || strings.TrimSpace(req.Content) != "" || len(req.BodyDoc) > 0 || len(req.SourceEntities) > 0 || len(req.Metadata) > 0 || len(req.Citations) > 0 {
		doc = sandboxTextureDocument{DocID: req.DocID, OwnerID: platformOwner, Title: strings.TrimSpace(req.Title)}
		rev = sandboxTextureRevision{RevisionID: req.RevisionID, DocID: req.DocID, OwnerID: platformOwner, Content: req.Content, BodyDoc: req.BodyDoc, SourceEntities: req.SourceEntities, Citations: req.Citations, Metadata: req.Metadata}
	} else {
		if err := h.fetchSandboxJSON(r, sandboxURL, "/internal/texture/documents/"+url.PathEscape(req.DocID), platformOwner, &doc); err != nil {
			log.Printf("proxy: wire publish fetch document: %v", err)
			writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to load wire document"})
			return
		}
		if doc.OwnerID != platformOwner || doc.DocID != req.DocID {
			writeJSON(w, http.StatusForbidden, errorResponse{Error: "document does not belong to platform owner"})
			return
		}
		if err := h.fetchSandboxJSON(r, sandboxURL, "/internal/texture/revisions/"+url.PathEscape(req.RevisionID), platformOwner, &rev); err != nil {
			log.Printf("proxy: wire publish fetch revision: %v", err)
			writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to load wire revision"})
			return
		}
		if rev.OwnerID != platformOwner || rev.DocID != req.DocID || rev.RevisionID != req.RevisionID {
			writeJSON(w, http.StatusForbidden, errorResponse{Error: "revision does not belong to wire document"})
			return
		}
	}
	if textureSourceEntitiesRequireBodyDoc(rev.SourceEntities, rev.BodyDoc) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "source_entities require body_doc source_ref nodes"})
		return
	}

	recMeta := map[string]any{}
	if len(req.RunMetadata) > 0 {
		_ = json.Unmarshal(req.RunMetadata, &recMeta)
	}
	if strings.TrimSpace(req.RequestIntent) != "" && strings.TrimSpace(fmt.Sprint(recMeta["request_intent"])) == "" {
		recMeta["request_intent"] = strings.TrimSpace(req.RequestIntent)
	}
	rec := &types.RunRecord{
		OwnerID:  platformOwner,
		RunID:    strings.TrimSpace(req.RunID),
		Metadata: recMeta,
	}
	docType := types.Document{
		DocID:   doc.DocID,
		OwnerID: doc.OwnerID,
		Title:   doc.Title,
	}
	revType := types.Revision{
		RevisionID:     rev.RevisionID,
		DocID:          rev.DocID,
		OwnerID:        rev.OwnerID,
		Content:        rev.Content,
		BodyDoc:        rev.BodyDoc,
		SourceEntities: rev.SourceEntities,
		Citations:      rev.Citations,
		Metadata:       rev.Metadata,
	}
	if !wirepublish.EligibleForAutonomousPublish(docType, revType, rec, platformOwner) {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "revision is not eligible for autonomous wire publish"})
		return
	}

	enrichedSourceEntities, err := h.enrichTexturePublicationSourceEntities(r, sandboxURL, platformOwner, rev.SourceEntities)
	if err != nil {
		log.Printf("proxy: wire publish enrich metadata: %v", err)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to prepare publication source metadata"})
		return
	}
	revType.SourceEntities = enrichedSourceEntities

	wireReq := wirepublish.BuildAutonomousPublishRequest(docType, revType, rec, rev.Metadata)
	platformReq := platform.PublishTextureRequest{
		OwnerID:          wireReq.OwnerID,
		SourceDocID:      wireReq.SourceDocID,
		SourceRevisionID: wireReq.SourceRevisionID,
		Title:            wireReq.Title,
		Content:          wireReq.Content,
		BodyDoc:          wireReq.BodyDoc,
		SourceEntities:   wireReq.SourceEntities,
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

	// Sync the full Texture (all revisions) to platformd so published articles
	// carry their complete revision history.
	go h.syncTextureToPlatformd(r, sandboxURL, platformOwner, req.DocID, doc.Title)

	writeJSON(w, status, platformResp)
}

func textureSourceEntitiesRequireBodyDoc(sourceEntities, bodyDoc json.RawMessage) bool {
	sourceEntitiesText := strings.TrimSpace(string(sourceEntities))
	if sourceEntitiesText == "" || sourceEntitiesText == "null" || sourceEntitiesText == "[]" {
		return false
	}
	return strings.TrimSpace(string(bodyDoc)) == ""
}

// sandboxRevisionEntry matches the sandbox /api/texture/revisions list item shape.
type sandboxRevisionEntry struct {
	RevisionID       string          `json:"revision_id"`
	ParentRevisionID string          `json:"parent_revision_id,omitempty"`
	AuthorKind       string          `json:"author_kind,omitempty"`
	AuthorLabel      string          `json:"author_label,omitempty"`
	Content          string          `json:"content"`
	BodyDoc          json.RawMessage `json:"body_doc,omitempty"`
	SourceEntities   json.RawMessage `json:"source_entities,omitempty"`
	Citations        json.RawMessage `json:"citations"`
	Metadata         json.RawMessage `json:"metadata"`
}

// syncTextureToPlatformd fetches all revisions of a Texture document from the
// platform sandbox and syncs them to platformd's DoltDB. This runs
// asynchronously after a successful publication so the publish response is
// not delayed.
func (h *Handler) syncTextureToPlatformd(r *http.Request, sandboxURL, ownerID, docID, title string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var revisions []sandboxRevisionEntry
	if err := h.fetchSandboxJSONWithContext(ctx, sandboxURL, "/api/texture/documents/"+url.PathEscape(docID)+"/revisions", ownerID, &revisions); err != nil {
		log.Printf("proxy: sync texture to platformd: fetch revisions for %s: %v", docID, err)
		return
	}
	if len(revisions) == 0 {
		log.Printf("proxy: sync texture to platformd: no revisions for %s", docID)
		return
	}

	syncReq := platform.SyncTextureDocumentRequest{
		DocID:   docID,
		OwnerID: ownerID,
		Title:   title,
	}
	for _, rev := range revisions {
		syncReq.Revisions = append(syncReq.Revisions, platform.SyncTextureRevision{
			RevisionID:       rev.RevisionID,
			ParentRevisionID: rev.ParentRevisionID,
			AuthorKind:       rev.AuthorKind,
			AuthorLabel:      rev.AuthorLabel,
			Content:          rev.Content,
			BodyDoc:          rev.BodyDoc,
			SourceEntities:   rev.SourceEntities,
			Citations:        rev.Citations,
			Metadata:         rev.Metadata,
		})
	}

	target, err := joinBasePath(h.cfg.PlatformdURL, "/internal/platform/texture/sync")
	if err != nil {
		log.Printf("proxy: sync texture to platformd: build URL: %v", err)
		return
	}
	data, err := json.Marshal(syncReq)
	if err != nil {
		log.Printf("proxy: sync texture to platformd: marshal: %v", err)
		return
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, target, strings.NewReader(string(data)))
	if err != nil {
		log.Printf("proxy: sync texture to platformd: build request: %v", err)
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Internal-Caller", "true")

	resp, err := h.platformd.Do(httpReq)
	if err != nil {
		log.Printf("proxy: sync texture to platformd: call: %v", err)
		return
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("proxy: sync texture to platformd: status %d for doc %s", resp.StatusCode, docID)
		return
	}
	log.Printf("proxy: synced %d revisions for doc %s to platformd", len(revisions), docID)
}
