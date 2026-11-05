package trust

import (
	"fmt"
	"math"
)

// Performative predictions (plan 10.2, Oesterheld and Conitzer): when an
// agent predicts the result of its own action and the prediction helps choose
// the action, it can look good by choosing what it predicts well. The remedy
// is to randomize a small share of gated choices among the admissible
// alternatives, log the propensity, and score with inverse-propensity weights.
// Alternatives not taken stay unscored unless one is actually run.

// MaxHeldOutShare bounds the randomized share of choices. The 10% held-out
// set of PSA 1.0 is the starting value.
const MaxHeldOutShare = 0.5

// Randomizer randomizes a small share of choices, seeded by a host secret.
type Randomizer struct {
	seed  []byte
	Share float64
}

// NewRandomizer builds a randomizer with the held-out share in (0, 0.5].
func NewRandomizer(seed []byte, share float64) (*Randomizer, error) {
	if len(seed) < MinSeedBytes {
		return nil, fmt.Errorf("trust: randomizer seed needs at least %d bytes", MinSeedBytes)
	}
	if !(share > 0 && share <= MaxHeldOutShare) {
		return nil, fmt.Errorf("trust: held-out share %v is outside (0, %v]", share, MaxHeldOutShare)
	}
	return &Randomizer{seed: append([]byte(nil), seed...), Share: share}, nil
}

// Choice is a logged choice. Propensity is the probability, before the draw,
// that the alternative taken would be taken; Preferred, Share and Alternatives
// let any other alternative's propensity be recomputed.
type Choice struct {
	DecisionID   string  `json:"decision_id"`
	Taken        string  `json:"taken"`
	Preferred    string  `json:"preferred"`
	Alternatives int     `json:"alternatives"`
	Share        float64 `json:"share"`
	Randomized   bool    `json:"randomized"`
	Propensity   float64 `json:"propensity"`
	SeedID       string  `json:"seed_id"`
}

// PropensityOf is the probability that alt would be taken at this decision.
func (c Choice) PropensityOf(alt string) float64 {
	p := c.Share / float64(c.Alternatives)
	if alt == c.Preferred {
		p += 1 - c.Share
	}
	return p
}

// Choose takes the preferred alternative, except with probability Share it
// takes one drawn uniformly from the admissible alternatives (the preferred
// one included). The draw is a pure function of the seed and the decision id.
func (r *Randomizer) Choose(decisionID string, admissible []string, preferred string) Choice {
	c := Choice{DecisionID: decisionID, Taken: preferred, Preferred: preferred, Alternatives: len(admissible), Share: r.Share, SeedID: seedID(r.seed)}
	if len(admissible) > 0 && hostDraw(r.seed, "choice", decisionID) < r.Share {
		c.Randomized = true
		c.Taken = admissible[hostIndex(r.seed, "pick", decisionID, len(admissible))]
	}
	c.Propensity = c.PropensityOf(c.Taken)
	return c
}

// ChoiceOutcome pairs a logged choice with the outcome that followed.
type ChoiceOutcome struct {
	Choice  Choice
	Outcome float64
}

// IPWValue estimates the mean outcome had alt always been taken, by
// inverse-propensity weighting: the mean over all choices of
// 1[taken == alt] * outcome / propensity(alt). ok is false with no choices.
func IPWValue(obs []ChoiceOutcome, alt string) (float64, bool) {
	if len(obs) == 0 {
		return 0, false
	}
	s := 0.0
	for _, o := range obs {
		if o.Choice.Taken == alt {
			if p := o.Choice.PropensityOf(alt); p > 0 {
				s += o.Outcome / p
			}
		}
	}
	return s / float64(len(obs)), true
}

// FlipSampler draws which consequences get the relevance measurement (plan
// 10.1): two decider calls per sampled consequence, so it is sampled, not run
// on every record. The draw is a pure function of the seed, the record and the
// consequence.
type FlipSampler struct {
	seed []byte
	Rate float64
}

// NewFlipSampler builds a sampler; rate is clamped to [0,1].
func NewFlipSampler(seed []byte, rate float64) *FlipSampler {
	return &FlipSampler{seed: append([]byte(nil), seed...), Rate: math.Max(0, math.Min(1, rate))}
}

// Sample reports whether the consequence is measured.
func (f *FlipSampler) Sample(recordID, consequence string) bool {
	return hostDraw(f.seed, "flip", recordID+"\x00"+consequence) < f.Rate
}

// FlipPair is the deciding desk's verdict with the consequence revealed and
// without it, on the same action.
type FlipPair struct {
	With, Without string
}

// RelevanceStat is the decision flip rate of a consequence: the share of
// sampled verdict pairs that differ, with a 95% Wilson interval.
type RelevanceStat struct {
	Value float64 `json:"value"`
	Lo    float64 `json:"lo"`
	Hi    float64 `json:"hi"`
	N     int     `json:"n"`
	Flips int     `json:"flips"`
}

// Relevance computes the flip rate of recorded verdict pairs. It is
// deterministic in the pairs, so a replay of stored verdicts reproduces it.
func Relevance(pairs []FlipPair) RelevanceStat {
	r := RelevanceStat{N: len(pairs)}
	if r.N == 0 {
		return r
	}
	for _, p := range pairs {
		if p.With != p.Without {
			r.Flips++
		}
	}
	n, k := float64(r.N), float64(r.Flips)
	phat := k / n
	const z = 1.959963984540054
	den := 1 + z*z/n
	centre := (phat + z*z/(2*n)) / den
	half := z * math.Sqrt(phat*(1-phat)/n+z*z/(4*n*n)) / den
	r.Value, r.Lo, r.Hi = phat, math.Max(0, centre-half), math.Min(1, centre+half)
	return r
}

// MeasureRelevance calls the decider twice for each of k samples, once with
// the consequence revealed and once without, and returns the flip rate. The
// pairs it saw are what a replay stores.
func MeasureRelevance(k int, decide func(reveal bool, sample int) (string, error)) (RelevanceStat, error) {
	pairs := make([]FlipPair, 0, k)
	for i := 0; i < k; i++ {
		with, err := decide(true, i)
		if err != nil {
			return RelevanceStat{}, err
		}
		without, err := decide(false, i)
		if err != nil {
			return RelevanceStat{}, err
		}
		pairs = append(pairs, FlipPair{with, without})
	}
	return Relevance(pairs), nil
}
