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
	if len(none) != 2 || none[0] != ClassRequest || none[1] != ClassAddSource {
		t.Fatalf("no document: the plain request (a new Texture) is the default, then add-source, which opens no Texture: %v", none)
	}
	for _, n := range owned {
		if n == ClassAddSource {
			t.Fatal("request kinds are offered only with no document in view")
		}
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

// Owner, 2026-11-16: the prompt bar router decides whether to open a new
// Texture or work in an existing one, and a new Texture has one way to open.
// With no document in view the only class that opens a Texture is request;
// add_source opens none (it adds a feed). No class names a kind of ask.
func TestTheRouterOffersOneNewTextureClassAndNoKindOfAsk(t *testing.T) {
	for _, state := range []DocState{DocNone, DocOwned, DocForeign} {
		for _, c := range ClassCandidates(state) {
			switch c.Desk {
			case ClassEdit, ClassComment, ClassRequest, ClassAddSource:
			default:
				t.Errorf("state %s offers class %q: a class that is neither a revision of an existing document, a new Texture, nor add-source", state, c.Desk)
			}
			if strings.Contains(c.Desk, "ask") {
				t.Errorf("state %s offers an ask kind %q", state, c.Desk)
			}
		}
	}
}
