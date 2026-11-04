package capsule

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// The walkthrough video (docs/problems/library-releases-have-no-screenshots-2026-11-04.md,
// design 5). choir-shot --video records one shot with Playwright's
// recordVideo and leaves <name>.webm beside the PNGs in ScreensDir. Freeze
// stages at most one, under evidence/screens/ like the screens, bound to the
// bundle by sha256 (CapsuleEffectBundle.Video). Its checks are the name, the
// size bound, the WebM header and the secret scan; no judge views it.
const (
	// MediaTypeWebM is the only video media type.
	MediaTypeWebM = "video/webm"
	// MaxVideoBytes bounds the video: a 20 s walkthrough in Playwright's VP8
	// at 1 Mbit/s is about 2.5 MB.
	MaxVideoBytes = 8 << 20
)

var videoNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,95}\.webm$`)

// ValidVideoName says name is a flat, plain WebM file name.
func ValidVideoName(name string) bool {
	return videoNamePattern.MatchString(name) && !strings.Contains(name, "..")
}

// VideoFile is the staged video as the bundle records it.
type VideoFile struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

// ValidateVideo checks a bundle record's video; nil is no video.
func ValidateVideo(v *VideoFile) error {
	if v == nil {
		return nil
	}
	if !ValidVideoName(v.Name) || !isHex64(v.SHA256) || v.Bytes <= 0 || v.Bytes > MaxVideoBytes {
		return fmt.Errorf("video: %q is not a content-addressed WebM of at most %d bytes", v.Name, MaxVideoBytes)
	}
	return nil
}

// videoName is the one .webm the capsule left directly in ScreensDir, or ""
// for none. Two are refused: choir-shot keeps one per run.
func videoName(changes []FileChange) (string, error) {
	var names []string
	for _, change := range changes {
		rel, ok := strings.CutPrefix(strings.TrimPrefix(change.Path, "/"), ScreensPrefix)
		if !ok || change.Kind == ChangeDeleted || change.Mode.IsDir() || rel == "" || !ValidVideoName(rel) {
			continue
		}
		names = append(names, rel)
	}
	if len(names) > 1 {
		return "", fmt.Errorf("capsule freeze: %s holds %d videos (%s); at most one", ScreensDir, len(names), strings.Join(names, ", "))
	}
	if len(names) == 0 {
		return "", nil
	}
	return names[0], nil
}

// IsWebM says raw starts with an EBML header whose DocType is webm.
func IsWebM(raw []byte) bool {
	if len(raw) < 8 || !bytes.Equal(raw[:4], []byte{0x1a, 0x45, 0xdf, 0xa3}) {
		return false
	}
	// DocType (0x4282), a one-byte size of 4, then "webm".
	return bytes.Contains(raw[:min(len(raw), 64)], []byte{0x42, 0x82, 0x84, 'w', 'e', 'b', 'm'})
}

// stageVideoFile reads the named video through open, bounds it, checks the
// WebM header, scans it for secrets and only then writes it under
// <temporary>/evidence/screens/, never over an existing file. nil for "".
func stageVideoFile(ctx context.Context, name string, open func(name string) (screenInput, error), temporary string) (*VideoFile, error) {
	if name == "" {
		return nil, nil
	}
	if !ValidVideoName(name) {
		return nil, fmt.Errorf("capsule freeze: video %q is not a plain .webm name", name)
	}
	input, err := open(name)
	if err != nil {
		return nil, fmt.Errorf("capsule freeze: video %q is unavailable: %w", name, err)
	}
	info, err := input.Stat()
	if err != nil || !info.Mode().IsRegular() {
		_ = input.Close()
		return nil, fmt.Errorf("capsule freeze: video %q is not a regular file", name)
	}
	data, err := io.ReadAll(io.LimitReader(input, MaxVideoBytes+1))
	_ = input.Close()
	if err != nil {
		return nil, err
	}
	if len(data) > MaxVideoBytes {
		return nil, fmt.Errorf("capsule freeze: video %q is larger than %d bytes", name, MaxVideoBytes)
	}
	if !IsWebM(data) {
		return nil, fmt.Errorf("capsule freeze: video %q is not a WebM file", name)
	}
	if err := refuseBinarySecrets(ctx, name, data); err != nil {
		return nil, err
	}
	sum := sha256.Sum256(data)
	out := &VideoFile{Name: name, SHA256: hex.EncodeToString(sum[:]), Bytes: int64(len(data))}
	// Release files never live under evidence/ (releasePathReserved), so the
	// directory is the screens' or new; the file itself must be new.
	target := filepath.Join(temporary, filepath.FromSlash(BundleScreensDir))
	if err := os.MkdirAll(target, 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(target, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o444)
	if err != nil {
		return nil, fmt.Errorf("capsule freeze: video %q: %w", name, err)
	}
	_, werr := f.Write(data)
	if err := errors.Join(werr, f.Sync(), f.Close()); err != nil {
		return nil, err
	}
	return out, nil
}
