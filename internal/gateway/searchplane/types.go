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
	// StatePaced is never stored: a search skips a healthy provider whose
	// next call slot (Config.MinInterval) is further away than PaceWait.
	StatePaced ProviderState = "paced"
	// StateCapped is never stored: the provider reached its daily call cap
	// (Config.DailyCaps) and is skipped until UTC midnight. Not a failure.
	StateCapped ProviderState = "capped"
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

	// OutcomeCallerGone: the caller cancelled, or its own deadline passed,
	// before the provider answered. Not the provider's failure; never a strike.
	OutcomeCallerGone OutcomeClass = "caller_gone"
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

// Outcome is passed to the health store after each provider call or skip.
type Outcome struct {
	Provider string
	Class    OutcomeClass
	Results  int
	Error    string
	// At is when the outcome was known (the call's completion): every
	// cooldown that follows a failure is anchored here, so a 30 s timeout
	// does not eat its own bench.
	At time.Time
	// StartedAt is when the call began (zero: At). A failure from a call
	// that began before the bench in force was set is the same episode
	// (sameEpisode).
	StartedAt time.Time
	// Uncapped marks a provider with nothing behind it: free tier, no quota
	// that a failure could mean is gone. Its transient failures (error,
	// server_error, timeout) are not a cooldown until
	// TransientStrikesBeforeCooldown in a row; its real refusals (rate
	// limit, quota, auth) back it off as for any provider.
	Uncapped bool
}
