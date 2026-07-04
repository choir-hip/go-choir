package computerversion

import (
	"strings"
	"testing"
)

func TestBuildBaseEquivalenceFailureBoundaryContractBuildsNonEquivalentBoundaryContract(t *testing.T) {
	left, right, result, evidence := baseEquivalenceFailureBoundaryContractInputs(t)

	contract, err := BuildBaseEquivalenceFailureBoundaryContract(left, right, result, evidence)
	if err != nil {
		t.Fatalf("BuildBaseEquivalenceFailureBoundaryContract(): %v", err)
	}

	if contract.Kind != BaseEquivalenceFailureBoundaryContractKind {
		t.Fatalf("kind = %q, want %q", contract.Kind, BaseEquivalenceFailureBoundaryContractKind)
	}
	if contract.Version != left.Version {
		t.Fatalf("version = %#v, want %#v", contract.Version, left.Version)
	}
	if contract.Boundary != BaseEquivalenceFailureBoundary {
		t.Fatalf("boundary = %q, want %q", contract.Boundary, BaseEquivalenceFailureBoundary)
	}
	if contract.Scope != BaseEquivalenceFailureScope {
		t.Fatalf("scope = %q, want %q", contract.Scope, BaseEquivalenceFailureScope)
	}
	if contract.LeftMaterializer != left.Materializer || contract.LeftSubstrate != left.Substrate {
		t.Fatalf("left identity = %q/%q, want %q/%q", contract.LeftMaterializer, contract.LeftSubstrate, left.Materializer, left.Substrate)
	}
	if contract.RightMaterializer != right.Materializer || contract.RightSubstrate != right.Substrate {
		t.Fatalf("right identity = %q/%q, want %q/%q", contract.RightMaterializer, contract.RightSubstrate, right.Materializer, right.Substrate)
	}
	assertObservationBundleKinds(t, contract.RequiredObservations, []ObservationKind{ObservationBlobSet, ObservationFileManifest})
	if contract.FailureStatus != EquivalenceNotEquivalent {
		t.Fatalf("failure status = %q, want %q", contract.FailureStatus, EquivalenceNotEquivalent)
	}
	if contract.DifferenceCount != 1 || contract.UnsupportedCapabilityCount != 0 {
		t.Fatalf("counts = differences %d unsupported %d, want 1/0", contract.DifferenceCount, contract.UnsupportedCapabilityCount)
	}
	if contract.LeftMaterializerContractRef != evidence.LeftMaterializerContractRef || contract.RightMaterializerContractRef != evidence.RightMaterializerContractRef || contract.FailureResultRef != evidence.FailureResultRef || contract.FailureFixtureRef != evidence.FailureFixtureRef {
		t.Fatalf("refs = left %q right %q result %q fixture %q, want %#v", contract.LeftMaterializerContractRef, contract.RightMaterializerContractRef, contract.FailureResultRef, contract.FailureFixtureRef, evidence)
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
	if contract.RuntimeBehaviorChanged || contract.DeployedRouteRegistered || contract.ProductionAuthTouched || contract.StagingClaimed || contract.PromotionClaimed || contract.VMLifecycleTouched || contract.FirecrackerBootClaimed || contract.RunAcceptanceRecordTouched || contract.SuccessfulEquivalenceClaimed || contract.FullSubstrateIndependenceClaim {
		t.Fatalf("runtime/deployed/auth/staging/promotion/VM/Firecracker/run-acceptance/success/full-substrate-independence claims must remain false in built contract: %#v", contract)
	}
}

func TestBuildBaseEquivalenceFailureBoundaryContractBuildsNarrowedBoundaryContract(t *testing.T) {
	left, right, _, evidence := baseEquivalenceFailureBoundaryContractInputs(t)
	result := EquivalenceResult{
		Status: EquivalenceNarrowed,
		Unsupported: []UnsupportedCapability{
			{Kind: ObservationFileManifest, Reason: "right materializer cannot expose file manifests for the seeded fixture"},
			{Kind: ObservationBlobSet, Reason: "right materializer cannot expose blob integrity for the seeded fixture"},
		},
	}

	contract, err := BuildBaseEquivalenceFailureBoundaryContract(left, right, result, evidence)
	if err != nil {
		t.Fatalf("BuildBaseEquivalenceFailureBoundaryContract(): %v", err)
	}

	if contract.FailureStatus != EquivalenceNarrowed {
		t.Fatalf("failure status = %q, want %q", contract.FailureStatus, EquivalenceNarrowed)
	}
	if contract.DifferenceCount != 0 || contract.UnsupportedCapabilityCount != len(result.Unsupported) {
		t.Fatalf("counts = differences %d unsupported %d, want 0/%d", contract.DifferenceCount, contract.UnsupportedCapabilityCount, len(result.Unsupported))
	}
	if !contract.NoRuntimeMaterialization || !contract.NoOpaqueDataImageDependency || !contract.NoMutation {
		t.Fatalf("narrowed failure boundary must remain local no-mutation evidence: %#v", contract)
	}
	if contract.SuccessfulEquivalenceClaimed {
		t.Fatalf("narrowed failure boundary claimed successful equivalence: %#v", contract)
	}
}

func TestBuildBaseEquivalenceFailureBoundaryContractRejectsInvalidInputs(t *testing.T) {
	foreignVersion := ComputerVersion{CodeRef: "git:foreign-base-equivalence-failure", ArtifactProgramRef: "base-journal:owner/main@cursor-foreign-base-equivalence-failure"}
	for _, tc := range []struct {
		name    string
		mutate  func(*BaseMaterializerBoundaryContract, *BaseMaterializerBoundaryContract, *EquivalenceResult, *BaseEquivalenceFailureBoundaryEvidence)
		wantErr string
	}{
		{
			name: "equivalent result",
			mutate: func(_ *BaseMaterializerBoundaryContract, _ *BaseMaterializerBoundaryContract, result *EquivalenceResult, _ *BaseEquivalenceFailureBoundaryEvidence) {
				*result = EquivalenceResult{Status: EquivalenceEquivalent}
			},
			wantErr: "equivalent result is not failure evidence",
		},
		{
			name: "not equivalent without differences",
			mutate: func(_ *BaseMaterializerBoundaryContract, _ *BaseMaterializerBoundaryContract, result *EquivalenceResult, _ *BaseEquivalenceFailureBoundaryEvidence) {
				result.Differences = nil
			},
			wantErr: "not_equivalent result must include differences",
		},
		{
			name: "not equivalent with unsupported mixed in",
			mutate: func(_ *BaseMaterializerBoundaryContract, _ *BaseMaterializerBoundaryContract, result *EquivalenceResult, _ *BaseEquivalenceFailureBoundaryEvidence) {
				result.Unsupported = []UnsupportedCapability{{Kind: ObservationBlobSet, Reason: "fixture also claimed unsupported blobs"}}
			},
			wantErr: "not_equivalent result cannot include unsupported capabilities",
		},
		{
			name: "narrowed without unsupported capabilities",
			mutate: func(_ *BaseMaterializerBoundaryContract, _ *BaseMaterializerBoundaryContract, result *EquivalenceResult, _ *BaseEquivalenceFailureBoundaryEvidence) {
				*result = EquivalenceResult{Status: EquivalenceNarrowed}
			},
			wantErr: "narrowed result must include unsupported capabilities",
		},
		{
			name: "narrowed with differences mixed in",
			mutate: func(_ *BaseMaterializerBoundaryContract, _ *BaseMaterializerBoundaryContract, result *EquivalenceResult, _ *BaseEquivalenceFailureBoundaryEvidence) {
				result.Status = EquivalenceNarrowed
				result.Unsupported = []UnsupportedCapability{{Kind: ObservationFileManifest, Reason: "fixture narrowed file manifests"}}
			},
			wantErr: "narrowed result cannot include differences",
		},
		{
			name: "invalid left materializer contract",
			mutate: func(left *BaseMaterializerBoundaryContract, _ *BaseMaterializerBoundaryContract, _ *EquivalenceResult, _ *BaseEquivalenceFailureBoundaryEvidence) {
				left.Kind = "base_substrate_equivalence_contract"
			},
			wantErr: "left materializer contract kind",
		},
		{
			name: "invalid right materializer contract",
			mutate: func(_ *BaseMaterializerBoundaryContract, right *BaseMaterializerBoundaryContract, _ *EquivalenceResult, _ *BaseEquivalenceFailureBoundaryEvidence) {
				right.Scope = "runtime_materialization_scope"
			},
			wantErr: "right materializer scope",
		},
		{
			name: "materializer contract version drift",
			mutate: func(_ *BaseMaterializerBoundaryContract, right *BaseMaterializerBoundaryContract, _ *EquivalenceResult, _ *BaseEquivalenceFailureBoundaryEvidence) {
				right.Version = foreignVersion
			},
			wantErr: "materializer contracts name different computer versions",
		},
		{
			name: "materializer substrate self comparison",
			mutate: func(left *BaseMaterializerBoundaryContract, right *BaseMaterializerBoundaryContract, _ *EquivalenceResult, _ *BaseEquivalenceFailureBoundaryEvidence) {
				right.Materializer = left.Materializer
				right.Substrate = left.Substrate
			},
			wantErr: "materializer contracts must be non-identical",
		},
		{
			name: "missing left materializer contract ref",
			mutate: func(_ *BaseMaterializerBoundaryContract, _ *BaseMaterializerBoundaryContract, _ *EquivalenceResult, evidence *BaseEquivalenceFailureBoundaryEvidence) {
				evidence.LeftMaterializerContractRef = "  "
			},
			wantErr: "left materializer contract ref is required",
		},
		{
			name: "missing right materializer contract ref",
			mutate: func(_ *BaseMaterializerBoundaryContract, _ *BaseMaterializerBoundaryContract, _ *EquivalenceResult, evidence *BaseEquivalenceFailureBoundaryEvidence) {
				evidence.RightMaterializerContractRef = ""
			},
			wantErr: "right materializer contract ref is required",
		},
		{
			name: "missing failure result ref",
			mutate: func(_ *BaseMaterializerBoundaryContract, _ *BaseMaterializerBoundaryContract, _ *EquivalenceResult, evidence *BaseEquivalenceFailureBoundaryEvidence) {
				evidence.FailureResultRef = "\t"
			},
			wantErr: "failure result ref is required",
		},
		{
			name: "missing failure fixture ref",
			mutate: func(_ *BaseMaterializerBoundaryContract, _ *BaseMaterializerBoundaryContract, _ *EquivalenceResult, evidence *BaseEquivalenceFailureBoundaryEvidence) {
				evidence.FailureFixtureRef = " "
			},
			wantErr: "failure fixture ref is required",
		},
		{
			name: "successful equivalence claimed",
			mutate: func(_ *BaseMaterializerBoundaryContract, _ *BaseMaterializerBoundaryContract, _ *EquivalenceResult, evidence *BaseEquivalenceFailureBoundaryEvidence) {
				evidence.SuccessfulEquivalenceClaimed = true
			},
			wantErr: "cannot claim successful equivalence",
		},
		{
			name: "runtime materialization not excluded",
			mutate: func(_ *BaseMaterializerBoundaryContract, _ *BaseMaterializerBoundaryContract, _ *EquivalenceResult, evidence *BaseEquivalenceFailureBoundaryEvidence) {
				evidence.NoRuntimeMaterialization = false
			},
			wantErr: "must prove no runtime materialization",
		},
		{
			name: "opaque data image dependency not excluded",
			mutate: func(_ *BaseMaterializerBoundaryContract, _ *BaseMaterializerBoundaryContract, _ *EquivalenceResult, evidence *BaseEquivalenceFailureBoundaryEvidence) {
				evidence.NoOpaqueDataImageDependency = false
			},
			wantErr: "must prove no opaque data.img dependency",
		},
		{
			name: "deployed route registered",
			mutate: func(_ *BaseMaterializerBoundaryContract, _ *BaseMaterializerBoundaryContract, _ *EquivalenceResult, evidence *BaseEquivalenceFailureBoundaryEvidence) {
				evidence.DeployedRouteRegistered = true
			},
			wantErr: "cannot register deployed routes",
		},
		{
			name: "production auth touched",
			mutate: func(_ *BaseMaterializerBoundaryContract, _ *BaseMaterializerBoundaryContract, _ *EquivalenceResult, evidence *BaseEquivalenceFailureBoundaryEvidence) {
				evidence.ProductionAuthTouched = true
			},
			wantErr: "cannot touch production auth/session",
		},
		{
			name: "staging claimed",
			mutate: func(_ *BaseMaterializerBoundaryContract, _ *BaseMaterializerBoundaryContract, _ *EquivalenceResult, evidence *BaseEquivalenceFailureBoundaryEvidence) {
				evidence.StagingClaimed = true
			},
			wantErr: "cannot claim staging",
		},
		{
			name: "promotion claimed",
			mutate: func(_ *BaseMaterializerBoundaryContract, _ *BaseMaterializerBoundaryContract, _ *EquivalenceResult, evidence *BaseEquivalenceFailureBoundaryEvidence) {
				evidence.PromotionClaimed = true
			},
			wantErr: "cannot claim promotion",
		},
		{
			name: "vm lifecycle touched",
			mutate: func(_ *BaseMaterializerBoundaryContract, _ *BaseMaterializerBoundaryContract, _ *EquivalenceResult, evidence *BaseEquivalenceFailureBoundaryEvidence) {
				evidence.VMLifecycleTouched = true
			},
			wantErr: "cannot touch VM lifecycle",
		},
		{
			name: "Firecracker boot claimed",
			mutate: func(_ *BaseMaterializerBoundaryContract, _ *BaseMaterializerBoundaryContract, _ *EquivalenceResult, evidence *BaseEquivalenceFailureBoundaryEvidence) {
				evidence.FirecrackerBootClaimed = true
			},
			wantErr: "cannot claim Firecracker boot",
		},
		{
			name: "run acceptance record touched",
			mutate: func(_ *BaseMaterializerBoundaryContract, _ *BaseMaterializerBoundaryContract, _ *EquivalenceResult, evidence *BaseEquivalenceFailureBoundaryEvidence) {
				evidence.RunAcceptanceRecordTouched = true
			},
			wantErr: "cannot touch run acceptance records",
		},
		{
			name: "full substrate independence claimed",
			mutate: func(_ *BaseMaterializerBoundaryContract, _ *BaseMaterializerBoundaryContract, _ *EquivalenceResult, evidence *BaseEquivalenceFailureBoundaryEvidence) {
				evidence.FullSubstrateIndependenceClaim = true
			},
			wantErr: "cannot claim full substrate independence",
		},
		{
			name: "evidence no mutation false",
			mutate: func(_ *BaseMaterializerBoundaryContract, _ *BaseMaterializerBoundaryContract, _ *EquivalenceResult, evidence *BaseEquivalenceFailureBoundaryEvidence) {
				evidence.NoMutation = false
			},
			wantErr: "evidence must be no-mutation",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			left, right, result, evidence := baseEquivalenceFailureBoundaryContractInputs(t)
			tc.mutate(&left, &right, &result, &evidence)

			contract, err := BuildBaseEquivalenceFailureBoundaryContract(left, right, result, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildBaseEquivalenceFailureBoundaryContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func baseEquivalenceFailureBoundaryContractInputs(t *testing.T) (BaseMaterializerBoundaryContract, BaseMaterializerBoundaryContract, EquivalenceResult, BaseEquivalenceFailureBoundaryEvidence) {
	t.Helper()

	left, right, _, _ := baseEquivalenceCheckBoundaryContractInputs(t)
	result := EquivalenceResult{
		Status: EquivalenceNotEquivalent,
		Differences: []Difference{{
			Kind:   ObservationFileManifest,
			Key:    "/workspace/base.txt",
			Left:   "sha256:left-file-manifest-root",
			Right:  "sha256:right-file-manifest-root",
			Reason: "seeded Base failure fixture changes the file manifest root",
		}},
	}
	evidence := BaseEquivalenceFailureBoundaryEvidence{
		LeftMaterializerContractRef:  "base-materializer-contract:left-pass-93",
		RightMaterializerContractRef: "base-materializer-contract:right-pass-93",
		FailureResultRef:             "equivalence-result:base-equivalence-failure-pass-93",
		FailureFixtureRef:            "fixture:base-equivalence-failure-pass-93",
		NoRuntimeMaterialization:     true,
		NoOpaqueDataImageDependency:  true,
		NoMutation:                   true,
	}
	return left, right, result, evidence
}
