package computerversion

import (
	"strings"
	"testing"
)

func TestBuildBaseCurrentStateUserIsomorphismContractBuildsScopedNoMutationContract(t *testing.T) {
	current, projection, equivalence, evidence := baseCurrentStateUserIsomorphismContractInputs(t)

	contract, err := BuildBaseCurrentStateUserIsomorphismContract(current, projection, equivalence, evidence)
	if err != nil {
		t.Fatalf("BuildBaseCurrentStateUserIsomorphismContract(): %v", err)
	}

	if contract.Kind != BaseCurrentStateUserIsomorphismContractKind {
		t.Fatalf("kind = %q, want %q", contract.Kind, BaseCurrentStateUserIsomorphismContractKind)
	}
	if contract.Boundary != BaseCurrentStateUserIsomorphismBoundary {
		t.Fatalf("boundary = %q, want %q", contract.Boundary, BaseCurrentStateUserIsomorphismBoundary)
	}
	if contract.Version != current.Version {
		t.Fatalf("version = %#v, want %#v", contract.Version, current.Version)
	}
	if contract.EquivalenceContractKind != BaseSubstrateEquivalenceContractKind {
		t.Fatalf("equivalence contract kind = %q, want %q", contract.EquivalenceContractKind, BaseSubstrateEquivalenceContractKind)
	}
	if contract.EquivalenceContractRef != evidence.EquivalenceContractRef {
		t.Fatalf("equivalence contract ref = %q, want %q", contract.EquivalenceContractRef, evidence.EquivalenceContractRef)
	}
	if contract.EquivalenceEvidenceRef != equivalence.EquivalenceEvidenceRef {
		t.Fatalf("equivalence evidence ref = %q, want %q", contract.EquivalenceEvidenceRef, equivalence.EquivalenceEvidenceRef)
	}
	if contract.ScopeRef != evidence.ScopeRef {
		t.Fatalf("scope ref = %q, want %q", contract.ScopeRef, evidence.ScopeRef)
	}
	if contract.UserIsomorphismEvidenceRef != evidence.UserIsomorphismEvidenceRef {
		t.Fatalf("user isomorphism evidence ref = %q, want %q", contract.UserIsomorphismEvidenceRef, evidence.UserIsomorphismEvidenceRef)
	}
	if contract.CurrentMaterializer != BaseCurrentStateReaderMaterializer || contract.CurrentSubstrate != BaseCurrentStateReaderSubstrate {
		t.Fatalf("current substrate identity = %q/%q, want %q/%q", contract.CurrentMaterializer, contract.CurrentSubstrate, BaseCurrentStateReaderMaterializer, BaseCurrentStateReaderSubstrate)
	}
	if contract.ProjectionMaterializer != BaseFileProjectionMaterializer || contract.ProjectionSubstrate != BaseFileProjectionSubstrate {
		t.Fatalf("projection substrate identity = %q/%q, want %q/%q", contract.ProjectionMaterializer, contract.ProjectionSubstrate, BaseFileProjectionMaterializer, BaseFileProjectionSubstrate)
	}
	assertObservationBundleKinds(t, contract.RequiredObservations, []ObservationKind{ObservationBlobSet, ObservationFileManifest})
	assertUserSemantics(t, contract.RequiredSemantics, []UserSemantic{UserSemanticDeletionState, UserSemanticFileContent, UserSemanticFilePath, UserSemanticFileProvenance})
	assertBaseCurrentStateUserIsomorphismScope(t, contract.Scope)
	assertUnsupportedUserSemantic(t, contract.UnsupportedSemantics, UserSemanticLiveProcessContinuity)
	if contract.Status != UserIsomorphismEquivalent {
		t.Fatalf("status = %q, want %q", contract.Status, UserIsomorphismEquivalent)
	}
	if !contract.NoMutation {
		t.Fatalf("no mutation = false, want true")
	}
	if contract.RuntimeBehaviorChanged || contract.DeployedRouteRegistered || contract.ProductionAuthTouched || contract.StagingClaimed || contract.PromotionClaimed || contract.VMLifecycleTouched || contract.RunAcceptanceRecordTouched || contract.FullComputerContinuityClaimed {
		t.Fatalf("unsafe flags must remain false in built contract: %#v", contract)
	}
}

func TestBuildBaseCurrentStateUserIsomorphismContractScopeDefinesNarrowFileBlobUserSemantics(t *testing.T) {
	scope := BaseCurrentStateUserIsomorphismScope()

	assertBaseCurrentStateUserIsomorphismScope(t, scope)
}

func TestBuildBaseCurrentStateUserIsomorphismContractRejectsInvalidInputs(t *testing.T) {
	foreignVersion := ComputerVersion{CodeRef: "git:foreign-base-user-isomorphism", ArtifactProgramRef: "base-journal:owner/main@cursor-foreign-user-isomorphism"}
	for _, tc := range []struct {
		name    string
		mutate  func(*Realization, *Realization, *BaseSubstrateEquivalenceContract, *BaseCurrentStateUserIsomorphismEvidence)
		wantErr string
	}{
		{
			name: "seeded observation mismatch",
			mutate: func(_ *Realization, projection *Realization, _ *BaseSubstrateEquivalenceContract, _ *BaseCurrentStateUserIsomorphismEvidence) {
				replaceFirstObservationValue(projection, ObservationBlobSet, "seeded-user-isomorphism-mismatch")
			},
			wantErr: "realizations are not user-isomorphic",
		},
		{
			name: "unsupported capability narrows claim",
			mutate: func(_ *Realization, projection *Realization, _ *BaseSubstrateEquivalenceContract, _ *BaseCurrentStateUserIsomorphismEvidence) {
				projection.Capabilities.Unsupported = []UnsupportedCapability{{Kind: ObservationBlobSet, Reason: "projection cannot prove blob integrity"}}
			},
			wantErr: "claim narrowed",
		},
		{
			name: "missing scoped observation narrows claim",
			mutate: func(current *Realization, projection *Realization, _ *BaseSubstrateEquivalenceContract, _ *BaseCurrentStateUserIsomorphismEvidence) {
				keepOnlyObservationKind(current, ObservationFileManifest)
				keepOnlyObservationKind(projection, ObservationFileManifest)
			},
			wantErr: "claim narrowed",
		},
		{
			name: "wrong equivalence contract kind",
			mutate: func(_ *Realization, _ *Realization, equivalence *BaseSubstrateEquivalenceContract, _ *BaseCurrentStateUserIsomorphismEvidence) {
				equivalence.Kind = "full_computer_equivalence_contract"
			},
			wantErr: "equivalence contract kind",
		},
		{
			name: "wrong equivalence claim scope",
			mutate: func(_ *Realization, _ *Realization, equivalence *BaseSubstrateEquivalenceContract, _ *BaseCurrentStateUserIsomorphismEvidence) {
				equivalence.ClaimScope = "full_computer"
			},
			wantErr: "equivalence claim scope",
		},
		{
			name: "non equivalent equivalence status",
			mutate: func(_ *Realization, _ *Realization, equivalence *BaseSubstrateEquivalenceContract, _ *BaseCurrentStateUserIsomorphismEvidence) {
				equivalence.EquivalenceStatus = EquivalenceNotEquivalent
			},
			wantErr: "equivalence status",
		},
		{
			name: "equivalence contract missing file manifest observation",
			mutate: func(_ *Realization, _ *Realization, equivalence *BaseSubstrateEquivalenceContract, _ *BaseCurrentStateUserIsomorphismEvidence) {
				equivalence.RequiredObservations = []ObservationKind{ObservationBlobSet}
			},
			wantErr: "equivalence contract must include file_manifest and blob_set",
		},
		{
			name: "equivalence contract missing blob set observation",
			mutate: func(_ *Realization, _ *Realization, equivalence *BaseSubstrateEquivalenceContract, _ *BaseCurrentStateUserIsomorphismEvidence) {
				equivalence.RequiredObservations = []ObservationKind{ObservationFileManifest}
			},
			wantErr: "equivalence contract must include file_manifest and blob_set",
		},
		{
			name: "equivalence contract unsafe flag",
			mutate: func(_ *Realization, _ *Realization, equivalence *BaseSubstrateEquivalenceContract, _ *BaseCurrentStateUserIsomorphismEvidence) {
				equivalence.RuntimeBehaviorChanged = true
			},
			wantErr: "equivalence contract must be no-mutation and unsafe flags false",
		},
		{
			name: "equivalence contract no mutation false",
			mutate: func(_ *Realization, _ *Realization, equivalence *BaseSubstrateEquivalenceContract, _ *BaseCurrentStateUserIsomorphismEvidence) {
				equivalence.NoMutation = false
			},
			wantErr: "equivalence contract must be no-mutation and unsafe flags false",
		},
		{
			name: "current realization version mismatch",
			mutate: func(current *Realization, _ *Realization, _ *BaseSubstrateEquivalenceContract, _ *BaseCurrentStateUserIsomorphismEvidence) {
				current.Version = foreignVersion
			},
			wantErr: "realization versions must match equivalence contract version",
		},
		{
			name: "projection realization version mismatch",
			mutate: func(_ *Realization, projection *Realization, _ *BaseSubstrateEquivalenceContract, _ *BaseCurrentStateUserIsomorphismEvidence) {
				projection.Version = foreignVersion
			},
			wantErr: "realization versions must match equivalence contract version",
		},
		{
			name: "current realization identity mismatch",
			mutate: func(current *Realization, _ *Realization, _ *BaseSubstrateEquivalenceContract, _ *BaseCurrentStateUserIsomorphismEvidence) {
				current.Capabilities.Materializer = "other-current-state-reader"
			},
			wantErr: "current realization identity does not match equivalence contract",
		},
		{
			name: "projection realization identity mismatch",
			mutate: func(_ *Realization, projection *Realization, _ *BaseSubstrateEquivalenceContract, _ *BaseCurrentStateUserIsomorphismEvidence) {
				projection.Capabilities.Substrate = "firecracker-runtime"
			},
			wantErr: "projection realization identity does not match equivalence contract",
		},
		{
			name: "missing equivalence contract ref",
			mutate: func(_ *Realization, _ *Realization, _ *BaseSubstrateEquivalenceContract, evidence *BaseCurrentStateUserIsomorphismEvidence) {
				evidence.EquivalenceContractRef = "  "
			},
			wantErr: "equivalence contract ref is required",
		},
		{
			name: "missing scope ref",
			mutate: func(_ *Realization, _ *Realization, _ *BaseSubstrateEquivalenceContract, evidence *BaseCurrentStateUserIsomorphismEvidence) {
				evidence.ScopeRef = "  "
			},
			wantErr: "scope ref is required",
		},
		{
			name: "missing user isomorphism evidence ref",
			mutate: func(_ *Realization, _ *Realization, _ *BaseSubstrateEquivalenceContract, evidence *BaseCurrentStateUserIsomorphismEvidence) {
				evidence.UserIsomorphismEvidenceRef = "  "
			},
			wantErr: "user isomorphism evidence ref is required",
		},
		{
			name: "runtime behavior changed",
			mutate: func(_ *Realization, _ *Realization, _ *BaseSubstrateEquivalenceContract, evidence *BaseCurrentStateUserIsomorphismEvidence) {
				evidence.RuntimeBehaviorChanged = true
			},
			wantErr: "cannot change runtime behavior",
		},
		{
			name: "deployed route registered",
			mutate: func(_ *Realization, _ *Realization, _ *BaseSubstrateEquivalenceContract, evidence *BaseCurrentStateUserIsomorphismEvidence) {
				evidence.DeployedRouteRegistered = true
			},
			wantErr: "cannot register deployed routes",
		},
		{
			name: "production auth touched",
			mutate: func(_ *Realization, _ *Realization, _ *BaseSubstrateEquivalenceContract, evidence *BaseCurrentStateUserIsomorphismEvidence) {
				evidence.ProductionAuthTouched = true
			},
			wantErr: "cannot touch production auth/session",
		},
		{
			name: "staging claimed",
			mutate: func(_ *Realization, _ *Realization, _ *BaseSubstrateEquivalenceContract, evidence *BaseCurrentStateUserIsomorphismEvidence) {
				evidence.StagingClaimed = true
			},
			wantErr: "cannot claim staging",
		},
		{
			name: "promotion claimed",
			mutate: func(_ *Realization, _ *Realization, _ *BaseSubstrateEquivalenceContract, evidence *BaseCurrentStateUserIsomorphismEvidence) {
				evidence.PromotionClaimed = true
			},
			wantErr: "cannot claim promotion",
		},
		{
			name: "vm lifecycle touched",
			mutate: func(_ *Realization, _ *Realization, _ *BaseSubstrateEquivalenceContract, evidence *BaseCurrentStateUserIsomorphismEvidence) {
				evidence.VMLifecycleTouched = true
			},
			wantErr: "cannot touch VM lifecycle",
		},
		{
			name: "run acceptance record touched",
			mutate: func(_ *Realization, _ *Realization, _ *BaseSubstrateEquivalenceContract, evidence *BaseCurrentStateUserIsomorphismEvidence) {
				evidence.RunAcceptanceRecordTouched = true
			},
			wantErr: "cannot touch run acceptance records",
		},
		{
			name: "full computer continuity claimed",
			mutate: func(_ *Realization, _ *Realization, _ *BaseSubstrateEquivalenceContract, evidence *BaseCurrentStateUserIsomorphismEvidence) {
				evidence.FullComputerContinuityClaimed = true
			},
			wantErr: "cannot claim full-computer continuity",
		},
		{
			name: "no mutation false",
			mutate: func(_ *Realization, _ *Realization, _ *BaseSubstrateEquivalenceContract, evidence *BaseCurrentStateUserIsomorphismEvidence) {
				evidence.NoMutation = false
			},
			wantErr: "evidence must be no-mutation",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			current, projection, equivalence, evidence := baseCurrentStateUserIsomorphismContractInputs(t)
			tc.mutate(&current, &projection, &equivalence, &evidence)

			contract, err := BuildBaseCurrentStateUserIsomorphismContract(current, projection, equivalence, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildBaseCurrentStateUserIsomorphismContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func baseCurrentStateUserIsomorphismContractInputs(t *testing.T) (Realization, Realization, BaseSubstrateEquivalenceContract, BaseCurrentStateUserIsomorphismEvidence) {
	t.Helper()

	current, projection, equivalenceEvidence := baseSubstrateEquivalenceContractInputs(t)
	equivalence, err := BuildBaseSubstrateEquivalenceContract(current, projection, equivalenceEvidence)
	if err != nil {
		t.Fatalf("BuildBaseSubstrateEquivalenceContract(): %v", err)
	}
	evidence := BaseCurrentStateUserIsomorphismEvidence{
		EquivalenceContractRef:     "contract:base-substrate-equivalence-pass-87",
		ScopeRef:                   "scope:base-current-state-file-manifest-blob-set-user-semantics",
		UserIsomorphismEvidenceRef: "user-isomorphism:base-current-state-to-file-projection",
		NoMutation:                 true,
	}
	return current, projection, equivalence, evidence
}

func assertBaseCurrentStateUserIsomorphismScope(t *testing.T, scope UserIsomorphismScope) {
	t.Helper()

	if scope.Name != BaseCurrentStateUserIsomorphismScopeName {
		t.Fatalf("scope name = %q, want %q", scope.Name, BaseCurrentStateUserIsomorphismScopeName)
	}
	assertObservationBundleKinds(t, scope.ObservationKinds, []ObservationKind{ObservationBlobSet, ObservationFileManifest})
	assertUserSemantics(t, scope.RequiredSemantics, []UserSemantic{UserSemanticFilePath, UserSemanticFileContent, UserSemanticDeletionState, UserSemanticFileProvenance})
	assertUserSemantics(t, scope.CoveredSemantics, []UserSemantic{UserSemanticFilePath, UserSemanticFileContent, UserSemanticDeletionState, UserSemanticFileProvenance})
	assertUnsupportedUserSemantic(t, scope.UnsupportedSemantics, UserSemanticLiveProcessContinuity)
}

func assertUserSemantics(t *testing.T, got, want []UserSemantic) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("user semantics = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("user semantics = %#v, want %#v", got, want)
		}
	}
}

func assertUnsupportedUserSemantic(t *testing.T, got []UnsupportedUserSemantic, want UserSemantic) {
	t.Helper()

	for _, unsupported := range got {
		if unsupported.Semantic == want && strings.TrimSpace(unsupported.Reason) != "" {
			return
		}
	}
	t.Fatalf("unsupported semantics = %#v, want %q with reason", got, want)
}
