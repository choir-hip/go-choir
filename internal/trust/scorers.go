package trust

import (
	"math"
	"sort"
)

// Scorer profiles (plan 10.2): a scorer's quality is a profile read against
// anchors, never a number and never its agreement with other scorers.

// MinProfileN is the fewest anchored items a question type needs before a
// scorer's reliability on it is reported as known.
const MinProfileN = 5

// Planted item kinds: meaningless or injected answers that a sound scorer
// rejects.
const (
	PlantedEmpty    = "empty"
	PlantedInjected = "injected"
)

// ScorerObservation is one scorer's judgment of one item. The binary yes/no
// judgments feed the bias metrics ("yes" is acceptance).
type ScorerObservation struct {
	Scorer       string
	Family       string
	QuestionType string
	ItemID       string
	Answers      []string
	Probs        map[string]float64
	// Position is where the candidate was shown in a comparison.
	Position int
	// Length is the candidate's length.
	Length          int
	CandidateFamily string
	// Run is the repetition index of the same judgment (test-retest).
	Run int
	// Planted marks an item planted to catch a scorer that accepts anything.
	Planted string
}

// topAnswer is the answer a distribution ranks first (ties alphabetical).
func topAnswer(p map[string]float64) string {
	best, bp := "", -1.0
	keys := make([]string, 0, len(p))
	for k := range p {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if p[k] > bp {
			best, bp = k, p[k]
		}
	}
	return best
}

// TypeProfile is a scorer's reliability on one question type.
type TypeProfile struct {
	QuestionType string  `json:"question_type"`
	N            int     `json:"n"`
	Known        bool    `json:"known"`
	Accuracy     float64 `json:"accuracy"`
	Brier        float64 `json:"brier"`
	ECE          float64 `json:"ece"`
	// AUC is the discrimination of P(yes) against the anchored truth.
	AUC float64 `json:"auc"`
}

// Metric is one measured quantity with its sample size.
type Metric struct {
	Value float64 `json:"value"`
	N     int     `json:"n"`
}

// ScorerProfile is a scorer against anchors: discrimination and calibration
// per question type, a bias profile (position, length, own family),
// test-retest consistency and exploitability.
type ScorerProfile struct {
	Scorer string        `json:"scorer"`
	Family string        `json:"family,omitempty"`
	Types  []TypeProfile `json:"types"`
	// PositionBias: how much more the scorer accepts what is shown first,
	// beyond what the anchors say. LengthBias: correlation of acceptance with
	// length beyond the anchors'. FamilyFavor: how much more it accepts its
	// own family's text beyond the anchors'.
	PositionBias Metric `json:"position_bias"`
	LengthBias   Metric `json:"length_bias"`
	FamilyFavor  Metric `json:"family_favor"`
	// TestRetest is the share of repeated judgments with the same top answer.
	TestRetest Metric `json:"test_retest"`
	// Exploitability is the share of planted meaningless or injected items
	// the scorer accepted.
	Exploitability Metric `json:"exploitability"`
}

// Type returns the profile for a question type; with no anchored items it is
// unknown (not extrapolated from other types).
func (p ScorerProfile) Type(q string) TypeProfile {
	for _, t := range p.Types {
		if t.QuestionType == q {
			return t
		}
	}
	return TypeProfile{QuestionType: q}
}

// ScorerProfiles grades every scorer in obs against the anchors. Only the
// anchors are read for truth: a scorer that agrees with its peers and not
// with the anchors is a poor scorer.
func ScorerProfiles(obs []ScorerObservation, anchors map[string]Anchor) []ScorerProfile {
	byScorer := map[string][]ScorerObservation{}
	fam := map[string]string{}
	for _, o := range obs {
		byScorer[o.Scorer] = append(byScorer[o.Scorer], o)
		if o.Family != "" {
			fam[o.Scorer] = o.Family
		}
	}
	names := make([]string, 0, len(byScorer))
	for n := range byScorer {
		names = append(names, n)
	}
	sort.Strings(names)
	var out []ScorerProfile
	for _, name := range names {
		out = append(out, profileOf(name, fam[name], byScorer[name], anchors))
	}
	return out
}

func profileOf(name, family string, obs []ScorerObservation, anchors map[string]Anchor) ScorerProfile {
	p := ScorerProfile{Scorer: name, Family: family}
	// graded observations: first run, not planted, with an anchor
	var graded []ScorerObservation
	byItem := map[string][]ScorerObservation{}
	for _, o := range obs {
		if o.Planted == "" {
			byItem[o.ItemID] = append(byItem[o.ItemID], o)
		}
		if o.Planted == "" && o.Run == 0 {
			if _, ok := anchors[o.ItemID]; ok {
				graded = append(graded, o)
			}
		}
	}
	types := map[string][]ScorerObservation{}
	for _, o := range graded {
		types[o.QuestionType] = append(types[o.QuestionType], o)
	}
	qs := make([]string, 0, len(types))
	for q := range types {
		qs = append(qs, q)
	}
	sort.Strings(qs)
	for _, q := range qs {
		p.Types = append(p.Types, typeProfile(q, types[q], anchors))
	}

	// bias metrics over binary judgments
	type row struct {
		accept, truth float64
		pos, length   int
		candFam       string
	}
	var rows []row
	for _, o := range graded {
		a, ok := anchors[o.ItemID]
		if !ok || !hasAnswer(o.Answers, "yes") {
			continue
		}
		t := 0.0
		if a.Truth == "yes" {
			t = 1
		}
		rows = append(rows, row{o.Probs["yes"], t, o.Position, o.Length, o.CandidateFamily})
	}
	{
		var a0, a1, t0, t1 []float64
		for _, r := range rows {
			if r.pos == 0 {
				a0, t0 = append(a0, r.accept), append(t0, r.truth)
			} else {
				a1, t1 = append(a1, r.accept), append(t1, r.truth)
			}
		}
		if len(a0) >= 3 && len(a1) >= 3 {
			p.PositionBias = Metric{(meanF(a0) - meanF(a1)) - (meanF(t0) - meanF(t1)), len(a0) + len(a1)}
		}
	}
	{
		var ls, as, ts []float64
		for _, r := range rows {
			if r.length > 0 {
				ls, as, ts = append(ls, float64(r.length)), append(as, r.accept), append(ts, r.truth)
			}
		}
		if len(ls) >= MinProfileN {
			p.LengthBias = Metric{pearson(ls, as) - pearson(ls, ts), len(ls)}
		}
	}
	if family != "" {
		var ownA, ownT, otherA, otherT []float64
		for _, r := range rows {
			switch {
			case r.candFam == "":
			case r.candFam == family:
				ownA, ownT = append(ownA, r.accept), append(ownT, r.truth)
			default:
				otherA, otherT = append(otherA, r.accept), append(otherT, r.truth)
			}
		}
		if len(ownA) >= 3 && len(otherA) >= 3 {
			p.FamilyFavor = Metric{(meanF(ownA) - meanF(otherA)) - (meanF(ownT) - meanF(otherT)), len(ownA) + len(otherA)}
		}
	}
	// test-retest: run 0 against each later run of the same item
	agree, pairs := 0, 0
	for _, items := range byItem {
		var first *ScorerObservation
		for i := range items {
			if items[i].Run == 0 {
				first = &items[i]
			}
		}
		if first == nil {
			continue
		}
		for i := range items {
			if items[i].Run > 0 {
				pairs++
				if topAnswer(items[i].Probs) == topAnswer(first.Probs) {
					agree++
				}
			}
		}
	}
	if pairs > 0 {
		p.TestRetest = Metric{float64(agree) / float64(pairs), pairs}
	}
	// exploitability: planted meaningless or injected items accepted
	acc, planted := 0, 0
	for _, o := range obs {
		if o.Planted == "" {
			continue
		}
		planted++
		if topAnswer(o.Probs) == "yes" {
			acc++
		}
	}
	if planted > 0 {
		p.Exploitability = Metric{float64(acc) / float64(planted), planted}
	}
	return p
}

func typeProfile(q string, obs []ScorerObservation, anchors map[string]Anchor) TypeProfile {
	t := TypeProfile{QuestionType: q, N: len(obs), Known: len(obs) >= MinProfileN}
	if len(obs) == 0 {
		return t
	}
	var pairs []pair
	correct := 0.0
	for _, o := range obs {
		truth := anchors[o.ItemID].Truth
		top := topAnswer(o.Probs)
		if top == truth {
			correct++
		}
		s := 0.0
		for _, a := range o.Answers {
			y := 0.0
			if a == truth {
				y = 1
			}
			d := o.Probs[a] - y
			s += d * d
		}
		t.Brier += s / 2
		// pair for ECE (confidence vs correctness) and AUC (P(yes) vs truth yes)
		conf := o.Probs[top]
		pairs = append(pairs, pair{mean: conf, fail: top != truth})
	}
	n := float64(len(obs))
	t.Accuracy, t.Brier = correct/n, t.Brier/n
	t.ECE = ece(pairs)
	t.AUC = yesAUC(obs, anchors)
	return t
}

// yesAUC is the discrimination of P(yes) for items whose anchored truth is yes
// against those whose truth is no (0.5 when either is missing).
func yesAUC(obs []ScorerObservation, anchors map[string]Anchor) float64 {
	var pos, neg []float64
	for _, o := range obs {
		if !hasAnswer(o.Answers, "yes") {
			continue
		}
		if anchors[o.ItemID].Truth == "yes" {
			pos = append(pos, o.Probs["yes"])
		} else {
			neg = append(neg, o.Probs["yes"])
		}
	}
	if len(pos) == 0 || len(neg) == 0 {
		return 0.5
	}
	w := 0.0
	for _, a := range pos {
		for _, b := range neg {
			switch {
			case a > b:
				w++
			case a == b:
				w += 0.5
			}
		}
	}
	return w / float64(len(pos)*len(neg))
}

func hasAnswer(answers []string, a string) bool {
	for _, x := range answers {
		if x == a {
			return true
		}
	}
	return false
}

func meanF(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := 0.0
	for _, x := range v {
		s += x
	}
	return s / float64(len(v))
}

func pearson(x, y []float64) float64 {
	mx, my := meanF(x), meanF(y)
	var sxy, sxx, syy float64
	for i := range x {
		sxy += (x[i] - mx) * (y[i] - my)
		sxx += (x[i] - mx) * (x[i] - mx)
		syy += (y[i] - my) * (y[i] - my)
	}
	if sxx == 0 || syy == 0 {
		return 0
	}
	return sxy / math.Sqrt(sxx*syy)
}

// MinPairItems is the fewest items two scorers must share before their error
// correlation is defined.
const MinPairItems = 10

// PairCorr is the correlation of two scorers' errors on the anchor sample.
type PairCorr struct {
	A          string  `json:"a"`
	B          string  `json:"b"`
	N          int     `json:"n"`
	Rho        float64 `json:"rho"`
	Defined    bool    `json:"defined"`
	SameFamily bool    `json:"same_family"`
}

// ErrorCorrelation computes, for every pair of scorers, the correlation of
// their error indicators (top answer against the anchored truth) over the items
// both scored. Independence is measured on anchored truth, never assumed:
// agreement between scorers whose errors correlate is not independent
// evidence, and a same-family pair is flagged.
func ErrorCorrelation(obs []ScorerObservation, anchors map[string]Anchor) []PairCorr {
	errs := map[string]map[string]float64{}
	fam := map[string]string{}
	for _, o := range obs {
		if o.Run != 0 || o.Planted != "" {
			continue
		}
		a, ok := anchors[o.ItemID]
		if !ok {
			continue
		}
		if errs[o.Scorer] == nil {
			errs[o.Scorer] = map[string]float64{}
		}
		e := 0.0
		if topAnswer(o.Probs) != a.Truth {
			e = 1
		}
		errs[o.Scorer][o.ItemID] = e
		if o.Family != "" {
			fam[o.Scorer] = o.Family
		}
	}
	names := make([]string, 0, len(errs))
	for n := range errs {
		names = append(names, n)
	}
	sort.Strings(names)
	var out []PairCorr
	for i := range names {
		for j := i + 1; j < len(names); j++ {
			a, b := names[i], names[j]
			var xs, ys []float64
			items := make([]string, 0, len(errs[a]))
			for it := range errs[a] {
				if _, ok := errs[b][it]; ok {
					items = append(items, it)
				}
			}
			sort.Strings(items)
			for _, it := range items {
				xs, ys = append(xs, errs[a][it]), append(ys, errs[b][it])
			}
			c := PairCorr{A: a, B: b, N: len(xs), SameFamily: fam[a] != "" && fam[a] == fam[b]}
			if len(xs) >= MinPairItems && variance(xs) > 0 && variance(ys) > 0 {
				c.Rho, c.Defined = pearson(xs, ys), true
			}
			out = append(out, c)
		}
	}
	return out
}

func variance(v []float64) float64 {
	m := meanF(v)
	s := 0.0
	for _, x := range v {
		s += (x - m) * (x - m)
	}
	return s
}

// IndependentWeight is the weight a scorer's evidence keeps once the
// correlation of its errors with the other scorers' is counted:
// 1 / (1 + sum of the positive correlations), so a copy of another scorer
// splits one opinion between two. A scorer with no measured pair keeps 1.
func IndependentWeight(scorer string, corr []PairCorr) float64 {
	sum := 0.0
	for _, c := range corr {
		if c.Defined && (c.A == scorer || c.B == scorer) && c.Rho > 0 {
			sum += c.Rho
		}
	}
	return 1 / (1 + sum)
}
