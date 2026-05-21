package runtime

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

type appChangePackageListResponse struct {
	Packages []types.AppChangePackageRecord `json:"packages"`
}

type appAdoptionListResponse struct {
	Adoptions []types.AppAdoptionRecord `json:"adoptions"`
}

type appChangePackageReviewEvidenceResponse struct {
	PackageID   string                              `json:"package_id"`
	Acceptances []appChangePackageAcceptanceSummary `json:"acceptances"`
}

type appChangePackageAcceptanceSummary struct {
	AcceptanceID          string                    `json:"acceptance_id"`
	TargetMissionID       string                    `json:"target_mission_id,omitempty"`
	SourcePromptObjective string                    `json:"source_prompt_or_objective,omitempty"`
	TrajectoryID          string                    `json:"trajectory_id"`
	AcceptanceLevel       types.RunAcceptanceLevel  `json:"acceptance_level"`
	State                 types.RunAcceptanceState  `json:"state"`
	AuthorityProfile      string                    `json:"authority_profile,omitempty"`
	EvidenceRefCount      int                       `json:"evidence_ref_count"`
	RollbackRefCount      int                       `json:"rollback_ref_count"`
	CheckpointKinds       []string                  `json:"checkpoint_kinds,omitempty"`
	VerifierContracts     []acceptanceContractState `json:"verifier_contracts,omitempty"`
	ReviewScope           string                    `json:"review_scope"`
	TraceVisible          bool                      `json:"trace_visible"`
	UpdatedAt             time.Time                 `json:"updated_at"`
}

type acceptanceContractState struct {
	Name  string `json:"name"`
	State string `json:"state"`
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
	const prefix = "/api/app-change-packages/"
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, prefix), "/")
	parts := strings.Split(rest, "/")
	if len(parts) == 2 && parts[1] == "review-evidence" {
		h.handleAppChangePackageReviewEvidence(w, r, ownerID, strings.TrimSpace(parts[0]))
		return
	}
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if len(parts) != 1 || strings.TrimSpace(parts[0]) == "" {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "app change package not found"})
		return
	}
	packageID := strings.TrimSpace(parts[0])
	rec, err := h.rt.store.GetAppChangePackageForViewer(r.Context(), ownerID, packageID)
	if err != nil {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "app change package not found"})
		return
	}
	writeAPIJSON(w, http.StatusOK, rec)
}

func (h *APIHandler) handleAppChangePackageReviewEvidence(w http.ResponseWriter, r *http.Request, viewerID, packageID string) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if packageID == "" {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "app change package not found"})
		return
	}
	pkg, err := h.rt.store.GetAppChangePackageForViewer(r.Context(), viewerID, packageID)
	if err != nil {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "app change package not found"})
		return
	}
	rawIDs := r.URL.Query()["acceptance_id"]
	if len(rawIDs) == 0 {
		writeAPIJSON(w, http.StatusOK, appChangePackageReviewEvidenceResponse{
			PackageID:   pkg.PackageID,
			Acceptances: []appChangePackageAcceptanceSummary{},
		})
		return
	}
	seen := map[string]bool{}
	summaries := []appChangePackageAcceptanceSummary{}
	for _, rawID := range rawIDs {
		acceptanceID := strings.TrimSpace(rawID)
		if acceptanceID == "" || seen[acceptanceID] {
			continue
		}
		seen[acceptanceID] = true
		if len(seen) > 8 {
			break
		}
		rec, err := h.rt.store.GetRunAcceptanceByID(r.Context(), acceptanceID)
		if err != nil {
			continue
		}
		scope, ok := reviewEvidenceScope(viewerID, pkg, rec)
		if !ok {
			continue
		}
		summaries = append(summaries, summarizePackageReviewAcceptance(viewerID, rec, scope))
	}
	writeAPIJSON(w, http.StatusOK, appChangePackageReviewEvidenceResponse{
		PackageID:   pkg.PackageID,
		Acceptances: summaries,
	})
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

func reviewEvidenceScope(viewerID string, pkg types.AppChangePackageRecord, rec types.RunAcceptanceRecord) (string, bool) {
	if rec.OwnerID == viewerID {
		return "viewer", true
	}
	if packageProvenanceNamesAcceptance(pkg, rec.AcceptanceID) {
		return "package-provenance", true
	}
	if acceptanceRecordReferencesPackage(rec, pkg.PackageID) {
		return "package-referenced", true
	}
	if pkg.TraceID != "" && rec.TrajectoryID == pkg.TraceID {
		return "package-trace", true
	}
	return "", false
}

func summarizePackageReviewAcceptance(viewerID string, rec types.RunAcceptanceRecord, scope string) appChangePackageAcceptanceSummary {
	checkpoints := make([]string, 0, len(rec.Checkpoints))
	for _, checkpoint := range rec.Checkpoints {
		if checkpoint.Kind == "" {
			continue
		}
		checkpoints = append(checkpoints, checkpoint.Kind)
		if len(checkpoints) >= 8 {
			break
		}
	}
	contracts := make([]acceptanceContractState, 0, len(rec.VerifierContracts))
	for _, contract := range rec.VerifierContracts {
		if contract.Name == "" {
			continue
		}
		contracts = append(contracts, acceptanceContractState{Name: contract.Name, State: contract.State})
		if len(contracts) >= 8 {
			break
		}
	}
	return appChangePackageAcceptanceSummary{
		AcceptanceID:          rec.AcceptanceID,
		TargetMissionID:       rec.TargetMissionID,
		SourcePromptObjective: rec.SourcePromptObjective,
		TrajectoryID:          rec.TrajectoryID,
		AcceptanceLevel:       rec.AcceptanceLevel,
		State:                 rec.State,
		AuthorityProfile:      rec.AuthorityProfile,
		EvidenceRefCount:      len(rec.EvidenceRefs),
		RollbackRefCount:      len(rec.RollbackRefs),
		CheckpointKinds:       checkpoints,
		VerifierContracts:     contracts,
		ReviewScope:           scope,
		TraceVisible:          rec.OwnerID == viewerID,
		UpdatedAt:             rec.UpdatedAt,
	}
}

func packageProvenanceNamesAcceptance(pkg types.AppChangePackageRecord, acceptanceID string) bool {
	if acceptanceID == "" || len(pkg.ProvenanceRefsJSON) == 0 {
		return false
	}
	return strings.Contains(string(pkg.ProvenanceRefsJSON), acceptanceID)
}

func acceptanceRecordReferencesPackage(rec types.RunAcceptanceRecord, packageID string) bool {
	if packageID == "" {
		return false
	}
	for _, text := range []string{
		rec.TargetMissionID,
		rec.SourcePromptObjective,
		rec.TrajectoryID,
		string(mustMarshalAcceptanceSurface(rec.Checkpoints)),
		string(mustMarshalAcceptanceSurface(rec.EvidenceRefs)),
		string(mustMarshalAcceptanceSurface(rec.RollbackRefs)),
		string(mustMarshalAcceptanceSurface(rec.VerifierContracts)),
		string(mustMarshalAcceptanceSurface(rec.InvariantChecks)),
	} {
		if strings.Contains(text, packageID) {
			return true
		}
	}
	return false
}

func mustMarshalAcceptanceSurface(v any) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return data
}
