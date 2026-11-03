package types

import (
	"strings"
	"testing"
)

// visual_captured read the platform capture, which is removed
// (docs/problems/selfdev-screens-are-not-files-2026-11-03.md). A new one is
// refused; one already on a tape must close, never wait: at Freeze and at the
// terminal it settles unresolved with the reason.
func TestVisualCapturedIsRetired(t *testing.T) {
	p := CommitmentPrecommit{Question: "q", Resolver: PrecommitRuleResolver, Distribution: map[string]float64{"yes": 0.5, "no": 0.5},
		Rule: &PrecommitRule{Kind: precommitRuleVisualCapturedRetired}}
	if err := ValidateRulePrecommit(p); err == nil {
		t.Fatal("a new visual_captured precommit was admitted")
	}
	for _, k := range PrecommitRuleKinds {
		if k == precommitRuleVisualCapturedRetired {
			t.Fatal("visual_captured is still in the vocabulary")
		}
	}
	rule := PrecommitRule{Kind: precommitRuleVisualCapturedRetired, Routes: []string{"/"}}
	s, ok := SettleRuleOnFreeze(rule, EngineeringFreezeObservation{})
	if !ok || s.Answer != PrecommitAnswerUnresolved || !strings.Contains(s.Excerpt, "retired") {
		t.Fatalf("at freeze: %+v %v", s, ok)
	}
	for _, frozen := range []bool{true, false} {
		s, ok = SettleRuleAtTerminal(rule, EngineeringTerminalObservation{Disposition: EngineeringAssignmentCompleted, CandidateFrozen: frozen})
		if !ok || s.Answer != PrecommitAnswerUnresolved || !strings.Contains(s.Excerpt, "retired") {
			t.Fatalf("at terminal (frozen %v): %+v %v", frozen, s, ok)
		}
	}
}
