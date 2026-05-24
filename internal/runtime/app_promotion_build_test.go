//go:build comprehensive

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
	rt.cfg.AppPromotionRuntimeBuildCommand = `test -d "$GOTMPDIR" && case "$GOTMPDIR" in *'.choir-promotion-scratch'*) ;; *) echo "GOTMPDIR=$GOTMPDIR"; exit 19;; esac && mkdir -p .choir-promotion-artifacts/runtime && cp runtime.txt .choir-promotion-artifacts/runtime/runtime.txt`
	rt.cfg.AppPromotionRuntimeArtifactPath = ".choir-promotion-artifacts/runtime/runtime.txt"
	rt.cfg.AppPromotionUIBuildCommand = `test -d "$NPM_CONFIG_CACHE" && mkdir -p frontend/dist && cp frontend/ui.txt frontend/dist/ui.txt`
	rt.cfg.AppPromotionUIArtifactPath = "frontend/dist"

	runtimePatch := testGitDiffForPath(t, sourceRepo, "runtime.txt", "runtime v1\n")
	uiPatch := testGitDiffForPath(t, sourceRepo, "frontend/ui.txt", "ui v1\n")
	bodyBytes, err := json.Marshal(map[string]any{
		"app_id":                      "podcast",
		"visibility":                  "unlisted",
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
	if !strings.Contains(string(pkg.ManifestJSON), `"recipient_build_required":true`) {
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
	events, err := rt.store.ListEventsByTrajectory(context.Background(), "user-build", "traj-app-build", 20)
	if err != nil {
		t.Fatalf("list adoption trace events: %v", err)
	}
	if !testEventsContainKind(events, types.EventAppAdoptionVerificationStarted) {
		t.Fatalf("trace events missing verification-started checkpoint: %+v", events)
	}
}

func TestAppAdoptionVerificationLeavesStartedEvidenceOnBuildFailure(t *testing.T) {
	rt, handler := testAPISetup(t)
	sourceRepo := testAppPromotionSourceRepo(t)
	rt.cfg.PromotionSourceRepo = sourceRepo
	rt.cfg.PromotionWorkspaceRoot = filepath.Join(t.TempDir(), "promotion-workspaces")
	rt.cfg.AppPromotionRuntimeBuildCommand = "echo runtime build started && exit 42"
	rt.cfg.AppPromotionRuntimeArtifactPath = ".choir-promotion-artifacts/runtime/runtime.txt"

	runtimePatch := testGitDiffForPath(t, sourceRepo, "runtime.txt", "runtime v1\n")
	uiPatch := testGitDiffForPath(t, sourceRepo, "frontend/ui.txt", "ui v1\n")
	bodyBytes, err := json.Marshal(map[string]any{
		"app_id":                      "podcast",
		"visibility":                  "unlisted",
		"source_computer_id":          "user-a-computer",
		"source_candidate_id":         "candidate-user-a-podcast-build-failure",
		"candidate_source_ref":        "refs/heads/computers/user-a-computer/candidates/candidate-user-a-podcast-build-failure",
		"source_ledger_repo":          "https://github.com/yusefmosiah/choir-source-ledger.git",
		"source_ledger_candidate_ref": "refs/heads/computers/user-a-computer/candidates/candidate-user-a-podcast-build-failure",
		"source_ledger_commit_sha":    "abc123",
		"runtime_source_delta":        runtimePatch,
		"ui_source_delta":             uiPatch,
		"app_protocol_contract":       "recipient_build_required: podcast API/UI contract",
		"trace_id":                    "traj-app-build-failure",
	})
	if err != nil {
		t.Fatalf("marshal package body: %v", err)
	}
	pkgW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/app-change-packages", string(bodyBytes), "user-build-failure")
	if pkgW.Code != http.StatusCreated {
		t.Fatalf("package status = %d body=%s", pkgW.Code, pkgW.Body.String())
	}
	var pkg types.AppChangePackageRecord
	if err := json.Unmarshal(pkgW.Body.Bytes(), &pkg); err != nil {
		t.Fatalf("decode package: %v", err)
	}

	adoptBody := `{"package_id":"` + pkg.PackageID + `","target_candidate_id":"candidate-user-b-podcast-build-failure","trace_id":"traj-app-build-failure"}`
	adoptW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/computers/user-b-computer/adoptions", adoptBody, "user-build-failure")
	if adoptW.Code != http.StatusCreated {
		t.Fatalf("adoption status = %d body=%s", adoptW.Code, adoptW.Body.String())
	}
	var adoption types.AppAdoptionRecord
	if err := json.Unmarshal(adoptW.Body.Bytes(), &adoption); err != nil {
		t.Fatalf("decode adoption: %v", err)
	}
	verifyW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/adoptions/"+adoption.AdoptionID+"/verify", `{"foreground_tail_merge_result":"no-conflict","merge_strategy":"rebase"}`, "user-build-failure")
	if verifyW.Code != http.StatusBadRequest {
		t.Fatalf("verify status = %d body=%s", verifyW.Code, verifyW.Body.String())
	}
	detailW := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/adoptions/"+adoption.AdoptionID, "", "user-build-failure")
	if detailW.Code != http.StatusOK {
		t.Fatalf("adoption detail status = %d body=%s", detailW.Code, detailW.Body.String())
	}
	if err := json.Unmarshal(detailW.Body.Bytes(), &adoption); err != nil {
		t.Fatalf("decode failed adoption detail: %v", err)
	}
	if adoption.Status != types.AppAdoptionBlocked {
		t.Fatalf("adoption status = %q, want blocked; body=%s", adoption.Status, detailW.Body.String())
	}
	if !strings.Contains(string(adoption.VerifierResultsJSON), "runtime build started") {
		t.Fatalf("blocked adoption did not preserve build output: %s", string(adoption.VerifierResultsJSON))
	}
	if !strings.Contains(string(adoption.RollbackProfileJSON), "previous_active_source_ref") {
		t.Fatalf("blocked adoption missing rollback profile: %s", string(adoption.RollbackProfileJSON))
	}
	events, err := rt.store.ListEventsByTrajectory(context.Background(), "user-build-failure", "traj-app-build-failure", 20)
	if err != nil {
		t.Fatalf("list failed adoption trace events: %v", err)
	}
	if !testEventsContainKind(events, types.EventAppAdoptionVerificationStarted) || !testEventsContainKind(events, types.EventAppAdoptionBlocked) {
		t.Fatalf("trace events missing started/blocked evidence: %+v", events)
	}
}

func TestAppPromotionBaseRefPrefersPackageLedgerBase(t *testing.T) {
	t.Setenv("RUNTIME_WORKER_REPO_BASE_SHA", "deployed-head")
	pkg := types.AppChangePackageRecord{
		ManifestJSON:    json.RawMessage(`{"source_ledger_base_ref":"package-base-sha"}`),
		SourceActiveRef: "source-active",
	}
	rec := types.AppAdoptionRecord{
		TargetActiveSourceRefAtCandidateStart: "target-start",
	}
	if got := appPromotionBaseRef(pkg, rec, "target-cutover"); got != "package-base-sha" {
		t.Fatalf("base ref = %q, want package ledger base", got)
	}
}

func TestAppPromotionBaseRefNormalizesLedgerGitToken(t *testing.T) {
	t.Setenv("RUNTIME_WORKER_REPO_BASE_SHA", "deployed-head")
	const deployedSHA = "357d20670af512af5fdb4eb04b310527047d2e5c"
	pkg := types.AppChangePackageRecord{
		ManifestJSON:    json.RawMessage(`{"source_ledger_base_ref":"git:go-choir-candidate@357d20670af512af5fdb4eb04b310527047d2e5c\n"}`),
		SourceActiveRef: "source-active",
	}
	rec := types.AppAdoptionRecord{
		TargetActiveSourceRefAtCandidateStart: "target-start",
	}
	if got := appPromotionBaseRef(pkg, rec, "target-cutover"); got != deployedSHA {
		t.Fatalf("base ref = %q, want checkoutable sha %q", got, deployedSHA)
	}
}

func TestAppPromotionBaseRefNormalizesPlainGitSHARef(t *testing.T) {
	t.Setenv("RUNTIME_WORKER_REPO_BASE_SHA", "deployed-head")
	const deployedSHA = "f3bcd44d6ac651e73138a53d636af47da0c5a606"
	pkg := types.AppChangePackageRecord{
		ManifestJSON:    json.RawMessage(`{"source_ledger_base_ref":"git:f3bcd44d6ac651e73138a53d636af47da0c5a606"}`),
		SourceActiveRef: "source-active",
	}
	rec := types.AppAdoptionRecord{
		TargetActiveSourceRefAtCandidateStart: "target-start",
	}
	if got := appPromotionBaseRef(pkg, rec, "target-cutover"); got != deployedSHA {
		t.Fatalf("base ref = %q, want checkoutable sha %q", got, deployedSHA)
	}
}

func TestAppPromotionBaseRefNormalizesLedgerRoleToken(t *testing.T) {
	t.Setenv("RUNTIME_WORKER_REPO_BASE_SHA", "deployed-head")
	const deployedSHA = "b11ed4f2f517b2f1a7a3d8a054b17490b76510ec"
	pkg := types.AppChangePackageRecord{
		ManifestJSON:    json.RawMessage(`{"source_ledger_base_ref":"base:b11ed4f2f517b2f1a7a3d8a054b17490b76510ec"}`),
		SourceActiveRef: "source-active",
	}
	rec := types.AppAdoptionRecord{
		TargetActiveSourceRefAtCandidateStart: "target-start",
	}
	if got := appPromotionBaseRef(pkg, rec, "target-cutover"); got != deployedSHA {
		t.Fatalf("base ref = %q, want checkoutable sha %q", got, deployedSHA)
	}
}

func TestAppPromotionBaseRefSkipsProductOnlyComputerRefs(t *testing.T) {
	t.Setenv("RUNTIME_WORKER_REPO_BASE_SHA", "deployed-head")
	pkg := types.AppChangePackageRecord{
		ManifestJSON:    json.RawMessage(`{"source_ledger_base_ref":"refs/computers/user-a/active"}`),
		SourceActiveRef: "refs/platform-computers/default/active",
	}
	rec := types.AppAdoptionRecord{
		TargetActiveSourceRefAtCandidateStart: "refs/computers/user-b/active",
	}
	if got := appPromotionBaseRef(pkg, rec, "refs/heads/app-adoptions/candidate"); got != "app-adoptions/candidate" {
		t.Fatalf("base ref = %q, want checkoutable branch", got)
	}
}

func TestAppPromotionBuildEnvUsesWorkspaceScratchPaths(t *testing.T) {
	root := filepath.Join(t.TempDir(), "promotion-workspaces")
	candidateDir := filepath.Join(root, "adoption-1")
	env, scratchRoot, err := appPromotionBuildEnv(candidateDir)
	if err != nil {
		t.Fatalf("appPromotionBuildEnv: %v", err)
	}
	if want := filepath.Join(candidateDir, ".choir-promotion-scratch"); scratchRoot != want {
		t.Fatalf("scratch root = %q, want %q", scratchRoot, want)
	}

	scratchKeys := []string{"TMPDIR", "GOTMPDIR", "GOCACHE", "XDG_CACHE_HOME"}
	for _, key := range scratchKeys {
		value := envValue(env, key)
		if !strings.HasPrefix(value, scratchRoot+string(os.PathSeparator)) {
			t.Fatalf("%s = %q, want under scratch root %q", key, value, scratchRoot)
		}
		if _, err := os.Stat(value); err != nil {
			t.Fatalf("%s dir %q not prepared: %v", key, value, err)
		}
	}

	cacheRoot := filepath.Join(root, ".choir-promotion-cache")
	for _, key := range []string{"GOMODCACHE", "NPM_CONFIG_CACHE"} {
		value := envValue(env, key)
		if !strings.HasPrefix(value, cacheRoot+string(os.PathSeparator)) {
			t.Fatalf("%s = %q, want under cache root %q", key, value, cacheRoot)
		}
		if _, err := os.Stat(value); err != nil {
			t.Fatalf("%s dir %q not prepared: %v", key, value, err)
		}
	}

	report, err := runAppPromotionShellCommand(context.Background(), t.TempDir(), env, "env-check", `test -d "$GOTMPDIR" && test -d "$GOCACHE" && test -d "$GOMODCACHE" && test -d "$NPM_CONFIG_CACHE"`)
	if err != nil {
		t.Fatalf("promotion command with build env failed: %+v err=%v", report, err)
	}
}

func TestDefaultAppPromotionBuildCommandsUseBuildCapableMemoryCaps(t *testing.T) {
	if !strings.Contains(DefaultAppPromotionRuntimeBuildCommand, "GOMEMLIMIT=1024MiB") {
		t.Fatalf("runtime promotion build command should use the build-capable memory cap: %s", DefaultAppPromotionRuntimeBuildCommand)
	}
	if !strings.Contains(DefaultAppPromotionUIBuildCommand, "--max-old-space-size=768") {
		t.Fatalf("UI promotion build command should use the build-capable memory cap: %s", DefaultAppPromotionUIBuildCommand)
	}
}

func TestTruncateAppPromotionOutputPreservesHeadAndTail(t *testing.T) {
	long := strings.Repeat("a", 13000) + "compiler-tail-error"
	got := truncateAppPromotionOutput(long)
	if len(got) > 12050 {
		t.Fatalf("truncated output too long: %d", len(got))
	}
	if !strings.Contains(got, "compiler-tail-error") {
		t.Fatalf("truncated output lost compiler tail: %q", got[len(got)-200:])
	}
	if !strings.Contains(got, "truncated middle") {
		t.Fatalf("truncated output missing marker: %q", got)
	}
}

func testEventsContainKind(events []types.EventRecord, kind types.EventKind) bool {
	for _, ev := range events {
		if ev.Kind == kind {
			return true
		}
	}
	return false
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
