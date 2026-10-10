package transaction

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// The verifier's in-cell choir.InspectBundle decodes the draft into its own
// mirror (it cannot import this package). A draft built from the real type,
// with every field set, must inspect cleanly: a field the mirror lacks fails
// the strict decode and every verification
// (problems/selfdev-verifier-rejects-frozen-bundle-2026-10-10.md).
func TestInBundleInspectionReadsEveryCapsuleEffectBundleField(t *testing.T) {
	root := t.TempDir()
	payload := []byte("#!/bin/sh\necho ok\n")
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "bin", "autoputer"), payload, 0o755); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(payload)
	fileSHA := hex.EncodeToString(sum[:])
	bundle := CapsuleEffectBundle{
		BundleVersion: 1, ComputerID: "computer-1", BaseEventHead: repeatHex('a'),
		TrajectoryRef: "trajectory-1", CapsuleIdentity: "capsule-1", CapabilityPolicyDigest: repeatHex('b'),
		SourceTreeRef:           "source-tree:sha256:" + repeatHex('c'),
		OrderedFileEffects:      []ChangeRecord{{Path: "workspace/platform/docs/evidence/m11.md", Kind: "added", Mode: 0o644}},
		GeneratedArtifactRefs:   []string{"artifact:sha256:" + fileSHA},
		BuildRecipeRef:          "capsule-exec:sha256:" + repeatHex('e'),
		RuntimeArtifactRef:      "runtime-artifact:sha256:" + repeatHex('f'),
		TestReceipts:            []string{"capsule-exec:sha256:" + repeatHex('1')},
		VerifierReceipts:        []string{},
		DependencyToolchainRefs: []string{"capsule-exec:sha256:" + repeatHex('2')},
		ResourceReceipts:        []string{"resource:sha256:" + repeatHex('3')},
		ClassifierV:             "v1", ClassifierDigest: repeatHex('6'),
		Groups:                map[string][]ChangeRecord{LedgerSource.String(): {{Path: "workspace/platform/docs/evidence/m11.md", Kind: "added", Mode: 0o644}}},
		Ignored:               []ChangeRecord{},
		Unknown:               []ChangeRecord{{Path: "unknown/x", Kind: "added", Mode: 0o644}},
		RejectReason:          "",
		RuntimeFiles:          []capsule.FrozenReleaseFile{{Path: "bin/autoputer", SHA256: fileSHA, Mode: 0o755}},
		SourcePatchSHA256:     repeatHex('7'),
		SourcePatchBaseCommit: "0123456789abcdef0123456789abcdef01234567",
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
	if err := os.WriteFile(filepath.Join(root, "bundle.draft.json"), draft, 0o400); err != nil {
		t.Fatal(err)
	}
	binding, err := json.Marshal(map[string]string{"operation_id": "op-1", "bundle_digest": bundle.ContentDigest})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "binding.json"), binding, 0o400); err != nil {
		t.Fatal(err)
	}
	out, err := yaegikernel.InspectMountedBundleAt(root)
	if err != nil {
		t.Fatalf("in-cell inspection refused a real draft: %v", err)
	}
	if out["content_digest"] != bundle.ContentDigest {
		t.Fatalf("content_digest = %v, want %s", out["content_digest"], bundle.ContentDigest)
	}
}
