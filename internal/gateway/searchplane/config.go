package searchplane

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config controls router fan-out and wave behavior.
type Config struct {
	ProvidersPerQuery int
	MinMergedResults  int
	MaxWaves          int
	RequestTimeout    time.Duration
	EndpointFor       func(provider string) string
}

// ConfigFromEnv loads router settings from environment variables.
//
// Defaults target agent retrieval breadth, not a human result page. The router
// keeps fanning out across providers until it has MinMergedResults deduped hits
// (capped by the request's max_results) or it exhausts waves/providers. With a
// small target it would stop after one provider returns a handful of hits; a
// ~40 target plus enough waves lets it accumulate a broad candidate set across
// the available providers for grounding and reranking. This costs more provider
// calls per search by design.
func ConfigFromEnv() Config {
	return Config{
		ProvidersPerQuery: envInt("CHOIR_SEARCH_PROVIDERS_PER_QUERY", 2),
		MinMergedResults:  envInt("CHOIR_SEARCH_MIN_MERGED_RESULTS", 40),
		MaxWaves:          envInt("CHOIR_SEARCH_MAX_WAVES", 4),
		RequestTimeout:    time.Duration(envInt("CHOIR_SEARCH_REQUEST_TIMEOUT_SECONDS", 30)) * time.Second,
	}
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
