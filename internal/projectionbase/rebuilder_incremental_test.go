package projectionbase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
)

// fixtureReplaySource adapts a fixture CASReplaySource: it records every replay
// page request (no-prefix assertions) and accepts synthesized fixture receipts.
// Production sources verify real signed receipts (HTTPClient wrapped with an
// EventHeadReceiptVerifier) or refuse (DiskEventSource).
type fixtureReplaySource struct {
	CASReplaySource
	mu        sync.Mutex
	requested []uint64
}

func fixtureReplay(src CASReplaySource) *fixtureReplaySource {
	return &fixtureReplaySource{CASReplaySource: src}
}

func (s *fixtureReplaySource) VerifyEventHeadReceipt(context.Context, computerevent.Receipt, computerevent.CASRequest) error {
	return nil
}

func (s *fixtureReplaySource) EventsPage(ctx context.Context, computerID string, afterSequence uint64, pageSize int) ([]computerevent.DurableEvent, error) {
	s.mu.Lock()
	s.requested = append(s.requested, afterSequence)
	s.mu.Unlock()
	return s.CASReplaySource.EventsPage(ctx, computerID, afterSequence, pageSize)
}

func (s *fixtureReplaySource) requests() []uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]uint64(nil), s.requested...)
}

func (s *fixtureReplaySource) minRequested() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.requested) == 0 {
		return 0
	}
	min := s.requested[0]
	for _, v := range s.requested {
		if v < min {
			min = v
		}
	}
	return min
}

func fixtureKey(fill byte) []byte {
	key := make([]byte, 32)
	for i := range key {
		key[i] = fill + byte(i)
	}
	return key
}

func runRebuild(t *testing.T, ctx context.Context, cfg Config, src CASReplaySource) *Result {
	t.Helper()
	rebuilder, err := NewRebuilder(cfg)
	if err != nil {
		t.Fatalf("NewRebuilder: %v", err)
	}
	result, err := rebuilder.Run(ctx, src)
	if err != nil {
		t.Fatalf("Rebuilder.Run: %v", err)
	}
	return result
}

// generateChain builds an n-event V1 fixture chain in memory (genesis plus
// research updates) with synthesized receipts, mirroring the disk layout
// DiskEventSource produces.
func generateChain(t *testing.T, computerID string, n int) []computerevent.DurableEvent {
	t.Helper()
	if n < 1 {
		t.Fatalf("chain length must be positive")
	}
	records := make([]computerevent.DurableEvent, 0, n)
	var current *computerevent.Head
	for i := 1; i <= n; i++ {
		seq := uint64(i)
		commitment := fmt.Sprintf("%064x", i)
		eventID, err := computerevent.NewEventID()
		if err != nil {
			t.Fatal(err)
		}
		prevHead := computerevent.ZeroHead
		expectedDesired, expectedEffective := computerevent.ZeroHead, computerevent.ZeroHead
		expectedDesiredCommit, expectedEffectiveCommit := computerevent.ZeroHead, computerevent.ZeroHead
		if current != nil {
			prevHead = current.CanonicalEventHead
			expectedDesired, expectedEffective = current.DesiredEventHead, current.EffectiveEventHead
			expectedDesiredCommit, expectedEffectiveCommit = current.DesiredStateCommitment, current.EffectiveStateCommitment
		}
		kind := computerevent.EventGenesisImported
		if seq > 1 {
			kind = computerevent.EventResearchUpdate
		}
		event := computerevent.Event{
			SchemaVersion:                    computerevent.SchemaVersionV1,
			EventID:                          eventID,
			ComputerID:                       computerID,
			Sequence:                         seq,
			PreviousHead:                     prevHead,
			EventKind:                        kind,
			OccurredAt:                       time.Date(2026, 9, 9, 12, 0, i, 0, time.UTC).Format(time.RFC3339Nano),
			IdempotencyKey:                   fmt.Sprintf("fixture-idem-%s-%d", computerID, i),
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
		next, err := computerevent.Reduce(current, event, input)
		if err != nil {
			t.Fatalf("fixture reduce seq %d: %v", seq, err)
		}
		digest, err := event.Digest()
		if err != nil {
			t.Fatal(err)
		}
		records = append(records, computerevent.DurableEvent{
			Request: computerevent.CASRequest{
				Event:               event,
				EventDigest:         digest,
				EventArtifactDigest: digest,
				Input:               input,
				Next:                next,
			},
			Receipt: computerevent.Receipt{
				ReceiptKind: "EventHeadReceipt",
				Issuer:      "corpusd",
				KindFields:  map[string]any{"event_digest": digest},
			},
		})
		current = &next
	}
	return records
}

// buildChainN writes an n-event V1 fixture chain into the disk artifact layout
// and returns the event digests in sequence order.
func buildChainN(t *testing.T, artifactsRoot, computerID string, n int) []string {
	t.Helper()
	records := generateChain(t, computerID, n)
	eventsDir := filepath.Join(artifactsRoot, "sha256", "computer-event")
	if err := os.MkdirAll(eventsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	digests := make([]string, 0, n)
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
	return digests
}

// memFixtureSource serves a fixture chain held in memory.
type memFixtureSource struct {
	computerID string
	records    []computerevent.DurableEvent
}

func (s *memFixtureSource) EventsPage(ctx context.Context, computerID string, afterSequence uint64, pageSize int) ([]computerevent.DurableEvent, error) {
	var page []computerevent.DurableEvent
	for _, record := range s.records {
		if record.Request.Event.Sequence <= afterSequence {
			continue
		}
		page = append(page, record)
		if pageSize > 0 && len(page) >= pageSize {
			break
		}
	}
	if page == nil {
		return []computerevent.DurableEvent{}, nil
	}
	return page, nil
}

func (s *memFixtureSource) Events(ctx context.Context, computerID string, afterSequence uint64) ([]computerevent.DurableEvent, error) {
	return s.EventsPage(ctx, computerID, afterSequence, 0)
}

func (s *memFixtureSource) Head(ctx context.Context, computerID string) (*computerevent.Head, error) {
	if len(s.records) == 0 {
		return nil, nil
	}
	head := s.records[len(s.records)-1].Request.Next
	return &head, nil
}

func (s *memFixtureSource) CompareAndSwap(ctx context.Context, request computerevent.CASRequest) (computerevent.Receipt, error) {
	return computerevent.Receipt{}, errors.New("mem fixture source: offline rebuilder cannot HeadCAS events")
}

func (s *memFixtureSource) PinEvent(ctx context.Context, computerID string, canonicalEvent []byte, requestCommitment string) (computerevent.PinResult, error) {
	return computerevent.PinResult{ArtifactDigest: computerevent.DigestBytes(canonicalEvent)}, nil
}

func (s *memFixtureSource) PinEventPayload(ctx context.Context, computerID, eventID string, payload []byte, mediaType, privacyClass, requestCommitment string) (computerevent.PinResult, error) {
	return computerevent.PinResult{ArtifactDigest: computerevent.DigestBytes(payload)}, nil
}

func (s *memFixtureSource) FetchPayload(ctx context.Context, computerID, artifactDigest string) ([]byte, error) {
	return nil, errors.New("mem fixture source: fixture chains carry no payloads")
}

func (s *memFixtureSource) VerifyEventHeadReceipt(ctx context.Context, receipt computerevent.Receipt, request computerevent.CASRequest) error {
	return nil
}

// witnessFromBlob unpacks a published base blob and extracts its content
// witness with the production extraction configuration.
func witnessFromBlob(t *testing.T, ctx context.Context, blobPath, computerID string) (selfdevprotocol.VMLocalContentWitness, string) {
	t.Helper()
	dir := t.TempDir()
	if err := Unpack(blobPath, dir); err != nil {
		t.Fatalf("unpack published blob: %v", err)
	}
	store, err := choirstore.Open(filepath.Join(dir, "runtime.db"))
	if err != nil {
		t.Fatalf("open unpacked store: %v", err)
	}
	head, err := store.Head(ctx, computerID)
	if err != nil || head == nil {
		_ = store.Close()
		t.Fatalf("read unpacked head: %v", err)
	}
	workspace := store.TexturePath()
	if err := store.Close(); err != nil {
		t.Fatalf("close unpacked store: %v", err)
	}
	witness, err := witnessForWorkspace(ctx, computerID, head.CanonicalEventHead, workspace)
	if err != nil {
		t.Fatalf("extract unpacked witness: %v", err)
	}
	return witness, head.CanonicalEventHead
}

func TestSeededRebuildMatchesGenesisAndReadsNoPrefix(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	computerID := "computer-seed-equiv"
	digests := buildChainN(t, root, computerID, 5)
	key := fixtureKey(5)

	seedResult := runRebuild(t, ctx, Config{
		ComputerID: computerID, TargetHead: digests[1], ArtifactsRoot: root,
		ScratchDir: t.TempDir(), KeyMaterial: key, BatchSize: 100,
		MemoryLimitRSS: 512 * 1024 * 1024, SeedPolicy: SeedNone, TargetSequence: 2,
	}, fixtureReplay(NewDiskEventSource(root, computerID, digests[1])))
	if seedResult.Descriptor.Sequence != 2 || seedResult.Seed != nil {
		t.Fatalf("seed base = seq %d seed %v, want genesis W=2", seedResult.Descriptor.Sequence, seedResult.Seed)
	}

	recorder := fixtureReplay(NewDiskEventSource(root, computerID, digests[4]))
	seeded := runRebuild(t, ctx, Config{
		ComputerID: computerID, TargetHead: digests[4], ArtifactsRoot: root,
		ScratchDir: t.TempDir(), KeyMaterial: key, BatchSize: 100,
		MemoryLimitRSS: 512 * 1024 * 1024, SeedPolicy: SeedAuto, SeedRequired: true,
	}, recorder)
	if seeded.Seed == nil || seeded.Seed.Sequence != 2 {
		t.Fatalf("seeded run did not select the W=2 base: %+v", seeded.Seed)
	}
	if seeded.Descriptor.Sequence != 5 || seeded.Descriptor.CanonicalHead != digests[4] {
		t.Fatalf("seeded descriptor = (%d, %s), want (5, %s)", seeded.Descriptor.Sequence, seeded.Descriptor.CanonicalHead, digests[4])
	}
	if seeded.TargetSequence != 5 {
		t.Fatalf("seeded target sequence = %d, want 5", seeded.TargetSequence)
	}
	requests := recorder.requests()
	if len(requests) == 0 {
		t.Fatal("seeded replay fetched no pages")
	}
	if requests[0] != 2 {
		t.Fatalf("seeded replay first request after=%d, want tail start 2", requests[0])
	}
	if min := recorder.minRequested(); min < 2 {
		t.Fatalf("seeded replay requested prefix after=%d below seed watermark 2", min)
	}

	genesis := runRebuild(t, ctx, Config{
		ComputerID: computerID, TargetHead: digests[4], ArtifactsRoot: root,
		ScratchDir: t.TempDir(), KeyMaterial: key, BatchSize: 100,
		MemoryLimitRSS: 512 * 1024 * 1024, SeedPolicy: SeedNone,
	}, fixtureReplay(NewDiskEventSource(root, computerID, digests[4])))
	if genesis.Seed != nil {
		t.Fatalf("genesis run seeded: %+v", genesis.Seed)
	}

	seededWitness, seededHead := witnessFromBlob(t, ctx, seeded.BlobPath, computerID)
	genesisWitness, genesisHead := witnessFromBlob(t, ctx, genesis.BlobPath, computerID)
	if seededHead != digests[4] || genesisHead != digests[4] {
		t.Fatalf("published heads = (%s, %s), want %s", seededHead, genesisHead, digests[4])
	}
	if err := selfdevprotocol.WitnessContentMatches(seededWitness, genesisWitness); err != nil {
		t.Fatalf("seeded and genesis witnesses diverge: %v", err)
	}
}

func TestSeedRequiredRefusesGenesisFallback(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	computerID := "computer-seed-required"
	digests := buildChainN(t, root, computerID, 2)
	rebuilder, err := NewRebuilder(Config{
		ComputerID: computerID, TargetHead: digests[1], ArtifactsRoot: root,
		ScratchDir: t.TempDir(), KeyMaterial: fixtureKey(3), BatchSize: 100,
		MemoryLimitRSS: 512 * 1024 * 1024, SeedRequired: true, TargetSequence: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rebuilder.Run(ctx, fixtureReplay(NewDiskEventSource(root, computerID, digests[1]))); !errors.Is(err, ErrBaseRefused) {
		t.Fatalf("seed-required genesis fallback admitted: %v", err)
	}
	entries, readErr := os.ReadDir(filepath.Join(root, "sha256", Namespace))
	if readErr == nil {
		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".descriptor.json") {
				t.Fatalf("refused run published a descriptor: %s", entry.Name())
			}
		}
	}
}

func TestPinnedSeedRefusesCorruptBlobWhileAutoFallsBackToGenesis(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	computerID := "computer-seed-corrupt"
	digests := buildChainN(t, root, computerID, 3)
	key := fixtureKey(7)
	published := runRebuild(t, ctx, Config{
		ComputerID: computerID, TargetHead: digests[0], ArtifactsRoot: root,
		ScratchDir: t.TempDir(), KeyMaterial: key, BatchSize: 100,
		MemoryLimitRSS: 512 * 1024 * 1024, SeedPolicy: SeedNone, TargetSequence: 1,
	}, fixtureReplay(NewDiskEventSource(root, computerID, digests[0])))

	raw, err := os.ReadFile(published.BlobPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(published.BlobPath, append(append([]byte{}, raw...), 0x00), 0o600); err != nil {
		t.Fatal(err)
	}

	pinned, err := NewRebuilder(Config{
		ComputerID: computerID, TargetHead: digests[2], ArtifactsRoot: root,
		ScratchDir: t.TempDir(), KeyMaterial: key, BatchSize: 100,
		MemoryLimitRSS: 512 * 1024 * 1024, SeedPolicy: SeedPinned,
		SeedBaseRef: published.Descriptor.BlobSHA256, TargetSequence: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pinned.Run(ctx, fixtureReplay(NewDiskEventSource(root, computerID, digests[2]))); !errors.Is(err, ErrBaseRefused) {
		t.Fatalf("corrupt pinned seed admitted: %v", err)
	}

	auto := runRebuild(t, ctx, Config{
		ComputerID: computerID, TargetHead: digests[2], ArtifactsRoot: root,
		ScratchDir: t.TempDir(), KeyMaterial: key, BatchSize: 100,
		MemoryLimitRSS: 512 * 1024 * 1024, SeedPolicy: SeedAuto, TargetSequence: 3,
	}, fixtureReplay(NewDiskEventSource(root, computerID, digests[2])))
	if auto.Seed != nil {
		t.Fatalf("auto seeded from a corrupt base: %+v", auto.Seed)
	}
	if auto.Descriptor.CanonicalHead != digests[2] || auto.Descriptor.Sequence != 3 {
		t.Fatalf("auto fallback descriptor = (%d, %s), want (3, %s)", auto.Descriptor.Sequence, auto.Descriptor.CanonicalHead, digests[2])
	}
}

func TestSeededRebuildRefusesNonAncestorSeedAndFallsBack(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	computerID := "computer-seed-ancestry"
	chainA := buildChainN(t, root, computerID, 5)
	chainB := buildChainN(t, root, computerID, 5)
	key := fixtureKey(13)

	published := runRebuild(t, ctx, Config{
		ComputerID: computerID, TargetHead: chainA[1], ArtifactsRoot: root,
		ScratchDir: t.TempDir(), KeyMaterial: key, BatchSize: 100,
		MemoryLimitRSS: 512 * 1024 * 1024, SeedPolicy: SeedNone, TargetSequence: 2,
	}, fixtureReplay(NewDiskEventSource(root, computerID, chainA[1])))

	pinned, err := NewRebuilder(Config{
		ComputerID: computerID, TargetHead: chainB[4], ArtifactsRoot: root,
		ScratchDir: t.TempDir(), KeyMaterial: key, BatchSize: 100,
		MemoryLimitRSS: 512 * 1024 * 1024, SeedPolicy: SeedPinned,
		SeedBaseRef: published.Descriptor.BlobSHA256, TargetSequence: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pinned.Run(ctx, fixtureReplay(NewDiskEventSource(root, computerID, chainB[4]))); !errors.Is(err, ErrBaseRefused) {
		t.Fatalf("non-ancestor pinned seed admitted: %v", err)
	}

	auto := runRebuild(t, ctx, Config{
		ComputerID: computerID, TargetHead: chainB[4], ArtifactsRoot: root,
		ScratchDir: t.TempDir(), KeyMaterial: key, BatchSize: 100,
		MemoryLimitRSS: 512 * 1024 * 1024, SeedPolicy: SeedAuto, TargetSequence: 5,
	}, fixtureReplay(NewDiskEventSource(root, computerID, chainB[4])))
	if auto.Seed != nil {
		t.Fatalf("auto seeded from a non-ancestor base: %+v", auto.Seed)
	}
	if auto.Descriptor.CanonicalHead != chainB[4] {
		t.Fatalf("ancestry fallback head = %s, want %s", auto.Descriptor.CanonicalHead, chainB[4])
	}
}

func TestMaxTailRefusesOverBoundBeforeReplay(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	computerID := "computer-max-tail"
	digests := buildChainN(t, root, computerID, 6)
	key := fixtureKey(9)

	runRebuild(t, ctx, Config{
		ComputerID: computerID, TargetHead: digests[1], ArtifactsRoot: root,
		ScratchDir: t.TempDir(), KeyMaterial: key, BatchSize: 100,
		MemoryLimitRSS: 512 * 1024 * 1024, SeedPolicy: SeedNone, TargetSequence: 2,
	}, fixtureReplay(NewDiskEventSource(root, computerID, digests[1])))

	recorder := fixtureReplay(NewDiskEventSource(root, computerID, digests[5]))
	over, err := NewRebuilder(Config{
		ComputerID: computerID, TargetHead: digests[5], ArtifactsRoot: root,
		ScratchDir: t.TempDir(), KeyMaterial: key, BatchSize: 100,
		MemoryLimitRSS: 512 * 1024 * 1024, SeedPolicy: SeedAuto, SeedRequired: true,
		MaxTailEvents: 3, TargetSequence: 6,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := over.Run(ctx, recorder); !errors.Is(err, ErrBaseRefused) {
		t.Fatalf("max-tail over-bound run admitted: %v", err)
	}
	if len(recorder.requests()) != 0 {
		t.Fatalf("max-tail refusal touched the chain: %v", recorder.requests())
	}

	exact := runRebuild(t, ctx, Config{
		ComputerID: computerID, TargetHead: digests[5], ArtifactsRoot: root,
		ScratchDir: t.TempDir(), KeyMaterial: key, BatchSize: 100,
		MemoryLimitRSS: 512 * 1024 * 1024, SeedPolicy: SeedAuto, SeedRequired: true,
		MaxTailEvents: 4, TargetSequence: 6,
	}, fixtureReplay(NewDiskEventSource(root, computerID, digests[5])))
	if exact.Descriptor.Sequence != 6 {
		t.Fatalf("max-tail boundary run = seq %d, want 6", exact.Descriptor.Sequence)
	}
}

func TestDurableScratchResumeSkipsReplayAndStaysTailBounded(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	computerID := "computer-scratch-resume"
	digests := buildChainN(t, root, computerID, 6)
	key := fixtureKey(11)
	scratch := t.TempDir()

	runRebuild(t, ctx, Config{
		ComputerID: computerID, TargetHead: digests[1], ArtifactsRoot: root,
		ScratchDir: t.TempDir(), KeyMaterial: key, BatchSize: 100,
		MemoryLimitRSS: 512 * 1024 * 1024, SeedPolicy: SeedNone, TargetSequence: 2,
	}, fixtureReplay(NewDiskEventSource(root, computerID, digests[1])))

	rec1 := fixtureReplay(NewDiskEventSource(root, computerID, digests[4]))
	first := runRebuild(t, ctx, Config{
		ComputerID: computerID, TargetHead: digests[4], ArtifactsRoot: root,
		ScratchDir: scratch, KeyMaterial: key, BatchSize: 100,
		MemoryLimitRSS: 512 * 1024 * 1024, SeedPolicy: SeedAuto, SeedRequired: true,
	}, rec1)
	if first.Seed == nil || first.Seed.Sequence != 2 || first.Descriptor.Sequence != 5 {
		t.Fatalf("first seeded run = seed %v seq %d, want seed 2 seq 5", first.Seed, first.Descriptor.Sequence)
	}
	if min := rec1.minRequested(); min < 2 {
		t.Fatalf("first seeded run requested prefix after=%d", min)
	}

	rec2 := fixtureReplay(NewDiskEventSource(root, computerID, digests[4]))
	second := runRebuild(t, ctx, Config{
		ComputerID: computerID, TargetHead: digests[4], ArtifactsRoot: root,
		ScratchDir: scratch, KeyMaterial: key, BatchSize: 100,
		MemoryLimitRSS: 512 * 1024 * 1024, SeedPolicy: SeedAuto, SeedRequired: true,
	}, rec2)
	if second.Descriptor.Sequence != 5 || second.Descriptor.CanonicalHead != digests[4] {
		t.Fatalf("resume-at-target descriptor = (%d, %s), want (5, %s)", second.Descriptor.Sequence, second.Descriptor.CanonicalHead, digests[4])
	}
	if got := len(rec2.requests()); got != 0 {
		t.Fatalf("resume at target fetched %d pages: %v", got, rec2.requests())
	}

	rec3 := fixtureReplay(NewDiskEventSource(root, computerID, digests[5]))
	third := runRebuild(t, ctx, Config{
		ComputerID: computerID, TargetHead: digests[5], ArtifactsRoot: root,
		ScratchDir: scratch, KeyMaterial: key, BatchSize: 100,
		MemoryLimitRSS: 512 * 1024 * 1024, SeedPolicy: SeedAuto, SeedRequired: true,
	}, rec3)
	if third.Descriptor.Sequence != 6 || third.Descriptor.CanonicalHead != digests[5] {
		t.Fatalf("extended resume descriptor = (%d, %s), want (6, %s)", third.Descriptor.Sequence, third.Descriptor.CanonicalHead, digests[5])
	}
	if min := rec3.minRequested(); min < 5 {
		t.Fatalf("extended resume requested prefix after=%d below committed head 5", min)
	}
}

func TestPartialSeedScratchIsResetNotAdopted(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	computerID := "computer-scratch-partial"
	digests := buildChainN(t, root, computerID, 4)
	key := fixtureKey(15)
	published := runRebuild(t, ctx, Config{
		ComputerID: computerID, TargetHead: digests[1], ArtifactsRoot: root,
		ScratchDir: t.TempDir(), KeyMaterial: key, BatchSize: 100,
		MemoryLimitRSS: 512 * 1024 * 1024, SeedPolicy: SeedNone, TargetSequence: 2,
	}, fixtureReplay(NewDiskEventSource(root, computerID, digests[1])))

	scratch := t.TempDir()
	meta := seedMeta{
		BlobSHA256:    published.Descriptor.BlobSHA256,
		Sequence:      published.Descriptor.Sequence,
		CanonicalHead: published.Descriptor.CanonicalHead,
		InstalledAt:   time.Now().UTC().Format(time.RFC3339Nano),
	}
	rawMeta, err := json.Marshal(meta)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scratch, seedMetaFileName), rawMeta, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scratch, "runtime.db"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(scratch, "runtime.texture"), 0o755); err != nil {
		t.Fatal(err)
	}

	recorder := fixtureReplay(NewDiskEventSource(root, computerID, digests[3]))
	result := runRebuild(t, ctx, Config{
		ComputerID: computerID, TargetHead: digests[3], ArtifactsRoot: root,
		ScratchDir: scratch, KeyMaterial: key, BatchSize: 100,
		MemoryLimitRSS: 512 * 1024 * 1024, SeedPolicy: SeedAuto, SeedRequired: true,
	}, recorder)
	if result.Seed == nil || result.Seed.Sequence != 2 {
		t.Fatalf("partial seed scratch was not re-seeded: %+v", result.Seed)
	}
	if result.Descriptor.Sequence != 4 || result.Descriptor.CanonicalHead != digests[3] {
		t.Fatalf("partial seed descriptor = (%d, %s), want (4, %s)", result.Descriptor.Sequence, result.Descriptor.CanonicalHead, digests[3])
	}
	if requests := recorder.requests(); len(requests) == 0 || requests[0] != 2 {
		t.Fatalf("partial seed scratch did not replay from the seed tail: %v", requests)
	}
}

func TestMemoryGuardRefusesBeforePublishing(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	computerID := "computer-memory-guard"
	digests := buildChainN(t, root, computerID, 3)
	scratch := t.TempDir()
	sentinel := filepath.Join(scratch, "sentinel.txt")
	if err := os.WriteFile(sentinel, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}

	prevMeasure := processRSSBytes
	calls := 0
	processRSSBytes = func() int64 {
		calls++
		if calls == 1 {
			return 64 << 20 // baseline before replay
		}
		return 64<<20 + 1<<40 // runaway growth after the first sample
	}
	t.Cleanup(func() { processRSSBytes = prevMeasure })

	rebuilder, err := NewRebuilder(Config{
		ComputerID: computerID, TargetHead: digests[2], ArtifactsRoot: root,
		ScratchDir: scratch, KeyMaterial: fixtureKey(17), BatchSize: 100,
		MemoryLimitRSS: 64 << 20, SeedPolicy: SeedNone, TargetSequence: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rebuilder.Run(ctx, fixtureReplay(NewDiskEventSource(root, computerID, digests[2]))); !errors.Is(err, ErrMemoryLimitExceeded) {
		t.Fatalf("memory guard admitted the run: %v", err)
	}
	entries, readErr := os.ReadDir(filepath.Join(root, "sha256", Namespace))
	if readErr == nil {
		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".descriptor.json") {
				t.Fatalf("memory-refused run published a descriptor: %s", entry.Name())
			}
		}
	}
	if _, err := os.Stat(sentinel); err != nil {
		t.Fatalf("refused run destroyed durable scratch contents: %v", err)
	}
}

func TestScratchCompactionRunsAfterCloseBeforePacking(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	computerID := "computer-scratch-gc"
	digests := buildChainN(t, root, computerID, 3)
	scratch := t.TempDir()

	prevGC := runScratchDoltGC
	var gcCalls []string
	runScratchDoltGC = func(workspacePath string) error {
		gcCalls = append(gcCalls, workspacePath)
		return nil
	}
	t.Cleanup(func() { runScratchDoltGC = prevGC })

	result := runRebuild(t, ctx, Config{
		ComputerID: computerID, TargetHead: digests[2], ArtifactsRoot: root,
		ScratchDir: scratch, KeyMaterial: fixtureKey(19), BatchSize: 100,
		MemoryLimitRSS: 512 * 1024 * 1024, SeedPolicy: SeedNone, TargetSequence: 3,
	}, fixtureReplay(NewDiskEventSource(root, computerID, digests[2])))
	if len(gcCalls) != 1 {
		t.Fatalf("scratch dolt gc calls = %d, want exactly 1", len(gcCalls))
	}
	wantWorkspace := filepath.Join(scratch, "runtime.texture")
	if gcCalls[0] != wantWorkspace {
		t.Fatalf("scratch dolt gc workspace = %s, want %s", gcCalls[0], wantWorkspace)
	}
	if result.Descriptor.Sequence != 3 || result.Descriptor.CanonicalHead != digests[2] {
		t.Fatalf("compacted descriptor = (%d, %s), want (3, %s)", result.Descriptor.Sequence, result.Descriptor.CanonicalHead, digests[2])
	}
}

func TestScratchCompactionFailureRefusesPublication(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	computerID := "computer-scratch-gc-fail"
	digests := buildChainN(t, root, computerID, 3)
	scratch := t.TempDir()

	prevGC := runScratchDoltGC
	runScratchDoltGC = func(workspacePath string) error {
		return errors.New("dolt gc exploded")
	}
	t.Cleanup(func() { runScratchDoltGC = prevGC })

	rebuilder, err := NewRebuilder(Config{
		ComputerID: computerID, TargetHead: digests[2], ArtifactsRoot: root,
		ScratchDir: scratch, KeyMaterial: fixtureKey(21), BatchSize: 100,
		MemoryLimitRSS: 512 * 1024 * 1024, SeedPolicy: SeedNone, TargetSequence: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rebuilder.Run(ctx, fixtureReplay(NewDiskEventSource(root, computerID, digests[2]))); err == nil {
		t.Fatal("scratch compaction failure published a base")
	}
	entries, readErr := os.ReadDir(filepath.Join(root, "sha256", Namespace))
	if readErr == nil {
		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".descriptor.json") {
				t.Fatalf("gc-refused run published a descriptor: %s", entry.Name())
			}
		}
	}
	if _, err := os.Stat(filepath.Join(scratch, "runtime.db")); err != nil {
		t.Fatalf("gc-refused run discarded the durable scratch store: %v", err)
	}
}

func TestVerifyPublishedBaseDetectsTamper(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	computerID := "computer-readback"
	digests := buildChainN(t, root, computerID, 3)
	result := runRebuild(t, ctx, Config{
		ComputerID: computerID, TargetHead: digests[2], ArtifactsRoot: root,
		ScratchDir: t.TempDir(), KeyMaterial: fixtureKey(23), BatchSize: 100,
		MemoryLimitRSS: 512 * 1024 * 1024, SeedPolicy: SeedNone, TargetSequence: 3,
	}, fixtureReplay(NewDiskEventSource(root, computerID, digests[2])))

	if err := VerifyPublishedBase(ctx, root, result.Descriptor); err != nil {
		t.Fatalf("published base failed read-back verification: %v", err)
	}

	pristine, err := os.ReadFile(result.BlobPath)
	if err != nil {
		t.Fatal(err)
	}
	tamperedBlob := append(append([]byte{}, pristine...), 0x00)
	if err := os.WriteFile(result.BlobPath, tamperedBlob, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := VerifyPublishedBase(ctx, root, result.Descriptor); err == nil {
		t.Fatal("tampered blob passed read-back verification")
	}
	if err := os.WriteFile(result.BlobPath, pristine, 0o600); err != nil {
		t.Fatal(err)
	}

	badWitness := result.Descriptor
	badWitness.VMLocalContentWitness.ContentRoot = strings.Repeat("f", 64)
	if err := VerifyPublishedBase(ctx, root, badWitness); err == nil {
		t.Fatal("tampered witness passed read-back verification")
	}
}

func TestPublishedCompactBaseInstallsAndReplaysToTarget(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	computerID := "computer-compact-install"
	digests := buildChainN(t, root, computerID, 5)
	key := fixtureKey(25)
	disk := NewDiskEventSource(root, computerID)

	runRebuild(t, ctx, Config{
		ComputerID: computerID, TargetHead: digests[1], ArtifactsRoot: root,
		ScratchDir: t.TempDir(), KeyMaterial: key, BatchSize: 100,
		MemoryLimitRSS: 512 * 1024 * 1024, SeedPolicy: SeedNone, TargetSequence: 2,
	}, fixtureReplay(NewDiskEventSource(root, computerID, digests[1])))
	seeded := runRebuild(t, ctx, Config{
		ComputerID: computerID, TargetHead: digests[4], ArtifactsRoot: root,
		ScratchDir: t.TempDir(), KeyMaterial: key, BatchSize: 100,
		MemoryLimitRSS: 512 * 1024 * 1024, SeedPolicy: SeedAuto, SeedRequired: true,
	}, fixtureReplay(NewDiskEventSource(root, computerID, digests[4])))

	blob, err := os.ReadFile(seeded.BlobPath)
	if err != nil {
		t.Fatal(err)
	}
	src := &fakeBaseSource{
		sequence:   seeded.Descriptor.Sequence,
		baseRef:    seeded.Descriptor.BlobSHA256,
		descriptor: seeded.Descriptor,
		blob:       blob,
		disk:       disk,
	}
	installDir := t.TempDir()
	descriptor, err := InstallVerifiedBase(ctx, src, installDir, "runtime.db", computerID, digests[4], 5)
	if err != nil {
		t.Fatalf("published compact base refused install: %v", err)
	}
	if descriptor.Sequence != 5 || descriptor.CanonicalHead != digests[4] {
		t.Fatalf("installed descriptor = (%d, %s), want (5, %s)", descriptor.Sequence, descriptor.CanonicalHead, digests[4])
	}

	installed, err := choirstore.Open(filepath.Join(installDir, "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	head, err := installed.Head(ctx, computerID)
	if err != nil || head == nil {
		_ = installed.Close()
		t.Fatalf("installed head unreadable: %v", err)
	}
	workspace := installed.TexturePath()
	if err := installed.Close(); err != nil {
		t.Fatal(err)
	}
	witness, err := witnessForWorkspace(ctx, computerID, head.CanonicalEventHead, workspace)
	if err != nil {
		t.Fatalf("extract installed witness: %v", err)
	}
	if err := selfdevprotocol.WitnessContentMatches(witness, seeded.Descriptor.VMLocalContentWitness); err != nil {
		t.Fatalf("installed content witness does not match the published descriptor: %v", err)
	}
}

func TestPartialSeedScratchHonorsExplicitGenesisRepair(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	computerID := "computer-scratch-partial-repair"
	digests := buildChainN(t, root, computerID, 3)
	key := fixtureKey(35)
	published := runRebuild(t, ctx, Config{
		ComputerID: computerID, TargetHead: digests[0], ArtifactsRoot: root,
		ScratchDir: t.TempDir(), KeyMaterial: key, BatchSize: 100,
		MemoryLimitRSS: 512 * 1024 * 1024, SeedPolicy: SeedNone, TargetSequence: 1,
	}, fixtureReplay(NewDiskEventSource(root, computerID, digests[0])))

	scratch := t.TempDir()
	meta := seedMeta{
		BlobSHA256:    published.Descriptor.BlobSHA256,
		Sequence:      published.Descriptor.Sequence,
		CanonicalHead: published.Descriptor.CanonicalHead,
		InstalledAt:   time.Now().UTC().Format(time.RFC3339Nano),
	}
	rawMeta, err := json.Marshal(meta)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scratch, seedMetaFileName), rawMeta, 0o600); err != nil {
		t.Fatal(err)
	}
	// Marker without a workspace: the seed install crashed mid-unpack.
	if err := os.WriteFile(filepath.Join(scratch, "runtime.db"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	recorder := fixtureReplay(NewDiskEventSource(root, computerID, digests[2]))
	result := runRebuild(t, ctx, Config{
		ComputerID: computerID, TargetHead: digests[2], ArtifactsRoot: root,
		ScratchDir: scratch, KeyMaterial: key, BatchSize: 100,
		MemoryLimitRSS: 512 * 1024 * 1024, SeedPolicy: SeedNone, TargetSequence: 3,
	}, recorder)
	if result.Seed != nil {
		t.Fatalf("explicit repair adopted a seed: %+v", result.Seed)
	}
	if result.Descriptor.Sequence != 3 || result.Descriptor.CanonicalHead != digests[2] {
		t.Fatalf("repair descriptor = (%d, %s), want (3, %s)", result.Descriptor.Sequence, result.Descriptor.CanonicalHead, digests[2])
	}
	if requests := recorder.requests(); len(requests) == 0 || requests[0] != 0 {
		t.Fatalf("explicit repair did not replay genesis: %v", requests)
	}
}

func TestAutoFallbackGenesisRefusesBeyondFirstBaseBound(t *testing.T) {
	ctx := context.Background()
	computerID := "computer-genesis-bound"
	records := generateChain(t, computerID, int(MaxRecoveryTailEvents)+1)
	head := records[len(records)-1].Request.Next
	source := fixtureReplay(&memFixtureSource{computerID: computerID, records: records})

	rebuilder, err := NewRebuilder(Config{
		ComputerID: computerID, TargetHead: head.CanonicalEventHead,
		ArtifactsRoot: t.TempDir(), ScratchDir: t.TempDir(), KeyMaterial: fixtureKey(27),
		BatchSize: 100, MemoryLimitRSS: 512 * 1024 * 1024, TargetSequence: head.Sequence,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rebuilder.Run(ctx, source); !errors.Is(err, ErrBaseRefused) {
		t.Fatalf("over-bound auto genesis admitted: %v", err)
	}
	if len(source.requests()) != 0 {
		t.Fatalf("over-bound auto genesis refusal touched the chain: %v", source.requests())
	}
}

func TestExplicitGenesisRepairAdmitsUnfrozenTarget(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	computerID := "computer-genesis-repair"
	digests := buildChainN(t, root, computerID, 2)

	// Routine auto fallback refuses an unfrozen target: the bound cannot be proven.
	refused, err := NewRebuilder(Config{
		ComputerID: computerID, TargetHead: digests[0], ArtifactsRoot: root,
		ScratchDir: t.TempDir(), KeyMaterial: fixtureKey(29), BatchSize: 100,
		MemoryLimitRSS: 512 * 1024 * 1024, SeedPolicy: SeedAuto,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := refused.Run(ctx, fixtureReplay(NewDiskEventSource(root, computerID))); !errors.Is(err, ErrBaseRefused) {
		t.Fatalf("unfrozen auto genesis admitted: %v", err)
	}

	// Explicit repair (SeedNone) replays genesis and resolves the target from the head.
	repair := runRebuild(t, ctx, Config{
		ComputerID: computerID, TargetHead: digests[0], ArtifactsRoot: root,
		ScratchDir: t.TempDir(), KeyMaterial: fixtureKey(29), BatchSize: 100,
		MemoryLimitRSS: 512 * 1024 * 1024, SeedPolicy: SeedNone,
	}, fixtureReplay(NewDiskEventSource(root, computerID)))
	if repair.Descriptor.CanonicalHead != digests[0] || repair.Descriptor.Sequence != 1 {
		t.Fatalf("repair descriptor = (%d, %s), want (1, %s)", repair.Descriptor.Sequence, repair.Descriptor.CanonicalHead, digests[0])
	}
	if repair.TargetSequence != 1 {
		t.Fatalf("repair target sequence = %d, want 1", repair.TargetSequence)
	}
}
