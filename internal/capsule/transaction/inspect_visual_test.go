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
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// The verifier's in-cell inspection reads the visual evidence field (the
// drift test's rule) and re-hashes the staged captures, so management can
// cite what it inspected (docs/design/visual-evidence-slice-2026-10-15.md).

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// mountedVisualBundle lays out a frozen bundle with one capture on disk.
func mountedVisualBundle(t *testing.T) (root string, bundle CapsuleEffectBundle, shotName string) {
	t.Helper()
	root = t.TempDir()
	write := func(rel string, data []byte, mode os.FileMode) string {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, mode); err != nil {
			t.Fatal(err)
		}
		return sha256Hex(data)
	}
	binSHA := write("bin/autoputer", []byte("#!/bin/sh\necho ok\n"), 0o755)
	indexSHA := write("frontend/index.html", []byte("<html>hi</html>"), 0o444)
	shot := []byte("\x89PNG\r\n\x1a\nnot a real image, the inspection only hashes it")
	shotSHA := write("evidence/visual/"+sha256Hex(shot)+".png", shot, 0o444)
	shotName = shotSHA + ".png"

	bundle = visualBundleBase()
	bundle.RuntimeFiles = []capsule.FrozenReleaseFile{
		{Path: "bin/autoputer", SHA256: binSHA, Mode: 0o755},
		{Path: "frontend/index.html", SHA256: indexSHA, Mode: 0o444},
	}
	bundle.GeneratedArtifactRefs = []string{"artifact:sha256:" + binSHA}
	bundle.VisualEvidence = &capsule.VisualEvidence{
		ManifestSHA256: repeatHex('8'), FrontendSHA256: capsule.ReleaseFrontendDigest(bundle.RuntimeFiles),
		CaptureReceipt: "capsule-exec:sha256:" + repeatHex('9'),
		Captures: []capsule.VisualEvidenceCapture{{
			Artifact: "artifact:sha256:" + shotSHA, Route: "/choir-drill/x.html", MediaType: "image/png",
			Bytes: int64(len(shot)), Width: 1280, Height: 800, Blank: true,
		}},
	}
	var err error
	bundle.ContentDigest, err = bundle.ComputeContentDigest()
	if err != nil {
		t.Fatal(err)
	}
	draft, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	write("bundle.draft.json", draft, 0o400)
	binding, _ := json.Marshal(map[string]string{"operation_id": "op-1", "bundle_digest": bundle.ContentDigest})
	write("binding.json", binding, 0o400)
	return root, bundle, shotName
}

func TestInBundleInspectionReportsAndRehashesTheCaptures(t *testing.T) {
	root, bundle, _ := mountedVisualBundle(t)
	out, err := yaegikernel.InspectMountedBundleAt(root)
	if err != nil {
		t.Fatalf("in-cell inspection refused a bundle with captures: %v", err)
	}
	visual, _ := out["visual_evidence"].(map[string]any)
	if visual == nil {
		t.Fatalf("inspection does not report the captures: %v", out)
	}
	if visual["capture_receipt"] != bundle.VisualEvidence.CaptureReceipt || visual["manifest_sha256"] != bundle.VisualEvidence.ManifestSHA256 {
		t.Fatalf("visual_evidence header: %v", visual)
	}
	captures, _ := visual["captures"].([]map[string]any)
	if len(captures) != 1 || captures[0]["artifact"] != bundle.VisualEvidence.Captures[0].Artifact ||
		captures[0]["route"] != "/choir-drill/x.html" || captures[0]["blank"] != true {
		t.Fatalf("captures: %v", visual["captures"])
	}
}

// Problem 7: the verifier's inspection names routes that share one
// screenshot, since the capture server answers an extensionless route with
// index.html.
func TestInBundleInspectionWarnsWhenRoutesShareOneScreenshot(t *testing.T) {
	root, bundle, _ := mountedVisualBundle(t)
	if out, err := yaegikernel.InspectMountedBundleAt(root); err != nil {
		t.Fatal(err)
	} else if visual, _ := out["visual_evidence"].(map[string]any); visual["warnings"] != nil {
		t.Fatalf("one route warns: %v", visual["warnings"])
	}
	dup := bundle.VisualEvidence.Captures[0]
	dup.Route = "/read"
	bundle.VisualEvidence.Captures = append(bundle.VisualEvidence.Captures, dup)
	bundle.ContentDigest, _ = bundle.ComputeContentDigest()
	draft, _ := json.Marshal(bundle)
	binding, _ := json.Marshal(map[string]string{"operation_id": "op-1", "bundle_digest": bundle.ContentDigest})
	for name, data := range map[string][]byte{"bundle.draft.json": draft, "binding.json": binding} {
		path := filepath.Join(root, name)
		_ = os.Chmod(path, 0o600)
		if err := os.WriteFile(path, data, 0o400); err != nil {
			t.Fatal(err)
		}
	}
	out, err := yaegikernel.InspectMountedBundleAt(root)
	if err != nil {
		t.Fatal(err)
	}
	visual, _ := out["visual_evidence"].(map[string]any)
	warnings, _ := visual["warnings"].([]string)
	if len(warnings) != 1 || !strings.Contains(warnings[0], "/choir-drill/x.html and /read") {
		t.Fatalf("warnings = %v", visual["warnings"])
	}
}

func TestInBundleInspectionRefusesCapturesThatDoNotMatchTheirBytes(t *testing.T) {
	t.Run("a capture changed on disk", func(t *testing.T) {
		root, _, name := mountedVisualBundle(t)
		path := filepath.Join(root, "evidence", "visual", name)
		_ = os.Chmod(path, 0o644)
		if err := os.WriteFile(path, []byte("swapped"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := yaegikernel.InspectMountedBundleAt(root); err == nil || !strings.Contains(err.Error(), "capture") {
			t.Fatalf("a swapped capture was inspected cleanly: %v", err)
		}
	})
	t.Run("a capture is missing", func(t *testing.T) {
		root, _, name := mountedVisualBundle(t)
		if err := os.Remove(filepath.Join(root, "evidence", "visual", name)); err != nil {
			t.Fatal(err)
		}
		if _, err := yaegikernel.InspectMountedBundleAt(root); err == nil {
			t.Fatal("a missing capture was inspected cleanly")
		}
	})
	t.Run("captures of another frontend than the runtime files", func(t *testing.T) {
		root, bundle, _ := mountedVisualBundle(t)
		bundle.VisualEvidence.FrontendSHA256 = repeatHex('5')
		bundle.ContentDigest, _ = bundle.ComputeContentDigest()
		draft, _ := json.Marshal(bundle)
		binding, _ := json.Marshal(map[string]string{"operation_id": "op-1", "bundle_digest": bundle.ContentDigest})
		for name, data := range map[string][]byte{"bundle.draft.json": draft, "binding.json": binding} {
			path := filepath.Join(root, name)
			_ = os.Chmod(path, 0o600)
			if err := os.WriteFile(path, data, 0o400); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := yaegikernel.InspectMountedBundleAt(root); err == nil || !strings.Contains(err.Error(), "frontend") {
			t.Fatalf("captures of a different frontend were inspected cleanly: %v", err)
		}
	})
}

func TestInBundleInspectionOfALegacyBundleReportsNoVisualEvidence(t *testing.T) {
	root, bundle, _ := mountedVisualBundle(t)
	bundle.VisualEvidence = nil
	bundle.ContentDigest, _ = bundle.ComputeContentDigest()
	draft, _ := json.Marshal(bundle)
	binding, _ := json.Marshal(map[string]string{"operation_id": "op-1", "bundle_digest": bundle.ContentDigest})
	for name, data := range map[string][]byte{"bundle.draft.json": draft, "binding.json": binding} {
		path := filepath.Join(root, name)
		_ = os.Chmod(path, 0o600)
		if err := os.WriteFile(path, data, 0o400); err != nil {
			t.Fatal(err)
		}
	}
	out, err := yaegikernel.InspectMountedBundleAt(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, present := out["visual_evidence"]; present {
		t.Fatal("a bundle without captures reports visual_evidence")
	}
}
