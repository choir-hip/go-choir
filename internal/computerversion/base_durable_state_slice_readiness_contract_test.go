package computerversion

import (
	"reflect"
	"strings"
	"testing"
)

func TestBuildBaseDurableStateSliceReadinessContractAcceptsBlockedAndNoopHandoffs(t *testing.T) {
	for _, tc := range []struct {
		name     string
		outcome  string
		decision string
	}{
		{name: "blocked", outcome: BasePromotionResultOutcomeBlocked, decision: BasePromotionSettlementDecisionBlocked},
		{name: "noop", outcome: BasePromotionResultOutcomeNoop, decision: BasePromotionSettlementDecisionNoop},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handoff, evidence := baseDurableStateSliceReadinessContractInputs(t, tc.outcome, tc.decision)

			contract, err := BuildBaseDurableStateSliceReadinessContract(handoff, evidence)
			if err != nil {
				t.Fatalf("BuildBaseDurableStateSliceReadinessContract(): %v", err)
			}

			if contract.Kind != BaseDurableStateSliceReadinessContractKind {
				t.Fatalf("kind = %q, want %q", contract.Kind, BaseDurableStateSliceReadinessContractKind)
			}
			if contract.Boundary != BaseDurableStateSliceReadinessBoundary {
				t.Fatalf("boundary = %q, want %q", contract.Boundary, BaseDurableStateSliceReadinessBoundary)
			}
			if contract.Scope != BaseDurableStateSliceReadinessScope {
				t.Fatalf("scope = %q, want %q", contract.Scope, BaseDurableStateSliceReadinessScope)
			}
			if contract.Version != handoff.Version || ArtifactProgramRef(contract.TypedArtifactProgramRef) != handoff.Version.ArtifactProgramRef {
				t.Fatalf("version/ref = %#v/%q, want handoff version %#v and artifact program %q", contract.Version, contract.TypedArtifactProgramRef, handoff.Version, handoff.Version.ArtifactProgramRef)
			}
			if contract.PostPromotionSettlementHandoffRef != strings.TrimSpace(evidence.PostPromotionSettlementHandoffRef) || contract.PromotionSettlementRef != strings.TrimSpace(handoff.PromotionSettlementRef) || contract.PromotionResultRef != strings.TrimSpace(handoff.PromotionResultRef) || contract.PromotionReadinessRef != strings.TrimSpace(handoff.PromotionReadinessRef) || contract.PackagePublicationProofRef != strings.TrimSpace(handoff.PackagePublicationProofRef) {
				t.Fatalf("handoff refs = handoff %q settlement %q result %q readiness %q package-proof %q, want trimmed refs from handoff %#v and evidence %#v", contract.PostPromotionSettlementHandoffRef, contract.PromotionSettlementRef, contract.PromotionResultRef, contract.PromotionReadinessRef, contract.PackagePublicationProofRef, handoff, evidence)
			}
			if contract.PromotionOutcome != tc.outcome || contract.SettlementDecision != tc.decision || contract.SettlementReasonRef != strings.TrimSpace(handoff.SettlementReasonRef) || contract.OperatorReviewRef != strings.TrimSpace(handoff.OperatorReviewRef) || contract.NextSubstrateProofPlanRef != strings.TrimSpace(handoff.NextSubstrateProofPlanRef) {
				t.Fatalf("settlement handoff values = outcome %q decision %q reason %q operator %q next-plan %q, want outcome %q decision %q and trimmed handoff refs", contract.PromotionOutcome, contract.SettlementDecision, contract.SettlementReasonRef, contract.OperatorReviewRef, contract.NextSubstrateProofPlanRef, tc.outcome, tc.decision)
			}
			if contract.DurableStateSlicePlanRef != strings.TrimSpace(evidence.DurableStateSlicePlanRef) || contract.FileManifestProbeRef != strings.TrimSpace(evidence.FileManifestProbeRef) || contract.BlobContentProbeRef != strings.TrimSpace(evidence.BlobContentProbeRef) || contract.ObservationSetRef != strings.TrimSpace(evidence.ObservationSetRef) || contract.MaterializerContractRef != strings.TrimSpace(evidence.MaterializerContractRef) || contract.EquivalenceCheckRef != strings.TrimSpace(evidence.EquivalenceCheckRef) || contract.ResidualRiskRef != strings.TrimSpace(evidence.ResidualRiskRef) || contract.RollbackPlanRef != strings.TrimSpace(evidence.RollbackPlanRef) {
				t.Fatalf("durable-state-slice refs = plan %q file %q blob %q observation %q materializer %q equivalence %q residual %q rollback %q, want trimmed evidence refs from %#v", contract.DurableStateSlicePlanRef, contract.FileManifestProbeRef, contract.BlobContentProbeRef, contract.ObservationSetRef, contract.MaterializerContractRef, contract.EquivalenceCheckRef, contract.ResidualRiskRef, contract.RollbackPlanRef, evidence)
			}
			if contract.ReadinessStatus != BaseDurableStateSliceReadinessStatusReady {
				t.Fatalf("readiness status = %q, want %q", contract.ReadinessStatus, BaseDurableStateSliceReadinessStatusReady)
			}
			wantPrerequisites := []string{
				BaseDurableStateSliceReadinessPrerequisiteFileManifestProbe,
				BaseDurableStateSliceReadinessPrerequisiteBlobContentProbe,
				BaseDurableStateSliceReadinessPrerequisiteObservationSet,
				BaseDurableStateSliceReadinessPrerequisiteMaterializerContract,
				BaseDurableStateSliceReadinessPrerequisiteEquivalenceCheck,
				BaseDurableStateSliceReadinessPrerequisiteResidualRiskReview,
			}
			if !reflect.DeepEqual(contract.RequiredPrerequisites, wantPrerequisites) {
				t.Fatalf("required prerequisites = %#v, want %#v", contract.RequiredPrerequisites, wantPrerequisites)
			}
			if !contract.PostSettlementHandoffRecorded || !contract.DurableStateSliceProbeRequired || !contract.FileManifestProbeRequired || !contract.BlobContentProbeRequired || !contract.ObservationSetRequired || !contract.MaterializerContractRequired || !contract.EquivalenceCheckRequired {
				t.Fatalf("durable-state-slice readiness must record handoff and require typed slice prerequisites: %#v", contract)
			}
			if !contract.PromotionProofRequired || !contract.RunAcceptanceProofRequired || !contract.FullSubstrateProofRequired {
				t.Fatalf("durable-state-slice readiness must preserve downstream proof obligations: %#v", contract)
			}
			if contract.RuntimeMaterializationAllowed || contract.DurableComputerMutationAllowed || contract.PackagePublicationAllowed || contract.PromotionAllowed || contract.RunAcceptanceSynthesisAllowed {
				t.Fatalf("durable-state-slice readiness must deny runtime materialization, durable mutation, publication, promotion, and run-acceptance authority: %#v", contract)
			}
			if !contract.NoRuntimeMaterialization || !contract.NoDurableComputerMutation || !contract.NoPackagePublicationMutation || !contract.NoPromotionMutation || !contract.NoRunAcceptanceMutation || !contract.NoProductionMutation {
				t.Fatalf("durable-state-slice readiness must carry no-runtime/no-durable/no-publication/no-promotion/no-run-acceptance/no-production evidence: %#v", contract)
			}
			if contract.RuntimeMaterialized || contract.DurableComputerStateMutated || contract.PackagePublished || contract.PromotionExecuted || contract.RunAcceptanceRecordTouched || contract.ProductionStateMutated || contract.FullSubstrateClaimed || contract.CompletionClaimed {
				t.Fatalf("durable-state-slice readiness must not materialize runtime, mutate durable state, publish, promote, touch run acceptance, mutate production, claim full substrate, or claim completion: %#v", contract)
			}
		})
	}
}

func TestBuildBaseDurableStateSliceReadinessContractRejectsInvalidInputs(t *testing.T) {
	for _, tc := range []struct {
		name     string
		outcome  string
		decision string
		mutate   func(*BasePostPromotionSettlementHandoffReadinessContract, *BaseDurableStateSliceReadinessEvidence)
		wantErr  string
	}{
		{
			name: "handoff wrong kind",
			mutate: func(handoff *BasePostPromotionSettlementHandoffReadinessContract, _ *BaseDurableStateSliceReadinessEvidence) {
				handoff.Kind = BasePromotionSettlementContractKind
			},
			wantErr: "handoff kind",
		},
		{
			name: "handoff boundary drift",
			mutate: func(handoff *BasePostPromotionSettlementHandoffReadinessContract, _ *BaseDurableStateSliceReadinessEvidence) {
				handoff.Boundary = BasePromotionSettlementBoundary
			},
			wantErr: "handoff boundary",
		},
		{
			name: "handoff scope drift",
			mutate: func(handoff *BasePostPromotionSettlementHandoffReadinessContract, _ *BaseDurableStateSliceReadinessEvidence) {
				handoff.Scope = BasePromotionSettlementScope
			},
			wantErr: "handoff scope",
		},
		{
			name: "handoff invalid version",
			mutate: func(handoff *BasePostPromotionSettlementHandoffReadinessContract, _ *BaseDurableStateSliceReadinessEvidence) {
				handoff.Version.CodeRef = "  "
			},
			wantErr: "handoff version is invalid",
		},
		{
			name: "handoff artifact ref drift",
			mutate: func(handoff *BasePostPromotionSettlementHandoffReadinessContract, _ *BaseDurableStateSliceReadinessEvidence) {
				handoff.TypedArtifactProgramRef = "artifact-program:base-different"
			},
			wantErr: "handoff typed artifact-program ref is invalid",
		},
		{
			name: "handoff missing promotion settlement ref",
			mutate: func(handoff *BasePostPromotionSettlementHandoffReadinessContract, _ *BaseDurableStateSliceReadinessEvidence) {
				handoff.PromotionSettlementRef = "  "
			},
			wantErr: "handoff refs are required",
		},
		{
			name: "handoff status not blocked",
			mutate: func(handoff *BasePostPromotionSettlementHandoffReadinessContract, _ *BaseDurableStateSliceReadinessEvidence) {
				handoff.ReadinessStatus = "ready"
			},
			wantErr: "handoff status",
		},
		{
			name: "handoff durable slice prerequisite drift",
			mutate: func(handoff *BasePostPromotionSettlementHandoffReadinessContract, _ *BaseDurableStateSliceReadinessEvidence) {
				handoff.DurableStateSliceRequired = false
			},
			wantErr: "handoff must preserve substrate proof prerequisites",
		},
		{
			name: "handoff downstream proof prerequisite drift",
			mutate: func(handoff *BasePostPromotionSettlementHandoffReadinessContract, _ *BaseDurableStateSliceReadinessEvidence) {
				handoff.RunAcceptanceProofRequired = false
			},
			wantErr: "handoff must preserve downstream proof requirements",
		},
		{
			name: "handoff allows package publication",
			mutate: func(handoff *BasePostPromotionSettlementHandoffReadinessContract, _ *BaseDurableStateSliceReadinessEvidence) {
				handoff.PackagePublicationAllowed = true
			},
			wantErr: "handoff allows downstream execution",
		},
		{
			name: "handoff missing no production mutation flag",
			mutate: func(handoff *BasePostPromotionSettlementHandoffReadinessContract, _ *BaseDurableStateSliceReadinessEvidence) {
				handoff.NoProductionMutation = false
			},
			wantErr: "handoff must prove no package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "handoff claims completion",
			mutate: func(handoff *BasePostPromotionSettlementHandoffReadinessContract, _ *BaseDurableStateSliceReadinessEvidence) {
				handoff.CompletionClaimed = true
			},
			wantErr: "handoff carries downstream execution, production mutation, or completion claims",
		},
		{
			name: "evidence missing handoff ref",
			mutate: func(_ *BasePostPromotionSettlementHandoffReadinessContract, evidence *BaseDurableStateSliceReadinessEvidence) {
				evidence.PostPromotionSettlementHandoffRef = "  "
			},
			wantErr: "evidence refs are required",
		},
		{
			name: "evidence missing durable state slice plan ref",
			mutate: func(_ *BasePostPromotionSettlementHandoffReadinessContract, evidence *BaseDurableStateSliceReadinessEvidence) {
				evidence.DurableStateSlicePlanRef = "\t"
			},
			wantErr: "evidence refs are required",
		},
		{
			name: "evidence durable slice ref mismatches handoff",
			mutate: func(_ *BasePostPromotionSettlementHandoffReadinessContract, evidence *BaseDurableStateSliceReadinessEvidence) {
				evidence.DurableStateSlicePlanRef = "durable-state-slice:foreign"
			},
			wantErr: "evidence refs do not match handoff",
		},
		{
			name: "evidence observation ref mismatches handoff",
			mutate: func(_ *BasePostPromotionSettlementHandoffReadinessContract, evidence *BaseDurableStateSliceReadinessEvidence) {
				evidence.ObservationSetRef = "observation-set:foreign"
			},
			wantErr: "evidence refs do not match handoff",
		},
		{
			name: "evidence missing no runtime materialization flag",
			mutate: func(_ *BasePostPromotionSettlementHandoffReadinessContract, evidence *BaseDurableStateSliceReadinessEvidence) {
				evidence.NoRuntimeMaterialization = false
			},
			wantErr: "evidence must prove no runtime, durable-computer, package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "evidence missing no durable computer mutation flag",
			mutate: func(_ *BasePostPromotionSettlementHandoffReadinessContract, evidence *BaseDurableStateSliceReadinessEvidence) {
				evidence.NoDurableComputerMutation = false
			},
			wantErr: "evidence must prove no runtime, durable-computer, package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "evidence missing no package publication mutation flag",
			mutate: func(_ *BasePostPromotionSettlementHandoffReadinessContract, evidence *BaseDurableStateSliceReadinessEvidence) {
				evidence.NoPackagePublicationMutation = false
			},
			wantErr: "evidence must prove no runtime, durable-computer, package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "evidence missing no promotion mutation flag",
			mutate: func(_ *BasePostPromotionSettlementHandoffReadinessContract, evidence *BaseDurableStateSliceReadinessEvidence) {
				evidence.NoPromotionMutation = false
			},
			wantErr: "evidence must prove no runtime, durable-computer, package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "evidence missing no run acceptance mutation flag",
			mutate: func(_ *BasePostPromotionSettlementHandoffReadinessContract, evidence *BaseDurableStateSliceReadinessEvidence) {
				evidence.NoRunAcceptanceMutation = false
			},
			wantErr: "evidence must prove no runtime, durable-computer, package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "evidence missing no production mutation flag",
			mutate: func(_ *BasePostPromotionSettlementHandoffReadinessContract, evidence *BaseDurableStateSliceReadinessEvidence) {
				evidence.NoProductionMutation = false
			},
			wantErr: "evidence must prove no runtime, durable-computer, package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "evidence claims runtime materialized",
			mutate: func(_ *BasePostPromotionSettlementHandoffReadinessContract, evidence *BaseDurableStateSliceReadinessEvidence) {
				evidence.RuntimeMaterialized = true
			},
			wantErr: "evidence carries materialization, mutation, downstream execution, production, full-substrate, or completion claims",
		},
		{
			name: "evidence claims durable computer mutation",
			mutate: func(_ *BasePostPromotionSettlementHandoffReadinessContract, evidence *BaseDurableStateSliceReadinessEvidence) {
				evidence.DurableComputerStateMutated = true
			},
			wantErr: "evidence carries materialization, mutation, downstream execution, production, full-substrate, or completion claims",
		},
		{
			name: "evidence claims package published",
			mutate: func(_ *BasePostPromotionSettlementHandoffReadinessContract, evidence *BaseDurableStateSliceReadinessEvidence) {
				evidence.PackagePublished = true
			},
			wantErr: "evidence carries materialization, mutation, downstream execution, production, full-substrate, or completion claims",
		},
		{
			name: "evidence claims promotion executed",
			mutate: func(_ *BasePostPromotionSettlementHandoffReadinessContract, evidence *BaseDurableStateSliceReadinessEvidence) {
				evidence.PromotionExecuted = true
			},
			wantErr: "evidence carries materialization, mutation, downstream execution, production, full-substrate, or completion claims",
		},
		{
			name: "evidence claims run acceptance touched",
			mutate: func(_ *BasePostPromotionSettlementHandoffReadinessContract, evidence *BaseDurableStateSliceReadinessEvidence) {
				evidence.RunAcceptanceRecordTouched = true
			},
			wantErr: "evidence carries materialization, mutation, downstream execution, production, full-substrate, or completion claims",
		},
		{
			name: "evidence claims production state mutated",
			mutate: func(_ *BasePostPromotionSettlementHandoffReadinessContract, evidence *BaseDurableStateSliceReadinessEvidence) {
				evidence.ProductionStateMutated = true
			},
			wantErr: "evidence carries materialization, mutation, downstream execution, production, full-substrate, or completion claims",
		},
		{
			name: "evidence claims full substrate",
			mutate: func(_ *BasePostPromotionSettlementHandoffReadinessContract, evidence *BaseDurableStateSliceReadinessEvidence) {
				evidence.FullSubstrateClaimed = true
			},
			wantErr: "evidence carries materialization, mutation, downstream execution, production, full-substrate, or completion claims",
		},
		{
			name: "evidence claims completion",
			mutate: func(_ *BasePostPromotionSettlementHandoffReadinessContract, evidence *BaseDurableStateSliceReadinessEvidence) {
				evidence.CompletionClaimed = true
			},
			wantErr: "evidence carries materialization, mutation, downstream execution, production, full-substrate, or completion claims",
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

			handoff, evidence := baseDurableStateSliceReadinessContractInputs(t, outcome, decision)
			if tc.mutate != nil {
				tc.mutate(&handoff, &evidence)
			}

			contract, err := BuildBaseDurableStateSliceReadinessContract(handoff, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildBaseDurableStateSliceReadinessContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func baseDurableStateSliceReadinessContractInputs(t *testing.T, outcome string, decision string) (BasePostPromotionSettlementHandoffReadinessContract, BaseDurableStateSliceReadinessEvidence) {
	t.Helper()

	settlement, handoffEvidence := basePostPromotionSettlementHandoffReadinessContractInputs(t, outcome, decision)
	handoff, err := BuildBasePostPromotionSettlementHandoffReadinessContract(settlement, handoffEvidence)
	if err != nil {
		t.Fatalf("BuildBasePostPromotionSettlementHandoffReadinessContract(): %v", err)
	}

	return handoff, baseDurableStateSliceReadinessEvidence(handoff)
}

func baseDurableStateSliceReadinessEvidence(handoff BasePostPromotionSettlementHandoffReadinessContract) BaseDurableStateSliceReadinessEvidence {
	return BaseDurableStateSliceReadinessEvidence{
		PostPromotionSettlementHandoffRef: " post-promotion-settlement-handoff:base-pass-117 ",
		DurableStateSlicePlanRef:          " " + handoff.DurableStateSliceRef + " ",
		FileManifestProbeRef:              " file-manifest-probe:base-pass-118 ",
		BlobContentProbeRef:               " blob-content-probe:base-pass-118 ",
		ObservationSetRef:                 " " + handoff.ObservationSetRef + " ",
		MaterializerContractRef:           " " + handoff.MaterializerContractRef + " ",
		EquivalenceCheckRef:               " " + handoff.EquivalenceCheckRef + " ",
		ResidualRiskRef:                   " " + handoff.ResidualRiskRef + " ",
		RollbackPlanRef:                   " " + handoff.RollbackPlanRef + " ",
		NoRuntimeMaterialization:          true,
		NoDurableComputerMutation:         true,
		NoPackagePublicationMutation:      true,
		NoPromotionMutation:               true,
		NoRunAcceptanceMutation:           true,
		NoProductionMutation:              true,
		RuntimeMaterialized:               false,
		DurableComputerStateMutated:       false,
		PackagePublished:                  false,
		PromotionExecuted:                 false,
		RunAcceptanceRecordTouched:        false,
		ProductionStateMutated:            false,
		FullSubstrateClaimed:              false,
		CompletionClaimed:                 false,
	}
}
