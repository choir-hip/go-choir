package apihandler

import (
	"net/http"
)

// HandlePluginsState handles GET /api/plugins/state: the Go kernel's snapshot
// (open scopes, pending effects, services, unit states and causes) for the
// authenticated owner. It is read-only and needs no SSH to inspect.
func (h *Handler) HandlePluginsState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if _, err := authenticateUser(r); err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	writeAPIJSON(w, http.StatusOK, h.kernel.Snapshot())
}
