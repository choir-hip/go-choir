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

// Failure 28, superseded by the source-only freeze (docs/design/
// source-only-freeze-and-candidate-shots-2026-11-06.md 3.1): Freeze ships the
// source and the host builds it, so no change is told to build or stage
// bin/autoputer, and no recipe says to build it on a refusal. Failure modes:
// the overlay still sends a frontend change to build the binary on a Freeze
// refusal; it still stages a release tree.
func TestEngineeringOverlaySaysAPlatformChangeNeverBuildsTheBinary(t *testing.T) {
	overlay := RLMEngineeringOverlay(RLMEngineeringOverlayOptions{HasSelfDevelopmentOperation: true})
	for _, absent := range []string{
		"build bin/autoputer from the workspace source as the second job",
		"the running binary was built from",
		"FREEZE REQUIRES THE BUILT RELEASE TREE",
		"Freeze stages this computer's running binary",
	} {
		if strings.Contains(overlay, absent) {
			t.Errorf("the overlay still says %q", absent)
		}
	}
	for _, want := range []string{"A platform change never builds bin/autoputer", "never stages a release tree", "never sets GOPROXY=off", "FREEZE SHIPS YOUR SOURCE"} {
		if !strings.Contains(overlay, want) {
			t.Errorf("the overlay does not say %q", want)
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
	frontend := overlay[strings.Index(overlay, "Check a frontend change"):strings.Index(overlay, "Check a Go change")]
	for _, want := range []string{"frontend check job", "package checks"} {
		if !strings.Contains(frontend, want) {
			t.Errorf("the frontend branch does not say %q", want)
		}
	}
}
