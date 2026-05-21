package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

type appChangePackagePullRequest struct {
	PackageID       string `json:"package_id"`
	SourceOwnerID   string `json:"source_owner_id,omitempty"`
	SourceDesktopID string `json:"source_desktop_id,omitempty"`
	TargetDesktopID string `json:"target_desktop_id,omitempty"`
}

type appChangePackagePullResponse struct {
	Package         types.AppChangePackageRecord `json:"package"`
	SourceOwnerID   string                       `json:"source_owner_id"`
	SourceDesktopID string                       `json:"source_desktop_id"`
	TargetOwnerID   string                       `json:"target_owner_id"`
	TargetDesktopID string                       `json:"target_desktop_id"`
}

func isAppChangePackageReviewEvidencePath(pathValue string) bool {
	_, ok := appChangePackageReviewEvidencePackageID(pathValue)
	return ok
}

func appChangePackageReviewEvidencePackageID(pathValue string) (string, bool) {
	const prefix = "/api/app-change-packages/"
	const suffix = "/review-evidence"
	if !strings.HasPrefix(pathValue, prefix) || !strings.HasSuffix(pathValue, suffix) {
		return "", false
	}
	packageID := strings.Trim(strings.TrimSuffix(strings.TrimPrefix(pathValue, prefix), suffix), "/")
	if packageID == "" || strings.Contains(packageID, "/") {
		return "", false
	}
	return packageID, true
}

// HandleAppChangePackageReviewEvidence resolves package review evidence from
// the package's source computer. Review summaries are intentionally redacted by
// the source runtime; the proxy only resolves the correct computer so recipient
// computers do not need to already contain another user's run-acceptance rows.
func (h *Handler) HandleAppChangePackageReviewEvidence(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	authResult, err := h.validateAccessJWT(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		return
	}
	packageID, ok := appChangePackageReviewEvidencePackageID(r.URL.Path)
	if !ok {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "app change package not found"})
		return
	}
	sourceOwnerID := strings.TrimSpace(r.URL.Query().Get("source_owner_id"))
	if sourceOwnerID == "" {
		sourceOwnerID = authResult.UserID
	}
	sourceDesktopID := strings.TrimSpace(r.URL.Query().Get("source_desktop_id"))
	if sourceDesktopID == "" {
		sourceDesktopID = vmctl.PrimaryDesktopID
	}
	sourceSandboxURL, err := h.resolveSandboxURL(r.Context(), sourceOwnerID, sourceDesktopID)
	if err != nil {
		log.Printf("proxy app package review evidence: resolve source owner=%s desktop=%s: %v", sourceOwnerID, sourceDesktopID, err)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to resolve source computer"})
		return
	}
	targetRaw, err := joinBasePath(sourceSandboxURL, "/api/app-change-packages/"+url.PathEscape(packageID)+"/review-evidence")
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to build source evidence request"})
		return
	}
	target, err := url.Parse(targetRaw)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to parse source evidence request"})
		return
	}
	q := r.URL.Query()
	q.Del("source_owner_id")
	q.Del("source_desktop_id")
	target.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, target.String(), nil)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to build source evidence request"})
		return
	}
	req.Header.Set("X-Authenticated-User", authResult.UserID)
	resp, err := h.platformd.Do(req)
	if err != nil {
		log.Printf("proxy app package review evidence: fetch package=%s source_owner=%s: %v", packageID, sourceOwnerID, err)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to fetch package review evidence"})
		return
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to read package review evidence"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(body)
}

// HandleAppChangePackagePull imports a public/unlisted AppChangePackage from a
// source computer into the authenticated user's current computer store. It is a
// product API: the browser never sees sandbox URLs and adoption still happens
// through the recipient computer's normal /api/computers/*/adoptions path.
func (h *Handler) HandleAppChangePackagePull(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	authResult, err := h.validateAccessJWT(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		return
	}
	var req appChangePackagePullRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid app change package pull request"})
		return
	}
	req.PackageID = strings.TrimSpace(req.PackageID)
	if req.PackageID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "package_id is required"})
		return
	}
	sourceOwnerID := strings.TrimSpace(req.SourceOwnerID)
	if sourceOwnerID == "" {
		sourceOwnerID = authResult.UserID
	}
	sourceDesktopID := strings.TrimSpace(req.SourceDesktopID)
	if sourceDesktopID == "" {
		sourceDesktopID = vmctl.PrimaryDesktopID
	}
	targetDesktopID := strings.TrimSpace(req.TargetDesktopID)
	if targetDesktopID == "" {
		targetDesktopID = requestDesktopID(r)
	}

	sourceSandboxURL, err := h.resolveSandboxURL(r.Context(), sourceOwnerID, sourceDesktopID)
	if err != nil {
		log.Printf("proxy app package pull: resolve source owner=%s desktop=%s: %v", sourceOwnerID, sourceDesktopID, err)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to resolve source computer"})
		return
	}
	targetSandboxURL, err := h.resolveSandboxURL(r.Context(), authResult.UserID, targetDesktopID)
	if err != nil {
		log.Printf("proxy app package pull: resolve target owner=%s desktop=%s: %v", authResult.UserID, targetDesktopID, err)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to resolve target computer"})
		return
	}

	rec, err := h.fetchSourceAppChangePackage(r, sourceSandboxURL, authResult.UserID, req.PackageID)
	if err != nil {
		log.Printf("proxy app package pull: fetch package=%s source_owner=%s: %v", req.PackageID, sourceOwnerID, err)
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "app change package not found"})
		return
	}
	imported, err := h.importAppChangePackage(r, targetSandboxURL, rec)
	if err != nil {
		log.Printf("proxy app package pull: import package=%s target_owner=%s: %v", rec.PackageID, authResult.UserID, err)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to import app change package"})
		return
	}
	writeJSON(w, http.StatusCreated, appChangePackagePullResponse{
		Package:         imported,
		SourceOwnerID:   sourceOwnerID,
		SourceDesktopID: sourceDesktopID,
		TargetOwnerID:   authResult.UserID,
		TargetDesktopID: targetDesktopID,
	})
}

func (h *Handler) fetchSourceAppChangePackage(r *http.Request, sandboxBase, viewerID, packageID string) (types.AppChangePackageRecord, error) {
	targetRaw, err := joinBasePath(sandboxBase, "/internal/runtime/app-change-packages/"+url.PathEscape(packageID))
	if err != nil {
		return types.AppChangePackageRecord{}, err
	}
	target, err := url.Parse(targetRaw)
	if err != nil {
		return types.AppChangePackageRecord{}, fmt.Errorf("parse source package URL: %w", err)
	}
	q := target.Query()
	q.Set("viewer_id", viewerID)
	target.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, target.String(), nil)
	if err != nil {
		return types.AppChangePackageRecord{}, fmt.Errorf("build source package request: %w", err)
	}
	req.Header.Set("X-Internal-Caller", "true")
	req.Header.Set("X-Authenticated-User", viewerID)

	resp, err := h.platformd.Do(req)
	if err != nil {
		return types.AppChangePackageRecord{}, fmt.Errorf("call source sandbox: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return types.AppChangePackageRecord{}, fmt.Errorf("read source package response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return types.AppChangePackageRecord{}, fmt.Errorf("source package status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var rec types.AppChangePackageRecord
	if err := json.Unmarshal(body, &rec); err != nil {
		return types.AppChangePackageRecord{}, fmt.Errorf("decode source package response: %w", err)
	}
	if strings.TrimSpace(rec.PackageID) == "" {
		return types.AppChangePackageRecord{}, fmt.Errorf("source package response missing package_id")
	}
	return rec, nil
}

func (h *Handler) importAppChangePackage(r *http.Request, sandboxBase string, rec types.AppChangePackageRecord) (types.AppChangePackageRecord, error) {
	target, err := joinBasePath(sandboxBase, "/internal/runtime/app-change-packages")
	if err != nil {
		return types.AppChangePackageRecord{}, err
	}
	data, err := json.Marshal(rec)
	if err != nil {
		return types.AppChangePackageRecord{}, fmt.Errorf("encode package import: %w", err)
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, target, bytes.NewReader(data))
	if err != nil {
		return types.AppChangePackageRecord{}, fmt.Errorf("build package import request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Caller", "true")

	resp, err := h.platformd.Do(req)
	if err != nil {
		return types.AppChangePackageRecord{}, fmt.Errorf("call target sandbox: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return types.AppChangePackageRecord{}, fmt.Errorf("read package import response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return types.AppChangePackageRecord{}, fmt.Errorf("target package import status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var imported types.AppChangePackageRecord
	if err := json.Unmarshal(body, &imported); err != nil {
		return types.AppChangePackageRecord{}, fmt.Errorf("decode package import response: %w", err)
	}
	return imported, nil
}
