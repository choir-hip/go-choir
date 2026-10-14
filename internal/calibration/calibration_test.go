package calibration

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

// E6a's management bins (box-score.json, calibration_resolved): the digest's
// ECE over them must be the experiment's preregistered baseline, 0.144.
func TestMeasureReproducesTheE6aBaselineECE(t *testing.T) {
	e6a := []struct {
		n       int
		p, y    float64
		wantLbl string
	}{
		{57, 0.373, 0.623, "<0.5"}, {250, 0.569, 0.752, "0.5-0.7"}, {176, 0.744, 0.844, "0.7-0.85"},
		{67, 0.878, 0.903, "0.85-0.95"}, {9, 0.964, 0.833, ">=0.95"},
	}
	var obs []Observation
	wantECE, total := 0.0, 0
	for _, b := range e6a {
		for i := 0; i < b.n; i++ {
			obs = append(obs, Observation{Desk: "management", ClaimType: types.CalibrationClaimHypothesis, P: b.p, Y: b.y, Key: fmt.Sprintf("%s-%d", b.wantLbl, i)})
		}
		wantECE += float64(b.n) * math.Abs(b.p-b.y)
		total += b.n
	}
	wantECE /= float64(total)
	st := Measure(obs)
	if st.N != 559 || !near(st.ECE, wantECE) {
		t.Fatalf("N=%d ECE=%v, want 559 and %v", st.N, st.ECE, wantECE)
	}
	if math.Round(st.ECE*1000)/1000 != 0.144 {
		t.Fatalf("ECE = %.4f, want the preregistered 0.144", st.ECE)
	}
	for i, b := range st.Bins {
		if b.Label != e6a[i].wantLbl || b.N != e6a[i].n || !near(b.MeanP, e6a[i].p) || !near(b.Observed, e6a[i].y) {
			t.Fatalf("bin %d = %+v, want %+v", i, b, e6a[i])
		}
	}
	if direction(st.MeanP, st.Observed) != "under-confident" {
		t.Fatalf("direction = %s", direction(st.MeanP, st.Observed))
	}
}

func TestMeasureBrierOnContinuousOutcomes(t *testing.T) {
	obs := []Observation{{P: 1, Y: 0.65}, {P: 0.8, Y: 1}, {P: 0.2, Y: 0}, {P: 0.6, Y: 0.5}}
	st := Measure(obs)
	want := (0.35*0.35 + 0.2*0.2 + 0.2*0.2 + 0.1*0.1) / 4
	if !near(st.Brier, want) || !near(st.MeanP, 0.65) || !near(st.Observed, 0.5375) {
		t.Fatalf("stats = %+v, want brier %v", st, want)
	}
	if got := Measure(nil); got.N != 0 || got.Brier != 0 || len(got.Bins) != 0 {
		t.Fatalf("empty = %+v", got)
	}
}

func TestBinEdges(t *testing.T) {
	for p, want := range map[float64]int{0: 0, 0.49: 0, 0.5: 1, 0.7: 2, 0.85: 3, 0.95: 4, 1: 4, -0.1: 0, 1.2: 4} {
		if got := BinOf(p); got != want {
			t.Errorf("BinOf(%v) = %d, want %d", p, got, want)
		}
	}
}

func obsN(desk, claim, model string, n int, p, y float64, at string) []Observation {
	out := make([]Observation, n)
	for i := range out {
		out[i] = Observation{Desk: desk, ClaimType: claim, ModelID: model, P: p, Y: y, Key: fmt.Sprintf("%s-%s-%s-%d", desk, claim, at, i), CommittedAt: at}
	}
	return out
}

// P4/held-out, the window, and the desk filter.
func TestComputeExcludesHeldOutOtherDesksAndOldObservations(t *testing.T) {
	var obs []Observation
	obs = append(obs, obsN("management", "hypothesis", "m1", 30, 0.6, 1, "2026-10-14T10:00:00Z")...)
	obs = append(obs, obsN("management", "hypothesis", "m1", 30, 0.6, 0, "2026-10-13T10:00:00Z")...) // older: cut by the window
	obs = append(obs, obsN("research", "completed", "m1", 30, 1, 0, "2026-10-14T10:00:00Z")...)
	held := obsN("management", "hypothesis", "m1", 7, 0.1, 1, "2026-10-14T11:00:00Z")
	for i := range held {
		held[i].HeldOut = true
	}
	obs = append(obs, held...)
	d := Compute("management", obs, Options{Window: 30, MinN: 20, MinBin: 5})
	if d.N != 30 || d.HeldOut != 7 {
		t.Fatalf("N=%d held_out=%d, want 30 and 7", d.N, d.HeldOut)
	}
	if !near(d.Observed, 1) || !near(d.MeanP, 0.6) || d.Direction != "under-confident" {
		t.Fatalf("digest = %+v, want only the newest 30 (all held, p 0.6)", d)
	}
	if strings.Contains(d.Text, "0.10") {
		t.Fatalf("held-out probabilities leaked into the text:\n%s", d.Text)
	}
}

// P5: no text under MinN; small bins and claim types omitted.
func TestComputeSmallSamples(t *testing.T) {
	d := Compute("texture", obsN("texture", "citation_support", "", 19, 0.9, 1, "t"), DefaultOptions)
	if d.N != 19 || d.Text != "" {
		t.Fatalf("19 observations: N=%d text=%q, want no text", d.N, d.Text)
	}
	obs := append(obsN("texture", "citation_support", "", 40, 0.9, 1, "t"), obsN("texture", "hypothesis", "", 4, 0.3, 0, "t")...)
	d = Compute("texture", obs, DefaultOptions)
	if len(d.Bins) != 1 || d.Bins[0].Label != "0.85-0.95" {
		t.Fatalf("bins = %+v, want the 4-observation bin omitted", d.Bins)
	}
	if len(d.ClaimTypes) != 1 || d.ClaimTypes[0].Key != "citation_support" {
		t.Fatalf("claim types = %+v, want the 4-observation type omitted", d.ClaimTypes)
	}
}

func TestComputeWorstClaimTypeAndModelLine(t *testing.T) {
	obs := append(obsN("texture", "citation_support", "chatgpt/gpt-6-luna", 30, 0.9, 1, "t"),
		obsN("texture", "hypothesis", "opencode-go/deepseek-v4.1-flash", 25, 0.9, 0.2, "t")...)
	d := Compute("texture", obs, Options{ModelID: "opencode-go/deepseek-v4.1-flash"})
	if d.Worst != "hypothesis" {
		t.Fatalf("worst = %q, want hypothesis", d.Worst)
	}
	if d.Model == nil || d.Model.N != 25 || !near(d.Model.Observed, 0.2) {
		t.Fatalf("model line = %+v", d.Model)
	}
	if !strings.Contains(d.Text, "Least calibrated: hypothesis probabilities") || !strings.Contains(d.Text, "Your model (opencode-go/deepseek-v4.1-flash)") {
		t.Fatalf("text:\n%s", d.Text)
	}
	if d2 := Compute("texture", obs, Options{ModelID: "chatgpt/unknown"}); d2.Model != nil {
		t.Fatalf("a model with no observations got a line: %+v", d2.Model)
	}
}

// P2: the text is a pure function of the record: same observations in any
// order, same text; it carries no time.
func TestRenderIsStable(t *testing.T) {
	obs := append(obsN("management", "hypothesis", "m", 25, 0.55, 0.8, "2026-10-14T10:00:00Z"), obsN("management", "hypothesis", "m", 25, 0.75, 0.85, "2026-10-14T09:00:00Z")...)
	a := Compute("management", obs, DefaultOptions)
	rev := make([]Observation, len(obs))
	for i := range obs {
		rev[len(obs)-1-i] = obs[i]
	}
	b := Compute("management", rev, DefaultOptions)
	if a.Text == "" || a.Text != b.Text {
		t.Fatalf("text depends on order:\n%s\n---\n%s", a.Text, b.Text)
	}
	if strings.Contains(a.Text, "2026") {
		t.Fatalf("text carries a time:\n%s", a.Text)
	}
	if len(a.Text) > 1200 {
		t.Fatalf("text is %d bytes; the digest is a short block", len(a.Text))
	}
}

// The arm and held-out rules are fixed and mirrored by
// scripts/icrl_calibration.py; these values were computed there.
func TestArmAndHeldOutAreTheScriptsRules(t *testing.T) {
	for thread, want := range map[string]string{"texture:doc-1": "digest", "management-task:t-1": "digest", "research:doc-1": "control", "management:owner-1": "control"} {
		if got := Arm(thread); got != want {
			t.Errorf("Arm(%s) = %s, want %s", thread, got, want)
		}
	}
	for key, want := range map[string]bool{"rec-1#H1": false, "rec-2#H1": true, "sp-abc": false} {
		if got := HeldOut(key); got != want {
			t.Errorf("HeldOut(%s) = %v, want %v", key, got, want)
		}
	}
	digest, held := 0, 0
	for i := 0; i < 2000; i++ {
		if Arm(fmt.Sprintf("texture:doc-%d", i)) == "digest" {
			digest++
		}
		if HeldOut(fmt.Sprintf("rec-%d", i)) {
			held++
		}
	}
	if digest < 900 || digest > 1100 || held < 150 || held > 250 {
		t.Fatalf("digest arm %d/2000, held out %d/2000: want about half and a tenth", digest, held)
	}
}
