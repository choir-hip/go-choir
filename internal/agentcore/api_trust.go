package agentcore

import (
	"log"
	"net/http"

	"github.com/yusefmosiah/go-choir/internal/trust"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// GET /api/trust[?replay=1]: the owner's read of the derived trust view
// (modular architecture MA7.2, trust draft section 4): per subject and action
// class the Beta posterior, its lower credible bound, the evidence counts and
// the circuit-breaker flag, derived from the settled sub-call ledger on every
// request and stored nowhere. With replay=1 it adds the replay of settled
// records: both weight candidates' calibration and AUC on the next failure,
// and the one the replay picks. It is read-only; nothing here grants or
// revokes (the lease policy and the act gate are later slices).

// trustLedgerWindow bounds the newest ledger records the view reads.
const trustLedgerWindow = 20000

type trustResponse struct {
	ComputerID string              `json:"computer_id"`
	Window     string              `json:"window"`
	View       trust.View          `json:"view"`
	Replay     *trust.ReplayReport `json:"replay,omitempty"`
}

// HandleTrust serves GET /api/trust.
func (h *APIHandler) HandleTrust(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	rt := h.rt
	if rt == nil || rt.store == nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "store unavailable"})
		return
	}
	ctx := r.Context()
	computerID := rt.TextureComputerID()
	casts, err := rt.store.ListSubCallRecords(ctx, ownerID, computerID, types.SubCallCast, 5000)
	if err != nil {
		log.Printf("runtime api: trust casts: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to read the sub-call ledger"})
		return
	}
	settlements, err := rt.store.ListSubCallRecords(ctx, ownerID, computerID, types.SubCallSettlement, trustLedgerWindow)
	if err != nil {
		log.Printf("runtime api: trust settlements: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to read the sub-call ledger"})
		return
	}
	in := trust.Inputs{Records: append(casts, settlements...)}
	resp := trustResponse{ComputerID: computerID, Window: "newest 5000 casts and 20000 settlements",
		View: trust.Derive(in, trust.DefaultParams())}
	if r.URL.Query().Get("replay") == "1" {
		rep := trust.Replay(in, trust.DefaultParams())
		resp.Replay = &rep
	}
	writeAPIJSON(w, http.StatusOK, resp)
}
