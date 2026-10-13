package agentcore

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/researchtools"
)

// maxCaptureRouteIDs bounds the ids one captures request may name.
const maxCaptureRouteIDs = 500

// HandleCaptures serves GET /api/captures?since=&until=&id=...: this
// computer's captures created in the window (RFC3339 bounds, each
// optional) plus the captures named by id, each with its blocks' cids and
// normalized text. It is the trace bundle's captures layer (Cut 2
// amendment A5); captures live in the shared store behind corpusd, and the
// route only reads them.
func (h *APIHandler) HandleCaptures(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	q := r.URL.Query()
	var since, until time.Time
	for _, bound := range []struct {
		name string
		into *time.Time
	}{{"since", &since}, {"until", &until}} {
		raw := strings.TrimSpace(q.Get(bound.name))
		if raw == "" {
			continue
		}
		parsed, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: bound.name + " must be an RFC3339 time"})
			return
		}
		*bound.into = parsed
	}
	ids := q["id"]
	if len(ids) > maxCaptureRouteIDs {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "too many capture ids"})
		return
	}
	if h.rt == nil || h.rt.researchDeps == nil || h.rt.researchDeps.Captures == nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "the shared capture store is not configured"})
		return
	}
	out, err := researchtools.TraceCapturesInWindow(r.Context(), h.rt.researchDeps.Captures, ownerID, h.rt.TextureComputerID(), since, until, ids)
	if err != nil {
		log.Printf("runtime api: captures: %v", err)
		writeAPIJSON(w, http.StatusBadGateway, apiError{Error: "failed to read captures: " + err.Error()})
		return
	}
	writeAPIJSON(w, http.StatusOK, out)
}
