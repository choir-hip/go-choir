package capsule

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

// walkUpperdir walks the overlay upperdir and records the manifest of every
// file/directory present. This is the "current state" snapshot used for
// diff computation.
func walkUpperdir(ctx context.Context, upperDir string) ([]FileManifest, error) {
	var manifests []FileManifest

	err := filepath.Walk(upperDir, func(path string, info os.FileInfo, err error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err != nil {
			if os.IsNotExist(err) {
				return nil // file was deleted between walk steps
			}
			return err
		}

		relPath, err := filepath.Rel(upperDir, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}
		if relPath == "." {
			return nil // skip root
		}

		mt := fileTypeString(info)
		fm := FileManifest{
			Path:  relPath,
			Size:  info.Size(),
			Mtime: info.ModTime(),
			Mode:  uint32(info.Mode()),
			Type:  mt,
		}

		// Compute hash for regular files only.
		if mt == "file" && info.Mode().IsRegular() {
			hash, err := hashFile(ctx, path)
			if err != nil {
				return fmt.Errorf("failed to hash %s: %w", path, err)
			}
			fm.Hash = hash
		}

		manifests = append(manifests, fm)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk upperdir %s: %w", upperDir, err)
	}

	// Sort by path for deterministic ordering.
	sort.Slice(manifests, func(i, j int) bool {
		return manifests[i].Path < manifests[j].Path
	})

	return manifests, nil
}

// diffManifests compares the current manifest against the last committed
// manifest and returns the list of changes. This is the snapshot diff
// algorithm (v2) — no remount required, crash-safe.
func diffManifests(lastCommit, current []FileManifest) []FileChange {
	lastMap := make(map[string]FileManifest, len(lastCommit))
	for _, fm := range lastCommit {
		lastMap[fm.Path] = fm
	}

	currMap := make(map[string]FileManifest, len(current))
	for _, fm := range current {
		currMap[fm.Path] = fm
	}

	var changes []FileChange

	// Check for added and modified files.
	for _, fm := range current {
		last, existed := lastMap[fm.Path]
		switch {
		case !existed:
			changes = append(changes, FileChange{
				Path: fm.Path,
				Kind: ChangeAdded,
				Mode: os.FileMode(fm.Mode),
			})
		case !manifestsEqual(last, fm):
			changes = append(changes, FileChange{
				Path: fm.Path,
				Kind: ChangeModified,
				Mode: os.FileMode(fm.Mode),
			})
		}
	}

	// Check for deleted files.
	for _, fm := range lastCommit {
		if _, exists := currMap[fm.Path]; !exists {
			changes = append(changes, FileChange{
				Path: fm.Path,
				Kind: ChangeDeleted,
				Mode: os.FileMode(fm.Mode),
			})
		}
	}

	// Sort by path for deterministic ordering.
	sort.Slice(changes, func(i, j int) bool {
		return changes[i].Path < changes[j].Path
	})

	return changes
}

// manifestsEqual compares two FileManifest entries to determine if they
// represent the same file state. Uses hash for regular files, mode+size
// for others.
func manifestsEqual(a, b FileManifest) bool {
	if a.Path != b.Path {
		return false
	}
	if a.Mode != b.Mode {
		return false
	}
	if a.Type != b.Type {
		return false
	}
	if a.Type == "file" {
		return a.Hash == b.Hash && a.Size == b.Size
	}
	return a.Size == b.Size
}

// hashFile computes the SHA-256 hash of a file and returns it as hex.
func hashFile(ctx context.Context, path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, &contextReader{ctx: ctx, reader: f}); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *contextReader) Read(buffer []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := r.reader.Read(buffer)
	if err == nil {
		if contextErr := r.ctx.Err(); contextErr != nil {
			return n, contextErr
		}
	}
	return n, err
}

// fileTypeString returns a string representation of the file type.
func fileTypeString(info os.FileInfo) string {
	switch {
	case info.IsDir():
		return "dir"
	case info.Mode()&os.ModeSymlink != 0:
		return "symlink"
	case info.Mode()&os.ModeDevice != 0:
		return "device"
	case info.Mode().IsRegular():
		return "file"
	default:
		return "other"
	}
}
