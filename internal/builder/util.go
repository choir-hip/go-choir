package builder

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// digestSHA256Hex returns the lowercase hex SHA-256 of b.
func digestSHA256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// writeFileAtomic writes content to path with the given mode via a temp file
// + atomic rename, so a partially written artifact is never observed.
func writeFileAtomic(path string, content []byte, mode os.FileMode) error {
	if path == "" {
		return fmt.Errorf("builder: output path required")
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".builder-tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
