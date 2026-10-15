package apppackage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var (
	ErrBuildNotFound     = errors.New("app package: build not found")
	ErrBuildFileNotFound = errors.New("app package: file is not part of the build")
)

const buildReceiptFile = "build.json"

// Source resolves a host build by digest. S7-0a has the release-shipped
// catalog (a BuildStore root); S7-0c adds the host content-addressed cache.
type Source interface {
	Lookup(buildDigest string) (Build, map[string][]byte, error)
}

// BuildStore is a content-addressed directory of package builds:
// <root>/builds/<build hex>/build.json and <root>/builds/<build hex>/files/<path>.
// The guest keeps installed builds in one; the release-shipped catalog is
// another, read-only.
type BuildStore struct {
	Root string
}

func digestHex(buildDigest string) (string, error) {
	if !IsDigest(buildDigest) {
		return "", fmt.Errorf("%w: build digest %q", ErrInvalidBuild, buildDigest)
	}
	return strings.TrimPrefix(buildDigest, "sha256:"), nil
}

func (s BuildStore) dir(buildDigest string) (string, error) {
	if strings.TrimSpace(s.Root) == "" {
		return "", errors.New("app package: build store has no root")
	}
	hex, err := digestHex(buildDigest)
	if err != nil {
		return "", err
	}
	return filepath.Join(s.Root, "builds", hex), nil
}

// Has reports whether the build's receipt is present.
func (s BuildStore) Has(buildDigest string) bool {
	dir, err := s.dir(buildDigest)
	if err != nil {
		return false
	}
	info, err := os.Stat(filepath.Join(dir, buildReceiptFile))
	return err == nil && info.Mode().IsRegular()
}

// Put stores a build after checking every file against its receipt. It
// writes into a temporary directory and renames it into place, so a reader
// never sees half a build. Storing a build already present is a no-op.
func (s BuildStore) Put(build Build, files map[string][]byte) (string, error) {
	if err := build.Validate(); err != nil {
		return "", err
	}
	digest, err := build.Digest()
	if err != nil {
		return "", err
	}
	if len(files) != len(build.Files) {
		return "", fmt.Errorf("%w: %d files for a receipt of %d", ErrInvalidBuild, len(files), len(build.Files))
	}
	for _, f := range build.Files {
		data, ok := files[f.Path]
		if !ok {
			return "", fmt.Errorf("%w: missing %s", ErrInvalidBuild, f.Path)
		}
		if int64(len(data)) != f.Size || bytesDigest(data) != f.SHA256 {
			return "", fmt.Errorf("%w: %s", ErrDigestMismatch, f.Path)
		}
	}
	if s.Has(digest) {
		return digest, nil
	}
	final, err := s.dir(digest)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(final), 0o755); err != nil {
		return "", fmt.Errorf("app package: build store: %w", err)
	}
	tmp, err := os.MkdirTemp(filepath.Dir(final), ".put-")
	if err != nil {
		return "", fmt.Errorf("app package: build store: %w", err)
	}
	defer os.RemoveAll(tmp)
	for _, f := range build.Files {
		target := filepath.Join(tmp, "files", filepath.FromSlash(f.Path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return "", fmt.Errorf("app package: build store: %w", err)
		}
		if err := os.WriteFile(target, files[f.Path], 0o644); err != nil {
			return "", fmt.Errorf("app package: build store: %w", err)
		}
	}
	receipt, err := json.MarshalIndent(build, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(tmp, buildReceiptFile), receipt, 0o644); err != nil {
		return "", fmt.Errorf("app package: build store: %w", err)
	}
	if err := os.Rename(tmp, final); err != nil {
		if s.Has(digest) {
			return digest, nil // a concurrent Put of the same build won
		}
		return "", fmt.Errorf("app package: build store: %w", err)
	}
	return digest, nil
}

// Get reads and checks a build's receipt.
func (s BuildStore) Get(buildDigest string) (Build, error) {
	dir, err := s.dir(buildDigest)
	if err != nil {
		return Build{}, err
	}
	raw, err := os.ReadFile(filepath.Join(dir, buildReceiptFile))
	if errors.Is(err, os.ErrNotExist) {
		return Build{}, fmt.Errorf("%w: %s", ErrBuildNotFound, buildDigest)
	}
	if err != nil {
		return Build{}, fmt.Errorf("app package: build store: %w", err)
	}
	var build Build
	if err := strictDecode(raw, &build); err != nil {
		return Build{}, fmt.Errorf("%w: %v", ErrInvalidBuild, err)
	}
	if err := build.Validate(); err != nil {
		return Build{}, err
	}
	if got, err := build.Digest(); err != nil || got != buildDigest {
		return Build{}, fmt.Errorf("%w: receipt in %s digests to %s", ErrDigestMismatch, buildDigest, got)
	}
	return build, nil
}

// Open returns one file of a build, re-verified against the receipt on
// every read: bytes changed on disk are refused, not served.
func (s BuildStore) Open(buildDigest, path string) ([]byte, BuildFile, error) {
	if err := checkRelPath(path); err != nil {
		return nil, BuildFile{}, fmt.Errorf("%w: %v", ErrBuildFileNotFound, err)
	}
	build, err := s.Get(buildDigest)
	if err != nil {
		return nil, BuildFile{}, err
	}
	record, ok := build.File(path)
	if !ok {
		return nil, BuildFile{}, fmt.Errorf("%w: %s", ErrBuildFileNotFound, path)
	}
	dir, _ := s.dir(buildDigest)
	data, err := os.ReadFile(filepath.Join(dir, "files", filepath.FromSlash(path)))
	if err != nil {
		return nil, BuildFile{}, fmt.Errorf("%w: %s: %v", ErrDigestMismatch, path, err)
	}
	if int64(len(data)) != record.Size || bytesDigest(data) != record.SHA256 {
		return nil, BuildFile{}, fmt.Errorf("%w: %s", ErrDigestMismatch, path)
	}
	return data, record, nil
}

// Lookup returns a verified build and all its files; BuildStore is a Source.
func (s BuildStore) Lookup(buildDigest string) (Build, map[string][]byte, error) {
	build, err := s.Get(buildDigest)
	if err != nil {
		return Build{}, nil, err
	}
	files := make(map[string][]byte, len(build.Files))
	for _, f := range build.Files {
		data, _, err := s.Open(buildDigest, f.Path)
		if err != nil {
			return Build{}, nil, err
		}
		files[f.Path] = data
	}
	return build, files, nil
}

// List returns every valid build receipt in the store, by package then
// version. Entries that do not verify are skipped.
func (s BuildStore) List() ([]Build, error) {
	entries, err := os.ReadDir(filepath.Join(s.Root, "builds"))
	if errors.Is(err, os.ErrNotExist) {
		return []Build{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("app package: build store: %w", err)
	}
	out := []Build{}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		build, err := s.Get("sha256:" + entry.Name())
		if err != nil {
			continue
		}
		out = append(out, build)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Package != out[j].Package {
			return out[i].Package < out[j].Package
		}
		return out[i].Version < out[j].Version
	})
	return out, nil
}
