package computerversion

import (
	"fmt"
	"strings"
)

const BaseSourceProvenanceReadinessContractKind = "base_source_provenance_readiness_contract"

const BaseSourceProvenanceReadinessBoundary = "base_source_provenance_readiness_without_runtime_or_completion_claim"

const BaseSourceProvenanceReadinessScope = "base_file_blob_source_provenance_runtime_ceremony_readiness"

// BaseSourceProvenanceReadinessEvidence records proof refs for connecting the
// local substrate proof summary to the typed artifact-program/provenance slice.
// It does not open or satisfy runtime materialization, staging, promotion,
// package-publication, or completion authority.
type BaseSourceProvenanceReadinessEvidence struct {
	LocalProofSummaryRef        string `json:"local_proof_summary_ref"`
	DurableStateSliceRef        string `json:"durable_state_slice_ref"`
	TypedArtifactProgramRef     string `json:"typed_artifact_program_ref"`
	SourceProvenanceEvidenceRef string `json:"source_provenance_evidence_ref"`
	NoRuntimeMaterialization    bool   `json:"no_runtime_materialization"`
	NoOpaqueDataImageDependency bool   `json:"no_opaque_data_img_dependency"`
	RuntimeBehaviorChanged      bool   `json:"runtime_behavior_changed"`
	DeployedRouteRegistered     bool   `json:"deployed_route_registered"`
	ProductionAuthTouched       bool   `json:"production_auth_touched"`
	StagingClaimed              bool   `json:"staging_claimed"`
	PromotionClaimed            bool   `json:"promotion_claimed"`
	VMLifecycleTouched          bool   `json:"vm_lifecycle_touched"`
	FirecrackerBootClaimed      bool   `json:"firecracker_boot_claimed"`
	RunAcceptanceRecordTouched  bool   `json:"run_acceptance_record_touched"`
	PackagePublicationClaimed   bool   `json:"package_publication_claimed"`
	FullSubstrateClaimed        bool   `json:"full_substrate_claimed"`
	CompletionClaimed           bool   `json:"completion_claimed"`
	NoMutation                  bool   `json:"no_mutation"`
}

// BaseSourceProvenanceReadinessContract binds local substrate proof to typed
// durable-state provenance for the same ComputerVersion. It is a readiness gate
// for deciding whether a red runtime-materialization ceremony can be opened; it
// is not runtime, staging, promotion, publication, or completion evidence.
type BaseSourceProvenanceReadinessContract struct {
	Kind                         string                  `json:"kind"`
	Version                      ComputerVersion         `json:"version"`
	Boundary                     string                  `json:"boundary"`
	Scope                        string                  `json:"scope"`
	ClaimScope                   string                  `json:"claim_scope"`
	TypedArtifactProgramRef      string                  `json:"typed_artifact_program_ref"`
	PersistentStateClasses       []BaseDurableStateClass `json:"persistent_state_classes"`
	RequiredObservations         []ObservationKind       `json:"required_observations"`
	RequiredSemantics            []UserSemantic          `json:"required_semantics"`
	LocalProofSummaryRef         string                  `json:"local_proof_summary_ref"`
	DurableStateSliceRef         string                  `json:"durable_state_slice_ref"`
	SourceProvenanceEvidenceRef  string                  `json:"source_provenance_evidence_ref"`
	LocalFileBlobProofSummarized bool                    `json:"local_file_blob_proof_summarized"`
	SourceProvenanceReady        bool                    `json:"source_provenance_ready"`
	RuntimeCeremonyMayOpen       bool                    `json:"runtime_ceremony_may_open"`
	RuntimeProofRequired         bool                    `json:"runtime_proof_required"`
	StagingProofRequired         bool                    `json:"staging_proof_required"`
	PromotionProofRequired       bool                    `json:"promotion_proof_required"`
	PackagePublicationRequired   bool                    `json:"package_publication_required"`
	NoRuntimeMaterialization     bool                    `json:"no_runtime_materialization"`
	NoOpaqueDataImageDependency  bool                    `json:"no_opaque_data_img_dependency"`
	RuntimeBehaviorChanged       bool                    `json:"runtime_behavior_changed"`
	DeployedRouteRegistered      bool                    `json:"deployed_route_registered"`
	ProductionAuthTouched        bool                    `json:"production_auth_touched"`
	StagingClaimed               bool                    `json:"staging_claimed"`
	PromotionClaimed             bool                    `json:"promotion_claimed"`
	VMLifecycleTouched           bool                    `json:"vm_lifecycle_touched"`
	FirecrackerBootClaimed       bool                    `json:"firecracker_boot_claimed"`
	RunAcceptanceRecordTouched   bool                    `json:"run_acceptance_record_touched"`
	PackagePublicationClaimed    bool                    `json:"package_publication_claimed"`
	FullSubstrateClaimed         bool                    `json:"full_substrate_claimed"`
	CompletionClaimed            bool                    `json:"completion_claimed"`
	NoMutation                   bool                    `json:"no_mutation"`
}

// BuildBaseSourceProvenanceReadinessContract verifies that the local substrate
// proof summary is backed by the typed durable-state/provenance slice for the
// same ComputerVersion before runtime materialization work is considered.
func BuildBaseSourceProvenanceReadinessContract(summary BaseLocalSubstrateProofSummaryContract, durable BaseDurableStateSliceContract, evidence BaseSourceProvenanceReadinessEvidence) (BaseSourceProvenanceReadinessContract, error) {
	if err := validateBaseSourceProvenanceReadinessSummary(summary); err != nil {
		return BaseSourceProvenanceReadinessContract{}, err
	}
	if err := validateBaseSourceProvenanceReadinessDurable(durable); err != nil {
		return BaseSourceProvenanceReadinessContract{}, err
	}
	if summary.Version != durable.Version {
		return BaseSourceProvenanceReadinessContract{}, fmt.Errorf("base source provenance readiness: summary and durable contracts name different computer versions")
	}
	if summary.ClaimScope != BaseSubstrateEquivalenceClaimScope {
		return BaseSourceProvenanceReadinessContract{}, fmt.Errorf("base source provenance readiness: summary claim scope is %q", summary.ClaimScope)
	}
	if ArtifactProgramRef(strings.TrimSpace(evidence.TypedArtifactProgramRef)) != durable.Version.ArtifactProgramRef {
		return BaseSourceProvenanceReadinessContract{}, fmt.Errorf("base source provenance readiness: typed artifact program ref does not match durable contract version")
	}
	if strings.TrimSpace(evidence.TypedArtifactProgramRef) != durable.TypedArtifactProgramRef {
		return BaseSourceProvenanceReadinessContract{}, fmt.Errorf("base source provenance readiness: typed artifact program ref does not match durable contract ref")
	}
	if err := validateBaseSourceProvenanceReadinessEvidence(evidence); err != nil {
		return BaseSourceProvenanceReadinessContract{}, err
	}
	return BaseSourceProvenanceReadinessContract{
		Kind:                         BaseSourceProvenanceReadinessContractKind,
		Version:                      summary.Version,
		Boundary:                     BaseSourceProvenanceReadinessBoundary,
		Scope:                        BaseSourceProvenanceReadinessScope,
		ClaimScope:                   summary.ClaimScope,
		TypedArtifactProgramRef:      strings.TrimSpace(evidence.TypedArtifactProgramRef),
		PersistentStateClasses:       []BaseDurableStateClass{BaseDurableStateClassBlobContent, BaseDurableStateClassFileManifest},
		RequiredObservations:         []ObservationKind{ObservationBlobSet, ObservationFileManifest},
		RequiredSemantics:            []UserSemantic{UserSemanticDeletionState, UserSemanticFileContent, UserSemanticFilePath, UserSemanticFileProvenance},
		LocalProofSummaryRef:         strings.TrimSpace(evidence.LocalProofSummaryRef),
		DurableStateSliceRef:         strings.TrimSpace(evidence.DurableStateSliceRef),
		SourceProvenanceEvidenceRef:  strings.TrimSpace(evidence.SourceProvenanceEvidenceRef),
		LocalFileBlobProofSummarized: true,
		SourceProvenanceReady:        true,
		RuntimeCeremonyMayOpen:       true,
		RuntimeProofRequired:         true,
		StagingProofRequired:         true,
		PromotionProofRequired:       true,
		PackagePublicationRequired:   true,
		NoRuntimeMaterialization:     true,
		NoOpaqueDataImageDependency:  true,
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
		NoMutation:                   true,
	}, nil
}

func validateBaseSourceProvenanceReadinessSummary(summary BaseLocalSubstrateProofSummaryContract) error {
	if summary.Kind != BaseLocalSubstrateProofSummaryContractKind {
		return fmt.Errorf("base source provenance readiness: summary contract kind is %q", summary.Kind)
	}
	if summary.Boundary != BaseLocalSubstrateProofSummaryBoundary {
		return fmt.Errorf("base source provenance readiness: summary contract boundary is %q", summary.Boundary)
	}
	if summary.Scope != BaseLocalSubstrateProofSummaryScope {
		return fmt.Errorf("base source provenance readiness: summary contract scope is %q", summary.Scope)
	}
	if !summary.LocalFileBlobProofSummarized || !summary.ReentryAllowed {
		return fmt.Errorf("base source provenance readiness: summary contract does not summarize local file/blob proof")
	}
	if !summary.RuntimeSubstrateProofRequired || !summary.StagingProofRequired || !summary.PromotionProofRequired || !baseLocalSubstrateProofSummaryHasRequiredGaps(summary.RemainingGaps) {
		return fmt.Errorf("base source provenance readiness: summary contract must preserve runtime, staging, and promotion gaps")
	}
	if !baseSubstrateEquivalenceHasRequiredScope(summary.RequiredObservations) {
		return fmt.Errorf("base source provenance readiness: summary contract must include file_manifest and blob_set")
	}
	if !summary.NoRuntimeMaterialization || !summary.NoOpaqueDataImageDependency || !summary.NoMutation {
		return fmt.Errorf("base source provenance readiness: summary contract has unsafe proof flags")
	}
	if summary.RuntimeBehaviorChanged || summary.DeployedRouteRegistered || summary.ProductionAuthTouched || summary.StagingClaimed || summary.PromotionClaimed || summary.VMLifecycleTouched || summary.FirecrackerBootClaimed || summary.RunAcceptanceRecordTouched || summary.PackagePublicationClaimed || summary.FullSubstrateIndependenceClaim || summary.CompletionClaimed {
		return fmt.Errorf("base source provenance readiness: summary contract carries protected-surface claims")
	}
	return nil
}

func validateBaseSourceProvenanceReadinessDurable(durable BaseDurableStateSliceContract) error {
	if durable.Kind != BaseDurableStateSliceContractKind {
		return fmt.Errorf("base source provenance readiness: durable contract kind is %q", durable.Kind)
	}
	if durable.Boundary != BaseDurableStateSliceBoundary {
		return fmt.Errorf("base source provenance readiness: durable contract boundary is %q", durable.Boundary)
	}
	if durable.Scope != BaseDurableStateSliceScope {
		return fmt.Errorf("base source provenance readiness: durable contract scope is %q", durable.Scope)
	}
	if !baseSubstrateEquivalenceHasRequiredScope(durable.RequiredObservations) {
		return fmt.Errorf("base source provenance readiness: durable contract must include file_manifest and blob_set")
	}
	if !baseDurableStateSliceHasRequiredClasses(durable.PersistentStateClasses) {
		return fmt.Errorf("base source provenance readiness: durable contract must cover file manifest and blob content classes")
	}
	if !baseDurableStateSliceHasRequiredSemantics(durable.RequiredSemantics) {
		return fmt.Errorf("base source provenance readiness: durable contract must cover file path, content, deletion, and provenance semantics")
	}
	if strings.TrimSpace(durable.TypedArtifactProgramRef) == "" || ArtifactProgramRef(strings.TrimSpace(durable.TypedArtifactProgramRef)) != durable.Version.ArtifactProgramRef {
		return fmt.Errorf("base source provenance readiness: durable typed artifact program ref is invalid")
	}
	if strings.TrimSpace(durable.EquivalenceContractRef) == "" || strings.TrimSpace(durable.UserIsomorphismContractRef) == "" || strings.TrimSpace(durable.DurableSliceEvidenceRef) == "" {
		return fmt.Errorf("base source provenance readiness: durable contract must carry proof refs")
	}
	if !durable.NoOpaqueDataImageDependency || !durable.NoMutation {
		return fmt.Errorf("base source provenance readiness: durable contract has unsafe proof flags")
	}
	if durable.RuntimeBehaviorChanged || durable.DeployedRouteRegistered || durable.ProductionAuthTouched || durable.StagingClaimed || durable.PromotionClaimed || durable.VMLifecycleTouched || durable.RunAcceptanceRecordTouched || durable.FullComputerClaimed || durable.DataImageDisposableClaimed {
		return fmt.Errorf("base source provenance readiness: durable contract carries protected-surface claims")
	}
	return nil
}

func validateBaseSourceProvenanceReadinessEvidence(evidence BaseSourceProvenanceReadinessEvidence) error {
	if strings.TrimSpace(evidence.LocalProofSummaryRef) == "" {
		return fmt.Errorf("base source provenance readiness: local proof summary ref is required")
	}
	if strings.TrimSpace(evidence.DurableStateSliceRef) == "" {
		return fmt.Errorf("base source provenance readiness: durable state slice ref is required")
	}
	if strings.TrimSpace(evidence.SourceProvenanceEvidenceRef) == "" {
		return fmt.Errorf("base source provenance readiness: source provenance evidence ref is required")
	}
	if !evidence.NoRuntimeMaterialization {
		return fmt.Errorf("base source provenance readiness: evidence must prove no runtime materialization")
	}
	if !evidence.NoOpaqueDataImageDependency {
		return fmt.Errorf("base source provenance readiness: evidence must prove no opaque data.img dependency")
	}
	if evidence.RuntimeBehaviorChanged || evidence.DeployedRouteRegistered || evidence.ProductionAuthTouched || evidence.StagingClaimed || evidence.PromotionClaimed || evidence.VMLifecycleTouched || evidence.FirecrackerBootClaimed || evidence.RunAcceptanceRecordTouched || evidence.PackagePublicationClaimed || evidence.FullSubstrateClaimed || evidence.CompletionClaimed {
		return fmt.Errorf("base source provenance readiness: evidence carries protected-surface or completion claims")
	}
	if !evidence.NoMutation {
		return fmt.Errorf("base source provenance readiness: evidence must be no-mutation")
	}
	return nil
}

func baseDurableStateSliceHasRequiredClasses(classes []BaseDurableStateClass) bool {
	seen := make(map[BaseDurableStateClass]struct{}, len(classes))
	for _, class := range classes {
		seen[class] = struct{}{}
	}
	_, hasManifest := seen[BaseDurableStateClassFileManifest]
	_, hasBlob := seen[BaseDurableStateClassBlobContent]
	return hasManifest && hasBlob
}
