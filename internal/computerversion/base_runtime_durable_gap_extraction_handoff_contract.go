package computerversion

import (
	"fmt"
	"strings"
)

const BaseRuntimeDurableGapExtractionHandoffContractKind = "base_runtime_durable_gap_extraction_handoff_contract"

const BaseRuntimeDurableGapExtractionHandoffBoundary = "runtime_durable_gap_extraction_handoff_without_retry_or_downstream_claim"

const BaseRuntimeDurableGapExtractionHandoffScope = "runtime_durable_gap_to_typed_runtime_file_blob_extraction"

const BaseRuntimeDurableGapExtractionHandoffStatusReady = "runtime_file_blob_extraction_admitted_retry_still_required"

// BaseRuntimeDurableGapExtractionHandoffEvidence records refs for admitting an
// existing typed runtime file/blob extraction contract under the newer
// runtime-durable proof gap. It is a handoff to retry evidence only; it does not
// claim runtime equivalence, mutate runtime state, or grant downstream authority.
type BaseRuntimeDurableGapExtractionHandoffEvidence struct {
	RuntimeDurableProofGapRef      string `json:"runtime_durable_proof_gap_ref"`
	RuntimeFileBlobExtractionRef   string `json:"runtime_file_blob_extraction_ref"`
	ExtractionHandoffReviewRef     string `json:"extraction_handoff_review_ref"`
	RuntimeEquivalenceRetryPlanRef string `json:"runtime_equivalence_retry_plan_ref"`
	RollbackPlanRef                string `json:"rollback_plan_ref"`
	NoOpaqueDataImageDependency    bool   `json:"no_opaque_data_img_dependency"`
	NoVMLifecycleMutation          bool   `json:"no_vm_lifecycle_mutation"`
	NoDurableComputerMutation      bool   `json:"no_durable_computer_mutation"`
	NoDeployedRouteMutation        bool   `json:"no_deployed_route_mutation"`
	NoProductionMutation           bool   `json:"no_production_mutation"`
	NoPackagePublicationMutation   bool   `json:"no_package_publication_mutation"`
	NoPromotionMutation            bool   `json:"no_promotion_mutation"`
	NoRunAcceptanceMutation        bool   `json:"no_run_acceptance_mutation"`
	RuntimeBehaviorChanged         bool   `json:"runtime_behavior_changed"`
	RuntimeEquivalenceClaimed      bool   `json:"runtime_equivalence_claimed"`
	DurableComputerStateMutated    bool   `json:"durable_computer_state_mutated"`
	DeployedRouteRegistered        bool   `json:"deployed_route_registered"`
	ProductionAuthTouched          bool   `json:"production_auth_touched"`
	ProductionStateMutated         bool   `json:"production_state_mutated"`
	PackagePublished               bool   `json:"package_published"`
	PromotionExecuted              bool   `json:"promotion_executed"`
	VMLifecycleTouched             bool   `json:"vm_lifecycle_touched"`
	FirecrackerBootClaimed         bool   `json:"firecracker_boot_claimed"`
	StagingClaimed                 bool   `json:"staging_claimed"`
	RunAcceptanceRecordTouched     bool   `json:"run_acceptance_record_touched"`
	FullSubstrateClaimed           bool   `json:"full_substrate_claimed"`
	CompletionClaimed              bool   `json:"completion_claimed"`
}

// BaseRuntimeDurableGapExtractionHandoffContract connects the open
// runtime-durable proof gap to a typed runtime file/blob extraction contract.
// It satisfies only the extraction precondition and leaves runtime equivalence
// retry and all downstream proof obligations open.
type BaseRuntimeDurableGapExtractionHandoffContract struct {
	Kind                               string                  `json:"kind"`
	Version                            ComputerVersion         `json:"version"`
	Boundary                           string                  `json:"boundary"`
	Scope                              string                  `json:"scope"`
	TypedArtifactProgramRef            string                  `json:"typed_artifact_program_ref"`
	RuntimeDurableProofGapRef          string                  `json:"runtime_durable_proof_gap_ref"`
	RuntimeFileBlobExtractionRef       string                  `json:"runtime_file_blob_extraction_ref"`
	ExtractionHandoffReviewRef         string                  `json:"extraction_handoff_review_ref"`
	RuntimeEquivalenceRetryPlanRef     string                  `json:"runtime_equivalence_retry_plan_ref"`
	RollbackPlanRef                    string                  `json:"rollback_plan_ref"`
	RuntimeEquivalenceReentryRef       string                  `json:"runtime_equivalence_reentry_ref"`
	RuntimeEquivalenceBoundaryRef      string                  `json:"runtime_equivalence_boundary_ref"`
	LocalSubstrateSummaryRef           string                  `json:"local_substrate_summary_ref"`
	SourceMaterializerReadinessRef     string                  `json:"source_materializer_readiness_ref"`
	RuntimeMaterializationRef          string                  `json:"runtime_materialization_ref"`
	RuntimeEvidenceReviewRef           string                  `json:"runtime_evidence_review_ref"`
	SourceProvenanceReadinessRef       string                  `json:"source_provenance_readiness_ref"`
	RuntimeObservationExtractionRef    string                  `json:"runtime_observation_extraction_ref"`
	ExtractorRef                       string                  `json:"extractor_ref"`
	ExtractedObservationSetName        string                  `json:"extracted_observation_set_name"`
	RuntimeMaterializer                string                  `json:"runtime_materializer"`
	RuntimeSubstrate                   string                  `json:"runtime_substrate"`
	RequiredRuntimeObservations        []ObservationKind       `json:"required_runtime_observations"`
	UnsupportedDurableObservations     []UnsupportedCapability `json:"unsupported_durable_observations"`
	RemainingGaps                      []string                `json:"remaining_gaps"`
	HandoffStatus                      string                  `json:"handoff_status"`
	RuntimeFileBlobExtractionSatisfied bool                    `json:"runtime_file_blob_extraction_satisfied"`
	RuntimeEquivalenceRetryRequired    bool                    `json:"runtime_equivalence_retry_required"`
	RuntimeEquivalenceMayBeRetried     bool                    `json:"runtime_equivalence_may_be_retried"`
	RuntimeEquivalenceClaimed          bool                    `json:"runtime_equivalence_claimed"`
	DurableStateEquivalenceRequired    bool                    `json:"durable_state_equivalence_required"`
	StagingProofRequired               bool                    `json:"staging_proof_required"`
	PromotionProofRequired             bool                    `json:"promotion_proof_required"`
	PackagePublicationProofRequired    bool                    `json:"package_publication_proof_required"`
	RunAcceptanceProofRequired         bool                    `json:"run_acceptance_proof_required"`
	FullSubstrateProofRequired         bool                    `json:"full_substrate_proof_required"`
	VMLifecycleMutationAllowed         bool                    `json:"vm_lifecycle_mutation_allowed"`
	DurableComputerMutationAllowed     bool                    `json:"durable_computer_mutation_allowed"`
	DeployedRouteRegistrationAllowed   bool                    `json:"deployed_route_registration_allowed"`
	ProductionMutationAllowed          bool                    `json:"production_mutation_allowed"`
	PackagePublicationAllowed          bool                    `json:"package_publication_allowed"`
	PromotionAllowed                   bool                    `json:"promotion_allowed"`
	RunAcceptanceSynthesisAllowed      bool                    `json:"run_acceptance_synthesis_allowed"`
	NoOpaqueDataImageDependency        bool                    `json:"no_opaque_data_img_dependency"`
	NoVMLifecycleMutation              bool                    `json:"no_vm_lifecycle_mutation"`
	NoDurableComputerMutation          bool                    `json:"no_durable_computer_mutation"`
	NoDeployedRouteMutation            bool                    `json:"no_deployed_route_mutation"`
	NoProductionMutation               bool                    `json:"no_production_mutation"`
	NoPackagePublicationMutation       bool                    `json:"no_package_publication_mutation"`
	NoPromotionMutation                bool                    `json:"no_promotion_mutation"`
	NoRunAcceptanceMutation            bool                    `json:"no_run_acceptance_mutation"`
	RuntimeBehaviorChanged             bool                    `json:"runtime_behavior_changed"`
	DurableComputerStateMutated        bool                    `json:"durable_computer_state_mutated"`
	DeployedRouteRegistered            bool                    `json:"deployed_route_registered"`
	ProductionAuthTouched              bool                    `json:"production_auth_touched"`
	ProductionStateMutated             bool                    `json:"production_state_mutated"`
	PackagePublished                   bool                    `json:"package_published"`
	PromotionExecuted                  bool                    `json:"promotion_executed"`
	VMLifecycleTouched                 bool                    `json:"vm_lifecycle_touched"`
	FirecrackerBootClaimed             bool                    `json:"firecracker_boot_claimed"`
	StagingClaimed                     bool                    `json:"staging_claimed"`
	RunAcceptanceRecordTouched         bool                    `json:"run_acceptance_record_touched"`
	FullSubstrateClaimed               bool                    `json:"full_substrate_claimed"`
	CompletionClaimed                  bool                    `json:"completion_claimed"`
}

// BuildBaseRuntimeDurableGapExtractionHandoffContract admits an existing typed
// runtime file/blob extraction contract under the runtime-durable proof gap. It
// removes only the extraction prerequisite from the remaining gap set and leaves
// retry plus downstream proof obligations intact.
func BuildBaseRuntimeDurableGapExtractionHandoffContract(gap BaseRuntimeDurableProofGapContract, extraction BaseRuntimeFileBlobExtractionContract, evidence BaseRuntimeDurableGapExtractionHandoffEvidence) (BaseRuntimeDurableGapExtractionHandoffContract, error) {
	if err := validateBaseRuntimeDurableGapExtractionHandoffGap(gap); err != nil {
		return BaseRuntimeDurableGapExtractionHandoffContract{}, err
	}
	if err := validateBaseRuntimeDurableGapExtractionHandoffExtraction(gap, extraction); err != nil {
		return BaseRuntimeDurableGapExtractionHandoffContract{}, err
	}
	if err := validateBaseRuntimeDurableGapExtractionHandoffEvidence(evidence); err != nil {
		return BaseRuntimeDurableGapExtractionHandoffContract{}, err
	}

	return BaseRuntimeDurableGapExtractionHandoffContract{
		Kind:                               BaseRuntimeDurableGapExtractionHandoffContractKind,
		Version:                            gap.Version,
		Boundary:                           BaseRuntimeDurableGapExtractionHandoffBoundary,
		Scope:                              BaseRuntimeDurableGapExtractionHandoffScope,
		TypedArtifactProgramRef:            strings.TrimSpace(gap.TypedArtifactProgramRef),
		RuntimeDurableProofGapRef:          strings.TrimSpace(evidence.RuntimeDurableProofGapRef),
		RuntimeFileBlobExtractionRef:       strings.TrimSpace(evidence.RuntimeFileBlobExtractionRef),
		ExtractionHandoffReviewRef:         strings.TrimSpace(evidence.ExtractionHandoffReviewRef),
		RuntimeEquivalenceRetryPlanRef:     strings.TrimSpace(evidence.RuntimeEquivalenceRetryPlanRef),
		RollbackPlanRef:                    strings.TrimSpace(evidence.RollbackPlanRef),
		RuntimeEquivalenceReentryRef:       strings.TrimSpace(gap.RuntimeEquivalenceReentryRef),
		RuntimeEquivalenceBoundaryRef:      strings.TrimSpace(gap.RuntimeEquivalenceBoundaryRef),
		LocalSubstrateSummaryRef:           strings.TrimSpace(gap.LocalSubstrateSummaryRef),
		SourceMaterializerReadinessRef:     strings.TrimSpace(gap.SourceMaterializerReadinessRef),
		RuntimeMaterializationRef:          strings.TrimSpace(gap.RuntimeMaterializationRef),
		RuntimeEvidenceReviewRef:           strings.TrimSpace(gap.RuntimeEvidenceReviewRef),
		SourceProvenanceReadinessRef:       strings.TrimSpace(gap.SourceProvenanceReadinessRef),
		RuntimeObservationExtractionRef:    strings.TrimSpace(extraction.RuntimeObservationExtractionRef),
		ExtractorRef:                       strings.TrimSpace(extraction.ExtractorRef),
		ExtractedObservationSetName:        strings.TrimSpace(extraction.ExtractedObservationSetName),
		RuntimeMaterializer:                strings.TrimSpace(gap.RuntimeMaterializer),
		RuntimeSubstrate:                   strings.TrimSpace(gap.RuntimeSubstrate),
		RequiredRuntimeObservations:        canonicalObservationKinds(extraction.RequiredRuntimeObservations),
		UnsupportedDurableObservations:     canonicalUnsupportedCapabilities(gap.UnsupportedDurableObservations),
		RemainingGaps:                      canonicalBaseRuntimeDurableGapExtractionHandoffRemainingGaps(gap.RemainingGaps),
		HandoffStatus:                      BaseRuntimeDurableGapExtractionHandoffStatusReady,
		RuntimeFileBlobExtractionSatisfied: true,
		RuntimeEquivalenceRetryRequired:    true,
		RuntimeEquivalenceMayBeRetried:     true,
		RuntimeEquivalenceClaimed:          false,
		DurableStateEquivalenceRequired:    true,
		StagingProofRequired:               true,
		PromotionProofRequired:             true,
		PackagePublicationProofRequired:    true,
		RunAcceptanceProofRequired:         true,
		FullSubstrateProofRequired:         true,
		VMLifecycleMutationAllowed:         false,
		DurableComputerMutationAllowed:     false,
		DeployedRouteRegistrationAllowed:   false,
		ProductionMutationAllowed:          false,
		PackagePublicationAllowed:          false,
		PromotionAllowed:                   false,
		RunAcceptanceSynthesisAllowed:      false,
		NoOpaqueDataImageDependency:        true,
		NoVMLifecycleMutation:              true,
		NoDurableComputerMutation:          true,
		NoDeployedRouteMutation:            true,
		NoProductionMutation:               true,
		NoPackagePublicationMutation:       true,
		NoPromotionMutation:                true,
		NoRunAcceptanceMutation:            true,
	}, nil
}

func validateBaseRuntimeDurableGapExtractionHandoffGap(gap BaseRuntimeDurableProofGapContract) error {
	if gap.Kind != BaseRuntimeDurableProofGapContractKind {
		return fmt.Errorf("base runtime durable gap extraction handoff: gap kind is %q", gap.Kind)
	}
	if gap.Boundary != BaseRuntimeDurableProofGapBoundary {
		return fmt.Errorf("base runtime durable gap extraction handoff: gap boundary is %q", gap.Boundary)
	}
	if gap.Scope != BaseRuntimeDurableProofGapScope {
		return fmt.Errorf("base runtime durable gap extraction handoff: gap scope is %q", gap.Scope)
	}
	if !gap.Version.Valid() || !gap.Version.ArtifactProgramRef.Valid() || ArtifactProgramRef(gap.TypedArtifactProgramRef) != gap.Version.ArtifactProgramRef {
		return fmt.Errorf("base runtime durable gap extraction handoff: gap version or artifact ref is invalid")
	}
	if strings.TrimSpace(gap.RuntimeEquivalenceReentryRef) == "" || strings.TrimSpace(gap.RuntimeEquivalenceBoundaryRef) == "" || strings.TrimSpace(gap.LocalSubstrateSummaryRef) == "" || strings.TrimSpace(gap.SourceMaterializerReadinessRef) == "" || strings.TrimSpace(gap.RuntimeMaterializationRef) == "" || strings.TrimSpace(gap.RuntimeEvidenceReviewRef) == "" || strings.TrimSpace(gap.SourceProvenanceReadinessRef) == "" || strings.TrimSpace(gap.RuntimeMaterializer) == "" || strings.TrimSpace(gap.RuntimeSubstrate) == "" {
		return fmt.Errorf("base runtime durable gap extraction handoff: gap refs are required")
	}
	if gap.GapStatus != BaseRuntimeDurableProofGapStatusOpen || !gap.RuntimeEquivalenceNarrowed || gap.RuntimeEquivalenceClaimed || !gap.LocalFileBlobProofSummarized {
		return fmt.Errorf("base runtime durable gap extraction handoff: gap must remain open and narrowed")
	}
	if !gap.RuntimeFileBlobExtractionRequired || !gap.RuntimeEquivalenceRetryRequired || !gap.DurableStateEquivalenceRequired || !gap.StagingProofRequired || !gap.PromotionProofRequired || !gap.PackagePublicationProofRequired || !gap.RunAcceptanceProofRequired || !gap.FullSubstrateProofRequired {
		return fmt.Errorf("base runtime durable gap extraction handoff: gap must preserve extraction, retry, and downstream proof requirements")
	}
	if !baseRuntimeDurableProofGapHasRequiredGaps(gap.RemainingGaps) {
		return fmt.Errorf("base runtime durable gap extraction handoff: gap remaining obligations are incomplete")
	}
	if !observationKindsContain(gap.RuntimeRequiredObservations, ObservationVMStateManifest) || !baseSubstrateEquivalenceHasRequiredScope(gap.LocalRequiredObservations) {
		return fmt.Errorf("base runtime durable gap extraction handoff: gap observations are incomplete")
	}
	if !unsupportedCapabilityContains(gap.UnsupportedDurableObservations, ObservationFileManifest) || !unsupportedCapabilityContains(gap.UnsupportedDurableObservations, ObservationBlobSet) {
		return fmt.Errorf("base runtime durable gap extraction handoff: gap must preserve unsupported durable observations")
	}
	if gap.VMLifecycleMutationAllowed || gap.DurableComputerMutationAllowed || gap.DeployedRouteRegistrationAllowed || gap.ProductionMutationAllowed || gap.PackagePublicationAllowed || gap.PromotionAllowed || gap.RunAcceptanceSynthesisAllowed {
		return fmt.Errorf("base runtime durable gap extraction handoff: gap allows downstream authority")
	}
	if !gap.NoRuntimeMaterialization || !gap.NoDurableComputerMutation || !gap.NoDeployedRouteMutation || !gap.NoProductionMutation || !gap.NoPackagePublicationMutation || !gap.NoPromotionMutation || !gap.NoRunAcceptanceMutation {
		return fmt.Errorf("base runtime durable gap extraction handoff: gap must prove no mutation")
	}
	if gap.RuntimeBehaviorChanged || gap.DurableComputerStateMutated || gap.DeployedRouteRegistered || gap.ProductionAuthTouched || gap.ProductionStateMutated || gap.PackagePublished || gap.PromotionExecuted || gap.VMLifecycleTouched || gap.FirecrackerBootClaimed || gap.StagingClaimed || gap.RunAcceptanceRecordTouched || gap.FullSubstrateClaimed || gap.CompletionClaimed {
		return fmt.Errorf("base runtime durable gap extraction handoff: gap carries protected-surface or completion claims")
	}
	return nil
}

func validateBaseRuntimeDurableGapExtractionHandoffExtraction(gap BaseRuntimeDurableProofGapContract, extraction BaseRuntimeFileBlobExtractionContract) error {
	if extraction.Kind != BaseRuntimeFileBlobExtractionContractKind {
		return fmt.Errorf("base runtime durable gap extraction handoff: extraction kind is %q", extraction.Kind)
	}
	if extraction.Boundary != BaseRuntimeFileBlobExtractionBoundary {
		return fmt.Errorf("base runtime durable gap extraction handoff: extraction boundary is %q", extraction.Boundary)
	}
	if extraction.Scope != BaseRuntimeFileBlobExtractionScope {
		return fmt.Errorf("base runtime durable gap extraction handoff: extraction scope is %q", extraction.Scope)
	}
	if extraction.Version != gap.Version {
		return fmt.Errorf("base runtime durable gap extraction handoff: extraction version does not match gap")
	}
	if ArtifactProgramRef(extraction.TypedArtifactProgramRef) != gap.Version.ArtifactProgramRef {
		return fmt.Errorf("base runtime durable gap extraction handoff: extraction typed artifact-program ref does not match gap")
	}
	if strings.TrimSpace(extraction.RuntimeEquivalenceBoundaryRef) == "" || strings.TrimSpace(extraction.RuntimeObservationExtractionRef) == "" || strings.TrimSpace(extraction.ExtractorRef) == "" || strings.TrimSpace(extraction.SourceProvenanceReadinessRef) == "" || strings.TrimSpace(extraction.ExtractedObservationSetName) == "" {
		return fmt.Errorf("base runtime durable gap extraction handoff: extraction refs are required")
	}
	if strings.TrimSpace(extraction.RuntimeEquivalenceBoundaryRef) != strings.TrimSpace(gap.RuntimeEquivalenceBoundaryRef) || strings.TrimSpace(extraction.SourceProvenanceReadinessRef) != strings.TrimSpace(gap.SourceProvenanceReadinessRef) {
		return fmt.Errorf("base runtime durable gap extraction handoff: extraction does not match gap refs")
	}
	if !extraction.RuntimeFileBlobObservationsReady || !extraction.RuntimeEquivalenceMayBeRetried || extraction.RuntimeEquivalenceClaimed {
		return fmt.Errorf("base runtime durable gap extraction handoff: extraction must satisfy file/blob observations without claiming equivalence")
	}
	if !baseSubstrateEquivalenceHasRequiredScope(extraction.RequiredRuntimeObservations) || observationKindsContain(extraction.RequiredRuntimeObservations, ObservationVMStateManifest) {
		return fmt.Errorf("base runtime durable gap extraction handoff: extraction must require only typed file/blob runtime observations")
	}
	if !extraction.NoOpaqueDataImageDependency || !extraction.NoVMLifecycleMutation || !extraction.NoProductionMutation {
		return fmt.Errorf("base runtime durable gap extraction handoff: extraction must prove no opaque data.img, VM lifecycle, or production mutation")
	}
	if extraction.RuntimeBehaviorChanged || extraction.DeployedRouteRegistered || extraction.ProductionAuthTouched || extraction.StagingClaimed || extraction.PromotionClaimed || extraction.VMLifecycleTouched || extraction.FirecrackerBootClaimed || extraction.RunAcceptanceRecordTouched || extraction.PackagePublicationClaimed || extraction.FullSubstrateClaimed || extraction.CompletionClaimed {
		return fmt.Errorf("base runtime durable gap extraction handoff: extraction carries protected-surface or completion claims")
	}
	return nil
}

func validateBaseRuntimeDurableGapExtractionHandoffEvidence(evidence BaseRuntimeDurableGapExtractionHandoffEvidence) error {
	if strings.TrimSpace(evidence.RuntimeDurableProofGapRef) == "" || strings.TrimSpace(evidence.RuntimeFileBlobExtractionRef) == "" || strings.TrimSpace(evidence.ExtractionHandoffReviewRef) == "" || strings.TrimSpace(evidence.RuntimeEquivalenceRetryPlanRef) == "" || strings.TrimSpace(evidence.RollbackPlanRef) == "" {
		return fmt.Errorf("base runtime durable gap extraction handoff: evidence refs are required")
	}
	if !evidence.NoOpaqueDataImageDependency || !evidence.NoVMLifecycleMutation || !evidence.NoDurableComputerMutation || !evidence.NoDeployedRouteMutation || !evidence.NoProductionMutation || !evidence.NoPackagePublicationMutation || !evidence.NoPromotionMutation || !evidence.NoRunAcceptanceMutation {
		return fmt.Errorf("base runtime durable gap extraction handoff: evidence must prove no opaque data.img, VM lifecycle, durable-computer, route, production, package, promotion, or run-acceptance mutation")
	}
	if evidence.RuntimeBehaviorChanged || evidence.RuntimeEquivalenceClaimed || evidence.DurableComputerStateMutated || evidence.DeployedRouteRegistered || evidence.ProductionAuthTouched || evidence.ProductionStateMutated || evidence.PackagePublished || evidence.PromotionExecuted || evidence.VMLifecycleTouched || evidence.FirecrackerBootClaimed || evidence.StagingClaimed || evidence.RunAcceptanceRecordTouched || evidence.FullSubstrateClaimed || evidence.CompletionClaimed {
		return fmt.Errorf("base runtime durable gap extraction handoff: evidence carries runtime-equivalence, mutation, downstream, full-substrate, or completion claims")
	}
	return nil
}

func canonicalBaseRuntimeDurableGapExtractionHandoffRemainingGaps(gaps []string) []string {
	seen := make(map[string]struct{}, len(gaps))
	for _, gap := range gaps {
		seen[strings.TrimSpace(gap)] = struct{}{}
	}
	ordered := []string{
		BaseRuntimeDurableProofGapRuntimeEquivalenceRetry,
		BaseRuntimeDurableProofGapStagingProof,
		BaseRuntimeDurableProofGapPromotionProof,
		BaseRuntimeDurableProofGapPackagePublicationProof,
		BaseRuntimeDurableProofGapRunAcceptanceProof,
		BaseRuntimeDurableProofGapFullSubstrateProof,
	}
	out := make([]string, 0, len(ordered))
	for _, gap := range ordered {
		if _, ok := seen[gap]; ok {
			out = append(out, gap)
		}
	}
	return out
}
