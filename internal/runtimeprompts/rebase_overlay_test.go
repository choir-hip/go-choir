package runtimeprompts

import (
	"strings"
	"testing"
)

// U4 (docs/design/u4-stack-rebase-build-2026-10-26.md, slice 5): management
// turns a root Texture Cast that names a rebase notice into the one engineering
// Cast the store's admission accepts. Ways it can fail: the overlay does not
// teach the spec fields, so management's Cast is refused as unbound; it does
// not say the objective must contain the notice id; it invites new work in a
// rebase round.
func TestManagementOverlayTeachesTheBoundRebaseCast(t *testing.T) {
	overlay := RLMManagementOverlay(RLMManagementOverlayOptions{})
	for _, want := range []string{
		"rebase_notice", "rebase_of", "\"base\"", "rebase-notice:<operation>:<base>:<n>",
		"an objective that contains the notice id", "adding no new work", "Any other spec at that moment is refused",
	} {
		if !strings.Contains(overlay, want) {
			t.Errorf("the management overlay does not say %q", want)
		}
	}
}
