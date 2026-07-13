//go:build comprehensive

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
	t.Parallel()
	rt, handler := testAPISetup(t)
	ownerID := "mission-owner"
	traceID := "traj-app-change-migration"
	sourceRepo := testAppPromotionSourceRepo(t)
	rt.cfg.PromotionSourceRepo = sourceRepo
	rt.cfg.PromotionWorkspaceRoot = filepath.Join(t.TempDir(), "promotion-workspaces")
	rt.cfg.AppPromotionRuntimeBuildCommand = `mkdir -p .choir-promotion-artifacts/runtime && cp runtime.txt .choir-promotion-artifacts/runtime/runtime.txt`
	rt.cfg.AppPromotionRuntimeArtifactPath = ".choir-promotion-artifacts/runtime/runtime.txt"
	rt.cfg.AppPromotionUIBuildCommand = `mkdir -p frontend/dist/assets && cp frontend/ui.txt frontend/dist/ui.txt && printf '<script type="module" src="/assets/app.js"></script><div id="app">candidate UI</div>' > frontend/dist/index.html && printf 'console.log("candidate app")' > frontend/dist/assets/app.js`
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
	pkgW := runtimeHandlerRequest(t, handler.HandleAppChangePackagesRoot, http.MethodPost, "/api/app-change-packages", string(packageBytes), ownerID)
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
	adoptW := runtimeHandlerRequest(t, handler.HandleComputersRouter, http.MethodPost, "/api/computers/user-b-computer/adoptions", adoptBody, ownerID)
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
	verifyW := runtimeHandlerRequest(t, handler.HandleAppAdoptionDetail, http.MethodPost, "/api/adoptions/"+adoption.AdoptionID+"/verify", verifyBody, ownerID)
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
	previewW := runtimeHandlerRequest(t, handler.HandleAppAdoptionDetail, http.MethodGet, "/api/adoptions/"+adoption.AdoptionID+"/preview", "", ownerID)
	if previewW.Code != http.StatusOK {
		t.Fatalf("preview status = %d body=%s", previewW.Code, previewW.Body.String())
	}
	if !strings.Contains(previewW.Body.String(), "/api/adoptions/"+adoption.AdoptionID+"/preview/assets/app.js") {
		t.Fatalf("preview index did not rewrite asset path: %s", previewW.Body.String())
	}
	assetW := runtimeHandlerRequest(t, handler.HandleAppAdoptionDetail, http.MethodGet, "/api/adoptions/"+adoption.AdoptionID+"/preview/assets/app.js", "", ownerID)
	if assetW.Code != http.StatusOK || !strings.Contains(assetW.Body.String(), "candidate app") {
		t.Fatalf("preview asset status = %d body=%s", assetW.Code, assetW.Body.String())
	}

	prematurePromoteW := runtimeHandlerRequest(t, handler.HandleAppAdoptionDetail, http.MethodPost, "/api/adoptions/"+adoption.AdoptionID+"/promote", `{}`, ownerID)
	if prematurePromoteW.Code != http.StatusBadRequest {
		t.Fatalf("promote without owner approval must be rejected: status = %d body=%s", prematurePromoteW.Code, prematurePromoteW.Body.String())
	}
	approveW := runtimeHandlerRequest(t, handler.HandleAppAdoptionDetail, http.MethodPost, "/api/adoptions/"+adoption.AdoptionID+"/approve", `{}`, ownerID)
	if approveW.Code != http.StatusOK {
		t.Fatalf("approve status = %d body=%s", approveW.Code, approveW.Body.String())
	}
	if err := json.Unmarshal(approveW.Body.Bytes(), &adoption); err != nil {
		t.Fatalf("decode approved adoption: %v", err)
	}
	if adoption.Status != types.AppAdoptionOwnerApproved {
		t.Fatalf("approved adoption status = %q", adoption.Status)
	}

	promoteW := runtimeHandlerRequest(t, handler.HandleAppAdoptionDetail, http.MethodPost, "/api/adoptions/"+adoption.AdoptionID+"/promote", `{}`, ownerID)
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
	lineageW := runtimeHandlerRequest(t, handler.HandleComputersRouter, http.MethodGet, "/api/computers/user-b-computer/source-lineage", "", ownerID)
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
	platformW := runtimeHandlerRequest(t, handler.HandleComputersRouter, http.MethodPost, "/api/computers/platform-default/adoptions", platformBody, ownerID)
	if platformW.Code != http.StatusCreated {
		t.Fatalf("platform adoption status = %d body=%s", platformW.Code, platformW.Body.String())
	}
	var platformAdoption types.AppAdoptionRecord
	if err := json.Unmarshal(platformW.Body.Bytes(), &platformAdoption); err != nil {
		t.Fatalf("decode platform adoption: %v", err)
	}
	verifyPlatform := runtimeHandlerRequest(t, handler.HandleAppAdoptionDetail, http.MethodPost, "/api/adoptions/"+platformAdoption.AdoptionID+"/verify", `{"foreground_tail_merge_result":"no-conflict","merge_strategy":"rebase"}`, ownerID)
	if verifyPlatform.Code != http.StatusOK {
		t.Fatalf("verify platform status = %d body=%s", verifyPlatform.Code, verifyPlatform.Body.String())
	}
	approvePlatform := runtimeHandlerRequest(t, handler.HandleAppAdoptionDetail, http.MethodPost, "/api/adoptions/"+platformAdoption.AdoptionID+"/approve", `{}`, ownerID)
	if approvePlatform.Code != http.StatusOK {
		t.Fatalf("approve platform status = %d body=%s", approvePlatform.Code, approvePlatform.Body.String())
	}
	promotePlatform := runtimeHandlerRequest(t, handler.HandleAppAdoptionDetail, http.MethodPost, "/api/adoptions/"+platformAdoption.AdoptionID+"/promote", `{}`, ownerID)
	if promotePlatform.Code != http.StatusOK {
		t.Fatalf("promote platform status = %d body=%s", promotePlatform.Code, promotePlatform.Body.String())
	}
	platformLineageW := runtimeHandlerRequest(t, handler.HandleComputersRouter, http.MethodGet, "/api/computers/platform-default/source-lineage?kind=platform", "", ownerID)
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

	traceW := runtimeHandlerRequest(t, http.NotFound, http.MethodGet, "/api/trace/trajectories/"+traceID, "", ownerID)
	if traceW.Code != http.StatusOK {
		t.Fatalf("trace status = %d body=%s", traceW.Code, traceW.Body.String())
	}
	traceBody := traceW.Body.String()
	for _, want := range []string{"published app package", "app adoption verified", "app adoption promoted"} {
		if !strings.Contains(traceBody, want) {
			t.Fatalf("trace missing %q: %s", want, traceBody)
		}
	}

	acceptanceBody := `{"target_mission_id":"mission-campaign-compiler-selfdev-v0","trajectory_id":"` + traceID + `","source_prompt_or_objective":"make the first podcast app migrate"}`
	acceptanceW := runtimeHandlerRequest(t, handler.HandleRunAcceptanceSynthesize, http.MethodPost, "/api/run-acceptances/synthesize", acceptanceBody, ownerID)
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
	t.Parallel()
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
	pkgW := runtimeHandlerRequest(t, handler.HandleAppChangePackagesRoot, http.MethodPost, "/api/app-change-packages", body, "user-alice")
	if pkgW.Code != http.StatusCreated {
		t.Fatalf("package status = %d body=%s", pkgW.Code, pkgW.Body.String())
	}
	var pkg types.AppChangePackageRecord
	if err := json.Unmarshal(pkgW.Body.Bytes(), &pkg); err != nil {
		t.Fatalf("decode package: %v", err)
	}
	otherW := runtimeHandlerRequest(t, handler.HandleAppChangePackageDetail, http.MethodGet, "/api/app-change-packages/"+pkg.PackageID, "", "user-bob")
	if otherW.Code != http.StatusNotFound {
		t.Fatalf("private package visible to other owner: status=%d body=%s", otherW.Code, otherW.Body.String())
	}
}

func TestAppChangePackageReviewEvidenceReturnsRedactedPackageScopedAcceptances(t *testing.T) {
	t.Parallel()
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
	pkgW := runtimeHandlerRequest(t, handler.HandleAppChangePackagesRoot, http.MethodPost, "/api/app-change-packages", body, "user-alice")
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

	detailW := runtimeHandlerRequest(t, handler.HandleRunAcceptanceDetail, http.MethodGet, "/api/run-acceptances/runacc-package-visible", "", "user-charlie")
	if detailW.Code != http.StatusNotFound {
		t.Fatalf("cross-owner full acceptance detail visible: status=%d body=%s", detailW.Code, detailW.Body.String())
	}

	reviewPath := "/api/app-change-packages/" + pkg.PackageID + "/review-evidence?acceptance_id=runacc-package-visible&acceptance_id=runacc-unrelated"
	reviewW := runtimeHandlerRequest(t, handler.HandleAppChangePackageDetail, http.MethodGet, reviewPath, "", "user-charlie")
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
	t.Parallel()
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
			"texture_doc_id":"doc-human-proof",
			"texture_revision_id":"rev-human-proof",
			"screenshot_refs":["test-results/human-proof.png"]
		}
	}`
	pkgW := runtimeHandlerRequest(t, handler.HandleAppChangePackagesRoot, http.MethodPost, "/api/app-change-packages", body, "user-alice")
	if pkgW.Code != http.StatusCreated {
		t.Fatalf("package status = %d body=%s", pkgW.Code, pkgW.Body.String())
	}
	var pkg types.AppChangePackageRecord
	if err := json.Unmarshal(pkgW.Body.Bytes(), &pkg); err != nil {
		t.Fatalf("decode package: %v", err)
	}
	reviewW := runtimeHandlerRequest(t, handler.HandleAppChangePackageDetail, http.MethodGet, "/api/app-change-packages/"+pkg.PackageID+"/review-evidence", "", "user-alice")
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

	legacyNarrative := `{
		"app_id":"human-proof-legacy-texture-narrative",
		"visibility":"unlisted",
		"source_computer_id":"user-a-computer",
		"source_candidate_id":"candidate-user-a-legacy-narrative",
		"runtime_source_delta":"runtime delta",
		"ui_source_delta":"ui delta",
		"app_protocol_contract":"contract",
		"provenance_refs":{
			"human_summary":"Owner-readable narrative stored before the Texture package provenance cutover.",
			"texture_doc_id":"doc-legacy-proof",
			"texture_revision_id":"rev-legacy-proof",
			"screenshot_refs":["test-results/legacy-proof.png"]
		}
	}` // texture-cutover-allow: legacy AppChangePackage provenance refs remain reviewable until package provenance migration.
	legacyW := runtimeHandlerRequest(t, handler.HandleAppChangePackagesRoot, http.MethodPost, "/api/app-change-packages", legacyNarrative, "user-alice")
	if legacyW.Code != http.StatusCreated {
		t.Fatalf("legacy narrative package status = %d body=%s", legacyW.Code, legacyW.Body.String())
	}
	var legacyPkg types.AppChangePackageRecord
	if err := json.Unmarshal(legacyW.Body.Bytes(), &legacyPkg); err != nil {
		t.Fatalf("decode legacy narrative package: %v", err)
	}
	legacyReviewW := runtimeHandlerRequest(t, handler.HandleAppChangePackageDetail, http.MethodGet, "/api/app-change-packages/"+legacyPkg.PackageID+"/review-evidence", "", "user-alice")
	if legacyReviewW.Code != http.StatusOK {
		t.Fatalf("legacy review evidence status = %d body=%s", legacyReviewW.Code, legacyReviewW.Body.String())
	}
	var legacyReview appChangePackageReviewEvidenceResponse
	if err := json.Unmarshal(legacyReviewW.Body.Bytes(), &legacyReview); err != nil {
		t.Fatalf("decode legacy review evidence: %v", err)
	}
	if legacyReview.HumanProof.State != "human_reviewable" {
		t.Fatalf("legacy package provenance should remain reviewable during cutover: %+v", legacyReview.HumanProof)
	}

	summaryOnlyNarrative := `{
		"app_id":"human-proof-summary-only",
		"visibility":"unlisted",
		"source_computer_id":"user-a-computer",
		"source_candidate_id":"candidate-user-a-summary-only",
		"runtime_source_delta":"runtime delta",
		"ui_source_delta":"ui delta",
		"app_protocol_contract":"contract",
		"provenance_refs":{
			"human_summary":"This prose summary is useful display copy, but it is not a durable Texture narrative.",
			"screenshot_refs":["test-results/summary-only.png"]
		}
	}`
	summaryOnlyW := runtimeHandlerRequest(t, handler.HandleAppChangePackagesRoot, http.MethodPost, "/api/app-change-packages", summaryOnlyNarrative, "user-alice")
	if summaryOnlyW.Code != http.StatusCreated {
		t.Fatalf("summary-only package status = %d body=%s", summaryOnlyW.Code, summaryOnlyW.Body.String())
	}
	var summaryOnlyPkg types.AppChangePackageRecord
	if err := json.Unmarshal(summaryOnlyW.Body.Bytes(), &summaryOnlyPkg); err != nil {
		t.Fatalf("decode summary-only package: %v", err)
	}
	summaryOnlyReviewW := runtimeHandlerRequest(t, handler.HandleAppChangePackageDetail, http.MethodGet, "/api/app-change-packages/"+summaryOnlyPkg.PackageID+"/review-evidence", "", "user-alice")
	if summaryOnlyReviewW.Code != http.StatusOK {
		t.Fatalf("summary-only review evidence status = %d body=%s", summaryOnlyReviewW.Code, summaryOnlyReviewW.Body.String())
	}
	var summaryOnlyReview appChangePackageReviewEvidenceResponse
	if err := json.Unmarshal(summaryOnlyReviewW.Body.Bytes(), &summaryOnlyReview); err != nil {
		t.Fatalf("decode summary-only review evidence: %v", err)
	}
	if summaryOnlyReview.HumanProof.State == "human_reviewable" {
		t.Fatalf("human_summary plus screenshot must not count as a causal Texture narrative: %+v", summaryOnlyReview.HumanProof)
	}
	if !containsString(summaryOnlyReview.HumanProof.Missing, "narrative Texture") {
		t.Fatalf("summary-only missing evidence should name Texture narrative: %+v", summaryOnlyReview.HumanProof)
	}

	blockedBenchmarkOnly := `{
		"app_id":"human-proof-blocked-benchmark",
		"visibility":"unlisted",
		"source_computer_id":"user-a-computer",
		"source_candidate_id":"candidate-user-a-blocked-benchmark",
		"runtime_source_delta":"runtime delta",
		"ui_source_delta":"ui delta",
		"app_protocol_contract":"contract",
		"provenance_refs":{
			"human_summary":"Owner-readable narrative, but media capture is blocked.",
			"texture_doc_id":"doc-blocked-benchmark",
			"texture_revision_id":"rev-blocked-benchmark",
			"benchmark_refs":[
				"npm --prefix frontend run build: PASS",
				"npm --prefix frontend exec playwright test shelf-live-observability.spec.js: BLOCKED by worker VM dynamic-loader incompatibility"
			]
		}
	}`
	blockedW := runtimeHandlerRequest(t, handler.HandleAppChangePackagesRoot, http.MethodPost, "/api/app-change-packages", blockedBenchmarkOnly, "user-alice")
	if blockedW.Code != http.StatusCreated {
		t.Fatalf("blocked benchmark package status = %d body=%s", blockedW.Code, blockedW.Body.String())
	}
	var blockedPkg types.AppChangePackageRecord
	if err := json.Unmarshal(blockedW.Body.Bytes(), &blockedPkg); err != nil {
		t.Fatalf("decode blocked benchmark package: %v", err)
	}
	blockedReviewW := runtimeHandlerRequest(t, handler.HandleAppChangePackageDetail, http.MethodGet, "/api/app-change-packages/"+blockedPkg.PackageID+"/review-evidence", "", "user-alice")
	if blockedReviewW.Code != http.StatusOK {
		t.Fatalf("blocked benchmark review evidence status = %d body=%s", blockedReviewW.Code, blockedReviewW.Body.String())
	}
	var blockedReview appChangePackageReviewEvidenceResponse
	if err := json.Unmarshal(blockedReviewW.Body.Bytes(), &blockedReview); err != nil {
		t.Fatalf("decode blocked benchmark review evidence: %v", err)
	}
	if blockedReview.HumanProof.State == "human_reviewable" {
		t.Fatalf("blocked benchmark refs must not count as human proof: %+v", blockedReview.HumanProof)
	}
	if !containsString(blockedReview.HumanProof.Missing, "successful screenshots, video, or benchmark evidence") {
		t.Fatalf("blocked benchmark missing evidence should be explicit: %+v", blockedReview.HumanProof)
	}

	launderedBuildProof := `{
		"app_id":"human-proof-laundered-build-proof",
		"visibility":"unlisted",
		"source_computer_id":"user-a-computer",
		"source_candidate_id":"candidate-user-a-laundered-build",
		"runtime_source_delta":"runtime delta",
		"ui_source_delta":"ui delta",
		"app_protocol_contract":"contract",
		"provenance_refs":{
			"human_summary":"Owner-readable narrative, but no screenshots, video, or measured behavior benchmark exists.",
			"recommendation":"reviewable_with_build_proof; media screenshot not available in this worker VM, but causal narrative and build benchmark are attached",
			"texture_doc_id":"doc-laundered-build",
			"texture_revision_id":"rev-laundered-build",
			"benchmark_refs":[
				"npm --prefix frontend ci && npm --prefix frontend run build (passed; standard chunk-size warnings; npm audit reported 8 moderate vulnerabilities)"
			]
		}
	}`
	launderedW := runtimeHandlerRequest(t, handler.HandleAppChangePackagesRoot, http.MethodPost, "/api/app-change-packages", launderedBuildProof, "user-alice")
	if launderedW.Code != http.StatusCreated {
		t.Fatalf("laundered build proof package status = %d body=%s", launderedW.Code, launderedW.Body.String())
	}
	var launderedPkg types.AppChangePackageRecord
	if err := json.Unmarshal(launderedW.Body.Bytes(), &launderedPkg); err != nil {
		t.Fatalf("decode laundered build proof package: %v", err)
	}
	launderedReviewW := runtimeHandlerRequest(t, handler.HandleAppChangePackageDetail, http.MethodGet, "/api/app-change-packages/"+launderedPkg.PackageID+"/review-evidence", "", "user-alice")
	if launderedReviewW.Code != http.StatusOK {
		t.Fatalf("laundered build proof review evidence status = %d body=%s", launderedReviewW.Code, launderedReviewW.Body.String())
	}
	var launderedReview appChangePackageReviewEvidenceResponse
	if err := json.Unmarshal(launderedReviewW.Body.Bytes(), &launderedReview); err != nil {
		t.Fatalf("decode laundered build proof review evidence: %v", err)
	}
	if launderedReview.HumanProof.State == "human_reviewable" {
		t.Fatalf("build-proof recommendation and build-only refs must not count as human proof: %+v", launderedReview.HumanProof)
	}
	if len(launderedReview.HumanProof.BenchmarkRefs) != 1 {
		t.Fatalf("expected only explicit benchmark_refs to be collected, not recommendation prose: %+v", launderedReview.HumanProof)
	}
	if !containsString(launderedReview.HumanProof.Missing, "successful screenshots, video, or benchmark evidence") {
		t.Fatalf("laundered build proof missing evidence should be explicit: %+v", launderedReview.HumanProof)
	}
}

func TestInternalAppChangePackageDetailRequiresInternalCaller(t *testing.T) {
	t.Parallel()
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
	pkgW := runtimeHandlerRequest(t, handler.HandleAppChangePackagesRoot, http.MethodPost, "/api/app-change-packages", body, "user-alice")
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
	t.Parallel()
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
	pkgW := runtimeHandlerRequest(t, handler.HandleAppChangePackagesRoot, http.MethodPost, "/api/app-change-packages", body, "user-alice")
	if pkgW.Code != http.StatusBadRequest {
		t.Fatalf("private source marker accepted: status=%d body=%s", pkgW.Code, pkgW.Body.String())
	}
}
