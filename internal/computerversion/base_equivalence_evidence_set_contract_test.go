package computerversion

import (
	"strings"
	"testing"
)

func TestBuildBaseEquivalenceEvidenceSetContractBuildsCalibrationSetWithNonEquivalentFailure(t *testing.T) {
	success, failure, evidence := baseEquivalenceEvidenceSetContractInputs(t)

	contract, err := BuildBaseEquivalenceEvidenceSetContract(success, failure, evidence)
	if err != nil {
		t.Fatalf("BuildBaseEquivalenceEvidenceSetContract(): %v", err)
	}

	if contract.Kind != BaseEquivalenceEvidenceSetContractKind {
		t.Fatalf("kind = %q, want %q", contract.Kind, BaseEquivalenceEvidenceSetContractKind)
	}
	if contract.Version != success.Version {
		t.Fatalf("version = %#v, want %#v", contract.Version, success.Version)
	}
	if contract.Boundary != BaseEquivalenceEvidenceSetBoundary {
		t.Fatalf("boundary = %q, want %q", contract.Boundary, BaseEquivalenceEvidenceSetBoundary)
	}
	if contract.Scope != BaseEquivalenceEvidenceSetScope {
		t.Fatalf("scope = %q, want %q", contract.Scope, BaseEquivalenceEvidenceSetScope)
	}
	if contract.SuccessStatus != EquivalenceEquivalent {
		t.Fatalf("success status = %q, want %q", contract.SuccessStatus, EquivalenceEquivalent)
	}
	if contract.FailureStatus != EquivalenceNotEquivalent {
		t.Fatalf("failure status = %q, want %q", contract.FailureStatus, EquivalenceNotEquivalent)
	}
	if contract.SuccessDifferenceCount != 0 || contract.FailureDifferenceCount != failure.DifferenceCount || contract.FailureUnsupportedCount != 0 {
		t.Fatalf("counts = success differences %d failure differences %d unsupported %d, want 0/%d/0", contract.SuccessDifferenceCount, contract.FailureDifferenceCount, contract.FailureUnsupportedCount, failure.DifferenceCount)
	}
	assertObservationBundleKinds(t, contract.RequiredObservations, []ObservationKind{ObservationBlobSet, ObservationFileManifest})
	if contract.SuccessContractRef != evidence.SuccessContractRef || contract.FailureContractRef != evidence.FailureContractRef || contract.CalibrationSuiteRef != evidence.CalibrationSuiteRef {
		t.Fatalf("refs = success %q failure %q calibration %q, want %#v", contract.SuccessContractRef, contract.FailureContractRef, contract.CalibrationSuiteRef, evidence)
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

func TestBuildBaseEquivalenceEvidenceSetContractBuildsCalibrationSetWithNarrowedFailure(t *testing.T) {
	success, _, evidence := baseEquivalenceEvidenceSetContractInputs(t)
	failure := baseEquivalenceEvidenceSetNarrowedFailureContract(t)

	contract, err := BuildBaseEquivalenceEvidenceSetContract(success, failure, evidence)
	if err != nil {
		t.Fatalf("BuildBaseEquivalenceEvidenceSetContract(): %v", err)
	}

	if contract.FailureStatus != EquivalenceNarrowed {
		t.Fatalf("failure status = %q, want %q", contract.FailureStatus, EquivalenceNarrowed)
	}
	if contract.FailureDifferenceCount != 0 || contract.FailureUnsupportedCount != failure.UnsupportedCapabilityCount {
		t.Fatalf("failure counts = differences %d unsupported %d, want 0/%d", contract.FailureDifferenceCount, contract.FailureUnsupportedCount, failure.UnsupportedCapabilityCount)
	}
	if contract.SuccessStatus != EquivalenceEquivalent || contract.SuccessDifferenceCount != 0 {
		t.Fatalf("success side = status %q differences %d, want %q/0", contract.SuccessStatus, contract.SuccessDifferenceCount, EquivalenceEquivalent)
	}
	if !contract.NoRuntimeMaterialization || !contract.NoOpaqueDataImageDependency || !contract.NoMutation {
		t.Fatalf("narrowed calibration set must remain local no-mutation evidence: %#v", contract)
	}
}

func TestBuildBaseEquivalenceEvidenceSetContractRejectsInvalidInputs(t *testing.T) {
	foreignVersion := ComputerVersion{CodeRef: "git:foreign-base-equivalence-evidence-set", ArtifactProgramRef: "base-journal:owner/main@cursor-foreign-base-equivalence-evidence-set"}
	for _, tc := range []struct {
		name    string
		mutate  func(*BaseEquivalenceCheckBoundaryContract, *BaseEquivalenceFailureBoundaryContract, *BaseEquivalenceEvidenceSetEvidence)
		wantErr string
	}{
		{
			name: "wrong success kind",
			mutate: func(success *BaseEquivalenceCheckBoundaryContract, _ *BaseEquivalenceFailureBoundaryContract, _ *BaseEquivalenceEvidenceSetEvidence) {
				success.Kind = BaseEquivalenceFailureBoundaryContractKind
			},
			wantErr: "success contract kind",
		},
		{
			name: "wrong success scope",
			mutate: func(success *BaseEquivalenceCheckBoundaryContract, _ *BaseEquivalenceFailureBoundaryContract, _ *BaseEquivalenceEvidenceSetEvidence) {
				success.Scope = BaseEquivalenceFailureScope
			},
			wantErr: "success contract scope",
		},
		{
			name: "wrong success boundary",
			mutate: func(success *BaseEquivalenceCheckBoundaryContract, _ *BaseEquivalenceFailureBoundaryContract, _ *BaseEquivalenceEvidenceSetEvidence) {
				success.Boundary = BaseEquivalenceFailureBoundary
			},
			wantErr: "success contract boundary",
		},
		{
			name: "success non-equivalent status",
			mutate: func(success *BaseEquivalenceCheckBoundaryContract, _ *BaseEquivalenceFailureBoundaryContract, _ *BaseEquivalenceEvidenceSetEvidence) {
				success.EquivalenceStatus = EquivalenceNotEquivalent
			},
			wantErr: "success contract must be equivalent",
		},
		{
			name: "success missing file manifest observation scope",
			mutate: func(success *BaseEquivalenceCheckBoundaryContract, _ *BaseEquivalenceFailureBoundaryContract, _ *BaseEquivalenceEvidenceSetEvidence) {
				success.RequiredObservations = []ObservationKind{ObservationBlobSet}
			},
			wantErr: "success contract must include file_manifest and blob_set",
		},
		{
			name: "success missing blob set observation scope",
			mutate: func(success *BaseEquivalenceCheckBoundaryContract, _ *BaseEquivalenceFailureBoundaryContract, _ *BaseEquivalenceEvidenceSetEvidence) {
				success.RequiredObservations = []ObservationKind{ObservationFileManifest}
			},
			wantErr: "success contract must include file_manifest and blob_set",
		},
		{
			name: "success unsafe no mutation flag",
			mutate: func(success *BaseEquivalenceCheckBoundaryContract, _ *BaseEquivalenceFailureBoundaryContract, _ *BaseEquivalenceEvidenceSetEvidence) {
				success.NoMutation = false
			},
			wantErr: "success contract has unsafe proof flags",
		},
		{
			name: "success protected-surface claim",
			mutate: func(success *BaseEquivalenceCheckBoundaryContract, _ *BaseEquivalenceFailureBoundaryContract, _ *BaseEquivalenceEvidenceSetEvidence) {
				success.RuntimeBehaviorChanged = true
			},
			wantErr: "success contract carries protected-surface claims",
		},
		{
			name: "wrong failure kind",
			mutate: func(_ *BaseEquivalenceCheckBoundaryContract, failure *BaseEquivalenceFailureBoundaryContract, _ *BaseEquivalenceEvidenceSetEvidence) {
				failure.Kind = BaseEquivalenceCheckBoundaryContractKind
			},
			wantErr: "failure contract kind",
		},
		{
			name: "wrong failure scope",
			mutate: func(_ *BaseEquivalenceCheckBoundaryContract, failure *BaseEquivalenceFailureBoundaryContract, _ *BaseEquivalenceEvidenceSetEvidence) {
				failure.Scope = BaseEquivalenceCheckScope
			},
			wantErr: "failure contract scope",
		},
		{
			name: "wrong failure boundary",
			mutate: func(_ *BaseEquivalenceCheckBoundaryContract, failure *BaseEquivalenceFailureBoundaryContract, _ *BaseEquivalenceEvidenceSetEvidence) {
				failure.Boundary = BaseEquivalenceCheckBoundary
			},
			wantErr: "failure contract boundary",
		},
		{
			name: "failure equivalent status",
			mutate: func(_ *BaseEquivalenceCheckBoundaryContract, failure *BaseEquivalenceFailureBoundaryContract, _ *BaseEquivalenceEvidenceSetEvidence) {
				failure.FailureStatus = EquivalenceEquivalent
			},
			wantErr: "failure contract status",
		},
		{
			name: "non-equivalent failure without differences",
			mutate: func(_ *BaseEquivalenceCheckBoundaryContract, failure *BaseEquivalenceFailureBoundaryContract, _ *BaseEquivalenceEvidenceSetEvidence) {
				failure.DifferenceCount = 0
			},
			wantErr: "non-equivalent failure contract has invalid counts",
		},
		{
			name: "non-equivalent failure with unsupported capabilities",
			mutate: func(_ *BaseEquivalenceCheckBoundaryContract, failure *BaseEquivalenceFailureBoundaryContract, _ *BaseEquivalenceEvidenceSetEvidence) {
				failure.UnsupportedCapabilityCount = 1
			},
			wantErr: "non-equivalent failure contract has invalid counts",
		},
		{
			name: "narrowed failure without unsupported capabilities",
			mutate: func(_ *BaseEquivalenceCheckBoundaryContract, failure *BaseEquivalenceFailureBoundaryContract, _ *BaseEquivalenceEvidenceSetEvidence) {
				failure.FailureStatus = EquivalenceNarrowed
				failure.DifferenceCount = 0
				failure.UnsupportedCapabilityCount = 0
			},
			wantErr: "narrowed failure contract has invalid counts",
		},
		{
			name: "narrowed failure with differences",
			mutate: func(_ *BaseEquivalenceCheckBoundaryContract, failure *BaseEquivalenceFailureBoundaryContract, _ *BaseEquivalenceEvidenceSetEvidence) {
				failure.FailureStatus = EquivalenceNarrowed
				failure.DifferenceCount = 1
				failure.UnsupportedCapabilityCount = 1
			},
			wantErr: "narrowed failure contract has invalid counts",
		},
		{
			name: "failure missing file manifest observation scope",
			mutate: func(_ *BaseEquivalenceCheckBoundaryContract, failure *BaseEquivalenceFailureBoundaryContract, _ *BaseEquivalenceEvidenceSetEvidence) {
				failure.RequiredObservations = []ObservationKind{ObservationBlobSet}
			},
			wantErr: "failure contract must include file_manifest and blob_set",
		},
		{
			name: "failure missing blob set observation scope",
			mutate: func(_ *BaseEquivalenceCheckBoundaryContract, failure *BaseEquivalenceFailureBoundaryContract, _ *BaseEquivalenceEvidenceSetEvidence) {
				failure.RequiredObservations = []ObservationKind{ObservationFileManifest}
			},
			wantErr: "failure contract must include file_manifest and blob_set",
		},
		{
			name: "failure successful-equivalence claim",
			mutate: func(_ *BaseEquivalenceCheckBoundaryContract, failure *BaseEquivalenceFailureBoundaryContract, _ *BaseEquivalenceEvidenceSetEvidence) {
				failure.SuccessfulEquivalenceClaimed = true
			},
			wantErr: "failure contract claims successful equivalence",
		},
		{
			name: "failure unsafe runtime materialization flag",
			mutate: func(_ *BaseEquivalenceCheckBoundaryContract, failure *BaseEquivalenceFailureBoundaryContract, _ *BaseEquivalenceEvidenceSetEvidence) {
				failure.NoRuntimeMaterialization = false
			},
			wantErr: "failure contract has unsafe proof flags",
		},
		{
			name: "failure protected-surface claim",
			mutate: func(_ *BaseEquivalenceCheckBoundaryContract, failure *BaseEquivalenceFailureBoundaryContract, _ *BaseEquivalenceEvidenceSetEvidence) {
				failure.VMLifecycleTouched = true
			},
			wantErr: "failure contract carries protected-surface claims",
		},
		{
			name: "version drift",
			mutate: func(_ *BaseEquivalenceCheckBoundaryContract, failure *BaseEquivalenceFailureBoundaryContract, _ *BaseEquivalenceEvidenceSetEvidence) {
				failure.Version = foreignVersion
			},
			wantErr: "success and failure contracts name different computer versions",
		},
		{
			name: "missing success contract ref",
			mutate: func(_ *BaseEquivalenceCheckBoundaryContract, _ *BaseEquivalenceFailureBoundaryContract, evidence *BaseEquivalenceEvidenceSetEvidence) {
				evidence.SuccessContractRef = "  "
			},
			wantErr: "success contract ref is required",
		},
		{
			name: "missing failure contract ref",
			mutate: func(_ *BaseEquivalenceCheckBoundaryContract, _ *BaseEquivalenceFailureBoundaryContract, evidence *BaseEquivalenceEvidenceSetEvidence) {
				evidence.FailureContractRef = ""
			},
			wantErr: "failure contract ref is required",
		},
		{
			name: "missing calibration suite ref",
			mutate: func(_ *BaseEquivalenceCheckBoundaryContract, _ *BaseEquivalenceFailureBoundaryContract, evidence *BaseEquivalenceEvidenceSetEvidence) {
				evidence.CalibrationSuiteRef = "\t"
			},
			wantErr: "calibration suite ref is required",
		},
		{
			name: "runtime materialization not excluded",
			mutate: func(_ *BaseEquivalenceCheckBoundaryContract, _ *BaseEquivalenceFailureBoundaryContract, evidence *BaseEquivalenceEvidenceSetEvidence) {
				evidence.NoRuntimeMaterialization = false
			},
			wantErr: "evidence must prove no runtime materialization",
		},
		{
			name: "opaque data image dependency not excluded",
			mutate: func(_ *BaseEquivalenceCheckBoundaryContract, _ *BaseEquivalenceFailureBoundaryContract, evidence *BaseEquivalenceEvidenceSetEvidence) {
				evidence.NoOpaqueDataImageDependency = false
			},
			wantErr: "evidence must prove no opaque data.img dependency",
		},
		{
			name: "deployed route registered",
			mutate: func(_ *BaseEquivalenceCheckBoundaryContract, _ *BaseEquivalenceFailureBoundaryContract, evidence *BaseEquivalenceEvidenceSetEvidence) {
				evidence.DeployedRouteRegistered = true
			},
			wantErr: "evidence cannot register deployed routes",
		},
		{
			name: "production auth touched",
			mutate: func(_ *BaseEquivalenceCheckBoundaryContract, _ *BaseEquivalenceFailureBoundaryContract, evidence *BaseEquivalenceEvidenceSetEvidence) {
				evidence.ProductionAuthTouched = true
			},
			wantErr: "evidence cannot touch production auth/session",
		},
		{
			name: "staging claimed",
			mutate: func(_ *BaseEquivalenceCheckBoundaryContract, _ *BaseEquivalenceFailureBoundaryContract, evidence *BaseEquivalenceEvidenceSetEvidence) {
				evidence.StagingClaimed = true
			},
			wantErr: "evidence cannot claim staging",
		},
		{
			name: "promotion claimed",
			mutate: func(_ *BaseEquivalenceCheckBoundaryContract, _ *BaseEquivalenceFailureBoundaryContract, evidence *BaseEquivalenceEvidenceSetEvidence) {
				evidence.PromotionClaimed = true
			},
			wantErr: "evidence cannot claim promotion",
		},
		{
			name: "vm lifecycle touched",
			mutate: func(_ *BaseEquivalenceCheckBoundaryContract, _ *BaseEquivalenceFailureBoundaryContract, evidence *BaseEquivalenceEvidenceSetEvidence) {
				evidence.VMLifecycleTouched = true
			},
			wantErr: "evidence cannot touch VM lifecycle",
		},
		{
			name: "Firecracker boot claimed",
			mutate: func(_ *BaseEquivalenceCheckBoundaryContract, _ *BaseEquivalenceFailureBoundaryContract, evidence *BaseEquivalenceEvidenceSetEvidence) {
				evidence.FirecrackerBootClaimed = true
			},
			wantErr: "evidence cannot claim Firecracker boot",
		},
		{
			name: "run acceptance record touched",
			mutate: func(_ *BaseEquivalenceCheckBoundaryContract, _ *BaseEquivalenceFailureBoundaryContract, evidence *BaseEquivalenceEvidenceSetEvidence) {
				evidence.RunAcceptanceRecordTouched = true
			},
			wantErr: "evidence cannot touch run acceptance records",
		},
		{
			name: "full substrate independence claimed",
			mutate: func(_ *BaseEquivalenceCheckBoundaryContract, _ *BaseEquivalenceFailureBoundaryContract, evidence *BaseEquivalenceEvidenceSetEvidence) {
				evidence.FullSubstrateIndependenceClaim = true
			},
			wantErr: "evidence cannot claim full substrate independence",
		},
		{
			name: "evidence no mutation false",
			mutate: func(_ *BaseEquivalenceCheckBoundaryContract, _ *BaseEquivalenceFailureBoundaryContract, evidence *BaseEquivalenceEvidenceSetEvidence) {
				evidence.NoMutation = false
			},
			wantErr: "evidence must be no-mutation",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			success, failure, evidence := baseEquivalenceEvidenceSetContractInputs(t)
			tc.mutate(&success, &failure, &evidence)

			contract, err := BuildBaseEquivalenceEvidenceSetContract(success, failure, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildBaseEquivalenceEvidenceSetContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func baseEquivalenceEvidenceSetContractInputs(t *testing.T) (BaseEquivalenceCheckBoundaryContract, BaseEquivalenceFailureBoundaryContract, BaseEquivalenceEvidenceSetEvidence) {
	t.Helper()

	left, right, result, checkEvidence := baseEquivalenceCheckBoundaryContractInputs(t)
	success, err := BuildBaseEquivalenceCheckBoundaryContract(left, right, result, checkEvidence)
	if err != nil {
		t.Fatalf("BuildBaseEquivalenceCheckBoundaryContract(): %v", err)
	}

	failureLeft, failureRight, failureResult, failureEvidence := baseEquivalenceFailureBoundaryContractInputs(t)
	failure, err := BuildBaseEquivalenceFailureBoundaryContract(failureLeft, failureRight, failureResult, failureEvidence)
	if err != nil {
		t.Fatalf("BuildBaseEquivalenceFailureBoundaryContract(): %v", err)
	}

	evidence := BaseEquivalenceEvidenceSetEvidence{
		SuccessContractRef:             "base-equivalence-check-contract:pass-94",
		FailureContractRef:             "base-equivalence-failure-contract:pass-94",
		CalibrationSuiteRef:            "calibration-suite:base-equivalence-evidence-set-pass-94",
		NoRuntimeMaterialization:       true,
		NoOpaqueDataImageDependency:    true,
		NoMutation:                     true,
	}
	return success, failure, evidence
}

func baseEquivalenceEvidenceSetNarrowedFailureContract(t *testing.T) BaseEquivalenceFailureBoundaryContract {
	t.Helper()

	left, right, _, evidence := baseEquivalenceFailureBoundaryContractInputs(t)
	result := EquivalenceResult{
		Status: EquivalenceNarrowed,
		Unsupported: []UnsupportedCapability{
			{Kind: ObservationFileManifest, Reason: "right materializer cannot expose file manifests for the seeded calibration fixture"},
			{Kind: ObservationBlobSet, Reason: "right materializer cannot expose blob integrity for the seeded calibration fixture"},
		},
	}
	failure, err := BuildBaseEquivalenceFailureBoundaryContract(left, right, result, evidence)
	if err != nil {
		t.Fatalf("BuildBaseEquivalenceFailureBoundaryContract(): %v", err)
	}
	return failure
}
