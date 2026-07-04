package computerversion

import (
	"fmt"
	"strings"
)

const BasePromotionRollbackReviewContractKind = "base_promotion_rollback_review_contract"

const BasePromotionRollbackReviewBoundary = "promotion_rollback_review_without_promotion_publication_or_run_acceptance"

const BasePromotionRollbackReviewScope = "owner_approval_to_promotion_rollback_review_readiness_only"

const BasePromotionRollbackReviewStatusReady = "ready_for_promotion_rollback_review_not_executed"

// BasePromotionRollbackReviewEvidence records the promotion and rollback review
// prerequisites after owner approval. It does not execute promotion, publish a
// package, synthesize run acceptance, or claim completion.
type BasePromotionRollbackReviewEvidence struct {
	OwnerApprovalRef             string `json:"owner_approval_ref"`
	PromotionPlanRef             string `json:"promotion_plan_ref"`
	RollbackPlanRef              string `json:"rollback_plan_ref"`
	PromotionRiskReviewRef       string `json:"promotion_risk_review_ref"`
	LedgerFreshnessCheckRef      string `json:"ledger_freshness_check_ref"`
	RouteContinuityCheckRef      string `json:"route_continuity_check_ref"`
	OperatorReviewPolicyRef      string `json:"operator_review_policy_ref"`
	NoPromotionMutation          bool   `json:"no_promotion_mutation"`
	NoPackagePublicationMutation bool   `json:"no_package_publication_mutation"`
	NoRunAcceptanceMutation      bool   `json:"no_run_acceptance_mutation"`
	NoProductionMutation         bool   `json:"no_production_mutation"`
	PromotionExecuted            bool   `json:"promotion_executed"`
	PackagePublished             bool   `json:"package_published"`
	RunAcceptanceRecordTouched   bool   `json:"run_acceptance_record_touched"`
	FullSubstrateClaimed         bool   `json:"full_substrate_claimed"`
	CompletionClaimed            bool   `json:"completion_claimed"`
}

// BasePromotionRollbackReviewContract packages promotion/rollback review inputs
// after owner approval. It is not promotion execution or publication authority.
type BasePromotionRollbackReviewContract struct {
	Kind                            string          `json:"kind"`
	Version                         ComputerVersion `json:"version"`
	Boundary                        string          `json:"boundary"`
	Scope                           string          `json:"scope"`
	TypedArtifactProgramRef         string          `json:"typed_artifact_program_ref"`
	OwnerApprovalRef                string          `json:"owner_approval_ref"`
	OwnerDecisionRef                string          `json:"owner_decision_ref"`
	OwnerIdentityRef                string          `json:"owner_identity_ref"`
	PromotionPlanRef                string          `json:"promotion_plan_ref"`
	RollbackPlanRef                 string          `json:"rollback_plan_ref"`
	PromotionRiskReviewRef          string          `json:"promotion_risk_review_ref"`
	LedgerFreshnessCheckRef         string          `json:"ledger_freshness_check_ref"`
	RouteContinuityCheckRef         string          `json:"route_continuity_check_ref"`
	OperatorReviewPolicyRef         string          `json:"operator_review_policy_ref"`
	ReviewStatus                    string          `json:"review_status"`
	VerifierContractSatisfied       bool            `json:"verifier_contract_satisfied"`
	OwnerApproved                   bool            `json:"owner_approved"`
	PromotionRollbackReviewReady    bool            `json:"promotion_rollback_review_ready"`
	PackagePublicationProofRequired bool            `json:"package_publication_proof_required"`
	RunAcceptanceProofRequired      bool            `json:"run_acceptance_proof_required"`
	FullSubstrateProofRequired      bool            `json:"full_substrate_proof_required"`
	PromotionAllowed                bool            `json:"promotion_allowed"`
	PackagePublicationAllowed       bool            `json:"package_publication_allowed"`
	RunAcceptanceSynthesisAllowed   bool            `json:"run_acceptance_synthesis_allowed"`
	NoPromotionMutation             bool            `json:"no_promotion_mutation"`
	NoPackagePublicationMutation    bool            `json:"no_package_publication_mutation"`
	NoRunAcceptanceMutation         bool            `json:"no_run_acceptance_mutation"`
	NoProductionMutation            bool            `json:"no_production_mutation"`
	PromotionExecuted               bool            `json:"promotion_executed"`
	PackagePublished                bool            `json:"package_published"`
	RunAcceptanceRecordTouched      bool            `json:"run_acceptance_record_touched"`
	FullSubstrateClaimed            bool            `json:"full_substrate_claimed"`
	CompletionClaimed               bool            `json:"completion_claimed"`
}

// BuildBasePromotionRollbackReviewContract records promotion/rollback review
// readiness after owner approval. It does not promote, publish, synthesize run
// acceptance, or claim completion.
func BuildBasePromotionRollbackReviewContract(owner BaseOwnerApprovalContract, evidence BasePromotionRollbackReviewEvidence) (BasePromotionRollbackReviewContract, error) {
	if err := validateBasePromotionRollbackReviewOwner(owner); err != nil {
		return BasePromotionRollbackReviewContract{}, err
	}
	if err := validateBasePromotionRollbackReviewEvidence(evidence); err != nil {
		return BasePromotionRollbackReviewContract{}, err
	}

	return BasePromotionRollbackReviewContract{
		Kind:                            BasePromotionRollbackReviewContractKind,
		Version:                         owner.Version,
		Boundary:                        BasePromotionRollbackReviewBoundary,
		Scope:                           BasePromotionRollbackReviewScope,
		TypedArtifactProgramRef:         string(owner.Version.ArtifactProgramRef),
		OwnerApprovalRef:                strings.TrimSpace(evidence.OwnerApprovalRef),
		OwnerDecisionRef:                strings.TrimSpace(owner.OwnerDecisionRef),
		OwnerIdentityRef:                strings.TrimSpace(owner.OwnerIdentityRef),
		PromotionPlanRef:                strings.TrimSpace(evidence.PromotionPlanRef),
		RollbackPlanRef:                 strings.TrimSpace(evidence.RollbackPlanRef),
		PromotionRiskReviewRef:          strings.TrimSpace(evidence.PromotionRiskReviewRef),
		LedgerFreshnessCheckRef:         strings.TrimSpace(evidence.LedgerFreshnessCheckRef),
		RouteContinuityCheckRef:         strings.TrimSpace(evidence.RouteContinuityCheckRef),
		OperatorReviewPolicyRef:         strings.TrimSpace(evidence.OperatorReviewPolicyRef),
		ReviewStatus:                    BasePromotionRollbackReviewStatusReady,
		VerifierContractSatisfied:       true,
		OwnerApproved:                   true,
		PromotionRollbackReviewReady:    true,
		PackagePublicationProofRequired: true,
		RunAcceptanceProofRequired:      true,
		FullSubstrateProofRequired:      true,
		PromotionAllowed:                false,
		PackagePublicationAllowed:       false,
		RunAcceptanceSynthesisAllowed:   false,
		NoPromotionMutation:             true,
		NoPackagePublicationMutation:    true,
		NoRunAcceptanceMutation:         true,
		NoProductionMutation:            true,
	}, nil
}

func validateBasePromotionRollbackReviewOwner(owner BaseOwnerApprovalContract) error {
	if owner.Kind != BaseOwnerApprovalContractKind {
		return fmt.Errorf("base promotion/rollback review: owner approval kind is %q", owner.Kind)
	}
	if owner.Boundary != BaseOwnerApprovalBoundary {
		return fmt.Errorf("base promotion/rollback review: owner approval boundary is %q", owner.Boundary)
	}
	if owner.Scope != BaseOwnerApprovalScope {
		return fmt.Errorf("base promotion/rollback review: owner approval scope is %q", owner.Scope)
	}
	if !owner.Version.Valid() {
		return fmt.Errorf("base promotion/rollback review: owner approval version is invalid")
	}
	if !owner.Version.ArtifactProgramRef.Valid() || ArtifactProgramRef(owner.TypedArtifactProgramRef) != owner.Version.ArtifactProgramRef {
		return fmt.Errorf("base promotion/rollback review: owner approval typed artifact-program ref is invalid")
	}
	if strings.TrimSpace(owner.OwnerDecisionRef) == "" || strings.TrimSpace(owner.OwnerIdentityRef) == "" || strings.TrimSpace(owner.RollbackPlanRef) == "" {
		return fmt.Errorf("base promotion/rollback review: owner approval refs are required")
	}
	if !owner.VerifierContractSatisfied || !owner.OwnerApprovalRecorded || !owner.OwnerApproved || owner.OwnerRejected || owner.OwnerRejectionBlocksDownstream || owner.OwnerDecision != BaseOwnerDecisionApprove || strings.TrimSpace(owner.RejectionReason) != "" {
		return fmt.Errorf("base promotion/rollback review: owner approval must be approved and unblocked")
	}
	if !owner.PromotionRollbackReviewRequired || !owner.PackagePublicationProofRequired || !owner.RunAcceptanceProofRequired || !owner.FullSubstrateProofRequired {
		return fmt.Errorf("base promotion/rollback review: owner approval must preserve downstream proof requirements")
	}
	if owner.PromotionAllowed || owner.PackagePublicationAllowed || owner.RunAcceptanceSynthesisAllowed {
		return fmt.Errorf("base promotion/rollback review: owner approval allows downstream execution")
	}
	if !owner.NoPromotionMutation || !owner.NoPackagePublicationMutation || !owner.NoRunAcceptanceMutation || !owner.NoProductionMutation {
		return fmt.Errorf("base promotion/rollback review: owner approval must prove no promotion, package publication, run-acceptance, or production mutation")
	}
	if owner.PromotionExecuted || owner.PackagePublished || owner.RunAcceptanceRecordTouched || owner.FullSubstrateClaimed || owner.CompletionClaimed {
		return fmt.Errorf("base promotion/rollback review: owner approval carries downstream execution or completion claims")
	}
	return nil
}

func validateBasePromotionRollbackReviewEvidence(evidence BasePromotionRollbackReviewEvidence) error {
	if strings.TrimSpace(evidence.OwnerApprovalRef) == "" || strings.TrimSpace(evidence.PromotionPlanRef) == "" || strings.TrimSpace(evidence.RollbackPlanRef) == "" || strings.TrimSpace(evidence.PromotionRiskReviewRef) == "" || strings.TrimSpace(evidence.LedgerFreshnessCheckRef) == "" || strings.TrimSpace(evidence.RouteContinuityCheckRef) == "" || strings.TrimSpace(evidence.OperatorReviewPolicyRef) == "" {
		return fmt.Errorf("base promotion/rollback review: review refs are required")
	}
	if !evidence.NoPromotionMutation || !evidence.NoPackagePublicationMutation || !evidence.NoRunAcceptanceMutation || !evidence.NoProductionMutation {
		return fmt.Errorf("base promotion/rollback review: evidence must prove no promotion, package publication, run-acceptance, or production mutation")
	}
	if evidence.PromotionExecuted || evidence.PackagePublished || evidence.RunAcceptanceRecordTouched || evidence.FullSubstrateClaimed || evidence.CompletionClaimed {
		return fmt.Errorf("base promotion/rollback review: evidence carries downstream execution or completion claims")
	}
	return nil
}
