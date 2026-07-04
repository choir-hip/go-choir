package computerversion

import (
	"fmt"
	"strings"
)

const BaseDurableStateSliceProbeContractKind = "base_durable_state_slice_probe_contract"

const BaseDurableStateSliceProbeBoundary = "durable_state_slice_probe_without_runtime_materialization_or_completion"

const BaseDurableStateSliceProbeScope = "typed_file_manifest_blob_content_probe_result_only"

const BaseDurableStateSliceProbeStatusProven = "typed_file_manifest_blob_content_slice_proven"

// BaseDurableStateSliceProbeEvidence records the proof refs that bind durable
// state slice readiness to the typed file-manifest/blob-content durable state
// slice. It does not materialize runtime state, mutate durable computer state,
// publish packages, execute promotion, synthesize run acceptance, mutate
// production state, or claim full-substrate completion.
type BaseDurableStateSliceProbeEvidence struct {
	DurableStateSliceReadinessRef string `json:"durable_state_slice_readiness_ref"`
	DurableStateSliceContractRef  string `json:"durable_state_slice_contract_ref"`
	FileManifestProbeRef          string `json:"file_manifest_probe_ref"`
	BlobContentProbeRef           string `json:"blob_content_probe_ref"`
	ProbeEvidenceRef              string `json:"probe_evidence_ref"`
	ResidualRiskRef               string `json:"residual_risk_ref"`
	RollbackPlanRef               string `json:"rollback_plan_ref"`
	NoRuntimeMaterialization      bool   `json:"no_runtime_materialization"`
	NoOpaqueDataImageDependency   bool   `json:"no_opaque_data_img_dependency"`
	NoDurableComputerMutation     bool   `json:"no_durable_computer_mutation"`
	NoPackagePublicationMutation  bool   `json:"no_package_publication_mutation"`
	NoPromotionMutation           bool   `json:"no_promotion_mutation"`
	NoRunAcceptanceMutation       bool   `json:"no_run_acceptance_mutation"`
	NoProductionMutation          bool   `json:"no_production_mutation"`
	RuntimeMaterialized           bool   `json:"runtime_materialized"`
	DurableComputerStateMutated   bool   `json:"durable_computer_state_mutated"`
	PackagePublished              bool   `json:"package_published"`
	PromotionExecuted             bool   `json:"promotion_executed"`
	RunAcceptanceRecordTouched    bool   `json:"run_acceptance_record_touched"`
	ProductionStateMutated        bool   `json:"production_state_mutated"`
	FullSubstrateClaimed          bool   `json:"full_substrate_claimed"`
	CompletionClaimed             bool   `json:"completion_claimed"`
}

// BaseDurableStateSliceProbeContract records that a readiness boundary has been
// bound to the existing typed Base file-manifest/blob-content durable slice. It
// is not runtime materialization, durable computer mutation, promotion, run
// acceptance, production mutation, full-substrate proof, or completion.
type BaseDurableStateSliceProbeContract struct {
	Kind                              string                  `json:"kind"`
	Version                           ComputerVersion         `json:"version"`
	Boundary                          string                  `json:"boundary"`
	Scope                             string                  `json:"scope"`
	TypedArtifactProgramRef           string                  `json:"typed_artifact_program_ref"`
	DurableStateSliceReadinessRef     string                  `json:"durable_state_slice_readiness_ref"`
	DurableStateSliceContractRef      string                  `json:"durable_state_slice_contract_ref"`
	PostPromotionSettlementHandoffRef string                  `json:"post_promotion_settlement_handoff_ref"`
	NextSubstrateProofPlanRef         string                  `json:"next_substrate_proof_plan_ref"`
	FileManifestProbeRef              string                  `json:"file_manifest_probe_ref"`
	BlobContentProbeRef               string                  `json:"blob_content_probe_ref"`
	ProbeEvidenceRef                  string                  `json:"probe_evidence_ref"`
	ResidualRiskRef                   string                  `json:"residual_risk_ref"`
	RollbackPlanRef                   string                  `json:"rollback_plan_ref"`
	ProbeStatus                       string                  `json:"probe_status"`
	PersistentStateClasses            []BaseDurableStateClass `json:"persistent_state_classes"`
	RequiredObservations              []ObservationKind       `json:"required_observations"`
	RequiredSemantics                 []UserSemantic          `json:"required_semantics"`
	ReadinessConsumed                 bool                    `json:"readiness_consumed"`
	DurableStateSliceProven           bool                    `json:"durable_state_slice_proven"`
	FileManifestProbeRecorded         bool                    `json:"file_manifest_probe_recorded"`
	BlobContentProbeRecorded          bool                    `json:"blob_content_probe_recorded"`
	RuntimeProofRequired              bool                    `json:"runtime_proof_required"`
	StagingProofRequired              bool                    `json:"staging_proof_required"`
	PromotionProofRequired            bool                    `json:"promotion_proof_required"`
	RunAcceptanceProofRequired        bool                    `json:"run_acceptance_proof_required"`
	FullSubstrateProofRequired        bool                    `json:"full_substrate_proof_required"`
	RuntimeMaterializationAllowed     bool                    `json:"runtime_materialization_allowed"`
	DurableComputerMutationAllowed    bool                    `json:"durable_computer_mutation_allowed"`
	PackagePublicationAllowed         bool                    `json:"package_publication_allowed"`
	PromotionAllowed                  bool                    `json:"promotion_allowed"`
	RunAcceptanceSynthesisAllowed     bool                    `json:"run_acceptance_synthesis_allowed"`
	NoRuntimeMaterialization          bool                    `json:"no_runtime_materialization"`
	NoOpaqueDataImageDependency       bool                    `json:"no_opaque_data_img_dependency"`
	NoDurableComputerMutation         bool                    `json:"no_durable_computer_mutation"`
	NoPackagePublicationMutation      bool                    `json:"no_package_publication_mutation"`
	NoPromotionMutation               bool                    `json:"no_promotion_mutation"`
	NoRunAcceptanceMutation           bool                    `json:"no_run_acceptance_mutation"`
	NoProductionMutation              bool                    `json:"no_production_mutation"`
	RuntimeMaterialized               bool                    `json:"runtime_materialized"`
	DurableComputerStateMutated       bool                    `json:"durable_computer_state_mutated"`
	PackagePublished                  bool                    `json:"package_published"`
	PromotionExecuted                 bool                    `json:"promotion_executed"`
	RunAcceptanceRecordTouched        bool                    `json:"run_acceptance_record_touched"`
	ProductionStateMutated            bool                    `json:"production_state_mutated"`
	FullSubstrateClaimed              bool                    `json:"full_substrate_claimed"`
	CompletionClaimed                 bool                    `json:"completion_claimed"`
}

// BuildBaseDurableStateSliceProbeContract consumes durable-state-slice readiness
// plus the typed durable state slice and records only the scoped probe result.
func BuildBaseDurableStateSliceProbeContract(readiness BaseDurableStateSliceReadinessContract, durable BaseDurableStateSliceContract, evidence BaseDurableStateSliceProbeEvidence) (BaseDurableStateSliceProbeContract, error) {
	if err := validateBaseDurableStateSliceProbeReadiness(readiness); err != nil {
		return BaseDurableStateSliceProbeContract{}, err
	}
	if err := validateBaseDurableStateSliceProbeDurable(readiness, durable); err != nil {
		return BaseDurableStateSliceProbeContract{}, err
	}
	if err := validateBaseDurableStateSliceProbeEvidence(readiness, evidence); err != nil {
		return BaseDurableStateSliceProbeContract{}, err
	}

	return BaseDurableStateSliceProbeContract{
		Kind:                              BaseDurableStateSliceProbeContractKind,
		Version:                           readiness.Version,
		Boundary:                          BaseDurableStateSliceProbeBoundary,
		Scope:                             BaseDurableStateSliceProbeScope,
		TypedArtifactProgramRef:           strings.TrimSpace(durable.TypedArtifactProgramRef),
		DurableStateSliceReadinessRef:     strings.TrimSpace(evidence.DurableStateSliceReadinessRef),
		DurableStateSliceContractRef:      strings.TrimSpace(evidence.DurableStateSliceContractRef),
		PostPromotionSettlementHandoffRef: strings.TrimSpace(readiness.PostPromotionSettlementHandoffRef),
		NextSubstrateProofPlanRef:         strings.TrimSpace(readiness.NextSubstrateProofPlanRef),
		FileManifestProbeRef:              strings.TrimSpace(evidence.FileManifestProbeRef),
		BlobContentProbeRef:               strings.TrimSpace(evidence.BlobContentProbeRef),
		ProbeEvidenceRef:                  strings.TrimSpace(evidence.ProbeEvidenceRef),
		ResidualRiskRef:                   strings.TrimSpace(evidence.ResidualRiskRef),
		RollbackPlanRef:                   strings.TrimSpace(evidence.RollbackPlanRef),
		ProbeStatus:                       BaseDurableStateSliceProbeStatusProven,
		PersistentStateClasses:            []BaseDurableStateClass{BaseDurableStateClassBlobContent, BaseDurableStateClassFileManifest},
		RequiredObservations:              []ObservationKind{ObservationBlobSet, ObservationFileManifest},
		RequiredSemantics:                 []UserSemantic{UserSemanticDeletionState, UserSemanticFileContent, UserSemanticFilePath, UserSemanticFileProvenance},
		ReadinessConsumed:                 true,
		DurableStateSliceProven:           true,
		FileManifestProbeRecorded:         true,
		BlobContentProbeRecorded:          true,
		RuntimeProofRequired:              true,
		StagingProofRequired:              true,
		PromotionProofRequired:            true,
		RunAcceptanceProofRequired:        true,
		FullSubstrateProofRequired:        true,
		RuntimeMaterializationAllowed:     false,
		DurableComputerMutationAllowed:    false,
		PackagePublicationAllowed:         false,
		PromotionAllowed:                  false,
		RunAcceptanceSynthesisAllowed:     false,
		NoRuntimeMaterialization:          true,
		NoOpaqueDataImageDependency:       true,
		NoDurableComputerMutation:         true,
		NoPackagePublicationMutation:      true,
		NoPromotionMutation:               true,
		NoRunAcceptanceMutation:           true,
		NoProductionMutation:              true,
	}, nil
}

func validateBaseDurableStateSliceProbeReadiness(readiness BaseDurableStateSliceReadinessContract) error {
	if readiness.Kind != BaseDurableStateSliceReadinessContractKind {
		return fmt.Errorf("base durable-state-slice probe: readiness kind is %q", readiness.Kind)
	}
	if readiness.Boundary != BaseDurableStateSliceReadinessBoundary {
		return fmt.Errorf("base durable-state-slice probe: readiness boundary is %q", readiness.Boundary)
	}
	if readiness.Scope != BaseDurableStateSliceReadinessScope {
		return fmt.Errorf("base durable-state-slice probe: readiness scope is %q", readiness.Scope)
	}
	if !readiness.Version.Valid() {
		return fmt.Errorf("base durable-state-slice probe: readiness version is invalid")
	}
	if !readiness.Version.ArtifactProgramRef.Valid() || ArtifactProgramRef(readiness.TypedArtifactProgramRef) != readiness.Version.ArtifactProgramRef {
		return fmt.Errorf("base durable-state-slice probe: readiness typed artifact-program ref is invalid")
	}
	if strings.TrimSpace(readiness.PostPromotionSettlementHandoffRef) == "" || strings.TrimSpace(readiness.NextSubstrateProofPlanRef) == "" || strings.TrimSpace(readiness.DurableStateSlicePlanRef) == "" || strings.TrimSpace(readiness.FileManifestProbeRef) == "" || strings.TrimSpace(readiness.BlobContentProbeRef) == "" || strings.TrimSpace(readiness.ResidualRiskRef) == "" || strings.TrimSpace(readiness.RollbackPlanRef) == "" {
		return fmt.Errorf("base durable-state-slice probe: readiness refs are required")
	}
	if readiness.ReadinessStatus != BaseDurableStateSliceReadinessStatusReady {
		return fmt.Errorf("base durable-state-slice probe: readiness status is %q", readiness.ReadinessStatus)
	}
	if !readiness.PostSettlementHandoffRecorded || !readiness.DurableStateSliceProbeRequired || !readiness.FileManifestProbeRequired || !readiness.BlobContentProbeRequired || !readiness.ObservationSetRequired || !readiness.MaterializerContractRequired || !readiness.EquivalenceCheckRequired {
		return fmt.Errorf("base durable-state-slice probe: readiness must preserve durable-state-slice prerequisites")
	}
	if !readiness.PromotionProofRequired || !readiness.RunAcceptanceProofRequired || !readiness.FullSubstrateProofRequired {
		return fmt.Errorf("base durable-state-slice probe: readiness must preserve downstream proof requirements")
	}
	if readiness.RuntimeMaterializationAllowed || readiness.DurableComputerMutationAllowed || readiness.PackagePublicationAllowed || readiness.PromotionAllowed || readiness.RunAcceptanceSynthesisAllowed {
		return fmt.Errorf("base durable-state-slice probe: readiness allows downstream execution")
	}
	if !readiness.NoRuntimeMaterialization || !readiness.NoDurableComputerMutation || !readiness.NoPackagePublicationMutation || !readiness.NoPromotionMutation || !readiness.NoRunAcceptanceMutation || !readiness.NoProductionMutation {
		return fmt.Errorf("base durable-state-slice probe: readiness must prove no runtime, durable-computer, package publication, promotion, run-acceptance, or production mutation")
	}
	if readiness.RuntimeMaterialized || readiness.DurableComputerStateMutated || readiness.PackagePublished || readiness.PromotionExecuted || readiness.RunAcceptanceRecordTouched || readiness.ProductionStateMutated || readiness.FullSubstrateClaimed || readiness.CompletionClaimed {
		return fmt.Errorf("base durable-state-slice probe: readiness carries materialization, mutation, downstream execution, production, full-substrate, or completion claims")
	}
	return nil
}

func validateBaseDurableStateSliceProbeDurable(readiness BaseDurableStateSliceReadinessContract, durable BaseDurableStateSliceContract) error {
	if durable.Kind != BaseDurableStateSliceContractKind {
		return fmt.Errorf("base durable-state-slice probe: durable contract kind is %q", durable.Kind)
	}
	if durable.Boundary != BaseDurableStateSliceBoundary {
		return fmt.Errorf("base durable-state-slice probe: durable contract boundary is %q", durable.Boundary)
	}
	if durable.Scope != BaseDurableStateSliceScope {
		return fmt.Errorf("base durable-state-slice probe: durable contract scope is %q", durable.Scope)
	}
	if durable.Version != readiness.Version {
		return fmt.Errorf("base durable-state-slice probe: durable contract version does not match readiness")
	}
	if strings.TrimSpace(durable.TypedArtifactProgramRef) == "" || ArtifactProgramRef(strings.TrimSpace(durable.TypedArtifactProgramRef)) != readiness.Version.ArtifactProgramRef || strings.TrimSpace(durable.TypedArtifactProgramRef) != strings.TrimSpace(readiness.TypedArtifactProgramRef) {
		return fmt.Errorf("base durable-state-slice probe: durable typed artifact-program ref is invalid")
	}
	if !baseDurableStateSliceHasRequiredClasses(durable.PersistentStateClasses) {
		return fmt.Errorf("base durable-state-slice probe: durable contract must cover file manifest and blob content classes")
	}
	if !baseSubstrateEquivalenceHasRequiredScope(durable.RequiredObservations) {
		return fmt.Errorf("base durable-state-slice probe: durable contract must include file_manifest and blob_set observations")
	}
	if !baseDurableStateSliceHasRequiredSemantics(durable.RequiredSemantics) {
		return fmt.Errorf("base durable-state-slice probe: durable contract must cover file path, content, deletion, and provenance semantics")
	}
	if strings.TrimSpace(durable.EquivalenceContractRef) == "" || strings.TrimSpace(durable.UserIsomorphismContractRef) == "" || strings.TrimSpace(durable.DurableSliceEvidenceRef) == "" {
		return fmt.Errorf("base durable-state-slice probe: durable contract must carry proof refs")
	}
	if !durable.NoOpaqueDataImageDependency || !durable.NoMutation {
		return fmt.Errorf("base durable-state-slice probe: durable contract must be no-opaque-data-img and no-mutation")
	}
	if durable.RuntimeBehaviorChanged || durable.DeployedRouteRegistered || durable.ProductionAuthTouched || durable.StagingClaimed || durable.PromotionClaimed || durable.VMLifecycleTouched || durable.RunAcceptanceRecordTouched || durable.FullComputerClaimed || durable.DataImageDisposableClaimed {
		return fmt.Errorf("base durable-state-slice probe: durable contract carries protected-surface or full-computer claims")
	}
	return nil
}

func validateBaseDurableStateSliceProbeEvidence(readiness BaseDurableStateSliceReadinessContract, evidence BaseDurableStateSliceProbeEvidence) error {
	if strings.TrimSpace(evidence.DurableStateSliceReadinessRef) == "" || strings.TrimSpace(evidence.DurableStateSliceContractRef) == "" || strings.TrimSpace(evidence.FileManifestProbeRef) == "" || strings.TrimSpace(evidence.BlobContentProbeRef) == "" || strings.TrimSpace(evidence.ProbeEvidenceRef) == "" || strings.TrimSpace(evidence.ResidualRiskRef) == "" || strings.TrimSpace(evidence.RollbackPlanRef) == "" {
		return fmt.Errorf("base durable-state-slice probe: evidence refs are required")
	}
	if strings.TrimSpace(evidence.DurableStateSliceContractRef) != strings.TrimSpace(readiness.DurableStateSlicePlanRef) || strings.TrimSpace(evidence.FileManifestProbeRef) != strings.TrimSpace(readiness.FileManifestProbeRef) || strings.TrimSpace(evidence.BlobContentProbeRef) != strings.TrimSpace(readiness.BlobContentProbeRef) || strings.TrimSpace(evidence.ResidualRiskRef) != strings.TrimSpace(readiness.ResidualRiskRef) || strings.TrimSpace(evidence.RollbackPlanRef) != strings.TrimSpace(readiness.RollbackPlanRef) {
		return fmt.Errorf("base durable-state-slice probe: evidence refs do not match readiness")
	}
	if !evidence.NoRuntimeMaterialization || !evidence.NoOpaqueDataImageDependency || !evidence.NoDurableComputerMutation || !evidence.NoPackagePublicationMutation || !evidence.NoPromotionMutation || !evidence.NoRunAcceptanceMutation || !evidence.NoProductionMutation {
		return fmt.Errorf("base durable-state-slice probe: evidence must prove no runtime, opaque-data-img, durable-computer, package publication, promotion, run-acceptance, or production mutation")
	}
	if evidence.RuntimeMaterialized || evidence.DurableComputerStateMutated || evidence.PackagePublished || evidence.PromotionExecuted || evidence.RunAcceptanceRecordTouched || evidence.ProductionStateMutated || evidence.FullSubstrateClaimed || evidence.CompletionClaimed {
		return fmt.Errorf("base durable-state-slice probe: evidence carries materialization, mutation, downstream execution, production, full-substrate, or completion claims")
	}
	return nil
}
