// Package basemanifest is the parts form of a projection base (S4 of
// docs/problems/paper-never-checkpointed-2026-10-27.md): a small JSON
// manifest, stored where a tar blob would be and addressed by its own
// sha256, that lists every directory and file of the base. Each file's bytes
// are a content-addressed part under sha256/projection-base-parts/<digest>,
// stored once and shared by every base that holds the same bytes. Dolt
// table files are immutable, so a new base writes only its new table files,
// journal and manifest.
//
// It is a leaf package so the platform (artifact GC, part serving) and
// projectionbase (publish, install) read one definition.
package basemanifest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	// Format is the manifest format tag; it is also the blob's prefix, which
	// is how a scan tells a manifest blob from a tar blob.
	Format = "choir.projection-base.manifest/v1"
	// PartsNamespace is the artifacts namespace that holds parts.
	PartsNamespace = "projection-base-parts"
	// BaseNamespace is the namespace that holds base blobs and manifests.
	BaseNamespace = "projection-base"
	// MaxManifestBytes bounds a manifest read. A paper store holds a few
	// thousand files; this is far above that.
	MaxManifestBytes = 64 << 20

	TypeDir  = "dir"
	TypeFile = "file"
)

// Prefix is the first bytes of every manifest blob.
var Prefix = []byte(`{"format":"` + Format + `"`)

var digestRE = regexp.MustCompile(`^[0-9a-f]{64}$`)

// Entry is one directory or file of the base, by slash-separated relative
// path. A file's bytes are the part SHA256 of Size bytes.
type Entry struct {
	Path   string `json:"path"`
	Type   string `json:"type"`
	Mode   uint32 `json:"mode"`
	Size   int64  `json:"size,omitempty"`
	SHA256 string `json:"sha256,omitempty"`
}

// Manifest lists a base's entries in walk order: a directory precedes what
// it holds.
type Manifest struct {
	Format  string  `json:"format"`
	Entries []Entry `json:"entries"`
}

// Marshal encodes the manifest canonically (Format first, so the blob
// starts with Prefix).
func Marshal(m Manifest) ([]byte, error) {
	m.Format = Format
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(m)
}

// Parse decodes and validates a manifest.
func Parse(raw []byte) (Manifest, error) {
	if !bytes.HasPrefix(raw, Prefix) {
		return Manifest{}, fmt.Errorf("base manifest: not a %s manifest", Format)
	}
	var m Manifest
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return Manifest{}, fmt.Errorf("base manifest: decode: %w", err)
	}
	if dec.More() {
		return Manifest{}, fmt.Errorf("base manifest: trailing data")
	}
	if err := m.Validate(); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

// Validate refuses an unknown format, a path that is absolute, unclean or
// escapes the base, a duplicate path, an unknown entry type, and a file
// without a lowercase sha256 part digest.
func (m Manifest) Validate() error {
	if m.Format != Format {
		return fmt.Errorf("base manifest: unknown format %q", m.Format)
	}
	seen := make(map[string]struct{}, len(m.Entries))
	for _, e := range m.Entries {
		if err := validPath(e.Path); err != nil {
			return err
		}
		if _, dup := seen[e.Path]; dup {
			return fmt.Errorf("base manifest: duplicate path %q", e.Path)
		}
		seen[e.Path] = struct{}{}
		switch e.Type {
		case TypeDir:
			if e.Size != 0 || e.SHA256 != "" {
				return fmt.Errorf("base manifest: directory %q carries content", e.Path)
			}
		case TypeFile:
			if e.Size < 0 || !digestRE.MatchString(e.SHA256) {
				return fmt.Errorf("base manifest: file %q needs a size and a lowercase sha256", e.Path)
			}
		default:
			return fmt.Errorf("base manifest: entry %q has unknown type %q", e.Path, e.Type)
		}
		if e.Mode&^uint32(0o777) != 0 {
			return fmt.Errorf("base manifest: entry %q has mode bits beyond 0777", e.Path)
		}
	}
	return nil
}

func validPath(p string) error {
	if p == "" || strings.HasPrefix(p, "/") || strings.Contains(p, "\\") || strings.ContainsRune(p, 0) {
		return fmt.Errorf("base manifest: invalid path %q", p)
	}
	if path.Clean(p) != p || p == "." || p == ".." || strings.HasPrefix(p, "../") {
		return fmt.Errorf("base manifest: path %q is not clean and inside the base", p)
	}
	return nil
}

// PartDigests is the set of part digests the manifest names, each once.
func (m Manifest) PartDigests() []string {
	seen := map[string]struct{}{}
	var out []string
	for _, e := range m.Entries {
		if e.Type != TypeFile {
			continue
		}
		if _, ok := seen[e.SHA256]; ok {
			continue
		}
		seen[e.SHA256] = struct{}{}
		out = append(out, e.SHA256)
	}
	return out
}

// Contains reports whether the manifest names part digest.
func (m Manifest) Contains(digest string) bool {
	for _, e := range m.Entries {
		if e.Type == TypeFile && e.SHA256 == digest {
			return true
		}
	}
	return false
}

// PartPath is where part digest lives under artifactsRoot.
func PartPath(artifactsRoot, digest string) string {
	return filepath.Join(filepath.Clean(artifactsRoot), "sha256", PartsNamespace, digest)
}

// ReadFile reads a manifest blob at p, bounded by MaxManifestBytes. It
// returns ok=false and no error for a blob that is not a manifest (a tar).
func ReadFile(p string) (raw []byte, ok bool, err error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, false, err
	}
	defer f.Close()
	head := make([]byte, len(Prefix))
	n, err := io.ReadFull(f, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return nil, false, err
	}
	if !bytes.Equal(head[:n], Prefix) {
		return nil, false, nil
	}
	rest, err := io.ReadAll(io.LimitReader(f, MaxManifestBytes))
	if err != nil {
		return nil, true, err
	}
	raw = append(head[:n], rest...)
	if len(raw) > MaxManifestBytes {
		return nil, true, fmt.Errorf("base manifest: %s exceeds %d bytes", filepath.Base(p), MaxManifestBytes)
	}
	return raw, true, nil
}

// LivePartsInBaseDir is the GC live set for parts: every part named by any
// manifest blob present in sha256/projection-base, live base or not, so a
// part outlives the last manifest naming it by at least one sweep. A blob
// that starts like a manifest but does not read or parse fails the scan: a
// live set it could have widened is unprovable.
func LivePartsInBaseDir(artifactsRoot string) (map[string]struct{}, error) {
	dir := filepath.Join(filepath.Clean(artifactsRoot), "sha256", BaseNamespace)
	live := map[string]struct{}{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return live, nil
		}
		return nil, fmt.Errorf("base manifest: scan %s: %w", dir, err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if !entry.Type().IsRegular() || strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".json") {
			continue
		}
		raw, ok, err := ReadFile(filepath.Join(dir, name))
		if errors.Is(err, os.ErrNotExist) {
			continue // removed since the listing
		}
		if err != nil {
			return nil, fmt.Errorf("base manifest: read %s: %w", name, err)
		}
		if !ok {
			continue
		}
		m, err := Parse(raw)
		if err != nil {
			return nil, fmt.Errorf("base manifest: %s: %w", name, err)
		}
		for _, d := range m.PartDigests() {
			live[d] = struct{}{}
		}
	}
	return live, nil
}
