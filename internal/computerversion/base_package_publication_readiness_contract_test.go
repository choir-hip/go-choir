package computerversion

import (
	"strings"
	"testing"
)

func TestBuildBasePackagePublicationReadinessContractRecordsReadinessOnly(t *testing.T) {
	review, evidence := basePackagePublicationReadinessContractInputs(t)

	contract, err := BuildBasePackagePublicationReadinessContract(review, evidence)
	if err != nil {
		t.Fatalf("BuildBasePackagePublicationReadinessContract(): %v", err)
	}

	if contract.Kind != BasePackagePublicationReadinessContractKind {
		t.Fatalf("kind = %q, want %q", contract.Kind, BasePackagePublicationReadinessContractKind)
	}
	if contract.Boundary != BasePackagePublicationReadinessBoundary {
		t.Fatalf("boundary = %q, want %q", contract.Boundary, BasePackagePublicationReadinessBoundary)
	}
	if contract.Scope != BasePackagePublicationReadinessScope {
		t.Fatalf("scope = %q, want %q", contract.Scope, BasePackagePublicationReadinessScope)
	}
	if contract.Version != review.Version || ArtifactProgramRef(contract.TypedArtifactProgramRef) != review.Version.ArtifactProgramRef {
		t.Fatalf("version/ref = %#v/%q, want promotion review version %#v and artifact program %q", contract.Version, contract.TypedArtifactProgramRef, review.Version, review.Version.ArtifactProgramRef)
	}
	if contract.PromotionRollbackReviewRef != strings.TrimSpace(evidence.PromotionRollbackReviewRef) || contract.PromotionPlanRef != strings.TrimSpace(review.PromotionPlanRef) || contract.RollbackPlanRef != strings.TrimSpace(evidence.RollbackPlanRef) {
		t.Fatalf("review refs = review %q promotion %q rollback %q, want trimmed review/evidence refs from review %#v evidence %#v", contract.PromotionRollbackReviewRef, contract.PromotionPlanRef, contract.RollbackPlanRef, review, evidence)
	}
	if contract.PackageManifestRef != strings.TrimSpace(evidence.PackageManifestRef) || contract.PublicationPayloadRef != strings.TrimSpace(evidence.PublicationPayloadRef) || contract.PublicationTargetRef != strings.TrimSpace(evidence.PublicationTargetRef) || contract.PublicationPolicyRef != strings.TrimSpace(evidence.PublicationPolicyRef) || contract.PublicationDryRunPlanRef != strings.TrimSpace(evidence.PublicationDryRunPlanRef) {
		t.Fatalf("publication refs = manifest %q payload %q target %q policy %q dry-run %q, want trimmed evidence refs from %#v", contract.PackageManifestRef, contract.PublicationPayloadRef, contract.PublicationTargetRef, contract.PublicationPolicyRef, contract.PublicationDryRunPlanRef, evidence)
	}
	if contract.ReadinessStatus != BasePackagePublicationReadinessStatusReady || !contract.OwnerApproved || !contract.PromotionRollbackReviewReady || !contract.PackagePublicationReady {
		t.Fatalf("publication readiness = status %q owner %v review-ready %v package-ready %v, want package-publication readiness from approved promotion/rollback review", contract.ReadinessStatus, contract.OwnerApproved, contract.PromotionRollbackReviewReady, contract.PackagePublicationReady)
	}
	if !contract.RunAcceptanceProofRequired || !contract.FullSubstrateProofRequired {
		t.Fatalf("package-publication readiness must preserve run-acceptance and full-substrate proof requirements: %#v", contract)
	}
	if contract.PackagePublicationAllowed || contract.PromotionAllowed || contract.RunAcceptanceSynthesisAllowed {
		t.Fatalf("package-publication readiness must not allow publication, promotion, or run-acceptance synthesis: %#v", contract)
	}
	if !contract.NoPackagePublicationMutation || !contract.NoPromotionMutation || !contract.NoRunAcceptanceMutation || !contract.NoProductionMutation {
		t.Fatalf("package-publication readiness must carry no-publication/no-promotion/no-run-acceptance/no-production mutation evidence: %#v", contract)
	}
	if contract.PackagePublished || contract.PromotionExecuted || contract.RunAcceptanceRecordTouched || contract.FullSubstrateClaimed || contract.CompletionClaimed {
		t.Fatalf("package-publication readiness must not publish, promote, touch run acceptance, claim full substrate, or claim completion: %#v", contract)
	}
}

func TestBuildBasePackagePublicationReadinessContractRejectsInvalidInputs(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*BasePromotionRollbackReviewContract, *BasePackagePublicationReadinessEvidence)
		wantErr string
	}{
		{
			name: "promotion review wrong kind",
			mutate: func(review *BasePromotionRollbackReviewContract, _ *BasePackagePublicationReadinessEvidence) {
				review.Kind = BaseOwnerApprovalContractKind
			},
			wantErr: "promotion/rollback review kind",
		},
		{
			name: "promotion review boundary drift",
			mutate: func(review *BasePromotionRollbackReviewContract, _ *BasePackagePublicationReadinessEvidence) {
				review.Boundary = BaseOwnerApprovalBoundary
			},
			wantErr: "promotion/rollback review boundary",
		},
		{
			name: "promotion review scope drift",
			mutate: func(review *BasePromotionRollbackReviewContract, _ *BasePackagePublicationReadinessEvidence) {
				review.Scope = BaseOwnerApprovalScope
			},
			wantErr: "promotion/rollback review scope",
		},
		{
			name: "promotion review invalid version",
			mutate: func(review *BasePromotionRollbackReviewContract, _ *BasePackagePublicationReadinessEvidence) {
				review.Version.ArtifactProgramRef = "  "
			},
			wantErr: "promotion/rollback review version is invalid",
		},
		{
			name: "promotion review artifact ref drift",
			mutate: func(review *BasePromotionRollbackReviewContract, _ *BasePackagePublicationReadinessEvidence) {
				review.TypedArtifactProgramRef = "artifact-program:base-different"
			},
			wantErr: "promotion/rollback review typed artifact-program ref is invalid",
		},
		{
			name: "promotion review status not ready",
			mutate: func(review *BasePromotionRollbackReviewContract, _ *BasePackagePublicationReadinessEvidence) {
				review.ReviewStatus = "needs-review"
			},
			wantErr: "promotion/rollback review must be ready",
		},
		{
			name: "promotion review missing promotion plan ref",
			mutate: func(review *BasePromotionRollbackReviewContract, _ *BasePackagePublicationReadinessEvidence) {
				review.PromotionPlanRef = "\t"
			},
			wantErr: "promotion/rollback review refs are required",
		},
		{
			name: "promotion review missing rollback plan ref",
			mutate: func(review *BasePromotionRollbackReviewContract, _ *BasePackagePublicationReadinessEvidence) {
				review.RollbackPlanRef = "  "
			},
			wantErr: "promotion/rollback review refs are required",
		},
		{
			name: "promotion review missing risk review ref",
			mutate: func(review *BasePromotionRollbackReviewContract, _ *BasePackagePublicationReadinessEvidence) {
				review.PromotionRiskReviewRef = ""
			},
			wantErr: "promotion/rollback review refs are required",
		},
		{
			name: "promotion review missing ledger freshness ref",
			mutate: func(review *BasePromotionRollbackReviewContract, _ *BasePackagePublicationReadinessEvidence) {
				review.LedgerFreshnessCheckRef = "  "
			},
			wantErr: "promotion/rollback review refs are required",
		},
		{
			name: "promotion review missing route continuity ref",
			mutate: func(review *BasePromotionRollbackReviewContract, _ *BasePackagePublicationReadinessEvidence) {
				review.RouteContinuityCheckRef = "\t"
			},
			wantErr: "promotion/rollback review refs are required",
		},
		{
			name: "promotion review missing operator policy ref",
			mutate: func(review *BasePromotionRollbackReviewContract, _ *BasePackagePublicationReadinessEvidence) {
				review.OperatorReviewPolicyRef = ""
			},
			wantErr: "promotion/rollback review refs are required",
		},
		{
			name: "promotion review verifier contract not satisfied",
			mutate: func(review *BasePromotionRollbackReviewContract, _ *BasePackagePublicationReadinessEvidence) {
				review.VerifierContractSatisfied = false
			},
			wantErr: "promotion/rollback review must be ready",
		},
		{
			name: "promotion review not owner approved",
			mutate: func(review *BasePromotionRollbackReviewContract, _ *BasePackagePublicationReadinessEvidence) {
				review.OwnerApproved = false
			},
			wantErr: "promotion/rollback review must be ready",
		},
		{
			name: "promotion review not ready",
			mutate: func(review *BasePromotionRollbackReviewContract, _ *BasePackagePublicationReadinessEvidence) {
				review.PromotionRollbackReviewReady = false
			},
			wantErr: "promotion/rollback review must be ready",
		},
		{
			name: "promotion review missing package publication proof requirement",
			mutate: func(review *BasePromotionRollbackReviewContract, _ *BasePackagePublicationReadinessEvidence) {
				review.PackagePublicationProofRequired = false
			},
			wantErr: "promotion/rollback review must preserve downstream proof requirements",
		},
		{
			name: "promotion review missing run acceptance proof requirement",
			mutate: func(review *BasePromotionRollbackReviewContract, _ *BasePackagePublicationReadinessEvidence) {
				review.RunAcceptanceProofRequired = false
			},
			wantErr: "promotion/rollback review must preserve downstream proof requirements",
		},
		{
			name: "promotion review missing full substrate proof requirement",
			mutate: func(review *BasePromotionRollbackReviewContract, _ *BasePackagePublicationReadinessEvidence) {
				review.FullSubstrateProofRequired = false
			},
			wantErr: "promotion/rollback review must preserve downstream proof requirements",
		},
		{
			name: "promotion review allows promotion",
			mutate: func(review *BasePromotionRollbackReviewContract, _ *BasePackagePublicationReadinessEvidence) {
				review.PromotionAllowed = true
			},
			wantErr: "promotion/rollback review allows downstream execution",
		},
		{
			name: "promotion review allows package publication",
			mutate: func(review *BasePromotionRollbackReviewContract, _ *BasePackagePublicationReadinessEvidence) {
				review.PackagePublicationAllowed = true
			},
			wantErr: "promotion/rollback review allows downstream execution",
		},
		{
			name: "promotion review allows run acceptance synthesis",
			mutate: func(review *BasePromotionRollbackReviewContract, _ *BasePackagePublicationReadinessEvidence) {
				review.RunAcceptanceSynthesisAllowed = true
			},
			wantErr: "promotion/rollback review allows downstream execution",
		},
		{
			name: "promotion review missing no promotion mutation flag",
			mutate: func(review *BasePromotionRollbackReviewContract, _ *BasePackagePublicationReadinessEvidence) {
				review.NoPromotionMutation = false
			},
			wantErr: "promotion/rollback review must prove no promotion, package publication, run-acceptance, or production mutation",
		},
		{
			name: "promotion review missing no package publication mutation flag",
			mutate: func(review *BasePromotionRollbackReviewContract, _ *BasePackagePublicationReadinessEvidence) {
				review.NoPackagePublicationMutation = false
			},
			wantErr: "promotion/rollback review must prove no promotion, package publication, run-acceptance, or production mutation",
		},
		{
			name: "promotion review missing no run acceptance mutation flag",
			mutate: func(review *BasePromotionRollbackReviewContract, _ *BasePackagePublicationReadinessEvidence) {
				review.NoRunAcceptanceMutation = false
			},
			wantErr: "promotion/rollback review must prove no promotion, package publication, run-acceptance, or production mutation",
		},
		{
			name: "promotion review missing no production mutation flag",
			mutate: func(review *BasePromotionRollbackReviewContract, _ *BasePackagePublicationReadinessEvidence) {
				review.NoProductionMutation = false
			},
			wantErr: "promotion/rollback review must prove no promotion, package publication, run-acceptance, or production mutation",
		},
		{
			name: "promotion review claims package published",
			mutate: func(review *BasePromotionRollbackReviewContract, _ *BasePackagePublicationReadinessEvidence) {
				review.PackagePublished = true
			},
			wantErr: "promotion/rollback review carries downstream execution or completion claims",
		},
		{
			name: "promotion review claims promotion executed",
			mutate: func(review *BasePromotionRollbackReviewContract, _ *BasePackagePublicationReadinessEvidence) {
				review.PromotionExecuted = true
			},
			wantErr: "promotion/rollback review carries downstream execution or completion claims",
		},
		{
			name: "promotion review claims run acceptance touched",
			mutate: func(review *BasePromotionRollbackReviewContract, _ *BasePackagePublicationReadinessEvidence) {
				review.RunAcceptanceRecordTouched = true
			},
			wantErr: "promotion/rollback review carries downstream execution or completion claims",
		},
		{
			name: "promotion review claims full substrate",
			mutate: func(review *BasePromotionRollbackReviewContract, _ *BasePackagePublicationReadinessEvidence) {
				review.FullSubstrateClaimed = true
			},
			wantErr: "promotion/rollback review carries downstream execution or completion claims",
		},
		{
			name: "promotion review claims completion",
			mutate: func(review *BasePromotionRollbackReviewContract, _ *BasePackagePublicationReadinessEvidence) {
				review.CompletionClaimed = true
			},
			wantErr: "promotion/rollback review carries downstream execution or completion claims",
		},
		{
			name: "missing evidence promotion review ref",
			mutate: func(_ *BasePromotionRollbackReviewContract, evidence *BasePackagePublicationReadinessEvidence) {
				evidence.PromotionRollbackReviewRef = "  "
			},
			wantErr: "publication refs are required",
		},
		{
			name: "missing evidence package manifest ref",
			mutate: func(_ *BasePromotionRollbackReviewContract, evidence *BasePackagePublicationReadinessEvidence) {
				evidence.PackageManifestRef = "\t"
			},
			wantErr: "publication refs are required",
		},
		{
			name: "missing evidence publication payload ref",
			mutate: func(_ *BasePromotionRollbackReviewContract, evidence *BasePackagePublicationReadinessEvidence) {
				evidence.PublicationPayloadRef = ""
			},
			wantErr: "publication refs are required",
		},
		{
			name: "missing evidence publication target ref",
			mutate: func(_ *BasePromotionRollbackReviewContract, evidence *BasePackagePublicationReadinessEvidence) {
				evidence.PublicationTargetRef = "  "
			},
			wantErr: "publication refs are required",
		},
		{
			name: "missing evidence publication policy ref",
			mutate: func(_ *BasePromotionRollbackReviewContract, evidence *BasePackagePublicationReadinessEvidence) {
				evidence.PublicationPolicyRef = "\t"
			},
			wantErr: "publication refs are required",
		},
		{
			name: "missing evidence publication dry-run plan ref",
			mutate: func(_ *BasePromotionRollbackReviewContract, evidence *BasePackagePublicationReadinessEvidence) {
				evidence.PublicationDryRunPlanRef = ""
			},
			wantErr: "publication refs are required",
		},
		{
			name: "missing evidence rollback plan ref",
			mutate: func(_ *BasePromotionRollbackReviewContract, evidence *BasePackagePublicationReadinessEvidence) {
				evidence.RollbackPlanRef = "  "
			},
			wantErr: "publication refs are required",
		},
		{
			name: "evidence missing no package publication mutation flag",
			mutate: func(_ *BasePromotionRollbackReviewContract, evidence *BasePackagePublicationReadinessEvidence) {
				evidence.NoPackagePublicationMutation = false
			},
			wantErr: "evidence must prove no package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "evidence missing no promotion mutation flag",
			mutate: func(_ *BasePromotionRollbackReviewContract, evidence *BasePackagePublicationReadinessEvidence) {
				evidence.NoPromotionMutation = false
			},
			wantErr: "evidence must prove no package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "evidence missing no run acceptance mutation flag",
			mutate: func(_ *BasePromotionRollbackReviewContract, evidence *BasePackagePublicationReadinessEvidence) {
				evidence.NoRunAcceptanceMutation = false
			},
			wantErr: "evidence must prove no package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "evidence missing no production mutation flag",
			mutate: func(_ *BasePromotionRollbackReviewContract, evidence *BasePackagePublicationReadinessEvidence) {
				evidence.NoProductionMutation = false
			},
			wantErr: "evidence must prove no package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "evidence claims package published",
			mutate: func(_ *BasePromotionRollbackReviewContract, evidence *BasePackagePublicationReadinessEvidence) {
				evidence.PackagePublished = true
			},
			wantErr: "evidence carries downstream execution or completion claims",
		},
		{
			name: "evidence claims promotion executed",
			mutate: func(_ *BasePromotionRollbackReviewContract, evidence *BasePackagePublicationReadinessEvidence) {
				evidence.PromotionExecuted = true
			},
			wantErr: "evidence carries downstream execution or completion claims",
		},
		{
			name: "evidence claims run acceptance touched",
			mutate: func(_ *BasePromotionRollbackReviewContract, evidence *BasePackagePublicationReadinessEvidence) {
				evidence.RunAcceptanceRecordTouched = true
			},
			wantErr: "evidence carries downstream execution or completion claims",
		},
		{
			name: "evidence claims full substrate",
			mutate: func(_ *BasePromotionRollbackReviewContract, evidence *BasePackagePublicationReadinessEvidence) {
				evidence.FullSubstrateClaimed = true
			},
			wantErr: "evidence carries downstream execution or completion claims",
		},
		{
			name: "evidence claims completion",
			mutate: func(_ *BasePromotionRollbackReviewContract, evidence *BasePackagePublicationReadinessEvidence) {
				evidence.CompletionClaimed = true
			},
			wantErr: "evidence carries downstream execution or completion claims",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			review, evidence := basePackagePublicationReadinessContractInputs(t)
			tc.mutate(&review, &evidence)

			contract, err := BuildBasePackagePublicationReadinessContract(review, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildBasePackagePublicationReadinessContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func basePackagePublicationReadinessContractInputs(t *testing.T) (BasePromotionRollbackReviewContract, BasePackagePublicationReadinessEvidence) {
	t.Helper()

	owner, reviewEvidence := basePromotionRollbackReviewContractInputs(t)
	review, err := BuildBasePromotionRollbackReviewContract(owner, reviewEvidence)
	if err != nil {
		t.Fatalf("BuildBasePromotionRollbackReviewContract(): %v", err)
	}
	return review, basePackagePublicationReadinessEvidence(review)
}

func basePackagePublicationReadinessEvidence(review BasePromotionRollbackReviewContract) BasePackagePublicationReadinessEvidence {
	return BasePackagePublicationReadinessEvidence{
		PromotionRollbackReviewRef:   " promotion-rollback-review:base-pass-112 ",
		PackageManifestRef:           " package-manifest:base-pass-112 ",
		PublicationPayloadRef:        " publication-payload:base-pass-112 ",
		PublicationTargetRef:         " publication-target:base-pass-112 ",
		PublicationPolicyRef:         " publication-policy:base-pass-112 ",
		PublicationDryRunPlanRef:     " publication-dry-run-plan:base-pass-112 ",
		RollbackPlanRef:              " " + review.RollbackPlanRef + " ",
		NoPackagePublicationMutation: true,
		NoPromotionMutation:          true,
		NoRunAcceptanceMutation:      true,
		NoProductionMutation:         true,
		PackagePublished:             false,
		PromotionExecuted:            false,
		RunAcceptanceRecordTouched:   false,
		FullSubstrateClaimed:         false,
		CompletionClaimed:            false,
	}
}
