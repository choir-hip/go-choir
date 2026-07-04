package computerversion

import (
	"strings"
	"testing"
)

func TestBuildBaseVerifierReadinessContractCreatesVerifierInputPacketOnly(t *testing.T) {
	owner, evidence := baseVerifierReadinessContractInputs(t)

	contract, err := BuildBaseVerifierReadinessContract(owner, evidence)
	if err != nil {
		t.Fatalf("BuildBaseVerifierReadinessContract(): %v", err)
	}

	if contract.Kind != BaseVerifierReadinessContractKind {
		t.Fatalf("kind = %q, want %q", contract.Kind, BaseVerifierReadinessContractKind)
	}
	if contract.Boundary != BaseVerifierReadinessBoundary {
		t.Fatalf("boundary = %q, want %q", contract.Boundary, BaseVerifierReadinessBoundary)
	}
	if contract.Scope != BaseVerifierReadinessScope {
		t.Fatalf("scope = %q, want %q", contract.Scope, BaseVerifierReadinessScope)
	}
	if contract.Version != owner.Version || ArtifactProgramRef(contract.TypedArtifactProgramRef) != owner.Version.ArtifactProgramRef {
		t.Fatalf("version/ref = %#v/%q, want owner-review version %#v and artifact program %q", contract.Version, contract.TypedArtifactProgramRef, owner.Version, owner.Version.ArtifactProgramRef)
	}
	if contract.ProductPathProbeRef != strings.TrimSpace(owner.ProductPathProbeRef) || contract.BuildIdentity != strings.TrimSpace(owner.BuildIdentity) || contract.RouteIdentity != strings.TrimSpace(owner.RouteIdentity) {
		t.Fatalf("product probe/build/route = %q/%q/%q, want trimmed owner-review identity %q/%q/%q", contract.ProductPathProbeRef, contract.BuildIdentity, contract.RouteIdentity, owner.ProductPathProbeRef, owner.BuildIdentity, owner.RouteIdentity)
	}
	if contract.OwnerReviewReadinessRef != strings.TrimSpace(evidence.OwnerReviewReadinessRef) || contract.ReviewPacketRef != strings.TrimSpace(owner.ReviewPacketRef) || contract.VerifierInputBundleRef != strings.TrimSpace(evidence.VerifierInputBundleRef) || contract.VerifierContractSpecRef != strings.TrimSpace(evidence.VerifierContractSpecRef) || contract.EvidenceManifestRef != strings.TrimSpace(evidence.EvidenceManifestRef) || contract.ExpectedVerdictPolicyRef != strings.TrimSpace(evidence.ExpectedVerdictPolicyRef) || contract.RollbackPlanRef != strings.TrimSpace(evidence.RollbackPlanRef) {
		t.Fatalf("verifier refs = owner-readiness %q review-packet %q input %q spec %q evidence %q verdict %q rollback %q, want trimmed refs from owner %#v and evidence %#v", contract.OwnerReviewReadinessRef, contract.ReviewPacketRef, contract.VerifierInputBundleRef, contract.VerifierContractSpecRef, contract.EvidenceManifestRef, contract.ExpectedVerdictPolicyRef, contract.RollbackPlanRef, owner, evidence)
	}
	if contract.ReadinessStatus != BaseVerifierReadinessStatusReady || !contract.VerifierReviewReady || contract.VerifierContractSatisfied {
		t.Fatalf("verifier readiness = status %q ready %v satisfied %v, want ready for verifier review without satisfaction", contract.ReadinessStatus, contract.VerifierReviewReady, contract.VerifierContractSatisfied)
	}
	if !contract.VerifierContractProofRequired || !contract.OwnerApprovalRequired || !contract.PromotionRollbackReviewRequired || !contract.PackagePublicationProofRequired || !contract.RunAcceptanceProofRequired || !contract.FullSubstrateProofRequired {
		t.Fatalf("contract must require verifier proof, owner approval, promotion/rollback review, package publication proof, run-acceptance proof, and full-substrate proof: %#v", contract)
	}
	if contract.VerifierContractSatisfactionAllowed || contract.OwnerApprovalAllowed || contract.PromotionAllowed || contract.PackagePublicationAllowed || contract.RunAcceptanceSynthesisAllowed {
		t.Fatalf("contract must not allow verifier satisfaction, owner approval, promotion, package publication, or run-acceptance synthesis execution: %#v", contract)
	}
	if !contract.NoOwnerApprovalMutation || !contract.NoPromotionMutation || !contract.NoPackagePublicationMutation || !contract.NoVerifierSatisfaction || !contract.NoRunAcceptanceMutation || !contract.NoProductionMutation {
		t.Fatalf("contract must carry no-owner-approval/no-promotion/no-package/no-verifier-satisfaction/no-run-acceptance/no-production flags: %#v", contract)
	}
	if contract.OwnerApproved || contract.PromotionExecuted || contract.PackagePublished || contract.VerifierContractSatisfied || contract.RunAcceptanceRecordTouched || contract.FullSubstrateClaimed || contract.CompletionClaimed {
		t.Fatalf("contract must not approve, promote, publish, satisfy verifier contract, touch run acceptance, claim full substrate, or claim completion: %#v", contract)
	}
}

func TestBuildBaseVerifierReadinessContractRejectsInvalidInputs(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*BaseOwnerReviewReadinessContract, *BaseVerifierReadinessEvidence)
		wantErr string
	}{
		{
			name: "owner-review wrong kind",
			mutate: func(owner *BaseOwnerReviewReadinessContract, _ *BaseVerifierReadinessEvidence) {
				owner.Kind = BaseVerifierReadinessContractKind
			},
			wantErr: "owner-review kind",
		},
		{
			name: "owner-review boundary drift",
			mutate: func(owner *BaseOwnerReviewReadinessContract, _ *BaseVerifierReadinessEvidence) {
				owner.Boundary = BaseVerifierReadinessBoundary
			},
			wantErr: "owner-review boundary",
		},
		{
			name: "owner-review scope drift",
			mutate: func(owner *BaseOwnerReviewReadinessContract, _ *BaseVerifierReadinessEvidence) {
				owner.Scope = BaseVerifierReadinessScope
			},
			wantErr: "owner-review scope",
		},
		{
			name: "invalid owner-review version",
			mutate: func(owner *BaseOwnerReviewReadinessContract, _ *BaseVerifierReadinessEvidence) {
				owner.Version.CodeRef = "  "
			},
			wantErr: "owner-review version is invalid",
		},
		{
			name: "invalid owner-review typed artifact ref",
			mutate: func(owner *BaseOwnerReviewReadinessContract, _ *BaseVerifierReadinessEvidence) {
				owner.TypedArtifactProgramRef = "artifact-program:base-different"
			},
			wantErr: "owner-review typed artifact-program ref is invalid",
		},
		{
			name: "missing owner-review post-smoke handoff ref",
			mutate: func(owner *BaseOwnerReviewReadinessContract, _ *BaseVerifierReadinessEvidence) {
				owner.PostSmokeHandoffRef = "  "
			},
			wantErr: "owner-review refs are required",
		},
		{
			name: "missing owner-review product path probe ref",
			mutate: func(owner *BaseOwnerReviewReadinessContract, _ *BaseVerifierReadinessEvidence) {
				owner.ProductPathProbeRef = "\t"
			},
			wantErr: "owner-review refs are required",
		},
		{
			name: "missing owner-review build identity",
			mutate: func(owner *BaseOwnerReviewReadinessContract, _ *BaseVerifierReadinessEvidence) {
				owner.BuildIdentity = ""
			},
			wantErr: "owner-review refs are required",
		},
		{
			name: "missing owner-review route identity",
			mutate: func(owner *BaseOwnerReviewReadinessContract, _ *BaseVerifierReadinessEvidence) {
				owner.RouteIdentity = "  "
			},
			wantErr: "owner-review refs are required",
		},
		{
			name: "missing owner-review packet ref",
			mutate: func(owner *BaseOwnerReviewReadinessContract, _ *BaseVerifierReadinessEvidence) {
				owner.ReviewPacketRef = "\t"
			},
			wantErr: "owner-review refs are required",
		},
		{
			name: "missing owner-review reviewer identity policy ref",
			mutate: func(owner *BaseOwnerReviewReadinessContract, _ *BaseVerifierReadinessEvidence) {
				owner.ReviewerIdentityPolicyRef = ""
			},
			wantErr: "owner-review refs are required",
		},
		{
			name: "missing owner-review instructions ref",
			mutate: func(owner *BaseOwnerReviewReadinessContract, _ *BaseVerifierReadinessEvidence) {
				owner.OwnerReviewInstructionsRef = "  "
			},
			wantErr: "owner-review refs are required",
		},
		{
			name: "missing owner-review risk summary ref",
			mutate: func(owner *BaseOwnerReviewReadinessContract, _ *BaseVerifierReadinessEvidence) {
				owner.RiskSummaryRef = "\t"
			},
			wantErr: "owner-review refs are required",
		},
		{
			name: "missing owner-review rollback plan ref",
			mutate: func(owner *BaseOwnerReviewReadinessContract, _ *BaseVerifierReadinessEvidence) {
				owner.RollbackPlanRef = ""
			},
			wantErr: "owner-review refs are required",
		},
		{
			name: "owner review status not ready",
			mutate: func(owner *BaseOwnerReviewReadinessContract, _ *BaseVerifierReadinessEvidence) {
				owner.ReadinessStatus = "blocked_before_owner_review"
			},
			wantErr: "owner-review must be ready but not approved",
		},
		{
			name: "owner review ready flag dropped",
			mutate: func(owner *BaseOwnerReviewReadinessContract, _ *BaseVerifierReadinessEvidence) {
				owner.OwnerReviewReady = false
			},
			wantErr: "owner-review must be ready but not approved",
		},
		{
			name: "owner-review missing owner approval requirement",
			mutate: func(owner *BaseOwnerReviewReadinessContract, _ *BaseVerifierReadinessEvidence) {
				owner.OwnerApprovalRequired = false
			},
			wantErr: "owner-review must preserve downstream proof requirements",
		},
		{
			name: "owner-review missing promotion rollback review requirement",
			mutate: func(owner *BaseOwnerReviewReadinessContract, _ *BaseVerifierReadinessEvidence) {
				owner.PromotionRollbackReviewRequired = false
			},
			wantErr: "owner-review must preserve downstream proof requirements",
		},
		{
			name: "owner-review missing package publication proof requirement",
			mutate: func(owner *BaseOwnerReviewReadinessContract, _ *BaseVerifierReadinessEvidence) {
				owner.PackagePublicationProofRequired = false
			},
			wantErr: "owner-review must preserve downstream proof requirements",
		},
		{
			name: "owner-review missing verifier contract proof requirement",
			mutate: func(owner *BaseOwnerReviewReadinessContract, _ *BaseVerifierReadinessEvidence) {
				owner.VerifierContractProofRequired = false
			},
			wantErr: "owner-review must preserve downstream proof requirements",
		},
		{
			name: "owner-review missing run acceptance proof requirement",
			mutate: func(owner *BaseOwnerReviewReadinessContract, _ *BaseVerifierReadinessEvidence) {
				owner.RunAcceptanceProofRequired = false
			},
			wantErr: "owner-review must preserve downstream proof requirements",
		},
		{
			name: "owner-review missing full substrate proof requirement",
			mutate: func(owner *BaseOwnerReviewReadinessContract, _ *BaseVerifierReadinessEvidence) {
				owner.FullSubstrateProofRequired = false
			},
			wantErr: "owner-review must preserve downstream proof requirements",
		},
		{
			name: "owner-review allows owner approval",
			mutate: func(owner *BaseOwnerReviewReadinessContract, _ *BaseVerifierReadinessEvidence) {
				owner.OwnerApprovalAllowed = true
			},
			wantErr: "owner-review allows downstream execution",
		},
		{
			name: "owner-review allows promotion",
			mutate: func(owner *BaseOwnerReviewReadinessContract, _ *BaseVerifierReadinessEvidence) {
				owner.PromotionAllowed = true
			},
			wantErr: "owner-review allows downstream execution",
		},
		{
			name: "owner-review allows package publication",
			mutate: func(owner *BaseOwnerReviewReadinessContract, _ *BaseVerifierReadinessEvidence) {
				owner.PackagePublicationAllowed = true
			},
			wantErr: "owner-review allows downstream execution",
		},
		{
			name: "owner-review allows verifier satisfaction",
			mutate: func(owner *BaseOwnerReviewReadinessContract, _ *BaseVerifierReadinessEvidence) {
				owner.VerifierContractSatisfactionAllowed = true
			},
			wantErr: "owner-review allows downstream execution",
		},
		{
			name: "owner-review allows run acceptance synthesis",
			mutate: func(owner *BaseOwnerReviewReadinessContract, _ *BaseVerifierReadinessEvidence) {
				owner.RunAcceptanceSynthesisAllowed = true
			},
			wantErr: "owner-review allows downstream execution",
		},
		{
			name: "owner-review missing no owner approval mutation flag",
			mutate: func(owner *BaseOwnerReviewReadinessContract, _ *BaseVerifierReadinessEvidence) {
				owner.NoOwnerApprovalMutation = false
			},
			wantErr: "owner-review must prove no owner approval, promotion, package publication, run-acceptance, or production mutation",
		},
		{
			name: "owner-review missing no promotion mutation flag",
			mutate: func(owner *BaseOwnerReviewReadinessContract, _ *BaseVerifierReadinessEvidence) {
				owner.NoPromotionMutation = false
			},
			wantErr: "owner-review must prove no owner approval, promotion, package publication, run-acceptance, or production mutation",
		},
		{
			name: "owner-review missing no package publication mutation flag",
			mutate: func(owner *BaseOwnerReviewReadinessContract, _ *BaseVerifierReadinessEvidence) {
				owner.NoPackagePublicationMutation = false
			},
			wantErr: "owner-review must prove no owner approval, promotion, package publication, run-acceptance, or production mutation",
		},
		{
			name: "owner-review missing no run acceptance mutation flag",
			mutate: func(owner *BaseOwnerReviewReadinessContract, _ *BaseVerifierReadinessEvidence) {
				owner.NoRunAcceptanceMutation = false
			},
			wantErr: "owner-review must prove no owner approval, promotion, package publication, run-acceptance, or production mutation",
		},
		{
			name: "owner-review missing no production mutation flag",
			mutate: func(owner *BaseOwnerReviewReadinessContract, _ *BaseVerifierReadinessEvidence) {
				owner.NoProductionMutation = false
			},
			wantErr: "owner-review must prove no owner approval, promotion, package publication, run-acceptance, or production mutation",
		},
		{
			name: "owner-review owner approved",
			mutate: func(owner *BaseOwnerReviewReadinessContract, _ *BaseVerifierReadinessEvidence) {
				owner.OwnerApproved = true
			},
			wantErr: "owner-review carries downstream execution or completion claims",
		},
		{
			name: "owner-review promotion executed",
			mutate: func(owner *BaseOwnerReviewReadinessContract, _ *BaseVerifierReadinessEvidence) {
				owner.PromotionExecuted = true
			},
			wantErr: "owner-review carries downstream execution or completion claims",
		},
		{
			name: "owner-review package published",
			mutate: func(owner *BaseOwnerReviewReadinessContract, _ *BaseVerifierReadinessEvidence) {
				owner.PackagePublished = true
			},
			wantErr: "owner-review carries downstream execution or completion claims",
		},
		{
			name: "owner-review verifier satisfied",
			mutate: func(owner *BaseOwnerReviewReadinessContract, _ *BaseVerifierReadinessEvidence) {
				owner.VerifierContractSatisfied = true
			},
			wantErr: "owner-review carries downstream execution or completion claims",
		},
		{
			name: "owner-review run acceptance touched",
			mutate: func(owner *BaseOwnerReviewReadinessContract, _ *BaseVerifierReadinessEvidence) {
				owner.RunAcceptanceRecordTouched = true
			},
			wantErr: "owner-review carries downstream execution or completion claims",
		},
		{
			name: "owner-review full substrate claimed",
			mutate: func(owner *BaseOwnerReviewReadinessContract, _ *BaseVerifierReadinessEvidence) {
				owner.FullSubstrateClaimed = true
			},
			wantErr: "owner-review carries downstream execution or completion claims",
		},
		{
			name: "owner-review completion claimed",
			mutate: func(owner *BaseOwnerReviewReadinessContract, _ *BaseVerifierReadinessEvidence) {
				owner.CompletionClaimed = true
			},
			wantErr: "owner-review carries downstream execution or completion claims",
		},
		{
			name: "missing verifier owner-review readiness ref",
			mutate: func(_ *BaseOwnerReviewReadinessContract, evidence *BaseVerifierReadinessEvidence) {
				evidence.OwnerReviewReadinessRef = "  "
			},
			wantErr: "verifier refs are required",
		},
		{
			name: "missing verifier input bundle ref",
			mutate: func(_ *BaseOwnerReviewReadinessContract, evidence *BaseVerifierReadinessEvidence) {
				evidence.VerifierInputBundleRef = "\t"
			},
			wantErr: "verifier refs are required",
		},
		{
			name: "missing verifier contract spec ref",
			mutate: func(_ *BaseOwnerReviewReadinessContract, evidence *BaseVerifierReadinessEvidence) {
				evidence.VerifierContractSpecRef = ""
			},
			wantErr: "verifier refs are required",
		},
		{
			name: "missing verifier evidence manifest ref",
			mutate: func(_ *BaseOwnerReviewReadinessContract, evidence *BaseVerifierReadinessEvidence) {
				evidence.EvidenceManifestRef = "  "
			},
			wantErr: "verifier refs are required",
		},
		{
			name: "missing verifier expected verdict policy ref",
			mutate: func(_ *BaseOwnerReviewReadinessContract, evidence *BaseVerifierReadinessEvidence) {
				evidence.ExpectedVerdictPolicyRef = "\t"
			},
			wantErr: "verifier refs are required",
		},
		{
			name: "missing verifier rollback plan ref",
			mutate: func(_ *BaseOwnerReviewReadinessContract, evidence *BaseVerifierReadinessEvidence) {
				evidence.RollbackPlanRef = ""
			},
			wantErr: "verifier refs are required",
		},
		{
			name: "missing verifier no owner approval mutation flag",
			mutate: func(_ *BaseOwnerReviewReadinessContract, evidence *BaseVerifierReadinessEvidence) {
				evidence.NoOwnerApprovalMutation = false
			},
			wantErr: "evidence must prove no owner approval, promotion, package publication, verifier satisfaction, run-acceptance, or production mutation",
		},
		{
			name: "missing verifier no promotion mutation flag",
			mutate: func(_ *BaseOwnerReviewReadinessContract, evidence *BaseVerifierReadinessEvidence) {
				evidence.NoPromotionMutation = false
			},
			wantErr: "evidence must prove no owner approval, promotion, package publication, verifier satisfaction, run-acceptance, or production mutation",
		},
		{
			name: "missing verifier no package publication mutation flag",
			mutate: func(_ *BaseOwnerReviewReadinessContract, evidence *BaseVerifierReadinessEvidence) {
				evidence.NoPackagePublicationMutation = false
			},
			wantErr: "evidence must prove no owner approval, promotion, package publication, verifier satisfaction, run-acceptance, or production mutation",
		},
		{
			name: "missing verifier no verifier satisfaction flag",
			mutate: func(_ *BaseOwnerReviewReadinessContract, evidence *BaseVerifierReadinessEvidence) {
				evidence.NoVerifierSatisfaction = false
			},
			wantErr: "evidence must prove no owner approval, promotion, package publication, verifier satisfaction, run-acceptance, or production mutation",
		},
		{
			name: "missing verifier no run acceptance mutation flag",
			mutate: func(_ *BaseOwnerReviewReadinessContract, evidence *BaseVerifierReadinessEvidence) {
				evidence.NoRunAcceptanceMutation = false
			},
			wantErr: "evidence must prove no owner approval, promotion, package publication, verifier satisfaction, run-acceptance, or production mutation",
		},
		{
			name: "missing verifier no production mutation flag",
			mutate: func(_ *BaseOwnerReviewReadinessContract, evidence *BaseVerifierReadinessEvidence) {
				evidence.NoProductionMutation = false
			},
			wantErr: "evidence must prove no owner approval, promotion, package publication, verifier satisfaction, run-acceptance, or production mutation",
		},
		{
			name: "owner approved",
			mutate: func(_ *BaseOwnerReviewReadinessContract, evidence *BaseVerifierReadinessEvidence) {
				evidence.OwnerApproved = true
			},
			wantErr: "evidence carries downstream execution or completion claims",
		},
		{
			name: "promotion executed",
			mutate: func(_ *BaseOwnerReviewReadinessContract, evidence *BaseVerifierReadinessEvidence) {
				evidence.PromotionExecuted = true
			},
			wantErr: "evidence carries downstream execution or completion claims",
		},
		{
			name: "package published",
			mutate: func(_ *BaseOwnerReviewReadinessContract, evidence *BaseVerifierReadinessEvidence) {
				evidence.PackagePublished = true
			},
			wantErr: "evidence carries downstream execution or completion claims",
		},
		{
			name: "verifier satisfied",
			mutate: func(_ *BaseOwnerReviewReadinessContract, evidence *BaseVerifierReadinessEvidence) {
				evidence.VerifierContractSatisfied = true
			},
			wantErr: "evidence carries downstream execution or completion claims",
		},
		{
			name: "run acceptance touched",
			mutate: func(_ *BaseOwnerReviewReadinessContract, evidence *BaseVerifierReadinessEvidence) {
				evidence.RunAcceptanceRecordTouched = true
			},
			wantErr: "evidence carries downstream execution or completion claims",
		},
		{
			name: "full substrate claimed",
			mutate: func(_ *BaseOwnerReviewReadinessContract, evidence *BaseVerifierReadinessEvidence) {
				evidence.FullSubstrateClaimed = true
			},
			wantErr: "evidence carries downstream execution or completion claims",
		},
		{
			name: "completion claimed",
			mutate: func(_ *BaseOwnerReviewReadinessContract, evidence *BaseVerifierReadinessEvidence) {
				evidence.CompletionClaimed = true
			},
			wantErr: "evidence carries downstream execution or completion claims",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			owner, evidence := baseVerifierReadinessContractInputs(t)
			tc.mutate(&owner, &evidence)

			contract, err := BuildBaseVerifierReadinessContract(owner, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildBaseVerifierReadinessContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func baseVerifierReadinessContractInputs(t *testing.T) (BaseOwnerReviewReadinessContract, BaseVerifierReadinessEvidence) {
	t.Helper()

	handoff, ownerEvidence := baseOwnerReviewReadinessContractInputs(t)
	owner, err := BuildBaseOwnerReviewReadinessContract(handoff, ownerEvidence)
	if err != nil {
		t.Fatalf("BuildBaseOwnerReviewReadinessContract(): %v", err)
	}

	evidence := BaseVerifierReadinessEvidence{
		OwnerReviewReadinessRef:      " owner-review-readiness:base-pass-107 ",
		VerifierInputBundleRef:       " verifier-input-bundle:base-pass-108 ",
		VerifierContractSpecRef:      " verifier-contract-spec:base-pass-108 ",
		EvidenceManifestRef:          " verifier-evidence-manifest:base-pass-108 ",
		ExpectedVerdictPolicyRef:     " expected-verdict-policy:base-pass-108 ",
		RollbackPlanRef:              " " + owner.RollbackPlanRef + " ",
		NoOwnerApprovalMutation:      true,
		NoPromotionMutation:          true,
		NoPackagePublicationMutation: true,
		NoVerifierSatisfaction:       true,
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
	return owner, evidence
}
