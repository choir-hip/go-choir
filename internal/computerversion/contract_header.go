package computerversion

import (
	"fmt"
	"strings"
)

// ContractHeader carries the identity fields shared by every boundary contract
// in the substrate-independent audited computer proof chain.  All contracts
// embed this struct so that Kind, Version, Boundary, and Scope are always
// present and validated through a single helper.
type ContractHeader struct {
	Kind    string          `json:"kind"`
	Version ComputerVersion `json:"version"`
	Boundary string         `json:"boundary"`
	Scope   string          `json:"scope"`
}

// ValidateContractHeader checks that a contract header has a non-empty Kind,
// a valid Version, and non-empty Boundary and Scope strings.  It does not
// check that the Kind/Boundary/Scope match a specific contract's constants;
// that remains the responsibility of each contract's own validator.
func ValidateContractHeader(h ContractHeader) error {
	if strings.TrimSpace(h.Kind) == "" {
		return fmt.Errorf("contract header: kind is required")
	}
	if !h.Version.Valid() {
		return fmt.Errorf("contract header: version is invalid")
	}
	if strings.TrimSpace(h.Boundary) == "" {
		return fmt.Errorf("contract header: boundary is required")
	}
	if strings.TrimSpace(h.Scope) == "" {
		return fmt.Errorf("contract header: scope is required")
	}
	return nil
}

// NegativeClaims carries the standard set of safety flags that every boundary
// contract must attest.  A "No*Mutation" flag set to true means the contract
// proves that no mutation of that class occurred.  A "*Claimed" or "*Touched"
// flag set to true means the contract illegally claims a protected-surface
// action; validators reject any evidence or contract with these set.
//
// Not every contract uses every flag; contracts embed this struct and set only
// the flags relevant to their boundary.  Validators check the flags that matter
// for their specific boundary.
type NegativeClaims struct {
	// Mutation denials: the contract proves these did not happen.
	NoDeploymentMutation         bool `json:"no_deployment_mutation"`
	NoRouteRegistrationMutation  bool `json:"no_route_registration_mutation"`
	NoRunAcceptanceMutation      bool `json:"no_run_acceptance_mutation"`
	NoProductionMutation         bool `json:"no_production_mutation"`
	NoPromotionMutation          bool `json:"no_promotion_mutation"`
	NoPackagePublicationMutation bool `json:"no_package_publication_mutation"`
	NoVMLifecycleMutation        bool `json:"no_vm_lifecycle_mutation"`
	NoOpaqueDataImageDependency  bool `json:"no_opaque_data_image_dependency"`
	NoRuntimeMaterialization     bool `json:"no_runtime_materialization"`
	NoMutation                   bool `json:"no_mutation"`

	// Protected-surface claims: these must always be false.
	DeploymentExecuted           bool `json:"deployment_executed"`
	StagingHealthClaimed         bool `json:"staging_health_claimed"`
	DeployedRouteIdentityClaimed bool `json:"deployed_route_identity_claimed"`
	RuntimeBehaviorChanged       bool `json:"runtime_behavior_changed"`
	DeployedRouteRegistered      bool `json:"deployed_route_registered"`
	ProductionAuthTouched        bool `json:"production_auth_touched"`
	PromotionClaimed             bool `json:"promotion_claimed"`
	PromotionExecuted            bool `json:"promotion_executed"`
	PackagePublished             bool `json:"package_published"`
	VMLifecycleTouched           bool `json:"vm_lifecycle_touched"`
	FirecrackerBootClaimed       bool `json:"firecracker_boot_claimed"`
	RunAcceptanceRecordTouched   bool `json:"run_acceptance_record_touched"`
	PackagePublicationClaimed    bool `json:"package_publication_claimed"`
	StagingClaimed               bool `json:"staging_claimed"`
	FullSubstrateClaimed         bool `json:"full_substrate_claimed"`
	FullSubstrateIndependenceClaim bool `json:"full_substrate_independence_claim"`
	CompletionClaimed            bool `json:"completion_claimed"`
}

// HasProtectedSurfaceClaim returns true if any protected-surface claim flag
// is set, indicating the contract or evidence illegally claims a protected
// action.  Validators call this to reject contracts that overclaim.
func (nc NegativeClaims) HasProtectedSurfaceClaim() bool {
	return nc.DeploymentExecuted ||
		nc.StagingHealthClaimed ||
		nc.DeployedRouteIdentityClaimed ||
		nc.RuntimeBehaviorChanged ||
		nc.DeployedRouteRegistered ||
		nc.ProductionAuthTouched ||
		nc.PromotionClaimed ||
		nc.PromotionExecuted ||
		nc.PackagePublished ||
		nc.VMLifecycleTouched ||
		nc.FirecrackerBootClaimed ||
		nc.RunAcceptanceRecordTouched ||
		nc.PackagePublicationClaimed ||
		nc.StagingClaimed ||
		nc.FullSubstrateClaimed ||
		nc.FullSubstrateIndependenceClaim ||
		nc.CompletionClaimed
}
