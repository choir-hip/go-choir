package trustharness

import (
	"math"
	"math/rand"
	"sort"
)

// r6 rounds to six decimals so a report's bytes do not depend on the last
// bits of a platform's floating point (fused multiply-add differs by arch).
func r6(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return math.Round(v*1e6) / 1e6
}

// scored is one unit's risk score under a predictor and its label.
type scored struct {
	subject string
	risk    float64
	pos     bool
}

// aucOf is the Mann-Whitney AUC of risk for the positives against the
// negatives, ties counting half. ok is false when a side is empty.
func aucOf(xs []scored) (float64, bool) {
	var pos, neg []float64
	for _, x := range xs {
		if x.pos {
			pos = append(pos, x.risk)
		} else {
			neg = append(neg, x.risk)
		}
	}
	if len(pos) == 0 || len(neg) == 0 {
		return 0.5, false
	}
	wins := 0.0
	for _, a := range pos {
		for _, b := range neg {
			switch {
			case a > b:
				wins++
			case a == b:
				wins += 0.5
			}
		}
	}
	return wins / float64(len(pos)*len(neg)), true
}

// degenerate says the scores are constant over the units (AUC 0.5 by
// construction; pre-registration P1).
func degenerate(xs []scored) bool {
	if len(xs) < 2 {
		return true
	}
	for _, x := range xs[1:] {
		if x.risk != xs[0].risk {
			return false
		}
	}
	return true
}

// bootstrapDiff is the 95% interval of AUC(a) - AUC(b) over the same units,
// resampling subjects (pre-registration section 1). a and b are aligned.
func bootstrapDiff(a, b []scored) (lo, hi float64, ok bool) {
	subj := map[string][]int{}
	for i, x := range a {
		subj[x.subject] = append(subj[x.subject], i)
	}
	names := make([]string, 0, len(subj))
	for s := range subj {
		names = append(names, s)
	}
	sort.Strings(names)
	rng := rand.New(rand.NewSource(BootstrapSeed))
	var diffs []float64
	for r := 0; r < BootstrapResamples; r++ {
		var ra, rb []scored
		for range names {
			s := names[rng.Intn(len(names))]
			for _, i := range subj[s] {
				ra, rb = append(ra, a[i]), append(rb, b[i])
			}
		}
		x, okA := aucOf(ra)
		y, okB := aucOf(rb)
		if okA && okB {
			diffs = append(diffs, x-y)
		}
	}
	if len(diffs) < BootstrapResamples/2 {
		return 0, 0, false
	}
	sort.Float64s(diffs)
	return quantile(diffs, 0.025), quantile(diffs, 0.975), true
}

// quantile of sorted xs by linear interpolation.
func quantile(xs []float64, q float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	pos := q * float64(len(xs)-1)
	i := int(math.Floor(pos))
	if i+1 >= len(xs) {
		return xs[len(xs)-1]
	}
	return xs[i] + (pos-float64(i))*(xs[i+1]-xs[i])
}

func median(xs []float64) float64 {
	s := append([]float64(nil), xs...)
	sort.Float64s(s)
	return quantile(s, 0.5)
}

func p90(xs []float64) float64 {
	s := append([]float64(nil), xs...)
	sort.Float64s(s)
	return quantile(s, 0.9)
}

func clip(p float64) float64 { return math.Min(0.99, math.Max(0.01, p)) }

// logLoss is the mean negative log likelihood of the failure probabilities.
func logLoss(pFail []float64, pos []bool) float64 {
	s := 0.0
	for i, p := range pFail {
		p = clip(p)
		if pos[i] {
			s -= math.Log(p)
		} else {
			s -= math.Log(1 - p)
		}
	}
	return s / float64(len(pFail))
}

func pearson(x, y []float64) (float64, bool) {
	n := float64(len(x))
	if n < 2 {
		return 0, false
	}
	var mx, my float64
	for i := range x {
		mx += x[i]
		my += y[i]
	}
	mx, my = mx/n, my/n
	var sxy, sxx, syy float64
	for i := range x {
		sxy += (x[i] - mx) * (y[i] - my)
		sxx += (x[i] - mx) * (x[i] - mx)
		syy += (y[i] - my) * (y[i] - my)
	}
	if sxx == 0 || syy == 0 {
		return 0, false
	}
	return sxy / math.Sqrt(sxx*syy), true
}
