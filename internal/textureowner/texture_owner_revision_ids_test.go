package textureowner

import (
	"strings"
	"testing"
)

// Texture names the owner's request by its revision so a cast carries it
// verbatim (docs/problems/texture-retypes-owner-directions-2026-10-25.md,
// chosen fix). Ways it can fail, written before the code: the request
// turn prints the original request with no revision id, so a cell can only
// retype it; the id is not the document's first revision; the id prints
// where no root revision was read (a seed carried in metadata names none).
func TestOriginalRequestNamesItsRevision(t *testing.T) {
	prompt := wakeAfterTheRequestWasRewritten(t, "ids", holdSwitches{hold: true, directions: true}).Prompt
	want := originalLabel + holdRequest + "\n(revision revision-hold-root-ids)"
	if !strings.Contains(prompt, want) {
		t.Fatalf("the original request does not name its revision; want %q in:\n%s", want, prompt)
	}

	off := wakeAfterTheRequestWasRewritten(t, "ids-off", holdSwitches{directions: true}).Prompt
	if strings.Contains(off, "revision-hold-root-ids-off)") {
		t.Fatalf("a request read from no root revision names one:\n%s", off)
	}
}
