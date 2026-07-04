package computerversion

import (
	"strings"
	"testing"
)

func TestBuildBaseRuntimeMaterializationCeremonyContractBindsSourceReadinessToVMManagerRealization(t *testing.T) {
	source, realization, evidence := baseRuntimeMaterializationCeremonyContractInputs(t)

	contract, err := BuildBaseRuntimeMaterializationCeremonyContract(source, realization, evidence)
	if err != nil {
		t.Fatalf("BuildBaseRuntimeMaterializationCeremonyContract(): %v", err)
	}

	if contract.Kind != BaseRuntimeMaterializationCeremonyContractKind {
		t.Fatalf("kind = %q, want %q", contract.Kind, BaseRuntimeMaterializationCeremonyContractKind)
	}
	if contract.Version != source.Version || contract.Version != realization.Version || ArtifactProgramRef(contract.TypedArtifactProgramRef) != source.Version.ArtifactProgramRef {
		t.Fatalf("version/ref = %#v/%q, want shared source/realization version %#v/%#v and artifact program %q", contract.Version, contract.TypedArtifactProgramRef, source.Version, realization.Version, source.Version.ArtifactProgramRef)
	}
	if contract.Boundary != BaseRuntimeMaterializationCeremonyBoundary {
		t.Fatalf("boundary = %q, want %q", contract.Boundary, BaseRuntimeMaterializationCeremonyBoundary)
	}
	if contract.Scope != BaseRuntimeMaterializationCeremonyScope {
		t.Fatalf("scope = %q, want %q", contract.Scope, BaseRuntimeMaterializationCeremonyScope)
	}
	if contract.SourceProvenanceReadinessRef != strings.TrimSpace(evidence.SourceProvenanceReadinessRef) || contract.RealizationEvidenceRef != strings.TrimSpace(evidence.RealizationEvidenceRef) || contract.MaterializationCommandRef != strings.TrimSpace(evidence.MaterializationCommandRef) {
		t.Fatalf("evidence refs = %q/%q/%q, want trimmed refs from evidence %#v", contract.SourceProvenanceReadinessRef, contract.RealizationEvidenceRef, contract.MaterializationCommandRef, evidence)
	}
	if contract.RealizationID != realization.ID || contract.Materializer != realization.Capabilities.Materializer || contract.Substrate != VMManagerSubstrateFirecracker {
		t.Fatalf("realization binding = id %q materializer %q substrate %q, want vmmanager realization %#v", contract.RealizationID, contract.Materializer, contract.Substrate, realization)
	}
	assertObservationBundleKinds(t, contract.SourceRequiredObservations, []ObservationKind{ObservationBlobSet, ObservationFileManifest})
	assertObservationBundleKinds(t, contract.RuntimeRequiredObservations, []ObservationKind{ObservationVMStateManifest})
	if contract.RuntimeObservationSetName != realization.Observations.Name {
		t.Fatalf("runtime observation set name = %q, want %q", contract.RuntimeObservationSetName, realization.Observations.Name)
	}
	if !contract.SourceProvenanceReady || !contract.RuntimeEvidenceAccepted || !contract.RuntimeEquivalenceRequired || !contract.StagingProofRequired || !contract.PromotionProofRequired || !contract.PackagePublicationRequired {
		t.Fatalf("ceremony must accept runtime evidence while preserving downstream proof requirements: %#v", contract)
	}
	if !contract.NoVMLifecycleMutation || !contract.NoProductionMutation {
		t.Fatalf("ceremony must preserve no VM lifecycle and no production mutation flags: %#v", contract)
	}
	if contract.RuntimeBehaviorChanged || contract.DeployedRouteRegistered || contract.ProductionAuthTouched || contract.StagingClaimed || contract.PromotionClaimed || contract.VMLifecycleTouched || contract.FirecrackerBootClaimed || contract.RunAcceptanceRecordTouched || contract.PackagePublicationClaimed || contract.FullSubstrateClaimed || contract.CompletionClaimed {
		t.Fatalf("ceremony must not claim runtime behavior, protected surfaces, downstream proofs, or completion: %#v", contract)
	}
}

func TestBuildBaseRuntimeMaterializationCeremonyContractRejectsInvalidInputs(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*BaseSourceProvenanceReadinessContract, *Realization, *BaseRuntimeMaterializationCeremonyEvidence)
		wantErr string
	}{
		{
			name: "wrong source kind",
			mutate: func(source *BaseSourceProvenanceReadinessContract, _ *Realization, _ *BaseRuntimeMaterializationCeremonyEvidence) {
				source.Kind = BaseRuntimeMaterializationCeremonyContractKind
			},
			wantErr: "source contract kind",
		},
		{
			name: "wrong source boundary",
			mutate: func(source *BaseSourceProvenanceReadinessContract, _ *Realization, _ *BaseRuntimeMaterializationCeremonyEvidence) {
				source.Boundary = BaseRuntimeMaterializationCeremonyBoundary
			},
			wantErr: "source contract boundary",
		},
		{
			name: "wrong source scope",
			mutate: func(source *BaseSourceProvenanceReadinessContract, _ *Realization, _ *BaseRuntimeMaterializationCeremonyEvidence) {
				source.Scope = BaseRuntimeMaterializationCeremonyScope
			},
			wantErr: "source contract scope",
		},
		{
			name: "source typed artifact program ref drift",
			mutate: func(source *BaseSourceProvenanceReadinessContract, _ *Realization, _ *BaseRuntimeMaterializationCeremonyEvidence) {
				source.TypedArtifactProgramRef = "base-journal:owner/main@foreign-runtime-materialization"
			},
			wantErr: "source typed artifact program ref does not match version",
		},
		{
			name: "source runtime ceremony may open false",
			mutate: func(source *BaseSourceProvenanceReadinessContract, _ *Realization, _ *BaseRuntimeMaterializationCeremonyEvidence) {
				source.RuntimeCeremonyMayOpen = false
			},
			wantErr: "source contract does not open runtime ceremony",
		},
		{
			name: "source missing file manifest observation",
			mutate: func(source *BaseSourceProvenanceReadinessContract, _ *Realization, _ *BaseRuntimeMaterializationCeremonyEvidence) {
				source.RequiredObservations = []ObservationKind{ObservationBlobSet}
			},
			wantErr: "source contract must preserve file_manifest and blob_set observations",
		},
		{
			name: "source runtime proof no longer required",
			mutate: func(source *BaseSourceProvenanceReadinessContract, _ *Realization, _ *BaseRuntimeMaterializationCeremonyEvidence) {
				source.RuntimeProofRequired = false
			},
			wantErr: "source contract must preserve downstream proof requirements",
		},
		{
			name: "source no runtime materialization false",
			mutate: func(source *BaseSourceProvenanceReadinessContract, _ *Realization, _ *BaseRuntimeMaterializationCeremonyEvidence) {
				source.NoRuntimeMaterialization = false
			},
			wantErr: "source contract has unsafe proof flags",
		},
		{
			name: "source VM lifecycle touched",
			mutate: func(source *BaseSourceProvenanceReadinessContract, _ *Realization, _ *BaseRuntimeMaterializationCeremonyEvidence) {
				source.VMLifecycleTouched = true
			},
			wantErr: "source contract carries protected-surface claims",
		},
		{
			name: "source Firecracker boot claimed",
			mutate: func(source *BaseSourceProvenanceReadinessContract, _ *Realization, _ *BaseRuntimeMaterializationCeremonyEvidence) {
				source.FirecrackerBootClaimed = true
			},
			wantErr: "source contract carries protected-surface claims",
		},
		{
			name: "realization version drift",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, realization *Realization, _ *BaseRuntimeMaterializationCeremonyEvidence) {
				realization.Version = ComputerVersion{CodeRef: "git:foreign-runtime-materialization", ArtifactProgramRef: realization.Version.ArtifactProgramRef}
			},
			wantErr: "realization version does not match source readiness",
		},
		{
			name: "realization observation version drift",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, realization *Realization, _ *BaseRuntimeMaterializationCeremonyEvidence) {
				realization.Observations.Version = ComputerVersion{CodeRef: "git:foreign-runtime-observations", ArtifactProgramRef: realization.Version.ArtifactProgramRef}
			},
			wantErr: "realization observations version does not match source readiness",
		},
		{
			name: "missing realization id",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, realization *Realization, _ *BaseRuntimeMaterializationCeremonyEvidence) {
				realization.ID = "  "
			},
			wantErr: "realization id is required",
		},
		{
			name: "missing vm state manifest capability",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, realization *Realization, _ *BaseRuntimeMaterializationCeremonyEvidence) {
				realization.Capabilities.Supported = nil
			},
			wantErr: "realization must support vm_state_manifest",
		},
		{
			name: "unsupported vm state manifest capability",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, realization *Realization, _ *BaseRuntimeMaterializationCeremonyEvidence) {
				realization.Capabilities.Unsupported = append(realization.Capabilities.Unsupported, UnsupportedCapability{Kind: ObservationVMStateManifest, Reason: "fixture drift"})
			},
			wantErr: "realization must support vm_state_manifest",
		},
		{
			name: "missing realization observation set name",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, realization *Realization, _ *BaseRuntimeMaterializationCeremonyEvidence) {
				realization.Observations.Name = "\t"
			},
			wantErr: "realization observation set name is required",
		},
		{
			name: "missing vm state manifest observation requirement",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, realization *Realization, _ *BaseRuntimeMaterializationCeremonyEvidence) {
				realization.Observations.Required = nil
				realization.Observations.Observations = nil
			},
			wantErr: "realization must carry vm_state_manifest observations",
		},
		{
			name: "durable file observation in runtime realization",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, realization *Realization, _ *BaseRuntimeMaterializationCeremonyEvidence) {
				realization.Observations.Required = append(realization.Observations.Required, ObservationFileManifest)
				realization.Observations.Observations = append(realization.Observations.Observations, FileManifestObservation("/home/alice/note.txt", "sha256:file"))
			},
			wantErr: "realization cannot convert durable file/blob observations into runtime proof",
		},
		{
			name: "durable blob observation in runtime realization",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, realization *Realization, _ *BaseRuntimeMaterializationCeremonyEvidence) {
				realization.Observations.Required = append(realization.Observations.Required, ObservationBlobSet)
				realization.Observations.Observations = append(realization.Observations.Observations, Observation{Kind: ObservationBlobSet, Key: "sha256:blob", Value: "present"})
			},
			wantErr: "realization cannot convert durable file/blob observations into runtime proof",
		},
		{
			name: "runtime observation kind outside ceremony scope",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, realization *Realization, _ *BaseRuntimeMaterializationCeremonyEvidence) {
				realization.Observations.Observations = append(realization.Observations.Observations, Observation{Kind: ObservationPromotionCertificate, Key: "promotion:fake", Value: "claimed"})
				realization.Capabilities.Supported = append(realization.Capabilities.Supported, ObservationPromotionCertificate)
			},
			wantErr: "outside runtime ceremony scope",
		},
		{
			name: "missing source provenance readiness ref",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, _ *Realization, evidence *BaseRuntimeMaterializationCeremonyEvidence) {
				evidence.SourceProvenanceReadinessRef = "  "
			},
			wantErr: "source provenance readiness ref is required",
		},
		{
			name: "missing realization evidence ref",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, _ *Realization, evidence *BaseRuntimeMaterializationCeremonyEvidence) {
				evidence.RealizationEvidenceRef = ""
			},
			wantErr: "realization evidence ref is required",
		},
		{
			name: "missing materialization command ref",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, _ *Realization, evidence *BaseRuntimeMaterializationCeremonyEvidence) {
				evidence.MaterializationCommandRef = "\t"
			},
			wantErr: "materialization command ref is required",
		},
		{
			name: "evidence no VM lifecycle mutation false",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, _ *Realization, evidence *BaseRuntimeMaterializationCeremonyEvidence) {
				evidence.NoVMLifecycleMutation = false
			},
			wantErr: "evidence must prove no VM lifecycle mutation",
		},
		{
			name: "evidence no production mutation false",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, _ *Realization, evidence *BaseRuntimeMaterializationCeremonyEvidence) {
				evidence.NoProductionMutation = false
			},
			wantErr: "evidence must prove no production mutation",
		},
		{
			name: "evidence VM lifecycle touched",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, _ *Realization, evidence *BaseRuntimeMaterializationCeremonyEvidence) {
				evidence.VMLifecycleTouched = true
			},
			wantErr: "evidence carries protected-surface or completion claims",
		},
		{
			name: "evidence Firecracker boot claimed",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, _ *Realization, evidence *BaseRuntimeMaterializationCeremonyEvidence) {
				evidence.FirecrackerBootClaimed = true
			},
			wantErr: "evidence carries protected-surface or completion claims",
		},
		{
			name: "evidence staging claimed",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, _ *Realization, evidence *BaseRuntimeMaterializationCeremonyEvidence) {
				evidence.StagingClaimed = true
			},
			wantErr: "evidence carries protected-surface or completion claims",
		},
		{
			name: "evidence promotion claimed",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, _ *Realization, evidence *BaseRuntimeMaterializationCeremonyEvidence) {
				evidence.PromotionClaimed = true
			},
			wantErr: "evidence carries protected-surface or completion claims",
		},
		{
			name: "evidence package publication claimed",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, _ *Realization, evidence *BaseRuntimeMaterializationCeremonyEvidence) {
				evidence.PackagePublicationClaimed = true
			},
			wantErr: "evidence carries protected-surface or completion claims",
		},
		{
			name: "evidence run acceptance record touched",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, _ *Realization, evidence *BaseRuntimeMaterializationCeremonyEvidence) {
				evidence.RunAcceptanceRecordTouched = true
			},
			wantErr: "evidence carries protected-surface or completion claims",
		},
		{
			name: "evidence full substrate claimed",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, _ *Realization, evidence *BaseRuntimeMaterializationCeremonyEvidence) {
				evidence.FullSubstrateClaimed = true
			},
			wantErr: "evidence carries protected-surface or completion claims",
		},
		{
			name: "evidence completion claimed",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, _ *Realization, evidence *BaseRuntimeMaterializationCeremonyEvidence) {
				evidence.CompletionClaimed = true
			},
			wantErr: "evidence carries protected-surface or completion claims",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source, realization, evidence := baseRuntimeMaterializationCeremonyContractInputs(t)
			tc.mutate(&source, &realization, &evidence)

			contract, err := BuildBaseRuntimeMaterializationCeremonyContract(source, realization, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildBaseRuntimeMaterializationCeremonyContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func baseRuntimeMaterializationCeremonyContractInputs(t *testing.T) (BaseSourceProvenanceReadinessContract, Realization, BaseRuntimeMaterializationCeremonyEvidence) {
	t.Helper()

	summary, durable, sourceEvidence := baseSourceProvenanceReadinessContractInputs(t)
	source, err := BuildBaseSourceProvenanceReadinessContract(summary, durable, sourceEvidence)
	if err != nil {
		t.Fatalf("BuildBaseSourceProvenanceReadinessContract(): %v", err)
	}

	realization := mustMaterializeVMManagerBoundary(t, "base-runtime-materialization-vmmanager", source.Version, vmManagerBoundaryPath(), VMManagerCapabilityManifest("base-runtime-materialization-vmmanager"))
	evidence := BaseRuntimeMaterializationCeremonyEvidence{
		SourceProvenanceReadinessRef: " contract:base-source-provenance-readiness-pass-99 ",
		RealizationEvidenceRef:       " vmmanager:base-runtime-materialization-pass-100 ",
		MaterializationCommandRef:    " go-test:internal/computerversion/vmmanager-scoped-materializer ",
		NoVMLifecycleMutation:        true,
		NoProductionMutation:         true,
	}
	return source, realization, evidence
}
