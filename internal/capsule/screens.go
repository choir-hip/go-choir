package capsule

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// Screens (docs/problems/selfdev-screens-are-not-files-2026-11-03.md): the
// screenshots the engineer takes itself, with Playwright and the store
// Chromium, of real content. It writes PNGs to ScreensDir in its capsule;
// Freeze stages them under <temporary>/evidence/screens/ (beside release/,
// never under it, so the release never carries them) and the bundle record
// binds each by sha256. The runtime then saves them as files in the
// computer's file tree, where Texture and the judge read them.
const (
	// ScreensDir is where an engineer writes its screenshots: beside the
	// release tree, outside /workspace/platform, so the source patch and the
	// release never carry them and the classifier files them as artifacts.
	ScreensDir = "/var/lib/artifact/screens"
	// ScreensPrefix is ScreensDir as a capsule diff path.
	ScreensPrefix = "var/lib/artifact/screens/"
	// BundleScreensDir is where Freeze stages them inside a bundle directory.
	BundleScreensDir = "evidence/screens"
	// MediaTypePNG is the only screen media type.
	MediaTypePNG = "image/png"

	// MaxScreens bounds one candidate's screens.
	MaxScreens = 12
	// MaxScreenBytes is under the gateway's 20 MiB image limit.
	MaxScreenBytes = 8 << 20
	// MaxScreensTotalBytes bounds one bundle's screens.
	MaxScreensTotalBytes = 32 << 20
	// MaxScreenPixels bounds a decode (a full-page capture of a long page
	// fits; a decompression bomb does not).
	MaxScreenPixels = 4096 * 16384
)

var screenNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,95}\.png$`)

// ValidScreenName says name is a flat, plain PNG file name.
func ValidScreenName(name string) bool {
	return screenNamePattern.MatchString(name) && !strings.Contains(name, "..")
}

// ScreenFile is one staged screenshot as the bundle records it.
type ScreenFile struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	// Blank is recomputed from the image: a uniform picture.
	Blank bool `json:"blank,omitempty"`
}

// ValidateScreens checks a bundle record's screens (names, digests, bounds).
func ValidateScreens(screens []ScreenFile) error {
	if len(screens) > MaxScreens {
		return fmt.Errorf("screens: at most %d are admitted", MaxScreens)
	}
	seen := map[string]bool{}
	var total int64
	for _, s := range screens {
		if !ValidScreenName(s.Name) || !isHex64(s.SHA256) || s.Bytes <= 0 || s.Bytes > MaxScreenBytes || s.Width <= 0 || s.Height <= 0 {
			return fmt.Errorf("screens: %q is not a content-addressed PNG", s.Name)
		}
		if seen[s.Name] {
			return fmt.Errorf("screens: %s is named twice", s.Name)
		}
		seen[s.Name] = true
		total += s.Bytes
	}
	if total > MaxScreensTotalBytes {
		return fmt.Errorf("screens: %d bytes exceed %d", total, MaxScreensTotalBytes)
	}
	return nil
}

// screenNames lists the PNGs the capsule left directly in ScreensDir, from
// its diff. Deleted entries and directories are skipped; a nested path or a
// file that is not a plain PNG name is refused.
func screenNames(changes []FileChange) ([]string, error) {
	var names []string
	for _, change := range changes {
		rel, ok := strings.CutPrefix(strings.TrimPrefix(change.Path, "/"), ScreensPrefix)
		if !ok || change.Kind == ChangeDeleted || change.Mode.IsDir() || rel == "" {
			continue
		}
		if !ValidScreenName(rel) {
			return nil, fmt.Errorf("capsule freeze: screen %q is not a .png file directly in %s (names: letters, digits, '.', '-', '_')", change.Path, ScreensDir)
		}
		names = append(names, rel)
	}
	if len(names) > MaxScreens {
		return nil, fmt.Errorf("capsule freeze: %s holds %d screens; at most %d", ScreensDir, len(names), MaxScreens)
	}
	return names, nil
}

// screenInput is an opened screen file; *os.File satisfies it.
type screenInput interface {
	io.Reader
	io.Closer
	Stat() (os.FileInfo, error)
}

// stageScreenFiles reads each named screen through open (the Linux executor
// opens beneath the capsule root without following symlinks), bounds it,
// decodes it as a PNG, scans it for secrets and only then writes the set
// under <temporary>/evidence/screens/. A refused set writes nothing.
func stageScreenFiles(ctx context.Context, names []string, open func(name string) (screenInput, error), temporary string) ([]ScreenFile, error) {
	if len(names) == 0 {
		return nil, nil
	}
	var out []ScreenFile
	contents := map[string][]byte{}
	var total int64
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		input, err := open(name)
		if err != nil {
			return nil, fmt.Errorf("capsule freeze: screen %q is unavailable: %w", name, err)
		}
		info, err := input.Stat()
		if err != nil || !info.Mode().IsRegular() {
			_ = input.Close()
			return nil, fmt.Errorf("capsule freeze: screen %q is not a regular file", name)
		}
		data, err := io.ReadAll(io.LimitReader(input, MaxScreenBytes+1))
		_ = input.Close()
		if err != nil {
			return nil, err
		}
		if len(data) > MaxScreenBytes {
			return nil, fmt.Errorf("capsule freeze: screen %q is larger than %d bytes", name, MaxScreenBytes)
		}
		if total += int64(len(data)); total > MaxScreensTotalBytes {
			return nil, fmt.Errorf("capsule freeze: screens exceed %d bytes", MaxScreensTotalBytes)
		}
		width, height, blank, err := CheckScreenPNG(data)
		if err != nil {
			return nil, fmt.Errorf("capsule freeze: screen %q: %w", name, err)
		}
		if err := refuseBinarySecrets(ctx, name, data); err != nil {
			return nil, err
		}
		sum := sha256.Sum256(data)
		out = append(out, ScreenFile{Name: name, SHA256: hex.EncodeToString(sum[:]), Bytes: int64(len(data)), Width: width, Height: height, Blank: blank})
		contents[name] = data
	}
	if err := ValidateScreens(out); err != nil {
		return nil, fmt.Errorf("capsule freeze: %w", err)
	}
	target := filepath.Join(temporary, filepath.FromSlash(BundleScreensDir))
	if err := os.MkdirAll(target, 0o700); err != nil {
		return nil, err
	}
	for name, data := range contents {
		if err := os.WriteFile(filepath.Join(target, name), data, 0o444); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// CheckScreenPNG decodes data as a PNG within the pixel bound and reports
// its size and whether it is uniform (blank).
func CheckScreenPNG(data []byte) (width, height int, blank bool, err error) {
	if len(data) == 0 {
		return 0, 0, false, fmt.Errorf("empty")
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0, false, fmt.Errorf("not a PNG: %w", err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width*cfg.Height > MaxScreenPixels {
		return 0, 0, false, fmt.Errorf("%dx%d exceeds %d pixels", cfg.Width, cfg.Height, MaxScreenPixels)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return 0, 0, false, fmt.Errorf("does not decode: %w", err)
	}
	return cfg.Width, cfg.Height, uniformImage(img), nil
}

func uniformImage(img image.Image) bool {
	b := img.Bounds()
	r0, g0, b0, a0 := img.At(b.Min.X, b.Min.Y).RGBA()
	for y := b.Min.Y; y < b.Max.Y; y += 2 {
		for x := b.Min.X; x < b.Max.X; x += 2 {
			r, g, bl, a := img.At(x, y).RGBA()
			if r != r0 || g != g0 || bl != b0 || a != a0 {
				return false
			}
		}
	}
	return true
}

// refuseBinarySecrets scans bytes in windows, carrying the tail of the
// previous window so a secret straddling a boundary is still found (the
// release scan's rule for binary payloads).
func refuseBinarySecrets(ctx context.Context, name string, data []byte) error {
	const window, carry = 64 << 10, 512
	for start := 0; start < len(data); start += window {
		if err := ctx.Err(); err != nil {
			return err
		}
		from := max(start-carry, 0)
		if len(computerevent.DetectBinarySecrets(data[from:min(start+window, len(data))])) != 0 {
			return fmt.Errorf("capsule freeze: screen %q holds secret content", name)
		}
	}
	return nil
}

func isHex64(s string) bool {
	if len(s) != 64 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil && strings.ToLower(s) == s
}
