package computerversion

import (
	"fmt"
	"strings"
)

const BaseDurableStateSliceReadinessContractKind = "base_durable_state_slice_readiness_contract"

const BaseDurableStateSliceReadinessBoundary = "durable_state_slice_readiness_without_runtime_materialization_or_promotion"

const BaseDurableStateSliceReadinessScope = "post_settlement_handoff_to_typed_durable_state_slice_readiness_only"

const BaseDurableStateSliceReadinessStatusReady = "ready_for_typed_durable_state_slice_probe_not_materialized"

const BaseDurableStateSliceReadinessPrerequisiteFileManifestProbe = "file_manifest_probe"
const BaseDurableStateSliceReadinessPrerequisiteBlobContentProbe = "blob_content_probe"
const BaseDurableStateSliceReadinessPrerequisiteObservationSet = "observation_set"
const BaseDurableStateSliceReadinessPrerequisiteMaterializerContract = "materializer_contract"
const BaseDurableStateSliceReadinessPrerequisiteEquivalenceCheck = "equivalence_check"
const BaseDurableStateSliceReadinessPrerequisiteResidualRiskReview = "residual_risk_review"

// BaseDurableStateSliceReadinessEvidence records the refs needed to move from a
// blocked post-promotion-settlement handoff into a typed durable-state-slice
// probe. It does not materialize runtime state, publish packages, execute
// promotion, synthesize run acceptance, mutate production state, or claim full
// substrate independence.
type BaseDurableStateSliceReadinessEvidence struct {
	PostPromotionSettlementHandoffRef string `json:"post_promotion_settlement_handoff_ref"`
	DurableStateSlicePlanRef          string `json:"durable_state_slice_plan_ref"`
	FileManifestProbeRef              string `json:"file_manifest_probe_ref"`
	BlobContentProbeRef               string `json:"blob_content_probe_ref"`
	ObservationSetRef                 string `json:"observation_set_ref"`
	MaterializerContractRef           string `json:"materializer_contract_ref"`
	EquivalenceCheckRef               string `json:"equivalence_check_ref"`
	ResidualRiskRef                   string `json:"residual_risk_ref"`
	RollbackPlanRef                   string `json:"rollback_plan_ref"`
	NoRuntimeMaterialization          bool   `json:"no_runtime_materialization"`
	NoDurableComputerMutation         bool   `json:"no_durable_computer_mutation"`
	NoPackagePublicationMutation      bool   `json:"no_package_publication_mutation"`
	NoPromotionMutation               bool   `json:"no_promotion_mutation"`
	NoRunAcceptanceMutation           bool   `json:"no_run_acceptance_mutation"`
	NoProductionMutation              bool   `json:"no_production_mutation"`
	RuntimeMaterialized               bool   `json:"runtime_materialized"`
	DurableComputerStateMutated       bool   `json:"durable_computer_state_mutated"`
	PackagePublished                  bool   `json:"package_published"`
	PromotionExecuted                 bool   `json:"promotion_executed"`
	RunAcceptanceRecordTouched        bool   `json:"run_acceptance_record_touched"`
	ProductionStateMutated            bool   `json:"production_state_mutated"`
	FullSubstrateClaimed              bool   `json:"full_substrate_claimed"`
	CompletionClaimed                 bool   `json:"completion_claimed"`
}

// BaseDurableStateSliceReadinessContract records readiness to run the first
// typed durable-state-slice probe after promotion settlement hands control back
// to substrate-independence work. It is not runtime materialization, promotion,
// run acceptance, production mutation, full-substrate proof, or completion.
type BaseDurableStateSliceReadinessContract struct {
	Kind                              string          `json:"kind"`
	Version                           ComputerVersion `json:"version"`
	Boundary                          string          `json:"boundary"`
	Scope                             string          `json:"scope"`
	TypedArtifactProgramRef           string          `json:"typed_artifact_program_ref"`
	PostPromotionSettlementHandoffRef string          `json:"post_promotion_settlement_handoff_ref"`
	PromotionSettlementRef            string          `json:"promotion_settlement_ref"`
	PromotionResultRef                string          `json:"promotion_result_ref"`
	PromotionReadinessRef             string          `json:"promotion_readiness_ref"`
	PackagePublicationProofRef        string          `json:"package_publication_proof_ref"`
	PromotionOutcome                  string          `json:"promotion_outcome"`
	SettlementDecision                string          `json:"settlement_decision"`
	SettlementReasonRef               string          `json:"settlement_reason_ref"`
	OperatorReviewRef                 string          `json:"operator_review_ref"`
	NextSubstrateProofPlanRef         string          `json:"next_substrate_proof_plan_ref"`
	DurableStateSlicePlanRef          string          `json:"durable_state_slice_plan_ref"`
	FileManifestProbeRef              string          `json:"file_manifest_probe_ref"`
	BlobContentProbeRef               string          `json:"blob_content_probe_ref"`
	ObservationSetRef                 string          `json:"observation_set_ref"`
	MaterializerContractRef           string          `json:"materializer_contract_ref"`
	EquivalenceCheckRef               string          `json:"equivalence_check_ref"`
	ResidualRiskRef                   string          `json:"residual_risk_ref"`
	RollbackPlanRef                   string          `json:"rollback_plan_ref"`
	ReadinessStatus                   string          `json:"readiness_status"`
	RequiredPrerequisites             []string        `json:"required_prerequisites"`
	PostSettlementHandoffRecorded     bool            `json:"post_settlement_handoff_recorded"`
	DurableStateSliceProbeRequired    bool            `json:"durable_state_slice_probe_required"`
	FileManifestProbeRequired         bool            `json:"file_manifest_probe_required"`
	BlobContentProbeRequired          bool            `json:"blob_content_probe_required"`
	ObservationSetRequired            bool            `json:"observation_set_required"`
	MaterializerContractRequired      bool            `json:"materializer_contract_required"`
	EquivalenceCheckRequired          bool            `json:"equivalence_check_required"`
	PromotionProofRequired            bool            `json:"promotion_proof_required"`
	RunAcceptanceProofRequired        bool            `json:"run_acceptance_proof_required"`
	FullSubstrateProofRequired        bool            `json:"full_substrate_proof_required"`
	RuntimeMaterializationAllowed     bool            `json:"runtime_materialization_allowed"`
	DurableComputerMutationAllowed    bool            `json:"durable_computer_mutation_allowed"`
	PackagePublicationAllowed         bool            `json:"package_publication_allowed"`
	PromotionAllowed                  bool            `json:"promotion_allowed"`
	RunAcceptanceSynthesisAllowed     bool            `json:"run_acceptance_synthesis_allowed"`
	NoRuntimeMaterialization          bool            `json:"no_runtime_materialization"`
	NoDurableComputerMutation         bool            `json:"no_durable_computer_mutation"`
	NoPackagePublicationMutation      bool            `json:"no_package_publication_mutation"`
	NoPromotionMutation               bool            `json:"no_promotion_mutation"`
	NoRunAcceptanceMutation           bool            `json:"no_run_acceptance_mutation"`
	NoProductionMutation              bool            `json:"no_production_mutation"`
	RuntimeMaterialized               bool            `json:"runtime_materialized"`
	DurableComputerStateMutated       bool            `json:"durable_computer_state_mutated"`
	PackagePublished                  bool            `json:"package_published"`
	PromotionExecuted                 bool            `json:"promotion_executed"`
	RunAcceptanceRecordTouched        bool            `json:"run_acceptance_record_touched"`
	ProductionStateMutated            bool            `json:"production_state_mutated"`
	FullSubstrateClaimed              bool            `json:"full_substrate_claimed"`
	CompletionClaimed                 bool            `json:"completion_claimed"`
}

// BuildBaseDurableStateSliceReadinessContract consumes a post-settlement
// handoff and records readiness for the typed durable-state-slice probe without
// granting materialization, promotion, run-acceptance, or completion authority.
func BuildBaseDurableStateSliceReadinessContract(handoff BasePostPromotionSettlementHandoffReadinessContract, evidence BaseDurableStateSliceReadinessEvidence) (BaseDurableStateSliceReadinessContract, error) {
	if err := validateBaseDurableStateSliceReadinessHandoff(handoff); err != nil {
		return BaseDurableStateSliceReadinessContract{}, err
	}
	if err := validateBaseDurableStateSliceReadinessEvidence(handoff, evidence); err != nil {
		return BaseDurableStateSliceReadinessContract{}, err
	}

	return BaseDurableStateSliceReadinessContract{
		Kind:                              BaseDurableStateSliceReadinessContractKind,
		Version:                           handoff.Version,
		Boundary:                          BaseDurableStateSliceReadinessBoundary,
		Scope:                             BaseDurableStateSliceReadinessScope,
		TypedArtifactProgramRef:           string(handoff.Version.ArtifactProgramRef),
		PostPromotionSettlementHandoffRef: strings.TrimSpace(evidence.PostPromotionSettlementHandoffRef),
		PromotionSettlementRef:            strings.TrimSpace(handoff.PromotionSettlementRef),
		PromotionResultRef:                strings.TrimSpace(handoff.PromotionResultRef),
		PromotionReadinessRef:             strings.TrimSpace(handoff.PromotionReadinessRef),
		PackagePublicationProofRef:        strings.TrimSpace(handoff.PackagePublicationProofRef),
		PromotionOutcome:                  strings.TrimSpace(handoff.PromotionOutcome),
		SettlementDecision:                strings.TrimSpace(handoff.SettlementDecision),
		SettlementReasonRef:               strings.TrimSpace(handoff.SettlementReasonRef),
		OperatorReviewRef:                 strings.TrimSpace(handoff.OperatorReviewRef),
		NextSubstrateProofPlanRef:         strings.TrimSpace(handoff.NextSubstrateProofPlanRef),
		DurableStateSlicePlanRef:          strings.TrimSpace(evidence.DurableStateSlicePlanRef),
		FileManifestProbeRef:              strings.TrimSpace(evidence.FileManifestProbeRef),
		BlobContentProbeRef:               strings.TrimSpace(evidence.BlobContentProbeRef),
		ObservationSetRef:                 strings.TrimSpace(evidence.ObservationSetRef),
		MaterializerContractRef:           strings.TrimSpace(evidence.MaterializerContractRef),
		EquivalenceCheckRef:               strings.TrimSpace(evidence.EquivalenceCheckRef),
		ResidualRiskRef:                   strings.TrimSpace(evidence.ResidualRiskRef),
		RollbackPlanRef:                   strings.TrimSpace(evidence.RollbackPlanRef),
		ReadinessStatus:                   BaseDurableStateSliceReadinessStatusReady,
		RequiredPrerequisites:             baseDurableStateSliceReadinessPrerequisites(),
		PostSettlementHandoffRecorded:     true,
		DurableStateSliceProbeRequired:    true,
		FileManifestProbeRequired:         true,
		BlobContentProbeRequired:          true,
		ObservationSetRequired:            true,
		MaterializerContractRequired:      true,
		EquivalenceCheckRequired:          true,
		PromotionProofRequired:            true,
		RunAcceptanceProofRequired:        true,
		FullSubstrateProofRequired:        true,
		RuntimeMaterializationAllowed:     false,
		DurableComputerMutationAllowed:    false,
		PackagePublicationAllowed:         false,
		PromotionAllowed:                  false,
		RunAcceptanceSynthesisAllowed:     false,
		NoRuntimeMaterialization:          true,
		NoDurableComputerMutation:         true,
		NoPackagePublicationMutation:      true,
		NoPromotionMutation:               true,
		NoRunAcceptanceMutation:           true,
		NoProductionMutation:              true,
	}, nil
}

func validateBaseDurableStateSliceReadinessHandoff(handoff BasePostPromotionSettlementHandoffReadinessContract) error {
	if handoff.Kind != BasePostPromotionSettlementHandoffReadinessContractKind {
		return fmt.Errorf("base durable-state-slice readiness: handoff kind is %q", handoff.Kind)
	}
	if handoff.Boundary != BasePostPromotionSettlementHandoffReadinessBoundary {
		return fmt.Errorf("base durable-state-slice readiness: handoff boundary is %q", handoff.Boundary)
	}
	if handoff.Scope != BasePostPromotionSettlementHandoffReadinessScope {
		return fmt.Errorf("base durable-state-slice readiness: handoff scope is %q", handoff.Scope)
	}
	if !handoff.Version.Valid() {
		return fmt.Errorf("base durable-state-slice readiness: handoff version is invalid")
	}
	if !handoff.Version.ArtifactProgramRef.Valid() || ArtifactProgramRef(handoff.TypedArtifactProgramRef) != handoff.Version.ArtifactProgramRef {
		return fmt.Errorf("base durable-state-slice readiness: handoff typed artifact-program ref is invalid")
	}
	if strings.TrimSpace(handoff.PromotionSettlementRef) == "" || strings.TrimSpace(handoff.PromotionResultRef) == "" || strings.TrimSpace(handoff.PromotionReadinessRef) == "" || strings.TrimSpace(handoff.PackagePublicationProofRef) == "" || strings.TrimSpace(handoff.PromotionOutcome) == "" || strings.TrimSpace(handoff.SettlementDecision) == "" || strings.TrimSpace(handoff.SettlementReasonRef) == "" || strings.TrimSpace(handoff.OperatorReviewRef) == "" || strings.TrimSpace(handoff.NextSubstrateProofPlanRef) == "" || strings.TrimSpace(handoff.DurableStateSliceRef) == "" || strings.TrimSpace(handoff.ObservationSetRef) == "" || strings.TrimSpace(handoff.MaterializerContractRef) == "" || strings.TrimSpace(handoff.EquivalenceCheckRef) == "" || strings.TrimSpace(handoff.ResidualRiskRef) == "" || strings.TrimSpace(handoff.RollbackPlanRef) == "" {
		return fmt.Errorf("base durable-state-slice readiness: handoff refs are required")
	}
	if handoff.ReadinessStatus != BasePostPromotionSettlementHandoffReadinessStatusBlocked {
		return fmt.Errorf("base durable-state-slice readiness: handoff status is %q", handoff.ReadinessStatus)
	}
	if !handoff.PromotionResultSettled || !handoff.NextSubstrateProofRequired || !handoff.DurableStateSliceRequired || !handoff.ObservationSetRequired || !handoff.MaterializerContractRequired || !handoff.EquivalenceCheckRequired {
		return fmt.Errorf("base durable-state-slice readiness: handoff must preserve substrate proof prerequisites")
	}
	if !handoff.PromotionProofRequired || !handoff.RunAcceptanceProofRequired || !handoff.FullSubstrateProofRequired {
		return fmt.Errorf("base durable-state-slice readiness: handoff must preserve downstream proof requirements")
	}
	if handoff.PackagePublicationAllowed || handoff.PromotionAllowed || handoff.RunAcceptanceSynthesisAllowed {
		return fmt.Errorf("base durable-state-slice readiness: handoff allows downstream execution")
	}
	if !handoff.NoPackagePublicationMutation || !handoff.NoPromotionMutation || !handoff.NoRunAcceptanceMutation || !handoff.NoProductionMutation {
		return fmt.Errorf("base durable-state-slice readiness: handoff must prove no package publication, promotion, run-acceptance, or production mutation")
	}
	if handoff.PackagePublished || handoff.PromotionExecuted || handoff.RunAcceptanceRecordTouched || handoff.ProductionStateMutated || handoff.FullSubstrateClaimed || handoff.CompletionClaimed {
		return fmt.Errorf("base durable-state-slice readiness: handoff carries downstream execution, production mutation, or completion claims")
	}
	return nil
}

func validateBaseDurableStateSliceReadinessEvidence(handoff BasePostPromotionSettlementHandoffReadinessContract, evidence BaseDurableStateSliceReadinessEvidence) error {
	if strings.TrimSpace(evidence.PostPromotionSettlementHandoffRef) == "" || strings.TrimSpace(evidence.DurableStateSlicePlanRef) == "" || strings.TrimSpace(evidence.FileManifestProbeRef) == "" || strings.TrimSpace(evidence.BlobContentProbeRef) == "" || strings.TrimSpace(evidence.ObservationSetRef) == "" || strings.TrimSpace(evidence.MaterializerContractRef) == "" || strings.TrimSpace(evidence.EquivalenceCheckRef) == "" || strings.TrimSpace(evidence.ResidualRiskRef) == "" || strings.TrimSpace(evidence.RollbackPlanRef) == "" {
		return fmt.Errorf("base durable-state-slice readiness: evidence refs are required")
	}
	if strings.TrimSpace(evidence.DurableStateSlicePlanRef) != strings.TrimSpace(handoff.DurableStateSliceRef) || strings.TrimSpace(evidence.ObservationSetRef) != strings.TrimSpace(handoff.ObservationSetRef) || strings.TrimSpace(evidence.MaterializerContractRef) != strings.TrimSpace(handoff.MaterializerContractRef) || strings.TrimSpace(evidence.EquivalenceCheckRef) != strings.TrimSpace(handoff.EquivalenceCheckRef) || strings.TrimSpace(evidence.ResidualRiskRef) != strings.TrimSpace(handoff.ResidualRiskRef) || strings.TrimSpace(evidence.RollbackPlanRef) != strings.TrimSpace(handoff.RollbackPlanRef) {
		return fmt.Errorf("base durable-state-slice readiness: evidence refs do not match handoff")
	}
	if !evidence.NoRuntimeMaterialization || !evidence.NoDurableComputerMutation || !evidence.NoPackagePublicationMutation || !evidence.NoPromotionMutation || !evidence.NoRunAcceptanceMutation || !evidence.NoProductionMutation {
		return fmt.Errorf("base durable-state-slice readiness: evidence must prove no runtime, durable-computer, package publication, promotion, run-acceptance, or production mutation")
	}
	if evidence.RuntimeMaterialized || evidence.DurableComputerStateMutated || evidence.PackagePublished || evidence.PromotionExecuted || evidence.RunAcceptanceRecordTouched || evidence.ProductionStateMutated || evidence.FullSubstrateClaimed || evidence.CompletionClaimed {
		return fmt.Errorf("base durable-state-slice readiness: evidence carries materialization, mutation, downstream execution, production, full-substrate, or completion claims")
	}
	return nil
}

func baseDurableStateSliceReadinessPrerequisites() []string {
	return []string{
		BaseDurableStateSliceReadinessPrerequisiteFileManifestProbe,
		BaseDurableStateSliceReadinessPrerequisiteBlobContentProbe,
		BaseDurableStateSliceReadinessPrerequisiteObservationSet,
		BaseDurableStateSliceReadinessPrerequisiteMaterializerContract,
		BaseDurableStateSliceReadinessPrerequisiteEquivalenceCheck,
		BaseDurableStateSliceReadinessPrerequisiteResidualRiskReview,
	}
}
