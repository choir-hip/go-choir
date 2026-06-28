// Package health provides shared health check utilities for go-choir services.
//
// It defines a lightweight, dependency-free set of primitives for reporting
// service liveness and readiness:
//
//   - Checker is the interface for a single dependency probe (Dolt, Qdrant,
//     Ollama, an LLM provider, etc.).
//   - HTTPChecker and TCPChecker are ready-to-use Checker implementations that
//     probe an HTTP or TCP endpoint with a short timeout.
//   - Aggregator runs a set of named Checkers, caches the result for a
//     configurable TTL (so /health/ready stays cheap to call), and reports an
//     overall Status of "ok", "degraded", or "unhealthy".
//   - LivenessHandler and ReadinessHandler are net/http handlers ready to mount
//     at /health and /health/ready respectively.
//
// Design invariants:
//
//   - Health endpoints must be lightweight. Readiness checks are cached and run
//     out-of-band; a request never blocks on a slow dependency unless the cache
//     is cold and refreshBlocking=true.
//   - Adding these endpoints must not disrupt existing service behavior. The
//     default /health handler registered by internal/server is unchanged; this
//     package only adds opt-in /health/ready handlers and reusable probes.
//   - No external dependencies.
package health

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

// Status is the coarse health status reported by a probe or aggregator.
type Status string

const (
	// StatusOK means the service or dependency is healthy.
	StatusOK Status = "ok"
	// StatusDegraded means the service is up but one or more dependencies are
	// unavailable. The service should still serve traffic, possibly with
	// reduced functionality.
	StatusDegraded Status = "degraded"
	// StatusUnhealthy means the service or a required dependency is down and
	// the service should not receive traffic.
	StatusUnhealthy Status = "unhealthy"
)

// Checker is a single dependency probe. Implementations must be cheap and
// side-effect free; a Check call should complete within a few seconds and must
// respect the supplied context deadline.
type Checker interface {
	// Name is the dependency identifier surfaced in readiness responses.
	Name() string
	// Check returns nil if the dependency is reachable, or an error describing
	// the failure otherwise. The error is included verbatim (truncated) in the
	// readiness payload for operators.
	Check(ctx context.Context) error
}

// CheckerFunc is a function adapter for the Checker interface.
type CheckerFunc struct {
	NameStr string
	Fn      func(ctx context.Context) error
}

// Name returns the checker name.
func (c CheckerFunc) Name() string { return c.NameStr }

// Check runs the configured probe function.
func (c CheckerFunc) Check(ctx context.Context) error { return c.Fn(ctx) }

// HTTPChecker probes an HTTP endpoint. A 2xx response is healthy; any other
// status or transport error is unhealthy. The probe uses a short timeout so a
// slow dependency cannot stall a readiness check.
type HTTPChecker struct {
	// NameStr is the dependency identifier.
	NameStr string
	// URL is the fully-qualified URL to GET.
	URL string
	// Timeout bounds a single probe. Defaults to 2s when zero.
	Timeout time.Duration
	// Client is an optional *http.Client. When nil a per-call client is built
	// from Timeout.
	Client *http.Client
}

// Name returns the checker name.
func (h HTTPChecker) Name() string { return h.NameStr }

// Check issues a GET against the configured URL and returns nil on a 2xx
// response.
func (h HTTPChecker) Check(ctx context.Context) error {
	timeout := h.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	client := h.Client
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.URL, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return nil
}

// TCPChecker probes a TCP address by attempting a dial. This is used for
// dependencies that do not expose an HTTP health endpoint (e.g. a raw Dolt
// SQL socket). The dial is bounded by Timeout.
type TCPChecker struct {
	// NameStr is the dependency identifier.
	NameStr string
	// Address is the host:port to dial.
	Address string
	// Timeout bounds a single dial. Defaults to 2s when zero.
	Timeout time.Duration
}

// Name returns the checker name.
func (t TCPChecker) Name() string { return t.NameStr }

// Check dials the configured address and returns nil on success.
func (t TCPChecker) Check(ctx context.Context) error {
	timeout := t.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	d := net.Dialer{Timeout: timeout}
	conn, err := d.DialContext(ctx, "tcp", t.Address)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	_ = conn.Close()
	return nil
}

// DependencyResult is the outcome of a single Checker within an Aggregator.
type DependencyResult struct {
	Status    Status  `json:"status"`
	Error     string  `json:"error,omitempty"`
	LatencyMs float64 `json:"latency_ms,omitempty"`
}

// ReadinessResponse is the JSON body returned by ReadinessHandler.
type ReadinessResponse struct {
	Status      string                      `json:"status"`
	Service     string                      `json:"service"`
	CheckedAt   time.Time                   `json:"checked_at"`
	Cached      bool                        `json:"cached"`
	Dependencies map[string]DependencyResult `json:"dependencies,omitempty"`
}

// Aggregator runs a set of named Checkers and caches the aggregated result for
// a TTL so readiness probes stay lightweight even under load. The zero value
// is not usable; use NewAggregator.
type Aggregator struct {
	serviceName  string
	checkers     []Checker
	ttl          time.Duration
	checkTimeout time.Duration

	mu          sync.Mutex
	cached      *ReadinessResponse
	cachedAt    time.Time
	refreshing  bool
	refreshDone chan struct{}
}

// NewAggregator returns an Aggregator for the given service that runs the
// provided checkers. The cached readiness result is held for ttl before a
// background refresh is triggered. A ttl of zero disables caching (every call
// runs the checks synchronously).
func NewAggregator(serviceName string, ttl time.Duration, checkers ...Checker) *Aggregator {
	return &Aggregator{
		serviceName:  serviceName,
		checkers:     append([]Checker(nil), checkers...),
		ttl:          ttl,
		checkTimeout: 3 * time.Second,
	}
}

// SetCheckTimeout overrides the per-checker timeout (default 3s). This must be
// called before the first refresh.
func (a *Aggregator) SetCheckTimeout(d time.Duration) {
	if d > 0 {
		a.checkTimeout = d
	}
}

// snapshot returns the current cached response, or nil if none.
func (a *Aggregator) snapshot() *ReadinessResponse {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cached == nil {
		return nil
	}
	out := *a.cached
	out.Dependencies = copyDeps(a.cached.Dependencies)
	return &out
}

func copyDeps(in map[string]DependencyResult) map[string]DependencyResult {
	if in == nil {
		return nil
	}
	out := make(map[string]DependencyResult, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// runChecks executes every checker with a per-check timeout and returns the
// aggregated response. The overall status is "degraded" if any dependency is
// unhealthy but at least one is healthy, and "unhealthy" only when every
// dependency failed. With a single dependency, "degraded" and "unhealthy"
// collapse to the dependency's status.
func (a *Aggregator) runChecks() *ReadinessResponse {
	deps := make(map[string]DependencyResult, len(a.checkers))
	healthy, total := 0, len(a.checkers)
	for _, c := range a.checkers {
		// Per-check timeout keeps one slow dependency from dominating.
		ctx, cancel := context.WithTimeout(context.Background(), a.checkTimeout)
		start := time.Now()
		err := c.Check(ctx)
		cancel()
		latency := float64(time.Since(start).Microseconds()) / 1000.0
		if err != nil {
			msg := err.Error()
			if len(msg) > 200 {
				msg = msg[:200] + "..."
			}
			deps[c.Name()] = DependencyResult{Status: StatusUnhealthy, Error: msg, LatencyMs: latency}
			continue
		}
		deps[c.Name()] = DependencyResult{Status: StatusOK, LatencyMs: latency}
		healthy++
	}

	overall := StatusOK
	switch {
	case healthy == total:
		overall = StatusOK
	case healthy == 0:
		overall = StatusUnhealthy
	default:
		overall = StatusDegraded
	}
	if total == 0 {
		overall = StatusOK
	}

	return &ReadinessResponse{
		Status:       string(overall),
		Service:      a.serviceName,
		CheckedAt:    time.Now().UTC(),
		Dependencies: deps,
	}
}

// RefreshIfStale forces a synchronous refresh when caching is disabled, or
// triggers a background refresh when the cache is stale. It is safe to call
// concurrently. Returns the freshest cached snapshot available and a cached
// flag indicating whether the response came from cache (true) or was freshly
// computed (false).
func (a *Aggregator) RefreshIfStale() *ReadinessResponse {
	if a.ttl <= 0 {
		resp := a.runChecks()
		resp.Cached = false
		a.mu.Lock()
		a.cached = resp
		a.cachedAt = time.Now()
		a.mu.Unlock()
		return resp
	}
	fresh := a.maybeRefresh()
	resp := a.snapshot()
	if resp == nil {
		// No cache yet and a background refresh was just scheduled; return a
		// neutral ok so the first concurrent caller does not block.
		resp = &ReadinessResponse{Status: string(StatusOK), Service: a.serviceName, CheckedAt: time.Now().UTC()}
	}
	resp.Cached = !fresh
	return resp
}

// maybeRefresh triggers a refresh when the cache is stale. It returns true when
// a synchronous refresh ran (fresh result), and false when the response will
// come from cache (either fresh cache or a background refresh in flight).
func (a *Aggregator) maybeRefresh() bool {
	a.mu.Lock()
	if a.cached != nil && time.Since(a.cachedAt) < a.ttl {
		a.mu.Unlock()
		return false
	}
	if a.refreshing {
		a.mu.Unlock()
		return false
	}
	// Cold cache: refresh synchronously so first response is populated.
	cold := a.cached == nil
	a.refreshing = true
	a.mu.Unlock()

	if cold {
		a.doRefresh()
		return true
	}
	go a.doRefresh()
	return false
}

func (a *Aggregator) doRefresh() {
	resp := a.runChecks()
	a.mu.Lock()
	a.cached = resp
	a.cachedAt = time.Now()
	a.refreshing = false
	a.mu.Unlock()
}

// LivenessResponse is the JSON body returned by LivenessHandler.
type LivenessResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

// LivenessHandler returns an http.HandlerFunc that reports simple liveness:
// the process is running. It performs no dependency checks and is safe to call
// on every request.
func LivenessHandler(serviceName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeHealthJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		writeHealthJSON(w, http.StatusOK, LivenessResponse{Status: string(StatusOK), Service: serviceName})
	}
}

// ReadinessHandler returns an http.HandlerFunc that reports readiness by
// running the Aggregator's dependency checks. The response is served from
// cache when available so the endpoint stays lightweight; a stale cache
// triggers a background refresh.
//
// The HTTP status code reflects readiness: 200 when ok or degraded, 503 when
// unhealthy. Degraded services remain routable because they can still serve
// reduced functionality; only fully unhealthy services return 503.
func ReadinessHandler(serviceName string, agg *Aggregator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeHealthJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		resp := agg.RefreshIfStale()
		if resp == nil {
			resp = &ReadinessResponse{Status: string(StatusOK), Service: serviceName, CheckedAt: time.Now().UTC()}
		}
		code := http.StatusOK
		if Status(resp.Status) == StatusUnhealthy {
			code = http.StatusServiceUnavailable
		}
		writeHealthJSON(w, code, resp)
	}
}

func writeHealthJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
