package platform

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

func (s *Service) writeFileCASImmutable(storageRef string, data []byte) error {
	path, err := s.artifactPath(storageRef)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("file cas: create artifact directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if errors.Is(err, os.ErrExist) {
		info, statErr := os.Stat(path)
		if statErr != nil {
			return fmt.Errorf("file cas: stat existing artifact: %w", statErr)
		}
		if info.Size() != int64(len(data)) {
			return fmt.Errorf("file cas: immutable artifact size mismatch")
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("file cas: create immutable artifact: %w", err)
	}
	if _, err = file.Write(data); err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(path)
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
