package projectionbase

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
)

type fakeBaseSource struct {
	sequence      uint64
	baseRef       string
	descriptor    Descriptor
	blob          []byte
	disk          *DiskEventSource
	watermarkErr  error
	descriptorErr error
	corruptBlob   bool
}

func (f *fakeBaseSource) Watermark(ctx context.Context, computerID string) (uint64, string, error) {
	if f.watermarkErr != nil {
		return 0, "", f.watermarkErr
	}
	return f.sequence, f.baseRef, nil
}

func (f *fakeBaseSource) Descriptor(ctx context.Context, computerID, baseRef string) (Descriptor, error) {
	if f.descriptorErr != nil {
		return Descriptor{}, f.descriptorErr
	}
	return f.descriptor, nil
}

func (f *fakeBaseSource) DownloadBlob(ctx context.Context, computerID, baseRef string, dst io.Writer) error {
	blob := f.blob
	if f.corruptBlob {
		blob = append(append([]byte{}, blob...), 0x00)
	}
	_, err := dst.Write(blob)
	return err
}

func (f *fakeBaseSource) TailPage(ctx context.Context, computerID string, afterSequence uint64, pageSize int) ([]computerevent.DurableEvent, error) {
	return f.disk.EventsPage(ctx, computerID, afterSequence, pageSize)
}
func installIdem(seq uint64) string {
	return "install-idem-" + strings.Repeat("x", int(seq))
}

// buildInstallChain writes a 3-event V1 chain (genesis + 2 researcher updates)
// to the disk layout and returns its digests.
func buildInstallChain(t *testing.T, artifactsRoot, computerID string) []string {
	t.Helper()
	eventsDir := filepath.Join(artifactsRoot, "sha256", "computer-event")
	if err := os.MkdirAll(eventsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sha := func(c string) string { return strings.Repeat(c, 64) }
	commitments := []string{sha("a"), sha("b"), sha("c")}
	var digests []string
	var prevHead = computerevent.ZeroHead
	var expectedDesired, expectedEffective = computerevent.ZeroHead, computerevent.ZeroHead
	var expectedDesiredCommit, expectedEffectiveCommit = computerevent.ZeroHead, computerevent.ZeroHead
	var current *computerevent.Head
	for i, commitment := range commitments {
		seq := uint64(i + 1)
		eventID, err := computerevent.NewEventID()
		if err != nil {
			t.Fatal(err)
		}
		kind := computerevent.EventGenesisImported
		if seq > 1 {
			kind = computerevent.EventResearcherUpdate
		}
		event := computerevent.Event{
			SchemaVersion:                    computerevent.SchemaVersionV1,
			EventID:                          eventID,
			ComputerID:                       computerID,
			Sequence:                         seq,
			PreviousHead:                     prevHead,
			EventKind:                        kind,
			OccurredAt:                       time.Date(2026, 9, 9, 12, 0, int(seq), 0, time.UTC).Format(time.RFC3339Nano),
			IdempotencyKey:                   installIdem(seq),
			ActorProfile:                     "trusted-core",
			AuthorityRef:                     "authority:test",
			PayloadCommitment:                commitment,
			PrivacyClass:                     "public",
			ReducerVersion:                   computerevent.ReducerVersionV1,
			ExpectedDesiredEventHead:         expectedDesired,
			ExpectedEffectiveEventHead:       expectedEffective,
			ExpectedDesiredStateCommitment:   expectedDesiredCommit,
			ExpectedEffectiveStateCommitment: expectedEffectiveCommit,
			ResultingEffectiveCommitment:     commitment,
		}
		input := computerevent.TransitionInput{TargetStateCommitment: commitment}
		pinIntent, err := computerevent.ComputePinIntentCommitment(event, input)
		if err != nil {
			t.Fatal(err)
		}
		reqCommitment, err := computerevent.ComputeRequestCommitment(event, input, pinIntent, nil)
		if err != nil {
			t.Fatal(err)
		}
		event.RequestCommitment = reqCommitment
		raw, err := event.CanonicalBytes()
		if err != nil {
			t.Fatal(err)
		}
		digest, err := event.Digest()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(eventsDir, digest), raw, 0o600); err != nil {
			t.Fatal(err)
		}
		next, err := computerevent.Reduce(current, event, input)
		if err != nil {
			t.Fatalf("fixture reduce seq %d: %v", seq, err)
		}
		prevHead = digest
		expectedDesired, expectedEffective = next.DesiredEventHead, next.EffectiveEventHead
		expectedDesiredCommit, expectedEffectiveCommit = next.DesiredStateCommitment, next.EffectiveStateCommitment
		current = &next
		digests = append(digests, digest)
	}
	return digests
}

func TestInstallVerifiedBaseThenTailReplaysToHead(t *testing.T) {
	ctx := context.Background()
	artifactsRoot := t.TempDir()
	computerID := "computer-install-test"
	keyMaterial := make([]byte, 32)
	for i := range keyMaterial {
		keyMaterial[i] = byte(i + 7)
	}
	digests := buildInstallChain(t, artifactsRoot, computerID)
	disk := NewDiskEventSource(artifactsRoot, computerID)

	// Publish a base at W=1 through the real offline rebuilder.
	rebuilder, err := NewRebuilder(Config{
		ComputerID: computerID, TargetHead: digests[0], ArtifactsRoot: artifactsRoot,
		ScratchDir: t.TempDir(), KeyMaterial: keyMaterial, BatchSize: 100, MemoryLimitRSS: 512 * 1024 * 1024,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := rebuilder.Run(ctx, disk)
	if err != nil {
		t.Fatalf("rebuild base at W=1: %v", err)
	}
	blob, err := os.ReadFile(result.BlobPath)
	if err != nil {
		t.Fatal(err)
	}
	sidecar, err := os.ReadFile(filepath.Join(artifactsRoot, "sha256", Namespace, result.Descriptor.BlobSHA256+".descriptor.json"))
	if err != nil {
		t.Fatalf("descriptor sidecar missing after publication: %v", err)
	}
	if _, err := ParseDescriptor(sidecar); err != nil {
		t.Fatalf("published sidecar refused: %v", err)
	}

	src := &fakeBaseSource{sequence: 1, baseRef: result.Descriptor.BlobSHA256, descriptor: result.Descriptor, blob: blob, disk: disk}
	installDir := t.TempDir()
	descriptor, err := InstallVerifiedBase(ctx, src, installDir, "runtime.db", computerID, digests[2], 3)
	if err != nil {
		t.Fatalf("verified install refused: %v", err)
	}
	if descriptor.Sequence != 1 || descriptor.CanonicalHead != digests[0] {
		t.Fatalf("installed descriptor = (%d), want W=1", descriptor.Sequence)
	}

	// The installed store head is the watermark; the prefix was never replayed here.
	installed, err := choirstore.Open(filepath.Join(installDir, "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	head, err := installed.Head(ctx, computerID)
	if err != nil || head == nil {
		t.Fatalf("installed head unreadable: %v", err)
	}
	if head.Sequence != 1 || head.CanonicalEventHead != digests[0] {
		t.Fatalf("installed head = (%d), want (1)", head.Sequence)
	}

	// Tail-only replay from the installed head reaches H through the real loop.
	appender, err := computerevent.NewComputerEventAppender(computerID, disk, installed, disk, disk)
	if err != nil {
		t.Fatal(err)
	}
	if err := appender.ReconstructThroughTarget(ctx, installed, digests[2]); err != nil {
		t.Fatalf("tail replay refused: %v", err)
	}
	final, err := installed.Head(ctx, computerID)
	if err != nil || final == nil {
		t.Fatalf("final head unreadable: %v", err)
	}
	if final.Sequence != 3 || final.CanonicalEventHead != digests[2] {
		t.Fatalf("final head = (%d), want (3, H)", final.Sequence)
	}
	_ = installed.Close()

	// Idempotent reinstall short-circuits: same target, same verified descriptor.
	again, err := InstallVerifiedBase(ctx, src, installDir, "runtime.db", computerID, digests[2], 3)
	if err != nil {
		t.Fatalf("idempotent reinstall refused: %v", err)
	}
	if again.BlobSHA256 != descriptor.BlobSHA256 {
		t.Fatalf("reinstall returned a different base")
	}
}

func TestInstallRefusesFailureClasses(t *testing.T) {
	ctx := context.Background()
	artifactsRoot := t.TempDir()
	computerID := "computer-install-refuse"
	keyMaterial := make([]byte, 32)
	for i := range keyMaterial {
		keyMaterial[i] = byte(i + 3)
	}
	digests := buildInstallChain(t, artifactsRoot, computerID)
	disk := NewDiskEventSource(artifactsRoot, computerID)
	rebuilder, err := NewRebuilder(Config{
		ComputerID: computerID, TargetHead: digests[0], ArtifactsRoot: artifactsRoot,
		ScratchDir: t.TempDir(), KeyMaterial: keyMaterial, BatchSize: 100, MemoryLimitRSS: 512 * 1024 * 1024,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := rebuilder.Run(ctx, disk)
	if err != nil {
		t.Fatal(err)
	}
	blob, err := os.ReadFile(result.BlobPath)
	if err != nil {
		t.Fatal(err)
	}
	base := &fakeBaseSource{sequence: 1, baseRef: result.Descriptor.BlobSHA256, descriptor: result.Descriptor, blob: blob, disk: disk}

	t.Run("missing watermark", func(t *testing.T) {
		src := *base
		src.watermarkErr = errors.New("platform outage")
		if _, err := InstallVerifiedBase(ctx, &src, t.TempDir(), "runtime.db", computerID, digests[2], 3); err == nil {
			t.Fatal("watermark outage admitted")
		}
	})
	t.Run("corrupt blob", func(t *testing.T) {
		src := *base
		src.corruptBlob = true
		if _, err := InstallVerifiedBase(ctx, &src, t.TempDir(), "runtime.db", computerID, digests[2], 3); !errors.Is(err, ErrBaseRefused) {
			t.Fatalf("corrupt blob did not refuse: %v", err)
		}
	})
	t.Run("foreign descriptor", func(t *testing.T) {
		src := *base
		foreign := result.Descriptor
		foreign.ComputerID = "computer-other"
		src.descriptor = foreign
		if _, err := InstallVerifiedBase(ctx, &src, t.TempDir(), "runtime.db", computerID, digests[2], 3); !errors.Is(err, ErrBaseRefused) {
			t.Fatalf("foreign descriptor did not refuse: %v", err)
		}
	})
	t.Run("base after target", func(t *testing.T) {
		if _, err := InstallVerifiedBase(ctx, base, t.TempDir(), "runtime.db", computerID, digests[0], 1); err != nil {
			t.Fatalf("W=H degenerate refused: %v", err)
		}
		// Rebuild a base at W=2, then demand target H=1: rewind refuses.
		rebuilder2, err := NewRebuilder(Config{
			ComputerID: computerID, TargetHead: digests[1], ArtifactsRoot: artifactsRoot,
			ScratchDir: t.TempDir(), KeyMaterial: keyMaterial, BatchSize: 100, MemoryLimitRSS: 512 * 1024 * 1024,
		})
		if err != nil {
			t.Fatal(err)
		}
		result2, err := rebuilder2.Run(ctx, disk)
		if err != nil {
			t.Fatal(err)
		}
		blob2, err := os.ReadFile(result2.BlobPath)
		if err != nil {
			t.Fatal(err)
		}
		src2 := &fakeBaseSource{sequence: 2, baseRef: result2.Descriptor.BlobSHA256, descriptor: result2.Descriptor, blob: blob2, disk: disk}
		if _, err := InstallVerifiedBase(ctx, src2, t.TempDir(), "runtime.db", computerID, digests[0], 1); !errors.Is(err, ErrBaseRefused) {
			t.Fatalf("base after target did not refuse: %v", err)
		}
	})
	t.Run("partial install suspected", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "garbage.bin"), []byte("partial"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := InstallVerifiedBase(ctx, base, dir, "runtime.db", computerID, digests[2], 3); !errors.Is(err, ErrBaseRefused) {
			t.Fatalf("partial store did not refuse: %v", err)
		}
	})
	t.Run("marker normalization for state layout", func(t *testing.T) {
		dir := t.TempDir()
		desc, err := InstallVerifiedBase(ctx, base, dir, "state", computerID, digests[2], 3)
		if err != nil {
			t.Fatalf("InstallVerifiedBase with state marker: %v", err)
		}
		if desc.Sequence != result.Descriptor.Sequence {
			t.Fatalf("unexpected descriptor: %+v", desc)
		}
		if _, err := os.Stat(filepath.Join(dir, "state")); err != nil {
			t.Fatalf("state marker missing: %v", err)
		}
		if _, err := os.Stat(filepath.Join(dir, "state.texture")); err != nil {
			t.Fatalf("state.texture workspace missing: %v", err)
		}
	})
	t.Run("missing marker refuses without creating store", func(t *testing.T) {
		dir := t.TempDir()
		err := verifyInstalledHead(dir, "nonexistent.db", result.Descriptor)
		if !errors.Is(err, ErrBaseRefused) {
			t.Fatalf("expected ErrBaseRefused, got %v", err)
		}
		if _, err := os.Stat(filepath.Join(dir, "nonexistent.db")); !os.IsNotExist(err) {
			t.Fatalf("verifyInstalledHead auto-created marker file")
		}
		if _, err := os.Stat(filepath.Join(dir, "nonexistent.texture")); !os.IsNotExist(err) {
			t.Fatalf("verifyInstalledHead auto-created texture workspace")
		}
	})
}
