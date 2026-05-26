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
func ConfigFromEnv() Config {
	return Config{
		ProvidersPerQuery: envInt("CHOIR_SEARCH_PROVIDERS_PER_QUERY", 2),
		MinMergedResults:  envInt("CHOIR_SEARCH_MIN_MERGED_RESULTS", 5),
		MaxWaves:          envInt("CHOIR_SEARCH_MAX_WAVES", 2),
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
