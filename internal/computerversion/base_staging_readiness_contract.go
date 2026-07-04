package computerversion

import (
	"fmt"
	"strings"
)

const BaseStagingReadinessContractKind = "base_staging_readiness_contract"

const BaseStagingReadinessBoundary = "staging_readiness_without_deployment_health_route_identity_or_downstream_claim"

const BaseStagingReadinessScope = "runtime_equivalence_authorizes_staging_smoke_probe_only"

// BaseStagingReadinessEvidence records proof refs for deciding whether a
// bounded runtime-equivalence retry can proceed to a staging smoke probe. It is
// not deployed health, route identity, promotion, package-publication,
// run-acceptance, full-substrate, or completion evidence.
type BaseStagingReadinessEvidence struct {
	RuntimeEquivalenceRetryRef   string `json:"runtime_equivalence_retry_ref"`
	StagingSmokePlanRef          string `json:"staging_smoke_plan_ref"`
	BuildIdentityExpectationRef  string `json:"build_identity_expectation_ref"`
	RollbackPlanRef              string `json:"rollback_plan_ref"`
	NoDeploymentMutation         bool   `json:"no_deployment_mutation"`
	NoRouteRegistrationMutation  bool   `json:"no_route_registration_mutation"`
	NoRunAcceptanceMutation      bool   `json:"no_run_acceptance_mutation"`
	NoProductionMutation         bool   `json:"no_production_mutation"`
	DeploymentExecuted           bool   `json:"deployment_executed"`
	StagingHealthClaimed         bool   `json:"staging_health_claimed"`
	DeployedRouteIdentityClaimed bool   `json:"deployed_route_identity_claimed"`
	RuntimeBehaviorChanged       bool   `json:"runtime_behavior_changed"`
	DeployedRouteRegistered      bool   `json:"deployed_route_registered"`
	ProductionAuthTouched        bool   `json:"production_auth_touched"`
	PromotionClaimed             bool   `json:"promotion_claimed"`
	VMLifecycleTouched           bool   `json:"vm_lifecycle_touched"`
	FirecrackerBootClaimed       bool   `json:"firecracker_boot_claimed"`
	RunAcceptanceRecordTouched   bool   `json:"run_acceptance_record_touched"`
	PackagePublicationClaimed    bool   `json:"package_publication_claimed"`
	FullSubstrateClaimed         bool   `json:"full_substrate_claimed"`
	CompletionClaimed            bool   `json:"completion_claimed"`
}

// BaseStagingReadinessContract records that a bounded runtime-equivalence retry
// is sufficient to plan a staging smoke probe. It deliberately does not claim a
// deployment occurred, that staging health matches a build, that route identity
// is verified, or that any promotion/publication/run-acceptance gate is settled.
type BaseStagingReadinessContract struct {
	Kind                            string            `json:"kind"`
	Version                         ComputerVersion   `json:"version"`
	Boundary                        string            `json:"boundary"`
	Scope                           string            `json:"scope"`
	TypedArtifactProgramRef         string            `json:"typed_artifact_program_ref"`
	RuntimeEquivalenceRetryRef      string            `json:"runtime_equivalence_retry_ref"`
	SourceProvenanceReadinessRef    string            `json:"source_provenance_readiness_ref"`
	StagingSmokePlanRef             string            `json:"staging_smoke_plan_ref"`
	BuildIdentityExpectationRef     string            `json:"build_identity_expectation_ref"`
	RollbackPlanRef                 string            `json:"rollback_plan_ref"`
	RequiredObservations            []ObservationKind `json:"required_observations"`
	RuntimeEquivalenceAccepted      bool              `json:"runtime_equivalence_accepted"`
	StagingSmokeMayRun              bool              `json:"staging_smoke_may_run"`
	DeploymentHealthProofRequired   bool              `json:"deployment_health_proof_required"`
	RouteIdentityProofRequired      bool              `json:"route_identity_proof_required"`
	PromotionProofRequired          bool              `json:"promotion_proof_required"`
	PackagePublicationProofRequired bool              `json:"package_publication_proof_required"`
	RunAcceptanceProofRequired      bool              `json:"run_acceptance_proof_required"`
	NoDeploymentMutation            bool              `json:"no_deployment_mutation"`
	NoRouteRegistrationMutation     bool              `json:"no_route_registration_mutation"`
	NoRunAcceptanceMutation         bool              `json:"no_run_acceptance_mutation"`
	NoProductionMutation            bool              `json:"no_production_mutation"`
	DeploymentExecuted              bool              `json:"deployment_executed"`
	StagingHealthClaimed            bool              `json:"staging_health_claimed"`
	DeployedRouteIdentityClaimed    bool              `json:"deployed_route_identity_claimed"`
	RuntimeBehaviorChanged          bool              `json:"runtime_behavior_changed"`
	DeployedRouteRegistered         bool              `json:"deployed_route_registered"`
	ProductionAuthTouched           bool              `json:"production_auth_touched"`
	PromotionClaimed                bool              `json:"promotion_claimed"`
	VMLifecycleTouched              bool              `json:"vm_lifecycle_touched"`
	FirecrackerBootClaimed          bool              `json:"firecracker_boot_claimed"`
	RunAcceptanceRecordTouched      bool              `json:"run_acceptance_record_touched"`
	PackagePublicationClaimed       bool              `json:"package_publication_claimed"`
	FullSubstrateClaimed            bool              `json:"full_substrate_claimed"`
	CompletionClaimed               bool              `json:"completion_claimed"`
}

// BuildBaseStagingReadinessContract converts bounded runtime equivalence into
// permission to run a staging smoke probe, while preserving every downstream
// proof boundary as still required.
func BuildBaseStagingReadinessContract(retry BaseRuntimeEquivalenceRetryContract, evidence BaseStagingReadinessEvidence) (BaseStagingReadinessContract, error) {
	if err := validateBaseStagingReadinessRetry(retry); err != nil {
		return BaseStagingReadinessContract{}, err
	}
	if err := validateBaseStagingReadinessEvidence(evidence); err != nil {
		return BaseStagingReadinessContract{}, err
	}

	return BaseStagingReadinessContract{
		Kind:                            BaseStagingReadinessContractKind,
		Version:                         retry.Version,
		Boundary:                        BaseStagingReadinessBoundary,
		Scope:                           BaseStagingReadinessScope,
		TypedArtifactProgramRef:         string(retry.Version.ArtifactProgramRef),
		RuntimeEquivalenceRetryRef:      strings.TrimSpace(evidence.RuntimeEquivalenceRetryRef),
		SourceProvenanceReadinessRef:    retry.SourceProvenanceReadinessRef,
		StagingSmokePlanRef:             strings.TrimSpace(evidence.StagingSmokePlanRef),
		BuildIdentityExpectationRef:     strings.TrimSpace(evidence.BuildIdentityExpectationRef),
		RollbackPlanRef:                 strings.TrimSpace(evidence.RollbackPlanRef),
		RequiredObservations:            canonicalObservationKinds(retry.RequiredObservations),
		RuntimeEquivalenceAccepted:      true,
		StagingSmokeMayRun:              true,
		DeploymentHealthProofRequired:   true,
		RouteIdentityProofRequired:      true,
		PromotionProofRequired:          true,
		PackagePublicationProofRequired: true,
		RunAcceptanceProofRequired:      true,
		NoDeploymentMutation:            true,
		NoRouteRegistrationMutation:     true,
		NoRunAcceptanceMutation:         true,
		NoProductionMutation:            true,
	}, nil
}

func validateBaseStagingReadinessRetry(retry BaseRuntimeEquivalenceRetryContract) error {
	if retry.Kind != BaseRuntimeEquivalenceRetryContractKind {
		return fmt.Errorf("base staging readiness: retry kind is %q", retry.Kind)
	}
	if retry.Boundary != BaseRuntimeEquivalenceRetryBoundary {
		return fmt.Errorf("base staging readiness: retry boundary is %q", retry.Boundary)
	}
	if retry.Scope != BaseRuntimeEquivalenceRetryScope {
		return fmt.Errorf("base staging readiness: retry scope is %q", retry.Scope)
	}
	if !retry.Version.Valid() {
		return fmt.Errorf("base staging readiness: retry version is invalid")
	}
	if !retry.Version.ArtifactProgramRef.Valid() || ArtifactProgramRef(retry.TypedArtifactProgramRef) != retry.Version.ArtifactProgramRef {
		return fmt.Errorf("base staging readiness: retry typed artifact-program ref is invalid")
	}
	if strings.TrimSpace(retry.SourceProvenanceReadinessRef) == "" || strings.TrimSpace(retry.RuntimeEquivalenceRetryRef) == "" {
		return fmt.Errorf("base staging readiness: retry refs are required")
	}
	if retry.RuntimeEquivalenceStatus != EquivalenceEquivalent || !retry.RuntimeEquivalenceClaimed {
		return fmt.Errorf("base staging readiness: retry must have accepted runtime equivalence")
	}
	if !baseSubstrateEquivalenceHasRequiredScope(retry.RequiredObservations) || observationKindsContain(retry.RequiredObservations, ObservationVMStateManifest) {
		return fmt.Errorf("base staging readiness: retry must require typed file/blob observations only")
	}
	if !retry.StagingProofRequired || !retry.PromotionProofRequired || !retry.PackagePublicationProofRequired || !retry.RunAcceptanceProofRequired {
		return fmt.Errorf("base staging readiness: retry must preserve downstream proof requirements")
	}
	if !retry.NoVMLifecycleMutation || !retry.NoProductionMutation || !retry.NoOpaqueDataImageDependency {
		return fmt.Errorf("base staging readiness: retry must prove no VM lifecycle mutation, production mutation, or opaque data.img dependency")
	}
	if retry.RuntimeBehaviorChanged || retry.DeployedRouteRegistered || retry.ProductionAuthTouched || retry.StagingClaimed || retry.PromotionClaimed || retry.VMLifecycleTouched || retry.FirecrackerBootClaimed || retry.RunAcceptanceRecordTouched || retry.PackagePublicationClaimed || retry.FullSubstrateClaimed || retry.CompletionClaimed {
		return fmt.Errorf("base staging readiness: retry carries protected-surface or completion claims")
	}
	return nil
}

func validateBaseStagingReadinessEvidence(evidence BaseStagingReadinessEvidence) error {
	if strings.TrimSpace(evidence.RuntimeEquivalenceRetryRef) == "" || strings.TrimSpace(evidence.StagingSmokePlanRef) == "" || strings.TrimSpace(evidence.BuildIdentityExpectationRef) == "" || strings.TrimSpace(evidence.RollbackPlanRef) == "" {
		return fmt.Errorf("base staging readiness: evidence refs are required")
	}
	if !evidence.NoDeploymentMutation || !evidence.NoRouteRegistrationMutation || !evidence.NoRunAcceptanceMutation || !evidence.NoProductionMutation {
		return fmt.Errorf("base staging readiness: evidence must prove no deployment, route registration, run-acceptance, or production mutation")
	}
	if evidence.DeploymentExecuted || evidence.StagingHealthClaimed || evidence.DeployedRouteIdentityClaimed {
		return fmt.Errorf("base staging readiness: evidence cannot claim deployment execution, staging health, or route identity")
	}
	if evidence.RuntimeBehaviorChanged || evidence.DeployedRouteRegistered || evidence.ProductionAuthTouched || evidence.PromotionClaimed || evidence.VMLifecycleTouched || evidence.FirecrackerBootClaimed || evidence.RunAcceptanceRecordTouched || evidence.PackagePublicationClaimed || evidence.FullSubstrateClaimed || evidence.CompletionClaimed {
		return fmt.Errorf("base staging readiness: evidence carries protected-surface or completion claims")
	}
	return nil
}
