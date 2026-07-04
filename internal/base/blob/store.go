// Package blob implements an immutable, content-addressed blob store for the
// Choir Base sync substrate.
//
// Blobs are addressed by SHA-256: the BlobRef is "sha256:<hex>". The
// filesystem backend stores each blob as a single file under a two-level
// sharded directory tree (first two hex chars as the shard directory) so that
// no single directory holds millions of entries.
//
// The store is append-only in spirit: Put is idempotent (re-uploading the
// same bytes is a no-op that returns the existing ref), and Get always
// verifies that the stored bytes hash to the requested ref, detecting on-disk
// corruption.
package blob

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/base/model"
)

// Store is a content-addressed blob store backed by a filesystem directory.
// It is safe for concurrent use: Put writes atomically (temp file + rename),
// and Get reads the file under a shared lock-free path (filesystem rename is
// atomic on POSIX).
type Store struct {
	root string
}

// NewStore creates a blob store rooted at dir. The directory (and the shard
// subdirectories) are created lazily on first Put.
func NewStore(dir string) (*Store, error) {
	if dir == "" {
		return nil, errors.New("blob store: root directory is required")
	}
	// Create the root now so that a missing path is caught early.
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("blob store: create root %s: %w", dir, err)
	}
	return &Store{root: dir}, nil
}

// OpenStore opens an existing blob store root without creating directories.
// Use this for read-only observation paths that must not materialize missing
// state.
func OpenStore(dir string) (*Store, error) {
	if dir == "" {
		return nil, errors.New("blob store: root directory is required")
	}
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("blob store: stat root %s: %w", dir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("blob store: root %s is not a directory", dir)
	}
	return &Store{root: dir}, nil
}

// Put hashes data with SHA-256 and stores it under the resulting ref. If the
// blob already exists, it is a no-op and the existing ref is returned. The
// returned BlobRef is "sha256:<hex>".
func (s *Store) Put(data []byte) (model.BlobRef, error) {
	h := sha256.Sum256(data)
	hexDigest := hex.EncodeToString(h[:])
	ref := model.BlobRef("sha256:" + hexDigest)

	path := s.path(hexDigest)

	// Fast path: already present.
	if exists, err := s.fileExists(path); err != nil {
		return "", fmt.Errorf("blob store: stat %s: %w", ref, err)
	} else if exists {
		return ref, nil
	}

	// Ensure the shard directory exists.
	shardDir := filepath.Dir(path)
	if err := os.MkdirAll(shardDir, 0o755); err != nil {
		return "", fmt.Errorf("blob store: create shard dir %s: %w", shardDir, err)
	}

	// Write to a temp file in the same shard directory, then rename. This is
	// atomic on POSIX filesystems so concurrent Puts of the same content race
	// harmlessly (the loser's rename overwrites the winner's identical bytes).
	tmp, err := os.CreateTemp(shardDir, ".blob-tmp-*")
	if err != nil {
		return "", fmt.Errorf("blob store: create temp file: %w", err)
	}
	tmpName := tmp.Name()
	// Clean up the temp file if we fail before the rename.
	defer func() {
		if tmp != nil {
			_ = tmp.Close()
		}
		_ = os.Remove(tmpName) // best-effort; may not exist after rename
	}()

	if _, err := tmp.Write(data); err != nil {
		return "", fmt.Errorf("blob store: write temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return "", fmt.Errorf("blob store: sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("blob store: close temp file: %w", err)
	}
	tmp = nil

	if err := os.Rename(tmpName, path); err != nil {
		// If another goroutine raced ahead and created the file, that's fine.
		if !os.IsExist(err) {
			return "", fmt.Errorf("blob store: rename temp to %s: %w", path, err)
		}
	}
	return ref, nil
}

// Get retrieves the bytes for ref. It always re-hashes the stored bytes and
// verifies they match ref; a mismatch (on-disk corruption) yields
// ErrCorruptBlob.
func (s *Store) Get(ref model.BlobRef) ([]byte, error) {
	if !ref.Valid() {
		return nil, fmt.Errorf("blob store: invalid blob ref %q", ref)
	}
	hexDigest := strings.TrimPrefix(string(ref), "sha256:")
	path := s.path(hexDigest)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("blob store: read %s: %w", ref, err)
	}

	// Verify the hash to detect corruption.
	got := sha256.Sum256(data)
	gotHex := hex.EncodeToString(got[:])
	if gotHex != hexDigest {
		return nil, fmt.Errorf("%w: ref %s expected %s got %s", ErrCorruptBlob, ref, hexDigest, gotHex)
	}
	return data, nil
}

// Has reports whether a blob with the given ref exists in the store.
func (s *Store) Has(ref model.BlobRef) (bool, error) {
	if !ref.Valid() {
		return false, fmt.Errorf("blob store: invalid blob ref %q", ref)
	}
	hexDigest := strings.TrimPrefix(string(ref), "sha256:")
	path := s.path(hexDigest)
	return s.fileExists(path)
}

// Stat returns metadata about a stored blob (size and SHA-256 hex), or
// ErrNotFound if it does not exist.
func (s *Store) Stat(ref model.BlobRef) (model.Blob, error) {
	if !ref.Valid() {
		return model.Blob{}, fmt.Errorf("blob store: invalid blob ref %q", ref)
	}
	hexDigest := strings.TrimPrefix(string(ref), "sha256:")
	path := s.path(hexDigest)
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return model.Blob{}, ErrNotFound
		}
		return model.Blob{}, fmt.Errorf("blob store: stat %s: %w", ref, err)
	}
	return model.Blob{
		BlobRef:   ref,
		SizeBytes: info.Size(),
		SHA256:    hexDigest,
	}, nil
}

// path returns the on-disk path for a hex digest, sharded by the first two
// hex characters.
func (s *Store) path(hexDigest string) string {
	if len(hexDigest) < 2 {
		return filepath.Join(s.root, "_misc", hexDigest)
	}
	return filepath.Join(s.root, hexDigest[:2], hexDigest)
}

// fileExists reports whether path exists and is not a directory.
func (s *Store) fileExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return !info.IsDir(), nil
}

// Root returns the filesystem root directory of the store.
func (s *Store) Root() string { return s.root }

// CopyFrom copies all bytes from src into the store, returning the resulting
// BlobRef. It is a convenience wrapper around Put for streaming sources.
func (s *Store) CopyFrom(src io.Reader) (model.BlobRef, error) {
	data, err := io.ReadAll(src)
	if err != nil {
		return "", fmt.Errorf("blob store: read source: %w", err)
	}
	return s.Put(data)
}

// Sentinel errors for the blob store.
var (
	// ErrNotFound is returned by Get/Stat when no blob exists for the ref.
	ErrNotFound = errors.New("blob: not found")
	// ErrCorruptBlob is returned by Get when the stored bytes do not hash to
	// the requested ref (on-disk corruption).
	ErrCorruptBlob = errors.New("blob: corrupt (hash mismatch)")
)
