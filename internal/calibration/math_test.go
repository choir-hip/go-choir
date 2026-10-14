package calibration

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// Failure modes for the arithmetic (design §6, items 5, 6, 10, 14), written
// before the code:
//   - Brier on an empty set, or p outside [0,1], yields NaN in JSON;
//   - partial scores against 1 or 0 instead of 0.5;
//   - unresolvable enters plain Brier (it must not) or is dropped from
//     strict Brier (it must count as false);
//   - the overconfidence index is computed on p rather than on the stated
//     direction, so a confident correct "no" (p = 0.05, false) reads as
//     overconfidence;
//   - p = 1.0 falls off the last calibration bin;
//   - the governing settlement is the latest instead of the scorer order
//     rule > owner > independent > self;
//   - the self-score gap counts a self "true" against an independent
//     "false" as agreement;
//   - the pasteable context includes open claims' pending verdicts.

func settled(id string, p float64, outcomes ...SettlementView) ClaimView {
	return ClaimView{ID: id, Agent: "a", Model: "m", Type: types.CalibrationClaimAssertion, Claim: "claim " + id, P: p, Settlements: outcomes}
}

func sv(method, scorer, outcome string, at string) SettlementView {
	return SettlementView{Method: method, Scorer: scorer, Outcome: outcome, SettledAt: at}
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestBrierAndOutcomeValues(t *testing.T) {
	for _, tc := range []struct {
		outcome  string
		y        float64
		resolved bool
	}{
		{types.CalibrationOutcomeTrue, 1, true},
		{types.CalibrationOutcomeFalse, 0, true},
		{types.CalibrationOutcomePartial, 0.5, true},
		{types.CalibrationOutcomeUnresolvable, 0, false},
	} {
		y, resolved, ok := OutcomeValue(tc.outcome)
		if !ok || y != tc.y || resolved != tc.resolved {
			t.Fatalf("OutcomeValue(%q) = %v %v %v", tc.outcome, y, resolved, ok)
		}
	}
	if _, _, ok := OutcomeValue("yes"); ok {
		t.Fatal("unknown outcome accepted")
	}
	if !near(BrierScore(0.95, 0), 0.9025) || !near(BrierScore(0.8, 0.5), 0.09) || BrierScore(1, 1) != 0 {
		t.Fatal("Brier arithmetic")
	}
}

func TestGoverningSettlementFollowsScorerOrder(t *testing.T) {
	c := settled("c1", 0.9,
		sv(types.CalibrationScorerSelf, "a", types.CalibrationOutcomeTrue, "2026-10-14T10:00:00Z"),
		sv(types.CalibrationScorerIndependent, "b", types.CalibrationOutcomeFalse, "2026-10-14T09:00:00Z"),
		sv(types.CalibrationScorerIndependent, "c", types.CalibrationOutcomeTrue, "2026-10-14T11:00:00Z"),
	)
	g, ok := Governing(c.Settlements, "")
	if !ok || g.Scorer != "b" {
		t.Fatalf("governing = %+v; want the earliest independent (b), not the latest or self", g)
	}
	c.Settlements = append(c.Settlements, sv(types.CalibrationScorerRule, "ci", types.CalibrationOutcomeTrue, "2026-10-14T12:00:00Z"))
	if g, _ := Governing(c.Settlements, ""); g.Method != types.CalibrationScorerRule {
		t.Fatalf("rule must govern: %+v", g)
	}
	if g, ok := Governing(c.Settlements, types.CalibrationScorerSelf); !ok || g.Scorer != "a" {
		t.Fatalf("scorer=self picks the self settlement: %+v", g)
	}
	if _, ok := Governing(c.Settlements, types.CalibrationScorerOwner); ok {
		t.Fatal("scorer=owner found a settlement that does not exist")
	}
}

func TestSummaryMath(t *testing.T) {
	now := time.Date(2026, 10, 14, 20, 0, 0, 0, time.UTC)
	ind := func(o string) SettlementView {
		return sv(types.CalibrationScorerIndependent, "lead", o, "2026-10-14T12:00:00Z")
	}
	claims := []ClaimView{
		settled("1", 0.95, ind(types.CalibrationOutcomeFalse)),
		settled("2", 0.95, ind(types.CalibrationOutcomeTrue)),
		settled("3", 1.0, ind(types.CalibrationOutcomeTrue)),
		settled("4", 0.05, ind(types.CalibrationOutcomeFalse)), // confident, correct "no"
		settled("5", 0.8, ind(types.CalibrationOutcomePartial)),
		settled("6", 0.7, ind(types.CalibrationOutcomeUnresolvable)),
		{ID: "7", Agent: "a", Model: "m", Type: types.CalibrationClaimForecast, P: 0.6, Deadline: "2026-10-14T00:00:00Z"}, // open, overdue
		{ID: "8", Agent: "a", Model: "m", Type: types.CalibrationClaimForecast, P: 0.6},                                   // open
	}
	sum := Summarize(claims, SummaryOptions{}, now)
	o := sum.Overall
	if o.Claims != 8 || o.Open != 2 || o.Overdue != 1 || o.Settled != 6 || o.Resolved != 5 || o.Unresolvable != 1 {
		t.Fatalf("counts = %+v", o)
	}
	// Brier over resolved: (.95-0)^2 + (.95-1)^2 + 0 + (.05-0)^2 + (.8-.5)^2 = .9025+.0025+0+.0025+.09 = .9975 / 5
	if o.Brier == nil || !near(*o.Brier, 0.9975/5) {
		t.Fatalf("brier = %v", o.Brier)
	}
	// Strict adds unresolvable as false: (.7-0)^2 = .49 → 1.4875 / 6.
	if o.BrierStrict == nil || !near(*o.BrierStrict, 1.4875/6) {
		t.Fatalf("strict brier = %v", o.BrierStrict)
	}
	// Direction: confidences .95 .95 1 .95 .8 → mean 0.93; hits 0 1 1 1 .5 → 0.7.
	if o.Overconfidence == nil || !near(*o.Overconfidence, 0.93-0.7) {
		t.Fatalf("overconfidence = %v", o.Overconfidence)
	}
	if o.HitRate == nil || !near(*o.HitRate, 0.7) {
		t.Fatalf("hit rate = %v", o.HitRate)
	}
	// p = 1.0 lands in the [0.9, 1.0] bin with the two 0.95s.
	var top *Bin
	for i := range o.Bins {
		if o.Bins[i].Lo == 0.9 {
			top = &o.Bins[i]
		}
	}
	if top == nil || top.N != 3 || !near(top.Observed, 2.0/3) {
		t.Fatalf("top bin = %+v (bins %+v)", top, o.Bins)
	}
	// Default grouping is agent x model x type: assertion and forecast.
	if len(sum.Groups) != 2 {
		t.Fatalf("groups = %d", len(sum.Groups))
	}
	raw, err := json.Marshal(sum)
	if err != nil || strings.Contains(string(raw), "NaN") {
		t.Fatalf("summary JSON: %v %s", err, raw)
	}
	if !strings.Contains(sum.Context, "Brier") || !strings.Contains(sum.Context, "claim 1") {
		t.Fatalf("context lacks the score or the largest miss:\n%s", sum.Context)
	}
	if strings.Contains(sum.Context, "claim 7") || strings.Contains(sum.Context, "claim 8") {
		t.Fatalf("context names open claims:\n%s", sum.Context)
	}
}

func TestSummaryOfNothingIsNullNotNaN(t *testing.T) {
	sum := Summarize(nil, SummaryOptions{}, time.Now())
	if sum.Overall.Brier != nil || sum.Overall.Overconfidence != nil {
		t.Fatalf("empty summary has scores: %+v", sum.Overall)
	}
	raw, err := json.Marshal(sum)
	if err != nil || strings.Contains(string(raw), "NaN") {
		t.Fatalf("empty summary JSON: %v %s", err, raw)
	}
}

func TestReconstructedClaimsAreExcludedUnlessAsked(t *testing.T) {
	ind := sv(types.CalibrationScorerIndependent, "lead", types.CalibrationOutcomeFalse, "2026-10-14T12:00:00Z")
	c := settled("r", 0.95, ind)
	c.Reconstructed = true
	sum := Summarize([]ClaimView{c}, SummaryOptions{}, time.Now())
	if sum.Overall.Claims != 0 || sum.ReconstructedExcluded != 1 {
		t.Fatalf("reconstructed counted by default: %+v excluded=%d", sum.Overall, sum.ReconstructedExcluded)
	}
	sum = Summarize([]ClaimView{c}, SummaryOptions{IncludeReconstructed: true}, time.Now())
	if sum.Overall.Claims != 1 || sum.Overall.Settled != 1 {
		t.Fatalf("include_reconstructed ignored: %+v", sum.Overall)
	}
}

func TestSelfScoreGap(t *testing.T) {
	claims := []ClaimView{
		settled("1", 0.9, sv(types.CalibrationScorerSelf, "a", types.CalibrationOutcomeTrue, "t1"), sv(types.CalibrationScorerIndependent, "b", types.CalibrationOutcomeFalse, "t2")),
		settled("2", 0.9, sv(types.CalibrationScorerSelf, "a", types.CalibrationOutcomeTrue, "t1"), sv(types.CalibrationScorerRule, "ci", types.CalibrationOutcomeTrue, "t2")),
		settled("3", 0.9, sv(types.CalibrationScorerSelf, "a", types.CalibrationOutcomeFalse, "t1"), sv(types.CalibrationScorerIndependent, "b", types.CalibrationOutcomePartial, "t2")),
		settled("4", 0.9, sv(types.CalibrationScorerSelf, "a", types.CalibrationOutcomeTrue, "t1")),
	}
	gap := Summarize(claims, SummaryOptions{}, time.Now()).Overall.SelfGap
	if gap == nil || gap.N != 3 || gap.Agree != 1 || gap.SelfFavorable != 1 || gap.SelfUnfavorable != 1 {
		t.Fatalf("self gap = %+v", gap)
	}
}
