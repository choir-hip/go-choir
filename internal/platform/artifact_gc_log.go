package platform

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/klauspost/compress/zstd"
)

// The artifact GC log is audit evidence, so it is compressed, never deleted
// (docs/problems/platform-artifact-retention-2026-10-22.md section 8, F6).

const (
	// artifactGCLogPlainDays is how many whole UTC days a daily log stays
	// plain: a file is rotated only when its day is more than 2 days before
	// today.
	artifactGCLogPlainDays = 2
	artifactGCLogZstSuffix = ".zst"
)

// artifactGCDayLogRE matches exactly the GC's own daily logs. It does not match
// tape-sweep-<run>.manifest.jsonl (a tape-sweep apply names a manifest by
// sha256) or checkpoint-scratch-<day>.jsonl (checkpointd's separate log).
var artifactGCDayLogRE = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})\.jsonl$`)

// artifactGCLogAppender appends records to one day's log.
type artifactGCLogAppender struct {
	file *os.File
	w    *bufio.Writer
}

// openArtifactGCLogForAppend opens (creating it) the day's log under the
// artifacts root.
func openArtifactGCLogForAppend(artifactsRoot string, day time.Time) (*artifactGCLogAppender, error) {
	dir := filepath.Join(artifactsRoot, artifactGCLogDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, day.UTC().Format("2006-01-02")+".jsonl"), os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	return &artifactGCLogAppender{file: f, w: bufio.NewWriter(f)}, nil
}

func (a *artifactGCLogAppender) append(rec artifactGCLogRecord) error {
	rec.At = time.Now().UTC().Format(time.RFC3339Nano)
	line, _ := json.Marshal(rec)
	_, err := a.w.Write(append(line, '\n'))
	return err
}

func (a *artifactGCLogAppender) sync() error {
	if err := a.w.Flush(); err != nil {
		return err
	}
	return a.file.Sync()
}

func (a *artifactGCLogAppender) Close() error {
	_ = a.w.Flush()
	return a.file.Close()
}

// openArtifactGCLog opens a daily log by its plain path (<day>.jsonl) and reads
// both forms: the plain file if it exists, otherwise the rotated .zst beside it.
func openArtifactGCLog(plainPath string) (io.ReadCloser, error) {
	f, err := os.Open(plainPath)
	if err == nil {
		return f, nil
	}
	if !os.IsNotExist(err) {
		return nil, err
	}
	z, zerr := os.Open(plainPath + artifactGCLogZstSuffix)
	if zerr != nil {
		return nil, err
	}
	dec, derr := zstd.NewReader(z)
	if derr != nil {
		z.Close()
		return nil, derr
	}
	return &zstdReadCloser{dec: dec, file: z}, nil
}

type zstdReadCloser struct {
	dec  *zstd.Decoder
	file *os.File
}

func (r *zstdReadCloser) Read(p []byte) (int, error) { return r.dec.Read(p) }
func (r *zstdReadCloser) Close() error {
	r.dec.Close()
	return r.file.Close()
}

// ArtifactGCLogRotation summarizes one rotation pass.
type ArtifactGCLogRotation struct {
	Compressed  int   `json:"logs_compressed,omitempty"`
	BytesBefore int64 `json:"log_bytes_before,omitempty"`
	BytesAfter  int64 `json:"log_bytes_after,omitempty"`
}

// rotateArtifactGCLogs compresses daily logs older than artifactGCLogPlainDays
// to <day>.jsonl.zst. The original is removed only after the compressed file,
// read back and decompressed, has the same sha256 and length as the original.
// Any failure keeps the original. A crash leaves one of: a stray .tmp (removed
// next pass), both copies (the .zst is re-verified, then the original goes), or
// a corrupt .zst (removed, the original is compressed again).
func (s *Service) rotateArtifactGCLogs(ctx context.Context, now time.Time) (ArtifactGCLogRotation, error) {
	var rot ArtifactGCLogRotation
	dir := filepath.Join(s.artifactsRoot, artifactGCLogDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return rot, nil
		}
		return rot, err
	}
	today := now.UTC().Truncate(24 * time.Hour)
	cutoff := today.AddDate(0, 0, -artifactGCLogPlainDays)
	var firstErr error
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return rot, err
		}
		m := artifactGCDayLogRE.FindStringSubmatch(e.Name())
		if m == nil || !e.Type().IsRegular() {
			continue
		}
		day, err := time.Parse("2006-01-02", m[1])
		if err != nil || !day.Before(cutoff) {
			continue
		}
		// A file modified within the plain window is not old, whatever its
		// name says (a clock step, or a log still being written).
		if fi, err := e.Info(); err != nil || fi.ModTime().After(now.Add(-artifactGCLogPlainDays*24*time.Hour)) {
			continue
		}
		before, after, err := rotateOneArtifactGCLog(filepath.Join(dir, e.Name()))
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("%s: %w", e.Name(), err)
			}
			continue
		}
		rot.Compressed++
		rot.BytesBefore += before
		rot.BytesAfter += after
	}
	// A stray .tmp from a crashed pass never matches a day log; clear them.
	if tmps, _ := filepath.Glob(filepath.Join(dir, "????-??-??.jsonl"+artifactGCLogZstSuffix+".tmp")); len(tmps) > 0 {
		for _, t := range tmps {
			_ = os.Remove(t)
		}
	}
	return rot, firstErr
}

func rotateOneArtifactGCLog(plain string) (before, after int64, err error) {
	zst := plain + artifactGCLogZstSuffix
	tmp := zst + ".tmp"
	info, err := os.Stat(plain)
	if err != nil {
		return 0, 0, err
	}
	before = info.Size()
	// Stale tmp from a crashed pass.
	_ = os.Remove(tmp)

	if zinfo, zerr := os.Lstat(zst); zerr == nil && zinfo.Mode().IsRegular() {
		verr := verifyZstdMatches(plain, zst)
		if verr == nil {
			// Crash after the rename: both copies, the compressed one is good.
			if err := os.Remove(plain); err != nil {
				return 0, 0, err
			}
			return before, zinfo.Size(), nil
		}
		if _, divergent := verr.(*zstdDivergentError); divergent {
			// A well-formed .zst that differs from the plain file may be the
			// only archived copy of earlier records: keep both.
			return 0, 0, fmt.Errorf("%s: both copies kept: %w", filepath.Base(zst), verr)
		}
		// A corrupt or partial .zst never passes for the log.
		if err := os.Remove(zst); err != nil {
			return 0, 0, err
		}
	}

	if err := compressZstdFile(plain, tmp); err != nil {
		_ = os.Remove(tmp)
		return 0, 0, err
	}
	if err := verifyZstdMatches(plain, tmp); err != nil {
		_ = os.Remove(tmp)
		return 0, 0, fmt.Errorf("round-trip check failed, original kept: %w", err)
	}
	if err := os.Rename(tmp, zst); err != nil {
		_ = os.Remove(tmp)
		return 0, 0, err
	}
	_ = os.Chtimes(zst, info.ModTime(), info.ModTime())
	if d, err := os.Open(filepath.Dir(zst)); err == nil {
		_ = d.Sync()
		d.Close()
	}
	// Verify the installed file once more before the original goes.
	if err := verifyZstdMatches(plain, zst); err != nil {
		_ = os.Remove(zst)
		return 0, 0, fmt.Errorf("installed file failed the round-trip check, original kept: %w", err)
	}
	zinfo, err := os.Stat(zst)
	if err != nil {
		return 0, 0, err
	}
	if err := os.Remove(plain); err != nil {
		return 0, 0, err
	}
	return before, zinfo.Size(), nil
}

func compressZstdFile(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := out.Close(); err == nil {
			err = cerr
		}
	}()
	enc, err := zstd.NewWriter(out, zstd.WithEncoderLevel(zstd.SpeedBetterCompression), zstd.WithEncoderConcurrency(1))
	if err != nil {
		return err
	}
	if _, err := io.Copy(enc, in); err != nil {
		enc.Close()
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	return out.Sync()
}

// verifyZstdMatches reads the compressed file back and requires its
// decompressed bytes to have the same length and sha256 as the original.
func verifyZstdMatches(plain, compressed string) error {
	want, wantN, err := sha256OfFile(plain, false)
	if err != nil {
		return err
	}
	got, gotN, err := sha256OfFile(compressed, true)
	if err != nil {
		return err
	}
	if wantN != gotN || !bytes.Equal(want, got) {
		return &zstdDivergentError{got: gotN, want: wantN}
	}
	return nil
}

// zstdDivergentError means the compressed file decoded cleanly but its bytes
// differ from the plain file's.
type zstdDivergentError struct{ got, want int64 }

func (e *zstdDivergentError) Error() string {
	return fmt.Sprintf("the compressed file differs from the plain file (decompressed %d bytes, plain %d bytes)", e.got, e.want)
}

func sha256OfFile(path string, decompress bool) ([]byte, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()
	var r io.Reader = f
	if decompress {
		dec, err := zstd.NewReader(f, zstd.WithDecoderConcurrency(1))
		if err != nil {
			return nil, 0, err
		}
		defer dec.Close()
		r = dec
	}
	h := sha256.New()
	n, err := io.Copy(h, r)
	if err != nil {
		return nil, 0, err
	}
	return h.Sum(nil), n, nil
}
