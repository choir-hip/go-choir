package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestAppChangePackageMigratesAcrossCandidateComputers(t *testing.T) {
	rt, handler := testAPISetup(t)
	ownerID := "mission-owner"
	traceID := "traj-app-change-migration"
	sourceRepo := testAppPromotionSourceRepo(t)
	rt.cfg.PromotionSourceRepo = sourceRepo
	rt.cfg.PromotionWorkspaceRoot = filepath.Join(t.TempDir(), "promotion-workspaces")
	rt.cfg.AppPromotionRuntimeBuildCommand = `mkdir -p .choir-promotion-artifacts/runtime && cp runtime.txt .choir-promotion-artifacts/runtime/runtime.txt`
	rt.cfg.AppPromotionRuntimeArtifactPath = ".choir-promotion-artifacts/runtime/runtime.txt"
	rt.cfg.AppPromotionUIBuildCommand = `mkdir -p frontend/dist && cp frontend/ui.txt frontend/dist/ui.txt`
	rt.cfg.AppPromotionUIArtifactPath = "frontend/dist"

	runtimePatch := testGitDiffForPath(t, sourceRepo, "runtime.txt", "runtime v1\n")
	uiPatch := testGitDiffForPath(t, sourceRepo, "frontend/ui.txt", "ui v1\n")
	packageBytes, err := json.Marshal(map[string]any{
		"app_id":                "podcast",
		"visibility":            "unlisted",
		"source_computer_id":    "user-a-computer",
		"source_candidate_id":   "candidate-user-a-podcast",
		"candidate_source_ref":  "refs/heads/computers/user-a-computer/candidates/candidate-user-a-podcast",
		"runtime_source_delta":  runtimePatch,
		"ui_source_delta":       uiPatch,
		"app_protocol_contract": "GET /api/podcast/library returns {items:[{podcast_id,title,episodes:[{episode_id,title,audio_url}]}]} for the Svelte podcast panel",
		"trace_id":              traceID,
	})
	if err != nil {
		t.Fatalf("marshal package: %v", err)
	}
	pkgW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/app-change-packages", string(packageBytes), ownerID)
	if pkgW.Code != http.StatusCreated {
		t.Fatalf("package status = %d body=%s", pkgW.Code, pkgW.Body.String())
	}
	var pkg types.AppChangePackageRecord
	if err := json.Unmarshal(pkgW.Body.Bytes(), &pkg); err != nil {
		t.Fatalf("decode package: %v", err)
	}
	if pkg.PackageID == "" || pkg.AppID != "podcast" {
		t.Fatalf("package identity = %+v", pkg)
	}
	if pkg.SourceCandidateID == "" || !strings.Contains(pkg.CandidateSourceRef, "/candidates/") {
		t.Fatalf("package did not preserve candidate source ref: %+v", pkg)
	}
	if pkg.SourceRuntimeArtifactDigest == "" || pkg.SourceUIArtifactDigest == "" {
		t.Fatalf("package missing source artifact digests: %+v", pkg)
	}

	adoptBody := `{
		"package_id":"` + pkg.PackageID + `",
		"target_candidate_id":"candidate-user-b-podcast",
		"trace_id":"` + traceID + `"
	}`
	adoptW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/computers/user-b-computer/adoptions", adoptBody, ownerID)
	if adoptW.Code != http.StatusCreated {
		t.Fatalf("adoption status = %d body=%s", adoptW.Code, adoptW.Body.String())
	}
	var adoption types.AppAdoptionRecord
	if err := json.Unmarshal(adoptW.Body.Bytes(), &adoption); err != nil {
		t.Fatalf("decode adoption: %v", err)
	}
	if adoption.Status != types.AppAdoptionCandidateApplied {
		t.Fatalf("initial adoption status = %q", adoption.Status)
	}
	if !strings.Contains(adoption.CandidateSourceRef, "/candidates/") {
		t.Fatalf("adoption must target candidate source ref: %+v", adoption)
	}

	verifyBody := `{
		"target_active_source_ref_at_cutover":"refs/computers/user-b-computer/active-foreground-tail",
		"foreground_tail_merge_result":"no-conflict",
		"merge_strategy":"rebase",
		"merge_conflicts":[]
	}`
	verifyW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/adoptions/"+adoption.AdoptionID+"/verify", verifyBody, ownerID)
	if verifyW.Code != http.StatusOK {
		t.Fatalf("verify status = %d body=%s", verifyW.Code, verifyW.Body.String())
	}
	if err := json.Unmarshal(verifyW.Body.Bytes(), &adoption); err != nil {
		t.Fatalf("decode verified adoption: %v", err)
	}
	if adoption.Status != types.AppAdoptionVerified {
		t.Fatalf("verified status = %q", adoption.Status)
	}
	if adoption.RuntimeArtifactDigest == "" || adoption.UIArtifactDigest == "" {
		t.Fatalf("verified adoption missing target artifact digests: %+v", adoption)
	}
	if adoption.RuntimeArtifactDigest == pkg.SourceRuntimeArtifactDigest || adoption.UIArtifactDigest == pkg.SourceUIArtifactDigest {
		t.Fatalf("target artifacts copied source digests: source=(%s,%s) target=(%s,%s)",
			pkg.SourceRuntimeArtifactDigest, pkg.SourceUIArtifactDigest, adoption.RuntimeArtifactDigest, adoption.UIArtifactDigest)
	}
	if adoption.ForegroundTailMergeResult != "no-conflict" || adoption.TargetActiveSourceRefAtCutover == adoption.TargetActiveSourceRefAtCandidateStart {
		t.Fatalf("foreground-tail accounting missing: %+v", adoption)
	}

	promoteW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/adoptions/"+adoption.AdoptionID+"/promote", `{}`, ownerID)
	if promoteW.Code != http.StatusOK {
		t.Fatalf("promote status = %d body=%s", promoteW.Code, promoteW.Body.String())
	}
	if err := json.Unmarshal(promoteW.Body.Bytes(), &adoption); err != nil {
		t.Fatalf("decode promoted adoption: %v", err)
	}
	if adoption.Status != types.AppAdoptionAdopted {
		t.Fatalf("promoted adoption status = %q", adoption.Status)
	}
	if !strings.Contains(string(adoption.RollbackProfileJSON), "refs/computers/user-b-computer/active-foreground-tail") {
		t.Fatalf("promoted adoption missing rollback source ref: %s", string(adoption.RollbackProfileJSON))
	}
	lineageW := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/computers/user-b-computer/source-lineage", "", ownerID)
	if lineageW.Code != http.StatusOK {
		t.Fatalf("lineage status = %d body=%s", lineageW.Code, lineageW.Body.String())
	}
	var lineage types.ComputerSourceLineageRecord
	if err := json.Unmarshal(lineageW.Body.Bytes(), &lineage); err != nil {
		t.Fatalf("decode lineage: %v", err)
	}
	if lineage.ActiveSourceRef != adoption.CandidateSourceRef || lineage.RuntimeDigest != adoption.RuntimeArtifactDigest || lineage.UIDigest != adoption.UIArtifactDigest {
		t.Fatalf("lineage did not advance to adopted candidate: lineage=%+v adoption=%+v", lineage, adoption)
	}

	platformBody := `{
		"package_id":"` + pkg.PackageID + `",
		"target_computer_kind":"platform",
		"target_candidate_id":"candidate-platform-podcast",
		"default_base_profile":"platform-default-with-podcast-v0",
		"trace_id":"` + traceID + `"
	}`
	platformW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/computers/platform-default/adoptions", platformBody, ownerID)
	if platformW.Code != http.StatusCreated {
		t.Fatalf("platform adoption status = %d body=%s", platformW.Code, platformW.Body.String())
	}
	var platformAdoption types.AppAdoptionRecord
	if err := json.Unmarshal(platformW.Body.Bytes(), &platformAdoption); err != nil {
		t.Fatalf("decode platform adoption: %v", err)
	}
	verifyPlatform := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/adoptions/"+platformAdoption.AdoptionID+"/verify", `{"foreground_tail_merge_result":"no-conflict","merge_strategy":"rebase"}`, ownerID)
	if verifyPlatform.Code != http.StatusOK {
		t.Fatalf("verify platform status = %d body=%s", verifyPlatform.Code, verifyPlatform.Body.String())
	}
	promotePlatform := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/adoptions/"+platformAdoption.AdoptionID+"/promote", `{}`, ownerID)
	if promotePlatform.Code != http.StatusOK {
		t.Fatalf("promote platform status = %d body=%s", promotePlatform.Code, promotePlatform.Body.String())
	}
	platformLineageW := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/computers/platform-default/source-lineage?kind=platform", "", ownerID)
	if platformLineageW.Code != http.StatusOK {
		t.Fatalf("platform lineage status = %d body=%s", platformLineageW.Code, platformLineageW.Body.String())
	}
	var platformLineage types.ComputerSourceLineageRecord
	if err := json.Unmarshal(platformLineageW.Body.Bytes(), &platformLineage); err != nil {
		t.Fatalf("decode platform lineage: %v", err)
	}
	if platformLineage.DefaultBaseProfile != "platform-default-with-podcast-v0" {
		t.Fatalf("platform default base not updated: %+v", platformLineage)
	}

	traceW := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/trace/trajectories/"+traceID, "", ownerID)
	if traceW.Code != http.StatusOK {
		t.Fatalf("trace status = %d body=%s", traceW.Code, traceW.Body.String())
	}
	traceBody := traceW.Body.String()
	for _, want := range []string{"published app package", "app adoption verified", "app adoption promoted"} {
		if !strings.Contains(traceBody, want) {
			t.Fatalf("trace missing %q: %s", want, traceBody)
		}
	}

	acceptanceBody := `{"target_mission_id":"mission-source-lineage-promotion-control-plane-v0","trajectory_id":"` + traceID + `","source_prompt_or_objective":"make the first podcast app migrate"}`
	acceptanceW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/run-acceptances/synthesize", acceptanceBody, ownerID)
	if acceptanceW.Code != http.StatusAccepted {
		t.Fatalf("acceptance status = %d body=%s", acceptanceW.Code, acceptanceW.Body.String())
	}
	var acceptance types.RunAcceptanceRecord
	if err := json.Unmarshal(acceptanceW.Body.Bytes(), &acceptance); err != nil {
		t.Fatalf("decode acceptance: %v", err)
	}
	if acceptance.AcceptanceLevel != types.RunAcceptancePromotionLevel || acceptance.State != types.RunAcceptanceAccepted {
		t.Fatalf("acceptance = %s/%s checkpoints=%+v", acceptance.AcceptanceLevel, acceptance.State, acceptance.Checkpoints)
	}
	if strings.Contains(strings.Join(acceptance.FailureResidualRisks, "\n"), "acceptance invariant product_path_observed is blocked") {
		t.Fatalf("direct package/adoption proof should satisfy product-path evidence: %+v", acceptance.FailureResidualRisks)
	}
	for _, want := range []string{"app_package_published", "app_adoption_verified", "app_adoption_promoted", "rollback_available"} {
		if !acceptanceHasCheckpoint(acceptance, want) {
			t.Fatalf("acceptance missing checkpoint %q: %+v", want, acceptance.Checkpoints)
		}
	}

	_ = rt
}

func TestPrivateAppChangePackageIsNotVisibleAcrossOwners(t *testing.T) {
	_, handler := testAPISetup(t)
	body := `{
		"app_id":"podcast",
		"visibility":"private",
		"source_computer_id":"user-a-computer",
		"source_candidate_id":"candidate-user-a-private",
		"runtime_source_delta":"runtime delta",
		"ui_source_delta":"ui delta",
		"app_protocol_contract":"contract"
	}`
	pkgW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/app-change-packages", body, "user-alice")
	if pkgW.Code != http.StatusCreated {
		t.Fatalf("package status = %d body=%s", pkgW.Code, pkgW.Body.String())
	}
	var pkg types.AppChangePackageRecord
	if err := json.Unmarshal(pkgW.Body.Bytes(), &pkg); err != nil {
		t.Fatalf("decode package: %v", err)
	}
	otherW := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/app-change-packages/"+pkg.PackageID, "", "user-bob")
	if otherW.Code != http.StatusNotFound {
		t.Fatalf("private package visible to other owner: status=%d body=%s", otherW.Code, otherW.Body.String())
	}
}

func TestAppChangePackageReviewEvidenceReturnsRedactedPackageScopedAcceptances(t *testing.T) {
	rt, handler := testAPISetup(t)
	body := `{
		"app_id":"portfolio-experiment",
		"visibility":"unlisted",
		"source_computer_id":"user-a-computer",
		"source_candidate_id":"candidate-user-a-review",
		"runtime_source_delta":"runtime delta",
		"ui_source_delta":"ui delta",
		"app_protocol_contract":"contract"
	}`
	pkgW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/app-change-packages", body, "user-alice")
	if pkgW.Code != http.StatusCreated {
		t.Fatalf("package status = %d body=%s", pkgW.Code, pkgW.Body.String())
	}
	var pkg types.AppChangePackageRecord
	if err := json.Unmarshal(pkgW.Body.Bytes(), &pkg); err != nil {
		t.Fatalf("decode package: %v", err)
	}

	_, err := rt.store.UpsertRunAcceptance(context.Background(), types.RunAcceptanceRecord{
		AcceptanceID:          "runacc-package-visible",
		OwnerID:               "user-bob",
		TargetMissionID:       "mission-review-evidence",
		TrajectoryID:          "traj-review-evidence",
		AcceptanceLevel:       types.RunAcceptancePromotionLevel,
		State:                 types.RunAcceptanceAccepted,
		SourcePromptObjective: "review package " + pkg.PackageID,
		EvidenceRefs: []types.RunAcceptanceEvidenceRef{{
			RefID:   "package-proof",
			Kind:    "app_change_package",
			Summary: "recipient build references package",
			Details: map[string]any{"package_id": pkg.PackageID},
		}},
		RollbackRefs: []types.RunAcceptanceRollbackRef{{
			Kind:    "source_ref",
			Ref:     "refs/computers/primary/active-before",
			Summary: "previous active source ref",
		}},
	})
	if err != nil {
		t.Fatalf("seed package acceptance: %v", err)
	}
	_, err = rt.store.UpsertRunAcceptance(context.Background(), types.RunAcceptanceRecord{
		AcceptanceID:    "runacc-unrelated",
		OwnerID:         "user-bob",
		TargetMissionID: "mission-unrelated",
		TrajectoryID:    "traj-unrelated",
		AcceptanceLevel: types.RunAcceptanceExportLevel,
		State:           types.RunAcceptanceAccepted,
		EvidenceRefs: []types.RunAcceptanceEvidenceRef{{
			RefID:   "other-proof",
			Kind:    "app_change_package",
			Summary: "different package",
			Details: map[string]any{"package_id": "pkg-other"},
		}},
	})
	if err != nil {
		t.Fatalf("seed unrelated acceptance: %v", err)
	}

	detailW := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/run-acceptances/runacc-package-visible", "", "user-charlie")
	if detailW.Code != http.StatusNotFound {
		t.Fatalf("cross-owner full acceptance detail visible: status=%d body=%s", detailW.Code, detailW.Body.String())
	}

	reviewPath := "/api/app-change-packages/" + pkg.PackageID + "/review-evidence?acceptance_id=runacc-package-visible&acceptance_id=runacc-unrelated"
	reviewW := registeredRuntimeRequest(t, handler, http.MethodGet, reviewPath, "", "user-charlie")
	if reviewW.Code != http.StatusOK {
		t.Fatalf("review evidence status = %d body=%s", reviewW.Code, reviewW.Body.String())
	}
	var review appChangePackageReviewEvidenceResponse
	if err := json.Unmarshal(reviewW.Body.Bytes(), &review); err != nil {
		t.Fatalf("decode review evidence: %v", err)
	}
	if len(review.Acceptances) != 1 {
		t.Fatalf("review acceptances = %+v, want one package-scoped summary", review.Acceptances)
	}
	got := review.Acceptances[0]
	if got.AcceptanceID != "runacc-package-visible" || got.State != types.RunAcceptanceAccepted || got.AcceptanceLevel != types.RunAcceptancePromotionLevel {
		t.Fatalf("review acceptance summary = %+v", got)
	}
	if got.TraceVisible {
		t.Fatalf("cross-owner package evidence should not claim Trace visibility: %+v", got)
	}
	if got.HumanProofState != "machine_receipt_only" || got.SupportsHumanReview || !got.MachineReceiptOnly {
		t.Fatalf("package-scoped acceptance should be machine receipt only: %+v", got)
	}
	if review.HumanProof.State != "evidence_pending" || len(review.HumanProof.Missing) == 0 {
		t.Fatalf("package without human proof should not be reviewable: %+v", review.HumanProof)
	}
	if strings.Contains(reviewW.Body.String(), "user-bob") {
		t.Fatalf("review evidence leaked source owner id: %s", reviewW.Body.String())
	}
}

func TestAppChangePackageReviewEvidenceRequiresNarrativeAndMediaForHumanReview(t *testing.T) {
	_, handler := testAPISetup(t)
	body := `{
		"app_id":"human-proof-experiment",
		"visibility":"unlisted",
		"source_computer_id":"user-a-computer",
		"source_candidate_id":"candidate-user-a-review",
		"runtime_source_delta":"runtime delta",
		"ui_source_delta":"ui delta",
		"app_protocol_contract":"contract",
		"provenance_refs":{
			"human_summary":"Owner-readable narrative for the whole candidate trajectory.",
			"recommendation":"try before install",
			"vtext_doc_id":"doc-human-proof",
			"vtext_revision_id":"rev-human-proof",
			"screenshot_refs":["test-results/human-proof.png"]
		}
	}`
	pkgW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/app-change-packages", body, "user-alice")
	if pkgW.Code != http.StatusCreated {
		t.Fatalf("package status = %d body=%s", pkgW.Code, pkgW.Body.String())
	}
	var pkg types.AppChangePackageRecord
	if err := json.Unmarshal(pkgW.Body.Bytes(), &pkg); err != nil {
		t.Fatalf("decode package: %v", err)
	}
	reviewW := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/app-change-packages/"+pkg.PackageID+"/review-evidence", "", "user-alice")
	if reviewW.Code != http.StatusOK {
		t.Fatalf("review evidence status = %d body=%s", reviewW.Code, reviewW.Body.String())
	}
	var review appChangePackageReviewEvidenceResponse
	if err := json.Unmarshal(reviewW.Body.Bytes(), &review); err != nil {
		t.Fatalf("decode review evidence: %v", err)
	}
	if review.HumanProof.State != "human_reviewable" {
		t.Fatalf("human proof state = %+v, want human_reviewable", review.HumanProof)
	}
	if len(review.HumanProof.NarrativeRefs) == 0 || len(review.HumanProof.ScreenshotRefs) != 1 {
		t.Fatalf("human proof refs = %+v", review.HumanProof)
	}
}

func TestInternalAppChangePackageDetailRequiresInternalCaller(t *testing.T) {
	_, handler := testAPISetup(t)
	body := `{
		"app_id":"portfolio-experiment",
		"visibility":"unlisted",
		"source_computer_id":"worker-computer",
		"source_candidate_id":"worker-candidate",
		"runtime_source_delta":"runtime delta from worker",
		"ui_source_delta":"ui delta from worker",
		"app_protocol_contract":"contract"
	}`
	pkgW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/app-change-packages", body, "user-alice")
	if pkgW.Code != http.StatusCreated {
		t.Fatalf("package status = %d body=%s", pkgW.Code, pkgW.Body.String())
	}
	var pkg types.AppChangePackageRecord
	if err := json.Unmarshal(pkgW.Body.Bytes(), &pkg); err != nil {
		t.Fatalf("decode package: %v", err)
	}

	publicReq := httptest.NewRequest(http.MethodGet, "/internal/runtime/app-change-packages/"+pkg.PackageID+"?owner_id=user-alice", nil)
	publicW := httptest.NewRecorder()
	handler.HandleInternalAppChangePackageDetail(publicW, publicReq)
	if publicW.Code != http.StatusForbidden {
		t.Fatalf("internal package detail without internal marker status = %d body=%s", publicW.Code, publicW.Body.String())
	}

	internalReq := httptest.NewRequest(http.MethodGet, "/internal/runtime/app-change-packages/"+pkg.PackageID+"?owner_id=user-alice", nil)
	internalReq.Header.Set("X-Internal-Caller", "true")
	internalW := httptest.NewRecorder()
	handler.HandleInternalAppChangePackageDetail(internalW, internalReq)
	if internalW.Code != http.StatusOK {
		t.Fatalf("internal package detail status = %d body=%s", internalW.Code, internalW.Body.String())
	}
	var got types.AppChangePackageRecord
	if err := json.Unmarshal(internalW.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode internal package detail: %v", err)
	}
	if got.PackageID != pkg.PackageID || !strings.Contains(got.RuntimeSourceDelta, "worker") || !strings.Contains(got.UISourceDelta, "worker") {
		t.Fatalf("internal package detail did not include full package source deltas: %+v", got)
	}

	importReq := httptest.NewRequest(http.MethodPost, "/internal/runtime/app-change-packages", strings.NewReader(`{
		"package_id":"package-imported-internal",
		"owner_id":"source-owner",
		"app_id":"portfolio-imported",
		"status":"published_unlisted",
		"visibility":"unlisted",
		"source_computer_id":"source-computer",
		"source_candidate_id":"source-candidate",
		"candidate_source_ref":"refs/computers/source-computer/candidates/source-candidate",
		"runtime_source_delta":"imported runtime delta",
		"ui_source_delta":"imported ui delta",
		"package_manifest_sha256":"manifest-sha"
	}`))
	importReq.Header.Set("X-Internal-Caller", "true")
	importW := httptest.NewRecorder()
	handler.HandleInternalAppChangePackagesRoot(importW, importReq)
	if importW.Code != http.StatusCreated {
		t.Fatalf("internal package import status = %d body=%s", importW.Code, importW.Body.String())
	}
	imported, err := handler.rt.store.GetAppChangePackageForViewer(context.Background(), "recipient-viewer", "package-imported-internal")
	if err != nil {
		t.Fatalf("imported package should be visible to recipient viewer: %v", err)
	}
	if imported.OwnerID != "source-owner" || !strings.Contains(imported.RuntimeSourceDelta, "imported runtime") {
		t.Fatalf("imported package did not preserve source identity/delta: %+v", imported)
	}
}

func TestAppChangePackageRejectsPrivateSourceMarkers(t *testing.T) {
	_, handler := testAPISetup(t)
	body := `{
		"app_id":"podcast",
		"visibility":"unlisted",
		"source_computer_id":"user-a-computer",
		"source_candidate_id":"candidate-user-a-secret",
		"runtime_source_delta":"runtime delta with api_key=leak",
		"ui_source_delta":"ui delta",
		"app_protocol_contract":"contract"
	}`
	pkgW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/app-change-packages", body, "user-alice")
	if pkgW.Code != http.StatusBadRequest {
		t.Fatalf("private source marker accepted: status=%d body=%s", pkgW.Code, pkgW.Body.String())
	}
}
