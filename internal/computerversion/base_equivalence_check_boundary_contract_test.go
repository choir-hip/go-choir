package computerversion

import (
	"strings"
	"testing"
)

func TestBuildBaseEquivalenceCheckBoundaryContractBuildsScopedNoMutationContract(t *testing.T) {
	left, right, result, evidence := baseEquivalenceCheckBoundaryContractInputs(t)

	contract, err := BuildBaseEquivalenceCheckBoundaryContract(left, right, result, evidence)
	if err != nil {
		t.Fatalf("BuildBaseEquivalenceCheckBoundaryContract(): %v", err)
	}

	if contract.Kind != BaseEquivalenceCheckBoundaryContractKind {
		t.Fatalf("kind = %q, want %q", contract.Kind, BaseEquivalenceCheckBoundaryContractKind)
	}
	if contract.Version != left.Version {
		t.Fatalf("version = %#v, want %#v", contract.Version, left.Version)
	}
	if contract.Boundary != BaseEquivalenceCheckBoundary {
		t.Fatalf("boundary = %q, want %q", contract.Boundary, BaseEquivalenceCheckBoundary)
	}
	if contract.Scope != BaseEquivalenceCheckScope {
		t.Fatalf("scope = %q, want %q", contract.Scope, BaseEquivalenceCheckScope)
	}
	if contract.LeftMaterializer != left.Materializer || contract.LeftSubstrate != left.Substrate {
		t.Fatalf("left identity = %q/%q, want %q/%q", contract.LeftMaterializer, contract.LeftSubstrate, left.Materializer, left.Substrate)
	}
	if contract.RightMaterializer != right.Materializer || contract.RightSubstrate != right.Substrate {
		t.Fatalf("right identity = %q/%q, want %q/%q", contract.RightMaterializer, contract.RightSubstrate, right.Materializer, right.Substrate)
	}
	assertObservationBundleKinds(t, contract.RequiredObservations, []ObservationKind{ObservationBlobSet, ObservationFileManifest})
	if contract.EquivalenceStatus != EquivalenceEquivalent {
		t.Fatalf("equivalence status = %q, want %q", contract.EquivalenceStatus, EquivalenceEquivalent)
	}
	if contract.LeftMaterializerContractRef != evidence.LeftMaterializerContractRef || contract.RightMaterializerContractRef != evidence.RightMaterializerContractRef || contract.EquivalenceResultRef != evidence.EquivalenceResultRef {
		t.Fatalf("refs = left %q right %q result %q, want %#v", contract.LeftMaterializerContractRef, contract.RightMaterializerContractRef, contract.EquivalenceResultRef, evidence)
	}
	if !contract.NoRuntimeMaterialization {
		t.Fatalf("no runtime materialization = false, want true")
	}
	if !contract.NoOpaqueDataImageDependency {
		t.Fatalf("no opaque data image dependency = false, want true")
	}
	if !contract.NoMutation {
		t.Fatalf("no mutation = false, want true")
	}
	if contract.RuntimeBehaviorChanged || contract.DeployedRouteRegistered || contract.ProductionAuthTouched || contract.StagingClaimed || contract.PromotionClaimed || contract.VMLifecycleTouched || contract.FirecrackerBootClaimed || contract.RunAcceptanceRecordTouched || contract.FullSubstrateIndependenceClaim {
		t.Fatalf("runtime/deployed/auth/staging/promotion/VM/Firecracker/run-acceptance/full-substrate-independence claims must remain false in built contract: %#v", contract)
	}
}

func TestBuildBaseEquivalenceCheckBoundaryContractRejectsInvalidInputs(t *testing.T) {
	foreignVersion := ComputerVersion{CodeRef: "git:foreign-base-equivalence-check", ArtifactProgramRef: "base-journal:owner/main@cursor-foreign-base-equivalence-check"}
	for _, tc := range []struct {
		name    string
		mutate  func(*BaseMaterializerBoundaryContract, *BaseMaterializerBoundaryContract, *EquivalenceResult, *BaseEquivalenceCheckBoundaryEvidence)
		wantErr string
	}{
		{
			name: "wrong left materializer contract kind",
			mutate: func(left *BaseMaterializerBoundaryContract, _ *BaseMaterializerBoundaryContract, _ *EquivalenceResult, _ *BaseEquivalenceCheckBoundaryEvidence) {
				left.Kind = "base_substrate_equivalence_contract"
			},
			wantErr: "left materializer contract kind",
		},
		{
			name: "wrong right materializer contract kind",
			mutate: func(_ *BaseMaterializerBoundaryContract, right *BaseMaterializerBoundaryContract, _ *EquivalenceResult, _ *BaseEquivalenceCheckBoundaryEvidence) {
				right.Kind = "base_substrate_equivalence_contract"
			},
			wantErr: "right materializer contract kind",
		},
		{
			name: "wrong left materializer scope",
			mutate: func(left *BaseMaterializerBoundaryContract, _ *BaseMaterializerBoundaryContract, _ *EquivalenceResult, _ *BaseEquivalenceCheckBoundaryEvidence) {
				left.Scope = "runtime_materialization_scope"
			},
			wantErr: "left materializer scope",
		},
		{
			name: "wrong right materializer scope",
			mutate: func(_ *BaseMaterializerBoundaryContract, right *BaseMaterializerBoundaryContract, _ *EquivalenceResult, _ *BaseEquivalenceCheckBoundaryEvidence) {
				right.Scope = "runtime_materialization_scope"
			},
			wantErr: "right materializer scope",
		},
		{
			name: "invalid materializer contract version",
			mutate: func(left *BaseMaterializerBoundaryContract, _ *BaseMaterializerBoundaryContract, _ *EquivalenceResult, _ *BaseEquivalenceCheckBoundaryEvidence) {
				left.Version = ComputerVersion{CodeRef: "git:base-equivalence-without-artifact-program"}
			},
			wantErr: "left materializer contract version is invalid",
		},
		{
			name: "missing materializer",
			mutate: func(left *BaseMaterializerBoundaryContract, _ *BaseMaterializerBoundaryContract, _ *EquivalenceResult, _ *BaseEquivalenceCheckBoundaryEvidence) {
				left.Materializer = "\t"
			},
			wantErr: "left materializer/substrate is required",
		},
		{
			name: "missing substrate",
			mutate: func(_ *BaseMaterializerBoundaryContract, right *BaseMaterializerBoundaryContract, _ *EquivalenceResult, _ *BaseEquivalenceCheckBoundaryEvidence) {
				right.Substrate = "  "
			},
			wantErr: "right materializer/substrate is required",
		},
		{
			name: "missing file manifest required observation",
			mutate: func(left *BaseMaterializerBoundaryContract, _ *BaseMaterializerBoundaryContract, _ *EquivalenceResult, _ *BaseEquivalenceCheckBoundaryEvidence) {
				left.RequiredObservations = []ObservationKind{ObservationBlobSet}
			},
			wantErr: "must require file_manifest and blob_set",
		},
		{
			name: "missing blob set required observation",
			mutate: func(_ *BaseMaterializerBoundaryContract, right *BaseMaterializerBoundaryContract, _ *EquivalenceResult, _ *BaseEquivalenceCheckBoundaryEvidence) {
				right.RequiredObservations = []ObservationKind{ObservationFileManifest}
			},
			wantErr: "must require file_manifest and blob_set",
		},
		{
			name: "left runtime materialization not excluded",
			mutate: func(left *BaseMaterializerBoundaryContract, _ *BaseMaterializerBoundaryContract, _ *EquivalenceResult, _ *BaseEquivalenceCheckBoundaryEvidence) {
				left.NoRuntimeMaterialization = false
			},
			wantErr: "left materializer contract must be local no-mutation evidence",
		},
		{
			name: "left opaque data image dependency not excluded",
			mutate: func(left *BaseMaterializerBoundaryContract, _ *BaseMaterializerBoundaryContract, _ *EquivalenceResult, _ *BaseEquivalenceCheckBoundaryEvidence) {
				left.NoOpaqueDataImageDependency = false
			},
			wantErr: "left materializer contract must be local no-mutation evidence",
		},
		{
			name: "right no mutation false",
			mutate: func(_ *BaseMaterializerBoundaryContract, right *BaseMaterializerBoundaryContract, _ *EquivalenceResult, _ *BaseEquivalenceCheckBoundaryEvidence) {
				right.NoMutation = false
			},
			wantErr: "right materializer contract must be local no-mutation evidence",
		},
		{
			name: "left materializer runtime behavior changed",
			mutate: func(left *BaseMaterializerBoundaryContract, _ *BaseMaterializerBoundaryContract, _ *EquivalenceResult, _ *BaseEquivalenceCheckBoundaryEvidence) {
				left.RuntimeBehaviorChanged = true
			},
			wantErr: "left materializer contract must be local no-mutation evidence",
		},
		{
			name: "right materializer deployed route registered",
			mutate: func(_ *BaseMaterializerBoundaryContract, right *BaseMaterializerBoundaryContract, _ *EquivalenceResult, _ *BaseEquivalenceCheckBoundaryEvidence) {
				right.DeployedRouteRegistered = true
			},
			wantErr: "right materializer contract must be local no-mutation evidence",
		},
		{
			name: "left materializer production auth touched",
			mutate: func(left *BaseMaterializerBoundaryContract, _ *BaseMaterializerBoundaryContract, _ *EquivalenceResult, _ *BaseEquivalenceCheckBoundaryEvidence) {
				left.ProductionAuthTouched = true
			},
			wantErr: "left materializer contract must be local no-mutation evidence",
		},
		{
			name: "right materializer staging claimed",
			mutate: func(_ *BaseMaterializerBoundaryContract, right *BaseMaterializerBoundaryContract, _ *EquivalenceResult, _ *BaseEquivalenceCheckBoundaryEvidence) {
				right.StagingClaimed = true
			},
			wantErr: "right materializer contract must be local no-mutation evidence",
		},
		{
			name: "left materializer promotion claimed",
			mutate: func(left *BaseMaterializerBoundaryContract, _ *BaseMaterializerBoundaryContract, _ *EquivalenceResult, _ *BaseEquivalenceCheckBoundaryEvidence) {
				left.PromotionClaimed = true
			},
			wantErr: "left materializer contract must be local no-mutation evidence",
		},
		{
			name: "right materializer vm lifecycle touched",
			mutate: func(_ *BaseMaterializerBoundaryContract, right *BaseMaterializerBoundaryContract, _ *EquivalenceResult, _ *BaseEquivalenceCheckBoundaryEvidence) {
				right.VMLifecycleTouched = true
			},
			wantErr: "right materializer contract must be local no-mutation evidence",
		},
		{
			name: "left materializer Firecracker boot claimed",
			mutate: func(left *BaseMaterializerBoundaryContract, _ *BaseMaterializerBoundaryContract, _ *EquivalenceResult, _ *BaseEquivalenceCheckBoundaryEvidence) {
				left.FirecrackerBootClaimed = true
			},
			wantErr: "left materializer contract must be local no-mutation evidence",
		},
		{
			name: "right materializer run acceptance record touched",
			mutate: func(_ *BaseMaterializerBoundaryContract, right *BaseMaterializerBoundaryContract, _ *EquivalenceResult, _ *BaseEquivalenceCheckBoundaryEvidence) {
				right.RunAcceptanceRecordTouched = true
			},
			wantErr: "right materializer contract must be local no-mutation evidence",
		},
		{
			name: "left materializer full substrate independence claimed",
			mutate: func(left *BaseMaterializerBoundaryContract, _ *BaseMaterializerBoundaryContract, _ *EquivalenceResult, _ *BaseEquivalenceCheckBoundaryEvidence) {
				left.FullSubstrateIndependenceClaim = true
			},
			wantErr: "left materializer contract must be local no-mutation evidence",
		},
		{
			name: "materializer contract version drift",
			mutate: func(_ *BaseMaterializerBoundaryContract, right *BaseMaterializerBoundaryContract, _ *EquivalenceResult, _ *BaseEquivalenceCheckBoundaryEvidence) {
				right.Version = foreignVersion
			},
			wantErr: "materializer contracts name different computer versions",
		},
		{
			name: "identical materializer and substrate self comparison",
			mutate: func(left *BaseMaterializerBoundaryContract, right *BaseMaterializerBoundaryContract, _ *EquivalenceResult, _ *BaseEquivalenceCheckBoundaryEvidence) {
				right.Materializer = left.Materializer
				right.Substrate = left.Substrate
			},
			wantErr: "materializer contracts must be non-identical",
		},
		{
			name: "not equivalent result",
			mutate: func(_ *BaseMaterializerBoundaryContract, _ *BaseMaterializerBoundaryContract, result *EquivalenceResult, _ *BaseEquivalenceCheckBoundaryEvidence) {
				result.Status = EquivalenceNotEquivalent
			},
			wantErr: "not equivalent",
		},
		{
			name: "narrowed result",
			mutate: func(_ *BaseMaterializerBoundaryContract, _ *BaseMaterializerBoundaryContract, result *EquivalenceResult, _ *BaseEquivalenceCheckBoundaryEvidence) {
				result.Status = EquivalenceNarrowed
				result.Unsupported = []UnsupportedCapability{{Kind: ObservationFileManifest, Reason: "right materializer narrowed file manifests"}}
			},
			wantErr: "not equivalent",
		},
		{
			name: "equivalent result with differences",
			mutate: func(_ *BaseMaterializerBoundaryContract, _ *BaseMaterializerBoundaryContract, result *EquivalenceResult, _ *BaseEquivalenceCheckBoundaryEvidence) {
				result.Differences = []Difference{{Kind: ObservationBlobSet, Key: "blob:sha256:base-equivalence", Left: "sha256:left", Right: "sha256:right", Reason: "blob root mismatch"}}
			},
			wantErr: "not equivalent",
		},
		{
			name: "equivalent result with unsupported capability",
			mutate: func(_ *BaseMaterializerBoundaryContract, _ *BaseMaterializerBoundaryContract, result *EquivalenceResult, _ *BaseEquivalenceCheckBoundaryEvidence) {
				result.Unsupported = []UnsupportedCapability{{Kind: ObservationBlobSet, Reason: "blob set excluded by materializer"}}
			},
			wantErr: "not equivalent",
		},
		{
			name: "missing left materializer contract ref",
			mutate: func(_ *BaseMaterializerBoundaryContract, _ *BaseMaterializerBoundaryContract, _ *EquivalenceResult, evidence *BaseEquivalenceCheckBoundaryEvidence) {
				evidence.LeftMaterializerContractRef = "  "
			},
			wantErr: "left materializer contract ref is required",
		},
		{
			name: "missing right materializer contract ref",
			mutate: func(_ *BaseMaterializerBoundaryContract, _ *BaseMaterializerBoundaryContract, _ *EquivalenceResult, evidence *BaseEquivalenceCheckBoundaryEvidence) {
				evidence.RightMaterializerContractRef = ""
			},
			wantErr: "right materializer contract ref is required",
		},
		{
			name: "missing equivalence result ref",
			mutate: func(_ *BaseMaterializerBoundaryContract, _ *BaseMaterializerBoundaryContract, _ *EquivalenceResult, evidence *BaseEquivalenceCheckBoundaryEvidence) {
				evidence.EquivalenceResultRef = "\t"
			},
			wantErr: "equivalence result ref is required",
		},
		{
			name: "evidence runtime materialization not excluded",
			mutate: func(_ *BaseMaterializerBoundaryContract, _ *BaseMaterializerBoundaryContract, _ *EquivalenceResult, evidence *BaseEquivalenceCheckBoundaryEvidence) {
				evidence.NoRuntimeMaterialization = false
			},
			wantErr: "must prove no runtime materialization",
		},
		{
			name: "evidence opaque data image dependency not excluded",
			mutate: func(_ *BaseMaterializerBoundaryContract, _ *BaseMaterializerBoundaryContract, _ *EquivalenceResult, evidence *BaseEquivalenceCheckBoundaryEvidence) {
				evidence.NoOpaqueDataImageDependency = false
			},
			wantErr: "must prove no opaque data.img dependency",
		},
		{
			name: "evidence runtime behavior changed",
			mutate: func(_ *BaseMaterializerBoundaryContract, _ *BaseMaterializerBoundaryContract, _ *EquivalenceResult, evidence *BaseEquivalenceCheckBoundaryEvidence) {
				evidence.RuntimeBehaviorChanged = true
			},
			wantErr: "cannot change runtime behavior",
		},
		{
			name: "evidence deployed route registered",
			mutate: func(_ *BaseMaterializerBoundaryContract, _ *BaseMaterializerBoundaryContract, _ *EquivalenceResult, evidence *BaseEquivalenceCheckBoundaryEvidence) {
				evidence.DeployedRouteRegistered = true
			},
			wantErr: "cannot register deployed routes",
		},
		{
			name: "evidence production auth touched",
			mutate: func(_ *BaseMaterializerBoundaryContract, _ *BaseMaterializerBoundaryContract, _ *EquivalenceResult, evidence *BaseEquivalenceCheckBoundaryEvidence) {
				evidence.ProductionAuthTouched = true
			},
			wantErr: "cannot touch production auth/session",
		},
		{
			name: "evidence staging claimed",
			mutate: func(_ *BaseMaterializerBoundaryContract, _ *BaseMaterializerBoundaryContract, _ *EquivalenceResult, evidence *BaseEquivalenceCheckBoundaryEvidence) {
				evidence.StagingClaimed = true
			},
			wantErr: "cannot claim staging",
		},
		{
			name: "evidence promotion claimed",
			mutate: func(_ *BaseMaterializerBoundaryContract, _ *BaseMaterializerBoundaryContract, _ *EquivalenceResult, evidence *BaseEquivalenceCheckBoundaryEvidence) {
				evidence.PromotionClaimed = true
			},
			wantErr: "cannot claim promotion",
		},
		{
			name: "evidence vm lifecycle touched",
			mutate: func(_ *BaseMaterializerBoundaryContract, _ *BaseMaterializerBoundaryContract, _ *EquivalenceResult, evidence *BaseEquivalenceCheckBoundaryEvidence) {
				evidence.VMLifecycleTouched = true
			},
			wantErr: "cannot touch VM lifecycle",
		},
		{
			name: "evidence run acceptance record touched",
			mutate: func(_ *BaseMaterializerBoundaryContract, _ *BaseMaterializerBoundaryContract, _ *EquivalenceResult, evidence *BaseEquivalenceCheckBoundaryEvidence) {
				evidence.RunAcceptanceRecordTouched = true
			},
			wantErr: "cannot touch run acceptance records",
		},
		{
			name: "evidence Firecracker boot claimed",
			mutate: func(_ *BaseMaterializerBoundaryContract, _ *BaseMaterializerBoundaryContract, _ *EquivalenceResult, evidence *BaseEquivalenceCheckBoundaryEvidence) {
				evidence.FirecrackerBootClaimed = true
			},
			wantErr: "cannot claim Firecracker boot",
		},
		{
			name: "evidence full substrate independence claimed",
			mutate: func(_ *BaseMaterializerBoundaryContract, _ *BaseMaterializerBoundaryContract, _ *EquivalenceResult, evidence *BaseEquivalenceCheckBoundaryEvidence) {
				evidence.FullSubstrateIndependenceClaim = true
			},
			wantErr: "cannot claim full substrate independence",
		},
		{
			name: "evidence no mutation false",
			mutate: func(_ *BaseMaterializerBoundaryContract, _ *BaseMaterializerBoundaryContract, _ *EquivalenceResult, evidence *BaseEquivalenceCheckBoundaryEvidence) {
				evidence.NoMutation = false
			},
			wantErr: "evidence must be no-mutation",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			left, right, result, evidence := baseEquivalenceCheckBoundaryContractInputs(t)
			tc.mutate(&left, &right, &result, &evidence)

			contract, err := BuildBaseEquivalenceCheckBoundaryContract(left, right, result, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildBaseEquivalenceCheckBoundaryContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func baseEquivalenceCheckBoundaryContractInputs(t *testing.T) (BaseMaterializerBoundaryContract, BaseMaterializerBoundaryContract, EquivalenceResult, BaseEquivalenceCheckBoundaryEvidence) {
	t.Helper()

	version := ComputerVersion{CodeRef: "git:base-equivalence-check-pass-92", ArtifactProgramRef: "base-journal:owner/main@cursor-base-equivalence-check-pass-92"}
	left := BaseMaterializerBoundaryContract{
		Kind:                        BaseMaterializerBoundaryContractKind,
		Version:                     version,
		Boundary:                    BaseMaterializerBoundary,
		Scope:                       BaseMaterializerScope,
		RealizationID:               "base-equivalence-left-realization-pass-92",
		Materializer:                "base-equivalence-left-materializer-pass-92",
		Substrate:                   "local-base-left-file-blob-store",
		ObservationSetName:          "base-equivalence-left-observation-set-pass-92",
		RequiredObservations:        []ObservationKind{ObservationFileManifest, ObservationBlobSet},
		RealizationRef:              "realization:base-equivalence-left-pass-92",
		CapabilityManifestRef:       "capability-manifest:base-equivalence-left-pass-92",
		ObservationSetRef:           "observation-set:base-equivalence-left-pass-92",
		NoRuntimeMaterialization:    true,
		NoOpaqueDataImageDependency: true,
		NoMutation:                  true,
	}
	right := BaseMaterializerBoundaryContract{
		Kind:                        BaseMaterializerBoundaryContractKind,
		Version:                     version,
		Boundary:                    BaseMaterializerBoundary,
		Scope:                       BaseMaterializerScope,
		RealizationID:               "base-equivalence-right-realization-pass-92",
		Materializer:                "base-equivalence-right-materializer-pass-92",
		Substrate:                   "local-base-right-file-blob-store",
		ObservationSetName:          "base-equivalence-right-observation-set-pass-92",
		RequiredObservations:        []ObservationKind{ObservationBlobSet, ObservationFileManifest},
		RealizationRef:              "realization:base-equivalence-right-pass-92",
		CapabilityManifestRef:       "capability-manifest:base-equivalence-right-pass-92",
		ObservationSetRef:           "observation-set:base-equivalence-right-pass-92",
		NoRuntimeMaterialization:    true,
		NoOpaqueDataImageDependency: true,
		NoMutation:                  true,
	}
	result := EquivalenceResult{Status: EquivalenceEquivalent}
	evidence := BaseEquivalenceCheckBoundaryEvidence{
		LeftMaterializerContractRef:  "base-materializer-contract:left-pass-92",
		RightMaterializerContractRef: "base-materializer-contract:right-pass-92",
		EquivalenceResultRef:         "equivalence-result:base-equivalence-pass-92",
		NoRuntimeMaterialization:     true,
		NoOpaqueDataImageDependency:  true,
		NoMutation:                   true,
	}
	return left, right, result, evidence
}
