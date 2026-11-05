package types

import (
	"encoding/json"
	"fmt"
	"math"
	"path"
	"sort"
	"strconv"
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
//
// Round predictions (docs/problems/selfdev-rounds-record-no-predictions-
// 2026-11-04.md): management's Cast to engineering states verify_pass,
// judge_accepts and freeze_minutes for the round, before the capsule starts.
// Failure modes added with them:
//
//	R13 A desk quotes a huge window and is "calibrated" on clock time.
//	    -> freeze_minutes states a point estimate inside its interval; the
//	       view reports coverage beside the log ratio of estimate to actual
//	       and the interval's log width (sharpness), never coverage alone.
//	R14 A freeze_minutes claim stated for a re-Freeze (after a rejection
//	    reused the engineer) is settled by the Freeze before it.
//	    -> a Freeze earlier than the claim's anchor does not decide it.
//	R15 Clock time disagrees with the round's phase receipt.
//	    -> the anchor is the Cast record's committed_at and the end the
//	       fresh Freeze's commit time, both written on the resolve record
//	       (cast_at, freeze_at, actual_s), so a reader can check one
//	       against the other.
//	R16 A round that never froze reads as "no data" and drops out, so a
//	    desk that misses badly looks better than one that froze late.
//	    -> the terminal pass settles it "no" with the elapsed time marked
//	       censored (actual_s is a lower bound).
//	R20 A claim stated after the evidence is visible flatters the record
//	    (live8, 2026-11-04: "Freeze within 40 minutes" staged 21 s before
//	    the Freeze it predicted).
//	    -> every rule precommit records stated_at; a clock claim stated after
//	       the run's first build or shot job, or any claim after its
//	       candidate froze, is kept but marked late, and the digest and the
//	       say-do view leave it out; the late share is its own metric.
//	R17 judge_accepts and verify_pass are one event today: management's
//	    Verify is the independent judgment (recordSelfDevelopmentVerdict).
//	    -> both are kept as claim types: verify_pass reads the Verify
//	       decision, judge_accepts the operation reaching the owner's
//	       decision (awaiting_approval) or failing on the judgment; they
//	       part when the judge leaves Verify.

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
	// PrecommitRuleJudgeAccepts: the independent judgment accepts the
	// assignment's candidate, so it reaches the owner's decision (R17).
	PrecommitRuleJudgeAccepts PrecommitRuleKind = "judge_accepts"
	// PrecommitRuleFreezeMinutes: a Freeze lands between LowMinutes and
	// HighMinutes after the anchor (the Cast), with EstimateMinutes the
	// point estimate the log ratio scores (R13). p(yes) is the stated chance
	// of landing inside the interval.
	PrecommitRuleFreezeMinutes PrecommitRuleKind = "freeze_minutes"
	// PrecommitRuleOwnerApproves: the owner's act approves the candidate
	// (approve, or an apply that implies it); a reject, a failed judgment or
	// a round with no candidate is "no". The settlement names the decision
	// actor and whether the act was delegated (R18).
	PrecommitRuleOwnerApproves PrecommitRuleKind = "owner_approves"
	// PrecommitRuleApprovalMinutes: the approving act lands between
	// LowMinutes and HighMinutes after the Cast; any other end is "no",
	// censored (the clock twin of owner_approves).
	PrecommitRuleApprovalMinutes PrecommitRuleKind = "approval_minutes"
	// PrecommitRuleApprovedWithinRounds: the request (the trajectory the
	// Cast serves) reaches an approved candidate within WithinRounds
	// Freezes, counted from the Cast: this round's Freezes (a reused
	// engineer's re-Freezes are each one) and every later round's on the
	// same trajectory (R19).
	PrecommitRuleApprovedWithinRounds PrecommitRuleKind = "approved_within_rounds"
	// PrecommitRuleReportSchemaValid: the child's Report is well-formed, with
	// a claim or finding and a stop reason (MA1.5, research-style).
	PrecommitRuleReportSchemaValid PrecommitRuleKind = "report_schema_valid"
	// PrecommitRuleCitationsResolve: every source the Report cites resolves
	// to a capture or evidence record (MA1.5, research-style).
	PrecommitRuleCitationsResolve PrecommitRuleKind = "citations_resolve"
	// precommitRuleVisualCapturedRetired read the platform capture, which is
	// removed (docs/problems/selfdev-screens-are-not-files-2026-11-03.md). A
	// new one is refused; one already on a tape settles unresolved.
	precommitRuleVisualCapturedRetired PrecommitRuleKind = "visual_captured"
)

// retiredRuleSettlement closes a retired rule: it never waits.
func retiredRuleSettlement(rule PrecommitRule) RuleSettlement {
	return RuleSettlement{Answer: PrecommitAnswerUnresolved, Excerpt: "the " + string(rule.Kind) + " rule is retired: the platform capture it read was removed", EvidenceRefs: nonEmpty("assignment:" + strings.TrimSpace(rule.AssignmentID))}
}

// clock reports a kind with an estimate and an interval in minutes.
func (k PrecommitRuleKind) clock() bool {
	return k == PrecommitRuleFreezeMinutes || k == PrecommitRuleApprovalMinutes
}

// PrecommitRuleKinds lists the vocabulary in a stable order.
var PrecommitRuleKinds = []PrecommitRuleKind{
	PrecommitRuleTestsPass, PrecommitRuleFreezeWithin, PrecommitRulePathsConfined,
	PrecommitRuleVerifyPass, PrecommitRuleCompletes, PrecommitRuleJudgeAccepts,
	PrecommitRuleFreezeMinutes, PrecommitRuleOwnerApproves, PrecommitRuleApprovalMinutes,
	PrecommitRuleApprovedWithinRounds,
	PrecommitRuleReportSchemaValid, PrecommitRuleCitationsResolve,
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
	// PrecommitRuleMaxApprovalMinutes bounds approval_minutes: an owner's
	// decision may wait days.
	PrecommitRuleMaxApprovalMinutes = 14 * 24 * 60
	PrecommitRuleMaxWithinRounds    = 20
	PrecommitRuleMaxPaths           = 32
)

// PrecommitRule is the typed settlement rule a rule precommit carries.
// AssignmentID and Since are runtime-stamped (R1).
type PrecommitRule struct {
	Kind          PrecommitRuleKind `json:"kind"`
	WithinMinutes int               `json:"within_minutes,omitempty"`
	// EstimateMinutes, LowMinutes and HighMinutes are freeze_minutes'
	// point estimate and interval.
	EstimateMinutes int `json:"estimate_minutes,omitempty"`
	LowMinutes      int `json:"low_minutes,omitempty"`
	HighMinutes     int `json:"high_minutes,omitempty"`
	// WithinRounds is approved_within_rounds' bound, in Freezes.
	WithinRounds int      `json:"within_rounds,omitempty"`
	Paths        []string `json:"paths,omitempty"`
	// Routes is read only from a retired visual_captured record.
	Routes       []string `json:"routes,omitempty"`
	AssignmentID string   `json:"assignment_id,omitempty"`
	Since        string   `json:"since,omitempty"`
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
	if p.Rule.Kind == PrecommitRuleReportSchemaValid || p.Rule.Kind == PrecommitRuleCitationsResolve {
		// These settle a sub-call's Report (subcall.go), not an assignment: a
		// precommit naming one would wait forever.
		return fmt.Errorf("precommit: rule %s settles a child's Report and is not a precommit rule", p.Rule.Kind)
	}
	return p.Rule.validate()
}

func (r PrecommitRule) validate() error {
	if len(r.Routes) != 0 {
		return fmt.Errorf("precommit rule %s takes no routes", r.Kind)
	}
	interval := r.EstimateMinutes != 0 || r.LowMinutes != 0 || r.HighMinutes != 0
	if interval && !r.Kind.clock() {
		return fmt.Errorf("precommit rule %s takes no estimate_minutes, low_minutes or high_minutes", r.Kind)
	}
	if r.WithinRounds != 0 && r.Kind != PrecommitRuleApprovedWithinRounds {
		return fmt.Errorf("precommit rule %s takes no within_rounds", r.Kind)
	}
	switch r.Kind {
	case PrecommitRuleFreezeMinutes, PrecommitRuleApprovalMinutes:
		if r.WithinMinutes != 0 || len(r.Paths) != 0 {
			return fmt.Errorf("precommit rule %s takes no within_minutes or paths", r.Kind)
		}
		bound := PrecommitRuleMaxWithinMinutes
		if r.Kind == PrecommitRuleApprovalMinutes {
			bound = PrecommitRuleMaxApprovalMinutes
		}
		if r.LowMinutes < 1 || r.LowMinutes > r.EstimateMinutes || r.EstimateMinutes > r.HighMinutes || r.HighMinutes > bound {
			return fmt.Errorf("precommit rule %s: whole minutes with 1 <= low <= estimate <= high <= %d (got low %d, estimate %d, high %d)", r.Kind, bound, r.LowMinutes, r.EstimateMinutes, r.HighMinutes)
		}
	case PrecommitRuleApprovedWithinRounds:
		if r.WithinMinutes != 0 || len(r.Paths) != 0 {
			return fmt.Errorf("precommit rule approved_within_rounds takes no within_minutes or paths")
		}
		if r.WithinRounds < 1 || r.WithinRounds > PrecommitRuleMaxWithinRounds {
			return fmt.Errorf("precommit rule approved_within_rounds: within_rounds must be 1..%d Freezes", PrecommitRuleMaxWithinRounds)
		}
	case PrecommitRuleOwnerApproves:
		if r.WithinMinutes != 0 || len(r.Paths) != 0 {
			return fmt.Errorf("precommit rule owner_approves takes no within_minutes or paths")
		}
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
	case PrecommitRuleTestsPass, PrecommitRuleVerifyPass, PrecommitRuleCompletes, PrecommitRuleJudgeAccepts,
		PrecommitRuleReportSchemaValid, PrecommitRuleCitationsResolve:
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
	// OperationID and OperationState are the operation after the verdict
	// (awaiting_approval or failed): judge_accepts reads the state.
	OperationID    string
	OperationState string
	// At is when the verdict landed (a failed judgment ends a clock claim).
	At time.Time
}

// EngineeringOwnerActObservation is an owner act on the assignment's
// candidate. DecisionAt is the decision's time: the approve act's record
// when there was one (an apply follows it), else the act itself, as the
// round receipt reads it. Delegated marks an act not made by the
// computer's owner in person (R18).
type EngineeringOwnerActObservation struct {
	Act         string
	OperationID string
	DecisionAt  time.Time
	Actor       string
	Delegated   bool
}

// SettleRuleOnOwnerAct decides owner_approves and approval_minutes from an
// owner act (R18).
func SettleRuleOnOwnerAct(rule PrecommitRule, obs EngineeringOwnerActObservation) (RuleSettlement, bool) {
	if rule.Kind != PrecommitRuleOwnerApproves && rule.Kind != PrecommitRuleApprovalMinutes {
		return RuleSettlement{}, false
	}
	refs := nonEmpty("selfdev-operation:"+strings.TrimSpace(obs.OperationID), "owner-act:"+strings.TrimSpace(obs.Act))
	actRefs := map[string]string{RuleRefDecisionActor: strings.TrimSpace(obs.Actor), RuleRefDelegated: strconv.FormatBool(obs.Delegated)}
	var s RuleSettlement
	switch obs.Act {
	case OwnerActApprove, OwnerActApply, OwnerActFail:
		// An apply, or a failed apply, follows an approval.
		if rule.Kind == PrecommitRuleApprovalMinutes {
			var ok bool
			if s, ok = settleClock(rule, obs.DecisionAt, "approved", refs); !ok {
				return s, false
			}
		} else {
			s = RuleSettlement{Answer: PrecommitAnswerYes, Excerpt: "the candidate was approved (" + obs.Act + ")", EvidenceRefs: refs}
		}
	case OwnerActReject, OwnerActAbandon:
		why := "the candidate was not approved (" + obs.Act + ")"
		if rule.Kind == PrecommitRuleApprovalMinutes {
			s = missedClock(rule, obs.DecisionAt, "approval", why, refs)
		} else {
			s = RuleSettlement{Answer: PrecommitAnswerNo, Excerpt: why, EvidenceRefs: refs}
		}
	default:
		return RuleSettlement{}, false
	}
	s.Refs = actRefs
	return s, true
}

// EngineeringRequestObservation is the request's state for
// approved_within_rounds (R19): the successful Freezes on its trajectory
// since the claim's Cast, whether one was approved, after how many Freezes,
// and whether the request was abandoned.
type EngineeringRequestObservation struct {
	At                  time.Time
	FreezesSince        int
	Approved            bool
	FreezesToApproval   int
	Abandoned           bool
	ApprovedOperationID string
}

// SettleRuleOnRequest decides approved_within_rounds (R19).
func SettleRuleOnRequest(rule PrecommitRule, obs EngineeringRequestObservation) (RuleSettlement, bool) {
	if rule.Kind != PrecommitRuleApprovedWithinRounds {
		return RuleSettlement{}, false
	}
	refs := nonEmpty("selfdev-operation:" + strings.TrimSpace(obs.ApprovedOperationID))
	switch {
	case obs.Approved && obs.FreezesToApproval <= rule.WithinRounds:
		return RuleSettlement{Answer: PrecommitAnswerYes, EvidenceRefs: refs, Refs: map[string]string{RuleRefFreezes: strconv.Itoa(obs.FreezesToApproval)},
			Excerpt: fmt.Sprintf("the request reached an approved candidate at Freeze %d of at most %d", obs.FreezesToApproval, rule.WithinRounds)}, true
	case obs.Approved:
		return RuleSettlement{Answer: PrecommitAnswerNo, EvidenceRefs: refs, Refs: map[string]string{RuleRefFreezes: strconv.Itoa(obs.FreezesToApproval)},
			Excerpt: fmt.Sprintf("the request reached an approved candidate only at Freeze %d, past %d", obs.FreezesToApproval, rule.WithinRounds)}, true
	case obs.FreezesSince > rule.WithinRounds:
		return RuleSettlement{Answer: PrecommitAnswerNo, Refs: map[string]string{RuleRefFreezes: strconv.Itoa(obs.FreezesSince)},
			Excerpt: fmt.Sprintf("%d Freezes without an approved candidate, past %d", obs.FreezesSince, rule.WithinRounds)}, true
	case obs.Abandoned:
		return RuleSettlement{Answer: PrecommitAnswerNo, Refs: map[string]string{RuleRefFreezes: strconv.Itoa(obs.FreezesSince)},
			Excerpt: fmt.Sprintf("the request was abandoned after %d Freezes without an approved candidate", obs.FreezesSince)}, true
	}
	return RuleSettlement{}, false
}

// EngineeringTerminalObservation is the assignment's terminal disposition.
// CandidateFrozen says a Freeze produced a bundle before the end.
type EngineeringTerminalObservation struct {
	Disposition     EngineeringAssignmentDisposition
	Reason          string
	At              time.Time
	CandidateFrozen bool
	CandidateRef    string
	// CandidateRetained says the round's frozen candidate was retained for
	// its Verify when the build's clock ended the round
	// (docs/problems/frozen-candidate-dies-with-build-deadline-2026-11-04.md),
	// so verify_pass stays open for that Verify.
	CandidateRetained bool
}

// RuleSettlement is one rule's answer with the observation and evidence it
// rests on.
type RuleSettlement struct {
	Answer       string
	Excerpt      string
	EvidenceRefs []string
	// Timing is set by a clock rule (freeze_minutes): the anchor, the end
	// and the elapsed seconds, written on the resolve record (R15, R16).
	Timing *RuleTiming
	// Refs join the resolve record's subject refs (the decision actor and
	// the delegated mark of an owner act, R18).
	Refs map[string]string
}

// RuleTiming is a clock rule's measured span. Censored marks a span that
// ended without the event (no Freeze): ActualS is then a lower bound.
type RuleTiming struct {
	Since    time.Time
	End      time.Time
	ActualS  int64
	Censored bool
}

// Subject-ref keys a clock rule's resolve record carries.
const (
	RuleRefCastAt   = "cast_at"
	RuleRefFreezeAt = "freeze_at"
	RuleRefEndAt    = "end_at"
	RuleRefActualS  = "actual_s"
	RuleRefCensored = "censored"
	// RuleRefDecisionAt names the decision a clock rule ended at.
	RuleRefDecisionAt = "decision_at"
	// RuleRefDecisionActor and RuleRefDelegated describe the act that
	// settled owner_approves (R18).
	RuleRefDecisionActor = "decision_actor"
	RuleRefDelegated     = "delegated"
	RuleRefFreezes       = "freezes"
	// RuleRefStatedAt is when a rule precommit was stated, RuleRefStatedBy
	// where ("cast" for a Cast's round predictions, "precommit" for a desk's
	// own), and RuleRefLate a claim stated after the work visibly progressed
	// (R20), with RuleRefLateReason why.
	RuleRefStatedAt   = "stated_at"
	RuleRefStatedBy   = "stated_by"
	RuleRefLate       = "late"
	RuleRefLateReason = "late_reason"
)

// ClockRule reports a claim about how long the work takes; any other rule
// is about its outcome. A clock claim is late once the first build or shot
// job ran, an outcome claim once the candidate froze (R20).
func ClockRule(k PrecommitRuleKind) bool {
	return k == PrecommitRuleFreezeWithin || k.clock()
}

// Calibration experiment E1 (docs/design/calibration-experiment-e1-2026-11-04.md):
// a restatement is a second statement of a claim already stated, recorded
// as its own rule precommit (stage p1) that names the first (p0) and is
// settled by the same event. It is no second claim: every score but the
// experiment's paired readout leaves it out (E1 F1).
const (
	RuleRefStage      = "calibration_stage"
	RuleRefRestates   = "restates"
	RuleStageFirst    = "p0"
	RuleStageRestated = "p1"
)

// RestatementRulePrecommit reports a restatement (stage p1).
func RestatementRulePrecommit(rec CommitmentRecord) bool {
	return rec.SubjectRefs[RuleRefStage] == RuleStageRestated
}

// LateRulePrecommit reports a precommit marked late (R20): kept on the
// tape, excluded from the digest and the scores.
func LateRulePrecommit(rec CommitmentRecord) bool {
	return rec.SubjectRefs[RuleRefLate] == "true"
}

// settleFreezeMinutes decides a freeze_minutes claim at a Freeze (R14).
func settleFreezeMinutes(rule PrecommitRule, frozenAt time.Time, refs []string) (RuleSettlement, bool) {
	return settleClock(rule, frozenAt, "froze", refs)
}

// settleClock decides a clock claim whose event happened at end (R13-R15).
func settleClock(rule PrecommitRule, end time.Time, verb string, refs []string) (RuleSettlement, bool) {
	since, ok := parseRuleTime(rule.Since)
	if !ok {
		return RuleSettlement{Answer: PrecommitAnswerUnresolved, Excerpt: "the precommit carries no Cast anchor", EvidenceRefs: refs}, true
	}
	if end.Before(since) {
		return RuleSettlement{}, false
	}
	elapsed := end.Sub(since)
	timing := &RuleTiming{Since: since, End: end, ActualS: int64(elapsed / time.Second)}
	minutes := elapsed.Minutes()
	answer, where := PrecommitAnswerYes, "inside"
	if minutes < float64(rule.LowMinutes) || minutes > float64(rule.HighMinutes) {
		answer, where = PrecommitAnswerNo, "outside"
	}
	return RuleSettlement{Answer: answer, EvidenceRefs: refs, Timing: timing,
		Excerpt: fmt.Sprintf("%s %s after the Cast, %s the %d-%dm interval (estimate %dm)", verb, roundMinutes(elapsed), where, rule.LowMinutes, rule.HighMinutes, rule.EstimateMinutes)}, true
}

// missedClock settles a clock claim whose event will not happen (R16, C7).
// Past the interval's high bound it is a miss: "no", the span censored.
// Before it, the round ended (a crash, an abandon, a reject, a platform
// failure) while the interval was still open, which says nothing about the
// clock: unresolved, the span kept and censored, so the claim is not scored
// (docs/design/calibration-wiring-permutations-2026-11-04.md, C7).
//
// Failure modes, written before the code (C7):
//   - a crash or abandon at minute 5 of a 15-45 interval scores as a
//     forecasting miss -> unresolved until the high bound has passed;
//   - a desk that never freezes escapes scoring by the same rule -> past the
//     high bound the miss stands, whatever ended the round;
//   - settlements written before this rule keep their "no" -> the readers
//     apply ClockCensoredUninformative to them as well.
func missedClock(rule PrecommitRule, at time.Time, what, why string, refs []string) RuleSettlement {
	since, ok := parseRuleTime(rule.Since)
	if !ok {
		return RuleSettlement{Answer: PrecommitAnswerUnresolved, Excerpt: "the precommit carries no Cast anchor; " + why, EvidenceRefs: refs}
	}
	elapsed := max(at.Sub(since), 0)
	timing := &RuleTiming{Since: since, End: at, ActualS: int64(elapsed / time.Second), Censored: true}
	if !clockIntervalClosed(rule, timing.ActualS) {
		return RuleSettlement{Answer: PrecommitAnswerUnresolved, EvidenceRefs: refs, Timing: timing,
			Excerpt: fmt.Sprintf("censored: the round ended %s after the Cast with no %s, before the %d-%dm interval closed, so the clock is unknown and the claim is not scored; %s", roundMinutes(elapsed), what, rule.LowMinutes, rule.HighMinutes, why)}
	}
	return RuleSettlement{Answer: PrecommitAnswerNo, EvidenceRefs: refs, Timing: timing,
		Excerpt: fmt.Sprintf("no %s within %s of the Cast (interval %d-%dm, estimate %dm); %s", what, roundMinutes(elapsed), rule.LowMinutes, rule.HighMinutes, rule.EstimateMinutes, why)}
}

// clockIntervalClosed reports a span past the interval's high bound.
func clockIntervalClosed(rule PrecommitRule, actualS int64) bool {
	return actualS > int64(rule.HighMinutes)*60
}

// ClockCensoredUninformative reports a clock claim's settlement that ended
// censored before the interval closed (C7): it tells nothing about the
// clock, and no score counts it, a settlement written before C7 included.
// refs are the resolve record's subject refs.
func ClockCensoredUninformative(rule PrecommitRule, refs map[string]string) bool {
	if !rule.Kind.clock() || refs[RuleRefCensored] != "true" {
		return false
	}
	actualS, err := strconv.ParseInt(refs[RuleRefActualS], 10, 64)
	if err != nil {
		return true
	}
	return !clockIntervalClosed(rule, actualS)
}

// SettleRuleOnFreeze decides the claims a successful Freeze settles:
// freeze_within, tests_pass and (when the paths were observed)
// paths_confined. ok is false for claims this observation does not decide.
func SettleRuleOnFreeze(rule PrecommitRule, obs EngineeringFreezeObservation) (RuleSettlement, bool) {
	refs := nonEmpty("bundle:"+strings.TrimSpace(obs.BundleDigest), "selfdev-operation:"+strings.TrimSpace(obs.OperationID))
	switch rule.Kind {
	case PrecommitRuleFreezeMinutes:
		return settleFreezeMinutes(rule, obs.FrozenAt, refs)
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
	case precommitRuleVisualCapturedRetired:
		return retiredRuleSettlement(rule), true
	}
	return RuleSettlement{}, false
}

// SettleRuleOnVerify decides verify_pass from management's Verify, and
// judge_accepts from where the judgment left the operation (R17).
func SettleRuleOnVerify(rule PrecommitRule, obs EngineeringVerifyObservation) (RuleSettlement, bool) {
	refs := append(nonEmpty("bundle:"+strings.TrimSpace(obs.BundleDigest), "selfdev-operation:"+strings.TrimSpace(obs.OperationID)), obs.VerifierRefs...)
	if rule.Kind == PrecommitRuleJudgeAccepts {
		switch strings.TrimSpace(obs.OperationState) {
		case "awaiting_approval":
			return RuleSettlement{Answer: PrecommitAnswerYes, Excerpt: "the independent judgment accepted the candidate; it awaits the owner's decision", EvidenceRefs: refs}, true
		case "failed":
			return RuleSettlement{Answer: PrecommitAnswerNo, Excerpt: "the independent judgment rejected the candidate", EvidenceRefs: refs}, true
		}
		return RuleSettlement{}, false
	}
	if rule.Kind == PrecommitRuleOwnerApproves || rule.Kind == PrecommitRuleApprovalMinutes {
		// A candidate the judgment rejected never reaches the owner.
		if strings.TrimSpace(obs.OperationState) != "failed" {
			return RuleSettlement{}, false
		}
		why := "the independent judgment rejected the candidate, so the owner never decided"
		if rule.Kind == PrecommitRuleApprovalMinutes {
			return missedClock(rule, obs.At, "approval", why, refs), true
		}
		return RuleSettlement{Answer: PrecommitAnswerNo, Excerpt: why, EvidenceRefs: refs}, true
	}
	if rule.Kind != PrecommitRuleVerifyPass {
		return RuleSettlement{}, false
	}
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
	case precommitRuleVisualCapturedRetired:
		return retiredRuleSettlement(rule), true
	case PrecommitRuleFreezeMinutes:
		if obs.CandidateFrozen {
			return RuleSettlement{Answer: PrecommitAnswerUnresolved, Excerpt: "a candidate froze but its freeze observation was not recorded; " + ended, EvidenceRefs: refs}, true
		}
		// R16: no Freeze is a miss, and the span is a lower bound.
		return missedClock(rule, obs.At, "Freeze", ended, refs), true
	case PrecommitRuleOwnerApproves, PrecommitRuleApprovalMinutes:
		if obs.CandidateFrozen && obs.Disposition == EngineeringAssignmentCompleted {
			return RuleSettlement{}, false // the judgment and the owner decide
		}
		if rule.Kind == PrecommitRuleApprovalMinutes {
			return missedClock(rule, obs.At, "approval", "no completed candidate to approve; "+ended, refs), true
		}
		return RuleSettlement{Answer: PrecommitAnswerNo, Excerpt: "no completed candidate to approve; " + ended, EvidenceRefs: refs}, true
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
	case PrecommitRuleVerifyPass, PrecommitRuleJudgeAccepts:
		if obs.CandidateFrozen && (obs.Disposition == EngineeringAssignmentCompleted || obs.CandidateRetained) {
			return RuleSettlement{}, false
		}
		return RuleSettlement{Answer: PrecommitAnswerNo, Excerpt: "no completed candidate to judge; " + ended, EvidenceRefs: refs}, true
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
	subjectRefs := precommit.SubjectRefs
	if s.Timing != nil || len(s.Refs) > 0 {
		subjectRefs = make(map[string]string, len(precommit.SubjectRefs)+len(s.Refs)+4)
		for k, v := range precommit.SubjectRefs {
			subjectRefs[k] = v
		}
		for k, v := range s.Refs {
			subjectRefs[k] = v
		}
	}
	if t := s.Timing; t != nil {
		subjectRefs[RuleRefCastAt] = t.Since.UTC().Format(time.RFC3339Nano)
		subjectRefs[RuleRefActualS] = strconv.FormatInt(t.ActualS, 10)
		if t.Censored {
			subjectRefs[RuleRefEndAt] = t.End.UTC().Format(time.RFC3339Nano)
			subjectRefs[RuleRefCensored] = "true"
		} else if kind == string(PrecommitRuleApprovalMinutes) {
			subjectRefs[RuleRefDecisionAt] = t.End.UTC().Format(time.RFC3339Nano)
		} else {
			subjectRefs[RuleRefFreezeAt] = t.End.UTC().Format(time.RFC3339Nano)
		}
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
		SubjectRefs:  subjectRefs,
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
	// Clock rules (freeze_minutes, R13): YesRate is interval coverage
	// against MeanP; Timed counts the settlements with a measured span
	// (censored ones excluded); MeanLogRatio is ln(estimate/actual) (above
	// 0 overestimates the time), MeanAbsLogRatio its size, and MeanLogWidth
	// ln(high/low) over every committed claim, the sharpness.
	Timed           int     `json:"timed,omitempty"`
	MeanLogRatio    float64 `json:"mean_log_ratio,omitempty"`
	MeanAbsLogRatio float64 `json:"mean_abs_log_ratio,omitempty"`
	MeanLogWidth    float64 `json:"mean_log_width,omitempty"`
}

// SayDoRecord is an agent family's rule-settled track record.
type SayDoRecord struct {
	AgentPrefix string `json:"agent_prefix"`
	// Late counts the claims marked late, left out of every bucket (R20).
	Late    int           `json:"late,omitempty"`
	Overall SayDoBucket   `json:"overall"`
	ByRule  []SayDoBucket `json:"by_rule"`
}

type sayDoAccumulator struct {
	bucket            SayDoBucket
	sumP, sumB, sumBS float64
	settled           int
	sumLR, sumALR     float64
	sumLW             float64
	widths            int
}

// addClock adds a clock claim's sharpness and, when its span was
// measured, its log ratio (R13).
func (a *sayDoAccumulator) addClock(rule PrecommitRule, settlement *CommitmentRecord) {
	if !rule.Kind.clock() || rule.LowMinutes < 1 || rule.HighMinutes < rule.LowMinutes {
		return
	}
	a.sumLW += math.Log(float64(rule.HighMinutes) / float64(rule.LowMinutes))
	a.widths++
	if settlement == nil || settlement.SubjectRefs[RuleRefCensored] == "true" || rule.EstimateMinutes < 1 {
		return
	}
	actualS, err := strconv.ParseInt(settlement.SubjectRefs[RuleRefActualS], 10, 64)
	if err != nil {
		return
	}
	actual := math.Max(float64(actualS)/60, 1.0/60)
	lr := math.Log(float64(rule.EstimateMinutes) / actual)
	a.bucket.Timed++
	a.sumLR += lr
	a.sumALR += math.Abs(lr)
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
	if b.Timed > 0 {
		b.MeanLogRatio = a.sumLR / float64(b.Timed)
		b.MeanAbsLogRatio = a.sumALR / float64(b.Timed)
	}
	if a.widths > 0 {
		b.MeanLogWidth = a.sumLW / float64(a.widths)
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
	var out SayDoRecord
	overall := &sayDoAccumulator{bucket: SayDoBucket{Rule: "all"}}
	byRule := map[string]*sayDoAccumulator{}
	for _, rec := range records {
		if rec.Precommit == nil || rec.Precommit.Rule == nil || rec.Resolve != nil ||
			!strings.HasPrefix(rec.Provenance.AgentID, agentPrefix) {
			continue
		}
		if RestatementRulePrecommit(rec) {
			continue // E1 F1: a restatement is no second claim
		}
		if LateRulePrecommit(rec) {
			out.Late++ // R20: counted, never scored
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
			if ClockCensoredUninformative(*rec.Precommit.Rule, settlement.SubjectRefs) {
				answer = PrecommitAnswerUnresolved // C7, a settlement from before it
			}
		}
		acc.add(p, answer, settled)
		overall.add(p, answer, settled)
		var settledRec *CommitmentRecord
		if settled {
			settledRec = &settlement
		}
		acc.addClock(*rec.Precommit.Rule, settledRec)
	}
	out.AgentPrefix, out.Overall = agentPrefix, overall.finish()
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

// RoundPredictionsField is the key of the predictions block in management's
// Cast spec to engineering. Every round states them before its capsule
// starts (owner, 2026-11-03), so a Cast without them is refused.
const RoundPredictionsField = "predictions"

// RoundPredictionsHow is the literal, copyable shape a refusal shows
// management.
const RoundPredictionsHow = RoundPredictionsExample

// RoundPredictionsMandatory are the claims every round states.
var RoundPredictionsMandatory = []PrecommitRuleKind{PrecommitRuleVerifyPass, PrecommitRuleJudgeAccepts, PrecommitRuleOwnerApproves, PrecommitRuleFreezeMinutes, PrecommitRuleApprovalMinutes}

// RoundPredictions is the predictions block: each p is the stater's chance
// in [0.02, 0.98] that the claim holds; minutes run from the Cast.
type RoundPredictions struct {
	VerifyPass      *float64              `json:"verify_pass"`
	JudgeAccepts    *float64              `json:"judge_accepts"`
	OwnerApproves   *float64              `json:"owner_approves"`
	FreezeMinutes   *RoundClockPrediction `json:"freeze_minutes"`
	ApprovalMinutes *RoundClockPrediction `json:"approval_minutes"`
	// ApprovedWithinRounds is optional.
	ApprovedWithinRounds *RoundCountPrediction `json:"approved_within_rounds,omitempty"`
}

// RoundCountPrediction is approved_within_rounds: the bound in Freezes and
// the chance the request is approved within it.
type RoundCountPrediction struct {
	Freezes int     `json:"freezes"`
	P       float64 `json:"p"`
}

// RoundClockPrediction is a clock claim: a point estimate in whole minutes,
// the interval around it, and the chance the actual falls inside.
type RoundClockPrediction struct {
	Estimate int     `json:"estimate"`
	Low      int     `json:"low"`
	High     int     `json:"high"`
	P        float64 `json:"p"`
}

// ParseRoundPredictions reads a Cast spec's predictions block strictly: an
// unknown key (a typo) or a missing mandatory claim is refused, never
// dropped. raw is the block's JSON (absent: nil).
func ParseRoundPredictions(raw json.RawMessage) (RoundPredictions, error) {
	var out RoundPredictions
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return out, fmt.Errorf("the spec states no predictions")
	}
	dec := json.NewDecoder(strings.NewReader(trimmed))
	dec.DisallowUnknownFields()
	if hint := PlaceholderHint(trimmed); hint != "" {
		return out, fmt.Errorf("predictions: %s", hint)
	}
	if err := dec.Decode(&out); err != nil {
		return out, fmt.Errorf("predictions: %w", err)
	}
	if dec.More() {
		return out, fmt.Errorf("predictions: trailing data after the block")
	}
	var missing []string
	if out.VerifyPass == nil {
		missing = append(missing, string(PrecommitRuleVerifyPass))
	}
	if out.JudgeAccepts == nil {
		missing = append(missing, string(PrecommitRuleJudgeAccepts))
	}
	if out.OwnerApproves == nil {
		missing = append(missing, string(PrecommitRuleOwnerApproves))
	}
	if out.FreezeMinutes == nil {
		missing = append(missing, string(PrecommitRuleFreezeMinutes))
	}
	if out.ApprovalMinutes == nil {
		missing = append(missing, string(PrecommitRuleApprovalMinutes))
	}
	if len(missing) > 0 {
		return out, fmt.Errorf("predictions: missing %s", strings.Join(missing, ", "))
	}
	for _, p := range out.Precommits() {
		if err := ValidateRulePrecommit(p); err != nil {
			return out, fmt.Errorf("predictions: %s: %w", p.Rule.Kind, err)
		}
	}
	return out, nil
}

// Precommits turns the block into rule precommits, in the mandatory order.
// AssignmentID and Since are the runtime's to stamp (R1).
func (r RoundPredictions) Precommits() []CommitmentPrecommit {
	binary := func(p float64) map[string]float64 {
		return map[string]float64{PrecommitAnswerYes: p, PrecommitAnswerNo: 1 - p}
	}
	var out []CommitmentPrecommit
	if r.VerifyPass != nil {
		out = append(out, CommitmentPrecommit{Question: "Management's Verify passes this round's candidate.",
			Distribution: binary(*r.VerifyPass), Resolver: PrecommitRuleResolver, Rule: &PrecommitRule{Kind: PrecommitRuleVerifyPass}})
	}
	if r.JudgeAccepts != nil {
		out = append(out, CommitmentPrecommit{Question: "The independent judgment accepts this round's candidate, so it reaches the owner's decision.",
			Distribution: binary(*r.JudgeAccepts), Resolver: PrecommitRuleResolver, Rule: &PrecommitRule{Kind: PrecommitRuleJudgeAccepts}})
	}
	if r.OwnerApproves != nil {
		out = append(out, CommitmentPrecommit{Question: "The owner approves this round's candidate.",
			Distribution: binary(*r.OwnerApproves), Resolver: PrecommitRuleResolver, Rule: &PrecommitRule{Kind: PrecommitRuleOwnerApproves}})
	}
	if c := r.FreezeMinutes; c != nil {
		out = append(out, CommitmentPrecommit{
			Question:     fmt.Sprintf("The round's Freeze lands %d to %d minutes after the Cast (estimate %d).", c.Low, c.High, c.Estimate),
			Distribution: binary(c.P), Resolver: PrecommitRuleResolver,
			Rule: &PrecommitRule{Kind: PrecommitRuleFreezeMinutes, EstimateMinutes: c.Estimate, LowMinutes: c.Low, HighMinutes: c.High}})
	}
	if c := r.ApprovalMinutes; c != nil {
		out = append(out, CommitmentPrecommit{
			Question:     fmt.Sprintf("The candidate is approved %d to %d minutes after the Cast (estimate %d).", c.Low, c.High, c.Estimate),
			Distribution: binary(c.P), Resolver: PrecommitRuleResolver,
			Rule: &PrecommitRule{Kind: PrecommitRuleApprovalMinutes, EstimateMinutes: c.Estimate, LowMinutes: c.Low, HighMinutes: c.High}})
	}
	if c := r.ApprovedWithinRounds; c != nil {
		out = append(out, CommitmentPrecommit{
			Question:     fmt.Sprintf("The request reaches an approved candidate within %d Freezes from this Cast.", c.Freezes),
			Distribution: binary(c.P), Resolver: PrecommitRuleResolver,
			Rule: &PrecommitRule{Kind: PrecommitRuleApprovedWithinRounds, WithinRounds: c.Freezes}})
	}
	return out
}

// Precommit shapes a desk can copy (live8 trace, 2026-11-04: management took
// six tries to find the body). Help("Precommit"), the decode errors and the
// Cast refusal show them literally. The shape is literal and the numbers are
// placeholders (C3 of docs/design/calibration-wiring-permutations-2026-11-04.md):
// a literal 0.8 became the most common stated tests_pass p, and the Cast
// example's numbers recurred in Casts. Failure modes, written first:
//   - a copied example anchors the stated numbers -> no number in any
//     example, only <p>, <estimate>, <low>, <high>, <n>;
//   - a desk copies a placeholder literally and reads a bare JSON error
//     -> the decode error says to replace each placeholder (PlaceholderHint).
const (
	PrecommitShapeExample       = `{"question": "The build passes on its first run.", "p": <p>, "resolver": "research"}`
	RulePrecommitJudgeExample   = `{"question": "The independent judgment accepts my candidate.", "p": <p>, "resolver": "rule", "rule": {"kind": "judge_accepts"}}`
	RulePrecommitClockExample   = `{"question": "My Freeze lands <low> to <high> minutes after the assignment opened.", "p": <p>, "resolver": "rule", "rule": {"kind": "freeze_minutes", "estimate_minutes": <estimate>, "low_minutes": <low>, "high_minutes": <high>}}`
	RoundPredictionsExample     = `"predictions": {"verify_pass": <p>, "judge_accepts": <p>, "owner_approves": <p>, "freeze_minutes": {"estimate": <estimate>, "low": <low>, "high": <high>, "p": <p>}, "approval_minutes": {"estimate": <estimate>, "low": <low>, "high": <high>, "p": <p>}, "approved_within_rounds": {"freezes": <n>, "p": <p>}}`
	precommitShapeErrorTemplate = "; the shape is %s (replace every <p> with your chance of yes from 0.02 to 0.98, and every <estimate>, <low>, <high> and <n> with your own whole number; \"p\" is short for \"distribution\": {\"yes\": p, \"no\": 1-p})"
)

// shapePlaceholders are the example's placeholders.
var shapePlaceholders = []string{"<p>", "<estimate>", "<low>", "<high>", "<n>"}

// PlaceholderHint is the reason a body that still holds an example's
// placeholder is refused; "" when it holds none.
func PlaceholderHint(raw string) string {
	for _, ph := range shapePlaceholders {
		if strings.Contains(raw, ph) {
			return "the body still holds the example's placeholder " + ph + ": replace every <p> with your own probability and every <estimate>, <low>, <high> and <n> with your own whole number"
		}
	}
	return ""
}

// FillShapePlaceholders puts neutral numbers in an example's placeholders,
// for the tests that check the shape decodes (0.5, 10 [5, 20], 2).
func FillShapePlaceholders(example string) string {
	return strings.NewReplacer("<p>", "0.5", "<estimate>", "10", "<low>", "5", "<high>", "20", "<n>", "2").Replace(example)
}

// PrecommitShapeHint is the suffix a precommit decode error carries.
func PrecommitShapeHint(rule bool) string {
	if rule {
		return fmt.Sprintf(precommitShapeErrorTemplate, RulePrecommitJudgeExample+" or "+RulePrecommitClockExample)
	}
	return fmt.Sprintf(precommitShapeErrorTemplate, PrecommitShapeExample)
}

// UnmarshalJSON reads a precommit body. Besides the stored form
// ("distribution": {answer: p}), it accepts the yes/no shorthand a desk
// writes: a scalar "p", or a scalar "distribution", both p(yes).
func (p *CommitmentPrecommit) UnmarshalJSON(data []byte) error {
	type stored CommitmentPrecommit
	var body struct {
		stored
		Distribution json.RawMessage `json:"distribution"`
		P            *float64        `json:"p"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		return err
	}
	*p = CommitmentPrecommit(body.stored)
	p.Distribution = nil
	yesNo := func(y float64) map[string]float64 {
		return map[string]float64{PrecommitAnswerYes: y, PrecommitAnswerNo: 1 - y}
	}
	if raw := strings.TrimSpace(string(body.Distribution)); raw != "" && raw != "null" {
		var scalar float64
		if err := json.Unmarshal(body.Distribution, &scalar); err == nil {
			p.Distribution = yesNo(scalar)
		} else if err := json.Unmarshal(body.Distribution, &p.Distribution); err != nil {
			return fmt.Errorf("distribution must be {\"yes\": p, \"no\": 1-p} or a number p: %w", err)
		}
	}
	// A body with both keys keeps its distribution: before the shorthand,
	// "p" was ignored, and a staged cell replayed after this landing must
	// decode as it did (Devin review 2, finding 1).
	if body.P != nil && p.Distribution == nil {
		p.Distribution = yesNo(*body.P)
	}
	return nil
}
