package capsule

import (
	"fmt"
	"io"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"
)

// The tunnel-identity check ships in observe (the metamission rule: new red
// behaviour ships off or in observe). The check refuses traffic, and a wrong
// refusal breaks builds (a GREASE Encrypted Client Hello, a client with no
// SNI), so observe judges the first record exactly as enforce does, writes
// the would-refuse verdict on the egress ledger entry, logs it once per
// capsule and host, and relays the record and the tunnel unchanged.
//
// Ways it could fail, pinned here:
//   - observe judges differently from enforce (a case enforce refuses that
//     observe passes, so the ledger's zero would-refuse count proves nothing);
//   - observe drops, alters or delays the first record, or closes the tunnel;
//   - observe records nothing, so the flip to enforce has no evidence;
//   - observe logs every connection (a busy build floods the journal) or
//     never logs;
//   - an unset or unknown mode is enforce (a typo breaks every build) or off
//     (a typo removes the evidence);
//   - the mode can be chosen from inside a capsule (it is the executor's,
//     read from the daemon's environment, set on the policy by the executor);
//   - off still reads or judges the first record.

func enforced(p EgressPolicy) EgressPolicy {
	p.TunnelIdentity = TunnelIdentityEnforce
	return p
}

func withMode(p EgressPolicy, mode TunnelIdentityMode) EgressPolicy {
	p.TunnelIdentity = mode
	return p
}

func TestTunnelIdentityModeFromEnv(t *testing.T) {
	for raw, want := range map[string]TunnelIdentityMode{
		"":          TunnelIdentityObserve,
		"observe":   TunnelIdentityObserve,
		" Observe ": TunnelIdentityObserve,
		"enforce":   TunnelIdentityEnforce,
		"ENFORCE":   TunnelIdentityEnforce,
		"off":       TunnelIdentityOff,
		"enforcing": TunnelIdentityObserve,
		"true":      TunnelIdentityObserve,
	} {
		got := TunnelIdentityModeFromEnv(func(key string) string {
			if key != TunnelIdentityEnv {
				t.Fatalf("read %q, want %q", key, TunnelIdentityEnv)
			}
			return raw
		})
		if got != want {
			t.Errorf("%s=%q: mode %q, want %q", TunnelIdentityEnv, raw, got, want)
		}
	}
}

// Every case enforce refuses through the proxy, plus the other malformed
// hellos the verifier table refuses.
func observeCases(t *testing.T) []struct {
	name, host string
	payload    []byte
	reason     string
} {
	return []struct {
		name, host string
		payload    []byte
		reason     string
	}{
		{"another server name", "github.com", realClientHello(t, "pypi.org"), "SNI"},
		{"no sni", "github.com", handBuiltHello(helloOpts{noSNI: true}), "SNI"},
		{"two names", "github.com", handBuiltHello(helloOpts{names: []string{"github.com", "pypi.org"}}), "SNI"},
		{"encrypted client hello", "github.com", handBuiltHello(helloOpts{names: []string{"github.com"}, ech: true}), "Encrypted Client Hello"},
		{"hello split across records", "github.com", handBuiltHello(helloOpts{names: []string{"github.com"}, fragmentTail: 64}), "split"},
		{"sni is an ip literal", "github.com", handBuiltHello(helloOpts{names: []string{"140.82.112.3"}}), "plain host name"},
		{"plain http with another host", "github.com", []byte("PUT / HTTP/1.1\r\nHost: evil.example\r\n\r\n"), "handshake record"},
		{"ssh", "github.com", []byte("SSH-2.0-OpenSSH_9.6\r\n"), "handshake record"},
	}
}

func TestEgressObserveRelaysAndRecordsWouldRefuse(t *testing.T) {
	for _, tc := range observeCases(t) {
		t.Run(tc.name, func(t *testing.T) {
			h := newEgressHarness(t, withMode(EcosystemEgressPolicy(), TunnelIdentityObserve))
			h.resolved[tc.host] = []netip.Addr{publicAddr}
			conn, br := connectNoRead(t, h, tc.host+":443")
			if _, err := conn.Write(tc.payload); err != nil {
				t.Fatal(err)
			}
			echo := make([]byte, len(tc.payload))
			_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
			if _, err := io.ReadFull(br, echo); err != nil || string(echo) != string(tc.payload) {
				t.Fatalf("observe did not relay the first record unchanged: %q, %v", echo, err)
			}
			// The tunnel stays open after the first record.
			if _, err := io.WriteString(conn, "more"); err != nil {
				t.Fatal(err)
			}
			more := make([]byte, 4)
			if _, err := io.ReadFull(br, more); err != nil || string(more) != "more" {
				t.Fatalf("observe did not keep the tunnel open: %q, %v", more, err)
			}
			conn.Close()
			records := h.waitRecords(1)
			if len(records) != 1 {
				t.Fatalf("records = %+v", records)
			}
			r := records[0]
			if r.Outcome != EgressOutcomeOK || r.Reason != "" || r.TunnelIdentity != TunnelVerdictWouldRefuse ||
				!strings.Contains(r.TunnelIdentityReason, tc.reason) || r.BytesUp < int64(len(tc.payload)) {
				t.Fatalf("record = %+v, want outcome ok and would_refuse naming %q", r, tc.reason)
			}
		})
	}
}

// Observe and enforce judge the same: for every observe case, enforce
// refuses with the same reason observe recorded.
func TestEgressEnforceRefusesWhatObserveWouldRefuse(t *testing.T) {
	for _, tc := range observeCases(t) {
		t.Run(tc.name, func(t *testing.T) {
			h := newEgressHarness(t, enforced(EcosystemEgressPolicy()))
			h.resolved[tc.host] = []netip.Addr{publicAddr}
			conn, br := connectNoRead(t, h, tc.host+":443")
			if _, err := conn.Write(tc.payload); err != nil {
				t.Fatal(err)
			}
			expectTunnelClosedWithoutEcho(t, conn, br)
			records := h.waitRecords(1)
			if len(records) != 1 {
				t.Fatalf("records = %+v", records)
			}
			r := records[0]
			if r.Outcome != EgressOutcomeRefused || r.TunnelIdentity != TunnelVerdictRefused ||
				!strings.Contains(r.Reason, tc.reason) || r.TunnelIdentityReason != r.Reason || r.BytesUp != 0 {
				t.Fatalf("record = %+v, want refused naming %q", r, tc.reason)
			}
		})
	}
}

func TestEgressObserveRecordsPassForARealHello(t *testing.T) {
	h := newEgressHarness(t, withMode(EcosystemEgressPolicy(), TunnelIdentityObserve))
	h.resolved["pypi.org"] = []netip.Addr{publicAddr}
	conn, br := connectNoRead(t, h, "pypi.org:443")
	hello := realClientHello(t, "pypi.org")
	if _, err := conn.Write(hello); err != nil {
		t.Fatal(err)
	}
	echo := make([]byte, len(hello))
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err := io.ReadFull(br, echo); err != nil || string(echo) != string(hello) {
		t.Fatalf("hello did not reach the upstream intact: %v", err)
	}
	conn.Close()
	if r := h.waitRecords(1); len(r) != 1 || r[0].Outcome != EgressOutcomeOK || r[0].TunnelIdentity != TunnelVerdictPass || r[0].TunnelIdentityReason != "" {
		t.Fatalf("records = %+v", r)
	}
}

// A silent client is judged after the hello timeout and then relayed.
func TestEgressObserveRelaysASilentTunnelAfterTheTimeout(t *testing.T) {
	h := newEgressHarness(t, withMode(EcosystemEgressPolicy(), TunnelIdentityObserve))
	h.proxy.helloTimeout = 100 * time.Millisecond
	h.resolved["github.com"] = []netip.Addr{publicAddr}
	conn, br := connectNoRead(t, h, "github.com:443")
	time.Sleep(300 * time.Millisecond)
	hello := realClientHello(t, "github.com")
	if _, err := conn.Write(hello); err != nil {
		t.Fatal(err)
	}
	echo := make([]byte, len(hello))
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err := io.ReadFull(br, echo); err != nil || string(echo) != string(hello) {
		t.Fatalf("observe closed or altered a slow tunnel: %v", err)
	}
	conn.Close()
	if r := h.waitRecords(1); len(r) != 1 || r[0].Outcome != EgressOutcomeOK || r[0].TunnelIdentity != TunnelVerdictWouldRefuse ||
		!strings.Contains(r[0].TunnelIdentityReason, "ClientHello") {
		t.Fatalf("records = %+v", r)
	}
}

// The zero mode on a policy is observe, the default.
func TestEgressUnsetModeIsObserve(t *testing.T) {
	h := newEgressHarness(t, EcosystemEgressPolicy())
	h.resolved["github.com"] = []netip.Addr{publicAddr}
	conn, br := connectNoRead(t, h, "github.com:443")
	hello := realClientHello(t, "pypi.org")
	if _, err := conn.Write(hello); err != nil {
		t.Fatal(err)
	}
	echo := make([]byte, len(hello))
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err := io.ReadFull(br, echo); err != nil {
		t.Fatalf("the default mode refused: %v", err)
	}
	conn.Close()
	if r := h.waitRecords(1); len(r) != 1 || r[0].TunnelIdentity != TunnelVerdictWouldRefuse {
		t.Fatalf("records = %+v", r)
	}
}

func TestEgressOffNeitherJudgesNorRecords(t *testing.T) {
	h := newEgressHarness(t, withMode(EcosystemEgressPolicy(), TunnelIdentityOff))
	h.proxy.helloTimeout = 50 * time.Millisecond
	h.resolved["github.com"] = []netip.Addr{publicAddr}
	conn, br := connectNoRead(t, h, "github.com:443")
	if _, err := io.WriteString(conn, "SSH-2.0-OpenSSH_9.6\r\n"); err != nil {
		t.Fatal(err)
	}
	echo := make([]byte, len("SSH-2.0-OpenSSH_9.6\r\n"))
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err := io.ReadFull(br, echo); err != nil {
		t.Fatalf("off refused: %v", err)
	}
	conn.Close()
	if r := h.waitRecords(1); len(r) != 1 || r[0].Outcome != EgressOutcomeOK || r[0].TunnelIdentity != "" || r[0].TunnelIdentityReason != "" {
		t.Fatalf("records = %+v", r)
	}
}

// Observe logs a would-refuse once per capsule and host, not per connection.
func TestEgressObserveLogsOncePerCapsuleAndHost(t *testing.T) {
	h := newEgressHarness(t, withMode(EcosystemEgressPolicy(), TunnelIdentityObserve))
	var mu sync.Mutex
	var lines []string
	h.proxy.logf = func(format string, args ...any) {
		mu.Lock()
		defer mu.Unlock()
		lines = append(lines, fmt.Sprintf(format, args...))
	}
	h.resolved["github.com"] = []netip.Addr{publicAddr}
	h.resolved["pypi.org"] = []netip.Addr{publicAddr}
	tunnel := func(host, sni string) {
		conn, br := connectNoRead(t, h, host+":443")
		hello := realClientHello(t, sni)
		if _, err := conn.Write(hello); err != nil {
			t.Fatal(err)
		}
		echo := make([]byte, len(hello))
		_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		if _, err := io.ReadFull(br, echo); err != nil {
			t.Fatal(err)
		}
		conn.Close()
	}
	tunnel("github.com", "pypi.org")
	tunnel("github.com", "example.com")
	tunnel("github.com", "github.com") // a pass is not logged
	tunnel("pypi.org", "github.com")
	tunnel("pypi.org", "example.com")
	h.waitRecords(5)
	mu.Lock()
	defer mu.Unlock()
	if len(lines) != 2 {
		t.Fatalf("would-refuse log lines = %q, want one for github.com and one for pypi.org", lines)
	}
	for i, host := range []string{"github.com", "pypi.org"} {
		if !strings.Contains(lines[i], "capsule-test") || !strings.Contains(lines[i], host) || !strings.Contains(lines[i], "would refuse") {
			t.Errorf("log line %d = %q, want capsule, host %s and the verdict", i, lines[i], host)
		}
	}
}
