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
