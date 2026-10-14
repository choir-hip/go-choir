package claimslab

import (
	"math"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/quotematch"
)

func near(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

func TestCheckQuoteIgnoresWrongBlockHint(t *testing.T) { // failure modes 2, 3
	blocks := []quotematch.Block{{ID: "b0", Text: "Intro."}, {ID: "b1", Text: "The ministry said about 600,000 consumers were without power."}}
	got := CheckQuote("about 600,000 consumers were without power", "b0", blocks)
	if !got.Retained || got.Result.BlockID != "b1" || got.HintMismatch != true {
		t.Fatalf("got %+v", got)
	}
	got = CheckQuote("about 600,000 consumers were without power", "missing", blocks)
	if !got.Retained || got.Result.BlockID != "b1" {
		t.Fatalf("missing hint: %+v", got)
	}
}

func TestCheckQuoteRejectsEmptyAndOverCap(t *testing.T) { // failure mode 4
	blocks := []quotematch.Block{{ID: "b0", Text: "x"}}
	if got := CheckQuote("  ... ", "", blocks); got.Retained || got.Reason != ReasonEmpty {
		t.Fatalf("empty: %+v", got)
	}
	long := make([]byte, QuoteCap+1)
	for i := range long {
		long[i] = 'a'
	}
	if got := CheckQuote(string(long), "", []quotematch.Block{{ID: "b0", Text: string(long)}}); got.Retained || got.Reason != ReasonOverCap {
		t.Fatalf("over cap: %+v", got)
	}
	if got := CheckQuoteCap(string(long), "", []quotematch.Block{{ID: "b0", Text: string(long)}}, 0); !got.Retained {
		t.Fatalf("no cap must retain a long matching quote: %+v", got)
	}
}

func TestCheckQuoteReportsTolerantTier(t *testing.T) { // failure mode 1
	blocks := []quotematch.Block{{ID: "b0", Text: "He said: “We will ship by October.”"}}
	got := CheckQuote("he said: \"we will ship by october.\"", "", blocks)
	if !got.Retained || got.Result.Tier == quotematch.TierExact {
		t.Fatalf("want tolerant retained, got %+v", got)
	}
}

func TestMatchSpansGreedyOneToOne(t *testing.T) { // failure modes 5, 6
	gold := []Span{{Block: "b1", Start: 0, End: 50}, {Block: "b2", Start: 0, End: 40}}
	pred := []Span{{Block: "b1", Start: 10, End: 50}, {Block: "b1", Start: 0, End: 45}, {Block: "b3", Start: 0, End: 40}}
	m := MatchSpans(gold, pred, 0.5)
	if m.TruePositives != 1 || m.Recall() != 0.5 || !near(m.Precision(), 1.0/3, 1e-9) {
		t.Fatalf("got %+v", m)
	}
}

func TestBrierAndMurphySum(t *testing.T) { // failure modes 7, 8
	ps := []float64{0.9, 0.9, 0.2, 0.2, 0.6}
	ys := []float64{1, 0, 0, 0, 1}
	b := Brier(ps, ys)
	if !b.Valid || !near(b.Brier, (0.01+0.81+0.04+0.04+0.16)/5, 1e-12) {
		t.Fatalf("brier %+v", b)
	}
	if !near(b.Reliability-b.Resolution+b.Uncertainty, b.Brier, 1e-12) {
		t.Fatalf("murphy does not sum: %+v", b)
	}
	if e := Brier(nil, nil); e.Valid {
		t.Fatalf("empty must be invalid")
	}
}

func TestNEffAndInterval(t *testing.T) { // failure modes 9, 10
	if n := NEff([]float64{1, 1, 1, 1}); !near(n, 4, 1e-12) {
		t.Fatalf("equal weights n_eff %v", n)
	}
	if n := NEff([]float64{100, 1, 1}); n > 1.1 {
		t.Fatalf("dominant weight n_eff %v", n)
	}
	iv := WeightedRate([]float64{1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, []float64{1, 1, 1, 1, 1, 1, 1, 1, 0, 0.5})
	if !iv.Valid || !near(iv.Mean, 0.85, 1e-12) || !(iv.Lo < 0.85 && iv.Hi > 0.85 && iv.Lo > 0.4 && iv.Hi < 1) {
		t.Fatalf("interval %+v", iv)
	}
	if z := WeightedRate([]float64{0}, []float64{1}); z.Valid {
		t.Fatalf("zero weight must be invalid")
	}
	// Beta(1+8,1+2) central 95% interval is about [0.48, 0.94].
	b := BetaInterval(8, 2, 0.95)
	if !near(b[0], 0.482, 0.01) || !near(b[1], 0.940, 0.01) {
		t.Fatalf("beta interval %v", b)
	}
}
