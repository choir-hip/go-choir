package apppackage

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

const (
	BuildSchemaV1 = "choir.app-package-build/v1"

	// BuilderStaticV1 names the static/v1 recipe implementation. The host
	// builder (design §8) adds its revision and toolchain digest and signs.
	BuilderStaticV1 = "choir-host-builder/static-v1"

	maxBuildFiles    = 256
	maxBuildFileSize = 16 << 20
)

var (
	ErrInvalidBuild   = errors.New("app package: invalid build receipt")
	ErrDigestMismatch = errors.New("app package: bytes do not match their digest")
)

// Build is choir.app-package-build/v1: the host's receipt for one build of
// one package source. Its digest names the build everywhere.
type Build struct {
	Schema         string `json:"schema"`
	Package        string `json:"package"`
	Version        string `json:"version"`
	ManifestDigest string `json:"manifest_digest"`
	// Manifest is the source manifest the build was made from, embedded so
	// a build alone is enough to install: no second lookup can pair a
	// build with another manifest.
	Manifest     Manifest    `json:"manifest"`
	SourceDigest string      `json:"source_digest"`
	Recipe       string      `json:"recipe"`
	Builder      string      `json:"builder"`
	Files        []BuildFile `json:"files"`
}

type BuildFile struct {
	Path      string `json:"path"`
	SHA256    string `json:"sha256"`
	Size      int64  `json:"size"`
	MediaType string `json:"media_type"`
}

// Digest is the SHA-256 of the receipt's canonical JSON.
func (b Build) Digest() (string, error) { return canonicalDigest(b) }

// Validate checks the receipt's own shape; binding to a manifest is the
// reducer's install check (design §5.3).
func (b Build) Validate() error {
	bad := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", ErrInvalidBuild, fmt.Sprintf(format, args...))
	}
	if b.Schema != BuildSchemaV1 {
		return bad("schema %q is not %s", b.Schema, BuildSchemaV1)
	}
	if !namePattern.MatchString(b.Package) || !versionPattern.MatchString(b.Version) {
		return bad("package %q version %q", b.Package, b.Version)
	}
	if !IsDigest(b.ManifestDigest) || !IsDigest(b.SourceDigest) {
		return bad("manifest and source digests are required")
	}
	if err := b.Manifest.Validate(); err != nil {
		return bad("embedded manifest: %v", err)
	}
	if digest, err := b.Manifest.Digest(); err != nil || digest != b.ManifestDigest {
		return bad("embedded manifest does not digest to manifest_digest")
	}
	if !knownRecipes[b.Recipe] || strings.TrimSpace(b.Builder) == "" {
		return bad("recipe %q builder %q", b.Recipe, b.Builder)
	}
	if len(b.Files) == 0 || len(b.Files) > maxBuildFiles {
		return bad("a build has 1 to %d files", maxBuildFiles)
	}
	for i, f := range b.Files {
		if err := checkRelPath(f.Path); err != nil {
			return bad("files[%d]: %v", i, err)
		}
		if i > 0 && b.Files[i-1].Path >= f.Path {
			return bad("files must be sorted by path and unique")
		}
		if !IsDigest(f.SHA256) || f.Size < 0 || f.Size > maxBuildFileSize || strings.TrimSpace(f.MediaType) == "" {
			return bad("files[%d] %q record is incomplete", i, f.Path)
		}
	}
	return nil
}

// File returns the receipt's record for path.
func (b Build) File(p string) (BuildFile, bool) {
	i := sort.Search(len(b.Files), func(i int) bool { return b.Files[i].Path >= p })
	if i < len(b.Files) && b.Files[i].Path == p {
		return b.Files[i], true
	}
	return BuildFile{}, false
}

// Paths lists the receipt's file paths in order.
func (b Build) Paths() []string {
	out := make([]string, len(b.Files))
	for i, f := range b.Files {
		out[i] = f.Path
	}
	return out
}

func bytesDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// mediaTypeFor names the served type of a build output. Unknown extensions
// are served as opaque bytes, never sniffed.
func mediaTypeFor(p string) string {
	switch strings.ToLower(path.Ext(p)) {
	case ".js", ".mjs":
		return "text/javascript"
	case ".css":
		return "text/css"
	case ".json":
		return "application/json"
	case ".svg":
		return "image/svg+xml"
	case ".png":
		return "image/png"
	case ".wasm":
		return "application/wasm"
	case ".html":
		return "text/html"
	default:
		return "application/octet-stream"
	}
}

// SourceDigest is the choir-tree/v1 digest of a package source tree: the
// SHA-256 of the canonical JSON list of (path, sha256) for every regular
// file, sorted by path, excluding the manifest and dot-files. Symlinks and
// other non-regular files are refused.
func SourceDigest(dir string) (string, error) {
	type entry struct {
		Path   string `json:"path"`
		SHA256 string `json:"sha256"`
	}
	var entries []entry
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("source tree entry %s is not a regular file", rel)
		}
		rel = filepath.ToSlash(rel)
		if rel == ManifestFile {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		entries = append(entries, entry{Path: rel, SHA256: bytesDigest(data)})
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("app package: source digest: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	if entries == nil {
		entries = []entry{}
	}
	return canonicalDigest(entries)
}

// VerifySource checks that dir's tree digest is the manifest's source digest.
func VerifySource(dir string, m Manifest) error {
	got, err := SourceDigest(dir)
	if err != nil {
		return err
	}
	if got != m.Source.Digest {
		return fmt.Errorf("%w: tree is %s, manifest says %s", ErrSourceMismatch, got, m.Source.Digest)
	}
	return nil
}

// BuildStatic runs the static/v1 recipe over a package source tree: it
// reads and validates the manifest, verifies the source digest, and copies
// each declared output verbatim. The host builder calls this for static
// packages; nothing on a computer builds.
func BuildStatic(dir string) (Manifest, Build, map[string][]byte, error) {
	raw, err := os.ReadFile(filepath.Join(dir, ManifestFile))
	if err != nil {
		return Manifest{}, Build{}, nil, fmt.Errorf("app package: read manifest: %w", err)
	}
	m, err := ParseManifest(raw)
	if err != nil {
		return Manifest{}, Build{}, nil, err
	}
	if m.Build.Recipe != RecipeStaticV1 {
		return Manifest{}, Build{}, nil, fmt.Errorf("%w: recipe %s is not built by %s", ErrInvalidBuild, m.Build.Recipe, RecipeStaticV1)
	}
	if err := VerifySource(dir, m); err != nil {
		return Manifest{}, Build{}, nil, err
	}
	manifestDigest, err := m.Digest()
	if err != nil {
		return Manifest{}, Build{}, nil, err
	}
	build := Build{
		Schema:         BuildSchemaV1,
		Package:        m.Name,
		Version:        m.Version,
		ManifestDigest: manifestDigest,
		Manifest:       m,
		SourceDigest:   m.Source.Digest,
		Recipe:         m.Build.Recipe,
		Builder:        BuilderStaticV1,
	}
	files := make(map[string][]byte, len(m.Build.Outputs))
	for _, out := range m.Build.Outputs {
		data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(out)))
		if err != nil {
			return Manifest{}, Build{}, nil, fmt.Errorf("app package: static output %s: %w", out, err)
		}
		files[out] = data
		build.Files = append(build.Files, BuildFile{Path: out, SHA256: bytesDigest(data), Size: int64(len(data)), MediaType: mediaTypeFor(out)})
	}
	if err := build.Validate(); err != nil {
		return Manifest{}, Build{}, nil, err
	}
	return m, build, files, nil
}
