package computerversion

import (
	"strings"
	"testing"
)

func TestBuildBasePackagePublicationProofContractRecordsProofOnly(t *testing.T) {
	readiness, evidence := basePackagePublicationProofContractInputs(t)

	contract, err := BuildBasePackagePublicationProofContract(readiness, evidence)
	if err != nil {
		t.Fatalf("BuildBasePackagePublicationProofContract(): %v", err)
	}

	if contract.Kind != BasePackagePublicationProofContractKind {
		t.Fatalf("kind = %q, want %q", contract.Kind, BasePackagePublicationProofContractKind)
	}
	if contract.Boundary != BasePackagePublicationProofBoundary {
		t.Fatalf("boundary = %q, want %q", contract.Boundary, BasePackagePublicationProofBoundary)
	}
	if contract.Scope != BasePackagePublicationProofScope {
		t.Fatalf("scope = %q, want %q", contract.Scope, BasePackagePublicationProofScope)
	}
	if contract.Version != readiness.Version || ArtifactProgramRef(contract.TypedArtifactProgramRef) != readiness.Version.ArtifactProgramRef {
		t.Fatalf("version/ref = %#v/%q, want readiness version %#v and artifact program %q", contract.Version, contract.TypedArtifactProgramRef, readiness.Version, readiness.Version.ArtifactProgramRef)
	}
	if contract.PublicationReadinessRef != strings.TrimSpace(evidence.PublicationReadinessRef) || contract.PromotionRollbackReviewRef != strings.TrimSpace(readiness.PromotionRollbackReviewRef) {
		t.Fatalf("readiness refs = publication %q promotion-review %q, want evidence/readiness refs from readiness %#v evidence %#v", contract.PublicationReadinessRef, contract.PromotionRollbackReviewRef, readiness, evidence)
	}
	if contract.PackageManifestRef != strings.TrimSpace(readiness.PackageManifestRef) || contract.PublicationPayloadRef != strings.TrimSpace(readiness.PublicationPayloadRef) || contract.PublicationTargetRef != strings.TrimSpace(readiness.PublicationTargetRef) || contract.PublicationPolicyRef != strings.TrimSpace(readiness.PublicationPolicyRef) || contract.PublicationDryRunPlanRef != strings.TrimSpace(readiness.PublicationDryRunPlanRef) {
		t.Fatalf("publication readiness refs = manifest %q payload %q target %q policy %q dry-run %q, want trimmed readiness refs from %#v", contract.PackageManifestRef, contract.PublicationPayloadRef, contract.PublicationTargetRef, contract.PublicationPolicyRef, contract.PublicationDryRunPlanRef, readiness)
	}
	if contract.PublicationProofRef != strings.TrimSpace(evidence.PublicationProofRef) || contract.PublishedPackageRef != strings.TrimSpace(evidence.PublishedPackageRef) || contract.PackageDigestRef != strings.TrimSpace(evidence.PackageDigestRef) || contract.PublicationReceiptRef != strings.TrimSpace(evidence.PublicationReceiptRef) || contract.PublicationLedgerRef != strings.TrimSpace(evidence.PublicationLedgerRef) || contract.PublicationReviewRef != strings.TrimSpace(evidence.PublicationReviewRef) || contract.RollbackPlanRef != strings.TrimSpace(evidence.RollbackPlanRef) {
		t.Fatalf("proof refs = proof %q package %q digest %q receipt %q ledger %q review %q rollback %q, want trimmed proof evidence refs from %#v", contract.PublicationProofRef, contract.PublishedPackageRef, contract.PackageDigestRef, contract.PublicationReceiptRef, contract.PublicationLedgerRef, contract.PublicationReviewRef, contract.RollbackPlanRef, evidence)
	}
	if contract.ProofStatus != BasePackagePublicationProofStatusSatisfied || !contract.OwnerApproved || !contract.PromotionRollbackReviewReady || !contract.PackagePublicationReady || !contract.PackagePublicationProof {
		t.Fatalf("proof status = status %q owner %v review-ready %v package-ready %v proof %v, want satisfied publication proof from ready package-publication contract", contract.ProofStatus, contract.OwnerApproved, contract.PromotionRollbackReviewReady, contract.PackagePublicationReady, contract.PackagePublicationProof)
	}
	if !contract.PromotionProofRequired || !contract.RunAcceptanceProofRequired || !contract.FullSubstrateProofRequired {
		t.Fatalf("package-publication proof must preserve promotion, run-acceptance, and full-substrate proof requirements: %#v", contract)
	}
	if contract.PackagePublicationAllowed || contract.PromotionAllowed || contract.RunAcceptanceSynthesisAllowed {
		t.Fatalf("package-publication proof must not allow package publication, promotion, or run-acceptance synthesis: %#v", contract)
	}
	if !contract.NoPackagePublicationMutation || !contract.NoPromotionMutation || !contract.NoRunAcceptanceMutation || !contract.NoProductionMutation {
		t.Fatalf("package-publication proof must carry no-publication/no-promotion/no-run-acceptance/no-production mutation evidence: %#v", contract)
	}
	if contract.PackagePublished || contract.PromotionExecuted || contract.RunAcceptanceRecordTouched || contract.ProductionStateMutated || contract.FullSubstrateClaimed || contract.CompletionClaimed {
		t.Fatalf("package-publication proof must not publish, promote, touch run acceptance, mutate production, claim full substrate, or claim completion: %#v", contract)
	}
}

func TestBuildBasePackagePublicationProofContractRejectsInvalidInputs(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*BasePackagePublicationReadinessContract, *BasePackagePublicationProofEvidence)
		wantErr string
	}{
		{
			name: "readiness wrong kind",
			mutate: func(readiness *BasePackagePublicationReadinessContract, _ *BasePackagePublicationProofEvidence) {
				readiness.Kind = BasePromotionRollbackReviewContractKind
			},
			wantErr: "publication readiness kind",
		},
		{
			name: "readiness boundary drift",
			mutate: func(readiness *BasePackagePublicationReadinessContract, _ *BasePackagePublicationProofEvidence) {
				readiness.Boundary = BasePromotionRollbackReviewBoundary
			},
			wantErr: "publication readiness boundary",
		},
		{
			name: "readiness scope drift",
			mutate: func(readiness *BasePackagePublicationReadinessContract, _ *BasePackagePublicationProofEvidence) {
				readiness.Scope = BasePromotionRollbackReviewScope
			},
			wantErr: "publication readiness scope",
		},
		{
			name: "readiness invalid version",
			mutate: func(readiness *BasePackagePublicationReadinessContract, _ *BasePackagePublicationProofEvidence) {
				readiness.Version.ArtifactProgramRef = "  "
			},
			wantErr: "publication readiness version is invalid",
		},
		{
			name: "readiness artifact ref drift",
			mutate: func(readiness *BasePackagePublicationReadinessContract, _ *BasePackagePublicationProofEvidence) {
				readiness.TypedArtifactProgramRef = "artifact-program:base-different"
			},
			wantErr: "publication readiness typed artifact-program ref is invalid",
		},
		{
			name: "readiness status not ready",
			mutate: func(readiness *BasePackagePublicationReadinessContract, _ *BasePackagePublicationProofEvidence) {
				readiness.ReadinessStatus = "ready_for_publication_execution"
			},
			wantErr: "publication readiness must be ready",
		},
		{
			name: "readiness owner approval missing",
			mutate: func(readiness *BasePackagePublicationReadinessContract, _ *BasePackagePublicationProofEvidence) {
				readiness.OwnerApproved = false
			},
			wantErr: "publication readiness must be ready",
		},
		{
			name: "readiness promotion rollback review not ready",
			mutate: func(readiness *BasePackagePublicationReadinessContract, _ *BasePackagePublicationProofEvidence) {
				readiness.PromotionRollbackReviewReady = false
			},
			wantErr: "publication readiness must be ready",
		},
		{
			name: "readiness package publication not ready",
			mutate: func(readiness *BasePackagePublicationReadinessContract, _ *BasePackagePublicationProofEvidence) {
				readiness.PackagePublicationReady = false
			},
			wantErr: "publication readiness must be ready",
		},
		{
			name: "readiness missing promotion review ref",
			mutate: func(readiness *BasePackagePublicationReadinessContract, _ *BasePackagePublicationProofEvidence) {
				readiness.PromotionRollbackReviewRef = "  "
			},
			wantErr: "publication readiness refs are required",
		},
		{
			name: "readiness missing package manifest ref",
			mutate: func(readiness *BasePackagePublicationReadinessContract, _ *BasePackagePublicationProofEvidence) {
				readiness.PackageManifestRef = "\t"
			},
			wantErr: "publication readiness refs are required",
		},
		{
			name: "readiness missing publication payload ref",
			mutate: func(readiness *BasePackagePublicationReadinessContract, _ *BasePackagePublicationProofEvidence) {
				readiness.PublicationPayloadRef = ""
			},
			wantErr: "publication readiness refs are required",
		},
		{
			name: "readiness missing publication target ref",
			mutate: func(readiness *BasePackagePublicationReadinessContract, _ *BasePackagePublicationProofEvidence) {
				readiness.PublicationTargetRef = "  "
			},
			wantErr: "publication readiness refs are required",
		},
		{
			name: "readiness missing publication policy ref",
			mutate: func(readiness *BasePackagePublicationReadinessContract, _ *BasePackagePublicationProofEvidence) {
				readiness.PublicationPolicyRef = "\t"
			},
			wantErr: "publication readiness refs are required",
		},
		{
			name: "readiness missing publication dry-run plan ref",
			mutate: func(readiness *BasePackagePublicationReadinessContract, _ *BasePackagePublicationProofEvidence) {
				readiness.PublicationDryRunPlanRef = ""
			},
			wantErr: "publication readiness refs are required",
		},
		{
			name: "readiness missing rollback plan ref",
			mutate: func(readiness *BasePackagePublicationReadinessContract, _ *BasePackagePublicationProofEvidence) {
				readiness.RollbackPlanRef = "  "
			},
			wantErr: "publication readiness refs are required",
		},
		{
			name: "readiness missing run acceptance proof requirement",
			mutate: func(readiness *BasePackagePublicationReadinessContract, _ *BasePackagePublicationProofEvidence) {
				readiness.RunAcceptanceProofRequired = false
			},
			wantErr: "publication readiness must preserve downstream proof requirements",
		},
		{
			name: "readiness missing full substrate proof requirement",
			mutate: func(readiness *BasePackagePublicationReadinessContract, _ *BasePackagePublicationProofEvidence) {
				readiness.FullSubstrateProofRequired = false
			},
			wantErr: "publication readiness must preserve downstream proof requirements",
		},
		{
			name: "readiness allows package publication",
			mutate: func(readiness *BasePackagePublicationReadinessContract, _ *BasePackagePublicationProofEvidence) {
				readiness.PackagePublicationAllowed = true
			},
			wantErr: "publication readiness allows downstream execution",
		},
		{
			name: "readiness allows promotion",
			mutate: func(readiness *BasePackagePublicationReadinessContract, _ *BasePackagePublicationProofEvidence) {
				readiness.PromotionAllowed = true
			},
			wantErr: "publication readiness allows downstream execution",
		},
		{
			name: "readiness allows run acceptance synthesis",
			mutate: func(readiness *BasePackagePublicationReadinessContract, _ *BasePackagePublicationProofEvidence) {
				readiness.RunAcceptanceSynthesisAllowed = true
			},
			wantErr: "publication readiness allows downstream execution",
		},
		{
			name: "readiness missing no package publication mutation flag",
			mutate: func(readiness *BasePackagePublicationReadinessContract, _ *BasePackagePublicationProofEvidence) {
				readiness.NoPackagePublicationMutation = false
			},
			wantErr: "publication readiness must prove no package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "readiness missing no promotion mutation flag",
			mutate: func(readiness *BasePackagePublicationReadinessContract, _ *BasePackagePublicationProofEvidence) {
				readiness.NoPromotionMutation = false
			},
			wantErr: "publication readiness must prove no package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "readiness missing no run acceptance mutation flag",
			mutate: func(readiness *BasePackagePublicationReadinessContract, _ *BasePackagePublicationProofEvidence) {
				readiness.NoRunAcceptanceMutation = false
			},
			wantErr: "publication readiness must prove no package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "readiness missing no production mutation flag",
			mutate: func(readiness *BasePackagePublicationReadinessContract, _ *BasePackagePublicationProofEvidence) {
				readiness.NoProductionMutation = false
			},
			wantErr: "publication readiness must prove no package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "readiness claims package published",
			mutate: func(readiness *BasePackagePublicationReadinessContract, _ *BasePackagePublicationProofEvidence) {
				readiness.PackagePublished = true
			},
			wantErr: "publication readiness carries downstream execution or completion claims",
		},
		{
			name: "readiness claims promotion executed",
			mutate: func(readiness *BasePackagePublicationReadinessContract, _ *BasePackagePublicationProofEvidence) {
				readiness.PromotionExecuted = true
			},
			wantErr: "publication readiness carries downstream execution or completion claims",
		},
		{
			name: "readiness claims run acceptance touched",
			mutate: func(readiness *BasePackagePublicationReadinessContract, _ *BasePackagePublicationProofEvidence) {
				readiness.RunAcceptanceRecordTouched = true
			},
			wantErr: "publication readiness carries downstream execution or completion claims",
		},
		{
			name: "readiness claims full substrate",
			mutate: func(readiness *BasePackagePublicationReadinessContract, _ *BasePackagePublicationProofEvidence) {
				readiness.FullSubstrateClaimed = true
			},
			wantErr: "publication readiness carries downstream execution or completion claims",
		},
		{
			name: "readiness claims completion",
			mutate: func(readiness *BasePackagePublicationReadinessContract, _ *BasePackagePublicationProofEvidence) {
				readiness.CompletionClaimed = true
			},
			wantErr: "publication readiness carries downstream execution or completion claims",
		},
		{
			name: "missing evidence publication readiness ref",
			mutate: func(_ *BasePackagePublicationReadinessContract, evidence *BasePackagePublicationProofEvidence) {
				evidence.PublicationReadinessRef = "  "
			},
			wantErr: "publication proof refs are required",
		},
		{
			name: "missing evidence publication proof ref",
			mutate: func(_ *BasePackagePublicationReadinessContract, evidence *BasePackagePublicationProofEvidence) {
				evidence.PublicationProofRef = "\t"
			},
			wantErr: "publication proof refs are required",
		},
		{
			name: "missing evidence published package ref",
			mutate: func(_ *BasePackagePublicationReadinessContract, evidence *BasePackagePublicationProofEvidence) {
				evidence.PublishedPackageRef = ""
			},
			wantErr: "publication proof refs are required",
		},
		{
			name: "missing evidence package digest ref",
			mutate: func(_ *BasePackagePublicationReadinessContract, evidence *BasePackagePublicationProofEvidence) {
				evidence.PackageDigestRef = "  "
			},
			wantErr: "publication proof refs are required",
		},
		{
			name: "missing evidence publication receipt ref",
			mutate: func(_ *BasePackagePublicationReadinessContract, evidence *BasePackagePublicationProofEvidence) {
				evidence.PublicationReceiptRef = "\t"
			},
			wantErr: "publication proof refs are required",
		},
		{
			name: "missing evidence publication ledger ref",
			mutate: func(_ *BasePackagePublicationReadinessContract, evidence *BasePackagePublicationProofEvidence) {
				evidence.PublicationLedgerRef = ""
			},
			wantErr: "publication proof refs are required",
		},
		{
			name: "missing evidence publication review ref",
			mutate: func(_ *BasePackagePublicationReadinessContract, evidence *BasePackagePublicationProofEvidence) {
				evidence.PublicationReviewRef = "  "
			},
			wantErr: "publication proof refs are required",
		},
		{
			name: "missing evidence rollback plan ref",
			mutate: func(_ *BasePackagePublicationReadinessContract, evidence *BasePackagePublicationProofEvidence) {
				evidence.RollbackPlanRef = "\t"
			},
			wantErr: "publication proof refs are required",
		},
		{
			name: "evidence missing no package publication mutation flag",
			mutate: func(_ *BasePackagePublicationReadinessContract, evidence *BasePackagePublicationProofEvidence) {
				evidence.NoPackagePublicationMutation = false
			},
			wantErr: "evidence must prove no package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "evidence missing no promotion mutation flag",
			mutate: func(_ *BasePackagePublicationReadinessContract, evidence *BasePackagePublicationProofEvidence) {
				evidence.NoPromotionMutation = false
			},
			wantErr: "evidence must prove no package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "evidence missing no run acceptance mutation flag",
			mutate: func(_ *BasePackagePublicationReadinessContract, evidence *BasePackagePublicationProofEvidence) {
				evidence.NoRunAcceptanceMutation = false
			},
			wantErr: "evidence must prove no package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "evidence missing no production mutation flag",
			mutate: func(_ *BasePackagePublicationReadinessContract, evidence *BasePackagePublicationProofEvidence) {
				evidence.NoProductionMutation = false
			},
			wantErr: "evidence must prove no package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "evidence claims package published",
			mutate: func(_ *BasePackagePublicationReadinessContract, evidence *BasePackagePublicationProofEvidence) {
				evidence.PackagePublished = true
			},
			wantErr: "evidence carries downstream execution, production mutation, or completion claims",
		},
		{
			name: "evidence claims promotion executed",
			mutate: func(_ *BasePackagePublicationReadinessContract, evidence *BasePackagePublicationProofEvidence) {
				evidence.PromotionExecuted = true
			},
			wantErr: "evidence carries downstream execution, production mutation, or completion claims",
		},
		{
			name: "evidence claims run acceptance touched",
			mutate: func(_ *BasePackagePublicationReadinessContract, evidence *BasePackagePublicationProofEvidence) {
				evidence.RunAcceptanceRecordTouched = true
			},
			wantErr: "evidence carries downstream execution, production mutation, or completion claims",
		},
		{
			name: "evidence claims production state mutated",
			mutate: func(_ *BasePackagePublicationReadinessContract, evidence *BasePackagePublicationProofEvidence) {
				evidence.ProductionStateMutated = true
			},
			wantErr: "evidence carries downstream execution, production mutation, or completion claims",
		},
		{
			name: "evidence claims full substrate",
			mutate: func(_ *BasePackagePublicationReadinessContract, evidence *BasePackagePublicationProofEvidence) {
				evidence.FullSubstrateClaimed = true
			},
			wantErr: "evidence carries downstream execution, production mutation, or completion claims",
		},
		{
			name: "evidence claims completion",
			mutate: func(_ *BasePackagePublicationReadinessContract, evidence *BasePackagePublicationProofEvidence) {
				evidence.CompletionClaimed = true
			},
			wantErr: "evidence carries downstream execution, production mutation, or completion claims",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			readiness, evidence := basePackagePublicationProofContractInputs(t)
			tc.mutate(&readiness, &evidence)

			contract, err := BuildBasePackagePublicationProofContract(readiness, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildBasePackagePublicationProofContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func basePackagePublicationProofContractInputs(t *testing.T) (BasePackagePublicationReadinessContract, BasePackagePublicationProofEvidence) {
	t.Helper()

	review, readinessEvidence := basePackagePublicationReadinessContractInputs(t)
	readiness, err := BuildBasePackagePublicationReadinessContract(review, readinessEvidence)
	if err != nil {
		t.Fatalf("BuildBasePackagePublicationReadinessContract(): %v", err)
	}
	return readiness, basePackagePublicationProofEvidence(readiness)
}

func basePackagePublicationProofEvidence(readiness BasePackagePublicationReadinessContract) BasePackagePublicationProofEvidence {
	return BasePackagePublicationProofEvidence{
		PublicationReadinessRef:      " publication-readiness:base-pass-113 ",
		PublicationProofRef:          " publication-proof:base-pass-113 ",
		PublishedPackageRef:          " package-publication-record:base-pass-113 ",
		PackageDigestRef:             " package-digest:base-pass-113 ",
		PublicationReceiptRef:        " publication-receipt:base-pass-113 ",
		PublicationLedgerRef:         " publication-ledger:base-pass-113 ",
		PublicationReviewRef:         " publication-review:base-pass-113 ",
		RollbackPlanRef:              " " + readiness.RollbackPlanRef + " ",
		NoPackagePublicationMutation: true,
		NoPromotionMutation:          true,
		NoRunAcceptanceMutation:      true,
		NoProductionMutation:         true,
		PackagePublished:             false,
		PromotionExecuted:            false,
		RunAcceptanceRecordTouched:   false,
		ProductionStateMutated:       false,
		FullSubstrateClaimed:         false,
		CompletionClaimed:            false,
	}
}
