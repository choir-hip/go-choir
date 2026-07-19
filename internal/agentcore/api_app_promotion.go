package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/promotion"
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
	HumanProof  appChangePackageHumanProof          `json:"human_proof"`
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
	HumanProofState       string                    `json:"human_proof_state"`
	SupportsHumanReview   bool                      `json:"supports_human_review"`
	MachineReceiptOnly    bool                      `json:"machine_receipt_only"`
	UpdatedAt             time.Time                 `json:"updated_at"`
}

type acceptanceContractState struct {
	Name  string `json:"name"`
	State string `json:"state"`
}

type appChangePackageHumanProof struct {
	State          string   `json:"state"`
	Summary        string   `json:"summary,omitempty"`
	Recommendation string   `json:"recommendation,omitempty"`
	NarrativeRefs  []string `json:"narrative_refs,omitempty"`
	ScreenshotRefs []string `json:"screenshot_refs,omitempty"`
	VideoRefs      []string `json:"video_refs,omitempty"`
	BenchmarkRefs  []string `json:"benchmark_refs,omitempty"`
	ArtifactRefs   []string `json:"artifact_refs,omitempty"`
	Missing        []string `json:"missing,omitempty"`
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
	case len(parts) >= 3 && parts[1] == "self-development":
		h.handleSelfDevelopmentRoute(w, r, ownerID, computerID, parts)
	case len(parts) == 2 && parts[1] == "source-lineage":
		if r.Method != http.MethodGet {
			writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
			return
		}
		kind := strings.TrimSpace(r.URL.Query().Get("kind"))
		rec, err := h.rt.promotion.EnsureComputerSourceLineage(r.Context(), ownerID, computerID, kind, "")
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
		var req promotion.CreateAppAdoptionInput
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid app adoption request"})
			return
		}
		rec, err := h.rt.promotion.CreateAppAdoption(r.Context(), ownerID, computerID, req)
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
		var req promotion.PublishAppChangePackageInput
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid app change package request"})
			return
		}
		rec, err := h.rt.promotion.PublishAppChangePackage(r.Context(), ownerID, req)
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
	imported, err := h.rt.promotion.ImportAppChangePackage(r.Context(), rec)
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
			HumanProof:  humanProofForAppChangePackage(pkg),
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
		HumanProof:  humanProofForAppChangePackage(pkg),
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
	if len(parts) >= 2 && parts[1] == "preview" {
		h.handleAppAdoptionPreview(w, r, ownerID, adoptionID, strings.Join(parts[2:], "/"))
		return
	}
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
		var req promotion.VerifyAppAdoptionInput
		if r.Body != nil && r.ContentLength != 0 {
			decoder := json.NewDecoder(r.Body)
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&req); err != nil {
				writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid app adoption verify request"})
				return
			}
		}
		var rec types.AppAdoptionRecord
		var err error
		if req.Async {
			rec, err = h.rt.promotion.StartVerifyAppAdoptionAsync(r.Context(), ownerID, adoptionID, req)
		} else {
			rec, err = h.rt.promotion.VerifyAppAdoption(r.Context(), ownerID, adoptionID, req)
		}
		if err != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
			return
		}
		if req.Async {
			writeAPIJSON(w, http.StatusAccepted, rec)
			return
		}
		writeAPIJSON(w, http.StatusOK, rec)
	case "approve":
		var req struct{}
		if r.Body != nil && r.ContentLength != 0 {
			decoder := json.NewDecoder(r.Body)
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&req); err != nil {
				writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid app adoption approve request"})
				return
			}
		}
		rec, err := h.rt.promotion.ApproveAppAdoption(r.Context(), ownerID, adoptionID)
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
		rec, err := h.rt.promotion.PromoteAppAdoption(r.Context(), ownerID, adoptionID)
		if err != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
			return
		}
		writeAPIJSON(w, http.StatusOK, rec)
	case "rollback":
		rec, err := h.rt.promotion.RollbackAppAdoption(r.Context(), ownerID, adoptionID)
		if err != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
			return
		}
		writeAPIJSON(w, http.StatusOK, rec)
	case "roll-forward":
		var req struct{}
		if r.Body != nil && r.ContentLength != 0 {
			decoder := json.NewDecoder(r.Body)
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&req); err != nil {
				writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid app adoption roll-forward request"})
				return
			}
		}
		rec, err := h.rt.promotion.RollForwardAppAdoption(r.Context(), ownerID, adoptionID)
		if err != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
			return
		}
		writeAPIJSON(w, http.StatusOK, rec)
	default:
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "app adoption action not found"})
	}
}

func (h *APIHandler) handleAppAdoptionPreview(w http.ResponseWriter, r *http.Request, ownerID, adoptionID, assetPath string) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	assetPath = strings.Trim(strings.TrimSpace(assetPath), "/")
	previewRoot, err := h.rt.appAdoptionUIPreviewRoot(r.Context(), ownerID, adoptionID)
	if err != nil {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: err.Error()})
		return
	}
	targetPath := filepath.Join(previewRoot, filepath.FromSlash(assetPath))
	if assetPath == "" {
		targetPath = filepath.Join(previewRoot, "index.html")
	}
	if !pathWithinRoot(previewRoot, targetPath) {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "unsafe preview path"})
		return
	}
	info, err := os.Stat(targetPath)
	if err != nil || info.IsDir() {
		targetPath = filepath.Join(previewRoot, "index.html")
		info, err = os.Stat(targetPath)
		if err != nil || info.IsDir() {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "adoption preview index not found"})
			return
		}
	}
	if filepath.Base(targetPath) == "index.html" {
		h.serveAppAdoptionPreviewIndex(w, r, targetPath, adoptionID)
		return
	}
	http.ServeFile(w, r, targetPath)
}

func (h *APIHandler) serveAppAdoptionPreviewIndex(w http.ResponseWriter, r *http.Request, indexPath, adoptionID string) {
	data, err := os.ReadFile(indexPath)
	if err != nil {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "adoption preview index not found"})
		return
	}
	prefix := "/api/adoptions/" + url.PathEscape(adoptionID) + "/preview"
	html := string(data)
	html = strings.ReplaceAll(html, `src="/assets/`, `src="`+prefix+`/assets/`)
	html = strings.ReplaceAll(html, `href="/assets/`, `href="`+prefix+`/assets/`)
	html = strings.ReplaceAll(html, `src=/assets/`, `src=`+prefix+`/assets/`)
	html = strings.ReplaceAll(html, `href=/assets/`, `href=`+prefix+`/assets/`)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(html))
}

func (rt *Runtime) appAdoptionUIPreviewRoot(ctx context.Context, ownerID, adoptionID string) (string, error) {
	if rt == nil || rt.store == nil {
		return "", fmt.Errorf("adoption preview: runtime store is unavailable")
	}
	rec, err := rt.store.GetAppAdoption(ctx, ownerID, adoptionID)
	if err != nil {
		return "", fmt.Errorf("adoption preview not found")
	}
	if rec.Status != types.AppAdoptionVerified && rec.Status != types.AppAdoptionAdopted && rec.Status != types.AppAdoptionRolledBack {
		return "", fmt.Errorf("adoption preview requires a verified recipient build")
	}
	root := strings.TrimSpace(rt.cfg.PromotionWorkspaceRoot)
	if root == "" {
		return "", fmt.Errorf("adoption preview workspace root is not configured")
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("adoption preview workspace root is invalid")
	}
	workspacePath, uiArtifactPath := appAdoptionPreviewPaths(rec.VerifierResultsJSON)
	if strings.TrimSpace(workspacePath) == "" {
		return "", fmt.Errorf("adoption preview workspace is not recorded")
	}
	if strings.TrimSpace(uiArtifactPath) == "" {
		uiArtifactPath = rt.cfg.AppPromotionUIArtifactPath
	}
	if strings.TrimSpace(uiArtifactPath) == "" {
		return "", fmt.Errorf("adoption preview UI artifact path is not configured")
	}
	absWorkspace, err := filepath.Abs(workspacePath)
	if err != nil {
		return "", fmt.Errorf("adoption preview workspace is invalid")
	}
	if !pathWithinRoot(absRoot, absWorkspace) {
		return "", fmt.Errorf("adoption preview workspace is outside promotion root")
	}
	previewRoot := filepath.Join(absWorkspace, filepath.FromSlash(uiArtifactPath))
	absPreviewRoot, err := filepath.Abs(previewRoot)
	if err != nil {
		return "", fmt.Errorf("adoption preview UI root is invalid")
	}
	if !pathWithinRoot(absRoot, absPreviewRoot) || !pathWithinRoot(absWorkspace, absPreviewRoot) {
		return "", fmt.Errorf("adoption preview UI root is outside promotion workspace")
	}
	if info, err := os.Stat(absPreviewRoot); err != nil || !info.IsDir() {
		return "", fmt.Errorf("adoption preview UI artifact is not available")
	}
	return absPreviewRoot, nil
}

func appAdoptionPreviewPaths(raw json.RawMessage) (string, string) {
	var results []struct {
		ContractID string `json:"contract_id"`
		Details    struct {
			WorkspacePath  string `json:"workspace_path"`
			UIArtifactPath string `json:"ui_artifact_path"`
		} `json:"details"`
	}
	if err := json.Unmarshal(raw, &results); err != nil {
		return "", ""
	}
	for _, result := range results {
		if result.ContractID == "actual-recipient-runtime-ui-build" {
			return strings.TrimSpace(result.Details.WorkspacePath), strings.TrimSpace(result.Details.UIArtifactPath)
		}
	}
	return "", ""
}

func pathWithinRoot(root, path string) bool {
	root, rootErr := filepath.Abs(filepath.Clean(root))
	path, pathErr := filepath.Abs(filepath.Clean(path))
	if rootErr != nil || pathErr != nil {
		return false
	}
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != "." && !strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel)
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
	humanState, supportsHumanReview := runAcceptanceHumanProofState(rec)
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
		HumanProofState:       humanState,
		SupportsHumanReview:   supportsHumanReview,
		MachineReceiptOnly:    !supportsHumanReview,
		UpdatedAt:             rec.UpdatedAt,
	}
}

func runAcceptanceHumanProofState(rec types.RunAcceptanceRecord) (string, bool) {
	hasNarrative := false
	hasMediaOrBenchmark := false
	for _, ref := range rec.EvidenceRefs {
		kind := strings.ToLower(strings.TrimSpace(ref.Kind))
		summary := ref.Summary + " " + ref.URL
		lowerSummary := strings.ToLower(summary)
		if strings.Contains(kind, "texture") || strings.Contains(lowerSummary, "texture") {
			hasNarrative = true
		}
		if strings.Contains(kind, "screenshot") ||
			strings.Contains(kind, "video") ||
			strings.Contains(lowerSummary, ".png") ||
			strings.Contains(lowerSummary, ".jpg") ||
			strings.Contains(lowerSummary, ".jpeg") ||
			strings.Contains(lowerSummary, ".webm") ||
			strings.Contains(lowerSummary, ".mp4") ||
			((strings.Contains(kind, "benchmark") || strings.Contains(lowerSummary, "benchmark")) && credibleHumanBenchmarkRef(summary)) {
			hasMediaOrBenchmark = true
		}
		for _, value := range ref.Details {
			text := fmt.Sprint(value)
			lowerText := strings.ToLower(text)
			if strings.Contains(lowerText, "texture") {
				hasNarrative = true
			}
			if strings.Contains(lowerText, ".png") || strings.Contains(lowerText, ".jpg") || strings.Contains(lowerText, ".jpeg") || strings.Contains(lowerText, ".webm") || strings.Contains(lowerText, ".mp4") || (strings.Contains(lowerText, "benchmark") && credibleHumanBenchmarkRef(text)) {
				hasMediaOrBenchmark = true
			}
		}
	}
	if hasNarrative && hasMediaOrBenchmark {
		return "human_reviewable", true
	}
	if rec.AcceptanceLevel == types.RunAcceptanceExportLevel || rec.AcceptanceLevel == types.RunAcceptancePromotionLevel || rec.AcceptanceLevel == types.RunAcceptanceContinuationLevel {
		return "machine_receipt_only", false
	}
	return "evidence_pending", false
}

func humanProofForAppChangePackage(pkg types.AppChangePackageRecord) appChangePackageHumanProof {
	var provenance any
	_ = json.Unmarshal(pkg.ProvenanceRefsJSON, &provenance)
	proof := appChangePackageHumanProof{
		State: "evidence_pending",
	}
	collectHumanProofValue(&proof, provenance, "")
	proof.NarrativeRefs = compactStringRefs(proof.NarrativeRefs)
	proof.ScreenshotRefs = compactStringRefs(proof.ScreenshotRefs)
	proof.VideoRefs = compactStringRefs(proof.VideoRefs)
	proof.BenchmarkRefs = compactStringRefs(proof.BenchmarkRefs)
	proof.ArtifactRefs = compactStringRefs(proof.ArtifactRefs)
	hasHumanEvidence := len(proof.ScreenshotRefs) > 0 || len(proof.VideoRefs) > 0 || hasCredibleHumanBenchmarkRefs(proof.BenchmarkRefs)
	if len(proof.NarrativeRefs) > 0 && hasHumanEvidence {
		proof.State = "human_reviewable"
		return proof
	}
	if len(proof.NarrativeRefs) == 0 {
		proof.Missing = append(proof.Missing, "narrative Texture")
	}
	if !hasHumanEvidence {
		proof.Missing = append(proof.Missing, "successful screenshots, video, or benchmark evidence")
	}
	return proof
}

func hasCredibleHumanBenchmarkRefs(refs []string) bool {
	for _, ref := range refs {
		if credibleHumanBenchmarkRef(ref) {
			return true
		}
	}
	return false
}

func credibleHumanBenchmarkRef(ref string) bool {
	text := strings.ToLower(strings.TrimSpace(ref))
	if text == "" {
		return false
	}
	for _, blocked := range []string{
		"blocked",
		"failed",
		"failure",
		"error",
		"unavailable",
		"not available",
		"pending",
		"not run",
		"not captured",
		"cannot run",
		"could not",
	} {
		if strings.Contains(text, blocked) {
			return false
		}
	}
	for _, receiptOnly := range []string{
		"npm --prefix frontend run build",
		"npm --prefix frontend ci",
		"npm ci",
		"npm install",
		"pnpm build",
		"go build",
		"vite build",
		"build proof",
		"build receipt",
		"build passed",
		"build pass",
		"frontend production build",
		"chunk-size warning",
		"npm audit",
	} {
		if strings.Contains(text, receiptOnly) {
			return false
		}
	}
	hasMeasurement := strings.ContainsAny(text, "0123456789")
	if !hasMeasurement {
		return false
	}
	for _, signal := range []string{
		"benchmark",
		"latency",
		"duration",
		"tokens",
		"fps",
		"memory",
		"cpu",
		"resource",
		"wall time",
		"p95",
		"median",
	} {
		if strings.Contains(text, signal) {
			return true
		}
	}
	return false
}

func collectHumanProofValue(proof *appChangePackageHumanProof, value any, key string) {
	switch typed := value.(type) {
	case map[string]any:
		for k, v := range typed {
			lowerKey := strings.ToLower(strings.TrimSpace(k))
			if lowerKey == "summary" || lowerKey == "human_summary" || lowerKey == "narrative_summary" {
				if proof.Summary == "" {
					proof.Summary = strings.TrimSpace(fmt.Sprint(v))
				}
			}
			if lowerKey == "recommendation" && proof.Recommendation == "" {
				proof.Recommendation = strings.TrimSpace(fmt.Sprint(v))
			}
			collectHumanProofValue(proof, v, lowerKey)
		}
	case []any:
		for _, item := range typed {
			collectHumanProofValue(proof, item, key)
		}
	case []string:
		for _, item := range typed {
			collectHumanProofString(proof, key, item)
		}
	case string:
		collectHumanProofString(proof, key, typed)
	}
}

func collectHumanProofString(proof *appChangePackageHumanProof, key, raw string) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return
	}
	lowerKey := strings.ToLower(key)
	lowerText := strings.ToLower(text)
	switch {
	case strings.Contains(lowerKey, "texture") || strings.Contains(lowerKey, "narrative_ref") || strings.Contains(lowerText, "texture:"):
		proof.NarrativeRefs = append(proof.NarrativeRefs, text)
	case strings.Contains(lowerKey, "screenshot") || strings.Contains(lowerKey, "image") || strings.HasSuffix(lowerText, ".png") || strings.HasSuffix(lowerText, ".jpg") || strings.HasSuffix(lowerText, ".jpeg"):
		proof.ScreenshotRefs = append(proof.ScreenshotRefs, text)
	case strings.Contains(lowerKey, "video") || strings.HasSuffix(lowerText, ".webm") || strings.HasSuffix(lowerText, ".mp4"):
		proof.VideoRefs = append(proof.VideoRefs, text)
	case strings.Contains(lowerKey, "benchmark"):
		proof.BenchmarkRefs = append(proof.BenchmarkRefs, text)
	case strings.Contains(lowerKey, "artifact") || strings.Contains(lowerKey, "evidence"):
		proof.ArtifactRefs = append(proof.ArtifactRefs, text)
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
