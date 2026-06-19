package platform

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	DefaultPort          = "8086"
	DefaultDoltDSN       = "root@tcp(127.0.0.1:13306)/platform?parseTime=true&multiStatements=true&clientFoundRows=true"
	DefaultArtifactsRoot = "/var/lib/go-choir/platform-artifacts"
)

type Config struct {
	Port            string
	DoltDSN         string
	ArtifactsRoot   string
	SigningKeyPath  string
}

func LoadConfig() (*Config, error) {
	cfg := &Config{
		Port:           envOr("PLATFORMD_PORT", DefaultPort),
		DoltDSN:        envOr("PLATFORMD_DOLT_DSN", DefaultDoltDSN),
		ArtifactsRoot:  envOr("PLATFORMD_ARTIFACTS_ROOT", DefaultArtifactsRoot),
		SigningKeyPath: envOr("PLATFORM_SIGNING_KEY_PATH", filepath.Join(envOr("PLATFORMD_ARTIFACTS_ROOT", DefaultArtifactsRoot), "signing-key")),
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	if c.Port == "" {
		return fmt.Errorf("platform config: PLATFORMD_PORT must not be empty")
	}
	if c.DoltDSN == "" {
		return fmt.Errorf("platform config: PLATFORMD_DOLT_DSN must not be empty")
	}
	if c.ArtifactsRoot == "" {
		return fmt.Errorf("platform config: PLATFORMD_ARTIFACTS_ROOT must not be empty")
	}
	return nil
}

func (c *Config) EnsureDirs() error {
	// Platform artifacts are content-addressed below this root.
	if err := os.MkdirAll(filepath.Join(c.ArtifactsRoot, "sha256"), 0o750); err != nil {
		return fmt.Errorf("platform config: create artifacts root: %w", err)
	}
	return nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
