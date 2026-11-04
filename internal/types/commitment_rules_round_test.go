package types

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"
)

func freezeMinutesRule(since time.Time) PrecommitRule {
	return PrecommitRule{Kind: PrecommitRuleFreezeMinutes, EstimateMinutes: 20, LowMinutes: 10, HighMinutes: 40,
		AssignmentID: "a1", Since: since.UTC().Format(time.RFC3339Nano)}
}

func TestParseRoundPredictionsIsStrict(t *testing.T) {
	ok := `{"verify_pass":0.8,"judge_accepts":0.7,"owner_approves":0.6,"approval_minutes":{"estimate":90,"low":40,"high":240,"p":0.6},"freeze_minutes":{"estimate":20,"low":10,"high":40,"p":0.75}}`
	p, err := ParseRoundPredictions([]byte(ok))
	if err != nil || len(p.Precommits()) != 5 {
		t.Fatalf("valid block: %v", err)
	}
	for i, pc := range p.Precommits() {
		if pc.Rule.Kind != RoundPredictionsMandatory[i] || ValidateRulePrecommit(pc) != nil {
			t.Fatalf("precommit %d: %+v", i, pc.Rule)
		}
	}
	for name, raw := range map[string]string{
		"absent":   ``,
		"trailing": `{"verify_pass":0.8,"judge_accepts":0.7,"owner_approves":0.6,"approval_minutes":{"estimate":90,"low":40,"high":240,"p":0.6},"freeze_minutes":{"estimate":20,"low":10,"high":40,"p":0.75}} {}`,
		"null":     `null`,
		"missing":  `{"verify_pass":0.8,"freeze_minutes":{"estimate":20,"low":10,"high":40,"p":0.75}}`,
		"unknown":  `{"verify_pass":0.8,"judge_accepts":0.7,"owner_approves":0.6,"approval_minutes":{"estimate":90,"low":40,"high":240,"p":0.6},"judge":0.1,"freeze_minutes":{"estimate":20,"low":10,"high":40,"p":0.75}}`,
		"clockkey": `{"verify_pass":0.8,"judge_accepts":0.7,"owner_approves":0.6,"approval_minutes":{"estimate":90,"low":40,"high":240,"p":0.6},"freeze_minutes":{"estimate":20,"low":10,"high":40,"p":0.75,"max":90}}`,
		"dogmatic": `{"verify_pass":0.99,"judge_accepts":0.7,"owner_approves":0.6,"approval_minutes":{"estimate":90,"low":40,"high":240,"p":0.6},"freeze_minutes":{"estimate":20,"low":10,"high":40,"p":0.75}}`,
		"outside":  `{"verify_pass":0.8,"judge_accepts":0.7,"owner_approves":0.6,"approval_minutes":{"estimate":90,"low":40,"high":240,"p":0.6},"freeze_minutes":{"estimate":5,"low":10,"high":40,"p":0.75}}`,
		"zero":     `{"verify_pass":0.8,"judge_accepts":0.7,"owner_approves":0.6,"approval_minutes":{"estimate":90,"low":40,"high":240,"p":0.6},"freeze_minutes":{"estimate":0,"low":0,"high":40,"p":0.75}}`,
		"toolong":  `{"verify_pass":0.8,"judge_accepts":0.7,"owner_approves":0.6,"approval_minutes":{"estimate":90,"low":40,"high":240,"p":0.6},"freeze_minutes":{"estimate":20,"low":10,"high":5000,"p":0.75}}`,
	} {
		if _, err := ParseRoundPredictions([]byte(raw)); err == nil {
			t.Fatalf("%s: accepted", name)
		}
	}
}

func TestIntervalFieldsBelongOnlyToFreezeMinutes(t *testing.T) {
	r := PrecommitRule{Kind: PrecommitRuleVerifyPass, EstimateMinutes: 5, LowMinutes: 1, HighMinutes: 9}
	if err := r.validate(); err == nil {
		t.Fatal("verify_pass took an interval")
	}
}

// R13, R14, R15: inside and outside the interval; a Freeze before the
// anchor does not decide; the timing is on the settlement.
func TestFreezeMinutesSettlesOnTheFreeze(t *testing.T) {
	cast := time.Date(2026, 11, 4, 1, 0, 0, 0, time.UTC)
	rule := freezeMinutesRule(cast)
	for _, c := range []struct {
		after  time.Duration
		answer string
	}{{25 * time.Minute, PrecommitAnswerYes}, {10 * time.Minute, PrecommitAnswerYes}, {5 * time.Minute, PrecommitAnswerNo}, {41 * time.Minute, PrecommitAnswerNo}} {
		s, ok := SettleRuleOnFreeze(rule, EngineeringFreezeObservation{FrozenAt: cast.Add(c.after)})
		if !ok || s.Answer != c.answer || s.Timing == nil || s.Timing.ActualS != int64(c.after/time.Second) || s.Timing.Censored {
			t.Fatalf("%s after the Cast: %+v %v", c.after, s, ok)
		}
	}
	if _, ok := SettleRuleOnFreeze(rule, EngineeringFreezeObservation{FrozenAt: cast.Add(-time.Minute)}); ok {
		t.Fatal("a Freeze before the anchor decided the claim (R14)")
	}
}

// R16: no Freeze is a miss with a censored span; a frozen candidate whose
// settlement was lost is unresolved, never a false no.
func TestFreezeMinutesAtTerminal(t *testing.T) {
	cast := time.Date(2026, 11, 4, 1, 0, 0, 0, time.UTC)
	rule := freezeMinutesRule(cast)
	s, ok := SettleRuleAtTerminal(rule, EngineeringTerminalObservation{Disposition: EngineeringAssignmentFailed, At: cast.Add(90 * time.Minute)})
	if !ok || s.Answer != PrecommitAnswerNo || s.Timing == nil || !s.Timing.Censored || s.Timing.ActualS != 5400 {
		t.Fatalf("no Freeze: %+v", s)
	}
	s, ok = SettleRuleAtTerminal(rule, EngineeringTerminalObservation{Disposition: EngineeringAssignmentCompleted, At: cast.Add(time.Hour), CandidateFrozen: true})
	if !ok || s.Answer != PrecommitAnswerUnresolved {
		t.Fatalf("lost freeze settlement: %+v", s)
	}
}

// R17: judge_accepts reads where the judgment left the operation.
func TestJudgeAcceptsSettlesOnTheJudgment(t *testing.T) {
	rule := PrecommitRule{Kind: PrecommitRuleJudgeAccepts}
	for state, want := range map[string]string{"awaiting_approval": PrecommitAnswerYes, "failed": PrecommitAnswerNo} {
		s, ok := SettleRuleOnVerify(rule, EngineeringVerifyObservation{Decision: "pass", OperationState: state, OperationID: "op"})
		if !ok || s.Answer != want || !strings.Contains(strings.Join(s.EvidenceRefs, " "), "selfdev-operation:op") {
			t.Fatalf("%s: %+v", state, s)
		}
	}
	if _, ok := SettleRuleOnVerify(rule, EngineeringVerifyObservation{Decision: "pass", OperationState: "verified"}); ok {
		t.Fatal("an unfinished verdict decided judge_accepts")
	}
	if s, ok := SettleRuleAtTerminal(rule, EngineeringTerminalObservation{Disposition: EngineeringAssignmentCompleted, CandidateFrozen: true}); ok {
		t.Fatalf("a completed candidate's judge_accepts settled before the judgment: %+v", s)
	}
	if s, ok := SettleRuleAtTerminal(rule, EngineeringTerminalObservation{Disposition: EngineeringAssignmentFailed}); !ok || s.Answer != PrecommitAnswerNo {
		t.Fatalf("no candidate: %+v", s)
	}
}

// R13: the view reports coverage beside the log ratio and the log width,
// and a censored span enters coverage but not the log ratio.
func TestSayDoReportsClockSharpnessAndLogRatio(t *testing.T) {
	cast := time.Date(2026, 11, 4, 1, 0, 0, 0, time.UTC)
	var records []CommitmentRecord
	add := func(id string, at time.Time, terminal bool) {
		rule := freezeMinutesRule(cast)
		pc := CommitmentRecord{RecordID: id, Precommit: &CommitmentPrecommit{Question: "q", Resolver: "rule",
			Distribution: map[string]float64{"yes": 0.8, "no": 0.2}, Rule: &rule}, Provenance: CommitmentProvenance{AgentID: "engineering:a1"}}
		var s RuleSettlement
		if terminal {
			s, _ = SettleRuleAtTerminal(rule, EngineeringTerminalObservation{Disposition: EngineeringAssignmentFailed, At: at})
		} else {
			s, _ = SettleRuleOnFreeze(rule, EngineeringFreezeObservation{FrozenAt: at})
		}
		records = append(records, pc, NewRuleResolveRecord(pc, s, at))
	}
	add("p1", cast.Add(20*time.Minute), false) // on the estimate: ln 1 = 0
	add("p2", cast.Add(40*time.Minute), false) // ln(20/40)
	add("p3", cast.Add(3*time.Hour), true)     // censored
	view := RulePrecommitSayDo(records, "engineering:")
	if len(view.ByRule) != 1 {
		t.Fatalf("by rule: %+v", view.ByRule)
	}
	b := view.ByRule[0]
	if b.Yes != 2 || b.No != 1 || b.Timed != 2 {
		t.Fatalf("bucket %+v", b)
	}
	if math.Abs(b.MeanLogRatio-math.Log(0.5)/2) > 1e-9 || math.Abs(b.MeanLogWidth-math.Log(4)) > 1e-9 {
		t.Fatalf("log ratio %v width %v", b.MeanLogRatio, b.MeanLogWidth)
	}
}

// live8 trace: a desk writes "p" or a scalar distribution; both mean p(yes).
func TestPrecommitAcceptsTheYesNoShorthand(t *testing.T) {
	for name, raw := range map[string]string{
		"p":      `{"question":"q","p":0.6,"resolver":"rule","rule":{"kind":"judge_accepts"}}`,
		"scalar": `{"question":"q","distribution":0.6,"resolver":"rule","rule":{"kind":"judge_accepts"}}`,
		"stored": `{"question":"q","distribution":{"yes":0.6,"no":0.4},"resolver":"rule","rule":{"kind":"judge_accepts"}}`,
	} {
		var p CommitmentPrecommit
		if err := json.Unmarshal([]byte(raw), &p); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if math.Abs(p.Distribution["yes"]-0.6) > 1e-9 || math.Abs(p.Distribution["no"]-0.4) > 1e-9 || ValidateRulePrecommit(p) != nil {
			t.Fatalf("%s: %+v %v", name, p.Distribution, ValidateRulePrecommit(p))
		}
		out, _ := json.Marshal(p)
		if strings.Contains(string(out), `"p"`) {
			t.Fatalf("%s: the stored form carries the shorthand: %s", name, out)
		}
	}
	var p CommitmentPrecommit
	if err := json.Unmarshal([]byte(`{"question":"q","p":0.9,"distribution":{"yes":0.6,"no":0.4},"resolver":"r"}`), &p); err != nil || p.Distribution["yes"] != 0.6 {
		t.Fatalf("both keys: the distribution must win, as before the shorthand: %+v %v", p.Distribution, err)
	}
	for _, example := range []string{PrecommitShapeExample, RulePrecommitJudgeExample, RulePrecommitClockExample} {
		var e CommitmentPrecommit
		if PlaceholderHint(example) == "" || strings.Contains(example, "0.") {
			t.Fatalf("example %s states a number (C3): it must hold placeholders only", example)
		}
		example = FillShapePlaceholders(example)
		if err := json.Unmarshal([]byte(example), &e); err != nil || len(e.Distribution) != 2 {
			t.Fatalf("example %s: %v", example, err)
		}
		if e.Rule != nil && ValidateRulePrecommit(e) != nil {
			t.Fatalf("rule example %s: %v", example, ValidateRulePrecommit(e))
		}
	}
	if _, err := ParseRoundPredictions([]byte(strings.TrimPrefix(RoundPredictionsExample, `"predictions": `))); err == nil || !strings.Contains(err.Error(), "placeholder") {
		t.Fatalf("a copied placeholder is refused with the hint: %v", err)
	}
	if _, err := ParseRoundPredictions([]byte(FillShapePlaceholders(strings.TrimPrefix(RoundPredictionsExample, `"predictions": `)))); err != nil {
		t.Fatalf("the round example does not parse: %v", err)
	}
}

// R18: the owner's act settles owner_approves and approval_minutes, with
// the actor and the delegated mark on the settlement.
func TestOwnerActSettlesApprovalClaims(t *testing.T) {
	cast := time.Date(2026, 11, 4, 1, 0, 0, 0, time.UTC)
	approves := PrecommitRule{Kind: PrecommitRuleOwnerApproves, Since: cast.Format(time.RFC3339Nano)}
	minutes := PrecommitRule{Kind: PrecommitRuleApprovalMinutes, EstimateMinutes: 90, LowMinutes: 40, HighMinutes: 240, Since: cast.Format(time.RFC3339Nano)}
	for _, c := range []struct {
		act               string
		after             time.Duration
		approved, inside  string
		censored, decided bool
	}{
		{OwnerActApprove, time.Hour, "yes", "yes", false, true},
		{OwnerActApply, 5 * time.Hour, "yes", "no", false, true},
		{OwnerActReject, time.Hour, "no", "no", true, true},
		{OwnerActAbandon, time.Hour, "no", "no", true, true},
		{"rollback", time.Hour, "", "", false, false},
	} {
		obs := EngineeringOwnerActObservation{Act: c.act, OperationID: "op", DecisionAt: cast.Add(c.after), Actor: "consensus:x", Delegated: true}
		a, okA := SettleRuleOnOwnerAct(approves, obs)
		m, okM := SettleRuleOnOwnerAct(minutes, obs)
		if okA != c.decided || okM != c.decided {
			t.Fatalf("%s decided %v %v", c.act, okA, okM)
		}
		if !c.decided {
			continue
		}
		if a.Answer != c.approved || m.Answer != c.inside || m.Timing == nil || m.Timing.Censored != c.censored || a.Refs[RuleRefDelegated] != "true" || a.Refs[RuleRefDecisionActor] != "consensus:x" {
			t.Fatalf("%s: approves %+v minutes %+v", c.act, a, m)
		}
	}
	// A failed judgment ends both before the owner sees the candidate.
	if s, ok := SettleRuleOnVerify(approves, EngineeringVerifyObservation{OperationState: "failed", At: cast.Add(time.Hour)}); !ok || s.Answer != "no" {
		t.Fatalf("judged out: %+v", s)
	}
	if s, ok := SettleRuleOnVerify(minutes, EngineeringVerifyObservation{OperationState: "failed", At: cast.Add(time.Hour)}); !ok || s.Answer != "no" || !s.Timing.Censored {
		t.Fatalf("judged out minutes: %+v", s)
	}
	if _, ok := SettleRuleOnVerify(approves, EngineeringVerifyObservation{OperationState: "awaiting_approval"}); ok {
		t.Fatal("a passed judgment decided owner_approves")
	}
	// The approval clock's resolve record names the decision, not a Freeze.
	pc := CommitmentRecord{RecordID: "p", Precommit: &CommitmentPrecommit{Question: "q", Distribution: map[string]float64{"yes": 0.6, "no": 0.4}, Resolver: "rule", Rule: &minutes}}
	m, _ := SettleRuleOnOwnerAct(minutes, EngineeringOwnerActObservation{Act: OwnerActApprove, DecisionAt: cast.Add(time.Hour)})
	rec := NewRuleResolveRecord(pc, m, cast.Add(time.Hour))
	if rec.SubjectRefs[RuleRefDecisionAt] == "" || rec.SubjectRefs[RuleRefFreezeAt] != "" || rec.SubjectRefs[RuleRefActualS] != "3600" {
		t.Fatalf("approval resolve refs %v", rec.SubjectRefs)
	}
}

// R19: approved_within_rounds counts Freezes from the Cast across rounds.
func TestApprovedWithinRoundsCountsFreezes(t *testing.T) {
	rule := PrecommitRule{Kind: PrecommitRuleApprovedWithinRounds, WithinRounds: 2}
	if err := rule.validate(); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		obs    EngineeringRequestObservation
		answer string
		ok     bool
	}{
		{EngineeringRequestObservation{FreezesSince: 1}, "", false},
		{EngineeringRequestObservation{FreezesSince: 2, Approved: true, FreezesToApproval: 2}, "yes", true},
		{EngineeringRequestObservation{FreezesSince: 3, Approved: true, FreezesToApproval: 3}, "no", true},
		{EngineeringRequestObservation{FreezesSince: 3}, "no", true},
		{EngineeringRequestObservation{FreezesSince: 1, Abandoned: true}, "no", true},
	} {
		s, ok := SettleRuleOnRequest(rule, c.obs)
		if ok != c.ok || s.Answer != c.answer {
			t.Fatalf("%+v: %+v %v", c.obs, s, ok)
		}
	}
	if err := (PrecommitRule{Kind: PrecommitRuleApprovedWithinRounds}).validate(); err == nil {
		t.Fatal("within_rounds 0 accepted")
	}
	if err := (PrecommitRule{Kind: PrecommitRuleOwnerApproves, WithinRounds: 2}).validate(); err == nil {
		t.Fatal("owner_approves took within_rounds")
	}
}
