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
}

// ConfigFromEnv loads router settings from environment variables.
func ConfigFromEnv() Config {
	return Config{
		RequestTimeout: time.Duration(envInt("CHOIR_SEARCH_REQUEST_TIMEOUT_SECONDS", 30)) * time.Second,
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
