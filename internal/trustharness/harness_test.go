package trustharness

// Failure modes of the evidence harness (MA7.9), written before the code:
//  H1 an outcome metric is computed below the minimum n before the deadline
//     (an interim look the pre-registration forbids);
//  H2 a test with no data reports a number instead of n=0 and its reason;
//  H3 the same stored inputs give different bytes (a map order, a float's
//     last bits, a clock read), or a JSON round trip of the inputs changes
//     the result (the reproduce path);
//  H4 a ledger transcription whose quote is not in the ledger, or that
//     contradicts the parsed resolution, is scored;
//  H5 the AUC orientation is wrong (a lower bound that orders failures last
//     scores below 0.5), or a constant baseline is counted as beaten;
//  H6 the bootstrap is not reproducible.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/trust"
)

const ledgerText = `# Ledger

**R0-01.** A ratio.
- Estimate: ratio 1.6, 80% interval [1.2, 2.5].
- Falsifier: above 2.5.

**R0-02.** A probability.
- p = 0.80.

**R0-03.** Another.
- p = 0.40.

**R0-04.** Open.
- p = 0.5.

**R0-01, settled contradicted.** Ratio about 4.5, outside [1.2, 2.5].

**R0-02, settled confirmed.** All ran. p = 0.80; Brier 0.04.

**R0-03, settled qualified (settler: rule).** The claim as worded: contradicted.
`

func f64(v float64) *float64 { return &v }

func TestLedgerScoresAndRefusals(t *testing.T) {
	trans := []Transcription{
		{ID: "R0-01", Settler: "rule", Observed: f64(4.5), Quote: "Ratio about 4.5, outside [1.2, 2.5]."},
		{ID: "R0-02", Settler: "devin", Quote: "All ran."},
		{ID: "R0-03", Settler: "rule", Outcome: f64(0), Quote: "The claim as worded: contradicted."},
	}
	l := ParseLedger("ledger.md", ledgerText, trans)
	if len(l.Records) != 4 || len(l.Problems) != 0 {
		t.Fatalf("records %d problems %v", len(l.Records), l.Problems)
	}
	s := ledgerScores(l)
	if s.Probability.N != 2 || s.Interval.N != 1 || s.Interval.Covered != 0 {
		t.Fatalf("scored %+v %+v", s.Probability, s.Interval)
	}
	// R0-02: (0.8-1)^2 = 0.04; R0-03: (0.4-0)^2 = 0.16.
	if s.Probability.Brier != 0.1 {
		t.Fatalf("brier %v", s.Probability.Brier)
	}
	// interval score: 1.3 + 10*(4.5-2.5) = 21.3
	if *s.Scored[0].IntervalScore != 21.3 {
		t.Fatalf("interval score %v", *s.Scored[0].IntervalScore)
	}
	if s.ByResolution["open"] != 1 || s.BySettler["devin"] != 1 || s.BySettler["rule"] != 2 {
		t.Fatalf("counts %v %v", s.ByResolution, s.BySettler)
	}

	// H4: a quote not in the ledger, an outcome against the resolution, an
	// observed value inside an interval the ledger says was missed.
	bad := []Transcription{
		{ID: "R0-01", Settler: "rule", Observed: f64(2.0), Quote: "Ratio about 4.5, outside [1.2, 2.5]."},
		{ID: "R0-02", Settler: "devin", Outcome: f64(0), Quote: "All ran."},
		{ID: "R0-03", Settler: "rule", Outcome: f64(0), Quote: "a sentence nobody wrote"},
	}
	l = ParseLedger("ledger.md", ledgerText, bad)
	if len(l.Problems) != 3 {
		t.Fatalf("problems %v", l.Problems)
	}
	s = ledgerScores(l)
	// R0-02 still scores from its parsed resolution; the qualified R0-03 and
	// the contradicted interval R0-01 are not scored without a valid entry.
	if s.Probability.N != 1 || s.Interval.N != 0 {
		t.Fatalf("refused transcriptions were scored: %+v %+v", s.Probability, s.Interval)
	}
}

// units builds n units over subjects; positives get a lower bound below the
// negatives' when separate is true.
func units(n, subjects int, separate bool) *trust.PreregUnitSet {
	set := &trust.PreregUnitSet{Window: "336h0m0s", Cutoffs: []time.Time{}, Units: []trust.PreregUnit{}}
	base := time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)
	for i := 0; i < n; i++ {
		pos := i%2 == 0
		lcb := 0.5 + float64(i%7)/100
		if separate && pos {
			lcb = 0.1 + float64(i%5)/100
		}
		set.Units = append(set.Units, trust.PreregUnit{Desk: fmt.Sprintf("app:d%d", i%subjects), Class: trust.ActionClass{Grant: "desk.commit", Impact: "I2"},
			Cutoff: base.AddDate(0, 0, 7*(i%10)), PriorItems: 3 + (i/2)%4, HasRow: true, LCB: lcb, Mean: lcb + 0.1, SuccessRate: 0.5 + float64(i%3)/10,
			ImpactRank: 2, RecencyHours: float64(i % 11), WindowActs: 1, Positive: pos})
	}
	return set
}

func testByID(r Report, id string) TestResult {
	for _, t := range r.Tests {
		if t.ID == id {
			return t
		}
	}
	return TestResult{}
}

func TestSealedBelowMinimumN(t *testing.T) {
	in := Inputs{Schema: InputsSchema, ComputerID: "c", FetchedAt: time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC),
		Served: map[string]bool{"prereg_units": true}, PreregUnits: units(40, 6, true), GateReplay: &trust.GateReport{}}
	// 20 positives: below P2's 30.
	r := Compute(in)
	for _, id := range []string{"P1", "P2"} {
		tr := testByID(r, id)
		if !tr.Sealed || tr.Status != "unresolved" || len(tr.Metrics) != 0 || len(tr.Baselines) != 0 {
			t.Fatalf("H1: %s computed below the minimum n: %+v", id, tr)
		}
		if tr.N["positive_units"] != 20 {
			t.Fatalf("%s counts %v", id, tr.N)
		}
	}
	// H2: tests with no data say so with n=0.
	for _, id := range []string{"P3", "P4", "P6", "P7", "P8", "P9"} {
		tr := testByID(r, id)
		if tr.N["units"] != 0 || !strings.HasPrefix(tr.Reason, "no data source") || len(tr.Metrics) != 0 {
			t.Fatalf("H2: %s %+v", id, tr)
		}
	}
	if r.Drills.N != 0 || !strings.HasPrefix(r.Drills.Note, "n=0") {
		t.Fatalf("drills %+v", r.Drills)
	}
}

func TestEvaluatedAtMinimumN(t *testing.T) {
	in := Inputs{Schema: InputsSchema, ComputerID: "c", FetchedAt: time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC),
		Served: map[string]bool{"prereg_units": true}, PreregUnits: units(80, 7, true),
		GateReplay: &trust.GateReport{Acts: 4, Gated: 2, ActsByDay: map[string]int{"2026-11-01": 2, "2026-11-03": 2}, GatedByDay: map[string]int{"2026-11-01": 2}}}
	r := Compute(in)
	p2 := testByID(r, "P2")
	if p2.Sealed || p2.Metrics["auc"] != 1 {
		t.Fatalf("H5: a perfectly separating lower bound scored %v (%+v)", p2.Metrics["auc"], p2)
	}
	for _, b := range p2.Baselines {
		if b.Name == "b1_allowlist_grant_impact" && !b.Degenerate {
			t.Fatal("H5: a constant grant impact was not marked degenerate")
		}
	}
	if p2.Status != "supported" {
		t.Fatalf("P2 status %s: %s %v %+v", p2.Status, p2.Reason, p2.Metrics, p2.Baselines)
	}
	// taps over three days: 2, 0, 0.
	if r.GateTaps.DaysCounted != 3 || r.GateTaps.TapsPerDayMedian != 0 || r.GateTaps.TapsPerDayP90 != 1.6 {
		t.Fatalf("taps %+v", r.GateTaps)
	}
	// H6: the bootstrap reproduces.
	if again := testByID(Compute(in), "P2"); again.Metrics["auc_difference_lo95"] != p2.Metrics["auc_difference_lo95"] {
		t.Fatal("H6: the bootstrap is not reproducible")
	}
}

func TestReproducesBytesFromStoredInputs(t *testing.T) {
	in := Inputs{Schema: InputsSchema, ComputerID: "c", FetchedAt: time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC),
		Served: map[string]bool{"prereg_units": true, "gated_acts": true}, PreregUnits: units(80, 8, false),
		GateReplay: &trust.GateReport{Acts: 1, Gated: 1, ActsByDay: map[string]int{"2026-11-01": 1}, GatedByDay: map[string]int{"2026-11-01": 1}},
		Leases:     []LeaseObs{{Holder: "app:x", State: "ended", IssuedAt: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), EndedAt: func() *time.Time { x := time.Date(2026, 11, 1, 1, 0, 0, 0, time.UTC); return &x }()}},
		Ledger:     ParseLedger("ledger.md", ledgerText, nil)}
	j1, m1, err := Render(Compute(in))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var back Inputs
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	j2, m2, err := Render(Compute(back))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(j1, j2) || !bytes.Equal(m1, m2) {
		t.Fatal("H3: a JSON round trip of the inputs changed the report bytes")
	}
	for i := 0; i < 3; i++ {
		j3, m3, _ := Render(Compute(in))
		if !bytes.Equal(j1, j3) || !bytes.Equal(m1, m3) {
			t.Fatal("H3: the same inputs gave different bytes")
		}
	}
}
