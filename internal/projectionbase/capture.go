package projectionbase

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
)

// A projection base captured from a running computer's store, not rebuilt
// from its tape (S2 of docs/problems/paper-never-checkpointed-2026-10-27.md;
// owner, 2026-10-27). The paper's only base is 4.36M events behind its tape,
// and a host replay runs at about 50 events/s: a day of replay for one base.
// The host instead reflinks the running guest's data image (vmmanager
// CaptureDataImage), mounts the clone, and publishes the store on it here.
//
// What is checked: the store opens, its head is the tape's event at that
// sequence (so the base is at a head the tape has), and the published blob
// re-verifies like every base (digest, head, witness of its own bytes). What
// is not: that the tape derives the state. It does not need to: the base is
// an epoch root (OriginLiveCaptured), the definition of state at its
// sequence, and derivability is measured from it (owner, 2026-10-27).

// captureStagingDirName is where the store is moved on the capture before it
// is packed, so the blob holds the store and nothing else from the guest.
const captureStagingDirName = ".capture-base"

// CaptureConfig names one captured store and where its base is published.
type CaptureConfig struct {
	ComputerID string
	// StoreMarker is the guest store's marker on the mounted capture
	// (<mount>/state); its workspace is <mount>/state.texture. The capture
	// is disposable: the store is moved, compacted and packed in place.
	StoreMarker   string
	ArtifactsRoot string
	// TapeHeadAt returns the canonical head of the computer's event at
	// sequence, from the platform's tape.
	TapeHeadAt func(ctx context.Context, computerID string, sequence uint64) (string, error)
	// MinFreeBytes is the free space the artifacts filesystem must keep
	// after each large write (compaction, blob, read-back).
	MinFreeBytes uint64
	// CaptureFS is a path on the filesystem that holds the capture image
	// (empty: not checked). Compaction inside the mounted image diverges
	// the reflink there; it keeps MinFreeBytes too.
	CaptureFS string
	// AboveSequence is the computer's advertised watermark: a store whose
	// head is not above it can never become the root, and refuses before
	// compaction (review 365). Zero: no watermark.
	AboveSequence uint64
	// FreeBytes reads free space (nil: statfs).
	FreeBytes func(path string) (uint64, error)
	// PublishParts publishes the root in the parts form (S4).
	PublishParts bool
}

// PublishCapturedBase publishes the captured store as a projection base
// marked OriginLiveCaptured, at the store's own head. Any refusal leaves no
// blob and no descriptor; the capture itself is the caller's to remove.
func PublishCapturedBase(ctx context.Context, cfg CaptureConfig) (*Result, error) {
	cfg.ComputerID = strings.TrimSpace(cfg.ComputerID)
	if cfg.ComputerID == "" || strings.TrimSpace(cfg.StoreMarker) == "" || strings.TrimSpace(cfg.ArtifactsRoot) == "" || cfg.TapeHeadAt == nil {
		return nil, fmt.Errorf("%w: capture needs a computer, a store, an artifacts root and the tape", ErrBaseRefused)
	}
	if cfg.FreeBytes == nil {
		cfg.FreeBytes = statfsFree
	}
	marker := filepath.Clean(cfg.StoreMarker)
	workspace := choirstore.TextureWorkspacePath(marker)
	if err := checkCapturedStoreTree(marker, workspace); err != nil {
		return nil, err
	}

	// Move the store alone into a staging directory under the names a base
	// blob carries (VerifyPublishedBase reads runtime.db).
	staging := filepath.Join(filepath.Dir(marker), captureStagingDirName)
	if err := os.RemoveAll(staging); err != nil {
		return nil, fmt.Errorf("capture: clear staging: %w", err)
	}
	if err := os.Mkdir(staging, 0o755); err != nil {
		return nil, fmt.Errorf("capture: create staging: %w", err)
	}
	stagedMarker := filepath.Join(staging, "runtime.db")
	stagedWorkspace := choirstore.TextureWorkspacePath(stagedMarker)
	if err := os.Rename(marker, stagedMarker); err != nil {
		return nil, fmt.Errorf("capture: stage store marker: %w", err)
	}
	if err := os.Rename(workspace, stagedWorkspace); err != nil {
		return nil, fmt.Errorf("capture: stage store workspace: %w", err)
	}
	// Dolt's temp files go on the capture, not the host's /tmp; they are
	// removed before the store is packed.
	restoreTemp, err := UseStoreTempDir(stagedWorkspace, compactionDatabase)
	if err != nil {
		return nil, err
	}
	defer restoreTemp()

	head, err := settleCapturedStore(ctx, cfg, stagedMarker)
	if err != nil {
		return nil, err
	}
	// Compaction writes up to the store's size again: inside the mounted
	// image (bounded by its own free space), onto the capture's filesystem
	// (the reflink diverges as it does), and the blob and its read-back
	// follow on the artifacts filesystem (review 364).
	storeBytes := uint64(workspaceBytes(stagedWorkspace))
	if err := cfg.roomOn(staging, "the capture image", storeBytes, 0); err != nil {
		return nil, err
	}
	if cfg.CaptureFS != "" {
		if err := cfg.roomOn(cfg.CaptureFS, "the capture's filesystem", storeBytes, cfg.MinFreeBytes); err != nil {
			return nil, err
		}
	}
	if err := cfg.room(storeBytes); err != nil {
		return nil, err
	}
	compacted, err := CompactWorkspace(ctx, cfg.ComputerID, head.CanonicalEventHead, stagedWorkspace)
	if err != nil {
		return nil, fmt.Errorf("capture: compact: %w", err)
	}
	log.Printf("capture: compacted computer=%s commits %d->%d bytes %d->%d", cfg.ComputerID, compacted.CommitsBefore, compacted.CommitsAfter, compacted.NomsBytesBefore, compacted.NomsBytesAfter)
	witness, err := witnessForWorkspace(ctx, cfg.ComputerID, head.CanonicalEventHead, stagedWorkspace)
	if err != nil {
		return nil, err
	}
	restoreTemp()
	if err := cfg.room(uint64(workspaceBytes(staging))); err != nil {
		return nil, err
	}

	publisher := NewPublisher(cfg.ArtifactsRoot)
	pub, err := publishBaseDir(publisher, staging, publishFormFor(cfg.PublishParts))
	if err != nil {
		return nil, fmt.Errorf("capture: publish blob: %w", err)
	}
	blobSHA256, blobSize := pub.digest, pub.size
	if pub.form == PublishFormParts {
		log.Printf("capture: published parts computer=%s sequence=%d files=%d new_parts=%d new_bytes=%d reused_bytes=%d", cfg.ComputerID, head.Sequence, pub.stats.Files, pub.stats.NewParts, pub.stats.NewBytes, pub.stats.ReusedBytes)
	}
	published := false
	defer func() {
		if !published {
			_ = os.Remove(filepath.Join(cfg.ArtifactsRoot, "sha256", Namespace, blobSHA256))
		}
	}()
	descriptor := Descriptor{
		ComputerID:            cfg.ComputerID,
		Sequence:              head.Sequence,
		CanonicalHead:         head.CanonicalEventHead,
		BlobSHA256:            blobSHA256,
		BlobSizeBytes:         blobSize,
		ReducerVersion:        head.ReducerVersion,
		SchemaVersion:         int(computerevent.SchemaVersionV1),
		VocabularyVersion:     VocabularyVersionForForm(OriginLiveCaptured, pub.form),
		VMLocalContentWitness: witness,
		CreatedAt:             time.Now().UTC(),
		Origin:                OriginLiveCaptured,
		Format:                descriptorFormat(pub.form),
	}
	if err := descriptor.Validate(); err != nil {
		return nil, fmt.Errorf("capture: validate descriptor: %w", err)
	}
	if err := cfg.room(uint64(blobSize)); err != nil {
		return nil, err
	}
	if err := VerifyPublishedBase(ctx, cfg.ArtifactsRoot, descriptor); err != nil {
		return nil, err
	}
	if _, err := publisher.PublishDescriptor(descriptor, blobSHA256); err != nil {
		return nil, fmt.Errorf("capture: publish descriptor: %w", err)
	}
	published = true
	return &Result{
		Descriptor:     descriptor,
		BlobPath:       filepath.Join(cfg.ArtifactsRoot, "sha256", Namespace, blobSHA256),
		TargetSequence: head.Sequence,
	}, nil
}

// settleCapturedStore opens the captured store, proves its head is on the
// tape, and brings it to the state at that head: an append the guest had
// prepared but not yet finalized is not part of it, and the serving
// vocabulary fence is the one every base carries.
func settleCapturedStore(ctx context.Context, cfg CaptureConfig, marker string) (*computerevent.Head, error) {
	store, err := choirstore.Open(marker)
	if err != nil {
		return nil, fmt.Errorf("%w: captured store does not open: %v", ErrBaseRefused, err)
	}
	defer store.Close()
	head, err := store.Head(ctx, cfg.ComputerID)
	if err != nil {
		return nil, fmt.Errorf("%w: captured store head: %v", ErrBaseRefused, err)
	}
	if head == nil || head.Sequence == 0 || !computerevent.IsSHA256(head.CanonicalEventHead) {
		return nil, fmt.Errorf("%w: captured store has no head for %s", ErrBaseRefused, cfg.ComputerID)
	}
	if head.Sequence <= cfg.AboveSequence {
		return nil, fmt.Errorf("%w: captured store head %d is not above the advertised watermark %d; it cannot become the root", ErrBaseRefused, head.Sequence, cfg.AboveSequence)
	}
	onTape, err := cfg.TapeHeadAt(ctx, cfg.ComputerID, head.Sequence)
	if err != nil {
		return nil, fmt.Errorf("%w: tape event at %d: %v", ErrBaseRefused, head.Sequence, err)
	}
	if onTape != head.CanonicalEventHead {
		return nil, fmt.Errorf("%w: captured head at %d is %s, the tape's is %s", ErrBaseRefused, head.Sequence, head.CanonicalEventHead, onTape)
	}
	prepared, err := store.Prepared(ctx, cfg.ComputerID)
	if err != nil {
		return nil, fmt.Errorf("capture: read prepared events: %w", err)
	}
	for _, p := range prepared {
		if err := store.DiscardPrepared(ctx, cfg.ComputerID, p.EventDigest); err != nil {
			return nil, fmt.Errorf("capture: discard prepared event %d: %w", p.Event.Sequence, err)
		}
	}
	if len(prepared) > 0 {
		log.Printf("capture: computer=%s discarded %d prepared, unfinalized event(s) above head %d", cfg.ComputerID, len(prepared), head.Sequence)
	}
	if _, err := store.MigrateAndFenceServingVocabulary(ctx, false, nil); err != nil {
		return nil, fmt.Errorf("capture: vocabulary fence refused: %w", err)
	}
	if err := store.Close(); err != nil {
		return nil, fmt.Errorf("capture: close captured store: %w", err)
	}
	return head, nil
}

// checkCapturedStoreTree refuses a store tree the host should not open. The
// tree is the guest's: before anything opens, stages or packs it, the
// marker must be a regular file and the workspace a real directory (neither
// a symlink, nor under one), and nothing under the workspace may be a
// symlink, device, fifo or socket (review 364). The mount itself is
// nosuid,nodev,noexec (checkpointd capture-base).
func checkCapturedStoreTree(marker, workspace string) error {
	parent := filepath.Dir(marker)
	if info, err := os.Lstat(parent); err != nil || !info.IsDir() {
		return fmt.Errorf("%w: captured store directory %s is not a directory: %v", ErrBaseRefused, parent, err)
	}
	if info, err := os.Lstat(marker); err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("%w: captured store marker %s is not a regular file: %v", ErrBaseRefused, marker, err)
	}
	if info, err := os.Lstat(workspace); err != nil || !info.IsDir() {
		return fmt.Errorf("%w: captured store workspace %s is not a directory: %v", ErrBaseRefused, workspace, err)
	}
	return filepath.WalkDir(workspace, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("%w: walk captured store: %v", ErrBaseRefused, err)
		}
		if t := entry.Type(); !entry.IsDir() && !t.IsRegular() {
			return fmt.Errorf("%w: captured store holds %s, which is not a file or directory (%s)", ErrBaseRefused, path, t)
		}
		return nil
	})
}

// room refuses a write of need bytes that would leave the artifacts
// filesystem below the floor.
func (cfg CaptureConfig) room(need uint64) error {
	return cfg.roomOn(cfg.ArtifactsRoot, "the artifacts filesystem", need, cfg.MinFreeBytes)
}

// roomOn refuses a write of need bytes that would leave path's filesystem
// below floor.
func (cfg CaptureConfig) roomOn(path, what string, need, floor uint64) error {
	free, err := cfg.FreeBytes(path)
	if err != nil {
		return fmt.Errorf("capture: read free space of %s: %w", what, err)
	}
	if free < need+floor {
		return fmt.Errorf("%w: capture needs %d bytes on %s with a %d-byte floor, %d free", ErrBaseRefused, need, what, floor, free)
	}
	return nil
}

func statfsFree(path string) (uint64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, fmt.Errorf("statfs %s: %w", path, err)
	}
	return uint64(st.Bavail) * uint64(st.Bsize), nil
}
