package agentcore

import (
	"strings"
	"testing"
)

// A requested text change that wraps differently is the change, not a
// regression (package-proof-1 on d@e, selfdev-f9b43e77: the judge failed
// "No readable image source is attached to this window yet." because the
// longer line sat wider and lower than the BEFORE line).
//
// Failure modes: the pair prompt drops the reflow exemption; the exemption
// leaks to shots that are not a before/after pair.
func TestShotReviewPromptExemptsTheRequestedChangesOwnReflow(t *testing.T) {
	pair := shotReviewPrompt([]string{"01-before-image-empty.png (the BEFORE of a " + beforeAfterMarker + ")", "01-after-image-empty.png (the AFTER of a " + beforeAfterMarker + ")"}, "")
	if !strings.Contains(pair, "may wrap, reflow, widen or shift") {
		t.Fatalf("pair prompt lacks the reflow exemption: %q", pair)
	}
	single := shotReviewPrompt([]string{"01-image-empty.png"}, "")
	if strings.Contains(single, "may wrap, reflow, widen or shift") {
		t.Fatalf("a shot with no pair carries the pair exemption")
	}
}
