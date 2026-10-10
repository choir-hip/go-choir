package capsule

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// docs/design/engineering-network-grants-2026-10-10.md §4.0b: the L2 egress
// proxy is the only way out of an ecosystem_proxy capsule. Failure modes
// pinned here:
//   - a host off the ecosystem allowlist is dialed;
//   - an allowlisted name that resolves to a private, loopback, link-local,
//     CGNAT or NAT64-wrapped private address is dialed (DNS rebinding);
//   - the proxy dials the hostname instead of the address it checked;
//   - CONNECT reaches a port other than 443, or plain HTTP reaches a port
//     other than 80 or uses an upload-shaped method;
//   - suffix matching admits "evilpypi.org" or "pypi.org.evil.com";
//   - bytes the client sent right behind the CONNECT header are dropped;
//   - the upload budget (the exfiltration bound) or the connection budget
//     is not enforced;
//   - a connection, refused or not, leaves no record;
//   - Close leaves a live tunnel behind.

type egressHarness struct {
	t        *testing.T
	proxy    *EgressProxy
	addr     string
	resolved map[string][]netip.Addr
	upstream net.Listener

	mu      sync.Mutex
	dialed  []string
	records []EgressRecord
}

func newEgressHarness(t *testing.T, policy EgressPolicy) *egressHarness {
	t.Helper()
	h := &egressHarness{t: t, resolved: map[string][]netip.Addr{}}
	upstream, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	h.upstream = upstream
	go serveEchoUpstream(upstream)
	h.proxy = NewEgressProxy("capsule-test", policy, func(r EgressRecord) {
		h.mu.Lock()
		h.records = append(h.records, r)
		h.mu.Unlock()
	})
	h.proxy.resolve = func(_ context.Context, host string) ([]netip.Addr, error) {
		addrs, ok := h.resolved[host]
		if !ok {
			return nil, fmt.Errorf("no such host %s", host)
		}
		return addrs, nil
	}
	// Every dial lands on the local upstream, but the proxy must ask for the
	// checked address, which the harness records.
	h.proxy.dial = func(ctx context.Context, network, address string) (net.Conn, error) {
		h.mu.Lock()
		h.dialed = append(h.dialed, address)
		h.mu.Unlock()
		var d net.Dialer
		return d.DialContext(ctx, network, upstream.Addr().String())
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	h.addr = listener.Addr().String()
	go h.proxy.Serve(listener)
	t.Cleanup(func() { h.proxy.Close(); upstream.Close() })
	return h
}

// serveEchoUpstream answers a plain HTTP request with a fixed body, and
// otherwise echoes bytes back (standing in for a TLS server).
func serveEchoUpstream(l net.Listener) {
	for {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		go func(c net.Conn) {
			defer c.Close()
			br := bufio.NewReader(c)
			peek, _ := br.Peek(4)
			if string(peek) == "GET " || string(peek) == "HEAD" {
				req, err := http.ReadRequest(br)
				if err != nil {
					return
				}
				body := "upstream saw " + req.Method + " " + req.URL.RequestURI() + " host=" + req.Host
				fmt.Fprintf(c, "HTTP/1.1 200 OK\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s", len(body), body)
				return
			}
			io.Copy(c, br)
		}(conn)
	}
}

func (h *egressHarness) connect(target string) (net.Conn, *bufio.Reader, *http.Response) {
	h.t.Helper()
	conn, err := net.Dial("tcp", h.addr)
	if err != nil {
		h.t.Fatal(err)
	}
	h.t.Cleanup(func() { conn.Close() })
	fmt.Fprintf(conn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", target, target)
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, nil)
	if err != nil {
		h.t.Fatalf("CONNECT %s: %v", target, err)
	}
	return conn, br, resp
}

func (h *egressHarness) waitRecords(n int) []EgressRecord {
	h.t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		h.mu.Lock()
		got := append([]EgressRecord(nil), h.records...)
		h.mu.Unlock()
		if len(got) >= n || time.Now().After(deadline) {
			return got
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func (h *egressHarness) dials() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.dialed...)
}

var publicAddr = netip.MustParseAddr("151.101.64.223")

func TestEgressTunnelsAllowlistedHostToTheCheckedAddress(t *testing.T) {
	h := newEgressHarness(t, EcosystemEgressPolicy())
	h.resolved["files.pythonhosted.org"] = []netip.Addr{publicAddr}
	conn, br, resp := h.connect("files.pythonhosted.org:443")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("CONNECT status = %d", resp.StatusCode)
	}
	if _, err := conn.Write([]byte("client-hello")); err != nil {
		t.Fatal(err)
	}
	echo := make([]byte, len("client-hello"))
	if _, err := io.ReadFull(br, echo); err != nil || string(echo) != "client-hello" {
		t.Fatalf("tunnel echo = %q, %v", echo, err)
	}
	conn.Close()
	records := h.waitRecords(1)
	if len(records) != 1 || records[0].Outcome != EgressOutcomeOK || records[0].Host != "files.pythonhosted.org" ||
		records[0].IP != publicAddr.String() || records[0].BytesUp < int64(len("client-hello")) || records[0].BytesDown < int64(len("client-hello")) {
		t.Fatalf("records = %+v", records)
	}
	if d := h.dials(); len(d) != 1 || d[0] != net.JoinHostPort(publicAddr.String(), "443") {
		t.Fatalf("dialed %v, want the checked address", d)
	}
}

func TestEgressForwardsBytesSentBehindTheConnectHeader(t *testing.T) {
	h := newEgressHarness(t, EcosystemEgressPolicy())
	h.resolved["github.com"] = []netip.Addr{publicAddr}
	conn, err := net.Dial("tcp", h.addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	// A client may pipeline its TLS hello in the same write as CONNECT.
	fmt.Fprintf(conn, "CONNECT github.com:443 HTTP/1.1\r\nHost: github.com:443\r\n\r\neager-bytes")
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, nil)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("CONNECT: %v %v", resp, err)
	}
	echo := make([]byte, len("eager-bytes"))
	if _, err := io.ReadFull(br, echo); err != nil || string(echo) != "eager-bytes" {
		t.Fatalf("eager bytes = %q, %v", echo, err)
	}
}

func TestEgressRefusesWhatL2DoesNotAllow(t *testing.T) {
	cases := []struct {
		name     string
		target   string
		resolved []netip.Addr
	}{
		{"off the allowlist", "example.com:443", []netip.Addr{publicAddr}},
		{"suffix without a dot boundary", "evilpypi.org:443", []netip.Addr{publicAddr}},
		{"allowlisted name as a prefix", "pypi.org.evil.com:443", []netip.Addr{publicAddr}},
		{"ip literal", "151.101.64.223:443", nil},
		{"port other than 443", "github.com:22", []netip.Addr{publicAddr}},
		{"rebinds to loopback", "pypi.org:443", []netip.Addr{netip.MustParseAddr("127.0.0.1")}},
		{"rebinds to private", "pypi.org:443", []netip.Addr{netip.MustParseAddr("10.200.0.1")}},
		{"rebinds to metadata", "pypi.org:443", []netip.Addr{netip.MustParseAddr("169.254.169.254")}},
		{"rebinds to cgnat", "pypi.org:443", []netip.Addr{netip.MustParseAddr("100.64.1.1")}},
		{"rebinds to nat64 private", "pypi.org:443", []netip.Addr{netip.MustParseAddr("64:ff9b::a00:1")}},
		{"one public one private", "pypi.org:443", []netip.Addr{publicAddr, netip.MustParseAddr("192.168.1.1")}},
		{"unresolvable", "pypi.org:443", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newEgressHarness(t, EcosystemEgressPolicy())
			host, _, _ := net.SplitHostPort(tc.target)
			if tc.resolved != nil {
				h.resolved[host] = tc.resolved
			}
			_, _, resp := h.connect(tc.target)
			if resp.StatusCode == http.StatusOK {
				t.Fatalf("CONNECT %s was allowed", tc.target)
			}
			if d := h.dials(); len(d) != 0 {
				t.Fatalf("refused CONNECT still dialed %v", d)
			}
			records := h.waitRecords(1)
			if len(records) != 1 || records[0].Outcome != EgressOutcomeRefused || records[0].Reason == "" {
				t.Fatalf("records = %+v", records)
			}
		})
	}
}

func TestEgressPlainHTTPAllowsOnlyReadsOnPort80(t *testing.T) {
	h := newEgressHarness(t, EcosystemEgressPolicy())
	h.resolved["arxiv.org"] = []netip.Addr{publicAddr}
	send := func(raw string) *http.Response {
		conn, err := net.Dial("tcp", h.addr)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		io.WriteString(conn, raw)
		resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		resp.Header.Set("X-Test-Body", string(body))
		return resp
	}
	ok := send("GET http://arxiv.org/abs/2310.06825 HTTP/1.1\r\nHost: arxiv.org\r\nProxy-Connection: keep-alive\r\n\r\n")
	if ok.StatusCode != http.StatusOK || !strings.Contains(ok.Header.Get("X-Test-Body"), "GET /abs/2310.06825 host=arxiv.org") {
		t.Fatalf("GET via proxy = %d %q", ok.StatusCode, ok.Header.Get("X-Test-Body"))
	}
	if post := send("POST http://arxiv.org/upload HTTP/1.1\r\nHost: arxiv.org\r\nContent-Length: 4\r\n\r\ndata"); post.StatusCode == http.StatusOK {
		t.Fatal("plain-HTTP POST was forwarded")
	}
	if port := send("GET http://arxiv.org:8080/ HTTP/1.1\r\nHost: arxiv.org:8080\r\n\r\n"); port.StatusCode == http.StatusOK {
		t.Fatal("plain HTTP to port 8080 was forwarded")
	}
	if d := h.dials(); len(d) != 1 || d[0] != net.JoinHostPort(publicAddr.String(), "80") {
		t.Fatalf("dialed %v, want one dial to the checked address on 80", d)
	}
}

func TestEgressUploadBudgetCutsTheTunnel(t *testing.T) {
	policy := EcosystemEgressPolicy()
	policy.MaxUploadBytes = 1024
	h := newEgressHarness(t, policy)
	h.resolved["pypi.org"] = []netip.Addr{publicAddr}
	conn, _, resp := h.connect("pypi.org:443")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("CONNECT status = %d", resp.StatusCode)
	}
	payload := strings.Repeat("x", 4096)
	conn.SetDeadline(time.Now().Add(3 * time.Second))
	for i := 0; i < 8; i++ {
		if _, err := io.WriteString(conn, payload); err != nil {
			break
		}
	}
	records := h.waitRecords(1)
	if len(records) != 1 || records[0].Outcome != EgressOutcomeBudget || records[0].BytesUp > policy.MaxUploadBytes {
		t.Fatalf("records = %+v", records)
	}
	// The budget is per capsule: the next connection is refused outright.
	h.resolved["github.com"] = []netip.Addr{publicAddr}
	if _, _, next := h.connect("github.com:443"); next.StatusCode == http.StatusOK {
		t.Fatal("connection after the upload budget was spent was allowed")
	}
}

func TestEgressConnectionBudget(t *testing.T) {
	policy := EcosystemEgressPolicy()
	policy.MaxConnections = 2
	h := newEgressHarness(t, policy)
	h.resolved["pypi.org"] = []netip.Addr{publicAddr}
	for i := 0; i < 2; i++ {
		conn, _, resp := h.connect("pypi.org:443")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("connection %d status = %d", i, resp.StatusCode)
		}
		conn.Close()
	}
	if _, _, resp := h.connect("pypi.org:443"); resp.StatusCode == http.StatusOK {
		t.Fatal("third connection over a budget of two was allowed")
	}
}

func TestEgressCloseTearsDownLiveTunnels(t *testing.T) {
	h := newEgressHarness(t, EcosystemEgressPolicy())
	h.resolved["pypi.org"] = []netip.Addr{publicAddr}
	conn, br, resp := h.connect("pypi.org:443")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("CONNECT status = %d", resp.StatusCode)
	}
	h.proxy.Close()
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err := br.ReadByte(); err == nil {
		t.Fatal("tunnel still open after Close")
	} else if ne, ok := err.(net.Error); ok && ne.Timeout() {
		t.Fatal("tunnel still open after Close (read timed out instead of EOF)")
	}
}

// The session worker's exec path passes through exactly the keys the broker
// sets; a key added on one side only would silently not reach cell commands.
func TestEgressProxyEnvMatchesWorkerPassthrough(t *testing.T) {
	var keys []string
	for _, kv := range EgressProxyEnv() {
		key, _, _ := strings.Cut(kv, "=")
		keys = append(keys, key)
	}
	if strings.Join(keys, ",") != strings.Join(yaegikernel.EgressProxyKeys, ",") {
		t.Fatalf("broker sets %v, worker passes through %v", keys, yaegikernel.EgressProxyKeys)
	}
}
