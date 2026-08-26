package recovery

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/filecas"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
)

func TestRecoveryCapsuleBuildAndVerify(t *testing.T) {
	computerID := "computer-capsule-test-1"
	eventHead := stringsRepeatHex("a", 64)
	keyEscrowDigest := stringsRepeatHex("b", 64)

	manifest, err := filecas.BuildManifest(computerID, []filecas.FileEntry{{
		Path:   "app/config.json",
		Mode:   0o644,
		Size:   12,
		Chunks: []string{stringsRepeatHex("c", 64)},
	}}, time.Now().UTC())
	if err != nil {
		t.Fatalf("BuildManifest failed: %v", err)
	}

	witness := selfdevprotocol.VMLocalContentWitness{
		Database:           "choir",
		ContentRoot:        stringsRepeatHex("d", 64),
		DerivabilityDigest: stringsRepeatHex("e", 64),
		Schema:             map[string]string{"agents": stringsRepeatHex("f", 64)},
		Tables:             map[string]string{"agents": stringsRepeatHex("1", 64)},
	}

	capsule, err := BuildCapsule(
		computerID,
		eventHead,
		1000,
		manifest.Root,
		manifest,
		stringsRepeatHex("2", 64),
		keyEscrowDigest,
		witness,
		time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("BuildCapsule failed: %v", err)
	}

	if capsule.CapsuleDigest == "" {
		t.Fatalf("expected non-empty CapsuleDigest")
	}

	if err := capsule.Verify(); err != nil {
		t.Fatalf("Verify failed: %v", err)
	}

	// Corrupting event head should fail verification
	capsule.EventHead = stringsRepeatHex("0", 64)
	if err := capsule.Verify(); err == nil {
		t.Fatalf("expected verification to fail after event head tampering")
	}
}

func TestDrillRunner(t *testing.T) {
	computerID := "computer-capsule-test-2"
	eventHead := stringsRepeatHex("a", 64)
	keyEscrowDigest := stringsRepeatHex("b", 64)

	manifest, err := filecas.BuildManifest(computerID, []filecas.FileEntry{{
		Path:   "test.txt",
		Mode:   0o600,
		Size:   4,
		Chunks: []string{stringsRepeatHex("c", 64)},
	}}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}

	witness := selfdevprotocol.VMLocalContentWitness{
		Database:           "choir",
		ContentRoot:        stringsRepeatHex("d", 64),
		DerivabilityDigest: stringsRepeatHex("e", 64),
		Schema:             map[string]string{"t": stringsRepeatHex("f", 64)},
		Tables:             map[string]string{"t": stringsRepeatHex("1", 64)},
	}

	capsule, err := BuildCapsule(
		computerID,
		eventHead,
		500,
		manifest.Root,
		manifest,
		"",
		keyEscrowDigest,
		witness,
		time.Now().UTC(),
	)
	if err != nil {
		t.Fatal(err)
	}

	mockExecutor := func(ctx context.Context, cap *RecoveryCapsule) (int, error) {
		time.Sleep(10 * time.Millisecond) // Simulate restore work
		return len(cap.FileManifest.Files), nil
	}

	runner := NewDrillRunner(mockExecutor)
	receipt, err := runner.RunDrill(context.Background(), "drill-001", capsule)
	if err != nil {
		t.Fatalf("RunDrill failed: %v", err)
	}

	if !receipt.Success || receipt.RestoredFiles != 1 || receipt.RTOSeconds <= 0 {
		t.Fatalf("unexpected drill receipt: %+v", receipt)
	}
}

func TestIntegrityScrubber(t *testing.T) {
	artifactsRoot := t.TempDir()
	nsDir := filepath.Join(artifactsRoot, "sha256", "file-cas-chunks")
	if err := os.MkdirAll(nsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	blobContent := []byte("valid artifact blob")
	hasher := sha256.New()
	hasher.Write(blobContent)
	digest := hex.EncodeToString(hasher.Sum(nil))

	blobPath := filepath.Join(nsDir, digest)
	if err := os.WriteFile(blobPath, blobContent, 0o644); err != nil {
		t.Fatal(err)
	}

	scrubber := NewIntegrityScrubber(artifactsRoot)
	report, err := scrubber.ScrubNamespaces(context.Background(), []string{"file-cas-chunks"})
	if err != nil {
		t.Fatalf("ScrubNamespaces failed: %v", err)
	}

	if report.ScannedBlobs != 1 || report.CorruptedBlobs != 0 || report.ScannedBytes != int64(len(blobContent)) {
		t.Fatalf("unexpected scrub report: %+v", report)
	}

	// Corrupt the blob
	if err := os.WriteFile(blobPath, []byte("corrupted"), 0o644); err != nil {
		t.Fatal(err)
	}

	report2, err := scrubber.ScrubNamespaces(context.Background(), []string{"file-cas-chunks"})
	if err != nil {
		t.Fatalf("ScrubNamespaces on corrupted failed: %v", err)
	}
	if report2.CorruptedBlobs != 1 {
		t.Fatalf("expected 1 corrupted blob, got %d", report2.CorruptedBlobs)
	}
}

func stringsRepeatHex(char string, count int) string {
	res := make([]byte, count)
	for i := range res {
		res[i] = char[0]
	}
	return string(res)
}
