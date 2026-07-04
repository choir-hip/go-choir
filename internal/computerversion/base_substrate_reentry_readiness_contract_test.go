package computerversion

import (
	"strings"
	"testing"
)

func TestBuildBaseSubstrateReentryReadinessContractBuildsLocalReentryWithNonEquivalentCalibration(t *testing.T) {
	substrate, calibration, evidence := baseSubstrateReentryReadinessContractInputs(t)

	contract, err := BuildBaseSubstrateReentryReadinessContract(substrate, calibration, evidence)
	if err != nil {
		t.Fatalf("BuildBaseSubstrateReentryReadinessContract(): %v", err)
	}

	if contract.Kind != BaseSubstrateReentryReadinessContractKind {
		t.Fatalf("kind = %q, want %q", contract.Kind, BaseSubstrateReentryReadinessContractKind)
	}
	if contract.Version != substrate.Version {
		t.Fatalf("version = %#v, want %#v", contract.Version, substrate.Version)
	}
	if contract.Boundary != BaseSubstrateReentryReadinessBoundary {
		t.Fatalf("boundary = %q, want %q", contract.Boundary, BaseSubstrateReentryReadinessBoundary)
	}
	if contract.Scope != BaseSubstrateReentryReadinessScope {
		t.Fatalf("scope = %q, want %q", contract.Scope, BaseSubstrateReentryReadinessScope)
	}
	if contract.ClaimScope != BaseSubstrateEquivalenceClaimScope {
		t.Fatalf("claim scope = %q, want %q", contract.ClaimScope, BaseSubstrateEquivalenceClaimScope)
	}
	if contract.CurrentMaterializer != substrate.CurrentMaterializer || contract.CurrentSubstrate != substrate.CurrentSubstrate || contract.ProjectionMaterializer != substrate.ProjectionMaterializer || contract.ProjectionSubstrate != substrate.ProjectionSubstrate {
		t.Fatalf("substrate identities = current %q/%q projection %q/%q, want %#v", contract.CurrentMaterializer, contract.CurrentSubstrate, contract.ProjectionMaterializer, contract.ProjectionSubstrate, substrate)
	}
	if contract.SubstrateEquivalenceStatus != EquivalenceEquivalent {
		t.Fatalf("substrate equivalence status = %q, want %q", contract.SubstrateEquivalenceStatus, EquivalenceEquivalent)
	}
	if contract.CalibrationSuccessStatus != EquivalenceEquivalent || contract.CalibrationFailureStatus != EquivalenceNotEquivalent {
		t.Fatalf("calibration statuses = success %q failure %q, want %q/%q", contract.CalibrationSuccessStatus, contract.CalibrationFailureStatus, EquivalenceEquivalent, EquivalenceNotEquivalent)
	}
	assertObservationBundleKinds(t, contract.RequiredObservations, []ObservationKind{ObservationBlobSet, ObservationFileManifest})
	if contract.SubstrateEquivalenceContractRef != strings.TrimSpace(evidence.SubstrateEquivalenceContractRef) || contract.EquivalenceEvidenceSetRef != strings.TrimSpace(evidence.EquivalenceEvidenceSetRef) || contract.NextProbeRef != strings.TrimSpace(evidence.NextProbeRef) {
		t.Fatalf("refs = substrate %q calibration %q next %q, want trimmed %#v", contract.SubstrateEquivalenceContractRef, contract.EquivalenceEvidenceSetRef, contract.NextProbeRef, evidence)
	}
	if !contract.LocalSubstrateReentryAllowed {
		t.Fatalf("local substrate reentry allowed = false, want true")
	}
	if !contract.NoRuntimeMaterialization || !contract.NoOpaqueDataImageDependency || !contract.NoMutation {
		t.Fatalf("readiness contract must remain local no-runtime/no-opaque/no-mutation evidence: %#v", contract)
	}
	if contract.RuntimeBehaviorChanged || contract.DeployedRouteRegistered || contract.ProductionAuthTouched || contract.StagingClaimed || contract.PromotionClaimed || contract.VMLifecycleTouched || contract.FirecrackerBootClaimed || contract.RunAcceptanceRecordTouched || contract.FullSubstrateIndependenceClaim || contract.CompletionClaimed {
		t.Fatalf("runtime/deployed/auth/staging/promotion/VM/Firecracker/run-acceptance/full-substrate/completion claims must remain false in built contract: %#v", contract)
	}
}

func TestBuildBaseSubstrateReentryReadinessContractBuildsLocalReentryWithNarrowedCalibration(t *testing.T) {
	substrate, calibration, evidence := baseSubstrateReentryReadinessContractInputs(t)
	calibration = baseSubstrateReentryReadinessCalibrationContract(t, substrate.Version, true)

	contract, err := BuildBaseSubstrateReentryReadinessContract(substrate, calibration, evidence)
	if err != nil {
		t.Fatalf("BuildBaseSubstrateReentryReadinessContract(): %v", err)
	}

	if contract.CalibrationSuccessStatus != EquivalenceEquivalent || contract.CalibrationFailureStatus != EquivalenceNarrowed {
		t.Fatalf("calibration statuses = success %q failure %q, want %q/%q", contract.CalibrationSuccessStatus, contract.CalibrationFailureStatus, EquivalenceEquivalent, EquivalenceNarrowed)
	}
	if !contract.LocalSubstrateReentryAllowed {
		t.Fatalf("local substrate reentry allowed = false, want true")
	}
	if contract.Version != substrate.Version || contract.Version != calibration.Version {
		t.Fatalf("version = %#v, want shared substrate/calibration version %#v/%#v", contract.Version, substrate.Version, calibration.Version)
	}
	if !contract.NoRuntimeMaterialization || !contract.NoOpaqueDataImageDependency || !contract.NoMutation {
		t.Fatalf("narrowed calibration readiness must remain local no-runtime/no-opaque/no-mutation evidence: %#v", contract)
	}
	if contract.CompletionClaimed || contract.FullSubstrateIndependenceClaim || contract.FirecrackerBootClaimed {
		t.Fatalf("narrowed calibration readiness must not escalate to completion/full-substrate/runtime claims: %#v", contract)
	}
}

func TestBuildBaseSubstrateReentryReadinessContractRejectsInvalidInputs(t *testing.T) {
	foreignVersion := ComputerVersion{CodeRef: "git:foreign-base-substrate-reentry-readiness", ArtifactProgramRef: "base-journal:owner/main@cursor-foreign-base-substrate-reentry-readiness"}
	for _, tc := range []struct {
		name    string
		mutate  func(*BaseSubstrateEquivalenceContract, *BaseEquivalenceEvidenceSetContract, *BaseSubstrateReentryReadinessEvidence)
		wantErr string
	}{
		{
			name: "wrong substrate kind",
			mutate: func(substrate *BaseSubstrateEquivalenceContract, _ *BaseEquivalenceEvidenceSetContract, _ *BaseSubstrateReentryReadinessEvidence) {
				substrate.Kind = BaseEquivalenceEvidenceSetContractKind
			},
			wantErr: "substrate contract kind",
		},
		{
			name: "wrong substrate boundary",
			mutate: func(substrate *BaseSubstrateEquivalenceContract, _ *BaseEquivalenceEvidenceSetContract, _ *BaseSubstrateReentryReadinessEvidence) {
				substrate.Boundary = BaseSubstrateReentryReadinessBoundary
			},
			wantErr: "substrate contract boundary",
		},
		{
			name: "wrong substrate claim scope",
			mutate: func(substrate *BaseSubstrateEquivalenceContract, _ *BaseEquivalenceEvidenceSetContract, _ *BaseSubstrateReentryReadinessEvidence) {
				substrate.ClaimScope = BaseSubstrateReentryReadinessScope
			},
			wantErr: "substrate claim scope",
		},
		{
			name: "substrate non-equivalent status",
			mutate: func(substrate *BaseSubstrateEquivalenceContract, _ *BaseEquivalenceEvidenceSetContract, _ *BaseSubstrateReentryReadinessEvidence) {
				substrate.EquivalenceStatus = EquivalenceNotEquivalent
			},
			wantErr: "substrate equivalence status",
		},
		{
			name: "substrate missing file manifest observation scope",
			mutate: func(substrate *BaseSubstrateEquivalenceContract, _ *BaseEquivalenceEvidenceSetContract, _ *BaseSubstrateReentryReadinessEvidence) {
				substrate.RequiredObservations = []ObservationKind{ObservationBlobSet}
			},
			wantErr: "substrate contract must include file_manifest and blob_set",
		},
		{
			name: "substrate missing blob set observation scope",
			mutate: func(substrate *BaseSubstrateEquivalenceContract, _ *BaseEquivalenceEvidenceSetContract, _ *BaseSubstrateReentryReadinessEvidence) {
				substrate.RequiredObservations = []ObservationKind{ObservationFileManifest}
			},
			wantErr: "substrate contract must include file_manifest and blob_set",
		},
		{
			name: "substrate missing current materializer",
			mutate: func(substrate *BaseSubstrateEquivalenceContract, _ *BaseEquivalenceEvidenceSetContract, _ *BaseSubstrateReentryReadinessEvidence) {
				substrate.CurrentMaterializer = "  "
			},
			wantErr: "substrate contract must name current and projection materializers",
		},
		{
			name: "substrate missing projection substrate",
			mutate: func(substrate *BaseSubstrateEquivalenceContract, _ *BaseEquivalenceEvidenceSetContract, _ *BaseSubstrateReentryReadinessEvidence) {
				substrate.ProjectionSubstrate = ""
			},
			wantErr: "substrate contract must name current and projection materializers",
		},
		{
			name: "substrate identical current and projection identities",
			mutate: func(substrate *BaseSubstrateEquivalenceContract, _ *BaseEquivalenceEvidenceSetContract, _ *BaseSubstrateReentryReadinessEvidence) {
				substrate.ProjectionMaterializer = substrate.CurrentMaterializer
				substrate.ProjectionSubstrate = substrate.CurrentSubstrate
			},
			wantErr: "substrate contract must compare non-identical materializer or substrate",
		},
		{
			name: "substrate runtime materialization not excluded",
			mutate: func(substrate *BaseSubstrateEquivalenceContract, _ *BaseEquivalenceEvidenceSetContract, _ *BaseSubstrateReentryReadinessEvidence) {
				substrate.NoRuntimeMaterialization = false
			},
			wantErr: "substrate contract has unsafe proof flags",
		},
		{
			name: "substrate opaque data image dependency not excluded",
			mutate: func(substrate *BaseSubstrateEquivalenceContract, _ *BaseEquivalenceEvidenceSetContract, _ *BaseSubstrateReentryReadinessEvidence) {
				substrate.NoOpaqueDataImageDependency = false
			},
			wantErr: "substrate contract has unsafe proof flags",
		},
		{
			name: "substrate runtime behavior changed",
			mutate: func(substrate *BaseSubstrateEquivalenceContract, _ *BaseEquivalenceEvidenceSetContract, _ *BaseSubstrateReentryReadinessEvidence) {
				substrate.RuntimeBehaviorChanged = true
			},
			wantErr: "substrate contract carries protected-surface claims",
		},
		{
			name: "substrate deployed route registered",
			mutate: func(substrate *BaseSubstrateEquivalenceContract, _ *BaseEquivalenceEvidenceSetContract, _ *BaseSubstrateReentryReadinessEvidence) {
				substrate.DeployedRouteRegistered = true
			},
			wantErr: "substrate contract carries protected-surface claims",
		},
		{
			name: "substrate production auth touched",
			mutate: func(substrate *BaseSubstrateEquivalenceContract, _ *BaseEquivalenceEvidenceSetContract, _ *BaseSubstrateReentryReadinessEvidence) {
				substrate.ProductionAuthTouched = true
			},
			wantErr: "substrate contract carries protected-surface claims",
		},
		{
			name: "substrate staging claimed",
			mutate: func(substrate *BaseSubstrateEquivalenceContract, _ *BaseEquivalenceEvidenceSetContract, _ *BaseSubstrateReentryReadinessEvidence) {
				substrate.StagingClaimed = true
			},
			wantErr: "substrate contract carries protected-surface claims",
		},
		{
			name: "substrate promotion claimed",
			mutate: func(substrate *BaseSubstrateEquivalenceContract, _ *BaseEquivalenceEvidenceSetContract, _ *BaseSubstrateReentryReadinessEvidence) {
				substrate.PromotionClaimed = true
			},
			wantErr: "substrate contract carries protected-surface claims",
		},
		{
			name: "substrate VM lifecycle touched",
			mutate: func(substrate *BaseSubstrateEquivalenceContract, _ *BaseEquivalenceEvidenceSetContract, _ *BaseSubstrateReentryReadinessEvidence) {
				substrate.VMLifecycleTouched = true
			},
			wantErr: "substrate contract carries protected-surface claims",
		},
		{
			name: "substrate run acceptance record touched",
			mutate: func(substrate *BaseSubstrateEquivalenceContract, _ *BaseEquivalenceEvidenceSetContract, _ *BaseSubstrateReentryReadinessEvidence) {
				substrate.RunAcceptanceRecordTouched = true
			},
			wantErr: "substrate contract carries protected-surface claims",
		},
		{
			name: "substrate Firecracker boot claimed",
			mutate: func(substrate *BaseSubstrateEquivalenceContract, _ *BaseEquivalenceEvidenceSetContract, _ *BaseSubstrateReentryReadinessEvidence) {
				substrate.FirecrackerBootClaimed = true
			},
			wantErr: "substrate contract carries protected-surface claims",
		},
		{
			name: "substrate full substrate independence claimed",
			mutate: func(substrate *BaseSubstrateEquivalenceContract, _ *BaseEquivalenceEvidenceSetContract, _ *BaseSubstrateReentryReadinessEvidence) {
				substrate.FullSubstrateIndependenceClaim = true
			},
			wantErr: "substrate contract carries protected-surface claims",
		},
		{
			name: "substrate completion claimed",
			mutate: func(substrate *BaseSubstrateEquivalenceContract, _ *BaseEquivalenceEvidenceSetContract, _ *BaseSubstrateReentryReadinessEvidence) {
				substrate.CompletionClaimed = true
			},
			wantErr: "substrate contract carries protected-surface claims",
		},
		{
			name: "substrate no mutation false",
			mutate: func(substrate *BaseSubstrateEquivalenceContract, _ *BaseEquivalenceEvidenceSetContract, _ *BaseSubstrateReentryReadinessEvidence) {
				substrate.NoMutation = false
			},
			wantErr: "substrate contract has unsafe proof flags",
		},
		{
			name: "wrong calibration kind",
			mutate: func(_ *BaseSubstrateEquivalenceContract, calibration *BaseEquivalenceEvidenceSetContract, _ *BaseSubstrateReentryReadinessEvidence) {
				calibration.Kind = BaseSubstrateEquivalenceContractKind
			},
			wantErr: "calibration contract kind",
		},
		{
			name: "wrong calibration boundary",
			mutate: func(_ *BaseSubstrateEquivalenceContract, calibration *BaseEquivalenceEvidenceSetContract, _ *BaseSubstrateReentryReadinessEvidence) {
				calibration.Boundary = BaseSubstrateEquivalenceBoundary
			},
			wantErr: "calibration contract boundary",
		},
		{
			name: "wrong calibration scope",
			mutate: func(_ *BaseSubstrateEquivalenceContract, calibration *BaseEquivalenceEvidenceSetContract, _ *BaseSubstrateReentryReadinessEvidence) {
				calibration.Scope = BaseSubstrateReentryReadinessScope
			},
			wantErr: "calibration contract scope",
		},
		{
			name: "calibration success non-equivalent status",
			mutate: func(_ *BaseSubstrateEquivalenceContract, calibration *BaseEquivalenceEvidenceSetContract, _ *BaseSubstrateReentryReadinessEvidence) {
				calibration.SuccessStatus = EquivalenceNotEquivalent
			},
			wantErr: "calibration success status",
		},
		{
			name: "calibration failure success status",
			mutate: func(_ *BaseSubstrateEquivalenceContract, calibration *BaseEquivalenceEvidenceSetContract, _ *BaseSubstrateReentryReadinessEvidence) {
				calibration.FailureStatus = EquivalenceEquivalent
			},
			wantErr: "calibration failure status",
		},
		{
			name: "calibration non-equivalent without differences",
			mutate: func(_ *BaseSubstrateEquivalenceContract, calibration *BaseEquivalenceEvidenceSetContract, _ *BaseSubstrateReentryReadinessEvidence) {
				calibration.FailureDifferenceCount = 0
			},
			wantErr: "calibration non-equivalent counts are invalid",
		},
		{
			name: "calibration non-equivalent with unsupported capabilities",
			mutate: func(_ *BaseSubstrateEquivalenceContract, calibration *BaseEquivalenceEvidenceSetContract, _ *BaseSubstrateReentryReadinessEvidence) {
				calibration.FailureUnsupportedCount = 1
			},
			wantErr: "calibration non-equivalent counts are invalid",
		},
		{
			name: "calibration narrowed without unsupported capabilities",
			mutate: func(_ *BaseSubstrateEquivalenceContract, calibration *BaseEquivalenceEvidenceSetContract, _ *BaseSubstrateReentryReadinessEvidence) {
				calibration.FailureStatus = EquivalenceNarrowed
				calibration.FailureDifferenceCount = 0
				calibration.FailureUnsupportedCount = 0
			},
			wantErr: "calibration narrowed counts are invalid",
		},
		{
			name: "calibration narrowed with differences",
			mutate: func(_ *BaseSubstrateEquivalenceContract, calibration *BaseEquivalenceEvidenceSetContract, _ *BaseSubstrateReentryReadinessEvidence) {
				calibration.FailureStatus = EquivalenceNarrowed
				calibration.FailureDifferenceCount = 1
				calibration.FailureUnsupportedCount = 1
			},
			wantErr: "calibration narrowed counts are invalid",
		},
		{
			name: "calibration missing file manifest observation scope",
			mutate: func(_ *BaseSubstrateEquivalenceContract, calibration *BaseEquivalenceEvidenceSetContract, _ *BaseSubstrateReentryReadinessEvidence) {
				calibration.RequiredObservations = []ObservationKind{ObservationBlobSet}
			},
			wantErr: "calibration must include file_manifest and blob_set",
		},
		{
			name: "calibration missing blob set observation scope",
			mutate: func(_ *BaseSubstrateEquivalenceContract, calibration *BaseEquivalenceEvidenceSetContract, _ *BaseSubstrateReentryReadinessEvidence) {
				calibration.RequiredObservations = []ObservationKind{ObservationFileManifest}
			},
			wantErr: "calibration must include file_manifest and blob_set",
		},
		{
			name: "calibration runtime materialization not excluded",
			mutate: func(_ *BaseSubstrateEquivalenceContract, calibration *BaseEquivalenceEvidenceSetContract, _ *BaseSubstrateReentryReadinessEvidence) {
				calibration.NoRuntimeMaterialization = false
			},
			wantErr: "calibration has unsafe proof flags",
		},
		{
			name: "calibration opaque data image dependency not excluded",
			mutate: func(_ *BaseSubstrateEquivalenceContract, calibration *BaseEquivalenceEvidenceSetContract, _ *BaseSubstrateReentryReadinessEvidence) {
				calibration.NoOpaqueDataImageDependency = false
			},
			wantErr: "calibration has unsafe proof flags",
		},
		{
			name: "calibration no mutation false",
			mutate: func(_ *BaseSubstrateEquivalenceContract, calibration *BaseEquivalenceEvidenceSetContract, _ *BaseSubstrateReentryReadinessEvidence) {
				calibration.NoMutation = false
			},
			wantErr: "calibration has unsafe proof flags",
		},
		{
			name: "calibration runtime behavior changed",
			mutate: func(_ *BaseSubstrateEquivalenceContract, calibration *BaseEquivalenceEvidenceSetContract, _ *BaseSubstrateReentryReadinessEvidence) {
				calibration.RuntimeBehaviorChanged = true
			},
			wantErr: "calibration carries protected-surface claims",
		},
		{
			name: "calibration deployed route registered",
			mutate: func(_ *BaseSubstrateEquivalenceContract, calibration *BaseEquivalenceEvidenceSetContract, _ *BaseSubstrateReentryReadinessEvidence) {
				calibration.DeployedRouteRegistered = true
			},
			wantErr: "calibration carries protected-surface claims",
		},
		{
			name: "calibration production auth touched",
			mutate: func(_ *BaseSubstrateEquivalenceContract, calibration *BaseEquivalenceEvidenceSetContract, _ *BaseSubstrateReentryReadinessEvidence) {
				calibration.ProductionAuthTouched = true
			},
			wantErr: "calibration carries protected-surface claims",
		},
		{
			name: "calibration staging claimed",
			mutate: func(_ *BaseSubstrateEquivalenceContract, calibration *BaseEquivalenceEvidenceSetContract, _ *BaseSubstrateReentryReadinessEvidence) {
				calibration.StagingClaimed = true
			},
			wantErr: "calibration carries protected-surface claims",
		},
		{
			name: "calibration promotion claimed",
			mutate: func(_ *BaseSubstrateEquivalenceContract, calibration *BaseEquivalenceEvidenceSetContract, _ *BaseSubstrateReentryReadinessEvidence) {
				calibration.PromotionClaimed = true
			},
			wantErr: "calibration carries protected-surface claims",
		},
		{
			name: "calibration VM lifecycle touched",
			mutate: func(_ *BaseSubstrateEquivalenceContract, calibration *BaseEquivalenceEvidenceSetContract, _ *BaseSubstrateReentryReadinessEvidence) {
				calibration.VMLifecycleTouched = true
			},
			wantErr: "calibration carries protected-surface claims",
		},
		{
			name: "calibration Firecracker boot claimed",
			mutate: func(_ *BaseSubstrateEquivalenceContract, calibration *BaseEquivalenceEvidenceSetContract, _ *BaseSubstrateReentryReadinessEvidence) {
				calibration.FirecrackerBootClaimed = true
			},
			wantErr: "calibration carries protected-surface claims",
		},
		{
			name: "calibration run acceptance record touched",
			mutate: func(_ *BaseSubstrateEquivalenceContract, calibration *BaseEquivalenceEvidenceSetContract, _ *BaseSubstrateReentryReadinessEvidence) {
				calibration.RunAcceptanceRecordTouched = true
			},
			wantErr: "calibration carries protected-surface claims",
		},
		{
			name: "calibration full substrate independence claimed",
			mutate: func(_ *BaseSubstrateEquivalenceContract, calibration *BaseEquivalenceEvidenceSetContract, _ *BaseSubstrateReentryReadinessEvidence) {
				calibration.FullSubstrateIndependenceClaim = true
			},
			wantErr: "calibration carries protected-surface claims",
		},
		{
			name: "version drift",
			mutate: func(_ *BaseSubstrateEquivalenceContract, calibration *BaseEquivalenceEvidenceSetContract, _ *BaseSubstrateReentryReadinessEvidence) {
				calibration.Version = foreignVersion
			},
			wantErr: "substrate and calibration contracts name different computer versions",
		},
		{
			name: "missing substrate equivalence contract ref",
			mutate: func(_ *BaseSubstrateEquivalenceContract, _ *BaseEquivalenceEvidenceSetContract, evidence *BaseSubstrateReentryReadinessEvidence) {
				evidence.SubstrateEquivalenceContractRef = "  "
			},
			wantErr: "substrate equivalence contract ref is required",
		},
		{
			name: "missing equivalence evidence set ref",
			mutate: func(_ *BaseSubstrateEquivalenceContract, _ *BaseEquivalenceEvidenceSetContract, evidence *BaseSubstrateReentryReadinessEvidence) {
				evidence.EquivalenceEvidenceSetRef = ""
			},
			wantErr: "equivalence evidence set ref is required",
		},
		{
			name: "missing next probe ref",
			mutate: func(_ *BaseSubstrateEquivalenceContract, _ *BaseEquivalenceEvidenceSetContract, evidence *BaseSubstrateReentryReadinessEvidence) {
				evidence.NextProbeRef = "\t"
			},
			wantErr: "next probe ref is required",
		},
		{
			name: "runtime materialization not excluded",
			mutate: func(_ *BaseSubstrateEquivalenceContract, _ *BaseEquivalenceEvidenceSetContract, evidence *BaseSubstrateReentryReadinessEvidence) {
				evidence.NoRuntimeMaterialization = false
			},
			wantErr: "evidence must prove no runtime materialization",
		},
		{
			name: "opaque data image dependency not excluded",
			mutate: func(_ *BaseSubstrateEquivalenceContract, _ *BaseEquivalenceEvidenceSetContract, evidence *BaseSubstrateReentryReadinessEvidence) {
				evidence.NoOpaqueDataImageDependency = false
			},
			wantErr: "evidence must prove no opaque data.img dependency",
		},
		{
			name: "evidence runtime behavior changed",
			mutate: func(_ *BaseSubstrateEquivalenceContract, _ *BaseEquivalenceEvidenceSetContract, evidence *BaseSubstrateReentryReadinessEvidence) {
				evidence.RuntimeBehaviorChanged = true
			},
			wantErr: "evidence cannot change runtime behavior",
		},
		{
			name: "evidence deployed route registered",
			mutate: func(_ *BaseSubstrateEquivalenceContract, _ *BaseEquivalenceEvidenceSetContract, evidence *BaseSubstrateReentryReadinessEvidence) {
				evidence.DeployedRouteRegistered = true
			},
			wantErr: "evidence cannot register deployed routes",
		},
		{
			name: "evidence production auth touched",
			mutate: func(_ *BaseSubstrateEquivalenceContract, _ *BaseEquivalenceEvidenceSetContract, evidence *BaseSubstrateReentryReadinessEvidence) {
				evidence.ProductionAuthTouched = true
			},
			wantErr: "evidence cannot touch production auth/session",
		},
		{
			name: "evidence staging claimed",
			mutate: func(_ *BaseSubstrateEquivalenceContract, _ *BaseEquivalenceEvidenceSetContract, evidence *BaseSubstrateReentryReadinessEvidence) {
				evidence.StagingClaimed = true
			},
			wantErr: "evidence cannot claim staging",
		},
		{
			name: "evidence promotion claimed",
			mutate: func(_ *BaseSubstrateEquivalenceContract, _ *BaseEquivalenceEvidenceSetContract, evidence *BaseSubstrateReentryReadinessEvidence) {
				evidence.PromotionClaimed = true
			},
			wantErr: "evidence cannot claim promotion",
		},
		{
			name: "evidence VM lifecycle touched",
			mutate: func(_ *BaseSubstrateEquivalenceContract, _ *BaseEquivalenceEvidenceSetContract, evidence *BaseSubstrateReentryReadinessEvidence) {
				evidence.VMLifecycleTouched = true
			},
			wantErr: "evidence cannot touch VM lifecycle",
		},
		{
			name: "evidence Firecracker boot claimed",
			mutate: func(_ *BaseSubstrateEquivalenceContract, _ *BaseEquivalenceEvidenceSetContract, evidence *BaseSubstrateReentryReadinessEvidence) {
				evidence.FirecrackerBootClaimed = true
			},
			wantErr: "evidence cannot claim Firecracker boot",
		},
		{
			name: "evidence run acceptance record touched",
			mutate: func(_ *BaseSubstrateEquivalenceContract, _ *BaseEquivalenceEvidenceSetContract, evidence *BaseSubstrateReentryReadinessEvidence) {
				evidence.RunAcceptanceRecordTouched = true
			},
			wantErr: "evidence cannot touch run acceptance records",
		},
		{
			name: "evidence full substrate independence claimed",
			mutate: func(_ *BaseSubstrateEquivalenceContract, _ *BaseEquivalenceEvidenceSetContract, evidence *BaseSubstrateReentryReadinessEvidence) {
				evidence.FullSubstrateIndependenceClaim = true
			},
			wantErr: "evidence cannot claim full substrate independence",
		},
		{
			name: "evidence completion claimed",
			mutate: func(_ *BaseSubstrateEquivalenceContract, _ *BaseEquivalenceEvidenceSetContract, evidence *BaseSubstrateReentryReadinessEvidence) {
				evidence.CompletionClaimed = true
			},
			wantErr: "evidence cannot claim completion",
		},
		{
			name: "evidence no mutation false",
			mutate: func(_ *BaseSubstrateEquivalenceContract, _ *BaseEquivalenceEvidenceSetContract, evidence *BaseSubstrateReentryReadinessEvidence) {
				evidence.NoMutation = false
			},
			wantErr: "evidence must be no-mutation",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			substrate, calibration, evidence := baseSubstrateReentryReadinessContractInputs(t)
			tc.mutate(&substrate, &calibration, &evidence)

			contract, err := BuildBaseSubstrateReentryReadinessContract(substrate, calibration, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildBaseSubstrateReentryReadinessContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func baseSubstrateReentryReadinessContractInputs(t *testing.T) (BaseSubstrateEquivalenceContract, BaseEquivalenceEvidenceSetContract, BaseSubstrateReentryReadinessEvidence) {
	t.Helper()

	current, projection, substrateEvidence := baseSubstrateEquivalenceContractInputs(t)
	substrate, err := BuildBaseSubstrateEquivalenceContract(current, projection, substrateEvidence)
	if err != nil {
		t.Fatalf("BuildBaseSubstrateEquivalenceContract(): %v", err)
	}

	calibration := baseSubstrateReentryReadinessCalibrationContract(t, substrate.Version, false)

	evidence := BaseSubstrateReentryReadinessEvidence{
		SubstrateEquivalenceContractRef: "  base-substrate-equivalence-contract:pass-95  ",
		EquivalenceEvidenceSetRef:       " base-equivalence-evidence-set-contract:pass-95 ",
		NextProbeRef:                    "base-substrate-equivalence-next-probe:pass-95",
		NoRuntimeMaterialization:        true,
		NoOpaqueDataImageDependency:     true,
		NoMutation:                      true,
	}
	return substrate, calibration, evidence
}

func baseSubstrateReentryReadinessCalibrationContract(t *testing.T, version ComputerVersion, narrowedFailure bool) BaseEquivalenceEvidenceSetContract {
	t.Helper()

	left, right, result, checkEvidence := baseEquivalenceCheckBoundaryContractInputs(t)
	left.Version = version
	right.Version = version
	success, err := BuildBaseEquivalenceCheckBoundaryContract(left, right, result, checkEvidence)
	if err != nil {
		t.Fatalf("BuildBaseEquivalenceCheckBoundaryContract(): %v", err)
	}

	failureLeft, failureRight, failureResult, failureEvidence := baseEquivalenceFailureBoundaryContractInputs(t)
	failureLeft.Version = version
	failureRight.Version = version
	if narrowedFailure {
		failureResult = EquivalenceResult{
			Status: EquivalenceNarrowed,
			Unsupported: []UnsupportedCapability{
				{Kind: ObservationFileManifest, Reason: "right materializer cannot expose file manifests for the seeded calibration fixture"},
				{Kind: ObservationBlobSet, Reason: "right materializer cannot expose blob integrity for the seeded calibration fixture"},
			},
		}
	}
	failure, err := BuildBaseEquivalenceFailureBoundaryContract(failureLeft, failureRight, failureResult, failureEvidence)
	if err != nil {
		t.Fatalf("BuildBaseEquivalenceFailureBoundaryContract(): %v", err)
	}

	evidence := BaseEquivalenceEvidenceSetEvidence{
		SuccessContractRef:          "base-equivalence-check-contract:reentry-pass-95",
		FailureContractRef:          "base-equivalence-failure-contract:reentry-pass-95",
		CalibrationSuiteRef:         "calibration-suite:base-substrate-reentry-readiness-pass-95",
		NoRuntimeMaterialization:    true,
		NoOpaqueDataImageDependency: true,
		NoMutation:                  true,
	}
	calibration, err := BuildBaseEquivalenceEvidenceSetContract(success, failure, evidence)
	if err != nil {
		t.Fatalf("BuildBaseEquivalenceEvidenceSetContract(): %v", err)
	}
	return calibration
}
