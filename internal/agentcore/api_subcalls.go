package agentcore

import (
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// GET /api/subcalls[?limit=<n>]: the owner's read of the sub-call ledger
// (modular architecture MA1.5, MA1.6). Sub-call records are addressed to the
// constant "subcall:ledger" and bypass inbox heads, so /api/inboxes/ cannot
// show them. The route reads the newest records through the store's indexed
// addressee lookup (never a body scan), and adds the summary counts, the
// self-favor view and the inert rates. It is read-only and has no switch.

const (
	subCallsDefaultLimit = 100
	subCallsMaxLimit     = 500
	// subCallsSummaryWindow bounds the newest ledger records the counts read.
	subCallsSummaryWindow = 2000
)

type subCallsResponse struct {
	ComputerID string                   `json:"computer_id"`
	Limit      int                      `json:"limit"`
	Records    []types.CommitmentRecord `json:"records"`
	Summary    subCallsSummary          `json:"summary"`
	SelfFavor  subCallsSelfFavor        `json:"self_favor"`
	Inert      subCallsInert            `json:"inert"`
}

type subCallsSummary struct {
	// Window is how many of the newest ledger records the counts cover.
	Window      int            `json:"window"`
	WindowLimit int            `json:"window_limit"`
	ByKind      map[string]int `json:"by_kind"`
	ByClass     map[string]int `json:"settlement_by_class"`
	// ByPlane splits settlement_by_class by plane: "subcall" for sub-RLM
	// calls, "desk_cast" for desk-to-desk Casts (plan 4.4, amendment
	// 2026-11-05).
	ByPlane map[string]map[string]int `json:"settlement_by_plane"`
}

type subCallsSelfFavor struct {
	Groups []types.SelfFavorGroup `json:"groups"`
	Rows   []types.SelfFavorRow   `json:"rows"`
	// Window names the bound: the newest casts and settlements replayed.
	Window string `json:"window"`
}

type subCallsInert struct {
	Rates     []types.InertRateRow      `json:"rates"`
	Agreement types.InertJudgeAgreement `json:"judge_agreement"`
	Window    string                    `json:"window"`
}

func clampSubCallsLimit(raw string) int {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n <= 0 {
		return subCallsDefaultLimit
	}
	if n > subCallsMaxLimit {
		return subCallsMaxLimit
	}
	return n
}

// HandleSubCalls serves GET /api/subcalls.
func (h *APIHandler) HandleSubCalls(w http.ResponseWriter, r *http.Request) {
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
	limit := clampSubCallsLimit(r.URL.Query().Get("limit"))
	window := subCallsSummaryWindow
	if limit > window {
		window = limit
	}
	ledger, err := rt.store.ListCommitmentRecords(ctx, ownerID, computerID, types.SubCallLedgerAddressee, window)
	if err != nil {
		log.Printf("runtime api: subcalls ledger: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to read the sub-call ledger"})
		return
	}
	resp := subCallsResponse{
		ComputerID: computerID,
		Limit:      limit,
		Records:    []types.CommitmentRecord{},
		Summary: subCallsSummary{Window: len(ledger), WindowLimit: window,
			ByKind: map[string]int{}, ByClass: map[string]int{}, ByPlane: map[string]map[string]int{}},
		SelfFavor: subCallsSelfFavor{Groups: []types.SelfFavorGroup{}, Rows: []types.SelfFavorRow{},
			Window: "newest 5000 casts and 20000 settlements"},
		Inert: subCallsInert{Rates: []types.InertRateRow{},
			Window: "newest " + strconv.Itoa(calibrationListLimit) + " records and 20000 inert judgements"},
	}
	for i, rec := range ledger {
		if i < limit {
			resp.Records = append(resp.Records, rec)
		}
		b := rec.SubCall
		if b == nil {
			continue
		}
		resp.Summary.ByKind[b.Kind]++
		if b.Kind == types.SubCallSettlement {
			class := string(b.Class)
			if class == "" {
				class = "unclassed"
			}
			resp.Summary.ByClass[class]++
			plane := firstNonEmpty(b.Plane, "subcall")
			if resp.Summary.ByPlane[plane] == nil {
				resp.Summary.ByPlane[plane] = map[string]int{}
			}
			resp.Summary.ByPlane[plane][class]++
		}
	}
	groups, rows, err := rt.SubCallSelfFavor(ctx, ownerID, computerID)
	if err != nil {
		log.Printf("runtime api: subcalls self-favor: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to compute self-favor"})
		return
	}
	if groups != nil {
		resp.SelfFavor.Groups = groups
	}
	if rows != nil {
		resp.SelfFavor.Rows = rows
	}
	rates, agreement, err := rt.InertRates(ctx, ownerID, computerID)
	if err != nil {
		log.Printf("runtime api: subcalls inert rates: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to compute inert rates"})
		return
	}
	if rates != nil {
		resp.Inert.Rates = rates
	}
	resp.Inert.Agreement = agreement
	writeAPIJSON(w, http.StatusOK, resp)
}
