// Package maild owns Choir's host-side email transport, mailbox state, and
// policy-labeled source packet ledger.
package maild

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

const (
	DefaultPort            = "8087"
	DefaultLocalDir        = "/tmp/go-choir-maild"
	DefaultPrimaryDomain   = "choir.news"
	DefaultRootOwnerID     = "root"
	DefaultResendBaseURL   = "https://api.resend.com"
	DefaultWebhookMaxBody  = 1 << 20
	DefaultAPIMaxBody      = 1 << 20
	DefaultProviderMaxBody = 4 << 20
)

// Config holds maild runtime configuration.
type Config struct {
	Port             string
	DBPath           string
	StorageRoot      string
	PrimaryDomain    string
	RootOwnerID      string
	ResendAPIKey     string
	ResendBaseURL    string
	VmctlURL         string
	WebhookSecret    string
	WebhookMaxBytes  int64
	APIMaxBytes      int64
	ProviderMaxBytes int64
	WebhookClockSkew time.Duration
}

// LoadConfig resolves MAILD_* and RESEND_* environment variables.
func LoadConfig() (*Config, error) {
	storageRoot := envOr("MAILD_STORAGE_ROOT", DefaultLocalDir)
	cfg := &Config{
		Port:             envOr("MAILD_PORT", DefaultPort),
		DBPath:           envOr("MAILD_DB_PATH", filepath.Join(storageRoot, "mail.db")),
		StorageRoot:      storageRoot,
		PrimaryDomain:    envOr("MAILD_PRIMARY_DOMAIN", DefaultPrimaryDomain),
		RootOwnerID:      envOr("MAILD_ROOT_OWNER_ID", DefaultRootOwnerID),
		ResendAPIKey:     os.Getenv("RESEND_API_KEY"),
		ResendBaseURL:    envOr("RESEND_BASE_URL", DefaultResendBaseURL),
		VmctlURL:         os.Getenv("MAILD_VMCTL_URL"),
		WebhookSecret:    os.Getenv("RESEND_WEBHOOK_SECRET"),
		WebhookMaxBytes:  int64EnvOr("MAILD_WEBHOOK_MAX_BYTES", DefaultWebhookMaxBody),
		APIMaxBytes:      int64EnvOr("MAILD_API_MAX_BYTES", DefaultAPIMaxBody),
		ProviderMaxBytes: int64EnvOr("MAILD_PROVIDER_MAX_BYTES", DefaultProviderMaxBody),
		WebhookClockSkew: 5 * time.Minute,
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	if c.Port == "" {
		return fmt.Errorf("MAILD_PORT must not be empty")
	}
	if c.DBPath == "" {
		return fmt.Errorf("MAILD_DB_PATH must not be empty")
	}
	if c.StorageRoot == "" {
		return fmt.Errorf("MAILD_STORAGE_ROOT must not be empty")
	}
	if c.PrimaryDomain == "" {
		return fmt.Errorf("MAILD_PRIMARY_DOMAIN must not be empty")
	}
	if c.RootOwnerID == "" {
		return fmt.Errorf("MAILD_ROOT_OWNER_ID must not be empty")
	}
	if c.ResendBaseURL == "" {
		return fmt.Errorf("RESEND_BASE_URL must not be empty")
	}
	if c.WebhookMaxBytes <= 0 {
		return fmt.Errorf("MAILD_WEBHOOK_MAX_BYTES must be positive")
	}
	if c.APIMaxBytes <= 0 {
		return fmt.Errorf("MAILD_API_MAX_BYTES must be positive")
	}
	if c.ProviderMaxBytes <= 0 {
		return fmt.Errorf("MAILD_PROVIDER_MAX_BYTES must be positive")
	}
	if c.VmctlURL == "" {
		return fmt.Errorf("MAILD_VMCTL_URL is required (host sandbox fallback removed)")
	}
	return nil
}

// EnsureDirs creates maild's durable state directories.
func (c *Config) EnsureDirs() error {
	dirs := []string{
		filepath.Dir(c.DBPath),
		c.StorageRoot,
		filepath.Join(c.StorageRoot, "raw"),
		filepath.Join(c.StorageRoot, "attachments", "quarantine"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
	}
	return nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func int64EnvOr(key string, fallback int64) int64 {
	if v := os.Getenv(key); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err == nil && n > 0 {
			return n
		}
	}
	return fallback
}
