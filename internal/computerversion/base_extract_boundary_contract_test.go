package computerversion

import (
	"strings"
	"testing"
)

func TestBuildBaseExtractBoundaryContractBuildsScopedNoMutationContract(t *testing.T) {
	request, observations, evidence := baseExtractBoundaryContractInputs(t)

	contract, err := BuildBaseExtractBoundaryContract(request, observations, evidence)
	if err != nil {
		t.Fatalf("BuildBaseExtractBoundaryContract(): %v", err)
	}

	if contract.Kind != BaseExtractBoundaryContractKind {
		t.Fatalf("kind = %q, want %q", contract.Kind, BaseExtractBoundaryContractKind)
	}
	if contract.Version != request.Version {
		t.Fatalf("version = %#v, want %#v", contract.Version, request.Version)
	}
	if contract.Boundary != BaseExtractBoundary {
		t.Fatalf("boundary = %q, want %q", contract.Boundary, BaseExtractBoundary)
	}
	if contract.Scope != BaseExtractScope {
		t.Fatalf("scope = %q, want %q", contract.Scope, BaseExtractScope)
	}
	if contract.ExtractorKind != BaseExtractorKindJournalBlobCurrentState {
		t.Fatalf("extractor kind = %q, want %q", contract.ExtractorKind, BaseExtractorKindJournalBlobCurrentState)
	}
	if contract.ExtractRequestName != request.Name {
		t.Fatalf("extract request name = %q, want %q", contract.ExtractRequestName, request.Name)
	}
	if contract.ExtractRequestRef != evidence.ExtractRequestRef {
		t.Fatalf("extract request ref = %q, want %q", contract.ExtractRequestRef, evidence.ExtractRequestRef)
	}
	if contract.ObservationSetName != observations.Name {
		t.Fatalf("observation set name = %q, want %q", contract.ObservationSetName, observations.Name)
	}
	if contract.ObservationSetRef != evidence.ObservationSetRef {
		t.Fatalf("observation set ref = %q, want %q", contract.ObservationSetRef, evidence.ObservationSetRef)
	}
	if contract.TypedArtifactProgramRef != evidence.TypedArtifactProgramRef {
		t.Fatalf("typed artifact program ref = %q, want %q", contract.TypedArtifactProgramRef, evidence.TypedArtifactProgramRef)
	}
	assertObservationBundleKinds(t, contract.RequiredObservations, []ObservationKind{ObservationBlobSet, ObservationFileManifest})
	if !contract.NoOpaqueDataImageDependency {
		t.Fatalf("no opaque data image dependency = false, want true")
	}
	if !contract.NoMutation {
		t.Fatalf("no mutation = false, want true")
	}
	if contract.RuntimeBehaviorChanged || contract.DeployedRouteRegistered || contract.ProductionAuthTouched || contract.StagingClaimed || contract.PromotionClaimed || contract.VMLifecycleTouched || contract.RunAcceptanceRecordTouched {
		t.Fatalf("protected-surface flags must remain false in built contract: %#v", contract)
	}
	if contract.MaterializationClaimed || contract.FullComputerContinuityClaimed || contract.DataImageRecoveryClaimed {
		t.Fatalf("materialization/full-computer/data.img recovery claims must remain false in built contract: %#v", contract)
	}
}

func TestBuildBaseExtractBoundaryContractRejectsInvalidInputs(t *testing.T) {
	foreignVersion := ComputerVersion{CodeRef: "git:foreign-base-extract", ArtifactProgramRef: "base-journal:owner/main@cursor-foreign-base-extract"}
	for _, tc := range []struct {
		name    string
		mutate  func(*ExtractRequest, *ObservationSet, *BaseExtractBoundaryEvidence)
		wantErr string
	}{
		{
			name: "missing request name",
			mutate: func(request *ExtractRequest, _ *ObservationSet, _ *BaseExtractBoundaryEvidence) {
				request.Name = "  "
			},
			wantErr: "extract request name is required",
		},
		{
			name: "invalid request version",
			mutate: func(request *ExtractRequest, _ *ObservationSet, evidence *BaseExtractBoundaryEvidence) {
				request.Version = ComputerVersion{CodeRef: "git:base-extract-without-artifact-program"}
				evidence.TypedArtifactProgramRef = ""
			},
			wantErr: "extract request version is invalid",
		},
		{
			name: "missing observation set name",
			mutate: func(_ *ExtractRequest, observations *ObservationSet, _ *BaseExtractBoundaryEvidence) {
				observations.Name = "\t"
			},
			wantErr: "observation set name is required",
		},
		{
			name: "mismatched observation version",
			mutate: func(_ *ExtractRequest, observations *ObservationSet, _ *BaseExtractBoundaryEvidence) {
				observations.Version = foreignVersion
			},
			wantErr: "observation set version does not match request version",
		},
		{
			name: "empty observations",
			mutate: func(_ *ExtractRequest, observations *ObservationSet, _ *BaseExtractBoundaryEvidence) {
				observations.Observations = nil
			},
			wantErr: "observation set is empty",
		},
		{
			name: "missing file manifest",
			mutate: func(_ *ExtractRequest, observations *ObservationSet, _ *BaseExtractBoundaryEvidence) {
				observations.Required = []ObservationKind{ObservationBlobSet}
				observations.Observations = []Observation{{Kind: ObservationBlobSet, Key: "blob:sha256:base-extract", Value: "present"}}
			},
			wantErr: "observation set must include file_manifest and blob_set",
		},
		{
			name: "missing blob set",
			mutate: func(_ *ExtractRequest, observations *ObservationSet, _ *BaseExtractBoundaryEvidence) {
				observations.Required = []ObservationKind{ObservationFileManifest}
				observations.Observations = []Observation{FileManifestObservation("/workspace/base.txt", "sha256:file-only")}
			},
			wantErr: "observation set must include file_manifest and blob_set",
		},
		{
			name: "missing extract request ref",
			mutate: func(_ *ExtractRequest, _ *ObservationSet, evidence *BaseExtractBoundaryEvidence) {
				evidence.ExtractRequestRef = "  "
			},
			wantErr: "extract request ref is required",
		},
		{
			name: "missing observation set ref",
			mutate: func(_ *ExtractRequest, _ *ObservationSet, evidence *BaseExtractBoundaryEvidence) {
				evidence.ObservationSetRef = ""
			},
			wantErr: "observation set ref is required",
		},
		{
			name: "missing typed artifact program ref",
			mutate: func(_ *ExtractRequest, _ *ObservationSet, evidence *BaseExtractBoundaryEvidence) {
				evidence.TypedArtifactProgramRef = "  "
			},
			wantErr: "typed artifact program ref does not match request version",
		},
		{
			name: "typed artifact program mismatch",
			mutate: func(_ *ExtractRequest, _ *ObservationSet, evidence *BaseExtractBoundaryEvidence) {
				evidence.TypedArtifactProgramRef = string(foreignVersion.ArtifactProgramRef)
			},
			wantErr: "typed artifact program ref does not match request version",
		},
		{
			name: "wrong extractor kind",
			mutate: func(_ *ExtractRequest, _ *ObservationSet, evidence *BaseExtractBoundaryEvidence) {
				evidence.ExtractorKind = "vm_materializer"
			},
			wantErr: "extractor kind",
		},
		{
			name: "opaque data image dependency not excluded",
			mutate: func(_ *ExtractRequest, _ *ObservationSet, evidence *BaseExtractBoundaryEvidence) {
				evidence.NoOpaqueDataImageDependency = false
			},
			wantErr: "must prove no opaque data.img dependency",
		},
		{
			name: "runtime behavior changed",
			mutate: func(_ *ExtractRequest, _ *ObservationSet, evidence *BaseExtractBoundaryEvidence) {
				evidence.RuntimeBehaviorChanged = true
			},
			wantErr: "cannot change runtime behavior",
		},
		{
			name: "deployed route registered",
			mutate: func(_ *ExtractRequest, _ *ObservationSet, evidence *BaseExtractBoundaryEvidence) {
				evidence.DeployedRouteRegistered = true
			},
			wantErr: "cannot register deployed routes",
		},
		{
			name: "production auth touched",
			mutate: func(_ *ExtractRequest, _ *ObservationSet, evidence *BaseExtractBoundaryEvidence) {
				evidence.ProductionAuthTouched = true
			},
			wantErr: "cannot touch production auth/session",
		},
		{
			name: "staging claimed",
			mutate: func(_ *ExtractRequest, _ *ObservationSet, evidence *BaseExtractBoundaryEvidence) {
				evidence.StagingClaimed = true
			},
			wantErr: "cannot claim staging",
		},
		{
			name: "promotion claimed",
			mutate: func(_ *ExtractRequest, _ *ObservationSet, evidence *BaseExtractBoundaryEvidence) {
				evidence.PromotionClaimed = true
			},
			wantErr: "cannot claim promotion",
		},
		{
			name: "vm lifecycle touched",
			mutate: func(_ *ExtractRequest, _ *ObservationSet, evidence *BaseExtractBoundaryEvidence) {
				evidence.VMLifecycleTouched = true
			},
			wantErr: "cannot touch VM lifecycle",
		},
		{
			name: "run acceptance record touched",
			mutate: func(_ *ExtractRequest, _ *ObservationSet, evidence *BaseExtractBoundaryEvidence) {
				evidence.RunAcceptanceRecordTouched = true
			},
			wantErr: "cannot touch run acceptance records",
		},
		{
			name: "materialization claim",
			mutate: func(_ *ExtractRequest, _ *ObservationSet, evidence *BaseExtractBoundaryEvidence) {
				evidence.MaterializationClaimed = true
			},
			wantErr: "cannot claim materialization",
		},
		{
			name: "full computer continuity claim",
			mutate: func(_ *ExtractRequest, _ *ObservationSet, evidence *BaseExtractBoundaryEvidence) {
				evidence.FullComputerContinuityClaimed = true
			},
			wantErr: "cannot claim full-computer continuity",
		},
		{
			name: "data image recovery claim",
			mutate: func(_ *ExtractRequest, _ *ObservationSet, evidence *BaseExtractBoundaryEvidence) {
				evidence.DataImageRecoveryClaimed = true
			},
			wantErr: "cannot claim data.img recovery",
		},
		{
			name: "evidence no mutation false",
			mutate: func(_ *ExtractRequest, _ *ObservationSet, evidence *BaseExtractBoundaryEvidence) {
				evidence.NoMutation = false
			},
			wantErr: "evidence must be no-mutation",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request, observations, evidence := baseExtractBoundaryContractInputs(t)
			tc.mutate(&request, &observations, &evidence)

			contract, err := BuildBaseExtractBoundaryContract(request, observations, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildBaseExtractBoundaryContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func baseExtractBoundaryContractInputs(t *testing.T) (ExtractRequest, ObservationSet, BaseExtractBoundaryEvidence) {
	t.Helper()

	version := ComputerVersion{CodeRef: "git:base-extract-pass-90", ArtifactProgramRef: "base-journal:owner/main@cursor-base-extract-pass-90"}
	request := ExtractRequest{
		Name:    "base-extract-request-pass-90",
		Version: version,
	}
	observations := ObservationSet{
		Name:     "base-extract-observation-set-pass-90",
		Version:  version,
		Required: []ObservationKind{ObservationFileManifest, ObservationBlobSet},
		Observations: []Observation{
			FileManifestObservation("/workspace/base.txt", "sha256:file-manifest-root"),
			{Kind: ObservationBlobSet, Key: "blob:sha256:base-extract", Value: "sha256:blob-root"},
		},
	}
	evidence := BaseExtractBoundaryEvidence{
		ExtractRequestRef:           "extract-request:base-extract-pass-90",
		ObservationSetRef:           "observation-set:base-extract-pass-90",
		TypedArtifactProgramRef:     string(version.ArtifactProgramRef),
		ExtractorKind:               BaseExtractorKindJournalBlobCurrentState,
		NoOpaqueDataImageDependency: true,
		NoMutation:                  true,
	}
	return request, observations, evidence
}
