package vmmanager

import (
	"bytes"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/claudedelegate"
)

// docs/design/claude-delegate-slice1-2026-10-14.md §5, host side. Failure
// modes pinned here:
//   F1  the token file is absent (or not owner-only, or not a token) and the
//       disk still says "available", or carries bytes from the file;
//   F2  a computer off the allowlist gets anything on its disk;
//   F3  an unset, unreadable or past expiry still hands out the token;
//   F5  a status line (vmctl's journal) or the grant carries the token;
//   the default (empty) configuration turns anything on.

const hostFakeToken = "sk-ant-oat01-FAKE-host-token-0123456789abcdefghijk"

func delegateEnv(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func writeHostToken(t *testing.T, content string, mode os.FileMode) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "claude-oauth-token")
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestClaudeDelegateDefaultIsOff(t *testing.T) {
	cfg := loadClaudeDelegateConfig(delegateEnv(nil))
	if len(cfg.Computers) != 0 {
		t.Fatalf("default allowlist = %v", cfg.Computers)
	}
	if files, status := cfg.claudeDelegateMaterial("computer-1", time.Now()); files != nil || status != "" {
		t.Fatalf("default config wrote %v / %q", files, status)
	}
}

func TestClaudeDelegateMaterial(t *testing.T) {
	now := time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)
	good := writeHostToken(t, hostFakeToken+"\n", 0o400)
	cases := []struct {
		name       string
		env        map[string]string
		computer   string
		wantFiles  bool
		wantToken  bool
		wantReason string
	}{
		{"not allowlisted", map[string]string{claudeDelegateComputersEnv: "computer-2", claudeDelegateExpiresEnv: "2026-10-22", claudeDelegateTokenFileEnv: good}, "computer-1", false, false, ""},
		{"available", map[string]string{claudeDelegateComputersEnv: "computer-2, computer-1", claudeDelegateExpiresEnv: "2026-10-22", claudeDelegateTokenFileEnv: good}, "computer-1", true, true, ""},
		{"expiry unset", map[string]string{claudeDelegateComputersEnv: "computer-1", claudeDelegateTokenFileEnv: good}, "computer-1", true, false, "is not set"},
		{"expiry garbage", map[string]string{claudeDelegateComputersEnv: "computer-1", claudeDelegateExpiresEnv: "next week", claudeDelegateTokenFileEnv: good}, "computer-1", true, false, "not an RFC 3339"},
		{"window closed", map[string]string{claudeDelegateComputersEnv: "computer-1", claudeDelegateExpiresEnv: "2026-10-15T08:59:59Z", claudeDelegateTokenFileEnv: good}, "computer-1", true, false, "window closed"},
		{"token file absent", map[string]string{claudeDelegateComputersEnv: "computer-1", claudeDelegateExpiresEnv: "2026-10-22", claudeDelegateTokenFileEnv: filepath.Join(t.TempDir(), "absent")}, "computer-1", true, false, "token file absent"},
		{"token path unset", map[string]string{claudeDelegateComputersEnv: "computer-1", claudeDelegateExpiresEnv: "2026-10-22"}, "computer-1", true, false, "is not set"},
		{"group readable", map[string]string{claudeDelegateComputersEnv: "computer-1", claudeDelegateExpiresEnv: "2026-10-22", claudeDelegateTokenFileEnv: writeHostToken(t, hostFakeToken, 0o440)}, "computer-1", true, false, "must be 0400"},
		{"not a token", map[string]string{claudeDelegateComputersEnv: "computer-1", claudeDelegateExpiresEnv: "2026-10-22", claudeDelegateTokenFileEnv: writeHostToken(t, "two words "+hostFakeToken, 0o400)}, "computer-1", true, false, "valid token"},
		{"empty file", map[string]string{claudeDelegateComputersEnv: "computer-1", claudeDelegateExpiresEnv: "2026-10-22", claudeDelegateTokenFileEnv: writeHostToken(t, "", 0o400)}, "computer-1", true, false, "valid token"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := loadClaudeDelegateConfig(delegateEnv(tc.env))
			files, status := cfg.claudeDelegateMaterial(tc.computer, now)
			if (files != nil) != tc.wantFiles {
				t.Fatalf("files = %v, want present=%v", files, tc.wantFiles)
			}
			if !tc.wantFiles {
				return
			}
			token, hasToken := files[claudedelegate.TokenFile]
			if hasToken != tc.wantToken || (hasToken && strings.TrimSpace(string(token)) != hostFakeToken) {
				t.Fatalf("token on disk = %v, want %v", hasToken, tc.wantToken)
			}
			var grant claudedelegate.Grant
			if err := json.Unmarshal(files[claudedelegate.GrantFile], &grant); err != nil {
				t.Fatal(err)
			}
			if grant.ComputerID != tc.computer || (grant.Status == claudedelegate.GrantAvailable) != tc.wantToken {
				t.Fatalf("grant = %+v", grant)
			}
			if !strings.Contains(grant.Reason, tc.wantReason) || !strings.Contains(status, tc.wantReason) {
				t.Fatalf("reason = %q / status %q, want ~%q", grant.Reason, status, tc.wantReason)
			}
			if tc.wantToken && grant.ExpiresAt != "2026-10-22T00:00:00Z" {
				t.Fatalf("expires_at = %q", grant.ExpiresAt)
			}
			if bytes.Contains(files[claudedelegate.GrantFile], []byte(hostFakeToken)) || strings.Contains(status, hostFakeToken) {
				t.Fatal("the grant or the status line carries the token")
			}
		})
	}
}

// The boot path logs the status line for an allowlisted computer; the
// journal must never see the token.
func TestClaudeDelegateBootLogNeverCarriesTheToken(t *testing.T) {
	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(prev)
	cfg := loadClaudeDelegateConfig(delegateEnv(map[string]string{
		claudeDelegateComputersEnv: "computer-1", claudeDelegateExpiresEnv: "2099-01-01",
		claudeDelegateTokenFileEnv: writeHostToken(t, hostFakeToken, 0o400),
	}))
	_, status := cfg.claudeDelegateMaterial("computer-1", time.Now())
	log.Printf("vmmanager: claude delegate for computer %s (VM %s): %s", "computer-1", "vm-1", status)
	if !strings.Contains(buf.String(), "available until 2099-01-01") || strings.Contains(buf.String(), hostFakeToken) {
		t.Fatalf("log = %q", buf.String())
	}
}
