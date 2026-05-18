package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestAppAdoptionRequiresActualRecipientBuild(t *testing.T) {
	rt, handler := testAPISetup(t)
	sourceRepo := testAppPromotionSourceRepo(t)
	rt.cfg.PromotionSourceRepo = sourceRepo
	rt.cfg.PromotionWorkspaceRoot = filepath.Join(t.TempDir(), "promotion-workspaces")
	rt.cfg.AppPromotionRuntimeBuildCommand = "mkdir -p .choir-promotion-artifacts/runtime && cp runtime.txt .choir-promotion-artifacts/runtime/runtime.txt"
	rt.cfg.AppPromotionRuntimeArtifactPath = ".choir-promotion-artifacts/runtime/runtime.txt"
	rt.cfg.AppPromotionUIBuildCommand = "mkdir -p frontend/dist && cp frontend/ui.txt frontend/dist/ui.txt"
	rt.cfg.AppPromotionUIArtifactPath = "frontend/dist"

	runtimePatch := testGitDiffForPath(t, sourceRepo, "runtime.txt", "runtime v1\n")
	uiPatch := testGitDiffForPath(t, sourceRepo, "frontend/ui.txt", "ui v1\n")
	bodyBytes, err := json.Marshal(map[string]any{
		"app_id":                      "podcast",
		"visibility":                  "unlisted",
		"require_recipient_build":     true,
		"source_computer_id":          "user-a-computer",
		"source_candidate_id":         "candidate-user-a-podcast-build",
		"candidate_source_ref":        "refs/heads/computers/user-a-computer/candidates/candidate-user-a-podcast-build",
		"source_ledger_repo":          "https://github.com/yusefmosiah/choir-source-ledger.git",
		"source_ledger_candidate_ref": "refs/heads/computers/user-a-computer/candidates/candidate-user-a-podcast-build",
		"source_ledger_commit_sha":    "abc123",
		"runtime_source_delta":        runtimePatch,
		"ui_source_delta":             uiPatch,
		"app_protocol_contract":       "recipient_build_required: podcast API/UI contract",
		"trace_id":                    "traj-app-build",
	})
	if err != nil {
		t.Fatalf("marshal package body: %v", err)
	}
	pkgW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/app-change-packages", string(bodyBytes), "user-build")
	if pkgW.Code != http.StatusCreated {
		t.Fatalf("package status = %d body=%s", pkgW.Code, pkgW.Body.String())
	}
	var pkg types.AppChangePackageRecord
	if err := json.Unmarshal(pkgW.Body.Bytes(), &pkg); err != nil {
		t.Fatalf("decode package: %v", err)
	}
	if !strings.Contains(string(pkg.ManifestJSON), `"require_recipient_build":true`) {
		t.Fatalf("package manifest does not require recipient build: %s", string(pkg.ManifestJSON))
	}

	adoptBody := `{"package_id":"` + pkg.PackageID + `","target_candidate_id":"candidate-user-b-podcast-build","trace_id":"traj-app-build"}`
	adoptW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/computers/user-b-computer/adoptions", adoptBody, "user-build")
	if adoptW.Code != http.StatusCreated {
		t.Fatalf("adoption status = %d body=%s", adoptW.Code, adoptW.Body.String())
	}
	var adoption types.AppAdoptionRecord
	if err := json.Unmarshal(adoptW.Body.Bytes(), &adoption); err != nil {
		t.Fatalf("decode adoption: %v", err)
	}
	verifyW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/adoptions/"+adoption.AdoptionID+"/verify", `{"foreground_tail_merge_result":"no-conflict","merge_strategy":"rebase"}`, "user-build")
	if verifyW.Code != http.StatusOK {
		t.Fatalf("verify status = %d body=%s", verifyW.Code, verifyW.Body.String())
	}
	if err := json.Unmarshal(verifyW.Body.Bytes(), &adoption); err != nil {
		t.Fatalf("decode verified adoption: %v", err)
	}
	if adoption.Status != types.AppAdoptionVerified {
		t.Fatalf("adoption status = %q error=%s", adoption.Status, adoption.Error)
	}
	if adoption.RuntimeArtifactDigest == "" || adoption.UIArtifactDigest == "" {
		t.Fatalf("missing actual artifact digests: %+v", adoption)
	}
	if !strings.Contains(string(adoption.VerifierResultsJSON), "actual-recipient-runtime-ui-build") {
		t.Fatalf("verifier results missing actual build contract: %s", string(adoption.VerifierResultsJSON))
	}
}

func testAppPromotionSourceRepo(t *testing.T) string {
	t.Helper()
	repo := filepath.Join(t.TempDir(), "source")
	if err := os.MkdirAll(filepath.Join(repo, "frontend"), 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "runtime.txt"), []byte("runtime v0\n"), 0o644); err != nil {
		t.Fatalf("write runtime: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "frontend", "ui.txt"), []byte("ui v0\n"), 0o644); err != nil {
		t.Fatalf("write ui: %v", err)
	}
	testGit(t, repo, "init", "-b", "main")
	testGit(t, repo, "config", "user.name", "Test")
	testGit(t, repo, "config", "user.email", "test@example.com")
	testGit(t, repo, "add", ".")
	testGit(t, repo, "commit", "-m", "base")
	return repo
}

func testGitDiffForPath(t *testing.T, repo, relPath, content string) string {
	t.Helper()
	fullPath := filepath.Join(repo, filepath.FromSlash(relPath))
	original, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("read original %s: %v", relPath, err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write changed %s: %v", relPath, err)
	}
	diff := testGitOutput(t, repo, "diff", "--", relPath)
	if err := os.WriteFile(fullPath, original, 0o644); err != nil {
		t.Fatalf("restore original %s: %v", relPath, err)
	}
	if strings.TrimSpace(diff) == "" {
		t.Fatalf("empty diff for %s", relPath)
	}
	return diff
}

func testGit(t *testing.T, repo string, args ...string) {
	t.Helper()
	_ = testGitOutput(t, repo, args...)
}

func testGitOutput(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(context.Background(), "git", args...)
	cmd.Dir = repo
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("git %s: %s: %v", strings.Join(args, " "), out.String(), err)
	}
	return out.String()
}
