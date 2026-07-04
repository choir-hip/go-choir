package computerversion

import (
	"fmt"
	"strings"
)

const BaseSourceMaterializerReadinessContractKind = "base_source_materializer_readiness_contract"

const BaseSourceMaterializerReadinessBoundary = "source_provenance_materializer_readiness_without_runtime_materialization_or_completion"

const BaseSourceMaterializerReadinessScope = "durable_slice_probe_to_source_materializer_readiness_only"

const BaseSourceMaterializerReadinessStatusReady = "ready_to_open_runtime_materialization_ceremony_not_materialized"

// BaseSourceMaterializerReadinessEvidence records refs that bind the scoped
// durable-state-slice probe to source-provenance and materializer readiness. It
// does not materialize runtime state, mutate durable computer state, publish
// packages, execute promotion, synthesize run acceptance, mutate production
// state, or claim full-substrate completion.
type BaseSourceMaterializerReadinessEvidence struct {
	DurableStateSliceProbeRef    string `json:"durable_state_slice_probe_ref"`
	SourceProvenanceReadinessRef string `json:"source_provenance_readiness_ref"`
	MaterializerBoundaryRef      string `json:"materializer_boundary_ref"`
	MaterializerReadinessPlanRef string `json:"materializer_readiness_plan_ref"`
	ResidualRiskRef              string `json:"residual_risk_ref"`
	RollbackPlanRef              string `json:"rollback_plan_ref"`
	NoRuntimeMaterialization     bool   `json:"no_runtime_materialization"`
	NoOpaqueDataImageDependency  bool   `json:"no_opaque_data_img_dependency"`
	NoDurableComputerMutation    bool   `json:"no_durable_computer_mutation"`
	NoPackagePublicationMutation bool   `json:"no_package_publication_mutation"`
	NoPromotionMutation          bool   `json:"no_promotion_mutation"`
	NoRunAcceptanceMutation      bool   `json:"no_run_acceptance_mutation"`
	NoProductionMutation         bool   `json:"no_production_mutation"`
	RuntimeMaterialized          bool   `json:"runtime_materialized"`
	DurableComputerStateMutated  bool   `json:"durable_computer_state_mutated"`
	PackagePublished             bool   `json:"package_published"`
	PromotionExecuted            bool   `json:"promotion_executed"`
	RunAcceptanceRecordTouched   bool   `json:"run_acceptance_record_touched"`
	ProductionStateMutated       bool   `json:"production_state_mutated"`
	FullSubstrateClaimed         bool   `json:"full_substrate_claimed"`
	CompletionClaimed            bool   `json:"completion_claimed"`
}

// BaseSourceMaterializerReadinessContract records that scoped durable-state
// proof, source provenance, and materializer boundary evidence are ready for a
// later runtime-materialization ceremony. It is not runtime materialization,
// durable computer mutation, promotion, run acceptance, production mutation,
// full-substrate proof, or completion.
type BaseSourceMaterializerReadinessContract struct {
	Kind                              string                  `json:"kind"`
	Version                           ComputerVersion         `json:"version"`
	Boundary                          string                  `json:"boundary"`
	Scope                             string                  `json:"scope"`
	TypedArtifactProgramRef           string                  `json:"typed_artifact_program_ref"`
	DurableStateSliceProbeRef         string                  `json:"durable_state_slice_probe_ref"`
	SourceProvenanceReadinessRef      string                  `json:"source_provenance_readiness_ref"`
	MaterializerBoundaryRef           string                  `json:"materializer_boundary_ref"`
	MaterializerReadinessPlanRef      string                  `json:"materializer_readiness_plan_ref"`
	PostPromotionSettlementHandoffRef string                  `json:"post_promotion_settlement_handoff_ref"`
	DurableStateSliceContractRef      string                  `json:"durable_state_slice_contract_ref"`
	SourceProvenanceEvidenceRef       string                  `json:"source_provenance_evidence_ref"`
	RealizationRef                    string                  `json:"realization_ref"`
	CapabilityManifestRef             string                  `json:"capability_manifest_ref"`
	ObservationSetRef                 string                  `json:"observation_set_ref"`
	ResidualRiskRef                   string                  `json:"residual_risk_ref"`
	RollbackPlanRef                   string                  `json:"rollback_plan_ref"`
	ReadinessStatus                   string                  `json:"readiness_status"`
	PersistentStateClasses            []BaseDurableStateClass `json:"persistent_state_classes"`
	RequiredObservations              []ObservationKind       `json:"required_observations"`
	RequiredSemantics                 []UserSemantic          `json:"required_semantics"`
	DurableStateSliceProbeConsumed    bool                    `json:"durable_state_slice_probe_consumed"`
	SourceProvenanceReady             bool                    `json:"source_provenance_ready"`
	MaterializerBoundaryReady         bool                    `json:"materializer_boundary_ready"`
	RuntimeCeremonyMayOpen            bool                    `json:"runtime_ceremony_may_open"`
	RuntimeProofRequired              bool                    `json:"runtime_proof_required"`
	StagingProofRequired              bool                    `json:"staging_proof_required"`
	PromotionProofRequired            bool                    `json:"promotion_proof_required"`
	PackagePublicationRequired        bool                    `json:"package_publication_required"`
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

// BuildBaseSourceMaterializerReadinessContract consumes the scoped durable slice
// probe, source-provenance readiness, and materializer boundary contract to mark
// only readiness for a later red runtime-materialization ceremony.
func BuildBaseSourceMaterializerReadinessContract(probe BaseDurableStateSliceProbeContract, source BaseSourceProvenanceReadinessContract, materializer BaseMaterializerBoundaryContract, evidence BaseSourceMaterializerReadinessEvidence) (BaseSourceMaterializerReadinessContract, error) {
	if err := validateBaseSourceMaterializerReadinessProbe(probe); err != nil {
		return BaseSourceMaterializerReadinessContract{}, err
	}
	if err := validateBaseSourceMaterializerReadinessSource(probe, source); err != nil {
		return BaseSourceMaterializerReadinessContract{}, err
	}
	if err := validateBaseSourceMaterializerReadinessMaterializer(probe, materializer); err != nil {
		return BaseSourceMaterializerReadinessContract{}, err
	}
	if err := validateBaseSourceMaterializerReadinessEvidence(evidence); err != nil {
		return BaseSourceMaterializerReadinessContract{}, err
	}

	return BaseSourceMaterializerReadinessContract{
		Kind:                              BaseSourceMaterializerReadinessContractKind,
		Version:                           probe.Version,
		Boundary:                          BaseSourceMaterializerReadinessBoundary,
		Scope:                             BaseSourceMaterializerReadinessScope,
		TypedArtifactProgramRef:           strings.TrimSpace(probe.TypedArtifactProgramRef),
		DurableStateSliceProbeRef:         strings.TrimSpace(evidence.DurableStateSliceProbeRef),
		SourceProvenanceReadinessRef:      strings.TrimSpace(evidence.SourceProvenanceReadinessRef),
		MaterializerBoundaryRef:           strings.TrimSpace(evidence.MaterializerBoundaryRef),
		MaterializerReadinessPlanRef:      strings.TrimSpace(evidence.MaterializerReadinessPlanRef),
		PostPromotionSettlementHandoffRef: strings.TrimSpace(probe.PostPromotionSettlementHandoffRef),
		DurableStateSliceContractRef:      strings.TrimSpace(probe.DurableStateSliceContractRef),
		SourceProvenanceEvidenceRef:       strings.TrimSpace(source.SourceProvenanceEvidenceRef),
		RealizationRef:                    strings.TrimSpace(materializer.RealizationRef),
		CapabilityManifestRef:             strings.TrimSpace(materializer.CapabilityManifestRef),
		ObservationSetRef:                 strings.TrimSpace(materializer.ObservationSetRef),
		ResidualRiskRef:                   strings.TrimSpace(evidence.ResidualRiskRef),
		RollbackPlanRef:                   strings.TrimSpace(evidence.RollbackPlanRef),
		ReadinessStatus:                   BaseSourceMaterializerReadinessStatusReady,
		PersistentStateClasses:            []BaseDurableStateClass{BaseDurableStateClassBlobContent, BaseDurableStateClassFileManifest},
		RequiredObservations:              []ObservationKind{ObservationBlobSet, ObservationFileManifest},
		RequiredSemantics:                 []UserSemantic{UserSemanticDeletionState, UserSemanticFileContent, UserSemanticFilePath, UserSemanticFileProvenance},
		DurableStateSliceProbeConsumed:    true,
		SourceProvenanceReady:             true,
		MaterializerBoundaryReady:         true,
		RuntimeCeremonyMayOpen:            true,
		RuntimeProofRequired:              true,
		StagingProofRequired:              true,
		PromotionProofRequired:            true,
		PackagePublicationRequired:        true,
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

func validateBaseSourceMaterializerReadinessProbe(probe BaseDurableStateSliceProbeContract) error {
	if probe.Kind != BaseDurableStateSliceProbeContractKind {
		return fmt.Errorf("base source materializer readiness: probe kind is %q", probe.Kind)
	}
	if probe.Boundary != BaseDurableStateSliceProbeBoundary {
		return fmt.Errorf("base source materializer readiness: probe boundary is %q", probe.Boundary)
	}
	if probe.Scope != BaseDurableStateSliceProbeScope {
		return fmt.Errorf("base source materializer readiness: probe scope is %q", probe.Scope)
	}
	if !probe.Version.Valid() {
		return fmt.Errorf("base source materializer readiness: probe version is invalid")
	}
	if !probe.Version.ArtifactProgramRef.Valid() || ArtifactProgramRef(probe.TypedArtifactProgramRef) != probe.Version.ArtifactProgramRef {
		return fmt.Errorf("base source materializer readiness: probe typed artifact-program ref is invalid")
	}
	if strings.TrimSpace(probe.DurableStateSliceReadinessRef) == "" || strings.TrimSpace(probe.DurableStateSliceContractRef) == "" || strings.TrimSpace(probe.PostPromotionSettlementHandoffRef) == "" || strings.TrimSpace(probe.NextSubstrateProofPlanRef) == "" || strings.TrimSpace(probe.FileManifestProbeRef) == "" || strings.TrimSpace(probe.BlobContentProbeRef) == "" || strings.TrimSpace(probe.ProbeEvidenceRef) == "" || strings.TrimSpace(probe.ResidualRiskRef) == "" || strings.TrimSpace(probe.RollbackPlanRef) == "" {
		return fmt.Errorf("base source materializer readiness: probe refs are required")
	}
	if probe.ProbeStatus != BaseDurableStateSliceProbeStatusProven {
		return fmt.Errorf("base source materializer readiness: probe status is %q", probe.ProbeStatus)
	}
	if !probe.ReadinessConsumed || !probe.DurableStateSliceProven || !probe.FileManifestProbeRecorded || !probe.BlobContentProbeRecorded {
		return fmt.Errorf("base source materializer readiness: probe must record durable state slice proof")
	}
	if !baseDurableStateSliceHasRequiredClasses(probe.PersistentStateClasses) || !baseSubstrateEquivalenceHasRequiredScope(probe.RequiredObservations) || !baseDurableStateSliceHasRequiredSemantics(probe.RequiredSemantics) {
		return fmt.Errorf("base source materializer readiness: probe must cover file/blob durable state semantics")
	}
	if !probe.RuntimeProofRequired || !probe.StagingProofRequired || !probe.PromotionProofRequired || !probe.RunAcceptanceProofRequired || !probe.FullSubstrateProofRequired {
		return fmt.Errorf("base source materializer readiness: probe must preserve downstream proof requirements")
	}
	if probe.RuntimeMaterializationAllowed || probe.DurableComputerMutationAllowed || probe.PackagePublicationAllowed || probe.PromotionAllowed || probe.RunAcceptanceSynthesisAllowed {
		return fmt.Errorf("base source materializer readiness: probe allows downstream execution")
	}
	if !probe.NoRuntimeMaterialization || !probe.NoOpaqueDataImageDependency || !probe.NoDurableComputerMutation || !probe.NoPackagePublicationMutation || !probe.NoPromotionMutation || !probe.NoRunAcceptanceMutation || !probe.NoProductionMutation {
		return fmt.Errorf("base source materializer readiness: probe must prove no runtime, durable-computer, package publication, promotion, run-acceptance, or production mutation")
	}
	if probe.RuntimeMaterialized || probe.DurableComputerStateMutated || probe.PackagePublished || probe.PromotionExecuted || probe.RunAcceptanceRecordTouched || probe.ProductionStateMutated || probe.FullSubstrateClaimed || probe.CompletionClaimed {
		return fmt.Errorf("base source materializer readiness: probe carries materialization, mutation, downstream execution, production, full-substrate, or completion claims")
	}
	return nil
}

func validateBaseSourceMaterializerReadinessSource(probe BaseDurableStateSliceProbeContract, source BaseSourceProvenanceReadinessContract) error {
	if source.Kind != BaseSourceProvenanceReadinessContractKind {
		return fmt.Errorf("base source materializer readiness: source kind is %q", source.Kind)
	}
	if source.Boundary != BaseSourceProvenanceReadinessBoundary {
		return fmt.Errorf("base source materializer readiness: source boundary is %q", source.Boundary)
	}
	if source.Scope != BaseSourceProvenanceReadinessScope {
		return fmt.Errorf("base source materializer readiness: source scope is %q", source.Scope)
	}
	if source.Version != probe.Version {
		return fmt.Errorf("base source materializer readiness: source version does not match probe")
	}
	if strings.TrimSpace(source.TypedArtifactProgramRef) == "" || strings.TrimSpace(source.TypedArtifactProgramRef) != strings.TrimSpace(probe.TypedArtifactProgramRef) || ArtifactProgramRef(strings.TrimSpace(source.TypedArtifactProgramRef)) != probe.Version.ArtifactProgramRef {
		return fmt.Errorf("base source materializer readiness: source typed artifact-program ref is invalid")
	}
	if strings.TrimSpace(source.DurableStateSliceRef) == "" || strings.TrimSpace(source.DurableStateSliceRef) != strings.TrimSpace(probe.DurableStateSliceContractRef) || strings.TrimSpace(source.SourceProvenanceEvidenceRef) == "" {
		return fmt.Errorf("base source materializer readiness: source proof refs are required")
	}
	if !source.LocalFileBlobProofSummarized || !source.SourceProvenanceReady || !source.RuntimeCeremonyMayOpen {
		return fmt.Errorf("base source materializer readiness: source contract must be ready for runtime ceremony planning")
	}
	if !baseDurableStateSliceHasRequiredClasses(source.PersistentStateClasses) || !baseSubstrateEquivalenceHasRequiredScope(source.RequiredObservations) || !baseDurableStateSliceHasRequiredSemantics(source.RequiredSemantics) {
		return fmt.Errorf("base source materializer readiness: source must cover file/blob durable state semantics")
	}
	if !source.RuntimeProofRequired || !source.StagingProofRequired || !source.PromotionProofRequired || !source.PackagePublicationRequired {
		return fmt.Errorf("base source materializer readiness: source must preserve downstream proof requirements")
	}
	if !source.NoRuntimeMaterialization || !source.NoOpaqueDataImageDependency || !source.NoMutation {
		return fmt.Errorf("base source materializer readiness: source must be no-runtime no-opaque-data-img and no-mutation")
	}
	if source.RuntimeBehaviorChanged || source.DeployedRouteRegistered || source.ProductionAuthTouched || source.StagingClaimed || source.PromotionClaimed || source.VMLifecycleTouched || source.FirecrackerBootClaimed || source.RunAcceptanceRecordTouched || source.PackagePublicationClaimed || source.FullSubstrateClaimed || source.CompletionClaimed {
		return fmt.Errorf("base source materializer readiness: source carries protected-surface or completion claims")
	}
	return nil
}

func validateBaseSourceMaterializerReadinessMaterializer(probe BaseDurableStateSliceProbeContract, materializer BaseMaterializerBoundaryContract) error {
	if materializer.Kind != BaseMaterializerBoundaryContractKind {
		return fmt.Errorf("base source materializer readiness: materializer kind is %q", materializer.Kind)
	}
	if materializer.Boundary != BaseMaterializerBoundary {
		return fmt.Errorf("base source materializer readiness: materializer boundary is %q", materializer.Boundary)
	}
	if materializer.Scope != BaseMaterializerScope {
		return fmt.Errorf("base source materializer readiness: materializer scope is %q", materializer.Scope)
	}
	if materializer.Version != probe.Version {
		return fmt.Errorf("base source materializer readiness: materializer version does not match probe")
	}
	if strings.TrimSpace(materializer.RealizationID) == "" || strings.TrimSpace(materializer.Materializer) == "" || strings.TrimSpace(materializer.Substrate) == "" || strings.TrimSpace(materializer.ObservationSetName) == "" || strings.TrimSpace(materializer.RealizationRef) == "" || strings.TrimSpace(materializer.CapabilityManifestRef) == "" || strings.TrimSpace(materializer.ObservationSetRef) == "" {
		return fmt.Errorf("base source materializer readiness: materializer refs are required")
	}
	if !baseSubstrateEquivalenceHasRequiredScope(materializer.RequiredObservations) {
		return fmt.Errorf("base source materializer readiness: materializer must include file_manifest and blob_set observations")
	}
	if !materializer.NoRuntimeMaterialization || !materializer.NoOpaqueDataImageDependency || !materializer.NoMutation {
		return fmt.Errorf("base source materializer readiness: materializer must be no-runtime no-opaque-data-img and no-mutation")
	}
	if materializer.RuntimeBehaviorChanged || materializer.DeployedRouteRegistered || materializer.ProductionAuthTouched || materializer.StagingClaimed || materializer.PromotionClaimed || materializer.VMLifecycleTouched || materializer.FirecrackerBootClaimed || materializer.RunAcceptanceRecordTouched || materializer.FullSubstrateIndependenceClaim {
		return fmt.Errorf("base source materializer readiness: materializer carries protected-surface or full-substrate claims")
	}
	return nil
}

func validateBaseSourceMaterializerReadinessEvidence(evidence BaseSourceMaterializerReadinessEvidence) error {
	if strings.TrimSpace(evidence.DurableStateSliceProbeRef) == "" || strings.TrimSpace(evidence.SourceProvenanceReadinessRef) == "" || strings.TrimSpace(evidence.MaterializerBoundaryRef) == "" || strings.TrimSpace(evidence.MaterializerReadinessPlanRef) == "" || strings.TrimSpace(evidence.ResidualRiskRef) == "" || strings.TrimSpace(evidence.RollbackPlanRef) == "" {
		return fmt.Errorf("base source materializer readiness: evidence refs are required")
	}
	if !evidence.NoRuntimeMaterialization || !evidence.NoOpaqueDataImageDependency || !evidence.NoDurableComputerMutation || !evidence.NoPackagePublicationMutation || !evidence.NoPromotionMutation || !evidence.NoRunAcceptanceMutation || !evidence.NoProductionMutation {
		return fmt.Errorf("base source materializer readiness: evidence must prove no runtime, opaque-data-img, durable-computer, package publication, promotion, run-acceptance, or production mutation")
	}
	if evidence.RuntimeMaterialized || evidence.DurableComputerStateMutated || evidence.PackagePublished || evidence.PromotionExecuted || evidence.RunAcceptanceRecordTouched || evidence.ProductionStateMutated || evidence.FullSubstrateClaimed || evidence.CompletionClaimed {
		return fmt.Errorf("base source materializer readiness: evidence carries materialization, mutation, downstream execution, production, full-substrate, or completion claims")
	}
	return nil
}
