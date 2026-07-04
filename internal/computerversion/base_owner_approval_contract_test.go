package computerversion

import (
	"strings"
	"testing"
)

func TestBuildBaseOwnerApprovalContractRecordsApprovedOwnerDecisionOnly(t *testing.T) {
	owner, verifier, evidence := baseOwnerApprovalContractInputs(t, BaseOwnerDecisionApprove)

	contract, err := BuildBaseOwnerApprovalContract(owner, verifier, evidence)
	if err != nil {
		t.Fatalf("BuildBaseOwnerApprovalContract(): %v", err)
	}

	if contract.Kind != BaseOwnerApprovalContractKind {
		t.Fatalf("kind = %q, want %q", contract.Kind, BaseOwnerApprovalContractKind)
	}
	if contract.Boundary != BaseOwnerApprovalBoundary {
		t.Fatalf("boundary = %q, want %q", contract.Boundary, BaseOwnerApprovalBoundary)
	}
	if contract.Scope != BaseOwnerApprovalScope {
		t.Fatalf("scope = %q, want %q", contract.Scope, BaseOwnerApprovalScope)
	}
	if contract.Version != owner.Version || ArtifactProgramRef(contract.TypedArtifactProgramRef) != owner.Version.ArtifactProgramRef {
		t.Fatalf("version/ref = %#v/%q, want owner version %#v and artifact program %q", contract.Version, contract.TypedArtifactProgramRef, owner.Version, owner.Version.ArtifactProgramRef)
	}
	if contract.OwnerReviewReadinessRef != strings.TrimSpace(evidence.OwnerReviewReadinessRef) || contract.ReviewPacketRef != strings.TrimSpace(owner.ReviewPacketRef) || contract.VerifierResultRef != strings.TrimSpace(verifier.VerifierResultRef) {
		t.Fatalf("input refs = owner-readiness %q review-packet %q verifier-result %q, want trimmed owner/verifier/evidence refs", contract.OwnerReviewReadinessRef, contract.ReviewPacketRef, contract.VerifierResultRef)
	}
	if contract.OwnerDecisionRef != strings.TrimSpace(evidence.OwnerDecisionRef) || contract.OwnerIdentityRef != strings.TrimSpace(evidence.OwnerIdentityRef) || contract.RollbackPlanRef != strings.TrimSpace(evidence.RollbackPlanRef) {
		t.Fatalf("decision refs = decision %q owner %q rollback %q, want trimmed evidence refs from %#v", contract.OwnerDecisionRef, contract.OwnerIdentityRef, contract.RollbackPlanRef, evidence)
	}
	if contract.OwnerDecision != BaseOwnerDecisionApprove || contract.RejectionReason != "" || !contract.OwnerApprovalRecorded || !contract.OwnerApproved || contract.OwnerRejected || contract.OwnerRejectionBlocksDownstream {
		t.Fatalf("approval decision = decision %q reason %q recorded %v approved %v rejected %v blocks %v, want local approval evidence without rejection blocking", contract.OwnerDecision, contract.RejectionReason, contract.OwnerApprovalRecorded, contract.OwnerApproved, contract.OwnerRejected, contract.OwnerRejectionBlocksDownstream)
	}
	if !contract.VerifierContractSatisfied {
		t.Fatalf("approval contract must consume a satisfied verifier result: %#v", contract)
	}
	if !contract.PromotionRollbackReviewRequired || !contract.PackagePublicationProofRequired || !contract.RunAcceptanceProofRequired || !contract.FullSubstrateProofRequired {
		t.Fatalf("approval must preserve promotion, publication, run-acceptance, and full-substrate proof requirements: %#v", contract)
	}
	if contract.PromotionAllowed || contract.PackagePublicationAllowed || contract.RunAcceptanceSynthesisAllowed {
		t.Fatalf("approval must not allow promotion, publication, or run-acceptance synthesis: %#v", contract)
	}
	if !contract.NoPromotionMutation || !contract.NoPackagePublicationMutation || !contract.NoRunAcceptanceMutation || !contract.NoProductionMutation {
		t.Fatalf("approval must carry no-promotion/no-package/no-run-acceptance/no-production mutation evidence: %#v", contract)
	}
	if contract.PromotionExecuted || contract.PackagePublished || contract.RunAcceptanceRecordTouched || contract.FullSubstrateClaimed || contract.CompletionClaimed {
		t.Fatalf("approval must not promote, publish, touch run acceptance, claim full substrate, or claim completion: %#v", contract)
	}
}

func TestBuildBaseOwnerApprovalContractRecordsRejectedOwnerDecisionAsBlockingEvidence(t *testing.T) {
	owner, verifier, evidence := baseOwnerApprovalContractInputs(t, BaseOwnerDecisionReject)

	contract, err := BuildBaseOwnerApprovalContract(owner, verifier, evidence)
	if err != nil {
		t.Fatalf("BuildBaseOwnerApprovalContract(): %v", err)
	}

	if contract.OwnerDecision != BaseOwnerDecisionReject || contract.RejectionReason != strings.TrimSpace(evidence.RejectionReason) {
		t.Fatalf("rejection decision = decision %q reason %q, want reject with trimmed reason %q", contract.OwnerDecision, contract.RejectionReason, evidence.RejectionReason)
	}
	if !contract.OwnerApprovalRecorded || contract.OwnerApproved || !contract.OwnerRejected || !contract.OwnerRejectionBlocksDownstream {
		t.Fatalf("rejection must be recorded as blocking owner evidence without approval: %#v", contract)
	}
	if !contract.VerifierContractSatisfied {
		t.Fatalf("rejection contract must still consume a satisfied verifier result: %#v", contract)
	}
	if !contract.PromotionRollbackReviewRequired || !contract.PackagePublicationProofRequired || !contract.RunAcceptanceProofRequired || !contract.FullSubstrateProofRequired {
		t.Fatalf("rejection must preserve promotion, publication, run-acceptance, and full-substrate proof requirements: %#v", contract)
	}
	if contract.PromotionAllowed || contract.PackagePublicationAllowed || contract.RunAcceptanceSynthesisAllowed {
		t.Fatalf("rejection must not allow promotion, publication, or run-acceptance synthesis: %#v", contract)
	}
	if contract.PromotionExecuted || contract.PackagePublished || contract.RunAcceptanceRecordTouched || contract.FullSubstrateClaimed || contract.CompletionClaimed {
		t.Fatalf("rejection must not promote, publish, touch run acceptance, claim full substrate, or claim completion: %#v", contract)
	}
}

func TestBuildBaseOwnerApprovalContractRejectsInvalidInputs(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*BaseOwnerReviewReadinessContract, *BaseVerifierResultContract, *BaseOwnerApprovalEvidence)
		wantErr string
	}{
		{
			name: "owner-review wrong kind",
			mutate: func(owner *BaseOwnerReviewReadinessContract, _ *BaseVerifierResultContract, _ *BaseOwnerApprovalEvidence) {
				owner.Kind = BaseVerifierResultContractKind
			},
			wantErr: "owner-review kind",
		},
		{
			name: "owner-review boundary drift",
			mutate: func(owner *BaseOwnerReviewReadinessContract, _ *BaseVerifierResultContract, _ *BaseOwnerApprovalEvidence) {
				owner.Boundary = BaseVerifierReadinessBoundary
			},
			wantErr: "owner-review boundary",
		},
		{
			name: "owner-review scope drift",
			mutate: func(owner *BaseOwnerReviewReadinessContract, _ *BaseVerifierResultContract, _ *BaseOwnerApprovalEvidence) {
				owner.Scope = BaseVerifierReadinessScope
			},
			wantErr: "owner-review scope",
		},
		{
			name: "owner-review typed artifact ref drift",
			mutate: func(owner *BaseOwnerReviewReadinessContract, _ *BaseVerifierResultContract, _ *BaseOwnerApprovalEvidence) {
				owner.TypedArtifactProgramRef = "artifact-program:base-different"
			},
			wantErr: "owner-review typed artifact-program ref is invalid",
		},
		{
			name: "owner-review readiness drift",
			mutate: func(owner *BaseOwnerReviewReadinessContract, _ *BaseVerifierResultContract, _ *BaseOwnerApprovalEvidence) {
				owner.ReadinessStatus = "blocked_before_owner_decision"
			},
			wantErr: "owner-review must be ready but not approved",
		},
		{
			name: "verifier result version drift",
			mutate: func(_ *BaseOwnerReviewReadinessContract, verifier *BaseVerifierResultContract, _ *BaseOwnerApprovalEvidence) {
				verifier.Version.CodeRef = "code:base-pass-different"
			},
			wantErr: "verifier result does not match owner-review version",
		},
		{
			name: "verifier result typed artifact ref drift",
			mutate: func(_ *BaseOwnerReviewReadinessContract, verifier *BaseVerifierResultContract, _ *BaseOwnerApprovalEvidence) {
				verifier.TypedArtifactProgramRef = "artifact-program:base-different"
			},
			wantErr: "verifier result does not match owner-review version",
		},
		{
			name: "failing verifier result rejected",
			mutate: func(_ *BaseOwnerReviewReadinessContract, verifier *BaseVerifierResultContract, _ *BaseOwnerApprovalEvidence) {
				verifier.Verdict = BaseVerifierVerdictFail
				verifier.FailureReason = " verifier failed base proof "
				verifier.VerifierContractSatisfied = false
				verifier.VerifierContractFailed = true
				verifier.VerifierFailureBlocksDownstream = true
			},
			wantErr: "verifier result must be a passing verifier result",
		},
		{
			name: "missing owner-review readiness ref",
			mutate: func(_ *BaseOwnerReviewReadinessContract, _ *BaseVerifierResultContract, evidence *BaseOwnerApprovalEvidence) {
				evidence.OwnerReviewReadinessRef = "  "
			},
			wantErr: "owner decision refs are required",
		},
		{
			name: "missing review packet ref",
			mutate: func(_ *BaseOwnerReviewReadinessContract, _ *BaseVerifierResultContract, evidence *BaseOwnerApprovalEvidence) {
				evidence.ReviewPacketRef = "\t"
			},
			wantErr: "owner decision refs are required",
		},
		{
			name: "missing verifier result ref",
			mutate: func(_ *BaseOwnerReviewReadinessContract, _ *BaseVerifierResultContract, evidence *BaseOwnerApprovalEvidence) {
				evidence.VerifierResultRef = ""
			},
			wantErr: "owner decision refs are required",
		},
		{
			name: "missing owner decision ref",
			mutate: func(_ *BaseOwnerReviewReadinessContract, _ *BaseVerifierResultContract, evidence *BaseOwnerApprovalEvidence) {
				evidence.OwnerDecisionRef = "  "
			},
			wantErr: "owner decision refs are required",
		},
		{
			name: "missing owner identity ref",
			mutate: func(_ *BaseOwnerReviewReadinessContract, _ *BaseVerifierResultContract, evidence *BaseOwnerApprovalEvidence) {
				evidence.OwnerIdentityRef = "\t"
			},
			wantErr: "owner decision refs are required",
		},
		{
			name: "missing rollback plan ref",
			mutate: func(_ *BaseOwnerReviewReadinessContract, _ *BaseVerifierResultContract, evidence *BaseOwnerApprovalEvidence) {
				evidence.RollbackPlanRef = ""
			},
			wantErr: "owner decision refs are required",
		},
		{
			name: "mismatched review packet ref",
			mutate: func(_ *BaseOwnerReviewReadinessContract, _ *BaseVerifierResultContract, evidence *BaseOwnerApprovalEvidence) {
				evidence.ReviewPacketRef = "owner-review-packet:other"
			},
			wantErr: "review packet ref does not match owner review",
		},
		{
			name: "mismatched verifier result ref",
			mutate: func(_ *BaseOwnerReviewReadinessContract, _ *BaseVerifierResultContract, evidence *BaseOwnerApprovalEvidence) {
				evidence.VerifierResultRef = "verifier-result:other"
			},
			wantErr: "verifier result ref does not match verifier result",
		},
		{
			name: "invalid owner decision",
			mutate: func(_ *BaseOwnerReviewReadinessContract, _ *BaseVerifierResultContract, evidence *BaseOwnerApprovalEvidence) {
				evidence.OwnerDecision = "defer"
			},
			wantErr: "owner decision must be approve or reject",
		},
		{
			name: "approval with rejection reason",
			mutate: func(_ *BaseOwnerReviewReadinessContract, _ *BaseVerifierResultContract, evidence *BaseOwnerApprovalEvidence) {
				evidence.OwnerDecision = BaseOwnerDecisionApprove
				evidence.RejectionReason = " cannot approve this base "
			},
			wantErr: "approval cannot include a rejection reason",
		},
		{
			name: "rejection without reason",
			mutate: func(_ *BaseOwnerReviewReadinessContract, _ *BaseVerifierResultContract, evidence *BaseOwnerApprovalEvidence) {
				evidence.OwnerDecision = BaseOwnerDecisionReject
				evidence.RejectionReason = "\t"
			},
			wantErr: "rejection requires a reason",
		},
		{
			name: "owner-review missing no promotion mutation flag",
			mutate: func(owner *BaseOwnerReviewReadinessContract, _ *BaseVerifierResultContract, _ *BaseOwnerApprovalEvidence) {
				owner.NoPromotionMutation = false
			},
			wantErr: "owner-review must prove no owner approval, promotion, package publication, run-acceptance, or production mutation",
		},
		{
			name: "verifier result missing no run acceptance mutation flag",
			mutate: func(_ *BaseOwnerReviewReadinessContract, verifier *BaseVerifierResultContract, _ *BaseOwnerApprovalEvidence) {
				verifier.NoRunAcceptanceMutation = false
			},
			wantErr: "verifier result must prove no owner approval, promotion, package publication, run-acceptance, or production mutation",
		},
		{
			name: "evidence missing no promotion mutation flag",
			mutate: func(_ *BaseOwnerReviewReadinessContract, _ *BaseVerifierResultContract, evidence *BaseOwnerApprovalEvidence) {
				evidence.NoPromotionMutation = false
			},
			wantErr: "evidence must prove no promotion, package publication, run-acceptance, or production mutation",
		},
		{
			name: "evidence missing no package publication mutation flag",
			mutate: func(_ *BaseOwnerReviewReadinessContract, _ *BaseVerifierResultContract, evidence *BaseOwnerApprovalEvidence) {
				evidence.NoPackagePublicationMutation = false
			},
			wantErr: "evidence must prove no promotion, package publication, run-acceptance, or production mutation",
		},
		{
			name: "evidence missing no run acceptance mutation flag",
			mutate: func(_ *BaseOwnerReviewReadinessContract, _ *BaseVerifierResultContract, evidence *BaseOwnerApprovalEvidence) {
				evidence.NoRunAcceptanceMutation = false
			},
			wantErr: "evidence must prove no promotion, package publication, run-acceptance, or production mutation",
		},
		{
			name: "evidence missing no production mutation flag",
			mutate: func(_ *BaseOwnerReviewReadinessContract, _ *BaseVerifierResultContract, evidence *BaseOwnerApprovalEvidence) {
				evidence.NoProductionMutation = false
			},
			wantErr: "evidence must prove no promotion, package publication, run-acceptance, or production mutation",
		},
		{
			name: "evidence claims promotion executed",
			mutate: func(_ *BaseOwnerReviewReadinessContract, _ *BaseVerifierResultContract, evidence *BaseOwnerApprovalEvidence) {
				evidence.PromotionExecuted = true
			},
			wantErr: "evidence carries downstream execution or completion claims",
		},
		{
			name: "evidence claims package published",
			mutate: func(_ *BaseOwnerReviewReadinessContract, _ *BaseVerifierResultContract, evidence *BaseOwnerApprovalEvidence) {
				evidence.PackagePublished = true
			},
			wantErr: "evidence carries downstream execution or completion claims",
		},
		{
			name: "evidence claims run acceptance touched",
			mutate: func(_ *BaseOwnerReviewReadinessContract, _ *BaseVerifierResultContract, evidence *BaseOwnerApprovalEvidence) {
				evidence.RunAcceptanceRecordTouched = true
			},
			wantErr: "evidence carries downstream execution or completion claims",
		},
		{
			name: "evidence claims full substrate",
			mutate: func(_ *BaseOwnerReviewReadinessContract, _ *BaseVerifierResultContract, evidence *BaseOwnerApprovalEvidence) {
				evidence.FullSubstrateClaimed = true
			},
			wantErr: "evidence carries downstream execution or completion claims",
		},
		{
			name: "evidence claims completion",
			mutate: func(_ *BaseOwnerReviewReadinessContract, _ *BaseVerifierResultContract, evidence *BaseOwnerApprovalEvidence) {
				evidence.CompletionClaimed = true
			},
			wantErr: "evidence carries downstream execution or completion claims",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			owner, verifier, evidence := baseOwnerApprovalContractInputs(t, BaseOwnerDecisionApprove)
			tc.mutate(&owner, &verifier, &evidence)

			contract, err := BuildBaseOwnerApprovalContract(owner, verifier, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildBaseOwnerApprovalContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func baseOwnerApprovalContractInputs(t *testing.T, decision string) (BaseOwnerReviewReadinessContract, BaseVerifierResultContract, BaseOwnerApprovalEvidence) {
	t.Helper()

	owner, verifierReadinessEvidence := baseVerifierReadinessContractInputs(t)
	verifierReadiness, err := BuildBaseVerifierReadinessContract(owner, verifierReadinessEvidence)
	if err != nil {
		t.Fatalf("BuildBaseVerifierReadinessContract(): %v", err)
	}

	verifierEvidence := BaseVerifierResultEvidence{
		VerifierReadinessRef:         " verifier-readiness:base-pass-108 ",
		VerifierRunRef:               " verifier-run:base-pass-109 ",
		VerifierResultRef:            " verifier-result:base-pass-109 ",
		VerifierLogRef:               " verifier-log:base-pass-109 ",
		Verdict:                      BaseVerifierVerdictPass,
		RollbackPlanRef:              " " + verifierReadiness.RollbackPlanRef + " ",
		NoOwnerApprovalMutation:      true,
		NoPromotionMutation:          true,
		NoPackagePublicationMutation: true,
		NoRunAcceptanceMutation:      true,
		NoProductionMutation:         true,
		VerifierContractSatisfied:    false,
		OwnerApproved:                false,
		PromotionExecuted:            false,
		PackagePublished:             false,
		RunAcceptanceRecordTouched:   false,
		FullSubstrateClaimed:         false,
		CompletionClaimed:            false,
	}
	verifier, err := BuildBaseVerifierResultContract(verifierReadiness, verifierEvidence)
	if err != nil {
		t.Fatalf("BuildBaseVerifierResultContract(): %v", err)
	}

	evidence := BaseOwnerApprovalEvidence{
		OwnerReviewReadinessRef:      " owner-review-readiness:base-pass-107 ",
		ReviewPacketRef:              " " + owner.ReviewPacketRef + " ",
		VerifierResultRef:            " " + verifier.VerifierResultRef + " ",
		OwnerDecisionRef:             " owner-decision:base-pass-110 ",
		OwnerIdentityRef:             " owner-identity:release-owner ",
		OwnerDecision:                decision,
		RollbackPlanRef:              " " + owner.RollbackPlanRef + " ",
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
	if decision == BaseOwnerDecisionReject {
		evidence.RejectionReason = " owner rejects until rollback packet is clarified "
	}
	return owner, verifier, evidence
}
