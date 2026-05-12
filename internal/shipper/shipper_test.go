package shipper

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportPatchsetAppliesManifestAndRunsChecks(t *testing.T) {
	ctx := context.Background()
	repo := initRepo(t)
	writeFile(t, filepath.Join(repo, "README.md"), "hello\n")
	git(t, repo, "add", "README.md")
	git(t, repo, "commit", "-m", "initial")
	base := strings.TrimSpace(git(t, repo, "rev-parse", "HEAD"))

	patch := filepath.Join(t.TempDir(), "change.patch")
	writeFile(t, patch, `diff --git a/README.md b/README.md
index ce01362..94954ab 100644
--- a/README.md
+++ b/README.md
@@ -1 +1,2 @@
 hello
+from worker
`)
	manifestPath := filepath.Join(t.TempDir(), "manifest.json")
	writeManifest(t, manifestPath, Manifest{
		RunID:         "run-123",
		TraceID:       "trace-456",
		VMID:          "vm-789",
		SnapshotID:    "snap-abc",
		BaseSHA:       base,
		Summary:       "Import worker change",
		ResidualRisks: []string{"test residual"},
	})
	reportPath := filepath.Join(t.TempDir(), "report.json")

	report, err := ImportPatchset(ctx, Options{
		RepoPath:     repo,
		ManifestPath: manifestPath,
		PatchsetPath: patch,
		Branch:       "agent/run-123/readme",
		Checks:       []string{"test -f README.md && grep -q 'from worker' README.md"},
		ReportPath:   reportPath,
	})
	if err != nil {
		t.Fatalf("ImportPatchset: %v", err)
	}
	if report.Status != "imported" {
		t.Fatalf("status = %q", report.Status)
	}
	if report.Branch != "agent/run-123/readme" {
		t.Fatalf("branch = %q", report.Branch)
	}
	if report.BaseSHA != base || report.HeadSHA == "" || report.HeadSHA == base {
		t.Fatalf("unexpected sha report: %+v", report)
	}
	if got := readFile(t, filepath.Join(repo, "README.md")); !strings.Contains(got, "from worker") {
		t.Fatalf("README missing patch: %q", got)
	}
	if got := git(t, repo, "log", "-1", "--pretty=%B"); !strings.Contains(got, "Choir-Run-ID: run-123") || !strings.Contains(got, "Choir-VM-ID: vm-789") {
		t.Fatalf("commit message missing provenance:\n%s", got)
	}
	if _, err := os.Stat(reportPath); err != nil {
		t.Fatalf("report not written: %v", err)
	}
}

func TestImportPatchsetRejectsDirtyRepo(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, filepath.Join(repo, "README.md"), "hello\n")
	git(t, repo, "add", "README.md")
	git(t, repo, "commit", "-m", "initial")
	base := strings.TrimSpace(git(t, repo, "rev-parse", "HEAD"))
	writeFile(t, filepath.Join(repo, "dirty.txt"), "dirty\n")

	manifestPath := filepath.Join(t.TempDir(), "manifest.json")
	writeManifest(t, manifestPath, Manifest{RunID: "run", TraceID: "trace", VMID: "vm", BaseSHA: base})
	patch := filepath.Join(t.TempDir(), "change.patch")
	writeFile(t, patch, "")

	_, err := ImportPatchset(context.Background(), Options{
		RepoPath:     repo,
		ManifestPath: manifestPath,
		PatchsetPath: patch,
		Branch:       "agent/run/test",
	})
	if err == nil || !strings.Contains(err.Error(), "clean") {
		t.Fatalf("expected clean repo error, got %v", err)
	}
}

func TestValidateBranchRejectsNonAgentBranches(t *testing.T) {
	for _, branch := range []string{"main", "agent/run", "agent/../bad/x", "agent/run/bad lock"} {
		if err := validateBranch(branch); err == nil {
			t.Fatalf("expected branch %q to be rejected", branch)
		}
	}
	if err := validateBranch("agent/run-123/feature_1"); err != nil {
		t.Fatalf("expected safe branch: %v", err)
	}
}

func initRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	git(t, repo, "init")
	git(t, repo, "config", "user.name", "Choir Test")
	git(t, repo, "config", "user.email", "choir-test@example.com")
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

func writeManifest(t *testing.T, path string, manifest Manifest) {
	t.Helper()
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	writeFile(t, path, string(data))
}
