package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Request body caps (B7,
// docs/problems/gateway-request-bodies-unbounded-2026-10-22.md). Every body
// the gateway decodes is read through one cap per route class: a declared
// Content-Length over the cap is refused unread, a streamed body is cut one
// byte past it, and both answer 413 with a short reason.
//
// GATEWAY_BODY_CAPS sets the model routes' rollout: "observe" serves a model
// body over its limit (up to the ceiling) and logs would_refuse with the
// size; unset, "enforce" or anything else refuses it. Every other route
// enforces in every mode: no legitimate body comes near its cap.
// GATEWAY_BODY_LIMITS overrides limits, read once when the handler is built:
// "model=160MiB,search=1MiB" (bytes, or a KiB, MiB or GiB suffix).
//
// Two bounds hold in every mode (bunch 88). in_flight bounds the bytes of
// all bodies the gateway holds at once, process-wide, whatever the
// scheduler's mode (F1): a body that does not fit waits, and past the body
// deadline it is answered 429 with Retry-After, unread. A model body's bytes
// are held until its handler returns, since the decoded request lives that
// long. One caller (the authenticated computer, or the host caller) holds
// at most per_caller of in_flight (bunch 109, F-A), except that its first
// body always fits: a caller over its share waits behind its own bodies
// only, and every other caller's body goes ahead of it.
// GATEWAY_BODY_READ_TIMEOUT (default 60 s) bounds the wait for in_flight,
// which starts when decodeBody runs, on the model routes after the
// scheduler slot is granted (F2, F4); the read then gets its own window of
// the same length from the moment the body fits (bunch 109, F-C). A read
// cut by it is answered 503 with Retry-After.
const (
	// DefaultModelBodyLimit is the enforced model limit (F3, bunch 88; the
	// arithmetic is in the problem record): 20 images (MaxImagesPerRequest)
	// at the gateway's own 5 MiB downscale bound (MaxImageBytes) are 100 MiB,
	// 133.3 MiB as base64, plus a full 1M-token context (about 16 MiB of
	// JSON): about 150 MiB.
	DefaultModelBodyLimit int64 = 160 << 20
	// DefaultModelBodyCeiling is the media contract's own ceiling, 20 images
	// of 20 MiB as base64 plus a full context: the bound in observe mode.
	DefaultModelBodyCeiling int64 = 576 << 20
	// DefaultSearchBodyLimit: a search request is a query, a count, a depth.
	DefaultSearchBodyLimit int64 = 1 << 20
	// DefaultOperatorBodyLimit: one token, one address or one provider name.
	DefaultOperatorBodyLimit int64 = 64 << 10
	// DefaultBodyInFlight bounds all bodies held at once; never below the
	// model ceiling, so one ceiling body always fits.
	DefaultBodyInFlight int64 = 1 << 30
	// DefaultBodyPerCaller is one caller's share of in_flight, a quarter of
	// it (bunch 109, F-A; the reasoning is in the problem record). Unset,
	// per_caller follows in_flight/4; it is never above in_flight. A
	// caller's first body fits whatever its size, so one caller holds at
	// most max(per_caller, one body): 256 MiB in enforce (above the
	// 160 MiB model limit), 576 MiB in observe (one ceiling body), leaving
	// 768 MiB or 448 MiB for every other caller.
	DefaultBodyPerCaller = DefaultBodyInFlight / 4
	// DefaultBodyReadTimeout: 576 MiB at 100 Mbit/s is 48 s; the guest's
	// tap link is much faster.
	DefaultBodyReadTimeout = 60 * time.Second
	bodyBusyRetry          = 5 // seconds, the Retry-After of 429 and 503
)

// Route classes, which are also the GATEWAY_BODY_LIMITS keys and the
// /health body_caps keys. bodyCeilingModel is a limit, not a route.
const (
	bodyRouteModel     = "model"
	bodyCeilingModel   = "model_ceiling"
	bodyRouteSearch    = "search"
	bodyRouteSourceAPI = "source_api"
	bodyRouteJudgment  = "judgment"
	bodyRouteOperator  = "operator"
	// bodyInFlight is the limit on all bodies held at once, and the
	// refused key of bodies turned away by it; bodyPerCaller is one
	// caller's share of it, and the refused key of bodies turned away
	// because their caller held its share (bunch 116, F3);
	// bodyReadDeadline is the refused key of reads cut by the body
	// deadline.
	bodyInFlight     = "in_flight"
	bodyPerCaller    = "per_caller"
	bodyReadDeadline = "read_deadline"
)

const (
	BodyCapsObserve = "observe"
	BodyCapsEnforce = "enforce"
)

func defaultBodyLimits() map[string]int64 {
	return map[string]int64{
		bodyRouteModel:     DefaultModelBodyLimit,
		bodyCeilingModel:   DefaultModelBodyCeiling,
		bodyRouteSearch:    DefaultSearchBodyLimit,
		bodyRouteSourceAPI: sourceAPIMaxRequest,
		bodyRouteJudgment:  jevTransportMaxBodySize,
		bodyRouteOperator:  DefaultOperatorBodyLimit,
		bodyInFlight:       DefaultBodyInFlight,
		bodyPerCaller:      DefaultBodyPerCaller,
	}
}

type bodyCaps struct {
	mode        string
	limits      map[string]int64 // fixed once built
	readTimeout time.Duration
	budget      *bodyBudget

	mu          sync.Mutex
	refused     map[string]int64
	wouldRefuse map[string]int64
	maxSeen     map[string]int64
}

// defaultCaps serves a Handler built without a constructor (tests).
var defaultCaps = newBodyCaps(BodyCapsEnforce, defaultBodyLimits())

func newBodyCaps(mode string, limits map[string]int64) *bodyCaps {
	if limits[bodyInFlight] < limits[bodyCeilingModel] {
		limits[bodyInFlight] = limits[bodyCeilingModel]
	}
	if limits[bodyPerCaller] <= 0 {
		limits[bodyPerCaller] = limits[bodyInFlight] / 4
	}
	if limits[bodyPerCaller] > limits[bodyInFlight] {
		limits[bodyPerCaller] = limits[bodyInFlight]
	}
	return &bodyCaps{mode: mode, limits: limits, readTimeout: DefaultBodyReadTimeout,
		budget:  &bodyBudget{size: limits[bodyInFlight], share: limits[bodyPerCaller]},
		refused: map[string]int64{}, wouldRefuse: map[string]int64{}, maxSeen: map[string]int64{}}
}

// bodyCapsFromGetenv reads GATEWAY_BODY_CAPS and GATEWAY_BODY_LIMITS. An
// unreadable value is logged and the default kept, so a typo never removes
// a cap.
func bodyCapsFromGetenv(get func(string) string) *bodyCaps {
	mode := BodyCapsEnforce
	switch v := strings.ToLower(strings.TrimSpace(get("GATEWAY_BODY_CAPS"))); v {
	case "", BodyCapsEnforce:
	case BodyCapsObserve:
		mode = BodyCapsObserve
	default:
		log.Printf("gateway: GATEWAY_BODY_CAPS=%q is not observe or enforce; enforcing", v)
	}
	limits := defaultBodyLimits()
	perCallerSet := false
	for _, pair := range strings.Split(get("GATEWAY_BODY_LIMITS"), ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		key, value, _ := strings.Cut(pair, "=")
		key = strings.TrimSpace(key)
		if _, known := limits[key]; !known {
			log.Printf("gateway: GATEWAY_BODY_LIMITS names unknown route %q; ignored", key)
			continue
		}
		n, err := parseByteSize(value)
		if err != nil {
			log.Printf("gateway: GATEWAY_BODY_LIMITS %s=%q: %v; keeping %d", key, value, err, limits[key])
			continue
		}
		limits[key] = n
		perCallerSet = perCallerSet || key == bodyPerCaller
	}
	if !perCallerSet {
		limits[bodyPerCaller] = 0 // newBodyCaps: in_flight/4
	}
	if limits[bodyCeilingModel] < limits[bodyRouteModel] {
		limits[bodyCeilingModel] = limits[bodyRouteModel]
	}
	c := newBodyCaps(mode, limits)
	if v := strings.TrimSpace(get("GATEWAY_BODY_READ_TIMEOUT")); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			c.readTimeout = d
		} else {
			log.Printf("gateway: GATEWAY_BODY_READ_TIMEOUT=%q is not a positive duration; keeping %s", v, c.readTimeout)
		}
	}
	return c
}

func parseByteSize(v string) (int64, error) {
	v = strings.TrimSpace(v)
	mult := int64(1)
	for _, u := range []struct {
		suffix string
		mult   int64
	}{{"GiB", 1 << 30}, {"MiB", 1 << 20}, {"KiB", 1 << 10}} {
		if strings.HasSuffix(v, u.suffix) {
			v, mult = strings.TrimSpace(strings.TrimSuffix(v, u.suffix)), u.mult
			break
		}
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n <= 0 || n > (1<<40)/mult {
		return 0, errors.New("want a positive size in bytes, KiB, MiB or GiB")
	}
	return n * mult, nil
}

func formatByteSize(n int64) string {
	switch {
	case n >= 1<<20 && n%(1<<20) == 0:
		return fmt.Sprintf("%d MiB", n>>20)
	case n >= 1<<10 && n%(1<<10) == 0:
		return fmt.Sprintf("%d KiB", n>>10)
	default:
		return fmt.Sprintf("%d bytes", n)
	}
}

func (c *bodyCaps) String() string {
	keys := make([]string, 0, len(c.limits))
	for k := range c.limits {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = k + "=" + formatByteSize(c.limits[k])
	}
	return "mode=" + c.mode + " " + strings.Join(parts, ",") + " read_timeout=" + c.readTimeout.String()
}

func (h *Handler) caps() *bodyCaps {
	if h.bodyCaps == nil {
		return defaultCaps
	}
	return h.bodyCaps
}

// BodyCapsStatus is the startup log's view of the caps.
func (h *Handler) BodyCapsStatus() string { return h.caps().String() }

// errBodyTooLarge is a body over its route's cap; Error is the 413's reason.
type errBodyTooLarge struct {
	route string
	limit int64
}

func (e *errBodyTooLarge) Error() string {
	return fmt.Sprintf("request body too large: the %s limit is %s", e.route, formatByteSize(e.limit))
}

// errBodyRetry is a body turned away for now: no room in in_flight (429)
// or cut by the body deadline (503). Both carry Retry-After, and both are
// transient to the guest.
type errBodyRetry struct {
	status int
	msg    string
}

func (e *errBodyRetry) Error() string { return e.msg }

// bodyRefusal is the status for a decodeBody error the route does not
// answer with its own 400, setting Retry-After when the guest should come
// back; ok is false for a malformed body.
func bodyRefusal(w http.ResponseWriter, err error) (status int, ok bool) {
	var tooLarge *errBodyTooLarge
	if errors.As(err, &tooLarge) {
		return http.StatusRequestEntityTooLarge, true
	}
	var retry *errBodyRetry
	if errors.As(err, &retry) {
		w.Header().Set("Retry-After", strconv.Itoa(bodyBusyRetry))
		return retry.status, true
	}
	return 0, false
}

type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

// decodeBody decodes one JSON value from r's body into v under route's cap.
// It returns an *errBodyTooLarge (answer 413 with its Error) or the
// decoder's error (answer the route's own 400). strict refuses unknown
// fields. Its bytes are given back when it returns.
func (h *Handler) decodeBody(w http.ResponseWriter, r *http.Request, route, caller string, v any, strict bool) error {
	release, err := h.decodeBodyHeld(w, r, route, caller, v, strict)
	release()
	return err
}

// decodeBodyHeld is decodeBody that keeps the body's bytes counted against
// in_flight until the caller runs release (the model routes, whose decoded
// request lives for the whole call). release is never nil.
func (h *Handler) decodeBodyHeld(w http.ResponseWriter, r *http.Request, route, caller string, v any, strict bool) (func(), error) {
	noop := func() {}
	c := h.caps()
	limit := c.limits[route]
	readCap := limit
	observe := route == bodyRouteModel && c.mode == BodyCapsObserve
	if observe {
		readCap = c.limits[bodyCeilingModel]
	}
	if r.ContentLength > readCap {
		c.refuse(route, caller, fmt.Sprintf("content_length=%d", r.ContentLength), readCap)
		return noop, &errBodyTooLarge{route: route, limit: readCap}
	}
	deadline := time.Now().Add(c.readTimeout)
	want := readCap
	if r.ContentLength >= 0 {
		want = r.ContentLength
	}
	held, waited, err := c.budget.acquire(r.Context(), caller, want, deadline)
	if err != nil {
		if errors.Is(err, errBodyBudgetWait) {
			c.refuse(bodyInFlight, caller, fmt.Sprintf("route=%s bytes=%d waited=%s", route, want, waited.Round(time.Millisecond)), c.limits[bodyInFlight])
			return noop, &errBodyRetry{status: http.StatusTooManyRequests, msg: "the gateway is holding as many request bodies as it can; retry"}
		}
		if errors.Is(err, errBodyShareWait) {
			c.refuse(bodyPerCaller, caller, fmt.Sprintf("route=%s bytes=%d waited=%s", route, want, waited.Round(time.Millisecond)), c.limits[bodyPerCaller])
			return noop, &errBodyRetry{status: http.StatusTooManyRequests, msg: "this caller's request bodies already hold its share of the gateway; retry"}
		}
		return noop, err
	}
	if waited > 0 {
		log.Printf("gateway: body in_flight waited route=%s caller=%s bytes=%d wait=%s", route, caller, want, waited.Round(time.Millisecond))
	}
	var once sync.Once
	release := func() { once.Do(func() { c.budget.release(caller, held) }) }
	// Every exit but success gives the bytes back, a panic in the decode
	// too (bunch 109, F-B): the caller's defer is not registered yet.
	kept := false
	defer func() {
		if !kept {
			release()
		}
	}()
	// The body's own deadline: the server's ReadTimeout runs from the
	// request's start and a queued call may have spent it (F2). It runs
	// from the moment the body fits, so a wait for room does not eat the
	// read (F-C).
	if err := http.NewResponseController(w).SetReadDeadline(time.Now().Add(c.readTimeout)); err != nil {
		readDeadlineUnset.Do(func() {
			log.Printf("gateway: body read deadline not set (%v); a queued model call can hit the server's ReadTimeout", err)
		})
	}
	body := &countingReader{r: http.MaxBytesReader(w, r.Body, readCap)}
	dec := json.NewDecoder(body)
	if strict {
		dec.DisallowUnknownFields()
	}
	err = dec.Decode(v)
	c.seen(route, body.n)
	var maxBytes *http.MaxBytesError
	if errors.As(err, &maxBytes) {
		c.refuse(route, caller, fmt.Sprintf("bytes>%d", readCap), readCap)
		return noop, &errBodyTooLarge{route: route, limit: readCap}
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		c.refuse(bodyReadDeadline, caller, fmt.Sprintf("route=%s bytes=%d", route, body.n), int64(c.readTimeout/time.Second))
		return noop, &errBodyRetry{status: http.StatusServiceUnavailable, msg: "request body not received within " + c.readTimeout.String() + "; retry"}
	}
	if err != nil {
		return noop, err
	}
	// Hold what was read, not what a chunked body could have been.
	if body.n < held {
		c.budget.release(caller, held-body.n)
		held = body.n
	}
	if err == nil && observe && body.n > limit {
		c.mu.Lock()
		c.wouldRefuse[route]++
		c.mu.Unlock()
		log.Printf("gateway: body cap would_refuse route=%s caller=%s bytes=%d limit=%d", route, caller, body.n, limit)
	}
	kept = true
	return release, nil
}

// readDeadlineUnset logs once that the ResponseWriter cannot take a read
// deadline (a wrapper without Unwrap; a test recorder).
var readDeadlineUnset sync.Once

// errBodyBudgetWait: the body waited its whole deadline for room in
// in_flight. errBodyShareWait: it waited its whole deadline passed over
// because its caller held its share (bunch 116, F3).
var (
	errBodyBudgetWait = errors.New("no room in the request body budget")
	errBodyShareWait  = errors.New("the caller holds its share of the request body budget")
)

// bodyBudget counts the bytes of request bodies held at once against size,
// and each caller's against share. Waiters are served in arrival order, so
// a large body is not starved by small ones, with one exception: a waiter
// whose caller is over its share is passed over (and so are that caller's
// later waiters, which keep their order), so a caller that holds its share
// delays only its own bodies (bunch 109, F-A).
type bodyBudget struct {
	mu       sync.Mutex
	size     int64
	share    int64
	used     int64
	byCaller map[string]int64
	waiters  []*budgetWaiter
}

type budgetWaiter struct {
	caller  string
	n       int64
	ready   chan struct{}
	granted bool
}

// acquire reserves n bytes (at most size) for caller until deadline or ctx
// ends, and returns what it reserved and how long it waited.
func (b *bodyBudget) acquire(ctx context.Context, caller string, n int64, deadline time.Time) (int64, time.Duration, error) {
	n = max(0, min(n, b.size))
	w := &budgetWaiter{caller: caller, n: n, ready: make(chan struct{})}
	b.mu.Lock()
	b.waiters = append(b.waiters, w)
	b.grantLocked()
	if w.granted {
		b.mu.Unlock()
		return n, 0, nil
	}
	b.mu.Unlock()
	started := time.Now()
	timer := time.NewTimer(time.Until(deadline))
	defer timer.Stop()
	var err error
	select {
	case <-w.ready:
		return n, time.Since(started), nil
	case <-ctx.Done():
		err = ctx.Err()
	case <-timer.C:
		err = errBodyBudgetWait
	}
	b.mu.Lock()
	if held := b.byCaller[caller]; err == errBodyBudgetWait && !w.granted && held > 0 && held+n > b.share {
		err = errBodyShareWait
	}
	if w.granted {
		b.releaseLocked(caller, n)
	} else {
		for i, o := range b.waiters {
			if o == w {
				b.waiters = append(b.waiters[:i], b.waiters[i+1:]...)
				break
			}
		}
	}
	b.grantLocked()
	b.mu.Unlock()
	return 0, time.Since(started), err
}

func (b *bodyBudget) release(caller string, n int64) {
	b.mu.Lock()
	b.releaseLocked(caller, n)
	b.grantLocked()
	b.mu.Unlock()
}

func (b *bodyBudget) releaseLocked(caller string, n int64) {
	b.used -= n
	if left := b.byCaller[caller] - n; left > 0 {
		b.byCaller[caller] = left
	} else {
		delete(b.byCaller, caller)
	}
}

// grantLocked grants waiters oldest first until one that is within its
// caller's share does not fit in size. A caller's first body is always
// within its share, so one body of any admitted size can be taken in.
func (b *bodyBudget) grantLocked() {
	if b.byCaller == nil {
		b.byCaller = map[string]int64{}
	}
	var overShare map[string]bool
	kept := b.waiters[:0]
	for i, w := range b.waiters {
		held := b.byCaller[w.caller]
		if overShare[w.caller] || (held > 0 && held+w.n > b.share) {
			if overShare == nil {
				overShare = map[string]bool{}
			}
			overShare[w.caller] = true
			kept = append(kept, w)
			continue
		}
		if b.used+w.n > b.size {
			kept = append(kept, b.waiters[i:]...)
			break
		}
		b.used += w.n
		b.byCaller[w.caller] = held + w.n
		w.granted = true
		close(w.ready)
	}
	clear(b.waiters[len(kept):])
	b.waiters = kept
}

func (b *bodyBudget) inUse() int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.used
}

func (c *bodyCaps) refuse(route, caller, size string, limit int64) {
	c.mu.Lock()
	c.refused[route]++
	c.mu.Unlock()
	log.Printf("gateway: body cap refused route=%s caller=%s %s limit=%d", route, caller, size, limit)
}

func (c *bodyCaps) seen(route string, n int64) {
	c.mu.Lock()
	if n > c.maxSeen[route] {
		c.maxSeen[route] = n
	}
	c.mu.Unlock()
}

// gatewayBodyCapsHealth is the caps as /health reports them: the mode, the
// limits in bytes, and per route class the refusals, the observe-mode
// would-refuse count and the largest body read since the gateway started.
type gatewayBodyCapsHealth struct {
	Mode        string           `json:"mode"`
	Limits      map[string]int64 `json:"limits"`
	Refused     map[string]int64 `json:"refused"`
	WouldRefuse map[string]int64 `json:"would_refuse"`
	MaxSeen     map[string]int64 `json:"max_seen"`
	// InFlightBytes is the bytes of bodies held now, against
	// limits.in_flight.
	InFlightBytes      int64 `json:"in_flight_bytes"`
	ReadTimeoutSeconds int64 `json:"read_timeout_seconds"`
}

func (h *Handler) bodyCapsHealth() *gatewayBodyCapsHealth {
	c := h.caps()
	inUse := c.budget.inUse()
	c.mu.Lock()
	defer c.mu.Unlock()
	copyMap := func(m map[string]int64) map[string]int64 {
		out := make(map[string]int64, len(m))
		for k, v := range m {
			out[k] = v
		}
		return out
	}
	return &gatewayBodyCapsHealth{Mode: c.mode, Limits: copyMap(c.limits), Refused: copyMap(c.refused),
		WouldRefuse: copyMap(c.wouldRefuse), MaxSeen: copyMap(c.maxSeen),
		InFlightBytes: inUse, ReadTimeoutSeconds: int64(c.readTimeout / time.Second)}
}

// newHandlerBodyCaps is the constructors' one read of the environment.
func newHandlerBodyCaps() *bodyCaps { return bodyCapsFromGetenv(os.Getenv) }
