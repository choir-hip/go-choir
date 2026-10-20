package runtimeprompts

import (
	"strings"
	"testing"
)

// Engineering reads the Texture request in the Cast that opened its assignment
// (docs/problems/claude-delegate-resume-sonnet-2026-10-20.md, "Fix design");
// management is told it does, so its objective adds or narrows instead of
// restating. Failure modes: engineering is not told where the request is, so
// it asks for a task it already holds; management keeps summarising a request
// engineering can already read.
func TestOverlaysSayWhereTheTextureRequestIs(t *testing.T) {
	engineering := RLMEngineeringOverlay(RLMEngineeringOverlayOptions{HasSelfDevelopmentOperation: true})
	for _, want := range []string{"request.from, request.objective, request.body", "Texture request it serves, verbatim", "management's objective says what it adds or narrows"} {
		if !strings.Contains(engineering, want) {
			t.Errorf("engineering overlay lacks %q", want)
		}
	}
	for _, labels := range []bool{false, true} {
		management := RLMManagementOverlay(RLMManagementOverlayOptions{IntakeLabels: labels})
		if want := "Engineering receives the Texture request verbatim in the Cast it reads, so make the objective say what you add or narrow"; !strings.Contains(management, want) {
			t.Errorf("management overlay (labels %v) lacks %q", labels, want)
		}
	}
}
