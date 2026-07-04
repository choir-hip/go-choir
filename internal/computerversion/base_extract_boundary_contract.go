package computerversion

import (
	"fmt"
	"strings"
)

const BaseExtractBoundaryContractKind = "base_extract_boundary_contract"

const BaseExtractBoundary = "base_extract_typed_artifact_program_to_observation_set_without_opaque_image_or_runtime_mutation"

const BaseExtractScope = "base_extract_file_manifest_blob_set_observations"

const (
	BaseExtractorKindJournalBlobCurrentState = "base_journal_blob_current_state"
)

// BaseExtractBoundaryEvidence records the non-runtime proof refs for one Base
// extraction boundary. It certifies that a typed artifact-program cursor produced
// a scoped ObservationSet; it does not authorize materialization, routing,
// promotion, VM lifecycle, or data.img recovery claims.
type BaseExtractBoundaryEvidence struct {
	ExtractRequestRef             string `json:"extract_request_ref"`
	ObservationSetRef             string `json:"observation_set_ref"`
	TypedArtifactProgramRef       string `json:"typed_artifact_program_ref"`
	ExtractorKind                 string `json:"extractor_kind"`
	NoOpaqueDataImageDependency   bool   `json:"no_opaque_data_img_dependency"`
	RuntimeBehaviorChanged        bool   `json:"runtime_behavior_changed"`
	DeployedRouteRegistered       bool   `json:"deployed_route_registered"`
	ProductionAuthTouched         bool   `json:"production_auth_touched"`
	StagingClaimed                bool   `json:"staging_claimed"`
	PromotionClaimed              bool   `json:"promotion_claimed"`
	VMLifecycleTouched            bool   `json:"vm_lifecycle_touched"`
	RunAcceptanceRecordTouched    bool   `json:"run_acceptance_record_touched"`
	MaterializationClaimed        bool   `json:"materialization_claimed"`
	FullComputerContinuityClaimed bool   `json:"full_computer_continuity_claimed"`
	DataImageRecoveryClaimed      bool   `json:"data_img_recovery_claimed"`
	NoMutation                    bool   `json:"no_mutation"`
}

// BaseExtractBoundaryContract records the smallest Base extraction authority:
// an ExtractRequest for a ComputerVersion produced a file-manifest/blob-set
// ObservationSet from typed artifact-program state. It is deliberately below
// materialization and equivalence authority.
type BaseExtractBoundaryContract struct {
	Kind                          string            `json:"kind"`
	Version                       ComputerVersion   `json:"version"`
	Boundary                      string            `json:"boundary"`
	Scope                         string            `json:"scope"`
	ExtractorKind                 string            `json:"extractor_kind"`
	ExtractRequestName            string            `json:"extract_request_name"`
	ExtractRequestRef             string            `json:"extract_request_ref"`
	ObservationSetName            string            `json:"observation_set_name"`
	ObservationSetRef             string            `json:"observation_set_ref"`
	TypedArtifactProgramRef       string            `json:"typed_artifact_program_ref"`
	RequiredObservations          []ObservationKind `json:"required_observations"`
	NoOpaqueDataImageDependency   bool              `json:"no_opaque_data_img_dependency"`
	MaterializationClaimed        bool              `json:"materialization_claimed"`
	FullComputerContinuityClaimed bool              `json:"full_computer_continuity_claimed"`
	DataImageRecoveryClaimed      bool              `json:"data_img_recovery_claimed"`
	RuntimeBehaviorChanged        bool              `json:"runtime_behavior_changed"`
	DeployedRouteRegistered       bool              `json:"deployed_route_registered"`
	ProductionAuthTouched         bool              `json:"production_auth_touched"`
	StagingClaimed                bool              `json:"staging_claimed"`
	PromotionClaimed              bool              `json:"promotion_claimed"`
	VMLifecycleTouched            bool              `json:"vm_lifecycle_touched"`
	RunAcceptanceRecordTouched    bool              `json:"run_acceptance_record_touched"`
	NoMutation                    bool              `json:"no_mutation"`
}

// BuildBaseExtractBoundaryContract verifies that Base extraction stays below
// materialization: it binds the request ComputerVersion, the observation set
// version, required file/blob observation kinds, typed artifact-program ref, and
// no-mutation evidence.
func BuildBaseExtractBoundaryContract(request ExtractRequest, observations ObservationSet, evidence BaseExtractBoundaryEvidence) (BaseExtractBoundaryContract, error) {
	if err := validateBaseExtractRequest(request); err != nil {
		return BaseExtractBoundaryContract{}, err
	}
	if err := validateBaseExtractBoundaryObservations(request, observations); err != nil {
		return BaseExtractBoundaryContract{}, err
	}
	if err := validateBaseExtractBoundaryEvidence(request, evidence); err != nil {
		return BaseExtractBoundaryContract{}, err
	}
	return BaseExtractBoundaryContract{
		Kind:                          BaseExtractBoundaryContractKind,
		Version:                       request.Version,
		Boundary:                      BaseExtractBoundary,
		Scope:                         BaseExtractScope,
		ExtractorKind:                 strings.TrimSpace(evidence.ExtractorKind),
		ExtractRequestName:            strings.TrimSpace(request.Name),
		ExtractRequestRef:             strings.TrimSpace(evidence.ExtractRequestRef),
		ObservationSetName:            strings.TrimSpace(observations.Name),
		ObservationSetRef:             strings.TrimSpace(evidence.ObservationSetRef),
		TypedArtifactProgramRef:       strings.TrimSpace(evidence.TypedArtifactProgramRef),
		RequiredObservations:          []ObservationKind{ObservationBlobSet, ObservationFileManifest},
		NoOpaqueDataImageDependency:   true,
		MaterializationClaimed:        false,
		FullComputerContinuityClaimed: false,
		DataImageRecoveryClaimed:      false,
		RuntimeBehaviorChanged:        false,
		DeployedRouteRegistered:       false,
		ProductionAuthTouched:         false,
		StagingClaimed:                false,
		PromotionClaimed:              false,
		VMLifecycleTouched:            false,
		RunAcceptanceRecordTouched:    false,
		NoMutation:                    true,
	}, nil
}

func validateBaseExtractRequest(request ExtractRequest) error {
	if strings.TrimSpace(request.Name) == "" {
		return fmt.Errorf("base extract boundary: extract request name is required")
	}
	if !request.Version.Valid() {
		return fmt.Errorf("base extract boundary: extract request version is invalid")
	}
	return nil
}

func validateBaseExtractBoundaryObservations(request ExtractRequest, observations ObservationSet) error {
	if strings.TrimSpace(observations.Name) == "" {
		return fmt.Errorf("base extract boundary: observation set name is required")
	}
	if observations.Version != request.Version {
		return fmt.Errorf("base extract boundary: observation set version does not match request version")
	}
	if len(observations.Observations) == 0 {
		return fmt.Errorf("base extract boundary: observation set is empty")
	}
	if !baseSubstrateEquivalenceHasRequiredScope(observations.RequiredKinds()) {
		return fmt.Errorf("base extract boundary: observation set must include file_manifest and blob_set")
	}
	return nil
}

func validateBaseExtractBoundaryEvidence(request ExtractRequest, evidence BaseExtractBoundaryEvidence) error {
	if strings.TrimSpace(evidence.ExtractRequestRef) == "" {
		return fmt.Errorf("base extract boundary: extract request ref is required")
	}
	if strings.TrimSpace(evidence.ObservationSetRef) == "" {
		return fmt.Errorf("base extract boundary: observation set ref is required")
	}
	if ArtifactProgramRef(strings.TrimSpace(evidence.TypedArtifactProgramRef)) != request.Version.ArtifactProgramRef {
		return fmt.Errorf("base extract boundary: typed artifact program ref does not match request version")
	}
	if strings.TrimSpace(evidence.ExtractorKind) != BaseExtractorKindJournalBlobCurrentState {
		return fmt.Errorf("base extract boundary: extractor kind %q is not %q", evidence.ExtractorKind, BaseExtractorKindJournalBlobCurrentState)
	}
	if !evidence.NoOpaqueDataImageDependency {
		return fmt.Errorf("base extract boundary: evidence must prove no opaque data.img dependency for this extraction")
	}
	switch {
	case evidence.RuntimeBehaviorChanged:
		return fmt.Errorf("base extract boundary: evidence cannot change runtime behavior")
	case evidence.DeployedRouteRegistered:
		return fmt.Errorf("base extract boundary: evidence cannot register deployed routes")
	case evidence.ProductionAuthTouched:
		return fmt.Errorf("base extract boundary: evidence cannot touch production auth/session")
	case evidence.StagingClaimed:
		return fmt.Errorf("base extract boundary: evidence cannot claim staging")
	case evidence.PromotionClaimed:
		return fmt.Errorf("base extract boundary: evidence cannot claim promotion")
	case evidence.VMLifecycleTouched:
		return fmt.Errorf("base extract boundary: evidence cannot touch VM lifecycle")
	case evidence.RunAcceptanceRecordTouched:
		return fmt.Errorf("base extract boundary: evidence cannot touch run acceptance records")
	case evidence.MaterializationClaimed:
		return fmt.Errorf("base extract boundary: evidence cannot claim materialization")
	case evidence.FullComputerContinuityClaimed:
		return fmt.Errorf("base extract boundary: evidence cannot claim full-computer continuity")
	case evidence.DataImageRecoveryClaimed:
		return fmt.Errorf("base extract boundary: evidence cannot claim data.img recovery")
	case !evidence.NoMutation:
		return fmt.Errorf("base extract boundary: evidence must be no-mutation")
	default:
		return nil
	}
}
