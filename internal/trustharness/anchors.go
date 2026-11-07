package trustharness

import (
	"fmt"
	"hash/fnv"
	"math/rand"
	"sort"

	"github.com/yusefmosiah/go-choir/internal/trust"
)

// P6 and P7 (MA7.10): the pre-registered tests that read the anchored sample
// (docs/evidence/preregistration-2026-11-04.md). Both are seeded
// (BootstrapSeed with the group's name), so the same stored inputs give the
// same bytes.

const (
	anchorDraws     = 1000
	p7MinPool       = 300
	p7MinAgents     = 2
	p7Alpha         = 0.10
	p6MinScorers    = 4
	p6MinAnchored   = 200
	p6MinTypes      = 2
	p6HeadlineNb    = 50
	p7CoverageLo    = 0.85
	p7CoverageHi    = 0.95
	p7SavingCeiling = 2.0
)

func seededRand(parts ...string) *rand.Rand {
	h := fnv.New64a()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return rand.New(rand.NewSource(BootstrapSeed + int64(h.Sum64()>>1)))
}

func b2f(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

// anchorTests returns P6 and P7.
func anchorTests(in Inputs, pastDeadline bool) (p6, p7 TestResult) {
	p6 = TestResult{ID: "P6", Name: "H-B3: spot-checks beat peer agreement", MinN: "4 scorers with 100 anchored answers each, 200 anchored items, 2 question types", Status: "unresolved", Sealed: true, N: map[string]int{}}
	p7 = TestResult{ID: "P7", Name: "H-B4: PPI covers, saves under 2x for strong agents", MinN: "2 agents with a 300-item anchored pool", Status: "unresolved", Sealed: true, N: map[string]int{}}
	if in.AnchorSample == nil {
		reason := "the computer's build did not serve an anchored sample"
		p6.Reason, p7.Reason = reason, reason
		p6.N["anchored_items"], p7.N["anchored_items"] = 0, 0
		return p6, p7
	}
	computeP6(in.AnchorSample, pastDeadline, &p6)
	computeP7(in.AnchorSample, pastDeadline, &p7)
	return p6, p7
}

// ---------------------------------------------------------------- P7

type ppiGroup struct {
	key, agent, typ string
	cheapAll        []float64
	cheapLab, truth []float64
	judgeRight      []float64
}

func computeP7(s *trust.AnchorSample, pastDeadline bool, r *TestResult) {
	byKey := map[string]*ppiGroup{}
	var keys []string
	anchored := 0
	for _, row := range s.Corpus {
		k := row.Agent + "\x00" + row.QuestionType
		g, ok := byKey[k]
		if !ok {
			g = &ppiGroup{key: k, agent: row.Agent, typ: row.QuestionType}
			byKey[k] = g
			keys = append(keys, k)
		}
		g.cheapAll = append(g.cheapAll, b2f(row.AgentAnswer == row.JudgeAnswer))
		if row.Anchored {
			anchored++
			g.cheapLab = append(g.cheapLab, b2f(row.AgentAnswer == row.JudgeAnswer))
			g.truth = append(g.truth, b2f(row.AgentAnswer == row.Truth))
			g.judgeRight = append(g.judgeRight, b2f(row.JudgeAnswer == row.Truth))
		}
	}
	sort.Strings(keys)
	var pools []*ppiGroup
	maxPool := 0
	typesPerAgent := map[string]int{}
	for _, k := range keys {
		g := byKey[k]
		if len(g.truth) > maxPool {
			maxPool = len(g.truth)
		}
		if len(g.truth) >= p7MinPool {
			pools = append(pools, g)
			typesPerAgent[g.agent]++
		}
	}
	agentsWithPool := len(typesPerAgent)
	r.N = map[string]int{"corpus_rows": len(s.Corpus), "anchored_items": anchored, "max_pool": maxPool, "pools": len(pools), "agents_with_pool": agentsWithPool}
	atOrAbove := 0
	capable := map[string]bool{}
	for _, g := range pools {
		if meanOf(g.truth) >= meanOf(g.judgeRight) {
			atOrAbove++
			capable[g.key] = true
		}
	}
	r.N["pools_at_or_above_judge"] = atOrAbove
	below := agentsWithPool < p7MinAgents || atOrAbove < 1
	if below && !pastDeadline {
		r.Reason = fmt.Sprintf("below the minimum n (%d agents with a pool of %d anchored items or more, %d pools at or above the judge; need %d agents and 1 pool); outcome metrics sealed until the minimum n or %s", agentsWithPool, p7MinPool, atOrAbove, p7MinAgents, Deadline.Format("2006-01-02"))
		return
	}
	r.Sealed = false
	if below {
		r.Reason = "the deadline passed below the minimum n"
		return
	}
	r.Metrics = map[string]float64{}
	refuted, supported := false, true
	for _, g := range pools {
		name := g.agent
		if typesPerAgent[g.agent] > 1 {
			name += "/" + g.typ
		}
		poolMean := meanOf(g.truth)
		for _, n := range []int{20, 40, 80} {
			rng := seededRand("p7", g.key, fmt.Sprint(n))
			cover, saving, savingN := 0, 0.0, 0
			for d := 0; d < anchorDraws; d++ {
				idx := rng.Perm(len(g.truth))[:n]
				cl, tr := make([]float64, n), make([]float64, n)
				for i, j := range idx {
					cl[i], tr[i] = g.cheapLab[j], g.truth[j]
				}
				ppi, err := trust.PPIMean(g.cheapAll, cl, tr, p7Alpha)
				if err != nil {
					continue
				}
				cls := trust.ClassicalMean(tr, p7Alpha)
				if ppi.Lo <= poolMean && poolMean <= ppi.Hi {
					cover++
				}
				if wp, wc := ppi.Hi-ppi.Lo, cls.Hi-cls.Lo; wp > 0 && wc > 0 {
					saving += (wc / wp) * (wc / wp)
					savingN++
				}
			}
			cov := float64(cover) / anchorDraws
			sv := 0.0
			if savingN > 0 {
				sv = saving / float64(savingN)
			}
			r.Metrics[fmt.Sprintf("coverage_%s_n%d", name, n)] = r6(cov)
			r.Metrics[fmt.Sprintf("saving_%s_n%d", name, n)] = r6(sv)
			if cov < p7CoverageLo || cov > p7CoverageHi {
				supported = false
			}
			if cov < p7CoverageLo && n >= 40 {
				refuted = true
			}
			if capable[g.key] && sv >= p7SavingCeiling {
				refuted, supported = true, false
			}
		}
		r.Metrics["pool_mean_"+name] = r6(poolMean)
	}
	switch {
	case refuted:
		r.Status, r.Reason = "refuted", "coverage fell below 0.85 at n of 40 or more for an agent, or the saving factor reached 2 for an agent at or above the judge"
	case supported:
		r.Status, r.Reason = "supported", "coverage is within [0.85, 0.95] for every agent and n, and the saving factor is below 2 for every agent at or above the judge"
	default:
		r.Status, r.Reason = "unresolved", "coverage left [0.85, 0.95] without falling below 0.85 at n of 40 or more"
	}
	r.Notes = append(r.Notes, "the anchors are settled by the strongest judge available, not by ground truth; the pool mean is itself an estimate (pre-registration P7, known edges)")
}

func meanOf(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := 0.0
	for _, x := range v {
		s += x
	}
	return s / float64(len(v))
}

// ---------------------------------------------------------------- P6

type scorerObs struct {
	item   int
	answer string
}

func computeP6(s *trust.AnchorSample, pastDeadline bool, r *TestResult) {
	truth := []string{}
	itemAnswers := []map[string]string{}
	types := map[string]bool{}
	kinds := map[trust.AnchorKind]int{}
	byScorer := map[string][]scorerObs{}
	for _, it := range s.Items {
		if it.Truth == "" {
			continue
		}
		idx := len(truth)
		truth = append(truth, it.Truth)
		types[it.QuestionType] = true
		kinds[it.Kind]++
		m := map[string]string{}
		for _, a := range it.Answers {
			m[a.Scorer] = a.Answer
			byScorer[a.Scorer] = append(byScorer[a.Scorer], scorerObs{idx, a.Answer})
		}
		itemAnswers = append(itemAnswers, m)
	}
	var names []string
	for n, obs := range byScorer {
		if len(obs) >= 2*p6HeadlineNb {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	r.N = map[string]int{"anchored_items": len(truth), "scorers": len(names), "question_types": len(types)}
	for k, c := range kinds {
		r.N["anchors_"+string(k)] = c
	}
	below := len(names) < p6MinScorers || len(truth) < p6MinAnchored || len(types) < p6MinTypes
	if below && !pastDeadline {
		r.Reason = fmt.Sprintf("below the minimum n (%d scorers with %d anchored answers or more, %d anchored items, %d question types; need %d, %d and %d); outcome metrics sealed until the minimum n or %s",
			len(names), 2*p6HeadlineNb, len(truth), len(types), p6MinScorers, p6MinAnchored, p6MinTypes, Deadline.Format("2006-01-02"))
		return
	}
	r.Sealed = false
	if below {
		r.Reason = "the deadline passed below the minimum n"
		return
	}
	r.Metrics = map[string]float64{}
	var headDiff []float64
	for _, nb := range []int{25, 50, 100} {
		rng := seededRand("p6", fmt.Sprint(nb))
		var spotMAEs, peerMAEs, diffs []float64
		for d := 0; d < anchorDraws; d++ {
			var spotErr, peerErr float64
			used := 0
			for _, name := range names {
				obs := byScorer[name]
				if len(obs) < 2*nb {
					continue
				}
				perm := rng.Perm(len(obs))
				half := len(obs) / 2
				var accB float64
				for _, p := range perm[half:] {
					accB += b2f(obs[p].answer == truth[obs[p].item])
				}
				accB /= float64(len(obs) - half)
				sub := perm[:nb]
				var spot float64
				agree, usable := 0.0, 0
				for _, p := range sub {
					o := obs[p]
					spot += b2f(o.answer == truth[o.item])
					if maj, ok := majorityOfOthers(itemAnswers[o.item], name); ok {
						agree += b2f(o.answer == maj)
						usable++
					}
				}
				if usable == 0 {
					continue
				}
				spotErr += absF(spot/float64(nb) - accB)
				peerErr += absF(agree/float64(usable) - accB)
				used++
			}
			if used == 0 {
				continue
			}
			spotMAEs = append(spotMAEs, spotErr/float64(used))
			peerMAEs = append(peerMAEs, peerErr/float64(used))
			diffs = append(diffs, (peerErr-spotErr)/float64(used))
		}
		if len(diffs) == 0 {
			continue
		}
		key := fmt.Sprintf("nb%d", nb)
		sd := append([]float64(nil), diffs...)
		sort.Float64s(sd)
		r.Metrics["spot_mae_"+key] = r6(meanOf(spotMAEs))
		r.Metrics["peer_mae_"+key] = r6(meanOf(peerMAEs))
		r.Metrics["difference_mean_"+key] = r6(meanOf(diffs))
		r.Metrics["difference_lo95_"+key] = r6(quantile(sd, 0.025))
		r.Metrics["difference_hi95_"+key] = r6(quantile(sd, 0.975))
		if nb == p6HeadlineNb {
			headDiff = sd
		}
	}
	switch {
	case len(headDiff) == 0:
		r.Status, r.Reason = "unresolved", "no scorer had enough anchored answers at n_b of 50"
	case quantile(headDiff, 0.025) > 0:
		r.Status, r.Reason = "supported", "the spot-check estimate's error is lower than the peer estimate's at n_b of 50 and the 95% interval of the difference excludes 0"
	case meanOf(headDiff) <= 0:
		r.Status, r.Reason = "refuted", "the peer estimate's error is equal or lower at n_b of 50"
	default:
		r.Status, r.Reason = "unresolved", "the spot-check estimate's error is lower at n_b of 50 but the 95% interval of the difference includes 0"
	}
	r.Notes = append(r.Notes,
		"the peer estimate is agreement with the majority of the other scorers on the same items; the Dawid-Skene estimate and the Kendall tau of the orderings are not computed",
		"the anchors are settled by the strongest judge available, not by ground truth; N lists the anchors by kind (pre-registration P6, known edges)")
}

func majorityOfOthers(answers map[string]string, self string) (string, bool) {
	tally := map[string]int{}
	for s, a := range answers {
		if s != self {
			tally[a]++
		}
	}
	best, bn, tie := "", 0, false
	var as []string
	for a := range tally {
		as = append(as, a)
	}
	sort.Strings(as)
	for _, a := range as {
		switch {
		case tally[a] > bn:
			best, bn, tie = a, tally[a], false
		case tally[a] == bn:
			tie = true
		}
	}
	return best, bn > 0 && !tie
}

func absF(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
