package pii

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// SLMRedactor is the local small-language-model redaction actor.
//
// It calls a local Ollama instance (7B-or-smaller model) to identify PII that
// deterministic regex misses — names, free-form addresses, prose-embedded
// identifiers — while preserving the surrounding semantic structure that the
// supervision and self-learning layers depend on.
//
// The SLM is invoked via Ollama's /api/chat endpoint with a strict prompt that
// requires a JSON array of findings. Each finding carries a class label and
// the exact substring to redact. The redactor locates each substring in the
// source text and replaces it with the class's redaction token. Substring
// matching (not byte offsets from the model) keeps the redactor robust to
// model offset drift and tokenization quirks.
//
// Failure mode: if Ollama is unreachable, returns an error, or produces a
// malformed response, SLMRedactor degrades to the configured fallback
// (RegexRedactor by default) so the ingestion invariant — never store raw
// PII — holds even during model outages. Callers may also set Fallback=nil to
// fail closed when the SLM is required.
//
// Concurrency: safe for concurrent use; the HTTP client and regex fallback are
// both concurrency-safe.
type SLMRedactor struct {
	baseURL  string
	model    string
	client   *http.Client
	fallback Redactor
	prompt   string
}

// SLMOption configures an SLMRedactor.
type SLMOption func(*SLMRedactor)

// WithSLMBaseURL overrides the Ollama base URL (default http://localhost:11434).
func WithSLMBaseURL(url string) SLMOption {
	return func(s *SLMRedactor) { s.baseURL = strings.TrimRight(url, "/") }
}

// WithSLMTimeout sets the per-request HTTP timeout (default 30s).
func WithSLMTimeout(d time.Duration) SLMOption {
	return func(s *SLMRedactor) { s.client.Timeout = d }
}

// WithSLMFallback sets the fallback Redactor used when the model is
// unavailable or returns a malformed response. Pass nil to fail closed.
func WithSLMFallback(r Redactor) SLMOption {
	return func(s *SLMRedactor) { s.fallback = r }
}

// WithSLMPrompt overrides the system prompt. The prompt must instruct the
// model to return a JSON array of {"class","text"} objects where text is the
// exact PII substring to redact.
func WithSLMPrompt(p string) SLMOption {
	return func(s *SLMRedactor) { s.prompt = p }
}

// NewSLMRedactor returns an SLMRedactor for the given Ollama model name (e.g.
// "llama3.2:3b", "qwen2.5:7b"). The model must be pulled locally.
func NewSLMRedactor(model string, opts ...SLMOption) *SLMRedactor {
	s := &SLMRedactor{
		baseURL: defaultSLMBaseURL,
		model:   model,
		client:  &http.Client{Timeout: defaultSLMTimeout},
		fallback: NewRegexRedactor(),
		prompt:   defaultSLMPrompt,
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

const (
	defaultSLMBaseURL = "http://localhost:11434"
	defaultSLMTimeout = 30 * time.Second
)

// Name implements Redactor.
func (s *SLMRedactor) Name() string { return "slm-ollama:" + s.model }

// defaultSLMPrompt is the system instruction sent to the local model. It is
// deliberately strict about output format to make substring-based redaction
// reliable.
const defaultSLMPrompt = `You are a PII redaction engine. Identify personally identifiable information in the user text. Return ONLY a JSON array (no prose, no markdown fences) of objects: {"class": "<class>", "text": "<exact substring>"}. Class must be one of: email, phone, ssn, credit_card, api_key, ip, name, address, credential. The text field MUST be the exact substring as it appears in the input, copied verbatim, so it can be located and replaced. If no PII is present, return []. Do not redact non-PII identifiers like UUIDs, run ids, or trajectory ids. Preserve all non-PII text.`

// slmChatRequest is the Ollama /api/chat payload.
type slmChatRequest struct {
	Model    string          `json:"model"`
	Stream   bool            `json:"stream"`
	Messages []slmChatMessage `json:"messages"`
}

type slmChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// slmChatResponse is a subset of the Ollama /api/chat response.
type slmChatResponse struct {
	Message slmChatMessage `json:"message"`
}

// slmFinding is the model's emitted finding. Text is the exact substring to
// redact; Class is the label.
type slmFinding struct {
	Class string `json:"class"`
	Text  string `json:"text"`
}

// RedactText implements Redactor. It calls the local SLM, parses the JSON
// findings, locates each substring in the source, and replaces it with the
// class's redaction token. On any error it falls back to the configured
// fallback redactor (if any).
func (s *SLMRedactor) RedactText(text string) (string, []Finding, error) {
	if text == "" {
		return text, nil, nil
	}

	redacted, findings, err := s.redactViaSLM(context.Background(), text)
	if err == nil {
		return redacted, findings, nil
	}

	if s.fallback != nil {
		// Degrade to the deterministic path. The ingestion invariant
		// (never store raw PII) takes precedence over SLM coverage.
		return s.fallback.RedactText(text)
	}
	return text, nil, fmt.Errorf("slm redactor: %w (no fallback configured)", err)
}

func (s *SLMRedactor) redactViaSLM(ctx context.Context, text string) (string, []Finding, error) {
	body, err := json.Marshal(slmChatRequest{
		Model: s.model,
		Stream: false,
		Messages: []slmChatMessage{
			{Role: "system", Content: s.prompt},
			{Role: "user", Content: text},
		},
	})
	if err != nil {
		return "", nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return "", nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return "", nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(raw))
	}

	var out slmChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", nil, fmt.Errorf("decode response: %w", err)
	}

	content := strings.TrimSpace(out.Message.Content)
	// Tolerate models that wrap JSON in markdown fences despite the prompt.
	content = stripCodeFences(content)
	if content == "" {
		return text, nil, nil
	}

	var emitted []slmFinding
	if err := json.Unmarshal([]byte(content), &emitted); err != nil {
		return "", nil, fmt.Errorf("parse model output as JSON array: %w (content=%q)", err, content)
	}

	findings := locateFindings(text, emitted)
	findings = sortFindings(findings)
	return redactFindings(text, findings), findings, nil
}

// stripCodeFences removes a single surrounding ```...``` fence pair if present.
func stripCodeFences(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	s = strings.TrimPrefix(s, "```")
	if idx := strings.Index(s, "\n"); idx >= 0 {
		// Drop the optional language tag line.
		s = s[idx+1:]
	}
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

// locateFindings converts model-emitted (class, text) pairs into byte-offset
// findings by locating each substring in the source. Substrings that do not
// appear verbatim are dropped — the model must return exact substrings per
// the prompt. Multiple occurrences of the same substring are all redacted.
func locateFindings(text string, emitted []slmFinding) []Finding {
	var out []Finding
	for _, f := range emitted {
		if f.Text == "" {
			continue
		}
		class := normalizeClass(f.Class)
		from := 0
		for {
			idx := strings.Index(text[from:], f.Text)
			if idx < 0 {
				break
			}
			start := from + idx
			end := start + len(f.Text)
			out = append(out, Finding{
				Class: class,
				Start: start,
				End:   end,
				Match: f.Text,
			})
			from = end
		}
	}
	return out
}

// normalizeClass maps a model-emitted class label to a PIIClass. Unknown
// labels collapse to ClassUnknown so the redaction token still fires.
func normalizeClass(s string) PIIClass {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "email":
		return ClassEmail
	case "phone":
		return ClassPhone
	case "ssn":
		return ClassSSN
	case "credit_card", "creditcard", "card":
		return ClassCreditCard
	case "api_key", "apikey", "key":
		return ClassAPIKey
	case "ip", "ipv4", "ip_address":
		return ClassIP
	case "name":
		return ClassName
	case "address":
		return ClassAddress
	case "credential", "password", "secret":
		return ClassCredential
	default:
		return ClassUnknown
	}
}
