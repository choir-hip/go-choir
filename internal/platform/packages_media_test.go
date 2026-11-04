package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
)

// Release media at the host (docs/problems/library-releases-have-no-screenshots-2026-11-04.md,
// design 3 and 4), written before the code. Failure modes pinned:
//
//	H1  a release is listed while a media item it names is absent from the
//	    store, or its stored bytes are not the item (another digest, a PNG
//	    of another size, not a WebM);
//	H2  a media upload over the cap lands, or a patch-sized cap blocks a
//	    video;
//	H3  media is served for a digest the record does not name, for a
//	    release the reader may not see (a withdrawn one to anyone but its
//	    publisher), or to a guest;
//	H4  a release with no media fails to publish, or its view has no media
//	    array (the gallery's "no screenshots" state needs an empty one);
//	H5  the view's and the catalog entry's media URLs are not the proxy's
//	    public route.

func hostPNG(t *testing.T, w, h int, seed uint8) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x) + seed, uint8(y), 120, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func hostWebM() []byte {
	head := []byte{0x1a, 0x45, 0xdf, 0xa3, 0x9f, 0x42, 0x86, 0x81, 0x01, 0x42, 0xf7, 0x81, 0x01, 0x42, 0xf2, 0x81, 0x04, 0x42, 0xf3, 0x81, 0x08, 0x42, 0x82, 0x84, 'w', 'e', 'b', 'm', 0x42, 0x87, 0x81, 0x04, 0x42, 0x85, 0x81, 0x02}
	return append(head, bytes.Repeat([]byte{5}, 256)...)
}

// withMedia re-seals b naming the given items.
func (w *pkgWorld) withMedia(b built, items ...pkgrelease.Media) built {
	b.record.Media = items
	return w.seal(b.record, b.blobs)
}

func mediaShot(name string, raw []byte, w, h int) pkgrelease.Media {
	return pkgrelease.Media{Kind: pkgrelease.MediaKindScreenshot, Name: name, MediaType: pkgrelease.MediaTypePNG, SHA256: computerevent.DigestBytes(raw), Bytes: len(raw), Width: w, Height: h}
}

func mediaVideo(raw []byte) pkgrelease.Media {
	return pkgrelease.Media{Kind: pkgrelease.MediaKindVideo, Name: "walk.webm", MediaType: pkgrelease.MediaTypeWebM, SHA256: computerevent.DigestBytes(raw), Bytes: len(raw)}
}

func TestPackageMediaPublishesAndServesPublicly(t *testing.T) {
	w := newPkgWorld(t)
	home, phone, video := hostPNG(t, 128, 80, 1), hostPNG(t, 39, 84, 2), hostWebM()
	b := w.withMedia(w.build("<p>v2</p>\n", "owner-a", nil), mediaShot("home.png", home, 128, 80), mediaShot("phone.png", phone, 39, 84), mediaVideo(video))

	// H1: nothing uploaded, the release is not listed.
	wantPkgRefusal(t, w.publish(b), http.StatusConflict, pkgrelease.ReasonBlobMismatch)
	for _, raw := range [][]byte{home, phone, video} {
		if rec := w.do(pkgCall{method: http.MethodPut, path: packagesRoute + "/media/" + computerevent.DigestBytes(raw), internal: true, body: raw}); rec.Code != http.StatusCreated {
			t.Fatalf("media upload = %d %s", rec.Code, rec.Body.String())
		}
	}
	view := w.published(b)
	if len(view.Media) != 3 {
		t.Fatalf("view media = %+v", view.Media)
	}
	for i, m := range view.Media {
		want := "/api/catalog/releases/" + b.id + "/media/" + b.record.Media[i].SHA256
		if m.URL != want || m.SHA256 != b.record.Media[i].SHA256 || m.Kind != b.record.Media[i].Kind || m.MediaType != b.record.Media[i].MediaType {
			t.Fatalf("H5 media %d = %+v, want url %s", i, m, want)
		}
	}
	if view.Media[0].Width != 128 || view.Media[0].Height != 80 || view.Media[2].Width != 0 {
		t.Fatalf("sizes %+v", view.Media)
	}

	// H3: any signed-in owner reads the bytes, with the record's type.
	for _, reader := range []string{"owner-a", "owner-b"} {
		for i, raw := range [][]byte{home, phone, video} {
			rec := w.read(packagesRoute + "/releases/" + b.id + "/media/" + computerevent.DigestBytes(raw) + "?owner_id=" + reader)
			if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), raw) || rec.Header().Get("Content-Type") != b.record.Media[i].MediaType ||
				rec.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Fatalf("%s media %d = %d %q", reader, i, rec.Code, rec.Header().Get("Content-Type"))
			}
		}
	}
	// H3: a digest the record does not name, even one in the store.
	other := hostPNG(t, 10, 10, 9)
	w.do(pkgCall{method: http.MethodPut, path: packagesRoute + "/media/" + computerevent.DigestBytes(other), internal: true, body: other})
	for _, path := range []string{
		"/releases/" + b.id + "/media/" + computerevent.DigestBytes(other) + "?owner_id=owner-a",
		"/releases/" + b.id + "/media/" + computerevent.DigestBytes(b.blobs.TreePatch) + "?owner_id=owner-a",
		"/releases/" + strings.Repeat("0", 64) + "/media/" + computerevent.DigestBytes(home) + "?owner_id=owner-a",
		"/releases/" + b.id + "/media/not-a-digest?owner_id=owner-a",
	} {
		if rec := w.read(packagesRoute + path); rec.Code != http.StatusNotFound {
			t.Fatalf("%s = %d, want 404", path, rec.Code)
		}
	}
	if rec := w.read(packagesRoute + "/releases/" + b.id + "/media/" + computerevent.DigestBytes(home)); rec.Code != http.StatusBadRequest {
		t.Fatalf("a media read naming no owner = %d, want 400", rec.Code)
	}
	if rec := w.do(pkgCall{method: http.MethodGet, path: packagesRoute + "/releases/" + b.id + "/media/" + computerevent.DigestBytes(home) + "?owner_id=owner-a", remote: guestTap}); rec.Code != http.StatusForbidden {
		t.Fatalf("a guest media read = %d, want 403", rec.Code)
	}
	if rec := w.do(pkgCall{method: http.MethodPut, path: packagesRoute + "/media/" + computerevent.DigestBytes(home), remote: guestTap, body: home}); rec.Code != http.StatusForbidden {
		t.Fatalf("a guest media upload = %d, want 403", rec.Code)
	}

	// H5: the catalog entry carries the head's media.
	var catalog struct {
		Packages []PackageCatalogEntry `json:"packages"`
	}
	if rec := w.read(packagesRoute + "/catalog?owner_id=owner-b"); rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &catalog) != nil || len(catalog.Packages) != 1 {
		t.Fatalf("catalog = %d %s", rec.Code, rec.Body.String())
	}
	if head := catalog.Packages[0].HeadMedia; len(head) != 3 || head[0].URL != view.Media[0].URL {
		t.Fatalf("head_media = %+v", head)
	}
	// History and the release read carry it too.
	if rec := w.read(packagesRoute + "/catalog/reader/releases?owner_id=owner-b&track=owner-a"); !strings.Contains(rec.Body.String(), view.Media[0].URL) {
		t.Fatalf("history = %s", rec.Body.String())
	}
	if rec := w.read(packagesRoute + "/releases/" + b.id + "?owner_id=owner-b"); !strings.Contains(rec.Body.String(), view.Media[2].URL) {
		t.Fatalf("release read = %s", rec.Body.String())
	}

	// H3: withdrawn, the media is its publisher's alone.
	withdrawn := view.Listing
	prev, err := view.Listing.Digest()
	if err != nil {
		t.Fatal(err)
	}
	withdrawn.Version, withdrawn.PrevVersionDigest = 2, prev
	withdrawn.State, withdrawn.StateReason, withdrawn.StateActor = pkgrelease.StateWithdrawn, "withdrawn", "owner_act:test"
	obj, err := listingObject("owner-a", "computer-a1", withdrawn, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.h.releaseStore().InsertObjectIfAbsent(context.Background(), obj); err != nil {
		t.Fatal(err)
	}
	media := "/releases/" + b.id + "/media/" + computerevent.DigestBytes(home)
	if rec := w.read(packagesRoute + media + "?owner_id=owner-b"); rec.Code != http.StatusNotFound {
		t.Fatalf("another owner's read of withdrawn media = %d, want 404", rec.Code)
	}
	if rec := w.read(packagesRoute + media + "?owner_id=owner-a"); rec.Code != http.StatusOK {
		t.Fatalf("the publisher's read of withdrawn media = %d", rec.Code)
	}
}

func TestPackageMediaThatIsNotTheItemIsRefused(t *testing.T) {
	w := newPkgWorld(t)
	home := hostPNG(t, 128, 80, 1)
	// The stored bytes hash right but are a PNG of another size than the
	// record states.
	wrongSize := w.withMedia(w.build("<p>v2</p>\n", "owner-a", nil), mediaShot("home.png", home, 64, 80))
	w.do(pkgCall{method: http.MethodPut, path: packagesRoute + "/media/" + computerevent.DigestBytes(home), internal: true, body: home})
	wantPkgRefusal(t, w.publish(wrongSize), http.StatusConflict, pkgrelease.ReasonBlobMismatch)
	// A "video" that is a PNG.
	notWebM := w.withMedia(w.build("<p>v3</p>\n", "owner-a", nil), pkgrelease.Media{Kind: pkgrelease.MediaKindVideo, Name: "walk.webm", MediaType: pkgrelease.MediaTypeWebM, SHA256: computerevent.DigestBytes(home), Bytes: len(home)})
	wantPkgRefusal(t, w.publish(notWebM), http.StatusConflict, pkgrelease.ReasonBlobMismatch)
}

func TestPackageMediaUploadCap(t *testing.T) {
	w := newPkgWorld(t)
	// H2: a video over the patch cap but within the media cap lands.
	big := append(hostWebM(), bytes.Repeat([]byte{1}, pkgrelease.MaxBlobBytes+1)...)
	if rec := w.do(pkgCall{method: http.MethodPut, path: packagesRoute + "/media/" + computerevent.DigestBytes(big), internal: true, body: big}); rec.Code != http.StatusCreated {
		t.Fatalf("a 2 MiB+ video upload = %d %s", rec.Code, rec.Body.String())
	}
	over := make([]byte, pkgrelease.MaxMediaBytes+1)
	rec := w.do(pkgCall{method: http.MethodPut, path: packagesRoute + "/media/" + computerevent.DigestBytes(over), internal: true, body: over})
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("an oversize media upload = %d, want 413", rec.Code)
	}
	bad := hostPNG(t, 4, 4, 1)
	if rec := w.do(pkgCall{method: http.MethodPut, path: packagesRoute + "/media/" + strings.Repeat("0", 64), internal: true, body: bad}); rec.Code != http.StatusBadRequest {
		t.Fatalf("a mismatched media upload = %d, want 400", rec.Code)
	}
}

func TestPackageWithoutMediaHasAnEmptyMediaArray(t *testing.T) {
	w := newPkgWorld(t)
	b := w.build("<p>v2</p>\n", "owner-a", nil)
	rec := w.publish(b)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"media":[]`) {
		t.Fatalf("H4 publish without media = %d %s", rec.Code, rec.Body.String())
	}
	if rec := w.read(packagesRoute + "/catalog?owner_id=owner-b"); !strings.Contains(rec.Body.String(), `"head_media":[]`) {
		t.Fatalf("H4 catalog without media = %s", rec.Body.String())
	}
}
