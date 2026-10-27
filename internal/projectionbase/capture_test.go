package projectionbase

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
)

// A projection base captured from a running computer's data image (S2 of
// docs/problems/paper-never-checkpointed-2026-10-27.md). Ways it can fail,
// written before the code:
//  1. the captured store's head is not on the tape (a foreign or forked
//     store, a guest ahead of an unacknowledged append): a base at a head the
//     tape never had;
//  2. a mismatch is found after the blob or its descriptor is published: a
//     half-published base the seeding scan or an install adopts;
//  3. guest files beside the store travel inside the blob;
//  4. the base is not marked as captured, so a host verdict calls a live
//     witness "derived from the tape";
//  5. a rebuild seeded from a captured base drops the mark;
//  6. the witness is taken before compaction and no longer matches the bytes;
//  7. the publication fills the host disk.

// captureFixture builds an n-event chain in root and lays a store at
// sequence `at` out as the guest data image holds it: <mount>/state and
// <mount>/state.texture, beside other guest files.
func captureFixture(t *testing.T, computerID string, n, at int) (root, mount string, digests []string, replayed Descriptor) {
	t.Helper()
	ctx := context.Background()
	root = t.TempDir()
	digests = buildChainN(t, root, computerID, n)
	fixtureRoot := t.TempDir()
	result := runRebuild(t, ctx, Config{
		ComputerID: computerID, TargetHead: digests[at-1], ArtifactsRoot: fixtureRoot,
		ScratchDir: t.TempDir(), KeyMaterial: fixtureKey(40), BatchSize: 100,
		MemoryLimitRSS: 512 * 1024 * 1024, SeedPolicy: SeedNone, TargetSequence: uint64(at),
	}, fixtureReplay(NewDiskEventSource(root, computerID, digests[at-1])))
	mount = t.TempDir()
	unpacked := t.TempDir()
	if err := Unpack(result.BlobPath, unpacked); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(unpacked, "runtime.db"), filepath.Join(mount, "state")); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(unpacked, "runtime.texture"), filepath.Join(mount, "state.texture")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(mount, "files"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mount, "files", "owner-note.txt"), []byte("not part of the store"), 0o600); err != nil {
		t.Fatal(err)
	}
	return root, mount, digests, result.Descriptor
}

func tapeOf(digests []string) func(context.Context, string, uint64) (string, error) {
	return func(_ context.Context, _ string, sequence uint64) (string, error) {
		if sequence == 0 || sequence > uint64(len(digests)) {
			return "", fmt.Errorf("no event at %d", sequence)
		}
		return digests[sequence-1], nil
	}
}

func publishedBaseFiles(t *testing.T, root string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(root, "sha256", Namespace))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// 3, 4, 6: the captured base is the store alone, marked captured, at the
// store's head, with the witness of its packed bytes (the same content as a
// replay of the tape to that head).
func TestPublishCapturedBaseMarksAndBindsTheStore(t *testing.T) {
	ctx := context.Background()
	computerID := "computer-capture"
	root, mount, digests, replayed := captureFixture(t, computerID, 5, 3)
	result, err := PublishCapturedBase(ctx, CaptureConfig{
		ComputerID: computerID, StoreMarker: filepath.Join(mount, "state"),
		ArtifactsRoot: root, TapeHeadAt: tapeOf(digests),
	})
	if err != nil {
		t.Fatalf("PublishCapturedBase: %v", err)
	}
	d := result.Descriptor
	if d.Origin != OriginLiveCaptured {
		t.Fatalf("origin = %q, want %q", d.Origin, OriginLiveCaptured)
	}
	if d.Sequence != 3 || d.CanonicalHead != digests[2] || d.VocabularyVersion != CurrentVocabularyVersion {
		t.Fatalf("descriptor = seq %d head %s vocab %s", d.Sequence, d.CanonicalHead, d.VocabularyVersion)
	}
	if err := selfdevprotocol.WitnessContentMatches(d.VMLocalContentWitness, replayed.VMLocalContentWitness); err != nil {
		t.Fatalf("captured witness differs from the replayed base at the same head: %v", err)
	}
	loaded, blobPath, err := LoadPublishedBase(root, computerID, d.BlobSHA256)
	if err != nil || loaded.Origin != OriginLiveCaptured {
		t.Fatalf("LoadPublishedBase = %+v, %v", loaded.Origin, err)
	}
	unpacked := t.TempDir()
	if err := Unpack(blobPath, unpacked); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(unpacked)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if strings.Join(names, ",") != "runtime.db,runtime.texture" {
		t.Fatalf("blob holds %v, want the store alone", names)
	}
	witness, head := witnessFromBlob(t, ctx, blobPath, computerID)
	if head != digests[2] {
		t.Fatalf("blob head = %s, want %s", head, digests[2])
	}
	if err := selfdevprotocol.WitnessContentMatches(witness, d.VMLocalContentWitness); err != nil {
		t.Fatalf("published witness does not match the packed bytes: %v", err)
	}
}

// 5: a rebuild seeded from a captured base replays the tail and stays marked.
func TestRebuildSeededFromCapturedBaseKeepsTheMark(t *testing.T) {
	ctx := context.Background()
	computerID := "computer-capture-seed"
	root, mount, digests, _ := captureFixture(t, computerID, 5, 3)
	captured, err := PublishCapturedBase(ctx, CaptureConfig{
		ComputerID: computerID, StoreMarker: filepath.Join(mount, "state"),
		ArtifactsRoot: root, TapeHeadAt: tapeOf(digests),
	})
	if err != nil {
		t.Fatalf("PublishCapturedBase: %v", err)
	}
	seeded := runRebuild(t, ctx, Config{
		ComputerID: computerID, TargetHead: digests[4], ArtifactsRoot: root,
		ScratchDir: t.TempDir(), KeyMaterial: fixtureKey(40), BatchSize: 100,
		MemoryLimitRSS: 512 * 1024 * 1024, SeedPolicy: SeedPinned, SeedBaseRef: captured.Descriptor.BlobSHA256, SeedRequired: true,
	}, fixtureReplay(NewDiskEventSource(root, computerID, digests[4])))
	if seeded.Seed == nil || seeded.Seed.BlobSHA256 != captured.Descriptor.BlobSHA256 {
		t.Fatalf("rebuild did not seed from the captured base: %+v", seeded.Seed)
	}
	if seeded.Descriptor.Sequence != 5 || seeded.Descriptor.Origin != OriginLiveCaptured {
		t.Fatalf("seeded rebuild = seq %d origin %q, want 5 %q", seeded.Descriptor.Sequence, seeded.Descriptor.Origin, OriginLiveCaptured)
	}
}

// 1, 2: a store whose head is not the tape's at that sequence publishes
// nothing at all.
func TestPublishCapturedBaseRefusesAHeadOffTheTape(t *testing.T) {
	ctx := context.Background()
	computerID := "computer-capture-foreign"
	root, mount, digests, _ := captureFixture(t, computerID, 5, 3)
	forged := append([]string(nil), digests...)
	forged[2] = strings.Repeat("e", 64)
	if _, err := PublishCapturedBase(ctx, CaptureConfig{
		ComputerID: computerID, StoreMarker: filepath.Join(mount, "state"),
		ArtifactsRoot: root, TapeHeadAt: tapeOf(forged),
	}); err == nil {
		t.Fatal("a head off the tape was published")
	}
	if files := publishedBaseFiles(t, root); len(files) != 0 {
		t.Fatalf("refusal left published files: %v", files)
	}
}

// 2: a missing store, or a store for another computer, refuses.
func TestPublishCapturedBaseRefusesMissingOrForeignStore(t *testing.T) {
	ctx := context.Background()
	root, mount, digests, _ := captureFixture(t, "computer-capture-own", 3, 3)
	if _, err := PublishCapturedBase(ctx, CaptureConfig{
		ComputerID: "computer-capture-own", StoreMarker: filepath.Join(mount, "nothing-here"),
		ArtifactsRoot: root, TapeHeadAt: tapeOf(digests),
	}); err == nil {
		t.Fatal("a missing store was published")
	}
	if _, err := PublishCapturedBase(ctx, CaptureConfig{
		ComputerID: "computer-someone-else", StoreMarker: filepath.Join(mount, "state"),
		ArtifactsRoot: root, TapeHeadAt: tapeOf(digests),
	}); err == nil {
		t.Fatal("a store with no head for this computer was published")
	}
	if files := publishedBaseFiles(t, root); len(files) != 0 {
		t.Fatalf("refusal left published files: %v", files)
	}
}

// 7: below the free-space floor nothing is written.
func TestPublishCapturedBaseRefusesBelowTheDiskFloor(t *testing.T) {
	ctx := context.Background()
	computerID := "computer-capture-disk"
	root, mount, digests, _ := captureFixture(t, computerID, 3, 3)
	if _, err := PublishCapturedBase(ctx, CaptureConfig{
		ComputerID: computerID, StoreMarker: filepath.Join(mount, "state"),
		ArtifactsRoot: root, TapeHeadAt: tapeOf(digests),
		MinFreeBytes: 1 << 40,
		FreeBytes:    func(string) (uint64, error) { return 1 << 30, nil },
	}); err == nil || !strings.Contains(err.Error(), "free") {
		t.Fatalf("err = %v, want a free-space refusal", err)
	}
	if files := publishedBaseFiles(t, root); len(files) != 0 {
		t.Fatalf("refusal left published files: %v", files)
	}
}

// 4: an unknown origin is refused, so a descriptor this build cannot read
// correctly is never installed as a replayed base.
func TestDescriptorRefusesUnknownOrigin(t *testing.T) {
	_, _, _, replayed := captureFixture(t, "computer-capture-origin", 2, 2)
	replayed.Origin = "imported_from_elsewhere"
	if err := replayed.Validate(); err == nil {
		t.Fatal("an unknown origin validated")
	}
	replayed.Origin = OriginLiveCaptured
	if err := replayed.Validate(); err != nil {
		t.Fatalf("live_captured refused: %v", err)
	}
}
