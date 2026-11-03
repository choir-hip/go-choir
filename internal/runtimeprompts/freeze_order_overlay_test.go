package runtimeprompts

import (
	"strings"
	"testing"
)

// N5 of docs/evidence/selfdev-trace-reviews-2026-10-25.md (Round 3): the
// overlay states the order that ends a round: the build, then the test,
// then Freeze, then Complete(completed).
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
}

// Screens (docs/problems/selfdev-screens-are-not-files-2026-11-03.md): every
// engineering overlay says where to put screenshots so they become files
// Texture reads, and no overlay teaches the removed platform capture.
func TestOverlaysTeachScreensAsFilesAndNoPlatformCapture(t *testing.T) {
	for _, opts := range []RLMEngineeringOverlayOptions{{}, {HasSelfDevelopmentOperation: true}, {ClaudeDelegate: true}} {
		eng := RLMEngineeringOverlay(opts)
		for _, want := range []string{"/var/lib/artifact/screens/", "Self-development/<operation>/screens/", "$CHOIR_BROWSER", "real content", ".png", "scripts/choir-shot.mjs", "--only", "choir.Edit"} {
			if !strings.Contains(eng, want) {
				t.Errorf("engineering overlay %+v lacks %q", opts, want)
			}
		}
		for _, absent := range []string{"choir-capture", "visual_captured", "visual_captures", "visual-evidence"} {
			if strings.Contains(eng, absent) {
				t.Errorf("engineering overlay %+v still names %q", opts, absent)
			}
		}
	}
	mgmt := RLMManagementOverlay(RLMManagementOverlayOptions{})
	for _, want := range []string{"Self-development/<operation_id>/screens/", "image_ref"} {
		if !strings.Contains(mgmt, want) {
			t.Errorf("management overlay lacks %q", want)
		}
	}
	for _, absent := range []string{"choir-capture", "visual-evidence", "granted visual capture"} {
		if strings.Contains(mgmt, absent) {
			t.Errorf("management overlay still names %q", absent)
		}
	}
}
