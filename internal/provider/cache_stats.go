package provider

import (
	"os"
	"strings"
	"sync"
)

// Prompt-cache statistics, per provider, since the process started. The
// gateway's /health reports them so the cached share of each provider can be
// read on staging by sampling the counters (an hourly delta) without the
// guest's cost API.
type CacheStats struct {
	// Calls is every completed call; CacheReportedCalls the ones whose usage
	// carried a cache field at all.
	Calls              int64 `json:"calls"`
	CacheReportedCalls int64 `json:"cache_reported_calls"`
	// InputTokens counts only calls that reported a cache field, so a
	// provider that does not report one does not read as a 0% hit rate.
	InputTokens       int64 `json:"input_tokens"`
	CachedInputTokens int64 `json:"cached_input_tokens"`
}

// HitRate is cached over input tokens of the calls that reported a cache.
func (s CacheStats) HitRate() float64 {
	if s.InputTokens <= 0 {
		return 0
	}
	return float64(s.CachedInputTokens) / float64(s.InputTokens)
}

var cacheStats = struct {
	mu sync.Mutex
	by map[string]CacheStats
}{by: map[string]CacheStats{}}

// RecordCacheUsage adds one completed call to the per-provider counters.
func RecordCacheUsage(providerName string, u Usage) {
	cacheStats.mu.Lock()
	defer cacheStats.mu.Unlock()
	s := cacheStats.by[providerName]
	s.Calls++
	if u.CacheReported {
		s.CacheReportedCalls++
		s.InputTokens += int64(u.InputTokens)
		s.CachedInputTokens += int64(u.CachedInputTokens)
	}
	cacheStats.by[providerName] = s
}

// CacheStatsSnapshot copies the per-provider counters.
func CacheStatsSnapshot() map[string]CacheStats {
	cacheStats.mu.Lock()
	defer cacheStats.mu.Unlock()
	out := make(map[string]CacheStats, len(cacheStats.by))
	for k, v := range cacheStats.by {
		out[k] = v
	}
	return out
}

// ResetCacheStats clears the counters (tests).
func ResetCacheStats() {
	cacheStats.mu.Lock()
	defer cacheStats.mu.Unlock()
	cacheStats.by = map[string]CacheStats{}
}

// chatGPTSessionHeadersEnabled is the rollback flag for the session_id and
// conversation_id headers on the chatgpt route: on unless
// CHOIR_CHATGPT_SESSION_HEADERS is 0, off or false.
func chatGPTSessionHeadersEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("CHOIR_CHATGPT_SESSION_HEADERS"))) {
	case "0", "off", "false", "no":
		return false
	}
	return true
}
