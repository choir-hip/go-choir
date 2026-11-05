package trust

import (
	"math"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// Extraction is the first half of the view: records and logs in, evidence
// items out. It reads each record's own frozen contents and its settlement,
// and nothing that happens later: a settled item never depends on a later
// outcome (the importance-judgment and materialized sets only add
// foreseeable-omission items, scored by their own frozen weights).

// Extracted is the result of extraction.
type Extracted struct {
	Items     []Item
	Excluded  []Excluded
	Surprises []Surprise
	Judgment  map[Subject]JudgmentStat
}

func (in Inputs) info(desk string) SubjectInfo {
	if in.Resolver == nil || desk == "" {
		return SubjectInfo{}
	}
	return in.Resolver(desk)
}

// Extract derives the evidence items of in, in a deterministic order.
func Extract(in Inputs, p Params) Extracted {
	recs := append([]types.CommitmentRecord(nil), in.Records...)
	sort.SliceStable(recs, func(i, j int) bool { return recs[i].RecordID < recs[j].RecordID })

	settlements := map[string][]types.CommitmentRecord{}
	resolves := map[string][]types.CommitmentRecord{}
	for _, r := range recs {
		if b, ok := r.SubCallBody(types.SubCallSettlement); ok {
			settlements[b.CastID] = append(settlements[b.CastID], r)
		}
		if r.Resolve != nil {
			parent := r.ParentID
			if parent == "" && len(r.RelatedIDs) > 0 {
				parent = r.RelatedIDs[0]
			}
			if parent != "" {
				resolves[parent] = append(resolves[parent], r)
			}
		}
	}

	x := &extractor{in: in, p: p, out: Extracted{Judgment: map[Subject]JudgmentStat{}}, base: baseRates(in)}
	judg := map[Subject][]float64{}
	for _, r := range recs {
		switch {
		case r.SubCall != nil && r.SubCall.Kind == types.SubCallCast:
			x.subCast(r, settlements[r.RecordID])
		case r.Precommit != nil && r.SubCall == nil:
			x.precommit(r, resolves[r.RecordID])
		case r.PSuccess != nil && r.SubCall == nil && r.MaterialityProcedure != nil && settled(r.Discrepancy):
			x.request(r)
		}
		if len(r.Consequences) > 0 || len(in.Materialized[r.RecordID]) > 0 {
			x.coverage(r, settlements[r.RecordID], resolves[r.RecordID])
		}
		x.judgment(r, judg)
	}
	x.frames()
	x.refusals()
	x.ownerActs()
	x.platform()
	x.costs()

	for s, vals := range judg {
		n := float64(len(vals))
		mean := 0.0
		for _, v := range vals {
			mean += v
		}
		mean /= n
		varr := 0.0
		for _, v := range vals {
			varr += (v - mean) * (v - mean)
		}
		half := 0.0
		if n > 1 {
			half = 1.96 * math.Sqrt(varr/(n-1)/n)
		}
		x.out.Judgment[s] = JudgmentStat{N: len(vals), MeanRPS: mean, Half: half}
	}
	sort.SliceStable(x.out.Items, func(i, j int) bool {
		a, b := x.out.Items[i], x.out.Items[j]
		if !a.At.Equal(b.At) {
			return a.At.Before(b.At)
		}
		if a.RecordID != b.RecordID {
			return a.RecordID < b.RecordID
		}
		if a.Class != b.Class {
			return a.Class < b.Class
		}
		return a.Desk < b.Desk
	})
	sort.SliceStable(x.out.Excluded, func(i, j int) bool {
		a, b := x.out.Excluded[i], x.out.Excluded[j]
		if a.RecordID != b.RecordID {
			return a.RecordID < b.RecordID
		}
		return a.Reason < b.Reason
	})
	sort.SliceStable(x.out.Surprises, func(i, j int) bool {
		a, b := x.out.Surprises[i], x.out.Surprises[j]
		if a.RecordID != b.RecordID {
			return a.RecordID < b.RecordID
		}
		return a.Label < b.Label
	})
	return x.out
}

func settled(d types.DiscrepancyClass) bool {
	return d == types.DiscrepancyConfirmed || d == types.DiscrepancyQualified || d == types.DiscrepancyContradicted
}

func baseRates(in Inputs) map[ActionClass]float64 { return in.BaseRates }

type extractor struct {
	in   Inputs
	p    Params
	out  Extracted
	base map[ActionClass]float64
}

func (x *extractor) exclude(rec types.CommitmentRecord, desk, reason string) {
	x.out.Excluded = append(x.out.Excluded, Excluded{RecordID: rec.RecordID, Desk: desk, Reason: reason})
}

func recordDesk(rec types.CommitmentRecord) string {
	if d := strings.TrimSpace(rec.Provenance.Desk); d != "" {
		return d
	}
	if rec.SubCall != nil {
		if d := DeskOf(rec.SubCall.ParentAgent); d != "" {
			return d
		}
	}
	return DeskOf(rec.Provenance.AgentID)
}

// actionOf is the record's action class: ClassOf, with a precommit's own
// stated class counted as frozen.
func actionOf(rec types.CommitmentRecord) ActionClass {
	if rec.Precommit != nil && rec.Precommit.ImpactClass != "" {
		rec.ImpactClass = maxClass(rec.ImpactClass, rec.Precommit.ImpactClass)
	}
	return ClassOf(rec)
}

// frozenChecks verifies the record's freeze and its reference. It returns the
// reference (zero value, false, when none) and an exclusion reason.
func (x *extractor) frozenChecks(rec types.CommitmentRecord) (FrozenReference, bool, string) {
	if rec.MaterialityProcedure != nil && !rec.MaterialityProcedure.Verify() {
		return FrozenReference{}, false, ExcludedFreezeHash
	}
	ref, ok := x.in.References[rec.RecordID]
	if !ok {
		return FrozenReference{}, false, ""
	}
	if ref.Hash == "" || ref.Hash != ref.Digest() || ref.RecordID != rec.RecordID {
		return FrozenReference{}, false, ExcludedReferenceHash
	}
	if rec.MaterialityProcedure != nil && ref.ProcedureHash != rec.MaterialityProcedure.Hash {
		return FrozenReference{}, false, ExcludedReferenceHash
	}
	return ref, true, ""
}

// weight is the frozen weight w = P_ref * impact * relevance. The sender's own
// stated weight is never read: with no frozen reference the class base applies
// (P_ref 1, relevance 1), so the agent cannot shrink the weight on what it
// would rather not be judged on.
func weight(class ActionClass, ref FrozenReference, hasRef bool) float64 {
	pref, rel := 1.0, 1.0
	if hasRef {
		pref, rel = ref.PRef, ref.Relevance
	}
	return pref * ImpactNumber(class.Impact) * rel
}

// brier is the Brier score of a distribution against the answer that held,
// sum over answers of (p - outcome)^2 / 2, which for two answers is (p-y)^2.
func brier(answers []string, probs map[string]float64, truth string) float64 {
	s := 0.0
	for _, a := range answers {
		o := 0.0
		if a == truth {
			o = 1
		}
		d := probs[a] - o
		s += d * d
	}
	return s / 2
}

// uniformBrier is the base-rate Brier score of ignorance: the uniform
// distribution.
func uniformBrier(answers []string, truth string) float64 {
	u := make(map[string]float64, len(answers))
	for _, a := range answers {
		u[a] = 1 / float64(len(answers))
	}
	return brier(answers, u, truth)
}

// questionSkill is d = S_ref - S for one typed question, unclipped.
func questionSkill(q types.TypedQuestion, truth string) (float64, bool) {
	if len(q.Probabilities) == 0 {
		return 0, false
	}
	known := false
	for _, a := range q.Answers {
		if a == truth {
			known = true
		}
	}
	if !known {
		return 0, false
	}
	return uniformBrier(q.Answers, truth) - brier(q.Answers, q.Probabilities, truth), true
}

func discrepancyY(d types.DiscrepancyClass) (float64, bool) {
	switch d {
	case types.DiscrepancyConfirmed, types.DiscrepancyAnswered:
		return 1, true
	case types.DiscrepancyQualified:
		return 0.5, true
	case types.DiscrepancyContradicted:
		return 0, true
	}
	return 0, false
}

func (x *extractor) selfFulfilling(rec types.CommitmentRecord, desk string) bool {
	if e := strings.TrimSpace(rec.SubjectRefs["outcome_emitter"]); e != "" && e == desk {
		return true
	}
	return x.in.SelfFulfilling != nil && x.in.SelfFulfilling(rec)
}

// independent reports whether a scorer is independent of the subject: not the
// subject's own desk, and not from the subject's publisher.
func (x *extractor) scorerExclusion(scorerAgent, desk string) string {
	sd := DeskOf(scorerAgent)
	if sd == "" {
		return ""
	}
	if sd == desk {
		return ExcludedSelfScored
	}
	pub, spub := x.in.info(desk).Publisher, x.in.info(sd).Publisher
	if pub != "" && pub == spub {
		return ExcludedSamePublisher
	}
	return ""
}

func (x *extractor) subCast(rec types.CommitmentRecord, setts []types.CommitmentRecord) {
	if len(setts) == 0 {
		return
	}
	desk := recordDesk(rec)
	role := strings.TrimSpace(rec.SubCall.Role)
	if desk == "" {
		return
	}
	if rec.IsWeightZero() {
		x.exclude(rec, desk, ExcludedWeightZero)
		return
	}
	ref, hasRef, why := x.frozenChecks(rec)
	if why != "" {
		x.exclude(rec, desk, why)
		return
	}
	if x.selfFulfilling(rec, desk) {
		x.exclude(rec, desk, ExcludedSelfFulfilling)
		return
	}
	// Which settlement decides, and why one might not count.
	var rule, judge, self *types.CommitmentSubCall
	var judgeRec types.CommitmentRecord
	interrupted, sameFamily := false, false
	for _, s := range setts {
		b := s.SubCall
		if b.Reason == types.ClosureReasonInterruptedByRestart || b.Phase == types.SubCallPhaseInterrupted {
			interrupted = true
			continue
		}
		switch b.Method {
		case types.CalibrationScorerRule:
			if rule == nil && b.Contradicted {
				rule = b
			}
		case types.CalibrationScorerIndependent:
			if b.SameFamily {
				sameFamily = true
			} else if judge == nil {
				judge, judgeRec = b, s
			}
		case types.CalibrationScorerSelf:
			self = b
		}
	}
	switch {
	case rule == nil && judge == nil && interrupted:
		x.exclude(rec, desk, ExcludedInterrupted)
		return
	case rule == nil && judge == nil && sameFamily:
		x.exclude(rec, desk, ExcludedSameFamily)
		return
	case rule == nil && judge == nil && self != nil:
		x.exclude(rec, desk, ExcludedSelfScored)
		return
	case rule == nil && judge == nil:
		return
	}
	settler := types.SubCallJudgeScorerID
	at := time.Time{}
	if judge != nil && rule == nil {
		if r := x.scorerExclusion(judgeRec.Provenance.AgentID, desk); r != "" {
			x.exclude(rec, desk, r)
			return
		}
		if b := judge.Scorer; b != "" {
			settler = b
		}
		at = parseTime(judgeRec.Provenance.CommittedAt)
	} else {
		settler = types.SubCallRuleScorerPrefix + rule.Rule
		for _, s := range setts {
			if s.SubCall == rule {
				at = parseTime(s.Provenance.CommittedAt)
			}
		}
	}
	class := actionOf(rec)
	w := weight(class, ref, hasRef)
	if w == 0 {
		x.exclude(rec, desk, ExcludedZeroWeight)
		return
	}
	model := rec.Provenance.ModelID

	// E1: the parent's precommitment about the result, as unclipped skill.
	d, skill, okSkill := 0.0, true, false
	if rule != nil {
		// a rule contradiction is final for the facts it covers
		d, okSkill = x.ruleSkill(rec), true
	} else {
		d, okSkill = x.typedSkill(rec, ref, hasRef, judge.Verdicts)
	}
	if !okSkill {
		// no typed probabilities: p_success against the settled class
		if y, ok := discrepancyY(judge.Class); ok && rec.PSuccess != nil && rule == nil && judge.Class != types.DiscrepancyAnswered {
			d, okSkill = x.binarySkill(*rec.PSuccess, y, class), true
		}
	}
	if okSkill {
		x.out.Items = append(x.out.Items, Item{Class: E1, RecordID: rec.RecordID, Desk: desk, Model: model, Action: class, At: at,
			W: w, D: d, Skill: skill, M: 1, Settler: settler})
	}
	// E5: the callee role's report accepted or contradicted by the settler.
	if role != "" && judge != nil && rule == nil && x.scorerExclusion(judgeRec.Provenance.AgentID, role) != "" {
		// the settler is not independent of the callee either
		x.exclude(rec, role, x.scorerExclusion(judgeRec.Provenance.AgentID, role))
	} else if role != "" {
		cls := types.DiscrepancyUnresolved
		if rule != nil {
			cls = types.DiscrepancyContradicted
		} else {
			cls = judge.Class
		}
		if y, ok := discrepancyY(cls); ok {
			x.out.Items = append(x.out.Items, Item{Class: E5, RecordID: rec.RecordID, Desk: role, Action: class, At: at,
				W: w * x.p.PeerFactor, Q: y, M: 1, Settler: settler})
		}
	}
}

func (x *extractor) ruleSkill(rec types.CommitmentRecord) float64 {
	// The rule contradicted the Cast: every ranked question is lost. Skill is
	// measured against a truth that is the least likely top answer.
	sum, n := 0.0, 0
	for _, q := range rec.Prediction.Questions {
		top, _, ok := types.TopAnswer(q)
		if !ok {
			continue
		}
		// truth: any answer other than the top; take the lowest-probability one
		worst, wp := "", 2.0
		for _, a := range q.Answers {
			if a != top && q.Probabilities[a] < wp {
				worst, wp = a, q.Probabilities[a]
			}
		}
		if d, ok := questionSkill(q, worst); ok {
			sum += d
			n++
		}
	}
	if n == 0 {
		return -0.25
	}
	return sum / float64(n)
}

func (x *extractor) typedSkill(rec types.CommitmentRecord, ref FrozenReference, hasRef bool, verdicts []types.SubCallVerdict) (float64, bool) {
	byQ := map[int]types.SubCallVerdict{}
	for _, v := range verdicts {
		byQ[v.Q] = v
	}
	sum, wsum := 0.0, 0.0
	for i, q := range rec.Prediction.Questions {
		v, ok := byQ[i]
		if !ok || v.Answer == "" {
			continue
		}
		d, ok := questionSkill(q, v.Answer)
		if !ok {
			continue
		}
		qw := 1.0
		if hasRef && i < len(ref.QuestionWeights) {
			qw = ref.QuestionWeights[i]
		}
		if qw <= 0 {
			continue
		}
		sum += qw * d
		wsum += qw
	}
	if wsum == 0 {
		return 0, false
	}
	return sum / wsum, true
}

func (x *extractor) binarySkill(p, y float64, class ActionClass) float64 {
	base := 0.5
	if b, ok := x.base[class]; ok {
		base = b
	}
	return (base-y)*(base-y) - (p-y)*(p-y)
}

func (x *extractor) precommit(rec types.CommitmentRecord, resolves []types.CommitmentRecord) {
	if len(resolves) == 0 || rec.Precommit == nil || len(rec.Precommit.Distribution) == 0 {
		return
	}
	desk := recordDesk(rec)
	if desk == "" {
		return
	}
	var res types.CommitmentRecord
	for _, r := range resolves {
		if res.RecordID == "" || r.RecordID < res.RecordID {
			res = r
		}
	}
	if rec.Discrepancy == types.DiscrepancyUnresolved && res.Resolve.Verdict == "" {
		return
	}
	if res.Observation.Excerpt == types.ClosureReasonInterruptedByRestart || res.Discrepancy == types.DiscrepancyUnresolved && res.Resolve.Verdict == "" {
		x.exclude(rec, desk, ExcludedInterrupted)
		return
	}
	ref, hasRef, why := x.frozenChecks(rec)
	if why != "" {
		x.exclude(rec, desk, why)
		return
	}
	if x.selfFulfilling(rec, desk) {
		x.exclude(rec, desk, ExcludedSelfFulfilling)
		return
	}
	settler := res.Provenance.AgentID
	if r := x.scorerExclusion(settler, desk); r != "" {
		x.exclude(rec, desk, r)
		return
	}
	class := actionOf(rec)
	w := weight(class, ref, hasRef)
	if w == 0 {
		x.exclude(rec, desk, ExcludedZeroWeight)
		return
	}
	answers := sortedKeys(rec.Precommit.Distribution)
	q := types.TypedQuestion{Question: rec.Precommit.Question, Answers: answers, Probabilities: rec.Precommit.Distribution}
	d, ok := questionSkill(q, res.Resolve.Verdict)
	if !ok {
		return
	}
	at := parseTime(res.Provenance.CommittedAt)
	x.out.Items = append(x.out.Items, Item{Class: E1, RecordID: rec.RecordID, Desk: desk, Model: rec.Provenance.ModelID, Action: class, At: at,
		W: w, D: d, Skill: true, M: 1, Settler: settler})
}

// request is a settled desk request with a stated p_success: the sender's
// forecast that the request achieves its outcome, scored against the settled
// class by the Brier skill against the class base rate.
func (x *extractor) request(rec types.CommitmentRecord) {
	desk := recordDesk(rec)
	if desk == "" {
		return
	}
	ref, hasRef, why := x.frozenChecks(rec)
	if why != "" {
		x.exclude(rec, desk, why)
		return
	}
	if x.selfFulfilling(rec, desk) {
		x.exclude(rec, desk, ExcludedSelfFulfilling)
		return
	}
	settler := ""
	for _, s := range rec.Scores {
		if !s.Self && s.ScorerAgentID != "" {
			settler = s.ScorerAgentID
		}
	}
	if settler == "" {
		settler = rec.Addressee
	}
	if r := x.scorerExclusion(settler, desk); r != "" {
		x.exclude(rec, desk, r)
		return
	}
	class := actionOf(rec)
	w := weight(class, ref, hasRef)
	if w == 0 {
		x.exclude(rec, desk, ExcludedZeroWeight)
		return
	}
	y, _ := discrepancyY(rec.Discrepancy)
	at := parseTime(rec.Provenance.ResolvedAt)
	if at.IsZero() {
		at = parseTime(rec.Observation.ObservedAt)
	}
	x.out.Items = append(x.out.Items, Item{Class: E1, RecordID: rec.RecordID, Desk: desk, Model: rec.Provenance.ModelID, Action: class, At: at,
		W: w, D: x.binarySkill(*rec.PSuccess, y, class), Skill: true, M: 1, Settler: settler})
}

// namingCounts reports whether a naming makes a consequence foreseeable for a
// record committed at commit by desk: named by a reference predictor or the
// critic panel, or by another desk's scored record carrying a probability
// above the floor from another publisher, and in every case before the
// commitment.
func (x *extractor) namingCounts(c FrozenConsequence, desk string, commit time.Time) bool {
	named := parseTime(c.NamedAt)
	if named.IsZero() || commit.IsZero() || !named.Before(commit) {
		return false
	}
	switch c.NamedBy {
	case NamedByReference, NamedByCritic:
		return true
	case NamedByPeer:
		if c.NamerDesk == "" || c.NamerDesk == desk || !c.NamerScored || c.NamerP < MinNamingProbability {
			return false
		}
		pub, npub := x.in.info(desk).Publisher, x.in.info(c.NamerDesk).Publisher
		if pub != "" && pub == npub {
			return false
		}
		return true
	}
	return false
}

// coverage is score 3: consequences that turned out material and that the
// agent left out. Foreseeable ones (named before the commitment) are E2
// evidence against the desk at the namer's frozen weight; the rest are world
// surprises and count against no one.
func (x *extractor) coverage(rec types.CommitmentRecord, setts, resolves []types.CommitmentRecord) {
	desk := recordDesk(rec)
	if desk == "" {
		return
	}
	ref, hasRef, why := x.frozenChecks(rec)
	if why != "" {
		if len(setts)+len(resolves) > 0 {
			x.exclude(rec, desk, why)
		}
		return
	}
	commit := parseTime(rec.Provenance.CommittedAt)
	if hasRef && !parseTime(ref.CommittedAt).IsZero() {
		commit = parseTime(ref.CommittedAt)
	}
	predicted := map[string]bool{}
	material := map[string]bool{}
	for _, c := range rec.Consequences {
		if c.Predicted {
			predicted[c.Label] = true
		}
		if c.Observed {
			material[c.Label] = true
		}
	}
	for _, l := range x.in.Materialized[rec.RecordID] {
		material[l] = true
	}
	named := map[string]FrozenConsequence{}
	if hasRef {
		for _, c := range ref.Consequences {
			if x.namingCounts(c, desk, commit) {
				if prev, ok := named[c.Label]; !ok || c.W() > prev.W() {
					named[c.Label] = c
				}
			}
		}
	}
	class := actionOf(rec)
	at := parseTime(rec.Observation.ObservedAt)
	if at.IsZero() {
		for _, s := range setts {
			if t := parseTime(s.Provenance.CommittedAt); t.After(at) {
				at = t
			}
		}
		for _, s := range resolves {
			if t := parseTime(s.Provenance.CommittedAt); t.After(at) {
				at = t
			}
		}
	}
	if at.IsZero() {
		at = commit
	}
	for _, label := range sortedKeys(boolKeys(material)) {
		if predicted[label] {
			continue
		}
		c, foreseeable := named[label]
		if !foreseeable {
			x.out.Surprises = append(x.out.Surprises, Surprise{RecordID: rec.RecordID, Label: label})
			continue
		}
		w := c.W()
		if w <= 0 {
			continue
		}
		x.out.Items = append(x.out.Items, Item{Class: E2, RecordID: rec.RecordID, Desk: desk, Model: rec.Provenance.ModelID, Action: class,
			At: at, W: w, Q: 0, M: x.p.mult(E2, class), Settler: "frozen:" + c.NamedBy})
	}
}

func boolKeys(m map[string]bool) map[string]bool { return m }

// judgment is score 2: the stated impact class, as a point-mass forecast of
// the realized class, scored by the ranked probability score.
func (x *extractor) judgment(rec types.CommitmentRecord, into map[Subject][]float64) {
	realized, ok := x.in.Realized[rec.RecordID]
	if !ok || rec.MaterialityProcedure == nil || rec.MaterialityProcedure.NoProcedure {
		return
	}
	desk := recordDesk(rec)
	if desk == "" {
		return
	}
	s := Subject{SubjectAgent, desk}
	into[s] = append(into[s], rps(rec.ImpactClass, realized))
}

// rps is the ranked probability score of a point-mass forecast of class f
// against the realized class r over the ordered classes I0 to I4.
func rps(f, r types.ImpactClass) float64 {
	fr, _ := f.Rank()
	rr, _ := r.Rank()
	s := 0.0
	for k := 0; k < 4; k++ {
		var F, O float64
		if fr <= k {
			F = 1
		}
		if rr <= k {
			O = 1
		}
		s += (F - O) * (F - O)
	}
	return s / 4
}

func (x *extractor) frames() {
	for _, f := range x.in.Frames {
		declared := map[string]bool{}
		for _, d := range f.Declared {
			declared[strings.TrimSpace(d)] = true
		}
		seen := map[string]bool{}
		for _, u := range f.Used {
			u = strings.TrimSpace(u)
			if declared[u] || seen[u] {
				continue
			}
			seen[u] = true
			imp, ok := GrantImpact(u)
			if !ok {
				continue
			}
			class := ActionClass{Grant: u, Impact: imp}
			w := ImpactNumber(imp)
			if w == 0 {
				continue
			}
			x.out.Items = append(x.out.Items, Item{Class: E3, RecordID: f.RecordID, Desk: f.Desk, Action: class, At: parseTime(f.At),
				W: w, Q: 0, M: x.p.mult(E3, class), Settler: "system:rule:frame"})
		}
	}
}

func (x *extractor) refusals() {
	for _, r := range x.in.Refusals {
		imp, ok := GrantImpact(r.Grant)
		if !ok || ImpactNumber(imp) == 0 {
			continue
		}
		class := ActionClass{Grant: r.Grant, Impact: imp}
		x.out.Items = append(x.out.Items, Item{Class: E4, RecordID: "refusal:" + r.Desk + ":" + r.Grant + ":" + r.At, Desk: r.Desk, Action: class,
			At: parseTime(r.At), W: ImpactNumber(imp), Q: 0, M: x.p.mult(E4, class), Settler: "system:rule:frame"})
	}
}

func (x *extractor) ownerActs() {
	for _, a := range x.in.OwnerActs {
		w := ImpactNumber(a.Class.Impact)
		if w == 0 {
			continue
		}
		q := 0.0
		if a.Act == "approve" {
			q = 1
		}
		x.out.Items = append(x.out.Items, Item{Class: E6, RecordID: "owner:" + a.Desk + ":" + a.Act + ":" + a.At, Desk: a.Desk, Action: a.Class,
			At: parseTime(a.At), W: w, Q: q, M: x.p.mult(E6, a.Class), Settler: "owner"})
	}
}

func (x *extractor) platform() {
	for _, e := range x.in.Platform {
		classes := e.Classes
		if len(classes) == 0 {
			classes = []ActionClass{wildcardClass}
		}
		var direct []Subject
		if e.Package != "" {
			direct = append(direct, Subject{SubjectPackage, e.Package})
		}
		if e.Publisher != "" {
			direct = append(direct, Subject{SubjectPublisher, e.Publisher})
		}
		for _, c := range classes {
			w := ImpactNumber(c.Impact)
			if c == wildcardClass {
				w = ImpactNumber(types.ImpactI4)
			}
			x.out.Items = append(x.out.Items, Item{Class: E7, RecordID: "platform:" + e.Kind + ":" + e.Package + ":" + e.At, Direct: direct, Action: c,
				At: parseTime(e.At), W: w, Q: 0, M: x.p.mult(E7, c), Settler: "platform"})
		}
	}
}

// wildcardClass marks platform evidence that applies to every class a subject
// holds.
var wildcardClass = ActionClass{Grant: "*", Impact: types.ImpactI4}

func (x *extractor) costs() {
	for _, c := range x.in.Costs {
		if c.Estimated <= 0 || ImpactNumber(c.Class.Impact) == 0 {
			continue
		}
		ratio := c.Metered / c.Estimated
		q := 0.0
		if ratio >= 0.5 && ratio <= 2 {
			q = 1
		}
		x.out.Items = append(x.out.Items, Item{Class: E8, RecordID: "cost:" + c.Desk + ":" + c.At, Desk: c.Desk, Action: c.Class, At: parseTime(c.At),
			W: ImpactNumber(c.Class.Impact) * x.p.CostFactor, Q: q, M: 1, Settler: "system:rule:gateway"})
	}
}
