package computerversion

import (
	"fmt"
	"strings"
)

const BaseRuntimeEquivalenceBoundaryContractKind = "base_runtime_equivalence_boundary_contract"

const BaseRuntimeEquivalenceBoundary = "base_runtime_equivalence_narrowed_without_durable_state_or_downstream_claim"

const BaseRuntimeEquivalenceScope = "vmmanager_runtime_evidence_narrowed_against_source_provenance_file_blob_scope"

// BaseRuntimeEquivalenceBoundaryEvidence records proof refs for the red boundary
// that checks whether accepted runtime materialization evidence can support the
// durable file/blob source-provenance equivalence claim. Current vmmanager-only
// evidence must narrow rather than pass that claim.
type BaseRuntimeEquivalenceBoundaryEvidence struct {
	RuntimeMaterializationCeremonyRef string `json:"runtime_materialization_ceremony_ref"`
	RuntimeEquivalenceEvidenceRef     string `json:"runtime_equivalence_evidence_ref"`
	NoVMLifecycleMutation             bool   `json:"no_vm_lifecycle_mutation"`
	NoProductionMutation              bool   `json:"no_production_mutation"`
	RuntimeBehaviorChanged            bool   `json:"runtime_behavior_changed"`
	DeployedRouteRegistered           bool   `json:"deployed_route_registered"`
	ProductionAuthTouched             bool   `json:"production_auth_touched"`
	StagingClaimed                    bool   `json:"staging_claimed"`
	PromotionClaimed                  bool   `json:"promotion_claimed"`
	VMLifecycleTouched                bool   `json:"vm_lifecycle_touched"`
	FirecrackerBootClaimed            bool   `json:"firecracker_boot_claimed"`
	RunAcceptanceRecordTouched        bool   `json:"run_acceptance_record_touched"`
	PackagePublicationClaimed         bool   `json:"package_publication_claimed"`
	FullSubstrateClaimed              bool   `json:"full_substrate_claimed"`
	CompletionClaimed                 bool   `json:"completion_claimed"`
}

// BaseRuntimeEquivalenceBoundaryContract records that runtime materialization
// evidence has reached the equivalence boundary but the durable file/blob claim
// is narrowed. It prevents vmmanager metadata or opaque data.img presence from
// standing in for typed durable-state observations.
type BaseRuntimeEquivalenceBoundaryContract struct {
	Kind                              string                  `json:"kind"`
	Version                           ComputerVersion         `json:"version"`
	Boundary                          string                  `json:"boundary"`
	Scope                             string                  `json:"scope"`
	TypedArtifactProgramRef           string                  `json:"typed_artifact_program_ref"`
	RuntimeMaterializationCeremonyRef string                  `json:"runtime_materialization_ceremony_ref"`
	RuntimeEquivalenceEvidenceRef     string                  `json:"runtime_equivalence_evidence_ref"`
	SourceProvenanceReadinessRef      string                  `json:"source_provenance_readiness_ref"`
	RealizationEvidenceRef            string                  `json:"realization_evidence_ref"`
	Materializer                      string                  `json:"materializer"`
	Substrate                         string                  `json:"substrate"`
	SourceRequiredObservations        []ObservationKind       `json:"source_required_observations"`
	RuntimeRequiredObservations       []ObservationKind       `json:"runtime_required_observations"`
	RuntimeEquivalenceStatus          EquivalenceStatus       `json:"runtime_equivalence_status"`
	UnsupportedDurableObservations    []UnsupportedCapability `json:"unsupported_durable_observations"`
	RuntimeEquivalenceNarrowed        bool                    `json:"runtime_equivalence_narrowed"`
	RuntimeEquivalenceClaimed         bool                    `json:"runtime_equivalence_claimed"`
	DurableStateEquivalenceRequired   bool                    `json:"durable_state_equivalence_required"`
	StagingProofRequired              bool                    `json:"staging_proof_required"`
	PromotionProofRequired            bool                    `json:"promotion_proof_required"`
	PackagePublicationRequired        bool                    `json:"package_publication_required"`
	NoVMLifecycleMutation             bool                    `json:"no_vm_lifecycle_mutation"`
	NoProductionMutation              bool                    `json:"no_production_mutation"`
	RuntimeBehaviorChanged            bool                    `json:"runtime_behavior_changed"`
	DeployedRouteRegistered           bool                    `json:"deployed_route_registered"`
	ProductionAuthTouched             bool                    `json:"production_auth_touched"`
	StagingClaimed                    bool                    `json:"staging_claimed"`
	PromotionClaimed                  bool                    `json:"promotion_claimed"`
	VMLifecycleTouched                bool                    `json:"vm_lifecycle_touched"`
	FirecrackerBootClaimed            bool                    `json:"firecracker_boot_claimed"`
	RunAcceptanceRecordTouched        bool                    `json:"run_acceptance_record_touched"`
	PackagePublicationClaimed         bool                    `json:"package_publication_claimed"`
	FullSubstrateClaimed              bool                    `json:"full_substrate_claimed"`
	CompletionClaimed                 bool                    `json:"completion_claimed"`
}

// BuildBaseRuntimeEquivalenceBoundaryContract verifies that the first runtime
// equivalence attempt is narrowed by missing durable file/blob support. A passing
// equivalence result is rejected here because current vmmanager-scoped runtime
// evidence has not observed typed durable state.
func BuildBaseRuntimeEquivalenceBoundaryContract(source BaseSourceProvenanceReadinessContract, ceremony BaseRuntimeMaterializationCeremonyContract, result EquivalenceResult, evidence BaseRuntimeEquivalenceBoundaryEvidence) (BaseRuntimeEquivalenceBoundaryContract, error) {
	if err := validateBaseRuntimeMaterializationCeremonySource(source); err != nil {
		return BaseRuntimeEquivalenceBoundaryContract{}, fmt.Errorf("base runtime equivalence boundary: invalid source readiness: %w", err)
	}
	if err := validateBaseRuntimeEquivalenceCeremony(source, ceremony); err != nil {
		return BaseRuntimeEquivalenceBoundaryContract{}, err
	}
	if err := validateBaseRuntimeEquivalenceResult(result); err != nil {
		return BaseRuntimeEquivalenceBoundaryContract{}, err
	}
	if err := validateBaseRuntimeEquivalenceEvidence(evidence); err != nil {
		return BaseRuntimeEquivalenceBoundaryContract{}, err
	}
	return BaseRuntimeEquivalenceBoundaryContract{
		Kind:                              BaseRuntimeEquivalenceBoundaryContractKind,
		Version:                           source.Version,
		Boundary:                          BaseRuntimeEquivalenceBoundary,
		Scope:                             BaseRuntimeEquivalenceScope,
		TypedArtifactProgramRef:           source.TypedArtifactProgramRef,
		RuntimeMaterializationCeremonyRef: strings.TrimSpace(evidence.RuntimeMaterializationCeremonyRef),
		RuntimeEquivalenceEvidenceRef:     strings.TrimSpace(evidence.RuntimeEquivalenceEvidenceRef),
		SourceProvenanceReadinessRef:      ceremony.SourceProvenanceReadinessRef,
		RealizationEvidenceRef:            ceremony.RealizationEvidenceRef,
		Materializer:                      ceremony.Materializer,
		Substrate:                         ceremony.Substrate,
		SourceRequiredObservations:        canonicalObservationKinds(source.RequiredObservations),
		RuntimeRequiredObservations:       canonicalObservationKinds(ceremony.RuntimeRequiredObservations),
		RuntimeEquivalenceStatus:          EquivalenceNarrowed,
		UnsupportedDurableObservations:    canonicalUnsupportedCapabilities(result.Unsupported),
		RuntimeEquivalenceNarrowed:        true,
		RuntimeEquivalenceClaimed:         false,
		DurableStateEquivalenceRequired:   true,
		StagingProofRequired:              true,
		PromotionProofRequired:            true,
		PackagePublicationRequired:        true,
		NoVMLifecycleMutation:             true,
		NoProductionMutation:              true,
		RuntimeBehaviorChanged:            false,
		DeployedRouteRegistered:           false,
		ProductionAuthTouched:             false,
		StagingClaimed:                    false,
		PromotionClaimed:                  false,
		VMLifecycleTouched:                false,
		FirecrackerBootClaimed:            false,
		RunAcceptanceRecordTouched:        false,
		PackagePublicationClaimed:         false,
		FullSubstrateClaimed:              false,
		CompletionClaimed:                 false,
	}, nil
}

func validateBaseRuntimeEquivalenceCeremony(source BaseSourceProvenanceReadinessContract, ceremony BaseRuntimeMaterializationCeremonyContract) error {
	if ceremony.Kind != BaseRuntimeMaterializationCeremonyContractKind {
		return fmt.Errorf("base runtime equivalence boundary: ceremony contract kind is %q", ceremony.Kind)
	}
	if ceremony.Boundary != BaseRuntimeMaterializationCeremonyBoundary {
		return fmt.Errorf("base runtime equivalence boundary: ceremony contract boundary is %q", ceremony.Boundary)
	}
	if ceremony.Scope != BaseRuntimeMaterializationCeremonyScope {
		return fmt.Errorf("base runtime equivalence boundary: ceremony contract scope is %q", ceremony.Scope)
	}
	if ceremony.Version != source.Version {
		return fmt.Errorf("base runtime equivalence boundary: ceremony version does not match source readiness")
	}
	if ArtifactProgramRef(strings.TrimSpace(ceremony.TypedArtifactProgramRef)) != source.Version.ArtifactProgramRef {
		return fmt.Errorf("base runtime equivalence boundary: ceremony typed artifact program ref does not match source readiness")
	}
	if strings.TrimSpace(ceremony.SourceProvenanceReadinessRef) == "" || strings.TrimSpace(ceremony.RealizationEvidenceRef) == "" {
		return fmt.Errorf("base runtime equivalence boundary: ceremony proof refs are required")
	}
	if strings.TrimSpace(ceremony.Materializer) == "" || strings.TrimSpace(ceremony.Substrate) == "" {
		return fmt.Errorf("base runtime equivalence boundary: ceremony materializer and substrate are required")
	}
	if !baseSubstrateEquivalenceHasRequiredScope(ceremony.SourceRequiredObservations) {
		return fmt.Errorf("base runtime equivalence boundary: ceremony must preserve source file_manifest and blob_set requirements")
	}
	if !observationKindsContain(ceremony.RuntimeRequiredObservations, ObservationVMStateManifest) {
		return fmt.Errorf("base runtime equivalence boundary: ceremony must preserve runtime vm_state_manifest requirement")
	}
	if !ceremony.SourceProvenanceReady || !ceremony.RuntimeEvidenceAccepted || !ceremony.RuntimeEquivalenceRequired {
		return fmt.Errorf("base runtime equivalence boundary: ceremony does not carry accepted runtime evidence")
	}
	if !ceremony.StagingProofRequired || !ceremony.PromotionProofRequired || !ceremony.PackagePublicationRequired {
		return fmt.Errorf("base runtime equivalence boundary: ceremony must preserve downstream proof requirements")
	}
	if !ceremony.NoVMLifecycleMutation || !ceremony.NoProductionMutation {
		return fmt.Errorf("base runtime equivalence boundary: ceremony has unsafe mutation flags")
	}
	if ceremony.RuntimeBehaviorChanged || ceremony.DeployedRouteRegistered || ceremony.ProductionAuthTouched || ceremony.StagingClaimed || ceremony.PromotionClaimed || ceremony.VMLifecycleTouched || ceremony.FirecrackerBootClaimed || ceremony.RunAcceptanceRecordTouched || ceremony.PackagePublicationClaimed || ceremony.FullSubstrateClaimed || ceremony.CompletionClaimed {
		return fmt.Errorf("base runtime equivalence boundary: ceremony carries protected-surface claims")
	}
	return nil
}

func validateBaseRuntimeEquivalenceResult(result EquivalenceResult) error {
	if result.Status != EquivalenceNarrowed {
		return fmt.Errorf("base runtime equivalence boundary: runtime equivalence status is %q", result.Status)
	}
	if len(result.Differences) > 0 {
		return fmt.Errorf("base runtime equivalence boundary: narrowed result cannot carry concrete differences")
	}
	if !unsupportedCapabilityContains(result.Unsupported, ObservationFileManifest) || !unsupportedCapabilityContains(result.Unsupported, ObservationBlobSet) {
		return fmt.Errorf("base runtime equivalence boundary: narrowed result must name unsupported file_manifest and blob_set observations")
	}
	return nil
}

func validateBaseRuntimeEquivalenceEvidence(evidence BaseRuntimeEquivalenceBoundaryEvidence) error {
	if strings.TrimSpace(evidence.RuntimeMaterializationCeremonyRef) == "" {
		return fmt.Errorf("base runtime equivalence boundary: runtime materialization ceremony ref is required")
	}
	if strings.TrimSpace(evidence.RuntimeEquivalenceEvidenceRef) == "" {
		return fmt.Errorf("base runtime equivalence boundary: runtime equivalence evidence ref is required")
	}
	if !evidence.NoVMLifecycleMutation {
		return fmt.Errorf("base runtime equivalence boundary: evidence must prove no VM lifecycle mutation")
	}
	if !evidence.NoProductionMutation {
		return fmt.Errorf("base runtime equivalence boundary: evidence must prove no production mutation")
	}
	if evidence.RuntimeBehaviorChanged || evidence.DeployedRouteRegistered || evidence.ProductionAuthTouched || evidence.StagingClaimed || evidence.PromotionClaimed || evidence.VMLifecycleTouched || evidence.FirecrackerBootClaimed || evidence.RunAcceptanceRecordTouched || evidence.PackagePublicationClaimed || evidence.FullSubstrateClaimed || evidence.CompletionClaimed {
		return fmt.Errorf("base runtime equivalence boundary: evidence carries protected-surface or completion claims")
	}
	return nil
}

func unsupportedCapabilityContains(capabilities []UnsupportedCapability, want ObservationKind) bool {
	for _, capability := range capabilities {
		if capability.Kind == want {
			return true
		}
	}
	return false
}

func canonicalUnsupportedCapabilities(capabilities []UnsupportedCapability) []UnsupportedCapability {
	seen := make(map[ObservationKind]struct{}, len(capabilities))
	out := make([]UnsupportedCapability, 0, len(capabilities))
	for _, capability := range capabilities {
		if !capability.Kind.Valid() {
			continue
		}
		if _, ok := seen[capability.Kind]; ok {
			continue
		}
		seen[capability.Kind] = struct{}{}
		if strings.TrimSpace(capability.Reason) == "" {
			capability.Reason = "capability not declared by materializer"
		}
		out = append(out, capability)
	}
	return out
}
