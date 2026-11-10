package promptroute

import (
	"strings"
	"testing"
)

// Lane D, the input class. Ways it could fail (see
// docs/evidence/prompt-bar-universal-input-failures-2026-11-10.md):
//  D2 edit is offered on a document the owner does not own;
//  D1 the default is not deterministic, or abstaining loses the one-tap choices.

func classNames(cs []Candidate) []string {
	out := []string{}
	for _, c := range cs {
		out = append(out, c.Desk)
	}
	return out
}

func TestClassCandidatesDependOnOwnership(t *testing.T) {
	owned := classNames(ClassCandidates(DocOwned))
	if len(owned) != 3 || owned[0] != ClassEdit {
		t.Fatalf("owned = %v", owned)
	}
	foreign := classNames(ClassCandidates(DocForeign))
	if len(foreign) != 2 || foreign[0] != ClassComment {
		t.Fatalf("D2: foreign = %v", foreign)
	}
	for _, n := range foreign {
		if n == ClassEdit {
			t.Fatal("D2: edit offered on a document the owner does not own")
		}
	}
	none := classNames(ClassCandidates(DocNone))
	if len(none) != 1 || none[0] != ClassRequest {
		t.Fatalf("no document: %v", none)
	}
}

func TestClassDecideAbstainsToTheDefaultWithAlternatives(t *testing.T) {
	cs := ClassCandidates(DocOwned)
	d := Decide(cs, map[string]float64{ClassEdit: 0.4, ClassComment: 0.35, ClassRequest: 0.25}, Theta)
	if d.Destination != ClassEdit || d.Rule != RuleAbstain || len(d.Alternatives) == 0 {
		t.Fatalf("decision = %+v", d)
	}
	d = Decide(cs, map[string]float64{ClassEdit: 0.1, ClassComment: 0.2, ClassRequest: 0.9}, Theta)
	if d.Destination != ClassRequest || d.Rule != RuleJudge {
		t.Fatalf("decision = %+v", d)
	}
}

func TestClassEvidenceNamesOnlyTheCandidates(t *testing.T) {
	ev := ClassDestinations(ClassCandidates(DocForeign))
	if !strings.Contains(ev, "- "+ClassComment) || !strings.Contains(ev, "- "+ClassRequest) || strings.Contains(ev, "- "+ClassEdit) {
		t.Fatalf("evidence = %q", ev)
	}
}
