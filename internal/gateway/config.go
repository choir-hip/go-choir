// Package gateway implements the host-side provider gateway for Mission 3.
//
// The gateway is the only component that holds real provider credentials and
// makes upstream LLM calls. Autoputeres authenticate to the gateway using
// per-autoputer credentials that the gateway issues and manages. Browser callers
// are denied at the proxy level.
//
// Key invariants:
//   - Provider credentials remain host-side (VAL-GATEWAY-004).
//   - Browser callers cannot use /provider/* as a raw inference bypass
//     (VAL-GATEWAY-002).
//   - Gateway denies unauthenticated or forged callers (VAL-GATEWAY-003).
//   - Upstream failures are sanitized before returning to callers
//     (VAL-GATEWAY-007).
//   - Stale autoputer credentials are invalidated after lifecycle changes
//     (VAL-GATEWAY-008).
package gateway

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
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

	// AutoputerTokenTTL is how long issued autoputer credentials remain valid.
	AutoputerTokenTTL time.Duration

	// IdentityStorePath persists autoputer credential hashes so gateway restarts
	// do not invalidate still-running desktop VMs.
	IdentityStorePath string

	// JevJudgmentsEnabled gates the experimental OpenRouter Decisions route.
	// Set GATEWAY_JEV_JUDGMENTS_ENABLED=0 to stop new Jev calls before
	// retiring its station-owned credentials and rate-limit buckets.
	JevJudgmentsEnabled bool

	// ServiceHealthURLs maps a dependency service name (none today) to the
	// URL the gateway should probe for GET /health/{service}. Empty entries fall back to defaults
	// (see DefaultServiceHealthURLs). These are used by the per-service
	// health endpoint and /health/ready aggregator so operators can observe
	// backend dependency health from outside the gateway (M22b / C20).
	ServiceHealthURLs map[string]string
}

const (
	// DefaultGatewayPort is the default gateway service port.
	DefaultGatewayPort = "8084"

	// DefaultAutoputerTokenTTL is the default TTL for autoputer credentials.
	DefaultAutoputerTokenTTL = 1 * time.Hour
)

// DefaultServiceHealthURLs are the default probe URLs for the per-service
// health endpoint and the /health/ready aggregator. Each must point at a
// service that runs on the host beside the gateway, or readiness can never
// go green. None does today: the former "runtime" and "dolt" probes targeted
// 127.0.0.1:8085, where nothing listens since computers run in VMs, "ollama"
// is not deployed, and qdrant was removed (owner, 2026-10-28)
// (docs/archive/problems/surface-broken-calls-2026-10-14.md, item 5). An entry added
// here can be overridden via GATEWAY_HEALTH_<SERVICE>_URL.
var DefaultServiceHealthURLs = map[string]string{}

// LoadConfig resolves gateway configuration from environment variables.
func LoadConfig() Config {
	port := envOr("GATEWAY_PORT", DefaultGatewayPort)

	ttl := DefaultAutoputerTokenTTL
	if v := os.Getenv("GATEWAY_AUTOPUTER_TOKEN_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			ttl = d
		}
	}

	return Config{
		Port:                port,
		AutoputerTokenTTL:   ttl,
		IdentityStorePath:   os.Getenv("GATEWAY_IDENTITY_STORE_PATH"),
		JevJudgmentsEnabled: envBool("GATEWAY_JEV_JUDGMENTS_ENABLED", false),
		ServiceHealthURLs:   loadServiceHealthURLs(),
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

func envBool(key string, fallback bool) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
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

// AutoputerIdentity represents a registered autoputer caller with its
// authentication credential.
type AutoputerIdentity struct {
	// ComputerID is the unique autoputer identifier.
	ComputerID string

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

	// PeerIP is the guest-network source address this bearer is pinned to.
	// The host moves it through the credential-ensure route (vmctl attests
	// each boot's address); in enforce mode the first authorized use of an
	// unpinned bearer sets it; the Jev route always does (BindPeer). It
	// prevents a bearer copied to a different VM from being reused there;
	// credential rotation clears the binding. Every gateway route that
	// serves a computer checks it (checkBearerBinding), under the mode set
	// by GATEWAY_BEARER_BINDING.
	//
	// The JSON key stays "JevPeerIP", from when only the Jev route used the
	// pin: a stored identity file keeps loading, and a rollback to a gateway
	// that only knows the Jev pin keeps reading it.
	PeerIP string `json:"JevPeerIP"`
}

// IdentityRegistry manages autoputer identities and their credentials.
// It supports issuance, validation, revocation, and invalidation of
// stale credentials (VAL-GATEWAY-008).
type IdentityRegistry struct {
	mu              sync.Mutex
	identities      map[string]*AutoputerIdentity // computer_id -> identity
	tokenTTL        time.Duration
	persistencePath string
	renewals        int64 // host-attested renewals since start
}

// NewIdentityRegistry creates a new identity registry with the given
// credential TTL.
func NewIdentityRegistry(tokenTTL time.Duration) *IdentityRegistry {
	return &IdentityRegistry{
		identities: make(map[string]*AutoputerIdentity),
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

	var identities map[string]*AutoputerIdentity
	if err := json.Unmarshal(data, &identities); err != nil {
		return fmt.Errorf("decode identity store: %w", err)
	}
	if identities == nil {
		identities = make(map[string]*AutoputerIdentity)
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
// The RawToken is shown once and must be communicated to the autoputer
// out-of-band (e.g., via VM bootstrap material).
type CredentialResult struct {
	ComputerID string
	RawToken   string
	ExpiresAt  time.Time
}

// CredentialEnsureResult is returned when an existing VM bootstrap token has
// been reconciled into the gateway's durable identity store.
type CredentialEnsureResult struct {
	ComputerID string    `json:"computer_id"`
	Status     string    `json:"status"`
	ExpiresAt  time.Time `json:"expires_at"`
	// JevPeerMoved reports that the ensure moved the Jev pin to the peer it
	// named (EnsureCredentialForPeer).
	JevPeerMoved bool `json:"jev_peer_moved,omitempty"`
	// Renewed reports that the ensure extended the credential's expiry
	// because less than half its life was left (EnsureCredentialForPeer).
	Renewed bool `json:"renewed,omitempty"`
}

// IssueCredential creates a new credential for the given autoputer ID.
// If an existing credential exists, it is revoked and replaced.
// Returns the raw token (shown once) and an error if generation fails.
func (r *IdentityRegistry) IssueCredential(computerID string) (*CredentialResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	token, err := generateSecureToken(32)
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	now := time.Now()
	hash := sha256.Sum256([]byte(token))

	// Revoke any existing credential for this autoputer.
	if existing, ok := r.identities[computerID]; ok {
		existing.Active = false
	}

	identity := &AutoputerIdentity{
		ComputerID: computerID,
		TokenHash:  hex.EncodeToString(hash[:]),
		IssuedAt:   now,
		ExpiresAt:  now.Add(r.tokenTTL),
		Active:     true,
	}
	r.identities[computerID] = identity
	if err := r.persistLocked(); err != nil {
		return nil, err
	}

	return &CredentialResult{
		ComputerID: computerID,
		RawToken:   computerID + ":" + token,
		ExpiresAt:  identity.ExpiresAt,
	}, nil
}

// EnsureCredential imports an already-issued raw autoputer credential into the
// registry if the autoputer identity is currently unknown. This is intentionally
// narrower than IssueCredential: it never returns the raw token, refuses to
// overwrite a different known credential, and refuses to reactivate revoked or
// expired credentials. It exists to repair VMs that were booted before gateway
// identity persistence existed, where the host still has the VM bootstrap token
// but the restarted gateway has lost the token hash.
func (r *IdentityRegistry) EnsureCredential(rawToken string) (*CredentialEnsureResult, error) {
	return r.EnsureCredentialForPeer(rawToken, "")
}

// EnsureCredentialForPeer is EnsureCredential from host lifecycle control
// that also names the computer's current guest address. A computer keeps its
// token across boots but takes a new guest address from each boot's host
// port, so the pin (PeerIP, BindPeer) is moved to the address the host
// attests; only this caller can move it, never the Jev route itself. An
// empty peer leaves the pin as it is; an address that is not a Choir guest
// address is refused and changes nothing
// (docs/archive/problems/l2a-merge-recall-on-the-paper-2026-10-14.md).
func (r *IdentityRegistry) EnsureCredentialForPeer(rawToken, peerIP string) (*CredentialEnsureResult, error) {
	peerIP = strings.TrimSpace(peerIP)
	if peerIP != "" {
		ip := net.ParseIP(peerIP)
		if ip == nil || !isChoirGuestPeer(ip) {
			return nil, fmt.Errorf("peer %q is not a Choir guest address", peerIP)
		}
		peerIP = ip.String()
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	computerID, token, ok := splitCredential(rawToken)
	if !ok || strings.TrimSpace(computerID) == "" || strings.TrimSpace(token) == "" {
		return nil, fmt.Errorf("invalid credential format")
	}
	hash := sha256.Sum256([]byte(token))
	hashText := hex.EncodeToString(hash[:])
	now := time.Now()

	if existing, ok := r.identities[computerID]; ok {
		if !hashesEqual(existing.TokenHash, hashText) {
			return nil, fmt.Errorf("credential conflict")
		}
		if !existing.Active {
			return nil, fmt.Errorf("credential revoked")
		}
		if now.After(existing.ExpiresAt) {
			return nil, fmt.Errorf("credential expired")
		}
		moved := peerIP != "" && existing.PeerIP != peerIP
		// The host vouches for a live computer each time it ensures its
		// credential, so an ensure also renews a credential with less than
		// half its life left. Without it a computer that is only ever
		// refreshed or recovered (both keep the token) loses its gateway one
		// TTL after its last cold boot. Only this host-internal route
		// renews; using a bearer never does, and an expired or revoked
		// credential was already refused above.
		renewed := r.tokenTTL > 0 && existing.ExpiresAt.Sub(now) < r.tokenTTL/2
		if moved || renewed {
			previousPeer, previousExpires := existing.PeerIP, existing.ExpiresAt
			if moved {
				existing.PeerIP = peerIP
			}
			if renewed {
				existing.ExpiresAt = now.Add(r.tokenTTL)
			}
			if err := r.persistLocked(); err != nil {
				existing.PeerIP, existing.ExpiresAt = previousPeer, previousExpires
				return nil, err
			}
			if renewed {
				r.renewals++
			}
		}
		return &CredentialEnsureResult{
			ComputerID:   computerID,
			Status:       "already_active",
			ExpiresAt:    existing.ExpiresAt,
			JevPeerMoved: moved,
			Renewed:      renewed,
		}, nil
	}

	identity := &AutoputerIdentity{
		ComputerID: computerID,
		TokenHash:  hashText,
		IssuedAt:   now,
		ExpiresAt:  now.Add(r.tokenTTL),
		Active:     true,
		PeerIP:     peerIP,
	}
	r.identities[computerID] = identity
	if err := r.persistLocked(); err != nil {
		return nil, err
	}
	return &CredentialEnsureResult{
		ComputerID: computerID,
		Status:     "imported",
		ExpiresAt:  identity.ExpiresAt,
	}, nil
}

// ValidateCredential checks whether a autoputer credential is valid.
// Returns the autoputer ID if valid, or an error explaining why not.
func (r *IdentityRegistry) ValidateCredential(rawToken string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	computerID, token, ok := splitCredential(rawToken)
	if !ok {
		return "", fmt.Errorf("invalid credential format")
	}

	identity, ok := r.identities[computerID]
	if !ok {
		return "", fmt.Errorf("unknown autoputer identity")
	}

	if !identity.Active {
		return "", fmt.Errorf("credential revoked")
	}

	if time.Now().After(identity.ExpiresAt) {
		return "", fmt.Errorf("credential expired")
	}

	// Verify the token hash in constant time (SEC-05): the stored value is
	// a hash of a random token, so a timing leak is of low value, but the
	// comparison costs nothing to make safe.
	hash := sha256.Sum256([]byte(token))
	if !hashesEqual(hex.EncodeToString(hash[:]), identity.TokenHash) {
		return "", fmt.Errorf("invalid credential")
	}

	return computerID, nil
}

// PeerState is the outcome of checking a call's source address against a
// bearer's pin.
type PeerState int

const (
	// PeerMatch: the pin is this address.
	PeerMatch PeerState = iota
	// PeerBound: the bearer was unpinned and is now pinned to this address.
	PeerBound
	// PeerUnpinned: the bearer has no pin and was left that way.
	PeerUnpinned
	// PeerMismatch: the bearer is pinned to another address.
	PeerMismatch
)

// CheckPeer compares a validated bearer's source address with its pin. With
// bindIfUnpinned an unpinned bearer is pinned to peerIP and the pin is made
// durable first: if it cannot be written, nothing is pinned and the error says
// so (a caller that enforces then refuses). The returned string is the pin as
// it stands after the call. It never moves an existing pin: only
// EnsureCredentialForPeer, which the host alone can call, does.
func (r *IdentityRegistry) CheckPeer(computerID, peerIP string, bindIfUnpinned bool) (PeerState, string, error) {
	peerIP = strings.TrimSpace(peerIP)
	if computerID == "" || peerIP == "" {
		return PeerUnpinned, "", fmt.Errorf("peer binding requires computer and peer")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	identity, ok := r.identities[computerID]
	if !ok || !identity.Active || time.Now().After(identity.ExpiresAt) {
		return PeerUnpinned, "", fmt.Errorf("credential is not active")
	}
	switch {
	case identity.PeerIP == peerIP:
		return PeerMatch, identity.PeerIP, nil
	case identity.PeerIP != "":
		return PeerMismatch, identity.PeerIP, nil
	case !bindIfUnpinned:
		return PeerUnpinned, "", nil
	}
	identity.PeerIP = peerIP
	if err := r.persistLocked(); err != nil {
		identity.PeerIP = ""
		return PeerUnpinned, "", err
	}
	return PeerBound, peerIP, nil
}

// BindPeer binds a validated bearer to its first authorized VM source
// address. Subsequent use from any other guest peer is refused. This binding
// is persisted alongside the token hash so a gateway restart cannot widen the
// bearer to every runtime VM again. The Jev route no longer calls it: its
// first-use pin is skipped only in strict (Handler.jevPeerAllowed).
func (r *IdentityRegistry) BindPeer(computerID, peerIP string) error {
	if strings.TrimSpace(computerID) == "" || strings.TrimSpace(peerIP) == "" {
		return fmt.Errorf("Jev peer binding requires computer and peer")
	}
	state, _, err := r.CheckPeer(computerID, peerIP, true)
	if err != nil {
		return err
	}
	if state == PeerMismatch {
		return fmt.Errorf("Jev bearer is bound to another VM")
	}
	return nil
}

// PinCounts reports how many active, unexpired identities have a pin and how
// many do not.
func (r *IdentityRegistry) PinCounts() (pinned, unpinned int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	for _, id := range r.identities {
		if !id.Active || !now.Before(id.ExpiresAt) {
			continue
		}
		if id.PeerIP != "" {
			pinned++
		} else {
			unpinned++
		}
	}
	return pinned, unpinned
}

// CredentialLifetime summarizes the active credentials' remaining life for
// /health: how many are active, how many expire within seven days, the time
// to the soonest expiry, and the host-attested renewals since start. It names
// no computer and no secret.
func (r *IdentityRegistry) CredentialLifetime() (active, expiringWithin7d int, soonest time.Duration, renewals int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	for _, id := range r.identities {
		if !id.Active || !now.Before(id.ExpiresAt) {
			continue
		}
		active++
		left := id.ExpiresAt.Sub(now)
		if left < 7*24*time.Hour {
			expiringWithin7d++
		}
		if active == 1 || left < soonest {
			soonest = left
		}
	}
	return active, expiringWithin7d, soonest, r.renewals
}

// hashesEqual compares two hex-encoded credential hashes in constant time.
// An empty hash matches nothing, not even another empty one.
func hashesEqual(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// RevokeCredential revokes the credential for the given autoputer ID.
// After revocation, the old credential no longer authorizes provider
// requests (VAL-GATEWAY-008).
func (r *IdentityRegistry) RevokeCredential(computerID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if identity, ok := r.identities[computerID]; ok {
		identity.Active = false
		_ = r.persistLocked()
	}
}

// RotateCredential revokes the existing credential and issues a new one.
// This is used when autoputer credentials are rotated for security or
// after lifecycle changes.
func (r *IdentityRegistry) RotateCredential(computerID string) (*CredentialResult, error) {
	return r.IssueCredential(computerID)
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

// splitCredential splits a "computerID:token" credential into its parts.
func splitCredential(raw string) (computerID, token string, ok bool) {
	for i := 0; i < len(raw); i++ {
		if raw[i] == ':' {
			return raw[:i], raw[i+1:], true
		}
	}
	return "", "", false
}
