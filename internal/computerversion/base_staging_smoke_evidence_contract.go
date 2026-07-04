package computerversion

import (
	"fmt"
	"strings"
)

const BaseStagingSmokeEvidenceContractKind = "base_staging_smoke_evidence_contract"

const BaseStagingSmokeEvidenceBoundary = "product_path_staging_smoke_evidence_without_promotion_publication_or_run_acceptance"

const BaseStagingSmokeEvidenceScope = "staging_product_path_probe_build_and_route_identity_only"

const BaseStagingSmokeHealthPassed = "passed"

// BaseStagingSmokeEvidence records one product-path staging probe result. It may
// claim that the observed staging build and route identity matched expectations,
// but it must not promote, publish, mutate production state, or synthesize a
// run-acceptance record.
type BaseStagingSmokeEvidence struct {
	StagingReadinessRef          string `json:"staging_readiness_ref"`
	ProductPathProbeRef          string `json:"product_path_probe_ref"`
	ProductPathURL               string `json:"product_path_url"`
	ExpectedBuildIdentity        string `json:"expected_build_identity"`
	ObservedBuildIdentity        string `json:"observed_build_identity"`
	ExpectedRouteIdentity        string `json:"expected_route_identity"`
	ObservedRouteIdentity        string `json:"observed_route_identity"`
	HealthStatus                 string `json:"health_status"`
	RollbackPlanRef              string `json:"rollback_plan_ref"`
	ProductPathObserved          bool   `json:"product_path_observed"`
	AuthenticatedProductPath     bool   `json:"authenticated_product_path"`
	ManualSuccessSeeded          bool   `json:"manual_success_seeded"`
	NoPromotionMutation          bool   `json:"no_promotion_mutation"`
	NoPackagePublicationMutation bool   `json:"no_package_publication_mutation"`
	NoRunAcceptanceMutation      bool   `json:"no_run_acceptance_mutation"`
	NoProductionMutation         bool   `json:"no_production_mutation"`
	PromotionClaimed             bool   `json:"promotion_claimed"`
	PackagePublicationClaimed    bool   `json:"package_publication_claimed"`
	RunAcceptanceRecordTouched   bool   `json:"run_acceptance_record_touched"`
	FullSubstrateClaimed         bool   `json:"full_substrate_claimed"`
	CompletionClaimed            bool   `json:"completion_claimed"`
}

// BaseStagingSmokeEvidenceContract records a passed product-path staging smoke
// observation and the build/route identity that was checked. It is still not
// promotion, package publication, run acceptance, full substrate independence,
// or mission completion.
type BaseStagingSmokeEvidenceContract struct {
	Kind                            string          `json:"kind"`
	Version                         ComputerVersion `json:"version"`
	Boundary                        string          `json:"boundary"`
	Scope                           string          `json:"scope"`
	TypedArtifactProgramRef         string          `json:"typed_artifact_program_ref"`
	StagingReadinessRef             string          `json:"staging_readiness_ref"`
	ProductPathProbeRef             string          `json:"product_path_probe_ref"`
	ProductPathURL                  string          `json:"product_path_url"`
	BuildIdentity                   string          `json:"build_identity"`
	RouteIdentity                   string          `json:"route_identity"`
	HealthStatus                    string          `json:"health_status"`
	RollbackPlanRef                 string          `json:"rollback_plan_ref"`
	ProductPathObserved             bool            `json:"product_path_observed"`
	AuthenticatedProductPath        bool            `json:"authenticated_product_path"`
	BuildIdentityMatched            bool            `json:"build_identity_matched"`
	RouteIdentityMatched            bool            `json:"route_identity_matched"`
	StagingSmokePassed              bool            `json:"staging_smoke_passed"`
	PromotionProofRequired          bool            `json:"promotion_proof_required"`
	PackagePublicationProofRequired bool            `json:"package_publication_proof_required"`
	RunAcceptanceProofRequired      bool            `json:"run_acceptance_proof_required"`
	FullSubstrateProofRequired      bool            `json:"full_substrate_proof_required"`
	NoPromotionMutation             bool            `json:"no_promotion_mutation"`
	NoPackagePublicationMutation    bool            `json:"no_package_publication_mutation"`
	NoRunAcceptanceMutation         bool            `json:"no_run_acceptance_mutation"`
	NoProductionMutation            bool            `json:"no_production_mutation"`
	PromotionClaimed                bool            `json:"promotion_claimed"`
	PackagePublicationClaimed       bool            `json:"package_publication_claimed"`
	RunAcceptanceRecordTouched      bool            `json:"run_acceptance_record_touched"`
	FullSubstrateClaimed            bool            `json:"full_substrate_claimed"`
	CompletionClaimed               bool            `json:"completion_claimed"`
}

// BuildBaseStagingSmokeEvidenceContract records product-path staging smoke
// evidence after staging readiness. It rejects internal/test-only bypasses and
// manual success seeding.
func BuildBaseStagingSmokeEvidenceContract(readiness BaseStagingReadinessContract, evidence BaseStagingSmokeEvidence) (BaseStagingSmokeEvidenceContract, error) {
	if err := validateBaseStagingSmokeReadiness(readiness); err != nil {
		return BaseStagingSmokeEvidenceContract{}, err
	}
	if err := validateBaseStagingSmokeEvidence(evidence); err != nil {
		return BaseStagingSmokeEvidenceContract{}, err
	}

	return BaseStagingSmokeEvidenceContract{
		Kind:                            BaseStagingSmokeEvidenceContractKind,
		Version:                         readiness.Version,
		Boundary:                        BaseStagingSmokeEvidenceBoundary,
		Scope:                           BaseStagingSmokeEvidenceScope,
		TypedArtifactProgramRef:         string(readiness.Version.ArtifactProgramRef),
		StagingReadinessRef:             strings.TrimSpace(evidence.StagingReadinessRef),
		ProductPathProbeRef:             strings.TrimSpace(evidence.ProductPathProbeRef),
		ProductPathURL:                  strings.TrimSpace(evidence.ProductPathURL),
		BuildIdentity:                   strings.TrimSpace(evidence.ObservedBuildIdentity),
		RouteIdentity:                   strings.TrimSpace(evidence.ObservedRouteIdentity),
		HealthStatus:                    BaseStagingSmokeHealthPassed,
		RollbackPlanRef:                 strings.TrimSpace(evidence.RollbackPlanRef),
		ProductPathObserved:             true,
		AuthenticatedProductPath:        true,
		BuildIdentityMatched:            true,
		RouteIdentityMatched:            true,
		StagingSmokePassed:              true,
		PromotionProofRequired:          true,
		PackagePublicationProofRequired: true,
		RunAcceptanceProofRequired:      true,
		FullSubstrateProofRequired:      true,
		NoPromotionMutation:             true,
		NoPackagePublicationMutation:    true,
		NoRunAcceptanceMutation:         true,
		NoProductionMutation:            true,
	}, nil
}

func validateBaseStagingSmokeReadiness(readiness BaseStagingReadinessContract) error {
	if readiness.Kind != BaseStagingReadinessContractKind {
		return fmt.Errorf("base staging smoke evidence: readiness kind is %q", readiness.Kind)
	}
	if readiness.Boundary != BaseStagingReadinessBoundary {
		return fmt.Errorf("base staging smoke evidence: readiness boundary is %q", readiness.Boundary)
	}
	if readiness.Scope != BaseStagingReadinessScope {
		return fmt.Errorf("base staging smoke evidence: readiness scope is %q", readiness.Scope)
	}
	if !readiness.Version.Valid() {
		return fmt.Errorf("base staging smoke evidence: readiness version is invalid")
	}
	if !readiness.Version.ArtifactProgramRef.Valid() || ArtifactProgramRef(readiness.TypedArtifactProgramRef) != readiness.Version.ArtifactProgramRef {
		return fmt.Errorf("base staging smoke evidence: readiness typed artifact-program ref is invalid")
	}
	if strings.TrimSpace(readiness.RuntimeEquivalenceRetryRef) == "" || strings.TrimSpace(readiness.StagingSmokePlanRef) == "" || strings.TrimSpace(readiness.BuildIdentityExpectationRef) == "" || strings.TrimSpace(readiness.RollbackPlanRef) == "" {
		return fmt.Errorf("base staging smoke evidence: readiness refs are required")
	}
	if !readiness.RuntimeEquivalenceAccepted || !readiness.StagingSmokeMayRun {
		return fmt.Errorf("base staging smoke evidence: readiness must authorize staging smoke from accepted runtime equivalence")
	}
	if !readiness.DeploymentHealthProofRequired || !readiness.RouteIdentityProofRequired || !readiness.PromotionProofRequired || !readiness.PackagePublicationProofRequired || !readiness.RunAcceptanceProofRequired {
		return fmt.Errorf("base staging smoke evidence: readiness must preserve downstream proof requirements")
	}
	if !readiness.NoDeploymentMutation || !readiness.NoRouteRegistrationMutation || !readiness.NoRunAcceptanceMutation || !readiness.NoProductionMutation {
		return fmt.Errorf("base staging smoke evidence: readiness must prove no deployment, route, run-acceptance, or production mutation")
	}
	if readiness.DeploymentExecuted || readiness.StagingHealthClaimed || readiness.DeployedRouteIdentityClaimed || readiness.RuntimeBehaviorChanged || readiness.DeployedRouteRegistered || readiness.ProductionAuthTouched || readiness.PromotionClaimed || readiness.VMLifecycleTouched || readiness.FirecrackerBootClaimed || readiness.RunAcceptanceRecordTouched || readiness.PackagePublicationClaimed || readiness.FullSubstrateClaimed || readiness.CompletionClaimed {
		return fmt.Errorf("base staging smoke evidence: readiness carries protected-surface or completion claims")
	}
	return nil
}

func validateBaseStagingSmokeEvidence(evidence BaseStagingSmokeEvidence) error {
	if strings.TrimSpace(evidence.StagingReadinessRef) == "" || strings.TrimSpace(evidence.ProductPathProbeRef) == "" || strings.TrimSpace(evidence.ProductPathURL) == "" || strings.TrimSpace(evidence.RollbackPlanRef) == "" {
		return fmt.Errorf("base staging smoke evidence: evidence refs are required")
	}
	if !baseStagingSmokeProductPathAllowed(evidence.ProductPathURL) {
		return fmt.Errorf("base staging smoke evidence: product path URL must not use internal or test-only routes")
	}
	if strings.TrimSpace(evidence.ExpectedBuildIdentity) == "" || strings.TrimSpace(evidence.ObservedBuildIdentity) == "" || strings.TrimSpace(evidence.ExpectedBuildIdentity) != strings.TrimSpace(evidence.ObservedBuildIdentity) {
		return fmt.Errorf("base staging smoke evidence: build identity must match")
	}
	if strings.TrimSpace(evidence.ExpectedRouteIdentity) == "" || strings.TrimSpace(evidence.ObservedRouteIdentity) == "" || strings.TrimSpace(evidence.ExpectedRouteIdentity) != strings.TrimSpace(evidence.ObservedRouteIdentity) {
		return fmt.Errorf("base staging smoke evidence: route identity must match")
	}
	if strings.TrimSpace(evidence.HealthStatus) != BaseStagingSmokeHealthPassed {
		return fmt.Errorf("base staging smoke evidence: health status must be passed")
	}
	if !evidence.ProductPathObserved || !evidence.AuthenticatedProductPath {
		return fmt.Errorf("base staging smoke evidence: product path must be observed through authenticated product/control evidence")
	}
	if evidence.ManualSuccessSeeded {
		return fmt.Errorf("base staging smoke evidence: manual success seeding is forbidden")
	}
	if !evidence.NoPromotionMutation || !evidence.NoPackagePublicationMutation || !evidence.NoRunAcceptanceMutation || !evidence.NoProductionMutation {
		return fmt.Errorf("base staging smoke evidence: evidence must prove no promotion, package publication, run-acceptance, or production mutation")
	}
	if evidence.PromotionClaimed || evidence.PackagePublicationClaimed || evidence.RunAcceptanceRecordTouched || evidence.FullSubstrateClaimed || evidence.CompletionClaimed {
		return fmt.Errorf("base staging smoke evidence: evidence carries downstream or completion claims")
	}
	return nil
}

func baseStagingSmokeProductPathAllowed(rawURL string) bool {
	url := strings.TrimSpace(rawURL)
	if url == "" {
		return false
	}
	for _, forbidden := range []string{"/internal/", "/api/agent/", "/api/prompts", "/api/test/"} {
		if strings.Contains(url, forbidden) {
			return false
		}
	}
	return strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "/api/")
}
