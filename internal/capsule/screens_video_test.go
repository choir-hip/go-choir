package capsule

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The walkthrough video (docs/problems/library-releases-have-no-screenshots-2026-11-04.md,
// design 5): choir-shot --video leaves at most one <name>.webm beside the
// PNGs, and Freeze stages it as the bundle's video. What can go wrong, each
// refused before the video is written to the bundle area:
//   - two videos, a nested path or an odd name;
//   - a file over MaxVideoBytes, bytes that are not a WebM (EBML header with
//     DocType webm), secret content inside the bytes;
//   - a staged file over an existing one;
//   - a record whose video digest or size is malformed (ValidateVideo).
// A .webm in the screens directory is no longer a refused screen name, and
// a Freeze with no video carries none.

func webmBytes(extra int) []byte {
	head := []byte{0x1a, 0x45, 0xdf, 0xa3, 0x9f, 0x42, 0x86, 0x81, 0x01, 0x42, 0xf7, 0x81, 0x01, 0x42, 0xf2, 0x81, 0x04, 0x42, 0xf3, 0x81, 0x08, 0x42, 0x82, 0x84, 'w', 'e', 'b', 'm', 0x42, 0x87, 0x81, 0x04, 0x42, 0x85, 0x81, 0x02}
	return append(head, bytes.Repeat([]byte{7}, extra)...)
}

func TestVideoNamesSplitFromScreens(t *testing.T) {
	changes := []FileChange{
		{Path: ScreensPrefix + "home.png", Kind: ChangeAdded, Mode: 0o644},
		{Path: ScreensPrefix + "home.webm", Kind: ChangeAdded, Mode: 0o644},
		{Path: ScreensPrefix + "old.webm", Kind: ChangeDeleted},
	}
	names, err := screenNames(changes)
	if err != nil || strings.Join(names, ",") != "home.png" {
		t.Fatalf("screens %v, %v", names, err)
	}
	video, err := videoName(changes)
	if err != nil || video != "home.webm" {
		t.Fatalf("video %q, %v", video, err)
	}
	if none, err := videoName(changes[:1]); none != "" || err != nil {
		t.Fatalf("no video: %q, %v", none, err)
	}
	two := append(changes, FileChange{Path: ScreensPrefix + "b.webm", Kind: ChangeAdded, Mode: 0o644})
	if _, err := videoName(two); err == nil {
		t.Fatal("two videos were admitted")
	}
	for _, bad := range []string{"a/b.webm", "x y.webm", ".webm", strings.Repeat("a", 120) + ".webm"} {
		c := []FileChange{{Path: ScreensPrefix + bad, Kind: ChangeAdded, Mode: 0o644}}
		_, serr := screenNames(c)
		_, verr := videoName(c)
		if serr == nil && verr == nil {
			t.Errorf("%q was admitted", bad)
		}
	}
}

func TestStageVideoFileBindsTheWebMBySHA256(t *testing.T) {
	ctx := context.Background()
	temporary := t.TempDir()
	home := testPNG(t, 64, 64, false)
	if _, err := stageScreenFiles(ctx, []string{"home.png"}, memOpen(map[string][]byte{"home.png": home}), temporary, time.Time{}, ""); err != nil {
		t.Fatal(err)
	}
	raw := webmBytes(4096)
	video, err := stageVideoFile(ctx, "home.webm", memOpen(map[string][]byte{"home.webm": raw}), temporary)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	if video == nil || video.Name != "home.webm" || video.SHA256 != hex.EncodeToString(sum[:]) || video.Bytes != int64(len(raw)) {
		t.Fatalf("video %+v", video)
	}
	if err := ValidateVideo(video); err != nil {
		t.Fatal(err)
	}
	staged, err := os.ReadFile(filepath.Join(temporary, filepath.FromSlash(BundleScreensDir), "home.webm"))
	if err != nil || !bytes.Equal(staged, raw) {
		t.Fatalf("staged bytes: %v", err)
	}
	// A video with no screens stages too.
	alone := t.TempDir()
	if _, err := stageVideoFile(ctx, "w.webm", memOpen(map[string][]byte{"w.webm": raw}), alone); err != nil {
		t.Fatal(err)
	}
	if none, err := stageVideoFile(ctx, "", memOpen(nil), t.TempDir()); none != nil || err != nil {
		t.Fatalf("no video: %v, %v", none, err)
	}
}

func TestStageVideoFileRefusesBadInputAndWritesNothing(t *testing.T) {
	ctx := context.Background()
	big := webmBytes(MaxVideoBytes)
	cases := map[string]func(string) (screenInput, error){
		"not a webm": memOpen(map[string][]byte{"a.webm": append([]byte("RIFF....AVI "), make([]byte, 64)...)}),
		"matroska":   memOpen(map[string][]byte{"a.webm": bytes.Replace(webmBytes(64), []byte("webm"), []byte("mkvx"), 1)}),
		"too large":  memOpen(map[string][]byte{"a.webm": big}),
		"missing":    memOpen(nil),
		"not regular": func(string) (screenInput, error) {
			return memInput{Reader: bytes.NewReader(webmBytes(64)), mode: os.ModeSymlink}, nil
		},
		"secret": memOpen(map[string][]byte{"a.webm": append(webmBytes(64),
			[]byte("\n-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEA\n-----END RSA PRIVATE KEY-----\n")...)}),
	}
	for name, open := range cases {
		temporary := t.TempDir()
		if _, err := stageVideoFile(ctx, "a.webm", open, temporary); err == nil {
			t.Errorf("%s: admitted", name)
		}
		if _, err := os.Stat(filepath.Join(temporary, filepath.FromSlash(BundleScreensDir), "a.webm")); !os.IsNotExist(err) {
			t.Errorf("%s: a refused video left a file in the bundle area", name)
		}
	}
	// Never over an existing file.
	temporary := t.TempDir()
	dir := filepath.Join(temporary, filepath.FromSlash(BundleScreensDir))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.webm"), []byte("other"), 0o444); err != nil {
		t.Fatal(err)
	}
	if _, err := stageVideoFile(ctx, "a.webm", memOpen(map[string][]byte{"a.webm": webmBytes(64)}), temporary); err == nil {
		t.Fatal("a video was staged over an existing file")
	}
}

func TestValidateVideoRefusesAMalformedRecord(t *testing.T) {
	if err := ValidateVideo(nil); err != nil {
		t.Fatal(err)
	}
	for name, v := range map[string]VideoFile{
		"bad digest": {Name: "a.webm", SHA256: "xyz", Bytes: 10},
		"bad name":   {Name: "a/b.webm", SHA256: strings.Repeat("a", 64), Bytes: 10},
		"a png name": {Name: "a.png", SHA256: strings.Repeat("a", 64), Bytes: 10},
		"no bytes":   {Name: "a.webm", SHA256: strings.Repeat("a", 64)},
		"too large":  {Name: "a.webm", SHA256: strings.Repeat("a", 64), Bytes: MaxVideoBytes + 1},
	} {
		if err := ValidateVideo(&v); err == nil {
			t.Errorf("%s: valid", name)
		}
	}
}
