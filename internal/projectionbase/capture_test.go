package projectionbase

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
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
//  4. the base is not marked as captured, so nothing can tell it is an
//     epoch root (owner, 2026-10-27: derivability is measured from it);
//  5. a rebuild seeded from the root loses its provenance (no record of the
//     root it descends from), or is still marked as captured itself;
//  5a. a replay starts below the root: it asks for the tape prefix the
//     root replaces (genesis repair, a pre-root seed, a resumed scratch);
//  6. the witness is taken before compaction and no longer matches the bytes;
//  7. the publication fills the host disk.

// captureFixture builds an n-event chain in root and lays a store at
// sequence `at` out as the guest data image holds it: <mount>/state and
// <mount>/state.texture, beside other guest files.
func captureFixture(t *testing.T, computerID string, n, at int) (root, mount string, digests []string, replayed Descriptor) {
	t.Helper()
	root, mount, records, replayed := captureFixtureRecords(t, computerID, n, at)
	for _, r := range records {
		digests = append(digests, r.Request.EventDigest)
	}
	return root, mount, digests, replayed
}

// captureFixtureRecords is captureFixture that also returns the chain's
// records (for a prepared, unfinalized append above the captured head).
func captureFixtureRecords(t *testing.T, computerID string, n, at int) (root, mount string, records []computerevent.DurableEvent, replayed Descriptor) {
	t.Helper()
	ctx := context.Background()
	root = t.TempDir()
	records = generateChain(t, computerID, n)
	eventsDir := filepath.Join(root, "sha256", "computer-event")
	if err := os.MkdirAll(eventsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	var digests []string
	for _, record := range records {
		raw, err := record.Request.Event.CanonicalBytes()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(eventsDir, record.Request.EventDigest), raw, 0o600); err != nil {
			t.Fatal(err)
		}
		digests = append(digests, record.Request.EventDigest)
	}
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
	return root, mount, records, result.Descriptor
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
	if d.Sequence != 3 || d.CanonicalHead != digests[2] || d.EffectiveVocabularyVersion() != CurrentVocabularyVersion {
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

// 5: a rebuild seeded from the epoch root is a normal derived base that
// records the root it descends from, and so does its own descendant.
func TestRebuildSeededFromTheEpochRootRecordsIt(t *testing.T) {
	ctx := context.Background()
	computerID := "computer-capture-seed"
	root, mount, digests, _ := captureFixture(t, computerID, 6, 3)
	captured, err := PublishCapturedBase(ctx, CaptureConfig{
		ComputerID: computerID, StoreMarker: filepath.Join(mount, "state"),
		ArtifactsRoot: root, TapeHeadAt: tapeOf(digests),
	})
	if err != nil {
		t.Fatalf("PublishCapturedBase: %v", err)
	}
	want := EpochRootRef{BaseRef: captured.Descriptor.BlobSHA256, Sequence: 3}
	if captured.Descriptor.EpochRoot != nil {
		t.Fatalf("the root records a root of its own: %+v", captured.Descriptor.EpochRoot)
	}
	seeded := runRebuild(t, ctx, Config{
		ComputerID: computerID, TargetHead: digests[4], ArtifactsRoot: root,
		ScratchDir: t.TempDir(), KeyMaterial: fixtureKey(40), BatchSize: 100,
		MemoryLimitRSS: 512 * 1024 * 1024, SeedPolicy: SeedPinned, SeedBaseRef: captured.Descriptor.BlobSHA256, SeedRequired: true,
	}, fixtureReplay(NewDiskEventSource(root, computerID, digests[4])))
	if seeded.Seed == nil || seeded.Seed.BlobSHA256 != captured.Descriptor.BlobSHA256 {
		t.Fatalf("rebuild did not seed from the captured base: %+v", seeded.Seed)
	}
	d := seeded.Descriptor
	if d.Sequence != 5 || d.Origin != "" || d.EpochRoot == nil || *d.EpochRoot != want {
		t.Fatalf("seeded rebuild = seq %d origin %q root %+v, want 5, a derived base, root %+v", d.Sequence, d.Origin, d.EpochRoot, want)
	}
	if d.VocabularyVersion != CurrentVocabularyVersion {
		t.Fatalf("derived base vocabulary = %q, want %q", d.VocabularyVersion, CurrentVocabularyVersion)
	}
	next := runRebuild(t, ctx, Config{
		ComputerID: computerID, TargetHead: digests[5], ArtifactsRoot: root,
		ScratchDir: t.TempDir(), KeyMaterial: fixtureKey(40), BatchSize: 100,
		MemoryLimitRSS: 512 * 1024 * 1024, SeedPolicy: SeedPinned, SeedBaseRef: d.BlobSHA256, SeedRequired: true,
	}, fixtureReplay(NewDiskEventSource(root, computerID, digests[5])))
	if n := next.Descriptor; n.Sequence != 6 || n.Origin != "" || n.EpochRoot == nil || *n.EpochRoot != want {
		t.Fatalf("second descendant = seq %d origin %q root %+v, want 6, root %+v", n.Sequence, n.Origin, n.EpochRoot, want)
	}
}

// 5a: once an epoch root is published, no replay starts below it: not an
// explicit genesis repair, not a pinned pre-root seed. Seeding from the root
// itself works.
func TestRebuildNeverReplaysBelowTheEpochRoot(t *testing.T) {
	ctx := context.Background()
	computerID := "computer-capture-floor"
	root, mount, digests, _ := captureFixture(t, computerID, 5, 3)
	preRoot := runRebuild(t, ctx, Config{
		ComputerID: computerID, TargetHead: digests[1], ArtifactsRoot: root,
		ScratchDir: t.TempDir(), KeyMaterial: fixtureKey(40), BatchSize: 100,
		MemoryLimitRSS: 512 * 1024 * 1024, SeedPolicy: SeedNone, TargetSequence: 2,
	}, fixtureReplay(NewDiskEventSource(root, computerID, digests[1])))
	captured, err := PublishCapturedBase(ctx, CaptureConfig{
		ComputerID: computerID, StoreMarker: filepath.Join(mount, "state"),
		ArtifactsRoot: root, TapeHeadAt: tapeOf(digests),
	})
	if err != nil {
		t.Fatalf("PublishCapturedBase: %v", err)
	}
	run := func(cfg Config) (*Result, error) {
		cfg.ComputerID, cfg.TargetHead, cfg.ArtifactsRoot = computerID, digests[4], root
		cfg.ScratchDir, cfg.KeyMaterial, cfg.BatchSize, cfg.MemoryLimitRSS = t.TempDir(), fixtureKey(40), 100, 512*1024*1024
		r, err := NewRebuilder(cfg)
		if err != nil {
			return nil, err
		}
		return r.Run(ctx, fixtureReplay(NewDiskEventSource(root, computerID, digests[4])))
	}
	if _, err := run(Config{SeedPolicy: SeedNone, TargetSequence: 5}); !errors.Is(err, ErrBaseRefused) || !strings.Contains(err.Error(), "epoch root") {
		t.Fatalf("genesis replay past an epoch root: err = %v, want an epoch-root refusal", err)
	}
	if _, err := run(Config{SeedPolicy: SeedPinned, SeedBaseRef: preRoot.Descriptor.BlobSHA256, SeedRequired: true}); !errors.Is(err, ErrBaseRefused) || !strings.Contains(err.Error(), "epoch root") {
		t.Fatalf("replay from a pre-root seed: err = %v, want an epoch-root refusal", err)
	}
	got, err := run(Config{SeedPolicy: SeedAuto, TargetSequence: 5})
	if err != nil {
		t.Fatalf("replay from the root: %v", err)
	}
	if got.Seed == nil || got.Seed.BlobSHA256 != captured.Descriptor.BlobSHA256 {
		t.Fatalf("auto seed = %+v, want the epoch root", got.Seed)
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

// Review 364 (Devin, MEDIUM): compaction writes inside the mounted image
// (bounded by its own free space) and diverges the reflink on the capture's
// filesystem. Either too full refuses before compaction, publishing nothing.
func TestPublishCapturedBaseRefusesAFullImageOrCaptureFilesystem(t *testing.T) {
	ctx := context.Background()
	for name, full := range map[string]func(mount, captureFS string) string{
		"the image":                func(mount, _ string) string { return mount },
		"the capture's filesystem": func(_, captureFS string) string { return captureFS },
	} {
		t.Run(name, func(t *testing.T) {
			computerID := "computer-capture-full"
			root, mount, digests, _ := captureFixture(t, computerID, 3, 3)
			captureFS := t.TempDir()
			tight := full(mount, captureFS)
			_, err := PublishCapturedBase(ctx, CaptureConfig{
				ComputerID: computerID, StoreMarker: filepath.Join(mount, "state"),
				ArtifactsRoot: root, TapeHeadAt: tapeOf(digests), CaptureFS: captureFS,
				MinFreeBytes: 1 << 20,
				FreeBytes: func(path string) (uint64, error) {
					if path == tight || strings.HasPrefix(path, tight+string(filepath.Separator)) {
						return 1 << 10, nil
					}
					return 1 << 40, nil
				},
			})
			if !errors.Is(err, ErrBaseRefused) || !strings.Contains(err.Error(), "free") {
				t.Fatalf("err = %v, want a free-space refusal", err)
			}
			if files := publishedBaseFiles(t, root); len(files) != 0 {
				t.Fatalf("refusal left published files: %v", files)
			}
		})
	}
}

// Review 365 (DeepSeek 3, 5): at the floor's own sequence, and on a resumed
// scratch above it, a replay is admitted only on the root's line: a pre-root
// base at exactly the root's sequence, or a scratch replayed from genesis
// (no recorded root), refuses.
func TestRebuildAtOrAboveTheFloorNeedsTheRootsLine(t *testing.T) {
	ctx := context.Background()
	computerID := "computer-capture-line"
	root, mount, digests, _ := captureFixture(t, computerID, 5, 3)
	cfg := func(target int, scratch string) Config {
		return Config{
			ComputerID: computerID, TargetHead: digests[target-1], ArtifactsRoot: root,
			ScratchDir: scratch, KeyMaterial: fixtureKey(40), BatchSize: 100,
			MemoryLimitRSS: 512 * 1024 * 1024, TargetSequence: uint64(target),
		}
	}
	// Before the root: a tape replay at the root's sequence, and a genesis
	// scratch left at 4.
	preRootCfg := cfg(3, t.TempDir())
	preRootCfg.SeedPolicy = SeedNone
	preRoot := runRebuild(t, ctx, preRootCfg, fixtureReplay(NewDiskEventSource(root, computerID, digests[2])))
	genesisScratch := t.TempDir()
	scratchCfg := cfg(4, genesisScratch)
	scratchCfg.SeedPolicy = SeedNone
	runRebuild(t, ctx, scratchCfg, fixtureReplay(NewDiskEventSource(root, computerID, digests[3])))
	captured, err := PublishCapturedBase(ctx, CaptureConfig{
		ComputerID: computerID, StoreMarker: filepath.Join(mount, "state"),
		ArtifactsRoot: root, TapeHeadAt: tapeOf(digests),
	})
	if err != nil {
		t.Fatalf("PublishCapturedBase: %v", err)
	}
	if captured.Descriptor.BlobSHA256 == preRoot.Descriptor.BlobSHA256 {
		t.Fatal("fixture: the root and the pre-root base are one blob")
	}
	run := func(c Config) error {
		r, err := NewRebuilder(c)
		if err != nil {
			return err
		}
		_, err = r.Run(ctx, fixtureReplay(NewDiskEventSource(root, computerID, digests[4])))
		return err
	}
	pinned := cfg(5, t.TempDir())
	pinned.SeedPolicy, pinned.SeedBaseRef, pinned.SeedRequired = SeedPinned, preRoot.Descriptor.BlobSHA256, true
	if err := run(pinned); !errors.Is(err, ErrBaseRefused) || !strings.Contains(err.Error(), "epoch root") {
		t.Fatalf("pre-root seed at the root's sequence: err = %v, want an epoch-root refusal", err)
	}
	// A resumed scratch with no record of the root is discarded, and the
	// error is transient (not ErrBaseRefused, which would block the job):
	// the next attempt on the same scratch seeds from the root.
	resumed := cfg(5, genesisScratch)
	resumed.SeedPolicy, resumed.SeedBaseRef, resumed.SeedRequired = SeedPinned, captured.Descriptor.BlobSHA256, true
	if err := run(resumed); err == nil || errors.Is(err, ErrBaseRefused) || !strings.Contains(err.Error(), "epoch root") {
		t.Fatalf("resumed genesis scratch above the root: err = %v, want a transient epoch-root error", err)
	}
	if err := run(resumed); err != nil {
		t.Fatalf("next attempt on the discarded scratch: %v", err)
	}
	fromRoot := cfg(5, t.TempDir())
	fromRoot.SeedPolicy, fromRoot.SeedBaseRef, fromRoot.SeedRequired = SeedPinned, captured.Descriptor.BlobSHA256, true
	if err := run(fromRoot); err != nil {
		t.Fatalf("replay from the root: %v", err)
	}
}

// Review 365 (DeepSeek 4): the floor scan fails closed on a descriptor of
// this computer it cannot parse (a torn root sidecar must not lower the
// floor), and ignores another computer's.
func TestEpochFloorFailsClosedOnATornDescriptor(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "sha256", Namespace)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	torn := DescriptorSidecarName(strings.Repeat("a", 64))
	if err := os.WriteFile(filepath.Join(dir, torn), []byte(`{"computer_id":"computer-torn","sequence":6000,"orig`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := publishedEpochFloor(root, "computer-torn"); err == nil {
		t.Fatal("a torn descriptor of this computer did not fail the floor scan")
	}
	if floor, err := publishedEpochFloor(root, "computer-other"); err != nil || floor != 0 {
		t.Fatalf("another computer's torn descriptor: floor %d err %v, want 0, nil", floor, err)
	}
}

// Review 365 (DeepSeek, MEDIUM): a captured store whose head is at or
// behind the advertised watermark can never become the root (corpusd keeps
// the higher watermark). It refuses before compaction and publishes nothing.
func TestPublishCapturedBaseRefusesAHeadAtOrBelowTheWatermark(t *testing.T) {
	ctx := context.Background()
	computerID := "computer-capture-behind"
	root, mount, digests, _ := captureFixture(t, computerID, 3, 3)
	_, err := PublishCapturedBase(ctx, CaptureConfig{
		ComputerID: computerID, StoreMarker: filepath.Join(mount, "state"),
		ArtifactsRoot: root, TapeHeadAt: tapeOf(digests), AboveSequence: 3,
	})
	if !errors.Is(err, ErrBaseRefused) || !strings.Contains(err.Error(), "watermark") {
		t.Fatalf("err = %v, want a refusal naming the watermark", err)
	}
	if files := publishedBaseFiles(t, root); len(files) != 0 {
		t.Fatalf("refusal left published files: %v", files)
	}
}

// 4: an unknown origin is refused, so a descriptor this build cannot read
// correctly is never installed as a replayed base.
func TestDescriptorRefusesUnknownOrigin(t *testing.T) {
	_, _, _, replayed := captureFixture(t, "computer-capture-origin", 2, 2)
	if err := replayed.Validate(); err != nil {
		t.Fatalf("a replayed (pre-S2) base refused: %v", err)
	}
	replayed.Origin = "imported_from_elsewhere"
	if err := replayed.Validate(); err == nil {
		t.Fatal("an unknown origin validated")
	}
	replayed.Origin = OriginLiveCaptured
	replayed.VocabularyVersion = VocabularyVersionFor(OriginLiveCaptured)
	if err := replayed.Validate(); err != nil {
		t.Fatalf("live_captured refused: %v", err)
	}
}

// Review 364 (DeepSeek, MEDIUM-HIGH): rollback safety. A build from before
// S2 ignores the origin field (ParseDescriptor allows unknown fields) and
// would read a captured base as a replayed one. Its Validate refuses any
// vocabulary outside {v1, v2} (IsKnownVocabularyVersion at 28bc1617a), so a
// captured base carries a vocabulary that set does not hold, and the mark
// and the vocabulary must agree both ways.
func TestCapturedBaseFailsClosedOnAPreS2Reader(t *testing.T) {
	preS2Known := map[string]bool{"v1": true, "v2": true}
	ctx := context.Background()
	computerID := "computer-capture-rollback"
	root, mount, digests, replayed := captureFixture(t, computerID, 5, 3)
	result, err := PublishCapturedBase(ctx, CaptureConfig{
		ComputerID: computerID, StoreMarker: filepath.Join(mount, "state"),
		ArtifactsRoot: root, TapeHeadAt: tapeOf(digests),
	})
	if err != nil {
		t.Fatalf("PublishCapturedBase: %v", err)
	}
	if preS2Known[result.Descriptor.VocabularyVersion] {
		t.Fatalf("captured base vocabulary %q is one a pre-S2 reader accepts", result.Descriptor.VocabularyVersion)
	}
	if got := result.Descriptor.EffectiveVocabularyVersion(); got != CurrentVocabularyVersion {
		t.Fatalf("effective vocabulary = %q, want %q", got, CurrentVocabularyVersion)
	}
	seeded := runRebuild(t, ctx, Config{
		ComputerID: computerID, TargetHead: digests[4], ArtifactsRoot: root,
		ScratchDir: t.TempDir(), KeyMaterial: fixtureKey(40), BatchSize: 100,
		MemoryLimitRSS: 512 * 1024 * 1024, SeedPolicy: SeedPinned, SeedBaseRef: result.Descriptor.BlobSHA256, SeedRequired: true,
	}, fixtureReplay(NewDiskEventSource(root, computerID, digests[4])))
	// A descendant is a normal derived base (its state is the root's plus
	// a replayed tail): a pre-S2 build may read it.
	if !preS2Known[seeded.Descriptor.VocabularyVersion] {
		t.Fatalf("a base derived from the root has vocabulary %q, which a pre-S2 reader refuses", seeded.Descriptor.VocabularyVersion)
	}
	if !preS2Known[replayed.VocabularyVersion] {
		t.Fatalf("a replayed base's vocabulary %q no longer reads on a pre-S2 build", replayed.VocabularyVersion)
	}
	mismatched := replayed
	mismatched.Origin = OriginLiveCaptured
	if err := mismatched.Validate(); err == nil {
		t.Fatal("a captured origin with a plain vocabulary validated")
	}
	mismatched = replayed
	mismatched.VocabularyVersion = VocabularyVersionFor(OriginLiveCaptured)
	if err := mismatched.Validate(); err == nil {
		t.Fatal("a captured vocabulary without the captured origin validated")
	}
}

// Review 364 (DeepSeek, HIGH): the store tree is the guest's. Before
// anything opens it, a symlink, device, fifo or socket anywhere under the
// store, or a store path that is itself a symlink, refuses; nothing is
// staged or published.
func TestPublishCapturedBaseRefusesPlantedLinks(t *testing.T) {
	ctx := context.Background()
	outside := filepath.Join(t.TempDir(), "host-secret")
	if err := os.WriteFile(outside, []byte("host bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, plant := range map[string]func(t *testing.T, mount string){
		"symlink inside the workspace": func(t *testing.T, mount string) {
			if err := os.Symlink(outside, filepath.Join(mount, "state.texture", "planted")); err != nil {
				t.Fatal(err)
			}
		},
		"fifo inside the workspace": func(t *testing.T, mount string) {
			if err := syscall.Mkfifo(filepath.Join(mount, "state.texture", "planted.fifo"), 0o600); err != nil {
				t.Fatal(err)
			}
		},
		"marker is a symlink": func(t *testing.T, mount string) {
			if err := os.Rename(filepath.Join(mount, "state"), filepath.Join(mount, "real-state")); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Join(mount, "real-state"), filepath.Join(mount, "state")); err != nil {
				t.Fatal(err)
			}
		},
		"workspace is a symlink": func(t *testing.T, mount string) {
			if err := os.Rename(filepath.Join(mount, "state.texture"), filepath.Join(mount, "real-texture")); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Join(mount, "real-texture"), filepath.Join(mount, "state.texture")); err != nil {
				t.Fatal(err)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			computerID := "computer-capture-planted"
			root, mount, digests, _ := captureFixture(t, computerID, 3, 3)
			plant(t, mount)
			_, err := PublishCapturedBase(ctx, CaptureConfig{
				ComputerID: computerID, StoreMarker: filepath.Join(mount, "state"),
				ArtifactsRoot: root, TapeHeadAt: tapeOf(digests),
			})
			if !errors.Is(err, ErrBaseRefused) {
				t.Fatalf("err = %v, want a refusal", err)
			}
			if _, err := os.Lstat(filepath.Join(mount, captureStagingDirName)); !os.IsNotExist(err) {
				t.Fatalf("the store was staged before the refusal: %v", err)
			}
			if files := publishedBaseFiles(t, root); len(files) != 0 {
				t.Fatalf("refusal left published files: %v", files)
			}
		})
	}
}

// Review 364 item 8: a capture taken while the guest had an append prepared
// but not finalized (a row above the head) publishes the base at the head,
// with the same witness as a replay to it.
func TestPublishCapturedBaseDropsAPreparedAppendAboveTheHead(t *testing.T) {
	ctx := context.Background()
	computerID := "computer-capture-prepared"
	root, mount, records, replayed := captureFixtureRecords(t, computerID, 5, 3)
	store, err := choirstore.Open(filepath.Join(mount, "state"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Prepare(ctx, records[3].Request); err != nil {
		_ = store.Close()
		t.Fatalf("prepare event 4: %v", err)
	}
	prepared, err := store.Prepared(ctx, computerID)
	if err != nil || len(prepared) != 1 {
		_ = store.Close()
		t.Fatalf("fixture: prepared = %d, %v; want one row above the head", len(prepared), err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	var digests []string
	for _, r := range records {
		digests = append(digests, r.Request.EventDigest)
	}
	result, err := PublishCapturedBase(ctx, CaptureConfig{
		ComputerID: computerID, StoreMarker: filepath.Join(mount, "state"),
		ArtifactsRoot: root, TapeHeadAt: tapeOf(digests),
	})
	if err != nil {
		t.Fatalf("PublishCapturedBase with a prepared row: %v", err)
	}
	if result.Descriptor.Sequence != 3 || result.Descriptor.CanonicalHead != digests[2] {
		t.Fatalf("captured base = seq %d head %s, want 3 %s", result.Descriptor.Sequence, result.Descriptor.CanonicalHead, digests[2])
	}
	if err := selfdevprotocol.WitnessContentMatches(result.Descriptor.VMLocalContentWitness, replayed.VMLocalContentWitness); err != nil {
		t.Fatalf("the prepared row reached the witness: %v", err)
	}
}

// Review 370 (DeepSeek 1): a parts capture's blob is the manifest, a few
// KiB, but the read-back that follows materializes the whole store. The room
// check before it reserves the store, not the manifest: with room for the
// manifest only once the publish is done, the capture refuses and advertises
// nothing.
func TestPublishCapturedPartsBaseReservesTheReadBackStore(t *testing.T) {
	ctx := context.Background()
	computerID := "computer-capture-parts-room"
	root, mount, digests, _ := captureFixture(t, computerID, 3, 3)
	if workspaceBytes(mount) < 40<<10 {
		t.Skip("fixture store is smaller than the room used below")
	}
	_, err := PublishCapturedBase(ctx, CaptureConfig{
		ComputerID: computerID, StoreMarker: filepath.Join(mount, "state"),
		ArtifactsRoot: root, TapeHeadAt: tapeOf(digests), PublishParts: true,
		FreeBytes: func(path string) (uint64, error) {
			if len(publishedBaseFiles(t, root)) > 0 {
				return 24 << 10, nil
			}
			return 1 << 40, nil
		},
	})
	if !errors.Is(err, ErrBaseRefused) || !strings.Contains(err.Error(), "free") {
		t.Fatalf("err = %v, want a free-space refusal before the read-back", err)
	}
	if files := publishedBaseFiles(t, root); len(files) != 0 {
		t.Fatalf("refusal left published files: %v", files)
	}
}
