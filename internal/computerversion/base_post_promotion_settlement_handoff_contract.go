package computerversion

import (
	"fmt"
	"strings"
)

const BasePostPromotionSettlementHandoffReadinessContractKind = "base_post_promotion_settlement_handoff_readiness_contract"

const BasePostPromotionSettlementHandoffReadinessBoundary = "post_promotion_settlement_handoff_without_promotion_or_run_acceptance"

const BasePostPromotionSettlementHandoffReadinessScope = "promotion_settlement_to_substrate_proof_readiness_only"

const BasePostPromotionSettlementHandoffReadinessStatusBlocked = "blocked_until_next_substrate_independence_proof"

const BasePostPromotionSettlementPrerequisiteDurableStateSlice = "durable_state_slice"
const BasePostPromotionSettlementPrerequisiteObservationSet = "observation_set"
const BasePostPromotionSettlementPrerequisiteMaterializerContract = "materializer_contract"
const BasePostPromotionSettlementPrerequisiteEquivalenceCheck = "equivalence_check"
const BasePostPromotionSettlementPrerequisiteResidualRiskReview = "residual_risk_review"

// BasePostPromotionSettlementHandoffReadinessEvidence records the refs needed to
// return from promotion settlement to substrate-independence proof work. It does
// not execute promotion, synthesize run acceptance, mutate production state, or
// claim completion.
type BasePostPromotionSettlementHandoffReadinessEvidence struct {
	PromotionSettlementRef       string `json:"promotion_settlement_ref"`
	NextSubstrateProofPlanRef    string `json:"next_substrate_proof_plan_ref"`
	DurableStateSliceRef         string `json:"durable_state_slice_ref"`
	ObservationSetRef            string `json:"observation_set_ref"`
	MaterializerContractRef      string `json:"materializer_contract_ref"`
	EquivalenceCheckRef          string `json:"equivalence_check_ref"`
	ResidualRiskRef              string `json:"residual_risk_ref"`
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

// BasePostPromotionSettlementHandoffReadinessContract records a blocked handoff
// from promotion settlement back to substrate-independence proof work. It is not
// promotion execution, run acceptance, full-substrate proof, or completion.
type BasePostPromotionSettlementHandoffReadinessContract struct {
	Kind                          string          `json:"kind"`
	Version                       ComputerVersion `json:"version"`
	Boundary                      string          `json:"boundary"`
	Scope                         string          `json:"scope"`
	TypedArtifactProgramRef       string          `json:"typed_artifact_program_ref"`
	PromotionSettlementRef        string          `json:"promotion_settlement_ref"`
	PromotionResultRef            string          `json:"promotion_result_ref"`
	PromotionReadinessRef         string          `json:"promotion_readiness_ref"`
	PackagePublicationProofRef    string          `json:"package_publication_proof_ref"`
	PromotionOutcome              string          `json:"promotion_outcome"`
	SettlementDecision            string          `json:"settlement_decision"`
	SettlementReasonRef           string          `json:"settlement_reason_ref"`
	OperatorReviewRef             string          `json:"operator_review_ref"`
	NextSubstrateProofPlanRef     string          `json:"next_substrate_proof_plan_ref"`
	DurableStateSliceRef          string          `json:"durable_state_slice_ref"`
	ObservationSetRef             string          `json:"observation_set_ref"`
	MaterializerContractRef       string          `json:"materializer_contract_ref"`
	EquivalenceCheckRef           string          `json:"equivalence_check_ref"`
	ResidualRiskRef               string          `json:"residual_risk_ref"`
	RollbackPlanRef               string          `json:"rollback_plan_ref"`
	ReadinessStatus               string          `json:"readiness_status"`
	RequiredPrerequisites         []string        `json:"required_prerequisites"`
	PromotionResultSettled        bool            `json:"promotion_result_settled"`
	NextSubstrateProofRequired    bool            `json:"next_substrate_proof_required"`
	DurableStateSliceRequired     bool            `json:"durable_state_slice_required"`
	ObservationSetRequired        bool            `json:"observation_set_required"`
	MaterializerContractRequired  bool            `json:"materializer_contract_required"`
	EquivalenceCheckRequired      bool            `json:"equivalence_check_required"`
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

// BuildBasePostPromotionSettlementHandoffReadinessContract consumes promotion
// settlement evidence and records the next substrate-proof handoff without
// granting downstream promotion, run-acceptance, or completion authority.
func BuildBasePostPromotionSettlementHandoffReadinessContract(settlement BasePromotionSettlementContract, evidence BasePostPromotionSettlementHandoffReadinessEvidence) (BasePostPromotionSettlementHandoffReadinessContract, error) {
	if err := validateBasePostPromotionSettlementHandoffSettlement(settlement); err != nil {
		return BasePostPromotionSettlementHandoffReadinessContract{}, err
	}
	if err := validateBasePostPromotionSettlementHandoffEvidence(evidence); err != nil {
		return BasePostPromotionSettlementHandoffReadinessContract{}, err
	}

	return BasePostPromotionSettlementHandoffReadinessContract{
		Kind:                          BasePostPromotionSettlementHandoffReadinessContractKind,
		Version:                       settlement.Version,
		Boundary:                      BasePostPromotionSettlementHandoffReadinessBoundary,
		Scope:                         BasePostPromotionSettlementHandoffReadinessScope,
		TypedArtifactProgramRef:       string(settlement.Version.ArtifactProgramRef),
		PromotionSettlementRef:        strings.TrimSpace(evidence.PromotionSettlementRef),
		PromotionResultRef:            strings.TrimSpace(settlement.PromotionResultRef),
		PromotionReadinessRef:         strings.TrimSpace(settlement.PromotionReadinessRef),
		PackagePublicationProofRef:    strings.TrimSpace(settlement.PackagePublicationProofRef),
		PromotionOutcome:              strings.TrimSpace(settlement.PromotionOutcome),
		SettlementDecision:            strings.TrimSpace(settlement.SettlementDecision),
		SettlementReasonRef:           strings.TrimSpace(settlement.SettlementReasonRef),
		OperatorReviewRef:             strings.TrimSpace(settlement.OperatorReviewRef),
		NextSubstrateProofPlanRef:     strings.TrimSpace(evidence.NextSubstrateProofPlanRef),
		DurableStateSliceRef:          strings.TrimSpace(evidence.DurableStateSliceRef),
		ObservationSetRef:             strings.TrimSpace(evidence.ObservationSetRef),
		MaterializerContractRef:       strings.TrimSpace(evidence.MaterializerContractRef),
		EquivalenceCheckRef:           strings.TrimSpace(evidence.EquivalenceCheckRef),
		ResidualRiskRef:               strings.TrimSpace(evidence.ResidualRiskRef),
		RollbackPlanRef:               strings.TrimSpace(evidence.RollbackPlanRef),
		ReadinessStatus:               BasePostPromotionSettlementHandoffReadinessStatusBlocked,
		RequiredPrerequisites:         basePostPromotionSettlementHandoffPrerequisites(),
		PromotionResultSettled:        true,
		NextSubstrateProofRequired:    true,
		DurableStateSliceRequired:     true,
		ObservationSetRequired:        true,
		MaterializerContractRequired:  true,
		EquivalenceCheckRequired:      true,
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

func validateBasePostPromotionSettlementHandoffSettlement(settlement BasePromotionSettlementContract) error {
	if settlement.Kind != BasePromotionSettlementContractKind {
		return fmt.Errorf("base post-promotion-settlement handoff: settlement kind is %q", settlement.Kind)
	}
	if settlement.Boundary != BasePromotionSettlementBoundary {
		return fmt.Errorf("base post-promotion-settlement handoff: settlement boundary is %q", settlement.Boundary)
	}
	if settlement.Scope != BasePromotionSettlementScope {
		return fmt.Errorf("base post-promotion-settlement handoff: settlement scope is %q", settlement.Scope)
	}
	if !settlement.Version.Valid() {
		return fmt.Errorf("base post-promotion-settlement handoff: settlement version is invalid")
	}
	if !settlement.Version.ArtifactProgramRef.Valid() || ArtifactProgramRef(settlement.TypedArtifactProgramRef) != settlement.Version.ArtifactProgramRef {
		return fmt.Errorf("base post-promotion-settlement handoff: settlement typed artifact-program ref is invalid")
	}
	if strings.TrimSpace(settlement.PromotionResultRef) == "" || strings.TrimSpace(settlement.PromotionReadinessRef) == "" || strings.TrimSpace(settlement.PackagePublicationProofRef) == "" || strings.TrimSpace(settlement.PromotionOutcome) == "" || strings.TrimSpace(settlement.SettlementRef) == "" || strings.TrimSpace(settlement.SettlementReasonRef) == "" || strings.TrimSpace(settlement.OperatorReviewRef) == "" {
		return fmt.Errorf("base post-promotion-settlement handoff: settlement refs are required")
	}
	if settlement.PromotionOutcome != BasePromotionResultOutcomeBlocked && settlement.PromotionOutcome != BasePromotionResultOutcomeNoop {
		return fmt.Errorf("base post-promotion-settlement handoff: settlement outcome must be blocked or noop")
	}
	if settlement.PromotionOutcome == BasePromotionResultOutcomeBlocked && settlement.SettlementDecision != BasePromotionSettlementDecisionBlocked {
		return fmt.Errorf("base post-promotion-settlement handoff: blocked outcome requires blocked settlement")
	}
	if settlement.PromotionOutcome == BasePromotionResultOutcomeNoop && settlement.SettlementDecision != BasePromotionSettlementDecisionNoop {
		return fmt.Errorf("base post-promotion-settlement handoff: noop outcome requires noop settlement")
	}
	if !settlement.PromotionResultRecorded || !settlement.PromotionResultSettled || !settlement.PromotionExecutionReady {
		return fmt.Errorf("base post-promotion-settlement handoff: settlement must settle a recorded promotion result")
	}
	if !settlement.PromotionProofRequired || !settlement.RunAcceptanceProofRequired || !settlement.FullSubstrateProofRequired {
		return fmt.Errorf("base post-promotion-settlement handoff: settlement must preserve downstream proof requirements")
	}
	if settlement.PackagePublicationAllowed || settlement.PromotionAllowed || settlement.RunAcceptanceSynthesisAllowed {
		return fmt.Errorf("base post-promotion-settlement handoff: settlement allows downstream execution")
	}
	if !settlement.NoPackagePublicationMutation || !settlement.NoPromotionMutation || !settlement.NoRunAcceptanceMutation || !settlement.NoProductionMutation {
		return fmt.Errorf("base post-promotion-settlement handoff: settlement must prove no package publication, promotion, run-acceptance, or production mutation")
	}
	if settlement.PackagePublished || settlement.PromotionExecuted || settlement.RunAcceptanceRecordTouched || settlement.ProductionStateMutated || settlement.FullSubstrateClaimed || settlement.CompletionClaimed {
		return fmt.Errorf("base post-promotion-settlement handoff: settlement carries downstream execution, production mutation, or completion claims")
	}
	return nil
}

func validateBasePostPromotionSettlementHandoffEvidence(evidence BasePostPromotionSettlementHandoffReadinessEvidence) error {
	if strings.TrimSpace(evidence.PromotionSettlementRef) == "" || strings.TrimSpace(evidence.NextSubstrateProofPlanRef) == "" || strings.TrimSpace(evidence.DurableStateSliceRef) == "" || strings.TrimSpace(evidence.ObservationSetRef) == "" || strings.TrimSpace(evidence.MaterializerContractRef) == "" || strings.TrimSpace(evidence.EquivalenceCheckRef) == "" || strings.TrimSpace(evidence.ResidualRiskRef) == "" || strings.TrimSpace(evidence.RollbackPlanRef) == "" {
		return fmt.Errorf("base post-promotion-settlement handoff: substrate proof refs are required")
	}
	if !evidence.NoPackagePublicationMutation || !evidence.NoPromotionMutation || !evidence.NoRunAcceptanceMutation || !evidence.NoProductionMutation {
		return fmt.Errorf("base post-promotion-settlement handoff: evidence must prove no package publication, promotion, run-acceptance, or production mutation")
	}
	if evidence.PackagePublished || evidence.PromotionExecuted || evidence.RunAcceptanceRecordTouched || evidence.ProductionStateMutated || evidence.FullSubstrateClaimed || evidence.CompletionClaimed {
		return fmt.Errorf("base post-promotion-settlement handoff: evidence carries downstream execution, production mutation, or completion claims")
	}
	return nil
}

func basePostPromotionSettlementHandoffPrerequisites() []string {
	return []string{
		BasePostPromotionSettlementPrerequisiteDurableStateSlice,
		BasePostPromotionSettlementPrerequisiteObservationSet,
		BasePostPromotionSettlementPrerequisiteMaterializerContract,
		BasePostPromotionSettlementPrerequisiteEquivalenceCheck,
		BasePostPromotionSettlementPrerequisiteResidualRiskReview,
	}
}
