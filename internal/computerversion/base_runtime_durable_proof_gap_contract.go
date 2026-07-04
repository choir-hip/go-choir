package computerversion

import (
	"fmt"
	"strings"
)

const BaseRuntimeDurableProofGapContractKind = "base_runtime_durable_proof_gap_contract"

const BaseRuntimeDurableProofGapBoundary = "runtime_durable_proof_gap_without_equivalence_success_or_downstream_mutation"

const BaseRuntimeDurableProofGapScope = "narrowed_runtime_reentry_plus_local_file_blob_summary_gap_only"

const BaseRuntimeDurableProofGapStatusOpen = "runtime_durable_gap_open_runtime_file_blob_retry_required"

const BaseRuntimeDurableProofGapRuntimeFileBlobExtraction = "runtime_file_blob_extraction_required"
const BaseRuntimeDurableProofGapRuntimeEquivalenceRetry = "runtime_equivalence_retry_required"
const BaseRuntimeDurableProofGapStagingProof = "staging_proof_required"
const BaseRuntimeDurableProofGapPromotionProof = "promotion_proof_required"
const BaseRuntimeDurableProofGapPackagePublicationProof = "package_publication_proof_required"
const BaseRuntimeDurableProofGapRunAcceptanceProof = "run_acceptance_proof_required"
const BaseRuntimeDurableProofGapFullSubstrateProof = "full_substrate_proof_required"

// BaseRuntimeDurableProofGapEvidence records refs for binding narrowed runtime
// equivalence reentry to the local file/blob substrate proof summary. It records
// the remaining proof gap only; it does not claim runtime equivalence, staging,
// promotion, run acceptance, full-substrate independence, or completion.
type BaseRuntimeDurableProofGapEvidence struct {
	RuntimeEquivalenceReentryRef string   `json:"runtime_equivalence_reentry_ref"`
	LocalSubstrateSummaryRef     string   `json:"local_substrate_summary_ref"`
	GapReviewRef                 string   `json:"gap_review_ref"`
	RuntimeFileBlobPlanRef       string   `json:"runtime_file_blob_plan_ref"`
	RuntimeEquivalenceRetryRef   string   `json:"runtime_equivalence_retry_ref"`
	RollbackPlanRef              string   `json:"rollback_plan_ref"`
	RemainingGaps                []string `json:"remaining_gaps"`
	NoRuntimeMaterialization     bool     `json:"no_runtime_materialization"`
	NoDurableComputerMutation    bool     `json:"no_durable_computer_mutation"`
	NoDeployedRouteMutation      bool     `json:"no_deployed_route_mutation"`
	NoProductionMutation         bool     `json:"no_production_mutation"`
	NoPackagePublicationMutation bool     `json:"no_package_publication_mutation"`
	NoPromotionMutation          bool     `json:"no_promotion_mutation"`
	NoRunAcceptanceMutation      bool     `json:"no_run_acceptance_mutation"`
	RuntimeBehaviorChanged       bool     `json:"runtime_behavior_changed"`
	RuntimeEquivalenceClaimed    bool     `json:"runtime_equivalence_claimed"`
	DurableComputerStateMutated  bool     `json:"durable_computer_state_mutated"`
	DeployedRouteRegistered      bool     `json:"deployed_route_registered"`
	ProductionAuthTouched        bool     `json:"production_auth_touched"`
	ProductionStateMutated       bool     `json:"production_state_mutated"`
	PackagePublished             bool     `json:"package_published"`
	PromotionExecuted            bool     `json:"promotion_executed"`
	VMLifecycleTouched           bool     `json:"vm_lifecycle_touched"`
	FirecrackerBootClaimed       bool     `json:"firecracker_boot_claimed"`
	StagingClaimed               bool     `json:"staging_claimed"`
	RunAcceptanceRecordTouched   bool     `json:"run_acceptance_record_touched"`
	FullSubstrateClaimed         bool     `json:"full_substrate_claimed"`
	CompletionClaimed            bool     `json:"completion_claimed"`
}

// BaseRuntimeDurableProofGapContract records that narrowed runtime-equivalence
// reentry and local file/blob proof are complementary but insufficient. Runtime
// file/blob extraction and retry evidence remain required before runtime
// substrate proof can be claimed.
type BaseRuntimeDurableProofGapContract struct {
	Kind                              string                  `json:"kind"`
	Version                           ComputerVersion         `json:"version"`
	Boundary                          string                  `json:"boundary"`
	Scope                             string                  `json:"scope"`
	TypedArtifactProgramRef           string                  `json:"typed_artifact_program_ref"`
	RuntimeEquivalenceReentryRef      string                  `json:"runtime_equivalence_reentry_ref"`
	RuntimeEquivalenceBoundaryRef     string                  `json:"runtime_equivalence_boundary_ref"`
	LocalSubstrateSummaryRef          string                  `json:"local_substrate_summary_ref"`
	GapReviewRef                      string                  `json:"gap_review_ref"`
	RuntimeFileBlobPlanRef            string                  `json:"runtime_file_blob_plan_ref"`
	RuntimeEquivalenceRetryRef        string                  `json:"runtime_equivalence_retry_ref"`
	RollbackPlanRef                   string                  `json:"rollback_plan_ref"`
	SourceMaterializerReadinessRef    string                  `json:"source_materializer_readiness_ref"`
	RuntimeMaterializationRef         string                  `json:"runtime_materialization_ref"`
	RuntimeEvidenceReviewRef          string                  `json:"runtime_evidence_review_ref"`
	SourceProvenanceReadinessRef      string                  `json:"source_provenance_readiness_ref"`
	RealizationEvidenceRef            string                  `json:"realization_evidence_ref"`
	RuntimeMaterializer               string                  `json:"runtime_materializer"`
	RuntimeSubstrate                  string                  `json:"runtime_substrate"`
	LocalCurrentMaterializer          string                  `json:"local_current_materializer"`
	LocalCurrentSubstrate             string                  `json:"local_current_substrate"`
	LocalProjectionMaterializer       string                  `json:"local_projection_materializer"`
	LocalProjectionSubstrate          string                  `json:"local_projection_substrate"`
	RuntimeRequiredObservations       []ObservationKind       `json:"runtime_required_observations"`
	LocalRequiredObservations         []ObservationKind       `json:"local_required_observations"`
	UnsupportedDurableObservations    []UnsupportedCapability `json:"unsupported_durable_observations"`
	RemainingGaps                     []string                `json:"remaining_gaps"`
	GapStatus                         string                  `json:"gap_status"`
	RuntimeEquivalenceNarrowed        bool                    `json:"runtime_equivalence_narrowed"`
	RuntimeEquivalenceClaimed         bool                    `json:"runtime_equivalence_claimed"`
	LocalFileBlobProofSummarized      bool                    `json:"local_file_blob_proof_summarized"`
	RuntimeFileBlobExtractionRequired bool                    `json:"runtime_file_blob_extraction_required"`
	RuntimeEquivalenceRetryRequired   bool                    `json:"runtime_equivalence_retry_required"`
	DurableStateEquivalenceRequired   bool                    `json:"durable_state_equivalence_required"`
	StagingProofRequired              bool                    `json:"staging_proof_required"`
	PromotionProofRequired            bool                    `json:"promotion_proof_required"`
	PackagePublicationProofRequired   bool                    `json:"package_publication_proof_required"`
	RunAcceptanceProofRequired        bool                    `json:"run_acceptance_proof_required"`
	FullSubstrateProofRequired        bool                    `json:"full_substrate_proof_required"`
	VMLifecycleMutationAllowed        bool                    `json:"vm_lifecycle_mutation_allowed"`
	DurableComputerMutationAllowed    bool                    `json:"durable_computer_mutation_allowed"`
	DeployedRouteRegistrationAllowed  bool                    `json:"deployed_route_registration_allowed"`
	ProductionMutationAllowed         bool                    `json:"production_mutation_allowed"`
	PackagePublicationAllowed         bool                    `json:"package_publication_allowed"`
	PromotionAllowed                  bool                    `json:"promotion_allowed"`
	RunAcceptanceSynthesisAllowed     bool                    `json:"run_acceptance_synthesis_allowed"`
	NoRuntimeMaterialization          bool                    `json:"no_runtime_materialization"`
	NoDurableComputerMutation         bool                    `json:"no_durable_computer_mutation"`
	NoDeployedRouteMutation           bool                    `json:"no_deployed_route_mutation"`
	NoProductionMutation              bool                    `json:"no_production_mutation"`
	NoPackagePublicationMutation      bool                    `json:"no_package_publication_mutation"`
	NoPromotionMutation               bool                    `json:"no_promotion_mutation"`
	NoRunAcceptanceMutation           bool                    `json:"no_run_acceptance_mutation"`
	RuntimeBehaviorChanged            bool                    `json:"runtime_behavior_changed"`
	DurableComputerStateMutated       bool                    `json:"durable_computer_state_mutated"`
	DeployedRouteRegistered           bool                    `json:"deployed_route_registered"`
	ProductionAuthTouched             bool                    `json:"production_auth_touched"`
	ProductionStateMutated            bool                    `json:"production_state_mutated"`
	PackagePublished                  bool                    `json:"package_published"`
	PromotionExecuted                 bool                    `json:"promotion_executed"`
	VMLifecycleTouched                bool                    `json:"vm_lifecycle_touched"`
	FirecrackerBootClaimed            bool                    `json:"firecracker_boot_claimed"`
	StagingClaimed                    bool                    `json:"staging_claimed"`
	RunAcceptanceRecordTouched        bool                    `json:"run_acceptance_record_touched"`
	FullSubstrateClaimed              bool                    `json:"full_substrate_claimed"`
	CompletionClaimed                 bool                    `json:"completion_claimed"`
}

// BuildBaseRuntimeDurableProofGapContract consumes narrowed runtime reentry and
// local file/blob proof, preserving the exact runtime-durable gap that remains.
func BuildBaseRuntimeDurableProofGapContract(reentry BaseRuntimeEquivalenceReentryContract, summary BaseLocalSubstrateProofSummaryContract, evidence BaseRuntimeDurableProofGapEvidence) (BaseRuntimeDurableProofGapContract, error) {
	if err := validateBaseRuntimeDurableProofGapReentry(reentry); err != nil {
		return BaseRuntimeDurableProofGapContract{}, err
	}
	if err := validateBaseRuntimeDurableProofGapSummary(reentry, summary); err != nil {
		return BaseRuntimeDurableProofGapContract{}, err
	}
	if err := validateBaseRuntimeDurableProofGapEvidence(evidence); err != nil {
		return BaseRuntimeDurableProofGapContract{}, err
	}
	gaps := canonicalBaseRuntimeDurableProofGapGaps(evidence.RemainingGaps)

	return BaseRuntimeDurableProofGapContract{
		Kind:                              BaseRuntimeDurableProofGapContractKind,
		Version:                           reentry.Version,
		Boundary:                          BaseRuntimeDurableProofGapBoundary,
		Scope:                             BaseRuntimeDurableProofGapScope,
		TypedArtifactProgramRef:           strings.TrimSpace(reentry.TypedArtifactProgramRef),
		RuntimeEquivalenceReentryRef:      strings.TrimSpace(evidence.RuntimeEquivalenceReentryRef),
		RuntimeEquivalenceBoundaryRef:     strings.TrimSpace(reentry.RuntimeEquivalenceBoundaryRef),
		LocalSubstrateSummaryRef:          strings.TrimSpace(evidence.LocalSubstrateSummaryRef),
		GapReviewRef:                      strings.TrimSpace(evidence.GapReviewRef),
		RuntimeFileBlobPlanRef:            strings.TrimSpace(evidence.RuntimeFileBlobPlanRef),
		RuntimeEquivalenceRetryRef:        strings.TrimSpace(evidence.RuntimeEquivalenceRetryRef),
		RollbackPlanRef:                   strings.TrimSpace(evidence.RollbackPlanRef),
		SourceMaterializerReadinessRef:    strings.TrimSpace(reentry.SourceMaterializerReadinessRef),
		RuntimeMaterializationRef:         strings.TrimSpace(reentry.RuntimeMaterializationRef),
		RuntimeEvidenceReviewRef:          strings.TrimSpace(reentry.RuntimeEvidenceReviewRef),
		SourceProvenanceReadinessRef:      strings.TrimSpace(reentry.SourceProvenanceReadinessRef),
		RealizationEvidenceRef:            strings.TrimSpace(reentry.RealizationEvidenceRef),
		RuntimeMaterializer:               strings.TrimSpace(reentry.Materializer),
		RuntimeSubstrate:                  strings.TrimSpace(reentry.Substrate),
		LocalCurrentMaterializer:          strings.TrimSpace(summary.CurrentMaterializer),
		LocalCurrentSubstrate:             strings.TrimSpace(summary.CurrentSubstrate),
		LocalProjectionMaterializer:       strings.TrimSpace(summary.ProjectionMaterializer),
		LocalProjectionSubstrate:          strings.TrimSpace(summary.ProjectionSubstrate),
		RuntimeRequiredObservations:       canonicalObservationKinds(reentry.RuntimeRequiredObservations),
		LocalRequiredObservations:         canonicalObservationKinds(summary.RequiredObservations),
		UnsupportedDurableObservations:    canonicalUnsupportedCapabilities(reentry.UnsupportedDurableObservations),
		RemainingGaps:                     gaps,
		GapStatus:                         BaseRuntimeDurableProofGapStatusOpen,
		RuntimeEquivalenceNarrowed:        true,
		RuntimeEquivalenceClaimed:         false,
		LocalFileBlobProofSummarized:      true,
		RuntimeFileBlobExtractionRequired: true,
		RuntimeEquivalenceRetryRequired:   true,
		DurableStateEquivalenceRequired:   true,
		StagingProofRequired:              true,
		PromotionProofRequired:            true,
		PackagePublicationProofRequired:   true,
		RunAcceptanceProofRequired:        true,
		FullSubstrateProofRequired:        true,
		VMLifecycleMutationAllowed:        false,
		DurableComputerMutationAllowed:    false,
		DeployedRouteRegistrationAllowed:  false,
		ProductionMutationAllowed:         false,
		PackagePublicationAllowed:         false,
		PromotionAllowed:                  false,
		RunAcceptanceSynthesisAllowed:     false,
		NoRuntimeMaterialization:          true,
		NoDurableComputerMutation:         true,
		NoDeployedRouteMutation:           true,
		NoProductionMutation:              true,
		NoPackagePublicationMutation:      true,
		NoPromotionMutation:               true,
		NoRunAcceptanceMutation:           true,
	}, nil
}

func validateBaseRuntimeDurableProofGapReentry(reentry BaseRuntimeEquivalenceReentryContract) error {
	if reentry.Kind != BaseRuntimeEquivalenceReentryContractKind {
		return fmt.Errorf("base runtime durable proof gap: reentry kind is %q", reentry.Kind)
	}
	if reentry.Boundary != BaseRuntimeEquivalenceReentryBoundary {
		return fmt.Errorf("base runtime durable proof gap: reentry boundary is %q", reentry.Boundary)
	}
	if reentry.Scope != BaseRuntimeEquivalenceReentryScope {
		return fmt.Errorf("base runtime durable proof gap: reentry scope is %q", reentry.Scope)
	}
	if !reentry.Version.Valid() || !reentry.Version.ArtifactProgramRef.Valid() || ArtifactProgramRef(reentry.TypedArtifactProgramRef) != reentry.Version.ArtifactProgramRef {
		return fmt.Errorf("base runtime durable proof gap: reentry version or artifact ref is invalid")
	}
	if strings.TrimSpace(reentry.RuntimeMaterializationBridgeRef) == "" || strings.TrimSpace(reentry.RuntimeEquivalenceBoundaryRef) == "" || strings.TrimSpace(reentry.SourceMaterializerReadinessRef) == "" || strings.TrimSpace(reentry.RuntimeMaterializationRef) == "" || strings.TrimSpace(reentry.RuntimeEvidenceReviewRef) == "" || strings.TrimSpace(reentry.SourceProvenanceReadinessRef) == "" || strings.TrimSpace(reentry.RealizationEvidenceRef) == "" || strings.TrimSpace(reentry.Materializer) == "" || strings.TrimSpace(reentry.Substrate) == "" {
		return fmt.Errorf("base runtime durable proof gap: reentry refs are required")
	}
	if reentry.ReentryStatus != BaseRuntimeEquivalenceReentryStatusNarrowed || !reentry.RuntimeEvidenceAccepted || !reentry.RuntimeEquivalenceNarrowed || reentry.RuntimeEquivalenceClaimed {
		return fmt.Errorf("base runtime durable proof gap: reentry must remain narrowed")
	}
	if !baseSubstrateEquivalenceHasRequiredScope(reentry.SourceRequiredObservations) || !observationKindsContain(reentry.RuntimeRequiredObservations, ObservationVMStateManifest) {
		return fmt.Errorf("base runtime durable proof gap: reentry observations are incomplete")
	}
	if !unsupportedCapabilityContains(reentry.UnsupportedDurableObservations, ObservationFileManifest) || !unsupportedCapabilityContains(reentry.UnsupportedDurableObservations, ObservationBlobSet) {
		return fmt.Errorf("base runtime durable proof gap: reentry must retain unsupported durable observations")
	}
	if !reentry.DurableStateEquivalenceRequired || !reentry.StagingProofRequired || !reentry.PromotionProofRequired || !reentry.PackagePublicationRequired || !reentry.RunAcceptanceProofRequired || !reentry.FullSubstrateProofRequired {
		return fmt.Errorf("base runtime durable proof gap: reentry must preserve downstream proof requirements")
	}
	if reentry.VMLifecycleMutationAllowed || reentry.DurableComputerMutationAllowed || reentry.DeployedRouteRegistrationAllowed || reentry.ProductionMutationAllowed || reentry.PackagePublicationAllowed || reentry.PromotionAllowed || reentry.RunAcceptanceSynthesisAllowed {
		return fmt.Errorf("base runtime durable proof gap: reentry allows downstream execution")
	}
	if !reentry.NoVMLifecycleMutation || !reentry.NoDurableComputerMutation || !reentry.NoDeployedRouteMutation || !reentry.NoProductionMutation || !reentry.NoPackagePublicationMutation || !reentry.NoPromotionMutation || !reentry.NoRunAcceptanceMutation {
		return fmt.Errorf("base runtime durable proof gap: reentry must prove no mutation")
	}
	if reentry.RuntimeBehaviorChanged || reentry.DurableComputerStateMutated || reentry.DeployedRouteRegistered || reentry.ProductionAuthTouched || reentry.ProductionStateMutated || reentry.PackagePublished || reentry.PromotionExecuted || reentry.VMLifecycleTouched || reentry.FirecrackerBootClaimed || reentry.RunAcceptanceRecordTouched || reentry.FullSubstrateClaimed || reentry.CompletionClaimed {
		return fmt.Errorf("base runtime durable proof gap: reentry carries protected-surface or completion claims")
	}
	return nil
}

func validateBaseRuntimeDurableProofGapSummary(reentry BaseRuntimeEquivalenceReentryContract, summary BaseLocalSubstrateProofSummaryContract) error {
	if summary.Kind != BaseLocalSubstrateProofSummaryContractKind {
		return fmt.Errorf("base runtime durable proof gap: summary kind is %q", summary.Kind)
	}
	if summary.Boundary != BaseLocalSubstrateProofSummaryBoundary {
		return fmt.Errorf("base runtime durable proof gap: summary boundary is %q", summary.Boundary)
	}
	if summary.Scope != BaseLocalSubstrateProofSummaryScope {
		return fmt.Errorf("base runtime durable proof gap: summary scope is %q", summary.Scope)
	}
	if summary.Version != reentry.Version {
		return fmt.Errorf("base runtime durable proof gap: summary version does not match reentry")
	}
	if !summary.Version.ArtifactProgramRef.Valid() || ArtifactProgramRef(reentry.TypedArtifactProgramRef) != summary.Version.ArtifactProgramRef {
		return fmt.Errorf("base runtime durable proof gap: summary artifact ref does not match reentry")
	}
	if summary.ClaimScope != BaseSubstrateEquivalenceClaimScope || summary.SubstrateEquivalenceStatus != EquivalenceEquivalent || !summary.ReentryAllowed || !summary.LocalFileBlobProofSummarized {
		return fmt.Errorf("base runtime durable proof gap: summary must prove only local file/blob substrate equivalence")
	}
	if strings.TrimSpace(summary.SubstrateEquivalenceContractRef) == "" || strings.TrimSpace(summary.ReentryReadinessContractRef) == "" || strings.TrimSpace(summary.EquivalenceEvidenceSetRef) == "" || strings.TrimSpace(summary.SummaryRef) == "" || strings.TrimSpace(summary.CurrentMaterializer) == "" || strings.TrimSpace(summary.CurrentSubstrate) == "" || strings.TrimSpace(summary.ProjectionMaterializer) == "" || strings.TrimSpace(summary.ProjectionSubstrate) == "" {
		return fmt.Errorf("base runtime durable proof gap: summary refs are required")
	}
	if !baseSubstrateEquivalenceHasRequiredScope(summary.RequiredObservations) {
		return fmt.Errorf("base runtime durable proof gap: summary must include file_manifest and blob_set")
	}
	if !summary.RuntimeSubstrateProofRequired || !summary.StagingProofRequired || !summary.PromotionProofRequired || !baseLocalSubstrateProofSummaryHasRequiredGaps(summary.RemainingGaps) {
		return fmt.Errorf("base runtime durable proof gap: summary must preserve runtime, staging, and promotion gaps")
	}
	if !summary.NoRuntimeMaterialization || !summary.NoOpaqueDataImageDependency || !summary.NoMutation {
		return fmt.Errorf("base runtime durable proof gap: summary must be local no-runtime no-mutation evidence")
	}
	if summary.RuntimeBehaviorChanged || summary.DeployedRouteRegistered || summary.ProductionAuthTouched || summary.StagingClaimed || summary.PromotionClaimed || summary.VMLifecycleTouched || summary.FirecrackerBootClaimed || summary.RunAcceptanceRecordTouched || summary.FullSubstrateIndependenceClaim || summary.PackagePublicationClaimed || summary.CompletionClaimed {
		return fmt.Errorf("base runtime durable proof gap: summary carries protected-surface or completion claims")
	}
	return nil
}

func validateBaseRuntimeDurableProofGapEvidence(evidence BaseRuntimeDurableProofGapEvidence) error {
	if strings.TrimSpace(evidence.RuntimeEquivalenceReentryRef) == "" || strings.TrimSpace(evidence.LocalSubstrateSummaryRef) == "" || strings.TrimSpace(evidence.GapReviewRef) == "" || strings.TrimSpace(evidence.RuntimeFileBlobPlanRef) == "" || strings.TrimSpace(evidence.RuntimeEquivalenceRetryRef) == "" || strings.TrimSpace(evidence.RollbackPlanRef) == "" {
		return fmt.Errorf("base runtime durable proof gap: evidence refs are required")
	}
	if !baseRuntimeDurableProofGapHasRequiredGaps(evidence.RemainingGaps) {
		return fmt.Errorf("base runtime durable proof gap: remaining gaps must require runtime file/blob extraction, runtime equivalence retry, staging, promotion, package, run acceptance, and full-substrate proof")
	}
	if !evidence.NoRuntimeMaterialization || !evidence.NoDurableComputerMutation || !evidence.NoDeployedRouteMutation || !evidence.NoProductionMutation || !evidence.NoPackagePublicationMutation || !evidence.NoPromotionMutation || !evidence.NoRunAcceptanceMutation {
		return fmt.Errorf("base runtime durable proof gap: evidence must prove no runtime, durable-computer, deployed-route, production, package, promotion, or run-acceptance mutation")
	}
	if evidence.RuntimeBehaviorChanged || evidence.RuntimeEquivalenceClaimed || evidence.DurableComputerStateMutated || evidence.DeployedRouteRegistered || evidence.ProductionAuthTouched || evidence.ProductionStateMutated || evidence.PackagePublished || evidence.PromotionExecuted || evidence.VMLifecycleTouched || evidence.FirecrackerBootClaimed || evidence.StagingClaimed || evidence.RunAcceptanceRecordTouched || evidence.FullSubstrateClaimed || evidence.CompletionClaimed {
		return fmt.Errorf("base runtime durable proof gap: evidence carries runtime-equivalence, mutation, downstream, full-substrate, or completion claims")
	}
	return nil
}

func baseRuntimeDurableProofGapHasRequiredGaps(gaps []string) bool {
	seen := make(map[string]struct{}, len(gaps))
	for _, gap := range gaps {
		seen[strings.TrimSpace(gap)] = struct{}{}
	}
	for _, required := range []string{
		BaseRuntimeDurableProofGapRuntimeFileBlobExtraction,
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

func canonicalBaseRuntimeDurableProofGapGaps(gaps []string) []string {
	ordered := []string{
		BaseRuntimeDurableProofGapRuntimeFileBlobExtraction,
		BaseRuntimeDurableProofGapRuntimeEquivalenceRetry,
		BaseRuntimeDurableProofGapStagingProof,
		BaseRuntimeDurableProofGapPromotionProof,
		BaseRuntimeDurableProofGapPackagePublicationProof,
		BaseRuntimeDurableProofGapRunAcceptanceProof,
		BaseRuntimeDurableProofGapFullSubstrateProof,
	}
	seen := make(map[string]struct{}, len(gaps))
	for _, gap := range gaps {
		seen[strings.TrimSpace(gap)] = struct{}{}
	}
	out := make([]string, 0, len(ordered))
	for _, gap := range ordered {
		if _, ok := seen[gap]; ok {
			out = append(out, gap)
		}
	}
	return out
}
