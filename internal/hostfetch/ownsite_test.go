package hostfetch

// The own-site exception (docs/problems/own-published-pages-unreadable-by-host-fetch-2026-11-10.md):
// the failure list F1 to F12, written before the code. The guard still refuses
// the host's own address; only the public published surface is served, from the
// loopback, rebuilt from validated parts.

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

const (
	ownBase   = "https://choir.news"
	ownHostIP = "203.0.113.7" // the host's own address (Self in upstreamExecutor)
)

type ownFixture struct {
	exec *Executor
	hits atomic.Int32
	last atomic.Value // *http.Request, a copy of the last upstream request
}

// newOwnFixture is an executor whose public host resolves to the host's own
// address (so the old guard refuses it) and whose loopback proxy is a test
// server that records what it was asked.
func newOwnFixture(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) *ownFixture {
	t.Helper()
	f := &ownFixture{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.hits.Add(1)
		f.last.Store(r.Clone(context.Background()))
		if handler != nil {
			handler(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = io.WriteString(w, "Harbor reopens.\n")
	}))
	t.Cleanup(srv.Close)
	site, err := NewOwnSite(ownBase, srv.URL)
	if err != nil {
		t.Fatalf("NewOwnSite: %v", err)
	}
	f.exec = &Executor{
		OwnSite: site,
		Lookup: func(context.Context, string) ([]net.IPAddr, error) {
			return []net.IPAddr{{IP: net.ParseIP(ownHostIP)}}, nil
		},
		Dial: func(context.Context, string, string) (net.Conn, error) {
			t.Error("the executor dialled a public address for an own-host or host-resolving URL")
			return nil, io.ErrClosedPipe
		},
		Self: func(ip net.IP) bool { return ip.Equal(net.ParseIP(ownHostIP)) },
	}
	return f
}

const exportURL = ownBase + "/api/platform/publications/export?format=txt&raw=1&route=%2Fpub%2Ftexture%2Fharbor-reopens"

func TestOwnSiteServesTheExportAndThePublicPage(t *testing.T) {
	for _, target := range []string{
		exportURL,
		ownBase + "/api/platform/publications/export?route=/pub/texture/harbor-reopens/v2&raw=1&format=txt",
		ownBase + "/pub/texture/harbor-reopens",
		ownBase + "/pub/texture/harbor-reopens/v3",
		"https://CHOIR.NEWS/pub/texture/harbor-reopens",
	} {
		f := newOwnFixture(t, nil)
		w, res := serveFetch(f.exec, Request{Method: "GET", URL: target})
		if w.Code != http.StatusOK || w.Header().Get(StatusHeader) != "200" || !strings.Contains(w.Body.String(), "Harbor reopens") {
			t.Errorf("%s: status %d upstream %q body %q refused=%q", target, w.Code, w.Header().Get(StatusHeader), w.Body.String(), res.Refused)
		}
		if f.hits.Load() != 1 {
			t.Errorf("%s: loopback hits = %d, want 1", target, f.hits.Load())
		}
	}
}

func TestOwnSiteRequestIsRebuiltFromValidatedParts(t *testing.T) { // F4
	f := newOwnFixture(t, nil)
	_, _ = serveFetch(f.exec, Request{Method: "GET", URL: exportURL, Header: http.Header{
		"Cookie": {"session=secret"}, "Authorization": {"Bearer gateway-token"}, "X-Forwarded-For": {"1.2.3.4"},
		"Accept": {"text/plain"}, "Range": {"bytes=0-5"},
	}})
	got, _ := f.last.Load().(*http.Request)
	if got == nil {
		t.Fatal("the loopback was not called")
	}
	for _, h := range []string{"Cookie", "Authorization", "X-Forwarded-For", "Range"} {
		if got.Header.Get(h) != "" {
			t.Errorf("header %s reached the public surface: %q", h, got.Header.Get(h))
		}
	}
	if got.Method != "GET" || got.URL.Path != "/api/platform/publications/export" || got.Host != "choir.news" {
		t.Errorf("loopback request = %s %s host %q", got.Method, got.URL.Path, got.Host)
	}
	q := got.URL.Query()
	if q.Get("route") != "/pub/texture/harbor-reopens" || q.Get("format") != "txt" || q.Get("raw") != "1" || len(q) != 3 {
		t.Errorf("loopback query = %v", q)
	}
}

func TestOwnSiteRefusesEverythingElseOnTheOwnHost(t *testing.T) { // F1, F2, F3, F6, F7, F8, F10
	cases := []struct{ name, method, url string }{
		{"internal route", "GET", ownBase + "/internal/platform/publications/export?route=/pub/texture/x&format=txt&raw=1"},
		{"auth", "GET", ownBase + "/auth/session"},
		{"other api", "GET", ownBase + "/api/other"},
		{"front api", "GET", ownBase + "/api/platform/publications/front"},
		{"root", "GET", ownBase + "/"},
		{"loopback port", "GET", ownBase + ":8086/pub/texture/x"},
		{"https port", "GET", ownBase + ":8443/pub/texture/x"},
		{"loopback address", "GET", "http://127.0.0.1:8082/pub/texture/x"},
		{"localhost", "GET", "http://localhost/pub/texture/x"},
		{"userinfo before", "GET", "https://choir.news@evil.example/pub/texture/x"},
		{"userinfo host", "GET", "https://evil@choir.news/pub/texture/x"},
		{"suffix host", "GET", "https://choir.news.evil.example/pub/texture/x"},
		{"prefix host", "GET", "https://evilchoir.news/pub/texture/x"},
		{"trailing dot", "GET", "https://choir.news./pub/texture/x"},
		{"dot segments", "GET", ownBase + "/pub/texture/../auth/x"},
		{"encoded dot segments", "GET", ownBase + "/pub/texture/%2e%2e/auth/x"},
		{"prefix only", "GET", ownBase + "/pub/textureX"},
		{"bare prefix", "GET", ownBase + "/pub/texture/"},
		{"encoded slash", "GET", ownBase + "/pub/texture/a%2Fb"},
		{"nested", "GET", ownBase + "/pub/texture/a/b"},
		{"version zero", "GET", ownBase + "/pub/texture/a/v0"},
		{"page with query", "GET", ownBase + "/pub/texture/a?x=1"},
		{"export route outside", "GET", ownBase + "/api/platform/publications/export?format=txt&raw=1&route=/auth/x"},
		{"export route dots", "GET", ownBase + "/api/platform/publications/export?format=txt&raw=1&route=/pub/texture/../x"},
		{"export json", "GET", ownBase + "/api/platform/publications/export?format=json&raw=1&route=/pub/texture/a"},
		{"export no raw", "GET", ownBase + "/api/platform/publications/export?format=txt&route=/pub/texture/a"},
		{"export extra param", "GET", ownBase + "/api/platform/publications/export?format=txt&raw=1&route=/pub/texture/a&x=1"},
		{"export repeated route", "GET", ownBase + "/api/platform/publications/export?format=txt&raw=1&route=/pub/texture/a&route=/auth/x"},
		{"export trailing slash path", "GET", ownBase + "/api/platform/publications/export/?format=txt&raw=1&route=/pub/texture/a"},
		{"post export", "POST", exportURL},
		{"post page", "POST", ownBase + "/pub/texture/a"},
		{"head page", "HEAD", ownBase + "/pub/texture/a"},
		{"non-own host on the host address", "GET", "https://other.example/pub/texture/a"},
		{"plain http own host", "GET", "http://choir.news/pub/texture/a"},
	}
	for _, c := range cases {
		f := newOwnFixture(t, nil)
		w, res := serveFetch(f.exec, Request{Method: c.method, URL: c.url, Body: nil})
		if w.Header().Get(StatusHeader) != "" {
			t.Errorf("%s: %s %s was served (upstream status %s)", c.name, c.method, c.url, w.Header().Get(StatusHeader))
		}
		if w.Code < 400 {
			t.Errorf("%s: answered %d, want a refusal", c.name, w.Code)
		}
		if f.hits.Load() != 0 {
			t.Errorf("%s: the loopback was called %d times", c.name, f.hits.Load())
		}
		_ = res
	}
}

func TestOwnSiteDoesNotFollowARedirect(t *testing.T) { // F5
	f := newOwnFixture(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, ownBase+"/auth/login", http.StatusFound)
	})
	w, _ := serveFetch(f.exec, Request{Method: "GET", URL: ownBase + "/pub/texture/harbor-reopens"})
	if w.Header().Get(StatusHeader) != "302" {
		t.Fatalf("status %q, want the 302 handed back to the guest", w.Header().Get(StatusHeader))
	}
	if f.hits.Load() != 1 {
		t.Fatalf("loopback hits = %d, the host followed the redirect", f.hits.Load())
	}
	// The guest re-sends the hop; the disallowed target is refused.
	w2, _ := serveFetch(f.exec, Request{Method: "GET", URL: ownBase + "/auth/login"})
	if w2.Header().Get(StatusHeader) != "" || w2.Code < 400 || f.hits.Load() != 1 {
		t.Fatalf("redirect target served: %d %q hits=%d", w2.Code, w2.Header().Get(StatusHeader), f.hits.Load())
	}
}

func TestOwnSiteKeepsTheBodyBound(t *testing.T) { // F9
	f := newOwnFixture(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, strings.Repeat("x", 4096))
	})
	f.exec.MaxBody = 1024
	defer func() {
		if recover() != http.ErrAbortHandler {
			t.Fatal("an over-cap own-site body was relayed as complete")
		}
	}()
	_, _ = serveFetch(f.exec, Request{Method: "GET", URL: ownBase + "/pub/texture/harbor-reopens"})
}

func TestOwnSiteOffWithoutConfigurationKeepsTheOldRefusal(t *testing.T) {
	f := newOwnFixture(t, nil)
	f.exec.OwnSite = nil
	w, res := serveFetch(f.exec, Request{Method: "GET", URL: exportURL})
	if w.Code != http.StatusForbidden || !strings.Contains(res.Refused, "forbidden address") {
		t.Fatalf("status %d refused %q, want the forbidden address refusal", w.Code, res.Refused)
	}
}

func TestNewOwnSiteNeedsALoopbackTarget(t *testing.T) {
	for _, bad := range []string{"", "http://203.0.113.7:8082", "https://choir.news", "http://example.com", "ftp://127.0.0.1", "http://user@127.0.0.1:8082"} {
		if _, err := NewOwnSite(ownBase, bad); err == nil {
			t.Errorf("NewOwnSite accepted loopback target %q", bad)
		}
	}
	for _, bad := range []string{"", "not a url", "choir.news", "https://choir.news/path", "https://u@choir.news"} {
		if _, err := NewOwnSite(bad, "http://127.0.0.1:8082"); err == nil {
			t.Errorf("NewOwnSite accepted public base %q", bad)
		}
	}
	if _, err := NewOwnSite("https://choir.news/", "http://127.0.0.1:8082"); err != nil {
		t.Errorf("trailing slash on the public base: %v", err)
	}
}

func TestOwnSiteFromEnvUsesThePublicBaseConfiguration(t *testing.T) { // F11
	env := map[string]string{"GATEWAY_OWN_SITE_URL": "http://127.0.0.1:8082", "CORPUSD_PUBLIC_BASE_URL": "https://paper.example"}
	site, err := OwnSiteFromEnv(func(k string) string { return env[k] })
	if err != nil || site == nil || site.Host() != "paper.example" {
		t.Fatalf("site %+v err %v, want host paper.example from the public base URL", site, err)
	}
	delete(env, "CORPUSD_PUBLIC_BASE_URL")
	site, err = OwnSiteFromEnv(func(k string) string { return env[k] })
	if err != nil || site == nil || site.Host() != "choir.news" {
		t.Fatalf("site %+v err %v, want the default public host", site, err)
	}
	delete(env, "GATEWAY_OWN_SITE_URL")
	if site, err = OwnSiteFromEnv(func(k string) string { return env[k] }); site != nil || err != nil {
		t.Fatalf("no loopback URL must mean no exception: %+v %v", site, err)
	}
}
