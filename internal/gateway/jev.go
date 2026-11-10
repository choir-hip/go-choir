package gateway

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	jevModel                = "~typesafe/jev-latest"
	openRouterDecisionsURL  = "https://openrouter.ai/api/alpha/decisions"
	jevTransportTimeout     = 2 * time.Minute
	jevTransportMaxBodySize = 1 << 20
)

// JudgmentRequest is the gateway wire contract for a Jev decision. The model
// is deliberately absent: the gateway pins it to ~typesafe/jev-latest
// (owner, 2026-10-16), and each receipt records the concrete model served.
type JudgmentRequest struct {
	State     json.RawMessage            `json:"state"`
	Questions map[string]json.RawMessage `json:"questions"`
	// Scope names the priority class of the request: "interactive" draws on
	// the share reserved for the prompt router, the disclosure gate and a
	// desk's own question; every other scope is bulk. It is never sent
	// upstream.
	Scope string `json:"scope,omitempty"`
}

// JudgmentScopeIntakeLabels is the scope of intake labels (bulk).
const JudgmentScopeIntakeLabels = "intake_labels"

// JevTransport calls OpenRouter's Decisions alpha endpoint without exposing
// the provider credential to VM guests.
type JevTransport struct {
	enabled    bool
	apiKey     string
	httpClient *http.Client
	receipts   *jevReceiptWriter
	// url is the upstream; a test points it at a stub.
	url string
	// now is the hold's clock.
	now func() time.Time
	// hold is the memory of the upstream failing: no call goes out while
	// it holds (upstream_hold.go).
	hold upstreamHold
	// limiter is the one rate and in-flight limit every door shares
	// (jev_limiter.go); built on first use so a literal transport has one.
	limiterOnce sync.Once
	limiter     *jevLimiter
	stats       jevStats
}

func (t *JevTransport) limit() *jevLimiter {
	t.limiterOnce.Do(func() {
		if t.limiter == nil {
			t.limiter = newJevLimiter()
		}
	})
	return t.limiter
}

// newJevHTTPClient keeps connections to the provider warm: the default
// transport keeps 2 idle connections per host, so 12 concurrent judgments
// would each open a fresh TLS connection.
func newJevHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = 128
	transport.MaxIdleConnsPerHost = 64
	return &http.Client{Timeout: jevTransportTimeout, Transport: transport}
}

func NewJevTransportFromEnv(enabled bool) *JevTransport {
	return &JevTransport{
		enabled:    enabled,
		apiKey:     strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY")),
		httpClient: newJevHTTPClient(),
		receipts:   newJevReceiptWriter(strings.TrimSpace(os.Getenv("GATEWAY_JEV_RECEIPT_PATH"))),
		url:        openRouterDecisionsURL,
		now:        time.Now,
	}
}

func (t *JevTransport) Enabled() bool {
	return t != nil && t.enabled
}

// Decide returns the exact successful OpenRouter response body. In particular,
// answer probability distributions are neither selected nor normalized here.
func (t *JevTransport) Decide(ctx context.Context, computerID string, request JudgmentRequest) ([]byte, error) {
	if !t.Enabled() {
		return nil, errJevTransportDisabled
	}
	if t.apiKey == "" {
		return nil, fmt.Errorf("OpenRouter credential is unavailable")
	}
	if err := validateJudgmentRequest(request); err != nil {
		return nil, err
	}
	// An upstream that just failed is not asked again until its hold ends.
	if until := t.hold.heldUntil(t.clock()); !until.IsZero() {
		return nil, &upstreamHeldError{name: "Jev judgment provider", until: until}
	}

	class, _ := judgmentRateScope(request.Scope)
	release, queued, err := t.limit().acquire(ctx, computerID, class)
	if err != nil {
		return nil, err
	}
	defer release()
	started := time.Now()
	response, status, err := t.call(ctx, computerID, request)
	upstream := time.Since(started)
	logJevCall(computerID, class, len(request.Questions), queued, upstream, status, err)
	if line := t.stats.observe(time.Now(), upstream, queued, status, err == nil); line != "" {
		log.Print(line)
	}
	return response, err
}

// call is one upstream request under a held limiter slot. It returns the
// upstream status (0 when none arrived).
func (t *JevTransport) call(ctx context.Context, computerID string, request JudgmentRequest) ([]byte, int, error) {
	upstreamRequest, err := json.Marshal(struct {
		Model     string                     `json:"model"`
		State     json.RawMessage            `json:"state"`
		Questions map[string]json.RawMessage `json:"questions"`
	}{
		Model:     jevModel,
		State:     request.State,
		Questions: request.Questions,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("encode Jev request: %w", err)
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, t.upstreamURL(), bytes.NewReader(upstreamRequest))
	if err != nil {
		return nil, 0, fmt.Errorf("create OpenRouter request: %w", err)
	}
	httpRequest.Header.Set("Authorization", "Bearer "+t.apiKey)
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Accept", "application/json")

	response, err := t.httpClient.Do(httpRequest)
	if err != nil {
		if ctx.Err() == nil {
			t.hold.fail(t.clock(), false)
		}
		return nil, 0, fmt.Errorf("call OpenRouter: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(response.Body, jevTransportMaxBodySize+1))
	if err != nil {
		return nil, response.StatusCode, fmt.Errorf("read OpenRouter response: %w", err)
	}
	if len(body) > jevTransportMaxBodySize {
		return nil, response.StatusCode, fmt.Errorf("OpenRouter response exceeds transport limit")
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		switch code := response.StatusCode; {
		case isHardUpstreamStatus(code):
			t.hold.fail(t.clock(), true)
		case code == http.StatusTooManyRequests:
			// A rate answer is not an outage: one shared backoff for the
			// time the provider names, not a minute-long hold on every
			// computer.
			t.limit().backoff(parseRetryAfter(response.Header, t.clock()))
		case code >= 500 || code == http.StatusRequestTimeout:
			t.hold.fail(t.clock(), false)
		}
		return nil, response.StatusCode, fmt.Errorf("OpenRouter returned %s", response.Status)
	}
	t.hold.succeed()
	if !json.Valid(body) {
		return nil, response.StatusCode, fmt.Errorf("OpenRouter returned invalid JSON")
	}
	var responseIdentity struct {
		ID    string `json:"id"`
		Model string `json:"model"`
	}
	_ = json.Unmarshal(body, &responseIdentity)
	if err := t.receipts.write(jevTransportReceipt{
		Schema:    "choir.jev_transport_receipt.v1",
		CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Request: jevReceiptRequest{
			ComputerID: computerID,
			Model:      jevModel,
			Digest:     sha256Digest(upstreamRequest),
		},
		Response: jevReceiptResponse{
			Status:            response.StatusCode,
			OpenRouterRequest: strings.TrimSpace(response.Header.Get("X-Request-ID")),
			ResponseID:        responseIdentity.ID,
			Model:             responseIdentity.Model,
			Digest:            sha256Digest(body),
			Distribution:      json.RawMessage(body),
		},
	}); err != nil {
		return nil, response.StatusCode, fmt.Errorf("write Jev transport receipt: %w", err)
	}
	return body, response.StatusCode, nil
}

func validateJudgmentRequest(request JudgmentRequest) error {
	if len(request.State) == 0 || !json.Valid(request.State) || !jsonObject(request.State) {
		return fmt.Errorf("state must be a JSON object")
	}
	if len(request.Questions) == 0 {
		return fmt.Errorf("questions are required")
	}
	for name, question := range request.Questions {
		if strings.TrimSpace(name) == "" || !json.Valid(question) || !jsonObject(question) {
			return fmt.Errorf("questions must contain named JSON objects")
		}
		var shape struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(question, &shape); err != nil {
			return fmt.Errorf("decode question: %w", err)
		}
		switch shape.Type {
		case "noul", "choice", "score":
		default:
			return fmt.Errorf("question %q has unsupported type", name)
		}
	}
	return nil
}

func jsonObject(raw json.RawMessage) bool {
	return len(bytes.TrimSpace(raw)) > 0 && bytes.TrimSpace(raw)[0] == '{'
}

func sha256Digest(body []byte) string {
	digest := sha256.Sum256(body)
	return hex.EncodeToString(digest[:])
}

type jevTransportDisabledError struct{}

func (jevTransportDisabledError) Error() string { return "Jev judgments are disabled" }

var errJevTransportDisabled error = jevTransportDisabledError{}

type jevTransportReceipt struct {
	Schema    string             `json:"schema"`
	CreatedAt string             `json:"created_at"`
	Request   jevReceiptRequest  `json:"request"`
	Response  jevReceiptResponse `json:"response"`
}

type jevReceiptRequest struct {
	ComputerID string `json:"computer_id"`
	Model      string `json:"model"`
	Digest     string `json:"digest"`
}

type jevReceiptResponse struct {
	Status            int             `json:"status"`
	OpenRouterRequest string          `json:"openrouter_request_id,omitempty"`
	ResponseID        string          `json:"response_id,omitempty"`
	Model             string          `json:"model,omitempty"`
	Digest            string          `json:"digest"`
	Distribution      json.RawMessage `json:"distribution"`
}

// jevReceiptWriter appends one self-contained diagnostic receipt per returned
// distribution. It is intentionally separate from commitment_score writes,
// which are M5's canonical-ledger responsibility.
type jevReceiptWriter struct {
	path string
	mu   sync.Mutex
}

func newJevReceiptWriter(path string) *jevReceiptWriter {
	if path == "" {
		path = "/var/lib/go-choir/gateway/jev-transport-receipts.jsonl"
	}
	return &jevReceiptWriter{path: path}
}

func (w *jevReceiptWriter) write(receipt jevTransportReceipt) error {
	if w == nil || strings.TrimSpace(w.path) == "" {
		return fmt.Errorf("receipt path is unavailable")
	}
	line, err := json.Marshal(receipt)
	if err != nil {
		return fmt.Errorf("encode receipt: %w", err)
	}
	file, err := w.append(line)
	if err != nil {
		return err
	}
	// The fsync is outside the mutex: concurrent calls each wait for their
	// own receipt to be durable without queueing behind one another's.
	syncErr := file.Sync()
	_ = file.Close()
	return syncErr
}

// append writes the line under the mutex, so lines never interleave, and
// returns the open file for the caller to sync.
func (w *jevReceiptWriter) append(line []byte) (*os.File, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(w.path), 0o700); err != nil {
		return nil, fmt.Errorf("create receipt directory: %w", err)
	}
	file, err := os.OpenFile(w.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open receipt file: %w", err)
	}
	if _, err := file.Write(append(line, '\n')); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("append receipt: %w", err)
	}
	return file, nil
}

// jevPeerAllowed is the Jev route's pin check. A mismatch is refused in
// every binding mode, as it always was, and an unpinned bearer is pinned to
// its first caller in every mode but strict: that first-use pin is Jev's own
// boundary from before SEC-05, so observe keeps it rather than serving a
// copied bearer until the host's ensure (SEC-05 record, finding 14). Strict
// refuses an unpinned bearer, so the Jev route cannot write the shared pin
// strict exists to withhold (finding 13).
func (h *Handler) jevPeerAllowed(computerID string, r *http.Request) bool {
	peerIP := remoteIP(r)
	if peerIP == nil {
		return false
	}
	bind := h.bearerBindingMode() != BindingStrict
	state, _, err := h.registry.CheckPeer(computerID, peerIP.String(), bind)
	if err != nil {
		return false
	}
	return state == PeerMatch || state == PeerBound
}

// HandleJudgment serves POST /provider/v1/judgments. It only exposes the
// pinned Jev Decisions transport to authenticated runtime peers; it never
// accepts a caller-selected model or emits a commitment_score.
func (h *Handler) HandleJudgment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeGatewayJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "method not allowed"})
		return
	}
	if h.jevTransport == nil || !h.jevTransport.Enabled() {
		writeGatewayJSON(w, http.StatusServiceUnavailable, ErrorResponse{Error: "Jev judgments are disabled"})
		return
	}
	computerID, err := h.authenticateBearer(r)
	if err != nil {
		writeGatewayJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "authentication required"})
		return
	}
	if !h.jevPeerAllowed(computerID, r) {
		writeGatewayJSON(w, http.StatusForbidden, ErrorResponse{Error: "Jev bearer is not authorized for this VM"})
		return
	}
	// The body is read first: the request's scope names its priority class.
	var request JudgmentRequest
	if err := h.decodeBody(w, r, bodyRouteJudgment, computerID, &request, false); err != nil {
		if status, ok := bodyRefusal(w, err); ok {
			writeGatewayJSON(w, status, ErrorResponse{Error: err.Error()})
			return
		}
		writeGatewayJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid judgment request"})
		return
	}
	if _, err := judgmentRateScope(request.Scope); err != nil {
		writeGatewayJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	if err := validateJudgmentRequest(request); err != nil {
		writeGatewayJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), jevTransportTimeout)
	defer cancel()
	response, err := h.jevTransport.Decide(ctx, computerID, request)
	if err != nil {
		// The cause (an OpenRouter status, a transport error) carries no
		// credential; without it a spent key read only as "unavailable".
		log.Printf("gateway: Jev judgment for %s failed: %v", computerID, err)
		if _, disabled := err.(jevTransportDisabledError); disabled {
			writeGatewayJSON(w, http.StatusServiceUnavailable, ErrorResponse{Error: "Jev judgments are disabled"})
			return
		}
		if limited, ok := err.(*jevLimitedError); ok {
			w.Header().Set("Retry-After", fmt.Sprintf("%d", max(1, int(limited.retryAfter.Seconds()+0.999))))
			writeGatewayJSON(w, http.StatusTooManyRequests, ErrorResponse{Error: "Jev judgment rate limit exceeded"})
			return
		}
		if held, ok := err.(*upstreamHeldError); ok {
			w.Header().Set("Retry-After", fmt.Sprintf("%d", held.retryAfterSeconds(time.Now())))
			writeGatewayJSON(w, http.StatusServiceUnavailable, ErrorResponse{Error: "Jev judgment provider unavailable (held after repeated failures)"})
			return
		}
		writeGatewayJSON(w, http.StatusBadGateway, ErrorResponse{Error: "Jev judgment provider unavailable"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(response); err != nil {
		// The receipt was already made durable before this write. The caller can
		// safely retry its idempotent transport request if it lost the response.
		return
	}
}

func (t *JevTransport) clock() time.Time {
	if t.now != nil {
		return t.now()
	}
	return time.Now()
}

func (t *JevTransport) upstreamURL() string {
	if t.url != "" {
		return t.url
	}
	return openRouterDecisionsURL
}
