package agentcore

import (
	"net/http"
	"strconv"
	"strings"
)

// HandleInternalControlRecords serves GET /internal/runtime/control-records:
// the newest control packets on this computer with the record each one
// names, for the P1 staging audit that every Texture control is on the
// ledger (docs/definitions/choir-protocol-p1-record-first-controls-2026-10-11.md).
// Read-only and host-sourced only.
func (h *APIHandler) HandleInternalControlRecords(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if err := requireInternalRuntimeCaller(r); err != nil {
		writeAPIJSON(w, http.StatusForbidden, apiError{Error: "internal runtime endpoints are not publicly accessible"})
		return
	}
	ownerID := strings.TrimSpace(r.URL.Query().Get("owner_id"))
	if ownerID == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "owner_id is required"})
		return
	}
	limit := 200
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}
	rows, err := h.rt.store.ListControlRecordAudit(r.Context(), ownerID, h.rt.TextureComputerID(), limit)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
		return
	}
	writeAPIJSON(w, http.StatusOK, map[string]any{"controls": rows})
}
