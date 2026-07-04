package computerversion

import (
	"strings"
	"testing"
)

func TestBuildBasePromotionRollbackReviewContractRecordsReadinessOnly(t *testing.T) {
	owner, evidence := basePromotionRollbackReviewContractInputs(t)

	contract, err := BuildBasePromotionRollbackReviewContract(owner, evidence)
	if err != nil {
		t.Fatalf("BuildBasePromotionRollbackReviewContract(): %v", err)
	}

	if contract.Kind != BasePromotionRollbackReviewContractKind {
		t.Fatalf("kind = %q, want %q", contract.Kind, BasePromotionRollbackReviewContractKind)
	}
	if contract.Boundary != BasePromotionRollbackReviewBoundary {
		t.Fatalf("boundary = %q, want %q", contract.Boundary, BasePromotionRollbackReviewBoundary)
	}
	if contract.Scope != BasePromotionRollbackReviewScope {
		t.Fatalf("scope = %q, want %q", contract.Scope, BasePromotionRollbackReviewScope)
	}
	if contract.Version != owner.Version || ArtifactProgramRef(contract.TypedArtifactProgramRef) != owner.Version.ArtifactProgramRef {
		t.Fatalf("version/ref = %#v/%q, want owner version %#v and artifact program %q", contract.Version, contract.TypedArtifactProgramRef, owner.Version, owner.Version.ArtifactProgramRef)
	}
	if contract.OwnerApprovalRef != strings.TrimSpace(evidence.OwnerApprovalRef) || contract.OwnerDecisionRef != strings.TrimSpace(owner.OwnerDecisionRef) || contract.OwnerIdentityRef != strings.TrimSpace(owner.OwnerIdentityRef) {
		t.Fatalf("owner refs = approval %q decision %q identity %q, want trimmed evidence/owner refs from owner %#v evidence %#v", contract.OwnerApprovalRef, contract.OwnerDecisionRef, contract.OwnerIdentityRef, owner, evidence)
	}
	if contract.PromotionPlanRef != strings.TrimSpace(evidence.PromotionPlanRef) || contract.RollbackPlanRef != strings.TrimSpace(evidence.RollbackPlanRef) || contract.PromotionRiskReviewRef != strings.TrimSpace(evidence.PromotionRiskReviewRef) || contract.LedgerFreshnessCheckRef != strings.TrimSpace(evidence.LedgerFreshnessCheckRef) || contract.RouteContinuityCheckRef != strings.TrimSpace(evidence.RouteContinuityCheckRef) || contract.OperatorReviewPolicyRef != strings.TrimSpace(evidence.OperatorReviewPolicyRef) {
		t.Fatalf("review refs = promotion %q rollback %q risk %q ledger %q route %q policy %q, want trimmed evidence refs from %#v", contract.PromotionPlanRef, contract.RollbackPlanRef, contract.PromotionRiskReviewRef, contract.LedgerFreshnessCheckRef, contract.RouteContinuityCheckRef, contract.OperatorReviewPolicyRef, evidence)
	}
	if contract.ReviewStatus != BasePromotionRollbackReviewStatusReady || !contract.VerifierContractSatisfied || !contract.OwnerApproved || !contract.PromotionRollbackReviewReady {
		t.Fatalf("review readiness = status %q verifier %v owner %v review-ready %v, want approved owner readiness only", contract.ReviewStatus, contract.VerifierContractSatisfied, contract.OwnerApproved, contract.PromotionRollbackReviewReady)
	}
	if !contract.PackagePublicationProofRequired || !contract.RunAcceptanceProofRequired || !contract.FullSubstrateProofRequired {
		t.Fatalf("promotion/rollback review must preserve package-publication, run-acceptance, and full-substrate proof requirements: %#v", contract)
	}
	if contract.PromotionAllowed || contract.PackagePublicationAllowed || contract.RunAcceptanceSynthesisAllowed {
		t.Fatalf("promotion/rollback review must not allow promotion, publication, or run-acceptance synthesis: %#v", contract)
	}
	if !contract.NoPromotionMutation || !contract.NoPackagePublicationMutation || !contract.NoRunAcceptanceMutation || !contract.NoProductionMutation {
		t.Fatalf("promotion/rollback review must carry no-promotion/no-package/no-run-acceptance/no-production mutation evidence: %#v", contract)
	}
	if contract.PromotionExecuted || contract.PackagePublished || contract.RunAcceptanceRecordTouched || contract.FullSubstrateClaimed || contract.CompletionClaimed {
		t.Fatalf("promotion/rollback review must not promote, publish, touch run acceptance, claim full substrate, or claim completion: %#v", contract)
	}
}

func TestBuildBasePromotionRollbackReviewContractRejectsRejectedOwnerApproval(t *testing.T) {
	ownerReview, verifier, approvalEvidence := baseOwnerApprovalContractInputs(t, BaseOwnerDecisionReject)
	owner, err := BuildBaseOwnerApprovalContract(ownerReview, verifier, approvalEvidence)
	if err != nil {
		t.Fatalf("BuildBaseOwnerApprovalContract(): %v", err)
	}
	evidence := basePromotionRollbackReviewEvidence(owner)

	contract, err := BuildBasePromotionRollbackReviewContract(owner, evidence)
	if err == nil || !strings.Contains(err.Error(), "owner approval must be approved and unblocked") {
		t.Fatalf("BuildBasePromotionRollbackReviewContract() = contract %#v error %v, want approved-and-unblocked owner error", contract, err)
	}
}

func TestBuildBasePromotionRollbackReviewContractRejectsInvalidInputs(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*BaseOwnerApprovalContract, *BasePromotionRollbackReviewEvidence)
		wantErr string
	}{
		{
			name: "owner approval wrong kind",
			mutate: func(owner *BaseOwnerApprovalContract, _ *BasePromotionRollbackReviewEvidence) {
				owner.Kind = BaseOwnerReviewReadinessContractKind
			},
			wantErr: "owner approval kind",
		},
		{
			name: "owner approval boundary drift",
			mutate: func(owner *BaseOwnerApprovalContract, _ *BasePromotionRollbackReviewEvidence) {
				owner.Boundary = BaseOwnerReviewReadinessBoundary
			},
			wantErr: "owner approval boundary",
		},
		{
			name: "owner approval scope drift",
			mutate: func(owner *BaseOwnerApprovalContract, _ *BasePromotionRollbackReviewEvidence) {
				owner.Scope = BaseOwnerReviewReadinessScope
			},
			wantErr: "owner approval scope",
		},
		{
			name: "owner approval invalid version",
			mutate: func(owner *BaseOwnerApprovalContract, _ *BasePromotionRollbackReviewEvidence) {
				owner.Version.CodeRef = "  "
			},
			wantErr: "owner approval version is invalid",
		},
		{
			name: "owner approval artifact ref drift",
			mutate: func(owner *BaseOwnerApprovalContract, _ *BasePromotionRollbackReviewEvidence) {
				owner.TypedArtifactProgramRef = "artifact-program:base-different"
			},
			wantErr: "owner approval typed artifact-program ref is invalid",
		},
		{
			name: "missing owner decision ref",
			mutate: func(owner *BaseOwnerApprovalContract, _ *BasePromotionRollbackReviewEvidence) {
				owner.OwnerDecisionRef = "\t"
			},
			wantErr: "owner approval refs are required",
		},
		{
			name: "missing owner identity ref",
			mutate: func(owner *BaseOwnerApprovalContract, _ *BasePromotionRollbackReviewEvidence) {
				owner.OwnerIdentityRef = "  "
			},
			wantErr: "owner approval refs are required",
		},
		{
			name: "missing owner rollback plan ref",
			mutate: func(owner *BaseOwnerApprovalContract, _ *BasePromotionRollbackReviewEvidence) {
				owner.RollbackPlanRef = ""
			},
			wantErr: "owner approval refs are required",
		},
		{
			name: "owner approval not recorded",
			mutate: func(owner *BaseOwnerApprovalContract, _ *BasePromotionRollbackReviewEvidence) {
				owner.OwnerApprovalRecorded = false
			},
			wantErr: "owner approval must be approved and unblocked",
		},
		{
			name: "owner approval rejected",
			mutate: func(owner *BaseOwnerApprovalContract, _ *BasePromotionRollbackReviewEvidence) {
				owner.OwnerDecision = BaseOwnerDecisionReject
				owner.OwnerApproved = false
				owner.OwnerRejected = true
				owner.OwnerRejectionBlocksDownstream = true
				owner.RejectionReason = " owner rejected promotion "
			},
			wantErr: "owner approval must be approved and unblocked",
		},
		{
			name: "owner approval blocked",
			mutate: func(owner *BaseOwnerApprovalContract, _ *BasePromotionRollbackReviewEvidence) {
				owner.OwnerRejectionBlocksDownstream = true
			},
			wantErr: "owner approval must be approved and unblocked",
		},
		{
			name: "owner approval missing promotion rollback review requirement",
			mutate: func(owner *BaseOwnerApprovalContract, _ *BasePromotionRollbackReviewEvidence) {
				owner.PromotionRollbackReviewRequired = false
			},
			wantErr: "owner approval must preserve downstream proof requirements",
		},
		{
			name: "owner approval missing package publication proof requirement",
			mutate: func(owner *BaseOwnerApprovalContract, _ *BasePromotionRollbackReviewEvidence) {
				owner.PackagePublicationProofRequired = false
			},
			wantErr: "owner approval must preserve downstream proof requirements",
		},
		{
			name: "owner approval missing run acceptance proof requirement",
			mutate: func(owner *BaseOwnerApprovalContract, _ *BasePromotionRollbackReviewEvidence) {
				owner.RunAcceptanceProofRequired = false
			},
			wantErr: "owner approval must preserve downstream proof requirements",
		},
		{
			name: "owner approval missing full substrate proof requirement",
			mutate: func(owner *BaseOwnerApprovalContract, _ *BasePromotionRollbackReviewEvidence) {
				owner.FullSubstrateProofRequired = false
			},
			wantErr: "owner approval must preserve downstream proof requirements",
		},
		{
			name: "owner approval allows promotion",
			mutate: func(owner *BaseOwnerApprovalContract, _ *BasePromotionRollbackReviewEvidence) {
				owner.PromotionAllowed = true
			},
			wantErr: "owner approval allows downstream execution",
		},
		{
			name: "owner approval allows package publication",
			mutate: func(owner *BaseOwnerApprovalContract, _ *BasePromotionRollbackReviewEvidence) {
				owner.PackagePublicationAllowed = true
			},
			wantErr: "owner approval allows downstream execution",
		},
		{
			name: "owner approval allows run acceptance synthesis",
			mutate: func(owner *BaseOwnerApprovalContract, _ *BasePromotionRollbackReviewEvidence) {
				owner.RunAcceptanceSynthesisAllowed = true
			},
			wantErr: "owner approval allows downstream execution",
		},
		{
			name: "owner approval missing no promotion mutation flag",
			mutate: func(owner *BaseOwnerApprovalContract, _ *BasePromotionRollbackReviewEvidence) {
				owner.NoPromotionMutation = false
			},
			wantErr: "owner approval must prove no promotion, package publication, run-acceptance, or production mutation",
		},
		{
			name: "owner approval missing no package publication mutation flag",
			mutate: func(owner *BaseOwnerApprovalContract, _ *BasePromotionRollbackReviewEvidence) {
				owner.NoPackagePublicationMutation = false
			},
			wantErr: "owner approval must prove no promotion, package publication, run-acceptance, or production mutation",
		},
		{
			name: "owner approval missing no run acceptance mutation flag",
			mutate: func(owner *BaseOwnerApprovalContract, _ *BasePromotionRollbackReviewEvidence) {
				owner.NoRunAcceptanceMutation = false
			},
			wantErr: "owner approval must prove no promotion, package publication, run-acceptance, or production mutation",
		},
		{
			name: "owner approval missing no production mutation flag",
			mutate: func(owner *BaseOwnerApprovalContract, _ *BasePromotionRollbackReviewEvidence) {
				owner.NoProductionMutation = false
			},
			wantErr: "owner approval must prove no promotion, package publication, run-acceptance, or production mutation",
		},
		{
			name: "owner approval claims promotion executed",
			mutate: func(owner *BaseOwnerApprovalContract, _ *BasePromotionRollbackReviewEvidence) {
				owner.PromotionExecuted = true
			},
			wantErr: "owner approval carries downstream execution or completion claims",
		},
		{
			name: "owner approval claims package published",
			mutate: func(owner *BaseOwnerApprovalContract, _ *BasePromotionRollbackReviewEvidence) {
				owner.PackagePublished = true
			},
			wantErr: "owner approval carries downstream execution or completion claims",
		},
		{
			name: "owner approval claims run acceptance touched",
			mutate: func(owner *BaseOwnerApprovalContract, _ *BasePromotionRollbackReviewEvidence) {
				owner.RunAcceptanceRecordTouched = true
			},
			wantErr: "owner approval carries downstream execution or completion claims",
		},
		{
			name: "owner approval claims full substrate",
			mutate: func(owner *BaseOwnerApprovalContract, _ *BasePromotionRollbackReviewEvidence) {
				owner.FullSubstrateClaimed = true
			},
			wantErr: "owner approval carries downstream execution or completion claims",
		},
		{
			name: "owner approval claims completion",
			mutate: func(owner *BaseOwnerApprovalContract, _ *BasePromotionRollbackReviewEvidence) {
				owner.CompletionClaimed = true
			},
			wantErr: "owner approval carries downstream execution or completion claims",
		},
		{
			name: "missing evidence owner approval ref",
			mutate: func(_ *BaseOwnerApprovalContract, evidence *BasePromotionRollbackReviewEvidence) {
				evidence.OwnerApprovalRef = "  "
			},
			wantErr: "review refs are required",
		},
		{
			name: "missing evidence promotion plan ref",
			mutate: func(_ *BaseOwnerApprovalContract, evidence *BasePromotionRollbackReviewEvidence) {
				evidence.PromotionPlanRef = "\t"
			},
			wantErr: "review refs are required",
		},
		{
			name: "missing evidence rollback plan ref",
			mutate: func(_ *BaseOwnerApprovalContract, evidence *BasePromotionRollbackReviewEvidence) {
				evidence.RollbackPlanRef = ""
			},
			wantErr: "review refs are required",
		},
		{
			name: "missing evidence promotion risk review ref",
			mutate: func(_ *BaseOwnerApprovalContract, evidence *BasePromotionRollbackReviewEvidence) {
				evidence.PromotionRiskReviewRef = "  "
			},
			wantErr: "review refs are required",
		},
		{
			name: "missing evidence ledger freshness check ref",
			mutate: func(_ *BaseOwnerApprovalContract, evidence *BasePromotionRollbackReviewEvidence) {
				evidence.LedgerFreshnessCheckRef = "\t"
			},
			wantErr: "review refs are required",
		},
		{
			name: "missing evidence route continuity check ref",
			mutate: func(_ *BaseOwnerApprovalContract, evidence *BasePromotionRollbackReviewEvidence) {
				evidence.RouteContinuityCheckRef = ""
			},
			wantErr: "review refs are required",
		},
		{
			name: "missing evidence operator review policy ref",
			mutate: func(_ *BaseOwnerApprovalContract, evidence *BasePromotionRollbackReviewEvidence) {
				evidence.OperatorReviewPolicyRef = "  "
			},
			wantErr: "review refs are required",
		},
		{
			name: "evidence missing no promotion mutation flag",
			mutate: func(_ *BaseOwnerApprovalContract, evidence *BasePromotionRollbackReviewEvidence) {
				evidence.NoPromotionMutation = false
			},
			wantErr: "evidence must prove no promotion, package publication, run-acceptance, or production mutation",
		},
		{
			name: "evidence missing no package publication mutation flag",
			mutate: func(_ *BaseOwnerApprovalContract, evidence *BasePromotionRollbackReviewEvidence) {
				evidence.NoPackagePublicationMutation = false
			},
			wantErr: "evidence must prove no promotion, package publication, run-acceptance, or production mutation",
		},
		{
			name: "evidence missing no run acceptance mutation flag",
			mutate: func(_ *BaseOwnerApprovalContract, evidence *BasePromotionRollbackReviewEvidence) {
				evidence.NoRunAcceptanceMutation = false
			},
			wantErr: "evidence must prove no promotion, package publication, run-acceptance, or production mutation",
		},
		{
			name: "evidence missing no production mutation flag",
			mutate: func(_ *BaseOwnerApprovalContract, evidence *BasePromotionRollbackReviewEvidence) {
				evidence.NoProductionMutation = false
			},
			wantErr: "evidence must prove no promotion, package publication, run-acceptance, or production mutation",
		},
		{
			name: "evidence claims promotion executed",
			mutate: func(_ *BaseOwnerApprovalContract, evidence *BasePromotionRollbackReviewEvidence) {
				evidence.PromotionExecuted = true
			},
			wantErr: "evidence carries downstream execution or completion claims",
		},
		{
			name: "evidence claims package published",
			mutate: func(_ *BaseOwnerApprovalContract, evidence *BasePromotionRollbackReviewEvidence) {
				evidence.PackagePublished = true
			},
			wantErr: "evidence carries downstream execution or completion claims",
		},
		{
			name: "evidence claims run acceptance touched",
			mutate: func(_ *BaseOwnerApprovalContract, evidence *BasePromotionRollbackReviewEvidence) {
				evidence.RunAcceptanceRecordTouched = true
			},
			wantErr: "evidence carries downstream execution or completion claims",
		},
		{
			name: "evidence claims full substrate",
			mutate: func(_ *BaseOwnerApprovalContract, evidence *BasePromotionRollbackReviewEvidence) {
				evidence.FullSubstrateClaimed = true
			},
			wantErr: "evidence carries downstream execution or completion claims",
		},
		{
			name: "evidence claims completion",
			mutate: func(_ *BaseOwnerApprovalContract, evidence *BasePromotionRollbackReviewEvidence) {
				evidence.CompletionClaimed = true
			},
			wantErr: "evidence carries downstream execution or completion claims",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			owner, evidence := basePromotionRollbackReviewContractInputs(t)
			tc.mutate(&owner, &evidence)

			contract, err := BuildBasePromotionRollbackReviewContract(owner, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildBasePromotionRollbackReviewContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func basePromotionRollbackReviewContractInputs(t *testing.T) (BaseOwnerApprovalContract, BasePromotionRollbackReviewEvidence) {
	t.Helper()

	ownerReview, verifier, approvalEvidence := baseOwnerApprovalContractInputs(t, BaseOwnerDecisionApprove)
	owner, err := BuildBaseOwnerApprovalContract(ownerReview, verifier, approvalEvidence)
	if err != nil {
		t.Fatalf("BuildBaseOwnerApprovalContract(): %v", err)
	}
	return owner, basePromotionRollbackReviewEvidence(owner)
}

func basePromotionRollbackReviewEvidence(owner BaseOwnerApprovalContract) BasePromotionRollbackReviewEvidence {
	return BasePromotionRollbackReviewEvidence{
		OwnerApprovalRef:             " owner-approval:base-pass-111 ",
		PromotionPlanRef:             " promotion-plan:base-pass-111 ",
		RollbackPlanRef:              " " + owner.RollbackPlanRef + " ",
		PromotionRiskReviewRef:       " promotion-risk-review:base-pass-111 ",
		LedgerFreshnessCheckRef:      " ledger-freshness:base-pass-111 ",
		RouteContinuityCheckRef:      " route-continuity:base-pass-111 ",
		OperatorReviewPolicyRef:      " operator-review-policy:base-pass-111 ",
		NoPromotionMutation:          true,
		NoPackagePublicationMutation: true,
		NoRunAcceptanceMutation:      true,
		NoProductionMutation:         true,
		PromotionExecuted:            false,
		PackagePublished:             false,
		RunAcceptanceRecordTouched:   false,
		FullSubstrateClaimed:         false,
		CompletionClaimed:            false,
	}
}
