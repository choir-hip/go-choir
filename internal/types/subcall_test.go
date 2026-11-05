package types

import (
	"strings"
	"testing"
)

func TestSubCallRuleKindsAreInTheVocabulary(t *testing.T) {
	for _, k := range []PrecommitRuleKind{PrecommitRuleReportSchemaValid, PrecommitRuleCitationsResolve} {
		if !ValidSettlement(string(k)) {
			t.Fatalf("%s is not a valid settlement", k)
		}
		if err := (PrecommitRule{Kind: k}).validate(); err != nil {
			t.Fatalf("%s: %v", k, err)
		}
		p := CommitmentPrecommit{Question: "q", Resolver: PrecommitRuleResolver, Distribution: map[string]float64{"yes": 0.5, "no": 0.5}, Rule: &PrecommitRule{Kind: k}}
		if err := ValidateRulePrecommit(p); err == nil || !strings.Contains(err.Error(), "not a precommit rule") {
			t.Fatalf("%s was admitted as an assignment precommit: %v", k, err)
		}
	}
}

func TestSettleReportSchemaValidAndCitationsResolve(t *testing.T) {
	good := &CommitmentReport{Claims: []ReportClaim{{Text: "found", Sources: []string{"a", "b"}}}, StopReason: "done"}
	if s := SettleReportSchemaValid(good); s.Answer != PrecommitAnswerYes {
		t.Fatalf("%+v", s)
	}
	for _, bad := range []*CommitmentReport{nil, {StopReason: "done"}, {Claims: []ReportClaim{{Text: "x"}}}} {
		if s := SettleReportSchemaValid(bad); s.Answer != PrecommitAnswerNo {
			t.Fatalf("%+v passed", bad)
		}
	}
	s := SettleCitationsResolve(good, func(ref string) bool { return ref == "a" })
	if s.Answer != PrecommitAnswerNo || len(s.EvidenceRefs) != 1 || s.EvidenceRefs[0] != "b" {
		t.Fatalf("%+v", s)
	}
	if s := SettleCitationsResolve(good, func(string) bool { return true }); s.Answer != PrecommitAnswerYes {
		t.Fatalf("%+v", s)
	}
	if s := SettleCitationsResolve(&CommitmentReport{Claims: []ReportClaim{{Text: "x"}}}, func(string) bool { return false }); s.Answer != PrecommitAnswerYes {
		t.Fatal("a Report that cites nothing failed the citation rule")
	}
}

func TestTypedQuestionsAreValidated(t *testing.T) {
	ok := TypedQuestion{Question: "q", Answers: []string{"yes", "no"}, Probabilities: map[string]float64{"yes": 0.7, "no": 0.3}}
	if err := ValidateTypedQuestions([]TypedQuestion{ok}); err != nil {
		t.Fatal(err)
	}
	for name, q := range map[string]TypedQuestion{
		"one answer":   {Question: "q", Answers: []string{"yes"}},
		"sum":          {Question: "q", Answers: []string{"yes", "no"}, Probabilities: map[string]float64{"yes": 0.7, "no": 0.7}},
		"missing":      {Question: "q", Answers: []string{"yes", "no"}, Probabilities: map[string]float64{"yes": 1}},
		"stranger":     {Question: "q", Answers: []string{"yes", "no"}, Probabilities: map[string]float64{"yes": 0.5, "maybe": 0.5}},
		"empty":        {Question: " ", Answers: []string{"yes", "no"}},
		"duplicate":    {Question: "q", Answers: []string{"yes", "yes"}},
		"weight":       {Question: "q", Answers: []string{"yes", "no"}, Weight: 2},
		"out of range": {Question: "q", Answers: []string{"yes", "no"}, Probabilities: map[string]float64{"yes": 1.5, "no": -0.5}},
	} {
		if err := ValidateTypedQuestions([]TypedQuestion{q}); err == nil {
			t.Fatalf("%s was admitted", name)
		}
	}
}

func TestSubCallWeightFloorFollowsTheGrants(t *testing.T) {
	read, task := SubCallWeightFloor("research_child"), SubCallWeightFloor("management_task")
	if read != subCallFloorBase || task <= read || task > subCallFloorMax {
		t.Fatalf("floors: read-only %v, task %v", read, task)
	}
	if SubCallWeightFloor("no-such-role") != subCallFloorBase {
		t.Fatal("an unknown role has no floor")
	}
	w := 0.01
	rec := CommitmentRecord{Weight: &w}
	if flags := InertFlagsFor("management_task", rec); len(flags) != 2 {
		t.Fatalf("flags %v", flags)
	}
}

func TestModelFamily(t *testing.T) {
	for in, want := range map[string]string{"deepseek-v4.1-flash": "deepseek", "~typesafe/jev-1.13": "jev", "typesafe/jev-latest": "jev", "gpt-5.4-mini": "gpt", "": ""} {
		if got := ModelFamily(in); got != want {
			t.Fatalf("%q: %q, want %q", in, got, want)
		}
	}
}

func TestThirdOpinionSamplingIsDeterministicAndWeightedByMateriality(t *testing.T) {
	if ThirdOpinionSampled("c1", 0.3) != ThirdOpinionSampled("c1", 0.3) {
		t.Fatal("not deterministic")
	}
	n0, n1 := 0, 0
	for i := 0; i < 2000; i++ {
		id := SubCallCastRecordID("f", i)
		if ThirdOpinionSampled(id, 0) {
			n0++
		}
		if ThirdOpinionSampled(id, 1) {
			n1++
		}
	}
	if n1 != 2000 || n0 < 350 || n0 > 650 {
		t.Fatalf("sampled %d of 2000 at weight 0 (want ~25%%), %d at weight 1 (want all)", n0, n1)
	}
}
