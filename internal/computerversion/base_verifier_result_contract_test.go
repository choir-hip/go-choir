package computerversion

import (
	"strings"
	"testing"
)

func TestBuildBaseVerifierResultContractAcceptsPassingVerifierResultOnly(t *testing.T) {
	readiness, evidence := baseVerifierResultContractInputs(t, BaseVerifierVerdictPass)

	contract, err := BuildBaseVerifierResultContract(readiness, evidence)
	if err != nil {
		t.Fatalf("BuildBaseVerifierResultContract(): %v", err)
	}

	if contract.Kind != BaseVerifierResultContractKind {
		t.Fatalf("kind = %q, want %q", contract.Kind, BaseVerifierResultContractKind)
	}
	if contract.Boundary != BaseVerifierResultBoundary {
		t.Fatalf("boundary = %q, want %q", contract.Boundary, BaseVerifierResultBoundary)
	}
	if contract.Scope != BaseVerifierResultScope {
		t.Fatalf("scope = %q, want %q", contract.Scope, BaseVerifierResultScope)
	}
	if contract.Version != readiness.Version || ArtifactProgramRef(contract.TypedArtifactProgramRef) != readiness.Version.ArtifactProgramRef {
		t.Fatalf("version/ref = %#v/%q, want readiness version %#v and artifact program %q", contract.Version, contract.TypedArtifactProgramRef, readiness.Version, readiness.Version.ArtifactProgramRef)
	}
	if contract.VerifierReadinessRef != strings.TrimSpace(evidence.VerifierReadinessRef) || contract.VerifierInputBundleRef != strings.TrimSpace(readiness.VerifierInputBundleRef) || contract.VerifierContractSpecRef != strings.TrimSpace(readiness.VerifierContractSpecRef) || contract.EvidenceManifestRef != strings.TrimSpace(readiness.EvidenceManifestRef) || contract.ExpectedVerdictPolicyRef != strings.TrimSpace(readiness.ExpectedVerdictPolicyRef) {
		t.Fatalf("readiness refs = readiness %q input %q spec %q evidence %q verdict-policy %q, want trimmed refs from readiness %#v and evidence %#v", contract.VerifierReadinessRef, contract.VerifierInputBundleRef, contract.VerifierContractSpecRef, contract.EvidenceManifestRef, contract.ExpectedVerdictPolicyRef, readiness, evidence)
	}
	if contract.VerifierRunRef != strings.TrimSpace(evidence.VerifierRunRef) || contract.VerifierResultRef != strings.TrimSpace(evidence.VerifierResultRef) || contract.VerifierLogRef != strings.TrimSpace(evidence.VerifierLogRef) || contract.RollbackPlanRef != strings.TrimSpace(evidence.RollbackPlanRef) {
		t.Fatalf("result refs = run %q result %q log %q rollback %q, want trimmed refs from evidence %#v", contract.VerifierRunRef, contract.VerifierResultRef, contract.VerifierLogRef, contract.RollbackPlanRef, evidence)
	}
	if contract.Verdict != BaseVerifierVerdictPass || contract.FailureReason != "" || !contract.VerifierContractSatisfied || contract.VerifierContractFailed || contract.VerifierFailureBlocksDownstream {
		t.Fatalf("pass verdict = verdict %q reason %q satisfied %v failed %v blocks %v, want satisfied pass without failure or blocking", contract.Verdict, contract.FailureReason, contract.VerifierContractSatisfied, contract.VerifierContractFailed, contract.VerifierFailureBlocksDownstream)
	}
	if !contract.OwnerApprovalRequired || !contract.PromotionRollbackReviewRequired || !contract.PackagePublicationProofRequired || !contract.RunAcceptanceProofRequired || !contract.FullSubstrateProofRequired {
		t.Fatalf("passing result must preserve owner/promotion/package/run-acceptance/full-substrate proof requirements: %#v", contract)
	}
	if contract.OwnerApprovalAllowed || contract.PromotionAllowed || contract.PackagePublicationAllowed || contract.RunAcceptanceSynthesisAllowed {
		t.Fatalf("passing result must not allow owner approval, promotion, publication, or run-acceptance synthesis: %#v", contract)
	}
	if !contract.NoOwnerApprovalMutation || !contract.NoPromotionMutation || !contract.NoPackagePublicationMutation || !contract.NoRunAcceptanceMutation || !contract.NoProductionMutation {
		t.Fatalf("passing result must carry no-owner-approval/no-promotion/no-package/no-run-acceptance/no-production mutation flags: %#v", contract)
	}
	if contract.OwnerApproved || contract.PromotionExecuted || contract.PackagePublished || contract.RunAcceptanceRecordTouched || contract.FullSubstrateClaimed || contract.CompletionClaimed {
		t.Fatalf("passing result must not approve, promote, publish, touch run acceptance, claim full substrate, or claim completion: %#v", contract)
	}
}

func TestBuildBaseVerifierResultContractAcceptsFailingVerifierResultAsBlockingEvidence(t *testing.T) {
	readiness, evidence := baseVerifierResultContractInputs(t, BaseVerifierVerdictFail)

	contract, err := BuildBaseVerifierResultContract(readiness, evidence)
	if err != nil {
		t.Fatalf("BuildBaseVerifierResultContract(): %v", err)
	}

	if contract.Verdict != BaseVerifierVerdictFail || contract.FailureReason != strings.TrimSpace(evidence.FailureReason) {
		t.Fatalf("fail verdict = verdict %q reason %q, want fail with trimmed reason %q", contract.Verdict, contract.FailureReason, evidence.FailureReason)
	}
	if contract.VerifierContractSatisfied || !contract.VerifierContractFailed || !contract.VerifierFailureBlocksDownstream {
		t.Fatalf("failing result must fail verifier contract and block downstream: %#v", contract)
	}
	if !contract.OwnerApprovalRequired || !contract.PromotionRollbackReviewRequired || !contract.PackagePublicationProofRequired || !contract.RunAcceptanceProofRequired || !contract.FullSubstrateProofRequired {
		t.Fatalf("failing result must preserve owner/promotion/package/run-acceptance/full-substrate proof requirements: %#v", contract)
	}
	if contract.OwnerApprovalAllowed || contract.PromotionAllowed || contract.PackagePublicationAllowed || contract.RunAcceptanceSynthesisAllowed {
		t.Fatalf("failing result must not allow owner approval, promotion, publication, or run-acceptance synthesis: %#v", contract)
	}
	if !contract.NoOwnerApprovalMutation || !contract.NoPromotionMutation || !contract.NoPackagePublicationMutation || !contract.NoRunAcceptanceMutation || !contract.NoProductionMutation {
		t.Fatalf("failing result must carry no-owner-approval/no-promotion/no-package/no-run-acceptance/no-production mutation flags: %#v", contract)
	}
	if contract.OwnerApproved || contract.PromotionExecuted || contract.PackagePublished || contract.RunAcceptanceRecordTouched || contract.FullSubstrateClaimed || contract.CompletionClaimed {
		t.Fatalf("failing result must not approve, promote, publish, touch run acceptance, claim full substrate, or claim completion: %#v", contract)
	}
}

func TestBuildBaseVerifierResultContractRejectsInvalidInputs(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*BaseVerifierReadinessContract, *BaseVerifierResultEvidence)
		wantErr string
	}{
		{
			name: "readiness wrong kind",
			mutate: func(readiness *BaseVerifierReadinessContract, _ *BaseVerifierResultEvidence) {
				readiness.Kind = BaseOwnerReviewReadinessContractKind
			},
			wantErr: "readiness kind",
		},
		{
			name: "readiness boundary drift",
			mutate: func(readiness *BaseVerifierReadinessContract, _ *BaseVerifierResultEvidence) {
				readiness.Boundary = BaseVerifierResultBoundary
			},
			wantErr: "readiness boundary",
		},
		{
			name: "readiness scope drift",
			mutate: func(readiness *BaseVerifierReadinessContract, _ *BaseVerifierResultEvidence) {
				readiness.Scope = BaseVerifierResultScope
			},
			wantErr: "readiness scope",
		},
		{
			name: "readiness invalid version",
			mutate: func(readiness *BaseVerifierReadinessContract, _ *BaseVerifierResultEvidence) {
				readiness.Version.CodeRef = "  "
			},
			wantErr: "readiness version is invalid",
		},
		{
			name: "readiness typed artifact ref drift",
			mutate: func(readiness *BaseVerifierReadinessContract, _ *BaseVerifierResultEvidence) {
				readiness.TypedArtifactProgramRef = "artifact-program:base-different"
			},
			wantErr: "readiness typed artifact-program ref is invalid",
		},
		{
			name: "missing readiness verifier input bundle ref",
			mutate: func(readiness *BaseVerifierReadinessContract, _ *BaseVerifierResultEvidence) {
				readiness.VerifierInputBundleRef = "\t"
			},
			wantErr: "readiness refs are required",
		},
		{
			name: "missing result verifier readiness ref",
			mutate: func(_ *BaseVerifierReadinessContract, evidence *BaseVerifierResultEvidence) {
				evidence.VerifierReadinessRef = "  "
			},
			wantErr: "result refs are required",
		},
		{
			name: "missing result verifier run ref",
			mutate: func(_ *BaseVerifierReadinessContract, evidence *BaseVerifierResultEvidence) {
				evidence.VerifierRunRef = "\t"
			},
			wantErr: "result refs are required",
		},
		{
			name: "missing result verifier result ref",
			mutate: func(_ *BaseVerifierReadinessContract, evidence *BaseVerifierResultEvidence) {
				evidence.VerifierResultRef = ""
			},
			wantErr: "result refs are required",
		},
		{
			name: "missing result verifier log ref",
			mutate: func(_ *BaseVerifierReadinessContract, evidence *BaseVerifierResultEvidence) {
				evidence.VerifierLogRef = "  "
			},
			wantErr: "result refs are required",
		},
		{
			name: "missing result rollback plan ref",
			mutate: func(_ *BaseVerifierReadinessContract, evidence *BaseVerifierResultEvidence) {
				evidence.RollbackPlanRef = "\t"
			},
			wantErr: "result refs are required",
		},
		{
			name: "readiness status drift",
			mutate: func(readiness *BaseVerifierReadinessContract, _ *BaseVerifierResultEvidence) {
				readiness.ReadinessStatus = "blocked_before_verifier_review"
			},
			wantErr: "readiness must be verifier-ready",
		},
		{
			name: "readiness ready flag dropped",
			mutate: func(readiness *BaseVerifierReadinessContract, _ *BaseVerifierResultEvidence) {
				readiness.VerifierReviewReady = false
			},
			wantErr: "readiness must be verifier-ready",
		},
		{
			name: "readiness missing verifier proof requirement",
			mutate: func(readiness *BaseVerifierReadinessContract, _ *BaseVerifierResultEvidence) {
				readiness.VerifierContractProofRequired = false
			},
			wantErr: "readiness must preserve downstream proof requirements",
		},
		{
			name: "readiness missing full substrate proof requirement",
			mutate: func(readiness *BaseVerifierReadinessContract, _ *BaseVerifierResultEvidence) {
				readiness.FullSubstrateProofRequired = false
			},
			wantErr: "readiness must preserve downstream proof requirements",
		},
		{
			name: "readiness allows verifier satisfaction",
			mutate: func(readiness *BaseVerifierReadinessContract, _ *BaseVerifierResultEvidence) {
				readiness.VerifierContractSatisfactionAllowed = true
			},
			wantErr: "readiness allows downstream execution",
		},
		{
			name: "readiness allows owner approval",
			mutate: func(readiness *BaseVerifierReadinessContract, _ *BaseVerifierResultEvidence) {
				readiness.OwnerApprovalAllowed = true
			},
			wantErr: "readiness allows downstream execution",
		},
		{
			name: "readiness missing no verifier satisfaction flag",
			mutate: func(readiness *BaseVerifierReadinessContract, _ *BaseVerifierResultEvidence) {
				readiness.NoVerifierSatisfaction = false
			},
			wantErr: "readiness must prove no owner approval, promotion, package publication, verifier satisfaction, run-acceptance, or production mutation",
		},
		{
			name: "readiness missing no run acceptance mutation flag",
			mutate: func(readiness *BaseVerifierReadinessContract, _ *BaseVerifierResultEvidence) {
				readiness.NoRunAcceptanceMutation = false
			},
			wantErr: "readiness must prove no owner approval, promotion, package publication, verifier satisfaction, run-acceptance, or production mutation",
		},
		{
			name: "readiness already satisfied verifier contract",
			mutate: func(readiness *BaseVerifierReadinessContract, _ *BaseVerifierResultEvidence) {
				readiness.VerifierContractSatisfied = true
			},
			wantErr: "readiness carries downstream execution or completion claims",
		},
		{
			name: "readiness already claimed completion",
			mutate: func(readiness *BaseVerifierReadinessContract, _ *BaseVerifierResultEvidence) {
				readiness.CompletionClaimed = true
			},
			wantErr: "readiness carries downstream execution or completion claims",
		},
		{
			name: "invalid verdict",
			mutate: func(_ *BaseVerifierReadinessContract, evidence *BaseVerifierResultEvidence) {
				evidence.Verdict = "inconclusive"
			},
			wantErr: "verdict must be pass or fail",
		},
		{
			name: "pass with failure reason",
			mutate: func(_ *BaseVerifierReadinessContract, evidence *BaseVerifierResultEvidence) {
				evidence.Verdict = BaseVerifierVerdictPass
				evidence.FailureReason = " verifier failed an invariant "
			},
			wantErr: "passing verdict cannot include a failure reason",
		},
		{
			name: "fail without failure reason",
			mutate: func(_ *BaseVerifierReadinessContract, evidence *BaseVerifierResultEvidence) {
				evidence.Verdict = BaseVerifierVerdictFail
				evidence.FailureReason = "\t"
			},
			wantErr: "failing verdict requires a failure reason",
		},
		{
			name: "missing evidence no owner approval mutation flag",
			mutate: func(_ *BaseVerifierReadinessContract, evidence *BaseVerifierResultEvidence) {
				evidence.NoOwnerApprovalMutation = false
			},
			wantErr: "evidence must prove no owner approval, promotion, package publication, run-acceptance, or production mutation",
		},
		{
			name: "missing evidence no package publication mutation flag",
			mutate: func(_ *BaseVerifierReadinessContract, evidence *BaseVerifierResultEvidence) {
				evidence.NoPackagePublicationMutation = false
			},
			wantErr: "evidence must prove no owner approval, promotion, package publication, run-acceptance, or production mutation",
		},
		{
			name: "missing evidence no production mutation flag",
			mutate: func(_ *BaseVerifierReadinessContract, evidence *BaseVerifierResultEvidence) {
				evidence.NoProductionMutation = false
			},
			wantErr: "evidence must prove no owner approval, promotion, package publication, run-acceptance, or production mutation",
		},
		{
			name: "evidence already satisfied verifier contract",
			mutate: func(_ *BaseVerifierReadinessContract, evidence *BaseVerifierResultEvidence) {
				evidence.VerifierContractSatisfied = true
			},
			wantErr: "evidence carries downstream execution or completion claims",
		},
		{
			name: "evidence claims owner approval",
			mutate: func(_ *BaseVerifierReadinessContract, evidence *BaseVerifierResultEvidence) {
				evidence.OwnerApproved = true
			},
			wantErr: "evidence carries downstream execution or completion claims",
		},
		{
			name: "evidence claims promotion",
			mutate: func(_ *BaseVerifierReadinessContract, evidence *BaseVerifierResultEvidence) {
				evidence.PromotionExecuted = true
			},
			wantErr: "evidence carries downstream execution or completion claims",
		},
		{
			name: "evidence claims package publication",
			mutate: func(_ *BaseVerifierReadinessContract, evidence *BaseVerifierResultEvidence) {
				evidence.PackagePublished = true
			},
			wantErr: "evidence carries downstream execution or completion claims",
		},
		{
			name: "evidence claims run acceptance",
			mutate: func(_ *BaseVerifierReadinessContract, evidence *BaseVerifierResultEvidence) {
				evidence.RunAcceptanceRecordTouched = true
			},
			wantErr: "evidence carries downstream execution or completion claims",
		},
		{
			name: "evidence claims full substrate",
			mutate: func(_ *BaseVerifierReadinessContract, evidence *BaseVerifierResultEvidence) {
				evidence.FullSubstrateClaimed = true
			},
			wantErr: "evidence carries downstream execution or completion claims",
		},
		{
			name: "evidence claims completion",
			mutate: func(_ *BaseVerifierReadinessContract, evidence *BaseVerifierResultEvidence) {
				evidence.CompletionClaimed = true
			},
			wantErr: "evidence carries downstream execution or completion claims",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			readiness, evidence := baseVerifierResultContractInputs(t, BaseVerifierVerdictPass)
			tc.mutate(&readiness, &evidence)

			contract, err := BuildBaseVerifierResultContract(readiness, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildBaseVerifierResultContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func baseVerifierResultContractInputs(t *testing.T, verdict string) (BaseVerifierReadinessContract, BaseVerifierResultEvidence) {
	t.Helper()

	owner, readinessEvidence := baseVerifierReadinessContractInputs(t)
	readiness, err := BuildBaseVerifierReadinessContract(owner, readinessEvidence)
	if err != nil {
		t.Fatalf("BuildBaseVerifierReadinessContract(): %v", err)
	}

	evidence := BaseVerifierResultEvidence{
		VerifierReadinessRef:         " verifier-readiness:base-pass-108 ",
		VerifierRunRef:               " verifier-run:base-pass-109 ",
		VerifierResultRef:            " verifier-result:base-pass-109 ",
		VerifierLogRef:               " verifier-log:base-pass-109 ",
		Verdict:                      verdict,
		RollbackPlanRef:              " " + readiness.RollbackPlanRef + " ",
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
	if verdict == BaseVerifierVerdictFail {
		evidence.FailureReason = " verifier failed base equivalence proof "
	}
	return readiness, evidence
}
