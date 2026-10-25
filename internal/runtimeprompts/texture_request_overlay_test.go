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

// Failure 19 (docs/problems/engineering-completion-path-cluster-2026-10-20.md):
// management's Cast spec is the only place a capsule's network mode comes
// from, and its prompt named only "forbidden", so the build round of a
// delegate rewrite was never cast with npm access. Failure modes: the prompt
// does not name "ecosystem_proxy"; it does not say the default on a delegate
// computer refuses npm; it names a value engineeringCastNetwork does not read.
func TestManagementOverlayNamesEveryCastNetworkMode(t *testing.T) {
	for _, labels := range []bool{false, true} {
		management := RLMManagementOverlay(RLMManagementOverlayOptions{IntakeLabels: labels})
		for _, want := range []string{
			`"network": "ecosystem_proxy"`,
			`"network": "forbidden"`,
			"npm",
			"Claude delegate",
		} {
			if !strings.Contains(management, want) {
				t.Errorf("management overlay (labels %v) lacks %q", labels, want)
			}
		}
	}
}

// Failure 25 (docs/problems/engineering-completion-path-cluster-2026-10-20.md):
// management refused to cast the build round because it believed it had to
// supply the carried patch itself. The overlay says the platform carries a
// failed round's changes into the next engineering Cast on its own, and that
// a report awaiting round two means: Cast the next round.
func TestManagementOverlaySaysThePlatformCarriesThePatch(t *testing.T) {
	for _, labels := range []bool{false, true} {
		management := RLMManagementOverlay(RLMManagementOverlayOptions{IntakeLabels: labels})
		for _, want := range []string{
			"/selfdev/carried/source.patch",
			"the platform carries",
			"never supply",
			"Cast the next round",
			"rejected or fails before it applies",
		} {
			if !strings.Contains(management, want) {
				t.Errorf("management overlay (labels %v) lacks %q", labels, want)
			}
		}
	}
}

// P1 of trace review round 4 (docs/evidence/selfdev-trace-reviews-2026-10-25.md):
// the owner's correction arrived in a Texture re-cast while the engineer was
// bound, and management (run 665fbdcf) said the round stays open and sent no
// Note, so the candidate kept the stale spec. Failure modes: the overlay does
// not say to forward a spec change to the bound engineer; it says so but lets
// management summarize the change; it lets management act on other records
// first; it reads as applying only once the assignment ends.
func TestManagementOverlayForwardsASpecChangeToTheBoundEngineer(t *testing.T) {
	for _, labels := range []bool{false, true} {
		management := RLMManagementOverlay(RLMManagementOverlayOptions{IntakeLabels: labels})
		for _, want := range []string{
			"changes the spec of an implementation assignment that is still open",
			"owner direction",
			"choir.Note to the bound engineer (engineering:<assignment>) carrying the change in full",
			"before anything else",
			"even while the assignment is bound and running",
			"never reaches it",
			// R5-2: a re-cast of a bound request is refused, not a second assignment.
			"A Cast to engineering for a request whose assignment is still open is refused",
		} {
			if !strings.Contains(management, want) {
				t.Errorf("management overlay (labels %v) lacks %q", labels, want)
			}
		}
	}
}
