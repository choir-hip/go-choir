package buildinfo

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSnapshotReadsDeployMetadataFile(t *testing.T) {
	t.Setenv("CHOIR_DEPLOYED_AT", "")
	t.Setenv("CHOIR_DEPLOYED_COMMIT", "")

	path := filepath.Join(t.TempDir(), "deploy.env")
	if err := os.WriteFile(path, []byte("CHOIR_DEPLOYED_AT=2026-05-24T12:00:00Z\nCHOIR_DEPLOYED_COMMIT=abc123\n"), 0o644); err != nil {
		t.Fatalf("write deploy env: %v", err)
	}
	t.Setenv("CHOIR_DEPLOY_ENV_PATH", path)

	info := Snapshot("proxy")
	if info.Commit != "abc123" {
		t.Fatalf("Commit = %q", info.Commit)
	}
	if info.DeployedAt != "2026-05-24T12:00:00Z" {
		t.Fatalf("DeployedAt = %q", info.DeployedAt)
	}
	if info.DeployedCommit != "abc123" {
		t.Fatalf("DeployedCommit = %q", info.DeployedCommit)
	}
}

func TestSnapshotDeployMetadataFileOverridesStaleEnvironment(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deploy.env")
	if err := os.WriteFile(path, []byte("CHOIR_DEPLOYED_AT=file-time\nCHOIR_DEPLOYED_COMMIT=file-commit\n"), 0o644); err != nil {
		t.Fatalf("write deploy env: %v", err)
	}
	t.Setenv("CHOIR_DEPLOY_ENV_PATH", path)
	t.Setenv("CHOIR_DEPLOYED_AT", "env-time")
	t.Setenv("CHOIR_DEPLOYED_COMMIT", "env-commit")

	info := Snapshot("proxy")
	if info.Commit != "file-commit" {
		t.Fatalf("Commit = %q", info.Commit)
	}
	if info.DeployedAt != "file-time" {
		t.Fatalf("DeployedAt = %q", info.DeployedAt)
	}
	if info.DeployedCommit != "file-commit" {
		t.Fatalf("DeployedCommit = %q", info.DeployedCommit)
	}
}
