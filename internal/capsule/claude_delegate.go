package capsule

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/claudedelegate"
)

// Claude delegate, guest side
// (docs/problems/claude-delegate-token-on-three-computers-2026-11-03.md,
// design section; docs/design/claude-delegate-slice1-2026-10-14.md). No
// computer holds the delegate's credential: the gateway does, and answers
// whether it serves this computer (its grant read). A claude-mode capsule's
// claude process reaches the gateway's relay through the capsule's egress
// proxy (ModelRelayRouteHost, egress_model_relay.go) with a relay secret the
// executor mints for that capsule; the proxy swaps the secret for this
// computer's gateway bearer.
const (
	// ClaudeDelegateBrokerTokenEnv carries the capsule's relay secret from
	// the executor into a claude-mode capsule's broker process, which
	// removes it from its environment as soon as it reads it.
	ClaudeDelegateBrokerTokenEnv = "CHOIR_CLAUDE_DELEGATE_TOKEN"
	// ClaudeDelegateBrokerBinEnv names the claude binary for the broker.
	// The binary is in the guest store but not on any PATH.
	ClaudeDelegateBrokerBinEnv = "CHOIR_CLAUDE_CODE_BIN"
	// claudeDelegateGrantPath is the gateway's grant read.
	claudeDelegateGrantPath = "/provider/anthropic/grant"
	// claudeDelegateGrantTTL is how long a grant read answers for; a
	// revoked or closed grant reaches the guest within it, and the gateway
	// refuses the relay calls at once in any case.
	claudeDelegateGrantTTL     = 30 * time.Second
	claudeDelegateGrantTimeout = 3 * time.Second
	claudeDelegateGrantMax     = 64 << 10
)

// ClaudeDelegateStatus is the delegate's availability on this computer, for
// the runtime, the prompt and /api/runtime/observability. Never a secret.
type ClaudeDelegateStatus struct {
	Available  bool   `json:"available"`
	Reason     string `json:"reason"`
	ComputerID string `json:"computer_id,omitempty"`
	ExpiresAt  string `json:"expires_at,omitempty"`
	Model      string `json:"model,omitempty"`
	// DailyTokenCap and DailyTokensUsed are the gateway's spend cap for this
	// computer and the day's use.
	DailyTokenCap   int64 `json:"daily_token_cap,omitempty"`
	DailyTokensUsed int64 `json:"daily_tokens_used,omitempty"`
}

// ErrClaudeDelegateUnavailable marks a claude-mode spawn refused because the
// delegate is not available now.
var ErrClaudeDelegateUnavailable = errors.New("claude delegate unavailable")

// ClaudeDelegateGateway reads this computer's delegate grant from the
// gateway. Its own lock serializes the reads; it never takes the executor's.
type ClaudeDelegateGateway struct {
	// URL is the gateway's base URL (RUNTIME_GATEWAY_URL); Token is this
	// computer's gateway bearer; Binary is the claude binary in the image.
	URL    string
	Token  func() string
	Binary string
	Client *http.Client
	Now    func() time.Time

	mu       sync.Mutex
	cached   ClaudeDelegateStatus
	cachedAt time.Time
}

// relayGrantReply is the gateway's grant read (internal/gateway,
// handleAnthropicRelayGrant).
type relayGrantReply struct {
	Available       bool   `json:"available"`
	Reason          string `json:"reason"`
	ExpiresAt       string `json:"expires_at"`
	Model           string `json:"model"`
	DailyTokenCap   int64  `json:"daily_token_cap"`
	DailyTokensUsed int64  `json:"daily_tokens_used"`
}

// Status is the delegate's availability now. A read that fails for any
// reason says unavailable, with the reason; it is cached for
// claudeDelegateGrantTTL either way.
func (g *ClaudeDelegateGateway) Status() ClaudeDelegateStatus {
	if g == nil {
		return ClaudeDelegateStatus{Reason: "this computer has no gateway for the Claude delegate"}
	}
	now := time.Now
	if g.Now != nil {
		now = g.Now
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.cachedAt.IsZero() && now().Sub(g.cachedAt) < claudeDelegateGrantTTL {
		return g.cached
	}
	g.cached, g.cachedAt = g.read(), now()
	return g.cached
}

func (g *ClaudeDelegateGateway) read() ClaudeDelegateStatus {
	binary := strings.TrimSpace(g.Binary)
	if binary == "" {
		return ClaudeDelegateStatus{Reason: "the guest image names no claude binary"}
	}
	if info, err := os.Stat(binary); err != nil || info.IsDir() || info.Mode()&0o111 == 0 {
		return ClaudeDelegateStatus{Reason: "the claude binary is missing from the guest image"}
	}
	base := strings.TrimRight(strings.TrimSpace(g.URL), "/")
	if base == "" {
		return ClaudeDelegateStatus{Reason: "this computer has no gateway URL"}
	}
	if _, ok := GatewayRouteAddr(base); !ok {
		return ClaudeDelegateStatus{Reason: "the gateway URL is not a literal address the capsule route can use"}
	}
	token := ""
	if g.Token != nil {
		token = strings.TrimSpace(g.Token())
	}
	if token == "" {
		return ClaudeDelegateStatus{Reason: "this computer has no gateway credential"}
	}
	ctx, cancel := context.WithTimeout(context.Background(), claudeDelegateGrantTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+claudeDelegateGrantPath, nil)
	if err != nil {
		return ClaudeDelegateStatus{Reason: "the grant read could not be built"}
	}
	req.Header.Set("Authorization", "Bearer "+token)
	client := g.Client
	if client == nil {
		client = &http.Client{Transport: &http.Transport{Proxy: nil}}
	}
	resp, err := client.Do(req)
	if err != nil {
		return ClaudeDelegateStatus{Reason: "the gateway did not answer the grant read"}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ClaudeDelegateStatus{Reason: fmt.Sprintf("the gateway refused the grant read (status %d)", resp.StatusCode)}
	}
	var grant relayGrantReply
	if err := json.NewDecoder(io.LimitReader(resp.Body, claudeDelegateGrantMax)).Decode(&grant); err != nil {
		return ClaudeDelegateStatus{Reason: "the gateway's grant read is malformed"}
	}
	status := ClaudeDelegateStatus{ExpiresAt: grant.ExpiresAt, Model: grant.Model, DailyTokenCap: grant.DailyTokenCap, DailyTokensUsed: grant.DailyTokensUsed}
	if status.Model == "" {
		status.Model = claudedelegate.Model
	}
	if !grant.Available {
		status.Reason = "the gateway withheld the delegate: " + strings.TrimSpace(grant.Reason)
		return status
	}
	status.Available = true
	status.Reason = "available through the gateway until " + grant.ExpiresAt
	return status
}

// GatewayRouteAddr is the gateway's address for a capsule route: the
// literal IP and port of its URL (port 80 when the URL names none). A name
// would need DNS, so it makes no route.
func GatewayRouteAddr(gatewayURL string) (netip.AddrPort, bool) {
	u, err := url.Parse(strings.TrimSpace(gatewayURL))
	if err != nil || u.Scheme != "http" || u.Hostname() == "" {
		return netip.AddrPort{}, false
	}
	ip, err := netip.ParseAddr(u.Hostname())
	if err != nil {
		return netip.AddrPort{}, false
	}
	port := uint64(80)
	if p := u.Port(); p != "" {
		if port, err = strconv.ParseUint(p, 10, 16); err != nil || port == 0 {
			return netip.AddrPort{}, false
		}
	}
	return netip.AddrPortFrom(ip.Unmap(), uint16(port)), true
}

// ClaudeRelaySecretPrefix starts every relay secret NewClaudeRelaySecret
// mints.
const ClaudeRelaySecretPrefix = "choir-relay-"

// NewClaudeRelaySecret is a claude-mode capsule's relay secret: 32 random
// bytes, hex. It lets the capsule's egress proxy tell the delegate's claude
// process from a cell in the same capsule, and dies with the capsule.
func NewClaudeRelaySecret() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return ClaudeRelaySecretPrefix + hex.EncodeToString(b[:]), nil
}

// IsClaudeRelaySecret reports a value of exactly the minted shape: the
// prefix and 64 lowercase hex digits. It is gateway-scoped (good only at the
// capsule's own egress proxy, which swaps it for the computer's bearer), so
// it is not the owner's credential.
func IsClaudeRelaySecret(s string) bool {
	rest, ok := strings.CutPrefix(s, ClaudeRelaySecretPrefix)
	if !ok || len(rest) != 64 {
		return false
	}
	for i := 0; i < len(rest); i++ {
		if c := rest[i]; !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// ClaudeEgressPolicy is the claude network mode: no host at all by CONNECT
// (the model API is the gateway's relay, through ModelRelayRouteHost, which
// the executor adds with the capsule's secret), and a budget sized for an
// LLM client that resends its whole conversation on every request (the L2
// upload budget is 16 MiB).
func ClaudeEgressPolicy() EgressPolicy {
	return EgressPolicy{
		MaxUploadBytes:   512 << 20,
		MaxDownloadBytes: 1 << 30,
		MaxConnections:   4096,
	}
}

// Claude-mode capsule size: 2 CPUs and 512 pids always (Claude Code beside
// go test needs more threads than 256), and 3 GiB when the guest's capsule
// budget holds it (an 8 GiB guest's does; a 4 GiB guest's budget is about
// 2.85 GiB), else the caller's default memory.
const (
	ClaudeCapsuleMemory   = int64(3 << 30)
	ClaudeCapsuleCPUQuota = int64(200000)
	ClaudeCapsulePidsMax  = int64(512)
)

// ClaudeCapsuleSize picks a claude-mode capsule's memory, CPU quota and pids
// for a guest whose capsule budget is budget bytes.
func ClaudeCapsuleSize(budget, defMemory int64) (memory, cpu, pids int64) {
	memory = defMemory
	if budget >= ClaudeCapsuleMemory {
		memory = ClaudeCapsuleMemory
	}
	return memory, ClaudeCapsuleCPUQuota, ClaudeCapsulePidsMax
}
