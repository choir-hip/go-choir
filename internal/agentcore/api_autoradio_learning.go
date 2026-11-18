package agentcore

// Station 3: GET /api/autoradio/learning, the listener's own read of what the
// desk has learned about them (learning engine rule 2: nothing is held about
// the listener that the listener cannot read). It shows the scorecard of the
// DJ's forecasts (calibration.Reader.Scorecard: the same Summarize as every
// other calibration read) and the ledger rows: forecast, entry title,
// outcome, when it was settled and the op that settled it, or the
// no-verdict record. Authentication is the sheet's. Read only.

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/calibration"
	"github.com/yusefmosiah/go-choir/internal/runsheet"
	"github.com/yusefmosiah/go-choir/internal/types"
)

const (
	learningDefaultRows = 50
	learningMaxRows     = 200
	// learningMaxNoVerdicts bounds the no-verdict records one read lists.
	learningMaxNoVerdicts = 5000
)

type learningOp struct {
	OpID            string  `json:"op_id"`
	By              string  `json:"by,omitempty"`
	Outcome         string  `json:"outcome,omitempty"`
	LastBlock       int     `json:"last_block"`
	EndedAt         string  `json:"ended_at,omitempty"`
	PositionSeconds float64 `json:"position_seconds"`
	// PositionSource says where position_seconds came from: "op" (the phone
	// said it) or "sheet" (the sheet's own position when the op applied).
	PositionSource string `json:"position_source,omitempty"`
	// Cutting is true when the evidence op is the listener's place or move at
	// now that cut this entry: the op is the cutting op, not the entry's own.
	Cutting bool  `json:"cutting,omitempty"`
	Version int64 `json:"version,omitempty"`
}

type learningRow struct {
	ClaimID    string      `json:"claim_id"`
	StretchID  string      `json:"stretch_id"`
	SegmentID  string      `json:"segment_id"`
	EntryID    string      `json:"entry_id"`
	Kind       string      `json:"kind"`
	Title      string      `json:"title"`
	TakeID     string      `json:"take_id,omitempty"` // the take a take forecast was about
	PHeard     float64     `json:"p_heard"`
	CausedBy   string      `json:"caused_by"`
	ForecastAt string      `json:"forecast_at"`
	Deadline   string      `json:"deadline"`
	Status     string      `json:"status"` // settled, no_verdict, open or overdue
	Outcome    string      `json:"outcome,omitempty"`
	Heard      *bool       `json:"heard_through,omitempty"`
	Brier      *float64    `json:"brier,omitempty"`
	SettledAt  string      `json:"settled_at,omitempty"`
	Reason     string      `json:"no_verdict_reason,omitempty"`
	Op         *learningOp `json:"op,omitempty"`
}

type learningCard struct {
	Forecasts        int                      `json:"forecasts"`
	Settled          int                      `json:"settled"`
	HeardThrough     int                      `json:"heard_through"`
	HeardThroughRate *float64                 `json:"heard_through_rate"`
	Brier            *float64                 `json:"brier"`
	Open             int                      `json:"open"`
	Overdue          int                      `json:"overdue"`
	NoVerdict        int                      `json:"no_verdict"`
	NoVerdictReasons map[string]int           `json:"no_verdict_reasons"`
	ByKind           []calibration.GroupStats `json:"by_kind"`
	ByCause          []calibration.CauseStats `json:"by_cause"`
}

// HandleAutoradioLearning is GET /api/autoradio/learning?limit=<n>&after=<claim id>.
func (h *APIHandler) HandleAutoradioLearning(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := h.sheetOwner(w, r, http.MethodGet)
	if !ok {
		return
	}
	limit := learningDefaultRows
	if raw := r.URL.Query().Get("limit"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v < 1 || v > learningMaxRows {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "limit: a number from 1 to " + strconv.Itoa(learningMaxRows)})
			return
		}
		limit = v
	}
	after := r.URL.Query().Get("after")
	line := r.URL.Query().Get("line") // behind a line: the forecasts it caused (design 7.3)
	ctx := r.Context()
	computer := h.rt.cfg.ComputerID
	now := time.Now()
	card, err := calibration.Reader{Ledger: h.rt.store}.Scorecard(ctx, ownerID, computer, now)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "cannot read the forecasts"})
		return
	}
	led := h.programmeLedger()
	nvs, _, _, err := led.ListCommitmentRecordsBySubjectRefOfComputer(ctx, ownerID, computer, "record_kind", kindNoVerdict, learningMaxNoVerdicts)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "cannot read the no-verdict records"})
		return
	}
	// An `unplaced` notice is stale once the entry is on the sheet (a replay
	// placed it): the live sheet wins over it.
	onSheet := map[string]bool{}
	if ls, serr := h.rt.sheetStore().get(ctx, h, ownerID, sheetOwnerID); serr == nil {
		ls.mu.Lock()
		for _, e := range ls.sheet.Entries {
			onSheet[e.ID] = true
		}
		ls.mu.Unlock()
	}
	nvByClaim := map[string]types.CommitmentRecord{}
	reasons := map[string]int{}
	for _, nv := range nvs {
		id := nv.SubjectRefs["claim_id"]
		if id == "" || (nv.SubjectRefs["reason"] == noVerdictUnplaced && onSheet[nv.SubjectRefs["entry_id"]]) {
			continue
		}
		if prev, ok := nvByClaim[id]; !ok || nv.Provenance.CommittedAt > prev.Provenance.CommittedAt {
			nvByClaim[id] = nv
		}
	}
	cs := learningCard{
		Forecasts: card.Overall.Claims, Settled: card.Overall.Resolved, Brier: card.Overall.Brier, Open: card.Overall.Open,
		Overdue: card.Overall.Overdue, NoVerdictReasons: reasons, ByKind: card.ByKind, ByCause: card.ByCause,
	}
	if cs.ByKind == nil {
		cs.ByKind = []calibration.GroupStats{}
	}
	if cs.ByCause == nil {
		cs.ByCause = []calibration.CauseStats{}
	}
	if card.Overall.BaseRate != nil {
		cs.HeardThroughRate = card.Overall.BaseRate
		cs.HeardThrough = int(*card.Overall.BaseRate*float64(card.Overall.Resolved) + 0.5)
	}
	for _, v := range card.Claims {
		if nv, ok := nvByClaim[v.ID]; ok && v.Status != calibration.StatusSettled {
			cs.NoVerdict++
			reasons[nv.SubjectRefs["reason"]]++
		}
	}

	if line != "" {
		var keep []calibration.ClaimView
		for _, v := range card.Claims {
			if strings.TrimPrefix(v.Criterion, "caused_by=") == line {
				keep = append(keep, v)
			}
		}
		card.Claims = keep
	}
	start := 0
	if after != "" {
		start = -1
		for i, v := range card.Claims {
			if v.ID == after {
				start = i + 1
				break
			}
		}
		if start < 0 {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "after: not a forecast of this listener"})
			return
		}
	}
	end := start + limit
	if end > len(card.Claims) {
		end = len(card.Claims)
	}
	rows := make([]learningRow, 0, end-start)
	for _, v := range card.Claims[start:end] {
		row, rerr := h.learningRow(ctx, led, ownerID, computer, v, nvByClaim)
		if rerr != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "cannot read a ledger row"})
			return
		}
		rows = append(rows, row)
	}
	ans := map[string]any{"scorecard": cs, "rows": rows, "truncated": card.Truncated || len(nvs) >= learningMaxNoVerdicts, "notebook": h.learningNotebook(ctx, ownerID, led)}
	if end < len(card.Claims) && end > start {
		ans["next"] = card.Claims[end-1].ID
	}
	writeAPIJSON(w, http.StatusOK, ans)
}

func (h *APIHandler) learningRow(ctx context.Context, led autoradioLedger, ownerID, computer string, v calibration.ClaimView, nvByClaim map[string]types.CommitmentRecord) (learningRow, error) {
	row := learningRow{
		ClaimID: v.ID, PHeard: v.P, ForecastAt: v.RecordedAt, Deadline: v.Deadline, Status: v.Status,
		Kind: v.Project, StretchID: strings.TrimPrefix(tagOf(v.Tags, "stretch:"), ""), SegmentID: tagOf(v.Tags, "seg:"),
		CausedBy: strings.TrimPrefix(v.Criterion, "caused_by="),
	}
	row.EntryID = programmeEntryID(row.SegmentID)
	row.TakeID = tagOf(v.Tags, "take:")
	if seg, err := led.GetCommitmentRecord(ctx, ownerID, computer, segmentRecordID(row.StretchID, row.SegmentID)); err != nil {
		return row, err
	} else if seg != nil && seg.Directive != nil {
		row.Title = seg.Directive.Objective
	}
	opID := ""
	if g, ok := calibration.Governing(v.Settlements, ""); ok {
		row.Status, row.Outcome, row.Brier, row.SettledAt = "settled", g.Outcome, g.Brier, g.SettledAt
		t := g.Outcome == "true"
		row.Heard = &t
		for _, ev := range g.Evidence {
			if id, ok := strings.CutPrefix(ev, "air:"); ok {
				opID = id
			}
		}
	} else if nv, ok := nvByClaim[v.ID]; ok {
		row.Status, row.Reason, opID = "no_verdict", nv.SubjectRefs["reason"], nv.SubjectRefs["op_id"]
		row.SettledAt = nv.Provenance.CommittedAt
	}
	if opID == "unplaced" {
		opID = "" // a pseudo op: there is no record to read
	}
	if nv, ok := nvByClaim[v.ID]; ok && row.Status == "no_verdict" {
		// What the no-verdict record kept: where the position came from.
		var body struct {
			Position float64 `json:"position_seconds"`
			Source   string  `json:"position_source"`
		}
		if nv.Directive != nil && json.Unmarshal([]byte(nv.Directive.Body), &body) == nil && opID != "" {
			row.Op = &learningOp{OpID: opID, PositionSeconds: body.Position, PositionSource: body.Source}
		}
	}
	if opID != "" {
		if row.Op == nil {
			row.Op = &learningOp{OpID: opID}
		}
		rec, err := led.GetCommitmentRecord(ctx, ownerID, computer, sheetOpRecordPrefix+opID)
		if err != nil {
			return row, err
		}
		if rec != nil && rec.Directive != nil {
			var b sheetOpBody
			if json.Unmarshal([]byte(rec.Directive.Body), &b) == nil {
				row.Op.Version, row.Op.By = b.Version, string(b.Op.By)
				if b.Op.Ended != nil {
					row.Op.Outcome, row.Op.LastBlock, row.Op.EndedAt = string(b.Op.Ended.Outcome), b.Op.Ended.LastBlock, b.Op.Ended.At
				} else if b.Op.Op == runsheet.OpSkip {
					row.Op.Outcome = string(runsheet.OutcomeSkipped)
				} else if b.Op.State == runsheet.StateHeard {
					row.Op.Outcome = string(runsheet.OutcomeFinished)
				}
				if b.Op.Op == runsheet.OpPlace || b.Op.Op == runsheet.OpMove {
					// The settlement's evidence is the op that cut this entry.
					row.Op.Outcome, row.Op.Cutting = string(runsheet.OutcomeCut), true
				}
				// The op's position belongs to the entry the op names.
				if b.Op.PositionSeconds != nil && (b.Op.ID == row.EntryID || row.Status == "settled") {
					row.Op.PositionSeconds, row.Op.PositionSource = *b.Op.PositionSeconds, "op"
				}
			}
		}
	}
	return row, nil
}

func tagOf(tags []string, prefix string) string {
	for _, t := range tags {
		if v, ok := strings.CutPrefix(t, prefix); ok {
			return v
		}
	}
	return ""
}
