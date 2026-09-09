package projectionbase

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	choirstore "github.com/yusefmosiah/go-choir/internal/store"
)

// RebaseRetainedStore installs a verified base into a sibling directory and
// atomically replaces the retained marker and texture workspace. The live
// SQLite/Dolt files are never overwritten in place. On failure the original
// realization is left unchanged (or restored if a flip is interrupted after
// quarantine).
func RebaseRetainedStore(ctx context.Context, src BaseSource, storePath, computerID, targetHead string, targetSequence uint64) (Descriptor, error) {
	storePath = filepath.Clean(strings.TrimSpace(storePath))
	computerID = strings.TrimSpace(computerID)
	if storePath == "" || storePath == "." || computerID == "" {
		return Descriptor{}, fmt.Errorf("%w: rebase paths and computer are required", ErrBaseRefused)
	}
	parent := filepath.Dir(storePath)
	markerName := filepath.Base(storePath)
	originalWorkspace := choirstore.TextureWorkspacePath(storePath)
	if originalWorkspace == "" {
		return Descriptor{}, fmt.Errorf("%w: retained texture workspace is required", ErrBaseRefused)
	}

	stamp := time.Now().UTC().Format("20060102T150405.000000000Z")
	stagingRoot := filepath.Join(parent, "restore-staging-"+stamp)
	if err := os.Mkdir(stagingRoot, 0o755); err != nil {
		return Descriptor{}, fmt.Errorf("%w: create rebase staging: %v", ErrBaseRefused, err)
	}
	cleanupStaging := true
	defer func() {
		if cleanupStaging {
			_ = os.RemoveAll(stagingRoot)
		}
	}()

	descriptor, err := InstallVerifiedBase(ctx, src, stagingRoot, markerName, computerID, targetHead, targetSequence)
	if err != nil {
		return Descriptor{}, err
	}
	stagedMarker := filepath.Join(stagingRoot, markerName)
	stagedWorkspace := choirstore.TextureWorkspacePath(stagedMarker)

	quarantineDir := filepath.Join(parent, "restore-quarantine-"+stamp)
	if err := os.Mkdir(quarantineDir, 0o755); err != nil {
		return Descriptor{}, fmt.Errorf("%w: create rebase quarantine: %v", ErrBaseRefused, err)
	}
	quarantinedMarker := filepath.Join(quarantineDir, markerName)
	quarantinedWorkspace := filepath.Join(quarantineDir, filepath.Base(originalWorkspace))

	if err := os.Rename(storePath, quarantinedMarker); err != nil {
		_ = os.RemoveAll(quarantineDir)
		return Descriptor{}, fmt.Errorf("%w: quarantine original marker: %v", ErrBaseRefused, err)
	}
	if err := os.Rename(originalWorkspace, quarantinedWorkspace); err != nil {
		_ = os.Rename(quarantinedMarker, storePath)
		_ = os.RemoveAll(quarantineDir)
		return Descriptor{}, fmt.Errorf("%w: quarantine original workspace: %v", ErrBaseRefused, err)
	}
	if err := os.Rename(stagedMarker, storePath); err != nil {
		_ = os.Rename(quarantinedMarker, storePath)
		_ = os.Rename(quarantinedWorkspace, originalWorkspace)
		_ = os.RemoveAll(quarantineDir)
		return Descriptor{}, fmt.Errorf("%w: flip staged marker: %v", ErrBaseRefused, err)
	}
	if err := os.Rename(stagedWorkspace, originalWorkspace); err != nil {
		_ = os.Remove(storePath)
		_ = os.Rename(quarantinedMarker, storePath)
		_ = os.Rename(quarantinedWorkspace, originalWorkspace)
		_ = os.RemoveAll(quarantineDir)
		return Descriptor{}, fmt.Errorf("%w: flip staged workspace: %v", ErrBaseRefused, err)
	}
	cleanupStaging = false
	_ = os.RemoveAll(stagingRoot)
	return descriptor, nil
}

// PeekLocalSequence opens the retained store only long enough to read its
// projection head. The caller must not hold another live handle on the same
// files.
func PeekLocalSequence(ctx context.Context, storePath, computerID string) (uint64, error) {
	storePath = filepath.Clean(strings.TrimSpace(storePath))
	computerID = strings.TrimSpace(computerID)
	if storePath == "" || computerID == "" {
		return 0, fmt.Errorf("%w: peek requires store and computer", ErrBaseRefused)
	}
	store, err := choirstore.Open(storePath)
	if err != nil {
		return 0, fmt.Errorf("%w: retained store does not open: %v", ErrBaseRefused, err)
	}
	defer store.Close()
	head, err := store.Head(ctx, computerID)
	if err != nil {
		return 0, fmt.Errorf("%w: retained store head: %v", ErrBaseRefused, err)
	}
	if head == nil {
		return 0, nil
	}
	return head.Sequence, nil
}
