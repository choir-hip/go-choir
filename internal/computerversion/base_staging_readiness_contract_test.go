package computerversion

import (
	"strings"
	"testing"
)

func TestBuildBaseStagingReadinessContractAuthorizesOnlyStagingSmokeProbe(t *testing.T) {
	retry, evidence := baseStagingReadinessContractInputs(t)

	contract, err := BuildBaseStagingReadinessContract(retry, evidence)
	if err != nil {
		t.Fatalf("BuildBaseStagingReadinessContract(): %v", err)
	}

	if contract.Kind != BaseStagingReadinessContractKind {
		t.Fatalf("kind = %q, want %q", contract.Kind, BaseStagingReadinessContractKind)
	}
	if contract.Version != retry.Version || ArtifactProgramRef(contract.TypedArtifactProgramRef) != retry.Version.ArtifactProgramRef {
		t.Fatalf("version/ref = %#v/%q, want retry version %#v and artifact program %q", contract.Version, contract.TypedArtifactProgramRef, retry.Version, retry.Version.ArtifactProgramRef)
	}
	if contract.Boundary != BaseStagingReadinessBoundary {
		t.Fatalf("boundary = %q, want %q", contract.Boundary, BaseStagingReadinessBoundary)
	}
	if contract.Scope != BaseStagingReadinessScope {
		t.Fatalf("scope = %q, want %q", contract.Scope, BaseStagingReadinessScope)
	}
	if contract.RuntimeEquivalenceRetryRef != strings.TrimSpace(evidence.RuntimeEquivalenceRetryRef) || contract.RuntimeEquivalenceRetryRef != retry.RuntimeEquivalenceRetryRef {
		t.Fatalf("runtime equivalence retry ref = %q, want trimmed evidence ref matching retry %q", contract.RuntimeEquivalenceRetryRef, retry.RuntimeEquivalenceRetryRef)
	}
	if contract.SourceProvenanceReadinessRef != retry.SourceProvenanceReadinessRef {
		t.Fatalf("source provenance ref = %q, want retry ref %q", contract.SourceProvenanceReadinessRef, retry.SourceProvenanceReadinessRef)
	}
	if contract.StagingSmokePlanRef != strings.TrimSpace(evidence.StagingSmokePlanRef) || contract.BuildIdentityExpectationRef != strings.TrimSpace(evidence.BuildIdentityExpectationRef) || contract.RollbackPlanRef != strings.TrimSpace(evidence.RollbackPlanRef) {
		t.Fatalf("evidence refs = smoke %q build %q rollback %q, want trimmed refs from %#v", contract.StagingSmokePlanRef, contract.BuildIdentityExpectationRef, contract.RollbackPlanRef, evidence)
	}
	assertObservationBundleKinds(t, contract.RequiredObservations, []ObservationKind{ObservationBlobSet, ObservationFileManifest})
	if !contract.RuntimeEquivalenceAccepted || !contract.StagingSmokeMayRun {
		t.Fatalf("staging readiness = runtime accepted %v smoke may run %v, want both true", contract.RuntimeEquivalenceAccepted, contract.StagingSmokeMayRun)
	}
	if !contract.DeploymentHealthProofRequired || !contract.RouteIdentityProofRequired || !contract.PromotionProofRequired || !contract.PackagePublicationProofRequired || !contract.RunAcceptanceProofRequired {
		t.Fatalf("staging readiness must preserve deployment health, route identity, promotion, package publication, and run-acceptance proof requirements: %#v", contract)
	}
	if !contract.NoDeploymentMutation || !contract.NoRouteRegistrationMutation || !contract.NoRunAcceptanceMutation || !contract.NoProductionMutation {
		t.Fatalf("staging readiness must carry no-deployment/no-route/no-run-acceptance/no-production mutation flags: %#v", contract)
	}
	if contract.DeploymentExecuted || contract.StagingHealthClaimed || contract.DeployedRouteIdentityClaimed || contract.RuntimeBehaviorChanged || contract.DeployedRouteRegistered || contract.ProductionAuthTouched || contract.PromotionClaimed || contract.VMLifecycleTouched || contract.FirecrackerBootClaimed || contract.RunAcceptanceRecordTouched || contract.PackagePublicationClaimed || contract.FullSubstrateClaimed || contract.CompletionClaimed {
		t.Fatalf("staging readiness must not claim deployment execution, staging health, route identity, protected surfaces, downstream proofs, full substrate, run acceptance, or completion: %#v", contract)
	}
}

func TestBuildBaseStagingReadinessContractRejectsInvalidInputs(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*BaseRuntimeEquivalenceRetryContract, *BaseStagingReadinessEvidence)
		wantErr string
	}{
		{
			name: "retry wrong kind",
			mutate: func(retry *BaseRuntimeEquivalenceRetryContract, _ *BaseStagingReadinessEvidence) {
				retry.Kind = BaseStagingReadinessContractKind
			},
			wantErr: "retry kind",
		},
		{
			name: "retry boundary drift",
			mutate: func(retry *BaseRuntimeEquivalenceRetryContract, _ *BaseStagingReadinessEvidence) {
				retry.Boundary = BaseStagingReadinessBoundary
			},
			wantErr: "retry boundary",
		},
		{
			name: "retry scope drift",
			mutate: func(retry *BaseRuntimeEquivalenceRetryContract, _ *BaseStagingReadinessEvidence) {
				retry.Scope = BaseStagingReadinessScope
			},
			wantErr: "retry scope",
		},
		{
			name: "retry missing runtime equivalence status",
			mutate: func(retry *BaseRuntimeEquivalenceRetryContract, _ *BaseStagingReadinessEvidence) {
				retry.RuntimeEquivalenceStatus = EquivalenceNarrowed
			},
			wantErr: "retry must have accepted runtime equivalence",
		},
		{
			name: "retry missing runtime equivalence claim",
			mutate: func(retry *BaseRuntimeEquivalenceRetryContract, _ *BaseStagingReadinessEvidence) {
				retry.RuntimeEquivalenceClaimed = false
			},
			wantErr: "retry must have accepted runtime equivalence",
		},
		{
			name: "retry missing file manifest scope",
			mutate: func(retry *BaseRuntimeEquivalenceRetryContract, _ *BaseStagingReadinessEvidence) {
				retry.RequiredObservations = []ObservationKind{ObservationBlobSet}
			},
			wantErr: "retry must require typed file/blob observations only",
		},
		{
			name: "retry missing blob set scope",
			mutate: func(retry *BaseRuntimeEquivalenceRetryContract, _ *BaseStagingReadinessEvidence) {
				retry.RequiredObservations = []ObservationKind{ObservationFileManifest}
			},
			wantErr: "retry must require typed file/blob observations only",
		},
		{
			name: "retry relies on vm state manifest",
			mutate: func(retry *BaseRuntimeEquivalenceRetryContract, _ *BaseStagingReadinessEvidence) {
				retry.RequiredObservations = append(retry.RequiredObservations, ObservationVMStateManifest)
			},
			wantErr: "retry must require typed file/blob observations only",
		},
		{
			name: "retry missing source provenance ref",
			mutate: func(retry *BaseRuntimeEquivalenceRetryContract, _ *BaseStagingReadinessEvidence) {
				retry.SourceProvenanceReadinessRef = "\t"
			},
			wantErr: "retry refs are required",
		},
		{
			name: "retry missing runtime equivalence retry ref",
			mutate: func(retry *BaseRuntimeEquivalenceRetryContract, _ *BaseStagingReadinessEvidence) {
				retry.RuntimeEquivalenceRetryRef = "  "
			},
			wantErr: "retry refs are required",
		},
		{
			name: "retry missing staging proof requirement",
			mutate: func(retry *BaseRuntimeEquivalenceRetryContract, _ *BaseStagingReadinessEvidence) {
				retry.StagingProofRequired = false
			},
			wantErr: "retry must preserve downstream proof requirements",
		},
		{
			name: "retry missing promotion proof requirement",
			mutate: func(retry *BaseRuntimeEquivalenceRetryContract, _ *BaseStagingReadinessEvidence) {
				retry.PromotionProofRequired = false
			},
			wantErr: "retry must preserve downstream proof requirements",
		},
		{
			name: "retry missing package publication proof requirement",
			mutate: func(retry *BaseRuntimeEquivalenceRetryContract, _ *BaseStagingReadinessEvidence) {
				retry.PackagePublicationProofRequired = false
			},
			wantErr: "retry must preserve downstream proof requirements",
		},
		{
			name: "retry missing run acceptance proof requirement",
			mutate: func(retry *BaseRuntimeEquivalenceRetryContract, _ *BaseStagingReadinessEvidence) {
				retry.RunAcceptanceProofRequired = false
			},
			wantErr: "retry must preserve downstream proof requirements",
		},
		{
			name: "retry missing no VM lifecycle mutation proof",
			mutate: func(retry *BaseRuntimeEquivalenceRetryContract, _ *BaseStagingReadinessEvidence) {
				retry.NoVMLifecycleMutation = false
			},
			wantErr: "retry must prove no VM lifecycle mutation, production mutation, or opaque data.img dependency",
		},
		{
			name: "retry missing no production mutation proof",
			mutate: func(retry *BaseRuntimeEquivalenceRetryContract, _ *BaseStagingReadinessEvidence) {
				retry.NoProductionMutation = false
			},
			wantErr: "retry must prove no VM lifecycle mutation, production mutation, or opaque data.img dependency",
		},
		{
			name: "retry missing no opaque data image dependency proof",
			mutate: func(retry *BaseRuntimeEquivalenceRetryContract, _ *BaseStagingReadinessEvidence) {
				retry.NoOpaqueDataImageDependency = false
			},
			wantErr: "retry must prove no VM lifecycle mutation, production mutation, or opaque data.img dependency",
		},
		{
			name: "retry runtime behavior changed",
			mutate: func(retry *BaseRuntimeEquivalenceRetryContract, _ *BaseStagingReadinessEvidence) {
				retry.RuntimeBehaviorChanged = true
			},
			wantErr: "retry carries protected-surface or completion claims",
		},
		{
			name: "retry deployed route registered",
			mutate: func(retry *BaseRuntimeEquivalenceRetryContract, _ *BaseStagingReadinessEvidence) {
				retry.DeployedRouteRegistered = true
			},
			wantErr: "retry carries protected-surface or completion claims",
		},
		{
			name: "retry production auth touched",
			mutate: func(retry *BaseRuntimeEquivalenceRetryContract, _ *BaseStagingReadinessEvidence) {
				retry.ProductionAuthTouched = true
			},
			wantErr: "retry carries protected-surface or completion claims",
		},
		{
			name: "retry staging claimed",
			mutate: func(retry *BaseRuntimeEquivalenceRetryContract, _ *BaseStagingReadinessEvidence) {
				retry.StagingClaimed = true
			},
			wantErr: "retry carries protected-surface or completion claims",
		},
		{
			name: "retry promotion claimed",
			mutate: func(retry *BaseRuntimeEquivalenceRetryContract, _ *BaseStagingReadinessEvidence) {
				retry.PromotionClaimed = true
			},
			wantErr: "retry carries protected-surface or completion claims",
		},
		{
			name: "retry VM lifecycle touched",
			mutate: func(retry *BaseRuntimeEquivalenceRetryContract, _ *BaseStagingReadinessEvidence) {
				retry.VMLifecycleTouched = true
			},
			wantErr: "retry carries protected-surface or completion claims",
		},
		{
			name: "retry firecracker boot claimed",
			mutate: func(retry *BaseRuntimeEquivalenceRetryContract, _ *BaseStagingReadinessEvidence) {
				retry.FirecrackerBootClaimed = true
			},
			wantErr: "retry carries protected-surface or completion claims",
		},
		{
			name: "retry run acceptance record touched",
			mutate: func(retry *BaseRuntimeEquivalenceRetryContract, _ *BaseStagingReadinessEvidence) {
				retry.RunAcceptanceRecordTouched = true
			},
			wantErr: "retry carries protected-surface or completion claims",
		},
		{
			name: "retry package publication claimed",
			mutate: func(retry *BaseRuntimeEquivalenceRetryContract, _ *BaseStagingReadinessEvidence) {
				retry.PackagePublicationClaimed = true
			},
			wantErr: "retry carries protected-surface or completion claims",
		},
		{
			name: "retry full substrate claimed",
			mutate: func(retry *BaseRuntimeEquivalenceRetryContract, _ *BaseStagingReadinessEvidence) {
				retry.FullSubstrateClaimed = true
			},
			wantErr: "retry carries protected-surface or completion claims",
		},
		{
			name: "retry completion claimed",
			mutate: func(retry *BaseRuntimeEquivalenceRetryContract, _ *BaseStagingReadinessEvidence) {
				retry.CompletionClaimed = true
			},
			wantErr: "retry carries protected-surface or completion claims",
		},
		{
			name: "missing runtime equivalence retry evidence ref",
			mutate: func(_ *BaseRuntimeEquivalenceRetryContract, evidence *BaseStagingReadinessEvidence) {
				evidence.RuntimeEquivalenceRetryRef = "  "
			},
			wantErr: "evidence refs are required",
		},
		{
			name: "missing staging smoke plan ref",
			mutate: func(_ *BaseRuntimeEquivalenceRetryContract, evidence *BaseStagingReadinessEvidence) {
				evidence.StagingSmokePlanRef = "\t"
			},
			wantErr: "evidence refs are required",
		},
		{
			name: "missing build identity expectation ref",
			mutate: func(_ *BaseRuntimeEquivalenceRetryContract, evidence *BaseStagingReadinessEvidence) {
				evidence.BuildIdentityExpectationRef = ""
			},
			wantErr: "evidence refs are required",
		},
		{
			name: "missing rollback plan ref",
			mutate: func(_ *BaseRuntimeEquivalenceRetryContract, evidence *BaseStagingReadinessEvidence) {
				evidence.RollbackPlanRef = "  "
			},
			wantErr: "evidence refs are required",
		},
		{
			name: "missing no deployment mutation flag",
			mutate: func(_ *BaseRuntimeEquivalenceRetryContract, evidence *BaseStagingReadinessEvidence) {
				evidence.NoDeploymentMutation = false
			},
			wantErr: "evidence must prove no deployment, route registration, run-acceptance, or production mutation",
		},
		{
			name: "missing no route registration mutation flag",
			mutate: func(_ *BaseRuntimeEquivalenceRetryContract, evidence *BaseStagingReadinessEvidence) {
				evidence.NoRouteRegistrationMutation = false
			},
			wantErr: "evidence must prove no deployment, route registration, run-acceptance, or production mutation",
		},
		{
			name: "missing no run acceptance mutation flag",
			mutate: func(_ *BaseRuntimeEquivalenceRetryContract, evidence *BaseStagingReadinessEvidence) {
				evidence.NoRunAcceptanceMutation = false
			},
			wantErr: "evidence must prove no deployment, route registration, run-acceptance, or production mutation",
		},
		{
			name: "missing no production mutation flag",
			mutate: func(_ *BaseRuntimeEquivalenceRetryContract, evidence *BaseStagingReadinessEvidence) {
				evidence.NoProductionMutation = false
			},
			wantErr: "evidence must prove no deployment, route registration, run-acceptance, or production mutation",
		},
		{
			name: "deployment executed",
			mutate: func(_ *BaseRuntimeEquivalenceRetryContract, evidence *BaseStagingReadinessEvidence) {
				evidence.DeploymentExecuted = true
			},
			wantErr: "evidence cannot claim deployment execution, staging health, or route identity",
		},
		{
			name: "staging health claimed",
			mutate: func(_ *BaseRuntimeEquivalenceRetryContract, evidence *BaseStagingReadinessEvidence) {
				evidence.StagingHealthClaimed = true
			},
			wantErr: "evidence cannot claim deployment execution, staging health, or route identity",
		},
		{
			name: "route identity claimed",
			mutate: func(_ *BaseRuntimeEquivalenceRetryContract, evidence *BaseStagingReadinessEvidence) {
				evidence.DeployedRouteIdentityClaimed = true
			},
			wantErr: "evidence cannot claim deployment execution, staging health, or route identity",
		},
		{
			name: "evidence runtime behavior changed",
			mutate: func(_ *BaseRuntimeEquivalenceRetryContract, evidence *BaseStagingReadinessEvidence) {
				evidence.RuntimeBehaviorChanged = true
			},
			wantErr: "evidence carries protected-surface or completion claims",
		},
		{
			name: "evidence deployed route registered",
			mutate: func(_ *BaseRuntimeEquivalenceRetryContract, evidence *BaseStagingReadinessEvidence) {
				evidence.DeployedRouteRegistered = true
			},
			wantErr: "evidence carries protected-surface or completion claims",
		},
		{
			name: "evidence production auth touched",
			mutate: func(_ *BaseRuntimeEquivalenceRetryContract, evidence *BaseStagingReadinessEvidence) {
				evidence.ProductionAuthTouched = true
			},
			wantErr: "evidence carries protected-surface or completion claims",
		},
		{
			name: "evidence promotion claimed",
			mutate: func(_ *BaseRuntimeEquivalenceRetryContract, evidence *BaseStagingReadinessEvidence) {
				evidence.PromotionClaimed = true
			},
			wantErr: "evidence carries protected-surface or completion claims",
		},
		{
			name: "evidence VM lifecycle touched",
			mutate: func(_ *BaseRuntimeEquivalenceRetryContract, evidence *BaseStagingReadinessEvidence) {
				evidence.VMLifecycleTouched = true
			},
			wantErr: "evidence carries protected-surface or completion claims",
		},
		{
			name: "evidence firecracker boot claimed",
			mutate: func(_ *BaseRuntimeEquivalenceRetryContract, evidence *BaseStagingReadinessEvidence) {
				evidence.FirecrackerBootClaimed = true
			},
			wantErr: "evidence carries protected-surface or completion claims",
		},
		{
			name: "evidence run acceptance record touched",
			mutate: func(_ *BaseRuntimeEquivalenceRetryContract, evidence *BaseStagingReadinessEvidence) {
				evidence.RunAcceptanceRecordTouched = true
			},
			wantErr: "evidence carries protected-surface or completion claims",
		},
		{
			name: "evidence package publication claimed",
			mutate: func(_ *BaseRuntimeEquivalenceRetryContract, evidence *BaseStagingReadinessEvidence) {
				evidence.PackagePublicationClaimed = true
			},
			wantErr: "evidence carries protected-surface or completion claims",
		},
		{
			name: "evidence full substrate claimed",
			mutate: func(_ *BaseRuntimeEquivalenceRetryContract, evidence *BaseStagingReadinessEvidence) {
				evidence.FullSubstrateClaimed = true
			},
			wantErr: "evidence carries protected-surface or completion claims",
		},
		{
			name: "evidence completion claimed",
			mutate: func(_ *BaseRuntimeEquivalenceRetryContract, evidence *BaseStagingReadinessEvidence) {
				evidence.CompletionClaimed = true
			},
			wantErr: "evidence carries protected-surface or completion claims",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			retry, evidence := baseStagingReadinessContractInputs(t)
			tc.mutate(&retry, &evidence)

			contract, err := BuildBaseStagingReadinessContract(retry, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildBaseStagingReadinessContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func baseStagingReadinessContractInputs(t *testing.T) (BaseRuntimeEquivalenceRetryContract, BaseStagingReadinessEvidence) {
	t.Helper()

	source, extraction, sourceObservations, runtimeObservations, retryEvidence := baseRuntimeEquivalenceRetryContractInputs(t)
	retry, err := BuildBaseRuntimeEquivalenceRetryContract(source, extraction, sourceObservations, runtimeObservations, retryEvidence)
	if err != nil {
		t.Fatalf("BuildBaseRuntimeEquivalenceRetryContract(): %v", err)
	}

	evidence := BaseStagingReadinessEvidence{
		RuntimeEquivalenceRetryRef:   " equivalence:base-runtime-file-blob-retry-pass-103 ",
		StagingSmokePlanRef:          " staging-smoke-plan:base-staging-readiness-pass-104 ",
		BuildIdentityExpectationRef:  " build-identity:base-staging-readiness-pass-104 ",
		RollbackPlanRef:              " rollback-plan:base-staging-readiness-pass-104 ",
		NoDeploymentMutation:         true,
		NoRouteRegistrationMutation:  true,
		NoRunAcceptanceMutation:      true,
		NoProductionMutation:         true,
		DeploymentExecuted:           false,
		StagingHealthClaimed:         false,
		DeployedRouteIdentityClaimed: false,
		RuntimeBehaviorChanged:       false,
		DeployedRouteRegistered:      false,
		ProductionAuthTouched:        false,
		PromotionClaimed:             false,
		VMLifecycleTouched:           false,
		FirecrackerBootClaimed:       false,
		RunAcceptanceRecordTouched:   false,
		PackagePublicationClaimed:    false,
		FullSubstrateClaimed:         false,
		CompletionClaimed:            false,
	}
	return retry, evidence
}
