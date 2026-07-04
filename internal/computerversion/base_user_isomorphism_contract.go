package computerversion

import (
	"fmt"
	"strings"
)

const BaseCurrentStateUserIsomorphismContractKind = "base_current_state_user_isomorphism_contract"

const BaseCurrentStateUserIsomorphismBoundary = "scoped_base_current_state_user_isomorphism_without_runtime_mutation"

const BaseCurrentStateUserIsomorphismScopeName = "base_current_state_file_manifest_blob_set_user_semantics"

// BaseCurrentStateUserIsomorphismScope returns the exact user-visible semantic
// scope covered by the Base current-state file-manifest/blob-set slice. It is a
// deliberately narrow claim: live processes, sockets, terminals, packages, and
// full-computer continuity remain outside this proof.
func BaseCurrentStateUserIsomorphismScope() UserIsomorphismScope {
	return UserIsomorphismScope{
		Name: BaseCurrentStateUserIsomorphismScopeName,
		ObservationKinds: []ObservationKind{
			ObservationBlobSet,
			ObservationFileManifest,
		},
		RequiredSemantics: []UserSemantic{
			UserSemanticFilePath,
			UserSemanticFileContent,
			UserSemanticDeletionState,
			UserSemanticFileProvenance,
		},
		CoveredSemantics: []UserSemantic{
			UserSemanticFilePath,
			UserSemanticFileContent,
			UserSemanticDeletionState,
			UserSemanticFileProvenance,
		},
		UnsupportedSemantics: []UnsupportedUserSemantic{
			{Semantic: UserSemanticLiveProcessContinuity, Reason: "base current-state file/blob slice excludes live process state"},
		},
	}
}

// BaseCurrentStateUserIsomorphismEvidence names proof artifacts for the narrow
// user-isomorphism claim produced from a BaseSubstrateEquivalenceContract. It
// records evidence refs only and must remain no-mutation.
type BaseCurrentStateUserIsomorphismEvidence struct {
	EquivalenceContractRef        string `json:"equivalence_contract_ref"`
	ScopeRef                      string `json:"scope_ref"`
	UserIsomorphismEvidenceRef    string `json:"user_isomorphism_evidence_ref"`
	RuntimeBehaviorChanged        bool   `json:"runtime_behavior_changed"`
	DeployedRouteRegistered       bool   `json:"deployed_route_registered"`
	ProductionAuthTouched         bool   `json:"production_auth_touched"`
	StagingClaimed                bool   `json:"staging_claimed"`
	PromotionClaimed              bool   `json:"promotion_claimed"`
	VMLifecycleTouched            bool   `json:"vm_lifecycle_touched"`
	RunAcceptanceRecordTouched    bool   `json:"run_acceptance_record_touched"`
	FullComputerContinuityClaimed bool   `json:"full_computer_continuity_claimed"`
	NoMutation                    bool   `json:"no_mutation"`
}

// BaseCurrentStateUserIsomorphismContract records that the Pass 87 Base
// substrate-equivalence contract supports a scoped user-isomorphism claim for
// file path/content/deletion/provenance semantics only.
type BaseCurrentStateUserIsomorphismContract struct {
	Kind                          string                    `json:"kind"`
	Version                       ComputerVersion           `json:"version"`
	Boundary                      string                    `json:"boundary"`
	Scope                         UserIsomorphismScope      `json:"scope"`
	Status                        UserIsomorphismStatus     `json:"status"`
	EquivalenceContractKind       string                    `json:"equivalence_contract_kind"`
	EquivalenceContractRef        string                    `json:"equivalence_contract_ref"`
	EquivalenceEvidenceRef        string                    `json:"equivalence_evidence_ref"`
	ScopeRef                      string                    `json:"scope_ref"`
	UserIsomorphismEvidenceRef    string                    `json:"user_isomorphism_evidence_ref"`
	CurrentMaterializer           string                    `json:"current_materializer"`
	CurrentSubstrate              string                    `json:"current_substrate"`
	ProjectionMaterializer        string                    `json:"projection_materializer"`
	ProjectionSubstrate           string                    `json:"projection_substrate"`
	RequiredObservations          []ObservationKind         `json:"required_observations"`
	RequiredSemantics             []UserSemantic            `json:"required_semantics"`
	UnsupportedSemantics          []UnsupportedUserSemantic `json:"unsupported_semantics,omitempty"`
	RuntimeBehaviorChanged        bool                      `json:"runtime_behavior_changed"`
	DeployedRouteRegistered       bool                      `json:"deployed_route_registered"`
	ProductionAuthTouched         bool                      `json:"production_auth_touched"`
	StagingClaimed                bool                      `json:"staging_claimed"`
	PromotionClaimed              bool                      `json:"promotion_claimed"`
	VMLifecycleTouched            bool                      `json:"vm_lifecycle_touched"`
	RunAcceptanceRecordTouched    bool                      `json:"run_acceptance_record_touched"`
	FullComputerContinuityClaimed bool                      `json:"full_computer_continuity_claimed"`
	NoMutation                    bool                      `json:"no_mutation"`
}

// BuildBaseCurrentStateUserIsomorphismContract verifies that the scoped Base
// current-state equivalence proof supports exactly the file/blob user semantics
// this package can prove. It must not be used as a full-computer claim.
func BuildBaseCurrentStateUserIsomorphismContract(current, projection Realization, equivalence BaseSubstrateEquivalenceContract, evidence BaseCurrentStateUserIsomorphismEvidence) (BaseCurrentStateUserIsomorphismContract, error) {
	if err := validateBaseCurrentStateUserIsomorphismEvidence(evidence); err != nil {
		return BaseCurrentStateUserIsomorphismContract{}, err
	}
	if err := validateBaseCurrentStateUserIsomorphismEquivalence(current, projection, equivalence); err != nil {
		return BaseCurrentStateUserIsomorphismContract{}, err
	}

	scope := BaseCurrentStateUserIsomorphismScope()
	result := UserIsomorphismChecker{}.CheckRealizations(current, projection, scope)
	if result.Status == UserIsomorphismNarrowed {
		return BaseCurrentStateUserIsomorphismContract{}, fmt.Errorf("base current-state user isomorphism: claim narrowed: %v", result.Unsupported)
	}
	if !result.UserIsomorphic() {
		return BaseCurrentStateUserIsomorphismContract{}, fmt.Errorf("base current-state user isomorphism: realizations are not user-isomorphic: %v", result.Differences)
	}

	return BaseCurrentStateUserIsomorphismContract{
		Kind:                          BaseCurrentStateUserIsomorphismContractKind,
		Version:                       current.Version,
		Boundary:                      BaseCurrentStateUserIsomorphismBoundary,
		Scope:                         scope,
		Status:                        UserIsomorphismEquivalent,
		EquivalenceContractKind:       equivalence.Kind,
		EquivalenceContractRef:        strings.TrimSpace(evidence.EquivalenceContractRef),
		EquivalenceEvidenceRef:        equivalence.EquivalenceEvidenceRef,
		ScopeRef:                      strings.TrimSpace(evidence.ScopeRef),
		UserIsomorphismEvidenceRef:    strings.TrimSpace(evidence.UserIsomorphismEvidenceRef),
		CurrentMaterializer:           equivalence.CurrentMaterializer,
		CurrentSubstrate:              equivalence.CurrentSubstrate,
		ProjectionMaterializer:        equivalence.ProjectionMaterializer,
		ProjectionSubstrate:           equivalence.ProjectionSubstrate,
		RequiredObservations:          canonicalObservationKinds(scope.ObservationKinds),
		RequiredSemantics:             canonicalUserSemantics(scope.RequiredSemantics),
		UnsupportedSemantics:          append([]UnsupportedUserSemantic(nil), scope.UnsupportedSemantics...),
		RuntimeBehaviorChanged:        false,
		DeployedRouteRegistered:       false,
		ProductionAuthTouched:         false,
		StagingClaimed:                false,
		PromotionClaimed:              false,
		VMLifecycleTouched:            false,
		RunAcceptanceRecordTouched:    false,
		FullComputerContinuityClaimed: false,
		NoMutation:                    true,
	}, nil
}

func validateBaseCurrentStateUserIsomorphismEvidence(evidence BaseCurrentStateUserIsomorphismEvidence) error {
	if strings.TrimSpace(evidence.EquivalenceContractRef) == "" {
		return fmt.Errorf("base current-state user isomorphism: equivalence contract ref is required")
	}
	if strings.TrimSpace(evidence.ScopeRef) == "" {
		return fmt.Errorf("base current-state user isomorphism: scope ref is required")
	}
	if strings.TrimSpace(evidence.UserIsomorphismEvidenceRef) == "" {
		return fmt.Errorf("base current-state user isomorphism: user isomorphism evidence ref is required")
	}
	switch {
	case evidence.RuntimeBehaviorChanged:
		return fmt.Errorf("base current-state user isomorphism: evidence cannot change runtime behavior")
	case evidence.DeployedRouteRegistered:
		return fmt.Errorf("base current-state user isomorphism: evidence cannot register deployed routes")
	case evidence.ProductionAuthTouched:
		return fmt.Errorf("base current-state user isomorphism: evidence cannot touch production auth/session")
	case evidence.StagingClaimed:
		return fmt.Errorf("base current-state user isomorphism: evidence cannot claim staging")
	case evidence.PromotionClaimed:
		return fmt.Errorf("base current-state user isomorphism: evidence cannot claim promotion")
	case evidence.VMLifecycleTouched:
		return fmt.Errorf("base current-state user isomorphism: evidence cannot touch VM lifecycle")
	case evidence.RunAcceptanceRecordTouched:
		return fmt.Errorf("base current-state user isomorphism: evidence cannot touch run acceptance records")
	case evidence.FullComputerContinuityClaimed:
		return fmt.Errorf("base current-state user isomorphism: evidence cannot claim full-computer continuity")
	case !evidence.NoMutation:
		return fmt.Errorf("base current-state user isomorphism: evidence must be no-mutation")
	default:
		return nil
	}
}

func validateBaseCurrentStateUserIsomorphismEquivalence(current, projection Realization, equivalence BaseSubstrateEquivalenceContract) error {
	if equivalence.Kind != BaseSubstrateEquivalenceContractKind {
		return fmt.Errorf("base current-state user isomorphism: equivalence contract kind %q is not %q", equivalence.Kind, BaseSubstrateEquivalenceContractKind)
	}
	if equivalence.ClaimScope != BaseSubstrateEquivalenceClaimScope {
		return fmt.Errorf("base current-state user isomorphism: equivalence claim scope %q is not %q", equivalence.ClaimScope, BaseSubstrateEquivalenceClaimScope)
	}
	if equivalence.EquivalenceStatus != EquivalenceEquivalent {
		return fmt.Errorf("base current-state user isomorphism: equivalence status %q is not %q", equivalence.EquivalenceStatus, EquivalenceEquivalent)
	}
	if !equivalence.NoMutation || equivalence.RuntimeBehaviorChanged || equivalence.DeployedRouteRegistered || equivalence.ProductionAuthTouched || equivalence.StagingClaimed || equivalence.PromotionClaimed || equivalence.VMLifecycleTouched || equivalence.RunAcceptanceRecordTouched {
		return fmt.Errorf("base current-state user isomorphism: equivalence contract must be no-mutation and unsafe flags false")
	}
	if !baseSubstrateEquivalenceHasRequiredScope(equivalence.RequiredObservations) {
		return fmt.Errorf("base current-state user isomorphism: equivalence contract must include file_manifest and blob_set")
	}
	if current.Version != equivalence.Version || projection.Version != equivalence.Version {
		return fmt.Errorf("base current-state user isomorphism: realization versions must match equivalence contract version")
	}
	if strings.TrimSpace(current.Capabilities.Materializer) != equivalence.CurrentMaterializer || strings.TrimSpace(current.Capabilities.Substrate) != equivalence.CurrentSubstrate {
		return fmt.Errorf("base current-state user isomorphism: current realization identity does not match equivalence contract")
	}
	if strings.TrimSpace(projection.Capabilities.Materializer) != equivalence.ProjectionMaterializer || strings.TrimSpace(projection.Capabilities.Substrate) != equivalence.ProjectionSubstrate {
		return fmt.Errorf("base current-state user isomorphism: projection realization identity does not match equivalence contract")
	}
	return nil
}

func canonicalUserSemantics(values []UserSemantic) []UserSemantic {
	ordered := append([]UserSemantic(nil), values...)
	for i := 0; i < len(ordered); i++ {
		for j := i + 1; j < len(ordered); j++ {
			if ordered[j] < ordered[i] {
				ordered[i], ordered[j] = ordered[j], ordered[i]
			}
		}
	}
	out := ordered[:0]
	for _, value := range ordered {
		if len(out) == 0 || out[len(out)-1] != value {
			out = append(out, value)
		}
	}
	return out
}
