package trustharness

// Failure modes of the anchor tests P6 and P7 (MA7.10 wiring), written first:
//  A1 an outcome is computed below the minimum n before the deadline (an
//     interim look): the test reports its counts and is sealed.
//  A2 P6 or P7 still says "no data source" when the computer served an
//     anchored sample.
//  A3 the same stored inputs give different numbers (an unseeded draw).
//  A4 P7 reads coverage of the wrong target: it is the coverage of the pool
//     mean, over 1,000 seeded subsamples at n of 20, 40 and 80.
//  A5 P6 grades peer agreement as if it were truth: on a synthetic corpus
//     where the scorers' errors are correlated, the spot-check estimate of a
//     scorer's accuracy is closer to its true accuracy than the peer estimate.
//  A6 an older build (no anchor_sample) is reported as zero rather than as
//     not served.

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/trust"
)

func other(a string) string { return map[string]string{"yes": "no", "no": "yes"}[a] }

// ppiCorpus is one agent on one type: n items, the first nAnch anchored. The
// agent is right with probability pAgent, the judge with pJudge.
func ppiCorpus(agent, typ string, n, nAnch int, pAgent, pJudge float64, seed int64) []trust.CorpusRow {
	rng := rand.New(rand.NewSource(seed))
	var rows []trust.CorpusRow
	for i := 0; i < n; i++ {
		truth := []string{"yes", "no"}[rng.Intn(2)]
		ag, jd := truth, truth
		if rng.Float64() >= pAgent {
			ag = other(truth)
		}
		if rng.Float64() >= pJudge {
			jd = other(truth)
		}
		r := trust.CorpusRow{Agent: agent, QuestionType: typ, ItemID: fmt.Sprintf("%s-%d#q0", agent, i), AgentAnswer: ag, JudgeAnswer: jd}
		if i < nAnch {
			r.Anchored, r.Truth = true, truth
		}
		rows = append(rows, r)
	}
	return rows
}

func inputsWith(s *trust.AnchorSample) Inputs {
	return Inputs{Schema: InputsSchema, ComputerID: "computer-x", FetchedAt: time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC),
		Served: map[string]bool{"anchor_sample": s != nil}, AnchorSample: s}
}

func anchorTestByID(t *testing.T, in Inputs, id string) TestResult {
	t.Helper()
	for _, r := range Compute(in).Tests {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("no test %s", id)
	return TestResult{}
}

func TestP7IsSealedBelowTheMinimumAndComputedAtIt(t *testing.T) {
	small := &trust.AnchorSample{Corpus: ppiCorpus("a", "t", 500, 100, 0.8, 0.75, 1)}
	r := anchorTestByID(t, inputsWith(small), "P7")
	if !r.Sealed || r.Status != "unresolved" || r.N["max_pool"] != 100 || r.Metrics != nil {
		t.Fatalf("A1: %+v", r)
	}
	if r.Reason == "" || contains(r.Reason, "no data source") {
		t.Fatalf("A2: %q", r.Reason)
	}
	rows := append(ppiCorpus("a", "t", 700, 320, 0.80, 0.75, 1), ppiCorpus("b", "t", 700, 320, 0.60, 0.85, 2)...)
	full := &trust.AnchorSample{Corpus: rows}
	r = anchorTestByID(t, inputsWith(full), "P7")
	if r.Sealed || r.N["agents_with_pool"] != 2 {
		t.Fatalf("P7 at the minimum: %+v", r)
	}
	for _, k := range []string{"coverage_a_n20", "coverage_a_n40", "coverage_a_n80", "coverage_b_n80", "saving_a_n80", "saving_b_n80"} {
		if v, ok := r.Metrics[k]; !ok || v <= 0 {
			t.Fatalf("A4: metric %s = %v (%v)", k, v, r.Metrics)
		}
	}
	// 90% intervals cover the pool mean at about the nominal rate
	for _, k := range []string{"coverage_a_n40", "coverage_b_n40"} {
		if c := r.Metrics[k]; c < 0.80 || c > 1.0 {
			t.Fatalf("A4: %s = %v", k, c)
		}
	}
	// A3: byte-identical on a recompute
	a, _, _ := Render(Compute(inputsWith(full)))
	b, _, _ := Render(Compute(inputsWith(full)))
	if string(a) != string(b) {
		t.Fatal("A3: P7 is not reproducible")
	}
}

func p6Sample(seed int64, nItems int) *trust.AnchorSample {
	rng := rand.New(rand.NewSource(seed))
	s := &trust.AnchorSample{}
	for i := 0; i < nItems; i++ {
		typ := []string{"research_web", "research_deep"}[i%2]
		agent := fmt.Sprintf("agent%d", i%3)
		truth := []string{"yes", "no"}[rng.Intn(2)]
		// the judge is right 90%; the three agents copy the judge's mistakes
		// (correlated errors) and are otherwise right 70% of the time
		jd := truth
		if rng.Float64() >= 0.9 {
			jd = other(truth)
		}
		ag := jd
		if jd == truth && rng.Float64() >= 0.75 {
			ag = other(truth)
		}
		s.Items = append(s.Items, trust.AnchorItem{ItemID: fmt.Sprintf("i%d#q0", i), Agent: agent, QuestionType: typ, Choices: []string{"yes", "no"},
			Answers: []trust.ScorerAnswer{{Scorer: "system:judge", Answer: jd}, {Scorer: "self:" + agent, Answer: ag}},
			Truth:   truth, Kind: trust.AnchorSpotCheck})
	}
	return s
}

func TestP6SpotChecksBeatPeerAgreementOnCorrelatedErrors(t *testing.T) {
	small := p6Sample(1, 60)
	r := anchorTestByID(t, inputsWith(small), "P6")
	if !r.Sealed || r.Status != "unresolved" || contains(r.Reason, "no data source") || r.N["anchored_items"] != 60 {
		t.Fatalf("A1/A2: %+v", r)
	}
	r = anchorTestByID(t, inputsWith(p6Sample(1, 900)), "P6")
	if r.Sealed || r.N["scorers"] < 4 || r.N["question_types"] != 2 {
		t.Fatalf("P6 at the minimum: %+v", r)
	}
	spot, peer := r.Metrics["spot_mae_nb50"], r.Metrics["peer_mae_nb50"]
	if !(spot > 0 && peer > 0 && spot < peer) {
		t.Fatalf("A5: spot MAE %v, peer MAE %v (%v)", spot, peer, r.Metrics)
	}
	if _, ok := r.Metrics["difference_lo95_nb50"]; !ok {
		t.Fatalf("no interval: %v", r.Metrics)
	}
	a, _, _ := Render(Compute(inputsWith(p6Sample(1, 900))))
	b, _, _ := Render(Compute(inputsWith(p6Sample(1, 900))))
	if string(a) != string(b) {
		t.Fatal("A3: P6 is not reproducible")
	}
}

func TestAnchorTestsReportNotServedForAnOlderBuild(t *testing.T) {
	for _, id := range []string{"P6", "P7"} {
		r := anchorTestByID(t, inputsWith(nil), id)
		if r.Status != "unresolved" || !contains(r.Reason, "did not serve") {
			t.Fatalf("A6: %s %+v", id, r)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// D3: the minimum is two agents, not two pools: one agent with a full pool in
// each of two question types stays sealed.
func TestP7OneAgentWithTwoFullPoolsStaysSealed(t *testing.T) {
	rows := append(ppiCorpus("a", "t1", 700, 320, 0.80, 0.75, 1), ppiCorpus("a", "t2", 700, 320, 0.80, 0.75, 2)...)
	r := anchorTestByID(t, inputsWith(&trust.AnchorSample{Corpus: rows}), "P7")
	if !r.Sealed || r.N["pools"] != 2 || r.N["agents_with_pool"] != 1 || r.Metrics != nil {
		t.Fatalf("D3: %+v", r)
	}
}
