package proxy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/hostauth"
)

// Host calls shared by the package routes: the owner's computer, the guest,
// and corpusd. (The software release routes these served first, P7, are
// deleted: docs/design/package-install-2026-11-04.md.)

const releaseReadScope = "computer:self_development:read"

const releasePublishScope = "computer:self_development:approve"

func (h *Handler) ownedComputerURL(w http.ResponseWriter, r *http.Request, authResult *AuthResult, computerID string) (string, bool) {
	var target *resolvedComputerTarget
	var err error
	if authResult.AuthMethod == "api_key" {
		var ok bool
		target, ok = h.requireAPIKeyComputerTarget(w, r, authResult, computerID, "")
		if !ok {
			return "", false
		}
		guestURL, err := h.resolveActiveComputerURLForComputerTarget(r.Context(), authResult, target, target.DesktopID)
		if err != nil {
			logComputerAuthorityFailure(r, computerID, target, err)
			writeAutoputerResolveFailure(w, http.StatusBadGateway, "computer authority unavailable", err)
			return "", false
		}
		return guestURL, true
	}
	target, err = h.resolveAuthorizedComputer(r.Context(), authResult, computerID)
	if err != nil || target == nil || target.ComputerID != computerID || target.UserID != authResult.UserID {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "computer ownership required"})
		return "", false
	}
	if target.State != "active" || strings.TrimSpace(target.ComputerURL) == "" {
		logComputerAuthorityFailure(r, computerID, target, nil)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "computer authority unavailable"})
		return "", false
	}
	return target.ComputerURL, true
}

func (h *Handler) guestJSON(ctx context.Context, guestURL, method, path string, authResult *AuthResult, computerID string, body []byte) (int, []byte, error) {
	return h.guestJSONLimit(ctx, guestURL, method, path, authResult, computerID, body, 4<<20)
}

func (h *Handler) guestJSONLimit(ctx context.Context, guestURL, method, path string, authResult *AuthResult, computerID string, body []byte, limit int64) (int, []byte, error) {
	target, err := joinBasePath(guestURL, strings.SplitN(path, "?", 2)[0])
	if err != nil {
		return 0, nil, err
	}
	if parts := strings.SplitN(path, "?", 2); len(parts) == 2 {
		target += "?" + parts[1]
	}
	request, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	request.Header.Set("X-Authenticated-User", authResult.UserID)
	request.Header.Set("X-Authenticated-Computer", computerID)
	if authResult.Email != "" {
		request.Header.Set("X-Authenticated-Email", authResult.Email)
	}
	if len(authResult.Scopes) > 0 {
		request.Header.Set("X-Authenticated-Scopes", strings.Join(authResult.Scopes, ","))
	}
	client := h.autoputerHTTP
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = response.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(response.Body, limit))
	return response.StatusCode, raw, err
}

func (h *Handler) corpusdCall(ctx context.Context, method, path, contentType string, body []byte) (int, []byte, error) {
	return h.corpusdCallHeaders(ctx, method, path, contentType, nil, body)
}

func (h *Handler) corpusdCallHeaders(ctx context.Context, method, path, contentType string, headers map[string]string, body []byte) (int, []byte, error) {
	base, err := joinBasePath(h.cfg.CorpusdURL, strings.SplitN(path, "?", 2)[0])
	if err != nil {
		return 0, nil, err
	}
	if parts := strings.SplitN(path, "?", 2); len(parts) == 2 && parts[1] != "" {
		base += "?" + parts[1]
	}
	request, err := http.NewRequestWithContext(ctx, method, base, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	hostauth.MarkInternal(request)
	client := h.corpusd
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return 0, nil, fmt.Errorf("call corpusd: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	return response.StatusCode, raw, err
}

func (h *Handler) corpusdFetch(ctx context.Context, path string, limit int64) (int, string, []byte, error) {
	base, err := joinBasePath(h.cfg.CorpusdURL, strings.SplitN(path, "?", 2)[0])
	if err != nil {
		return 0, "", nil, err
	}
	if parts := strings.SplitN(path, "?", 2); len(parts) == 2 && parts[1] != "" {
		base += "?" + parts[1]
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, base, nil)
	if err != nil {
		return 0, "", nil, err
	}
	hostauth.MarkInternal(request)
	client := h.corpusd
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return 0, "", nil, fmt.Errorf("call corpusd: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(response.Body, limit))
	return response.StatusCode, response.Header.Get("Content-Type"), raw, err
}

func relayJSON(w http.ResponseWriter, status int, raw []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(raw)
}
