package computerversion

import (
	"strings"
	"testing"
)

func TestBuildBaseStagingSmokeEvidenceContractRecordsProductPathEvidence(t *testing.T) {
	readiness, evidence := baseStagingSmokeEvidenceContractInputs(t)

	contract, err := BuildBaseStagingSmokeEvidenceContract(readiness, evidence)
	if err != nil {
		t.Fatalf("BuildBaseStagingSmokeEvidenceContract(): %v", err)
	}

	if contract.Kind != BaseStagingSmokeEvidenceContractKind {
		t.Fatalf("kind = %q, want %q", contract.Kind, BaseStagingSmokeEvidenceContractKind)
	}
	if contract.Version != readiness.Version || ArtifactProgramRef(contract.TypedArtifactProgramRef) != readiness.Version.ArtifactProgramRef {
		t.Fatalf("version/ref = %#v/%q, want readiness version %#v and artifact program %q", contract.Version, contract.TypedArtifactProgramRef, readiness.Version, readiness.Version.ArtifactProgramRef)
	}
	if contract.Boundary != BaseStagingSmokeEvidenceBoundary {
		t.Fatalf("boundary = %q, want %q", contract.Boundary, BaseStagingSmokeEvidenceBoundary)
	}
	if contract.Scope != BaseStagingSmokeEvidenceScope {
		t.Fatalf("scope = %q, want %q", contract.Scope, BaseStagingSmokeEvidenceScope)
	}
	if contract.StagingReadinessRef != strings.TrimSpace(evidence.StagingReadinessRef) || contract.ProductPathProbeRef != strings.TrimSpace(evidence.ProductPathProbeRef) || contract.RollbackPlanRef != strings.TrimSpace(evidence.RollbackPlanRef) {
		t.Fatalf("evidence refs = readiness %q probe %q rollback %q, want trimmed refs from %#v", contract.StagingReadinessRef, contract.ProductPathProbeRef, contract.RollbackPlanRef, evidence)
	}
	if contract.ProductPathURL != strings.TrimSpace(evidence.ProductPathURL) {
		t.Fatalf("product path URL = %q, want trimmed %q", contract.ProductPathURL, strings.TrimSpace(evidence.ProductPathURL))
	}
	if contract.BuildIdentity != strings.TrimSpace(evidence.ObservedBuildIdentity) || contract.RouteIdentity != strings.TrimSpace(evidence.ObservedRouteIdentity) {
		t.Fatalf("identity = build %q route %q, want observed build %q route %q", contract.BuildIdentity, contract.RouteIdentity, strings.TrimSpace(evidence.ObservedBuildIdentity), strings.TrimSpace(evidence.ObservedRouteIdentity))
	}
	if contract.HealthStatus != BaseStagingSmokeHealthPassed {
		t.Fatalf("health status = %q, want %q", contract.HealthStatus, BaseStagingSmokeHealthPassed)
	}
	if !contract.StagingSmokePassed || !contract.BuildIdentityMatched || !contract.RouteIdentityMatched || !contract.ProductPathObserved || !contract.AuthenticatedProductPath {
		t.Fatalf("staging smoke must record passed health, matched build/route identity, and authenticated product-path observation: %#v", contract)
	}
	if !contract.PromotionProofRequired || !contract.PackagePublicationProofRequired || !contract.RunAcceptanceProofRequired || !contract.FullSubstrateProofRequired {
		t.Fatalf("staging smoke evidence must preserve promotion, package publication, run-acceptance, and full-substrate proof requirements: %#v", contract)
	}
	if !contract.NoPromotionMutation || !contract.NoPackagePublicationMutation || !contract.NoRunAcceptanceMutation || !contract.NoProductionMutation {
		t.Fatalf("staging smoke evidence must carry no-promotion/no-package/no-run-acceptance/no-production mutation flags: %#v", contract)
	}
	if contract.PromotionClaimed || contract.PackagePublicationClaimed || contract.RunAcceptanceRecordTouched || contract.FullSubstrateClaimed || contract.CompletionClaimed {
		t.Fatalf("staging smoke evidence must not claim promotion, package publication, run acceptance, full substrate, or completion: %#v", contract)
	}
}

func TestBuildBaseStagingSmokeEvidenceContractRejectsInvalidInputs(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*BaseStagingReadinessContract, *BaseStagingSmokeEvidence)
		wantErr string
	}{
		{
			name: "readiness wrong kind",
			mutate: func(readiness *BaseStagingReadinessContract, _ *BaseStagingSmokeEvidence) {
				readiness.Kind = BaseStagingSmokeEvidenceContractKind
			},
			wantErr: "readiness kind",
		},
		{
			name: "readiness boundary drift",
			mutate: func(readiness *BaseStagingReadinessContract, _ *BaseStagingSmokeEvidence) {
				readiness.Boundary = BaseStagingSmokeEvidenceBoundary
			},
			wantErr: "readiness boundary",
		},
		{
			name: "readiness scope drift",
			mutate: func(readiness *BaseStagingReadinessContract, _ *BaseStagingSmokeEvidence) {
				readiness.Scope = BaseStagingSmokeEvidenceScope
			},
			wantErr: "readiness scope",
		},
		{
			name: "readiness missing accepted runtime equivalence",
			mutate: func(readiness *BaseStagingReadinessContract, _ *BaseStagingSmokeEvidence) {
				readiness.RuntimeEquivalenceAccepted = false
			},
			wantErr: "readiness must authorize staging smoke from accepted runtime equivalence",
		},
		{
			name: "readiness does not authorize staging smoke",
			mutate: func(readiness *BaseStagingReadinessContract, _ *BaseStagingSmokeEvidence) {
				readiness.StagingSmokeMayRun = false
			},
			wantErr: "readiness must authorize staging smoke from accepted runtime equivalence",
		},
		{
			name: "readiness missing deployment health proof requirement",
			mutate: func(readiness *BaseStagingReadinessContract, _ *BaseStagingSmokeEvidence) {
				readiness.DeploymentHealthProofRequired = false
			},
			wantErr: "readiness must preserve downstream proof requirements",
		},
		{
			name: "readiness missing route identity proof requirement",
			mutate: func(readiness *BaseStagingReadinessContract, _ *BaseStagingSmokeEvidence) {
				readiness.RouteIdentityProofRequired = false
			},
			wantErr: "readiness must preserve downstream proof requirements",
		},
		{
			name: "readiness claims deployed health",
			mutate: func(readiness *BaseStagingReadinessContract, _ *BaseStagingSmokeEvidence) {
				readiness.StagingHealthClaimed = true
			},
			wantErr: "readiness carries protected-surface or completion claims",
		},
		{
			name: "readiness claims deployed route identity",
			mutate: func(readiness *BaseStagingReadinessContract, _ *BaseStagingSmokeEvidence) {
				readiness.DeployedRouteIdentityClaimed = true
			},
			wantErr: "readiness carries protected-surface or completion claims",
		},
		{
			name: "readiness claims production auth protected surface",
			mutate: func(readiness *BaseStagingReadinessContract, _ *BaseStagingSmokeEvidence) {
				readiness.ProductionAuthTouched = true
			},
			wantErr: "readiness carries protected-surface or completion claims",
		},
		{
			name: "missing staging readiness evidence ref",
			mutate: func(_ *BaseStagingReadinessContract, evidence *BaseStagingSmokeEvidence) {
				evidence.StagingReadinessRef = "  "
			},
			wantErr: "evidence refs are required",
		},
		{
			name: "missing product path probe ref",
			mutate: func(_ *BaseStagingReadinessContract, evidence *BaseStagingSmokeEvidence) {
				evidence.ProductPathProbeRef = "\t"
			},
			wantErr: "evidence refs are required",
		},
		{
			name: "missing product path URL",
			mutate: func(_ *BaseStagingReadinessContract, evidence *BaseStagingSmokeEvidence) {
				evidence.ProductPathURL = ""
			},
			wantErr: "evidence refs are required",
		},
		{
			name: "missing rollback plan ref",
			mutate: func(_ *BaseStagingReadinessContract, evidence *BaseStagingSmokeEvidence) {
				evidence.RollbackPlanRef = "  "
			},
			wantErr: "evidence refs are required",
		},
		{
			name: "forbidden internal URL",
			mutate: func(_ *BaseStagingReadinessContract, evidence *BaseStagingSmokeEvidence) {
				evidence.ProductPathURL = "https://staging.example.test/internal/health"
			},
			wantErr: "product path URL must not use internal or test-only routes",
		},
		{
			name: "forbidden agent bypass URL",
			mutate: func(_ *BaseStagingReadinessContract, evidence *BaseStagingSmokeEvidence) {
				evidence.ProductPathURL = "/api/agent/staging/smoke"
			},
			wantErr: "product path URL must not use internal or test-only routes",
		},
		{
			name: "forbidden test-only URL",
			mutate: func(_ *BaseStagingReadinessContract, evidence *BaseStagingSmokeEvidence) {
				evidence.ProductPathURL = "/api/test/staging/smoke"
			},
			wantErr: "product path URL must not use internal or test-only routes",
		},
		{
			name: "build identity mismatch",
			mutate: func(_ *BaseStagingReadinessContract, evidence *BaseStagingSmokeEvidence) {
				evidence.ObservedBuildIdentity = "build:base@sha256:different"
			},
			wantErr: "build identity must match",
		},
		{
			name: "route identity mismatch",
			mutate: func(_ *BaseStagingReadinessContract, evidence *BaseStagingSmokeEvidence) {
				evidence.ObservedRouteIdentity = "route:staging-product-path:different"
			},
			wantErr: "route identity must match",
		},
		{
			name: "failed health",
			mutate: func(_ *BaseStagingReadinessContract, evidence *BaseStagingSmokeEvidence) {
				evidence.HealthStatus = "failed"
			},
			wantErr: "health status must be passed",
		},
		{
			name: "unobserved product path",
			mutate: func(_ *BaseStagingReadinessContract, evidence *BaseStagingSmokeEvidence) {
				evidence.ProductPathObserved = false
			},
			wantErr: "product path must be observed through authenticated product/control evidence",
		},
		{
			name: "unauthenticated product path",
			mutate: func(_ *BaseStagingReadinessContract, evidence *BaseStagingSmokeEvidence) {
				evidence.AuthenticatedProductPath = false
			},
			wantErr: "product path must be observed through authenticated product/control evidence",
		},
		{
			name: "manual success seeding",
			mutate: func(_ *BaseStagingReadinessContract, evidence *BaseStagingSmokeEvidence) {
				evidence.ManualSuccessSeeded = true
			},
			wantErr: "manual success seeding is forbidden",
		},
		{
			name: "missing no promotion mutation flag",
			mutate: func(_ *BaseStagingReadinessContract, evidence *BaseStagingSmokeEvidence) {
				evidence.NoPromotionMutation = false
			},
			wantErr: "evidence must prove no promotion, package publication, run-acceptance, or production mutation",
		},
		{
			name: "missing no package publication mutation flag",
			mutate: func(_ *BaseStagingReadinessContract, evidence *BaseStagingSmokeEvidence) {
				evidence.NoPackagePublicationMutation = false
			},
			wantErr: "evidence must prove no promotion, package publication, run-acceptance, or production mutation",
		},
		{
			name: "missing no run acceptance mutation flag",
			mutate: func(_ *BaseStagingReadinessContract, evidence *BaseStagingSmokeEvidence) {
				evidence.NoRunAcceptanceMutation = false
			},
			wantErr: "evidence must prove no promotion, package publication, run-acceptance, or production mutation",
		},
		{
			name: "missing no production mutation flag",
			mutate: func(_ *BaseStagingReadinessContract, evidence *BaseStagingSmokeEvidence) {
				evidence.NoProductionMutation = false
			},
			wantErr: "evidence must prove no promotion, package publication, run-acceptance, or production mutation",
		},
		{
			name: "promotion claimed",
			mutate: func(_ *BaseStagingReadinessContract, evidence *BaseStagingSmokeEvidence) {
				evidence.PromotionClaimed = true
			},
			wantErr: "evidence carries downstream or completion claims",
		},
		{
			name: "package publication claimed",
			mutate: func(_ *BaseStagingReadinessContract, evidence *BaseStagingSmokeEvidence) {
				evidence.PackagePublicationClaimed = true
			},
			wantErr: "evidence carries downstream or completion claims",
		},
		{
			name: "run acceptance record touched",
			mutate: func(_ *BaseStagingReadinessContract, evidence *BaseStagingSmokeEvidence) {
				evidence.RunAcceptanceRecordTouched = true
			},
			wantErr: "evidence carries downstream or completion claims",
		},
		{
			name: "full substrate claimed",
			mutate: func(_ *BaseStagingReadinessContract, evidence *BaseStagingSmokeEvidence) {
				evidence.FullSubstrateClaimed = true
			},
			wantErr: "evidence carries downstream or completion claims",
		},
		{
			name: "completion claimed",
			mutate: func(_ *BaseStagingReadinessContract, evidence *BaseStagingSmokeEvidence) {
				evidence.CompletionClaimed = true
			},
			wantErr: "evidence carries downstream or completion claims",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			readiness, evidence := baseStagingSmokeEvidenceContractInputs(t)
			tc.mutate(&readiness, &evidence)

			contract, err := BuildBaseStagingSmokeEvidenceContract(readiness, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildBaseStagingSmokeEvidenceContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func baseStagingSmokeEvidenceContractInputs(t *testing.T) (BaseStagingReadinessContract, BaseStagingSmokeEvidence) {
	t.Helper()

	retry, readinessEvidence := baseStagingReadinessContractInputs(t)
	readiness, err := BuildBaseStagingReadinessContract(retry, readinessEvidence)
	if err != nil {
		t.Fatalf("BuildBaseStagingReadinessContract(): %v", err)
	}

	buildIdentity := "build:base-artifact-program@sha256:105"
	routeIdentity := "route:staging-product-path:/computers/base"
	evidence := BaseStagingSmokeEvidence{
		StagingReadinessRef:          " staging-readiness:base-pass-104 ",
		ProductPathProbeRef:          " staging-product-path-probe:base-pass-105 ",
		ProductPathURL:               " https://staging.example.test/computers/base ",
		ExpectedBuildIdentity:        " " + buildIdentity + " ",
		ObservedBuildIdentity:        buildIdentity,
		ExpectedRouteIdentity:        " " + routeIdentity + " ",
		ObservedRouteIdentity:        routeIdentity,
		HealthStatus:                 " " + BaseStagingSmokeHealthPassed + " ",
		RollbackPlanRef:              " " + readiness.RollbackPlanRef + " ",
		ProductPathObserved:          true,
		AuthenticatedProductPath:     true,
		ManualSuccessSeeded:          false,
		NoPromotionMutation:          true,
		NoPackagePublicationMutation: true,
		NoRunAcceptanceMutation:      true,
		NoProductionMutation:         true,
		PromotionClaimed:             false,
		PackagePublicationClaimed:    false,
		RunAcceptanceRecordTouched:   false,
		FullSubstrateClaimed:         false,
		CompletionClaimed:            false,
	}
	return readiness, evidence
}
