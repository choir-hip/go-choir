package projectionbase

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
)

func TestPublisherAtomicFsyncAndUnpack(t *testing.T) {
	artifactsRoot := t.TempDir()
	srcDir := t.TempDir()

	// Populate test files in srcDir.
	if err := os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("content of file 1"), 0o600); err != nil {
		t.Fatal(err)
	}
	subDir := filepath.Join(srcDir, "subdir")
	if err := os.MkdirAll(subDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "nested.bin"), []byte{0x01, 0x02, 0x03, 0x04}, 0o600); err != nil {
		t.Fatal(err)
	}

	publisher := NewPublisher(artifactsRoot)
	blobSHA256, blobSize, err := publisher.PublishDir(srcDir)
	if err != nil {
		t.Fatalf("PublishDir: %v", err)
	}

	if blobSHA256 == "" || blobSize <= 0 {
		t.Fatalf("invalid publish output: sha256=%s, size=%d", blobSHA256, blobSize)
	}

	blobPath := filepath.Join(artifactsRoot, "sha256", Namespace, blobSHA256)
	if _, err := os.Stat(blobPath); err != nil {
		t.Fatalf("published blob does not exist at %s: %v", blobPath, err)
	}

	// Unpack into a clean target directory.
	dstDir := t.TempDir()
	if err := Unpack(blobPath, dstDir); err != nil {
		t.Fatalf("Unpack: %v", err)
	}

	// Verify unpacked files match original.
	content1, err := os.ReadFile(filepath.Join(dstDir, "file1.txt"))
	if err != nil || string(content1) != "content of file 1" {
		t.Fatalf("unpacked file1 mismatch: %q, %v", content1, err)
	}

	content2, err := os.ReadFile(filepath.Join(dstDir, "subdir", "nested.bin"))
	if err != nil || len(content2) != 4 || content2[0] != 0x01 {
		t.Fatalf("unpacked nested.bin mismatch: %v, %v", content2, err)
	}
}

func TestRebuilderReplaysAndPublishesBase(t *testing.T) {
	ctx := context.Background()
	artifactsRoot := t.TempDir()
	computerID := "computer-rebuild-test"

	// Generate key material.
	keyMaterial := make([]byte, 32)
	for i := range keyMaterial {
		keyMaterial[i] = byte(i + 1)
	}

	eventsDir := filepath.Join(artifactsRoot, "sha256", "computer-event")
	if err := os.MkdirAll(eventsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	testCommitment := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	eventID, err := computerevent.NewEventID()
	if err != nil {
		t.Fatal(err)
	}

	event1 := computerevent.Event{
		SchemaVersion:                    computerevent.SchemaVersionV1,
		EventID:                          eventID,
		ComputerID:                       computerID,
		Sequence:                         1,
		PreviousHead:                     computerevent.ZeroHead,
		EventKind:                        computerevent.EventGenesisImported,
		OccurredAt:                       time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		IdempotencyKey:                   "idem-1",
		ActorProfile:                     "trusted-core",
		AuthorityRef:                     "authority:test",
		PayloadCommitment:                testCommitment,
		PrivacyClass:                     "public",
		ReducerVersion:                   1,
		ExpectedDesiredEventHead:         computerevent.ZeroHead,
		ExpectedEffectiveEventHead:       computerevent.ZeroHead,
		ExpectedDesiredStateCommitment:   computerevent.ZeroHead,
		ExpectedEffectiveStateCommitment: computerevent.ZeroHead,
		ResultingEffectiveCommitment:     testCommitment,
	}

	// Compute pin and request commitments for event validation.
	input := computerevent.TransitionInput{TargetStateCommitment: testCommitment}
	pinIntent, err := computerevent.ComputePinIntentCommitment(event1, input)
	if err != nil {
		t.Fatal(err)
	}
	reqCommitment, err := computerevent.ComputeRequestCommitment(event1, input, pinIntent, nil)
	if err != nil {
		t.Fatal(err)
	}
	event1.RequestCommitment = reqCommitment

	event1JSON, err := event1.CanonicalBytes()
	if err != nil {
		t.Fatal(err)
	}
	digest1, err := event1.Digest()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(eventsDir, digest1), event1JSON, 0o600); err != nil {
		t.Fatal(err)
	}
	diskSource := NewDiskEventSource(artifactsRoot, computerID)

	scratchDir := t.TempDir()
	cfg := Config{
		ComputerID:     computerID,
		TargetHead:     digest1,
		ArtifactsRoot:  artifactsRoot,
		ScratchDir:     scratchDir,
		KeyMaterial:    keyMaterial,
		BatchSize:      100,
		MemoryLimitRSS: 512 * 1024 * 1024,
	}

	rebuilder, err := NewRebuilder(cfg)
	if err != nil {
		t.Fatalf("NewRebuilder: %v", err)
	}

	result, err := rebuilder.Run(ctx, diskSource)
	if err != nil {
		t.Fatalf("Rebuilder.Run: %v", err)
	}

	if result.Descriptor.ComputerID != computerID {
		t.Fatalf("Descriptor ComputerID = %s, want %s", result.Descriptor.ComputerID, computerID)
	}
	if result.Descriptor.CanonicalHead != digest1 {
		t.Fatalf("Descriptor CanonicalHead = %s, want %s", result.Descriptor.CanonicalHead, digest1)
	}
	if result.Descriptor.Sequence != 1 {
		t.Fatalf("Descriptor Sequence = %d, want 1", result.Descriptor.Sequence)
	}

	if _, err := os.Stat(result.BlobPath); err != nil {
		t.Fatalf("result blob does not exist at %s: %v", result.BlobPath, err)
	}

	// Verify unpacking the blob reconstructs the database that can be opened.
	testUnpackDir := t.TempDir()
	if err := Unpack(result.BlobPath, testUnpackDir); err != nil {
		t.Fatalf("Unpack result blob: %v", err)
	}

	reopenedStore, err := choirstore.Open(filepath.Join(testUnpackDir, "runtime.db"))
	if err != nil {
		t.Fatalf("Open unpacked store: %v", err)
	}
	defer reopenedStore.Close()

	head, err := reopenedStore.Head(ctx, computerID)
	if err != nil || head == nil {
		t.Fatalf("read head from unpacked store: %v", err)
	}
	if head.CanonicalEventHead != digest1 || head.Sequence != 1 {
		t.Fatalf("unpacked store head = (%d, %s), want (1, %s)", head.Sequence, head.CanonicalEventHead, digest1)
	}
}

func TestRebuilderRejectsMismatchedHeadOrInvalidConfig(t *testing.T) {
	ctx := context.Background()
	artifactsRoot := t.TempDir()
	computerID := "computer-invalid-test"

	invalidCfg := Config{
		ComputerID:    computerID,
		TargetHead:    "",
		ArtifactsRoot: artifactsRoot,
	}
	if _, err := NewRebuilder(invalidCfg); err == nil {
		t.Fatal("expected error for empty target head")
	}

	keyMaterial := make([]byte, 32)
	rand.Read(keyMaterial)

	cfg := Config{
		ComputerID:    computerID,
		TargetHead:    "0000000000000000000000000000000000000000000000000000000000000000",
		ArtifactsRoot: artifactsRoot,
		ScratchDir:    t.TempDir(),
		KeyMaterial:   keyMaterial,
	}

	rebuilder, err := NewRebuilder(cfg)
	if err != nil {
		t.Fatal(err)
	}

	diskSource := NewDiskEventSource(artifactsRoot, computerID)
	if _, err := rebuilder.Run(ctx, diskSource); err == nil {
		t.Fatal("expected error when target head is not reached on disk")
	}
}

func dummyKey() ed25519.PrivateKey {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	return priv
}
