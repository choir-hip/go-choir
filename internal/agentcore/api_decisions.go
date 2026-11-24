package agentcore

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// HandleDecisions serves GET /api/decisions?capture=&stage=&since=&limit=:
// the decision record's rows for this computer, newest first (R0). It is how
// an experiment asks "what was decided about this capture, by whom, from
// which candidates". Read only; the rows are written by host code, never
// through this route.
func (h *APIHandler) HandleDecisions(w http.ResponseWriter, r *http.Request) {
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
	f := types.DecisionFilter{CaptureID: q.Get("capture"), Stage: q.Get("stage")}
	if raw := strings.TrimSpace(q.Get("since")); raw != "" {
		since, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "since must be an RFC3339 time"})
			return
		}
		f.Since = since
	}
	if raw := strings.TrimSpace(q.Get("limit")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "limit must be a positive integer"})
			return
		}
		f.Limit = n
	}
	if h.rt == nil || h.rt.store == nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "the store is not configured"})
		return
	}
	rows, err := h.rt.store.ListDecisionRecords(r.Context(), ownerID, h.rt.TextureComputerID(), f)
	if err != nil {
		writeAPIJSON(w, http.StatusBadGateway, apiError{Error: "failed to read the decision record: " + err.Error()})
		return
	}
	writeAPIJSON(w, http.StatusOK, map[string]any{"computer_id": h.rt.TextureComputerID(), "decisions": rows})
}
