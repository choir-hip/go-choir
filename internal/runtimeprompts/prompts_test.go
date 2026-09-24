package runtimeprompts

import (
	"strings"
	"testing"
)

func TestResearcherRuntimeOverlayIncludesParallelSaturation(t *testing.T) {
	overlay := ResearcherRuntimeOverlay()
	for _, want := range []string{
		"parallel tool-call block",
		"Send another update_coagent after each additional search/fetch batch",
		"persistent communicating coagent",
	} {
		if !strings.Contains(overlay, want) {
			t.Fatalf("researcher runtime overlay missing %q: %q", want, overlay)
		}
	}
}

func TestSuperRuntimeOverlayIncludesAuthorityBoundary(t *testing.T) {
	overlay := SuperRuntimeOverlay()
	if !strings.Contains(overlay, "Management authority boundary") {
		t.Fatalf("management runtime overlay missing authority boundary: %q", overlay)
	}
}

func TestRLMCoSuperOverlayGatesFreezeMandate(t *testing.T) {
	const mandate = "Freeze the capsule diff with choir.Freeze."
	if overlay := RLMCoSuperOverlay(RLMCoSuperOverlayOptions{}); strings.Contains(overlay, mandate) {
		t.Fatalf("document-cast overlay mandates Freeze: %q", overlay)
	}
	if overlay := RLMCoSuperOverlay(RLMCoSuperOverlayOptions{HasSelfDevelopmentOperation: true}); !strings.Contains(overlay, mandate) {
		t.Fatalf("self-development overlay omits Freeze mandate: %q", overlay)
	}
}
