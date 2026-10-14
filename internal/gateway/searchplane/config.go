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
	// CacheTTL > 0 turns on the result cache and in-flight join (cache.go);
	// CacheFreshTTL is the TTL for time-sensitive queries, CacheMaxBytes
	// bounds the result text held, and CacheShared lets one computer's
	// search answer another's (default: per computer).
	CacheTTL      time.Duration
	CacheFreshTTL time.Duration
	CacheMaxBytes int64
	CacheShared   bool
}

// DefaultProviderRPS paces Brave to its Free AI plan's one request a second.
const DefaultProviderRPS = "brave=1"

// DefaultPaceWait is the default PaceWait.
const DefaultPaceWait = 3 * time.Second

// Result cache defaults (docs/reports/search-efficiency-and-reuse-2026-10-14.md
// §1): 177 of 300 measured repeats came within 10 minutes, 255 within an
// hour.
const (
	DefaultCacheTTL      = 30 * time.Minute
	DefaultCacheFreshTTL = 10 * time.Minute
	DefaultCacheMaxMB    = 64
)

// ConfigFromEnv loads router settings from environment variables.
// CHOIR_SEARCH_PROVIDER_RPS ("brave=1,tavily=5"; rate 0 disables) replaces
// DefaultProviderRPS when set. CHOIR_SEARCH_CACHE_TTL_SECONDS=0 turns the
// result cache and in-flight join off; CHOIR_SEARCH_CACHE_SCOPE=shared
// shares answers across computers.
func ConfigFromEnv() Config {
	rps := strings.TrimSpace(os.Getenv("CHOIR_SEARCH_PROVIDER_RPS"))
	if rps == "" {
		rps = DefaultProviderRPS
	}
	return Config{
		RequestTimeout: time.Duration(envInt("CHOIR_SEARCH_REQUEST_TIMEOUT_SECONDS", 30)) * time.Second,
		MinInterval:    parseProviderRPS(rps),
		PaceWait:       time.Duration(envInt("CHOIR_SEARCH_PACE_WAIT_MS", int(DefaultPaceWait/time.Millisecond))) * time.Millisecond,
		CacheTTL:       time.Duration(envInt("CHOIR_SEARCH_CACHE_TTL_SECONDS", int(DefaultCacheTTL/time.Second))) * time.Second,
		CacheFreshTTL:  time.Duration(envInt("CHOIR_SEARCH_CACHE_FRESH_TTL_SECONDS", int(DefaultCacheFreshTTL/time.Second))) * time.Second,
		CacheMaxBytes:  int64(envInt("CHOIR_SEARCH_CACHE_MAX_MB", DefaultCacheMaxMB)) << 20,
		CacheShared:    strings.EqualFold(strings.TrimSpace(os.Getenv("CHOIR_SEARCH_CACHE_SCOPE")), "shared"),
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
