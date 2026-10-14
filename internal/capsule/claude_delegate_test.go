package capsule

import (
	"encoding/json"
	"net"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// docs/design/claude-delegate-slice1-2026-10-14.md §5, guest side. Failure
// modes pinned here:
//   F1  the grant says the host has no token, yet the guest reports the
//       delegate available or hands out a token;
//   F2  no grant (not allowlisted) and the guest reports it available;
//   F3  the grant's window has closed (a snapshot-resumed disk) and the
//       guest still hands out the token;
//   F4  a capsule hop's environment builder carries the delegate token;
//   F5  a status reason quotes the token or a file's content;
//   F10 claude-mode egress reaches any host but the model API (GitHub,
//       npm, PyPI, sign-in hosts, look-alike names), or keeps the 16 MiB
//       L2 upload budget;
//   F17 a 3 GiB claude capsule is requested where the budget cannot hold it.

const testDelegateToken = "sk-ant-oat01-FAKE-guest-token-0123456789abcdefghij"

func writeDelegateFixture(t *testing.T, grant *ClaudeDelegateGrant, token string) (dir, binary string) {
	t.Helper()
	dir = t.TempDir()
	if grant != nil {
		raw, _ := json.Marshal(grant)
		if err := os.WriteFile(filepath.Join(dir, ClaudeDelegateGrantFile), raw, 0o400); err != nil {
			t.Fatal(err)
		}
	}
	if token != "" {
		if err := os.WriteFile(filepath.Join(dir, ClaudeDelegateTokenFile), []byte(token+"\n"), 0o400); err != nil {
			t.Fatal(err)
		}
	}
	binary = filepath.Join(t.TempDir(), "claude")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir, binary
}

func TestReadClaudeDelegate(t *testing.T) {
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	future := now.Add(48 * time.Hour).Format(time.RFC3339)
	past := now.Add(-time.Minute).Format(time.RFC3339)
	available := &ClaudeDelegateGrant{ComputerID: "computer-1", Status: ClaudeDelegateGrantAvailable, ExpiresAt: future}
	cases := []struct {
		name      string
		grant     *ClaudeDelegateGrant
		token     string
		noBinary  bool
		available bool
		reason    string
	}{
		{"not allowlisted", nil, "", false, false, "not on the host's Claude delegate allowlist"},
		{"host has no token", &ClaudeDelegateGrant{ComputerID: "computer-1", Status: "unavailable", Reason: "token file absent on the host", ExpiresAt: future}, "", false, false, "token file absent"},
		{"window closed", &ClaudeDelegateGrant{ComputerID: "computer-1", Status: ClaudeDelegateGrantAvailable, ExpiresAt: past}, testDelegateToken, false, false, "window closed"},
		{"no expiry", &ClaudeDelegateGrant{ComputerID: "computer-1", Status: ClaudeDelegateGrantAvailable}, testDelegateToken, false, false, "no valid expiry"},
		{"grant without token", available, "", false, false, "token is not on the credential disk"},
		{"malformed token", available, "short", false, false, "not a valid token"},
		{"binary missing", available, testDelegateToken, true, false, "binary is missing"},
		{"available", available, testDelegateToken, false, true, "available until"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir, binary := writeDelegateFixture(t, tc.grant, tc.token)
			if tc.noBinary {
				binary = filepath.Join(dir, "absent-claude")
			}
			token, status := ReadClaudeDelegate(dir, binary, now)
			if status.Available != tc.available || !strings.Contains(status.Reason, tc.reason) {
				t.Fatalf("status = %+v, want available=%v reason~%q", status, tc.available, tc.reason)
			}
			if tc.available != (token == testDelegateToken) {
				t.Fatalf("token handed out = %v, available = %v", token != "", tc.available)
			}
			raw, _ := json.Marshal(status)
			if strings.Contains(string(raw), testDelegateToken) || strings.Contains(string(raw), "short") && tc.token == "short" {
				t.Fatalf("status quotes the token or file content: %s", raw)
			}
		})
	}
	if _, status := ReadClaudeDelegate("", "/bin/sh", now); status.Available {
		t.Fatal("no credential dir must be unavailable")
	}
}

func TestBrokerAndCellEnvironmentsNeverCarryTheDelegateToken(t *testing.T) {
	env := map[string]string{
		"PATH": "/run/current-system/sw/bin", yaegikernel.BuildToolchainPathEnv: "/nix/store/gcc/bin",
		"CGO_CFLAGS": "-I/x", ClaudeDelegateBrokerTokenEnv: testDelegateToken, "CLAUDE_CODE_OAUTH_TOKEN": testDelegateToken,
	}
	lookup := func(k string) (string, bool) { v, ok := env[k]; return v, ok }
	for name, got := range map[string][]string{
		"broker process env":  BrokerProcessEnv(lookup),
		"toolchain carry":     yaegikernel.BuildToolchainCarry(lookup),
		"workload egress env": EgressProxyEnv(),
		"workload toolchain":  func() []string { p, rest := yaegikernel.WorkloadToolchainEnv("/bin", lookup); return append(rest, p) }(),
	} {
		if strings.Contains(strings.Join(got, "\n"), testDelegateToken) {
			t.Errorf("%s carries the delegate token", name)
		}
	}
}

func TestClaudeEgressReachesOnlyTheModelAPI(t *testing.T) {
	policy := ClaudeEgressPolicy()
	if policy.MaxUploadBytes < 512<<20 || policy.MaxUploadBytes <= EcosystemEgressPolicy().MaxUploadBytes {
		t.Fatalf("claude upload budget = %d", policy.MaxUploadBytes)
	}
	policy.Routes = map[string]netip.AddrPort{GoProxyRouteHost: goProxyRoute}
	h := newEgressHarness(t, policy)
	h.resolved["api.anthropic.com"] = []netip.Addr{publicAddr}
	_, _, resp := h.connect("api.anthropic.com:443")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("CONNECT api.anthropic.com:443 = %d", resp.StatusCode)
	}
	for _, target := range []string{
		"github.com:443", "codeload.github.com:443", "registry.npmjs.org:443", "pypi.org:443", "proxy.golang.org:443",
		"platform.claude.com:443", "claude.ai:443", "statsig.anthropic.com:443", "downloads.claude.ai:443",
		"api.anthropic.com.evil.com:443", "evilapi.anthropic.com:443", "api.anthropic.com:80", "huggingface.co:443",
	} {
		host, _, _ := net.SplitHostPort(target)
		h.resolved[host] = []netip.Addr{publicAddr}
		before := len(h.dials())
		_, _, resp := h.connect(target)
		if resp.StatusCode == http.StatusOK {
			t.Errorf("claude mode allowed CONNECT %s", target)
		}
		if len(h.dials()) != before {
			t.Errorf("refused %s still dialed", target)
		}
	}
	status, body := sendRaw(t, h.addr, "GET http://goproxy.choir.internal/golang.org/x/mod/@v/list HTTP/1.1\r\nHost: goproxy.choir.internal\r\n\r\n")
	if status != http.StatusOK || !strings.Contains(body, "host=goproxy.choir.internal") {
		t.Fatalf("Go module proxy route in claude mode = %d %q", status, body)
	}
}

func TestClaudeCapsuleSize(t *testing.T) {
	if m, c, p := ClaudeCapsuleSize(6<<30, 2<<30); m != 3<<30 || c != 200000 || p != 512 {
		t.Fatalf("8 GiB guest: %d %d %d", m, c, p)
	}
	// A 4096 MiB guest's budget is 3/4 of MemTotal, under 3 GiB.
	if m, c, _ := ClaudeCapsuleSize(int64(3_900_000_000)*3/4, 2<<30); m != 2<<30 || c != 200000 {
		t.Fatalf("4 GiB guest: memory %d cpu %d", m, c)
	}
}
