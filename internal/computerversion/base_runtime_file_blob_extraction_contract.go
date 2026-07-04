package computerversion

import (
	"fmt"
	"strings"
)

const BaseRuntimeFileBlobExtractionContractKind = "base_runtime_file_blob_extraction_contract"

const BaseRuntimeFileBlobExtractionBoundary = "runtime_file_blob_observation_extraction_without_opaque_vm_state_or_downstream_claim"

const BaseRuntimeFileBlobExtractionScope = "typed_runtime_file_blob_observations_for_retrying_base_runtime_equivalence"

// BaseRuntimeFileBlobExtractionEvidence records the proof refs for a typed
// runtime file/blob observation boundary. It may make a later runtime
// equivalence check constructive, but it does not itself claim staging,
// promotion, publication, full-substrate independence, or completion.
type BaseRuntimeFileBlobExtractionEvidence struct {
	RuntimeEquivalenceBoundaryRef   string `json:"runtime_equivalence_boundary_ref"`
	RuntimeObservationExtractionRef string `json:"runtime_observation_extraction_ref"`
	ExtractorRef                    string `json:"extractor_ref"`
	NoOpaqueDataImageDependency     bool   `json:"no_opaque_data_img_dependency"`
	NoVMLifecycleMutation           bool   `json:"no_vm_lifecycle_mutation"`
	NoProductionMutation            bool   `json:"no_production_mutation"`
	RuntimeBehaviorChanged          bool   `json:"runtime_behavior_changed"`
	DeployedRouteRegistered         bool   `json:"deployed_route_registered"`
	ProductionAuthTouched           bool   `json:"production_auth_touched"`
	StagingClaimed                  bool   `json:"staging_claimed"`
	PromotionClaimed                bool   `json:"promotion_claimed"`
	VMLifecycleTouched              bool   `json:"vm_lifecycle_touched"`
	FirecrackerBootClaimed          bool   `json:"firecracker_boot_claimed"`
	RunAcceptanceRecordTouched      bool   `json:"run_acceptance_record_touched"`
	PackagePublicationClaimed       bool   `json:"package_publication_claimed"`
	FullSubstrateClaimed            bool   `json:"full_substrate_claimed"`
	CompletionClaimed               bool   `json:"completion_claimed"`
}

// BaseRuntimeFileBlobExtractionContract binds a typed runtime ObservationSet to
// the prior narrowed runtime-equivalence boundary. It is the first constructive
// evidence shape for retrying runtime equivalence because it requires actual
// file_manifest and blob_set observations instead of vm_state_manifest metadata
// or opaque data.img presence.
type BaseRuntimeFileBlobExtractionContract struct {
	Kind                             string            `json:"kind"`
	Version                          ComputerVersion   `json:"version"`
	Boundary                         string            `json:"boundary"`
	Scope                            string            `json:"scope"`
	TypedArtifactProgramRef          string            `json:"typed_artifact_program_ref"`
	RuntimeEquivalenceBoundaryRef    string            `json:"runtime_equivalence_boundary_ref"`
	RuntimeEquivalenceEvidenceRef    string            `json:"runtime_equivalence_evidence_ref"`
	SourceProvenanceReadinessRef     string            `json:"source_provenance_readiness_ref"`
	RuntimeObservationExtractionRef  string            `json:"runtime_observation_extraction_ref"`
	ExtractorRef                     string            `json:"extractor_ref"`
	ExtractedObservationSetName      string            `json:"extracted_observation_set_name"`
	RequiredRuntimeObservations      []ObservationKind `json:"required_runtime_observations"`
	RuntimeFileBlobObservationsReady bool              `json:"runtime_file_blob_observations_ready"`
	RuntimeEquivalenceMayBeRetried   bool              `json:"runtime_equivalence_may_be_retried"`
	RuntimeEquivalenceClaimed        bool              `json:"runtime_equivalence_claimed"`
	NoOpaqueDataImageDependency      bool              `json:"no_opaque_data_img_dependency"`
	NoVMLifecycleMutation            bool              `json:"no_vm_lifecycle_mutation"`
	NoProductionMutation             bool              `json:"no_production_mutation"`
	RuntimeBehaviorChanged           bool              `json:"runtime_behavior_changed"`
	DeployedRouteRegistered          bool              `json:"deployed_route_registered"`
	ProductionAuthTouched            bool              `json:"production_auth_touched"`
	StagingClaimed                   bool              `json:"staging_claimed"`
	PromotionClaimed                 bool              `json:"promotion_claimed"`
	VMLifecycleTouched               bool              `json:"vm_lifecycle_touched"`
	FirecrackerBootClaimed           bool              `json:"firecracker_boot_claimed"`
	RunAcceptanceRecordTouched       bool              `json:"run_acceptance_record_touched"`
	PackagePublicationClaimed        bool              `json:"package_publication_claimed"`
	FullSubstrateClaimed             bool              `json:"full_substrate_claimed"`
	CompletionClaimed                bool              `json:"completion_claimed"`
}

// BuildBaseRuntimeFileBlobExtractionContract verifies that the next runtime
// equivalence proof can be retried only after a typed runtime ObservationSet has
// file_manifest and blob_set observations for the same ComputerVersion.
func BuildBaseRuntimeFileBlobExtractionContract(boundary BaseRuntimeEquivalenceBoundaryContract, observations ObservationSet, evidence BaseRuntimeFileBlobExtractionEvidence) (BaseRuntimeFileBlobExtractionContract, error) {
	if err := validateBaseRuntimeFileBlobExtractionBoundary(boundary); err != nil {
		return BaseRuntimeFileBlobExtractionContract{}, err
	}
	if err := validateBaseRuntimeFileBlobExtractionObservations(boundary, observations); err != nil {
		return BaseRuntimeFileBlobExtractionContract{}, err
	}
	if err := validateBaseRuntimeFileBlobExtractionEvidence(evidence); err != nil {
		return BaseRuntimeFileBlobExtractionContract{}, err
	}

	required := canonicalObservationKinds(observations.RequiredKinds())
	return BaseRuntimeFileBlobExtractionContract{
		Kind:                             BaseRuntimeFileBlobExtractionContractKind,
		Version:                          boundary.Version,
		Boundary:                         BaseRuntimeFileBlobExtractionBoundary,
		Scope:                            BaseRuntimeFileBlobExtractionScope,
		TypedArtifactProgramRef:          string(boundary.Version.ArtifactProgramRef),
		RuntimeEquivalenceBoundaryRef:    strings.TrimSpace(evidence.RuntimeEquivalenceBoundaryRef),
		RuntimeEquivalenceEvidenceRef:    boundary.RuntimeEquivalenceEvidenceRef,
		SourceProvenanceReadinessRef:     boundary.SourceProvenanceReadinessRef,
		RuntimeObservationExtractionRef:  strings.TrimSpace(evidence.RuntimeObservationExtractionRef),
		ExtractorRef:                     strings.TrimSpace(evidence.ExtractorRef),
		ExtractedObservationSetName:      strings.TrimSpace(observations.Name),
		RequiredRuntimeObservations:      required,
		RuntimeFileBlobObservationsReady: true,
		RuntimeEquivalenceMayBeRetried:   true,
		RuntimeEquivalenceClaimed:        false,
		NoOpaqueDataImageDependency:      true,
		NoVMLifecycleMutation:            true,
		NoProductionMutation:             true,
	}, nil
}

func validateBaseRuntimeFileBlobExtractionBoundary(boundary BaseRuntimeEquivalenceBoundaryContract) error {
	if boundary.Kind != BaseRuntimeEquivalenceBoundaryContractKind {
		return fmt.Errorf("base runtime file/blob extraction: boundary kind is %q", boundary.Kind)
	}
	if boundary.Boundary != BaseRuntimeEquivalenceBoundary {
		return fmt.Errorf("base runtime file/blob extraction: boundary contract uses %q", boundary.Boundary)
	}
	if boundary.Scope != BaseRuntimeEquivalenceScope {
		return fmt.Errorf("base runtime file/blob extraction: boundary scope is %q", boundary.Scope)
	}
	if !boundary.Version.Valid() {
		return fmt.Errorf("base runtime file/blob extraction: boundary version is invalid")
	}
	if !boundary.Version.ArtifactProgramRef.Valid() || ArtifactProgramRef(boundary.TypedArtifactProgramRef) != boundary.Version.ArtifactProgramRef {
		return fmt.Errorf("base runtime file/blob extraction: boundary typed artifact-program ref is invalid")
	}
	if strings.TrimSpace(boundary.SourceProvenanceReadinessRef) == "" || strings.TrimSpace(boundary.RuntimeEquivalenceEvidenceRef) == "" {
		return fmt.Errorf("base runtime file/blob extraction: boundary evidence refs are required")
	}
	if !boundary.RuntimeEquivalenceNarrowed || boundary.RuntimeEquivalenceClaimed || boundary.RuntimeEquivalenceStatus != EquivalenceNarrowed {
		return fmt.Errorf("base runtime file/blob extraction: boundary must be narrowed without claiming equivalence")
	}
	if !boundary.DurableStateEquivalenceRequired || !boundary.StagingProofRequired || !boundary.PromotionProofRequired || !boundary.PackagePublicationRequired {
		return fmt.Errorf("base runtime file/blob extraction: boundary must preserve downstream proof requirements")
	}
	if !unsupportedCapabilityContains(boundary.UnsupportedDurableObservations, ObservationFileManifest) || !unsupportedCapabilityContains(boundary.UnsupportedDurableObservations, ObservationBlobSet) {
		return fmt.Errorf("base runtime file/blob extraction: boundary must name unsupported file_manifest and blob_set observations")
	}
	if boundary.RuntimeBehaviorChanged || boundary.DeployedRouteRegistered || boundary.ProductionAuthTouched || boundary.StagingClaimed || boundary.PromotionClaimed || boundary.VMLifecycleTouched || boundary.FirecrackerBootClaimed || boundary.RunAcceptanceRecordTouched || boundary.PackagePublicationClaimed || boundary.FullSubstrateClaimed || boundary.CompletionClaimed {
		return fmt.Errorf("base runtime file/blob extraction: boundary carries protected-surface or completion claims")
	}
	return nil
}

func validateBaseRuntimeFileBlobExtractionObservations(boundary BaseRuntimeEquivalenceBoundaryContract, observations ObservationSet) error {
	if strings.TrimSpace(observations.Name) == "" {
		return fmt.Errorf("base runtime file/blob extraction: observation set name is required")
	}
	if observations.Version != boundary.Version {
		return fmt.Errorf("base runtime file/blob extraction: observation version does not match boundary version")
	}
	if len(observations.Observations) == 0 {
		return fmt.Errorf("base runtime file/blob extraction: observations are empty")
	}
	for _, observation := range observations.Observations {
		if !observation.Valid() {
			return fmt.Errorf("base runtime file/blob extraction: invalid observation %q/%q", observation.Kind, observation.Key)
		}
	}
	required := observations.RequiredKinds()
	if !baseSubstrateEquivalenceHasRequiredScope(required) {
		return fmt.Errorf("base runtime file/blob extraction: observation set must include file_manifest and blob_set")
	}
	if observationKindsContain(required, ObservationVMStateManifest) {
		return fmt.Errorf("base runtime file/blob extraction: observation set cannot rely on vm_state_manifest")
	}
	return nil
}

func validateBaseRuntimeFileBlobExtractionEvidence(evidence BaseRuntimeFileBlobExtractionEvidence) error {
	if strings.TrimSpace(evidence.RuntimeEquivalenceBoundaryRef) == "" || strings.TrimSpace(evidence.RuntimeObservationExtractionRef) == "" || strings.TrimSpace(evidence.ExtractorRef) == "" {
		return fmt.Errorf("base runtime file/blob extraction: evidence refs are required")
	}
	if !evidence.NoOpaqueDataImageDependency {
		return fmt.Errorf("base runtime file/blob extraction: evidence must reject opaque data.img dependency")
	}
	if !evidence.NoVMLifecycleMutation || !evidence.NoProductionMutation {
		return fmt.Errorf("base runtime file/blob extraction: evidence must prove no VM lifecycle or production mutation")
	}
	if evidence.RuntimeBehaviorChanged || evidence.DeployedRouteRegistered || evidence.ProductionAuthTouched || evidence.StagingClaimed || evidence.PromotionClaimed || evidence.VMLifecycleTouched || evidence.FirecrackerBootClaimed || evidence.RunAcceptanceRecordTouched || evidence.PackagePublicationClaimed || evidence.FullSubstrateClaimed || evidence.CompletionClaimed {
		return fmt.Errorf("base runtime file/blob extraction: evidence carries protected-surface or completion claims")
	}
	return nil
}
