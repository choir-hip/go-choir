package yaegikernel

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// writeMountedBundleFixture installs a binding.json + bundle.draft.json +
// runtime file tree under root with a content digest computed exactly as
// CapsuleEffectBundle.ComputeContentDigest does (canonical JSON with
// content_digest and verifier_receipts cleared).
func writeMountedBundleFixture(t *testing.T, root string) (bundleDraftMirror, string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	payload := []byte("#!/bin/sh\necho ok\n")
	if err := os.WriteFile(filepath.Join(root, "bin", "run.sh"), payload, 0o755); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(payload)
	record := bundleDraftMirror{
		BundleVersion:           1,
		ComputerID:              "computer-test",
		BaseEventHead:           hex.EncodeToString(make([]byte, 32)),
		TrajectoryRef:           "traj-test",
		CapsuleIdentity:         "capsule-test",
		CapabilityPolicyDigest:  hex.EncodeToString(make([]byte, 32)),
		SourceTreeRef:           "source-tree:sha256:" + hex.EncodeToString(make([]byte, 32)),
		OrderedFileEffects:      []json.RawMessage{json.RawMessage(`{"path":"bin/run.sh","kind":"added","mode":493}`)},
		GeneratedArtifactRefs:   []string{"artifact:sha256:" + hex.EncodeToString(sum[:])},
		BuildRecipeRef:          "capsule-exec:sha256:" + hex.EncodeToString(make([]byte, 32)),
		RuntimeArtifactRef:      "runtime-artifact:sha256:" + hex.EncodeToString(make([]byte, 32)),
		TestReceipts:            []string{"capsule-exec:sha256:" + hex.EncodeToString(make([]byte, 32))},
		VerifierReceipts:        []string{},
		DependencyToolchainRefs: []string{"capsule-exec:sha256:" + hex.EncodeToString(make([]byte, 32))},
		ResourceReceipts:        []string{"artifact:sha256:" + hex.EncodeToString(make([]byte, 32))},
		ClassifierV:             "v1",
		ClassifierDigest:        hex.EncodeToString(make([]byte, 32)),
		Groups:                  map[string]json.RawMessage{"release": json.RawMessage(`[{"path":"bin/run.sh"}]`)},
		Ignored:                 []json.RawMessage{},
		RuntimeFiles:            []mountedRuntimeFile{{Path: "bin/run.sh", SHA256: hex.EncodeToString(sum[:]), Mode: 0o755}},
	}
	recompute := record
	recompute.ContentDigest = ""
	recompute.VerifierReceipts = []string{}
	canonical, err := computerevent.CanonicalJSON(recompute)
	if err != nil {
		t.Fatal(err)
	}
	record.ContentDigest = computerevent.DigestBytes(canonical)
	draft, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "bundle.draft.json"), draft, 0o400); err != nil {
		t.Fatal(err)
	}
	binding, err := json.Marshal(mountedBundleBinding{OperationID: "op-test", BundleDigest: record.ContentDigest})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "binding.json"), binding, 0o644); err != nil {
		t.Fatal(err)
	}
	return record, record.ContentDigest
}

func TestInspectMountedBundleReturnsCanonicalReceipt(t *testing.T) {
	root := t.TempDir()
	record, digest := writeMountedBundleFixture(t, root)
	out, err := inspectMountedBundleAt(root)
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if out["operation_id"] != "op-test" || out["content_digest"] != digest {
		t.Fatalf("inspect identity = %v / %v, want op-test / %s", out["operation_id"], out["content_digest"], digest)
	}
	files, ok := out["runtime_files"].([]map[string]any)
	if !ok || len(files) != 1 || files[0]["path"] != "bin/run.sh" {
		t.Fatalf("runtime_files = %+v", out["runtime_files"])
	}
	execRefs, ok := out["execution_receipts"].([]string)
	if !ok || len(execRefs) != 1+len(record.TestReceipts)+len(record.DependencyToolchainRefs) {
		t.Fatalf("execution_receipts = %+v", out["execution_receipts"])
	}
}

func TestInspectMountedBundleRejectsTamperedFile(t *testing.T) {
	root := t.TempDir()
	writeMountedBundleFixture(t, root)
	if err := os.WriteFile(filepath.Join(root, "bin", "run.sh"), []byte("tampered"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := inspectMountedBundleAt(root); err == nil {
		t.Fatal("tampered runtime file must fail inspection")
	}
}

func TestInspectMountedBundleRejectsDigestMismatch(t *testing.T) {
	root := t.TempDir()
	writeMountedBundleFixture(t, root)
	binding, _ := json.Marshal(mountedBundleBinding{OperationID: "op-test", BundleDigest: hex.EncodeToString(make([]byte, 32))})
	if err := os.WriteFile(filepath.Join(root, "binding.json"), binding, 0o400); err != nil {
		t.Fatal(err)
	}
	if _, err := inspectMountedBundleAt(root); err == nil {
		t.Fatal("binding/draft digest mismatch must fail inspection")
	}
}

func TestInspectMountedBundleRequiresBinding(t *testing.T) {
	root := t.TempDir()
	if _, err := inspectMountedBundleAt(root); err == nil {
		t.Fatal("missing binding must fail inspection")
	}
}
