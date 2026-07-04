package computerversion

import (
	"strings"
	"testing"
)

func TestBuildBasePromotionExecutionReadinessContractRecordsReadinessOnly(t *testing.T) {
	proof, evidence := basePromotionExecutionReadinessContractInputs(t)

	contract, err := BuildBasePromotionExecutionReadinessContract(proof, evidence)
	if err != nil {
		t.Fatalf("BuildBasePromotionExecutionReadinessContract(): %v", err)
	}

	if contract.Kind != BasePromotionExecutionReadinessContractKind {
		t.Fatalf("kind = %q, want %q", contract.Kind, BasePromotionExecutionReadinessContractKind)
	}
	if contract.Boundary != BasePromotionExecutionReadinessBoundary {
		t.Fatalf("boundary = %q, want %q", contract.Boundary, BasePromotionExecutionReadinessBoundary)
	}
	if contract.Scope != BasePromotionExecutionReadinessScope {
		t.Fatalf("scope = %q, want %q", contract.Scope, BasePromotionExecutionReadinessScope)
	}
	if contract.Version != proof.Version || ArtifactProgramRef(contract.TypedArtifactProgramRef) != proof.Version.ArtifactProgramRef {
		t.Fatalf("version/ref = %#v/%q, want package-publication proof version %#v and artifact program %q", contract.Version, contract.TypedArtifactProgramRef, proof.Version, proof.Version.ArtifactProgramRef)
	}
	if contract.PackagePublicationProofRef != strings.TrimSpace(evidence.PackagePublicationProofRef) || contract.PublicationReadinessRef != strings.TrimSpace(proof.PublicationReadinessRef) || contract.PublicationProofRef != strings.TrimSpace(proof.PublicationProofRef) || contract.PublishedPackageRef != strings.TrimSpace(proof.PublishedPackageRef) || contract.PackageDigestRef != strings.TrimSpace(proof.PackageDigestRef) || contract.PublicationLedgerRef != strings.TrimSpace(proof.PublicationLedgerRef) {
		t.Fatalf("publication refs = package-proof %q readiness %q proof %q package %q digest %q ledger %q, want trimmed proof/evidence refs from proof %#v evidence %#v", contract.PackagePublicationProofRef, contract.PublicationReadinessRef, contract.PublicationProofRef, contract.PublishedPackageRef, contract.PackageDigestRef, contract.PublicationLedgerRef, proof, evidence)
	}
	if contract.PromotionCandidateRef != strings.TrimSpace(evidence.PromotionCandidateRef) || contract.PromotionExecutionPlanRef != strings.TrimSpace(evidence.PromotionExecutionPlanRef) || contract.PromotionPreflightRef != strings.TrimSpace(evidence.PromotionPreflightRef) || contract.PromotionOperatorPolicyRef != strings.TrimSpace(evidence.PromotionOperatorPolicyRef) || contract.PromotionRollbackPlanRef != strings.TrimSpace(evidence.PromotionRollbackPlanRef) || contract.RouteCutoverPlanRef != strings.TrimSpace(evidence.RouteCutoverPlanRef) || contract.LedgerFreshnessCheckRef != strings.TrimSpace(evidence.LedgerFreshnessCheckRef) {
		t.Fatalf("promotion readiness refs = candidate %q execution-plan %q preflight %q policy %q rollback %q cutover %q ledger %q, want trimmed evidence refs from %#v", contract.PromotionCandidateRef, contract.PromotionExecutionPlanRef, contract.PromotionPreflightRef, contract.PromotionOperatorPolicyRef, contract.PromotionRollbackPlanRef, contract.RouteCutoverPlanRef, contract.LedgerFreshnessCheckRef, evidence)
	}
	if contract.ReadinessStatus != BasePromotionExecutionReadinessStatusReady || !contract.OwnerApproved || !contract.PackagePublicationProof || !contract.PromotionExecutionReady {
		t.Fatalf("promotion execution readiness = status %q owner %v package-proof %v promotion-ready %v, want ready promotion-execution prerequisites from satisfied package-publication proof", contract.ReadinessStatus, contract.OwnerApproved, contract.PackagePublicationProof, contract.PromotionExecutionReady)
	}
	if !contract.PromotionProofRequired || !contract.RunAcceptanceProofRequired || !contract.FullSubstrateProofRequired {
		t.Fatalf("promotion execution readiness must preserve promotion, run-acceptance, and full-substrate proof requirements: %#v", contract)
	}
	if contract.PackagePublicationAllowed || contract.PromotionAllowed || contract.RunAcceptanceSynthesisAllowed {
		t.Fatalf("promotion execution readiness must not allow package publication, promotion, or run-acceptance synthesis: %#v", contract)
	}
	if !contract.NoPackagePublicationMutation || !contract.NoPromotionMutation || !contract.NoRunAcceptanceMutation || !contract.NoProductionMutation {
		t.Fatalf("promotion execution readiness must carry no-publication/no-promotion/no-run-acceptance/no-production mutation evidence: %#v", contract)
	}
	if contract.PackagePublished || contract.PromotionExecuted || contract.RunAcceptanceRecordTouched || contract.ProductionStateMutated || contract.FullSubstrateClaimed || contract.CompletionClaimed {
		t.Fatalf("promotion execution readiness must not publish, promote, touch run acceptance, mutate production, claim full substrate, or claim completion: %#v", contract)
	}
}

func TestBuildBasePromotionExecutionReadinessContractRejectsInvalidInputs(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*BasePackagePublicationProofContract, *BasePromotionExecutionReadinessEvidence)
		wantErr string
	}{
		{
			name: "proof wrong kind",
			mutate: func(proof *BasePackagePublicationProofContract, _ *BasePromotionExecutionReadinessEvidence) {
				proof.Kind = BasePackagePublicationReadinessContractKind
			},
			wantErr: "package-publication proof kind",
		},
		{
			name: "proof boundary drift",
			mutate: func(proof *BasePackagePublicationProofContract, _ *BasePromotionExecutionReadinessEvidence) {
				proof.Boundary = BasePackagePublicationReadinessBoundary
			},
			wantErr: "package-publication proof boundary",
		},
		{
			name: "proof scope drift",
			mutate: func(proof *BasePackagePublicationProofContract, _ *BasePromotionExecutionReadinessEvidence) {
				proof.Scope = BasePackagePublicationReadinessScope
			},
			wantErr: "package-publication proof scope",
		},
		{
			name: "proof invalid version",
			mutate: func(proof *BasePackagePublicationProofContract, _ *BasePromotionExecutionReadinessEvidence) {
				proof.Version.ArtifactProgramRef = "  "
			},
			wantErr: "package-publication proof version is invalid",
		},
		{
			name: "proof artifact ref drift",
			mutate: func(proof *BasePackagePublicationProofContract, _ *BasePromotionExecutionReadinessEvidence) {
				proof.TypedArtifactProgramRef = "artifact-program:base-different"
			},
			wantErr: "package-publication proof typed artifact-program ref is invalid",
		},
		{
			name: "proof status not satisfied",
			mutate: func(proof *BasePackagePublicationProofContract, _ *BasePromotionExecutionReadinessEvidence) {
				proof.ProofStatus = "package_publication_proof_pending"
			},
			wantErr: "package-publication proof must be satisfied",
		},
		{
			name: "proof owner approval missing",
			mutate: func(proof *BasePackagePublicationProofContract, _ *BasePromotionExecutionReadinessEvidence) {
				proof.OwnerApproved = false
			},
			wantErr: "package-publication proof must be satisfied",
		},
		{
			name: "proof marker missing",
			mutate: func(proof *BasePackagePublicationProofContract, _ *BasePromotionExecutionReadinessEvidence) {
				proof.PackagePublicationProof = false
			},
			wantErr: "package-publication proof must be satisfied",
		},
		{
			name: "proof missing publication readiness ref",
			mutate: func(proof *BasePackagePublicationProofContract, _ *BasePromotionExecutionReadinessEvidence) {
				proof.PublicationReadinessRef = "  "
			},
			wantErr: "package-publication proof refs are required",
		},
		{
			name: "proof missing publication proof ref",
			mutate: func(proof *BasePackagePublicationProofContract, _ *BasePromotionExecutionReadinessEvidence) {
				proof.PublicationProofRef = "\t"
			},
			wantErr: "package-publication proof refs are required",
		},
		{
			name: "proof missing published package ref",
			mutate: func(proof *BasePackagePublicationProofContract, _ *BasePromotionExecutionReadinessEvidence) {
				proof.PublishedPackageRef = ""
			},
			wantErr: "package-publication proof refs are required",
		},
		{
			name: "proof missing package digest ref",
			mutate: func(proof *BasePackagePublicationProofContract, _ *BasePromotionExecutionReadinessEvidence) {
				proof.PackageDigestRef = "  "
			},
			wantErr: "package-publication proof refs are required",
		},
		{
			name: "proof missing publication ledger ref",
			mutate: func(proof *BasePackagePublicationProofContract, _ *BasePromotionExecutionReadinessEvidence) {
				proof.PublicationLedgerRef = "\t"
			},
			wantErr: "package-publication proof refs are required",
		},
		{
			name: "proof missing rollback plan ref",
			mutate: func(proof *BasePackagePublicationProofContract, _ *BasePromotionExecutionReadinessEvidence) {
				proof.RollbackPlanRef = ""
			},
			wantErr: "package-publication proof refs are required",
		},
		{
			name: "proof missing promotion proof requirement",
			mutate: func(proof *BasePackagePublicationProofContract, _ *BasePromotionExecutionReadinessEvidence) {
				proof.PromotionProofRequired = false
			},
			wantErr: "package-publication proof must preserve downstream proof requirements",
		},
		{
			name: "proof missing run acceptance proof requirement",
			mutate: func(proof *BasePackagePublicationProofContract, _ *BasePromotionExecutionReadinessEvidence) {
				proof.RunAcceptanceProofRequired = false
			},
			wantErr: "package-publication proof must preserve downstream proof requirements",
		},
		{
			name: "proof missing full substrate proof requirement",
			mutate: func(proof *BasePackagePublicationProofContract, _ *BasePromotionExecutionReadinessEvidence) {
				proof.FullSubstrateProofRequired = false
			},
			wantErr: "package-publication proof must preserve downstream proof requirements",
		},
		{
			name: "proof allows package publication",
			mutate: func(proof *BasePackagePublicationProofContract, _ *BasePromotionExecutionReadinessEvidence) {
				proof.PackagePublicationAllowed = true
			},
			wantErr: "package-publication proof allows downstream execution",
		},
		{
			name: "proof allows promotion",
			mutate: func(proof *BasePackagePublicationProofContract, _ *BasePromotionExecutionReadinessEvidence) {
				proof.PromotionAllowed = true
			},
			wantErr: "package-publication proof allows downstream execution",
		},
		{
			name: "proof allows run acceptance synthesis",
			mutate: func(proof *BasePackagePublicationProofContract, _ *BasePromotionExecutionReadinessEvidence) {
				proof.RunAcceptanceSynthesisAllowed = true
			},
			wantErr: "package-publication proof allows downstream execution",
		},
		{
			name: "proof missing no package publication mutation flag",
			mutate: func(proof *BasePackagePublicationProofContract, _ *BasePromotionExecutionReadinessEvidence) {
				proof.NoPackagePublicationMutation = false
			},
			wantErr: "package-publication proof must prove no package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "proof missing no promotion mutation flag",
			mutate: func(proof *BasePackagePublicationProofContract, _ *BasePromotionExecutionReadinessEvidence) {
				proof.NoPromotionMutation = false
			},
			wantErr: "package-publication proof must prove no package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "proof missing no run acceptance mutation flag",
			mutate: func(proof *BasePackagePublicationProofContract, _ *BasePromotionExecutionReadinessEvidence) {
				proof.NoRunAcceptanceMutation = false
			},
			wantErr: "package-publication proof must prove no package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "proof missing no production mutation flag",
			mutate: func(proof *BasePackagePublicationProofContract, _ *BasePromotionExecutionReadinessEvidence) {
				proof.NoProductionMutation = false
			},
			wantErr: "package-publication proof must prove no package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "proof claims package published",
			mutate: func(proof *BasePackagePublicationProofContract, _ *BasePromotionExecutionReadinessEvidence) {
				proof.PackagePublished = true
			},
			wantErr: "package-publication proof carries downstream execution, production mutation, or completion claims",
		},
		{
			name: "proof claims promotion executed",
			mutate: func(proof *BasePackagePublicationProofContract, _ *BasePromotionExecutionReadinessEvidence) {
				proof.PromotionExecuted = true
			},
			wantErr: "package-publication proof carries downstream execution, production mutation, or completion claims",
		},
		{
			name: "proof claims run acceptance touched",
			mutate: func(proof *BasePackagePublicationProofContract, _ *BasePromotionExecutionReadinessEvidence) {
				proof.RunAcceptanceRecordTouched = true
			},
			wantErr: "package-publication proof carries downstream execution, production mutation, or completion claims",
		},
		{
			name: "proof claims production state mutated",
			mutate: func(proof *BasePackagePublicationProofContract, _ *BasePromotionExecutionReadinessEvidence) {
				proof.ProductionStateMutated = true
			},
			wantErr: "package-publication proof carries downstream execution, production mutation, or completion claims",
		},
		{
			name: "proof claims full substrate",
			mutate: func(proof *BasePackagePublicationProofContract, _ *BasePromotionExecutionReadinessEvidence) {
				proof.FullSubstrateClaimed = true
			},
			wantErr: "package-publication proof carries downstream execution, production mutation, or completion claims",
		},
		{
			name: "proof claims completion",
			mutate: func(proof *BasePackagePublicationProofContract, _ *BasePromotionExecutionReadinessEvidence) {
				proof.CompletionClaimed = true
			},
			wantErr: "package-publication proof carries downstream execution, production mutation, or completion claims",
		},
		{
			name: "evidence missing package-publication proof ref",
			mutate: func(_ *BasePackagePublicationProofContract, evidence *BasePromotionExecutionReadinessEvidence) {
				evidence.PackagePublicationProofRef = "  "
			},
			wantErr: "promotion readiness refs are required",
		},
		{
			name: "evidence missing promotion candidate ref",
			mutate: func(_ *BasePackagePublicationProofContract, evidence *BasePromotionExecutionReadinessEvidence) {
				evidence.PromotionCandidateRef = "\t"
			},
			wantErr: "promotion readiness refs are required",
		},
		{
			name: "evidence missing promotion execution plan ref",
			mutate: func(_ *BasePackagePublicationProofContract, evidence *BasePromotionExecutionReadinessEvidence) {
				evidence.PromotionExecutionPlanRef = ""
			},
			wantErr: "promotion readiness refs are required",
		},
		{
			name: "evidence missing promotion preflight ref",
			mutate: func(_ *BasePackagePublicationProofContract, evidence *BasePromotionExecutionReadinessEvidence) {
				evidence.PromotionPreflightRef = "  "
			},
			wantErr: "promotion readiness refs are required",
		},
		{
			name: "evidence missing promotion operator policy ref",
			mutate: func(_ *BasePackagePublicationProofContract, evidence *BasePromotionExecutionReadinessEvidence) {
				evidence.PromotionOperatorPolicyRef = "\t"
			},
			wantErr: "promotion readiness refs are required",
		},
		{
			name: "evidence missing promotion rollback plan ref",
			mutate: func(_ *BasePackagePublicationProofContract, evidence *BasePromotionExecutionReadinessEvidence) {
				evidence.PromotionRollbackPlanRef = ""
			},
			wantErr: "promotion readiness refs are required",
		},
		{
			name: "evidence missing route cutover plan ref",
			mutate: func(_ *BasePackagePublicationProofContract, evidence *BasePromotionExecutionReadinessEvidence) {
				evidence.RouteCutoverPlanRef = "  "
			},
			wantErr: "promotion readiness refs are required",
		},
		{
			name: "evidence missing ledger freshness check ref",
			mutate: func(_ *BasePackagePublicationProofContract, evidence *BasePromotionExecutionReadinessEvidence) {
				evidence.LedgerFreshnessCheckRef = "\t"
			},
			wantErr: "promotion readiness refs are required",
		},
		{
			name: "evidence missing no package publication mutation flag",
			mutate: func(_ *BasePackagePublicationProofContract, evidence *BasePromotionExecutionReadinessEvidence) {
				evidence.NoPackagePublicationMutation = false
			},
			wantErr: "evidence must prove no package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "evidence missing no promotion mutation flag",
			mutate: func(_ *BasePackagePublicationProofContract, evidence *BasePromotionExecutionReadinessEvidence) {
				evidence.NoPromotionMutation = false
			},
			wantErr: "evidence must prove no package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "evidence missing no run acceptance mutation flag",
			mutate: func(_ *BasePackagePublicationProofContract, evidence *BasePromotionExecutionReadinessEvidence) {
				evidence.NoRunAcceptanceMutation = false
			},
			wantErr: "evidence must prove no package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "evidence missing no production mutation flag",
			mutate: func(_ *BasePackagePublicationProofContract, evidence *BasePromotionExecutionReadinessEvidence) {
				evidence.NoProductionMutation = false
			},
			wantErr: "evidence must prove no package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "evidence claims package published",
			mutate: func(_ *BasePackagePublicationProofContract, evidence *BasePromotionExecutionReadinessEvidence) {
				evidence.PackagePublished = true
			},
			wantErr: "evidence carries downstream execution, production mutation, or completion claims",
		},
		{
			name: "evidence claims promotion executed",
			mutate: func(_ *BasePackagePublicationProofContract, evidence *BasePromotionExecutionReadinessEvidence) {
				evidence.PromotionExecuted = true
			},
			wantErr: "evidence carries downstream execution, production mutation, or completion claims",
		},
		{
			name: "evidence claims run acceptance touched",
			mutate: func(_ *BasePackagePublicationProofContract, evidence *BasePromotionExecutionReadinessEvidence) {
				evidence.RunAcceptanceRecordTouched = true
			},
			wantErr: "evidence carries downstream execution, production mutation, or completion claims",
		},
		{
			name: "evidence claims production state mutated",
			mutate: func(_ *BasePackagePublicationProofContract, evidence *BasePromotionExecutionReadinessEvidence) {
				evidence.ProductionStateMutated = true
			},
			wantErr: "evidence carries downstream execution, production mutation, or completion claims",
		},
		{
			name: "evidence claims full substrate",
			mutate: func(_ *BasePackagePublicationProofContract, evidence *BasePromotionExecutionReadinessEvidence) {
				evidence.FullSubstrateClaimed = true
			},
			wantErr: "evidence carries downstream execution, production mutation, or completion claims",
		},
		{
			name: "evidence claims completion",
			mutate: func(_ *BasePackagePublicationProofContract, evidence *BasePromotionExecutionReadinessEvidence) {
				evidence.CompletionClaimed = true
			},
			wantErr: "evidence carries downstream execution, production mutation, or completion claims",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			proof, evidence := basePromotionExecutionReadinessContractInputs(t)
			tc.mutate(&proof, &evidence)

			contract, err := BuildBasePromotionExecutionReadinessContract(proof, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildBasePromotionExecutionReadinessContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func basePromotionExecutionReadinessContractInputs(t *testing.T) (BasePackagePublicationProofContract, BasePromotionExecutionReadinessEvidence) {
	t.Helper()

	readiness, proofEvidence := basePackagePublicationProofContractInputs(t)
	proof, err := BuildBasePackagePublicationProofContract(readiness, proofEvidence)
	if err != nil {
		t.Fatalf("BuildBasePackagePublicationProofContract(): %v", err)
	}
	return proof, basePromotionExecutionReadinessEvidence(proof)
}

func basePromotionExecutionReadinessEvidence(proof BasePackagePublicationProofContract) BasePromotionExecutionReadinessEvidence {
	return BasePromotionExecutionReadinessEvidence{
		PackagePublicationProofRef:   " package-publication-proof:base-pass-114 ",
		PromotionCandidateRef:        " promotion-candidate:base-pass-114 ",
		PromotionExecutionPlanRef:    " promotion-execution-plan:base-pass-114 ",
		PromotionPreflightRef:        " promotion-preflight:base-pass-114 ",
		PromotionOperatorPolicyRef:   " promotion-operator-policy:base-pass-114 ",
		PromotionRollbackPlanRef:     " " + proof.RollbackPlanRef + " ",
		RouteCutoverPlanRef:          " route-cutover-plan:base-pass-114 ",
		LedgerFreshnessCheckRef:      " ledger-freshness:base-pass-114 ",
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
