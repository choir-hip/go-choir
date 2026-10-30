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

// writeReceipt points CHOIR_DEPLOY_RECEIPT_PATH at a receipt with the given
// body for the rest of the test.
func writeReceipt(t *testing.T, body string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "deploy-receipt.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write deploy receipt: %v", err)
	}
	t.Setenv("CHOIR_DEPLOY_RECEIPT_PATH", path)
}

// A partial deploy that did not move the proxy carries the proxy's artifact
// forward with its own commit. /health must still answer the platform
// commit (docs/archive/problems/partial-deploy-receipt-blanks-deployed-commit-2026-10-14.md,
// M3/M6), while the proxy's per-service identity stays strict: it was not
// deployed by this receipt's deploy.
func TestPlatformSnapshotReadsMergedReceiptAfterPartialDeploy(t *testing.T) {
	const target = "1111111111111111111111111111111111111111"
	const carried = "2222222222222222222222222222222222222222"
	writeReceipt(t, `{"schema_version":1,"target_commit":"`+target+`","deployed_commit":"`+target+`","activated_at":"2026-10-14T19:39:00Z",`+
		`"artifacts":{"vmctl":{"commit":"`+target+`","status":"active"},"proxy":{"commit":"`+carried+`","status":"active","carried_forward_from":"`+carried+`"}}}`)

	platform := PlatformSnapshot("proxy")
	if platform.DeployedCommit != target || platform.DeployedAt != "2026-10-14T19:39:00Z" || platform.Source != PlatformSourceReceipt {
		t.Fatalf("PlatformSnapshot = %+v, want receipt platform commit %s", platform, target)
	}
	if info := Snapshot("proxy"); info.DeployedCommit != "" || info.DeployedAt != "" {
		t.Fatalf("proxy per-service identity = %+v, want empty: a carried artifact is not this deploy's", info)
	}
	if info := Snapshot("vmctl"); info.DeployedCommit != target {
		t.Fatalf("vmctl per-service DeployedCommit = %q, want %q", info.DeployedCommit, target)
	}
}

// An old receipt (no platform field) falls back to the per-service rule.
func TestPlatformSnapshotFallsBackToServiceArtifactForOldReceipts(t *testing.T) {
	const target = "3333333333333333333333333333333333333333"
	writeReceipt(t, `{"schema_version":1,"target_commit":"`+target+`","activated_at":"2026-10-14T19:39:00Z","artifacts":{"proxy":{"commit":"`+target+`","status":"active"}}}`)
	platform := PlatformSnapshot("proxy")
	if platform.DeployedCommit != target || platform.Source != PlatformSourceServiceArtifact {
		t.Fatalf("PlatformSnapshot = %+v, want per-service fallback %s", platform, target)
	}

	writeReceipt(t, `{"schema_version":1,"target_commit":"`+target+`","activated_at":"2026-10-14T19:39:00Z","artifacts":{"vmctl":{"commit":"`+target+`","status":"active"}}}`)
	if platform := PlatformSnapshot("proxy"); platform != (Platform{}) {
		t.Fatalf("PlatformSnapshot = %+v, want empty for an old partial receipt without the proxy", platform)
	}
}

// A receipt whose platform field conflicts with its target is not trusted
// for either answer (R2).
func TestReceiptWithConflictingPlatformCommitReportsNothing(t *testing.T) {
	const target = "4444444444444444444444444444444444444444"
	const other = "5555555555555555555555555555555555555555"
	writeReceipt(t, `{"schema_version":1,"target_commit":"`+target+`","deployed_commit":"`+other+`","activated_at":"2026-10-14T19:39:00Z","artifacts":{"proxy":{"commit":"`+target+`","status":"active"}}}`)
	if platform := PlatformSnapshot("proxy"); platform != (Platform{}) {
		t.Fatalf("PlatformSnapshot = %+v, want empty for a conflicting receipt", platform)
	}
	if info := Snapshot("proxy"); info.DeployedCommit != "" {
		t.Fatalf("Snapshot DeployedCommit = %q, want empty for a conflicting receipt", info.DeployedCommit)
	}
}

func TestPlatformSnapshotWithoutReceiptIsEmpty(t *testing.T) {
	t.Setenv("CHOIR_DEPLOY_RECEIPT_PATH", filepath.Join(t.TempDir(), "missing.json"))
	if platform := PlatformSnapshot("proxy"); platform != (Platform{}) {
		t.Fatalf("PlatformSnapshot = %+v, want empty without a receipt", platform)
	}
}
