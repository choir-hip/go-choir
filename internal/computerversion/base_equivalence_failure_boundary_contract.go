package computerversion

import (
	"fmt"
	"strings"
)

const BaseEquivalenceFailureBoundaryContractKind = "base_equivalence_failure_boundary_contract"

const BaseEquivalenceFailureBoundary = "base_equivalence_failure_or_narrowing_boundary"

const BaseEquivalenceFailureScope = "base_file_manifest_blob_set_failure_proof"

// BaseEquivalenceFailureBoundaryEvidence records proof refs for a deliberate
// mismatch or unsupported-capability equivalence result. It exists to prove the
// checker has teeth; it does not authorize deployed mutation or a successful
// equivalence claim.
type BaseEquivalenceFailureBoundaryEvidence struct {
	LeftMaterializerContractRef    string `json:"left_materializer_contract_ref"`
	RightMaterializerContractRef   string `json:"right_materializer_contract_ref"`
	FailureResultRef               string `json:"failure_result_ref"`
	FailureFixtureRef              string `json:"failure_fixture_ref"`
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
	SuccessfulEquivalenceClaimed   bool   `json:"successful_equivalence_claimed"`
	FullSubstrateIndependenceClaim bool   `json:"full_substrate_independence_claim"`
	NoMutation                     bool   `json:"no_mutation"`
}

// BaseEquivalenceFailureBoundaryContract records that a scoped Base equivalence
// comparison failed or narrowed for an explicit reason. It prevents seeded
// mismatch evidence from being collapsed into a passing equivalence claim.
type BaseEquivalenceFailureBoundaryContract struct {
	Kind                           string            `json:"kind"`
	Version                        ComputerVersion   `json:"version"`
	Boundary                       string            `json:"boundary"`
	Scope                          string            `json:"scope"`
	LeftMaterializer               string            `json:"left_materializer"`
	LeftSubstrate                  string            `json:"left_substrate"`
	RightMaterializer              string            `json:"right_materializer"`
	RightSubstrate                 string            `json:"right_substrate"`
	RequiredObservations           []ObservationKind `json:"required_observations"`
	FailureStatus                  EquivalenceStatus `json:"failure_status"`
	DifferenceCount                int               `json:"difference_count"`
	UnsupportedCapabilityCount     int               `json:"unsupported_capability_count"`
	LeftMaterializerContractRef    string            `json:"left_materializer_contract_ref"`
	RightMaterializerContractRef   string            `json:"right_materializer_contract_ref"`
	FailureResultRef               string            `json:"failure_result_ref"`
	FailureFixtureRef              string            `json:"failure_fixture_ref"`
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
	SuccessfulEquivalenceClaimed   bool              `json:"successful_equivalence_claimed"`
	FullSubstrateIndependenceClaim bool              `json:"full_substrate_independence_claim"`
	NoMutation                     bool              `json:"no_mutation"`
}

// BuildBaseEquivalenceFailureBoundaryContract verifies that a scoped Base
// equivalence check failed with concrete differences or narrowed with unsupported
// capabilities, while staying inside local no-mutation evidence.
func BuildBaseEquivalenceFailureBoundaryContract(left, right BaseMaterializerBoundaryContract, result EquivalenceResult, evidence BaseEquivalenceFailureBoundaryEvidence) (BaseEquivalenceFailureBoundaryContract, error) {
	if err := validateBaseEquivalenceCheckMaterializerContract("left", left); err != nil {
		return BaseEquivalenceFailureBoundaryContract{}, err
	}
	if err := validateBaseEquivalenceCheckMaterializerContract("right", right); err != nil {
		return BaseEquivalenceFailureBoundaryContract{}, err
	}
	if left.Version != right.Version {
		return BaseEquivalenceFailureBoundaryContract{}, fmt.Errorf("base equivalence failure boundary: materializer contracts name different computer versions")
	}
	if left.Materializer == right.Materializer && left.Substrate == right.Substrate {
		return BaseEquivalenceFailureBoundaryContract{}, fmt.Errorf("base equivalence failure boundary: materializer contracts must be non-identical")
	}
	if err := validateBaseEquivalenceFailureResult(result); err != nil {
		return BaseEquivalenceFailureBoundaryContract{}, err
	}
	if err := validateBaseEquivalenceFailureBoundaryEvidence(evidence); err != nil {
		return BaseEquivalenceFailureBoundaryContract{}, err
	}
	return BaseEquivalenceFailureBoundaryContract{
		Kind:                           BaseEquivalenceFailureBoundaryContractKind,
		Version:                        left.Version,
		Boundary:                       BaseEquivalenceFailureBoundary,
		Scope:                          BaseEquivalenceFailureScope,
		LeftMaterializer:               left.Materializer,
		LeftSubstrate:                  left.Substrate,
		RightMaterializer:              right.Materializer,
		RightSubstrate:                 right.Substrate,
		RequiredObservations:           []ObservationKind{ObservationBlobSet, ObservationFileManifest},
		FailureStatus:                  result.Status,
		DifferenceCount:                len(result.Differences),
		UnsupportedCapabilityCount:     len(result.Unsupported),
		LeftMaterializerContractRef:    strings.TrimSpace(evidence.LeftMaterializerContractRef),
		RightMaterializerContractRef:   strings.TrimSpace(evidence.RightMaterializerContractRef),
		FailureResultRef:               strings.TrimSpace(evidence.FailureResultRef),
		FailureFixtureRef:              strings.TrimSpace(evidence.FailureFixtureRef),
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
		SuccessfulEquivalenceClaimed:   false,
		FullSubstrateIndependenceClaim: false,
		NoMutation:                     true,
	}, nil
}

func validateBaseEquivalenceFailureResult(result EquivalenceResult) error {
	switch result.Status {
	case EquivalenceNotEquivalent:
		if len(result.Differences) == 0 {
			return fmt.Errorf("base equivalence failure boundary: not_equivalent result must include differences")
		}
		if len(result.Unsupported) != 0 {
			return fmt.Errorf("base equivalence failure boundary: not_equivalent result cannot include unsupported capabilities")
		}
		return nil
	case EquivalenceNarrowed:
		if len(result.Unsupported) == 0 {
			return fmt.Errorf("base equivalence failure boundary: narrowed result must include unsupported capabilities")
		}
		if len(result.Differences) != 0 {
			return fmt.Errorf("base equivalence failure boundary: narrowed result cannot include differences")
		}
		return nil
	case EquivalenceEquivalent:
		return fmt.Errorf("base equivalence failure boundary: equivalent result is not failure evidence")
	default:
		return fmt.Errorf("base equivalence failure boundary: unknown equivalence status %q", result.Status)
	}
}

func validateBaseEquivalenceFailureBoundaryEvidence(evidence BaseEquivalenceFailureBoundaryEvidence) error {
	if strings.TrimSpace(evidence.LeftMaterializerContractRef) == "" {
		return fmt.Errorf("base equivalence failure boundary: left materializer contract ref is required")
	}
	if strings.TrimSpace(evidence.RightMaterializerContractRef) == "" {
		return fmt.Errorf("base equivalence failure boundary: right materializer contract ref is required")
	}
	if strings.TrimSpace(evidence.FailureResultRef) == "" {
		return fmt.Errorf("base equivalence failure boundary: failure result ref is required")
	}
	if strings.TrimSpace(evidence.FailureFixtureRef) == "" {
		return fmt.Errorf("base equivalence failure boundary: failure fixture ref is required")
	}
	if !evidence.NoRuntimeMaterialization {
		return fmt.Errorf("base equivalence failure boundary: evidence must prove no runtime materialization")
	}
	if !evidence.NoOpaqueDataImageDependency {
		return fmt.Errorf("base equivalence failure boundary: evidence must prove no opaque data.img dependency for this failure check")
	}
	switch {
	case evidence.RuntimeBehaviorChanged:
		return fmt.Errorf("base equivalence failure boundary: evidence cannot change runtime behavior")
	case evidence.DeployedRouteRegistered:
		return fmt.Errorf("base equivalence failure boundary: evidence cannot register deployed routes")
	case evidence.ProductionAuthTouched:
		return fmt.Errorf("base equivalence failure boundary: evidence cannot touch production auth/session")
	case evidence.StagingClaimed:
		return fmt.Errorf("base equivalence failure boundary: evidence cannot claim staging")
	case evidence.PromotionClaimed:
		return fmt.Errorf("base equivalence failure boundary: evidence cannot claim promotion")
	case evidence.VMLifecycleTouched:
		return fmt.Errorf("base equivalence failure boundary: evidence cannot touch VM lifecycle")
	case evidence.FirecrackerBootClaimed:
		return fmt.Errorf("base equivalence failure boundary: evidence cannot claim Firecracker boot")
	case evidence.RunAcceptanceRecordTouched:
		return fmt.Errorf("base equivalence failure boundary: evidence cannot touch run acceptance records")
	case evidence.SuccessfulEquivalenceClaimed:
		return fmt.Errorf("base equivalence failure boundary: evidence cannot claim successful equivalence")
	case evidence.FullSubstrateIndependenceClaim:
		return fmt.Errorf("base equivalence failure boundary: evidence cannot claim full substrate independence")
	case !evidence.NoMutation:
		return fmt.Errorf("base equivalence failure boundary: evidence must be no-mutation")
	default:
		return nil
	}
}
