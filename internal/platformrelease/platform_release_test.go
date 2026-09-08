package platformrelease

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGenerateBaselineRelease(t *testing.T) {
	ts := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	commit := "653f0105abcdef1234567890abcdef1234567890"

	pr := GenerateBaselineRelease(commit, ts)
	if pr.ReleaseID != "pr-20260908-653f0105" {
		t.Fatalf("unexpected release_id: %s", pr.ReleaseID)
	}
	if pr.PlatformBaseRef != "refs/heads/main@"+commit {
		t.Fatalf("unexpected platform_base_ref: %s", pr.PlatformBaseRef)
	}
	if len(pr.Components) == 0 {
		t.Fatalf("expected default components, got 0")
	}
	if !strings.HasPrefix(pr.ContentDigest, "sha256:") {
		t.Fatalf("expected sha256 content digest, got: %s", pr.ContentDigest)
	}

	if err := Validate(pr); err != nil {
		t.Fatalf("generated release failed validation: %v", err)
	}
}

func TestContentDigestDeterminism(t *testing.T) {
	ts := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	commit := "653f0105abcdef1234567890abcdef1234567890"

	pr1 := GenerateBaselineRelease(commit, ts)
	pr2 := GenerateBaselineRelease(commit, ts)

	// Swap component order in pr2 to verify sort-order independence
	pr2.Components[0], pr2.Components[len(pr2.Components)-1] = pr2.Components[len(pr2.Components)-1], pr2.Components[0]

	d1 := ComputeContentDigest(pr1)
	d2 := ComputeContentDigest(pr2)

	if d1 != d2 {
		t.Fatalf("digests differ despite identical canonical content: %s != %s", d1, d2)
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "platform-release.json")

	ts := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	pr := GenerateBaselineRelease("3f2a874a1234567890abcdef1234567890abcdef", ts)

	if err := Save(pr, path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.ReleaseID != pr.ReleaseID {
		t.Fatalf("loaded release_id mismatch: %s != %s", loaded.ReleaseID, pr.ReleaseID)
	}
	if loaded.ContentDigest != pr.ContentDigest {
		t.Fatalf("loaded digest mismatch: %s != %s", loaded.ContentDigest, pr.ContentDigest)
	}
}

func TestComputerLineage(t *testing.T) {
	tracking := &ComputerLineage{
		ComputerID:           "computer-1",
		PlatformBaseRef:      "main@commit1",
		DivergenceStatus:     DivergenceTracking,
		PlatformFollowPolicy: FollowPolicyAuto,
	}
	if !tracking.IsTracking() {
		t.Fatalf("expected tracking computer to be tracking")
	}
	if !tracking.CanFastForwardComponent("email") {
		t.Fatalf("expected clean tracking computer to fast-forward email")
	}

	divergent := &ComputerLineage{
		ComputerID:           "computer-2",
		PlatformBaseRef:      "main@commit1",
		DivergenceStatus:     DivergenceDivergent,
		DivergedComponents:   []string{"texture"},
		PlatformFollowPolicy: FollowPolicyAuto,
	}
	if divergent.IsTracking() {
		t.Fatalf("expected divergent computer not to report IsTracking() == true")
	}
	if !divergent.CanFastForwardComponent("email") {
		t.Fatalf("expected divergent computer to fast-forward un-diverged email")
	}
	if divergent.CanFastForwardComponent("texture") {
		t.Fatalf("expected divergent computer NOT to fast-forward diverged texture")
	}

	canary := &ComputerLineage{
		ComputerID:           "computer-3",
		PlatformBaseRef:      "main@commit1",
		DivergenceStatus:     DivergenceCanary,
		PlatformFollowPolicy: FollowPolicyPinned,
	}
	if canary.CanFastForwardComponent("email") {
		t.Fatalf("expected canary computer NOT to fast-forward email")
	}
}
