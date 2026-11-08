package trustharness

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/yusefmosiah/go-choir/internal/trust"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Report is one harness run's result.
type Report struct {
	Schema       string          `json:"schema"`
	ComputerID   string          `json:"computer_id"`
	BuildCommit  string          `json:"build_commit"`
	AsOf         time.Time       `json:"as_of"`
	InputsSHA256 string          `json:"inputs_sha256"`
	Deadline     string          `json:"deadline"`
	Served       map[string]bool `json:"served"`

	Inert    InertSection `json:"inert"`
	Ledger   LedgerScores `json:"ledger"`
	Tests    []TestResult `json:"tests"`
	GateTaps GateTaps     `json:"gate_taps"`
	Measured Measured     `json:"measured"`
	Drills   DrillResult  `json:"drills"`
}

// InertSection is (0'): the inert-precommitment and inert-delegation rates
// per desk, model and prompt version.
type InertSection struct {
	Rows      []types.InertRateRow      `json:"rows"`
	Agreement types.InertJudgeAgreement `json:"judge_agreement"`
	// Totals over every row.
	Precommitments      int     `json:"precommitments"`
	InertPrecommitments int     `json:"inert_precommitments"`
	InertPrecommitRate  float64 `json:"inert_precommit_rate"`
	Delegations         int     `json:"delegations"`
	InertDelegations    int     `json:"inert_delegations"`
	InertDelegateRate   float64 `json:"inert_delegation_rate"`
	Note                string  `json:"note"`
}

// LedgerScores is (0): the mission's own precommitment ledger scored.
type LedgerScores struct {
	Path         string         `json:"path"`
	SHA256       string         `json:"sha256"`
	Records      int            `json:"records"`
	ByResolution map[string]int `json:"by_resolution"`
	BySettler    map[string]int `json:"by_settler"`
	Probability  ProbScores     `json:"probability"`
	Interval     IntervalScores `json:"interval"`
	Scored       []LedgerRow    `json:"scored"`
	Problems     []string       `json:"problems,omitempty"`
}

// ProbScores are the probability records' scores.
type ProbScores struct {
	N int `json:"n"`
	// Brier is the mean (p - y)^2; LogScore the mean ln of the probability
	// given to what happened; BrierSkillVsHalf is 1 - Brier/0.25 (skill
	// against a 0.5 forecaster: the ledger states no base rate per record).
	Brier            float64 `json:"brier"`
	LogScore         float64 `json:"log_score"`
	BrierSkillVsHalf float64 `json:"brier_skill_vs_half"`
	// Sharpness is the mean |p - 0.5|.
	Sharpness   float64    `json:"sharpness"`
	Calibration []CalibBin `json:"calibration"`
}

// CalibBin is one reliability bin.
type CalibBin struct {
	Lo    float64 `json:"lo"`
	Hi    float64 `json:"hi"`
	N     int     `json:"n"`
	MeanP float64 `json:"mean_p"`
	Freq  float64 `json:"freq"`
}

// IntervalScores are the 80%-interval records' scores.
type IntervalScores struct {
	N        int     `json:"n"`
	Covered  int     `json:"covered"`
	Coverage float64 `json:"coverage"`
	Nominal  float64 `json:"nominal"`
	// MeanScaledIntervalScore is the mean Gneiting-Raftery interval score at
	// alpha 0.2, divided by |estimate| so units compare.
	MeanScaledIntervalScore float64 `json:"mean_scaled_interval_score"`
	// MeanLogRatio and MeanAbsLogRatio are ln(observed/estimate) over the
	// records where both are positive (NLogRatio of them).
	NLogRatio       int     `json:"n_log_ratio"`
	MeanLogRatio    float64 `json:"mean_log_ratio"`
	MeanAbsLogRatio float64 `json:"mean_abs_log_ratio"`
	// MeanRelativeWidth is the mean (hi - lo)/|estimate| (sharpness).
	MeanRelativeWidth float64 `json:"mean_relative_width"`
}

// LedgerRow is one scored record.
type LedgerRow struct {
	ID            string   `json:"id"`
	Kind          string   `json:"kind"`
	Resolution    string   `json:"resolution"`
	Settler       string   `json:"settler"`
	P             *float64 `json:"p,omitempty"`
	Outcome       *float64 `json:"outcome,omitempty"`
	Brier         *float64 `json:"brier,omitempty"`
	Log           *float64 `json:"log,omitempty"`
	Estimate      *float64 `json:"estimate,omitempty"`
	Lo            *float64 `json:"lo,omitempty"`
	Hi            *float64 `json:"hi,omitempty"`
	Observed      *float64 `json:"observed,omitempty"`
	Covered       *bool    `json:"covered,omitempty"`
	IntervalScore *float64 `json:"interval_score,omitempty"`
	LogRatio      *float64 `json:"log_ratio,omitempty"`
}

// TestResult is one pre-registered test.
type TestResult struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
	Reason string `json:"reason"`
	MinN   string `json:"min_n"`
	// N are the counts the test watches (they may be watched before the
	// deadline; section 1).
	N map[string]int `json:"n"`
	// Sealed says the outcome metrics were not computed: below the minimum n
	// before the deadline.
	Sealed    bool               `json:"sealed"`
	Metrics   map[string]float64 `json:"metrics,omitempty"`
	Baselines []BaselineResult   `json:"baselines,omitempty"`
	Notes     []string           `json:"notes,omitempty"`
}

// BaselineResult is one baseline on the same units.
type BaselineResult struct {
	Name       string  `json:"name"`
	Available  bool    `json:"available"`
	Degenerate bool    `json:"degenerate,omitempty"`
	AUC        float64 `json:"auc,omitempty"`
	Note       string  `json:"note,omitempty"`
}

// GateTaps is (a)'s tap rate: the act gate's rule replayed.
type GateTaps struct {
	Served           bool           `json:"served"`
	Acts             int            `json:"acts"`
	Gated            int            `json:"gated"`
	Breaker          int            `json:"breaker"`
	TapsPerDayMean   float64        `json:"taps_per_day_mean"`
	DaysCounted      int            `json:"days_counted"`
	TapsPerDayMedian float64        `json:"taps_per_day_median"`
	TapsPerDayP90    float64        `json:"taps_per_day_p90"`
	ByImpact         map[string]int `json:"by_impact,omitempty"`
	Note             string         `json:"note"`
}

// Measured is (c): lease durations and revocations, gate rates.
type Measured struct {
	Leases    LeaseStats       `json:"leases"`
	GatedActs GatedStats       `json:"gated_acts"`
	ActGate   map[string]int64 `json:"act_gate_counts"`
}

// LeaseStats summarizes the lease records.
type LeaseStats struct {
	N                   int            `json:"n"`
	ByState             map[string]int `json:"by_state"`
	ByEndReason         map[string]int `json:"by_end_reason"`
	Durable             int            `json:"durable"`
	Renewals            int            `json:"renewals"`
	RevokedByFidelity   int            `json:"revoked_by_fidelity"`
	NEnded              int            `json:"n_ended"`
	DurationHoursMedian float64        `json:"duration_hours_median"`
	DurationHoursP90    float64        `json:"duration_hours_p90"`
	DurableHoursMedian  float64        `json:"durable_hours_median"`
}

// GatedStats summarizes the gated acts on the tape.
type GatedStats struct {
	Served              bool           `json:"served"`
	N                   int            `json:"n"`
	Approved            int            `json:"approved"`
	Denied              int            `json:"denied"`
	Open                int            `json:"open"`
	Breaker             int            `json:"breaker"`
	ByClass             map[string]int `json:"by_class"`
	AnswerMinutesMedian float64        `json:"answer_minutes_median"`
	AnswerMinutesP90    float64        `json:"answer_minutes_p90"`
}

// DrillResult is (d): time from a planted package's install to its first
// gated act.
type DrillResult struct {
	N                     int     `json:"n"`
	Gated                 int     `json:"gated"`
	TimeToGateHoursMedian float64 `json:"time_to_gate_hours_median"`
	TimeToGateHoursMax    float64 `json:"time_to_gate_hours_max"`
	Note                  string  `json:"note"`
}

// failureReasonClass is the lease end reason the breaker writes.
const fidelityReasonPrefix = "revoked by the circuit breaker"

// Compute produces the report of in. It is a pure function of in.
func Compute(in Inputs) Report {
	raw, _ := json.Marshal(in)
	sum := sha256.Sum256(raw)
	rep := Report{Schema: ReportSchema, ComputerID: in.ComputerID, BuildCommit: in.BuildCommit, AsOf: in.FetchedAt.UTC(),
		InputsSHA256: hex.EncodeToString(sum[:]), Deadline: Deadline.Format("2006-01-02"), Served: in.Served}
	if rep.Served == nil {
		rep.Served = map[string]bool{}
	}
	rep.Inert = inertSection(in)
	rep.Ledger = ledgerScores(in.Ledger)
	rep.GateTaps = gateTaps(in)
	rep.Tests = tests(in, rep.GateTaps)
	rep.Measured = measured(in)
	rep.Drills = drills(in.Drills)
	return rep
}

func inertSection(in Inputs) InertSection {
	s := InertSection{Rows: []types.InertRateRow{}, Agreement: in.InertAgreement}
	for _, r := range in.InertRates {
		r.InertPrecommitRate, r.InertDelegateRate = r6(r.InertPrecommitRate), r6(r.InertDelegateRate)
		s.Rows = append(s.Rows, r)
		s.Precommitments += r.Precommitments
		s.InertPrecommitments += r.InertPrecommitments
		s.Delegations += r.Delegations
		s.InertDelegations += r.InertDelegations
	}
	s.Agreement.AgreementRate = r6(s.Agreement.AgreementRate)
	if s.Precommitments > 0 {
		s.InertPrecommitRate = r6(float64(s.InertPrecommitments) / float64(s.Precommitments))
	}
	if s.Delegations > 0 {
		s.InertDelegateRate = r6(float64(s.InertDelegations) / float64(s.Delegations))
	}
	switch {
	case len(s.Rows) == 0:
		s.Note = "n=0: no request record states a prediction and no Cast was written"
	default:
		s.Note = "before tuning: no prompt has been tuned against these rates yet (plan 4.4, measure then tune); each prompt version is its own row, so a tuned prompt shows as a new row"
	}
	return s
}

func fp(v float64) *float64 { v = r6(v); return &v }

func ledgerScores(l LedgerInput) LedgerScores {
	s := LedgerScores{Path: l.Path, SHA256: l.SHA256, Records: len(l.Records), ByResolution: map[string]int{}, BySettler: map[string]int{},
		Scored: []LedgerRow{}, Problems: l.Problems, Interval: IntervalScores{Nominal: 0.8}}
	type bin struct {
		lo, hi, sp, sy float64
		n              int
	}
	bins := []*bin{{lo: 0, hi: 0.5}, {lo: 0.5, hi: 0.7}, {lo: 0.7, hi: 0.9}, {lo: 0.9, hi: 1.0001}}
	var brier, logs, sharp float64
	var isc, lr, alr, width float64
	nScaled := 0
	for _, r := range l.Records {
		s.ByResolution[r.Resolution]++
		switch r.Resolution {
		case "confirmed", "contradicted", "qualified":
			settler := r.Settler
			if settler == "" {
				settler = "unstated"
			}
			s.BySettler[settler]++
		}
		row := LedgerRow{ID: r.ID, Kind: r.Kind, Resolution: r.Resolution, Settler: r.Settler}
		switch {
		case r.Kind == "probability" && r.Outcome != nil:
			p, y := *r.P, *r.Outcome
			b := (p - y) * (p - y)
			py := p
			if y == 0 {
				py = 1 - p
			}
			lg := math.Log(math.Max(py, 1e-9))
			row.P, row.Outcome, row.Brier, row.Log = fp(p), fp(y), fp(b), fp(lg)
			s.Probability.N++
			brier += b
			logs += lg
			sharp += math.Abs(p - 0.5)
			for _, bn := range bins {
				if p >= bn.lo && p < bn.hi {
					bn.n++
					bn.sp += p
					bn.sy += y
				}
			}
		case r.Kind == "interval" && r.Observed != nil:
			e, lo, hi, x := *r.Estimate, *r.Lo, *r.Hi, *r.Observed
			const alpha = 0.2
			score := hi - lo
			if x < lo {
				score += 2 / alpha * (lo - x)
			}
			if x > hi {
				score += 2 / alpha * (x - hi)
			}
			cov := x >= lo && x <= hi
			row.Estimate, row.Lo, row.Hi, row.Observed, row.Covered, row.IntervalScore = fp(e), fp(lo), fp(hi), fp(x), &cov, fp(score)
			s.Interval.N++
			if cov {
				s.Interval.Covered++
			}
			if e != 0 {
				isc += score / math.Abs(e)
				width += (hi - lo) / math.Abs(e)
				nScaled++
			}
			if e > 0 && x > 0 {
				v := math.Log(x / e)
				row.LogRatio = fp(v)
				s.Interval.NLogRatio++
				lr += v
				alr += math.Abs(v)
			}
		default:
			continue
		}
		s.Scored = append(s.Scored, row)
	}
	if n := float64(s.Probability.N); n > 0 {
		s.Probability.Brier = r6(brier / n)
		s.Probability.LogScore = r6(logs / n)
		s.Probability.BrierSkillVsHalf = r6(1 - (brier/n)/0.25)
		s.Probability.Sharpness = r6(sharp / n)
	}
	s.Probability.Calibration = []CalibBin{}
	for _, bn := range bins {
		cb := CalibBin{Lo: bn.lo, Hi: math.Min(bn.hi, 1), N: bn.n}
		if bn.n > 0 {
			cb.MeanP, cb.Freq = r6(bn.sp/float64(bn.n)), r6(bn.sy/float64(bn.n))
		}
		s.Probability.Calibration = append(s.Probability.Calibration, cb)
	}
	if n := float64(s.Interval.N); n > 0 {
		s.Interval.Coverage = r6(float64(s.Interval.Covered) / n)
	}
	// The scaled means are over the records with a nonzero estimate only.
	if n := float64(nScaled); n > 0 {
		s.Interval.MeanScaledIntervalScore = r6(isc / n)
		s.Interval.MeanRelativeWidth = r6(width / n)
	}
	if n := float64(s.Interval.NLogRatio); n > 0 {
		s.Interval.MeanLogRatio, s.Interval.MeanAbsLogRatio = r6(lr/n), r6(alr/n)
	}
	return s
}

func gateTaps(in Inputs) GateTaps {
	g := GateTaps{Served: in.GateReplay != nil}
	if in.GateReplay == nil {
		g.Note = "the computer's build served no gate_replay"
		return g
	}
	gr := in.GateReplay
	g.Acts, g.Gated, g.Breaker, g.TapsPerDayMean, g.ByImpact = gr.Acts, gr.Gated, gr.Breaker, r6(gr.TapsPerDay), gr.ByImpact
	if gr.Acts == 0 {
		g.Note = "n=0: no material act (I2 and up) of a desk in the settled records"
		return g
	}
	if len(gr.ActsByDay) == 0 {
		g.Note = "the computer's build served no per-day counts; only the mean is known"
		return g
	}
	var days []string
	for d := range gr.ActsByDay {
		days = append(days, d)
	}
	sort.Strings(days)
	first, _ := time.Parse("2006-01-02", days[0])
	last, _ := time.Parse("2006-01-02", days[len(days)-1])
	var taps []float64
	for d := first; !d.After(last); d = d.AddDate(0, 0, 1) {
		taps = append(taps, float64(gr.GatedByDay[d.Format("2006-01-02")]))
	}
	g.DaysCounted = len(taps)
	g.TapsPerDayMedian, g.TapsPerDayP90 = r6(median(taps)), r6(p90(taps))
	g.Note = "taps per computer-day over every day from the first to the last material act, days without an act counted as zero; breaker gates are counted apart (P2 known edges)"
	return g
}

func scoreUnits(units []trust.PreregUnit, risk func(trust.PreregUnit) float64) []scored {
	out := make([]scored, len(units))
	for i, u := range units {
		out[i] = scored{subject: u.Desk, risk: risk(u), pos: u.Positive}
	}
	return out
}

type candidate struct {
	name  string
	risk  func(trust.PreregUnit) float64
	avail bool
	note  string
}

// compare scores the predictor and the baselines on the same units and
// returns the predictor's AUC, the best non-degenerate available baseline's
// AUC and name, and the paired bootstrap interval of the difference.
func compare(units []trust.PreregUnit, pred func(trust.PreregUnit) float64, bases []candidate) (map[string]float64, []BaselineResult, string) {
	m := map[string]float64{}
	ps := scoreUnits(units, pred)
	// An AUC with no positives or no negatives is undefined, so it is left
	// out rather than stored as 0.5 and judged.
	a, aOK := aucOf(ps)
	if aOK {
		m["auc"] = r6(a)
	}
	var out []BaselineResult
	best, bestName := -1.0, ""
	var bestScores []scored
	for _, b := range bases {
		br := BaselineResult{Name: b.name, Available: b.avail, Note: b.note}
		if b.avail {
			bs := scoreUnits(units, b.risk)
			br.Degenerate = degenerate(bs)
			x, _ := aucOf(bs)
			br.AUC = r6(x)
			if !br.Degenerate && x > best {
				best, bestName, bestScores = x, b.name, bs
			}
		}
		out = append(out, br)
	}
	if bestScores != nil {
		m["best_baseline_auc"] = r6(best)
		if aOK {
			m["auc_difference"] = r6(a - best)
		}
		if lo, hi, ok := bootstrapDiff(ps, bestScores); ok {
			m["auc_difference_lo95"], m["auc_difference_hi95"] = r6(lo), r6(hi)
		}
	}
	return m, out, bestName
}

func tests(in Inputs, taps GateTaps) []TestResult {
	asOf := in.FetchedAt.UTC()
	pastDeadline := !asOf.Before(Deadline)
	var units []trust.PreregUnit
	n := map[string]int{}
	unitsServed := in.PreregUnits != nil
	if unitsServed {
		for _, u := range in.PreregUnits.Units {
			if u.HasRow {
				units = append(units, u)
			}
		}
		n["material_items"] = in.PreregUnits.MaterialItems
		n["cutoffs"] = len(in.PreregUnits.Cutoffs)
		n["units"] = len(units)
		n["units_without_row"] = len(in.PreregUnits.Units) - len(units)
		n["dropped_no_act"] = in.PreregUnits.DroppedNoAct
		n["dropped_open_window"] = in.PreregUnits.DroppedOpen
	}
	pos, subjects := 0, map[string]bool{}
	for _, u := range units {
		subjects[u.Desk] = true
		if u.Positive {
			pos++
		}
	}
	if unitsServed {
		n["positive_units"] = pos
		n["subjects"] = len(subjects)
	}
	if in.Replay != nil {
		n["replay_pairs"] = in.Replay.Pairs
	}
	n["view_rows"] = in.ViewRows
	copyN := func(extra map[string]int) map[string]int {
		o := map[string]int{}
		for k, v := range n {
			o[k] = v
		}
		for k, v := range extra {
			o[k] = v
		}
		return o
	}
	noUnits := func(t *TestResult) bool {
		if unitsServed {
			return false
		}
		t.Status, t.Sealed = "unresolved", true
		t.Reason = fmt.Sprintf("the computer's build served no prereg_units; the replay walked %d pairs over %d trust rows", n["replay_pairs"], in.ViewRows)
		return true
	}
	successRate := candidate{name: "success_rate", avail: true, risk: func(u trust.PreregUnit) float64 { return -u.SuccessRate }}
	popularity := candidate{name: "popularity_install_count", note: "no install counts in the inputs; the Library counts installs per computer and d@e has no third-party installs to rank"}

	var out []TestResult

	// P1: H4.
	p1 := TestResult{ID: "P1", Name: "H4: a track record beats success rate, benchmarks, popularity and ratings", MinN: "30 positive units from at least 5 subjects"}
	p1.N = copyN(nil)
	if !noUnits(&p1) {
		below := pos < 30 || len(subjects) < 5
		if below && !pastDeadline {
			p1.Status, p1.Sealed = "unresolved", true
			p1.Reason = fmt.Sprintf("below the minimum n (%d of 30 positive units, %d of 5 subjects); outcome metrics sealed until the minimum n or %s", pos, len(subjects), Deadline.Format("2006-01-02"))
		} else {
			m, bases, best := compare(units, func(u trust.PreregUnit) float64 { return 1 - u.Mean }, []candidate{
				{name: "b1_success_rate", avail: true, risk: successRate.risk},
				{name: "b2_benchmark", note: "constant per model; degenerate within one computer (P1)"},
				{name: "b3_" + popularity.name, note: popularity.note},
				{name: "b4_uncalibrated_rating", note: "no System 1 0-to-10 ratings are recorded"},
			})
			p1.Metrics, p1.Baselines = m, bases
			var split time.Time
			cut := make([]float64, len(units))
			for i, u := range units {
				cut[i] = float64(u.Cutoff.Unix())
			}
			split = time.Unix(int64(median(cut)), 0).UTC()
			var pm, ps []float64
			var ys []bool
			var xs, zs []float64
			for _, u := range units {
				xs, zs = append(xs, u.SuccessRate), append(zs, u.Mean)
				if !u.Cutoff.Before(split) {
					pm, ps, ys = append(pm, 1-u.Mean), append(ps, 1-u.SuccessRate), append(ys, u.Positive)
				}
			}
			if len(ys) > 0 {
				m["heldout_logloss_track_record"], m["heldout_logloss_success_rate"] = r6(logLoss(pm, ys)), r6(logLoss(ps, ys))
				p1.N["heldout_units"] = len(ys)
				p1.Notes = append(p1.Notes, "time split at the median cutoff "+split.Format(time.RFC3339))
			}
			if c, ok := pearson(xs, zs); ok {
				m["corr_success_rate_track_record"] = r6(c)
			}
			p1.Status, p1.Reason = decideDiff(m, best, below, 0.02)
			p1.Notes = append(p1.Notes, "secondary (domain-specific against pooled) is not computed: the pooled posterior is not in the units")
		}
	}
	out = append(out, p1)

	// P2: the trust conjecture.
	p2 := TestResult{ID: "P2", Name: "Trust conjecture: the LCB ordering predicts the next material failure at a usable tap rate", MinN: "30 events"}
	p2.N = copyN(map[string]int{"gate_acts": taps.Acts, "gate_taps": taps.Gated})
	if !noUnits(&p2) {
		below := pos < 30
		if below && !pastDeadline {
			p2.Status, p2.Sealed = "unresolved", true
			p2.Reason = fmt.Sprintf("below the minimum n (%d of 30 events); outcome metrics sealed until the minimum n or %s", pos, Deadline.Format("2006-01-02"))
		} else {
			m, bases, best := compare(units, func(u trust.PreregUnit) float64 { return -u.LCB }, []candidate{
				{name: "b1_allowlist_grant_impact", avail: true, risk: func(u trust.PreregUnit) float64 { return float64(u.ImpactRank) }},
				{name: "b1_sandbox_trust_class", note: "the sandbox trust class (R40) is not in the units"},
				{name: "b2_" + popularity.name, note: popularity.note},
				successRate,
				{name: "count", avail: true, risk: func(u trust.PreregUnit) float64 { return -float64(u.PriorItems) }},
				{name: "recency_hours", avail: true, risk: func(u trust.PreregUnit) float64 { return u.RecencyHours }},
			})
			p2.Metrics, p2.Baselines = m, bases
			srAUC := 0.0
			for _, b := range bases {
				if b.Name == "success_rate" {
					srAUC = b.AUC
				}
			}
			usable := taps.TapsPerDayMedian <= 5 && taps.TapsPerDayP90 <= 15
			m["taps_per_day_median"], m["taps_per_day_p90"] = taps.TapsPerDayMedian, taps.TapsPerDayP90
			breakerUnits := 0
			for _, u := range units {
				if u.Breaker {
					breakerUnits++
				}
			}
			p2.N["breaker_units"] = breakerUnits
			auc, hasAUC := m["auc"]
			lo, hasLo := m["auc_difference_lo95"]
			switch {
			case below:
				p2.Status, p2.Reason = "unresolved", fmt.Sprintf("the deadline passed below the minimum n (%d of 30 events)", pos)
			case !hasAUC:
				p2.Status, p2.Reason = "unresolved", "the AUC is undefined: the units have no failures or no non-failures"
			case auc < 0.55 || auc < srAUC:
				p2.Status, p2.Reason = "refuted", "AUC below 0.55 or below the success-rate baseline"
			case hasLo && lo > 0 && usable:
				p2.Status, p2.Reason = "supported", "AUC at least 0.55, its difference against "+best+" above 0, and a usable tap rate"
			case hasLo && lo > 0:
				p2.Status, p2.Reason = "predictive, not usable", "the ordering predicts, but the tap rate exceeds median 5 or p90 15 per computer-day"
			default:
				p2.Status, p2.Reason = "unresolved", "the interval of the difference against the best baseline includes 0, or there is no non-degenerate baseline"
			}
			p2.Notes = append(p2.Notes, "missing_oracle: failures nobody settles never enter the units; the share of I2+ acts with a rule settler is not yet in the inputs")
		}
	}
	out = append(out, p2)

	// P5: the replay's weighting choice.
	p5 := TestResult{ID: "P5", Name: "H-C1: raw pseudo-counts overconfident; tempered or capped chosen", MinN: "30 failure events and 20 (s, k) cells with at least 10 settled records", Sealed: true, N: copyN(nil)}
	p5.Status = "unresolved"
	if in.Replay == nil {
		p5.Reason = "the computer's build served no replay"
	} else {
		p5.Reason = fmt.Sprintf("capped k=3 ships provisionally; the choice was not made by replay (%d pairs; the held-out log-loss test against raw is not built, the server replay picks by Brier at %d pairs or more)", in.Replay.Pairs, trust.MinReplayPairs)
	}
	out = append(out, p5)

	for _, nd := range []struct{ id, name, minN, why string }{
		{"P3", "H-A1: outcome-built weights reward extremizers", "200 settled records with 3+ consequences, 20 with an I3/I4 consequence", "no data source: the frozen consequence sets and their realized materiality are not in the trust inputs"},
		{"P4", "H-A2: flip-rate relevance predicts reverts", "300 actions with sampled relevance and 40 reverts", "no data source: the flip-rate sampler (MA7.10) is not built"},
		{"P6", "H-B3: spot-checks beat peer agreement", "4 scorers, 200 anchored items, 2 question types", "no data source: the anchor sample (MA7.10) is not built"},
		{"P7", "H-B4: PPI covers, saves under 2x for strong agents", "2 agents with a 300-item anchored pool", "no data source: the anchor sample (MA7.10) is not built"},
		{"P8", "H8: satisfice, then diversify", "6 blocks and 20 rated apps per policy", "no data source: the Phase 8 switchback has not started"},
		{"P9", "Score 2 adds no predictive value beyond scores 1 and 3", "30 failure events among units with a score-2 aggregate", "no data source: no realized-materiality window has closed in the trust inputs"},
	} {
		out = append(out, TestResult{ID: nd.id, Name: nd.name, MinN: nd.minN, Status: "unresolved", Reason: nd.why, Sealed: true, N: map[string]int{"units": 0}})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// decideDiff is P1's rule on the AUC difference's interval.
func decideDiff(m map[string]float64, best string, below bool, refuteBelow float64) (string, string) {
	if below {
		return "unresolved", "the deadline passed below the minimum n"
	}
	lo, okLo := m["auc_difference_lo95"]
	hi := m["auc_difference_hi95"]
	switch {
	case best == "" || !okLo:
		return "unresolved", "no non-degenerate baseline to compare, or the bootstrap had too few defined resamples"
	case lo > 0:
		return "supported", "the AUC difference against " + best + " has its 95% interval above 0"
	case hi < refuteBelow:
		return "refuted", fmt.Sprintf("the AUC difference's upper bound is below %.2f", refuteBelow)
	default:
		return "unresolved", "the interval neither clears 0 nor falls below the refutation line"
	}
}

func measured(in Inputs) Measured {
	m := Measured{ActGate: in.ActGateCounts}
	if m.ActGate == nil {
		m.ActGate = map[string]int64{}
	}
	ls := LeaseStats{N: len(in.Leases), ByState: map[string]int{}, ByEndReason: map[string]int{}}
	var durs, durable []float64
	for _, l := range in.Leases {
		ls.ByState[l.State]++
		if l.EndReason != "" {
			ls.ByEndReason[l.EndReason]++
		}
		if len(l.EndReason) >= len(fidelityReasonPrefix) && l.EndReason[:len(fidelityReasonPrefix)] == fidelityReasonPrefix {
			ls.RevokedByFidelity++
		}
		if l.Durable {
			ls.Durable++
		}
		ls.Renewals += l.Renewals
		if l.EndedAt != nil {
			h := l.EndedAt.Sub(l.IssuedAt).Hours()
			durs = append(durs, h)
			if l.Durable {
				durable = append(durable, h)
			}
		}
	}
	ls.NEnded = len(durs)
	if len(durs) > 0 {
		ls.DurationHoursMedian, ls.DurationHoursP90 = r6(median(durs)), r6(p90(durs))
	}
	if len(durable) > 0 {
		ls.DurableHoursMedian = r6(median(durable))
	}
	m.Leases = ls
	gs := GatedStats{Served: in.Served["gated_acts"], ByClass: map[string]int{}}
	var mins []float64
	for _, g := range in.GatedActs {
		gs.N++
		gs.ByClass[g.Class.String()]++
		if g.Breaker {
			gs.Breaker++
		}
		switch g.Answer {
		case "approved":
			gs.Approved++
		case "denied":
			gs.Denied++
		default:
			gs.Open++
		}
		if g.AnsweredAt != nil {
			mins = append(mins, g.AnsweredAt.Sub(g.AskedAt).Minutes())
		}
	}
	if len(mins) > 0 {
		gs.AnswerMinutesMedian, gs.AnswerMinutesP90 = r6(median(mins)), r6(p90(mins))
	}
	m.GatedActs = gs
	return m
}

func drills(ds []Drill) DrillResult {
	r := DrillResult{N: len(ds)}
	if len(ds) == 0 {
		r.Note = "n=0: no planted-package drill has run on d@e"
		return r
	}
	var hs []float64
	for _, d := range ds {
		if d.FirstGatedAt != nil {
			r.Gated++
			hs = append(hs, d.FirstGatedAt.Sub(d.InstalledAt).Hours())
		}
	}
	if len(hs) > 0 {
		r.TimeToGateHoursMedian = r6(median(hs))
		mx := hs[0]
		for _, h := range hs {
			mx = math.Max(mx, h)
		}
		r.TimeToGateHoursMax = r6(mx)
	}
	r.Note = fmt.Sprintf("%d of %d planted packages gated", r.Gated, r.N)
	return r
}
