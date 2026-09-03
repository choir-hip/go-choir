package proxy

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// computerProducerReportsComputerID matches the two computer-scoped
// producer-report settlement routes that forward to the guest runtime:
// GET  /api/computers/<id>/lifecycle/producer-reports
// POST /api/computers/<id>/lifecycle/settle-producer-reports
func computerProducerReportsComputerID(path string) (computerID string, settle bool, ok bool) {
	const prefix = "/api/computers/"
	if !strings.HasPrefix(path, prefix) {
		return "", false, false
	}
	rest := strings.TrimPrefix(path, prefix)
	rawID, action, found := strings.Cut(rest, "/lifecycle/")
	if !found || strings.TrimSpace(rawID) == "" || strings.Contains(rawID, "/") {
		return "", false, false
	}
	switch action {
	case "producer-reports":
		settle = false
	case "settle-producer-reports":
		settle = true
	default:
		return "", false, false
	}
	computerID, err := url.PathUnescape(rawID)
	if err != nil || strings.TrimSpace(computerID) == "" {
		return "", false, false
	}
	return strings.TrimSpace(computerID), settle, true
}

// HandleComputerProducerReports forwards the owner-scoped producer-report
// enumeration and CAS settlement commands to the guest runtime with the
// authenticated computer binding. Settlement tombstones are lifecycle/CAS
// store writes under owner authority: API-key access requires the exact
// computer:lifecycle scope, never Texture revision authority and never
// Super consumption.
func (h *Handler) HandleComputerProducerReports(w http.ResponseWriter, r *http.Request) {
	computerID, settle, ok := computerProducerReportsComputerID(r.URL.Path)
	if !ok {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "not found"})
		return
	}
	if settle && r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	if !settle && r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	authResult, err := h.authenticate(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		return
	}
	if authResult.AuthMethod == "api_key" {
		const requiredScope = "computer:lifecycle"
		if !hasAPIKeyScope(authResult.Scopes, "admin") && !hasAPIKeyScope(authResult.Scopes, requiredScope) {
			writeJSON(w, http.StatusForbidden, errorResponse{Error: "missing exact computer:lifecycle scope"})
			return
		}
	}
	var target *resolvedComputerTarget
	if authResult.AuthMethod == "api_key" {
		target, ok = h.requireAPIKeyComputerTarget(w, r, authResult, computerID, "")
		if !ok {
			return
		}
	} else {
		target, err = h.resolveAuthorizedComputer(r.Context(), authResult, computerID)
		if err != nil || target == nil || target.ComputerID != computerID || target.UserID != authResult.UserID {
			writeJSON(w, http.StatusForbidden, errorResponse{Error: "computer ownership required"})
			return
		}
	}
	var autoputerURL string
	if authResult.AuthMethod == "api_key" {
		autoputerURL, err = h.resolveComputerURLForComputerTarget(r.Context(), authResult, target, target.DesktopID)
	} else if target.State == "active" && strings.TrimSpace(target.ComputerURL) != "" {
		autoputerURL = target.ComputerURL
	}
	if err != nil || strings.TrimSpace(autoputerURL) == "" {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "computer authority unavailable"})
		return
	}
	targetURL, err := joinBasePath(autoputerURL, r.URL.Path)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to build producer reports request"})
		return
	}
	u, err := url.Parse(targetURL)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to build producer reports request"})
		return
	}
	u.RawQuery = r.URL.RawQuery
	maxBody := int64(1 << 20)
	upstream, err := http.NewRequestWithContext(r.Context(), r.Method, u.String(), http.MaxBytesReader(w, r.Body, maxBody))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid producer reports request"})
		return
	}
	if contentType := r.Header.Get("Content-Type"); contentType != "" {
		upstream.Header.Set("Content-Type", contentType)
	}
	upstream.Header.Set("X-Authenticated-User", authResult.UserID)
	upstream.Header.Set("X-Authenticated-Computer", computerID)
	if authResult.Email != "" {
		upstream.Header.Set("X-Authenticated-Email", authResult.Email)
	}
	if len(authResult.Scopes) > 0 {
		upstream.Header.Set("X-Authenticated-Scopes", strings.Join(authResult.Scopes, ","))
	}
	client := h.autoputerHTTP
	if client == nil {
		client = http.DefaultClient
	}
	if client.Timeout > 0 {
		_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(client.Timeout + 30*time.Second))
	}
	response, err := client.Do(upstream)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "producer reports authority unavailable"})
		return
	}
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "invalid producer reports response"})
		return
	}
	if contentType := response.Header.Get("Content-Type"); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.WriteHeader(response.StatusCode)
	_, _ = w.Write(body)
}

func isComputerProducerReportsPath(path string) bool {
	_, _, ok := computerProducerReportsComputerID(path)
	return ok
}
