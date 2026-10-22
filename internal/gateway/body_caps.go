package gateway

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
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
// "model=64MiB,search=1MiB" (bytes, or a KiB, MiB or GiB suffix).
const (
	// DefaultModelBodyLimit covers a full 1M-token context (about 16 MiB of
	// JSON) and one visual bundle (24 MiB, 32 MiB as base64): about 48 MiB.
	DefaultModelBodyLimit int64 = 64 << 20
	// DefaultModelBodyCeiling is the media contract's own ceiling, 20 images
	// of 20 MiB as base64 plus a full context: the bound in observe mode.
	DefaultModelBodyCeiling int64 = 576 << 20
	// DefaultSearchBodyLimit: a search request is a query, a count, a depth.
	DefaultSearchBodyLimit int64 = 1 << 20
	// DefaultOperatorBodyLimit: one token, one address or one provider name.
	DefaultOperatorBodyLimit int64 = 64 << 10
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
	}
}

type bodyCaps struct {
	mode   string
	limits map[string]int64 // fixed once built

	mu          sync.Mutex
	refused     map[string]int64
	wouldRefuse map[string]int64
	maxSeen     map[string]int64
}

// defaultCaps serves a Handler built without a constructor (tests).
var defaultCaps = newBodyCaps(BodyCapsEnforce, defaultBodyLimits())

func newBodyCaps(mode string, limits map[string]int64) *bodyCaps {
	return &bodyCaps{mode: mode, limits: limits,
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
	}
	if limits[bodyCeilingModel] < limits[bodyRouteModel] {
		limits[bodyCeilingModel] = limits[bodyRouteModel]
	}
	return newBodyCaps(mode, limits)
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
	return "mode=" + c.mode + " " + strings.Join(parts, ",")
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

func isBodyTooLarge(err error) bool {
	var tooLarge *errBodyTooLarge
	return errors.As(err, &tooLarge)
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
// fields.
func (h *Handler) decodeBody(w http.ResponseWriter, r *http.Request, route, caller string, v any, strict bool) error {
	c := h.caps()
	limit := c.limits[route]
	readCap := limit
	observe := route == bodyRouteModel && c.mode == BodyCapsObserve
	if observe {
		readCap = c.limits[bodyCeilingModel]
	}
	if r.ContentLength > readCap {
		c.refuse(route, caller, fmt.Sprintf("content_length=%d", r.ContentLength), readCap)
		return &errBodyTooLarge{route: route, limit: readCap}
	}
	body := &countingReader{r: http.MaxBytesReader(w, r.Body, readCap)}
	dec := json.NewDecoder(body)
	if strict {
		dec.DisallowUnknownFields()
	}
	err := dec.Decode(v)
	c.seen(route, body.n)
	var maxBytes *http.MaxBytesError
	if errors.As(err, &maxBytes) {
		c.refuse(route, caller, fmt.Sprintf("bytes>%d", readCap), readCap)
		return &errBodyTooLarge{route: route, limit: readCap}
	}
	if err == nil && observe && body.n > limit {
		c.mu.Lock()
		c.wouldRefuse[route]++
		c.mu.Unlock()
		log.Printf("gateway: body cap would_refuse route=%s caller=%s bytes=%d limit=%d", route, caller, body.n, limit)
	}
	return err
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
}

func (h *Handler) bodyCapsHealth() *gatewayBodyCapsHealth {
	c := h.caps()
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
		WouldRefuse: copyMap(c.wouldRefuse), MaxSeen: copyMap(c.maxSeen)}
}

// newHandlerBodyCaps is the constructors' one read of the environment.
func newHandlerBodyCaps() *bodyCaps { return bodyCapsFromGetenv(os.Getenv) }
