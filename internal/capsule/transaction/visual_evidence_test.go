package transaction

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/capsule"
)

// V18 and V8 (docs/design/visual-evidence-slice-2026-10-15.md): a bundle
// with no captures canonicalizes exactly as before, and a bundle that cites
// captures binds them to the frontend it freezes and to a capture receipt
// shape; the content digest covers them.

func visualBundleBase() CapsuleEffectBundle {
	digest := repeatHex
	files := []capsule.FrozenReleaseFile{
		{Path: "bin/autoputer", SHA256: digest('d'), Mode: 0o555},
		{Path: "frontend/index.html", SHA256: digest('7'), Mode: 0o444},
	}
	return CapsuleEffectBundle{
		BundleVersion: 1, ComputerID: "computer-1", BaseEventHead: digest('a'),
		TrajectoryRef: "trajectory-1", CapsuleIdentity: "capsule-1", CapabilityPolicyDigest: digest('b'),
		SourceTreeRef:         "source-tree:sha256:" + digest('c'),
		OrderedFileEffects:    []ChangeRecord{{Path: "workspace/platform/frontend/public/x.html", Kind: "added", Mode: 0o644}},
		GeneratedArtifactRefs: []string{"artifact:sha256:" + digest('d')},
		BuildRecipeRef:        "capsule-go-eval:sha256:" + digest('e'),
		RuntimeArtifactRef:    "runtime-artifact:sha256:" + digest('f'),
		TestReceipts:          []string{"capsule-go-eval:sha256:" + digest('1')}, VerifierReceipts: []string{},
		DependencyToolchainRefs: []string{"capsule-go-eval:sha256:" + digest('2')},
		ResourceReceipts:        []string{"resource:sha256:" + digest('3')},
		RuntimeFiles:            files,
		Groups:                  map[string][]ChangeRecord{}, Ignored: []ChangeRecord{},
	}
}

func visualEvidenceFor(b CapsuleEffectBundle) *capsule.VisualEvidence {
	return &capsule.VisualEvidence{
		ManifestSHA256: repeatHex('8'), FrontendSHA256: capsule.ReleaseFrontendDigest(b.RuntimeFiles),
		CaptureReceipt: "capsule-exec:sha256:" + repeatHex('9'),
		Captures: []capsule.VisualEvidenceCapture{{
			Artifact: "artifact:sha256:" + repeatHex('c'), Route: "/", MediaType: "image/png", Bytes: 1234, Width: 1280, Height: 800,
		}},
	}
}

func validateDraft(b CapsuleEffectBundle) error {
	b.ContentDigest, _ = b.ComputeContentDigest()
	return b.Validate(false)
}

func TestBundleWithoutCapturesCanonicalizesAsBefore(t *testing.T) {
	bundle := visualBundleBase()
	raw, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "visual_evidence") {
		t.Fatalf("a bundle with no captures names the field: %s", raw)
	}
	if err := validateDraft(bundle); err != nil {
		t.Fatalf("legacy bundle refused: %v", err)
	}
}

func TestBundleCitingCapturesBindsThemToTheFrozenFrontend(t *testing.T) {
	bundle := visualBundleBase()
	bundle.VisualEvidence = visualEvidenceFor(bundle)
	if err := validateDraft(bundle); err != nil {
		t.Fatalf("bound captures refused: %v", err)
	}

	// The content digest covers the captures: adding one changes it.
	plain := visualBundleBase()
	withCaptures, _ := bundle.ComputeContentDigest()
	without, _ := plain.ComputeContentDigest()
	if withCaptures == without {
		t.Fatal("the content digest does not cover the visual evidence")
	}
	other := bundle
	other.VisualEvidence = visualEvidenceFor(bundle)
	other.VisualEvidence.Captures[0].Artifact = "artifact:sha256:" + repeatHex('d')
	otherDigest, _ := other.ComputeContentDigest()
	if otherDigest == withCaptures {
		t.Fatal("changing a capture's bytes did not change the content digest")
	}

	// V8: captures of another build than the one the bundle freezes.
	stale := bundle
	stale.VisualEvidence = visualEvidenceFor(bundle)
	stale.VisualEvidence.FrontendSHA256 = repeatHex('5')
	if err := validateDraft(stale); err == nil {
		t.Error("captures of a different frontend were accepted")
	}

	// Malformed records are refused by the bundle's own validation.
	for name, mutate := range map[string]func(*capsule.VisualEvidence){
		"no captures":          func(v *capsule.VisualEvidence) { v.Captures = nil },
		"not a content digest": func(v *capsule.VisualEvidence) { v.Captures[0].Artifact = "file:/etc/passwd" },
		"receipt of any kind":  func(v *capsule.VisualEvidence) { v.CaptureReceipt = "artifact:sha256:" + repeatHex('9') },
	} {
		bad := bundle
		bad.VisualEvidence = visualEvidenceFor(bundle)
		mutate(bad.VisualEvidence)
		if err := validateDraft(bad); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}

	// A bundle with no frontend files cannot bind captures to anything.
	bare := visualBundleBase()
	bare.RuntimeFiles = bare.RuntimeFiles[:1]
	bare.VisualEvidence = visualEvidenceFor(visualBundleBase())
	if err := validateDraft(bare); err == nil {
		t.Error("captures bound to a release with no frontend")
	}
}
