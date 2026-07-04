package computerversion

import (
	"strings"
	"testing"
)

func TestBuildBaseMaterializerBoundaryContractBuildsScopedLocalMaterializerContract(t *testing.T) {
	realization, evidence := baseMaterializerBoundaryContractInputs(t)

	contract, err := BuildBaseMaterializerBoundaryContract(realization, evidence)
	if err != nil {
		t.Fatalf("BuildBaseMaterializerBoundaryContract(): %v", err)
	}

	if contract.Kind != BaseMaterializerBoundaryContractKind {
		t.Fatalf("kind = %q, want %q", contract.Kind, BaseMaterializerBoundaryContractKind)
	}
	if contract.Version != realization.Version {
		t.Fatalf("version = %#v, want %#v", contract.Version, realization.Version)
	}
	if contract.Boundary != BaseMaterializerBoundary {
		t.Fatalf("boundary = %q, want %q", contract.Boundary, BaseMaterializerBoundary)
	}
	if contract.Scope != BaseMaterializerScope {
		t.Fatalf("scope = %q, want %q", contract.Scope, BaseMaterializerScope)
	}
	if contract.RealizationID != realization.ID {
		t.Fatalf("realization id = %q, want %q", contract.RealizationID, realization.ID)
	}
	if contract.Materializer != realization.Capabilities.Materializer {
		t.Fatalf("materializer = %q, want %q", contract.Materializer, realization.Capabilities.Materializer)
	}
	if contract.Substrate != realization.Capabilities.Substrate {
		t.Fatalf("substrate = %q, want %q", contract.Substrate, realization.Capabilities.Substrate)
	}
	if contract.ObservationSetName != realization.Observations.Name {
		t.Fatalf("observation set name = %q, want %q", contract.ObservationSetName, realization.Observations.Name)
	}
	if contract.RealizationRef != evidence.RealizationRef {
		t.Fatalf("realization ref = %q, want %q", contract.RealizationRef, evidence.RealizationRef)
	}
	if contract.CapabilityManifestRef != evidence.CapabilityManifestRef {
		t.Fatalf("capability manifest ref = %q, want %q", contract.CapabilityManifestRef, evidence.CapabilityManifestRef)
	}
	if contract.ObservationSetRef != evidence.ObservationSetRef {
		t.Fatalf("observation set ref = %q, want %q", contract.ObservationSetRef, evidence.ObservationSetRef)
	}
	assertObservationBundleKinds(t, contract.RequiredObservations, []ObservationKind{ObservationBlobSet, ObservationFileManifest})
	if !contract.NoRuntimeMaterialization {
		t.Fatalf("no runtime materialization = false, want true")
	}
	if !contract.NoOpaqueDataImageDependency {
		t.Fatalf("no opaque data image dependency = false, want true")
	}
	if !contract.NoMutation {
		t.Fatalf("no mutation = false, want true")
	}
	if contract.RuntimeBehaviorChanged || contract.DeployedRouteRegistered || contract.ProductionAuthTouched || contract.StagingClaimed || contract.PromotionClaimed || contract.VMLifecycleTouched || contract.FirecrackerBootClaimed || contract.RunAcceptanceRecordTouched {
		t.Fatalf("runtime/deployed/auth/staging/promotion/VM/Firecracker/run-acceptance claims must remain false in built contract: %#v", contract)
	}
	if contract.FullSubstrateIndependenceClaim {
		t.Fatalf("full substrate-independence claim must remain false in built contract: %#v", contract)
	}
}

func TestBuildBaseMaterializerBoundaryContractRejectsInvalidInputs(t *testing.T) {
	foreignVersion := ComputerVersion{CodeRef: "git:foreign-base-materializer", ArtifactProgramRef: "base-journal:owner/main@cursor-foreign-base-materializer"}
	for _, tc := range []struct {
		name    string
		mutate  func(*Realization, *BaseMaterializerBoundaryEvidence)
		wantErr string
	}{
		{
			name: "missing realization id",
			mutate: func(realization *Realization, _ *BaseMaterializerBoundaryEvidence) {
				realization.ID = "  "
			},
			wantErr: "realization id is required",
		},
		{
			name: "invalid version",
			mutate: func(realization *Realization, _ *BaseMaterializerBoundaryEvidence) {
				realization.Version = ComputerVersion{CodeRef: "git:base-materializer-without-artifact-program"}
			},
			wantErr: "realization version is invalid",
		},
		{
			name: "missing materializer",
			mutate: func(realization *Realization, _ *BaseMaterializerBoundaryEvidence) {
				realization.Capabilities.Materializer = "\t"
			},
			wantErr: "materializer name is required",
		},
		{
			name: "missing substrate",
			mutate: func(realization *Realization, _ *BaseMaterializerBoundaryEvidence) {
				realization.Capabilities.Substrate = ""
			},
			wantErr: "substrate name is required",
		},
		{
			name: "missing observation set name",
			mutate: func(realization *Realization, _ *BaseMaterializerBoundaryEvidence) {
				realization.Observations.Name = "  "
			},
			wantErr: "observation set name is required",
		},
		{
			name: "observation version mismatch",
			mutate: func(realization *Realization, _ *BaseMaterializerBoundaryEvidence) {
				realization.Observations.Version = foreignVersion
			},
			wantErr: "observation set version does not match realization version",
		},
		{
			name: "empty observations",
			mutate: func(realization *Realization, _ *BaseMaterializerBoundaryEvidence) {
				realization.Observations.Observations = nil
			},
			wantErr: "observation set is empty",
		},
		{
			name: "missing file manifest",
			mutate: func(realization *Realization, _ *BaseMaterializerBoundaryEvidence) {
				realization.Observations.Required = []ObservationKind{ObservationBlobSet}
				realization.Observations.Observations = []Observation{{Kind: ObservationBlobSet, Key: "blob:sha256:base-materializer", Value: "sha256:blob-root"}}
			},
			wantErr: "realization must include file_manifest and blob_set",
		},
		{
			name: "missing blob set",
			mutate: func(realization *Realization, _ *BaseMaterializerBoundaryEvidence) {
				realization.Observations.Required = []ObservationKind{ObservationFileManifest}
				realization.Observations.Observations = []Observation{FileManifestObservation("/workspace/base.txt", "sha256:file-manifest-root")}
			},
			wantErr: "realization must include file_manifest and blob_set",
		},
		{
			name: "unsupported capability manifest",
			mutate: func(realization *Realization, _ *BaseMaterializerBoundaryEvidence) {
				realization.Capabilities.Unsupported = []UnsupportedCapability{{Kind: ObservationBlobSet, Reason: "local materializer intentionally narrows blobs"}}
			},
			wantErr: "capability manifest lacks required observation \"blob_set\"",
		},
		{
			name: "missing realization ref",
			mutate: func(_ *Realization, evidence *BaseMaterializerBoundaryEvidence) {
				evidence.RealizationRef = "\t"
			},
			wantErr: "realization ref is required",
		},
		{
			name: "missing capability manifest ref",
			mutate: func(_ *Realization, evidence *BaseMaterializerBoundaryEvidence) {
				evidence.CapabilityManifestRef = ""
			},
			wantErr: "capability manifest ref is required",
		},
		{
			name: "missing observation set ref",
			mutate: func(_ *Realization, evidence *BaseMaterializerBoundaryEvidence) {
				evidence.ObservationSetRef = "  "
			},
			wantErr: "observation set ref is required",
		},
		{
			name: "runtime materialization not excluded",
			mutate: func(_ *Realization, evidence *BaseMaterializerBoundaryEvidence) {
				evidence.NoRuntimeMaterialization = false
			},
			wantErr: "must prove no runtime materialization",
		},
		{
			name: "opaque data image dependency not excluded",
			mutate: func(_ *Realization, evidence *BaseMaterializerBoundaryEvidence) {
				evidence.NoOpaqueDataImageDependency = false
			},
			wantErr: "must prove no opaque data.img dependency",
		},
		{
			name: "runtime behavior changed",
			mutate: func(_ *Realization, evidence *BaseMaterializerBoundaryEvidence) {
				evidence.RuntimeBehaviorChanged = true
			},
			wantErr: "cannot change runtime behavior",
		},
		{
			name: "deployed route registered",
			mutate: func(_ *Realization, evidence *BaseMaterializerBoundaryEvidence) {
				evidence.DeployedRouteRegistered = true
			},
			wantErr: "cannot register deployed routes",
		},
		{
			name: "production auth touched",
			mutate: func(_ *Realization, evidence *BaseMaterializerBoundaryEvidence) {
				evidence.ProductionAuthTouched = true
			},
			wantErr: "cannot touch production auth/session",
		},
		{
			name: "staging claimed",
			mutate: func(_ *Realization, evidence *BaseMaterializerBoundaryEvidence) {
				evidence.StagingClaimed = true
			},
			wantErr: "cannot claim staging",
		},
		{
			name: "promotion claimed",
			mutate: func(_ *Realization, evidence *BaseMaterializerBoundaryEvidence) {
				evidence.PromotionClaimed = true
			},
			wantErr: "cannot claim promotion",
		},
		{
			name: "vm lifecycle touched",
			mutate: func(_ *Realization, evidence *BaseMaterializerBoundaryEvidence) {
				evidence.VMLifecycleTouched = true
			},
			wantErr: "cannot touch VM lifecycle",
		},
		{
			name: "firecracker boot claim",
			mutate: func(_ *Realization, evidence *BaseMaterializerBoundaryEvidence) {
				evidence.FirecrackerBootClaimed = true
			},
			wantErr: "cannot claim Firecracker boot",
		},
		{
			name: "run acceptance record touched",
			mutate: func(_ *Realization, evidence *BaseMaterializerBoundaryEvidence) {
				evidence.RunAcceptanceRecordTouched = true
			},
			wantErr: "cannot touch run acceptance records",
		},
		{
			name: "full substrate independence claim",
			mutate: func(_ *Realization, evidence *BaseMaterializerBoundaryEvidence) {
				evidence.FullSubstrateIndependenceClaim = true
			},
			wantErr: "cannot claim full substrate independence",
		},
		{
			name: "evidence no mutation false",
			mutate: func(_ *Realization, evidence *BaseMaterializerBoundaryEvidence) {
				evidence.NoMutation = false
			},
			wantErr: "evidence must be no-mutation",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			realization, evidence := baseMaterializerBoundaryContractInputs(t)
			tc.mutate(&realization, &evidence)

			contract, err := BuildBaseMaterializerBoundaryContract(realization, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildBaseMaterializerBoundaryContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func baseMaterializerBoundaryContractInputs(t *testing.T) (Realization, BaseMaterializerBoundaryEvidence) {
	t.Helper()

	version := ComputerVersion{CodeRef: "git:base-materializer-pass-91", ArtifactProgramRef: "base-journal:owner/main@cursor-base-materializer-pass-91"}
	realization := Realization{
		ID:      "base-file-blob-materializer-pass-91",
		Version: version,
		Capabilities: BaseCurrentStateCapabilityManifest(
			"base-file-blob-materializer-pass-91",
			"local-base-file-blob-store",
		),
		Observations: ObservationSet{
			Name:     "base-materializer-observation-set-pass-91",
			Version:  version,
			Required: []ObservationKind{ObservationFileManifest, ObservationBlobSet},
			Observations: []Observation{
				FileManifestObservation("/workspace/base.txt", "sha256:file-manifest-root"),
				{Kind: ObservationBlobSet, Key: "blob:sha256:base-materializer", Value: "sha256:blob-root"},
			},
		},
	}
	evidence := BaseMaterializerBoundaryEvidence{
		RealizationRef:              "realization:base-materializer-pass-91",
		CapabilityManifestRef:       "capability-manifest:base-materializer-pass-91",
		ObservationSetRef:           "observation-set:base-materializer-pass-91",
		NoRuntimeMaterialization:    true,
		NoOpaqueDataImageDependency: true,
		NoMutation:                  true,
	}
	return realization, evidence
}
