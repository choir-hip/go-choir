package computerversion

import (
	"strings"
	"testing"
)

func TestBuildBaseRuntimeDurableGapExtractionHandoffContractAdmitsTypedExtractionWithoutDownstreamClaims(t *testing.T) {
	reentry, gap, extraction, evidence := baseRuntimeDurableGapExtractionHandoffContractInputs(t)

	contract, err := BuildBaseRuntimeDurableGapExtractionHandoffContract(gap, extraction, evidence)
	if err != nil {
		t.Fatalf("BuildBaseRuntimeDurableGapExtractionHandoffContract(): %v", err)
	}

	if contract.Kind != BaseRuntimeDurableGapExtractionHandoffContractKind {
		t.Fatalf("kind = %q, want %q", contract.Kind, BaseRuntimeDurableGapExtractionHandoffContractKind)
	}
	if contract.Version != gap.Version || contract.Version != extraction.Version {
		t.Fatalf("version = %#v, want shared gap/extraction version %#v/%#v", contract.Version, gap.Version, extraction.Version)
	}
	if contract.Boundary != BaseRuntimeDurableGapExtractionHandoffBoundary {
		t.Fatalf("boundary = %q, want %q", contract.Boundary, BaseRuntimeDurableGapExtractionHandoffBoundary)
	}
	if contract.Scope != BaseRuntimeDurableGapExtractionHandoffScope {
		t.Fatalf("scope = %q, want %q", contract.Scope, BaseRuntimeDurableGapExtractionHandoffScope)
	}
	if ArtifactProgramRef(contract.TypedArtifactProgramRef) != gap.Version.ArtifactProgramRef || contract.TypedArtifactProgramRef != strings.TrimSpace(gap.TypedArtifactProgramRef) {
		t.Fatalf("typed artifact program ref = %q, want trimmed gap artifact program %q", contract.TypedArtifactProgramRef, gap.Version.ArtifactProgramRef)
	}
	if gap.RuntimeEquivalenceBoundaryRef != strings.TrimSpace(reentry.RuntimeEquivalenceBoundaryRef) {
		t.Fatalf("gap runtime boundary ref = %q, want copied reentry boundary ref %q", gap.RuntimeEquivalenceBoundaryRef, strings.TrimSpace(reentry.RuntimeEquivalenceBoundaryRef))
	}
	if extraction.RuntimeEquivalenceBoundaryRef != gap.RuntimeEquivalenceBoundaryRef || contract.RuntimeEquivalenceBoundaryRef != gap.RuntimeEquivalenceBoundaryRef {
		t.Fatalf("runtime boundary refs = extraction %q handoff %q, want gap/reentry boundary %q", extraction.RuntimeEquivalenceBoundaryRef, contract.RuntimeEquivalenceBoundaryRef, gap.RuntimeEquivalenceBoundaryRef)
	}
	if extraction.SourceProvenanceReadinessRef != gap.SourceProvenanceReadinessRef {
		t.Fatalf("test fixture extraction source provenance ref = %q, want gap source provenance ref %q", extraction.SourceProvenanceReadinessRef, gap.SourceProvenanceReadinessRef)
	}
	if contract.RuntimeDurableProofGapRef != strings.TrimSpace(evidence.RuntimeDurableProofGapRef) || contract.RuntimeFileBlobExtractionRef != strings.TrimSpace(evidence.RuntimeFileBlobExtractionRef) || contract.ExtractionHandoffReviewRef != strings.TrimSpace(evidence.ExtractionHandoffReviewRef) || contract.RuntimeEquivalenceRetryPlanRef != strings.TrimSpace(evidence.RuntimeEquivalenceRetryPlanRef) || contract.RollbackPlanRef != strings.TrimSpace(evidence.RollbackPlanRef) {
		t.Fatalf("handoff refs = gap %q extraction %q review %q retry-plan %q rollback %q, want trimmed refs from %#v", contract.RuntimeDurableProofGapRef, contract.RuntimeFileBlobExtractionRef, contract.ExtractionHandoffReviewRef, contract.RuntimeEquivalenceRetryPlanRef, contract.RollbackPlanRef, evidence)
	}
	if contract.RuntimeEquivalenceReentryRef != gap.RuntimeEquivalenceReentryRef || contract.LocalSubstrateSummaryRef != gap.LocalSubstrateSummaryRef || contract.SourceMaterializerReadinessRef != gap.SourceMaterializerReadinessRef || contract.RuntimeMaterializationRef != gap.RuntimeMaterializationRef || contract.RuntimeEvidenceReviewRef != gap.RuntimeEvidenceReviewRef || contract.SourceProvenanceReadinessRef != gap.SourceProvenanceReadinessRef {
		t.Fatalf("gap refs = reentry %q local summary %q source materializer %q materialization %q runtime review %q source provenance %q, want gap refs %#v", contract.RuntimeEquivalenceReentryRef, contract.LocalSubstrateSummaryRef, contract.SourceMaterializerReadinessRef, contract.RuntimeMaterializationRef, contract.RuntimeEvidenceReviewRef, contract.SourceProvenanceReadinessRef, gap)
	}
	if contract.RuntimeObservationExtractionRef != extraction.RuntimeObservationExtractionRef || contract.ExtractorRef != extraction.ExtractorRef || contract.ExtractedObservationSetName != extraction.ExtractedObservationSetName {
		t.Fatalf("extraction refs = observation %q extractor %q set %q, want extraction refs %#v", contract.RuntimeObservationExtractionRef, contract.ExtractorRef, contract.ExtractedObservationSetName, extraction)
	}
	if contract.RuntimeMaterializer != gap.RuntimeMaterializer || contract.RuntimeSubstrate != gap.RuntimeSubstrate {
		t.Fatalf("runtime identity = %q/%q, want gap %q/%q", contract.RuntimeMaterializer, contract.RuntimeSubstrate, gap.RuntimeMaterializer, gap.RuntimeSubstrate)
	}
	assertObservationBundleKinds(t, contract.RequiredRuntimeObservations, []ObservationKind{ObservationBlobSet, ObservationFileManifest})
	assertUnsupportedDurableObservations(t, contract.UnsupportedDurableObservations, []ObservationKind{ObservationFileManifest, ObservationBlobSet})
	assertBaseRuntimeDurableGapExtractionHandoffRemainingGaps(t, contract.RemainingGaps)
	if contract.HandoffStatus != BaseRuntimeDurableGapExtractionHandoffStatusReady {
		t.Fatalf("handoff status = %q, want %q", contract.HandoffStatus, BaseRuntimeDurableGapExtractionHandoffStatusReady)
	}
	if !contract.RuntimeFileBlobExtractionSatisfied || !contract.RuntimeEquivalenceRetryRequired || !contract.RuntimeEquivalenceMayBeRetried || contract.RuntimeEquivalenceClaimed {
		t.Fatalf("handoff retry gate = extraction satisfied %v retry required %v may retry %v claimed %v, want true/true/true/false", contract.RuntimeFileBlobExtractionSatisfied, contract.RuntimeEquivalenceRetryRequired, contract.RuntimeEquivalenceMayBeRetried, contract.RuntimeEquivalenceClaimed)
	}
	if !contract.DurableStateEquivalenceRequired || !contract.StagingProofRequired || !contract.PromotionProofRequired || !contract.PackagePublicationProofRequired || !contract.RunAcceptanceProofRequired || !contract.FullSubstrateProofRequired {
		t.Fatalf("handoff must preserve durable-state, staging, promotion, package, run, and full-substrate proof requirements: %#v", contract)
	}
	if contract.VMLifecycleMutationAllowed || contract.DurableComputerMutationAllowed || contract.DeployedRouteRegistrationAllowed || contract.ProductionMutationAllowed || contract.PackagePublicationAllowed || contract.PromotionAllowed || contract.RunAcceptanceSynthesisAllowed {
		t.Fatalf("handoff must deny VM lifecycle, durable computer, deployed route, production, package, promotion, and run-acceptance authority: %#v", contract)
	}
	if !contract.NoOpaqueDataImageDependency || !contract.NoVMLifecycleMutation || !contract.NoDurableComputerMutation || !contract.NoDeployedRouteMutation || !contract.NoProductionMutation || !contract.NoPackagePublicationMutation || !contract.NoPromotionMutation || !contract.NoRunAcceptanceMutation {
		t.Fatalf("handoff must carry no-opaque/no-VM/no-durable/no-route/no-production/no-package/no-promotion/no-run flags: %#v", contract)
	}
	if contract.RuntimeBehaviorChanged || contract.DurableComputerStateMutated || contract.DeployedRouteRegistered || contract.ProductionAuthTouched || contract.ProductionStateMutated || contract.PackagePublished || contract.PromotionExecuted || contract.VMLifecycleTouched || contract.FirecrackerBootClaimed || contract.StagingClaimed || contract.RunAcceptanceRecordTouched || contract.FullSubstrateClaimed || contract.CompletionClaimed {
		t.Fatalf("handoff must not claim runtime behavior, protected surfaces, downstream proofs, full-substrate, or completion: %#v", contract)
	}
}

func TestBuildBaseRuntimeDurableGapExtractionHandoffContractRejectsInvalidInputs(t *testing.T) {
	foreignVersion := ComputerVersion{CodeRef: "git:foreign-base-runtime-durable-gap-extraction-handoff", ArtifactProgramRef: "base-journal:owner/main@foreign-base-runtime-durable-gap-extraction-handoff"}
	for _, tc := range []struct {
		name    string
		mutate  func(*BaseRuntimeDurableProofGapContract, *BaseRuntimeFileBlobExtractionContract, *BaseRuntimeDurableGapExtractionHandoffEvidence)
		wantErr string
	}{
		{
			name: "gap kind drift",
			mutate: func(gap *BaseRuntimeDurableProofGapContract, _ *BaseRuntimeFileBlobExtractionContract, _ *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				gap.Kind = BaseRuntimeDurableGapExtractionHandoffContractKind
			},
			wantErr: "gap kind",
		},
		{
			name: "gap boundary drift",
			mutate: func(gap *BaseRuntimeDurableProofGapContract, _ *BaseRuntimeFileBlobExtractionContract, _ *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				gap.Boundary = BaseRuntimeDurableGapExtractionHandoffBoundary
			},
			wantErr: "gap boundary",
		},
		{
			name: "gap scope drift",
			mutate: func(gap *BaseRuntimeDurableProofGapContract, _ *BaseRuntimeFileBlobExtractionContract, _ *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				gap.Scope = BaseRuntimeDurableGapExtractionHandoffScope
			},
			wantErr: "gap scope",
		},
		{
			name: "gap invalid version",
			mutate: func(gap *BaseRuntimeDurableProofGapContract, _ *BaseRuntimeFileBlobExtractionContract, _ *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				gap.Version.CodeRef = "  "
			},
			wantErr: "gap version or artifact ref is invalid",
		},
		{
			name: "gap typed artifact ref drift",
			mutate: func(gap *BaseRuntimeDurableProofGapContract, _ *BaseRuntimeFileBlobExtractionContract, _ *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				gap.TypedArtifactProgramRef = string(foreignVersion.ArtifactProgramRef)
			},
			wantErr: "gap version or artifact ref is invalid",
		},
		{
			name: "gap missing runtime boundary ref",
			mutate: func(gap *BaseRuntimeDurableProofGapContract, _ *BaseRuntimeFileBlobExtractionContract, _ *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				gap.RuntimeEquivalenceBoundaryRef = "\t"
			},
			wantErr: "gap refs are required",
		},
		{
			name: "gap status drift",
			mutate: func(gap *BaseRuntimeDurableProofGapContract, _ *BaseRuntimeFileBlobExtractionContract, _ *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				gap.GapStatus = "runtime_durable_gap_closed"
			},
			wantErr: "gap must remain open and narrowed",
		},
		{
			name: "gap narrowed flag drift",
			mutate: func(gap *BaseRuntimeDurableProofGapContract, _ *BaseRuntimeFileBlobExtractionContract, _ *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				gap.RuntimeEquivalenceNarrowed = false
			},
			wantErr: "gap must remain open and narrowed",
		},
		{
			name: "gap extraction proof obligation missing",
			mutate: func(gap *BaseRuntimeDurableProofGapContract, _ *BaseRuntimeFileBlobExtractionContract, _ *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				gap.RuntimeFileBlobExtractionRequired = false
			},
			wantErr: "gap must preserve extraction, retry, and downstream proof requirements",
		},
		{
			name: "gap retry proof obligation missing",
			mutate: func(gap *BaseRuntimeDurableProofGapContract, _ *BaseRuntimeFileBlobExtractionContract, _ *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				gap.RuntimeEquivalenceRetryRequired = false
			},
			wantErr: "gap must preserve extraction, retry, and downstream proof requirements",
		},
		{
			name: "gap downstream proof obligation missing",
			mutate: func(gap *BaseRuntimeDurableProofGapContract, _ *BaseRuntimeFileBlobExtractionContract, _ *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				gap.FullSubstrateProofRequired = false
			},
			wantErr: "gap must preserve extraction, retry, and downstream proof requirements",
		},
		{
			name: "gap remaining runtime file blob extraction missing",
			mutate: func(gap *BaseRuntimeDurableProofGapContract, _ *BaseRuntimeFileBlobExtractionContract, _ *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				gap.RemainingGaps = removeBaseRuntimeDurableProofGap(gap.RemainingGaps, BaseRuntimeDurableProofGapRuntimeFileBlobExtraction)
			},
			wantErr: "gap remaining obligations are incomplete",
		},
		{
			name: "gap remaining retry missing",
			mutate: func(gap *BaseRuntimeDurableProofGapContract, _ *BaseRuntimeFileBlobExtractionContract, _ *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				gap.RemainingGaps = removeBaseRuntimeDurableProofGap(gap.RemainingGaps, BaseRuntimeDurableProofGapRuntimeEquivalenceRetry)
			},
			wantErr: "gap remaining obligations are incomplete",
		},
		{
			name: "gap runtime observation drift",
			mutate: func(gap *BaseRuntimeDurableProofGapContract, _ *BaseRuntimeFileBlobExtractionContract, _ *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				gap.RuntimeRequiredObservations = nil
			},
			wantErr: "gap observations are incomplete",
		},
		{
			name: "gap local observation drift",
			mutate: func(gap *BaseRuntimeDurableProofGapContract, _ *BaseRuntimeFileBlobExtractionContract, _ *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				gap.LocalRequiredObservations = []ObservationKind{ObservationFileManifest}
			},
			wantErr: "gap observations are incomplete",
		},
		{
			name: "gap unsupported file manifest missing",
			mutate: func(gap *BaseRuntimeDurableProofGapContract, _ *BaseRuntimeFileBlobExtractionContract, _ *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				gap.UnsupportedDurableObservations = []UnsupportedCapability{{Kind: ObservationBlobSet, Reason: "runtime evidence does not expose source blobs"}}
			},
			wantErr: "gap must preserve unsupported durable observations",
		},
		{
			name: "gap unsupported blob set missing",
			mutate: func(gap *BaseRuntimeDurableProofGapContract, _ *BaseRuntimeFileBlobExtractionContract, _ *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				gap.UnsupportedDurableObservations = []UnsupportedCapability{{Kind: ObservationFileManifest, Reason: "runtime evidence does not expose source file manifests"}}
			},
			wantErr: "gap must preserve unsupported durable observations",
		},
		{
			name: "gap authority drift",
			mutate: func(gap *BaseRuntimeDurableProofGapContract, _ *BaseRuntimeFileBlobExtractionContract, _ *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				gap.PromotionAllowed = true
			},
			wantErr: "gap allows downstream authority",
		},
		{
			name: "gap no mutation flag drift",
			mutate: func(gap *BaseRuntimeDurableProofGapContract, _ *BaseRuntimeFileBlobExtractionContract, _ *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				gap.NoProductionMutation = false
			},
			wantErr: "gap must prove no mutation",
		},
		{
			name: "gap runtime behavior claim drift",
			mutate: func(gap *BaseRuntimeDurableProofGapContract, _ *BaseRuntimeFileBlobExtractionContract, _ *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				gap.RuntimeBehaviorChanged = true
			},
			wantErr: "gap carries protected-surface or completion claims",
		},
		{
			name: "gap completion claim drift",
			mutate: func(gap *BaseRuntimeDurableProofGapContract, _ *BaseRuntimeFileBlobExtractionContract, _ *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				gap.CompletionClaimed = true
			},
			wantErr: "gap carries protected-surface or completion claims",
		},
		{
			name: "extraction kind drift",
			mutate: func(_ *BaseRuntimeDurableProofGapContract, extraction *BaseRuntimeFileBlobExtractionContract, _ *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				extraction.Kind = BaseRuntimeDurableGapExtractionHandoffContractKind
			},
			wantErr: "extraction kind",
		},
		{
			name: "extraction boundary drift",
			mutate: func(_ *BaseRuntimeDurableProofGapContract, extraction *BaseRuntimeFileBlobExtractionContract, _ *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				extraction.Boundary = BaseRuntimeDurableGapExtractionHandoffBoundary
			},
			wantErr: "extraction boundary",
		},
		{
			name: "extraction scope drift",
			mutate: func(_ *BaseRuntimeDurableProofGapContract, extraction *BaseRuntimeFileBlobExtractionContract, _ *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				extraction.Scope = BaseRuntimeDurableGapExtractionHandoffScope
			},
			wantErr: "extraction scope",
		},
		{
			name: "extraction version drift",
			mutate: func(_ *BaseRuntimeDurableProofGapContract, extraction *BaseRuntimeFileBlobExtractionContract, _ *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				extraction.Version = foreignVersion
			},
			wantErr: "extraction version does not match gap",
		},
		{
			name: "extraction typed artifact drift",
			mutate: func(_ *BaseRuntimeDurableProofGapContract, extraction *BaseRuntimeFileBlobExtractionContract, _ *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				extraction.TypedArtifactProgramRef = string(foreignVersion.ArtifactProgramRef)
			},
			wantErr: "extraction typed artifact-program ref does not match gap",
		},
		{
			name: "extraction missing observation extraction ref",
			mutate: func(_ *BaseRuntimeDurableProofGapContract, extraction *BaseRuntimeFileBlobExtractionContract, _ *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				extraction.RuntimeObservationExtractionRef = "  "
			},
			wantErr: "extraction refs are required",
		},
		{
			name: "extraction missing extractor ref",
			mutate: func(_ *BaseRuntimeDurableProofGapContract, extraction *BaseRuntimeFileBlobExtractionContract, _ *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				extraction.ExtractorRef = "\t"
			},
			wantErr: "extraction refs are required",
		},
		{
			name: "extraction source provenance alignment drift",
			mutate: func(_ *BaseRuntimeDurableProofGapContract, extraction *BaseRuntimeFileBlobExtractionContract, _ *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				extraction.SourceProvenanceReadinessRef = "source-provenance:other-base"
			},
			wantErr: "extraction does not match gap refs",
		},
		{
			name: "extraction runtime boundary alignment drift",
			mutate: func(_ *BaseRuntimeDurableProofGapContract, extraction *BaseRuntimeFileBlobExtractionContract, _ *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				extraction.RuntimeEquivalenceBoundaryRef = "runtime-equivalence-boundary:other-base"
			},
			wantErr: "extraction does not match gap refs",
		},
		{
			name: "extraction readiness drift",
			mutate: func(_ *BaseRuntimeDurableProofGapContract, extraction *BaseRuntimeFileBlobExtractionContract, _ *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				extraction.RuntimeFileBlobObservationsReady = false
			},
			wantErr: "extraction must satisfy file/blob observations without claiming equivalence",
		},
		{
			name: "extraction retry drift",
			mutate: func(_ *BaseRuntimeDurableProofGapContract, extraction *BaseRuntimeFileBlobExtractionContract, _ *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				extraction.RuntimeEquivalenceMayBeRetried = false
			},
			wantErr: "extraction must satisfy file/blob observations without claiming equivalence",
		},
		{
			name: "extraction claim drift",
			mutate: func(_ *BaseRuntimeDurableProofGapContract, extraction *BaseRuntimeFileBlobExtractionContract, _ *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				extraction.RuntimeEquivalenceClaimed = true
			},
			wantErr: "extraction must satisfy file/blob observations without claiming equivalence",
		},
		{
			name: "extraction missing file manifest observation",
			mutate: func(_ *BaseRuntimeDurableProofGapContract, extraction *BaseRuntimeFileBlobExtractionContract, _ *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				extraction.RequiredRuntimeObservations = []ObservationKind{ObservationBlobSet}
			},
			wantErr: "extraction must require only typed file/blob runtime observations",
		},
		{
			name: "extraction includes vm state observation",
			mutate: func(_ *BaseRuntimeDurableProofGapContract, extraction *BaseRuntimeFileBlobExtractionContract, _ *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				extraction.RequiredRuntimeObservations = append(extraction.RequiredRuntimeObservations, ObservationVMStateManifest)
			},
			wantErr: "extraction must require only typed file/blob runtime observations",
		},
		{
			name: "extraction opaque dependency flag missing",
			mutate: func(_ *BaseRuntimeDurableProofGapContract, extraction *BaseRuntimeFileBlobExtractionContract, _ *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				extraction.NoOpaqueDataImageDependency = false
			},
			wantErr: "extraction must prove no opaque data.img, VM lifecycle, or production mutation",
		},
		{
			name: "extraction VM lifecycle flag missing",
			mutate: func(_ *BaseRuntimeDurableProofGapContract, extraction *BaseRuntimeFileBlobExtractionContract, _ *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				extraction.NoVMLifecycleMutation = false
			},
			wantErr: "extraction must prove no opaque data.img, VM lifecycle, or production mutation",
		},
		{
			name: "extraction production flag missing",
			mutate: func(_ *BaseRuntimeDurableProofGapContract, extraction *BaseRuntimeFileBlobExtractionContract, _ *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				extraction.NoProductionMutation = false
			},
			wantErr: "extraction must prove no opaque data.img, VM lifecycle, or production mutation",
		},
		{
			name: "extraction protected claim drift",
			mutate: func(_ *BaseRuntimeDurableProofGapContract, extraction *BaseRuntimeFileBlobExtractionContract, _ *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				extraction.DeployedRouteRegistered = true
			},
			wantErr: "extraction carries protected-surface or completion claims",
		},
		{
			name: "missing evidence durable proof gap ref",
			mutate: func(_ *BaseRuntimeDurableProofGapContract, _ *BaseRuntimeFileBlobExtractionContract, evidence *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				evidence.RuntimeDurableProofGapRef = "  "
			},
			wantErr: "evidence refs are required",
		},
		{
			name: "missing evidence extraction ref",
			mutate: func(_ *BaseRuntimeDurableProofGapContract, _ *BaseRuntimeFileBlobExtractionContract, evidence *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				evidence.RuntimeFileBlobExtractionRef = "\t"
			},
			wantErr: "evidence refs are required",
		},
		{
			name: "missing evidence handoff review ref",
			mutate: func(_ *BaseRuntimeDurableProofGapContract, _ *BaseRuntimeFileBlobExtractionContract, evidence *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				evidence.ExtractionHandoffReviewRef = ""
			},
			wantErr: "evidence refs are required",
		},
		{
			name: "missing evidence retry plan ref",
			mutate: func(_ *BaseRuntimeDurableProofGapContract, _ *BaseRuntimeFileBlobExtractionContract, evidence *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				evidence.RuntimeEquivalenceRetryPlanRef = "  "
			},
			wantErr: "evidence refs are required",
		},
		{
			name: "missing evidence rollback ref",
			mutate: func(_ *BaseRuntimeDurableProofGapContract, _ *BaseRuntimeFileBlobExtractionContract, evidence *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				evidence.RollbackPlanRef = "\t"
			},
			wantErr: "evidence refs are required",
		},
		{
			name: "missing evidence no opaque dependency flag",
			mutate: func(_ *BaseRuntimeDurableProofGapContract, _ *BaseRuntimeFileBlobExtractionContract, evidence *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				evidence.NoOpaqueDataImageDependency = false
			},
			wantErr: "evidence must prove no opaque data.img",
		},
		{
			name: "missing evidence no VM lifecycle flag",
			mutate: func(_ *BaseRuntimeDurableProofGapContract, _ *BaseRuntimeFileBlobExtractionContract, evidence *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				evidence.NoVMLifecycleMutation = false
			},
			wantErr: "evidence must prove no opaque data.img",
		},
		{
			name: "missing evidence no durable mutation flag",
			mutate: func(_ *BaseRuntimeDurableProofGapContract, _ *BaseRuntimeFileBlobExtractionContract, evidence *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				evidence.NoDurableComputerMutation = false
			},
			wantErr: "evidence must prove no opaque data.img",
		},
		{
			name: "missing evidence no route flag",
			mutate: func(_ *BaseRuntimeDurableProofGapContract, _ *BaseRuntimeFileBlobExtractionContract, evidence *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				evidence.NoDeployedRouteMutation = false
			},
			wantErr: "evidence must prove no opaque data.img",
		},
		{
			name: "missing evidence no production flag",
			mutate: func(_ *BaseRuntimeDurableProofGapContract, _ *BaseRuntimeFileBlobExtractionContract, evidence *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				evidence.NoProductionMutation = false
			},
			wantErr: "evidence must prove no opaque data.img",
		},
		{
			name: "missing evidence no package flag",
			mutate: func(_ *BaseRuntimeDurableProofGapContract, _ *BaseRuntimeFileBlobExtractionContract, evidence *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				evidence.NoPackagePublicationMutation = false
			},
			wantErr: "evidence must prove no opaque data.img",
		},
		{
			name: "missing evidence no promotion flag",
			mutate: func(_ *BaseRuntimeDurableProofGapContract, _ *BaseRuntimeFileBlobExtractionContract, evidence *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				evidence.NoPromotionMutation = false
			},
			wantErr: "evidence must prove no opaque data.img",
		},
		{
			name: "missing evidence no run acceptance flag",
			mutate: func(_ *BaseRuntimeDurableProofGapContract, _ *BaseRuntimeFileBlobExtractionContract, evidence *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				evidence.NoRunAcceptanceMutation = false
			},
			wantErr: "evidence must prove no opaque data.img",
		},
		{
			name: "evidence runtime behavior changed",
			mutate: func(_ *BaseRuntimeDurableProofGapContract, _ *BaseRuntimeFileBlobExtractionContract, evidence *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				evidence.RuntimeBehaviorChanged = true
			},
			wantErr: "evidence carries runtime-equivalence",
		},
		{
			name: "evidence runtime equivalence claimed",
			mutate: func(_ *BaseRuntimeDurableProofGapContract, _ *BaseRuntimeFileBlobExtractionContract, evidence *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				evidence.RuntimeEquivalenceClaimed = true
			},
			wantErr: "evidence carries runtime-equivalence",
		},
		{
			name: "evidence durable computer mutation",
			mutate: func(_ *BaseRuntimeDurableProofGapContract, _ *BaseRuntimeFileBlobExtractionContract, evidence *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				evidence.DurableComputerStateMutated = true
			},
			wantErr: "evidence carries runtime-equivalence",
		},
		{
			name: "evidence deployed route registration",
			mutate: func(_ *BaseRuntimeDurableProofGapContract, _ *BaseRuntimeFileBlobExtractionContract, evidence *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				evidence.DeployedRouteRegistered = true
			},
			wantErr: "evidence carries runtime-equivalence",
		},
		{
			name: "evidence production auth touch",
			mutate: func(_ *BaseRuntimeDurableProofGapContract, _ *BaseRuntimeFileBlobExtractionContract, evidence *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				evidence.ProductionAuthTouched = true
			},
			wantErr: "evidence carries runtime-equivalence",
		},
		{
			name: "evidence production state touch",
			mutate: func(_ *BaseRuntimeDurableProofGapContract, _ *BaseRuntimeFileBlobExtractionContract, evidence *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				evidence.ProductionStateMutated = true
			},
			wantErr: "evidence carries runtime-equivalence",
		},
		{
			name: "evidence package publication",
			mutate: func(_ *BaseRuntimeDurableProofGapContract, _ *BaseRuntimeFileBlobExtractionContract, evidence *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				evidence.PackagePublished = true
			},
			wantErr: "evidence carries runtime-equivalence",
		},
		{
			name: "evidence promotion execution",
			mutate: func(_ *BaseRuntimeDurableProofGapContract, _ *BaseRuntimeFileBlobExtractionContract, evidence *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				evidence.PromotionExecuted = true
			},
			wantErr: "evidence carries runtime-equivalence",
		},
		{
			name: "evidence VM lifecycle touch",
			mutate: func(_ *BaseRuntimeDurableProofGapContract, _ *BaseRuntimeFileBlobExtractionContract, evidence *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				evidence.VMLifecycleTouched = true
			},
			wantErr: "evidence carries runtime-equivalence",
		},
		{
			name: "evidence Firecracker boot claim",
			mutate: func(_ *BaseRuntimeDurableProofGapContract, _ *BaseRuntimeFileBlobExtractionContract, evidence *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				evidence.FirecrackerBootClaimed = true
			},
			wantErr: "evidence carries runtime-equivalence",
		},
		{
			name: "evidence staging claim",
			mutate: func(_ *BaseRuntimeDurableProofGapContract, _ *BaseRuntimeFileBlobExtractionContract, evidence *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				evidence.StagingClaimed = true
			},
			wantErr: "evidence carries runtime-equivalence",
		},
		{
			name: "evidence run acceptance touch",
			mutate: func(_ *BaseRuntimeDurableProofGapContract, _ *BaseRuntimeFileBlobExtractionContract, evidence *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				evidence.RunAcceptanceRecordTouched = true
			},
			wantErr: "evidence carries runtime-equivalence",
		},
		{
			name: "evidence full substrate claim",
			mutate: func(_ *BaseRuntimeDurableProofGapContract, _ *BaseRuntimeFileBlobExtractionContract, evidence *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				evidence.FullSubstrateClaimed = true
			},
			wantErr: "evidence carries runtime-equivalence",
		},
		{
			name: "evidence completion claim",
			mutate: func(_ *BaseRuntimeDurableProofGapContract, _ *BaseRuntimeFileBlobExtractionContract, evidence *BaseRuntimeDurableGapExtractionHandoffEvidence) {
				evidence.CompletionClaimed = true
			},
			wantErr: "evidence carries runtime-equivalence",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, gap, extraction, evidence := baseRuntimeDurableGapExtractionHandoffContractInputs(t)
			tc.mutate(&gap, &extraction, &evidence)

			contract, err := BuildBaseRuntimeDurableGapExtractionHandoffContract(gap, extraction, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildBaseRuntimeDurableGapExtractionHandoffContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func baseRuntimeDurableGapExtractionHandoffContractInputs(t *testing.T) (BaseRuntimeEquivalenceReentryContract, BaseRuntimeDurableProofGapContract, BaseRuntimeFileBlobExtractionContract, BaseRuntimeDurableGapExtractionHandoffEvidence) {
	t.Helper()

	bridge, equivalence, reentryEvidence := baseRuntimeEquivalenceReentryContractInputs(t)
	reentry, err := BuildBaseRuntimeEquivalenceReentryContract(bridge, equivalence, reentryEvidence)
	if err != nil {
		t.Fatalf("BuildBaseRuntimeEquivalenceReentryContract(): %v", err)
	}

	summary := baseRuntimeDurableProofGapAlignedLocalSummary(t, reentry.Version)
	gapEvidence := BaseRuntimeDurableProofGapEvidence{
		RuntimeEquivalenceReentryRef: " contract:base-runtime-equivalence-reentry-pass-124 ",
		LocalSubstrateSummaryRef:     "\tcontract:base-local-substrate-proof-summary-pass-124\t",
		GapReviewRef:                 " review:base-runtime-durable-proof-gap-pass-124 ",
		RuntimeFileBlobPlanRef:       " plan:base-runtime-file-blob-extraction-pass-124 ",
		RuntimeEquivalenceRetryRef:   " retry:base-runtime-equivalence-pass-124 ",
		RollbackPlanRef:              " rollback:base-runtime-durable-proof-gap-pass-124 ",
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
	gap, err := BuildBaseRuntimeDurableProofGapContract(reentry, summary, gapEvidence)
	if err != nil {
		t.Fatalf("BuildBaseRuntimeDurableProofGapContract(): %v", err)
	}

	observations := ObservationSet{
		Name:     " base-runtime-durable-gap-extraction-handoff-observation-set-pass-124 ",
		Version:  equivalence.Version,
		Required: []ObservationKind{ObservationFileManifest, ObservationBlobSet},
		Observations: []Observation{
			FileManifestObservation("/workspace/base.txt", "sha256:file-manifest-root"),
			{Kind: ObservationBlobSet, Key: "blob:sha256:runtime-durable-gap-extraction-handoff", Value: "sha256:blob-root"},
		},
	}
	extractionEvidence := BaseRuntimeFileBlobExtractionEvidence{
		RuntimeEquivalenceBoundaryRef:   " " + gap.RuntimeEquivalenceBoundaryRef + " ",
		RuntimeObservationExtractionRef: " observation-set:base-runtime-durable-gap-extraction-handoff-pass-124 ",
		ExtractorRef:                    " extractor:typed-runtime-file-blob-pass-124 ",
		NoOpaqueDataImageDependency:     true,
		NoVMLifecycleMutation:           true,
		NoProductionMutation:            true,
	}
	extraction, err := BuildBaseRuntimeFileBlobExtractionContract(equivalence, observations, extractionEvidence)
	if err != nil {
		t.Fatalf("BuildBaseRuntimeFileBlobExtractionContract(): %v", err)
	}
	if extraction.RuntimeEquivalenceBoundaryRef != gap.RuntimeEquivalenceBoundaryRef {
		t.Fatalf("extraction runtime boundary ref = %q, want gap runtime boundary ref %q", extraction.RuntimeEquivalenceBoundaryRef, gap.RuntimeEquivalenceBoundaryRef)
	}
	if extraction.SourceProvenanceReadinessRef != gap.SourceProvenanceReadinessRef {
		t.Fatalf("extraction source provenance ref = %q, want gap source provenance ref %q", extraction.SourceProvenanceReadinessRef, gap.SourceProvenanceReadinessRef)
	}

	evidence := BaseRuntimeDurableGapExtractionHandoffEvidence{
		RuntimeDurableProofGapRef:      " contract:base-runtime-durable-proof-gap-pass-124 ",
		RuntimeFileBlobExtractionRef:   " contract:base-runtime-file-blob-extraction-pass-124 ",
		ExtractionHandoffReviewRef:     " review:base-runtime-durable-gap-extraction-handoff-pass-124 ",
		RuntimeEquivalenceRetryPlanRef: " plan:base-runtime-equivalence-retry-pass-124 ",
		RollbackPlanRef:                " rollback:base-runtime-durable-gap-extraction-handoff-pass-124 ",
		NoOpaqueDataImageDependency:    true,
		NoVMLifecycleMutation:          true,
		NoDurableComputerMutation:      true,
		NoDeployedRouteMutation:        true,
		NoProductionMutation:           true,
		NoPackagePublicationMutation:   true,
		NoPromotionMutation:            true,
		NoRunAcceptanceMutation:        true,
	}
	return reentry, gap, extraction, evidence
}

func assertBaseRuntimeDurableGapExtractionHandoffRemainingGaps(t *testing.T, got []string) {
	t.Helper()

	want := []string{
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
