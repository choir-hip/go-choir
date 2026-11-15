package calibration

import (
	"math"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// Failure list for the pooled causal base rate (design 5, 12.7), written
// before the code:
//  1. the base rate of a forecast uses outcomes settled at or after the
//     forecast was committed (it must use only strictly earlier settlements);
//  2. the base rate is the in-sample mean, which the existing BaseRate already
//     is, instead of the running pooled rate;
//  3. the per-kind rate is not shrunk toward the pooled rate with weight 10,
//     or the pooled rate is not 0.5 until one forecast is settled;
//  4. unresolvable, open, overdue and unsettled forecasts enter the base rate
//     or the pairs;
//  5. the pool is cut to the group instead of all forecasts of the summary
//     (a rare kind would then compare with 0.5);
//  6. the paired difference or its standard error is computed from anything
//     but the per-forecast Brier of p minus the Brier of the base rate;
//  7. the fields are NaN or zero instead of absent when nothing is resolved,
//     and the standard error is reported for fewer than two pairs;
//  8. the extension changes an existing field of GroupStats.

func causalClaim(id, project string, p float64, recorded string, outcome, settled string) ClaimView {
	c := ClaimView{ID: id, Agent: "autoradio-dj", Model: "m", Type: types.CalibrationClaimForecast, Claim: id, P: p, Project: project, RecordedAt: recorded}
	if outcome != "" {
		c.Settlements = []SettlementView{{ID: id + ":s", Method: types.CalibrationScorerRule, Scorer: "autoradio_heard", Outcome: outcome, SettledAt: settled}}
	}
	return c
}

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestCausalBaseRateBrier(t *testing.T) {
	now := mustTime(t, "2026-11-16T00:00:00Z")
	claims := []ClaimView{
		causalClaim("A", "x", 0.8, "2026-11-15T09:00:00Z", types.CalibrationOutcomeTrue, "2026-11-15T09:30:00Z"),
		causalClaim("B", "x", 0.6, "2026-11-15T10:00:00Z", types.CalibrationOutcomeFalse, "2026-11-15T10:30:00Z"),
		causalClaim("C", "y", 0.3, "2026-11-15T11:00:00Z", types.CalibrationOutcomeTrue, "2026-11-15T11:30:00Z"),
		causalClaim("D", "x", 0.5, "2026-11-15T12:00:00Z", types.CalibrationOutcomeTrue, "2026-11-15T12:30:00Z"),
		// G is committed at the instant A settled: A is not strictly earlier.
		causalClaim("G", "z", 0.5, "2026-11-15T09:30:00Z", types.CalibrationOutcomeTrue, "2026-11-15T13:00:00Z"),
		// Never enter: open, unresolvable.
		causalClaim("E", "x", 0.9, "2026-11-15T08:00:00Z", "", ""),
		causalClaim("F", "x", 0.9, "2026-11-15T08:30:00Z", types.CalibrationOutcomeUnresolvable, "2026-11-15T08:45:00Z"),
	}
	baseD := (1 + 10*(2.0/3.0)) / 12 // kind x: one yes, one no; pool 2 of 3
	brierBase := map[string]float64{
		"A": BrierScore(0.5, 1), "B": BrierScore(1, 0), "C": BrierScore(0.5, 1), "D": BrierScore(baseD, 1), "G": BrierScore(0.5, 1),
	}
	brierP := map[string]float64{"A": BrierScore(0.8, 1), "B": BrierScore(0.6, 0), "C": BrierScore(0.3, 1), "D": BrierScore(0.5, 1), "G": BrierScore(0.5, 1)}
	summarize := func(ids ...string) (mb, md, se float64, n int) {
		var diffs []float64
		for _, id := range ids {
			mb += brierBase[id]
			diffs = append(diffs, brierP[id]-brierBase[id])
		}
		n = len(ids)
		mb /= float64(n)
		for _, d := range diffs {
			md += d
		}
		md /= float64(n)
		var ss float64
		for _, d := range diffs {
			ss += (d - md) * (d - md)
		}
		se = math.Sqrt(ss/float64(n-1)) / math.Sqrt(float64(n))
		return
	}

	sum := Summarize(claims, SummaryOptions{GroupBy: []string{"project"}}, now)
	mb, md, se, n := summarize("A", "B", "C", "D", "G")
	o := sum.Overall
	if o.CausalN != n || o.CausalBaseRateBrier == nil || !approx(*o.CausalBaseRateBrier, mb) {
		t.Fatalf("overall causal base-rate Brier = %v (n=%d), want %v (n=%d)", o.CausalBaseRateBrier, o.CausalN, mb, n)
	}
	if o.CausalBrierDiff == nil || !approx(*o.CausalBrierDiff, md) || o.CausalBrierDiffSE == nil || !approx(*o.CausalBrierDiffSE, se) {
		t.Fatalf("overall diff = %v se = %v, want %v %v", o.CausalBrierDiff, o.CausalBrierDiffSE, md, se)
	}
	// The existing fields are as they were.
	if o.Resolved != 5 || o.Unresolvable != 1 || o.Claims != 7 || o.BaseRate == nil || !approx(*o.BaseRate, 0.8) {
		t.Fatalf("existing fields changed: %+v", o)
	}
	// Per kind: the pool stays the whole summary.
	var gx *GroupStats
	for i := range sum.Groups {
		if sum.Groups[i].Project == "x" {
			gx = &sum.Groups[i]
		}
	}
	if gx == nil {
		t.Fatal("no group x")
	}
	mbx, mdx, sex, nx := summarize("A", "B", "D")
	if gx.CausalN != nx || gx.CausalBaseRateBrier == nil || !approx(*gx.CausalBaseRateBrier, mbx) ||
		gx.CausalBrierDiff == nil || !approx(*gx.CausalBrierDiff, mdx) || gx.CausalBrierDiffSE == nil || !approx(*gx.CausalBrierDiffSE, sex) {
		t.Fatalf("group x = %+v", gx)
	}
	// The scorer selection applies: scorer=owner sees no rule settlement.
	own := Summarize(claims, SummaryOptions{Scorer: types.CalibrationScorerOwner}, now)
	if own.Overall.CausalBaseRateBrier != nil || own.Overall.CausalBrierDiff != nil || own.Overall.CausalN != 0 {
		t.Fatalf("scorer=owner: %+v", own.Overall)
	}
}

func TestCausalBaseRateAbsentAndSingle(t *testing.T) {
	now := mustTime(t, "2026-11-16T00:00:00Z")
	none := Summarize([]ClaimView{causalClaim("E", "x", 0.9, "2026-11-15T08:00:00Z", "", "")}, SummaryOptions{}, now)
	if none.Overall.CausalBaseRateBrier != nil || none.Overall.CausalBrierDiff != nil || none.Overall.CausalBrierDiffSE != nil || none.Overall.CausalN != 0 {
		t.Fatalf("nothing resolved: %+v", none.Overall)
	}
	one := Summarize([]ClaimView{causalClaim("A", "x", 0.8, "2026-11-15T09:00:00Z", types.CalibrationOutcomeTrue, "2026-11-15T09:30:00Z")}, SummaryOptions{}, now)
	if one.Overall.CausalN != 1 || one.Overall.CausalBaseRateBrier == nil || !approx(*one.Overall.CausalBaseRateBrier, 0.25) {
		t.Fatalf("one forecast: %+v", one.Overall)
	}
	if one.Overall.CausalBrierDiff == nil || !approx(*one.Overall.CausalBrierDiff, BrierScore(0.8, 1)-0.25) || one.Overall.CausalBrierDiffSE != nil {
		t.Fatalf("one forecast has a standard error: %+v", one.Overall)
	}
	// Reconstructed claims stay excluded from the pool as from everything.
	rec := causalClaim("R", "x", 0.8, "2026-11-15T08:00:00Z", types.CalibrationOutcomeTrue, "2026-11-15T08:10:00Z")
	rec.Reconstructed = true
	two := Summarize([]ClaimView{rec, causalClaim("A", "x", 0.8, "2026-11-15T09:00:00Z", types.CalibrationOutcomeTrue, "2026-11-15T09:30:00Z")}, SummaryOptions{}, now)
	if two.Overall.CausalN != 1 || !approx(*two.Overall.CausalBaseRateBrier, 0.25) {
		t.Fatalf("a reconstructed claim entered the pool: %+v", two.Overall)
	}
}

func TestCausalBaseRateIsLinearInTheNumberOfClaims(t *testing.T) {
	// 20000 settled forecasts must not take quadratic time.
	base := mustTime(t, "2026-11-01T00:00:00Z")
	claims := make([]ClaimView, 0, 20000)
	for i := 0; i < 20000; i++ {
		at := base.Add(time.Duration(i) * time.Minute)
		claims = append(claims, causalClaim("c"+time.Duration(i).String(), "x", 0.5, at.Format(time.RFC3339), types.CalibrationOutcomeTrue, at.Add(30*time.Second).Format(time.RFC3339)))
	}
	start := time.Now()
	sum := Summarize(claims, SummaryOptions{}, base.Add(400*time.Hour))
	if sum.Overall.CausalN != 20000 {
		t.Fatalf("n = %d", sum.Overall.CausalN)
	}
	if time.Since(start) > 10*time.Second {
		t.Fatalf("20000 forecasts took %v", time.Since(start))
	}
}
