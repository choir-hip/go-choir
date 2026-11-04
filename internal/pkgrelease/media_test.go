package pkgrelease

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// Release media (docs/problems/library-releases-have-no-screenshots-2026-11-04.md).
// Failure modes pinned:
//
//	M1  a record published before media hashes differently (its release id
//	    moves) because an empty media list canonicalizes as a field;
//	M2  a record names more than 8 screenshots or more than 1 video, an
//	    oversize item, a path-shaped name, a duplicate, or a kind/type pair
//	    that does not agree;
//	M3  bytes that are not the item pass VerifyMedia: another digest, a
//	    truncated file, a PNG of another size, a non-PNG under image/png, a
//	    non-WebM under video/webm.

func testPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 90, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// testWebM is the EBML header ffmpeg's matroska muxer writes for WebM,
// followed by filler.
func testWebM() []byte {
	head := []byte{0x1a, 0x45, 0xdf, 0xa3, 0x9f, 0x42, 0x86, 0x81, 0x01, 0x42, 0xf7, 0x81, 0x01, 0x42, 0xf2, 0x81, 0x04, 0x42, 0xf3, 0x81, 0x08, 0x42, 0x82, 0x84, 'w', 'e', 'b', 'm', 0x42, 0x87, 0x81, 0x04, 0x42, 0x85, 0x81, 0x02}
	return append(head, bytes.Repeat([]byte{0}, 64)...)
}

func shotOf(name string, raw []byte, w, h int) Media {
	return Media{Kind: MediaKindScreenshot, Name: name, MediaType: MediaTypePNG, SHA256: computerevent.DigestBytes(raw), Bytes: len(raw), Width: w, Height: h}
}

func videoOf(name string, raw []byte) Media {
	return Media{Kind: MediaKindVideo, Name: name, MediaType: MediaTypeWebM, SHA256: computerevent.DigestBytes(raw), Bytes: len(raw)}
}

func mediaRecord() Record {
	return Record{
		Schema: Schema, Package: Package{Name: "reader", ManifestVersion: "1.0.0"}, Track: "owner-a",
		Parent: Parent{Kind: ParentBase, Commit: strings.Repeat("c", 40), Tree: EmptyTree},
		Source: Source{
			Root: Root("reader"), Tree: strings.Repeat("d", 40), TreePatch: Blob{SHA256: strings.Repeat("1", 64), Bytes: 10},
			ReviewPatch: Blob{SHA256: strings.Repeat("2", 64), Bytes: 10}, ChangedPaths: []string{Root("reader") + "/App.svelte"}, BuiltOn: strings.Repeat("c", 40),
		},
		Manifest:  Manifest{SHA256: strings.Repeat("3", 64), ABI: "choir.package.v1"},
		Publisher: Publisher{OwnerID: "owner-a", ComputerID: "computer-a1"},
		Operation: Operation{OperationID: "op-1", BundleDigest: strings.Repeat("b", 64), DecisionEvent: strings.Repeat("e", 64)},
	}
}

func TestMediaOmittedKeepsTheReleaseID(t *testing.T) {
	record := mediaRecord()
	if err := record.Validate(); err != nil {
		t.Fatal(err)
	}
	without, err := record.ReleaseID()
	if err != nil {
		t.Fatal(err)
	}
	record.Media = []Media{}
	empty, err := record.ReleaseID()
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := record.Canonical()
	if without != empty || bytes.Contains(raw, []byte(`"media"`)) {
		t.Fatalf("an empty media list changed the record: %s vs %s, %s", without, empty, raw)
	}
	record.Media = []Media{shotOf("home.png", testPNG(t, 8, 6), 8, 6)}
	with, err := record.ReleaseID()
	if err != nil || with == without {
		t.Fatalf("media is not bound into the release id: %v", err)
	}
	decoded, err := Decode(mustCanonical(t, record))
	if err != nil || len(decoded.Media) != 1 || decoded.Media[0] != record.Media[0] {
		t.Fatalf("decode round trip: %v %+v", err, decoded.Media)
	}
}

func mustCanonical(t *testing.T, r Record) []byte {
	t.Helper()
	raw, err := r.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestMediaBounds(t *testing.T) {
	pngBytes := testPNG(t, 8, 6)
	shot := func(i int) Media {
		m := shotOf("s"+string(rune('a'+i))+".png", pngBytes, 8, 6)
		m.SHA256 = computerevent.DigestBytes([]byte{byte(i)})
		return m
	}
	eight := []Media{}
	for i := 0; i < MaxScreenshots; i++ {
		eight = append(eight, shot(i))
	}
	ok := mediaRecord()
	ok.Media = append(append([]Media{}, eight...), videoOf("walk.webm", testWebM()))
	if err := ok.Validate(); err != nil {
		t.Fatalf("8 screenshots and a video: %v", err)
	}
	for name, c := range map[string]struct {
		media  []Media
		reason string
	}{
		"nine screenshots": {append(append([]Media{}, eight...), shot(9)), ReasonInvalid},
		"two videos":       {[]Media{videoOf("a.webm", testWebM()), videoOf("b.webm", append(testWebM(), 1))}, ReasonInvalid},
		"oversize shot": {[]Media{func() Media {
			m := shot(0)
			m.Bytes = MaxScreenshotBytes + 1
			return m
		}()}, ReasonBlobTooLarge},
		"oversize video": {[]Media{func() Media {
			m := videoOf("w.webm", testWebM())
			m.Bytes = MaxVideoBytes + 1
			return m
		}()}, ReasonBlobTooLarge},
		"a path name":   {[]Media{func() Media { m := shot(0); m.Name = "../x.png"; return m }()}, ReasonInvalid},
		"a nested name": {[]Media{func() Media { m := shot(0); m.Name = "a/x.png"; return m }()}, ReasonInvalid},
		"a duplicate":   {[]Media{shot(0), shot(0)}, ReasonInvalid},
		"png as video":  {[]Media{func() Media { m := shot(0); m.Kind = MediaKindVideo; return m }()}, ReasonInvalid},
		"webm as shot": {[]Media{func() Media {
			m := videoOf("w.webm", testWebM())
			m.Kind = MediaKindScreenshot
			m.Width, m.Height = 8, 6
			return m
		}()}, ReasonInvalid},
		"a huge shot":     {[]Media{func() Media { m := shot(0); m.Height = MaxScreenshotSide + 1; return m }()}, ReasonInvalid},
		"a sizeless png":  {[]Media{func() Media { m := shot(0); m.Width = 0; return m }()}, ReasonInvalid},
		"an unknown kind": {[]Media{func() Media { m := shot(0); m.Kind = "gif"; return m }()}, ReasonInvalid},
		"a bad digest":    {[]Media{func() Media { m := shot(0); m.SHA256 = "x"; return m }()}, ReasonInvalid},
	} {
		r := mediaRecord()
		r.Media = c.media
		if err := r.Validate(); ReasonOf(err) != c.reason {
			t.Fatalf("%s: error %v, want %s", name, err, c.reason)
		}
	}
}

func TestVerifyMediaRefusesOtherBytes(t *testing.T) {
	pngBytes := testPNG(t, 8, 6)
	shot := shotOf("home.png", pngBytes, 8, 6)
	if err := VerifyMedia(shot, pngBytes); err != nil {
		t.Fatal(err)
	}
	webm := testWebM()
	video := videoOf("walk.webm", webm)
	if err := VerifyMedia(video, webm); err != nil {
		t.Fatal(err)
	}
	other := testPNG(t, 6, 8)
	notPNG := append([]byte("GIF89a"), make([]byte, 40)...)
	for name, c := range map[string]struct {
		item Media
		raw  []byte
	}{
		"other bytes":        {shot, other},
		"truncated":          {shot, pngBytes[:len(pngBytes)-1]},
		"other size":         {shotOf("home.png", other, 8, 6), other},
		"not a png":          {shotOf("home.png", notPNG, 8, 6), notPNG},
		"png as a video":     {videoOf("walk.webm", pngBytes), pngBytes},
		"matroska, not webm": {videoOf("walk.webm", bytes.Replace(webm, []byte("webm"), []byte("mkvx"), 1)), bytes.Replace(webm, []byte("webm"), []byte("mkvx"), 1)},
	} {
		if err := VerifyMedia(c.item, c.raw); err == nil {
			t.Fatalf("%s: verified", name)
		}
	}
}

// The app store lists no release without a screenshot: none, or a video
// alone, is refused with screenshot_required; Validate still decodes it, so
// records published before the rule stay readable.
func TestRequireScreenshotRefusesNoneAndVideoOnly(t *testing.T) {
	shot := Media{Kind: MediaKindScreenshot, Name: "a.png"}
	video := Media{Kind: MediaKindVideo, Name: "a.webm"}
	for name, media := range map[string][]Media{"none": nil, "video only": {video}} {
		if err := RequireScreenshot(Record{Media: media}); ReasonOf(err) != ReasonScreenshotRequired {
			t.Fatalf("%s = %v, want screenshot_required", name, err)
		}
	}
	if err := RequireScreenshot(Record{Media: []Media{video, shot}}); err != nil {
		t.Fatalf("a screenshot and a video = %v", err)
	}
}

// A manifest may hold a multi-line display description; the card's one-line
// form is derived from it by the host.
func TestManifestDescriptionIsOneBoundedLine(t *testing.T) {
	raw := []byte(`{"schema":"choir.app-package/v1","name":"reader","version":"1.0.0","origin":"first_party",` +
		`"display":{"title":"Reader","icon":"R","description":"Reads\n  the   news\r\n today"},` +
		`"app":{"entry":"App.svelte","abi":"choir.app-frontend/svelte-inline/v1","aliases":[],"launcher":{"menu":true},"window":{"singleton":false},"auth":{"preview":"public-preview"},"theme":{}},` +
		`"opens":[],"kinds":[],"uses_kinds":[],"desk":null,"backend":null}`)
	if got := ManifestDescription(raw); got != "Reads the news today" {
		t.Fatalf("description = %q", got)
	}
	// A description at the manifest's own bound still passes the record's.
	edge := strings.Repeat("a", 198) + `\n` + "b"
	got := ManifestDescription([]byte(strings.Replace(string(raw), "Reads\\n  the   news\\r\\n today", edge, 1)))
	if n := len([]rune(got)); n == 0 || n > 200 || strings.ContainsAny(got, "\r\n") {
		t.Fatalf("edge description is %d runes: %q", n, got)
	}
}
