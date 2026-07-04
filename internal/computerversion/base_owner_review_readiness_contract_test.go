package computerversion

import (
	"strings"
	"testing"
)

func TestBuildBaseOwnerReviewReadinessContractCreatesReviewPacketOnly(t *testing.T) {
	handoff, evidence := baseOwnerReviewReadinessContractInputs(t)

	contract, err := BuildBaseOwnerReviewReadinessContract(handoff, evidence)
	if err != nil {
		t.Fatalf("BuildBaseOwnerReviewReadinessContract(): %v", err)
	}

	if contract.Kind != BaseOwnerReviewReadinessContractKind {
		t.Fatalf("kind = %q, want %q", contract.Kind, BaseOwnerReviewReadinessContractKind)
	}
	if contract.Boundary != BaseOwnerReviewReadinessBoundary {
		t.Fatalf("boundary = %q, want %q", contract.Boundary, BaseOwnerReviewReadinessBoundary)
	}
	if contract.Scope != BaseOwnerReviewReadinessScope {
		t.Fatalf("scope = %q, want %q", contract.Scope, BaseOwnerReviewReadinessScope)
	}
	if contract.Version != handoff.Version || ArtifactProgramRef(contract.TypedArtifactProgramRef) != handoff.Version.ArtifactProgramRef {
		t.Fatalf("version/ref = %#v/%q, want handoff version %#v and artifact program %q", contract.Version, contract.TypedArtifactProgramRef, handoff.Version, handoff.Version.ArtifactProgramRef)
	}
	if contract.ProductPathProbeRef != strings.TrimSpace(handoff.ProductPathProbeRef) || contract.BuildIdentity != strings.TrimSpace(handoff.BuildIdentity) || contract.RouteIdentity != strings.TrimSpace(handoff.RouteIdentity) {
		t.Fatalf("product probe/build/route = %q/%q/%q, want trimmed handoff identity %q/%q/%q", contract.ProductPathProbeRef, contract.BuildIdentity, contract.RouteIdentity, handoff.ProductPathProbeRef, handoff.BuildIdentity, handoff.RouteIdentity)
	}
	if contract.PostSmokeHandoffRef != strings.TrimSpace(evidence.PostSmokeHandoffRef) || contract.ReviewPacketRef != strings.TrimSpace(evidence.ReviewPacketRef) || contract.ReviewerIdentityPolicyRef != strings.TrimSpace(evidence.ReviewerIdentityPolicyRef) || contract.OwnerReviewInstructionsRef != strings.TrimSpace(evidence.OwnerReviewInstructionsRef) || contract.RiskSummaryRef != strings.TrimSpace(evidence.RiskSummaryRef) || contract.RollbackPlanRef != strings.TrimSpace(evidence.RollbackPlanRef) {
		t.Fatalf("review refs = handoff %q packet %q reviewer-policy %q instructions %q risk %q rollback %q, want trimmed evidence refs from %#v", contract.PostSmokeHandoffRef, contract.ReviewPacketRef, contract.ReviewerIdentityPolicyRef, contract.OwnerReviewInstructionsRef, contract.RiskSummaryRef, contract.RollbackPlanRef, evidence)
	}
	if contract.ReadinessStatus != BaseOwnerReviewReadinessStatusReady || !contract.OwnerReviewReady || contract.OwnerApproved {
		t.Fatalf("owner review readiness = status %q ready %v approved %v, want ready for owner review without approval", contract.ReadinessStatus, contract.OwnerReviewReady, contract.OwnerApproved)
	}
	if !contract.OwnerApprovalRequired || !contract.PromotionRollbackReviewRequired || !contract.PackagePublicationProofRequired || !contract.VerifierContractProofRequired || !contract.RunAcceptanceProofRequired || !contract.FullSubstrateProofRequired {
		t.Fatalf("contract must require owner approval, promotion/rollback review, package publication proof, verifier contract proof, run-acceptance proof, and full-substrate proof: %#v", contract)
	}
	if contract.OwnerApprovalAllowed || contract.PromotionAllowed || contract.PackagePublicationAllowed || contract.VerifierContractSatisfactionAllowed || contract.RunAcceptanceSynthesisAllowed {
		t.Fatalf("contract must not allow owner approval, promotion, package publication, verifier satisfaction, or run-acceptance synthesis execution: %#v", contract)
	}
	if !contract.NoOwnerApprovalMutation || !contract.NoPromotionMutation || !contract.NoPackagePublicationMutation || !contract.NoRunAcceptanceMutation || !contract.NoProductionMutation {
		t.Fatalf("contract must carry no-owner-approval/no-promotion/no-package/no-run-acceptance/no-production mutation flags: %#v", contract)
	}
	if contract.OwnerApproved || contract.PromotionExecuted || contract.PackagePublished || contract.VerifierContractSatisfied || contract.RunAcceptanceRecordTouched || contract.FullSubstrateClaimed || contract.CompletionClaimed {
		t.Fatalf("contract must not approve, promote, publish, satisfy verifier contract, touch run acceptance, claim full substrate, or claim completion: %#v", contract)
	}
}

func TestBuildBaseOwnerReviewReadinessContractRejectsInvalidInputs(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*BasePostSmokeHandoffReadinessContract, *BaseOwnerReviewReadinessEvidence)
		wantErr string
	}{
		{
			name: "handoff wrong kind",
			mutate: func(handoff *BasePostSmokeHandoffReadinessContract, _ *BaseOwnerReviewReadinessEvidence) {
				handoff.Kind = BaseOwnerReviewReadinessContractKind
			},
			wantErr: "handoff kind",
		},
		{
			name: "handoff boundary drift",
			mutate: func(handoff *BasePostSmokeHandoffReadinessContract, _ *BaseOwnerReviewReadinessEvidence) {
				handoff.Boundary = BaseOwnerReviewReadinessBoundary
			},
			wantErr: "handoff boundary",
		},
		{
			name: "handoff scope drift",
			mutate: func(handoff *BasePostSmokeHandoffReadinessContract, _ *BaseOwnerReviewReadinessEvidence) {
				handoff.Scope = BaseOwnerReviewReadinessScope
			},
			wantErr: "handoff scope",
		},
		{
			name: "invalid handoff version",
			mutate: func(handoff *BasePostSmokeHandoffReadinessContract, _ *BaseOwnerReviewReadinessEvidence) {
				handoff.Version.CodeRef = "  "
			},
			wantErr: "handoff version is invalid",
		},
		{
			name: "invalid handoff typed artifact ref",
			mutate: func(handoff *BasePostSmokeHandoffReadinessContract, _ *BaseOwnerReviewReadinessEvidence) {
				handoff.TypedArtifactProgramRef = "artifact-program:base-different"
			},
			wantErr: "handoff typed artifact-program ref is invalid",
		},
		{
			name: "missing handoff staging smoke evidence ref",
			mutate: func(handoff *BasePostSmokeHandoffReadinessContract, _ *BaseOwnerReviewReadinessEvidence) {
				handoff.StagingSmokeEvidenceRef = "\t"
			},
			wantErr: "handoff refs are required",
		},
		{
			name: "missing handoff product path probe ref",
			mutate: func(handoff *BasePostSmokeHandoffReadinessContract, _ *BaseOwnerReviewReadinessEvidence) {
				handoff.ProductPathProbeRef = "  "
			},
			wantErr: "handoff refs are required",
		},
		{
			name: "missing handoff build identity",
			mutate: func(handoff *BasePostSmokeHandoffReadinessContract, _ *BaseOwnerReviewReadinessEvidence) {
				handoff.BuildIdentity = ""
			},
			wantErr: "handoff refs are required",
		},
		{
			name: "missing handoff route identity",
			mutate: func(handoff *BasePostSmokeHandoffReadinessContract, _ *BaseOwnerReviewReadinessEvidence) {
				handoff.RouteIdentity = "\t"
			},
			wantErr: "handoff refs are required",
		},
		{
			name: "missing handoff owner review plan ref",
			mutate: func(handoff *BasePostSmokeHandoffReadinessContract, _ *BaseOwnerReviewReadinessEvidence) {
				handoff.OwnerReviewPlanRef = "  "
			},
			wantErr: "handoff refs are required",
		},
		{
			name: "missing handoff promotion rollback plan ref",
			mutate: func(handoff *BasePostSmokeHandoffReadinessContract, _ *BaseOwnerReviewReadinessEvidence) {
				handoff.PromotionRollbackPlanRef = ""
			},
			wantErr: "handoff refs are required",
		},
		{
			name: "missing handoff package publication plan ref",
			mutate: func(handoff *BasePostSmokeHandoffReadinessContract, _ *BaseOwnerReviewReadinessEvidence) {
				handoff.PackagePublicationPlanRef = "\t"
			},
			wantErr: "handoff refs are required",
		},
		{
			name: "missing handoff verifier contract plan ref",
			mutate: func(handoff *BasePostSmokeHandoffReadinessContract, _ *BaseOwnerReviewReadinessEvidence) {
				handoff.VerifierContractPlanRef = "  "
			},
			wantErr: "handoff refs are required",
		},
		{
			name: "missing handoff run acceptance synthesis plan ref",
			mutate: func(handoff *BasePostSmokeHandoffReadinessContract, _ *BaseOwnerReviewReadinessEvidence) {
				handoff.RunAcceptanceSynthesisPlanRef = ""
			},
			wantErr: "handoff refs are required",
		},
		{
			name: "missing handoff rollback plan ref",
			mutate: func(handoff *BasePostSmokeHandoffReadinessContract, _ *BaseOwnerReviewReadinessEvidence) {
				handoff.RollbackPlanRef = "\t"
			},
			wantErr: "handoff refs are required",
		},
		{
			name: "handoff status not blocked",
			mutate: func(handoff *BasePostSmokeHandoffReadinessContract, _ *BaseOwnerReviewReadinessEvidence) {
				handoff.ReadinessStatus = BaseOwnerReviewReadinessStatusReady
			},
			wantErr: "handoff must remain blocked",
		},
		{
			name: "handoff missing owner review requirement",
			mutate: func(handoff *BasePostSmokeHandoffReadinessContract, _ *BaseOwnerReviewReadinessEvidence) {
				handoff.OwnerReviewRequired = false
			},
			wantErr: "handoff must preserve downstream proof requirements",
		},
		{
			name: "handoff missing promotion rollback review requirement",
			mutate: func(handoff *BasePostSmokeHandoffReadinessContract, _ *BaseOwnerReviewReadinessEvidence) {
				handoff.PromotionRollbackReviewRequired = false
			},
			wantErr: "handoff must preserve downstream proof requirements",
		},
		{
			name: "handoff missing package publication proof requirement",
			mutate: func(handoff *BasePostSmokeHandoffReadinessContract, _ *BaseOwnerReviewReadinessEvidence) {
				handoff.PackagePublicationProofRequired = false
			},
			wantErr: "handoff must preserve downstream proof requirements",
		},
		{
			name: "handoff missing verifier contract proof requirement",
			mutate: func(handoff *BasePostSmokeHandoffReadinessContract, _ *BaseOwnerReviewReadinessEvidence) {
				handoff.VerifierContractProofRequired = false
			},
			wantErr: "handoff must preserve downstream proof requirements",
		},
		{
			name: "handoff missing run acceptance proof requirement",
			mutate: func(handoff *BasePostSmokeHandoffReadinessContract, _ *BaseOwnerReviewReadinessEvidence) {
				handoff.RunAcceptanceProofRequired = false
			},
			wantErr: "handoff must preserve downstream proof requirements",
		},
		{
			name: "handoff missing full substrate proof requirement",
			mutate: func(handoff *BasePostSmokeHandoffReadinessContract, _ *BaseOwnerReviewReadinessEvidence) {
				handoff.FullSubstrateProofRequired = false
			},
			wantErr: "handoff must preserve downstream proof requirements",
		},
		{
			name: "handoff allows owner approval",
			mutate: func(handoff *BasePostSmokeHandoffReadinessContract, _ *BaseOwnerReviewReadinessEvidence) {
				handoff.OwnerApprovalAllowed = true
			},
			wantErr: "handoff allows downstream execution",
		},
		{
			name: "handoff allows promotion",
			mutate: func(handoff *BasePostSmokeHandoffReadinessContract, _ *BaseOwnerReviewReadinessEvidence) {
				handoff.PromotionAllowed = true
			},
			wantErr: "handoff allows downstream execution",
		},
		{
			name: "handoff allows package publication",
			mutate: func(handoff *BasePostSmokeHandoffReadinessContract, _ *BaseOwnerReviewReadinessEvidence) {
				handoff.PackagePublicationAllowed = true
			},
			wantErr: "handoff allows downstream execution",
		},
		{
			name: "handoff allows run acceptance synthesis",
			mutate: func(handoff *BasePostSmokeHandoffReadinessContract, _ *BaseOwnerReviewReadinessEvidence) {
				handoff.RunAcceptanceSynthesisAllowed = true
			},
			wantErr: "handoff allows downstream execution",
		},
		{
			name: "handoff missing no owner approval mutation flag",
			mutate: func(handoff *BasePostSmokeHandoffReadinessContract, _ *BaseOwnerReviewReadinessEvidence) {
				handoff.NoOwnerApprovalMutation = false
			},
			wantErr: "handoff must prove no owner approval, promotion, package publication, run-acceptance, or production mutation",
		},
		{
			name: "handoff missing no promotion mutation flag",
			mutate: func(handoff *BasePostSmokeHandoffReadinessContract, _ *BaseOwnerReviewReadinessEvidence) {
				handoff.NoPromotionMutation = false
			},
			wantErr: "handoff must prove no owner approval, promotion, package publication, run-acceptance, or production mutation",
		},
		{
			name: "handoff missing no package publication mutation flag",
			mutate: func(handoff *BasePostSmokeHandoffReadinessContract, _ *BaseOwnerReviewReadinessEvidence) {
				handoff.NoPackagePublicationMutation = false
			},
			wantErr: "handoff must prove no owner approval, promotion, package publication, run-acceptance, or production mutation",
		},
		{
			name: "handoff missing no run acceptance mutation flag",
			mutate: func(handoff *BasePostSmokeHandoffReadinessContract, _ *BaseOwnerReviewReadinessEvidence) {
				handoff.NoRunAcceptanceMutation = false
			},
			wantErr: "handoff must prove no owner approval, promotion, package publication, run-acceptance, or production mutation",
		},
		{
			name: "handoff missing no production mutation flag",
			mutate: func(handoff *BasePostSmokeHandoffReadinessContract, _ *BaseOwnerReviewReadinessEvidence) {
				handoff.NoProductionMutation = false
			},
			wantErr: "handoff must prove no owner approval, promotion, package publication, run-acceptance, or production mutation",
		},
		{
			name: "handoff owner approved",
			mutate: func(handoff *BasePostSmokeHandoffReadinessContract, _ *BaseOwnerReviewReadinessEvidence) {
				handoff.OwnerApproved = true
			},
			wantErr: "handoff carries downstream execution or completion claims",
		},
		{
			name: "handoff promotion executed",
			mutate: func(handoff *BasePostSmokeHandoffReadinessContract, _ *BaseOwnerReviewReadinessEvidence) {
				handoff.PromotionExecuted = true
			},
			wantErr: "handoff carries downstream execution or completion claims",
		},
		{
			name: "handoff package published",
			mutate: func(handoff *BasePostSmokeHandoffReadinessContract, _ *BaseOwnerReviewReadinessEvidence) {
				handoff.PackagePublished = true
			},
			wantErr: "handoff carries downstream execution or completion claims",
		},
		{
			name: "handoff verifier satisfied",
			mutate: func(handoff *BasePostSmokeHandoffReadinessContract, _ *BaseOwnerReviewReadinessEvidence) {
				handoff.VerifierContractSatisfied = true
			},
			wantErr: "handoff carries downstream execution or completion claims",
		},
		{
			name: "handoff run acceptance touched",
			mutate: func(handoff *BasePostSmokeHandoffReadinessContract, _ *BaseOwnerReviewReadinessEvidence) {
				handoff.RunAcceptanceRecordTouched = true
			},
			wantErr: "handoff carries downstream execution or completion claims",
		},
		{
			name: "handoff full substrate claimed",
			mutate: func(handoff *BasePostSmokeHandoffReadinessContract, _ *BaseOwnerReviewReadinessEvidence) {
				handoff.FullSubstrateClaimed = true
			},
			wantErr: "handoff carries downstream execution or completion claims",
		},
		{
			name: "handoff completion claimed",
			mutate: func(handoff *BasePostSmokeHandoffReadinessContract, _ *BaseOwnerReviewReadinessEvidence) {
				handoff.CompletionClaimed = true
			},
			wantErr: "handoff carries downstream execution or completion claims",
		},
		{
			name: "missing owner-review post-smoke handoff ref",
			mutate: func(_ *BasePostSmokeHandoffReadinessContract, evidence *BaseOwnerReviewReadinessEvidence) {
				evidence.PostSmokeHandoffRef = "  "
			},
			wantErr: "review packet refs are required",
		},
		{
			name: "missing owner-review packet ref",
			mutate: func(_ *BasePostSmokeHandoffReadinessContract, evidence *BaseOwnerReviewReadinessEvidence) {
				evidence.ReviewPacketRef = "\t"
			},
			wantErr: "review packet refs are required",
		},
		{
			name: "missing owner-review reviewer identity policy ref",
			mutate: func(_ *BasePostSmokeHandoffReadinessContract, evidence *BaseOwnerReviewReadinessEvidence) {
				evidence.ReviewerIdentityPolicyRef = ""
			},
			wantErr: "review packet refs are required",
		},
		{
			name: "missing owner-review instructions ref",
			mutate: func(_ *BasePostSmokeHandoffReadinessContract, evidence *BaseOwnerReviewReadinessEvidence) {
				evidence.OwnerReviewInstructionsRef = "  "
			},
			wantErr: "review packet refs are required",
		},
		{
			name: "missing owner-review risk summary ref",
			mutate: func(_ *BasePostSmokeHandoffReadinessContract, evidence *BaseOwnerReviewReadinessEvidence) {
				evidence.RiskSummaryRef = "\t"
			},
			wantErr: "review packet refs are required",
		},
		{
			name: "missing owner-review rollback plan ref",
			mutate: func(_ *BasePostSmokeHandoffReadinessContract, evidence *BaseOwnerReviewReadinessEvidence) {
				evidence.RollbackPlanRef = ""
			},
			wantErr: "review packet refs are required",
		},
		{
			name: "missing owner-review no owner approval mutation flag",
			mutate: func(_ *BasePostSmokeHandoffReadinessContract, evidence *BaseOwnerReviewReadinessEvidence) {
				evidence.NoOwnerApprovalMutation = false
			},
			wantErr: "evidence must prove no owner approval, promotion, package publication, run-acceptance, or production mutation",
		},
		{
			name: "missing owner-review no promotion mutation flag",
			mutate: func(_ *BasePostSmokeHandoffReadinessContract, evidence *BaseOwnerReviewReadinessEvidence) {
				evidence.NoPromotionMutation = false
			},
			wantErr: "evidence must prove no owner approval, promotion, package publication, run-acceptance, or production mutation",
		},
		{
			name: "missing owner-review no package publication mutation flag",
			mutate: func(_ *BasePostSmokeHandoffReadinessContract, evidence *BaseOwnerReviewReadinessEvidence) {
				evidence.NoPackagePublicationMutation = false
			},
			wantErr: "evidence must prove no owner approval, promotion, package publication, run-acceptance, or production mutation",
		},
		{
			name: "missing owner-review no run acceptance mutation flag",
			mutate: func(_ *BasePostSmokeHandoffReadinessContract, evidence *BaseOwnerReviewReadinessEvidence) {
				evidence.NoRunAcceptanceMutation = false
			},
			wantErr: "evidence must prove no owner approval, promotion, package publication, run-acceptance, or production mutation",
		},
		{
			name: "missing owner-review no production mutation flag",
			mutate: func(_ *BasePostSmokeHandoffReadinessContract, evidence *BaseOwnerReviewReadinessEvidence) {
				evidence.NoProductionMutation = false
			},
			wantErr: "evidence must prove no owner approval, promotion, package publication, run-acceptance, or production mutation",
		},
		{
			name: "owner approved",
			mutate: func(_ *BasePostSmokeHandoffReadinessContract, evidence *BaseOwnerReviewReadinessEvidence) {
				evidence.OwnerApproved = true
			},
			wantErr: "evidence carries downstream execution or completion claims",
		},
		{
			name: "promotion executed",
			mutate: func(_ *BasePostSmokeHandoffReadinessContract, evidence *BaseOwnerReviewReadinessEvidence) {
				evidence.PromotionExecuted = true
			},
			wantErr: "evidence carries downstream execution or completion claims",
		},
		{
			name: "package published",
			mutate: func(_ *BasePostSmokeHandoffReadinessContract, evidence *BaseOwnerReviewReadinessEvidence) {
				evidence.PackagePublished = true
			},
			wantErr: "evidence carries downstream execution or completion claims",
		},
		{
			name: "verifier satisfied",
			mutate: func(_ *BasePostSmokeHandoffReadinessContract, evidence *BaseOwnerReviewReadinessEvidence) {
				evidence.VerifierContractSatisfied = true
			},
			wantErr: "evidence carries downstream execution or completion claims",
		},
		{
			name: "run acceptance touched",
			mutate: func(_ *BasePostSmokeHandoffReadinessContract, evidence *BaseOwnerReviewReadinessEvidence) {
				evidence.RunAcceptanceRecordTouched = true
			},
			wantErr: "evidence carries downstream execution or completion claims",
		},
		{
			name: "full substrate claimed",
			mutate: func(_ *BasePostSmokeHandoffReadinessContract, evidence *BaseOwnerReviewReadinessEvidence) {
				evidence.FullSubstrateClaimed = true
			},
			wantErr: "evidence carries downstream execution or completion claims",
		},
		{
			name: "completion claimed",
			mutate: func(_ *BasePostSmokeHandoffReadinessContract, evidence *BaseOwnerReviewReadinessEvidence) {
				evidence.CompletionClaimed = true
			},
			wantErr: "evidence carries downstream execution or completion claims",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handoff, evidence := baseOwnerReviewReadinessContractInputs(t)
			tc.mutate(&handoff, &evidence)

			contract, err := BuildBaseOwnerReviewReadinessContract(handoff, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildBaseOwnerReviewReadinessContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func baseOwnerReviewReadinessContractInputs(t *testing.T) (BasePostSmokeHandoffReadinessContract, BaseOwnerReviewReadinessEvidence) {
	t.Helper()

	smoke, handoffEvidence := basePostSmokeHandoffReadinessContractInputs(t)
	handoff, err := BuildBasePostSmokeHandoffReadinessContract(smoke, handoffEvidence)
	if err != nil {
		t.Fatalf("BuildBasePostSmokeHandoffReadinessContract(): %v", err)
	}

	evidence := BaseOwnerReviewReadinessEvidence{
		PostSmokeHandoffRef:          " post-smoke-handoff:base-pass-106 ",
		ReviewPacketRef:              " owner-review-packet:base-pass-107 ",
		ReviewerIdentityPolicyRef:    " reviewer-identity-policy:base-pass-107 ",
		OwnerReviewInstructionsRef:   " owner-review-instructions:base-pass-107 ",
		RiskSummaryRef:               " owner-review-risk-summary:base-pass-107 ",
		RollbackPlanRef:              " " + handoff.RollbackPlanRef + " ",
		NoOwnerApprovalMutation:      true,
		NoPromotionMutation:          true,
		NoPackagePublicationMutation: true,
		NoRunAcceptanceMutation:      true,
		NoProductionMutation:         true,
		OwnerApproved:                false,
		PromotionExecuted:            false,
		PackagePublished:             false,
		VerifierContractSatisfied:    false,
		RunAcceptanceRecordTouched:   false,
		FullSubstrateClaimed:         false,
		CompletionClaimed:            false,
	}
	return handoff, evidence
}
