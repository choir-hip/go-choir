package platform

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
)

// The catalog fixture's media (docs/problems/library-releases-have-no-screenshots-2026-11-04.md,
// design 6). A Library round renders its gallery in a capsule against the
// fixture, so the fixture's releases carry screenshots and one walkthrough
// video published through the real handler. The bytes live beside the
// fixture as scripts/fixtures/capsule-catalog-media/<sha256>.<png|webm>, and
// choir-shot answers /api/catalog/releases/<id>/media/<sha> from them.
//
// The screenshots are drawn here, deterministically: a mock app window in
// the package's accent colour at the desktop (1280x800) and phone (390x844)
// sizes, never a capture of a real computer. The video is an input: the one
// .webm in the directory, a real Playwright recordVideo walkthrough of the
// Choir desktop (choir-shot --video, the mocked backend, no computer).

const capsuleCatalogMediaDir = "../../scripts/fixtures/capsule-catalog-media"

var fixtureAccents = map[string]color.RGBA{
	"files-next":   {46, 125, 196, 255},
	"slides-next":  {201, 92, 52, 255},
	"library-next": {92, 74, 170, 255},
}

func fill(img *image.RGBA, r image.Rectangle, c color.RGBA) {
	draw.Draw(img, r, &image.Uniform{C: c}, image.Point{}, draw.Src)
}

// fixtureScreenshot draws one mock screen of pkg: a desktop band, an app
// window with a title bar, a sidebar (desktop) and a grid of cards whose
// count and tint vary with seq, so every shot differs.
func fixtureScreenshot(t *testing.T, pkg string, seq, w, h int) []byte {
	t.Helper()
	accent := fixtureAccents[pkg]
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	fill(img, img.Bounds(), color.RGBA{214, 226, 238, 255})
	phone := w < h
	win := image.Rect(40, 30, w-40, h-70)
	if phone {
		win = image.Rect(0, 0, w, h-60)
	}
	fill(img, win, color.RGBA{250, 250, 252, 255})
	fill(img, image.Rect(win.Min.X, win.Min.Y, win.Max.X, win.Min.Y+36), accent)
	fill(img, image.Rect(0, h-56, w, h), color.RGBA{236, 238, 244, 255}) // the taskbar
	content := image.Rect(win.Min.X+16, win.Min.Y+52, win.Max.X-16, win.Max.Y-16)
	if !phone {
		side := image.Rect(win.Min.X, win.Min.Y+36, win.Min.X+220, win.Max.Y)
		fill(img, side, color.RGBA{238, 240, 246, 255})
		for i := 0; i < 6; i++ {
			y := side.Min.Y + 20 + i*34
			c := color.RGBA{200, 204, 214, 255}
			if i == seq%6 {
				c = accent
			}
			fill(img, image.Rect(side.Min.X+16, y, side.Max.X-16, y+20), c)
		}
		content.Min.X = side.Max.X + 20
	}
	cols := 3
	if phone {
		cols = 1
	}
	rows := 2 + seq%3
	gap := 16
	cw := (content.Dx() - gap*(cols-1)) / cols
	ch := (content.Dy() - gap*(rows-1)) / rows
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			x, y := content.Min.X+c*(cw+gap), content.Min.Y+r*(ch+gap)
			card := image.Rect(x, y, x+cw, y+ch)
			fill(img, card, color.RGBA{255, 255, 255, 255})
			tint := uint8(40 * ((r*cols + c + seq) % 5))
			fill(img, image.Rect(card.Min.X, card.Min.Y, card.Max.X, card.Min.Y+ch/2), color.RGBA{accent.R/2 + tint/2, accent.G/2 + 60, accent.B/2 + 80, 255})
			fill(img, image.Rect(card.Min.X+12, card.Min.Y+ch/2+14, card.Max.X-40, card.Min.Y+ch/2+26), color.RGBA{60, 64, 74, 255})
			fill(img, image.Rect(card.Min.X+12, card.Min.Y+ch/2+36, card.Max.X-90, card.Min.Y+ch/2+44), color.RGBA{150, 154, 164, 255})
		}
	}
	var buf bytes.Buffer
	if err := (&png.Encoder{CompressionLevel: png.BestCompression}).Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// fixtureVideo is the one .webm input in the media directory.
func fixtureVideo(t *testing.T) []byte {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(capsuleCatalogMediaDir, "*.webm"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("the fixture media directory holds %d .webm files (%v); it needs exactly one", len(matches), err)
	}
	raw, err := os.ReadFile(matches[0])
	if err != nil || !pkgrelease.IsWebM(raw) || len(raw) > pkgrelease.MaxVideoBytes {
		t.Fatalf("the fixture video %s is not a WebM within the cap: %v", matches[0], err)
	}
	return raw
}

// fixtureReleaseMedia draws fr's screenshots (seq is its publish order) and
// uploads them, with the video when fr names it, through the real media
// route. It returns the items and their bytes by digest.
func (w *pkgWorld) fixtureReleaseMedia(fr fixtureRelease, seq int, out map[string][]byte) []pkgrelease.Media {
	t := w.t
	t.Helper()
	var items []pkgrelease.Media
	add := func(item pkgrelease.Media, raw []byte) {
		if rec := w.do(pkgCall{method: "PUT", path: packagesRoute + "/media/" + item.SHA256, internal: true, body: raw}); rec.Code != 201 {
			t.Fatalf("fixture media upload = %d %s", rec.Code, rec.Body.String())
		}
		items = append(items, item)
		out[item.SHA256] = raw
	}
	shot := func(name string, n, width, height int) {
		raw := fixtureScreenshot(t, fr.pkg, seq*7+n, width, height)
		add(pkgrelease.Media{Kind: pkgrelease.MediaKindScreenshot, Name: name, MediaType: pkgrelease.MediaTypePNG,
			SHA256: computerevent.DigestBytes(raw), Bytes: len(raw), Width: width, Height: height}, raw)
	}
	// The store lists no release without a screenshot: one that names none
	// gets a single desktop shot.
	desktop := fr.desktop
	if desktop == 0 && fr.phone == 0 {
		desktop = 1
	}
	for i := 0; i < desktop; i++ {
		shot(fr.pkg+"-desktop-"+string(rune('1'+i))+".png", i, 1280, 800)
	}
	for i := 0; i < fr.phone; i++ {
		shot(fr.pkg+"-phone-"+string(rune('1'+i))+"-mobile.png", 10+i, 390, 844)
	}
	if fr.video {
		raw := fixtureVideo(t)
		add(pkgrelease.Media{Kind: pkgrelease.MediaKindVideo, Name: fr.pkg + "-walkthrough.webm", MediaType: pkgrelease.MediaTypeWebM,
			SHA256: computerevent.DigestBytes(raw), Bytes: len(raw)}, raw)
	}
	return items
}

// writeFixtureMedia writes each screenshot as <sha>.png; the video is the
// input already there.
func writeFixtureMedia(t *testing.T, media map[string][]byte) {
	t.Helper()
	if err := os.MkdirAll(capsuleCatalogMediaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	old, _ := filepath.Glob(filepath.Join(capsuleCatalogMediaDir, "*.png"))
	for _, path := range old {
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
	for digest, raw := range media {
		if pkgrelease.IsWebM(raw) {
			continue
		}
		if err := os.WriteFile(filepath.Join(capsuleCatalogMediaDir, digest+".png"), raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// checkFixtureMedia holds the fixture to its files: every item a held
// release names has its file, hashing to its name and verifying as the
// item; the media directory holds nothing else; every release carries
// screenshots and one a video.
func checkFixtureMedia(t *testing.T, held capsuleCatalogFixture) {
	t.Helper()
	named := map[string]bool{}
	withMedia, without, videos := 0, 0, 0
	for _, r := range held.Releases {
		var view struct {
			ReleaseID string             `json:"release_id"`
			Media     []PackageMediaView `json:"media"`
		}
		if err := json.Unmarshal(r, &view); err != nil {
			t.Fatal(err)
		}
		if len(view.Media) == 0 {
			without++
		} else {
			withMedia++
		}
		for _, m := range view.Media {
			ext := ".png"
			if m.Kind == pkgrelease.MediaKindVideo {
				ext, videos = ".webm", videos+1
			}
			raw, err := os.ReadFile(filepath.Join(capsuleCatalogMediaDir, m.SHA256+ext))
			if err != nil || pkgrelease.VerifyMedia(m.Media, raw) != nil {
				t.Fatalf("release %s media %s has no verifying file in %s (regenerate with CHOIR_WRITE_CAPSULE_CATALOG_FIXTURE=1): %v", view.ReleaseID, m.Name, capsuleCatalogMediaDir, err)
			}
			if m.URL != PackageMediaURL(view.ReleaseID, m.SHA256) {
				t.Fatalf("media url %q", m.URL)
			}
			named[m.SHA256+ext] = true
		}
	}
	if withMedia == 0 || without != 0 || videos == 0 {
		t.Fatalf("every fixture release needs a screenshot (%d with, %d without), and one a video (%d)", withMedia, without, videos)
	}
	entries, err := os.ReadDir(capsuleCatalogMediaDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !named[e.Name()] {
			t.Fatalf("%s/%s is named by no fixture release", capsuleCatalogMediaDir, e.Name())
		}
	}
	if !strings.Contains(string(held.Packages), `"head_media"`) {
		t.Fatal("the fixture's catalog entries carry no head_media")
	}
}
