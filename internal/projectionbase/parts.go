package projectionbase

// The parts form of a published base (S4 of
// docs/problems/paper-never-checkpointed-2026-10-27.md). The manifest takes
// the tar blob's place in sha256/projection-base/<digest>, and every file's
// bytes are a content-addressed part stored once. Descriptor.Format names
// the form; the tar form stays the default and stays readable.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/basemanifest"
	"github.com/yusefmosiah/go-choir/internal/storeschema"
)

// PublishForm selects how a base is published.
type PublishForm string

const (
	// PublishFormTar is one tar blob per base (the default).
	PublishFormTar PublishForm = "tar"
	// PublishFormParts is a manifest plus content-addressed parts.
	PublishFormParts PublishForm = "parts"
)

// PartsSelectorEnv names the computers whose bases are published in the
// parts form (owner, 2026-10-28: progressive rollout, "on for at least
// one", never shipped off). Tokens as vmctl's *_ONLY lists
// (internal/vmmanager/computer_select.go): a computer id, class:<warmness
// class>, or "*". Every other computer publishes the tar form. Readers
// accept both forms everywhere, whatever the list says.
const PartsSelectorEnv = "CHECKPOINTD_BASE_PARTS_ONLY"

// ParseComputerSelector splits a comma, space or newline separated list.
func ParseComputerSelector(raw string) []string {
	var out []string
	for _, tok := range strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\n' || r == '\t'
	}) {
		if tok = strings.TrimSpace(tok); tok != "" {
			out = append(out, tok)
		}
	}
	return out
}

// SelectsComputer reports whether the list names the computer. class is
// the computer's warmness class when the caller knows it; an empty class
// never matches a class: token.
func SelectsComputer(only []string, computerID, class string) bool {
	computerID, class = strings.TrimSpace(computerID), strings.TrimSpace(class)
	for _, tok := range only {
		tok = strings.TrimSpace(tok)
		switch {
		case tok == "":
		case tok == "*":
			return true
		case strings.HasPrefix(tok, "class:"):
			if want := strings.TrimPrefix(tok, "class:"); want != "" && want == class {
				return true
			}
		case computerID != "" && tok == computerID:
			return true
		}
	}
	return false
}

// publishFormFor is the form a publish uses.
func publishFormFor(parts bool) PublishForm {
	if parts {
		return PublishFormParts
	}
	return PublishFormTar
}

// publishedBase is one published base's blob identity and form.
type publishedBase struct {
	digest string
	size   int64
	form   PublishForm
	stats  PartsStats
}

// publishBaseDir publishes dir in form.
func publishBaseDir(p *Publisher, dir string, form PublishForm) (publishedBase, error) {
	if form != PublishFormParts {
		digest, size, err := p.PublishDir(dir)
		return publishedBase{digest: digest, size: size, form: PublishFormTar}, err
	}
	digest, size, stats, err := p.PublishDirParts(dir)
	return publishedBase{digest: digest, size: size, form: PublishFormParts, stats: stats}, err
}

const (
	// FormatParts is Descriptor.Format for a parts-form base. An empty
	// Format is the tar form.
	FormatParts = "parts/v1"
	// partsVocabularySuffix is appended to a parts-form base's vocabulary
	// version, after any live_captured suffix. A build from before S4
	// ignores the format field and refuses a vocabulary outside {v1, v2},
	// so it fails closed on the descriptor instead of unpacking a manifest
	// as a tar.
	partsVocabularySuffix = "+parts"

	// PartsNamespace and ManifestFormat re-export the leaf package's names.
	PartsNamespace = basemanifest.PartsNamespace
	ManifestFormat = basemanifest.Format
)

// Manifest is a parts-form base's manifest.
type Manifest = basemanifest.Manifest

// ParseManifest decodes and validates a manifest blob.
func ParseManifest(raw []byte) (Manifest, error) { return basemanifest.Parse(raw) }

// PartPath is where a part lives under artifactsRoot.
func PartPath(artifactsRoot, digest string) string {
	return basemanifest.PartPath(artifactsRoot, digest)
}

// ManifestPartsInBaseDir is every part an on-disk manifest names.
func ManifestPartsInBaseDir(artifactsRoot string) (map[string]struct{}, error) {
	return basemanifest.LivePartsInBaseDir(artifactsRoot)
}

// VocabularyVersionForForm is the vocabulary version a base of this origin
// and publish form is published under.
func VocabularyVersionForForm(origin string, form PublishForm) string {
	v := VocabularyVersionFor(origin)
	if form == PublishFormParts {
		v += partsVocabularySuffix
	}
	return v
}

// descriptorFormat is Descriptor.Format for a publish form.
func descriptorFormat(form PublishForm) string {
	if form == PublishFormParts {
		return FormatParts
	}
	return ""
}

// PartsStats counts what one parts publish wrote.
type PartsStats struct {
	Files       int   `json:"files"`
	NewParts    int   `json:"new_parts"`
	NewBytes    int64 `json:"new_bytes"`
	ReusedBytes int64 `json:"reused_bytes"`
}

// PublishDirParts publishes srcDir in the parts form: each regular file is
// hashed, stored under its digest only when no such part exists (a reused
// part's mtime is refreshed, so GC grace covers it until a manifest names
// it), and the manifest is written atomically to the base namespace under
// its own digest. Like PublishDir, every store workspace gets the stream_seq
// lease untrust marker beside its receipt.
func (p *Publisher) PublishDirParts(srcDir string) (string, int64, PartsStats, error) {
	var stats PartsStats
	srcDir = filepath.Clean(srcDir)
	info, err := os.Stat(srcDir)
	if err != nil || !info.IsDir() {
		return "", 0, stats, fmt.Errorf("publisher: invalid source directory %q: %w", srcDir, err)
	}
	partsDir := filepath.Join(p.artifactsRoot, "sha256", PartsNamespace)
	if err := os.MkdirAll(partsDir, 0o755); err != nil {
		return "", 0, stats, fmt.Errorf("publisher: create parts directory: %w", err)
	}
	var entries []basemanifest.Entry
	addBytes := func(rel string, mode os.FileMode, body []byte) error {
		sum := sha256.Sum256(body)
		digest := hex.EncodeToString(sum[:])
		fresh, err := p.storePart(partsDir, digest, int64(len(body)), func(w io.Writer) error {
			_, err := w.Write(body)
			return err
		})
		if err != nil {
			return err
		}
		stats.Files++
		if fresh {
			stats.NewParts++
			stats.NewBytes += int64(len(body))
		} else {
			stats.ReusedBytes += int64(len(body))
		}
		entries = append(entries, basemanifest.Entry{Path: rel, Type: basemanifest.TypeFile, Mode: uint32(mode.Perm()), Size: int64(len(body)), SHA256: digest})
		return nil
	}
	err = filepath.Walk(srcDir, func(path string, fi os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		switch {
		case fi.IsDir():
			entries = append(entries, basemanifest.Entry{Path: rel, Type: basemanifest.TypeDir, Mode: uint32(fi.Mode().Perm())})
		case fi.Mode().IsRegular():
			digest, size, err := hashFile(path)
			if err != nil {
				return err
			}
			fresh, err := p.storePart(partsDir, digest, size, func(w io.Writer) error {
				f, err := os.Open(path)
				if err != nil {
					return err
				}
				defer f.Close()
				_, err = io.Copy(w, f)
				return err
			})
			if err != nil {
				return fmt.Errorf("part %q: %w", rel, err)
			}
			stats.Files++
			if fresh {
				stats.NewParts++
				stats.NewBytes += size
			} else {
				stats.ReusedBytes += size
			}
			entries = append(entries, basemanifest.Entry{Path: rel, Type: basemanifest.TypeFile, Mode: uint32(fi.Mode().Perm()), Size: size, SHA256: digest})
			// The lease untrust marker, as PublishDir adds it.
			if fi.Name() == storeschema.File {
				if _, statErr := os.Lstat(filepath.Join(filepath.Dir(path), storeschema.LeaseUntrustedFile)); statErr != nil {
					markerRel := filepath.ToSlash(filepath.Join(filepath.Dir(rel), storeschema.LeaseUntrustedFile))
					if err := addBytes(markerRel, 0o644, storeschema.LeaseUntrustedBody("base")); err != nil {
						return fmt.Errorf("lease untrust marker for %q: %w", rel, err)
					}
				}
			}
		default:
			// Symlinks and special files are not part of a store; the tar
			// form records their headers but Unpack never materializes them.
		}
		return nil
	})
	if err != nil {
		return "", 0, stats, fmt.Errorf("publisher: parts: %w", err)
	}
	raw, err := basemanifest.Marshal(basemanifest.Manifest{Entries: entries})
	if err != nil {
		return "", 0, stats, fmt.Errorf("publisher: manifest: %w", err)
	}
	sum := sha256.Sum256(raw)
	digest := hex.EncodeToString(sum[:])
	targetDir := filepath.Join(p.artifactsRoot, "sha256", Namespace)
	if err := writeFileAtomic(targetDir, ".tmp-projection-base-manifest-*", digest, raw); err != nil {
		return "", 0, stats, fmt.Errorf("publisher: write manifest: %w", err)
	}
	return digest, int64(len(raw)), stats, nil
}

// storePart makes part digest exist in partsDir. An existing regular part
// of the right size is reused and its mtime refreshed; otherwise the bytes
// are written to a temporary file, re-hashed, fsync'd and renamed into
// place. fresh reports whether bytes were written.
func (p *Publisher) storePart(partsDir, digest string, size int64, write func(io.Writer) error) (bool, error) {
	final := filepath.Join(partsDir, digest)
	if info, err := os.Stat(final); err == nil && info.Mode().IsRegular() && info.Size() == size {
		now := time.Now()
		if err := os.Chtimes(final, now, now); err != nil {
			return false, fmt.Errorf("refresh part: %w", err)
		}
		return false, nil
	}
	tmp, err := os.CreateTemp(partsDir, ".tmp-part-*")
	if err != nil {
		return false, err
	}
	tmpPath := tmp.Name()
	defer func() {
		if tmpPath != "" {
			_ = tmp.Close()
			_ = os.Remove(tmpPath)
		}
	}()
	hasher := sha256.New()
	counter := &countingWriter{}
	if err := write(io.MultiWriter(tmp, hasher, counter)); err != nil {
		return false, err
	}
	if got := hex.EncodeToString(hasher.Sum(nil)); got != digest || counter.n != size {
		return false, fmt.Errorf("file changed while publishing (digest %s size %d, want %s size %d)", got, counter.n, digest, size)
	}
	if err := tmp.Sync(); err != nil {
		return false, err
	}
	if err := tmp.Close(); err != nil {
		return false, err
	}
	if err := os.Rename(tmpPath, final); err != nil {
		return false, err
	}
	tmpPath = ""
	syncDir(partsDir)
	return true, nil
}

type countingWriter struct{ n int64 }

func (c *countingWriter) Write(b []byte) (int, error) {
	c.n += int64(len(b))
	return len(b), nil
}

func hashFile(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

func writeFileAtomic(dir, pattern, name string, body []byte) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}()
	if _, err := tmp.Write(body); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, filepath.Join(dir, name)); err != nil {
		return err
	}
	syncDir(dir)
	return nil
}

func syncDir(dir string) {
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
}

// partWriter copies part digest's bytes to dst.
type partWriter func(ctx context.Context, digest string, dst io.Writer) error

// materializeManifest builds the base the manifest lists into dstDir. Every
// file is written from its part while hashed and counted; a part that is
// missing, short, long or of other bytes refuses the base.
func materializeManifest(ctx context.Context, m Manifest, dstDir string, fetch partWriter) error {
	dstDir = filepath.Clean(dstDir)
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return fmt.Errorf("%w: create destination: %v", ErrBaseRefused, err)
	}
	for _, e := range m.Entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		target := filepath.Join(dstDir, filepath.FromSlash(e.Path))
		// Validate already refused unclean and escaping paths; this is the
		// belt to its braces.
		if rel, err := filepath.Rel(dstDir, target); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("%w: manifest path %q escapes the base", ErrBaseRefused, e.Path)
		}
		mode := os.FileMode(e.Mode).Perm()
		switch e.Type {
		case basemanifest.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("%w: mkdir %q: %v", ErrBaseRefused, e.Path, err)
			}
		case basemanifest.TypeFile:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("%w: mkdir parent %q: %v", ErrBaseRefused, e.Path, err)
			}
			if mode == 0 {
				mode = 0o644
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, mode|0o200)
			if err != nil {
				return fmt.Errorf("%w: create %q: %v", ErrBaseRefused, e.Path, err)
			}
			hasher := sha256.New()
			counter := &countingWriter{}
			ferr := fetch(ctx, e.SHA256, io.MultiWriter(out, hasher, counter))
			syncErr := out.Sync()
			closeErr := out.Close()
			if ferr != nil {
				// A refusal stays a refusal; a transport error stays plain,
				// so callers can tell an outage from a bad base (as for
				// DownloadBlob).
				return fmt.Errorf("part %s for %q: %w", e.SHA256, e.Path, ferr)
			}
			if syncErr != nil || closeErr != nil {
				return fmt.Errorf("%w: write %q: %v %v", ErrBaseRefused, e.Path, syncErr, closeErr)
			}
			if counter.n != e.Size || hex.EncodeToString(hasher.Sum(nil)) != e.SHA256 {
				return fmt.Errorf("%w: part %s for %q does not match its digest", ErrBaseRefused, e.SHA256, e.Path)
			}
		}
	}
	return nil
}

// localPartWriter reads parts from this host's artifacts root.
func localPartWriter(artifactsRoot string) partWriter {
	return func(ctx context.Context, digest string, dst io.Writer) error {
		f, err := os.Open(PartPath(artifactsRoot, digest))
		if err != nil {
			return fmt.Errorf("%w: part %s: %v", ErrBaseRefused, digest, err)
		}
		defer f.Close()
		_, err = io.Copy(dst, f)
		return err
	}
}

// readManifestBlob reads and verifies a manifest blob against its digest.
func readManifestBlob(blobPath, expectedDigest string) (Manifest, error) {
	raw, ok, err := basemanifest.ReadFile(blobPath)
	if err != nil {
		return Manifest{}, fmt.Errorf("%w: read manifest: %v", ErrBaseRefused, err)
	}
	if !ok {
		return Manifest{}, fmt.Errorf("%w: parts-form base blob is not a manifest", ErrBaseRefused)
	}
	sum := sha256.Sum256(raw)
	if hex.EncodeToString(sum[:]) != strings.ToLower(strings.TrimSpace(expectedDigest)) {
		return Manifest{}, fmt.Errorf("%w: base manifest digest mismatch", ErrBaseRefused)
	}
	m, err := basemanifest.Parse(raw)
	if err != nil {
		return Manifest{}, fmt.Errorf("%w: %v", ErrBaseRefused, err)
	}
	return m, nil
}

// materializeLocalBase builds a published base from this host's artifacts
// root into dstDir, in either form: the tar form streams and digest-checks
// the blob; the parts form verifies the manifest's digest and every part.
func materializeLocalBase(artifactsRoot, blobPath, digest, format, dstDir string) error {
	switch format {
	case "":
		return unpackVerified(blobPath, dstDir, digest)
	case FormatParts:
		m, err := readManifestBlob(blobPath, digest)
		if err != nil {
			return err
		}
		return materializeManifest(context.Background(), m, dstDir, localPartWriter(artifactsRoot))
	default:
		return fmt.Errorf("%w: unknown base format %q", ErrBaseRefused, format)
	}
}

// artifactsRootOfBlob is the artifacts root a published blob path
// (<root>/sha256/projection-base/<digest>) lives under.
func artifactsRootOfBlob(blobPath string) string {
	return filepath.Dir(filepath.Dir(filepath.Dir(filepath.Clean(blobPath))))
}

// PartSource is a BaseSource that can serve a parts-form base's parts. A
// source without it refuses a parts-form install.
type PartSource interface {
	DownloadPart(ctx context.Context, computerID, baseRef, partDigest string, dst io.Writer) error
}
