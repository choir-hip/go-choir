package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// SEC-08, Option K, gateway half: a relay for the Claude delegate's Messages
// API calls that holds a Claude Console API key, so the guest never carries
// one (docs/problems/claude-delegate-key-sits-on-the-guest-disk-2026-10-21.md;
// design in docs/definitions/choir-launch-fp7-gateway-identity-2026-10-21.md).
//
// What this is not: it never holds or relays the owner's Claude subscription
// token (Anthropic's terms forbid a service from intermediating it); it is a
// narrow relay for an API key the owner places in gateway-provider.env, for
// the identities and window the gateway is told, and nothing else. It is dark
// until all of that exists: no key, no identities or no open window each
// refuse every call, and nothing falls back to another credential.

const (
	// AnthropicAPIKeyEnv is the Console API key, in gateway-provider.env.
	AnthropicAPIKeyEnv = "ANTHROPIC_API_KEY"
	// AnthropicRelayIdentitiesEnv lists the gateway identities (the ids
	// vmctl issues bearers for) the relay serves.
	AnthropicRelayIdentitiesEnv = "GATEWAY_ANTHROPIC_RELAY_IDENTITIES"
	// AnthropicRelayExpiresEnv is the end of the window (RFC 3339 or
	// YYYY-MM-DD, UTC). Missing, unreadable or past means the relay is dark.
	AnthropicRelayExpiresEnv = "GATEWAY_ANTHROPIC_RELAY_EXPIRES"
	// AnthropicRelayModelsEnv lists the models the relay serves.
	AnthropicRelayModelsEnv = "GATEWAY_ANTHROPIC_RELAY_MODELS"
	// AnthropicRelayMaxInflightEnv is the per-identity concurrent-call cap.
	AnthropicRelayMaxInflightEnv = "GATEWAY_ANTHROPIC_RELAY_MAX_INFLIGHT"

	anthropicAPIBase          = "https://api.anthropic.com"
	anthropicRelayPrefix      = "/provider/anthropic"
	anthropicDefaultVersion   = "2023-06-01"
	anthropicDefaultModel     = "claude-sonnet-5-5"
	anthropicRelayMaxBody     = 32 << 20 // the Messages API's own request limit
	anthropicRelayMaxInflight = 2
	anthropicRelayTimeout     = inferenceTimeout
	anthropicErrBodyMax       = 64 << 10
	anthropicUsageJSONMax     = 4 << 20
	anthropicUsageLineMax     = 1 << 20
)

// AnthropicRelay is the relay's configuration and state.
type AnthropicRelay struct {
	key         string
	identities  map[string]bool
	expires     time.Time
	models      map[string]bool
	baseURL     string
	client      *http.Client
	maxBody     int64
	maxInflight int
	now         func() time.Time

	mu       sync.Mutex
	inflight map[string]int
	usage    relayUsage
}

// relayUsage is the spend record since the gateway started: tracked, never
// capped (owner: track spending, don't cap).
type relayUsage struct {
	Requests            int64
	InputTokens         int64
	OutputTokens        int64
	CachedInputTokens   int64
	CacheCreationTokens int64
}

// NewAnthropicRelayFromEnv reads the relay's configuration.
func NewAnthropicRelayFromEnv() *AnthropicRelay {
	return newAnthropicRelayFromGetenv(os.Getenv)
}

func newAnthropicRelayFromGetenv(get func(string) string) *AnthropicRelay {
	r := &AnthropicRelay{
		key:         strings.TrimSpace(get(AnthropicAPIKeyEnv)),
		identities:  map[string]bool{},
		models:      map[string]bool{},
		baseURL:     anthropicAPIBase, // never from the environment
		maxBody:     anthropicRelayMaxBody,
		maxInflight: anthropicRelayMaxInflight,
		now:         time.Now,
		client: &http.Client{Transport: &http.Transport{
			Proxy:                 nil,
			ResponseHeaderTimeout: 5 * time.Minute,
			MaxIdleConnsPerHost:   4,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   15 * time.Second,
		}},
	}
	split := func(raw string) []string {
		return strings.FieldsFunc(raw, func(c rune) bool { return c == ',' || c == ' ' || c == '\n' || c == '\t' || c == '\r' })
	}
	for _, id := range split(get(AnthropicRelayIdentitiesEnv)) {
		r.identities[id] = true
	}
	models := split(get(AnthropicRelayModelsEnv))
	if len(models) == 0 {
		models = []string{anthropicDefaultModel}
	}
	for _, m := range models {
		r.models[m] = true
	}
	if raw := strings.TrimSpace(get(AnthropicRelayExpiresEnv)); raw != "" {
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			r.expires = t.UTC()
		} else if t, err := time.Parse("2006-01-02", raw); err == nil {
			r.expires = t.UTC()
		}
	}
	if n, err := strconv.Atoi(strings.TrimSpace(get(AnthropicRelayMaxInflightEnv))); err == nil && n > 0 {
		r.maxInflight = n
	}
	return r
}

func (r *AnthropicRelay) windowOpen() bool {
	return r != nil && !r.expires.IsZero() && r.now().Before(r.expires)
}

// status is "off" without a key, "dark" with a key but no identities or no
// open window, "on" otherwise. It names neither.
func (r *AnthropicRelay) status() string {
	switch {
	case r == nil || r.key == "":
		return "off"
	case len(r.identities) == 0 || !r.windowOpen():
		return "dark"
	}
	return "on"
}

func (r *AnthropicRelay) usageSnapshot() relayUsage {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.usage
}

func (r *AnthropicRelay) acquire(identity string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.inflight == nil {
		r.inflight = map[string]int{}
	}
	if r.inflight[identity] >= r.maxInflight {
		return false
	}
	r.inflight[identity]++
	return true
}

func (r *AnthropicRelay) release(identity string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.inflight[identity] > 0 {
		r.inflight[identity]--
	}
	if r.inflight[identity] == 0 {
		delete(r.inflight, identity)
	}
}

// SetAnthropicRelay attaches the relay. A handler without one answers every
// relay call with 503.
func (h *Handler) SetAnthropicRelay(r *AnthropicRelay) {
	h.anthropicRelay = r
}

type relayRoute struct {
	method string
	path   string // upstream path
	body   bool
}

// relayRoutes is everything the relay serves: what `claude -p` needs and
// nothing else. No batches, files, organizations or legacy completions.
var relayRoutes = map[string]relayRoute{
	"/v1/messages":              {http.MethodPost, "/v1/messages", true},
	"/v1/messages/count_tokens": {http.MethodPost, "/v1/messages/count_tokens", true},
	"/v1/models":                {http.MethodGet, "/v1/models", false},
}

func relayError(w http.ResponseWriter, status int, errType, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"type":  "error",
		"error": map[string]string{"type": errType, "message": message},
	})
}

// HandleAnthropicRelay serves /provider/anthropic/v1/{messages,
// messages/count_tokens,models}. The order is the design's: path and method,
// the computer's bearer (bound to its address, SEC-05), the allowlist and
// window, the key, the rate and in-flight bounds, the body and its model, and
// only then the upstream call.
func (h *Handler) HandleAnthropicRelay(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	route, ok := relayRoutes[strings.TrimPrefix(r.URL.Path, anthropicRelayPrefix)]
	if !strings.HasPrefix(r.URL.Path, anthropicRelayPrefix+"/") || !ok {
		relayError(w, http.StatusNotFound, "not_found_error", "not found")
		return
	}
	if r.Method != route.method {
		w.Header().Set("Allow", route.method)
		relayError(w, http.StatusMethodNotAllowed, "invalid_request_error", "method not allowed")
		return
	}
	computerID, err := h.authenticateAutoputer(r)
	if err != nil {
		log.Printf("gateway: anthropic relay authentication denied: %v", err)
		status, retry, message := authFailure(err)
		if retry != "" {
			w.Header().Set("Retry-After", retry)
		}
		errType := "authentication_error"
		if status == http.StatusTooManyRequests {
			errType = "rate_limit_error"
		} else if status != http.StatusUnauthorized {
			errType = "api_error"
		}
		relayError(w, status, errType, message)
		return
	}
	relay := h.anthropicRelay
	if relay == nil {
		relayError(w, http.StatusServiceUnavailable, "api_error", "anthropic relay is not configured")
		return
	}
	if !relay.identities[computerID] || !relay.windowOpen() {
		log.Printf("gateway: anthropic relay refused %s: not enabled for this computer or the window is closed", computerID)
		relayError(w, http.StatusForbidden, "permission_error", "anthropic relay is not enabled for this computer")
		return
	}
	if relay.key == "" {
		log.Printf("gateway: anthropic relay refused %s: no API key is configured", computerID)
		relayError(w, http.StatusServiceUnavailable, "api_error", "anthropic relay is not configured")
		return
	}
	if h.rateLimiter != nil {
		bucket := rateLimitBucketKey(computerID, "anthropic_relay")
		if !h.rateLimiter.Record(bucket) {
			_, _, resetIn := h.rateLimiter.Status(bucket)
			w.Header().Set("Retry-After", strconv.Itoa(max(1, int(resetIn.Seconds()))))
			relayError(w, http.StatusTooManyRequests, "rate_limit_error", "rate limit exceeded")
			return
		}
	}
	if !relay.acquire(computerID) {
		w.Header().Set("Retry-After", "5")
		relayError(w, http.StatusTooManyRequests, "rate_limit_error", "too many concurrent relay calls")
		return
	}
	defer relay.release(computerID)

	var body []byte
	model := ""
	if route.body {
		if mt := strings.ToLower(strings.TrimSpace(strings.SplitN(r.Header.Get("Content-Type"), ";", 2)[0])); mt != "application/json" {
			relayError(w, http.StatusUnsupportedMediaType, "invalid_request_error", "content-type must be application/json")
			return
		}
		var err error
		body, err = io.ReadAll(http.MaxBytesReader(w, r.Body, relay.maxBody))
		if err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				relayError(w, http.StatusRequestEntityTooLarge, "request_too_large", "request body is too large")
				return
			}
			relayError(w, http.StatusBadRequest, "invalid_request_error", "could not read the request body")
			return
		}
		var modelErr string
		model, modelErr = requestModel(body)
		if modelErr != "" {
			relayError(w, http.StatusBadRequest, "invalid_request_error", modelErr)
			return
		}
		if !relay.models[model] {
			log.Printf("gateway: anthropic relay refused %s: model not served by the relay", computerID)
			relayError(w, http.StatusForbidden, "permission_error", "model is not served by this relay")
			return
		}
	}

	query := ""
	if r.URL.Query().Get("beta") == "true" {
		query = "?beta=true"
	}
	ctx, cancel := context.WithTimeout(r.Context(), anthropicRelayTimeout)
	defer cancel()
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	up, err := http.NewRequestWithContext(ctx, route.method, strings.TrimRight(relay.baseURL, "/")+route.path+query, reader)
	if err != nil {
		relayError(w, http.StatusBadGateway, "api_error", "relay could not build the upstream request")
		return
	}
	// Only these cross, and the key is the gateway's own. Nothing the caller
	// sent that could authenticate, identify or route does.
	up.Header.Set("X-Api-Key", relay.key)
	up.Header.Set("Anthropic-Version", anthropicDefaultVersion)
	if v := strings.TrimSpace(r.Header.Get("Anthropic-Version")); v != "" && len(v) <= 32 && relayTokenText.MatchString(v) {
		up.Header.Set("Anthropic-Version", v)
	}
	if v := strings.TrimSpace(r.Header.Get("Anthropic-Beta")); v != "" && len(v) <= 512 && relayTokenText.MatchString(v) {
		up.Header.Set("Anthropic-Beta", v)
	}
	if route.body {
		up.Header.Set("Content-Type", "application/json")
	}
	up.Header.Set("Accept", "application/json")
	if a := r.Header.Get("Accept"); strings.Contains(a, "text/event-stream") {
		up.Header.Set("Accept", "text/event-stream")
	}
	up.Header.Set("Accept-Encoding", "identity")
	up.Header.Set("User-Agent", "choir-gateway-anthropic-relay")

	// A copy of the client that never follows a redirect, so the key cannot
	// be carried to another host whatever client was configured.
	client := *relay.client
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Do(up)
	if err != nil {
		if ctx.Err() != nil && r.Context().Err() != nil {
			log.Printf("gateway: anthropic relay %s: client went away", computerID)
			return
		}
		log.Printf("gateway: anthropic relay %s: upstream call failed: %s", computerID, relay.redact(sanitizeError(err)))
		relayError(w, http.StatusBadGateway, "api_error", "relay upstream unavailable")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		log.Printf("gateway: anthropic relay %s: upstream answered %d with a redirect; not followed", computerID, resp.StatusCode)
		relayError(w, http.StatusBadGateway, "api_error", "relay upstream redirected")
		return
	}
	copyRelayHeaders(w.Header(), resp.Header)

	if resp.StatusCode >= 400 {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, anthropicErrBodyMax))
		out := []byte(relay.redact(string(errBody)))
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(out)
		log.Printf("gateway: anthropic relay %s model=%s path=%s status=%d bytes_in=%d bytes_out=%d dur=%s",
			computerID, model, route.path, resp.StatusCode, len(body), len(out), time.Since(started).Round(time.Millisecond))
		return
	}

	w.WriteHeader(resp.StatusCode)
	flusher, _ := w.(http.Flusher)
	tap := newUsageTap(strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream"))
	var out int64
	buf := make([]byte, 32<<10)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			tap.write(buf[:n])
			if _, werr := w.Write(buf[:n]); werr != nil {
				break
			}
			out += int64(n)
			if flusher != nil {
				flusher.Flush()
			}
		}
		if rerr != nil {
			break
		}
	}
	u := tap.finish()
	relay.recordUsage(u)
	log.Printf("gateway: anthropic relay %s model=%s path=%s status=%d bytes_in=%d bytes_out=%d tokens=%d+%d cached=%d dur=%s",
		computerID, model, route.path, resp.StatusCode, len(body), out, u.InputTokens, u.OutputTokens, u.CachedInputTokens, time.Since(started).Round(time.Millisecond))
}

// requestModel reads the top-level "model" of a Messages request, and refuses
// a body whose model is ambiguous: repeated, spelled in another case (Go's
// decoder would match it, a stricter or looser upstream parser might not), or
// not a plain string. The allowlist must check the model the upstream will
// run, so a doubt is a refusal.
func requestModel(body []byte) (string, string) {
	dec := json.NewDecoder(bytes.NewReader(body))
	tok, err := dec.Token()
	if d, ok := tok.(json.Delim); err != nil || !ok || d != '{' {
		return "", "request body must be a JSON object"
	}
	model, found := "", false
	for dec.More() {
		keyTok, err := dec.Token()
		key, ok := keyTok.(string)
		if err != nil || !ok {
			return "", "request body must be a JSON object"
		}
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return "", "request body must be a JSON object"
		}
		if !strings.EqualFold(key, "model") {
			continue
		}
		if key != "model" || found {
			return "", "model must appear exactly once"
		}
		if json.Unmarshal(raw, &model) != nil {
			return "", "model must be a string"
		}
		found = true
	}
	if _, err := dec.Token(); err != nil { // the closing brace
		return "", "request body must be a JSON object"
	}
	if dec.More() {
		return "", "request body must be one JSON object"
	}
	if !found || model == "" {
		return "", "model is required"
	}
	return model, ""
}

// relayTokenText admits the characters an anthropic-version or anthropic-beta
// value uses and nothing that could split a header.
var relayTokenText = regexp.MustCompile(`^[A-Za-z0-9._,: -]+$`)

var relayKeyShape = regexp.MustCompile(`sk-ant-[A-Za-z0-9_\-]{8,}`)

// redact removes the gateway's key, and anything shaped like an Anthropic
// key, from text bound for a log or a client.
func (r *AnthropicRelay) redact(s string) string {
	if r != nil && r.key != "" {
		s = strings.ReplaceAll(s, r.key, "[redacted]")
	}
	return relayKeyShape.ReplaceAllString(s, "[redacted]")
}

// copyRelayHeaders passes the response headers a client needs and drops the
// rest (cookies, edge and server identification, anything unnamed).
func copyRelayHeaders(dst, src http.Header) {
	for name, values := range src {
		canon := http.CanonicalHeaderKey(name)
		switch {
		case canon == "Content-Type", canon == "Cache-Control", canon == "Request-Id", canon == "X-Request-Id", canon == "Retry-After",
			strings.HasPrefix(canon, "Anthropic-"):
		default:
			continue
		}
		for _, v := range values {
			if strings.ContainsAny(v, "\r\n") {
				continue
			}
			dst.Add(canon, v)
		}
	}
	if strings.HasPrefix(dst.Get("Content-Type"), "text/event-stream") {
		dst.Set("X-Accel-Buffering", "no")
	}
}

func (r *AnthropicRelay) recordUsage(u relayUsage) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.usage.Requests++
	r.usage.InputTokens += u.InputTokens
	r.usage.OutputTokens += u.OutputTokens
	r.usage.CachedInputTokens += u.CachedInputTokens
	r.usage.CacheCreationTokens += u.CacheCreationTokens
}

// usageTap reads token counts out of a relayed reply without holding it: a
// server-sent-event stream line by line (the last non-zero value of each
// counter wins, since message_delta repeats the running output total), a JSON
// reply in a bounded buffer. A counter it cannot read is zero; the reply is
// never altered.
type usageTap struct {
	sse     bool
	line    bytes.Buffer
	skip    bool
	json    bytes.Buffer
	overrun bool
	usage   relayUsage
}

func newUsageTap(sse bool) *usageTap { return &usageTap{sse: sse} }

type usageFields struct {
	InputTokens              int64 `json:"input_tokens"`
	OutputTokens             int64 `json:"output_tokens"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
}

type usageCarrier struct {
	Usage   *usageFields `json:"usage"`
	Message *struct {
		Usage *usageFields `json:"usage"`
	} `json:"message"`
}

func (t *usageTap) take(c usageCarrier) {
	for _, f := range []*usageFields{c.Usage, func() *usageFields {
		if c.Message != nil {
			return c.Message.Usage
		}
		return nil
	}()} {
		if f == nil {
			continue
		}
		if f.InputTokens > 0 {
			t.usage.InputTokens = f.InputTokens
		}
		if f.OutputTokens > 0 {
			t.usage.OutputTokens = f.OutputTokens
		}
		if f.CacheReadInputTokens > 0 {
			t.usage.CachedInputTokens = f.CacheReadInputTokens
		}
		if f.CacheCreationInputTokens > 0 {
			t.usage.CacheCreationTokens = f.CacheCreationInputTokens
		}
	}
}

func (t *usageTap) write(p []byte) {
	if !t.sse {
		if t.overrun {
			return
		}
		if t.json.Len()+len(p) > anthropicUsageJSONMax {
			t.overrun = true
			t.json.Reset()
			return
		}
		t.json.Write(p)
		return
	}
	for len(p) > 0 {
		i := bytes.IndexByte(p, '\n')
		if i < 0 {
			if !t.skip {
				if t.line.Len()+len(p) > anthropicUsageLineMax {
					t.skip = true
					t.line.Reset()
				} else {
					t.line.Write(p)
				}
			}
			return
		}
		if !t.skip {
			if t.line.Len()+i <= anthropicUsageLineMax {
				t.line.Write(p[:i])
				t.handleLine(t.line.Bytes())
			}
		}
		t.line.Reset()
		t.skip = false
		p = p[i+1:]
	}
}

func (t *usageTap) handleLine(line []byte) {
	line = bytes.TrimSpace(line)
	data, ok := bytes.CutPrefix(line, []byte("data:"))
	if !ok || !bytes.Contains(data, []byte(`"usage"`)) {
		return
	}
	var c usageCarrier
	if json.Unmarshal(bytes.TrimSpace(data), &c) == nil {
		t.take(c)
	}
}

func (t *usageTap) finish() relayUsage {
	if t.sse {
		if t.line.Len() > 0 && !t.skip {
			t.handleLine(t.line.Bytes())
		}
	} else if !t.overrun && t.json.Len() > 0 {
		var c usageCarrier
		if json.Unmarshal(t.json.Bytes(), &c) == nil {
			t.take(c)
		}
	}
	return t.usage
}

// Status is the relay's state for the startup log: "off", "dark" or "on".
func (r *AnthropicRelay) Status() string { return r.status() }
