package computerversion

import (
	"fmt"
	"strings"
)

const BasePromotionResultContractKind = "base_promotion_result_contract"

const BasePromotionResultBoundary = "promotion_result_without_promotion_execution_or_run_acceptance"

const BasePromotionResultScope = "promotion_readiness_to_blocked_or_noop_result_only"

const BasePromotionResultOutcomeBlocked = "blocked"

const BasePromotionResultOutcomeNoop = "noop"

// BasePromotionResultEvidence records a blocked or no-op promotion outcome after
// promotion-execution readiness. It does not execute promotion, synthesize run
// acceptance, mutate production state, or claim completion.
type BasePromotionResultEvidence struct {
	PromotionReadinessRef        string `json:"promotion_readiness_ref"`
	PromotionOutcomeRef          string `json:"promotion_outcome_ref"`
	PromotionOutcome             string `json:"promotion_outcome"`
	PromotionOutcomeReasonRef    string `json:"promotion_outcome_reason_ref"`
	OperatorDecisionRef          string `json:"operator_decision_ref"`
	PromotionAttemptRef          string `json:"promotion_attempt_ref"`
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

// BasePromotionResultContract records a blocked or no-op promotion result. It is
// not promotion execution, run acceptance, full-substrate proof, or completion
// authority.
type BasePromotionResultContract struct {
	Kind                          string          `json:"kind"`
	Version                       ComputerVersion `json:"version"`
	Boundary                      string          `json:"boundary"`
	Scope                         string          `json:"scope"`
	TypedArtifactProgramRef       string          `json:"typed_artifact_program_ref"`
	PromotionReadinessRef         string          `json:"promotion_readiness_ref"`
	PackagePublicationProofRef    string          `json:"package_publication_proof_ref"`
	PromotionCandidateRef         string          `json:"promotion_candidate_ref"`
	PromotionExecutionPlanRef     string          `json:"promotion_execution_plan_ref"`
	PromotionPreflightRef         string          `json:"promotion_preflight_ref"`
	PromotionOperatorPolicyRef    string          `json:"promotion_operator_policy_ref"`
	PromotionRollbackPlanRef      string          `json:"promotion_rollback_plan_ref"`
	RouteCutoverPlanRef           string          `json:"route_cutover_plan_ref"`
	LedgerFreshnessCheckRef       string          `json:"ledger_freshness_check_ref"`
	PromotionOutcomeRef           string          `json:"promotion_outcome_ref"`
	PromotionOutcome              string          `json:"promotion_outcome"`
	PromotionOutcomeReasonRef     string          `json:"promotion_outcome_reason_ref"`
	OperatorDecisionRef           string          `json:"operator_decision_ref"`
	PromotionAttemptRef           string          `json:"promotion_attempt_ref"`
	PromotionResultRecorded       bool            `json:"promotion_result_recorded"`
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

// BuildBasePromotionResultContract records a blocked or no-op promotion outcome
// after promotion-execution readiness. It does not promote, synthesize run
// acceptance, mutate production state, or claim completion.
func BuildBasePromotionResultContract(readiness BasePromotionExecutionReadinessContract, evidence BasePromotionResultEvidence) (BasePromotionResultContract, error) {
	if err := validateBasePromotionResultReadiness(readiness); err != nil {
		return BasePromotionResultContract{}, err
	}
	outcome, err := validateBasePromotionResultEvidence(evidence)
	if err != nil {
		return BasePromotionResultContract{}, err
	}

	return BasePromotionResultContract{
		Kind:                          BasePromotionResultContractKind,
		Version:                       readiness.Version,
		Boundary:                      BasePromotionResultBoundary,
		Scope:                         BasePromotionResultScope,
		TypedArtifactProgramRef:       string(readiness.Version.ArtifactProgramRef),
		PromotionReadinessRef:         strings.TrimSpace(evidence.PromotionReadinessRef),
		PackagePublicationProofRef:    strings.TrimSpace(readiness.PackagePublicationProofRef),
		PromotionCandidateRef:         strings.TrimSpace(readiness.PromotionCandidateRef),
		PromotionExecutionPlanRef:     strings.TrimSpace(readiness.PromotionExecutionPlanRef),
		PromotionPreflightRef:         strings.TrimSpace(readiness.PromotionPreflightRef),
		PromotionOperatorPolicyRef:    strings.TrimSpace(readiness.PromotionOperatorPolicyRef),
		PromotionRollbackPlanRef:      strings.TrimSpace(evidence.RollbackPlanRef),
		RouteCutoverPlanRef:           strings.TrimSpace(readiness.RouteCutoverPlanRef),
		LedgerFreshnessCheckRef:       strings.TrimSpace(readiness.LedgerFreshnessCheckRef),
		PromotionOutcomeRef:           strings.TrimSpace(evidence.PromotionOutcomeRef),
		PromotionOutcome:              outcome,
		PromotionOutcomeReasonRef:     strings.TrimSpace(evidence.PromotionOutcomeReasonRef),
		OperatorDecisionRef:           strings.TrimSpace(evidence.OperatorDecisionRef),
		PromotionAttemptRef:           strings.TrimSpace(evidence.PromotionAttemptRef),
		PromotionResultRecorded:       true,
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

func validateBasePromotionResultReadiness(readiness BasePromotionExecutionReadinessContract) error {
	if readiness.Kind != BasePromotionExecutionReadinessContractKind {
		return fmt.Errorf("base promotion result: promotion readiness kind is %q", readiness.Kind)
	}
	if readiness.Boundary != BasePromotionExecutionReadinessBoundary {
		return fmt.Errorf("base promotion result: promotion readiness boundary is %q", readiness.Boundary)
	}
	if readiness.Scope != BasePromotionExecutionReadinessScope {
		return fmt.Errorf("base promotion result: promotion readiness scope is %q", readiness.Scope)
	}
	if !readiness.Version.Valid() {
		return fmt.Errorf("base promotion result: promotion readiness version is invalid")
	}
	if !readiness.Version.ArtifactProgramRef.Valid() || ArtifactProgramRef(readiness.TypedArtifactProgramRef) != readiness.Version.ArtifactProgramRef {
		return fmt.Errorf("base promotion result: promotion readiness typed artifact-program ref is invalid")
	}
	if strings.TrimSpace(readiness.PackagePublicationProofRef) == "" || strings.TrimSpace(readiness.PromotionCandidateRef) == "" || strings.TrimSpace(readiness.PromotionExecutionPlanRef) == "" || strings.TrimSpace(readiness.PromotionPreflightRef) == "" || strings.TrimSpace(readiness.PromotionOperatorPolicyRef) == "" || strings.TrimSpace(readiness.PromotionRollbackPlanRef) == "" || strings.TrimSpace(readiness.RouteCutoverPlanRef) == "" || strings.TrimSpace(readiness.LedgerFreshnessCheckRef) == "" {
		return fmt.Errorf("base promotion result: promotion readiness refs are required")
	}
	if readiness.ReadinessStatus != BasePromotionExecutionReadinessStatusReady || !readiness.OwnerApproved || !readiness.PackagePublicationProof || !readiness.PromotionExecutionReady {
		return fmt.Errorf("base promotion result: promotion readiness must be ready")
	}
	if !readiness.PromotionProofRequired || !readiness.RunAcceptanceProofRequired || !readiness.FullSubstrateProofRequired {
		return fmt.Errorf("base promotion result: promotion readiness must preserve downstream proof requirements")
	}
	if readiness.PackagePublicationAllowed || readiness.PromotionAllowed || readiness.RunAcceptanceSynthesisAllowed {
		return fmt.Errorf("base promotion result: promotion readiness allows downstream execution")
	}
	if !readiness.NoPackagePublicationMutation || !readiness.NoPromotionMutation || !readiness.NoRunAcceptanceMutation || !readiness.NoProductionMutation {
		return fmt.Errorf("base promotion result: promotion readiness must prove no package publication, promotion, run-acceptance, or production mutation")
	}
	if readiness.PackagePublished || readiness.PromotionExecuted || readiness.RunAcceptanceRecordTouched || readiness.ProductionStateMutated || readiness.FullSubstrateClaimed || readiness.CompletionClaimed {
		return fmt.Errorf("base promotion result: promotion readiness carries downstream execution, production mutation, or completion claims")
	}
	return nil
}

func validateBasePromotionResultEvidence(evidence BasePromotionResultEvidence) (string, error) {
	outcome := strings.TrimSpace(evidence.PromotionOutcome)
	if outcome != BasePromotionResultOutcomeBlocked && outcome != BasePromotionResultOutcomeNoop {
		return "", fmt.Errorf("base promotion result: promotion outcome must be blocked or noop")
	}
	if strings.TrimSpace(evidence.PromotionReadinessRef) == "" || strings.TrimSpace(evidence.PromotionOutcomeRef) == "" || strings.TrimSpace(evidence.PromotionOutcomeReasonRef) == "" || strings.TrimSpace(evidence.OperatorDecisionRef) == "" || strings.TrimSpace(evidence.PromotionAttemptRef) == "" || strings.TrimSpace(evidence.RollbackPlanRef) == "" {
		return "", fmt.Errorf("base promotion result: promotion result refs are required")
	}
	if !evidence.NoPackagePublicationMutation || !evidence.NoPromotionMutation || !evidence.NoRunAcceptanceMutation || !evidence.NoProductionMutation {
		return "", fmt.Errorf("base promotion result: evidence must prove no package publication, promotion, run-acceptance, or production mutation")
	}
	if evidence.PackagePublished || evidence.PromotionExecuted || evidence.RunAcceptanceRecordTouched || evidence.ProductionStateMutated || evidence.FullSubstrateClaimed || evidence.CompletionClaimed {
		return "", fmt.Errorf("base promotion result: evidence carries downstream execution, production mutation, or completion claims")
	}
	return outcome, nil
}
