package transaction

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Screens in the bundle (docs/problems/selfdev-screens-are-not-files-2026-11-03.md):
// the engineer's screenshots are bound to the frozen candidate by sha256 in
// the bundle record, so the content digest covers them; the judge's in-cell
// inspection re-hashes the staged bytes. What can go wrong: a record that
// is malformed (refused by Validate), bytes swapped or missing on disk
// (refused by the inspection), a bundle with none (no field at all, so an
// older bundle's digest is unchanged), and a screens path the classifier
// does not know (Freeze would reject the candidate).

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func screensBundleBase() CapsuleEffectBundle {
	digest := repeatHex
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
		RuntimeFiles: []capsule.FrozenReleaseFile{
			{Path: "bin/autoputer", SHA256: digest('d'), Mode: 0o555},
			{Path: "frontend/index.html", SHA256: digest('7'), Mode: 0o444},
		},
		Groups: map[string][]ChangeRecord{}, Ignored: []ChangeRecord{},
	}
}

func validateDraft(b CapsuleEffectBundle) error {
	b.ContentDigest, _ = b.ComputeContentDigest()
	return b.Validate(false)
}

func TestBundleWithoutScreensCanonicalizesAsBefore(t *testing.T) {
	bundle := screensBundleBase()
	raw, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "screens") || strings.Contains(string(raw), "visual_evidence") {
		t.Fatalf("a bundle with no screens names the field: %s", raw)
	}
	if err := validateDraft(bundle); err != nil {
		t.Fatalf("bundle refused: %v", err)
	}
}

func TestBundleScreensAreCoveredByTheContentDigest(t *testing.T) {
	bundle := screensBundleBase()
	bundle.Screens = []capsule.ScreenFile{{Name: "home.png", SHA256: repeatHex('c'), Bytes: 1234, Width: 1280, Height: 800}}
	if err := validateDraft(bundle); err != nil {
		t.Fatalf("screens refused: %v", err)
	}
	with, _ := bundle.ComputeContentDigest()
	without, _ := screensBundleBase().ComputeContentDigest()
	if with == without {
		t.Fatal("the content digest does not cover the screens")
	}
	other := bundle
	other.Screens = []capsule.ScreenFile{{Name: "home.png", SHA256: repeatHex('d'), Bytes: 1234, Width: 1280, Height: 800}}
	if d, _ := other.ComputeContentDigest(); d == with {
		t.Fatal("changing a screen's bytes did not change the content digest")
	}
	bad := bundle
	bad.Screens = []capsule.ScreenFile{{Name: "../etc/passwd", SHA256: repeatHex('c'), Bytes: 1, Width: 1, Height: 1}}
	if err := validateDraft(bad); err == nil {
		t.Error("a screen with a path name was accepted")
	}
}

func TestScreensPathsClassifyAsArtifactNotUnknown(t *testing.T) {
	classifier := NewClassifier()
	for _, prefix := range []string{"", "/"} {
		result := classifier.Classify([]capsule.FileChange{{Path: prefix + capsule.ScreensPrefix + "home.png", Kind: capsule.ChangeAdded, Mode: 0o644}})
		if result.HasUnknown() || len(result.Groups[LedgerArtifact]) != 1 {
			t.Fatalf("screens under %q: unknown %+v groups %+v", prefix, result.Unknown, result.Groups)
		}
	}
}

// mountedScreensBundle lays out a frozen bundle with one screen on disk.
func mountedScreensBundle(t *testing.T, mutate func(*CapsuleEffectBundle)) (root string, bundle CapsuleEffectBundle) {
	t.Helper()
	root = t.TempDir()
	write := func(rel string, data []byte, mode os.FileMode) string {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		_ = os.Chmod(path, 0o600)
		if err := os.WriteFile(path, data, mode); err != nil {
			t.Fatal(err)
		}
		return sha256Hex(data)
	}
	binSHA := write("bin/autoputer", []byte("#!/bin/sh\necho ok\n"), 0o755)
	indexSHA := write("frontend/index.html", []byte("<html>hi</html>"), 0o444)
	shot := []byte("\x89PNG\r\n\x1a\nnot a real image, the inspection only hashes it")
	shotSHA := write(capsule.BundleScreensDir+"/home.png", shot, 0o444)
	bundle = screensBundleBase()
	bundle.RuntimeFiles = []capsule.FrozenReleaseFile{{Path: "bin/autoputer", SHA256: binSHA, Mode: 0o755}, {Path: "frontend/index.html", SHA256: indexSHA, Mode: 0o444}}
	bundle.GeneratedArtifactRefs = []string{"artifact:sha256:" + binSHA}
	bundle.Screens = []capsule.ScreenFile{{Name: "home.png", SHA256: shotSHA, Bytes: int64(len(shot)), Width: 1280, Height: 800}}
	if mutate != nil {
		mutate(&bundle)
	}
	var err error
	if bundle.ContentDigest, err = bundle.ComputeContentDigest(); err != nil {
		t.Fatal(err)
	}
	draft, _ := json.Marshal(bundle)
	write("bundle.draft.json", draft, 0o400)
	binding, _ := json.Marshal(map[string]string{"operation_id": "op-1", "bundle_digest": bundle.ContentDigest})
	write("binding.json", binding, 0o400)
	return root, bundle
}

func TestInBundleInspectionReportsAndRehashesTheScreens(t *testing.T) {
	root, bundle := mountedScreensBundle(t, nil)
	out, err := yaegikernel.InspectMountedBundleAt(root)
	if err != nil {
		t.Fatalf("inspection refused a bundle with screens: %v", err)
	}
	screens, _ := out["screens"].([]map[string]any)
	if len(screens) != 1 || screens[0]["name"] != "home.png" || screens[0]["sha256"] != bundle.Screens[0].SHA256 ||
		screens[0]["file"] != "Self-development/op-1/screens/home.png" || screens[0]["image_ref"] != "artifact:sha256:"+bundle.Screens[0].SHA256 {
		t.Fatalf("screens: %v", out["screens"])
	}
	if _, present := out["visual_evidence"]; present {
		t.Fatal("inspection still reports visual_evidence")
	}
}

func TestInBundleInspectionRefusesScreensThatDoNotMatchTheirBytes(t *testing.T) {
	root, _ := mountedScreensBundle(t, nil)
	path := filepath.Join(root, filepath.FromSlash(capsule.BundleScreensDir), "home.png")
	_ = os.Chmod(path, 0o644)
	if err := os.WriteFile(path, []byte("swapped"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := yaegikernel.InspectMountedBundleAt(root); err == nil || !strings.Contains(err.Error(), "screen") {
		t.Fatalf("a swapped screen was inspected cleanly: %v", err)
	}
	root, _ = mountedScreensBundle(t, nil)
	if err := os.Remove(filepath.Join(root, filepath.FromSlash(capsule.BundleScreensDir), "home.png")); err != nil {
		t.Fatal(err)
	}
	if _, err := yaegikernel.InspectMountedBundleAt(root); err == nil {
		t.Fatal("a missing screen was inspected cleanly")
	}
}

func TestInBundleInspectionOfABundleWithoutScreensReportsNone(t *testing.T) {
	root, _ := mountedScreensBundle(t, func(b *CapsuleEffectBundle) { b.Screens = nil })
	out, err := yaegikernel.InspectMountedBundleAt(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, present := out["screens"]; present {
		t.Fatal("a bundle without screens reports screens")
	}
}

// A bundle frozen before the platform capture was removed carries
// visual_evidence. It must still decode strictly, recompute the digest it was
// frozen under, and validate, or its operation would be stuck.
func TestABundleWithLegacyVisualEvidenceStillDecodesAndValidates(t *testing.T) {
	base := screensBundleBase()
	raw, _ := json.Marshal(base)
	var asMap map[string]any
	if err := json.Unmarshal(raw, &asMap); err != nil {
		t.Fatal(err)
	}
	asMap["visual_evidence"] = map[string]any{"manifest_sha256": repeatHex('8'), "frontend_sha256": repeatHex('9'),
		"captures": []any{map[string]any{"artifact": "artifact:sha256:" + repeatHex('c'), "route": "/", "media_type": "image/png", "bytes": 1234, "width": 1280, "height": 800}}}
	asMap["content_digest"] = ""
	asMap["verifier_receipts"] = []any{}
	canon, err := computerevent.CanonicalJSON(asMap)
	if err != nil {
		t.Fatal(err)
	}
	frozenUnder := computerevent.DigestBytes(canon)
	asMap["content_digest"] = frozenUnder
	legacy, _ := json.Marshal(asMap)
	var decoded CapsuleEffectBundle
	dec := json.NewDecoder(strings.NewReader(string(legacy)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&decoded); err != nil {
		t.Fatalf("a legacy bundle no longer decodes: %v", err)
	}
	if got, _ := decoded.ComputeContentDigest(); got != frozenUnder {
		t.Fatalf("a legacy bundle recomputes %s, frozen under %s", got, frozenUnder)
	}
	if err := decoded.Validate(false); err != nil {
		t.Fatalf("a legacy bundle no longer validates: %v", err)
	}
}
