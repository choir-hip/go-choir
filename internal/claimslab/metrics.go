package claimslab

import (
	"math"
	"sort"
)

// SpanMatch is the result of matching extracted claims to labels.
type SpanMatch struct {
	Gold          int      `json:"gold"`
	Predicted     int      `json:"predicted"`
	TruePositives int      `json:"true_positives"`
	Pairs         [][2]int `json:"pairs"` // (gold index, predicted index)
}

// Recall is TP over labels; zero labels gives 0.
func (m SpanMatch) Recall() float64 {
	if m.Gold == 0 {
		return 0
	}
	return float64(m.TruePositives) / float64(m.Gold)
}

// Precision is TP over predictions; zero predictions gives 0.
func (m SpanMatch) Precision() float64 {
	if m.Predicted == 0 {
		return 0
	}
	return float64(m.TruePositives) / float64(m.Predicted)
}

// overlap returns the overlap of two spans as a share of the shorter one.
func overlap(a, b Span) float64 {
	if a.Block != b.Block {
		return 0
	}
	lo, hi := max(a.Start, b.Start), min(a.End, b.End)
	if hi <= lo {
		return 0
	}
	shorter := min(a.End-a.Start, b.End-b.Start)
	if shorter <= 0 {
		return 0
	}
	return float64(hi-lo) / float64(shorter)
}

// MatchSpans matches predicted spans to gold spans one to one, greedily by
// overlap (largest first), counting a pair when the overlap is at least
// minOverlap of the shorter span.
func MatchSpans(gold, pred []Span, minOverlap float64) SpanMatch {
	type cand struct {
		g, p int
		o    float64
	}
	var cands []cand
	for i, g := range gold {
		for j, p := range pred {
			if o := overlap(g, p); o >= minOverlap && o > 0 {
				cands = append(cands, cand{i, j, o})
			}
		}
	}
	sort.SliceStable(cands, func(a, b int) bool { return cands[a].o > cands[b].o })
	usedG, usedP := map[int]bool{}, map[int]bool{}
	m := SpanMatch{Gold: len(gold), Predicted: len(pred), Pairs: [][2]int{}}
	for _, c := range cands {
		if usedG[c.g] || usedP[c.p] {
			continue
		}
		usedG[c.g], usedP[c.p] = true, true
		m.TruePositives++
		m.Pairs = append(m.Pairs, [2]int{c.g, c.p})
	}
	return m
}

// BrierResult is the Brier score of binary forecasts and its Murphy
// decomposition, binned by distinct forecast value (exact, so
// Reliability - Resolution + Uncertainty == Brier).
type BrierResult struct {
	Valid       bool    `json:"valid"`
	N           int     `json:"n"`
	Brier       float64 `json:"brier"`
	Reliability float64 `json:"reliability"`
	Resolution  float64 `json:"resolution"`
	Uncertainty float64 `json:"uncertainty"`
	BaseRate    float64 `json:"base_rate"`
	// BrierClimatology is the Brier score of always forecasting the base
	// rate (= Uncertainty); Skill is 1 - Brier/BrierClimatology.
	Skill float64 `json:"skill"`
}

// Brier scores forecasts ps against outcomes ys in [0,1] (fractional
// outcomes are allowed: an expected outcome under settlement uncertainty).
func Brier(ps, ys []float64) BrierResult {
	if len(ps) == 0 || len(ps) != len(ys) {
		return BrierResult{}
	}
	n := float64(len(ps))
	var sum, base float64
	for i := range ps {
		p := math.Min(1, math.Max(0, ps[i]))
		sum += (p - ys[i]) * (p - ys[i])
		base += ys[i]
	}
	base /= n
	type bin struct{ n, y float64 }
	bins := map[float64]*bin{}
	for i := range ps {
		p := math.Min(1, math.Max(0, ps[i]))
		b := bins[p]
		if b == nil {
			b = &bin{}
			bins[p] = b
		}
		b.n++
		b.y += ys[i]
	}
	var rel, res float64
	for p, b := range bins {
		o := b.y / b.n
		rel += b.n * (p - o) * (p - o)
		res += b.n * (o - base) * (o - base)
	}
	r := BrierResult{Valid: true, N: len(ps), Brier: sum / n, Reliability: rel / n, Resolution: res / n,
		Uncertainty: base * (1 - base), BaseRate: base}
	// Fractional outcomes add within-bin outcome variance to the Brier
	// score that the binary identity omits; fold it into Uncertainty's
	// complement so the parts still sum (it is zero for binary outcomes).
	if extra := r.Brier - (r.Reliability - r.Resolution + r.Uncertainty); math.Abs(extra) > 1e-12 {
		r.Uncertainty += extra
	}
	if r.Uncertainty > 0 {
		r.Skill = 1 - r.Brier/(base*(1-base))
	}
	return r
}

// NEff is the effective sample size of weights: (Σw)² / Σw².
func NEff(w []float64) float64 {
	var s, s2 float64
	for _, x := range w {
		s += x
		s2 += x * x
	}
	if s2 == 0 {
		return 0
	}
	return s * s / s2
}

// Rate is a weighted rate with an interval.
type Rate struct {
	Valid bool    `json:"valid"`
	N     int     `json:"n"`
	NEff  float64 `json:"n_eff"`
	Mean  float64 `json:"mean"`
	Lo    float64 `json:"lo"`
	Hi    float64 `json:"hi"`
}

// WeightedRate is the weighted mean of outcomes in [0,1] with a central 95%
// interval from Beta(1 + n_eff·m, 1 + n_eff·(1−m)) (ontology §6a).
func WeightedRate(w, ys []float64) Rate {
	if len(w) == 0 || len(w) != len(ys) {
		return Rate{}
	}
	var s, sy float64
	for i := range w {
		s += w[i]
		sy += w[i] * ys[i]
	}
	if s <= 0 {
		return Rate{N: len(w)}
	}
	m := sy / s
	ne := NEff(w)
	iv := BetaInterval(ne*m, ne*(1-m), 0.95)
	return Rate{Valid: true, N: len(w), NEff: ne, Mean: m, Lo: iv[0], Hi: iv[1]}
}

// BetaInterval returns the central interval of Beta(1+k, 1+f) holding mass
// level, where k and f are (pseudo-)counts of successes and failures.
func BetaInterval(k, f, level float64) [2]float64 {
	a, b := 1+k, 1+f
	tail := (1 - level) / 2
	return [2]float64{betaQuantile(tail, a, b), betaQuantile(1-tail, a, b)}
}

func betaQuantile(q, a, b float64) float64 {
	lo, hi := 0.0, 1.0
	for i := 0; i < 100; i++ {
		mid := (lo + hi) / 2
		if regIncBeta(mid, a, b) < q {
			lo = mid
		} else {
			hi = mid
		}
	}
	return (lo + hi) / 2
}

// regIncBeta is the regularized incomplete beta I_x(a,b) by Lentz's
// continued fraction (Numerical Recipes betacf).
func regIncBeta(x, a, b float64) float64 {
	if x <= 0 {
		return 0
	}
	if x >= 1 {
		return 1
	}
	la, _ := math.Lgamma(a + b)
	lb, _ := math.Lgamma(a)
	lc, _ := math.Lgamma(b)
	front := math.Exp(la - lb - lc + a*math.Log(x) + b*math.Log(1-x))
	if x < (a+1)/(a+b+2) {
		return front * betacf(x, a, b) / a
	}
	return 1 - front*betacf(1-x, b, a)/b
}

func betacf(x, a, b float64) float64 {
	const tiny = 1e-300
	qab, qap, qam := a+b, a+1, a-1
	c, d := 1.0, 1-qab*x/qap
	if math.Abs(d) < tiny {
		d = tiny
	}
	d = 1 / d
	h := d
	for m := 1; m <= 300; m++ {
		fm := float64(m)
		m2 := 2 * fm
		aa := fm * (b - fm) * x / ((qam + m2) * (a + m2))
		d = 1 + aa*d
		if math.Abs(d) < tiny {
			d = tiny
		}
		c = 1 + aa/c
		if math.Abs(c) < tiny {
			c = tiny
		}
		d = 1 / d
		h *= d * c
		aa = -(a + fm) * (qab + fm) * x / ((a + m2) * (qap + m2))
		d = 1 + aa*d
		if math.Abs(d) < tiny {
			d = tiny
		}
		c = 1 + aa/c
		if math.Abs(c) < tiny {
			c = tiny
		}
		d = 1 / d
		del := d * c
		h *= del
		if math.Abs(del-1) < 3e-14 {
			break
		}
	}
	return h
}
