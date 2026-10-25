package runtimeprompts

import (
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/visualcapture"
)

// The capture and review sections (docs/design/visual-evidence-slice-2026-
// 10-15.md parts 2 and 4, V24, V25, V29). They appear only on a computer the
// host granted visual capture, because teaching a tool that refuses is worse
// than silence; and what they teach must be what the runtime enforces.

func TestEngineeringOverlayTeachesVisualCaptureOnlyWhenGranted(t *testing.T) {
	plain := RLMEngineeringOverlay(RLMEngineeringOverlayOptions{HasSelfDevelopmentOperation: true})
	for _, absent := range []string{visualcapture.ToolName, "visual_captured", "visual_captures"} {
		if strings.Contains(plain, absent) {
			t.Errorf("an ungranted computer's overlay teaches %q", absent)
		}
	}
	granted := RLMEngineeringOverlay(RLMEngineeringOverlayOptions{HasSelfDevelopmentOperation: true, VisualCapture: true})
	for _, want := range []string{
		// The call, as the executor admits it: a job, with its memory.
		`choir.Start([]string{"choir-capture", "--route"`, `"memory_mib"`,
		// Where it runs in the sequence, and why.
		"after the frontend build", "last build", "capture again",
		// The rule and its limits.
		`"kind": "visual_captured"`, "never says the picture is right",
		// What Freeze returns, and the bounds the tool enforces.
		"visual_captures", "at most 6 routes", "1280", "blank",
		// V14: the capsule needs loopback.
		"ecosystem network",
		// V16 as the app shell reads it: a signed-out session, not a 503.
		"/auth/session answers as a signed-out session", "signed-out desktop",
		// Problem 7: an extensionless route is index.html; the warning.
		"by its file", "visual_warnings",
	} {
		if !strings.Contains(granted, want) {
			t.Errorf("granted overlay lacks %q", want)
		}
	}
	if strings.Contains(granted, "{{") {
		t.Fatal("unrendered template in the granted overlay")
	}
	if strings.Contains(granted, "/auth/ answer 503") {
		t.Fatal("the overlay still says /auth/ answers 503")
	}
	// The granted text does not change anything else in the overlay.
	if !strings.Contains(granted, "Precommit before you build") || !strings.Contains(granted, "FREEZE REQUIRES THE BUILT RELEASE TREE") {
		t.Fatal("the granted overlay lost the base text")
	}
}

func TestManagementOverlayTeachesVisualReviewOnlyWhenGranted(t *testing.T) {
	plain := RLMManagementOverlay(RLMManagementOverlayOptions{})
	if strings.Contains(plain, "visual-evidence") || strings.Contains(plain, "verifier_multimodal") {
		t.Fatal("an ungranted computer's management overlay teaches visual review")
	}
	granted := RLMManagementOverlay(RLMManagementOverlayOptions{VisualReview: true})
	for _, want := range []string{
		// Where to read the captures, and how to put one to a vision model.
		"/visual-evidence", "/api/model-policy/verify", "verifier_multimodal", `"image_ref"`,
		// V24: the answer is input to the Verify, cited, not a settlement.
		"cite", "not a settlement",
		// V25: bounded in the prompt, since no runtime cap exists yet.
		"one vision call",
		// The owner's review is in Texture; the desk does not build a view.
		"Texture",
	} {
		if !strings.Contains(granted, want) {
			t.Errorf("granted management overlay lacks %q", want)
		}
	}
	if strings.Contains(granted, "{{") {
		t.Fatal("unrendered template in the granted management overlay")
	}
}
