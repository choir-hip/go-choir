package capsule

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/claudedelegate"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// docs/design/claude-delegate-slice1-2026-10-14.md §5 and
// docs/problems/claude-delegate-token-on-three-computers-2026-11-03.md (design
// section), guest side. Failure modes pinned here:
//   F1/F2/F3 the gateway withholds the grant (no credential, not listed,
//       window closed) and the guest reports the delegate available;
//   20  the grant read hangs, or a failed read says available;
//   F4  a capsule hop's environment builder carries the relay secret;
//   F5  a status reason quotes a secret or the gateway's body beyond its
//       reason;
//   F10/19 claude-mode egress reaches any host by CONNECT, the model API
//       included, or keeps the 16 MiB L2 upload budget;
//   22  the relay secret is predictable;
//   F17 a 3 GiB claude capsule is requested where the budget cannot hold it.

const testDelegateToken = "choir-relay-FAKE-guest-secret-0123456789abcdefghij"

func fakeClaudeBinary(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "claude")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return binary
}

func TestClaudeDelegateGrantFromTheGateway(t *testing.T) {
	const bearer = "gateway-bearer-FAKE-0123456789"
	var reply atomic.Value
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/provider/anthropic/grant" || r.Method != http.MethodGet || r.Header.Get("Authorization") != "Bearer "+bearer {
			http.Error(w, "no", http.StatusUnauthorized)
			return
		}
		body := reply.Load().(string)
		if body == "hang" {
			<-r.Context().Done()
			return
		}
		if body == "500" {
			http.Error(w, "boom "+bearer, http.StatusInternalServerError)
			return
		}
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	binary := fakeClaudeBinary(t)
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	newGW := func(token string) *ClaudeDelegateGateway {
		return &ClaudeDelegateGateway{URL: srv.URL, Token: func() string { return token }, Binary: binary, Now: func() time.Time { return now }}
	}
	cases := []struct {
		name, reply, token string
		available          bool
		reason             string
	}{
		{"available", `{"available":true,"expires_at":"2026-11-17T00:00:00Z","model":"claude-sonnet-5-5","daily_token_cap":50000000}`, bearer, true, "available through the gateway until 2026-11-17"},
		{"not listed", `{"available":false,"reason":"this computer has no Claude delegate grant"}`, bearer, false, "withheld the delegate: this computer has no Claude delegate grant"},
		{"no credential", `{"available":false,"reason":"the gateway holds no Claude credential"}`, bearer, false, "holds no Claude credential"},
		{"window closed", `{"available":false,"reason":"the Claude delegate window is closed"}`, bearer, false, "window is closed"},
		{"refused", `{}`, "wrong-bearer", false, "refused the grant read (status 401)"},
		{"server error", "500", bearer, false, "refused the grant read (status 500)"},
		{"malformed", `{"available":`, bearer, false, "malformed"},
		{"no bearer", `{"available":true}`, "", false, "no gateway credential"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reply.Store(tc.reply)
			status := newGW(tc.token).Status()
			if status.Available != tc.available || !strings.Contains(status.Reason, tc.reason) {
				t.Fatalf("status = %+v, want available=%v reason~%q", status, tc.available, tc.reason)
			}
			raw, _ := json.Marshal(status)
			if strings.Contains(string(raw), bearer) {
				t.Fatalf("status quotes the gateway bearer: %s", raw)
			}
		})
	}
	// The answer is cached: a revoke reaches the guest within the TTL.
	reply.Store(cases[0].reply)
	gw := newGW(bearer)
	before := calls.Load()
	if !gw.Status().Available || !gw.Status().Available || calls.Load() != before+1 {
		t.Fatalf("two reads within the TTL made %d calls", calls.Load()-before)
	}
	reply.Store(cases[1].reply)
	now = now.Add(claudeDelegateGrantTTL)
	if gw.Status().Available {
		t.Fatal("a revoked grant still reads available after the TTL")
	}
	// A hung gateway answers unavailable within the timeout.
	reply.Store("hang")
	start := time.Now()
	if st := newGW(bearer).Status(); st.Available || !strings.Contains(st.Reason, "did not answer") {
		t.Fatalf("hung gateway: %+v", st)
	}
	if time.Since(start) > claudeDelegateGrantTimeout+2*time.Second {
		t.Fatalf("the grant read took %s", time.Since(start))
	}
	// No binary, no URL, a name instead of an address, a nil source.
	reply.Store(cases[0].reply)
	missing := newGW(bearer)
	missing.Binary = filepath.Join(t.TempDir(), "absent")
	if st := missing.Status(); st.Available || !strings.Contains(st.Reason, "binary is missing") {
		t.Fatalf("no binary: %+v", st)
	}
	named := newGW(bearer)
	named.URL = "http://gateway.example:8084"
	if st := named.Status(); st.Available {
		t.Fatalf("a named gateway URL: %+v", st)
	}
	if st := (*ClaudeDelegateGateway)(nil).Status(); st.Available {
		t.Fatal("a nil source is available")
	}
}

func TestGatewayRouteAddr(t *testing.T) {
	for raw, want := range map[string]string{
		"http://172.16.0.1:8084":  "172.16.0.1:8084",
		"http://172.16.0.1":       "172.16.0.1:80",
		"http://172.16.0.1:8084/": "172.16.0.1:8084",
	} {
		if got, ok := GatewayRouteAddr(raw); !ok || got.String() != want {
			t.Errorf("%s = %v %v, want %s", raw, got, ok, want)
		}
	}
	for _, raw := range []string{"", "https://172.16.0.1:8084", "http://gateway:8084", "http://172.16.0.1:0", "::"} {
		if _, ok := GatewayRouteAddr(raw); ok {
			t.Errorf("%q made a route", raw)
		}
	}
}

func TestClaudeRelaySecretIsRandom(t *testing.T) {
	a, err := NewClaudeRelaySecret()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := NewClaudeRelaySecret()
	if a == b || len(a) < 64 || !claudedelegate.ValidToken(a) {
		t.Fatalf("secrets %q %q", a, b)
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

func TestClaudeEgressReachesNoHostByConnect(t *testing.T) {
	policy := ClaudeEgressPolicy()
	if policy.MaxUploadBytes < 512<<20 || policy.MaxUploadBytes <= EcosystemEgressPolicy().MaxUploadBytes {
		t.Fatalf("claude upload budget = %d", policy.MaxUploadBytes)
	}
	policy.Routes = map[string]netip.AddrPort{GoProxyRouteHost: goProxyRoute}
	h := newEgressHarness(t, policy)
	if len(policy.Hosts) != 0 {
		t.Fatalf("claude mode allows CONNECT hosts %v; the model API is the gateway relay", policy.Hosts)
	}
	for _, target := range []string{
		"api.anthropic.com:443", "github.com:443", "codeload.github.com:443", "registry.npmjs.org:443", "pypi.org:443", "proxy.golang.org:443",
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
