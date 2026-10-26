package proxy

import (
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// HandlePlatformEntityGrowth serves the entity lexicon's growth read to
// anyone (docs/problems/related-stories-no-entity-layer-2026-10-26.md, slice
// c0 §7). Like the front list it forwards to corpusd, which reads Store B
// alone; no computer is called. A corpusd error is a plain 502.
func (h *Handler) HandlePlatformEntityGrowth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	target, err := joinBasePath(h.cfg.CorpusdURL, "/internal/platform/entities/growth")
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to build platform request"})
		return
	}
	u, err := url.Parse(target)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to build platform request"})
		return
	}
	if days := strings.TrimSpace(r.URL.Query().Get("days")); days != "" {
		q := u.Query()
		q.Set("days", days)
		u.RawQuery = q.Encode()
	}
	var out types.EntityGrowthResponse
	status, err := h.getPlatformJSON(r, u.String(), &out)
	if err != nil {
		w.Header().Set("Cache-Control", "no-store")
		if platformErr, ok := err.(*platformStatusError); ok && platformErr.status == http.StatusBadRequest {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid growth request"})
			return
		}
		log.Printf("proxy: platform entity growth: %v", err)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to read entity growth"})
		return
	}
	w.Header().Set("Cache-Control", relatedCacheControl)
	writeJSON(w, status, out)
}
