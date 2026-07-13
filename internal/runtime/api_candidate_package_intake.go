package runtime

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/server"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type candidatePackageIntakeListResponse struct {
	Intakes []types.CandidatePackageIntakeRecord `json:"intakes"`
}

// RegisterCandidatePackageIntakeRoutes mounts the full candidate-computer
// package intake API on an explicit registrar. It includes write/transition
// routes, so deployed runtime registration must use the narrow read-only
// review-surface registrar instead of this opt-in harness registrar.
func RegisterCandidatePackageIntakeRoutes(s *server.Server, h *APIHandler) {
	if candidatePackageIntakeWriteRoutesDisabled() {
		panic("candidate package intake write routes are disabled for deployed runtime; use RegisterCandidatePackageReviewSurfaceRoutes")
	}
	s.HandleFunc("/api/candidate-package-intakes", h.HandleCandidatePackageIntakesRoot)
	s.HandleFunc("/api/candidate-package-intakes/", h.HandleCandidatePackageIntakeDetail)
}

func candidatePackageIntakeWriteRoutesDisabled() bool {
	for _, key := range []string{"CHOIR_DEPLOYED_RUNTIME", "CHOIR_PRODUCTION", "RUNTIME_DEPLOYED"} {
		switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
		case "1", "true", "yes", "on", "production", "deployed":
			return true
		}
	}
	return strings.EqualFold(strings.TrimSpace(os.Getenv("CHOIR_MODE")), "cloud")
}

func (h *APIHandler) HandleCandidatePackageReviewSurfaceReadOnly(w http.ResponseWriter, r *http.Request) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	const prefix = "/api/candidate-package-intakes/"
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, prefix), "/")
	parts := strings.Split(rest, "/")
	if len(parts) == 5 && parts[1] == "adoption-review" && parts[3] == "promotion-switch" && parts[4] == "review-surface" {
		h.handleCandidatePackageIntakePromotionReviewSurface(w, r, ownerID, strings.TrimSpace(parts[0]), strings.TrimSpace(parts[2]))
		return
	}
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	writeAPIJSON(w, http.StatusNotFound, apiError{Error: "candidate package review surface not found"})
}

func (h *APIHandler) HandleCandidatePackageIntakesRoot(w http.ResponseWriter, r *http.Request) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		intakes, err := h.rt.ListCandidatePackageIntakes(r.Context(), ownerID, apiLimit(r, 100))
		if err != nil {
			log.Printf("runtime api: list candidate package intakes: %v", err)
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list candidate package intakes"})
			return
		}
		writeAPIJSON(w, http.StatusOK, candidatePackageIntakeListResponse{Intakes: intakes})
	case http.MethodPost:
		var req candidatePackageIntakeCreateInput
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid candidate package intake request"})
			return
		}
		rec, err := h.rt.CreateCandidatePackageIntake(r.Context(), ownerID, req)
		if err != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
			return
		}
		writeAPIJSON(w, http.StatusCreated, rec)
	default:
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
	}
}

func (h *APIHandler) HandleCandidatePackageIntakeDetail(w http.ResponseWriter, r *http.Request) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	const prefix = "/api/candidate-package-intakes/"
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, prefix), "/")
	parts := strings.Split(rest, "/")
	if len(parts) == 2 {
		switch parts[1] {
		case "review":
			h.handleCandidatePackageIntakeReview(w, r, ownerID, strings.TrimSpace(parts[0]))
			return
		case "adoption-boundary":
			h.handleCandidatePackageIntakeAdoptionBoundary(w, r, ownerID, strings.TrimSpace(parts[0]))
			return
		case "publication-draft":
			h.handleCandidatePackageIntakePublicationDraft(w, r, ownerID, strings.TrimSpace(parts[0]))
			return
		case "adoption-review":
			h.handleCandidatePackageIntakeAdoptionReviewCreate(w, r, ownerID, strings.TrimSpace(parts[0]))
			return
		}
	}
	if len(parts) == 5 && parts[1] == "adoption-review" && parts[3] == "promotion-switch" {
		switch parts[4] {
		case "rollback":
			h.handleCandidatePackageIntakePromotionSwitchRollback(w, r, ownerID, strings.TrimSpace(parts[0]), strings.TrimSpace(parts[2]))
			return
		case "roll-forward":
			h.handleCandidatePackageIntakePromotionSwitchRollForward(w, r, ownerID, strings.TrimSpace(parts[0]), strings.TrimSpace(parts[2]))
			return
		case "acceptance":
			h.handleCandidatePackageIntakePromotionAcceptance(w, r, ownerID, strings.TrimSpace(parts[0]), strings.TrimSpace(parts[2]))
			return
		case "review-surface":
			h.handleCandidatePackageIntakePromotionReviewSurface(w, r, ownerID, strings.TrimSpace(parts[0]), strings.TrimSpace(parts[2]))
			return
		}
	}
	if len(parts) == 4 && parts[1] == "adoption-review" && parts[3] == "promotion-switch" {
		h.handleCandidatePackageIntakePromotionSwitch(w, r, ownerID, strings.TrimSpace(parts[0]), strings.TrimSpace(parts[2]))
		return
	}
	if len(parts) == 3 && parts[1] == "adoption-review" {
		h.handleCandidatePackageIntakeAdoptionReviewDecision(w, r, ownerID, strings.TrimSpace(parts[0]), strings.TrimSpace(parts[2]))
		return
	}
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if len(parts) != 1 || strings.TrimSpace(parts[0]) == "" {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "candidate package intake not found"})
		return
	}
	intakeID := strings.TrimSpace(parts[0])
	rec, err := h.rt.GetCandidatePackageIntake(r.Context(), ownerID, intakeID)
	if err != nil {
		if err == store.ErrNotFound {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "candidate package intake not found"})
			return
		}
		log.Printf("runtime api: get candidate package intake: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to load candidate package intake"})
		return
	}
	writeAPIJSON(w, http.StatusOK, rec)
}

func (h *APIHandler) handleCandidatePackageIntakeReview(w http.ResponseWriter, r *http.Request, ownerID, intakeID string) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if intakeID == "" {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "candidate package intake not found"})
		return
	}
	var req candidatePackageIntakeReviewInput
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid candidate package intake review request"})
		return
	}
	rec, err := h.rt.ReviewCandidatePackageIntake(r.Context(), ownerID, intakeID, req)
	if err != nil {
		if err == store.ErrNotFound {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "candidate package intake not found"})
			return
		}
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	writeAPIJSON(w, http.StatusOK, rec)
}

func (h *APIHandler) handleCandidatePackageIntakeAdoptionBoundary(w http.ResponseWriter, r *http.Request, ownerID, intakeID string) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if intakeID == "" {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "candidate package intake not found"})
		return
	}
	var req candidatePackageIntakeAdoptionBoundaryInput
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid candidate package intake adoption boundary request"})
		return
	}
	rec, err := h.rt.BindCandidatePackageIntakeAdoptionBoundary(r.Context(), ownerID, intakeID, req)
	if err != nil {
		if err == store.ErrNotFound {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "candidate package intake not found"})
			return
		}
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	writeAPIJSON(w, http.StatusOK, rec)
}

func (h *APIHandler) handleCandidatePackageIntakePublicationDraft(w http.ResponseWriter, r *http.Request, ownerID, intakeID string) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if intakeID == "" {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "candidate package intake not found"})
		return
	}
	var req candidatePackageIntakePublicationDraftInput
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid candidate package intake publication draft request"})
		return
	}
	rec, err := h.rt.CreateCandidatePackageIntakePublicationDraft(r.Context(), ownerID, intakeID, req)
	if err != nil {
		if err == store.ErrNotFound {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "candidate package intake not found"})
			return
		}
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	writeAPIJSON(w, http.StatusCreated, rec)
}

func (h *APIHandler) handleCandidatePackageIntakeAdoptionReviewCreate(w http.ResponseWriter, r *http.Request, ownerID, intakeID string) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if intakeID == "" {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "candidate package intake not found"})
		return
	}
	var req candidatePackageIntakeAdoptionReviewCreateInput
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid candidate package intake adoption review request"})
		return
	}
	rec, err := h.rt.CreateCandidatePackageIntakeAdoptionReview(r.Context(), ownerID, intakeID, req)
	if err != nil {
		if err == store.ErrNotFound {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "candidate package intake not found"})
			return
		}
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	writeAPIJSON(w, http.StatusCreated, rec)
}

func (h *APIHandler) handleCandidatePackageIntakeAdoptionReviewDecision(w http.ResponseWriter, r *http.Request, ownerID, intakeID, adoptionID string) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if intakeID == "" || adoptionID == "" {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "candidate package intake adoption review not found"})
		return
	}
	var req candidatePackageIntakeAdoptionReviewDecisionInput
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid candidate package intake adoption review decision request"})
		return
	}
	rec, err := h.rt.ReviewCandidatePackageIntakeAdoption(r.Context(), ownerID, intakeID, adoptionID, req)
	if err != nil {
		if err == store.ErrNotFound {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "candidate package intake adoption review not found"})
			return
		}
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	writeAPIJSON(w, http.StatusOK, rec)
}

func (h *APIHandler) handleCandidatePackageIntakePromotionSwitch(w http.ResponseWriter, r *http.Request, ownerID, intakeID, adoptionID string) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if intakeID == "" || adoptionID == "" {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "candidate package intake promotion switch not found"})
		return
	}
	var req candidatePackageIntakePromotionSwitchInput
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid candidate package intake promotion switch request"})
		return
	}
	rec, err := h.rt.SwitchCandidatePackageIntakeAdoptionReview(r.Context(), ownerID, intakeID, adoptionID, req)
	if err != nil {
		if err == store.ErrNotFound {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "candidate package intake promotion switch not found"})
			return
		}
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	writeAPIJSON(w, http.StatusOK, rec)
}

func (h *APIHandler) handleCandidatePackageIntakePromotionSwitchRollback(w http.ResponseWriter, r *http.Request, ownerID, intakeID, adoptionID string) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if intakeID == "" || adoptionID == "" {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "candidate package intake promotion switch rollback not found"})
		return
	}
	var req candidatePackageIntakePromotionSwitchRollbackInput
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid candidate package intake promotion switch rollback request"})
		return
	}
	rec, err := h.rt.RollbackCandidatePackageIntakeAdoptionReview(r.Context(), ownerID, intakeID, adoptionID, req)
	if err != nil {
		if err == store.ErrNotFound {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "candidate package intake promotion switch rollback not found"})
			return
		}
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	writeAPIJSON(w, http.StatusOK, rec)
}

func (h *APIHandler) handleCandidatePackageIntakePromotionSwitchRollForward(w http.ResponseWriter, r *http.Request, ownerID, intakeID, adoptionID string) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if intakeID == "" || adoptionID == "" {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "candidate package intake promotion switch roll-forward not found"})
		return
	}
	var req candidatePackageIntakePromotionSwitchRollForwardInput
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid candidate package intake promotion switch roll-forward request"})
		return
	}
	rec, err := h.rt.RollForwardCandidatePackageIntakeAdoptionReview(r.Context(), ownerID, intakeID, adoptionID, req)
	if err != nil {
		if err == store.ErrNotFound {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "candidate package intake promotion switch roll-forward not found"})
			return
		}
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	writeAPIJSON(w, http.StatusOK, rec)
}

func (h *APIHandler) handleCandidatePackageIntakePromotionAcceptance(w http.ResponseWriter, r *http.Request, ownerID, intakeID, adoptionID string) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if intakeID == "" || adoptionID == "" {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "candidate package promotion acceptance evidence not found"})
		return
	}
	rec, err := h.rt.CandidatePackagePromotionAcceptanceEvidence(r.Context(), ownerID, intakeID, adoptionID)
	if err != nil {
		if err == store.ErrNotFound {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "candidate package promotion acceptance evidence not found"})
			return
		}
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	writeAPIJSON(w, http.StatusOK, rec)
}

func (h *APIHandler) handleCandidatePackageIntakePromotionReviewSurface(w http.ResponseWriter, r *http.Request, ownerID, intakeID, adoptionID string) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if intakeID == "" || adoptionID == "" {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "candidate package promotion review surface not found"})
		return
	}
	rec, err := h.rt.CandidatePackagePromotionReviewSurface(r.Context(), ownerID, intakeID, adoptionID)
	if err != nil {
		if err == store.ErrNotFound {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "candidate package promotion review surface not found"})
			return
		}
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	writeAPIJSON(w, http.StatusOK, rec)
}
