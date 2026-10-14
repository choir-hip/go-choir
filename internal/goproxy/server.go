package goproxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// Defaults for the Node B service.
const (
	DefaultUpstream      = "https://proxy.golang.org"
	DefaultSumDBUpstream = "https://" + SumDBName
	DefaultMaxBytes      = 8 << 30

	// CapsuleHeader carries the capsule id the guest egress proxy stamps on
	// routed requests. It is attribution for the log, never authority.
	CapsuleHeader = "X-Choir-Capsule"

	maxZipBytes   = 500 << 20
	maxOtherBytes = 16 << 20
	maxRedirects  = 5
	fetchTimeout  = 10 * time.Minute
)

// DefaultRedirectHosts are the hosts a module request may be redirected to:
// proxy.golang.org answers zips with a redirect to its storage bucket.
var DefaultRedirectHosts = []string{"proxy.golang.org", "storage.googleapis.com"}

// Request outcomes in the log.
const (
	OutcomeHit           = "hit"
	OutcomeFetched       = "fetched"
	OutcomePassthrough   = "passthrough"
	OutcomeRefused       = "refused"
	OutcomeNotFound      = "not_found"
	OutcomeUpstreamError = "upstream_error"
	OutcomeHashMismatch  = "hash_mismatch"
)

// Record is one request in the service log.
type Record struct {
	Time       time.Time `json:"time"`
	Event      string    `json:"event"`
	Source     string    `json:"source,omitempty"`
	Capsule    string    `json:"capsule,omitempty"`
	Path       string    `json:"path"`
	Outcome    string    `json:"outcome"`
	Status     int       `json:"status"`
	Bytes      int64     `json:"bytes"`
	DurationMS int64     `json:"duration_ms"`
	Detail     string    `json:"detail,omitempty"`
}

// Config configures a Server.
type Config struct {
	CacheDir      string
	MaxBytes      int64
	Upstream      string   // GOPROXY upstream base; DefaultUpstream when empty
	SumDBUpstream string   // checksum database base; DefaultSumDBUpstream when empty
	RedirectHosts []string // hosts (host or host:port) module redirects may reach
	// KnownSums are go.sum hashes ("mod ver" or "mod ver/go.mod" -> h1);
	// a fetched file with an entry here must match it.
	KnownSums map[string]string
	// PerSourceFetches and TotalFetches bound concurrent upstream fetches.
	PerSourceFetches int
	TotalFetches     int
	// Transport overrides the upstream transport (tests).
	Transport http.RoundTripper
	// Record receives one record per request; logs JSON when nil.
	Record func(Record)
}

// Server is the module proxy handler.
type Server struct {
	cache        *diskCache
	upstream     *url.URL
	sumdb        *url.URL
	moduleClient *http.Client
	sumdbClient  *http.Client
	record       func(Record)

	knownMu sync.RWMutex
	known   map[string]string

	perSource int
	total     chan struct{}
	srcMu     sync.Mutex
	sources   map[string]chan struct{}

	flightMu sync.Mutex
	flights  map[string]*flight
}

type flight struct {
	done chan struct{}
	err  error
}

// New opens the cache and builds the handler.
func New(cfg Config) (*Server, error) {
	if strings.TrimSpace(cfg.CacheDir) == "" {
		return nil, errors.New("goproxy: cache directory is required")
	}
	if cfg.Upstream == "" {
		cfg.Upstream = DefaultUpstream
	}
	if cfg.SumDBUpstream == "" {
		cfg.SumDBUpstream = DefaultSumDBUpstream
	}
	if cfg.RedirectHosts == nil {
		cfg.RedirectHosts = DefaultRedirectHosts
	}
	if cfg.PerSourceFetches <= 0 {
		cfg.PerSourceFetches = 4
	}
	if cfg.TotalFetches <= 0 {
		cfg.TotalFetches = 16
	}
	upstream, err := parseBase(cfg.Upstream)
	if err != nil {
		return nil, err
	}
	sumdb, err := parseBase(cfg.SumDBUpstream)
	if err != nil {
		return nil, err
	}
	cache, err := openDiskCache(cfg.CacheDir, cfg.MaxBytes)
	if err != nil {
		return nil, fmt.Errorf("goproxy: open cache: %w", err)
	}
	transport := cfg.Transport
	if transport == nil {
		t := http.DefaultTransport.(*http.Transport).Clone()
		t.Proxy = nil
		t.ResponseHeaderTimeout = time.Minute
		transport = t
	}
	s := &Server{
		cache:     cache,
		upstream:  upstream,
		sumdb:     sumdb,
		known:     map[string]string{},
		record:    cfg.Record,
		perSource: cfg.PerSourceFetches,
		total:     make(chan struct{}, cfg.TotalFetches),
		sources:   map[string]chan struct{}{},
		flights:   map[string]*flight{},
	}
	moduleHosts := append([]string{upstream.Host}, cfg.RedirectHosts...)
	s.moduleClient = &http.Client{Transport: transport, CheckRedirect: redirectPolicy(upstream.Scheme, moduleHosts)}
	s.sumdbClient = &http.Client{Transport: transport, CheckRedirect: redirectPolicy(sumdb.Scheme, []string{sumdb.Host})}
	s.addKnown(cfg.KnownSums)
	if s.record == nil {
		s.record = func(r Record) {
			line, _ := json.Marshal(r)
			log.Printf("goproxy: %s", line)
		}
	}
	return s, nil
}

func parseBase(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimRight(raw, "/"))
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.RawQuery != "" || u.User != nil {
		return nil, fmt.Errorf("goproxy: bad upstream %q", raw)
	}
	return u, nil
}

// redirectPolicy follows a bounded number of redirects, each only to a host
// on the list and only over https (or the upstream's own scheme in tests).
func redirectPolicy(baseScheme string, hosts []string) func(*http.Request, []*http.Request) error {
	allowed := map[string]bool{}
	for _, h := range hosts {
		allowed[strings.ToLower(h)] = true
	}
	return func(req *http.Request, via []*http.Request) error {
		if len(via) >= maxRedirects {
			return errors.New("too many redirects")
		}
		if req.URL.Scheme != "https" && req.URL.Scheme != baseScheme {
			return fmt.Errorf("redirect to %s refused", req.URL.Scheme)
		}
		if !allowed[strings.ToLower(req.URL.Host)] {
			return fmt.Errorf("redirect to %s refused", req.URL.Host)
		}
		return nil
	}
}

// addKnown merges go.sum hashes into the set fetched files must match.
func (s *Server) addKnown(sums map[string]string) {
	s.knownMu.Lock()
	defer s.knownMu.Unlock()
	for k, v := range sums {
		s.known[k] = v
	}
}

// CacheBytes is the cache's current size.
func (s *Server) CacheBytes() int64 { return s.cache.bytes() }

type upstreamStatusError struct {
	status int
	body   []byte
}

func (e *upstreamStatusError) Error() string {
	return fmt.Sprintf("upstream status %d", e.status)
}

type hashMismatchError struct{ key, want, got string }

func (e *hashMismatchError) Error() string {
	return fmt.Sprintf("%s: go.sum has %s, upstream served %s", e.key, e.want, e.got)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	rec := Record{Event: "request", Path: r.URL.Path, Capsule: r.Header.Get(CapsuleHeader)}
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
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "cache_bytes": s.cache.bytes(), "max_bytes": s.cache.maxBytes})
		return
	}
	req, err := parseRequest(r.URL.Path)
	if err != nil {
		http.Error(w, "not found: this proxy serves only Go module and sum.golang.org paths", http.StatusNotFound)
		finish(OutcomeRefused, http.StatusNotFound, 0, err.Error())
		return
	}
	if req.kind == kindSumDBSupported {
		w.WriteHeader(http.StatusOK)
		finish(OutcomeHit, http.StatusOK, 0, "")
		return
	}
	if req.cacheable() {
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
				status, outcome, body := s.failure(err)
				http.Error(w, body, status)
				finish(outcome, status, 0, err.Error())
				return
			}
			outcome = OutcomeFetched
		}
		n, err := serveFile(w, r, path, contentType(req.kind))
		if err != nil {
			finish(OutcomeUpstreamError, http.StatusInternalServerError, n, err.Error())
			return
		}
		finish(outcome, http.StatusOK, n, "")
		return
	}
	release, err := s.acquire(r.Context(), rec.Source)
	if err != nil {
		http.Error(w, "request cancelled", http.StatusServiceUnavailable)
		finish(OutcomeUpstreamError, http.StatusServiceUnavailable, 0, err.Error())
		return
	}
	body, err := s.passthrough(r.Context(), req)
	release()
	if err != nil {
		status, outcome, msg := s.failure(err)
		http.Error(w, msg, status)
		finish(outcome, status, 0, err.Error())
		return
	}
	w.Header().Set("Content-Type", contentType(req.kind))
	w.Header().Set("Content-Length", fmt.Sprint(len(body)))
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodGet {
		_, _ = w.Write(body)
	}
	finish(OutcomePassthrough, http.StatusOK, int64(len(body)), "")
}

// failure maps a fetch error to the client status: upstream 404/410 pass
// through with their text, a hash mismatch and anything else are 502.
func (s *Server) failure(err error) (int, string, string) {
	var se *upstreamStatusError
	if errors.As(err, &se) && (se.status == http.StatusNotFound || se.status == http.StatusGone) {
		return se.status, OutcomeNotFound, strings.TrimSpace(string(se.body))
	}
	var he *hashMismatchError
	if errors.As(err, &he) {
		return http.StatusBadGateway, OutcomeHashMismatch, "SECURITY ERROR: " + he.Error()
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

// ensure returns the cached path for a cacheable request, fetching it once
// however many requests wait for it. The fetch does not belong to any one
// request, so a caller that goes away does not cut it for the others.
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

func (s *Server) fetchToCache(req request) error {
	ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
	defer cancel()
	resp, err := s.get(ctx, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	tmp, err := s.cache.create(req.cacheRel)
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	n, err := copyCapped(tmp, resp.Body, sizeCap(req.kind))
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = s.verify(req, tmpName)
	}
	if err != nil {
		os.Remove(tmpName)
		return err
	}
	return s.cache.commit(tmpName, req.cacheRel, n)
}

// verify checks a fetched .mod or .zip against the known go.sum hash.
func (s *Server) verify(req request, path string) error {
	key, ok := sumKey(req)
	if !ok {
		return nil
	}
	s.knownMu.RLock()
	want, ok := s.known[key]
	s.knownMu.RUnlock()
	if !ok {
		return nil
	}
	got, err := hashFile(req.kind, path)
	if err != nil {
		return fmt.Errorf("hash %s: %w", key, err)
	}
	if got != want {
		return &hashMismatchError{key: key, want: want, got: got}
	}
	return nil
}

func hashFile(kind requestKind, path string) (string, error) {
	if kind == kindZip {
		return hashZip(path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return hashGoMod(data)
}

func (s *Server) passthrough(ctx context.Context, req request) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	resp, err := s.get(ctx, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	if _, err := copyCapped(&buf, resp.Body, maxOtherBytes); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// get fetches the checked path from its fixed upstream. Any status but 200
// is an upstreamStatusError carrying a bounded body.
func (s *Server) get(ctx context.Context, req request) (*http.Response, error) {
	base, client := s.upstream, s.moduleClient
	if req.sumdb() {
		base, client = s.sumdb, s.sumdbClient
	}
	target := *base
	target.Path = base.Path + "/" + req.upstreamPath
	target.RawPath = ""
	hreq, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, err
	}
	hreq.Header.Set("User-Agent", "go-choir-goproxy")
	resp, err := client.Do(hreq)
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

func sizeCap(kind requestKind) int64 {
	if kind == kindZip {
		return maxZipBytes
	}
	return maxOtherBytes
}

func contentType(kind requestKind) string {
	switch kind {
	case kindInfo, kindLatest:
		return "application/json"
	case kindZip:
		return "application/zip"
	case kindSumDBTile:
		return "application/octet-stream"
	}
	return "text/plain; charset=utf-8"
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
