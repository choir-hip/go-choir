package computerversion

import (
	"fmt"
	"strings"
)

const BaseRuntimeEquivalenceRetryContractKind = "base_runtime_equivalence_retry_contract"

const BaseRuntimeEquivalenceRetryBoundary = "source_to_typed_runtime_file_blob_equivalence_without_downstream_claim"

const BaseRuntimeEquivalenceRetryScope = "base_source_file_blob_observations_compared_to_typed_runtime_file_blob_observations"

// BaseRuntimeEquivalenceRetryEvidence records proof refs for retrying runtime
// equivalence after typed runtime file/blob extraction. It must not carry staging,
// promotion, publication, full-substrate, run-acceptance, or completion authority.
type BaseRuntimeEquivalenceRetryEvidence struct {
	SourceObservationSetRef      string `json:"source_observation_set_ref"`
	RuntimeFileBlobExtractionRef string `json:"runtime_file_blob_extraction_ref"`
	RuntimeEquivalenceRetryRef   string `json:"runtime_equivalence_retry_ref"`
	NoVMLifecycleMutation        bool   `json:"no_vm_lifecycle_mutation"`
	NoProductionMutation         bool   `json:"no_production_mutation"`
	NoOpaqueDataImageDependency  bool   `json:"no_opaque_data_img_dependency"`
	RuntimeBehaviorChanged       bool   `json:"runtime_behavior_changed"`
	DeployedRouteRegistered      bool   `json:"deployed_route_registered"`
	ProductionAuthTouched        bool   `json:"production_auth_touched"`
	StagingClaimed               bool   `json:"staging_claimed"`
	PromotionClaimed             bool   `json:"promotion_claimed"`
	VMLifecycleTouched           bool   `json:"vm_lifecycle_touched"`
	FirecrackerBootClaimed       bool   `json:"firecracker_boot_claimed"`
	RunAcceptanceRecordTouched   bool   `json:"run_acceptance_record_touched"`
	PackagePublicationClaimed    bool   `json:"package_publication_claimed"`
	FullSubstrateClaimed         bool   `json:"full_substrate_claimed"`
	CompletionClaimed            bool   `json:"completion_claimed"`
}

// BaseRuntimeEquivalenceRetryContract records a successful equivalence retry
// between source/provenance file/blob observations and typed runtime file/blob
// observations for the same ComputerVersion. It settles only this runtime
// equivalence boundary and preserves every downstream protected surface.
type BaseRuntimeEquivalenceRetryContract struct {
	Kind                            string            `json:"kind"`
	Version                         ComputerVersion   `json:"version"`
	Boundary                        string            `json:"boundary"`
	Scope                           string            `json:"scope"`
	TypedArtifactProgramRef         string            `json:"typed_artifact_program_ref"`
	SourceProvenanceReadinessRef    string            `json:"source_provenance_readiness_ref"`
	SourceObservationSetRef         string            `json:"source_observation_set_ref"`
	RuntimeFileBlobExtractionRef    string            `json:"runtime_file_blob_extraction_ref"`
	RuntimeEquivalenceBoundaryRef   string            `json:"runtime_equivalence_boundary_ref"`
	RuntimeEquivalenceRetryRef      string            `json:"runtime_equivalence_retry_ref"`
	SourceObservationSetName        string            `json:"source_observation_set_name"`
	RuntimeObservationSetName       string            `json:"runtime_observation_set_name"`
	RequiredObservations            []ObservationKind `json:"required_observations"`
	RuntimeEquivalenceStatus        EquivalenceStatus `json:"runtime_equivalence_status"`
	RuntimeEquivalenceClaimed       bool              `json:"runtime_equivalence_claimed"`
	StagingProofRequired            bool              `json:"staging_proof_required"`
	PromotionProofRequired          bool              `json:"promotion_proof_required"`
	PackagePublicationProofRequired bool              `json:"package_publication_proof_required"`
	RunAcceptanceProofRequired      bool              `json:"run_acceptance_proof_required"`
	NoVMLifecycleMutation           bool              `json:"no_vm_lifecycle_mutation"`
	NoProductionMutation            bool              `json:"no_production_mutation"`
	NoOpaqueDataImageDependency     bool              `json:"no_opaque_data_img_dependency"`
	RuntimeBehaviorChanged          bool              `json:"runtime_behavior_changed"`
	DeployedRouteRegistered         bool              `json:"deployed_route_registered"`
	ProductionAuthTouched           bool              `json:"production_auth_touched"`
	StagingClaimed                  bool              `json:"staging_claimed"`
	PromotionClaimed                bool              `json:"promotion_claimed"`
	VMLifecycleTouched              bool              `json:"vm_lifecycle_touched"`
	FirecrackerBootClaimed          bool              `json:"firecracker_boot_claimed"`
	RunAcceptanceRecordTouched      bool              `json:"run_acceptance_record_touched"`
	PackagePublicationClaimed       bool              `json:"package_publication_claimed"`
	FullSubstrateClaimed            bool              `json:"full_substrate_claimed"`
	CompletionClaimed               bool              `json:"completion_claimed"`
}

// BuildBaseRuntimeEquivalenceRetryContract compares source/provenance and typed
// runtime file/blob observations for the same ComputerVersion. Any mismatch is a
// retry failure, not a downstream authority claim.
func BuildBaseRuntimeEquivalenceRetryContract(source BaseSourceProvenanceReadinessContract, extraction BaseRuntimeFileBlobExtractionContract, sourceObservations ObservationSet, runtimeObservations ObservationSet, evidence BaseRuntimeEquivalenceRetryEvidence) (BaseRuntimeEquivalenceRetryContract, error) {
	if err := validateBaseRuntimeEquivalenceRetrySource(source); err != nil {
		return BaseRuntimeEquivalenceRetryContract{}, err
	}
	if err := validateBaseRuntimeEquivalenceRetryExtraction(source, extraction); err != nil {
		return BaseRuntimeEquivalenceRetryContract{}, err
	}
	if err := validateBaseRuntimeEquivalenceRetryObservationSet("source", source.Version, sourceObservations); err != nil {
		return BaseRuntimeEquivalenceRetryContract{}, err
	}
	if err := validateBaseRuntimeEquivalenceRetryObservationSet("runtime", source.Version, runtimeObservations); err != nil {
		return BaseRuntimeEquivalenceRetryContract{}, err
	}
	if err := validateBaseRuntimeEquivalenceRetryEvidence(evidence); err != nil {
		return BaseRuntimeEquivalenceRetryContract{}, err
	}

	result := EquivalenceChecker{}.CheckObservationSets(sourceObservations, runtimeObservations)
	if !result.Equivalent() {
		return BaseRuntimeEquivalenceRetryContract{}, fmt.Errorf("base runtime equivalence retry: observations are not equivalent: %s", result.Status)
	}

	required := canonicalObservationKinds(mergeKinds(sourceObservations.RequiredKinds(), runtimeObservations.RequiredKinds()))
	return BaseRuntimeEquivalenceRetryContract{
		Kind:                            BaseRuntimeEquivalenceRetryContractKind,
		Version:                         source.Version,
		Boundary:                        BaseRuntimeEquivalenceRetryBoundary,
		Scope:                           BaseRuntimeEquivalenceRetryScope,
		TypedArtifactProgramRef:         string(source.Version.ArtifactProgramRef),
		SourceProvenanceReadinessRef:    extraction.SourceProvenanceReadinessRef,
		SourceObservationSetRef:         strings.TrimSpace(evidence.SourceObservationSetRef),
		RuntimeFileBlobExtractionRef:    strings.TrimSpace(evidence.RuntimeFileBlobExtractionRef),
		RuntimeEquivalenceBoundaryRef:   extraction.RuntimeEquivalenceBoundaryRef,
		RuntimeEquivalenceRetryRef:      strings.TrimSpace(evidence.RuntimeEquivalenceRetryRef),
		SourceObservationSetName:        strings.TrimSpace(sourceObservations.Name),
		RuntimeObservationSetName:       strings.TrimSpace(runtimeObservations.Name),
		RequiredObservations:            required,
		RuntimeEquivalenceStatus:        EquivalenceEquivalent,
		RuntimeEquivalenceClaimed:       true,
		StagingProofRequired:            true,
		PromotionProofRequired:          true,
		PackagePublicationProofRequired: true,
		RunAcceptanceProofRequired:      true,
		NoVMLifecycleMutation:           true,
		NoProductionMutation:            true,
		NoOpaqueDataImageDependency:     true,
	}, nil
}

func validateBaseRuntimeEquivalenceRetrySource(source BaseSourceProvenanceReadinessContract) error {
	if err := validateBaseRuntimeMaterializationCeremonySource(source); err != nil {
		return fmt.Errorf("base runtime equivalence retry: invalid source readiness: %w", err)
	}
	if !source.SourceProvenanceReady || !source.RuntimeCeremonyMayOpen || !source.RuntimeProofRequired {
		return fmt.Errorf("base runtime equivalence retry: source readiness must preserve runtime proof requirements")
	}
	return nil
}

func validateBaseRuntimeEquivalenceRetryExtraction(source BaseSourceProvenanceReadinessContract, extraction BaseRuntimeFileBlobExtractionContract) error {
	if extraction.Kind != BaseRuntimeFileBlobExtractionContractKind {
		return fmt.Errorf("base runtime equivalence retry: extraction kind is %q", extraction.Kind)
	}
	if extraction.Boundary != BaseRuntimeFileBlobExtractionBoundary {
		return fmt.Errorf("base runtime equivalence retry: extraction boundary is %q", extraction.Boundary)
	}
	if extraction.Scope != BaseRuntimeFileBlobExtractionScope {
		return fmt.Errorf("base runtime equivalence retry: extraction scope is %q", extraction.Scope)
	}
	if extraction.Version != source.Version {
		return fmt.Errorf("base runtime equivalence retry: extraction version does not match source version")
	}
	if ArtifactProgramRef(extraction.TypedArtifactProgramRef) != source.Version.ArtifactProgramRef {
		return fmt.Errorf("base runtime equivalence retry: extraction typed artifact-program ref does not match source version")
	}
	if strings.TrimSpace(extraction.SourceProvenanceReadinessRef) == "" || strings.TrimSpace(extraction.RuntimeEquivalenceBoundaryRef) == "" || strings.TrimSpace(extraction.RuntimeObservationExtractionRef) == "" {
		return fmt.Errorf("base runtime equivalence retry: extraction refs are required")
	}
	if !extraction.RuntimeFileBlobObservationsReady || !extraction.RuntimeEquivalenceMayBeRetried || extraction.RuntimeEquivalenceClaimed {
		return fmt.Errorf("base runtime equivalence retry: extraction must allow retry without claiming equivalence")
	}
	if !baseSubstrateEquivalenceHasRequiredScope(extraction.RequiredRuntimeObservations) || observationKindsContain(extraction.RequiredRuntimeObservations, ObservationVMStateManifest) {
		return fmt.Errorf("base runtime equivalence retry: extraction must require only typed file/blob runtime observations")
	}
	if !extraction.NoOpaqueDataImageDependency || !extraction.NoVMLifecycleMutation || !extraction.NoProductionMutation {
		return fmt.Errorf("base runtime equivalence retry: extraction must reject opaque data.img, VM lifecycle mutation, and production mutation")
	}
	if extraction.RuntimeBehaviorChanged || extraction.DeployedRouteRegistered || extraction.ProductionAuthTouched || extraction.StagingClaimed || extraction.PromotionClaimed || extraction.VMLifecycleTouched || extraction.FirecrackerBootClaimed || extraction.RunAcceptanceRecordTouched || extraction.PackagePublicationClaimed || extraction.FullSubstrateClaimed || extraction.CompletionClaimed {
		return fmt.Errorf("base runtime equivalence retry: extraction carries protected-surface or completion claims")
	}
	return nil
}

func validateBaseRuntimeEquivalenceRetryObservationSet(label string, version ComputerVersion, observations ObservationSet) error {
	if strings.TrimSpace(observations.Name) == "" {
		return fmt.Errorf("base runtime equivalence retry: %s observation set name is required", label)
	}
	if observations.Version != version {
		return fmt.Errorf("base runtime equivalence retry: %s observation version does not match source version", label)
	}
	if len(observations.Observations) == 0 {
		return fmt.Errorf("base runtime equivalence retry: %s observations are empty", label)
	}
	for _, observation := range observations.Observations {
		if !observation.Valid() {
			return fmt.Errorf("base runtime equivalence retry: invalid %s observation %q/%q", label, observation.Kind, observation.Key)
		}
	}
	required := observations.RequiredKinds()
	if !baseSubstrateEquivalenceHasRequiredScope(required) {
		return fmt.Errorf("base runtime equivalence retry: %s observation set must include file_manifest and blob_set", label)
	}
	if observationKindsContain(required, ObservationVMStateManifest) {
		return fmt.Errorf("base runtime equivalence retry: %s observation set cannot rely on vm_state_manifest", label)
	}
	return nil
}

func validateBaseRuntimeEquivalenceRetryEvidence(evidence BaseRuntimeEquivalenceRetryEvidence) error {
	if strings.TrimSpace(evidence.SourceObservationSetRef) == "" || strings.TrimSpace(evidence.RuntimeFileBlobExtractionRef) == "" || strings.TrimSpace(evidence.RuntimeEquivalenceRetryRef) == "" {
		return fmt.Errorf("base runtime equivalence retry: evidence refs are required")
	}
	if !evidence.NoVMLifecycleMutation || !evidence.NoProductionMutation || !evidence.NoOpaqueDataImageDependency {
		return fmt.Errorf("base runtime equivalence retry: evidence must prove no VM lifecycle mutation, production mutation, or opaque data.img dependency")
	}
	if evidence.RuntimeBehaviorChanged || evidence.DeployedRouteRegistered || evidence.ProductionAuthTouched || evidence.StagingClaimed || evidence.PromotionClaimed || evidence.VMLifecycleTouched || evidence.FirecrackerBootClaimed || evidence.RunAcceptanceRecordTouched || evidence.PackagePublicationClaimed || evidence.FullSubstrateClaimed || evidence.CompletionClaimed {
		return fmt.Errorf("base runtime equivalence retry: evidence carries protected-surface or completion claims")
	}
	return nil
}
