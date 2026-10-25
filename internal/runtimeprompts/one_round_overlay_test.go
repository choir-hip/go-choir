package runtimeprompts

import (
	"strings"
	"testing"
)

// Failure 27 (docs/problems/engineering-completion-path-cluster-2026-10-20.md):
// management's overlay still said a delegate capsule cannot run a frontend
// build, and management withheld the network, so no delegate ran. The
// overlay must say the delegate's round builds a frontend change whose
// package files are unchanged, and that a delegate rewrite leaves the
// network field out.
func TestManagementOverlaySaysTheDelegateRoundBuilds(t *testing.T) {
	overlay := RLMManagementOverlay(RLMManagementOverlayOptions{})
	if strings.Contains(overlay, "no npm install or frontend build runs there") {
		t.Error("the overlay still says a delegate capsule cannot build the frontend")
	}
	for _, want := range []string{
		"builds a frontend change whose package.json and package-lock.json are unchanged",
		"leave \"network\" out",
	} {
		if !strings.Contains(overlay, want) {
			t.Errorf("the overlay does not say %q", want)
		}
	}
}

// Failure 28: Freeze refused to reuse the running binary because it was
// built from another commit than the capsule source, and the overlay said
// a frontend-only change never builds bin/autoputer. The no-Go branch must
// say what to do on that refusal.
func TestEngineeringOverlaySaysWhenAFrontendChangeBuildsTheBinary(t *testing.T) {
	overlay := RLMEngineeringOverlay(RLMEngineeringOverlayOptions{HasSelfDevelopmentOperation: true})
	noGo := overlay[strings.Index(overlay, "touches no Go input"):strings.Index(overlay, "A change that touches a Go input")]
	for _, want := range []string{
		"the running binary was built from",
		"build bin/autoputer from the workspace source as the second job",
		"then Freeze again",
	} {
		if !strings.Contains(noGo, want) {
			t.Errorf("the no-Go branch does not say %q", want)
		}
	}
}

// Problems 2, 3, 4 and 5 of docs/evidence/selfdev-trace-reviews-2026-10-25.md:
// round 1 re-ran a 269 s failing test five times when a 1 s probe would have
// shown the cause, bounded a job with `timeout`, which the capsule's seccomp
// breaks (timer_create), was cancelled by the activation budget it never
// saw, and ran Go tests for a frontend-only change on a cold module cache.
func TestEngineeringOverlayTeachesBoundedJobsAndTheBudget(t *testing.T) {
	overlay := RLMEngineeringOverlay(RLMEngineeringOverlayOptions{HasSelfDevelopmentOperation: true})
	for _, want := range []string{
		"smoke check that finishes in under 10 s",
		"timer_create",
		"bound every step yourself",
		"no new output for 60 s",
		"activation budget",
		"Freeze what you have",
		"output tail",
	} {
		if !strings.Contains(overlay, want) {
			t.Errorf("the overlay does not say %q", want)
		}
	}
	noGo := overlay[strings.Index(overlay, "touches no Go input"):strings.Index(overlay, "A change that touches a Go input")]
	for _, want := range []string{"no go test", "frontend checks"} {
		if !strings.Contains(noGo, want) {
			t.Errorf("the no-Go branch does not say %q", want)
		}
	}
}
