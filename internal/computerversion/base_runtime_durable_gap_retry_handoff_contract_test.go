package computerversion

import (
	"strings"
	"testing"
)

func TestBuildBaseRuntimeDurableGapRetryHandoffContractAdmitsScopedRetryWithoutDownstreamClaims(t *testing.T) {
	handoff, retry, evidence := baseRuntimeDurableGapRetryHandoffContractInputs(t)

	contract, err := BuildBaseRuntimeDurableGapRetryHandoffContract(handoff, retry, evidence)
	if err != nil {
		t.Fatalf("BuildBaseRuntimeDurableGapRetryHandoffContract(): %v", err)
	}

	if contract.Kind != BaseRuntimeDurableGapRetryHandoffContractKind {
		t.Fatalf("kind = %q, want %q", contract.Kind, BaseRuntimeDurableGapRetryHandoffContractKind)
	}
	if contract.Version != handoff.Version || contract.Version != retry.Version {
		t.Fatalf("version = %#v, want shared handoff/retry version %#v/%#v", contract.Version, handoff.Version, retry.Version)
	}
	if contract.Boundary != BaseRuntimeDurableGapRetryHandoffBoundary {
		t.Fatalf("boundary = %q, want %q", contract.Boundary, BaseRuntimeDurableGapRetryHandoffBoundary)
	}
	if contract.Scope != BaseRuntimeDurableGapRetryHandoffScope {
		t.Fatalf("scope = %q, want %q", contract.Scope, BaseRuntimeDurableGapRetryHandoffScope)
	}
	if ArtifactProgramRef(contract.TypedArtifactProgramRef) != handoff.Version.ArtifactProgramRef || contract.TypedArtifactProgramRef != strings.TrimSpace(handoff.TypedArtifactProgramRef) {
		t.Fatalf("typed artifact program ref = %q, want trimmed handoff artifact program %q", contract.TypedArtifactProgramRef, handoff.Version.ArtifactProgramRef)
	}
	if contract.RuntimeDurableGapExtractionHandoffRef != strings.TrimSpace(evidence.RuntimeDurableGapExtractionHandoffRef) || contract.RuntimeEquivalenceRetryRef != strings.TrimSpace(evidence.RuntimeEquivalenceRetryRef) || contract.RetryHandoffReviewRef != strings.TrimSpace(evidence.RetryHandoffReviewRef) || contract.DownstreamProofPlanRef != strings.TrimSpace(evidence.DownstreamProofPlanRef) || contract.RollbackPlanRef != strings.TrimSpace(evidence.RollbackPlanRef) {
		t.Fatalf("handoff evidence refs = extraction-handoff %q retry %q review %q downstream %q rollback %q, want trimmed refs from %#v", contract.RuntimeDurableGapExtractionHandoffRef, contract.RuntimeEquivalenceRetryRef, contract.RetryHandoffReviewRef, contract.DownstreamProofPlanRef, contract.RollbackPlanRef, evidence)
	}
	if contract.RuntimeDurableProofGapRef != handoff.RuntimeDurableProofGapRef || contract.RuntimeFileBlobExtractionRef != handoff.RuntimeFileBlobExtractionRef {
		t.Fatalf("gap/extraction refs = gap %q extraction %q, want handoff refs %q/%q", contract.RuntimeDurableProofGapRef, contract.RuntimeFileBlobExtractionRef, handoff.RuntimeDurableProofGapRef, handoff.RuntimeFileBlobExtractionRef)
	}
	if contract.RuntimeEquivalenceReentryRef != handoff.RuntimeEquivalenceReentryRef || contract.RuntimeEquivalenceBoundaryRef != handoff.RuntimeEquivalenceBoundaryRef {
		t.Fatalf("runtime refs = reentry %q boundary %q, want handoff refs %q/%q", contract.RuntimeEquivalenceReentryRef, contract.RuntimeEquivalenceBoundaryRef, handoff.RuntimeEquivalenceReentryRef, handoff.RuntimeEquivalenceBoundaryRef)
	}
	if contract.LocalSubstrateSummaryRef != handoff.LocalSubstrateSummaryRef || contract.SourceMaterializerReadinessRef != handoff.SourceMaterializerReadinessRef || contract.RuntimeMaterializationRef != handoff.RuntimeMaterializationRef || contract.RuntimeEvidenceReviewRef != handoff.RuntimeEvidenceReviewRef || contract.SourceProvenanceReadinessRef != handoff.SourceProvenanceReadinessRef {
		t.Fatalf("proof chain refs = local %q source-materializer %q runtime-materialization %q runtime-evidence %q source-provenance %q, want handoff refs %#v", contract.LocalSubstrateSummaryRef, contract.SourceMaterializerReadinessRef, contract.RuntimeMaterializationRef, contract.RuntimeEvidenceReviewRef, contract.SourceProvenanceReadinessRef, handoff)
	}
	if contract.RuntimeObservationExtractionRef != handoff.RuntimeObservationExtractionRef {
		t.Fatalf("runtime observation extraction ref = %q, want handoff ref %q", contract.RuntimeObservationExtractionRef, handoff.RuntimeObservationExtractionRef)
	}
	if contract.SourceObservationSetRef != retry.SourceObservationSetRef || contract.SourceObservationSetName != retry.SourceObservationSetName || contract.RuntimeObservationSetName != retry.RuntimeObservationSetName {
		t.Fatalf("retry observation refs/names = source-ref %q source-name %q runtime-name %q, want retry values %q/%q/%q", contract.SourceObservationSetRef, contract.SourceObservationSetName, contract.RuntimeObservationSetName, retry.SourceObservationSetRef, retry.SourceObservationSetName, retry.RuntimeObservationSetName)
	}
	assertObservationBundleKinds(t, contract.RequiredObservations, []ObservationKind{ObservationBlobSet, ObservationFileManifest})
	assertBaseRuntimeDurableGapRetryHandoffRemainingGaps(t, contract.RemainingGaps)
	if contract.HandoffStatus != BaseRuntimeDurableGapRetryHandoffStatusReady {
		t.Fatalf("handoff status = %q, want %q", contract.HandoffStatus, BaseRuntimeDurableGapRetryHandoffStatusReady)
	}
	if !contract.RuntimeFileBlobExtractionSatisfied || !contract.RuntimeEquivalenceRetrySatisfied || !contract.ScopedRuntimeFileBlobEquivalenceClaimed {
		t.Fatalf("handoff closed gates = extraction %v retry %v scoped-equivalence %v, want all true", contract.RuntimeFileBlobExtractionSatisfied, contract.RuntimeEquivalenceRetrySatisfied, contract.ScopedRuntimeFileBlobEquivalenceClaimed)
	}
	if !contract.StagingProofRequired || !contract.PromotionProofRequired || !contract.PackagePublicationProofRequired || !contract.RunAcceptanceProofRequired || !contract.FullSubstrateProofRequired {
		t.Fatalf("handoff must preserve staging, promotion, package, run-acceptance, and full-substrate proof requirements: %#v", contract)
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

func TestBuildBaseRuntimeDurableGapRetryHandoffContractRejectsInvalidExtractionHandoff(t *testing.T) {
	foreignVersion := ComputerVersion{CodeRef: "git:foreign-base-runtime-durable-gap-retry-handoff", ArtifactProgramRef: "base-journal:owner/main@foreign-base-runtime-durable-gap-retry-handoff"}
	for _, tc := range []struct {
		name    string
		mutate  func(*BaseRuntimeDurableGapExtractionHandoffContract)
		wantErr string
	}{
		{
			name: "handoff kind drift",
			mutate: func(handoff *BaseRuntimeDurableGapExtractionHandoffContract) {
				handoff.Kind = BaseRuntimeDurableGapRetryHandoffContractKind
			},
			wantErr: "extraction handoff kind",
		},
		{
			name: "handoff boundary drift",
			mutate: func(handoff *BaseRuntimeDurableGapExtractionHandoffContract) {
				handoff.Boundary = BaseRuntimeDurableGapRetryHandoffBoundary
			},
			wantErr: "extraction handoff boundary",
		},
		{
			name: "handoff scope drift",
			mutate: func(handoff *BaseRuntimeDurableGapExtractionHandoffContract) {
				handoff.Scope = BaseRuntimeDurableGapRetryHandoffScope
			},
			wantErr: "extraction handoff scope",
		},
		{
			name: "handoff invalid version",
			mutate: func(handoff *BaseRuntimeDurableGapExtractionHandoffContract) {
				handoff.Version.CodeRef = "  "
			},
			wantErr: "extraction handoff version or artifact ref is invalid",
		},
		{
			name: "handoff typed artifact ref drift",
			mutate: func(handoff *BaseRuntimeDurableGapExtractionHandoffContract) {
				handoff.TypedArtifactProgramRef = string(foreignVersion.ArtifactProgramRef)
			},
			wantErr: "extraction handoff version or artifact ref is invalid",
		},
		{
			name: "handoff missing durable proof gap ref",
			mutate: func(handoff *BaseRuntimeDurableGapExtractionHandoffContract) {
				handoff.RuntimeDurableProofGapRef = "\t"
			},
			wantErr: "extraction handoff refs are required",
		},
		{
			name: "handoff missing runtime observation extraction ref",
			mutate: func(handoff *BaseRuntimeDurableGapExtractionHandoffContract) {
				handoff.RuntimeObservationExtractionRef = "  "
			},
			wantErr: "extraction handoff refs are required",
		},
		{
			name: "handoff status drift",
			mutate: func(handoff *BaseRuntimeDurableGapExtractionHandoffContract) {
				handoff.HandoffStatus = "runtime_durable_gap_extraction_closed"
			},
			wantErr: "extraction handoff must satisfy extraction and leave retry open",
		},
		{
			name: "handoff extraction satisfied drift",
			mutate: func(handoff *BaseRuntimeDurableGapExtractionHandoffContract) {
				handoff.RuntimeFileBlobExtractionSatisfied = false
			},
			wantErr: "extraction handoff must satisfy extraction and leave retry open",
		},
		{
			name: "handoff retry requirement missing",
			mutate: func(handoff *BaseRuntimeDurableGapExtractionHandoffContract) {
				handoff.RuntimeEquivalenceRetryRequired = false
			},
			wantErr: "extraction handoff must satisfy extraction and leave retry open",
		},
		{
			name: "handoff remaining retry gap missing",
			mutate: func(handoff *BaseRuntimeDurableGapExtractionHandoffContract) {
				handoff.RemainingGaps = removeBaseRuntimeDurableProofGap(handoff.RemainingGaps, BaseRuntimeDurableProofGapRuntimeEquivalenceRetry)
			},
			wantErr: "extraction handoff remaining gaps are incomplete",
		},
		{
			name: "handoff remaining staging gap missing",
			mutate: func(handoff *BaseRuntimeDurableGapExtractionHandoffContract) {
				handoff.RemainingGaps = removeBaseRuntimeDurableProofGap(handoff.RemainingGaps, BaseRuntimeDurableProofGapStagingProof)
			},
			wantErr: "extraction handoff remaining gaps are incomplete",
		},
		{
			name: "handoff missing file manifest observation scope",
			mutate: func(handoff *BaseRuntimeDurableGapExtractionHandoffContract) {
				handoff.RequiredRuntimeObservations = []ObservationKind{ObservationBlobSet}
			},
			wantErr: "extraction handoff must require only file/blob observations",
		},
		{
			name: "handoff includes VM state observation scope",
			mutate: func(handoff *BaseRuntimeDurableGapExtractionHandoffContract) {
				handoff.RequiredRuntimeObservations = append(handoff.RequiredRuntimeObservations, ObservationVMStateManifest)
			},
			wantErr: "extraction handoff must require only file/blob observations",
		},
		{
			name: "handoff authority drift",
			mutate: func(handoff *BaseRuntimeDurableGapExtractionHandoffContract) {
				handoff.PromotionAllowed = true
			},
			wantErr: "extraction handoff allows downstream authority",
		},
		{
			name: "handoff no mutation flag drift",
			mutate: func(handoff *BaseRuntimeDurableGapExtractionHandoffContract) {
				handoff.NoDurableComputerMutation = false
			},
			wantErr: "extraction handoff must prove no mutation",
		},
		{
			name: "handoff runtime behavior claim drift",
			mutate: func(handoff *BaseRuntimeDurableGapExtractionHandoffContract) {
				handoff.RuntimeBehaviorChanged = true
			},
			wantErr: "extraction handoff carries protected-surface or completion claims",
		},
		{
			name: "handoff protected claim drift",
			mutate: func(handoff *BaseRuntimeDurableGapExtractionHandoffContract) {
				handoff.DeployedRouteRegistered = true
			},
			wantErr: "extraction handoff carries protected-surface or completion claims",
		},
		{
			name: "handoff completion claim drift",
			mutate: func(handoff *BaseRuntimeDurableGapExtractionHandoffContract) {
				handoff.CompletionClaimed = true
			},
			wantErr: "extraction handoff carries protected-surface or completion claims",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handoff, retry, evidence := baseRuntimeDurableGapRetryHandoffContractInputs(t)
			tc.mutate(&handoff)

			contract, err := BuildBaseRuntimeDurableGapRetryHandoffContract(handoff, retry, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildBaseRuntimeDurableGapRetryHandoffContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func TestBuildBaseRuntimeDurableGapRetryHandoffContractRejectsInvalidRetry(t *testing.T) {
	foreignVersion := ComputerVersion{CodeRef: "git:foreign-base-runtime-durable-gap-retry-handoff", ArtifactProgramRef: "base-journal:owner/main@foreign-base-runtime-durable-gap-retry-handoff"}
	for _, tc := range []struct {
		name    string
		mutate  func(*BaseRuntimeEquivalenceRetryContract)
		wantErr string
	}{
		{
			name: "retry kind drift",
			mutate: func(retry *BaseRuntimeEquivalenceRetryContract) {
				retry.Kind = BaseRuntimeDurableGapRetryHandoffContractKind
			},
			wantErr: "retry kind",
		},
		{
			name: "retry boundary drift",
			mutate: func(retry *BaseRuntimeEquivalenceRetryContract) {
				retry.Boundary = BaseRuntimeDurableGapRetryHandoffBoundary
			},
			wantErr: "retry boundary",
		},
		{
			name: "retry scope drift",
			mutate: func(retry *BaseRuntimeEquivalenceRetryContract) {
				retry.Scope = BaseRuntimeDurableGapRetryHandoffScope
			},
			wantErr: "retry scope",
		},
		{
			name: "retry version drift",
			mutate: func(retry *BaseRuntimeEquivalenceRetryContract) {
				retry.Version = foreignVersion
			},
			wantErr: "retry version does not match extraction handoff",
		},
		{
			name: "retry typed artifact ref drift",
			mutate: func(retry *BaseRuntimeEquivalenceRetryContract) {
				retry.TypedArtifactProgramRef = string(foreignVersion.ArtifactProgramRef)
			},
			wantErr: "retry typed artifact-program ref does not match handoff",
		},
		{
			name: "retry missing source observation ref",
			mutate: func(retry *BaseRuntimeEquivalenceRetryContract) {
				retry.SourceObservationSetRef = "  "
			},
			wantErr: "retry refs are required",
		},
		{
			name: "retry missing runtime observation set name",
			mutate: func(retry *BaseRuntimeEquivalenceRetryContract) {
				retry.RuntimeObservationSetName = "\t"
			},
			wantErr: "retry refs are required",
		},
		{
			name: "retry source provenance alignment drift",
			mutate: func(retry *BaseRuntimeEquivalenceRetryContract) {
				retry.SourceProvenanceReadinessRef = "source-provenance:other-base"
			},
			wantErr: "retry does not match extraction handoff refs",
		},
		{
			name: "retry extraction ref alignment drift",
			mutate: func(retry *BaseRuntimeEquivalenceRetryContract) {
				retry.RuntimeFileBlobExtractionRef = "contract:other-runtime-file-blob-extraction"
			},
			wantErr: "retry does not match extraction handoff refs",
		},
		{
			name: "retry boundary ref alignment drift",
			mutate: func(retry *BaseRuntimeEquivalenceRetryContract) {
				retry.RuntimeEquivalenceBoundaryRef = "contract:other-runtime-equivalence-boundary"
			},
			wantErr: "retry does not match extraction handoff refs",
		},
		{
			name: "retry missing file manifest observation scope",
			mutate: func(retry *BaseRuntimeEquivalenceRetryContract) {
				retry.RequiredObservations = []ObservationKind{ObservationBlobSet}
			},
			wantErr: "retry must require only file/blob observations",
		},
		{
			name: "retry includes VM state observation scope",
			mutate: func(retry *BaseRuntimeEquivalenceRetryContract) {
				retry.RequiredObservations = append(retry.RequiredObservations, ObservationVMStateManifest)
			},
			wantErr: "retry must require only file/blob observations",
		},
		{
			name: "retry status drift",
			mutate: func(retry *BaseRuntimeEquivalenceRetryContract) {
				retry.RuntimeEquivalenceStatus = EquivalenceNarrowed
			},
			wantErr: "retry must be scoped equivalent",
		},
		{
			name: "retry claim missing",
			mutate: func(retry *BaseRuntimeEquivalenceRetryContract) {
				retry.RuntimeEquivalenceClaimed = false
			},
			wantErr: "retry must be scoped equivalent",
		},
		{
			name: "retry staging proof requirement missing",
			mutate: func(retry *BaseRuntimeEquivalenceRetryContract) {
				retry.StagingProofRequired = false
			},
			wantErr: "retry must preserve downstream proof requirements",
		},
		{
			name: "retry promotion proof requirement missing",
			mutate: func(retry *BaseRuntimeEquivalenceRetryContract) {
				retry.PromotionProofRequired = false
			},
			wantErr: "retry must preserve downstream proof requirements",
		},
		{
			name: "retry package proof requirement missing",
			mutate: func(retry *BaseRuntimeEquivalenceRetryContract) {
				retry.PackagePublicationProofRequired = false
			},
			wantErr: "retry must preserve downstream proof requirements",
		},
		{
			name: "retry run acceptance proof requirement missing",
			mutate: func(retry *BaseRuntimeEquivalenceRetryContract) {
				retry.RunAcceptanceProofRequired = false
			},
			wantErr: "retry must preserve downstream proof requirements",
		},
		{
			name: "retry opaque dependency flag missing",
			mutate: func(retry *BaseRuntimeEquivalenceRetryContract) {
				retry.NoOpaqueDataImageDependency = false
			},
			wantErr: "retry must prove no opaque data.img",
		},
		{
			name: "retry VM lifecycle flag missing",
			mutate: func(retry *BaseRuntimeEquivalenceRetryContract) {
				retry.NoVMLifecycleMutation = false
			},
			wantErr: "retry must prove no opaque data.img",
		},
		{
			name: "retry production flag missing",
			mutate: func(retry *BaseRuntimeEquivalenceRetryContract) {
				retry.NoProductionMutation = false
			},
			wantErr: "retry must prove no opaque data.img",
		},
		{
			name: "retry runtime behavior claim drift",
			mutate: func(retry *BaseRuntimeEquivalenceRetryContract) {
				retry.RuntimeBehaviorChanged = true
			},
			wantErr: "retry carries protected-surface or completion claims",
		},
		{
			name: "retry deployed route claim drift",
			mutate: func(retry *BaseRuntimeEquivalenceRetryContract) {
				retry.DeployedRouteRegistered = true
			},
			wantErr: "retry carries protected-surface or completion claims",
		},
		{
			name: "retry full substrate claim drift",
			mutate: func(retry *BaseRuntimeEquivalenceRetryContract) {
				retry.FullSubstrateClaimed = true
			},
			wantErr: "retry carries protected-surface or completion claims",
		},
		{
			name: "retry completion claim drift",
			mutate: func(retry *BaseRuntimeEquivalenceRetryContract) {
				retry.CompletionClaimed = true
			},
			wantErr: "retry carries protected-surface or completion claims",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handoff, retry, evidence := baseRuntimeDurableGapRetryHandoffContractInputs(t)
			tc.mutate(&retry)

			contract, err := BuildBaseRuntimeDurableGapRetryHandoffContract(handoff, retry, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildBaseRuntimeDurableGapRetryHandoffContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func TestBuildBaseRuntimeDurableGapRetryHandoffContractRejectsInvalidEvidence(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*BaseRuntimeDurableGapRetryHandoffEvidence)
		wantErr string
	}{
		{
			name: "missing extraction handoff evidence ref",
			mutate: func(evidence *BaseRuntimeDurableGapRetryHandoffEvidence) {
				evidence.RuntimeDurableGapExtractionHandoffRef = "  "
			},
			wantErr: "evidence refs are required",
		},
		{
			name: "missing retry evidence ref",
			mutate: func(evidence *BaseRuntimeDurableGapRetryHandoffEvidence) {
				evidence.RuntimeEquivalenceRetryRef = "\t"
			},
			wantErr: "evidence refs are required",
		},
		{
			name: "missing review evidence ref",
			mutate: func(evidence *BaseRuntimeDurableGapRetryHandoffEvidence) {
				evidence.RetryHandoffReviewRef = ""
			},
			wantErr: "evidence refs are required",
		},
		{
			name: "missing downstream plan evidence ref",
			mutate: func(evidence *BaseRuntimeDurableGapRetryHandoffEvidence) {
				evidence.DownstreamProofPlanRef = "  "
			},
			wantErr: "evidence refs are required",
		},
		{
			name: "missing rollback evidence ref",
			mutate: func(evidence *BaseRuntimeDurableGapRetryHandoffEvidence) {
				evidence.RollbackPlanRef = "\t"
			},
			wantErr: "evidence refs are required",
		},
		{
			name: "missing evidence no opaque dependency flag",
			mutate: func(evidence *BaseRuntimeDurableGapRetryHandoffEvidence) {
				evidence.NoOpaqueDataImageDependency = false
			},
			wantErr: "evidence must prove no opaque data.img",
		},
		{
			name: "missing evidence no VM lifecycle flag",
			mutate: func(evidence *BaseRuntimeDurableGapRetryHandoffEvidence) {
				evidence.NoVMLifecycleMutation = false
			},
			wantErr: "evidence must prove no opaque data.img",
		},
		{
			name: "missing evidence no durable mutation flag",
			mutate: func(evidence *BaseRuntimeDurableGapRetryHandoffEvidence) {
				evidence.NoDurableComputerMutation = false
			},
			wantErr: "evidence must prove no opaque data.img",
		},
		{
			name: "missing evidence no route flag",
			mutate: func(evidence *BaseRuntimeDurableGapRetryHandoffEvidence) {
				evidence.NoDeployedRouteMutation = false
			},
			wantErr: "evidence must prove no opaque data.img",
		},
		{
			name: "missing evidence no production flag",
			mutate: func(evidence *BaseRuntimeDurableGapRetryHandoffEvidence) {
				evidence.NoProductionMutation = false
			},
			wantErr: "evidence must prove no opaque data.img",
		},
		{
			name: "missing evidence no package flag",
			mutate: func(evidence *BaseRuntimeDurableGapRetryHandoffEvidence) {
				evidence.NoPackagePublicationMutation = false
			},
			wantErr: "evidence must prove no opaque data.img",
		},
		{
			name: "missing evidence no promotion flag",
			mutate: func(evidence *BaseRuntimeDurableGapRetryHandoffEvidence) {
				evidence.NoPromotionMutation = false
			},
			wantErr: "evidence must prove no opaque data.img",
		},
		{
			name: "missing evidence no run acceptance flag",
			mutate: func(evidence *BaseRuntimeDurableGapRetryHandoffEvidence) {
				evidence.NoRunAcceptanceMutation = false
			},
			wantErr: "evidence must prove no opaque data.img",
		},
		{
			name: "evidence runtime behavior changed",
			mutate: func(evidence *BaseRuntimeDurableGapRetryHandoffEvidence) {
				evidence.RuntimeBehaviorChanged = true
			},
			wantErr: "evidence carries mutation, downstream, full-substrate, or completion claims",
		},
		{
			name: "evidence durable computer mutation",
			mutate: func(evidence *BaseRuntimeDurableGapRetryHandoffEvidence) {
				evidence.DurableComputerStateMutated = true
			},
			wantErr: "evidence carries mutation, downstream, full-substrate, or completion claims",
		},
		{
			name: "evidence deployed route registration",
			mutate: func(evidence *BaseRuntimeDurableGapRetryHandoffEvidence) {
				evidence.DeployedRouteRegistered = true
			},
			wantErr: "evidence carries mutation, downstream, full-substrate, or completion claims",
		},
		{
			name: "evidence production auth touch",
			mutate: func(evidence *BaseRuntimeDurableGapRetryHandoffEvidence) {
				evidence.ProductionAuthTouched = true
			},
			wantErr: "evidence carries mutation, downstream, full-substrate, or completion claims",
		},
		{
			name: "evidence production state touch",
			mutate: func(evidence *BaseRuntimeDurableGapRetryHandoffEvidence) {
				evidence.ProductionStateMutated = true
			},
			wantErr: "evidence carries mutation, downstream, full-substrate, or completion claims",
		},
		{
			name: "evidence package publication",
			mutate: func(evidence *BaseRuntimeDurableGapRetryHandoffEvidence) {
				evidence.PackagePublished = true
			},
			wantErr: "evidence carries mutation, downstream, full-substrate, or completion claims",
		},
		{
			name: "evidence promotion execution",
			mutate: func(evidence *BaseRuntimeDurableGapRetryHandoffEvidence) {
				evidence.PromotionExecuted = true
			},
			wantErr: "evidence carries mutation, downstream, full-substrate, or completion claims",
		},
		{
			name: "evidence VM lifecycle touch",
			mutate: func(evidence *BaseRuntimeDurableGapRetryHandoffEvidence) {
				evidence.VMLifecycleTouched = true
			},
			wantErr: "evidence carries mutation, downstream, full-substrate, or completion claims",
		},
		{
			name: "evidence Firecracker boot claim",
			mutate: func(evidence *BaseRuntimeDurableGapRetryHandoffEvidence) {
				evidence.FirecrackerBootClaimed = true
			},
			wantErr: "evidence carries mutation, downstream, full-substrate, or completion claims",
		},
		{
			name: "evidence staging claim",
			mutate: func(evidence *BaseRuntimeDurableGapRetryHandoffEvidence) {
				evidence.StagingClaimed = true
			},
			wantErr: "evidence carries mutation, downstream, full-substrate, or completion claims",
		},
		{
			name: "evidence run acceptance touch",
			mutate: func(evidence *BaseRuntimeDurableGapRetryHandoffEvidence) {
				evidence.RunAcceptanceRecordTouched = true
			},
			wantErr: "evidence carries mutation, downstream, full-substrate, or completion claims",
		},
		{
			name: "evidence full substrate claim",
			mutate: func(evidence *BaseRuntimeDurableGapRetryHandoffEvidence) {
				evidence.FullSubstrateClaimed = true
			},
			wantErr: "evidence carries mutation, downstream, full-substrate, or completion claims",
		},
		{
			name: "evidence completion claim",
			mutate: func(evidence *BaseRuntimeDurableGapRetryHandoffEvidence) {
				evidence.CompletionClaimed = true
			},
			wantErr: "evidence carries mutation, downstream, full-substrate, or completion claims",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handoff, retry, evidence := baseRuntimeDurableGapRetryHandoffContractInputs(t)
			tc.mutate(&evidence)

			contract, err := BuildBaseRuntimeDurableGapRetryHandoffContract(handoff, retry, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildBaseRuntimeDurableGapRetryHandoffContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func baseRuntimeDurableGapRetryHandoffContractInputs(t *testing.T) (BaseRuntimeDurableGapExtractionHandoffContract, BaseRuntimeEquivalenceRetryContract, BaseRuntimeDurableGapRetryHandoffEvidence) {
	t.Helper()

	_, gap, extraction, extractionHandoffEvidence := baseRuntimeDurableGapExtractionHandoffContractInputs(t)
	handoff, err := BuildBaseRuntimeDurableGapExtractionHandoffContract(gap, extraction, extractionHandoffEvidence)
	if err != nil {
		t.Fatalf("BuildBaseRuntimeDurableGapExtractionHandoffContract(): %v", err)
	}

	source, _, _, _, _ := baseRuntimeEquivalenceRetryContractInputs(t)
	source.Version = handoff.Version
	source.TypedArtifactProgramRef = handoff.TypedArtifactProgramRef
	sourceObservations := baseRuntimeEquivalenceRetryObservationSet(" base-runtime-durable-gap-retry-handoff-source-observation-set-pass-125 ", handoff.Version)
	runtimeObservations := baseRuntimeEquivalenceRetryObservationSet(" base-runtime-durable-gap-retry-handoff-runtime-observation-set-pass-125 ", handoff.Version)
	retryEvidence := BaseRuntimeEquivalenceRetryEvidence{
		SourceObservationSetRef:      " observation-set:base-runtime-durable-gap-retry-handoff-source-pass-125 ",
		RuntimeFileBlobExtractionRef: " " + handoff.RuntimeFileBlobExtractionRef + " ",
		RuntimeEquivalenceRetryRef:   " equivalence:base-runtime-durable-gap-retry-handoff-pass-125 ",
		NoVMLifecycleMutation:        true,
		NoProductionMutation:         true,
		NoOpaqueDataImageDependency:  true,
	}
	retry, err := BuildBaseRuntimeEquivalenceRetryContract(source, extraction, sourceObservations, runtimeObservations, retryEvidence)
	if err != nil {
		t.Fatalf("BuildBaseRuntimeEquivalenceRetryContract(): %v", err)
	}
	if retry.SourceProvenanceReadinessRef != handoff.SourceProvenanceReadinessRef || retry.RuntimeFileBlobExtractionRef != handoff.RuntimeFileBlobExtractionRef || retry.RuntimeEquivalenceBoundaryRef != handoff.RuntimeEquivalenceBoundaryRef {
		t.Fatalf("retry refs = source %q extraction %q boundary %q, want handoff refs %q/%q/%q", retry.SourceProvenanceReadinessRef, retry.RuntimeFileBlobExtractionRef, retry.RuntimeEquivalenceBoundaryRef, handoff.SourceProvenanceReadinessRef, handoff.RuntimeFileBlobExtractionRef, handoff.RuntimeEquivalenceBoundaryRef)
	}

	evidence := BaseRuntimeDurableGapRetryHandoffEvidence{
		RuntimeDurableGapExtractionHandoffRef: " contract:base-runtime-durable-gap-extraction-handoff-pass-125 ",
		RuntimeEquivalenceRetryRef:            " " + retry.RuntimeEquivalenceRetryRef + " ",
		RetryHandoffReviewRef:                 " review:base-runtime-durable-gap-retry-handoff-pass-125 ",
		DownstreamProofPlanRef:                " plan:base-runtime-durable-gap-downstream-proof-pass-125 ",
		RollbackPlanRef:                       " rollback:base-runtime-durable-gap-retry-handoff-pass-125 ",
		NoOpaqueDataImageDependency:           true,
		NoVMLifecycleMutation:                 true,
		NoDurableComputerMutation:             true,
		NoDeployedRouteMutation:               true,
		NoProductionMutation:                  true,
		NoPackagePublicationMutation:          true,
		NoPromotionMutation:                   true,
		NoRunAcceptanceMutation:               true,
	}
	return handoff, retry, evidence
}

func assertBaseRuntimeDurableGapRetryHandoffRemainingGaps(t *testing.T, got []string) {
	t.Helper()

	want := []string{
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
	for _, gap := range got {
		if gap == BaseRuntimeDurableProofGapRuntimeFileBlobExtraction || gap == BaseRuntimeDurableProofGapRuntimeEquivalenceRetry {
			t.Fatalf("remaining gaps = %#v, must exclude closed runtime extraction/retry gaps", got)
		}
	}
}
