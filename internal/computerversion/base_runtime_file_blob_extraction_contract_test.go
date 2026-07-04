package computerversion

import (
	"strings"
	"testing"
)

func TestBuildBaseRuntimeFileBlobExtractionContractBindsRuntimeFileBlobObservationsToNarrowedBoundary(t *testing.T) {
	boundary, observations, evidence := baseRuntimeFileBlobExtractionContractInputs(t)

	contract, err := BuildBaseRuntimeFileBlobExtractionContract(boundary, observations, evidence)
	if err != nil {
		t.Fatalf("BuildBaseRuntimeFileBlobExtractionContract(): %v", err)
	}

	if contract.Kind != BaseRuntimeFileBlobExtractionContractKind {
		t.Fatalf("kind = %q, want %q", contract.Kind, BaseRuntimeFileBlobExtractionContractKind)
	}
	if contract.Version != boundary.Version || ArtifactProgramRef(contract.TypedArtifactProgramRef) != boundary.Version.ArtifactProgramRef {
		t.Fatalf("version/ref = %#v/%q, want boundary version %#v and artifact program %q", contract.Version, contract.TypedArtifactProgramRef, boundary.Version, boundary.Version.ArtifactProgramRef)
	}
	if contract.Boundary != BaseRuntimeFileBlobExtractionBoundary {
		t.Fatalf("boundary = %q, want %q", contract.Boundary, BaseRuntimeFileBlobExtractionBoundary)
	}
	if contract.Scope != BaseRuntimeFileBlobExtractionScope {
		t.Fatalf("scope = %q, want %q", contract.Scope, BaseRuntimeFileBlobExtractionScope)
	}
	if contract.RuntimeEquivalenceBoundaryRef != strings.TrimSpace(evidence.RuntimeEquivalenceBoundaryRef) || contract.RuntimeObservationExtractionRef != strings.TrimSpace(evidence.RuntimeObservationExtractionRef) || contract.ExtractorRef != strings.TrimSpace(evidence.ExtractorRef) {
		t.Fatalf("evidence refs = boundary %q extraction %q extractor %q, want trimmed refs from %#v", contract.RuntimeEquivalenceBoundaryRef, contract.RuntimeObservationExtractionRef, contract.ExtractorRef, evidence)
	}
	if contract.RuntimeEquivalenceEvidenceRef != boundary.RuntimeEquivalenceEvidenceRef || contract.SourceProvenanceReadinessRef != boundary.SourceProvenanceReadinessRef {
		t.Fatalf("prior refs = equivalence %q source %q, want boundary refs %q/%q", contract.RuntimeEquivalenceEvidenceRef, contract.SourceProvenanceReadinessRef, boundary.RuntimeEquivalenceEvidenceRef, boundary.SourceProvenanceReadinessRef)
	}
	if contract.ExtractedObservationSetName != strings.TrimSpace(observations.Name) {
		t.Fatalf("extracted observation set name = %q, want %q", contract.ExtractedObservationSetName, strings.TrimSpace(observations.Name))
	}
	assertObservationBundleKinds(t, contract.RequiredRuntimeObservations, []ObservationKind{ObservationBlobSet, ObservationFileManifest})
	if !contract.RuntimeFileBlobObservationsReady || !contract.RuntimeEquivalenceMayBeRetried || contract.RuntimeEquivalenceClaimed {
		t.Fatalf("runtime retry gate = ready %v retry %v claimed %v, want ready/retry true and claimed false", contract.RuntimeFileBlobObservationsReady, contract.RuntimeEquivalenceMayBeRetried, contract.RuntimeEquivalenceClaimed)
	}
	if !contract.NoOpaqueDataImageDependency || !contract.NoVMLifecycleMutation || !contract.NoProductionMutation {
		t.Fatalf("extraction must carry no opaque data.img, no VM lifecycle mutation, and no production mutation flags: %#v", contract)
	}
	if contract.RuntimeBehaviorChanged || contract.DeployedRouteRegistered || contract.ProductionAuthTouched || contract.StagingClaimed || contract.PromotionClaimed || contract.VMLifecycleTouched || contract.FirecrackerBootClaimed || contract.RunAcceptanceRecordTouched || contract.PackagePublicationClaimed || contract.FullSubstrateClaimed || contract.CompletionClaimed {
		t.Fatalf("extraction must not claim runtime behavior, protected surfaces, downstream proofs, or completion: %#v", contract)
	}
}

func TestBuildBaseRuntimeFileBlobExtractionContractRejectsInvalidInputs(t *testing.T) {
	foreignVersion := ComputerVersion{CodeRef: "git:foreign-runtime-file-blob-extraction", ArtifactProgramRef: "base-journal:owner/main@foreign-runtime-file-blob-extraction"}
	for _, tc := range []struct {
		name    string
		mutate  func(*BaseRuntimeEquivalenceBoundaryContract, *ObservationSet, *BaseRuntimeFileBlobExtractionEvidence)
		wantErr string
	}{
		{
			name: "wrong prior boundary kind",
			mutate: func(boundary *BaseRuntimeEquivalenceBoundaryContract, _ *ObservationSet, _ *BaseRuntimeFileBlobExtractionEvidence) {
				boundary.Kind = BaseRuntimeFileBlobExtractionContractKind
			},
			wantErr: "boundary kind",
		},
		{
			name: "prior boundary string drift",
			mutate: func(boundary *BaseRuntimeEquivalenceBoundaryContract, _ *ObservationSet, _ *BaseRuntimeFileBlobExtractionEvidence) {
				boundary.Boundary = BaseRuntimeFileBlobExtractionBoundary
			},
			wantErr: "boundary contract uses",
		},
		{
			name: "wrong prior boundary scope",
			mutate: func(boundary *BaseRuntimeEquivalenceBoundaryContract, _ *ObservationSet, _ *BaseRuntimeFileBlobExtractionEvidence) {
				boundary.Scope = BaseRuntimeFileBlobExtractionScope
			},
			wantErr: "boundary scope",
		},
		{
			name: "prior boundary equivalent status",
			mutate: func(boundary *BaseRuntimeEquivalenceBoundaryContract, _ *ObservationSet, _ *BaseRuntimeFileBlobExtractionEvidence) {
				boundary.RuntimeEquivalenceStatus = EquivalenceEquivalent
			},
			wantErr: "boundary must be narrowed",
		},
		{
			name: "prior boundary narrowed flag missing",
			mutate: func(boundary *BaseRuntimeEquivalenceBoundaryContract, _ *ObservationSet, _ *BaseRuntimeFileBlobExtractionEvidence) {
				boundary.RuntimeEquivalenceNarrowed = false
			},
			wantErr: "boundary must be narrowed",
		},
		{
			name: "prior boundary already claims equivalence",
			mutate: func(boundary *BaseRuntimeEquivalenceBoundaryContract, _ *ObservationSet, _ *BaseRuntimeFileBlobExtractionEvidence) {
				boundary.RuntimeEquivalenceClaimed = true
			},
			wantErr: "boundary must be narrowed",
		},
		{
			name: "prior boundary missing unsupported file manifest",
			mutate: func(boundary *BaseRuntimeEquivalenceBoundaryContract, _ *ObservationSet, _ *BaseRuntimeFileBlobExtractionEvidence) {
				boundary.UnsupportedDurableObservations = []UnsupportedCapability{{Kind: ObservationBlobSet, Reason: "runtime evidence does not expose source blobs"}}
			},
			wantErr: "unsupported file_manifest and blob_set observations",
		},
		{
			name: "prior boundary missing unsupported blob set",
			mutate: func(boundary *BaseRuntimeEquivalenceBoundaryContract, _ *ObservationSet, _ *BaseRuntimeFileBlobExtractionEvidence) {
				boundary.UnsupportedDurableObservations = []UnsupportedCapability{{Kind: ObservationFileManifest, Reason: "runtime evidence does not expose source file manifests"}}
			},
			wantErr: "unsupported file_manifest and blob_set observations",
		},
		{
			name: "observation version drift",
			mutate: func(_ *BaseRuntimeEquivalenceBoundaryContract, observations *ObservationSet, _ *BaseRuntimeFileBlobExtractionEvidence) {
				observations.Version = foreignVersion
			},
			wantErr: "observation version does not match boundary version",
		},
		{
			name: "missing observation set name",
			mutate: func(_ *BaseRuntimeEquivalenceBoundaryContract, observations *ObservationSet, _ *BaseRuntimeFileBlobExtractionEvidence) {
				observations.Name = "\t"
			},
			wantErr: "observation set name is required",
		},
		{
			name: "empty observations",
			mutate: func(_ *BaseRuntimeEquivalenceBoundaryContract, observations *ObservationSet, _ *BaseRuntimeFileBlobExtractionEvidence) {
				observations.Observations = nil
			},
			wantErr: "observations are empty",
		},
		{
			name: "invalid observation",
			mutate: func(_ *BaseRuntimeEquivalenceBoundaryContract, observations *ObservationSet, _ *BaseRuntimeFileBlobExtractionEvidence) {
				observations.Observations = []Observation{{Kind: ObservationFileManifest, Key: "", Value: "sha256:file-manifest-root"}, {Kind: ObservationBlobSet, Key: "blob:sha256:runtime-extraction", Value: "sha256:blob-root"}}
			},
			wantErr: "invalid observation",
		},
		{
			name: "observation set missing file manifest",
			mutate: func(_ *BaseRuntimeEquivalenceBoundaryContract, observations *ObservationSet, _ *BaseRuntimeFileBlobExtractionEvidence) {
				observations.Required = []ObservationKind{ObservationBlobSet}
				observations.Observations = []Observation{{Kind: ObservationBlobSet, Key: "blob:sha256:runtime-extraction", Value: "sha256:blob-root"}}
			},
			wantErr: "observation set must include file_manifest and blob_set",
		},
		{
			name: "observation set missing blob set",
			mutate: func(_ *BaseRuntimeEquivalenceBoundaryContract, observations *ObservationSet, _ *BaseRuntimeFileBlobExtractionEvidence) {
				observations.Required = []ObservationKind{ObservationFileManifest}
				observations.Observations = []Observation{FileManifestObservation("/workspace/base.txt", "sha256:file-manifest-root")}
			},
			wantErr: "observation set must include file_manifest and blob_set",
		},
		{
			name: "vm state manifest only observations",
			mutate: func(_ *BaseRuntimeEquivalenceBoundaryContract, observations *ObservationSet, _ *BaseRuntimeFileBlobExtractionEvidence) {
				observations.Required = []ObservationKind{ObservationVMStateManifest}
				observations.Observations = []Observation{{Kind: ObservationVMStateManifest, Key: "vm:state:manifest", Value: "sha256:opaque-runtime-state"}}
			},
			wantErr: "observation set must include file_manifest and blob_set",
		},
		{
			name: "vm state manifest reliance with file blob observations",
			mutate: func(_ *BaseRuntimeEquivalenceBoundaryContract, observations *ObservationSet, _ *BaseRuntimeFileBlobExtractionEvidence) {
				observations.Required = append(observations.Required, ObservationVMStateManifest)
				observations.Observations = append(observations.Observations, Observation{Kind: ObservationVMStateManifest, Key: "vm:state:manifest", Value: "sha256:opaque-runtime-state"})
			},
			wantErr: "observation set cannot rely on vm_state_manifest",
		},
		{
			name: "missing boundary equivalence evidence ref",
			mutate: func(boundary *BaseRuntimeEquivalenceBoundaryContract, _ *ObservationSet, _ *BaseRuntimeFileBlobExtractionEvidence) {
				boundary.RuntimeEquivalenceEvidenceRef = "  "
			},
			wantErr: "boundary evidence refs are required",
		},
		{
			name: "missing extraction boundary ref",
			mutate: func(_ *BaseRuntimeEquivalenceBoundaryContract, _ *ObservationSet, evidence *BaseRuntimeFileBlobExtractionEvidence) {
				evidence.RuntimeEquivalenceBoundaryRef = "\t"
			},
			wantErr: "evidence refs are required",
		},
		{
			name: "missing runtime observation extraction ref",
			mutate: func(_ *BaseRuntimeEquivalenceBoundaryContract, _ *ObservationSet, evidence *BaseRuntimeFileBlobExtractionEvidence) {
				evidence.RuntimeObservationExtractionRef = ""
			},
			wantErr: "evidence refs are required",
		},
		{
			name: "missing extractor ref",
			mutate: func(_ *BaseRuntimeEquivalenceBoundaryContract, _ *ObservationSet, evidence *BaseRuntimeFileBlobExtractionEvidence) {
				evidence.ExtractorRef = "  "
			},
			wantErr: "evidence refs are required",
		},
		{
			name: "opaque data image dependency not excluded",
			mutate: func(_ *BaseRuntimeEquivalenceBoundaryContract, _ *ObservationSet, evidence *BaseRuntimeFileBlobExtractionEvidence) {
				evidence.NoOpaqueDataImageDependency = false
			},
			wantErr: "evidence must reject opaque data.img dependency",
		},
		{
			name: "no VM lifecycle mutation flag missing",
			mutate: func(_ *BaseRuntimeEquivalenceBoundaryContract, _ *ObservationSet, evidence *BaseRuntimeFileBlobExtractionEvidence) {
				evidence.NoVMLifecycleMutation = false
			},
			wantErr: "evidence must prove no VM lifecycle or production mutation",
		},
		{
			name: "no production mutation flag missing",
			mutate: func(_ *BaseRuntimeEquivalenceBoundaryContract, _ *ObservationSet, evidence *BaseRuntimeFileBlobExtractionEvidence) {
				evidence.NoProductionMutation = false
			},
			wantErr: "evidence must prove no VM lifecycle or production mutation",
		},
		{
			name: "prior boundary staging claimed",
			mutate: func(boundary *BaseRuntimeEquivalenceBoundaryContract, _ *ObservationSet, _ *BaseRuntimeFileBlobExtractionEvidence) {
				boundary.StagingClaimed = true
			},
			wantErr: "boundary carries protected-surface or completion claims",
		},
		{
			name: "prior boundary promotion claimed",
			mutate: func(boundary *BaseRuntimeEquivalenceBoundaryContract, _ *ObservationSet, _ *BaseRuntimeFileBlobExtractionEvidence) {
				boundary.PromotionClaimed = true
			},
			wantErr: "boundary carries protected-surface or completion claims",
		},
		{
			name: "prior boundary package publication claimed",
			mutate: func(boundary *BaseRuntimeEquivalenceBoundaryContract, _ *ObservationSet, _ *BaseRuntimeFileBlobExtractionEvidence) {
				boundary.PackagePublicationClaimed = true
			},
			wantErr: "boundary carries protected-surface or completion claims",
		},
		{
			name: "prior boundary run acceptance touched",
			mutate: func(boundary *BaseRuntimeEquivalenceBoundaryContract, _ *ObservationSet, _ *BaseRuntimeFileBlobExtractionEvidence) {
				boundary.RunAcceptanceRecordTouched = true
			},
			wantErr: "boundary carries protected-surface or completion claims",
		},
		{
			name: "prior boundary full substrate claimed",
			mutate: func(boundary *BaseRuntimeEquivalenceBoundaryContract, _ *ObservationSet, _ *BaseRuntimeFileBlobExtractionEvidence) {
				boundary.FullSubstrateClaimed = true
			},
			wantErr: "boundary carries protected-surface or completion claims",
		},
		{
			name: "prior boundary completion claimed",
			mutate: func(boundary *BaseRuntimeEquivalenceBoundaryContract, _ *ObservationSet, _ *BaseRuntimeFileBlobExtractionEvidence) {
				boundary.CompletionClaimed = true
			},
			wantErr: "boundary carries protected-surface or completion claims",
		},
		{
			name: "evidence staging claimed",
			mutate: func(_ *BaseRuntimeEquivalenceBoundaryContract, _ *ObservationSet, evidence *BaseRuntimeFileBlobExtractionEvidence) {
				evidence.StagingClaimed = true
			},
			wantErr: "evidence carries protected-surface or completion claims",
		},
		{
			name: "evidence promotion claimed",
			mutate: func(_ *BaseRuntimeEquivalenceBoundaryContract, _ *ObservationSet, evidence *BaseRuntimeFileBlobExtractionEvidence) {
				evidence.PromotionClaimed = true
			},
			wantErr: "evidence carries protected-surface or completion claims",
		},
		{
			name: "evidence package publication claimed",
			mutate: func(_ *BaseRuntimeEquivalenceBoundaryContract, _ *ObservationSet, evidence *BaseRuntimeFileBlobExtractionEvidence) {
				evidence.PackagePublicationClaimed = true
			},
			wantErr: "evidence carries protected-surface or completion claims",
		},
		{
			name: "evidence run acceptance touched",
			mutate: func(_ *BaseRuntimeEquivalenceBoundaryContract, _ *ObservationSet, evidence *BaseRuntimeFileBlobExtractionEvidence) {
				evidence.RunAcceptanceRecordTouched = true
			},
			wantErr: "evidence carries protected-surface or completion claims",
		},
		{
			name: "evidence full substrate claimed",
			mutate: func(_ *BaseRuntimeEquivalenceBoundaryContract, _ *ObservationSet, evidence *BaseRuntimeFileBlobExtractionEvidence) {
				evidence.FullSubstrateClaimed = true
			},
			wantErr: "evidence carries protected-surface or completion claims",
		},
		{
			name: "evidence completion claimed",
			mutate: func(_ *BaseRuntimeEquivalenceBoundaryContract, _ *ObservationSet, evidence *BaseRuntimeFileBlobExtractionEvidence) {
				evidence.CompletionClaimed = true
			},
			wantErr: "evidence carries protected-surface or completion claims",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			boundary, observations, evidence := baseRuntimeFileBlobExtractionContractInputs(t)
			tc.mutate(&boundary, &observations, &evidence)

			contract, err := BuildBaseRuntimeFileBlobExtractionContract(boundary, observations, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildBaseRuntimeFileBlobExtractionContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func baseRuntimeFileBlobExtractionContractInputs(t *testing.T) (BaseRuntimeEquivalenceBoundaryContract, ObservationSet, BaseRuntimeFileBlobExtractionEvidence) {
	t.Helper()

	source, ceremony, result, boundaryEvidence := baseRuntimeEquivalenceBoundaryContractInputs(t)
	boundary, err := BuildBaseRuntimeEquivalenceBoundaryContract(source, ceremony, result, boundaryEvidence)
	if err != nil {
		t.Fatalf("BuildBaseRuntimeEquivalenceBoundaryContract(): %v", err)
	}

	observations := ObservationSet{
		Name:     " base-runtime-file-blob-extraction-observation-set-pass-102 ",
		Version:  boundary.Version,
		Required: []ObservationKind{ObservationFileManifest, ObservationBlobSet},
		Observations: []Observation{
			FileManifestObservation("/workspace/base.txt", "sha256:file-manifest-root"),
			{Kind: ObservationBlobSet, Key: "blob:sha256:runtime-extraction", Value: "sha256:blob-root"},
		},
	}
	evidence := BaseRuntimeFileBlobExtractionEvidence{
		RuntimeEquivalenceBoundaryRef:   " contract:base-runtime-equivalence-boundary-pass-101 ",
		RuntimeObservationExtractionRef: " observation-set:base-runtime-file-blob-extraction-pass-102 ",
		ExtractorRef:                    " extractor:typed-runtime-file-blob-pass-102 ",
		NoOpaqueDataImageDependency:     true,
		NoVMLifecycleMutation:           true,
		NoProductionMutation:            true,
	}
	return boundary, observations, evidence
}
