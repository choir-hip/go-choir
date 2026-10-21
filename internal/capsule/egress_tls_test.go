package capsule

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"testing"
	"time"
)

// docs/problems/egress-proxy-tunnel-identity-2026-10-21.md: the egress proxy
// admits a CONNECT by the host name in the request and then relays opaque
// bytes, so the allowlist limited which front doors a capsule could knock on,
// not which service it reached through them. The fix binds the tunnel to the
// name: the first thing a tunnel carries must be a TLS ClientHello whose
// server_name is the host the CONNECT named, and everything else is refused.
//
// Ways it could fail, pinned here:
//   - an allowlisted CONNECT host carries a ClientHello for a different name
//     (domain fronting through a shared front end);
//   - the tunnel carries no TLS at all (plain HTTP with another Host header,
//     SSH on 443, any other protocol);
//   - the ClientHello names no server (no SNI), or names it twice, or in a
//     non-host form;
//   - the ClientHello hides the name (Encrypted Client Hello), so the proxy
//     cannot see what the tunnel is for;
//   - the hello is fragmented, truncated, oversized or malformed in a way a
//     lenient parser would read differently from the destination;
//   - a silent client holds a tunnel open forever;
//   - the check dials or forwards a byte before it has passed;
//   - a real TLS client's hello (crypto/tls) is refused, which would break
//     every legitimate build.

// realClientHello captures the first TLS record a crypto/tls client sends
// for serverName, with no network.
func realClientHello(t *testing.T, serverName string) []byte {
	t.Helper()
	client, server := net.Pipe()
	defer server.Close()
	go func() {
		defer client.Close()
		_ = tls.Client(client, &tls.Config{ServerName: serverName, MinVersion: tls.VersionTLS12}).Handshake()
	}()
	_ = server.SetDeadline(time.Now().Add(3 * time.Second))
	header := make([]byte, 5)
	if _, err := io.ReadFull(server, header); err != nil {
		t.Fatalf("read hello header: %v", err)
	}
	body := make([]byte, int(header[3])<<8|int(header[4]))
	if _, err := io.ReadFull(server, body); err != nil {
		t.Fatalf("read hello body: %v", err)
	}
	return append(header, body...)
}

type helloOpts struct {
	names         []string // host_name entries in the server_name extension
	noSNI         bool
	nameType      byte // 0 (host_name) unless set non-zero via nonHostType
	nonHostType   bool
	ech           bool // encrypted_client_hello (0xfe0d)
	echDraft      bool // encrypted_server_name draft (0xffce)
	recordType    byte // 0x16 unless set
	handshakeType byte // 1 (client_hello) unless set
	recordVersion []byte
	extraBytes    int // bytes the extension block claims beyond what follows
	fragmentTail  int // bytes the handshake claims beyond this record
	rawName       []byte
}

func u16(n int) []byte { return []byte{byte(n >> 8), byte(n)} }

func handBuiltHello(o helloOpts) []byte {
	var ext bytes.Buffer
	if !o.noSNI {
		var list bytes.Buffer
		names := o.names
		if o.rawName != nil {
			names = []string{string(o.rawName)}
		}
		for _, name := range names {
			typ := byte(0)
			if o.nonHostType {
				typ = o.nameType
			}
			list.WriteByte(typ)
			list.Write(u16(len(name)))
			list.WriteString(name)
		}
		ext.Write(u16(0x0000))
		ext.Write(u16(list.Len() + 2))
		ext.Write(u16(list.Len()))
		ext.Write(list.Bytes())
	}
	// A supported_versions extension, so the block is not just the name.
	ext.Write(u16(0x002b))
	ext.Write(u16(3))
	ext.Write([]byte{2, 0x03, 0x04})
	if o.ech {
		ext.Write(u16(0xfe0d))
		ext.Write(u16(4))
		ext.Write([]byte{0, 0, 0, 0})
	}
	if o.echDraft {
		ext.Write(u16(0xffce))
		ext.Write(u16(2))
		ext.Write([]byte{0, 0})
	}

	var body bytes.Buffer
	body.Write([]byte{0x03, 0x03})
	body.Write(make([]byte, 32))
	body.WriteByte(0) // session id
	body.Write(u16(2))
	body.Write([]byte{0x13, 0x01})
	body.WriteByte(1)
	body.WriteByte(0)
	body.Write(u16(ext.Len() + o.extraBytes))
	body.Write(ext.Bytes())

	hsType := byte(1)
	if o.handshakeType != 0 {
		hsType = o.handshakeType
	}
	var hs bytes.Buffer
	hs.WriteByte(hsType)
	n := body.Len() + o.fragmentTail
	hs.Write([]byte{byte(n >> 16), byte(n >> 8), byte(n)})
	hs.Write(body.Bytes())

	recType := byte(0x16)
	if o.recordType != 0 {
		recType = o.recordType
	}
	version := []byte{0x03, 0x01}
	if o.recordVersion != nil {
		version = o.recordVersion
	}
	var rec bytes.Buffer
	rec.WriteByte(recType)
	rec.Write(version)
	rec.Write(u16(hs.Len()))
	rec.Write(hs.Bytes())
	return rec.Bytes()
}

func TestVerifyTunnelClientHelloAcceptsARealTLSClient(t *testing.T) {
	hello := realClientHello(t, "files.pythonhosted.org")
	for _, host := range []string{"files.pythonhosted.org", "FILES.PythonHosted.ORG", "files.pythonhosted.org."} {
		if err := verifyTunnelClientHello(hello, host); err != nil {
			t.Errorf("real hello for files.pythonhosted.org refused for CONNECT host %q: %v", host, err)
		}
	}
}

func TestVerifyTunnelClientHelloRefusesAnotherName(t *testing.T) {
	hello := realClientHello(t, "pypi.org")
	for _, host := range []string{"github.com", "evilpypi.org", "pypi.org.evil.com", "files.pypi.org"} {
		err := verifyTunnelClientHello(hello, host)
		if err == nil {
			t.Errorf("hello for pypi.org accepted for CONNECT host %q", host)
			continue
		}
		if !strings.Contains(err.Error(), "SNI") {
			t.Errorf("refusal for %q does not say why: %v", host, err)
		}
	}
}

func TestVerifyTunnelClientHelloTable(t *testing.T) {
	good := handBuiltHello(helloOpts{names: []string{"pypi.org"}})
	if err := verifyTunnelClientHello(good, "pypi.org"); err != nil {
		t.Fatalf("hand-built hello for pypi.org refused: %v", err)
	}
	cases := []struct {
		name   string
		record []byte
		host   string
	}{
		{"no sni", handBuiltHello(helloOpts{noSNI: true}), "pypi.org"},
		{"empty sni", handBuiltHello(helloOpts{names: []string{""}}), "pypi.org"},
		{"two host names, first matches", handBuiltHello(helloOpts{names: []string{"pypi.org", "github.com"}}), "pypi.org"},
		{"two host names, second matches", handBuiltHello(helloOpts{names: []string{"github.com", "pypi.org"}}), "pypi.org"},
		{"name type is not host_name", handBuiltHello(helloOpts{names: []string{"pypi.org"}, nonHostType: true, nameType: 1}), "pypi.org"},
		{"sni with a nul byte", handBuiltHello(helloOpts{rawName: []byte("pypi.org\x00.evil.com")}), "pypi.org"},
		{"sni with a space", handBuiltHello(helloOpts{rawName: []byte("pypi.org .evil.com")}), "pypi.org"},
		{"sni with a non-ascii byte", handBuiltHello(helloOpts{rawName: []byte("pypi.org\xff")}), "pypi.org"},
		{"sni is an ip literal", handBuiltHello(helloOpts{names: []string{"151.101.64.223"}}), "pypi.org"},
		{"encrypted client hello", handBuiltHello(helloOpts{names: []string{"pypi.org"}, ech: true}), "pypi.org"},
		{"encrypted server name draft", handBuiltHello(helloOpts{names: []string{"pypi.org"}, echDraft: true}), "pypi.org"},
		{"first record is application data", handBuiltHello(helloOpts{names: []string{"pypi.org"}, recordType: 0x17}), "pypi.org"},
		{"first record is an alert", handBuiltHello(helloOpts{names: []string{"pypi.org"}, recordType: 0x15}), "pypi.org"},
		{"handshake is a server hello", handBuiltHello(helloOpts{names: []string{"pypi.org"}, handshakeType: 2}), "pypi.org"},
		{"record version zero", handBuiltHello(helloOpts{names: []string{"pypi.org"}, recordVersion: []byte{0, 0}}), "pypi.org"},
		{"record version ssl2", handBuiltHello(helloOpts{names: []string{"pypi.org"}, recordVersion: []byte{0x02, 0x00}}), "pypi.org"},
		{"hello continues in a later record", handBuiltHello(helloOpts{names: []string{"pypi.org"}, fragmentTail: 64}), "pypi.org"},
		{"extension block longer than the hello", handBuiltHello(helloOpts{names: []string{"pypi.org"}, extraBytes: 64}), "pypi.org"},
		{"plain http", []byte("GET / HTTP/1.1\r\nHost: pypi.org\r\n\r\n"), "pypi.org"},
		{"ssh banner", []byte("SSH-2.0-OpenSSH_9.6\r\n"), "pypi.org"},
		{"empty", nil, "pypi.org"},
		{"header only", []byte{0x16, 0x03, 0x01, 0x00, 0x10}, "pypi.org"},
		{"truncated mid hello", good[:len(good)-5], "pypi.org"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := verifyTunnelClientHello(tc.record, tc.host); err == nil {
				t.Fatalf("hello accepted: %x", tc.record)
			}
		})
	}
}

// Trailing bytes after the record belong to the next record or to the stream;
// the verifier is handed exactly one record and must not read past it.
func TestVerifyTunnelClientHelloIgnoresNothingInsideTheRecord(t *testing.T) {
	good := handBuiltHello(helloOpts{names: []string{"pypi.org"}})
	if err := verifyTunnelClientHello(append(append([]byte{}, good...), 0xde, 0xad), "pypi.org"); err == nil {
		t.Fatal("a record with bytes after its declared length was accepted as one record")
	}
}

// ---------------------------------------------------------------------------
// Through the proxy.

func connectNoRead(t *testing.T, h *egressHarness, target string) (net.Conn, *bufio.Reader) {
	t.Helper()
	conn, br, resp := h.connect(target)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("CONNECT %s status = %d", target, resp.StatusCode)
	}
	return conn, br
}

func expectTunnelClosedWithoutEcho(t *testing.T, conn net.Conn, br *bufio.Reader) {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	n, err := io.Copy(io.Discard, br)
	if n != 0 {
		t.Fatalf("a refused tunnel still carried %d bytes back from the upstream", n)
	}
	if ne, ok := err.(net.Error); ok && ne.Timeout() {
		t.Fatal("a refused tunnel was left open (read timed out instead of closing)")
	}
}

func TestEgressForwardsARealClientHelloUnchanged(t *testing.T) {
	h := newEgressHarness(t, EcosystemEgressPolicy())
	h.resolved["pypi.org"] = []netip.Addr{publicAddr}
	conn, br := connectNoRead(t, h, "pypi.org:443")
	hello := realClientHello(t, "pypi.org")
	if _, err := conn.Write(hello); err != nil {
		t.Fatal(err)
	}
	echo := make([]byte, len(hello))
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err := io.ReadFull(br, echo); err != nil || !bytes.Equal(echo, hello) {
		t.Fatalf("hello did not reach the upstream intact: %v", err)
	}
	conn.Close()
	records := h.waitRecords(1)
	if len(records) != 1 || records[0].Outcome != EgressOutcomeOK {
		t.Fatalf("records = %+v", records)
	}
}

func TestEgressRefusesATunnelWhoseServerNameIsNotTheConnectHost(t *testing.T) {
	h := newEgressHarness(t, EcosystemEgressPolicy())
	// Both names are on the allowlist: the point is that naming one in the
	// CONNECT does not buy the other.
	h.resolved["github.com"] = []netip.Addr{publicAddr}
	conn, br := connectNoRead(t, h, "github.com:443")
	if _, err := conn.Write(realClientHello(t, "pypi.org")); err != nil {
		t.Fatal(err)
	}
	expectTunnelClosedWithoutEcho(t, conn, br)
	records := h.waitRecords(1)
	if len(records) != 1 || records[0].Outcome != EgressOutcomeRefused || !strings.Contains(records[0].Reason, "SNI") ||
		records[0].Host != "github.com" {
		t.Fatalf("records = %+v", records)
	}
}

func TestEgressRefusesATunnelThatIsNotTLS(t *testing.T) {
	for name, payload := range map[string]string{
		"plain http with another host": "GET / HTTP/1.1\r\nHost: evil.example\r\n\r\n",
		"ssh":                          "SSH-2.0-OpenSSH_9.6\r\n",
		"random bytes":                 "\x00\x01\x02\x03\x04\x05\x06\x07",
	} {
		t.Run(name, func(t *testing.T) {
			h := newEgressHarness(t, EcosystemEgressPolicy())
			h.resolved["github.com"] = []netip.Addr{publicAddr}
			conn, br := connectNoRead(t, h, "github.com:443")
			if _, err := io.WriteString(conn, payload); err != nil {
				t.Fatal(err)
			}
			expectTunnelClosedWithoutEcho(t, conn, br)
			records := h.waitRecords(1)
			if len(records) != 1 || records[0].Outcome != EgressOutcomeRefused || records[0].Reason == "" {
				t.Fatalf("records = %+v", records)
			}
		})
	}
}

func TestEgressRefusesAHelloThatHidesItsName(t *testing.T) {
	h := newEgressHarness(t, EcosystemEgressPolicy())
	h.resolved["github.com"] = []netip.Addr{publicAddr}
	conn, br := connectNoRead(t, h, "github.com:443")
	if _, err := conn.Write(handBuiltHello(helloOpts{names: []string{"github.com"}, ech: true})); err != nil {
		t.Fatal(err)
	}
	expectTunnelClosedWithoutEcho(t, conn, br)
	records := h.waitRecords(1)
	if len(records) != 1 || records[0].Outcome != EgressOutcomeRefused {
		t.Fatalf("records = %+v", records)
	}
}

func TestEgressRefusesASilentTunnel(t *testing.T) {
	h := newEgressHarness(t, EcosystemEgressPolicy())
	h.proxy.helloTimeout = 150 * time.Millisecond
	h.resolved["github.com"] = []netip.Addr{publicAddr}
	conn, br := connectNoRead(t, h, "github.com:443")
	expectTunnelClosedWithoutEcho(t, conn, br)
	records := h.waitRecords(1)
	if len(records) != 1 || records[0].Outcome != EgressOutcomeRefused || !strings.Contains(records[0].Reason, "ClientHello") {
		t.Fatalf("records = %+v", records)
	}
}

func TestEgressRefusesAnOversizedFirstRecord(t *testing.T) {
	h := newEgressHarness(t, EcosystemEgressPolicy())
	h.resolved["github.com"] = []netip.Addr{publicAddr}
	conn, br := connectNoRead(t, h, "github.com:443")
	// A record header that claims more than a TLS record may carry.
	if _, err := conn.Write([]byte{0x16, 0x03, 0x01, 0xff, 0xff}); err != nil {
		t.Fatal(err)
	}
	expectTunnelClosedWithoutEcho(t, conn, br)
	records := h.waitRecords(1)
	if len(records) != 1 || records[0].Outcome != EgressOutcomeRefused {
		t.Fatalf("records = %+v", records)
	}
}

// The name check applies to the Claude delegate's policy as well.
func TestEgressClaudeTunnelIsBoundToItsName(t *testing.T) {
	h := newEgressHarness(t, ClaudeEgressPolicy())
	h.resolved["api.anthropic.com"] = []netip.Addr{publicAddr}
	conn, br := connectNoRead(t, h, "api.anthropic.com:443")
	if _, err := conn.Write(realClientHello(t, "example.com")); err != nil {
		t.Fatal(err)
	}
	expectTunnelClosedWithoutEcho(t, conn, br)
}
