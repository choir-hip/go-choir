package types

import (
	"math"
	"strings"
	"testing"
	"time"
)

func rulePrecommit(p float64, rule PrecommitRule) CommitmentPrecommit {
	return CommitmentPrecommit{
		Question:     "claim",
		Distribution: map[string]float64{PrecommitAnswerYes: p, PrecommitAnswerNo: 1 - p},
		Resolver:     PrecommitRuleResolver,
		Rule:         &rule,
	}
}

// R3: the staged shape a rule precommit must have.
func TestValidateRulePrecommit(t *testing.T) {
	ok := []CommitmentPrecommit{
		rulePrecommit(0.7, PrecommitRule{Kind: PrecommitRuleTestsPass}),
		rulePrecommit(0.02, PrecommitRule{Kind: PrecommitRuleFreezeWithin, WithinMinutes: 30}),
		rulePrecommit(0.98, PrecommitRule{Kind: PrecommitRulePathsConfined, Paths: []string{"frontend/", "/workspace/platform/internal/capsule"}}),
		rulePrecommit(0.5, PrecommitRule{Kind: PrecommitRuleVerifyPass}),
		rulePrecommit(0.6, PrecommitRule{Kind: PrecommitRuleCompletes}),
	}
	for i, p := range ok {
		if err := ValidateRulePrecommit(p); err != nil {
			t.Fatalf("valid precommit %d refused: %v", i, err)
		}
	}
	noRule := rulePrecommit(0.5, PrecommitRule{Kind: PrecommitRuleCompletes})
	noRule.Rule = nil
	wrongResolver := rulePrecommit(0.5, PrecommitRule{Kind: PrecommitRuleCompletes})
	wrongResolver.Resolver = "management"
	otherAnswers := rulePrecommit(0.5, PrecommitRule{Kind: PrecommitRuleCompletes})
	otherAnswers.Distribution = map[string]float64{"pass": 0.5, "fail": 0.5}
	notSummed := rulePrecommit(0.5, PrecommitRule{Kind: PrecommitRuleCompletes})
	notSummed.Distribution[PrecommitAnswerNo] = 0.6
	bad := map[string]CommitmentPrecommit{
		"no rule":             noRule,
		"wrong resolver":      wrongResolver,
		"other answers":       otherAnswers,
		"not summing to one":  notSummed,
		"dogmatic one":        rulePrecommit(1, PrecommitRule{Kind: PrecommitRuleCompletes}),
		"dogmatic zero":       rulePrecommit(0, PrecommitRule{Kind: PrecommitRuleCompletes}),
		"unknown kind":        rulePrecommit(0.5, PrecommitRule{Kind: "lines_changed"}),
		"freeze without time": rulePrecommit(0.5, PrecommitRule{Kind: PrecommitRuleFreezeWithin}),
		"freeze past a day":   rulePrecommit(0.5, PrecommitRule{Kind: PrecommitRuleFreezeWithin, WithinMinutes: 24*60 + 1}),
		"paths empty":         rulePrecommit(0.5, PrecommitRule{Kind: PrecommitRulePathsConfined}),
		"paths escaping":      rulePrecommit(0.5, PrecommitRule{Kind: PrecommitRulePathsConfined, Paths: []string{"../etc"}}),
		"paths the root":      rulePrecommit(0.5, PrecommitRule{Kind: PrecommitRulePathsConfined, Paths: []string{"/workspace/platform/"}}),
		"tests with paths":    rulePrecommit(0.5, PrecommitRule{Kind: PrecommitRuleTestsPass, Paths: []string{"x"}}),
	}
	for name, p := range bad {
		if err := ValidateRulePrecommit(p); err == nil {
			t.Fatalf("%s: accepted", name)
		}
	}
}

// R8: whole-segment prefixes.
func TestWithinPathPrefixMatchesWholeSegments(t *testing.T) {
	if !WithinPathPrefix("frontend/src/App.svelte", "frontend") || !WithinPathPrefix("frontend", "frontend") {
		t.Fatal("a file under the prefix must match")
	}
	if WithinPathPrefix("frontend-old/x", "frontend") || WithinPathPrefix("front/x", "frontend") {
		t.Fatal("a sibling sharing the prefix's letters must not match")
	}
}

func TestSettleRuleOnFreeze(t *testing.T) {
	opened := time.Date(2026, 10, 14, 18, 0, 0, 0, time.UTC)
	since := opened.Format(time.RFC3339Nano)
	within := PrecommitRule{Kind: PrecommitRuleFreezeWithin, WithinMinutes: 30, Since: since}
	frozen := func(after time.Duration) EngineeringFreezeObservation {
		return EngineeringFreezeObservation{FrozenAt: opened.Add(after), BundleDigest: "sha256:b", OperationID: "op1"}
	}
	if s, ok := SettleRuleOnFreeze(within, frozen(29*time.Minute)); !ok || s.Answer != PrecommitAnswerYes {
		t.Fatalf("freeze inside the bound = %+v %v, want yes", s, ok)
	}
	if s, ok := SettleRuleOnFreeze(within, frozen(31*time.Minute)); !ok || s.Answer != PrecommitAnswerNo || !strings.Contains(s.Excerpt, "past the 30m bound") {
		t.Fatalf("late freeze = %+v %v, want no (late)", s, ok)
	}
	if s, _ := SettleRuleOnFreeze(PrecommitRule{Kind: PrecommitRuleFreezeWithin, WithinMinutes: 30}, frozen(time.Minute)); s.Answer != PrecommitAnswerUnresolved {
		t.Fatalf("freeze_within with no anchor = %+v, want unresolved", s)
	}

	tests := PrecommitRule{Kind: PrecommitRuleTestsPass}
	pass := frozen(time.Minute)
	pass.TestReceipts = []EngineeringReceiptExit{{Ref: "capsule-go-eval:sha256:a", ExitCode: 0}, {Ref: "capsule-go-eval:sha256:b", ExitCode: 0}}
	if s, ok := SettleRuleOnFreeze(tests, pass); !ok || s.Answer != PrecommitAnswerYes || len(s.EvidenceRefs) != 4 {
		t.Fatalf("passing tests = %+v %v, want yes with bundle, operation and both receipts", s, ok)
	}
	fail := pass
	fail.TestReceipts = []EngineeringReceiptExit{{Ref: "capsule-go-eval:sha256:a", ExitCode: 0}, {Ref: "capsule-go-eval:sha256:b", ExitCode: 1}}
	if s, ok := SettleRuleOnFreeze(tests, fail); !ok || s.Answer != PrecommitAnswerNo || !strings.Contains(s.Excerpt, "sha256:b exited 1") {
		t.Fatalf("failing test receipt = %+v %v, want no naming it", s, ok)
	}

	confined := PrecommitRule{Kind: PrecommitRulePathsConfined, Paths: []string{"frontend/", "docs"}}
	inside := frozen(time.Minute)
	inside.SourcePathsKnown, inside.SourcePaths = true, []string{"frontend/src/a.ts", "docs/x.md"}
	if s, ok := SettleRuleOnFreeze(confined, inside); !ok || s.Answer != PrecommitAnswerYes {
		t.Fatalf("confined change = %+v %v, want yes", s, ok)
	}
	outside := inside
	outside.SourcePaths = []string{"frontend/src/a.ts", "internal/store/x.go"}
	if s, ok := SettleRuleOnFreeze(confined, outside); !ok || s.Answer != PrecommitAnswerNo || !strings.Contains(s.Excerpt, "internal/store/x.go") {
		t.Fatalf("change outside = %+v %v, want no naming the file", s, ok)
	}
	unknown := inside
	unknown.SourcePathsKnown = false
	if _, ok := SettleRuleOnFreeze(confined, unknown); ok {
		t.Fatal("paths_confined settled without observed paths")
	}
	for _, kind := range []PrecommitRuleKind{PrecommitRuleVerifyPass, PrecommitRuleCompletes} {
		if _, ok := SettleRuleOnFreeze(PrecommitRule{Kind: kind}, pass); ok {
			t.Fatalf("a freeze settled %s", kind)
		}
	}
}

func TestSettleRuleOnVerify(t *testing.T) {
	rule := PrecommitRule{Kind: PrecommitRuleVerifyPass}
	if s, ok := SettleRuleOnVerify(rule, EngineeringVerifyObservation{Decision: "pass", BundleDigest: "sha256:b", VerifierRefs: []string{"v1"}}); !ok || s.Answer != PrecommitAnswerYes || len(s.EvidenceRefs) != 2 {
		t.Fatalf("pass = %+v %v", s, ok)
	}
	if s, ok := SettleRuleOnVerify(rule, EngineeringVerifyObservation{Decision: "fail"}); !ok || s.Answer != PrecommitAnswerNo {
		t.Fatalf("fail = %+v %v", s, ok)
	}
	if _, ok := SettleRuleOnVerify(PrecommitRule{Kind: PrecommitRuleTestsPass}, EngineeringVerifyObservation{Decision: "pass"}); ok {
		t.Fatal("verify settled tests_pass")
	}
}

func TestSettleRuleAtTerminal(t *testing.T) {
	opened := time.Date(2026, 10, 14, 18, 0, 0, 0, time.UTC)
	since := opened.Format(time.RFC3339Nano)
	cancelledLate := EngineeringTerminalObservation{Disposition: EngineeringAssignmentCancelled, Reason: "deadline expired", At: opened.Add(2 * time.Hour)}
	// Timeout: the assignment ended with no freeze, past the bound.
	if s, ok := SettleRuleAtTerminal(PrecommitRule{Kind: PrecommitRuleFreezeWithin, WithinMinutes: 30, Since: since}, cancelledLate); !ok || s.Answer != PrecommitAnswerNo || !strings.HasPrefix(s.Excerpt, "timeout") {
		t.Fatalf("timeout = %+v %v, want no (timeout)", s, ok)
	}
	if s, ok := SettleRuleAtTerminal(PrecommitRule{Kind: PrecommitRuleTestsPass}, cancelledLate); !ok || s.Answer != PrecommitAnswerNo {
		t.Fatalf("tests with no freeze = %+v %v, want no", s, ok)
	}
	if s, ok := SettleRuleAtTerminal(PrecommitRule{Kind: PrecommitRulePathsConfined, Paths: []string{"x"}}, cancelledLate); !ok || s.Answer != PrecommitAnswerUnresolved {
		t.Fatalf("paths with no candidate = %+v %v, want unresolved", s, ok)
	}
	if s, ok := SettleRuleAtTerminal(PrecommitRule{Kind: PrecommitRuleCompletes}, cancelledLate); !ok || s.Answer != PrecommitAnswerNo {
		t.Fatalf("completes when cancelled = %+v %v, want no", s, ok)
	}
	if s, ok := SettleRuleAtTerminal(PrecommitRule{Kind: PrecommitRuleVerifyPass}, cancelledLate); !ok || s.Answer != PrecommitAnswerNo {
		t.Fatalf("verify with nothing to verify = %+v %v, want no", s, ok)
	}

	completed := EngineeringTerminalObservation{Disposition: EngineeringAssignmentCompleted, At: opened.Add(10 * time.Minute), CandidateFrozen: true, CandidateRef: "selfdev-operation:op1"}
	if s, ok := SettleRuleAtTerminal(PrecommitRule{Kind: PrecommitRuleCompletes}, completed); !ok || s.Answer != PrecommitAnswerYes {
		t.Fatalf("completes = %+v %v, want yes", s, ok)
	}
	// R10: a completed candidate waits for management's Verify.
	if _, ok := SettleRuleAtTerminal(PrecommitRule{Kind: PrecommitRuleVerifyPass}, completed); ok {
		t.Fatal("verify_pass settled before Verify")
	}
	// R5: a candidate froze but its freeze settlement is missing.
	for _, rule := range []PrecommitRule{{Kind: PrecommitRuleFreezeWithin, WithinMinutes: 30, Since: since}, {Kind: PrecommitRuleTestsPass}, {Kind: PrecommitRulePathsConfined, Paths: []string{"x"}}} {
		if s, ok := SettleRuleAtTerminal(rule, completed); !ok || s.Answer != PrecommitAnswerUnresolved {
			t.Fatalf("%s with an unrecorded freeze = %+v %v, want unresolved", rule.Kind, s, ok)
		}
	}
}

func TestNewRuleResolveRecordLinksAndClassifies(t *testing.T) {
	pc := rulePrecommit(0.8, PrecommitRule{Kind: PrecommitRuleTestsPass, AssignmentID: "a1"})
	act := CommitmentRecord{RecordID: "cell:precommit:1", Kind: CommitmentKindPrecommit, Precommit: &pc,
		Prediction: CommitmentPrediction{Hypothesis: "claim"}, Provenance: CommitmentProvenance{AgentID: "engineering:a1", ContextRef: "ch"}}
	at := time.Date(2026, 10, 14, 19, 0, 0, 0, time.UTC)
	yes := NewRuleResolveRecord(act, RuleSettlement{Answer: PrecommitAnswerYes, Excerpt: "ok", EvidenceRefs: []string{"bundle:sha256:b"}}, at)
	if yes.RecordID != "cell:precommit:1:resolve:rule" || yes.ParentID != act.RecordID || yes.RecordKind() != CommitmentKindResolve ||
		yes.Discrepancy != DiscrepancyConfirmed || yes.Provenance.AgentID != RuleScorerAgentID || yes.Scores[0].ScorerModelID != "rule:tests_pass" {
		t.Fatalf("yes resolve = %+v", yes)
	}
	if no := NewRuleResolveRecord(act, RuleSettlement{Answer: PrecommitAnswerNo}, at); no.Discrepancy != DiscrepancyContradicted {
		t.Fatalf("no against p=0.8 = %s, want contradicted", no.Discrepancy)
	}
	if un := NewRuleResolveRecord(act, RuleSettlement{Answer: PrecommitAnswerUnresolved}, at); un.Discrepancy != DiscrepancyUnresolved {
		t.Fatalf("unresolved = %s", un.Discrepancy)
	}
	// The existing views join it as the act's governing resolution.
	resolved := ResolveCommitments([]CommitmentRecord{act, yes}, at)
	if len(resolved) != 1 || resolved[0].Discrepancy != DiscrepancyConfirmed || resolved[0].Observation.Excerpt != "ok" {
		t.Fatalf("ResolveCommitments = %+v", resolved)
	}
	// R12: the acting pack shows the discrepancy and observation, never the score.
	pack := BuildActingPack([]CommitmentRecord{act, yes}, "engineering:a1", 10)
	if len(pack.Items) != 1 || pack.Items[0].Discrepancy != DiscrepancyConfirmed {
		t.Fatalf("acting pack = %+v", pack)
	}
}

func TestBrierBinary(t *testing.T) {
	cases := []struct {
		p    float64
		yes  bool
		want float64
	}{{0.8, true, 0.04}, {0.8, false, 0.64}, {0.5, true, 0.25}, {0.02, false, 0.0004}}
	for _, c := range cases {
		if got := BrierBinary(c.p, c.yes); math.Abs(got-c.want) > 1e-12 {
			t.Fatalf("BrierBinary(%v, %v) = %v, want %v", c.p, c.yes, got, c.want)
		}
	}
}

// The say–do view: per rule and overall, resolved-only and strict Brier,
// only the rule scorer governs, other agents excluded.
func TestRulePrecommitSayDo(t *testing.T) {
	at := time.Date(2026, 10, 14, 19, 0, 0, 0, time.UTC)
	mk := func(id, agent string, p float64, kind PrecommitRuleKind) CommitmentRecord {
		pc := rulePrecommit(p, PrecommitRule{Kind: kind})
		return CommitmentRecord{RecordID: id, Kind: CommitmentKindPrecommit, Precommit: &pc, Prediction: CommitmentPrediction{Hypothesis: id}, Provenance: CommitmentProvenance{AgentID: agent}}
	}
	t1 := mk("t1", "engineering:a1", 0.9, PrecommitRuleTestsPass)
	t2 := mk("t2", "engineering:a2", 0.6, PrecommitRuleTestsPass)
	f1 := mk("f1", "engineering:a1", 0.7, PrecommitRuleFreezeWithin)
	c1 := mk("c1", "engineering:a1", 0.5, PrecommitRuleCompletes)
	m1 := mk("m1", "management:o", 0.9, PrecommitRuleCompletes)
	records := []CommitmentRecord{t1, t2, f1, c1, m1,
		NewRuleResolveRecord(t1, RuleSettlement{Answer: PrecommitAnswerYes}, at),
		NewRuleResolveRecord(t2, RuleSettlement{Answer: PrecommitAnswerNo}, at),
		NewRuleResolveRecord(f1, RuleSettlement{Answer: PrecommitAnswerUnresolved}, at),
		NewRuleResolveRecord(m1, RuleSettlement{Answer: PrecommitAnswerYes}, at),
	}
	// A desk's own resolve is a different scorer and does not govern here.
	desk := NewRuleResolveRecord(c1, RuleSettlement{Answer: PrecommitAnswerYes}, at)
	desk.RecordID, desk.Provenance.AgentID = "c1:resolve:self", "engineering:a1"
	records = append(records, desk)

	got := RulePrecommitSayDo(records, "engineering:")
	o := got.Overall
	if o.Committed != 4 || o.Open != 1 || o.Yes != 1 || o.No != 1 || o.Unresolved != 1 {
		t.Fatalf("overall counts = %+v", o)
	}
	approx := func(name string, got, want float64) {
		t.Helper()
		if math.Abs(got-want) > 1e-9 {
			t.Fatalf("%s = %v, want %v", name, got, want)
		}
	}
	approx("mean p", o.MeanP, (0.9+0.6+0.7+0.5)/4)
	approx("brier (resolved)", o.Brier, (0.01+0.36)/2)
	approx("brier strict", o.BrierStrict, (0.01+0.36+0.49)/3)
	approx("yes rate", o.YesRate, 0.5)
	if len(got.ByRule) != 3 || got.ByRule[0].Rule != "completes" || got.ByRule[2].Rule != "tests_pass" {
		t.Fatalf("by rule = %+v", got.ByRule)
	}
	tp := got.ByRule[2]
	if tp.Committed != 2 || tp.Yes != 1 || tp.No != 1 {
		t.Fatalf("tests_pass bucket = %+v", tp)
	}
	approx("tests_pass brier", tp.Brier, (0.01+0.36)/2)
}
