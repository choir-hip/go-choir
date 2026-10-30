package agentcore

import (
	"net/http"

	"github.com/yusefmosiah/go-choir/internal/calibration"
)

// HandlePrecommit serves /api/precommit/ (calibration level 2,
// docs/archive/reports/choir-mcp-precommitments-2026-10-14.md): agents outside the
// desks write claims and settlements as commitment records on this
// computer and read their calibration. The proxy gates the prefix with the
// precommit:read / precommit:write API key scopes. It is not on
// management's ProductAPI allowlist; desks write their own records through
// their in-cell module surface.
func (h *APIHandler) HandlePrecommit(w http.ResponseWriter, r *http.Request) {
	rt := h.rt
	if rt == nil || rt.store == nil {
		http.Error(w, `{"error":"store unavailable"}`, http.StatusServiceUnavailable)
		return
	}
	handler := &calibration.Handler{
		Ledger:       rt.store,
		ComputerID:   rt.TextureComputerID,
		Authenticate: authenticateUser,
	}
	handler.ServeHTTP(w, r)
}
