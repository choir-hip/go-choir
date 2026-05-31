package sandbox

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBootstrapSourceWorkspaceCreatesRootsAndLineage(t *testing.T) {
	root := t.TempDir()
	t.Setenv("RUNTIME_PROMOTION_SOURCE_REPO", "https://example.com/platform.git")
	t.Setenv("RUNTIME_SOURCE_LEDGER_REPO", "https://example.com/source-ledger.git")
	t.Setenv("RUNTIME_PROMOTION_WORKSPACE_ROOT", filepath.Join(root, "promotion-workspaces"))
	t.Setenv("RUNTIME_WORKER_REPO_BASE_SHA", "abc123")

	projection, err := BootstrapSourceWorkspace(root, SourceWorkspaceOptions{
		ComputerID: "candidate-computer-1",
		OwnerID:    "owner@example.com",
		SessionID:  "zot-7",
	})
	if err != nil {
		t.Fatalf("BootstrapSourceWorkspace returned error: %v", err)
	}

	for _, dir := range []string{
		filepath.Join(root, "Source", "platform"),
		filepath.Join(root, "Source", "user"),
		filepath.Join(root, "Source", "candidate"),
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

	if projection.ComputerKind != "candidate" {
		t.Fatalf("ComputerKind = %q, want candidate", projection.ComputerKind)
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
	if projection.CandidateSourceRef == "" {
		t.Fatalf("CandidateSourceRef should be populated for candidate computers")
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

func TestBootstrapSourceWorkspaceUsesGuestIdentityEnv(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SANDBOX_ID", "vm-worker-123")
	t.Setenv("CHOIR_COMPUTER_KIND", "worker")
	t.Setenv("CHOIR_OWNER_ID", "owner@example.com")
	t.Setenv("CHOIR_DESKTOP_ID", "primary")
	t.Setenv("CHOIR_WORKER_ID", "worker-abc")

	projection, err := BootstrapSourceWorkspace(root, SourceWorkspaceOptions{})
	if err != nil {
		t.Fatalf("BootstrapSourceWorkspace returned error: %v", err)
	}
	if projection.ComputerID != "vm-worker-123" {
		t.Fatalf("ComputerID = %q", projection.ComputerID)
	}
	if projection.ComputerKind != "worker" {
		t.Fatalf("ComputerKind = %q", projection.ComputerKind)
	}
	if projection.OwnerID != "owner@example.com" {
		t.Fatalf("OwnerID = %q", projection.OwnerID)
	}
	if projection.DesktopID != "primary" {
		t.Fatalf("DesktopID = %q", projection.DesktopID)
	}
	if !strings.Contains(projection.CandidateSourceRef, "/candidates/worker-abc") {
		t.Fatalf("CandidateSourceRef = %q", projection.CandidateSourceRef)
	}
}

func TestBootstrapSourceWorkspaceMaterializesPlatformAndCandidateCheckouts(t *testing.T) {
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
	t.Setenv("RUNTIME_WORKER_REPO_BASE_SHA", commit)
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
	if projection.CandidateCheckoutStatus != "ok_candidate_at_base" {
		t.Fatalf("CandidateCheckoutStatus = %q error=%q", projection.CandidateCheckoutStatus, projection.CandidateCheckoutError)
	}
	for _, checkout := range []string{projection.PlatformSourceMount, projection.CandidateSourceMount} {
		if _, err := os.Stat(filepath.Join(checkout, "README.md")); err != nil {
			t.Fatalf("expected materialized README in %s: %v", checkout, err)
		}
	}
	candidateBranch, err := runSourceGitCommand(projection.CandidateSourceMount, "branch", "--show-current")
	if err != nil {
		t.Fatalf("candidate branch: %v", err)
	}
	if strings.TrimSpace(candidateBranch) != "choir-candidate" {
		t.Fatalf("candidate branch = %q, want choir-candidate", strings.TrimSpace(candidateBranch))
	}

	raw, err := os.ReadFile(filepath.Join(filesRoot, ".choir", "source-lineage.json"))
	if err != nil {
		t.Fatalf("read lineage projection: %v", err)
	}
	if !strings.Contains(string(raw), `"platform_checkout_status": "ok_platform_at_base"`) ||
		!strings.Contains(string(raw), `"candidate_checkout_status": "ok_candidate_at_base"`) {
		t.Fatalf("lineage missing checkout statuses: %s", string(raw))
	}
}

func TestBootstrapSourceWorkspacePreservesDirtyCandidateCheckout(t *testing.T) {
	root := t.TempDir()
	platform := filepath.Join(root, "files", "Source", "platform")
	candidate := filepath.Join(root, "files", "Source", "candidate")
	if err := os.MkdirAll(platform, 0o755); err != nil {
		t.Fatalf("mkdir platform: %v", err)
	}
	if err := os.WriteFile(filepath.Join(platform, "sentinel.txt"), []byte("do not clone here\n"), 0o644); err != nil {
		t.Fatalf("write platform sentinel: %v", err)
	}
	if err := os.MkdirAll(candidate, 0o755); err != nil {
		t.Fatalf("mkdir candidate: %v", err)
	}
	if _, err := runSourceGitCommand(root, "init", candidate); err != nil {
		t.Fatalf("git init candidate: %v", err)
	}
	if err := os.WriteFile(filepath.Join(candidate, "local.txt"), []byte("local edit\n"), 0o644); err != nil {
		t.Fatalf("write local edit: %v", err)
	}
	t.Setenv("RUNTIME_PROMOTION_SOURCE_REPO", "https://example.com/repo.git")

	projection, err := BootstrapSourceWorkspace(filepath.Join(root, "files"), SourceWorkspaceOptions{
		MaterializeGitCheckouts: true,
	})
	if err != nil {
		t.Fatalf("BootstrapSourceWorkspace returned error: %v", err)
	}
	if projection.CandidateCheckoutStatus != "dirty_preserved" {
		t.Fatalf("CandidateCheckoutStatus = %q", projection.CandidateCheckoutStatus)
	}
	if _, err := os.Stat(filepath.Join(candidate, "local.txt")); err != nil {
		t.Fatalf("dirty candidate file was not preserved: %v", err)
	}
}
