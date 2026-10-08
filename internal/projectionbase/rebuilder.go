package projectionbase

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
)

// ErrMemoryLimitExceeded is the typed refusal for a run whose process RSS grew
// beyond MemoryLimitRSS. The durable scratch store is left in place for resume.
var ErrMemoryLimitExceeded = errors.New("projection base: replay memory limit exceeded")

// Rebuilder executes an isolated offline rebuild outside the guest VM and
// publishes a content-addressed ProjectionBase blob. Routine runs seed from the
// latest compatible verified published base and replay only its tail; genesis
// replay is exceptional repair bounded by the caller's MaxTailEvents.
type Rebuilder struct {
	cfg Config
}

// NewRebuilder returns an offline projection rebuilder.
func NewRebuilder(cfg Config) (*Rebuilder, error) {
	cfg.ComputerID = strings.TrimSpace(cfg.ComputerID)
	cfg.TargetHead = strings.ToLower(strings.TrimSpace(cfg.TargetHead))
	cfg.ArtifactsRoot = strings.TrimSpace(cfg.ArtifactsRoot)
	cfg.ScratchDir = strings.TrimSpace(cfg.ScratchDir)
	cfg.SeedBaseRef = strings.ToLower(strings.TrimSpace(cfg.SeedBaseRef))
	policy, err := normalizeSeedPolicy(cfg.SeedPolicy)
	if err != nil {
		return nil, err
	}
	cfg.SeedPolicy = policy
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = DefaultBatchSize
	}
	if cfg.MemoryLimitRSS <= 0 {
		cfg.MemoryLimitRSS = DefaultMemoryLimitRSSBytes
	}
	return &Rebuilder{cfg: cfg}, nil
}

// Result contains the output of a successful offline rebuild.
type Result struct {
	Descriptor     Descriptor  `json:"descriptor"`
	BlobPath       string      `json:"blob_path"`
	Seed           *Descriptor `json:"seed,omitempty"`
	TargetSequence uint64      `json:"target_sequence"`
}

// CASReplaySource combines EventSource, ArtifactReader, and HeadCAS interfaces.
type CASReplaySource interface {
	computerevent.PagedEventSource
	computerevent.EventSource
	computerevent.ArtifactPinner
	computerevent.ArtifactReader
	computerevent.HeadCAS
	computerevent.ReceiptVerifier
	Head(ctx context.Context, computerID string) (*computerevent.Head, error)
}

// processRSSBytes reports the process resident set size where the platform
// exposes it, falling back to runtime OS memory. Indirected for tests.
var processRSSBytes = measureProcessRSSBytes

func measureProcessRSSBytes() int64 {
	if raw, err := os.ReadFile("/proc/self/statm"); err == nil {
		fields := strings.Fields(string(raw))
		if len(fields) >= 2 {
			if pages, err := strconv.ParseInt(fields[1], 10, 64); err == nil && pages > 0 {
				return pages * int64(os.Getpagesize())
			}
		}
	}
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return int64(m.Sys)
}

// runScratchDoltGC compacts the closed scratch Dolt workspace before packing.
// It is mandatory: a scratch that cannot be compacted is never published.
var runScratchDoltGC = choirstore.RunDoltGCWorkspace

// memoryGuardSampleEvery bounds RSS sampling overhead during replay.
const memoryGuardSampleEvery = 256

// memoryGuardObserver samples process RSS growth during replay and cancels the
// replay context once the configured budget is exceeded. Cancellation lets the
// appender flush a durable checkpoint so the scratch store resumes safely.
type memoryGuardObserver struct {
	limit    int64
	baseline int64
	cancel   context.CancelFunc
	exceeded atomic.Bool
	applied  atomic.Uint64
}

func (g *memoryGuardObserver) PageFetched(uint64, int) { g.sample() }

func (g *memoryGuardObserver) RecordApplied(uint64) {
	if g.applied.Add(1)%memoryGuardSampleEvery == 0 {
		g.sample()
	}
}

func (g *memoryGuardObserver) CheckpointCommitted(uint64) {}

func (g *memoryGuardObserver) sample() {
	if g.exceeded.Load() {
		return
	}
	if processRSSBytes()-g.baseline > g.limit {
		g.exceeded.Store(true)
		g.cancel()
	}
}

// boundedReplaySource enforces the replay window: pages are never requested
// before the seed/local watermark (no prefix reads) nor beyond the frozen
// target sequence.
type boundedReplaySource struct {
	CASReplaySource
	floor   uint64
	ceiling uint64
}

func (s *boundedReplaySource) EventsPage(ctx context.Context, computerID string, afterSequence uint64, pageSize int) ([]computerevent.DurableEvent, error) {
	if afterSequence < s.floor {
		return nil, fmt.Errorf("%w: replay requested prefix after=%d below watermark=%d", ErrBaseRefused, afterSequence, s.floor)
	}
	if s.ceiling != 0 && afterSequence > s.ceiling {
		return nil, fmt.Errorf("%w: replay requested after=%d beyond frozen target=%d", ErrBaseRefused, afterSequence, s.ceiling)
	}
	return s.CASReplaySource.EventsPage(ctx, computerID, afterSequence, pageSize)
}

// pagedTailAdapter exposes PagedEventSource as the TailSource ResolveTargetSequence consumes.
type pagedTailAdapter struct {
	source computerevent.PagedEventSource
}

func (a pagedTailAdapter) TailPage(ctx context.Context, computerID string, afterSequence uint64, pageSize int) ([]computerevent.DurableEvent, error) {
	return a.source.EventsPage(ctx, computerID, afterSequence, pageSize)
}

// Run executes the incremental offline rebuild into an isolated scratch
// directory and publishes the base blob: seed selection and verification,
// frozen target/head/ancestry checks, bounded no-prefix replay, the serving
// vocabulary fence, mandatory scratch compaction, packing, and install/read-back
// witness verification before the descriptor sidecar is published.
func (r *Rebuilder) Run(ctx context.Context, source CASReplaySource) (*Result, error) {
	if source == nil {
		return nil, fmt.Errorf("rebuilder: CAS replay source is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	cfg := r.cfg

	baselineRSS := processRSSBytes()

	chainHead, err := source.Head(ctx, cfg.ComputerID)
	if err != nil {
		return nil, fmt.Errorf("rebuilder: read canonical chain head: %w", err)
	}
	if chainHead == nil {
		return nil, fmt.Errorf("%w: no canonical chain head is available for %s", ErrBaseRefused, cfg.ComputerID)
	}
	targetSequence := cfg.TargetSequence
	switch {
	case chainHead.CanonicalEventHead == cfg.TargetHead:
		if targetSequence != 0 && targetSequence != chainHead.Sequence {
			return nil, fmt.Errorf("%w: frozen target sequence %d does not match chain head sequence %d", ErrBaseRefused, targetSequence, chainHead.Sequence)
		}
		targetSequence = chainHead.Sequence
	case targetSequence != 0 && chainHead.Sequence < targetSequence:
		return nil, fmt.Errorf("%w: frozen target sequence %d is beyond the chain head %d", ErrBaseRefused, targetSequence, chainHead.Sequence)
	}

	scratchDir := cfg.ScratchDir
	if scratchDir == "" {
		tempDir, err := os.MkdirTemp("", "choir-projection-base-rebuild-*")
		if err != nil {
			return nil, fmt.Errorf("rebuilder: create scratch directory: %w", err)
		}
		scratchDir = tempDir
		defer os.RemoveAll(tempDir)
	} else if err := os.MkdirAll(scratchDir, 0o755); err != nil {
		return nil, fmt.Errorf("rebuilder: create scratch directory: %w", err)
	}
	markerName := "runtime.db"
	markerPath := filepath.Join(scratchDir, markerName)

	meta, err := readSeedMeta(filepath.Join(scratchDir, seedMetaFileName))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBaseRefused, err)
	}
	_, statErr := os.Stat(markerPath)
	markerExists := statErr == nil
	if statErr != nil && !os.IsNotExist(statErr) {
		return nil, fmt.Errorf("rebuilder: stat scratch store: %w", statErr)
	}

	var scratchStore *choirstore.Store
	var seed *Descriptor
	switch {
	case markerExists:
		// Durable scratch: resume from its committed head. Seeding is skipped;
		// the appender validates every replayed event against the committed
		// chain, so a resumed store can never adopt foreign state.
		opened, err := choirstore.Open(markerPath)
		if err != nil {
			return nil, fmt.Errorf("%w: scratch store does not open: %v", ErrBaseRefused, err)
		}
		head, err := opened.Head(ctx, cfg.ComputerID)
		if err != nil {
			_ = opened.Close()
			return nil, fmt.Errorf("%w: scratch store head: %v", ErrBaseRefused, err)
		}
		if head == nil && meta != nil {
			// A crashed seed install: the marker landed but no projection did.
			// Reset and reinstall (or, for explicit SeedNone repair, start
			// clean) rather than adopting partial bytes.
			if err := opened.Close(); err != nil {
				return nil, fmt.Errorf("rebuilder: close partial scratch store: %w", err)
			}
			log.Printf("rebuilder: resetting partial seed scratch %s", scratchDir)
			if err := resetScratchContent(scratchDir); err != nil {
				return nil, fmt.Errorf("%w: reset partial seed scratch: %v", ErrBaseRefused, err)
			}
			if cfg.SeedPolicy != SeedNone {
				seed, targetSequence, err = r.seedScratch(ctx, source, scratchDir, targetSequence)
				if err != nil {
					return nil, err
				}
			}
			opened, err = choirstore.Open(markerPath)
			if err != nil {
				return nil, fmt.Errorf("rebuilder: open seeded scratch store: %w", err)
			}
		} else if head != nil && meta != nil {
			if err := validateResumedSeed(head, meta); err != nil {
				_ = opened.Close()
				return nil, err
			}
		}
		scratchStore = opened
	case meta != nil:
		// A crashed seed install before the marker landed: restart the install
		// (or, for explicit SeedNone repair, start clean).
		if err := resetScratchContent(scratchDir); err != nil {
			return nil, fmt.Errorf("%w: reset partial seed scratch: %v", ErrBaseRefused, err)
		}
		if cfg.SeedPolicy != SeedNone {
			seed, targetSequence, err = r.seedScratch(ctx, source, scratchDir, targetSequence)
			if err != nil {
				return nil, err
			}
		}
		scratchStore, err = choirstore.Open(markerPath)
		if err != nil {
			return nil, fmt.Errorf("rebuilder: open seeded scratch store: %w", err)
		}
	default:
		if cfg.SeedPolicy != SeedNone {
			seed, targetSequence, err = r.seedScratch(ctx, source, scratchDir, targetSequence)
			if err != nil {
				return nil, err
			}
		}
		scratchStore, err = choirstore.Open(markerPath)
		if err != nil {
			return nil, fmt.Errorf("rebuilder: open scratch store: %w", err)
		}
	}
	closed := false
	defer func() {
		if !closed {
			_ = scratchStore.Close()
		}
	}()

	localHead, err := scratchStore.Head(ctx, cfg.ComputerID)
	if err != nil {
		return nil, fmt.Errorf("rebuilder: read scratch head: %w", err)
	}
	if seed != nil && localHead != nil && localHead.Sequence < seed.Sequence {
		return nil, fmt.Errorf("%w: scratch head %d is behind seed watermark %d", ErrBaseRefused, localHead.Sequence, seed.Sequence)
	}
	if targetSequence == 0 && localHead != nil && localHead.CanonicalEventHead != cfg.TargetHead {
		// Resolve the frozen target from the committed local head without
		// reading the prefix; resolution pages are tail-only.
		resolved, err := ResolveTargetSequence(ctx, pagedTailAdapter{source}, cfg.ComputerID, cfg.TargetHead, localHead.Sequence, localHead.CanonicalEventHead)
		if err != nil {
			return nil, err
		}
		targetSequence = resolved
	}
	if localHead == nil && seed == nil && cfg.SeedPolicy == SeedAuto {
		// Routine path fell back to genesis: refuse beyond the first-base bound.
		switch {
		case targetSequence == 0:
			return nil, fmt.Errorf("%w: no compatible verified base and no frozen target sequence; pass TargetSequence or use explicit repair", ErrBaseRefused)
		case targetSequence > MaxRecoveryTailEvents:
			return nil, fmt.Errorf("%w: no compatible verified base and genesis replay of %d events exceeds the %d-event first-base bound; explicit repair required", ErrBaseRefused, targetSequence, MaxRecoveryTailEvents)
		}
	}
	if cfg.MaxTailEvents > 0 {
		if targetSequence == 0 {
			return nil, fmt.Errorf("%w: MaxTailEvents requires a frozen target sequence", ErrBaseRefused)
		}
		seedSequence := uint64(0)
		if seed != nil {
			seedSequence = seed.Sequence
		}
		if targetSequence-seedSequence > cfg.MaxTailEvents {
			return nil, fmt.Errorf("%w: replay span %d exceeds MaxTailEvents %d", ErrBaseRefused, targetSequence-seedSequence, cfg.MaxTailEvents)
		}
	}

	cipher, err := computerevent.NewPrivateArtifactCipher(cfg.ComputerID, cfg.KeyMaterial)
	if err != nil {
		return nil, fmt.Errorf("rebuilder: initialize privacy cipher: %w", err)
	}
	appender, err := computerevent.NewComputerEventAppender(
		cfg.ComputerID,
		source,
		scratchStore,
		source,
		source,
	)
	if err != nil {
		return nil, fmt.Errorf("rebuilder: initialize event appender: %w", err)
	}
	appender.SetPayloadResolver(source, cipher)
	appender.SetReplayMode(true)

	runCtx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()
	guard := &memoryGuardObserver{limit: cfg.MemoryLimitRSS, baseline: baselineRSS, cancel: cancelRun}
	appender.SetReplayObserver(guard)
	floor := uint64(0)
	if seed != nil {
		floor = seed.Sequence
	}
	if localHead != nil && localHead.Sequence > floor {
		floor = localHead.Sequence
	}
	bounded := &boundedReplaySource{CASReplaySource: source, floor: floor, ceiling: targetSequence}

	if err := appender.ReconstructThrough(runCtx, bounded, cfg.TargetHead); err != nil {
		if guard.exceeded.Load() {
			return nil, fmt.Errorf("%w: process RSS grew beyond %d bytes during replay: %v", ErrMemoryLimitExceeded, cfg.MemoryLimitRSS, err)
		}
		return nil, fmt.Errorf("rebuilder: replay through target head %s: %w", cfg.TargetHead, err)
	}
	if guard.exceeded.Load() {
		return nil, fmt.Errorf("%w: process RSS grew beyond %d bytes during replay", ErrMemoryLimitExceeded, cfg.MemoryLimitRSS)
	}

	finalHead, err := scratchStore.Head(ctx, cfg.ComputerID)
	if err != nil || finalHead == nil {
		return nil, fmt.Errorf("rebuilder: read scratch head after replay: %w", err)
	}
	if finalHead.CanonicalEventHead != cfg.TargetHead {
		return nil, fmt.Errorf("rebuilder: final head mismatch: got %s, want %s", finalHead.CanonicalEventHead, cfg.TargetHead)
	}
	if targetSequence != 0 && finalHead.Sequence != targetSequence {
		return nil, fmt.Errorf("rebuilder: final head sequence %d does not match frozen target %d", finalHead.Sequence, targetSequence)
	}
	targetSequence = finalHead.Sequence

	// The serving fence plus report stamp. Replay deposits were upcast to the
	// active V2 vocabulary at deposit time (Replay's versioned deposit upcast);
	// this pass rewrites any straggler references and fences the store before
	// the witness and publish.
	if _, err := scratchStore.MigrateAndFenceServingVocabulary(ctx, true, nil); err != nil {
		return nil, fmt.Errorf("rebuilder: vocabulary migration refused: %w", err)
	}
	// Durability: commit any staged replay/migration writes before the witness
	// is taken, so the packed workspace is a committed root and the mandatory
	// DOLT_GC below runs over settled history.
	if err := scratchStore.CommitReplay(ctx); err != nil {
		return nil, fmt.Errorf("rebuilder: commit scratch before packing: %w", err)
	}
	witness, err := witnessForWorkspace(ctx, cfg.ComputerID, finalHead.CanonicalEventHead, scratchStore.TexturePath())
	if err != nil {
		return nil, err
	}
	if err := scratchStore.Close(); err != nil {
		return nil, fmt.Errorf("rebuilder: close scratch store: %w", err)
	}
	closed = true

	// Mandatory compaction of the closed scratch workspace before packing: the
	// noms journal is collectible garbage and must never travel inside a
	// published base blob.
	workspacePath := choirstore.TextureWorkspacePath(markerPath)
	if err := runScratchDoltGC(workspacePath); err != nil {
		return nil, fmt.Errorf("rebuilder: compact closed scratch store: %w", err)
	}
	if rss := processRSSBytes(); rss-baselineRSS > cfg.MemoryLimitRSS {
		return nil, fmt.Errorf("%w: process RSS grew by %d bytes above baseline, exceeding %d bytes before publication", ErrMemoryLimitExceeded, rss-baselineRSS, cfg.MemoryLimitRSS)
	}
	if err := cleanScratchBookkeeping(scratchDir); err != nil {
		return nil, fmt.Errorf("rebuilder: clean scratch bookkeeping: %w", err)
	}

	publisher := NewPublisher(cfg.ArtifactsRoot)
	blobSHA256, blobSize, err := publisher.PublishDir(scratchDir)
	if err != nil {
		return nil, fmt.Errorf("rebuilder: publish projection base blob: %w", err)
	}

	descriptor := Descriptor{
		ComputerID:            cfg.ComputerID,
		Sequence:              finalHead.Sequence,
		CanonicalHead:         finalHead.CanonicalEventHead,
		BlobSHA256:            blobSHA256,
		BlobSizeBytes:         blobSize,
		ReducerVersion:        finalHead.ReducerVersion,
		SchemaVersion:         int(computerevent.SchemaVersionV1),
		VocabularyVersion:     CurrentVocabularyVersion,
		VMLocalContentWitness: witness,
		CreatedAt:             time.Now().UTC(),
	}
	if err := descriptor.Validate(); err != nil {
		return nil, fmt.Errorf("rebuilder: validate descriptor: %w", err)
	}
	// Read back the packed artifact before the descriptor sidecar exists: the
	// blob must re-verify (digest, unpacked head, installed content witness) or
	// nothing is published. This is what prevents advertising a mismatched
	// witness.
	if err := VerifyPublishedBase(ctx, cfg.ArtifactsRoot, descriptor); err != nil {
		return nil, err
	}
	if _, err := publisher.PublishDescriptor(descriptor, blobSHA256); err != nil {
		return nil, fmt.Errorf("rebuilder: publish descriptor sidecar: %w", err)
	}
	blobPath := filepath.Join(cfg.ArtifactsRoot, "sha256", Namespace, blobSHA256)
	return &Result{
		Descriptor:     descriptor,
		BlobPath:       blobPath,
		Seed:           seed,
		TargetSequence: targetSequence,
	}, nil
}

// seedScratch selects and installs the verified seed for a fresh scratch dir.
// It returns a nil descriptor when SeedAuto found no compatible base that
// verifies (bounded genesis fallback); SeedRequired turns that into a refusal.
func (r *Rebuilder) seedScratch(ctx context.Context, source CASReplaySource, scratchDir string, targetSequence uint64) (*Descriptor, uint64, error) {
	cfg := r.cfg
	if cfg.SeedPolicy == SeedPinned {
		descriptor, blobPath, err := LoadPublishedBase(cfg.ArtifactsRoot, cfg.ComputerID, cfg.SeedBaseRef)
		if err != nil {
			return nil, 0, err
		}
		resolved, err := r.adoptSeed(ctx, source, scratchDir, SeedCandidate{Descriptor: descriptor, BlobPath: blobPath}, targetSequence)
		if err != nil {
			return nil, 0, err
		}
		return &descriptor, resolved, nil
	}
	candidates, err := listCompatibleBases(cfg.ArtifactsRoot, cfg.ComputerID, cfg.TargetHead, targetSequence)
	if err != nil {
		return nil, 0, err
	}
	var lastErr error
	for _, candidate := range candidates {
		resolved, err := r.adoptSeed(ctx, source, scratchDir, candidate, targetSequence)
		if err != nil {
			lastErr = err
			log.Printf("rebuilder: seed candidate %s rejected: %v", candidate.Descriptor.BlobSHA256, err)
			if err := resetScratchContent(scratchDir); err != nil {
				return nil, 0, fmt.Errorf("%w: reset rejected seed: %v", ErrBaseRefused, err)
			}
			continue
		}
		descriptor := candidate.Descriptor
		return &descriptor, resolved, nil
	}
	if cfg.SeedRequired {
		if lastErr != nil {
			return nil, 0, fmt.Errorf("%w: no compatible verified base for %s: %v", ErrBaseRefused, cfg.ComputerID, lastErr)
		}
		return nil, 0, fmt.Errorf("%w: no compatible verified base for %s", ErrBaseRefused, cfg.ComputerID)
	}
	if lastErr != nil {
		log.Printf("rebuilder: no seed candidate verified (%v); falling back to genesis replay", lastErr)
	}
	return nil, targetSequence, nil
}

// adoptSeed resolves the frozen target for a candidate, enforces the seed
// compatibility, ancestry, and bound gates from immutable tape, then installs
// and witness-verifies the candidate into the scratch directory.
func (r *Rebuilder) adoptSeed(ctx context.Context, source CASReplaySource, scratchDir string, candidate SeedCandidate, targetSequence uint64) (uint64, error) {
	cfg := r.cfg
	descriptor := candidate.Descriptor
	if targetSequence == 0 {
		if descriptor.CanonicalHead == cfg.TargetHead {
			targetSequence = descriptor.Sequence
		} else {
			resolved, err := ResolveTargetSequence(ctx, pagedTailAdapter{source}, cfg.ComputerID, cfg.TargetHead, descriptor.Sequence, descriptor.CanonicalHead)
			if err != nil {
				return 0, err
			}
			targetSequence = resolved
		}
	}
	if err := descriptor.VerifyForRecovery(cfg.ComputerID, cfg.TargetHead, targetSequence); err != nil {
		return 0, err
	}
	if cfg.MaxTailEvents > 0 && targetSequence-descriptor.Sequence > cfg.MaxTailEvents {
		return 0, fmt.Errorf("%w: seed tail %d exceeds MaxTailEvents %d", ErrBaseRefused, targetSequence-descriptor.Sequence, cfg.MaxTailEvents)
	}
	if descriptor.Sequence < targetSequence {
		// Ancestry gate from immutable tape: the first tail event must chain
		// exactly from the seed head before any bytes are adopted.
		page, err := source.EventsPage(ctx, cfg.ComputerID, descriptor.Sequence, 1)
		if err != nil {
			return 0, fmt.Errorf("%w: read tail start after seed watermark %d: %v", ErrBaseRefused, descriptor.Sequence, err)
		}
		if len(page) == 0 {
			return 0, fmt.Errorf("%w: tail is empty after seed watermark %d", ErrBaseRefused, descriptor.Sequence)
		}
		if err := descriptor.VerifyTailHead(page[0].Request.Event); err != nil {
			return 0, err
		}
	}
	if err := installSeedCandidate(ctx, candidate, scratchDir); err != nil {
		return 0, err
	}
	return targetSequence, nil
}

// validateResumedSeed refuses a durable scratch whose committed head cannot
// descend from the seed recorded at install time.
func validateResumedSeed(head *computerevent.Head, meta *seedMeta) error {
	if head.Sequence < meta.Sequence {
		return fmt.Errorf("%w: scratch head %d predates seed watermark %d", ErrBaseRefused, head.Sequence, meta.Sequence)
	}
	if head.Sequence == meta.Sequence && head.CanonicalEventHead != meta.CanonicalHead {
		return fmt.Errorf("%w: scratch head does not match the recorded seed", ErrBaseRefused)
	}
	return nil
}
