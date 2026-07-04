package computerversion

import (
	"strings"
	"testing"
)

func TestBuildBasePromotionResultContractAcceptsBlockedAndNoopResultsOnly(t *testing.T) {
	for _, tc := range []struct {
		name    string
		outcome string
	}{
		{name: "blocked", outcome: BasePromotionResultOutcomeBlocked},
		{name: "noop", outcome: BasePromotionResultOutcomeNoop},
	} {
		t.Run(tc.name, func(t *testing.T) {
			readiness, evidence := basePromotionResultContractInputs(t, tc.outcome)

			contract, err := BuildBasePromotionResultContract(readiness, evidence)
			if err != nil {
				t.Fatalf("BuildBasePromotionResultContract(): %v", err)
			}

			if contract.Kind != BasePromotionResultContractKind {
				t.Fatalf("kind = %q, want %q", contract.Kind, BasePromotionResultContractKind)
			}
			if contract.Boundary != BasePromotionResultBoundary {
				t.Fatalf("boundary = %q, want %q", contract.Boundary, BasePromotionResultBoundary)
			}
			if contract.Scope != BasePromotionResultScope {
				t.Fatalf("scope = %q, want %q", contract.Scope, BasePromotionResultScope)
			}
			if contract.Version != readiness.Version || ArtifactProgramRef(contract.TypedArtifactProgramRef) != readiness.Version.ArtifactProgramRef {
				t.Fatalf("version/ref = %#v/%q, want readiness version %#v and artifact program %q", contract.Version, contract.TypedArtifactProgramRef, readiness.Version, readiness.Version.ArtifactProgramRef)
			}
			if contract.PromotionReadinessRef != strings.TrimSpace(evidence.PromotionReadinessRef) || contract.PackagePublicationProofRef != strings.TrimSpace(readiness.PackagePublicationProofRef) || contract.PromotionCandidateRef != strings.TrimSpace(readiness.PromotionCandidateRef) || contract.PromotionExecutionPlanRef != strings.TrimSpace(readiness.PromotionExecutionPlanRef) || contract.PromotionPreflightRef != strings.TrimSpace(readiness.PromotionPreflightRef) || contract.PromotionOperatorPolicyRef != strings.TrimSpace(readiness.PromotionOperatorPolicyRef) || contract.PromotionRollbackPlanRef != strings.TrimSpace(evidence.RollbackPlanRef) || contract.RouteCutoverPlanRef != strings.TrimSpace(readiness.RouteCutoverPlanRef) || contract.LedgerFreshnessCheckRef != strings.TrimSpace(readiness.LedgerFreshnessCheckRef) {
				t.Fatalf("readiness refs = readiness %q package-proof %q candidate %q execution-plan %q preflight %q policy %q rollback %q cutover %q ledger %q, want trimmed refs from readiness %#v and evidence %#v", contract.PromotionReadinessRef, contract.PackagePublicationProofRef, contract.PromotionCandidateRef, contract.PromotionExecutionPlanRef, contract.PromotionPreflightRef, contract.PromotionOperatorPolicyRef, contract.PromotionRollbackPlanRef, contract.RouteCutoverPlanRef, contract.LedgerFreshnessCheckRef, readiness, evidence)
			}
			if contract.PromotionOutcomeRef != strings.TrimSpace(evidence.PromotionOutcomeRef) || contract.PromotionOutcome != tc.outcome || contract.PromotionOutcomeReasonRef != strings.TrimSpace(evidence.PromotionOutcomeReasonRef) || contract.OperatorDecisionRef != strings.TrimSpace(evidence.OperatorDecisionRef) || contract.PromotionAttemptRef != strings.TrimSpace(evidence.PromotionAttemptRef) {
				t.Fatalf("result refs/outcome = outcome-ref %q outcome %q reason %q operator %q attempt %q, want trimmed evidence refs and outcome %q from %#v", contract.PromotionOutcomeRef, contract.PromotionOutcome, contract.PromotionOutcomeReasonRef, contract.OperatorDecisionRef, contract.PromotionAttemptRef, tc.outcome, evidence)
			}
			if !contract.PromotionResultRecorded || !contract.PromotionExecutionReady {
				t.Fatalf("promotion result must record a result after promotion execution readiness: %#v", contract)
			}
			if !contract.PromotionProofRequired || !contract.RunAcceptanceProofRequired || !contract.FullSubstrateProofRequired {
				t.Fatalf("promotion result must preserve promotion, run-acceptance, and full-substrate proof requirements: %#v", contract)
			}
			if contract.PackagePublicationAllowed || contract.PromotionAllowed || contract.RunAcceptanceSynthesisAllowed {
				t.Fatalf("promotion result must not allow package publication, promotion, or run-acceptance synthesis: %#v", contract)
			}
			if !contract.NoPackagePublicationMutation || !contract.NoPromotionMutation || !contract.NoRunAcceptanceMutation || !contract.NoProductionMutation {
				t.Fatalf("promotion result must carry no-publication/no-promotion/no-run-acceptance/no-production mutation evidence: %#v", contract)
			}
			if contract.PackagePublished || contract.PromotionExecuted || contract.RunAcceptanceRecordTouched || contract.ProductionStateMutated || contract.FullSubstrateClaimed || contract.CompletionClaimed {
				t.Fatalf("promotion result must not publish, promote, touch run acceptance, mutate production, claim full substrate, or claim completion: %#v", contract)
			}
		})
	}
}

func TestBuildBasePromotionResultContractRejectsInvalidInputs(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*BasePromotionExecutionReadinessContract, *BasePromotionResultEvidence)
		wantErr string
	}{
		{
			name: "readiness wrong kind",
			mutate: func(readiness *BasePromotionExecutionReadinessContract, _ *BasePromotionResultEvidence) {
				readiness.Kind = BasePackagePublicationProofContractKind
			},
			wantErr: "promotion readiness kind",
		},
		{
			name: "readiness boundary drift",
			mutate: func(readiness *BasePromotionExecutionReadinessContract, _ *BasePromotionResultEvidence) {
				readiness.Boundary = BasePromotionResultBoundary
			},
			wantErr: "promotion readiness boundary",
		},
		{
			name: "readiness scope drift",
			mutate: func(readiness *BasePromotionExecutionReadinessContract, _ *BasePromotionResultEvidence) {
				readiness.Scope = BasePromotionResultScope
			},
			wantErr: "promotion readiness scope",
		},
		{
			name: "readiness invalid version",
			mutate: func(readiness *BasePromotionExecutionReadinessContract, _ *BasePromotionResultEvidence) {
				readiness.Version.ArtifactProgramRef = "  "
			},
			wantErr: "promotion readiness version is invalid",
		},
		{
			name: "readiness typed artifact ref drift",
			mutate: func(readiness *BasePromotionExecutionReadinessContract, _ *BasePromotionResultEvidence) {
				readiness.TypedArtifactProgramRef = "artifact-program:base-different"
			},
			wantErr: "promotion readiness typed artifact-program ref is invalid",
		},
		{
			name: "readiness missing package-publication proof ref",
			mutate: func(readiness *BasePromotionExecutionReadinessContract, _ *BasePromotionResultEvidence) {
				readiness.PackagePublicationProofRef = "  "
			},
			wantErr: "promotion readiness refs are required",
		},
		{
			name: "readiness missing promotion candidate ref",
			mutate: func(readiness *BasePromotionExecutionReadinessContract, _ *BasePromotionResultEvidence) {
				readiness.PromotionCandidateRef = "\t"
			},
			wantErr: "promotion readiness refs are required",
		},
		{
			name: "readiness missing promotion execution plan ref",
			mutate: func(readiness *BasePromotionExecutionReadinessContract, _ *BasePromotionResultEvidence) {
				readiness.PromotionExecutionPlanRef = ""
			},
			wantErr: "promotion readiness refs are required",
		},
		{
			name: "readiness missing promotion preflight ref",
			mutate: func(readiness *BasePromotionExecutionReadinessContract, _ *BasePromotionResultEvidence) {
				readiness.PromotionPreflightRef = "  "
			},
			wantErr: "promotion readiness refs are required",
		},
		{
			name: "readiness missing promotion operator policy ref",
			mutate: func(readiness *BasePromotionExecutionReadinessContract, _ *BasePromotionResultEvidence) {
				readiness.PromotionOperatorPolicyRef = "\t"
			},
			wantErr: "promotion readiness refs are required",
		},
		{
			name: "readiness missing promotion rollback plan ref",
			mutate: func(readiness *BasePromotionExecutionReadinessContract, _ *BasePromotionResultEvidence) {
				readiness.PromotionRollbackPlanRef = ""
			},
			wantErr: "promotion readiness refs are required",
		},
		{
			name: "readiness missing route cutover plan ref",
			mutate: func(readiness *BasePromotionExecutionReadinessContract, _ *BasePromotionResultEvidence) {
				readiness.RouteCutoverPlanRef = "  "
			},
			wantErr: "promotion readiness refs are required",
		},
		{
			name: "readiness missing ledger freshness check ref",
			mutate: func(readiness *BasePromotionExecutionReadinessContract, _ *BasePromotionResultEvidence) {
				readiness.LedgerFreshnessCheckRef = "\t"
			},
			wantErr: "promotion readiness refs are required",
		},
		{
			name: "readiness status drift",
			mutate: func(readiness *BasePromotionExecutionReadinessContract, _ *BasePromotionResultEvidence) {
				readiness.ReadinessStatus = "promotion_execution_readiness_pending"
			},
			wantErr: "promotion readiness must be ready",
		},
		{
			name: "readiness owner approval missing",
			mutate: func(readiness *BasePromotionExecutionReadinessContract, _ *BasePromotionResultEvidence) {
				readiness.OwnerApproved = false
			},
			wantErr: "promotion readiness must be ready",
		},
		{
			name: "readiness package-publication proof missing",
			mutate: func(readiness *BasePromotionExecutionReadinessContract, _ *BasePromotionResultEvidence) {
				readiness.PackagePublicationProof = false
			},
			wantErr: "promotion readiness must be ready",
		},
		{
			name: "readiness promotion execution ready flag missing",
			mutate: func(readiness *BasePromotionExecutionReadinessContract, _ *BasePromotionResultEvidence) {
				readiness.PromotionExecutionReady = false
			},
			wantErr: "promotion readiness must be ready",
		},
		{
			name: "readiness missing promotion proof requirement",
			mutate: func(readiness *BasePromotionExecutionReadinessContract, _ *BasePromotionResultEvidence) {
				readiness.PromotionProofRequired = false
			},
			wantErr: "promotion readiness must preserve downstream proof requirements",
		},
		{
			name: "readiness missing run acceptance proof requirement",
			mutate: func(readiness *BasePromotionExecutionReadinessContract, _ *BasePromotionResultEvidence) {
				readiness.RunAcceptanceProofRequired = false
			},
			wantErr: "promotion readiness must preserve downstream proof requirements",
		},
		{
			name: "readiness missing full substrate proof requirement",
			mutate: func(readiness *BasePromotionExecutionReadinessContract, _ *BasePromotionResultEvidence) {
				readiness.FullSubstrateProofRequired = false
			},
			wantErr: "promotion readiness must preserve downstream proof requirements",
		},
		{
			name: "readiness allows package publication",
			mutate: func(readiness *BasePromotionExecutionReadinessContract, _ *BasePromotionResultEvidence) {
				readiness.PackagePublicationAllowed = true
			},
			wantErr: "promotion readiness allows downstream execution",
		},
		{
			name: "readiness allows promotion",
			mutate: func(readiness *BasePromotionExecutionReadinessContract, _ *BasePromotionResultEvidence) {
				readiness.PromotionAllowed = true
			},
			wantErr: "promotion readiness allows downstream execution",
		},
		{
			name: "readiness allows run acceptance synthesis",
			mutate: func(readiness *BasePromotionExecutionReadinessContract, _ *BasePromotionResultEvidence) {
				readiness.RunAcceptanceSynthesisAllowed = true
			},
			wantErr: "promotion readiness allows downstream execution",
		},
		{
			name: "readiness missing no package publication mutation flag",
			mutate: func(readiness *BasePromotionExecutionReadinessContract, _ *BasePromotionResultEvidence) {
				readiness.NoPackagePublicationMutation = false
			},
			wantErr: "promotion readiness must prove no package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "readiness missing no promotion mutation flag",
			mutate: func(readiness *BasePromotionExecutionReadinessContract, _ *BasePromotionResultEvidence) {
				readiness.NoPromotionMutation = false
			},
			wantErr: "promotion readiness must prove no package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "readiness missing no run acceptance mutation flag",
			mutate: func(readiness *BasePromotionExecutionReadinessContract, _ *BasePromotionResultEvidence) {
				readiness.NoRunAcceptanceMutation = false
			},
			wantErr: "promotion readiness must prove no package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "readiness missing no production mutation flag",
			mutate: func(readiness *BasePromotionExecutionReadinessContract, _ *BasePromotionResultEvidence) {
				readiness.NoProductionMutation = false
			},
			wantErr: "promotion readiness must prove no package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "readiness claims package published",
			mutate: func(readiness *BasePromotionExecutionReadinessContract, _ *BasePromotionResultEvidence) {
				readiness.PackagePublished = true
			},
			wantErr: "promotion readiness carries downstream execution, production mutation, or completion claims",
		},
		{
			name: "readiness claims promotion executed",
			mutate: func(readiness *BasePromotionExecutionReadinessContract, _ *BasePromotionResultEvidence) {
				readiness.PromotionExecuted = true
			},
			wantErr: "promotion readiness carries downstream execution, production mutation, or completion claims",
		},
		{
			name: "readiness claims run acceptance touched",
			mutate: func(readiness *BasePromotionExecutionReadinessContract, _ *BasePromotionResultEvidence) {
				readiness.RunAcceptanceRecordTouched = true
			},
			wantErr: "promotion readiness carries downstream execution, production mutation, or completion claims",
		},
		{
			name: "readiness claims production state mutated",
			mutate: func(readiness *BasePromotionExecutionReadinessContract, _ *BasePromotionResultEvidence) {
				readiness.ProductionStateMutated = true
			},
			wantErr: "promotion readiness carries downstream execution, production mutation, or completion claims",
		},
		{
			name: "readiness claims full substrate",
			mutate: func(readiness *BasePromotionExecutionReadinessContract, _ *BasePromotionResultEvidence) {
				readiness.FullSubstrateClaimed = true
			},
			wantErr: "promotion readiness carries downstream execution, production mutation, or completion claims",
		},
		{
			name: "readiness claims completion",
			mutate: func(readiness *BasePromotionExecutionReadinessContract, _ *BasePromotionResultEvidence) {
				readiness.CompletionClaimed = true
			},
			wantErr: "promotion readiness carries downstream execution, production mutation, or completion claims",
		},
		{
			name: "invalid promotion outcome",
			mutate: func(_ *BasePromotionExecutionReadinessContract, evidence *BasePromotionResultEvidence) {
				evidence.PromotionOutcome = "executed"
			},
			wantErr: "promotion outcome must be blocked or noop",
		},
		{
			name: "missing result promotion readiness ref",
			mutate: func(_ *BasePromotionExecutionReadinessContract, evidence *BasePromotionResultEvidence) {
				evidence.PromotionReadinessRef = "  "
			},
			wantErr: "promotion result refs are required",
		},
		{
			name: "missing result promotion outcome ref",
			mutate: func(_ *BasePromotionExecutionReadinessContract, evidence *BasePromotionResultEvidence) {
				evidence.PromotionOutcomeRef = "\t"
			},
			wantErr: "promotion result refs are required",
		},
		{
			name: "missing result promotion outcome reason ref",
			mutate: func(_ *BasePromotionExecutionReadinessContract, evidence *BasePromotionResultEvidence) {
				evidence.PromotionOutcomeReasonRef = ""
			},
			wantErr: "promotion result refs are required",
		},
		{
			name: "missing result operator decision ref",
			mutate: func(_ *BasePromotionExecutionReadinessContract, evidence *BasePromotionResultEvidence) {
				evidence.OperatorDecisionRef = "  "
			},
			wantErr: "promotion result refs are required",
		},
		{
			name: "missing result promotion attempt ref",
			mutate: func(_ *BasePromotionExecutionReadinessContract, evidence *BasePromotionResultEvidence) {
				evidence.PromotionAttemptRef = "\t"
			},
			wantErr: "promotion result refs are required",
		},
		{
			name: "missing result rollback plan ref",
			mutate: func(_ *BasePromotionExecutionReadinessContract, evidence *BasePromotionResultEvidence) {
				evidence.RollbackPlanRef = ""
			},
			wantErr: "promotion result refs are required",
		},
		{
			name: "evidence missing no package publication mutation flag",
			mutate: func(_ *BasePromotionExecutionReadinessContract, evidence *BasePromotionResultEvidence) {
				evidence.NoPackagePublicationMutation = false
			},
			wantErr: "evidence must prove no package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "evidence missing no promotion mutation flag",
			mutate: func(_ *BasePromotionExecutionReadinessContract, evidence *BasePromotionResultEvidence) {
				evidence.NoPromotionMutation = false
			},
			wantErr: "evidence must prove no package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "evidence missing no run acceptance mutation flag",
			mutate: func(_ *BasePromotionExecutionReadinessContract, evidence *BasePromotionResultEvidence) {
				evidence.NoRunAcceptanceMutation = false
			},
			wantErr: "evidence must prove no package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "evidence missing no production mutation flag",
			mutate: func(_ *BasePromotionExecutionReadinessContract, evidence *BasePromotionResultEvidence) {
				evidence.NoProductionMutation = false
			},
			wantErr: "evidence must prove no package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "evidence claims package published",
			mutate: func(_ *BasePromotionExecutionReadinessContract, evidence *BasePromotionResultEvidence) {
				evidence.PackagePublished = true
			},
			wantErr: "evidence carries downstream execution, production mutation, or completion claims",
		},
		{
			name: "evidence claims promotion executed",
			mutate: func(_ *BasePromotionExecutionReadinessContract, evidence *BasePromotionResultEvidence) {
				evidence.PromotionExecuted = true
			},
			wantErr: "evidence carries downstream execution, production mutation, or completion claims",
		},
		{
			name: "evidence claims run acceptance touched",
			mutate: func(_ *BasePromotionExecutionReadinessContract, evidence *BasePromotionResultEvidence) {
				evidence.RunAcceptanceRecordTouched = true
			},
			wantErr: "evidence carries downstream execution, production mutation, or completion claims",
		},
		{
			name: "evidence claims production state mutated",
			mutate: func(_ *BasePromotionExecutionReadinessContract, evidence *BasePromotionResultEvidence) {
				evidence.ProductionStateMutated = true
			},
			wantErr: "evidence carries downstream execution, production mutation, or completion claims",
		},
		{
			name: "evidence claims full substrate",
			mutate: func(_ *BasePromotionExecutionReadinessContract, evidence *BasePromotionResultEvidence) {
				evidence.FullSubstrateClaimed = true
			},
			wantErr: "evidence carries downstream execution, production mutation, or completion claims",
		},
		{
			name: "evidence claims completion",
			mutate: func(_ *BasePromotionExecutionReadinessContract, evidence *BasePromotionResultEvidence) {
				evidence.CompletionClaimed = true
			},
			wantErr: "evidence carries downstream execution, production mutation, or completion claims",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			readiness, evidence := basePromotionResultContractInputs(t, BasePromotionResultOutcomeBlocked)
			tc.mutate(&readiness, &evidence)

			contract, err := BuildBasePromotionResultContract(readiness, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildBasePromotionResultContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func basePromotionResultContractInputs(t *testing.T, outcome string) (BasePromotionExecutionReadinessContract, BasePromotionResultEvidence) {
	t.Helper()

	proof, readinessEvidence := basePromotionExecutionReadinessContractInputs(t)
	readiness, err := BuildBasePromotionExecutionReadinessContract(proof, readinessEvidence)
	if err != nil {
		t.Fatalf("BuildBasePromotionExecutionReadinessContract(): %v", err)
	}

	return readiness, BasePromotionResultEvidence{
		PromotionReadinessRef:        " promotion-readiness:base-pass-115 ",
		PromotionOutcomeRef:          " promotion-outcome:base-pass-115 ",
		PromotionOutcome:             outcome,
		PromotionOutcomeReasonRef:    " promotion-outcome-reason:base-pass-115 ",
		OperatorDecisionRef:          " operator-decision:base-pass-115 ",
		PromotionAttemptRef:          " promotion-attempt:base-pass-115 ",
		RollbackPlanRef:              " " + readiness.PromotionRollbackPlanRef + " ",
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
