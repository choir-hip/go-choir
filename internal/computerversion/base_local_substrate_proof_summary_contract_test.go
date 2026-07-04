package computerversion

import (
	"strings"
	"testing"
)

func TestBuildBaseLocalSubstrateProofSummaryContractBuildsScopedLocalSummary(t *testing.T) {
	substrate, reentry, evidence := baseLocalSubstrateProofSummaryContractInputs(t)

	contract, err := BuildBaseLocalSubstrateProofSummaryContract(substrate, reentry, evidence)
	if err != nil {
		t.Fatalf("BuildBaseLocalSubstrateProofSummaryContract(): %v", err)
	}

	if contract.Kind != BaseLocalSubstrateProofSummaryContractKind {
		t.Fatalf("kind = %q, want %q", contract.Kind, BaseLocalSubstrateProofSummaryContractKind)
	}
	if contract.Version != substrate.Version || contract.Version != reentry.Version {
		t.Fatalf("version = %#v, want shared substrate/reentry version %#v/%#v", contract.Version, substrate.Version, reentry.Version)
	}
	if contract.Boundary != BaseLocalSubstrateProofSummaryBoundary {
		t.Fatalf("boundary = %q, want %q", contract.Boundary, BaseLocalSubstrateProofSummaryBoundary)
	}
	if contract.Scope != BaseLocalSubstrateProofSummaryScope {
		t.Fatalf("scope = %q, want %q", contract.Scope, BaseLocalSubstrateProofSummaryScope)
	}
	if contract.ClaimScope != BaseSubstrateEquivalenceClaimScope || contract.ClaimScope != reentry.ClaimScope {
		t.Fatalf("claim scope = %q, want substrate/reentry scope %q/%q", contract.ClaimScope, BaseSubstrateEquivalenceClaimScope, reentry.ClaimScope)
	}
	if contract.CurrentMaterializer != substrate.CurrentMaterializer || contract.CurrentSubstrate != substrate.CurrentSubstrate || contract.ProjectionMaterializer != substrate.ProjectionMaterializer || contract.ProjectionSubstrate != substrate.ProjectionSubstrate {
		t.Fatalf("substrate identities = current %q/%q projection %q/%q, want %#v", contract.CurrentMaterializer, contract.CurrentSubstrate, contract.ProjectionMaterializer, contract.ProjectionSubstrate, substrate)
	}
	if contract.CurrentMaterializer != reentry.CurrentMaterializer || contract.CurrentSubstrate != reentry.CurrentSubstrate || contract.ProjectionMaterializer != reentry.ProjectionMaterializer || contract.ProjectionSubstrate != reentry.ProjectionSubstrate {
		t.Fatalf("summary identities must bind the same reentry contract identities: summary %#v reentry %#v", contract, reentry)
	}
	if contract.SubstrateEquivalenceStatus != EquivalenceEquivalent {
		t.Fatalf("substrate equivalence status = %q, want %q", contract.SubstrateEquivalenceStatus, EquivalenceEquivalent)
	}
	if !contract.ReentryAllowed {
		t.Fatalf("reentry allowed = false, want true")
	}
	assertObservationBundleKinds(t, contract.RequiredObservations, []ObservationKind{ObservationBlobSet, ObservationFileManifest})
	if contract.SubstrateEquivalenceContractRef != strings.TrimSpace(evidence.SubstrateEquivalenceContractRef) || contract.SubstrateEquivalenceContractRef != reentry.SubstrateEquivalenceContractRef {
		t.Fatalf("substrate ref = %q, want trimmed evidence ref matching reentry %q", contract.SubstrateEquivalenceContractRef, reentry.SubstrateEquivalenceContractRef)
	}
	if contract.ReentryReadinessContractRef != strings.TrimSpace(evidence.ReentryReadinessContractRef) {
		t.Fatalf("reentry ref = %q, want trimmed %q", contract.ReentryReadinessContractRef, strings.TrimSpace(evidence.ReentryReadinessContractRef))
	}
	if contract.EquivalenceEvidenceSetRef != strings.TrimSpace(evidence.EquivalenceEvidenceSetRef) || contract.EquivalenceEvidenceSetRef != reentry.EquivalenceEvidenceSetRef {
		t.Fatalf("evidence set ref = %q, want trimmed evidence ref matching reentry %q", contract.EquivalenceEvidenceSetRef, reentry.EquivalenceEvidenceSetRef)
	}
	if contract.SummaryRef != strings.TrimSpace(evidence.SummaryRef) {
		t.Fatalf("summary ref = %q, want trimmed %q", contract.SummaryRef, strings.TrimSpace(evidence.SummaryRef))
	}
	wantGaps := map[string]bool{
		BaseLocalSubstrateProofSummaryRemainingRuntimeSubstrateProof: false,
		BaseLocalSubstrateProofSummaryRemainingStagingProof:          false,
		BaseLocalSubstrateProofSummaryRemainingPromotionProof:        false,
	}
	if len(contract.RemainingGaps) != len(wantGaps) {
		t.Fatalf("remaining gaps = %#v, want exactly runtime/staging/promotion proof requirements", contract.RemainingGaps)
	}
	for _, gap := range contract.RemainingGaps {
		if _, ok := wantGaps[gap]; !ok {
			t.Fatalf("remaining gaps = %#v, want exactly runtime/staging/promotion proof requirements", contract.RemainingGaps)
		}
		wantGaps[gap] = true
	}
	for gap, seen := range wantGaps {
		if !seen {
			t.Fatalf("remaining gaps = %#v, missing %q", contract.RemainingGaps, gap)
		}
	}
	if !contract.LocalFileBlobProofSummarized {
		t.Fatalf("local file/blob proof summarized = false, want true")
	}
	if !contract.RuntimeSubstrateProofRequired || !contract.StagingProofRequired || !contract.PromotionProofRequired {
		t.Fatalf("runtime/staging/promotion proof gaps must remain required: %#v", contract)
	}
	if !contract.NoRuntimeMaterialization || !contract.NoOpaqueDataImageDependency || !contract.NoMutation {
		t.Fatalf("summary must emit no-runtime/no-opaque/no-mutation flags: %#v", contract)
	}
	if contract.RuntimeBehaviorChanged || contract.DeployedRouteRegistered || contract.ProductionAuthTouched || contract.StagingClaimed || contract.PromotionClaimed || contract.VMLifecycleTouched || contract.FirecrackerBootClaimed || contract.RunAcceptanceRecordTouched || contract.FullSubstrateIndependenceClaim || contract.PackagePublicationClaimed || contract.CompletionClaimed {
		t.Fatalf("summary must not claim runtime, staging, VM lifecycle, promotion, package publication, full substrate independence, run acceptance, or completion: %#v", contract)
	}
}

func TestBuildBaseLocalSubstrateProofSummaryContractRejectsInvalidInputs(t *testing.T) {
	foreignVersion := ComputerVersion{CodeRef: "git:foreign-base-local-substrate-proof-summary", ArtifactProgramRef: "base-journal:owner/main@cursor-foreign-base-local-substrate-proof-summary"}
	for _, tc := range []struct {
		name    string
		mutate  func(*BaseSubstrateEquivalenceContract, *BaseSubstrateReentryReadinessContract, *BaseLocalSubstrateProofSummaryEvidence)
		wantErr string
	}{
		{
			name: "wrong substrate kind",
			mutate: func(substrate *BaseSubstrateEquivalenceContract, _ *BaseSubstrateReentryReadinessContract, _ *BaseLocalSubstrateProofSummaryEvidence) {
				substrate.Kind = BaseSubstrateReentryReadinessContractKind
			},
			wantErr: "substrate contract kind",
		},
		{
			name: "wrong substrate boundary",
			mutate: func(substrate *BaseSubstrateEquivalenceContract, _ *BaseSubstrateReentryReadinessContract, _ *BaseLocalSubstrateProofSummaryEvidence) {
				substrate.Boundary = BaseLocalSubstrateProofSummaryBoundary
			},
			wantErr: "substrate contract boundary",
		},
		{
			name: "wrong substrate claim scope",
			mutate: func(substrate *BaseSubstrateEquivalenceContract, _ *BaseSubstrateReentryReadinessContract, _ *BaseLocalSubstrateProofSummaryEvidence) {
				substrate.ClaimScope = BaseSubstrateReentryReadinessScope
			},
			wantErr: "substrate claim scope",
		},
		{
			name: "substrate missing file manifest scope",
			mutate: func(substrate *BaseSubstrateEquivalenceContract, _ *BaseSubstrateReentryReadinessContract, _ *BaseLocalSubstrateProofSummaryEvidence) {
				substrate.RequiredObservations = []ObservationKind{ObservationBlobSet}
			},
			wantErr: "substrate contract must include file_manifest and blob_set",
		},
		{
			name: "substrate missing blob set scope",
			mutate: func(substrate *BaseSubstrateEquivalenceContract, _ *BaseSubstrateReentryReadinessContract, _ *BaseLocalSubstrateProofSummaryEvidence) {
				substrate.RequiredObservations = []ObservationKind{ObservationFileManifest}
			},
			wantErr: "substrate contract must include file_manifest and blob_set",
		},
		{
			name: "wrong reentry kind",
			mutate: func(_ *BaseSubstrateEquivalenceContract, reentry *BaseSubstrateReentryReadinessContract, _ *BaseLocalSubstrateProofSummaryEvidence) {
				reentry.Kind = BaseSubstrateEquivalenceContractKind
			},
			wantErr: "reentry contract kind",
		},
		{
			name: "wrong reentry boundary",
			mutate: func(_ *BaseSubstrateEquivalenceContract, reentry *BaseSubstrateReentryReadinessContract, _ *BaseLocalSubstrateProofSummaryEvidence) {
				reentry.Boundary = BaseLocalSubstrateProofSummaryBoundary
			},
			wantErr: "reentry contract boundary",
		},
		{
			name: "wrong reentry scope",
			mutate: func(_ *BaseSubstrateEquivalenceContract, reentry *BaseSubstrateReentryReadinessContract, _ *BaseLocalSubstrateProofSummaryEvidence) {
				reentry.Scope = BaseLocalSubstrateProofSummaryScope
			},
			wantErr: "reentry contract scope",
		},
		{
			name: "reentry missing blob set scope",
			mutate: func(_ *BaseSubstrateEquivalenceContract, reentry *BaseSubstrateReentryReadinessContract, _ *BaseLocalSubstrateProofSummaryEvidence) {
				reentry.RequiredObservations = []ObservationKind{ObservationFileManifest}
			},
			wantErr: "reentry contract must include file_manifest and blob_set",
		},
		{
			name: "version drift",
			mutate: func(_ *BaseSubstrateEquivalenceContract, reentry *BaseSubstrateReentryReadinessContract, _ *BaseLocalSubstrateProofSummaryEvidence) {
				reentry.Version = foreignVersion
			},
			wantErr: "substrate and reentry contracts name different computer versions",
		},
		{
			name: "claim scope drift",
			mutate: func(_ *BaseSubstrateEquivalenceContract, reentry *BaseSubstrateReentryReadinessContract, _ *BaseLocalSubstrateProofSummaryEvidence) {
				reentry.ClaimScope = BaseSubstrateReentryReadinessScope
			},
			wantErr: "substrate and reentry claim scopes differ",
		},
		{
			name: "identity drift",
			mutate: func(_ *BaseSubstrateEquivalenceContract, reentry *BaseSubstrateReentryReadinessContract, _ *BaseLocalSubstrateProofSummaryEvidence) {
				reentry.ProjectionSubstrate = "base:file-projection-drift"
			},
			wantErr: "substrate and reentry materializer identities differ",
		},
		{
			name: "reentry not allowed",
			mutate: func(_ *BaseSubstrateEquivalenceContract, reentry *BaseSubstrateReentryReadinessContract, _ *BaseLocalSubstrateProofSummaryEvidence) {
				reentry.LocalSubstrateReentryAllowed = false
			},
			wantErr: "reentry contract does not allow local substrate reentry",
		},
		{
			name: "reentry missing proof refs",
			mutate: func(_ *BaseSubstrateEquivalenceContract, reentry *BaseSubstrateReentryReadinessContract, _ *BaseLocalSubstrateProofSummaryEvidence) {
				reentry.NextProbeRef = "  "
			},
			wantErr: "reentry contract must carry proof refs",
		},
		{
			name: "missing substrate equivalence ref",
			mutate: func(_ *BaseSubstrateEquivalenceContract, _ *BaseSubstrateReentryReadinessContract, evidence *BaseLocalSubstrateProofSummaryEvidence) {
				evidence.SubstrateEquivalenceContractRef = "  "
			},
			wantErr: "substrate equivalence contract ref is required",
		},
		{
			name: "missing reentry readiness ref",
			mutate: func(_ *BaseSubstrateEquivalenceContract, _ *BaseSubstrateReentryReadinessContract, evidence *BaseLocalSubstrateProofSummaryEvidence) {
				evidence.ReentryReadinessContractRef = ""
			},
			wantErr: "reentry readiness contract ref is required",
		},
		{
			name: "missing equivalence evidence set ref",
			mutate: func(_ *BaseSubstrateEquivalenceContract, _ *BaseSubstrateReentryReadinessContract, evidence *BaseLocalSubstrateProofSummaryEvidence) {
				evidence.EquivalenceEvidenceSetRef = "\t"
			},
			wantErr: "equivalence evidence set ref is required",
		},
		{
			name: "missing summary ref",
			mutate: func(_ *BaseSubstrateEquivalenceContract, _ *BaseSubstrateReentryReadinessContract, evidence *BaseLocalSubstrateProofSummaryEvidence) {
				evidence.SummaryRef = "  "
			},
			wantErr: "summary ref is required",
		},
		{
			name: "substrate ref mismatch with reentry",
			mutate: func(_ *BaseSubstrateEquivalenceContract, _ *BaseSubstrateReentryReadinessContract, evidence *BaseLocalSubstrateProofSummaryEvidence) {
				evidence.SubstrateEquivalenceContractRef = "base-substrate-equivalence-contract:other"
			},
			wantErr: "substrate equivalence contract ref does not match reentry",
		},
		{
			name: "equivalence evidence set ref mismatch with reentry",
			mutate: func(_ *BaseSubstrateEquivalenceContract, _ *BaseSubstrateReentryReadinessContract, evidence *BaseLocalSubstrateProofSummaryEvidence) {
				evidence.EquivalenceEvidenceSetRef = "base-equivalence-evidence-set-contract:other"
			},
			wantErr: "equivalence evidence set ref does not match reentry",
		},
		{
			name: "missing runtime proof gap",
			mutate: func(_ *BaseSubstrateEquivalenceContract, _ *BaseSubstrateReentryReadinessContract, evidence *BaseLocalSubstrateProofSummaryEvidence) {
				evidence.RemainingGaps = []string{BaseLocalSubstrateProofSummaryRemainingStagingProof, BaseLocalSubstrateProofSummaryRemainingPromotionProof}
			},
			wantErr: "remaining gaps must preserve runtime, staging, and promotion proof requirements",
		},
		{
			name: "missing staging proof gap",
			mutate: func(_ *BaseSubstrateEquivalenceContract, _ *BaseSubstrateReentryReadinessContract, evidence *BaseLocalSubstrateProofSummaryEvidence) {
				evidence.RemainingGaps = []string{BaseLocalSubstrateProofSummaryRemainingRuntimeSubstrateProof, BaseLocalSubstrateProofSummaryRemainingPromotionProof}
			},
			wantErr: "remaining gaps must preserve runtime, staging, and promotion proof requirements",
		},
		{
			name: "missing promotion proof gap",
			mutate: func(_ *BaseSubstrateEquivalenceContract, _ *BaseSubstrateReentryReadinessContract, evidence *BaseLocalSubstrateProofSummaryEvidence) {
				evidence.RemainingGaps = []string{BaseLocalSubstrateProofSummaryRemainingRuntimeSubstrateProof, BaseLocalSubstrateProofSummaryRemainingStagingProof}
			},
			wantErr: "remaining gaps must preserve runtime, staging, and promotion proof requirements",
		},
		{
			name: "substrate unsafe no runtime flag",
			mutate: func(substrate *BaseSubstrateEquivalenceContract, _ *BaseSubstrateReentryReadinessContract, _ *BaseLocalSubstrateProofSummaryEvidence) {
				substrate.NoRuntimeMaterialization = false
			},
			wantErr: "substrate contract has unsafe proof flags",
		},
		{
			name: "substrate unsafe no opaque flag",
			mutate: func(substrate *BaseSubstrateEquivalenceContract, _ *BaseSubstrateReentryReadinessContract, _ *BaseLocalSubstrateProofSummaryEvidence) {
				substrate.NoOpaqueDataImageDependency = false
			},
			wantErr: "substrate contract has unsafe proof flags",
		},
		{
			name: "substrate unsafe no mutation flag",
			mutate: func(substrate *BaseSubstrateEquivalenceContract, _ *BaseSubstrateReentryReadinessContract, _ *BaseLocalSubstrateProofSummaryEvidence) {
				substrate.NoMutation = false
			},
			wantErr: "substrate contract has unsafe proof flags",
		},
		{
			name: "substrate Firecracker boot claimed",
			mutate: func(substrate *BaseSubstrateEquivalenceContract, _ *BaseSubstrateReentryReadinessContract, _ *BaseLocalSubstrateProofSummaryEvidence) {
				substrate.FirecrackerBootClaimed = true
			},
			wantErr: "substrate contract carries protected-surface claims",
		},
		{
			name: "substrate full substrate independence claimed",
			mutate: func(substrate *BaseSubstrateEquivalenceContract, _ *BaseSubstrateReentryReadinessContract, _ *BaseLocalSubstrateProofSummaryEvidence) {
				substrate.FullSubstrateIndependenceClaim = true
			},
			wantErr: "substrate contract carries protected-surface claims",
		},
		{
			name: "substrate completion claimed",
			mutate: func(substrate *BaseSubstrateEquivalenceContract, _ *BaseSubstrateReentryReadinessContract, _ *BaseLocalSubstrateProofSummaryEvidence) {
				substrate.CompletionClaimed = true
			},
			wantErr: "substrate contract carries protected-surface claims",
		},
		{
			name: "reentry unsafe no runtime flag",
			mutate: func(_ *BaseSubstrateEquivalenceContract, reentry *BaseSubstrateReentryReadinessContract, _ *BaseLocalSubstrateProofSummaryEvidence) {
				reentry.NoRuntimeMaterialization = false
			},
			wantErr: "reentry contract has unsafe proof flags",
		},
		{
			name: "reentry unsafe no opaque flag",
			mutate: func(_ *BaseSubstrateEquivalenceContract, reentry *BaseSubstrateReentryReadinessContract, _ *BaseLocalSubstrateProofSummaryEvidence) {
				reentry.NoOpaqueDataImageDependency = false
			},
			wantErr: "reentry contract has unsafe proof flags",
		},
		{
			name: "reentry unsafe no mutation flag",
			mutate: func(_ *BaseSubstrateEquivalenceContract, reentry *BaseSubstrateReentryReadinessContract, _ *BaseLocalSubstrateProofSummaryEvidence) {
				reentry.NoMutation = false
			},
			wantErr: "reentry contract has unsafe proof flags",
		},
		{
			name: "reentry Firecracker boot claimed",
			mutate: func(_ *BaseSubstrateEquivalenceContract, reentry *BaseSubstrateReentryReadinessContract, _ *BaseLocalSubstrateProofSummaryEvidence) {
				reentry.FirecrackerBootClaimed = true
			},
			wantErr: "reentry contract carries protected-surface claims",
		},
		{
			name: "reentry full substrate independence claimed",
			mutate: func(_ *BaseSubstrateEquivalenceContract, reentry *BaseSubstrateReentryReadinessContract, _ *BaseLocalSubstrateProofSummaryEvidence) {
				reentry.FullSubstrateIndependenceClaim = true
			},
			wantErr: "reentry contract carries protected-surface claims",
		},
		{
			name: "reentry completion claimed",
			mutate: func(_ *BaseSubstrateEquivalenceContract, reentry *BaseSubstrateReentryReadinessContract, _ *BaseLocalSubstrateProofSummaryEvidence) {
				reentry.CompletionClaimed = true
			},
			wantErr: "reentry contract carries protected-surface claims",
		},
		{
			name: "evidence unsafe no runtime flag",
			mutate: func(_ *BaseSubstrateEquivalenceContract, _ *BaseSubstrateReentryReadinessContract, evidence *BaseLocalSubstrateProofSummaryEvidence) {
				evidence.NoRuntimeMaterialization = false
			},
			wantErr: "evidence must prove no runtime materialization",
		},
		{
			name: "evidence unsafe no opaque flag",
			mutate: func(_ *BaseSubstrateEquivalenceContract, _ *BaseSubstrateReentryReadinessContract, evidence *BaseLocalSubstrateProofSummaryEvidence) {
				evidence.NoOpaqueDataImageDependency = false
			},
			wantErr: "evidence must prove no opaque data.img dependency",
		},
		{
			name: "evidence unsafe no mutation flag",
			mutate: func(_ *BaseSubstrateEquivalenceContract, _ *BaseSubstrateReentryReadinessContract, evidence *BaseLocalSubstrateProofSummaryEvidence) {
				evidence.NoMutation = false
			},
			wantErr: "evidence must be no-mutation",
		},
		{
			name: "evidence runtime behavior changed",
			mutate: func(_ *BaseSubstrateEquivalenceContract, _ *BaseSubstrateReentryReadinessContract, evidence *BaseLocalSubstrateProofSummaryEvidence) {
				evidence.RuntimeBehaviorChanged = true
			},
			wantErr: "evidence carries protected-surface or completion claims",
		},
		{
			name: "evidence staging claimed",
			mutate: func(_ *BaseSubstrateEquivalenceContract, _ *BaseSubstrateReentryReadinessContract, evidence *BaseLocalSubstrateProofSummaryEvidence) {
				evidence.StagingClaimed = true
			},
			wantErr: "evidence carries protected-surface or completion claims",
		},
		{
			name: "evidence promotion claimed",
			mutate: func(_ *BaseSubstrateEquivalenceContract, _ *BaseSubstrateReentryReadinessContract, evidence *BaseLocalSubstrateProofSummaryEvidence) {
				evidence.PromotionClaimed = true
			},
			wantErr: "evidence carries protected-surface or completion claims",
		},
		{
			name: "evidence VM lifecycle touched",
			mutate: func(_ *BaseSubstrateEquivalenceContract, _ *BaseSubstrateReentryReadinessContract, evidence *BaseLocalSubstrateProofSummaryEvidence) {
				evidence.VMLifecycleTouched = true
			},
			wantErr: "evidence carries protected-surface or completion claims",
		},
		{
			name: "evidence package publication claimed",
			mutate: func(_ *BaseSubstrateEquivalenceContract, _ *BaseSubstrateReentryReadinessContract, evidence *BaseLocalSubstrateProofSummaryEvidence) {
				evidence.PackagePublicationClaimed = true
			},
			wantErr: "evidence carries protected-surface or completion claims",
		},
		{
			name: "evidence full substrate independence claimed",
			mutate: func(_ *BaseSubstrateEquivalenceContract, _ *BaseSubstrateReentryReadinessContract, evidence *BaseLocalSubstrateProofSummaryEvidence) {
				evidence.FullSubstrateIndependenceClaim = true
			},
			wantErr: "evidence carries protected-surface or completion claims",
		},
		{
			name: "evidence Firecracker boot claimed",
			mutate: func(_ *BaseSubstrateEquivalenceContract, _ *BaseSubstrateReentryReadinessContract, evidence *BaseLocalSubstrateProofSummaryEvidence) {
				evidence.FirecrackerBootClaimed = true
			},
			wantErr: "evidence carries protected-surface or completion claims",
		},
		{
			name: "evidence run acceptance record touched",
			mutate: func(_ *BaseSubstrateEquivalenceContract, _ *BaseSubstrateReentryReadinessContract, evidence *BaseLocalSubstrateProofSummaryEvidence) {
				evidence.RunAcceptanceRecordTouched = true
			},
			wantErr: "evidence carries protected-surface or completion claims",
		},
		{
			name: "evidence completion claimed",
			mutate: func(_ *BaseSubstrateEquivalenceContract, _ *BaseSubstrateReentryReadinessContract, evidence *BaseLocalSubstrateProofSummaryEvidence) {
				evidence.CompletionClaimed = true
			},
			wantErr: "evidence carries protected-surface or completion claims",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			substrate, reentry, evidence := baseLocalSubstrateProofSummaryContractInputs(t)
			tc.mutate(&substrate, &reentry, &evidence)

			contract, err := BuildBaseLocalSubstrateProofSummaryContract(substrate, reentry, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildBaseLocalSubstrateProofSummaryContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func baseLocalSubstrateProofSummaryContractInputs(t *testing.T) (BaseSubstrateEquivalenceContract, BaseSubstrateReentryReadinessContract, BaseLocalSubstrateProofSummaryEvidence) {
	t.Helper()

	substrate, calibration, reentryEvidence := baseSubstrateReentryReadinessContractInputs(t)
	reentry, err := BuildBaseSubstrateReentryReadinessContract(substrate, calibration, reentryEvidence)
	if err != nil {
		t.Fatalf("BuildBaseSubstrateReentryReadinessContract(): %v", err)
	}

	evidence := BaseLocalSubstrateProofSummaryEvidence{
		SubstrateEquivalenceContractRef: "  " + reentry.SubstrateEquivalenceContractRef + "  ",
		ReentryReadinessContractRef:     " base-substrate-reentry-readiness-contract:pass-97 ",
		EquivalenceEvidenceSetRef:       "\t" + reentry.EquivalenceEvidenceSetRef + "\t",
		SummaryRef:                      " base-local-substrate-proof-summary:pass-97 ",
		RemainingGaps: []string{
			BaseLocalSubstrateProofSummaryRemainingPromotionProof,
			BaseLocalSubstrateProofSummaryRemainingRuntimeSubstrateProof,
			" ",
			BaseLocalSubstrateProofSummaryRemainingStagingProof,
			BaseLocalSubstrateProofSummaryRemainingRuntimeSubstrateProof,
		},
		NoRuntimeMaterialization:    true,
		NoOpaqueDataImageDependency: true,
		NoMutation:                  true,
	}
	return substrate, reentry, evidence
}
