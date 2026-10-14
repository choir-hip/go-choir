package runtimeprompts

import (
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/capsule"
)

func TestRLMEngineeringOverlayGatesFreezeMandate(t *testing.T) {
	const mandate = "Freeze the capsule diff with choir.Freeze."
	if overlay := RLMEngineeringOverlay(RLMEngineeringOverlayOptions{}); strings.Contains(overlay, mandate) {
		t.Fatalf("document-cast overlay mandates Freeze: %q", overlay)
	}
	if overlay := RLMEngineeringOverlay(RLMEngineeringOverlayOptions{HasSelfDevelopmentOperation: true}); !strings.Contains(overlay, mandate) {
		t.Fatalf("self-development overlay omits Freeze mandate: %q", overlay)
	}
}

// Gate 2 track D: with no pre-created operation, an armed computer still lets
// an ordinary assignment propose its change by freezing it; an unarmed one
// does not mention it.
func TestRLMEngineeringOverlayOffersProposalWhenArmed(t *testing.T) {
	const offer = "To propose a change meant to land on this computer, freeze it with choir.Freeze"
	if overlay := RLMEngineeringOverlay(RLMEngineeringOverlayOptions{}); strings.Contains(overlay, offer) {
		t.Fatalf("unarmed overlay offers proposal: %q", overlay)
	}
	if overlay := RLMEngineeringOverlay(RLMEngineeringOverlayOptions{CanProposeChange: true}); !strings.Contains(overlay, offer) {
		t.Fatalf("armed overlay omits the proposal offer: %q", overlay)
	}
}

// Support judgement left management
// (docs/reports/support-judgement-to-jev-design-2026-10-14.md): Jev judges a
// Texture commit's citation pairs off management's turn, so the overlay
// must not teach Judge, support_pairs records or judging pairs by hand.
func TestRLMManagementOverlayCarriesNoSupportJudgement(t *testing.T) {
	for _, labels := range []bool{false, true} {
		overlay := RLMManagementOverlay(RLMManagementOverlayOptions{IntakeLabels: labels})
		for _, absent := range []string{"choir.Judge", "support_pairs", "judge yourself", "Support judgement"} {
			if strings.Contains(overlay, absent) {
				t.Errorf("management overlay (labels %v) still says %q", labels, absent)
			}
		}
	}
}

// The overlay must teach the release root the freeze actually reads. The C1e
// B1 drill (docs/problems/c1e-b1-engineering-assignments-cancelled-on-arrival-2026-10-14.md)
// staged the tree under /workspace/platform because the overlay said so, and
// Freeze refused with "capsule release contains no frozen runtime artifacts".
//
// Failure modes: the overlay names a root the executor does not read; the
// overlay places the release tree inside the workspace; the executor's
// prefix moves and the overlay keeps the old one.
func TestRLMEngineeringOverlayNamesTheReleaseRootFreezeReads(t *testing.T) {
	root := "/" + capsule.ReleaseTreePrefix
	overlay := RLMEngineeringOverlay(RLMEngineeringOverlayOptions{HasSelfDevelopmentOperation: true})
	if !strings.Contains(overlay, "at the capsule root, "+root) {
		t.Fatalf("overlay does not name the capsule-root release tree %s: %q", root, overlay)
	}
	if !strings.Contains(overlay, "go build -o "+root+"bin/autoputer") {
		t.Fatalf("overlay does not build bin/autoputer into %s", root)
	}
	if strings.Contains(overlay, capsule.ReleaseTreePrefix+" inside /workspace/platform") {
		t.Fatalf("overlay places the release tree inside the workspace: %q", overlay)
	}
}
