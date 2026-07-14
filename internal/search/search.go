package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Client searches through the canonical runtime gateway.
type Client interface {
	Search(ctx context.Context, query string, maxResults int) (*Response, error)
}

// Response is the gateway's merged web-search response.
type Response struct {
	Query          string           `json:"query"`
	Provider       string           `json:"provider"`
	Providers      []string         `json:"providers,omitempty"`
	Attempts       []map[string]any `json:"attempts,omitempty"`
	Results        []map[string]any `json:"results"`
	MergedCount    int              `json:"merged_count,omitempty"`
	Waves          int              `json:"waves,omitempty"`
	Degraded       bool             `json:"degraded,omitempty"`
	ProviderHealth map[string]any   `json:"provider_health,omitempty"`
	Outage         bool             `json:"outage,omitempty"`
	Code           string           `json:"code,omitempty"`
	Error          string           `json:"error,omitempty"`
}

type gatewaySearchClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

type gatewaySearchAttempt struct {
	Provider  string `json:"provider"`
	Endpoint  string `json:"endpoint,omitempty"`
	Status    string `json:"status"`
	LatencyMs int64  `json:"latency_ms"`
	Results   int    `json:"results"`
	Error     string `json:"error,omitempty"`
}

type gatewaySearchOutageBody struct {
	Error          string                           `json:"error"`
	Code           string                           `json:"code"`
	Query          string                           `json:"query,omitempty"`
	ProviderHealth map[string]gatewayProviderHealth `json:"provider_health,omitempty"`
	Attempts       []gatewaySearchAttempt           `json:"attempts,omitempty"`
}

type gatewayProviderHealth struct {
	State            string `json:"state"`
	CooldownUntil    string `json:"cooldown_until,omitempty"`
	StrikeCount      int    `json:"strike_count"`
	LastFailureClass string `json:"last_failure_class,omitempty"`
	LastErrorSummary string `json:"last_error_summary,omitempty"`
}

// NewGatewayClientFromEnv returns the configured gateway client, or nil when
// the gateway URL or token is unavailable.
func NewGatewayClientFromEnv() Client {
	baseURL := strings.TrimSpace(os.Getenv("RUNTIME_GATEWAY_URL"))
	if baseURL == "" {
		baseURL = strings.TrimSpace(os.Getenv("PROXY_VMCTL_URL"))
	}
	token := strings.TrimSpace(os.Getenv("RUNTIME_GATEWAY_TOKEN"))
	if baseURL == "" || token == "" {
		return nil
	}
	return &gatewaySearchClient{
		baseURL:    baseURL,
		token:      token,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *gatewaySearchClient) Search(ctx context.Context, query string, maxResults int) (*Response, error) {
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
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gateway search: read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		if outage, ok := parseGatewaySearchOutage(body, query); ok {
			return outage, nil
		}
		var errResp struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(body, &errResp); err == nil && strings.TrimSpace(errResp.Error) != "" {
			return nil, fmt.Errorf("gateway search: %s", errResp.Error)
		}
		return nil, fmt.Errorf("gateway search: status %s", resp.Status)
	}

	var result Response
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("gateway search: decode response: %w", err)
	}
	return &result, nil
}

func parseGatewaySearchOutage(body []byte, fallbackQuery string) (*Response, bool) {
	var outage gatewaySearchOutageBody
	if err := json.Unmarshal(body, &outage); err != nil {
		return nil, false
	}
	if strings.TrimSpace(outage.Code) != "search_outage" && strings.TrimSpace(outage.Error) != "search_outage" {
		return nil, false
	}
	query := strings.TrimSpace(outage.Query)
	if query == "" {
		query = strings.TrimSpace(fallbackQuery)
	}
	attempts := make([]map[string]any, 0, len(outage.Attempts))
	for _, attempt := range outage.Attempts {
		entry := map[string]any{
			"provider":   attempt.Provider,
			"status":     attempt.Status,
			"latency_ms": attempt.LatencyMs,
			"results":    attempt.Results,
		}
		if attempt.Endpoint != "" {
			entry["endpoint"] = attempt.Endpoint
		}
		if attempt.Error != "" {
			entry["error"] = attempt.Error
		}
		attempts = append(attempts, entry)
	}
	providerHealth := make(map[string]any, len(outage.ProviderHealth))
	for name, health := range outage.ProviderHealth {
		entry := map[string]any{
			"state":        health.State,
			"strike_count": health.StrikeCount,
		}
		if health.CooldownUntil != "" {
			entry["cooldown_until"] = health.CooldownUntil
		}
		if health.LastFailureClass != "" {
			entry["last_failure_class"] = health.LastFailureClass
		}
		if health.LastErrorSummary != "" {
			entry["last_error_summary"] = health.LastErrorSummary
		}
		providerHealth[name] = entry
	}
	return &Response{
		Query:          query,
		Results:        []map[string]any{},
		Attempts:       attempts,
		ProviderHealth: providerHealth,
		Outage:         true,
		Code:           firstNonEmptyString(outage.Code, "search_outage"),
		Error:          firstNonEmptyString(outage.Error, "search_outage"),
		Degraded:       true,
	}, true
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
