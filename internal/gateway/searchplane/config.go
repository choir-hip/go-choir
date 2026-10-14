package searchplane

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config controls the provider calls of one search.
type Config struct {
	// RequestTimeout bounds each provider call.
	RequestTimeout time.Duration
	EndpointFor    func(provider string) string
	// MinInterval paces calls to a provider: at most one call per interval,
	// across every computer. A per-second plan limit is a fact about our
	// traffic, not the provider's health, so it is met by pacing, not by a
	// bench (docs/problems/search-providers-cooling-most-of-the-time-2026-10-14.md).
	MinInterval map[string]time.Duration
	// PaceWait is how long a call may wait for its slot; a provider whose
	// slot is further away is skipped for that search as paced.
	PaceWait time.Duration
}

// DefaultProviderRPS paces Brave to its Free AI plan's one request a second.
const DefaultProviderRPS = "brave=1"

// DefaultPaceWait is the default PaceWait.
const DefaultPaceWait = 3 * time.Second

// ConfigFromEnv loads router settings from environment variables.
// CHOIR_SEARCH_PROVIDER_RPS ("brave=1,tavily=5"; rate 0 disables) replaces
// DefaultProviderRPS when set.
func ConfigFromEnv() Config {
	rps := strings.TrimSpace(os.Getenv("CHOIR_SEARCH_PROVIDER_RPS"))
	if rps == "" {
		rps = DefaultProviderRPS
	}
	return Config{
		RequestTimeout: time.Duration(envInt("CHOIR_SEARCH_REQUEST_TIMEOUT_SECONDS", 30)) * time.Second,
		MinInterval:    parseProviderRPS(rps),
		PaceWait:       time.Duration(envInt("CHOIR_SEARCH_PACE_WAIT_MS", int(DefaultPaceWait/time.Millisecond))) * time.Millisecond,
	}
}

// parseProviderRPS reads "name=rate,..." into per-provider call intervals.
// Malformed entries and rates <= 0 are left out (no pacing).
func parseProviderRPS(raw string) map[string]time.Duration {
	out := map[string]time.Duration{}
	for _, part := range strings.Split(raw, ",") {
		name, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		rate, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil || rate <= 0 {
			continue
		}
		out[strings.TrimSpace(name)] = time.Duration(float64(time.Second) / rate)
	}
	return out
}

func envInt(name string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return v
}
