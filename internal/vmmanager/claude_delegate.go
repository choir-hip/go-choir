package vmmanager

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/yusefmosiah/go-choir/internal/claudedelegate"
)

// Claude delegate, host side (docs/design/claude-delegate-slice1-2026-10-14.md,
// Option T, owner-ratified 2026-10-14). Off unless both are set on vmctl
// (nix/node-b.nix, go-choir-vmctl Environment):
//
//	CHOIR_CLAUDE_DELEGATE_COMPUTERS   ComputerIDs, comma or space separated
//	CHOIR_CLAUDE_DELEGATE_EXPIRES     RFC 3339 time or YYYY-MM-DD (UTC)
//	CHOIR_CLAUDE_DELEGATE_TOKEN_FILE  where vmctl reads the owner's token
//
// At each cold boot of an allowlisted computer, vmctl writes a grant (and,
// when the token is usable and the window open, the token) onto that VM's
// root-only credential disk. vmctl never logs the token.
const (
	claudeDelegateComputersEnv = "CHOIR_CLAUDE_DELEGATE_COMPUTERS"
	claudeDelegateExpiresEnv   = "CHOIR_CLAUDE_DELEGATE_EXPIRES"
	claudeDelegateTokenFileEnv = "CHOIR_CLAUDE_DELEGATE_TOKEN_FILE"
	maxClaudeTokenFileBytes    = 8 << 10
)

// ClaudeDelegateConfig is the host switch.
type ClaudeDelegateConfig struct {
	Computers map[string]bool
	ExpiresAt time.Time
	TokenFile string
	// ConfigError explains an expiry that could not be read (the delegate is
	// then off for every listed computer).
	ConfigError string
}

func loadClaudeDelegateConfig(getenv func(string) string) ClaudeDelegateConfig {
	cfg := ClaudeDelegateConfig{Computers: map[string]bool{}, TokenFile: strings.TrimSpace(getenv(claudeDelegateTokenFileEnv))}
	for _, id := range strings.FieldsFunc(getenv(claudeDelegateComputersEnv), func(r rune) bool { return r == ',' || r == ' ' || r == '\n' || r == '\t' }) {
		cfg.Computers[id] = true
	}
	raw := strings.TrimSpace(getenv(claudeDelegateExpiresEnv))
	switch {
	case raw == "":
		cfg.ConfigError = claudeDelegateExpiresEnv + " is not set"
	default:
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			cfg.ExpiresAt = t.UTC()
		} else if t, err := time.Parse("2006-01-02", raw); err == nil {
			cfg.ExpiresAt = t.UTC()
		} else {
			cfg.ConfigError = claudeDelegateExpiresEnv + " is not an RFC 3339 time or YYYY-MM-DD"
		}
	}
	return cfg
}

// claudeDelegateMaterial decides what an allowlisted computer's credential
// disk carries. It returns no files for any other computer. The status line
// is safe to log: it never quotes the token or the file's bytes.
func (c ClaudeDelegateConfig) claudeDelegateMaterial(computerID string, now time.Time) (map[string][]byte, string) {
	computerID = strings.TrimSpace(computerID)
	if computerID == "" || !c.Computers[computerID] {
		return nil, ""
	}
	grant := claudedelegate.Grant{ComputerID: computerID, Status: claudedelegate.GrantUnavailable, GrantedAt: now.UTC().Format(time.RFC3339)}
	if !c.ExpiresAt.IsZero() {
		grant.ExpiresAt = c.ExpiresAt.Format(time.RFC3339)
	}
	var token string
	switch {
	case c.ConfigError != "":
		grant.Reason = c.ConfigError
	case !now.Before(c.ExpiresAt):
		grant.Reason = "the delegate window closed at " + grant.ExpiresAt
	default:
		var reason string
		token, reason = readClaudeDelegateToken(c.TokenFile)
		if token == "" {
			grant.Reason = reason
		} else {
			grant.Status = claudedelegate.GrantAvailable
		}
	}
	raw, _ := json.Marshal(grant)
	files := map[string][]byte{claudedelegate.GrantFile: append(raw, '\n')}
	if grant.Status == claudedelegate.GrantAvailable {
		files[claudedelegate.TokenFile] = []byte(token + "\n")
		return files, "available until " + grant.ExpiresAt
	}
	return files, "unavailable: " + grant.Reason
}

// readClaudeDelegateToken reads the owner's token file. It must be a
// regular, owner-only file owned by vmctl's own uid (root on Node B).
// Reasons never quote the content.
func readClaudeDelegateToken(path string) (string, string) {
	if strings.TrimSpace(path) == "" {
		return "", claudeDelegateTokenFileEnv + " is not set"
	}
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if errors.Is(err, os.ErrNotExist) {
		return "", "token file absent on the host"
	}
	if err != nil {
		return "", "token file unreadable on the host"
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return "", "token path on the host is not a regular file"
	}
	if info.Mode().Perm()&0o077 != 0 {
		return "", fmt.Sprintf("token file mode %04o is readable by group or others; it must be 0400", info.Mode().Perm())
	}
	if st, ok := info.Sys().(*syscall.Stat_t); ok && int(st.Uid) != os.Geteuid() {
		return "", "token file is not owned by root"
	}
	data, err := io.ReadAll(io.LimitReader(f, maxClaudeTokenFileBytes+1))
	if err != nil || len(data) > maxClaudeTokenFileBytes {
		return "", "token file is unreadable or too large"
	}
	token := strings.TrimSpace(string(data))
	if !claudedelegate.ValidToken(token) {
		return "", "token file does not hold a valid token"
	}
	return token, ""
}
