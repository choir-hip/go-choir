package computerversion

import (
	"fmt"
	"strings"
)

const BasePostSmokeHandoffReadinessContractKind = "base_post_smoke_handoff_readiness_contract"

const BasePostSmokeHandoffReadinessBoundary = "post_staging_smoke_handoff_blocked_until_owner_review_publication_promotion_and_run_acceptance"

const BasePostSmokeHandoffReadinessScope = "staging_smoke_to_downstream_readiness_without_execution"

const BasePostSmokeHandoffReadinessStatusBlocked = "blocked_until_owner_review_publication_promotion_and_run_acceptance"

const BasePostSmokePrerequisiteOwnerReview = "owner_review"
const BasePostSmokePrerequisitePromotionRollbackReview = "promotion_rollback_review"
const BasePostSmokePrerequisitePackagePublicationReview = "package_publication_review"
const BasePostSmokePrerequisiteVerifierContractReview = "verifier_contract_review"
const BasePostSmokePrerequisiteRunAcceptanceSynthesisReview = "run_acceptance_synthesis_review"

// BasePostSmokeHandoffReadinessEvidence records refs needed after staging smoke
// and before any owner approval, package publication, promotion/rollback, or
// run-acceptance synthesis. It must not perform those downstream actions.
type BasePostSmokeHandoffReadinessEvidence struct {
	StagingSmokeEvidenceRef       string `json:"staging_smoke_evidence_ref"`
	OwnerReviewPlanRef            string `json:"owner_review_plan_ref"`
	PromotionRollbackPlanRef      string `json:"promotion_rollback_plan_ref"`
	PackagePublicationPlanRef     string `json:"package_publication_plan_ref"`
	VerifierContractPlanRef       string `json:"verifier_contract_plan_ref"`
	RunAcceptanceSynthesisPlanRef string `json:"run_acceptance_synthesis_plan_ref"`
	RollbackPlanRef               string `json:"rollback_plan_ref"`
	NoOwnerApprovalMutation       bool   `json:"no_owner_approval_mutation"`
	NoPromotionMutation           bool   `json:"no_promotion_mutation"`
	NoPackagePublicationMutation  bool   `json:"no_package_publication_mutation"`
	NoRunAcceptanceMutation       bool   `json:"no_run_acceptance_mutation"`
	NoProductionMutation          bool   `json:"no_production_mutation"`
	OwnerApproved                 bool   `json:"owner_approved"`
	PromotionExecuted             bool   `json:"promotion_executed"`
	PackagePublished              bool   `json:"package_published"`
	RunAcceptanceRecordTouched    bool   `json:"run_acceptance_record_touched"`
	VerifierContractSatisfied     bool   `json:"verifier_contract_satisfied"`
	FullSubstrateClaimed          bool   `json:"full_substrate_claimed"`
	CompletionClaimed             bool   `json:"completion_claimed"`
}

// BasePostSmokeHandoffReadinessContract names the blocked handoff after staging
// smoke. It preserves the downstream authority boundaries instead of executing
// owner approval, package publication, promotion, or run acceptance.
type BasePostSmokeHandoffReadinessContract struct {
	Kind                            string          `json:"kind"`
	Version                         ComputerVersion `json:"version"`
	Boundary                        string          `json:"boundary"`
	Scope                           string          `json:"scope"`
	TypedArtifactProgramRef         string          `json:"typed_artifact_program_ref"`
	StagingSmokeEvidenceRef         string          `json:"staging_smoke_evidence_ref"`
	ProductPathProbeRef             string          `json:"product_path_probe_ref"`
	BuildIdentity                   string          `json:"build_identity"`
	RouteIdentity                   string          `json:"route_identity"`
	OwnerReviewPlanRef              string          `json:"owner_review_plan_ref"`
	PromotionRollbackPlanRef        string          `json:"promotion_rollback_plan_ref"`
	PackagePublicationPlanRef       string          `json:"package_publication_plan_ref"`
	VerifierContractPlanRef         string          `json:"verifier_contract_plan_ref"`
	RunAcceptanceSynthesisPlanRef   string          `json:"run_acceptance_synthesis_plan_ref"`
	RollbackPlanRef                 string          `json:"rollback_plan_ref"`
	ReadinessStatus                 string          `json:"readiness_status"`
	RequiredPrerequisites           []string        `json:"required_prerequisites"`
	OwnerReviewRequired             bool            `json:"owner_review_required"`
	PromotionRollbackReviewRequired bool            `json:"promotion_rollback_review_required"`
	PackagePublicationProofRequired bool            `json:"package_publication_proof_required"`
	VerifierContractProofRequired   bool            `json:"verifier_contract_proof_required"`
	RunAcceptanceProofRequired      bool            `json:"run_acceptance_proof_required"`
	FullSubstrateProofRequired      bool            `json:"full_substrate_proof_required"`
	OwnerApprovalAllowed            bool            `json:"owner_approval_allowed"`
	PromotionAllowed                bool            `json:"promotion_allowed"`
	PackagePublicationAllowed       bool            `json:"package_publication_allowed"`
	RunAcceptanceSynthesisAllowed   bool            `json:"run_acceptance_synthesis_allowed"`
	NoOwnerApprovalMutation         bool            `json:"no_owner_approval_mutation"`
	NoPromotionMutation             bool            `json:"no_promotion_mutation"`
	NoPackagePublicationMutation    bool            `json:"no_package_publication_mutation"`
	NoRunAcceptanceMutation         bool            `json:"no_run_acceptance_mutation"`
	NoProductionMutation            bool            `json:"no_production_mutation"`
	OwnerApproved                   bool            `json:"owner_approved"`
	PromotionExecuted               bool            `json:"promotion_executed"`
	PackagePublished                bool            `json:"package_published"`
	RunAcceptanceRecordTouched      bool            `json:"run_acceptance_record_touched"`
	VerifierContractSatisfied       bool            `json:"verifier_contract_satisfied"`
	FullSubstrateClaimed            bool            `json:"full_substrate_claimed"`
	CompletionClaimed               bool            `json:"completion_claimed"`
}

// BuildBasePostSmokeHandoffReadinessContract consumes staging-smoke evidence and
// creates a blocked readiness handoff for the next authority boundary. It does
// not approve, promote, publish, synthesize run acceptance, or claim completion.
func BuildBasePostSmokeHandoffReadinessContract(smoke BaseStagingSmokeEvidenceContract, evidence BasePostSmokeHandoffReadinessEvidence) (BasePostSmokeHandoffReadinessContract, error) {
	if err := validateBasePostSmokeHandoffSmoke(smoke); err != nil {
		return BasePostSmokeHandoffReadinessContract{}, err
	}
	if err := validateBasePostSmokeHandoffEvidence(evidence); err != nil {
		return BasePostSmokeHandoffReadinessContract{}, err
	}

	return BasePostSmokeHandoffReadinessContract{
		Kind:                            BasePostSmokeHandoffReadinessContractKind,
		Version:                         smoke.Version,
		Boundary:                        BasePostSmokeHandoffReadinessBoundary,
		Scope:                           BasePostSmokeHandoffReadinessScope,
		TypedArtifactProgramRef:         string(smoke.Version.ArtifactProgramRef),
		StagingSmokeEvidenceRef:         strings.TrimSpace(evidence.StagingSmokeEvidenceRef),
		ProductPathProbeRef:             strings.TrimSpace(smoke.ProductPathProbeRef),
		BuildIdentity:                   strings.TrimSpace(smoke.BuildIdentity),
		RouteIdentity:                   strings.TrimSpace(smoke.RouteIdentity),
		OwnerReviewPlanRef:              strings.TrimSpace(evidence.OwnerReviewPlanRef),
		PromotionRollbackPlanRef:        strings.TrimSpace(evidence.PromotionRollbackPlanRef),
		PackagePublicationPlanRef:       strings.TrimSpace(evidence.PackagePublicationPlanRef),
		VerifierContractPlanRef:         strings.TrimSpace(evidence.VerifierContractPlanRef),
		RunAcceptanceSynthesisPlanRef:   strings.TrimSpace(evidence.RunAcceptanceSynthesisPlanRef),
		RollbackPlanRef:                 strings.TrimSpace(evidence.RollbackPlanRef),
		ReadinessStatus:                 BasePostSmokeHandoffReadinessStatusBlocked,
		RequiredPrerequisites:           basePostSmokeHandoffPrerequisites(),
		OwnerReviewRequired:             true,
		PromotionRollbackReviewRequired: true,
		PackagePublicationProofRequired: true,
		VerifierContractProofRequired:   true,
		RunAcceptanceProofRequired:      true,
		FullSubstrateProofRequired:      true,
		OwnerApprovalAllowed:            false,
		PromotionAllowed:                false,
		PackagePublicationAllowed:       false,
		RunAcceptanceSynthesisAllowed:   false,
		NoOwnerApprovalMutation:         true,
		NoPromotionMutation:             true,
		NoPackagePublicationMutation:    true,
		NoRunAcceptanceMutation:         true,
		NoProductionMutation:            true,
	}, nil
}

func validateBasePostSmokeHandoffSmoke(smoke BaseStagingSmokeEvidenceContract) error {
	if smoke.Kind != BaseStagingSmokeEvidenceContractKind {
		return fmt.Errorf("base post-smoke handoff: smoke kind is %q", smoke.Kind)
	}
	if smoke.Boundary != BaseStagingSmokeEvidenceBoundary {
		return fmt.Errorf("base post-smoke handoff: smoke boundary is %q", smoke.Boundary)
	}
	if smoke.Scope != BaseStagingSmokeEvidenceScope {
		return fmt.Errorf("base post-smoke handoff: smoke scope is %q", smoke.Scope)
	}
	if !smoke.Version.Valid() {
		return fmt.Errorf("base post-smoke handoff: smoke version is invalid")
	}
	if !smoke.Version.ArtifactProgramRef.Valid() || ArtifactProgramRef(smoke.TypedArtifactProgramRef) != smoke.Version.ArtifactProgramRef {
		return fmt.Errorf("base post-smoke handoff: smoke typed artifact-program ref is invalid")
	}
	if strings.TrimSpace(smoke.StagingReadinessRef) == "" || strings.TrimSpace(smoke.ProductPathProbeRef) == "" || strings.TrimSpace(smoke.ProductPathURL) == "" || strings.TrimSpace(smoke.BuildIdentity) == "" || strings.TrimSpace(smoke.RouteIdentity) == "" || strings.TrimSpace(smoke.RollbackPlanRef) == "" {
		return fmt.Errorf("base post-smoke handoff: smoke refs are required")
	}
	if smoke.HealthStatus != BaseStagingSmokeHealthPassed || !smoke.StagingSmokePassed || !smoke.ProductPathObserved || !smoke.AuthenticatedProductPath || !smoke.BuildIdentityMatched || !smoke.RouteIdentityMatched {
		return fmt.Errorf("base post-smoke handoff: smoke evidence must have passed product-path health and identity checks")
	}
	if !smoke.PromotionProofRequired || !smoke.PackagePublicationProofRequired || !smoke.RunAcceptanceProofRequired || !smoke.FullSubstrateProofRequired {
		return fmt.Errorf("base post-smoke handoff: smoke evidence must preserve downstream proof requirements")
	}
	if !smoke.NoPromotionMutation || !smoke.NoPackagePublicationMutation || !smoke.NoRunAcceptanceMutation || !smoke.NoProductionMutation {
		return fmt.Errorf("base post-smoke handoff: smoke evidence must prove no downstream or production mutation")
	}
	if smoke.PromotionClaimed || smoke.PackagePublicationClaimed || smoke.RunAcceptanceRecordTouched || smoke.FullSubstrateClaimed || smoke.CompletionClaimed {
		return fmt.Errorf("base post-smoke handoff: smoke evidence carries downstream or completion claims")
	}
	return nil
}

func validateBasePostSmokeHandoffEvidence(evidence BasePostSmokeHandoffReadinessEvidence) error {
	if strings.TrimSpace(evidence.StagingSmokeEvidenceRef) == "" || strings.TrimSpace(evidence.OwnerReviewPlanRef) == "" || strings.TrimSpace(evidence.PromotionRollbackPlanRef) == "" || strings.TrimSpace(evidence.PackagePublicationPlanRef) == "" || strings.TrimSpace(evidence.VerifierContractPlanRef) == "" || strings.TrimSpace(evidence.RunAcceptanceSynthesisPlanRef) == "" || strings.TrimSpace(evidence.RollbackPlanRef) == "" {
		return fmt.Errorf("base post-smoke handoff: prerequisite refs are required")
	}
	if !evidence.NoOwnerApprovalMutation || !evidence.NoPromotionMutation || !evidence.NoPackagePublicationMutation || !evidence.NoRunAcceptanceMutation || !evidence.NoProductionMutation {
		return fmt.Errorf("base post-smoke handoff: evidence must prove no owner approval, promotion, package publication, run-acceptance, or production mutation")
	}
	if evidence.OwnerApproved || evidence.PromotionExecuted || evidence.PackagePublished || evidence.RunAcceptanceRecordTouched || evidence.VerifierContractSatisfied || evidence.FullSubstrateClaimed || evidence.CompletionClaimed {
		return fmt.Errorf("base post-smoke handoff: evidence carries downstream execution or completion claims")
	}
	return nil
}

func basePostSmokeHandoffPrerequisites() []string {
	return []string{
		BasePostSmokePrerequisiteOwnerReview,
		BasePostSmokePrerequisitePromotionRollbackReview,
		BasePostSmokePrerequisitePackagePublicationReview,
		BasePostSmokePrerequisiteVerifierContractReview,
		BasePostSmokePrerequisiteRunAcceptanceSynthesisReview,
	}
}
