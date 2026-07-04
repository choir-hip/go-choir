package computerversion

import (
	"strings"
	"testing"
)

func TestBuildBaseSourceMaterializerReadinessContractBuildsReadinessOnlyGate(t *testing.T) {
	probe, source, materializer, evidence := baseSourceMaterializerReadinessContractInputs(t)

	contract, err := BuildBaseSourceMaterializerReadinessContract(probe, source, materializer, evidence)
	if err != nil {
		t.Fatalf("BuildBaseSourceMaterializerReadinessContract(): %v", err)
	}

	if contract.Kind != BaseSourceMaterializerReadinessContractKind {
		t.Fatalf("kind = %q, want %q", contract.Kind, BaseSourceMaterializerReadinessContractKind)
	}
	if contract.Version != probe.Version {
		t.Fatalf("version = %#v, want probe version %#v", contract.Version, probe.Version)
	}
	if contract.Boundary != BaseSourceMaterializerReadinessBoundary {
		t.Fatalf("boundary = %q, want %q", contract.Boundary, BaseSourceMaterializerReadinessBoundary)
	}
	if contract.Scope != BaseSourceMaterializerReadinessScope {
		t.Fatalf("scope = %q, want %q", contract.Scope, BaseSourceMaterializerReadinessScope)
	}
	if contract.TypedArtifactProgramRef != strings.TrimSpace(probe.TypedArtifactProgramRef) || ArtifactProgramRef(contract.TypedArtifactProgramRef) != probe.Version.ArtifactProgramRef {
		t.Fatalf("typed artifact-program ref = %q, want probe/version ref %q/%q", contract.TypedArtifactProgramRef, probe.TypedArtifactProgramRef, probe.Version.ArtifactProgramRef)
	}
	if contract.DurableStateSliceProbeRef != strings.TrimSpace(evidence.DurableStateSliceProbeRef) || contract.SourceProvenanceReadinessRef != strings.TrimSpace(evidence.SourceProvenanceReadinessRef) || contract.MaterializerBoundaryRef != strings.TrimSpace(evidence.MaterializerBoundaryRef) {
		t.Fatalf("readiness input refs = probe %q source %q materializer %q, want trimmed evidence refs %#v", contract.DurableStateSliceProbeRef, contract.SourceProvenanceReadinessRef, contract.MaterializerBoundaryRef, evidence)
	}
	if contract.MaterializerReadinessPlanRef != strings.TrimSpace(evidence.MaterializerReadinessPlanRef) {
		t.Fatalf("materializer readiness plan ref = %q, want %q", contract.MaterializerReadinessPlanRef, strings.TrimSpace(evidence.MaterializerReadinessPlanRef))
	}
	if contract.PostPromotionSettlementHandoffRef != strings.TrimSpace(probe.PostPromotionSettlementHandoffRef) || contract.DurableStateSliceContractRef != strings.TrimSpace(probe.DurableStateSliceContractRef) {
		t.Fatalf("durable probe refs = handoff %q durable slice %q, want trimmed probe refs %#v", contract.PostPromotionSettlementHandoffRef, contract.DurableStateSliceContractRef, probe)
	}
	if contract.SourceProvenanceEvidenceRef != strings.TrimSpace(source.SourceProvenanceEvidenceRef) {
		t.Fatalf("source provenance evidence ref = %q, want %q", contract.SourceProvenanceEvidenceRef, source.SourceProvenanceEvidenceRef)
	}
	if contract.RealizationRef != strings.TrimSpace(materializer.RealizationRef) || contract.CapabilityManifestRef != strings.TrimSpace(materializer.CapabilityManifestRef) || contract.ObservationSetRef != strings.TrimSpace(materializer.ObservationSetRef) {
		t.Fatalf("materializer refs = realization %q capability %q observation %q, want trimmed materializer refs %#v", contract.RealizationRef, contract.CapabilityManifestRef, contract.ObservationSetRef, materializer)
	}
	if contract.ResidualRiskRef != strings.TrimSpace(evidence.ResidualRiskRef) || contract.RollbackPlanRef != strings.TrimSpace(evidence.RollbackPlanRef) {
		t.Fatalf("risk/rollback refs = %q/%q, want trimmed evidence refs %#v", contract.ResidualRiskRef, contract.RollbackPlanRef, evidence)
	}
	if contract.ReadinessStatus != BaseSourceMaterializerReadinessStatusReady {
		t.Fatalf("readiness status = %q, want %q", contract.ReadinessStatus, BaseSourceMaterializerReadinessStatusReady)
	}
	assertBaseDurableStateClasses(t, contract.PersistentStateClasses, []BaseDurableStateClass{BaseDurableStateClassBlobContent, BaseDurableStateClassFileManifest})
	assertObservationBundleKinds(t, contract.RequiredObservations, []ObservationKind{ObservationBlobSet, ObservationFileManifest})
	assertUserSemantics(t, contract.RequiredSemantics, []UserSemantic{UserSemanticDeletionState, UserSemanticFileContent, UserSemanticFilePath, UserSemanticFileProvenance})
	if !contract.DurableStateSliceProbeConsumed || !contract.SourceProvenanceReady || !contract.MaterializerBoundaryReady || !contract.RuntimeCeremonyMayOpen {
		t.Fatalf("contract must consume durable probe, mark source/materializer ready, and only open the runtime ceremony gate: %#v", contract)
	}
	if !contract.RuntimeProofRequired || !contract.StagingProofRequired || !contract.PromotionProofRequired || !contract.PackagePublicationRequired || !contract.RunAcceptanceProofRequired || !contract.FullSubstrateProofRequired {
		t.Fatalf("contract must preserve runtime/staging/promotion/package/run/full-substrate proof requirements: %#v", contract)
	}
	if contract.RuntimeMaterializationAllowed || contract.DurableComputerMutationAllowed || contract.PackagePublicationAllowed || contract.PromotionAllowed || contract.RunAcceptanceSynthesisAllowed {
		t.Fatalf("contract must deny runtime materialization, durable mutation, package publication, promotion, and run-acceptance authority: %#v", contract)
	}
	if !contract.NoRuntimeMaterialization || !contract.NoOpaqueDataImageDependency || !contract.NoDurableComputerMutation || !contract.NoPackagePublicationMutation || !contract.NoPromotionMutation || !contract.NoRunAcceptanceMutation || !contract.NoProductionMutation {
		t.Fatalf("contract must carry no-runtime/no-opaque/no-durable/no-package/no-promotion/no-run/no-production flags: %#v", contract)
	}
	if contract.RuntimeMaterialized || contract.DurableComputerStateMutated || contract.PackagePublished || contract.PromotionExecuted || contract.RunAcceptanceRecordTouched || contract.ProductionStateMutated || contract.FullSubstrateClaimed || contract.CompletionClaimed {
		t.Fatalf("contract must not materialize runtime, mutate durable computer or production, publish, promote, touch run acceptance, claim full-substrate, or claim completion: %#v", contract)
	}
}

func TestBuildBaseSourceMaterializerReadinessContractRejectsInvalidInputs(t *testing.T) {
	foreignVersion := ComputerVersion{CodeRef: "git:foreign-base-source-materializer-readiness", ArtifactProgramRef: "base-journal:owner/main@cursor-foreign-base-source-materializer-readiness"}
	for _, tc := range []struct {
		name    string
		mutate  func(*BaseDurableStateSliceProbeContract, *BaseSourceProvenanceReadinessContract, *BaseMaterializerBoundaryContract, *BaseSourceMaterializerReadinessEvidence)
		wantErr string
	}{
		{
			name: "probe kind drift",
			mutate: func(probe *BaseDurableStateSliceProbeContract, _ *BaseSourceProvenanceReadinessContract, _ *BaseMaterializerBoundaryContract, _ *BaseSourceMaterializerReadinessEvidence) {
				probe.Kind = BaseSourceProvenanceReadinessContractKind
			},
			wantErr: "probe kind",
		},
		{
			name: "probe boundary drift",
			mutate: func(probe *BaseDurableStateSliceProbeContract, _ *BaseSourceProvenanceReadinessContract, _ *BaseMaterializerBoundaryContract, _ *BaseSourceMaterializerReadinessEvidence) {
				probe.Boundary = BaseSourceMaterializerReadinessBoundary
			},
			wantErr: "probe boundary",
		},
		{
			name: "probe scope drift",
			mutate: func(probe *BaseDurableStateSliceProbeContract, _ *BaseSourceProvenanceReadinessContract, _ *BaseMaterializerBoundaryContract, _ *BaseSourceMaterializerReadinessEvidence) {
				probe.Scope = BaseSourceMaterializerReadinessScope
			},
			wantErr: "probe scope",
		},
		{
			name: "probe invalid version",
			mutate: func(probe *BaseDurableStateSliceProbeContract, _ *BaseSourceProvenanceReadinessContract, _ *BaseMaterializerBoundaryContract, _ *BaseSourceMaterializerReadinessEvidence) {
				probe.Version.CodeRef = "  "
			},
			wantErr: "probe version is invalid",
		},
		{
			name: "probe typed artifact ref drift",
			mutate: func(probe *BaseDurableStateSliceProbeContract, _ *BaseSourceProvenanceReadinessContract, _ *BaseMaterializerBoundaryContract, _ *BaseSourceMaterializerReadinessEvidence) {
				probe.TypedArtifactProgramRef = "base-journal:owner/main@cursor-drifted-base-source-materializer-probe"
			},
			wantErr: "probe typed artifact-program ref is invalid",
		},
		{
			name: "probe missing durable state slice ref",
			mutate: func(probe *BaseDurableStateSliceProbeContract, _ *BaseSourceProvenanceReadinessContract, _ *BaseMaterializerBoundaryContract, _ *BaseSourceMaterializerReadinessEvidence) {
				probe.DurableStateSliceContractRef = "\t"
			},
			wantErr: "probe refs are required",
		},
		{
			name: "probe status drift",
			mutate: func(probe *BaseDurableStateSliceProbeContract, _ *BaseSourceProvenanceReadinessContract, _ *BaseMaterializerBoundaryContract, _ *BaseSourceMaterializerReadinessEvidence) {
				probe.ProbeStatus = "runtime_materialized"
			},
			wantErr: "probe status",
		},
		{
			name: "probe proof record drift",
			mutate: func(probe *BaseDurableStateSliceProbeContract, _ *BaseSourceProvenanceReadinessContract, _ *BaseMaterializerBoundaryContract, _ *BaseSourceMaterializerReadinessEvidence) {
				probe.BlobContentProbeRecorded = false
			},
			wantErr: "probe must record durable state slice proof",
		},
		{
			name: "probe durable state class coverage drift",
			mutate: func(probe *BaseDurableStateSliceProbeContract, _ *BaseSourceProvenanceReadinessContract, _ *BaseMaterializerBoundaryContract, _ *BaseSourceMaterializerReadinessEvidence) {
				probe.PersistentStateClasses = []BaseDurableStateClass{BaseDurableStateClassBlobContent}
			},
			wantErr: "probe must cover file/blob durable state semantics",
		},
		{
			name: "probe observation coverage drift",
			mutate: func(probe *BaseDurableStateSliceProbeContract, _ *BaseSourceProvenanceReadinessContract, _ *BaseMaterializerBoundaryContract, _ *BaseSourceMaterializerReadinessEvidence) {
				probe.RequiredObservations = []ObservationKind{ObservationFileManifest}
			},
			wantErr: "probe must cover file/blob durable state semantics",
		},
		{
			name: "probe semantic coverage drift",
			mutate: func(probe *BaseDurableStateSliceProbeContract, _ *BaseSourceProvenanceReadinessContract, _ *BaseMaterializerBoundaryContract, _ *BaseSourceMaterializerReadinessEvidence) {
				probe.RequiredSemantics = withoutUserSemantic(probe.RequiredSemantics, UserSemanticFileProvenance)
			},
			wantErr: "probe must cover file/blob durable state semantics",
		},
		{
			name: "probe downstream proof requirement drift",
			mutate: func(probe *BaseDurableStateSliceProbeContract, _ *BaseSourceProvenanceReadinessContract, _ *BaseMaterializerBoundaryContract, _ *BaseSourceMaterializerReadinessEvidence) {
				probe.FullSubstrateProofRequired = false
			},
			wantErr: "probe must preserve downstream proof requirements",
		},
		{
			name: "probe downstream authority drift",
			mutate: func(probe *BaseDurableStateSliceProbeContract, _ *BaseSourceProvenanceReadinessContract, _ *BaseMaterializerBoundaryContract, _ *BaseSourceMaterializerReadinessEvidence) {
				probe.PackagePublicationAllowed = true
			},
			wantErr: "probe allows downstream execution",
		},
		{
			name: "probe no mutation flag drift",
			mutate: func(probe *BaseDurableStateSliceProbeContract, _ *BaseSourceProvenanceReadinessContract, _ *BaseMaterializerBoundaryContract, _ *BaseSourceMaterializerReadinessEvidence) {
				probe.NoProductionMutation = false
			},
			wantErr: "probe must prove no runtime",
		},
		{
			name: "probe claim drift",
			mutate: func(probe *BaseDurableStateSliceProbeContract, _ *BaseSourceProvenanceReadinessContract, _ *BaseMaterializerBoundaryContract, _ *BaseSourceMaterializerReadinessEvidence) {
				probe.RuntimeMaterialized = true
			},
			wantErr: "probe carries materialization",
		},
		{
			name: "source kind drift",
			mutate: func(_ *BaseDurableStateSliceProbeContract, source *BaseSourceProvenanceReadinessContract, _ *BaseMaterializerBoundaryContract, _ *BaseSourceMaterializerReadinessEvidence) {
				source.Kind = BaseDurableStateSliceProbeContractKind
			},
			wantErr: "source kind",
		},
		{
			name: "source boundary drift",
			mutate: func(_ *BaseDurableStateSliceProbeContract, source *BaseSourceProvenanceReadinessContract, _ *BaseMaterializerBoundaryContract, _ *BaseSourceMaterializerReadinessEvidence) {
				source.Boundary = BaseSourceMaterializerReadinessBoundary
			},
			wantErr: "source boundary",
		},
		{
			name: "source scope drift",
			mutate: func(_ *BaseDurableStateSliceProbeContract, source *BaseSourceProvenanceReadinessContract, _ *BaseMaterializerBoundaryContract, _ *BaseSourceMaterializerReadinessEvidence) {
				source.Scope = BaseSourceMaterializerReadinessScope
			},
			wantErr: "source scope",
		},
		{
			name: "source version drift",
			mutate: func(_ *BaseDurableStateSliceProbeContract, source *BaseSourceProvenanceReadinessContract, _ *BaseMaterializerBoundaryContract, _ *BaseSourceMaterializerReadinessEvidence) {
				source.Version = foreignVersion
			},
			wantErr: "source version does not match probe",
		},
		{
			name: "source typed artifact ref drift",
			mutate: func(_ *BaseDurableStateSliceProbeContract, source *BaseSourceProvenanceReadinessContract, _ *BaseMaterializerBoundaryContract, _ *BaseSourceMaterializerReadinessEvidence) {
				source.TypedArtifactProgramRef = "base-journal:owner/main@cursor-drifted-base-source-materializer-source"
			},
			wantErr: "source typed artifact-program ref is invalid",
		},
		{
			name: "source missing durable proof ref",
			mutate: func(_ *BaseDurableStateSliceProbeContract, source *BaseSourceProvenanceReadinessContract, _ *BaseMaterializerBoundaryContract, _ *BaseSourceMaterializerReadinessEvidence) {
				source.DurableStateSliceRef = "  "
			},
			wantErr: "source proof refs are required",
		},
		{
			name: "source durable proof ref drift",
			mutate: func(_ *BaseDurableStateSliceProbeContract, source *BaseSourceProvenanceReadinessContract, _ *BaseMaterializerBoundaryContract, _ *BaseSourceMaterializerReadinessEvidence) {
				source.DurableStateSliceRef = "contract:other-durable-state-slice"
			},
			wantErr: "source proof refs are required",
		},
		{
			name: "source readiness authority drift",
			mutate: func(_ *BaseDurableStateSliceProbeContract, source *BaseSourceProvenanceReadinessContract, _ *BaseMaterializerBoundaryContract, _ *BaseSourceMaterializerReadinessEvidence) {
				source.RuntimeCeremonyMayOpen = false
			},
			wantErr: "source contract must be ready for runtime ceremony planning",
		},
		{
			name: "source proof coverage drift",
			mutate: func(_ *BaseDurableStateSliceProbeContract, source *BaseSourceProvenanceReadinessContract, _ *BaseMaterializerBoundaryContract, _ *BaseSourceMaterializerReadinessEvidence) {
				source.RequiredObservations = []ObservationKind{ObservationBlobSet}
			},
			wantErr: "source must cover file/blob durable state semantics",
		},
		{
			name: "source downstream proof requirement drift",
			mutate: func(_ *BaseDurableStateSliceProbeContract, source *BaseSourceProvenanceReadinessContract, _ *BaseMaterializerBoundaryContract, _ *BaseSourceMaterializerReadinessEvidence) {
				source.PackagePublicationRequired = false
			},
			wantErr: "source must preserve downstream proof requirements",
		},
		{
			name: "source no mutation flag drift",
			mutate: func(_ *BaseDurableStateSliceProbeContract, source *BaseSourceProvenanceReadinessContract, _ *BaseMaterializerBoundaryContract, _ *BaseSourceMaterializerReadinessEvidence) {
				source.NoMutation = false
			},
			wantErr: "source must be no-runtime",
		},
		{
			name: "source claim drift",
			mutate: func(_ *BaseDurableStateSliceProbeContract, source *BaseSourceProvenanceReadinessContract, _ *BaseMaterializerBoundaryContract, _ *BaseSourceMaterializerReadinessEvidence) {
				source.PackagePublicationClaimed = true
			},
			wantErr: "source carries protected-surface",
		},
		{
			name: "materializer kind drift",
			mutate: func(_ *BaseDurableStateSliceProbeContract, _ *BaseSourceProvenanceReadinessContract, materializer *BaseMaterializerBoundaryContract, _ *BaseSourceMaterializerReadinessEvidence) {
				materializer.Kind = BaseSourceProvenanceReadinessContractKind
			},
			wantErr: "materializer kind",
		},
		{
			name: "materializer boundary drift",
			mutate: func(_ *BaseDurableStateSliceProbeContract, _ *BaseSourceProvenanceReadinessContract, materializer *BaseMaterializerBoundaryContract, _ *BaseSourceMaterializerReadinessEvidence) {
				materializer.Boundary = BaseSourceMaterializerReadinessBoundary
			},
			wantErr: "materializer boundary",
		},
		{
			name: "materializer scope drift",
			mutate: func(_ *BaseDurableStateSliceProbeContract, _ *BaseSourceProvenanceReadinessContract, materializer *BaseMaterializerBoundaryContract, _ *BaseSourceMaterializerReadinessEvidence) {
				materializer.Scope = BaseSourceMaterializerReadinessScope
			},
			wantErr: "materializer scope",
		},
		{
			name: "materializer version drift",
			mutate: func(_ *BaseDurableStateSliceProbeContract, _ *BaseSourceProvenanceReadinessContract, materializer *BaseMaterializerBoundaryContract, _ *BaseSourceMaterializerReadinessEvidence) {
				materializer.Version = foreignVersion
			},
			wantErr: "materializer version does not match probe",
		},
		{
			name: "materializer missing realization ref",
			mutate: func(_ *BaseDurableStateSliceProbeContract, _ *BaseSourceProvenanceReadinessContract, materializer *BaseMaterializerBoundaryContract, _ *BaseSourceMaterializerReadinessEvidence) {
				materializer.RealizationRef = "  "
			},
			wantErr: "materializer refs are required",
		},
		{
			name: "materializer observation drift",
			mutate: func(_ *BaseDurableStateSliceProbeContract, _ *BaseSourceProvenanceReadinessContract, materializer *BaseMaterializerBoundaryContract, _ *BaseSourceMaterializerReadinessEvidence) {
				materializer.RequiredObservations = []ObservationKind{ObservationFileManifest}
			},
			wantErr: "materializer must include file_manifest and blob_set observations",
		},
		{
			name: "materializer no mutation flag drift",
			mutate: func(_ *BaseDurableStateSliceProbeContract, _ *BaseSourceProvenanceReadinessContract, materializer *BaseMaterializerBoundaryContract, _ *BaseSourceMaterializerReadinessEvidence) {
				materializer.NoRuntimeMaterialization = false
			},
			wantErr: "materializer must be no-runtime",
		},
		{
			name: "materializer claim drift",
			mutate: func(_ *BaseDurableStateSliceProbeContract, _ *BaseSourceProvenanceReadinessContract, materializer *BaseMaterializerBoundaryContract, _ *BaseSourceMaterializerReadinessEvidence) {
				materializer.FullSubstrateIndependenceClaim = true
			},
			wantErr: "materializer carries protected-surface",
		},
		{
			name: "missing durable probe evidence ref",
			mutate: func(_ *BaseDurableStateSliceProbeContract, _ *BaseSourceProvenanceReadinessContract, _ *BaseMaterializerBoundaryContract, evidence *BaseSourceMaterializerReadinessEvidence) {
				evidence.DurableStateSliceProbeRef = "\t"
			},
			wantErr: "evidence refs are required",
		},
		{
			name: "missing source readiness evidence ref",
			mutate: func(_ *BaseDurableStateSliceProbeContract, _ *BaseSourceProvenanceReadinessContract, _ *BaseMaterializerBoundaryContract, evidence *BaseSourceMaterializerReadinessEvidence) {
				evidence.SourceProvenanceReadinessRef = " "
			},
			wantErr: "evidence refs are required",
		},
		{
			name: "missing materializer boundary evidence ref",
			mutate: func(_ *BaseDurableStateSliceProbeContract, _ *BaseSourceProvenanceReadinessContract, _ *BaseMaterializerBoundaryContract, evidence *BaseSourceMaterializerReadinessEvidence) {
				evidence.MaterializerBoundaryRef = ""
			},
			wantErr: "evidence refs are required",
		},
		{
			name: "missing materializer readiness plan ref",
			mutate: func(_ *BaseDurableStateSliceProbeContract, _ *BaseSourceProvenanceReadinessContract, _ *BaseMaterializerBoundaryContract, evidence *BaseSourceMaterializerReadinessEvidence) {
				evidence.MaterializerReadinessPlanRef = "\t"
			},
			wantErr: "evidence refs are required",
		},
		{
			name: "missing residual risk ref",
			mutate: func(_ *BaseDurableStateSliceProbeContract, _ *BaseSourceProvenanceReadinessContract, _ *BaseMaterializerBoundaryContract, evidence *BaseSourceMaterializerReadinessEvidence) {
				evidence.ResidualRiskRef = " "
			},
			wantErr: "evidence refs are required",
		},
		{
			name: "missing rollback plan ref",
			mutate: func(_ *BaseDurableStateSliceProbeContract, _ *BaseSourceProvenanceReadinessContract, _ *BaseMaterializerBoundaryContract, evidence *BaseSourceMaterializerReadinessEvidence) {
				evidence.RollbackPlanRef = ""
			},
			wantErr: "evidence refs are required",
		},
		{
			name: "missing evidence no runtime flag",
			mutate: func(_ *BaseDurableStateSliceProbeContract, _ *BaseSourceProvenanceReadinessContract, _ *BaseMaterializerBoundaryContract, evidence *BaseSourceMaterializerReadinessEvidence) {
				evidence.NoRuntimeMaterialization = false
			},
			wantErr: "evidence must prove no runtime",
		},
		{
			name: "missing evidence no opaque data image flag",
			mutate: func(_ *BaseDurableStateSliceProbeContract, _ *BaseSourceProvenanceReadinessContract, _ *BaseMaterializerBoundaryContract, evidence *BaseSourceMaterializerReadinessEvidence) {
				evidence.NoOpaqueDataImageDependency = false
			},
			wantErr: "evidence must prove no runtime",
		},
		{
			name: "missing evidence no durable computer mutation flag",
			mutate: func(_ *BaseDurableStateSliceProbeContract, _ *BaseSourceProvenanceReadinessContract, _ *BaseMaterializerBoundaryContract, evidence *BaseSourceMaterializerReadinessEvidence) {
				evidence.NoDurableComputerMutation = false
			},
			wantErr: "evidence must prove no runtime",
		},
		{
			name: "missing evidence no package publication flag",
			mutate: func(_ *BaseDurableStateSliceProbeContract, _ *BaseSourceProvenanceReadinessContract, _ *BaseMaterializerBoundaryContract, evidence *BaseSourceMaterializerReadinessEvidence) {
				evidence.NoPackagePublicationMutation = false
			},
			wantErr: "evidence must prove no runtime",
		},
		{
			name: "missing evidence no promotion flag",
			mutate: func(_ *BaseDurableStateSliceProbeContract, _ *BaseSourceProvenanceReadinessContract, _ *BaseMaterializerBoundaryContract, evidence *BaseSourceMaterializerReadinessEvidence) {
				evidence.NoPromotionMutation = false
			},
			wantErr: "evidence must prove no runtime",
		},
		{
			name: "missing evidence no run acceptance flag",
			mutate: func(_ *BaseDurableStateSliceProbeContract, _ *BaseSourceProvenanceReadinessContract, _ *BaseMaterializerBoundaryContract, evidence *BaseSourceMaterializerReadinessEvidence) {
				evidence.NoRunAcceptanceMutation = false
			},
			wantErr: "evidence must prove no runtime",
		},
		{
			name: "missing evidence no production flag",
			mutate: func(_ *BaseDurableStateSliceProbeContract, _ *BaseSourceProvenanceReadinessContract, _ *BaseMaterializerBoundaryContract, evidence *BaseSourceMaterializerReadinessEvidence) {
				evidence.NoProductionMutation = false
			},
			wantErr: "evidence must prove no runtime",
		},
		{
			name: "runtime materialization claim",
			mutate: func(_ *BaseDurableStateSliceProbeContract, _ *BaseSourceProvenanceReadinessContract, _ *BaseMaterializerBoundaryContract, evidence *BaseSourceMaterializerReadinessEvidence) {
				evidence.RuntimeMaterialized = true
			},
			wantErr: "evidence carries materialization",
		},
		{
			name: "durable computer mutation claim",
			mutate: func(_ *BaseDurableStateSliceProbeContract, _ *BaseSourceProvenanceReadinessContract, _ *BaseMaterializerBoundaryContract, evidence *BaseSourceMaterializerReadinessEvidence) {
				evidence.DurableComputerStateMutated = true
			},
			wantErr: "evidence carries materialization",
		},
		{
			name: "package publication claim",
			mutate: func(_ *BaseDurableStateSliceProbeContract, _ *BaseSourceProvenanceReadinessContract, _ *BaseMaterializerBoundaryContract, evidence *BaseSourceMaterializerReadinessEvidence) {
				evidence.PackagePublished = true
			},
			wantErr: "evidence carries materialization",
		},
		{
			name: "promotion execution claim",
			mutate: func(_ *BaseDurableStateSliceProbeContract, _ *BaseSourceProvenanceReadinessContract, _ *BaseMaterializerBoundaryContract, evidence *BaseSourceMaterializerReadinessEvidence) {
				evidence.PromotionExecuted = true
			},
			wantErr: "evidence carries materialization",
		},
		{
			name: "run acceptance touch claim",
			mutate: func(_ *BaseDurableStateSliceProbeContract, _ *BaseSourceProvenanceReadinessContract, _ *BaseMaterializerBoundaryContract, evidence *BaseSourceMaterializerReadinessEvidence) {
				evidence.RunAcceptanceRecordTouched = true
			},
			wantErr: "evidence carries materialization",
		},
		{
			name: "production mutation claim",
			mutate: func(_ *BaseDurableStateSliceProbeContract, _ *BaseSourceProvenanceReadinessContract, _ *BaseMaterializerBoundaryContract, evidence *BaseSourceMaterializerReadinessEvidence) {
				evidence.ProductionStateMutated = true
			},
			wantErr: "evidence carries materialization",
		},
		{
			name: "full substrate claim",
			mutate: func(_ *BaseDurableStateSliceProbeContract, _ *BaseSourceProvenanceReadinessContract, _ *BaseMaterializerBoundaryContract, evidence *BaseSourceMaterializerReadinessEvidence) {
				evidence.FullSubstrateClaimed = true
			},
			wantErr: "evidence carries materialization",
		},
		{
			name: "completion claim",
			mutate: func(_ *BaseDurableStateSliceProbeContract, _ *BaseSourceProvenanceReadinessContract, _ *BaseMaterializerBoundaryContract, evidence *BaseSourceMaterializerReadinessEvidence) {
				evidence.CompletionClaimed = true
			},
			wantErr: "evidence carries materialization",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			probe, source, materializer, evidence := baseSourceMaterializerReadinessContractInputs(t)
			tc.mutate(&probe, &source, &materializer, &evidence)

			contract, err := BuildBaseSourceMaterializerReadinessContract(probe, source, materializer, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildBaseSourceMaterializerReadinessContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func baseSourceMaterializerReadinessContractInputs(t *testing.T) (BaseDurableStateSliceProbeContract, BaseSourceProvenanceReadinessContract, BaseMaterializerBoundaryContract, BaseSourceMaterializerReadinessEvidence) {
	t.Helper()

	readiness, durable, probeEvidence := baseDurableStateSliceProbeContractInputs(t)
	probe, err := BuildBaseDurableStateSliceProbeContract(readiness, durable, probeEvidence)
	if err != nil {
		t.Fatalf("BuildBaseDurableStateSliceProbeContract(): %v", err)
	}

	summary, durableForSource, sourceEvidence := baseSourceProvenanceReadinessContractInputs(t)
	if durableForSource.Version != probe.Version {
		t.Fatalf("test fixtures must share computer version: source durable %#v probe %#v", durableForSource.Version, probe.Version)
	}
	sourceEvidence.DurableStateSliceRef = " " + probe.DurableStateSliceContractRef + " "
	sourceEvidence.TypedArtifactProgramRef = "\t" + probe.TypedArtifactProgramRef + "\t"
	source, err := BuildBaseSourceProvenanceReadinessContract(summary, durableForSource, sourceEvidence)
	if err != nil {
		t.Fatalf("BuildBaseSourceProvenanceReadinessContract(): %v", err)
	}
	if source.Version != probe.Version {
		t.Fatalf("test fixtures must produce source/probe version match: source %#v probe %#v", source.Version, probe.Version)
	}

	realization, materializerEvidence := baseMaterializerBoundaryContractInputs(t)
	realization.Version = probe.Version
	realization.Observations.Version = probe.Version
	materializer, err := BuildBaseMaterializerBoundaryContract(realization, materializerEvidence)
	if err != nil {
		t.Fatalf("BuildBaseMaterializerBoundaryContract(): %v", err)
	}

	evidence := BaseSourceMaterializerReadinessEvidence{
		DurableStateSliceProbeRef:    " durable-state-slice-probe:base-pass-120 ",
		SourceProvenanceReadinessRef: " source-provenance-readiness:base-pass-120 ",
		MaterializerBoundaryRef:      " materializer-boundary:base-pass-120 ",
		MaterializerReadinessPlanRef: " materializer-readiness-plan:base-pass-120 ",
		ResidualRiskRef:              " residual-risk:base-source-materializer-pass-120 ",
		RollbackPlanRef:              " rollback-plan:base-source-materializer-pass-120 ",
		NoRuntimeMaterialization:     true,
		NoOpaqueDataImageDependency:  true,
		NoDurableComputerMutation:    true,
		NoPackagePublicationMutation: true,
		NoPromotionMutation:          true,
		NoRunAcceptanceMutation:      true,
		NoProductionMutation:         true,
	}
	return probe, source, materializer, evidence
}
