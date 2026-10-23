package autoputer

// Two host-peer-only guest diagnostics for deployed-proof steps that need a
// view from inside a guest, which has no shell:
//
//	GET  /internal/diag/dns               SEC-10 step 3 (guest DNS)
//	GET|POST /internal/diag/gateway-body-cap   FP7 step 11 (gateway body caps)
//
// Record and provider-safety argument:
// docs/problems/guest-proof-steps-have-no-route-2026-10-23.md. Both answer
// only the host peer (server.HostPeerCaller, like /internal/diag/tcp-dial),
// read no caller input, and use fixed targets.

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yusefmosiah/go-choir/internal/gatewaycapprobe"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/server"
)

const (
	diagDNSLookupName = "example.com"
	diagDNSUDPTarget  = "1.1.1.1:53"

	diagResolvConfPath    = "/etc/resolv.conf"
	diagNetworkUnitPath   = "/run/systemd/network/10-vm-runtime.network"
	diagResolvedConfPath  = "/run/systemd/resolve/resolv.conf"
	diagMaxFileBytes      = 16 << 10
	diagMaxResolvectlOut  = 8 << 10
	diagBodyPrefixBytes   = 2048
	diagBodyCapMinSpacing = 10 * time.Second
)

// ---- GET /internal/diag/dns ---------------------------------------------

type dnsDiagSources struct {
	resolvConfPath   string
	networkUnitPath  string
	resolvedConfPath string
	// resolvectl runs the fixed command `resolvectl status`; available is
	// false when the binary is not in the image.
	resolvectl func(ctx context.Context) (out string, available bool, err error)
	lookup     func(ctx context.Context, name string) ([]string, error)
	udpQuery   func(ctx context.Context, addr, name string, timeout time.Duration) (bool, error)

	fixedLookupName   string
	fixedUDPTarget    string
	udpTimeout        time.Duration
	lookupTimeout     time.Duration
	resolvectlTimeout time.Duration
}

func defaultDNSDiagSources() dnsDiagSources {
	return dnsDiagSources{
		resolvConfPath:   diagResolvConfPath,
		networkUnitPath:  diagNetworkUnitPath,
		resolvedConfPath: diagResolvedConfPath,
		resolvectl:       runResolvectlStatus,
		lookup: func(ctx context.Context, name string) ([]string, error) {
			return net.DefaultResolver.LookupHost(ctx, name)
		},
		udpQuery:          udpDNSReachable,
		fixedLookupName:   diagDNSLookupName,
		fixedUDPTarget:    diagDNSUDPTarget,
		udpTimeout:        2 * time.Second,
		lookupTimeout:     3 * time.Second,
		resolvectlTimeout: 3 * time.Second,
	}
}

var handleDiagDNS = dnsDiagHandler(defaultDNSDiagSources())

func dnsDiagHandler(src dnsDiagSources) http.HandlerFunc {
	var running sync.Mutex
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !server.HostPeerCaller(r) {
			http.Error(w, "host peer required", http.StatusForbidden)
			return
		}
		// One run at a time: each run sends a UDP query and a lookup.
		if !running.TryLock() {
			w.Header().Set("Retry-After", "5")
			http.Error(w, "a dns probe is already running", http.StatusTooManyRequests)
			return
		}
		defer running.Unlock()
		// No query parameter is read: names, addresses and paths are fixed.
		resp := map[string]any{"schema_version": 1, "kind": "guest_dns_probe"}
		if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
			resp["caller_ip"] = host
		} else if r.RemoteAddr != "" {
			resp["caller_ip"] = r.RemoteAddr
		}

		resp["resolv_conf"] = readDiagFile(src.resolvConfPath, func(s string) (string, any) {
			return "nameservers", parseNameservers(s)
		})
		resp["network_unit"] = readDiagFile(src.networkUnitPath, func(s string) (string, any) {
			return "dns", parseNetworkUnitDNS(s)
		})
		resolved := readDiagFile(src.resolvedConfPath, func(s string) (string, any) {
			return "nameservers", parseNameservers(s)
		})
		rv := map[string]any{"available": false}
		if src.resolvectl != nil {
			ctx, cancel := context.WithTimeout(r.Context(), src.resolvectlTimeout)
			out, available, err := src.resolvectl(ctx)
			cancel()
			rv["available"] = available
			if available {
				if len(out) > diagMaxResolvectlOut {
					out = out[:diagMaxResolvectlOut]
				}
				p := parseResolvectlStatus(out)
				rv["output"] = out
				rv["dns_servers"] = nonNil(p.DNSServers)
				rv["current_dns_server"] = nonNil(p.CurrentDNSServer)
				rv["fallback_dns_servers"] = nonNil(p.FallbackDNSServers)
			}
			if err != nil {
				rv["error"] = err.Error()
			}
		}
		resolved["resolvectl"] = rv
		resp["resolved"] = resolved

		lookup := map[string]any{"name": src.fixedLookupName}
		if src.lookup != nil {
			ctx, cancel := context.WithTimeout(r.Context(), src.lookupTimeout)
			started := time.Now()
			addrs, err := src.lookup(ctx, src.fixedLookupName)
			cancel()
			lookup["latency_ms"] = time.Since(started).Milliseconds()
			lookup["ok"] = err == nil && len(addrs) > 0
			lookup["addrs"] = nonNil(addrs)
			if err != nil {
				lookup["error"] = err.Error()
			}
		}
		resp["system_lookup"] = lookup

		udp := map[string]any{"target": src.fixedUDPTarget, "name": src.fixedLookupName, "timeout_ms": src.udpTimeout.Milliseconds()}
		if src.udpQuery != nil {
			ctx, cancel := context.WithTimeout(r.Context(), src.udpTimeout+time.Second)
			started := time.Now()
			ok, err := src.udpQuery(ctx, src.fixedUDPTarget, src.fixedLookupName, src.udpTimeout)
			cancel()
			udp["latency_ms"] = time.Since(started).Milliseconds()
			udp["reachable"] = ok
			if err != nil {
				udp["error"] = err.Error()
			}
		}
		resp["direct_udp53"] = udp

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// readDiagFile reads one bounded file and returns {path, <key>: parsed} or
// {path, error}.
func readDiagFile(path string, parse func(string) (string, any)) map[string]any {
	out := map[string]any{"path": path}
	f, err := os.Open(path)
	if err != nil {
		out["error"] = err.Error()
		return out
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, diagMaxFileBytes))
	if err != nil {
		out["error"] = err.Error()
		return out
	}
	key, val := parse(string(data))
	out[key] = val
	return out
}

// parseNameservers returns the `nameserver <addr>` values of a resolv.conf.
func parseNameservers(text string) []string {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "nameserver" {
			out = append(out, fields[1])
		}
	}
	return nonNil(out)
}

// parseNetworkUnitDNS returns the DNS= values of the [Network] section of a
// systemd.network unit; a DNS= in another section is not the link's DNS.
func parseNetworkUnitDNS(text string) []string {
	var out []string
	section := ""
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = line
			continue
		}
		if section != "[Network]" {
			continue
		}
		if key, value, ok := strings.Cut(line, "="); ok && strings.TrimSpace(key) == "DNS" {
			out = append(out, strings.Fields(value)...)
		}
	}
	return nonNil(out)
}

type resolvectlStatus struct {
	DNSServers         []string
	CurrentDNSServer   []string
	FallbackDNSServers []string
}

// parseResolvectlStatus reads the DNS Servers, Current DNS Server and
// Fallback DNS Servers fields of `resolvectl status`, including the indented
// continuation lines that carry further servers.
func parseResolvectlStatus(text string) resolvectlStatus {
	var st resolvectlStatus
	var target *[]string
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		key, value, hasKey := strings.Cut(trimmed, ":")
		if hasKey {
			switch strings.TrimSpace(key) {
			case "DNS Servers":
				target = &st.DNSServers
			case "Current DNS Server":
				target = &st.CurrentDNSServer
			case "Fallback DNS Servers":
				target = &st.FallbackDNSServers
			default:
				// An IPv6 address on a continuation line also contains colons.
				if target != nil && looksLikeServer(trimmed) {
					*target = append(*target, strings.Fields(trimmed)[0])
					continue
				}
				target = nil
				continue
			}
			*target = append(*target, strings.Fields(value)...)
			continue
		}
		if target != nil && looksLikeServer(trimmed) {
			*target = append(*target, strings.Fields(trimmed)[0])
			continue
		}
		target = nil
	}
	st.DNSServers, st.CurrentDNSServer, st.FallbackDNSServers = nonNil(st.DNSServers), nonNil(st.CurrentDNSServer), nonNil(st.FallbackDNSServers)
	return st
}

func looksLikeServer(s string) bool {
	f := strings.Fields(s)
	if len(f) != 1 {
		return false
	}
	host, _, _ := strings.Cut(f[0], "#")
	host, _, _ = strings.Cut(host, "%")
	return net.ParseIP(host) != nil
}

// runResolvectlStatus runs the fixed command `resolvectl status`, if the
// image has resolvectl. No argument is caller-supplied.
func runResolvectlStatus(ctx context.Context) (string, bool, error) {
	path, err := exec.LookPath("resolvectl")
	if err != nil {
		for _, p := range []string{"/run/current-system/sw/bin/resolvectl", "/usr/bin/resolvectl", "/bin/resolvectl"} {
			if st, statErr := os.Stat(p); statErr == nil && !st.IsDir() {
				path, err = p, nil
				break
			}
		}
	}
	if err != nil {
		return "", false, nil
	}
	var buf bytes.Buffer
	cmd := exec.CommandContext(ctx, path, "status", "--no-pager")
	cmd.Stdout = &limitedWriter{w: &buf, n: diagMaxResolvectlOut}
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return buf.String(), true, err
	}
	return buf.String(), true, nil
}

type limitedWriter struct {
	w io.Writer
	n int
}

func (l *limitedWriter) Write(p []byte) (int, error) {
	if l.n <= 0 {
		return len(p), nil
	}
	if len(p) > l.n {
		_, _ = l.w.Write(p[:l.n])
		l.n = 0
		return len(p), nil
	}
	l.n -= len(p)
	return l.w.Write(p)
}

// buildDNSQuery is one recursive A query for name.
func buildDNSQuery(id uint16, name string) []byte {
	var b bytes.Buffer
	_ = binary.Write(&b, binary.BigEndian, id)
	b.Write([]byte{0x01, 0x00}) // flags: recursion desired
	b.Write([]byte{0, 1, 0, 0, 0, 0, 0, 0})
	for _, label := range strings.Split(strings.TrimSuffix(name, "."), ".") {
		b.WriteByte(byte(len(label)))
		b.WriteString(label)
	}
	b.WriteByte(0)
	b.Write([]byte{0, 1, 0, 1}) // type A, class IN
	return b.Bytes()
}

// udpDNSReachable sends one UDP DNS query to addr and reports whether a
// response carrying the query's ID came back within timeout. Any answer,
// including an error rcode, counts as reachable: the question is whether UDP
// 53 to that address is open, not what it says.
func udpDNSReachable(ctx context.Context, addr, name string, timeout time.Duration) (bool, error) {
	var idb [2]byte
	if _, err := rand.Read(idb[:]); err != nil {
		return false, err
	}
	id := binary.BigEndian.Uint16(idb[:])
	d := net.Dialer{Timeout: timeout}
	conn, err := d.DialContext(ctx, "udp", addr)
	if err != nil {
		return false, err
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(timeout))
	if _, err := conn.Write(buildDNSQuery(id, name)); err != nil {
		return false, err
	}
	buf := make([]byte, 512)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			return false, err
		}
		if n >= 12 && binary.BigEndian.Uint16(buf[:2]) == id && buf[2]&0x80 != 0 {
			return true, nil
		}
	}
}

// ---- GET|POST /internal/diag/gateway-body-cap ---------------------------

type gatewayBodyCapOptions struct {
	// minInterval is the spacing between two runs (429 inside it).
	minInterval      time.Duration
	searchLegTimeout time.Duration
	modelLegTimeout  time.Duration
}

func envGatewayConfig() (string, string) {
	return strings.TrimSpace(os.Getenv("RUNTIME_GATEWAY_URL")), provideriface.GatewayToken()
}

var handleDiagGatewayBodyCap = gatewayBodyCapHandler(envGatewayConfig, gatewayBodyCapOptions{
	minInterval:      diagBodyCapMinSpacing,
	searchLegTimeout: 20 * time.Second,
	modelLegTimeout:  5 * time.Second,
})

// gatewayBodyCapHandler sends two fixed requests to the gateway with the
// guest's own token and reports each answer: a chunked 1 MiB + 4 KiB search
// body (an unterminated JSON string) and a model request declaring 700 MiB
// with no body byte sent. Neither can decode into a request, so no provider
// is ever reached whatever the caps do (gatewaycapprobe, and the pinned
// gateway test). One run at a time; a run inside minInterval of the last one
// is refused with 429.
func gatewayBodyCapHandler(cfg func() (baseURL, token string), opts gatewayBodyCapOptions) http.HandlerFunc {
	var (
		running sync.Mutex
		lastMu  sync.Mutex
		last    time.Time
	)
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !server.HostPeerCaller(r) {
			http.Error(w, "host peer required", http.StatusForbidden)
			return
		}
		if !running.TryLock() {
			w.Header().Set("Retry-After", "10")
			http.Error(w, "a gateway body-cap probe is already running", http.StatusTooManyRequests)
			return
		}
		defer running.Unlock()
		lastMu.Lock()
		if wait := opts.minInterval - time.Since(last); !last.IsZero() && wait > 0 {
			lastMu.Unlock()
			w.Header().Set("Retry-After", strconv.Itoa(int(wait/time.Second)+1))
			http.Error(w, "gateway body-cap probe is rate limited", http.StatusTooManyRequests)
			return
		}
		lastMu.Unlock()

		base, token := cfg()
		if base == "" || token == "" {
			http.Error(w, "no gateway url or token in this guest", http.StatusServiceUnavailable)
			return
		}
		u, err := url.Parse(base)
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
			http.Error(w, "gateway url unusable", http.StatusServiceUnavailable)
			return
		}
		lastMu.Lock()
		last = time.Now()
		lastMu.Unlock()

		scrub := func(s string) string { return strings.ReplaceAll(s, token, "[redacted]") }
		resp := map[string]any{
			"schema_version": 1,
			"kind":           "guest_gateway_body_cap",
			"gateway":        scrub(u.Scheme + "://" + u.Host),
			"token_present":  true,
			"probes": []any{
				searchStreamedLeg(r.Context(), u, token, opts.searchLegTimeout, scrub),
				modelDeclaredLeg(r.Context(), u, token, opts.modelLegTimeout, scrub),
			},
			"note": "no provider is reachable from either leg: the search body is an unterminated JSON string and the model leg sends no body byte",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

func flattenHeaders(h http.Header, scrub func(string) string) map[string]string {
	out := make(map[string]string, len(h))
	for k, v := range h {
		out[scrub(k)] = scrub(strings.Join(v, ", "))
	}
	return out
}

func searchStreamedLeg(ctx context.Context, base *url.URL, token string, timeout time.Duration, scrub func(string) string) map[string]any {
	leg := map[string]any{"name": "search_streamed", "path": gatewaycapprobe.SearchPath,
		"body_bytes_planned": gatewaycapprobe.SearchBodySize, "body_bytes_sent": 0, "chunked": true}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cr := &sentCounter{r: gatewaycapprobe.SearchBody()}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base.Scheme+"://"+base.Host+gatewaycapprobe.SearchPath, cr)
	if err != nil {
		leg["error"] = scrub(err.Error())
		return leg
	}
	req.ContentLength = -1 // chunked: the cap must cut the stream, not read a declared length
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	started := time.Now()
	// Proxy nil: the bearer goes to the gateway address only, never to a
	// proxy named by the environment.
	client := &http.Client{Timeout: timeout, Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	leg["latency_ms"] = time.Since(started).Milliseconds()
	leg["body_bytes_sent"] = cr.n.Load()
	if err != nil {
		// A gateway that refuses mid-upload closes the connection; that is
		// an answer worth reporting, not a probe failure.
		leg["error"] = scrub(err.Error())
		return leg
	}
	defer func() { _ = resp.Body.Close() }()
	prefix, _ := io.ReadAll(io.LimitReader(resp.Body, diagBodyPrefixBytes))
	leg["status"] = resp.StatusCode
	leg["headers"] = flattenHeaders(resp.Header, scrub)
	leg["body_prefix"] = scrub(string(prefix))
	return leg
}

// modelDeclaredLeg writes the request head of a model call declaring 700 MiB
// and no body, then reads the gateway's answer. The raw connection is the
// only way to send a Content-Length with no body: net/http would insist on
// sending it.
func modelDeclaredLeg(ctx context.Context, base *url.URL, token string, timeout time.Duration, scrub func(string) string) map[string]any {
	leg := map[string]any{"name": "model_declared", "path": gatewaycapprobe.ModelPath,
		"declared_content_length": gatewaycapprobe.ModelDeclaredLength, "body_bytes_sent": 0}
	if base.Scheme != "http" {
		leg["error"] = "model_declared needs an http gateway url"
		return leg
	}
	host := base.Host
	if base.Port() == "" {
		host = net.JoinHostPort(base.Hostname(), "80")
	}
	d := net.Dialer{Timeout: timeout}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	started := time.Now()
	conn, err := d.DialContext(ctx, "tcp", host)
	if err != nil {
		leg["error"] = scrub(err.Error())
		return leg
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(timeout))
	if _, err := conn.Write(gatewaycapprobe.ModelDeclaredHead(base.Host, token)); err != nil {
		leg["error"] = scrub(err.Error())
		return leg
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	leg["latency_ms"] = time.Since(started).Milliseconds()
	if err != nil {
		// No answer before the deadline: the gateway is waiting for the
		// body it was promised. Closing the connection ends the request
		// with nothing decoded.
		leg["error"] = scrub(fmt.Sprintf("no response: %v", err))
		return leg
	}
	defer func() { _ = resp.Body.Close() }()
	prefix, _ := io.ReadAll(io.LimitReader(resp.Body, diagBodyPrefixBytes))
	leg["status"] = resp.StatusCode
	leg["headers"] = flattenHeaders(resp.Header, scrub)
	leg["body_prefix"] = scrub(string(prefix))
	return leg
}

// sentCounter counts the bytes the HTTP client actually read from the body.
type sentCounter struct {
	r io.Reader
	n atomic.Int64
}

func (s *sentCounter) Read(p []byte) (int, error) {
	n, err := s.r.Read(p)
	s.n.Add(int64(n))
	return n, err
}
