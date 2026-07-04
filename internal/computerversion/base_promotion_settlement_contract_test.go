package computerversion

import (
	"strings"
	"testing"
)

func TestBuildBasePromotionSettlementContractAcceptsBlockedAndNoopResultsOnly(t *testing.T) {
	for _, tc := range []struct {
		name     string
		outcome  string
		decision string
	}{
		{name: "blocked", outcome: BasePromotionResultOutcomeBlocked, decision: BasePromotionSettlementDecisionBlocked},
		{name: "noop", outcome: BasePromotionResultOutcomeNoop, decision: BasePromotionSettlementDecisionNoop},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, evidence := basePromotionSettlementContractInputs(t, tc.outcome, tc.decision)

			contract, err := BuildBasePromotionSettlementContract(result, evidence)
			if err != nil {
				t.Fatalf("BuildBasePromotionSettlementContract(): %v", err)
			}

			if contract.Kind != BasePromotionSettlementContractKind {
				t.Fatalf("kind = %q, want %q", contract.Kind, BasePromotionSettlementContractKind)
			}
			if contract.Boundary != BasePromotionSettlementBoundary {
				t.Fatalf("boundary = %q, want %q", contract.Boundary, BasePromotionSettlementBoundary)
			}
			if contract.Scope != BasePromotionSettlementScope {
				t.Fatalf("scope = %q, want %q", contract.Scope, BasePromotionSettlementScope)
			}
			if contract.Version != result.Version || ArtifactProgramRef(contract.TypedArtifactProgramRef) != result.Version.ArtifactProgramRef {
				t.Fatalf("version/ref = %#v/%q, want promotion result version %#v and artifact program %q", contract.Version, contract.TypedArtifactProgramRef, result.Version, result.Version.ArtifactProgramRef)
			}
			if contract.PromotionResultRef != strings.TrimSpace(evidence.PromotionResultRef) || contract.PromotionReadinessRef != strings.TrimSpace(result.PromotionReadinessRef) || contract.PackagePublicationProofRef != strings.TrimSpace(result.PackagePublicationProofRef) || contract.PromotionCandidateRef != strings.TrimSpace(result.PromotionCandidateRef) || contract.PromotionExecutionPlanRef != strings.TrimSpace(result.PromotionExecutionPlanRef) || contract.PromotionPreflightRef != strings.TrimSpace(result.PromotionPreflightRef) || contract.PromotionOperatorPolicyRef != strings.TrimSpace(result.PromotionOperatorPolicyRef) || contract.PromotionRollbackPlanRef != strings.TrimSpace(evidence.RollbackPlanRef) || contract.RouteCutoverPlanRef != strings.TrimSpace(result.RouteCutoverPlanRef) || contract.LedgerFreshnessCheckRef != strings.TrimSpace(result.LedgerFreshnessCheckRef) {
				t.Fatalf("settled promotion refs = result %q readiness %q package-proof %q candidate %q execution-plan %q preflight %q policy %q rollback %q cutover %q ledger %q, want trimmed refs from result %#v and evidence %#v", contract.PromotionResultRef, contract.PromotionReadinessRef, contract.PackagePublicationProofRef, contract.PromotionCandidateRef, contract.PromotionExecutionPlanRef, contract.PromotionPreflightRef, contract.PromotionOperatorPolicyRef, contract.PromotionRollbackPlanRef, contract.RouteCutoverPlanRef, contract.LedgerFreshnessCheckRef, result, evidence)
			}
			if contract.PromotionOutcomeRef != strings.TrimSpace(result.PromotionOutcomeRef) || contract.PromotionOutcome != tc.outcome || contract.PromotionOutcomeReasonRef != strings.TrimSpace(result.PromotionOutcomeReasonRef) || contract.PromotionAttemptRef != strings.TrimSpace(result.PromotionAttemptRef) {
				t.Fatalf("settled result outcome = outcome-ref %q outcome %q reason %q attempt %q, want trimmed result refs and outcome %q from %#v", contract.PromotionOutcomeRef, contract.PromotionOutcome, contract.PromotionOutcomeReasonRef, contract.PromotionAttemptRef, tc.outcome, result)
			}
			if contract.SettlementRef != strings.TrimSpace(evidence.SettlementRef) || contract.SettlementDecision != tc.decision || contract.SettlementReasonRef != strings.TrimSpace(evidence.SettlementReasonRef) || contract.OperatorReviewRef != strings.TrimSpace(evidence.OperatorReviewRef) {
				t.Fatalf("settlement refs/decision = settlement %q decision %q reason %q operator %q, want trimmed evidence refs and decision %q from %#v", contract.SettlementRef, contract.SettlementDecision, contract.SettlementReasonRef, contract.OperatorReviewRef, tc.decision, evidence)
			}
			if !contract.PromotionResultRecorded || !contract.PromotionResultSettled || !contract.PromotionExecutionReady {
				t.Fatalf("promotion settlement must settle a recorded promotion result after promotion execution readiness: %#v", contract)
			}
			if !contract.PromotionProofRequired || !contract.RunAcceptanceProofRequired || !contract.FullSubstrateProofRequired {
				t.Fatalf("promotion settlement must preserve promotion, run-acceptance, and full-substrate proof requirements: %#v", contract)
			}
			if contract.PackagePublicationAllowed || contract.PromotionAllowed || contract.RunAcceptanceSynthesisAllowed {
				t.Fatalf("promotion settlement must not allow package publication, promotion, or run-acceptance synthesis: %#v", contract)
			}
			if !contract.NoPackagePublicationMutation || !contract.NoPromotionMutation || !contract.NoRunAcceptanceMutation || !contract.NoProductionMutation {
				t.Fatalf("promotion settlement must carry no-publication/no-promotion/no-run-acceptance/no-production mutation evidence: %#v", contract)
			}
			if contract.PackagePublished || contract.PromotionExecuted || contract.RunAcceptanceRecordTouched || contract.ProductionStateMutated || contract.FullSubstrateClaimed || contract.CompletionClaimed {
				t.Fatalf("promotion settlement must not publish, promote, touch run acceptance, mutate production, claim full substrate, or claim completion: %#v", contract)
			}
		})
	}
}

func TestBuildBasePromotionSettlementContractRejectsInvalidInputs(t *testing.T) {
	for _, tc := range []struct {
		name     string
		outcome  string
		decision string
		mutate   func(*BasePromotionResultContract, *BasePromotionSettlementEvidence)
		wantErr  string
	}{
		{
			name:     "blocked result with noop settlement decision",
			outcome:  BasePromotionResultOutcomeBlocked,
			decision: BasePromotionSettlementDecisionNoop,
			wantErr:  "blocked result requires blocked settlement decision",
		},
		{
			name:     "noop result with blocked settlement decision",
			outcome:  BasePromotionResultOutcomeNoop,
			decision: BasePromotionSettlementDecisionBlocked,
			wantErr:  "noop result requires noop settlement decision",
		},
		{
			name: "result wrong kind",
			mutate: func(result *BasePromotionResultContract, _ *BasePromotionSettlementEvidence) {
				result.Kind = BasePromotionSettlementContractKind
			},
			wantErr: "promotion result kind",
		},
		{
			name: "result boundary drift",
			mutate: func(result *BasePromotionResultContract, _ *BasePromotionSettlementEvidence) {
				result.Boundary = BasePromotionSettlementBoundary
			},
			wantErr: "promotion result boundary",
		},
		{
			name: "result scope drift",
			mutate: func(result *BasePromotionResultContract, _ *BasePromotionSettlementEvidence) {
				result.Scope = BasePromotionSettlementScope
			},
			wantErr: "promotion result scope",
		},
		{
			name: "result invalid version",
			mutate: func(result *BasePromotionResultContract, _ *BasePromotionSettlementEvidence) {
				result.Version.CodeRef = "  "
			},
			wantErr: "promotion result version is invalid",
		},
		{
			name: "result typed artifact ref drift",
			mutate: func(result *BasePromotionResultContract, _ *BasePromotionSettlementEvidence) {
				result.TypedArtifactProgramRef = "artifact-program:base-different"
			},
			wantErr: "promotion result typed artifact-program ref is invalid",
		},
		{
			name: "result missing promotion readiness ref",
			mutate: func(result *BasePromotionResultContract, _ *BasePromotionSettlementEvidence) {
				result.PromotionReadinessRef = "  "
			},
			wantErr: "promotion result refs are required",
		},
		{
			name: "result missing package-publication proof ref",
			mutate: func(result *BasePromotionResultContract, _ *BasePromotionSettlementEvidence) {
				result.PackagePublicationProofRef = "\t"
			},
			wantErr: "promotion result refs are required",
		},
		{
			name: "result missing promotion candidate ref",
			mutate: func(result *BasePromotionResultContract, _ *BasePromotionSettlementEvidence) {
				result.PromotionCandidateRef = ""
			},
			wantErr: "promotion result refs are required",
		},
		{
			name: "result missing promotion execution plan ref",
			mutate: func(result *BasePromotionResultContract, _ *BasePromotionSettlementEvidence) {
				result.PromotionExecutionPlanRef = "  "
			},
			wantErr: "promotion result refs are required",
		},
		{
			name: "result missing promotion preflight ref",
			mutate: func(result *BasePromotionResultContract, _ *BasePromotionSettlementEvidence) {
				result.PromotionPreflightRef = "\t"
			},
			wantErr: "promotion result refs are required",
		},
		{
			name: "result missing promotion operator policy ref",
			mutate: func(result *BasePromotionResultContract, _ *BasePromotionSettlementEvidence) {
				result.PromotionOperatorPolicyRef = ""
			},
			wantErr: "promotion result refs are required",
		},
		{
			name: "result missing promotion rollback plan ref",
			mutate: func(result *BasePromotionResultContract, _ *BasePromotionSettlementEvidence) {
				result.PromotionRollbackPlanRef = "  "
			},
			wantErr: "promotion result refs are required",
		},
		{
			name: "result missing route cutover plan ref",
			mutate: func(result *BasePromotionResultContract, _ *BasePromotionSettlementEvidence) {
				result.RouteCutoverPlanRef = "\t"
			},
			wantErr: "promotion result refs are required",
		},
		{
			name: "result missing ledger freshness check ref",
			mutate: func(result *BasePromotionResultContract, _ *BasePromotionSettlementEvidence) {
				result.LedgerFreshnessCheckRef = ""
			},
			wantErr: "promotion result refs are required",
		},
		{
			name: "result missing promotion outcome ref",
			mutate: func(result *BasePromotionResultContract, _ *BasePromotionSettlementEvidence) {
				result.PromotionOutcomeRef = "  "
			},
			wantErr: "promotion result refs are required",
		},
		{
			name: "result missing promotion outcome reason ref",
			mutate: func(result *BasePromotionResultContract, _ *BasePromotionSettlementEvidence) {
				result.PromotionOutcomeReasonRef = "\t"
			},
			wantErr: "promotion result refs are required",
		},
		{
			name: "result missing operator decision ref",
			mutate: func(result *BasePromotionResultContract, _ *BasePromotionSettlementEvidence) {
				result.OperatorDecisionRef = ""
			},
			wantErr: "promotion result refs are required",
		},
		{
			name: "result missing promotion attempt ref",
			mutate: func(result *BasePromotionResultContract, _ *BasePromotionSettlementEvidence) {
				result.PromotionAttemptRef = "  "
			},
			wantErr: "promotion result refs are required",
		},
		{
			name: "invalid result outcome",
			mutate: func(result *BasePromotionResultContract, _ *BasePromotionSettlementEvidence) {
				result.PromotionOutcome = "executed"
			},
			wantErr: "promotion result outcome must be blocked or noop",
		},
		{
			name: "missing result outcome",
			mutate: func(result *BasePromotionResultContract, _ *BasePromotionSettlementEvidence) {
				result.PromotionOutcome = "  "
			},
			wantErr: "promotion result outcome must be blocked or noop",
		},
		{
			name: "result not recorded",
			mutate: func(result *BasePromotionResultContract, _ *BasePromotionSettlementEvidence) {
				result.PromotionResultRecorded = false
			},
			wantErr: "promotion result must be recorded after readiness",
		},
		{
			name: "result not promotion execution ready",
			mutate: func(result *BasePromotionResultContract, _ *BasePromotionSettlementEvidence) {
				result.PromotionExecutionReady = false
			},
			wantErr: "promotion result must be recorded after readiness",
		},
		{
			name: "result missing promotion proof requirement",
			mutate: func(result *BasePromotionResultContract, _ *BasePromotionSettlementEvidence) {
				result.PromotionProofRequired = false
			},
			wantErr: "promotion result must preserve downstream proof requirements",
		},
		{
			name: "result missing run acceptance proof requirement",
			mutate: func(result *BasePromotionResultContract, _ *BasePromotionSettlementEvidence) {
				result.RunAcceptanceProofRequired = false
			},
			wantErr: "promotion result must preserve downstream proof requirements",
		},
		{
			name: "result missing full substrate proof requirement",
			mutate: func(result *BasePromotionResultContract, _ *BasePromotionSettlementEvidence) {
				result.FullSubstrateProofRequired = false
			},
			wantErr: "promotion result must preserve downstream proof requirements",
		},
		{
			name: "result allows package publication",
			mutate: func(result *BasePromotionResultContract, _ *BasePromotionSettlementEvidence) {
				result.PackagePublicationAllowed = true
			},
			wantErr: "promotion result allows downstream execution",
		},
		{
			name: "result allows promotion",
			mutate: func(result *BasePromotionResultContract, _ *BasePromotionSettlementEvidence) {
				result.PromotionAllowed = true
			},
			wantErr: "promotion result allows downstream execution",
		},
		{
			name: "result allows run acceptance synthesis",
			mutate: func(result *BasePromotionResultContract, _ *BasePromotionSettlementEvidence) {
				result.RunAcceptanceSynthesisAllowed = true
			},
			wantErr: "promotion result allows downstream execution",
		},
		{
			name: "result missing no package publication mutation flag",
			mutate: func(result *BasePromotionResultContract, _ *BasePromotionSettlementEvidence) {
				result.NoPackagePublicationMutation = false
			},
			wantErr: "promotion result must prove no package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "result missing no promotion mutation flag",
			mutate: func(result *BasePromotionResultContract, _ *BasePromotionSettlementEvidence) {
				result.NoPromotionMutation = false
			},
			wantErr: "promotion result must prove no package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "result missing no run acceptance mutation flag",
			mutate: func(result *BasePromotionResultContract, _ *BasePromotionSettlementEvidence) {
				result.NoRunAcceptanceMutation = false
			},
			wantErr: "promotion result must prove no package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "result missing no production mutation flag",
			mutate: func(result *BasePromotionResultContract, _ *BasePromotionSettlementEvidence) {
				result.NoProductionMutation = false
			},
			wantErr: "promotion result must prove no package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "result claims package published",
			mutate: func(result *BasePromotionResultContract, _ *BasePromotionSettlementEvidence) {
				result.PackagePublished = true
			},
			wantErr: "promotion result carries downstream execution, production mutation, or completion claims",
		},
		{
			name: "result claims promotion executed",
			mutate: func(result *BasePromotionResultContract, _ *BasePromotionSettlementEvidence) {
				result.PromotionExecuted = true
			},
			wantErr: "promotion result carries downstream execution, production mutation, or completion claims",
		},
		{
			name: "result claims run acceptance touched",
			mutate: func(result *BasePromotionResultContract, _ *BasePromotionSettlementEvidence) {
				result.RunAcceptanceRecordTouched = true
			},
			wantErr: "promotion result carries downstream execution, production mutation, or completion claims",
		},
		{
			name: "result claims production state mutated",
			mutate: func(result *BasePromotionResultContract, _ *BasePromotionSettlementEvidence) {
				result.ProductionStateMutated = true
			},
			wantErr: "promotion result carries downstream execution, production mutation, or completion claims",
		},
		{
			name: "result claims full substrate",
			mutate: func(result *BasePromotionResultContract, _ *BasePromotionSettlementEvidence) {
				result.FullSubstrateClaimed = true
			},
			wantErr: "promotion result carries downstream execution, production mutation, or completion claims",
		},
		{
			name: "result claims completion",
			mutate: func(result *BasePromotionResultContract, _ *BasePromotionSettlementEvidence) {
				result.CompletionClaimed = true
			},
			wantErr: "promotion result carries downstream execution, production mutation, or completion claims",
		},
		{
			name: "missing settlement promotion result ref",
			mutate: func(_ *BasePromotionResultContract, evidence *BasePromotionSettlementEvidence) {
				evidence.PromotionResultRef = "  "
			},
			wantErr: "settlement refs are required",
		},
		{
			name: "missing settlement ref",
			mutate: func(_ *BasePromotionResultContract, evidence *BasePromotionSettlementEvidence) {
				evidence.SettlementRef = "\t"
			},
			wantErr: "settlement refs are required",
		},
		{
			name: "missing settlement reason ref",
			mutate: func(_ *BasePromotionResultContract, evidence *BasePromotionSettlementEvidence) {
				evidence.SettlementReasonRef = ""
			},
			wantErr: "settlement refs are required",
		},
		{
			name: "missing settlement operator review ref",
			mutate: func(_ *BasePromotionResultContract, evidence *BasePromotionSettlementEvidence) {
				evidence.OperatorReviewRef = "  "
			},
			wantErr: "settlement refs are required",
		},
		{
			name: "missing settlement rollback plan ref",
			mutate: func(_ *BasePromotionResultContract, evidence *BasePromotionSettlementEvidence) {
				evidence.RollbackPlanRef = "\t"
			},
			wantErr: "settlement refs are required",
		},
		{
			name: "evidence missing no package publication mutation flag",
			mutate: func(_ *BasePromotionResultContract, evidence *BasePromotionSettlementEvidence) {
				evidence.NoPackagePublicationMutation = false
			},
			wantErr: "evidence must prove no package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "evidence missing no promotion mutation flag",
			mutate: func(_ *BasePromotionResultContract, evidence *BasePromotionSettlementEvidence) {
				evidence.NoPromotionMutation = false
			},
			wantErr: "evidence must prove no package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "evidence missing no run acceptance mutation flag",
			mutate: func(_ *BasePromotionResultContract, evidence *BasePromotionSettlementEvidence) {
				evidence.NoRunAcceptanceMutation = false
			},
			wantErr: "evidence must prove no package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "evidence missing no production mutation flag",
			mutate: func(_ *BasePromotionResultContract, evidence *BasePromotionSettlementEvidence) {
				evidence.NoProductionMutation = false
			},
			wantErr: "evidence must prove no package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "evidence claims package published",
			mutate: func(_ *BasePromotionResultContract, evidence *BasePromotionSettlementEvidence) {
				evidence.PackagePublished = true
			},
			wantErr: "evidence carries downstream execution, production mutation, or completion claims",
		},
		{
			name: "evidence claims promotion executed",
			mutate: func(_ *BasePromotionResultContract, evidence *BasePromotionSettlementEvidence) {
				evidence.PromotionExecuted = true
			},
			wantErr: "evidence carries downstream execution, production mutation, or completion claims",
		},
		{
			name: "evidence claims run acceptance touched",
			mutate: func(_ *BasePromotionResultContract, evidence *BasePromotionSettlementEvidence) {
				evidence.RunAcceptanceRecordTouched = true
			},
			wantErr: "evidence carries downstream execution, production mutation, or completion claims",
		},
		{
			name: "evidence claims production state mutated",
			mutate: func(_ *BasePromotionResultContract, evidence *BasePromotionSettlementEvidence) {
				evidence.ProductionStateMutated = true
			},
			wantErr: "evidence carries downstream execution, production mutation, or completion claims",
		},
		{
			name: "evidence claims full substrate",
			mutate: func(_ *BasePromotionResultContract, evidence *BasePromotionSettlementEvidence) {
				evidence.FullSubstrateClaimed = true
			},
			wantErr: "evidence carries downstream execution, production mutation, or completion claims",
		},
		{
			name: "evidence claims completion",
			mutate: func(_ *BasePromotionResultContract, evidence *BasePromotionSettlementEvidence) {
				evidence.CompletionClaimed = true
			},
			wantErr: "evidence carries downstream execution, production mutation, or completion claims",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			outcome := tc.outcome
			if outcome == "" {
				outcome = BasePromotionResultOutcomeBlocked
			}
			decision := tc.decision
			if decision == "" {
				decision = BasePromotionSettlementDecisionBlocked
				if outcome == BasePromotionResultOutcomeNoop {
					decision = BasePromotionSettlementDecisionNoop
				}
			}

			result, evidence := basePromotionSettlementContractInputs(t, outcome, decision)
			if tc.mutate != nil {
				tc.mutate(&result, &evidence)
			}

			contract, err := BuildBasePromotionSettlementContract(result, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildBasePromotionSettlementContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func basePromotionSettlementContractInputs(t *testing.T, outcome string, decision string) (BasePromotionResultContract, BasePromotionSettlementEvidence) {
	t.Helper()

	readiness, resultEvidence := basePromotionResultContractInputs(t, outcome)
	result, err := BuildBasePromotionResultContract(readiness, resultEvidence)
	if err != nil {
		t.Fatalf("BuildBasePromotionResultContract(): %v", err)
	}

	return result, BasePromotionSettlementEvidence{
		PromotionResultRef:           " promotion-result:base-pass-116 ",
		SettlementRef:                " promotion-settlement:base-pass-116 ",
		SettlementDecision:           " " + decision + " ",
		SettlementReasonRef:          " promotion-settlement-reason:base-pass-116 ",
		OperatorReviewRef:            " operator-settlement-review:base-pass-116 ",
		RollbackPlanRef:              " " + result.PromotionRollbackPlanRef + " ",
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
