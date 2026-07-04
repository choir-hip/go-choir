package computerversion

import (
	"strings"
	"testing"
)

func TestBuildBaseSourceProvenanceReadinessContractBuildsReadinessGate(t *testing.T) {
	summary, durable, evidence := baseSourceProvenanceReadinessContractInputs(t)

	contract, err := BuildBaseSourceProvenanceReadinessContract(summary, durable, evidence)
	if err != nil {
		t.Fatalf("BuildBaseSourceProvenanceReadinessContract(): %v", err)
	}

	if contract.Kind != BaseSourceProvenanceReadinessContractKind {
		t.Fatalf("kind = %q, want %q", contract.Kind, BaseSourceProvenanceReadinessContractKind)
	}
	if contract.Version != summary.Version || contract.Version != durable.Version {
		t.Fatalf("version = %#v, want shared summary/durable version %#v/%#v", contract.Version, summary.Version, durable.Version)
	}
	if contract.Boundary != BaseSourceProvenanceReadinessBoundary {
		t.Fatalf("boundary = %q, want %q", contract.Boundary, BaseSourceProvenanceReadinessBoundary)
	}
	if contract.Scope != BaseSourceProvenanceReadinessScope {
		t.Fatalf("scope = %q, want %q", contract.Scope, BaseSourceProvenanceReadinessScope)
	}
	if contract.ClaimScope != BaseSubstrateEquivalenceClaimScope || contract.ClaimScope != summary.ClaimScope {
		t.Fatalf("claim scope = %q, want base equivalence scope %q from summary", contract.ClaimScope, summary.ClaimScope)
	}
	if contract.TypedArtifactProgramRef != strings.TrimSpace(evidence.TypedArtifactProgramRef) || contract.TypedArtifactProgramRef != durable.TypedArtifactProgramRef || ArtifactProgramRef(contract.TypedArtifactProgramRef) != summary.Version.ArtifactProgramRef {
		t.Fatalf("typed artifact program ref = %q, want trimmed evidence ref bound to durable/version %q/%q", contract.TypedArtifactProgramRef, durable.TypedArtifactProgramRef, summary.Version.ArtifactProgramRef)
	}
	assertBaseDurableStateClasses(t, contract.PersistentStateClasses, []BaseDurableStateClass{BaseDurableStateClassBlobContent, BaseDurableStateClassFileManifest})
	assertObservationBundleKinds(t, contract.RequiredObservations, []ObservationKind{ObservationBlobSet, ObservationFileManifest})
	assertUserSemantics(t, contract.RequiredSemantics, []UserSemantic{UserSemanticDeletionState, UserSemanticFileContent, UserSemanticFilePath, UserSemanticFileProvenance})
	if contract.LocalProofSummaryRef != strings.TrimSpace(evidence.LocalProofSummaryRef) || contract.LocalProofSummaryRef != summary.SummaryRef {
		t.Fatalf("local proof summary ref = %q, want trimmed evidence ref matching summary ref %q", contract.LocalProofSummaryRef, summary.SummaryRef)
	}
	if contract.DurableStateSliceRef != strings.TrimSpace(evidence.DurableStateSliceRef) {
		t.Fatalf("durable state slice ref = %q, want trimmed evidence ref %q", contract.DurableStateSliceRef, strings.TrimSpace(evidence.DurableStateSliceRef))
	}
	if contract.SourceProvenanceEvidenceRef != strings.TrimSpace(evidence.SourceProvenanceEvidenceRef) {
		t.Fatalf("source provenance evidence ref = %q, want trimmed evidence ref %q", contract.SourceProvenanceEvidenceRef, strings.TrimSpace(evidence.SourceProvenanceEvidenceRef))
	}
	if !contract.LocalFileBlobProofSummarized || !contract.SourceProvenanceReady || !contract.RuntimeCeremonyMayOpen {
		t.Fatalf("readiness flags must summarize local file/blob proof and open only the runtime ceremony gate: %#v", contract)
	}
	if !contract.RuntimeProofRequired || !contract.StagingProofRequired || !contract.PromotionProofRequired || !contract.PackagePublicationRequired {
		t.Fatalf("runtime/staging/promotion/package publication proof requirements must remain required: %#v", contract)
	}
	if !contract.NoRuntimeMaterialization || !contract.NoOpaqueDataImageDependency || !contract.NoMutation {
		t.Fatalf("readiness contract must emit no-runtime/no-opaque/no-mutation flags: %#v", contract)
	}
	if contract.RuntimeBehaviorChanged || contract.DeployedRouteRegistered || contract.ProductionAuthTouched || contract.StagingClaimed || contract.PromotionClaimed || contract.VMLifecycleTouched || contract.FirecrackerBootClaimed || contract.RunAcceptanceRecordTouched || contract.PackagePublicationClaimed || contract.FullSubstrateClaimed || contract.CompletionClaimed {
		t.Fatalf("readiness must not claim runtime, staging, VM lifecycle, promotion, package publication, full substrate independence, run acceptance, or completion: %#v", contract)
	}
}

func TestBuildBaseSourceProvenanceReadinessContractRejectsInvalidInputs(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*BaseLocalSubstrateProofSummaryContract, *BaseDurableStateSliceContract, *BaseSourceProvenanceReadinessEvidence)
		wantErr string
	}{
		{
			name: "wrong summary kind",
			mutate: func(summary *BaseLocalSubstrateProofSummaryContract, _ *BaseDurableStateSliceContract, _ *BaseSourceProvenanceReadinessEvidence) {
				summary.Kind = BaseDurableStateSliceContractKind
			},
			wantErr: "summary contract kind",
		},
		{
			name: "wrong summary boundary",
			mutate: func(summary *BaseLocalSubstrateProofSummaryContract, _ *BaseDurableStateSliceContract, _ *BaseSourceProvenanceReadinessEvidence) {
				summary.Boundary = BaseDurableStateSliceBoundary
			},
			wantErr: "summary contract boundary",
		},
		{
			name: "wrong summary scope",
			mutate: func(summary *BaseLocalSubstrateProofSummaryContract, _ *BaseDurableStateSliceContract, _ *BaseSourceProvenanceReadinessEvidence) {
				summary.Scope = BaseDurableStateSliceScope
			},
			wantErr: "summary contract scope",
		},
		{
			name: "wrong summary claim scope",
			mutate: func(summary *BaseLocalSubstrateProofSummaryContract, _ *BaseDurableStateSliceContract, _ *BaseSourceProvenanceReadinessEvidence) {
				summary.ClaimScope = "full_computer"
			},
			wantErr: "summary claim scope",
		},
		{
			name: "summary missing reentry authorization",
			mutate: func(summary *BaseLocalSubstrateProofSummaryContract, _ *BaseDurableStateSliceContract, _ *BaseSourceProvenanceReadinessEvidence) {
				summary.ReentryAllowed = false
			},
			wantErr: "does not summarize local file/blob proof",
		},
		{
			name: "summary missing local file blob proof",
			mutate: func(summary *BaseLocalSubstrateProofSummaryContract, _ *BaseDurableStateSliceContract, _ *BaseSourceProvenanceReadinessEvidence) {
				summary.LocalFileBlobProofSummarized = false
			},
			wantErr: "does not summarize local file/blob proof",
		},
		{
			name: "summary missing runtime proof gap",
			mutate: func(summary *BaseLocalSubstrateProofSummaryContract, _ *BaseDurableStateSliceContract, _ *BaseSourceProvenanceReadinessEvidence) {
				summary.RemainingGaps = []string{BaseLocalSubstrateProofSummaryRemainingStagingProof, BaseLocalSubstrateProofSummaryRemainingPromotionProof}
			},
			wantErr: "must preserve runtime, staging, and promotion gaps",
		},
		{
			name: "summary runtime proof no longer required",
			mutate: func(summary *BaseLocalSubstrateProofSummaryContract, _ *BaseDurableStateSliceContract, _ *BaseSourceProvenanceReadinessEvidence) {
				summary.RuntimeSubstrateProofRequired = false
			},
			wantErr: "must preserve runtime, staging, and promotion gaps",
		},
		{
			name: "summary missing file manifest observation",
			mutate: func(summary *BaseLocalSubstrateProofSummaryContract, _ *BaseDurableStateSliceContract, _ *BaseSourceProvenanceReadinessEvidence) {
				summary.RequiredObservations = []ObservationKind{ObservationBlobSet}
			},
			wantErr: "must include file_manifest and blob_set",
		},
		{
			name: "summary no runtime materialization false",
			mutate: func(summary *BaseLocalSubstrateProofSummaryContract, _ *BaseDurableStateSliceContract, _ *BaseSourceProvenanceReadinessEvidence) {
				summary.NoRuntimeMaterialization = false
			},
			wantErr: "summary contract has unsafe proof flags",
		},
		{
			name: "summary no opaque data image dependency false",
			mutate: func(summary *BaseLocalSubstrateProofSummaryContract, _ *BaseDurableStateSliceContract, _ *BaseSourceProvenanceReadinessEvidence) {
				summary.NoOpaqueDataImageDependency = false
			},
			wantErr: "summary contract has unsafe proof flags",
		},
		{
			name: "summary no mutation false",
			mutate: func(summary *BaseLocalSubstrateProofSummaryContract, _ *BaseDurableStateSliceContract, _ *BaseSourceProvenanceReadinessEvidence) {
				summary.NoMutation = false
			},
			wantErr: "summary contract has unsafe proof flags",
		},
		{
			name: "summary runtime behavior changed",
			mutate: func(summary *BaseLocalSubstrateProofSummaryContract, _ *BaseDurableStateSliceContract, _ *BaseSourceProvenanceReadinessEvidence) {
				summary.RuntimeBehaviorChanged = true
			},
			wantErr: "summary contract carries protected-surface claims",
		},
		{
			name: "summary Firecracker boot claimed",
			mutate: func(summary *BaseLocalSubstrateProofSummaryContract, _ *BaseDurableStateSliceContract, _ *BaseSourceProvenanceReadinessEvidence) {
				summary.FirecrackerBootClaimed = true
			},
			wantErr: "summary contract carries protected-surface claims",
		},
		{
			name: "summary package publication claimed",
			mutate: func(summary *BaseLocalSubstrateProofSummaryContract, _ *BaseDurableStateSliceContract, _ *BaseSourceProvenanceReadinessEvidence) {
				summary.PackagePublicationClaimed = true
			},
			wantErr: "summary contract carries protected-surface claims",
		},
		{
			name: "summary full substrate independence claimed",
			mutate: func(summary *BaseLocalSubstrateProofSummaryContract, _ *BaseDurableStateSliceContract, _ *BaseSourceProvenanceReadinessEvidence) {
				summary.FullSubstrateIndependenceClaim = true
			},
			wantErr: "summary contract carries protected-surface claims",
		},
		{
			name: "summary completion claimed",
			mutate: func(summary *BaseLocalSubstrateProofSummaryContract, _ *BaseDurableStateSliceContract, _ *BaseSourceProvenanceReadinessEvidence) {
				summary.CompletionClaimed = true
			},
			wantErr: "summary contract carries protected-surface claims",
		},
		{
			name: "wrong durable kind",
			mutate: func(_ *BaseLocalSubstrateProofSummaryContract, durable *BaseDurableStateSliceContract, _ *BaseSourceProvenanceReadinessEvidence) {
				durable.Kind = BaseLocalSubstrateProofSummaryContractKind
			},
			wantErr: "durable contract kind",
		},
		{
			name: "wrong durable boundary",
			mutate: func(_ *BaseLocalSubstrateProofSummaryContract, durable *BaseDurableStateSliceContract, _ *BaseSourceProvenanceReadinessEvidence) {
				durable.Boundary = BaseSourceProvenanceReadinessBoundary
			},
			wantErr: "durable contract boundary",
		},
		{
			name: "wrong durable scope",
			mutate: func(_ *BaseLocalSubstrateProofSummaryContract, durable *BaseDurableStateSliceContract, _ *BaseSourceProvenanceReadinessEvidence) {
				durable.Scope = BaseSourceProvenanceReadinessScope
			},
			wantErr: "durable contract scope",
		},
		{
			name: "durable missing file manifest class",
			mutate: func(_ *BaseLocalSubstrateProofSummaryContract, durable *BaseDurableStateSliceContract, _ *BaseSourceProvenanceReadinessEvidence) {
				durable.PersistentStateClasses = []BaseDurableStateClass{BaseDurableStateClassBlobContent}
			},
			wantErr: "must cover file manifest and blob content classes",
		},
		{
			name: "durable missing blob set observation",
			mutate: func(_ *BaseLocalSubstrateProofSummaryContract, durable *BaseDurableStateSliceContract, _ *BaseSourceProvenanceReadinessEvidence) {
				durable.RequiredObservations = []ObservationKind{ObservationFileManifest}
			},
			wantErr: "must include file_manifest and blob_set",
		},
		{
			name: "durable missing provenance semantic",
			mutate: func(_ *BaseLocalSubstrateProofSummaryContract, durable *BaseDurableStateSliceContract, _ *BaseSourceProvenanceReadinessEvidence) {
				durable.RequiredSemantics = withoutUserSemantic(durable.RequiredSemantics, UserSemanticFileProvenance)
			},
			wantErr: "must cover file path, content, deletion, and provenance semantics",
		},
		{
			name: "durable missing proof refs",
			mutate: func(_ *BaseLocalSubstrateProofSummaryContract, durable *BaseDurableStateSliceContract, _ *BaseSourceProvenanceReadinessEvidence) {
				durable.EquivalenceContractRef = "  "
			},
			wantErr: "durable contract must carry proof refs",
		},
		{
			name: "durable typed artifact program ref missing",
			mutate: func(_ *BaseLocalSubstrateProofSummaryContract, durable *BaseDurableStateSliceContract, _ *BaseSourceProvenanceReadinessEvidence) {
				durable.TypedArtifactProgramRef = "  "
			},
			wantErr: "durable typed artifact program ref is invalid",
		},
		{
			name: "version drift",
			mutate: func(_ *BaseLocalSubstrateProofSummaryContract, durable *BaseDurableStateSliceContract, _ *BaseSourceProvenanceReadinessEvidence) {
				durable.Version = ComputerVersion{CodeRef: "git:foreign-source-provenance", ArtifactProgramRef: ArtifactProgramRef(durable.TypedArtifactProgramRef)}
			},
			wantErr: "summary and durable contracts name different computer versions",
		},
		{
			name: "evidence typed artifact program ref mismatches version",
			mutate: func(_ *BaseLocalSubstrateProofSummaryContract, _ *BaseDurableStateSliceContract, evidence *BaseSourceProvenanceReadinessEvidence) {
				evidence.TypedArtifactProgramRef = "base-journal:owner/main@foreign-source-provenance-readiness"
			},
			wantErr: "typed artifact program ref does not match durable contract version",
		},
		{
			name: "evidence typed artifact program ref mismatches durable contract ref",
			mutate: func(_ *BaseLocalSubstrateProofSummaryContract, durable *BaseDurableStateSliceContract, evidence *BaseSourceProvenanceReadinessEvidence) {
				durable.TypedArtifactProgramRef = "\t" + durable.TypedArtifactProgramRef + "\t"
			},
			wantErr: "typed artifact program ref does not match durable contract ref",
		},
		{
			name: "durable no opaque data image dependency false",
			mutate: func(_ *BaseLocalSubstrateProofSummaryContract, durable *BaseDurableStateSliceContract, _ *BaseSourceProvenanceReadinessEvidence) {
				durable.NoOpaqueDataImageDependency = false
			},
			wantErr: "durable contract has unsafe proof flags",
		},
		{
			name: "durable no mutation false",
			mutate: func(_ *BaseLocalSubstrateProofSummaryContract, durable *BaseDurableStateSliceContract, _ *BaseSourceProvenanceReadinessEvidence) {
				durable.NoMutation = false
			},
			wantErr: "durable contract has unsafe proof flags",
		},
		{
			name: "durable runtime behavior changed",
			mutate: func(_ *BaseLocalSubstrateProofSummaryContract, durable *BaseDurableStateSliceContract, _ *BaseSourceProvenanceReadinessEvidence) {
				durable.RuntimeBehaviorChanged = true
			},
			wantErr: "durable contract carries protected-surface claims",
		},
		{
			name: "durable staging claimed",
			mutate: func(_ *BaseLocalSubstrateProofSummaryContract, durable *BaseDurableStateSliceContract, _ *BaseSourceProvenanceReadinessEvidence) {
				durable.StagingClaimed = true
			},
			wantErr: "durable contract carries protected-surface claims",
		},
		{
			name: "durable promotion claimed",
			mutate: func(_ *BaseLocalSubstrateProofSummaryContract, durable *BaseDurableStateSliceContract, _ *BaseSourceProvenanceReadinessEvidence) {
				durable.PromotionClaimed = true
			},
			wantErr: "durable contract carries protected-surface claims",
		},
		{
			name: "durable full computer claimed",
			mutate: func(_ *BaseLocalSubstrateProofSummaryContract, durable *BaseDurableStateSliceContract, _ *BaseSourceProvenanceReadinessEvidence) {
				durable.FullComputerClaimed = true
			},
			wantErr: "durable contract carries protected-surface claims",
		},
		{
			name: "durable data image disposable claimed",
			mutate: func(_ *BaseLocalSubstrateProofSummaryContract, durable *BaseDurableStateSliceContract, _ *BaseSourceProvenanceReadinessEvidence) {
				durable.DataImageDisposableClaimed = true
			},
			wantErr: "durable contract carries protected-surface claims",
		},
		{
			name: "missing local proof summary ref",
			mutate: func(_ *BaseLocalSubstrateProofSummaryContract, _ *BaseDurableStateSliceContract, evidence *BaseSourceProvenanceReadinessEvidence) {
				evidence.LocalProofSummaryRef = "  "
			},
			wantErr: "local proof summary ref is required",
		},
		{
			name: "missing durable state slice ref",
			mutate: func(_ *BaseLocalSubstrateProofSummaryContract, _ *BaseDurableStateSliceContract, evidence *BaseSourceProvenanceReadinessEvidence) {
				evidence.DurableStateSliceRef = ""
			},
			wantErr: "durable state slice ref is required",
		},
		{
			name: "missing source provenance evidence ref",
			mutate: func(_ *BaseLocalSubstrateProofSummaryContract, _ *BaseDurableStateSliceContract, evidence *BaseSourceProvenanceReadinessEvidence) {
				evidence.SourceProvenanceEvidenceRef = "\t"
			},
			wantErr: "source provenance evidence ref is required",
		},
		{
			name: "evidence no runtime materialization false",
			mutate: func(_ *BaseLocalSubstrateProofSummaryContract, _ *BaseDurableStateSliceContract, evidence *BaseSourceProvenanceReadinessEvidence) {
				evidence.NoRuntimeMaterialization = false
			},
			wantErr: "evidence must prove no runtime materialization",
		},
		{
			name: "evidence no opaque data image dependency false",
			mutate: func(_ *BaseLocalSubstrateProofSummaryContract, _ *BaseDurableStateSliceContract, evidence *BaseSourceProvenanceReadinessEvidence) {
				evidence.NoOpaqueDataImageDependency = false
			},
			wantErr: "evidence must prove no opaque data.img dependency",
		},
		{
			name: "evidence no mutation false",
			mutate: func(_ *BaseLocalSubstrateProofSummaryContract, _ *BaseDurableStateSliceContract, evidence *BaseSourceProvenanceReadinessEvidence) {
				evidence.NoMutation = false
			},
			wantErr: "evidence must be no-mutation",
		},
		{
			name: "evidence runtime behavior changed",
			mutate: func(_ *BaseLocalSubstrateProofSummaryContract, _ *BaseDurableStateSliceContract, evidence *BaseSourceProvenanceReadinessEvidence) {
				evidence.RuntimeBehaviorChanged = true
			},
			wantErr: "evidence carries protected-surface or completion claims",
		},
		{
			name: "evidence staging claimed",
			mutate: func(_ *BaseLocalSubstrateProofSummaryContract, _ *BaseDurableStateSliceContract, evidence *BaseSourceProvenanceReadinessEvidence) {
				evidence.StagingClaimed = true
			},
			wantErr: "evidence carries protected-surface or completion claims",
		},
		{
			name: "evidence promotion claimed",
			mutate: func(_ *BaseLocalSubstrateProofSummaryContract, _ *BaseDurableStateSliceContract, evidence *BaseSourceProvenanceReadinessEvidence) {
				evidence.PromotionClaimed = true
			},
			wantErr: "evidence carries protected-surface or completion claims",
		},
		{
			name: "evidence VM lifecycle touched",
			mutate: func(_ *BaseLocalSubstrateProofSummaryContract, _ *BaseDurableStateSliceContract, evidence *BaseSourceProvenanceReadinessEvidence) {
				evidence.VMLifecycleTouched = true
			},
			wantErr: "evidence carries protected-surface or completion claims",
		},
		{
			name: "evidence Firecracker boot claimed",
			mutate: func(_ *BaseLocalSubstrateProofSummaryContract, _ *BaseDurableStateSliceContract, evidence *BaseSourceProvenanceReadinessEvidence) {
				evidence.FirecrackerBootClaimed = true
			},
			wantErr: "evidence carries protected-surface or completion claims",
		},
		{
			name: "evidence run acceptance record touched",
			mutate: func(_ *BaseLocalSubstrateProofSummaryContract, _ *BaseDurableStateSliceContract, evidence *BaseSourceProvenanceReadinessEvidence) {
				evidence.RunAcceptanceRecordTouched = true
			},
			wantErr: "evidence carries protected-surface or completion claims",
		},
		{
			name: "evidence package publication claimed",
			mutate: func(_ *BaseLocalSubstrateProofSummaryContract, _ *BaseDurableStateSliceContract, evidence *BaseSourceProvenanceReadinessEvidence) {
				evidence.PackagePublicationClaimed = true
			},
			wantErr: "evidence carries protected-surface or completion claims",
		},
		{
			name: "evidence full substrate claimed",
			mutate: func(_ *BaseLocalSubstrateProofSummaryContract, _ *BaseDurableStateSliceContract, evidence *BaseSourceProvenanceReadinessEvidence) {
				evidence.FullSubstrateClaimed = true
			},
			wantErr: "evidence carries protected-surface or completion claims",
		},
		{
			name: "evidence completion claimed",
			mutate: func(_ *BaseLocalSubstrateProofSummaryContract, _ *BaseDurableStateSliceContract, evidence *BaseSourceProvenanceReadinessEvidence) {
				evidence.CompletionClaimed = true
			},
			wantErr: "evidence carries protected-surface or completion claims",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			summary, durable, evidence := baseSourceProvenanceReadinessContractInputs(t)
			tc.mutate(&summary, &durable, &evidence)

			contract, err := BuildBaseSourceProvenanceReadinessContract(summary, durable, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildBaseSourceProvenanceReadinessContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func baseSourceProvenanceReadinessContractInputs(t *testing.T) (BaseLocalSubstrateProofSummaryContract, BaseDurableStateSliceContract, BaseSourceProvenanceReadinessEvidence) {
	t.Helper()

	substrate, reentry, summaryEvidence := baseLocalSubstrateProofSummaryContractInputs(t)
	summary, err := BuildBaseLocalSubstrateProofSummaryContract(substrate, reentry, summaryEvidence)
	if err != nil {
		t.Fatalf("BuildBaseLocalSubstrateProofSummaryContract(): %v", err)
	}

	equivalence, user, durableEvidence := baseDurableStateSliceContractInputs(t)
	durable, err := BuildBaseDurableStateSliceContract(equivalence, user, durableEvidence)
	if err != nil {
		t.Fatalf("BuildBaseDurableStateSliceContract(): %v", err)
	}
	if summary.Version != durable.Version {
		t.Fatalf("test fixtures must share computer version: summary %#v durable %#v", summary.Version, durable.Version)
	}

	evidence := BaseSourceProvenanceReadinessEvidence{
		LocalProofSummaryRef:        "  " + summary.SummaryRef + "  ",
		DurableStateSliceRef:        " contract:base-durable-state-slice-pass-98 ",
		TypedArtifactProgramRef:     "\t" + durable.TypedArtifactProgramRef + "\t",
		SourceProvenanceEvidenceRef: " source-provenance:base-file-blob-pass-98 ",
		NoRuntimeMaterialization:    true,
		NoOpaqueDataImageDependency: true,
		NoMutation:                  true,
	}
	return summary, durable, evidence
}
