package platform

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/buildinfo"
	"github.com/yusefmosiah/go-choir/internal/server"
)

type Handler struct {
	service              *Service
	eventCAS             *ComputerEventCAS
	eventArtifacts       *EventArtifactService
	eventAuth            EventCapabilityAuthorizer
	selfDevelopmentModes *SelfDevelopmentModeCAS
	checkpointAuthority  *CheckpointAuthority
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

type EventCapabilityAuthorizer interface {
	Authorize(r *http.Request, computerID, requiredScope string) error
}

func (h *Handler) ConfigureComputerEvents(cas *ComputerEventCAS, artifacts *EventArtifactService, auth EventCapabilityAuthorizer) error {
	if h == nil || cas == nil || artifacts == nil || auth == nil {
		return fmt.Errorf("corpusd handler: complete computer event dependencies are required")
	}
	h.eventCAS = cas
	h.eventArtifacts = artifacts
	h.eventAuth = auth
	checkpoints, err := NewCheckpointAuthority(cas, h.service)
	if err != nil {
		return err
	}
	h.checkpointAuthority = checkpoints
	return nil
}

func (h *Handler) ConfigureSelfDevelopmentModes(modes *SelfDevelopmentModeCAS) error {
	if h == nil || modes == nil {
		return fmt.Errorf("corpusd handler: self-development mode authority is required")
	}
	h.selfDevelopmentModes = modes
	return nil
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
		Service: "corpusd",
		Store:   storeStatus,
		Build:   buildinfo.Snapshot("corpusd"),
	})
}

func (h *Handler) HandleInternalPublishTexture(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if r.Header.Get("X-Internal-Caller") != "true" {
		writeJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return
	}
	var req PublishTextureRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}
	resp, err := h.service.PublishTexture(r.Context(), req)
	if err != nil {
		log.Printf("corpusd: publish texture: %v", err)
		writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (h *Handler) HandlePublicTexture(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusNotFound, apiError{Error: "corpusd does not render public HTML"})
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
		log.Printf("corpusd: resolve publication %s: %v", routePath, err)
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
		log.Printf("corpusd: export publication %s: %v", routePath, err)
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
		log.Printf("corpusd: retrieval search: %v", err)
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "failed to search published retrieval sources"})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) HandleInternalUniversalWireStories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if r.Header.Get("X-Internal-Caller") != "true" {
		writeJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return
	}
	resp, err := h.service.ListUniversalWireStories(r.Context())
	if err != nil {
		log.Printf("corpusd: list universal wire stories: %v", err)
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list universal wire stories"})
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
		log.Printf("corpusd: submit publication proposal: %v", err)
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
		log.Printf("corpusd: update proposal delivery: %v", err)
		writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) HandleInternalSyncTextureDocument(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if r.Header.Get("X-Internal-Caller") != "true" {
		writeJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return
	}
	var req SyncTextureDocumentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}
	resp, err := h.service.SyncTextureDocument(r.Context(), req)
	if err != nil {
		log.Printf("corpusd: sync texture document: %v", err)
		writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) HandleInternalGetTextureDocument(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if r.Header.Get("X-Internal-Caller") != "true" {
		writeJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return
	}
	docID := strings.TrimPrefix(r.URL.Path, "/internal/platform/texture/documents/")
	if docID == r.URL.Path {
		docID = strings.TrimPrefix(r.URL.Path, "/internal/platform/texture/documents")
	}
	docID = strings.TrimSuffix(docID, "/revisions")
	docID = strings.Trim(docID, "/")
	docID = strings.TrimSpace(docID)
	if docID == "" {
		http.NotFound(w, r)
		return
	}
	doc, err := h.service.GetPlatformTextureDocument(r.Context(), docID)
	if err != nil {
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
			return
		}
		log.Printf("corpusd: get texture document %s: %v", docID, err)
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "failed to get texture document"})
		return
	}
	writeJSON(w, http.StatusOK, doc)
}

func (h *Handler) HandleInternalListTextureRevisions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if r.Header.Get("X-Internal-Caller") != "true" {
		writeJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return
	}
	if !strings.HasSuffix(r.URL.Path, "/revisions") {
		h.HandleInternalGetTextureDocument(w, r)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/internal/platform/texture/documents/")
	docID := strings.TrimSuffix(path, "/revisions")
	docID = strings.Trim(docID, "/")
	docID = strings.TrimSpace(docID)
	if docID == "" {
		http.NotFound(w, r)
		return
	}
	revisions, err := h.service.ListPlatformTextureRevisions(r.Context(), docID)
	if err != nil {
		log.Printf("corpusd: list texture revisions %s: %v", docID, err)
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list texture revisions"})
		return
	}
	writeJSON(w, http.StatusOK, PlatformTextureRevisionListResponse{Revisions: revisions})
}

func (h *Handler) HandleInternalGetTextureRevision(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if r.Header.Get("X-Internal-Caller") != "true" {
		writeJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return
	}
	revisionID := strings.TrimPrefix(r.URL.Path, "/internal/platform/texture/revisions/")
	revisionID = strings.Trim(revisionID, "/")
	revisionID = strings.TrimSpace(revisionID)
	if revisionID == "" {
		http.NotFound(w, r)
		return
	}
	rev, err := h.service.GetPlatformTextureRevision(r.Context(), revisionID)
	if err != nil {
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
			return
		}
		log.Printf("corpusd: get texture revision %s: %v", revisionID, err)
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "failed to get texture revision"})
		return
	}
	writeJSON(w, http.StatusOK, rev)
}

func RegisterRoutes(s *server.Server, h *Handler) {
	s.SetHealthHandler(h.HandleHealth)
	s.HandleFunc("/internal/computers/credentials/exchange", h.HandleComputerCredentialExchange)
	s.HandleFunc("/internal/computers/credentials/consume", h.HandleComputerCredentialConsume)
	s.HandleFunc("/internal/computers/credentials/issue", h.HandleComputerCredentialIssue)
	s.HandleFunc("/internal/computers/credentials/renew", h.HandleComputerCredentialRenew)
	s.HandleFunc("/internal/computers/lifecycle/control", h.HandleComputerLifecycleControl)
	s.HandleFunc("/internal/computers/self-development/mode", h.HandleSelfDevelopmentMode)
	s.HandleFunc("/internal/computers/events/head", h.HandleComputerEventHead)
	s.HandleFunc("/internal/computers/events/pin", h.HandleComputerEventPin)
	s.HandleFunc("/internal/computers/events/append", h.HandleComputerEventAppend)
	s.HandleFunc("/internal/computers/events/replay", h.HandleComputerEventReplay)
	s.HandleFunc("/internal/computers/checkpoints", h.HandleComputerCheckpoint)
	s.HandleFunc("/internal/computers/route-projection-certificates", h.HandleRouteProjectionCertificate)
	s.HandleFunc("/internal/platform/control-key", h.HandlePlatformControlPublicKey)
	s.HandleFunc("/internal/platform/publications/texture", h.HandleInternalPublishTexture)
	s.HandleFunc("/internal/platform/publications/resolve", h.HandleInternalResolvePublication)
	s.HandleFunc("/internal/platform/publications/export", h.HandleInternalExportPublication)
	s.HandleFunc("/internal/platform/retrieval/search", h.HandleInternalRetrievalSearch)
	s.HandleFunc("/internal/platform/universal-wire/stories", h.HandleInternalUniversalWireStories)
	s.HandleFunc("/internal/platform/proposal-deliveries/state", h.HandleInternalProposalDeliveryState)
	s.HandleFunc("/internal/platform/publications/", h.HandleInternalPublicationProposal)
	s.HandleFunc("/internal/platform/texture/sync", h.HandleInternalSyncTextureDocument)
	s.HandleFunc("/internal/platform/texture/revisions/", h.HandleInternalGetTextureRevision)
	s.HandleFunc("/internal/platform/texture/documents/", h.HandleInternalListTextureRevisions)
	s.HandleFunc("/internal/platform/texture/documents", h.HandleInternalGetTextureDocument)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("corpusd: json encode: %v", err)
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
