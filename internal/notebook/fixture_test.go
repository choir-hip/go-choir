package notebook

import (
	"os"
	"strings"
	"testing"
)

// The phone's notebook fixture (mobile/fixtures/learning/notebook.md, written
// by lane learn-phone) must parse with the server's grammar, line for line, and
// render back to itself, so the two sides cannot drift.
func TestThePhonesNotebookFixtureParses(t *testing.T) {
	raw, err := os.ReadFile("../../mobile/fixtures/learning/notebook.md")
	if err != nil {
		t.Fatal(err)
	}
	d := Parse(string(raw))
	asked, learned, trying, steer := d.Lines(SecAsked), d.Lines(SecLearned), d.Lines(SecTrying), d.Lines(SecSteering)
	if len(asked) != 3 || len(learned) != 2 || len(trying) != 1 || len(steer) != 2 {
		t.Fatalf("lines: %d %d %d %d", len(asked), len(learned), len(trying), len(steer))
	}
	if learned[0].Tag != "L1" || learned[0].Heard != 9 || learned[0].Of != 11 || learned[1].Tag != "L9" || trying[0].Tag != "T2" || trying[0].Of != 3 {
		t.Errorf("tags: %+v %+v", learned, trying)
	}
	if asked[0].Ref() != MineRef("Mornings: world news first, then science.") || learned[0].Ref() != "lesson:L1" || trying[0].Ref() != "try:T2" {
		t.Errorf("refs: %q %q %q", asked[0].Ref(), learned[0].Ref(), trying[0].Ref())
	}
	for _, l := range append(append([]Line{}, learned...), trying...) {
		if err := CheckText(l.Text); err != nil {
			t.Errorf("%q: %v", l.Text, err)
		}
	}
	if got := d.Render(); strings.TrimSpace(got) != strings.TrimSpace(string(raw)) {
		t.Errorf("render differs:\n%s", got)
	}
	if d.WrittenWords() > HardWords {
		t.Errorf("fixture over the cap")
	}
}
