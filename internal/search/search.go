package search

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/hostlink"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
)

// Client searches through the canonical runtime gateway.
type Client interface {
	// Stream starts a search and returns once the gateway has named the
	// providers it started; each provider's answer is read with Next.
	Stream(ctx context.Context, query string, maxResults int) (*Stream, error)
}

// Start is a search stream's first line: the providers the search started
// (the N of "M of N answered") and the ones it skipped.
type Start struct {
	Query     string    `json:"query"`
	Providers []string  `json:"providers"`
	Cooling   []Cooling `json:"cooling"`
}

// Cooling is a provider a search skipped because it was cooling down or
// disabled. It is not a failure of that search.
type Cooling struct {
	Provider string `json:"provider"`
	State    string `json:"state"`
	Until    string `json:"until,omitempty"`
}

// Arrival is one provider's completed call.
type Arrival struct {
	Provider  string   `json:"provider"`
	Status    string   `json:"status"`
	LatencyMs int64    `json:"latency_ms"`
	Results   []Result `json:"results"`
	Error     string   `json:"error,omitempty"`
}

// Answered reports whether the provider returned an answer, possibly empty.
func (a Arrival) Answered() bool {
	return a.Status == "success" || a.Status == "success_empty"
}

// Result is one search hit.
type Result struct {
	Title       string  `json:"title"`
	URL         string  `json:"url"`
	Snippet     string  `json:"snippet"`
	PublishedAt string  `json:"published_at,omitempty"`
	Score       float64 `json:"score,omitempty"`
	Provider    string  `json:"provider,omitempty"`
}

// Done is a search stream's final line. Pending names the providers still
// out when the gateway's bound ended the stream.
type Done struct {
	Answered int       `json:"answered"`
	Total    int       `json:"total"`
	Pending  []string  `json:"pending,omitempty"`
	Cooling  []Cooling `json:"cooling"`
}

// readLine decodes the next NDJSON line and returns its type with the raw
// line for the caller to decode into that type's shape.
func readLine(dec *json.Decoder) (string, json.RawMessage, error) {
	var raw json.RawMessage
	if err := dec.Decode(&raw); err != nil {
		return "", nil, err
	}
	var head struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &head); err != nil {
		return "", nil, err
	}
	return head.Type, raw, nil
}

// Stream reads one search's lines as the gateway writes them.
type Stream struct {
	Start Start
	body  io.ReadCloser
	dec   *json.Decoder
	done  *Done
}

// Next returns the next provider's arrival. After the final line it returns
// io.EOF and Done reports that line; a stream cut before its final line is
// an error.
func (s *Stream) Next() (Arrival, error) {
	if s.done != nil {
		return Arrival{}, io.EOF
	}
	kind, raw, err := readLine(s.dec)
	if err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return Arrival{}, fmt.Errorf("gateway search: stream ended before its final line")
		}
		return Arrival{}, fmt.Errorf("gateway search: read line: %w", err)
	}
	switch kind {
	case "provider":
		var arrival Arrival
		if err := json.Unmarshal(raw, &arrival); err != nil {
			return Arrival{}, fmt.Errorf("gateway search: decode provider line: %w", err)
		}
		return arrival, nil
	case "done":
		var done Done
		if err := json.Unmarshal(raw, &done); err != nil {
			return Arrival{}, fmt.Errorf("gateway search: decode final line: %w", err)
		}
		s.done = &done
		return Arrival{}, io.EOF
	default:
		return Arrival{}, fmt.Errorf("gateway search: unexpected line %q", kind)
	}
}

// Done returns the final line once Next has read it.
func (s *Stream) Done() (Done, bool) {
	if s.done == nil {
		return Done{}, false
	}
	return *s.done, true
}

// Close ends the stream; the gateway's provider calls for it are cancelled.
func (s *Stream) Close() error {
	return s.body.Close()
}

type gatewaySearchClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewGatewayClientFromEnv returns the configured gateway client, or nil when
// the gateway URL or token is unavailable.
func NewGatewayClientFromEnv() Client {
	baseURL := strings.TrimSpace(os.Getenv("RUNTIME_GATEWAY_URL"))
	if baseURL == "" {
		baseURL = strings.TrimSpace(os.Getenv("PROXY_VMCTL_URL"))
	}
	token := provideriface.GatewayToken()
	if baseURL == "" || token == "" {
		return nil
	}
	return &gatewaySearchClient{
		baseURL: baseURL,
		token:   token,
		// No per-attempt bound: the gateway keeps a stream open for the
		// provider timeout plus a margin and ends it itself, and a bound
		// here would cut the body it is still writing. The caller's context
		// ends a stream early; hostlink keeps resending a refused dial.
		httpClient: hostlink.NewClient(0),
	}
}

func (c *gatewaySearchClient) Stream(ctx context.Context, query string, maxResults int) (*Stream, error) {
	payload := map[string]any{
		"query": query,
	}
	if maxResults > 0 {
		payload["max_results"] = maxResults
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("gateway search: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/provider/v1/search", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("gateway search: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gateway search: http call: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		defer func() { _ = resp.Body.Close() }()
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		var errResp struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(body, &errResp); err == nil && strings.TrimSpace(errResp.Error) != "" {
			return nil, fmt.Errorf("gateway search: %s", errResp.Error)
		}
		return nil, fmt.Errorf("gateway search: status %s", resp.Status)
	}

	return ReadStream(resp.Body)
}

// ReadStream reads a search stream's start line from body and returns the
// stream; the caller reads the provider lines with Next and closes it. On
// error body is closed.
func ReadStream(body io.ReadCloser) (*Stream, error) {
	stream := &Stream{body: body, dec: json.NewDecoder(body)}
	kind, raw, err := readLine(stream.dec)
	switch {
	case err != nil:
		err = fmt.Errorf("read start line: %w", err)
	case kind != "start":
		err = fmt.Errorf("first line is %q, want start", kind)
	default:
		err = json.Unmarshal(raw, &stream.Start)
	}
	if err != nil {
		_ = body.Close()
		return nil, fmt.Errorf("gateway search: %w", err)
	}
	return stream, nil
}
