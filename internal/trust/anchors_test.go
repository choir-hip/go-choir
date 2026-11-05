package trust

// Failure modes (MA7.10), written first:
//  A1 a desk can tell a record was spot-checked (a marker in its context,
//     pack or the record itself): the sampler returns a separate log entry,
//     never touches a record, and no desk-facing source names a marker.
//  A2 the sample is predictable or unreproducible: it is seeded by a host
//     secret and a pure function of (seed, record id), logged with the seed's
//     fingerprint, never the seed.
//  A3 scorers graded by agreement with each other settle into a cheap
//     equilibrium: profiles read the anchors only.
//  A4 two same-family judges counted as two opinions: error correlation is
//     measured on the anchor sample and discounts correlated scorers.
//  A5 a PPI interval that does not cover: coverage at the nominal rate on a
//     synthetic corpus.
//  A6 an anchor settled by a split panel: no anchor, the question stays
//     "unknown", never extrapolated.
//  A7 a gated choice randomized with no propensity logged, or with a wrong
//     one: IPW recovers the known value of an alternative not taken.
//  A8 the flip-rate sampler is not reproducible: the same recorded verdict
//     pairs give the same relevance and interval, and the sampled set is a
//     pure function of the seed.

import (
	"encoding/json"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestSpotSamplerIsSeededLoggedAndLeavesRecordsAlone(t *testing.T) {
	seed := []byte("0123456789abcdef-host-secret")
	s, err := NewSpotSampler(seed, 0.1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewSpotSampler([]byte("short"), 0.1); err == nil {
		t.Fatal("a short seed is guessable")
	}
	if _, err := NewSpotSampler(seed, 0); err == nil {
		t.Fatal("rate 0 samples nothing")
	}
	rec := subCast("c", "texture", "research_child", "m", 0, types.ImpactI1, 0.5, yesNo(0.9))
	before := mustJSON(t, rec)
	d := s.Decide(rec.RecordID, t0)
	if after := mustJSON(t, rec); before != after {
		t.Fatal("the sampler touched the record")
	}
	if d.SeedID == "" || strings.Contains(mustJSON(t, d), string(seed)) {
		t.Fatalf("the log carries the seed's fingerprint, never the seed: %+v", d)
	}
	// reproducible: the same seed and id give the same draw; another seed differs
	if s.Decide(rec.RecordID, t0.Add(time.Hour)).Draw != d.Draw {
		t.Fatal("the draw must be a pure function of the seed and the id")
	}
	s2, _ := NewSpotSampler([]byte("another-secret-seed-0123456789"), 0.1)
	same := 0
	for i := 0; i < 100; i++ {
		id := string(rune('a'+i%26)) + string(rune('A'+i/26))
		if s.Draw(id) == s2.Draw(id) {
			same++
		}
	}
	if same > 2 {
		t.Fatalf("seeds do not separate: %d equal draws", same)
	}
	// the sampled share is the rate
	n, hit := 20000, 0
	for i := 0; i < n; i++ {
		if s.Decide("rec-"+itoa(i), t0).Sampled {
			hit++
		}
	}
	if share := float64(hit) / float64(n); math.Abs(share-0.1) > 0.01 {
		t.Fatalf("sampled share %v", share)
	}
}

func itoa(i int) string {
	const digits = "0123456789"
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{digits[i%10]}, b...)
		i /= 10
	}
	return string(b)
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// No desk-facing source (prompts, packs, context framing) names a spot-check
// marker: a marker any scorer or desk can see defeats the sample.
func TestNoDeskFacingSourceCarriesASpotCheckMarker(t *testing.T) {
	roots := []string{"../runtimeprompts", "../promptspec", "../promptstore", "../textureprompts", "../types"}
	needles := []string{"spot_check", "spotcheck", "spot-check"}
	for _, root := range roots {
		_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return nil
			}
			raw, _ := os.ReadFile(p)
			low := strings.ToLower(string(raw))
			for _, n := range needles {
				if strings.Contains(low, n) {
					t.Errorf("%s names %q", p, n)
				}
			}
			return nil
		})
	}
	// the record type has no field that could carry one
	if strings.Contains(strings.ToLower(mustJSON(t, types.CommitmentRecord{})), "spot") {
		t.Fatal("CommitmentRecord carries a spot field")
	}
}

func TestAnchorSettlementByTheStrongestJudge(t *testing.T) {
	at := t0
	a, ok := SettleAnchor("i1", AnchorSpotCheck, []AnchorVote{
		{Judge: "model-a", Strength: 1, Answer: "yes"}, {Judge: "model-b", Strength: 1, Answer: "yes"},
		{Judge: "panel", Strength: 2, Answer: "no"}}, at)
	if !ok || a.Truth != "no" || len(a.Judges) != 3 {
		t.Fatalf("the strongest judge decides: %+v %v", a, ok)
	}
	if math.Abs(a.Agreement-1.0/3) > 1e-9 {
		t.Fatalf("agreement %v", a.Agreement)
	}
	// a split among the strongest: no anchor
	if _, ok := SettleAnchor("i2", AnchorSpotCheck, []AnchorVote{{Judge: "p1", Strength: 2, Answer: "yes"}, {Judge: "p2", Strength: 2, Answer: "no"}}, at); ok {
		t.Fatal("a split panel must not settle an anchor")
	}
	if _, ok := SettleAnchor("i3", AnchorWorld, nil, at); ok {
		t.Fatal("no votes, no anchor")
	}
}

// synthetic scorers: good is accurate; sycophant agrees with a peer but not
// with the truth; twin copies good's errors.
func scorerFixture() ([]ScorerObservation, map[string]Anchor) {
	rng := rand.New(rand.NewSource(3))
	anchors := map[string]Anchor{}
	var obs []ScorerObservation
	for i := 0; i < 200; i++ {
		id := "item-" + itoa(i)
		truth := "yes"
		if rng.Float64() < 0.5 {
			truth = "no"
		}
		anchors[id] = Anchor{ItemID: id, Kind: AnchorSpotCheck, Truth: truth}
		other := map[string]string{"yes": "no", "no": "yes"}[truth]
		goodWrong := rng.Float64() < 0.1
		goodAns := truth
		if goodWrong {
			goodAns = other
		}
		sycAns := other
		if rng.Float64() < 0.4 {
			sycAns = truth
		}
		twinAns := goodAns // same family, copies good's errors
		mk := func(scorer, fam, ans string, conf float64) ScorerObservation {
			p := map[string]float64{ans: conf, map[string]string{"yes": "no", "no": "yes"}[ans]: 1 - conf}
			return ScorerObservation{Scorer: scorer, Family: fam, QuestionType: "report_answers_question", ItemID: id, Answers: []string{"yes", "no"}, Probs: p,
				Position: i % 2, Length: 100 + i, CandidateFamily: "other"}
		}
		obs = append(obs, mk("good", "alpha", goodAns, 0.85), mk("twin", "alpha", twinAns, 0.85), mk("syc", "beta", sycAns, 0.95))
	}
	return obs, anchors
}

func TestScorerProfilesReadAnchorsNotPeerAgreement(t *testing.T) {
	obs, anchors := scorerFixture()
	profs := ScorerProfiles(obs, anchors)
	by := map[string]ScorerProfile{}
	for _, p := range profs {
		by[p.Scorer] = p
	}
	good, syc := by["good"], by["syc"]
	gt, st := good.Type("report_answers_question"), syc.Type("report_answers_question")
	if !gt.Known || !st.Known || gt.N != 200 {
		t.Fatalf("%+v %+v", gt, st)
	}
	if !(gt.Accuracy > 0.85 && st.Accuracy < 0.5) {
		t.Fatalf("accuracy good %v syc %v", gt.Accuracy, st.Accuracy)
	}
	if !(gt.AUC > 0.8 && st.AUC < 0.5) {
		t.Fatalf("discrimination good %v syc %v", gt.AUC, st.AUC)
	}
	if !(gt.Brier < st.Brier) || st.ECE < 0.3 {
		t.Fatalf("calibration good %+v syc %+v", gt, st)
	}
	// a question type with no anchors is unknown, not extrapolated
	other := ScorerProfiles([]ScorerObservation{{Scorer: "good", QuestionType: "never_anchored", ItemID: "zz", Answers: []string{"yes", "no"}, Probs: map[string]float64{"yes": 1}}}, anchors)
	if tp := other[0].Type("never_anchored"); tp.Known || tp.N != 0 {
		t.Fatalf("unanchored type must be unknown: %+v", tp)
	}
}

func TestBiasProfileTestRetestAndExploitability(t *testing.T) {
	anchors := map[string]Anchor{}
	var obs []ScorerObservation
	// a position-biased scorer accepts whatever is shown first
	for i := 0; i < 60; i++ {
		id := "p" + itoa(i)
		anchors[id] = Anchor{ItemID: id, Truth: []string{"yes", "no"}[(i/2)%2]}
		ans, conf := "no", 0.9
		if i%2 == 0 {
			ans = "yes"
		}
		obs = append(obs, ScorerObservation{Scorer: "biased", Family: "f", QuestionType: "q", ItemID: id, Answers: []string{"yes", "no"},
			Probs: map[string]float64{ans: conf, map[string]string{"yes": "no", "no": "yes"}[ans]: 1 - conf}, Position: i % 2, Length: 50, CandidateFamily: "g"})
	}
	// planted meaningless items: the scorer accepts 3 of 10
	for i := 0; i < 10; i++ {
		id := "planted-" + itoa(i)
		anchors[id] = Anchor{ItemID: id, Truth: "no"}
		ans := "no"
		if i < 3 {
			ans = "yes"
		}
		obs = append(obs, ScorerObservation{Scorer: "biased", Family: "f", QuestionType: "q", ItemID: id, Answers: []string{"yes", "no"},
			Probs: map[string]float64{ans: 0.9, map[string]string{"yes": "no", "no": "yes"}[ans]: 0.1}, Planted: PlantedEmpty})
	}
	// retest: the same items again, flipping a third of the answers
	for i := 0; i < 30; i++ {
		id := "p" + itoa(i)
		ans := "yes"
		if (i%2 != 0) != (i%3 == 0) {
			ans = "no"
		}
		obs = append(obs, ScorerObservation{Scorer: "biased", Family: "f", QuestionType: "q", ItemID: id, Answers: []string{"yes", "no"},
			Probs: map[string]float64{ans: 0.9, map[string]string{"yes": "no", "no": "yes"}[ans]: 0.1}, Position: i % 2, Run: 1, CandidateFamily: "g"})
	}
	p := ScorerProfiles(obs, anchors)[0]
	if !(p.PositionBias.N > 0 && p.PositionBias.Value > 0.5) {
		t.Fatalf("position bias %+v", p.PositionBias)
	}
	if p.Exploitability.N != 10 || math.Abs(p.Exploitability.Value-0.3) > 1e-9 {
		t.Fatalf("exploitability %+v", p.Exploitability)
	}
	if p.TestRetest.N != 30 || !(p.TestRetest.Value > 0.5 && p.TestRetest.Value < 0.9) {
		t.Fatalf("test-retest %+v", p.TestRetest)
	}
}

func TestErrorCorrelationDiscountsSameFamilyJudges(t *testing.T) {
	obs, anchors := scorerFixture()
	corr := ErrorCorrelation(obs, anchors)
	find := func(a, b string) PairCorr {
		for _, c := range corr {
			if (c.A == a && c.B == b) || (c.A == b && c.B == a) {
				return c
			}
		}
		t.Fatalf("no pair %s %s", a, b)
		return PairCorr{}
	}
	twin := find("good", "twin")
	if !twin.Defined || twin.Rho < 0.95 || !twin.SameFamily {
		t.Fatalf("a copy's errors correlate: %+v", twin)
	}
	cross := find("good", "syc")
	if math.Abs(cross.Rho) > 0.3 || cross.SameFamily {
		t.Fatalf("independent errors: %+v", cross)
	}
	wGood, wSyc := IndependentWeight("good", corr), IndependentWeight("syc", corr)
	if !(wGood < 0.6) || !(wSyc > wGood) {
		t.Fatalf("correlated agreement must not count twice: good %v syc %v", wGood, wSyc)
	}
	if w := IndependentWeight("nobody", corr); w != 1 {
		t.Fatalf("an unmeasured scorer keeps weight 1 only when it has no pairs: %v", w)
	}
}

func TestPPIIntervalCoversAtTheNominalRate(t *testing.T) {
	rng := rand.New(rand.NewSource(11))
	const trials, bigN, smallN = 600, 2000, 120
	truth := 0.62
	cover, widthPPI, widthClassical := 0, 0.0, 0.0
	for trial := 0; trial < trials; trial++ {
		// a cheap judge: truth plus noise and a bias of +0.05
		ys := make([]float64, bigN)
		fs := make([]float64, bigN)
		for i := range ys {
			if rng.Float64() < truth {
				ys[i] = 1
			}
			fs[i] = math.Max(0, math.Min(1, ys[i]*0.8+0.1+rng.NormFloat64()*0.15+0.05))
		}
		lab := rng.Perm(bigN)[:smallN]
		var fl, yl []float64
		for _, i := range lab {
			fl = append(fl, fs[i])
			yl = append(yl, ys[i])
		}
		pop := truth // the known true score the corpus was drawn from
		iv, err := PPIMean(fs, fl, yl, 0.05)
		if err != nil {
			t.Fatal(err)
		}
		if iv.Lo <= pop && pop <= iv.Hi {
			cover++
		}
		widthPPI += iv.Hi - iv.Lo
		cl := ClassicalMean(yl, 0.05)
		widthClassical += cl.Hi - cl.Lo
	}
	rate := float64(cover) / trials
	if rate < 0.92 || rate > 0.985 {
		t.Fatalf("coverage %v at nominal 0.95", rate)
	}
	if !(widthPPI < widthClassical) {
		t.Fatalf("cheap labels cut variance: ppi %v classical %v", widthPPI/trials, widthClassical/trials)
	}
	if _, err := PPIMean([]float64{1}, []float64{1, 1}, []float64{1}, 0.05); err == nil {
		t.Fatal("mismatched labeled arrays are refused")
	}
}

func TestRandomizedChoiceLogsPropensityAndIPWRecoversTheAlternative(t *testing.T) {
	r, err := NewRandomizer([]byte("seed-seed-seed-seed-seed-1234"), 0.1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewRandomizer([]byte("seed-seed-seed-seed-seed-1234"), 0.6); err == nil {
		t.Fatal("the held-out share is small")
	}
	alts := []string{"A", "B", "C"}
	value := map[string]float64{"A": 0.8, "B": 0.5, "C": 0.2} // known truth of each alternative
	rng := rand.New(rand.NewSource(5))
	var obs []ChoiceOutcome
	held := 0
	n := 40000
	for i := 0; i < n; i++ {
		c := r.Choose("decision-"+itoa(i), alts, "A")
		if c.Propensity <= 0 || c.Propensity > 1 || c.SeedID == "" {
			t.Fatalf("propensity not logged: %+v", c)
		}
		if c.Randomized {
			held++
		}
		y := 0.0
		if rng.Float64() < value[c.Taken] {
			y = 1
		}
		obs = append(obs, ChoiceOutcome{Choice: c, Outcome: y})
	}
	if share := float64(held) / float64(n); math.Abs(share-0.1) > 0.01 {
		t.Fatalf("held-out share %v", share)
	}
	if got := r.Choose("decision-7", alts, "A"); got != r.Choose("decision-7", alts, "A") {
		t.Fatal("the choice must be a pure function of the seed and the decision")
	}
	for alt, want := range value {
		est, ok := IPWValue(obs, alt)
		if !ok {
			t.Fatalf("no estimate for %s", alt)
		}
		if math.Abs(est-want) > 0.05 {
			t.Errorf("IPW(%s) = %v, want %v", alt, est, want)
		}
	}
	// the naive mean over the taken action is biased for B (rarely taken, and
	// only through the randomized share): IPW is the honest one
	if _, ok := IPWValue(nil, "A"); ok {
		t.Fatal("no data, no estimate")
	}
}

func TestFlipRateRelevanceReproducesOnReplay(t *testing.T) {
	pairs := []FlipPair{{"allow", "allow"}, {"deny", "allow"}, {"deny", "allow"}, {"allow", "allow"}, {"deny", "deny"}, {"allow", "deny"}}
	a, b := Relevance(pairs), Relevance(pairs)
	if a != b {
		t.Fatal("replay must reproduce")
	}
	if a.N != 6 || a.Flips != 3 || math.Abs(a.Value-0.5) > 1e-9 || !(a.Lo < 0.5 && a.Hi > 0.5 && a.Lo >= 0 && a.Hi <= 1) {
		t.Fatalf("%+v", a)
	}
	if z := Relevance([]FlipPair{{"x", "x"}, {"y", "y"}, {"z", "z"}}); z.Value != 0 || z.Hi <= 0 {
		t.Fatalf("no flips: relevance zero with an honest upper bound: %+v", z)
	}
	if Relevance(nil).N != 0 {
		t.Fatal("empty")
	}
	// measure with a decider: reveals flip it half the time
	calls := 0
	rel, err := MeasureRelevance(6, func(reveal bool, sample int) (string, error) {
		calls++
		if reveal && sample%2 == 0 {
			return "deny", nil
		}
		return "allow", nil
	})
	if err != nil || calls != 12 || rel.Flips != 3 || rel.N != 6 {
		t.Fatalf("%v %d %+v", err, calls, rel)
	}
	// the sampled set is a pure function of the seed
	fs := NewFlipSampler([]byte("flip-seed-flip-seed-1234"), 0.25)
	n := 0
	for i := 0; i < 4000; i++ {
		x := fs.Sample("rec-"+itoa(i), "consequence")
		if x != fs.Sample("rec-"+itoa(i), "consequence") {
			t.Fatal("not a pure function")
		}
		if x {
			n++
		}
	}
	if share := float64(n) / 4000; math.Abs(share-0.25) > 0.03 {
		t.Fatalf("flip sample share %v", share)
	}
}
