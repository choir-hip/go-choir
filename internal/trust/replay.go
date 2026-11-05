package trust

import (
	"math"
	"sort"
)

// The replay of settled records (trust draft 4.1, research H-C1): which of the
// two weight candidates, tempered or capped, better predicts a desk's next
// failure, scored by calibration (Brier and expected calibration error of the
// posterior mean against the next outcome) and by AUC of the lower credible
// bound ordering against the next failure. The replay picks one; it never
// leaves the choice to taste.

// MinReplayPairs is the least number of (prediction, next outcome) pairs the
// replay needs before it will pick by score; below it the capped default
// stands and the report says so.
const MinReplayPairs = 20

// Candidate is one weighting configuration and how it did.
type Candidate struct {
	Mode WeightMode `json:"mode"`
	Eta  float64    `json:"eta,omitempty"`
	Cap  float64    `json:"cap,omitempty"`
	// N is the number of pairs scored.
	N     int     `json:"n"`
	Brier float64 `json:"brier"`
	ECE   float64 `json:"ece"`
	// AUC is the probability that a failing next outcome had a lower bound
	// than a non-failing one (1 is perfect, 0.5 chance). Defined says both
	// outcomes occurred.
	AUC        float64 `json:"auc"`
	AUCDefined bool    `json:"auc_defined"`
}

// ReplayReport is the replay's result: the best configuration of each mode
// (the two candidates), the whole grid, and the one chosen.
type ReplayReport struct {
	Candidates []Candidate `json:"candidates"`
	Grid       []Candidate `json:"grid"`
	Chosen     Candidate   `json:"chosen"`
	// Insufficient says there were too few pairs to choose by score.
	Insufficient bool `json:"insufficient"`
	Pairs        int  `json:"pairs"`
}

var (
	replayEtas = []float64{0.25, 0.5, 1}
	replayCaps = []float64{2, 3, 5}
)

// Replay runs the candidates over in. The records are walked in time; at each
// item the posterior of its subject and class from the earlier items predicts
// whether this item is a failure.
func Replay(in Inputs, base Params) ReplayReport {
	if base.Lambda == 0 {
		d := DefaultParams()
		d.Now = base.Now
		base = d
	}
	ex := Extract(in, base)
	var rep ReplayReport
	for _, eta := range replayEtas {
		p := base
		p.Mode, p.Eta = ModeTempered, eta
		c := evalCandidate(ex.Items, in, p)
		c.Eta = eta
		rep.Grid = append(rep.Grid, c)
	}
	for _, k := range replayCaps {
		p := base
		p.Mode, p.Cap = ModeCapped, k
		c := evalCandidate(ex.Items, in, p)
		c.Cap = k
		rep.Grid = append(rep.Grid, c)
	}
	for _, mode := range []WeightMode{ModeCapped, ModeTempered} {
		var best *Candidate
		for i := range rep.Grid {
			c := rep.Grid[i]
			if c.Mode != mode {
				continue
			}
			if best == nil || better(c, *best) {
				cc := c
				best = &cc
			}
		}
		if best != nil {
			rep.Candidates = append(rep.Candidates, *best)
		}
	}
	rep.Pairs = rep.Candidates[0].N
	rep.Chosen = rep.Candidates[0]
	if rep.Pairs < MinReplayPairs {
		rep.Insufficient = true
		return rep
	}
	for _, c := range rep.Candidates[1:] {
		if better(c, rep.Chosen) {
			rep.Chosen = c
		}
	}
	return rep
}

// better orders candidates: lower Brier, then higher AUC; a tie keeps the
// earlier (capped) one.
func better(a, b Candidate) bool {
	const eps = 1e-12
	if math.Abs(a.Brier-b.Brier) > eps {
		return a.Brier < b.Brier
	}
	return a.AUC > b.AUC+eps
}

type pair struct {
	mean, lcb float64
	fail      bool
}

func evalCandidate(items []Item, in Inputs, p Params) Candidate {
	type gk struct {
		desk string
		c    ActionClass
	}
	by := map[gk][]Item{}
	for _, it := range items {
		if it.Desk == "" || it.Class == E7 {
			continue
		}
		k := gk{it.Desk, it.Action}
		by[k] = append(by[k], it)
	}
	keys := make([]gk, 0, len(by))
	for k := range by {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].desk != keys[j].desk {
			return keys[i].desk < keys[j].desk
		}
		if keys[i].c.Grant != keys[j].c.Grant {
			return keys[i].c.Grant < keys[j].c.Grant
		}
		return keys[i].c.Impact < keys[j].c.Impact
	})
	var pairs []pair
	for _, k := range keys {
		seq := by[k]
		for j := 1; j < len(seq); j++ {
			pp := p
			pp.Now = seq[j].At
			// strictly earlier evidence only
			prior := make([]Item, 0, j)
			for _, e := range seq[:j] {
				if e.At.Before(seq[j].At) {
					prior = append(prior, e)
				}
			}
			if len(prior) == 0 {
				continue
			}
			rows := Fold(prior, in, pp)
			for _, r := range rows {
				if r.Subject.Kind == SubjectAgent && r.Subject.ID == k.desk && r.Class == k.c {
					pairs = append(pairs, pair{mean: r.Mean, lcb: r.LCB, fail: seq[j].Failure()})
				}
			}
		}
	}
	c := Candidate{Mode: p.Mode, N: len(pairs)}
	if len(pairs) == 0 {
		c.AUC = 0.5
		return c
	}
	for _, pr := range pairs {
		y := 1.0
		if pr.fail {
			y = 0
		}
		c.Brier += (pr.mean - y) * (pr.mean - y)
	}
	c.Brier /= float64(len(pairs))
	c.ECE = ece(pairs)
	c.AUC, c.AUCDefined = auc(pairs)
	return c
}

func ece(pairs []pair) float64 {
	const bins = 5
	var cnt, sumP, sumY [bins]float64
	for _, p := range pairs {
		b := int(p.mean * bins)
		if b >= bins {
			b = bins - 1
		}
		cnt[b]++
		sumP[b] += p.mean
		if !p.fail {
			sumY[b]++
		}
	}
	e := 0.0
	for b := 0; b < bins; b++ {
		if cnt[b] > 0 {
			e += cnt[b] / float64(len(pairs)) * math.Abs(sumP[b]/cnt[b]-sumY[b]/cnt[b])
		}
	}
	return e
}

// auc is the Mann-Whitney AUC of a failure score (the lower bound, negated)
// for the failing outcomes against the rest, ties counting half.
func auc(pairs []pair) (float64, bool) {
	var pos, neg int
	for _, p := range pairs {
		if p.fail {
			pos++
		} else {
			neg++
		}
	}
	if pos == 0 || neg == 0 {
		return 0.5, false
	}
	wins := 0.0
	for _, a := range pairs {
		if !a.fail {
			continue
		}
		for _, b := range pairs {
			if b.fail {
				continue
			}
			switch {
			case a.lcb < b.lcb:
				wins++
			case a.lcb == b.lcb:
				wins += 0.5
			}
		}
	}
	return wins / float64(pos*neg), true
}
