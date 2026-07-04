package computerversion

import (
	"fmt"
	"strings"
)

const BaseSubstrateEquivalenceContractKind = "base_substrate_equivalence_contract"

const BaseSubstrateEquivalenceBoundary = "scoped_base_current_state_projection_equivalence_without_runtime_mutation"

const BaseSubstrateEquivalenceClaimScope = "base_current_state_file_manifest_blob_set"

// BaseSubstrateEquivalenceEvidence names the proof artifacts for a scoped Base
// current-state equivalence comparison. It records evidence refs only; it does
// not mutate runtime behavior, register routes, claim staging, or promote.
type BaseSubstrateEquivalenceEvidence struct {
	ClaimScope                     string `json:"claim_scope"`
	CurrentRealizationRef          string `json:"current_realization_ref"`
	ProjectionRealizationRef       string `json:"projection_realization_ref"`
	CurrentObservationRef          string `json:"current_observation_ref"`
	ProjectionObservationRef       string `json:"projection_observation_ref"`
	EquivalenceEvidenceRef         string `json:"equivalence_evidence_ref"`
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
	CompletionClaimed              bool   `json:"completion_claimed"`
	NoMutation                     bool   `json:"no_mutation"`
}

// BaseSubstrateEquivalenceContract records a passed equivalence comparison
// between the existing Base current-state reader and a non-identical projection
// materializer for the file-manifest/blob-set slice of one ComputerVersion.
type BaseSubstrateEquivalenceContract struct {
	Kind                           string            `json:"kind"`
	Version                        ComputerVersion   `json:"version"`
	Boundary                       string            `json:"boundary"`
	ClaimScope                     string            `json:"claim_scope"`
	CurrentRealizationRef          string            `json:"current_realization_ref"`
	ProjectionRealizationRef       string            `json:"projection_realization_ref"`
	CurrentObservationRef          string            `json:"current_observation_ref"`
	ProjectionObservationRef       string            `json:"projection_observation_ref"`
	EquivalenceEvidenceRef         string            `json:"equivalence_evidence_ref"`
	CurrentMaterializer            string            `json:"current_materializer"`
	CurrentSubstrate               string            `json:"current_substrate"`
	ProjectionMaterializer         string            `json:"projection_materializer"`
	ProjectionSubstrate            string            `json:"projection_substrate"`
	RequiredObservations           []ObservationKind `json:"required_observations"`
	EquivalenceStatus              EquivalenceStatus `json:"equivalence_status"`
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
	CompletionClaimed              bool              `json:"completion_claimed"`
	NoMutation                     bool              `json:"no_mutation"`
}

// BuildBaseSubstrateEquivalenceContract verifies and records a scoped
// cross-projection equivalence proof. It accepts only equivalent realization
// comparisons for non-identical materializer/substrate identities.
func BuildBaseSubstrateEquivalenceContract(current, projection Realization, evidence BaseSubstrateEquivalenceEvidence) (BaseSubstrateEquivalenceContract, error) {
	if err := validateBaseSubstrateEquivalenceEvidence(evidence); err != nil {
		return BaseSubstrateEquivalenceContract{}, err
	}
	if err := validateBaseSubstrateEquivalenceRealization("current", current); err != nil {
		return BaseSubstrateEquivalenceContract{}, err
	}
	if err := validateBaseSubstrateEquivalenceRealization("projection", projection); err != nil {
		return BaseSubstrateEquivalenceContract{}, err
	}
	if current.Version != projection.Version {
		return BaseSubstrateEquivalenceContract{}, fmt.Errorf("base substrate equivalence: realizations name different computer versions")
	}
	if strings.TrimSpace(current.Capabilities.Materializer) == strings.TrimSpace(projection.Capabilities.Materializer) && strings.TrimSpace(current.Capabilities.Substrate) == strings.TrimSpace(projection.Capabilities.Substrate) {
		return BaseSubstrateEquivalenceContract{}, fmt.Errorf("base substrate equivalence: projection must use a non-identical materializer or substrate")
	}
	required := canonicalObservationKinds(mergeKinds(current.Observations.RequiredKinds(), projection.Observations.RequiredKinds()))
	if !baseSubstrateEquivalenceHasRequiredScope(required) {
		return BaseSubstrateEquivalenceContract{}, fmt.Errorf("base substrate equivalence: required observations must include file_manifest and blob_set")
	}
	result := EquivalenceChecker{}.CheckRealizations(current, projection)
	if result.Status == EquivalenceNarrowed {
		return BaseSubstrateEquivalenceContract{}, fmt.Errorf("base substrate equivalence: claim narrowed by unsupported capabilities: %v", result.Unsupported)
	}
	if !result.Equivalent() {
		return BaseSubstrateEquivalenceContract{}, fmt.Errorf("base substrate equivalence: realizations are not equivalent: %v", result.Differences)
	}

	return BaseSubstrateEquivalenceContract{
		Kind:                           BaseSubstrateEquivalenceContractKind,
		Version:                        current.Version,
		Boundary:                       BaseSubstrateEquivalenceBoundary,
		ClaimScope:                     BaseSubstrateEquivalenceClaimScope,
		CurrentRealizationRef:          strings.TrimSpace(evidence.CurrentRealizationRef),
		ProjectionRealizationRef:       strings.TrimSpace(evidence.ProjectionRealizationRef),
		CurrentObservationRef:          strings.TrimSpace(evidence.CurrentObservationRef),
		ProjectionObservationRef:       strings.TrimSpace(evidence.ProjectionObservationRef),
		EquivalenceEvidenceRef:         strings.TrimSpace(evidence.EquivalenceEvidenceRef),
		CurrentMaterializer:            strings.TrimSpace(current.Capabilities.Materializer),
		CurrentSubstrate:               strings.TrimSpace(current.Capabilities.Substrate),
		ProjectionMaterializer:         strings.TrimSpace(projection.Capabilities.Materializer),
		ProjectionSubstrate:            strings.TrimSpace(projection.Capabilities.Substrate),
		RequiredObservations:           required,
		EquivalenceStatus:              EquivalenceEquivalent,
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
		CompletionClaimed:              false,
		NoMutation:                     true,
	}, nil
}

func validateBaseSubstrateEquivalenceEvidence(evidence BaseSubstrateEquivalenceEvidence) error {
	if strings.TrimSpace(evidence.ClaimScope) != BaseSubstrateEquivalenceClaimScope {
		return fmt.Errorf("base substrate equivalence: claim scope %q is not %q", evidence.ClaimScope, BaseSubstrateEquivalenceClaimScope)
	}
	if strings.TrimSpace(evidence.CurrentRealizationRef) == "" {
		return fmt.Errorf("base substrate equivalence: current realization ref is required")
	}
	if strings.TrimSpace(evidence.ProjectionRealizationRef) == "" {
		return fmt.Errorf("base substrate equivalence: projection realization ref is required")
	}
	if strings.TrimSpace(evidence.CurrentObservationRef) == "" {
		return fmt.Errorf("base substrate equivalence: current observation ref is required")
	}
	if strings.TrimSpace(evidence.ProjectionObservationRef) == "" {
		return fmt.Errorf("base substrate equivalence: projection observation ref is required")
	}
	if strings.TrimSpace(evidence.EquivalenceEvidenceRef) == "" {
		return fmt.Errorf("base substrate equivalence: equivalence evidence ref is required")
	}
	if !evidence.NoRuntimeMaterialization {
		return fmt.Errorf("base substrate equivalence: evidence must prove no runtime materialization")
	}
	if !evidence.NoOpaqueDataImageDependency {
		return fmt.Errorf("base substrate equivalence: evidence must prove no opaque data.img dependency")
	}
	switch {
	case evidence.RuntimeBehaviorChanged:
		return fmt.Errorf("base substrate equivalence: evidence cannot change runtime behavior")
	case evidence.DeployedRouteRegistered:
		return fmt.Errorf("base substrate equivalence: evidence cannot register deployed routes")
	case evidence.ProductionAuthTouched:
		return fmt.Errorf("base substrate equivalence: evidence cannot touch production auth/session")
	case evidence.StagingClaimed:
		return fmt.Errorf("base substrate equivalence: evidence cannot claim staging")
	case evidence.PromotionClaimed:
		return fmt.Errorf("base substrate equivalence: evidence cannot claim promotion")
	case evidence.VMLifecycleTouched:
		return fmt.Errorf("base substrate equivalence: evidence cannot touch VM lifecycle")
	case evidence.FirecrackerBootClaimed:
		return fmt.Errorf("base substrate equivalence: evidence cannot claim Firecracker boot")
	case evidence.RunAcceptanceRecordTouched:
		return fmt.Errorf("base substrate equivalence: evidence cannot touch run acceptance records")
	case evidence.FullSubstrateIndependenceClaim:
		return fmt.Errorf("base substrate equivalence: evidence cannot claim full substrate independence")
	case evidence.CompletionClaimed:
		return fmt.Errorf("base substrate equivalence: evidence cannot claim completion")
	case !evidence.NoMutation:
		return fmt.Errorf("base substrate equivalence: evidence must be no-mutation")
	default:
		return nil
	}
}

func validateBaseSubstrateEquivalenceRealization(label string, realization Realization) error {
	if strings.TrimSpace(realization.ID) == "" {
		return fmt.Errorf("base substrate equivalence: %s realization id is required", label)
	}
	if !realization.Version.Valid() {
		return fmt.Errorf("base substrate equivalence: %s realization version is invalid", label)
	}
	if strings.TrimSpace(realization.Capabilities.Materializer) == "" {
		return fmt.Errorf("base substrate equivalence: %s materializer is required", label)
	}
	if strings.TrimSpace(realization.Capabilities.Substrate) == "" {
		return fmt.Errorf("base substrate equivalence: %s substrate is required", label)
	}
	if realization.Observations.Version != realization.Version {
		return fmt.Errorf("base substrate equivalence: %s observation version does not match realization version", label)
	}
	if len(realization.Observations.Observations) == 0 {
		return fmt.Errorf("base substrate equivalence: %s observations are empty", label)
	}
	return nil
}

func baseSubstrateEquivalenceHasRequiredScope(kinds []ObservationKind) bool {
	seen := make(map[ObservationKind]struct{}, len(kinds))
	for _, kind := range kinds {
		seen[kind] = struct{}{}
	}
	_, hasFileManifest := seen[ObservationFileManifest]
	_, hasBlobSet := seen[ObservationBlobSet]
	return hasFileManifest && hasBlobSet
}
