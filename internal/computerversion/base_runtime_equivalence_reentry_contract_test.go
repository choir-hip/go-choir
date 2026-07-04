package computerversion

import (
	"strings"
	"testing"
)

func TestBuildBaseRuntimeEquivalenceReentryContractRecordsNarrowedRuntimeEquivalenceReentry(t *testing.T) {
	bridge, equivalence, evidence := baseRuntimeEquivalenceReentryContractInputs(t)

	contract, err := BuildBaseRuntimeEquivalenceReentryContract(bridge, equivalence, evidence)
	if err != nil {
		t.Fatalf("BuildBaseRuntimeEquivalenceReentryContract(): %v", err)
	}

	if contract.Kind != BaseRuntimeEquivalenceReentryContractKind {
		t.Fatalf("kind = %q, want %q", contract.Kind, BaseRuntimeEquivalenceReentryContractKind)
	}
	if contract.Version != bridge.Version || contract.Version != equivalence.Version || ArtifactProgramRef(contract.TypedArtifactProgramRef) != bridge.Version.ArtifactProgramRef {
		t.Fatalf("version/ref = %#v/%q, want shared bridge/equivalence version %#v/%#v and artifact program %q", contract.Version, contract.TypedArtifactProgramRef, bridge.Version, equivalence.Version, bridge.Version.ArtifactProgramRef)
	}
	if contract.Boundary != BaseRuntimeEquivalenceReentryBoundary {
		t.Fatalf("boundary = %q, want %q", contract.Boundary, BaseRuntimeEquivalenceReentryBoundary)
	}
	if contract.Scope != BaseRuntimeEquivalenceReentryScope {
		t.Fatalf("scope = %q, want %q", contract.Scope, BaseRuntimeEquivalenceReentryScope)
	}
	if contract.RuntimeMaterializationBridgeRef != strings.TrimSpace(evidence.RuntimeMaterializationBridgeRef) || contract.RuntimeEquivalenceBoundaryRef != strings.TrimSpace(evidence.RuntimeEquivalenceBoundaryRef) || contract.ReentryReviewRef != strings.TrimSpace(evidence.ReentryReviewRef) || contract.RollbackPlanRef != strings.TrimSpace(evidence.RollbackPlanRef) {
		t.Fatalf("reentry refs = bridge %q equivalence %q review %q rollback %q, want trimmed refs from %#v", contract.RuntimeMaterializationBridgeRef, contract.RuntimeEquivalenceBoundaryRef, contract.ReentryReviewRef, contract.RollbackPlanRef, evidence)
	}
	if contract.SourceMaterializerReadinessRef != bridge.SourceMaterializerReadinessRef || contract.RuntimeMaterializationRef != bridge.RuntimeMaterializationRef || contract.RuntimeEvidenceReviewRef != bridge.RuntimeEvidenceReviewRef || contract.SourceProvenanceReadinessRef != bridge.SourceProvenanceReadinessRef || contract.MaterializerBoundaryRef != bridge.MaterializerBoundaryRef {
		t.Fatalf("bridge refs = source materializer %q runtime materialization %q runtime review %q source %q materializer boundary %q, want bridge refs %#v", contract.SourceMaterializerReadinessRef, contract.RuntimeMaterializationRef, contract.RuntimeEvidenceReviewRef, contract.SourceProvenanceReadinessRef, contract.MaterializerBoundaryRef, bridge)
	}
	if contract.RealizationEvidenceRef != bridge.RealizationEvidenceRef || contract.Materializer != bridge.Materializer || contract.Substrate != bridge.Substrate {
		t.Fatalf("runtime identity = realization %q materializer %q substrate %q, want bridge %q/%q/%q", contract.RealizationEvidenceRef, contract.Materializer, contract.Substrate, bridge.RealizationEvidenceRef, bridge.Materializer, bridge.Substrate)
	}
	if equivalence.RuntimeMaterializationCeremonyRef != bridge.RuntimeMaterializationRef {
		t.Fatalf("test fixture boundary runtime materialization ref = %q, want bridge runtime materialization ref %q", equivalence.RuntimeMaterializationCeremonyRef, bridge.RuntimeMaterializationRef)
	}
	assertObservationBundleKinds(t, contract.SourceRequiredObservations, []ObservationKind{ObservationBlobSet, ObservationFileManifest})
	assertObservationBundleKinds(t, contract.RuntimeRequiredObservations, []ObservationKind{ObservationVMStateManifest})
	assertUnsupportedDurableObservations(t, contract.UnsupportedDurableObservations, []ObservationKind{ObservationFileManifest, ObservationBlobSet})
	if contract.ReentryStatus != BaseRuntimeEquivalenceReentryStatusNarrowed {
		t.Fatalf("reentry status = %q, want %q", contract.ReentryStatus, BaseRuntimeEquivalenceReentryStatusNarrowed)
	}
	if !contract.RuntimeEvidenceAccepted || !contract.RuntimeEquivalenceNarrowed || contract.RuntimeEquivalenceClaimed {
		t.Fatalf("reentry equivalence = accepted %v narrowed %v claimed %v, want accepted/narrowed true and claimed false", contract.RuntimeEvidenceAccepted, contract.RuntimeEquivalenceNarrowed, contract.RuntimeEquivalenceClaimed)
	}
	if !contract.DurableStateEquivalenceRequired || !contract.StagingProofRequired || !contract.PromotionProofRequired || !contract.PackagePublicationRequired || !contract.RunAcceptanceProofRequired || !contract.FullSubstrateProofRequired {
		t.Fatalf("reentry contract must preserve downstream durable/staging/promotion/package/run/full-substrate proof requirements: %#v", contract)
	}
	if contract.VMLifecycleMutationAllowed || contract.DurableComputerMutationAllowed || contract.DeployedRouteRegistrationAllowed || contract.ProductionMutationAllowed || contract.PackagePublicationAllowed || contract.PromotionAllowed || contract.RunAcceptanceSynthesisAllowed {
		t.Fatalf("reentry contract must deny VM lifecycle, durable computer, deployed route, production, package, promotion, and run acceptance authority: %#v", contract)
	}
	if !contract.NoVMLifecycleMutation || !contract.NoDurableComputerMutation || !contract.NoDeployedRouteMutation || !contract.NoProductionMutation || !contract.NoPackagePublicationMutation || !contract.NoPromotionMutation || !contract.NoRunAcceptanceMutation {
		t.Fatalf("reentry contract must carry no-VM/no-durable/no-route/no-production/no-package/no-promotion/no-run flags: %#v", contract)
	}
	if contract.RuntimeBehaviorChanged || contract.DurableComputerStateMutated || contract.DeployedRouteRegistered || contract.ProductionAuthTouched || contract.ProductionStateMutated || contract.PackagePublished || contract.PromotionExecuted || contract.VMLifecycleTouched || contract.FirecrackerBootClaimed || contract.RunAcceptanceRecordTouched || contract.FullSubstrateClaimed || contract.CompletionClaimed {
		t.Fatalf("reentry contract must not claim runtime behavior, durable mutation, deployed route, production touch, package publication, promotion, VM lifecycle, Firecracker boot, run acceptance, full-substrate, or completion: %#v", contract)
	}
}

func TestBuildBaseRuntimeEquivalenceReentryContractRejectsInvalidInputs(t *testing.T) {
	foreignVersion := ComputerVersion{CodeRef: "git:foreign-base-runtime-equivalence-reentry", ArtifactProgramRef: "base-journal:owner/main@foreign-base-runtime-equivalence-reentry"}
	for _, tc := range []struct {
		name    string
		mutate  func(*BaseRuntimeMaterializationBridgeContract, *BaseRuntimeEquivalenceBoundaryContract, *BaseRuntimeEquivalenceReentryEvidence)
		wantErr string
	}{
		{
			name: "bridge kind drift",
			mutate: func(bridge *BaseRuntimeMaterializationBridgeContract, _ *BaseRuntimeEquivalenceBoundaryContract, _ *BaseRuntimeEquivalenceReentryEvidence) {
				bridge.Kind = BaseRuntimeEquivalenceReentryContractKind
			},
			wantErr: "bridge kind",
		},
		{
			name: "bridge boundary drift",
			mutate: func(bridge *BaseRuntimeMaterializationBridgeContract, _ *BaseRuntimeEquivalenceBoundaryContract, _ *BaseRuntimeEquivalenceReentryEvidence) {
				bridge.Boundary = BaseRuntimeEquivalenceReentryBoundary
			},
			wantErr: "bridge boundary",
		},
		{
			name: "bridge scope drift",
			mutate: func(bridge *BaseRuntimeMaterializationBridgeContract, _ *BaseRuntimeEquivalenceBoundaryContract, _ *BaseRuntimeEquivalenceReentryEvidence) {
				bridge.Scope = BaseRuntimeEquivalenceReentryScope
			},
			wantErr: "bridge scope",
		},
		{
			name: "bridge invalid version",
			mutate: func(bridge *BaseRuntimeMaterializationBridgeContract, _ *BaseRuntimeEquivalenceBoundaryContract, _ *BaseRuntimeEquivalenceReentryEvidence) {
				bridge.Version.CodeRef = "  "
			},
			wantErr: "bridge version or artifact ref is invalid",
		},
		{
			name: "bridge typed artifact ref drift",
			mutate: func(bridge *BaseRuntimeMaterializationBridgeContract, _ *BaseRuntimeEquivalenceBoundaryContract, _ *BaseRuntimeEquivalenceReentryEvidence) {
				bridge.TypedArtifactProgramRef = string(foreignVersion.ArtifactProgramRef)
			},
			wantErr: "bridge version or artifact ref is invalid",
		},
		{
			name: "bridge missing runtime materialization ref",
			mutate: func(bridge *BaseRuntimeMaterializationBridgeContract, _ *BaseRuntimeEquivalenceBoundaryContract, _ *BaseRuntimeEquivalenceReentryEvidence) {
				bridge.RuntimeMaterializationRef = "  "
			},
			wantErr: "bridge refs are required",
		},
		{
			name: "bridge status drift",
			mutate: func(bridge *BaseRuntimeMaterializationBridgeContract, _ *BaseRuntimeEquivalenceBoundaryContract, _ *BaseRuntimeEquivalenceReentryEvidence) {
				bridge.BridgeStatus = "runtime_materialization_pending"
			},
			wantErr: "bridge status",
		},
		{
			name: "bridge source observations incomplete",
			mutate: func(bridge *BaseRuntimeMaterializationBridgeContract, _ *BaseRuntimeEquivalenceBoundaryContract, _ *BaseRuntimeEquivalenceReentryEvidence) {
				bridge.SourceRequiredObservations = []ObservationKind{ObservationFileManifest}
			},
			wantErr: "bridge observations are incomplete",
		},
		{
			name: "bridge runtime observations incomplete",
			mutate: func(bridge *BaseRuntimeMaterializationBridgeContract, _ *BaseRuntimeEquivalenceBoundaryContract, _ *BaseRuntimeEquivalenceReentryEvidence) {
				bridge.RuntimeRequiredObservations = nil
			},
			wantErr: "bridge observations are incomplete",
		},
		{
			name: "bridge runtime evidence not accepted",
			mutate: func(bridge *BaseRuntimeMaterializationBridgeContract, _ *BaseRuntimeEquivalenceBoundaryContract, _ *BaseRuntimeEquivalenceReentryEvidence) {
				bridge.RuntimeEvidenceAccepted = false
			},
			wantErr: "bridge must carry accepted runtime evidence",
		},
		{
			name: "bridge downstream proof requirement drift",
			mutate: func(bridge *BaseRuntimeMaterializationBridgeContract, _ *BaseRuntimeEquivalenceBoundaryContract, _ *BaseRuntimeEquivalenceReentryEvidence) {
				bridge.FullSubstrateProofRequired = false
			},
			wantErr: "bridge must preserve downstream proof requirements",
		},
		{
			name: "bridge authority drift",
			mutate: func(bridge *BaseRuntimeMaterializationBridgeContract, _ *BaseRuntimeEquivalenceBoundaryContract, _ *BaseRuntimeEquivalenceReentryEvidence) {
				bridge.DeployedRouteRegistrationAllowed = true
			},
			wantErr: "bridge allows downstream execution",
		},
		{
			name: "bridge no mutation flag drift",
			mutate: func(bridge *BaseRuntimeMaterializationBridgeContract, _ *BaseRuntimeEquivalenceBoundaryContract, _ *BaseRuntimeEquivalenceReentryEvidence) {
				bridge.NoDurableComputerMutation = false
			},
			wantErr: "bridge must prove no mutation",
		},
		{
			name: "bridge claim drift",
			mutate: func(bridge *BaseRuntimeMaterializationBridgeContract, _ *BaseRuntimeEquivalenceBoundaryContract, _ *BaseRuntimeEquivalenceReentryEvidence) {
				bridge.PackagePublished = true
			},
			wantErr: "bridge carries protected-surface or completion claims",
		},
		{
			name: "equivalence kind drift",
			mutate: func(_ *BaseRuntimeMaterializationBridgeContract, equivalence *BaseRuntimeEquivalenceBoundaryContract, _ *BaseRuntimeEquivalenceReentryEvidence) {
				equivalence.Kind = BaseRuntimeEquivalenceReentryContractKind
			},
			wantErr: "equivalence kind",
		},
		{
			name: "equivalence boundary drift",
			mutate: func(_ *BaseRuntimeMaterializationBridgeContract, equivalence *BaseRuntimeEquivalenceBoundaryContract, _ *BaseRuntimeEquivalenceReentryEvidence) {
				equivalence.Boundary = BaseRuntimeEquivalenceReentryBoundary
			},
			wantErr: "equivalence boundary",
		},
		{
			name: "equivalence scope drift",
			mutate: func(_ *BaseRuntimeMaterializationBridgeContract, equivalence *BaseRuntimeEquivalenceBoundaryContract, _ *BaseRuntimeEquivalenceReentryEvidence) {
				equivalence.Scope = BaseRuntimeEquivalenceReentryScope
			},
			wantErr: "equivalence scope",
		},
		{
			name: "equivalence version drift",
			mutate: func(_ *BaseRuntimeMaterializationBridgeContract, equivalence *BaseRuntimeEquivalenceBoundaryContract, _ *BaseRuntimeEquivalenceReentryEvidence) {
				equivalence.Version = foreignVersion
			},
			wantErr: "equivalence version does not match bridge",
		},
		{
			name: "equivalence typed artifact ref drift",
			mutate: func(_ *BaseRuntimeMaterializationBridgeContract, equivalence *BaseRuntimeEquivalenceBoundaryContract, _ *BaseRuntimeEquivalenceReentryEvidence) {
				equivalence.TypedArtifactProgramRef = string(foreignVersion.ArtifactProgramRef)
			},
			wantErr: "equivalence typed artifact ref is invalid",
		},
		{
			name: "equivalence runtime materialization ref drift",
			mutate: func(_ *BaseRuntimeMaterializationBridgeContract, equivalence *BaseRuntimeEquivalenceBoundaryContract, _ *BaseRuntimeEquivalenceReentryEvidence) {
				equivalence.RuntimeMaterializationCeremonyRef = "runtime-materialization:other-base-pass"
			},
			wantErr: "equivalence refs do not match bridge",
		},
		{
			name: "equivalence source observations incomplete",
			mutate: func(_ *BaseRuntimeMaterializationBridgeContract, equivalence *BaseRuntimeEquivalenceBoundaryContract, _ *BaseRuntimeEquivalenceReentryEvidence) {
				equivalence.SourceRequiredObservations = []ObservationKind{ObservationBlobSet}
			},
			wantErr: "equivalence observations are incomplete",
		},
		{
			name: "equivalence unsupported file manifest missing",
			mutate: func(_ *BaseRuntimeMaterializationBridgeContract, equivalence *BaseRuntimeEquivalenceBoundaryContract, _ *BaseRuntimeEquivalenceReentryEvidence) {
				equivalence.UnsupportedDurableObservations = []UnsupportedCapability{{Kind: ObservationBlobSet, Reason: "runtime evidence does not expose source blobs"}}
			},
			wantErr: "equivalence must remain narrowed by unsupported file_manifest and blob_set observations",
		},
		{
			name: "equivalence unsupported blob set missing",
			mutate: func(_ *BaseRuntimeMaterializationBridgeContract, equivalence *BaseRuntimeEquivalenceBoundaryContract, _ *BaseRuntimeEquivalenceReentryEvidence) {
				equivalence.UnsupportedDurableObservations = []UnsupportedCapability{{Kind: ObservationFileManifest, Reason: "runtime evidence does not expose source file manifests"}}
			},
			wantErr: "equivalence must remain narrowed by unsupported file_manifest and blob_set observations",
		},
		{
			name: "equivalence status equivalent",
			mutate: func(_ *BaseRuntimeMaterializationBridgeContract, equivalence *BaseRuntimeEquivalenceBoundaryContract, _ *BaseRuntimeEquivalenceReentryEvidence) {
				equivalence.RuntimeEquivalenceStatus = EquivalenceEquivalent
			},
			wantErr: "equivalence must remain narrowed",
		},
		{
			name: "equivalence already claimed",
			mutate: func(_ *BaseRuntimeMaterializationBridgeContract, equivalence *BaseRuntimeEquivalenceBoundaryContract, _ *BaseRuntimeEquivalenceReentryEvidence) {
				equivalence.RuntimeEquivalenceClaimed = true
			},
			wantErr: "equivalence must remain narrowed",
		},
		{
			name: "equivalence proof requirement drift",
			mutate: func(_ *BaseRuntimeMaterializationBridgeContract, equivalence *BaseRuntimeEquivalenceBoundaryContract, _ *BaseRuntimeEquivalenceReentryEvidence) {
				equivalence.DurableStateEquivalenceRequired = false
			},
			wantErr: "equivalence must preserve downstream proof requirements",
		},
		{
			name: "equivalence no mutation flag drift",
			mutate: func(_ *BaseRuntimeMaterializationBridgeContract, equivalence *BaseRuntimeEquivalenceBoundaryContract, _ *BaseRuntimeEquivalenceReentryEvidence) {
				equivalence.NoProductionMutation = false
			},
			wantErr: "equivalence must prove no VM lifecycle or production mutation",
		},
		{
			name: "equivalence claim drift",
			mutate: func(_ *BaseRuntimeMaterializationBridgeContract, equivalence *BaseRuntimeEquivalenceBoundaryContract, _ *BaseRuntimeEquivalenceReentryEvidence) {
				equivalence.PromotionClaimed = true
			},
			wantErr: "equivalence carries protected-surface or completion claims",
		},
		{
			name: "missing evidence bridge ref",
			mutate: func(_ *BaseRuntimeMaterializationBridgeContract, _ *BaseRuntimeEquivalenceBoundaryContract, evidence *BaseRuntimeEquivalenceReentryEvidence) {
				evidence.RuntimeMaterializationBridgeRef = "\t"
			},
			wantErr: "evidence refs are required",
		},
		{
			name: "missing evidence equivalence ref",
			mutate: func(_ *BaseRuntimeMaterializationBridgeContract, _ *BaseRuntimeEquivalenceBoundaryContract, evidence *BaseRuntimeEquivalenceReentryEvidence) {
				evidence.RuntimeEquivalenceBoundaryRef = " "
			},
			wantErr: "evidence refs are required",
		},
		{
			name: "missing evidence reentry review ref",
			mutate: func(_ *BaseRuntimeMaterializationBridgeContract, _ *BaseRuntimeEquivalenceBoundaryContract, evidence *BaseRuntimeEquivalenceReentryEvidence) {
				evidence.ReentryReviewRef = ""
			},
			wantErr: "evidence refs are required",
		},
		{
			name: "missing evidence rollback ref",
			mutate: func(_ *BaseRuntimeMaterializationBridgeContract, _ *BaseRuntimeEquivalenceBoundaryContract, evidence *BaseRuntimeEquivalenceReentryEvidence) {
				evidence.RollbackPlanRef = "  "
			},
			wantErr: "evidence refs are required",
		},
		{
			name: "missing evidence no VM lifecycle flag",
			mutate: func(_ *BaseRuntimeMaterializationBridgeContract, _ *BaseRuntimeEquivalenceBoundaryContract, evidence *BaseRuntimeEquivalenceReentryEvidence) {
				evidence.NoVMLifecycleMutation = false
			},
			wantErr: "evidence must prove no VM lifecycle",
		},
		{
			name: "missing evidence no durable computer flag",
			mutate: func(_ *BaseRuntimeMaterializationBridgeContract, _ *BaseRuntimeEquivalenceBoundaryContract, evidence *BaseRuntimeEquivalenceReentryEvidence) {
				evidence.NoDurableComputerMutation = false
			},
			wantErr: "evidence must prove no VM lifecycle",
		},
		{
			name: "missing evidence no deployed route flag",
			mutate: func(_ *BaseRuntimeMaterializationBridgeContract, _ *BaseRuntimeEquivalenceBoundaryContract, evidence *BaseRuntimeEquivalenceReentryEvidence) {
				evidence.NoDeployedRouteMutation = false
			},
			wantErr: "evidence must prove no VM lifecycle",
		},
		{
			name: "missing evidence no production flag",
			mutate: func(_ *BaseRuntimeMaterializationBridgeContract, _ *BaseRuntimeEquivalenceBoundaryContract, evidence *BaseRuntimeEquivalenceReentryEvidence) {
				evidence.NoProductionMutation = false
			},
			wantErr: "evidence must prove no VM lifecycle",
		},
		{
			name: "missing evidence no package publication flag",
			mutate: func(_ *BaseRuntimeMaterializationBridgeContract, _ *BaseRuntimeEquivalenceBoundaryContract, evidence *BaseRuntimeEquivalenceReentryEvidence) {
				evidence.NoPackagePublicationMutation = false
			},
			wantErr: "evidence must prove no VM lifecycle",
		},
		{
			name: "missing evidence no promotion flag",
			mutate: func(_ *BaseRuntimeMaterializationBridgeContract, _ *BaseRuntimeEquivalenceBoundaryContract, evidence *BaseRuntimeEquivalenceReentryEvidence) {
				evidence.NoPromotionMutation = false
			},
			wantErr: "evidence must prove no VM lifecycle",
		},
		{
			name: "missing evidence no run acceptance flag",
			mutate: func(_ *BaseRuntimeMaterializationBridgeContract, _ *BaseRuntimeEquivalenceBoundaryContract, evidence *BaseRuntimeEquivalenceReentryEvidence) {
				evidence.NoRunAcceptanceMutation = false
			},
			wantErr: "evidence must prove no VM lifecycle",
		},
		{
			name: "runtime behavior changed evidence claim",
			mutate: func(_ *BaseRuntimeMaterializationBridgeContract, _ *BaseRuntimeEquivalenceBoundaryContract, evidence *BaseRuntimeEquivalenceReentryEvidence) {
				evidence.RuntimeBehaviorChanged = true
			},
			wantErr: "evidence carries runtime",
		},
		{
			name: "durable computer mutation evidence claim",
			mutate: func(_ *BaseRuntimeMaterializationBridgeContract, _ *BaseRuntimeEquivalenceBoundaryContract, evidence *BaseRuntimeEquivalenceReentryEvidence) {
				evidence.DurableComputerStateMutated = true
			},
			wantErr: "evidence carries runtime",
		},
		{
			name: "deployed route registration evidence claim",
			mutate: func(_ *BaseRuntimeMaterializationBridgeContract, _ *BaseRuntimeEquivalenceBoundaryContract, evidence *BaseRuntimeEquivalenceReentryEvidence) {
				evidence.DeployedRouteRegistered = true
			},
			wantErr: "evidence carries runtime",
		},
		{
			name: "production auth touch evidence claim",
			mutate: func(_ *BaseRuntimeMaterializationBridgeContract, _ *BaseRuntimeEquivalenceBoundaryContract, evidence *BaseRuntimeEquivalenceReentryEvidence) {
				evidence.ProductionAuthTouched = true
			},
			wantErr: "evidence carries runtime",
		},
		{
			name: "production state mutation evidence claim",
			mutate: func(_ *BaseRuntimeMaterializationBridgeContract, _ *BaseRuntimeEquivalenceBoundaryContract, evidence *BaseRuntimeEquivalenceReentryEvidence) {
				evidence.ProductionStateMutated = true
			},
			wantErr: "evidence carries runtime",
		},
		{
			name: "package publication evidence claim",
			mutate: func(_ *BaseRuntimeMaterializationBridgeContract, _ *BaseRuntimeEquivalenceBoundaryContract, evidence *BaseRuntimeEquivalenceReentryEvidence) {
				evidence.PackagePublished = true
			},
			wantErr: "evidence carries runtime",
		},
		{
			name: "promotion execution evidence claim",
			mutate: func(_ *BaseRuntimeMaterializationBridgeContract, _ *BaseRuntimeEquivalenceBoundaryContract, evidence *BaseRuntimeEquivalenceReentryEvidence) {
				evidence.PromotionExecuted = true
			},
			wantErr: "evidence carries runtime",
		},
		{
			name: "VM lifecycle evidence touch",
			mutate: func(_ *BaseRuntimeMaterializationBridgeContract, _ *BaseRuntimeEquivalenceBoundaryContract, evidence *BaseRuntimeEquivalenceReentryEvidence) {
				evidence.VMLifecycleTouched = true
			},
			wantErr: "evidence carries runtime",
		},
		{
			name: "Firecracker boot evidence claim",
			mutate: func(_ *BaseRuntimeMaterializationBridgeContract, _ *BaseRuntimeEquivalenceBoundaryContract, evidence *BaseRuntimeEquivalenceReentryEvidence) {
				evidence.FirecrackerBootClaimed = true
			},
			wantErr: "evidence carries runtime",
		},
		{
			name: "run acceptance evidence touch",
			mutate: func(_ *BaseRuntimeMaterializationBridgeContract, _ *BaseRuntimeEquivalenceBoundaryContract, evidence *BaseRuntimeEquivalenceReentryEvidence) {
				evidence.RunAcceptanceRecordTouched = true
			},
			wantErr: "evidence carries runtime",
		},
		{
			name: "full substrate evidence claim",
			mutate: func(_ *BaseRuntimeMaterializationBridgeContract, _ *BaseRuntimeEquivalenceBoundaryContract, evidence *BaseRuntimeEquivalenceReentryEvidence) {
				evidence.FullSubstrateClaimed = true
			},
			wantErr: "evidence carries runtime",
		},
		{
			name: "completion evidence claim",
			mutate: func(_ *BaseRuntimeMaterializationBridgeContract, _ *BaseRuntimeEquivalenceBoundaryContract, evidence *BaseRuntimeEquivalenceReentryEvidence) {
				evidence.CompletionClaimed = true
			},
			wantErr: "evidence carries runtime",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bridge, equivalence, evidence := baseRuntimeEquivalenceReentryContractInputs(t)
			tc.mutate(&bridge, &equivalence, &evidence)

			contract, err := BuildBaseRuntimeEquivalenceReentryContract(bridge, equivalence, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildBaseRuntimeEquivalenceReentryContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func baseRuntimeEquivalenceReentryContractInputs(t *testing.T) (BaseRuntimeMaterializationBridgeContract, BaseRuntimeEquivalenceBoundaryContract, BaseRuntimeEquivalenceReentryEvidence) {
	t.Helper()

	probe, source, materializer, readinessEvidence := baseSourceMaterializerReadinessContractInputs(t)
	readiness, err := BuildBaseSourceMaterializerReadinessContract(probe, source, materializer, readinessEvidence)
	if err != nil {
		t.Fatalf("BuildBaseSourceMaterializerReadinessContract(): %v", err)
	}

	realization := mustMaterializeVMManagerBoundary(t, "base-runtime-equivalence-reentry-vmmanager", readiness.Version, vmManagerBoundaryPath(), VMManagerCapabilityManifest("base-runtime-equivalence-reentry-vmmanager"))
	runtimeEvidence := BaseRuntimeMaterializationCeremonyEvidence{
		SourceProvenanceReadinessRef: " " + readiness.SourceProvenanceReadinessRef + " ",
		RealizationEvidenceRef:       " vmmanager:base-runtime-equivalence-reentry-pass-122 ",
		MaterializationCommandRef:    " go-test:internal/computerversion/base-runtime-equivalence-reentry ",
		NoVMLifecycleMutation:        true,
		NoProductionMutation:         true,
	}
	runtime, err := BuildBaseRuntimeMaterializationCeremonyContract(source, realization, runtimeEvidence)
	if err != nil {
		t.Fatalf("BuildBaseRuntimeMaterializationCeremonyContract(): %v", err)
	}

	bridgeEvidence := BaseRuntimeMaterializationBridgeEvidence{
		SourceMaterializerReadinessRef: " source-materializer-readiness:base-pass-122 ",
		RuntimeMaterializationRef:      " runtime-materialization:base-pass-122 ",
		RuntimeEvidenceReviewRef:       " runtime-evidence-review:base-pass-122 ",
		BridgeReviewRef:                " bridge-review:base-runtime-materialization-pass-122 ",
		RollbackPlanRef:                " rollback-plan:base-runtime-materialization-bridge-pass-122 ",
		NoVMLifecycleMutation:          true,
		NoDurableComputerMutation:      true,
		NoDeployedRouteMutation:        true,
		NoProductionMutation:           true,
		NoPackagePublicationMutation:   true,
		NoPromotionMutation:            true,
		NoRunAcceptanceMutation:        true,
	}
	bridge, err := BuildBaseRuntimeMaterializationBridgeContract(readiness, runtime, bridgeEvidence)
	if err != nil {
		t.Fatalf("BuildBaseRuntimeMaterializationBridgeContract(): %v", err)
	}

	boundaryResult := EquivalenceResult{
		Status: EquivalenceNarrowed,
		Unsupported: []UnsupportedCapability{
			{Kind: ObservationFileManifest, Reason: "vmmanager runtime evidence does not expose source file manifests"},
			{Kind: ObservationBlobSet, Reason: "vmmanager runtime evidence does not expose source blobs"},
		},
	}
	boundaryEvidence := BaseRuntimeEquivalenceBoundaryEvidence{
		RuntimeMaterializationCeremonyRef: " " + bridge.RuntimeMaterializationRef + " ",
		RuntimeEquivalenceEvidenceRef:     " equivalence:runtime-equivalence-reentry-boundary-pass-122 ",
		NoVMLifecycleMutation:             true,
		NoProductionMutation:              true,
	}
	equivalence, err := BuildBaseRuntimeEquivalenceBoundaryContract(source, runtime, boundaryResult, boundaryEvidence)
	if err != nil {
		t.Fatalf("BuildBaseRuntimeEquivalenceBoundaryContract(): %v", err)
	}

	evidence := BaseRuntimeEquivalenceReentryEvidence{
		RuntimeMaterializationBridgeRef: " contract:base-runtime-materialization-bridge-pass-122 ",
		RuntimeEquivalenceBoundaryRef:   " contract:base-runtime-equivalence-boundary-pass-122 ",
		ReentryReviewRef:                " reentry-review:base-runtime-equivalence-pass-122 ",
		RollbackPlanRef:                 " rollback-plan:base-runtime-equivalence-reentry-pass-122 ",
		NoVMLifecycleMutation:           true,
		NoDurableComputerMutation:       true,
		NoDeployedRouteMutation:         true,
		NoProductionMutation:            true,
		NoPackagePublicationMutation:    true,
		NoPromotionMutation:             true,
		NoRunAcceptanceMutation:         true,
	}
	return bridge, equivalence, evidence
}
