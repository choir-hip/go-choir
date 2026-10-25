package runtimeprompts

import (
	"strings"
	"testing"
)

// N5 of docs/evidence/selfdev-trace-reviews-2026-10-25.md (Round 3): 9d5d's
// two Freezes were refused, first for a build job during which the workspace
// changed, then for a capture of an earlier frontend build. The overlay
// states the order that ends a round: the build, then (with visual capture)
// the capture after the last build, then Freeze, then Complete(completed).
func TestEngineeringOverlayStatesTheRoundEndOrder(t *testing.T) {
	plain := RLMEngineeringOverlay(RLMEngineeringOverlayOptions{HasSelfDevelopmentOperation: true})
	for _, want := range []string{
		`then the test cell or job, then choir.Freeze, then choir.Complete(result="completed")`,
		"write nothing under /workspace/platform while a build or test job runs",
	} {
		if !strings.Contains(plain, want) {
			t.Errorf("the overlay lacks %q", want)
		}
	}
	if strings.Contains(plain, "choir-capture") {
		t.Error("an ungranted computer's overlay names the capture")
	}
	granted := RLMEngineeringOverlay(RLMEngineeringOverlayOptions{HasSelfDevelopmentOperation: true, VisualCapture: true})
	for _, want := range []string{
		`End the round in this order: the last build job, then the choir-capture job, then choir.Freeze, then choir.Complete(result="completed")`,
		"Freeze refuses a capture from an earlier build",
	} {
		if !strings.Contains(granted, want) {
			t.Errorf("the granted overlay lacks %q", want)
		}
	}
}
