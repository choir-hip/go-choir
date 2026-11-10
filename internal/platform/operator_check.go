package platform

import (
	"net/http"

	"github.com/yusefmosiah/go-choir/internal/hostauth"
)

// PlatformOperatorCheck is the answer of PlatformOperatorCheckRoute.
type PlatformOperatorCheck struct {
	Operator bool `json:"operator"`
}

// HandlePlatformOperatorCheck serves GET /internal/platform/operator/check
// ?user_id=<id>: host only (loopback and the host credential), exact match
// on the roster. An empty or malformed roster, or an id that is not exactly
// a roster entry (no case folding, no trimming), answers false.
func (h *Handler) HandlePlatformOperatorCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if h == nil || !trustedInternalTransport(r) || !hostauth.Authorize(r) {
		writeJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return
	}
	id := r.URL.Query().Get("user_id")
	writeJSON(w, http.StatusOK, PlatformOperatorCheck{Operator: platformOperatorUserRE.MatchString(id) && h.platformOperatorUsers[id]})
}
