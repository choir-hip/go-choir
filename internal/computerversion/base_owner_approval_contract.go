package computerversion

import (
	"fmt"
	"strings"
)

const BaseOwnerApprovalContractKind = "base_owner_approval_contract"

const BaseOwnerApprovalBoundary = "owner_approval_without_promotion_publication_or_run_acceptance"

const BaseOwnerApprovalScope = "passing_verifier_result_and_owner_review_packet_to_owner_decision_only"

const BaseOwnerDecisionApprove = "approve"
const BaseOwnerDecisionReject = "reject"

// BaseOwnerApprovalEvidence records one owner decision over a review packet after
// the verifier has passed. It does not execute promotion, publish packages,
// synthesize run acceptance, or claim full substrate completion.
type BaseOwnerApprovalEvidence struct {
	OwnerReviewReadinessRef      string `json:"owner_review_readiness_ref"`
	ReviewPacketRef              string `json:"review_packet_ref"`
	VerifierResultRef            string `json:"verifier_result_ref"`
	OwnerDecisionRef             string `json:"owner_decision_ref"`
	OwnerIdentityRef             string `json:"owner_identity_ref"`
	OwnerDecision                string `json:"owner_decision"`
	RejectionReason              string `json:"rejection_reason"`
	RollbackPlanRef              string `json:"rollback_plan_ref"`
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

// BaseOwnerApprovalContract records an owner approve/reject decision. Approval
// is a local authority fact only; promotion, publication, run acceptance, and
// completion remain separate gates.
type BaseOwnerApprovalContract struct {
	Kind                            string          `json:"kind"`
	Version                         ComputerVersion `json:"version"`
	Boundary                        string          `json:"boundary"`
	Scope                           string          `json:"scope"`
	TypedArtifactProgramRef         string          `json:"typed_artifact_program_ref"`
	OwnerReviewReadinessRef         string          `json:"owner_review_readiness_ref"`
	ReviewPacketRef                 string          `json:"review_packet_ref"`
	VerifierResultRef               string          `json:"verifier_result_ref"`
	OwnerDecisionRef                string          `json:"owner_decision_ref"`
	OwnerIdentityRef                string          `json:"owner_identity_ref"`
	OwnerDecision                   string          `json:"owner_decision"`
	RejectionReason                 string          `json:"rejection_reason"`
	RollbackPlanRef                 string          `json:"rollback_plan_ref"`
	VerifierContractSatisfied       bool            `json:"verifier_contract_satisfied"`
	OwnerApprovalRecorded           bool            `json:"owner_approval_recorded"`
	OwnerApproved                   bool            `json:"owner_approved"`
	OwnerRejected                   bool            `json:"owner_rejected"`
	OwnerRejectionBlocksDownstream  bool            `json:"owner_rejection_blocks_downstream"`
	PromotionRollbackReviewRequired bool            `json:"promotion_rollback_review_required"`
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

// BuildBaseOwnerApprovalContract records an owner approve/reject decision after
// verifier pass. It does not promote, publish, synthesize run acceptance, or
// claim completion.
func BuildBaseOwnerApprovalContract(owner BaseOwnerReviewReadinessContract, verifier BaseVerifierResultContract, evidence BaseOwnerApprovalEvidence) (BaseOwnerApprovalContract, error) {
	if err := validateBaseOwnerApprovalOwnerReview(owner); err != nil {
		return BaseOwnerApprovalContract{}, err
	}
	if err := validateBaseOwnerApprovalVerifierResult(owner, verifier); err != nil {
		return BaseOwnerApprovalContract{}, err
	}
	decision, rejectionReason, err := validateBaseOwnerApprovalEvidence(owner, verifier, evidence)
	if err != nil {
		return BaseOwnerApprovalContract{}, err
	}
	approved := decision == BaseOwnerDecisionApprove
	rejected := decision == BaseOwnerDecisionReject

	return BaseOwnerApprovalContract{
		Kind:                            BaseOwnerApprovalContractKind,
		Version:                         owner.Version,
		Boundary:                        BaseOwnerApprovalBoundary,
		Scope:                           BaseOwnerApprovalScope,
		TypedArtifactProgramRef:         string(owner.Version.ArtifactProgramRef),
		OwnerReviewReadinessRef:         strings.TrimSpace(evidence.OwnerReviewReadinessRef),
		ReviewPacketRef:                 strings.TrimSpace(evidence.ReviewPacketRef),
		VerifierResultRef:               strings.TrimSpace(evidence.VerifierResultRef),
		OwnerDecisionRef:                strings.TrimSpace(evidence.OwnerDecisionRef),
		OwnerIdentityRef:                strings.TrimSpace(evidence.OwnerIdentityRef),
		OwnerDecision:                   decision,
		RejectionReason:                 rejectionReason,
		RollbackPlanRef:                 strings.TrimSpace(evidence.RollbackPlanRef),
		VerifierContractSatisfied:       true,
		OwnerApprovalRecorded:           true,
		OwnerApproved:                   approved,
		OwnerRejected:                   rejected,
		OwnerRejectionBlocksDownstream:  rejected,
		PromotionRollbackReviewRequired: true,
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

func validateBaseOwnerApprovalOwnerReview(owner BaseOwnerReviewReadinessContract) error {
	if owner.Kind != BaseOwnerReviewReadinessContractKind {
		return fmt.Errorf("base owner approval: owner-review kind is %q", owner.Kind)
	}
	if owner.Boundary != BaseOwnerReviewReadinessBoundary {
		return fmt.Errorf("base owner approval: owner-review boundary is %q", owner.Boundary)
	}
	if owner.Scope != BaseOwnerReviewReadinessScope {
		return fmt.Errorf("base owner approval: owner-review scope is %q", owner.Scope)
	}
	if !owner.Version.Valid() {
		return fmt.Errorf("base owner approval: owner-review version is invalid")
	}
	if !owner.Version.ArtifactProgramRef.Valid() || ArtifactProgramRef(owner.TypedArtifactProgramRef) != owner.Version.ArtifactProgramRef {
		return fmt.Errorf("base owner approval: owner-review typed artifact-program ref is invalid")
	}
	if strings.TrimSpace(owner.ReviewPacketRef) == "" || strings.TrimSpace(owner.ReviewerIdentityPolicyRef) == "" || strings.TrimSpace(owner.OwnerReviewInstructionsRef) == "" || strings.TrimSpace(owner.RiskSummaryRef) == "" || strings.TrimSpace(owner.RollbackPlanRef) == "" {
		return fmt.Errorf("base owner approval: owner-review packet refs are required")
	}
	if owner.ReadinessStatus != BaseOwnerReviewReadinessStatusReady || !owner.OwnerReviewReady {
		return fmt.Errorf("base owner approval: owner-review must be ready but not approved")
	}
	if !owner.OwnerApprovalRequired || !owner.PromotionRollbackReviewRequired || !owner.PackagePublicationProofRequired || !owner.VerifierContractProofRequired || !owner.RunAcceptanceProofRequired || !owner.FullSubstrateProofRequired {
		return fmt.Errorf("base owner approval: owner-review must preserve downstream proof requirements")
	}
	if owner.OwnerApprovalAllowed || owner.PromotionAllowed || owner.PackagePublicationAllowed || owner.VerifierContractSatisfactionAllowed || owner.RunAcceptanceSynthesisAllowed {
		return fmt.Errorf("base owner approval: owner-review allows downstream execution")
	}
	if !owner.NoOwnerApprovalMutation || !owner.NoPromotionMutation || !owner.NoPackagePublicationMutation || !owner.NoRunAcceptanceMutation || !owner.NoProductionMutation {
		return fmt.Errorf("base owner approval: owner-review must prove no owner approval, promotion, package publication, run-acceptance, or production mutation")
	}
	if owner.OwnerApproved || owner.PromotionExecuted || owner.PackagePublished || owner.VerifierContractSatisfied || owner.RunAcceptanceRecordTouched || owner.FullSubstrateClaimed || owner.CompletionClaimed {
		return fmt.Errorf("base owner approval: owner-review carries downstream execution or completion claims")
	}
	return nil
}

func validateBaseOwnerApprovalVerifierResult(owner BaseOwnerReviewReadinessContract, verifier BaseVerifierResultContract) error {
	if verifier.Kind != BaseVerifierResultContractKind {
		return fmt.Errorf("base owner approval: verifier result kind is %q", verifier.Kind)
	}
	if verifier.Boundary != BaseVerifierResultBoundary {
		return fmt.Errorf("base owner approval: verifier result boundary is %q", verifier.Boundary)
	}
	if verifier.Scope != BaseVerifierResultScope {
		return fmt.Errorf("base owner approval: verifier result scope is %q", verifier.Scope)
	}
	if verifier.Version != owner.Version || ArtifactProgramRef(verifier.TypedArtifactProgramRef) != owner.Version.ArtifactProgramRef {
		return fmt.Errorf("base owner approval: verifier result does not match owner-review version")
	}
	if strings.TrimSpace(verifier.VerifierResultRef) == "" || strings.TrimSpace(verifier.VerifierRunRef) == "" || strings.TrimSpace(verifier.VerifierLogRef) == "" || strings.TrimSpace(verifier.RollbackPlanRef) == "" {
		return fmt.Errorf("base owner approval: verifier result refs are required")
	}
	if verifier.Verdict != BaseVerifierVerdictPass || !verifier.VerifierContractSatisfied || verifier.VerifierContractFailed || verifier.VerifierFailureBlocksDownstream || strings.TrimSpace(verifier.FailureReason) != "" {
		return fmt.Errorf("base owner approval: verifier result must be a passing verifier result")
	}
	if !verifier.OwnerApprovalRequired || !verifier.PromotionRollbackReviewRequired || !verifier.PackagePublicationProofRequired || !verifier.RunAcceptanceProofRequired || !verifier.FullSubstrateProofRequired {
		return fmt.Errorf("base owner approval: verifier result must preserve downstream proof requirements")
	}
	if verifier.OwnerApprovalAllowed || verifier.PromotionAllowed || verifier.PackagePublicationAllowed || verifier.RunAcceptanceSynthesisAllowed {
		return fmt.Errorf("base owner approval: verifier result allows downstream execution")
	}
	if !verifier.NoOwnerApprovalMutation || !verifier.NoPromotionMutation || !verifier.NoPackagePublicationMutation || !verifier.NoRunAcceptanceMutation || !verifier.NoProductionMutation {
		return fmt.Errorf("base owner approval: verifier result must prove no owner approval, promotion, package publication, run-acceptance, or production mutation")
	}
	if verifier.OwnerApproved || verifier.PromotionExecuted || verifier.PackagePublished || verifier.RunAcceptanceRecordTouched || verifier.FullSubstrateClaimed || verifier.CompletionClaimed {
		return fmt.Errorf("base owner approval: verifier result carries downstream execution or completion claims")
	}
	return nil
}

func validateBaseOwnerApprovalEvidence(owner BaseOwnerReviewReadinessContract, verifier BaseVerifierResultContract, evidence BaseOwnerApprovalEvidence) (string, string, error) {
	if strings.TrimSpace(evidence.OwnerReviewReadinessRef) == "" || strings.TrimSpace(evidence.ReviewPacketRef) == "" || strings.TrimSpace(evidence.VerifierResultRef) == "" || strings.TrimSpace(evidence.OwnerDecisionRef) == "" || strings.TrimSpace(evidence.OwnerIdentityRef) == "" || strings.TrimSpace(evidence.RollbackPlanRef) == "" {
		return "", "", fmt.Errorf("base owner approval: owner decision refs are required")
	}
	if strings.TrimSpace(evidence.ReviewPacketRef) != strings.TrimSpace(owner.ReviewPacketRef) {
		return "", "", fmt.Errorf("base owner approval: review packet ref does not match owner review")
	}
	if strings.TrimSpace(evidence.VerifierResultRef) != strings.TrimSpace(verifier.VerifierResultRef) {
		return "", "", fmt.Errorf("base owner approval: verifier result ref does not match verifier result")
	}
	decision := strings.TrimSpace(evidence.OwnerDecision)
	rejectionReason := strings.TrimSpace(evidence.RejectionReason)
	if decision != BaseOwnerDecisionApprove && decision != BaseOwnerDecisionReject {
		return "", "", fmt.Errorf("base owner approval: owner decision must be approve or reject")
	}
	if decision == BaseOwnerDecisionApprove && rejectionReason != "" {
		return "", "", fmt.Errorf("base owner approval: approval cannot include a rejection reason")
	}
	if decision == BaseOwnerDecisionReject && rejectionReason == "" {
		return "", "", fmt.Errorf("base owner approval: rejection requires a reason")
	}
	if !evidence.NoPromotionMutation || !evidence.NoPackagePublicationMutation || !evidence.NoRunAcceptanceMutation || !evidence.NoProductionMutation {
		return "", "", fmt.Errorf("base owner approval: evidence must prove no promotion, package publication, run-acceptance, or production mutation")
	}
	if evidence.PromotionExecuted || evidence.PackagePublished || evidence.RunAcceptanceRecordTouched || evidence.FullSubstrateClaimed || evidence.CompletionClaimed {
		return "", "", fmt.Errorf("base owner approval: evidence carries downstream execution or completion claims")
	}
	return decision, rejectionReason, nil
}
