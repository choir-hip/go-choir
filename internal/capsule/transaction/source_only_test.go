package transaction

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Source-only freeze (docs/design/source-only-freeze-and-candidate-shots-2026-11-06.md
// 3.1). Failure modes, decided before the code:
//
//   - FM1 a source-only bundle (no RuntimeFiles) is refused by Validate, as
//     before the change, so no platform candidate can freeze;
//   - the opposite: a bundle with no files and no patch is admitted;
//   - the runtime_artifact_ref loses its "runtime-artifact:" prefix, which
//     every reader of a bundle checks;
//   - the strict in-cell mirror lacks shot_plan, so every verification of a
//     plan bundle fails the strict decode;
//   - the host_build provenance is admitted with files or without a patch;
//   - a bad shot plan rides into a bundle.

func sourceOnlyBundle(t *testing.T) CapsuleEffectBundle {
	t.Helper()
	patchSHA := repeatHex('7')
	ref := "runtime-artifact:sha256:" + computerevent.DigestBytes([]byte(`{"source_patch_base_commit":"0123456789abcdef0123456789abcdef01234567","source_patch_sha256":"`+patchSHA+`"}`))
	bundle := CapsuleEffectBundle{
		BundleVersion: 1, ComputerID: "computer-1", BaseEventHead: repeatHex('a'),
		TrajectoryRef: "trajectory-1", CapsuleIdentity: "capsule-1", CapabilityPolicyDigest: repeatHex('b'),
		SourceTreeRef:           "source-tree:sha256:" + repeatHex('c'),
		OrderedFileEffects:      []ChangeRecord{{Path: "workspace/platform/internal/x/x.go", Kind: "modified", Mode: 0o644}},
		GeneratedArtifactRefs:   []string{ref},
		BuildRecipeRef:          "capsule-exec:sha256:" + repeatHex('e'),
		RuntimeArtifactRef:      ref,
		TestReceipts:            []string{"capsule-exec:sha256:" + repeatHex('1')},
		VerifierReceipts:        []string{},
		DependencyToolchainRefs: []string{"capsule-exec:sha256:" + repeatHex('2')},
		ResourceReceipts:        []string{"resource:sha256:" + repeatHex('3')},
		ClassifierV:             "v1", ClassifierDigest: repeatHex('6'),
		Groups:                map[string][]ChangeRecord{LedgerSource.String(): {{Path: "workspace/platform/internal/x/x.go", Kind: "modified", Mode: 0o644}}},
		Ignored:               []ChangeRecord{},
		RuntimeFiles:          []capsule.FrozenReleaseFile{},
		SourcePatchSHA256:     patchSHA,
		SourcePatchBaseCommit: "0123456789abcdef0123456789abcdef01234567",
		ReleaseBinary:         &capsule.ReleaseBinaryProvenance{Origin: capsule.ReleaseBinaryHostBuild},
	}
	var err error
	if bundle.ContentDigest, err = bundle.ComputeContentDigest(); err != nil {
		t.Fatal(err)
	}
	return bundle
}

func redigest(t *testing.T, b CapsuleEffectBundle) CapsuleEffectBundle {
	t.Helper()
	var err error
	if b.ContentDigest, err = b.ComputeContentDigest(); err != nil {
		t.Fatal(err)
	}
	return b
}

func TestSourceOnlyBundleValidatesAndKeepsTheRuntimeArtifactPrefix(t *testing.T) {
	bundle := sourceOnlyBundle(t)
	if err := bundle.Validate(false); err != nil {
		t.Fatalf("a source-only bundle must validate: %v", err)
	}
	if !bundle.SourceOnly() {
		t.Fatal("SourceOnly() = false for a patch with no release files")
	}
	if !strings.HasPrefix(bundle.RuntimeArtifactRef, "runtime-artifact:sha256:") {
		t.Fatalf("runtime_artifact_ref lost its prefix: %s", bundle.RuntimeArtifactRef)
	}
	// The prefix is enforced, not just produced.
	bad := bundle
	bad.RuntimeArtifactRef = "artifact:sha256:" + repeatHex('9')
	if err := redigest(t, bad).Validate(false); err == nil {
		t.Fatal("a runtime_artifact_ref without its prefix validated")
	}
}

func TestSourceOnlyBundleNeedsAPatchAndHostBuildProvenance(t *testing.T) {
	// No files and no patch: still refused (the old refusal stands).
	none := sourceOnlyBundle(t)
	none.SourcePatchSHA256, none.SourcePatchBaseCommit, none.ReleaseBinary = "", "", nil
	if err := redigest(t, none).Validate(false); err == nil {
		t.Fatal("a bundle with neither release files nor a source patch validated")
	}
	// Source-only without host_build provenance.
	missing := sourceOnlyBundle(t)
	missing.ReleaseBinary = nil
	if err := redigest(t, missing).Validate(false); err == nil {
		t.Fatal("a source-only bundle without host_build provenance validated")
	}
	// host_build provenance with a sha, or with files, is not admitted.
	withSHA := sourceOnlyBundle(t)
	withSHA.ReleaseBinary = &capsule.ReleaseBinaryProvenance{Origin: capsule.ReleaseBinaryHostBuild, SHA256: repeatHex('4')}
	if err := redigest(t, withSHA).Validate(false); err == nil {
		t.Fatal("host_build provenance carrying a sha validated")
	}
	withFiles := sourceOnlyBundle(t)
	withFiles.RuntimeFiles = []capsule.FrozenReleaseFile{{Path: "bin/autoputer", SHA256: repeatHex('5'), Mode: 0o755}}
	if err := redigest(t, withFiles).Validate(false); err == nil {
		t.Fatal("host_build provenance beside release files validated")
	}
	if err := capsule.ValidateReleaseBinary(nil, &capsule.ReleaseBinaryProvenance{Origin: capsule.ReleaseBinaryHostBuild}); err == nil {
		t.Fatal("host_build admitted with no source patch")
	}
}

func samplePlan() *capsule.ShotPlan {
	return &capsule.ShotPlan{
		Views: []capsule.ShotView{{Name: "home", Path: "/", Viewport: "desktop", Steps: []capsule.ShotStep{{Op: "wait_for", Selector: "main"}}}},
		Video: &capsule.ShotVideo{View: "home", Seconds: 5},
	}
}

func TestShotPlanBundleValidatesIsBoundByTheDigestAndRejectsABadPlan(t *testing.T) {
	bundle := sourceOnlyBundle(t)
	bundle.ShotPlan = samplePlan()
	bundle = redigest(t, bundle)
	if err := bundle.Validate(false); err != nil {
		t.Fatalf("a plan bundle must validate: %v", err)
	}
	plain := redigest(t, sourceOnlyBundle(t))
	if plain.ContentDigest == bundle.ContentDigest {
		t.Fatal("the shot plan is not bound by the content digest")
	}
	// A plan changed after the digest is refused.
	tampered := bundle
	tampered.ShotPlan = &capsule.ShotPlan{Views: []capsule.ShotView{{Name: "other", Path: "/", Viewport: "phone"}}}
	if err := tampered.Validate(false); err == nil {
		t.Fatal("a plan changed after the digest validated")
	}
	bad := sourceOnlyBundle(t)
	bad.ShotPlan = &capsule.ShotPlan{Views: []capsule.ShotView{{Name: "home", Path: "/", Viewport: "tablet"}}}
	if err := redigest(t, bad).Validate(false); err == nil {
		t.Fatal("a plan with an unknown viewport validated")
	}
}

func TestInBundleInspectionReadsASourceOnlyPlanBundle(t *testing.T) {
	root := t.TempDir()
	bundle := sourceOnlyBundle(t)
	bundle.ShotPlan = samplePlan()
	bundle = redigest(t, bundle)
	draft, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	binding, _ := json.Marshal(map[string]string{"operation_id": "op-1", "bundle_digest": bundle.ContentDigest})
	if err := os.WriteFile(filepath.Join(root, "bundle.draft.json"), draft, 0o400); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "binding.json"), binding, 0o400); err != nil {
		t.Fatal(err)
	}
	out, err := yaegikernel.InspectMountedBundleAt(root)
	if err != nil {
		t.Fatalf("the in-cell strict mirror refused a real plan bundle: %v", err)
	}
	if out["shot_plan"] == nil {
		t.Fatal("the inspection does not surface the shot plan")
	}
	if files, _ := out["runtime_files"].([]map[string]any); len(files) != 0 {
		t.Fatalf("a source-only bundle inspects with runtime files: %v", files)
	}
	// A field the plan type has and the mirror lacks must fail the strict
	// decode, not be dropped: simulate drift with an unknown plan field.
	var raw map[string]any
	_ = json.Unmarshal(draft, &raw)
	raw["shot_plan"].(map[string]any)["surprise"] = true
	drifted, _ := json.Marshal(raw)
	_ = os.Remove(filepath.Join(root, "bundle.draft.json"))
	if err := os.WriteFile(filepath.Join(root, "bundle.draft.json"), drifted, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := yaegikernel.InspectMountedBundleAt(root); err == nil {
		t.Fatal("an unknown field inside shot_plan was accepted by the strict mirror")
	}
}
