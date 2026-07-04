package computerversion

import (
	"strings"
	"testing"
)

func TestBuildBaseRuntimeEquivalenceBoundaryContractAcceptsOnlyNarrowedRuntimeEvidence(t *testing.T) {
	source, ceremony, result, evidence := baseRuntimeEquivalenceBoundaryContractInputs(t)

	contract, err := BuildBaseRuntimeEquivalenceBoundaryContract(source, ceremony, result, evidence)
	if err != nil {
		t.Fatalf("BuildBaseRuntimeEquivalenceBoundaryContract(): %v", err)
	}

	if contract.Kind != BaseRuntimeEquivalenceBoundaryContractKind {
		t.Fatalf("kind = %q, want %q", contract.Kind, BaseRuntimeEquivalenceBoundaryContractKind)
	}
	if contract.Version != source.Version || contract.Version != ceremony.Version || ArtifactProgramRef(contract.TypedArtifactProgramRef) != source.Version.ArtifactProgramRef {
		t.Fatalf("version/ref = %#v/%q, want shared source/ceremony version %#v/%#v and artifact program %q", contract.Version, contract.TypedArtifactProgramRef, source.Version, ceremony.Version, source.Version.ArtifactProgramRef)
	}
	if contract.Boundary != BaseRuntimeEquivalenceBoundary {
		t.Fatalf("boundary = %q, want %q", contract.Boundary, BaseRuntimeEquivalenceBoundary)
	}
	if contract.Scope != BaseRuntimeEquivalenceScope {
		t.Fatalf("scope = %q, want %q", contract.Scope, BaseRuntimeEquivalenceScope)
	}
	if contract.RuntimeMaterializationCeremonyRef != strings.TrimSpace(evidence.RuntimeMaterializationCeremonyRef) || contract.RuntimeEquivalenceEvidenceRef != strings.TrimSpace(evidence.RuntimeEquivalenceEvidenceRef) {
		t.Fatalf("runtime refs = ceremony %q equivalence %q, want trimmed refs from %#v", contract.RuntimeMaterializationCeremonyRef, contract.RuntimeEquivalenceEvidenceRef, evidence)
	}
	if contract.SourceProvenanceReadinessRef != ceremony.SourceProvenanceReadinessRef || contract.RealizationEvidenceRef != ceremony.RealizationEvidenceRef {
		t.Fatalf("ceremony proof refs = source %q realization %q, want %q/%q", contract.SourceProvenanceReadinessRef, contract.RealizationEvidenceRef, ceremony.SourceProvenanceReadinessRef, ceremony.RealizationEvidenceRef)
	}
	if contract.Materializer != ceremony.Materializer || contract.Substrate != ceremony.Substrate {
		t.Fatalf("runtime identity = %q/%q, want ceremony %q/%q", contract.Materializer, contract.Substrate, ceremony.Materializer, ceremony.Substrate)
	}
	assertObservationBundleKinds(t, contract.SourceRequiredObservations, []ObservationKind{ObservationBlobSet, ObservationFileManifest})
	assertObservationBundleKinds(t, contract.RuntimeRequiredObservations, []ObservationKind{ObservationVMStateManifest})
	assertUnsupportedDurableObservations(t, contract.UnsupportedDurableObservations, []ObservationKind{ObservationFileManifest, ObservationBlobSet})
	if contract.RuntimeEquivalenceStatus != EquivalenceNarrowed || !contract.RuntimeEquivalenceNarrowed || contract.RuntimeEquivalenceClaimed {
		t.Fatalf("runtime equivalence boundary = status %q narrowed %v claimed %v, want narrowed true and claimed false", contract.RuntimeEquivalenceStatus, contract.RuntimeEquivalenceNarrowed, contract.RuntimeEquivalenceClaimed)
	}
	if !contract.DurableStateEquivalenceRequired || !contract.StagingProofRequired || !contract.PromotionProofRequired || !contract.PackagePublicationRequired {
		t.Fatalf("boundary must preserve durable-state and downstream proof requirements: %#v", contract)
	}
	if !contract.NoVMLifecycleMutation || !contract.NoProductionMutation {
		t.Fatalf("boundary must preserve no VM lifecycle and no production mutation flags: %#v", contract)
	}
	if contract.RuntimeBehaviorChanged || contract.DeployedRouteRegistered || contract.ProductionAuthTouched || contract.StagingClaimed || contract.PromotionClaimed || contract.VMLifecycleTouched || contract.FirecrackerBootClaimed || contract.RunAcceptanceRecordTouched || contract.PackagePublicationClaimed || contract.FullSubstrateClaimed || contract.CompletionClaimed {
		t.Fatalf("boundary must not claim runtime behavior, protected surfaces, downstream proofs, or completion: %#v", contract)
	}
}

func TestBuildBaseRuntimeEquivalenceBoundaryContractRejectsInvalidInputs(t *testing.T) {
	foreignVersion := ComputerVersion{CodeRef: "git:foreign-runtime-equivalence-boundary", ArtifactProgramRef: "base-journal:owner/main@foreign-runtime-equivalence-boundary"}
	for _, tc := range []struct {
		name    string
		mutate  func(*BaseSourceProvenanceReadinessContract, *BaseRuntimeMaterializationCeremonyContract, *EquivalenceResult, *BaseRuntimeEquivalenceBoundaryEvidence)
		wantErr string
	}{
		{
			name: "wrong source kind",
			mutate: func(source *BaseSourceProvenanceReadinessContract, _ *BaseRuntimeMaterializationCeremonyContract, _ *EquivalenceResult, _ *BaseRuntimeEquivalenceBoundaryEvidence) {
				source.Kind = BaseRuntimeEquivalenceBoundaryContractKind
			},
			wantErr: "source contract kind",
		},
		{
			name: "wrong source boundary",
			mutate: func(source *BaseSourceProvenanceReadinessContract, _ *BaseRuntimeMaterializationCeremonyContract, _ *EquivalenceResult, _ *BaseRuntimeEquivalenceBoundaryEvidence) {
				source.Boundary = BaseRuntimeEquivalenceBoundary
			},
			wantErr: "source contract boundary",
		},
		{
			name: "wrong source scope",
			mutate: func(source *BaseSourceProvenanceReadinessContract, _ *BaseRuntimeMaterializationCeremonyContract, _ *EquivalenceResult, _ *BaseRuntimeEquivalenceBoundaryEvidence) {
				source.Scope = BaseRuntimeEquivalenceScope
			},
			wantErr: "source contract scope",
		},
		{
			name: "wrong ceremony kind",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, ceremony *BaseRuntimeMaterializationCeremonyContract, _ *EquivalenceResult, _ *BaseRuntimeEquivalenceBoundaryEvidence) {
				ceremony.Kind = BaseRuntimeEquivalenceBoundaryContractKind
			},
			wantErr: "ceremony contract kind",
		},
		{
			name: "wrong ceremony boundary",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, ceremony *BaseRuntimeMaterializationCeremonyContract, _ *EquivalenceResult, _ *BaseRuntimeEquivalenceBoundaryEvidence) {
				ceremony.Boundary = BaseRuntimeEquivalenceBoundary
			},
			wantErr: "ceremony contract boundary",
		},
		{
			name: "wrong ceremony scope",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, ceremony *BaseRuntimeMaterializationCeremonyContract, _ *EquivalenceResult, _ *BaseRuntimeEquivalenceBoundaryEvidence) {
				ceremony.Scope = BaseRuntimeEquivalenceScope
			},
			wantErr: "ceremony contract scope",
		},
		{
			name: "source version drift from ceremony",
			mutate: func(source *BaseSourceProvenanceReadinessContract, _ *BaseRuntimeMaterializationCeremonyContract, _ *EquivalenceResult, _ *BaseRuntimeEquivalenceBoundaryEvidence) {
				source.Version = foreignVersion
				source.TypedArtifactProgramRef = string(foreignVersion.ArtifactProgramRef)
			},
			wantErr: "ceremony version does not match source readiness",
		},
		{
			name: "ceremony version drift from source",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, ceremony *BaseRuntimeMaterializationCeremonyContract, _ *EquivalenceResult, _ *BaseRuntimeEquivalenceBoundaryEvidence) {
				ceremony.Version = foreignVersion
			},
			wantErr: "ceremony version does not match source readiness",
		},
		{
			name: "missing ceremony source proof ref",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, ceremony *BaseRuntimeMaterializationCeremonyContract, _ *EquivalenceResult, _ *BaseRuntimeEquivalenceBoundaryEvidence) {
				ceremony.SourceProvenanceReadinessRef = "  "
			},
			wantErr: "ceremony proof refs are required",
		},
		{
			name: "missing ceremony realization proof ref",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, ceremony *BaseRuntimeMaterializationCeremonyContract, _ *EquivalenceResult, _ *BaseRuntimeEquivalenceBoundaryEvidence) {
				ceremony.RealizationEvidenceRef = ""
			},
			wantErr: "ceremony proof refs are required",
		},
		{
			name: "ceremony runtime evidence not accepted",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, ceremony *BaseRuntimeMaterializationCeremonyContract, _ *EquivalenceResult, _ *BaseRuntimeEquivalenceBoundaryEvidence) {
				ceremony.RuntimeEvidenceAccepted = false
			},
			wantErr: "ceremony does not carry accepted runtime evidence",
		},
		{
			name: "missing vm state manifest runtime requirement",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, ceremony *BaseRuntimeMaterializationCeremonyContract, _ *EquivalenceResult, _ *BaseRuntimeEquivalenceBoundaryEvidence) {
				ceremony.RuntimeRequiredObservations = nil
			},
			wantErr: "ceremony must preserve runtime vm_state_manifest requirement",
		},
		{
			name: "equivalent result",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, _ *BaseRuntimeMaterializationCeremonyContract, result *EquivalenceResult, _ *BaseRuntimeEquivalenceBoundaryEvidence) {
				result.Status = EquivalenceEquivalent
				result.Unsupported = nil
			},
			wantErr: "runtime equivalence status",
		},
		{
			name: "not equivalent result",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, _ *BaseRuntimeMaterializationCeremonyContract, result *EquivalenceResult, _ *BaseRuntimeEquivalenceBoundaryEvidence) {
				result.Status = EquivalenceNotEquivalent
				result.Unsupported = nil
				result.Differences = []Difference{{Kind: ObservationFileManifest, Key: "/workspace/base.txt", Left: "sha256:left-file", Right: "sha256:right-file", Reason: "file manifest root mismatch"}}
			},
			wantErr: "runtime equivalence status",
		},
		{
			name: "narrowed result with concrete differences",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, _ *BaseRuntimeMaterializationCeremonyContract, result *EquivalenceResult, _ *BaseRuntimeEquivalenceBoundaryEvidence) {
				result.Differences = []Difference{{Kind: ObservationBlobSet, Key: "blob:sha256:base", Left: "sha256:left-blob", Right: "sha256:right-blob", Reason: "blob set root mismatch"}}
			},
			wantErr: "narrowed result cannot carry concrete differences",
		},
		{
			name: "missing file manifest unsupported capability",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, _ *BaseRuntimeMaterializationCeremonyContract, result *EquivalenceResult, _ *BaseRuntimeEquivalenceBoundaryEvidence) {
				result.Unsupported = []UnsupportedCapability{{Kind: ObservationBlobSet, Reason: "vmmanager runtime evidence does not expose source blobs"}}
			},
			wantErr: "must name unsupported file_manifest and blob_set observations",
		},
		{
			name: "missing blob set unsupported capability",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, _ *BaseRuntimeMaterializationCeremonyContract, result *EquivalenceResult, _ *BaseRuntimeEquivalenceBoundaryEvidence) {
				result.Unsupported = []UnsupportedCapability{{Kind: ObservationFileManifest, Reason: "vmmanager runtime evidence does not expose source file manifests"}}
			},
			wantErr: "must name unsupported file_manifest and blob_set observations",
		},
		{
			name: "missing runtime materialization ceremony ref",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, _ *BaseRuntimeMaterializationCeremonyContract, _ *EquivalenceResult, evidence *BaseRuntimeEquivalenceBoundaryEvidence) {
				evidence.RuntimeMaterializationCeremonyRef = "\t"
			},
			wantErr: "runtime materialization ceremony ref is required",
		},
		{
			name: "missing runtime equivalence evidence ref",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, _ *BaseRuntimeMaterializationCeremonyContract, _ *EquivalenceResult, evidence *BaseRuntimeEquivalenceBoundaryEvidence) {
				evidence.RuntimeEquivalenceEvidenceRef = ""
			},
			wantErr: "runtime equivalence evidence ref is required",
		},
		{
			name: "evidence no VM lifecycle mutation false",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, _ *BaseRuntimeMaterializationCeremonyContract, _ *EquivalenceResult, evidence *BaseRuntimeEquivalenceBoundaryEvidence) {
				evidence.NoVMLifecycleMutation = false
			},
			wantErr: "evidence must prove no VM lifecycle mutation",
		},
		{
			name: "evidence no production mutation false",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, _ *BaseRuntimeMaterializationCeremonyContract, _ *EquivalenceResult, evidence *BaseRuntimeEquivalenceBoundaryEvidence) {
				evidence.NoProductionMutation = false
			},
			wantErr: "evidence must prove no production mutation",
		},
		{
			name: "ceremony staging claimed",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, ceremony *BaseRuntimeMaterializationCeremonyContract, _ *EquivalenceResult, _ *BaseRuntimeEquivalenceBoundaryEvidence) {
				ceremony.StagingClaimed = true
			},
			wantErr: "ceremony carries protected-surface claims",
		},
		{
			name: "ceremony promotion claimed",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, ceremony *BaseRuntimeMaterializationCeremonyContract, _ *EquivalenceResult, _ *BaseRuntimeEquivalenceBoundaryEvidence) {
				ceremony.PromotionClaimed = true
			},
			wantErr: "ceremony carries protected-surface claims",
		},
		{
			name: "ceremony package publication claimed",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, ceremony *BaseRuntimeMaterializationCeremonyContract, _ *EquivalenceResult, _ *BaseRuntimeEquivalenceBoundaryEvidence) {
				ceremony.PackagePublicationClaimed = true
			},
			wantErr: "ceremony carries protected-surface claims",
		},
		{
			name: "ceremony run acceptance record touched",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, ceremony *BaseRuntimeMaterializationCeremonyContract, _ *EquivalenceResult, _ *BaseRuntimeEquivalenceBoundaryEvidence) {
				ceremony.RunAcceptanceRecordTouched = true
			},
			wantErr: "ceremony carries protected-surface claims",
		},
		{
			name: "ceremony full substrate claimed",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, ceremony *BaseRuntimeMaterializationCeremonyContract, _ *EquivalenceResult, _ *BaseRuntimeEquivalenceBoundaryEvidence) {
				ceremony.FullSubstrateClaimed = true
			},
			wantErr: "ceremony carries protected-surface claims",
		},
		{
			name: "ceremony completion claimed",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, ceremony *BaseRuntimeMaterializationCeremonyContract, _ *EquivalenceResult, _ *BaseRuntimeEquivalenceBoundaryEvidence) {
				ceremony.CompletionClaimed = true
			},
			wantErr: "ceremony carries protected-surface claims",
		},
		{
			name: "evidence staging claimed",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, _ *BaseRuntimeMaterializationCeremonyContract, _ *EquivalenceResult, evidence *BaseRuntimeEquivalenceBoundaryEvidence) {
				evidence.StagingClaimed = true
			},
			wantErr: "evidence carries protected-surface or completion claims",
		},
		{
			name: "evidence promotion claimed",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, _ *BaseRuntimeMaterializationCeremonyContract, _ *EquivalenceResult, evidence *BaseRuntimeEquivalenceBoundaryEvidence) {
				evidence.PromotionClaimed = true
			},
			wantErr: "evidence carries protected-surface or completion claims",
		},
		{
			name: "evidence package publication claimed",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, _ *BaseRuntimeMaterializationCeremonyContract, _ *EquivalenceResult, evidence *BaseRuntimeEquivalenceBoundaryEvidence) {
				evidence.PackagePublicationClaimed = true
			},
			wantErr: "evidence carries protected-surface or completion claims",
		},
		{
			name: "evidence run acceptance record touched",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, _ *BaseRuntimeMaterializationCeremonyContract, _ *EquivalenceResult, evidence *BaseRuntimeEquivalenceBoundaryEvidence) {
				evidence.RunAcceptanceRecordTouched = true
			},
			wantErr: "evidence carries protected-surface or completion claims",
		},
		{
			name: "evidence full substrate claimed",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, _ *BaseRuntimeMaterializationCeremonyContract, _ *EquivalenceResult, evidence *BaseRuntimeEquivalenceBoundaryEvidence) {
				evidence.FullSubstrateClaimed = true
			},
			wantErr: "evidence carries protected-surface or completion claims",
		},
		{
			name: "evidence completion claimed",
			mutate: func(_ *BaseSourceProvenanceReadinessContract, _ *BaseRuntimeMaterializationCeremonyContract, _ *EquivalenceResult, evidence *BaseRuntimeEquivalenceBoundaryEvidence) {
				evidence.CompletionClaimed = true
			},
			wantErr: "evidence carries protected-surface or completion claims",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source, ceremony, result, evidence := baseRuntimeEquivalenceBoundaryContractInputs(t)
			tc.mutate(&source, &ceremony, &result, &evidence)

			contract, err := BuildBaseRuntimeEquivalenceBoundaryContract(source, ceremony, result, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildBaseRuntimeEquivalenceBoundaryContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func baseRuntimeEquivalenceBoundaryContractInputs(t *testing.T) (BaseSourceProvenanceReadinessContract, BaseRuntimeMaterializationCeremonyContract, EquivalenceResult, BaseRuntimeEquivalenceBoundaryEvidence) {
	t.Helper()

	source, realization, ceremonyEvidence := baseRuntimeMaterializationCeremonyContractInputs(t)
	ceremony, err := BuildBaseRuntimeMaterializationCeremonyContract(source, realization, ceremonyEvidence)
	if err != nil {
		t.Fatalf("BuildBaseRuntimeMaterializationCeremonyContract(): %v", err)
	}

	result := EquivalenceResult{
		Status: EquivalenceNarrowed,
		Unsupported: []UnsupportedCapability{
			{Kind: ObservationFileManifest, Reason: "vmmanager runtime evidence does not expose source file manifests"},
			{Kind: ObservationBlobSet, Reason: "vmmanager runtime evidence does not expose source blobs"},
		},
	}
	evidence := BaseRuntimeEquivalenceBoundaryEvidence{
		RuntimeMaterializationCeremonyRef: " contract:base-runtime-materialization-ceremony-pass-100 ",
		RuntimeEquivalenceEvidenceRef:     " equivalence:vmmanager-source-provenance-boundary-pass-101 ",
		NoVMLifecycleMutation:             true,
		NoProductionMutation:              true,
	}
	return source, ceremony, result, evidence
}

func assertUnsupportedDurableObservations(t *testing.T, got []UnsupportedCapability, want []ObservationKind) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("unsupported durable observations length = %d (%#v), want %d", len(got), got, len(want))
	}
	for i, wantKind := range want {
		if got[i].Kind != wantKind {
			t.Fatalf("unsupported durable observation[%d] = %q, want %q in %#v", i, got[i].Kind, wantKind, got)
		}
		if strings.TrimSpace(got[i].Reason) == "" {
			t.Fatalf("unsupported durable observation[%d] reason is empty in %#v", i, got)
		}
	}
}
