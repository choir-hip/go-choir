package calibration

import (
	"math"
	"sort"
	"time"
)

// CausalShrinkWeight is the weight, in forecasts, of the pooled rate against a
// kind's own mean outcome in the causal base rate (design 12.7).
const CausalShrinkWeight = 10.0

// The causal base rate (design 5): for each resolved forecast, the rate a
// forecaster who knew only the past would have said. It is the pooled mean
// outcome of every forecast settled strictly before this one was committed,
// with each kind (the claim's project) shrunk toward that pooled rate:
//
//	pooled = mean outcome of all forecasts settled before          (0.5 if none)
//	base   = (kindSum + w*pooled) / (kindN + w),   w = CausalShrinkWeight
//
// where kindN and kindSum count the kind's own forecasts settled before. A
// DJ that merely restates the running base rate scores the base rate's Brier;
// ARL-C1 reads whether p_heard separates better than it. The in-sample mean
// that GroupStats.BaseRate holds could not be known at the time.
//
// The pool is every resolved forecast of the summary, not the group: pooling
// is what stops a rare kind from being compared with a fixed 0.5. Brier of
// both is per forecast, so the paired difference (the Brier of p minus the
// Brier of the base rate; negative means p did better) and its standard error
// are over forecasts.

type causalPoint struct {
	committedAt time.Time
	settledAt   time.Time
	kind        string
	p, y        float64
}

func causalPoints(items []scored) []causalPoint {
	var out []causalPoint
	for _, it := range items {
		if it.status != StatusSettled || !it.resolved || it.settledAt == "" {
			continue
		}
		c, err1 := time.Parse(time.RFC3339Nano, it.claim.RecordedAt)
		s, err2 := time.Parse(time.RFC3339Nano, it.settledAt)
		if err1 != nil || err2 != nil {
			continue
		}
		out = append(out, causalPoint{committedAt: c, settledAt: s, kind: it.claim.Project, p: it.claim.P, y: it.y})
	}
	return out
}

// causalPool is the pooled forecasts ordered by settlement time.
func causalPool(pts []causalPoint) []causalPoint {
	pool := append([]causalPoint(nil), pts...)
	sort.SliceStable(pool, func(i, j int) bool { return pool[i].settledAt.Before(pool[j].settledAt) })
	return pool
}

// applyCausal fills the causal fields of g for the forecasts eval, against
// the pool. One sweep in commit order keeps it linear in their count.
func applyCausal(g *GroupStats, pool []causalPoint, eval []causalPoint) {
	e := append([]causalPoint(nil), eval...)
	sort.SliceStable(e, func(i, j int) bool { return e[i].committedAt.Before(e[j].committedAt) })
	type tally struct {
		n   float64
		sum float64
	}
	kinds := map[string]*tally{}
	var poolN, poolSum float64
	next := 0
	var baseSum, diffSum float64
	diffs := make([]float64, 0, len(e))
	for _, pt := range e {
		for next < len(pool) && pool[next].settledAt.Before(pt.committedAt) {
			k := kinds[pool[next].kind]
			if k == nil {
				k = &tally{}
				kinds[pool[next].kind] = k
			}
			k.n++
			k.sum += pool[next].y
			poolN++
			poolSum += pool[next].y
			next++
		}
		pooled := 0.5
		if poolN > 0 {
			pooled = poolSum / poolN
		}
		base := pooled
		if k := kinds[pt.kind]; k != nil {
			base = (k.sum + CausalShrinkWeight*pooled) / (k.n + CausalShrinkWeight)
		}
		bBase, bP := BrierScore(base, pt.y), BrierScore(pt.p, pt.y)
		baseSum += bBase
		diffSum += bP - bBase
		diffs = append(diffs, bP-bBase)
	}
	g.CausalN = len(e)
	if len(e) == 0 {
		return
	}
	n := float64(len(e))
	g.CausalBaseRateBrier = ptr(baseSum / n)
	mean := diffSum / n
	g.CausalBrierDiff = ptr(mean)
	if len(e) >= 2 {
		var ss float64
		for _, d := range diffs {
			ss += (d - mean) * (d - mean)
		}
		g.CausalBrierDiffSE = ptr(math.Sqrt(ss/(n-1)) / math.Sqrt(n))
	}
}
