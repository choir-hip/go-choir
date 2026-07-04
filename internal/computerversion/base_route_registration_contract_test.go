package computerversion

import (
	"reflect"
	"strings"
	"testing"
)

func TestBuildBaseRouteRegistrationReadinessContractBuildsBlockedNoMutationContract(t *testing.T) {
	harness, evidence := baseRouteRegistrationReadinessInputs()

	contract, err := BuildBaseRouteRegistrationReadinessContract(harness, evidence)
	if err != nil {
		t.Fatalf("build base route registration readiness contract: %v", err)
	}

	if contract.Kind != BaseRouteRegistrationReadinessContractKind {
		t.Fatalf("kind = %q, want %q", contract.Kind, BaseRouteRegistrationReadinessContractKind)
	}
	if contract.Version != harness.Version {
		t.Fatalf("version = %#v, want harness version %#v", contract.Version, harness.Version)
	}
	if contract.HarnessEvidenceRef != harness.HarnessEvidenceRef {
		t.Fatalf("harness evidence ref = %q, want %q", contract.HarnessEvidenceRef, harness.HarnessEvidenceRef)
	}
	if contract.ObservationEvidenceRef != harness.ObservationEvidenceRef {
		t.Fatalf("observation evidence ref = %q, want %q", contract.ObservationEvidenceRef, harness.ObservationEvidenceRef)
	}
	if contract.ComparisonEvidenceRef != harness.ComparisonEvidenceRef {
		t.Fatalf("comparison evidence ref = %q, want %q", contract.ComparisonEvidenceRef, harness.ComparisonEvidenceRef)
	}
	if contract.RoutePrefix != "/api/base/" {
		t.Fatalf("route prefix = %q, want /api/base/", contract.RoutePrefix)
	}
	if !reflect.DeepEqual(contract.RequiredScopes, []string{"read:base", "write:base"}) {
		t.Fatalf("required scopes = %#v, want read/write base scopes", contract.RequiredScopes)
	}
	if contract.ReadinessBoundary != BaseRouteRegistrationReadinessBoundary {
		t.Fatalf("readiness boundary = %q, want %q", contract.ReadinessBoundary, BaseRouteRegistrationReadinessBoundary)
	}
	if contract.ReadinessStatus != BaseRouteRegistrationReadinessStatusBlocked {
		t.Fatalf("readiness status = %q, want %q", contract.ReadinessStatus, BaseRouteRegistrationReadinessStatusBlocked)
	}
	if !contract.NoMutation {
		t.Fatalf("no mutation = false, want true")
	}
	if contract.RouteRegistrationAllowed {
		t.Fatalf("route registration allowed = true, want false")
	}
	assertBaseRouteRegistrationReadinessUnsafeFlagsFalse(t, contract)
	if !reflect.DeepEqual(contract.RequiredPrerequisiteRefs, canonicalStrings(baseRouteRegistrationBlockedPrerequisites())) {
		t.Fatalf("required prerequisite refs = %#v, want canonical blocked prerequisites %#v", contract.RequiredPrerequisiteRefs, canonicalStrings(baseRouteRegistrationBlockedPrerequisites()))
	}
	if !reflect.DeepEqual(contract.BlockedPrerequisites, baseRouteRegistrationBlockedPrerequisites()) {
		t.Fatalf("blocked prerequisites = %#v, want %#v", contract.BlockedPrerequisites, baseRouteRegistrationBlockedPrerequisites())
	}
	if contract.RollbackPlanRef != evidence.RollbackPlanRef {
		t.Fatalf("rollback plan ref = %q, want %q", contract.RollbackPlanRef, evidence.RollbackPlanRef)
	}
}

func TestBuildBaseRouteRegistrationReadinessContractRejectsUnsafeOrIncompleteEvidence(t *testing.T) {
	foreignVersion := ComputerVersion{CodeRef: "git:foreign-base-route-registration-readiness", ArtifactProgramRef: "tape:org/foreign-base-route-registration-readiness@2026-07-04"}
	for _, tc := range []struct {
		name    string
		mutate  func(*LocalBaseProductPathHarnessEvidence, *BaseRouteRegistrationReadinessEvidence)
		wantErr string
	}{
		{
			name: "missing explicit state paths",
			mutate: func(harness *LocalBaseProductPathHarnessEvidence, _ *BaseRouteRegistrationReadinessEvidence) {
				harness.ExplicitStatePaths = false
			},
			wantErr: "explicit state paths",
		},
		{
			name: "missing auth-backed local route",
			mutate: func(harness *LocalBaseProductPathHarnessEvidence, _ *BaseRouteRegistrationReadinessEvidence) {
				harness.AuthBacked = false
			},
			wantErr: "auth-backed local routes",
		},
		{
			name: "missing local route registration",
			mutate: func(harness *LocalBaseProductPathHarnessEvidence, _ *BaseRouteRegistrationReadinessEvidence) {
				harness.LocalRouteRegistered = false
			},
			wantErr: "local route registration",
		},
		{
			name: "missing persisted observation",
			mutate: func(harness *LocalBaseProductPathHarnessEvidence, _ *BaseRouteRegistrationReadinessEvidence) {
				harness.PersistedObservationSet = false
			},
			wantErr: "persisted observation set",
		},
		{
			name: "missing seeded mismatch failure",
			mutate: func(harness *LocalBaseProductPathHarnessEvidence, _ *BaseRouteRegistrationReadinessEvidence) {
				harness.SeededMismatchFailed = false
			},
			wantErr: "seeded mismatch failure",
		},
		{
			name: "mismatched version",
			mutate: func(_ *LocalBaseProductPathHarnessEvidence, evidence *BaseRouteRegistrationReadinessEvidence) {
				evidence.Version = foreignVersion
			},
			wantErr: "evidence version does not match harness",
		},
		{
			name: "mismatched harness ref",
			mutate: func(_ *LocalBaseProductPathHarnessEvidence, evidence *BaseRouteRegistrationReadinessEvidence) {
				evidence.HarnessEvidenceRef = "evidence:foreign-harness"
			},
			wantErr: "evidence harness ref does not match harness",
		},
		{
			name: "mismatched observation ref",
			mutate: func(_ *LocalBaseProductPathHarnessEvidence, evidence *BaseRouteRegistrationReadinessEvidence) {
				evidence.ObservationEvidenceRef = "evidence:foreign-observation"
			},
			wantErr: "evidence observation ref does not match harness",
		},
		{
			name: "mismatched comparison ref",
			mutate: func(_ *LocalBaseProductPathHarnessEvidence, evidence *BaseRouteRegistrationReadinessEvidence) {
				evidence.ComparisonEvidenceRef = "evidence:foreign-comparison"
			},
			wantErr: "evidence comparison ref does not match harness",
		},
		{
			name: "missing route prefix",
			mutate: func(_ *LocalBaseProductPathHarnessEvidence, evidence *BaseRouteRegistrationReadinessEvidence) {
				evidence.RoutePrefix = "  "
			},
			wantErr: "route prefix",
		},
		{
			name: "missing scopes",
			mutate: func(_ *LocalBaseProductPathHarnessEvidence, evidence *BaseRouteRegistrationReadinessEvidence) {
				evidence.RequiredScopes = nil
			},
			wantErr: "required scopes must be read:base and write:base",
		},
		{
			name: "missing prerequisite refs",
			mutate: func(_ *LocalBaseProductPathHarnessEvidence, evidence *BaseRouteRegistrationReadinessEvidence) {
				evidence.RequiredPrerequisiteRefs = nil
			},
			wantErr: "prerequisite refs are required",
		},
		{
			name: "missing one prerequisite ref",
			mutate: func(_ *LocalBaseProductPathHarnessEvidence, evidence *BaseRouteRegistrationReadinessEvidence) {
				evidence.RequiredPrerequisiteRefs = []string{
					BaseRouteRegistrationPrerequisiteAuthSessionScope,
					BaseRouteRegistrationPrerequisiteDeployedServiceRegistration,
					BaseRouteRegistrationPrerequisiteStagingBuildIdentity,
					BaseRouteRegistrationPrerequisiteRollbackRouteRevert,
				}
			},
			wantErr: BaseRouteRegistrationPrerequisiteProductionStateBoundary,
		},
		{
			name: "missing rollback ref",
			mutate: func(_ *LocalBaseProductPathHarnessEvidence, evidence *BaseRouteRegistrationReadinessEvidence) {
				evidence.RollbackPlanRef = "\t"
			},
			wantErr: "rollback plan ref is required",
		},
		{
			name: "route registration allowed",
			mutate: func(_ *LocalBaseProductPathHarnessEvidence, evidence *BaseRouteRegistrationReadinessEvidence) {
				evidence.RouteRegistrationAllowed = true
			},
			wantErr: "cannot allow route registration",
		},
		{
			name: "mutation boundary crossed",
			mutate: func(_ *LocalBaseProductPathHarnessEvidence, evidence *BaseRouteRegistrationReadinessEvidence) {
				evidence.NoMutation = false
			},
			wantErr: "evidence must be no-mutation",
		},
		{
			name: "harness deployed route registered",
			mutate: func(harness *LocalBaseProductPathHarnessEvidence, _ *BaseRouteRegistrationReadinessEvidence) {
				harness.DeployedRouteRegistered = true
			},
			wantErr: "harness cannot register deployed routes",
		},
		{
			name: "harness production auth touched",
			mutate: func(harness *LocalBaseProductPathHarnessEvidence, _ *BaseRouteRegistrationReadinessEvidence) {
				harness.ProductionAuthTouched = true
			},
			wantErr: "harness cannot touch production auth/session",
		},
		{
			name: "harness staging claimed",
			mutate: func(harness *LocalBaseProductPathHarnessEvidence, _ *BaseRouteRegistrationReadinessEvidence) {
				harness.StagingClaimed = true
			},
			wantErr: "harness cannot claim staging",
		},
		{
			name: "harness production state mutated",
			mutate: func(harness *LocalBaseProductPathHarnessEvidence, _ *BaseRouteRegistrationReadinessEvidence) {
				harness.ProductionStateMutated = true
			},
			wantErr: "harness cannot mutate production state",
		},
		{
			name: "harness vm lifecycle touched",
			mutate: func(harness *LocalBaseProductPathHarnessEvidence, _ *BaseRouteRegistrationReadinessEvidence) {
				harness.VMLifecycleTouched = true
			},
			wantErr: "harness cannot touch VM lifecycle",
		},
		{
			name: "harness promotion rollback touched",
			mutate: func(harness *LocalBaseProductPathHarnessEvidence, _ *BaseRouteRegistrationReadinessEvidence) {
				harness.PromotionRollbackTouched = true
			},
			wantErr: "harness cannot touch promotion/rollback",
		},
		{
			name: "harness run acceptance record touched",
			mutate: func(harness *LocalBaseProductPathHarnessEvidence, _ *BaseRouteRegistrationReadinessEvidence) {
				harness.RunAcceptanceRecordTouched = true
			},
			wantErr: "harness cannot touch run acceptance records",
		},
		{
			name: "evidence deployed route registered",
			mutate: func(_ *LocalBaseProductPathHarnessEvidence, evidence *BaseRouteRegistrationReadinessEvidence) {
				evidence.DeployedRouteRegistered = true
			},
			wantErr: "evidence cannot register deployed routes",
		},
		{
			name: "evidence production auth touched",
			mutate: func(_ *LocalBaseProductPathHarnessEvidence, evidence *BaseRouteRegistrationReadinessEvidence) {
				evidence.ProductionAuthTouched = true
			},
			wantErr: "evidence cannot touch production auth/session",
		},
		{
			name: "evidence staging claimed",
			mutate: func(_ *LocalBaseProductPathHarnessEvidence, evidence *BaseRouteRegistrationReadinessEvidence) {
				evidence.StagingClaimed = true
			},
			wantErr: "evidence cannot claim staging",
		},
		{
			name: "evidence production state mutated",
			mutate: func(_ *LocalBaseProductPathHarnessEvidence, evidence *BaseRouteRegistrationReadinessEvidence) {
				evidence.ProductionStateMutated = true
			},
			wantErr: "evidence cannot mutate production state",
		},
		{
			name: "evidence vm lifecycle touched",
			mutate: func(_ *LocalBaseProductPathHarnessEvidence, evidence *BaseRouteRegistrationReadinessEvidence) {
				evidence.VMLifecycleTouched = true
			},
			wantErr: "evidence cannot touch VM lifecycle",
		},
		{
			name: "evidence promotion rollback touched",
			mutate: func(_ *LocalBaseProductPathHarnessEvidence, evidence *BaseRouteRegistrationReadinessEvidence) {
				evidence.PromotionRollbackTouched = true
			},
			wantErr: "evidence cannot touch promotion/rollback",
		},
		{
			name: "evidence run acceptance record touched",
			mutate: func(_ *LocalBaseProductPathHarnessEvidence, evidence *BaseRouteRegistrationReadinessEvidence) {
				evidence.RunAcceptanceRecordTouched = true
			},
			wantErr: "evidence cannot touch run acceptance records",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			harness, evidence := baseRouteRegistrationReadinessInputs()
			tc.mutate(&harness, &evidence)

			contract, err := BuildBaseRouteRegistrationReadinessContract(harness, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildBaseRouteRegistrationReadinessContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func TestBuildBaseRouteRegistrationAuthorityReviewContractRecordsReadOnlyBoundary(t *testing.T) {
	readiness, evidence := baseRouteRegistrationAuthorityReviewInputs(t)

	contract, err := BuildBaseRouteRegistrationAuthorityReviewContract(readiness, evidence)
	if err != nil {
		t.Fatalf("build base route registration authority review contract: %v", err)
	}

	if contract.Kind != BaseRouteRegistrationAuthorityReviewContractKind {
		t.Fatalf("kind = %q, want %q", contract.Kind, BaseRouteRegistrationAuthorityReviewContractKind)
	}
	if contract.Version != readiness.Version {
		t.Fatalf("version = %#v, want readiness version %#v", contract.Version, readiness.Version)
	}
	if contract.HarnessEvidenceRef != readiness.HarnessEvidenceRef {
		t.Fatalf("harness evidence ref = %q, want %q", contract.HarnessEvidenceRef, readiness.HarnessEvidenceRef)
	}
	if contract.ObservationEvidenceRef != readiness.ObservationEvidenceRef {
		t.Fatalf("observation evidence ref = %q, want %q", contract.ObservationEvidenceRef, readiness.ObservationEvidenceRef)
	}
	if contract.ComparisonEvidenceRef != readiness.ComparisonEvidenceRef {
		t.Fatalf("comparison evidence ref = %q, want %q", contract.ComparisonEvidenceRef, readiness.ComparisonEvidenceRef)
	}
	if contract.RoutePrefix != "/api/base/" {
		t.Fatalf("route prefix = %q, want /api/base/", contract.RoutePrefix)
	}
	if contract.ReadinessContractRef != evidence.ReadinessContractRef {
		t.Fatalf("readiness contract ref = %q, want %q", contract.ReadinessContractRef, evidence.ReadinessContractRef)
	}
	if contract.AuthorityReviewBoundary != BaseRouteRegistrationAuthorityReviewBoundary {
		t.Fatalf("authority review boundary = %q, want %q", contract.AuthorityReviewBoundary, BaseRouteRegistrationAuthorityReviewBoundary)
	}
	if contract.AuthorityReviewStatus != BaseRouteRegistrationAuthorityReviewStatusRecorded {
		t.Fatalf("authority review status = %q, want %q", contract.AuthorityReviewStatus, BaseRouteRegistrationAuthorityReviewStatusRecorded)
	}
	if contract.OwnerAuthorizationRef != evidence.OwnerAuthorizationRef {
		t.Fatalf("owner authorization ref = %q, want %q", contract.OwnerAuthorizationRef, evidence.OwnerAuthorizationRef)
	}
	if contract.ReviewerAuthorizationRef != evidence.ReviewerAuthorizationRef {
		t.Fatalf("reviewer authorization ref = %q, want %q", contract.ReviewerAuthorizationRef, evidence.ReviewerAuthorizationRef)
	}
	if contract.RedCeremonyPlanRef != evidence.RedCeremonyPlanRef {
		t.Fatalf("red ceremony plan ref = %q, want %q", contract.RedCeremonyPlanRef, evidence.RedCeremonyPlanRef)
	}
	if !reflect.DeepEqual(contract.RequiredPrerequisiteRefs, canonicalStrings(readiness.RequiredPrerequisiteRefs)) {
		t.Fatalf("required prerequisite refs = %#v, want canonical readiness prerequisites %#v", contract.RequiredPrerequisiteRefs, canonicalStrings(readiness.RequiredPrerequisiteRefs))
	}
	if !reflect.DeepEqual(contract.ChecklistItemRefs, canonicalStrings(evidence.ChecklistItemRefs)) {
		t.Fatalf("checklist item refs = %#v, want canonical review items %#v", contract.ChecklistItemRefs, canonicalStrings(evidence.ChecklistItemRefs))
	}
	if !reflect.DeepEqual(contract.ReviewerFindingRefs, canonicalStrings(evidence.ReviewerFindingRefs)) {
		t.Fatalf("reviewer finding refs = %#v, want canonical findings %#v", contract.ReviewerFindingRefs, canonicalStrings(evidence.ReviewerFindingRefs))
	}
	if !reflect.DeepEqual(contract.OpenQuestionRefs, canonicalStrings(evidence.OpenQuestionRefs)) {
		t.Fatalf("open question refs = %#v, want canonical open questions %#v", contract.OpenQuestionRefs, canonicalStrings(evidence.OpenQuestionRefs))
	}
	if contract.ReviewReportRef != evidence.ReviewReportRef {
		t.Fatalf("review report ref = %q, want %q", contract.ReviewReportRef, evidence.ReviewReportRef)
	}
	if contract.RollbackPlanRef != readiness.RollbackPlanRef {
		t.Fatalf("rollback plan ref = %q, want %q", contract.RollbackPlanRef, readiness.RollbackPlanRef)
	}
	if !contract.NoMutation {
		t.Fatalf("no mutation = false, want true")
	}
	if contract.RouteRegistrationAuthorized {
		t.Fatalf("route registration authorized = true, want false")
	}
	if contract.RedCeremonyOpened {
		t.Fatalf("red ceremony opened = true, want false")
	}
	if contract.RedCeremonyApproved {
		t.Fatalf("red ceremony approved = true, want false")
	}
	assertBaseRouteRegistrationAuthorityReviewUnsafeFlagsFalse(t, contract)
	if !reflect.DeepEqual(contract.BlockedPrerequisites, baseRouteRegistrationBlockedPrerequisites()) {
		t.Fatalf("blocked prerequisites = %#v, want %#v", contract.BlockedPrerequisites, baseRouteRegistrationBlockedPrerequisites())
	}
}

func TestBuildBaseRouteRegistrationAuthorityReviewContractRejectsUnsafeOrIncompleteInputs(t *testing.T) {
	foreignVersion := ComputerVersion{CodeRef: "git:foreign-base-route-registration-authority-review", ArtifactProgramRef: "tape:org/foreign-base-route-registration-authority-review@2026-07-04"}
	for _, tc := range []struct {
		name    string
		mutate  func(*BaseRouteRegistrationReadinessContract, *BaseRouteRegistrationAuthorityReviewEvidence)
		wantErr string
	}{
		{
			name: "invalid readiness kind",
			mutate: func(readiness *BaseRouteRegistrationReadinessContract, _ *BaseRouteRegistrationAuthorityReviewEvidence) {
				readiness.Kind = "foreign_readiness_contract"
			},
			wantErr: "readiness kind",
		},
		{
			name: "invalid readiness boundary",
			mutate: func(readiness *BaseRouteRegistrationReadinessContract, _ *BaseRouteRegistrationAuthorityReviewEvidence) {
				readiness.ReadinessBoundary = "deployed_route_registration_boundary"
			},
			wantErr: "readiness boundary",
		},
		{
			name: "invalid readiness status",
			mutate: func(readiness *BaseRouteRegistrationReadinessContract, _ *BaseRouteRegistrationAuthorityReviewEvidence) {
				readiness.ReadinessStatus = "ready_for_red_ceremony"
			},
			wantErr: "readiness status",
		},
		{
			name: "invalid readiness scopes",
			mutate: func(readiness *BaseRouteRegistrationReadinessContract, _ *BaseRouteRegistrationAuthorityReviewEvidence) {
				readiness.RequiredScopes = []string{"read:base"}
			},
			wantErr: "readiness scopes",
		},
		{
			name: "invalid readiness prerequisites",
			mutate: func(readiness *BaseRouteRegistrationReadinessContract, _ *BaseRouteRegistrationAuthorityReviewEvidence) {
				readiness.RequiredPrerequisiteRefs = []string{
					BaseRouteRegistrationPrerequisiteAuthSessionScope,
					BaseRouteRegistrationPrerequisiteDeployedServiceRegistration,
					BaseRouteRegistrationPrerequisiteStagingBuildIdentity,
					BaseRouteRegistrationPrerequisiteRollbackRouteRevert,
				}
			},
			wantErr: "readiness prerequisite refs",
		},
		{
			name: "readiness mutation boundary crossed",
			mutate: func(readiness *BaseRouteRegistrationReadinessContract, _ *BaseRouteRegistrationAuthorityReviewEvidence) {
				readiness.NoMutation = false
			},
			wantErr: "readiness must be no-mutation",
		},
		{
			name: "readiness route registration allowed",
			mutate: func(readiness *BaseRouteRegistrationReadinessContract, _ *BaseRouteRegistrationAuthorityReviewEvidence) {
				readiness.RouteRegistrationAllowed = true
			},
			wantErr: "readiness cannot allow route registration",
		},
		{
			name: "mismatched evidence version",
			mutate: func(_ *BaseRouteRegistrationReadinessContract, evidence *BaseRouteRegistrationAuthorityReviewEvidence) {
				evidence.Version = foreignVersion
			},
			wantErr: "evidence version does not match readiness",
		},
		{
			name: "mismatched evidence harness ref",
			mutate: func(_ *BaseRouteRegistrationReadinessContract, evidence *BaseRouteRegistrationAuthorityReviewEvidence) {
				evidence.HarnessEvidenceRef = "evidence:foreign-base-route-registration-harness"
			},
			wantErr: "evidence harness ref does not match readiness",
		},
		{
			name: "mismatched evidence observation ref",
			mutate: func(_ *BaseRouteRegistrationReadinessContract, evidence *BaseRouteRegistrationAuthorityReviewEvidence) {
				evidence.ObservationEvidenceRef = "evidence:foreign-base-route-registration-observation"
			},
			wantErr: "evidence observation ref does not match readiness",
		},
		{
			name: "mismatched evidence comparison ref",
			mutate: func(_ *BaseRouteRegistrationReadinessContract, evidence *BaseRouteRegistrationAuthorityReviewEvidence) {
				evidence.ComparisonEvidenceRef = "evidence:foreign-base-route-registration-comparison"
			},
			wantErr: "evidence comparison ref does not match readiness",
		},
		{
			name: "mismatched evidence route prefix",
			mutate: func(_ *BaseRouteRegistrationReadinessContract, evidence *BaseRouteRegistrationAuthorityReviewEvidence) {
				evidence.RoutePrefix = "/api/foreign/"
			},
			wantErr: "evidence route prefix does not match readiness",
		},
		{
			name: "mismatched evidence rollback ref",
			mutate: func(_ *BaseRouteRegistrationReadinessContract, evidence *BaseRouteRegistrationAuthorityReviewEvidence) {
				evidence.RollbackPlanRef = "rollback:foreign-base-route-registration"
			},
			wantErr: "evidence rollback plan ref does not match readiness",
		},
		{
			name: "mismatched evidence prerequisites",
			mutate: func(_ *BaseRouteRegistrationReadinessContract, evidence *BaseRouteRegistrationAuthorityReviewEvidence) {
				evidence.RequiredPrerequisiteRefs = []string{
					BaseRouteRegistrationPrerequisiteAuthSessionScope,
					BaseRouteRegistrationPrerequisiteDeployedServiceRegistration,
					BaseRouteRegistrationPrerequisiteStagingBuildIdentity,
					BaseRouteRegistrationPrerequisiteRollbackRouteRevert,
				}
			},
			wantErr: "evidence prerequisite refs do not match readiness",
		},
		{
			name: "missing readiness contract ref",
			mutate: func(_ *BaseRouteRegistrationReadinessContract, evidence *BaseRouteRegistrationAuthorityReviewEvidence) {
				evidence.ReadinessContractRef = " "
			},
			wantErr: "readiness contract ref is required",
		},
		{
			name: "missing owner authorization ref",
			mutate: func(_ *BaseRouteRegistrationReadinessContract, evidence *BaseRouteRegistrationAuthorityReviewEvidence) {
				evidence.OwnerAuthorizationRef = ""
			},
			wantErr: "owner authorization ref is required",
		},
		{
			name: "missing reviewer authorization ref",
			mutate: func(_ *BaseRouteRegistrationReadinessContract, evidence *BaseRouteRegistrationAuthorityReviewEvidence) {
				evidence.ReviewerAuthorizationRef = "\t"
			},
			wantErr: "reviewer authorization ref is required",
		},
		{
			name: "missing red ceremony plan ref",
			mutate: func(_ *BaseRouteRegistrationReadinessContract, evidence *BaseRouteRegistrationAuthorityReviewEvidence) {
				evidence.RedCeremonyPlanRef = ""
			},
			wantErr: "red ceremony plan ref is required",
		},
		{
			name: "missing checklist item refs",
			mutate: func(_ *BaseRouteRegistrationReadinessContract, evidence *BaseRouteRegistrationAuthorityReviewEvidence) {
				evidence.ChecklistItemRefs = nil
			},
			wantErr: "checklist item refs",
		},
		{
			name: "missing reviewer finding refs",
			mutate: func(_ *BaseRouteRegistrationReadinessContract, evidence *BaseRouteRegistrationAuthorityReviewEvidence) {
				evidence.ReviewerFindingRefs = nil
			},
			wantErr: "reviewer finding refs are required",
		},
		{
			name: "missing review report ref",
			mutate: func(_ *BaseRouteRegistrationReadinessContract, evidence *BaseRouteRegistrationAuthorityReviewEvidence) {
				evidence.ReviewReportRef = " "
			},
			wantErr: "review report ref is required",
		},
		{
			name: "route registration authorized",
			mutate: func(_ *BaseRouteRegistrationReadinessContract, evidence *BaseRouteRegistrationAuthorityReviewEvidence) {
				evidence.RouteRegistrationAuthorized = true
			},
			wantErr: "evidence cannot authorize route registration",
		},
		{
			name: "red ceremony opened",
			mutate: func(_ *BaseRouteRegistrationReadinessContract, evidence *BaseRouteRegistrationAuthorityReviewEvidence) {
				evidence.RedCeremonyOpened = true
			},
			wantErr: "evidence cannot open red ceremony",
		},
		{
			name: "red ceremony approved",
			mutate: func(_ *BaseRouteRegistrationReadinessContract, evidence *BaseRouteRegistrationAuthorityReviewEvidence) {
				evidence.RedCeremonyApproved = true
			},
			wantErr: "evidence cannot approve red ceremony",
		},
		{
			name: "authority review mutation boundary crossed",
			mutate: func(_ *BaseRouteRegistrationReadinessContract, evidence *BaseRouteRegistrationAuthorityReviewEvidence) {
				evidence.NoMutation = false
			},
			wantErr: "evidence must be no-mutation",
		},
		{
			name: "evidence deployed route registered",
			mutate: func(_ *BaseRouteRegistrationReadinessContract, evidence *BaseRouteRegistrationAuthorityReviewEvidence) {
				evidence.DeployedRouteRegistered = true
			},
			wantErr: "authority review evidence cannot register deployed routes",
		},
		{
			name: "evidence production auth touched",
			mutate: func(_ *BaseRouteRegistrationReadinessContract, evidence *BaseRouteRegistrationAuthorityReviewEvidence) {
				evidence.ProductionAuthTouched = true
			},
			wantErr: "authority review evidence cannot touch production auth/session",
		},
		{
			name: "evidence staging claimed",
			mutate: func(_ *BaseRouteRegistrationReadinessContract, evidence *BaseRouteRegistrationAuthorityReviewEvidence) {
				evidence.StagingClaimed = true
			},
			wantErr: "authority review evidence cannot claim staging",
		},
		{
			name: "evidence production state mutated",
			mutate: func(_ *BaseRouteRegistrationReadinessContract, evidence *BaseRouteRegistrationAuthorityReviewEvidence) {
				evidence.ProductionStateMutated = true
			},
			wantErr: "authority review evidence cannot mutate production state",
		},
		{
			name: "evidence vm lifecycle touched",
			mutate: func(_ *BaseRouteRegistrationReadinessContract, evidence *BaseRouteRegistrationAuthorityReviewEvidence) {
				evidence.VMLifecycleTouched = true
			},
			wantErr: "authority review evidence cannot touch VM lifecycle",
		},
		{
			name: "evidence promotion rollback touched",
			mutate: func(_ *BaseRouteRegistrationReadinessContract, evidence *BaseRouteRegistrationAuthorityReviewEvidence) {
				evidence.PromotionRollbackTouched = true
			},
			wantErr: "authority review evidence cannot touch promotion/rollback",
		},
		{
			name: "evidence run acceptance record touched",
			mutate: func(_ *BaseRouteRegistrationReadinessContract, evidence *BaseRouteRegistrationAuthorityReviewEvidence) {
				evidence.RunAcceptanceRecordTouched = true
			},
			wantErr: "authority review evidence cannot touch run acceptance records",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			readiness, evidence := baseRouteRegistrationAuthorityReviewInputs(t)
			tc.mutate(&readiness, &evidence)

			contract, err := BuildBaseRouteRegistrationAuthorityReviewContract(readiness, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildBaseRouteRegistrationAuthorityReviewContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func baseRouteRegistrationAuthorityReviewInputs(t *testing.T) (BaseRouteRegistrationReadinessContract, BaseRouteRegistrationAuthorityReviewEvidence) {
	t.Helper()
	harness, readinessEvidence := baseRouteRegistrationReadinessInputs()
	readiness, err := BuildBaseRouteRegistrationReadinessContract(harness, readinessEvidence)
	if err != nil {
		t.Fatalf("build base route registration readiness contract fixture: %v", err)
	}

	evidence := BaseRouteRegistrationAuthorityReviewEvidence{
		Version:                  readiness.Version,
		HarnessEvidenceRef:       readiness.HarnessEvidenceRef,
		ObservationEvidenceRef:   readiness.ObservationEvidenceRef,
		ComparisonEvidenceRef:    readiness.ComparisonEvidenceRef,
		RoutePrefix:              readiness.RoutePrefix,
		ReadinessContractRef:     "contract:base-route-registration-readiness",
		OwnerAuthorizationRef:    "auth:base-route-owner",
		ReviewerAuthorizationRef: "auth:base-route-reviewer",
		RedCeremonyPlanRef:       "red-ceremony:base-route-registration-plan",
		RequiredPrerequisiteRefs: append([]string(nil), readiness.RequiredPrerequisiteRefs...),
		ChecklistItemRefs: []string{
			BaseRouteRegistrationAuthorityReviewItemProductionStateBoundary,
			BaseRouteRegistrationAuthorityReviewItemRollbackRehearsal,
			BaseRouteRegistrationAuthorityReviewItemStagingIdentity,
			BaseRouteRegistrationAuthorityReviewItemServiceRouting,
			BaseRouteRegistrationAuthorityReviewItemAuthSessionScope,
			BaseRouteRegistrationAuthorityReviewItemRedCeremonyScope,
		},
		ReviewerFindingRefs: []string{
			"finding:base-route-registration-owner-boundary",
			"finding:base-route-registration-red-ceremony-blocked",
		},
		OpenQuestionRefs: []string{
			"question:base-route-registration-production-window",
			"question:base-route-registration-auth-scope-owner",
		},
		ReviewReportRef: "report:base-route-registration-authority-review",
		RollbackPlanRef: "rollback:base-route-registration-readiness",
		NoMutation:      true,
	}
	return readiness, evidence
}

func assertBaseRouteRegistrationAuthorityReviewUnsafeFlagsFalse(t *testing.T, contract BaseRouteRegistrationAuthorityReviewContract) {
	t.Helper()
	if contract.DeployedRouteRegistered {
		t.Fatalf("deployed route registered = true, want false")
	}
	if contract.ProductionAuthTouched {
		t.Fatalf("production auth touched = true, want false")
	}
	if contract.StagingClaimed {
		t.Fatalf("staging claimed = true, want false")
	}
	if contract.ProductionStateMutated {
		t.Fatalf("production state mutated = true, want false")
	}
	if contract.VMLifecycleTouched {
		t.Fatalf("vm lifecycle touched = true, want false")
	}
	if contract.PromotionRollbackTouched {
		t.Fatalf("promotion rollback touched = true, want false")
	}
	if contract.RunAcceptanceRecordTouched {
		t.Fatalf("run acceptance record touched = true, want false")
	}
}

func baseRouteRegistrationReadinessInputs() (LocalBaseProductPathHarnessEvidence, BaseRouteRegistrationReadinessEvidence) {
	version := baseRouteRegistrationReadinessVersion()
	harness := LocalBaseProductPathHarnessEvidence{
		Version:                 version,
		HarnessEvidenceRef:      "evidence:base-route-registration-harness",
		ObservationEvidenceRef:  "evidence:base-route-registration-observation",
		ComparisonEvidenceRef:   "evidence:base-route-registration-comparison",
		ExplicitStatePaths:      true,
		AuthBacked:              true,
		LocalRouteRegistered:    true,
		PersistedObservationSet: true,
		SeededMismatchFailed:    true,
	}
	evidence := BaseRouteRegistrationReadinessEvidence{
		Version:                  version,
		HarnessEvidenceRef:       harness.HarnessEvidenceRef,
		ObservationEvidenceRef:   harness.ObservationEvidenceRef,
		ComparisonEvidenceRef:    harness.ComparisonEvidenceRef,
		RoutePrefix:              "/api/base/",
		RequiredScopes:           []string{"write:base", "read:base"},
		RequiredPrerequisiteRefs: baseRouteRegistrationBlockedPrerequisites(),
		RollbackPlanRef:          "rollback:base-route-registration-readiness",
		NoMutation:               true,
	}
	return harness, evidence
}

func baseRouteRegistrationReadinessVersion() ComputerVersion {
	return ComputerVersion{CodeRef: "git:base-route-registration-readiness", ArtifactProgramRef: "tape:org/base-route-registration-readiness@2026-07-04"}
}

func assertBaseRouteRegistrationReadinessUnsafeFlagsFalse(t *testing.T, contract BaseRouteRegistrationReadinessContract) {
	t.Helper()
	if contract.DeployedRouteRegistered {
		t.Fatalf("deployed route registered = true, want false")
	}
	if contract.ProductionAuthTouched {
		t.Fatalf("production auth touched = true, want false")
	}
	if contract.StagingClaimed {
		t.Fatalf("staging claimed = true, want false")
	}
	if contract.ProductionStateMutated {
		t.Fatalf("production state mutated = true, want false")
	}
	if contract.VMLifecycleTouched {
		t.Fatalf("vm lifecycle touched = true, want false")
	}
	if contract.PromotionRollbackTouched {
		t.Fatalf("promotion rollback touched = true, want false")
	}
	if contract.RunAcceptanceRecordTouched {
		t.Fatalf("run acceptance record touched = true, want false")
	}
}
