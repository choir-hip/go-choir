package proxy

// Failure list (written before the code), for the 1 MiB read bound on corpusd's
// answers (docs/problems/staging-resolve-502-on-a-live-story-2026-11-19.md):
//
//  F1. A valid resolve bundle larger than 1 MiB makes the handler answer 502
//      ("unexpected end of JSON input" in the log). It must arrive whole.
//  F2. A 3 MiB bundle must reach the client byte for byte, with its length.
//  F3. A resolve of 1 MiB or more logs its size, so growth is seen.
//  F4. Where the proxy must decode (getPlatformJSON, the owned read, the
//      proposal post, the publish post), a body over the deliberate bound
//      answers an explicit "too large" error carrying the upstream size, and
//      the log says so; never "unexpected end of JSON input".
//  F6. The relay's time bound is the server's write budget, not corpusd's 30 s:
//      a body trickled past the shared client's timeout arrives whole.
//  F7. A relay that breaks mid-body is a transport error for the client, not a
//      clean (chunked) 200 with truncated JSON.
//  F8. A 2xx with an empty body is a logged 502, as the old decode made it.
//  F9. A known Content-Length over the bound is refused without reading.
//  F10. The owned-publication read and the proposal post answer the explicit
//      error through their own handlers.
//  F5. The nav view and the 404 path behave as before; an upstream 5xx still
//      answers 502.

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/platform"
)

func resolveSizeHandler(t *testing.T, corpusd http.HandlerFunc) *Handler {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	srv := httptest.NewServer(corpusd)
	t.Cleanup(srv.Close)
	h, err := NewHandler(&Config{AllowDirectAutoputerForTests: true, Port: "0",
		ComputerURL:       "http://127.0.0.1:1",
		AuthPublicKeyPath: "/unused/in/test",
		CorpusdURL:        srv.URL}, pub)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	return h
}

// lockedBuf is a log sink the handler goroutines write and the test reads.
type lockedBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuf) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}
func (l *lockedBuf) String() string { l.mu.Lock(); defer l.mu.Unlock(); return l.b.String() }

func captureLog(t *testing.T) *lockedBuf {
	t.Helper()
	buf := &lockedBuf{}
	old := log.Writer()
	log.SetOutput(buf)
	t.Cleanup(func() { log.SetOutput(old) })
	return buf
}

func bigBundleJSON(t *testing.T, size int) []byte {
	t.Helper()
	b := platform.PublicationBundle{
		Route:       platform.PublicationRoute{Path: "/pub/texture/big", State: "active"},
		Publication: platform.PublicationSummary{ID: "pub-big", Title: "Big"},
		Artifact:    platform.PublicationArtifact{Content: strings.Repeat("x", size)},
	}
	data, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestResolveRelaysBundleOverOneMiBByteForByte(t *testing.T) { // F1, F2
	want := bigBundleJSON(t, 3<<20)
	h := resolveSizeHandler(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(want)
	})
	w := httptest.NewRecorder()
	h.HandleAPI(w, httptest.NewRequest(http.MethodGet, "/api/platform/publications/resolve?route=/pub/texture/big", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d, want 200 (body starts %.120s)", w.Code, w.Body.String())
	}
	if !bytes.Equal(w.Body.Bytes(), want) {
		t.Fatalf("body differs: got %d bytes, want %d", w.Body.Len(), len(want))
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("content type %q", ct)
	}
	var back platform.PublicationBundle
	if err := json.Unmarshal(w.Body.Bytes(), &back); err != nil || len(back.Artifact.Content) != 3<<20 {
		t.Fatalf("decode relayed bundle: %v", err)
	}
}

func TestResolveLogsBundleSizeAtOneMiB(t *testing.T) { // F3
	logs := captureLog(t)
	big := bigBundleJSON(t, 2<<20)
	small := bigBundleJSON(t, 1000)
	h := resolveSizeHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("route") == "/small" {
			_, _ = w.Write(small)
			return
		}
		_, _ = w.Write(big)
	})
	h.HandleAPI(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/platform/publications/resolve?route=/small", nil))
	if strings.Contains(logs.String(), "resolve bundle size") {
		t.Fatalf("small bundle logged: %s", logs.String())
	}
	h.HandleAPI(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/platform/publications/resolve?route=/big", nil))
	if !strings.Contains(logs.String(), "resolve bundle size") || !strings.Contains(logs.String(), `route="/big"`) {
		t.Fatalf("big bundle not logged: %q", logs.String())
	}
}

func TestResolveNavAndNotFoundAndUpstreamError(t *testing.T) { // F5
	logs := captureLog(t)
	var gotView string
	h := resolveSizeHandler(t, func(w http.ResponseWriter, r *http.Request) {
		gotView = r.URL.Query().Get("view")
		switch r.URL.Query().Get("route") {
		case "/gone":
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		case "/boom":
			http.Error(w, "kaput", http.StatusInternalServerError)
		default:
			_ = json.NewEncoder(w).Encode(platform.PublicationBundle{Route: platform.PublicationRoute{Path: "/n"}})
		}
	})
	do := func(q string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		h.HandleAPI(w, httptest.NewRequest(http.MethodGet, "/api/platform/publications/resolve?route="+q, nil))
		return w
	}
	if w := do("/n&view=nav"); w.Code != 200 || gotView != "nav" {
		t.Fatalf("nav: %d view=%q", w.Code, gotView)
	}
	if w := do("/gone"); w.Code != http.StatusNotFound {
		t.Fatalf("404 path: %d", w.Code)
	}
	w := do("/boom")
	if w.Code != http.StatusBadGateway || !strings.Contains(w.Body.String(), "failed to resolve publication") {
		t.Fatalf("5xx: %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(logs.String(), "status 500") {
		t.Fatalf("upstream 500 not logged: %q", logs.String())
	}
}

func TestReadPlatformBodyOverBoundIsExplicit(t *testing.T) { // F4
	old := maxPlatformDecodeBytes
	maxPlatformDecodeBytes = 2048
	t.Cleanup(func() { maxPlatformDecodeBytes = old })

	if got, err := readPlatformBody(strings.NewReader(strings.Repeat("a", 2048)), -1); err != nil || len(got) != 2048 {
		t.Fatalf("at the bound: %d %v", len(got), err)
	}
	_, err := readPlatformBody(strings.NewReader(strings.Repeat("a", 5000)), 5000)
	tooLarge, ok := err.(*platformBodyTooLargeError)
	if !ok {
		t.Fatalf("want *platformBodyTooLargeError, got %T %v", err, err)
	}
	if !strings.Contains(err.Error(), "too large") || !strings.Contains(err.Error(), "5000") || tooLarge.limit != 2048 {
		t.Fatalf("message: %v", err)
	}
	// unknown upstream length: says "more than the bound" rather than guessing.
	_, err = readPlatformBody(io.LimitReader(strings.NewReader(strings.Repeat("a", 9000)), 9000), -1)
	if err == nil || !strings.Contains(err.Error(), "too large") || strings.Contains(err.Error(), "unexpected end") {
		t.Fatalf("unknown length: %v", err)
	}
}

func TestDecodeSitesAnswerTooLargeNotTruncatedJSON(t *testing.T) { // F4
	old := maxPlatformDecodeBytes
	maxPlatformDecodeBytes = 2048
	t.Cleanup(func() { maxPlatformDecodeBytes = old })
	logs := captureLog(t)
	h := resolveSizeHandler(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(bigBundleJSON(t, 6000))
	})
	// getPlatformJSON through the front list.
	w := httptest.NewRecorder()
	h.HandleAPI(w, httptest.NewRequest(http.MethodGet, "/api/platform/publications/front", nil))
	if w.Code != http.StatusBadGateway {
		t.Fatalf("front status %d", w.Code)
	}
	if !strings.Contains(logs.String(), "too large") || strings.Contains(logs.String(), "unexpected end of JSON") {
		t.Fatalf("front log: %q", logs.String())
	}
	// postPlatformPublication.
	_, _, err := h.postPlatformPublication(httptest.NewRequest(http.MethodPost, "/x", nil), platform.PublishTextureRequest{})
	if err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("publish post: %v", err)
	}
}

func serveHandler(t *testing.T, h *Handler) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(h.HandleAPI))
	t.Cleanup(srv.Close)
	return srv
}

func TestResolveRelayOutlivesTheSharedCorpusdTimeout(t *testing.T) { // F6
	want := bigBundleJSON(t, 3000)
	h := resolveSizeHandler(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fl := w.(http.Flusher)
		for i := 0; i < len(want); i += 1000 {
			_, _ = w.Write(want[i:min(i+1000, len(want))])
			fl.Flush()
			time.Sleep(150 * time.Millisecond)
		}
	})
	h.corpusd.Timeout = 300 * time.Millisecond // the 30 s of production, shortened
	h.corpusdRelay.Timeout = 10 * time.Second
	srv := serveHandler(t, h)
	resp, err := http.Get(srv.URL + "/api/platform/publications/resolve?route=/slow")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	got, err := io.ReadAll(resp.Body)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("slow body: %d of %d bytes, err %v", len(got), len(want), err)
	}
}

func TestResolveMidBodyBreakIsATransportError(t *testing.T) { // F7
	logs := captureLog(t)
	h := resolveSizeHandler(t, func(w http.ResponseWriter, r *http.Request) {
		conn, buf, _ := w.(http.Hijacker).Hijack()
		defer conn.Close()
		chunk := strings.Repeat("x", 4096)
		_, _ = buf.WriteString("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nTransfer-Encoding: chunked\r\n\r\n")
		_, _ = buf.WriteString(strconv.FormatInt(int64(len(chunk)), 16) + "\r\n" + chunk + "\r\n")
		_ = buf.Flush() // then close with no terminating chunk
	})
	srv := serveHandler(t, h)
	resp, err := http.Get(srv.URL + "/api/platform/publications/resolve?route=/broken")
	if err == nil {
		defer resp.Body.Close()
		_, err = io.ReadAll(resp.Body)
	}
	if err == nil {
		t.Fatal("client saw a clean response for a body that broke mid-stream")
	}
	if !strings.Contains(logs.String(), "relay broke after") {
		t.Fatalf("break not logged: %q", logs.String())
	}
}

func TestResolveEmptyTwoXXIsA502(t *testing.T) { // F8
	logs := captureLog(t)
	h := resolveSizeHandler(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	w := httptest.NewRecorder()
	h.HandleAPI(w, httptest.NewRequest(http.MethodGet, "/api/platform/publications/resolve?route=/empty", nil))
	if w.Code != http.StatusBadGateway || !strings.Contains(logs.String(), "empty or unreadable") {
		t.Fatalf("empty 200: status %d log %q", w.Code, logs.String())
	}
}

type neverRead struct{ t *testing.T }

func (n neverRead) Read([]byte) (int, error) {
	n.t.Fatal("body read despite a known length over the bound")
	return 0, io.EOF
}

func TestReadPlatformBodyRefusesKnownLengthWithoutReading(t *testing.T) { // F9
	old := maxPlatformDecodeBytes
	maxPlatformDecodeBytes = 2048
	t.Cleanup(func() { maxPlatformDecodeBytes = old })
	_, err := readPlatformBody(neverRead{t}, 5000)
	if _, ok := err.(*platformBodyTooLargeError); !ok {
		t.Fatalf("got %v", err)
	}
}

func TestOwnedReadAndProposalPostAnswerTooLarge(t *testing.T) { // F10
	old := maxPlatformDecodeBytes
	maxPlatformDecodeBytes = 2048
	t.Cleanup(func() { maxPlatformDecodeBytes = old })
	logs := captureLog(t)
	pub, priv, _ := ed25519.GenerateKey(nil)
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(bigBundleJSON(t, 6000))
	}))
	defer corpusd.Close()
	autoputer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/texture/documents/doc-1":
			_ = json.NewEncoder(w).Encode(autoputerTextureDocument{DocID: "doc-1", OwnerID: "reader-1", Title: "d", CurrentRevisionID: "rev-1"})
		case "/api/texture/revisions/rev-1":
			_ = json.NewEncoder(w).Encode(autoputerTextureRevision{RevisionID: "rev-1", DocID: "doc-1", OwnerID: "reader-1", Content: "c"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer autoputer.Close()
	h, err := NewHandler(&Config{AllowDirectAutoputerForTests: true, Port: "0",
		ComputerURL: autoputer.URL, AuthPublicKeyPath: "/unused/in/test", CorpusdURL: corpusd.URL}, pub)
	if err != nil {
		t.Fatal(err)
	}
	cookie := &http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(priv, "reader-1")}

	req := httptest.NewRequest(http.MethodGet, "https://choir.news/api/platform/texture/publications?doc_id=doc-1", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	h.HandleTexturePublication(w, req)
	if w.Code != http.StatusBadGateway || !strings.Contains(logs.String(), "owned publication read: corpusd response too large") {
		t.Fatalf("owned read: %d log %q", w.Code, logs.String())
	}

	body := `{"doc_id":"doc-1","revision_id":"rev-1"}`
	req = httptest.NewRequest(http.MethodPost, "/api/platform/publications/pub-1/proposals", strings.NewReader(body))
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	h.HandleAPI(w, req)
	if w.Code != http.StatusBadGateway || !strings.Contains(logs.String(), "platform proposal post: corpusd response too large") {
		t.Fatalf("proposal post: %d %s log %q", w.Code, w.Body.String(), logs.String())
	}
	_ = fmt.Sprint
}

// The route is a public query parameter: a newline in it must not forge a log line.
func TestResolveLogLinesQuoteTheRoute(t *testing.T) {
	logs := captureLog(t)
	big := bigBundleJSON(t, 2<<20)
	h := resolveSizeHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Query().Get("route"), "/boom") {
			http.Error(w, "kaput", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(big)
	})
	for _, route := range []string{"/big\nproxy: FORGED line", "/boom\nproxy: FORGED line"} {
		w := httptest.NewRecorder()
		u := "/api/platform/publications/resolve?route=" + url.QueryEscape(route)
		h.HandleAPI(w, httptest.NewRequest(http.MethodGet, u, nil))
	}
	out := logs.String()
	if strings.Contains(out, "\nproxy: FORGED") {
		t.Fatalf("a forged log line got through:\n%s", out)
	}
	if n := len(strings.Split(strings.TrimSpace(out), "\n")); n != 2 && !strings.Contains(out, `\n`) {
		t.Fatalf("unexpected log shape (%d lines):\n%s", n, out)
	}
	if !strings.Contains(out, "bundle size: route=") {
		t.Fatalf("size line missing:\n%s", out)
	}
}
