package autoputer

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/gatewaycapprobe"
)

// Guest diagnostic probes for two deployed-proof steps that need a view from
// inside a guest (docs/problems/guest-proof-steps-have-no-route-2026-10-23.md).
// Ways they can fail, written before the code:
//  1. a caller that is not the host peer drives them
//     (TestDiagProbesRefuseNonHostPeer);
//  2. the DNS probe misreads the files it reports
//     (TestParse..., TestDNSProbeReportsFixtures, ...MissingFiles);
//  3. the DNS probe takes a name or an address from the caller and becomes a
//     resolver oracle (TestDNSProbeTakesNoCallerInput);
//  4. the body probe leaks the guest's gateway token
//     (TestBodyCapProbeNeverEchoesTheToken);
//  5. the body probe reaches a provider path, with any gateway behavior
//     (TestBodyCapProbeCannotReachAProvider, in every gateway mode);
//  6. the body probe is caller-sized or can be used to flood the gateway
//     (TestBodyCapProbeIsFixedSizeSerializedAndSpaced).

const diagProbeToken = "guest-token-0123456789abcdef-must-never-be-echoed"

func nonHostPeerRequest(method, target string) *http.Request {
	req := httptest.NewRequest(method, target, nil)
	req.RemoteAddr = "127.0.0.1:41000" // an in-guest caller
	return req
}

func hostPeerRequest(method, target string) *http.Request {
	req := httptest.NewRequest(method, target, nil)
	req.RemoteAddr = "192.0.2.44:40000" // the synthetic host peer HostPeerCaller accepts
	return req
}

func TestDiagProbesRefuseNonHostPeer(t *testing.T) {
	var gatewayHits atomic.Int64
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gatewayHits.Add(1)
	}))
	defer gw.Close()
	var lookups atomic.Int64
	dns := dnsDiagHandler(dnsDiagSources{
		lookup: func(context.Context, string) ([]string, error) { lookups.Add(1); return nil, nil },
		udpQuery: func(context.Context, string, string, time.Duration) (bool, error) {
			lookups.Add(1)
			return false, nil
		},
	})
	body := gatewayBodyCapHandler(func() (string, string) { return gw.URL, diagProbeToken }, fastBodyCapOptions())
	for _, addr := range []string{"127.0.0.1:5000", "[::1]:5000", "8.8.8.8:5000", "10.200.9.2:5000"} {
		for name, h := range map[string]http.HandlerFunc{"dns": dns, "body-cap": body} {
			req := httptest.NewRequest(http.MethodGet, "/internal/diag/"+name, nil)
			req.RemoteAddr = addr
			rec := httptest.NewRecorder()
			h(rec, req)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("%s from %s = %d, want 403", name, addr, rec.Code)
			}
			if strings.Contains(rec.Body.String(), diagProbeToken) {
				t.Fatalf("%s 403 body carries the token", name)
			}
		}
	}
	if lookups.Load() != 0 || gatewayHits.Load() != 0 {
		t.Fatalf("a refused caller still caused work: lookups=%d gateway hits=%d", lookups.Load(), gatewayHits.Load())
	}
}

func TestDiagProbesRefuseOtherMethods(t *testing.T) {
	dns := dnsDiagHandler(dnsDiagSources{})
	rec := httptest.NewRecorder()
	dns(rec, hostPeerRequest(http.MethodPost, "/internal/diag/dns"))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST dns = %d, want 405", rec.Code)
	}
	body := gatewayBodyCapHandler(func() (string, string) { return "http://127.0.0.1:1", diagProbeToken }, fastBodyCapOptions())
	rec = httptest.NewRecorder()
	body(rec, hostPeerRequest(http.MethodDelete, "/internal/diag/gateway-body-cap"))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("DELETE body-cap = %d, want 405", rec.Code)
	}
}

// ---- DNS ---------------------------------------------------------------

const fixtureResolvConf = `# This is /run/systemd/resolve/stub-resolv.conf managed by man:systemd-resolved(8).
# Do not edit.
nameserver 127.0.0.53
options edns0 trust-ad
search .
`

const fixtureResolvedConf = `# This is /run/systemd/resolve/resolv.conf managed by man:systemd-resolved(8).
nameserver 10.200.42.1
search .
`

const fixtureNetworkUnit = `[Match]
Driver=virtio_net

[Network]
Address=10.200.42.2/30
Gateway=10.200.42.1
DHCP=no
IPv6AcceptRA=no
LinkLocalAddressing=no
DNS=10.200.42.1
`

const fixtureResolvectl = `Global
       Protocols: +LLMNR +mDNS -DNSOverTLS DNSSEC=no/unsupported
resolv.conf mode: stub
Fallback DNS Servers: 1.1.1.1#cloudflare-dns.com 8.8.8.8#dns.google

Link 2 (eth0)
    Current Scopes: DNS
         Protocols: +DefaultRoute
Current DNS Server: 10.200.42.1
       DNS Servers: 10.200.42.1
                    10.200.42.7
`

func TestParseResolvConfNameservers(t *testing.T) {
	got := parseNameservers(fixtureResolvConf + "nameserver   10.0.0.1  \n#nameserver 9.9.9.9\nnameserver\n")
	want := []string{"127.0.0.53", "10.0.0.1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("nameservers = %v, want %v", got, want)
	}
	if got := parseNameservers(""); len(got) != 0 {
		t.Fatalf("empty file gave %v", got)
	}
}

func TestParseNetworkUnitDNS(t *testing.T) {
	if got, want := parseNetworkUnitDNS(fixtureNetworkUnit), []string{"10.200.42.1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("DNS= = %v, want %v", got, want)
	}
	// Several values on a line, several lines, and a DNS= outside [Network]
	// (which systemd does not read as the link's DNS) is not counted.
	odd := "[Network]\nDNS=10.0.0.1 10.0.0.2\nDNS=10.0.0.3\n[DHCPv4]\nDNS=9.9.9.9\n"
	if got, want := parseNetworkUnitDNS(odd), []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("DNS= = %v, want %v", got, want)
	}
	if got := parseNetworkUnitDNS("[Network]\nAddress=1.2.3.4/30\n"); len(got) != 0 {
		t.Fatalf("a unit with no DNS= gave %v", got)
	}
}

func TestParseResolvectlStatus(t *testing.T) {
	got := parseResolvectlStatus(fixtureResolvectl)
	if want := []string{"10.200.42.1", "10.200.42.7"}; !reflect.DeepEqual(got.DNSServers, want) {
		t.Fatalf("DNS servers = %v, want %v", got.DNSServers, want)
	}
	if want := []string{"10.200.42.1"}; !reflect.DeepEqual(got.CurrentDNSServer, want) {
		t.Fatalf("current = %v, want %v", got.CurrentDNSServer, want)
	}
	if want := []string{"1.1.1.1#cloudflare-dns.com", "8.8.8.8#dns.google"}; !reflect.DeepEqual(got.FallbackDNSServers, want) {
		t.Fatalf("fallback = %v, want %v", got.FallbackDNSServers, want)
	}
}

func writeFixture(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func decodeJSON(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("response is not JSON: %v\n%s", err, rec.Body.String())
	}
	return out
}

func TestDNSProbeReportsFixtures(t *testing.T) {
	dir := t.TempDir()
	src := dnsDiagSources{
		resolvConfPath:   writeFixture(t, dir, "resolv.conf", fixtureResolvConf),
		networkUnitPath:  writeFixture(t, dir, "10-vm-runtime.network", fixtureNetworkUnit),
		resolvedConfPath: writeFixture(t, dir, "resolved-resolv.conf", fixtureResolvedConf),
		resolvectl:       func(context.Context) (string, bool, error) { return fixtureResolvectl, true, nil },
		lookup:           func(context.Context, string) ([]string, error) { return []string{"93.184.216.34"}, nil },
		udpQuery: func(context.Context, string, string, time.Duration) (bool, error) {
			return false, errors.New("i/o timeout")
		},
		fixedLookupName:   "example.com",
		fixedUDPTarget:    "1.1.1.1:53",
		udpTimeout:        2 * time.Second,
		lookupTimeout:     3 * time.Second,
		resolvectlTimeout: 3 * time.Second,
	}
	rec := httptest.NewRecorder()
	req := hostPeerRequest(http.MethodGet, "/internal/diag/dns")
	req.RemoteAddr = "192.0.2.1:7"
	dnsDiagHandler(src)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	out := decodeJSON(t, rec)
	if out["kind"] != "guest_dns_probe" || out["schema_version"] != float64(1) {
		t.Fatalf("kind/schema = %v/%v", out["kind"], out["schema_version"])
	}
	rc := out["resolv_conf"].(map[string]any)
	if !reflect.DeepEqual(rc["nameservers"], []any{"127.0.0.53"}) {
		t.Fatalf("resolv_conf.nameservers = %v", rc["nameservers"])
	}
	nu := out["network_unit"].(map[string]any)
	if !reflect.DeepEqual(nu["dns"], []any{"10.200.42.1"}) {
		t.Fatalf("network_unit.dns = %v", nu["dns"])
	}
	rs := out["resolved"].(map[string]any)
	if !reflect.DeepEqual(rs["nameservers"], []any{"10.200.42.1"}) {
		t.Fatalf("resolved.nameservers = %v", rs["nameservers"])
	}
	rv := rs["resolvectl"].(map[string]any)
	if rv["available"] != true || !strings.Contains(rv["output"].(string), "Current DNS Server: 10.200.42.1") {
		t.Fatalf("resolvectl = %v", rv)
	}
	if !reflect.DeepEqual(rv["dns_servers"], []any{"10.200.42.1", "10.200.42.7"}) ||
		!reflect.DeepEqual(rv["fallback_dns_servers"], []any{"1.1.1.1#cloudflare-dns.com", "8.8.8.8#dns.google"}) {
		t.Fatalf("resolvectl parse = %v", rv)
	}
	sl := out["system_lookup"].(map[string]any)
	if sl["name"] != "example.com" || sl["ok"] != true || !reflect.DeepEqual(sl["addrs"], []any{"93.184.216.34"}) {
		t.Fatalf("system_lookup = %v", sl)
	}
	u := out["direct_udp53"].(map[string]any)
	if u["target"] != "1.1.1.1:53" || u["reachable"] != false || u["error"] != "i/o timeout" {
		t.Fatalf("direct_udp53 = %v", u)
	}
	if out["caller_ip"] != "192.0.2.1" {
		t.Fatalf("caller_ip = %v", out["caller_ip"])
	}
}

func TestDNSProbeReportsMissingFilesAndNoResolvectl(t *testing.T) {
	dir := t.TempDir()
	src := dnsDiagSources{
		resolvConfPath:   filepath.Join(dir, "absent-resolv.conf"),
		networkUnitPath:  filepath.Join(dir, "absent.network"),
		resolvedConfPath: filepath.Join(dir, "absent-resolved.conf"),
		resolvectl:       func(context.Context) (string, bool, error) { return "", false, nil },
		lookup:           func(context.Context, string) ([]string, error) { return nil, errors.New("no such host") },
		udpQuery:         func(context.Context, string, string, time.Duration) (bool, error) { return true, nil },
		fixedLookupName:  "example.com", fixedUDPTarget: "1.1.1.1:53",
		udpTimeout: time.Second, lookupTimeout: time.Second, resolvectlTimeout: time.Second,
	}
	rec := httptest.NewRecorder()
	dnsDiagHandler(src)(rec, hostPeerRequest(http.MethodGet, "/internal/diag/dns"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: a missing file is a finding, not a probe failure", rec.Code)
	}
	out := decodeJSON(t, rec)
	for _, k := range []string{"resolv_conf", "network_unit"} {
		if e, _ := out[k].(map[string]any)["error"].(string); e == "" {
			t.Fatalf("%s has no error for a missing file: %v", k, out[k])
		}
	}
	rv := out["resolved"].(map[string]any)["resolvectl"].(map[string]any)
	if rv["available"] != false {
		t.Fatalf("resolvectl.available = %v, want false", rv["available"])
	}
	if sl := out["system_lookup"].(map[string]any); sl["ok"] != false || sl["error"] != "no such host" {
		t.Fatalf("system_lookup = %v", sl)
	}
	if u := out["direct_udp53"].(map[string]any); u["reachable"] != true {
		t.Fatalf("direct_udp53 = %v", u)
	}
}

func TestDNSProbeTakesNoCallerInput(t *testing.T) {
	var mu sync.Mutex
	var names, targets []string
	src := defaultDNSDiagSources()
	src.resolvConfPath, src.networkUnitPath, src.resolvedConfPath = "/nonexistent/a", "/nonexistent/b", "/nonexistent/c"
	src.resolvectl = func(context.Context) (string, bool, error) { return "", false, nil }
	src.lookup = func(_ context.Context, name string) ([]string, error) {
		mu.Lock()
		names = append(names, name)
		mu.Unlock()
		return nil, nil
	}
	src.udpQuery = func(_ context.Context, addr, name string, _ time.Duration) (bool, error) {
		mu.Lock()
		targets = append(targets, addr+" "+name)
		mu.Unlock()
		return false, nil
	}
	rec := httptest.NewRecorder()
	dnsDiagHandler(src)(rec, hostPeerRequest(http.MethodGet,
		"/internal/diag/dns?name=evil.example&addr=9.9.9.9:53&target=8.8.8.8&server=203.0.113.9&path=/etc/shadow"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !reflect.DeepEqual(names, []string{"example.com"}) || !reflect.DeepEqual(targets, []string{"1.1.1.1:53 example.com"}) {
		t.Fatalf("caller input reached a lookup: names=%v targets=%v", names, targets)
	}
}

func TestDNSQueryPacketAndUDPReachability(t *testing.T) {
	pkt := buildDNSQuery(0xBEEF, "example.com")
	if pkt[0] != 0xBE || pkt[1] != 0xEF || pkt[2]&0x80 != 0 || pkt[5] != 1 {
		t.Fatalf("header = % x", pkt[:12])
	}
	if got := string(pkt[12:]); !strings.HasPrefix(got, "\x07example\x03com\x00") {
		t.Fatalf("question = %q", got)
	}
	answering, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("no loopback UDP: %v", err)
	}
	defer answering.Close()
	go func() {
		buf := make([]byte, 512)
		for {
			n, from, err := answering.ReadFrom(buf)
			if err != nil {
				return
			}
			resp := append([]byte(nil), buf[:n]...)
			resp[2] |= 0x80 // QR: a response with the query's ID
			_, _ = answering.WriteTo(resp, from)
		}
	}()
	ok, err := udpDNSReachable(context.Background(), answering.LocalAddr().String(), "example.com", time.Second)
	if err != nil || !ok {
		t.Fatalf("answering server: reachable=%v err=%v", ok, err)
	}
	silent, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("no loopback UDP: %v", err)
	}
	defer silent.Close()
	started := time.Now()
	ok, err = udpDNSReachable(context.Background(), silent.LocalAddr().String(), "example.com", 200*time.Millisecond)
	if ok || err == nil {
		t.Fatalf("silent server: reachable=%v err=%v, want not reachable with an error", ok, err)
	}
	if time.Since(started) > 2*time.Second {
		t.Fatalf("a silent resolver held the probe for %s", time.Since(started))
	}
}

// ---- gateway body cap ---------------------------------------------------

func fastBodyCapOptions() gatewayBodyCapOptions {
	return gatewayBodyCapOptions{minInterval: 0, searchLegTimeout: 5 * time.Second, modelLegTimeout: 400 * time.Millisecond}
}

// gatewayDouble stands in for the gateway. It counts calls to a "provider"
// that only a successfully decoded request can reach, and it tries to leak:
// it echoes the Authorization header into a response header and the body.
type gatewayDouble struct {
	srv        *httptest.Server
	provider   atomic.Int64 // calls that reached a provider
	searchSeen atomic.Int64 // bytes of search body read
	modelSeen  atomic.Int64 // bytes of model body read
	auths      []string
	paths      []string
	declared   []int64
	mu         sync.Mutex
	enforce    bool // cap model routes at 576 MiB declared, search at 1 MiB; false: no caps at all
	hold       chan struct{}
}

func newGatewayDouble(t *testing.T, enforce bool) *gatewayDouble {
	t.Helper()
	g := &gatewayDouble{enforce: enforce}
	mux := http.NewServeMux()
	mux.HandleFunc(gatewaycapprobe.SearchPath, func(w http.ResponseWriter, r *http.Request) {
		g.record(r)
		if g.hold != nil {
			select {
			case <-g.hold:
			case <-time.After(5 * time.Second):
			}
		}
		g.leak(w, r)
		var body io.Reader = r.Body
		if g.enforce {
			body = http.MaxBytesReader(w, r.Body, 1<<20)
		}
		cr := &countReader{r: body, n: &g.searchSeen}
		var req struct {
			Query string `json:"query"`
		}
		if err := json.NewDecoder(cr).Decode(&req); err != nil {
			var mbe *http.MaxBytesError
			if errors.As(err, &mbe) {
				http.Error(w, "request body too large: Bearer "+r.Header.Get("Authorization"), http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if req.Query != "" {
			g.provider.Add(1)
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc(gatewaycapprobe.ModelPath, func(w http.ResponseWriter, r *http.Request) {
		g.record(r)
		g.leak(w, r)
		if g.enforce && r.ContentLength > 576<<20 {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		cr := &countReader{r: r.Body, n: &g.modelSeen}
		var req map[string]any
		if err := json.NewDecoder(cr).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		g.provider.Add(1)
	})
	g.srv = httptest.NewServer(mux)
	t.Cleanup(g.srv.Close)
	return g
}

func (g *gatewayDouble) record(r *http.Request) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.auths = append(g.auths, r.Header.Get("Authorization"))
	g.paths = append(g.paths, r.URL.Path)
	g.declared = append(g.declared, r.ContentLength)
}

func (g *gatewayDouble) leak(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Echo-Authorization", r.Header.Get("Authorization"))
	w.Header().Set("X-Echo-Token", strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
}

type countReader struct {
	r io.Reader
	n *atomic.Int64
}

func (c *countReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n.Add(int64(n))
	return n, err
}

func runBodyCap(t *testing.T, g *gatewayDouble, opts gatewayBodyCapOptions, method string) *httptest.ResponseRecorder {
	t.Helper()
	h := gatewayBodyCapHandler(func() (string, string) { return g.srv.URL, diagProbeToken }, opts)
	rec := httptest.NewRecorder()
	h(rec, hostPeerRequest(method, "/internal/diag/gateway-body-cap"))
	return rec
}

func TestBodyCapProbeNeverEchoesTheToken(t *testing.T) {
	g := newGatewayDouble(t, true)
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		rec := runBodyCap(t, g, fastBodyCapOptions(), method)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s = %d: %s", method, rec.Code, rec.Body.String())
		}
		raw := rec.Body.String()
		if strings.Contains(raw, diagProbeToken) {
			t.Fatalf("%s response contains the token:\n%s", method, raw)
		}
		for k, v := range rec.Header() {
			if strings.Contains(strings.Join(v, ","), diagProbeToken) {
				t.Fatalf("%s response header %s contains the token", method, k)
			}
		}
		// The double really did try to leak it, and the probe really did
		// send the guest's own token: the scrub, not luck, kept it out.
		g.mu.Lock()
		sent := append([]string(nil), g.auths...)
		g.mu.Unlock()
		for _, a := range sent {
			if a != "Bearer "+diagProbeToken {
				t.Fatalf("the gateway saw Authorization %q, want the guest's own bearer", a)
			}
		}
		if !strings.Contains(raw, "[redacted]") {
			t.Fatalf("expected a redaction marker where the double echoed the token:\n%s", raw)
		}
	}
}

func TestBodyCapProbeReportsStatusAndHeadersOfBothLegs(t *testing.T) {
	g := newGatewayDouble(t, true)
	rec := runBodyCap(t, g, fastBodyCapOptions(), http.MethodGet)
	out := decodeJSON(t, rec)
	if out["kind"] != "guest_gateway_body_cap" || out["token_present"] != true {
		t.Fatalf("shape = %v", out)
	}
	legs := out["probes"].([]any)
	if len(legs) != 2 {
		t.Fatalf("legs = %d, want 2", len(legs))
	}
	search, model := legs[0].(map[string]any), legs[1].(map[string]any)
	if search["name"] != "search_streamed" || search["path"] != gatewaycapprobe.SearchPath || search["status"] != float64(413) {
		t.Fatalf("search leg = %v", search)
	}
	if model["name"] != "model_declared" || model["path"] != gatewaycapprobe.ModelPath || model["status"] != float64(413) {
		t.Fatalf("model leg = %v", model)
	}
	if model["declared_content_length"] != float64(gatewaycapprobe.ModelDeclaredLength) || model["body_bytes_sent"] != float64(0) {
		t.Fatalf("model leg declared/sent = %v", model)
	}
	if search["body_bytes_planned"] != float64(gatewaycapprobe.SearchBodySize) {
		t.Fatalf("search leg planned = %v, want %d", search["body_bytes_planned"], gatewaycapprobe.SearchBodySize)
	}
	if sent, _ := search["body_bytes_sent"].(float64); sent <= 0 || sent > float64(gatewaycapprobe.SearchBodySize) {
		t.Fatalf("search leg sent = %v, want 1..%d", search["body_bytes_sent"], gatewaycapprobe.SearchBodySize)
	}
	if h, _ := search["headers"].(map[string]any); h["Content-Type"] == nil {
		t.Fatalf("search headers = %v", search["headers"])
	}
}

// The probe's requests cannot reach a provider path: in a gateway that
// enforces its caps, in a gateway with no caps at all (the worst case: every
// request is read to the end), and with the model leg's body never sent.
func TestBodyCapProbeCannotReachAProvider(t *testing.T) {
	for _, enforce := range []bool{true, false} {
		g := newGatewayDouble(t, enforce)
		rec := runBodyCap(t, g, fastBodyCapOptions(), http.MethodPost)
		if rec.Code != http.StatusOK {
			t.Fatalf("enforce=%v: probe status %d: %s", enforce, rec.Code, rec.Body.String())
		}
		if n := g.provider.Load(); n != 0 {
			t.Fatalf("enforce=%v: the gateway reached a provider %d time(s)", enforce, n)
		}
		if n := g.modelSeen.Load(); n != 0 {
			t.Fatalf("enforce=%v: the model leg sent %d body bytes, want 0", enforce, n)
		}
		g.mu.Lock()
		paths := append([]string(nil), g.paths...)
		g.mu.Unlock()
		for _, p := range paths {
			if p != gatewaycapprobe.SearchPath && p != gatewaycapprobe.ModelPath {
				t.Fatalf("enforce=%v: the probe called %s", enforce, p)
			}
		}
		if !enforce {
			// No caps: the search body is read whole, and still decodes to
			// nothing, because it is an unterminated JSON string.
			if n := g.searchSeen.Load(); n != gatewaycapprobe.SearchBodySize {
				t.Fatalf("uncapped search read %d bytes, want %d", n, gatewaycapprobe.SearchBodySize)
			}
			out := decodeJSON(t, rec)
			if s := out["probes"].([]any)[0].(map[string]any)["status"]; s != float64(400) {
				t.Fatalf("uncapped search answered %v, want 400 (invalid body)", s)
			}
		}
	}
}

func TestBodyCapProbeIsFixedSizeSerializedAndSpaced(t *testing.T) {
	g := newGatewayDouble(t, true)
	opts := fastBodyCapOptions()
	opts.minInterval = 300 * time.Millisecond
	h := gatewayBodyCapHandler(func() (string, string) { return g.srv.URL, diagProbeToken }, opts)

	// Caller-supplied sizes and targets are ignored.
	rec := httptest.NewRecorder()
	h(rec, hostPeerRequest(http.MethodPost, "/internal/diag/gateway-body-cap?size=999999999&bytes=1&url=http://203.0.113.9&path=/provider/v1/inference"))
	if rec.Code != http.StatusOK {
		t.Fatalf("first = %d: %s", rec.Code, rec.Body.String())
	}
	if n := g.searchSeen.Load(); n > 1<<20+1 {
		t.Fatalf("the gateway read %d search bytes; a capped gateway reads at most the cap plus one", n)
	}
	// Spaced: an immediate second run is turned away with Retry-After.
	rec = httptest.NewRecorder()
	h(rec, hostPeerRequest(http.MethodGet, "/internal/diag/gateway-body-cap"))
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("second immediate run = %d Retry-After=%q, want 429 with Retry-After", rec.Code, rec.Header().Get("Retry-After"))
	}
	time.Sleep(350 * time.Millisecond)
	rec = httptest.NewRecorder()
	h(rec, hostPeerRequest(http.MethodGet, "/internal/diag/gateway-body-cap"))
	if rec.Code != http.StatusOK {
		t.Fatalf("run after the interval = %d: %s", rec.Code, rec.Body.String())
	}
}

func TestBodyCapProbeIsSerialized(t *testing.T) {
	g := newGatewayDouble(t, true)
	g.hold = make(chan struct{})
	h := gatewayBodyCapHandler(func() (string, string) { return g.srv.URL, diagProbeToken }, fastBodyCapOptions())
	first := make(chan int, 1)
	go func() {
		rec := httptest.NewRecorder()
		h(rec, hostPeerRequest(http.MethodGet, "/internal/diag/gateway-body-cap"))
		first <- rec.Code
	}()
	deadline := time.After(3 * time.Second)
	for {
		g.mu.Lock()
		n := len(g.paths)
		g.mu.Unlock()
		if n > 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("the first probe never reached the gateway double")
		case <-time.After(5 * time.Millisecond):
		}
	}
	rec := httptest.NewRecorder()
	h(rec, hostPeerRequest(http.MethodGet, "/internal/diag/gateway-body-cap"))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("a probe while one is running = %d, want 429", rec.Code)
	}
	close(g.hold)
	select {
	case code := <-first:
		if code != http.StatusOK {
			t.Fatalf("first probe = %d", code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("first probe never finished")
	}
}

func TestBodyCapProbeWithoutATokenSendsNothing(t *testing.T) {
	g := newGatewayDouble(t, true)
	h := gatewayBodyCapHandler(func() (string, string) { return g.srv.URL, "" }, fastBodyCapOptions())
	rec := httptest.NewRecorder()
	h(rec, hostPeerRequest(http.MethodGet, "/internal/diag/gateway-body-cap"))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("no token = %d, want 503", rec.Code)
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.paths) != 0 {
		t.Fatalf("a probe with no token still called the gateway: %v", g.paths)
	}
}
