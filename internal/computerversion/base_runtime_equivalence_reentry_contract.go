package computerversion

import (
	"fmt"
	"strings"
)

const BaseRuntimeEquivalenceReentryContractKind = "base_runtime_equivalence_reentry_contract"

const BaseRuntimeEquivalenceReentryBoundary = "runtime_equivalence_reentry_narrowed_without_vm_lifecycle_or_downstream_mutation"

const BaseRuntimeEquivalenceReentryScope = "runtime_materialization_bridge_to_narrowed_runtime_equivalence_only"

const BaseRuntimeEquivalenceReentryStatusNarrowed = "runtime_equivalence_reentry_narrowed_not_deployed"

// BaseRuntimeEquivalenceReentryEvidence records refs for re-entering the
// runtime-equivalence boundary through the runtime-materialization bridge. It
// does not mutate VM lifecycle, durable computer state, deployed routing,
// production state, package publication, promotion, run acceptance, or
// completion state.
type BaseRuntimeEquivalenceReentryEvidence struct {
	RuntimeMaterializationBridgeRef string `json:"runtime_materialization_bridge_ref"`
	RuntimeEquivalenceBoundaryRef   string `json:"runtime_equivalence_boundary_ref"`
	ReentryReviewRef                string `json:"reentry_review_ref"`
	RollbackPlanRef                 string `json:"rollback_plan_ref"`
	NoVMLifecycleMutation           bool   `json:"no_vm_lifecycle_mutation"`
	NoDurableComputerMutation       bool   `json:"no_durable_computer_mutation"`
	NoDeployedRouteMutation         bool   `json:"no_deployed_route_mutation"`
	NoProductionMutation            bool   `json:"no_production_mutation"`
	NoPackagePublicationMutation    bool   `json:"no_package_publication_mutation"`
	NoPromotionMutation             bool   `json:"no_promotion_mutation"`
	NoRunAcceptanceMutation         bool   `json:"no_run_acceptance_mutation"`
	RuntimeBehaviorChanged          bool   `json:"runtime_behavior_changed"`
	DurableComputerStateMutated     bool   `json:"durable_computer_state_mutated"`
	DeployedRouteRegistered         bool   `json:"deployed_route_registered"`
	ProductionAuthTouched           bool   `json:"production_auth_touched"`
	ProductionStateMutated          bool   `json:"production_state_mutated"`
	PackagePublished                bool   `json:"package_published"`
	PromotionExecuted               bool   `json:"promotion_executed"`
	VMLifecycleTouched              bool   `json:"vm_lifecycle_touched"`
	FirecrackerBootClaimed          bool   `json:"firecracker_boot_claimed"`
	RunAcceptanceRecordTouched      bool   `json:"run_acceptance_record_touched"`
	FullSubstrateClaimed            bool   `json:"full_substrate_claimed"`
	CompletionClaimed               bool   `json:"completion_claimed"`
}

// BaseRuntimeEquivalenceReentryContract records that the runtime-materialization
// bridge can re-enter runtime equivalence only as a narrowed result. It does not
// turn vmmanager metadata into durable file/blob equivalence.
type BaseRuntimeEquivalenceReentryContract struct {
	Kind                             string                  `json:"kind"`
	Version                          ComputerVersion         `json:"version"`
	Boundary                         string                  `json:"boundary"`
	Scope                            string                  `json:"scope"`
	TypedArtifactProgramRef          string                  `json:"typed_artifact_program_ref"`
	RuntimeMaterializationBridgeRef  string                  `json:"runtime_materialization_bridge_ref"`
	RuntimeEquivalenceBoundaryRef    string                  `json:"runtime_equivalence_boundary_ref"`
	ReentryReviewRef                 string                  `json:"reentry_review_ref"`
	RollbackPlanRef                  string                  `json:"rollback_plan_ref"`
	SourceMaterializerReadinessRef   string                  `json:"source_materializer_readiness_ref"`
	RuntimeMaterializationRef        string                  `json:"runtime_materialization_ref"`
	RuntimeEvidenceReviewRef         string                  `json:"runtime_evidence_review_ref"`
	SourceProvenanceReadinessRef     string                  `json:"source_provenance_readiness_ref"`
	MaterializerBoundaryRef          string                  `json:"materializer_boundary_ref"`
	RealizationEvidenceRef           string                  `json:"realization_evidence_ref"`
	Materializer                     string                  `json:"materializer"`
	Substrate                        string                  `json:"substrate"`
	SourceRequiredObservations       []ObservationKind       `json:"source_required_observations"`
	RuntimeRequiredObservations      []ObservationKind       `json:"runtime_required_observations"`
	UnsupportedDurableObservations   []UnsupportedCapability `json:"unsupported_durable_observations"`
	ReentryStatus                    string                  `json:"reentry_status"`
	RuntimeEvidenceAccepted          bool                    `json:"runtime_evidence_accepted"`
	RuntimeEquivalenceNarrowed       bool                    `json:"runtime_equivalence_narrowed"`
	RuntimeEquivalenceClaimed        bool                    `json:"runtime_equivalence_claimed"`
	DurableStateEquivalenceRequired  bool                    `json:"durable_state_equivalence_required"`
	StagingProofRequired             bool                    `json:"staging_proof_required"`
	PromotionProofRequired           bool                    `json:"promotion_proof_required"`
	PackagePublicationRequired       bool                    `json:"package_publication_required"`
	RunAcceptanceProofRequired       bool                    `json:"run_acceptance_proof_required"`
	FullSubstrateProofRequired       bool                    `json:"full_substrate_proof_required"`
	VMLifecycleMutationAllowed       bool                    `json:"vm_lifecycle_mutation_allowed"`
	DurableComputerMutationAllowed   bool                    `json:"durable_computer_mutation_allowed"`
	DeployedRouteRegistrationAllowed bool                    `json:"deployed_route_registration_allowed"`
	ProductionMutationAllowed        bool                    `json:"production_mutation_allowed"`
	PackagePublicationAllowed        bool                    `json:"package_publication_allowed"`
	PromotionAllowed                 bool                    `json:"promotion_allowed"`
	RunAcceptanceSynthesisAllowed    bool                    `json:"run_acceptance_synthesis_allowed"`
	NoVMLifecycleMutation            bool                    `json:"no_vm_lifecycle_mutation"`
	NoDurableComputerMutation        bool                    `json:"no_durable_computer_mutation"`
	NoDeployedRouteMutation          bool                    `json:"no_deployed_route_mutation"`
	NoProductionMutation             bool                    `json:"no_production_mutation"`
	NoPackagePublicationMutation     bool                    `json:"no_package_publication_mutation"`
	NoPromotionMutation              bool                    `json:"no_promotion_mutation"`
	NoRunAcceptanceMutation          bool                    `json:"no_run_acceptance_mutation"`
	RuntimeBehaviorChanged           bool                    `json:"runtime_behavior_changed"`
	DurableComputerStateMutated      bool                    `json:"durable_computer_state_mutated"`
	DeployedRouteRegistered          bool                    `json:"deployed_route_registered"`
	ProductionAuthTouched            bool                    `json:"production_auth_touched"`
	ProductionStateMutated           bool                    `json:"production_state_mutated"`
	PackagePublished                 bool                    `json:"package_published"`
	PromotionExecuted                bool                    `json:"promotion_executed"`
	VMLifecycleTouched               bool                    `json:"vm_lifecycle_touched"`
	FirecrackerBootClaimed           bool                    `json:"firecracker_boot_claimed"`
	RunAcceptanceRecordTouched       bool                    `json:"run_acceptance_record_touched"`
	FullSubstrateClaimed             bool                    `json:"full_substrate_claimed"`
	CompletionClaimed                bool                    `json:"completion_claimed"`
}

// BuildBaseRuntimeEquivalenceReentryContract consumes the runtime bridge and
// existing narrowed runtime-equivalence boundary, preserving the narrowed result
// and all downstream proof obligations.
func BuildBaseRuntimeEquivalenceReentryContract(bridge BaseRuntimeMaterializationBridgeContract, equivalence BaseRuntimeEquivalenceBoundaryContract, evidence BaseRuntimeEquivalenceReentryEvidence) (BaseRuntimeEquivalenceReentryContract, error) {
	if err := validateBaseRuntimeEquivalenceReentryBridge(bridge); err != nil {
		return BaseRuntimeEquivalenceReentryContract{}, err
	}
	if err := validateBaseRuntimeEquivalenceReentryBoundary(bridge, equivalence); err != nil {
		return BaseRuntimeEquivalenceReentryContract{}, err
	}
	if err := validateBaseRuntimeEquivalenceReentryEvidence(evidence); err != nil {
		return BaseRuntimeEquivalenceReentryContract{}, err
	}

	return BaseRuntimeEquivalenceReentryContract{
		Kind:                             BaseRuntimeEquivalenceReentryContractKind,
		Version:                          bridge.Version,
		Boundary:                         BaseRuntimeEquivalenceReentryBoundary,
		Scope:                            BaseRuntimeEquivalenceReentryScope,
		TypedArtifactProgramRef:          strings.TrimSpace(bridge.TypedArtifactProgramRef),
		RuntimeMaterializationBridgeRef:  strings.TrimSpace(evidence.RuntimeMaterializationBridgeRef),
		RuntimeEquivalenceBoundaryRef:    strings.TrimSpace(evidence.RuntimeEquivalenceBoundaryRef),
		ReentryReviewRef:                 strings.TrimSpace(evidence.ReentryReviewRef),
		RollbackPlanRef:                  strings.TrimSpace(evidence.RollbackPlanRef),
		SourceMaterializerReadinessRef:   strings.TrimSpace(bridge.SourceMaterializerReadinessRef),
		RuntimeMaterializationRef:        strings.TrimSpace(bridge.RuntimeMaterializationRef),
		RuntimeEvidenceReviewRef:         strings.TrimSpace(bridge.RuntimeEvidenceReviewRef),
		SourceProvenanceReadinessRef:     strings.TrimSpace(bridge.SourceProvenanceReadinessRef),
		MaterializerBoundaryRef:          strings.TrimSpace(bridge.MaterializerBoundaryRef),
		RealizationEvidenceRef:           strings.TrimSpace(bridge.RealizationEvidenceRef),
		Materializer:                     strings.TrimSpace(bridge.Materializer),
		Substrate:                        strings.TrimSpace(bridge.Substrate),
		SourceRequiredObservations:       canonicalObservationKinds(equivalence.SourceRequiredObservations),
		RuntimeRequiredObservations:      canonicalObservationKinds(equivalence.RuntimeRequiredObservations),
		UnsupportedDurableObservations:   canonicalUnsupportedCapabilities(equivalence.UnsupportedDurableObservations),
		ReentryStatus:                    BaseRuntimeEquivalenceReentryStatusNarrowed,
		RuntimeEvidenceAccepted:          true,
		RuntimeEquivalenceNarrowed:       true,
		RuntimeEquivalenceClaimed:        false,
		DurableStateEquivalenceRequired:  true,
		StagingProofRequired:             true,
		PromotionProofRequired:           true,
		PackagePublicationRequired:       true,
		RunAcceptanceProofRequired:       true,
		FullSubstrateProofRequired:       true,
		VMLifecycleMutationAllowed:       false,
		DurableComputerMutationAllowed:   false,
		DeployedRouteRegistrationAllowed: false,
		ProductionMutationAllowed:        false,
		PackagePublicationAllowed:        false,
		PromotionAllowed:                 false,
		RunAcceptanceSynthesisAllowed:    false,
		NoVMLifecycleMutation:            true,
		NoDurableComputerMutation:        true,
		NoDeployedRouteMutation:          true,
		NoProductionMutation:             true,
		NoPackagePublicationMutation:     true,
		NoPromotionMutation:              true,
		NoRunAcceptanceMutation:          true,
	}, nil
}

func validateBaseRuntimeEquivalenceReentryBridge(bridge BaseRuntimeMaterializationBridgeContract) error {
	if bridge.Kind != BaseRuntimeMaterializationBridgeContractKind {
		return fmt.Errorf("base runtime equivalence reentry: bridge kind is %q", bridge.Kind)
	}
	if bridge.Boundary != BaseRuntimeMaterializationBridgeBoundary {
		return fmt.Errorf("base runtime equivalence reentry: bridge boundary is %q", bridge.Boundary)
	}
	if bridge.Scope != BaseRuntimeMaterializationBridgeScope {
		return fmt.Errorf("base runtime equivalence reentry: bridge scope is %q", bridge.Scope)
	}
	if !bridge.Version.Valid() || !bridge.Version.ArtifactProgramRef.Valid() || ArtifactProgramRef(bridge.TypedArtifactProgramRef) != bridge.Version.ArtifactProgramRef {
		return fmt.Errorf("base runtime equivalence reentry: bridge version or artifact ref is invalid")
	}
	if strings.TrimSpace(bridge.SourceMaterializerReadinessRef) == "" || strings.TrimSpace(bridge.RuntimeMaterializationRef) == "" || strings.TrimSpace(bridge.RuntimeEvidenceReviewRef) == "" || strings.TrimSpace(bridge.BridgeReviewRef) == "" || strings.TrimSpace(bridge.RollbackPlanRef) == "" || strings.TrimSpace(bridge.SourceProvenanceReadinessRef) == "" || strings.TrimSpace(bridge.MaterializerBoundaryRef) == "" || strings.TrimSpace(bridge.RealizationEvidenceRef) == "" || strings.TrimSpace(bridge.MaterializationCommandRef) == "" || strings.TrimSpace(bridge.RealizationID) == "" || strings.TrimSpace(bridge.Materializer) == "" || strings.TrimSpace(bridge.Substrate) == "" {
		return fmt.Errorf("base runtime equivalence reentry: bridge refs are required")
	}
	if bridge.BridgeStatus != BaseRuntimeMaterializationBridgeStatusAccepted {
		return fmt.Errorf("base runtime equivalence reentry: bridge status is %q", bridge.BridgeStatus)
	}
	if !baseSubstrateEquivalenceHasRequiredScope(bridge.SourceRequiredObservations) || !observationKindsContain(bridge.RuntimeRequiredObservations, ObservationVMStateManifest) {
		return fmt.Errorf("base runtime equivalence reentry: bridge observations are incomplete")
	}
	if !bridge.SourceMaterializerReady || !bridge.RuntimeEvidenceAccepted || !bridge.RuntimeEquivalenceRequired {
		return fmt.Errorf("base runtime equivalence reentry: bridge must carry accepted runtime evidence")
	}
	if !bridge.StagingProofRequired || !bridge.PromotionProofRequired || !bridge.PackagePublicationRequired || !bridge.RunAcceptanceProofRequired || !bridge.FullSubstrateProofRequired {
		return fmt.Errorf("base runtime equivalence reentry: bridge must preserve downstream proof requirements")
	}
	if bridge.VMLifecycleMutationAllowed || bridge.DurableComputerMutationAllowed || bridge.DeployedRouteRegistrationAllowed || bridge.ProductionMutationAllowed || bridge.PackagePublicationAllowed || bridge.PromotionAllowed || bridge.RunAcceptanceSynthesisAllowed {
		return fmt.Errorf("base runtime equivalence reentry: bridge allows downstream execution")
	}
	if !bridge.NoVMLifecycleMutation || !bridge.NoDurableComputerMutation || !bridge.NoDeployedRouteMutation || !bridge.NoProductionMutation || !bridge.NoPackagePublicationMutation || !bridge.NoPromotionMutation || !bridge.NoRunAcceptanceMutation {
		return fmt.Errorf("base runtime equivalence reentry: bridge must prove no mutation")
	}
	if bridge.RuntimeBehaviorChanged || bridge.DurableComputerStateMutated || bridge.DeployedRouteRegistered || bridge.ProductionAuthTouched || bridge.ProductionStateMutated || bridge.PackagePublished || bridge.PromotionExecuted || bridge.VMLifecycleTouched || bridge.FirecrackerBootClaimed || bridge.RunAcceptanceRecordTouched || bridge.FullSubstrateClaimed || bridge.CompletionClaimed {
		return fmt.Errorf("base runtime equivalence reentry: bridge carries protected-surface or completion claims")
	}
	return nil
}

func validateBaseRuntimeEquivalenceReentryBoundary(bridge BaseRuntimeMaterializationBridgeContract, equivalence BaseRuntimeEquivalenceBoundaryContract) error {
	if equivalence.Kind != BaseRuntimeEquivalenceBoundaryContractKind {
		return fmt.Errorf("base runtime equivalence reentry: equivalence kind is %q", equivalence.Kind)
	}
	if equivalence.Boundary != BaseRuntimeEquivalenceBoundary {
		return fmt.Errorf("base runtime equivalence reentry: equivalence boundary is %q", equivalence.Boundary)
	}
	if equivalence.Scope != BaseRuntimeEquivalenceScope {
		return fmt.Errorf("base runtime equivalence reentry: equivalence scope is %q", equivalence.Scope)
	}
	if equivalence.Version != bridge.Version {
		return fmt.Errorf("base runtime equivalence reentry: equivalence version does not match bridge")
	}
	if strings.TrimSpace(equivalence.TypedArtifactProgramRef) == "" || strings.TrimSpace(equivalence.TypedArtifactProgramRef) != strings.TrimSpace(bridge.TypedArtifactProgramRef) || ArtifactProgramRef(equivalence.TypedArtifactProgramRef) != bridge.Version.ArtifactProgramRef {
		return fmt.Errorf("base runtime equivalence reentry: equivalence typed artifact ref is invalid")
	}
	if strings.TrimSpace(equivalence.RuntimeMaterializationCeremonyRef) == "" || strings.TrimSpace(equivalence.RuntimeMaterializationCeremonyRef) != strings.TrimSpace(bridge.RuntimeMaterializationRef) || strings.TrimSpace(equivalence.RuntimeEquivalenceEvidenceRef) == "" || strings.TrimSpace(equivalence.SourceProvenanceReadinessRef) != strings.TrimSpace(bridge.SourceProvenanceReadinessRef) || strings.TrimSpace(equivalence.RealizationEvidenceRef) != strings.TrimSpace(bridge.RealizationEvidenceRef) || strings.TrimSpace(equivalence.Materializer) != strings.TrimSpace(bridge.Materializer) || strings.TrimSpace(equivalence.Substrate) != strings.TrimSpace(bridge.Substrate) {
		return fmt.Errorf("base runtime equivalence reentry: equivalence refs do not match bridge")
	}
	if !baseSubstrateEquivalenceHasRequiredScope(equivalence.SourceRequiredObservations) || !observationKindsContain(equivalence.RuntimeRequiredObservations, ObservationVMStateManifest) {
		return fmt.Errorf("base runtime equivalence reentry: equivalence observations are incomplete")
	}
	if !unsupportedCapabilityContains(equivalence.UnsupportedDurableObservations, ObservationFileManifest) || !unsupportedCapabilityContains(equivalence.UnsupportedDurableObservations, ObservationBlobSet) {
		return fmt.Errorf("base runtime equivalence reentry: equivalence must remain narrowed by unsupported file_manifest and blob_set observations")
	}
	if equivalence.RuntimeEquivalenceStatus != EquivalenceNarrowed || !equivalence.RuntimeEquivalenceNarrowed || equivalence.RuntimeEquivalenceClaimed {
		return fmt.Errorf("base runtime equivalence reentry: equivalence must remain narrowed")
	}
	if !equivalence.DurableStateEquivalenceRequired || !equivalence.StagingProofRequired || !equivalence.PromotionProofRequired || !equivalence.PackagePublicationRequired {
		return fmt.Errorf("base runtime equivalence reentry: equivalence must preserve downstream proof requirements")
	}
	if !equivalence.NoVMLifecycleMutation || !equivalence.NoProductionMutation {
		return fmt.Errorf("base runtime equivalence reentry: equivalence must prove no VM lifecycle or production mutation")
	}
	if equivalence.RuntimeBehaviorChanged || equivalence.DeployedRouteRegistered || equivalence.ProductionAuthTouched || equivalence.StagingClaimed || equivalence.PromotionClaimed || equivalence.VMLifecycleTouched || equivalence.FirecrackerBootClaimed || equivalence.RunAcceptanceRecordTouched || equivalence.PackagePublicationClaimed || equivalence.FullSubstrateClaimed || equivalence.CompletionClaimed {
		return fmt.Errorf("base runtime equivalence reentry: equivalence carries protected-surface or completion claims")
	}
	return nil
}

func validateBaseRuntimeEquivalenceReentryEvidence(evidence BaseRuntimeEquivalenceReentryEvidence) error {
	if strings.TrimSpace(evidence.RuntimeMaterializationBridgeRef) == "" || strings.TrimSpace(evidence.RuntimeEquivalenceBoundaryRef) == "" || strings.TrimSpace(evidence.ReentryReviewRef) == "" || strings.TrimSpace(evidence.RollbackPlanRef) == "" {
		return fmt.Errorf("base runtime equivalence reentry: evidence refs are required")
	}
	if !evidence.NoVMLifecycleMutation || !evidence.NoDurableComputerMutation || !evidence.NoDeployedRouteMutation || !evidence.NoProductionMutation || !evidence.NoPackagePublicationMutation || !evidence.NoPromotionMutation || !evidence.NoRunAcceptanceMutation {
		return fmt.Errorf("base runtime equivalence reentry: evidence must prove no VM lifecycle, durable-computer, deployed-route, production, package, promotion, or run-acceptance mutation")
	}
	if evidence.RuntimeBehaviorChanged || evidence.DurableComputerStateMutated || evidence.DeployedRouteRegistered || evidence.ProductionAuthTouched || evidence.ProductionStateMutated || evidence.PackagePublished || evidence.PromotionExecuted || evidence.VMLifecycleTouched || evidence.FirecrackerBootClaimed || evidence.RunAcceptanceRecordTouched || evidence.FullSubstrateClaimed || evidence.CompletionClaimed {
		return fmt.Errorf("base runtime equivalence reentry: evidence carries runtime, mutation, downstream, full-substrate, or completion claims")
	}
	return nil
}
