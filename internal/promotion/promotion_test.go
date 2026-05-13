package promotion

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/shipper"
)

func TestPrepareIntegrationCandidateVerifiesWithoutMutatingDestination(t *testing.T) {
	ctx := context.Background()
	baseRepo, workerRepo, base, workerHead, exportReport := prepareLauncherCandidate(t)
	integrationRepo := cloneRepoAtBase(t, baseRepo, base)
	reportPath := filepath.Join(t.TempDir(), "promotion-report.json")

	report, err := PrepareIntegrationCandidate(ctx, PrepareOptions{
		RepoPath:          integrationRepo,
		ManifestPath:      exportReport.ManifestPath,
		PatchsetPath:      exportReport.PatchsetPath,
		IntegrationBranch: "agent/run-cwp-v0/launcher-demo",
		DestinationBranch: "main",
		ReportPath:        reportPath,
		Candidate: candidateFixture(exportReport, CandidateWorld{
			CandidateID:         "candidate-launcher-demo",
			OwnerID:             "user-alice",
			ForegroundDesktopID: "primary",
			ParentRunID:         "run-parent",
			CandidateRunID:      "run-cwp-v0",
			VMID:                "vm-candidate-launcher",
			SnapshotID:          "snapshot-launcher",
			Purpose:             "Dogfood launcher product patch",
			BaseSHA:             base,
			WorkerHeadSHA:       workerHead,
		}),
		Contracts: []VerifierContract{{
			ContractID:              "launcher-marker",
			Target:                  "frontend/src/lib/Launcher.svelte",
			Purpose:                 "Prove the product-visible launcher patch exists in the integration candidate.",
			Invariants:              []string{"foreground main branch remains at base until explicit promotion"},
			RequiredChecks:          []string{"grep -q 'launch-with-uploads-themes' frontend/src/lib/Launcher.svelte"},
			CapabilityProfile:       "shell",
			IndependenceRequirement: "contract runs after patch import in a clean integration checkout",
			ResultSchema:            "all required checks exit zero",
			EvidencePaths:           []string{"frontend/src/lib/Launcher.svelte"},
		}},
	})
	if err != nil {
		t.Fatalf("PrepareIntegrationCandidate: %v", err)
	}
	if report.Status != "verified" || report.CanonicalMutated {
		t.Fatalf("unexpected report status/mutation: %+v", report)
	}
	if report.Candidate.WorkerHeadSHA != workerHead || report.Rollback.BaseSHA != base {
		t.Fatalf("missing rollback/candidate metadata: %+v", report)
	}
	if report.Integration.Branch != "agent/run-cwp-v0/launcher-demo" {
		t.Fatalf("integration branch = %q", report.Integration.Branch)
	}
	mainHead := strings.TrimSpace(git(t, integrationRepo, "rev-parse", "main"))
	if mainHead != base {
		t.Fatalf("main mutated before promotion: got %s want %s", mainHead, base)
	}
	integrationHead := strings.TrimSpace(git(t, integrationRepo, "rev-parse", "agent/run-cwp-v0/launcher-demo"))
	if integrationHead == base {
		t.Fatalf("integration branch did not receive candidate patch")
	}
	if _, err := os.Stat(reportPath); err != nil {
		t.Fatalf("promotion report not written: %v", err)
	}

	_ = workerRepo
}

func TestApplyVerifiedPromotionRequiresApprovalAndBlocksDivergence(t *testing.T) {
	ctx := context.Background()
	baseRepo, _, base, workerHead, exportReport := prepareLauncherCandidate(t)
	integrationRepo := cloneRepoAtBase(t, baseRepo, base)
	report, err := PrepareIntegrationCandidate(ctx, PrepareOptions{
		RepoPath:          integrationRepo,
		ManifestPath:      exportReport.ManifestPath,
		PatchsetPath:      exportReport.PatchsetPath,
		IntegrationBranch: "agent/run-cwp-v0/divergence",
		DestinationBranch: "main",
		Candidate: candidateFixture(exportReport, CandidateWorld{
			CandidateID:    "candidate-divergence",
			OwnerID:        "user-alice",
			CandidateRunID: "run-cwp-v0",
			VMID:           "vm-candidate-divergence",
			Purpose:        "Prove divergence blocks promotion",
			BaseSHA:        base,
			WorkerHeadSHA:  workerHead,
		}),
		Contracts: []VerifierContract{{
			ContractID:     "launcher-marker",
			Target:         "frontend/src/lib/Launcher.svelte",
			RequiredChecks: []string{"grep -q 'launch-with-uploads-themes' frontend/src/lib/Launcher.svelte"},
		}},
	})
	if err != nil {
		t.Fatalf("PrepareIntegrationCandidate: %v", err)
	}

	if _, err := ApplyVerifiedPromotion(ctx, integrationRepo, report, false); err == nil {
		t.Fatalf("expected approval error")
	}

	git(t, integrationRepo, "switch", "main")
	writeFile(t, filepath.Join(integrationRepo, "README.md"), "foreground user divergence\n")
	git(t, integrationRepo, "add", "README.md")
	git(t, integrationRepo, "commit", "-m", "foreground divergence")

	_, err = ApplyVerifiedPromotion(ctx, integrationRepo, report, true)
	if err == nil || !strings.Contains(err.Error(), "diverged") {
		t.Fatalf("expected divergence block, got %v", err)
	}
}

func TestCandidateWorldPromotionDogfoodsLauncherPatch(t *testing.T) {
	ctx := context.Background()
	baseRepo, _, base, workerHead, exportReport := prepareLauncherCandidate(t)
	integrationRepo := cloneRepoAtBase(t, baseRepo, base)
	reportPath := filepath.Join(t.TempDir(), "promotion-report.json")

	report, err := PrepareIntegrationCandidate(ctx, PrepareOptions{
		RepoPath:          integrationRepo,
		ManifestPath:      exportReport.ManifestPath,
		PatchsetPath:      exportReport.PatchsetPath,
		IntegrationBranch: "agent/run-cwp-v0/launcher-product-patch",
		DestinationBranch: "main",
		ReportPath:        reportPath,
		Candidate: candidateFixture(exportReport, CandidateWorld{
			CandidateID:         "candidate-product-launcher",
			OwnerID:             "user-alice",
			ForegroundDesktopID: "primary",
			ParentRunID:         "run-parent",
			CandidateRunID:      "run-cwp-v0",
			VMID:                "vm-branch-per-candidate",
			SnapshotID:          "snapshot-before-mutation",
			Purpose:             "Narrow Choir-in-Choir product patch for launcher/uploads/themes onboarding copy",
			BaseSHA:             base,
			WorkerHeadSHA:       workerHead,
		}),
		Contracts: []VerifierContract{{
			ContractID:              "launcher-product-patch",
			Target:                  "frontend/src/lib/Launcher.svelte",
			Purpose:                 "Verify the background VM candidate adds the launch-with-uploads-themes product marker.",
			Invariants:              []string{"patch is imported on agent branch first", "destination main moves only after explicit approval"},
			RequiredChecks:          []string{"grep -q 'launch-with-uploads-themes' frontend/src/lib/Launcher.svelte"},
			CapabilityProfile:       "shell",
			IndependenceRequirement: "verification happens in integration candidate after import",
			EvidencePaths:           []string{"frontend/src/lib/Launcher.svelte", exportReport.ManifestPath, exportReport.PatchsetPath},
		}},
	})
	if err != nil {
		t.Fatalf("PrepareIntegrationCandidate: %v", err)
	}
	if got := readFile(t, filepath.Join(integrationRepo, "frontend/src/lib/Launcher.svelte")); !strings.Contains(got, "launch-with-uploads-themes") {
		t.Fatalf("integration checkout missing launcher marker: %s", got)
	}

	git(t, integrationRepo, "switch", "main")
	if got := readFile(t, filepath.Join(integrationRepo, "frontend/src/lib/Launcher.svelte")); strings.Contains(got, "launch-with-uploads-themes") {
		t.Fatalf("main contains candidate marker before promotion: %s", got)
	}

	promoted, err := ApplyVerifiedPromotion(ctx, integrationRepo, report, true)
	if err != nil {
		t.Fatalf("ApplyVerifiedPromotion: %v", err)
	}
	if promoted.Status != "promoted" || !promoted.CanonicalMutated || !promoted.PromotionApproved {
		t.Fatalf("unexpected promoted report: %+v", promoted)
	}
	if got := readFile(t, filepath.Join(integrationRepo, "frontend/src/lib/Launcher.svelte")); !strings.Contains(got, "launch-with-uploads-themes") {
		t.Fatalf("promoted main missing launcher marker: %s", got)
	}

	var saved Report
	if err := json.Unmarshal([]byte(readFile(t, reportPath)), &saved); err != nil {
		t.Fatalf("decode saved report: %v", err)
	}
	if saved.Status != "promoted" || saved.Rollback.BaseSHA != base {
		t.Fatalf("saved report missing promotion/rollback evidence: %+v", saved)
	}
}

func prepareLauncherCandidate(t *testing.T) (baseRepo, workerRepo, base, workerHead string, exportReport *shipper.ExportReport) {
	t.Helper()
	ctx := context.Background()

	baseRepo = initRepo(t)
	writeFile(t, filepath.Join(baseRepo, "README.md"), "candidate world promotion test repo\n")
	writeFile(t, filepath.Join(baseRepo, "frontend/src/lib/Launcher.svelte"), `<script>
  export let title = "Launcher";
</script>

<button>{title}</button>
`)
	git(t, baseRepo, "add", ".")
	git(t, baseRepo, "commit", "-m", "initial product shell")
	base = strings.TrimSpace(git(t, baseRepo, "rev-parse", "HEAD"))

	workerRepo = cloneRepoAtBase(t, baseRepo, base)
	git(t, workerRepo, "switch", "-c", "agent/run-cwp-v0/background-vm")
	writeFile(t, filepath.Join(workerRepo, "frontend/src/lib/Launcher.svelte"), `<script>
  export let title = "Launcher";
  const onboardingMarker = "launch-with-uploads-themes";
</script>

<button data-onboarding-marker={onboardingMarker}>{title}</button>
`)
	git(t, workerRepo, "add", "frontend/src/lib/Launcher.svelte")
	git(t, workerRepo, "commit", "-m", "Add launcher onboarding marker")
	workerHead = strings.TrimSpace(git(t, workerRepo, "rev-parse", "HEAD"))

	exportDir := t.TempDir()
	report, err := shipper.ExportPatchset(ctx, shipper.ExportOptions{
		RepoPath:   workerRepo,
		OutputDir:  exportDir,
		BaseSHA:    base,
		RunID:      "run-cwp-v0",
		TraceID:    "trace-cwp-v0",
		VMID:       "vm-branch-per-candidate",
		SnapshotID: "snapshot-before-mutation",
		Summary:    "Dogfood launcher product patch",
		Checks:     []string{"grep -q 'launch-with-uploads-themes' frontend/src/lib/Launcher.svelte"},
	})
	if err != nil {
		t.Fatalf("ExportPatchset: %v", err)
	}
	return baseRepo, workerRepo, base, workerHead, report
}

func candidateFixture(exportReport *shipper.ExportReport, candidate CandidateWorld) CandidateWorld {
	candidate.ManifestPath = exportReport.ManifestPath
	candidate.PatchsetPath = exportReport.PatchsetPath
	if candidate.BaseSHA == "" {
		candidate.BaseSHA = exportReport.BaseSHA
	}
	if candidate.WorkerHeadSHA == "" {
		candidate.WorkerHeadSHA = exportReport.HeadSHA
	}
	return candidate
}

func initRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	git(t, repo, "init", "-b", "main")
	git(t, repo, "config", "user.name", "Choir Test")
	git(t, repo, "config", "user.email", "choir-test@example.com")
	return repo
}

func cloneRepoAtBase(t *testing.T, source, base string) string {
	t.Helper()
	repo := filepath.Join(t.TempDir(), "clone")
	cmd := exec.Command("git", "clone", source, repo)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git clone: %s: %v", string(out), err)
	}
	git(t, repo, "config", "user.name", "Choir Test")
	git(t, repo, "config", "user.email", "choir-test@example.com")
	git(t, repo, "switch", "main")
	git(t, repo, "reset", "--hard", base)
	return repo
}

func git(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %s: %v", strings.Join(args, " "), string(out), err)
	}
	return string(out)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
