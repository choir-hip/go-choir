package runtimeprompts

import (
	"strings"
	"testing"
)

func TestResearchRuntimeOverlayIncludesParallelSaturation(t *testing.T) {
	overlay := ResearchRuntimeOverlay()
	for _, want := range []string{
		"Use web_search and fetch_url with parallelism appropriate to the task",
		"Research reports claims, evidence, uncertainty, and blockers",
	} {
		if !strings.Contains(overlay, want) {
			t.Fatalf("research runtime overlay missing %q: %q", want, overlay)
		}
	}
	if strings.Contains(overlay, "update_"+"coagent") {
		t.Fatalf("research runtime overlay names deleted tool: %q", overlay)
	}
}

func TestManagementRuntimeOverlayIncludesAuthorityBoundary(t *testing.T) {
	overlay := ManagementRuntimeOverlay()
	for _, want := range []string{
		"Management is the persistent root desk",
		"report_to_texture",
	} {
		if !strings.Contains(overlay, want) {
			t.Fatalf("management runtime overlay missing %q: %q", want, overlay)
		}
	}
}

func TestRLMEngineeringOverlayGatesFreezeMandate(t *testing.T) {
	const mandate = "Freeze the capsule diff with choir.Freeze."
	if overlay := RLMEngineeringOverlay(RLMEngineeringOverlayOptions{}); strings.Contains(overlay, mandate) {
		t.Fatalf("document-cast overlay mandates Freeze: %q", overlay)
	}
	if overlay := RLMEngineeringOverlay(RLMEngineeringOverlayOptions{HasSelfDevelopmentOperation: true}); !strings.Contains(overlay, mandate) {
		t.Fatalf("self-development overlay omits Freeze mandate: %q", overlay)
	}
}
