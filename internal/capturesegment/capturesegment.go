// Package capturesegment stores Autopaper capture bodies outside Dolt: one
// append-only segment per UTC day in the computer's file root, one JSON line
// per body. The file root is synced to the platform in fixed-size chunks, so
// an append changes only a segment's last chunk. A ref names a body by day,
// byte offset, line length and sha256; Read verifies the hash.
// (docs/definitions/choir-autopaper-w1-content-in-the-graph-2026-10-11.md)
package capturesegment

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const refPrefix = "capseg:"

// Appender appends bodies to the day's segment under root/captures.
type Appender struct {
	root string
}

type line struct {
	SHA256 string `json:"sha256"`
	Body   []byte `json:"body"`
}

// appendLocks serializes appends per root across every Appender in the
// process, so two pollers never interleave lines.
var appendLocks sync.Map

func New(root string) *Appender {
	return &Appender{root: filepath.Clean(root)}
}

func (a *Appender) segmentPath(day time.Time) string {
	day = day.UTC()
	return filepath.Join(a.root, "captures", day.Format("2006"), day.Format("01"), day.Format("02")+".jsonl")
}

// Append writes body to the segment for day's UTC date and returns its ref.
// The bytes are synced to disk before the ref is returned, so a ref never
// names bytes that are not there.
func (a *Appender) Append(day time.Time, body []byte) (string, error) {
	mu, _ := appendLocks.LoadOrStore(a.root, &sync.Mutex{})
	mu.(*sync.Mutex).Lock()
	defer mu.(*sync.Mutex).Unlock()

	path := a.segmentPath(day)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", fmt.Errorf("capture segment: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o600)
	if err != nil {
		return "", fmt.Errorf("capture segment: %w", err)
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return "", fmt.Errorf("capture segment: %w", err)
	}
	offset := info.Size()
	var prefix []byte
	if offset > 0 {
		// A crash can leave a partial last line; start a fresh one.
		last := make([]byte, 1)
		if _, err := f.ReadAt(last, offset-1); err != nil {
			return "", fmt.Errorf("capture segment: %w", err)
		}
		if last[0] != '\n' {
			prefix = []byte{'\n'}
			offset++
		}
	}
	sum := sha256.Sum256(body)
	hash := hex.EncodeToString(sum[:])
	encoded, err := json.Marshal(line{SHA256: hash, Body: body})
	if err != nil {
		return "", fmt.Errorf("capture segment: %w", err)
	}
	if _, err := f.Write(append(append(prefix, encoded...), '\n')); err != nil {
		return "", fmt.Errorf("capture segment: %w", err)
	}
	if err := f.Sync(); err != nil {
		return "", fmt.Errorf("capture segment: %w", err)
	}
	return fmt.Sprintf("%s%s:%d:%d:%s", refPrefix, day.UTC().Format("2006-01-02"), offset, len(encoded), hash), nil
}

// Read returns the body a ref names, verifying its hash.
func Read(root, ref string) ([]byte, error) {
	path, offset, length, hash, err := parseRef(filepath.Clean(root), ref)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("capture segment: %w", err)
	}
	defer func() { _ = f.Close() }()
	buf := make([]byte, length)
	if _, err := f.ReadAt(buf, offset); err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("capture segment: ref %s names bytes past the end of the segment", ref)
		}
		return nil, fmt.Errorf("capture segment: %w", err)
	}
	var l line
	if err := json.Unmarshal(buf, &l); err != nil {
		return nil, fmt.Errorf("capture segment: ref %s does not name a whole line: %w", ref, err)
	}
	sum := sha256.Sum256(l.Body)
	if got := hex.EncodeToString(sum[:]); got != hash || l.SHA256 != hash {
		return nil, fmt.Errorf("capture segment: ref %s hash mismatch", ref)
	}
	return l.Body, nil
}

func parseRef(root, ref string) (path string, offset, length int64, hash string, err error) {
	parts := strings.Split(strings.TrimPrefix(ref, refPrefix), ":")
	if !strings.HasPrefix(ref, refPrefix) || len(parts) != 4 {
		return "", 0, 0, "", fmt.Errorf("capture segment: malformed ref %q", ref)
	}
	day, err := time.Parse("2006-01-02", parts[0])
	if err != nil {
		return "", 0, 0, "", fmt.Errorf("capture segment: malformed ref day %q", parts[0])
	}
	offset, err1 := strconv.ParseInt(parts[1], 10, 64)
	length, err2 := strconv.ParseInt(parts[2], 10, 64)
	if err1 != nil || err2 != nil || offset < 0 || length <= 0 || length > 64<<20 {
		return "", 0, 0, "", fmt.Errorf("capture segment: malformed ref range in %q", ref)
	}
	if _, err := hex.DecodeString(parts[3]); err != nil || len(parts[3]) != 64 {
		return "", 0, 0, "", fmt.Errorf("capture segment: malformed ref hash in %q", ref)
	}
	return New(root).segmentPath(day), offset, length, parts[3], nil
}
