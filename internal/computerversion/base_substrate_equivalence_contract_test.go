package computerversion

import (
	"context"
	"strings"
	"testing"
)

func TestBuildBaseSubstrateEquivalenceContractBuildsScopedNoMutationContract(t *testing.T) {
	current, projection, evidence := baseSubstrateEquivalenceContractInputs(t)

	contract, err := BuildBaseSubstrateEquivalenceContract(current, projection, evidence)
	if err != nil {
		t.Fatalf("BuildBaseSubstrateEquivalenceContract(): %v", err)
	}

	if contract.Kind != BaseSubstrateEquivalenceContractKind {
		t.Fatalf("kind = %q, want %q", contract.Kind, BaseSubstrateEquivalenceContractKind)
	}
	if contract.Boundary != BaseSubstrateEquivalenceBoundary {
		t.Fatalf("boundary = %q, want %q", contract.Boundary, BaseSubstrateEquivalenceBoundary)
	}
	if contract.ClaimScope != BaseSubstrateEquivalenceClaimScope {
		t.Fatalf("claim scope = %q, want %q", contract.ClaimScope, BaseSubstrateEquivalenceClaimScope)
	}
	if contract.Version != current.Version {
		t.Fatalf("version = %#v, want %#v", contract.Version, current.Version)
	}
	if contract.CurrentMaterializer != BaseCurrentStateReaderMaterializer || contract.CurrentSubstrate != BaseCurrentStateReaderSubstrate {
		t.Fatalf("current substrate identity = %q/%q, want %q/%q", contract.CurrentMaterializer, contract.CurrentSubstrate, BaseCurrentStateReaderMaterializer, BaseCurrentStateReaderSubstrate)
	}
	if contract.ProjectionMaterializer != BaseFileProjectionMaterializer || contract.ProjectionSubstrate != BaseFileProjectionSubstrate {
		t.Fatalf("projection substrate identity = %q/%q, want %q/%q", contract.ProjectionMaterializer, contract.ProjectionSubstrate, BaseFileProjectionMaterializer, BaseFileProjectionSubstrate)
	}
	if contract.CurrentRealizationRef != evidence.CurrentRealizationRef || contract.ProjectionRealizationRef != evidence.ProjectionRealizationRef || contract.CurrentObservationRef != evidence.CurrentObservationRef || contract.ProjectionObservationRef != evidence.ProjectionObservationRef || contract.EquivalenceEvidenceRef != evidence.EquivalenceEvidenceRef {
		t.Fatalf("evidence refs = current realization %q projection realization %q current observation %q projection observation %q equivalence %q, want %#v", contract.CurrentRealizationRef, contract.ProjectionRealizationRef, contract.CurrentObservationRef, contract.ProjectionObservationRef, contract.EquivalenceEvidenceRef, evidence)
	}
	assertObservationBundleKinds(t, contract.RequiredObservations, []ObservationKind{ObservationBlobSet, ObservationFileManifest})
	if contract.EquivalenceStatus != EquivalenceEquivalent {
		t.Fatalf("equivalence status = %q, want %q", contract.EquivalenceStatus, EquivalenceEquivalent)
	}
	if !contract.NoRuntimeMaterialization {
		t.Fatalf("no runtime materialization = false, want true")
	}
	if !contract.NoOpaqueDataImageDependency {
		t.Fatalf("no opaque data.img dependency = false, want true")
	}
	if !contract.NoMutation {
		t.Fatalf("no mutation = false, want true")
	}
	if contract.RuntimeBehaviorChanged || contract.DeployedRouteRegistered || contract.ProductionAuthTouched || contract.StagingClaimed || contract.PromotionClaimed || contract.VMLifecycleTouched || contract.FirecrackerBootClaimed || contract.RunAcceptanceRecordTouched || contract.FullSubstrateIndependenceClaim || contract.CompletionClaimed {
		t.Fatalf("unsafe flags must remain false in built contract: %#v", contract)
	}
}

func TestBuildBaseSubstrateEquivalenceContractRejectsInvalidInputs(t *testing.T) {
	foreignVersion := ComputerVersion{CodeRef: "git:foreign-base-slice", ArtifactProgramRef: "base-journal:owner/main@cursor-foreign"}
	for _, tc := range []struct {
		name    string
		mutate  func(*Realization, *Realization, *BaseSubstrateEquivalenceEvidence)
		wantErr string
	}{
		{
			name: "seeded observation mismatch",
			mutate: func(_ *Realization, projection *Realization, _ *BaseSubstrateEquivalenceEvidence) {
				replaceFirstObservationValue(projection, ObservationBlobSet, "seeded-mismatch")
			},
			wantErr: "realizations are not equivalent",
		},
		{
			name: "unsupported capability narrows claim",
			mutate: func(_ *Realization, projection *Realization, _ *BaseSubstrateEquivalenceEvidence) {
				projection.Capabilities.Unsupported = []UnsupportedCapability{{Kind: ObservationBlobSet, Reason: "projection cannot prove blob integrity"}}
			},
			wantErr: "claim narrowed by unsupported capabilities",
		},
		{
			name: "identical materializer and substrate",
			mutate: func(current *Realization, projection *Realization, _ *BaseSubstrateEquivalenceEvidence) {
				projection.Capabilities.Materializer = current.Capabilities.Materializer
				projection.Capabilities.Substrate = current.Capabilities.Substrate
			},
			wantErr: "projection must use a non-identical materializer or substrate",
		},
		{
			name: "missing file manifest required scope",
			mutate: func(current *Realization, projection *Realization, _ *BaseSubstrateEquivalenceEvidence) {
				keepOnlyObservationKind(current, ObservationBlobSet)
				keepOnlyObservationKind(projection, ObservationBlobSet)
			},
			wantErr: "required observations must include file_manifest and blob_set",
		},
		{
			name: "missing blob set required scope",
			mutate: func(current *Realization, projection *Realization, _ *BaseSubstrateEquivalenceEvidence) {
				keepOnlyObservationKind(current, ObservationFileManifest)
				keepOnlyObservationKind(projection, ObservationFileManifest)
			},
			wantErr: "required observations must include file_manifest and blob_set",
		},
		{
			name: "different computer version",
			mutate: func(_ *Realization, projection *Realization, _ *BaseSubstrateEquivalenceEvidence) {
				projection.Version = foreignVersion
				projection.Observations.Version = foreignVersion
			},
			wantErr: "realizations name different computer versions",
		},
		{
			name: "observation version drift",
			mutate: func(current *Realization, _ *Realization, _ *BaseSubstrateEquivalenceEvidence) {
				current.Observations.Version = foreignVersion
			},
			wantErr: "current observation version does not match realization version",
		},
		{
			name: "empty observations",
			mutate: func(_ *Realization, projection *Realization, _ *BaseSubstrateEquivalenceEvidence) {
				projection.Observations.Observations = nil
			},
			wantErr: "projection observations are empty",
		},
		{
			name: "invalid realization identity",
			mutate: func(current *Realization, _ *Realization, _ *BaseSubstrateEquivalenceEvidence) {
				current.ID = "  "
			},
			wantErr: "current realization id is required",
		},
		{
			name: "missing claim scope",
			mutate: func(_ *Realization, _ *Realization, evidence *BaseSubstrateEquivalenceEvidence) {
				evidence.ClaimScope = "  "
			},
			wantErr: "claim scope",
		},
		{
			name: "missing current realization ref",
			mutate: func(_ *Realization, _ *Realization, evidence *BaseSubstrateEquivalenceEvidence) {
				evidence.CurrentRealizationRef = "  "
			},
			wantErr: "current realization ref is required",
		},
		{
			name: "missing projection realization ref",
			mutate: func(_ *Realization, _ *Realization, evidence *BaseSubstrateEquivalenceEvidence) {
				evidence.ProjectionRealizationRef = "  "
			},
			wantErr: "projection realization ref is required",
		},
		{
			name: "missing current observation ref",
			mutate: func(_ *Realization, _ *Realization, evidence *BaseSubstrateEquivalenceEvidence) {
				evidence.CurrentObservationRef = "  "
			},
			wantErr: "current observation ref is required",
		},
		{
			name: "missing projection observation ref",
			mutate: func(_ *Realization, _ *Realization, evidence *BaseSubstrateEquivalenceEvidence) {
				evidence.ProjectionObservationRef = "  "
			},
			wantErr: "projection observation ref is required",
		},
		{
			name: "missing equivalence evidence ref",
			mutate: func(_ *Realization, _ *Realization, evidence *BaseSubstrateEquivalenceEvidence) {
				evidence.EquivalenceEvidenceRef = "  "
			},
			wantErr: "equivalence evidence ref is required",
		},
		{
			name: "runtime materialization not ruled out",
			mutate: func(_ *Realization, _ *Realization, evidence *BaseSubstrateEquivalenceEvidence) {
				evidence.NoRuntimeMaterialization = false
			},
			wantErr: "evidence must prove no runtime materialization",
		},
		{
			name: "opaque data image dependency not ruled out",
			mutate: func(_ *Realization, _ *Realization, evidence *BaseSubstrateEquivalenceEvidence) {
				evidence.NoOpaqueDataImageDependency = false
			},
			wantErr: "evidence must prove no opaque data.img dependency",
		},
		{
			name: "runtime behavior changed",
			mutate: func(_ *Realization, _ *Realization, evidence *BaseSubstrateEquivalenceEvidence) {
				evidence.RuntimeBehaviorChanged = true
			},
			wantErr: "cannot change runtime behavior",
		},
		{
			name: "deployed route registered",
			mutate: func(_ *Realization, _ *Realization, evidence *BaseSubstrateEquivalenceEvidence) {
				evidence.DeployedRouteRegistered = true
			},
			wantErr: "cannot register deployed routes",
		},
		{
			name: "production auth touched",
			mutate: func(_ *Realization, _ *Realization, evidence *BaseSubstrateEquivalenceEvidence) {
				evidence.ProductionAuthTouched = true
			},
			wantErr: "cannot touch production auth/session",
		},
		{
			name: "staging claimed",
			mutate: func(_ *Realization, _ *Realization, evidence *BaseSubstrateEquivalenceEvidence) {
				evidence.StagingClaimed = true
			},
			wantErr: "cannot claim staging",
		},
		{
			name: "promotion claimed",
			mutate: func(_ *Realization, _ *Realization, evidence *BaseSubstrateEquivalenceEvidence) {
				evidence.PromotionClaimed = true
			},
			wantErr: "cannot claim promotion",
		},
		{
			name: "vm lifecycle touched",
			mutate: func(_ *Realization, _ *Realization, evidence *BaseSubstrateEquivalenceEvidence) {
				evidence.VMLifecycleTouched = true
			},
			wantErr: "cannot touch VM lifecycle",
		},
		{
			name: "firecracker boot claimed",
			mutate: func(_ *Realization, _ *Realization, evidence *BaseSubstrateEquivalenceEvidence) {
				evidence.FirecrackerBootClaimed = true
			},
			wantErr: "cannot claim Firecracker boot",
		},
		{
			name: "run acceptance record touched",
			mutate: func(_ *Realization, _ *Realization, evidence *BaseSubstrateEquivalenceEvidence) {
				evidence.RunAcceptanceRecordTouched = true
			},
			wantErr: "cannot touch run acceptance records",
		},
		{
			name: "full substrate independence claimed",
			mutate: func(_ *Realization, _ *Realization, evidence *BaseSubstrateEquivalenceEvidence) {
				evidence.FullSubstrateIndependenceClaim = true
			},
			wantErr: "cannot claim full substrate independence",
		},
		{
			name: "completion claimed",
			mutate: func(_ *Realization, _ *Realization, evidence *BaseSubstrateEquivalenceEvidence) {
				evidence.CompletionClaimed = true
			},
			wantErr: "cannot claim completion",
		},
		{
			name: "no mutation false",
			mutate: func(_ *Realization, _ *Realization, evidence *BaseSubstrateEquivalenceEvidence) {
				evidence.NoMutation = false
			},
			wantErr: "evidence must be no-mutation",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			current, projection, evidence := baseSubstrateEquivalenceContractInputs(t)
			tc.mutate(&current, &projection, &evidence)

			contract, err := BuildBaseSubstrateEquivalenceContract(current, projection, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildBaseSubstrateEquivalenceContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func baseSubstrateEquivalenceContractInputs(t *testing.T) (Realization, Realization, BaseSubstrateEquivalenceEvidence) {
	t.Helper()

	version := baseSliceComputerVersion()
	blobs := newBaseBlobStore(t, t.TempDir())
	ref, contentHash := putBaseBlob(t, blobs, []byte("base substrate equivalence proof"))
	jr := newSQLiteJournalWithEvent(t, baseCreateEventWithBlob(1, ref, contentHash))

	currentState, err := BaseCurrentStateObservationSet(context.Background(), "base-current-state", version, jr, blobs)
	if err != nil {
		t.Fatalf("current-state observations: %v", err)
	}
	current, err := (ProjectionMaterializer{ID: BaseCurrentStateReaderMaterializer, Observations: currentState}).Materialize(
		context.Background(),
		version,
		BaseCurrentStateCapabilityManifest(BaseCurrentStateReaderMaterializer, BaseCurrentStateReaderSubstrate),
	)
	if err != nil {
		t.Fatalf("current-state materialize: %v", err)
	}

	projectionState := currentState
	projectionState.Name = "base-file-projection"
	projectionState.Observations = append([]Observation{}, currentState.Observations...)
	projection, err := (ProjectionMaterializer{ID: BaseFileProjectionMaterializer, Observations: projectionState}).Materialize(
		context.Background(),
		version,
		BaseCurrentStateCapabilityManifest(BaseFileProjectionMaterializer, BaseFileProjectionSubstrate),
	)
	if err != nil {
		t.Fatalf("file projection materialize: %v", err)
	}

	evidence := BaseSubstrateEquivalenceEvidence{
		ClaimScope:                  BaseSubstrateEquivalenceClaimScope,
		CurrentRealizationRef:       "realization:base-current-state-reader",
		ProjectionRealizationRef:    "realization:base-file-projection",
		CurrentObservationRef:       "observation:base-current-state",
		ProjectionObservationRef:    "observation:base-file-projection",
		EquivalenceEvidenceRef:      "equivalence:base-current-state-to-file-projection",
		NoRuntimeMaterialization:    true,
		NoOpaqueDataImageDependency: true,
		NoMutation:                  true,
	}
	return current, projection, evidence
}

func replaceFirstObservationValue(realization *Realization, kind ObservationKind, value string) {
	for i := range realization.Observations.Observations {
		if realization.Observations.Observations[i].Kind == kind {
			realization.Observations.Observations[i].Value = value
			return
		}
	}
}

func keepOnlyObservationKind(realization *Realization, kind ObservationKind) {
	kept := make([]Observation, 0, len(realization.Observations.Observations))
	for _, observation := range realization.Observations.Observations {
		if observation.Kind == kind {
			kept = append(kept, observation)
		}
	}
	realization.Observations.Observations = kept
	realization.Observations.Required = []ObservationKind{kind}
}
