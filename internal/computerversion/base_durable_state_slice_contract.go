package computerversion

import (
	"fmt"
	"strings"
)

const BaseDurableStateSliceContractKind = "base_durable_state_slice_contract"

const BaseDurableStateSliceBoundary = "typed_base_file_blob_state_slice_without_data_img_or_runtime_mutation"

const BaseDurableStateSliceScope = "base_file_manifest_blob_typed_artifact_program_slice"

type BaseDurableStateClass string

const (
	BaseDurableStateClassFileManifest BaseDurableStateClass = "base_file_manifest"
	BaseDurableStateClassBlobContent  BaseDurableStateClass = "base_blob_content"
)

// BaseDurableStateSliceEvidence names the proof refs for the minimum typed
// persistent Base slice. It certifies only the file/blob slice represented by
// the Base artifact-program ref; it does not certify data.img disposability for
// the full computer.
type BaseDurableStateSliceEvidence struct {
	EquivalenceContractRef      string `json:"equivalence_contract_ref"`
	UserIsomorphismContractRef  string `json:"user_isomorphism_contract_ref"`
	TypedArtifactProgramRef     string `json:"typed_artifact_program_ref"`
	DurableSliceEvidenceRef     string `json:"durable_slice_evidence_ref"`
	NoOpaqueDataImageDependency bool   `json:"no_opaque_data_img_dependency"`
	RuntimeBehaviorChanged      bool   `json:"runtime_behavior_changed"`
	DeployedRouteRegistered     bool   `json:"deployed_route_registered"`
	ProductionAuthTouched       bool   `json:"production_auth_touched"`
	StagingClaimed              bool   `json:"staging_claimed"`
	PromotionClaimed            bool   `json:"promotion_claimed"`
	VMLifecycleTouched          bool   `json:"vm_lifecycle_touched"`
	RunAcceptanceRecordTouched  bool   `json:"run_acceptance_record_touched"`
	FullComputerClaimed         bool   `json:"full_computer_claimed"`
	DataImageDisposableClaimed  bool   `json:"data_img_disposable_claimed"`
	NoMutation                  bool   `json:"no_mutation"`
}

// BaseDurableStateSliceContract records that the Base file-manifest/blob-set
// slice is represented by typed artifact-program references and proven through
// scoped equivalence plus scoped user-isomorphism. It is not a full-computer
// substrate-independence or data.img-disposable certificate.
type BaseDurableStateSliceContract struct {
	Kind                        string                  `json:"kind"`
	Version                     ComputerVersion         `json:"version"`
	Boundary                    string                  `json:"boundary"`
	Scope                       string                  `json:"scope"`
	PersistentStateClasses      []BaseDurableStateClass `json:"persistent_state_classes"`
	RequiredObservations        []ObservationKind       `json:"required_observations"`
	RequiredSemantics           []UserSemantic          `json:"required_semantics"`
	EquivalenceContractKind     string                  `json:"equivalence_contract_kind"`
	EquivalenceContractRef      string                  `json:"equivalence_contract_ref"`
	UserIsomorphismContractKind string                  `json:"user_isomorphism_contract_kind"`
	UserIsomorphismContractRef  string                  `json:"user_isomorphism_contract_ref"`
	TypedArtifactProgramRef     string                  `json:"typed_artifact_program_ref"`
	DurableSliceEvidenceRef     string                  `json:"durable_slice_evidence_ref"`
	NoOpaqueDataImageDependency bool                    `json:"no_opaque_data_img_dependency"`
	FullComputerClaimed         bool                    `json:"full_computer_claimed"`
	DataImageDisposableClaimed  bool                    `json:"data_img_disposable_claimed"`
	RuntimeBehaviorChanged      bool                    `json:"runtime_behavior_changed"`
	DeployedRouteRegistered     bool                    `json:"deployed_route_registered"`
	ProductionAuthTouched       bool                    `json:"production_auth_touched"`
	StagingClaimed              bool                    `json:"staging_claimed"`
	PromotionClaimed            bool                    `json:"promotion_claimed"`
	VMLifecycleTouched          bool                    `json:"vm_lifecycle_touched"`
	RunAcceptanceRecordTouched  bool                    `json:"run_acceptance_record_touched"`
	NoMutation                  bool                    `json:"no_mutation"`
}

// BuildBaseDurableStateSliceContract verifies that the Base file/blob slice is
// represented as typed artifact-program evidence and backed by the scoped
// equivalence and user-isomorphism contracts. It intentionally keeps data.img
// disposability and full-computer claims false.
func BuildBaseDurableStateSliceContract(equivalence BaseSubstrateEquivalenceContract, user BaseCurrentStateUserIsomorphismContract, evidence BaseDurableStateSliceEvidence) (BaseDurableStateSliceContract, error) {
	if err := validateBaseDurableStateSliceEvidence(evidence); err != nil {
		return BaseDurableStateSliceContract{}, err
	}
	if err := validateBaseDurableStateSliceEquivalence(equivalence); err != nil {
		return BaseDurableStateSliceContract{}, err
	}
	if ArtifactProgramRef(strings.TrimSpace(evidence.TypedArtifactProgramRef)) != equivalence.Version.ArtifactProgramRef {
		return BaseDurableStateSliceContract{}, fmt.Errorf("base durable state slice: typed artifact program ref does not match equivalence version")
	}
	if err := validateBaseDurableStateSliceUserIsomorphism(equivalence, user); err != nil {
		return BaseDurableStateSliceContract{}, err
	}

	return BaseDurableStateSliceContract{
		Kind:                        BaseDurableStateSliceContractKind,
		Version:                     equivalence.Version,
		Boundary:                    BaseDurableStateSliceBoundary,
		Scope:                       BaseDurableStateSliceScope,
		PersistentStateClasses:      []BaseDurableStateClass{BaseDurableStateClassBlobContent, BaseDurableStateClassFileManifest},
		RequiredObservations:        canonicalObservationKinds(equivalence.RequiredObservations),
		RequiredSemantics:           canonicalUserSemantics(user.RequiredSemantics),
		EquivalenceContractKind:     equivalence.Kind,
		EquivalenceContractRef:      strings.TrimSpace(evidence.EquivalenceContractRef),
		UserIsomorphismContractKind: user.Kind,
		UserIsomorphismContractRef:  strings.TrimSpace(evidence.UserIsomorphismContractRef),
		TypedArtifactProgramRef:     strings.TrimSpace(evidence.TypedArtifactProgramRef),
		DurableSliceEvidenceRef:     strings.TrimSpace(evidence.DurableSliceEvidenceRef),
		NoOpaqueDataImageDependency: true,
		FullComputerClaimed:         false,
		DataImageDisposableClaimed:  false,
		RuntimeBehaviorChanged:      false,
		DeployedRouteRegistered:     false,
		ProductionAuthTouched:       false,
		StagingClaimed:              false,
		PromotionClaimed:            false,
		VMLifecycleTouched:          false,
		RunAcceptanceRecordTouched:  false,
		NoMutation:                  true,
	}, nil
}

func validateBaseDurableStateSliceEvidence(evidence BaseDurableStateSliceEvidence) error {
	if strings.TrimSpace(evidence.EquivalenceContractRef) == "" {
		return fmt.Errorf("base durable state slice: equivalence contract ref is required")
	}
	if strings.TrimSpace(evidence.UserIsomorphismContractRef) == "" {
		return fmt.Errorf("base durable state slice: user isomorphism contract ref is required")
	}
	if strings.TrimSpace(evidence.TypedArtifactProgramRef) == "" {
		return fmt.Errorf("base durable state slice: typed artifact program ref is required")
	}
	if strings.TrimSpace(evidence.DurableSliceEvidenceRef) == "" {
		return fmt.Errorf("base durable state slice: durable slice evidence ref is required")
	}
	if !evidence.NoOpaqueDataImageDependency {
		return fmt.Errorf("base durable state slice: evidence must prove no opaque data.img dependency for this slice")
	}
	switch {
	case evidence.RuntimeBehaviorChanged:
		return fmt.Errorf("base durable state slice: evidence cannot change runtime behavior")
	case evidence.DeployedRouteRegistered:
		return fmt.Errorf("base durable state slice: evidence cannot register deployed routes")
	case evidence.ProductionAuthTouched:
		return fmt.Errorf("base durable state slice: evidence cannot touch production auth/session")
	case evidence.StagingClaimed:
		return fmt.Errorf("base durable state slice: evidence cannot claim staging")
	case evidence.PromotionClaimed:
		return fmt.Errorf("base durable state slice: evidence cannot claim promotion")
	case evidence.VMLifecycleTouched:
		return fmt.Errorf("base durable state slice: evidence cannot touch VM lifecycle")
	case evidence.RunAcceptanceRecordTouched:
		return fmt.Errorf("base durable state slice: evidence cannot touch run acceptance records")
	case evidence.FullComputerClaimed:
		return fmt.Errorf("base durable state slice: evidence cannot claim full-computer coverage")
	case evidence.DataImageDisposableClaimed:
		return fmt.Errorf("base durable state slice: evidence cannot claim data.img disposability")
	case !evidence.NoMutation:
		return fmt.Errorf("base durable state slice: evidence must be no-mutation")
	default:
		return nil
	}
}

func validateBaseDurableStateSliceEquivalence(equivalence BaseSubstrateEquivalenceContract) error {
	if equivalence.Kind != BaseSubstrateEquivalenceContractKind {
		return fmt.Errorf("base durable state slice: equivalence contract kind %q is not %q", equivalence.Kind, BaseSubstrateEquivalenceContractKind)
	}
	if equivalence.ClaimScope != BaseSubstrateEquivalenceClaimScope {
		return fmt.Errorf("base durable state slice: equivalence claim scope %q is not %q", equivalence.ClaimScope, BaseSubstrateEquivalenceClaimScope)
	}
	if equivalence.EquivalenceStatus != EquivalenceEquivalent {
		return fmt.Errorf("base durable state slice: equivalence status %q is not %q", equivalence.EquivalenceStatus, EquivalenceEquivalent)
	}
	if !equivalence.NoRuntimeMaterialization || !equivalence.NoOpaqueDataImageDependency || !equivalence.NoMutation || equivalence.RuntimeBehaviorChanged || equivalence.DeployedRouteRegistered || equivalence.ProductionAuthTouched || equivalence.StagingClaimed || equivalence.PromotionClaimed || equivalence.VMLifecycleTouched || equivalence.FirecrackerBootClaimed || equivalence.RunAcceptanceRecordTouched || equivalence.FullSubstrateIndependenceClaim || equivalence.CompletionClaimed {
		return fmt.Errorf("base durable state slice: equivalence contract must be local no-runtime no-mutation evidence with unsafe flags false")
	}
	if !baseSubstrateEquivalenceHasRequiredScope(equivalence.RequiredObservations) {
		return fmt.Errorf("base durable state slice: equivalence contract must include file_manifest and blob_set")
	}
	if !equivalence.Version.ArtifactProgramRef.Valid() {
		return fmt.Errorf("base durable state slice: equivalence artifact program ref is required")
	}
	return nil
}

func validateBaseDurableStateSliceUserIsomorphism(equivalence BaseSubstrateEquivalenceContract, user BaseCurrentStateUserIsomorphismContract) error {
	if user.Kind != BaseCurrentStateUserIsomorphismContractKind {
		return fmt.Errorf("base durable state slice: user isomorphism contract kind %q is not %q", user.Kind, BaseCurrentStateUserIsomorphismContractKind)
	}
	if user.Version != equivalence.Version {
		return fmt.Errorf("base durable state slice: user isomorphism version does not match equivalence version")
	}
	if user.Status != UserIsomorphismEquivalent {
		return fmt.Errorf("base durable state slice: user isomorphism status %q is not %q", user.Status, UserIsomorphismEquivalent)
	}
	if user.EquivalenceContractKind != equivalence.Kind || user.EquivalenceEvidenceRef != equivalence.EquivalenceEvidenceRef {
		return fmt.Errorf("base durable state slice: user isomorphism contract does not bind equivalence evidence")
	}
	if user.CurrentMaterializer != equivalence.CurrentMaterializer || user.CurrentSubstrate != equivalence.CurrentSubstrate || user.ProjectionMaterializer != equivalence.ProjectionMaterializer || user.ProjectionSubstrate != equivalence.ProjectionSubstrate {
		return fmt.Errorf("base durable state slice: user isomorphism realization identity does not match equivalence contract")
	}
	if !user.NoMutation || user.RuntimeBehaviorChanged || user.DeployedRouteRegistered || user.ProductionAuthTouched || user.StagingClaimed || user.PromotionClaimed || user.VMLifecycleTouched || user.RunAcceptanceRecordTouched || user.FullComputerContinuityClaimed {
		return fmt.Errorf("base durable state slice: user isomorphism contract must be no-mutation and unsafe flags false")
	}
	if !baseSubstrateEquivalenceHasRequiredScope(user.RequiredObservations) {
		return fmt.Errorf("base durable state slice: user isomorphism contract must include file_manifest and blob_set")
	}
	if !baseDurableStateSliceHasRequiredSemantics(user.RequiredSemantics) {
		return fmt.Errorf("base durable state slice: user isomorphism contract must cover file path, content, deletion, and provenance semantics")
	}
	if !baseDurableStateSliceMarksLiveProcessUnsupported(user.UnsupportedSemantics) {
		return fmt.Errorf("base durable state slice: user isomorphism contract must mark live_process_continuity unsupported")
	}
	return nil
}

func baseDurableStateSliceHasRequiredSemantics(semantics []UserSemantic) bool {
	seen := make(map[UserSemantic]struct{}, len(semantics))
	for _, semantic := range semantics {
		seen[semantic] = struct{}{}
	}
	for _, required := range []UserSemantic{UserSemanticFilePath, UserSemanticFileContent, UserSemanticDeletionState, UserSemanticFileProvenance} {
		if _, ok := seen[required]; !ok {
			return false
		}
	}
	return true
}

func baseDurableStateSliceMarksLiveProcessUnsupported(unsupported []UnsupportedUserSemantic) bool {
	for _, semantic := range unsupported {
		if semantic.Semantic == UserSemanticLiveProcessContinuity && strings.TrimSpace(semantic.Reason) != "" {
			return true
		}
	}
	return false
}
