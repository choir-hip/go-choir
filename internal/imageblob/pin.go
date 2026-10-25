package imageblob

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Pins (docs/design/texture-evidence-embeds-te-2026-10-25.md section 4;
// visual evidence V35). A Texture revision that embeds an asset pins it, so a
// later reclamation cannot leave the document pointing at nothing. The store
// has no sweep yet; a sweep must keep every digest that has a pin. A pin
// names its holder (texture-revision:<doc>:<revision>) and does not need
// the bytes: a capture whose store write failed is still cited.
//
// Layout: <dir>/pins/sha256/<digest>/<sha256(holder)>, holding the holder.

// Pin records that holder holds digest. Pinning again is a no-op.
func (s *Store) Pin(digest, holder string) error {
	if s == nil || s.dir == "" {
		return fmt.Errorf("image store is not configured")
	}
	if !validDigest(digest) {
		return ErrBadDigest
	}
	holder = strings.TrimSpace(holder)
	if holder == "" || strings.ContainsAny(holder, "\r\n\x00") {
		return fmt.Errorf("a pin needs a one-line holder")
	}
	dir := s.pinDir(digest)
	path := filepath.Join(dir, pinName(holder))
	if existing, err := os.ReadFile(path); err == nil && string(existing) == holder {
		return nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create pin directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".pin-*")
	if err != nil {
		return fmt.Errorf("stage pin: %w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.WriteString(holder); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write pin: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write pin: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write pin: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("record pin: %w", err)
	}
	return nil
}

// Pins lists the holders of digest, sorted.
func (s *Store) Pins(digest string) ([]string, error) {
	if s == nil || s.dir == "" {
		return nil, fmt.Errorf("image store is not configured")
	}
	if !validDigest(digest) {
		return nil, ErrBadDigest
	}
	entries, err := os.ReadDir(s.pinDir(digest))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read pins: %w", err)
	}
	var out []string
	for _, e := range entries {
		if !e.Type().IsRegular() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.pinDir(digest), e.Name()))
		if err != nil {
			return nil, fmt.Errorf("read pin: %w", err)
		}
		out = append(out, string(data))
	}
	sort.Strings(out)
	return out, nil
}

func (s *Store) pinDir(digest string) string {
	return filepath.Join(s.dir, "pins", "sha256", digest)
}

func pinName(holder string) string {
	sum := sha256.Sum256([]byte(holder))
	return hex.EncodeToString(sum[:])
}
