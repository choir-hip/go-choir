// Package autoputer provides the autoputer service that hosts the placeholder
// shell handlers and the runtime engine for Mission 3.
//
// The autoputer service runs as a host process on port 8085 (during the
// host-process milestone) and provides both the legacy placeholder endpoints
// and the real runtime API endpoints for task submission, status lookup,
// and event streaming.
package autoputer

import (
	"os"
)

// Config holds the autoputer service configuration resolved from environment
// variables.
type Config struct {
	// Port is the listen port for the autoputer HTTP server.
	Port string

	// ComputerID is the stable identity string returned in bootstrap and
	// validation responses. It proves which autoputer instance handled a request.
	ComputerID string

	// StorePath is the marker path used to derive the embedded Dolt workspace.
	// If empty, the runtime package default is used.
	StorePath string
}

// LoadConfig resolves autoputer configuration from environment variables.
func LoadConfig() Config {
	port := portFromEnv("AUTOPUTER_PORT", "8085")
	computerID := fromEnv("AUTOPUTER_ID", "autoputer-dev")
	storePath := fromEnv("RUNTIME_STORE_PATH", "")
	return Config{
		Port:       port,
		ComputerID: computerID,
		StorePath:  storePath,
	}
}

func portFromEnv(envVar, defaultPort string) string {
	if v := os.Getenv(envVar); v != "" {
		return v
	}
	return defaultPort
}

func fromEnv(envVar, defaultVal string) string {
	if v := os.Getenv(envVar); v != "" {
		return v
	}
	return defaultVal
}
