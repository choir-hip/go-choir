package proxy

import (
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/platform"
)

// relatedCacheControl lets a browser or an intermediate cache absorb the
// reader's repeat opens; it matches corpusd's own five minute answer cache.
const relatedCacheControl = "public, max-age=300"

// HandlePlatformPublicationRelated serves a story's related stories to
// anyone, signed in or not (related-stories design 5.7). Like the front
// list it forwards to corpusd, which reads Store B alone: no computer is
// resolved or called, so the list works while the paper computer is
// stopped. A complete answer is cacheable; a partial one (the walk overran
// its budget) and the kill-switch answer (CORPUSD_RELATED=off) are not, so
// an empty list does not outlive the fault that made it. The reader treats
// any failure as "nothing to show", so a corpusd error is a plain 502.
func (h *Handler) HandlePlatformPublicationRelated(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	routePath := strings.TrimSpace(r.URL.Query().Get("route"))
	if routePath == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "route is required"})
		return
	}
	target, err := joinBasePath(h.cfg.CorpusdURL, "/internal/platform/publications/related")
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to build platform request"})
		return
	}
	u, err := url.Parse(target)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to build platform request"})
		return
	}
	q := u.Query()
	q.Set("route", routePath)
	if limit := strings.TrimSpace(r.URL.Query().Get("limit")); limit != "" {
		q.Set("limit", limit)
	}
	u.RawQuery = q.Encode()
	var out platform.RelatedStories
	status, err := h.getPlatformJSON(r, u.String(), &out)
	if err != nil {
		if platformErr, ok := err.(*platformStatusError); ok && platformErr.status == http.StatusBadRequest {
			w.Header().Set("Cache-Control", "no-store")
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid related request"})
			h.lifecycle.record("platform_related.list", lifecycleHTTPStatus(http.StatusBadRequest), time.Since(started))
			return
		}
		log.Printf("proxy: platform related stories: %v", err)
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to list related stories"})
		h.lifecycle.record("platform_related.list", "platform_error", time.Since(started))
		return
	}
	if out.Links == nil {
		out.Links = []platform.RelatedLink{}
	}
	if out.Partial || out.Basis.Mode == "off" {
		w.Header().Set("Cache-Control", "no-store")
	} else {
		w.Header().Set("Cache-Control", relatedCacheControl)
	}
	writeJSON(w, status, out)
	h.lifecycle.record("platform_related.list", lifecycleHTTPStatus(status), time.Since(started))
}
