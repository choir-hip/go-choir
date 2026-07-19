package proxy

import (
	"io"
	"net/http"
	"net/url"
	"strings"
)

func selfDevelopmentModeComputerID(path string) (string, bool) {

	const prefix = "/api/computers/"
	const suffix = "/self-development/mode"
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		return "", false
	}
	raw := strings.TrimSuffix(strings.TrimPrefix(path, prefix), suffix)
	if raw == "" || strings.Contains(raw, "/") {
		return "", false
	}
	computerID, err := url.PathUnescape(raw)
	if err != nil || strings.TrimSpace(computerID) == "" {
		return "", false
	}
	return strings.TrimSpace(computerID), true
}

func isSelfDevelopmentModePath(path string) bool {
	_, ok := selfDevelopmentModeComputerID(path)
	return ok
}

func (h *Handler) HandleSelfDevelopmentMode(w http.ResponseWriter, r *http.Request) {
	computerID, ok := selfDevelopmentModeComputerID(r.URL.Path)
	if !ok {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "not found"})
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	authResult, err := h.authenticate(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		return
	}
	requiredScope := "computer:self_development:read"
	if r.Method == http.MethodPost {
		requiredScope = "computer:self_development:mode"
	}
	if authResult.AuthMethod == "api_key" {
		if authResult.ComputerID != computerID {
			writeJSON(w, http.StatusForbidden, errorResponse{Error: "api key is bound to another computer"})
			return
		}
		if !hasAPIKeyScope(authResult.Scopes, "admin") && !hasAPIKeyScope(authResult.Scopes, requiredScope) {
			writeJSON(w, http.StatusForbidden, errorResponse{Error: "missing required scope: " + requiredScope})
			return
		}
	}
	target, err := joinBasePath(h.cfg.CorpusdURL, "/internal/computers/self-development/mode")
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to build self-development request"})
		return
	}
	u, err := url.Parse(target)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to build self-development request"})
		return
	}
	query := u.Query()
	query.Set("computer_id", computerID)
	u.RawQuery = query.Encode()
	upstream, err := http.NewRequestWithContext(r.Context(), r.Method, u.String(), http.MaxBytesReader(w, r.Body, 64<<10))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid self-development request"})
		return
	}
	upstream.Header.Set("X-Internal-Caller", "true")
	upstream.Header.Set("X-Authenticated-User", authResult.UserID)
	if r.Method == http.MethodPost {
		upstream.Header.Set("Content-Type", "application/json")
	}
	response, err := h.corpusd.Do(upstream)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "self-development authority unavailable"})
		return
	}
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "invalid self-development response"})
		return
	}
	if response.Header.Get("Content-Type") != "" {
		w.Header().Set("Content-Type", response.Header.Get("Content-Type"))
	}
	w.WriteHeader(response.StatusCode)
	_, _ = w.Write(body)
}
