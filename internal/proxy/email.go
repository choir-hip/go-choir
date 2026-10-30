package proxy

import (
	"io"
	"net/http"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/hostauth"
)

// HandleEmailAPI handles authenticated /api/email/* routes. Webhook traffic is
// intentionally excluded by HandleAPI so raw Resend requests go directly to
// maild through Caddy instead of this auth/proxy path.
func (h *Handler) HandleEmailAPI(w http.ResponseWriter, r *http.Request) {
	h.forwardMaildAuthenticated(w, r)
}

func (h *Handler) forwardMaildAuthenticated(w http.ResponseWriter, r *http.Request) {
	authResult, err := h.authenticate(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		return
	}
	if !h.authorizeAPIKeyScope(w, r, authResult) {
		return
	}
	// Approving and sending is an owner act: it needs the owner's browser
	// session, never a headless API key, which an agent inside the computer
	// may hold (docs/archive/problems/maild-send-gate-defects-2026-10-14.md).
	if authResult.AuthMethod == "api_key" && isEmailOwnerActRoute(r) {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "owner session required: approving and sending mail needs the owner's signed-in session, not an API key"})
		return
	}
	target, err := joinBasePath(h.cfg.MaildURL, r.URL.Path)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "mail service is not configured"})
		return
	}
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}
	req, err := http.NewRequestWithContext(r.Context(), r.Method, target, r.Body)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to create mail request"})
		return
	}
	req.Header = r.Header.Clone()
	stripClientIdentityHeaders(req.Header)
	req.Header.Del("Authorization")
	req.Header.Del("Cookie")
	req.Header.Set("X-Authenticated-User", authResult.UserID)
	if authResult.Email != "" {
		req.Header.Set("X-Authenticated-Email", authResult.Email)
	}
	hostauth.MarkInternal(req)

	resp, err := h.maild.Do(req)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "mail service unavailable"})
		return
	}
	defer func() { _ = resp.Body.Close() }()
	copyResponseHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

// isEmailOwnerActRoute reports whether r is an email owner act that an API key
// must not perform: maild's approve-and-send route, POST
// /api/email/drafts/{id}/send, or a route that opts the owner in to or causes
// recurring mail, the Autopaper daily digest's PUT and DELETE
// /api/email/digest and POST /api/email/digest/send-now. Empty path segments
// are ignored, so every spelling maild could treat as the same route is caught
// (and a few it would not, which only refuses more). Reading the digest
// status stays open to an API key.
func isEmailOwnerActRoute(r *http.Request) bool {
	if r == nil || r.URL == nil || !strings.HasPrefix(r.URL.Path, "/api/email/") {
		return false
	}
	var parts []string
	for _, part := range strings.Split(strings.TrimPrefix(r.URL.Path, "/api/email/"), "/") {
		if part != "" {
			parts = append(parts, part)
		}
	}
	switch {
	case r.Method == http.MethodPost && len(parts) == 3 && parts[0] == "drafts" && parts[2] == "send":
		return true
	case (r.Method == http.MethodPut || r.Method == http.MethodDelete) && len(parts) == 1 && parts[0] == "digest":
		return true
	case r.Method == http.MethodPost && len(parts) == 2 && parts[0] == "digest" && parts[1] == "send-now":
		return true
	}
	return false
}

func stripClientIdentityHeaders(header http.Header) {
	for _, h := range clientIdentityHeaders {
		header.Del(h)
	}
}

func copyResponseHeaders(dst, src http.Header) {
	for k, values := range src {
		if strings.EqualFold(k, "Content-Length") {
			continue
		}
		for _, value := range values {
			dst.Add(k, value)
		}
	}
}
