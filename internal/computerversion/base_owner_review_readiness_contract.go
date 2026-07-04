package computerversion

import (
	"fmt"
	"strings"
)

const BaseOwnerReviewReadinessContractKind = "base_owner_review_readiness_contract"

const BaseOwnerReviewReadinessBoundary = "owner_review_readiness_without_owner_approval_or_downstream_execution"

const BaseOwnerReviewReadinessScope = "post_smoke_handoff_to_owner_review_packet_only"

const BaseOwnerReviewReadinessStatusReady = "ready_for_owner_review_not_approved"

// BaseOwnerReviewReadinessEvidence records the packet refs needed to ask an
// owner to review a post-smoke handoff. It does not record approval and does not
// execute promotion, publication, verifier satisfaction, or run acceptance.
type BaseOwnerReviewReadinessEvidence struct {
	PostSmokeHandoffRef          string `json:"post_smoke_handoff_ref"`
	ReviewPacketRef              string `json:"review_packet_ref"`
	ReviewerIdentityPolicyRef    string `json:"reviewer_identity_policy_ref"`
	OwnerReviewInstructionsRef   string `json:"owner_review_instructions_ref"`
	RiskSummaryRef               string `json:"risk_summary_ref"`
	RollbackPlanRef              string `json:"rollback_plan_ref"`
	NoOwnerApprovalMutation      bool   `json:"no_owner_approval_mutation"`
	NoPromotionMutation          bool   `json:"no_promotion_mutation"`
	NoPackagePublicationMutation bool   `json:"no_package_publication_mutation"`
	NoRunAcceptanceMutation      bool   `json:"no_run_acceptance_mutation"`
	NoProductionMutation         bool   `json:"no_production_mutation"`
	OwnerApproved                bool   `json:"owner_approved"`
	PromotionExecuted            bool   `json:"promotion_executed"`
	PackagePublished             bool   `json:"package_published"`
	VerifierContractSatisfied    bool   `json:"verifier_contract_satisfied"`
	RunAcceptanceRecordTouched   bool   `json:"run_acceptance_record_touched"`
	FullSubstrateClaimed         bool   `json:"full_substrate_claimed"`
	CompletionClaimed            bool   `json:"completion_claimed"`
}

// BaseOwnerReviewReadinessContract packages verified post-smoke evidence for
// owner review. It is review readiness only; owner approval and every downstream
// authority remain separately required.
type BaseOwnerReviewReadinessContract struct {
	Kind                                string          `json:"kind"`
	Version                             ComputerVersion `json:"version"`
	Boundary                            string          `json:"boundary"`
	Scope                               string          `json:"scope"`
	TypedArtifactProgramRef             string          `json:"typed_artifact_program_ref"`
	PostSmokeHandoffRef                 string          `json:"post_smoke_handoff_ref"`
	ProductPathProbeRef                 string          `json:"product_path_probe_ref"`
	BuildIdentity                       string          `json:"build_identity"`
	RouteIdentity                       string          `json:"route_identity"`
	ReviewPacketRef                     string          `json:"review_packet_ref"`
	ReviewerIdentityPolicyRef           string          `json:"reviewer_identity_policy_ref"`
	OwnerReviewInstructionsRef          string          `json:"owner_review_instructions_ref"`
	RiskSummaryRef                      string          `json:"risk_summary_ref"`
	RollbackPlanRef                     string          `json:"rollback_plan_ref"`
	ReadinessStatus                     string          `json:"readiness_status"`
	OwnerReviewReady                    bool            `json:"owner_review_ready"`
	OwnerApprovalRequired               bool            `json:"owner_approval_required"`
	PromotionRollbackReviewRequired     bool            `json:"promotion_rollback_review_required"`
	PackagePublicationProofRequired     bool            `json:"package_publication_proof_required"`
	VerifierContractProofRequired       bool            `json:"verifier_contract_proof_required"`
	RunAcceptanceProofRequired          bool            `json:"run_acceptance_proof_required"`
	FullSubstrateProofRequired          bool            `json:"full_substrate_proof_required"`
	OwnerApprovalAllowed                bool            `json:"owner_approval_allowed"`
	PromotionAllowed                    bool            `json:"promotion_allowed"`
	PackagePublicationAllowed           bool            `json:"package_publication_allowed"`
	VerifierContractSatisfactionAllowed bool            `json:"verifier_contract_satisfaction_allowed"`
	RunAcceptanceSynthesisAllowed       bool            `json:"run_acceptance_synthesis_allowed"`
	NoOwnerApprovalMutation             bool            `json:"no_owner_approval_mutation"`
	NoPromotionMutation                 bool            `json:"no_promotion_mutation"`
	NoPackagePublicationMutation        bool            `json:"no_package_publication_mutation"`
	NoRunAcceptanceMutation             bool            `json:"no_run_acceptance_mutation"`
	NoProductionMutation                bool            `json:"no_production_mutation"`
	OwnerApproved                       bool            `json:"owner_approved"`
	PromotionExecuted                   bool            `json:"promotion_executed"`
	PackagePublished                    bool            `json:"package_published"`
	VerifierContractSatisfied           bool            `json:"verifier_contract_satisfied"`
	RunAcceptanceRecordTouched          bool            `json:"run_acceptance_record_touched"`
	FullSubstrateClaimed                bool            `json:"full_substrate_claimed"`
	CompletionClaimed                   bool            `json:"completion_claimed"`
}

// BuildBaseOwnerReviewReadinessContract turns a blocked post-smoke handoff into
// a review packet boundary. It never approves, promotes, publishes, satisfies a
// verifier contract, synthesizes run acceptance, or claims completion.
func BuildBaseOwnerReviewReadinessContract(handoff BasePostSmokeHandoffReadinessContract, evidence BaseOwnerReviewReadinessEvidence) (BaseOwnerReviewReadinessContract, error) {
	if err := validateBaseOwnerReviewHandoff(handoff); err != nil {
		return BaseOwnerReviewReadinessContract{}, err
	}
	if err := validateBaseOwnerReviewEvidence(evidence); err != nil {
		return BaseOwnerReviewReadinessContract{}, err
	}

	return BaseOwnerReviewReadinessContract{
		Kind:                                BaseOwnerReviewReadinessContractKind,
		Version:                             handoff.Version,
		Boundary:                            BaseOwnerReviewReadinessBoundary,
		Scope:                               BaseOwnerReviewReadinessScope,
		TypedArtifactProgramRef:             string(handoff.Version.ArtifactProgramRef),
		PostSmokeHandoffRef:                 strings.TrimSpace(evidence.PostSmokeHandoffRef),
		ProductPathProbeRef:                 strings.TrimSpace(handoff.ProductPathProbeRef),
		BuildIdentity:                       strings.TrimSpace(handoff.BuildIdentity),
		RouteIdentity:                       strings.TrimSpace(handoff.RouteIdentity),
		ReviewPacketRef:                     strings.TrimSpace(evidence.ReviewPacketRef),
		ReviewerIdentityPolicyRef:           strings.TrimSpace(evidence.ReviewerIdentityPolicyRef),
		OwnerReviewInstructionsRef:          strings.TrimSpace(evidence.OwnerReviewInstructionsRef),
		RiskSummaryRef:                      strings.TrimSpace(evidence.RiskSummaryRef),
		RollbackPlanRef:                     strings.TrimSpace(evidence.RollbackPlanRef),
		ReadinessStatus:                     BaseOwnerReviewReadinessStatusReady,
		OwnerReviewReady:                    true,
		OwnerApprovalRequired:               true,
		PromotionRollbackReviewRequired:     true,
		PackagePublicationProofRequired:     true,
		VerifierContractProofRequired:       true,
		RunAcceptanceProofRequired:          true,
		FullSubstrateProofRequired:          true,
		OwnerApprovalAllowed:                false,
		PromotionAllowed:                    false,
		PackagePublicationAllowed:           false,
		VerifierContractSatisfactionAllowed: false,
		RunAcceptanceSynthesisAllowed:       false,
		NoOwnerApprovalMutation:             true,
		NoPromotionMutation:                 true,
		NoPackagePublicationMutation:        true,
		NoRunAcceptanceMutation:             true,
		NoProductionMutation:                true,
	}, nil
}

func validateBaseOwnerReviewHandoff(handoff BasePostSmokeHandoffReadinessContract) error {
	if handoff.Kind != BasePostSmokeHandoffReadinessContractKind {
		return fmt.Errorf("base owner-review readiness: handoff kind is %q", handoff.Kind)
	}
	if handoff.Boundary != BasePostSmokeHandoffReadinessBoundary {
		return fmt.Errorf("base owner-review readiness: handoff boundary is %q", handoff.Boundary)
	}
	if handoff.Scope != BasePostSmokeHandoffReadinessScope {
		return fmt.Errorf("base owner-review readiness: handoff scope is %q", handoff.Scope)
	}
	if !handoff.Version.Valid() {
		return fmt.Errorf("base owner-review readiness: handoff version is invalid")
	}
	if !handoff.Version.ArtifactProgramRef.Valid() || ArtifactProgramRef(handoff.TypedArtifactProgramRef) != handoff.Version.ArtifactProgramRef {
		return fmt.Errorf("base owner-review readiness: handoff typed artifact-program ref is invalid")
	}
	if strings.TrimSpace(handoff.StagingSmokeEvidenceRef) == "" || strings.TrimSpace(handoff.ProductPathProbeRef) == "" || strings.TrimSpace(handoff.BuildIdentity) == "" || strings.TrimSpace(handoff.RouteIdentity) == "" || strings.TrimSpace(handoff.OwnerReviewPlanRef) == "" || strings.TrimSpace(handoff.PromotionRollbackPlanRef) == "" || strings.TrimSpace(handoff.PackagePublicationPlanRef) == "" || strings.TrimSpace(handoff.VerifierContractPlanRef) == "" || strings.TrimSpace(handoff.RunAcceptanceSynthesisPlanRef) == "" || strings.TrimSpace(handoff.RollbackPlanRef) == "" {
		return fmt.Errorf("base owner-review readiness: handoff refs are required")
	}
	if handoff.ReadinessStatus != BasePostSmokeHandoffReadinessStatusBlocked {
		return fmt.Errorf("base owner-review readiness: handoff must remain blocked")
	}
	if !handoff.OwnerReviewRequired || !handoff.PromotionRollbackReviewRequired || !handoff.PackagePublicationProofRequired || !handoff.VerifierContractProofRequired || !handoff.RunAcceptanceProofRequired || !handoff.FullSubstrateProofRequired {
		return fmt.Errorf("base owner-review readiness: handoff must preserve downstream proof requirements")
	}
	if handoff.OwnerApprovalAllowed || handoff.PromotionAllowed || handoff.PackagePublicationAllowed || handoff.RunAcceptanceSynthesisAllowed {
		return fmt.Errorf("base owner-review readiness: handoff allows downstream execution")
	}
	if !handoff.NoOwnerApprovalMutation || !handoff.NoPromotionMutation || !handoff.NoPackagePublicationMutation || !handoff.NoRunAcceptanceMutation || !handoff.NoProductionMutation {
		return fmt.Errorf("base owner-review readiness: handoff must prove no owner approval, promotion, package publication, run-acceptance, or production mutation")
	}
	if handoff.OwnerApproved || handoff.PromotionExecuted || handoff.PackagePublished || handoff.RunAcceptanceRecordTouched || handoff.VerifierContractSatisfied || handoff.FullSubstrateClaimed || handoff.CompletionClaimed {
		return fmt.Errorf("base owner-review readiness: handoff carries downstream execution or completion claims")
	}
	return nil
}

func validateBaseOwnerReviewEvidence(evidence BaseOwnerReviewReadinessEvidence) error {
	if strings.TrimSpace(evidence.PostSmokeHandoffRef) == "" || strings.TrimSpace(evidence.ReviewPacketRef) == "" || strings.TrimSpace(evidence.ReviewerIdentityPolicyRef) == "" || strings.TrimSpace(evidence.OwnerReviewInstructionsRef) == "" || strings.TrimSpace(evidence.RiskSummaryRef) == "" || strings.TrimSpace(evidence.RollbackPlanRef) == "" {
		return fmt.Errorf("base owner-review readiness: review packet refs are required")
	}
	if !evidence.NoOwnerApprovalMutation || !evidence.NoPromotionMutation || !evidence.NoPackagePublicationMutation || !evidence.NoRunAcceptanceMutation || !evidence.NoProductionMutation {
		return fmt.Errorf("base owner-review readiness: evidence must prove no owner approval, promotion, package publication, run-acceptance, or production mutation")
	}
	if evidence.OwnerApproved || evidence.PromotionExecuted || evidence.PackagePublished || evidence.VerifierContractSatisfied || evidence.RunAcceptanceRecordTouched || evidence.FullSubstrateClaimed || evidence.CompletionClaimed {
		return fmt.Errorf("base owner-review readiness: evidence carries downstream execution or completion claims")
	}
	return nil
}
