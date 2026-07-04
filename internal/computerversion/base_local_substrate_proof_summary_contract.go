package computerversion

import (
	"fmt"
	"strings"
)

const BaseLocalSubstrateProofSummaryContractKind = "base_local_substrate_proof_summary_contract"

const BaseLocalSubstrateProofSummaryBoundary = "base_local_file_blob_substrate_proof_summary_without_runtime_or_completion_claim"

const BaseLocalSubstrateProofSummaryScope = "base_local_file_manifest_blob_set_substrate_equivalence_summary"

const BaseLocalSubstrateProofSummaryRemainingRuntimeSubstrateProof = "runtime_substrate_materialization_proof_required"
const BaseLocalSubstrateProofSummaryRemainingStagingProof = "staging_authenticated_user_computer_proof_required"
const BaseLocalSubstrateProofSummaryRemainingPromotionProof = "promotion_package_publication_proof_required"

// BaseLocalSubstrateProofSummaryEvidence records the proof refs that justify
// summarizing the local file-manifest/blob-set substrate-equivalence slice. It
// does not certify runtime materialization, staging behavior, promotion, VM
// lifecycle, package publication, or mission completion.
type BaseLocalSubstrateProofSummaryEvidence struct {
	SubstrateEquivalenceContractRef string   `json:"substrate_equivalence_contract_ref"`
	ReentryReadinessContractRef     string   `json:"reentry_readiness_contract_ref"`
	EquivalenceEvidenceSetRef       string   `json:"equivalence_evidence_set_ref"`
	SummaryRef                      string   `json:"summary_ref"`
	RemainingGaps                   []string `json:"remaining_gaps"`
	NoRuntimeMaterialization        bool     `json:"no_runtime_materialization"`
	NoOpaqueDataImageDependency     bool     `json:"no_opaque_data_img_dependency"`
	RuntimeBehaviorChanged          bool     `json:"runtime_behavior_changed"`
	DeployedRouteRegistered         bool     `json:"deployed_route_registered"`
	ProductionAuthTouched           bool     `json:"production_auth_touched"`
	StagingClaimed                  bool     `json:"staging_claimed"`
	PromotionClaimed                bool     `json:"promotion_claimed"`
	VMLifecycleTouched              bool     `json:"vm_lifecycle_touched"`
	FirecrackerBootClaimed          bool     `json:"firecracker_boot_claimed"`
	RunAcceptanceRecordTouched      bool     `json:"run_acceptance_record_touched"`
	FullSubstrateIndependenceClaim  bool     `json:"full_substrate_independence_claim"`
	PackagePublicationClaimed       bool     `json:"package_publication_claimed"`
	CompletionClaimed               bool     `json:"completion_claimed"`
	NoMutation                      bool     `json:"no_mutation"`
}

// BaseLocalSubstrateProofSummaryContract records that the local Base file/blob
// substrate-equivalence slice has a passing substrate contract, calibrated
// failure evidence, and reentry authorization. It deliberately preserves the
// remaining runtime/staging/promotion gaps instead of upgrading local evidence
// into full substrate independence or completion.
type BaseLocalSubstrateProofSummaryContract struct {
	Kind                            string            `json:"kind"`
	Version                         ComputerVersion   `json:"version"`
	Boundary                        string            `json:"boundary"`
	Scope                           string            `json:"scope"`
	ClaimScope                      string            `json:"claim_scope"`
	CurrentMaterializer             string            `json:"current_materializer"`
	CurrentSubstrate                string            `json:"current_substrate"`
	ProjectionMaterializer          string            `json:"projection_materializer"`
	ProjectionSubstrate             string            `json:"projection_substrate"`
	SubstrateEquivalenceStatus      EquivalenceStatus `json:"substrate_equivalence_status"`
	ReentryAllowed                  bool              `json:"reentry_allowed"`
	RequiredObservations            []ObservationKind `json:"required_observations"`
	SubstrateEquivalenceContractRef string            `json:"substrate_equivalence_contract_ref"`
	ReentryReadinessContractRef     string            `json:"reentry_readiness_contract_ref"`
	EquivalenceEvidenceSetRef       string            `json:"equivalence_evidence_set_ref"`
	SummaryRef                      string            `json:"summary_ref"`
	RemainingGaps                   []string          `json:"remaining_gaps"`
	LocalFileBlobProofSummarized    bool              `json:"local_file_blob_proof_summarized"`
	RuntimeSubstrateProofRequired   bool              `json:"runtime_substrate_proof_required"`
	StagingProofRequired            bool              `json:"staging_proof_required"`
	PromotionProofRequired          bool              `json:"promotion_proof_required"`
	NoRuntimeMaterialization        bool              `json:"no_runtime_materialization"`
	NoOpaqueDataImageDependency     bool              `json:"no_opaque_data_img_dependency"`
	RuntimeBehaviorChanged          bool              `json:"runtime_behavior_changed"`
	DeployedRouteRegistered         bool              `json:"deployed_route_registered"`
	ProductionAuthTouched           bool              `json:"production_auth_touched"`
	StagingClaimed                  bool              `json:"staging_claimed"`
	PromotionClaimed                bool              `json:"promotion_claimed"`
	VMLifecycleTouched              bool              `json:"vm_lifecycle_touched"`
	FirecrackerBootClaimed          bool              `json:"firecracker_boot_claimed"`
	RunAcceptanceRecordTouched      bool              `json:"run_acceptance_record_touched"`
	FullSubstrateIndependenceClaim  bool              `json:"full_substrate_independence_claim"`
	PackagePublicationClaimed       bool              `json:"package_publication_claimed"`
	CompletionClaimed               bool              `json:"completion_claimed"`
	NoMutation                      bool              `json:"no_mutation"`
}

// BuildBaseLocalSubstrateProofSummaryContract binds the strengthened substrate
// equivalence contract to the reentry-readiness contract. It summarizes only the
// local file/blob proof slice and refuses to turn it into runtime, staging,
// promotion, package-publication, full-substrate, or completion authority.
func BuildBaseLocalSubstrateProofSummaryContract(substrate BaseSubstrateEquivalenceContract, reentry BaseSubstrateReentryReadinessContract, evidence BaseLocalSubstrateProofSummaryEvidence) (BaseLocalSubstrateProofSummaryContract, error) {
	if err := validateBaseLocalSubstrateProofSummarySubstrate(substrate); err != nil {
		return BaseLocalSubstrateProofSummaryContract{}, err
	}
	if err := validateBaseLocalSubstrateProofSummaryReentry(reentry); err != nil {
		return BaseLocalSubstrateProofSummaryContract{}, err
	}
	if substrate.Version != reentry.Version {
		return BaseLocalSubstrateProofSummaryContract{}, fmt.Errorf("base local substrate proof summary: substrate and reentry contracts name different computer versions")
	}
	if substrate.ClaimScope != reentry.ClaimScope {
		return BaseLocalSubstrateProofSummaryContract{}, fmt.Errorf("base local substrate proof summary: substrate and reentry claim scopes differ")
	}
	if substrate.CurrentMaterializer != reentry.CurrentMaterializer || substrate.CurrentSubstrate != reentry.CurrentSubstrate || substrate.ProjectionMaterializer != reentry.ProjectionMaterializer || substrate.ProjectionSubstrate != reentry.ProjectionSubstrate {
		return BaseLocalSubstrateProofSummaryContract{}, fmt.Errorf("base local substrate proof summary: substrate and reentry materializer identities differ")
	}
	if err := validateBaseLocalSubstrateProofSummaryEvidence(evidence, reentry); err != nil {
		return BaseLocalSubstrateProofSummaryContract{}, err
	}
	remaining := canonicalBaseLocalSubstrateProofSummaryGaps(evidence.RemainingGaps)
	return BaseLocalSubstrateProofSummaryContract{
		Kind:                            BaseLocalSubstrateProofSummaryContractKind,
		Version:                         substrate.Version,
		Boundary:                        BaseLocalSubstrateProofSummaryBoundary,
		Scope:                           BaseLocalSubstrateProofSummaryScope,
		ClaimScope:                      substrate.ClaimScope,
		CurrentMaterializer:             substrate.CurrentMaterializer,
		CurrentSubstrate:                substrate.CurrentSubstrate,
		ProjectionMaterializer:          substrate.ProjectionMaterializer,
		ProjectionSubstrate:             substrate.ProjectionSubstrate,
		SubstrateEquivalenceStatus:      substrate.EquivalenceStatus,
		ReentryAllowed:                  reentry.LocalSubstrateReentryAllowed,
		RequiredObservations:            []ObservationKind{ObservationBlobSet, ObservationFileManifest},
		SubstrateEquivalenceContractRef: strings.TrimSpace(evidence.SubstrateEquivalenceContractRef),
		ReentryReadinessContractRef:     strings.TrimSpace(evidence.ReentryReadinessContractRef),
		EquivalenceEvidenceSetRef:       strings.TrimSpace(evidence.EquivalenceEvidenceSetRef),
		SummaryRef:                      strings.TrimSpace(evidence.SummaryRef),
		RemainingGaps:                   remaining,
		LocalFileBlobProofSummarized:    true,
		RuntimeSubstrateProofRequired:   true,
		StagingProofRequired:            true,
		PromotionProofRequired:          true,
		NoRuntimeMaterialization:        true,
		NoOpaqueDataImageDependency:     true,
		RuntimeBehaviorChanged:          false,
		DeployedRouteRegistered:         false,
		ProductionAuthTouched:           false,
		StagingClaimed:                  false,
		PromotionClaimed:                false,
		VMLifecycleTouched:              false,
		FirecrackerBootClaimed:          false,
		RunAcceptanceRecordTouched:      false,
		FullSubstrateIndependenceClaim:  false,
		PackagePublicationClaimed:       false,
		CompletionClaimed:               false,
		NoMutation:                      true,
	}, nil
}

func validateBaseLocalSubstrateProofSummarySubstrate(substrate BaseSubstrateEquivalenceContract) error {
	if substrate.Kind != BaseSubstrateEquivalenceContractKind {
		return fmt.Errorf("base local substrate proof summary: substrate contract kind is %q", substrate.Kind)
	}
	if substrate.Boundary != BaseSubstrateEquivalenceBoundary {
		return fmt.Errorf("base local substrate proof summary: substrate contract boundary is %q", substrate.Boundary)
	}
	if substrate.ClaimScope != BaseSubstrateEquivalenceClaimScope {
		return fmt.Errorf("base local substrate proof summary: substrate claim scope is %q", substrate.ClaimScope)
	}
	if substrate.EquivalenceStatus != EquivalenceEquivalent {
		return fmt.Errorf("base local substrate proof summary: substrate equivalence status is %q", substrate.EquivalenceStatus)
	}
	if !baseSubstrateEquivalenceHasRequiredScope(substrate.RequiredObservations) {
		return fmt.Errorf("base local substrate proof summary: substrate contract must include file_manifest and blob_set")
	}
	if strings.TrimSpace(substrate.CurrentMaterializer) == "" || strings.TrimSpace(substrate.CurrentSubstrate) == "" || strings.TrimSpace(substrate.ProjectionMaterializer) == "" || strings.TrimSpace(substrate.ProjectionSubstrate) == "" {
		return fmt.Errorf("base local substrate proof summary: substrate contract must name current and projection materializers")
	}
	if substrate.CurrentMaterializer == substrate.ProjectionMaterializer && substrate.CurrentSubstrate == substrate.ProjectionSubstrate {
		return fmt.Errorf("base local substrate proof summary: substrate contract must compare non-identical materializer or substrate")
	}
	if !substrate.NoRuntimeMaterialization || !substrate.NoOpaqueDataImageDependency || !substrate.NoMutation {
		return fmt.Errorf("base local substrate proof summary: substrate contract has unsafe proof flags")
	}
	if substrate.RuntimeBehaviorChanged || substrate.DeployedRouteRegistered || substrate.ProductionAuthTouched || substrate.StagingClaimed || substrate.PromotionClaimed || substrate.VMLifecycleTouched || substrate.FirecrackerBootClaimed || substrate.RunAcceptanceRecordTouched || substrate.FullSubstrateIndependenceClaim || substrate.CompletionClaimed {
		return fmt.Errorf("base local substrate proof summary: substrate contract carries protected-surface claims")
	}
	return nil
}

func validateBaseLocalSubstrateProofSummaryReentry(reentry BaseSubstrateReentryReadinessContract) error {
	if reentry.Kind != BaseSubstrateReentryReadinessContractKind {
		return fmt.Errorf("base local substrate proof summary: reentry contract kind is %q", reentry.Kind)
	}
	if reentry.Boundary != BaseSubstrateReentryReadinessBoundary {
		return fmt.Errorf("base local substrate proof summary: reentry contract boundary is %q", reentry.Boundary)
	}
	if reentry.Scope != BaseSubstrateReentryReadinessScope {
		return fmt.Errorf("base local substrate proof summary: reentry contract scope is %q", reentry.Scope)
	}
	if reentry.SubstrateEquivalenceStatus != EquivalenceEquivalent {
		return fmt.Errorf("base local substrate proof summary: reentry substrate status is %q", reentry.SubstrateEquivalenceStatus)
	}
	if !reentry.LocalSubstrateReentryAllowed {
		return fmt.Errorf("base local substrate proof summary: reentry contract does not allow local substrate reentry")
	}
	if !baseSubstrateEquivalenceHasRequiredScope(reentry.RequiredObservations) {
		return fmt.Errorf("base local substrate proof summary: reentry contract must include file_manifest and blob_set")
	}
	if strings.TrimSpace(reentry.SubstrateEquivalenceContractRef) == "" || strings.TrimSpace(reentry.EquivalenceEvidenceSetRef) == "" || strings.TrimSpace(reentry.NextProbeRef) == "" {
		return fmt.Errorf("base local substrate proof summary: reentry contract must carry proof refs")
	}
	if !reentry.NoRuntimeMaterialization || !reentry.NoOpaqueDataImageDependency || !reentry.NoMutation {
		return fmt.Errorf("base local substrate proof summary: reentry contract has unsafe proof flags")
	}
	if reentry.RuntimeBehaviorChanged || reentry.DeployedRouteRegistered || reentry.ProductionAuthTouched || reentry.StagingClaimed || reentry.PromotionClaimed || reentry.VMLifecycleTouched || reentry.FirecrackerBootClaimed || reentry.RunAcceptanceRecordTouched || reentry.FullSubstrateIndependenceClaim || reentry.CompletionClaimed {
		return fmt.Errorf("base local substrate proof summary: reentry contract carries protected-surface claims")
	}
	return nil
}

func validateBaseLocalSubstrateProofSummaryEvidence(evidence BaseLocalSubstrateProofSummaryEvidence, reentry BaseSubstrateReentryReadinessContract) error {
	if strings.TrimSpace(evidence.SubstrateEquivalenceContractRef) == "" {
		return fmt.Errorf("base local substrate proof summary: substrate equivalence contract ref is required")
	}
	if strings.TrimSpace(evidence.SubstrateEquivalenceContractRef) != reentry.SubstrateEquivalenceContractRef {
		return fmt.Errorf("base local substrate proof summary: substrate equivalence contract ref does not match reentry")
	}
	if strings.TrimSpace(evidence.ReentryReadinessContractRef) == "" {
		return fmt.Errorf("base local substrate proof summary: reentry readiness contract ref is required")
	}
	if strings.TrimSpace(evidence.EquivalenceEvidenceSetRef) == "" {
		return fmt.Errorf("base local substrate proof summary: equivalence evidence set ref is required")
	}
	if strings.TrimSpace(evidence.EquivalenceEvidenceSetRef) != reentry.EquivalenceEvidenceSetRef {
		return fmt.Errorf("base local substrate proof summary: equivalence evidence set ref does not match reentry")
	}
	if strings.TrimSpace(evidence.SummaryRef) == "" {
		return fmt.Errorf("base local substrate proof summary: summary ref is required")
	}
	if !baseLocalSubstrateProofSummaryHasRequiredGaps(evidence.RemainingGaps) {
		return fmt.Errorf("base local substrate proof summary: remaining gaps must preserve runtime, staging, and promotion proof requirements")
	}
	if !evidence.NoRuntimeMaterialization {
		return fmt.Errorf("base local substrate proof summary: evidence must prove no runtime materialization")
	}
	if !evidence.NoOpaqueDataImageDependency {
		return fmt.Errorf("base local substrate proof summary: evidence must prove no opaque data.img dependency")
	}
	if evidence.RuntimeBehaviorChanged || evidence.DeployedRouteRegistered || evidence.ProductionAuthTouched || evidence.StagingClaimed || evidence.PromotionClaimed || evidence.VMLifecycleTouched || evidence.FirecrackerBootClaimed || evidence.RunAcceptanceRecordTouched || evidence.FullSubstrateIndependenceClaim || evidence.PackagePublicationClaimed || evidence.CompletionClaimed {
		return fmt.Errorf("base local substrate proof summary: evidence carries protected-surface or completion claims")
	}
	if !evidence.NoMutation {
		return fmt.Errorf("base local substrate proof summary: evidence must be no-mutation")
	}
	return nil
}

func baseLocalSubstrateProofSummaryHasRequiredGaps(gaps []string) bool {
	seen := map[string]struct{}{}
	for _, gap := range gaps {
		trimmed := strings.TrimSpace(gap)
		if trimmed != "" {
			seen[trimmed] = struct{}{}
		}
	}
	_, hasRuntime := seen[BaseLocalSubstrateProofSummaryRemainingRuntimeSubstrateProof]
	_, hasStaging := seen[BaseLocalSubstrateProofSummaryRemainingStagingProof]
	_, hasPromotion := seen[BaseLocalSubstrateProofSummaryRemainingPromotionProof]
	return hasRuntime && hasStaging && hasPromotion
}

func canonicalBaseLocalSubstrateProofSummaryGaps(gaps []string) []string {
	canonical := []string{
		BaseLocalSubstrateProofSummaryRemainingRuntimeSubstrateProof,
		BaseLocalSubstrateProofSummaryRemainingStagingProof,
		BaseLocalSubstrateProofSummaryRemainingPromotionProof,
	}
	seen := map[string]struct{}{
		BaseLocalSubstrateProofSummaryRemainingRuntimeSubstrateProof: {},
		BaseLocalSubstrateProofSummaryRemainingStagingProof:          {},
		BaseLocalSubstrateProofSummaryRemainingPromotionProof:        {},
	}
	for _, gap := range gaps {
		trimmed := strings.TrimSpace(gap)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		canonical = append(canonical, trimmed)
	}
	return canonical
}
