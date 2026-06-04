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
	"time"

	"github.com/yusefmosiah/go-choir/internal/platform"
)

type publishVTextRequest struct {
	DocID      string `json:"doc_id"`
	RevisionID string `json:"revision_id,omitempty"`
	Slug       string `json:"slug,omitempty"`
}

type sandboxVTextDocument struct {
	DocID             string `json:"doc_id"`
	OwnerID           string `json:"owner_id"`
	Title             string `json:"title"`
	CurrentRevisionID string `json:"current_revision_id,omitempty"`
}

type sandboxVTextRevision struct {
	RevisionID string          `json:"revision_id"`
	DocID      string          `json:"doc_id"`
	OwnerID    string          `json:"owner_id"`
	Content    string          `json:"content"`
	Citations  json.RawMessage `json:"citations,omitempty"`
	Metadata   json.RawMessage `json:"metadata,omitempty"`
}

func (h *Handler) HandleVTextPublication(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		h.lifecycle.record("platform_publish.method", "method_not_allowed", time.Since(started))
		return
	}

	authStarted := time.Now()
	authResult, err := h.validateAccessJWT(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		h.lifecycle.record("platform_publish.auth", "unauthorized", time.Since(authStarted))
		h.lifecycle.record("platform_publish.total", "unauthorized", time.Since(started))
		return
	}
	h.lifecycle.record("platform_publish.auth", "ok", time.Since(authStarted))

	var req publishVTextRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request body"})
		h.lifecycle.record("platform_publish.total", "bad_request", time.Since(started))
		return
	}
	req.DocID = strings.TrimSpace(req.DocID)
	req.RevisionID = strings.TrimSpace(req.RevisionID)
	req.Slug = strings.TrimSpace(req.Slug)
	if req.DocID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "doc_id is required"})
		h.lifecycle.record("platform_publish.total", "bad_request", time.Since(started))
		return
	}

	desktopID := requestDesktopID(r)
	resolveStarted := time.Now()
	sandboxURL, err := h.resolveSandboxURL(r.Context(), authResult.UserID, desktopID)
	if err != nil {
		log.Printf("proxy: platform publish failed to resolve sandbox for user %s desktop %s: %v", authResult.UserID, desktopID, err)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to resolve user sandbox"})
		h.lifecycle.record("platform_publish.resolve", "error", time.Since(resolveStarted))
		h.lifecycle.record("platform_publish.total", "resolve_error", time.Since(started))
		return
	}
	h.lifecycle.record("platform_publish.resolve", "ok", time.Since(resolveStarted))

	var doc sandboxVTextDocument
	if err := h.fetchSandboxJSON(r, sandboxURL, "/api/vtext/documents/"+url.PathEscape(req.DocID), authResult.UserID, &doc); err != nil {
		log.Printf("proxy: platform publish fetch document: %v", err)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to load private vtext document"})
		h.lifecycle.record("platform_publish.private_read", "document_error", time.Since(started))
		return
	}
	if doc.OwnerID != authResult.UserID || doc.DocID != req.DocID {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "document does not belong to authenticated user"})
		h.lifecycle.record("platform_publish.private_read", "owner_mismatch", time.Since(started))
		return
	}
	if req.RevisionID == "" {
		req.RevisionID = doc.CurrentRevisionID
	}
	if req.RevisionID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "document has no revision to publish"})
		h.lifecycle.record("platform_publish.total", "bad_request", time.Since(started))
		return
	}

	var rev sandboxVTextRevision
	if err := h.fetchSandboxJSON(r, sandboxURL, "/api/vtext/revisions/"+url.PathEscape(req.RevisionID), authResult.UserID, &rev); err != nil {
		log.Printf("proxy: platform publish fetch revision: %v", err)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to load private vtext revision"})
		h.lifecycle.record("platform_publish.private_read", "revision_error", time.Since(started))
		return
	}
	if rev.OwnerID != authResult.UserID || rev.DocID != req.DocID || rev.RevisionID != req.RevisionID {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "revision does not belong to authenticated document"})
		h.lifecycle.record("platform_publish.private_read", "revision_mismatch", time.Since(started))
		return
	}
	h.lifecycle.record("platform_publish.private_read", "ok", time.Since(started))

	platformReq := platform.PublishVTextRequest{
		OwnerID:          authResult.UserID,
		SourceDocID:      doc.DocID,
		SourceRevisionID: rev.RevisionID,
		Title:            doc.Title,
		Content:          rev.Content,
		Citations:        rev.Citations,
		Metadata:         rev.Metadata,
		Slug:             req.Slug,
		RequestedBy:      authResult.UserID,
	}
	platformResp, status, err := h.postPlatformPublication(r, platformReq)
	if err != nil {
		log.Printf("proxy: platform publish post platformd: %v", err)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to publish vtext"})
		h.lifecycle.record("platform_publish.platformd", "error", time.Since(started))
		h.lifecycle.record("platform_publish.total", "platform_error", time.Since(started))
		return
	}
	if status < 200 || status >= 300 {
		writeJSON(w, status, platformResp)
		h.lifecycle.record("platform_publish.platformd", lifecycleHTTPStatus(status), time.Since(started))
		h.lifecycle.record("platform_publish.total", lifecycleHTTPStatus(status), time.Since(started))
		return
	}
	if resp, ok := platformResp.(*platform.PublishVTextResponse); ok && resp.RoutePath != "" {
		resp.PublicURL = publicURLForRoute(r, resp.RoutePath)
	}
	writeJSON(w, status, platformResp)
	h.lifecycle.record("platform_publish.platformd", lifecycleHTTPStatus(status), time.Since(started))
	h.lifecycle.record("platform_publish.total", "published", time.Since(started))
}

func (h *Handler) fetchSandboxJSON(r *http.Request, sandboxBase, path, userID string, out any) error {
	target, err := joinBasePath(sandboxBase, path)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, target, nil)
	if err != nil {
		return fmt.Errorf("build sandbox request: %w", err)
	}
	req.Header.Set("X-Authenticated-User", userID)
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("call sandbox: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("sandbox status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode sandbox response: %w", err)
	}
	return nil
}

func (h *Handler) postPlatformPublication(r *http.Request, req platform.PublishVTextRequest) (any, int, error) {
	target, err := joinBasePath(h.cfg.PlatformdURL, "/internal/platform/publications/vtext")
	if err != nil {
		return nil, 0, err
	}
	data, err := json.Marshal(req)
	if err != nil {
		return nil, 0, fmt.Errorf("marshal platform request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, target, bytes.NewReader(data))
	if err != nil {
		return nil, 0, fmt.Errorf("build platform request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Internal-Caller", "true")
	resp, err := h.platformd.Do(httpReq)
	if err != nil {
		return nil, 0, fmt.Errorf("call platformd: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read platformd response: %w", err)
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var out platform.PublishVTextResponse
		if err := json.Unmarshal(body, &out); err != nil {
			return nil, resp.StatusCode, fmt.Errorf("decode platformd response: %w", err)
		}
		return &out, resp.StatusCode, nil
	}
	var out errorResponse
	if err := json.Unmarshal(body, &out); err != nil || out.Error == "" {
		out.Error = strings.TrimSpace(string(body))
		if out.Error == "" {
			out.Error = fmt.Sprintf("platformd status %d", resp.StatusCode)
		}
	}
	return out, resp.StatusCode, nil
}

func joinBasePath(rawBase, path string) (string, error) {
	u, err := url.Parse(strings.TrimRight(rawBase, "/"))
	if err != nil {
		return "", fmt.Errorf("parse base URL: %w", err)
	}
	u.Path = "/" + strings.TrimLeft(path, "/")
	u.RawQuery = ""
	return u.String(), nil
}

func publicURLForRoute(r *http.Request, routePath string) string {
	proto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto"))
	if proto == "" {
		proto = "https"
		if r.TLS == nil && strings.HasPrefix(r.Host, "127.0.0.1") {
			proto = "http"
		}
	}
	host := strings.TrimSpace(r.Host)
	if host == "" {
		return routePath
	}
	return proto + "://" + host + "/" + strings.TrimLeft(routePath, "/")
}
