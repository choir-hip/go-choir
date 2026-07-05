package computerversion

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

// TestCrossSubstrateEquivalence proves SIAC completion gate 4: the same
// ComputerVersion is materialized through Firecracker and a non-identical
// substrate/projection path, and the equivalence checker passes for a declared
// ObservationSet.
//
// The test creates identical fixture files in two separate directories
// (representing a Firecracker persistent dir and a host-process projection),
// extracts observations through both substrates, materializes them under
// different capability manifests, and asserts equivalence.
func TestCrossSubstrateEquivalence(t *testing.T) {
	version := ComputerVersion{
		CodeRef:            CodeRef("code-abc-123"),
		ArtifactProgramRef: ArtifactProgramRef("artifact-xyz-789"),
	}

	// Create fixture data representing a ComputerVersion's durable state.
	fixtureFiles := map[string]string{
		"config.yaml":          "key: value\n",
		"data/notes.txt":       "hello world\n",
		"data/settings.json":   `{"enabled":true}`,
		"bin/program":          "\x7fELF\x02\x01\x01\x00",
	}

	// Create the Firecracker persistent dir.
	firecrackerDir := t.TempDir()
	for path, content := range fixtureFiles {
		full := filepath.Join(firecrackerDir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", path, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	// Create the host-process projection dir with identical files.
	hostDir := t.TempDir()
	for path, content := range fixtureFiles {
		full := filepath.Join(hostDir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", path, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	ctx := context.Background()

	// Extract through the Firecracker substrate.
	firecrackerObs, err := (FirecrackerStateExtractor{PersistentDir: firecrackerDir}).Extract(ctx, ExtractRequest{
		Name:    "firecracker-state",
		Version: version,
	})
	if err != nil {
		t.Fatalf("firecracker extraction: %v", err)
	}

	// Extract through the host-process projection substrate.
	hostObs, err := (HostProjectionExtractor{RootDir: hostDir}).Extract(ctx, ExtractRequest{
		Name:    "host-projection",
		Version: version,
	})
	if err != nil {
		t.Fatalf("host projection extraction: %v", err)
	}

	// Materialize both under their respective capability manifests.
	firecrackerRealization, err := (ProjectionMaterializer{
		ID:           FirecrackerStateMaterializer,
		Observations: firecrackerObs,
	}).Materialize(ctx, version, FirecrackerStateCapabilityManifest("", ""))
	if err != nil {
		t.Fatalf("firecracker materialization: %v", err)
	}

	hostRealization, err := (ProjectionMaterializer{
		ID:           HostProjectionMaterializer,
		Observations: hostObs,
	}).Materialize(ctx, version, HostProjectionCapabilityManifest("", ""))
	if err != nil {
		t.Fatalf("host projection materialization: %v", err)
	}

	// Verify non-identical substrate identities.
	if firecrackerRealization.Capabilities.Materializer == hostRealization.Capabilities.Materializer {
		t.Fatal("materializers must be non-identical for cross-substrate proof")
	}
	if firecrackerRealization.Capabilities.Substrate == hostRealization.Capabilities.Substrate {
		t.Fatal("substrates must be non-identical for cross-substrate proof")
	}

	// Run the equivalence checker.
	result := EquivalenceChecker{}.CheckRealizations(firecrackerRealization, hostRealization)

	if result.Status != EquivalenceEquivalent {
		t.Errorf("expected equivalence, got %s", result.Status)
		for _, d := range result.Differences {
			t.Errorf("  diff: kind=%s key=%s left=%s right=%s reason=%s",
				d.Kind, d.Key, d.Left, d.Right, d.Reason)
		}
		for _, u := range result.Unsupported {
			t.Errorf("  unsupported: kind=%s reason=%s", u.Kind, u.Reason)
		}
	}

	if !result.Equivalent() {
		t.Fatal("equivalence check failed: result is not equivalent")
	}

	// Build a BaseSubstrateEquivalenceContract from the proof.
	evidence := BaseSubstrateEquivalenceEvidence{
		ClaimScope:                  BaseSubstrateEquivalenceClaimScope,
		CurrentRealizationRef:       "firecracker-state-realization",
		ProjectionRealizationRef:    "host-projection-realization",
		CurrentObservationRef:       "firecracker-state-observations",
		ProjectionObservationRef:    "host-projection-observations",
		EquivalenceEvidenceRef:      "cross-substrate-equivalence-test",
		NoRuntimeMaterialization:    true,
		NoOpaqueDataImageDependency: true,
		NoMutation:                  true,
	}
	contract, err := BuildBaseSubstrateEquivalenceContract(firecrackerRealization, hostRealization, evidence)
	if err != nil {
		t.Fatalf("build substrate equivalence contract: %v", err)
	}

	if contract.EquivalenceStatus != EquivalenceEquivalent {
		t.Errorf("contract equivalence status: expected %s, got %s", EquivalenceEquivalent, contract.EquivalenceStatus)
	}
	if contract.CurrentMaterializer != FirecrackerStateMaterializer {
		t.Errorf("contract current materializer: expected %s, got %s", FirecrackerStateMaterializer, contract.CurrentMaterializer)
	}
	if contract.ProjectionMaterializer != HostProjectionMaterializer {
		t.Errorf("contract projection materializer: expected %s, got %s", HostProjectionMaterializer, contract.ProjectionMaterializer)
	}
	if contract.CurrentSubstrate != FirecrackerStateSubstrate {
		t.Errorf("contract current substrate: expected %s, got %s", FirecrackerStateSubstrate, contract.CurrentSubstrate)
	}
	if contract.ProjectionSubstrate != HostProjectionSubstrate {
		t.Errorf("contract projection substrate: expected %s, got %s", HostProjectionSubstrate, contract.ProjectionSubstrate)
	}

	t.Logf("cross-substrate equivalence proven: %s (%s) == %s (%s) for %s@%s",
		contract.CurrentMaterializer, contract.CurrentSubstrate,
		contract.ProjectionMaterializer, contract.ProjectionSubstrate,
		version.CodeRef, version.ArtifactProgramRef)
}

// TestCrossSubstrateFailureProof proves SIAC completion gate 5: a seeded
// mismatch causes the equivalence checker to fail, proving the verifier is not
// ceremonial.
func TestCrossSubstrateFailureProof(t *testing.T) {
	version := ComputerVersion{
		CodeRef:            CodeRef("code-abc-123"),
		ArtifactProgramRef: ArtifactProgramRef("artifact-xyz-789"),
	}

	fixtureFiles := map[string]string{
		"config.yaml":    "key: value\n",
		"data/notes.txt": "hello world\n",
	}

	// Create the Firecracker persistent dir with original content.
	firecrackerDir := t.TempDir()
	for path, content := range fixtureFiles {
		full := filepath.Join(firecrackerDir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", path, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	// Create the host-process projection dir with a seeded mismatch:
	// "data/notes.txt" has different content.
	hostDir := t.TempDir()
	for path, content := range fixtureFiles {
		full := filepath.Join(hostDir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", path, err)
		}
		if path == "data/notes.txt" {
			content = "hello CORRUPTED world\n"
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	ctx := context.Background()

	firecrackerObs, err := (FirecrackerStateExtractor{PersistentDir: firecrackerDir}).Extract(ctx, ExtractRequest{
		Name:    "firecracker-state",
		Version: version,
	})
	if err != nil {
		t.Fatalf("firecracker extraction: %v", err)
	}

	hostObs, err := (HostProjectionExtractor{RootDir: hostDir}).Extract(ctx, ExtractRequest{
		Name:    "host-projection",
		Version: version,
	})
	if err != nil {
		t.Fatalf("host projection extraction: %v", err)
	}

	firecrackerRealization, err := (ProjectionMaterializer{
		ID:           FirecrackerStateMaterializer,
		Observations: firecrackerObs,
	}).Materialize(ctx, version, FirecrackerStateCapabilityManifest("", ""))
	if err != nil {
		t.Fatalf("firecracker materialization: %v", err)
	}

	hostRealization, err := (ProjectionMaterializer{
		ID:           HostProjectionMaterializer,
		Observations: hostObs,
	}).Materialize(ctx, version, HostProjectionCapabilityManifest("", ""))
	if err != nil {
		t.Fatalf("host projection materialization: %v", err)
	}

	result := EquivalenceChecker{}.CheckRealizations(firecrackerRealization, hostRealization)

	if result.Status != EquivalenceNotEquivalent {
		t.Fatalf("expected not_equivalent, got %s", result.Status)
	}

	if len(result.Differences) == 0 {
		t.Fatal("expected at least one difference, got none")
	}

	// Verify the difference names the corrupted file.
	foundMismatch := false
	for _, d := range result.Differences {
		t.Logf("  diff: kind=%s key=%s reason=%s", d.Kind, d.Key, d.Reason)
		if d.Key == "data/notes.txt" || d.Key == "sha256:"+sha256Of("hello world\n") || d.Key == "sha256:"+sha256Of("hello CORRUPTED world\n") {
			foundMismatch = true
		}
	}
	if !foundMismatch {
		t.Errorf("expected a difference naming data/notes.txt or its blob ref, got: %+v", result.Differences)
	}

	// Verify the contract builder rejects the mismatched evidence.
	evidence := BaseSubstrateEquivalenceEvidence{
		ClaimScope:                  BaseSubstrateEquivalenceClaimScope,
		CurrentRealizationRef:       "firecracker-state-realization",
		ProjectionRealizationRef:    "host-projection-realization",
		CurrentObservationRef:       "firecracker-state-observations",
		ProjectionObservationRef:    "host-projection-observations",
		EquivalenceEvidenceRef:      "cross-substrate-failure-test",
		NoRuntimeMaterialization:    true,
		NoOpaqueDataImageDependency: true,
		NoMutation:                  true,
	}
	_, err = BuildBaseSubstrateEquivalenceContract(firecrackerRealization, hostRealization, evidence)
	if err == nil {
		t.Fatal("expected contract builder to reject mismatched realizations")
	}

	t.Logf("failure proof confirmed: equivalence checker detected seeded mismatch with %d differences", len(result.Differences))
}

// TestCrossSubstrateNarrowedProof verifies that a capability mismatch narrows
// the equivalence claim rather than passing or failing outright. We use the
// VMManagerScopedMaterializer (which only supports vm_state_manifest) on one
// side and the FirecrackerStateExtractor on the other. The merged required
// kinds include both vm_state_manifest and file_manifest/blob_set, but neither
// manifest supports all three, so the claim narrows.
func TestCrossSubstrateNarrowedProof(t *testing.T) {
	version := ComputerVersion{
		CodeRef:            CodeRef("code-abc-123"),
		ArtifactProgramRef: ArtifactProgramRef("artifact-xyz-789"),
	}

	fixtureFiles := map[string]string{
		"config.yaml": "key: value\n",
	}

	firecrackerDir := t.TempDir()
	for path, content := range fixtureFiles {
		full := filepath.Join(firecrackerDir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", path, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	ctx := context.Background()

	firecrackerObs, err := (FirecrackerStateExtractor{PersistentDir: firecrackerDir}).Extract(ctx, ExtractRequest{
		Name:    "firecracker-state",
		Version: version,
	})
	if err != nil {
		t.Fatalf("firecracker extraction: %v", err)
	}

	// Materialize the Firecracker side with full file_manifest + blob_set support.
	firecrackerRealization, err := (ProjectionMaterializer{
		ID:           FirecrackerStateMaterializer,
		Observations: firecrackerObs,
	}).Materialize(ctx, version, FirecrackerStateCapabilityManifest("", ""))
	if err != nil {
		t.Fatalf("firecracker materialization: %v", err)
	}

	// Materialize the VMManager side with only vm_state_manifest support.
	// Its observations only require vm_state_manifest, but when merged with
	// the Firecracker side's file_manifest/blob_set, the VMManager manifest
	// cannot support those kinds, so the claim narrows.
	vmPath := VMManagerScopedPath{
		VMID:          "vm-test-001",
		PersistentDir: firecrackerDir,
		DataImageClass: StateClassDurableLegacyOpaque,
	}
	vmObs, err := vmPath.ObservationSet("vmmanager-scoped", version)
	if err != nil {
		t.Fatalf("vmmanager observation: %v", err)
	}

	vmRealization, err := (VMManagerScopedMaterializer{
		ID:    "vmmanager-scoped",
		State: vmPath,
	}).Materialize(ctx, version, VMManagerCapabilityManifest(""))
	if err != nil {
		t.Fatalf("vmmanager materialization: %v", err)
	}

	// Use the VMManager observation set directly — it should have vm_state_manifest
	_ = vmObs

	result := EquivalenceChecker{}.CheckRealizations(firecrackerRealization, vmRealization)

	if result.Status != EquivalenceNarrowed {
		t.Fatalf("expected narrowed, got %s (diffs: %v, unsupported: %v)",
			result.Status, result.Differences, result.Unsupported)
	}

	if len(result.Unsupported) == 0 {
		t.Fatal("expected unsupported capabilities, got none")
	}

	t.Logf("narrowed proof confirmed: %d unsupported capabilities", len(result.Unsupported))
	for _, u := range result.Unsupported {
		t.Logf("  unsupported: kind=%s reason=%s", u.Kind, u.Reason)
	}
}

func sha256Of(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
