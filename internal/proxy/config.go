// Package proxy provides the Mission 2 proxy service: validates auth-issued
// access JWTs, gates protected routes, and forwards authenticated traffic to
// the hardcoded placeholder sandbox without rewriting the public request path,
// method, query, or upstream status/body unexpectedly.
package proxy

import (
	"crypto/ed25519"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/ssh"
)

// Config holds all runtime configuration for the proxy service, resolved from
// PROXY_* environment variables.
type Config struct {
	// Port is the TCP port the proxy service listens on.
	Port string

	// SandboxURL is the base URL of the sandbox upstream. It is used to
	// initialize the reverse proxy. When vmctl routing is enabled, per-user
	// URLs resolved through vmctl override this for actual request routing.
	SandboxURL string

	// AuthPublicKeyPath is the path to the Ed25519 public key used to verify
	// auth-issued access JWTs.
	AuthPublicKeyPath string

	// VmctlURL is the base URL of the vmctl service. When set, the proxy
	// resolves user VM ownership through vmctl instead of using the static
	// SandboxURL (VAL-VM-001, VAL-VM-002).
	VmctlURL string

	// VmctlTimeout is the proxy -> vmctl request timeout. It must exceed the
	// vmmanager boot-ready timeout so cold computer boots do not fail halfway
	// through normal readiness probing.
	VmctlTimeout time.Duration

	// CorpusdURL is the internal platform service URL. The proxy uses it
	// for controlled private-computer to platform-publication transitions.
	CorpusdURL string

	// MaildURL is the internal mail service URL. The proxy uses it for
	// authenticated mailbox APIs and proxy-owned mail source handoff.
	MaildURL string

	// AuthDBPath is the filesystem path to the auth SQLite database. When
	// set, the proxy opens the auth store and can validate API keys (Bearer
	// tokens) as a fallback to cookie-based JWT auth. When empty, API key
	// auth is disabled and only cookie auth is used.
	AuthDBPath string

	// RuntimeDBPath is the filesystem path to the runtime Dolt workspace.
	// When set, the proxy opens a read-only lineage reader and can resolve
	// the platform route through ComputerSourceLineage (route-over-
	// ComputerVersion). When empty, the proxy uses the hard-coded platform
	// constants (H031 fallback).
	RuntimeDBPath string
}

const (
	// DefaultProxyPort is the default proxy service port.
	DefaultProxyPort = "8082"

	// DefaultSandboxURL is the default placeholder sandbox URL.
	DefaultSandboxURL = "http://127.0.0.1:8085"

	// defaultLocalDir is the base directory for local worker defaults when
	// explicit path env vars are omitted.
	defaultLocalDir = "/tmp/go-choir-m2"

	// DefaultAuthPublicKeyPath is the default path to the auth public key.
	DefaultAuthPublicKeyPath = defaultLocalDir + "/auth-signing-key.pub"

	// DefaultVmctlTimeout keeps proxy resolve calls aligned with the staging
	// VM_BOOT_READY_TIMEOUT=150s deployment setting.
	DefaultVmctlTimeout = 180 * time.Second

	// DefaultCorpusdURL is the localhost-only platform service endpoint.
	DefaultCorpusdURL = "http://127.0.0.1:8086"

	// DefaultMaildURL is the localhost-only mail service endpoint.
	DefaultMaildURL = "http://127.0.0.1:8087"
)

// LoadConfig resolves a Config from PROXY_* environment variables.
// When explicit path env vars are omitted, local worker defaults resolve
// under /tmp/go-choir-m2.
func LoadConfig() (*Config, error) {
	cfg := &Config{
		Port:              envOr("PROXY_PORT", DefaultProxyPort),
		SandboxURL:        envOr("PROXY_SANDBOX_URL", DefaultSandboxURL),
		AuthPublicKeyPath: defaultAuthPublicKeyPath(),
		VmctlURL:          os.Getenv("PROXY_VMCTL_URL"),
		VmctlTimeout:      durationEnvOr("PROXY_VMCTL_TIMEOUT", DefaultVmctlTimeout),
		CorpusdURL:      envOr("PROXY_CORPUSD_URL", DefaultCorpusdURL),
		MaildURL:          envOr("PROXY_MAILD_URL", DefaultMaildURL),
		AuthDBPath:        os.Getenv("PROXY_AUTH_DB_PATH"),
		RuntimeDBPath:     os.Getenv("PROXY_RUNTIME_DB_PATH"),
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// validate checks that the config is consistent and usable.
func (c *Config) validate() error {
	if c.Port == "" {
		return fmt.Errorf("proxy config: PROXY_PORT must not be empty")
	}
	if c.SandboxURL == "" {
		return fmt.Errorf("proxy config: PROXY_SANDBOX_URL must not be empty")
	}
	if c.AuthPublicKeyPath == "" {
		return fmt.Errorf("proxy config: PROXY_AUTH_PUBLIC_KEY_PATH must not be empty")
	}
	if c.VmctlTimeout <= 0 {
		return fmt.Errorf("proxy config: PROXY_VMCTL_TIMEOUT must be positive")
	}
	if c.CorpusdURL == "" {
		return fmt.Errorf("proxy config: PROXY_CORPUSD_URL must not be empty")
	}
	if c.MaildURL == "" {
		return fmt.Errorf("proxy config: PROXY_MAILD_URL must not be empty")
	}
	return nil
}

// defaultAuthPublicKeyPath resolves the proxy verifier key path. Explicit
// PROXY_AUTH_PUBLIC_KEY_PATH wins. When only AUTH_JWT_PRIVATE_KEY_PATH is set,
// derive the sibling public key path so local auth/proxy restarts stay aligned.
func defaultAuthPublicKeyPath() string {
	if v := os.Getenv("PROXY_AUTH_PUBLIC_KEY_PATH"); v != "" {
		return v
	}
	if authKeyPath := os.Getenv("AUTH_JWT_PRIVATE_KEY_PATH"); authKeyPath != "" {
		return authKeyPath + ".pub"
	}
	return DefaultAuthPublicKeyPath
}

// EnsureDirs creates the parent directories for file paths in the config.
func (c *Config) EnsureDirs() error {
	if c.AuthPublicKeyPath != "" {
		if err := os.MkdirAll(filepath.Dir(c.AuthPublicKeyPath), 0o755); err != nil {
			return fmt.Errorf("proxy config: cannot create public key directory %s: %w", filepath.Dir(c.AuthPublicKeyPath), err)
		}
	}
	return nil
}

// VmctlRoutingEnabled returns true when vmctl-backed routing is configured.
// When true, protected routes resolve through vmctl ownership rather than
// falling back to the static host sandbox URL (VAL-VM-002).
func (c *Config) VmctlRoutingEnabled() bool {
	return c.VmctlURL != ""
}

// LoadAuthPublicKey loads the Ed25519 public key from the configured path.
// The public key file is expected to be in OpenSSH format (as written by
// ssh-keygen).
func (c *Config) LoadAuthPublicKey() (ed25519.PublicKey, error) {
	return LoadPublicKey(c.AuthPublicKeyPath)
}

// LoadPublicKey loads an Ed25519 public key from an OpenSSH public key file.
// The file should contain a single public key line in the format written by
// ssh-keygen (e.g. "ssh-ed25519 AAAA... user@host").
func LoadPublicKey(path string) (ed25519.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read public key %s: %w", path, err)
	}

	// Parse the OpenSSH public key line.
	pub, _, _, _, err := ssh.ParseAuthorizedKey(data)
	if err != nil {
		return nil, fmt.Errorf("parse authorized key %s: %w", path, err)
	}

	// Extract the Ed25519 public key from the SSH CryptoPublicKey interface.
	cryptoPub, ok := pub.(ssh.CryptoPublicKey)
	if !ok {
		return nil, fmt.Errorf("public key is not a CryptoPublicKey, got %T", pub)
	}

	rawPub := cryptoPub.CryptoPublicKey()
	edKey, ok := rawPub.(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("public key is not Ed25519, got %T", rawPub)
	}

	return edKey, nil
}

// envOr returns the value of the environment variable named key, or fallback
// if the variable is unset or empty.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func durationEnvOr(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return fallback
}
