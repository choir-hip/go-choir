package proxy

import (
	"log"
	"net/http"
	"time"

	"github.com/yusefmosiah/go-choir/internal/platform"
)

// HandleUniversalWireStories serves the authenticated product route directly
// from corpusd. It intentionally does not resolve, start, or contact a user VM.
func (h *Handler) HandleUniversalWireStories(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	authResult, err := h.authenticate(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		h.lifecycle.record("universal_wire.auth", "unauthorized", time.Since(started))
		return
	}
	if !h.authorizeAPIKeyScope(w, r, authResult) {
		h.lifecycle.record("universal_wire.authz", "forbidden", time.Since(started))
		return
	}
	target, err := joinBasePath(h.cfg.CorpusdURL, "/internal/platform/universal-wire/stories")
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to build Universal Wire request"})
		return
	}
	var out platform.UniversalWireStoriesResponse
	status, err := h.getPlatformJSON(r, target, &out)
	if err != nil {
		log.Printf("proxy: universal wire stories: %v", err)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to load Universal Wire stories"})
		h.lifecycle.record("universal_wire.upstream", "error", time.Since(started))
		return
	}
	writeJSON(w, status, out)
	h.lifecycle.record("universal_wire.total", lifecycleHTTPStatus(status), time.Since(started))
}
