package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

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
// owner taps per day), the pre-registered units of the evidence harness
// (MA7.9) and the gated acts as the tape measured them. It is read-only;
// nothing here grants or revokes (the lease policy and the act gate read
// their own derivation, leases.go and
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
	// PreregUnits are the pre-registered units of P1 and P2 drawn from the
	// same records (MA7.9): predictors from before each weekly cutoff and the
	// label from its 14-day window. They are the evidence harness's input
	// (cmd/trustharness); no outcome metric is computed here.
	PreregUnits *trust.PreregUnitSet `json:"prereg_units,omitempty"`
	// GatedActs are the act gate's taps as measured on the disclosure tape:
	// when each was asked and how and when the owner answered (MA7.9 c).
	GatedActs []gatedActObservation `json:"gated_acts,omitempty"`
}

// gatedActObservation is one gated act on the disclosure tape, without its
// summary or target (the harness counts and times them; it reads no content).
type gatedActObservation struct {
	Desk       string            `json:"desk"`
	Class      trust.ActionClass `json:"class"`
	Breaker    bool              `json:"breaker,omitempty"`
	AskedAt    time.Time         `json:"asked_at"`
	Answer     string            `json:"answer,omitempty"`
	AnsweredAt *time.Time        `json:"answered_at,omitempty"`
}

// gatedActs reads the act gate's taps from the disclosure tape in time order.
func (rt *Runtime) gatedActs(ctx context.Context, owner, computer string) ([]gatedActObservation, error) {
	_, events, err := rt.disclosureFold(ctx, owner, computer)
	if err != nil {
		return nil, err
	}
	out := []gatedActObservation{}
	byPending := map[string]int{}
	for _, ev := range events {
		if ev.Kind != types.EventDisclosure {
			continue
		}
		var p disclosurePayload
		if json.Unmarshal(ev.Payload, &p) != nil || p.Sink != actGateSink {
			continue
		}
		switch ev.Phase {
		case disclosurePhasePending:
			if p.Gate == nil {
				continue
			}
			byPending[ev.EventID] = len(out)
			out = append(out, gatedActObservation{Desk: p.Desk, Class: p.Gate.Class, Breaker: p.Gate.Breaker, AskedAt: ev.Timestamp.UTC()})
		case disclosurePhaseApproved, disclosurePhaseDenied:
			i, ok := byPending[p.DisclosureID]
			if !ok || out[i].Answer != "" {
				continue
			}
			at := ev.Timestamp.UTC()
			out[i].Answer, out[i].AnsweredAt = ev.Phase, &at
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].AskedAt.Before(out[j].AskedAt) })
	return out, nil
}

// trustDeskCastWindow bounds the desk Casts the view reads back by id: the
// Casts of the newest desk_cast-plane settlements, one keyed read each.
const trustDeskCastWindow = 5000

// trustInputs reads the ledger window a trust view is derived from: the
// newest casts and settlements of the owner on the computer, and the desk
// Casts the desk-cast judge's settlements settle.
func (rt *Runtime) trustInputs(ctx context.Context, ownerID, computerID string) (trust.Inputs, error) {
	casts, err := rt.store.ListSubCallRecords(ctx, ownerID, computerID, types.SubCallCast, 5000)
	if err != nil {
		return trust.Inputs{}, fmt.Errorf("casts: %w", err)
	}
	settlements, err := rt.store.ListSubCallRecords(ctx, ownerID, computerID, types.SubCallSettlement, trustLedgerWindow)
	if err != nil {
		return trust.Inputs{}, fmt.Errorf("settlements: %w", err)
	}
	deskCasts, err := rt.trustDeskCasts(ctx, ownerID, computerID, settlements)
	if err != nil {
		return trust.Inputs{}, fmt.Errorf("desk casts: %w", err)
	}
	records := append(casts, settlements...)
	in := trust.Inputs{Records: append(records, deskCasts...)}
	if err := rt.trustGateInputs(ctx, ownerID, computerID, &in); err != nil {
		return trust.Inputs{}, err
	}
	return in, nil
}

// trustDeskCasts reads the desk-to-desk Casts the desk-cast judge settled
// (plane desk_cast), shaped as the judge read them (deskScoredCast): a desk
// Cast is an ordinary Cast directive with no sub-call body, so the sub-call
// list never returns it, and without it its settlement is evidence about no
// one. It reads the Casts of the newest trustDeskCastWindow settlements by
// record id. A Cast that does not read back, or is not a scorable desk Cast,
// is left out; the view then excludes its settlement as cast_missing.
func (rt *Runtime) trustDeskCasts(ctx context.Context, ownerID, computerID string, settlements []types.CommitmentRecord) ([]types.CommitmentRecord, error) {
	type want struct{ castID, key, at string }
	var wants []want
	seen := map[string]bool{}
	for _, s := range settlements {
		b, ok := s.SubCallBody(types.SubCallSettlement)
		if !ok || b.Plane != types.SubCallPlaneDeskCast || strings.TrimSpace(b.CastID) == "" || seen[b.CastID] {
			continue
		}
		seen[b.CastID] = true
		wants = append(wants, want{castID: b.CastID, key: b.FanOutID, at: s.Provenance.CommittedAt})
	}
	sort.SliceStable(wants, func(i, j int) bool { return wants[i].at > wants[j].at })
	if len(wants) > trustDeskCastWindow {
		wants = wants[:trustDeskCastWindow]
	}
	out := make([]types.CommitmentRecord, 0, len(wants))
	for _, w := range wants {
		rec, err := rt.store.GetCommitmentRecord(ctx, ownerID, computerID, w.castID)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", w.castID, err)
		}
		if rec == nil || !types.IsDeskCastScorable(*rec) {
			continue
		}
		out = append(out, deskScoredCast(*rec, w.key))
	}
	return out, nil
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
	resp := trustResponse{ComputerID: computerID, Window: "newest 5000 casts and 20000 settlements, and the desk Casts of the newest 5000 desk Cast settlements",
		View: trust.Derive(in, trust.DefaultParams())}
	if r.URL.Query().Get("replay") == "1" {
		rep := trust.Replay(replayIn, trust.DefaultParams())
		resp.Replay = &rep
		gate := trust.GateReplay(replayIn, trust.DefaultParams())
		resp.GateReplay = &gate
		resp.ReplayNote = replayNote
		units := trust.PreregUnits(replayIn, trust.DefaultParams())
		resp.PreregUnits = &units
		gated, err := rt.gatedActs(ctx, ownerID, computerID)
		if err != nil {
			log.Printf("runtime api: trust: gated acts: %v", err)
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to read the gated acts"})
			return
		}
		resp.GatedActs = gated
	}
	writeAPIJSON(w, http.StatusOK, resp)
}
