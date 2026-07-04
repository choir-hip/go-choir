package computerversion

import (
	"strings"
	"testing"
)

func TestBuildBaseDurableStateSliceProbeContractBuildsScopedProbeResult(t *testing.T) {
	readiness, durable, evidence := baseDurableStateSliceProbeContractInputs(t)

	contract, err := BuildBaseDurableStateSliceProbeContract(readiness, durable, evidence)
	if err != nil {
		t.Fatalf("BuildBaseDurableStateSliceProbeContract(): %v", err)
	}

	if contract.Kind != BaseDurableStateSliceProbeContractKind {
		t.Fatalf("kind = %q, want %q", contract.Kind, BaseDurableStateSliceProbeContractKind)
	}
	if contract.Version != readiness.Version {
		t.Fatalf("version = %#v, want readiness version %#v", contract.Version, readiness.Version)
	}
	if contract.Boundary != BaseDurableStateSliceProbeBoundary {
		t.Fatalf("boundary = %q, want %q", contract.Boundary, BaseDurableStateSliceProbeBoundary)
	}
	if contract.Scope != BaseDurableStateSliceProbeScope {
		t.Fatalf("scope = %q, want %q", contract.Scope, BaseDurableStateSliceProbeScope)
	}
	if contract.TypedArtifactProgramRef != strings.TrimSpace(durable.TypedArtifactProgramRef) {
		t.Fatalf("typed artifact-program ref = %q, want durable ref %q", contract.TypedArtifactProgramRef, durable.TypedArtifactProgramRef)
	}
	if contract.DurableStateSliceReadinessRef != strings.TrimSpace(evidence.DurableStateSliceReadinessRef) || contract.DurableStateSliceContractRef != strings.TrimSpace(evidence.DurableStateSliceContractRef) {
		t.Fatalf("durable-state-slice refs = readiness %q contract %q, want trimmed evidence refs %#v", contract.DurableStateSliceReadinessRef, contract.DurableStateSliceContractRef, evidence)
	}
	if contract.PostPromotionSettlementHandoffRef != strings.TrimSpace(readiness.PostPromotionSettlementHandoffRef) || contract.NextSubstrateProofPlanRef != strings.TrimSpace(readiness.NextSubstrateProofPlanRef) {
		t.Fatalf("readiness refs = handoff %q next-plan %q, want trimmed readiness refs from %#v", contract.PostPromotionSettlementHandoffRef, contract.NextSubstrateProofPlanRef, readiness)
	}
	if contract.FileManifestProbeRef != strings.TrimSpace(evidence.FileManifestProbeRef) || contract.BlobContentProbeRef != strings.TrimSpace(evidence.BlobContentProbeRef) || contract.ProbeEvidenceRef != strings.TrimSpace(evidence.ProbeEvidenceRef) || contract.ResidualRiskRef != strings.TrimSpace(evidence.ResidualRiskRef) || contract.RollbackPlanRef != strings.TrimSpace(evidence.RollbackPlanRef) {
		t.Fatalf("probe refs = file %q blob %q evidence %q residual %q rollback %q, want trimmed evidence refs from %#v", contract.FileManifestProbeRef, contract.BlobContentProbeRef, contract.ProbeEvidenceRef, contract.ResidualRiskRef, contract.RollbackPlanRef, evidence)
	}
	if contract.ProbeStatus != BaseDurableStateSliceProbeStatusProven {
		t.Fatalf("probe status = %q, want %q", contract.ProbeStatus, BaseDurableStateSliceProbeStatusProven)
	}
	assertBaseDurableStateClasses(t, contract.PersistentStateClasses, []BaseDurableStateClass{BaseDurableStateClassBlobContent, BaseDurableStateClassFileManifest})
	assertObservationBundleKinds(t, contract.RequiredObservations, []ObservationKind{ObservationBlobSet, ObservationFileManifest})
	assertUserSemantics(t, contract.RequiredSemantics, []UserSemantic{UserSemanticDeletionState, UserSemanticFileContent, UserSemanticFilePath, UserSemanticFileProvenance})
	if !contract.ReadinessConsumed || !contract.DurableStateSliceProven || !contract.FileManifestProbeRecorded || !contract.BlobContentProbeRecorded {
		t.Fatalf("probe must consume readiness and record durable/file/blob proof: %#v", contract)
	}
	if !contract.RuntimeProofRequired || !contract.StagingProofRequired || !contract.PromotionProofRequired || !contract.RunAcceptanceProofRequired || !contract.FullSubstrateProofRequired {
		t.Fatalf("probe must preserve runtime/staging/promotion/run-acceptance/full-substrate proof obligations: %#v", contract)
	}
	if contract.RuntimeMaterializationAllowed || contract.DurableComputerMutationAllowed || contract.PackagePublicationAllowed || contract.PromotionAllowed || contract.RunAcceptanceSynthesisAllowed {
		t.Fatalf("probe must deny runtime materialization, durable mutation, package publication, promotion, and run-acceptance authority: %#v", contract)
	}
	if !contract.NoRuntimeMaterialization || !contract.NoOpaqueDataImageDependency || !contract.NoDurableComputerMutation || !contract.NoPackagePublicationMutation || !contract.NoPromotionMutation || !contract.NoRunAcceptanceMutation || !contract.NoProductionMutation {
		t.Fatalf("probe must carry no-runtime/no-opaque/no-durable/no-package/no-promotion/no-run/no-production flags: %#v", contract)
	}
	if contract.RuntimeMaterialized || contract.DurableComputerStateMutated || contract.PackagePublished || contract.PromotionExecuted || contract.RunAcceptanceRecordTouched || contract.ProductionStateMutated || contract.FullSubstrateClaimed || contract.CompletionClaimed {
		t.Fatalf("probe must not materialize runtime, mutate durable computer or production, publish, promote, touch run acceptance, claim full-substrate, or claim completion: %#v", contract)
	}
}

func TestBuildBaseDurableStateSliceProbeContractRejectsInvalidInputs(t *testing.T) {
	foreignVersion := ComputerVersion{CodeRef: "git:foreign-base-durable-state-slice-probe", ArtifactProgramRef: "base-journal:owner/main@cursor-foreign-base-durable-state-slice-probe"}
	for _, tc := range []struct {
		name    string
		mutate  func(*BaseDurableStateSliceReadinessContract, *BaseDurableStateSliceContract, *BaseDurableStateSliceProbeEvidence)
		wantErr string
	}{
		{
			name: "readiness kind drift",
			mutate: func(readiness *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				readiness.Kind = BasePostPromotionSettlementHandoffReadinessContractKind
			},
			wantErr: "readiness kind",
		},
		{
			name: "readiness boundary drift",
			mutate: func(readiness *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				readiness.Boundary = BasePostPromotionSettlementHandoffReadinessBoundary
			},
			wantErr: "readiness boundary",
		},
		{
			name: "readiness scope drift",
			mutate: func(readiness *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				readiness.Scope = BasePostPromotionSettlementHandoffReadinessScope
			},
			wantErr: "readiness scope",
		},
		{
			name: "readiness invalid version",
			mutate: func(readiness *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				readiness.Version.CodeRef = "  "
			},
			wantErr: "readiness version is invalid",
		},
		{
			name: "readiness typed artifact ref drift",
			mutate: func(readiness *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				readiness.TypedArtifactProgramRef = "base-journal:owner/main@cursor-drifted-probe"
			},
			wantErr: "readiness typed artifact-program ref is invalid",
		},
		{
			name: "readiness missing handoff ref",
			mutate: func(readiness *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				readiness.PostPromotionSettlementHandoffRef = "\t"
			},
			wantErr: "readiness refs are required",
		},
		{
			name: "readiness missing next substrate plan ref",
			mutate: func(readiness *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				readiness.NextSubstrateProofPlanRef = "  "
			},
			wantErr: "readiness refs are required",
		},
		{
			name: "readiness missing durable slice ref",
			mutate: func(readiness *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				readiness.DurableStateSlicePlanRef = ""
			},
			wantErr: "readiness refs are required",
		},
		{
			name: "readiness missing file manifest probe ref",
			mutate: func(readiness *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				readiness.FileManifestProbeRef = " "
			},
			wantErr: "readiness refs are required",
		},
		{
			name: "readiness missing blob content probe ref",
			mutate: func(readiness *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				readiness.BlobContentProbeRef = " "
			},
			wantErr: "readiness refs are required",
		},
		{
			name: "readiness missing residual risk ref",
			mutate: func(readiness *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				readiness.ResidualRiskRef = " "
			},
			wantErr: "readiness refs are required",
		},
		{
			name: "readiness missing rollback plan ref",
			mutate: func(readiness *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				readiness.RollbackPlanRef = " "
			},
			wantErr: "readiness refs are required",
		},
		{
			name: "readiness status drift",
			mutate: func(readiness *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				readiness.ReadinessStatus = "materialized"
			},
			wantErr: "readiness status",
		},
		{
			name: "readiness handoff prerequisite drift",
			mutate: func(readiness *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				readiness.PostSettlementHandoffRecorded = false
			},
			wantErr: "readiness must preserve durable-state-slice prerequisites",
		},
		{
			name: "readiness durable probe prerequisite drift",
			mutate: func(readiness *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				readiness.DurableStateSliceProbeRequired = false
			},
			wantErr: "readiness must preserve durable-state-slice prerequisites",
		},
		{
			name: "readiness file manifest prerequisite drift",
			mutate: func(readiness *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				readiness.FileManifestProbeRequired = false
			},
			wantErr: "readiness must preserve durable-state-slice prerequisites",
		},
		{
			name: "readiness blob content prerequisite drift",
			mutate: func(readiness *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				readiness.BlobContentProbeRequired = false
			},
			wantErr: "readiness must preserve durable-state-slice prerequisites",
		},
		{
			name: "readiness observation prerequisite drift",
			mutate: func(readiness *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				readiness.ObservationSetRequired = false
			},
			wantErr: "readiness must preserve durable-state-slice prerequisites",
		},
		{
			name: "readiness materializer prerequisite drift",
			mutate: func(readiness *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				readiness.MaterializerContractRequired = false
			},
			wantErr: "readiness must preserve durable-state-slice prerequisites",
		},
		{
			name: "readiness equivalence prerequisite drift",
			mutate: func(readiness *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				readiness.EquivalenceCheckRequired = false
			},
			wantErr: "readiness must preserve durable-state-slice prerequisites",
		},
		{
			name: "readiness promotion proof requirement drift",
			mutate: func(readiness *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				readiness.PromotionProofRequired = false
			},
			wantErr: "readiness must preserve downstream proof requirements",
		},
		{
			name: "readiness run acceptance proof requirement drift",
			mutate: func(readiness *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				readiness.RunAcceptanceProofRequired = false
			},
			wantErr: "readiness must preserve downstream proof requirements",
		},
		{
			name: "readiness full substrate proof requirement drift",
			mutate: func(readiness *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				readiness.FullSubstrateProofRequired = false
			},
			wantErr: "readiness must preserve downstream proof requirements",
		},
		{
			name: "readiness allows runtime materialization",
			mutate: func(readiness *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				readiness.RuntimeMaterializationAllowed = true
			},
			wantErr: "readiness allows downstream execution",
		},
		{
			name: "readiness allows durable computer mutation",
			mutate: func(readiness *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				readiness.DurableComputerMutationAllowed = true
			},
			wantErr: "readiness allows downstream execution",
		},
		{
			name: "readiness allows package publication",
			mutate: func(readiness *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				readiness.PackagePublicationAllowed = true
			},
			wantErr: "readiness allows downstream execution",
		},
		{
			name: "readiness allows promotion",
			mutate: func(readiness *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				readiness.PromotionAllowed = true
			},
			wantErr: "readiness allows downstream execution",
		},
		{
			name: "readiness allows run acceptance synthesis",
			mutate: func(readiness *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				readiness.RunAcceptanceSynthesisAllowed = true
			},
			wantErr: "readiness allows downstream execution",
		},
		{
			name: "readiness missing no runtime materialization flag",
			mutate: func(readiness *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				readiness.NoRuntimeMaterialization = false
			},
			wantErr: "readiness must prove no runtime, durable-computer, package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "readiness missing no durable computer mutation flag",
			mutate: func(readiness *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				readiness.NoDurableComputerMutation = false
			},
			wantErr: "readiness must prove no runtime, durable-computer, package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "readiness missing no package publication mutation flag",
			mutate: func(readiness *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				readiness.NoPackagePublicationMutation = false
			},
			wantErr: "readiness must prove no runtime, durable-computer, package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "readiness missing no promotion mutation flag",
			mutate: func(readiness *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				readiness.NoPromotionMutation = false
			},
			wantErr: "readiness must prove no runtime, durable-computer, package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "readiness missing no run acceptance mutation flag",
			mutate: func(readiness *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				readiness.NoRunAcceptanceMutation = false
			},
			wantErr: "readiness must prove no runtime, durable-computer, package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "readiness missing no production mutation flag",
			mutate: func(readiness *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				readiness.NoProductionMutation = false
			},
			wantErr: "readiness must prove no runtime, durable-computer, package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "readiness claims runtime materialized",
			mutate: func(readiness *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				readiness.RuntimeMaterialized = true
			},
			wantErr: "readiness carries materialization, mutation, downstream execution, production, full-substrate, or completion claims",
		},
		{
			name: "readiness claims durable computer mutation",
			mutate: func(readiness *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				readiness.DurableComputerStateMutated = true
			},
			wantErr: "readiness carries materialization, mutation, downstream execution, production, full-substrate, or completion claims",
		},
		{
			name: "readiness claims package published",
			mutate: func(readiness *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				readiness.PackagePublished = true
			},
			wantErr: "readiness carries materialization, mutation, downstream execution, production, full-substrate, or completion claims",
		},
		{
			name: "readiness claims promotion executed",
			mutate: func(readiness *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				readiness.PromotionExecuted = true
			},
			wantErr: "readiness carries materialization, mutation, downstream execution, production, full-substrate, or completion claims",
		},
		{
			name: "readiness claims run acceptance touched",
			mutate: func(readiness *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				readiness.RunAcceptanceRecordTouched = true
			},
			wantErr: "readiness carries materialization, mutation, downstream execution, production, full-substrate, or completion claims",
		},
		{
			name: "readiness claims production mutation",
			mutate: func(readiness *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				readiness.ProductionStateMutated = true
			},
			wantErr: "readiness carries materialization, mutation, downstream execution, production, full-substrate, or completion claims",
		},
		{
			name: "readiness claims full substrate",
			mutate: func(readiness *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				readiness.FullSubstrateClaimed = true
			},
			wantErr: "readiness carries materialization, mutation, downstream execution, production, full-substrate, or completion claims",
		},
		{
			name: "readiness claims completion",
			mutate: func(readiness *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				readiness.CompletionClaimed = true
			},
			wantErr: "readiness carries materialization, mutation, downstream execution, production, full-substrate, or completion claims",
		},
		{
			name: "durable contract kind drift",
			mutate: func(_ *BaseDurableStateSliceReadinessContract, durable *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				durable.Kind = BaseSubstrateEquivalenceContractKind
			},
			wantErr: "durable contract kind",
		},
		{
			name: "durable contract boundary drift",
			mutate: func(_ *BaseDurableStateSliceReadinessContract, durable *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				durable.Boundary = BaseSubstrateEquivalenceClaimScope
			},
			wantErr: "durable contract boundary",
		},
		{
			name: "durable contract scope drift",
			mutate: func(_ *BaseDurableStateSliceReadinessContract, durable *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				durable.Scope = BaseSubstrateEquivalenceClaimScope
			},
			wantErr: "durable contract scope",
		},
		{
			name: "durable contract version drift",
			mutate: func(_ *BaseDurableStateSliceReadinessContract, durable *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				durable.Version = foreignVersion
			},
			wantErr: "durable contract version does not match readiness",
		},
		{
			name: "durable typed artifact ref drift",
			mutate: func(_ *BaseDurableStateSliceReadinessContract, durable *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				durable.TypedArtifactProgramRef = "base-journal:owner/main@cursor-durable-drift"
			},
			wantErr: "durable typed artifact-program ref is invalid",
		},
		{
			name: "durable missing blob content class",
			mutate: func(_ *BaseDurableStateSliceReadinessContract, durable *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				durable.PersistentStateClasses = withoutBaseDurableStateClass(durable.PersistentStateClasses, BaseDurableStateClassBlobContent)
			},
			wantErr: "durable contract must cover file manifest and blob content classes",
		},
		{
			name: "durable missing file manifest class",
			mutate: func(_ *BaseDurableStateSliceReadinessContract, durable *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				durable.PersistentStateClasses = withoutBaseDurableStateClass(durable.PersistentStateClasses, BaseDurableStateClassFileManifest)
			},
			wantErr: "durable contract must cover file manifest and blob content classes",
		},
		{
			name: "durable missing blob set observation",
			mutate: func(_ *BaseDurableStateSliceReadinessContract, durable *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				durable.RequiredObservations = withoutObservationKind(durable.RequiredObservations, ObservationBlobSet)
			},
			wantErr: "durable contract must include file_manifest and blob_set observations",
		},
		{
			name: "durable missing file manifest observation",
			mutate: func(_ *BaseDurableStateSliceReadinessContract, durable *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				durable.RequiredObservations = withoutObservationKind(durable.RequiredObservations, ObservationFileManifest)
			},
			wantErr: "durable contract must include file_manifest and blob_set observations",
		},
		{
			name: "durable missing file path semantic",
			mutate: func(_ *BaseDurableStateSliceReadinessContract, durable *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				durable.RequiredSemantics = withoutUserSemantic(durable.RequiredSemantics, UserSemanticFilePath)
			},
			wantErr: "durable contract must cover file path, content, deletion, and provenance semantics",
		},
		{
			name: "durable missing file content semantic",
			mutate: func(_ *BaseDurableStateSliceReadinessContract, durable *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				durable.RequiredSemantics = withoutUserSemantic(durable.RequiredSemantics, UserSemanticFileContent)
			},
			wantErr: "durable contract must cover file path, content, deletion, and provenance semantics",
		},
		{
			name: "durable missing deletion semantic",
			mutate: func(_ *BaseDurableStateSliceReadinessContract, durable *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				durable.RequiredSemantics = withoutUserSemantic(durable.RequiredSemantics, UserSemanticDeletionState)
			},
			wantErr: "durable contract must cover file path, content, deletion, and provenance semantics",
		},
		{
			name: "durable missing provenance semantic",
			mutate: func(_ *BaseDurableStateSliceReadinessContract, durable *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				durable.RequiredSemantics = withoutUserSemantic(durable.RequiredSemantics, UserSemanticFileProvenance)
			},
			wantErr: "durable contract must cover file path, content, deletion, and provenance semantics",
		},
		{
			name: "durable missing equivalence proof ref",
			mutate: func(_ *BaseDurableStateSliceReadinessContract, durable *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				durable.EquivalenceContractRef = " "
			},
			wantErr: "durable contract must carry proof refs",
		},
		{
			name: "durable missing user isomorphism proof ref",
			mutate: func(_ *BaseDurableStateSliceReadinessContract, durable *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				durable.UserIsomorphismContractRef = " "
			},
			wantErr: "durable contract must carry proof refs",
		},
		{
			name: "durable missing durable slice evidence ref",
			mutate: func(_ *BaseDurableStateSliceReadinessContract, durable *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				durable.DurableSliceEvidenceRef = " "
			},
			wantErr: "durable contract must carry proof refs",
		},
		{
			name: "durable opaque data image dependency not excluded",
			mutate: func(_ *BaseDurableStateSliceReadinessContract, durable *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				durable.NoOpaqueDataImageDependency = false
			},
			wantErr: "durable contract must be no-opaque-data-img and no-mutation",
		},
		{
			name: "durable no mutation false",
			mutate: func(_ *BaseDurableStateSliceReadinessContract, durable *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				durable.NoMutation = false
			},
			wantErr: "durable contract must be no-opaque-data-img and no-mutation",
		},
		{
			name: "durable runtime behavior changed",
			mutate: func(_ *BaseDurableStateSliceReadinessContract, durable *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				durable.RuntimeBehaviorChanged = true
			},
			wantErr: "durable contract carries protected-surface or full-computer claims",
		},
		{
			name: "durable deployed route registered",
			mutate: func(_ *BaseDurableStateSliceReadinessContract, durable *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				durable.DeployedRouteRegistered = true
			},
			wantErr: "durable contract carries protected-surface or full-computer claims",
		},
		{
			name: "durable production auth touched",
			mutate: func(_ *BaseDurableStateSliceReadinessContract, durable *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				durable.ProductionAuthTouched = true
			},
			wantErr: "durable contract carries protected-surface or full-computer claims",
		},
		{
			name: "durable staging claimed",
			mutate: func(_ *BaseDurableStateSliceReadinessContract, durable *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				durable.StagingClaimed = true
			},
			wantErr: "durable contract carries protected-surface or full-computer claims",
		},
		{
			name: "durable promotion claimed",
			mutate: func(_ *BaseDurableStateSliceReadinessContract, durable *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				durable.PromotionClaimed = true
			},
			wantErr: "durable contract carries protected-surface or full-computer claims",
		},
		{
			name: "durable vm lifecycle touched",
			mutate: func(_ *BaseDurableStateSliceReadinessContract, durable *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				durable.VMLifecycleTouched = true
			},
			wantErr: "durable contract carries protected-surface or full-computer claims",
		},
		{
			name: "durable run acceptance touched",
			mutate: func(_ *BaseDurableStateSliceReadinessContract, durable *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				durable.RunAcceptanceRecordTouched = true
			},
			wantErr: "durable contract carries protected-surface or full-computer claims",
		},
		{
			name: "durable full computer claimed",
			mutate: func(_ *BaseDurableStateSliceReadinessContract, durable *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				durable.FullComputerClaimed = true
			},
			wantErr: "durable contract carries protected-surface or full-computer claims",
		},
		{
			name: "durable data image disposable claimed",
			mutate: func(_ *BaseDurableStateSliceReadinessContract, durable *BaseDurableStateSliceContract, _ *BaseDurableStateSliceProbeEvidence) {
				durable.DataImageDisposableClaimed = true
			},
			wantErr: "durable contract carries protected-surface or full-computer claims",
		},
		{
			name: "evidence missing durable readiness ref",
			mutate: func(_ *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, evidence *BaseDurableStateSliceProbeEvidence) {
				evidence.DurableStateSliceReadinessRef = " "
			},
			wantErr: "evidence refs are required",
		},
		{
			name: "evidence missing durable contract ref",
			mutate: func(_ *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, evidence *BaseDurableStateSliceProbeEvidence) {
				evidence.DurableStateSliceContractRef = " "
			},
			wantErr: "evidence refs are required",
		},
		{
			name: "evidence missing file manifest probe ref",
			mutate: func(_ *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, evidence *BaseDurableStateSliceProbeEvidence) {
				evidence.FileManifestProbeRef = " "
			},
			wantErr: "evidence refs are required",
		},
		{
			name: "evidence missing blob content probe ref",
			mutate: func(_ *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, evidence *BaseDurableStateSliceProbeEvidence) {
				evidence.BlobContentProbeRef = " "
			},
			wantErr: "evidence refs are required",
		},
		{
			name: "evidence missing probe evidence ref",
			mutate: func(_ *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, evidence *BaseDurableStateSliceProbeEvidence) {
				evidence.ProbeEvidenceRef = " "
			},
			wantErr: "evidence refs are required",
		},
		{
			name: "evidence missing residual risk ref",
			mutate: func(_ *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, evidence *BaseDurableStateSliceProbeEvidence) {
				evidence.ResidualRiskRef = " "
			},
			wantErr: "evidence refs are required",
		},
		{
			name: "evidence missing rollback plan ref",
			mutate: func(_ *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, evidence *BaseDurableStateSliceProbeEvidence) {
				evidence.RollbackPlanRef = " "
			},
			wantErr: "evidence refs are required",
		},
		{
			name: "evidence durable contract ref mismatches readiness",
			mutate: func(_ *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, evidence *BaseDurableStateSliceProbeEvidence) {
				evidence.DurableStateSliceContractRef = "durable-state-slice:foreign"
			},
			wantErr: "evidence refs do not match readiness",
		},
		{
			name: "evidence file manifest probe ref mismatches readiness",
			mutate: func(_ *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, evidence *BaseDurableStateSliceProbeEvidence) {
				evidence.FileManifestProbeRef = "file-manifest-probe:foreign"
			},
			wantErr: "evidence refs do not match readiness",
		},
		{
			name: "evidence blob content probe ref mismatches readiness",
			mutate: func(_ *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, evidence *BaseDurableStateSliceProbeEvidence) {
				evidence.BlobContentProbeRef = "blob-content-probe:foreign"
			},
			wantErr: "evidence refs do not match readiness",
		},
		{
			name: "evidence residual risk ref mismatches readiness",
			mutate: func(_ *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, evidence *BaseDurableStateSliceProbeEvidence) {
				evidence.ResidualRiskRef = "residual-risk:foreign"
			},
			wantErr: "evidence refs do not match readiness",
		},
		{
			name: "evidence rollback plan ref mismatches readiness",
			mutate: func(_ *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, evidence *BaseDurableStateSliceProbeEvidence) {
				evidence.RollbackPlanRef = "rollback-plan:foreign"
			},
			wantErr: "evidence refs do not match readiness",
		},
		{
			name: "evidence missing no runtime materialization flag",
			mutate: func(_ *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, evidence *BaseDurableStateSliceProbeEvidence) {
				evidence.NoRuntimeMaterialization = false
			},
			wantErr: "evidence must prove no runtime, opaque-data-img, durable-computer, package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "evidence missing no opaque data image dependency flag",
			mutate: func(_ *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, evidence *BaseDurableStateSliceProbeEvidence) {
				evidence.NoOpaqueDataImageDependency = false
			},
			wantErr: "evidence must prove no runtime, opaque-data-img, durable-computer, package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "evidence missing no durable computer mutation flag",
			mutate: func(_ *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, evidence *BaseDurableStateSliceProbeEvidence) {
				evidence.NoDurableComputerMutation = false
			},
			wantErr: "evidence must prove no runtime, opaque-data-img, durable-computer, package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "evidence missing no package publication mutation flag",
			mutate: func(_ *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, evidence *BaseDurableStateSliceProbeEvidence) {
				evidence.NoPackagePublicationMutation = false
			},
			wantErr: "evidence must prove no runtime, opaque-data-img, durable-computer, package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "evidence missing no promotion mutation flag",
			mutate: func(_ *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, evidence *BaseDurableStateSliceProbeEvidence) {
				evidence.NoPromotionMutation = false
			},
			wantErr: "evidence must prove no runtime, opaque-data-img, durable-computer, package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "evidence missing no run acceptance mutation flag",
			mutate: func(_ *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, evidence *BaseDurableStateSliceProbeEvidence) {
				evidence.NoRunAcceptanceMutation = false
			},
			wantErr: "evidence must prove no runtime, opaque-data-img, durable-computer, package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "evidence missing no production mutation flag",
			mutate: func(_ *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, evidence *BaseDurableStateSliceProbeEvidence) {
				evidence.NoProductionMutation = false
			},
			wantErr: "evidence must prove no runtime, opaque-data-img, durable-computer, package publication, promotion, run-acceptance, or production mutation",
		},
		{
			name: "evidence claims runtime materialization",
			mutate: func(_ *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, evidence *BaseDurableStateSliceProbeEvidence) {
				evidence.RuntimeMaterialized = true
			},
			wantErr: "evidence carries materialization, mutation, downstream execution, production, full-substrate, or completion claims",
		},
		{
			name: "evidence claims durable computer mutation",
			mutate: func(_ *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, evidence *BaseDurableStateSliceProbeEvidence) {
				evidence.DurableComputerStateMutated = true
			},
			wantErr: "evidence carries materialization, mutation, downstream execution, production, full-substrate, or completion claims",
		},
		{
			name: "evidence claims package publication",
			mutate: func(_ *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, evidence *BaseDurableStateSliceProbeEvidence) {
				evidence.PackagePublished = true
			},
			wantErr: "evidence carries materialization, mutation, downstream execution, production, full-substrate, or completion claims",
		},
		{
			name: "evidence claims promotion execution",
			mutate: func(_ *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, evidence *BaseDurableStateSliceProbeEvidence) {
				evidence.PromotionExecuted = true
			},
			wantErr: "evidence carries materialization, mutation, downstream execution, production, full-substrate, or completion claims",
		},
		{
			name: "evidence claims run acceptance touch",
			mutate: func(_ *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, evidence *BaseDurableStateSliceProbeEvidence) {
				evidence.RunAcceptanceRecordTouched = true
			},
			wantErr: "evidence carries materialization, mutation, downstream execution, production, full-substrate, or completion claims",
		},
		{
			name: "evidence claims production mutation",
			mutate: func(_ *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, evidence *BaseDurableStateSliceProbeEvidence) {
				evidence.ProductionStateMutated = true
			},
			wantErr: "evidence carries materialization, mutation, downstream execution, production, full-substrate, or completion claims",
		},
		{
			name: "evidence claims full substrate",
			mutate: func(_ *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, evidence *BaseDurableStateSliceProbeEvidence) {
				evidence.FullSubstrateClaimed = true
			},
			wantErr: "evidence carries materialization, mutation, downstream execution, production, full-substrate, or completion claims",
		},
		{
			name: "evidence claims completion",
			mutate: func(_ *BaseDurableStateSliceReadinessContract, _ *BaseDurableStateSliceContract, evidence *BaseDurableStateSliceProbeEvidence) {
				evidence.CompletionClaimed = true
			},
			wantErr: "evidence carries materialization, mutation, downstream execution, production, full-substrate, or completion claims",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			readiness, durable, evidence := baseDurableStateSliceProbeContractInputs(t)
			tc.mutate(&readiness, &durable, &evidence)

			contract, err := BuildBaseDurableStateSliceProbeContract(readiness, durable, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildBaseDurableStateSliceProbeContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func baseDurableStateSliceProbeContractInputs(t *testing.T) (BaseDurableStateSliceReadinessContract, BaseDurableStateSliceContract, BaseDurableStateSliceProbeEvidence) {
	t.Helper()

	handoff, readinessEvidence := baseDurableStateSliceReadinessContractInputs(t, BasePromotionResultOutcomeBlocked, BasePromotionSettlementDecisionBlocked)
	readiness, err := BuildBaseDurableStateSliceReadinessContract(handoff, readinessEvidence)
	if err != nil {
		t.Fatalf("BuildBaseDurableStateSliceReadinessContract(): %v", err)
	}

	equivalence, user, durableEvidence := baseDurableStateSliceContractInputs(t)
	durable, err := BuildBaseDurableStateSliceContract(equivalence, user, durableEvidence)
	if err != nil {
		t.Fatalf("BuildBaseDurableStateSliceContract(): %v", err)
	}

	readiness.Version = durable.Version
	readiness.TypedArtifactProgramRef = durable.TypedArtifactProgramRef

	return readiness, durable, BaseDurableStateSliceProbeEvidence{
		DurableStateSliceReadinessRef: " durable-state-slice-readiness:base-pass-118 ",
		DurableStateSliceContractRef:  " " + readiness.DurableStateSlicePlanRef + " ",
		FileManifestProbeRef:          " " + readiness.FileManifestProbeRef + " ",
		BlobContentProbeRef:           " " + readiness.BlobContentProbeRef + " ",
		ProbeEvidenceRef:              " durable-state-slice-probe:base-pass-119 ",
		ResidualRiskRef:               " " + readiness.ResidualRiskRef + " ",
		RollbackPlanRef:               " " + readiness.RollbackPlanRef + " ",
		NoRuntimeMaterialization:      true,
		NoOpaqueDataImageDependency:   true,
		NoDurableComputerMutation:     true,
		NoPackagePublicationMutation:  true,
		NoPromotionMutation:           true,
		NoRunAcceptanceMutation:       true,
		NoProductionMutation:          true,
		RuntimeMaterialized:           false,
		DurableComputerStateMutated:   false,
		PackagePublished:              false,
		PromotionExecuted:             false,
		RunAcceptanceRecordTouched:    false,
		ProductionStateMutated:        false,
		FullSubstrateClaimed:          false,
		CompletionClaimed:             false,
	}
}

func withoutBaseDurableStateClass(classes []BaseDurableStateClass, drop BaseDurableStateClass) []BaseDurableStateClass {
	out := make([]BaseDurableStateClass, 0, len(classes))
	for _, class := range classes {
		if class != drop {
			out = append(out, class)
		}
	}
	return out
}

func withoutObservationKind(observations []ObservationKind, drop ObservationKind) []ObservationKind {
	out := make([]ObservationKind, 0, len(observations))
	for _, observation := range observations {
		if observation != drop {
			out = append(out, observation)
		}
	}
	return out
}
