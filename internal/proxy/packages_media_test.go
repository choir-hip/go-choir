package proxy

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
)

// Release media at the proxy (docs/problems/library-releases-have-no-screenshots-2026-11-04.md,
// design 3 and 4), written before the code. Failure modes pinned:
//
//	P1  a media item the draft names reaches corpusd without the proxy
//	    having fetched it from the publisher computer and verified it
//	    (digest, length, PNG size or WebM header), or the record is
//	    published while an item failed;
//	P2  the public media read names an owner the caller chose, relays a
//	    type corpusd did not mean (HTML from an error page served as an
//	    image), or omits nosniff;
//	P3  a release with no media makes any media call.

func proxyPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 7, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func proxyWebM() []byte {
	head := []byte{0x1a, 0x45, 0xdf, 0xa3, 0x9f, 0x42, 0x86, 0x81, 0x01, 0x42, 0xf7, 0x81, 0x01, 0x42, 0xf2, 0x81, 0x04, 0x42, 0xf3, 0x81, 0x08, 0x42, 0x82, 0x84, 'w', 'e', 'b', 'm'}
	return append(head, bytes.Repeat([]byte{9}, 128)...)
}

func TestPackagePublishCarriesTheVerifiedMedia(t *testing.T) {
	env := newPackageProxyEnv(t)
	owner := env.secret(t, "computer:self_development:approve", OwnerActsScope)
	const path = "/api/computers/computer-a/packages/releases"
	const body = `{"operation_id":"op-1","package":"reader","confirm_paths":"` + "eeee" + `"}`
	shot, video := proxyPNG(t, 40, 30), proxyWebM()
	items := []pkgrelease.Media{
		{Kind: pkgrelease.MediaKindScreenshot, Name: "home.png", MediaType: pkgrelease.MediaTypePNG, SHA256: computerevent.DigestBytes(shot), Bytes: len(shot), Width: 40, Height: 30},
		{Kind: pkgrelease.MediaKindVideo, Name: "walk.webm", MediaType: pkgrelease.MediaTypeWebM, SHA256: computerevent.DigestBytes(video), Bytes: len(video)},
	}
	env.draft = func(string) []byte {
		return packageDraftFor(t, env.user, func(r *pkgrelease.Record) { r.Media = items }, false)
	}
	reset := func() {
		env.mu.Lock()
		env.calls = nil
		env.mu.Unlock()
	}

	// P1: the publisher serves other bytes for the video: nothing is
	// written to corpusd.
	env.guestMedia = map[string][]byte{items[0].SHA256: shot, items[1].SHA256: append([]byte{}, shot...)}
	reset()
	if rec := env.send(t, http.MethodPost, path, body, owner); rec.Code != http.StatusBadGateway || strings.Contains(env.joined(), "corpusd PUT") || strings.Contains(env.joined(), "corpusd POST") {
		t.Fatalf("P1 a media item that does not verify = %d %s, calls:\n%s", rec.Code, rec.Body.String(), env.joined())
	}
	// P1: an item the publisher does not answer.
	env.guestMedia = map[string][]byte{items[0].SHA256: shot}
	reset()
	if rec := env.send(t, http.MethodPost, path, body, owner); rec.Code != http.StatusBadGateway || strings.Contains(env.joined(), "corpusd POST") {
		t.Fatalf("P1 a missing media item = %d, calls:\n%s", rec.Code, env.joined())
	}

	env.guestMedia = map[string][]byte{items[0].SHA256: shot, items[1].SHA256: video}
	reset()
	if rec := env.send(t, http.MethodPost, path, body, owner); rec.Code != http.StatusCreated {
		t.Fatalf("publish with media = %d %s", rec.Code, rec.Body.String())
	}
	calls := env.joined()
	post := strings.Index(calls, "corpusd POST")
	for _, item := range items {
		fetch := strings.Index(calls, "guest GET /api/computers/computer-a/packages/operations/op-1/media/"+item.SHA256+"? owner="+env.user)
		put := strings.Index(calls, "corpusd PUT /internal/platform/packages/media/"+item.SHA256+" internal=true")
		if fetch < 0 || put < 0 || post < 0 || put > post || fetch > put {
			t.Fatalf("P1 %s: fetch %d put %d post %d:\n%s", item.Name, fetch, put, post, calls)
		}
	}

	// P3: no media, no media call.
	env.draft = func(string) []byte { return packageDraftFor(t, env.user, nil, false) }
	reset()
	if rec := env.send(t, http.MethodPost, path, body, owner); rec.Code != http.StatusCreated || strings.Contains(env.joined(), "/media/") {
		t.Fatalf("P3 publish without media = %d, calls:\n%s", rec.Code, env.joined())
	}
}

func TestPackageMediaReadIsTheAuthenticatedOwnersAndTyped(t *testing.T) {
	env := newPackageProxyEnv(t)
	reader := env.secret(t, "computer:self_development:read")
	id, shot := strings.Repeat("a", 64), proxyPNG(t, 4, 4)
	digest := computerevent.DigestBytes(shot)
	html := strings.Repeat("b", 64)
	env.corpusdMedia = map[string][2]string{digest: {string(shot), pkgrelease.MediaTypePNG}, html: {"<script>alert(1)</script>", "text/html"}}

	rec := env.send(t, http.MethodGet, "/api/catalog/releases/"+id+"/media/"+digest+"?owner_id=evil", "", reader)
	if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), shot) || rec.Header().Get("Content-Type") != pkgrelease.MediaTypePNG ||
		rec.Header().Get("X-Content-Type-Options") != "nosniff" || !strings.Contains(rec.Header().Get("Cache-Control"), "private") {
		t.Fatalf("media read = %d %v", rec.Code, rec.Header())
	}
	if want := "corpusd GET /internal/platform/packages/releases/" + id + "/media/" + digest + " internal=true query=owner_id=" + env.user; !strings.Contains(env.joined(), want) || strings.Contains(env.joined(), "evil") {
		t.Fatalf("P2 calls %v; want %q", env.joined(), want)
	}
	// P2: a type corpusd did not mean is never relayed.
	if rec := env.send(t, http.MethodGet, "/api/catalog/releases/"+id+"/media/"+html, "", reader); rec.Code != http.StatusBadGateway || strings.Contains(rec.Body.String(), "<script>") {
		t.Fatalf("P2 an HTML answer = %d %s", rec.Code, rec.Body.String())
	}
	// corpusd's 404 is relayed as JSON.
	if rec := env.send(t, http.MethodGet, "/api/catalog/releases/"+id+"/media/"+strings.Repeat("c", 64), "", reader); rec.Code != http.StatusNotFound {
		t.Fatalf("a missing item = %d", rec.Code)
	}
	if rec := env.send(t, http.MethodGet, "/api/catalog/releases/"+id+"/media/not-a-sha", "", reader); rec.Code != http.StatusNotFound {
		t.Fatalf("a malformed digest = %d", rec.Code)
	}
	if rec := env.send(t, http.MethodGet, "/api/catalog/releases/"+id+"/media/"+digest, "", env.secret(t, "computer:self_development:approve")); rec.Code != http.StatusForbidden {
		t.Fatalf("a media read without the read scope = %d", rec.Code)
	}
}
