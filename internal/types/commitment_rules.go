package types

import (
	"fmt"
	"math"
	"path"
	"sort"
	"strings"
	"time"
)

// Rule-settled precommitments: engineering's say–do record (2026-10-14).
//
// When engineering takes an assignment it records what it expects with
// choir.Precommit, one yes/no question per claim, each carrying a closed rule
// kind. The runtime settles each one from receipts it already holds (the
// capsule's execution receipts and changed paths at Freeze, management's
// Verify, the assignment's terminal disposition) — rules settle, models do
// not. A settlement is an ordinary resolve record (kind resolve, linked by
// ParentID/RelatedIDs) written by the rule scorer "system:rule"; the Brier
// score is a derived view (RulePrecommitSayDo), never stored, as the memo's
// "scores are views" requires.
//
// Failure modes, written before the code (CLAUDE.md "Testing"):
//
//	R1  A model-supplied assignment id or clock anchor lets a desk settle a
//	    claim against another assignment, or start the freeze clock late.
//	    -> the reducer stamps AssignmentID and Since from the bound
//	       assignment; model values are overwritten.
//	R2  A precommit made after the evidence exists is a free prediction.
//	    -> refused once the assignment's candidate is frozen or the
//	       assignment is terminal, and refused in a cell that also stages
//	       Freeze or Complete. Since anchors freeze_within at the
//	       assignment's opening, so a late precommit buys no time.
//	R3  A dogmatic probability (0 or 1) or a distribution over other answers
//	    cannot be recalibrated or scored as yes/no.
//	    -> ValidateRulePrecommit: answers exactly yes/no, sum 1, p(yes) in
//	       [0.02, 0.98].
//	R4  A claim settled twice, or by two different rule passes.
//	    -> one deterministic resolve id per precommit (RuleResolveRecordID),
//	       written with a not-exists condition: the first settlement wins.
//	R5  A crash between a freeze and its settlement makes the terminal pass
//	    record "no freeze" for a candidate that did freeze.
//	    -> the terminal pass sees the frozen candidate and records
//	       unresolved, never a false no.
//	R6  Settlement failure blocks the freeze, verify or terminal path.
//	    -> callers treat settlement as best effort and log; it never fails
//	       the act it observes.
//	R7  tests_pass reads cell exit codes. A cell that runs a failing test
//	    and does not fail itself reads as passing.
//	    -> named edge (missing_oracle): the prompt tells the desk to fail
//	       the cell when the test command fails; per-command receipts are
//	       the observer upgrade.
//	R8  Naive prefix matching lets "front" confine "frontend/x".
//	    -> paths match on whole segments (WithinPathPrefix).
//	R9  Build outputs (the release tree, caches) count as source changes.
//	    -> only files under workspace/platform/ count; directories are
//	       ignored (CapsuleSourcePaths in agentcore).
//	R10 verify_pass is silently scored "no" when management simply has not
//	    verified yet.
//	    -> it stays open while a completed candidate awaits Verify.
//	R11 Unresolved outcomes inflate or hide error.
//	    -> the view reports resolved-only Brier and strict Brier
//	       (unresolved scored as o = 0) side by side, as E6a did.
//	R12 Scores leak into the acting desk's context (the 2026-09-29 rule).
//	    -> the resolve record carries a discrepancy and an observation,
//	       which the acting pack already shows; the Brier view is a
//	       supervision view only.

// PrecommitRuleKind is the closed vocabulary of rule-settled engineering
// claims. Adding a kind is a reviewed change: the rules are the ground truth
// other scorers are measured against.
type PrecommitRuleKind string

const (
	// PrecommitRuleTestsPass: every test receipt the assignment's successful
	// Freeze cites exited 0.
	PrecommitRuleTestsPass PrecommitRuleKind = "tests_pass"
	// PrecommitRuleFreezeWithin: a Freeze succeeds within WithinMinutes of
	// the assignment opening.
	PrecommitRuleFreezeWithin PrecommitRuleKind = "freeze_within"
	// PrecommitRulePathsConfined: the frozen candidate changes only files
	// under Paths (relative to /workspace/platform).
	PrecommitRulePathsConfined PrecommitRuleKind = "paths_confined"
	// PrecommitRuleVerifyPass: management's Verify passes the candidate.
	PrecommitRuleVerifyPass PrecommitRuleKind = "verify_pass"
	// PrecommitRuleCompletes: the assignment ends completed.
	PrecommitRuleCompletes PrecommitRuleKind = "completes"
)

// PrecommitRuleKinds lists the vocabulary in a stable order.
var PrecommitRuleKinds = []PrecommitRuleKind{
	PrecommitRuleTestsPass, PrecommitRuleFreezeWithin, PrecommitRulePathsConfined,
	PrecommitRuleVerifyPass, PrecommitRuleCompletes,
}

const (
	PrecommitAnswerYes        = "yes"
	PrecommitAnswerNo         = "no"
	PrecommitAnswerUnresolved = "unresolved"

	// PrecommitRuleResolver is the resolver a rule precommit names.
	PrecommitRuleResolver = "rule"
	// RuleScorerAgentID is the provenance agent of a rule settlement.
	RuleScorerAgentID = "system:rule"
	// RuleScorerPrefix prefixes the scorer id on a rule settlement's score
	// ("rule:tests_pass").
	RuleScorerPrefix = "rule:"

	PrecommitRuleMinP             = 0.02
	PrecommitRuleMaxP             = 0.98
	PrecommitRuleMaxWithinMinutes = 24 * 60
	PrecommitRuleMaxPaths         = 32
)

// PrecommitRule is the typed settlement rule a rule precommit carries.
// AssignmentID and Since are runtime-stamped (R1).
type PrecommitRule struct {
	Kind          PrecommitRuleKind `json:"kind"`
	WithinMinutes int               `json:"within_minutes,omitempty"`
	Paths         []string          `json:"paths,omitempty"`
	AssignmentID  string            `json:"assignment_id,omitempty"`
	Since         string            `json:"since,omitempty"`
}

// ValidateRulePrecommit checks a rule precommit as staged (R3).
func ValidateRulePrecommit(p CommitmentPrecommit) error {
	if p.Rule == nil {
		return fmt.Errorf("precommit: a rule is required (kind one of %s)", ruleKindList())
	}
	if strings.TrimSpace(p.Question) == "" {
		return fmt.Errorf("precommit: a question is required")
	}
	if strings.TrimSpace(p.Resolver) != PrecommitRuleResolver {
		return fmt.Errorf("precommit: a rule precommit names resolver %q, not %q", PrecommitRuleResolver, p.Resolver)
	}
	if len(p.Distribution) != 2 {
		return fmt.Errorf("precommit: distribution must be exactly {\"yes\": p, \"no\": 1-p}")
	}
	yes, okYes := p.Distribution[PrecommitAnswerYes]
	no, okNo := p.Distribution[PrecommitAnswerNo]
	if !okYes || !okNo {
		return fmt.Errorf("precommit: distribution must be exactly {\"yes\": p, \"no\": 1-p}")
	}
	if math.IsNaN(yes) || math.IsNaN(no) || math.Abs(yes+no-1) > 1e-6 {
		return fmt.Errorf("precommit: yes and no must sum to 1 (got %v + %v)", yes, no)
	}
	if yes < PrecommitRuleMinP || yes > PrecommitRuleMaxP {
		return fmt.Errorf("precommit: p(yes) must be in [%.2f, %.2f]; 0 and 1 cannot be recalibrated (got %v)", PrecommitRuleMinP, PrecommitRuleMaxP, yes)
	}
	return p.Rule.validate()
}

func (r PrecommitRule) validate() error {
	switch r.Kind {
	case PrecommitRuleFreezeWithin:
		if r.WithinMinutes < 1 || r.WithinMinutes > PrecommitRuleMaxWithinMinutes {
			return fmt.Errorf("precommit rule freeze_within: within_minutes must be 1..%d", PrecommitRuleMaxWithinMinutes)
		}
		if len(r.Paths) != 0 {
			return fmt.Errorf("precommit rule freeze_within takes no paths")
		}
	case PrecommitRulePathsConfined:
		if r.WithinMinutes != 0 {
			return fmt.Errorf("precommit rule paths_confined takes no within_minutes")
		}
		if len(r.Paths) == 0 || len(r.Paths) > PrecommitRuleMaxPaths {
			return fmt.Errorf("precommit rule paths_confined: 1..%d paths relative to /workspace/platform", PrecommitRuleMaxPaths)
		}
		for _, p := range r.Paths {
			if _, err := NormalizeRulePath(p); err != nil {
				return err
			}
		}
	case PrecommitRuleTestsPass, PrecommitRuleVerifyPass, PrecommitRuleCompletes:
		if r.WithinMinutes != 0 || len(r.Paths) != 0 {
			return fmt.Errorf("precommit rule %s takes no within_minutes or paths", r.Kind)
		}
	default:
		return fmt.Errorf("precommit: unknown rule kind %q (one of %s)", r.Kind, ruleKindList())
	}
	return nil
}

func ruleKindList() string {
	names := make([]string, len(PrecommitRuleKinds))
	for i, k := range PrecommitRuleKinds {
		names[i] = string(k)
	}
	return strings.Join(names, ", ")
}

// NormalizeRulePath turns a paths_confined entry into a clean relative path
// without a trailing slash. "/workspace/platform/" is accepted as a prefix
// and stripped; ".." and an empty path are refused.
func NormalizeRulePath(raw string) (string, error) {
	p := strings.TrimSpace(raw)
	p = strings.TrimPrefix(p, "/workspace/platform")
	p = strings.TrimPrefix(p, "/")
	if p == "" {
		return "", fmt.Errorf("precommit rule paths_confined: empty path %q (name directories or files under /workspace/platform)", raw)
	}
	clean := path.Clean(p)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("precommit rule paths_confined: path %q leaves /workspace/platform", raw)
	}
	return clean, nil
}

// WithinPathPrefix reports whether file lies at or under prefix, on whole
// path segments (R8).
func WithinPathPrefix(file, prefix string) bool {
	return file == prefix || strings.HasPrefix(file, prefix+"/")
}

// YesProbability returns the committed p(yes).
func (p CommitmentPrecommit) YesProbability() float64 {
	return p.Distribution[PrecommitAnswerYes]
}

// EngineeringReceiptExit is one cited execution receipt and its exit code.
type EngineeringReceiptExit struct {
	Ref      string
	ExitCode int
}

// EngineeringFreezeObservation is what the runtime saw at one successful
// Freeze: when, which bundle, the cited test receipts' exit codes and the
// changed source files. SourcePathsKnown is false when the paths were not
// observed (a replayed freeze), so paths_confined does not settle.
type EngineeringFreezeObservation struct {
	FrozenAt         time.Time
	BundleDigest     string
	OperationID      string
	TestReceipts     []EngineeringReceiptExit
	SourcePaths      []string
	SourcePathsKnown bool
}

// EngineeringVerifyObservation is management's committed Verify.
type EngineeringVerifyObservation struct {
	Decision     string
	BundleDigest string
	VerifierRefs []string
}

// EngineeringTerminalObservation is the assignment's terminal disposition.
// CandidateFrozen says a Freeze produced a bundle before the end.
type EngineeringTerminalObservation struct {
	Disposition     EngineeringAssignmentDisposition
	Reason          string
	At              time.Time
	CandidateFrozen bool
	CandidateRef    string
}

// RuleSettlement is one rule's answer with the observation and evidence it
// rests on.
type RuleSettlement struct {
	Answer       string
	Excerpt      string
	EvidenceRefs []string
}

// SettleRuleOnFreeze decides the claims a successful Freeze settles:
// freeze_within, tests_pass and (when the paths were observed)
// paths_confined. ok is false for claims this observation does not decide.
func SettleRuleOnFreeze(rule PrecommitRule, obs EngineeringFreezeObservation) (RuleSettlement, bool) {
	refs := nonEmpty("bundle:"+strings.TrimSpace(obs.BundleDigest), "selfdev-operation:"+strings.TrimSpace(obs.OperationID))
	switch rule.Kind {
	case PrecommitRuleFreezeWithin:
		since, ok := parseRuleTime(rule.Since)
		if !ok {
			return RuleSettlement{Answer: PrecommitAnswerUnresolved, Excerpt: "the precommit carries no assignment-opening anchor", EvidenceRefs: refs}, true
		}
		elapsed := obs.FrozenAt.Sub(since)
		bound := time.Duration(rule.WithinMinutes) * time.Minute
		if elapsed <= bound {
			return RuleSettlement{Answer: PrecommitAnswerYes, Excerpt: fmt.Sprintf("froze %s after the assignment opened, within the %dm bound", roundMinutes(elapsed), rule.WithinMinutes), EvidenceRefs: refs}, true
		}
		return RuleSettlement{Answer: PrecommitAnswerNo, Excerpt: fmt.Sprintf("froze %s after the assignment opened, past the %dm bound", roundMinutes(elapsed), rule.WithinMinutes), EvidenceRefs: refs}, true
	case PrecommitRuleTestsPass:
		testRefs := make([]string, 0, len(obs.TestReceipts))
		var failing []string
		for _, r := range obs.TestReceipts {
			testRefs = append(testRefs, r.Ref)
			if r.ExitCode != 0 {
				failing = append(failing, fmt.Sprintf("%s exited %d", r.Ref, r.ExitCode))
			}
		}
		refs = append(refs, testRefs...)
		switch {
		case len(obs.TestReceipts) == 0:
			return RuleSettlement{Answer: PrecommitAnswerNo, Excerpt: "the Freeze cited no test receipt", EvidenceRefs: refs}, true
		case len(failing) == 0:
			return RuleSettlement{Answer: PrecommitAnswerYes, Excerpt: fmt.Sprintf("all %d test receipts the Freeze cites exited 0", len(obs.TestReceipts)), EvidenceRefs: refs}, true
		default:
			return RuleSettlement{Answer: PrecommitAnswerNo, Excerpt: fmt.Sprintf("%d of %d test receipts exited nonzero: %s", len(failing), len(obs.TestReceipts), strings.Join(failing, "; ")), EvidenceRefs: refs}, true
		}
	case PrecommitRulePathsConfined:
		if !obs.SourcePathsKnown {
			return RuleSettlement{}, false
		}
		var outside []string
		for _, file := range obs.SourcePaths {
			if !withinAnyPrefix(file, rule.Paths) {
				outside = append(outside, file)
			}
		}
		if len(outside) == 0 {
			return RuleSettlement{Answer: PrecommitAnswerYes, Excerpt: fmt.Sprintf("all %d changed files lie under %s", len(obs.SourcePaths), strings.Join(rule.Paths, ", ")), EvidenceRefs: refs}, true
		}
		shown := outside
		if len(shown) > 5 {
			shown = shown[:5]
		}
		return RuleSettlement{Answer: PrecommitAnswerNo, Excerpt: fmt.Sprintf("%d of %d changed files lie outside %s: %s", len(outside), len(obs.SourcePaths), strings.Join(rule.Paths, ", "), strings.Join(shown, ", ")), EvidenceRefs: refs}, true
	}
	return RuleSettlement{}, false
}

// SettleRuleOnVerify decides verify_pass from management's Verify.
func SettleRuleOnVerify(rule PrecommitRule, obs EngineeringVerifyObservation) (RuleSettlement, bool) {
	if rule.Kind != PrecommitRuleVerifyPass {
		return RuleSettlement{}, false
	}
	refs := append(nonEmpty("bundle:"+strings.TrimSpace(obs.BundleDigest)), obs.VerifierRefs...)
	switch strings.TrimSpace(obs.Decision) {
	case "pass":
		return RuleSettlement{Answer: PrecommitAnswerYes, Excerpt: "management's Verify passed the candidate", EvidenceRefs: refs}, true
	case "fail":
		return RuleSettlement{Answer: PrecommitAnswerNo, Excerpt: "management's Verify failed the candidate", EvidenceRefs: refs}, true
	}
	return RuleSettlement{}, false
}

// SettleRuleAtTerminal decides whatever is still open when the assignment
// ends. A claim the terminal state cannot decide returns ok false: a
// completed candidate's verify_pass waits for Verify (R10).
func SettleRuleAtTerminal(rule PrecommitRule, obs EngineeringTerminalObservation) (RuleSettlement, bool) {
	ended := fmt.Sprintf("the assignment ended %s", obs.Disposition)
	if reason := strings.TrimSpace(obs.Reason); reason != "" {
		ended += " (" + truncateRunes(reason, 160) + ")"
	}
	refs := nonEmpty("assignment:"+strings.TrimSpace(rule.AssignmentID), strings.TrimSpace(obs.CandidateRef))
	switch rule.Kind {
	case PrecommitRuleCompletes:
		if obs.Disposition == EngineeringAssignmentCompleted {
			return RuleSettlement{Answer: PrecommitAnswerYes, Excerpt: ended, EvidenceRefs: refs}, true
		}
		return RuleSettlement{Answer: PrecommitAnswerNo, Excerpt: ended, EvidenceRefs: refs}, true
	case PrecommitRuleFreezeWithin, PrecommitRuleTestsPass, PrecommitRulePathsConfined:
		if obs.CandidateFrozen {
			// R5: the freeze happened but its settlement is missing.
			return RuleSettlement{Answer: PrecommitAnswerUnresolved, Excerpt: "a candidate froze but its freeze observation was not recorded; " + ended, EvidenceRefs: refs}, true
		}
		if rule.Kind == PrecommitRulePathsConfined {
			return RuleSettlement{Answer: PrecommitAnswerUnresolved, Excerpt: "no candidate froze, so no change to confine; " + ended, EvidenceRefs: refs}, true
		}
		excerpt := "no Freeze succeeded before " + ended
		if rule.Kind == PrecommitRuleFreezeWithin {
			if since, ok := parseRuleTime(rule.Since); ok && obs.At.Sub(since) > time.Duration(rule.WithinMinutes)*time.Minute {
				excerpt = fmt.Sprintf("timeout: no Freeze within %dm; %s", rule.WithinMinutes, ended)
			}
		}
		return RuleSettlement{Answer: PrecommitAnswerNo, Excerpt: excerpt, EvidenceRefs: refs}, true
	case PrecommitRuleVerifyPass:
		if obs.CandidateFrozen && obs.Disposition == EngineeringAssignmentCompleted {
			return RuleSettlement{}, false
		}
		return RuleSettlement{Answer: PrecommitAnswerNo, Excerpt: "no completed candidate to verify; " + ended, EvidenceRefs: refs}, true
	}
	return RuleSettlement{}, false
}

// RuleResolveRecordID is the one settlement id per precommit (R4).
func RuleResolveRecordID(precommitID string) string {
	return strings.TrimSpace(precommitID) + ":resolve:rule"
}

// NewRuleResolveRecord builds the rule scorer's resolve record for one
// precommit. Discrepancy is confirmed when the answer matches the side the
// precommit put at least 0.5 on, contradicted otherwise, unresolved when the
// rule could not decide.
func NewRuleResolveRecord(precommit CommitmentRecord, s RuleSettlement, at time.Time) CommitmentRecord {
	stamp := at.UTC().Format(time.RFC3339Nano)
	kind := ""
	question := strings.TrimSpace(precommit.Prediction.Hypothesis)
	discrepancy := DiscrepancyUnresolved
	if precommit.Precommit != nil {
		if precommit.Precommit.Rule != nil {
			kind = string(precommit.Precommit.Rule.Kind)
		}
		question = strings.TrimSpace(precommit.Precommit.Question)
		predictsYes := precommit.Precommit.YesProbability() >= 0.5
		switch s.Answer {
		case PrecommitAnswerYes, PrecommitAnswerNo:
			if predictsYes == (s.Answer == PrecommitAnswerYes) {
				discrepancy = DiscrepancyConfirmed
			} else {
				discrepancy = DiscrepancyContradicted
			}
		}
	}
	sourceRef := precommit.RecordID
	if len(s.EvidenceRefs) > 0 {
		sourceRef = s.EvidenceRefs[0]
	}
	return CommitmentRecord{
		SchemaID:    CommitmentRecordSchemaV1,
		RecordID:    RuleResolveRecordID(precommit.RecordID),
		Kind:        CommitmentKindResolve,
		Discrepancy: discrepancy,
		Resolve:     &CommitmentResolve{Verdict: s.Answer, EvidenceRefs: append([]string(nil), s.EvidenceRefs...)},
		Observation: CommitmentObservation{Excerpt: s.Excerpt, SourceRef: sourceRef, ObservedAt: stamp},
		Scores: []CommitmentScore{{
			ScorerModelID: RuleScorerPrefix + kind,
			Answers:       map[string]string{question: s.Answer},
			ScoredAt:      stamp,
		}},
		Provenance: CommitmentProvenance{
			AgentID: RuleScorerAgentID, ContextRef: precommit.Provenance.ContextRef,
			CommittedAt: stamp, ResolvedAt: stamp,
		},
		ParentID:     precommit.RecordID,
		RelatedIDs:   []string{precommit.RecordID},
		EvidenceRefs: append([]string(nil), s.EvidenceRefs...),
		EmitterKind:  CommitmentEmitterSystem,
		ComputerID:   precommit.ComputerID,
		SubjectRefs:  precommit.SubjectRefs,
	}
}

// BrierBinary is the Brier score of p(yes) against the outcome.
func BrierBinary(p float64, yes bool) float64 {
	o := 0.0
	if yes {
		o = 1
	}
	return (p - o) * (p - o)
}

// SayDoBucket is one rule kind's (or all kinds') say–do tally. Brier is over
// resolved claims only; BrierStrict scores unresolved claims as o = 0, the
// claim not shown to hold (R11). Calibration is mean p against YesRate.
type SayDoBucket struct {
	Rule        string  `json:"rule"`
	Committed   int     `json:"committed"`
	Open        int     `json:"open"`
	Yes         int     `json:"yes"`
	No          int     `json:"no"`
	Unresolved  int     `json:"unresolved"`
	MeanP       float64 `json:"mean_p"`
	YesRate     float64 `json:"yes_rate"`
	Brier       float64 `json:"brier"`
	BrierStrict float64 `json:"brier_strict"`
}

// SayDoRecord is an agent family's rule-settled track record.
type SayDoRecord struct {
	AgentPrefix string        `json:"agent_prefix"`
	Overall     SayDoBucket   `json:"overall"`
	ByRule      []SayDoBucket `json:"by_rule"`
}

type sayDoAccumulator struct {
	bucket            SayDoBucket
	sumP, sumB, sumBS float64
	settled           int
}

func (a *sayDoAccumulator) add(p float64, answer string, settled bool) {
	a.bucket.Committed++
	a.sumP += p
	if !settled {
		a.bucket.Open++
		return
	}
	a.settled++
	switch answer {
	case PrecommitAnswerYes:
		a.bucket.Yes++
		a.sumB += BrierBinary(p, true)
		a.sumBS += BrierBinary(p, true)
	case PrecommitAnswerNo:
		a.bucket.No++
		a.sumB += BrierBinary(p, false)
		a.sumBS += BrierBinary(p, false)
	default:
		a.bucket.Unresolved++
		a.sumBS += BrierBinary(p, false)
	}
}

func (a *sayDoAccumulator) finish() SayDoBucket {
	b := a.bucket
	if b.Committed > 0 {
		b.MeanP = a.sumP / float64(b.Committed)
	}
	if resolved := b.Yes + b.No; resolved > 0 {
		b.Brier = a.sumB / float64(resolved)
		b.YesRate = float64(b.Yes) / float64(resolved)
	}
	if a.settled > 0 {
		b.BrierStrict = a.sumBS / float64(a.settled)
	}
	return b
}

// RulePrecommitSayDo derives the rule-settled track record of every agent
// whose id starts with agentPrefix ("engineering:" for the engineering
// desk's assignments). Only the rule scorer's settlement governs here;
// other scorers' resolutions are their own records (multi-scoring).
func RulePrecommitSayDo(records []CommitmentRecord, agentPrefix string) SayDoRecord {
	ruleAnswers := map[string]CommitmentRecord{}
	for _, rec := range records {
		if rec.Resolve == nil || rec.Provenance.AgentID != RuleScorerAgentID {
			continue
		}
		target := resolutionTarget(rec)
		if prior, seen := ruleAnswers[target]; !seen || resolutionChronological(prior, rec) {
			ruleAnswers[target] = rec
		}
	}
	overall := &sayDoAccumulator{bucket: SayDoBucket{Rule: "all"}}
	byRule := map[string]*sayDoAccumulator{}
	for _, rec := range records {
		if rec.Precommit == nil || rec.Precommit.Rule == nil || rec.Resolve != nil ||
			!strings.HasPrefix(rec.Provenance.AgentID, agentPrefix) {
			continue
		}
		kind := string(rec.Precommit.Rule.Kind)
		acc := byRule[kind]
		if acc == nil {
			acc = &sayDoAccumulator{bucket: SayDoBucket{Rule: kind}}
			byRule[kind] = acc
		}
		p := rec.Precommit.YesProbability()
		settlement, settled := ruleAnswers[rec.RecordID]
		answer := ""
		if settled {
			answer = settlement.Resolve.Verdict
		}
		acc.add(p, answer, settled)
		overall.add(p, answer, settled)
	}
	out := SayDoRecord{AgentPrefix: agentPrefix, Overall: overall.finish()}
	kinds := make([]string, 0, len(byRule))
	for k := range byRule {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	for _, k := range kinds {
		out.ByRule = append(out.ByRule, byRule[k].finish())
	}
	return out
}

func withinAnyPrefix(file string, prefixes []string) bool {
	for _, raw := range prefixes {
		prefix, err := NormalizeRulePath(raw)
		if err == nil && WithinPathPrefix(file, prefix) {
			return true
		}
	}
	return false
}

func parseRuleTime(raw string) (time.Time, bool) {
	return parseCommitmentTime(raw)
}

func roundMinutes(d time.Duration) string {
	return d.Round(time.Second).String()
}

func nonEmpty(values ...string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" || strings.HasSuffix(v, ":") {
			continue
		}
		out = append(out, v)
	}
	return out
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
