package platform

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/filecas"
)

func fileCASChunkRef(computerID, digest string) (string, error) {
	if !safeFileCASComponent(computerID) || !validFileCASDigest(digest) {
		return "", fmt.Errorf("file cas: invalid chunk reference")
	}
	return filepath.Join("sha256", "file-cas-chunks", computerID, digest), nil
}

func fileCASManifestRef(computerID, root string) (string, error) {
	if !safeFileCASComponent(computerID) || !validFileCASDigest(root) {
		return "", fmt.Errorf("file cas: invalid manifest reference")
	}
	return filepath.Join("sha256", "file-cas-roots", computerID, root+".json"), nil
}

// PinFileChunk stores an encrypted chunk once. Existing chunks are immutable.
func (s *Service) PinFileChunk(ctx context.Context, computerID, digest string, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	ref, err := fileCASChunkRef(computerID, digest)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	if digest != hex.EncodeToString(sum[:]) {
		return fmt.Errorf("file cas: chunk digest mismatch")
	}
	return s.writeFileCASImmutable(ref, data)
}

func (s *Service) GetFileChunk(ctx context.Context, computerID, digest string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ref, err := fileCASChunkRef(computerID, digest)
	if err != nil {
		return nil, err
	}
	return s.readBlob(ref)
}

// ClaimFileChunk reports whether the computer's namespace holds a non-empty
// chunk for digest and, when it does, refreshes its mtime so the artifact GC's
// grace window protects it until a root that names it commits. The namespace
// is the computer's own: another computer's chunk is never found. The chunk's
// bytes are not read (they were verified against their address when stored
// and are written by rename, never torn).
func (s *Service) ClaimFileChunk(ctx context.Context, computerID, digest string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	ref, err := fileCASChunkRef(computerID, digest)
	if err != nil {
		return false, err
	}
	path, err := s.artifactPath(ref)
	if err != nil {
		return false, err
	}
	if s.store != nil {
		s.store.casReuseMu.RLock()
		defer s.store.casReuseMu.RUnlock()
	}
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("file cas: stat chunk: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		return false, nil
	}
	now := time.Now()
	if err := os.Chtimes(path, now, now); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("file cas: claim chunk: %w", err)
	}
	return true, nil
}

func (s *Service) pinFileManifest(computerID, root string, data []byte) (string, error) {
	ref, err := fileCASManifestRef(computerID, root)
	if err != nil {
		return "", err
	}
	if err := s.writeFileCASImmutable(ref, data); err != nil {
		return "", err
	}
	return ref, nil
}

func (s *Service) getFileManifest(computerID, root string) ([]byte, error) {
	ref, err := fileCASManifestRef(computerID, root)
	if err != nil {
		return nil, err
	}
	return s.readBlob(ref)
}

// writeFileCASImmutable stores a content-addressed file CAS object. An
// identical existing object is reused and refreshed (mtime) so the artifact
// GC's grace window protects it until the writer's reference commits. A
// different existing object under the same name is torn or corrupt (the
// caller verified data's address), so it is replaced atomically. New bytes
// land through a temp file and rename: a crash never leaves a partial object
// under its final name.
func (s *Service) writeFileCASImmutable(storageRef string, data []byte) error {
	path, err := s.artifactPath(storageRef)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("file cas: create artifact directory: %w", err)
	}
	if s.store != nil {
		s.store.casReuseMu.RLock()
		defer s.store.casReuseMu.RUnlock()
	}
	existing, err := os.ReadFile(path)
	if err == nil && bytes.Equal(existing, data) {
		now := time.Now()
		if err := os.Chtimes(path, now, now); err != nil {
			return fmt.Errorf("file cas: refresh reused artifact: %w", err)
		}
		return nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("file cas: read existing artifact: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".tmp-filecas-*")
	if err != nil {
		return fmt.Errorf("file cas: create temp artifact: %w", err)
	}
	tmpName := tmp.Name()
	if _, err = tmp.Write(data); err == nil {
		err = tmp.Chmod(0o640)
	}
	if err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(tmpName, path)
	}
	if err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("file cas: write immutable artifact: %w", err)
	}
	return nil
}

func safeFileCASComponent(value string) bool {
	return value != "" && value == strings.TrimSpace(value) && value != "." && value != ".." && !strings.ContainsAny(value, `/\\`)
}

func validFileCASDigest(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

// missingFileChunk returns the first chunk the manifest names that this
// computer's namespace does not hold, or "" when it holds them all.
func (s *Service) missingFileChunk(ctx context.Context, computerID string, manifest *filecas.Manifest) (string, error) {
	for _, entry := range manifest.Files {
		for _, digest := range entry.Chunks {
			if err := ctx.Err(); err != nil {
				return "", err
			}
			ref, err := fileCASChunkRef(computerID, digest)
			if err != nil {
				return "", err
			}
			path, err := s.artifactPath(ref)
			if err != nil {
				return "", err
			}
			info, err := os.Stat(path)
			if errors.Is(err, os.ErrNotExist) || (err == nil && (!info.Mode().IsRegular() || info.Size() == 0)) {
				return digest, nil
			}
			if err != nil {
				return "", fmt.Errorf("file cas: stat chunk: %w", err)
			}
		}
	}
	return "", nil
}
