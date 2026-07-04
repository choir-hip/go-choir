package computerversion

import (
	"fmt"
	"strings"
)

const BaseSubstrateReentryReadinessContractKind = "base_substrate_reentry_readiness_contract"

const BaseSubstrateReentryReadinessBoundary = "base_substrate_equivalence_reentry_without_runtime_or_completion_claim"

const BaseSubstrateReentryReadinessScope = "base_file_manifest_blob_set_substrate_reentry_readiness"

// BaseSubstrateReentryReadinessEvidence records the proof refs that justify
// re-entering scoped substrate-equivalence work after local checker calibration.
// It does not certify Firecracker boot, deployment, promotion, or full mission
// completion.
type BaseSubstrateReentryReadinessEvidence struct {
	SubstrateEquivalenceContractRef string `json:"substrate_equivalence_contract_ref"`
	EquivalenceEvidenceSetRef       string `json:"equivalence_evidence_set_ref"`
	NextProbeRef                    string `json:"next_probe_ref"`
	NoRuntimeMaterialization        bool   `json:"no_runtime_materialization"`
	NoOpaqueDataImageDependency     bool   `json:"no_opaque_data_img_dependency"`
	RuntimeBehaviorChanged          bool   `json:"runtime_behavior_changed"`
	DeployedRouteRegistered         bool   `json:"deployed_route_registered"`
	ProductionAuthTouched           bool   `json:"production_auth_touched"`
	StagingClaimed                  bool   `json:"staging_claimed"`
	PromotionClaimed                bool   `json:"promotion_claimed"`
	VMLifecycleTouched              bool   `json:"vm_lifecycle_touched"`
	FirecrackerBootClaimed          bool   `json:"firecracker_boot_claimed"`
	RunAcceptanceRecordTouched      bool   `json:"run_acceptance_record_touched"`
	FullSubstrateIndependenceClaim  bool   `json:"full_substrate_independence_claim"`
	CompletionClaimed               bool   `json:"completion_claimed"`
	NoMutation                      bool   `json:"no_mutation"`
}

// BaseSubstrateReentryReadinessContract binds the prior scoped substrate
// equivalence contract to the positive+negative checker calibration set. It
// authorizes only another local substrate-equivalence probe, not runtime or
// product completion claims.
type BaseSubstrateReentryReadinessContract struct {
	Kind                            string            `json:"kind"`
	Version                         ComputerVersion   `json:"version"`
	Boundary                        string            `json:"boundary"`
	Scope                           string            `json:"scope"`
	ClaimScope                      string            `json:"claim_scope"`
	CurrentMaterializer             string            `json:"current_materializer"`
	CurrentSubstrate                string            `json:"current_substrate"`
	ProjectionMaterializer          string            `json:"projection_materializer"`
	ProjectionSubstrate             string            `json:"projection_substrate"`
	SubstrateEquivalenceStatus      EquivalenceStatus `json:"substrate_equivalence_status"`
	CalibrationSuccessStatus        EquivalenceStatus `json:"calibration_success_status"`
	CalibrationFailureStatus        EquivalenceStatus `json:"calibration_failure_status"`
	RequiredObservations            []ObservationKind `json:"required_observations"`
	SubstrateEquivalenceContractRef string            `json:"substrate_equivalence_contract_ref"`
	EquivalenceEvidenceSetRef       string            `json:"equivalence_evidence_set_ref"`
	NextProbeRef                    string            `json:"next_probe_ref"`
	LocalSubstrateReentryAllowed    bool              `json:"local_substrate_reentry_allowed"`
	NoRuntimeMaterialization        bool              `json:"no_runtime_materialization"`
	NoOpaqueDataImageDependency     bool              `json:"no_opaque_data_img_dependency"`
	RuntimeBehaviorChanged          bool              `json:"runtime_behavior_changed"`
	DeployedRouteRegistered         bool              `json:"deployed_route_registered"`
	ProductionAuthTouched           bool              `json:"production_auth_touched"`
	StagingClaimed                  bool              `json:"staging_claimed"`
	PromotionClaimed                bool              `json:"promotion_claimed"`
	VMLifecycleTouched              bool              `json:"vm_lifecycle_touched"`
	FirecrackerBootClaimed          bool              `json:"firecracker_boot_claimed"`
	RunAcceptanceRecordTouched      bool              `json:"run_acceptance_record_touched"`
	FullSubstrateIndependenceClaim  bool              `json:"full_substrate_independence_claim"`
	CompletionClaimed               bool              `json:"completion_claimed"`
	NoMutation                      bool              `json:"no_mutation"`
}

// BuildBaseSubstrateReentryReadinessContract verifies that scoped substrate
// equivalence can be re-entered only after both a passing substrate comparison
// and a calibrated success/failure equivalence evidence set exist for the same
// ComputerVersion.
func BuildBaseSubstrateReentryReadinessContract(substrate BaseSubstrateEquivalenceContract, calibration BaseEquivalenceEvidenceSetContract, evidence BaseSubstrateReentryReadinessEvidence) (BaseSubstrateReentryReadinessContract, error) {
	if err := validateBaseSubstrateReentryEquivalence(substrate); err != nil {
		return BaseSubstrateReentryReadinessContract{}, err
	}
	if err := validateBaseSubstrateReentryCalibration(calibration); err != nil {
		return BaseSubstrateReentryReadinessContract{}, err
	}
	if substrate.Version != calibration.Version {
		return BaseSubstrateReentryReadinessContract{}, fmt.Errorf("base substrate reentry readiness: substrate and calibration contracts name different computer versions")
	}
	if err := validateBaseSubstrateReentryReadinessEvidence(evidence); err != nil {
		return BaseSubstrateReentryReadinessContract{}, err
	}
	return BaseSubstrateReentryReadinessContract{
		Kind:                            BaseSubstrateReentryReadinessContractKind,
		Version:                         substrate.Version,
		Boundary:                        BaseSubstrateReentryReadinessBoundary,
		Scope:                           BaseSubstrateReentryReadinessScope,
		ClaimScope:                      substrate.ClaimScope,
		CurrentMaterializer:             substrate.CurrentMaterializer,
		CurrentSubstrate:                substrate.CurrentSubstrate,
		ProjectionMaterializer:          substrate.ProjectionMaterializer,
		ProjectionSubstrate:             substrate.ProjectionSubstrate,
		SubstrateEquivalenceStatus:      substrate.EquivalenceStatus,
		CalibrationSuccessStatus:        calibration.SuccessStatus,
		CalibrationFailureStatus:        calibration.FailureStatus,
		RequiredObservations:            []ObservationKind{ObservationBlobSet, ObservationFileManifest},
		SubstrateEquivalenceContractRef: strings.TrimSpace(evidence.SubstrateEquivalenceContractRef),
		EquivalenceEvidenceSetRef:       strings.TrimSpace(evidence.EquivalenceEvidenceSetRef),
		NextProbeRef:                    strings.TrimSpace(evidence.NextProbeRef),
		LocalSubstrateReentryAllowed:    true,
		NoRuntimeMaterialization:        true,
		NoOpaqueDataImageDependency:     true,
		RuntimeBehaviorChanged:          false,
		DeployedRouteRegistered:         false,
		ProductionAuthTouched:           false,
		StagingClaimed:                  false,
		PromotionClaimed:                false,
		VMLifecycleTouched:              false,
		FirecrackerBootClaimed:          false,
		RunAcceptanceRecordTouched:      false,
		FullSubstrateIndependenceClaim:  false,
		CompletionClaimed:               false,
		NoMutation:                      true,
	}, nil
}

func validateBaseSubstrateReentryEquivalence(substrate BaseSubstrateEquivalenceContract) error {
	if substrate.Kind != BaseSubstrateEquivalenceContractKind {
		return fmt.Errorf("base substrate reentry readiness: substrate contract kind is %q", substrate.Kind)
	}
	if substrate.Boundary != BaseSubstrateEquivalenceBoundary {
		return fmt.Errorf("base substrate reentry readiness: substrate contract boundary is %q", substrate.Boundary)
	}
	if substrate.ClaimScope != BaseSubstrateEquivalenceClaimScope {
		return fmt.Errorf("base substrate reentry readiness: substrate claim scope is %q", substrate.ClaimScope)
	}
	if substrate.EquivalenceStatus != EquivalenceEquivalent {
		return fmt.Errorf("base substrate reentry readiness: substrate equivalence status is %q", substrate.EquivalenceStatus)
	}
	if !baseSubstrateEquivalenceHasRequiredScope(substrate.RequiredObservations) {
		return fmt.Errorf("base substrate reentry readiness: substrate contract must include file_manifest and blob_set")
	}
	if strings.TrimSpace(substrate.CurrentMaterializer) == "" || strings.TrimSpace(substrate.CurrentSubstrate) == "" || strings.TrimSpace(substrate.ProjectionMaterializer) == "" || strings.TrimSpace(substrate.ProjectionSubstrate) == "" {
		return fmt.Errorf("base substrate reentry readiness: substrate contract must name current and projection materializers")
	}
	if substrate.CurrentMaterializer == substrate.ProjectionMaterializer && substrate.CurrentSubstrate == substrate.ProjectionSubstrate {
		return fmt.Errorf("base substrate reentry readiness: substrate contract must compare non-identical materializer or substrate")
	}
	if !substrate.NoRuntimeMaterialization || !substrate.NoOpaqueDataImageDependency || !substrate.NoMutation {
		return fmt.Errorf("base substrate reentry readiness: substrate contract has unsafe proof flags")
	}
	if substrate.RuntimeBehaviorChanged || substrate.DeployedRouteRegistered || substrate.ProductionAuthTouched || substrate.StagingClaimed || substrate.PromotionClaimed || substrate.VMLifecycleTouched || substrate.FirecrackerBootClaimed || substrate.RunAcceptanceRecordTouched || substrate.FullSubstrateIndependenceClaim || substrate.CompletionClaimed {
		return fmt.Errorf("base substrate reentry readiness: substrate contract carries protected-surface claims")
	}
	return nil
}

func validateBaseSubstrateReentryCalibration(calibration BaseEquivalenceEvidenceSetContract) error {
	if calibration.Kind != BaseEquivalenceEvidenceSetContractKind {
		return fmt.Errorf("base substrate reentry readiness: calibration contract kind is %q", calibration.Kind)
	}
	if calibration.Boundary != BaseEquivalenceEvidenceSetBoundary {
		return fmt.Errorf("base substrate reentry readiness: calibration contract boundary is %q", calibration.Boundary)
	}
	if calibration.Scope != BaseEquivalenceEvidenceSetScope {
		return fmt.Errorf("base substrate reentry readiness: calibration contract scope is %q", calibration.Scope)
	}
	if calibration.SuccessStatus != EquivalenceEquivalent {
		return fmt.Errorf("base substrate reentry readiness: calibration success status is %q", calibration.SuccessStatus)
	}
	switch calibration.FailureStatus {
	case EquivalenceNotEquivalent:
		if calibration.FailureDifferenceCount == 0 || calibration.FailureUnsupportedCount != 0 {
			return fmt.Errorf("base substrate reentry readiness: calibration non-equivalent counts are invalid")
		}
	case EquivalenceNarrowed:
		if calibration.FailureUnsupportedCount == 0 || calibration.FailureDifferenceCount != 0 {
			return fmt.Errorf("base substrate reentry readiness: calibration narrowed counts are invalid")
		}
	default:
		return fmt.Errorf("base substrate reentry readiness: calibration failure status is %q", calibration.FailureStatus)
	}
	if !baseSubstrateEquivalenceHasRequiredScope(calibration.RequiredObservations) {
		return fmt.Errorf("base substrate reentry readiness: calibration must include file_manifest and blob_set")
	}
	if !calibration.NoRuntimeMaterialization || !calibration.NoOpaqueDataImageDependency || !calibration.NoMutation {
		return fmt.Errorf("base substrate reentry readiness: calibration has unsafe proof flags")
	}
	if calibration.RuntimeBehaviorChanged || calibration.DeployedRouteRegistered || calibration.ProductionAuthTouched || calibration.StagingClaimed || calibration.PromotionClaimed || calibration.VMLifecycleTouched || calibration.FirecrackerBootClaimed || calibration.RunAcceptanceRecordTouched || calibration.FullSubstrateIndependenceClaim {
		return fmt.Errorf("base substrate reentry readiness: calibration carries protected-surface claims")
	}
	return nil
}

func validateBaseSubstrateReentryReadinessEvidence(evidence BaseSubstrateReentryReadinessEvidence) error {
	if strings.TrimSpace(evidence.SubstrateEquivalenceContractRef) == "" {
		return fmt.Errorf("base substrate reentry readiness: substrate equivalence contract ref is required")
	}
	if strings.TrimSpace(evidence.EquivalenceEvidenceSetRef) == "" {
		return fmt.Errorf("base substrate reentry readiness: equivalence evidence set ref is required")
	}
	if strings.TrimSpace(evidence.NextProbeRef) == "" {
		return fmt.Errorf("base substrate reentry readiness: next probe ref is required")
	}
	if !evidence.NoRuntimeMaterialization {
		return fmt.Errorf("base substrate reentry readiness: evidence must prove no runtime materialization")
	}
	if !evidence.NoOpaqueDataImageDependency {
		return fmt.Errorf("base substrate reentry readiness: evidence must prove no opaque data.img dependency")
	}
	switch {
	case evidence.RuntimeBehaviorChanged:
		return fmt.Errorf("base substrate reentry readiness: evidence cannot change runtime behavior")
	case evidence.DeployedRouteRegistered:
		return fmt.Errorf("base substrate reentry readiness: evidence cannot register deployed routes")
	case evidence.ProductionAuthTouched:
		return fmt.Errorf("base substrate reentry readiness: evidence cannot touch production auth/session")
	case evidence.StagingClaimed:
		return fmt.Errorf("base substrate reentry readiness: evidence cannot claim staging")
	case evidence.PromotionClaimed:
		return fmt.Errorf("base substrate reentry readiness: evidence cannot claim promotion")
	case evidence.VMLifecycleTouched:
		return fmt.Errorf("base substrate reentry readiness: evidence cannot touch VM lifecycle")
	case evidence.FirecrackerBootClaimed:
		return fmt.Errorf("base substrate reentry readiness: evidence cannot claim Firecracker boot")
	case evidence.RunAcceptanceRecordTouched:
		return fmt.Errorf("base substrate reentry readiness: evidence cannot touch run acceptance records")
	case evidence.FullSubstrateIndependenceClaim:
		return fmt.Errorf("base substrate reentry readiness: evidence cannot claim full substrate independence")
	case evidence.CompletionClaimed:
		return fmt.Errorf("base substrate reentry readiness: evidence cannot claim completion")
	case !evidence.NoMutation:
		return fmt.Errorf("base substrate reentry readiness: evidence must be no-mutation")
	default:
		return nil
	}
}
