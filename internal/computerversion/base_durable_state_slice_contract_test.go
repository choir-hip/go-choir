package computerversion

import (
	"strings"
	"testing"
)

func TestBuildBaseDurableStateSliceContractBuildsScopedNoMutationContract(t *testing.T) {
	equivalence, user, evidence := baseDurableStateSliceContractInputs(t)

	contract, err := BuildBaseDurableStateSliceContract(equivalence, user, evidence)
	if err != nil {
		t.Fatalf("BuildBaseDurableStateSliceContract(): %v", err)
	}

	if contract.Kind != BaseDurableStateSliceContractKind {
		t.Fatalf("kind = %q, want %q", contract.Kind, BaseDurableStateSliceContractKind)
	}
	if contract.Version != equivalence.Version {
		t.Fatalf("version = %#v, want %#v", contract.Version, equivalence.Version)
	}
	if contract.Boundary != BaseDurableStateSliceBoundary {
		t.Fatalf("boundary = %q, want %q", contract.Boundary, BaseDurableStateSliceBoundary)
	}
	if contract.Scope != BaseDurableStateSliceScope {
		t.Fatalf("scope = %q, want %q", contract.Scope, BaseDurableStateSliceScope)
	}
	assertBaseDurableStateClasses(t, contract.PersistentStateClasses, []BaseDurableStateClass{BaseDurableStateClassBlobContent, BaseDurableStateClassFileManifest})
	assertObservationBundleKinds(t, contract.RequiredObservations, []ObservationKind{ObservationBlobSet, ObservationFileManifest})
	assertUserSemantics(t, contract.RequiredSemantics, []UserSemantic{UserSemanticDeletionState, UserSemanticFileContent, UserSemanticFilePath, UserSemanticFileProvenance})
	if contract.EquivalenceContractKind != BaseSubstrateEquivalenceContractKind {
		t.Fatalf("equivalence contract kind = %q, want %q", contract.EquivalenceContractKind, BaseSubstrateEquivalenceContractKind)
	}
	if contract.EquivalenceContractRef != evidence.EquivalenceContractRef {
		t.Fatalf("equivalence contract ref = %q, want %q", contract.EquivalenceContractRef, evidence.EquivalenceContractRef)
	}
	if contract.UserIsomorphismContractKind != BaseCurrentStateUserIsomorphismContractKind {
		t.Fatalf("user isomorphism contract kind = %q, want %q", contract.UserIsomorphismContractKind, BaseCurrentStateUserIsomorphismContractKind)
	}
	if contract.UserIsomorphismContractRef != evidence.UserIsomorphismContractRef {
		t.Fatalf("user isomorphism contract ref = %q, want %q", contract.UserIsomorphismContractRef, evidence.UserIsomorphismContractRef)
	}
	if contract.TypedArtifactProgramRef != evidence.TypedArtifactProgramRef {
		t.Fatalf("typed artifact program ref = %q, want %q", contract.TypedArtifactProgramRef, evidence.TypedArtifactProgramRef)
	}
	if contract.DurableSliceEvidenceRef != evidence.DurableSliceEvidenceRef {
		t.Fatalf("durable slice evidence ref = %q, want %q", contract.DurableSliceEvidenceRef, evidence.DurableSliceEvidenceRef)
	}
	if !contract.NoOpaqueDataImageDependency {
		t.Fatalf("no opaque data image dependency = false, want true")
	}
	if !contract.NoMutation {
		t.Fatalf("no mutation = false, want true")
	}
	if contract.FullComputerClaimed || contract.DataImageDisposableClaimed {
		t.Fatalf("full-computer/data.img claims must remain false in built contract: %#v", contract)
	}
	if contract.RuntimeBehaviorChanged || contract.DeployedRouteRegistered || contract.ProductionAuthTouched || contract.StagingClaimed || contract.PromotionClaimed || contract.VMLifecycleTouched || contract.RunAcceptanceRecordTouched {
		t.Fatalf("unsafe flags must remain false in built contract: %#v", contract)
	}
	if user.FullComputerContinuityClaimed {
		t.Fatalf("test fixture user contract unexpectedly claims full-computer continuity: %#v", user)
	}
}

func TestBuildBaseDurableStateSliceContractRejectsInvalidInputs(t *testing.T) {
	foreignVersion := ComputerVersion{CodeRef: "git:foreign-base-durable-slice", ArtifactProgramRef: "base-journal:owner/main@cursor-foreign-durable-slice"}
	for _, tc := range []struct {
		name    string
		mutate  func(*BaseSubstrateEquivalenceContract, *BaseCurrentStateUserIsomorphismContract, *BaseDurableStateSliceEvidence)
		wantErr string
	}{
		{
			name: "missing equivalence contract ref",
			mutate: func(_ *BaseSubstrateEquivalenceContract, _ *BaseCurrentStateUserIsomorphismContract, evidence *BaseDurableStateSliceEvidence) {
				evidence.EquivalenceContractRef = "  "
			},
			wantErr: "equivalence contract ref is required",
		},
		{
			name: "missing user isomorphism contract ref",
			mutate: func(_ *BaseSubstrateEquivalenceContract, _ *BaseCurrentStateUserIsomorphismContract, evidence *BaseDurableStateSliceEvidence) {
				evidence.UserIsomorphismContractRef = "\t"
			},
			wantErr: "user isomorphism contract ref is required",
		},
		{
			name: "missing typed artifact program ref",
			mutate: func(_ *BaseSubstrateEquivalenceContract, _ *BaseCurrentStateUserIsomorphismContract, evidence *BaseDurableStateSliceEvidence) {
				evidence.TypedArtifactProgramRef = ""
			},
			wantErr: "typed artifact program ref is required",
		},
		{
			name: "typed artifact program ref mismatches version",
			mutate: func(_ *BaseSubstrateEquivalenceContract, _ *BaseCurrentStateUserIsomorphismContract, evidence *BaseDurableStateSliceEvidence) {
				evidence.TypedArtifactProgramRef = "base-journal:owner/main@foreign-durable-slice"
			},
			wantErr: "typed artifact program ref does not match equivalence version",
		},
		{
			name: "missing durable slice evidence ref",
			mutate: func(_ *BaseSubstrateEquivalenceContract, _ *BaseCurrentStateUserIsomorphismContract, evidence *BaseDurableStateSliceEvidence) {
				evidence.DurableSliceEvidenceRef = "  "
			},
			wantErr: "durable slice evidence ref is required",
		},
		{
			name: "opaque data image dependency not excluded",
			mutate: func(_ *BaseSubstrateEquivalenceContract, _ *BaseCurrentStateUserIsomorphismContract, evidence *BaseDurableStateSliceEvidence) {
				evidence.NoOpaqueDataImageDependency = false
			},
			wantErr: "must prove no opaque data.img dependency",
		},
		{
			name: "runtime behavior changed",
			mutate: func(_ *BaseSubstrateEquivalenceContract, _ *BaseCurrentStateUserIsomorphismContract, evidence *BaseDurableStateSliceEvidence) {
				evidence.RuntimeBehaviorChanged = true
			},
			wantErr: "cannot change runtime behavior",
		},
		{
			name: "deployed route registered",
			mutate: func(_ *BaseSubstrateEquivalenceContract, _ *BaseCurrentStateUserIsomorphismContract, evidence *BaseDurableStateSliceEvidence) {
				evidence.DeployedRouteRegistered = true
			},
			wantErr: "cannot register deployed routes",
		},
		{
			name: "production auth touched",
			mutate: func(_ *BaseSubstrateEquivalenceContract, _ *BaseCurrentStateUserIsomorphismContract, evidence *BaseDurableStateSliceEvidence) {
				evidence.ProductionAuthTouched = true
			},
			wantErr: "cannot touch production auth/session",
		},
		{
			name: "staging claimed",
			mutate: func(_ *BaseSubstrateEquivalenceContract, _ *BaseCurrentStateUserIsomorphismContract, evidence *BaseDurableStateSliceEvidence) {
				evidence.StagingClaimed = true
			},
			wantErr: "cannot claim staging",
		},
		{
			name: "promotion claimed",
			mutate: func(_ *BaseSubstrateEquivalenceContract, _ *BaseCurrentStateUserIsomorphismContract, evidence *BaseDurableStateSliceEvidence) {
				evidence.PromotionClaimed = true
			},
			wantErr: "cannot claim promotion",
		},
		{
			name: "vm lifecycle touched",
			mutate: func(_ *BaseSubstrateEquivalenceContract, _ *BaseCurrentStateUserIsomorphismContract, evidence *BaseDurableStateSliceEvidence) {
				evidence.VMLifecycleTouched = true
			},
			wantErr: "cannot touch VM lifecycle",
		},
		{
			name: "run acceptance record touched",
			mutate: func(_ *BaseSubstrateEquivalenceContract, _ *BaseCurrentStateUserIsomorphismContract, evidence *BaseDurableStateSliceEvidence) {
				evidence.RunAcceptanceRecordTouched = true
			},
			wantErr: "cannot touch run acceptance records",
		},
		{
			name: "full computer claimed",
			mutate: func(_ *BaseSubstrateEquivalenceContract, _ *BaseCurrentStateUserIsomorphismContract, evidence *BaseDurableStateSliceEvidence) {
				evidence.FullComputerClaimed = true
			},
			wantErr: "cannot claim full-computer coverage",
		},
		{
			name: "data image disposable claimed",
			mutate: func(_ *BaseSubstrateEquivalenceContract, _ *BaseCurrentStateUserIsomorphismContract, evidence *BaseDurableStateSliceEvidence) {
				evidence.DataImageDisposableClaimed = true
			},
			wantErr: "cannot claim data.img disposability",
		},
		{
			name: "evidence no mutation false",
			mutate: func(_ *BaseSubstrateEquivalenceContract, _ *BaseCurrentStateUserIsomorphismContract, evidence *BaseDurableStateSliceEvidence) {
				evidence.NoMutation = false
			},
			wantErr: "evidence must be no-mutation",
		},
		{
			name: "wrong equivalence kind",
			mutate: func(equivalence *BaseSubstrateEquivalenceContract, _ *BaseCurrentStateUserIsomorphismContract, _ *BaseDurableStateSliceEvidence) {
				equivalence.Kind = "full_computer_equivalence_contract"
			},
			wantErr: "equivalence contract kind",
		},
		{
			name: "wrong equivalence claim scope",
			mutate: func(equivalence *BaseSubstrateEquivalenceContract, _ *BaseCurrentStateUserIsomorphismContract, _ *BaseDurableStateSliceEvidence) {
				equivalence.ClaimScope = "full_computer"
			},
			wantErr: "equivalence claim scope",
		},
		{
			name: "wrong equivalence status",
			mutate: func(equivalence *BaseSubstrateEquivalenceContract, _ *BaseCurrentStateUserIsomorphismContract, _ *BaseDurableStateSliceEvidence) {
				equivalence.EquivalenceStatus = EquivalenceNotEquivalent
			},
			wantErr: "equivalence status",
		},
		{
			name: "equivalence missing file manifest observation",
			mutate: func(equivalence *BaseSubstrateEquivalenceContract, _ *BaseCurrentStateUserIsomorphismContract, _ *BaseDurableStateSliceEvidence) {
				equivalence.RequiredObservations = []ObservationKind{ObservationBlobSet}
			},
			wantErr: "equivalence contract must include file_manifest and blob_set",
		},
		{
			name: "equivalence missing blob set observation",
			mutate: func(equivalence *BaseSubstrateEquivalenceContract, _ *BaseCurrentStateUserIsomorphismContract, _ *BaseDurableStateSliceEvidence) {
				equivalence.RequiredObservations = []ObservationKind{ObservationFileManifest}
			},
			wantErr: "equivalence contract must include file_manifest and blob_set",
		},
		{
			name: "equivalence no runtime materialization false",
			mutate: func(equivalence *BaseSubstrateEquivalenceContract, _ *BaseCurrentStateUserIsomorphismContract, _ *BaseDurableStateSliceEvidence) {
				equivalence.NoRuntimeMaterialization = false
			},
			wantErr: "equivalence contract must be local no-runtime no-mutation evidence with unsafe flags false",
		},
		{
			name: "equivalence no opaque data image dependency false",
			mutate: func(equivalence *BaseSubstrateEquivalenceContract, _ *BaseCurrentStateUserIsomorphismContract, _ *BaseDurableStateSliceEvidence) {
				equivalence.NoOpaqueDataImageDependency = false
			},
			wantErr: "equivalence contract must be local no-runtime no-mutation evidence with unsafe flags false",
		},
		{
			name: "equivalence Firecracker boot claimed",
			mutate: func(equivalence *BaseSubstrateEquivalenceContract, _ *BaseCurrentStateUserIsomorphismContract, _ *BaseDurableStateSliceEvidence) {
				equivalence.FirecrackerBootClaimed = true
			},
			wantErr: "equivalence contract must be local no-runtime no-mutation evidence with unsafe flags false",
		},
		{
			name: "equivalence full substrate independence claimed",
			mutate: func(equivalence *BaseSubstrateEquivalenceContract, _ *BaseCurrentStateUserIsomorphismContract, _ *BaseDurableStateSliceEvidence) {
				equivalence.FullSubstrateIndependenceClaim = true
			},
			wantErr: "equivalence contract must be local no-runtime no-mutation evidence with unsafe flags false",
		},
		{
			name: "equivalence completion claimed",
			mutate: func(equivalence *BaseSubstrateEquivalenceContract, _ *BaseCurrentStateUserIsomorphismContract, _ *BaseDurableStateSliceEvidence) {
				equivalence.CompletionClaimed = true
			},
			wantErr: "equivalence contract must be local no-runtime no-mutation evidence with unsafe flags false",
		},
		{
			name: "unsafe equivalence flag",
			mutate: func(equivalence *BaseSubstrateEquivalenceContract, _ *BaseCurrentStateUserIsomorphismContract, _ *BaseDurableStateSliceEvidence) {
				equivalence.RuntimeBehaviorChanged = true
			},
			wantErr: "equivalence contract must be local no-runtime no-mutation evidence with unsafe flags false",
		},
		{
			name: "equivalence no mutation false",
			mutate: func(equivalence *BaseSubstrateEquivalenceContract, _ *BaseCurrentStateUserIsomorphismContract, _ *BaseDurableStateSliceEvidence) {
				equivalence.NoMutation = false
			},
			wantErr: "equivalence contract must be local no-runtime no-mutation evidence with unsafe flags false",
		},
		{
			name: "wrong user isomorphism kind",
			mutate: func(_ *BaseSubstrateEquivalenceContract, user *BaseCurrentStateUserIsomorphismContract, _ *BaseDurableStateSliceEvidence) {
				user.Kind = "full_computer_user_isomorphism_contract"
			},
			wantErr: "user isomorphism contract kind",
		},
		{
			name: "wrong user isomorphism version",
			mutate: func(_ *BaseSubstrateEquivalenceContract, user *BaseCurrentStateUserIsomorphismContract, _ *BaseDurableStateSliceEvidence) {
				user.Version = foreignVersion
			},
			wantErr: "user isomorphism version does not match",
		},
		{
			name: "wrong user isomorphism status",
			mutate: func(_ *BaseSubstrateEquivalenceContract, user *BaseCurrentStateUserIsomorphismContract, _ *BaseDurableStateSliceEvidence) {
				user.Status = UserIsomorphismNotEquivalent
			},
			wantErr: "user isomorphism status",
		},
		{
			name: "wrong user isomorphism equivalence ref",
			mutate: func(_ *BaseSubstrateEquivalenceContract, user *BaseCurrentStateUserIsomorphismContract, _ *BaseDurableStateSliceEvidence) {
				user.EquivalenceEvidenceRef = "equivalence:foreign-base-slice"
			},
			wantErr: "does not bind equivalence evidence",
		},
		{
			name: "wrong user isomorphism equivalence kind binding",
			mutate: func(_ *BaseSubstrateEquivalenceContract, user *BaseCurrentStateUserIsomorphismContract, _ *BaseDurableStateSliceEvidence) {
				user.EquivalenceContractKind = "foreign_equivalence_contract"
			},
			wantErr: "does not bind equivalence evidence",
		},
		{
			name: "wrong user isomorphism current identity",
			mutate: func(_ *BaseSubstrateEquivalenceContract, user *BaseCurrentStateUserIsomorphismContract, _ *BaseDurableStateSliceEvidence) {
				user.CurrentMaterializer = "foreign-current-state-reader"
			},
			wantErr: "realization identity does not match",
		},
		{
			name: "wrong user isomorphism projection identity",
			mutate: func(_ *BaseSubstrateEquivalenceContract, user *BaseCurrentStateUserIsomorphismContract, _ *BaseDurableStateSliceEvidence) {
				user.ProjectionSubstrate = "firecracker-runtime"
			},
			wantErr: "realization identity does not match",
		},
		{
			name: "unsafe user isomorphism flag",
			mutate: func(_ *BaseSubstrateEquivalenceContract, user *BaseCurrentStateUserIsomorphismContract, _ *BaseDurableStateSliceEvidence) {
				user.ProductionAuthTouched = true
			},
			wantErr: "user isomorphism contract must be no-mutation and unsafe flags false",
		},
		{
			name: "user isomorphism full computer continuity claimed",
			mutate: func(_ *BaseSubstrateEquivalenceContract, user *BaseCurrentStateUserIsomorphismContract, _ *BaseDurableStateSliceEvidence) {
				user.FullComputerContinuityClaimed = true
			},
			wantErr: "user isomorphism contract must be no-mutation and unsafe flags false",
		},
		{
			name: "user isomorphism no mutation false",
			mutate: func(_ *BaseSubstrateEquivalenceContract, user *BaseCurrentStateUserIsomorphismContract, _ *BaseDurableStateSliceEvidence) {
				user.NoMutation = false
			},
			wantErr: "user isomorphism contract must be no-mutation and unsafe flags false",
		},
		{
			name: "user isomorphism missing file manifest observation",
			mutate: func(_ *BaseSubstrateEquivalenceContract, user *BaseCurrentStateUserIsomorphismContract, _ *BaseDurableStateSliceEvidence) {
				user.RequiredObservations = []ObservationKind{ObservationBlobSet}
			},
			wantErr: "user isomorphism contract must include file_manifest and blob_set",
		},
		{
			name: "user isomorphism missing blob set observation",
			mutate: func(_ *BaseSubstrateEquivalenceContract, user *BaseCurrentStateUserIsomorphismContract, _ *BaseDurableStateSliceEvidence) {
				user.RequiredObservations = []ObservationKind{ObservationFileManifest}
			},
			wantErr: "user isomorphism contract must include file_manifest and blob_set",
		},
		{
			name: "missing file path semantic",
			mutate: func(_ *BaseSubstrateEquivalenceContract, user *BaseCurrentStateUserIsomorphismContract, _ *BaseDurableStateSliceEvidence) {
				user.RequiredSemantics = withoutUserSemantic(user.RequiredSemantics, UserSemanticFilePath)
			},
			wantErr: "must cover file path, content, deletion, and provenance semantics",
		},
		{
			name: "missing file content semantic",
			mutate: func(_ *BaseSubstrateEquivalenceContract, user *BaseCurrentStateUserIsomorphismContract, _ *BaseDurableStateSliceEvidence) {
				user.RequiredSemantics = withoutUserSemantic(user.RequiredSemantics, UserSemanticFileContent)
			},
			wantErr: "must cover file path, content, deletion, and provenance semantics",
		},
		{
			name: "missing deletion semantic",
			mutate: func(_ *BaseSubstrateEquivalenceContract, user *BaseCurrentStateUserIsomorphismContract, _ *BaseDurableStateSliceEvidence) {
				user.RequiredSemantics = withoutUserSemantic(user.RequiredSemantics, UserSemanticDeletionState)
			},
			wantErr: "must cover file path, content, deletion, and provenance semantics",
		},
		{
			name: "missing provenance semantic",
			mutate: func(_ *BaseSubstrateEquivalenceContract, user *BaseCurrentStateUserIsomorphismContract, _ *BaseDurableStateSliceEvidence) {
				user.RequiredSemantics = withoutUserSemantic(user.RequiredSemantics, UserSemanticFileProvenance)
			},
			wantErr: "must cover file path, content, deletion, and provenance semantics",
		},
		{
			name: "missing unsupported live process continuity",
			mutate: func(_ *BaseSubstrateEquivalenceContract, user *BaseCurrentStateUserIsomorphismContract, _ *BaseDurableStateSliceEvidence) {
				user.UnsupportedSemantics = nil
			},
			wantErr: "must mark live_process_continuity unsupported",
		},
		{
			name: "unsupported live process continuity missing reason",
			mutate: func(_ *BaseSubstrateEquivalenceContract, user *BaseCurrentStateUserIsomorphismContract, _ *BaseDurableStateSliceEvidence) {
				user.UnsupportedSemantics = []UnsupportedUserSemantic{{Semantic: UserSemanticLiveProcessContinuity, Reason: "  "}}
			},
			wantErr: "must mark live_process_continuity unsupported",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			equivalence, user, evidence := baseDurableStateSliceContractInputs(t)
			tc.mutate(&equivalence, &user, &evidence)

			contract, err := BuildBaseDurableStateSliceContract(equivalence, user, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildBaseDurableStateSliceContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func baseDurableStateSliceContractInputs(t *testing.T) (BaseSubstrateEquivalenceContract, BaseCurrentStateUserIsomorphismContract, BaseDurableStateSliceEvidence) {
	t.Helper()

	current, projection, equivalence, userEvidence := baseCurrentStateUserIsomorphismContractInputs(t)
	user, err := BuildBaseCurrentStateUserIsomorphismContract(current, projection, equivalence, userEvidence)
	if err != nil {
		t.Fatalf("BuildBaseCurrentStateUserIsomorphismContract(): %v", err)
	}
	evidence := BaseDurableStateSliceEvidence{
		EquivalenceContractRef:      "contract:base-substrate-equivalence-pass-87",
		UserIsomorphismContractRef:  "contract:base-current-state-user-isomorphism-pass-88",
		TypedArtifactProgramRef:     string(equivalence.Version.ArtifactProgramRef),
		DurableSliceEvidenceRef:     "durable-slice:base-file-manifest-blob-set",
		NoOpaqueDataImageDependency: true,
		NoMutation:                  true,
	}
	return equivalence, user, evidence
}

func assertBaseDurableStateClasses(t *testing.T, got, want []BaseDurableStateClass) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("persistent state classes = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("persistent state classes = %#v, want %#v", got, want)
		}
	}
}

func withoutUserSemantic(semantics []UserSemantic, drop UserSemantic) []UserSemantic {
	out := make([]UserSemantic, 0, len(semantics))
	for _, semantic := range semantics {
		if semantic != drop {
			out = append(out, semantic)
		}
	}
	return out
}
