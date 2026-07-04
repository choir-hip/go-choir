package computerversion

import (
	"fmt"
	"strings"
)

const BasePackagePublicationProofContractKind = "base_package_publication_proof_contract"

const BasePackagePublicationProofBoundary = "package_publication_proof_without_promotion_or_run_acceptance"

const BasePackagePublicationProofScope = "publication_readiness_to_publication_proof_only"

const BasePackagePublicationProofStatusSatisfied = "package_publication_proof_satisfied_without_promotion_or_run_acceptance"

// BasePackagePublicationProofEvidence records proof refs after publication
// readiness. It does not publish a package, execute promotion, synthesize run
// acceptance, mutate production state, or claim completion.
type BasePackagePublicationProofEvidence struct {
	PublicationReadinessRef      string `json:"publication_readiness_ref"`
	PublicationProofRef          string `json:"publication_proof_ref"`
	PublishedPackageRef          string `json:"published_package_ref"`
	PackageDigestRef             string `json:"package_digest_ref"`
	PublicationReceiptRef        string `json:"publication_receipt_ref"`
	PublicationLedgerRef         string `json:"publication_ledger_ref"`
	PublicationReviewRef         string `json:"publication_review_ref"`
	RollbackPlanRef              string `json:"rollback_plan_ref"`
	NoPackagePublicationMutation bool   `json:"no_package_publication_mutation"`
	NoPromotionMutation          bool   `json:"no_promotion_mutation"`
	NoRunAcceptanceMutation      bool   `json:"no_run_acceptance_mutation"`
	NoProductionMutation         bool   `json:"no_production_mutation"`
	PackagePublished             bool   `json:"package_published"`
	PromotionExecuted            bool   `json:"promotion_executed"`
	RunAcceptanceRecordTouched   bool   `json:"run_acceptance_record_touched"`
	ProductionStateMutated       bool   `json:"production_state_mutated"`
	FullSubstrateClaimed         bool   `json:"full_substrate_claimed"`
	CompletionClaimed            bool   `json:"completion_claimed"`
}

// BasePackagePublicationProofContract records package-publication proof refs.
// It is not package-publication execution, promotion execution, run acceptance,
// full-substrate proof, or completion authority.
type BasePackagePublicationProofContract struct {
	Kind                          string          `json:"kind"`
	Version                       ComputerVersion `json:"version"`
	Boundary                      string          `json:"boundary"`
	Scope                         string          `json:"scope"`
	TypedArtifactProgramRef       string          `json:"typed_artifact_program_ref"`
	PublicationReadinessRef       string          `json:"publication_readiness_ref"`
	PromotionRollbackReviewRef    string          `json:"promotion_rollback_review_ref"`
	PackageManifestRef            string          `json:"package_manifest_ref"`
	PublicationPayloadRef         string          `json:"publication_payload_ref"`
	PublicationTargetRef          string          `json:"publication_target_ref"`
	PublicationPolicyRef          string          `json:"publication_policy_ref"`
	PublicationDryRunPlanRef      string          `json:"publication_dry_run_plan_ref"`
	PublicationProofRef           string          `json:"publication_proof_ref"`
	PublishedPackageRef           string          `json:"published_package_ref"`
	PackageDigestRef              string          `json:"package_digest_ref"`
	PublicationReceiptRef         string          `json:"publication_receipt_ref"`
	PublicationLedgerRef          string          `json:"publication_ledger_ref"`
	PublicationReviewRef          string          `json:"publication_review_ref"`
	RollbackPlanRef               string          `json:"rollback_plan_ref"`
	ProofStatus                   string          `json:"proof_status"`
	OwnerApproved                 bool            `json:"owner_approved"`
	PromotionRollbackReviewReady  bool            `json:"promotion_rollback_review_ready"`
	PackagePublicationReady       bool            `json:"package_publication_ready"`
	PackagePublicationProof       bool            `json:"package_publication_proof"`
	PromotionProofRequired        bool            `json:"promotion_proof_required"`
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
	ProductionStateMutated        bool            `json:"production_state_mutated"`
	FullSubstrateClaimed          bool            `json:"full_substrate_claimed"`
	CompletionClaimed             bool            `json:"completion_claimed"`
}

// BuildBasePackagePublicationProofContract records package-publication proof
// refs after publication readiness. It does not publish, promote, synthesize
// run acceptance, mutate production state, or claim completion.
func BuildBasePackagePublicationProofContract(readiness BasePackagePublicationReadinessContract, evidence BasePackagePublicationProofEvidence) (BasePackagePublicationProofContract, error) {
	if err := validateBasePackagePublicationProofReadiness(readiness); err != nil {
		return BasePackagePublicationProofContract{}, err
	}
	if err := validateBasePackagePublicationProofEvidence(evidence); err != nil {
		return BasePackagePublicationProofContract{}, err
	}

	return BasePackagePublicationProofContract{
		Kind:                          BasePackagePublicationProofContractKind,
		Version:                       readiness.Version,
		Boundary:                      BasePackagePublicationProofBoundary,
		Scope:                         BasePackagePublicationProofScope,
		TypedArtifactProgramRef:       string(readiness.Version.ArtifactProgramRef),
		PublicationReadinessRef:       strings.TrimSpace(evidence.PublicationReadinessRef),
		PromotionRollbackReviewRef:    strings.TrimSpace(readiness.PromotionRollbackReviewRef),
		PackageManifestRef:            strings.TrimSpace(readiness.PackageManifestRef),
		PublicationPayloadRef:         strings.TrimSpace(readiness.PublicationPayloadRef),
		PublicationTargetRef:          strings.TrimSpace(readiness.PublicationTargetRef),
		PublicationPolicyRef:          strings.TrimSpace(readiness.PublicationPolicyRef),
		PublicationDryRunPlanRef:      strings.TrimSpace(readiness.PublicationDryRunPlanRef),
		PublicationProofRef:           strings.TrimSpace(evidence.PublicationProofRef),
		PublishedPackageRef:           strings.TrimSpace(evidence.PublishedPackageRef),
		PackageDigestRef:              strings.TrimSpace(evidence.PackageDigestRef),
		PublicationReceiptRef:         strings.TrimSpace(evidence.PublicationReceiptRef),
		PublicationLedgerRef:          strings.TrimSpace(evidence.PublicationLedgerRef),
		PublicationReviewRef:          strings.TrimSpace(evidence.PublicationReviewRef),
		RollbackPlanRef:               strings.TrimSpace(evidence.RollbackPlanRef),
		ProofStatus:                   BasePackagePublicationProofStatusSatisfied,
		OwnerApproved:                 true,
		PromotionRollbackReviewReady:  true,
		PackagePublicationReady:       true,
		PackagePublicationProof:       true,
		PromotionProofRequired:        true,
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

func validateBasePackagePublicationProofReadiness(readiness BasePackagePublicationReadinessContract) error {
	if readiness.Kind != BasePackagePublicationReadinessContractKind {
		return fmt.Errorf("base package publication proof: publication readiness kind is %q", readiness.Kind)
	}
	if readiness.Boundary != BasePackagePublicationReadinessBoundary {
		return fmt.Errorf("base package publication proof: publication readiness boundary is %q", readiness.Boundary)
	}
	if readiness.Scope != BasePackagePublicationReadinessScope {
		return fmt.Errorf("base package publication proof: publication readiness scope is %q", readiness.Scope)
	}
	if !readiness.Version.Valid() {
		return fmt.Errorf("base package publication proof: publication readiness version is invalid")
	}
	if !readiness.Version.ArtifactProgramRef.Valid() || ArtifactProgramRef(readiness.TypedArtifactProgramRef) != readiness.Version.ArtifactProgramRef {
		return fmt.Errorf("base package publication proof: publication readiness typed artifact-program ref is invalid")
	}
	if strings.TrimSpace(readiness.PromotionRollbackReviewRef) == "" || strings.TrimSpace(readiness.PackageManifestRef) == "" || strings.TrimSpace(readiness.PublicationPayloadRef) == "" || strings.TrimSpace(readiness.PublicationTargetRef) == "" || strings.TrimSpace(readiness.PublicationPolicyRef) == "" || strings.TrimSpace(readiness.PublicationDryRunPlanRef) == "" || strings.TrimSpace(readiness.RollbackPlanRef) == "" {
		return fmt.Errorf("base package publication proof: publication readiness refs are required")
	}
	if readiness.ReadinessStatus != BasePackagePublicationReadinessStatusReady || !readiness.OwnerApproved || !readiness.PromotionRollbackReviewReady || !readiness.PackagePublicationReady {
		return fmt.Errorf("base package publication proof: publication readiness must be ready")
	}
	if !readiness.RunAcceptanceProofRequired || !readiness.FullSubstrateProofRequired {
		return fmt.Errorf("base package publication proof: publication readiness must preserve downstream proof requirements")
	}
	if readiness.PackagePublicationAllowed || readiness.PromotionAllowed || readiness.RunAcceptanceSynthesisAllowed {
		return fmt.Errorf("base package publication proof: publication readiness allows downstream execution")
	}
	if !readiness.NoPackagePublicationMutation || !readiness.NoPromotionMutation || !readiness.NoRunAcceptanceMutation || !readiness.NoProductionMutation {
		return fmt.Errorf("base package publication proof: publication readiness must prove no package publication, promotion, run-acceptance, or production mutation")
	}
	if readiness.PackagePublished || readiness.PromotionExecuted || readiness.RunAcceptanceRecordTouched || readiness.FullSubstrateClaimed || readiness.CompletionClaimed {
		return fmt.Errorf("base package publication proof: publication readiness carries downstream execution or completion claims")
	}
	return nil
}

func validateBasePackagePublicationProofEvidence(evidence BasePackagePublicationProofEvidence) error {
	if strings.TrimSpace(evidence.PublicationReadinessRef) == "" || strings.TrimSpace(evidence.PublicationProofRef) == "" || strings.TrimSpace(evidence.PublishedPackageRef) == "" || strings.TrimSpace(evidence.PackageDigestRef) == "" || strings.TrimSpace(evidence.PublicationReceiptRef) == "" || strings.TrimSpace(evidence.PublicationLedgerRef) == "" || strings.TrimSpace(evidence.PublicationReviewRef) == "" || strings.TrimSpace(evidence.RollbackPlanRef) == "" {
		return fmt.Errorf("base package publication proof: publication proof refs are required")
	}
	if !evidence.NoPackagePublicationMutation || !evidence.NoPromotionMutation || !evidence.NoRunAcceptanceMutation || !evidence.NoProductionMutation {
		return fmt.Errorf("base package publication proof: evidence must prove no package publication, promotion, run-acceptance, or production mutation")
	}
	if evidence.PackagePublished || evidence.PromotionExecuted || evidence.RunAcceptanceRecordTouched || evidence.ProductionStateMutated || evidence.FullSubstrateClaimed || evidence.CompletionClaimed {
		return fmt.Errorf("base package publication proof: evidence carries downstream execution, production mutation, or completion claims")
	}
	return nil
}
