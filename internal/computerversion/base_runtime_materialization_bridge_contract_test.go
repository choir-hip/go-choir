package computerversion

import (
	"strings"
	"testing"
)

func TestBuildBaseRuntimeMaterializationBridgeContractRecordsScopedRuntimeCeremonyEvidence(t *testing.T) {
	readiness, runtime, evidence := baseRuntimeMaterializationBridgeContractInputs(t)

	contract, err := BuildBaseRuntimeMaterializationBridgeContract(readiness, runtime, evidence)
	if err != nil {
		t.Fatalf("BuildBaseRuntimeMaterializationBridgeContract(): %v", err)
	}

	if contract.Kind != BaseRuntimeMaterializationBridgeContractKind {
		t.Fatalf("kind = %q, want %q", contract.Kind, BaseRuntimeMaterializationBridgeContractKind)
	}
	if contract.Version != readiness.Version || contract.Version != runtime.Version || ArtifactProgramRef(contract.TypedArtifactProgramRef) != readiness.Version.ArtifactProgramRef {
		t.Fatalf("version/ref = %#v/%q, want shared readiness/runtime version %#v/%#v and artifact program %q", contract.Version, contract.TypedArtifactProgramRef, readiness.Version, runtime.Version, readiness.Version.ArtifactProgramRef)
	}
	if contract.Boundary != BaseRuntimeMaterializationBridgeBoundary {
		t.Fatalf("boundary = %q, want %q", contract.Boundary, BaseRuntimeMaterializationBridgeBoundary)
	}
	if contract.Scope != BaseRuntimeMaterializationBridgeScope {
		t.Fatalf("scope = %q, want %q", contract.Scope, BaseRuntimeMaterializationBridgeScope)
	}
	if contract.SourceMaterializerReadinessRef != strings.TrimSpace(evidence.SourceMaterializerReadinessRef) || contract.RuntimeMaterializationRef != strings.TrimSpace(evidence.RuntimeMaterializationRef) || contract.RuntimeEvidenceReviewRef != strings.TrimSpace(evidence.RuntimeEvidenceReviewRef) || contract.BridgeReviewRef != strings.TrimSpace(evidence.BridgeReviewRef) || contract.RollbackPlanRef != strings.TrimSpace(evidence.RollbackPlanRef) {
		t.Fatalf("bridge refs = readiness %q runtime %q review %q bridge %q rollback %q, want trimmed evidence refs %#v", contract.SourceMaterializerReadinessRef, contract.RuntimeMaterializationRef, contract.RuntimeEvidenceReviewRef, contract.BridgeReviewRef, contract.RollbackPlanRef, evidence)
	}
	if contract.PostPromotionSettlementHandoffRef != strings.TrimSpace(readiness.PostPromotionSettlementHandoffRef) || contract.SourceProvenanceReadinessRef != strings.TrimSpace(readiness.SourceProvenanceReadinessRef) || contract.MaterializerBoundaryRef != strings.TrimSpace(readiness.MaterializerBoundaryRef) {
		t.Fatalf("readiness refs = handoff %q source %q materializer %q, want trimmed readiness refs %#v", contract.PostPromotionSettlementHandoffRef, contract.SourceProvenanceReadinessRef, contract.MaterializerBoundaryRef, readiness)
	}
	if contract.RealizationEvidenceRef != strings.TrimSpace(runtime.RealizationEvidenceRef) || contract.MaterializationCommandRef != strings.TrimSpace(runtime.MaterializationCommandRef) || contract.RealizationID != strings.TrimSpace(runtime.RealizationID) || contract.Materializer != strings.TrimSpace(runtime.Materializer) || contract.Substrate != strings.TrimSpace(runtime.Substrate) {
		t.Fatalf("runtime refs = realization evidence %q command %q id %q materializer %q substrate %q, want trimmed runtime refs %#v", contract.RealizationEvidenceRef, contract.MaterializationCommandRef, contract.RealizationID, contract.Materializer, contract.Substrate, runtime)
	}
	assertObservationBundleKinds(t, contract.SourceRequiredObservations, []ObservationKind{ObservationBlobSet, ObservationFileManifest})
	assertObservationBundleKinds(t, contract.RuntimeRequiredObservations, []ObservationKind{ObservationVMStateManifest})
	if contract.BridgeStatus != BaseRuntimeMaterializationBridgeStatusAccepted {
		t.Fatalf("bridge status = %q, want %q", contract.BridgeStatus, BaseRuntimeMaterializationBridgeStatusAccepted)
	}
	if !contract.SourceMaterializerReady || !contract.RuntimeEvidenceAccepted {
		t.Fatalf("bridge must mark source materializer and runtime evidence ready: %#v", contract)
	}
	if !contract.RuntimeEquivalenceRequired || !contract.StagingProofRequired || !contract.PromotionProofRequired || !contract.PackagePublicationRequired || !contract.RunAcceptanceProofRequired || !contract.FullSubstrateProofRequired {
		t.Fatalf("bridge must preserve downstream runtime-equivalence/staging/promotion/package/run/full-substrate proof requirements: %#v", contract)
	}
	if contract.VMLifecycleMutationAllowed || contract.DurableComputerMutationAllowed || contract.DeployedRouteRegistrationAllowed || contract.ProductionMutationAllowed || contract.PackagePublicationAllowed || contract.PromotionAllowed || contract.RunAcceptanceSynthesisAllowed {
		t.Fatalf("bridge must deny VM lifecycle, durable computer, deployed-route, production, package, promotion, and run-acceptance authority: %#v", contract)
	}
	if !contract.NoVMLifecycleMutation || !contract.NoDurableComputerMutation || !contract.NoDeployedRouteMutation || !contract.NoProductionMutation || !contract.NoPackagePublicationMutation || !contract.NoPromotionMutation || !contract.NoRunAcceptanceMutation {
		t.Fatalf("bridge must carry no-VM/no-durable/no-route/no-production/no-package/no-promotion/no-run flags: %#v", contract)
	}
	if contract.RuntimeBehaviorChanged || contract.DurableComputerStateMutated || contract.DeployedRouteRegistered || contract.ProductionAuthTouched || contract.ProductionStateMutated || contract.PackagePublished || contract.PromotionExecuted || contract.VMLifecycleTouched || contract.FirecrackerBootClaimed || contract.RunAcceptanceRecordTouched || contract.FullSubstrateClaimed || contract.CompletionClaimed {
		t.Fatalf("bridge must not claim runtime behavior, durable mutation, deployed route, production touch, package publication, promotion, VM lifecycle, Firecracker boot, run acceptance, full-substrate, or completion: %#v", contract)
	}
}

func TestBuildBaseRuntimeMaterializationBridgeContractRejectsInvalidInputs(t *testing.T) {
	foreignVersion := ComputerVersion{CodeRef: "git:foreign-base-runtime-materialization-bridge", ArtifactProgramRef: "base-journal:owner/main@cursor-foreign-base-runtime-materialization-bridge"}
	for _, tc := range []struct {
		name    string
		mutate  func(*BaseSourceMaterializerReadinessContract, *BaseRuntimeMaterializationCeremonyContract, *BaseRuntimeMaterializationBridgeEvidence)
		wantErr string
	}{
		{
			name: "readiness kind drift",
			mutate: func(readiness *BaseSourceMaterializerReadinessContract, _ *BaseRuntimeMaterializationCeremonyContract, _ *BaseRuntimeMaterializationBridgeEvidence) {
				readiness.Kind = BaseRuntimeMaterializationBridgeContractKind
			},
			wantErr: "readiness kind",
		},
		{
			name: "readiness boundary drift",
			mutate: func(readiness *BaseSourceMaterializerReadinessContract, _ *BaseRuntimeMaterializationCeremonyContract, _ *BaseRuntimeMaterializationBridgeEvidence) {
				readiness.Boundary = BaseRuntimeMaterializationBridgeBoundary
			},
			wantErr: "readiness boundary",
		},
		{
			name: "readiness scope drift",
			mutate: func(readiness *BaseSourceMaterializerReadinessContract, _ *BaseRuntimeMaterializationCeremonyContract, _ *BaseRuntimeMaterializationBridgeEvidence) {
				readiness.Scope = BaseRuntimeMaterializationBridgeScope
			},
			wantErr: "readiness scope",
		},
		{
			name: "readiness invalid version",
			mutate: func(readiness *BaseSourceMaterializerReadinessContract, _ *BaseRuntimeMaterializationCeremonyContract, _ *BaseRuntimeMaterializationBridgeEvidence) {
				readiness.Version.CodeRef = "  "
			},
			wantErr: "readiness version or artifact ref is invalid",
		},
		{
			name: "readiness typed artifact ref drift",
			mutate: func(readiness *BaseSourceMaterializerReadinessContract, _ *BaseRuntimeMaterializationCeremonyContract, _ *BaseRuntimeMaterializationBridgeEvidence) {
				readiness.TypedArtifactProgramRef = "base-journal:owner/main@cursor-drifted-base-runtime-bridge-readiness"
			},
			wantErr: "readiness version or artifact ref is invalid",
		},
		{
			name: "readiness missing source provenance ref",
			mutate: func(readiness *BaseSourceMaterializerReadinessContract, _ *BaseRuntimeMaterializationCeremonyContract, _ *BaseRuntimeMaterializationBridgeEvidence) {
				readiness.SourceProvenanceReadinessRef = "\t"
			},
			wantErr: "readiness refs are required",
		},
		{
			name: "readiness status drift",
			mutate: func(readiness *BaseSourceMaterializerReadinessContract, _ *BaseRuntimeMaterializationCeremonyContract, _ *BaseRuntimeMaterializationBridgeEvidence) {
				readiness.ReadinessStatus = "runtime_materialized"
			},
			wantErr: "readiness status",
		},
		{
			name: "readiness prerequisite drift",
			mutate: func(readiness *BaseSourceMaterializerReadinessContract, _ *BaseRuntimeMaterializationCeremonyContract, _ *BaseRuntimeMaterializationBridgeEvidence) {
				readiness.RuntimeCeremonyMayOpen = false
			},
			wantErr: "readiness must open runtime ceremony planning",
		},
		{
			name: "readiness downstream proof requirement drift",
			mutate: func(readiness *BaseSourceMaterializerReadinessContract, _ *BaseRuntimeMaterializationCeremonyContract, _ *BaseRuntimeMaterializationBridgeEvidence) {
				readiness.RunAcceptanceProofRequired = false
			},
			wantErr: "readiness must preserve downstream proof requirements",
		},
		{
			name: "readiness authority drift",
			mutate: func(readiness *BaseSourceMaterializerReadinessContract, _ *BaseRuntimeMaterializationCeremonyContract, _ *BaseRuntimeMaterializationBridgeEvidence) {
				readiness.RuntimeMaterializationAllowed = true
			},
			wantErr: "readiness allows downstream execution",
		},
		{
			name: "readiness no mutation flag drift",
			mutate: func(readiness *BaseSourceMaterializerReadinessContract, _ *BaseRuntimeMaterializationCeremonyContract, _ *BaseRuntimeMaterializationBridgeEvidence) {
				readiness.NoProductionMutation = false
			},
			wantErr: "readiness must prove no runtime",
		},
		{
			name: "readiness claim drift",
			mutate: func(readiness *BaseSourceMaterializerReadinessContract, _ *BaseRuntimeMaterializationCeremonyContract, _ *BaseRuntimeMaterializationBridgeEvidence) {
				readiness.ProductionStateMutated = true
			},
			wantErr: "readiness carries materialization",
		},
		{
			name: "runtime kind drift",
			mutate: func(_ *BaseSourceMaterializerReadinessContract, runtime *BaseRuntimeMaterializationCeremonyContract, _ *BaseRuntimeMaterializationBridgeEvidence) {
				runtime.Kind = BaseRuntimeMaterializationBridgeContractKind
			},
			wantErr: "runtime kind",
		},
		{
			name: "runtime boundary drift",
			mutate: func(_ *BaseSourceMaterializerReadinessContract, runtime *BaseRuntimeMaterializationCeremonyContract, _ *BaseRuntimeMaterializationBridgeEvidence) {
				runtime.Boundary = BaseRuntimeMaterializationBridgeBoundary
			},
			wantErr: "runtime boundary",
		},
		{
			name: "runtime scope drift",
			mutate: func(_ *BaseSourceMaterializerReadinessContract, runtime *BaseRuntimeMaterializationCeremonyContract, _ *BaseRuntimeMaterializationBridgeEvidence) {
				runtime.Scope = BaseRuntimeMaterializationBridgeScope
			},
			wantErr: "runtime scope",
		},
		{
			name: "runtime version drift",
			mutate: func(_ *BaseSourceMaterializerReadinessContract, runtime *BaseRuntimeMaterializationCeremonyContract, _ *BaseRuntimeMaterializationBridgeEvidence) {
				runtime.Version = foreignVersion
			},
			wantErr: "runtime version does not match readiness",
		},
		{
			name: "runtime typed artifact ref drift",
			mutate: func(_ *BaseSourceMaterializerReadinessContract, runtime *BaseRuntimeMaterializationCeremonyContract, _ *BaseRuntimeMaterializationBridgeEvidence) {
				runtime.TypedArtifactProgramRef = "base-journal:owner/main@cursor-drifted-base-runtime-bridge-runtime"
			},
			wantErr: "runtime typed artifact ref is invalid",
		},
		{
			name: "runtime source readiness ref drift",
			mutate: func(_ *BaseSourceMaterializerReadinessContract, runtime *BaseRuntimeMaterializationCeremonyContract, _ *BaseRuntimeMaterializationBridgeEvidence) {
				runtime.SourceProvenanceReadinessRef = "source-provenance-readiness:other-base-pass"
			},
			wantErr: "runtime refs are required",
		},
		{
			name: "runtime missing materialization command ref",
			mutate: func(_ *BaseSourceMaterializerReadinessContract, runtime *BaseRuntimeMaterializationCeremonyContract, _ *BaseRuntimeMaterializationBridgeEvidence) {
				runtime.MaterializationCommandRef = " "
			},
			wantErr: "runtime refs are required",
		},
		{
			name: "runtime source observations incomplete",
			mutate: func(_ *BaseSourceMaterializerReadinessContract, runtime *BaseRuntimeMaterializationCeremonyContract, _ *BaseRuntimeMaterializationBridgeEvidence) {
				runtime.SourceRequiredObservations = []ObservationKind{ObservationBlobSet}
			},
			wantErr: "runtime observations are incomplete",
		},
		{
			name: "runtime vm state observation missing",
			mutate: func(_ *BaseSourceMaterializerReadinessContract, runtime *BaseRuntimeMaterializationCeremonyContract, _ *BaseRuntimeMaterializationBridgeEvidence) {
				runtime.RuntimeRequiredObservations = nil
			},
			wantErr: "runtime observations are incomplete",
		},
		{
			name: "runtime downstream proof requirement drift",
			mutate: func(_ *BaseSourceMaterializerReadinessContract, runtime *BaseRuntimeMaterializationCeremonyContract, _ *BaseRuntimeMaterializationBridgeEvidence) {
				runtime.PromotionProofRequired = false
			},
			wantErr: "runtime contract must preserve proof requirements",
		},
		{
			name: "runtime no VM lifecycle flag drift",
			mutate: func(_ *BaseSourceMaterializerReadinessContract, runtime *BaseRuntimeMaterializationCeremonyContract, _ *BaseRuntimeMaterializationBridgeEvidence) {
				runtime.NoVMLifecycleMutation = false
			},
			wantErr: "runtime contract must prove no VM lifecycle or production mutation",
		},
		{
			name: "runtime no production flag drift",
			mutate: func(_ *BaseSourceMaterializerReadinessContract, runtime *BaseRuntimeMaterializationCeremonyContract, _ *BaseRuntimeMaterializationBridgeEvidence) {
				runtime.NoProductionMutation = false
			},
			wantErr: "runtime contract must prove no VM lifecycle or production mutation",
		},
		{
			name: "runtime behavior changed claim",
			mutate: func(_ *BaseSourceMaterializerReadinessContract, runtime *BaseRuntimeMaterializationCeremonyContract, _ *BaseRuntimeMaterializationBridgeEvidence) {
				runtime.RuntimeBehaviorChanged = true
			},
			wantErr: "runtime contract carries protected-surface or completion claims",
		},
		{
			name: "runtime deployed route registration claim",
			mutate: func(_ *BaseSourceMaterializerReadinessContract, runtime *BaseRuntimeMaterializationCeremonyContract, _ *BaseRuntimeMaterializationBridgeEvidence) {
				runtime.DeployedRouteRegistered = true
			},
			wantErr: "runtime contract carries protected-surface or completion claims",
		},
		{
			name: "runtime production auth touch claim",
			mutate: func(_ *BaseSourceMaterializerReadinessContract, runtime *BaseRuntimeMaterializationCeremonyContract, _ *BaseRuntimeMaterializationBridgeEvidence) {
				runtime.ProductionAuthTouched = true
			},
			wantErr: "runtime contract carries protected-surface or completion claims",
		},
		{
			name: "runtime VM lifecycle touch claim",
			mutate: func(_ *BaseSourceMaterializerReadinessContract, runtime *BaseRuntimeMaterializationCeremonyContract, _ *BaseRuntimeMaterializationBridgeEvidence) {
				runtime.VMLifecycleTouched = true
			},
			wantErr: "runtime contract carries protected-surface or completion claims",
		},
		{
			name: "runtime Firecracker boot claim",
			mutate: func(_ *BaseSourceMaterializerReadinessContract, runtime *BaseRuntimeMaterializationCeremonyContract, _ *BaseRuntimeMaterializationBridgeEvidence) {
				runtime.FirecrackerBootClaimed = true
			},
			wantErr: "runtime contract carries protected-surface or completion claims",
		},
		{
			name: "runtime run acceptance touch claim",
			mutate: func(_ *BaseSourceMaterializerReadinessContract, runtime *BaseRuntimeMaterializationCeremonyContract, _ *BaseRuntimeMaterializationBridgeEvidence) {
				runtime.RunAcceptanceRecordTouched = true
			},
			wantErr: "runtime contract carries protected-surface or completion claims",
		},
		{
			name: "runtime package publication claim",
			mutate: func(_ *BaseSourceMaterializerReadinessContract, runtime *BaseRuntimeMaterializationCeremonyContract, _ *BaseRuntimeMaterializationBridgeEvidence) {
				runtime.PackagePublicationClaimed = true
			},
			wantErr: "runtime contract carries protected-surface or completion claims",
		},
		{
			name: "runtime full substrate claim",
			mutate: func(_ *BaseSourceMaterializerReadinessContract, runtime *BaseRuntimeMaterializationCeremonyContract, _ *BaseRuntimeMaterializationBridgeEvidence) {
				runtime.FullSubstrateClaimed = true
			},
			wantErr: "runtime contract carries protected-surface or completion claims",
		},
		{
			name: "runtime completion claim",
			mutate: func(_ *BaseSourceMaterializerReadinessContract, runtime *BaseRuntimeMaterializationCeremonyContract, _ *BaseRuntimeMaterializationBridgeEvidence) {
				runtime.CompletionClaimed = true
			},
			wantErr: "runtime contract carries protected-surface or completion claims",
		},
		{
			name: "missing source materializer readiness evidence ref",
			mutate: func(_ *BaseSourceMaterializerReadinessContract, _ *BaseRuntimeMaterializationCeremonyContract, evidence *BaseRuntimeMaterializationBridgeEvidence) {
				evidence.SourceMaterializerReadinessRef = "\t"
			},
			wantErr: "evidence refs are required",
		},
		{
			name: "missing runtime materialization evidence ref",
			mutate: func(_ *BaseSourceMaterializerReadinessContract, _ *BaseRuntimeMaterializationCeremonyContract, evidence *BaseRuntimeMaterializationBridgeEvidence) {
				evidence.RuntimeMaterializationRef = " "
			},
			wantErr: "evidence refs are required",
		},
		{
			name: "missing runtime evidence review ref",
			mutate: func(_ *BaseSourceMaterializerReadinessContract, _ *BaseRuntimeMaterializationCeremonyContract, evidence *BaseRuntimeMaterializationBridgeEvidence) {
				evidence.RuntimeEvidenceReviewRef = ""
			},
			wantErr: "evidence refs are required",
		},
		{
			name: "missing bridge review ref",
			mutate: func(_ *BaseSourceMaterializerReadinessContract, _ *BaseRuntimeMaterializationCeremonyContract, evidence *BaseRuntimeMaterializationBridgeEvidence) {
				evidence.BridgeReviewRef = "\t"
			},
			wantErr: "evidence refs are required",
		},
		{
			name: "missing rollback ref",
			mutate: func(_ *BaseSourceMaterializerReadinessContract, _ *BaseRuntimeMaterializationCeremonyContract, evidence *BaseRuntimeMaterializationBridgeEvidence) {
				evidence.RollbackPlanRef = " "
			},
			wantErr: "evidence refs are required",
		},
		{
			name: "missing evidence no VM lifecycle flag",
			mutate: func(_ *BaseSourceMaterializerReadinessContract, _ *BaseRuntimeMaterializationCeremonyContract, evidence *BaseRuntimeMaterializationBridgeEvidence) {
				evidence.NoVMLifecycleMutation = false
			},
			wantErr: "evidence must prove no VM lifecycle",
		},
		{
			name: "missing evidence no durable computer flag",
			mutate: func(_ *BaseSourceMaterializerReadinessContract, _ *BaseRuntimeMaterializationCeremonyContract, evidence *BaseRuntimeMaterializationBridgeEvidence) {
				evidence.NoDurableComputerMutation = false
			},
			wantErr: "evidence must prove no VM lifecycle",
		},
		{
			name: "missing evidence no deployed route flag",
			mutate: func(_ *BaseSourceMaterializerReadinessContract, _ *BaseRuntimeMaterializationCeremonyContract, evidence *BaseRuntimeMaterializationBridgeEvidence) {
				evidence.NoDeployedRouteMutation = false
			},
			wantErr: "evidence must prove no VM lifecycle",
		},
		{
			name: "missing evidence no production flag",
			mutate: func(_ *BaseSourceMaterializerReadinessContract, _ *BaseRuntimeMaterializationCeremonyContract, evidence *BaseRuntimeMaterializationBridgeEvidence) {
				evidence.NoProductionMutation = false
			},
			wantErr: "evidence must prove no VM lifecycle",
		},
		{
			name: "missing evidence no package publication flag",
			mutate: func(_ *BaseSourceMaterializerReadinessContract, _ *BaseRuntimeMaterializationCeremonyContract, evidence *BaseRuntimeMaterializationBridgeEvidence) {
				evidence.NoPackagePublicationMutation = false
			},
			wantErr: "evidence must prove no VM lifecycle",
		},
		{
			name: "missing evidence no promotion flag",
			mutate: func(_ *BaseSourceMaterializerReadinessContract, _ *BaseRuntimeMaterializationCeremonyContract, evidence *BaseRuntimeMaterializationBridgeEvidence) {
				evidence.NoPromotionMutation = false
			},
			wantErr: "evidence must prove no VM lifecycle",
		},
		{
			name: "missing evidence no run acceptance flag",
			mutate: func(_ *BaseSourceMaterializerReadinessContract, _ *BaseRuntimeMaterializationCeremonyContract, evidence *BaseRuntimeMaterializationBridgeEvidence) {
				evidence.NoRunAcceptanceMutation = false
			},
			wantErr: "evidence must prove no VM lifecycle",
		},
		{
			name: "runtime behavior changed evidence claim",
			mutate: func(_ *BaseSourceMaterializerReadinessContract, _ *BaseRuntimeMaterializationCeremonyContract, evidence *BaseRuntimeMaterializationBridgeEvidence) {
				evidence.RuntimeBehaviorChanged = true
			},
			wantErr: "evidence carries runtime",
		},
		{
			name: "durable computer mutation evidence claim",
			mutate: func(_ *BaseSourceMaterializerReadinessContract, _ *BaseRuntimeMaterializationCeremonyContract, evidence *BaseRuntimeMaterializationBridgeEvidence) {
				evidence.DurableComputerStateMutated = true
			},
			wantErr: "evidence carries runtime",
		},
		{
			name: "deployed route registration evidence claim",
			mutate: func(_ *BaseSourceMaterializerReadinessContract, _ *BaseRuntimeMaterializationCeremonyContract, evidence *BaseRuntimeMaterializationBridgeEvidence) {
				evidence.DeployedRouteRegistered = true
			},
			wantErr: "evidence carries runtime",
		},
		{
			name: "production auth touch evidence claim",
			mutate: func(_ *BaseSourceMaterializerReadinessContract, _ *BaseRuntimeMaterializationCeremonyContract, evidence *BaseRuntimeMaterializationBridgeEvidence) {
				evidence.ProductionAuthTouched = true
			},
			wantErr: "evidence carries runtime",
		},
		{
			name: "production state mutation evidence claim",
			mutate: func(_ *BaseSourceMaterializerReadinessContract, _ *BaseRuntimeMaterializationCeremonyContract, evidence *BaseRuntimeMaterializationBridgeEvidence) {
				evidence.ProductionStateMutated = true
			},
			wantErr: "evidence carries runtime",
		},
		{
			name: "package publication evidence claim",
			mutate: func(_ *BaseSourceMaterializerReadinessContract, _ *BaseRuntimeMaterializationCeremonyContract, evidence *BaseRuntimeMaterializationBridgeEvidence) {
				evidence.PackagePublished = true
			},
			wantErr: "evidence carries runtime",
		},
		{
			name: "promotion execution evidence claim",
			mutate: func(_ *BaseSourceMaterializerReadinessContract, _ *BaseRuntimeMaterializationCeremonyContract, evidence *BaseRuntimeMaterializationBridgeEvidence) {
				evidence.PromotionExecuted = true
			},
			wantErr: "evidence carries runtime",
		},
		{
			name: "VM lifecycle evidence touch",
			mutate: func(_ *BaseSourceMaterializerReadinessContract, _ *BaseRuntimeMaterializationCeremonyContract, evidence *BaseRuntimeMaterializationBridgeEvidence) {
				evidence.VMLifecycleTouched = true
			},
			wantErr: "evidence carries runtime",
		},
		{
			name: "Firecracker boot evidence claim",
			mutate: func(_ *BaseSourceMaterializerReadinessContract, _ *BaseRuntimeMaterializationCeremonyContract, evidence *BaseRuntimeMaterializationBridgeEvidence) {
				evidence.FirecrackerBootClaimed = true
			},
			wantErr: "evidence carries runtime",
		},
		{
			name: "run acceptance evidence touch",
			mutate: func(_ *BaseSourceMaterializerReadinessContract, _ *BaseRuntimeMaterializationCeremonyContract, evidence *BaseRuntimeMaterializationBridgeEvidence) {
				evidence.RunAcceptanceRecordTouched = true
			},
			wantErr: "evidence carries runtime",
		},
		{
			name: "full substrate evidence claim",
			mutate: func(_ *BaseSourceMaterializerReadinessContract, _ *BaseRuntimeMaterializationCeremonyContract, evidence *BaseRuntimeMaterializationBridgeEvidence) {
				evidence.FullSubstrateClaimed = true
			},
			wantErr: "evidence carries runtime",
		},
		{
			name: "completion evidence claim",
			mutate: func(_ *BaseSourceMaterializerReadinessContract, _ *BaseRuntimeMaterializationCeremonyContract, evidence *BaseRuntimeMaterializationBridgeEvidence) {
				evidence.CompletionClaimed = true
			},
			wantErr: "evidence carries runtime",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			readiness, runtime, evidence := baseRuntimeMaterializationBridgeContractInputs(t)
			tc.mutate(&readiness, &runtime, &evidence)

			contract, err := BuildBaseRuntimeMaterializationBridgeContract(readiness, runtime, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildBaseRuntimeMaterializationBridgeContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func baseRuntimeMaterializationBridgeContractInputs(t *testing.T) (BaseSourceMaterializerReadinessContract, BaseRuntimeMaterializationCeremonyContract, BaseRuntimeMaterializationBridgeEvidence) {
	t.Helper()

	probe, source, materializer, readinessEvidence := baseSourceMaterializerReadinessContractInputs(t)
	readiness, err := BuildBaseSourceMaterializerReadinessContract(probe, source, materializer, readinessEvidence)
	if err != nil {
		t.Fatalf("BuildBaseSourceMaterializerReadinessContract(): %v", err)
	}

	realization := mustMaterializeVMManagerBoundary(t, "base-runtime-materialization-bridge-vmmanager", readiness.Version, vmManagerBoundaryPath(), VMManagerCapabilityManifest("base-runtime-materialization-bridge-vmmanager"))
	runtimeEvidence := BaseRuntimeMaterializationCeremonyEvidence{
		SourceProvenanceReadinessRef: " " + readiness.SourceProvenanceReadinessRef + " ",
		RealizationEvidenceRef:       " vmmanager:base-runtime-materialization-bridge-pass-121 ",
		MaterializationCommandRef:    " go-test:internal/computerversion/base-runtime-materialization-bridge ",
		NoVMLifecycleMutation:        true,
		NoProductionMutation:         true,
	}
	runtime, err := BuildBaseRuntimeMaterializationCeremonyContract(source, realization, runtimeEvidence)
	if err != nil {
		t.Fatalf("BuildBaseRuntimeMaterializationCeremonyContract(): %v", err)
	}

	evidence := BaseRuntimeMaterializationBridgeEvidence{
		SourceMaterializerReadinessRef: " source-materializer-readiness:base-pass-121 ",
		RuntimeMaterializationRef:      " runtime-materialization:base-pass-121 ",
		RuntimeEvidenceReviewRef:       " runtime-evidence-review:base-pass-121 ",
		BridgeReviewRef:                " bridge-review:base-runtime-materialization-pass-121 ",
		RollbackPlanRef:                " rollback-plan:base-runtime-materialization-bridge-pass-121 ",
		NoVMLifecycleMutation:          true,
		NoDurableComputerMutation:      true,
		NoDeployedRouteMutation:        true,
		NoProductionMutation:           true,
		NoPackagePublicationMutation:   true,
		NoPromotionMutation:            true,
		NoRunAcceptanceMutation:        true,
	}
	return readiness, runtime, evidence
}
