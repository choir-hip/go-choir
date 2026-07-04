package computerversion

import (
	"fmt"
	"strings"
)

const BaseEquivalenceCheckBoundaryContractKind = "base_equivalence_check_boundary_contract"

const BaseEquivalenceCheckBoundary = "base_equivalence_check_between_materializer_boundary_contracts"

const BaseEquivalenceCheckScope = "base_file_manifest_blob_set_equivalence_check"

// BaseEquivalenceCheckBoundaryEvidence records proof refs for the pure
// EquivalenceCheck boundary over two already-scoped Base materializer contracts.
// It does not certify deployed behavior, VM lifecycle, Firecracker boot, or full
// substrate independence.
type BaseEquivalenceCheckBoundaryEvidence struct {
	LeftMaterializerContractRef    string `json:"left_materializer_contract_ref"`
	RightMaterializerContractRef   string `json:"right_materializer_contract_ref"`
	EquivalenceResultRef           string `json:"equivalence_result_ref"`
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

// BaseEquivalenceCheckBoundaryContract records a passing equivalence check over
// two local Base materializer boundary contracts. It is narrower than the full
// mission cross-substrate proof because it only certifies the named local
// materializer contracts and file/blob observation scope.
type BaseEquivalenceCheckBoundaryContract struct {
	Kind                           string            `json:"kind"`
	Version                        ComputerVersion   `json:"version"`
	Boundary                       string            `json:"boundary"`
	Scope                          string            `json:"scope"`
	LeftMaterializer               string            `json:"left_materializer"`
	LeftSubstrate                  string            `json:"left_substrate"`
	RightMaterializer              string            `json:"right_materializer"`
	RightSubstrate                 string            `json:"right_substrate"`
	RequiredObservations           []ObservationKind `json:"required_observations"`
	EquivalenceStatus              EquivalenceStatus `json:"equivalence_status"`
	LeftMaterializerContractRef    string            `json:"left_materializer_contract_ref"`
	RightMaterializerContractRef   string            `json:"right_materializer_contract_ref"`
	EquivalenceResultRef           string            `json:"equivalence_result_ref"`
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

// BuildBaseEquivalenceCheckBoundaryContract verifies that two scoped Base
// materializer contracts can be compared by EquivalenceCheck and that the result
// is actually equivalent. It rejects narrowed or mismatch results rather than
// converting them into a passing boundary contract.
func BuildBaseEquivalenceCheckBoundaryContract(left, right BaseMaterializerBoundaryContract, result EquivalenceResult, evidence BaseEquivalenceCheckBoundaryEvidence) (BaseEquivalenceCheckBoundaryContract, error) {
	if err := validateBaseEquivalenceCheckMaterializerContract("left", left); err != nil {
		return BaseEquivalenceCheckBoundaryContract{}, err
	}
	if err := validateBaseEquivalenceCheckMaterializerContract("right", right); err != nil {
		return BaseEquivalenceCheckBoundaryContract{}, err
	}
	if left.Version != right.Version {
		return BaseEquivalenceCheckBoundaryContract{}, fmt.Errorf("base equivalence check boundary: materializer contracts name different computer versions")
	}
	if left.Materializer == right.Materializer && left.Substrate == right.Substrate {
		return BaseEquivalenceCheckBoundaryContract{}, fmt.Errorf("base equivalence check boundary: materializer contracts must be non-identical")
	}
	if !result.Equivalent() {
		return BaseEquivalenceCheckBoundaryContract{}, fmt.Errorf("base equivalence check boundary: equivalence result is %q, not equivalent", result.Status)
	}
	if err := validateBaseEquivalenceCheckBoundaryEvidence(evidence); err != nil {
		return BaseEquivalenceCheckBoundaryContract{}, err
	}
	return BaseEquivalenceCheckBoundaryContract{
		Kind:                           BaseEquivalenceCheckBoundaryContractKind,
		Version:                        left.Version,
		Boundary:                       BaseEquivalenceCheckBoundary,
		Scope:                          BaseEquivalenceCheckScope,
		LeftMaterializer:               left.Materializer,
		LeftSubstrate:                  left.Substrate,
		RightMaterializer:              right.Materializer,
		RightSubstrate:                 right.Substrate,
		RequiredObservations:           []ObservationKind{ObservationBlobSet, ObservationFileManifest},
		EquivalenceStatus:              result.Status,
		LeftMaterializerContractRef:    strings.TrimSpace(evidence.LeftMaterializerContractRef),
		RightMaterializerContractRef:   strings.TrimSpace(evidence.RightMaterializerContractRef),
		EquivalenceResultRef:           strings.TrimSpace(evidence.EquivalenceResultRef),
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

func validateBaseEquivalenceCheckMaterializerContract(label string, contract BaseMaterializerBoundaryContract) error {
	if contract.Kind != BaseMaterializerBoundaryContractKind {
		return fmt.Errorf("base equivalence check boundary: %s materializer contract kind %q is not %q", label, contract.Kind, BaseMaterializerBoundaryContractKind)
	}
	if !contract.Version.Valid() {
		return fmt.Errorf("base equivalence check boundary: %s materializer contract version is invalid", label)
	}
	if contract.Scope != BaseMaterializerScope {
		return fmt.Errorf("base equivalence check boundary: %s materializer scope %q is not %q", label, contract.Scope, BaseMaterializerScope)
	}
	if strings.TrimSpace(contract.Materializer) == "" || strings.TrimSpace(contract.Substrate) == "" {
		return fmt.Errorf("base equivalence check boundary: %s materializer/substrate is required", label)
	}
	if !baseSubstrateEquivalenceHasRequiredScope(contract.RequiredObservations) {
		return fmt.Errorf("base equivalence check boundary: %s materializer contract must require file_manifest and blob_set", label)
	}
	if !contract.NoRuntimeMaterialization || !contract.NoOpaqueDataImageDependency || !contract.NoMutation || contract.RuntimeBehaviorChanged || contract.DeployedRouteRegistered || contract.ProductionAuthTouched || contract.StagingClaimed || contract.PromotionClaimed || contract.VMLifecycleTouched || contract.FirecrackerBootClaimed || contract.RunAcceptanceRecordTouched || contract.FullSubstrateIndependenceClaim {
		return fmt.Errorf("base equivalence check boundary: %s materializer contract must be local no-mutation evidence", label)
	}
	return nil
}

func validateBaseEquivalenceCheckBoundaryEvidence(evidence BaseEquivalenceCheckBoundaryEvidence) error {
	if strings.TrimSpace(evidence.LeftMaterializerContractRef) == "" {
		return fmt.Errorf("base equivalence check boundary: left materializer contract ref is required")
	}
	if strings.TrimSpace(evidence.RightMaterializerContractRef) == "" {
		return fmt.Errorf("base equivalence check boundary: right materializer contract ref is required")
	}
	if strings.TrimSpace(evidence.EquivalenceResultRef) == "" {
		return fmt.Errorf("base equivalence check boundary: equivalence result ref is required")
	}
	if !evidence.NoRuntimeMaterialization {
		return fmt.Errorf("base equivalence check boundary: evidence must prove no runtime materialization")
	}
	if !evidence.NoOpaqueDataImageDependency {
		return fmt.Errorf("base equivalence check boundary: evidence must prove no opaque data.img dependency for this check")
	}
	switch {
	case evidence.RuntimeBehaviorChanged:
		return fmt.Errorf("base equivalence check boundary: evidence cannot change runtime behavior")
	case evidence.DeployedRouteRegistered:
		return fmt.Errorf("base equivalence check boundary: evidence cannot register deployed routes")
	case evidence.ProductionAuthTouched:
		return fmt.Errorf("base equivalence check boundary: evidence cannot touch production auth/session")
	case evidence.StagingClaimed:
		return fmt.Errorf("base equivalence check boundary: evidence cannot claim staging")
	case evidence.PromotionClaimed:
		return fmt.Errorf("base equivalence check boundary: evidence cannot claim promotion")
	case evidence.VMLifecycleTouched:
		return fmt.Errorf("base equivalence check boundary: evidence cannot touch VM lifecycle")
	case evidence.FirecrackerBootClaimed:
		return fmt.Errorf("base equivalence check boundary: evidence cannot claim Firecracker boot")
	case evidence.RunAcceptanceRecordTouched:
		return fmt.Errorf("base equivalence check boundary: evidence cannot touch run acceptance records")
	case evidence.FullSubstrateIndependenceClaim:
		return fmt.Errorf("base equivalence check boundary: evidence cannot claim full substrate independence")
	case !evidence.NoMutation:
		return fmt.Errorf("base equivalence check boundary: evidence must be no-mutation")
	default:
		return nil
	}
}
