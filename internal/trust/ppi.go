package trust

import (
	"fmt"
	"math"
)

// Prediction-powered inference for an agent's true score (plan 10.2,
// Angelopoulos et al.): cheap System 1 scores run on everything; the anchored
// sample is small. The cheap labels cut variance and the anchored sample
// corrects their bias, giving an estimate with an honest interval.

// Interval is an estimate with a two-sided interval.
type Interval struct {
	Estimate float64 `json:"estimate"`
	Lo       float64 `json:"lo"`
	Hi       float64 `json:"hi"`
	// N is the corpus size scored cheaply and NLabeled the anchored sample.
	N        int `json:"n"`
	NLabeled int `json:"n_labeled"`
}

// zTwoSided is the standard normal quantile for a two-sided level 1-alpha
// (Acklam's rational approximation refined by one Newton step on erfc).
func zTwoSided(alpha float64) float64 {
	p := 1 - alpha/2
	// bisection on the normal CDF: exact enough and dependency free
	lo, hi := 0.0, 10.0
	for i := 0; i < 80; i++ {
		mid := (lo + hi) / 2
		if 0.5*math.Erfc(-mid/math.Sqrt2) < p {
			lo = mid
		} else {
			hi = mid
		}
	}
	return (lo + hi) / 2
}

// PPIMean estimates the mean true score over a corpus. cheapAll are the cheap
// scores of every item (including the labeled ones); cheapLabeled and
// truthLabeled are the cheap score and the anchored truth of the anchored
// sample. The estimate is mean(cheapAll) + mean(truth - cheap) over the
// sample, with variance var(cheapAll)/N + var(truth - cheap)/n.
func PPIMean(cheapAll, cheapLabeled, truthLabeled []float64, alpha float64) (Interval, error) {
	n, bigN := len(truthLabeled), len(cheapAll)
	if n != len(cheapLabeled) {
		return Interval{}, fmt.Errorf("trust: %d cheap scores for %d anchored truths", len(cheapLabeled), n)
	}
	if n < 2 || bigN < 2 {
		return Interval{}, fmt.Errorf("trust: PPI needs at least two anchored and two cheap scores")
	}
	if !(alpha > 0 && alpha < 1) {
		return Interval{}, fmt.Errorf("trust: alpha %v is outside (0,1)", alpha)
	}
	rect := make([]float64, n)
	for i := range rect {
		rect[i] = truthLabeled[i] - cheapLabeled[i]
	}
	est := meanF(cheapAll) + meanF(rect)
	v := variance(cheapAll)/float64(bigN-1)/float64(bigN) + variance(rect)/float64(n-1)/float64(n)
	h := zTwoSided(alpha) * math.Sqrt(v)
	return Interval{Estimate: est, Lo: est - h, Hi: est + h, N: bigN, NLabeled: n}, nil
}

// ClassicalMean is the anchored sample's mean with a normal interval, the
// baseline the cheap labels beat.
func ClassicalMean(truthLabeled []float64, alpha float64) Interval {
	n := len(truthLabeled)
	if n < 2 {
		return Interval{NLabeled: n}
	}
	m := meanF(truthLabeled)
	h := zTwoSided(alpha) * math.Sqrt(variance(truthLabeled)/float64(n-1)/float64(n))
	return Interval{Estimate: m, Lo: m - h, Hi: m + h, NLabeled: n}
}
