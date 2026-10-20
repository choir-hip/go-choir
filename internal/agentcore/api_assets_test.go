package agentcore

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/imageblob"
)

// GET /api/assets/sha256/<hex> (docs/reports/texture-media-and-autoradio-
// design-2026-10-20.md section 4): the owner's browser reads a content-
// addressed asset, with range requests, the stored kind's Content-Type,
// nosniff and immutable caching, re-hashing on every read. Ways it could
// fail, written before the route:
//   - it serves to a caller who is not the owner;
//   - a digest that is not 64 lowercase hex reaches the filesystem (a
//     traversal), or a path with extra segments is accepted;
//   - bytes that no longer hash to their name are served;
//   - a stored non-image is served under a guessed or attacker-chosen type
//     (the browser would sniff and run it);
//   - a range request returns the whole body, or an unsatisfiable range 200s;
//   - the response is cached as mutable, or by a shared cache;
//   - the route accepts a write.

func assetPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 40, 30))
	for x := 5; x < 20; x++ {
		img.Set(x, 10, color.RGBA{R: 200, A: 255})
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func assetRequest(handler *APIHandler, method, path, user string, headers map[string]string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, nil)
	if user != "" {
		request.Header.Set("X-Authenticated-User", user)
	}
	for k, v := range headers {
		request.Header.Set(k, v)
	}
	response := httptest.NewRecorder()
	handler.HandleAssets(response, request)
	return response
}

func TestAssetRouteServesAStoredImageToItsOwner(t *testing.T) {
	runtime, _ := testRuntime(t)
	handler := &APIHandler{rt: runtime}
	data := assetPNG(t)
	ref, _, err := runtime.imageStore().Put(data)
	if err != nil {
		t.Fatal(err)
	}
	digest := strings.TrimPrefix(ref, "artifact:sha256:")
	path := "/api/assets/sha256/" + digest

	response := assetRequest(handler, http.MethodGet, path, "owner", nil)
	if response.Code != http.StatusOK || !bytes.Equal(response.Body.Bytes(), data) {
		t.Fatalf("GET status=%d, %d bytes", response.Code, response.Body.Len())
	}
	header := response.Header()
	if header.Get("Content-Type") != "image/png" {
		t.Errorf("Content-Type = %q", header.Get("Content-Type"))
	}
	if header.Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q", header.Get("X-Content-Type-Options"))
	}
	cache := header.Get("Cache-Control")
	if !strings.Contains(cache, "immutable") || !strings.Contains(cache, "private") || !strings.Contains(cache, "max-age=31536000") {
		t.Errorf("Cache-Control = %q, want private, immutable, one year", cache)
	}
	if etag := header.Get("ETag"); etag != `"sha256-`+digest+`"` {
		t.Errorf("ETag = %q", etag)
	}
	if header.Get("Accept-Ranges") != "bytes" {
		t.Errorf("Accept-Ranges = %q", header.Get("Accept-Ranges"))
	}
	if !strings.Contains(header.Get("Content-Security-Policy"), "sandbox") {
		t.Errorf("Content-Security-Policy = %q, want a sandbox", header.Get("Content-Security-Policy"))
	}

	// Conditional request: the digest is the validator.
	cached := assetRequest(handler, http.MethodGet, path, "owner", map[string]string{"If-None-Match": `"sha256-` + digest + `"`})
	if cached.Code != http.StatusNotModified || cached.Body.Len() != 0 {
		t.Errorf("If-None-Match status=%d, %d bytes", cached.Code, cached.Body.Len())
	}

	// HEAD answers the headers and no body.
	head := assetRequest(handler, http.MethodHead, path, "owner", nil)
	if head.Code != http.StatusOK || head.Body.Len() != 0 || head.Header().Get("Content-Length") != strconv.Itoa(len(data)) {
		t.Errorf("HEAD status=%d, %d bytes, Content-Length=%q", head.Code, head.Body.Len(), head.Header().Get("Content-Length"))
	}
}

func TestAssetRouteAnswersRangeRequests(t *testing.T) {
	runtime, _ := testRuntime(t)
	handler := &APIHandler{rt: runtime}
	data := assetPNG(t)
	ref, _, _ := runtime.imageStore().Put(data)
	path := "/api/assets/sha256/" + strings.TrimPrefix(ref, "artifact:sha256:")

	part := assetRequest(handler, http.MethodGet, path, "owner", map[string]string{"Range": "bytes=2-9"})
	if part.Code != http.StatusPartialContent || !bytes.Equal(part.Body.Bytes(), data[2:10]) {
		t.Fatalf("range status=%d body=%x", part.Code, part.Body.Bytes())
	}
	if want := "bytes 2-9/" + strconv.Itoa(len(data)); part.Header().Get("Content-Range") != want {
		t.Errorf("Content-Range = %q, want %q", part.Header().Get("Content-Range"), want)
	}
	tail := assetRequest(handler, http.MethodGet, path, "owner", map[string]string{"Range": "bytes=-7"})
	if tail.Code != http.StatusPartialContent || !bytes.Equal(tail.Body.Bytes(), data[len(data)-7:]) {
		t.Fatalf("suffix range status=%d", tail.Code)
	}
	past := assetRequest(handler, http.MethodGet, path, "owner", map[string]string{"Range": "bytes=" + strconv.Itoa(len(data)+5) + "-"})
	if past.Code != http.StatusRequestedRangeNotSatisfiable {
		t.Errorf("unsatisfiable range status=%d, want 416", past.Code)
	}
}

func TestAssetRouteRefusesWhatItShouldNotServe(t *testing.T) {
	runtime, _ := testRuntime(t)
	handler := &APIHandler{rt: runtime}
	data := assetPNG(t)
	ref, _, _ := runtime.imageStore().Put(data)
	digest := strings.TrimPrefix(ref, "artifact:sha256:")

	if got := assetRequest(handler, http.MethodGet, "/api/assets/sha256/"+digest, "", nil).Code; got != http.StatusUnauthorized {
		t.Errorf("unauthenticated status=%d, want 401", got)
	}
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		if got := assetRequest(handler, method, "/api/assets/sha256/"+digest, "owner", nil).Code; got != http.StatusMethodNotAllowed {
			t.Errorf("%s status=%d, want 405", method, got)
		}
	}
	notFound := []string{
		"/api/assets/sha256/" + strings.ToUpper(digest),
		"/api/assets/sha256/" + digest[:63],
		"/api/assets/sha256/" + digest + "0",
		"/api/assets/sha256/" + digest + "/extra",
		"/api/assets/sha256/" + digest + "/",
		"/api/assets/sha256/../../etc/passwd",
		"/api/assets/sha256/%2e%2e%2f%2e%2e%2fetc%2fpasswd",
		"/api/assets/sha256/",
		"/api/assets/sha1/" + digest,
		"/api/assets/" + digest,
		"/api/assets/sha256/" + strings.Repeat("0", 64),
	}
	for _, path := range notFound {
		if got := assetRequest(handler, http.MethodGet, path, "owner", nil); got.Code != http.StatusNotFound || bytes.Contains(got.Body.Bytes(), []byte("PNG")) {
			t.Errorf("GET %s status=%d, want 404", path, got.Code)
		}
	}
}

func TestAssetRouteNeverServesTamperedOrUnsupportedBytes(t *testing.T) {
	runtime, _ := testRuntime(t)
	handler := &APIHandler{rt: runtime}
	data := assetPNG(t)
	ref, _, _ := runtime.imageStore().Put(data)
	digest := strings.TrimPrefix(ref, "artifact:sha256:")
	dir := imageblob.DirForStorePath(runtime.cfg.StorePath)

	// A blob whose bytes were swapped after it was stored.
	if err := os.WriteFile(filepath.Join(dir, "sha256", digest), []byte("<script>alert(1)</script>"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := assetRequest(handler, http.MethodGet, "/api/assets/sha256/"+digest, "owner", nil)
	if got.Code != http.StatusInternalServerError || strings.Contains(got.Body.String(), "script") {
		t.Errorf("tampered blob status=%d body=%q", got.Code, got.Body.String())
	}
	if strings.Contains(got.Body.String(), dir) {
		t.Error("the error leaks the store path")
	}

	// Bytes that hash right but are no served kind (a typed TODO: audio and
	// video) are refused, never sniffed into a type.
	other := []byte("<html><script>alert(1)</script></html>")
	otherDigest := sha256HexForTest(other)
	if err := os.WriteFile(filepath.Join(dir, "sha256", otherDigest), other, 0o600); err != nil {
		t.Fatal(err)
	}
	got = assetRequest(handler, http.MethodGet, "/api/assets/sha256/"+otherDigest, "owner", nil)
	if got.Code != http.StatusUnsupportedMediaType || strings.Contains(got.Body.String(), "script") {
		t.Errorf("unsupported kind status=%d body=%q", got.Code, got.Body.String())
	}
}

func sha256HexForTest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
