package pkgmirror

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/blake2b"
)

// Failure modes pinned here (the station file's list,
// docs/definitions/choir-launch-fp10-net02-mirrors-2026-10-21.md):
//   F1  a path outside an ecosystem's grammar reaches an upstream;
//   F2  a crafted name or version escapes the cache directory;
//   F3  an upstream redirect leads off the fixed host table;
//   F4  bytes that do not match the registry's hash, or whose hash is
//       unknown, enter the shared cache;
//   F5  an upstream body past the cap is accepted;
//   F6  a cache entry is replaced, or fetched twice by concurrent misses;
//   F7  a disabled ecosystem serves anything;
//   F8  a method other than GET or HEAD is served;
//   F9  the PyPI index rewrite points a client at the internet, drops the
//       hash, or lets a URL outside files.pythonhosted.org/packages/ through.

const testBase = "http://mirror.choir.internal"

// upstream is one fake registry host that counts what reaches it.
type upstream struct {
	srv    *httptest.Server
	mu     sync.Mutex
	hits   map[string]int
	routes map[string]func(http.ResponseWriter, *http.Request)
	gate   chan struct{}
}

func newUpstream(t *testing.T) *upstream {
	u := &upstream{hits: map[string]int{}, routes: map[string]func(http.ResponseWriter, *http.Request){}}
	u.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.EscapedPath()
		u.mu.Lock()
		u.hits[key]++
		h, gate := u.routes[key], u.gate
		u.mu.Unlock()
		if gate != nil {
			<-gate
		}
		if h == nil {
			http.NotFound(w, r)
			return
		}
		h(w, r)
	}))
	t.Cleanup(u.srv.Close)
	return u
}

func (u *upstream) handle(path string, h func(http.ResponseWriter, *http.Request)) {
	u.mu.Lock()
	u.routes[path] = h
	u.mu.Unlock()
}

func (u *upstream) serve(path, ctype string, body []byte) {
	u.handle(path, func(w http.ResponseWriter, r *http.Request) {
		if ctype != "" {
			w.Header().Set("Content-Type", ctype)
		}
		w.Write(body)
	})
}

func (u *upstream) n(path string) int {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.hits[path]
}

func (u *upstream) total() int {
	u.mu.Lock()
	defer u.mu.Unlock()
	n := 0
	for _, v := range u.hits {
		n += v
	}
	return n
}

type harness struct {
	t                                   *testing.T
	npm, pypi, files, cindex, cdl, evil *upstream
	srv                                 *Server
	ts                                  *httptest.Server
	dir                                 string
	mu                                  sync.Mutex
	records                             []Record
}

func newHarness(t *testing.T, mutate func(*Config)) *harness {
	t.Helper()
	h := &harness{t: t, dir: t.TempDir()}
	h.npm, h.pypi, h.files, h.cindex, h.cdl, h.evil = newUpstream(t), newUpstream(t), newUpstream(t), newUpstream(t), newUpstream(t), newUpstream(t)
	cfg := Config{
		CacheDir:   h.dir,
		PublicBase: testBase,
		Ecosystems: []string{EcosystemNPM, EcosystemPyPI, EcosystemCrates},
		Upstreams: Upstreams{
			NPM: h.npm.srv.URL, PyPIIndex: h.pypi.srv.URL, PyPIFiles: h.files.srv.URL,
			CratesIndex: h.cindex.srv.URL, CratesDownload: h.cdl.srv.URL,
		},
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

func (h *harness) do(method, path string, header ...string) (int, http.Header, string) {
	h.t.Helper()
	req, err := http.NewRequest(method, h.ts.URL+path, nil)
	if err != nil {
		h.t.Fatal(err)
	}
	// Keep the path exactly as written (an escaped slash stays escaped).
	req.URL.Opaque = "//" + req.URL.Host + path
	for i := 0; i+1 < len(header); i += 2 {
		req.Header.Set(header[i], header[i+1])
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		h.t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header, string(body)
}

func (h *harness) get(path string, header ...string) (int, string) {
	h.t.Helper()
	status, _, body := h.do(http.MethodGet, path, header...)
	return status, body
}

func (h *harness) upstreamTotal() int {
	return h.npm.total() + h.pypi.total() + h.files.total() + h.cindex.total() + h.cdl.total() + h.evil.total()
}

func (h *harness) lastRecord() Record {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.records) == 0 {
		h.t.Fatal("no records")
	}
	return h.records[len(h.records)-1]
}

func (h *harness) cached(rel string) bool {
	_, err := os.Stat(filepath.Join(h.dir, "files", filepath.FromSlash(rel)))
	return err == nil
}

// ── npm fixtures ──────────────────────────────────────────────────────────

func npmTarball(t testing.TB, name, version string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	files := map[string]string{
		"package/package.json": fmt.Sprintf(`{"name":%q,"version":%q,"main":"index.js"}`, name, version),
		"package/index.js":     "module.exports = 42;\n",
	}
	for _, n := range []string{"package/package.json", "package/index.js"} {
		body := files[n]
		if err := tw.WriteHeader(&tar.Header{Name: n, Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		io.WriteString(tw, body)
	}
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

func sri512(b []byte) string {
	sum := sha512.Sum512(b)
	return "sha512-" + base64.StdEncoding.EncodeToString(sum[:])
}

// packument is an abbreviated npm document for name with one version whose
// tarball sits at tarballURL with the given integrity.
func packument(name, version, tarballURL, integrity string) []byte {
	doc := map[string]any{
		"name":      name,
		"dist-tags": map[string]string{"latest": version},
		"versions": map[string]any{
			version: map[string]any{
				"name": name, "version": version,
				"dist": map[string]any{"tarball": tarballURL, "integrity": integrity},
			},
		},
	}
	b, _ := json.Marshal(doc)
	return b
}

func (h *harness) npmPackage(name, version string, tarball []byte, integrity string) (packumentPath, tarballPath string) {
	base := name
	scopePath := name
	if strings.HasPrefix(name, "@") {
		scope, pkg, _ := strings.Cut(name, "/")
		base = pkg
		scopePath = scope + "%2F" + pkg
	}
	file := base + "-" + version + ".tgz"
	tarballPath = "/" + name + "/-/" + file
	h.npm.serve("/"+scopePath, "application/json", packument(name, version, "https://registry.npmjs.org"+tarballPath, integrity))
	h.npm.serve(tarballPath, "application/octet-stream", tarball)
	return "/" + scopePath, tarballPath
}

// ── tests ─────────────────────────────────────────────────────────────────

func TestNPMTarballVerifiedCachedAndServedOnce(t *testing.T) {
	h := newHarness(t, nil)
	tgz := npmTarball(t, "left-pad", "1.3.0")
	_, tarPath := h.npmPackage("left-pad", "1.3.0", tgz, sri512(tgz))

	for i := 0; i < 2; i++ {
		status, body := h.get("/npm" + tarPath)
		if status != http.StatusOK || body != string(tgz) {
			t.Fatalf("GET tarball #%d = %d (%d bytes)", i, status, len(body))
		}
	}
	if n := h.npm.n(tarPath); n != 1 {
		t.Fatalf("tarball reached upstream %d times, want 1", n)
	}
	if !h.cached("npm/left-pad/-/left-pad-1.3.0.tgz") {
		t.Fatal("tarball not in the cache")
	}
	if rec := h.lastRecord(); rec.Outcome != OutcomeHit || rec.Ecosystem != EcosystemNPM {
		t.Fatalf("second request record = %+v, want an npm hit", rec)
	}
}

func TestNPMPackumentPassesThroughUncachedForScopedNames(t *testing.T) {
	h := newHarness(t, nil)
	tgz := npmTarball(t, "@types/node", "1.0.0")
	pkPath, tarPath := h.npmPackage("@types/node", "1.0.0", tgz, sri512(tgz))

	// npm escapes the scope's slash; the mirror forwards it escaped.
	for _, path := range []string{"/npm/@types%2Fnode", "/npm/@types%2fnode", "/npm/@types/node"} {
		status, body := h.get(path, "Accept", "application/vnd.npm.install-v1+json")
		if status != http.StatusOK || !strings.Contains(body, `"@types/node"`) {
			t.Fatalf("GET %s = %d %q", path, status, body)
		}
	}
	if n := h.npm.n(pkPath); n != 3 {
		t.Fatalf("packument reached upstream %d times, want 3 (never cached)", n)
	}
	if status, body := h.get("/npm" + tarPath); status != http.StatusOK || body != string(tgz) {
		t.Fatalf("scoped tarball = %d", status)
	}
	if !h.cached("npm/@types/node/-/node-1.0.0.tgz") {
		t.Fatal("scoped tarball not in the cache")
	}
}

func TestNPMTarballIntegrityMismatchIsNotCached(t *testing.T) {
	h := newHarness(t, nil)
	tgz := npmTarball(t, "left-pad", "1.3.0")
	other := npmTarball(t, "left-pad", "9.9.9")
	_, tarPath := h.npmPackage("left-pad", "1.3.0", tgz, sri512(other))

	status, body := h.get("/npm" + tarPath)
	if status != http.StatusBadGateway || !strings.Contains(body, "SECURITY ERROR") {
		t.Fatalf("mismatch = %d %q, want 502 SECURITY ERROR", status, body)
	}
	if h.cached("npm/left-pad/-/left-pad-1.3.0.tgz") {
		t.Fatal("F4: a mismatched tarball entered the cache")
	}
	if rec := h.lastRecord(); rec.Outcome != OutcomeHashMismatch {
		t.Fatalf("outcome = %q, want hash_mismatch", rec.Outcome)
	}
}

func TestNPMTarballWithUnknownIntegrityIsRefused(t *testing.T) {
	h := newHarness(t, nil)
	tgz := npmTarball(t, "left-pad", "1.3.0")
	// The packument names another version only, then no integrity at all.
	h.npm.serve("/left-pad", "application/json", packument("left-pad", "1.2.0", "https://registry.npmjs.org/left-pad/-/left-pad-1.2.0.tgz", sri512(tgz)))
	h.npm.serve("/left-pad/-/left-pad-1.3.0.tgz", "", tgz)
	if status, _ := h.get("/npm/left-pad/-/left-pad-1.3.0.tgz"); status == http.StatusOK {
		t.Fatal("F4: a tarball whose version the packument lacks was served")
	}
	h.npm.serve("/left-pad", "application/json", packument("left-pad", "1.3.0", "https://registry.npmjs.org/left-pad/-/left-pad-1.3.0.tgz", ""))
	if status, _ := h.get("/npm/left-pad/-/left-pad-1.3.0.tgz"); status == http.StatusOK {
		t.Fatal("F4: a tarball with no published integrity was served")
	}
	// A packument whose tarball URL names another file does not vouch for this one.
	h.npm.serve("/left-pad", "application/json", packument("left-pad", "1.3.0", "https://registry.npmjs.org/left-pad/-/other-1.3.0.tgz", sri512(tgz)))
	if status, _ := h.get("/npm/left-pad/-/left-pad-1.3.0.tgz"); status == http.StatusOK {
		t.Fatal("F4: a packument naming another tarball vouched for this one")
	}
	if h.cached("npm/left-pad/-/left-pad-1.3.0.tgz") {
		t.Fatal("F4: an unverified tarball entered the cache")
	}
}

// ── PyPI ──────────────────────────────────────────────────────────────────

func blakePath(b []byte, filename string) string {
	sum := blake2b.Sum256(b)
	x := hex.EncodeToString(sum[:])
	return "/packages/" + x[:2] + "/" + x[2:4] + "/" + x[4:] + "/" + filename
}

func sha256hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func (h *harness) pypiProject(name string, files map[string][]byte, extra []map[string]any) map[string]string {
	paths := map[string]string{}
	var list []map[string]any
	for fn, body := range files {
		p := blakePath(body, fn)
		paths[fn] = p
		h.files.serve(p, "application/octet-stream", body)
		list = append(list, map[string]any{
			"filename": fn, "url": "https://files.pythonhosted.org" + p,
			"hashes": map[string]string{"sha256": sha256hex(body)}, "requires-python": ">=3.8",
			"core-metadata": map[string]string{"sha256": strings.Repeat("ab", 32)},
		})
	}
	list = append(list, extra...)
	doc, _ := json.Marshal(map[string]any{"meta": map[string]string{"api-version": "1.1"}, "name": name, "files": list, "versions": []string{"1.0"}})
	h.pypi.serve("/simple/"+name+"/", "application/vnd.pypi.simple.v1+json", doc)
	return paths
}

func TestPyPIIndexIsRewrittenToTheMirrorAsJSONAndHTML(t *testing.T) {
	h := newHarness(t, nil)
	wheel := []byte("PK wheel bytes")
	paths := h.pypiProject("six", map[string][]byte{"six-1.0-py3-none-any.whl": wheel}, []map[string]any{
		// F9: a file hosted anywhere else, or without a sha256, is dropped.
		{"filename": "six-1.0.tar.gz", "url": "https://evil.example/packages/aa/bb/cc/six-1.0.tar.gz", "hashes": map[string]string{"sha256": sha256hex([]byte("x"))}},
		{"filename": "six-0.9.tar.gz", "url": "https://files.pythonhosted.org/packages/aa/bb/cc/six-0.9.tar.gz", "hashes": map[string]string{}},
		{"filename": "six-0.8.tar.gz", "url": "http://files.pythonhosted.org/packages/aa/bb/cc/six-0.8.tar.gz", "hashes": map[string]string{"sha256": sha256hex([]byte("y"))}},
	})
	want := testBase + "/pypi" + paths["six-1.0-py3-none-any.whl"]

	status, hdr, body := h.do(http.MethodGet, "/pypi/simple/six/", "Accept", "application/vnd.pypi.simple.v1+json")
	if status != http.StatusOK || !strings.HasPrefix(hdr.Get("Content-Type"), "application/vnd.pypi.simple.v1+json") {
		t.Fatalf("JSON index = %d %q", status, hdr.Get("Content-Type"))
	}
	var doc struct {
		Files []struct {
			Filename string            `json:"filename"`
			URL      string            `json:"url"`
			Hashes   map[string]string `json:"hashes"`
		} `json:"files"`
	}
	if err := json.Unmarshal([]byte(body), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Files) != 1 || doc.Files[0].URL != want || doc.Files[0].Hashes["sha256"] != sha256hex(wheel) {
		t.Fatalf("F9: rewritten files = %+v, want only the wheel at %s", doc.Files, want)
	}
	for _, bad := range []string{"evil.example", "six-0.9", "six-0.8", "files.pythonhosted.org"} {
		if strings.Contains(body, bad) {
			t.Fatalf("F9: JSON index still carries %q", bad)
		}
	}

	// A client that does not ask for JSON gets PEP 503 HTML with the hash.
	status, hdr, body = h.do(http.MethodGet, "/pypi/simple/Six/", "Accept", "text/html")
	if status != http.StatusOK || !strings.HasPrefix(hdr.Get("Content-Type"), "text/html") {
		t.Fatalf("HTML index = %d %q", status, hdr.Get("Content-Type"))
	}
	if !strings.Contains(body, `href="`+want+`#sha256=`+sha256hex(wheel)+`"`) || !strings.Contains(body, `data-requires-python="&gt;=3.8"`) {
		t.Fatalf("HTML index lacks the rewritten link: %s", body)
	}
	if strings.Contains(body, "evil.example") || strings.Contains(body, "files.pythonhosted.org") {
		t.Fatal("F9: HTML index points at the internet")
	}
}

func TestPyPIFileVerifiedAgainstItsPathHash(t *testing.T) {
	h := newHarness(t, nil)
	wheel := []byte("PK wheel bytes")
	paths := h.pypiProject("six", map[string][]byte{"six-1.0-py3-none-any.whl": wheel}, nil)
	p := paths["six-1.0-py3-none-any.whl"]
	for i := 0; i < 2; i++ {
		if status, body := h.get("/pypi" + p); status != http.StatusOK || body != string(wheel) {
			t.Fatalf("GET file #%d = %d", i, status)
		}
	}
	if n := h.files.n(p); n != 1 {
		t.Fatalf("file reached upstream %d times, want 1", n)
	}

	// F4: bytes that do not match the BLAKE2b-256 in the path never enter.
	forged := blakePath([]byte("the real bytes"), "evil-1.0-py3-none-any.whl")
	h.files.serve(forged, "", []byte("other bytes"))
	if status, body := h.get("/pypi" + forged); status != http.StatusBadGateway || !strings.Contains(body, "SECURITY ERROR") {
		t.Fatalf("forged file = %d %q, want 502 SECURITY ERROR", status, body)
	}
	if h.cached("pypi" + forged) {
		t.Fatal("F4: a file whose hash does not match its path entered the cache")
	}
}

// ── crates ────────────────────────────────────────────────────────────────

func indexLine(name, vers, cksum string) string {
	b, _ := json.Marshal(map[string]any{"name": name, "vers": vers, "deps": []any{}, "cksum": cksum, "features": map[string]any{}, "yanked": false})
	return string(b)
}

func TestCratesConfigIndexAndVerifiedDownload(t *testing.T) {
	h := newHarness(t, nil)
	crate := []byte("crate bytes for serde")
	h.cindex.serve("/se/rd/serde", "text/plain", []byte(indexLine("serde", "1.0.0", sha256hex([]byte("old")))+"\n"+indexLine("serde", "1.0.1", sha256hex(crate))+"\n"))
	h.cdl.serve("/crates/serde/serde-1.0.1.crate", "application/gzip", crate)
	h.cdl.serve("/crates/serde/serde-1.0.0.crate", "application/gzip", crate)

	status, body := h.get("/crates/index/config.json")
	var cfg map[string]any
	if status != http.StatusOK || json.Unmarshal([]byte(body), &cfg) != nil || cfg["dl"] != testBase+"/crates/dl" {
		t.Fatalf("config.json = %d %q", status, body)
	}
	if h.cindex.total() != 0 {
		t.Fatal("config.json was fetched upstream; the mirror writes its own")
	}
	if status, body := h.get("/crates/index/se/rd/serde"); status != http.StatusOK || !strings.Contains(body, `"1.0.1"`) {
		t.Fatalf("index file = %d %q", status, body)
	}
	for i := 0; i < 2; i++ {
		if status, body := h.get("/crates/dl/serde/1.0.1/download"); status != http.StatusOK || body != string(crate) {
			t.Fatalf("download #%d = %d", i, status)
		}
	}
	if n := h.cdl.n("/crates/serde/serde-1.0.1.crate"); n != 1 {
		t.Fatalf("crate reached upstream %d times, want 1", n)
	}
	// F4: the index's cksum for 1.0.0 does not match.
	if status, body := h.get("/crates/dl/serde/1.0.0/download"); status != http.StatusBadGateway || !strings.Contains(body, "SECURITY ERROR") {
		t.Fatalf("mismatched crate = %d %q", status, body)
	}
	// F4: a version the index does not list is refused, not served.
	if status, _ := h.get("/crates/dl/serde/2.0.0/download"); status == http.StatusOK {
		t.Fatal("F4: a crate version the index lacks was served")
	}
}

// ── shared contracts ──────────────────────────────────────────────────────

func TestPathsOutsideTheGrammarNeverReachAnUpstream(t *testing.T) {
	h := newHarness(t, nil)
	for _, path := range []string{
		"/", "/npm", "/npm/", "/npm/../etc/passwd", "/npm/%2e%2e/x", "/npm/.hidden", "/npm/_private",
		"/npm/left-pad/-/../../x.tgz", "/npm/left-pad/-/right-pad-1.0.0.tgz", "/npm/left-pad/-/left-pad-.tgz",
		"/npm/left-pad/-/left-pad-1.0.0.tar", "/npm/@scope", "/npm/@scope/", "/npm/@/x", "/npm/a/b/c",
		"/npm/-/v1/search", "/npm/-/npm/v1/security/audits/quick", "/npm/left-pad?write=1",
		"/npm/x%252fy", "/npm/https:%2F%2Fevil.example", "/npm/left%00pad",
		"/pypi/simple/", "/pypi/simple/../../x/", "/pypi/simple/-bad/", "/pypi/simple/a/b/",
		"/pypi/packages/source/s/six/six-1.0.tar.gz", "/pypi/packages/aa/bb/cc/six.whl",
		"/pypi/packages/zz/bb/" + strings.Repeat("c", 60) + "/six.whl",
		"/pypi/packages/aa/bb/" + strings.Repeat("c", 60) + "/../six.whl",
		"/pypi/packages/aa/bb/" + strings.Repeat("c", 60) + "/.six.whl",
		"/pypi/pypi/six/json",
		"/crates/index/se/rd/Serde", "/crates/index/xx/yy/serde", "/crates/index/1/ab", "/crates/index/3/b/abc",
		"/crates/index/../config.json", "/crates/dl/serde/1.0.0", "/crates/dl/serde/../1.0.0/download",
		"/crates/dl/-serde/1.0.0/download", "/crates/dl/serde/1.0.0/download/x", "/crates/api/v1/crates/new",
		"/git/", "/other/x",
	} {
		status, _ := h.get(path)
		if status == http.StatusOK {
			t.Errorf("F1: GET %s = 200", path)
		}
	}
	if n := h.upstreamTotal(); n != 0 {
		t.Fatalf("F1: refused paths reached upstreams %d times", n)
	}
	// F2: nothing was written outside the cache's files and git trees.
	entries, _ := os.ReadDir(h.dir)
	for _, e := range entries {
		if e.Name() != "files" && e.Name() != "git" {
			t.Fatalf("F2: cache root has %q", e.Name())
		}
	}
}

func TestOnlyGetAndHeadAreServed(t *testing.T) {
	h := newHarness(t, nil)
	tgz := npmTarball(t, "left-pad", "1.3.0")
	_, tarPath := h.npmPackage("left-pad", "1.3.0", tgz, sri512(tgz))
	for _, m := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		if status, _, _ := h.do(m, "/npm"+tarPath); status != http.StatusMethodNotAllowed {
			t.Fatalf("F8: %s = %d, want 405", m, status)
		}
	}
	if h.upstreamTotal() != 0 {
		t.Fatal("F8: a refused method reached an upstream")
	}
	status, hdr, body := h.do(http.MethodHead, "/npm"+tarPath)
	if status != http.StatusOK || body != "" || hdr.Get("Content-Length") != fmt.Sprint(len(tgz)) {
		t.Fatalf("HEAD = %d len %q body %d", status, hdr.Get("Content-Length"), len(body))
	}
}

func TestDisabledEcosystemsServeNothing(t *testing.T) {
	h := newHarness(t, func(c *Config) { c.Ecosystems = nil })
	tgz := npmTarball(t, "left-pad", "1.3.0")
	_, tarPath := h.npmPackage("left-pad", "1.3.0", tgz, sri512(tgz))
	for _, path := range []string{"/npm" + tarPath, "/npm/left-pad", "/pypi/simple/six/", "/crates/index/config.json", "/crates/index/se/rd/serde"} {
		status, body := h.get(path)
		if status != http.StatusNotFound || !strings.Contains(body, "not enabled") {
			t.Fatalf("F7: GET %s with every ecosystem off = %d %q", path, status, body)
		}
	}
	if h.upstreamTotal() != 0 {
		t.Fatal("F7: a disabled ecosystem reached an upstream")
	}
	if status, body := h.get("/healthz"); status != http.StatusOK || !strings.Contains(body, `"ecosystems":[]`) {
		t.Fatalf("healthz = %d %q", status, body)
	}
}

func TestRedirectsOffTheHostTableAreRefused(t *testing.T) {
	h := newHarness(t, nil)
	tgz := npmTarball(t, "left-pad", "1.3.0")
	h.npm.serve("/left-pad", "application/json", packument("left-pad", "1.3.0", "https://registry.npmjs.org/left-pad/-/left-pad-1.3.0.tgz", sri512(tgz)))
	h.evil.serve("/x.tgz", "", tgz)
	h.npm.handle("/left-pad/-/left-pad-1.3.0.tgz", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, h.evil.srv.URL+"/x.tgz", http.StatusFound)
	})
	if status, _ := h.get("/npm/left-pad/-/left-pad-1.3.0.tgz"); status != http.StatusBadGateway {
		t.Fatalf("F3: redirect off the table = %d, want 502", status)
	}
	if h.evil.total() != 0 {
		t.Fatal("F3: the mirror followed a redirect off its host table")
	}
}

func TestUpstreamBodiesPastTheCapAreRefused(t *testing.T) {
	h := newHarness(t, func(c *Config) { c.FileCap = 64; c.MetaCap = 1 << 20 })
	big := bytes.Repeat([]byte("x"), 65)
	h.cindex.serve("/se/rd/serde", "", []byte(indexLine("serde", "1.0.0", sha256hex(big))+"\n"))
	h.cdl.serve("/crates/serde/serde-1.0.0.crate", "", big)
	if status, _ := h.get("/crates/dl/serde/1.0.0/download"); status != http.StatusBadGateway {
		t.Fatalf("F5: oversized crate = %d, want 502", status)
	}
	if h.cached("crates/serde/1.0.0.crate") {
		t.Fatal("F5: an oversized crate entered the cache")
	}
	h2 := newHarness(t, func(c *Config) { c.MetaCap = 16 })
	h2.npm.serve("/left-pad", "application/json", bytes.Repeat([]byte(" "), 64))
	if status, body := h2.get("/npm/left-pad"); status == http.StatusOK && len(body) > 16 {
		t.Fatalf("F5: an oversized packument passed through whole (%d bytes)", len(body))
	}
}

func TestConcurrentMissesFetchOnce(t *testing.T) {
	h := newHarness(t, func(c *Config) { c.PerSourceFetches = 8 })
	crate := []byte("crate bytes")
	h.cindex.serve("/se/rd/serde", "", []byte(indexLine("serde", "1.0.0", sha256hex(crate))+"\n"))
	h.cdl.serve("/crates/serde/serde-1.0.0.crate", "", crate)
	gate := make(chan struct{})
	h.cdl.mu.Lock()
	h.cdl.gate = gate
	h.cdl.mu.Unlock()
	var wg sync.WaitGroup
	errs := make(chan string, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := http.Get(h.ts.URL + "/crates/dl/serde/1.0.0/download")
			if err != nil {
				errs <- err.Error()
				return
			}
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK || string(b) != string(crate) {
				errs <- fmt.Sprintf("status %d", resp.StatusCode)
			}
		}()
	}
	// Let every request reach the flight before the upstream answers.
	deadline := time.Now().Add(5 * time.Second)
	for h.srv.flightWaiters("crates/serde/1.0.0.crate") < 8 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	close(gate)
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Fatal(e)
	}
	if n := h.cdl.n("/crates/serde/serde-1.0.0.crate"); n != 1 {
		t.Fatalf("F6: concurrent misses fetched %d times, want 1", n)
	}
}

// flightWaiters is how many requests wait on rel's fetch.
func (s *Server) flightWaiters(rel string) int {
	s.flightMu.Lock()
	defer s.flightMu.Unlock()
	if f, ok := s.flights[rel]; ok {
		return f.waiters
	}
	return 0
}

// TestRealNPMInstallsThroughTheMirror pins the protocol against the real npm
// client: npm fetches a tarball the packument names on registry.npmjs.org
// from the configured registry (replace-registry-host's default), so the
// mirror needs no packument rewrite.
func TestRealNPMInstallsThroughTheMirror(t *testing.T) {
	npm, err := exec.LookPath("npm")
	if err != nil {
		t.Skip("npm not on PATH")
	}
	h := newHarness(t, nil)
	tgz := npmTarball(t, "left-pad", "1.3.0")
	_, tarPath := h.npmPackage("left-pad", "1.3.0", tgz, sri512(tgz))
	work := t.TempDir()
	if err := os.WriteFile(filepath.Join(work, "package.json"), []byte(`{"name":"probe","version":"1.0.0","dependencies":{"left-pad":"1.3.0"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(npm, "install", "--no-audit", "--no-fund", "--ignore-scripts")
	cmd.Dir = work
	cmd.Env = []string{
		"PATH=" + os.Getenv("PATH"), "HOME=" + work,
		"npm_config_registry=" + h.ts.URL + "/npm/",
		"npm_config_cache=" + filepath.Join(work, ".npm"),
		"npm_config_update_notifier=false",
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("npm install: %v\n%s", err, out)
	}
	got, err := os.ReadFile(filepath.Join(work, "node_modules", "left-pad", "index.js"))
	if err != nil || string(got) != "module.exports = 42;\n" {
		t.Fatalf("installed package: %q %v", got, err)
	}
	if h.npm.n(tarPath) != 1 || !h.cached("npm/left-pad/-/left-pad-1.3.0.tgz") {
		t.Fatal("the tarball did not come through the mirror's cache")
	}
}
