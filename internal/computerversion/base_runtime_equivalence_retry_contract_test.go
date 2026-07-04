package computerversion

import (
	"strings"
	"testing"
)

func TestBuildBaseRuntimeEquivalenceRetryContractClaimsOnlyRuntimeFileBlobEquivalence(t *testing.T) {
	source, extraction, sourceObservations, runtimeObservations, evidence := baseRuntimeEquivalenceRetryContractInputs(t)

	contract, err := BuildBaseRuntimeEquivalenceRetryContract(source, extraction, sourceObservations, runtimeObservations, evidence)
	if err != nil {
		t.Fatalf("BuildBaseRuntimeEquivalenceRetryContract(): %v", err)
	}

	if contract.Kind != BaseRuntimeEquivalenceRetryContractKind {
		t.Fatalf("kind = %q, want %q", contract.Kind, BaseRuntimeEquivalenceRetryContractKind)
	}
	if contract.Version != source.Version || contract.Version != extraction.Version || ArtifactProgramRef(contract.TypedArtifactProgramRef) != source.Version.ArtifactProgramRef {
		t.Fatalf("version/ref = %#v/%q, want shared source/extraction version %#v/%#v and artifact program %q", contract.Version, contract.TypedArtifactProgramRef, source.Version, extraction.Version, source.Version.ArtifactProgramRef)
	}
	if contract.Boundary != BaseRuntimeEquivalenceRetryBoundary {
		t.Fatalf("boundary = %q, want %q", contract.Boundary, BaseRuntimeEquivalenceRetryBoundary)
	}
	if contract.Scope != BaseRuntimeEquivalenceRetryScope {
		t.Fatalf("scope = %q, want %q", contract.Scope, BaseRuntimeEquivalenceRetryScope)
	}
	if contract.SourceProvenanceReadinessRef != extraction.SourceProvenanceReadinessRef || contract.RuntimeEquivalenceBoundaryRef != extraction.RuntimeEquivalenceBoundaryRef {
		t.Fatalf("prior refs = source %q boundary %q, want extraction refs %q/%q", contract.SourceProvenanceReadinessRef, contract.RuntimeEquivalenceBoundaryRef, extraction.SourceProvenanceReadinessRef, extraction.RuntimeEquivalenceBoundaryRef)
	}
	if contract.SourceObservationSetRef != strings.TrimSpace(evidence.SourceObservationSetRef) || contract.RuntimeFileBlobExtractionRef != strings.TrimSpace(evidence.RuntimeFileBlobExtractionRef) || contract.RuntimeEquivalenceRetryRef != strings.TrimSpace(evidence.RuntimeEquivalenceRetryRef) {
		t.Fatalf("evidence refs = source observations %q extraction %q retry %q, want trimmed refs from %#v", contract.SourceObservationSetRef, contract.RuntimeFileBlobExtractionRef, contract.RuntimeEquivalenceRetryRef, evidence)
	}
	if contract.SourceObservationSetName != strings.TrimSpace(sourceObservations.Name) || contract.RuntimeObservationSetName != strings.TrimSpace(runtimeObservations.Name) {
		t.Fatalf("observation set names = source %q runtime %q, want %q/%q", contract.SourceObservationSetName, contract.RuntimeObservationSetName, strings.TrimSpace(sourceObservations.Name), strings.TrimSpace(runtimeObservations.Name))
	}
	assertObservationBundleKinds(t, contract.RequiredObservations, []ObservationKind{ObservationBlobSet, ObservationFileManifest})
	if contract.RuntimeEquivalenceStatus != EquivalenceEquivalent || !contract.RuntimeEquivalenceClaimed {
		t.Fatalf("runtime equivalence = status %q claimed %v, want equivalent claimed true", contract.RuntimeEquivalenceStatus, contract.RuntimeEquivalenceClaimed)
	}
	if !contract.StagingProofRequired || !contract.PromotionProofRequired || !contract.PackagePublicationProofRequired || !contract.RunAcceptanceProofRequired {
		t.Fatalf("retry contract must preserve downstream proof requirements: %#v", contract)
	}
	if !contract.NoVMLifecycleMutation || !contract.NoProductionMutation || !contract.NoOpaqueDataImageDependency {
		t.Fatalf("retry contract must carry no VM lifecycle mutation, no production mutation, and no opaque data.img dependency flags: %#v", contract)
	}
	if contract.RuntimeBehaviorChanged || contract.DeployedRouteRegistered || contract.ProductionAuthTouched || contract.StagingClaimed || contract.PromotionClaimed || contract.VMLifecycleTouched || contract.FirecrackerBootClaimed || contract.RunAcceptanceRecordTouched || contract.PackagePublicationClaimed || contract.FullSubstrateClaimed || contract.CompletionClaimed {
		t.Fatalf("retry contract must not claim runtime behavior, protected surfaces, downstream proofs, full substrate, run acceptance, or completion: %#v", contract)
	}
}

func TestBuildBaseRuntimeEquivalenceRetryContractRejectsInvalidInputs(t *testing.T) {
	foreignVersion := ComputerVersion{CodeRef: "git:foreign-runtime-equivalence-retry", ArtifactProgramRef: "base-journal:owner/main@foreign-runtime-equivalence-retry"}
	for _, tc := range []struct {
		name    string
		mutate  func(*BaseSourceProvenanceReadinessContract, *BaseRuntimeFileBlobExtractionContract, *ObservationSet, *ObservationSet, *BaseRuntimeEquivalenceRetryEvidence)
		wantErr string
	}{
		{
			name: "wrong extraction kind",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, extraction *BaseRuntimeFileBlobExtractionContract, _ *ObservationSet, _ *ObservationSet, _ *BaseRuntimeEquivalenceRetryEvidence) {
				extraction.Kind = BaseRuntimeEquivalenceRetryContractKind
			},
			wantErr: "extraction kind",
		},
		{
			name: "extraction boundary drift",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, extraction *BaseRuntimeFileBlobExtractionContract, _ *ObservationSet, _ *ObservationSet, _ *BaseRuntimeEquivalenceRetryEvidence) {
				extraction.Boundary = BaseRuntimeEquivalenceRetryBoundary
			},
			wantErr: "extraction boundary",
		},
		{
			name: "extraction scope drift",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, extraction *BaseRuntimeFileBlobExtractionContract, _ *ObservationSet, _ *ObservationSet, _ *BaseRuntimeEquivalenceRetryEvidence) {
				extraction.Scope = BaseRuntimeEquivalenceRetryScope
			},
			wantErr: "extraction scope",
		},
		{
			name: "extraction version drift",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, extraction *BaseRuntimeFileBlobExtractionContract, _ *ObservationSet, _ *ObservationSet, _ *BaseRuntimeEquivalenceRetryEvidence) {
				extraction.Version = foreignVersion
			},
			wantErr: "extraction version does not match source version",
		},
		{
			name: "extraction typed artifact ref drift",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, extraction *BaseRuntimeFileBlobExtractionContract, _ *ObservationSet, _ *ObservationSet, _ *BaseRuntimeEquivalenceRetryEvidence) {
				extraction.TypedArtifactProgramRef = string(foreignVersion.ArtifactProgramRef)
			},
			wantErr: "extraction typed artifact-program ref does not match source version",
		},
		{
			name: "extraction missing source readiness ref",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, extraction *BaseRuntimeFileBlobExtractionContract, _ *ObservationSet, _ *ObservationSet, _ *BaseRuntimeEquivalenceRetryEvidence) {
				extraction.SourceProvenanceReadinessRef = "\t"
			},
			wantErr: "extraction refs are required",
		},
		{
			name: "extraction missing runtime boundary ref",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, extraction *BaseRuntimeFileBlobExtractionContract, _ *ObservationSet, _ *ObservationSet, _ *BaseRuntimeEquivalenceRetryEvidence) {
				extraction.RuntimeEquivalenceBoundaryRef = ""
			},
			wantErr: "extraction refs are required",
		},
		{
			name: "extraction missing runtime extraction ref",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, extraction *BaseRuntimeFileBlobExtractionContract, _ *ObservationSet, _ *ObservationSet, _ *BaseRuntimeEquivalenceRetryEvidence) {
				extraction.RuntimeObservationExtractionRef = "  "
			},
			wantErr: "extraction refs are required",
		},
		{
			name: "source no longer preserves runtime proof requirement",
			mutate: func(source *BaseSourceProvenanceReadinessContract, _ *BaseRuntimeFileBlobExtractionContract, _ *ObservationSet, _ *ObservationSet, _ *BaseRuntimeEquivalenceRetryEvidence) {
				source.RuntimeProofRequired = false
			},
			wantErr: "source contract must preserve downstream proof requirements",
		},
		{
			name: "extraction runtime file blob observations not ready",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, extraction *BaseRuntimeFileBlobExtractionContract, _ *ObservationSet, _ *ObservationSet, _ *BaseRuntimeEquivalenceRetryEvidence) {
				extraction.RuntimeFileBlobObservationsReady = false
			},
			wantErr: "extraction must allow retry without claiming equivalence",
		},
		{
			name: "extraction retry gate narrowed closed",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, extraction *BaseRuntimeFileBlobExtractionContract, _ *ObservationSet, _ *ObservationSet, _ *BaseRuntimeEquivalenceRetryEvidence) {
				extraction.RuntimeEquivalenceMayBeRetried = false
			},
			wantErr: "extraction must allow retry without claiming equivalence",
		},
		{
			name: "extraction already claimed runtime equivalence",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, extraction *BaseRuntimeFileBlobExtractionContract, _ *ObservationSet, _ *ObservationSet, _ *BaseRuntimeEquivalenceRetryEvidence) {
				extraction.RuntimeEquivalenceClaimed = true
			},
			wantErr: "extraction must allow retry without claiming equivalence",
		},
		{
			name: "source runtime observation value mismatch",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, _ *BaseRuntimeFileBlobExtractionContract, sourceObservations *ObservationSet, _ *ObservationSet, _ *BaseRuntimeEquivalenceRetryEvidence) {
				sourceObservations.Observations[0] = FileManifestObservation("/workspace/base.txt", "sha256:source-drift")
			},
			wantErr: "observations are not equivalent",
		},
		{
			name: "source missing file manifest scope",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, _ *BaseRuntimeFileBlobExtractionContract, sourceObservations *ObservationSet, _ *ObservationSet, _ *BaseRuntimeEquivalenceRetryEvidence) {
				sourceObservations.Required = []ObservationKind{ObservationBlobSet}
				sourceObservations.Observations = []Observation{{Kind: ObservationBlobSet, Key: "blob:sha256:base-runtime-equivalence-retry", Value: "sha256:blob-root"}}
			},
			wantErr: "source observation set must include file_manifest and blob_set",
		},
		{
			name: "source missing blob set scope",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, _ *BaseRuntimeFileBlobExtractionContract, sourceObservations *ObservationSet, _ *ObservationSet, _ *BaseRuntimeEquivalenceRetryEvidence) {
				sourceObservations.Required = []ObservationKind{ObservationFileManifest}
				sourceObservations.Observations = []Observation{FileManifestObservation("/workspace/base.txt", "sha256:file-manifest-root")}
			},
			wantErr: "source observation set must include file_manifest and blob_set",
		},
		{
			name: "runtime missing file manifest scope",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, _ *BaseRuntimeFileBlobExtractionContract, _ *ObservationSet, runtimeObservations *ObservationSet, _ *BaseRuntimeEquivalenceRetryEvidence) {
				runtimeObservations.Required = []ObservationKind{ObservationBlobSet}
				runtimeObservations.Observations = []Observation{{Kind: ObservationBlobSet, Key: "blob:sha256:base-runtime-equivalence-retry", Value: "sha256:blob-root"}}
			},
			wantErr: "runtime observation set must include file_manifest and blob_set",
		},
		{
			name: "runtime missing blob set scope",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, _ *BaseRuntimeFileBlobExtractionContract, _ *ObservationSet, runtimeObservations *ObservationSet, _ *BaseRuntimeEquivalenceRetryEvidence) {
				runtimeObservations.Required = []ObservationKind{ObservationFileManifest}
				runtimeObservations.Observations = []Observation{FileManifestObservation("/workspace/base.txt", "sha256:file-manifest-root")}
			},
			wantErr: "runtime observation set must include file_manifest and blob_set",
		},
		{
			name: "extraction relies on vm state manifest",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, extraction *BaseRuntimeFileBlobExtractionContract, _ *ObservationSet, _ *ObservationSet, _ *BaseRuntimeEquivalenceRetryEvidence) {
				extraction.RequiredRuntimeObservations = append(extraction.RequiredRuntimeObservations, ObservationVMStateManifest)
			},
			wantErr: "extraction must require only typed file/blob runtime observations",
		},
		{
			name: "source observations rely on vm state manifest",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, _ *BaseRuntimeFileBlobExtractionContract, sourceObservations *ObservationSet, _ *ObservationSet, _ *BaseRuntimeEquivalenceRetryEvidence) {
				sourceObservations.Required = append(sourceObservations.Required, ObservationVMStateManifest)
				sourceObservations.Observations = append(sourceObservations.Observations, Observation{Kind: ObservationVMStateManifest, Key: "vm:state:manifest", Value: "sha256:opaque-runtime-state"})
			},
			wantErr: "source observation set cannot rely on vm_state_manifest",
		},
		{
			name: "runtime observations rely on vm state manifest",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, _ *BaseRuntimeFileBlobExtractionContract, _ *ObservationSet, runtimeObservations *ObservationSet, _ *BaseRuntimeEquivalenceRetryEvidence) {
				runtimeObservations.Required = append(runtimeObservations.Required, ObservationVMStateManifest)
				runtimeObservations.Observations = append(runtimeObservations.Observations, Observation{Kind: ObservationVMStateManifest, Key: "vm:state:manifest", Value: "sha256:opaque-runtime-state"})
			},
			wantErr: "runtime observation set cannot rely on vm_state_manifest",
		},
		{
			name: "source observations empty",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, _ *BaseRuntimeFileBlobExtractionContract, sourceObservations *ObservationSet, _ *ObservationSet, _ *BaseRuntimeEquivalenceRetryEvidence) {
				sourceObservations.Observations = nil
			},
			wantErr: "source observations are empty",
		},
		{
			name: "runtime observation invalid",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, _ *BaseRuntimeFileBlobExtractionContract, _ *ObservationSet, runtimeObservations *ObservationSet, _ *BaseRuntimeEquivalenceRetryEvidence) {
				runtimeObservations.Observations[0] = Observation{Kind: ObservationFileManifest, Key: "  ", Value: "sha256:file-manifest-root"}
			},
			wantErr: "invalid runtime observation",
		},
		{
			name: "source observation name missing",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, _ *BaseRuntimeFileBlobExtractionContract, sourceObservations *ObservationSet, _ *ObservationSet, _ *BaseRuntimeEquivalenceRetryEvidence) {
				sourceObservations.Name = "\t"
			},
			wantErr: "source observation set name is required",
		},
		{
			name: "runtime observation version drift",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, _ *BaseRuntimeFileBlobExtractionContract, _ *ObservationSet, runtimeObservations *ObservationSet, _ *BaseRuntimeEquivalenceRetryEvidence) {
				runtimeObservations.Version = foreignVersion
			},
			wantErr: "runtime observation version does not match source version",
		},
		{
			name: "missing source observation evidence ref",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, _ *BaseRuntimeFileBlobExtractionContract, _ *ObservationSet, _ *ObservationSet, evidence *BaseRuntimeEquivalenceRetryEvidence) {
				evidence.SourceObservationSetRef = "  "
			},
			wantErr: "evidence refs are required",
		},
		{
			name: "missing runtime extraction evidence ref",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, _ *BaseRuntimeFileBlobExtractionContract, _ *ObservationSet, _ *ObservationSet, evidence *BaseRuntimeEquivalenceRetryEvidence) {
				evidence.RuntimeFileBlobExtractionRef = ""
			},
			wantErr: "evidence refs are required",
		},
		{
			name: "missing retry evidence ref",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, _ *BaseRuntimeFileBlobExtractionContract, _ *ObservationSet, _ *ObservationSet, evidence *BaseRuntimeEquivalenceRetryEvidence) {
				evidence.RuntimeEquivalenceRetryRef = "\t"
			},
			wantErr: "evidence refs are required",
		},
		{
			name: "extraction opaque data image dependency not excluded",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, extraction *BaseRuntimeFileBlobExtractionContract, _ *ObservationSet, _ *ObservationSet, _ *BaseRuntimeEquivalenceRetryEvidence) {
				extraction.NoOpaqueDataImageDependency = false
			},
			wantErr: "extraction must reject opaque data.img, VM lifecycle mutation, and production mutation",
		},
		{
			name: "extraction no VM lifecycle mutation flag missing",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, extraction *BaseRuntimeFileBlobExtractionContract, _ *ObservationSet, _ *ObservationSet, _ *BaseRuntimeEquivalenceRetryEvidence) {
				extraction.NoVMLifecycleMutation = false
			},
			wantErr: "extraction must reject opaque data.img, VM lifecycle mutation, and production mutation",
		},
		{
			name: "extraction no production mutation flag missing",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, extraction *BaseRuntimeFileBlobExtractionContract, _ *ObservationSet, _ *ObservationSet, _ *BaseRuntimeEquivalenceRetryEvidence) {
				extraction.NoProductionMutation = false
			},
			wantErr: "extraction must reject opaque data.img, VM lifecycle mutation, and production mutation",
		},
		{
			name: "evidence opaque data image dependency not excluded",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, _ *BaseRuntimeFileBlobExtractionContract, _ *ObservationSet, _ *ObservationSet, evidence *BaseRuntimeEquivalenceRetryEvidence) {
				evidence.NoOpaqueDataImageDependency = false
			},
			wantErr: "evidence must prove no VM lifecycle mutation, production mutation, or opaque data.img dependency",
		},
		{
			name: "evidence no VM lifecycle mutation flag missing",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, _ *BaseRuntimeFileBlobExtractionContract, _ *ObservationSet, _ *ObservationSet, evidence *BaseRuntimeEquivalenceRetryEvidence) {
				evidence.NoVMLifecycleMutation = false
			},
			wantErr: "evidence must prove no VM lifecycle mutation, production mutation, or opaque data.img dependency",
		},
		{
			name: "evidence no production mutation flag missing",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, _ *BaseRuntimeFileBlobExtractionContract, _ *ObservationSet, _ *ObservationSet, evidence *BaseRuntimeEquivalenceRetryEvidence) {
				evidence.NoProductionMutation = false
			},
			wantErr: "evidence must prove no VM lifecycle mutation, production mutation, or opaque data.img dependency",
		},
		{
			name: "extraction protected surface claimed",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, extraction *BaseRuntimeFileBlobExtractionContract, _ *ObservationSet, _ *ObservationSet, _ *BaseRuntimeEquivalenceRetryEvidence) {
				extraction.StagingClaimed = true
			},
			wantErr: "extraction carries protected-surface or completion claims",
		},
		{
			name: "extraction completion claimed",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, extraction *BaseRuntimeFileBlobExtractionContract, _ *ObservationSet, _ *ObservationSet, _ *BaseRuntimeEquivalenceRetryEvidence) {
				extraction.CompletionClaimed = true
			},
			wantErr: "extraction carries protected-surface or completion claims",
		},
		{
			name: "evidence protected surface claimed",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, _ *BaseRuntimeFileBlobExtractionContract, _ *ObservationSet, _ *ObservationSet, evidence *BaseRuntimeEquivalenceRetryEvidence) {
				evidence.PackagePublicationClaimed = true
			},
			wantErr: "evidence carries protected-surface or completion claims",
		},
		{
			name: "evidence completion claimed",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, _ *BaseRuntimeFileBlobExtractionContract, _ *ObservationSet, _ *ObservationSet, evidence *BaseRuntimeEquivalenceRetryEvidence) {
				evidence.CompletionClaimed = true
			},
			wantErr: "evidence carries protected-surface or completion claims",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source, extraction, sourceObservations, runtimeObservations, evidence := baseRuntimeEquivalenceRetryContractInputs(t)
			tc.mutate(&source, &extraction, &sourceObservations, &runtimeObservations, &evidence)

			contract, err := BuildBaseRuntimeEquivalenceRetryContract(source, extraction, sourceObservations, runtimeObservations, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildBaseRuntimeEquivalenceRetryContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func baseRuntimeEquivalenceRetryContractInputs(t *testing.T) (BaseSourceProvenanceReadinessContract, BaseRuntimeFileBlobExtractionContract, ObservationSet, ObservationSet, BaseRuntimeEquivalenceRetryEvidence) {
	t.Helper()

	source, ceremony, boundaryResult, boundaryEvidence := baseRuntimeEquivalenceBoundaryContractInputs(t)
	boundary, err := BuildBaseRuntimeEquivalenceBoundaryContract(source, ceremony, boundaryResult, boundaryEvidence)
	if err != nil {
		t.Fatalf("BuildBaseRuntimeEquivalenceBoundaryContract(): %v", err)
	}

	runtimeObservations := baseRuntimeEquivalenceRetryObservationSet(" base-runtime-equivalence-retry-runtime-observation-set-pass-103 ", boundary.Version)
	extractionEvidence := BaseRuntimeFileBlobExtractionEvidence{
		RuntimeEquivalenceBoundaryRef:   " contract:base-runtime-equivalence-boundary-pass-101 ",
		RuntimeObservationExtractionRef: " observation-set:base-runtime-file-blob-extraction-pass-102 ",
		ExtractorRef:                    " extractor:typed-runtime-file-blob-pass-102 ",
		NoOpaqueDataImageDependency:     true,
		NoVMLifecycleMutation:           true,
		NoProductionMutation:            true,
	}
	extraction, err := BuildBaseRuntimeFileBlobExtractionContract(boundary, runtimeObservations, extractionEvidence)
	if err != nil {
		t.Fatalf("BuildBaseRuntimeFileBlobExtractionContract(): %v", err)
	}

	sourceObservations := baseRuntimeEquivalenceRetryObservationSet(" base-runtime-equivalence-retry-source-observation-set-pass-103 ", source.Version)
	evidence := BaseRuntimeEquivalenceRetryEvidence{
		SourceObservationSetRef:      " observation-set:base-source-file-blob-pass-103 ",
		RuntimeFileBlobExtractionRef: " contract:base-runtime-file-blob-extraction-pass-102 ",
		RuntimeEquivalenceRetryRef:   " equivalence:base-runtime-file-blob-retry-pass-103 ",
		NoVMLifecycleMutation:        true,
		NoProductionMutation:         true,
		NoOpaqueDataImageDependency:  true,
	}
	return source, extraction, sourceObservations, runtimeObservations, evidence
}

func baseRuntimeEquivalenceRetryObservationSet(name string, version ComputerVersion) ObservationSet {
	return ObservationSet{
		Name:     name,
		Version:  version,
		Required: []ObservationKind{ObservationFileManifest, ObservationBlobSet},
		Observations: []Observation{
			FileManifestObservation("/workspace/base.txt", "sha256:file-manifest-root"),
			{Kind: ObservationBlobSet, Key: "blob:sha256:base-runtime-equivalence-retry", Value: "sha256:blob-root"},
		},
	}
}
