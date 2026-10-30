package goproxy

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Failure modes pinned here (design §3, host side):
//   F8  a request makes the proxy fetch an arbitrary URL, by path tricks or a
//       redirect off the fixed host list;
//   F9  a path writes outside the cache directory;
//   F10 a mutable answer (list, @latest, a query, sumdb latest/lookup, a
//       partial tile) is cached;
//   F11 an upstream 404/410 does not reach the go command as a 404/410, or a
//       5xx is reported as not found;
//   F12 a cached entry is overwritten by a later fetch;
//   F13 concurrent misses for one entry fetch it more than once;
//   F14 a fetched file that does not match go.sum is cached or served;
//   F15 the cache grows past its cap;
//   F17 the warm fails to fill or to re-verify the cache;
//   and the real go command cannot use the proxy (protocol drift).

const (
	testMod  = "example.com/m"
	testVer  = "v1.0.0"
	testInfo = `{"Version":"v1.0.0","Time":"2026-01-01T00:00:00Z"}`
	testGo   = "module example.com/m\n\ngo 1.21\n"
)

func moduleZip(t testing.TB, mod, ver string, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, body := range files {
		f, err := w.Create(mod + "@" + ver + "/" + name)
		if err != nil {
			t.Fatal(err)
		}
		io.WriteString(f, body)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

type fakeUpstream struct {
	t       *testing.T
	proxy   *httptest.Server
	storage *httptest.Server
	sumdb   *httptest.Server
	evil    *httptest.Server

	mu         sync.Mutex
	hits       map[string]int // path -> upstream requests (proxy, storage, sumdb)
	evilHits   int
	zipBody    []byte
	gate       chan struct{} // when non-nil, storage waits on it
	redirectTo string        // "storage" (default) or "evil"
	status     map[string]int
}

func newFakeUpstream(t *testing.T) *fakeUpstream {
	f := &fakeUpstream{t: t, hits: map[string]int{}, status: map[string]int{}}
	f.zipBody = moduleZip(t, testMod, testVer, map[string]string{"go.mod": testGo, "m.go": "package m\n"})
	f.storage = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.count("storage:" + r.URL.Path)
		f.mu.Lock()
		gate := f.gate
		f.mu.Unlock()
		if gate != nil {
			<-gate
		}
		w.Write(f.zipBody)
	}))
	f.evil = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.evilHits++
		f.mu.Unlock()
		w.Write([]byte("evil"))
	}))
	f.proxy = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.count(r.URL.Path)
		f.mu.Lock()
		status, redirect := f.status[r.URL.Path], f.redirectTo
		f.mu.Unlock()
		if status != 0 {
			http.Error(w, fmt.Sprintf("not found: upstream says %d", status), status)
			return
		}
		base := "/" + testMod + "/@v/"
		switch r.URL.Path {
		case base + "list":
			io.WriteString(w, "v1.0.0\n")
		case "/" + testMod + "/@latest", base + "master.info", base + testVer + ".info":
			io.WriteString(w, testInfo)
		case base + testVer + ".mod":
			io.WriteString(w, testGo)
		case base + testVer + ".zip":
			target := f.storage.URL
			if redirect == "evil" {
				target = f.evil.URL
			}
			http.Redirect(w, r, target+"/proxy-golang-org-prod/abc", http.StatusFound)
		default:
			http.NotFound(w, r)
		}
	}))
	f.sumdb = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.count("sumdb:" + r.URL.Path)
		io.WriteString(w, "sumdb "+r.URL.Path)
	}))
	t.Cleanup(func() { f.proxy.Close(); f.storage.Close(); f.sumdb.Close(); f.evil.Close() })
	return f
}

func (f *fakeUpstream) setStatus(path string, status int) {
	f.mu.Lock()
	f.status[path] = status
	f.mu.Unlock()
}

func (f *fakeUpstream) count(path string) {
	f.mu.Lock()
	f.hits[path]++
	f.mu.Unlock()
}

func (f *fakeUpstream) n(path string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.hits[path]
}

func (f *fakeUpstream) total() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := f.evilHits
	for _, v := range f.hits {
		n += v
	}
	return n
}

type proxyHarness struct {
	t       *testing.T
	up      *fakeUpstream
	srv     *Server
	ts      *httptest.Server
	dir     string
	mu      sync.Mutex
	records []Record
}

func newProxyHarness(t *testing.T, mutate func(*Config)) *proxyHarness {
	t.Helper()
	h := &proxyHarness{t: t, up: newFakeUpstream(t), dir: t.TempDir()}
	storageHost := strings.TrimPrefix(h.up.storage.URL, "http://")
	cfg := Config{
		CacheDir:      h.dir,
		MaxBytes:      DefaultMaxBytes,
		Upstream:      h.up.proxy.URL,
		SumDBUpstream: h.up.sumdb.URL,
		RedirectHosts: []string{storageHost},
		Record: func(r Record) {
			h.mu.Lock()
			h.records = append(h.records, r)
			h.mu.Unlock()
		},
	}
	if mutate != nil {
		mutate(&cfg)
	}
	srv, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	h.srv = srv
	h.ts = httptest.NewServer(srv)
	t.Cleanup(h.ts.Close)
	return h
}

func (h *proxyHarness) get(path string, header ...string) (int, string) {
	h.t.Helper()
	req, err := http.NewRequest(http.MethodGet, h.ts.URL+path, nil)
	if err != nil {
		h.t.Fatal(err)
	}
	for i := 0; i+1 < len(header); i += 2 {
		req.Header.Set(header[i], header[i+1])
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		h.t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

func (h *proxyHarness) lastRecord() Record {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.records) == 0 {
		h.t.Fatal("no records")
	}
	return h.records[len(h.records)-1]
}

func TestServesModuleProtocolAndCachesImmutableAnswers(t *testing.T) {
	h := newProxyHarness(t, nil)
	base := "/" + testMod + "/@v/"

	for _, path := range []string{base + testVer + ".info", base + testVer + ".mod", base + testVer + ".zip"} {
		for i := 0; i < 2; i++ {
			status, body := h.get(path)
			if status != http.StatusOK {
				t.Fatalf("GET %s = %d %q", path, status, body)
			}
			if strings.HasSuffix(path, ".zip") && body != string(h.up.zipBody) {
				t.Fatalf("zip body differs from upstream")
			}
		}
		if n := h.up.n(path); n != 1 {
			t.Fatalf("%s reached upstream %d times, want 1 (second is a cache hit)", path, n)
		}
		if _, err := os.Stat(filepath.Join(h.dir, "cache", filepath.FromSlash(strings.TrimPrefix(path, "/")))); err != nil {
			t.Fatalf("%s not in the GOMODCACHE-layout cache: %v", path, err)
		}
	}
	// The zip came from the storage host by redirect, followed on the host.
	if n := h.up.n("storage:/proxy-golang-org-prod/abc"); n != 1 {
		t.Fatalf("storage fetches = %d, want 1", n)
	}
	if rec := h.lastRecord(); rec.Outcome != OutcomeHit {
		t.Fatalf("second zip request outcome = %q, want hit", rec.Outcome)
	}

	// F10: mutable answers pass through every time and are never cached.
	for _, path := range []string{base + "list", "/" + testMod + "/@latest", base + "master.info"} {
		for i := 0; i < 2; i++ {
			if status, body := h.get(path); status != http.StatusOK || body == "" {
				t.Fatalf("GET %s = %d %q", path, status, body)
			}
		}
		if n := h.up.n(path); n != 2 {
			t.Fatalf("%s reached upstream %d times, want 2 (not cached)", path, n)
		}
	}
	if _, err := os.Stat(filepath.Join(h.dir, "cache", testMod, "@v", "master.info")); err == nil {
		t.Fatal("a version query was cached")
	}
}

func TestRedirectOffTheHostListIsNotFollowed(t *testing.T) {
	h := newProxyHarness(t, nil)
	h.up.mu.Lock()
	h.up.redirectTo = "evil"
	h.up.mu.Unlock()
	status, _ := h.get("/" + testMod + "/@v/" + testVer + ".zip")
	if status != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", status)
	}
	if h.up.total() != 1 { // the proxy's own 302 only
		t.Fatal("the proxy followed a redirect to a host off its list")
	}
	if _, ok := h.srv.cache.lookup("cache/" + testMod + "/@v/" + testVer + ".zip"); ok {
		t.Fatal("a refused fetch left a cache entry")
	}
	if rec := h.lastRecord(); rec.Outcome != OutcomeUpstreamError {
		t.Fatalf("outcome = %q", rec.Outcome)
	}
}

func TestRefusesEverythingButModuleAndSumDBPaths(t *testing.T) {
	h := newProxyHarness(t, nil)
	paths := []string{
		"/",
		"/etc/passwd",
		"/../../etc/passwd",
		"/example.com/../../../etc/@v/v1.0.0.zip",
		"/example.com/m/../../x/@v/list",
		"/example.com/./m/@v/list",
		"/http://evil.example/x/@v/list",
		"/http:/evil.example/@v/list",
		"/Example.com/m/@v/list",               // unescaped uppercase
		"/example.com/m!/@v/list",              // dangling escape
		"/nodot/m/@v/list",                     // first element without a dot
		"/-example.com/m/@v/list",              // leading dash
		"/example.com/m/@v/v1.0.0.exe",         // not a proxy file
		"/example.com/m/@v/master.zip",         // zip of a query
		"/example.com/m/@v/v1.0.0/..%2f..%2fx", // encoded traversal
		"/example.com/m/@v/..%2f..%2fx.info",   // encoded traversal in version
		"/example.com/m@v1/@v/list",            // @ in module path
		"/example.com/m/@v/v1.0.0.info%00",     // NUL
		"/sumdb/sum.example.org/supported",     // other sumdb
		"/sumdb/sum.golang.org/../../etc/passwd",
		"/sumdb/sum.golang.org/tile/8/0/../../x",
		"/sumdb/sum.golang.org/lookup/example.com/m",
		"/sumdb/sum.golang.org/anything",
		"/" + strings.Repeat("a.", 300) + "com/@v/list", // too long
	}
	for _, p := range paths {
		status, _ := h.get(p)
		if status != http.StatusNotFound {
			t.Errorf("GET %q = %d, want 404", p, status)
		}
	}
	req, _ := http.NewRequest(http.MethodPost, h.ts.URL+"/example.com/m/@v/list", strings.NewReader("x"))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("POST = %d, want 405", resp.StatusCode)
	}
	if n := h.up.total(); n != 0 {
		t.Fatalf("refused requests reached upstream %d times", n)
	}
	// F9: nothing was written outside the cache directory (or at all).
	entries, _ := os.ReadDir(h.dir)
	if len(entries) != 0 {
		t.Fatalf("refused requests wrote %v", entries)
	}
}

func TestSumDBProxy(t *testing.T) {
	h := newProxyHarness(t, nil)
	if status, _ := h.get("/sumdb/sum.golang.org/supported"); status != http.StatusOK {
		t.Fatalf("supported = %d, want 200 (the go command then never dials sum.golang.org)", status)
	}
	if h.up.total() != 0 {
		t.Fatal("supported reached upstream")
	}
	full := "/sumdb/sum.golang.org/tile/8/0/x001/234"
	partial := "/sumdb/sum.golang.org/tile/8/0/005.p/12"
	data := "/sumdb/sum.golang.org/tile/8/data/000"
	lookup := "/sumdb/sum.golang.org/lookup/github.com/!burnt!sushi/toml@v1.3.2"
	latest := "/sumdb/sum.golang.org/latest"
	for _, p := range []string{full, partial, data, lookup, latest} {
		for i := 0; i < 2; i++ {
			status, body := h.get(p)
			want := "sumdb " + strings.TrimPrefix(p, "/sumdb/sum.golang.org")
			if status != http.StatusOK || body != want {
				t.Fatalf("GET %s = %d %q, want %q", p, status, body, want)
			}
		}
	}
	for p, want := range map[string]int{full: 1, data: 1, partial: 2, lookup: 2, latest: 2} {
		if n := h.up.n("sumdb:" + strings.TrimPrefix(p, "/sumdb/sum.golang.org")); n != want {
			t.Errorf("%s reached the sumdb %d times, want %d", p, n, want)
		}
	}
	if h.up.n("/sumdb/sum.golang.org/latest") != 0 {
		t.Fatal("a sumdb request went to the module upstream")
	}
}

func TestUpstreamStatusesReachTheGoCommandFaithfully(t *testing.T) {
	h := newProxyHarness(t, nil)
	base := "/" + testMod + "/@v/"
	h.up.setStatus(base+"v9.9.9.mod", http.StatusNotFound)
	h.up.setStatus(base+"v9.9.8.mod", http.StatusGone)
	h.up.setStatus(base+"v9.9.7.mod", http.StatusInternalServerError)
	for path, want := range map[string]int{base + "v9.9.9.mod": 404, base + "v9.9.8.mod": 410, base + "v9.9.7.mod": 502} {
		status, body := h.get(path)
		if status != want {
			t.Errorf("GET %s = %d, want %d", path, status, want)
		}
		if want != 502 && !strings.Contains(body, "upstream says") {
			t.Errorf("GET %s body %q lost the upstream text", path, body)
		}
		if _, ok := h.srv.cache.lookup("cache" + path); ok {
			t.Errorf("%s: a failed fetch was cached", path)
		}
	}
}

func TestKnownSumMismatchIsRefusedAndNotCached(t *testing.T) {
	good, err := hashGoMod([]byte(testGo))
	if err != nil {
		t.Fatal(err)
	}
	bad := newProxyHarness(t, func(c *Config) {
		c.KnownSums = map[string]string{testMod + " " + testVer + "/go.mod": "h1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="}
	})
	status, body := bad.get("/" + testMod + "/@v/" + testVer + ".mod")
	if status != http.StatusBadGateway || !strings.Contains(body, "SECURITY ERROR") {
		t.Fatalf("mismatch = %d %q, want 502 SECURITY ERROR", status, body)
	}
	if _, ok := bad.srv.cache.lookup("cache/" + testMod + "/@v/" + testVer + ".mod"); ok {
		t.Fatal("a mismatching file was cached")
	}
	if rec := bad.lastRecord(); rec.Outcome != OutcomeHashMismatch {
		t.Fatalf("outcome = %q", rec.Outcome)
	}

	zipHash := hashOfBytes(t, bad.up.zipBody)
	ok := newProxyHarness(t, func(c *Config) {
		c.KnownSums = map[string]string{testMod + " " + testVer + "/go.mod": good, testMod + " " + testVer: zipHash}
	})
	for _, ext := range []string{".mod", ".zip"} {
		if status, _ := ok.get("/" + testMod + "/@v/" + testVer + ext); status != http.StatusOK {
			t.Fatalf("matching %s = %d", ext, status)
		}
	}
}

func hashOfBytes(t *testing.T, zipBytes []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "m.zip")
	if err := os.WriteFile(p, zipBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	h, err := hashZip(p)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// The h1 algorithm must match golang.org/x/mod/sumdb/dirhash; goldens were
// computed with x/mod v0.36.0 (zip) and taken from this repo's go.sum
// (github.com/google/uuid v1.6.0/go.mod).
func TestHash1MatchesGoSum(t *testing.T) {
	got, err := hashGoMod([]byte("module github.com/google/uuid\n"))
	if err != nil || got != "h1:TIyPZe4MgqvfeYDBFedMoGGpEw/LqOeaOT+nhxU+yHo=" {
		t.Fatalf("go.mod hash = %q, %v", got, err)
	}
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, e := range [][2]string{
		{"example.com/m@v1.0.0/go.mod", "module example.com/m\n"},
		{"example.com/m@v1.0.0/b.go", "package m\n\nconst B = 2\n"},
		{"example.com/m@v1.0.0/a.go", "package m\n\nconst A = 1\n"},
	} {
		f, _ := w.Create(e[0])
		io.WriteString(f, e[1])
	}
	w.Close()
	if got := hashOfBytes(t, buf.Bytes()); got != "h1:BQCg3y5L5dtask7KeGU60iS4a/orPe21kNjq9bNyIM4=" {
		t.Fatalf("zip hash = %q", got)
	}
}

func TestConcurrentMissesFetchOnce(t *testing.T) {
	h := newProxyHarness(t, func(c *Config) { c.PerSourceFetches = 16 })
	gate := make(chan struct{})
	h.up.mu.Lock()
	h.up.gate = gate
	h.up.mu.Unlock()
	path := "/" + testMod + "/@v/" + testVer + ".zip"
	var wg sync.WaitGroup
	var okCount atomic.Int32
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := http.Get(h.ts.URL + path)
			if err == nil {
				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK && bytes.Equal(body, h.up.zipBody) {
					okCount.Add(1)
				}
			}
		}()
	}
	time.Sleep(200 * time.Millisecond)
	close(gate)
	wg.Wait()
	if okCount.Load() != 10 {
		t.Fatalf("%d of 10 concurrent requests got the zip", okCount.Load())
	}
	if n := h.up.n("storage:/proxy-golang-org-prod/abc"); n != 1 {
		t.Fatalf("10 concurrent misses fetched %d times, want 1", n)
	}
}

func TestCacheIsAppendOnly(t *testing.T) {
	h := newProxyHarness(t, nil)
	rel := "cache/" + testMod + "/@v/" + testVer + ".mod"
	p := cacheFile(h.dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("module example.com/m // first\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, body := h.get("/" + rel[len("cache/"):]); body != "module example.com/m // first\n" {
		t.Fatalf("served %q, want the existing entry", body)
	}
	if h.up.total() != 0 {
		t.Fatal("a cached entry was fetched again")
	}
	// A commit racing an existing entry leaves the existing one in place.
	tmp, err := h.srv.cache.create(rel)
	if err != nil {
		t.Fatal(err)
	}
	io.WriteString(tmp, "second")
	tmp.Close()
	if err := h.srv.cache.commit(tmp.Name(), rel, 6); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(p); string(got) != "module example.com/m // first\n" {
		t.Fatalf("entry overwritten: %q", got)
	}
	if _, err := os.Stat(tmp.Name()); !os.IsNotExist(err) {
		t.Fatal("temp file left behind")
	}
}

func TestEvictionKeepsTheCacheUnderItsCap(t *testing.T) {
	dir := t.TempDir()
	c, err := openDiskCache(dir, 1000)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Now().Add(-time.Hour)
	for i := 0; i < 5; i++ {
		rel := fmt.Sprintf("cache/example.com/m%d/@v/v1.0.0.zip", i)
		tmp, err := c.create(rel)
		if err != nil {
			t.Fatal(err)
		}
		tmp.Write(bytes.Repeat([]byte{'x'}, 300))
		tmp.Close()
		if err := c.commit(tmp.Name(), rel, 300); err != nil {
			t.Fatal(err)
		}
		at := base.Add(time.Duration(i) * time.Minute)
		os.Chtimes(c.path(rel), at, at)
	}
	if c.bytes() > 1000 {
		t.Fatalf("cache is %d bytes over a 1000-byte cap", c.bytes())
	}
	// The newest entry survives; the oldest went first.
	if _, ok := c.lookup("cache/example.com/m4/@v/v1.0.0.zip"); !ok {
		t.Fatal("the newest entry was evicted")
	}
	if _, ok := c.lookup("cache/example.com/m0/@v/v1.0.0.zip"); ok {
		t.Fatal("the oldest entry survived eviction")
	}
	// Reopening recounts from disk and drops stale temp files.
	os.WriteFile(filepath.Join(dir, "cache", tempPrefix+"stale"), []byte("x"), 0o644)
	c2, err := openDiskCache(dir, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if c2.bytes() != c.bytes() {
		t.Fatalf("reopened size %d, want %d", c2.bytes(), c.bytes())
	}
	if _, err := os.Stat(filepath.Join(dir, "cache", tempPrefix+"stale")); !os.IsNotExist(err) {
		t.Fatal("stale temp file survived reopen")
	}
}

// A systemd DynamicUser StateDirectory is a symlink into /var/lib/private
// (docs/archive/problems/goproxy-cache-root-is-a-symlink-2026-10-14.md). The cache
// opened through the link must count the real files and evict files, never
// the link itself.
func TestCacheRootThroughSymlinkCountsAndEvictsRealFiles(t *testing.T) {
	base := t.TempDir()
	real := filepath.Join(base, "private", "goproxy")
	if err := os.MkdirAll(filepath.Join(real, "cache", "example.com", "old", "@v"), 0o755); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(real, "cache", "example.com", "old", "@v", "v1.0.0.zip")
	if err := os.WriteFile(old, bytes.Repeat([]byte{'o'}, 400), 0o644); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-2 * time.Hour)
	os.Chtimes(old, past, past)
	link := filepath.Join(base, "goproxy")
	if err := os.Symlink(filepath.Join("private", "goproxy"), link); err != nil {
		t.Fatal(err)
	}
	c, err := openDiskCache(link, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if c.bytes() != 400 {
		t.Fatalf("cache opened through a symlink counts %d bytes, want 400", c.bytes())
	}
	for i := 0; i < 3; i++ {
		rel := fmt.Sprintf("cache/example.com/m%d/@v/v1.0.0.zip", i)
		tmp, err := c.create(rel)
		if err != nil {
			t.Fatal(err)
		}
		tmp.Write(bytes.Repeat([]byte{'x'}, 300))
		tmp.Close()
		if err := c.commit(tmp.Name(), rel, 300); err != nil {
			t.Fatal(err)
		}
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("eviction removed or replaced the state directory link: %v", err)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatal("the oldest real entry survived eviction")
	}
	if c.bytes() > 1000 {
		t.Fatalf("cache is %d bytes over a 1000-byte cap", c.bytes())
	}
}

func TestWarmFillsAndReverifiesTheCache(t *testing.T) {
	h := newProxyHarness(t, nil)
	modHash, _ := hashGoMod([]byte(testGo))
	sums := map[string]string{
		testMod + " " + testVer:             hashOfBytes(t, h.up.zipBody),
		testMod + " " + testVer + "/go.mod": modHash,
	}
	rep := h.srv.Warm(context.Background(), sums, 2)
	if rep.Entries != 2 || rep.Fetched != 2 || rep.Failed != 0 || rep.Mismatch != 0 {
		t.Fatalf("first warm = %+v", rep)
	}
	for _, ext := range []string{".info", ".mod", ".zip"} {
		if _, ok := h.srv.cache.lookup("cache/" + testMod + "/@v/" + testVer + ext); !ok {
			t.Fatalf("warm did not cache %s", ext)
		}
	}
	if rep := h.srv.Warm(context.Background(), sums, 2); rep.Hits != 2 || rep.Fetched != 0 {
		t.Fatalf("second warm = %+v, want two verified hits", rep)
	}
	// Corrupt the cached go.mod: the next warm notices, refetches, and the
	// served file is right again.
	p := cacheFile(h.dir, "cache/"+testMod+"/@v/"+testVer+".mod")
	os.Chmod(p, 0o644)
	if err := os.WriteFile(p, []byte("module tampered\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if rep := h.srv.Warm(context.Background(), sums, 2); rep.Hits != 1 || rep.Fetched != 1 {
		t.Fatalf("warm after tampering = %+v, want one hit and one refetch", rep)
	}
	if got, _ := os.ReadFile(p); string(got) != testGo {
		t.Fatalf("go.mod after rewarm = %q", got)
	}
	// A go.sum line the upstream cannot satisfy is counted, not fatal.
	sums["example.com/m v1.0.0-missing/go.mod"] = modHash
	h.up.setStatus("/"+testMod+"/@v/v1.0.0-missing.mod", http.StatusNotFound)
	if rep := h.srv.Warm(context.Background(), sums, 2); rep.Failed != 1 || rep.Hits != 2 {
		t.Fatalf("warm with an unfetchable line = %+v", rep)
	}
}

func TestRecordsAttributeTheCapsule(t *testing.T) {
	h := newProxyHarness(t, nil)
	h.get("/"+testMod+"/@v/"+testVer+".mod", CapsuleHeader, "capsule-abc")
	rec := h.lastRecord()
	if rec.Capsule != "capsule-abc" || rec.Outcome != OutcomeFetched || rec.Source == "" || rec.Path == "" {
		t.Fatalf("record = %+v", rec)
	}
	h.get("/etc/passwd")
	if rec := h.lastRecord(); rec.Outcome != OutcomeRefused {
		t.Fatalf("refusal record = %+v", rec)
	}
}

func TestEscapeRoundTrip(t *testing.T) {
	for _, s := range []string{"github.com/BurntSushi/toml", "v1.0.0-RC1", "example.com/m"} {
		e, err := escape(s)
		if err != nil {
			t.Fatal(err)
		}
		if strings.ToLower(e) != e {
			t.Fatalf("escape(%q) = %q has uppercase", s, e)
		}
		u, err := unescape(e)
		if err != nil || u != s {
			t.Fatalf("unescape(escape(%q)) = %q, %v", s, u, err)
		}
	}
	if e, _ := escape("github.com/BurntSushi/toml"); e != "github.com/!burnt!sushi/toml" {
		t.Fatalf("escape = %q", e)
	}
}

// The real go command downloads a module through the proxy with an empty
// module cache: the proxy speaks the protocol the go command expects,
// including the zip redirect it follows on the host.
func TestGoCommandDownloadsThroughTheProxy(t *testing.T) {
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go command not on PATH")
	}
	h := newProxyHarness(t, nil)
	work := t.TempDir()
	os.WriteFile(filepath.Join(work, "go.mod"), []byte("module example.com/client\n\ngo 1.21\n"), 0o644)
	cmd := exec.Command(goBin, "mod", "download", "-json", testMod+"@"+testVer)
	cmd.Dir = work
	cmd.Env = append(os.Environ(),
		"GOPROXY="+h.ts.URL+",off",
		"GOMODCACHE="+filepath.Join(work, "modcache"),
		"GONOSUMDB=example.com", // the fake module is not in sum.golang.org
		"GOFLAGS=-mod=mod -modcacherw",
		"GOTOOLCHAIN=local",
		"HTTP_PROXY=", "HTTPS_PROXY=", "http_proxy=", "https_proxy=",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go mod download: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), `"Sum": "h1:`) {
		t.Fatalf("go mod download output has no sum:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(work, "modcache", "example.com", "m@v1.0.0", "m.go")); err != nil {
		t.Fatalf("module not extracted: %v", err)
	}
}
