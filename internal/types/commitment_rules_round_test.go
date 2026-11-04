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
	ok := `{"verify_pass":0.8,"judge_accepts":0.7,"freeze_minutes":{"estimate":20,"low":10,"high":40,"p":0.75}}`
	p, err := ParseRoundPredictions([]byte(ok))
	if err != nil || len(p.Precommits()) != 3 {
		t.Fatalf("valid block: %v", err)
	}
	for i, pc := range p.Precommits() {
		if pc.Rule.Kind != RoundPredictionsMandatory[i] || ValidateRulePrecommit(pc) != nil {
			t.Fatalf("precommit %d: %+v", i, pc.Rule)
		}
	}
	for name, raw := range map[string]string{
		"absent":   ``,
		"trailing": `{"verify_pass":0.8,"judge_accepts":0.7,"freeze_minutes":{"estimate":20,"low":10,"high":40,"p":0.75}} {}`,
		"null":     `null`,
		"missing":  `{"verify_pass":0.8,"freeze_minutes":{"estimate":20,"low":10,"high":40,"p":0.75}}`,
		"unknown":  `{"verify_pass":0.8,"judge_accepts":0.7,"judge":0.1,"freeze_minutes":{"estimate":20,"low":10,"high":40,"p":0.75}}`,
		"clockkey": `{"verify_pass":0.8,"judge_accepts":0.7,"freeze_minutes":{"estimate":20,"low":10,"high":40,"p":0.75,"max":90}}`,
		"dogmatic": `{"verify_pass":0.99,"judge_accepts":0.7,"freeze_minutes":{"estimate":20,"low":10,"high":40,"p":0.75}}`,
		"outside":  `{"verify_pass":0.8,"judge_accepts":0.7,"freeze_minutes":{"estimate":5,"low":10,"high":40,"p":0.75}}`,
		"zero":     `{"verify_pass":0.8,"judge_accepts":0.7,"freeze_minutes":{"estimate":0,"low":0,"high":40,"p":0.75}}`,
		"toolong":  `{"verify_pass":0.8,"judge_accepts":0.7,"freeze_minutes":{"estimate":20,"low":10,"high":5000,"p":0.75}}`,
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
		if err := json.Unmarshal([]byte(example), &e); err != nil || len(e.Distribution) != 2 {
			t.Fatalf("example %s: %v", example, err)
		}
		if e.Rule != nil && ValidateRulePrecommit(e) != nil {
			t.Fatalf("rule example %s: %v", example, ValidateRulePrecommit(e))
		}
	}
	if _, err := ParseRoundPredictions([]byte(strings.TrimPrefix(RoundPredictionsExample, `"predictions": `))); err != nil {
		t.Fatalf("the round example does not parse: %v", err)
	}
}
