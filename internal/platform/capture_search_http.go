package platform

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// Corpus search and URL lookup routes over the capture projection
// (capture_index.go). Both answer a bound guest for its own owner and
// computer, or an internal caller. CORPUSD_CAPTURE_SEARCH=off makes both
// refuse, which sends every guest back to the window of its newest
// captures: the host-side rollback switch.

// captureSearchEnv is corpusd's switch for the capture index routes.
const captureSearchEnv = "CORPUSD_CAPTURE_SEARCH"

func captureSearchDisabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(captureSearchEnv)), "off")
}

// ConfigureCaptureIndex gives the handler the platform store whose capture
// projection the search routes read.
func (h *ObjectGraphHandler) ConfigureCaptureIndex(store *Store) {
	if h != nil {
		h.captureIndex = store
	}
}

// captureIndexRefusal is the status and message of a route that cannot
// answer from the index; the guest falls back on any of them.
func (h *ObjectGraphHandler) captureIndexRefusal() (int, string) {
	switch {
	case captureSearchDisabled():
		return http.StatusServiceUnavailable, "capture search is off on this host"
	case h == nil || h.captureIndex == nil:
		return http.StatusServiceUnavailable, "capture index unavailable"
	}
	return 0, ""
}

// HandleCaptureSearch handles POST /internal/platform/captures/search.
func (h *ObjectGraphHandler) HandleCaptureSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	var req CaptureSearchRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}
	if strings.TrimSpace(req.OwnerID) == "" || strings.TrimSpace(req.ComputerID) == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "owner_id and computer_id are required"})
		return
	}
	if err := h.requireInternalOrBoundGuest(r, req.OwnerID, req.ComputerID, false); err != nil {
		writeJSON(w, http.StatusForbidden, apiError{Error: err.Error()})
		return
	}
	if status, msg := h.captureIndexRefusal(); status != 0 {
		writeJSON(w, status, apiError{Error: msg})
		return
	}
	resp, err := h.captureIndex.SearchCaptures(r.Context(), req)
	if err != nil {
		if errors.Is(err, ErrCaptureIndexIncomplete) {
			writeJSON(w, http.StatusServiceUnavailable, apiError{Error: err.Error()})
			return
		}
		log.Printf("corpusd: capture search for %s: %v", req.ComputerID, err)
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "capture search failed"})
		return
	}
	if resp.Objects == nil {
		resp.Objects = []objectgraph.Object{}
	}
	writeJSON(w, http.StatusOK, resp)
}

// HandleCaptureByURL handles GET /internal/platform/captures/by-url
// ?owner=&computer=&url=[&blocks=1].
func (h *ObjectGraphHandler) HandleCaptureByURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	q := r.URL.Query()
	owner, computer, pageURL := strings.TrimSpace(q.Get("owner")), strings.TrimSpace(q.Get("computer")), strings.TrimSpace(q.Get("url"))
	if owner == "" || computer == "" || pageURL == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "owner, computer and url are required"})
		return
	}
	if err := h.requireInternalOrBoundGuest(r, owner, computer, false); err != nil {
		writeJSON(w, http.StatusForbidden, apiError{Error: err.Error()})
		return
	}
	if status, msg := h.captureIndexRefusal(); status != 0 {
		writeJSON(w, status, apiError{Error: msg})
		return
	}
	res, err := h.captureIndex.CaptureByURL(r.Context(), owner, computer, pageURL, q.Get("blocks") == "1")
	if err != nil {
		if errors.Is(err, ErrCaptureIndexIncomplete) {
			writeJSON(w, http.StatusServiceUnavailable, apiError{Error: err.Error()})
			return
		}
		log.Printf("corpusd: capture by url for %s: %v", computer, err)
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "capture lookup failed"})
		return
	}
	writeJSON(w, http.StatusOK, res)
}
