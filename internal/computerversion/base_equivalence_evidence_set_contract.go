package computerversion

import (
	"fmt"
	"strings"
)

const BaseEquivalenceEvidenceSetContractKind = "base_equivalence_evidence_set_contract"

const BaseEquivalenceEvidenceSetBoundary = "base_equivalence_positive_and_negative_fixture_boundary"

const BaseEquivalenceEvidenceSetScope = "base_file_manifest_blob_set_equivalence_calibration"

// BaseEquivalenceEvidenceSetEvidence binds one successful equivalence proof and
// one deliberate failure proof into a calibration set. The set proves the local
// checker accepts and rejects under scoped fixtures; it is not substrate-wide
// authority or product runtime evidence.
type BaseEquivalenceEvidenceSetEvidence struct {
	SuccessContractRef             string `json:"success_contract_ref"`
	FailureContractRef             string `json:"failure_contract_ref"`
	CalibrationSuiteRef            string `json:"calibration_suite_ref"`
	NoRuntimeMaterialization       bool   `json:"no_runtime_materialization"`
	NoOpaqueDataImageDependency    bool   `json:"no_opaque_data_img_dependency"`
	RuntimeBehaviorChanged         bool   `json:"runtime_behavior_changed"`
	DeployedRouteRegistered        bool   `json:"deployed_route_registered"`
	ProductionAuthTouched          bool   `json:"production_auth_touched"`
	StagingClaimed                 bool   `json:"staging_claimed"`
	PromotionClaimed               bool   `json:"promotion_claimed"`
	VMLifecycleTouched             bool   `json:"vm_lifecycle_touched"`
	FirecrackerBootClaimed         bool   `json:"firecracker_boot_claimed"`
	RunAcceptanceRecordTouched     bool   `json:"run_acceptance_record_touched"`
	FullSubstrateIndependenceClaim bool   `json:"full_substrate_independence_claim"`
	NoMutation                     bool   `json:"no_mutation"`
}

// BaseEquivalenceEvidenceSetContract records that the scoped Base equivalence
// checker has both passing and failing evidence below the product/runtime line.
type BaseEquivalenceEvidenceSetContract struct {
	Kind                           string            `json:"kind"`
	Version                        ComputerVersion   `json:"version"`
	Boundary                       string            `json:"boundary"`
	Scope                          string            `json:"scope"`
	SuccessStatus                  EquivalenceStatus `json:"success_status"`
	FailureStatus                  EquivalenceStatus `json:"failure_status"`
	SuccessDifferenceCount         int               `json:"success_difference_count"`
	FailureDifferenceCount         int               `json:"failure_difference_count"`
	FailureUnsupportedCount        int               `json:"failure_unsupported_count"`
	RequiredObservations           []ObservationKind `json:"required_observations"`
	SuccessContractRef             string            `json:"success_contract_ref"`
	FailureContractRef             string            `json:"failure_contract_ref"`
	CalibrationSuiteRef            string            `json:"calibration_suite_ref"`
	NoRuntimeMaterialization       bool              `json:"no_runtime_materialization"`
	NoOpaqueDataImageDependency    bool              `json:"no_opaque_data_img_dependency"`
	RuntimeBehaviorChanged         bool              `json:"runtime_behavior_changed"`
	DeployedRouteRegistered        bool              `json:"deployed_route_registered"`
	ProductionAuthTouched          bool              `json:"production_auth_touched"`
	StagingClaimed                 bool              `json:"staging_claimed"`
	PromotionClaimed               bool              `json:"promotion_claimed"`
	VMLifecycleTouched             bool              `json:"vm_lifecycle_touched"`
	FirecrackerBootClaimed         bool              `json:"firecracker_boot_claimed"`
	RunAcceptanceRecordTouched     bool              `json:"run_acceptance_record_touched"`
	FullSubstrateIndependenceClaim bool              `json:"full_substrate_independence_claim"`
	NoMutation                     bool              `json:"no_mutation"`
}

// BuildBaseEquivalenceEvidenceSetContract binds a successful equivalence
// boundary and a failure boundary for the same ComputerVersion into a local
// calibration set. It preserves the distinction between checker calibration and
// full substrate-independence proof.
func BuildBaseEquivalenceEvidenceSetContract(success BaseEquivalenceCheckBoundaryContract, failure BaseEquivalenceFailureBoundaryContract, evidence BaseEquivalenceEvidenceSetEvidence) (BaseEquivalenceEvidenceSetContract, error) {
	if err := validateBaseEquivalenceEvidenceSetSuccess(success); err != nil {
		return BaseEquivalenceEvidenceSetContract{}, err
	}
	if err := validateBaseEquivalenceEvidenceSetFailure(failure); err != nil {
		return BaseEquivalenceEvidenceSetContract{}, err
	}
	if success.Version != failure.Version {
		return BaseEquivalenceEvidenceSetContract{}, fmt.Errorf("base equivalence evidence set: success and failure contracts name different computer versions")
	}
	if err := validateBaseEquivalenceEvidenceSetEvidence(evidence); err != nil {
		return BaseEquivalenceEvidenceSetContract{}, err
	}
	return BaseEquivalenceEvidenceSetContract{
		Kind:                           BaseEquivalenceEvidenceSetContractKind,
		Version:                        success.Version,
		Boundary:                       BaseEquivalenceEvidenceSetBoundary,
		Scope:                          BaseEquivalenceEvidenceSetScope,
		SuccessStatus:                  success.EquivalenceStatus,
		FailureStatus:                  failure.FailureStatus,
		SuccessDifferenceCount:         0,
		FailureDifferenceCount:         failure.DifferenceCount,
		FailureUnsupportedCount:        failure.UnsupportedCapabilityCount,
		RequiredObservations:           []ObservationKind{ObservationBlobSet, ObservationFileManifest},
		SuccessContractRef:             strings.TrimSpace(evidence.SuccessContractRef),
		FailureContractRef:             strings.TrimSpace(evidence.FailureContractRef),
		CalibrationSuiteRef:            strings.TrimSpace(evidence.CalibrationSuiteRef),
		NoRuntimeMaterialization:       true,
		NoOpaqueDataImageDependency:    true,
		RuntimeBehaviorChanged:         false,
		DeployedRouteRegistered:        false,
		ProductionAuthTouched:          false,
		StagingClaimed:                 false,
		PromotionClaimed:               false,
		VMLifecycleTouched:             false,
		FirecrackerBootClaimed:         false,
		RunAcceptanceRecordTouched:     false,
		FullSubstrateIndependenceClaim: false,
		NoMutation:                     true,
	}, nil
}

func validateBaseEquivalenceEvidenceSetSuccess(success BaseEquivalenceCheckBoundaryContract) error {
	if success.Kind != BaseEquivalenceCheckBoundaryContractKind {
		return fmt.Errorf("base equivalence evidence set: success contract kind is %q", success.Kind)
	}
	if success.Boundary != BaseEquivalenceCheckBoundary {
		return fmt.Errorf("base equivalence evidence set: success contract boundary is %q", success.Boundary)
	}
	if success.Scope != BaseEquivalenceCheckScope {
		return fmt.Errorf("base equivalence evidence set: success contract scope is %q", success.Scope)
	}
	if success.EquivalenceStatus != EquivalenceEquivalent {
		return fmt.Errorf("base equivalence evidence set: success contract must be equivalent")
	}
	if !baseEquivalenceEvidenceSetHasRequiredScope(success.RequiredObservations) {
		return fmt.Errorf("base equivalence evidence set: success contract must include file_manifest and blob_set")
	}
	if !success.NoRuntimeMaterialization || !success.NoOpaqueDataImageDependency || !success.NoMutation {
		return fmt.Errorf("base equivalence evidence set: success contract has unsafe proof flags")
	}
	if success.RuntimeBehaviorChanged || success.DeployedRouteRegistered || success.ProductionAuthTouched || success.StagingClaimed || success.PromotionClaimed || success.VMLifecycleTouched || success.FirecrackerBootClaimed || success.RunAcceptanceRecordTouched || success.FullSubstrateIndependenceClaim {
		return fmt.Errorf("base equivalence evidence set: success contract carries protected-surface claims")
	}
	return nil
}

func validateBaseEquivalenceEvidenceSetFailure(failure BaseEquivalenceFailureBoundaryContract) error {
	if failure.Kind != BaseEquivalenceFailureBoundaryContractKind {
		return fmt.Errorf("base equivalence evidence set: failure contract kind is %q", failure.Kind)
	}
	if failure.Boundary != BaseEquivalenceFailureBoundary {
		return fmt.Errorf("base equivalence evidence set: failure contract boundary is %q", failure.Boundary)
	}
	if failure.Scope != BaseEquivalenceFailureScope {
		return fmt.Errorf("base equivalence evidence set: failure contract scope is %q", failure.Scope)
	}
	switch failure.FailureStatus {
	case EquivalenceNotEquivalent:
		if failure.DifferenceCount == 0 || failure.UnsupportedCapabilityCount != 0 {
			return fmt.Errorf("base equivalence evidence set: non-equivalent failure contract has invalid counts")
		}
	case EquivalenceNarrowed:
		if failure.UnsupportedCapabilityCount == 0 || failure.DifferenceCount != 0 {
			return fmt.Errorf("base equivalence evidence set: narrowed failure contract has invalid counts")
		}
	default:
		return fmt.Errorf("base equivalence evidence set: failure contract status is %q", failure.FailureStatus)
	}
	if !baseEquivalenceEvidenceSetHasRequiredScope(failure.RequiredObservations) {
		return fmt.Errorf("base equivalence evidence set: failure contract must include file_manifest and blob_set")
	}
	if failure.SuccessfulEquivalenceClaimed {
		return fmt.Errorf("base equivalence evidence set: failure contract claims successful equivalence")
	}
	if !failure.NoRuntimeMaterialization || !failure.NoOpaqueDataImageDependency || !failure.NoMutation {
		return fmt.Errorf("base equivalence evidence set: failure contract has unsafe proof flags")
	}
	if failure.RuntimeBehaviorChanged || failure.DeployedRouteRegistered || failure.ProductionAuthTouched || failure.StagingClaimed || failure.PromotionClaimed || failure.VMLifecycleTouched || failure.FirecrackerBootClaimed || failure.RunAcceptanceRecordTouched || failure.FullSubstrateIndependenceClaim {
		return fmt.Errorf("base equivalence evidence set: failure contract carries protected-surface claims")
	}
	return nil
}

func validateBaseEquivalenceEvidenceSetEvidence(evidence BaseEquivalenceEvidenceSetEvidence) error {
	if strings.TrimSpace(evidence.SuccessContractRef) == "" {
		return fmt.Errorf("base equivalence evidence set: success contract ref is required")
	}
	if strings.TrimSpace(evidence.FailureContractRef) == "" {
		return fmt.Errorf("base equivalence evidence set: failure contract ref is required")
	}
	if strings.TrimSpace(evidence.CalibrationSuiteRef) == "" {
		return fmt.Errorf("base equivalence evidence set: calibration suite ref is required")
	}
	if !evidence.NoRuntimeMaterialization {
		return fmt.Errorf("base equivalence evidence set: evidence must prove no runtime materialization")
	}
	if !evidence.NoOpaqueDataImageDependency {
		return fmt.Errorf("base equivalence evidence set: evidence must prove no opaque data.img dependency")
	}
	switch {
	case evidence.RuntimeBehaviorChanged:
		return fmt.Errorf("base equivalence evidence set: evidence cannot change runtime behavior")
	case evidence.DeployedRouteRegistered:
		return fmt.Errorf("base equivalence evidence set: evidence cannot register deployed routes")
	case evidence.ProductionAuthTouched:
		return fmt.Errorf("base equivalence evidence set: evidence cannot touch production auth/session")
	case evidence.StagingClaimed:
		return fmt.Errorf("base equivalence evidence set: evidence cannot claim staging")
	case evidence.PromotionClaimed:
		return fmt.Errorf("base equivalence evidence set: evidence cannot claim promotion")
	case evidence.VMLifecycleTouched:
		return fmt.Errorf("base equivalence evidence set: evidence cannot touch VM lifecycle")
	case evidence.FirecrackerBootClaimed:
		return fmt.Errorf("base equivalence evidence set: evidence cannot claim Firecracker boot")
	case evidence.RunAcceptanceRecordTouched:
		return fmt.Errorf("base equivalence evidence set: evidence cannot touch run acceptance records")
	case evidence.FullSubstrateIndependenceClaim:
		return fmt.Errorf("base equivalence evidence set: evidence cannot claim full substrate independence")
	case !evidence.NoMutation:
		return fmt.Errorf("base equivalence evidence set: evidence must be no-mutation")
	default:
		return nil
	}
}

func baseEquivalenceEvidenceSetHasRequiredScope(kinds []ObservationKind) bool {
	seen := make(map[ObservationKind]struct{}, len(kinds))
	for _, kind := range kinds {
		seen[kind] = struct{}{}
	}
	_, hasFile := seen[ObservationFileManifest]
	_, hasBlob := seen[ObservationBlobSet]
	return hasFile && hasBlob
}
