package computerversion

import (
	"context"
	"encoding/json"
	"testing"
)

func TestVMManagerScopedMaterializerEmitsOnlyVMStateManifest(t *testing.T) {
	version := vmManagerBoundaryComputerVersion()
	manifest := VMManagerCapabilityManifest("vmmanager-fixture")

	realization, err := (VMManagerScopedMaterializer{
		ID:    "vmmanager-fixture",
		State: vmManagerBoundaryPath(),
	}).Materialize(context.Background(), version, manifest)
	if err != nil {
		t.Fatalf("materialize vmmanager scoped path: %v", err)
	}

	if realization.ID != "vmmanager-fixture" {
		t.Fatalf("expected materializer id to be preserved, got %q", realization.ID)
	}
	if realization.Version != version {
		t.Fatalf("expected realization version %v, got %v", version, realization.Version)
	}
	if realization.Capabilities.Supports(ObservationFileManifest) || realization.Capabilities.Supports(ObservationBlobSet) {
		t.Fatalf("vmmanager capability must not support durable file/blob claims: %#v", realization.Capabilities)
	}
	if !realization.Capabilities.Supports(ObservationVMStateManifest) {
		t.Fatalf("vmmanager capability must support vm state manifests: %#v", realization.Capabilities)
	}

	observations := realization.Observations
	if observations.Version != version {
		t.Fatalf("expected observation version %v, got %v", version, observations.Version)
	}
	if len(observations.Required) != 1 || observations.Required[0] != ObservationVMStateManifest {
		t.Fatalf("expected exactly vm_state_manifest requirement, got %#v", observations.Required)
	}
	if containsObservationKind(observations.RequiredKinds(), ObservationFileManifest) || containsObservationKind(observations.RequiredKinds(), ObservationBlobSet) {
		t.Fatalf("vmmanager observation set must not require durable file/blob claims: %#v", observations.RequiredKinds())
	}
	if len(observations.Observations) != 1 {
		t.Fatalf("expected exactly one vm state observation, got %#v", observations.Observations)
	}
	observation := observations.Observations[0]
	if observation.Kind != ObservationVMStateManifest {
		t.Fatalf("expected vm_state_manifest observation, got %#v", observation)
	}
	if observation.Key != "vmmanager:vm-boundary-1" {
		t.Fatalf("expected vmmanager-scoped observation key, got %q", observation.Key)
	}

	var payload struct {
		Substrate      string `json:"substrate"`
		VMID           string `json:"vm_id"`
		DataImagePath  string `json:"data_image_path"`
		DataImageClass string `json:"data_image_class"`
	}
	if err := json.Unmarshal([]byte(observation.Value), &payload); err != nil {
		t.Fatalf("vm state manifest should be valid json: %v", err)
	}
	if payload.Substrate != VMManagerSubstrateFirecracker || payload.VMID != "vm-boundary-1" || payload.DataImagePath != "/var/lib/choir/vm-boundary-1/data.img" || payload.DataImageClass != StateClassDurableLegacyOpaque {
		t.Fatalf("unexpected vm state manifest payload: %#v", payload)
	}
}

func TestVMManagerScopedRealizationsCompareEquivalentAndMismatchFails(t *testing.T) {
	version := vmManagerBoundaryComputerVersion()
	manifest := VMManagerCapabilityManifest("vmmanager-fixture")

	left := mustMaterializeVMManagerBoundary(t, "left", version, vmManagerBoundaryPath(), manifest)
	right := mustMaterializeVMManagerBoundary(t, "right", version, vmManagerBoundaryPath(), manifest)

	result := EquivalenceChecker{}.CheckRealizations(left, right)
	if !result.Equivalent() {
		t.Fatalf("expected identical vmmanager scoped realizations to compare equivalent, got %#v", result)
	}

	mismatchedPath := vmManagerBoundaryPath()
	mismatchedPath.DataImagePath = "/var/lib/choir/vm-boundary-1/other-data.img"
	mismatched := mustMaterializeVMManagerBoundary(t, "mismatched", version, mismatchedPath, manifest)

	result = EquivalenceChecker{}.CheckRealizations(left, mismatched)
	if result.Status != EquivalenceNotEquivalent {
		t.Fatalf("expected data image path mismatch to fail equivalence, got %#v", result)
	}
	if len(result.Differences) != 1 {
		t.Fatalf("expected exactly one vm state difference, got %#v", result.Differences)
	}
	diff := result.Differences[0]
	if diff.Kind != ObservationVMStateManifest || diff.Key != "vmmanager:vm-boundary-1" || diff.Left == diff.Right {
		t.Fatalf("unexpected vm state difference: %#v", diff)
	}
}

func TestVMManagerCapabilityManifestNarrowsOrBlocksDurableClaims(t *testing.T) {
	version := vmManagerBoundaryComputerVersion()
	manifest := VMManagerCapabilityManifest("vmmanager-fixture")

	left := Realization{
		ID:           "left",
		Version:      version,
		Capabilities: manifest,
		Observations: ObservationSet{
			Name:     "left-files",
			Version:  version,
			Required: []ObservationKind{ObservationFileManifest},
			Observations: []Observation{
				FileManifestObservation("/home/alice/note.txt", "sha256:aaa"),
			},
		},
	}
	right := left
	right.ID = "right"

	result := EquivalenceChecker{}.CheckRealizations(left, right)
	if result.Status != EquivalenceNarrowed {
		t.Fatalf("expected durable file-manifest claim to narrow under vmmanager capability, got %#v", result)
	}
	if !unsupportedContains(result.Unsupported, ObservationFileManifest) {
		t.Fatalf("expected unsupported file manifest capability, got %#v", result.Unsupported)
	}

	fileOnlyManifest := CapabilityManifest{
		Materializer: "vmmanager-fixture",
		Substrate:    VMManagerSubstrateFirecracker,
		Supported:    []ObservationKind{ObservationFileManifest},
	}
	realization, err := (VMManagerScopedMaterializer{
		ID:    "vmmanager-fixture",
		State: vmManagerBoundaryPath(),
	}).Materialize(context.Background(), version, fileOnlyManifest)
	if err == nil {
		t.Fatalf("expected manifest without vm_state_manifest support to be rejected, got realization %#v", realization)
	}
	if len(realization.Observations.Observations) != 0 || len(realization.Observations.Required) != 0 {
		t.Fatalf("rejected materializer must not emit observations, got %#v", realization.Observations)
	}
}

func TestVMManagerScopedMaterializerRejectsInvalidInputsBeforeClaim(t *testing.T) {
	validVersion := vmManagerBoundaryComputerVersion()
	validManifest := VMManagerCapabilityManifest("vmmanager-fixture")

	for _, tc := range []struct {
		name    string
		version ComputerVersion
		state   VMManagerScopedPath
	}{
		{
			name:    "missing computer version",
			version: ComputerVersion{},
			state:   vmManagerBoundaryPath(),
		},
		{
			name:    "missing vm id",
			version: validVersion,
			state: func() VMManagerScopedPath {
				state := vmManagerBoundaryPath()
				state.VMID = ""
				return state
			}(),
		},
		{
			name:    "missing state path",
			version: validVersion,
			state: func() VMManagerScopedPath {
				state := vmManagerBoundaryPath()
				state.PersistentDir = ""
				state.DataImagePath = ""
				return state
			}(),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			realization, err := (VMManagerScopedMaterializer{
				ID:    "vmmanager-fixture",
				State: tc.state,
			}).Materialize(context.Background(), tc.version, validManifest)
			if err == nil {
				t.Fatalf("expected invalid vmmanager input to be rejected, got realization %#v", realization)
			}
			if len(realization.Observations.Observations) != 0 || len(realization.Observations.Required) != 0 {
				t.Fatalf("invalid input must not emit observation claims, got %#v", realization.Observations)
			}
		})
	}
}

func vmManagerBoundaryComputerVersion() ComputerVersion {
	return ComputerVersion{CodeRef: "git:vmmanager-boundary", ArtifactProgramRef: "tape:owner/vmmanager-boundary"}
}

func vmManagerBoundaryPath() VMManagerScopedPath {
	return VMManagerScopedPath{
		VMID:               "vm-boundary-1",
		PersistentDir:      "/var/lib/choir/vm-boundary-1",
		DataImagePath:      "/var/lib/choir/vm-boundary-1/data.img",
		KernelImagePath:    "/nix/store/kernel/vmlinux",
		RootfsPath:         "/nix/store/rootfs/rootfs.ext4",
		StoreDiskPath:      "/var/lib/choir/vm-boundary-1/nix-store.ext4",
		ComputerKind:       "desktop",
		OwnerID:            "owner-1",
		DesktopID:          "desktop-1",
		WorkerID:           "worker-1",
		CandidateID:        "candidate-1",
		Epoch:              42,
		DataImageClass:     StateClassDurableLegacyOpaque,
		PersistentDirClass: StateClassDurableLegacyOpaque,
		BootArtifactClass:  StateClassCodeArtifact,
	}
}

func mustMaterializeVMManagerBoundary(t *testing.T, id string, version ComputerVersion, state VMManagerScopedPath, manifest CapabilityManifest) Realization {
	t.Helper()
	realization, err := (VMManagerScopedMaterializer{ID: id, State: state}).Materialize(context.Background(), version, manifest)
	if err != nil {
		t.Fatalf("materialize %s: %v", id, err)
	}
	return realization
}

func containsObservationKind(kinds []ObservationKind, want ObservationKind) bool {
	for _, kind := range kinds {
		if kind == want {
			return true
		}
	}
	return false
}

func unsupportedContains(capabilities []UnsupportedCapability, want ObservationKind) bool {
	for _, capability := range capabilities {
		if capability.Kind == want {
			return true
		}
	}
	return false
}
