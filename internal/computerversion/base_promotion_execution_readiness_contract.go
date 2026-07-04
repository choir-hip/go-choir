package computerversion

import (
	"fmt"
	"strings"
)

const BasePromotionExecutionReadinessContractKind = "base_promotion_execution_readiness_contract"

const BasePromotionExecutionReadinessBoundary = "promotion_execution_readiness_without_promotion_or_run_acceptance"

const BasePromotionExecutionReadinessScope = "package_publication_proof_to_promotion_execution_prerequisites_only"

const BasePromotionExecutionReadinessStatusReady = "ready_for_promotion_execution_not_executed"

// BasePromotionExecutionReadinessEvidence records promotion execution
// prerequisites after package-publication proof. It does not execute promotion,
// synthesize run acceptance, mutate production state, or claim completion.
type BasePromotionExecutionReadinessEvidence struct {
	PackagePublicationProofRef   string `json:"package_publication_proof_ref"`
	PromotionCandidateRef        string `json:"promotion_candidate_ref"`
	PromotionExecutionPlanRef    string `json:"promotion_execution_plan_ref"`
	PromotionPreflightRef        string `json:"promotion_preflight_ref"`
	PromotionOperatorPolicyRef   string `json:"promotion_operator_policy_ref"`
	PromotionRollbackPlanRef     string `json:"promotion_rollback_plan_ref"`
	RouteCutoverPlanRef          string `json:"route_cutover_plan_ref"`
	LedgerFreshnessCheckRef      string `json:"ledger_freshness_check_ref"`
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

// BasePromotionExecutionReadinessContract records promotion execution
// prerequisites. It is not promotion execution, run acceptance,
// full-substrate proof, or completion authority.
type BasePromotionExecutionReadinessContract struct {
	Kind                          string          `json:"kind"`
	Version                       ComputerVersion `json:"version"`
	Boundary                      string          `json:"boundary"`
	Scope                         string          `json:"scope"`
	TypedArtifactProgramRef       string          `json:"typed_artifact_program_ref"`
	PackagePublicationProofRef    string          `json:"package_publication_proof_ref"`
	PublicationReadinessRef       string          `json:"publication_readiness_ref"`
	PublicationProofRef           string          `json:"publication_proof_ref"`
	PublishedPackageRef           string          `json:"published_package_ref"`
	PackageDigestRef              string          `json:"package_digest_ref"`
	PublicationLedgerRef          string          `json:"publication_ledger_ref"`
	PromotionCandidateRef         string          `json:"promotion_candidate_ref"`
	PromotionExecutionPlanRef     string          `json:"promotion_execution_plan_ref"`
	PromotionPreflightRef         string          `json:"promotion_preflight_ref"`
	PromotionOperatorPolicyRef    string          `json:"promotion_operator_policy_ref"`
	PromotionRollbackPlanRef      string          `json:"promotion_rollback_plan_ref"`
	RouteCutoverPlanRef           string          `json:"route_cutover_plan_ref"`
	LedgerFreshnessCheckRef       string          `json:"ledger_freshness_check_ref"`
	ReadinessStatus               string          `json:"readiness_status"`
	OwnerApproved                 bool            `json:"owner_approved"`
	PackagePublicationProof       bool            `json:"package_publication_proof"`
	PromotionExecutionReady       bool            `json:"promotion_execution_ready"`
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

// BuildBasePromotionExecutionReadinessContract records promotion execution
// readiness after package-publication proof. It does not promote, synthesize run
// acceptance, mutate production state, or claim completion.
func BuildBasePromotionExecutionReadinessContract(proof BasePackagePublicationProofContract, evidence BasePromotionExecutionReadinessEvidence) (BasePromotionExecutionReadinessContract, error) {
	if err := validateBasePromotionExecutionReadinessProof(proof); err != nil {
		return BasePromotionExecutionReadinessContract{}, err
	}
	if err := validateBasePromotionExecutionReadinessEvidence(evidence); err != nil {
		return BasePromotionExecutionReadinessContract{}, err
	}

	return BasePromotionExecutionReadinessContract{
		Kind:                          BasePromotionExecutionReadinessContractKind,
		Version:                       proof.Version,
		Boundary:                      BasePromotionExecutionReadinessBoundary,
		Scope:                         BasePromotionExecutionReadinessScope,
		TypedArtifactProgramRef:       string(proof.Version.ArtifactProgramRef),
		PackagePublicationProofRef:    strings.TrimSpace(evidence.PackagePublicationProofRef),
		PublicationReadinessRef:       strings.TrimSpace(proof.PublicationReadinessRef),
		PublicationProofRef:           strings.TrimSpace(proof.PublicationProofRef),
		PublishedPackageRef:           strings.TrimSpace(proof.PublishedPackageRef),
		PackageDigestRef:              strings.TrimSpace(proof.PackageDigestRef),
		PublicationLedgerRef:          strings.TrimSpace(proof.PublicationLedgerRef),
		PromotionCandidateRef:         strings.TrimSpace(evidence.PromotionCandidateRef),
		PromotionExecutionPlanRef:     strings.TrimSpace(evidence.PromotionExecutionPlanRef),
		PromotionPreflightRef:         strings.TrimSpace(evidence.PromotionPreflightRef),
		PromotionOperatorPolicyRef:    strings.TrimSpace(evidence.PromotionOperatorPolicyRef),
		PromotionRollbackPlanRef:      strings.TrimSpace(evidence.PromotionRollbackPlanRef),
		RouteCutoverPlanRef:           strings.TrimSpace(evidence.RouteCutoverPlanRef),
		LedgerFreshnessCheckRef:       strings.TrimSpace(evidence.LedgerFreshnessCheckRef),
		ReadinessStatus:               BasePromotionExecutionReadinessStatusReady,
		OwnerApproved:                 true,
		PackagePublicationProof:       true,
		PromotionExecutionReady:       true,
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

func validateBasePromotionExecutionReadinessProof(proof BasePackagePublicationProofContract) error {
	if proof.Kind != BasePackagePublicationProofContractKind {
		return fmt.Errorf("base promotion execution readiness: package-publication proof kind is %q", proof.Kind)
	}
	if proof.Boundary != BasePackagePublicationProofBoundary {
		return fmt.Errorf("base promotion execution readiness: package-publication proof boundary is %q", proof.Boundary)
	}
	if proof.Scope != BasePackagePublicationProofScope {
		return fmt.Errorf("base promotion execution readiness: package-publication proof scope is %q", proof.Scope)
	}
	if !proof.Version.Valid() {
		return fmt.Errorf("base promotion execution readiness: package-publication proof version is invalid")
	}
	if !proof.Version.ArtifactProgramRef.Valid() || ArtifactProgramRef(proof.TypedArtifactProgramRef) != proof.Version.ArtifactProgramRef {
		return fmt.Errorf("base promotion execution readiness: package-publication proof typed artifact-program ref is invalid")
	}
	if strings.TrimSpace(proof.PublicationReadinessRef) == "" || strings.TrimSpace(proof.PublicationProofRef) == "" || strings.TrimSpace(proof.PublishedPackageRef) == "" || strings.TrimSpace(proof.PackageDigestRef) == "" || strings.TrimSpace(proof.PublicationLedgerRef) == "" || strings.TrimSpace(proof.RollbackPlanRef) == "" {
		return fmt.Errorf("base promotion execution readiness: package-publication proof refs are required")
	}
	if proof.ProofStatus != BasePackagePublicationProofStatusSatisfied || !proof.OwnerApproved || !proof.PackagePublicationProof {
		return fmt.Errorf("base promotion execution readiness: package-publication proof must be satisfied")
	}
	if !proof.PromotionProofRequired || !proof.RunAcceptanceProofRequired || !proof.FullSubstrateProofRequired {
		return fmt.Errorf("base promotion execution readiness: package-publication proof must preserve downstream proof requirements")
	}
	if proof.PackagePublicationAllowed || proof.PromotionAllowed || proof.RunAcceptanceSynthesisAllowed {
		return fmt.Errorf("base promotion execution readiness: package-publication proof allows downstream execution")
	}
	if !proof.NoPackagePublicationMutation || !proof.NoPromotionMutation || !proof.NoRunAcceptanceMutation || !proof.NoProductionMutation {
		return fmt.Errorf("base promotion execution readiness: package-publication proof must prove no package publication, promotion, run-acceptance, or production mutation")
	}
	if proof.PackagePublished || proof.PromotionExecuted || proof.RunAcceptanceRecordTouched || proof.ProductionStateMutated || proof.FullSubstrateClaimed || proof.CompletionClaimed {
		return fmt.Errorf("base promotion execution readiness: package-publication proof carries downstream execution, production mutation, or completion claims")
	}
	return nil
}

func validateBasePromotionExecutionReadinessEvidence(evidence BasePromotionExecutionReadinessEvidence) error {
	if strings.TrimSpace(evidence.PackagePublicationProofRef) == "" || strings.TrimSpace(evidence.PromotionCandidateRef) == "" || strings.TrimSpace(evidence.PromotionExecutionPlanRef) == "" || strings.TrimSpace(evidence.PromotionPreflightRef) == "" || strings.TrimSpace(evidence.PromotionOperatorPolicyRef) == "" || strings.TrimSpace(evidence.PromotionRollbackPlanRef) == "" || strings.TrimSpace(evidence.RouteCutoverPlanRef) == "" || strings.TrimSpace(evidence.LedgerFreshnessCheckRef) == "" {
		return fmt.Errorf("base promotion execution readiness: promotion readiness refs are required")
	}
	if !evidence.NoPackagePublicationMutation || !evidence.NoPromotionMutation || !evidence.NoRunAcceptanceMutation || !evidence.NoProductionMutation {
		return fmt.Errorf("base promotion execution readiness: evidence must prove no package publication, promotion, run-acceptance, or production mutation")
	}
	if evidence.PackagePublished || evidence.PromotionExecuted || evidence.RunAcceptanceRecordTouched || evidence.ProductionStateMutated || evidence.FullSubstrateClaimed || evidence.CompletionClaimed {
		return fmt.Errorf("base promotion execution readiness: evidence carries downstream execution, production mutation, or completion claims")
	}
	return nil
}
