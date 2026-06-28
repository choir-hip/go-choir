// Package gateway implements the host-side provider gateway for Mission 3.
//
// The gateway is the only component that holds real provider credentials and
// makes upstream LLM calls. Sandboxes authenticate to the gateway using
// per-sandbox credentials that the gateway issues and manages. Browser callers
// are denied at the proxy level.
//
// Key invariants:
//   - Provider credentials remain host-side (VAL-GATEWAY-004).
//   - Browser callers cannot use /provider/* as a raw inference bypass
//     (VAL-GATEWAY-002).
//   - Gateway denies unauthenticated or forged callers (VAL-GATEWAY-003).
//   - Upstream failures are sanitized before returning to callers
//     (VAL-GATEWAY-007).
//   - Stale sandbox credentials are invalidated after lifecycle changes
//     (VAL-GATEWAY-008).
package gateway

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Config holds gateway service configuration resolved from environment variables.
type Config struct {
	// Port is the gateway listen port.
	Port string

	// SandboxTokenTTL is how long issued sandbox credentials remain valid.
	SandboxTokenTTL time.Duration

	// IdentityStorePath persists sandbox credential hashes so gateway restarts
	// do not invalidate still-running desktop VMs.
	IdentityStorePath string

	// ServiceHealthURLs maps a dependency service name (e.g. "sourcecycled",
	// "runtime", "qdrant", "dolt", "ollama") to the URL the gateway should
	// probe for GET /health/{service}. Empty entries fall back to defaults
	// (see DefaultServiceHealthURLs). These are used by the per-service
	// health endpoint and the /health/ready aggregator so operators can
	// observe backend dependency health from outside the gateway
	// (M22b / C20).
	ServiceHealthURLs map[string]string
}

const (
	// DefaultGatewayPort is the default gateway service port.
	DefaultGatewayPort = "8084"

	// DefaultSandboxTokenTTL is the default TTL for sandbox credentials.
	DefaultSandboxTokenTTL = 1 * time.Hour
)

// DefaultServiceHealthURLs are the default probe URLs for the per-service
// health endpoint. Each points at a localhost-only backend service the
// gateway depends on (directly or transitively). "dolt" is probed through
// the sandbox runtime readiness endpoint because Dolt is an embedded store
// owned by the runtime, not a standalone TCP service. Operators can override
// any entry via GATEWAY_HEALTH_<SERVICE>_URL.
var DefaultServiceHealthURLs = map[string]string{
	"sourcecycled": "http://127.0.0.1:8787/health",
	"runtime":      "http://127.0.0.1:8085/health",
	"qdrant":       "http://127.0.0.1:6333/healthz",
	"dolt":         "http://127.0.0.1:8085/health/ready",
	"ollama":       "http://127.0.0.1:11434/api/tags",
}

// LoadConfig resolves gateway configuration from environment variables.
func LoadConfig() Config {
	port := envOr("GATEWAY_PORT", DefaultGatewayPort)

	ttl := DefaultSandboxTokenTTL
	if v := os.Getenv("GATEWAY_SANDBOX_TOKEN_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			ttl = d
		}
	}

	return Config{
		Port:              port,
		SandboxTokenTTL:   ttl,
		IdentityStorePath: os.Getenv("GATEWAY_IDENTITY_STORE_PATH"),
		ServiceHealthURLs: loadServiceHealthURLs(),
	}
}

// loadServiceHealthURLs resolves the per-service health probe URLs. It starts
// from DefaultServiceHealthURLs and applies GATEWAY_HEALTH_<SERVICE>_URL
// overrides (uppercased service name, hyphens replaced with underscores).
func loadServiceHealthURLs() map[string]string {
	out := make(map[string]string, len(DefaultServiceHealthURLs))
	for name, url := range DefaultServiceHealthURLs {
		envKey := "GATEWAY_HEALTH_" + strings.ToUpper(strings.ReplaceAll(name, "-", "_")) + "_URL"
		if v := strings.TrimSpace(os.Getenv(envKey)); v != "" {
			out[name] = v
		} else {
			out[name] = url
		}
	}
	return out
}

// LoadRateLimiterConfig resolves rate limiter configuration from
// environment variables and returns a resolved RateLimiterConfig
// with defaults applied.
func LoadRateLimiterConfig() RateLimiterConfig {
	cfg := RateLimiterConfig{}

	if v := os.Getenv("GATEWAY_RATE_LIMIT_MAX_REQUESTS"); v != "" {
		if n, err := fmt.Sscanf(v, "%d", &cfg.MaxRequests); err == nil && n == 1 {
			// parsed successfully - cfg.MaxRequests is now set
			_ = cfg.MaxRequests // silence staticcheck: empty branch
		}
	}

	if v := os.Getenv("GATEWAY_RATE_LIMIT_WINDOW"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.WindowSize = d
		}
	}

	return cfg.Resolve()
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// SandboxIdentity represents a registered sandbox caller with its
// authentication credential.
type SandboxIdentity struct {
	// SandboxID is the unique sandbox identifier.
	SandboxID string

	// TokenHash is the SHA-256 hash of the issued credential. The raw
	// credential is only returned once at issuance time and is never stored.
	TokenHash string

	// IssuedAt is when the credential was issued.
	IssuedAt time.Time

	// ExpiresAt is when the credential expires.
	ExpiresAt time.Time

	// Active indicates whether the credential is currently valid.
	// Revoked or replaced credentials have Active=false.
	Active bool
}

// IdentityRegistry manages sandbox identities and their credentials.
// It supports issuance, validation, revocation, and invalidation of
// stale credentials (VAL-GATEWAY-008).
type IdentityRegistry struct {
	mu              sync.Mutex
	identities      map[string]*SandboxIdentity // sandbox_id -> identity
	tokenTTL        time.Duration
	persistencePath string
}

// NewIdentityRegistry creates a new identity registry with the given
// credential TTL.
func NewIdentityRegistry(tokenTTL time.Duration) *IdentityRegistry {
	return &IdentityRegistry{
		identities: make(map[string]*SandboxIdentity),
		tokenTTL:   tokenTTL,
	}
}

// SetPersistencePath loads and enables file-backed identity persistence. The
// file stores token hashes only, never raw gateway credentials.
func (r *IdentityRegistry) SetPersistencePath(path string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.persistencePath = path
	if r.persistencePath == "" {
		return nil
	}
	return r.loadLocked()
}

func (r *IdentityRegistry) loadLocked() error {
	data, err := os.ReadFile(r.persistencePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read identity store: %w", err)
	}
	if len(data) == 0 {
		return nil
	}

	var identities map[string]*SandboxIdentity
	if err := json.Unmarshal(data, &identities); err != nil {
		return fmt.Errorf("decode identity store: %w", err)
	}
	if identities == nil {
		identities = make(map[string]*SandboxIdentity)
	}
	r.identities = identities
	return nil
}

func (r *IdentityRegistry) persistLocked() error {
	if r.persistencePath == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(r.persistencePath), 0o700); err != nil {
		return fmt.Errorf("create identity store dir: %w", err)
	}
	data, err := json.MarshalIndent(r.identities, "", "  ")
	if err != nil {
		return fmt.Errorf("encode identity store: %w", err)
	}
	tmp := r.persistencePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write identity store temp: %w", err)
	}
	if err := os.Rename(tmp, r.persistencePath); err != nil {
		return fmt.Errorf("replace identity store: %w", err)
	}
	return nil
}

// CredentialResult is returned when a new credential is issued.
// The RawToken is shown once and must be communicated to the sandbox
// out-of-band (e.g., via VM bootstrap material).
type CredentialResult struct {
	SandboxID string
	RawToken  string
	ExpiresAt time.Time
}

// CredentialEnsureResult is returned when an existing VM bootstrap token has
// been reconciled into the gateway's durable identity store.
type CredentialEnsureResult struct {
	SandboxID string    `json:"sandbox_id"`
	Status    string    `json:"status"`
	ExpiresAt time.Time `json:"expires_at"`
}

// IssueCredential creates a new credential for the given sandbox ID.
// If an existing credential exists, it is revoked and replaced.
// Returns the raw token (shown once) and an error if generation fails.
func (r *IdentityRegistry) IssueCredential(sandboxID string) (*CredentialResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	token, err := generateSecureToken(32)
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	now := time.Now()
	hash := sha256.Sum256([]byte(token))

	// Revoke any existing credential for this sandbox.
	if existing, ok := r.identities[sandboxID]; ok {
		existing.Active = false
	}

	identity := &SandboxIdentity{
		SandboxID: sandboxID,
		TokenHash: hex.EncodeToString(hash[:]),
		IssuedAt:  now,
		ExpiresAt: now.Add(r.tokenTTL),
		Active:    true,
	}
	r.identities[sandboxID] = identity
	if err := r.persistLocked(); err != nil {
		return nil, err
	}

	return &CredentialResult{
		SandboxID: sandboxID,
		RawToken:  sandboxID + ":" + token,
		ExpiresAt: identity.ExpiresAt,
	}, nil
}

// EnsureCredential imports an already-issued raw sandbox credential into the
// registry if the sandbox identity is currently unknown. This is intentionally
// narrower than IssueCredential: it never returns the raw token, refuses to
// overwrite a different known credential, and refuses to reactivate revoked or
// expired credentials. It exists to repair VMs that were booted before gateway
// identity persistence existed, where the host still has the VM bootstrap token
// but the restarted gateway has lost the token hash.
func (r *IdentityRegistry) EnsureCredential(rawToken string) (*CredentialEnsureResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	sandboxID, token, ok := splitCredential(rawToken)
	if !ok || strings.TrimSpace(sandboxID) == "" || strings.TrimSpace(token) == "" {
		return nil, fmt.Errorf("invalid credential format")
	}
	hash := sha256.Sum256([]byte(token))
	hashText := hex.EncodeToString(hash[:])
	now := time.Now()

	if existing, ok := r.identities[sandboxID]; ok {
		if existing.TokenHash != hashText {
			return nil, fmt.Errorf("credential conflict")
		}
		if !existing.Active {
			return nil, fmt.Errorf("credential revoked")
		}
		if now.After(existing.ExpiresAt) {
			return nil, fmt.Errorf("credential expired")
		}
		return &CredentialEnsureResult{
			SandboxID: sandboxID,
			Status:    "already_active",
			ExpiresAt: existing.ExpiresAt,
		}, nil
	}

	identity := &SandboxIdentity{
		SandboxID: sandboxID,
		TokenHash: hashText,
		IssuedAt:  now,
		ExpiresAt: now.Add(r.tokenTTL),
		Active:    true,
	}
	r.identities[sandboxID] = identity
	if err := r.persistLocked(); err != nil {
		return nil, err
	}
	return &CredentialEnsureResult{
		SandboxID: sandboxID,
		Status:    "imported",
		ExpiresAt: identity.ExpiresAt,
	}, nil
}

// ValidateCredential checks whether a sandbox credential is valid.
// Returns the sandbox ID if valid, or an error explaining why not.
func (r *IdentityRegistry) ValidateCredential(rawToken string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	sandboxID, token, ok := splitCredential(rawToken)
	if !ok {
		return "", fmt.Errorf("invalid credential format")
	}

	identity, ok := r.identities[sandboxID]
	if !ok {
		return "", fmt.Errorf("unknown sandbox identity")
	}

	if !identity.Active {
		return "", fmt.Errorf("credential revoked")
	}

	if time.Now().After(identity.ExpiresAt) {
		return "", fmt.Errorf("credential expired")
	}

	// Verify the token hash.
	hash := sha256.Sum256([]byte(token))
	if hex.EncodeToString(hash[:]) != identity.TokenHash {
		return "", fmt.Errorf("invalid credential")
	}

	return sandboxID, nil
}

// RevokeCredential revokes the credential for the given sandbox ID.
// After revocation, the old credential no longer authorizes provider
// requests (VAL-GATEWAY-008).
func (r *IdentityRegistry) RevokeCredential(sandboxID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if identity, ok := r.identities[sandboxID]; ok {
		identity.Active = false
		_ = r.persistLocked()
	}
}

// RotateCredential revokes the existing credential and issues a new one.
// This is used when sandbox credentials are rotated for security or
// after lifecycle changes.
func (r *IdentityRegistry) RotateCredential(sandboxID string) (*CredentialResult, error) {
	return r.IssueCredential(sandboxID)
}

// ActiveCount reports currently active, unexpired identities.
func (r *IdentityRegistry) ActiveCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	count := 0
	for _, id := range r.identities {
		if id.Active && now.Before(id.ExpiresAt) {
			count++
		}
	}
	return count
}

// generateSecureToken generates a cryptographically secure random token
// of the given byte length, hex-encoded.
func generateSecureToken(byteLen int) (string, error) {
	b := make([]byte, byteLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// splitCredential splits a "sandboxID:token" credential into its parts.
func splitCredential(raw string) (sandboxID, token string, ok bool) {
	for i := 0; i < len(raw); i++ {
		if raw[i] == ':' {
			return raw[:i], raw[i+1:], true
		}
	}
	return "", "", false
}
