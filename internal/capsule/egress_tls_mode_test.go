package capsule

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"os"
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
// capsule and host, and relays the record's bytes and the tunnel unchanged.
//
// Observe does change one thing a client sees: timing. Nothing is relayed in
// either direction until the first record is judged, and the check waits up
// to the hello timeout (10 s) for a complete first record. A TLS client
// speaks first and is judged as soon as its hello arrives; a client that
// stays silent, or one waiting for a server that speaks first, is relayed
// only after that wait. TestEgressObserveDelaysAServerFirstGreetingUntilTheTimeout
// pins it.
//
// Ways it could fail, pinned here:
//   - observe judges differently from enforce (a case enforce refuses that
//     observe passes, so the ledger's zero would-refuse count proves nothing);
//   - observe drops, alters or reorders the first record, or closes the
//     tunnel; or holds it longer than the hello timeout;
//   - observe records nothing, so the flip to enforce has no evidence;
//   - observe repeats its once-per-host warning on every connection, or
//     never logs it (every connection's record is logged separately);
//   - an unset or unknown mode is enforce (a typo breaks every build) or off
//     (a typo removes the evidence), or an unknown mode passes silently;
//   - the mode can be chosen from inside a capsule (it is the executor's,
//     read from the daemon's environment, set on the policy by the executor);
//   - off still reads or judges the first record;
//   - a valid hello larger than the request reader's buffer (4 KiB) is judged
//     incomplete, in either mode, whether it follows the CONNECT in its own
//     write or in the same one;
//   - an enforce refusal sends any byte to the upstream (counted upstream).

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
			expectNothingReachedUpstream(t, h)
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

// A client that sends some of its first record and the rest after the hello
// timeout has not sent a complete first record in time: observe records it
// and relays everything; enforce would refuse it.
func TestEgressObserveRecordsAnIncompleteFirstRecordAndRelaysIt(t *testing.T) {
	h := newEgressHarness(t, withMode(EcosystemEgressPolicy(), TunnelIdentityObserve))
	h.proxy.helloTimeout = 100 * time.Millisecond
	h.resolved["github.com"] = []netip.Addr{publicAddr}
	conn, br := connectNoRead(t, h, "github.com:443")
	hello := realClientHello(t, "github.com")
	if _, err := conn.Write(hello[:10]); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	if _, err := conn.Write(hello[10:]); err != nil {
		t.Fatal(err)
	}
	echo := make([]byte, len(hello))
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err := io.ReadFull(br, echo); err != nil || string(echo) != string(hello) {
		t.Fatalf("observe did not relay a slow hello intact: %v", err)
	}
	conn.Close()
	if r := h.waitRecords(1); len(r) != 1 || r[0].Outcome != EgressOutcomeOK || r[0].TunnelIdentity != TunnelVerdictWouldRefuse ||
		!strings.Contains(r[0].TunnelIdentityReason, "incomplete") {
		t.Fatalf("records = %+v", r)
	}
}

// paddedHello is a valid hello for name whose record payload is exactly
// payload bytes (at most a TLS record, 16384).
func paddedHello(t *testing.T, name string, payload int) []byte {
	t.Helper()
	base := len(handBuiltHello(helloOpts{names: []string{name}})) - 5
	pad := payload - base - 4 // the padding extension's own header
	if pad < 0 {
		t.Fatalf("payload %d is below the unpadded hello's %d", payload, base)
	}
	rec := handBuiltHello(helloOpts{names: []string{name}, padding: pad})
	if len(rec)-5 != payload {
		t.Fatalf("padded hello payload = %d, want %d", len(rec)-5, payload)
	}
	if err := verifyTunnelClientHello(rec, name); err != nil {
		t.Fatalf("padded hello refused by the verifier: %v", err)
	}
	return rec
}

// A valid hello larger than 4 KiB, up to the largest record TLS allows,
// passes through the proxy in both modes, sent after the 200 or pipelined
// behind the CONNECT header.
func TestEgressLargeHellosPassInBothModes(t *testing.T) {
	for _, mode := range []TunnelIdentityMode{TunnelIdentityObserve, TunnelIdentityEnforce} {
		for _, size := range []int{9 * 1024, maxTunnelRecord} {
			for _, pipelined := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%d/pipelined=%t", mode, size, pipelined), func(t *testing.T) {
					h := newEgressHarness(t, withMode(EcosystemEgressPolicy(), mode))
					h.resolved["pypi.org"] = []netip.Addr{publicAddr}
					hello := paddedHello(t, "pypi.org", size)
					var conn net.Conn
					var br *bufio.Reader
					if pipelined {
						c, err := net.Dial("tcp", h.addr)
						if err != nil {
							t.Fatal(err)
						}
						t.Cleanup(func() { c.Close() })
						if _, err := c.Write(append([]byte("CONNECT pypi.org:443 HTTP/1.1\r\nHost: pypi.org:443\r\n\r\n"), hello...)); err != nil {
							t.Fatal(err)
						}
						conn, br = c, bufio.NewReader(c)
						resp, err := http.ReadResponse(br, nil)
						if err != nil || resp.StatusCode != http.StatusOK {
							t.Fatalf("CONNECT: %v %v", resp, err)
						}
					} else {
						conn, br = connectNoRead(t, h, "pypi.org:443")
						if _, err := conn.Write(hello); err != nil {
							t.Fatal(err)
						}
					}
					echo := make([]byte, len(hello))
					_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
					if _, err := io.ReadFull(br, echo); err != nil || !bytes.Equal(echo, hello) {
						t.Fatalf("a %d-byte hello did not pass intact: %v", size, err)
					}
					conn.Close()
					if r := h.waitRecords(1); len(r) != 1 || r[0].Outcome != EgressOutcomeOK || r[0].TunnelIdentity != TunnelVerdictPass {
						t.Fatalf("records = %+v", r)
					}
				})
			}
		}
	}
}

// Observe's one visible effect, pinned: a server that speaks first is not
// heard by a silent client until the hello timeout has passed; then the
// greeting arrives intact and the tunnel is recorded would_refuse.
func TestEgressObserveDelaysAServerFirstGreetingUntilTheTimeout(t *testing.T) {
	const timeout = 200 * time.Millisecond
	h := newEgressHarness(t, withMode(EcosystemEgressPolicy(), TunnelIdentityObserve))
	h.proxy.helloTimeout = timeout
	greeter, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { greeter.Close() })
	go func() {
		for {
			c, err := greeter.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				_, _ = io.WriteString(c, "SSH-2.0-greeting\r\n")
				_, _ = io.Copy(io.Discard, c)
			}(c)
		}
	}()
	h.proxy.dial = func(ctx context.Context, network, _ string) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, network, greeter.Addr().String())
	}
	h.resolved["github.com"] = []netip.Addr{publicAddr}
	conn, br := connectNoRead(t, h, "github.com:443")
	start := time.Now()
	greeting := make([]byte, len("SSH-2.0-greeting\r\n"))
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err := io.ReadFull(br, greeting); err != nil || string(greeting) != "SSH-2.0-greeting\r\n" {
		t.Fatalf("greeting = %q, %v", greeting, err)
	}
	if elapsed := time.Since(start); elapsed < timeout*9/10 || elapsed > timeout+2*time.Second {
		t.Fatalf("greeting arrived after %v; observe holds it for the hello timeout (%v)", elapsed, timeout)
	}
	conn.Close()
	if r := h.waitRecords(1); len(r) != 1 || r[0].TunnelIdentity != TunnelVerdictWouldRefuse || !strings.Contains(r[0].TunnelIdentityReason, "no TLS ClientHello") {
		t.Fatalf("records = %+v", r)
	}
}

// A set value that is not one of the three is observe, and says so: a typo in
// the flip must not read like success at boot.
func TestTunnelIdentityEnvWarning(t *testing.T) {
	env := func(v string) func(string) string { return func(string) string { return v } }
	for _, ok := range []string{"", "  ", "off", "observe", " Enforce "} {
		if w := TunnelIdentityEnvWarning(env(ok)); w != "" {
			t.Errorf("%q warned: %s", ok, w)
		}
	}
	for _, bad := range []string{"enforse", "enable", "1", "true"} {
		w := TunnelIdentityEnvWarning(env(bad))
		if !strings.Contains(w, TunnelIdentityEnv) || !strings.Contains(w, bad) || !strings.Contains(w, "observe") {
			t.Errorf("%q: warning %q, want the variable, the value and the mode used", bad, w)
		}
	}
}

// The guest image names the mode, observe, rather than relying on the
// default; and a computer the host selects (vmctl's
// VM_CAPSULE_TUNNEL_ENFORCE_ONLY, a kernel boot argument) gets enforce
// through the daemon's environment file, which overrides the unit's
// environment. Only the exact value enforce is copied, so the boot argument
// can turn enforce on and nothing else.
func TestGuestImageNamesTheTunnelIdentityModeAndTheHostSelection(t *testing.T) {
	raw, err := os.ReadFile("../../nix/autoputer-vm.nix")
	if err != nil {
		t.Skipf("nix/autoputer-vm.nix not readable from the package: %v", err)
	}
	img := string(raw)
	if !strings.Contains(img, `CHOIR_CAPSULE_EGRESS_TUNNEL_IDENTITY = "observe";`) {
		t.Fatal(`the guest image does not set CHOIR_CAPSULE_EGRESS_TUNNEL_IDENTITY = "observe"`)
	}
	if !strings.Contains(img, `choir.capsule_egress_tunnel_identity=enforce)`) ||
		!strings.Contains(img, `echo "CHOIR_CAPSULE_EGRESS_TUNNEL_IDENTITY=enforce" >> "$ENV_FILE"`) {
		t.Fatal("the guest boot script does not copy the host's enforce selection into the daemon's environment file")
	}
	if strings.Contains(img, `choir.capsule_egress_tunnel_identity=*)`) {
		t.Fatal("the guest boot script copies any value of the selection, not only enforce")
	}
}
