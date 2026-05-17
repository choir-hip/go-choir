package runtime

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/promotion"
	"github.com/yusefmosiah/go-choir/internal/shipper"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestRuntimePromotionQueueDogfoodsLauncherUploadsThemesPatch(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)

	source, err := rt.StartRunWithMetadata(ctx, "own candidate-world promotion", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileSuper,
		runMetadataAgentRole:    AgentProfileSuper,
	})
	if err != nil {
		t.Fatalf("start source run: %v", err)
	}
	sourceDone := waitForRunTerminalState(t, rt, source.RunID, "user-alice", 5*time.Second)

	baseRepo, _, base, workerHead, exportReport := prepareRuntimeLauncherCandidate(t)
	integrationRepo := runtimeCloneRepoAtBase(t, baseRepo, base)
	candidate := promotion.CandidateWorld{
		CandidateID:         "candidate-runtime-launcher-uploads-themes",
		OwnerID:             "user-alice",
		ForegroundDesktopID: types.PrimaryDesktopID,
		ParentRunID:         sourceDone.RunID,
		CandidateRunID:      "run-vsuper-runtime-dogfood",
		VMID:                "vm-runtime-candidate",
		SnapshotID:          "snapshot-runtime-candidate",
		Purpose:             "Dogfood launcher/uploads/themes patch through runtime promotion queue",
		BaseSHA:             base,
		WorkerHeadSHA:       workerHead,
		ManifestPath:        exportReport.ManifestPath,
		PatchsetPath:        exportReport.PatchsetPath,
		IntegrationBranch:   "agent/run-vsuper-runtime-dogfood/launcher-uploads-themes",
	}
	candidateJSON, err := json.Marshal(candidate)
	if err != nil {
		t.Fatalf("marshal candidate: %v", err)
	}
	contractsJSON, err := json.Marshal([]promotion.VerifierContract{{
		ContractID:              "launcher-uploads-themes-marker",
		Target:                  "frontend/src/lib/Launcher.svelte",
		Purpose:                 "Verify the product patch carries launcher, upload, and theme onboarding affordance markers.",
		Invariants:              []string{"main branch remains at rollback base until explicit promotion"},
		RequiredChecks:          []string{"grep -q 'launch-with-uploads-themes' frontend/src/lib/Launcher.svelte"},
		CapabilityProfile:       "shell",
		IndependenceRequirement: "verification runs after import in the integration checkout",
		EvidencePaths:           []string{"frontend/src/lib/Launcher.svelte", exportReport.ManifestPath, exportReport.PatchsetPath},
	}})
	if err != nil {
		t.Fatalf("marshal contracts: %v", err)
	}

	queued, err := rt.QueuePromotionCandidate(ctx, types.PromotionCandidateRecord{
		CandidateID:       candidate.CandidateID,
		OwnerID:           "user-alice",
		Status:            types.PromotionCandidateQueued,
		SourceRunID:       sourceDone.RunID,
		TraceID:           "trace-runtime-dogfood",
		VMID:              candidate.VMID,
		SnapshotID:        candidate.SnapshotID,
		BaseSHA:           base,
		WorkerHeadSHA:     workerHead,
		ManifestPath:      exportReport.ManifestPath,
		PatchsetPath:      exportReport.PatchsetPath,
		IntegrationBranch: candidate.IntegrationBranch,
		DestinationBranch: "main",
		Summary:           "launcher/uploads/themes onboarding patch",
		CandidateJSON:     candidateJSON,
		ContractsJSON:     contractsJSON,
	})
	if err != nil {
		t.Fatalf("queue promotion candidate: %v", err)
	}

	verified, err := rt.VerifyPromotionCandidate(ctx, "user-alice", queued.CandidateID, integrationRepo)
	if err != nil {
		t.Fatalf("verify promotion candidate: %v", err)
	}
	if verified.Status != types.PromotionCandidateVerified {
		t.Fatalf("verified status = %s", verified.Status)
	}
	if mainHead := strings.TrimSpace(runGit(t, integrationRepo, "rev-parse", "main")); mainHead != base {
		t.Fatalf("main mutated before promotion: got %s want %s", mainHead, base)
	}

	if _, err := rt.PromotePromotionCandidate(ctx, "user-alice", queued.CandidateID, integrationRepo, true); err == nil {
		t.Fatal("promotion should require owner approval after verification")
	}
	approved, err := rt.ReviewPromotionCandidate(ctx, "user-alice", queued.CandidateID, "approve")
	if err != nil {
		t.Fatalf("approve promotion candidate: %v", err)
	}
	if approved.Status != types.PromotionCandidateVerified {
		t.Fatalf("approved status = %s", approved.Status)
	}

	promoted, err := rt.PromotePromotionCandidate(ctx, "user-alice", queued.CandidateID, integrationRepo, true)
	if err != nil {
		t.Fatalf("promote candidate: %v", err)
	}
	if promoted.Status != types.PromotionCandidatePromoted {
		t.Fatalf("promoted status = %s", promoted.Status)
	}
	if got := runtimeReadFile(t, filepath.Join(integrationRepo, "frontend/src/lib/Launcher.svelte")); !strings.Contains(got, "launch-with-uploads-themes") {
		t.Fatalf("promoted main missing product marker: %s", got)
	}

	events, err := s.ListEvents(ctx, sourceDone.RunID, 100)
	if err != nil {
		t.Fatalf("list source events: %v", err)
	}
	kinds := map[types.EventKind]bool{}
	for _, ev := range events {
		kinds[ev.Kind] = true
	}
	for _, kind := range []types.EventKind{
		types.EventPromotionCandidateQueued,
		types.EventPromotionCandidateVerified,
		types.EventPromotionCandidateReviewed,
		types.EventPromotionCandidatePromoted,
	} {
		if !kinds[kind] {
			t.Fatalf("source run missing event %s; got %+v", kind, kinds)
		}
	}
}

func TestRuntimePromotionWorkspaceVerifiesQueuedExportWithoutCanonicalMutation(t *testing.T) {
	ctx := context.Background()
	rt, _ := testRuntime(t)

	source, err := rt.StartRunWithMetadata(ctx, "verify candidate in product workspace", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileSuper,
		runMetadataAgentRole:    AgentProfileSuper,
	})
	if err != nil {
		t.Fatalf("start source run: %v", err)
	}
	sourceDone := waitForRunTerminalState(t, rt, source.RunID, "user-alice", 5*time.Second)

	baseRepo, _, base, workerHead, exportReport := prepareRuntimeLauncherCandidate(t)
	rt.cfg.PromotionSourceRepo = baseRepo
	rt.cfg.PromotionWorkspaceRoot = filepath.Join(t.TempDir(), "promotion-workspaces")

	candidateID := "candidate-runtime-product-workspace"
	manifestPath, patchsetPath := runtimeCopyPromotionArtifacts(t, rt, candidateID, exportReport.ManifestPath, exportReport.PatchsetPath)
	queued, err := rt.QueuePromotionCandidate(ctx, types.PromotionCandidateRecord{
		CandidateID:       candidateID,
		OwnerID:           "user-alice",
		Status:            types.PromotionCandidateQueued,
		SourceRunID:       sourceDone.RunID,
		TraceID:           "trace-runtime-product-workspace",
		VMID:              "vm-runtime-product-workspace",
		BaseSHA:           base,
		WorkerHeadSHA:     workerHead,
		ManifestPath:      manifestPath,
		PatchsetPath:      patchsetPath,
		IntegrationBranch: "agent/run-vsuper-runtime-product-workspace/candidate",
		DestinationBranch: "main",
		Summary:           "product-safe promotion workspace proof",
		ContractsJSON:     json.RawMessage(`[]`),
	})
	if err != nil {
		t.Fatalf("queue promotion candidate: %v", err)
	}

	verified, err := rt.VerifyPromotionCandidateInWorkspace(ctx, "user-alice", queued.CandidateID)
	if err != nil {
		t.Fatalf("verify candidate in workspace: %v", err)
	}
	if verified.Status != types.PromotionCandidateVerified {
		t.Fatalf("verified status = %s", verified.Status)
	}

	var report promotion.Report
	if err := json.Unmarshal(verified.ReportJSON, &report); err != nil {
		t.Fatalf("decode promotion report: %v", err)
	}
	if report.Status != "verified" || report.CanonicalMutated {
		t.Fatalf("report status/canonical mutation = %q/%v", report.Status, report.CanonicalMutated)
	}
	if len(report.VerifierContracts) != 1 || report.VerifierContracts[0].ContractID != "product-safe-patch-import" {
		t.Fatalf("verifier contracts = %+v", report.VerifierContracts)
	}
	if len(report.VerifierResults) != 1 || report.VerifierResults[0].Status != "passed" {
		t.Fatalf("verifier results = %+v", report.VerifierResults)
	}
	if !strings.HasPrefix(report.ReportPath, rt.cfg.PromotionWorkspaceRoot) {
		t.Fatalf("report path %q is outside workspace root %q", report.ReportPath, rt.cfg.PromotionWorkspaceRoot)
	}
	workspaceRepo := filepath.Join(rt.cfg.PromotionWorkspaceRoot, sanitizeExportPart(candidateID), "repo")
	if mainHead := strings.TrimSpace(runGit(t, workspaceRepo, "rev-parse", "main")); mainHead != base {
		t.Fatalf("workspace main mutated before promotion: got %s want %s", mainHead, base)
	}
	if sourceHead := strings.TrimSpace(runGit(t, baseRepo, "rev-parse", "main")); sourceHead != base {
		t.Fatalf("source repo mutated by verification: got %s want %s", sourceHead, base)
	}
}

func prepareRuntimeLauncherCandidate(t *testing.T) (baseRepo, workerRepo, base, workerHead string, exportReport *shipper.ExportReport) {
	t.Helper()
	ctx := context.Background()

	baseRepo = runtimeInitRepo(t)
	runtimeWriteFile(t, filepath.Join(baseRepo, "README.md"), "runtime promotion queue test repo\n")
	runtimeWriteFile(t, filepath.Join(baseRepo, "frontend/src/lib/Launcher.svelte"), `<script>
  export let title = "Launcher";
</script>

<button>{title}</button>
`)
	runGit(t, baseRepo, "add", ".")
	runGit(t, baseRepo, "commit", "-m", "initial product shell")
	base = strings.TrimSpace(runGit(t, baseRepo, "rev-parse", "HEAD"))

	workerRepo = runtimeCloneRepoAtBase(t, baseRepo, base)
	runGit(t, workerRepo, "switch", "-c", "agent/run-vsuper-runtime-dogfood/background-vm")
	runtimeWriteFile(t, filepath.Join(workerRepo, "frontend/src/lib/Launcher.svelte"), `<script>
  export let title = "Launcher";
  const onboardingMarker = "launch-with-uploads-themes";
  const uploadTarget = "files-app-upload-ui";
  const themeTarget = "theme-editor-onboarding";
</script>

<button data-onboarding-marker={onboardingMarker} data-upload-target={uploadTarget} data-theme-target={themeTarget}>{title}</button>
`)
	runGit(t, workerRepo, "add", "frontend/src/lib/Launcher.svelte")
	runGit(t, workerRepo, "commit", "-m", "Add launcher uploads themes onboarding marker")
	workerHead = strings.TrimSpace(runGit(t, workerRepo, "rev-parse", "HEAD"))

	exportReport, err := shipper.ExportPatchset(ctx, shipper.ExportOptions{
		RepoPath:   workerRepo,
		OutputDir:  t.TempDir(),
		BaseSHA:    base,
		RunID:      "run-vsuper-runtime-dogfood",
		TraceID:    "trace-runtime-dogfood",
		VMID:       "vm-runtime-candidate",
		SnapshotID: "snapshot-runtime-candidate",
		Summary:    "Dogfood launcher/uploads/themes onboarding patch",
		Checks:     []string{"grep -q 'launch-with-uploads-themes' frontend/src/lib/Launcher.svelte"},
	})
	if err != nil {
		t.Fatalf("export patchset: %v", err)
	}
	return baseRepo, workerRepo, base, workerHead, exportReport
}

func runtimeInitRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	runGit(t, repo, "init", "-b", "main")
	runGit(t, repo, "config", "user.name", "Choir Runtime Test")
	runGit(t, repo, "config", "user.email", "choir-runtime-test@example.com")
	return repo
}

func runtimeCloneRepoAtBase(t *testing.T, source, base string) string {
	t.Helper()
	repo := filepath.Join(t.TempDir(), "clone")
	cmd := exec.Command("git", "clone", source, repo)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git clone: %s: %v", string(out), err)
	}
	runGit(t, repo, "config", "user.name", "Choir Runtime Test")
	runGit(t, repo, "config", "user.email", "choir-runtime-test@example.com")
	runGit(t, repo, "switch", "main")
	runGit(t, repo, "reset", "--hard", base)
	return repo
}

func runtimeWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func runtimeReadFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func runtimeCopyPromotionArtifacts(t *testing.T, rt *Runtime, candidateID, manifestPath, patchsetPath string) (string, string) {
	t.Helper()
	dir := filepath.Join(promotionArtifactRoot(rt), sanitizeExportPart(candidateID))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create promotion artifact dir: %v", err)
	}
	manifestOut := filepath.Join(dir, "manifest.json")
	patchsetOut := filepath.Join(dir, "changes.patch")
	runtimeCopyFile(t, manifestPath, manifestOut)
	runtimeCopyFile(t, patchsetPath, patchsetOut)
	return manifestOut, patchsetOut
}

func runtimeCopyFile(t *testing.T, src, dst string) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read %s: %v", src, err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", dst, err)
	}
}
