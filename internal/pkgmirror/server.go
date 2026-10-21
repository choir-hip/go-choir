// Package pkgmirror is the host-side package mirror engineering capsules
// reach through their L2 egress route mirror.choir.internal (station FP10,
// docs/definitions/choir-launch-fp10-net02-mirrors-2026-10-21.md). It
// follows the Go module proxy's pattern (internal/goproxy): plain HTTP to the
// capsule, GET and HEAD only, a path grammar per protocol, fixed upstreams
// over TLS, and an append-only cache that admits an artifact only after it
// matches the hash its registry publishes. Each ecosystem is off unless it is
// named in Config.Ecosystems.
package pkgmirror

import (
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/blake2b"
)

// Ecosystems, as named in Config.Ecosystems and in each path's first segment.
const (
	EcosystemNPM    = "npm"
	EcosystemPyPI   = "pypi"
	EcosystemCrates = "crates"
)

// Default upstreams. They are the only hosts each ecosystem fetches from,
// and the only hosts its redirects may reach.
const (
	DefaultNPM            = "https://registry.npmjs.org"
	DefaultPyPIIndex      = "https://pypi.org"
	DefaultPyPIFiles      = "https://files.pythonhosted.org"
	DefaultCratesIndex    = "https://index.crates.io"
	DefaultCratesDownload = "https://static.crates.io"
)

// Defaults for the Node B service.
const (
	DefaultMaxBytes = 16 << 30
	DefaultFileCap  = 2 << 30
	DefaultMetaCap  = 128 << 20

	// CapsuleHeader carries the capsule id the guest egress proxy stamps on
	// routed requests. It is attribution for the log, never authority.
	CapsuleHeader = "X-Choir-Capsule"

	maxRedirects    = 5
	artifactTimeout = 30 * time.Minute
	metaTimeout     = 2 * time.Minute
)

// Request outcomes in the log.
const (
	OutcomeHit           = "hit"
	OutcomeFetched       = "fetched"
	OutcomePassthrough   = "passthrough"
	OutcomeLocal         = "local"
	OutcomeRefused       = "refused"
	OutcomeNotFound      = "not_found"
	OutcomeUpstreamError = "upstream_error"
	OutcomeHashMismatch  = "hash_mismatch"
	OutcomeUnverifiable  = "unverifiable"
)

// Record is one request in the service log.
type Record struct {
	Time       time.Time `json:"time"`
	Event      string    `json:"event"`
	Ecosystem  string    `json:"ecosystem,omitempty"`
	Source     string    `json:"source,omitempty"`
	Capsule    string    `json:"capsule,omitempty"`
	Path       string    `json:"path"`
	Outcome    string    `json:"outcome"`
	Status     int       `json:"status"`
	Bytes      int64     `json:"bytes"`
	DurationMS int64     `json:"duration_ms"`
	Detail     string    `json:"detail,omitempty"`
}

// Upstreams overrides the default upstream bases (tests).
type Upstreams struct {
	NPM            string
	PyPIIndex      string
	PyPIFiles      string
	CratesIndex    string
	CratesDownload string
}

// Config configures a Server.
type Config struct {
	CacheDir string
	MaxBytes int64
	// PublicBase is the base URL capsules use for this service (the egress
	// route, http://mirror.choir.internal); rewritten index links and the
	// crates config name it.
	PublicBase string
	// Ecosystems lists the ecosystems served; any other prefix answers 404.
	Ecosystems []string
	Upstreams  Upstreams
	// FileCap bounds one artifact, MetaCap one metadata document.
	FileCap int64
	MetaCap int64
	// PerSourceFetches and TotalFetches bound concurrent upstream fetches.
	PerSourceFetches int
	TotalFetches     int
	// Transport overrides the upstream transport (tests).
	Transport http.RoundTripper
	// Record receives one record per request; logs JSON when nil.
	Record func(Record)
}

// Server is the mirror handler.
type Server struct {
	cache      *diskCache
	publicBase string
	enabled    map[string]bool
	fileCap    int64
	metaCap    int64
	record     func(Record)

	npm, pypiIndex, pypiFiles, cratesIndex, cratesDL upstreamBase

	perSource int
	total     chan struct{}
	srcMu     sync.Mutex
	sources   map[string]chan struct{}

	flightMu sync.Mutex
	flights  map[string]*flight
}

// upstreamBase is one fixed upstream and the client that may reach only it.
type upstreamBase struct {
	base   *url.URL
	client *http.Client
}

type flight struct {
	done    chan struct{}
	err     error
	waiters int
}

// New opens the cache and builds the handler.
func New(cfg Config) (*Server, error) {
	if strings.TrimSpace(cfg.CacheDir) == "" {
		return nil, errors.New("pkgmirror: cache directory is required")
	}
	public, err := parseBase(cfg.PublicBase)
	if err != nil {
		return nil, fmt.Errorf("pkgmirror: public base: %w", err)
	}
	if cfg.MaxBytes == 0 {
		cfg.MaxBytes = DefaultMaxBytes
	}
	if cfg.FileCap <= 0 {
		cfg.FileCap = DefaultFileCap
	}
	if cfg.MetaCap <= 0 {
		cfg.MetaCap = DefaultMetaCap
	}
	if cfg.PerSourceFetches <= 0 {
		cfg.PerSourceFetches = 4
	}
	if cfg.TotalFetches <= 0 {
		cfg.TotalFetches = 16
	}
	enabled := map[string]bool{}
	for _, e := range cfg.Ecosystems {
		switch e {
		case EcosystemNPM, EcosystemPyPI, EcosystemCrates:
			enabled[e] = true
		default:
			return nil, fmt.Errorf("pkgmirror: unknown ecosystem %q", e)
		}
	}
	transport := cfg.Transport
	if transport == nil {
		t := http.DefaultTransport.(*http.Transport).Clone()
		t.Proxy = nil
		t.ResponseHeaderTimeout = time.Minute
		transport = t
	}
	up := func(raw, def string) (upstreamBase, error) {
		if raw == "" {
			raw = def
		}
		u, err := parseBase(raw)
		if err != nil {
			return upstreamBase{}, err
		}
		return upstreamBase{base: u, client: &http.Client{Transport: transport, CheckRedirect: redirectPolicy(u)}}, nil
	}
	s := &Server{
		publicBase: public.String(),
		enabled:    enabled,
		fileCap:    cfg.FileCap,
		metaCap:    cfg.MetaCap,
		record:     cfg.Record,
		perSource:  cfg.PerSourceFetches,
		total:      make(chan struct{}, cfg.TotalFetches),
		sources:    map[string]chan struct{}{},
		flights:    map[string]*flight{},
	}
	for _, b := range []struct {
		dst      *upstreamBase
		raw, def string
	}{
		{&s.npm, cfg.Upstreams.NPM, DefaultNPM},
		{&s.pypiIndex, cfg.Upstreams.PyPIIndex, DefaultPyPIIndex},
		{&s.pypiFiles, cfg.Upstreams.PyPIFiles, DefaultPyPIFiles},
		{&s.cratesIndex, cfg.Upstreams.CratesIndex, DefaultCratesIndex},
		{&s.cratesDL, cfg.Upstreams.CratesDownload, DefaultCratesDownload},
	} {
		if *b.dst, err = up(b.raw, b.def); err != nil {
			return nil, err
		}
	}
	// Artifacts live under files/; the root holds nothing else of ours.
	cache, err := openDiskCache(cacheFile(cfg.CacheDir, "files"), cfg.MaxBytes)
	if err != nil {
		return nil, fmt.Errorf("pkgmirror: open cache: %w", err)
	}
	s.cache = cache
	if s.record == nil {
		s.record = func(r Record) {
			line, _ := json.Marshal(r)
			log.Printf("pkgmirror: %s", line)
		}
	}
	return s, nil
}

func parseBase(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimRight(raw, "/"))
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return nil, fmt.Errorf("pkgmirror: bad base URL %q", raw)
	}
	return u, nil
}

// redirectPolicy follows a bounded number of redirects, each only to the
// upstream's own host over its own scheme or https.
func redirectPolicy(base *url.URL) func(*http.Request, []*http.Request) error {
	host := strings.ToLower(base.Host)
	return func(req *http.Request, via []*http.Request) error {
		if len(via) >= maxRedirects {
			return errors.New("too many redirects")
		}
		if req.URL.Scheme != "https" && req.URL.Scheme != base.Scheme {
			return fmt.Errorf("redirect to %s refused", req.URL.Scheme)
		}
		if strings.ToLower(req.URL.Host) != host {
			return fmt.Errorf("redirect to %s refused", req.URL.Host)
		}
		return nil
	}
}

// Ecosystems lists the ecosystems this server serves, sorted.
func (s *Server) Ecosystems() []string {
	out := []string{}
	for e := range s.enabled {
		out = append(out, e)
	}
	sort.Strings(out)
	return out
}

// CacheBytes is the cache's current size.
func (s *Server) CacheBytes() int64 { return s.cache.bytes() }

// requestKind is what a checked request asks for.
type requestKind int

const (
	// kindArtifact is an immutable file: cached once verified.
	kindArtifact requestKind = iota
	// kindPassthrough is mutable metadata: streamed, never cached.
	kindPassthrough
	// kindPyPIIndex is a PEP 691 index, rewritten to this mirror.
	kindPyPIIndex
	// kindLocal is answered by the mirror itself.
	kindLocal
)

// request is a checked mirror request. Its upstream URL is built only from
// checked elements on a fixed base.
type request struct {
	eco      string
	kind     requestKind
	up       upstreamBase
	upPath   string // escaped path under up.base, leading slash
	accept   string // Accept header sent upstream
	ctype    string // Content-Type served
	cacheRel string // artifacts only
	// expect returns the hashes the registry publishes for an artifact; an
	// artifact with none is not served.
	expect func(ctx context.Context) ([]expectedHash, error)
	local  []byte // kindLocal
}

func (r request) upstreamURL() string { return r.up.base.String() + r.upPath }

// expectedHash is one published digest of an artifact.
type expectedHash struct {
	algo string // sha512, sha256, sha1, blake2b-256
	sum  []byte
}

func newHash(algo string) hash.Hash {
	switch algo {
	case "sha512":
		return sha512.New()
	case "sha256":
		return sha256.New()
	case "sha1":
		return sha1.New()
	case "blake2b-256":
		h, _ := blake2b.New256(nil)
		return h
	}
	return nil
}

// errRefused is a path no enabled grammar admits.
var errRefused = errors.New("not a mirror path")

type upstreamStatusError struct {
	status int
	body   []byte
}

func (e *upstreamStatusError) Error() string {
	return fmt.Sprintf("upstream status %d", e.status)
}

type hashMismatchError struct{ what, algo string }

func (e *hashMismatchError) Error() string {
	return fmt.Sprintf("%s: upstream bytes do not match the published %s", e.what, e.algo)
}

type unverifiableError struct{ what, why string }

func (e *unverifiableError) Error() string {
	return fmt.Sprintf("%s: %s; not served", e.what, e.why)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	rec := Record{Event: "request", Path: r.URL.EscapedPath(), Capsule: r.Header.Get(CapsuleHeader)}
	rec.Source, _, _ = net.SplitHostPort(r.RemoteAddr)
	finish := func(outcome string, status int, n int64, detail string) {
		rec.Time, rec.Outcome, rec.Status, rec.Bytes, rec.Detail = start.UTC(), outcome, status, n, detail
		rec.DurationMS = time.Since(start).Milliseconds()
		s.record(rec)
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		finish(OutcomeRefused, http.StatusMethodNotAllowed, 0, "method "+r.Method)
		return
	}
	if r.URL.Path == "/healthz" {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "ecosystems": s.Ecosystems(), "cache_bytes": s.cache.bytes(), "max_bytes": s.cache.maxBytes})
		return
	}
	eco, rest, _ := strings.Cut(strings.TrimPrefix(rec.Path, "/"), "/")
	rec.Ecosystem = eco
	if eco != EcosystemNPM && eco != EcosystemPyPI && eco != EcosystemCrates {
		rec.Ecosystem = ""
		http.Error(w, "not found: this mirror serves /npm/, /pypi/ and /crates/", http.StatusNotFound)
		finish(OutcomeRefused, http.StatusNotFound, 0, "unknown prefix")
		return
	}
	if !s.enabled[eco] {
		http.Error(w, "not found: the "+eco+" mirror is not enabled on this host", http.StatusNotFound)
		finish(OutcomeRefused, http.StatusNotFound, 0, "ecosystem not enabled")
		return
	}
	var req request
	err := errRefused
	if r.URL.RawQuery == "" && r.URL.Fragment == "" && !strings.ContainsAny(rest, "\\\x00") {
		switch eco {
		case EcosystemNPM:
			req, err = s.parseNPM(rest)
		case EcosystemPyPI:
			req, err = s.parsePyPI(rest)
		case EcosystemCrates:
			req, err = s.parseCrates(rest)
		}
	}
	if err != nil {
		http.Error(w, "not found: not a "+eco+" mirror path", http.StatusNotFound)
		finish(OutcomeRefused, http.StatusNotFound, 0, err.Error())
		return
	}
	switch req.kind {
	case kindLocal:
		w.Header().Set("Content-Type", req.ctype)
		w.Header().Set("Content-Length", fmt.Sprint(len(req.local)))
		if r.Method == http.MethodGet {
			_, _ = w.Write(req.local)
		}
		finish(OutcomeLocal, http.StatusOK, int64(len(req.local)), "")
	case kindArtifact:
		s.serveArtifact(w, r, req, &rec, finish)
	case kindPassthrough:
		s.servePassthrough(w, r, req, &rec, finish)
	case kindPyPIIndex:
		s.servePyPIIndex(w, r, req, &rec, finish)
	}
}

type finishFunc func(outcome string, status int, n int64, detail string)

func (s *Server) serveArtifact(w http.ResponseWriter, r *http.Request, req request, rec *Record, finish finishFunc) {
	path, hit := s.cache.lookup(req.cacheRel)
	outcome := OutcomeHit
	if !hit {
		release, err := s.acquire(r.Context(), rec.Source)
		if err != nil {
			http.Error(w, "request cancelled", http.StatusServiceUnavailable)
			finish(OutcomeUpstreamError, http.StatusServiceUnavailable, 0, err.Error())
			return
		}
		path, err = s.ensure(req)
		release()
		if err != nil {
			status, outcome, body := failure(err)
			http.Error(w, body, status)
			finish(outcome, status, 0, err.Error())
			return
		}
		outcome = OutcomeFetched
	}
	n, err := serveFile(w, r, path, req.ctype)
	if err != nil {
		finish(OutcomeUpstreamError, http.StatusInternalServerError, n, err.Error())
		return
	}
	finish(outcome, http.StatusOK, n, "")
}

// servePassthrough streams mutable metadata from the upstream, capped. A body
// past the cap that announced its length is refused before any byte; one
// that did not is cut by aborting the connection, so the client never takes
// a truncated document for a whole one.
func (s *Server) servePassthrough(w http.ResponseWriter, r *http.Request, req request, rec *Record, finish finishFunc) {
	release, err := s.acquire(r.Context(), rec.Source)
	if err != nil {
		http.Error(w, "request cancelled", http.StatusServiceUnavailable)
		finish(OutcomeUpstreamError, http.StatusServiceUnavailable, 0, err.Error())
		return
	}
	defer release()
	ctx, cancel := context.WithTimeout(r.Context(), metaTimeout)
	defer cancel()
	resp, err := s.get(ctx, req.up, req.upPath, req.accept)
	if err == nil && resp.ContentLength > s.metaCap {
		resp.Body.Close()
		err = errTooLarge
	}
	if err != nil {
		status, outcome, body := failure(err)
		http.Error(w, body, status)
		finish(outcome, status, 0, err.Error())
		return
	}
	defer resp.Body.Close()
	ctype := req.ctype
	if up := resp.Header.Get("Content-Type"); req.eco == EcosystemNPM && strings.HasPrefix(up, "application/") && !strings.ContainsAny(up, "\r\n") {
		ctype = up
	}
	w.Header().Set("Content-Type", ctype)
	if resp.ContentLength >= 0 {
		w.Header().Set("Content-Length", fmt.Sprint(resp.ContentLength))
	}
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		finish(OutcomePassthrough, http.StatusOK, 0, "")
		return
	}
	n, err := copyCapped(w, resp.Body, s.metaCap)
	if err != nil {
		finish(OutcomeUpstreamError, http.StatusOK, n, err.Error())
		panic(http.ErrAbortHandler)
	}
	finish(OutcomePassthrough, http.StatusOK, n, "")
}

// failure maps a fetch error to the client status: upstream 404/410 pass
// through, a hash mismatch and an unverifiable artifact are 502 with the
// reason, anything else is 502.
func failure(err error) (int, string, string) {
	var se *upstreamStatusError
	if errors.As(err, &se) && (se.status == http.StatusNotFound || se.status == http.StatusGone) {
		return se.status, OutcomeNotFound, "not found upstream"
	}
	var he *hashMismatchError
	if errors.As(err, &he) {
		return http.StatusBadGateway, OutcomeHashMismatch, "SECURITY ERROR: " + he.Error()
	}
	var ue *unverifiableError
	if errors.As(err, &ue) {
		return http.StatusBadGateway, OutcomeUnverifiable, ue.Error()
	}
	return http.StatusBadGateway, OutcomeUpstreamError, "upstream fetch failed: " + err.Error()
}

// acquire takes one per-source and one global fetch slot.
func (s *Server) acquire(ctx context.Context, source string) (func(), error) {
	s.srcMu.Lock()
	sem, ok := s.sources[source]
	if !ok {
		sem = make(chan struct{}, s.perSource)
		s.sources[source] = sem
	}
	s.srcMu.Unlock()
	select {
	case sem <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	select {
	case s.total <- struct{}{}:
	case <-ctx.Done():
		<-sem
		return nil, ctx.Err()
	}
	return func() { <-s.total; <-sem }, nil
}

// ensure returns the cached path for an artifact, fetching it once however
// many requests wait for it. The fetch does not belong to any one request,
// so a caller that goes away does not cut it for the others.
func (s *Server) ensure(req request) (string, error) {
	if path, ok := s.cache.lookup(req.cacheRel); ok {
		return path, nil
	}
	s.flightMu.Lock()
	f, ok := s.flights[req.cacheRel]
	if !ok {
		f = &flight{done: make(chan struct{})}
		s.flights[req.cacheRel] = f
		go func() {
			f.err = s.fetchToCache(req)
			s.flightMu.Lock()
			delete(s.flights, req.cacheRel)
			s.flightMu.Unlock()
			close(f.done)
		}()
	}
	f.waiters++
	s.flightMu.Unlock()
	<-f.done
	if f.err != nil {
		return "", f.err
	}
	path, ok := s.cache.lookup(req.cacheRel)
	if !ok {
		return "", errors.New("cache entry vanished after fetch")
	}
	return path, nil
}

// fetchToCache learns the published hashes first, then streams the artifact
// into a temp file while hashing it, and links it into the cache only when
// every published hash matches.
func (s *Server) fetchToCache(req request) error {
	ctx, cancel := context.WithTimeout(context.Background(), artifactTimeout)
	defer cancel()
	want, err := req.expect(ctx)
	if err != nil {
		return err
	}
	if len(want) == 0 {
		return &unverifiableError{what: req.cacheRel, why: "the registry publishes no hash for it"}
	}
	hashes := make([]hash.Hash, len(want))
	writers := []io.Writer{}
	for i, e := range want {
		if hashes[i] = newHash(e.algo); hashes[i] == nil {
			return &unverifiableError{what: req.cacheRel, why: "unknown hash " + e.algo}
		}
		writers = append(writers, hashes[i])
	}
	resp, err := s.get(ctx, req.up, req.upPath, "")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.ContentLength > s.fileCap {
		return errTooLarge
	}
	tmp, err := s.cache.create(req.cacheRel)
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	n, err := copyCapped(io.MultiWriter(append(writers, tmp)...), resp.Body, s.fileCap)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		for i, e := range want {
			if got := hashes[i].Sum(nil); string(got) != string(e.sum) {
				err = &hashMismatchError{what: req.cacheRel, algo: e.algo}
				break
			}
		}
	}
	if err != nil {
		os.Remove(tmpName)
		return err
	}
	return s.cache.commit(tmpName, req.cacheRel, n)
}

// get fetches a checked path from a fixed upstream. Any status but 200 is an
// upstreamStatusError carrying a bounded body.
func (s *Server) get(ctx context.Context, up upstreamBase, escapedPath, accept string) (*http.Response, error) {
	hreq, err := http.NewRequestWithContext(ctx, http.MethodGet, up.base.String()+escapedPath, nil)
	if err != nil {
		return nil, err
	}
	hreq.Header.Set("User-Agent", "go-choir-pkgmirror")
	if accept != "" {
		hreq.Header.Set("Accept", accept)
	}
	resp, err := up.client.Do(hreq)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		resp.Body.Close()
		return nil, &upstreamStatusError{status: resp.StatusCode, body: body}
	}
	return resp, nil
}

// getMeta reads a whole metadata document, capped.
func (s *Server) getMeta(ctx context.Context, up upstreamBase, escapedPath, accept string) ([]byte, http.Header, error) {
	ctx, cancel := context.WithTimeout(ctx, metaTimeout)
	defer cancel()
	resp, err := s.get(ctx, up, escapedPath, accept)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	if resp.ContentLength > s.metaCap {
		return nil, nil, errTooLarge
	}
	var buf strings.Builder
	if _, err := copyCapped(&buf, resp.Body, s.metaCap); err != nil {
		return nil, nil, err
	}
	return []byte(buf.String()), resp.Header, nil
}

func serveFile(w http.ResponseWriter, r *http.Request, path, ctype string) (int64, error) {
	f, err := os.Open(path)
	if err != nil {
		http.Error(w, "cache read failed", http.StatusInternalServerError)
		return 0, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		http.Error(w, "cache read failed", http.StatusInternalServerError)
		return 0, err
	}
	w.Header().Set("Content-Type", ctype)
	http.ServeContent(w, r, "", info.ModTime(), f)
	return info.Size(), nil
}
