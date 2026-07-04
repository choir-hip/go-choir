package computerversion

import (
	"reflect"
	"strings"
	"testing"
)

func TestBuildBasePostSmokeHandoffReadinessContractBlocksDownstreamExecution(t *testing.T) {
	smoke, evidence := basePostSmokeHandoffReadinessContractInputs(t)

	contract, err := BuildBasePostSmokeHandoffReadinessContract(smoke, evidence)
	if err != nil {
		t.Fatalf("BuildBasePostSmokeHandoffReadinessContract(): %v", err)
	}

	if contract.Kind != BasePostSmokeHandoffReadinessContractKind {
		t.Fatalf("kind = %q, want %q", contract.Kind, BasePostSmokeHandoffReadinessContractKind)
	}
	if contract.Version != smoke.Version || ArtifactProgramRef(contract.TypedArtifactProgramRef) != smoke.Version.ArtifactProgramRef {
		t.Fatalf("version/ref = %#v/%q, want smoke version %#v and artifact program %q", contract.Version, contract.TypedArtifactProgramRef, smoke.Version, smoke.Version.ArtifactProgramRef)
	}
	if contract.Boundary != BasePostSmokeHandoffReadinessBoundary {
		t.Fatalf("boundary = %q, want %q", contract.Boundary, BasePostSmokeHandoffReadinessBoundary)
	}
	if contract.Scope != BasePostSmokeHandoffReadinessScope {
		t.Fatalf("scope = %q, want %q", contract.Scope, BasePostSmokeHandoffReadinessScope)
	}
	if contract.StagingSmokeEvidenceRef != strings.TrimSpace(evidence.StagingSmokeEvidenceRef) || contract.ProductPathProbeRef != strings.TrimSpace(smoke.ProductPathProbeRef) || contract.BuildIdentity != strings.TrimSpace(smoke.BuildIdentity) || contract.RouteIdentity != strings.TrimSpace(smoke.RouteIdentity) {
		t.Fatalf("handoff refs/identity = smoke %q probe %q build %q route %q, want trimmed evidence/smoke values", contract.StagingSmokeEvidenceRef, contract.ProductPathProbeRef, contract.BuildIdentity, contract.RouteIdentity)
	}
	if contract.OwnerReviewPlanRef != strings.TrimSpace(evidence.OwnerReviewPlanRef) || contract.PromotionRollbackPlanRef != strings.TrimSpace(evidence.PromotionRollbackPlanRef) || contract.PackagePublicationPlanRef != strings.TrimSpace(evidence.PackagePublicationPlanRef) || contract.VerifierContractPlanRef != strings.TrimSpace(evidence.VerifierContractPlanRef) || contract.RunAcceptanceSynthesisPlanRef != strings.TrimSpace(evidence.RunAcceptanceSynthesisPlanRef) || contract.RollbackPlanRef != strings.TrimSpace(evidence.RollbackPlanRef) {
		t.Fatalf("prerequisite refs = owner %q promotion %q package %q verifier %q run %q rollback %q, want trimmed evidence refs from %#v", contract.OwnerReviewPlanRef, contract.PromotionRollbackPlanRef, contract.PackagePublicationPlanRef, contract.VerifierContractPlanRef, contract.RunAcceptanceSynthesisPlanRef, contract.RollbackPlanRef, evidence)
	}
	if contract.ReadinessStatus != BasePostSmokeHandoffReadinessStatusBlocked {
		t.Fatalf("readiness status = %q, want %q", contract.ReadinessStatus, BasePostSmokeHandoffReadinessStatusBlocked)
	}
	wantPrerequisites := []string{
		BasePostSmokePrerequisiteOwnerReview,
		BasePostSmokePrerequisitePromotionRollbackReview,
		BasePostSmokePrerequisitePackagePublicationReview,
		BasePostSmokePrerequisiteVerifierContractReview,
		BasePostSmokePrerequisiteRunAcceptanceSynthesisReview,
	}
	if !reflect.DeepEqual(contract.RequiredPrerequisites, wantPrerequisites) {
		t.Fatalf("required prerequisites = %#v, want %#v", contract.RequiredPrerequisites, wantPrerequisites)
	}
	if !contract.OwnerReviewRequired || !contract.PromotionRollbackReviewRequired || !contract.PackagePublicationProofRequired || !contract.VerifierContractProofRequired || !contract.RunAcceptanceProofRequired || !contract.FullSubstrateProofRequired {
		t.Fatalf("handoff must require owner review, promotion/rollback review, package publication, verifier contract, run acceptance, and full-substrate proof: %#v", contract)
	}
	if contract.OwnerApprovalAllowed || contract.PromotionAllowed || contract.PackagePublicationAllowed || contract.RunAcceptanceSynthesisAllowed {
		t.Fatalf("handoff must block owner approval, promotion, package publication, and run-acceptance synthesis execution: %#v", contract)
	}
	if !contract.NoOwnerApprovalMutation || !contract.NoPromotionMutation || !contract.NoPackagePublicationMutation || !contract.NoRunAcceptanceMutation || !contract.NoProductionMutation {
		t.Fatalf("handoff must carry no-owner-approval/no-promotion/no-package/no-run-acceptance/no-production mutation flags: %#v", contract)
	}
	if contract.OwnerApproved || contract.PromotionExecuted || contract.PackagePublished || contract.VerifierContractSatisfied || contract.RunAcceptanceRecordTouched || contract.FullSubstrateClaimed || contract.CompletionClaimed {
		t.Fatalf("handoff must not approve, promote, publish, satisfy verifier contract, touch run acceptance, claim full substrate, or claim completion: %#v", contract)
	}
}

func TestBuildBasePostSmokeHandoffReadinessContractRejectsInvalidInputs(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*BaseStagingSmokeEvidenceContract, *BasePostSmokeHandoffReadinessEvidence)
		wantErr string
	}{
		{
			name: "smoke wrong kind",
			mutate: func(smoke *BaseStagingSmokeEvidenceContract, _ *BasePostSmokeHandoffReadinessEvidence) {
				smoke.Kind = BasePostSmokeHandoffReadinessContractKind
			},
			wantErr: "smoke kind",
		},
		{
			name: "smoke boundary drift",
			mutate: func(smoke *BaseStagingSmokeEvidenceContract, _ *BasePostSmokeHandoffReadinessEvidence) {
				smoke.Boundary = BasePostSmokeHandoffReadinessBoundary
			},
			wantErr: "smoke boundary",
		},
		{
			name: "smoke scope drift",
			mutate: func(smoke *BaseStagingSmokeEvidenceContract, _ *BasePostSmokeHandoffReadinessEvidence) {
				smoke.Scope = BasePostSmokeHandoffReadinessScope
			},
			wantErr: "smoke scope",
		},
		{
			name: "invalid smoke version",
			mutate: func(smoke *BaseStagingSmokeEvidenceContract, _ *BasePostSmokeHandoffReadinessEvidence) {
				smoke.Version.CodeRef = "  "
			},
			wantErr: "smoke version is invalid",
		},
		{
			name: "invalid smoke typed artifact ref",
			mutate: func(smoke *BaseStagingSmokeEvidenceContract, _ *BasePostSmokeHandoffReadinessEvidence) {
				smoke.TypedArtifactProgramRef = "artifact-program:base-different"
			},
			wantErr: "smoke typed artifact-program ref is invalid",
		},
		{
			name: "missing smoke staging readiness ref",
			mutate: func(smoke *BaseStagingSmokeEvidenceContract, _ *BasePostSmokeHandoffReadinessEvidence) {
				smoke.StagingReadinessRef = "\t"
			},
			wantErr: "smoke refs are required",
		},
		{
			name: "missing smoke product path probe ref",
			mutate: func(smoke *BaseStagingSmokeEvidenceContract, _ *BasePostSmokeHandoffReadinessEvidence) {
				smoke.ProductPathProbeRef = "  "
			},
			wantErr: "smoke refs are required",
		},
		{
			name: "missing smoke product path URL",
			mutate: func(smoke *BaseStagingSmokeEvidenceContract, _ *BasePostSmokeHandoffReadinessEvidence) {
				smoke.ProductPathURL = ""
			},
			wantErr: "smoke refs are required",
		},
		{
			name: "missing smoke build identity",
			mutate: func(smoke *BaseStagingSmokeEvidenceContract, _ *BasePostSmokeHandoffReadinessEvidence) {
				smoke.BuildIdentity = "  "
			},
			wantErr: "smoke refs are required",
		},
		{
			name: "missing smoke route identity",
			mutate: func(smoke *BaseStagingSmokeEvidenceContract, _ *BasePostSmokeHandoffReadinessEvidence) {
				smoke.RouteIdentity = "\t"
			},
			wantErr: "smoke refs are required",
		},
		{
			name: "missing smoke rollback plan ref",
			mutate: func(smoke *BaseStagingSmokeEvidenceContract, _ *BasePostSmokeHandoffReadinessEvidence) {
				smoke.RollbackPlanRef = "  "
			},
			wantErr: "smoke refs are required",
		},
		{
			name: "failed smoke health",
			mutate: func(smoke *BaseStagingSmokeEvidenceContract, _ *BasePostSmokeHandoffReadinessEvidence) {
				smoke.HealthStatus = "failed"
			},
			wantErr: "smoke evidence must have passed product-path health and identity checks",
		},
		{
			name: "smoke passed flag false",
			mutate: func(smoke *BaseStagingSmokeEvidenceContract, _ *BasePostSmokeHandoffReadinessEvidence) {
				smoke.StagingSmokePassed = false
			},
			wantErr: "smoke evidence must have passed product-path health and identity checks",
		},
		{
			name: "unobserved product path",
			mutate: func(smoke *BaseStagingSmokeEvidenceContract, _ *BasePostSmokeHandoffReadinessEvidence) {
				smoke.ProductPathObserved = false
			},
			wantErr: "smoke evidence must have passed product-path health and identity checks",
		},
		{
			name: "unauthenticated product path",
			mutate: func(smoke *BaseStagingSmokeEvidenceContract, _ *BasePostSmokeHandoffReadinessEvidence) {
				smoke.AuthenticatedProductPath = false
			},
			wantErr: "smoke evidence must have passed product-path health and identity checks",
		},
		{
			name: "build identity match flag false",
			mutate: func(smoke *BaseStagingSmokeEvidenceContract, _ *BasePostSmokeHandoffReadinessEvidence) {
				smoke.BuildIdentityMatched = false
			},
			wantErr: "smoke evidence must have passed product-path health and identity checks",
		},
		{
			name: "route identity match flag false",
			mutate: func(smoke *BaseStagingSmokeEvidenceContract, _ *BasePostSmokeHandoffReadinessEvidence) {
				smoke.RouteIdentityMatched = false
			},
			wantErr: "smoke evidence must have passed product-path health and identity checks",
		},
		{
			name: "missing promotion proof requirement",
			mutate: func(smoke *BaseStagingSmokeEvidenceContract, _ *BasePostSmokeHandoffReadinessEvidence) {
				smoke.PromotionProofRequired = false
			},
			wantErr: "smoke evidence must preserve downstream proof requirements",
		},
		{
			name: "missing package publication proof requirement",
			mutate: func(smoke *BaseStagingSmokeEvidenceContract, _ *BasePostSmokeHandoffReadinessEvidence) {
				smoke.PackagePublicationProofRequired = false
			},
			wantErr: "smoke evidence must preserve downstream proof requirements",
		},
		{
			name: "missing run acceptance proof requirement",
			mutate: func(smoke *BaseStagingSmokeEvidenceContract, _ *BasePostSmokeHandoffReadinessEvidence) {
				smoke.RunAcceptanceProofRequired = false
			},
			wantErr: "smoke evidence must preserve downstream proof requirements",
		},
		{
			name: "missing full substrate proof requirement",
			mutate: func(smoke *BaseStagingSmokeEvidenceContract, _ *BasePostSmokeHandoffReadinessEvidence) {
				smoke.FullSubstrateProofRequired = false
			},
			wantErr: "smoke evidence must preserve downstream proof requirements",
		},
		{
			name: "missing smoke no promotion mutation flag",
			mutate: func(smoke *BaseStagingSmokeEvidenceContract, _ *BasePostSmokeHandoffReadinessEvidence) {
				smoke.NoPromotionMutation = false
			},
			wantErr: "smoke evidence must prove no downstream or production mutation",
		},
		{
			name: "missing smoke no package publication mutation flag",
			mutate: func(smoke *BaseStagingSmokeEvidenceContract, _ *BasePostSmokeHandoffReadinessEvidence) {
				smoke.NoPackagePublicationMutation = false
			},
			wantErr: "smoke evidence must prove no downstream or production mutation",
		},
		{
			name: "missing smoke no run acceptance mutation flag",
			mutate: func(smoke *BaseStagingSmokeEvidenceContract, _ *BasePostSmokeHandoffReadinessEvidence) {
				smoke.NoRunAcceptanceMutation = false
			},
			wantErr: "smoke evidence must prove no downstream or production mutation",
		},
		{
			name: "missing smoke no production mutation flag",
			mutate: func(smoke *BaseStagingSmokeEvidenceContract, _ *BasePostSmokeHandoffReadinessEvidence) {
				smoke.NoProductionMutation = false
			},
			wantErr: "smoke evidence must prove no downstream or production mutation",
		},
		{
			name: "smoke promotion claimed",
			mutate: func(smoke *BaseStagingSmokeEvidenceContract, _ *BasePostSmokeHandoffReadinessEvidence) {
				smoke.PromotionClaimed = true
			},
			wantErr: "smoke evidence carries downstream or completion claims",
		},
		{
			name: "smoke package publication claimed",
			mutate: func(smoke *BaseStagingSmokeEvidenceContract, _ *BasePostSmokeHandoffReadinessEvidence) {
				smoke.PackagePublicationClaimed = true
			},
			wantErr: "smoke evidence carries downstream or completion claims",
		},
		{
			name: "smoke run acceptance touched",
			mutate: func(smoke *BaseStagingSmokeEvidenceContract, _ *BasePostSmokeHandoffReadinessEvidence) {
				smoke.RunAcceptanceRecordTouched = true
			},
			wantErr: "smoke evidence carries downstream or completion claims",
		},
		{
			name: "smoke full substrate claimed",
			mutate: func(smoke *BaseStagingSmokeEvidenceContract, _ *BasePostSmokeHandoffReadinessEvidence) {
				smoke.FullSubstrateClaimed = true
			},
			wantErr: "smoke evidence carries downstream or completion claims",
		},
		{
			name: "smoke completion claimed",
			mutate: func(smoke *BaseStagingSmokeEvidenceContract, _ *BasePostSmokeHandoffReadinessEvidence) {
				smoke.CompletionClaimed = true
			},
			wantErr: "smoke evidence carries downstream or completion claims",
		},
		{
			name: "missing handoff staging smoke evidence ref",
			mutate: func(_ *BaseStagingSmokeEvidenceContract, evidence *BasePostSmokeHandoffReadinessEvidence) {
				evidence.StagingSmokeEvidenceRef = "  "
			},
			wantErr: "prerequisite refs are required",
		},
		{
			name: "missing handoff owner review plan ref",
			mutate: func(_ *BaseStagingSmokeEvidenceContract, evidence *BasePostSmokeHandoffReadinessEvidence) {
				evidence.OwnerReviewPlanRef = "\t"
			},
			wantErr: "prerequisite refs are required",
		},
		{
			name: "missing handoff promotion rollback plan ref",
			mutate: func(_ *BaseStagingSmokeEvidenceContract, evidence *BasePostSmokeHandoffReadinessEvidence) {
				evidence.PromotionRollbackPlanRef = ""
			},
			wantErr: "prerequisite refs are required",
		},
		{
			name: "missing handoff package publication plan ref",
			mutate: func(_ *BaseStagingSmokeEvidenceContract, evidence *BasePostSmokeHandoffReadinessEvidence) {
				evidence.PackagePublicationPlanRef = "  "
			},
			wantErr: "prerequisite refs are required",
		},
		{
			name: "missing handoff verifier contract plan ref",
			mutate: func(_ *BaseStagingSmokeEvidenceContract, evidence *BasePostSmokeHandoffReadinessEvidence) {
				evidence.VerifierContractPlanRef = "\t"
			},
			wantErr: "prerequisite refs are required",
		},
		{
			name: "missing handoff run acceptance synthesis plan ref",
			mutate: func(_ *BaseStagingSmokeEvidenceContract, evidence *BasePostSmokeHandoffReadinessEvidence) {
				evidence.RunAcceptanceSynthesisPlanRef = ""
			},
			wantErr: "prerequisite refs are required",
		},
		{
			name: "missing handoff rollback plan ref",
			mutate: func(_ *BaseStagingSmokeEvidenceContract, evidence *BasePostSmokeHandoffReadinessEvidence) {
				evidence.RollbackPlanRef = "  "
			},
			wantErr: "prerequisite refs are required",
		},
		{
			name: "missing handoff no owner approval mutation flag",
			mutate: func(_ *BaseStagingSmokeEvidenceContract, evidence *BasePostSmokeHandoffReadinessEvidence) {
				evidence.NoOwnerApprovalMutation = false
			},
			wantErr: "evidence must prove no owner approval, promotion, package publication, run-acceptance, or production mutation",
		},
		{
			name: "missing handoff no promotion mutation flag",
			mutate: func(_ *BaseStagingSmokeEvidenceContract, evidence *BasePostSmokeHandoffReadinessEvidence) {
				evidence.NoPromotionMutation = false
			},
			wantErr: "evidence must prove no owner approval, promotion, package publication, run-acceptance, or production mutation",
		},
		{
			name: "missing handoff no package publication mutation flag",
			mutate: func(_ *BaseStagingSmokeEvidenceContract, evidence *BasePostSmokeHandoffReadinessEvidence) {
				evidence.NoPackagePublicationMutation = false
			},
			wantErr: "evidence must prove no owner approval, promotion, package publication, run-acceptance, or production mutation",
		},
		{
			name: "missing handoff no run acceptance mutation flag",
			mutate: func(_ *BaseStagingSmokeEvidenceContract, evidence *BasePostSmokeHandoffReadinessEvidence) {
				evidence.NoRunAcceptanceMutation = false
			},
			wantErr: "evidence must prove no owner approval, promotion, package publication, run-acceptance, or production mutation",
		},
		{
			name: "missing handoff no production mutation flag",
			mutate: func(_ *BaseStagingSmokeEvidenceContract, evidence *BasePostSmokeHandoffReadinessEvidence) {
				evidence.NoProductionMutation = false
			},
			wantErr: "evidence must prove no owner approval, promotion, package publication, run-acceptance, or production mutation",
		},
		{
			name: "owner approved",
			mutate: func(_ *BaseStagingSmokeEvidenceContract, evidence *BasePostSmokeHandoffReadinessEvidence) {
				evidence.OwnerApproved = true
			},
			wantErr: "evidence carries downstream execution or completion claims",
		},
		{
			name: "promotion executed",
			mutate: func(_ *BaseStagingSmokeEvidenceContract, evidence *BasePostSmokeHandoffReadinessEvidence) {
				evidence.PromotionExecuted = true
			},
			wantErr: "evidence carries downstream execution or completion claims",
		},
		{
			name: "package published",
			mutate: func(_ *BaseStagingSmokeEvidenceContract, evidence *BasePostSmokeHandoffReadinessEvidence) {
				evidence.PackagePublished = true
			},
			wantErr: "evidence carries downstream execution or completion claims",
		},
		{
			name: "verifier satisfied",
			mutate: func(_ *BaseStagingSmokeEvidenceContract, evidence *BasePostSmokeHandoffReadinessEvidence) {
				evidence.VerifierContractSatisfied = true
			},
			wantErr: "evidence carries downstream execution or completion claims",
		},
		{
			name: "run acceptance touched",
			mutate: func(_ *BaseStagingSmokeEvidenceContract, evidence *BasePostSmokeHandoffReadinessEvidence) {
				evidence.RunAcceptanceRecordTouched = true
			},
			wantErr: "evidence carries downstream execution or completion claims",
		},
		{
			name: "full substrate claimed",
			mutate: func(_ *BaseStagingSmokeEvidenceContract, evidence *BasePostSmokeHandoffReadinessEvidence) {
				evidence.FullSubstrateClaimed = true
			},
			wantErr: "evidence carries downstream execution or completion claims",
		},
		{
			name: "completion claimed",
			mutate: func(_ *BaseStagingSmokeEvidenceContract, evidence *BasePostSmokeHandoffReadinessEvidence) {
				evidence.CompletionClaimed = true
			},
			wantErr: "evidence carries downstream execution or completion claims",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			smoke, evidence := basePostSmokeHandoffReadinessContractInputs(t)
			tc.mutate(&smoke, &evidence)

			contract, err := BuildBasePostSmokeHandoffReadinessContract(smoke, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildBasePostSmokeHandoffReadinessContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func basePostSmokeHandoffReadinessContractInputs(t *testing.T) (BaseStagingSmokeEvidenceContract, BasePostSmokeHandoffReadinessEvidence) {
	t.Helper()

	readiness, smokeEvidence := baseStagingSmokeEvidenceContractInputs(t)
	smoke, err := BuildBaseStagingSmokeEvidenceContract(readiness, smokeEvidence)
	if err != nil {
		t.Fatalf("BuildBaseStagingSmokeEvidenceContract(): %v", err)
	}

	evidence := BasePostSmokeHandoffReadinessEvidence{
		StagingSmokeEvidenceRef:       " staging-smoke-evidence:base-pass-105 ",
		OwnerReviewPlanRef:            " owner-review:base-pass-106 ",
		PromotionRollbackPlanRef:      " promotion-rollback-review:base-pass-106 ",
		PackagePublicationPlanRef:     " package-publication-review:base-pass-106 ",
		VerifierContractPlanRef:       " verifier-contract-review:base-pass-106 ",
		RunAcceptanceSynthesisPlanRef: " run-acceptance-synthesis-review:base-pass-106 ",
		RollbackPlanRef:               " " + smoke.RollbackPlanRef + " ",
		NoOwnerApprovalMutation:       true,
		NoPromotionMutation:           true,
		NoPackagePublicationMutation:  true,
		NoRunAcceptanceMutation:       true,
		NoProductionMutation:          true,
		OwnerApproved:                 false,
		PromotionExecuted:             false,
		PackagePublished:              false,
		RunAcceptanceRecordTouched:    false,
		VerifierContractSatisfied:     false,
		FullSubstrateClaimed:          false,
		CompletionClaimed:             false,
	}
	return smoke, evidence
}
