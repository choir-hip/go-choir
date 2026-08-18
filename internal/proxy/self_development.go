package proxy

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func selfDevelopmentGuestRoute(path string) (computerID, action string, ok bool) {
	const prefix = "/api/computers/"
	if !strings.HasPrefix(path, prefix) {
		return "", "", false
	}
	rest := strings.TrimPrefix(path, prefix)
	rawID, after, found := strings.Cut(rest, "/self-development/")
	if !found || strings.TrimSpace(rawID) == "" || strings.Contains(rawID, "/") || strings.TrimSpace(after) == "" {
		return "", "", false
	}
	decoded, err := url.PathUnescape(rawID)
	if err != nil || strings.TrimSpace(decoded) == "" {
		return "", "", false
	}
	switch strings.TrimSpace(after) {
	case "mode", "replay-completeness", "genesis":
		return "", "", false
	case "operations":
		return strings.TrimSpace(decoded), "propose", true
	case "rollbacks":
		return strings.TrimSpace(decoded), "rollback", true
	case "kernel-capabilities":
		return strings.TrimSpace(decoded), "read", true
	}
	parts := strings.Split(strings.Trim(after, "/"), "/")
	if len(parts) == 2 && parts[0] == "operations" && strings.TrimSpace(parts[1]) != "" {
		return strings.TrimSpace(decoded), "read", true
	}
	if len(parts) == 3 && parts[0] == "operations" && strings.TrimSpace(parts[1]) != "" && parts[2] == "decision" {
		return strings.TrimSpace(decoded), "approve", true
	}
	if len(parts) == 3 && parts[0] == "operations" && strings.TrimSpace(parts[1]) != "" && parts[2] == "receipts" {
		return strings.TrimSpace(decoded), "read", true
	}
	return "", "", false
}

func isSelfDevelopmentGuestPath(path string) bool {
	_, _, ok := selfDevelopmentGuestRoute(path)
	return ok
}

func selfDevelopmentGuestMethodAllowed(method, action string) bool {
	switch action {
	case "propose", "approve", "rollback":
		return method == http.MethodPost
	case "read":
		return method == http.MethodGet
	default:
		return false
	}
}

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
func selfDevelopmentReplayCompletenessComputerID(path string) (string, bool) {
	const prefix = "/api/computers/"
	const suffix = "/self-development/replay-completeness"
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

func isSelfDevelopmentReplayCompletenessPath(path string) bool {
	_, ok := selfDevelopmentReplayCompletenessComputerID(path)
	return ok
}

func (h *Handler) HandleSelfDevelopmentMode(w http.ResponseWriter, r *http.Request) {
	computerID, ok := selfDevelopmentModeComputerID(r.URL.Path)
	if !ok {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "not found"})
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodPut {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	authResult, err := h.authenticate(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		return
	}
	requiredScope := "computer:self_development:read"
	if r.Method == http.MethodPut {
		requiredScope = "computer:self_development:mode"
	}
	if authResult.AuthMethod == "api_key" {
		if !hasAPIKeyScope(authResult.Scopes, "admin") && !hasAPIKeyScope(authResult.Scopes, requiredScope) {
			writeJSON(w, http.StatusForbidden, errorResponse{Error: "missing required scope: " + requiredScope})
			return
		}
		if _, ok := h.requireAPIKeyComputerTarget(w, r, authResult, computerID, ""); !ok {
			return
		}
	} else {
		if h.vmctlClient == nil {
			writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "computer ownership authority unavailable"})
			return
		}
		ownership, ownershipErr := h.vmctlClient.LookupComputerContext(r.Context(), authResult.UserID, computerID)
		if ownershipErr != nil || ownership == nil || ownership.ComputerID != computerID {
			writeJSON(w, http.StatusForbidden, errorResponse{Error: "computer ownership required"})
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
	upstreamMethod := r.Method
	if upstreamMethod == http.MethodPut {
		upstreamMethod = http.MethodPost
	}
	upstream, err := http.NewRequestWithContext(r.Context(), upstreamMethod, u.String(), http.MaxBytesReader(w, r.Body, 64<<10))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid self-development request"})
		return
	}
	upstream.Header.Set("X-Internal-Caller", "true")
	upstream.Header.Set("X-Authenticated-User", authResult.UserID)
	if r.Method == http.MethodPut {
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

const replayCompletenessWriteGrace = time.Second

// extendReplayCompletenessWriteDeadline gives the owner-only replay probe a
// route-local response budget. The proxy server's default write deadline must
// remain short for ordinary interactive routes; ResponseController lets this
// one long-running route extend its connection deadline without weakening that
// global fail-fast boundary. Test response writers may not support deadlines,
// so an unsupported writer is left at its existing deadline.
func (h *Handler) extendReplayCompletenessWriteDeadline(w http.ResponseWriter) {
	timeout := DefaultReplayCompletenessTimeout
	if h != nil && h.replayAutoputerHTTP != nil && h.replayAutoputerHTTP.Timeout > 0 {
		timeout = h.replayAutoputerHTTP.Timeout
	} else if h != nil && h.cfg != nil && h.cfg.ReplayCompletenessTimeout > 0 {
		timeout = h.cfg.ReplayCompletenessTimeout
	}
	if timeout <= 0 {
		return
	}
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(timeout + replayCompletenessWriteGrace))
}

func (h *Handler) HandleSelfDevelopmentReplayCompleteness(w http.ResponseWriter, r *http.Request) {
	computerID, ok := selfDevelopmentReplayCompletenessComputerID(r.URL.Path)
	if !ok {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "not found"})
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	authResult, err := h.authenticate(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		return
	}
	if authResult.AuthMethod == "api_key" {
		const requiredScope = "computer:self_development:read"
		if !hasAPIKeyScope(authResult.Scopes, "admin") && !hasAPIKeyScope(authResult.Scopes, requiredScope) {
			writeJSON(w, http.StatusForbidden, errorResponse{Error: "missing required scope: " + requiredScope})
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
	} else {
		err = fmt.Errorf("computer realization is not active")
	}
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "computer authority unavailable"})
		return
	}

	targetURL, err := joinBasePath(autoputerURL, r.URL.Path)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to build replay completeness request"})
		return
	}
	u, err := url.Parse(targetURL)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to build replay completeness request"})
		return
	}
	u.RawQuery = r.URL.RawQuery
	upstream, err := http.NewRequestWithContext(r.Context(), http.MethodGet, u.String(), nil)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid replay completeness request"})
		return
	}
	upstream.Header.Set("X-Authenticated-User", authResult.UserID)
	upstream.Header.Set("X-Authenticated-Computer", computerID)
	if authResult.Email != "" {
		upstream.Header.Set("X-Authenticated-Email", authResult.Email)
	}
	if len(authResult.Scopes) > 0 {
		upstream.Header.Set("X-Authenticated-Scopes", strings.Join(authResult.Scopes, ","))
	}
	client := h.replayAutoputerHTTP
	if client == nil {
		client = &http.Client{Timeout: DefaultReplayCompletenessTimeout}
	}
	h.extendReplayCompletenessWriteDeadline(w)
	response, err := client.Do(upstream)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "replay completeness authority unavailable"})
		return
	}
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "invalid replay completeness response"})
		return
	}
	if contentType := response.Header.Get("Content-Type"); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.WriteHeader(response.StatusCode)
	_, _ = w.Write(body)
}

func (h *Handler) HandleSelfDevelopmentGuest(w http.ResponseWriter, r *http.Request) {
	computerID, action, ok := selfDevelopmentGuestRoute(r.URL.Path)
	if !ok {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "not found"})
		return
	}
	if !selfDevelopmentGuestMethodAllowed(r.Method, action) {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	authResult, err := h.authenticate(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		return
	}
	requiredScope := "computer:self_development:" + action
	if authResult.AuthMethod == "api_key" {
		if !hasAPIKeyScope(authResult.Scopes, "admin") && !hasAPIKeyScope(authResult.Scopes, requiredScope) {
			writeJSON(w, http.StatusForbidden, errorResponse{Error: "missing required scope: " + requiredScope})
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
	} else {
		err = fmt.Errorf("computer realization is not active")
	}
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "computer authority unavailable"})
		return
	}

	targetURL, err := joinBasePath(autoputerURL, r.URL.Path)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to build self-development request"})
		return
	}
	u, err := url.Parse(targetURL)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to build self-development request"})
		return
	}
	u.RawQuery = r.URL.RawQuery
	maxBody := int64(64 << 10)
	if action == "propose" || action == "approve" {
		maxBody = 1 << 20
	}
	upstream, err := http.NewRequestWithContext(r.Context(), r.Method, u.String(), http.MaxBytesReader(w, r.Body, maxBody))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid self-development request"})
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
	response, err := client.Do(upstream)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "self-development authority unavailable"})
		return
	}
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "invalid self-development response"})
		return
	}
	if contentType := response.Header.Get("Content-Type"); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.WriteHeader(response.StatusCode)
	_, _ = w.Write(body)
}
