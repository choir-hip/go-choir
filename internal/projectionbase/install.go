package projectionbase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
)

// BaseSource is the platform read surface a verified base install consumes:
// one advertised watermark, its descriptor sidecar, the content-addressed
// blob stream, and the immutable tail head. Implementations stream; the
// installer verifies every byte against the descriptor before serving.
type BaseSource interface {
	Watermark(ctx context.Context, computerID string) (sequence uint64, baseRef string, err error)
	Descriptor(ctx context.Context, computerID, baseRef string) (Descriptor, error)
	DownloadBlob(ctx context.Context, computerID, baseRef string, dst io.Writer) error
	TailPage(ctx context.Context, computerID string, afterSequence uint64, pageSize int) ([]computerevent.DurableEvent, error)
}

// InstallVerifiedBase installs the required base for recovery into storeDir
// and proves it before returning: descriptor bindings plus W<=H ordering plus
// live compatibility (VerifyForRecovery), blob digest match, unpacked store
// head equal to the descriptor watermark, and tail-start ancestry from
// immutable tape (VerifyTailHead). Any failure refuses via ErrBaseRefused and
// leaves the staged bytes in place as evidence; it never falls back to an
// empty store or genesis replay.
//
// storeDir must be empty. A store whose head already equals the recovery
// target short-circuits as an idempotent reinstall (crash resume after
// install, before replay); any other non-empty state refuses.
func InstallVerifiedBase(ctx context.Context, src BaseSource, storeDir, markerName, computerID, targetHead string, targetSequence uint64) (Descriptor, error) {
	storeDir = filepath.Clean(strings.TrimSpace(storeDir))
	markerName = strings.TrimSpace(markerName)
	computerID = strings.TrimSpace(computerID)
	if storeDir == "" || storeDir == "." || markerName == "" || computerID == "" {
		return Descriptor{}, fmt.Errorf("%w: install paths and computer are required", ErrBaseRefused)
	}
	if !computerevent.IsSHA256(targetHead) || targetSequence == 0 {
		return Descriptor{}, fmt.Errorf("%w: recovery target is required", ErrBaseRefused)
	}
	if src == nil {
		return Descriptor{}, fmt.Errorf("%w: base source is required", ErrBaseRefused)
	}
	if err := ctx.Err(); err != nil {
		return Descriptor{}, err
	}

	sequence, baseRef, err := src.Watermark(ctx, computerID)
	if err != nil {
		return Descriptor{}, err
	}
	if sequence == 0 || strings.TrimSpace(baseRef) == "" {
		return Descriptor{}, fmt.Errorf("%w: no advertised base for %s", ErrBaseRefused, computerID)
	}
	descriptor, err := src.Descriptor(ctx, computerID, strings.TrimSpace(baseRef))
	if err != nil {
		return Descriptor{}, err
	}
	if descriptor.Sequence != sequence {
		return Descriptor{}, fmt.Errorf("%w: watermark sequence %d does not match descriptor %d", ErrBaseRefused, sequence, descriptor.Sequence)
	}
	if err := descriptor.VerifyForRecovery(computerID, targetHead, targetSequence); err != nil {
		return Descriptor{}, err
	}

	complete, err := reinstallComplete(storeDir, markerName, computerID, targetHead, targetSequence)
	if err != nil {
		return Descriptor{}, err
	}
	if complete {
		return descriptor, nil
	}

	tmpFile, err := os.CreateTemp(storeDir, ".base-download-*")
	if err != nil {
		return Descriptor{}, fmt.Errorf("%w: stage base download: %v", ErrBaseRefused, err)
	}
	tmpPath := tmpFile.Name()
	hasher := sha256.New()
	if err := src.DownloadBlob(ctx, computerID, descriptor.BlobSHA256, io.MultiWriter(tmpFile, hasher)); err != nil {
		_ = tmpFile.Close()
		return Descriptor{}, err
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return Descriptor{}, fmt.Errorf("%w: close base download: %v", ErrBaseRefused, err)
	}
	if digest := hex.EncodeToString(hasher.Sum(nil)); digest != descriptor.BlobSHA256 {
		return Descriptor{}, fmt.Errorf("%w: base blob digest mismatch", ErrBaseRefused)
	}

	stagingDir := filepath.Join(storeDir, ".base-staging")
	_ = os.RemoveAll(stagingDir)
	if err := os.MkdirAll(stagingDir, 0o755); err != nil {
		return Descriptor{}, fmt.Errorf("%w: create staging dir: %v", ErrBaseRefused, err)
	}
	if err := Unpack(tmpPath, stagingDir); err != nil {
		return Descriptor{}, fmt.Errorf("%w: unpack base: %v", ErrBaseRefused, err)
	}
	_ = os.Remove(tmpPath)
	entries, err := os.ReadDir(stagingDir)
	if err != nil {
		return Descriptor{}, fmt.Errorf("%w: read staging dir: %v", ErrBaseRefused, err)
	}
	for _, entry := range entries {
		if err := os.Rename(filepath.Join(stagingDir, entry.Name()), filepath.Join(storeDir, entry.Name())); err != nil {
			return Descriptor{}, fmt.Errorf("%w: install entry %s: %v", ErrBaseRefused, entry.Name(), err)
		}
	}
	_ = os.RemoveAll(stagingDir)

	if err := verifyInstalledHead(storeDir, markerName, descriptor); err != nil {
		return Descriptor{}, err
	}
	if dir, err := os.Open(storeDir); err == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}

	if descriptor.Sequence < targetSequence {
		page, err := src.TailPage(ctx, computerID, descriptor.Sequence, 1)
		if err != nil {
			return Descriptor{}, fmt.Errorf("%w: read tail start: %v", ErrBaseRefused, err)
		}
		if len(page) == 0 {
			return Descriptor{}, fmt.Errorf("%w: tail is empty after watermark %d", ErrBaseRefused, descriptor.Sequence)
		}
		if err := descriptor.VerifyTailHead(page[0].Request.Event); err != nil {
			return Descriptor{}, err
		}
	}
	return descriptor, nil
}

// reinstallComplete admits an empty storeDir for install, or short-circuits
// when the store head already equals the recovery target (crash resume after
// install, before replay). Every other non-empty state — partial install,
// foreign store, corrupt head — refuses so it can never be mistaken for a
// verified base.
func reinstallComplete(storeDir, markerName, computerID, targetHead string, targetSequence uint64) (bool, error) {
	entries, err := os.ReadDir(storeDir)
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(storeDir, 0o755); err != nil {
				return false, fmt.Errorf("%w: create store dir: %v", ErrBaseRefused, err)
			}
			return false, nil
		}
		return false, fmt.Errorf("%w: read store dir: %v", ErrBaseRefused, err)
	}
	if len(entries) == 0 {
		return false, nil
	}
	store, err := choirstore.Open(filepath.Join(storeDir, markerName))
	if err != nil {
		return false, fmt.Errorf("%w: non-empty store is unopenable (partial install suspected)", ErrBaseRefused)
	}
	head, herr := store.Head(context.Background(), computerID)
	_ = store.Close()
	if herr != nil || head == nil {
		return false, fmt.Errorf("%w: non-empty store has no readable head (partial install suspected)", ErrBaseRefused)
	}
	if head.Sequence == targetSequence && head.CanonicalEventHead == strings.ToLower(strings.TrimSpace(targetHead)) {
		return true, nil
	}
	return false, fmt.Errorf("%w: non-empty store at (%d) is not the recovery target (%d)", ErrBaseRefused, head.Sequence, targetSequence)
}

// verifyInstalledHead proves the unpacked bytes are the advertised watermark:
// the store opens and its head equals the descriptor computer, sequence, and
// canonical head.
func verifyInstalledHead(storeDir, markerName string, descriptor Descriptor) error {
	store, err := choirstore.Open(filepath.Join(storeDir, markerName))
	if err != nil {
		return fmt.Errorf("%w: installed base does not open: %v", ErrBaseRefused, err)
	}
	defer store.Close()
	head, err := store.Head(context.Background(), descriptor.ComputerID)
	if err != nil || head == nil {
		return fmt.Errorf("%w: installed base has no head: %v", ErrBaseRefused, err)
	}
	if head.Sequence != descriptor.Sequence || head.CanonicalEventHead != descriptor.CanonicalHead {
		return fmt.Errorf("%w: installed head (%d) is not the advertised watermark (%d)", ErrBaseRefused, head.Sequence, descriptor.Sequence)
	}
	return nil
}
