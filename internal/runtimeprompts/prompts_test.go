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

// A change with no Go input ships the running binary (docs/problems/
// selfdev-drill-capsule-cannot-build-autoputer-2026-10-14.md, option A): the
// overlay must stop telling the desk to build bin/autoputer for a
// frontend-only change, must name the Go inputs the executor classifies,
// and must keep the build instruction for a Go change. Failure modes: the
// overlay still demands a build for every change (the drill spends its
// window linking); the overlay tells the desk to copy a binary itself; the
// overlay's Go-input list drifts from capsule.BinaryInputPath.
func TestRLMEngineeringOverlayTeachesBinaryReuseForNonGoChanges(t *testing.T) {
	overlay := RLMEngineeringOverlay(RLMEngineeringOverlayOptions{HasSelfDevelopmentOperation: true})
	if strings.Contains(overlay, "a frontend-only change still ships the binary") {
		t.Fatalf("overlay still demands a build for a frontend-only change")
	}
	for _, want := range []string{"touches no Go input", "Freeze stages this computer's running binary", "Never stage a stub"} {
		if !strings.Contains(overlay, want) {
			t.Errorf("overlay does not say %q", want)
		}
	}
	for _, rel := range []string{"cmd/", "internal/", "vendor/", "nix/", "go.mod", "go.sum", "flake.nix", "flake.lock"} {
		if !strings.Contains(overlay, rel) {
			t.Errorf("overlay does not name Go input %q", rel)
		}
		probe := strings.TrimSuffix(rel, "/")
		if strings.HasSuffix(rel, "/") {
			probe = rel + "x.go"
		}
		if !capsule.BinaryInputPath(probe) {
			t.Errorf("overlay names %q as a Go input but the executor does not classify %q as one", rel, probe)
		}
	}
	if !capsule.BinaryInputPath("frontend/tools/gen.go") || capsule.BinaryInputPath("internal/x/y_test.go") {
		t.Fatalf("executor classification drifted from the overlay's .go and _test.go rules")
	}
}

// Research searches the computer's own corpus before the web (symptom 3 of
// docs/problems/prompt-bar-research-slow-unsynthesized-mangled-2026-10-14.md):
// the overlay's evidence doctrine names choir.SearchWireCorpus before it
// teaches web search, and says a capture it returns is citable.
func TestRLMResearchOverlaySearchesTheCorpusFirst(t *testing.T) {
	overlay := RLMResearchOverlay()
	at := strings.Index(overlay, "Evidence Doctrine")
	if at < 0 {
		t.Fatalf("overlay has no evidence doctrine: %q", overlay)
	}
	doctrine := overlay[at:]
	corpus, web := strings.Index(doctrine, "choir.SearchWireCorpus("), strings.Index(doctrine, "choir.SearchStart")
	if corpus < 0 || web < 0 || corpus > web {
		t.Fatalf("evidence doctrine does not search the corpus before the web: %q", doctrine)
	}
	if !strings.Contains(doctrine, "capture_id") {
		t.Fatalf("evidence doctrine does not say a corpus capture is citable by its capture_id: %q", doctrine)
	}
}
