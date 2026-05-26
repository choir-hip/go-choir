package searchplane

import (
	"context"
	"time"
)

// ProviderState is the operational state of a search provider.
type ProviderState string

const (
	StateActive      ProviderState = "active"
	StateCoolingDown ProviderState = "cooling_down"
	StateDisabled    ProviderState = "disabled"
)

// OutcomeClass classifies a single provider call for backoff policy.
type OutcomeClass string

const (
	OutcomeSuccess            OutcomeClass = "success"
	OutcomeSuccessEmpty       OutcomeClass = "success_empty"
	OutcomeRateLimited        OutcomeClass = "rate_limited"
	OutcomeQuotaLimited       OutcomeClass = "quota_limited"
	OutcomeAuthError          OutcomeClass = "auth_error"
	OutcomeServerError        OutcomeClass = "server_error"
	OutcomeTimeout            OutcomeClass = "timeout"
	OutcomeSkippedCoolingDown OutcomeClass = "skipped_cooling_down"
	OutcomeError              OutcomeClass = "error"
)

// Result is a normalized search hit from a provider adapter.
type Result struct {
	Title       string
	URL         string
	Snippet     string
	PublishedAt string
	Score       float64
	Provider    string
}

// Provider is a pluggable search backend.
type Provider interface {
	Name() string
	Search(ctx context.Context, query string, maxResults int) ([]Result, error)
}

// ProviderHealth is the durable health snapshot for one provider.
type ProviderHealth struct {
	Provider           string        `json:"provider"`
	State              ProviderState `json:"state"`
	CooldownUntil      *time.Time    `json:"cooldown_until,omitempty"`
	StrikeCount        int           `json:"strike_count"`
	WindowAttempts     int           `json:"window_attempts"`
	WindowSuccesses    int           `json:"window_successes"`
	WindowResultsTotal int           `json:"window_results_total"`
	LastFailureClass   string        `json:"last_failure_class,omitempty"`
	LastErrorSummary   string        `json:"last_error_summary,omitempty"`
	UpdatedAt          time.Time     `json:"updated_at"`
}

// Attempt records one provider interaction within a logical search.
type Attempt struct {
	Provider  string `json:"provider"`
	Endpoint  string `json:"endpoint,omitempty"`
	Status    string `json:"status"`
	LatencyMs int64  `json:"latency_ms"`
	Results   int    `json:"results"`
	Error     string `json:"error,omitempty"`
}

// Outcome is passed to the health store after each provider call or skip.
type Outcome struct {
	Provider string
	Class    OutcomeClass
	Results  int
	Error    string
	At       time.Time
}
