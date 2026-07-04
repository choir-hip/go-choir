package computerversion

import (
	"strings"
	"testing"
)

func TestBuildBaseRuntimeDurableProofGapContractRecordsOpenRuntimeDurableProofGap(t *testing.T) {
	reentry, summary, evidence := baseRuntimeDurableProofGapContractInputs(t)

	contract, err := BuildBaseRuntimeDurableProofGapContract(reentry, summary, evidence)
	if err != nil {
		t.Fatalf("BuildBaseRuntimeDurableProofGapContract(): %v", err)
	}

	if contract.Kind != BaseRuntimeDurableProofGapContractKind {
		t.Fatalf("kind = %q, want %q", contract.Kind, BaseRuntimeDurableProofGapContractKind)
	}
	if contract.Version != reentry.Version || contract.Version != summary.Version {
		t.Fatalf("version = %#v, want shared reentry/summary version %#v/%#v", contract.Version, reentry.Version, summary.Version)
	}
	if contract.Boundary != BaseRuntimeDurableProofGapBoundary {
		t.Fatalf("boundary = %q, want %q", contract.Boundary, BaseRuntimeDurableProofGapBoundary)
	}
	if contract.Scope != BaseRuntimeDurableProofGapScope {
		t.Fatalf("scope = %q, want %q", contract.Scope, BaseRuntimeDurableProofGapScope)
	}
	if ArtifactProgramRef(contract.TypedArtifactProgramRef) != reentry.Version.ArtifactProgramRef || contract.TypedArtifactProgramRef != strings.TrimSpace(reentry.TypedArtifactProgramRef) {
		t.Fatalf("typed artifact program ref = %q, want trimmed reentry artifact program %q", contract.TypedArtifactProgramRef, reentry.Version.ArtifactProgramRef)
	}
	if contract.RuntimeEquivalenceReentryRef != strings.TrimSpace(evidence.RuntimeEquivalenceReentryRef) || contract.LocalSubstrateSummaryRef != strings.TrimSpace(evidence.LocalSubstrateSummaryRef) || contract.GapReviewRef != strings.TrimSpace(evidence.GapReviewRef) || contract.RuntimeFileBlobPlanRef != strings.TrimSpace(evidence.RuntimeFileBlobPlanRef) || contract.RuntimeEquivalenceRetryRef != strings.TrimSpace(evidence.RuntimeEquivalenceRetryRef) || contract.RollbackPlanRef != strings.TrimSpace(evidence.RollbackPlanRef) {
		t.Fatalf("gap evidence refs = reentry %q summary %q review %q plan %q retry %q rollback %q, want trimmed refs from %#v", contract.RuntimeEquivalenceReentryRef, contract.LocalSubstrateSummaryRef, contract.GapReviewRef, contract.RuntimeFileBlobPlanRef, contract.RuntimeEquivalenceRetryRef, contract.RollbackPlanRef, evidence)
	}
	if contract.SourceMaterializerReadinessRef != reentry.SourceMaterializerReadinessRef || contract.RuntimeMaterializationRef != reentry.RuntimeMaterializationRef || contract.RuntimeEvidenceReviewRef != reentry.RuntimeEvidenceReviewRef || contract.SourceProvenanceReadinessRef != reentry.SourceProvenanceReadinessRef || contract.RealizationEvidenceRef != reentry.RealizationEvidenceRef {
		t.Fatalf("runtime refs = source materializer %q materialization %q review %q source provenance %q realization %q, want reentry refs %#v", contract.SourceMaterializerReadinessRef, contract.RuntimeMaterializationRef, contract.RuntimeEvidenceReviewRef, contract.SourceProvenanceReadinessRef, contract.RealizationEvidenceRef, reentry)
	}
	if contract.RuntimeMaterializer != reentry.Materializer || contract.RuntimeSubstrate != reentry.Substrate {
		t.Fatalf("runtime identity = %q/%q, want reentry %q/%q", contract.RuntimeMaterializer, contract.RuntimeSubstrate, reentry.Materializer, reentry.Substrate)
	}
	if contract.LocalCurrentMaterializer != summary.CurrentMaterializer || contract.LocalCurrentSubstrate != summary.CurrentSubstrate || contract.LocalProjectionMaterializer != summary.ProjectionMaterializer || contract.LocalProjectionSubstrate != summary.ProjectionSubstrate {
		t.Fatalf("local identities = current %q/%q projection %q/%q, want summary %#v", contract.LocalCurrentMaterializer, contract.LocalCurrentSubstrate, contract.LocalProjectionMaterializer, contract.LocalProjectionSubstrate, summary)
	}
	assertObservationBundleKinds(t, contract.RuntimeRequiredObservations, []ObservationKind{ObservationVMStateManifest})
	assertObservationBundleKinds(t, contract.LocalRequiredObservations, []ObservationKind{ObservationBlobSet, ObservationFileManifest})
	assertUnsupportedDurableObservations(t, contract.UnsupportedDurableObservations, []ObservationKind{ObservationFileManifest, ObservationBlobSet})
	assertBaseRuntimeDurableProofGapRemainingGaps(t, contract.RemainingGaps)
	if contract.GapStatus != BaseRuntimeDurableProofGapStatusOpen {
		t.Fatalf("gap status = %q, want %q", contract.GapStatus, BaseRuntimeDurableProofGapStatusOpen)
	}
	if !contract.RuntimeEquivalenceNarrowed || contract.RuntimeEquivalenceClaimed {
		t.Fatalf("runtime equivalence = narrowed %v claimed %v, want narrowed true and claimed false", contract.RuntimeEquivalenceNarrowed, contract.RuntimeEquivalenceClaimed)
	}
	if !contract.LocalFileBlobProofSummarized {
		t.Fatalf("local file/blob proof summarized = false, want true")
	}
	if !contract.RuntimeFileBlobExtractionRequired || !contract.RuntimeEquivalenceRetryRequired || !contract.DurableStateEquivalenceRequired || !contract.StagingProofRequired || !contract.PromotionProofRequired || !contract.PackagePublicationProofRequired || !contract.RunAcceptanceProofRequired || !contract.FullSubstrateProofRequired {
		t.Fatalf("gap contract must preserve runtime file/blob, retry, durable-state, staging, promotion, package, run-acceptance, and full-substrate proof requirements: %#v", contract)
	}
	if contract.VMLifecycleMutationAllowed || contract.DurableComputerMutationAllowed || contract.DeployedRouteRegistrationAllowed || contract.ProductionMutationAllowed || contract.PackagePublicationAllowed || contract.PromotionAllowed || contract.RunAcceptanceSynthesisAllowed {
		t.Fatalf("gap contract must deny VM lifecycle, durable computer, deployed route, production, package, promotion, and run acceptance authority: %#v", contract)
	}
	if !contract.NoRuntimeMaterialization || !contract.NoDurableComputerMutation || !contract.NoDeployedRouteMutation || !contract.NoProductionMutation || !contract.NoPackagePublicationMutation || !contract.NoPromotionMutation || !contract.NoRunAcceptanceMutation {
		t.Fatalf("gap contract must carry no-runtime/no-durable/no-route/no-production/no-package/no-promotion/no-run flags: %#v", contract)
	}
	if contract.RuntimeBehaviorChanged || contract.DurableComputerStateMutated || contract.DeployedRouteRegistered || contract.ProductionAuthTouched || contract.ProductionStateMutated || contract.PackagePublished || contract.PromotionExecuted || contract.VMLifecycleTouched || contract.FirecrackerBootClaimed || contract.StagingClaimed || contract.RunAcceptanceRecordTouched || contract.FullSubstrateClaimed || contract.CompletionClaimed {
		t.Fatalf("gap contract must not claim runtime, deployed-route, VM lifecycle, production, full-substrate, package, promotion, run-acceptance, staging, or completion effects: %#v", contract)
	}
}

func TestBuildBaseRuntimeDurableProofGapContractRejectsInvalidInputs(t *testing.T) {
	foreignVersion := ComputerVersion{CodeRef: "git:foreign-base-runtime-durable-proof-gap", ArtifactProgramRef: "base-journal:owner/main@foreign-base-runtime-durable-proof-gap"}
	for _, tc := range []struct {
		name    string
		mutate  func(*BaseRuntimeEquivalenceReentryContract, *BaseLocalSubstrateProofSummaryContract, *BaseRuntimeDurableProofGapEvidence)
		wantErr string
	}{
		{
			name: "reentry wrong kind",
			mutate: func(reentry *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				reentry.Kind = BaseRuntimeDurableProofGapContractKind
			},
			wantErr: "reentry kind",
		},
		{
			name: "reentry wrong boundary",
			mutate: func(reentry *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				reentry.Boundary = BaseRuntimeDurableProofGapBoundary
			},
			wantErr: "reentry boundary",
		},
		{
			name: "reentry wrong scope",
			mutate: func(reentry *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				reentry.Scope = BaseRuntimeDurableProofGapScope
			},
			wantErr: "reentry scope",
		},
		{
			name: "reentry invalid version",
			mutate: func(reentry *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				reentry.Version.CodeRef = "  "
			},
			wantErr: "reentry version or artifact ref is invalid",
		},
		{
			name: "reentry artifact ref drift",
			mutate: func(reentry *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				reentry.TypedArtifactProgramRef = string(foreignVersion.ArtifactProgramRef)
			},
			wantErr: "reentry version or artifact ref is invalid",
		},
		{
			name: "reentry missing runtime materialization bridge ref",
			mutate: func(reentry *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				reentry.RuntimeMaterializationBridgeRef = "  "
			},
			wantErr: "reentry refs are required",
		},
		{
			name: "reentry missing source provenance ref",
			mutate: func(reentry *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				reentry.SourceProvenanceReadinessRef = "\t"
			},
			wantErr: "reentry refs are required",
		},
		{
			name: "reentry status drift",
			mutate: func(reentry *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				reentry.ReentryStatus = "runtime_equivalence_reentry_complete"
			},
			wantErr: "reentry must remain narrowed",
		},
		{
			name: "reentry runtime evidence not accepted",
			mutate: func(reentry *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				reentry.RuntimeEvidenceAccepted = false
			},
			wantErr: "reentry must remain narrowed",
		},
		{
			name: "reentry runtime equivalence not narrowed",
			mutate: func(reentry *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				reentry.RuntimeEquivalenceNarrowed = false
			},
			wantErr: "reentry must remain narrowed",
		},
		{
			name: "reentry runtime equivalence claimed",
			mutate: func(reentry *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				reentry.RuntimeEquivalenceClaimed = true
			},
			wantErr: "reentry must remain narrowed",
		},
		{
			name: "reentry source observations missing file manifest",
			mutate: func(reentry *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				reentry.SourceRequiredObservations = []ObservationKind{ObservationBlobSet}
			},
			wantErr: "reentry observations are incomplete",
		},
		{
			name: "reentry runtime observations missing VM state",
			mutate: func(reentry *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				reentry.RuntimeRequiredObservations = nil
			},
			wantErr: "reentry observations are incomplete",
		},
		{
			name: "reentry unsupported durable file manifest missing",
			mutate: func(reentry *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				reentry.UnsupportedDurableObservations = []UnsupportedCapability{{Kind: ObservationBlobSet, Reason: "runtime evidence does not expose source blobs"}}
			},
			wantErr: "reentry must retain unsupported durable observations",
		},
		{
			name: "reentry unsupported durable blob set missing",
			mutate: func(reentry *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				reentry.UnsupportedDurableObservations = []UnsupportedCapability{{Kind: ObservationFileManifest, Reason: "runtime evidence does not expose source file manifests"}}
			},
			wantErr: "reentry must retain unsupported durable observations",
		},
		{
			name: "reentry durable-state proof requirement missing",
			mutate: func(reentry *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				reentry.DurableStateEquivalenceRequired = false
			},
			wantErr: "reentry must preserve downstream proof requirements",
		},
		{
			name: "reentry staging proof requirement missing",
			mutate: func(reentry *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				reentry.StagingProofRequired = false
			},
			wantErr: "reentry must preserve downstream proof requirements",
		},
		{
			name: "reentry promotion proof requirement missing",
			mutate: func(reentry *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				reentry.PromotionProofRequired = false
			},
			wantErr: "reentry must preserve downstream proof requirements",
		},
		{
			name: "reentry package proof requirement missing",
			mutate: func(reentry *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				reentry.PackagePublicationRequired = false
			},
			wantErr: "reentry must preserve downstream proof requirements",
		},
		{
			name: "reentry run acceptance proof requirement missing",
			mutate: func(reentry *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				reentry.RunAcceptanceProofRequired = false
			},
			wantErr: "reentry must preserve downstream proof requirements",
		},
		{
			name: "reentry full substrate proof requirement missing",
			mutate: func(reentry *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				reentry.FullSubstrateProofRequired = false
			},
			wantErr: "reentry must preserve downstream proof requirements",
		},
		{
			name: "reentry VM lifecycle authority allowed",
			mutate: func(reentry *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				reentry.VMLifecycleMutationAllowed = true
			},
			wantErr: "reentry allows downstream execution",
		},
		{
			name: "reentry durable computer authority allowed",
			mutate: func(reentry *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				reentry.DurableComputerMutationAllowed = true
			},
			wantErr: "reentry allows downstream execution",
		},
		{
			name: "reentry deployed route authority allowed",
			mutate: func(reentry *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				reentry.DeployedRouteRegistrationAllowed = true
			},
			wantErr: "reentry allows downstream execution",
		},
		{
			name: "reentry production authority allowed",
			mutate: func(reentry *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				reentry.ProductionMutationAllowed = true
			},
			wantErr: "reentry allows downstream execution",
		},
		{
			name: "reentry package authority allowed",
			mutate: func(reentry *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				reentry.PackagePublicationAllowed = true
			},
			wantErr: "reentry allows downstream execution",
		},
		{
			name: "reentry promotion authority allowed",
			mutate: func(reentry *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				reentry.PromotionAllowed = true
			},
			wantErr: "reentry allows downstream execution",
		},
		{
			name: "reentry run acceptance authority allowed",
			mutate: func(reentry *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				reentry.RunAcceptanceSynthesisAllowed = true
			},
			wantErr: "reentry allows downstream execution",
		},
		{
			name: "reentry no VM lifecycle flag missing",
			mutate: func(reentry *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				reentry.NoVMLifecycleMutation = false
			},
			wantErr: "reentry must prove no mutation",
		},
		{
			name: "reentry no durable flag missing",
			mutate: func(reentry *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				reentry.NoDurableComputerMutation = false
			},
			wantErr: "reentry must prove no mutation",
		},
		{
			name: "reentry no deployed route flag missing",
			mutate: func(reentry *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				reentry.NoDeployedRouteMutation = false
			},
			wantErr: "reentry must prove no mutation",
		},
		{
			name: "reentry no production flag missing",
			mutate: func(reentry *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				reentry.NoProductionMutation = false
			},
			wantErr: "reentry must prove no mutation",
		},
		{
			name: "reentry no package flag missing",
			mutate: func(reentry *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				reentry.NoPackagePublicationMutation = false
			},
			wantErr: "reentry must prove no mutation",
		},
		{
			name: "reentry no promotion flag missing",
			mutate: func(reentry *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				reentry.NoPromotionMutation = false
			},
			wantErr: "reentry must prove no mutation",
		},
		{
			name: "reentry no run acceptance flag missing",
			mutate: func(reentry *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				reentry.NoRunAcceptanceMutation = false
			},
			wantErr: "reentry must prove no mutation",
		},
		{
			name: "reentry runtime behavior changed",
			mutate: func(reentry *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				reentry.RuntimeBehaviorChanged = true
			},
			wantErr: "reentry carries protected-surface or completion claims",
		},
		{
			name: "reentry durable computer mutation",
			mutate: func(reentry *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				reentry.DurableComputerStateMutated = true
			},
			wantErr: "reentry carries protected-surface or completion claims",
		},
		{
			name: "reentry deployed route registration",
			mutate: func(reentry *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				reentry.DeployedRouteRegistered = true
			},
			wantErr: "reentry carries protected-surface or completion claims",
		},
		{
			name: "reentry production state touch",
			mutate: func(reentry *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				reentry.ProductionStateMutated = true
			},
			wantErr: "reentry carries protected-surface or completion claims",
		},
		{
			name: "reentry package publication",
			mutate: func(reentry *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				reentry.PackagePublished = true
			},
			wantErr: "reentry carries protected-surface or completion claims",
		},
		{
			name: "reentry promotion execution",
			mutate: func(reentry *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				reentry.PromotionExecuted = true
			},
			wantErr: "reentry carries protected-surface or completion claims",
		},
		{
			name: "reentry VM lifecycle touch",
			mutate: func(reentry *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				reentry.VMLifecycleTouched = true
			},
			wantErr: "reentry carries protected-surface or completion claims",
		},
		{
			name: "reentry Firecracker boot claim",
			mutate: func(reentry *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				reentry.FirecrackerBootClaimed = true
			},
			wantErr: "reentry carries protected-surface or completion claims",
		},
		{
			name: "reentry run acceptance touch",
			mutate: func(reentry *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				reentry.RunAcceptanceRecordTouched = true
			},
			wantErr: "reentry carries protected-surface or completion claims",
		},
		{
			name: "reentry full-substrate claim",
			mutate: func(reentry *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				reentry.FullSubstrateClaimed = true
			},
			wantErr: "reentry carries protected-surface or completion claims",
		},
		{
			name: "reentry completion claim",
			mutate: func(reentry *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				reentry.CompletionClaimed = true
			},
			wantErr: "reentry carries protected-surface or completion claims",
		},
		{
			name: "summary wrong kind",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, summary *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				summary.Kind = BaseRuntimeDurableProofGapContractKind
			},
			wantErr: "summary kind",
		},
		{
			name: "summary wrong boundary",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, summary *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				summary.Boundary = BaseRuntimeDurableProofGapBoundary
			},
			wantErr: "summary boundary",
		},
		{
			name: "summary wrong scope",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, summary *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				summary.Scope = BaseRuntimeDurableProofGapScope
			},
			wantErr: "summary scope",
		},
		{
			name: "summary version drift",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, summary *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				summary.Version = foreignVersion
			},
			wantErr: "summary version does not match reentry",
		},
		{
			name: "summary artifact version drift",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, summary *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				summary.Version.ArtifactProgramRef = foreignVersion.ArtifactProgramRef
			},
			wantErr: "summary version does not match reentry",
		},
		{
			name: "summary claim scope drift",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, summary *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				summary.ClaimScope = BaseRuntimeDurableProofGapScope
			},
			wantErr: "summary must prove only local file/blob substrate equivalence",
		},
		{
			name: "summary non-equivalent status",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, summary *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				summary.SubstrateEquivalenceStatus = EquivalenceNarrowed
			},
			wantErr: "summary must prove only local file/blob substrate equivalence",
		},
		{
			name: "summary reentry not allowed",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, summary *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				summary.ReentryAllowed = false
			},
			wantErr: "summary must prove only local file/blob substrate equivalence",
		},
		{
			name: "summary local proof not summarized",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, summary *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				summary.LocalFileBlobProofSummarized = false
			},
			wantErr: "summary must prove only local file/blob substrate equivalence",
		},
		{
			name: "summary missing substrate equivalence ref",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, summary *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				summary.SubstrateEquivalenceContractRef = "  "
			},
			wantErr: "summary refs are required",
		},
		{
			name: "summary missing current materializer ref",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, summary *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				summary.CurrentMaterializer = "\t"
			},
			wantErr: "summary refs are required",
		},
		{
			name: "summary observations missing blob set",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, summary *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				summary.RequiredObservations = []ObservationKind{ObservationFileManifest}
			},
			wantErr: "summary must include file_manifest and blob_set",
		},
		{
			name: "summary missing runtime remaining gap",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, summary *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				summary.RemainingGaps = []string{BaseLocalSubstrateProofSummaryRemainingStagingProof, BaseLocalSubstrateProofSummaryRemainingPromotionProof}
			},
			wantErr: "summary must preserve runtime, staging, and promotion gaps",
		},
		{
			name: "summary runtime proof requirement missing",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, summary *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				summary.RuntimeSubstrateProofRequired = false
			},
			wantErr: "summary must preserve runtime, staging, and promotion gaps",
		},
		{
			name: "summary staging proof requirement missing",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, summary *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				summary.StagingProofRequired = false
			},
			wantErr: "summary must preserve runtime, staging, and promotion gaps",
		},
		{
			name: "summary promotion proof requirement missing",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, summary *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				summary.PromotionProofRequired = false
			},
			wantErr: "summary must preserve runtime, staging, and promotion gaps",
		},
		{
			name: "summary no runtime flag missing",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, summary *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				summary.NoRuntimeMaterialization = false
			},
			wantErr: "summary must be local no-runtime no-mutation evidence",
		},
		{
			name: "summary no opaque data flag missing",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, summary *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				summary.NoOpaqueDataImageDependency = false
			},
			wantErr: "summary must be local no-runtime no-mutation evidence",
		},
		{
			name: "summary no mutation flag missing",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, summary *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				summary.NoMutation = false
			},
			wantErr: "summary must be local no-runtime no-mutation evidence",
		},
		{
			name: "summary runtime behavior changed",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, summary *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				summary.RuntimeBehaviorChanged = true
			},
			wantErr: "summary carries protected-surface or completion claims",
		},
		{
			name: "summary deployed route registration",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, summary *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				summary.DeployedRouteRegistered = true
			},
			wantErr: "summary carries protected-surface or completion claims",
		},
		{
			name: "summary production auth touch",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, summary *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				summary.ProductionAuthTouched = true
			},
			wantErr: "summary carries protected-surface or completion claims",
		},
		{
			name: "summary staging claim",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, summary *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				summary.StagingClaimed = true
			},
			wantErr: "summary carries protected-surface or completion claims",
		},
		{
			name: "summary promotion claim",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, summary *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				summary.PromotionClaimed = true
			},
			wantErr: "summary carries protected-surface or completion claims",
		},
		{
			name: "summary VM lifecycle touch",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, summary *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				summary.VMLifecycleTouched = true
			},
			wantErr: "summary carries protected-surface or completion claims",
		},
		{
			name: "summary Firecracker boot claim",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, summary *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				summary.FirecrackerBootClaimed = true
			},
			wantErr: "summary carries protected-surface or completion claims",
		},
		{
			name: "summary run acceptance touch",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, summary *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				summary.RunAcceptanceRecordTouched = true
			},
			wantErr: "summary carries protected-surface or completion claims",
		},
		{
			name: "summary full-substrate claim",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, summary *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				summary.FullSubstrateIndependenceClaim = true
			},
			wantErr: "summary carries protected-surface or completion claims",
		},
		{
			name: "summary package publication claim",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, summary *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				summary.PackagePublicationClaimed = true
			},
			wantErr: "summary carries protected-surface or completion claims",
		},
		{
			name: "summary completion claim",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, summary *BaseLocalSubstrateProofSummaryContract, _ *BaseRuntimeDurableProofGapEvidence) {
				summary.CompletionClaimed = true
			},
			wantErr: "summary carries protected-surface or completion claims",
		},
		{
			name: "missing evidence runtime reentry ref",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, evidence *BaseRuntimeDurableProofGapEvidence) {
				evidence.RuntimeEquivalenceReentryRef = "  "
			},
			wantErr: "evidence refs are required",
		},
		{
			name: "missing evidence local summary ref",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, evidence *BaseRuntimeDurableProofGapEvidence) {
				evidence.LocalSubstrateSummaryRef = "\t"
			},
			wantErr: "evidence refs are required",
		},
		{
			name: "missing evidence gap review ref",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, evidence *BaseRuntimeDurableProofGapEvidence) {
				evidence.GapReviewRef = ""
			},
			wantErr: "evidence refs are required",
		},
		{
			name: "missing evidence runtime file blob plan ref",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, evidence *BaseRuntimeDurableProofGapEvidence) {
				evidence.RuntimeFileBlobPlanRef = "  "
			},
			wantErr: "evidence refs are required",
		},
		{
			name: "missing evidence runtime equivalence retry ref",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, evidence *BaseRuntimeDurableProofGapEvidence) {
				evidence.RuntimeEquivalenceRetryRef = "  "
			},
			wantErr: "evidence refs are required",
		},
		{
			name: "missing evidence rollback ref",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, evidence *BaseRuntimeDurableProofGapEvidence) {
				evidence.RollbackPlanRef = "  "
			},
			wantErr: "evidence refs are required",
		},
		{
			name: "missing runtime file blob extraction gap",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, evidence *BaseRuntimeDurableProofGapEvidence) {
				evidence.RemainingGaps = removeBaseRuntimeDurableProofGap(evidence.RemainingGaps, BaseRuntimeDurableProofGapRuntimeFileBlobExtraction)
			},
			wantErr: "remaining gaps must require runtime file/blob extraction",
		},
		{
			name: "missing runtime equivalence retry gap",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, evidence *BaseRuntimeDurableProofGapEvidence) {
				evidence.RemainingGaps = removeBaseRuntimeDurableProofGap(evidence.RemainingGaps, BaseRuntimeDurableProofGapRuntimeEquivalenceRetry)
			},
			wantErr: "remaining gaps must require runtime file/blob extraction",
		},
		{
			name: "missing staging gap",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, evidence *BaseRuntimeDurableProofGapEvidence) {
				evidence.RemainingGaps = removeBaseRuntimeDurableProofGap(evidence.RemainingGaps, BaseRuntimeDurableProofGapStagingProof)
			},
			wantErr: "remaining gaps must require runtime file/blob extraction",
		},
		{
			name: "missing promotion gap",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, evidence *BaseRuntimeDurableProofGapEvidence) {
				evidence.RemainingGaps = removeBaseRuntimeDurableProofGap(evidence.RemainingGaps, BaseRuntimeDurableProofGapPromotionProof)
			},
			wantErr: "remaining gaps must require runtime file/blob extraction",
		},
		{
			name: "missing package gap",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, evidence *BaseRuntimeDurableProofGapEvidence) {
				evidence.RemainingGaps = removeBaseRuntimeDurableProofGap(evidence.RemainingGaps, BaseRuntimeDurableProofGapPackagePublicationProof)
			},
			wantErr: "remaining gaps must require runtime file/blob extraction",
		},
		{
			name: "missing run acceptance gap",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, evidence *BaseRuntimeDurableProofGapEvidence) {
				evidence.RemainingGaps = removeBaseRuntimeDurableProofGap(evidence.RemainingGaps, BaseRuntimeDurableProofGapRunAcceptanceProof)
			},
			wantErr: "remaining gaps must require runtime file/blob extraction",
		},
		{
			name: "missing full substrate gap",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, evidence *BaseRuntimeDurableProofGapEvidence) {
				evidence.RemainingGaps = removeBaseRuntimeDurableProofGap(evidence.RemainingGaps, BaseRuntimeDurableProofGapFullSubstrateProof)
			},
			wantErr: "remaining gaps must require runtime file/blob extraction",
		},
		{
			name: "evidence no runtime flag missing",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, evidence *BaseRuntimeDurableProofGapEvidence) {
				evidence.NoRuntimeMaterialization = false
			},
			wantErr: "evidence must prove no runtime",
		},
		{
			name: "evidence no durable computer flag missing",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, evidence *BaseRuntimeDurableProofGapEvidence) {
				evidence.NoDurableComputerMutation = false
			},
			wantErr: "evidence must prove no runtime",
		},
		{
			name: "evidence no deployed route flag missing",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, evidence *BaseRuntimeDurableProofGapEvidence) {
				evidence.NoDeployedRouteMutation = false
			},
			wantErr: "evidence must prove no runtime",
		},
		{
			name: "evidence no production flag missing",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, evidence *BaseRuntimeDurableProofGapEvidence) {
				evidence.NoProductionMutation = false
			},
			wantErr: "evidence must prove no runtime",
		},
		{
			name: "evidence no package flag missing",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, evidence *BaseRuntimeDurableProofGapEvidence) {
				evidence.NoPackagePublicationMutation = false
			},
			wantErr: "evidence must prove no runtime",
		},
		{
			name: "evidence no promotion flag missing",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, evidence *BaseRuntimeDurableProofGapEvidence) {
				evidence.NoPromotionMutation = false
			},
			wantErr: "evidence must prove no runtime",
		},
		{
			name: "evidence no run acceptance flag missing",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, evidence *BaseRuntimeDurableProofGapEvidence) {
				evidence.NoRunAcceptanceMutation = false
			},
			wantErr: "evidence must prove no runtime",
		},
		{
			name: "evidence runtime behavior changed",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, evidence *BaseRuntimeDurableProofGapEvidence) {
				evidence.RuntimeBehaviorChanged = true
			},
			wantErr: "evidence carries runtime-equivalence",
		},
		{
			name: "evidence runtime equivalence claimed",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, evidence *BaseRuntimeDurableProofGapEvidence) {
				evidence.RuntimeEquivalenceClaimed = true
			},
			wantErr: "evidence carries runtime-equivalence",
		},
		{
			name: "evidence durable computer mutation",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, evidence *BaseRuntimeDurableProofGapEvidence) {
				evidence.DurableComputerStateMutated = true
			},
			wantErr: "evidence carries runtime-equivalence",
		},
		{
			name: "evidence deployed route registration",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, evidence *BaseRuntimeDurableProofGapEvidence) {
				evidence.DeployedRouteRegistered = true
			},
			wantErr: "evidence carries runtime-equivalence",
		},
		{
			name: "evidence production auth touch",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, evidence *BaseRuntimeDurableProofGapEvidence) {
				evidence.ProductionAuthTouched = true
			},
			wantErr: "evidence carries runtime-equivalence",
		},
		{
			name: "evidence production state touch",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, evidence *BaseRuntimeDurableProofGapEvidence) {
				evidence.ProductionStateMutated = true
			},
			wantErr: "evidence carries runtime-equivalence",
		},
		{
			name: "evidence package publication",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, evidence *BaseRuntimeDurableProofGapEvidence) {
				evidence.PackagePublished = true
			},
			wantErr: "evidence carries runtime-equivalence",
		},
		{
			name: "evidence promotion execution",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, evidence *BaseRuntimeDurableProofGapEvidence) {
				evidence.PromotionExecuted = true
			},
			wantErr: "evidence carries runtime-equivalence",
		},
		{
			name: "evidence VM lifecycle touch",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, evidence *BaseRuntimeDurableProofGapEvidence) {
				evidence.VMLifecycleTouched = true
			},
			wantErr: "evidence carries runtime-equivalence",
		},
		{
			name: "evidence Firecracker boot claim",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, evidence *BaseRuntimeDurableProofGapEvidence) {
				evidence.FirecrackerBootClaimed = true
			},
			wantErr: "evidence carries runtime-equivalence",
		},
		{
			name: "evidence staging claim",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, evidence *BaseRuntimeDurableProofGapEvidence) {
				evidence.StagingClaimed = true
			},
			wantErr: "evidence carries runtime-equivalence",
		},
		{
			name: "evidence run acceptance touch",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, evidence *BaseRuntimeDurableProofGapEvidence) {
				evidence.RunAcceptanceRecordTouched = true
			},
			wantErr: "evidence carries runtime-equivalence",
		},
		{
			name: "evidence full-substrate claim",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, evidence *BaseRuntimeDurableProofGapEvidence) {
				evidence.FullSubstrateClaimed = true
			},
			wantErr: "evidence carries runtime-equivalence",
		},
		{
			name: "evidence completion claim",
			mutate: func(_ *BaseRuntimeEquivalenceReentryContract, _ *BaseLocalSubstrateProofSummaryContract, evidence *BaseRuntimeDurableProofGapEvidence) {
				evidence.CompletionClaimed = true
			},
			wantErr: "evidence carries runtime-equivalence",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reentry, summary, evidence := baseRuntimeDurableProofGapContractInputs(t)
			tc.mutate(&reentry, &summary, &evidence)

			contract, err := BuildBaseRuntimeDurableProofGapContract(reentry, summary, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildBaseRuntimeDurableProofGapContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func baseRuntimeDurableProofGapContractInputs(t *testing.T) (BaseRuntimeEquivalenceReentryContract, BaseLocalSubstrateProofSummaryContract, BaseRuntimeDurableProofGapEvidence) {
	t.Helper()

	bridge, equivalence, reentryEvidence := baseRuntimeEquivalenceReentryContractInputs(t)
	reentry, err := BuildBaseRuntimeEquivalenceReentryContract(bridge, equivalence, reentryEvidence)
	if err != nil {
		t.Fatalf("BuildBaseRuntimeEquivalenceReentryContract(): %v", err)
	}

	summary := baseRuntimeDurableProofGapAlignedLocalSummary(t, reentry.Version)
	evidence := BaseRuntimeDurableProofGapEvidence{
		RuntimeEquivalenceReentryRef: " contract:base-runtime-equivalence-reentry-pass-123 ",
		LocalSubstrateSummaryRef:     "\tcontract:base-local-substrate-proof-summary-pass-123\t",
		GapReviewRef:                 " review:base-runtime-durable-proof-gap-pass-123 ",
		RuntimeFileBlobPlanRef:       " plan:base-runtime-file-blob-extraction-pass-123 ",
		RuntimeEquivalenceRetryRef:   " retry:base-runtime-equivalence-pass-123 ",
		RollbackPlanRef:              " rollback:base-runtime-durable-proof-gap-pass-123 ",
		RemainingGaps: []string{
			BaseRuntimeDurableProofGapPromotionProof,
			BaseRuntimeDurableProofGapRuntimeEquivalenceRetry,
			" ",
			BaseRuntimeDurableProofGapFullSubstrateProof,
			BaseRuntimeDurableProofGapRuntimeFileBlobExtraction,
			BaseRuntimeDurableProofGapPackagePublicationProof,
			BaseRuntimeDurableProofGapRunAcceptanceProof,
			BaseRuntimeDurableProofGapStagingProof,
			BaseRuntimeDurableProofGapRuntimeEquivalenceRetry,
		},
		NoRuntimeMaterialization:     true,
		NoDurableComputerMutation:    true,
		NoDeployedRouteMutation:      true,
		NoProductionMutation:         true,
		NoPackagePublicationMutation: true,
		NoPromotionMutation:          true,
		NoRunAcceptanceMutation:      true,
	}
	return reentry, summary, evidence
}

func baseRuntimeDurableProofGapAlignedLocalSummary(t *testing.T, version ComputerVersion) BaseLocalSubstrateProofSummaryContract {
	t.Helper()

	current, projection, substrateEvidence := baseSubstrateEquivalenceContractInputs(t)
	setRealizationVersionForBaseRuntimeDurableProofGap(&current, version)
	setRealizationVersionForBaseRuntimeDurableProofGap(&projection, version)
	substrate, err := BuildBaseSubstrateEquivalenceContract(current, projection, substrateEvidence)
	if err != nil {
		t.Fatalf("BuildBaseSubstrateEquivalenceContract(): %v", err)
	}

	calibration := baseSubstrateReentryReadinessCalibrationContract(t, version, false)
	reentryEvidence := BaseSubstrateReentryReadinessEvidence{
		SubstrateEquivalenceContractRef: " base-substrate-equivalence-contract:durable-gap-pass-123 ",
		EquivalenceEvidenceSetRef:       " base-equivalence-evidence-set-contract:durable-gap-pass-123 ",
		NextProbeRef:                    " base-substrate-equivalence-next-probe:durable-gap-pass-123 ",
		NoRuntimeMaterialization:        true,
		NoOpaqueDataImageDependency:     true,
		NoMutation:                      true,
	}
	localReentry, err := BuildBaseSubstrateReentryReadinessContract(substrate, calibration, reentryEvidence)
	if err != nil {
		t.Fatalf("BuildBaseSubstrateReentryReadinessContract(): %v", err)
	}

	summaryEvidence := BaseLocalSubstrateProofSummaryEvidence{
		SubstrateEquivalenceContractRef: " " + localReentry.SubstrateEquivalenceContractRef + " ",
		ReentryReadinessContractRef:     " base-substrate-reentry-readiness-contract:durable-gap-pass-123 ",
		EquivalenceEvidenceSetRef:       "\t" + localReentry.EquivalenceEvidenceSetRef + "\t",
		SummaryRef:                      " base-local-substrate-proof-summary:durable-gap-pass-123 ",
		RemainingGaps: []string{
			BaseLocalSubstrateProofSummaryRemainingPromotionProof,
			BaseLocalSubstrateProofSummaryRemainingRuntimeSubstrateProof,
			BaseLocalSubstrateProofSummaryRemainingStagingProof,
		},
		NoRuntimeMaterialization:    true,
		NoOpaqueDataImageDependency: true,
		NoMutation:                  true,
	}
	summary, err := BuildBaseLocalSubstrateProofSummaryContract(substrate, localReentry, summaryEvidence)
	if err != nil {
		t.Fatalf("BuildBaseLocalSubstrateProofSummaryContract(): %v", err)
	}
	return summary
}

func setRealizationVersionForBaseRuntimeDurableProofGap(realization *Realization, version ComputerVersion) {
	realization.Version = version
	realization.Observations.Version = version
}

func assertBaseRuntimeDurableProofGapRemainingGaps(t *testing.T, got []string) {
	t.Helper()

	want := []string{
		BaseRuntimeDurableProofGapRuntimeFileBlobExtraction,
		BaseRuntimeDurableProofGapRuntimeEquivalenceRetry,
		BaseRuntimeDurableProofGapStagingProof,
		BaseRuntimeDurableProofGapPromotionProof,
		BaseRuntimeDurableProofGapPackagePublicationProof,
		BaseRuntimeDurableProofGapRunAcceptanceProof,
		BaseRuntimeDurableProofGapFullSubstrateProof,
	}
	if len(got) != len(want) {
		t.Fatalf("remaining gaps = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("remaining gaps = %#v, want %#v", got, want)
		}
	}
}

func removeBaseRuntimeDurableProofGap(gaps []string, remove string) []string {
	out := make([]string, 0, len(gaps))
	for _, gap := range gaps {
		if strings.TrimSpace(gap) == remove {
			continue
		}
		out = append(out, gap)
	}
	return out
}
