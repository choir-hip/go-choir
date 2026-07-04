package computerversion

import (
	"fmt"
	"strings"
)

const BaseRuntimeDurableGapRetryHandoffContractKind = "base_runtime_durable_gap_retry_handoff_contract"

const BaseRuntimeDurableGapRetryHandoffBoundary = "runtime_durable_gap_retry_handoff_without_downstream_claim"

const BaseRuntimeDurableGapRetryHandoffScope = "runtime_durable_gap_extraction_handoff_to_scoped_runtime_equivalence_retry"

const BaseRuntimeDurableGapRetryHandoffStatusReady = "scoped_runtime_equivalence_retry_admitted_downstream_proofs_still_required"

// BaseRuntimeDurableGapRetryHandoffEvidence records refs for admitting an
// existing runtime-equivalence retry contract under the runtime-durable gap
// extraction handoff. It closes only the retry obligation; downstream authority
// remains outside this evidence class.
type BaseRuntimeDurableGapRetryHandoffEvidence struct {
	RuntimeDurableGapExtractionHandoffRef string `json:"runtime_durable_gap_extraction_handoff_ref"`
	RuntimeEquivalenceRetryRef            string `json:"runtime_equivalence_retry_ref"`
	RetryHandoffReviewRef                 string `json:"retry_handoff_review_ref"`
	DownstreamProofPlanRef                string `json:"downstream_proof_plan_ref"`
	RollbackPlanRef                       string `json:"rollback_plan_ref"`
	NoOpaqueDataImageDependency           bool   `json:"no_opaque_data_img_dependency"`
	NoVMLifecycleMutation                 bool   `json:"no_vm_lifecycle_mutation"`
	NoDurableComputerMutation             bool   `json:"no_durable_computer_mutation"`
	NoDeployedRouteMutation               bool   `json:"no_deployed_route_mutation"`
	NoProductionMutation                  bool   `json:"no_production_mutation"`
	NoPackagePublicationMutation          bool   `json:"no_package_publication_mutation"`
	NoPromotionMutation                   bool   `json:"no_promotion_mutation"`
	NoRunAcceptanceMutation               bool   `json:"no_run_acceptance_mutation"`
	RuntimeBehaviorChanged                bool   `json:"runtime_behavior_changed"`
	DurableComputerStateMutated           bool   `json:"durable_computer_state_mutated"`
	DeployedRouteRegistered               bool   `json:"deployed_route_registered"`
	ProductionAuthTouched                 bool   `json:"production_auth_touched"`
	ProductionStateMutated                bool   `json:"production_state_mutated"`
	PackagePublished                      bool   `json:"package_published"`
	PromotionExecuted                     bool   `json:"promotion_executed"`
	VMLifecycleTouched                    bool   `json:"vm_lifecycle_touched"`
	FirecrackerBootClaimed                bool   `json:"firecracker_boot_claimed"`
	StagingClaimed                        bool   `json:"staging_claimed"`
	RunAcceptanceRecordTouched            bool   `json:"run_acceptance_record_touched"`
	FullSubstrateClaimed                  bool   `json:"full_substrate_claimed"`
	CompletionClaimed                     bool   `json:"completion_claimed"`
}

// BaseRuntimeDurableGapRetryHandoffContract admits a scoped runtime equivalence
// retry after runtime file/blob extraction has been admitted under the durable
// proof gap. It preserves downstream proof obligations and does not claim full
// substrate independence or completion.
type BaseRuntimeDurableGapRetryHandoffContract struct {
	Kind                                    string            `json:"kind"`
	Version                                 ComputerVersion   `json:"version"`
	Boundary                                string            `json:"boundary"`
	Scope                                   string            `json:"scope"`
	TypedArtifactProgramRef                 string            `json:"typed_artifact_program_ref"`
	RuntimeDurableGapExtractionHandoffRef   string            `json:"runtime_durable_gap_extraction_handoff_ref"`
	RuntimeEquivalenceRetryRef              string            `json:"runtime_equivalence_retry_ref"`
	RetryHandoffReviewRef                   string            `json:"retry_handoff_review_ref"`
	DownstreamProofPlanRef                  string            `json:"downstream_proof_plan_ref"`
	RollbackPlanRef                         string            `json:"rollback_plan_ref"`
	RuntimeDurableProofGapRef               string            `json:"runtime_durable_proof_gap_ref"`
	RuntimeFileBlobExtractionRef            string            `json:"runtime_file_blob_extraction_ref"`
	RuntimeEquivalenceReentryRef            string            `json:"runtime_equivalence_reentry_ref"`
	RuntimeEquivalenceBoundaryRef           string            `json:"runtime_equivalence_boundary_ref"`
	LocalSubstrateSummaryRef                string            `json:"local_substrate_summary_ref"`
	SourceMaterializerReadinessRef          string            `json:"source_materializer_readiness_ref"`
	RuntimeMaterializationRef               string            `json:"runtime_materialization_ref"`
	RuntimeEvidenceReviewRef                string            `json:"runtime_evidence_review_ref"`
	SourceProvenanceReadinessRef            string            `json:"source_provenance_readiness_ref"`
	RuntimeObservationExtractionRef         string            `json:"runtime_observation_extraction_ref"`
	SourceObservationSetRef                 string            `json:"source_observation_set_ref"`
	SourceObservationSetName                string            `json:"source_observation_set_name"`
	RuntimeObservationSetName               string            `json:"runtime_observation_set_name"`
	RequiredObservations                    []ObservationKind `json:"required_observations"`
	RemainingGaps                           []string          `json:"remaining_gaps"`
	HandoffStatus                           string            `json:"handoff_status"`
	RuntimeFileBlobExtractionSatisfied      bool              `json:"runtime_file_blob_extraction_satisfied"`
	RuntimeEquivalenceRetrySatisfied        bool              `json:"runtime_equivalence_retry_satisfied"`
	ScopedRuntimeFileBlobEquivalenceClaimed bool              `json:"scoped_runtime_file_blob_equivalence_claimed"`
	StagingProofRequired                    bool              `json:"staging_proof_required"`
	PromotionProofRequired                  bool              `json:"promotion_proof_required"`
	PackagePublicationProofRequired         bool              `json:"package_publication_proof_required"`
	RunAcceptanceProofRequired              bool              `json:"run_acceptance_proof_required"`
	FullSubstrateProofRequired              bool              `json:"full_substrate_proof_required"`
	VMLifecycleMutationAllowed              bool              `json:"vm_lifecycle_mutation_allowed"`
	DurableComputerMutationAllowed          bool              `json:"durable_computer_mutation_allowed"`
	DeployedRouteRegistrationAllowed        bool              `json:"deployed_route_registration_allowed"`
	ProductionMutationAllowed               bool              `json:"production_mutation_allowed"`
	PackagePublicationAllowed               bool              `json:"package_publication_allowed"`
	PromotionAllowed                        bool              `json:"promotion_allowed"`
	RunAcceptanceSynthesisAllowed           bool              `json:"run_acceptance_synthesis_allowed"`
	NoOpaqueDataImageDependency             bool              `json:"no_opaque_data_img_dependency"`
	NoVMLifecycleMutation                   bool              `json:"no_vm_lifecycle_mutation"`
	NoDurableComputerMutation               bool              `json:"no_durable_computer_mutation"`
	NoDeployedRouteMutation                 bool              `json:"no_deployed_route_mutation"`
	NoProductionMutation                    bool              `json:"no_production_mutation"`
	NoPackagePublicationMutation            bool              `json:"no_package_publication_mutation"`
	NoPromotionMutation                     bool              `json:"no_promotion_mutation"`
	NoRunAcceptanceMutation                 bool              `json:"no_run_acceptance_mutation"`
	RuntimeBehaviorChanged                  bool              `json:"runtime_behavior_changed"`
	DurableComputerStateMutated             bool              `json:"durable_computer_state_mutated"`
	DeployedRouteRegistered                 bool              `json:"deployed_route_registered"`
	ProductionAuthTouched                   bool              `json:"production_auth_touched"`
	ProductionStateMutated                  bool              `json:"production_state_mutated"`
	PackagePublished                        bool              `json:"package_published"`
	PromotionExecuted                       bool              `json:"promotion_executed"`
	VMLifecycleTouched                      bool              `json:"vm_lifecycle_touched"`
	FirecrackerBootClaimed                  bool              `json:"firecracker_boot_claimed"`
	StagingClaimed                          bool              `json:"staging_claimed"`
	RunAcceptanceRecordTouched              bool              `json:"run_acceptance_record_touched"`
	FullSubstrateClaimed                    bool              `json:"full_substrate_claimed"`
	CompletionClaimed                       bool              `json:"completion_claimed"`
}

// BuildBaseRuntimeDurableGapRetryHandoffContract admits a scoped successful
// retry under the extraction handoff while preserving all downstream gates.
func BuildBaseRuntimeDurableGapRetryHandoffContract(handoff BaseRuntimeDurableGapExtractionHandoffContract, retry BaseRuntimeEquivalenceRetryContract, evidence BaseRuntimeDurableGapRetryHandoffEvidence) (BaseRuntimeDurableGapRetryHandoffContract, error) {
	if err := validateBaseRuntimeDurableGapRetryHandoffExtractionHandoff(handoff); err != nil {
		return BaseRuntimeDurableGapRetryHandoffContract{}, err
	}
	if err := validateBaseRuntimeDurableGapRetryHandoffRetry(handoff, retry); err != nil {
		return BaseRuntimeDurableGapRetryHandoffContract{}, err
	}
	if err := validateBaseRuntimeDurableGapRetryHandoffEvidence(evidence); err != nil {
		return BaseRuntimeDurableGapRetryHandoffContract{}, err
	}

	return BaseRuntimeDurableGapRetryHandoffContract{
		Kind:                                    BaseRuntimeDurableGapRetryHandoffContractKind,
		Version:                                 handoff.Version,
		Boundary:                                BaseRuntimeDurableGapRetryHandoffBoundary,
		Scope:                                   BaseRuntimeDurableGapRetryHandoffScope,
		TypedArtifactProgramRef:                 strings.TrimSpace(handoff.TypedArtifactProgramRef),
		RuntimeDurableGapExtractionHandoffRef:   strings.TrimSpace(evidence.RuntimeDurableGapExtractionHandoffRef),
		RuntimeEquivalenceRetryRef:              strings.TrimSpace(evidence.RuntimeEquivalenceRetryRef),
		RetryHandoffReviewRef:                   strings.TrimSpace(evidence.RetryHandoffReviewRef),
		DownstreamProofPlanRef:                  strings.TrimSpace(evidence.DownstreamProofPlanRef),
		RollbackPlanRef:                         strings.TrimSpace(evidence.RollbackPlanRef),
		RuntimeDurableProofGapRef:               strings.TrimSpace(handoff.RuntimeDurableProofGapRef),
		RuntimeFileBlobExtractionRef:            strings.TrimSpace(handoff.RuntimeFileBlobExtractionRef),
		RuntimeEquivalenceReentryRef:            strings.TrimSpace(handoff.RuntimeEquivalenceReentryRef),
		RuntimeEquivalenceBoundaryRef:           strings.TrimSpace(handoff.RuntimeEquivalenceBoundaryRef),
		LocalSubstrateSummaryRef:                strings.TrimSpace(handoff.LocalSubstrateSummaryRef),
		SourceMaterializerReadinessRef:          strings.TrimSpace(handoff.SourceMaterializerReadinessRef),
		RuntimeMaterializationRef:               strings.TrimSpace(handoff.RuntimeMaterializationRef),
		RuntimeEvidenceReviewRef:                strings.TrimSpace(handoff.RuntimeEvidenceReviewRef),
		SourceProvenanceReadinessRef:            strings.TrimSpace(handoff.SourceProvenanceReadinessRef),
		RuntimeObservationExtractionRef:         strings.TrimSpace(handoff.RuntimeObservationExtractionRef),
		SourceObservationSetRef:                 strings.TrimSpace(retry.SourceObservationSetRef),
		SourceObservationSetName:                strings.TrimSpace(retry.SourceObservationSetName),
		RuntimeObservationSetName:               strings.TrimSpace(retry.RuntimeObservationSetName),
		RequiredObservations:                    canonicalObservationKinds(retry.RequiredObservations),
		RemainingGaps:                           canonicalBaseRuntimeDurableGapRetryHandoffRemainingGaps(handoff.RemainingGaps),
		HandoffStatus:                           BaseRuntimeDurableGapRetryHandoffStatusReady,
		RuntimeFileBlobExtractionSatisfied:      true,
		RuntimeEquivalenceRetrySatisfied:        true,
		ScopedRuntimeFileBlobEquivalenceClaimed: true,
		StagingProofRequired:                    true,
		PromotionProofRequired:                  true,
		PackagePublicationProofRequired:         true,
		RunAcceptanceProofRequired:              true,
		FullSubstrateProofRequired:              true,
		VMLifecycleMutationAllowed:              false,
		DurableComputerMutationAllowed:          false,
		DeployedRouteRegistrationAllowed:        false,
		ProductionMutationAllowed:               false,
		PackagePublicationAllowed:               false,
		PromotionAllowed:                        false,
		RunAcceptanceSynthesisAllowed:           false,
		NoOpaqueDataImageDependency:             true,
		NoVMLifecycleMutation:                   true,
		NoDurableComputerMutation:               true,
		NoDeployedRouteMutation:                 true,
		NoProductionMutation:                    true,
		NoPackagePublicationMutation:            true,
		NoPromotionMutation:                     true,
		NoRunAcceptanceMutation:                 true,
	}, nil
}

func validateBaseRuntimeDurableGapRetryHandoffExtractionHandoff(handoff BaseRuntimeDurableGapExtractionHandoffContract) error {
	if handoff.Kind != BaseRuntimeDurableGapExtractionHandoffContractKind {
		return fmt.Errorf("base runtime durable gap retry handoff: extraction handoff kind is %q", handoff.Kind)
	}
	if handoff.Boundary != BaseRuntimeDurableGapExtractionHandoffBoundary {
		return fmt.Errorf("base runtime durable gap retry handoff: extraction handoff boundary is %q", handoff.Boundary)
	}
	if handoff.Scope != BaseRuntimeDurableGapExtractionHandoffScope {
		return fmt.Errorf("base runtime durable gap retry handoff: extraction handoff scope is %q", handoff.Scope)
	}
	if !handoff.Version.Valid() || !handoff.Version.ArtifactProgramRef.Valid() || ArtifactProgramRef(handoff.TypedArtifactProgramRef) != handoff.Version.ArtifactProgramRef {
		return fmt.Errorf("base runtime durable gap retry handoff: extraction handoff version or artifact ref is invalid")
	}
	if strings.TrimSpace(handoff.RuntimeDurableProofGapRef) == "" || strings.TrimSpace(handoff.RuntimeFileBlobExtractionRef) == "" || strings.TrimSpace(handoff.RuntimeEquivalenceReentryRef) == "" || strings.TrimSpace(handoff.RuntimeEquivalenceBoundaryRef) == "" || strings.TrimSpace(handoff.LocalSubstrateSummaryRef) == "" || strings.TrimSpace(handoff.SourceProvenanceReadinessRef) == "" || strings.TrimSpace(handoff.RuntimeObservationExtractionRef) == "" {
		return fmt.Errorf("base runtime durable gap retry handoff: extraction handoff refs are required")
	}
	if handoff.HandoffStatus != BaseRuntimeDurableGapExtractionHandoffStatusReady || !handoff.RuntimeFileBlobExtractionSatisfied || !handoff.RuntimeEquivalenceRetryRequired || !handoff.RuntimeEquivalenceMayBeRetried || handoff.RuntimeEquivalenceClaimed {
		return fmt.Errorf("base runtime durable gap retry handoff: extraction handoff must satisfy extraction and leave retry open")
	}
	if !baseRuntimeDurableGapExtractionHandoffHasRetryAndDownstreamGaps(handoff.RemainingGaps) {
		return fmt.Errorf("base runtime durable gap retry handoff: extraction handoff remaining gaps are incomplete")
	}
	if !baseSubstrateEquivalenceHasRequiredScope(handoff.RequiredRuntimeObservations) || observationKindsContain(handoff.RequiredRuntimeObservations, ObservationVMStateManifest) {
		return fmt.Errorf("base runtime durable gap retry handoff: extraction handoff must require only file/blob observations")
	}
	if handoff.VMLifecycleMutationAllowed || handoff.DurableComputerMutationAllowed || handoff.DeployedRouteRegistrationAllowed || handoff.ProductionMutationAllowed || handoff.PackagePublicationAllowed || handoff.PromotionAllowed || handoff.RunAcceptanceSynthesisAllowed {
		return fmt.Errorf("base runtime durable gap retry handoff: extraction handoff allows downstream authority")
	}
	if !handoff.NoOpaqueDataImageDependency || !handoff.NoVMLifecycleMutation || !handoff.NoDurableComputerMutation || !handoff.NoDeployedRouteMutation || !handoff.NoProductionMutation || !handoff.NoPackagePublicationMutation || !handoff.NoPromotionMutation || !handoff.NoRunAcceptanceMutation {
		return fmt.Errorf("base runtime durable gap retry handoff: extraction handoff must prove no mutation")
	}
	if handoff.RuntimeBehaviorChanged || handoff.DurableComputerStateMutated || handoff.DeployedRouteRegistered || handoff.ProductionAuthTouched || handoff.ProductionStateMutated || handoff.PackagePublished || handoff.PromotionExecuted || handoff.VMLifecycleTouched || handoff.FirecrackerBootClaimed || handoff.StagingClaimed || handoff.RunAcceptanceRecordTouched || handoff.FullSubstrateClaimed || handoff.CompletionClaimed {
		return fmt.Errorf("base runtime durable gap retry handoff: extraction handoff carries protected-surface or completion claims")
	}
	return nil
}

func validateBaseRuntimeDurableGapRetryHandoffRetry(handoff BaseRuntimeDurableGapExtractionHandoffContract, retry BaseRuntimeEquivalenceRetryContract) error {
	if retry.Kind != BaseRuntimeEquivalenceRetryContractKind {
		return fmt.Errorf("base runtime durable gap retry handoff: retry kind is %q", retry.Kind)
	}
	if retry.Boundary != BaseRuntimeEquivalenceRetryBoundary {
		return fmt.Errorf("base runtime durable gap retry handoff: retry boundary is %q", retry.Boundary)
	}
	if retry.Scope != BaseRuntimeEquivalenceRetryScope {
		return fmt.Errorf("base runtime durable gap retry handoff: retry scope is %q", retry.Scope)
	}
	if retry.Version != handoff.Version {
		return fmt.Errorf("base runtime durable gap retry handoff: retry version does not match extraction handoff")
	}
	if ArtifactProgramRef(retry.TypedArtifactProgramRef) != handoff.Version.ArtifactProgramRef {
		return fmt.Errorf("base runtime durable gap retry handoff: retry typed artifact-program ref does not match handoff")
	}
	if strings.TrimSpace(retry.SourceProvenanceReadinessRef) == "" || strings.TrimSpace(retry.SourceObservationSetRef) == "" || strings.TrimSpace(retry.RuntimeFileBlobExtractionRef) == "" || strings.TrimSpace(retry.RuntimeEquivalenceBoundaryRef) == "" || strings.TrimSpace(retry.RuntimeEquivalenceRetryRef) == "" || strings.TrimSpace(retry.SourceObservationSetName) == "" || strings.TrimSpace(retry.RuntimeObservationSetName) == "" {
		return fmt.Errorf("base runtime durable gap retry handoff: retry refs are required")
	}
	if strings.TrimSpace(retry.SourceProvenanceReadinessRef) != strings.TrimSpace(handoff.SourceProvenanceReadinessRef) || strings.TrimSpace(retry.RuntimeFileBlobExtractionRef) != strings.TrimSpace(handoff.RuntimeFileBlobExtractionRef) || strings.TrimSpace(retry.RuntimeEquivalenceBoundaryRef) != strings.TrimSpace(handoff.RuntimeEquivalenceBoundaryRef) {
		return fmt.Errorf("base runtime durable gap retry handoff: retry does not match extraction handoff refs")
	}
	if !baseSubstrateEquivalenceHasRequiredScope(retry.RequiredObservations) || observationKindsContain(retry.RequiredObservations, ObservationVMStateManifest) {
		return fmt.Errorf("base runtime durable gap retry handoff: retry must require only file/blob observations")
	}
	if retry.RuntimeEquivalenceStatus != EquivalenceEquivalent || !retry.RuntimeEquivalenceClaimed {
		return fmt.Errorf("base runtime durable gap retry handoff: retry must be scoped equivalent")
	}
	if !retry.StagingProofRequired || !retry.PromotionProofRequired || !retry.PackagePublicationProofRequired || !retry.RunAcceptanceProofRequired {
		return fmt.Errorf("base runtime durable gap retry handoff: retry must preserve downstream proof requirements")
	}
	if !retry.NoOpaqueDataImageDependency || !retry.NoVMLifecycleMutation || !retry.NoProductionMutation {
		return fmt.Errorf("base runtime durable gap retry handoff: retry must prove no opaque data.img, VM lifecycle, or production mutation")
	}
	if retry.RuntimeBehaviorChanged || retry.DeployedRouteRegistered || retry.ProductionAuthTouched || retry.StagingClaimed || retry.PromotionClaimed || retry.VMLifecycleTouched || retry.FirecrackerBootClaimed || retry.RunAcceptanceRecordTouched || retry.PackagePublicationClaimed || retry.FullSubstrateClaimed || retry.CompletionClaimed {
		return fmt.Errorf("base runtime durable gap retry handoff: retry carries protected-surface or completion claims")
	}
	return nil
}

func validateBaseRuntimeDurableGapRetryHandoffEvidence(evidence BaseRuntimeDurableGapRetryHandoffEvidence) error {
	if strings.TrimSpace(evidence.RuntimeDurableGapExtractionHandoffRef) == "" || strings.TrimSpace(evidence.RuntimeEquivalenceRetryRef) == "" || strings.TrimSpace(evidence.RetryHandoffReviewRef) == "" || strings.TrimSpace(evidence.DownstreamProofPlanRef) == "" || strings.TrimSpace(evidence.RollbackPlanRef) == "" {
		return fmt.Errorf("base runtime durable gap retry handoff: evidence refs are required")
	}
	if !evidence.NoOpaqueDataImageDependency || !evidence.NoVMLifecycleMutation || !evidence.NoDurableComputerMutation || !evidence.NoDeployedRouteMutation || !evidence.NoProductionMutation || !evidence.NoPackagePublicationMutation || !evidence.NoPromotionMutation || !evidence.NoRunAcceptanceMutation {
		return fmt.Errorf("base runtime durable gap retry handoff: evidence must prove no opaque data.img, VM lifecycle, durable-computer, route, production, package, promotion, or run-acceptance mutation")
	}
	if evidence.RuntimeBehaviorChanged || evidence.DurableComputerStateMutated || evidence.DeployedRouteRegistered || evidence.ProductionAuthTouched || evidence.ProductionStateMutated || evidence.PackagePublished || evidence.PromotionExecuted || evidence.VMLifecycleTouched || evidence.FirecrackerBootClaimed || evidence.StagingClaimed || evidence.RunAcceptanceRecordTouched || evidence.FullSubstrateClaimed || evidence.CompletionClaimed {
		return fmt.Errorf("base runtime durable gap retry handoff: evidence carries mutation, downstream, full-substrate, or completion claims")
	}
	return nil
}

func baseRuntimeDurableGapExtractionHandoffHasRetryAndDownstreamGaps(gaps []string) bool {
	seen := make(map[string]struct{}, len(gaps))
	for _, gap := range gaps {
		seen[strings.TrimSpace(gap)] = struct{}{}
	}
	for _, required := range []string{
		BaseRuntimeDurableProofGapRuntimeEquivalenceRetry,
		BaseRuntimeDurableProofGapStagingProof,
		BaseRuntimeDurableProofGapPromotionProof,
		BaseRuntimeDurableProofGapPackagePublicationProof,
		BaseRuntimeDurableProofGapRunAcceptanceProof,
		BaseRuntimeDurableProofGapFullSubstrateProof,
	} {
		if _, ok := seen[required]; !ok {
			return false
		}
	}
	return true
}

func canonicalBaseRuntimeDurableGapRetryHandoffRemainingGaps(gaps []string) []string {
	seen := make(map[string]struct{}, len(gaps))
	for _, gap := range gaps {
		seen[strings.TrimSpace(gap)] = struct{}{}
	}
	ordered := []string{
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
