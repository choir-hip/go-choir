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

func RegisterRoutes(s *server.Server, h *Handler) {
	s.SetHealthHandler(h.HandleHealth)
	s.HandleFunc("/internal/platform/publications/vtext", h.HandleInternalPublishVText)
	s.HandleFunc("/internal/platform/publications/resolve", h.HandleInternalResolvePublication)
	s.HandleFunc("/internal/platform/retrieval/search", h.HandleInternalRetrievalSearch)
	s.HandleFunc("/internal/platform/proposal-deliveries/state", h.HandleInternalProposalDeliveryState)
	s.HandleFunc("/internal/platform/publications/", h.HandleInternalPublicationProposal)
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
