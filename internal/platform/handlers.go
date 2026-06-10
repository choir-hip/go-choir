package platform

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/buildinfo"
	"github.com/yusefmosiah/go-choir/internal/server"
)

type Handler struct {
	service *Service
}

type healthResponse struct {
	Status  string         `json:"status"`
	Service string         `json:"service"`
	Store   string         `json:"store"`
	Build   buildinfo.Info `json:"build"`
}

type apiError struct {
	Error string `json:"error"`
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	status := "ok"
	storeStatus := "ok"
	if h == nil || h.service == nil || h.service.Health(r.Context()) != nil {
		status = "degraded"
		storeStatus = "unreachable"
	}
	writeJSON(w, http.StatusOK, healthResponse{
		Status:  status,
		Service: "platformd",
		Store:   storeStatus,
		Build:   buildinfo.Snapshot("platformd"),
	})
}

func (h *Handler) HandleInternalPublishVText(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if r.Header.Get("X-Internal-Caller") != "true" {
		writeJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return
	}
	var req PublishVTextRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}
	resp, err := h.service.PublishVText(r.Context(), req)
	if err != nil {
		log.Printf("platformd: publish vtext: %v", err)
		writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (h *Handler) HandlePublicVText(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusNotFound, apiError{Error: "platformd does not render public HTML"})
}

func (h *Handler) HandleInternalResolvePublication(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if r.Header.Get("X-Internal-Caller") != "true" {
		writeJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return
	}
	routePath := strings.TrimSpace(r.URL.Query().Get("route"))
	if routePath == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "route is required"})
		return
	}
	bundle, err := h.service.GetPublicationBundleByRoute(r.Context(), routePath)
	if err != nil {
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
			return
		}
		log.Printf("platformd: resolve publication %s: %v", routePath, err)
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "failed to resolve publication"})
		return
	}
	writeJSON(w, http.StatusOK, bundle)
}

func (h *Handler) HandleInternalExportPublication(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if r.Header.Get("X-Internal-Caller") != "true" {
		writeJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return
	}
	routePath := strings.TrimSpace(r.URL.Query().Get("route"))
	if routePath == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "route is required"})
		return
	}
	format := strings.TrimSpace(r.URL.Query().Get("format"))
	out, err := h.service.ExportPublicationByRoute(r.Context(), routePath, format)
	if err != nil {
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
			return
		}
		log.Printf("platformd: export publication %s: %v", routePath, err)
		writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) HandleInternalRetrievalSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if r.Header.Get("X-Internal-Caller") != "true" {
		writeJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return
	}
	resp, err := h.service.SearchPublished(r.Context(), r.URL.Query().Get("q"))
	if err != nil {
		log.Printf("platformd: retrieval search: %v", err)
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "failed to search published retrieval sources"})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) HandleInternalPublicationProposal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if r.Header.Get("X-Internal-Caller") != "true" {
		writeJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return
	}
	publicationID := publicationIDFromProposalPath(r.URL.Path)
	if publicationID == "" {
		http.NotFound(w, r)
		return
	}
	var req SubmitPublicationProposalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}
	req.PublicationID = publicationID
	resp, err := h.service.SubmitPublicationProposal(r.Context(), req)
	if err != nil {
		log.Printf("platformd: submit publication proposal: %v", err)
		writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (h *Handler) HandleInternalProposalDeliveryState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if r.Header.Get("X-Internal-Caller") != "true" {
		writeJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return
	}
	var req UpdateProposalDeliveryStateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}
	resp, err := h.service.UpdateProposalDeliveryState(r.Context(), req)
	if err != nil {
		log.Printf("platformd: update proposal delivery: %v", err)
		writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) HandleInternalSyncVTextDocument(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if r.Header.Get("X-Internal-Caller") != "true" {
		writeJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return
	}
	var req SyncVTextDocumentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}
	resp, err := h.service.SyncVTextDocument(r.Context(), req)
	if err != nil {
		log.Printf("platformd: sync vtext document: %v", err)
		writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) HandleInternalGetVTextDocument(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if r.Header.Get("X-Internal-Caller") != "true" {
		writeJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return
	}
	docID := strings.TrimPrefix(r.URL.Path, "/internal/platform/vtext/documents/")
	if docID == r.URL.Path {
		docID = strings.TrimPrefix(r.URL.Path, "/internal/platform/vtext/documents")
	}
	docID = strings.TrimSuffix(docID, "/revisions")
	docID = strings.Trim(docID, "/")
	docID = strings.TrimSpace(docID)
	if docID == "" {
		http.NotFound(w, r)
		return
	}
	doc, err := h.service.GetPlatformVTextDocument(r.Context(), docID)
	if err != nil {
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
			return
		}
		log.Printf("platformd: get vtext document %s: %v", docID, err)
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "failed to get vtext document"})
		return
	}
	writeJSON(w, http.StatusOK, doc)
}

func (h *Handler) HandleInternalListVTextRevisions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if r.Header.Get("X-Internal-Caller") != "true" {
		writeJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return
	}
	if !strings.HasSuffix(r.URL.Path, "/revisions") {
		h.HandleInternalGetVTextDocument(w, r)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/internal/platform/vtext/documents/")
	docID := strings.TrimSuffix(path, "/revisions")
	docID = strings.Trim(docID, "/")
	docID = strings.TrimSpace(docID)
	if docID == "" {
		http.NotFound(w, r)
		return
	}
	revisions, err := h.service.ListPlatformVTextRevisions(r.Context(), docID)
	if err != nil {
		log.Printf("platformd: list vtext revisions %s: %v", docID, err)
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list vtext revisions"})
		return
	}
	writeJSON(w, http.StatusOK, revisions)
}

func (h *Handler) HandleInternalGetVTextRevision(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if r.Header.Get("X-Internal-Caller") != "true" {
		writeJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return
	}
	revisionID := strings.TrimPrefix(r.URL.Path, "/internal/platform/vtext/revisions/")
	revisionID = strings.Trim(revisionID, "/")
	revisionID = strings.TrimSpace(revisionID)
	if revisionID == "" {
		http.NotFound(w, r)
		return
	}
	rev, err := h.service.GetPlatformVTextRevision(r.Context(), revisionID)
	if err != nil {
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
			return
		}
		log.Printf("platformd: get vtext revision %s: %v", revisionID, err)
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "failed to get vtext revision"})
		return
	}
	writeJSON(w, http.StatusOK, rev)
}

func RegisterRoutes(s *server.Server, h *Handler) {
	s.SetHealthHandler(h.HandleHealth)
	s.HandleFunc("/internal/platform/publications/vtext", h.HandleInternalPublishVText)
	s.HandleFunc("/internal/platform/publications/resolve", h.HandleInternalResolvePublication)
	s.HandleFunc("/internal/platform/publications/export", h.HandleInternalExportPublication)
	s.HandleFunc("/internal/platform/retrieval/search", h.HandleInternalRetrievalSearch)
	s.HandleFunc("/internal/platform/proposal-deliveries/state", h.HandleInternalProposalDeliveryState)
	s.HandleFunc("/internal/platform/publications/", h.HandleInternalPublicationProposal)
	s.HandleFunc("/internal/platform/vtext/sync", h.HandleInternalSyncVTextDocument)
	s.HandleFunc("/internal/platform/vtext/revisions/", h.HandleInternalGetVTextRevision)
	s.HandleFunc("/internal/platform/vtext/documents/", h.HandleInternalListVTextRevisions)
	s.HandleFunc("/internal/platform/vtext/documents", h.HandleInternalGetVTextDocument)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("platformd: json encode: %v", err)
	}
}

func publicationIDFromProposalPath(path string) string {
	const prefix = "/internal/platform/publications/"
	const suffix = "/proposals"
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		return ""
	}
	return strings.Trim(strings.TrimSuffix(strings.TrimPrefix(path, prefix), suffix), "/")
}
