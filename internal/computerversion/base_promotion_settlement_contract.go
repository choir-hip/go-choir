package computerversion

import (
	"fmt"
	"strings"
)

const BasePromotionSettlementContractKind = "base_promotion_settlement_contract"

const BasePromotionSettlementBoundary = "promotion_settlement_without_promotion_execution_or_run_acceptance"

const BasePromotionSettlementScope = "promotion_result_to_operator_settlement_only"

const BasePromotionSettlementDecisionBlocked = "settle_blocked_result"

const BasePromotionSettlementDecisionNoop = "settle_noop_result"

// BasePromotionSettlementEvidence records operator settlement of a blocked or
// no-op promotion result. It does not execute promotion, synthesize run
// acceptance, mutate production state, or claim completion.
type BasePromotionSettlementEvidence struct {
	PromotionResultRef           string `json:"promotion_result_ref"`
	SettlementRef                string `json:"settlement_ref"`
	SettlementDecision           string `json:"settlement_decision"`
	SettlementReasonRef          string `json:"settlement_reason_ref"`
	OperatorReviewRef            string `json:"operator_review_ref"`
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

// BasePromotionSettlementContract records operator settlement of a blocked or
// no-op promotion result. It is not promotion execution, run acceptance,
// full-substrate proof, or completion authority.
type BasePromotionSettlementContract struct {
	Kind                          string          `json:"kind"`
	Version                       ComputerVersion `json:"version"`
	Boundary                      string          `json:"boundary"`
	Scope                         string          `json:"scope"`
	TypedArtifactProgramRef       string          `json:"typed_artifact_program_ref"`
	PromotionResultRef            string          `json:"promotion_result_ref"`
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
	PromotionAttemptRef           string          `json:"promotion_attempt_ref"`
	SettlementRef                 string          `json:"settlement_ref"`
	SettlementDecision            string          `json:"settlement_decision"`
	SettlementReasonRef           string          `json:"settlement_reason_ref"`
	OperatorReviewRef             string          `json:"operator_review_ref"`
	PromotionResultRecorded       bool            `json:"promotion_result_recorded"`
	PromotionResultSettled        bool            `json:"promotion_result_settled"`
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

// BuildBasePromotionSettlementContract records operator settlement of a blocked
// or no-op promotion result. It does not promote, synthesize run acceptance,
// mutate production state, or claim completion.
func BuildBasePromotionSettlementContract(result BasePromotionResultContract, evidence BasePromotionSettlementEvidence) (BasePromotionSettlementContract, error) {
	if err := validateBasePromotionSettlementResult(result); err != nil {
		return BasePromotionSettlementContract{}, err
	}
	decision, err := validateBasePromotionSettlementEvidence(result, evidence)
	if err != nil {
		return BasePromotionSettlementContract{}, err
	}

	return BasePromotionSettlementContract{
		Kind:                          BasePromotionSettlementContractKind,
		Version:                       result.Version,
		Boundary:                      BasePromotionSettlementBoundary,
		Scope:                         BasePromotionSettlementScope,
		TypedArtifactProgramRef:       string(result.Version.ArtifactProgramRef),
		PromotionResultRef:            strings.TrimSpace(evidence.PromotionResultRef),
		PromotionReadinessRef:         strings.TrimSpace(result.PromotionReadinessRef),
		PackagePublicationProofRef:    strings.TrimSpace(result.PackagePublicationProofRef),
		PromotionCandidateRef:         strings.TrimSpace(result.PromotionCandidateRef),
		PromotionExecutionPlanRef:     strings.TrimSpace(result.PromotionExecutionPlanRef),
		PromotionPreflightRef:         strings.TrimSpace(result.PromotionPreflightRef),
		PromotionOperatorPolicyRef:    strings.TrimSpace(result.PromotionOperatorPolicyRef),
		PromotionRollbackPlanRef:      strings.TrimSpace(evidence.RollbackPlanRef),
		RouteCutoverPlanRef:           strings.TrimSpace(result.RouteCutoverPlanRef),
		LedgerFreshnessCheckRef:       strings.TrimSpace(result.LedgerFreshnessCheckRef),
		PromotionOutcomeRef:           strings.TrimSpace(result.PromotionOutcomeRef),
		PromotionOutcome:              strings.TrimSpace(result.PromotionOutcome),
		PromotionOutcomeReasonRef:     strings.TrimSpace(result.PromotionOutcomeReasonRef),
		PromotionAttemptRef:           strings.TrimSpace(result.PromotionAttemptRef),
		SettlementRef:                 strings.TrimSpace(evidence.SettlementRef),
		SettlementDecision:            decision,
		SettlementReasonRef:           strings.TrimSpace(evidence.SettlementReasonRef),
		OperatorReviewRef:             strings.TrimSpace(evidence.OperatorReviewRef),
		PromotionResultRecorded:       true,
		PromotionResultSettled:        true,
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

func validateBasePromotionSettlementResult(result BasePromotionResultContract) error {
	if result.Kind != BasePromotionResultContractKind {
		return fmt.Errorf("base promotion settlement: promotion result kind is %q", result.Kind)
	}
	if result.Boundary != BasePromotionResultBoundary {
		return fmt.Errorf("base promotion settlement: promotion result boundary is %q", result.Boundary)
	}
	if result.Scope != BasePromotionResultScope {
		return fmt.Errorf("base promotion settlement: promotion result scope is %q", result.Scope)
	}
	if !result.Version.Valid() {
		return fmt.Errorf("base promotion settlement: promotion result version is invalid")
	}
	if !result.Version.ArtifactProgramRef.Valid() || ArtifactProgramRef(result.TypedArtifactProgramRef) != result.Version.ArtifactProgramRef {
		return fmt.Errorf("base promotion settlement: promotion result typed artifact-program ref is invalid")
	}
	if strings.TrimSpace(result.PromotionReadinessRef) == "" || strings.TrimSpace(result.PackagePublicationProofRef) == "" || strings.TrimSpace(result.PromotionCandidateRef) == "" || strings.TrimSpace(result.PromotionExecutionPlanRef) == "" || strings.TrimSpace(result.PromotionPreflightRef) == "" || strings.TrimSpace(result.PromotionOperatorPolicyRef) == "" || strings.TrimSpace(result.PromotionRollbackPlanRef) == "" || strings.TrimSpace(result.RouteCutoverPlanRef) == "" || strings.TrimSpace(result.LedgerFreshnessCheckRef) == "" || strings.TrimSpace(result.PromotionOutcomeRef) == "" || strings.TrimSpace(result.PromotionOutcomeReasonRef) == "" || strings.TrimSpace(result.OperatorDecisionRef) == "" || strings.TrimSpace(result.PromotionAttemptRef) == "" {
		return fmt.Errorf("base promotion settlement: promotion result refs are required")
	}
	if result.PromotionOutcome != BasePromotionResultOutcomeBlocked && result.PromotionOutcome != BasePromotionResultOutcomeNoop {
		return fmt.Errorf("base promotion settlement: promotion result outcome must be blocked or noop")
	}
	if !result.PromotionResultRecorded || !result.PromotionExecutionReady {
		return fmt.Errorf("base promotion settlement: promotion result must be recorded after readiness")
	}
	if !result.PromotionProofRequired || !result.RunAcceptanceProofRequired || !result.FullSubstrateProofRequired {
		return fmt.Errorf("base promotion settlement: promotion result must preserve downstream proof requirements")
	}
	if result.PackagePublicationAllowed || result.PromotionAllowed || result.RunAcceptanceSynthesisAllowed {
		return fmt.Errorf("base promotion settlement: promotion result allows downstream execution")
	}
	if !result.NoPackagePublicationMutation || !result.NoPromotionMutation || !result.NoRunAcceptanceMutation || !result.NoProductionMutation {
		return fmt.Errorf("base promotion settlement: promotion result must prove no package publication, promotion, run-acceptance, or production mutation")
	}
	if result.PackagePublished || result.PromotionExecuted || result.RunAcceptanceRecordTouched || result.ProductionStateMutated || result.FullSubstrateClaimed || result.CompletionClaimed {
		return fmt.Errorf("base promotion settlement: promotion result carries downstream execution, production mutation, or completion claims")
	}
	return nil
}

func validateBasePromotionSettlementEvidence(result BasePromotionResultContract, evidence BasePromotionSettlementEvidence) (string, error) {
	decision := strings.TrimSpace(evidence.SettlementDecision)
	if result.PromotionOutcome == BasePromotionResultOutcomeBlocked && decision != BasePromotionSettlementDecisionBlocked {
		return "", fmt.Errorf("base promotion settlement: blocked result requires blocked settlement decision")
	}
	if result.PromotionOutcome == BasePromotionResultOutcomeNoop && decision != BasePromotionSettlementDecisionNoop {
		return "", fmt.Errorf("base promotion settlement: noop result requires noop settlement decision")
	}
	if strings.TrimSpace(evidence.PromotionResultRef) == "" || strings.TrimSpace(evidence.SettlementRef) == "" || strings.TrimSpace(evidence.SettlementReasonRef) == "" || strings.TrimSpace(evidence.OperatorReviewRef) == "" || strings.TrimSpace(evidence.RollbackPlanRef) == "" {
		return "", fmt.Errorf("base promotion settlement: settlement refs are required")
	}
	if !evidence.NoPackagePublicationMutation || !evidence.NoPromotionMutation || !evidence.NoRunAcceptanceMutation || !evidence.NoProductionMutation {
		return "", fmt.Errorf("base promotion settlement: evidence must prove no package publication, promotion, run-acceptance, or production mutation")
	}
	if evidence.PackagePublished || evidence.PromotionExecuted || evidence.RunAcceptanceRecordTouched || evidence.ProductionStateMutated || evidence.FullSubstrateClaimed || evidence.CompletionClaimed {
		return "", fmt.Errorf("base promotion settlement: evidence carries downstream execution, production mutation, or completion claims")
	}
	return decision, nil
}
