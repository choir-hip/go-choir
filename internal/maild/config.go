// Package maild owns Choir's host-side email transport, mailbox state, and
// policy-labeled source packet ledger.
package maild

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
	DefaultCorpusdURL      = "http://127.0.0.1:8086"
)

// Send gate modes (MAILD_SEND_GATE_MODE;
// docs/archive/problems/maild-send-gate-defects-2026-10-14.md).
const (
	// SendGateEnforce: only the owner's session (a host caller) may approve
	// and send; approval replies and trusted senders need an aligned DMARC
	// or DKIM pass written by a trusted receiving server. The default, and
	// what an unknown value means (fail closed).
	SendGateEnforce = "enforce"
	// SendGateLegacy is the rollback: the pre-E1 rules for guest owner acts
	// (G1), reply authentication (G2) and the trusted-sender verdict (G3).
	SendGateLegacy = "legacy"
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
	// AttachmentMaxBytes caps a single uploaded attachment (default 15 MiB).
	AttachmentMaxBytes int64
	// DraftAttachmentMaxBytes caps total attachment bytes per draft (25 MiB).
	DraftAttachmentMaxBytes int64
	WebhookClockSkew        time.Duration
	// SendGateMode is SendGateEnforce or SendGateLegacy; empty means enforce.
	SendGateMode string
	// TrustedAuthservIDs are the authserv-ids (RFC 8601) of the receiving
	// server whose Authentication-Results maild believes, lowercased. Empty
	// means no inbound sender is ever "authenticated" (fail closed).
	TrustedAuthservIDs []string
	// CorpusdURL is where the daily digest reads the Autopaper front list
	// (MAILD_CORPUSD_URL; host loopback). The host-internal credential is sent
	// only to the default loopback service ports (hostauth.MarkInternal).
	CorpusdURL string
	// DigestOff is the digest kill switch (MAILD_DIGEST=off). The zero value
	// is on, but nobody is subscribed until the owner opts in.
	DigestOff bool
}

// LoadConfig resolves MAILD_* and RESEND_* environment variables.
func LoadConfig() (*Config, error) {
	storageRoot := envOr("MAILD_STORAGE_ROOT", DefaultLocalDir)
	cfg := &Config{
		Port:                    envOr("MAILD_PORT", DefaultPort),
		DBPath:                  envOr("MAILD_DB_PATH", filepath.Join(storageRoot, "mail.db")),
		StorageRoot:             storageRoot,
		PrimaryDomain:           envOr("MAILD_PRIMARY_DOMAIN", DefaultPrimaryDomain),
		RootOwnerID:             envOr("MAILD_ROOT_OWNER_ID", DefaultRootOwnerID),
		ResendAPIKey:            os.Getenv("RESEND_API_KEY"),
		ResendBaseURL:           envOr("RESEND_BASE_URL", DefaultResendBaseURL),
		VmctlURL:                os.Getenv("MAILD_VMCTL_URL"),
		WebhookSecret:           os.Getenv("RESEND_WEBHOOK_SECRET"),
		WebhookMaxBytes:         int64EnvOr("MAILD_WEBHOOK_MAX_BYTES", DefaultWebhookMaxBody),
		APIMaxBytes:             int64EnvOr("MAILD_API_MAX_BYTES", DefaultAPIMaxBody),
		ProviderMaxBytes:        int64EnvOr("MAILD_PROVIDER_MAX_BYTES", DefaultProviderMaxBody),
		AttachmentMaxBytes:      int64EnvOr("MAILD_ATTACHMENT_MAX_BYTES", DefaultAttachmentMaxBytes),
		DraftAttachmentMaxBytes: int64EnvOr("MAILD_DRAFT_ATTACHMENT_MAX_BYTES", DefaultDraftAttachmentMaxBytes),
		WebhookClockSkew:        5 * time.Minute,
		SendGateMode:            parseSendGateMode(os.Getenv("MAILD_SEND_GATE_MODE")),
		TrustedAuthservIDs:      parseAuthservIDs(os.Getenv("MAILD_TRUSTED_AUTHSERV_IDS")),
		CorpusdURL:              envOr("MAILD_CORPUSD_URL", DefaultCorpusdURL),
		DigestOff:               strings.EqualFold(strings.TrimSpace(os.Getenv("MAILD_DIGEST")), "off"),
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
		return fmt.Errorf("MAILD_VMCTL_URL is required (host autoputer fallback removed)")
	}
	return nil
}

func parseSendGateMode(raw string) string {
	if strings.EqualFold(strings.TrimSpace(raw), SendGateLegacy) {
		return SendGateLegacy
	}
	return SendGateEnforce
}

func parseAuthservIDs(raw string) []string {
	var out []string
	for _, id := range strings.Split(raw, ",") {
		if id = strings.ToLower(strings.TrimSpace(id)); id != "" {
			out = append(out, id)
		}
	}
	return out
}

// legacySendGate reports whether the rollback switch is set.
func (c *Config) legacySendGate() bool {
	return c != nil && c.SendGateMode == SendGateLegacy
}

// SendGateStatus describes the send gate for the startup log.
func (c *Config) SendGateStatus() string {
	mode := SendGateEnforce
	if c.legacySendGate() {
		mode = SendGateLegacy
	}
	note := ""
	if raw := strings.TrimSpace(os.Getenv("MAILD_SEND_GATE_MODE")); raw != "" &&
		!strings.EqualFold(raw, SendGateEnforce) && !strings.EqualFold(raw, SendGateLegacy) {
		note = fmt.Sprintf(" (unknown MAILD_SEND_GATE_MODE %q treated as enforce)", raw)
	}
	verdict := "trusted authserv-ids " + strings.Join(c.TrustedAuthservIDs, ",")
	if len(c.TrustedAuthservIDs) == 0 {
		verdict = "no trusted authserv-id (MAILD_TRUSTED_AUTHSERV_IDS unset): approval replies and trusted senders are never authenticated"
	}
	return fmt.Sprintf("send gate mode=%s%s; %s", mode, note, verdict)
}

// EnsureDirs creates maild's durable state directories.
func (c *Config) EnsureDirs() error {
	dirs := []string{
		filepath.Dir(c.DBPath),
		c.StorageRoot,
		filepath.Join(c.StorageRoot, "raw"),
		filepath.Join(c.StorageRoot, "attachments", "quarantine"),
		filepath.Join(c.StorageRoot, "attachments", "outbound"),
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
