package types

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
)

// Sub-RLM calls are Casts, and who scores them (modular architecture MA1.5,
// plan section 4.4).
//
// A call that makes a child agent do work for a desk is a Cast carrying the
// parent's precommitment about the result: typed questions with the parent's
// probabilities, in Prediction.Questions. The child's Report is the
// observation. Scoring has layers, in order: rules first (final for the facts
// they cover), a blind independent judge, the parent's self-score as a
// learning signal (it never settles), and management's third opinion on
// sampled material disagreements. Nothing is exempt.
//
// Every record of the sub-call carries a CommitmentSubCall body and is
// written through the store's plain commitment-record path (no inbox, no
// round, no wake): the sub-call plane is an audit plane, not a conversation.
// Records written before the slice have none of this, and still decode.

// Sub-call record kinds.
const (
	SubCallCast         = "cast"
	SubCallReport       = "report"
	SubCallSettlement   = "settlement"
	SubCallSelfScore    = "self_score"
	SubCallJudgeWork    = "judge_work"
	SubCallJudgeClose   = "judge_close"
	SubCallDisagreement = "disagreement"
)

// Judge work phases (a judge_close record's Phase).
const (
	SubCallPhaseAnswered    = "answered"
	SubCallPhaseFailed      = "failed"
	SubCallPhaseInterrupted = "interrupted"
)

// Scorer identities. A settlement's scorer is a host service, never a desk.
const (
	// SubCallJudgeScorerID is the independent judge's agent id.
	SubCallJudgeScorerID = "system:judge"
	// SubCallRuleScorerPrefix prefixes a rule scorer ("system:rule:<kind>").
	SubCallRuleScorerPrefix = "system:rule:"
)

// Bounds. The parameters are the Phase 1 ledger's to tune (plan 10.2).
const (
	// MaxSubCallQuestions is the most typed questions one Cast carries.
	MaxSubCallQuestions = 6
	// MaxSubCallAnswers is the most answers one typed question has.
	MaxSubCallAnswers = 8
	// SubCallProbabilityTolerance is how far a distribution may be from 1.
	SubCallProbabilityTolerance = 1e-3
	// MaterialDisagreementGap is the |self - independent| that counts as a
	// material disagreement.
	MaterialDisagreementGap = 0.4
	// ThirdOpinionBaseRate is the base chance a material disagreement is
	// sampled for management's third opinion; weight raises it to 1.
	ThirdOpinionBaseRate = 0.25
)

// SubCallVerdict is a scorer's answer to one typed question of a Cast. Q is
// the question's index in Prediction.Questions.
type SubCallVerdict struct {
	Q             int                `json:"q"`
	Answer        string             `json:"answer,omitempty"`
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
}

// SubCallSelfAnswer is the parent's own answer to a question: the answer it
// ranked first and its probability for it (its confidence).
type SubCallSelfAnswer struct {
	Q      int     `json:"q"`
	Answer string  `json:"answer"`
	P      float64 `json:"p"`
}

// CommitmentSubCall is the body of a sub-call record. Kind says which fields
// are read.
type CommitmentSubCall struct {
	Kind string `json:"kind"`
	// FanOutID names the fan-out the call belongs to; a judge scores one
	// fan-out in one call.
	FanOutID string `json:"fan_out_id,omitempty"`
	// Role is the callee role, ParentRunID and ParentAgent the caller.
	Role        string `json:"role,omitempty"`
	ParentRunID string `json:"parent_run_id,omitempty"`
	ParentAgent string `json:"parent_agent,omitempty"`
	ChildKey    string `json:"child_key,omitempty"`
	// Model is the model that did the work (cast, report) or scored it
	// (settlement).
	Model string `json:"model,omitempty"`
	// CastID is the Cast a report, settlement, self-score or disagreement
	// concerns.
	CastID string `json:"cast_id,omitempty"`

	// Settlement fields.
	Method       string           `json:"method,omitempty"`
	Scorer       string           `json:"scorer,omitempty"`
	Rule         string           `json:"rule,omitempty"`
	Contradicted bool             `json:"contradicted,omitempty"`
	Class        DiscrepancyClass `json:"class,omitempty"`
	Verdicts     []SubCallVerdict `json:"verdicts,omitempty"`
	// Answered is the judge's distribution over "did the report answer the
	// question" (answered, partly, not_answered).
	Answered map[string]float64 `json:"answered,omitempty"`
	// SameFamily marks a settlement whose scorer shares a model family with
	// the work it scored; it is kept and never counted as independent.
	SameFamily bool   `json:"same_family,omitempty"`
	Reason     string `json:"reason,omitempty"`

	// Self-score fields.
	Self []SubCallSelfAnswer `json:"self,omitempty"`

	// Judge work fields. Casts are the batch; WorkID ties a close to it.
	Casts  []string `json:"casts,omitempty"`
	WorkID string   `json:"work_id,omitempty"`
	Phase  string   `json:"phase,omitempty"`
	// Cost, tracked and never capped.
	InputTokens  int     `json:"input_tokens,omitempty"`
	OutputTokens int     `json:"output_tokens,omitempty"`
	CostUSD      float64 `json:"cost_usd,omitempty"`
}

// Record ids of a sub-call. Deterministic, so a replay or a second settler
// re-mints nothing.

// SubCallCastRecordID is the Cast of child i of a fan-out.
func SubCallCastRecordID(fanOut string, i int) string {
	return fmt.Sprintf("subcall:%s:c%d", strings.TrimSpace(fanOut), i)
}

// SubCallReportRecordID is the child's Report for a Cast.
func SubCallReportRecordID(castID string) string { return castID + ":report" }

// SubCallSelfScoreRecordID is the parent's self-score for a Cast.
func SubCallSelfScoreRecordID(castID string) string { return castID + ":self" }

// SubCallSettlementRecordID is one scorer's settlement of a Cast.
func SubCallSettlementRecordID(castID, scorer string) string {
	return castID + ":settle:" + slugID(scorer)
}

// SubCallDisagreementRecordID is the third-opinion request for a Cast.
func SubCallDisagreementRecordID(castID string) string { return castID + ":disagree" }

// SubCallJudgeWorkID is the judge's work on one batch of Casts.
func SubCallJudgeWorkID(fanOut string, casts []string) string {
	ids := append([]string(nil), casts...)
	sort.Strings(ids)
	sum := sha256.Sum256([]byte(strings.Join(ids, "\n")))
	return "judgework:" + strings.TrimSpace(fanOut) + ":" + hex.EncodeToString(sum[:6])
}

// SubCallJudgeCloseRecordID closes a judge work record.
func SubCallJudgeCloseRecordID(workID string) string { return workID + ":close" }

// SubCallQuestionID is the id of typed question q of child i in the judge's
// request; SubCallAnsweredID is the id of the "did it answer" question.
func SubCallQuestionID(child, q int) string { return fmt.Sprintf("c%d.q%d", child, q) }
func SubCallAnsweredID(child int) string    { return fmt.Sprintf("c%d.answered", child) }

func slugID(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			return r
		case r >= 'A' && r <= 'Z':
			return r + ('a' - 'A')
		}
		return '-'
	}, strings.TrimSpace(s))
}

// SubCallBody returns a record's sub-call body of kind kind.
func (rec CommitmentRecord) SubCallBody(kind string) (*CommitmentSubCall, bool) {
	if rec.SubCall == nil || rec.SubCall.Kind != kind {
		return nil, false
	}
	return rec.SubCall, true
}

// IsTyped reports a sub-call Cast that carries at least one typed question.
func (rec CommitmentRecord) IsTyped() bool { return len(rec.Prediction.Questions) > 0 }

// IsWeightZero reports a Cast that states no weight and says why.
func (rec CommitmentRecord) IsWeightZero() bool { return strings.TrimSpace(rec.WeightZero) != "" }

// ValidateSubCallCast checks the proof's invariant on a sub-call Cast: at
// least one typed question, or the weight-zero mark with its reason.
func ValidateSubCallCast(rec CommitmentRecord) error {
	if rec.SubCall == nil || rec.SubCall.Kind != SubCallCast {
		return fmt.Errorf("record %s is not a sub-call Cast", rec.RecordID)
	}
	if !rec.IsTyped() && !rec.IsWeightZero() {
		return fmt.Errorf("sub-call Cast %s has no typed question and is not marked weight zero", rec.RecordID)
	}
	if rec.IsWeightZero() && (rec.Weight != nil || rec.PSuccess != nil) {
		return fmt.Errorf("sub-call Cast %s is weight zero and also states a weight or p_success", rec.RecordID)
	}
	if err := ValidateTypedQuestions(rec.Prediction.Questions); err != nil {
		return fmt.Errorf("sub-call Cast %s: %w", rec.RecordID, err)
	}
	return ValidateRequestRecord(rec)
}

// ValidateTypedQuestions checks typed questions with probabilities. An empty
// list is valid here (the writer decides whether one is required). A question
// with no probabilities is valid only as an unanswered prompt of a weight-zero
// Cast: when Probabilities is present it must cover every answer and sum to 1.
func ValidateTypedQuestions(qs []TypedQuestion) error {
	if len(qs) > MaxSubCallQuestions {
		return fmt.Errorf("%d typed questions; at most %d", len(qs), MaxSubCallQuestions)
	}
	for i, q := range qs {
		if strings.TrimSpace(q.Question) == "" {
			return fmt.Errorf("question %d is empty", i)
		}
		if len(q.Answers) < 2 || len(q.Answers) > MaxSubCallAnswers {
			return fmt.Errorf("question %d needs 2 to %d answers", i, MaxSubCallAnswers)
		}
		seen := map[string]bool{}
		for _, a := range q.Answers {
			a = strings.TrimSpace(a)
			if a == "" || seen[a] {
				return fmt.Errorf("question %d: answers must be non-empty and distinct", i)
			}
			seen[a] = true
		}
		if q.Weight < 0 || q.Weight > 1 || math.IsNaN(q.Weight) {
			return fmt.Errorf("question %d: weight %v is outside [0,1]", i, q.Weight)
		}
		if len(q.Probabilities) == 0 {
			continue
		}
		sum := 0.0
		for a, p := range q.Probabilities {
			if !seen[a] {
				return fmt.Errorf("question %d: probability for %q, which is not an answer", i, a)
			}
			if !finiteUnit(p) {
				return fmt.Errorf("question %d: probability %v for %q is outside [0,1]", i, p, a)
			}
			sum += p
		}
		if len(q.Probabilities) != len(q.Answers) || math.Abs(sum-1) > SubCallProbabilityTolerance {
			return fmt.Errorf("question %d: probabilities must cover every answer and sum to 1 (got %d of %d, sum %v)", i, len(q.Probabilities), len(q.Answers), sum)
		}
	}
	return nil
}

// TopAnswer returns the answer a question's probabilities rank first and its
// probability; ties go to the answer listed first. ok is false with none.
func TopAnswer(q TypedQuestion) (answer string, p float64, ok bool) {
	for _, a := range q.Answers {
		if v, has := q.Probabilities[a]; has && (!ok || v > p) {
			answer, p, ok = a, v, true
		}
	}
	return answer, p, ok
}

// SelfAnswers is the parent's own score of its Cast: for each question with
// probabilities, the answer it ranked first and its confidence.
func SelfAnswers(qs []TypedQuestion) []SubCallSelfAnswer {
	var out []SubCallSelfAnswer
	for i, q := range qs {
		if a, p, ok := TopAnswer(q); ok {
			out = append(out, SubCallSelfAnswer{Q: i, Answer: a, P: p})
		}
	}
	return out
}

// Weight floor from grants (plan 4.4 and 10.2). The floor is the lowest
// weight a delegation to the role can honestly state: a callee that can only
// read the world has little consequence to weigh, one that disposes of
// captures or writes has more. It is derived from the role's registered verbs
// (the grants), never from its name. A weight below it flags the record
// inert; nothing is refused.
const (
	subCallFloorBase    = 0.05
	subCallFloorPerVerb = 0.05
	subCallFloorMax     = 0.30
)

// effectfulVerbs are the registered verbs that change something outside the
// callee's own cell.
var effectfulVerbs = map[string]bool{
	"Cast": true, "Feed": true, "Decline": true, "Done": true, "Report": true, "Write": true,
	"WriteFile": true, "Exec": true, "Precommit": true, "Freeze": true, "Delegate": true,
}

// SubCallWeightFloor is the weight floor of a callee role, from its grants.
func SubCallWeightFloor(role string) float64 {
	d, ok := deskmanifest.Lookup(role)
	if !ok {
		return subCallFloorBase
	}
	n := 0
	for _, v := range d.Verbs {
		if effectfulVerbs[v] {
			n++
		}
	}
	return math.Min(subCallFloorMax, subCallFloorBase+subCallFloorPerVerb*float64(n))
}

// Inertness flags the writer sets (never a refusal).
const (
	InertWeightBelowFloor = "weight_below_floor"
	InertNoFalsifier      = "no_typed_question"
)

// InertFlagsFor returns the cheap inertness flags of a Cast: a weight below
// the callee's floor from its grants, and no typed question.
func InertFlagsFor(role string, rec CommitmentRecord) []string {
	var flags []string
	if rec.Weight != nil && *rec.Weight < SubCallWeightFloor(role) {
		flags = append(flags, InertWeightBelowFloor)
	}
	if !rec.IsTyped() {
		flags = append(flags, InertNoFalsifier)
	}
	return flags
}

// ModelFamily is the family a model id belongs to: the leading word of its id
// after any provider prefix ("deepseek-v4.1-flash" is "deepseek",
// "~typesafe/jev-1.13" is "jev"). Two models of one family do not score each
// other independently.
func ModelFamily(model string) string {
	m := strings.ToLower(strings.TrimSpace(model))
	if i := strings.LastIndex(m, "/"); i >= 0 {
		m = m[i+1:]
	}
	m = strings.TrimLeft(m, "~")
	end := len(m)
	for i, r := range m {
		if !(r >= 'a' && r <= 'z') {
			end = i
			break
		}
	}
	return m[:end]
}

// ClassifySubCall folds a scorer's verdicts against the parent's typed
// answers into a discrepancy class: every question the parent ranked matches
// is confirmed, some is qualified, none is contradicted. A Cast with no
// ranked question is classified from the "did the report answer" distribution:
// answered, qualified (partly) or contradicted (not answered).
func ClassifySubCall(qs []TypedQuestion, verdicts []SubCallVerdict, answered map[string]float64) DiscrepancyClass {
	byQ := map[int]SubCallVerdict{}
	for _, v := range verdicts {
		byQ[v.Q] = v
	}
	match, total := 0, 0
	for i, q := range qs {
		self, _, ok := TopAnswer(q)
		if !ok {
			continue
		}
		v, has := byQ[i]
		if !has || v.Answer == "" {
			continue
		}
		total++
		if v.Answer == self {
			match++
		}
	}
	switch {
	case total > 0 && match == total:
		return DiscrepancyConfirmed
	case total > 0 && match > 0:
		return DiscrepancyQualified
	case total > 0:
		return DiscrepancyContradicted
	}
	best, bp := "", -1.0
	for _, k := range []string{"answered", "partly", "not_answered"} {
		if p, ok := answered[k]; ok && p > bp {
			best, bp = k, p
		}
	}
	switch best {
	case "answered":
		return DiscrepancyAnswered
	case "partly":
		return DiscrepancyQualified
	case "not_answered":
		return DiscrepancyContradicted
	}
	return DiscrepancyUnresolved
}

// SubCallVerdictClass is the effective verdict of a Cast from its
// settlements: a rule settlement that contradicts decides it whatever the
// judge said (rules are final for the facts they cover); otherwise the
// independent settlement that is not same-family decides; otherwise the Cast
// is unsettled. The parent's self-score never decides.
func SubCallVerdictClass(settlements []CommitmentRecord) (DiscrepancyClass, string) {
	var judge *CommitmentSubCall
	for _, rec := range settlements {
		b, ok := rec.SubCallBody(SubCallSettlement)
		if !ok {
			continue
		}
		switch b.Method {
		case CalibrationScorerRule:
			if b.Contradicted {
				return DiscrepancyContradicted, CalibrationScorerRule
			}
		case CalibrationScorerIndependent:
			if !b.SameFamily && judge == nil {
				judge = b
			}
		}
	}
	if judge != nil {
		return judge.Class, CalibrationScorerIndependent
	}
	return DiscrepancyUnresolved, ""
}

// SelfFavorRow is one Cast's self-score against its independent score.
type SelfFavorRow struct {
	CastID string
	Agent  string
	Model  string
	Weight float64
	// Self and Independent are the weighted means over the Cast's questions
	// of the parent's confidence in its top answer and of the independent
	// scorer's probability of that same answer. A rule contradiction makes
	// Independent 0.
	Self, Independent float64
	Questions         int
}

// Gap is the self-favor gap of a row: positive when the parent scored its own
// Cast better than the independent scorer did.
func (r SelfFavorRow) Gap() float64 { return r.Self - r.Independent }

// Material reports a disagreement large enough for a third opinion.
func (r SelfFavorRow) Material() bool { return math.Abs(r.Gap()) >= MaterialDisagreementGap }

// SelfFavorRows computes the self-favor rows of the Casts that have an
// independent (non same-family) settlement and at least one ranked question.
// casts and settlements are replayed records, in any order.
func SelfFavorRows(casts, settlements []CommitmentRecord) []SelfFavorRow {
	bySettled := map[string][]CommitmentRecord{}
	for _, s := range settlements {
		if b, ok := s.SubCallBody(SubCallSettlement); ok {
			bySettled[b.CastID] = append(bySettled[b.CastID], s)
		}
	}
	var rows []SelfFavorRow
	for _, c := range casts {
		if b, ok := c.SubCallBody(SubCallCast); !ok || b == nil {
			continue
		}
		var judge *CommitmentSubCall
		ruleContradicted := false
		for _, s := range bySettled[c.RecordID] {
			sb, _ := s.SubCallBody(SubCallSettlement)
			switch {
			case sb.Method == CalibrationScorerRule && sb.Contradicted:
				ruleContradicted = true
			case sb.Method == CalibrationScorerIndependent && !sb.SameFamily && judge == nil:
				judge = sb
			}
		}
		if judge == nil && !ruleContradicted {
			continue
		}
		verdict := map[int]SubCallVerdict{}
		if judge != nil {
			for _, v := range judge.Verdicts {
				verdict[v.Q] = v
			}
		}
		var selfSum, indepSum, wSum float64
		n := 0
		for i, q := range c.Prediction.Questions {
			top, p, ok := TopAnswer(q)
			if !ok {
				continue
			}
			w := q.Weight
			if w <= 0 {
				w = 1
			}
			ind := 0.0
			if !ruleContradicted {
				ind = verdict[i].Probabilities[top]
			}
			selfSum += w * p
			indepSum += w * ind
			wSum += w
			n++
		}
		if n == 0 {
			continue
		}
		weight := SubCallWeightFloor(c.SubCall.Role)
		if c.Weight != nil {
			weight = *c.Weight
		}
		rows = append(rows, SelfFavorRow{CastID: c.RecordID, Agent: firstNonBlank(c.Provenance.Desk, c.Provenance.AgentID), Model: c.Provenance.ModelID,
			Weight: weight, Self: selfSum / wSum, Independent: indepSum / wSum, Questions: n})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].CastID < rows[j].CastID })
	return rows
}

func firstNonBlank(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// SelfFavorGroup is the self-favor gap of one agent and model.
type SelfFavorGroup struct {
	Agent           string  `json:"agent"`
	Model           string  `json:"model"`
	N               int     `json:"n"`
	MeanSelf        float64 `json:"mean_self"`
	MeanIndependent float64 `json:"mean_independent"`
	// Gap is MeanSelf - MeanIndependent: positive when the agent scores its
	// own sub-calls better than the independent scorer does.
	Gap             float64 `json:"gap"`
	SelfFavorable   int     `json:"self_favorable"`
	SelfUnfavorable int     `json:"self_unfavorable"`
	Material        int     `json:"material"`
}

// SelfFavorView groups rows per agent and model (the plan's "self-favor gap
// per agent and model"), sorted by agent then model.
func SelfFavorView(rows []SelfFavorRow) []SelfFavorGroup {
	type key struct{ agent, model string }
	groups := map[key]*SelfFavorGroup{}
	for _, r := range rows {
		k := key{r.Agent, r.Model}
		g := groups[k]
		if g == nil {
			g = &SelfFavorGroup{Agent: r.Agent, Model: r.Model}
			groups[k] = g
		}
		g.N++
		g.MeanSelf += r.Self
		g.MeanIndependent += r.Independent
		switch gap := r.Gap(); {
		case gap > 0:
			g.SelfFavorable++
		case gap < 0:
			g.SelfUnfavorable++
		}
		if r.Material() {
			g.Material++
		}
	}
	out := make([]SelfFavorGroup, 0, len(groups))
	for _, g := range groups {
		g.MeanSelf /= float64(g.N)
		g.MeanIndependent /= float64(g.N)
		g.Gap = g.MeanSelf - g.MeanIndependent
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Agent != out[j].Agent {
			return out[i].Agent < out[j].Agent
		}
		return out[i].Model < out[j].Model
	})
	return out
}

// ThirdOpinionSampled reports whether a material disagreement goes to
// management's third opinion: sampled by a hash of the Cast (so a replay
// samples the same ones), with a chance that rises with the record's weight
// from ThirdOpinionBaseRate to 1.
func ThirdOpinionSampled(castID string, weight float64) bool {
	if weight < 0 || math.IsNaN(weight) {
		weight = 0
	}
	if weight > 1 {
		weight = 1
	}
	p := ThirdOpinionBaseRate + (1-ThirdOpinionBaseRate)*weight
	sum := sha256.Sum256([]byte("third-opinion:" + castID))
	u := float64(uint32(sum[0])<<16|uint32(sum[1])<<8|uint32(sum[2])) / float64(1<<24)
	return u < p
}

// Research-style rule settlements (plan 4.4, layer 1). They are pure: the
// caller supplies what resolves.

// SettleReportSchemaValid decides report_schema_valid: the Report is
// well-formed, with at least one non-empty claim or finding, and a stop
// reason.
func SettleReportSchemaValid(report *CommitmentReport) RuleSettlement {
	reason := ""
	switch {
	case report == nil:
		reason = "the child returned no Report"
	case strings.TrimSpace(report.StopReason) == "":
		reason = "the Report has no stop reason"
	case !reportHasContent(report):
		reason = "the Report has no claim or finding"
	}
	if reason != "" {
		return RuleSettlement{Answer: PrecommitAnswerNo, Excerpt: reason}
	}
	return RuleSettlement{Answer: PrecommitAnswerYes, Excerpt: "the Report is well-formed"}
}

func reportHasContent(r *CommitmentReport) bool {
	for _, c := range r.Claims {
		if strings.TrimSpace(c.Text) != "" {
			return true
		}
	}
	for _, f := range r.Findings {
		if strings.TrimSpace(f.Text) != "" {
			return true
		}
	}
	return false
}

// ReportCitations are the sources a Report cites, distinct and in order.
func ReportCitations(report *CommitmentReport) []string {
	if report == nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, c := range report.Claims {
		for _, s := range c.Sources {
			if s = strings.TrimSpace(s); s != "" && !seen[s] {
				seen[s] = true
				out = append(out, s)
			}
		}
	}
	return out
}

// SettleCitationsResolve decides citations_resolve: every source the Report
// cites resolves to a capture or evidence record. A Report that cites nothing
// passes (it asserts no source); the rule covers only the facts it can check.
func SettleCitationsResolve(report *CommitmentReport, resolves func(ref string) bool) RuleSettlement {
	refs := ReportCitations(report)
	var missing []string
	for _, ref := range refs {
		if !resolves(ref) {
			missing = append(missing, ref)
		}
	}
	if len(missing) > 0 {
		return RuleSettlement{Answer: PrecommitAnswerNo, EvidenceRefs: missing,
			Excerpt: fmt.Sprintf("%d of %d cited sources do not resolve to a capture or evidence record", len(missing), len(refs))}
	}
	return RuleSettlement{Answer: PrecommitAnswerYes, EvidenceRefs: refs,
		Excerpt: fmt.Sprintf("all %d cited sources resolve", len(refs))}
}
