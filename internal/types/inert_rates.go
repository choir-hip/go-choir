package types

import (
	"fmt"
	"sort"
	"strings"
)

// The inert rates and the judge's agreement with the rules (MA1.6, plan
// 4.4): measured, never enforced. Nothing is refused for being inert.

// InertJudgement is the body of an inert_judgement record: the blind judge's
// answer to "could any plausible outcome have contradicted this prediction?"
// for one sampled request record, with the rule flags the record carried
// beside it, so the judge is scored against the rules (score the scorer).
type InertJudgement struct {
	// RecordID is the request record that was sampled.
	RecordID string `json:"record_id"`
	// Flagged says the rules had flagged a prediction fault on it.
	Flagged   bool     `json:"flagged"`
	RuleFlags []string `json:"rule_flags,omitempty"`
	// Contradictable is the judge's probability that a plausible outcome
	// could have contradicted the prediction (yes); JudgeInert is true when
	// the judge's answer was no (nothing plausible could).
	Contradictable float64 `json:"contradictable"`
	JudgeInert     bool    `json:"judge_inert"`
	// Agree is JudgeInert == Flagged.
	Agree bool `json:"agree"`
}

// InertJudgementRecordID is the judgement of one request record.
func InertJudgementRecordID(recordID string) string { return recordID + ":inertjudge" }

// InertJudgeAgreement is the judge's agreement with the rules over its
// judgements: a confusion matrix with the rules as the reference. The
// unflagged sample can only show missed inertness (FalseNegatives); the
// flagged control sample shows the rules' false positives.
type InertJudgeAgreement struct {
	Sampled int `json:"sampled"`
	Agree   int `json:"agree"`
	// BothInert: rules flagged and judge said inert. BothInformative:
	// neither did. RulesOnly: rules flagged, judge said a plausible outcome
	// could contradict (a possible false positive of the rules).
	// JudgeOnly: the judge said inert and the rules had not flagged (a
	// possible gap in the rules).
	BothInert       int     `json:"both_inert"`
	BothInformative int     `json:"both_informative"`
	RulesOnly       int     `json:"rules_only"`
	JudgeOnly       int     `json:"judge_only"`
	AgreementRate   float64 `json:"agreement_rate"`
}

// InertJudgeAgreementOf scores the judgement records against the rules.
func InertJudgeAgreementOf(judgements []CommitmentRecord) InertJudgeAgreement {
	var a InertJudgeAgreement
	for _, rec := range judgements {
		if rec.SubCall == nil || rec.SubCall.Kind != SubCallInertJudgement || rec.SubCall.Inert == nil {
			continue
		}
		j := rec.SubCall.Inert
		a.Sampled++
		switch {
		case j.Flagged && j.JudgeInert:
			a.BothInert++
		case !j.Flagged && !j.JudgeInert:
			a.BothInformative++
		case j.Flagged:
			a.RulesOnly++
		default:
			a.JudgeOnly++
		}
	}
	a.Agree = a.BothInert + a.BothInformative
	if a.Sampled > 0 {
		a.AgreementRate = float64(a.Agree) / float64(a.Sampled)
	}
	return a
}

// InertRateRow is one group's inert rates. Prompt version is the host's
// stamp on the record ("" for records written before it existed).
type InertRateRow struct {
	Desk          string `json:"desk"`
	Model         string `json:"model"`
	PromptVersion string `json:"prompt_version"`
	// Precommitments are the requests that state a prediction; Inert are
	// those with a prediction fault (an inert precommitment, the fault is in
	// the prediction).
	Precommitments      int     `json:"precommitments"`
	InertPrecommitments int     `json:"inert_precommitments"`
	InertPrecommitRate  float64 `json:"inert_precommit_rate"`
	// Delegations are the Casts; Inert are those with a weight below the
	// floor of their callee's grants (an inert delegation, the fault is in
	// the fan-out). Whether the child's report was cited downstream is not
	// yet counted: the record has no downstream-citation edge.
	Delegations       int            `json:"delegations"`
	InertDelegations  int            `json:"inert_delegations"`
	InertDelegateRate float64        `json:"inert_delegation_rate"`
	ByFlag            map[string]int `json:"by_flag,omitempty"`
}

// InertRates groups the request records of one owner's computer by desk,
// model and prompt version and returns the inert-precommitment and
// inert-delegation rates. The flags are recomputed from each record's own
// content (so records written before the slice are measured too); baseRates
// nil takes the default table. The input must be one owner's records: the
// function holds no owner, and the callers list per owner and computer.
func InertRates(records []CommitmentRecord, baseRates map[string]float64) []InertRateRow {
	type key struct{ desk, model, prompt string }
	groups := map[key]*InertRateRow{}
	for _, rec := range records {
		kind := InertKind(rec)
		if kind == "" {
			continue
		}
		flags := DetectInertness("", rec, baseRates)
		isCast := kind == RequestCast
		states := statesPrediction(rec)
		if !states && !isCast {
			continue
		}
		k := key{InertDesk(rec), strings.TrimSpace(rec.Provenance.ModelID), strings.TrimSpace(rec.Provenance.PromptVersion)}
		g := groups[k]
		if g == nil {
			g = &InertRateRow{Desk: k.desk, Model: k.model, PromptVersion: k.prompt, ByFlag: map[string]int{}}
			groups[k] = g
		}
		if states {
			g.Precommitments++
			if HasPredictionFault(flags) {
				g.InertPrecommitments++
			}
		}
		if isCast {
			g.Delegations++
			if HasFlag(flags, InertWeightBelowFloor) {
				g.InertDelegations++
			}
		}
		for _, f := range flags {
			g.ByFlag[f]++
		}
	}
	out := make([]InertRateRow, 0, len(groups))
	for _, g := range groups {
		if g.Precommitments > 0 {
			g.InertPrecommitRate = float64(g.InertPrecommitments) / float64(g.Precommitments)
		}
		if g.Delegations > 0 {
			g.InertDelegateRate = float64(g.InertDelegations) / float64(g.Delegations)
		}
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Desk != b.Desk {
			return a.Desk < b.Desk
		}
		if a.Model != b.Model {
			return a.Model < b.Model
		}
		return a.PromptVersion < b.PromptVersion
	})
	return out
}

// InertPredictionText is what the blind judge is shown: the prediction and
// its stated probabilities, and nothing about who wrote it (no desk, model,
// agent or weight).
func InertPredictionText(rec CommitmentRecord) string {
	var b strings.Builder
	hyp := strings.TrimSpace(rec.Prediction.Hypothesis)
	if hyp == "" && rec.Directive != nil {
		hyp = strings.TrimSpace(firstNonEmptyString(rec.Directive.Objective, rec.Directive.Body))
	}
	if hyp != "" {
		fmt.Fprintf(&b, "Prediction: %s\n", hyp)
	}
	if rec.PSuccess != nil {
		fmt.Fprintf(&b, "Stated chance that the request achieves its outcome: %.2f\n", *rec.PSuccess)
	}
	for _, q := range rec.Prediction.Questions {
		fmt.Fprintf(&b, "Question: %s\n", q.Question)
		answers := q.Answers
		if len(answers) == 0 {
			for a := range q.Probabilities {
				answers = append(answers, a)
			}
			sort.Strings(answers)
		}
		for _, a := range answers {
			if p, ok := q.Probabilities[a]; ok {
				fmt.Fprintf(&b, "  answer %q stated at %.2f\n", a, p)
			} else {
				fmt.Fprintf(&b, "  answer %q\n", a)
			}
		}
	}
	if rec.Cast != nil {
		for _, h := range rec.Cast.Hypotheses {
			fmt.Fprintf(&b, "Hypothesis: %s (criterion: %s)\n", h.Statement, h.Criterion)
		}
	}
	return strings.TrimSpace(b.String())
}
