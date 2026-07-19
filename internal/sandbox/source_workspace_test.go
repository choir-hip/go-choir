package sandbox

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/buildinfo"
)

func TestSourceWorkspaceUsesCompiledCommitBeforeDeploymentEnvironment(t *testing.T) {
	originalCommit := buildinfo.Commit
	buildinfo.Commit = "ffffffffffffffffffffffffffffffffffffffff"
	t.Cleanup(func() { buildinfo.Commit = originalCommit })
	t.Setenv("CHOIR_DEPLOYED_COMMIT", "stale-deploy-target")
	t.Setenv("RUNTIME_WORKER_REPO_BASE_SHA", "stale-runtime-target")

	projection := sourceWorkspaceProjection(t.TempDir(), t.TempDir(), SourceWorkspaceOptions{})
	if projection.PlatformBaseCommit != buildinfo.Commit {
		t.Fatalf("PlatformBaseCommit = %q, want compiled commit %q", projection.PlatformBaseCommit, buildinfo.Commit)
	}
}

func TestBootstrapSourceWorkspaceCreatesRootsAndLineage(t *testing.T) {
	root := t.TempDir()
	t.Setenv("RUNTIME_PROMOTION_SOURCE_REPO", "https://example.com/platform.git")
	t.Setenv("RUNTIME_SOURCE_LEDGER_REPO", "https://example.com/source-ledger.git")
	t.Setenv("RUNTIME_PROMOTION_WORKSPACE_ROOT", filepath.Join(root, "promotion-workspaces"))
	originalCommit := buildinfo.Commit
	buildinfo.Commit = "abc123"
	t.Cleanup(func() { buildinfo.Commit = originalCommit })

	projection, err := BootstrapSourceWorkspace(root, SourceWorkspaceOptions{
		ComputerID: "computer-1",
		OwnerID:    "owner@example.com",
		SessionID:  "zot-7",
	})
	if err != nil {
		t.Fatalf("BootstrapSourceWorkspace returned error: %v", err)
	}

	for _, dir := range []string{
		filepath.Join(root, "Source", "platform"),
		filepath.Join(root, "Source", "user"),
		filepath.Join(root, "Build"),
		filepath.Join(root, ".choir"),
	} {
		info, statErr := os.Stat(dir)
		if statErr != nil {
			t.Fatalf("expected directory %s: %v", dir, statErr)
		}
		if !info.IsDir() {
			t.Fatalf("%s is not a directory", dir)
		}
	}

	if projection.ComputerKind != "active" {
		t.Fatalf("ComputerKind = %q, want active", projection.ComputerKind)
	}
	if projection.OwnerID != "owner@example.com" {
		t.Fatalf("OwnerID = %q", projection.OwnerID)
	}
	if projection.PlatformBaseCommit != "abc123" {
		t.Fatalf("PlatformBaseCommit = %q, want abc123", projection.PlatformBaseCommit)
	}
	if projection.PlatformSourceMount != filepath.Join(root, "Source", "platform") {
		t.Fatalf("PlatformSourceMount = %q", projection.PlatformSourceMount)
	}

	raw, err := os.ReadFile(filepath.Join(root, ".choir", "source-lineage.json"))
	if err != nil {
		t.Fatalf("read lineage projection: %v", err)
	}
	var saved SourceWorkspaceProjection
	if err := json.Unmarshal(raw, &saved); err != nil {
		t.Fatalf("decode lineage projection: %v", err)
	}
	if saved.UserSourceMount != filepath.Join(root, "Source", "user") {
		t.Fatalf("saved UserSourceMount = %q", saved.UserSourceMount)
	}
	if saved.SourceLedgerRepo != "https://example.com/source-ledger.git" {
		t.Fatalf("saved SourceLedgerRepo = %q", saved.SourceLedgerRepo)
	}
}

func TestBootstrapSourceWorkspaceRefreshesOwnerForSuperConsoleSession(t *testing.T) {
	root := t.TempDir()
	if _, err := BootstrapSourceWorkspace(root, SourceWorkspaceOptions{
		ComputerID: "sandbox-m1",
	}); err != nil {
		t.Fatalf("initial bootstrap: %v", err)
	}

	projection, err := BootstrapSourceWorkspace(root, SourceWorkspaceOptions{
		ComputerID: "sandbox-m1",
		OwnerID:    "owner@example.com",
		SessionID:  "zot-2",
	})
	if err != nil {
		t.Fatalf("session bootstrap: %v", err)
	}
	if projection.OwnerID != "owner@example.com" {
		t.Fatalf("OwnerID = %q", projection.OwnerID)
	}
	if projection.SuperConsoleSessionID != "zot-2" {
		t.Fatalf("SuperConsoleSessionID = %q", projection.SuperConsoleSessionID)
	}
	if projection.ComputerKind != "active" {
		t.Fatalf("ComputerKind = %q, want active", projection.ComputerKind)
	}
}

func TestBootstrapSourceWorkspaceRejectsLegacyWorkerIdentity(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SANDBOX_ID", "vm-worker-123")
	t.Setenv("CHOIR_COMPUTER_KIND", "worker")
	if _, err := BootstrapSourceWorkspace(root, SourceWorkspaceOptions{}); err == nil || !strings.Contains(err.Error(), "unsupported computer kind") {
		t.Fatalf("legacy worker identity error = %v", err)
	}
}

func TestBootstrapSourceWorkspaceMaterializesPinnedPlatformCheckout(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	if _, err := runSourceGitCommand(root, "init", repo); err != nil {
		t.Fatalf("git init: %v", err)
	}
	if _, err := runSourceGitCommand(repo, "config", "user.email", "test@example.com"); err != nil {
		t.Fatalf("git config email: %v", err)
	}
	if _, err := runSourceGitCommand(repo, "config", "user.name", "Test User"); err != nil {
		t.Fatalf("git config name: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write repo file: %v", err)
	}
	if _, err := runSourceGitCommand(repo, "add", "README.md"); err != nil {
		t.Fatalf("git add: %v", err)
	}
	if _, err := runSourceGitCommand(repo, "commit", "-m", "init"); err != nil {
		t.Fatalf("git commit: %v", err)
	}
	commit, err := runSourceGitCommand(repo, "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("git rev-parse: %v", err)
	}
	commit = strings.TrimSpace(commit)

	filesRoot := filepath.Join(root, "files")
	t.Setenv("RUNTIME_PROMOTION_SOURCE_REPO", repo)
	originalCommit := buildinfo.Commit
	buildinfo.Commit = commit
	t.Cleanup(func() { buildinfo.Commit = originalCommit })
	projection, err := BootstrapSourceWorkspace(filesRoot, SourceWorkspaceOptions{
		ComputerID:              "sandbox-m1",
		MaterializeGitCheckouts: true,
	})
	if err != nil {
		t.Fatalf("BootstrapSourceWorkspace returned error: %v", err)
	}
	if projection.PlatformCheckoutStatus != "ok_platform_at_base" {
		t.Fatalf("PlatformCheckoutStatus = %q error=%q", projection.PlatformCheckoutStatus, projection.PlatformCheckoutError)
	}
	if projection.DirtyStateSummary != "clean" {
		t.Fatalf("DirtyStateSummary = %q, want clean", projection.DirtyStateSummary)
	}
	if _, err := os.Stat(filepath.Join(projection.PlatformSourceMount, "README.md")); err != nil {
		t.Fatalf("expected materialized README: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(filesRoot, ".choir", "source-lineage.json"))
	if err != nil {
		t.Fatalf("read lineage projection: %v", err)
	}
	if !strings.Contains(string(raw), `"platform_checkout_status": "ok_platform_at_base"`) ||
		!strings.Contains(string(raw), `"dirty_state_summary": "clean"`) {
		t.Fatalf("lineage missing checkout status summary: %s", string(raw))
	}
}

func TestSourceCheckoutDirtyStateSummary(t *testing.T) {
	tests := []struct {
		status string
		want   string
	}{
		{status: "ok_platform_at_base", want: "clean"},
		{status: "dirty_preserved", want: "dirty_preserved"},
		{status: "blocked_non_git_non_empty", want: "blocked"},
		{status: "checkout_failed", want: "failed"},
		{status: "not_configured", want: "not_inspected"},
	}
	for _, tt := range tests {
		if got := sourceCheckoutDirtyStateSummary(tt.status); got != tt.want {
			t.Fatalf("sourceCheckoutDirtyStateSummary(%q) = %q, want %q", tt.status, got, tt.want)
		}
	}
}
