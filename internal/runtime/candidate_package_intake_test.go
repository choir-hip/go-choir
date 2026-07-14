package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	choirserver "github.com/yusefmosiah/go-choir/internal/server"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestCandidatePackageIntakeOptInRoutesCreateListAndGetOwnerScopedRecord(t *testing.T) {
	t.Parallel()
	rt, handler := testAPISetup(t)
	srv := candidatePackageIntakeTestServer(handler)

	aliceBody := candidatePackageIntakeRequestBody(t, map[string]any{
		"owner_id":                          "user-alice",
		"candidate_package_id":              "pkg-intake-alice",
		"candidate_package_manifest_sha256": "sha256:alice-manifest",
		"source_computer_id":                "computer-alice-source",
		"source_candidate_id":               "candidate-alice-source",
		"candidate_source_ref":              "refs/computers/computer-alice-source/candidates/candidate-alice-source",
		"intake_boundary":                   "owner review blocks product adoption until a later explicit adoption path acts",
		"status":                            string(types.CandidatePackageIntakeOwnerReviewPending),
		"owner_review_state":                string(types.CandidatePackageOwnerReviewRequired),
		"owner_review_required":             true,
		"adoption_ready":                    false,
		"trace_id":                          "trace-alice-intake",
	})
	createdW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes", aliceBody, "user-alice")
	if createdW.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201; body=%s", createdW.Code, createdW.Body.String())
	}
	var created types.CandidatePackageIntakeRecord
	if err := json.Unmarshal(createdW.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created intake: %v", err)
	}
	assertCandidatePackageIntakeRecordFromAPI(t, created, candidatePackageIntakeRecordContract{
		OwnerID:                        "user-alice",
		CandidatePackageID:             "pkg-intake-alice",
		CandidatePackageManifestSHA256: "sha256:alice-manifest",
		SourceComputerID:               "computer-alice-source",
		SourceCandidateID:              "candidate-alice-source",
		CandidateSourceRef:             "refs/computers/computer-alice-source/candidates/candidate-alice-source",
		IntakeBoundary:                 "owner review blocks product adoption until a later explicit adoption path acts",
		Status:                         types.CandidatePackageIntakeOwnerReviewPending,
		OwnerReviewState:               types.CandidatePackageOwnerReviewRequired,
		OwnerReviewRequired:            true,
		AdoptionReady:                  false,
		TraceID:                        "trace-alice-intake",
	})

	bobBody := candidatePackageIntakeRequestBody(t, map[string]any{
		"owner_id":                          "user-bob",
		"candidate_package_id":              "pkg-intake-bob",
		"candidate_package_manifest_sha256": "sha256:bob-manifest",
		"source_computer_id":                "computer-bob-source",
		"source_candidate_id":               "candidate-bob-source",
		"candidate_source_ref":              "refs/computers/computer-bob-source/candidates/candidate-bob-source",
		"intake_boundary":                   "bob owner review boundary",
		"status":                            string(types.CandidatePackageIntakeOwnerReviewPending),
		"owner_review_state":                string(types.CandidatePackageOwnerReviewRequired),
		"owner_review_required":             true,
		"adoption_ready":                    false,
	})
	bobW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes", bobBody, "user-bob")
	if bobW.Code != http.StatusCreated {
		t.Fatalf("create bob status = %d, want 201; body=%s", bobW.Code, bobW.Body.String())
	}

	listW := serveCandidatePackageIntakeRequest(srv, http.MethodGet, "/api/candidate-package-intakes", "", "user-alice")
	if listW.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200; body=%s", listW.Code, listW.Body.String())
	}
	var listed candidatePackageIntakeListResponse
	if err := json.Unmarshal(listW.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listed.Intakes) != 1 {
		t.Fatalf("alice list returned %d intakes, want exactly her created intake: %+v", len(listed.Intakes), listed.Intakes)
	}
	if listed.Intakes[0].IntakeID != created.IntakeID || listed.Intakes[0].OwnerID != "user-alice" {
		t.Fatalf("alice list crossed owner boundary or lost created intake: got %+v want intake_id=%q owner=user-alice", listed.Intakes[0], created.IntakeID)
	}

	detailW := serveCandidatePackageIntakeRequest(srv, http.MethodGet, "/api/candidate-package-intakes/"+created.IntakeID, "", "user-alice")
	if detailW.Code != http.StatusOK {
		t.Fatalf("detail status = %d, want 200; body=%s", detailW.Code, detailW.Body.String())
	}
	var detail types.CandidatePackageIntakeRecord
	if err := json.Unmarshal(detailW.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode detail response: %v", err)
	}
	if detail.IntakeID != created.IntakeID {
		t.Fatalf("detail intake_id = %q, want created intake_id %q", detail.IntakeID, created.IntakeID)
	}
	assertCandidatePackageIntakeRecordFromAPI(t, detail, candidatePackageIntakeRecordContract{
		OwnerID:                        "user-alice",
		CandidatePackageID:             "pkg-intake-alice",
		CandidatePackageManifestSHA256: "sha256:alice-manifest",
		SourceComputerID:               "computer-alice-source",
		SourceCandidateID:              "candidate-alice-source",
		CandidateSourceRef:             "refs/computers/computer-alice-source/candidates/candidate-alice-source",
		IntakeBoundary:                 "owner review blocks product adoption until a later explicit adoption path acts",
		Status:                         types.CandidatePackageIntakeOwnerReviewPending,
		OwnerReviewState:               types.CandidatePackageOwnerReviewRequired,
		OwnerReviewRequired:            true,
		AdoptionReady:                  false,
		TraceID:                        "trace-alice-intake",
	})

	otherOwnerDetailW := serveCandidatePackageIntakeRequest(srv, http.MethodGet, "/api/candidate-package-intakes/"+created.IntakeID, "", "user-bob")
	if otherOwnerDetailW.Code != http.StatusNotFound {
		t.Fatalf("other owner detail status = %d, want 404; body=%s", otherOwnerDetailW.Code, otherOwnerDetailW.Body.String())
	}

	assertNoCandidatePackageIntakePromotionSideEffects(t, rt, "user-alice")
	assertNoCandidatePackageIntakePromotionSideEffects(t, rt, "user-bob")
}

func TestCandidatePackageIntakeReviewRouteApprovesPendingOwnerRecord(t *testing.T) {
	t.Parallel()
	rt, handler := testAPISetup(t)
	srv := candidatePackageIntakeTestServer(handler)

	pending := createCandidatePackageIntakeForReview(t, srv, "user-alice", "intake-review-approve", map[string]any{
		"adoption_blockers_json": []string{
			"owner_review_not_recorded",
			"verification_pending",
		},
		"evidence_refs_json": []string{
			"texture://evidence/intake-review-approve/source",
		},
	})

	reviewW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+pending.IntakeID+"/review", candidatePackageIntakeReviewRequestBody(t, "approve", "texture://review/intake-review-approve/owner"), "user-alice")
	if reviewW.Code != http.StatusOK {
		t.Fatalf("review approve status = %d, want 200; body=%s", reviewW.Code, reviewW.Body.String())
	}
	var reviewed types.CandidatePackageIntakeRecord
	if err := json.Unmarshal(reviewW.Body.Bytes(), &reviewed); err != nil {
		t.Fatalf("decode approve review response: %v", err)
	}
	assertCandidatePackageIntakeReviewTransition(t, reviewed, pending, types.CandidatePackageIntakeOwnerApproved, types.CandidatePackageOwnerReviewApproved)
	assertCandidatePackageIntakeJSONStringArrayContains(t, "approved intake evidence refs", reviewed.EvidenceRefsJSON, "texture://evidence/intake-review-approve/source")
	assertCandidatePackageIntakeJSONStringArrayContains(t, "approved intake evidence refs", reviewed.EvidenceRefsJSON, "texture://review/intake-review-approve/owner")
	assertCandidatePackageIntakeJSONStringArrayContains(t, "approved intake blockers", reviewed.AdoptionBlockersJSON, "verification_pending")
	assertCandidatePackageIntakeJSONStringArrayNotContains(t, "approved intake blockers", reviewed.AdoptionBlockersJSON, "owner_review_not_recorded")

	assertNoCandidatePackageIntakePromotionSideEffects(t, rt, "user-alice")
}

func TestCandidatePackageIntakeAdoptionBoundaryRouteBindsApprovedIntakeWithoutPromotionSideEffects(t *testing.T) {
	t.Parallel()
	rt, handler := testAPISetup(t)
	srv := candidatePackageIntakeTestServer(handler)

	pending := createCandidatePackageIntakeForReview(t, srv, "user-alice", "intake-adoption-boundary-success", map[string]any{
		"adoption_blockers_json": []string{"owner_review_not_recorded"},
		"evidence_refs_json":     []string{"texture://evidence/intake-adoption-boundary-success/source"},
		"acceptance_json": map[string]any{
			"source_intake_contract": "preserved",
		},
	})
	approveW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+pending.IntakeID+"/review", candidatePackageIntakeReviewRequestBody(t, "approve", "texture://review/intake-adoption-boundary-success/owner"), "user-alice")
	if approveW.Code != http.StatusOK {
		t.Fatalf("approve before adoption boundary status = %d, want 200; body=%s", approveW.Code, approveW.Body.String())
	}
	var approved types.CandidatePackageIntakeRecord
	if err := json.Unmarshal(approveW.Body.Bytes(), &approved); err != nil {
		t.Fatalf("decode approved intake: %v", err)
	}
	if approved.AdoptionReady {
		t.Fatalf("approved intake adoption_ready = true before rollback boundary binding: %+v", approved)
	}
	assertCandidatePackageIntakeJSONStringArrayContains(t, "approved adoption blockers", approved.AdoptionBlockersJSON, "adoption_rollback_boundary_not_bound")

	boundaryW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+pending.IntakeID+"/adoption-boundary", candidatePackageIntakeAdoptionBoundaryRequestBody(t, "texture://contracts/intake-adoption-boundary-success/adoption", "texture://contracts/intake-adoption-boundary-success/rollback", "texture://evidence/intake-adoption-boundary-success/boundary"), "user-alice")
	if boundaryW.Code != http.StatusOK {
		t.Fatalf("adoption boundary status = %d, want 200; body=%s", boundaryW.Code, boundaryW.Body.String())
	}
	var bound types.CandidatePackageIntakeRecord
	if err := json.Unmarshal(boundaryW.Body.Bytes(), &bound); err != nil {
		t.Fatalf("decode adoption boundary response: %v", err)
	}
	assertCandidatePackageIntakeAdoptionBoundaryPreservedRecord(t, bound, approved)
	if !bound.AdoptionReady {
		t.Fatalf("adoption boundary did not mark blocker-free approved intake adoption-ready: %+v", bound)
	}
	if blockers := candidatePackageIntakeJSONStringArray(t, "bound adoption blockers", bound.AdoptionBlockersJSON); len(blockers) != 0 {
		t.Fatalf("bound adoption blockers = %v, want no blockers after rollback boundary was bound", blockers)
	}
	assertCandidatePackageIntakeJSONStringArrayNotContains(t, "bound adoption blockers", bound.AdoptionBlockersJSON, "adoption_rollback_boundary_not_bound")
	assertCandidatePackageIntakeJSONStringArrayContains(t, "bound evidence refs", bound.EvidenceRefsJSON, "texture://evidence/intake-adoption-boundary-success/source")
	assertCandidatePackageIntakeJSONStringArrayContains(t, "bound evidence refs", bound.EvidenceRefsJSON, "texture://review/intake-adoption-boundary-success/owner")
	assertCandidatePackageIntakeJSONStringArrayContains(t, "bound evidence refs", bound.EvidenceRefsJSON, "texture://evidence/intake-adoption-boundary-success/boundary")
	assertCandidatePackageIntakeAdoptionBoundaryAcceptance(t, bound.AcceptanceJSON, "texture://contracts/intake-adoption-boundary-success/adoption", "texture://contracts/intake-adoption-boundary-success/rollback")
	assertCandidatePackageIntakeAcceptanceStringField(t, bound.AcceptanceJSON, "source_intake_contract", "preserved")

	detail := candidatePackageIntakeDetail(t, srv, pending.IntakeID, "user-alice")
	assertCandidatePackageIntakeAdoptionBoundaryPreservedRecord(t, detail, bound)
	if !detail.AdoptionReady {
		t.Fatalf("persisted adoption boundary detail adoption_ready = false: %+v", detail)
	}
	assertCandidatePackageIntakeAdoptionBoundaryAcceptance(t, detail.AcceptanceJSON, "texture://contracts/intake-adoption-boundary-success/adoption", "texture://contracts/intake-adoption-boundary-success/rollback")
	assertCandidatePackageIntakeAcceptanceStringField(t, detail.AcceptanceJSON, "source_intake_contract", "preserved")

	assertNoCandidatePackageIntakePromotionSideEffects(t, rt, "user-alice")
}

func TestCandidatePackageIntakePublicationDraftRouteCreatesPrivateDraftPackageWithoutAdoptionSideEffects(t *testing.T) {
	t.Parallel()
	rt, handler := testAPISetup(t)
	srv := candidatePackageIntakeTestServer(handler)
	intakeID := "intake-publication-draft-success"
	appID := "candidate-intake-review-app"
	publicationContractRef := "texture://contracts/" + intakeID + "/publication"
	draftEvidenceRef := "texture://evidence/" + intakeID + "/draft"

	ready := createCandidatePackageIntakeReadyForPublication(t, srv, "user-alice", intakeID, map[string]any{
		"evidence_refs_json": []string{"texture://evidence/" + intakeID + "/source"},
		"verifier_contracts_json": []map[string]any{{
			"name":         "source-candidate-package-intake",
			"state":        "recorded",
			"contract_ref": "texture://contracts/" + intakeID + "/source",
		}},
	})

	draftW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+ready.IntakeID+"/publication-draft", candidatePackageIntakePublicationDraftRequestBody(t, map[string]any{
		"app_id":                   appID,
		"publication_contract_ref": publicationContractRef,
		"draft_evidence_ref":       draftEvidenceRef,
	}), "user-alice")
	if draftW.Code != http.StatusCreated {
		t.Fatalf("publication draft status = %d, want 201; body=%s", draftW.Code, draftW.Body.String())
	}
	var draft types.AppChangePackageRecord
	if err := json.Unmarshal(draftW.Body.Bytes(), &draft); err != nil {
		t.Fatalf("decode publication draft response: %v", err)
	}
	assertCandidatePackageIntakePublicationDraftPackage(t, draft, ready, appID, publicationContractRef, draftEvidenceRef)

	packages, err := rt.store.ListAppChangePackages(context.Background(), "user-alice", 10)
	if err != nil {
		t.Fatalf("list app change packages after publication draft: %v", err)
	}
	if len(packages) != 1 {
		t.Fatalf("publication draft created %d app change packages, want exactly one draft package: %+v", len(packages), packages)
	}
	assertCandidatePackageIntakePublicationDraftPackage(t, packages[0], ready, appID, publicationContractRef, draftEvidenceRef)

	adoptions, err := rt.store.ListAppAdoptions(context.Background(), "user-alice", 10)
	if err != nil {
		t.Fatalf("list app adoptions after publication draft: %v", err)
	}
	if len(adoptions) != 0 {
		t.Fatalf("publication draft created app adoptions: %+v", adoptions)
	}
}

func TestCandidatePackageIntakeAdoptionReviewRouteCreatesOwnerReviewWithoutPromotionSideEffects(t *testing.T) {
	t.Parallel()
	rt, handler := testAPISetup(t)
	srv := candidatePackageIntakeTestServer(handler)
	ctx := context.Background()
	intakeID := "intake-adoption-review-success"
	appID := "candidate-intake-adoption-review-app"
	targetComputerID := "computer-adoption-review-target"
	targetActiveRef := "refs/computers/" + targetComputerID + "/active-before-review"
	adoptionReviewContractRef := "texture://contracts/" + intakeID + "/adoption-review"
	publicationContractRef := "texture://contracts/" + intakeID + "/publication"
	draftEvidenceRef := "texture://evidence/" + intakeID + "/draft"

	if _, err := rt.promotion.EnsureComputerSourceLineage(ctx, "user-alice", targetComputerID, "workspace", targetActiveRef); err != nil {
		t.Fatalf("seed target source lineage: %v", err)
	}
	ready := createCandidatePackageIntakeReadyForPublication(t, srv, "user-alice", intakeID, map[string]any{
		"evidence_refs_json": []string{"texture://evidence/" + intakeID + "/source"},
	})
	draft := createCandidatePackageIntakePublicationDraftThroughRoute(t, srv, ready, "user-alice", appID, publicationContractRef, draftEvidenceRef)
	assertCandidatePackageIntakePublicationDraftPackage(t, draft, ready, appID, publicationContractRef, draftEvidenceRef)

	createW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+ready.IntakeID+"/adoption-review", candidatePackageIntakeAdoptionReviewRequestBody(t, map[string]any{
		"target_computer_id":           targetComputerID,
		"adoption_review_contract_ref": adoptionReviewContractRef,
	}), "user-alice")
	if createW.Code != http.StatusCreated {
		t.Fatalf("adoption review create status = %d, want 201; body=%s", createW.Code, createW.Body.String())
	}
	var pending types.AppAdoptionRecord
	if err := json.Unmarshal(createW.Body.Bytes(), &pending); err != nil {
		t.Fatalf("decode adoption review create response: %v", err)
	}
	assertCandidatePackageIntakeAdoptionReviewRecord(t, pending, ready, draft, targetComputerID, "owner_review_pending", targetActiveRef, adoptionReviewContractRef)
	assertCandidatePackageIntakeAdoptionReviewContracts(t, pending, ready, adoptionReviewContractRef)
	assertCandidatePackageIntakeSingleDraftPackage(t, rt, ready, appID, publicationContractRef, draftEvidenceRef)
	assertCandidatePackageIntakeTargetLineageActiveRef(t, rt, "user-alice", targetComputerID, targetActiveRef)
	adoptions := assertCandidatePackageIntakeAdoptionCount(t, rt, "user-alice", 1)
	if adoptions[0].AdoptionID != pending.AdoptionID {
		t.Fatalf("persisted adoption id = %q, want response adoption %q", adoptions[0].AdoptionID, pending.AdoptionID)
	}

	approveW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+ready.IntakeID+"/adoption-review/"+pending.AdoptionID, candidatePackageIntakeAdoptionReviewDecisionRequestBody(t, "approve", "texture://review/"+intakeID+"/adoption-approve"), "user-alice")
	if approveW.Code != http.StatusOK {
		t.Fatalf("adoption review approve status = %d, want 200; body=%s", approveW.Code, approveW.Body.String())
	}
	var approved types.AppAdoptionRecord
	if err := json.Unmarshal(approveW.Body.Bytes(), &approved); err != nil {
		t.Fatalf("decode adoption review approve response: %v", err)
	}
	assertCandidatePackageIntakeAdoptionReviewRecord(t, approved, ready, draft, targetComputerID, "owner_review_approved", targetActiveRef, adoptionReviewContractRef)
	assertCandidatePackageIntakeAdoptionReviewContracts(t, approved, ready, adoptionReviewContractRef)
	assertCandidatePackageIntakeSingleDraftPackage(t, rt, ready, appID, publicationContractRef, draftEvidenceRef)
	assertCandidatePackageIntakeTargetLineageActiveRef(t, rt, "user-alice", targetComputerID, targetActiveRef)
	adoptions = assertCandidatePackageIntakeAdoptionCount(t, rt, "user-alice", 1)
	if adoptions[0].AdoptionID != pending.AdoptionID || string(adoptions[0].Status) != "owner_review_approved" {
		t.Fatalf("approve created or selected wrong adoption: %+v; want one approved adoption %q", adoptions, pending.AdoptionID)
	}
}

func TestCandidatePackageIntakePromotionSwitchRouteSwitchesSourceLineageWithoutPublishingPackage(t *testing.T) {
	t.Parallel()
	rt, handler := testAPISetup(t)
	srv := candidatePackageIntakeTestServer(handler)
	intakeID := "intake-promotion-switch-success"

	fixture := createCandidatePackageIntakePromotionSwitchReviewThroughRoute(t, rt, srv, "user-alice", intakeID)
	approved := approveCandidatePackageIntakePromotionSwitchReviewThroughRoute(t, srv, fixture)

	switchW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+fixture.Intake.IntakeID+"/adoption-review/"+approved.AdoptionID+"/promotion-switch", candidatePackageIntakePromotionSwitchRequestBody(t, "texture://evidence/"+intakeID+"/switch"), "user-alice")
	if switchW.Code != http.StatusOK {
		t.Fatalf("promotion switch status = %d, want 200; body=%s", switchW.Code, switchW.Body.String())
	}
	var switched types.AppAdoptionRecord
	if err := json.Unmarshal(switchW.Body.Bytes(), &switched); err != nil {
		t.Fatalf("decode promotion switch response: %v", err)
	}
	if switched.AdoptionID != approved.AdoptionID || switched.Status != types.AppAdoptionSourceLineageSwitched {
		t.Fatalf("promotion switch adoption = id:%q status:%q, want id:%q status:%q", switched.AdoptionID, switched.Status, approved.AdoptionID, types.AppAdoptionSourceLineageSwitched)
	}
	if switched.CandidateSourceRef != approved.CandidateSourceRef || switched.TargetActiveSourceRefAtCutover != fixture.TargetActiveRef {
		t.Fatalf("promotion switch source refs = candidate:%q cutover:%q, want candidate:%q cutover:%q", switched.CandidateSourceRef, switched.TargetActiveSourceRefAtCutover, approved.CandidateSourceRef, fixture.TargetActiveRef)
	}
	if switched.RuntimeArtifactDigest != "" || switched.UIArtifactDigest != "" || switched.ForegroundTailMergeResult != "" || switched.MergeStrategy != "" {
		t.Fatalf("promotion switch created deployed promotion/build fields: %+v", switched)
	}
	candidatePackageIntakeJSONContainsStringField(t, "promotion switch verifier_results_json", switched.VerifierResultsJSON, "status", "source_lineage_switched")
	candidatePackageIntakeJSONContainsStringField(t, "promotion switch verifier_results_json", switched.VerifierResultsJSON, "promotion_mode", "source_lineage_only")
	candidatePackageIntakeJSONContainsStringField(t, "promotion switch rollback_profile_json", switched.RollbackProfileJSON, "source_lineage_switch_status", "source_lineage_switched")
	candidatePackageIntakeJSONContainsStringField(t, "promotion switch rollback_profile_json", switched.RollbackProfileJSON, "source_lineage_switch_ref", approved.CandidateSourceRef)

	assertCandidatePackageIntakeTargetLineageSwitch(t, rt, "user-alice", fixture.TargetComputerID, approved.CandidateSourceRef, approved.AdoptionID, fixture.Draft.PackageID, approved.CandidateSourceRef)
	assertCandidatePackageIntakeSingleDraftPackage(t, rt, fixture.Intake, fixture.AppID, fixture.PublicationContractRef, fixture.DraftEvidenceRef)
	adoptions := assertCandidatePackageIntakeAdoptionCount(t, rt, "user-alice", 1)
	if adoptions[0].AdoptionID != approved.AdoptionID || adoptions[0].Status != types.AppAdoptionSourceLineageSwitched || adoptions[0].TargetActiveSourceRefAtCutover != fixture.TargetActiveRef {
		t.Fatalf("persisted switched adoption = %+v, want one source-lineage-switched adoption %q with cutover %q", adoptions[0], approved.AdoptionID, fixture.TargetActiveRef)
	}
}

func TestCandidatePackageIntakePromotionSwitchRouteRollsBackAndRollsForwardSourceLineage(t *testing.T) {
	t.Parallel()
	rt, handler := testAPISetup(t)
	srv := candidatePackageIntakeTestServer(handler)
	intakeID := "intake-promotion-switch-rollback-roll-forward-success"

	fixture := createCandidatePackageIntakePromotionSwitchReviewThroughRoute(t, rt, srv, "user-alice", intakeID)
	approved := approveCandidatePackageIntakePromotionSwitchReviewThroughRoute(t, srv, fixture)
	switched := switchCandidatePackageIntakePromotionSwitchThroughRoute(t, srv, fixture, approved, "texture://evidence/"+intakeID+"/switch")
	assertCandidatePackageIntakeTargetLineageSwitch(t, rt, "user-alice", fixture.TargetComputerID, approved.CandidateSourceRef, approved.AdoptionID, fixture.Draft.PackageID, approved.CandidateSourceRef)
	assertCandidatePackageIntakeSingleDraftPackage(t, rt, fixture.Intake, fixture.AppID, fixture.PublicationContractRef, fixture.DraftEvidenceRef)
	assertCandidatePackageIntakeAdoptionHasNoDeployedPromotionFields(t, switched)

	rollbackW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+fixture.Intake.IntakeID+"/adoption-review/"+approved.AdoptionID+"/promotion-switch/rollback", candidatePackageIntakePromotionSwitchRollbackRequestBody(t, "texture://evidence/"+intakeID+"/rollback"), "user-alice")
	if rollbackW.Code != http.StatusOK {
		t.Fatalf("promotion switch rollback status = %d, want 200; body=%s", rollbackW.Code, rollbackW.Body.String())
	}
	var rolledBack types.AppAdoptionRecord
	if err := json.Unmarshal(rollbackW.Body.Bytes(), &rolledBack); err != nil {
		t.Fatalf("decode promotion switch rollback response: %v", err)
	}
	if rolledBack.AdoptionID != approved.AdoptionID || rolledBack.Status != types.AppAdoptionRolledBack {
		t.Fatalf("promotion switch rollback adoption = id:%q status:%q, want id:%q status:%q", rolledBack.AdoptionID, rolledBack.Status, approved.AdoptionID, types.AppAdoptionRolledBack)
	}
	if rolledBack.CandidateSourceRef != approved.CandidateSourceRef || rolledBack.TargetActiveSourceRefAtCutover != fixture.TargetActiveRef {
		t.Fatalf("promotion switch rollback refs = candidate:%q cutover:%q, want candidate:%q cutover:%q", rolledBack.CandidateSourceRef, rolledBack.TargetActiveSourceRefAtCutover, approved.CandidateSourceRef, fixture.TargetActiveRef)
	}
	assertCandidatePackageIntakeAdoptionHasNoDeployedPromotionFields(t, rolledBack)
	candidatePackageIntakeJSONContainsStringField(t, "promotion switch rollback verifier_results_json", rolledBack.VerifierResultsJSON, "status", "source_lineage_rolled_back")
	candidatePackageIntakeJSONContainsStringField(t, "promotion switch rollback verifier_results_json", rolledBack.VerifierResultsJSON, "rollback_mode", "source_lineage_only")
	candidatePackageIntakeJSONContainsStringField(t, "promotion switch rollback verifier_results_json", rolledBack.VerifierResultsJSON, "rollback_evidence_ref", "texture://evidence/"+intakeID+"/rollback")
	candidatePackageIntakeJSONContainsStringField(t, "promotion switch rollback rollback_profile_json", rolledBack.RollbackProfileJSON, "source_lineage_switch_status", "source_lineage_rolled_back")
	candidatePackageIntakeJSONContainsStringField(t, "promotion switch rollback rollback_profile_json", rolledBack.RollbackProfileJSON, "source_lineage_restored_ref", fixture.TargetActiveRef)
	candidatePackageIntakeJSONContainsStringField(t, "promotion switch rollback rollback_profile_json", rolledBack.RollbackProfileJSON, "rollback_evidence_ref", "texture://evidence/"+intakeID+"/rollback")
	assertCandidatePackageIntakeTargetLineageNonDeployedActiveRef(t, rt, "user-alice", fixture.TargetComputerID, fixture.TargetActiveRef)
	assertCandidatePackageIntakeSingleDraftPackage(t, rt, fixture.Intake, fixture.AppID, fixture.PublicationContractRef, fixture.DraftEvidenceRef)
	assertCandidatePackageIntakeAdoptionStatus(t, rt, "user-alice", approved.AdoptionID, types.AppAdoptionRolledBack, fixture.TargetActiveRef)

	rollForwardW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+fixture.Intake.IntakeID+"/adoption-review/"+approved.AdoptionID+"/promotion-switch/roll-forward", candidatePackageIntakePromotionSwitchRollForwardRequestBody(t, "texture://evidence/"+intakeID+"/roll-forward"), "user-alice")
	if rollForwardW.Code != http.StatusOK {
		t.Fatalf("promotion switch roll-forward status = %d, want 200; body=%s", rollForwardW.Code, rollForwardW.Body.String())
	}
	var rolledForward types.AppAdoptionRecord
	if err := json.Unmarshal(rollForwardW.Body.Bytes(), &rolledForward); err != nil {
		t.Fatalf("decode promotion switch roll-forward response: %v", err)
	}
	if rolledForward.AdoptionID != approved.AdoptionID || rolledForward.Status != types.AppAdoptionSourceLineageSwitched {
		t.Fatalf("promotion switch roll-forward adoption = id:%q status:%q, want id:%q status:%q", rolledForward.AdoptionID, rolledForward.Status, approved.AdoptionID, types.AppAdoptionSourceLineageSwitched)
	}
	if rolledForward.CandidateSourceRef != approved.CandidateSourceRef || rolledForward.TargetActiveSourceRefAtCutover != fixture.TargetActiveRef {
		t.Fatalf("promotion switch roll-forward refs = candidate:%q cutover:%q, want candidate:%q cutover:%q", rolledForward.CandidateSourceRef, rolledForward.TargetActiveSourceRefAtCutover, approved.CandidateSourceRef, fixture.TargetActiveRef)
	}
	assertCandidatePackageIntakeAdoptionHasNoDeployedPromotionFields(t, rolledForward)
	candidatePackageIntakeJSONContainsStringField(t, "promotion switch roll-forward verifier_results_json", rolledForward.VerifierResultsJSON, "status", "source_lineage_switched")
	candidatePackageIntakeJSONContainsStringField(t, "promotion switch roll-forward verifier_results_json", rolledForward.VerifierResultsJSON, "promotion_mode", "source_lineage_only_roll_forward")
	candidatePackageIntakeJSONContainsStringField(t, "promotion switch roll-forward verifier_results_json", rolledForward.VerifierResultsJSON, "roll_forward_evidence_ref", "texture://evidence/"+intakeID+"/roll-forward")
	candidatePackageIntakeJSONContainsStringField(t, "promotion switch roll-forward rollback_profile_json", rolledForward.RollbackProfileJSON, "source_lineage_switch_status", "source_lineage_switched")
	candidatePackageIntakeJSONContainsStringField(t, "promotion switch roll-forward rollback_profile_json", rolledForward.RollbackProfileJSON, "source_lineage_switch_ref", approved.CandidateSourceRef)
	candidatePackageIntakeJSONContainsStringField(t, "promotion switch roll-forward rollback_profile_json", rolledForward.RollbackProfileJSON, "promotion_mode", "source_lineage_only_roll_forward")
	candidatePackageIntakeJSONContainsStringField(t, "promotion switch roll-forward rollback_profile_json", rolledForward.RollbackProfileJSON, "roll_forward_evidence_ref", "texture://evidence/"+intakeID+"/roll-forward")
	assertCandidatePackageIntakeTargetLineageSwitch(t, rt, "user-alice", fixture.TargetComputerID, approved.CandidateSourceRef, approved.AdoptionID, fixture.Draft.PackageID, approved.CandidateSourceRef)
	assertCandidatePackageIntakeSingleDraftPackage(t, rt, fixture.Intake, fixture.AppID, fixture.PublicationContractRef, fixture.DraftEvidenceRef)
	assertCandidatePackageIntakeAdoptionStatus(t, rt, "user-alice", approved.AdoptionID, types.AppAdoptionSourceLineageSwitched, fixture.TargetActiveRef)
}

func TestCandidatePackageIntakePromotionSwitchAcceptanceEvidenceRouteAcceptsOnlyLocalLineageProof(t *testing.T) {
	t.Parallel()
	rt, handler := testAPISetup(t)
	srv := candidatePackageIntakeTestServer(handler)
	intakeID := "intake-promotion-switch-acceptance-evidence-success"

	fixture := createCandidatePackageIntakePromotionSwitchReviewThroughRoute(t, rt, srv, "user-alice", intakeID)
	approved := approveCandidatePackageIntakePromotionSwitchReviewThroughRoute(t, srv, fixture)
	switchCandidatePackageIntakePromotionSwitchThroughRoute(t, srv, fixture, approved, "texture://evidence/"+intakeID+"/switch")
	rollbackCandidatePackageIntakePromotionSwitchThroughRoute(t, srv, fixture, approved.AdoptionID)
	rolledForward := rollForwardCandidatePackageIntakePromotionSwitchThroughRoute(t, srv, fixture, approved.AdoptionID, "texture://evidence/"+intakeID+"/roll-forward")
	assertCandidatePackageIntakeTargetLineageSwitch(t, rt, "user-alice", fixture.TargetComputerID, approved.CandidateSourceRef, approved.AdoptionID, fixture.Draft.PackageID, approved.CandidateSourceRef)
	assertCandidatePackageIntakeSingleDraftPackage(t, rt, fixture.Intake, fixture.AppID, fixture.PublicationContractRef, fixture.DraftEvidenceRef)
	assertCandidatePackageIntakeAdoptionHasNoDeployedPromotionFields(t, rolledForward)

	before := candidatePackageIntakeAcceptanceSnapshotForOwner(t, rt, "user-alice")
	acceptanceW := serveCandidatePackageIntakeRequest(srv, http.MethodGet, candidatePackageIntakePromotionSwitchAcceptancePath(fixture.Intake.IntakeID, approved.AdoptionID), "", "user-alice")
	if acceptanceW.Code != http.StatusOK {
		t.Fatalf("promotion switch acceptance status = %d, want 200; body=%s", acceptanceW.Code, acceptanceW.Body.String())
	}
	evidence := candidatePackageIntakePromotionSwitchAcceptanceJSON(t, acceptanceW)
	assertCandidatePackageIntakePromotionSwitchAcceptanceAccepted(t, evidence, fixture, approved)
	assertCandidatePackageIntakeAcceptanceSnapshotUnchanged(t, rt, "user-alice", before)
	assertCandidatePackageIntakeAcceptanceSnapshotHasOnlyLocalDraftAndAdoption(t, before, fixture, approved.AdoptionID)
}

func TestCandidatePackageIntakePromotionSwitchReviewSurfaceRouteReturnsReadOnlyProductSurface(t *testing.T) {
	t.Parallel()
	rt, handler := testAPISetup(t)
	srv := candidatePackageIntakeTestServer(handler)
	intakeID := "intake-promotion-switch-review-surface-success"

	fixture := createCandidatePackageIntakePromotionSwitchReviewThroughRoute(t, rt, srv, "user-alice", intakeID)
	approved := approveCandidatePackageIntakePromotionSwitchReviewThroughRoute(t, srv, fixture)
	switchCandidatePackageIntakePromotionSwitchThroughRoute(t, srv, fixture, approved, "texture://evidence/"+intakeID+"/switch")
	rollbackCandidatePackageIntakePromotionSwitchThroughRoute(t, srv, fixture, approved.AdoptionID)
	rolledForward := rollForwardCandidatePackageIntakePromotionSwitchThroughRoute(t, srv, fixture, approved.AdoptionID, "texture://evidence/"+intakeID+"/roll-forward")
	assertCandidatePackageIntakeTargetLineageSwitch(t, rt, "user-alice", fixture.TargetComputerID, approved.CandidateSourceRef, approved.AdoptionID, fixture.Draft.PackageID, approved.CandidateSourceRef)
	assertCandidatePackageIntakeSingleDraftPackage(t, rt, fixture.Intake, fixture.AppID, fixture.PublicationContractRef, fixture.DraftEvidenceRef)
	assertCandidatePackageIntakeAdoptionHasNoDeployedPromotionFields(t, rolledForward)

	before := candidatePackageIntakeAcceptanceSnapshotForOwner(t, rt, "user-alice")
	reviewSurfaceW := serveCandidatePackageIntakeRequest(srv, http.MethodGet, candidatePackageIntakePromotionSwitchReviewSurfacePath(fixture.Intake.IntakeID, approved.AdoptionID), "", "user-alice")
	if reviewSurfaceW.Code != http.StatusOK {
		t.Fatalf("promotion switch review surface status = %d, want 200; body=%s", reviewSurfaceW.Code, reviewSurfaceW.Body.String())
	}
	surface := candidatePackageIntakePromotionSwitchReviewSurfaceJSON(t, reviewSurfaceW)
	assertCandidatePackageIntakePromotionSwitchReviewSurfaceAccepted(t, surface, fixture, approved)
	assertCandidatePackageIntakeAcceptanceSnapshotUnchanged(t, rt, "user-alice", before)
	assertCandidatePackageIntakeAcceptanceSnapshotHasOnlyLocalDraftAndAdoption(t, before, fixture, approved.AdoptionID)
	assertCandidatePackageIntakeTargetLineageSwitch(t, rt, "user-alice", fixture.TargetComputerID, approved.CandidateSourceRef, approved.AdoptionID, fixture.Draft.PackageID, approved.CandidateSourceRef)
}

func TestCandidatePackageIntakeDeployedReviewHandlerServesOnlyReviewSurface(t *testing.T) {
	t.Parallel()
	rt, handler := testAPISetup(t)
	localSrv := candidatePackageIntakeTestServer(handler)
	deployedSrv := http.HandlerFunc(handler.HandleCandidatePackageReviewSurfaceReadOnly)
	intakeID := "intake-deployed-review-handler-surface"

	fixture := createCandidatePackageIntakePromotionSwitchReviewThroughRoute(t, rt, localSrv, "user-alice", intakeID)
	approved := approveCandidatePackageIntakePromotionSwitchReviewThroughRoute(t, localSrv, fixture)
	switchCandidatePackageIntakePromotionSwitchThroughRoute(t, localSrv, fixture, approved, "texture://evidence/"+intakeID+"/switch")
	rollbackCandidatePackageIntakePromotionSwitchThroughRoute(t, localSrv, fixture, approved.AdoptionID)
	rolledForward := rollForwardCandidatePackageIntakePromotionSwitchThroughRoute(t, localSrv, fixture, approved.AdoptionID, "texture://evidence/"+intakeID+"/roll-forward")
	assertCandidatePackageIntakeAdoptionHasNoDeployedPromotionFields(t, rolledForward)
	assertCandidatePackageIntakeTargetLineageSwitch(t, rt, "user-alice", fixture.TargetComputerID, approved.CandidateSourceRef, approved.AdoptionID, fixture.Draft.PackageID, approved.CandidateSourceRef)
	assertCandidatePackageIntakeSingleDraftPackage(t, rt, fixture.Intake, fixture.AppID, fixture.PublicationContractRef, fixture.DraftEvidenceRef)

	before := candidatePackageIntakeDeployedRouteSnapshotForFixture(t, rt, fixture)
	reviewSurfaceW := serveCandidatePackageIntakeRequest(deployedSrv, http.MethodGet, candidatePackageIntakePromotionSwitchReviewSurfacePath(fixture.Intake.IntakeID, approved.AdoptionID), "", "user-alice")
	if reviewSurfaceW.Code != http.StatusOK {
		t.Fatalf("deployed review-surface status = %d, want 200; body=%s", reviewSurfaceW.Code, reviewSurfaceW.Body.String())
	}
	surface := candidatePackageIntakePromotionSwitchReviewSurfaceJSON(t, reviewSurfaceW)
	assertCandidatePackageIntakePromotionSwitchReviewSurfaceAccepted(t, surface, fixture, approved)
	assertCandidatePackageIntakeDeployedRouteSnapshotUnchanged(t, rt, fixture, "deployed review-surface GET", before)
	assertCandidatePackageIntakeAcceptanceSnapshotHasOnlyLocalDraftAndAdoption(t, before.Acceptance, fixture, approved.AdoptionID)

	for _, tc := range []struct {
		name            string
		method          string
		path            string
		body            string
		wantStatus      int
		forbiddenStatus int
	}{
		{
			name:            "root create is not deployed",
			method:          http.MethodPost,
			path:            "/api/candidate-package-intakes",
			body:            candidatePackageIntakeRequestBody(t, map[string]any{"owner_id": "user-alice", "intake_id": "intake-deployed-register-routes-unexpected-create"}),
			forbiddenStatus: http.StatusCreated,
		},
		{
			name:            "root list is not deployed",
			method:          http.MethodGet,
			path:            "/api/candidate-package-intakes",
			forbiddenStatus: http.StatusOK,
		},
		{
			name:       "intake detail read is not deployed",
			method:     http.MethodGet,
			path:       "/api/candidate-package-intakes/" + fixture.Intake.IntakeID,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "owner review mutation is not deployed",
			method:     http.MethodPost,
			path:       "/api/candidate-package-intakes/" + fixture.Intake.IntakeID + "/review",
			body:       candidatePackageIntakeReviewRequestBody(t, "approve", "texture://review/"+intakeID+"/unexpected-owner-review"),
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:   "adoption boundary mutation is not deployed",
			method: http.MethodPost,
			path:   "/api/candidate-package-intakes/" + fixture.Intake.IntakeID + "/adoption-boundary",
			body: candidatePackageIntakeAdoptionBoundaryRequestBody(
				t,
				"texture://contracts/"+intakeID+"/unexpected-adoption",
				"texture://contracts/"+intakeID+"/unexpected-rollback",
				"texture://evidence/"+intakeID+"/unexpected-boundary",
			),
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:   "publication draft mutation is not deployed",
			method: http.MethodPost,
			path:   "/api/candidate-package-intakes/" + fixture.Intake.IntakeID + "/publication-draft",
			body: candidatePackageIntakePublicationDraftRequestBody(t, map[string]any{
				"app_id":                   "unexpected-deployed-publication-draft-app",
				"publication_contract_ref": "texture://contracts/" + intakeID + "/unexpected-publication",
				"draft_evidence_ref":       "texture://evidence/" + intakeID + "/unexpected-draft",
			}),
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:   "adoption review create mutation is not deployed",
			method: http.MethodPost,
			path:   "/api/candidate-package-intakes/" + fixture.Intake.IntakeID + "/adoption-review",
			body: candidatePackageIntakeAdoptionReviewRequestBody(t, map[string]any{
				"target_computer_id":           fixture.TargetComputerID,
				"target_candidate_id":          "unexpected-deployed-target-candidate",
				"candidate_source_ref":         "refs/computers/" + fixture.TargetComputerID + "/candidates/unexpected-deployed-target-candidate",
				"adoption_review_contract_ref": "texture://contracts/" + intakeID + "/unexpected-adoption-review",
				"review_evidence_ref":          "texture://review/" + intakeID + "/unexpected-adoption-review",
			}),
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "adoption review decision mutation is not deployed",
			method:     http.MethodPost,
			path:       "/api/candidate-package-intakes/" + fixture.Intake.IntakeID + "/adoption-review/" + approved.AdoptionID,
			body:       candidatePackageIntakeAdoptionReviewDecisionRequestBody(t, "approve", "texture://review/"+intakeID+"/unexpected-adoption-decision"),
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "promotion switch mutation is not deployed",
			method:     http.MethodPost,
			path:       "/api/candidate-package-intakes/" + fixture.Intake.IntakeID + "/adoption-review/" + approved.AdoptionID + "/promotion-switch",
			body:       candidatePackageIntakePromotionSwitchRequestBody(t, "texture://evidence/"+intakeID+"/unexpected-switch"),
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "promotion rollback mutation is not deployed",
			method:     http.MethodPost,
			path:       "/api/candidate-package-intakes/" + fixture.Intake.IntakeID + "/adoption-review/" + approved.AdoptionID + "/promotion-switch/rollback",
			body:       candidatePackageIntakePromotionSwitchRollbackRequestBody(t, "texture://evidence/"+intakeID+"/unexpected-rollback"),
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "promotion roll-forward mutation is not deployed",
			method:     http.MethodPost,
			path:       "/api/candidate-package-intakes/" + fixture.Intake.IntakeID + "/adoption-review/" + approved.AdoptionID + "/promotion-switch/roll-forward",
			body:       candidatePackageIntakePromotionSwitchRollForwardRequestBody(t, "texture://evidence/"+intakeID+"/unexpected-roll-forward"),
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "acceptance evidence read is not deployed",
			method:     http.MethodGet,
			path:       candidatePackageIntakePromotionSwitchAcceptancePath(fixture.Intake.IntakeID, approved.AdoptionID),
			wantStatus: http.StatusNotFound,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := candidatePackageIntakeDeployedRouteSnapshotForFixture(t, rt, fixture)
			w := serveCandidatePackageIntakeRequest(deployedSrv, tc.method, tc.path, tc.body, "user-alice")
			assertCandidatePackageIntakeDeployedRouteSnapshotUnchanged(t, rt, fixture, tc.name, before)
			if tc.wantStatus != 0 {
				if w.Code != tc.wantStatus {
					t.Fatalf("%s status = %d, want %d; body=%s", tc.name, w.Code, tc.wantStatus, w.Body.String())
				}
				return
			}
			if w.Code == tc.forbiddenStatus || (w.Code >= 200 && w.Code < 300) {
				t.Fatalf("%s status = %d, want deployed review handler to reject; body=%s", tc.name, w.Code, w.Body.String())
			}
		})
	}
}

func TestRegisterCandidatePackageIntakeRoutesPanicsWhenDeployedEnvSet(t *testing.T) {
	t.Setenv("CHOIR_DEPLOYED_RUNTIME", "true")
	_, handler := testAPISetup(t)
	srv := choirserver.NewServer("candidate-package-intake-deployed-write-route-guard-test", "0")

	defer func() {
		got := recover()
		if got == nil {
			t.Fatalf("RegisterCandidatePackageIntakeRoutes panic = nil, want deployed write-route guard")
		}
		if !strings.Contains(got.(string), "write routes are disabled for deployed runtime") {
			t.Fatalf("RegisterCandidatePackageIntakeRoutes panic = %q, want deployed write-route guard", got)
		}
	}()
	RegisterCandidatePackageIntakeRoutes(srv, handler)
}

func TestCandidatePackageIntakePromotionSwitchReviewSurfaceRouteRejectsWithoutMutation(t *testing.T) {
	t.Parallel()

	t.Run("switched without rollback and roll forward has no accepted review surface", func(t *testing.T) {
		rt, handler := testAPISetup(t)
		srv := candidatePackageIntakeTestServer(handler)
		intakeID := "intake-promotion-switch-review-surface-incomplete"
		fixture := createCandidatePackageIntakePromotionSwitchReviewThroughRoute(t, rt, srv, "user-alice", intakeID)
		approved := approveCandidatePackageIntakePromotionSwitchReviewThroughRoute(t, srv, fixture)
		switchCandidatePackageIntakePromotionSwitchThroughRoute(t, srv, fixture, approved, "texture://evidence/"+intakeID+"/switch")
		before := candidatePackageIntakeAcceptanceSnapshotForOwner(t, rt, "user-alice")

		reviewSurfaceW := serveCandidatePackageIntakeRequest(srv, http.MethodGet, candidatePackageIntakePromotionSwitchReviewSurfacePath(fixture.Intake.IntakeID, approved.AdoptionID), "", "user-alice")
		if reviewSurfaceW.Code != http.StatusBadRequest {
			t.Fatalf("incomplete promotion switch review surface status = %d, want 400 rejection; body=%s", reviewSurfaceW.Code, reviewSurfaceW.Body.String())
		}
		if !strings.Contains(reviewSurfaceW.Body.String(), "rollback") {
			t.Fatalf("incomplete promotion switch review surface body %q does not name missing rollback evidence", reviewSurfaceW.Body.String())
		}
		assertCandidatePackageIntakeAcceptanceSnapshotUnchanged(t, rt, "user-alice", before)
		assertCandidatePackageIntakeTargetLineageSwitch(t, rt, "user-alice", fixture.TargetComputerID, approved.CandidateSourceRef, approved.AdoptionID, fixture.Draft.PackageID, approved.CandidateSourceRef)
	})

	t.Run("rolled back current lineage has no accepted review surface until roll forward", func(t *testing.T) {
		rt, handler := testAPISetup(t)
		srv := candidatePackageIntakeTestServer(handler)
		intakeID := "intake-promotion-switch-review-surface-rolled-back"
		fixture := createCandidatePackageIntakePromotionSwitchReviewThroughRoute(t, rt, srv, "user-alice", intakeID)
		approved := approveCandidatePackageIntakePromotionSwitchReviewThroughRoute(t, srv, fixture)
		switchCandidatePackageIntakePromotionSwitchThroughRoute(t, srv, fixture, approved, "texture://evidence/"+intakeID+"/switch")
		rollbackCandidatePackageIntakePromotionSwitchThroughRoute(t, srv, fixture, approved.AdoptionID)
		before := candidatePackageIntakeAcceptanceSnapshotForOwner(t, rt, "user-alice")

		reviewSurfaceW := serveCandidatePackageIntakeRequest(srv, http.MethodGet, candidatePackageIntakePromotionSwitchReviewSurfacePath(fixture.Intake.IntakeID, approved.AdoptionID), "", "user-alice")
		if reviewSurfaceW.Code != http.StatusBadRequest {
			t.Fatalf("rolled-back promotion switch review surface status = %d, want 400 rejection; body=%s", reviewSurfaceW.Code, reviewSurfaceW.Body.String())
		}
		if !strings.Contains(reviewSurfaceW.Body.String(), "not source_lineage_switched") {
			t.Fatalf("rolled-back promotion switch review surface body %q does not contain current-state contract", reviewSurfaceW.Body.String())
		}
		assertCandidatePackageIntakeAcceptanceSnapshotUnchanged(t, rt, "user-alice", before)
		assertCandidatePackageIntakeTargetLineageNonDeployedActiveRef(t, rt, "user-alice", fixture.TargetComputerID, fixture.TargetActiveRef)
	})

	t.Run("another owner cannot read or create a review surface", func(t *testing.T) {
		rt, handler := testAPISetup(t)
		srv := candidatePackageIntakeTestServer(handler)
		intakeID := "intake-promotion-switch-review-surface-owner-scope"
		fixture := createCandidatePackageIntakePromotionSwitchReviewThroughRoute(t, rt, srv, "user-alice", intakeID)
		approved := approveCandidatePackageIntakePromotionSwitchReviewThroughRoute(t, srv, fixture)
		switchCandidatePackageIntakePromotionSwitchThroughRoute(t, srv, fixture, approved, "texture://evidence/"+intakeID+"/switch")
		rollbackCandidatePackageIntakePromotionSwitchThroughRoute(t, srv, fixture, approved.AdoptionID)
		rollForwardCandidatePackageIntakePromotionSwitchThroughRoute(t, srv, fixture, approved.AdoptionID, "texture://evidence/"+intakeID+"/roll-forward")
		aliceBefore := candidatePackageIntakeAcceptanceSnapshotForOwner(t, rt, "user-alice")
		bobBefore := candidatePackageIntakeAcceptanceSnapshotForOwner(t, rt, "user-bob")

		reviewSurfaceW := serveCandidatePackageIntakeRequest(srv, http.MethodGet, candidatePackageIntakePromotionSwitchReviewSurfacePath(fixture.Intake.IntakeID, approved.AdoptionID), "", "user-bob")
		if reviewSurfaceW.Code != http.StatusNotFound {
			t.Fatalf("other-owner promotion switch review surface status = %d, want 404; body=%s", reviewSurfaceW.Code, reviewSurfaceW.Body.String())
		}
		assertCandidatePackageIntakeAcceptanceSnapshotUnchanged(t, rt, "user-alice", aliceBefore)
		assertCandidatePackageIntakeAcceptanceSnapshotUnchanged(t, rt, "user-bob", bobBefore)
		assertCandidatePackageIntakeTargetLineageSwitch(t, rt, "user-alice", fixture.TargetComputerID, approved.CandidateSourceRef, approved.AdoptionID, fixture.Draft.PackageID, approved.CandidateSourceRef)
	})

	t.Run("non-get method cannot create a review surface", func(t *testing.T) {
		rt, handler := testAPISetup(t)
		srv := candidatePackageIntakeTestServer(handler)
		intakeID := "intake-promotion-switch-review-surface-method"
		fixture := createCandidatePackageIntakePromotionSwitchReviewThroughRoute(t, rt, srv, "user-alice", intakeID)
		approved := approveCandidatePackageIntakePromotionSwitchReviewThroughRoute(t, srv, fixture)
		switchCandidatePackageIntakePromotionSwitchThroughRoute(t, srv, fixture, approved, "texture://evidence/"+intakeID+"/switch")
		rollbackCandidatePackageIntakePromotionSwitchThroughRoute(t, srv, fixture, approved.AdoptionID)
		rollForwardCandidatePackageIntakePromotionSwitchThroughRoute(t, srv, fixture, approved.AdoptionID, "texture://evidence/"+intakeID+"/roll-forward")
		before := candidatePackageIntakeAcceptanceSnapshotForOwner(t, rt, "user-alice")

		reviewSurfaceW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, candidatePackageIntakePromotionSwitchReviewSurfacePath(fixture.Intake.IntakeID, approved.AdoptionID), `{}`, "user-alice")
		if reviewSurfaceW.Code != http.StatusMethodNotAllowed {
			t.Fatalf("post promotion switch review surface status = %d, want 405; body=%s", reviewSurfaceW.Code, reviewSurfaceW.Body.String())
		}
		assertCandidatePackageIntakeAcceptanceSnapshotUnchanged(t, rt, "user-alice", before)
		assertCandidatePackageIntakeTargetLineageSwitch(t, rt, "user-alice", fixture.TargetComputerID, approved.CandidateSourceRef, approved.AdoptionID, fixture.Draft.PackageID, approved.CandidateSourceRef)
	})
}
func TestCandidatePackageIntakePromotionSwitchAcceptanceEvidenceRouteBlocksIncompleteLocalLineageProof(t *testing.T) {
	t.Parallel()

	t.Run("switched without rollback or roll forward is not accepted", func(t *testing.T) {
		rt, handler := testAPISetup(t)
		srv := candidatePackageIntakeTestServer(handler)
		intakeID := "intake-promotion-switch-acceptance-missing-rollback-roll-forward"
		fixture := createCandidatePackageIntakePromotionSwitchReviewThroughRoute(t, rt, srv, "user-alice", intakeID)
		approved := approveCandidatePackageIntakePromotionSwitchReviewThroughRoute(t, srv, fixture)
		switchCandidatePackageIntakePromotionSwitchThroughRoute(t, srv, fixture, approved, "texture://evidence/"+intakeID+"/switch")
		before := candidatePackageIntakeAcceptanceSnapshotForOwner(t, rt, "user-alice")

		acceptanceW := serveCandidatePackageIntakeRequest(srv, http.MethodGet, candidatePackageIntakePromotionSwitchAcceptancePath(fixture.Intake.IntakeID, approved.AdoptionID), "", "user-alice")
		if acceptanceW.Code != http.StatusBadRequest {
			t.Fatalf("incomplete promotion switch acceptance status = %d, want 400 rejection; body=%s", acceptanceW.Code, acceptanceW.Body.String())
		}
		if !strings.Contains(acceptanceW.Body.String(), "rollback") {
			t.Fatalf("incomplete promotion switch acceptance body %q does not name missing rollback evidence", acceptanceW.Body.String())
		}
		assertCandidatePackageIntakeAcceptanceSnapshotUnchanged(t, rt, "user-alice", before)
		assertCandidatePackageIntakeTargetLineageSwitch(t, rt, "user-alice", fixture.TargetComputerID, approved.CandidateSourceRef, approved.AdoptionID, fixture.Draft.PackageID, approved.CandidateSourceRef)
	})

	t.Run("rolled back current lineage is not accepted until roll forward", func(t *testing.T) {
		rt, handler := testAPISetup(t)
		srv := candidatePackageIntakeTestServer(handler)
		intakeID := "intake-promotion-switch-acceptance-rolled-back"
		fixture := createCandidatePackageIntakePromotionSwitchReviewThroughRoute(t, rt, srv, "user-alice", intakeID)
		approved := approveCandidatePackageIntakePromotionSwitchReviewThroughRoute(t, srv, fixture)
		switchCandidatePackageIntakePromotionSwitchThroughRoute(t, srv, fixture, approved, "texture://evidence/"+intakeID+"/switch")
		rollbackCandidatePackageIntakePromotionSwitchThroughRoute(t, srv, fixture, approved.AdoptionID)
		before := candidatePackageIntakeAcceptanceSnapshotForOwner(t, rt, "user-alice")

		acceptanceW := serveCandidatePackageIntakeRequest(srv, http.MethodGet, candidatePackageIntakePromotionSwitchAcceptancePath(fixture.Intake.IntakeID, approved.AdoptionID), "", "user-alice")
		if acceptanceW.Code != http.StatusBadRequest {
			t.Fatalf("rolled-back promotion switch acceptance status = %d, want 400 rejection; body=%s", acceptanceW.Code, acceptanceW.Body.String())
		}
		if !strings.Contains(acceptanceW.Body.String(), "not source_lineage_switched") {
			t.Fatalf("rolled-back promotion switch acceptance body %q does not contain current-state contract", acceptanceW.Body.String())
		}
		assertCandidatePackageIntakeAcceptanceSnapshotUnchanged(t, rt, "user-alice", before)
		assertCandidatePackageIntakeTargetLineageNonDeployedActiveRef(t, rt, "user-alice", fixture.TargetComputerID, fixture.TargetActiveRef)
	})

	t.Run("another owner cannot read or create local acceptance evidence", func(t *testing.T) {
		rt, handler := testAPISetup(t)
		srv := candidatePackageIntakeTestServer(handler)
		intakeID := "intake-promotion-switch-acceptance-owner-scope"
		fixture := createCandidatePackageIntakePromotionSwitchReviewThroughRoute(t, rt, srv, "user-alice", intakeID)
		approved := approveCandidatePackageIntakePromotionSwitchReviewThroughRoute(t, srv, fixture)
		switchCandidatePackageIntakePromotionSwitchThroughRoute(t, srv, fixture, approved, "texture://evidence/"+intakeID+"/switch")
		rollbackCandidatePackageIntakePromotionSwitchThroughRoute(t, srv, fixture, approved.AdoptionID)
		rollForwardCandidatePackageIntakePromotionSwitchThroughRoute(t, srv, fixture, approved.AdoptionID, "texture://evidence/"+intakeID+"/roll-forward")
		aliceBefore := candidatePackageIntakeAcceptanceSnapshotForOwner(t, rt, "user-alice")
		bobBefore := candidatePackageIntakeAcceptanceSnapshotForOwner(t, rt, "user-bob")

		acceptanceW := serveCandidatePackageIntakeRequest(srv, http.MethodGet, candidatePackageIntakePromotionSwitchAcceptancePath(fixture.Intake.IntakeID, approved.AdoptionID), "", "user-bob")
		if acceptanceW.Code != http.StatusNotFound {
			t.Fatalf("other-owner promotion switch acceptance status = %d, want 404; body=%s", acceptanceW.Code, acceptanceW.Body.String())
		}
		assertCandidatePackageIntakeAcceptanceSnapshotUnchanged(t, rt, "user-alice", aliceBefore)
		assertCandidatePackageIntakeAcceptanceSnapshotUnchanged(t, rt, "user-bob", bobBefore)
		assertCandidatePackageIntakeTargetLineageSwitch(t, rt, "user-alice", fixture.TargetComputerID, approved.CandidateSourceRef, approved.AdoptionID, fixture.Draft.PackageID, approved.CandidateSourceRef)
	})
}

func TestCandidatePackageIntakePromotionSwitchRollbackRollForwardRouteRejectsUnsafeTransitions(t *testing.T) {
	t.Parallel()

	t.Run("pending adoption review cannot rollback or roll forward source lineage", func(t *testing.T) {
		rt, handler := testAPISetup(t)
		srv := candidatePackageIntakeTestServer(handler)
		intakeID := "intake-promotion-switch-rollback-pending-review"
		fixture := createCandidatePackageIntakePromotionSwitchReviewThroughRoute(t, rt, srv, "user-alice", intakeID)

		rollbackW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+fixture.Intake.IntakeID+"/adoption-review/"+fixture.Adoption.AdoptionID+"/promotion-switch/rollback", candidatePackageIntakePromotionSwitchRollbackRequestBody(t, "texture://evidence/"+intakeID+"/rollback"), "user-alice")
		if rollbackW.Code != http.StatusBadRequest {
			t.Fatalf("pending-review promotion switch rollback status = %d, want 400; body=%s", rollbackW.Code, rollbackW.Body.String())
		}
		if !strings.Contains(rollbackW.Body.String(), "not source_lineage_switched") {
			t.Fatalf("pending-review promotion switch rollback body %q does not contain switched-state contract", rollbackW.Body.String())
		}
		assertCandidatePackageIntakeTargetLineageSwitch(t, rt, "user-alice", fixture.TargetComputerID, fixture.TargetActiveRef, "", "", "")
		assertCandidatePackageIntakeSingleDraftPackage(t, rt, fixture.Intake, fixture.AppID, fixture.PublicationContractRef, fixture.DraftEvidenceRef)
		assertCandidatePackageIntakeAdoptionStatus(t, rt, "user-alice", fixture.Adoption.AdoptionID, types.AppAdoptionOwnerReviewPending, "")

		rollForwardW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+fixture.Intake.IntakeID+"/adoption-review/"+fixture.Adoption.AdoptionID+"/promotion-switch/roll-forward", candidatePackageIntakePromotionSwitchRollForwardRequestBody(t, "texture://evidence/"+intakeID+"/roll-forward"), "user-alice")
		if rollForwardW.Code != http.StatusBadRequest {
			t.Fatalf("pending-review promotion switch roll-forward status = %d, want 400; body=%s", rollForwardW.Code, rollForwardW.Body.String())
		}
		if !strings.Contains(rollForwardW.Body.String(), "not rolled_back") {
			t.Fatalf("pending-review promotion switch roll-forward body %q does not contain rolled-back-state contract", rollForwardW.Body.String())
		}
		assertCandidatePackageIntakeTargetLineageSwitch(t, rt, "user-alice", fixture.TargetComputerID, fixture.TargetActiveRef, "", "", "")
		assertCandidatePackageIntakeSingleDraftPackage(t, rt, fixture.Intake, fixture.AppID, fixture.PublicationContractRef, fixture.DraftEvidenceRef)
		assertCandidatePackageIntakeAdoptionStatus(t, rt, "user-alice", fixture.Adoption.AdoptionID, types.AppAdoptionOwnerReviewPending, "")
	})

	t.Run("approved but unswitched adoption review cannot rollback source lineage", func(t *testing.T) {
		rt, handler := testAPISetup(t)
		srv := candidatePackageIntakeTestServer(handler)
		intakeID := "intake-promotion-switch-rollback-approved-review"
		fixture := createCandidatePackageIntakePromotionSwitchReviewThroughRoute(t, rt, srv, "user-alice", intakeID)
		approved := approveCandidatePackageIntakePromotionSwitchReviewThroughRoute(t, srv, fixture)

		rollbackW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+fixture.Intake.IntakeID+"/adoption-review/"+approved.AdoptionID+"/promotion-switch/rollback", candidatePackageIntakePromotionSwitchRollbackRequestBody(t, "texture://evidence/"+intakeID+"/rollback"), "user-alice")
		if rollbackW.Code != http.StatusBadRequest {
			t.Fatalf("approved-unswitched promotion switch rollback status = %d, want 400; body=%s", rollbackW.Code, rollbackW.Body.String())
		}
		if !strings.Contains(rollbackW.Body.String(), "not source_lineage_switched") {
			t.Fatalf("approved-unswitched promotion switch rollback body %q does not contain switched-state contract", rollbackW.Body.String())
		}
		assertCandidatePackageIntakeTargetLineageSwitch(t, rt, "user-alice", fixture.TargetComputerID, fixture.TargetActiveRef, "", "", "")
		assertCandidatePackageIntakeSingleDraftPackage(t, rt, fixture.Intake, fixture.AppID, fixture.PublicationContractRef, fixture.DraftEvidenceRef)
		assertCandidatePackageIntakeAdoptionStatus(t, rt, "user-alice", approved.AdoptionID, types.AppAdoptionOwnerReviewApproved, "")
	})

	t.Run("stale target lineage cannot rollback source lineage switch", func(t *testing.T) {
		rt, handler := testAPISetup(t)
		srv := candidatePackageIntakeTestServer(handler)
		intakeID := "intake-promotion-switch-rollback-stale-lineage"
		fixture := createCandidatePackageIntakePromotionSwitchReviewThroughRoute(t, rt, srv, "user-alice", intakeID)
		approved := approveCandidatePackageIntakePromotionSwitchReviewThroughRoute(t, srv, fixture)
		switchCandidatePackageIntakePromotionSwitchThroughRoute(t, srv, fixture, approved, "texture://evidence/"+intakeID+"/switch")
		staleActiveRef := "refs/computers/" + fixture.TargetComputerID + "/active-after-switch"
		lineage, err := rt.store.GetComputerSourceLineage(context.Background(), "user-alice", fixture.TargetComputerID)
		if err != nil {
			t.Fatalf("get stale-rollback fixture: %v", err)
		}
		lineage.ActiveSourceRef = staleActiveRef
		if _, err := rt.store.UpsertComputerSourceLineage(context.Background(), lineage); err != nil {
			t.Fatalf("persist stale-rollback fixture: %v", err)
		}

		rollbackW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+fixture.Intake.IntakeID+"/adoption-review/"+approved.AdoptionID+"/promotion-switch/rollback", candidatePackageIntakePromotionSwitchRollbackRequestBody(t, "texture://evidence/"+intakeID+"/rollback"), "user-alice")
		if rollbackW.Code != http.StatusBadRequest {
			t.Fatalf("stale-lineage promotion switch rollback status = %d, want 400; body=%s", rollbackW.Code, rollbackW.Body.String())
		}
		if !strings.Contains(rollbackW.Body.String(), "foreground lineage moved") {
			t.Fatalf("stale-lineage promotion switch rollback body %q does not contain stale lineage contract", rollbackW.Body.String())
		}
		assertCandidatePackageIntakeTargetLineageNonDeployedActiveRef(t, rt, "user-alice", fixture.TargetComputerID, staleActiveRef)
		assertCandidatePackageIntakeSingleDraftPackage(t, rt, fixture.Intake, fixture.AppID, fixture.PublicationContractRef, fixture.DraftEvidenceRef)
		assertCandidatePackageIntakeAdoptionStatus(t, rt, "user-alice", approved.AdoptionID, types.AppAdoptionSourceLineageSwitched, fixture.TargetActiveRef)
	})

	t.Run("rolled back adoption review rejects duplicate rollback", func(t *testing.T) {
		rt, handler := testAPISetup(t)
		srv := candidatePackageIntakeTestServer(handler)
		intakeID := "intake-promotion-switch-rollback-duplicate"
		fixture := createCandidatePackageIntakePromotionSwitchReviewThroughRoute(t, rt, srv, "user-alice", intakeID)
		approved := approveCandidatePackageIntakePromotionSwitchReviewThroughRoute(t, srv, fixture)
		switchCandidatePackageIntakePromotionSwitchThroughRoute(t, srv, fixture, approved, "texture://evidence/"+intakeID+"/switch")
		rollbackCandidatePackageIntakePromotionSwitchThroughRoute(t, srv, fixture, approved.AdoptionID)

		secondRollbackW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+fixture.Intake.IntakeID+"/adoption-review/"+approved.AdoptionID+"/promotion-switch/rollback", candidatePackageIntakePromotionSwitchRollbackRequestBody(t, "texture://evidence/"+intakeID+"/rollback-again"), "user-alice")
		if secondRollbackW.Code != http.StatusBadRequest {
			t.Fatalf("duplicate promotion switch rollback status = %d, want 400; body=%s", secondRollbackW.Code, secondRollbackW.Body.String())
		}
		if !strings.Contains(secondRollbackW.Body.String(), "already rolled back") {
			t.Fatalf("duplicate promotion switch rollback body %q does not contain already-rolled-back contract", secondRollbackW.Body.String())
		}
		assertCandidatePackageIntakeTargetLineageNonDeployedActiveRef(t, rt, "user-alice", fixture.TargetComputerID, fixture.TargetActiveRef)
		assertCandidatePackageIntakeSingleDraftPackage(t, rt, fixture.Intake, fixture.AppID, fixture.PublicationContractRef, fixture.DraftEvidenceRef)
		assertCandidatePackageIntakeAdoptionStatus(t, rt, "user-alice", approved.AdoptionID, types.AppAdoptionRolledBack, fixture.TargetActiveRef)
	})

	t.Run("stale target lineage cannot roll forward rolled back switch", func(t *testing.T) {
		rt, handler := testAPISetup(t)
		srv := candidatePackageIntakeTestServer(handler)
		intakeID := "intake-promotion-switch-roll-forward-stale-lineage"
		fixture := createCandidatePackageIntakePromotionSwitchReviewThroughRoute(t, rt, srv, "user-alice", intakeID)
		approved := approveCandidatePackageIntakePromotionSwitchReviewThroughRoute(t, srv, fixture)
		switchCandidatePackageIntakePromotionSwitchThroughRoute(t, srv, fixture, approved, "texture://evidence/"+intakeID+"/switch")
		rollbackCandidatePackageIntakePromotionSwitchThroughRoute(t, srv, fixture, approved.AdoptionID)
		staleActiveRef := "refs/computers/" + fixture.TargetComputerID + "/active-after-rollback"
		lineage, err := rt.store.GetComputerSourceLineage(context.Background(), "user-alice", fixture.TargetComputerID)
		if err != nil {
			t.Fatalf("get stale-roll-forward fixture: %v", err)
		}
		lineage.ActiveSourceRef = staleActiveRef
		if _, err := rt.store.UpsertComputerSourceLineage(context.Background(), lineage); err != nil {
			t.Fatalf("persist stale-roll-forward fixture: %v", err)
		}

		rollForwardW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+fixture.Intake.IntakeID+"/adoption-review/"+approved.AdoptionID+"/promotion-switch/roll-forward", candidatePackageIntakePromotionSwitchRollForwardRequestBody(t, "texture://evidence/"+intakeID+"/roll-forward"), "user-alice")
		if rollForwardW.Code != http.StatusBadRequest {
			t.Fatalf("stale-lineage promotion switch roll-forward status = %d, want 400; body=%s", rollForwardW.Code, rollForwardW.Body.String())
		}
		if !strings.Contains(rollForwardW.Body.String(), "foreground lineage moved") {
			t.Fatalf("stale-lineage promotion switch roll-forward body %q does not contain stale lineage contract", rollForwardW.Body.String())
		}
		assertCandidatePackageIntakeTargetLineageNonDeployedActiveRef(t, rt, "user-alice", fixture.TargetComputerID, staleActiveRef)
		assertCandidatePackageIntakeSingleDraftPackage(t, rt, fixture.Intake, fixture.AppID, fixture.PublicationContractRef, fixture.DraftEvidenceRef)
		assertCandidatePackageIntakeAdoptionStatus(t, rt, "user-alice", approved.AdoptionID, types.AppAdoptionRolledBack, fixture.TargetActiveRef)
	})
}

func TestCandidatePackageIntakePromotionSwitchRouteRejectsUnsafeTransitions(t *testing.T) {
	t.Parallel()

	t.Run("pending adoption review cannot switch source lineage", func(t *testing.T) {
		rt, handler := testAPISetup(t)
		srv := candidatePackageIntakeTestServer(handler)
		fixture := createCandidatePackageIntakePromotionSwitchReviewThroughRoute(t, rt, srv, "user-alice", "intake-promotion-switch-pending-review")

		switchW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+fixture.Intake.IntakeID+"/adoption-review/"+fixture.Adoption.AdoptionID+"/promotion-switch", candidatePackageIntakePromotionSwitchRequestBody(t, "texture://evidence/intake-promotion-switch-pending-review/switch"), "user-alice")
		if switchW.Code != http.StatusBadRequest {
			t.Fatalf("pending-review promotion switch status = %d, want 400; body=%s", switchW.Code, switchW.Body.String())
		}
		if !strings.Contains(switchW.Body.String(), "not owner_review_approved") {
			t.Fatalf("pending-review promotion switch body %q does not contain owner approval contract", switchW.Body.String())
		}
		assertCandidatePackageIntakeTargetLineageSwitch(t, rt, "user-alice", fixture.TargetComputerID, fixture.TargetActiveRef, "", "", "")
		assertCandidatePackageIntakeSingleDraftPackage(t, rt, fixture.Intake, fixture.AppID, fixture.PublicationContractRef, fixture.DraftEvidenceRef)
		assertCandidatePackageIntakeAdoptionStatus(t, rt, "user-alice", fixture.Adoption.AdoptionID, types.AppAdoptionOwnerReviewPending, "")
	})

	t.Run("rejected adoption review cannot switch source lineage", func(t *testing.T) {
		rt, handler := testAPISetup(t)
		srv := candidatePackageIntakeTestServer(handler)
		intakeID := "intake-promotion-switch-rejected-review"
		fixture := createCandidatePackageIntakePromotionSwitchReviewThroughRoute(t, rt, srv, "user-alice", intakeID)
		rejectW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+fixture.Intake.IntakeID+"/adoption-review/"+fixture.Adoption.AdoptionID, candidatePackageIntakeAdoptionReviewDecisionRequestBody(t, "reject", "texture://review/"+intakeID+"/adoption-reject"), "user-alice")
		if rejectW.Code != http.StatusOK {
			t.Fatalf("reject before promotion switch status = %d, want 200; body=%s", rejectW.Code, rejectW.Body.String())
		}

		switchW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+fixture.Intake.IntakeID+"/adoption-review/"+fixture.Adoption.AdoptionID+"/promotion-switch", candidatePackageIntakePromotionSwitchRequestBody(t, "texture://evidence/"+intakeID+"/switch"), "user-alice")
		if switchW.Code != http.StatusBadRequest {
			t.Fatalf("rejected-review promotion switch status = %d, want 400; body=%s", switchW.Code, switchW.Body.String())
		}
		if !strings.Contains(switchW.Body.String(), "not owner_review_approved") {
			t.Fatalf("rejected-review promotion switch body %q does not contain owner approval contract", switchW.Body.String())
		}
		assertCandidatePackageIntakeTargetLineageSwitch(t, rt, "user-alice", fixture.TargetComputerID, fixture.TargetActiveRef, "", "", "")
		assertCandidatePackageIntakeSingleDraftPackage(t, rt, fixture.Intake, fixture.AppID, fixture.PublicationContractRef, fixture.DraftEvidenceRef)
		assertCandidatePackageIntakeAdoptionStatus(t, rt, "user-alice", fixture.Adoption.AdoptionID, types.AppAdoptionOwnerReviewRejected, "")
	})

	t.Run("stale target lineage cannot switch source lineage", func(t *testing.T) {
		rt, handler := testAPISetup(t)
		srv := candidatePackageIntakeTestServer(handler)
		intakeID := "intake-promotion-switch-stale-lineage"
		fixture := createCandidatePackageIntakePromotionSwitchReviewThroughRoute(t, rt, srv, "user-alice", intakeID)
		approved := approveCandidatePackageIntakePromotionSwitchReviewThroughRoute(t, srv, fixture)
		staleActiveRef := "refs/computers/" + fixture.TargetComputerID + "/active-after-review"
		lineage, err := rt.store.GetComputerSourceLineage(context.Background(), "user-alice", fixture.TargetComputerID)
		if err != nil {
			t.Fatalf("get stale-lineage fixture: %v", err)
		}
		lineage.ActiveSourceRef = staleActiveRef
		if _, err := rt.store.UpsertComputerSourceLineage(context.Background(), lineage); err != nil {
			t.Fatalf("persist stale-lineage fixture: %v", err)
		}

		switchW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+fixture.Intake.IntakeID+"/adoption-review/"+approved.AdoptionID+"/promotion-switch", candidatePackageIntakePromotionSwitchRequestBody(t, "texture://evidence/"+intakeID+"/switch"), "user-alice")
		if switchW.Code != http.StatusBadRequest {
			t.Fatalf("stale-lineage promotion switch status = %d, want 400; body=%s", switchW.Code, switchW.Body.String())
		}
		if !strings.Contains(switchW.Body.String(), "foreground lineage moved") {
			t.Fatalf("stale-lineage promotion switch body %q does not contain stale lineage contract", switchW.Body.String())
		}
		assertCandidatePackageIntakeTargetLineageSwitch(t, rt, "user-alice", fixture.TargetComputerID, staleActiveRef, "", "", "")
		assertCandidatePackageIntakeSingleDraftPackage(t, rt, fixture.Intake, fixture.AppID, fixture.PublicationContractRef, fixture.DraftEvidenceRef)
		assertCandidatePackageIntakeAdoptionStatus(t, rt, "user-alice", approved.AdoptionID, types.AppAdoptionOwnerReviewApproved, "")
	})

	t.Run("missing candidate source ref cannot switch source lineage", func(t *testing.T) {
		rt, handler := testAPISetup(t)
		srv := candidatePackageIntakeTestServer(handler)
		intakeID := "intake-promotion-switch-missing-candidate-ref"
		fixture := createCandidatePackageIntakePromotionSwitchReviewThroughRoute(t, rt, srv, "user-alice", intakeID)
		approved := approveCandidatePackageIntakePromotionSwitchReviewThroughRoute(t, srv, fixture)
		approved.CandidateSourceRef = ""
		if _, err := rt.store.UpsertAppAdoption(context.Background(), approved); err != nil {
			t.Fatalf("persist missing candidate_source_ref fixture: %v", err)
		}

		switchW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+fixture.Intake.IntakeID+"/adoption-review/"+approved.AdoptionID+"/promotion-switch", candidatePackageIntakePromotionSwitchRequestBody(t, "texture://evidence/"+intakeID+"/switch"), "user-alice")
		if switchW.Code != http.StatusBadRequest {
			t.Fatalf("missing-candidate-ref promotion switch status = %d, want 400; body=%s", switchW.Code, switchW.Body.String())
		}
		if !strings.Contains(switchW.Body.String(), "candidate_source_ref must be a candidate ref") {
			t.Fatalf("missing-candidate-ref promotion switch body %q does not contain candidate ref contract", switchW.Body.String())
		}
		assertCandidatePackageIntakeTargetLineageSwitch(t, rt, "user-alice", fixture.TargetComputerID, fixture.TargetActiveRef, "", "", "")
		assertCandidatePackageIntakeSingleDraftPackage(t, rt, fixture.Intake, fixture.AppID, fixture.PublicationContractRef, fixture.DraftEvidenceRef)
		assertCandidatePackageIntakeAdoptionStatus(t, rt, "user-alice", approved.AdoptionID, types.AppAdoptionOwnerReviewApproved, "")
	})

	t.Run("invalid candidate source ref cannot switch source lineage", func(t *testing.T) {
		rt, handler := testAPISetup(t)
		srv := candidatePackageIntakeTestServer(handler)
		intakeID := "intake-promotion-switch-invalid-candidate-ref"
		fixture := createCandidatePackageIntakePromotionSwitchReviewThroughRoute(t, rt, srv, "user-alice", intakeID)
		approved := approveCandidatePackageIntakePromotionSwitchReviewThroughRoute(t, srv, fixture)
		approved.CandidateSourceRef = "refs/computers/" + fixture.TargetComputerID + "/not-a-candidate-ref"
		if _, err := rt.store.UpsertAppAdoption(context.Background(), approved); err != nil {
			t.Fatalf("persist invalid candidate_source_ref fixture: %v", err)
		}

		switchW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+fixture.Intake.IntakeID+"/adoption-review/"+approved.AdoptionID+"/promotion-switch", candidatePackageIntakePromotionSwitchRequestBody(t, "texture://evidence/"+intakeID+"/switch"), "user-alice")
		if switchW.Code != http.StatusBadRequest {
			t.Fatalf("invalid-candidate-ref promotion switch status = %d, want 400; body=%s", switchW.Code, switchW.Body.String())
		}
		if !strings.Contains(switchW.Body.String(), "candidate_source_ref must be a candidate ref") {
			t.Fatalf("invalid-candidate-ref promotion switch body %q does not contain candidate ref contract", switchW.Body.String())
		}
		assertCandidatePackageIntakeTargetLineageSwitch(t, rt, "user-alice", fixture.TargetComputerID, fixture.TargetActiveRef, "", "", "")
		assertCandidatePackageIntakeSingleDraftPackage(t, rt, fixture.Intake, fixture.AppID, fixture.PublicationContractRef, fixture.DraftEvidenceRef)
		assertCandidatePackageIntakeAdoptionStatus(t, rt, "user-alice", approved.AdoptionID, types.AppAdoptionOwnerReviewApproved, "")
	})

	t.Run("another owner cannot switch source lineage", func(t *testing.T) {
		rt, handler := testAPISetup(t)
		srv := candidatePackageIntakeTestServer(handler)
		intakeID := "intake-promotion-switch-owner-scope"
		fixture := createCandidatePackageIntakePromotionSwitchReviewThroughRoute(t, rt, srv, "user-alice", intakeID)
		approved := approveCandidatePackageIntakePromotionSwitchReviewThroughRoute(t, srv, fixture)
		bobBefore := candidatePackageIntakePromotionSnapshotForOwner(t, rt, "user-bob")

		switchW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+fixture.Intake.IntakeID+"/adoption-review/"+approved.AdoptionID+"/promotion-switch", candidatePackageIntakePromotionSwitchRequestBody(t, "texture://evidence/"+intakeID+"/bob-switch"), "user-bob")
		if switchW.Code != http.StatusNotFound {
			t.Fatalf("other-owner promotion switch status = %d, want 404; body=%s", switchW.Code, switchW.Body.String())
		}
		assertCandidatePackageIntakeTargetLineageSwitch(t, rt, "user-alice", fixture.TargetComputerID, fixture.TargetActiveRef, "", "", "")
		assertCandidatePackageIntakeSingleDraftPackage(t, rt, fixture.Intake, fixture.AppID, fixture.PublicationContractRef, fixture.DraftEvidenceRef)
		assertCandidatePackageIntakeAdoptionStatus(t, rt, "user-alice", approved.AdoptionID, types.AppAdoptionOwnerReviewApproved, "")
		assertCandidatePackageIntakePromotionSnapshotUnchanged(t, rt, "user-bob", bobBefore)
	})

	t.Run("already switched adoption review cannot switch source lineage again", func(t *testing.T) {
		rt, handler := testAPISetup(t)
		srv := candidatePackageIntakeTestServer(handler)
		intakeID := "intake-promotion-switch-duplicate"
		fixture := createCandidatePackageIntakePromotionSwitchReviewThroughRoute(t, rt, srv, "user-alice", intakeID)
		approved := approveCandidatePackageIntakePromotionSwitchReviewThroughRoute(t, srv, fixture)
		firstSwitchW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+fixture.Intake.IntakeID+"/adoption-review/"+approved.AdoptionID+"/promotion-switch", candidatePackageIntakePromotionSwitchRequestBody(t, "texture://evidence/"+intakeID+"/switch"), "user-alice")
		if firstSwitchW.Code != http.StatusOK {
			t.Fatalf("initial promotion switch status = %d, want 200; body=%s", firstSwitchW.Code, firstSwitchW.Body.String())
		}

		secondSwitchW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+fixture.Intake.IntakeID+"/adoption-review/"+approved.AdoptionID+"/promotion-switch", candidatePackageIntakePromotionSwitchRequestBody(t, "texture://evidence/"+intakeID+"/switch-again"), "user-alice")
		if secondSwitchW.Code != http.StatusBadRequest {
			t.Fatalf("duplicate promotion switch status = %d, want 400; body=%s", secondSwitchW.Code, secondSwitchW.Body.String())
		}
		if !strings.Contains(secondSwitchW.Body.String(), "already switched source lineage") {
			t.Fatalf("duplicate promotion switch body %q does not contain already-switched contract", secondSwitchW.Body.String())
		}
		assertCandidatePackageIntakeTargetLineageSwitch(t, rt, "user-alice", fixture.TargetComputerID, approved.CandidateSourceRef, approved.AdoptionID, fixture.Draft.PackageID, approved.CandidateSourceRef)
		assertCandidatePackageIntakeSingleDraftPackage(t, rt, fixture.Intake, fixture.AppID, fixture.PublicationContractRef, fixture.DraftEvidenceRef)
		assertCandidatePackageIntakeAdoptionStatus(t, rt, "user-alice", approved.AdoptionID, types.AppAdoptionSourceLineageSwitched, fixture.TargetActiveRef)
	})
}

func TestCandidatePackageIntakePublicationDraftRouteRejectsUnsafeTransitions(t *testing.T) {
	t.Parallel()

	t.Run("pending intake cannot create publication draft", func(t *testing.T) {
		rt, handler := testAPISetup(t)
		srv := candidatePackageIntakeTestServer(handler)
		pending := createCandidatePackageIntakeForReview(t, srv, "user-alice", "intake-publication-draft-pending", map[string]any{
			"adoption_blockers_json": []string{"owner_review_not_recorded"},
			"evidence_refs_json":     []string{"texture://evidence/intake-publication-draft-pending/source"},
		})
		before := candidatePackageIntakePromotionSnapshotForOwner(t, rt, "user-alice")

		draftW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+pending.IntakeID+"/publication-draft", candidatePackageIntakePublicationDraftRequestBody(t, map[string]any{
			"publication_contract_ref": "texture://contracts/intake-publication-draft-pending/publication",
			"draft_evidence_ref":       "texture://evidence/intake-publication-draft-pending/draft",
		}), "user-alice")
		if draftW.Code != http.StatusBadRequest {
			t.Fatalf("pending publication draft status = %d, want 400; body=%s", draftW.Code, draftW.Body.String())
		}
		if !strings.Contains(draftW.Body.String(), "not owner-approved") {
			t.Fatalf("pending publication draft body %q does not contain owner-approved contract", draftW.Body.String())
		}
		assertCandidatePackageIntakePromotionSnapshotUnchanged(t, rt, "user-alice", before)
	})

	t.Run("approved intake without adoption readiness cannot create publication draft", func(t *testing.T) {
		rt, handler := testAPISetup(t)
		srv := candidatePackageIntakeTestServer(handler)
		pending := createCandidatePackageIntakeForReview(t, srv, "user-alice", "intake-publication-draft-not-ready", map[string]any{
			"adoption_blockers_json": []string{"owner_review_not_recorded"},
			"evidence_refs_json":     []string{"texture://evidence/intake-publication-draft-not-ready/source"},
		})
		approveW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+pending.IntakeID+"/review", candidatePackageIntakeReviewRequestBody(t, "approve", "texture://review/intake-publication-draft-not-ready/owner"), "user-alice")
		if approveW.Code != http.StatusOK {
			t.Fatalf("approve before publication draft status = %d, want 200; body=%s", approveW.Code, approveW.Body.String())
		}
		before := candidatePackageIntakePromotionSnapshotForOwner(t, rt, "user-alice")

		draftW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+pending.IntakeID+"/publication-draft", candidatePackageIntakePublicationDraftRequestBody(t, map[string]any{
			"publication_contract_ref": "texture://contracts/intake-publication-draft-not-ready/publication",
			"draft_evidence_ref":       "texture://evidence/intake-publication-draft-not-ready/draft",
		}), "user-alice")
		if draftW.Code != http.StatusBadRequest {
			t.Fatalf("not-ready publication draft status = %d, want 400; body=%s", draftW.Code, draftW.Body.String())
		}
		if !strings.Contains(draftW.Body.String(), "not adoption-ready") {
			t.Fatalf("not-ready publication draft body %q does not contain adoption-ready contract", draftW.Body.String())
		}
		assertCandidatePackageIntakePromotionSnapshotUnchanged(t, rt, "user-alice", before)
	})

	t.Run("adoption-ready intake without bound adoption boundary cannot create publication draft", func(t *testing.T) {
		rt, handler := testAPISetup(t)
		srv := candidatePackageIntakeTestServer(handler)
		pending := createCandidatePackageIntakeForReview(t, srv, "user-alice", "intake-publication-draft-missing-boundary", map[string]any{
			"adoption_blockers_json": []string{"owner_review_not_recorded"},
			"evidence_refs_json":     []string{"texture://evidence/intake-publication-draft-missing-boundary/source"},
		})
		approveW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+pending.IntakeID+"/review", candidatePackageIntakeReviewRequestBody(t, "approve", "texture://review/intake-publication-draft-missing-boundary/owner"), "user-alice")
		if approveW.Code != http.StatusOK {
			t.Fatalf("approve before missing-boundary publication draft status = %d, want 200; body=%s", approveW.Code, approveW.Body.String())
		}
		approved := candidatePackageIntakeDetail(t, srv, pending.IntakeID, "user-alice")
		approved.AdoptionReady = true
		approved.AdoptionBlockersJSON = json.RawMessage(`[]`)
		approved.AcceptanceJSON = json.RawMessage(`{}`)
		if _, err := rt.store.UpsertCandidatePackageIntake(context.Background(), approved); err != nil {
			t.Fatalf("persist inconsistent adoption-ready intake fixture: %v", err)
		}
		before := candidatePackageIntakePromotionSnapshotForOwner(t, rt, "user-alice")

		draftW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+pending.IntakeID+"/publication-draft", candidatePackageIntakePublicationDraftRequestBody(t, map[string]any{
			"publication_contract_ref": "texture://contracts/intake-publication-draft-missing-boundary/publication",
			"draft_evidence_ref":       "texture://evidence/intake-publication-draft-missing-boundary/draft",
		}), "user-alice")
		if draftW.Code != http.StatusBadRequest {
			t.Fatalf("missing-boundary publication draft status = %d, want 400; body=%s", draftW.Code, draftW.Body.String())
		}
		if !strings.Contains(draftW.Body.String(), "adoption_rollback_boundary is required") {
			t.Fatalf("missing-boundary publication draft body %q does not contain adoption boundary contract", draftW.Body.String())
		}
		assertCandidatePackageIntakePromotionSnapshotUnchanged(t, rt, "user-alice", before)
	})

	t.Run("rejected intake cannot create publication draft", func(t *testing.T) {
		rt, handler := testAPISetup(t)
		srv := candidatePackageIntakeTestServer(handler)
		pending := createCandidatePackageIntakeForReview(t, srv, "user-alice", "intake-publication-draft-rejected", map[string]any{
			"adoption_blockers_json": []string{"owner_review_not_recorded"},
			"evidence_refs_json":     []string{"texture://evidence/intake-publication-draft-rejected/source"},
		})
		rejectW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+pending.IntakeID+"/review", candidatePackageIntakeReviewRequestBody(t, "reject", "texture://review/intake-publication-draft-rejected/owner"), "user-alice")
		if rejectW.Code != http.StatusOK {
			t.Fatalf("reject before publication draft status = %d, want 200; body=%s", rejectW.Code, rejectW.Body.String())
		}
		before := candidatePackageIntakePromotionSnapshotForOwner(t, rt, "user-alice")

		draftW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+pending.IntakeID+"/publication-draft", candidatePackageIntakePublicationDraftRequestBody(t, map[string]any{
			"publication_contract_ref": "texture://contracts/intake-publication-draft-rejected/publication",
			"draft_evidence_ref":       "texture://evidence/intake-publication-draft-rejected/draft",
		}), "user-alice")
		if draftW.Code != http.StatusBadRequest {
			t.Fatalf("rejected publication draft status = %d, want 400; body=%s", draftW.Code, draftW.Body.String())
		}
		if !strings.Contains(draftW.Body.String(), "not owner-approved") {
			t.Fatalf("rejected publication draft body %q does not contain owner-approved contract", draftW.Body.String())
		}
		assertCandidatePackageIntakePromotionSnapshotUnchanged(t, rt, "user-alice", before)
	})

	t.Run("another owner cannot create publication draft", func(t *testing.T) {
		rt, handler := testAPISetup(t)
		srv := candidatePackageIntakeTestServer(handler)
		ready := createCandidatePackageIntakeReadyForPublication(t, srv, "user-alice", "intake-publication-draft-owner-scope", map[string]any{
			"evidence_refs_json": []string{"texture://evidence/intake-publication-draft-owner-scope/source"},
		})
		aliceBefore := candidatePackageIntakePromotionSnapshotForOwner(t, rt, "user-alice")
		bobBefore := candidatePackageIntakePromotionSnapshotForOwner(t, rt, "user-bob")

		draftW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+ready.IntakeID+"/publication-draft", candidatePackageIntakePublicationDraftRequestBody(t, map[string]any{
			"publication_contract_ref": "texture://contracts/intake-publication-draft-owner-scope/publication",
			"draft_evidence_ref":       "texture://evidence/intake-publication-draft-owner-scope/bob-draft",
		}), "user-bob")
		if draftW.Code != http.StatusNotFound {
			t.Fatalf("other-owner publication draft status = %d, want 404; body=%s", draftW.Code, draftW.Body.String())
		}
		assertCandidatePackageIntakePromotionSnapshotUnchanged(t, rt, "user-alice", aliceBefore)
		assertCandidatePackageIntakePromotionSnapshotUnchanged(t, rt, "user-bob", bobBefore)
	})

	t.Run("missing publication contract ref cannot create publication draft", func(t *testing.T) {
		rt, handler := testAPISetup(t)
		srv := candidatePackageIntakeTestServer(handler)
		ready := createCandidatePackageIntakeReadyForPublication(t, srv, "user-alice", "intake-publication-draft-missing-contract", map[string]any{
			"evidence_refs_json": []string{"texture://evidence/intake-publication-draft-missing-contract/source"},
		})
		before := candidatePackageIntakePromotionSnapshotForOwner(t, rt, "user-alice")

		draftW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+ready.IntakeID+"/publication-draft", candidatePackageIntakePublicationDraftRequestBody(t, map[string]any{
			"publication_contract_ref": "",
			"draft_evidence_ref":       "texture://evidence/intake-publication-draft-missing-contract/draft",
		}), "user-alice")
		if draftW.Code != http.StatusBadRequest {
			t.Fatalf("missing-contract publication draft status = %d, want 400; body=%s", draftW.Code, draftW.Body.String())
		}
		if !strings.Contains(draftW.Body.String(), "publication_contract_ref is required") {
			t.Fatalf("missing-contract publication draft body %q does not contain publication contract requirement", draftW.Body.String())
		}
		assertCandidatePackageIntakePromotionSnapshotUnchanged(t, rt, "user-alice", before)
	})

	t.Run("package id collision with unrelated package cannot create publication draft", func(t *testing.T) {
		rt, handler := testAPISetup(t)
		srv := candidatePackageIntakeTestServer(handler)
		ready := createCandidatePackageIntakeReadyForPublication(t, srv, "user-alice", "intake-publication-draft-collision", map[string]any{
			"evidence_refs_json": []string{"texture://evidence/intake-publication-draft-collision/source"},
		})
		collidingPackageID := "pkg-publication-draft-collision"
		if _, err := rt.store.UpsertAppChangePackage(context.Background(), types.AppChangePackageRecord{
			PackageID:             collidingPackageID,
			OwnerID:               "user-alice",
			AppID:                 "unrelated-package",
			Status:                types.AppChangePackagePublishedPrivate,
			Visibility:            "private",
			SourceComputerID:      "computer-unrelated-source",
			SourceCandidateID:     "candidate-unrelated-source",
			CandidateSourceRef:    "refs/computers/computer-unrelated-source/candidates/candidate-unrelated-source",
			PackageManifestSHA256: "sha256:unrelated-package",
			ManifestJSON:          json.RawMessage(`{"kind":"unrelated_app_change_package"}`),
			VerifierContractsJSON: json.RawMessage(`[]`),
			ProvenanceRefsJSON:    json.RawMessage(`[]`),
		}); err != nil {
			t.Fatalf("persist colliding package fixture: %v", err)
		}
		before := candidatePackageIntakePromotionSnapshotForOwner(t, rt, "user-alice")

		draftW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+ready.IntakeID+"/publication-draft", candidatePackageIntakePublicationDraftRequestBody(t, map[string]any{
			"package_id":               collidingPackageID,
			"publication_contract_ref": "texture://contracts/intake-publication-draft-collision/publication",
			"draft_evidence_ref":       "texture://evidence/intake-publication-draft-collision/draft",
		}), "user-alice")
		if draftW.Code != http.StatusBadRequest {
			t.Fatalf("colliding-package publication draft status = %d, want 400; body=%s", draftW.Code, draftW.Body.String())
		}
		if !strings.Contains(draftW.Body.String(), "already exists for another package") {
			t.Fatalf("colliding-package publication draft body %q does not contain package collision contract", draftW.Body.String())
		}
		assertCandidatePackageIntakePromotionSnapshotUnchanged(t, rt, "user-alice", before)
	})
}

func TestCandidatePackageIntakeAdoptionReviewRouteRejectsUnsafeTransitions(t *testing.T) {
	t.Parallel()

	t.Run("no draft package cannot create adoption review", func(t *testing.T) {
		rt, handler := testAPISetup(t)
		srv := candidatePackageIntakeTestServer(handler)
		ready := createCandidatePackageIntakeReadyForPublication(t, srv, "user-alice", "intake-adoption-review-no-draft", map[string]any{
			"evidence_refs_json": []string{"texture://evidence/intake-adoption-review-no-draft/source"},
		})
		before := candidatePackageIntakePromotionSnapshotForOwner(t, rt, "user-alice")

		createW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+ready.IntakeID+"/adoption-review", candidatePackageIntakeAdoptionReviewRequestBody(t, map[string]any{
			"target_computer_id":           "computer-adoption-review-no-draft-target",
			"adoption_review_contract_ref": "texture://contracts/intake-adoption-review-no-draft/adoption-review",
		}), "user-alice")
		if createW.Code != http.StatusBadRequest {
			t.Fatalf("no-draft adoption review status = %d, want 400; body=%s", createW.Code, createW.Body.String())
		}
		if !strings.Contains(createW.Body.String(), "publication draft") {
			t.Fatalf("no-draft adoption review body %q does not contain publication draft contract", createW.Body.String())
		}
		assertCandidatePackageIntakePromotionSnapshotUnchanged(t, rt, "user-alice", before)
	})

	t.Run("another owner cannot create adoption review", func(t *testing.T) {
		rt, handler := testAPISetup(t)
		srv := candidatePackageIntakeTestServer(handler)
		ready := createCandidatePackageIntakeReadyForPublication(t, srv, "user-alice", "intake-adoption-review-owner-scope", map[string]any{
			"evidence_refs_json": []string{"texture://evidence/intake-adoption-review-owner-scope/source"},
		})
		createCandidatePackageIntakePublicationDraftThroughRoute(t, srv, ready, "user-alice", "candidate-intake-adoption-review-owner-scope-app", "texture://contracts/intake-adoption-review-owner-scope/publication", "texture://evidence/intake-adoption-review-owner-scope/draft")
		alicePackagesBefore := candidatePackageIntakePromotionSnapshotForOwner(t, rt, "user-alice").Packages
		bobBefore := candidatePackageIntakePromotionSnapshotForOwner(t, rt, "user-bob")

		createW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+ready.IntakeID+"/adoption-review", candidatePackageIntakeAdoptionReviewRequestBody(t, map[string]any{
			"target_computer_id":           "computer-adoption-review-owner-scope-target",
			"adoption_review_contract_ref": "texture://contracts/intake-adoption-review-owner-scope/adoption-review",
		}), "user-bob")
		if createW.Code != http.StatusNotFound {
			t.Fatalf("other-owner adoption review status = %d, want 404; body=%s", createW.Code, createW.Body.String())
		}
		assertCandidatePackageIntakePackagesUnchanged(t, rt, "user-alice", alicePackagesBefore)
		assertCandidatePackageIntakePromotionSnapshotUnchanged(t, rt, "user-bob", bobBefore)
		assertCandidatePackageIntakeAdoptionCount(t, rt, "user-alice", 0)
	})

	t.Run("draft manifest must match intake manifest", func(t *testing.T) {
		rt, handler := testAPISetup(t)
		srv := candidatePackageIntakeTestServer(handler)
		ready := createCandidatePackageIntakeReadyForPublication(t, srv, "user-alice", "intake-adoption-review-manifest-mismatch", map[string]any{
			"evidence_refs_json": []string{"texture://evidence/intake-adoption-review-manifest-mismatch/source"},
		})
		upsertCandidatePackageIntakePublicationDraftFixture(t, rt, ready, "candidate-intake-adoption-review-manifest-mismatch-app", types.AppChangePackageDraft, "private", map[string]any{
			"candidate_package_manifest_sha256": "sha256:not-the-intake-manifest",
		})
		packagesBefore := candidatePackageIntakePromotionSnapshotForOwner(t, rt, "user-alice").Packages

		createW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+ready.IntakeID+"/adoption-review", candidatePackageIntakeAdoptionReviewRequestBody(t, map[string]any{
			"target_computer_id":           "computer-adoption-review-manifest-mismatch-target",
			"adoption_review_contract_ref": "texture://contracts/intake-adoption-review-manifest-mismatch/adoption-review",
		}), "user-alice")
		if createW.Code != http.StatusBadRequest {
			t.Fatalf("manifest-mismatch adoption review status = %d, want 400; body=%s", createW.Code, createW.Body.String())
		}
		if !strings.Contains(createW.Body.String(), "manifest") {
			t.Fatalf("manifest-mismatch adoption review body %q does not contain manifest contract", createW.Body.String())
		}
		assertCandidatePackageIntakePackagesUnchanged(t, rt, "user-alice", packagesBefore)
		assertCandidatePackageIntakeAdoptionCount(t, rt, "user-alice", 0)
	})

	t.Run("published package cannot create adoption review", func(t *testing.T) {
		rt, handler := testAPISetup(t)
		srv := candidatePackageIntakeTestServer(handler)
		ready := createCandidatePackageIntakeReadyForPublication(t, srv, "user-alice", "intake-adoption-review-published-package", map[string]any{
			"evidence_refs_json": []string{"texture://evidence/intake-adoption-review-published-package/source"},
		})
		draft := createCandidatePackageIntakePublicationDraftThroughRoute(t, srv, ready, "user-alice", "candidate-intake-adoption-review-published-package-app", "texture://contracts/intake-adoption-review-published-package/publication", "texture://evidence/intake-adoption-review-published-package/draft")
		draft.Status = types.AppChangePackagePublishedPrivate
		if _, err := rt.store.UpsertAppChangePackage(context.Background(), draft); err != nil {
			t.Fatalf("publish adoption review draft fixture: %v", err)
		}
		packagesBefore := candidatePackageIntakePromotionSnapshotForOwner(t, rt, "user-alice").Packages

		createW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+ready.IntakeID+"/adoption-review", candidatePackageIntakeAdoptionReviewRequestBody(t, map[string]any{
			"target_computer_id":           "computer-adoption-review-published-package-target",
			"adoption_review_contract_ref": "texture://contracts/intake-adoption-review-published-package/adoption-review",
		}), "user-alice")
		if createW.Code != http.StatusBadRequest {
			t.Fatalf("published-package adoption review status = %d, want 400; body=%s", createW.Code, createW.Body.String())
		}
		if !strings.Contains(createW.Body.String(), "private draft") {
			t.Fatalf("published-package adoption review body %q does not contain private draft contract", createW.Body.String())
		}
		assertCandidatePackageIntakePackagesUnchanged(t, rt, "user-alice", packagesBefore)
		assertCandidatePackageIntakeAdoptionCount(t, rt, "user-alice", 0)
	})

	t.Run("missing adoption review contract ref cannot create adoption review", func(t *testing.T) {
		rt, handler := testAPISetup(t)
		srv := candidatePackageIntakeTestServer(handler)
		ready := createCandidatePackageIntakeReadyForPublication(t, srv, "user-alice", "intake-adoption-review-missing-contract", map[string]any{
			"evidence_refs_json": []string{"texture://evidence/intake-adoption-review-missing-contract/source"},
		})
		createCandidatePackageIntakePublicationDraftThroughRoute(t, srv, ready, "user-alice", "candidate-intake-adoption-review-missing-contract-app", "texture://contracts/intake-adoption-review-missing-contract/publication", "texture://evidence/intake-adoption-review-missing-contract/draft")
		packagesBefore := candidatePackageIntakePromotionSnapshotForOwner(t, rt, "user-alice").Packages

		createW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+ready.IntakeID+"/adoption-review", candidatePackageIntakeAdoptionReviewRequestBody(t, map[string]any{
			"target_computer_id":           "computer-adoption-review-missing-contract-target",
			"adoption_review_contract_ref": "",
		}), "user-alice")
		if createW.Code != http.StatusBadRequest {
			t.Fatalf("missing-contract adoption review status = %d, want 400; body=%s", createW.Code, createW.Body.String())
		}
		if !strings.Contains(createW.Body.String(), "adoption_review_contract_ref is required") {
			t.Fatalf("missing-contract adoption review body %q does not contain contract requirement", createW.Body.String())
		}
		assertCandidatePackageIntakePackagesUnchanged(t, rt, "user-alice", packagesBefore)
		assertCandidatePackageIntakeAdoptionCount(t, rt, "user-alice", 0)
	})

	t.Run("pending intake cannot create adoption review even with matching draft", func(t *testing.T) {
		rt, handler := testAPISetup(t)
		srv := candidatePackageIntakeTestServer(handler)
		pending := createCandidatePackageIntakeForReview(t, srv, "user-alice", "intake-adoption-review-pending", map[string]any{
			"adoption_blockers_json": []string{"owner_review_not_recorded"},
			"evidence_refs_json":     []string{"texture://evidence/intake-adoption-review-pending/source"},
		})
		upsertCandidatePackageIntakePublicationDraftFixture(t, rt, pending, "candidate-intake-adoption-review-pending-app", types.AppChangePackageDraft, "private", nil)
		packagesBefore := candidatePackageIntakePromotionSnapshotForOwner(t, rt, "user-alice").Packages

		createW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+pending.IntakeID+"/adoption-review", candidatePackageIntakeAdoptionReviewRequestBody(t, map[string]any{
			"target_computer_id":           "computer-adoption-review-pending-target",
			"adoption_review_contract_ref": "texture://contracts/intake-adoption-review-pending/adoption-review",
		}), "user-alice")
		if createW.Code != http.StatusBadRequest {
			t.Fatalf("pending adoption review status = %d, want 400; body=%s", createW.Code, createW.Body.String())
		}
		if !strings.Contains(createW.Body.String(), "not owner-approved") {
			t.Fatalf("pending adoption review body %q does not contain owner-approved contract", createW.Body.String())
		}
		assertCandidatePackageIntakePackagesUnchanged(t, rt, "user-alice", packagesBefore)
		assertCandidatePackageIntakeAdoptionCount(t, rt, "user-alice", 0)
	})

	t.Run("owner-approved intake without adoption readiness cannot create adoption review", func(t *testing.T) {
		rt, handler := testAPISetup(t)
		srv := candidatePackageIntakeTestServer(handler)
		pending := createCandidatePackageIntakeForReview(t, srv, "user-alice", "intake-adoption-review-not-ready", map[string]any{
			"adoption_blockers_json": []string{"owner_review_not_recorded"},
			"evidence_refs_json":     []string{"texture://evidence/intake-adoption-review-not-ready/source"},
		})
		approveW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+pending.IntakeID+"/review", candidatePackageIntakeReviewRequestBody(t, "approve", "texture://review/intake-adoption-review-not-ready/owner"), "user-alice")
		if approveW.Code != http.StatusOK {
			t.Fatalf("approve before not-ready adoption review status = %d, want 200; body=%s", approveW.Code, approveW.Body.String())
		}
		approved := candidatePackageIntakeDetail(t, srv, pending.IntakeID, "user-alice")
		upsertCandidatePackageIntakePublicationDraftFixture(t, rt, approved, "candidate-intake-adoption-review-not-ready-app", types.AppChangePackageDraft, "private", nil)
		packagesBefore := candidatePackageIntakePromotionSnapshotForOwner(t, rt, "user-alice").Packages

		createW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+approved.IntakeID+"/adoption-review", candidatePackageIntakeAdoptionReviewRequestBody(t, map[string]any{
			"target_computer_id":           "computer-adoption-review-not-ready-target",
			"adoption_review_contract_ref": "texture://contracts/intake-adoption-review-not-ready/adoption-review",
		}), "user-alice")
		if createW.Code != http.StatusBadRequest {
			t.Fatalf("not-ready adoption review status = %d, want 400; body=%s", createW.Code, createW.Body.String())
		}
		if !strings.Contains(createW.Body.String(), "not adoption-ready") {
			t.Fatalf("not-ready adoption review body %q does not contain adoption-ready contract", createW.Body.String())
		}
		assertCandidatePackageIntakePackagesUnchanged(t, rt, "user-alice", packagesBefore)
		assertCandidatePackageIntakeAdoptionCount(t, rt, "user-alice", 0)
	})

	t.Run("duplicate adoption review create is bounded to one pending adoption", func(t *testing.T) {
		rt, handler := testAPISetup(t)
		srv := candidatePackageIntakeTestServer(handler)
		intakeID := "intake-adoption-review-duplicate"
		ready := createCandidatePackageIntakeReadyForPublication(t, srv, "user-alice", intakeID, map[string]any{
			"evidence_refs_json": []string{"texture://evidence/" + intakeID + "/source"},
		})
		createCandidatePackageIntakePublicationDraftThroughRoute(t, srv, ready, "user-alice", "candidate-intake-adoption-review-duplicate-app", "texture://contracts/"+intakeID+"/publication", "texture://evidence/"+intakeID+"/draft")
		targetComputerID := "computer-adoption-review-duplicate-target"
		targetActiveRef := "refs/computers/" + targetComputerID + "/active-before-review"
		if _, err := rt.promotion.EnsureComputerSourceLineage(context.Background(), "user-alice", targetComputerID, "workspace", targetActiveRef); err != nil {
			t.Fatalf("seed duplicate target source lineage: %v", err)
		}
		firstW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+ready.IntakeID+"/adoption-review", candidatePackageIntakeAdoptionReviewRequestBody(t, map[string]any{
			"target_computer_id":           targetComputerID,
			"adoption_review_contract_ref": "texture://contracts/" + intakeID + "/adoption-review",
		}), "user-alice")
		if firstW.Code != http.StatusCreated {
			t.Fatalf("initial duplicate adoption review status = %d, want 201; body=%s", firstW.Code, firstW.Body.String())
		}
		var first types.AppAdoptionRecord
		if err := json.Unmarshal(firstW.Body.Bytes(), &first); err != nil {
			t.Fatalf("decode initial duplicate adoption review: %v", err)
		}

		duplicateW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+ready.IntakeID+"/adoption-review", candidatePackageIntakeAdoptionReviewRequestBody(t, map[string]any{
			"target_computer_id":           targetComputerID,
			"adoption_review_contract_ref": "texture://contracts/" + intakeID + "/adoption-review-second",
		}), "user-alice")
		if duplicateW.Code != http.StatusBadRequest {
			t.Fatalf("duplicate adoption review status = %d, want 400; body=%s", duplicateW.Code, duplicateW.Body.String())
		}
		if !strings.Contains(duplicateW.Body.String(), "already has an adoption review") {
			t.Fatalf("duplicate adoption review body %q does not contain duplicate contract", duplicateW.Body.String())
		}
		adoptions := assertCandidatePackageIntakeAdoptionCount(t, rt, "user-alice", 1)
		if adoptions[0].AdoptionID != first.AdoptionID || string(adoptions[0].Status) != "owner_review_pending" {
			t.Fatalf("duplicate adoption review changed adoption: %+v; want original pending %q", adoptions, first.AdoptionID)
		}
		assertCandidatePackageIntakeSingleDraftPackage(t, rt, ready, "candidate-intake-adoption-review-duplicate-app", "texture://contracts/"+intakeID+"/publication", "texture://evidence/"+intakeID+"/draft")
		assertCandidatePackageIntakeTargetLineageActiveRef(t, rt, "user-alice", targetComputerID, targetActiveRef)
	})

	t.Run("rejecting pending adoption review records owner review rejection", func(t *testing.T) {
		rt, handler := testAPISetup(t)
		srv := candidatePackageIntakeTestServer(handler)
		intakeID := "intake-adoption-review-reject"
		targetComputerID := "computer-adoption-review-reject-target"
		targetActiveRef := "refs/computers/" + targetComputerID + "/active-before-review"
		if _, err := rt.promotion.EnsureComputerSourceLineage(context.Background(), "user-alice", targetComputerID, "workspace", targetActiveRef); err != nil {
			t.Fatalf("seed reject target source lineage: %v", err)
		}
		ready := createCandidatePackageIntakeReadyForPublication(t, srv, "user-alice", intakeID, map[string]any{
			"evidence_refs_json": []string{"texture://evidence/" + intakeID + "/source"},
		})
		draft := createCandidatePackageIntakePublicationDraftThroughRoute(t, srv, ready, "user-alice", "candidate-intake-adoption-review-reject-app", "texture://contracts/"+intakeID+"/publication", "texture://evidence/"+intakeID+"/draft")
		createW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+ready.IntakeID+"/adoption-review", candidatePackageIntakeAdoptionReviewRequestBody(t, map[string]any{
			"target_computer_id":           targetComputerID,
			"adoption_review_contract_ref": "texture://contracts/" + intakeID + "/adoption-review",
		}), "user-alice")
		if createW.Code != http.StatusCreated {
			t.Fatalf("reject fixture adoption review status = %d, want 201; body=%s", createW.Code, createW.Body.String())
		}
		var pending types.AppAdoptionRecord
		if err := json.Unmarshal(createW.Body.Bytes(), &pending); err != nil {
			t.Fatalf("decode reject fixture adoption review: %v", err)
		}

		rejectW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+ready.IntakeID+"/adoption-review/"+pending.AdoptionID, candidatePackageIntakeAdoptionReviewDecisionRequestBody(t, "reject", "texture://review/"+intakeID+"/adoption-reject"), "user-alice")
		if rejectW.Code != http.StatusOK {
			t.Fatalf("adoption review reject status = %d, want 200; body=%s", rejectW.Code, rejectW.Body.String())
		}
		var rejected types.AppAdoptionRecord
		if err := json.Unmarshal(rejectW.Body.Bytes(), &rejected); err != nil {
			t.Fatalf("decode adoption review reject response: %v", err)
		}
		assertCandidatePackageIntakeAdoptionReviewRecord(t, rejected, ready, draft, targetComputerID, "owner_review_rejected", targetActiveRef, "texture://contracts/"+intakeID+"/adoption-review")
		assertCandidatePackageIntakeAdoptionCount(t, rt, "user-alice", 1)
		assertCandidatePackageIntakeSingleDraftPackage(t, rt, ready, "candidate-intake-adoption-review-reject-app", "texture://contracts/"+intakeID+"/publication", "texture://evidence/"+intakeID+"/draft")
		assertCandidatePackageIntakeTargetLineageActiveRef(t, rt, "user-alice", targetComputerID, targetActiveRef)
	})

	t.Run("terminal rejected adoption review cannot be reviewed again", func(t *testing.T) {
		rt, handler := testAPISetup(t)
		srv := candidatePackageIntakeTestServer(handler)
		intakeID := "intake-adoption-review-terminal-rejected"
		targetComputerID := "computer-adoption-review-terminal-rejected-target"
		targetActiveRef := "refs/computers/" + targetComputerID + "/active-before-review"
		if _, err := rt.promotion.EnsureComputerSourceLineage(context.Background(), "user-alice", targetComputerID, "workspace", targetActiveRef); err != nil {
			t.Fatalf("seed terminal reject target source lineage: %v", err)
		}
		ready := createCandidatePackageIntakeReadyForPublication(t, srv, "user-alice", intakeID, map[string]any{
			"evidence_refs_json": []string{"texture://evidence/" + intakeID + "/source"},
		})
		createCandidatePackageIntakePublicationDraftThroughRoute(t, srv, ready, "user-alice", "candidate-intake-adoption-review-terminal-rejected-app", "texture://contracts/"+intakeID+"/publication", "texture://evidence/"+intakeID+"/draft")
		createW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+ready.IntakeID+"/adoption-review", candidatePackageIntakeAdoptionReviewRequestBody(t, map[string]any{
			"target_computer_id":           targetComputerID,
			"adoption_review_contract_ref": "texture://contracts/" + intakeID + "/adoption-review",
		}), "user-alice")
		if createW.Code != http.StatusCreated {
			t.Fatalf("terminal rejected fixture adoption review status = %d, want 201; body=%s", createW.Code, createW.Body.String())
		}
		var pending types.AppAdoptionRecord
		if err := json.Unmarshal(createW.Body.Bytes(), &pending); err != nil {
			t.Fatalf("decode terminal rejected fixture adoption review: %v", err)
		}
		rejectW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+ready.IntakeID+"/adoption-review/"+pending.AdoptionID, candidatePackageIntakeAdoptionReviewDecisionRequestBody(t, "reject", "texture://review/"+intakeID+"/adoption-reject"), "user-alice")
		if rejectW.Code != http.StatusOK {
			t.Fatalf("terminal rejected fixture reject status = %d, want 200; body=%s", rejectW.Code, rejectW.Body.String())
		}

		rereviewW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+ready.IntakeID+"/adoption-review/"+pending.AdoptionID, candidatePackageIntakeAdoptionReviewDecisionRequestBody(t, "approve", "texture://review/"+intakeID+"/adoption-approve-after-reject"), "user-alice")
		if rereviewW.Code != http.StatusBadRequest {
			t.Fatalf("terminal rejected rereview status = %d, want 400; body=%s", rereviewW.Code, rereviewW.Body.String())
		}
		if !strings.Contains(rereviewW.Body.String(), "already terminal") {
			t.Fatalf("terminal rejected rereview body %q does not contain terminal contract", rereviewW.Body.String())
		}
		adoptions := assertCandidatePackageIntakeAdoptionCount(t, rt, "user-alice", 1)
		if adoptions[0].AdoptionID != pending.AdoptionID || string(adoptions[0].Status) != "owner_review_rejected" {
			t.Fatalf("terminal rejected rereview changed adoption: %+v; want rejected %q", adoptions, pending.AdoptionID)
		}
		assertCandidatePackageIntakeTargetLineageActiveRef(t, rt, "user-alice", targetComputerID, targetActiveRef)
	})

	t.Run("another owner cannot review pending adoption review", func(t *testing.T) {
		rt, handler := testAPISetup(t)
		srv := candidatePackageIntakeTestServer(handler)
		intakeID := "intake-adoption-review-review-owner-scope"
		ready := createCandidatePackageIntakeReadyForPublication(t, srv, "user-alice", intakeID, map[string]any{
			"evidence_refs_json": []string{"texture://evidence/" + intakeID + "/source"},
		})
		createCandidatePackageIntakePublicationDraftThroughRoute(t, srv, ready, "user-alice", "candidate-intake-adoption-review-review-owner-scope-app", "texture://contracts/"+intakeID+"/publication", "texture://evidence/"+intakeID+"/draft")
		createW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+ready.IntakeID+"/adoption-review", candidatePackageIntakeAdoptionReviewRequestBody(t, map[string]any{
			"target_computer_id":           "computer-adoption-review-review-owner-scope-target",
			"adoption_review_contract_ref": "texture://contracts/" + intakeID + "/adoption-review",
		}), "user-alice")
		if createW.Code != http.StatusCreated {
			t.Fatalf("review owner-scope fixture adoption review status = %d, want 201; body=%s", createW.Code, createW.Body.String())
		}
		var pending types.AppAdoptionRecord
		if err := json.Unmarshal(createW.Body.Bytes(), &pending); err != nil {
			t.Fatalf("decode review owner-scope fixture adoption review: %v", err)
		}

		bobReviewW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+ready.IntakeID+"/adoption-review/"+pending.AdoptionID, candidatePackageIntakeAdoptionReviewDecisionRequestBody(t, "approve", "texture://review/"+intakeID+"/bob-approve"), "user-bob")
		if bobReviewW.Code != http.StatusNotFound {
			t.Fatalf("other-owner adoption review decision status = %d, want 404; body=%s", bobReviewW.Code, bobReviewW.Body.String())
		}
		adoptions := assertCandidatePackageIntakeAdoptionCount(t, rt, "user-alice", 1)
		if adoptions[0].AdoptionID != pending.AdoptionID || string(adoptions[0].Status) != "owner_review_pending" {
			t.Fatalf("other-owner review changed adoption: %+v; want pending %q", adoptions, pending.AdoptionID)
		}
		assertCandidatePackageIntakeAdoptionCount(t, rt, "user-bob", 0)
	})
}

func TestCandidatePackageIntakeAdoptionBoundaryRouteRejectsUnsafeTransitions(t *testing.T) {
	t.Parallel()

	t.Run("pending intake cannot bind adoption rollback boundary", func(t *testing.T) {
		rt, handler := testAPISetup(t)
		srv := candidatePackageIntakeTestServer(handler)
		pending := createCandidatePackageIntakeForReview(t, srv, "user-alice", "intake-adoption-boundary-pending", map[string]any{
			"adoption_blockers_json": []string{"owner_review_not_recorded"},
			"evidence_refs_json":     []string{"texture://evidence/intake-adoption-boundary-pending/source"},
		})

		boundaryW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+pending.IntakeID+"/adoption-boundary", candidatePackageIntakeAdoptionBoundaryRequestBody(t, "texture://contracts/intake-adoption-boundary-pending/adoption", "texture://contracts/intake-adoption-boundary-pending/rollback", "texture://evidence/intake-adoption-boundary-pending/boundary"), "user-alice")
		if boundaryW.Code != http.StatusBadRequest {
			t.Fatalf("pending adoption boundary status = %d, want 400; body=%s", boundaryW.Code, boundaryW.Body.String())
		}
		if !strings.Contains(boundaryW.Body.String(), "not owner-approved") {
			t.Fatalf("pending adoption boundary body %q does not contain owner-approved contract", boundaryW.Body.String())
		}
		assertCandidatePackageIntakeDetailMatchesPending(t, srv, pending, "user-alice")
		detail := candidatePackageIntakeDetail(t, srv, pending.IntakeID, "user-alice")
		assertCandidatePackageIntakeJSONStringArrayNotContains(t, "pending evidence refs", detail.EvidenceRefsJSON, "texture://evidence/intake-adoption-boundary-pending/boundary")
		assertCandidatePackageIntakeJSONNoAdoptionBoundary(t, detail.AcceptanceJSON)
		assertNoCandidatePackageIntakePromotionSideEffects(t, rt, "user-alice")
	})

	t.Run("rejected intake cannot bind adoption rollback boundary", func(t *testing.T) {
		rt, handler := testAPISetup(t)
		srv := candidatePackageIntakeTestServer(handler)
		pending := createCandidatePackageIntakeForReview(t, srv, "user-alice", "intake-adoption-boundary-rejected", map[string]any{
			"adoption_blockers_json": []string{"owner_review_not_recorded"},
			"evidence_refs_json":     []string{"texture://evidence/intake-adoption-boundary-rejected/source"},
		})
		rejectW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+pending.IntakeID+"/review", candidatePackageIntakeReviewRequestBody(t, "reject", "texture://review/intake-adoption-boundary-rejected/owner"), "user-alice")
		if rejectW.Code != http.StatusOK {
			t.Fatalf("reject before adoption boundary status = %d, want 200; body=%s", rejectW.Code, rejectW.Body.String())
		}

		boundaryW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+pending.IntakeID+"/adoption-boundary", candidatePackageIntakeAdoptionBoundaryRequestBody(t, "texture://contracts/intake-adoption-boundary-rejected/adoption", "texture://contracts/intake-adoption-boundary-rejected/rollback", "texture://evidence/intake-adoption-boundary-rejected/boundary"), "user-alice")
		if boundaryW.Code != http.StatusBadRequest {
			t.Fatalf("rejected adoption boundary status = %d, want 400; body=%s", boundaryW.Code, boundaryW.Body.String())
		}
		if !strings.Contains(boundaryW.Body.String(), "not owner-approved") {
			t.Fatalf("rejected adoption boundary body %q does not contain owner-approved contract", boundaryW.Body.String())
		}
		detail := candidatePackageIntakeDetail(t, srv, pending.IntakeID, "user-alice")
		assertCandidatePackageIntakeReviewTransition(t, detail, pending, types.CandidatePackageIntakeRejected, types.CandidatePackageOwnerReviewRejected)
		assertCandidatePackageIntakeJSONStringArrayContains(t, "rejected blockers", detail.AdoptionBlockersJSON, "owner_review_rejected")
		assertCandidatePackageIntakeJSONStringArrayNotContains(t, "rejected evidence refs", detail.EvidenceRefsJSON, "texture://evidence/intake-adoption-boundary-rejected/boundary")
		assertCandidatePackageIntakeJSONNoAdoptionBoundary(t, detail.AcceptanceJSON)
		assertNoCandidatePackageIntakePromotionSideEffects(t, rt, "user-alice")
	})

	t.Run("another owner cannot bind adoption rollback boundary", func(t *testing.T) {
		rt, handler := testAPISetup(t)
		srv := candidatePackageIntakeTestServer(handler)
		pending := createCandidatePackageIntakeForReview(t, srv, "user-alice", "intake-adoption-boundary-owner-scope", map[string]any{
			"adoption_blockers_json": []string{"owner_review_not_recorded"},
			"evidence_refs_json":     []string{"texture://evidence/intake-adoption-boundary-owner-scope/source"},
		})
		approveW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+pending.IntakeID+"/review", candidatePackageIntakeReviewRequestBody(t, "approve", "texture://review/intake-adoption-boundary-owner-scope/owner"), "user-alice")
		if approveW.Code != http.StatusOK {
			t.Fatalf("approve before other-owner adoption boundary status = %d, want 200; body=%s", approveW.Code, approveW.Body.String())
		}

		boundaryW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+pending.IntakeID+"/adoption-boundary", candidatePackageIntakeAdoptionBoundaryRequestBody(t, "texture://contracts/intake-adoption-boundary-owner-scope/adoption", "texture://contracts/intake-adoption-boundary-owner-scope/rollback", "texture://evidence/intake-adoption-boundary-owner-scope/bob"), "user-bob")
		if boundaryW.Code != http.StatusNotFound {
			t.Fatalf("other-owner adoption boundary status = %d, want 404; body=%s", boundaryW.Code, boundaryW.Body.String())
		}
		detail := candidatePackageIntakeDetail(t, srv, pending.IntakeID, "user-alice")
		assertCandidatePackageIntakeReviewTransition(t, detail, pending, types.CandidatePackageIntakeOwnerApproved, types.CandidatePackageOwnerReviewApproved)
		assertCandidatePackageIntakeJSONStringArrayContains(t, "approved owner-scoped blockers", detail.AdoptionBlockersJSON, "adoption_rollback_boundary_not_bound")
		assertCandidatePackageIntakeJSONStringArrayNotContains(t, "approved owner-scoped evidence refs", detail.EvidenceRefsJSON, "texture://evidence/intake-adoption-boundary-owner-scope/bob")
		assertCandidatePackageIntakeJSONNoAdoptionBoundary(t, detail.AcceptanceJSON)
		assertNoCandidatePackageIntakePromotionSideEffects(t, rt, "user-alice")
		assertNoCandidatePackageIntakePromotionSideEffects(t, rt, "user-bob")
	})

	t.Run("missing required refs leave approved intake unchanged", func(t *testing.T) {
		for _, tc := range []struct {
			name                string
			adoptionContractRef string
			rollbackContractRef string
			wantBody            string
		}{
			{
				name:                "missing adoption contract ref",
				adoptionContractRef: "",
				rollbackContractRef: "texture://contracts/intake-adoption-boundary-missing-ref/rollback",
				wantBody:            "adoption_contract_ref is required",
			},
			{
				name:                "missing rollback contract ref",
				adoptionContractRef: "texture://contracts/intake-adoption-boundary-missing-ref/adoption",
				rollbackContractRef: "",
				wantBody:            "rollback_contract_ref is required",
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				rt, handler := testAPISetup(t)
				srv := candidatePackageIntakeTestServer(handler)
				pending := createCandidatePackageIntakeForReview(t, srv, "user-alice", "intake-adoption-boundary-"+strings.ReplaceAll(tc.name, " ", "-"), map[string]any{
					"adoption_blockers_json": []string{"owner_review_not_recorded"},
					"evidence_refs_json":     []string{"texture://evidence/intake-adoption-boundary-missing-ref/source"},
				})
				approveW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+pending.IntakeID+"/review", candidatePackageIntakeReviewRequestBody(t, "approve", "texture://review/intake-adoption-boundary-missing-ref/owner"), "user-alice")
				if approveW.Code != http.StatusOK {
					t.Fatalf("approve before invalid adoption boundary status = %d, want 200; body=%s", approveW.Code, approveW.Body.String())
				}

				boundaryW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+pending.IntakeID+"/adoption-boundary", candidatePackageIntakeAdoptionBoundaryRequestBody(t, tc.adoptionContractRef, tc.rollbackContractRef, "texture://evidence/intake-adoption-boundary-missing-ref/boundary"), "user-alice")
				if boundaryW.Code != http.StatusBadRequest {
					t.Fatalf("missing-ref adoption boundary status = %d, want 400; body=%s", boundaryW.Code, boundaryW.Body.String())
				}
				if !strings.Contains(boundaryW.Body.String(), tc.wantBody) {
					t.Fatalf("missing-ref adoption boundary body %q does not contain %q", boundaryW.Body.String(), tc.wantBody)
				}
				detail := candidatePackageIntakeDetail(t, srv, pending.IntakeID, "user-alice")
				assertCandidatePackageIntakeReviewTransition(t, detail, pending, types.CandidatePackageIntakeOwnerApproved, types.CandidatePackageOwnerReviewApproved)
				assertCandidatePackageIntakeJSONStringArrayContains(t, "approved missing-ref blockers", detail.AdoptionBlockersJSON, "adoption_rollback_boundary_not_bound")
				assertCandidatePackageIntakeJSONStringArrayNotContains(t, "approved missing-ref evidence refs", detail.EvidenceRefsJSON, "texture://evidence/intake-adoption-boundary-missing-ref/boundary")
				assertCandidatePackageIntakeJSONNoAdoptionBoundary(t, detail.AcceptanceJSON)
				assertNoCandidatePackageIntakePromotionSideEffects(t, rt, "user-alice")
			})
		}
	})

	t.Run("adoption-ready intake cannot be rebound", func(t *testing.T) {
		rt, handler := testAPISetup(t)
		srv := candidatePackageIntakeTestServer(handler)
		pending := createCandidatePackageIntakeForReview(t, srv, "user-alice", "intake-adoption-boundary-rebind", map[string]any{
			"adoption_blockers_json": []string{"owner_review_not_recorded"},
			"evidence_refs_json":     []string{"texture://evidence/intake-adoption-boundary-rebind/source"},
		})
		approveW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+pending.IntakeID+"/review", candidatePackageIntakeReviewRequestBody(t, "approve", "texture://review/intake-adoption-boundary-rebind/owner"), "user-alice")
		if approveW.Code != http.StatusOK {
			t.Fatalf("approve before rebind status = %d, want 200; body=%s", approveW.Code, approveW.Body.String())
		}
		firstW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+pending.IntakeID+"/adoption-boundary", candidatePackageIntakeAdoptionBoundaryRequestBody(t, "texture://contracts/intake-adoption-boundary-rebind/adoption", "texture://contracts/intake-adoption-boundary-rebind/rollback", "texture://evidence/intake-adoption-boundary-rebind/boundary"), "user-alice")
		if firstW.Code != http.StatusOK {
			t.Fatalf("initial adoption boundary status = %d, want 200; body=%s", firstW.Code, firstW.Body.String())
		}

		rebindW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+pending.IntakeID+"/adoption-boundary", candidatePackageIntakeAdoptionBoundaryRequestBody(t, "texture://contracts/intake-adoption-boundary-rebind/adoption-second", "texture://contracts/intake-adoption-boundary-rebind/rollback-second", "texture://evidence/intake-adoption-boundary-rebind/boundary-second"), "user-alice")
		if rebindW.Code != http.StatusBadRequest {
			t.Fatalf("rebind adoption boundary status = %d, want 400; body=%s", rebindW.Code, rebindW.Body.String())
		}
		if !strings.Contains(rebindW.Body.String(), "already adoption-ready") {
			t.Fatalf("rebind adoption boundary body %q does not contain adoption-ready contract", rebindW.Body.String())
		}
		detail := candidatePackageIntakeDetail(t, srv, pending.IntakeID, "user-alice")
		if !detail.AdoptionReady {
			t.Fatalf("rebind changed adoption_ready to false: %+v", detail)
		}
		assertCandidatePackageIntakeJSONStringArrayContains(t, "rebind evidence refs", detail.EvidenceRefsJSON, "texture://evidence/intake-adoption-boundary-rebind/boundary")
		assertCandidatePackageIntakeJSONStringArrayNotContains(t, "rebind evidence refs", detail.EvidenceRefsJSON, "texture://evidence/intake-adoption-boundary-rebind/boundary-second")
		assertCandidatePackageIntakeAdoptionBoundaryAcceptance(t, detail.AcceptanceJSON, "texture://contracts/intake-adoption-boundary-rebind/adoption", "texture://contracts/intake-adoption-boundary-rebind/rollback")
		assertNoCandidatePackageIntakePromotionSideEffects(t, rt, "user-alice")
	})
}

func TestCandidatePackageIntakeReviewRouteRejectsPendingOwnerRecord(t *testing.T) {
	t.Parallel()
	rt, handler := testAPISetup(t)
	srv := candidatePackageIntakeTestServer(handler)

	pending := createCandidatePackageIntakeForReview(t, srv, "user-alice", "intake-review-reject", map[string]any{
		"adoption_blockers_json": []string{
			"owner_review_not_recorded",
			"verification_pending",
		},
		"evidence_refs_json": []string{
			"texture://evidence/intake-review-reject/source",
		},
	})

	reviewW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+pending.IntakeID+"/review", candidatePackageIntakeReviewRequestBody(t, "reject", "texture://review/intake-review-reject/owner"), "user-alice")
	if reviewW.Code != http.StatusOK {
		t.Fatalf("review reject status = %d, want 200; body=%s", reviewW.Code, reviewW.Body.String())
	}
	var reviewed types.CandidatePackageIntakeRecord
	if err := json.Unmarshal(reviewW.Body.Bytes(), &reviewed); err != nil {
		t.Fatalf("decode reject review response: %v", err)
	}
	assertCandidatePackageIntakeReviewTransition(t, reviewed, pending, types.CandidatePackageIntakeRejected, types.CandidatePackageOwnerReviewRejected)
	assertCandidatePackageIntakeJSONStringArrayContains(t, "rejected intake evidence refs", reviewed.EvidenceRefsJSON, "texture://evidence/intake-review-reject/source")
	assertCandidatePackageIntakeJSONStringArrayContains(t, "rejected intake evidence refs", reviewed.EvidenceRefsJSON, "texture://review/intake-review-reject/owner")
	assertCandidatePackageIntakeJSONStringArrayContains(t, "rejected intake blockers", reviewed.AdoptionBlockersJSON, "owner_review_rejected")
	assertCandidatePackageIntakeJSONStringArrayContains(t, "rejected intake blockers", reviewed.AdoptionBlockersJSON, "verification_pending")

	assertNoCandidatePackageIntakePromotionSideEffects(t, rt, "user-alice")
}

func TestCandidatePackageIntakeReviewRouteRejectsUnsafeTransitions(t *testing.T) {
	t.Parallel()

	t.Run("another owner cannot review an owner-scoped pending intake", func(t *testing.T) {
		rt, handler := testAPISetup(t)
		srv := candidatePackageIntakeTestServer(handler)
		pending := createCandidatePackageIntakeForReview(t, srv, "user-alice", "intake-review-owner-scope", map[string]any{
			"adoption_blockers_json": []string{"owner_review_not_recorded"},
			"evidence_refs_json":     []string{"texture://evidence/intake-review-owner-scope/source"},
		})

		reviewW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+pending.IntakeID+"/review", candidatePackageIntakeReviewRequestBody(t, "approve", "texture://review/intake-review-owner-scope/bob"), "user-bob")
		if reviewW.Code != http.StatusNotFound {
			t.Fatalf("other owner review status = %d, want 404; body=%s", reviewW.Code, reviewW.Body.String())
		}
		assertCandidatePackageIntakeDetailMatchesPending(t, srv, pending, "user-alice")
		assertNoCandidatePackageIntakePromotionSideEffects(t, rt, "user-alice")
		assertNoCandidatePackageIntakePromotionSideEffects(t, rt, "user-bob")
	})

	t.Run("invalid decision leaves pending intake unchanged", func(t *testing.T) {
		rt, handler := testAPISetup(t)
		srv := candidatePackageIntakeTestServer(handler)
		pending := createCandidatePackageIntakeForReview(t, srv, "user-alice", "intake-review-invalid-decision", map[string]any{
			"adoption_blockers_json": []string{"owner_review_not_recorded", "verification_pending"},
			"evidence_refs_json":     []string{"texture://evidence/intake-review-invalid-decision/source"},
		})

		reviewW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+pending.IntakeID+"/review", candidatePackageIntakeReviewRequestBody(t, "defer", "texture://review/intake-review-invalid-decision/owner"), "user-alice")
		if reviewW.Code != http.StatusBadRequest {
			t.Fatalf("invalid decision status = %d, want 400; body=%s", reviewW.Code, reviewW.Body.String())
		}
		if !strings.Contains(reviewW.Body.String(), "decision must be approve or reject") {
			t.Fatalf("invalid decision body %q does not contain decision contract", reviewW.Body.String())
		}
		assertCandidatePackageIntakeDetailMatchesPending(t, srv, pending, "user-alice")
		assertNoCandidatePackageIntakePromotionSideEffects(t, rt, "user-alice")
	})

	t.Run("approved intake cannot be reviewed again", func(t *testing.T) {
		rt, handler := testAPISetup(t)
		srv := candidatePackageIntakeTestServer(handler)
		pending := createCandidatePackageIntakeForReview(t, srv, "user-alice", "intake-review-terminal-approved", map[string]any{
			"adoption_blockers_json": []string{"owner_review_not_recorded", "verification_pending"},
			"evidence_refs_json":     []string{"texture://evidence/intake-review-terminal-approved/source"},
		})
		approveW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+pending.IntakeID+"/review", candidatePackageIntakeReviewRequestBody(t, "approve", "texture://review/intake-review-terminal-approved/approved"), "user-alice")
		if approveW.Code != http.StatusOK {
			t.Fatalf("initial approve status = %d, want 200; body=%s", approveW.Code, approveW.Body.String())
		}

		rejectW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+pending.IntakeID+"/review", candidatePackageIntakeReviewRequestBody(t, "reject", "texture://review/intake-review-terminal-approved/rejected"), "user-alice")
		if rejectW.Code != http.StatusBadRequest {
			t.Fatalf("terminal reject-after-approve status = %d, want 400; body=%s", rejectW.Code, rejectW.Body.String())
		}
		if !strings.Contains(rejectW.Body.String(), "already terminal") {
			t.Fatalf("terminal reject-after-approve body %q does not contain terminal transition contract", rejectW.Body.String())
		}
		detail := candidatePackageIntakeDetail(t, srv, pending.IntakeID, "user-alice")
		assertCandidatePackageIntakeReviewTransition(t, detail, pending, types.CandidatePackageIntakeOwnerApproved, types.CandidatePackageOwnerReviewApproved)
		assertCandidatePackageIntakeJSONStringArrayContains(t, "approved terminal evidence refs", detail.EvidenceRefsJSON, "texture://review/intake-review-terminal-approved/approved")
		assertCandidatePackageIntakeJSONStringArrayNotContains(t, "approved terminal evidence refs", detail.EvidenceRefsJSON, "texture://review/intake-review-terminal-approved/rejected")
		assertCandidatePackageIntakeJSONStringArrayNotContains(t, "approved terminal blockers", detail.AdoptionBlockersJSON, "owner_review_rejected")
		assertNoCandidatePackageIntakePromotionSideEffects(t, rt, "user-alice")
	})

	t.Run("rejected intake cannot be reviewed again", func(t *testing.T) {
		rt, handler := testAPISetup(t)
		srv := candidatePackageIntakeTestServer(handler)
		pending := createCandidatePackageIntakeForReview(t, srv, "user-alice", "intake-review-terminal-rejected", map[string]any{
			"adoption_blockers_json": []string{"owner_review_not_recorded", "verification_pending"},
			"evidence_refs_json":     []string{"texture://evidence/intake-review-terminal-rejected/source"},
		})
		rejectW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+pending.IntakeID+"/review", candidatePackageIntakeReviewRequestBody(t, "reject", "texture://review/intake-review-terminal-rejected/rejected"), "user-alice")
		if rejectW.Code != http.StatusOK {
			t.Fatalf("initial reject status = %d, want 200; body=%s", rejectW.Code, rejectW.Body.String())
		}

		approveW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+pending.IntakeID+"/review", candidatePackageIntakeReviewRequestBody(t, "approve", "texture://review/intake-review-terminal-rejected/approved"), "user-alice")
		if approveW.Code != http.StatusBadRequest {
			t.Fatalf("terminal approve-after-reject status = %d, want 400; body=%s", approveW.Code, approveW.Body.String())
		}
		if !strings.Contains(approveW.Body.String(), "already terminal") {
			t.Fatalf("terminal approve-after-reject body %q does not contain terminal transition contract", approveW.Body.String())
		}
		detail := candidatePackageIntakeDetail(t, srv, pending.IntakeID, "user-alice")
		assertCandidatePackageIntakeReviewTransition(t, detail, pending, types.CandidatePackageIntakeRejected, types.CandidatePackageOwnerReviewRejected)
		assertCandidatePackageIntakeJSONStringArrayContains(t, "rejected terminal evidence refs", detail.EvidenceRefsJSON, "texture://review/intake-review-terminal-rejected/rejected")
		assertCandidatePackageIntakeJSONStringArrayNotContains(t, "rejected terminal evidence refs", detail.EvidenceRefsJSON, "texture://review/intake-review-terminal-rejected/approved")
		assertCandidatePackageIntakeJSONStringArrayContains(t, "rejected terminal blockers", detail.AdoptionBlockersJSON, "owner_review_rejected")
		assertNoCandidatePackageIntakePromotionSideEffects(t, rt, "user-alice")
	})
}

func TestCandidatePackageIntakeRoutesRejectUnauthenticatedAndUnsafeRequests(t *testing.T) {
	t.Parallel()
	rt, handler := testAPISetup(t)
	srv := candidatePackageIntakeTestServer(handler)

	for _, tc := range []struct {
		name       string
		owner      string
		body       string
		wantStatus int
		wantBody   string
	}{
		{
			name:       "missing auth",
			owner:      "",
			body:       candidatePackageIntakeRequestBody(t, map[string]any{"owner_id": "user-alice"}),
			wantStatus: http.StatusUnauthorized,
			wantBody:   "authentication required",
		},
		{
			name:       "body owner must match authenticated owner",
			owner:      "user-alice",
			body:       candidatePackageIntakeRequestBody(t, map[string]any{"owner_id": "user-bob"}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "owner_id does not match authenticated owner",
		},
		{
			name:       "adoption ready is rejected at intake boundary",
			owner:      "user-alice",
			body:       candidatePackageIntakeRequestBody(t, map[string]any{"owner_id": "user-alice", "adoption_ready": true}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "adoption_ready is not allowed at intake creation",
		},
		{
			name:  "approved adoption ready payload is rejected at intake boundary",
			owner: "user-alice",
			body: candidatePackageIntakeRequestBody(t, map[string]any{
				"owner_id":                "user-alice",
				"status":                  "owner_approved",
				"owner_review_state":      "approved",
				"owner_review_required":   false,
				"adoption_ready":          true,
				"adoption_blockers_json":  []string{},
				"verifier_contracts_json": []string{},
			}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "adoption_ready is not allowed at intake creation",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes", tc.body, tc.owner)
			if w.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", w.Code, tc.wantStatus, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), tc.wantBody) {
				t.Fatalf("body %q does not contain %q", w.Body.String(), tc.wantBody)
			}
		})
	}

	for _, ownerID := range []string{"user-alice", "user-bob"} {
		intakes, err := rt.store.ListCandidatePackageIntakes(context.Background(), ownerID, 10)
		if err != nil {
			t.Fatalf("list candidate package intakes for %s: %v", ownerID, err)
		}
		if len(intakes) != 0 {
			t.Fatalf("rejected requests persisted intakes for %s: %+v", ownerID, intakes)
		}
		assertNoCandidatePackageIntakePromotionSideEffects(t, rt, ownerID)
	}
}

func candidatePackageIntakeTestServer(handler *APIHandler) *choirserver.Server {
	srv := choirserver.NewServer("candidate-package-intake-api-test", "0")
	RegisterCandidatePackageIntakeRoutes(srv, handler)
	return srv
}

type candidatePackageIntakeDeployedRouteSnapshot struct {
	Intakes       []types.CandidatePackageIntakeRecord     `json:"intakes"`
	Acceptance    candidatePackageIntakeAcceptanceSnapshot `json:"acceptance"`
	TargetLineage types.ComputerSourceLineageRecord        `json:"target_lineage"`
}

func candidatePackageIntakeDeployedRouteSnapshotForFixture(t *testing.T, rt *Runtime, fixture candidatePackageIntakePromotionSwitchFixture) candidatePackageIntakeDeployedRouteSnapshot {
	t.Helper()
	ctx := context.Background()
	intakes, err := rt.store.ListCandidatePackageIntakes(ctx, fixture.Intake.OwnerID, 10)
	if err != nil {
		t.Fatalf("list candidate package intakes for %s: %v", fixture.Intake.OwnerID, err)
	}
	lineage, err := rt.store.GetComputerSourceLineage(ctx, fixture.Intake.OwnerID, fixture.TargetComputerID)
	if err != nil {
		t.Fatalf("get target source lineage for %s/%s: %v", fixture.Intake.OwnerID, fixture.TargetComputerID, err)
	}
	return candidatePackageIntakeDeployedRouteSnapshot{
		Intakes:       intakes,
		Acceptance:    candidatePackageIntakeAcceptanceSnapshotForOwner(t, rt, fixture.Intake.OwnerID),
		TargetLineage: lineage,
	}
}

func assertCandidatePackageIntakeDeployedRouteSnapshotUnchanged(t *testing.T, rt *Runtime, fixture candidatePackageIntakePromotionSwitchFixture, routeName string, before candidatePackageIntakeDeployedRouteSnapshot) {
	t.Helper()
	after := candidatePackageIntakeDeployedRouteSnapshotForFixture(t, rt, fixture)
	beforeJSON := candidatePackageIntakeMustMarshalJSON(t, before)
	afterJSON := candidatePackageIntakeMustMarshalJSON(t, after)
	if string(afterJSON) != string(beforeJSON) {
		t.Fatalf("%s mutated deployed candidate package state:\nbefore=%s\nafter=%s", routeName, beforeJSON, afterJSON)
	}
}

func serveCandidatePackageIntakeRequest(srv http.Handler, method, path, body, ownerID string) *httptest.ResponseRecorder {
	req := authenticatedRequest(method, path, body, ownerID)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	return w
}

func createCandidatePackageIntakeForReview(t *testing.T, srv *choirserver.Server, ownerID, intakeID string, overrides map[string]any) types.CandidatePackageIntakeRecord {
	t.Helper()
	body := map[string]any{
		"owner_id":                          ownerID,
		"intake_id":                         intakeID,
		"candidate_package_id":              "pkg-" + intakeID,
		"candidate_package_manifest_sha256": "sha256:" + intakeID,
		"source_computer_id":                "computer-" + intakeID,
		"source_candidate_id":               "candidate-" + intakeID,
		"candidate_source_ref":              "refs/candidates/" + intakeID,
		"intake_boundary":                   "owner review transition test boundary",
		"trace_id":                          "trace-" + intakeID,
	}
	for key, value := range overrides {
		body[key] = value
	}
	createW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes", candidatePackageIntakeRequestBody(t, body), ownerID)
	if createW.Code != http.StatusCreated {
		t.Fatalf("create review intake status = %d, want 201; body=%s", createW.Code, createW.Body.String())
	}
	var rec types.CandidatePackageIntakeRecord
	if err := json.Unmarshal(createW.Body.Bytes(), &rec); err != nil {
		t.Fatalf("decode created review intake: %v", err)
	}
	return rec
}

func candidatePackageIntakeReviewRequestBody(t *testing.T, decision, reviewEvidenceRef string) string {
	t.Helper()
	body := map[string]any{
		"decision":            decision,
		"review_evidence_ref": reviewEvidenceRef,
	}
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal candidate package intake review request: %v", err)
	}
	return string(data)
}

func candidatePackageIntakeAdoptionBoundaryRequestBody(t *testing.T, adoptionContractRef, rollbackContractRef, boundaryEvidenceRef string) string {
	t.Helper()
	body := map[string]any{
		"adoption_contract_ref": adoptionContractRef,
		"rollback_contract_ref": rollbackContractRef,
		"boundary_evidence_ref": boundaryEvidenceRef,
	}
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal candidate package intake adoption boundary request: %v", err)
	}
	return string(data)
}

func candidatePackageIntakePublicationDraftRequestBody(t *testing.T, overrides map[string]any) string {
	t.Helper()
	body := map[string]any{
		"publication_contract_ref": "texture://contracts/candidate-package-publication-draft/default",
		"draft_evidence_ref":       "texture://evidence/candidate-package-publication-draft/default",
	}
	for key, value := range overrides {
		body[key] = value
	}
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal candidate package intake publication draft request: %v", err)
	}
	return string(data)
}

func candidatePackageIntakeAdoptionReviewRequestBody(t *testing.T, overrides map[string]any) string {
	t.Helper()
	body := map[string]any{
		"target_computer_id":           "computer-candidate-package-adoption-review-target",
		"adoption_review_contract_ref": "texture://contracts/candidate-package-adoption-review/default",
	}
	for key, value := range overrides {
		body[key] = value
	}
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal candidate package intake adoption review request: %v", err)
	}
	return string(data)
}

func candidatePackageIntakeAdoptionReviewDecisionRequestBody(t *testing.T, decision, reviewEvidenceRef string) string {
	t.Helper()
	body := map[string]any{
		"decision":            decision,
		"review_evidence_ref": reviewEvidenceRef,
	}
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal candidate package intake adoption review decision request: %v", err)
	}
	return string(data)
}

func candidatePackageIntakePromotionSwitchRequestBody(t *testing.T, switchEvidenceRef string) string {
	t.Helper()
	body := map[string]any{
		"switch_evidence_ref": switchEvidenceRef,
	}
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal candidate package intake promotion switch request: %v", err)
	}
	return string(data)
}

func candidatePackageIntakePromotionSwitchRollbackRequestBody(t *testing.T, rollbackEvidenceRef string) string {
	t.Helper()
	body := map[string]any{
		"rollback_evidence_ref": rollbackEvidenceRef,
	}
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal candidate package intake promotion switch rollback request: %v", err)
	}
	return string(data)
}

func candidatePackageIntakePromotionSwitchRollForwardRequestBody(t *testing.T, rollForwardEvidenceRef string) string {
	t.Helper()
	body := map[string]any{
		"roll_forward_evidence_ref": rollForwardEvidenceRef,
	}
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal candidate package intake promotion switch roll-forward request: %v", err)
	}
	return string(data)
}

func rollForwardCandidatePackageIntakePromotionSwitchThroughRoute(t *testing.T, srv *choirserver.Server, fixture candidatePackageIntakePromotionSwitchFixture, adoptionID, rollForwardEvidenceRef string) types.AppAdoptionRecord {
	t.Helper()
	rollForwardW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+fixture.Intake.IntakeID+"/adoption-review/"+adoptionID+"/promotion-switch/roll-forward", candidatePackageIntakePromotionSwitchRollForwardRequestBody(t, rollForwardEvidenceRef), fixture.Intake.OwnerID)
	if rollForwardW.Code != http.StatusOK {
		t.Fatalf("promotion switch roll-forward fixture status = %d, want 200; body=%s", rollForwardW.Code, rollForwardW.Body.String())
	}
	var rolledForward types.AppAdoptionRecord
	if err := json.Unmarshal(rollForwardW.Body.Bytes(), &rolledForward); err != nil {
		t.Fatalf("decode promotion switch roll-forward fixture response: %v", err)
	}
	if rolledForward.AdoptionID != adoptionID || rolledForward.Status != types.AppAdoptionSourceLineageSwitched || rolledForward.TargetActiveSourceRefAtCutover != fixture.TargetActiveRef {
		t.Fatalf("promotion switch roll-forward fixture = id:%q status:%q cutover:%q, want id:%q status:%q cutover:%q", rolledForward.AdoptionID, rolledForward.Status, rolledForward.TargetActiveSourceRefAtCutover, adoptionID, types.AppAdoptionSourceLineageSwitched, fixture.TargetActiveRef)
	}
	assertCandidatePackageIntakeAdoptionHasNoDeployedPromotionFields(t, rolledForward)
	return rolledForward
}

func candidatePackageIntakePromotionSwitchReviewSurfacePath(intakeID, adoptionID string) string {
	return "/api/candidate-package-intakes/" + intakeID + "/adoption-review/" + adoptionID + "/promotion-switch/review-surface"
}

func candidatePackageIntakePromotionSwitchReviewSurfaceJSON(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var surface map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &surface); err != nil {
		t.Fatalf("decode promotion switch review surface: %v; body=%s", err, w.Body.String())
	}
	return surface
}

func assertCandidatePackageIntakePromotionSwitchReviewSurfaceAccepted(t *testing.T, surface map[string]any, fixture candidatePackageIntakePromotionSwitchFixture, approved types.AppAdoptionRecord) {
	t.Helper()
	assertCandidatePackageIntakePromotionSwitchAcceptanceCommon(t, surface, fixture, approved, types.AppAdoptionSourceLineageSwitched)
	for field, want := range map[string]string{
		"artifact_kind":               "candidate_package_adoption_promotion_review_surface",
		"state":                       "reviewable",
		"surface_scope":               "product_visible_non_deployed",
		"review_scope":                "non-deployed-candidate-package-source-lineage",
		"package_publication":         "blocked",
		"deployed_promotion":          "blocked",
		"deployed_route_mutation":     "blocked",
		"auth_session":                "unproven",
		"staging":                     "unproven",
		"vm_lifecycle":                "blocked",
		"run_acceptance_record":       "not_created",
		"promotion_level":             "not_claimed",
		"app_change_package_mutation": "not_created",
		"app_adoption_mutation":       "not_created",
	} {
		assertCandidatePackageIntakeEvidenceStringField(t, surface, field, want)
	}
	assertCandidatePackageIntakeReviewSurfaceAllowedActionsOnly(t, surface)
	assertCandidatePackageIntakeReviewSurfaceActivationDecisionBoundary(t, surface, approved)
	assertCandidatePackageIntakeReviewSurfaceReferencesAcceptedLineageEvidence(t, surface, approved)
	for _, want := range []string{
		"texture://evidence/" + fixture.Intake.IntakeID + "/switch",
		"texture://evidence/" + fixture.Intake.IntakeID + "/rollback",
		"texture://evidence/" + fixture.Intake.IntakeID + "/roll-forward",
	} {
		assertCandidatePackageIntakeEvidenceStringValue(t, surface, want)
	}
	assertCandidatePackageIntakeEvidenceStringValue(t, surface, "local-source-lineage-evidence is not deployed promotion-level acceptance")
	assertCandidatePackageIntakeEvidenceNoDeployedBuildFields(t, surface)
}

func assertCandidatePackageIntakeReviewSurfaceAllowedActionsOnly(t *testing.T, surface map[string]any) {
	t.Helper()
	rawActions, ok := surface["allowed_actions"].([]any)
	if !ok {
		t.Fatalf("promotion switch review surface missing allowed_actions array: %+v", surface)
	}
	seen := map[string]bool{}
	for _, raw := range rawActions {
		action, ok := raw.(string)
		if !ok {
			t.Fatalf("promotion switch review surface allowed action %#v is not a string; actions=%+v", raw, rawActions)
		}
		if action != "review" && action != "inspect" && action != "prepare_activation_decision" {
			t.Fatalf("promotion switch review surface allowed action %q is not an allowed read/decision-boundary action; actions=%+v", action, rawActions)
		}
		if seen[action] {
			t.Fatalf("promotion switch review surface allowed action %q is duplicated; actions=%+v", action, rawActions)
		}
		seen[action] = true
	}
	if len(seen) != 3 || !seen["review"] || !seen["inspect"] || !seen["prepare_activation_decision"] {
		t.Fatalf("promotion switch review surface allowed_actions = %+v, want exactly review, inspect, and prepare_activation_decision", rawActions)
	}
}

func assertCandidatePackageIntakeReviewSurfaceActivationDecisionBoundary(t *testing.T, surface map[string]any, approved types.AppAdoptionRecord) {
	t.Helper()
	raw, ok := surface["activation_decision_boundary"].(map[string]any)
	if !ok {
		t.Fatalf("promotion switch review surface missing activation_decision_boundary object: %+v", surface)
	}
	acceptanceID := "candidate-package-local-acceptance-" + approved.AdoptionID
	for field, want := range map[string]string{
		"state":                    "owner_decision_preparable",
		"prepared_action":          "prepare_activation_decision",
		"uses_local_acceptance_id": acceptanceID,
		"next_boundary":            "app_adoption_promotion_requires_separate_product_activation_contract",
	} {
		assertCandidatePackageIntakeEvidenceStringField(t, raw, field, want)
	}
	for field, want := range map[string]bool{
		"owner_controlled":             true,
		"requires_authenticated_owner": true,
		"no_mutation":                  true,
	} {
		got, ok := raw[field].(bool)
		if !ok || got != want {
			t.Fatalf("activation decision boundary %s = %#v, want %v; boundary=%+v", field, raw[field], want, raw)
		}
	}
	for _, want := range []string{
		"POST /api/adoptions/{adoption_id}/verify",
		"POST /api/adoptions/{adoption_id}/approve",
		"POST /api/adoptions/{adoption_id}/promote",
		"POST /api/candidate-package-intakes",
		"POST /api/candidate-package-intakes/{intake_id}/review",
		"POST /api/candidate-package-intakes/{intake_id}/adoption-boundary",
		"POST /api/candidate-package-intakes/{intake_id}/publication-draft",
		"POST /api/candidate-package-intakes/{intake_id}/adoption-review",
		"POST /api/candidate-package-intakes/{intake_id}/adoption-review/{adoption_id}",
		"POST /api/candidate-package-intakes/{intake_id}/adoption-review/{adoption_id}/promotion-switch",
		"POST /api/candidate-package-intakes/{intake_id}/adoption-review/{adoption_id}/promotion-switch/rollback",
		"POST /api/candidate-package-intakes/{intake_id}/adoption-review/{adoption_id}/promotion-switch/roll-forward",
		"POST /api/run-acceptances/synthesize",
		"DELETE /auth/sessions/{session_id}",
		"POST /auth/logout",
		"POST /api/staging/claims",
		"POST /api/vm/lifecycle",
	} {
		assertCandidatePackageIntakeStringArrayFieldContains(t, raw, "blocked_routes", want)
	}
	for _, want := range []string{
		"authenticated owner decision contract",
		"package publication contract",
		"AppAdoption mutation contract",
		"deployed route mutation contract",
		"staging identity contract",
		"VM lifecycle contract",
		"run-acceptance contract",
	} {
		assertCandidatePackageIntakeStringArrayFieldContains(t, raw, "required_contracts", want)
	}
}

func assertCandidatePackageIntakeStringArrayFieldContains(t *testing.T, value map[string]any, field, want string) {
	t.Helper()
	rawValues, ok := value[field].([]any)
	if !ok {
		t.Fatalf("activation decision boundary missing %s array: %+v", field, value)
	}
	for _, raw := range rawValues {
		got, ok := raw.(string)
		if !ok {
			t.Fatalf("activation decision boundary %s contains non-string %#v: %+v", field, raw, value)
		}
		if got == want {
			return
		}
	}
	t.Fatalf("activation decision boundary %s = %+v, want to contain %q", field, rawValues, want)
}

func assertCandidatePackageIntakeReviewSurfaceReferencesAcceptedLineageEvidence(t *testing.T, surface map[string]any, approved types.AppAdoptionRecord) {
	t.Helper()
	acceptanceID := "candidate-package-local-acceptance-" + approved.AdoptionID
	hasEmbeddedAcceptedEvidence := candidatePackageIntakeJSONValueContainsStringField(surface, "artifact_kind", "candidate_package_promotion_switch_acceptance_evidence") &&
		candidatePackageIntakeJSONValueContainsStringField(surface, "acceptance_id", acceptanceID) &&
		candidatePackageIntakeJSONValueContainsStringField(surface, "acceptance_level", "local-source-lineage-evidence") &&
		candidatePackageIntakeJSONValueContainsStringField(surface, "state", "accepted")
	hasAcceptedEvidenceRef := candidatePackageIntakeJSONValueContainsStringField(surface, "accepted_local_source_lineage_evidence_ref", acceptanceID) ||
		candidatePackageIntakeJSONValueContainsStringField(surface, "local_source_lineage_acceptance_id", acceptanceID) ||
		candidatePackageIntakeJSONValueContainsStringField(surface, "acceptance_id", acceptanceID)
	if !hasEmbeddedAcceptedEvidence && !hasAcceptedEvidenceRef {
		t.Fatalf("promotion switch review surface does not embed or reference accepted local-source-lineage evidence %q: %+v", acceptanceID, surface)
	}
}

func candidatePackageIntakePromotionSwitchAcceptancePath(intakeID, adoptionID string) string {
	return "/api/candidate-package-intakes/" + intakeID + "/adoption-review/" + adoptionID + "/promotion-switch/acceptance"
}

func candidatePackageIntakePromotionSwitchAcceptanceJSON(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var evidence map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &evidence); err != nil {
		t.Fatalf("decode promotion switch acceptance evidence: %v; body=%s", err, w.Body.String())
	}
	return evidence
}

func assertCandidatePackageIntakePromotionSwitchAcceptanceAccepted(t *testing.T, evidence map[string]any, fixture candidatePackageIntakePromotionSwitchFixture, approved types.AppAdoptionRecord) {
	t.Helper()
	assertCandidatePackageIntakePromotionSwitchAcceptanceCommon(t, evidence, fixture, approved, types.AppAdoptionSourceLineageSwitched)
	for field, want := range map[string]string{
		"artifact_kind":           "candidate_package_promotion_switch_acceptance_evidence",
		"acceptance_level":        "local-source-lineage-evidence",
		"state":                   "accepted",
		"evidence_scope":          "local_source_lineage",
		"review_scope":            "non-deployed-candidate-package-source-lineage",
		"package_publication":     "blocked",
		"deployed_promotion":      "blocked",
		"deployed_route_mutation": "blocked",
		"auth_session":            "unproven",
		"staging":                 "unproven",
		"vm_lifecycle":            "blocked",
		"run_acceptance_record":   "not_created",
		"promotion_level":         "not_claimed",
	} {
		assertCandidatePackageIntakeEvidenceStringField(t, evidence, field, want)
	}
	for field, want := range map[string]bool{
		"owner_review_approved":         true,
		"source_lineage_switched":       true,
		"source_lineage_rolled_back":    true,
		"source_lineage_roll_forwarded": true,
	} {
		assertCandidatePackageIntakeEvidenceTopBoolField(t, evidence, field, want)
	}
	for _, checkpoint := range []string{
		"owner_review_approved",
		"source_lineage_switched",
		"source_lineage_rolled_back",
		"source_lineage_roll_forwarded",
	} {
		assertCandidatePackageIntakeEvidenceCheckpoint(t, evidence, checkpoint, "verified")
	}
	assertCandidatePackageIntakeEvidenceStringValue(t, evidence, "local-source-lineage-evidence is not deployed promotion-level acceptance")
	assertCandidatePackageIntakeEvidenceNoDeployedBuildFields(t, evidence)
}

func assertCandidatePackageIntakePromotionSwitchAcceptanceBlocked(t *testing.T, evidence map[string]any, fixture candidatePackageIntakePromotionSwitchFixture, approved types.AppAdoptionRecord, currentStatus types.AppAdoptionStatus, switched, rolledBack, rolledForward bool) {
	t.Helper()
	assertCandidatePackageIntakePromotionSwitchAcceptanceCommon(t, evidence, fixture, approved, currentStatus)
	for field, want := range map[string]string{
		"artifact_kind":           "candidate_package_promotion_switch_acceptance_evidence",
		"acceptance_level":        "local-source-lineage",
		"state":                   "blocked",
		"evidence_scope":          "local_source_lineage",
		"review_scope":            "non-deployed-candidate-package-source-lineage",
		"package_publication":     "blocked",
		"deployed_promotion":      "blocked",
		"deployed_route_mutation": "blocked",
		"auth_session":            "unproven",
		"staging":                 "unproven",
		"vm_lifecycle":            "blocked",
		"run_acceptance_record":   "not_created",
	} {
		assertCandidatePackageIntakeEvidenceStringField(t, evidence, field, want)
	}
	for field, want := range map[string]bool{
		"owner_review_approved":         true,
		"source_lineage_switched":       switched,
		"source_lineage_rolled_back":    rolledBack,
		"source_lineage_roll_forwarded": rolledForward,
	} {
		assertCandidatePackageIntakeEvidenceTopBoolField(t, evidence, field, want)
	}
	assertCandidatePackageIntakeEvidenceNoDeployedBuildFields(t, evidence)
}

func assertCandidatePackageIntakePromotionSwitchAcceptanceCommon(t *testing.T, evidence map[string]any, fixture candidatePackageIntakePromotionSwitchFixture, approved types.AppAdoptionRecord, currentStatus types.AppAdoptionStatus) {
	t.Helper()
	for field, want := range map[string]string{
		"intake_id":                  fixture.Intake.IntakeID,
		"adoption_id":                approved.AdoptionID,
		"package_id":                 fixture.Draft.PackageID,
		"app_id":                     fixture.AppID,
		"target_computer_id":         fixture.TargetComputerID,
		"candidate_source_ref":       approved.CandidateSourceRef,
		"previous_active_source_ref": fixture.TargetActiveRef,
		"current_adoption_status":    string(currentStatus),
	} {
		assertCandidatePackageIntakeEvidenceStringField(t, evidence, field, want)
	}
}

func assertCandidatePackageIntakeEvidenceStringField(t *testing.T, evidence map[string]any, field, want string) {
	t.Helper()
	if !candidatePackageIntakeJSONValueContainsStringField(evidence, field, want) {
		t.Fatalf("promotion switch acceptance evidence missing string field %q=%q: %+v", field, want, evidence)
	}
}

func assertCandidatePackageIntakeEvidenceTopBoolField(t *testing.T, evidence map[string]any, field string, want bool) {
	t.Helper()
	got, ok := evidence[field].(bool)
	if !ok || got != want {
		t.Fatalf("promotion switch acceptance evidence[%q] = %#v, want %v; evidence=%+v", field, evidence[field], want, evidence)
	}
}

func assertCandidatePackageIntakeEvidenceCheckpoint(t *testing.T, evidence map[string]any, kind, state string) {
	t.Helper()
	rawCheckpoints, ok := evidence["checkpoints"].([]any)
	if !ok {
		t.Fatalf("promotion switch acceptance evidence checkpoints missing or not an array: %+v", evidence)
	}
	for _, raw := range rawCheckpoints {
		checkpoint, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if gotKind, _ := checkpoint["kind"].(string); gotKind != kind {
			continue
		}
		if gotState, _ := checkpoint["state"].(string); gotState != state {
			t.Fatalf("promotion switch acceptance checkpoint %q state = %#v, want %q; checkpoints=%+v", kind, checkpoint["state"], state, rawCheckpoints)
		}
		return
	}
	t.Fatalf("promotion switch acceptance evidence missing checkpoint %q=%q: %+v", kind, state, evidence)
}

func assertCandidatePackageIntakeEvidenceStringValue(t *testing.T, evidence map[string]any, want string) {
	t.Helper()
	if !candidatePackageIntakeJSONValueContainsString(evidence, want) {
		t.Fatalf("promotion switch acceptance evidence missing string value %q: %+v", want, evidence)
	}
}

func candidatePackageIntakeJSONValueContainsString(value any, want string) bool {
	switch typed := value.(type) {
	case string:
		return typed == want
	case map[string]any:
		for _, child := range typed {
			if candidatePackageIntakeJSONValueContainsString(child, want) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if candidatePackageIntakeJSONValueContainsString(child, want) {
				return true
			}
		}
	}
	return false
}

func assertCandidatePackageIntakeEvidenceNoDeployedBuildFields(t *testing.T, evidence map[string]any) {
	t.Helper()
	for _, field := range []string{"runtime_artifact_digest", "ui_artifact_digest", "foreground_tail_merge_result", "merge_strategy", "staging_url", "deploy_run_id"} {
		if got, ok := candidatePackageIntakeJSONValueStringField(evidence, field); ok && got != "" {
			t.Fatalf("promotion switch acceptance evidence contains deployed promotion/build field %q=%q: %+v", field, got, evidence)
		}
	}
}

func candidatePackageIntakeJSONValueStringField(value any, field string) (string, bool) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if key == field {
				got, ok := child.(string)
				return got, ok
			}
			if got, ok := candidatePackageIntakeJSONValueStringField(child, field); ok {
				return got, true
			}
		}
	case []any:
		for _, child := range typed {
			if got, ok := candidatePackageIntakeJSONValueStringField(child, field); ok {
				return got, true
			}
		}
	}
	return "", false
}

type candidatePackageIntakeAcceptanceSnapshot struct {
	Packages       []types.AppChangePackageRecord `json:"packages"`
	Adoptions      []types.AppAdoptionRecord      `json:"adoptions"`
	RunAcceptances []types.RunAcceptanceRecord    `json:"run_acceptances"`
}

func candidatePackageIntakeAcceptanceSnapshotForOwner(t *testing.T, rt *Runtime, ownerID string) candidatePackageIntakeAcceptanceSnapshot {
	t.Helper()
	ctx := context.Background()
	packages, err := rt.store.ListAppChangePackages(ctx, ownerID, 10)
	if err != nil {
		t.Fatalf("list app change packages for %s: %v", ownerID, err)
	}
	adoptions, err := rt.store.ListAppAdoptions(ctx, ownerID, 10)
	if err != nil {
		t.Fatalf("list app adoptions for %s: %v", ownerID, err)
	}
	runAcceptances, err := rt.store.ListRunAcceptances(ctx, ownerID, 10)
	if err != nil {
		t.Fatalf("list run acceptances for %s: %v", ownerID, err)
	}
	return candidatePackageIntakeAcceptanceSnapshot{Packages: packages, Adoptions: adoptions, RunAcceptances: runAcceptances}
}

func assertCandidatePackageIntakeAcceptanceSnapshotUnchanged(t *testing.T, rt *Runtime, ownerID string, before candidatePackageIntakeAcceptanceSnapshot) {
	t.Helper()
	after := candidatePackageIntakeAcceptanceSnapshotForOwner(t, rt, ownerID)
	beforeJSON := candidatePackageIntakeMustMarshalJSON(t, before)
	afterJSON := candidatePackageIntakeMustMarshalJSON(t, after)
	if string(afterJSON) != string(beforeJSON) {
		t.Fatalf("promotion switch acceptance evidence mutated candidate package state for %s:\nbefore=%s\nafter=%s", ownerID, beforeJSON, afterJSON)
	}
}

func assertCandidatePackageIntakeAcceptanceSnapshotHasOnlyLocalDraftAndAdoption(t *testing.T, snapshot candidatePackageIntakeAcceptanceSnapshot, fixture candidatePackageIntakePromotionSwitchFixture, adoptionID string) {
	t.Helper()
	if len(snapshot.Packages) != 1 || snapshot.Packages[0].PackageID != fixture.Draft.PackageID || snapshot.Packages[0].Status != types.AppChangePackageDraft {
		t.Fatalf("acceptance evidence snapshot packages = %+v, want only draft package %q", snapshot.Packages, fixture.Draft.PackageID)
	}
	if len(snapshot.Adoptions) != 1 || snapshot.Adoptions[0].AdoptionID != adoptionID {
		t.Fatalf("acceptance evidence snapshot adoptions = %+v, want only adoption %q", snapshot.Adoptions, adoptionID)
	}
	assertCandidatePackageIntakeAdoptionHasNoDeployedPromotionFields(t, snapshot.Adoptions[0])
	if len(snapshot.RunAcceptances) != 0 {
		t.Fatalf("acceptance evidence created run acceptance records: %+v", snapshot.RunAcceptances)
	}
}

type candidatePackageIntakePromotionSwitchFixture struct {
	Intake                    types.CandidatePackageIntakeRecord
	Draft                     types.AppChangePackageRecord
	Adoption                  types.AppAdoptionRecord
	AppID                     string
	TargetComputerID          string
	TargetActiveRef           string
	CandidateSourceRef        string
	PublicationContractRef    string
	DraftEvidenceRef          string
	AdoptionReviewContractRef string
}

func createCandidatePackageIntakePromotionSwitchReviewThroughRoute(t *testing.T, rt *Runtime, srv *choirserver.Server, ownerID, intakeID string) candidatePackageIntakePromotionSwitchFixture {
	t.Helper()
	appID := "candidate-" + intakeID + "-app"
	targetComputerID := "computer-" + intakeID + "-target"
	targetActiveRef := "refs/computers/" + targetComputerID + "/active-before-switch"
	targetCandidateID := "candidate-" + intakeID + "-target"
	candidateSourceRef := "refs/computers/" + targetComputerID + "/candidates/" + targetCandidateID
	publicationContractRef := "texture://contracts/" + intakeID + "/publication"
	draftEvidenceRef := "texture://evidence/" + intakeID + "/draft"
	adoptionReviewContractRef := "texture://contracts/" + intakeID + "/adoption-review"
	if _, err := rt.promotion.EnsureComputerSourceLineage(context.Background(), ownerID, targetComputerID, "workspace", targetActiveRef); err != nil {
		t.Fatalf("seed promotion switch target source lineage: %v", err)
	}
	ready := createCandidatePackageIntakeReadyForPublication(t, srv, ownerID, intakeID, map[string]any{
		"evidence_refs_json": []string{"texture://evidence/" + intakeID + "/source"},
	})
	draft := createCandidatePackageIntakePublicationDraftThroughRoute(t, srv, ready, ownerID, appID, publicationContractRef, draftEvidenceRef)
	createW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+ready.IntakeID+"/adoption-review", candidatePackageIntakeAdoptionReviewRequestBody(t, map[string]any{
		"target_computer_id":           targetComputerID,
		"target_candidate_id":          targetCandidateID,
		"candidate_source_ref":         candidateSourceRef,
		"adoption_review_contract_ref": adoptionReviewContractRef,
		"review_evidence_ref":          "texture://review/" + intakeID + "/adoption-pending",
	}), ownerID)
	if createW.Code != http.StatusCreated {
		t.Fatalf("promotion switch fixture adoption review status = %d, want 201; body=%s", createW.Code, createW.Body.String())
	}
	var pending types.AppAdoptionRecord
	if err := json.Unmarshal(createW.Body.Bytes(), &pending); err != nil {
		t.Fatalf("decode promotion switch fixture adoption review: %v", err)
	}
	assertCandidatePackageIntakeAdoptionReviewRecord(t, pending, ready, draft, targetComputerID, "owner_review_pending", targetActiveRef, adoptionReviewContractRef)
	return candidatePackageIntakePromotionSwitchFixture{
		Intake:                    ready,
		Draft:                     draft,
		Adoption:                  pending,
		AppID:                     appID,
		TargetComputerID:          targetComputerID,
		TargetActiveRef:           targetActiveRef,
		CandidateSourceRef:        candidateSourceRef,
		PublicationContractRef:    publicationContractRef,
		DraftEvidenceRef:          draftEvidenceRef,
		AdoptionReviewContractRef: adoptionReviewContractRef,
	}
}

func approveCandidatePackageIntakePromotionSwitchReviewThroughRoute(t *testing.T, srv *choirserver.Server, fixture candidatePackageIntakePromotionSwitchFixture) types.AppAdoptionRecord {
	t.Helper()
	approveW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+fixture.Intake.IntakeID+"/adoption-review/"+fixture.Adoption.AdoptionID, candidatePackageIntakeAdoptionReviewDecisionRequestBody(t, "approve", "texture://review/"+fixture.Intake.IntakeID+"/adoption-approve"), fixture.Intake.OwnerID)
	if approveW.Code != http.StatusOK {
		t.Fatalf("approve before promotion switch status = %d, want 200; body=%s", approveW.Code, approveW.Body.String())
	}
	var approved types.AppAdoptionRecord
	if err := json.Unmarshal(approveW.Body.Bytes(), &approved); err != nil {
		t.Fatalf("decode approved promotion switch review: %v", err)
	}
	if approved.AdoptionID != fixture.Adoption.AdoptionID || approved.Status != types.AppAdoptionOwnerReviewApproved || approved.CandidateSourceRef != fixture.CandidateSourceRef {
		t.Fatalf("approved promotion switch review = id:%q status:%q candidate:%q, want id:%q status:%q candidate:%q", approved.AdoptionID, approved.Status, approved.CandidateSourceRef, fixture.Adoption.AdoptionID, types.AppAdoptionOwnerReviewApproved, fixture.CandidateSourceRef)
	}
	return approved
}

func switchCandidatePackageIntakePromotionSwitchThroughRoute(t *testing.T, srv *choirserver.Server, fixture candidatePackageIntakePromotionSwitchFixture, adoption types.AppAdoptionRecord, switchEvidenceRef string) types.AppAdoptionRecord {
	t.Helper()
	switchW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+fixture.Intake.IntakeID+"/adoption-review/"+adoption.AdoptionID+"/promotion-switch", candidatePackageIntakePromotionSwitchRequestBody(t, switchEvidenceRef), fixture.Intake.OwnerID)
	if switchW.Code != http.StatusOK {
		t.Fatalf("promotion switch fixture status = %d, want 200; body=%s", switchW.Code, switchW.Body.String())
	}
	var switched types.AppAdoptionRecord
	if err := json.Unmarshal(switchW.Body.Bytes(), &switched); err != nil {
		t.Fatalf("decode promotion switch fixture response: %v", err)
	}
	if switched.AdoptionID != adoption.AdoptionID || switched.Status != types.AppAdoptionSourceLineageSwitched || switched.CandidateSourceRef != adoption.CandidateSourceRef || switched.TargetActiveSourceRefAtCutover != fixture.TargetActiveRef {
		t.Fatalf("promotion switch fixture = id:%q status:%q candidate:%q cutover:%q, want id:%q status:%q candidate:%q cutover:%q", switched.AdoptionID, switched.Status, switched.CandidateSourceRef, switched.TargetActiveSourceRefAtCutover, adoption.AdoptionID, types.AppAdoptionSourceLineageSwitched, adoption.CandidateSourceRef, fixture.TargetActiveRef)
	}
	assertCandidatePackageIntakeAdoptionHasNoDeployedPromotionFields(t, switched)
	return switched
}

func rollbackCandidatePackageIntakePromotionSwitchThroughRoute(t *testing.T, srv *choirserver.Server, fixture candidatePackageIntakePromotionSwitchFixture, adoptionID string) types.AppAdoptionRecord {
	t.Helper()
	rollbackW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+fixture.Intake.IntakeID+"/adoption-review/"+adoptionID+"/promotion-switch/rollback", candidatePackageIntakePromotionSwitchRollbackRequestBody(t, "texture://evidence/"+fixture.Intake.IntakeID+"/rollback"), fixture.Intake.OwnerID)
	if rollbackW.Code != http.StatusOK {
		t.Fatalf("promotion switch rollback fixture status = %d, want 200; body=%s", rollbackW.Code, rollbackW.Body.String())
	}
	var rolledBack types.AppAdoptionRecord
	if err := json.Unmarshal(rollbackW.Body.Bytes(), &rolledBack); err != nil {
		t.Fatalf("decode promotion switch rollback fixture response: %v", err)
	}
	if rolledBack.AdoptionID != adoptionID || rolledBack.Status != types.AppAdoptionRolledBack || rolledBack.TargetActiveSourceRefAtCutover != fixture.TargetActiveRef {
		t.Fatalf("promotion switch rollback fixture = id:%q status:%q cutover:%q, want id:%q status:%q cutover:%q", rolledBack.AdoptionID, rolledBack.Status, rolledBack.TargetActiveSourceRefAtCutover, adoptionID, types.AppAdoptionRolledBack, fixture.TargetActiveRef)
	}
	assertCandidatePackageIntakeAdoptionHasNoDeployedPromotionFields(t, rolledBack)
	return rolledBack
}

func createCandidatePackageIntakePublicationDraftThroughRoute(t *testing.T, srv *choirserver.Server, intake types.CandidatePackageIntakeRecord, ownerID, appID, publicationContractRef, draftEvidenceRef string) types.AppChangePackageRecord {
	t.Helper()
	draftW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+intake.IntakeID+"/publication-draft", candidatePackageIntakePublicationDraftRequestBody(t, map[string]any{
		"app_id":                   appID,
		"publication_contract_ref": publicationContractRef,
		"draft_evidence_ref":       draftEvidenceRef,
	}), ownerID)
	if draftW.Code != http.StatusCreated {
		t.Fatalf("publication draft fixture status = %d, want 201; body=%s", draftW.Code, draftW.Body.String())
	}
	var draft types.AppChangePackageRecord
	if err := json.Unmarshal(draftW.Body.Bytes(), &draft); err != nil {
		t.Fatalf("decode publication draft fixture: %v", err)
	}
	return draft
}

func upsertCandidatePackageIntakePublicationDraftFixture(t *testing.T, rt *Runtime, intake types.CandidatePackageIntakeRecord, appID string, status types.AppChangePackageStatus, visibility string, manifestOverrides map[string]any) types.AppChangePackageRecord {
	t.Helper()
	if visibility == "" {
		visibility = "private"
	}
	adoptionContractRef := "texture://contracts/" + intake.IntakeID + "/adoption"
	rollbackContractRef := "texture://contracts/" + intake.IntakeID + "/rollback"
	manifest := map[string]any{
		"kind":                              "candidate_package_publication_draft",
		"package_id":                        intake.CandidatePackageID,
		"app_id":                            appID,
		"owner_id":                          intake.OwnerID,
		"candidate_package_intake_id":       intake.IntakeID,
		"candidate_package_id":              intake.CandidatePackageID,
		"candidate_package_manifest_sha256": intake.CandidatePackageManifestSHA256,
		"source_computer_id":                intake.SourceComputerID,
		"source_candidate_id":               intake.SourceCandidateID,
		"candidate_source_ref":              intake.CandidateSourceRef,
		"intake_boundary":                   intake.IntakeBoundary,
		"publication_contract_ref":          "texture://contracts/" + intake.IntakeID + "/publication",
		"adoption_contract_ref":             adoptionContractRef,
		"rollback_contract_ref":             rollbackContractRef,
		"direct_app_change_package_publish": "blocked",
		"app_adoption_creation":             "blocked",
		"promotion":                         "blocked",
		"deployed_route_mutation":           "blocked",
		"vm_lifecycle":                      "blocked",
		"adoption_ready":                    intake.AdoptionReady,
	}
	for key, value := range manifestOverrides {
		manifest[key] = value
	}
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal publication draft fixture manifest: %v", err)
	}
	manifestSHA, _ := manifest["candidate_package_manifest_sha256"].(string)
	if manifestSHA == "" {
		manifestSHA = intake.CandidatePackageManifestSHA256
	}
	rec := types.AppChangePackageRecord{
		PackageID:             intake.CandidatePackageID,
		OwnerID:               intake.OwnerID,
		AppID:                 appID,
		Status:                status,
		Visibility:            visibility,
		SourceComputerID:      intake.SourceComputerID,
		SourceCandidateID:     intake.SourceCandidateID,
		CandidateSourceRef:    intake.CandidateSourceRef,
		PackageManifestSHA256: manifestSHA,
		ManifestJSON:          manifestJSON,
		VerifierContractsJSON: json.RawMessage(`[]`),
		ProvenanceRefsJSON:    json.RawMessage(`[]`),
		TraceID:               intake.TraceID,
	}
	if rec.Status == "" {
		rec.Status = types.AppChangePackageDraft
	}
	got, err := rt.store.UpsertAppChangePackage(context.Background(), rec)
	if err != nil {
		t.Fatalf("upsert publication draft fixture: %v", err)
	}
	return got
}

func assertCandidatePackageIntakeAdoptionReviewRecord(t *testing.T, got types.AppAdoptionRecord, intake types.CandidatePackageIntakeRecord, draft types.AppChangePackageRecord, targetComputerID, wantStatus, targetActiveRef, adoptionReviewContractRef string) {
	t.Helper()
	if got.AdoptionID == "" {
		t.Fatal("adoption review adoption_id is empty")
	}
	if got.OwnerID != intake.OwnerID || got.PackageID != draft.PackageID || got.AppID != draft.AppID {
		t.Fatalf("adoption review package provenance = owner:%q package:%q app:%q, want owner:%q package:%q app:%q", got.OwnerID, got.PackageID, got.AppID, intake.OwnerID, draft.PackageID, draft.AppID)
	}
	if got.TargetComputerID != targetComputerID {
		t.Fatalf("adoption review target_computer_id = %q, want %q", got.TargetComputerID, targetComputerID)
	}
	if got.TargetActiveSourceRefAtCandidateStart != targetActiveRef || got.TargetActiveSourceRefAtCutover != "" {
		t.Fatalf("adoption review target lineage refs = start:%q cutover:%q, want start:%q and no cutover", got.TargetActiveSourceRefAtCandidateStart, got.TargetActiveSourceRefAtCutover, targetActiveRef)
	}
	if got.TargetCandidateID == "" || got.CandidateSourceRef == "" || !strings.Contains(got.CandidateSourceRef, targetComputerID) || !strings.Contains(got.CandidateSourceRef, got.TargetCandidateID) {
		t.Fatalf("adoption review target candidate provenance = candidate_id:%q candidate_ref:%q, want target candidate ref for %q", got.TargetCandidateID, got.CandidateSourceRef, targetComputerID)
	}
	if got.TraceID != intake.TraceID {
		t.Fatalf("adoption review trace_id = %q, want %q", got.TraceID, intake.TraceID)
	}
	if string(got.Status) != wantStatus {
		t.Fatalf("adoption review status = %q, want %q", got.Status, wantStatus)
	}
	if got.RuntimeArtifactDigest != "" || got.UIArtifactDigest != "" || got.TargetActiveSourceRefAtCutover != "" || got.ForegroundTailMergeResult != "" || got.MergeStrategy != "" {
		t.Fatalf("adoption review created promotion/build side effects: %+v", got)
	}
	candidatePackageIntakeJSONContainsStringField(t, "adoption review verifier_results_json", got.VerifierResultsJSON, "candidate_package_intake_id", intake.IntakeID)
	candidatePackageIntakeJSONContainsStringField(t, "adoption review verifier_results_json", got.VerifierResultsJSON, "package_id", draft.PackageID)
	candidatePackageIntakeJSONContainsStringField(t, "adoption review verifier_results_json", got.VerifierResultsJSON, "adoption_review_contract_ref", adoptionReviewContractRef)
}

func assertCandidatePackageIntakeAdoptionReviewContracts(t *testing.T, got types.AppAdoptionRecord, intake types.CandidatePackageIntakeRecord, adoptionReviewContractRef string) {
	t.Helper()
	candidatePackageIntakeJSONContainsStringField(t, "adoption review verifier_results_json", got.VerifierResultsJSON, "adoption_review_contract_ref", adoptionReviewContractRef)
	candidatePackageIntakeJSONContainsStringField(t, "adoption review rollback_profile_json", got.RollbackProfileJSON, "rollback_contract_ref", "texture://contracts/"+intake.IntakeID+"/rollback")
	candidatePackageIntakeJSONContainsStringField(t, "adoption review rollback_profile_json", got.RollbackProfileJSON, "adoption_review_contract_ref", adoptionReviewContractRef)
}

func assertCandidatePackageIntakeSingleDraftPackage(t *testing.T, rt *Runtime, intake types.CandidatePackageIntakeRecord, appID, publicationContractRef, draftEvidenceRef string) {
	t.Helper()
	packages, err := rt.store.ListAppChangePackages(context.Background(), intake.OwnerID, 10)
	if err != nil {
		t.Fatalf("list app change packages for %s: %v", intake.OwnerID, err)
	}
	if len(packages) != 1 {
		t.Fatalf("app change packages = %+v, want exactly one candidate publication draft", packages)
	}
	assertCandidatePackageIntakePublicationDraftPackage(t, packages[0], intake, appID, publicationContractRef, draftEvidenceRef)
}

func assertCandidatePackageIntakePackagesUnchanged(t *testing.T, rt *Runtime, ownerID string, before []types.AppChangePackageRecord) {
	t.Helper()
	after, err := rt.store.ListAppChangePackages(context.Background(), ownerID, 10)
	if err != nil {
		t.Fatalf("list app change packages for %s: %v", ownerID, err)
	}
	beforeJSON := candidatePackageIntakeMustMarshalJSON(t, before)
	afterJSON := candidatePackageIntakeMustMarshalJSON(t, after)
	if string(afterJSON) != string(beforeJSON) {
		t.Fatalf("app change packages changed for %s:\nbefore=%s\nafter=%s", ownerID, beforeJSON, afterJSON)
	}
}

func assertCandidatePackageIntakeTargetLineageActiveRef(t *testing.T, rt *Runtime, ownerID, targetComputerID, wantActiveRef string) {
	t.Helper()
	lineage, err := rt.store.GetComputerSourceLineage(context.Background(), ownerID, targetComputerID)
	if err != nil {
		t.Fatalf("get target source lineage for %s/%s: %v", ownerID, targetComputerID, err)
	}
	if lineage.ActiveSourceRef != wantActiveRef {
		t.Fatalf("target source lineage active ref = %q, want %q; lineage=%+v", lineage.ActiveSourceRef, wantActiveRef, lineage)
	}
}

func assertCandidatePackageIntakeTargetLineageNonDeployedActiveRef(t *testing.T, rt *Runtime, ownerID, targetComputerID, wantActiveRef string) {
	t.Helper()
	lineage, err := rt.store.GetComputerSourceLineage(context.Background(), ownerID, targetComputerID)
	if err != nil {
		t.Fatalf("get target source lineage for %s/%s: %v", ownerID, targetComputerID, err)
	}
	if lineage.ActiveSourceRef != wantActiveRef {
		t.Fatalf("target source lineage active ref = %q, want %q; lineage=%+v", lineage.ActiveSourceRef, wantActiveRef, lineage)
	}
	if lineage.RuntimeDigest != "" || lineage.UIDigest != "" {
		t.Fatalf("target source lineage gained deployed artifact digests during promotion switch rollback boundary: %+v", lineage)
	}
}

func assertCandidatePackageIntakeTargetLineageSwitch(t *testing.T, rt *Runtime, ownerID, targetComputerID, wantActiveRef, wantAdoptionID, wantPackageID, wantCandidateRef string) {
	t.Helper()
	lineage, err := rt.store.GetComputerSourceLineage(context.Background(), ownerID, targetComputerID)
	if err != nil {
		t.Fatalf("get target source lineage for %s/%s: %v", ownerID, targetComputerID, err)
	}
	if lineage.ActiveSourceRef != wantActiveRef || lineage.LastAdoptionID != wantAdoptionID || lineage.LastPackageID != wantPackageID || lineage.LastCandidateRef != wantCandidateRef {
		t.Fatalf("target source lineage = active:%q adoption:%q package:%q candidate:%q, want active:%q adoption:%q package:%q candidate:%q; lineage=%+v", lineage.ActiveSourceRef, lineage.LastAdoptionID, lineage.LastPackageID, lineage.LastCandidateRef, wantActiveRef, wantAdoptionID, wantPackageID, wantCandidateRef, lineage)
	}
	if lineage.RuntimeDigest != "" || lineage.UIDigest != "" {
		t.Fatalf("target source lineage gained deployed artifact digests during promotion switch boundary: %+v", lineage)
	}
}

func assertCandidatePackageIntakeAdoptionStatus(t *testing.T, rt *Runtime, ownerID, adoptionID string, wantStatus types.AppAdoptionStatus, wantCutoverRef string) types.AppAdoptionRecord {
	t.Helper()
	adoptions := assertCandidatePackageIntakeAdoptionCount(t, rt, ownerID, 1)
	if adoptions[0].AdoptionID != adoptionID {
		t.Fatalf("app adoption for %s = %q, want only adoption %q; adoptions=%+v", ownerID, adoptions[0].AdoptionID, adoptionID, adoptions)
	}
	adoption, err := rt.store.GetAppAdoption(context.Background(), ownerID, adoptionID)
	if err != nil {
		t.Fatalf("get app adoption %s/%s: %v", ownerID, adoptionID, err)
	}
	if adoption.Status != wantStatus || adoption.TargetActiveSourceRefAtCutover != wantCutoverRef {
		t.Fatalf("app adoption %s = status:%q cutover:%q, want status:%q cutover:%q; adoption=%+v", adoptionID, adoption.Status, adoption.TargetActiveSourceRefAtCutover, wantStatus, wantCutoverRef, adoption)
	}
	if adoption.RuntimeArtifactDigest != "" || adoption.UIArtifactDigest != "" || adoption.ForegroundTailMergeResult != "" || adoption.MergeStrategy != "" {
		t.Fatalf("candidate package promotion switch adoption has deployed promotion/build fields: %+v", adoption)
	}
	return adoption
}

func assertCandidatePackageIntakeAdoptionHasNoDeployedPromotionFields(t *testing.T, adoption types.AppAdoptionRecord) {
	t.Helper()
	if adoption.RuntimeArtifactDigest != "" || adoption.UIArtifactDigest != "" || adoption.ForegroundTailMergeResult != "" || adoption.MergeStrategy != "" {
		t.Fatalf("candidate package promotion switch adoption has deployed promotion/build fields: %+v", adoption)
	}
}

func assertCandidatePackageIntakeAdoptionCount(t *testing.T, rt *Runtime, ownerID string, want int) []types.AppAdoptionRecord {
	t.Helper()
	adoptions, err := rt.store.ListAppAdoptions(context.Background(), ownerID, 10)
	if err != nil {
		t.Fatalf("list app adoptions for %s: %v", ownerID, err)
	}
	if len(adoptions) != want {
		t.Fatalf("app adoptions for %s = %+v, want %d", ownerID, adoptions, want)
	}
	return adoptions
}

func candidatePackageIntakeJSONContainsStringField(t *testing.T, name string, raw json.RawMessage, field, want string) {
	t.Helper()
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatalf("decode %s: %v; raw=%s", name, err, raw)
	}
	if !candidatePackageIntakeJSONValueContainsStringField(value, field, want) {
		t.Fatalf("%s = %s, want field %q with value %q", name, raw, field, want)
	}
}

func candidatePackageIntakeJSONValueContainsStringField(value any, field, want string) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if key == field {
				if got, ok := child.(string); ok && got == want {
					return true
				}
			}
			if candidatePackageIntakeJSONValueContainsStringField(child, field, want) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if candidatePackageIntakeJSONValueContainsStringField(child, field, want) {
				return true
			}
		}
	}
	return false
}

func createCandidatePackageIntakeReadyForPublication(t *testing.T, srv *choirserver.Server, ownerID, intakeID string, overrides map[string]any) types.CandidatePackageIntakeRecord {
	t.Helper()
	createOverrides := map[string]any{
		"adoption_blockers_json": []string{"owner_review_not_recorded"},
		"evidence_refs_json":     []string{"texture://evidence/" + intakeID + "/source"},
	}
	for key, value := range overrides {
		createOverrides[key] = value
	}
	pending := createCandidatePackageIntakeForReview(t, srv, ownerID, intakeID, createOverrides)
	approveW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+pending.IntakeID+"/review", candidatePackageIntakeReviewRequestBody(t, "approve", "texture://review/"+intakeID+"/owner"), ownerID)
	if approveW.Code != http.StatusOK {
		t.Fatalf("approve before publication draft status = %d, want 200; body=%s", approveW.Code, approveW.Body.String())
	}
	boundaryW := serveCandidatePackageIntakeRequest(srv, http.MethodPost, "/api/candidate-package-intakes/"+pending.IntakeID+"/adoption-boundary", candidatePackageIntakeAdoptionBoundaryRequestBody(t, "texture://contracts/"+intakeID+"/adoption", "texture://contracts/"+intakeID+"/rollback", "texture://evidence/"+intakeID+"/boundary"), ownerID)
	if boundaryW.Code != http.StatusOK {
		t.Fatalf("adoption boundary before publication draft status = %d, want 200; body=%s", boundaryW.Code, boundaryW.Body.String())
	}
	var ready types.CandidatePackageIntakeRecord
	if err := json.Unmarshal(boundaryW.Body.Bytes(), &ready); err != nil {
		t.Fatalf("decode adoption-ready intake: %v", err)
	}
	if !ready.AdoptionReady {
		t.Fatalf("publication draft fixture did not become adoption-ready: %+v", ready)
	}
	return ready
}

func assertCandidatePackageIntakePublicationDraftPackage(t *testing.T, got types.AppChangePackageRecord, intake types.CandidatePackageIntakeRecord, appID, publicationContractRef, draftEvidenceRef string) {
	t.Helper()
	adoptionContractRef := "texture://contracts/" + intake.IntakeID + "/adoption"
	rollbackContractRef := "texture://contracts/" + intake.IntakeID + "/rollback"
	assertCandidatePackageIntakeAdoptionBoundaryAcceptance(t, intake.AcceptanceJSON, adoptionContractRef, rollbackContractRef)
	if got.PackageID != intake.CandidatePackageID || got.OwnerID != intake.OwnerID || got.AppID != appID {
		t.Fatalf("publication draft identity = package:%q owner:%q app:%q, want package:%q owner:%q app:%q", got.PackageID, got.OwnerID, got.AppID, intake.CandidatePackageID, intake.OwnerID, appID)
	}
	if got.Status != types.AppChangePackageDraft || got.Visibility != "private" {
		t.Fatalf("publication draft state = status:%q visibility:%q, want draft private", got.Status, got.Visibility)
	}
	if got.SourceComputerID != intake.SourceComputerID || got.SourceCandidateID != intake.SourceCandidateID || got.CandidateSourceRef != intake.CandidateSourceRef || got.TraceID != intake.TraceID {
		t.Fatalf("publication draft provenance fields = computer:%q candidate:%q ref:%q trace:%q, want computer:%q candidate:%q ref:%q trace:%q", got.SourceComputerID, got.SourceCandidateID, got.CandidateSourceRef, got.TraceID, intake.SourceComputerID, intake.SourceCandidateID, intake.CandidateSourceRef, intake.TraceID)
	}
	manifest := candidatePackageIntakeJSONMap(t, "publication draft manifest", got.ManifestJSON)
	for field, want := range map[string]string{
		"kind":                              "candidate_package_publication_draft",
		"package_id":                        got.PackageID,
		"app_id":                            appID,
		"owner_id":                          intake.OwnerID,
		"candidate_package_intake_id":       intake.IntakeID,
		"candidate_package_id":              intake.CandidatePackageID,
		"candidate_package_manifest_sha256": intake.CandidatePackageManifestSHA256,
		"source_computer_id":                intake.SourceComputerID,
		"source_candidate_id":               intake.SourceCandidateID,
		"candidate_source_ref":              intake.CandidateSourceRef,
		"intake_boundary":                   intake.IntakeBoundary,
		"publication_contract_ref":          publicationContractRef,
		"adoption_contract_ref":             adoptionContractRef,
		"rollback_contract_ref":             rollbackContractRef,
		"direct_app_change_package_publish": "blocked",
		"app_adoption_creation":             "blocked",
		"promotion":                         "blocked",
		"deployed_route_mutation":           "blocked",
		"vm_lifecycle":                      "blocked",
	} {
		if value, _ := manifest[field].(string); value != want {
			t.Fatalf("publication draft manifest[%q] = %#v, want %q; manifest=%+v", field, manifest[field], want, manifest)
		}
	}
	for field, want := range map[string]bool{
		"adoption_ready":                       true,
		"requires_runtime_or_ui_source_delta":  true,
		"requires_adoption_consumer_follow_up": true,
	} {
		if value, _ := manifest[field].(bool); value != want {
			t.Fatalf("publication draft manifest[%q] = %#v, want %v; manifest=%+v", field, manifest[field], want, manifest)
		}
	}
	assertCandidatePackageIntakePublicationDraftContract(t, got.VerifierContractsJSON, "candidate-package-publication-draft", "draft_only", publicationContractRef, "blocked")
	assertCandidatePackageIntakePublicationDraftContract(t, got.VerifierContractsJSON, "candidate-package-adoption-boundary", "bound", adoptionContractRef, "")
	assertCandidatePackageIntakePublicationDraftContract(t, got.VerifierContractsJSON, "candidate-package-rollback-boundary", "bound", rollbackContractRef, "")
	for _, want := range []string{
		"texture://evidence/" + intake.IntakeID + "/source",
		"texture://review/" + intake.IntakeID + "/owner",
		"texture://evidence/" + intake.IntakeID + "/boundary",
		"candidate-package-intake:" + intake.IntakeID,
		draftEvidenceRef,
	} {
		assertCandidatePackageIntakeJSONStringArrayContains(t, "publication draft provenance refs", got.ProvenanceRefsJSON, want)
	}
}

func candidatePackageIntakeJSONMap(t *testing.T, name string, raw json.RawMessage) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode %s as object: %v; raw=%s", name, err, raw)
	}
	return out
}

func assertCandidatePackageIntakePublicationDraftContract(t *testing.T, raw json.RawMessage, name, state, contractRef, publish string) {
	t.Helper()
	var contracts []map[string]any
	if err := json.Unmarshal(raw, &contracts); err != nil {
		t.Fatalf("decode publication draft verifier_contracts_json: %v; raw=%s", err, raw)
	}
	for _, contract := range contracts {
		if gotName, _ := contract["name"].(string); gotName != name {
			continue
		}
		if gotState, _ := contract["state"].(string); gotState != state {
			t.Fatalf("verifier contract %q state = %#v, want %q; contracts=%+v", name, contract["state"], state, contracts)
		}
		if gotRef, _ := contract["contract_ref"].(string); gotRef != contractRef {
			t.Fatalf("verifier contract %q contract_ref = %#v, want %q; contracts=%+v", name, contract["contract_ref"], contractRef, contracts)
		}
		if publish != "" {
			if gotPublish, _ := contract["publish"].(string); gotPublish != publish {
				t.Fatalf("verifier contract %q publish = %#v, want %q; contracts=%+v", name, contract["publish"], publish, contracts)
			}
		}
		return
	}
	t.Fatalf("verifier_contracts_json missing contract %q: %s", name, raw)
}

type candidatePackageIntakePromotionSnapshot struct {
	Packages  []types.AppChangePackageRecord
	Adoptions []types.AppAdoptionRecord
}

func candidatePackageIntakePromotionSnapshotForOwner(t *testing.T, rt *Runtime, ownerID string) candidatePackageIntakePromotionSnapshot {
	t.Helper()
	ctx := context.Background()
	packages, err := rt.store.ListAppChangePackages(ctx, ownerID, 10)
	if err != nil {
		t.Fatalf("list app change packages for %s: %v", ownerID, err)
	}
	adoptions, err := rt.store.ListAppAdoptions(ctx, ownerID, 10)
	if err != nil {
		t.Fatalf("list app adoptions for %s: %v", ownerID, err)
	}
	return candidatePackageIntakePromotionSnapshot{Packages: packages, Adoptions: adoptions}
}

func assertCandidatePackageIntakePromotionSnapshotUnchanged(t *testing.T, rt *Runtime, ownerID string, before candidatePackageIntakePromotionSnapshot) {
	t.Helper()
	after := candidatePackageIntakePromotionSnapshotForOwner(t, rt, ownerID)
	beforeJSON := candidatePackageIntakeMustMarshalJSON(t, before)
	afterJSON := candidatePackageIntakeMustMarshalJSON(t, after)
	if string(afterJSON) != string(beforeJSON) {
		t.Fatalf("publication draft rejection changed promotion state for %s:\nbefore=%s\nafter=%s", ownerID, beforeJSON, afterJSON)
	}
}

func candidatePackageIntakeMustMarshalJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal candidate package intake assertion value: %v", err)
	}
	return data
}

func assertCandidatePackageIntakeAdoptionBoundaryPreservedRecord(t *testing.T, got, want types.CandidatePackageIntakeRecord) {
	t.Helper()
	if got.IntakeID != want.IntakeID || got.OwnerID != want.OwnerID || got.CandidatePackageID != want.CandidatePackageID || got.CandidatePackageManifestSHA256 != want.CandidatePackageManifestSHA256 {
		t.Fatalf("adoption boundary changed intake identity: got intake:%q owner:%q package:%q manifest:%q, want intake:%q owner:%q package:%q manifest:%q", got.IntakeID, got.OwnerID, got.CandidatePackageID, got.CandidatePackageManifestSHA256, want.IntakeID, want.OwnerID, want.CandidatePackageID, want.CandidatePackageManifestSHA256)
	}
	if got.SourceComputerID != want.SourceComputerID || got.SourceCandidateID != want.SourceCandidateID || got.CandidateSourceRef != want.CandidateSourceRef {
		t.Fatalf("adoption boundary changed source refs: got computer:%q candidate:%q ref:%q, want computer:%q candidate:%q ref:%q", got.SourceComputerID, got.SourceCandidateID, got.CandidateSourceRef, want.SourceComputerID, want.SourceCandidateID, want.CandidateSourceRef)
	}
	if got.IntakeBoundary != want.IntakeBoundary || got.TraceID != want.TraceID {
		t.Fatalf("adoption boundary changed evidence metadata: intake_boundary:%q trace_id:%q, want intake_boundary:%q trace_id:%q", got.IntakeBoundary, got.TraceID, want.IntakeBoundary, want.TraceID)
	}
	if got.Status != want.Status || got.OwnerReviewState != want.OwnerReviewState || got.OwnerReviewRequired != want.OwnerReviewRequired {
		t.Fatalf("adoption boundary changed review state: status:%q state:%q required:%v, want status:%q state:%q required:%v", got.Status, got.OwnerReviewState, got.OwnerReviewRequired, want.Status, want.OwnerReviewState, want.OwnerReviewRequired)
	}
}

func assertCandidatePackageIntakeAdoptionBoundaryAcceptance(t *testing.T, raw json.RawMessage, adoptionContractRef, rollbackContractRef string) {
	t.Helper()
	boundary := candidatePackageIntakeAdoptionBoundaryObject(t, raw)
	for _, field := range []string{
		"direct_app_change_package_publish",
		"app_adoption_creation",
		"promotion",
		"deployed_route_mutation",
		"vm_lifecycle",
	} {
		if got, _ := boundary[field].(string); got != "blocked" {
			t.Fatalf("acceptance adoption_rollback_boundary[%q] = %#v, want blocked; boundary=%+v", field, boundary[field], boundary)
		}
	}
	for field, want := range map[string]string{
		"status":                "bound",
		"adoption_contract_ref": adoptionContractRef,
		"rollback_contract_ref": rollbackContractRef,
	} {
		if got, _ := boundary[field].(string); got != want {
			t.Fatalf("acceptance adoption_rollback_boundary[%q] = %#v, want %q; boundary=%+v", field, boundary[field], want, boundary)
		}
	}
}

func assertCandidatePackageIntakeAcceptanceStringField(t *testing.T, raw json.RawMessage, field, want string) {
	t.Helper()
	var envelope map[string]any
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("decode acceptance_json: %v; raw=%s", err, raw)
	}
	if got, _ := envelope[field].(string); got != want {
		t.Fatalf("acceptance_json[%q] = %#v, want %q; raw=%s", field, envelope[field], want, raw)
	}
}

func assertCandidatePackageIntakeJSONNoAdoptionBoundary(t *testing.T, raw json.RawMessage) {
	t.Helper()
	var envelope map[string]any
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("decode acceptance_json: %v; raw=%s", err, raw)
	}
	if _, ok := envelope["adoption_rollback_boundary"]; ok {
		t.Fatalf("acceptance_json unexpectedly contains adoption_rollback_boundary: %s", raw)
	}
}

func candidatePackageIntakeAdoptionBoundaryObject(t *testing.T, raw json.RawMessage) map[string]any {
	t.Helper()
	var envelope map[string]any
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("decode acceptance_json: %v; raw=%s", err, raw)
	}
	boundary, ok := envelope["adoption_rollback_boundary"].(map[string]any)
	if !ok {
		t.Fatalf("acceptance_json missing object adoption_rollback_boundary: %s", raw)
	}
	return boundary
}

func candidatePackageIntakeDetail(t *testing.T, srv *choirserver.Server, intakeID, ownerID string) types.CandidatePackageIntakeRecord {
	t.Helper()
	detailW := serveCandidatePackageIntakeRequest(srv, http.MethodGet, "/api/candidate-package-intakes/"+intakeID, "", ownerID)
	if detailW.Code != http.StatusOK {
		t.Fatalf("detail status = %d, want 200; body=%s", detailW.Code, detailW.Body.String())
	}
	var detail types.CandidatePackageIntakeRecord
	if err := json.Unmarshal(detailW.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode candidate package intake detail: %v", err)
	}
	return detail
}

func assertCandidatePackageIntakeDetailMatchesPending(t *testing.T, srv *choirserver.Server, pending types.CandidatePackageIntakeRecord, ownerID string) {
	t.Helper()
	detail := candidatePackageIntakeDetail(t, srv, pending.IntakeID, ownerID)
	if detail.Status != types.CandidatePackageIntakeOwnerReviewPending || detail.OwnerReviewState != types.CandidatePackageOwnerReviewRequired || !detail.OwnerReviewRequired || detail.AdoptionReady {
		t.Fatalf("pending review boundary changed: status:%q state:%q required:%v adoption_ready:%v", detail.Status, detail.OwnerReviewState, detail.OwnerReviewRequired, detail.AdoptionReady)
	}
	assertCandidatePackageIntakeJSONStringArrayContains(t, "pending blockers", detail.AdoptionBlockersJSON, "owner_review_not_recorded")
	if string(detail.EvidenceRefsJSON) != string(pending.EvidenceRefsJSON) {
		t.Fatalf("pending evidence refs changed: got %s want %s", detail.EvidenceRefsJSON, pending.EvidenceRefsJSON)
	}
}

func assertCandidatePackageIntakeReviewTransition(t *testing.T, got, pending types.CandidatePackageIntakeRecord, wantStatus types.CandidatePackageIntakeStatus, wantReviewState types.CandidatePackageOwnerReviewState) {
	t.Helper()
	if got.IntakeID != pending.IntakeID || got.OwnerID != pending.OwnerID || got.CandidatePackageID != pending.CandidatePackageID || got.CandidatePackageManifestSHA256 != pending.CandidatePackageManifestSHA256 {
		t.Fatalf("review transition changed intake identity: got intake:%q owner:%q package:%q manifest:%q, want intake:%q owner:%q package:%q manifest:%q", got.IntakeID, got.OwnerID, got.CandidatePackageID, got.CandidatePackageManifestSHA256, pending.IntakeID, pending.OwnerID, pending.CandidatePackageID, pending.CandidatePackageManifestSHA256)
	}
	if got.SourceComputerID != pending.SourceComputerID || got.SourceCandidateID != pending.SourceCandidateID || got.CandidateSourceRef != pending.CandidateSourceRef {
		t.Fatalf("review transition changed source refs: got computer:%q candidate:%q ref:%q, want computer:%q candidate:%q ref:%q", got.SourceComputerID, got.SourceCandidateID, got.CandidateSourceRef, pending.SourceComputerID, pending.SourceCandidateID, pending.CandidateSourceRef)
	}
	if got.Status != wantStatus || got.OwnerReviewState != wantReviewState || got.OwnerReviewRequired || got.AdoptionReady {
		t.Fatalf("review transition boundary = status:%q state:%q required:%v adoption_ready:%v, want status:%q state:%q required:false adoption_ready:false", got.Status, got.OwnerReviewState, got.OwnerReviewRequired, got.AdoptionReady, wantStatus, wantReviewState)
	}
	if got.IntakeBoundary != pending.IntakeBoundary || got.TraceID != pending.TraceID {
		t.Fatalf("review transition changed evidence metadata: intake_boundary:%q trace_id:%q, want intake_boundary:%q trace_id:%q", got.IntakeBoundary, got.TraceID, pending.IntakeBoundary, pending.TraceID)
	}
}

func assertCandidatePackageIntakeJSONStringArrayContains(t *testing.T, name string, raw json.RawMessage, want string) {
	t.Helper()
	for _, got := range candidatePackageIntakeJSONStringArray(t, name, raw) {
		if got == want {
			return
		}
	}
	t.Fatalf("%s = %s, want to contain %q", name, raw, want)
}

func assertCandidatePackageIntakeJSONStringArrayNotContains(t *testing.T, name string, raw json.RawMessage, notWant string) {
	t.Helper()
	for _, got := range candidatePackageIntakeJSONStringArray(t, name, raw) {
		if got == notWant {
			t.Fatalf("%s = %s, want not to contain %q", name, raw, notWant)
		}
	}
}

func candidatePackageIntakeJSONStringArray(t *testing.T, name string, raw json.RawMessage) []string {
	t.Helper()
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		t.Fatalf("decode %s as string array: %v; raw=%s", name, err, raw)
	}
	return values
}

func candidatePackageIntakeRequestBody(t *testing.T, overrides map[string]any) string {
	t.Helper()
	body := map[string]any{
		"candidate_package_id":              "pkg-intake-request",
		"candidate_package_manifest_sha256": "sha256:intake-request",
		"source_computer_id":                "computer-intake-source",
		"source_candidate_id":               "candidate-intake-source",
		"candidate_source_ref":              "refs/computers/computer-intake-source/candidates/candidate-intake-source",
		"intake_boundary":                   "owner review boundary",
		"status":                            string(types.CandidatePackageIntakeOwnerReviewPending),
		"owner_review_state":                string(types.CandidatePackageOwnerReviewRequired),
		"owner_review_required":             true,
		"adoption_ready":                    false,
	}
	for key, value := range overrides {
		body[key] = value
	}
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal candidate package intake request: %v", err)
	}
	return string(data)
}

type candidatePackageIntakeRecordContract struct {
	OwnerID                        string
	CandidatePackageID             string
	CandidatePackageManifestSHA256 string
	SourceComputerID               string
	SourceCandidateID              string
	CandidateSourceRef             string
	IntakeBoundary                 string
	Status                         types.CandidatePackageIntakeStatus
	OwnerReviewState               types.CandidatePackageOwnerReviewState
	OwnerReviewRequired            bool
	AdoptionReady                  bool
	TraceID                        string
}

func assertCandidatePackageIntakeRecordFromAPI(t *testing.T, got types.CandidatePackageIntakeRecord, want candidatePackageIntakeRecordContract) {
	t.Helper()
	if got.IntakeID == "" {
		t.Fatal("intake_id is empty")
	}
	if got.OwnerID != want.OwnerID || got.CandidatePackageID != want.CandidatePackageID || got.CandidatePackageManifestSHA256 != want.CandidatePackageManifestSHA256 {
		t.Fatalf("intake identity = owner:%q package:%q manifest:%q, want owner:%q package:%q manifest:%q", got.OwnerID, got.CandidatePackageID, got.CandidatePackageManifestSHA256, want.OwnerID, want.CandidatePackageID, want.CandidatePackageManifestSHA256)
	}
	if got.SourceComputerID != want.SourceComputerID || got.SourceCandidateID != want.SourceCandidateID || got.CandidateSourceRef != want.CandidateSourceRef {
		t.Fatalf("source provenance = computer:%q candidate:%q ref:%q, want computer:%q candidate:%q ref:%q", got.SourceComputerID, got.SourceCandidateID, got.CandidateSourceRef, want.SourceComputerID, want.SourceCandidateID, want.CandidateSourceRef)
	}
	if got.IntakeBoundary != want.IntakeBoundary {
		t.Fatalf("intake_boundary = %q, want %q", got.IntakeBoundary, want.IntakeBoundary)
	}
	if got.Status != want.Status || got.OwnerReviewState != want.OwnerReviewState || got.OwnerReviewRequired != want.OwnerReviewRequired || got.AdoptionReady != want.AdoptionReady {
		t.Fatalf("review boundary = status:%q state:%q required:%v adoption_ready:%v, want status:%q state:%q required:%v adoption_ready:%v", got.Status, got.OwnerReviewState, got.OwnerReviewRequired, got.AdoptionReady, want.Status, want.OwnerReviewState, want.OwnerReviewRequired, want.AdoptionReady)
	}
	if got.TraceID != want.TraceID {
		t.Fatalf("trace_id = %q, want %q", got.TraceID, want.TraceID)
	}
}

func assertNoCandidatePackageIntakePromotionSideEffects(t *testing.T, rt *Runtime, ownerID string) {
	t.Helper()
	ctx := context.Background()
	packages, err := rt.store.ListAppChangePackages(ctx, ownerID, 10)
	if err != nil {
		t.Fatalf("list app change packages for %s: %v", ownerID, err)
	}
	if len(packages) != 0 {
		t.Fatalf("candidate package intake created app change packages for %s: %+v", ownerID, packages)
	}
	adoptions, err := rt.store.ListAppAdoptions(ctx, ownerID, 10)
	if err != nil {
		t.Fatalf("list app adoptions for %s: %v", ownerID, err)
	}
	if len(adoptions) != 0 {
		t.Fatalf("candidate package intake created app adoptions for %s: %+v", ownerID, adoptions)
	}
}

func TestCandidatePackageIntakePromotionSwitchNormalizesRouteProfile(t *testing.T) {
	t.Parallel()
	rt, handler := testAPISetup(t)
	srv := candidatePackageIntakeTestServer(handler)
	intakeID := "intake-promotion-switch-route-profile-normalize"
	ownerID := "user-alice"

	fixture := createCandidatePackageIntakePromotionSwitchReviewThroughRoute(t, rt, srv, ownerID, intakeID)
	approved := approveCandidatePackageIntakePromotionSwitchReviewThroughRoute(t, srv, fixture)

	// Seed the lineage with a legacy route: profile before the switch. The
	// switch must write a canonical owner/computer value, and the rollback must
	// restore the pre-switch value (normalized from legacy).
	lineage, err := rt.store.GetComputerSourceLineage(context.Background(), ownerID, fixture.TargetComputerID)
	if err != nil {
		t.Fatalf("get target source lineage: %v", err)
	}
	lineage.RouteProfile = "route:legacy"
	if _, err := rt.store.UpsertComputerSourceLineage(context.Background(), lineage); err != nil {
		t.Fatalf("upsert legacy route_profile lineage: %v", err)
	}

	wantSwitchedProfile := "user-alice/" + fixture.TargetComputerID
	wantLegacyNormalizedProfile := "user-alice/legacy"

	switchCandidatePackageIntakePromotionSwitchThroughRoute(t, srv, fixture, approved, "texture://evidence/"+intakeID+"/switch")
	lineage, err = rt.store.GetComputerSourceLineage(context.Background(), ownerID, fixture.TargetComputerID)
	if err != nil {
		t.Fatalf("get target source lineage after switch: %v", err)
	}
	if lineage.RouteProfile != wantSwitchedProfile {
		t.Fatalf("route_profile after switch = %q, want %q", lineage.RouteProfile, wantSwitchedProfile)
	}

	rollbackCandidatePackageIntakePromotionSwitchThroughRoute(t, srv, fixture, approved.AdoptionID)
	lineage, err = rt.store.GetComputerSourceLineage(context.Background(), ownerID, fixture.TargetComputerID)
	if err != nil {
		t.Fatalf("get target source lineage after rollback: %v", err)
	}
	if lineage.RouteProfile != wantLegacyNormalizedProfile {
		t.Fatalf("route_profile after rollback = %q, want %q", lineage.RouteProfile, wantLegacyNormalizedProfile)
	}

	rollForwardCandidatePackageIntakePromotionSwitchThroughRoute(t, srv, fixture, approved.AdoptionID, "texture://evidence/"+intakeID+"/roll-forward")
	lineage, err = rt.store.GetComputerSourceLineage(context.Background(), ownerID, fixture.TargetComputerID)
	if err != nil {
		t.Fatalf("get target source lineage after roll-forward: %v", err)
	}
	if lineage.RouteProfile != wantSwitchedProfile {
		t.Fatalf("route_profile after roll-forward = %q, want %q", lineage.RouteProfile, wantSwitchedProfile)
	}
}
