package provider

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DevinProvider implements the Provider interface for Cognition's Devin
// (SWE) models. Devin has no OpenAI-compatible inference endpoint: the CLI
// and Desktop products call the Codeium/Windsurf backend
// (server.codeium.com) over Connect-RPC with protobuf bodies. The credential
// is the `windsurf_api_key` from the Devin CLI credentials.toml, stored as
// `devin-session-token$<JWT>`; it is exchanged for a short-lived user JWT
// through AuthService/GetUserJwt and then carried inside the protobuf
// Metadata message (there is no Authorization header).
//
// RISK, stated plainly: this authenticates with one person's Devin CLI
// session and identifies itself to the service as the Devin CLI
// (devinMetadataIDEName). It is an unofficial client of a private API. The
// service can revoke the session or the account at any time. Every refusal
// is therefore typed (devinClassifyError) so the model pool holds Devin and
// degrades to its other members instead of retrying it on every call.
//
// Wire reference: the endpoints and message shapes mirror omp's
// packages/ai/src/providers/devin.ts, recovered from the omp binary
// 2026-11-09 and smoke-tested against server.codeium.com.
type DevinProvider struct {
	apiKey  string
	baseURL string
	modelID string

	httpClient *http.Client

	// refreshMu serializes user-JWT refreshes (single flight): callers that
	// find a fresh token after the wait use it instead of exchanging again.
	refreshMu sync.Mutex

	mu              sync.Mutex
	userJWT         string
	userJWTAt       time.Time
	baseURLOverride string
}

// devinUserJWTTTL is how long a cached user JWT is trusted. The service
// issues about 24 minutes; refreshing at 20 avoids a 401 round trip.
const devinUserJWTTTL = 20 * time.Minute

// devinUnaryTimeout bounds GetUserJwt.
const devinUnaryTimeout = 30 * time.Second

// NewDevinProvider creates a Devin provider with the given session
// credential ("devin-session-token$<JWT>" or a bare JWT, which is
// normalized) and base URL (defaults to https://server.codeium.com).
func NewDevinProvider(apiKey, baseURL, modelID string) (*DevinProvider, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, fmt.Errorf("devin: api key is required")
	}
	apiKey = normalizeDevinAPIKey(apiKey)
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = "https://server.codeium.com"
	}
	if modelID == "" {
		modelID = "swe-2-flash"
	}
	return &DevinProvider{
		apiKey:     apiKey,
		baseURL:    baseURL,
		modelID:    modelID,
		httpClient: &http.Client{Timeout: defaultProviderHTTPTimeout},
	}, nil
}

// NewDevinProviderFromEnv reads DEVIN_API_KEY (the windsurf_api_key value
// from ~/.local/share/devin/credentials.toml) and DEVIN_BASE_URL.
func NewDevinProviderFromEnv(modelID string) (*DevinProvider, error) {
	return NewDevinProvider(os.Getenv("DEVIN_API_KEY"), os.Getenv("DEVIN_BASE_URL"), modelID)
}

// normalizeDevinAPIKey prefixes a bare three-part JWT with the
// devin-session-token$ marker, matching omp's credential normalization.
func normalizeDevinAPIKey(key string) string {
	if strings.HasPrefix(key, "devin-session-token$") {
		return key
	}
	if strings.Count(strings.TrimSpace(key), ".") == 2 {
		return "devin-session-token$" + key
	}
	return key
}

func (p *DevinProvider) Name() string { return "devin" }

func (p *DevinProvider) IsReal() bool { return true }

// Call executes the request through the streaming endpoint and accumulates
// the result; Devin's GetChatMessage is server-streaming only.
func (p *DevinProvider) Call(ctx context.Context, req LLMRequest) (*LLMResponse, error) {
	resp, err := p.Stream(ctx, req, nil)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, fmt.Errorf("devin: empty stream response")
	}
	return resp, nil
}

// ---------------------------------------------------------------------------
// Errors. Every failure leaves this file typed for the pool: a spent or
// refused Devin is a QuotaExhaustedError (the gateway's breaker holds the
// model until its reset, or on a doubling cooldown when none is known, and
// the guest records it as provider_quota and routes to the next member); a
// short rate limit is an upstreamConcurrencyError; everything else carries
// "status NNN" text so the breaker counts 5xx and ignores other 4xx.
// ---------------------------------------------------------------------------

// devinRPCError is a sanitized upstream failure. Code is the Connect error
// code ("resource_exhausted", ...). The wrapped error is the typed answer the
// pool reads; Error() is its text. The message is redacted, truncated and
// never includes request bytes or credentials.
type devinRPCError struct {
	Endpoint string
	Code     string
	inner    error
}

func (e *devinRPCError) Error() string { return e.inner.Error() }
func (e *devinRPCError) Unwrap() error { return e.inner }

// devinConnectHTTPStatus is the Connect protocol's HTTP status for a code.
var devinConnectHTTPStatus = map[string]int{
	"canceled": 499, "unknown": 500, "invalid_argument": 400, "deadline_exceeded": 504,
	"not_found": 404, "already_exists": 409, "permission_denied": 403, "resource_exhausted": 429,
	"failed_precondition": 400, "aborted": 409, "out_of_range": 400, "unimplemented": 501,
	"internal": 500, "unavailable": 503, "data_loss": 500, "unauthenticated": 401,
}

// devinCodeFromHTTP is the Connect code for a bare HTTP status when the body
// names none.
func devinCodeFromHTTP(status int) string {
	switch status {
	case 400:
		return "invalid_argument"
	case 401:
		return "unauthenticated"
	case 403:
		return "permission_denied"
	case 404:
		return "not_found"
	case 408, 504:
		return "deadline_exceeded"
	case 409:
		return "aborted"
	case 429:
		return "resource_exhausted"
	case 501:
		return "unimplemented"
	case 502, 503:
		return "unavailable"
	}
	if status >= 500 {
		return "internal"
	}
	return "unknown"
}

var (
	devinSecretPattern = regexp.MustCompile(`devin-session-token\$\S+|eyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{4,}\.?[A-Za-z0-9_-]*`)
	devinRetryInText   = regexp.MustCompile(`(?i)\b(?:in|after)\s+(\d{1,6})\s*(seconds?|secs?|minutes?|mins?|hours?|hrs?|days?)\b`)
	devinBurstWords    = []string{"rate limit", "rate-limit", "too many requests", "concurrent", "slow down"}
)

// devinSanitize redacts credential-shaped text and bounds the length.
func (p *DevinProvider) devinSanitize(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if p != nil {
		p.mu.Lock()
		jwt := p.userJWT
		p.mu.Unlock()
		for _, secret := range []string{p.apiKey, jwt} {
			if len(secret) >= 8 {
				s = strings.ReplaceAll(s, secret, "[redacted]")
			}
		}
	}
	s = devinSecretPattern.ReplaceAllString(s, "[redacted]")
	if len(s) > 300 {
		s = s[:300]
	}
	return s
}

// devinErrorBody is a Connect error object: an HTTP error body, or the
// "error" member of an end-stream frame.
type devinErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details []struct {
		Debug json.RawMessage `json:"debug"`
		// RetryDelay is the google.rpc.RetryInfo shape some details carry.
		RetryDelay json.RawMessage `json:"retryDelay"`
	} `json:"details"`
}

// devinParseDelay reads a duration given as "30s", a number of seconds, or
// {"seconds": N}.
func devinParseDelay(raw json.RawMessage) time.Duration {
	if len(raw) == 0 {
		return 0
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		if d, err := time.ParseDuration(strings.TrimSpace(s)); err == nil {
			return d
		}
		if n, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
			return time.Duration(n * float64(time.Second))
		}
		return 0
	}
	var n float64
	if json.Unmarshal(raw, &n) == nil {
		return time.Duration(n * float64(time.Second))
	}
	var o struct {
		Seconds float64 `json:"seconds"`
	}
	if json.Unmarshal(raw, &o) == nil {
		return time.Duration(o.Seconds * float64(time.Second))
	}
	return 0
}

// devinResetAt reads a reset time from, in order: the Retry-After header, the
// end-stream trailer metadata, the error details' retry delay, and a "in N
// minutes" phrase in the message. Zero when none is believable.
func devinResetAt(h http.Header, metadata map[string][]string, body devinErrorBody, now time.Time) time.Time {
	if at := retryAfterAt(h, now); !at.IsZero() {
		return at
	}
	for k, v := range metadata {
		if strings.EqualFold(k, "retry-after") && len(v) > 0 {
			hh := http.Header{}
			hh.Set("Retry-After", v[0])
			if at := retryAfterAt(hh, now); !at.IsZero() {
				return at
			}
		}
	}
	within := func(d time.Duration) time.Time {
		if d <= 0 || d > quotaRetryAtHorizon {
			return time.Time{}
		}
		return now.Add(d).UTC()
	}
	for _, d := range body.Details {
		if at := within(devinParseDelay(d.RetryDelay)); !at.IsZero() {
			return at
		}
		var dbg struct {
			RetryDelay json.RawMessage `json:"retryDelay"`
		}
		if json.Unmarshal(d.Debug, &dbg) == nil {
			if at := within(devinParseDelay(dbg.RetryDelay)); !at.IsZero() {
				return at
			}
		}
	}
	if m := devinRetryInText.FindStringSubmatch(body.Message); m != nil {
		n, _ := strconv.Atoi(m[1])
		unit := time.Second
		switch strings.ToLower(m[2])[0] {
		case 'm':
			unit = time.Minute
		case 'h':
			unit = time.Hour
		case 'd':
			unit = 24 * time.Hour
		}
		return within(time.Duration(n) * unit)
	}
	return time.Time{}
}

// devinClassify turns a Connect error into the typed answer the pool reads.
// httpStatus is 0 for an end-stream frame (the error rode a 200).
func (p *DevinProvider) devinClassify(endpoint string, httpStatus int, h http.Header, metadata map[string][]string, body devinErrorBody) error {
	code := strings.ToLower(strings.TrimSpace(body.Code))
	if _, known := devinConnectHTTPStatus[code]; !known {
		code = devinCodeFromHTTP(httpStatus)
	}
	if httpStatus < 400 {
		httpStatus = devinConnectHTTPStatus[code]
	}
	msg := p.devinSanitize(body.Message)
	status := fmt.Sprintf("%d %s", httpStatus, code)
	now := time.Now()
	var inner error
	switch code {
	case "resource_exhausted":
		// A spent allowance or plan, unless the words say a short rate
		// limit and no reset time came with it: then the guest retries the
		// same model and the breaker counts only a long streak. When in
		// doubt it is quota: a held Devin degrades to the other members,
		// a hammered one is the failure the pool exists to avoid.
		at := devinResetAt(h, metadata, body, now)
		lower := strings.ToLower(body.Message)
		burst := false
		for _, w := range devinBurstWords {
			if strings.Contains(lower, w) {
				burst = true
			}
		}
		if burst && at.IsZero() && !reasonSaysQuota(lower) {
			inner = &upstreamConcurrencyError{provider: "devin", status: status}
		} else {
			inner = &QuotaExhaustedError{Provider: "devin", Status: status, RetryAt: at}
		}
	case "permission_denied", "unauthenticated":
		// The session was revoked, expired for good, or the plan refuses
		// the model. No reset time exists: the breaker's doubling cooldown
		// holds Devin (the model) while the pool serves from the others.
		inner = &QuotaExhaustedError{Provider: "devin", Status: status + " (session or plan refused)"}
	default:
		if msg != "" {
			inner = fmt.Errorf("devin: status %s (sanitized): %s", status, msg)
		} else {
			inner = fmt.Errorf("devin: status %s (sanitized)", status)
		}
	}
	return &devinRPCError{Endpoint: endpoint, Code: code, inner: inner}
}

// devinHTTPError reads a non-2xx response (body capped) into a classified error.
func (p *DevinProvider) devinHTTPError(endpoint string, resp *http.Response) error {
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var body devinErrorBody
	if json.Unmarshal(raw, &body) != nil {
		body = devinErrorBody{Message: string(raw)}
	}
	return p.devinClassify(endpoint, resp.StatusCode, resp.Header, nil, body)
}

// devinUnary POSTs a raw protobuf body and returns the raw response bytes.
func (p *DevinProvider) devinUnary(ctx context.Context, path string, body []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, devinUnaryTimeout)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURLFor()+path, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("devin: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/proto")
	httpReq.Header.Set("Connect-Protocol-Version", "1")
	httpReq.Header.Set("Accept", "*/*")
	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("devin: http call: %s request failed: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, p.devinHTTPError(path, resp)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, fmt.Errorf("devin: %s read response: %w", path, err)
	}
	return data, nil
}

// baseURLFor returns the API base URL, honoring a customApiServerUrl
// returned by GetUserJwt when present.
func (p *DevinProvider) baseURLFor() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.baseURLOverride != "" {
		return p.baseURLOverride
	}
	return p.baseURL
}

// devinAcceptableOverride admits a server-named API host only when it is
// https on a DNS name: the session credential travels in every request, so a
// response must not be able to point it at a bare IP, localhost or plain http.
func devinAcceptableOverride(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil {
		return false
	}
	host := u.Hostname()
	if host == "" || !strings.Contains(host, ".") || net.ParseIP(host) != nil {
		return false
	}
	return !strings.HasSuffix(host, ".local") && !strings.HasSuffix(host, ".internal")
}

// ensureUserJWT returns a user JWT for the session credential. stale is the
// token the caller just saw rejected ("" if none): it is never returned.
// Refreshes are single flight under refreshMu, and a token younger than
// devinUserJWTTTL is reused.
func (p *DevinProvider) ensureUserJWT(ctx context.Context, stale string) (string, error) {
	fresh := func() string {
		p.mu.Lock()
		defer p.mu.Unlock()
		if p.userJWT != "" && p.userJWT != stale && time.Since(p.userJWTAt) < devinUserJWTTTL {
			return p.userJWT
		}
		return ""
	}
	if jwt := fresh(); jwt != "" {
		return jwt, nil
	}
	p.refreshMu.Lock()
	defer p.refreshMu.Unlock()
	if jwt := fresh(); jwt != "" {
		return jwt, nil
	}
	respBody, err := p.devinUnary(ctx, "/exa.auth_pb.AuthService/GetUserJwt", devinGetUserJwtRequest(p.apiKey))
	if err != nil {
		return "", err
	}
	fields := pbFields(respBody)
	jwt := pbStringField(fields, 1)
	custom := strings.TrimRight(pbStringField(fields, 2), "/")
	if jwt == "" {
		return "", fmt.Errorf("devin: GetUserJwt returned an empty user JWT")
	}
	p.mu.Lock()
	p.userJWT = jwt
	p.userJWTAt = time.Now()
	if custom != "" && devinAcceptableOverride(custom) {
		p.baseURLOverride = custom
	}
	p.mu.Unlock()
	return jwt, nil
}

// ---------------------------------------------------------------------------
// Minimal protobuf encoding. Only the field shapes the Devin wire uses are
// implemented: length-delimited strings/messages, varints, and fixed64 doubles.
// ---------------------------------------------------------------------------

func pbVarint(n uint64) []byte {
	var out [binary.MaxVarintLen64]byte
	i := binary.PutUvarint(out[:], n)
	return out[:i]
}

// pbKey encodes a field tag: (fieldNumber << 3) | wireType.
func pbKey(field int, wire int) []byte {
	return pbVarint(uint64(field<<3 | wire))
}

func pbString(field int, s string) []byte {
	if s == "" {
		return nil
	}
	out := append(pbKey(field, 2), pbVarint(uint64(len(s)))...)
	return append(out, s...)
}

func pbBytes(field int, b []byte) []byte {
	if len(b) == 0 {
		return nil
	}
	out := append(pbKey(field, 2), pbVarint(uint64(len(b)))...)
	return append(out, b...)
}

func pbMessage(field int, inner []byte) []byte {
	return pbBytes(field, inner)
}

func pbVarintField(field int, v uint64) []byte {
	return append(pbKey(field, 0), pbVarint(v)...)
}

func pbDouble(field int, v float64) []byte {
	out := append(pbKey(field, 1), make([]byte, 8)...)
	binary.LittleEndian.PutUint64(out[len(out)-8:], math.Float64bits(v))
	return out
}

// pbField is one decoded protobuf field.
type pbField struct {
	Num  int
	Wire int
	// Bytes is the payload for wire type 2 (length-delimited).
	Bytes []byte
	// Varint is the value for wire type 0.
	Varint uint64
}

// pbFields decodes a protobuf message into its top-level fields. Malformed
// input stops the scan; callers treat missing fields as absent.
func pbFields(data []byte) []pbField {
	var out []pbField
	for len(data) > 0 {
		key, n := binary.Uvarint(data)
		if n <= 0 {
			break
		}
		data = data[n:]
		f := pbField{Num: int(key >> 3), Wire: int(key & 7)}
		switch f.Wire {
		case 0:
			v, m := binary.Uvarint(data)
			if m <= 0 {
				return out
			}
			f.Varint = v
			data = data[m:]
		case 1:
			if len(data) < 8 {
				return out
			}
			data = data[8:]
		case 2:
			l, m := binary.Uvarint(data)
			if m <= 0 || uint64(len(data)-m) < l {
				return out
			}
			data = data[m:]
			f.Bytes = data[:l]
			data = data[l:]
		case 5:
			if len(data) < 4 {
				return out
			}
			data = data[4:]
		default:
			return out
		}
		out = append(out, f)
	}
	return out
}

func pbStringField(fields []pbField, num int) string {
	for _, f := range fields {
		if f.Num == num && f.Wire == 2 {
			return string(f.Bytes)
		}
	}
	return ""
}

// pbSubmessage returns the decoded fields of the first length-delimited
// submessage at field num.
func pbSubmessage(fields []pbField, num int) []pbField {
	for _, f := range fields {
		if f.Num == num && f.Wire == 2 {
			return pbFields(f.Bytes)
		}
	}
	return nil
}

// pbSubmessages returns decoded fields for every occurrence of field num.
func pbSubmessages(fields []pbField, num int) [][]pbField {
	var out [][]pbField
	for _, f := range fields {
		if f.Num == num && f.Wire == 2 {
			out = append(out, pbFields(f.Bytes))
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Devin wire messages (exa.* protobuf schemas).
// ---------------------------------------------------------------------------

const (
	devinMetadataIDEName      = "devin-cli"
	devinMetadataIDEType      = "chisel"
	devinMetadataIDEVersion   = "3000.11.205"
	devinRequestTypeCascade   = 5
	devinPlannerModeDefault   = 1
	devinPromptCacheEphemeral = 1
)

// devinMetadata encodes exa.codeium_common_pb.Metadata with the fields the
// Devin CLI client populates.
func devinMetadata(apiKey, userJWT string) []byte {
	var m []byte
	m = append(m, pbString(1, devinMetadataIDEName)...)
	m = append(m, pbString(7, devinMetadataIDEVersion)...)
	m = append(m, pbString(28, devinMetadataIDEType)...)
	m = append(m, pbString(12, "chisel")...)
	m = append(m, pbString(2, devinMetadataIDEVersion)...)
	m = append(m, pbString(3, apiKey)...)
	locale := "en"
	m = append(m, pbString(4, locale)...)
	m = append(m, pbString(5, runtime.GOOS)...)
	m = append(m, pbString(13, "go-choir/"+runtime.GOOS)...)
	m = append(m, pbString(10, devinSessionID())...)
	if userJWT != "" {
		m = append(m, pbString(21, userJWT)...)
	}
	return m
}

var devinSessionID = sync.OnceValue(func() string {
	return fmt.Sprintf("go-choir-%d", time.Now().UnixNano())
})

// devinGetUserJwtRequest encodes exa.auth_pb.GetUserJwtRequest{metadata}.
func devinGetUserJwtRequest(apiKey string) []byte {
	return pbMessage(1, devinMetadata(apiKey, ""))
}

// devinChatMessagePrompt encodes exa.chat_pb.ChatMessagePrompt.
// source: 1=USER, 2=SYSTEM(assistant), 4=TOOL.
func devinChatMessagePrompt(messageID string, source int, prompt, thinking, signature, toolCallID string, toolResultIsError bool, toolCalls [][]byte, images [][]byte) []byte {
	var m []byte
	m = append(m, pbString(1, messageID)...)
	m = append(m, pbVarintField(2, uint64(source))...)
	m = append(m, pbString(3, prompt)...)
	if toolCallID != "" {
		m = append(m, pbString(7, toolCallID)...)
	}
	if toolResultIsError {
		m = append(m, pbVarintField(9, 1)...)
	}
	for _, tc := range toolCalls {
		m = append(m, pbMessage(6, tc)...)
	}
	if thinking != "" {
		m = append(m, pbString(11, thinking)...)
	}
	if signature != "" {
		m = append(m, pbString(12, signature)...)
	}
	for _, img := range images {
		m = append(m, pbMessage(10, img)...)
	}
	return m
}

// devinToolCall encodes exa.codeium_common_pb.ChatToolCall.
func devinToolCall(id, name, argumentsJSON string) []byte {
	var t []byte
	t = append(t, pbString(1, id)...)
	t = append(t, pbString(2, name)...)
	t = append(t, pbString(3, argumentsJSON)...)
	return t
}

// devinImageData encodes exa.codeium_common_pb.ImageData.
func devinImageData(base64Data, mimeType string) []byte {
	var i []byte
	i = append(i, pbString(1, base64Data)...)
	i = append(i, pbString(2, mimeType)...)
	return i
}

// devinToolDefinition encodes exa.chat_pb.ChatToolDefinition.
func devinToolDefinition(name, description, jsonSchema string, strict bool) []byte {
	var t []byte
	t = append(t, pbString(1, name)...)
	t = append(t, pbString(2, description)...)
	t = append(t, pbString(3, jsonSchema)...)
	if strict {
		t = append(t, pbVarintField(12, 1)...)
	}
	return t
}

// devinCompletionConfiguration encodes exa.codeium_common_pb.
// CompletionConfiguration.
func devinCompletionConfiguration(maxTokens int) []byte {
	if maxTokens <= 0 {
		maxTokens = 64000
	}
	var c []byte
	c = append(c, pbVarintField(1, 1)...)                 // numCompletions
	c = append(c, pbVarintField(2, uint64(maxTokens))...) // maxTokens
	c = append(c, pbVarintField(3, 400)...)               // maxNewlines
	c = append(c, pbDouble(5, 1.0)...)                    // temperature
	c = append(c, pbVarintField(7, 40)...)                // topK
	c = append(c, pbDouble(8, 0.95)...)                   // topP
	return c
}

// devinMessageID derives the deterministic message id the Devin client uses:
// a hash of cascadeID, message index, and role.
func devinMessageID(cascadeID string, index int, role string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%d\x00%s", cascadeID, index, role)))
	return "m" + hex.EncodeToString(sum[:8])
}

// devinModelUID maps a Choir model request to the server-issued chatModelUid.
// Effort variants route through requestModelId-style suffixes: swe-2 and
// swe-2-flash accept -medium/-high/-max.
func devinModelUID(model, effort string) string {
	base := strings.TrimSpace(model)
	// Already an effort variant uid.
	for _, suffix := range []string{"-medium", "-high", "-max", "-lightning"} {
		if strings.HasSuffix(base, suffix) {
			return base
		}
	}
	switch base {
	case "swe-2", "swe-2-flash":
		e := strings.ToLower(strings.TrimSpace(effort))
		switch e {
		case "medium", "high", "max":
			return base + "-" + e
		case "low", "minimal":
			return base + "-medium"
		case "xhigh":
			return base + "-max"
		}
		if base == "swe-2-flash" {
			return "swe-2-flash-medium"
		}
		return "swe-2-high"
	default:
		return base
	}
}

// devinSupportsParallelToolCalls mirrors the catalog flags: swe-2-flash does
// not support parallel tool calls.
func devinSupportsParallelToolCalls(modelUID string) bool {
	return !strings.HasPrefix(modelUID, "swe-2-flash")
}

// devinCall is one tool call in a history turn.
type devinCall struct{ id, name, args string }

// devinTurn is one ChatMessagePrompt before encoding.
type devinTurn struct {
	index  int
	source int // 1 user, 2 assistant, 4 tool
	text   string
	toolID string
	isErr  bool
	calls  []devinCall
	images [][]byte
}

func (t devinTurn) encode(cascadeID string) []byte {
	var calls [][]byte
	for _, c := range t.calls {
		calls = append(calls, devinToolCall(c.id, c.name, c.args))
	}
	role := map[int]string{1: "user", 2: "assistant", 4: "tool\x00" + t.toolID}[t.source]
	return devinChatMessagePrompt(devinMessageID(cascadeID, t.index, role), t.source, t.text, "", "", t.toolID, t.isErr, calls, t.images)
}

// devinNoToolResult answers a call whose result the history never recorded
// (a carried or failed-over transcript can end a run on one).
const devinNoToolResult = "(no result was recorded for this call)"

// devinHistory converts a transcript, possibly begun on another provider,
// into Devin turns and system text:
//   - tool_result blocks (which the tool loop sends inside a user message)
//     become their own tool turns, in order, right after the assistant turn
//     that made the call;
//   - an unanswered tool_use gets a placeholder result and a result that
//     answers no earlier call becomes plain text, so the pairing the server
//     expects always holds;
//   - thinking blocks and ReasoningContent are dropped (never signed by
//     Devin, so never valid for it);
//   - images are sent only when allowImages (swe-2-flash is text only; an
//     omitted image leaves a text note); non-base64 sources are skipped.
func devinHistory(messages []Message, allowImages bool) (turns []devinTurn, system []string) {
	var open []string // call ids of the last assistant turn, unanswered so far
	flush := func(index int) {
		for _, id := range open {
			turns = append(turns, devinTurn{index: index, source: 4, toolID: id, text: devinNoToolResult})
		}
		open = nil
	}
	imageNote := false // an image of the current message was omitted
	images := func(b Block) [][]byte {
		if img := devinImageBlock(b.Source); img != nil {
			if allowImages {
				return [][]byte{img}
			}
			imageNote = true
		}
		return nil
	}
	for i, msg := range messages {
		switch msg.Role {
		case "assistant":
			flush(i)
			turn := devinTurn{index: i, source: 2}
			seen := map[string]bool{}
			for _, b := range msg.Content {
				switch b.Type {
				case "text":
					if b.Text != "" {
						if turn.text != "" {
							turn.text += "\n"
						}
						turn.text += b.Text
					}
				case "tool_use":
					if b.ID == "" || seen[b.ID] {
						continue
					}
					seen[b.ID] = true
					args := strings.TrimSpace(string(b.Input))
					if args == "" || !json.Valid([]byte(args)) {
						args = "{}"
					}
					turn.calls = append(turn.calls, devinCall{id: b.ID, name: b.Name, args: args})
					open = append(open, b.ID)
				}
			}
			if turn.text != "" || len(turn.calls) > 0 {
				turns = append(turns, turn)
			}
		case "system":
			text, _ := devinMessageContent(msg.Content)
			if text != "" {
				system = append(system, text)
			}
		default: // user, developer, and the legacy tool roles
			imageNote = false
			isOpen := map[string]bool{}
			for _, id := range open {
				isOpen[id] = true
			}
			answered := map[string]bool{}
			var results []devinTurn
			user := devinTurn{index: i, source: 1}
			addText := func(s string) {
				if s == "" {
					return
				}
				if user.text != "" {
					user.text += "\n"
				}
				user.text += s
			}
			for _, b := range msg.Content {
				switch b.Type {
				case "text":
					addText(b.Text)
				case "image":
					user.images = append(user.images, images(b)...)
				case "tool_result":
					text := b.Text
					if isOpen[b.ToolUseID] && !answered[b.ToolUseID] {
						answered[b.ToolUseID] = true
						if text == "" {
							text = "(empty result)"
						}
						results = append(results, devinTurn{index: i, source: 4, toolID: b.ToolUseID, isErr: b.IsError, text: text, images: images(b)})
					} else if text != "" {
						addText("[tool result] " + text)
					}
				}
			}
			// Results first, in the order of the calls they answer; a call
			// with no result here gets the placeholder.
			byID := map[string]devinTurn{}
			for _, r := range results {
				byID[r.toolID] = r
			}
			for _, id := range open {
				if r, ok := byID[id]; ok {
					turns = append(turns, r)
				} else {
					turns = append(turns, devinTurn{index: i, source: 4, toolID: id, text: devinNoToolResult})
				}
			}
			open = nil
			if imageNote {
				addText("[an image was omitted: this model reads text only]")
			}
			if user.text != "" || len(user.images) > 0 {
				turns = append(turns, user)
			}
		}
	}
	flush(len(messages))
	return turns, system
}

// devinMergeTurns folds consecutive user or assistant turns (the server
// rejects runs of three or more same-source text turns). Tool turns are
// never merged: each carries its own call id.
func devinMergeTurns(turns []devinTurn) []devinTurn {
	var out []devinTurn
	for _, t := range turns {
		if n := len(out); n > 0 && t.source != 4 && out[n-1].source == t.source {
			last := &out[n-1]
			if t.text != "" {
				if last.text != "" {
					last.text += "\n"
				}
				last.text += t.text
			}
			last.calls = append(last.calls, t.calls...)
			last.images = append(last.images, t.images...)
			continue
		}
		out = append(out, t)
	}
	return out
}

// devinMaxOutputTokens is swe-2-flash's catalogued maximum; a larger request
// is clamped, and none (0) takes the 64000 default.
const devinMaxOutputTokens = 131072

// devinBuildChatRequest encodes exa.api_server_pb.GetChatMessageRequest.
func (p *DevinProvider) devinBuildChatRequest(req LLMRequest, userJWT, cascadeID, executionID string) []byte {
	var out []byte
	out = append(out, pbMessage(1, devinMetadata(p.apiKey, userJWT))...)

	modelUID := p.devinModelUIDFor(req)
	turns, systemMsgs := devinHistory(req.Messages, !strings.HasPrefix(modelUID, "swe-2-flash"))
	var systemParts []string
	if s := strings.TrimSpace(req.System); s != "" {
		systemParts = append(systemParts, s)
	}
	systemParts = append(systemParts, systemMsgs...)
	if len(systemParts) > 0 {
		out = append(out, pbString(2, strings.Join(systemParts, "\n\n"))...)
	}
	for _, t := range devinMergeTurns(turns) {
		out = append(out, pbMessage(3, t.encode(cascadeID))...)
	}
	out = append(out, pbString(21, modelUID)...)
	out = append(out, pbVarintField(7, devinRequestTypeCascade)...)

	maxTokens := min(req.MaxTokens, devinMaxOutputTokens)
	out = append(out, pbMessage(8, devinCompletionConfiguration(maxTokens))...)

	if req.ToolChoice != "none" && len(req.Tools) > 0 {
		for _, t := range req.Tools {
			schema := "{}"
			if t.InputSchema != nil {
				if data, err := json.Marshal(t.InputSchema); err == nil {
					schema = string(data)
				}
			}
			out = append(out, pbMessage(10, devinToolDefinition(t.Name, t.Description, schema, false))...)
		}
		// Field 11 is "disable parallel tool calls": set for swe-2-flash,
		// which does not support them. The loop also tolerates several calls.
		out = append(out, pbVarintField(11, boolToVarint(!devinSupportsParallelToolCalls(modelUID)))...)
		out = append(out, pbMessage(12, devinToolChoice(req.ToolChoice))...)
	}
	// PromptCacheOptions{type: EPHEMERAL}
	out = append(out, pbMessage(13, pbVarintField(1, devinPromptCacheEphemeral))...)
	out = append(out, pbString(16, cascadeID)...)
	out = append(out, pbString(22, executionID)...)
	out = append(out, pbVarintField(20, devinPlannerModeDefault)...)
	return out
}

// devinModelUIDFor is the chatModelUid for a request: the request's model,
// else the provider's seed model, with the effort suffix.
func (p *DevinProvider) devinModelUIDFor(req LLMRequest) string {
	if uid := devinModelUID(req.Model, req.ReasoningEffort); uid != "" {
		return uid
	}
	return devinModelUID(p.modelID, req.ReasoningEffort)
}

// devinToolChoice encodes exa.chat_pb.ChatToolChoice: either
// optionName (field 1) or toolName (field 2). The shared modes map to
// "auto"/"any"; "function:<name>" maps to a named tool.
func devinToolChoice(choice string) []byte {
	switch c := strings.TrimSpace(choice); {
	case c == "" || c == "auto":
		return pbString(1, "auto")
	case c == "required":
		return pbString(1, "any")
	case strings.HasPrefix(c, "function:"):
		if name := strings.TrimSpace(strings.TrimPrefix(c, "function:")); name != "" {
			return pbString(2, name)
		}
		return pbString(1, "auto")
	default:
		return pbString(1, "auto")
	}
}

func boolToVarint(b bool) uint64 {
	if b {
		return 1
	}
	return 0
}

// devinMessageContent flattens text blocks and collects image blocks.
func devinMessageContent(blocks []Block) (string, [][]byte) {
	var text strings.Builder
	var images [][]byte
	for _, b := range blocks {
		switch b.Type {
		case "text":
			if text.Len() > 0 {
				text.WriteString("\n")
			}
			text.WriteString(b.Text)
		case "image":
			if img := devinImageBlock(b.Source); img != nil {
				images = append(images, img)
			}
		}
	}
	return text.String(), images
}

// devinImageBlock converts a MediaSource into ImageData wire bytes. Only
// inline base64 sources are supported; url/artifact_ref sources are skipped
// (Choir resolves those upstream when the provider advertises no URL input).
func devinImageBlock(src *MediaSource) []byte {
	if src == nil || src.Kind != "base64" || src.Data == "" {
		return nil
	}
	mime := src.MIMEType
	if mime == "" {
		mime = "image/png"
	}
	return devinImageData(src.Data, mime)
}

// ---------------------------------------------------------------------------
// Streaming
// ---------------------------------------------------------------------------

const (
	devinConnectFlagCompressed = 1
	devinConnectFlagTrailer    = 2
	// devinMaxFrameBytes bounds one frame, before and after gzip.
	devinMaxFrameBytes = 16 << 20
	// devinMaxStreamBytes bounds all frames of one response.
	devinMaxStreamBytes = 64 << 20
)

// Stream executes the request against GetChatMessage and emits StreamChunks.
// A 401, or an unauthenticated end-stream error before any output, refreshes
// the user JWT once and retries.
func (p *DevinProvider) Stream(ctx context.Context, req LLMRequest, onChunk func(StreamChunk)) (*LLMResponse, error) {
	userJWT, err := p.ensureUserJWT(ctx, "")
	if err != nil {
		return nil, err
	}
	cascadeID := strings.TrimSpace(req.ConversationID)
	if cascadeID == "" {
		cascadeID = strings.TrimSpace(req.CacheKey)
	}
	if cascadeID == "" {
		cascadeID = fmt.Sprintf("choir-%d", time.Now().UnixNano())
	}
	executionID := fmt.Sprintf("%d", time.Now().UnixNano())
	modelUID := p.devinModelUIDFor(req)

	emitted := false
	wrapped := onChunk
	if onChunk != nil {
		wrapped = func(c StreamChunk) { emitted = true; onChunk(c) }
	}
	for attempt := 0; ; attempt++ {
		resp, err := p.streamOnce(ctx, req, userJWT, cascadeID, executionID, modelUID, wrapped)
		var rpc *devinRPCError
		if err != nil && attempt == 0 && !emitted && errors.As(err, &rpc) && rpc.Code == "unauthenticated" {
			if userJWT, err = p.ensureUserJWT(ctx, userJWT); err != nil {
				return nil, err
			}
			continue
		}
		return resp, err
	}
}

func (p *DevinProvider) streamOnce(ctx context.Context, req LLMRequest, userJWT, cascadeID, executionID, modelUID string, onChunk func(StreamChunk)) (*LLMResponse, error) {
	requestBytes := p.devinBuildChatRequest(req, userJWT, cascadeID, executionID)
	compressed := len(requestBytes) >= 512*1024
	payload := requestBytes
	if compressed {
		var buf bytes.Buffer
		w := gzip.NewWriter(&buf)
		if _, err := w.Write(requestBytes); err != nil {
			return nil, fmt.Errorf("devin: gzip request: %w", err)
		}
		if err := w.Close(); err != nil {
			return nil, fmt.Errorf("devin: gzip request: %w", err)
		}
		payload = buf.Bytes()
	}
	const path = "/exa.api_server_pb.ApiServerService/GetChatMessage"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURLFor()+path, bytes.NewReader(devinConnectFrame(payload, 0)))
	if err != nil {
		return nil, fmt.Errorf("devin: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/connect+proto")
	httpReq.Header.Set("Connect-Protocol-Version", "1")
	httpReq.Header.Set("Accept-Encoding", "identity")
	httpReq.Header.Set("Connect-Accept-Encoding", "gzip")
	httpReq.Header.Set("User-Agent", "connect-go/1.18.1 (go1.26.3)")
	if compressed {
		httpReq.Header.Set("Connect-Content-Encoding", "gzip")
	}
	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("devin: http call: chat request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, p.devinHTTPError(path, resp)
	}
	return p.consumeChatStream(resp.Body, resp.Header, req, modelUID, onChunk)
}

// devinConnectFrame wraps a payload in the Connect streaming envelope:
// 1 byte flags + 4 byte big-endian length + payload.
func devinConnectFrame(payload []byte, flags byte) []byte {
	out := make([]byte, 5+len(payload))
	out[0] = flags
	binary.BigEndian.PutUint32(out[1:5], uint32(len(payload)))
	copy(out[5:], payload)
	return out
}

// consumeChatStream reads Connect-framed GetChatMessageResponse messages.
// A hostile or truncated length prefix cannot allocate more than the bytes
// that actually arrive (capped per frame and per stream); a stream that ends
// without its end-stream frame is an error, not a short answer.
func (p *DevinProvider) consumeChatStream(body io.Reader, h http.Header, req LLMRequest, modelUID string, onChunk func(StreamChunk)) (*LLMResponse, error) {
	result := &LLMResponse{
		Model:        coalesce(req.Model, modelUID),
		ProviderName: "devin",
		Usage:        Usage{CacheReported: true},
	}

	var text, reasoning strings.Builder
	toolCalls := map[string]*devinAccumToolCall{}
	var toolOrder []string
	blockIndex := 0
	textBlockOpen, thinkBlockOpen := false, false
	started := false
	var lastStop uint64
	var total int64
	sawEnd := false

	reader := bufio.NewReaderSize(body, 64*1024)
	for !sawEnd {
		var header [5]byte
		if _, err := io.ReadFull(reader, header[:]); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("devin: read stream frame: %w", err)
		}
		flags := header[0]
		length := binary.BigEndian.Uint32(header[1:5])
		if length > devinMaxFrameBytes {
			return nil, fmt.Errorf("devin: stream frame length %d exceeds %d cap", length, devinMaxFrameBytes)
		}
		if total += int64(length) + 5; total > devinMaxStreamBytes {
			return nil, fmt.Errorf("devin: stream exceeds %d byte cap", devinMaxStreamBytes)
		}
		payload, err := io.ReadAll(io.LimitReader(reader, int64(length)))
		if err != nil {
			return nil, fmt.Errorf("devin: read stream payload: %w", err)
		}
		if uint32(len(payload)) != length {
			return nil, fmt.Errorf("devin: stream truncated inside a frame")
		}
		if flags&devinConnectFlagCompressed != 0 {
			zr, err := gzip.NewReader(bytes.NewReader(payload))
			if err != nil {
				return nil, fmt.Errorf("devin: decompress stream frame: %w", err)
			}
			payload, err = io.ReadAll(io.LimitReader(zr, devinMaxFrameBytes+1))
			zr.Close()
			if err != nil {
				return nil, fmt.Errorf("devin: decompress stream frame: %w", err)
			}
			if len(payload) > devinMaxFrameBytes {
				return nil, fmt.Errorf("devin: decompressed frame exceeds %d cap", devinMaxFrameBytes)
			}
		}
		if flags&devinConnectFlagTrailer != 0 {
			sawEnd = true
			if err := p.devinTrailerError(payload, h); err != nil {
				return nil, err
			}
			continue
		}

		fields := pbFields(payload)
		if !started {
			started = true
			result.ID = pbStringField(fields, 1)
			if onChunk != nil {
				onChunk(StreamChunk{Type: "message_start", Model: result.Model, ID: result.ID})
			}
		}
		for _, f := range fields {
			switch f.Num {
			case 1: // messageId
				if result.ID == "" {
					result.ID = string(f.Bytes)
				}
			case 3: // deltaText
				delta := string(f.Bytes)
				if delta == "" {
					continue
				}
				if thinkBlockOpen {
					thinkBlockOpen = false
					blockIndex++
				}
				if !textBlockOpen {
					textBlockOpen = true
					if onChunk != nil {
						onChunk(StreamChunk{Type: "content_block_start", Index: blockIndex})
					}
				}
				text.WriteString(delta)
				if onChunk != nil {
					onChunk(StreamChunk{Type: "content_block_delta", Index: blockIndex, Delta: delta})
				}
			case 5: // stopReason (varint enum)
				lastStop = f.Varint
			case 6: // deltaToolCalls (repeated ChatToolCall)
				p.consumeToolCallDelta(f.Bytes, toolCalls, &toolOrder, &blockIndex, &textBlockOpen, &thinkBlockOpen, onChunk)
			case 7: // usage (ModelUsageStats)
				p.consumeUsage(f.Bytes, result)
			case 9: // deltaThinking
				delta := string(f.Bytes)
				if delta == "" {
					continue
				}
				if textBlockOpen {
					textBlockOpen = false
					blockIndex++
				}
				if !thinkBlockOpen {
					thinkBlockOpen = true
					if onChunk != nil {
						onChunk(StreamChunk{Type: "content_block_start", Index: blockIndex})
					}
				}
				reasoning.WriteString(delta)
				if onChunk != nil {
					onChunk(StreamChunk{Type: "content_block_delta", Index: blockIndex, ReasoningDelta: delta})
				}
			case 23: // actualModelUid
				if uid := strings.TrimSpace(string(f.Bytes)); uid != "" {
					result.Model = uid
				}
			}
		}
	}
	if !sawEnd {
		return nil, fmt.Errorf("devin: stream ended without an end-stream frame (truncated)")
	}

	for _, id := range toolOrder {
		tc := toolCalls[id]
		args := strings.TrimSpace(tc.argsJSON)
		if args == "" {
			args = "{}"
		}
		if tc.name == "" || !json.Valid([]byte(args)) {
			// A cut-off or nameless call must not run with guessed arguments.
			return nil, fmt.Errorf("devin: stream carried an incomplete tool call (truncated)")
		}
		result.ToolCalls = append(result.ToolCalls, ContentToolCall{ID: tc.id, Name: tc.name, Arguments: json.RawMessage(args)})
		if onChunk != nil {
			onChunk(StreamChunk{Type: "toolcall_end", ToolCallID: tc.id, ToolCallName: tc.name, Index: tc.blockIndex})
		}
	}
	if onChunk != nil && (textBlockOpen || thinkBlockOpen || len(toolCalls) > 0) {
		onChunk(StreamChunk{Type: "content_block_stop", Index: blockIndex})
	}
	if text.Len() == 0 && len(result.ToolCalls) == 0 && reasoning.Len() == 0 {
		return nil, fmt.Errorf("devin: status 502 empty_response (sanitized): the stream carried no content")
	}

	result.Text = text.String()
	result.ReasoningContent = reasoning.String()
	result.StopReason = devinStopReason(lastStop, len(result.ToolCalls) > 0)
	if result.ID == "" {
		result.ID = fmt.Sprintf("devin-%d", time.Now().UnixNano())
	}
	if onChunk != nil {
		if result.Usage.OutputTokens > 0 {
			onChunk(StreamChunk{Type: "message_delta", StopReason: result.StopReason, Usage: result.Usage.streamUsage()})
		}
		onChunk(StreamChunk{Type: "message_stop", StopReason: result.StopReason})
	}
	return result, nil
}

type devinAccumToolCall struct {
	id         string
	name       string
	argsJSON   string
	blockIndex int
}

// consumeToolCallDelta merges one ChatToolCall proto into the accumulator.
// argumentsJson arrives as cumulative snapshots; keep the longest and emit
// only the new suffix.
func (p *DevinProvider) consumeToolCallDelta(raw []byte, calls map[string]*devinAccumToolCall, order *[]string, blockIndex *int, textOpen, thinkOpen *bool, onChunk func(StreamChunk)) {
	fields := pbFields(raw)
	id := pbStringField(fields, 1)
	name := pbStringField(fields, 2)
	args := pbStringField(fields, 3)
	if id == "" {
		return
	}
	tc, ok := calls[id]
	if !ok {
		if *textOpen || *thinkOpen {
			*textOpen, *thinkOpen = false, false
			*blockIndex++
		}
		tc = &devinAccumToolCall{id: id, blockIndex: *blockIndex}
		*blockIndex++
		calls[id] = tc
		*order = append(*order, id)
		if onChunk != nil {
			onChunk(StreamChunk{Type: "toolcall_start", ToolCallID: id, Index: tc.blockIndex})
		}
	}
	if name != "" {
		tc.name = name
	}
	if len(args) > len(tc.argsJSON) {
		delta := args
		if strings.HasPrefix(args, tc.argsJSON) {
			delta = args[len(tc.argsJSON):]
		}
		tc.argsJSON = args
		if onChunk != nil && tc.name != "" {
			onChunk(StreamChunk{Type: "toolcall_delta", ToolCallDelta: delta, ToolCallID: id, ToolCallName: tc.name, Index: tc.blockIndex})
		}
	}
}

// consumeUsage reads exa.codeium_common_pb.ModelUsageStats: field 2 input,
// 3 output, 5 cache read (cache writes, field 4, are not tracked). The
// stream gives all three counts; a response with no usage message leaves
// Usage zero, which no cost or budget path treats as an error (Devin is
// priced at zero until its promotions end).
func (p *DevinProvider) consumeUsage(raw []byte, result *LLMResponse) {
	for _, f := range pbFields(raw) {
		if f.Wire != 0 {
			continue
		}
		v := int(min(f.Varint, math.MaxInt32))
		switch f.Num {
		case 2:
			result.Usage.InputTokens = v
		case 3:
			result.Usage.OutputTokens = v
		case 5:
			result.Usage.CachedInputTokens = v
		}
	}
}

// devinStopReason maps the wire stopReason enum onto the shared vocabulary.
// Wire values (exa.api_server_pb): 1 INCOMPLETE, 2 STOP_PATTERN,
// 3 MAX_TOKENS, 9 PARTIAL, 10 FUNCTION_CALL, 11 CONTENT_FILTER, 13 ERROR.
func devinStopReason(stop uint64, hadToolCalls bool) string {
	if hadToolCalls {
		return "tool_use"
	}
	switch stop {
	case 1, 3, 9: // INCOMPLETE, MAX_TOKENS, PARTIAL
		return "max_tokens"
	case 10: // FUNCTION_CALL without parsed tool calls
		return "tool_use"
	default:
		return "end_turn"
	}
}

// devinTrailerError parses an end-stream frame
// ({"error":{"code","message","details":[...]},"metadata":{...}}) into a
// classified error; nil when the stream ended cleanly.
func (p *DevinProvider) devinTrailerError(payload []byte, h http.Header) error {
	if len(payload) == 0 {
		return nil
	}
	var trailer struct {
		Error    *devinErrorBody     `json:"error"`
		Metadata map[string][]string `json:"metadata"`
	}
	if err := json.Unmarshal(payload, &trailer); err != nil || trailer.Error == nil {
		return nil
	}
	return p.devinClassify("GetChatMessage", 0, h, trailer.Metadata, *trailer.Error)
}
