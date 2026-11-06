package agentcore

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sort"

	"github.com/yusefmosiah/go-choir/internal/trust"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// GET /api/trust[?replay=1]: the owner's read of the derived trust view
// (modular architecture MA7.2, trust draft section 4): per subject and action
// class the Beta posterior, its lower credible bound, the evidence counts and
// the circuit-breaker flag, derived from the settled sub-call ledger on every
// request and stored nowhere. With replay=1 it adds the replay of settled
// records: both weight candidates' calibration and AUC on the next failure,
// the one the replay picks, and the act gate's rule replayed (gated acts and
// owner taps per day). It is read-only; nothing here grants or revokes (the
// lease policy and the act gate read their own derivation, leases.go and
// act_gate.go). The inputs include the breaker's trips (E3) and the owner's
// answers to gated acts (E6).

// trustLedgerWindow bounds the newest ledger records the view reads.
const trustLedgerWindow = 20000

// trustReplayMaxRecords bounds the replay, which is quadratic in the evidence.
const trustReplayMaxRecords = 3000

type trustResponse struct {
	ComputerID string              `json:"computer_id"`
	Window     string              `json:"window"`
	View       trust.View          `json:"view"`
	Replay     *trust.ReplayReport `json:"replay,omitempty"`
	// GateReplay is the act gate's rule replayed over the same records: the
	// material acts it would have gated and the owner taps per day (MA7.8).
	GateReplay *trust.GateReport `json:"gate_replay,omitempty"`
	// ReplayNote says when the replay read fewer records than the view.
	ReplayNote string `json:"replay_note,omitempty"`
}

// trustInputs reads the ledger window a trust view is derived from: the
// newest casts and settlements of the owner on the computer.
func (rt *Runtime) trustInputs(ctx context.Context, ownerID, computerID string) (trust.Inputs, error) {
	casts, err := rt.store.ListSubCallRecords(ctx, ownerID, computerID, types.SubCallCast, 5000)
	if err != nil {
		return trust.Inputs{}, fmt.Errorf("casts: %w", err)
	}
	settlements, err := rt.store.ListSubCallRecords(ctx, ownerID, computerID, types.SubCallSettlement, trustLedgerWindow)
	if err != nil {
		return trust.Inputs{}, fmt.Errorf("settlements: %w", err)
	}
	in := trust.Inputs{Records: append(casts, settlements...)}
	if err := rt.trustGateInputs(ctx, ownerID, computerID, &in); err != nil {
		return trust.Inputs{}, err
	}
	return in, nil
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
	in, err := rt.trustInputs(ctx, ownerID, computerID)
	if err != nil {
		log.Printf("runtime api: trust: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to read the sub-call ledger"})
		return
	}
	replayNote := ""
	replayIn := in
	if len(in.Records) > trustReplayMaxRecords {
		recs := append([]types.CommitmentRecord(nil), in.Records...)
		sort.SliceStable(recs, func(i, j int) bool { return recs[i].Provenance.CommittedAt > recs[j].Provenance.CommittedAt })
		replayIn.Records = recs[:trustReplayMaxRecords]
		replayNote = fmt.Sprintf("replay read the newest %d of %d records", trustReplayMaxRecords, len(in.Records))
	}
	resp := trustResponse{ComputerID: computerID, Window: "newest 5000 casts and 20000 settlements",
		View: trust.Derive(in, trust.DefaultParams())}
	if r.URL.Query().Get("replay") == "1" {
		rep := trust.Replay(replayIn, trust.DefaultParams())
		resp.Replay = &rep
		gate := trust.GateReplay(replayIn, trust.DefaultParams())
		resp.GateReplay = &gate
		resp.ReplayNote = replayNote
	}
	writeAPIJSON(w, http.StatusOK, resp)
}
