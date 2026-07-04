package computerversion

import (
	"fmt"
	"strings"
)

const BaseRuntimeMaterializationBridgeContractKind = "base_runtime_materialization_bridge_contract"

const BaseRuntimeMaterializationBridgeBoundary = "runtime_materialization_ceremony_bridge_without_vm_lifecycle_or_downstream_mutation"

const BaseRuntimeMaterializationBridgeScope = "source_materializer_readiness_to_runtime_ceremony_evidence_only"

const BaseRuntimeMaterializationBridgeStatusAccepted = "runtime_ceremony_evidence_admissible_not_deployed"

// BaseRuntimeMaterializationBridgeEvidence records refs for binding
// source-provenance/materializer readiness to the existing scoped runtime
// materialization ceremony evidence. It does not mutate VM lifecycle, durable
// computer state, deployed routing, production state, promotion, package
// publication, run acceptance, or completion state.
type BaseRuntimeMaterializationBridgeEvidence struct {
	SourceMaterializerReadinessRef string `json:"source_materializer_readiness_ref"`
	RuntimeMaterializationRef      string `json:"runtime_materialization_ref"`
	RuntimeEvidenceReviewRef       string `json:"runtime_evidence_review_ref"`
	BridgeReviewRef                string `json:"bridge_review_ref"`
	RollbackPlanRef                string `json:"rollback_plan_ref"`
	NoVMLifecycleMutation          bool   `json:"no_vm_lifecycle_mutation"`
	NoDurableComputerMutation      bool   `json:"no_durable_computer_mutation"`
	NoDeployedRouteMutation        bool   `json:"no_deployed_route_mutation"`
	NoProductionMutation           bool   `json:"no_production_mutation"`
	NoPackagePublicationMutation   bool   `json:"no_package_publication_mutation"`
	NoPromotionMutation            bool   `json:"no_promotion_mutation"`
	NoRunAcceptanceMutation        bool   `json:"no_run_acceptance_mutation"`
	RuntimeBehaviorChanged         bool   `json:"runtime_behavior_changed"`
	DurableComputerStateMutated    bool   `json:"durable_computer_state_mutated"`
	DeployedRouteRegistered        bool   `json:"deployed_route_registered"`
	ProductionAuthTouched          bool   `json:"production_auth_touched"`
	ProductionStateMutated         bool   `json:"production_state_mutated"`
	PackagePublished               bool   `json:"package_published"`
	PromotionExecuted              bool   `json:"promotion_executed"`
	VMLifecycleTouched             bool   `json:"vm_lifecycle_touched"`
	FirecrackerBootClaimed         bool   `json:"firecracker_boot_claimed"`
	RunAcceptanceRecordTouched     bool   `json:"run_acceptance_record_touched"`
	FullSubstrateClaimed           bool   `json:"full_substrate_claimed"`
	CompletionClaimed              bool   `json:"completion_claimed"`
}

// BaseRuntimeMaterializationBridgeContract records that scoped runtime ceremony
// evidence is admissible after source-provenance/materializer readiness. It is
// not VM lifecycle mutation, deployed routing, production mutation, promotion,
// run acceptance, full-substrate proof, or completion.
type BaseRuntimeMaterializationBridgeContract struct {
	Kind                              string            `json:"kind"`
	Version                           ComputerVersion   `json:"version"`
	Boundary                          string            `json:"boundary"`
	Scope                             string            `json:"scope"`
	TypedArtifactProgramRef           string            `json:"typed_artifact_program_ref"`
	SourceMaterializerReadinessRef    string            `json:"source_materializer_readiness_ref"`
	RuntimeMaterializationRef         string            `json:"runtime_materialization_ref"`
	RuntimeEvidenceReviewRef          string            `json:"runtime_evidence_review_ref"`
	BridgeReviewRef                   string            `json:"bridge_review_ref"`
	RollbackPlanRef                   string            `json:"rollback_plan_ref"`
	PostPromotionSettlementHandoffRef string            `json:"post_promotion_settlement_handoff_ref"`
	SourceProvenanceReadinessRef      string            `json:"source_provenance_readiness_ref"`
	MaterializerBoundaryRef           string            `json:"materializer_boundary_ref"`
	RealizationEvidenceRef            string            `json:"realization_evidence_ref"`
	MaterializationCommandRef         string            `json:"materialization_command_ref"`
	RealizationID                     string            `json:"realization_id"`
	Materializer                      string            `json:"materializer"`
	Substrate                         string            `json:"substrate"`
	BridgeStatus                      string            `json:"bridge_status"`
	SourceRequiredObservations        []ObservationKind `json:"source_required_observations"`
	RuntimeRequiredObservations       []ObservationKind `json:"runtime_required_observations"`
	SourceMaterializerReady           bool              `json:"source_materializer_ready"`
	RuntimeEvidenceAccepted           bool              `json:"runtime_evidence_accepted"`
	RuntimeEquivalenceRequired        bool              `json:"runtime_equivalence_required"`
	StagingProofRequired              bool              `json:"staging_proof_required"`
	PromotionProofRequired            bool              `json:"promotion_proof_required"`
	PackagePublicationRequired        bool              `json:"package_publication_required"`
	RunAcceptanceProofRequired        bool              `json:"run_acceptance_proof_required"`
	FullSubstrateProofRequired        bool              `json:"full_substrate_proof_required"`
	VMLifecycleMutationAllowed        bool              `json:"vm_lifecycle_mutation_allowed"`
	DurableComputerMutationAllowed    bool              `json:"durable_computer_mutation_allowed"`
	DeployedRouteRegistrationAllowed  bool              `json:"deployed_route_registration_allowed"`
	ProductionMutationAllowed         bool              `json:"production_mutation_allowed"`
	PackagePublicationAllowed         bool              `json:"package_publication_allowed"`
	PromotionAllowed                  bool              `json:"promotion_allowed"`
	RunAcceptanceSynthesisAllowed     bool              `json:"run_acceptance_synthesis_allowed"`
	NoVMLifecycleMutation             bool              `json:"no_vm_lifecycle_mutation"`
	NoDurableComputerMutation         bool              `json:"no_durable_computer_mutation"`
	NoDeployedRouteMutation           bool              `json:"no_deployed_route_mutation"`
	NoProductionMutation              bool              `json:"no_production_mutation"`
	NoPackagePublicationMutation      bool              `json:"no_package_publication_mutation"`
	NoPromotionMutation               bool              `json:"no_promotion_mutation"`
	NoRunAcceptanceMutation           bool              `json:"no_run_acceptance_mutation"`
	RuntimeBehaviorChanged            bool              `json:"runtime_behavior_changed"`
	DurableComputerStateMutated       bool              `json:"durable_computer_state_mutated"`
	DeployedRouteRegistered           bool              `json:"deployed_route_registered"`
	ProductionAuthTouched             bool              `json:"production_auth_touched"`
	ProductionStateMutated            bool              `json:"production_state_mutated"`
	PackagePublished                  bool              `json:"package_published"`
	PromotionExecuted                 bool              `json:"promotion_executed"`
	VMLifecycleTouched                bool              `json:"vm_lifecycle_touched"`
	FirecrackerBootClaimed            bool              `json:"firecracker_boot_claimed"`
	RunAcceptanceRecordTouched        bool              `json:"run_acceptance_record_touched"`
	FullSubstrateClaimed              bool              `json:"full_substrate_claimed"`
	CompletionClaimed                 bool              `json:"completion_claimed"`
}

// BuildBaseRuntimeMaterializationBridgeContract consumes source/materializer
// readiness plus scoped runtime ceremony evidence and records only admissibility
// for runtime-equivalence follow-up work.
func BuildBaseRuntimeMaterializationBridgeContract(readiness BaseSourceMaterializerReadinessContract, runtime BaseRuntimeMaterializationCeremonyContract, evidence BaseRuntimeMaterializationBridgeEvidence) (BaseRuntimeMaterializationBridgeContract, error) {
	if err := validateBaseRuntimeMaterializationBridgeReadiness(readiness); err != nil {
		return BaseRuntimeMaterializationBridgeContract{}, err
	}
	if err := validateBaseRuntimeMaterializationBridgeRuntime(readiness, runtime); err != nil {
		return BaseRuntimeMaterializationBridgeContract{}, err
	}
	if err := validateBaseRuntimeMaterializationBridgeEvidence(evidence); err != nil {
		return BaseRuntimeMaterializationBridgeContract{}, err
	}

	return BaseRuntimeMaterializationBridgeContract{
		Kind:                              BaseRuntimeMaterializationBridgeContractKind,
		Version:                           readiness.Version,
		Boundary:                          BaseRuntimeMaterializationBridgeBoundary,
		Scope:                             BaseRuntimeMaterializationBridgeScope,
		TypedArtifactProgramRef:           strings.TrimSpace(readiness.TypedArtifactProgramRef),
		SourceMaterializerReadinessRef:    strings.TrimSpace(evidence.SourceMaterializerReadinessRef),
		RuntimeMaterializationRef:         strings.TrimSpace(evidence.RuntimeMaterializationRef),
		RuntimeEvidenceReviewRef:          strings.TrimSpace(evidence.RuntimeEvidenceReviewRef),
		BridgeReviewRef:                   strings.TrimSpace(evidence.BridgeReviewRef),
		RollbackPlanRef:                   strings.TrimSpace(evidence.RollbackPlanRef),
		PostPromotionSettlementHandoffRef: strings.TrimSpace(readiness.PostPromotionSettlementHandoffRef),
		SourceProvenanceReadinessRef:      strings.TrimSpace(readiness.SourceProvenanceReadinessRef),
		MaterializerBoundaryRef:           strings.TrimSpace(readiness.MaterializerBoundaryRef),
		RealizationEvidenceRef:            strings.TrimSpace(runtime.RealizationEvidenceRef),
		MaterializationCommandRef:         strings.TrimSpace(runtime.MaterializationCommandRef),
		RealizationID:                     strings.TrimSpace(runtime.RealizationID),
		Materializer:                      strings.TrimSpace(runtime.Materializer),
		Substrate:                         strings.TrimSpace(runtime.Substrate),
		BridgeStatus:                      BaseRuntimeMaterializationBridgeStatusAccepted,
		SourceRequiredObservations:        []ObservationKind{ObservationBlobSet, ObservationFileManifest},
		RuntimeRequiredObservations:       []ObservationKind{ObservationVMStateManifest},
		SourceMaterializerReady:           true,
		RuntimeEvidenceAccepted:           true,
		RuntimeEquivalenceRequired:        true,
		StagingProofRequired:              true,
		PromotionProofRequired:            true,
		PackagePublicationRequired:        true,
		RunAcceptanceProofRequired:        true,
		FullSubstrateProofRequired:        true,
		VMLifecycleMutationAllowed:        false,
		DurableComputerMutationAllowed:    false,
		DeployedRouteRegistrationAllowed:  false,
		ProductionMutationAllowed:         false,
		PackagePublicationAllowed:         false,
		PromotionAllowed:                  false,
		RunAcceptanceSynthesisAllowed:     false,
		NoVMLifecycleMutation:             true,
		NoDurableComputerMutation:         true,
		NoDeployedRouteMutation:           true,
		NoProductionMutation:              true,
		NoPackagePublicationMutation:      true,
		NoPromotionMutation:               true,
		NoRunAcceptanceMutation:           true,
	}, nil
}

func validateBaseRuntimeMaterializationBridgeReadiness(readiness BaseSourceMaterializerReadinessContract) error {
	if readiness.Kind != BaseSourceMaterializerReadinessContractKind {
		return fmt.Errorf("base runtime materialization bridge: readiness kind is %q", readiness.Kind)
	}
	if readiness.Boundary != BaseSourceMaterializerReadinessBoundary {
		return fmt.Errorf("base runtime materialization bridge: readiness boundary is %q", readiness.Boundary)
	}
	if readiness.Scope != BaseSourceMaterializerReadinessScope {
		return fmt.Errorf("base runtime materialization bridge: readiness scope is %q", readiness.Scope)
	}
	if !readiness.Version.Valid() || !readiness.Version.ArtifactProgramRef.Valid() || ArtifactProgramRef(readiness.TypedArtifactProgramRef) != readiness.Version.ArtifactProgramRef {
		return fmt.Errorf("base runtime materialization bridge: readiness version or artifact ref is invalid")
	}
	if strings.TrimSpace(readiness.SourceProvenanceReadinessRef) == "" || strings.TrimSpace(readiness.MaterializerBoundaryRef) == "" || strings.TrimSpace(readiness.PostPromotionSettlementHandoffRef) == "" || strings.TrimSpace(readiness.MaterializerReadinessPlanRef) == "" || strings.TrimSpace(readiness.ResidualRiskRef) == "" || strings.TrimSpace(readiness.RollbackPlanRef) == "" {
		return fmt.Errorf("base runtime materialization bridge: readiness refs are required")
	}
	if readiness.ReadinessStatus != BaseSourceMaterializerReadinessStatusReady {
		return fmt.Errorf("base runtime materialization bridge: readiness status is %q", readiness.ReadinessStatus)
	}
	if !readiness.DurableStateSliceProbeConsumed || !readiness.SourceProvenanceReady || !readiness.MaterializerBoundaryReady || !readiness.RuntimeCeremonyMayOpen {
		return fmt.Errorf("base runtime materialization bridge: readiness must open runtime ceremony planning")
	}
	if !readiness.RuntimeProofRequired || !readiness.StagingProofRequired || !readiness.PromotionProofRequired || !readiness.PackagePublicationRequired || !readiness.RunAcceptanceProofRequired || !readiness.FullSubstrateProofRequired {
		return fmt.Errorf("base runtime materialization bridge: readiness must preserve downstream proof requirements")
	}
	if readiness.RuntimeMaterializationAllowed || readiness.DurableComputerMutationAllowed || readiness.PackagePublicationAllowed || readiness.PromotionAllowed || readiness.RunAcceptanceSynthesisAllowed {
		return fmt.Errorf("base runtime materialization bridge: readiness allows downstream execution")
	}
	if !readiness.NoRuntimeMaterialization || !readiness.NoOpaqueDataImageDependency || !readiness.NoDurableComputerMutation || !readiness.NoPackagePublicationMutation || !readiness.NoPromotionMutation || !readiness.NoRunAcceptanceMutation || !readiness.NoProductionMutation {
		return fmt.Errorf("base runtime materialization bridge: readiness must prove no runtime, durable-computer, package publication, promotion, run-acceptance, or production mutation")
	}
	if readiness.RuntimeMaterialized || readiness.DurableComputerStateMutated || readiness.PackagePublished || readiness.PromotionExecuted || readiness.RunAcceptanceRecordTouched || readiness.ProductionStateMutated || readiness.FullSubstrateClaimed || readiness.CompletionClaimed {
		return fmt.Errorf("base runtime materialization bridge: readiness carries materialization, mutation, downstream execution, production, full-substrate, or completion claims")
	}
	return nil
}

func validateBaseRuntimeMaterializationBridgeRuntime(readiness BaseSourceMaterializerReadinessContract, runtime BaseRuntimeMaterializationCeremonyContract) error {
	if runtime.Kind != BaseRuntimeMaterializationCeremonyContractKind {
		return fmt.Errorf("base runtime materialization bridge: runtime kind is %q", runtime.Kind)
	}
	if runtime.Boundary != BaseRuntimeMaterializationCeremonyBoundary {
		return fmt.Errorf("base runtime materialization bridge: runtime boundary is %q", runtime.Boundary)
	}
	if runtime.Scope != BaseRuntimeMaterializationCeremonyScope {
		return fmt.Errorf("base runtime materialization bridge: runtime scope is %q", runtime.Scope)
	}
	if runtime.Version != readiness.Version {
		return fmt.Errorf("base runtime materialization bridge: runtime version does not match readiness")
	}
	if strings.TrimSpace(runtime.TypedArtifactProgramRef) == "" || strings.TrimSpace(runtime.TypedArtifactProgramRef) != strings.TrimSpace(readiness.TypedArtifactProgramRef) || ArtifactProgramRef(strings.TrimSpace(runtime.TypedArtifactProgramRef)) != readiness.Version.ArtifactProgramRef {
		return fmt.Errorf("base runtime materialization bridge: runtime typed artifact ref is invalid")
	}
	if strings.TrimSpace(runtime.SourceProvenanceReadinessRef) == "" || strings.TrimSpace(runtime.SourceProvenanceReadinessRef) != strings.TrimSpace(readiness.SourceProvenanceReadinessRef) || strings.TrimSpace(runtime.RealizationEvidenceRef) == "" || strings.TrimSpace(runtime.MaterializationCommandRef) == "" || strings.TrimSpace(runtime.RealizationID) == "" || strings.TrimSpace(runtime.Materializer) == "" || strings.TrimSpace(runtime.Substrate) == "" {
		return fmt.Errorf("base runtime materialization bridge: runtime refs are required")
	}
	if !baseSubstrateEquivalenceHasRequiredScope(runtime.SourceRequiredObservations) || !observationKindsContain(runtime.RuntimeRequiredObservations, ObservationVMStateManifest) {
		return fmt.Errorf("base runtime materialization bridge: runtime observations are incomplete")
	}
	if !runtime.SourceProvenanceReady || !runtime.RuntimeEvidenceAccepted || !runtime.RuntimeEquivalenceRequired || !runtime.StagingProofRequired || !runtime.PromotionProofRequired || !runtime.PackagePublicationRequired {
		return fmt.Errorf("base runtime materialization bridge: runtime contract must preserve proof requirements")
	}
	if !runtime.NoVMLifecycleMutation || !runtime.NoProductionMutation {
		return fmt.Errorf("base runtime materialization bridge: runtime contract must prove no VM lifecycle or production mutation")
	}
	if runtime.RuntimeBehaviorChanged || runtime.DeployedRouteRegistered || runtime.ProductionAuthTouched || runtime.StagingClaimed || runtime.PromotionClaimed || runtime.VMLifecycleTouched || runtime.FirecrackerBootClaimed || runtime.RunAcceptanceRecordTouched || runtime.PackagePublicationClaimed || runtime.FullSubstrateClaimed || runtime.CompletionClaimed {
		return fmt.Errorf("base runtime materialization bridge: runtime contract carries protected-surface or completion claims")
	}
	return nil
}

func validateBaseRuntimeMaterializationBridgeEvidence(evidence BaseRuntimeMaterializationBridgeEvidence) error {
	if strings.TrimSpace(evidence.SourceMaterializerReadinessRef) == "" || strings.TrimSpace(evidence.RuntimeMaterializationRef) == "" || strings.TrimSpace(evidence.RuntimeEvidenceReviewRef) == "" || strings.TrimSpace(evidence.BridgeReviewRef) == "" || strings.TrimSpace(evidence.RollbackPlanRef) == "" {
		return fmt.Errorf("base runtime materialization bridge: evidence refs are required")
	}
	if !evidence.NoVMLifecycleMutation || !evidence.NoDurableComputerMutation || !evidence.NoDeployedRouteMutation || !evidence.NoProductionMutation || !evidence.NoPackagePublicationMutation || !evidence.NoPromotionMutation || !evidence.NoRunAcceptanceMutation {
		return fmt.Errorf("base runtime materialization bridge: evidence must prove no VM lifecycle, durable-computer, deployed-route, production, package, promotion, or run-acceptance mutation")
	}
	if evidence.RuntimeBehaviorChanged || evidence.DurableComputerStateMutated || evidence.DeployedRouteRegistered || evidence.ProductionAuthTouched || evidence.ProductionStateMutated || evidence.PackagePublished || evidence.PromotionExecuted || evidence.VMLifecycleTouched || evidence.FirecrackerBootClaimed || evidence.RunAcceptanceRecordTouched || evidence.FullSubstrateClaimed || evidence.CompletionClaimed {
		return fmt.Errorf("base runtime materialization bridge: evidence carries runtime, mutation, downstream, full-substrate, or completion claims")
	}
	return nil
}
