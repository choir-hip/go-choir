package runtimeprompts

import (
	"strings"
	"testing"
)

// The partial trap and the early follow-up
// (docs/evidence/selfdev-trace-reviews-2026-10-25.md, Round 2). Ways the
// text can fail: engineering's overlay does not say partial is not a fate,
// or that a partial after a Freeze is refused and which results end the
// assignment; management's overlay does not tell it to wait for the answer
// to its question, or the assignment's final Report, before casting a
// follow-up for the same request.
func TestEngineeringOverlaySaysPartialAfterFreezeIsRefused(t *testing.T) {
	overlay := RLMEngineeringOverlay(RLMEngineeringOverlayOptions{HasSelfDevelopmentOperation: true})
	for _, want := range []string{
		"it is not a terminal fate",
		"Once you have frozen a candidate, partial is refused",
		`end with result="completed" to submit it`,
	} {
		if !strings.Contains(overlay, want) {
			t.Errorf("the engineering overlay does not say %q", want)
		}
	}
}

func TestManagementOverlayWaitsForTheAnswerBeforeAFollowUp(t *testing.T) {
	overlay := RLMManagementOverlay(RLMManagementOverlayOptions{})
	for _, want := range []string{
		"Do not Cast a follow-up for a request while the assignment you cast for it is still open and a question you sent it",
		"wait for its answer or its final Report, then decide",
	} {
		if !strings.Contains(overlay, want) {
			t.Errorf("the management overlay does not say %q", want)
		}
	}
}
