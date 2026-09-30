package gateway

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	jevModel                = "typesafe/jev-1.13"
	openRouterDecisionsURL  = "https://openrouter.ai/api/alpha/decisions"
	jevTransportTimeout     = 2 * time.Minute
	jevTransportMaxBodySize = 1 << 20
)

// JudgmentRequest is the gateway wire contract for a Jev decision. The model
// is deliberately absent: the gateway pins it to typesafe/jev-1.13.
type JudgmentRequest struct {
	State     json.RawMessage            `json:"state"`
	Questions map[string]json.RawMessage `json:"questions"`
}

// JevTransport calls OpenRouter's Decisions alpha endpoint without exposing
// the provider credential to VM guests.
type JevTransport struct {
	enabled    bool
	apiKey     string
	httpClient *http.Client
	receipts   *jevReceiptWriter
}

func NewJevTransportFromEnv(enabled bool) *JevTransport {
	return &JevTransport{
		enabled:    enabled,
		apiKey:     strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY")),
		httpClient: &http.Client{Timeout: jevTransportTimeout},
		receipts:   newJevReceiptWriter(strings.TrimSpace(os.Getenv("GATEWAY_JEV_RECEIPT_PATH"))),
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

	upstreamRequest, err := json.Marshal(struct {
		Model     string                         `json:"model"`
		State     json.RawMessage                `json:"state"`
		Questions map[string]json.RawMessage     `json:"questions"`
	}{
		Model:     jevModel,
		State:     request.State,
		Questions: request.Questions,
	})
	if err != nil {
		return nil, fmt.Errorf("encode Jev request: %w", err)
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, openRouterDecisionsURL, bytes.NewReader(upstreamRequest))
	if err != nil {
		return nil, fmt.Errorf("create OpenRouter request: %w", err)
	}
	httpRequest.Header.Set("Authorization", "Bearer "+t.apiKey)
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Accept", "application/json")

	response, err := t.httpClient.Do(httpRequest)
	if err != nil {
		return nil, fmt.Errorf("call OpenRouter: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(response.Body, jevTransportMaxBodySize+1))
	if err != nil {
		return nil, fmt.Errorf("read OpenRouter response: %w", err)
	}
	if len(body) > jevTransportMaxBodySize {
		return nil, fmt.Errorf("OpenRouter response exceeds transport limit")
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("OpenRouter returned %s", response.Status)
	}
	if !json.Valid(body) {
		return nil, fmt.Errorf("OpenRouter returned invalid JSON")
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
		return nil, fmt.Errorf("write Jev transport receipt: %w", err)
	}
	return body, nil
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
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(w.path), 0o700); err != nil {
		return fmt.Errorf("create receipt directory: %w", err)
	}
	file, err := os.OpenFile(w.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open receipt file: %w", err)
	}
	defer func() { _ = file.Close() }()
	if _, err := file.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("append receipt: %w", err)
	}
	return file.Sync()
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
	computerID, err := h.authenticateAutoputer(r)
	if err != nil {
		writeGatewayJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "authentication required"})
		return
	}
	peerIP := remoteIP(r)
	if peerIP == nil || h.registry.BindJevPeer(computerID, peerIP.String()) != nil {
		writeGatewayJSON(w, http.StatusForbidden, ErrorResponse{Error: "Jev bearer is not authorized for this VM"})
		return
	}
	if h.rateLimiter != nil {
		bucket := rateLimitBucketKey(computerID, "judgments")
		if !h.rateLimiter.Record(bucket) {
			_, _, resetIn := h.rateLimiter.Status(bucket)
			retrySeconds := int(resetIn.Seconds())
			if retrySeconds < 1 {
				retrySeconds = 1
			}
			w.Header().Set("Retry-After", fmt.Sprintf("%d", retrySeconds))
			writeGatewayJSON(w, http.StatusTooManyRequests, ErrorResponse{Error: "Jev judgment rate limit exceeded"})
			return
		}
	}

	var request JudgmentRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, jevTransportMaxBodySize))
	if err := decoder.Decode(&request); err != nil {
		writeGatewayJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid judgment request"})
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
		if _, disabled := err.(jevTransportDisabledError); disabled {
			writeGatewayJSON(w, http.StatusServiceUnavailable, ErrorResponse{Error: "Jev judgments are disabled"})
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
