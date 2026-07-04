package computerversion

import (
	"fmt"
	"strings"
)

const BasePackagePublicationReadinessContractKind = "base_package_publication_readiness_contract"

const BasePackagePublicationReadinessBoundary = "package_publication_readiness_without_publication_promotion_or_run_acceptance"

const BasePackagePublicationReadinessScope = "promotion_rollback_review_to_publication_prerequisites_only"

const BasePackagePublicationReadinessStatusReady = "ready_for_package_publication_review_not_published"

// BasePackagePublicationReadinessEvidence records publication prerequisites
// after promotion/rollback review. It does not publish a package, execute
// promotion, synthesize run acceptance, or claim completion.
type BasePackagePublicationReadinessEvidence struct {
	PromotionRollbackReviewRef   string `json:"promotion_rollback_review_ref"`
	PackageManifestRef           string `json:"package_manifest_ref"`
	PublicationPayloadRef        string `json:"publication_payload_ref"`
	PublicationTargetRef         string `json:"publication_target_ref"`
	PublicationPolicyRef         string `json:"publication_policy_ref"`
	PublicationDryRunPlanRef     string `json:"publication_dry_run_plan_ref"`
	RollbackPlanRef              string `json:"rollback_plan_ref"`
	NoPackagePublicationMutation bool   `json:"no_package_publication_mutation"`
	NoPromotionMutation          bool   `json:"no_promotion_mutation"`
	NoRunAcceptanceMutation      bool   `json:"no_run_acceptance_mutation"`
	NoProductionMutation         bool   `json:"no_production_mutation"`
	PackagePublished             bool   `json:"package_published"`
	PromotionExecuted            bool   `json:"promotion_executed"`
	RunAcceptanceRecordTouched   bool   `json:"run_acceptance_record_touched"`
	FullSubstrateClaimed         bool   `json:"full_substrate_claimed"`
	CompletionClaimed            bool   `json:"completion_claimed"`
}

// BasePackagePublicationReadinessContract packages publication prerequisites.
// It is not package publication, promotion execution, run acceptance, or
// completion authority.
type BasePackagePublicationReadinessContract struct {
	Kind                          string          `json:"kind"`
	Version                       ComputerVersion `json:"version"`
	Boundary                      string          `json:"boundary"`
	Scope                         string          `json:"scope"`
	TypedArtifactProgramRef       string          `json:"typed_artifact_program_ref"`
	PromotionRollbackReviewRef    string          `json:"promotion_rollback_review_ref"`
	PromotionPlanRef              string          `json:"promotion_plan_ref"`
	RollbackPlanRef               string          `json:"rollback_plan_ref"`
	PackageManifestRef            string          `json:"package_manifest_ref"`
	PublicationPayloadRef         string          `json:"publication_payload_ref"`
	PublicationTargetRef          string          `json:"publication_target_ref"`
	PublicationPolicyRef          string          `json:"publication_policy_ref"`
	PublicationDryRunPlanRef      string          `json:"publication_dry_run_plan_ref"`
	ReadinessStatus               string          `json:"readiness_status"`
	OwnerApproved                 bool            `json:"owner_approved"`
	PromotionRollbackReviewReady  bool            `json:"promotion_rollback_review_ready"`
	PackagePublicationReady       bool            `json:"package_publication_ready"`
	RunAcceptanceProofRequired    bool            `json:"run_acceptance_proof_required"`
	FullSubstrateProofRequired    bool            `json:"full_substrate_proof_required"`
	PackagePublicationAllowed     bool            `json:"package_publication_allowed"`
	PromotionAllowed              bool            `json:"promotion_allowed"`
	RunAcceptanceSynthesisAllowed bool            `json:"run_acceptance_synthesis_allowed"`
	NoPackagePublicationMutation  bool            `json:"no_package_publication_mutation"`
	NoPromotionMutation           bool            `json:"no_promotion_mutation"`
	NoRunAcceptanceMutation       bool            `json:"no_run_acceptance_mutation"`
	NoProductionMutation          bool            `json:"no_production_mutation"`
	PackagePublished              bool            `json:"package_published"`
	PromotionExecuted             bool            `json:"promotion_executed"`
	RunAcceptanceRecordTouched    bool            `json:"run_acceptance_record_touched"`
	FullSubstrateClaimed          bool            `json:"full_substrate_claimed"`
	CompletionClaimed             bool            `json:"completion_claimed"`
}

// BuildBasePackagePublicationReadinessContract records package-publication
// readiness after promotion/rollback review. It does not publish, promote,
// synthesize run acceptance, or claim completion.
func BuildBasePackagePublicationReadinessContract(review BasePromotionRollbackReviewContract, evidence BasePackagePublicationReadinessEvidence) (BasePackagePublicationReadinessContract, error) {
	if err := validateBasePackagePublicationReadinessReview(review); err != nil {
		return BasePackagePublicationReadinessContract{}, err
	}
	if err := validateBasePackagePublicationReadinessEvidence(evidence); err != nil {
		return BasePackagePublicationReadinessContract{}, err
	}

	return BasePackagePublicationReadinessContract{
		Kind:                          BasePackagePublicationReadinessContractKind,
		Version:                       review.Version,
		Boundary:                      BasePackagePublicationReadinessBoundary,
		Scope:                         BasePackagePublicationReadinessScope,
		TypedArtifactProgramRef:       string(review.Version.ArtifactProgramRef),
		PromotionRollbackReviewRef:    strings.TrimSpace(evidence.PromotionRollbackReviewRef),
		PromotionPlanRef:              strings.TrimSpace(review.PromotionPlanRef),
		RollbackPlanRef:               strings.TrimSpace(evidence.RollbackPlanRef),
		PackageManifestRef:            strings.TrimSpace(evidence.PackageManifestRef),
		PublicationPayloadRef:         strings.TrimSpace(evidence.PublicationPayloadRef),
		PublicationTargetRef:          strings.TrimSpace(evidence.PublicationTargetRef),
		PublicationPolicyRef:          strings.TrimSpace(evidence.PublicationPolicyRef),
		PublicationDryRunPlanRef:      strings.TrimSpace(evidence.PublicationDryRunPlanRef),
		ReadinessStatus:               BasePackagePublicationReadinessStatusReady,
		OwnerApproved:                 true,
		PromotionRollbackReviewReady:  true,
		PackagePublicationReady:       true,
		RunAcceptanceProofRequired:    true,
		FullSubstrateProofRequired:    true,
		PackagePublicationAllowed:     false,
		PromotionAllowed:              false,
		RunAcceptanceSynthesisAllowed: false,
		NoPackagePublicationMutation:  true,
		NoPromotionMutation:           true,
		NoRunAcceptanceMutation:       true,
		NoProductionMutation:          true,
	}, nil
}

func validateBasePackagePublicationReadinessReview(review BasePromotionRollbackReviewContract) error {
	if review.Kind != BasePromotionRollbackReviewContractKind {
		return fmt.Errorf("base package publication readiness: promotion/rollback review kind is %q", review.Kind)
	}
	if review.Boundary != BasePromotionRollbackReviewBoundary {
		return fmt.Errorf("base package publication readiness: promotion/rollback review boundary is %q", review.Boundary)
	}
	if review.Scope != BasePromotionRollbackReviewScope {
		return fmt.Errorf("base package publication readiness: promotion/rollback review scope is %q", review.Scope)
	}
	if !review.Version.Valid() {
		return fmt.Errorf("base package publication readiness: promotion/rollback review version is invalid")
	}
	if !review.Version.ArtifactProgramRef.Valid() || ArtifactProgramRef(review.TypedArtifactProgramRef) != review.Version.ArtifactProgramRef {
		return fmt.Errorf("base package publication readiness: promotion/rollback review typed artifact-program ref is invalid")
	}
	if strings.TrimSpace(review.PromotionPlanRef) == "" || strings.TrimSpace(review.RollbackPlanRef) == "" || strings.TrimSpace(review.PromotionRiskReviewRef) == "" || strings.TrimSpace(review.LedgerFreshnessCheckRef) == "" || strings.TrimSpace(review.RouteContinuityCheckRef) == "" || strings.TrimSpace(review.OperatorReviewPolicyRef) == "" {
		return fmt.Errorf("base package publication readiness: promotion/rollback review refs are required")
	}
	if review.ReviewStatus != BasePromotionRollbackReviewStatusReady || !review.VerifierContractSatisfied || !review.OwnerApproved || !review.PromotionRollbackReviewReady {
		return fmt.Errorf("base package publication readiness: promotion/rollback review must be ready")
	}
	if !review.PackagePublicationProofRequired || !review.RunAcceptanceProofRequired || !review.FullSubstrateProofRequired {
		return fmt.Errorf("base package publication readiness: promotion/rollback review must preserve downstream proof requirements")
	}
	if review.PromotionAllowed || review.PackagePublicationAllowed || review.RunAcceptanceSynthesisAllowed {
		return fmt.Errorf("base package publication readiness: promotion/rollback review allows downstream execution")
	}
	if !review.NoPromotionMutation || !review.NoPackagePublicationMutation || !review.NoRunAcceptanceMutation || !review.NoProductionMutation {
		return fmt.Errorf("base package publication readiness: promotion/rollback review must prove no promotion, package publication, run-acceptance, or production mutation")
	}
	if review.PromotionExecuted || review.PackagePublished || review.RunAcceptanceRecordTouched || review.FullSubstrateClaimed || review.CompletionClaimed {
		return fmt.Errorf("base package publication readiness: promotion/rollback review carries downstream execution or completion claims")
	}
	return nil
}

func validateBasePackagePublicationReadinessEvidence(evidence BasePackagePublicationReadinessEvidence) error {
	if strings.TrimSpace(evidence.PromotionRollbackReviewRef) == "" || strings.TrimSpace(evidence.PackageManifestRef) == "" || strings.TrimSpace(evidence.PublicationPayloadRef) == "" || strings.TrimSpace(evidence.PublicationTargetRef) == "" || strings.TrimSpace(evidence.PublicationPolicyRef) == "" || strings.TrimSpace(evidence.PublicationDryRunPlanRef) == "" || strings.TrimSpace(evidence.RollbackPlanRef) == "" {
		return fmt.Errorf("base package publication readiness: publication refs are required")
	}
	if !evidence.NoPackagePublicationMutation || !evidence.NoPromotionMutation || !evidence.NoRunAcceptanceMutation || !evidence.NoProductionMutation {
		return fmt.Errorf("base package publication readiness: evidence must prove no package publication, promotion, run-acceptance, or production mutation")
	}
	if evidence.PackagePublished || evidence.PromotionExecuted || evidence.RunAcceptanceRecordTouched || evidence.FullSubstrateClaimed || evidence.CompletionClaimed {
		return fmt.Errorf("base package publication readiness: evidence carries downstream execution or completion claims")
	}
	return nil
}
