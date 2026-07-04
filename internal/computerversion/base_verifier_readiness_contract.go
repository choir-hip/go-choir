package computerversion

import (
	"fmt"
	"strings"
)

const BaseVerifierReadinessContractKind = "base_verifier_readiness_contract"

const BaseVerifierReadinessBoundary = "verifier_readiness_without_verifier_satisfaction_or_downstream_execution"

const BaseVerifierReadinessScope = "owner_review_readiness_to_verifier_input_packet_only"

const BaseVerifierReadinessStatusReady = "ready_for_verifier_review_not_satisfied"

// BaseVerifierReadinessEvidence records the input bundle refs needed before a
// verifier contract can be evaluated. It does not satisfy the verifier or grant
// owner approval, promotion, publication, or run-acceptance authority.
type BaseVerifierReadinessEvidence struct {
	OwnerReviewReadinessRef      string `json:"owner_review_readiness_ref"`
	VerifierInputBundleRef       string `json:"verifier_input_bundle_ref"`
	VerifierContractSpecRef      string `json:"verifier_contract_spec_ref"`
	EvidenceManifestRef          string `json:"evidence_manifest_ref"`
	ExpectedVerdictPolicyRef     string `json:"expected_verdict_policy_ref"`
	RollbackPlanRef              string `json:"rollback_plan_ref"`
	NoOwnerApprovalMutation      bool   `json:"no_owner_approval_mutation"`
	NoPromotionMutation          bool   `json:"no_promotion_mutation"`
	NoPackagePublicationMutation bool   `json:"no_package_publication_mutation"`
	NoVerifierSatisfaction       bool   `json:"no_verifier_satisfaction"`
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

// BaseVerifierReadinessContract packages owner-review readiness into verifier
// inputs. It is not a verifier result and cannot unlock downstream execution.
type BaseVerifierReadinessContract struct {
	Kind                                string          `json:"kind"`
	Version                             ComputerVersion `json:"version"`
	Boundary                            string          `json:"boundary"`
	Scope                               string          `json:"scope"`
	TypedArtifactProgramRef             string          `json:"typed_artifact_program_ref"`
	OwnerReviewReadinessRef             string          `json:"owner_review_readiness_ref"`
	ProductPathProbeRef                 string          `json:"product_path_probe_ref"`
	BuildIdentity                       string          `json:"build_identity"`
	RouteIdentity                       string          `json:"route_identity"`
	ReviewPacketRef                     string          `json:"review_packet_ref"`
	VerifierInputBundleRef              string          `json:"verifier_input_bundle_ref"`
	VerifierContractSpecRef             string          `json:"verifier_contract_spec_ref"`
	EvidenceManifestRef                 string          `json:"evidence_manifest_ref"`
	ExpectedVerdictPolicyRef            string          `json:"expected_verdict_policy_ref"`
	RollbackPlanRef                     string          `json:"rollback_plan_ref"`
	ReadinessStatus                     string          `json:"readiness_status"`
	VerifierReviewReady                 bool            `json:"verifier_review_ready"`
	VerifierContractProofRequired       bool            `json:"verifier_contract_proof_required"`
	OwnerApprovalRequired               bool            `json:"owner_approval_required"`
	PromotionRollbackReviewRequired     bool            `json:"promotion_rollback_review_required"`
	PackagePublicationProofRequired     bool            `json:"package_publication_proof_required"`
	RunAcceptanceProofRequired          bool            `json:"run_acceptance_proof_required"`
	FullSubstrateProofRequired          bool            `json:"full_substrate_proof_required"`
	VerifierContractSatisfactionAllowed bool            `json:"verifier_contract_satisfaction_allowed"`
	OwnerApprovalAllowed                bool            `json:"owner_approval_allowed"`
	PromotionAllowed                    bool            `json:"promotion_allowed"`
	PackagePublicationAllowed           bool            `json:"package_publication_allowed"`
	RunAcceptanceSynthesisAllowed       bool            `json:"run_acceptance_synthesis_allowed"`
	NoOwnerApprovalMutation             bool            `json:"no_owner_approval_mutation"`
	NoPromotionMutation                 bool            `json:"no_promotion_mutation"`
	NoPackagePublicationMutation        bool            `json:"no_package_publication_mutation"`
	NoVerifierSatisfaction              bool            `json:"no_verifier_satisfaction"`
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

// BuildBaseVerifierReadinessContract turns owner-review readiness into a
// verifier input packet. It does not satisfy the verifier or execute downstream
// promotion, publication, run acceptance, or completion.
func BuildBaseVerifierReadinessContract(owner BaseOwnerReviewReadinessContract, evidence BaseVerifierReadinessEvidence) (BaseVerifierReadinessContract, error) {
	if err := validateBaseVerifierOwnerReadiness(owner); err != nil {
		return BaseVerifierReadinessContract{}, err
	}
	if err := validateBaseVerifierReadinessEvidence(evidence); err != nil {
		return BaseVerifierReadinessContract{}, err
	}

	return BaseVerifierReadinessContract{
		Kind:                                BaseVerifierReadinessContractKind,
		Version:                             owner.Version,
		Boundary:                            BaseVerifierReadinessBoundary,
		Scope:                               BaseVerifierReadinessScope,
		TypedArtifactProgramRef:             string(owner.Version.ArtifactProgramRef),
		OwnerReviewReadinessRef:             strings.TrimSpace(evidence.OwnerReviewReadinessRef),
		ProductPathProbeRef:                 strings.TrimSpace(owner.ProductPathProbeRef),
		BuildIdentity:                       strings.TrimSpace(owner.BuildIdentity),
		RouteIdentity:                       strings.TrimSpace(owner.RouteIdentity),
		ReviewPacketRef:                     strings.TrimSpace(owner.ReviewPacketRef),
		VerifierInputBundleRef:              strings.TrimSpace(evidence.VerifierInputBundleRef),
		VerifierContractSpecRef:             strings.TrimSpace(evidence.VerifierContractSpecRef),
		EvidenceManifestRef:                 strings.TrimSpace(evidence.EvidenceManifestRef),
		ExpectedVerdictPolicyRef:            strings.TrimSpace(evidence.ExpectedVerdictPolicyRef),
		RollbackPlanRef:                     strings.TrimSpace(evidence.RollbackPlanRef),
		ReadinessStatus:                     BaseVerifierReadinessStatusReady,
		VerifierReviewReady:                 true,
		VerifierContractProofRequired:       true,
		OwnerApprovalRequired:               true,
		PromotionRollbackReviewRequired:     true,
		PackagePublicationProofRequired:     true,
		RunAcceptanceProofRequired:          true,
		FullSubstrateProofRequired:          true,
		VerifierContractSatisfactionAllowed: false,
		OwnerApprovalAllowed:                false,
		PromotionAllowed:                    false,
		PackagePublicationAllowed:           false,
		RunAcceptanceSynthesisAllowed:       false,
		NoOwnerApprovalMutation:             true,
		NoPromotionMutation:                 true,
		NoPackagePublicationMutation:        true,
		NoVerifierSatisfaction:              true,
		NoRunAcceptanceMutation:             true,
		NoProductionMutation:                true,
	}, nil
}

func validateBaseVerifierOwnerReadiness(owner BaseOwnerReviewReadinessContract) error {
	if owner.Kind != BaseOwnerReviewReadinessContractKind {
		return fmt.Errorf("base verifier readiness: owner-review kind is %q", owner.Kind)
	}
	if owner.Boundary != BaseOwnerReviewReadinessBoundary {
		return fmt.Errorf("base verifier readiness: owner-review boundary is %q", owner.Boundary)
	}
	if owner.Scope != BaseOwnerReviewReadinessScope {
		return fmt.Errorf("base verifier readiness: owner-review scope is %q", owner.Scope)
	}
	if !owner.Version.Valid() {
		return fmt.Errorf("base verifier readiness: owner-review version is invalid")
	}
	if !owner.Version.ArtifactProgramRef.Valid() || ArtifactProgramRef(owner.TypedArtifactProgramRef) != owner.Version.ArtifactProgramRef {
		return fmt.Errorf("base verifier readiness: owner-review typed artifact-program ref is invalid")
	}
	if strings.TrimSpace(owner.PostSmokeHandoffRef) == "" || strings.TrimSpace(owner.ProductPathProbeRef) == "" || strings.TrimSpace(owner.BuildIdentity) == "" || strings.TrimSpace(owner.RouteIdentity) == "" || strings.TrimSpace(owner.ReviewPacketRef) == "" || strings.TrimSpace(owner.ReviewerIdentityPolicyRef) == "" || strings.TrimSpace(owner.OwnerReviewInstructionsRef) == "" || strings.TrimSpace(owner.RiskSummaryRef) == "" || strings.TrimSpace(owner.RollbackPlanRef) == "" {
		return fmt.Errorf("base verifier readiness: owner-review refs are required")
	}
	if owner.ReadinessStatus != BaseOwnerReviewReadinessStatusReady || !owner.OwnerReviewReady {
		return fmt.Errorf("base verifier readiness: owner-review must be ready but not approved")
	}
	if !owner.OwnerApprovalRequired || !owner.PromotionRollbackReviewRequired || !owner.PackagePublicationProofRequired || !owner.VerifierContractProofRequired || !owner.RunAcceptanceProofRequired || !owner.FullSubstrateProofRequired {
		return fmt.Errorf("base verifier readiness: owner-review must preserve downstream proof requirements")
	}
	if owner.OwnerApprovalAllowed || owner.PromotionAllowed || owner.PackagePublicationAllowed || owner.VerifierContractSatisfactionAllowed || owner.RunAcceptanceSynthesisAllowed {
		return fmt.Errorf("base verifier readiness: owner-review allows downstream execution")
	}
	if !owner.NoOwnerApprovalMutation || !owner.NoPromotionMutation || !owner.NoPackagePublicationMutation || !owner.NoRunAcceptanceMutation || !owner.NoProductionMutation {
		return fmt.Errorf("base verifier readiness: owner-review must prove no owner approval, promotion, package publication, run-acceptance, or production mutation")
	}
	if owner.OwnerApproved || owner.PromotionExecuted || owner.PackagePublished || owner.VerifierContractSatisfied || owner.RunAcceptanceRecordTouched || owner.FullSubstrateClaimed || owner.CompletionClaimed {
		return fmt.Errorf("base verifier readiness: owner-review carries downstream execution or completion claims")
	}
	return nil
}

func validateBaseVerifierReadinessEvidence(evidence BaseVerifierReadinessEvidence) error {
	if strings.TrimSpace(evidence.OwnerReviewReadinessRef) == "" || strings.TrimSpace(evidence.VerifierInputBundleRef) == "" || strings.TrimSpace(evidence.VerifierContractSpecRef) == "" || strings.TrimSpace(evidence.EvidenceManifestRef) == "" || strings.TrimSpace(evidence.ExpectedVerdictPolicyRef) == "" || strings.TrimSpace(evidence.RollbackPlanRef) == "" {
		return fmt.Errorf("base verifier readiness: verifier refs are required")
	}
	if !evidence.NoOwnerApprovalMutation || !evidence.NoPromotionMutation || !evidence.NoPackagePublicationMutation || !evidence.NoVerifierSatisfaction || !evidence.NoRunAcceptanceMutation || !evidence.NoProductionMutation {
		return fmt.Errorf("base verifier readiness: evidence must prove no owner approval, promotion, package publication, verifier satisfaction, run-acceptance, or production mutation")
	}
	if evidence.OwnerApproved || evidence.PromotionExecuted || evidence.PackagePublished || evidence.VerifierContractSatisfied || evidence.RunAcceptanceRecordTouched || evidence.FullSubstrateClaimed || evidence.CompletionClaimed {
		return fmt.Errorf("base verifier readiness: evidence carries downstream execution or completion claims")
	}
	return nil
}
