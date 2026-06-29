package proxy

import (
	"io"
	"net/http"
	"strings"
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
	req.Header.Set("X-Internal-Caller", "true")

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
