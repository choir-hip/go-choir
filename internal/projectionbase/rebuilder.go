package projectionbase

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
)

// Rebuilder executes an isolated offline full-tape rebuild outside the guest VM
// and publishes a content-addressed ProjectionBase blob.
type Rebuilder struct {
	cfg Config
}

// NewRebuilder returns an offline projection rebuilder.
func NewRebuilder(cfg Config) (*Rebuilder, error) {
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
	Descriptor Descriptor
	BlobPath   string
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

// Run executes the offline replay into an isolated scratch directory and publishes the base blob.
func (r *Rebuilder) Run(ctx context.Context, source CASReplaySource) (*Result, error) {
	if source == nil {
		return nil, fmt.Errorf("rebuilder: CAS replay source is required")
	}

	scratchDir := r.cfg.ScratchDir
	if scratchDir == "" {
		tempDir, err := os.MkdirTemp("", "choir-projection-base-rebuild-*")
		if err != nil {
			return nil, fmt.Errorf("rebuilder: create scratch directory: %w", err)
		}
		scratchDir = tempDir
		defer os.RemoveAll(tempDir)
	}

	scratchStorePath := filepath.Join(scratchDir, "runtime.db")
	scratchStore, err := choirstore.Open(scratchStorePath)
	if err != nil {
		return nil, fmt.Errorf("rebuilder: open scratch store: %w", err)
	}
	defer scratchStore.Close()

	cipher, err := computerevent.NewPrivateArtifactCipher(r.cfg.ComputerID, r.cfg.KeyMaterial)
	if err != nil {
		return nil, fmt.Errorf("rebuilder: initialize privacy cipher: %w", err)
	}

	appender, err := computerevent.NewComputerEventAppender(
		r.cfg.ComputerID,
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

	// Replay canonical chain up to target head.
	if err := appender.ReconstructThrough(ctx, source, r.cfg.TargetHead); err != nil {
		return nil, fmt.Errorf("rebuilder: replay through target head %s: %w", r.cfg.TargetHead, err)
	}

	finalHead, err := scratchStore.Head(ctx, r.cfg.ComputerID)
	if err != nil || finalHead == nil {
		return nil, fmt.Errorf("rebuilder: read scratch head after replay: %w", err)
	}
	if finalHead.CanonicalEventHead != r.cfg.TargetHead {
		return nil, fmt.Errorf("rebuilder: final head mismatch: got %s, want %s", finalHead.CanonicalEventHead, r.cfg.TargetHead)
	}

	// Extract state witness for canonical verification.
	version := computerversion.ComputerVersion{
		CodeRef:            computerversion.CodeRef("runtime:" + r.cfg.ComputerID),
		ArtifactProgramRef: computerversion.ArtifactProgramRef("event-chain:" + finalHead.CanonicalEventHead),
	}
	extractor := computerversion.DoltStateExtractor{
		WorkspacePath: scratchStore.TexturePath(),
		Database:      "texture",
		IgnoredContentColumns: map[string]map[string]struct{}{
			"computer_event_index": {
				"prepared_at":  {},
				"finalized_at": {},
			},
			"computer_event_projection_heads": {
				"updated_at": {},
			},
		},
	}
	extracted, err := extractor.Extract(ctx, computerversion.ExtractRequest{
		Name:    "projection-base-extraction",
		Version: version,
	})
	if err != nil {
		return nil, fmt.Errorf("rebuilder: extract scratch state witness: %w", err)
	}

	witness, err := selfdevprotocol.WitnessFromObservationSets(extracted, extracted, computerversion.EquivalenceChecker{}.CheckObservationSets(extracted, extracted))
	if err != nil {
		return nil, fmt.Errorf("rebuilder: compute content witness: %w", err)
	}

	// Flush scratch database and close before archiving directory.
	if err := scratchStore.Close(); err != nil {
		return nil, fmt.Errorf("rebuilder: close scratch store: %w", err)
	}

	// Memory guard check before tar publication.
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	if r.cfg.MemoryLimitRSS > 0 && int64(m.Alloc) > r.cfg.MemoryLimitRSS {
		return nil, fmt.Errorf("rebuilder: memory allocation %d bytes exceeds limit %d bytes", m.Alloc, r.cfg.MemoryLimitRSS)
	}

	// Publish scratch directory into atomic tar blob.
	publisher := NewPublisher(r.cfg.ArtifactsRoot)
	blobSHA256, blobSize, err := publisher.PublishDir(scratchDir)
	if err != nil {
		return nil, fmt.Errorf("rebuilder: publish projection base blob: %w", err)
	}

	descriptor := Descriptor{
		ComputerID:            r.cfg.ComputerID,
		Sequence:              finalHead.Sequence,
		CanonicalHead:         finalHead.CanonicalEventHead,
		BlobSHA256:            blobSHA256,
		BlobSizeBytes:         blobSize,
		ReducerVersion:        finalHead.ReducerVersion,
		SchemaVersion:         int(computerevent.SchemaVersionV1),
		VMLocalContentWitness: witness,
		CreatedAt:             time.Now().UTC(),
	}

	if err := descriptor.Validate(); err != nil {
		return nil, fmt.Errorf("rebuilder: validate descriptor: %w", err)
	}

	blobPath := filepath.Join(r.cfg.ArtifactsRoot, "sha256", Namespace, blobSHA256)
	return &Result{
		Descriptor: descriptor,
		BlobPath:   blobPath,
	}, nil
}
