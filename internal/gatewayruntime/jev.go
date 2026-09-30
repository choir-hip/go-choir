package gatewayruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const judgmentClientMaxBodySize = 1 << 20

// JudgmentRequest is the VM-to-gateway wire contract for a Jev decision. The
// model is intentionally not caller-selectable: the gateway pins Jev's model.
type JudgmentRequest struct {
	State     json.RawMessage            `json:"state"`
	Questions map[string]json.RawMessage `json:"questions"`
}

// JudgmentResponse preserves the complete OpenRouter response distribution.
// Callers must interpret it themselves; the transport never flattens it into a
// score or applies a confidence gate.
type JudgmentResponse struct {
	Distribution json.RawMessage
}

// JudgmentClient calls the Jev transport through the gateway using this VM's
// bootstrap bearer. It contains no OpenRouter credential or provider endpoint.
type JudgmentClient struct {
	gatewayURL string
	token      string
	httpClient *http.Client
}

func NewJudgmentClient(gatewayURL, token string) *JudgmentClient {
	return &JudgmentClient{
		gatewayURL: strings.TrimRight(strings.TrimSpace(gatewayURL), "/"),
		token:      strings.TrimSpace(token),
		httpClient: &http.Client{Timeout: gatewayClientTimeout},
	}
}

func (c *JudgmentClient) Decide(ctx context.Context, request JudgmentRequest) (*JudgmentResponse, error) {
	if c == nil || c.gatewayURL == "" {
		return nil, fmt.Errorf("judgment client: missing gateway URL")
	}
	if c.token == "" {
		return nil, fmt.Errorf("judgment client: missing autoputer credential")
	}
	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("judgment client: marshal request: %w", err)
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, c.gatewayURL+"/provider/v1/judgments", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("judgment client: create request: %w", err)
	}
	httpRequest.Header.Set("Authorization", "Bearer "+c.token)
	httpRequest.Header.Set("Content-Type", "application/json")
	httpResponse, err := c.httpClient.Do(httpRequest)
	if err != nil {
		return nil, fmt.Errorf("judgment client: http call: %w", err)
	}
	defer func() { _ = httpResponse.Body.Close() }()
	response, err := io.ReadAll(io.LimitReader(httpResponse.Body, judgmentClientMaxBodySize+1))
	if err != nil {
		return nil, fmt.Errorf("judgment client: read response: %w", err)
	}
	if len(response) > judgmentClientMaxBodySize {
		return nil, fmt.Errorf("judgment client: response exceeds transport limit")
	}
	if httpResponse.StatusCode != http.StatusOK {
		return nil, gatewayStatusError(httpResponse.Status, response)
	}
	if !json.Valid(response) {
		return nil, fmt.Errorf("judgment client: invalid JSON response")
	}
	return &JudgmentResponse{Distribution: json.RawMessage(response)}, nil
}
