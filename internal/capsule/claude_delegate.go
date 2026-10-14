package capsule

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/claudedelegate"
)

// Claude delegate, guest side (docs/design/claude-delegate-slice1-2026-10-14.md).
// vmctl puts a grant record, and for an allowlisted computer with a usable
// token the token itself, on the root-only credential disk. The executor
// reads them only when it spawns a claude-mode capsule and hands the token
// to that capsule's broker; nothing else in the guest core reads them.
const (
	// DefaultClaudeDelegateCredentialDir is where the guest mounts the
	// credential disk (nix/autoputer-vm.nix, /run/choir-bootstrap).
	DefaultClaudeDelegateCredentialDir = "/run/choir-bootstrap"
	// ClaudeDelegateGrantFile and ClaudeDelegateTokenFile are the names
	// vmctl writes (internal/vmmanager/claude_delegate.go).
	ClaudeDelegateGrantFile = claudedelegate.GrantFile
	ClaudeDelegateTokenFile = claudedelegate.TokenFile
	// ClaudeDelegateBrokerTokenEnv carries the token from the executor into
	// a claude-mode capsule's broker process, which removes it from its
	// environment as soon as it reads it.
	ClaudeDelegateBrokerTokenEnv = "CHOIR_CLAUDE_DELEGATE_TOKEN"
	// ClaudeDelegateBrokerBinEnv names the claude binary for the broker.
	// The binary is in the guest store but not on any PATH.
	ClaudeDelegateBrokerBinEnv = "CHOIR_CLAUDE_CODE_BIN"
	// ClaudeDelegateGrantAvailable is the grant status that admits the mode.
	ClaudeDelegateGrantAvailable = claudedelegate.GrantAvailable
)

// ClaudeDelegateGrant is the grant record vmctl writes. It never carries the
// token.
type ClaudeDelegateGrant = claudedelegate.Grant

// ClaudeDelegateStatus is the delegate's availability on this computer, for
// the runtime, the prompt and /api/runtime/observability. Never the token.
type ClaudeDelegateStatus struct {
	Available  bool   `json:"available"`
	Reason     string `json:"reason"`
	ComputerID string `json:"computer_id,omitempty"`
	ExpiresAt  string `json:"expires_at,omitempty"`
	Model      string `json:"model,omitempty"`
}

// ErrClaudeDelegateUnavailable marks a claude-mode spawn refused because the
// delegate is not available now.
var ErrClaudeDelegateUnavailable = errors.New("claude delegate unavailable")

// ReadClaudeDelegate reads the grant and the token from the credential
// directory. The returned token is empty unless the status is available.
// Reasons never quote file contents.
func ReadClaudeDelegate(dir, binary string, now time.Time) (string, ClaudeDelegateStatus) {
	if strings.TrimSpace(dir) == "" {
		return "", ClaudeDelegateStatus{Reason: "no credential directory is configured"}
	}
	raw, err := os.ReadFile(filepath.Join(dir, ClaudeDelegateGrantFile))
	if errors.Is(err, os.ErrNotExist) {
		return "", ClaudeDelegateStatus{Reason: "this computer is not on the host's Claude delegate allowlist (no grant on the credential disk)"}
	}
	if err != nil {
		return "", ClaudeDelegateStatus{Reason: "the delegate grant is unreadable"}
	}
	var grant ClaudeDelegateGrant
	if err := json.Unmarshal(raw, &grant); err != nil {
		return "", ClaudeDelegateStatus{Reason: "the delegate grant is malformed"}
	}
	status := ClaudeDelegateStatus{ComputerID: grant.ComputerID, ExpiresAt: grant.ExpiresAt, Model: claudedelegate.Model}
	if grant.Status != ClaudeDelegateGrantAvailable {
		status.Reason = "the host withheld the delegate: " + strings.TrimSpace(grant.Reason)
		return "", status
	}
	expires, err := time.Parse(time.RFC3339, grant.ExpiresAt)
	if err != nil {
		status.Reason = "the delegate grant has no valid expiry"
		return "", status
	}
	if !now.Before(expires) {
		status.Reason = fmt.Sprintf("the delegate window closed at %s", expires.UTC().Format(time.RFC3339))
		return "", status
	}
	tokenBytes, err := os.ReadFile(filepath.Join(dir, ClaudeDelegateTokenFile))
	if err != nil {
		status.Reason = "the grant is present but the token is not on the credential disk"
		return "", status
	}
	token := strings.TrimSpace(string(tokenBytes))
	if !claudedelegate.ValidToken(token) {
		status.Reason = "the token on the credential disk is not a valid token"
		return "", status
	}
	if strings.TrimSpace(binary) == "" {
		status.Reason = "the guest image names no claude binary"
		return "", status
	}
	if info, err := os.Stat(binary); err != nil || info.IsDir() || info.Mode()&0o111 == 0 {
		status.Reason = "the claude binary is missing from the guest image"
		return "", status
	}
	status.Available = true
	status.Reason = "available until " + expires.UTC().Format(time.RFC3339)
	return token, status
}

// claudeAPIHosts is everything a claude-mode capsule may reach besides the
// Go module proxy route. Claude Code needs only the model API once it holds
// a setup-token token and nonessential traffic is off (UNVERIFIED for
// startup: the egress ledger records any refused host).
var claudeAPIHosts = []string{"api.anthropic.com"}

// ClaudeEgressPolicy is the claude network mode: the model API only, and a
// budget sized for an LLM client that resends its whole conversation on
// every request (the L2 upload budget is 16 MiB).
func ClaudeEgressPolicy() EgressPolicy {
	return EgressPolicy{
		Hosts:            append([]string(nil), claudeAPIHosts...),
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
