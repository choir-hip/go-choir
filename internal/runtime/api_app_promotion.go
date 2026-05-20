package runtime

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/types"
)

type appChangePackageListResponse struct {
	Packages []types.AppChangePackageRecord `json:"packages"`
}

type appAdoptionListResponse struct {
	Adoptions []types.AppAdoptionRecord `json:"adoptions"`
}

func (h *APIHandler) HandleComputersRouter(w http.ResponseWriter, r *http.Request) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	const prefix = "/api/computers/"
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, prefix), "/")
	parts := strings.Split(rest, "/")
	if len(parts) < 2 || strings.TrimSpace(parts[0]) == "" {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "computer route not found"})
		return
	}
	computerID := strings.TrimSpace(parts[0])
	switch {
	case len(parts) == 2 && parts[1] == "source-lineage":
		if r.Method != http.MethodGet {
			writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
			return
		}
		kind := strings.TrimSpace(r.URL.Query().Get("kind"))
		rec, err := h.rt.EnsureComputerSourceLineage(r.Context(), ownerID, computerID, kind, "")
		if err != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
			return
		}
		writeAPIJSON(w, http.StatusOK, rec)
	case len(parts) == 2 && parts[1] == "adoptions":
		if r.Method != http.MethodPost {
			writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
			return
		}
		var req createAppAdoptionInput
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid app adoption request"})
			return
		}
		rec, err := h.rt.CreateAppAdoption(r.Context(), ownerID, computerID, req)
		if err != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
			return
		}
		writeAPIJSON(w, http.StatusCreated, rec)
	default:
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "computer route not found"})
	}
}

func (h *APIHandler) HandleAppChangePackagesRoot(w http.ResponseWriter, r *http.Request) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		limit := apiLimit(r, 100)
		packages, err := h.rt.store.ListAppChangePackages(r.Context(), ownerID, limit)
		if err != nil {
			log.Printf("runtime api: list app change packages: %v", err)
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list app change packages"})
			return
		}
		writeAPIJSON(w, http.StatusOK, appChangePackageListResponse{Packages: packages})
	case http.MethodPost:
		var req publishAppChangePackageInput
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid app change package request"})
			return
		}
		rec, err := h.rt.PublishAppChangePackage(r.Context(), ownerID, req)
		if err != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
			return
		}
		writeAPIJSON(w, http.StatusCreated, rec)
	default:
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
	}
}

func (h *APIHandler) HandleInternalAppChangePackagesRoot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if err := requireInternalRuntimeCaller(r); err != nil {
		writeAPIJSON(w, http.StatusForbidden, apiError{Error: "internal runtime endpoints are not publicly accessible"})
		return
	}
	var rec types.AppChangePackageRecord
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&rec); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid app change package import"})
		return
	}
	imported, err := h.rt.store.UpsertAppChangePackage(r.Context(), rec)
	if err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	writeAPIJSON(w, http.StatusCreated, imported)
}

func (h *APIHandler) HandleAppChangePackageDetail(w http.ResponseWriter, r *http.Request) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	const prefix = "/api/app-change-packages/"
	packageID := strings.Trim(strings.TrimPrefix(r.URL.Path, prefix), "/")
	if packageID == "" || strings.Contains(packageID, "/") {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "app change package not found"})
		return
	}
	rec, err := h.rt.store.GetAppChangePackageForViewer(r.Context(), ownerID, packageID)
	if err != nil {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "app change package not found"})
		return
	}
	writeAPIJSON(w, http.StatusOK, rec)
}

func (h *APIHandler) HandleInternalAppChangePackageDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if err := requireInternalRuntimeCaller(r); err != nil {
		writeAPIJSON(w, http.StatusForbidden, apiError{Error: "internal runtime endpoints are not publicly accessible"})
		return
	}
	viewerID := strings.TrimSpace(r.URL.Query().Get("viewer_id"))
	if viewerID == "" {
		viewerID = strings.TrimSpace(r.URL.Query().Get("owner_id"))
	}
	if viewerID == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "viewer_id is required"})
		return
	}
	const prefix = "/internal/runtime/app-change-packages/"
	packageID := strings.Trim(strings.TrimPrefix(r.URL.Path, prefix), "/")
	if packageID == "" || strings.Contains(packageID, "/") {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "app change package not found"})
		return
	}
	rec, err := h.rt.store.GetAppChangePackageForViewer(r.Context(), viewerID, packageID)
	if err != nil {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "app change package not found"})
		return
	}
	writeAPIJSON(w, http.StatusOK, rec)
}

func (h *APIHandler) HandleAppAdoptionsRoot(w http.ResponseWriter, r *http.Request) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	limit := apiLimit(r, 100)
	adoptions, err := h.rt.store.ListAppAdoptions(r.Context(), ownerID, limit)
	if err != nil {
		log.Printf("runtime api: list app adoptions: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list app adoptions"})
		return
	}
	writeAPIJSON(w, http.StatusOK, appAdoptionListResponse{Adoptions: adoptions})
}

func (h *APIHandler) HandleAppAdoptionDetail(w http.ResponseWriter, r *http.Request) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	const prefix = "/api/adoptions/"
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, prefix), "/")
	parts := strings.Split(rest, "/")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "app adoption not found"})
		return
	}
	adoptionID := strings.TrimSpace(parts[0])
	if len(parts) == 1 {
		if r.Method != http.MethodGet {
			writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
			return
		}
		rec, err := h.rt.store.GetAppAdoption(r.Context(), ownerID, adoptionID)
		if err != nil {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "app adoption not found"})
			return
		}
		writeAPIJSON(w, http.StatusOK, rec)
		return
	}
	if len(parts) != 2 || r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "app adoption action not found"})
		return
	}
	switch parts[1] {
	case "verify":
		var req verifyAppAdoptionInput
		if r.Body != nil && r.ContentLength != 0 {
			decoder := json.NewDecoder(r.Body)
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&req); err != nil {
				writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid app adoption verify request"})
				return
			}
		}
		rec, err := h.rt.VerifyAppAdoption(r.Context(), ownerID, adoptionID, req)
		if err != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
			return
		}
		writeAPIJSON(w, http.StatusOK, rec)
	case "promote":
		var req struct{}
		if r.Body != nil && r.ContentLength != 0 {
			decoder := json.NewDecoder(r.Body)
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&req); err != nil {
				writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid app adoption promote request"})
				return
			}
		}
		rec, err := h.rt.PromoteAppAdoption(r.Context(), ownerID, adoptionID)
		if err != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
			return
		}
		writeAPIJSON(w, http.StatusOK, rec)
	case "rollback":
		rec, err := h.rt.RollbackAppAdoption(r.Context(), ownerID, adoptionID)
		if err != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
			return
		}
		writeAPIJSON(w, http.StatusOK, rec)
	default:
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "app adoption action not found"})
	}
}

func apiLimit(r *http.Request, fallback int) int {
	limit := fallback
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err == nil && parsed > 0 {
			limit = parsed
		}
	}
	return limit
}
