package capsule

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Screens (docs/problems/selfdev-screens-are-not-files-2026-11-03.md): the
// engineer's own screenshots, PNGs it wrote to ScreensDir, staged at Freeze
// beside release/ and bound to the bundle by sha256. What can go wrong, each
// refused before anything is written to the bundle area:
//   - a nested path, a non-PNG name, a name with odd characters, or a
//     symlink (the Linux opener refuses symlinks; here: a non-regular file);
//   - too many files, one too large, too many bytes in all;
//   - bytes that are not a PNG, or a PNG too large to decode safely;
//   - secret content inside the bytes;
//   - a staged digest that is not the bytes' (Validate on the record).
// Deleted entries and directories are not screens, and no screens is not a
// refusal: a Freeze with none carries none.

func testPNG(t *testing.T, w, h int, uniform bool) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := color.RGBA{R: 20, G: 30, B: 40, A: 255}
			if !uniform && (x+y)%7 == 0 {
				c = color.RGBA{R: 200, G: 210, B: 220, A: 255}
			}
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

type memInput struct {
	*bytes.Reader
	mode os.FileMode
	mod  time.Time
}

func (m memInput) Close() error { return nil }
func (m memInput) Stat() (os.FileInfo, error) {
	return memInfo{size: int64(m.Len()), mode: m.mode, mod: m.mod}, nil
}

type memInfo struct {
	size int64
	mode os.FileMode
	mod  time.Time
}

func (i memInfo) Name() string       { return "x" }
func (i memInfo) Size() int64        { return i.size }
func (i memInfo) Mode() os.FileMode  { return i.mode }
func (i memInfo) ModTime() time.Time { return i.mod }
func (i memInfo) IsDir() bool        { return i.mode.IsDir() }
func (i memInfo) Sys() any           { return nil }

func memOpen(files map[string][]byte) func(string) (screenInput, error) {
	return func(name string) (screenInput, error) {
		data, ok := files[name]
		if !ok {
			return nil, os.ErrNotExist
		}
		return memInput{Reader: bytes.NewReader(data)}, nil
	}
}

func TestScreenNamesReadsTheFlatScreensDirectory(t *testing.T) {
	changes := []FileChange{
		{Path: "/" + ScreensPrefix + "home.png", Kind: ChangeAdded, Mode: 0o644},
		{Path: ScreensPrefix + "slides-dark.png", Kind: ChangeModified, Mode: 0o644},
		{Path: ScreensPrefix + "gone.png", Kind: ChangeDeleted},
		{Path: ScreensPrefix, Kind: ChangeAdded, Mode: os.ModeDir | 0o755},
		{Path: "workspace/platform/x.go", Kind: ChangeAdded, Mode: 0o644},
	}
	names, err := screenNames(changes)
	if err != nil || strings.Join(names, ",") != "home.png,slides-dark.png" {
		t.Fatalf("names %v, %v", names, err)
	}
	for _, bad := range []string{"a/b.png", "notes.txt", "x y.png", ".png", "../x.png", strings.Repeat("a", 120) + ".png"} {
		if _, err := screenNames([]FileChange{{Path: ScreensPrefix + bad, Kind: ChangeAdded, Mode: 0o644}}); err == nil {
			t.Errorf("%q was admitted", bad)
		}
	}
	var many []FileChange
	for i := 0; i <= MaxScreens; i++ {
		many = append(many, FileChange{Path: ScreensPrefix + "s" + string(rune('a'+i)) + ".png", Kind: ChangeAdded, Mode: 0o644})
	}
	if _, err := screenNames(many); err == nil {
		t.Fatal("more than MaxScreens were admitted")
	}
}

func TestStageScreenFilesBindsEachPNGBySHA256(t *testing.T) {
	ctx := context.Background()
	home, blank := testPNG(t, 320, 200, false), testPNG(t, 300, 200, true)
	temporary := t.TempDir()
	screens, err := stageScreenFiles(ctx, []string{"home.png", "blank.png"}, memOpen(map[string][]byte{"home.png": home, "blank.png": blank}), temporary, time.Time{}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(screens) != 2 {
		t.Fatalf("screens %+v", screens)
	}
	sum := sha256.Sum256(home)
	got := screens[0]
	if got.Name != "home.png" || got.SHA256 != hex.EncodeToString(sum[:]) || got.Bytes != int64(len(home)) || got.Width != 320 || got.Height != 200 || got.Blank {
		t.Fatalf("home %+v", got)
	}
	if !screens[1].Blank {
		t.Fatalf("a uniform PNG is not marked blank: %+v", screens[1])
	}
	staged, err := os.ReadFile(filepath.Join(temporary, filepath.FromSlash(BundleScreensDir), "home.png"))
	if err != nil || !bytes.Equal(staged, home) {
		t.Fatalf("staged bytes: %v", err)
	}
	if err := ValidateScreens(screens); err != nil {
		t.Fatal(err)
	}
	if none, err := stageScreenFiles(ctx, nil, memOpen(nil), t.TempDir(), time.Time{}, ""); none != nil || err != nil {
		t.Fatalf("no screens: %v, %v", none, err)
	}
}

func TestStageScreenFilesRefusesBadInputAndWritesNothing(t *testing.T) {
	ctx := context.Background()
	good := testPNG(t, 64, 64, false)
	big := make([]byte, MaxScreenBytes+1)
	copy(big, good)
	cases := map[string]func(string) (screenInput, error){
		"not a PNG": memOpen(map[string][]byte{"a.png": []byte("GIF89a not a png")}),
		"too large": memOpen(map[string][]byte{"a.png": big}),
		"missing":   memOpen(nil),
		"not regular": func(string) (screenInput, error) {
			return memInput{Reader: bytes.NewReader(good), mode: os.ModeSymlink}, nil
		},
		"secret": memOpen(map[string][]byte{"a.png": append(append([]byte{}, good...),
			[]byte("\n-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEA\n-----END RSA PRIVATE KEY-----\n")...)}),
	}
	for name, open := range cases {
		temporary := t.TempDir()
		if _, err := stageScreenFiles(ctx, []string{"a.png"}, open, temporary, time.Time{}, ""); err == nil {
			t.Errorf("%s: admitted", name)
		}
		if _, err := os.Stat(filepath.Join(temporary, filepath.FromSlash(BundleScreensDir))); !os.IsNotExist(err) {
			t.Errorf("%s: a refused set left files in the bundle area", name)
		}
	}
}

func TestValidateScreensRefusesAMalformedRecord(t *testing.T) {
	ok := ScreenFile{Name: "a.png", SHA256: strings.Repeat("a", 64), Bytes: 10, Width: 10, Height: 10}
	for name, s := range map[string][]ScreenFile{
		"bad digest": {{Name: "a.png", SHA256: "xyz", Bytes: 10, Width: 10, Height: 10}},
		"bad name":   {{Name: "a/b.png", SHA256: ok.SHA256, Bytes: 10, Width: 10, Height: 10}},
		"no bytes":   {{Name: "a.png", SHA256: ok.SHA256, Width: 10, Height: 10}},
		"twice":      {ok, ok},
	} {
		if err := ValidateScreens(s); err == nil {
			t.Errorf("%s: valid", name)
		}
	}
}

func TestStageScreenFilesRefusesToShareTheStagingPathWithAReleaseFile(t *testing.T) {
	temporary := t.TempDir()
	dir := filepath.Join(temporary, filepath.FromSlash(BundleScreensDir))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "home.png"), []byte("release bytes"), 0o444); err != nil {
		t.Fatal(err)
	}
	good := testPNG(t, 64, 64, false)
	if _, err := stageScreenFiles(context.Background(), []string{"home.png"}, memOpen(map[string][]byte{"home.png": good}), temporary, time.Time{}, ""); err == nil {
		t.Fatal("a screen was staged over an existing file")
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "home.png")); string(got) != "release bytes" {
		t.Fatalf("existing file overwritten: %q", got)
	}
}

func TestReleasePathUnderEvidenceIsReserved(t *testing.T) {
	for _, p := range []string{"evidence/screens/x.png", "evidence", "evidence/other"} {
		if !releasePathReserved(p) {
			t.Errorf("%s is not reserved", p)
		}
	}
	for _, p := range []string{"frontend/index.html", "bin/autoputer", "frontend/evidence/x"} {
		if releasePathReserved(p) {
			t.Errorf("%s is reserved", p)
		}
	}
}

func TestBlankCheckIsExactAndPixelBoundIsSane(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 40, 40))
	for y := 0; y < 40; y++ {
		for x := 0; x < 40; x++ {
			img.Set(x, y, color.RGBA{R: 9, G: 9, B: 9, A: 255})
		}
	}
	img.Set(5, 7, color.RGBA{R: 200, A: 255}) // odd coordinates: a stride-2 sample misses it
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	if _, _, blank, err := CheckScreenPNG(buf.Bytes()); err != nil || blank {
		t.Fatalf("one differing pixel: blank=%v err=%v", blank, err)
	}
	if MaxScreenPixels > 4096*8192 {
		t.Fatalf("MaxScreenPixels = %d, want at most %d", MaxScreenPixels, 4096*8192)
	}
	big := image.NewPaletted(image.Rect(0, 0, 4096, 8200), color.Palette{color.Black, color.White})
	buf.Reset()
	if err := png.Encode(&buf, big); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := CheckScreenPNG(buf.Bytes()); err == nil {
		t.Fatal("an oversized PNG was admitted")
	}
}

// A screen shot before the newest source change shows an older build: the
// engineer edited and rebuilt but froze the old shots (claude arm Calendar
// candidate 2, 2026-11-04). What can go wrong: build outputs or node_modules
// counted as source (every screen stale); equal times (mtime granularity)
// refused; no source change or no screens refused; deleted or directory
// changes statted.
func TestStageScreenFilesRefusesScreensOlderThanTheLastSourceChange(t *testing.T) {
	ctx := context.Background()
	shot := testPNG(t, 32, 32, false)
	at := time.Date(2026, 11, 4, 11, 8, 30, 0, time.UTC)
	openAt := func(mod time.Time) func(string) (screenInput, error) {
		return func(string) (screenInput, error) {
			return memInput{Reader: bytes.NewReader(shot), mod: mod}, nil
		}
	}
	names := []string{"01-month.png", "02-week.png"}
	_, err := stageScreenFiles(ctx, names, openAt(at.Add(-time.Second)), t.TempDir(), at, "calendar-next.css")
	if err == nil || !strings.Contains(err.Error(), "2 screens predate the last source change (calendar-next.css at 11:08:30Z)") || !strings.Contains(err.Error(), "01-month.png, 02-week.png") {
		t.Fatalf("stale screens: %v", err)
	}
	for name, c := range map[string]struct {
		mod, notBefore time.Time
	}{"fresh": {at.Add(time.Second), at}, "equal": {at, at}, "no source change": {at.Add(-time.Hour), time.Time{}}} {
		if _, err := stageScreenFiles(ctx, names, openAt(c.mod), t.TempDir(), c.notBefore, ""); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	if none, err := stageScreenFiles(ctx, nil, memOpen(nil), t.TempDir(), at, "x.css"); none != nil || err != nil {
		t.Errorf("no screens: %v, %v", none, err)
	}
}

func TestNewestSourceChangeCountsOnlySourceFiles(t *testing.T) {
	base := time.Date(2026, 11, 4, 11, 0, 0, 0, time.UTC)
	mods := map[string]time.Time{
		"/workspace/platform/a.css":                   base.Add(time.Minute),
		"workspace/platform/b.css":                    base.Add(2 * time.Minute),
		"/workspace/platform/node_modules/x/index.js": base.Add(time.Hour),
		"/workspace/platform/frontend/dist/app.js":    base.Add(time.Hour),
		"/workspace/platform/docs/problems/x.md":      base.Add(time.Hour),
		"/workspace/platform/.claude/settings.json":   base.Add(time.Hour),
		"/workspace/platform/.git/index":              base.Add(time.Hour),
		"/" + ScreensPrefix + "home.png":              base.Add(time.Hour),
		"/workspace/platform/gone.css":                base.Add(time.Hour),
		"/workspace/platform/sub":                     base.Add(time.Hour),
	}
	changes := []FileChange{
		{Path: "/workspace/platform/a.css", Kind: ChangeModified, Mode: 0o644},
		{Path: "workspace/platform/b.css", Kind: ChangeAdded, Mode: 0o644},
		{Path: "/workspace/platform/node_modules/x/index.js", Kind: ChangeAdded, Mode: 0o644},
		{Path: "/workspace/platform/frontend/dist/app.js", Kind: ChangeModified, Mode: 0o644},
		{Path: "/workspace/platform/docs/problems/x.md", Kind: ChangeAdded, Mode: 0o644},
		{Path: "/workspace/platform/.claude/settings.json", Kind: ChangeAdded, Mode: 0o644},
		{Path: "/workspace/platform/.git/index", Kind: ChangeModified, Mode: 0o644},
		{Path: "/" + ScreensPrefix + "home.png", Kind: ChangeAdded, Mode: 0o644},
		{Path: "/workspace/platform/gone.css", Kind: ChangeDeleted},
		{Path: "/workspace/platform/sub", Kind: ChangeAdded, Mode: os.ModeDir | 0o755},
	}
	got, path, err := newestSourceChange(changes, func(p string) (time.Time, error) { return mods[p], nil })
	if err != nil || !got.Equal(base.Add(2*time.Minute)) || path != "b.css" {
		t.Fatalf("newest %v %q %v", got, path, err)
	}
	if zero, _, err := newestSourceChange(changes[5:], func(string) (time.Time, error) { return time.Now(), nil }); err != nil || !zero.IsZero() {
		t.Fatalf("no source: %v %v", zero, err)
	}
}
