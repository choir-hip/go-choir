// Package imageblob is a computer's content-addressed image store. An image
// a desk or the owner hands to a model is stored once, by the sha256 of its
// bytes, and travels as an artifact:sha256 ref; transcripts, run events,
// logs and API responses carry the ref, and only the HTTP body to the
// gateway carries the bytes
// (docs/archive/problems/models-configured-text-only-2026-10-15.md, FM11-FM12).
package imageblob

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/provider"
)

// Store keeps images under <dir>/sha256/<hex>.
type Store struct {
	dir string
}

// New returns a store rooted at dir. Nothing is created until Put.
func New(dir string) *Store { return &Store{dir: strings.TrimSpace(dir)} }

// DirForStorePath is the image store beside the runtime store, named the
// way the store names its own siblings (state -> state.texture), so the
// gateway bridge and the API resolve the same blobs. On a guest that is
// /mnt/persistent/state.image-blobs.
func DirForStorePath(storePath string) string {
	storePath = strings.TrimSpace(storePath)
	if storePath == "" {
		return ""
	}
	trimmed := strings.TrimSuffix(storePath, filepath.Ext(storePath))
	if trimmed == "" {
		trimmed = storePath
	}
	return trimmed + ".image-blobs"
}

// Put stores an image and returns its artifact ref and media type. Bytes
// that are not a PNG, JPEG, GIF or WebP image, or are over the gateway's
// pre-resize limit, are refused.
func (s *Store) Put(data []byte) (string, string, error) {
	if s == nil || s.dir == "" {
		return "", "", fmt.Errorf("image store is not configured")
	}
	if len(data) > provider.MaxImageInputBytes {
		return "", "", fmt.Errorf("image is %d bytes; the limit is %d", len(data), provider.MaxImageInputBytes)
	}
	mimeType := provider.SniffImageType(data)
	if mimeType == "" {
		return "", "", fmt.Errorf("bytes are not a PNG, JPEG, GIF or WebP image")
	}
	sum := sha256.Sum256(data)
	digest := hex.EncodeToString(sum[:])
	ref, err := computerevent.ArtifactRefFromDigest(digest)
	if err != nil {
		return "", "", err
	}
	path := s.path(digest)
	if _, err := os.Stat(path); err == nil {
		return ref.String(), mimeType, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", "", fmt.Errorf("create image store: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".put-*")
	if err != nil {
		return "", "", fmt.Errorf("stage image: %w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return "", "", fmt.Errorf("write image: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", "", fmt.Errorf("write image: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return "", "", fmt.Errorf("store image: %w", err)
	}
	return ref.String(), mimeType, nil
}

// Get returns the bytes and media type of a stored image. The ref must be
// artifact:sha256:<64 lowercase hex>; the bytes are re-hashed against it.
func (s *Store) Get(ref string) ([]byte, string, error) {
	if s == nil || s.dir == "" {
		return nil, "", fmt.Errorf("image store is not configured")
	}
	parsed, err := computerevent.ParseArtifactRef(strings.TrimSpace(ref))
	if err != nil {
		return nil, "", fmt.Errorf("image ref: %w", err)
	}
	digest := parsed.Digest().String()
	data, err := os.ReadFile(s.path(digest))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, "", fmt.Errorf("image %s is not in this computer's image store", parsed.String())
		}
		return nil, "", fmt.Errorf("read image: %w", err)
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != digest {
		return nil, "", fmt.Errorf("image %s failed its content hash", parsed.String())
	}
	mimeType := provider.SniffImageType(data)
	if mimeType == "" {
		return nil, "", fmt.Errorf("image %s is not an image", parsed.String())
	}
	return data, mimeType, nil
}

func (s *Store) path(digest string) string {
	return filepath.Join(s.dir, "sha256", digest)
}
