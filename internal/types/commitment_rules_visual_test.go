package types

import (
	"strings"
	"testing"
	"time"
)

// visual_captured (docs/design/visual-evidence-slice-2026-10-15.md V17, V23):
// Freeze carries a screenshot of the frozen frontend, vouched for by the
// capture job's receipt. The rule settles from the runtime's Freeze
// observation only, never from a desk's word.

func visualObs(receipt string, captures ...VisualCaptureObservation) EngineeringFreezeObservation {
	return EngineeringFreezeObservation{
		FrozenAt: time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC), BundleDigest: "b1", OperationID: "op1",
		VisualKnown: true, VisualReceipt: receipt, VisualCaptures: captures,
	}
}

func capture(route string, blank bool) VisualCaptureObservation {
	return VisualCaptureObservation{Route: route, Artifact: "artifact:sha256:" + strings.Repeat("a", 63) + route[len(route)-1:], Blank: blank}
}

const visualReceipt = "capsule-exec:sha256:" + "1111111111111111111111111111111111111111111111111111111111111111"

func TestValidateVisualCapturedRule(t *testing.T) {
	ok := []PrecommitRule{
		{Kind: PrecommitRuleVisualCaptured},
		{Kind: PrecommitRuleVisualCaptured, Routes: []string{"/"}},
		{Kind: PrecommitRuleVisualCaptured, Routes: []string{"/", "/choir-drill/x.html", "/a?b=c"}},
	}
	for i, r := range ok {
		if err := ValidateRulePrecommit(rulePrecommit(0.7, r)); err != nil {
			t.Fatalf("valid rule %d refused: %v", i, err)
		}
	}
	tooMany := []string{"/1", "/2", "/3", "/4", "/5", "/6", "/7"}
	bad := map[string]PrecommitRule{
		"relative route":        {Kind: PrecommitRuleVisualCaptured, Routes: []string{"index.html"}},
		"another origin":        {Kind: PrecommitRuleVisualCaptured, Routes: []string{"https://example.com/"}},
		"escaping route":        {Kind: PrecommitRuleVisualCaptured, Routes: []string{"/a/../../b"}},
		"duplicate route":       {Kind: PrecommitRuleVisualCaptured, Routes: []string{"/", "/"}},
		"too many routes":       {Kind: PrecommitRuleVisualCaptured, Routes: tooMany},
		"with paths":            {Kind: PrecommitRuleVisualCaptured, Paths: []string{"frontend"}},
		"with minutes":          {Kind: PrecommitRuleVisualCaptured, WithinMinutes: 5},
		"routes on another one": {Kind: PrecommitRuleTestsPass, Routes: []string{"/"}},
	}
	for name, r := range bad {
		if err := ValidateRulePrecommit(rulePrecommit(0.7, r)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestSettleVisualCapturedOnFreeze(t *testing.T) {
	whole := PrecommitRule{Kind: PrecommitRuleVisualCaptured}
	root := PrecommitRule{Kind: PrecommitRuleVisualCaptured, Routes: []string{"/"}}
	two := PrecommitRule{Kind: PrecommitRuleVisualCaptured, Routes: []string{"/", "/drill.html"}}

	cases := []struct {
		name   string
		rule   PrecommitRule
		obs    EngineeringFreezeObservation
		want   string
		inText string
	}{
		{"a vouched capture", whole, visualObs(visualReceipt, capture("/", false)), PrecommitAnswerYes, "1 capture"},
		{"the named route", root, visualObs(visualReceipt, capture("/", false), capture("/x", false)), PrecommitAnswerYes, "/"},
		{"every named route", two, visualObs(visualReceipt, capture("/", false), capture("/drill.html", false)), PrecommitAnswerYes, "2 capture"},
		{"no captures at all", whole, visualObs(visualReceipt), PrecommitAnswerNo, "no visual capture"},
		{"hand-written evidence has no receipt (V17)", whole, visualObs("", capture("/", false)), PrecommitAnswerNo, "receipt"},
		{"a named route missing", two, visualObs(visualReceipt, capture("/", false)), PrecommitAnswerNo, "/drill.html"},
		{"a named route blank", root, visualObs(visualReceipt, capture("/", true)), PrecommitAnswerNo, "blank"},
		{"only blank captures", whole, visualObs(visualReceipt, capture("/", true)), PrecommitAnswerNo, "blank"},
		{"a blank extra beside a good one", whole, visualObs(visualReceipt, capture("/", false), capture("/x", true)), PrecommitAnswerYes, "capture"},
	}
	for _, tc := range cases {
		s, ok := SettleRuleOnFreeze(tc.rule, tc.obs)
		if !ok || s.Answer != tc.want {
			t.Errorf("%s: ok=%t answer=%q (%s)", tc.name, ok, s.Answer, s.Excerpt)
			continue
		}
		if !strings.Contains(s.Excerpt, tc.inText) {
			t.Errorf("%s: excerpt %q lacks %q", tc.name, s.Excerpt, tc.inText)
		}
	}

	// The evidence refs name what the answer rests on.
	s, _ := SettleRuleOnFreeze(whole, visualObs(visualReceipt, capture("/", false)))
	joined := strings.Join(s.EvidenceRefs, " ")
	for _, want := range []string{"bundle:b1", "selfdev-operation:op1", visualReceipt, "artifact:sha256:"} {
		if !strings.Contains(joined, want) {
			t.Errorf("evidence refs %v lack %q", s.EvidenceRefs, want)
		}
	}

	// A freeze that did not observe the evidence (a replay) decides nothing.
	replay := visualObs(visualReceipt, capture("/", false))
	replay.VisualKnown = false
	if _, ok := SettleRuleOnFreeze(whole, replay); ok {
		t.Error("an unobserved freeze settled visual_captured")
	}
}

func TestSettleVisualCapturedAtTerminalAndVerify(t *testing.T) {
	rule := PrecommitRule{Kind: PrecommitRuleVisualCaptured, AssignmentID: "a1"}
	end := EngineeringTerminalObservation{Disposition: EngineeringAssignmentCompleted, At: time.Now()}

	// R5: a candidate froze but its freeze observation was lost.
	frozen := end
	frozen.CandidateFrozen = true
	if s, ok := SettleRuleAtTerminal(rule, frozen); !ok || s.Answer != PrecommitAnswerUnresolved {
		t.Fatalf("frozen without observation: %+v ok=%t", s, ok)
	}
	// No Freeze succeeded: the claim is false.
	if s, ok := SettleRuleAtTerminal(rule, end); !ok || s.Answer != PrecommitAnswerNo {
		t.Fatalf("no candidate: %+v ok=%t", s, ok)
	}
	// Verify decides nothing about screenshots.
	if _, ok := SettleRuleOnVerify(rule, EngineeringVerifyObservation{Decision: "pass"}); ok {
		t.Fatal("Verify settled visual_captured")
	}
}

func TestVisualCapturedIsInTheClosedVocabulary(t *testing.T) {
	found := false
	for _, k := range PrecommitRuleKinds {
		found = found || k == PrecommitRuleVisualCaptured
	}
	if !found || !strings.Contains(ruleKindList(), "visual_captured") {
		t.Fatal("visual_captured is not in PrecommitRuleKinds")
	}
}
