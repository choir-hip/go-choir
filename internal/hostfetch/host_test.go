package hostfetch

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// Ways the host side could fail, written before the code:
//  1. it dials a private, loopback, link-local (metadata), CGNAT or
//     unspecified answer, or a literal of one;
//  2. it dials one of the host's own addresses (a public address reaches
//     host listeners the guest floor drops);
//  3. it follows a redirect itself, so the next hop skips the guest's
//     re-check and the host's per-hop log;
//  4. it accepts a non-http scheme, user info, localhost or a malformed URL;
//  5. it forwards hop-by-hop or proxy headers, or the gateway bearer;
//  6. it reads an unbounded request or response body;
//  7. it returns a cut body that looks complete.

const publicIP = "93.184.216.34"

// upstreamExecutor answers every lookup with answers and dials the test
// server whatever address the guard picked, recording that address.
func upstreamExecutor(t *testing.T, h http.Handler, answers ...string) (*Executor, *atomic.Value) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	dialed := &atomic.Value{}
	dialed.Store("")
	var d net.Dialer
	return &Executor{
		Lookup: func(ctx context.Context, host string) ([]net.IPAddr, error) {
			var out []net.IPAddr
			for _, a := range answers {
				out = append(out, net.IPAddr{IP: net.ParseIP(a)})
			}
			return out, nil
		},
		Dial: func(ctx context.Context, network, addr string) (net.Conn, error) {
			dialed.Store(addr)
			return d.DialContext(ctx, network, srv.Listener.Addr().String())
		},
		Self: func(ip net.IP) bool { return ip.Equal(net.ParseIP("203.0.113.7")) },
	}, dialed
}

func serveFetch(e *Executor, req Request) (*httptest.ResponseRecorder, Result) {
	body, _ := json.Marshal(req)
	r := httptest.NewRequest(http.MethodPost, Path, bytes.NewReader(body))
	w := httptest.NewRecorder()
	res := e.Serve(w, r)
	return w, res
}

func TestHostFetchDialsTheCheckedPublicAddress(t *testing.T) {
	e, dialed := upstreamExecutor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("X-Upstream", "yes")
		_, _ = io.WriteString(w, "<p>hello</p>")
	}), publicIP)
	w, res := serveFetch(e, Request{Method: "GET", URL: "https://example.com/page?q=1"})
	if w.Code != http.StatusOK || w.Header().Get(StatusHeader) != "200" {
		t.Fatalf("status %d, upstream %q: %s", w.Code, w.Header().Get(StatusHeader), w.Body.String())
	}
	if got := dialed.Load().(string); got != net.JoinHostPort(publicIP, "443") {
		t.Fatalf("dialed %q, want the checked address on 443", got)
	}
	hdr, err := DecodeHeaders(w.Header().Get(HeadersHeader))
	if err != nil || hdr.Get("X-Upstream") != "yes" || hdr.Get("Content-Type") != "text/html" {
		t.Fatalf("upstream headers %v, %v", hdr, err)
	}
	if w.Body.String() != "<p>hello</p>" {
		t.Fatalf("body %q", w.Body.String())
	}
	if res.Host != "example.com" || res.Status != 200 || res.Bytes != int64(len("<p>hello</p>")) || res.Refused != "" {
		t.Fatalf("result %+v", res)
	}
}

func TestHostFetchRefusesNonPublicAndHostOwnAnswers(t *testing.T) {
	for _, answer := range []string{"127.0.0.1", "10.0.0.5", "192.168.1.1", "172.16.0.1", "169.254.169.254", "100.64.0.1", "0.0.0.0", "::1", "fe80::1", "64:ff9b::a00:1", "203.0.113.7"} {
		var hits atomic.Int32
		e, _ := upstreamExecutor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1) }), publicIP, answer)
		w, res := serveFetch(e, Request{Method: "GET", URL: "https://rebind.example/"})
		if w.Code != http.StatusForbidden || w.Header().Get(StatusHeader) != "" || hits.Load() != 0 || res.Refused == "" {
			t.Fatalf("answer %s: status %d, upstream hits %d, result %+v", answer, w.Code, hits.Load(), res)
		}
	}
}

func TestHostFetchRefusesBadURLsBeforeResolving(t *testing.T) {
	e, dialed := upstreamExecutor(t, http.NotFoundHandler(), publicIP)
	for _, raw := range []string{"file:///etc/passwd", "gopher://example.com/", "http://user:pw@example.com/", "http://localhost:8084/", "http://metadata.google.internal/", "http://127.0.0.1/", "http://[::1]/", "http://169.254.169.254/latest/meta-data", "/relative", ""} {
		w, res := serveFetch(e, Request{Method: "GET", URL: raw})
		if w.Code != http.StatusForbidden && w.Code != http.StatusBadRequest {
			t.Fatalf("%q: status %d", raw, w.Code)
		}
		if res.Refused == "" || dialed.Load().(string) != "" {
			t.Fatalf("%q: result %+v, dialed %q", raw, res, dialed.Load())
		}
	}
	w, _ := serveFetch(e, Request{Method: "CONNECT", URL: "https://example.com/"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("CONNECT: status %d", w.Code)
	}
}

func TestHostFetchReturnsRedirectsWithoutFollowing(t *testing.T) {
	var hits atomic.Int32
	e, _ := upstreamExecutor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		http.Redirect(w, r, "http://10.0.0.1/next", http.StatusFound)
	}), publicIP)
	w, _ := serveFetch(e, Request{Method: "GET", URL: "http://example.com/start"})
	hdr, _ := DecodeHeaders(w.Header().Get(HeadersHeader))
	if w.Code != http.StatusOK || w.Header().Get(StatusHeader) != "302" || hdr.Get("Location") != "http://10.0.0.1/next" || hits.Load() != 1 {
		t.Fatalf("status %d upstream %q location %q hits %d", w.Code, w.Header().Get(StatusHeader), hdr.Get("Location"), hits.Load())
	}
}

func TestHostFetchForwardsCallerHeadersButNotHopByHopOrBearer(t *testing.T) {
	seen := make(chan http.Header, 1)
	gotBody := make(chan string, 1)
	e, _ := upstreamExecutor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen <- r.Header.Clone()
		b, _ := io.ReadAll(r.Body)
		gotBody <- string(b)
	}), publicIP)
	h := http.Header{}
	h.Set("User-Agent", "ChoirTest/1")
	h.Set("Content-Type", "application/json")
	h.Set("Connection", "X-Secret")
	h.Set("X-Secret", "hop")
	h.Set("Proxy-Authorization", "Basic abc")
	h.Set("Keep-Alive", "300")
	h.Set("Upgrade", "websocket")
	w, _ := serveFetch(e, Request{Method: "POST", URL: "https://www.youtube.com/youtubei/v1/player", Header: h, Body: []byte(`{"videoId":"x"}`)})
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	got := <-seen
	if got.Get("User-Agent") != "ChoirTest/1" || got.Get("Content-Type") != "application/json" {
		t.Fatalf("caller headers lost: %v", got)
	}
	for _, k := range []string{"X-Secret", "Proxy-Authorization", "Keep-Alive", "Upgrade"} {
		if got.Get(k) != "" {
			t.Fatalf("%s forwarded: %v", k, got)
		}
	}
	if b := <-gotBody; b != `{"videoId":"x"}` {
		t.Fatalf("body %q", b)
	}
}

func TestHostFetchDropsTheGatewayBearer(t *testing.T) {
	seen := make(chan string, 1)
	e, _ := upstreamExecutor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { seen <- r.Header.Get("Authorization") }), publicIP)
	body, _ := json.Marshal(Request{Method: "GET", URL: "https://example.com/"})
	r := httptest.NewRequest(http.MethodPost, Path, bytes.NewReader(body))
	r.Header.Set("Authorization", "Bearer gateway-token")
	e.Serve(httptest.NewRecorder(), r)
	if got := <-seen; got != "" {
		t.Fatalf("gateway bearer reached upstream: %q", got)
	}
}

func TestHostFetchBoundsRequestAndResponseBodies(t *testing.T) {
	e, _ := upstreamExecutor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}), publicIP)
	w, res := serveFetch(e, Request{Method: "POST", URL: "https://example.com/", Body: bytes.Repeat([]byte("a"), MaxRequestBody+1)})
	if w.Code != http.StatusRequestEntityTooLarge || res.Refused == "" {
		t.Fatalf("request body over cap: status %d result %+v", w.Code, res)
	}

	// Declared too long: refused before any byte is relayed.
	big, _ := upstreamExecutor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1000")
		_, _ = w.Write(bytes.Repeat([]byte("b"), 1000))
	}), publicIP)
	big.MaxBody = 100
	w, res = serveFetch(big, Request{Method: "GET", URL: "https://example.com/"})
	if w.Code != http.StatusBadGateway || w.Header().Get(StatusHeader) != "" || res.Refused == "" {
		t.Fatalf("declared over cap: status %d upstream %q result %+v", w.Code, w.Header().Get(StatusHeader), res)
	}

	// Undeclared and too long: the relay is aborted, never cut silently.
	chunked, _ := upstreamExecutor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for i := 0; i < 10; i++ {
			_, _ = w.Write(bytes.Repeat([]byte("c"), 50))
			w.(http.Flusher).Flush()
		}
	}), publicIP)
	chunked.MaxBody = 100
	defer func() {
		if p := recover(); p != http.ErrAbortHandler {
			t.Fatalf("undeclared over cap: recovered %v, want http.ErrAbortHandler", p)
		}
	}()
	serveFetch(chunked, Request{Method: "GET", URL: "https://example.com/"})
	t.Fatal("undeclared over cap: the relay completed")
}

func TestDecodeHeadersRefusesGarbage(t *testing.T) {
	if _, err := DecodeHeaders("not base64!"); err == nil {
		t.Fatal("garbage decoded")
	}
	if h, err := DecodeHeaders(""); err != nil || len(h) != 0 {
		t.Fatalf("empty: %v %v", h, err)
	}
	in := http.Header{"Content-Type": {"text/html; charset=utf-8"}, "X-Odd": {"café", "two"}}
	enc := EncodeHeaders(in)
	if strings.ContainsAny(enc, "\r\né") {
		t.Fatalf("encoding is not header-safe: %q", enc)
	}
	out, err := DecodeHeaders(enc)
	if err != nil || out.Get("Content-Type") != "text/html; charset=utf-8" || len(out["X-Odd"]) != 2 || out["X-Odd"][0] != "café" {
		t.Fatalf("round trip: %v %v", out, err)
	}
}
