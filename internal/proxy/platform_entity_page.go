package proxy

import (
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// HandlePlatformEntityPage serves one entity of the Autopaper object graph to
// anyone, signed in or not (docs/design/entity-layer-2026-11-20.md section 7):
// GET /api/platform/entities/{id}[?stories=n]. Like the growth read it
// forwards to corpusd, which reads Store B alone; no computer is called. The
// id is checked for an entity id's shape here, so junk never reaches corpusd;
// corpusd bounds the rest and answers 404 for an unknown entity and for a
// person who has no page.
func (h *Handler) HandlePlatformEntityPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, types.EntityPagePrefix)
	if !types.EntityIDRE.MatchString(id) {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "no page for this entity"})
		return
	}
	target, err := joinBasePath(h.cfg.CorpusdURL, platform.EntityPageInternalPrefix+id)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to build platform request"})
		return
	}
	u, err := url.Parse(target)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to build platform request"})
		return
	}
	if stories := strings.TrimSpace(r.URL.Query().Get("stories")); stories != "" {
		q := u.Query()
		q.Set("stories", stories)
		u.RawQuery = q.Encode()
	}
	var out platform.EntityPage
	status, err := h.getPlatformJSON(r, u.String(), &out)
	if err != nil {
		if platformErr, ok := err.(*platformStatusError); ok {
			switch platformErr.status {
			case http.StatusNotFound:
				w.Header().Set("Cache-Control", "public, max-age=60")
				writeJSON(w, http.StatusNotFound, errorResponse{Error: "no page for this entity"})
				return
			case http.StatusBadRequest:
				w.Header().Set("Cache-Control", "no-store")
				writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid entity request"})
				return
			}
		}
		log.Printf("proxy: platform entity page: %v", err)
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to read the entity"})
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=60")
	writeJSON(w, status, out)
}
