package capsule

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/sourcefetch"
)

// EgressSocketPath is where an ecosystem_proxy (L2) capsule finds the egress
// proxy, relative to the capsule root. The executor listens on it from
// outside the capsule; the broker splices loopback TCP onto it
// (docs/design/engineering-network-grants-2026-10-10.md §4.0b).
const EgressSocketPath = "/run/capsule/egress.sock"

// EgressLoopbackAddr is the in-capsule proxy address workloads see.
const EgressLoopbackAddr = "127.0.0.1:3128"

// EgressProxyEnv is the environment that points native tools (pip, uv, npm,
// git, go, cargo, curl, huggingface_hub) at the in-capsule proxy.
func EgressProxyEnv() []string {
	proxy := "http://" + EgressLoopbackAddr
	noProxy := "localhost,127.0.0.1,::1"
	return []string{
		"HTTP_PROXY=" + proxy, "HTTPS_PROXY=" + proxy, "http_proxy=" + proxy, "https_proxy=" + proxy,
		"NO_PROXY=" + noProxy, "no_proxy=" + noProxy,
		"npm_config_proxy=" + proxy, "npm_config_https_proxy=" + proxy,
		"SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt", "NIX_SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt",
		// The go command fetches modules and checksum-database data only
		// from the host module proxy: module zips redirect from
		// proxy.golang.org to a storage host L2 does not allow, and ",off"
		// keeps a miss from trying any other route.
		"GOPROXY=http://" + GoProxyRouteHost + ",off", "GOSUMDB=sum.golang.org", "GOFLAGS=-mod=mod",
	}
}

// GoProxyRouteHost is the name capsule workloads use for the host Go module
// proxy. It is an egress route, not a DNS name: the executor maps it to the
// configured host address and the capsule never learns that address
// (docs/reports/capsule-go-module-proxy-design-2026-10-14.md).
const GoProxyRouteHost = "goproxy.choir.internal"

// GoProxyHostPort is go-choir-goproxy's port on the guest's tap peer.
const GoProxyHostPort = 8088

// CapsuleHeader names the capsule on requests sent through a route.
const CapsuleHeader = "X-Choir-Capsule"

// GoModuleProxyAddr is where the executor routes GoProxyRouteHost: the
// explicit address when one is configured, else the gateway URL's host (the
// guest's tap peer) on GoProxyHostPort. ok is false when neither names an
// address; the route is then absent and module fetches are refused.
func GoModuleProxyAddr(explicit, gatewayURL string) (addr netip.AddrPort, ok bool, err error) {
	return hostServiceAddr(explicit, gatewayURL, GoProxyHostPort, "Go module proxy")
}

// hostServiceAddr is a host service's address for an egress route: the
// explicit ip:port when configured, else the gateway URL's literal host
// address on port. ok is false when neither names an address.
func hostServiceAddr(explicit, gatewayURL string, port uint16, what string) (addr netip.AddrPort, ok bool, err error) {
	if explicit = strings.TrimSpace(explicit); explicit != "" {
		addr, err := netip.ParseAddrPort(explicit)
		if err != nil {
			return netip.AddrPort{}, false, fmt.Errorf("capsule %s address %q: want ip:port", what, explicit)
		}
		return netip.AddrPortFrom(addr.Addr().Unmap(), addr.Port()), true, nil
	}
	gatewayURL = strings.TrimSpace(gatewayURL)
	if gatewayURL == "" {
		return netip.AddrPort{}, false, nil
	}
	u, err := url.Parse(gatewayURL)
	if err != nil || u.Hostname() == "" {
		return netip.AddrPort{}, false, fmt.Errorf("gateway URL %q has no host", gatewayURL)
	}
	ip, err := netip.ParseAddr(u.Hostname())
	if err != nil {
		// Only a literal tap address makes a route; a name would need DNS.
		return netip.AddrPort{}, false, nil
	}
	return netip.AddrPortFrom(ip.Unmap(), port), true, nil
}

// EgressPolicy is what an L2 capsule may reach. Hosts are DNS suffixes
// matched on a label boundary; widening the list is a reviewed code change.
// Routes map a fixed name to a configured address that is dialed without
// DNS or the public-address check, for plain-HTTP GET/HEAD only: host
// services a capsule may use (today only GoProxyRouteHost). The executor
// fills Routes and TunnelIdentity from its own configuration, never from
// the capsule. TunnelIdentity is how CONNECT tunnels are bound to their name
// (egress_tls.go); the zero value is observe.
type EgressPolicy struct {
	Hosts            []string
	Routes           map[string]netip.AddrPort
	TunnelIdentity   TunnelIdentityMode
	MaxUploadBytes   int64
	MaxDownloadBytes int64
	MaxConnections   int
}

// ecosystemHosts are the package, source and paper ecosystems engineering
// needs for ordinary building and replication (§4.0a, L2).
var ecosystemHosts = []string{
	// Python
	"pypi.org", "pythonhosted.org", "download.pytorch.org",
	// JavaScript
	"registry.npmjs.org", "registry.yarnpkg.com", "repo.yarnpkg.com",
	// Rust
	"crates.io",
	// Go
	"proxy.golang.org", "sum.golang.org",
	// conda
	"conda.anaconda.org", "repo.anaconda.com",
	// source forges
	"github.com", "githubusercontent.com", "gitlab.com", "codeberg.org",
	// models, papers and datasets
	"huggingface.co", "hf.co", "arxiv.org", "zenodo.org", "doi.org",
}

// EcosystemEgressPolicy is the default L2 policy for implementation capsules.
func EcosystemEgressPolicy() EgressPolicy {
	return EgressPolicy{
		Hosts:            append([]string(nil), ecosystemHosts...),
		MaxUploadBytes:   16 << 20,
		MaxDownloadBytes: 8 << 30,
		MaxConnections:   4096,
	}
}

func (p EgressPolicy) allowsHost(host string) bool {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if host == "" {
		return false
	}
	if _, err := netip.ParseAddr(host); err == nil {
		return false
	}
	for _, suffix := range p.Hosts {
		if host == suffix || strings.HasSuffix(host, "."+suffix) {
			return true
		}
	}
	return false
}

// Egress record outcomes.
const (
	EgressOutcomeOK      = "ok"
	EgressOutcomeRefused = "refused"
	EgressOutcomeFailed  = "failed"
	EgressOutcomeBudget  = "budget_exhausted"
)

// EgressRecord is one connection through the proxy.
type EgressRecord struct {
	Time       time.Time `json:"time"`
	CapsuleID  string    `json:"capsule_id"`
	Method     string    `json:"method"`
	Host       string    `json:"host"`
	Port       int       `json:"port"`
	IP         string    `json:"ip,omitempty"`
	BytesUp    int64     `json:"bytes_up"`
	BytesDown  int64     `json:"bytes_down"`
	DurationMS int64     `json:"duration_ms"`
	Outcome    string    `json:"outcome"`
	Reason     string    `json:"reason,omitempty"`
	// TunnelIdentity is a CONNECT tunnel's verdict (egress_tls.go): pass,
	// would_refuse (observe relayed it) or refused; empty when the check is
	// off or the connection never reached it.
	TunnelIdentity       string `json:"tunnel_identity,omitempty"`
	TunnelIdentityReason string `json:"tunnel_identity_reason,omitempty"`
}

// EgressProxy serves one capsule's egress socket. Every decision is made
// here, outside the capsule: the host allowlist, the address check, the
// dial of the checked address, and the per-capsule budgets.
type EgressProxy struct {
	capsuleID string
	policy    EgressPolicy
	record    func(EgressRecord)
	resolve   func(ctx context.Context, host string) ([]netip.Addr, error)
	dial      func(ctx context.Context, network, address string) (net.Conn, error)
	// helloTimeout bounds how long a CONNECT tunnel may stay silent before
	// its ClientHello arrives (egress_tls.go); zero means the default.
	helloTimeout time.Duration
	// logf takes the once-per-host observe log line; log.Printf by default.
	logf func(format string, args ...any)

	mu          sync.Mutex
	closed      bool
	listeners   []net.Listener
	live        map[net.Conn]struct{}
	connections int
	up, down    int64
	spent       bool
	// wouldRefuseLogged holds the hosts whose observe would-refuse has been
	// logged for this capsule.
	wouldRefuseLogged map[string]bool
}

func NewEgressProxy(capsuleID string, policy EgressPolicy, record func(EgressRecord)) *EgressProxy {
	if record == nil {
		record = func(EgressRecord) {}
	}
	var dialer net.Dialer
	return &EgressProxy{
		capsuleID: capsuleID, policy: policy, record: record,
		resolve: func(ctx context.Context, host string) ([]netip.Addr, error) {
			return net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		},
		dial: dialer.DialContext,
		logf: log.Printf,
		live: map[net.Conn]struct{}{},
	}
}

// Serve accepts connections until the listener closes or Close is called.
func (p *EgressProxy) Serve(listener net.Listener) error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		listener.Close()
		return net.ErrClosed
	}
	p.listeners = append(p.listeners, listener)
	p.mu.Unlock()
	for {
		conn, err := listener.Accept()
		if err != nil {
			return err
		}
		go p.serveConn(conn)
	}
}

// Close stops every listener and cuts every live connection.
func (p *EgressProxy) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
	for _, l := range p.listeners {
		l.Close()
	}
	for c := range p.live {
		c.Close()
	}
}

func (p *EgressProxy) track(conns ...net.Conn) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return false
	}
	for _, c := range conns {
		p.live[c] = struct{}{}
	}
	return true
}

func (p *EgressProxy) untrack(conns ...net.Conn) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, c := range conns {
		delete(p.live, c)
	}
}

// admit counts a connection against the budget.
func (p *EgressProxy) admit() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	switch {
	case p.spent || p.up >= p.policy.MaxUploadBytes || p.down >= p.policy.MaxDownloadBytes:
		return "capsule egress byte budget is spent"
	case p.connections >= p.policy.MaxConnections:
		return "capsule egress connection budget is spent"
	}
	p.connections++
	return ""
}

// charge adds n bytes in one direction; false means the budget is exceeded
// and the connection must be cut.
func (p *EgressProxy) charge(upload bool, n int64) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if upload {
		p.up += n
		if p.up > p.policy.MaxUploadBytes {
			p.spent = true
		}
	} else {
		p.down += n
		if p.down > p.policy.MaxDownloadBytes {
			p.spent = true
		}
	}
	return !p.spent
}

func (p *EgressProxy) serveConn(client net.Conn) {
	defer client.Close()
	if !p.track(client) {
		return
	}
	defer p.untrack(client)
	start := time.Now()
	rec := EgressRecord{Time: start.UTC(), CapsuleID: p.capsuleID}
	finish := func(outcome, reason string) {
		rec.Outcome, rec.Reason, rec.DurationMS = outcome, reason, time.Since(start).Milliseconds()
		p.record(rec)
	}
	refuse := func(status int, reason string) {
		fmt.Fprintf(client, "HTTP/1.1 %d %s\r\nContent-Type: text/plain\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s",
			status, http.StatusText(status), len(reason)+1, reason+"\n")
		finish(EgressOutcomeRefused, reason)
	}

	_ = client.SetReadDeadline(time.Now().Add(30 * time.Second))
	br := bufio.NewReaderSize(client, tunnelBufferSize)
	req, err := http.ReadRequest(br)
	if err != nil {
		rec.Method = "invalid"
		refuse(http.StatusBadRequest, "malformed proxy request")
		return
	}
	_ = client.SetReadDeadline(time.Time{})
	rec.Method = req.Method

	var host, port string
	switch {
	case req.Method == http.MethodConnect:
		host, port, err = net.SplitHostPort(req.Host)
		if err != nil {
			refuse(http.StatusBadRequest, "CONNECT target must be host:port")
			return
		}
	case req.Method == http.MethodGet || req.Method == http.MethodHead:
		if req.URL == nil || req.URL.Scheme != "http" || req.URL.Host == "" {
			refuse(http.StatusBadRequest, "plain-HTTP proxying needs an absolute http:// URL")
			return
		}
		host, port = req.URL.Hostname(), req.URL.Port()
		if port == "" {
			port = "80"
		}
	default:
		rec.Host = req.Host
		refuse(http.StatusMethodNotAllowed, "only CONNECT and plain-HTTP GET/HEAD leave an L2 capsule")
		return
	}
	rec.Host = strings.TrimSuffix(strings.ToLower(host), ".")
	rec.Port, _ = strconv.Atoi(port)
	wantPort := 80
	if req.Method == http.MethodConnect {
		wantPort = 443
	}
	if rec.Port != wantPort {
		refuse(http.StatusForbidden, fmt.Sprintf("%s is allowed only to port %d", req.Method, wantPort))
		return
	}
	route, routed := p.policy.Routes[rec.Host]
	switch {
	case routed && req.Method == http.MethodConnect:
		refuse(http.StatusForbidden, rec.Host+" serves plain HTTP only")
		return
	case !routed && rec.Host == GoProxyRouteHost:
		refuse(http.StatusForbidden, "no host Go module proxy is configured for this computer")
		return
	case !routed && rec.Host == MirrorRouteHost:
		refuse(http.StatusForbidden, "no host package mirror is granted to this computer")
		return
	case !routed && !p.policy.allowsHost(rec.Host):
		refuse(http.StatusForbidden, "host is not on the L2 ecosystem allowlist; ask management for a wider grant")
		return
	}
	if reason := p.admit(); reason != "" {
		rec.Outcome = EgressOutcomeBudget
		fmt.Fprintf(client, "HTTP/1.1 429 Too Many Requests\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s\n", len(reason)+1, reason)
		finish(EgressOutcomeBudget, reason)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	var target string
	if routed {
		// A configured host service: no DNS, and its private address is the
		// point. The capsule named the route; it cannot name the address.
		rec.IP = route.Addr().String()
		target = route.String()
	} else {
		addrs, err := p.resolve(ctx, rec.Host)
		if err != nil || len(addrs) == 0 {
			cancel()
			refuse(http.StatusBadGateway, "host did not resolve")
			return
		}
		// Refuse when any answer is non-public: a mixed answer is how DNS
		// rebinding reaches the guest network.
		for _, addr := range addrs {
			if sourcefetch.AddressBlocked(net.IP(addr.Unmap().AsSlice())) {
				cancel()
				refuse(http.StatusForbidden, "host resolves to a non-public address")
				return
			}
		}
		checked := addrs[0].Unmap()
		rec.IP = checked.String()
		target = net.JoinHostPort(checked.String(), port)
	}
	upstream, err := p.dial(ctx, "tcp", target)
	cancel()
	if err != nil {
		fmt.Fprintf(client, "HTTP/1.1 502 Bad Gateway\r\nContent-Length: 0\r\nConnection: close\r\n\r\n")
		finish(EgressOutcomeFailed, "dial failed")
		return
	}
	defer upstream.Close()
	if !p.track(upstream) {
		finish(EgressOutcomeFailed, "proxy closed")
		return
	}
	defer p.untrack(upstream)

	var upstreamReader io.Reader = upstream
	if req.Method == http.MethodConnect {
		if _, err := io.WriteString(client, "HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
			finish(EgressOutcomeFailed, "client went away")
			return
		}
		// The tunnel is bound to the name the CONNECT asked for: its first
		// record must be a ClientHello for exactly that name. Under enforce
		// the tunnel is closed before a byte reaches the upstream; under
		// observe the verdict is recorded and the tunnel relayed unchanged
		// (egress_tls.go).
		if !p.checkTunnel(client, br, &rec) {
			finish(EgressOutcomeRefused, rec.TunnelIdentityReason)
			return
		}
	} else {
		req.Header.Del("Proxy-Connection")
		req.Header.Del("Proxy-Authorization")
		req.Header.Del(CapsuleHeader)
		if routed {
			// Attribution for the host service's log, never authority.
			req.Header.Set(CapsuleHeader, p.capsuleID)
		}
		req.Header.Set("Connection", "close")
		req.Close = true
		req.RequestURI = ""
		var head strings.Builder
		if err := req.Write(&head); err != nil {
			finish(EgressOutcomeFailed, "request rewrite failed")
			return
		}
		if !p.charge(true, int64(head.Len())) {
			finish(EgressOutcomeBudget, "upload budget exceeded")
			return
		}
		rec.BytesUp += int64(head.Len())
		if _, err := io.WriteString(upstream, head.String()); err != nil {
			finish(EgressOutcomeFailed, "upstream write failed")
			return
		}
		br = bufio.NewReader(strings.NewReader("")) // GET/HEAD carry no body upstream
	}

	outcome := p.splice(client, br, upstream, upstreamReader, &rec, req.Method == http.MethodConnect)
	finish(outcome, map[string]string{EgressOutcomeBudget: "byte budget exceeded"}[outcome])
}

// splice copies both ways, charging each direction against the capsule
// budget. Bytes already buffered behind the request header go first.
func (p *EgressProxy) splice(client net.Conn, clientReader io.Reader, upstream net.Conn, upstreamReader io.Reader, rec *EgressRecord, bidirectional bool) string {
	var wg sync.WaitGroup
	var overBudget bool
	var mu sync.Mutex
	copyCharged := func(dst net.Conn, src io.Reader, upload bool, counter *int64) {
		buf := make([]byte, 32<<10)
		for {
			n, err := src.Read(buf)
			if n > 0 {
				if !p.charge(upload, int64(n)) {
					mu.Lock()
					overBudget = true
					mu.Unlock()
					client.Close()
					upstream.Close()
					return
				}
				mu.Lock()
				*counter += int64(n)
				mu.Unlock()
				if _, werr := dst.Write(buf[:n]); werr != nil {
					return
				}
			}
			if err != nil {
				if cw, ok := dst.(interface{ CloseWrite() error }); ok && errors.Is(err, io.EOF) {
					_ = cw.CloseWrite()
				} else {
					dst.Close()
				}
				return
			}
		}
	}
	if bidirectional {
		wg.Add(1)
		go func() { defer wg.Done(); copyCharged(upstream, clientReader, true, &rec.BytesUp) }()
	}
	copyCharged(client, upstreamReader, false, &rec.BytesDown)
	if !bidirectional {
		client.Close()
	}
	wg.Wait()
	if overBudget {
		return EgressOutcomeBudget
	}
	return EgressOutcomeOK
}
