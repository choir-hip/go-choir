package computerversion

import (
	"fmt"
	"strings"
)

const BaseRuntimeMaterializationCeremonyContractKind = "base_runtime_materialization_ceremony_contract"

const BaseRuntimeMaterializationCeremonyBoundary = "base_runtime_materialization_evidence_without_vm_lifecycle_or_downstream_claim"

const BaseRuntimeMaterializationCeremonyScope = "vmmanager_scoped_realization_binding_to_source_provenance_readiness"

// BaseRuntimeMaterializationCeremonyEvidence records the proof refs for the
// first red runtime-materialization ceremony gate. It can accept a scoped
// vmmanager Realization as runtime-boundary evidence, but it cannot claim VM
// lifecycle mutation, Firecracker boot, staging, promotion, package publication,
// run acceptance, full substrate independence, or mission completion.
type BaseRuntimeMaterializationCeremonyEvidence struct {
	SourceProvenanceReadinessRef string `json:"source_provenance_readiness_ref"`
	RealizationEvidenceRef       string `json:"realization_evidence_ref"`
	MaterializationCommandRef    string `json:"materialization_command_ref"`
	NoVMLifecycleMutation        bool   `json:"no_vm_lifecycle_mutation"`
	NoProductionMutation         bool   `json:"no_production_mutation"`
	RuntimeBehaviorChanged       bool   `json:"runtime_behavior_changed"`
	DeployedRouteRegistered      bool   `json:"deployed_route_registered"`
	ProductionAuthTouched        bool   `json:"production_auth_touched"`
	StagingClaimed               bool   `json:"staging_claimed"`
	PromotionClaimed             bool   `json:"promotion_claimed"`
	VMLifecycleTouched           bool   `json:"vm_lifecycle_touched"`
	FirecrackerBootClaimed       bool   `json:"firecracker_boot_claimed"`
	RunAcceptanceRecordTouched   bool   `json:"run_acceptance_record_touched"`
	PackagePublicationClaimed    bool   `json:"package_publication_claimed"`
	FullSubstrateClaimed         bool   `json:"full_substrate_claimed"`
	CompletionClaimed            bool   `json:"completion_claimed"`
}

// BaseRuntimeMaterializationCeremonyContract binds source/provenance readiness
// to a scoped runtime Realization for the same ComputerVersion. It records only
// that runtime-materialization evidence is admissible at this boundary; it does
// not certify durable-state equivalence, staging behavior, promotion,
// package-publication, full substrate independence, or completion.
type BaseRuntimeMaterializationCeremonyContract struct {
	Kind                         string            `json:"kind"`
	Version                      ComputerVersion   `json:"version"`
	Boundary                     string            `json:"boundary"`
	Scope                        string            `json:"scope"`
	TypedArtifactProgramRef      string            `json:"typed_artifact_program_ref"`
	SourceProvenanceReadinessRef string            `json:"source_provenance_readiness_ref"`
	RealizationEvidenceRef       string            `json:"realization_evidence_ref"`
	MaterializationCommandRef    string            `json:"materialization_command_ref"`
	RealizationID                string            `json:"realization_id"`
	Materializer                 string            `json:"materializer"`
	Substrate                    string            `json:"substrate"`
	SourceRequiredObservations   []ObservationKind `json:"source_required_observations"`
	RuntimeRequiredObservations  []ObservationKind `json:"runtime_required_observations"`
	RuntimeObservationSetName    string            `json:"runtime_observation_set_name"`
	SourceProvenanceReady        bool              `json:"source_provenance_ready"`
	RuntimeEvidenceAccepted      bool              `json:"runtime_evidence_accepted"`
	RuntimeEquivalenceRequired   bool              `json:"runtime_equivalence_required"`
	StagingProofRequired         bool              `json:"staging_proof_required"`
	PromotionProofRequired       bool              `json:"promotion_proof_required"`
	PackagePublicationRequired   bool              `json:"package_publication_required"`
	NoVMLifecycleMutation        bool              `json:"no_vm_lifecycle_mutation"`
	NoProductionMutation         bool              `json:"no_production_mutation"`
	RuntimeBehaviorChanged       bool              `json:"runtime_behavior_changed"`
	DeployedRouteRegistered      bool              `json:"deployed_route_registered"`
	ProductionAuthTouched        bool              `json:"production_auth_touched"`
	StagingClaimed               bool              `json:"staging_claimed"`
	PromotionClaimed             bool              `json:"promotion_claimed"`
	VMLifecycleTouched           bool              `json:"vm_lifecycle_touched"`
	FirecrackerBootClaimed       bool              `json:"firecracker_boot_claimed"`
	RunAcceptanceRecordTouched   bool              `json:"run_acceptance_record_touched"`
	PackagePublicationClaimed    bool              `json:"package_publication_claimed"`
	FullSubstrateClaimed         bool              `json:"full_substrate_claimed"`
	CompletionClaimed            bool              `json:"completion_claimed"`
}

// BuildBaseRuntimeMaterializationCeremonyContract verifies that a scoped runtime
// Realization is bound to the source/provenance readiness contract before any
// downstream runtime-equivalence, staging, promotion, package-publication, or
// completion claim can be made.
func BuildBaseRuntimeMaterializationCeremonyContract(source BaseSourceProvenanceReadinessContract, realization Realization, evidence BaseRuntimeMaterializationCeremonyEvidence) (BaseRuntimeMaterializationCeremonyContract, error) {
	if err := validateBaseRuntimeMaterializationCeremonySource(source); err != nil {
		return BaseRuntimeMaterializationCeremonyContract{}, err
	}
	if err := validateBaseRuntimeMaterializationCeremonyRealization(realization, source); err != nil {
		return BaseRuntimeMaterializationCeremonyContract{}, err
	}
	if err := validateBaseRuntimeMaterializationCeremonyEvidence(evidence); err != nil {
		return BaseRuntimeMaterializationCeremonyContract{}, err
	}
	return BaseRuntimeMaterializationCeremonyContract{
		Kind:                         BaseRuntimeMaterializationCeremonyContractKind,
		Version:                      source.Version,
		Boundary:                     BaseRuntimeMaterializationCeremonyBoundary,
		Scope:                        BaseRuntimeMaterializationCeremonyScope,
		TypedArtifactProgramRef:      source.TypedArtifactProgramRef,
		SourceProvenanceReadinessRef: strings.TrimSpace(evidence.SourceProvenanceReadinessRef),
		RealizationEvidenceRef:       strings.TrimSpace(evidence.RealizationEvidenceRef),
		MaterializationCommandRef:    strings.TrimSpace(evidence.MaterializationCommandRef),
		RealizationID:                strings.TrimSpace(realization.ID),
		Materializer:                 strings.TrimSpace(realization.Capabilities.Materializer),
		Substrate:                    strings.TrimSpace(realization.Capabilities.Substrate),
		SourceRequiredObservations:   canonicalObservationKinds(source.RequiredObservations),
		RuntimeRequiredObservations:  canonicalObservationKinds(realization.Observations.RequiredKinds()),
		RuntimeObservationSetName:    strings.TrimSpace(realization.Observations.Name),
		SourceProvenanceReady:        true,
		RuntimeEvidenceAccepted:      true,
		RuntimeEquivalenceRequired:   true,
		StagingProofRequired:         true,
		PromotionProofRequired:       true,
		PackagePublicationRequired:   true,
		NoVMLifecycleMutation:        true,
		NoProductionMutation:         true,
		RuntimeBehaviorChanged:       false,
		DeployedRouteRegistered:      false,
		ProductionAuthTouched:        false,
		StagingClaimed:               false,
		PromotionClaimed:             false,
		VMLifecycleTouched:           false,
		FirecrackerBootClaimed:       false,
		RunAcceptanceRecordTouched:   false,
		PackagePublicationClaimed:    false,
		FullSubstrateClaimed:         false,
		CompletionClaimed:            false,
	}, nil
}

func validateBaseRuntimeMaterializationCeremonySource(source BaseSourceProvenanceReadinessContract) error {
	if source.Kind != BaseSourceProvenanceReadinessContractKind {
		return fmt.Errorf("base runtime materialization ceremony: source contract kind is %q", source.Kind)
	}
	if source.Boundary != BaseSourceProvenanceReadinessBoundary {
		return fmt.Errorf("base runtime materialization ceremony: source contract boundary is %q", source.Boundary)
	}
	if source.Scope != BaseSourceProvenanceReadinessScope {
		return fmt.Errorf("base runtime materialization ceremony: source contract scope is %q", source.Scope)
	}
	if !source.Version.Valid() {
		return fmt.Errorf("base runtime materialization ceremony: source contract version is invalid")
	}
	if ArtifactProgramRef(strings.TrimSpace(source.TypedArtifactProgramRef)) != source.Version.ArtifactProgramRef {
		return fmt.Errorf("base runtime materialization ceremony: source typed artifact program ref does not match version")
	}
	if !source.SourceProvenanceReady || !source.RuntimeCeremonyMayOpen || !source.LocalFileBlobProofSummarized {
		return fmt.Errorf("base runtime materialization ceremony: source contract does not open runtime ceremony")
	}
	if !baseSubstrateEquivalenceHasRequiredScope(source.RequiredObservations) {
		return fmt.Errorf("base runtime materialization ceremony: source contract must preserve file_manifest and blob_set observations")
	}
	if !source.RuntimeProofRequired || !source.StagingProofRequired || !source.PromotionProofRequired || !source.PackagePublicationRequired {
		return fmt.Errorf("base runtime materialization ceremony: source contract must preserve downstream proof requirements")
	}
	if !source.NoRuntimeMaterialization || !source.NoOpaqueDataImageDependency || !source.NoMutation {
		return fmt.Errorf("base runtime materialization ceremony: source contract has unsafe proof flags")
	}
	if source.RuntimeBehaviorChanged || source.DeployedRouteRegistered || source.ProductionAuthTouched || source.StagingClaimed || source.PromotionClaimed || source.VMLifecycleTouched || source.FirecrackerBootClaimed || source.RunAcceptanceRecordTouched || source.PackagePublicationClaimed || source.FullSubstrateClaimed || source.CompletionClaimed {
		return fmt.Errorf("base runtime materialization ceremony: source contract carries protected-surface claims")
	}
	return nil
}

func validateBaseRuntimeMaterializationCeremonyRealization(realization Realization, source BaseSourceProvenanceReadinessContract) error {
	if strings.TrimSpace(realization.ID) == "" {
		return fmt.Errorf("base runtime materialization ceremony: realization id is required")
	}
	if realization.Version != source.Version {
		return fmt.Errorf("base runtime materialization ceremony: realization version does not match source readiness")
	}
	if !realization.Version.Valid() {
		return fmt.Errorf("base runtime materialization ceremony: realization version is invalid")
	}
	if strings.TrimSpace(realization.Capabilities.Materializer) == "" {
		return fmt.Errorf("base runtime materialization ceremony: realization materializer is required")
	}
	if strings.TrimSpace(realization.Capabilities.Substrate) == "" {
		return fmt.Errorf("base runtime materialization ceremony: realization substrate is required")
	}
	if !realization.Capabilities.Supports(ObservationVMStateManifest) {
		return fmt.Errorf("base runtime materialization ceremony: realization must support vm_state_manifest")
	}
	for _, unsupported := range realization.Capabilities.Unsupported {
		if unsupported.Kind == ObservationVMStateManifest {
			return fmt.Errorf("base runtime materialization ceremony: realization cannot mark vm_state_manifest unsupported")
		}
	}
	if realization.Observations.Version != source.Version {
		return fmt.Errorf("base runtime materialization ceremony: realization observations version does not match source readiness")
	}
	if strings.TrimSpace(realization.Observations.Name) == "" {
		return fmt.Errorf("base runtime materialization ceremony: realization observation set name is required")
	}
	if !observationKindsContain(realization.Observations.RequiredKinds(), ObservationVMStateManifest) {
		return fmt.Errorf("base runtime materialization ceremony: realization must carry vm_state_manifest observations")
	}
	if observationKindsContain(realization.Observations.RequiredKinds(), ObservationFileManifest) || observationKindsContain(realization.Observations.RequiredKinds(), ObservationBlobSet) {
		return fmt.Errorf("base runtime materialization ceremony: realization cannot convert durable file/blob observations into runtime proof")
	}
	if missing := realization.Capabilities.MissingRequired(realization.Observations.RequiredKinds()); len(missing) > 0 {
		return fmt.Errorf("base runtime materialization ceremony: realization capability %q is missing", missing[0].Kind)
	}
	if len(realization.Observations.Observations) == 0 {
		return fmt.Errorf("base runtime materialization ceremony: realization observations are required")
	}
	for _, observation := range realization.Observations.Observations {
		if !observation.Valid() {
			return fmt.Errorf("base runtime materialization ceremony: realization observation is invalid")
		}
		if observation.Kind != ObservationVMStateManifest {
			return fmt.Errorf("base runtime materialization ceremony: realization observation kind %q is outside runtime ceremony scope", observation.Kind)
		}
	}
	return nil
}

func validateBaseRuntimeMaterializationCeremonyEvidence(evidence BaseRuntimeMaterializationCeremonyEvidence) error {
	if strings.TrimSpace(evidence.SourceProvenanceReadinessRef) == "" {
		return fmt.Errorf("base runtime materialization ceremony: source provenance readiness ref is required")
	}
	if strings.TrimSpace(evidence.RealizationEvidenceRef) == "" {
		return fmt.Errorf("base runtime materialization ceremony: realization evidence ref is required")
	}
	if strings.TrimSpace(evidence.MaterializationCommandRef) == "" {
		return fmt.Errorf("base runtime materialization ceremony: materialization command ref is required")
	}
	if !evidence.NoVMLifecycleMutation {
		return fmt.Errorf("base runtime materialization ceremony: evidence must prove no VM lifecycle mutation")
	}
	if !evidence.NoProductionMutation {
		return fmt.Errorf("base runtime materialization ceremony: evidence must prove no production mutation")
	}
	if evidence.RuntimeBehaviorChanged || evidence.DeployedRouteRegistered || evidence.ProductionAuthTouched || evidence.StagingClaimed || evidence.PromotionClaimed || evidence.VMLifecycleTouched || evidence.FirecrackerBootClaimed || evidence.RunAcceptanceRecordTouched || evidence.PackagePublicationClaimed || evidence.FullSubstrateClaimed || evidence.CompletionClaimed {
		return fmt.Errorf("base runtime materialization ceremony: evidence carries protected-surface or completion claims")
	}
	return nil
}

func observationKindsContain(kinds []ObservationKind, want ObservationKind) bool {
	for _, kind := range kinds {
		if kind == want {
			return true
		}
	}
	return false
}
