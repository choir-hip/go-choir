package computerversion

import (
	"reflect"
	"strings"
	"testing"
)

func TestBuildBasePostPromotionSettlementHandoffReadinessContractAcceptsBlockedAndNoopSettlements(t *testing.T) {
	for _, tc := range []struct {
		name     string
		outcome  string
		decision string
	}{
		{name: "blocked", outcome: BasePromotionResultOutcomeBlocked, decision: BasePromotionSettlementDecisionBlocked},
		{name: "noop", outcome: BasePromotionResultOutcomeNoop, decision: BasePromotionSettlementDecisionNoop},
	} {
		t.Run(tc.name, func(t *testing.T) {
			settlement, evidence := basePostPromotionSettlementHandoffReadinessContractInputs(t, tc.outcome, tc.decision)

			contract, err := BuildBasePostPromotionSettlementHandoffReadinessContract(settlement, evidence)
			if err != nil {
				t.Fatalf("BuildBasePostPromotionSettlementHandoffReadinessContract(): %v", err)
			}

			if contract.Kind != BasePostPromotionSettlementHandoffReadinessContractKind {
				t.Fatalf("kind = %q, want %q", contract.Kind, BasePostPromotionSettlementHandoffReadinessContractKind)
			}
			if contract.Version != settlement.Version || ArtifactProgramRef(contract.TypedArtifactProgramRef) != settlement.Version.ArtifactProgramRef {
				t.Fatalf("version/ref = %#v/%q, want settlement version %#v and artifact program %q", contract.Version, contract.TypedArtifactProgramRef, settlement.Version, settlement.Version.ArtifactProgramRef)
			}
			if contract.Boundary != BasePostPromotionSettlementHandoffReadinessBoundary {
				t.Fatalf("boundary = %q, want %q", contract.Boundary, BasePostPromotionSettlementHandoffReadinessBoundary)
			}
			if contract.Scope != BasePostPromotionSettlementHandoffReadinessScope {
				t.Fatalf("scope = %q, want %q", contract.Scope, BasePostPromotionSettlementHandoffReadinessScope)
			}
			if contract.PromotionSettlementRef != strings.TrimSpace(evidence.PromotionSettlementRef) || contract.PromotionResultRef != strings.TrimSpace(settlement.PromotionResultRef) || contract.PromotionReadinessRef != strings.TrimSpace(settlement.PromotionReadinessRef) || contract.PackagePublicationProofRef != strings.TrimSpace(settlement.PackagePublicationProofRef) {
				t.Fatalf("settlement refs = settlement %q result %q readiness %q package-proof %q, want trimmed refs from settlement %#v and evidence %#v", contract.PromotionSettlementRef, contract.PromotionResultRef, contract.PromotionReadinessRef, contract.PackagePublicationProofRef, settlement, evidence)
			}
			if contract.PromotionOutcome != tc.outcome || contract.SettlementDecision != tc.decision || contract.SettlementReasonRef != strings.TrimSpace(settlement.SettlementReasonRef) || contract.OperatorReviewRef != strings.TrimSpace(settlement.OperatorReviewRef) {
				t.Fatalf("settlement outcome/decision refs = outcome %q decision %q reason %q operator %q, want outcome %q decision %q and trimmed settlement refs", contract.PromotionOutcome, contract.SettlementDecision, contract.SettlementReasonRef, contract.OperatorReviewRef, tc.outcome, tc.decision)
			}
			if contract.NextSubstrateProofPlanRef != strings.TrimSpace(evidence.NextSubstrateProofPlanRef) || contract.DurableStateSliceRef != strings.TrimSpace(evidence.DurableStateSliceRef) || contract.ObservationSetRef != strings.TrimSpace(evidence.ObservationSetRef) || contract.MaterializerContractRef != strings.TrimSpace(evidence.MaterializerContractRef) || contract.EquivalenceCheckRef != strings.TrimSpace(evidence.EquivalenceCheckRef) || contract.ResidualRiskRef != strings.TrimSpace(evidence.ResidualRiskRef) || contract.RollbackPlanRef != strings.TrimSpace(evidence.RollbackPlanRef) {
				t.Fatalf("substrate proof refs = plan %q durable %q observation %q materializer %q equivalence %q residual %q rollback %q, want trimmed evidence refs from %#v", contract.NextSubstrateProofPlanRef, contract.DurableStateSliceRef, contract.ObservationSetRef, contract.MaterializerContractRef, contract.EquivalenceCheckRef, contract.ResidualRiskRef, contract.RollbackPlanRef, evidence)
			}
			if contract.ReadinessStatus != BasePostPromotionSettlementHandoffReadinessStatusBlocked {
				t.Fatalf("readiness status = %q, want %q", contract.ReadinessStatus, BasePostPromotionSettlementHandoffReadinessStatusBlocked)
			}
			wantPrerequisites := []string{
				BasePostPromotionSettlementPrerequisiteDurableStateSlice,
				BasePostPromotionSettlementPrerequisiteObservationSet,
				BasePostPromotionSettlementPrerequisiteMaterializerContract,
				BasePostPromotionSettlementPrerequisiteEquivalenceCheck,
				BasePostPromotionSettlementPrerequisiteResidualRiskReview,
			}
			if !reflect.DeepEqual(contract.RequiredPrerequisites, wantPrerequisites) {
				t.Fatalf("required prerequisites = %#v, want %#v", contract.RequiredPrerequisites, wantPrerequisites)
			}
			if !contract.PromotionResultSettled || !contract.NextSubstrateProofRequired || !contract.DurableStateSliceRequired || !contract.ObservationSetRequired || !contract.MaterializerContractRequired || !contract.EquivalenceCheckRequired {
				t.Fatalf("handoff must preserve settlement and require every next substrate proof prerequisite: %#v", contract)
			}
			if !contract.PromotionProofRequired || !contract.RunAcceptanceProofRequired || !contract.FullSubstrateProofRequired {
				t.Fatalf("handoff must preserve promotion, run-acceptance, and full-substrate residual proof obligations: %#v", contract)
			}
			if contract.PackagePublicationAllowed || contract.PromotionAllowed || contract.RunAcceptanceSynthesisAllowed {
				t.Fatalf("handoff must not authorize package publication, promotion, or run-acceptance synthesis: %#v", contract)
			}
			if !contract.NoPackagePublicationMutation || !contract.NoPromotionMutation || !contract.NoRunAcceptanceMutation || !contract.NoProductionMutation {
				t.Fatalf("handoff must carry no-publication/no-promotion/no-run-acceptance/no-production mutation evidence: %#v", contract)
			}
			if contract.PackagePublished || contract.PromotionExecuted || contract.RunAcceptanceRecordTouched || contract.ProductionStateMutated || contract.FullSubstrateClaimed || contract.CompletionClaimed {
				t.Fatalf("handoff must not publish, promote, touch run acceptance, mutate production, claim full substrate, or claim completion: %#v", contract)
			}
		})
	}
}

func TestBuildBasePostPromotionSettlementHandoffReadinessContractRejectsInvalidInputs(t *testing.T) {
	for _, tc := range []struct {
		name     string
		outcome  string
		decision string
		mutate   func(*BasePromotionSettlementContract, *BasePostPromotionSettlementHandoffReadinessEvidence)
		wantErr  string
	}{
		{
			name: "settlement wrong kind",
			mutate: func(settlement *BasePromotionSettlementContract, _ *BasePostPromotionSettlementHandoffReadinessEvidence) {
				settlement.Kind = BasePostPromotionSettlementHandoffReadinessContractKind
			},
			wantErr: "settlement kind",
		},
		{
			name: "settlement boundary drift",
			mutate: func(settlement *BasePromotionSettlementContract, _ *BasePostPromotionSettlementHandoffReadinessEvidence) {
				settlement.Boundary = BasePostPromotionSettlementHandoffReadinessBoundary
			},
			wantErr: "settlement boundary",
		},
		{
			name: "settlement scope drift",
			mutate: func(settlement *BasePromotionSettlementContract, _ *BasePostPromotionSettlementHandoffReadinessEvidence) {
				settlement.Scope = BasePostPromotionSettlementHandoffReadinessScope
			},
			wantErr: "settlement scope",
		},
		{
			name: "settlement invalid version",
			mutate: func(settlement *BasePromotionSettlementContract, _ *BasePostPromotionSettlementHandoffReadinessEvidence) {
				settlement.Version.CodeRef = "  "
			},
			wantErr: "settlement version is invalid",
		},
		{
			name: "settlement artifact ref drift",
			mutate: func(settlement *BasePromotionSettlementContract, _ *BasePostPromotionSettlementHandoffReadinessEvidence) {
				settlement.TypedArtifactProgramRef = "artifact-program:base-different"
			},
			wantErr: "settlement typed artifact-program ref is invalid",
		},
		{
			name: "missing settlement promotion result ref",
			mutate: func(settlement *BasePromotionSettlementContract, _ *BasePostPromotionSettlementHandoffReadinessEvidence) {
				settlement.PromotionResultRef = "  "
			},
			wantErr: "settlement refs are required",
		},
		{
			name: "missing settlement promotion readiness ref",
			mutate: func(settlement *BasePromotionSettlementContract, _ *BasePostPromotionSettlementHandoffReadinessEvidence) {
				settlement.PromotionReadinessRef = "\t"
			},
			wantErr: "settlement refs are required",
		},
		{
			name: "missing settlement package publication proof ref",
			mutate: func(settlement *BasePromotionSettlementContract, _ *BasePostPromotionSettlementHandoffReadinessEvidence) {
				settlement.PackagePublicationProofRef = ""
			},
			wantErr: "settlement refs are required",
		},
		{
			name: "missing settlement outcome",
			mutate: func(settlement *BasePromotionSettlementContract, _ *BasePostPromotionSettlementHandoffReadinessEvidence) {
				settlement.PromotionOutcome = "  "
			},
			wantErr: "settlement refs are required",
		},
		{
			name: "missing settlement ref",
			mutate: func(settlement *BasePromotionSettlementContract, _ *BasePostPromotionSettlementHandoffReadinessEvidence) {
				settlement.SettlementRef = "\t"
			},
			wantErr: "settlement refs are required",
		},
		{
			name: "missing settlement reason ref",
			mutate: func(settlement *BasePromotionSettlementContract, _ *BasePostPromotionSettlementHandoffReadinessEvidence) {
				settlement.SettlementReasonRef = ""
			},
			wantErr: "settlement refs are required",
		},
		{
			name: "missing settlement operator review ref",
			mutate: func(settlement *BasePromotionSettlementContract, _ *BasePostPromotionSettlementHandoffReadinessEvidence) {
				settlement.OperatorReviewRef = "  "
			},
			wantErr: "settlement refs are required",
		},
		{
			name: "invalid settlement outcome",
			mutate: func(settlement *BasePromotionSettlementContract, _ *BasePostPromotionSettlementHandoffReadinessEvidence) {
				settlement.PromotionOutcome = "executed"
			},
			wantErr: "settlement outcome must be blocked or noop",
		},
		{
			name: "blocked settlement with noop decision",
			mutate: func(settlement *BasePromotionSettlementContract, _ *BasePostPromotionSettlementHandoffReadinessEvidence) {
				settlement.SettlementDecision = BasePromotionSettlementDecisionNoop
			},
			wantErr: "blocked outcome requires blocked settlement",
		},
		{
			name:    "noop settlement with blocked decision",
			outcome: BasePromotionResultOutcomeNoop,
			mutate: func(settlement *BasePromotionSettlementContract, _ *BasePostPromotionSettlementHandoffReadinessEvidence) {
				settlement.SettlementDecision = BasePromotionSettlementDecisionBlocked
			},
			wantErr: "noop outcome requires noop settlement",
		},
		{
			name: "settlement not recorded",
			mutate: func(settlement *BasePromotionSettlementContract, _ *BasePostPromotionSettlementHandoffReadinessEvidence) {
				settlement.PromotionResultRecorded = false
			},
			wantErr: "settlement must settle a recorded promotion result",
		},
		{
			name: "settlement unsettled",
			mutate: func(settlement *BasePromotionSettlementContract, _ *BasePostPromotionSettlementHandoffReadinessEvidence) {
				settlement.PromotionResultSettled = false
			},
			wantErr: "settlement must settle a recorded promotion result",
		},
		{
			name: "settlement not execution ready",
			mutate: func(settlement *BasePromotionSettlementContract, _ *BasePostPromotionSettlementHandoffReadinessEvidence) {
				settlement.PromotionExecutionReady = false
			},
			wantErr: "settlement must settle a recorded promotion result",
		},
		{
			name: "missing settlement promotion proof requirement",
			mutate: func(settlement *BasePromotionSettlementContract, _ *BasePostPromotionSettlementHandoffReadinessEvidence) {
				settlement.PromotionProofRequired = false
			},
			wantErr: "settlement must preserve downstream proof requirements",
		},
		{
			name: "missing settlement run acceptance proof requirement",
			mutate: func(settlement *BasePromotionSettlementContract, _ *BasePostPromotionSettlementHandoffReadinessEvidence) {
				settlement.RunAcceptanceProofRequired = false
			},
			wantErr: "settlement must preserve downstream proof requirements",
		},
		{
			name: "missing settlement full substrate proof requirement",
			mutate: func(settlement *BasePromotionSettlementContract, _ *BasePostPromotionSettlementHandoffReadinessEvidence) {
				settlement.FullSubstrateProofRequired = false
			},
			wantErr: "settlement must preserve downstream proof requirements",
		},
		{
			name: "settlement allows package publication",
			mutate: func(settlement *BasePromotionSettlementContract, _ *BasePostPromotionSettlementHandoffReadinessEvidence) {
				settlement.PackagePublicationAllowed = true
			},
			wantErr: "settlement allows downstream execution",
		},
		{
			name: "settlement allows promotion",
			mutate: func(settlement *BasePromotionSettlementContract, _ *BasePostPromotionSettlementHandoffReadinessEvidence) {
				settlement.PromotionAllowed = true
			},
			wantErr: "settlement allows downstream execution",
		},
		{
			name: "settlement allows run acceptance synthesis",
			mutate: func(settlement *BasePromotionSettlementContract, _ *BasePostPromotionSettlementHandoffReadinessEvidence) {
				settlement.RunAcceptanceSynthesisAllowed = true
			},
			wantErr: "settlement allows downstream execution",
		},
		{
			name: "settlement missing no package publication mutation flag",
			mutate: func(settlement *BasePromotionSettlementContract, _ *BasePostPromotionSettlementHandoffReadinessEvidence) {
				settlement.NoPackagePublicationMutation = false
			},
			wantErr: "settlement must prove no package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "settlement missing no promotion mutation flag",
			mutate: func(settlement *BasePromotionSettlementContract, _ *BasePostPromotionSettlementHandoffReadinessEvidence) {
				settlement.NoPromotionMutation = false
			},
			wantErr: "settlement must prove no package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "settlement missing no run acceptance mutation flag",
			mutate: func(settlement *BasePromotionSettlementContract, _ *BasePostPromotionSettlementHandoffReadinessEvidence) {
				settlement.NoRunAcceptanceMutation = false
			},
			wantErr: "settlement must prove no package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "settlement missing no production mutation flag",
			mutate: func(settlement *BasePromotionSettlementContract, _ *BasePostPromotionSettlementHandoffReadinessEvidence) {
				settlement.NoProductionMutation = false
			},
			wantErr: "settlement must prove no package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "settlement package published",
			mutate: func(settlement *BasePromotionSettlementContract, _ *BasePostPromotionSettlementHandoffReadinessEvidence) {
				settlement.PackagePublished = true
			},
			wantErr: "settlement carries downstream execution, production mutation, or completion claims",
		},
		{
			name: "settlement promotion executed",
			mutate: func(settlement *BasePromotionSettlementContract, _ *BasePostPromotionSettlementHandoffReadinessEvidence) {
				settlement.PromotionExecuted = true
			},
			wantErr: "settlement carries downstream execution, production mutation, or completion claims",
		},
		{
			name: "settlement run acceptance touched",
			mutate: func(settlement *BasePromotionSettlementContract, _ *BasePostPromotionSettlementHandoffReadinessEvidence) {
				settlement.RunAcceptanceRecordTouched = true
			},
			wantErr: "settlement carries downstream execution, production mutation, or completion claims",
		},
		{
			name: "settlement production mutated",
			mutate: func(settlement *BasePromotionSettlementContract, _ *BasePostPromotionSettlementHandoffReadinessEvidence) {
				settlement.ProductionStateMutated = true
			},
			wantErr: "settlement carries downstream execution, production mutation, or completion claims",
		},
		{
			name: "settlement full substrate claimed",
			mutate: func(settlement *BasePromotionSettlementContract, _ *BasePostPromotionSettlementHandoffReadinessEvidence) {
				settlement.FullSubstrateClaimed = true
			},
			wantErr: "settlement carries downstream execution, production mutation, or completion claims",
		},
		{
			name: "settlement completion claimed",
			mutate: func(settlement *BasePromotionSettlementContract, _ *BasePostPromotionSettlementHandoffReadinessEvidence) {
				settlement.CompletionClaimed = true
			},
			wantErr: "settlement carries downstream execution, production mutation, or completion claims",
		},
		{
			name: "missing handoff promotion settlement ref",
			mutate: func(_ *BasePromotionSettlementContract, evidence *BasePostPromotionSettlementHandoffReadinessEvidence) {
				evidence.PromotionSettlementRef = "  "
			},
			wantErr: "substrate proof refs are required",
		},
		{
			name: "missing next substrate proof plan ref",
			mutate: func(_ *BasePromotionSettlementContract, evidence *BasePostPromotionSettlementHandoffReadinessEvidence) {
				evidence.NextSubstrateProofPlanRef = "\t"
			},
			wantErr: "substrate proof refs are required",
		},
		{
			name: "missing durable state slice ref",
			mutate: func(_ *BasePromotionSettlementContract, evidence *BasePostPromotionSettlementHandoffReadinessEvidence) {
				evidence.DurableStateSliceRef = ""
			},
			wantErr: "substrate proof refs are required",
		},
		{
			name: "missing observation set ref",
			mutate: func(_ *BasePromotionSettlementContract, evidence *BasePostPromotionSettlementHandoffReadinessEvidence) {
				evidence.ObservationSetRef = "  "
			},
			wantErr: "substrate proof refs are required",
		},
		{
			name: "missing materializer contract ref",
			mutate: func(_ *BasePromotionSettlementContract, evidence *BasePostPromotionSettlementHandoffReadinessEvidence) {
				evidence.MaterializerContractRef = "\t"
			},
			wantErr: "substrate proof refs are required",
		},
		{
			name: "missing equivalence check ref",
			mutate: func(_ *BasePromotionSettlementContract, evidence *BasePostPromotionSettlementHandoffReadinessEvidence) {
				evidence.EquivalenceCheckRef = ""
			},
			wantErr: "substrate proof refs are required",
		},
		{
			name: "missing residual risk ref",
			mutate: func(_ *BasePromotionSettlementContract, evidence *BasePostPromotionSettlementHandoffReadinessEvidence) {
				evidence.ResidualRiskRef = "  "
			},
			wantErr: "substrate proof refs are required",
		},
		{
			name: "missing rollback plan ref",
			mutate: func(_ *BasePromotionSettlementContract, evidence *BasePostPromotionSettlementHandoffReadinessEvidence) {
				evidence.RollbackPlanRef = "\t"
			},
			wantErr: "substrate proof refs are required",
		},
		{
			name: "evidence missing no package publication mutation flag",
			mutate: func(_ *BasePromotionSettlementContract, evidence *BasePostPromotionSettlementHandoffReadinessEvidence) {
				evidence.NoPackagePublicationMutation = false
			},
			wantErr: "evidence must prove no package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "evidence missing no promotion mutation flag",
			mutate: func(_ *BasePromotionSettlementContract, evidence *BasePostPromotionSettlementHandoffReadinessEvidence) {
				evidence.NoPromotionMutation = false
			},
			wantErr: "evidence must prove no package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "evidence missing no run acceptance mutation flag",
			mutate: func(_ *BasePromotionSettlementContract, evidence *BasePostPromotionSettlementHandoffReadinessEvidence) {
				evidence.NoRunAcceptanceMutation = false
			},
			wantErr: "evidence must prove no package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "evidence missing no production mutation flag",
			mutate: func(_ *BasePromotionSettlementContract, evidence *BasePostPromotionSettlementHandoffReadinessEvidence) {
				evidence.NoProductionMutation = false
			},
			wantErr: "evidence must prove no package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "evidence package published",
			mutate: func(_ *BasePromotionSettlementContract, evidence *BasePostPromotionSettlementHandoffReadinessEvidence) {
				evidence.PackagePublished = true
			},
			wantErr: "evidence carries downstream execution, production mutation, or completion claims",
		},
		{
			name: "evidence promotion executed",
			mutate: func(_ *BasePromotionSettlementContract, evidence *BasePostPromotionSettlementHandoffReadinessEvidence) {
				evidence.PromotionExecuted = true
			},
			wantErr: "evidence carries downstream execution, production mutation, or completion claims",
		},
		{
			name: "evidence run acceptance touched",
			mutate: func(_ *BasePromotionSettlementContract, evidence *BasePostPromotionSettlementHandoffReadinessEvidence) {
				evidence.RunAcceptanceRecordTouched = true
			},
			wantErr: "evidence carries downstream execution, production mutation, or completion claims",
		},
		{
			name: "evidence production mutated",
			mutate: func(_ *BasePromotionSettlementContract, evidence *BasePostPromotionSettlementHandoffReadinessEvidence) {
				evidence.ProductionStateMutated = true
			},
			wantErr: "evidence carries downstream execution, production mutation, or completion claims",
		},
		{
			name: "evidence full substrate claimed",
			mutate: func(_ *BasePromotionSettlementContract, evidence *BasePostPromotionSettlementHandoffReadinessEvidence) {
				evidence.FullSubstrateClaimed = true
			},
			wantErr: "evidence carries downstream execution, production mutation, or completion claims",
		},
		{
			name: "evidence completion claimed",
			mutate: func(_ *BasePromotionSettlementContract, evidence *BasePostPromotionSettlementHandoffReadinessEvidence) {
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

			settlement, evidence := basePostPromotionSettlementHandoffReadinessContractInputs(t, outcome, decision)
			if tc.mutate != nil {
				tc.mutate(&settlement, &evidence)
			}

			contract, err := BuildBasePostPromotionSettlementHandoffReadinessContract(settlement, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildBasePostPromotionSettlementHandoffReadinessContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func basePostPromotionSettlementHandoffReadinessContractInputs(t *testing.T, outcome string, decision string) (BasePromotionSettlementContract, BasePostPromotionSettlementHandoffReadinessEvidence) {
	t.Helper()

	result, settlementEvidence := basePromotionSettlementContractInputs(t, outcome, decision)
	settlement, err := BuildBasePromotionSettlementContract(result, settlementEvidence)
	if err != nil {
		t.Fatalf("BuildBasePromotionSettlementContract(): %v", err)
	}

	return settlement, BasePostPromotionSettlementHandoffReadinessEvidence{
		PromotionSettlementRef:       " promotion-settlement:base-pass-116 ",
		NextSubstrateProofPlanRef:    " substrate-proof-plan:base-pass-117 ",
		DurableStateSliceRef:         " durable-state-slice:base-pass-117 ",
		ObservationSetRef:            " observation-set:base-pass-117 ",
		MaterializerContractRef:      " materializer-contract:base-pass-117 ",
		EquivalenceCheckRef:          " equivalence-check:base-pass-117 ",
		ResidualRiskRef:              " residual-risk-review:base-pass-117 ",
		RollbackPlanRef:              " " + settlement.PromotionRollbackPlanRef + " ",
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
