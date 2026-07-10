package buildinfo

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSnapshotReadsActivationReceipt(t *testing.T) {
	originalCommit := Commit
	Commit = "compiled-commit"
	t.Cleanup(func() { Commit = originalCommit })

	path := filepath.Join(t.TempDir(), "deploy-receipt.json")
	const commit = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if err := os.WriteFile(path, []byte(`{"schema_version":1,"target_commit":"`+commit+`","activated_at":"2026-05-24T12:00:00Z","artifacts":{"proxy":{"commit":"`+commit+`","status":"active"}}}`), 0o644); err != nil {
		t.Fatalf("write deploy receipt: %v", err)
	}
	t.Setenv("CHOIR_DEPLOY_RECEIPT_PATH", path)

	info := Snapshot("proxy")
	if info.Commit != "compiled-commit" {
		t.Fatalf("Commit = %q", info.Commit)
	}
	if info.DeployedAt != "2026-05-24T12:00:00Z" {
		t.Fatalf("DeployedAt = %q", info.DeployedAt)
	}
	if info.DeployedCommit != commit {
		t.Fatalf("DeployedCommit = %q", info.DeployedCommit)
	}
}

func TestSnapshotDoesNotTreatSelectedTargetEnvironmentAsActivation(t *testing.T) {
	originalCommit := Commit
	Commit = "compiled-commit"
	t.Cleanup(func() { Commit = originalCommit })

	t.Setenv("CHOIR_DEPLOY_RECEIPT_PATH", filepath.Join(t.TempDir(), "missing-receipt.json"))
	t.Setenv("CHOIR_DEPLOYED_AT", "env-time")
	t.Setenv("CHOIR_DEPLOYED_COMMIT", "env-commit")

	info := Snapshot("proxy")
	if info.Commit != "compiled-commit" {
		t.Fatalf("Commit = %q", info.Commit)
	}
	if info.DeployedAt != "" {
		t.Fatalf("DeployedAt = %q, want empty without activation receipt", info.DeployedAt)
	}
	if info.DeployedCommit != "" {
		t.Fatalf("DeployedCommit = %q, want empty without activation receipt", info.DeployedCommit)
	}
}

func TestSnapshotIgnoresMalformedActivationReceipt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deploy-receipt.json")
	if err := os.WriteFile(path, []byte(`{"target_commit":`), 0o644); err != nil {
		t.Fatalf("write malformed deploy receipt: %v", err)
	}
	t.Setenv("CHOIR_DEPLOY_RECEIPT_PATH", path)

	info := Snapshot("proxy")
	if info.DeployedAt != "" || info.DeployedCommit != "" {
		t.Fatalf("malformed receipt produced deployment metadata: %+v", info)
	}
}

func TestSnapshotRejectsReceiptWithoutVerifiedArtifacts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deploy-receipt.json")
	const commit = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	if err := os.WriteFile(path, []byte(`{"schema_version":1,"target_commit":"`+commit+`","activated_at":"2026-05-24T12:00:00Z","artifacts":{}}`), 0o644); err != nil {
		t.Fatalf("write empty deploy receipt: %v", err)
	}
	t.Setenv("CHOIR_DEPLOY_RECEIPT_PATH", path)

	info := Snapshot("proxy")
	if info.DeployedAt != "" || info.DeployedCommit != "" {
		t.Fatalf("empty receipt produced deployment metadata: %+v", info)
	}
}

func TestSnapshotRejectsReceiptWithMismatchedArtifactCommit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deploy-receipt.json")
	const target = "cccccccccccccccccccccccccccccccccccccccc"
	const artifact = "dddddddddddddddddddddddddddddddddddddddd"
	if err := os.WriteFile(path, []byte(`{"schema_version":1,"target_commit":"`+target+`","activated_at":"2026-05-24T12:00:00Z","artifacts":{"proxy":{"commit":"`+artifact+`","status":"active"}}}`), 0o644); err != nil {
		t.Fatalf("write mismatched deploy receipt: %v", err)
	}
	t.Setenv("CHOIR_DEPLOY_RECEIPT_PATH", path)

	info := Snapshot("proxy")
	if info.DeployedAt != "" || info.DeployedCommit != "" {
		t.Fatalf("mismatched receipt produced deployment metadata: %+v", info)
	}
}

func TestSnapshotDoesNotLeakCrossServiceDeploymentIdentity(t *testing.T) {
	originalCommit := Commit
	Commit = "compiled-commit"
	t.Cleanup(func() { Commit = originalCommit })

	path := filepath.Join(t.TempDir(), "deploy-receipt.json")
	const commit = "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	receipt := `{"schema_version":1,"target_commit":"` + commit + `","activated_at":"2026-05-24T12:00:00Z","artifacts":{"frontend":{"commit":"` + commit + `","status":"active"}}}`
	if err := os.WriteFile(path, []byte(receipt), 0o644); err != nil {
		t.Fatalf("write deploy receipt: %v", err)
	}
	t.Setenv("CHOIR_DEPLOY_RECEIPT_PATH", path)

	// proxy service is absent from the receipt; it must not inherit
	// frontend's deployment identity.
	info := Snapshot("proxy")
	if info.DeployedCommit != "" {
		t.Fatalf("DeployedCommit = %q, want empty: proxy leaked frontend artifact identity", info.DeployedCommit)
	}
	if info.DeployedAt != "" {
		t.Fatalf("DeployedAt = %q, want empty: proxy leaked frontend activation time", info.DeployedAt)
	}

	// frontend service IS in the receipt; it should report deployment.
	frontendInfo := Snapshot("frontend")
	if frontendInfo.DeployedCommit != commit {
		t.Fatalf("frontend DeployedCommit = %q, want %q", frontendInfo.DeployedCommit, commit)
	}
}
