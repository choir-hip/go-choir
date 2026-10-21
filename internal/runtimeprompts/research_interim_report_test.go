package runtimeprompts

import (
	"strings"
	"testing"
)

// Research reports as it goes, the ordinary policy
// (docs/problems/research-more-slow-and-hidden-2026-10-21.md E5): in both
// traced asks research sent one packet, after every search and read, and
// Texture waited minutes for it.
func TestResearchOverlaySendsTheFirstPacketAfterTheFirstSource(t *testing.T) {
	overlay := RLMResearchOverlay()
	for _, want := range []string{"as soon as you have read one new source you can cite", "do not hold sources back"} {
		if !strings.Contains(overlay, want) {
			t.Fatalf("research overlay missing %q", want)
		}
	}
}
