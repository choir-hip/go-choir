package computerversion

import (
	"fmt"
	"strings"
)

const BaseMaterializerBoundaryContractKind = "base_materializer_boundary_contract"

const BaseMaterializerBoundary = "base_materializer_realization_without_vm_lifecycle_or_runtime_mutation"

const BaseMaterializerScope = "base_file_manifest_blob_set_realization"

// BaseMaterializerBoundaryEvidence records proof refs for one local Base
// materializer boundary. It certifies a Realization shape and capability scope;
// it does not certify Firecracker boot, VM lifecycle, deployed routing, or full
// computer substrate independence.
type BaseMaterializerBoundaryEvidence struct {
	RealizationRef                 string `json:"realization_ref"`
	CapabilityManifestRef          string `json:"capability_manifest_ref"`
	ObservationSetRef              string `json:"observation_set_ref"`
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

// BaseMaterializerBoundaryContract records that a Base file/blob ObservationSet
// has been projected into a Realization through a declared CapabilityManifest.
// It is below VM lifecycle authority and below cross-substrate completion.
type BaseMaterializerBoundaryContract struct {
	Kind                           string            `json:"kind"`
	Version                        ComputerVersion   `json:"version"`
	Boundary                       string            `json:"boundary"`
	Scope                          string            `json:"scope"`
	RealizationID                  string            `json:"realization_id"`
	Materializer                   string            `json:"materializer"`
	Substrate                      string            `json:"substrate"`
	ObservationSetName             string            `json:"observation_set_name"`
	RequiredObservations           []ObservationKind `json:"required_observations"`
	RealizationRef                 string            `json:"realization_ref"`
	CapabilityManifestRef          string            `json:"capability_manifest_ref"`
	ObservationSetRef              string            `json:"observation_set_ref"`
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

// BuildBaseMaterializerBoundaryContract verifies a local Base realization shape:
// one ComputerVersion, one declared materializer/substrate pair, a scoped Base
// ObservationSet, and no VM/runtime/deployed mutation claims.
func BuildBaseMaterializerBoundaryContract(realization Realization, evidence BaseMaterializerBoundaryEvidence) (BaseMaterializerBoundaryContract, error) {
	if err := validateBaseMaterializerRealization(realization); err != nil {
		return BaseMaterializerBoundaryContract{}, err
	}
	if err := validateBaseMaterializerBoundaryEvidence(evidence); err != nil {
		return BaseMaterializerBoundaryContract{}, err
	}
	return BaseMaterializerBoundaryContract{
		Kind:                           BaseMaterializerBoundaryContractKind,
		Version:                        realization.Version,
		Boundary:                       BaseMaterializerBoundary,
		Scope:                          BaseMaterializerScope,
		RealizationID:                  strings.TrimSpace(realization.ID),
		Materializer:                   strings.TrimSpace(realization.Capabilities.Materializer),
		Substrate:                      strings.TrimSpace(realization.Capabilities.Substrate),
		ObservationSetName:             strings.TrimSpace(realization.Observations.Name),
		RequiredObservations:           []ObservationKind{ObservationBlobSet, ObservationFileManifest},
		RealizationRef:                 strings.TrimSpace(evidence.RealizationRef),
		CapabilityManifestRef:          strings.TrimSpace(evidence.CapabilityManifestRef),
		ObservationSetRef:              strings.TrimSpace(evidence.ObservationSetRef),
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

func validateBaseMaterializerRealization(realization Realization) error {
	if strings.TrimSpace(realization.ID) == "" {
		return fmt.Errorf("base materializer boundary: realization id is required")
	}
	if !realization.Version.Valid() {
		return fmt.Errorf("base materializer boundary: realization version is invalid")
	}
	if strings.TrimSpace(realization.Capabilities.Materializer) == "" {
		return fmt.Errorf("base materializer boundary: materializer name is required")
	}
	if strings.TrimSpace(realization.Capabilities.Substrate) == "" {
		return fmt.Errorf("base materializer boundary: substrate name is required")
	}
	if strings.TrimSpace(realization.Observations.Name) == "" {
		return fmt.Errorf("base materializer boundary: observation set name is required")
	}
	if realization.Observations.Version != realization.Version {
		return fmt.Errorf("base materializer boundary: observation set version does not match realization version")
	}
	if len(realization.Observations.Observations) == 0 {
		return fmt.Errorf("base materializer boundary: observation set is empty")
	}
	if missing := realization.Capabilities.MissingRequired(realization.Observations.RequiredKinds()); len(missing) > 0 {
		return fmt.Errorf("base materializer boundary: capability manifest lacks required observation %q", missing[0].Kind)
	}
	if !baseSubstrateEquivalenceHasRequiredScope(realization.Observations.RequiredKinds()) {
		return fmt.Errorf("base materializer boundary: realization must include file_manifest and blob_set")
	}
	return nil
}

func validateBaseMaterializerBoundaryEvidence(evidence BaseMaterializerBoundaryEvidence) error {
	if strings.TrimSpace(evidence.RealizationRef) == "" {
		return fmt.Errorf("base materializer boundary: realization ref is required")
	}
	if strings.TrimSpace(evidence.CapabilityManifestRef) == "" {
		return fmt.Errorf("base materializer boundary: capability manifest ref is required")
	}
	if strings.TrimSpace(evidence.ObservationSetRef) == "" {
		return fmt.Errorf("base materializer boundary: observation set ref is required")
	}
	if !evidence.NoRuntimeMaterialization {
		return fmt.Errorf("base materializer boundary: evidence must prove no runtime materialization")
	}
	if !evidence.NoOpaqueDataImageDependency {
		return fmt.Errorf("base materializer boundary: evidence must prove no opaque data.img dependency for this realization")
	}
	switch {
	case evidence.RuntimeBehaviorChanged:
		return fmt.Errorf("base materializer boundary: evidence cannot change runtime behavior")
	case evidence.DeployedRouteRegistered:
		return fmt.Errorf("base materializer boundary: evidence cannot register deployed routes")
	case evidence.ProductionAuthTouched:
		return fmt.Errorf("base materializer boundary: evidence cannot touch production auth/session")
	case evidence.StagingClaimed:
		return fmt.Errorf("base materializer boundary: evidence cannot claim staging")
	case evidence.PromotionClaimed:
		return fmt.Errorf("base materializer boundary: evidence cannot claim promotion")
	case evidence.VMLifecycleTouched:
		return fmt.Errorf("base materializer boundary: evidence cannot touch VM lifecycle")
	case evidence.FirecrackerBootClaimed:
		return fmt.Errorf("base materializer boundary: evidence cannot claim Firecracker boot")
	case evidence.RunAcceptanceRecordTouched:
		return fmt.Errorf("base materializer boundary: evidence cannot touch run acceptance records")
	case evidence.FullSubstrateIndependenceClaim:
		return fmt.Errorf("base materializer boundary: evidence cannot claim full substrate independence")
	case !evidence.NoMutation:
		return fmt.Errorf("base materializer boundary: evidence must be no-mutation")
	default:
		return nil
	}
}
