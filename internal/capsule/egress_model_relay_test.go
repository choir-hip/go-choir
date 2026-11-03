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
)

// The model relay route (egress_model_relay.go). Failure modes, written
// before the code (problem record, design section):
//  15  a caller's Authorization or x-api-key, or any header outside the
//      list, reaches the gateway; a path other than the three relay paths
//      (the grant, another gateway route, a "..", an encoded path) is
//      forwarded;
//  16  a request without the capsule's relay secret is served, or a route
//      with no secret serves anything;
//  17  the gateway bearer reaches the capsule (the reply path never adds it);
//  18  a streamed answer is held until it ends;
//  19  CONNECT to the route on 443, or to api.anthropic.com, is served.

const (
	relayRouteSecret = "choir-relay-FAKE-route-secret-0123456789abcdef"
	relayRouteBearer = "gateway-bearer-FAKE-route-0123456789"
)

var relayRouteGateway = netip.MustParseAddrPort("10.200.0.1:8084")

// relayGateway is a stand-in for the gateway: it records each request and
// answers with a body, or with a stream it sends in two halves.
type relayGateway struct {
	l       net.Listener
	mu      sync.Mutex
	seen    []*http.Request
	bodies  []string
	stream  bool
	release chan struct{}
}

func newRelayGateway(t *testing.T) *relayGateway {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	g := &relayGateway{l: l, release: make(chan struct{})}
	t.Cleanup(func() { l.Close() })
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go g.serve(c)
		}
	}()
	return g
}

func (g *relayGateway) serve(c net.Conn) {
	defer c.Close()
	req, err := http.ReadRequest(bufio.NewReader(c))
	if err != nil {
		return
	}
	body, _ := io.ReadAll(req.Body)
	g.mu.Lock()
	g.seen = append(g.seen, req)
	g.bodies = append(g.bodies, string(body))
	stream := g.stream
	g.mu.Unlock()
	if stream {
		io.WriteString(c, "HTTP/1.1 200 OK\r\nContent-Type: text/event-stream\r\nConnection: close\r\n\r\nevent: one\ndata: {}\n\n")
		select {
		case <-g.release:
		case <-time.After(10 * time.Second):
		}
		io.WriteString(c, "event: two\ndata: {}\n\n")
		return
	}
	reply := "relayed " + req.Method + " " + req.URL.RequestURI()
	fmt.Fprintf(c, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s", len(reply), reply)
}

func (g *relayGateway) requests() ([]*http.Request, []string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]*http.Request(nil), g.seen...), append([]string(nil), g.bodies...)
}

func relayRouteHarness(t *testing.T, route *ModelRelayRoute) (*egressHarness, *relayGateway) {
	t.Helper()
	gw := newRelayGateway(t)
	policy := ClaudeEgressPolicy()
	policy.ModelRelay = route
	h := newEgressHarness(t, policy)
	// The route dials the gateway address; the harness sends it to the stand-in.
	h.proxy.dial = func(ctx context.Context, network, address string) (net.Conn, error) {
		h.mu.Lock()
		h.dialed = append(h.dialed, address)
		h.mu.Unlock()
		var d net.Dialer
		return d.DialContext(ctx, network, gw.l.Addr().String())
	}
	return h, gw
}

func defaultRelayRoute() *ModelRelayRoute {
	return &ModelRelayRoute{Addr: relayRouteGateway, Secret: relayRouteSecret, Bearer: func() string { return relayRouteBearer }}
}

func relayPost(path, auth, extra, body string) string {
	return "POST http://anthropic.choir.internal" + path + " HTTP/1.1\r\nHost: anthropic.choir.internal\r\n" + auth +
		"Content-Type: application/json\r\n" + extra + fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body)) + body
}

func TestModelRelayRouteForwardsOnlyTheRelayCall(t *testing.T) {
	h, gw := relayRouteHarness(t, defaultRelayRoute())
	body := `{"model":"claude-sonnet-5-5","messages":[]}`
	status, reply := sendRaw(t, h.addr, relayPost("/v1/messages?beta=true", "Authorization: Bearer "+relayRouteSecret+"\r\n",
		"X-Api-Key: sk-ant-should-not-pass\r\nCookie: a=b\r\nProxy-Authorization: Basic eA==\r\nAnthropic-Beta: claude-code-20250219\r\nUser-Agent: claude-cli/2.1.0 (external, sdk-cli)\r\nX-Choir-Capsule: someone-else\r\nX-Stainless-Os: Linux\r\n", body))
	if status != http.StatusOK || reply != "relayed POST /provider/anthropic/v1/messages?beta=true" {
		t.Fatalf("relayed = %d %q", status, reply)
	}
	if d := h.dials(); len(d) != 1 || d[0] != relayRouteGateway.String() {
		t.Fatalf("dialed %v, want only the gateway %s", d, relayRouteGateway)
	}
	seen, bodies := gw.requests()
	got := seen[0]
	if got.Header.Get("Authorization") != "Bearer "+relayRouteBearer || len(got.Header.Values("Authorization")) != 1 {
		t.Fatalf("gateway authorization = %v, want the computer's bearer once", got.Header.Values("Authorization"))
	}
	for _, name := range []string{"X-Api-Key", "Cookie", "Proxy-Authorization", "X-Stainless-Os"} {
		if v := got.Header.Get(name); v != "" {
			t.Errorf("gateway received %s: %q", name, v)
		}
	}
	for k, vs := range got.Header {
		for _, v := range vs {
			if strings.Contains(v, relayRouteSecret) {
				t.Errorf("header %s carries the relay secret", k)
			}
		}
	}
	if got.Header.Get(CapsuleHeader) != "capsule-test" || got.Header.Get("Anthropic-Beta") != "claude-code-20250219" || got.Header.Get("User-Agent") != "claude-cli/2.1.0 (external, sdk-cli)" {
		t.Fatalf("gateway headers = %v", got.Header)
	}
	if bodies[0] != body {
		t.Fatalf("body = %q", bodies[0])
	}
	// x-api-key carries the secret too (ANTHROPIC_API_KEY clients).
	if status, _ := sendRaw(t, h.addr, relayPost("/v1/messages/count_tokens", "X-Api-Key: "+relayRouteSecret+"\r\n", "", "{}")); status != http.StatusOK {
		t.Fatalf("x-api-key secret = %d", status)
	}
	if status, reply := sendRaw(t, h.addr, "GET http://anthropic.choir.internal/v1/models HTTP/1.1\r\nHost: anthropic.choir.internal\r\nAuthorization: Bearer "+relayRouteSecret+"\r\n\r\n"); status != http.StatusOK || reply != "relayed GET /provider/anthropic/v1/models" {
		t.Fatalf("models = %d %q", status, reply)
	}
	if strings.Contains(reply, relayRouteBearer) {
		t.Fatal("the gateway bearer reached the capsule")
	}
	recs := h.waitRecords(3)
	for _, rec := range recs {
		if raw := fmt.Sprintf("%+v", rec); strings.Contains(raw, relayRouteBearer) || strings.Contains(raw, relayRouteSecret) {
			t.Fatalf("the ledger carries a secret: %s", raw)
		}
	}
	if recs[0].Host != ModelRelayRouteHost || recs[0].Outcome != EgressOutcomeOK || recs[0].IP != "10.200.0.1" || recs[0].BytesUp == 0 || recs[0].BytesDown == 0 {
		t.Fatalf("ledger = %+v", recs[0])
	}
}

func TestModelRelayRouteRefusesBeforeTheGateway(t *testing.T) {
	h, gw := relayRouteHarness(t, defaultRelayRoute())
	secret := "Authorization: Bearer " + relayRouteSecret + "\r\n"
	for name, tc := range map[string]struct {
		raw  string
		want int
	}{
		"no secret":         {relayPost("/v1/messages", "", "", "{}"), http.StatusUnauthorized},
		"wrong secret":      {relayPost("/v1/messages", "Authorization: Bearer "+relayRouteSecret+"x\r\n", "", "{}"), http.StatusUnauthorized},
		"empty bearer":      {relayPost("/v1/messages", "Authorization: Bearer \r\n", "", "{}"), http.StatusUnauthorized},
		"the grant":         {"GET http://anthropic.choir.internal/grant HTTP/1.1\r\nHost: anthropic.choir.internal\r\n" + secret + "\r\n", http.StatusNotFound},
		"gateway path":      {relayPost("/provider/anthropic/v1/messages", secret, "", "{}"), http.StatusNotFound},
		"dot dot":           {relayPost("/v1/messages/../batches", secret, "", "{}"), http.StatusNotFound},
		"encoded":           {relayPost("/v1/messages%2F..%2Fbatches", secret, "", "{}"), http.StatusNotFound},
		"batches":           {relayPost("/v1/messages/batches", secret, "", "{}"), http.StatusNotFound},
		"wrong method":      {"GET http://anthropic.choir.internal/v1/messages HTTP/1.1\r\nHost: anthropic.choir.internal\r\n" + secret + "\r\n", http.StatusMethodNotAllowed},
		"delete":            {"DELETE http://anthropic.choir.internal/v1/models HTTP/1.1\r\nHost: anthropic.choir.internal\r\n" + secret + "\r\n", http.StatusMethodNotAllowed},
		"other port":        {"POST http://anthropic.choir.internal:8084/v1/messages HTTP/1.1\r\nHost: anthropic.choir.internal\r\n" + secret + "Content-Length: 2\r\n\r\n{}", http.StatusForbidden},
		"too large":         {"POST http://anthropic.choir.internal/v1/messages HTTP/1.1\r\nHost: anthropic.choir.internal\r\n" + secret + fmt.Sprintf("Content-Length: %d\r\n\r\n", modelRelayMaxBody+1), http.StatusRequestEntityTooLarge},
		"connect 443":       {"CONNECT anthropic.choir.internal:443 HTTP/1.1\r\nHost: anthropic.choir.internal:443\r\n\r\n", http.StatusForbidden},
		"post to the world": {"POST http://example.com/v1/messages HTTP/1.1\r\nHost: example.com\r\n" + secret + "Content-Length: 2\r\n\r\n{}", http.StatusMethodNotAllowed},
	} {
		if status, _ := sendRaw(t, h.addr, tc.raw); status != tc.want {
			t.Errorf("%s = %d, want %d", name, status, tc.want)
		}
	}
	if seen, _ := gw.requests(); len(seen) != 0 || len(h.dials()) != 0 {
		t.Fatalf("a refused request reached the gateway: %d requests, dials %v", len(seen), h.dials())
	}
	// A route with no secret, no bearer or no address serves nothing.
	for name, route := range map[string]*ModelRelayRoute{
		"no route":   nil,
		"no secret":  {Addr: relayRouteGateway, Bearer: func() string { return relayRouteBearer }},
		"no bearer":  {Addr: relayRouteGateway, Secret: relayRouteSecret},
		"no address": {Secret: relayRouteSecret, Bearer: func() string { return relayRouteBearer }},
	} {
		h, gw := relayRouteHarness(t, route)
		for _, raw := range []string{relayPost("/v1/messages", "", "", "{}"), relayPost("/v1/messages", "Authorization: Bearer \r\n", "", "{}"), relayPost("/v1/messages", secret, "", "{}")} {
			if status, _ := sendRaw(t, h.addr, raw); status == http.StatusOK {
				t.Errorf("%s: served", name)
			}
		}
		if seen, _ := gw.requests(); len(seen) != 0 {
			t.Errorf("%s: reached the gateway", name)
		}
	}
	// An empty bearer at call time refuses too.
	h2, gw2 := relayRouteHarness(t, &ModelRelayRoute{Addr: relayRouteGateway, Secret: relayRouteSecret, Bearer: func() string { return "" }})
	if status, _ := sendRaw(t, h2.addr, relayPost("/v1/messages", secret, "", "{}")); status != http.StatusServiceUnavailable {
		t.Fatalf("no bearer now = %d", status)
	}
	if seen, _ := gw2.requests(); len(seen) != 0 {
		t.Fatal("an empty bearer reached the gateway")
	}
}

func TestModelRelayRouteServesATunneledRequestOnce(t *testing.T) {
	h, gw := relayRouteHarness(t, defaultRelayRoute())
	conn, err := net.Dial("tcp", h.addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	io.WriteString(conn, "CONNECT anthropic.choir.internal:80 HTTP/1.1\r\nHost: anthropic.choir.internal:80\r\n\r\n")
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, nil)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("CONNECT :80 = %v %v", resp, err)
	}
	// Two requests on the tunnel: only the first is relayed; the tunnel
	// closes after its answer.
	io.WriteString(conn, "POST /v1/messages HTTP/1.1\r\nHost: anthropic.choir.internal\r\nAuthorization: Bearer "+relayRouteSecret+"\r\nContent-Length: 2\r\n\r\n{}"+
		"POST /v1/messages/batches HTTP/1.1\r\nHost: anthropic.choir.internal\r\nContent-Length: 2\r\n\r\n{}")
	inner, err := http.ReadResponse(br, nil)
	if err != nil {
		t.Fatal(err)
	}
	reply, _ := io.ReadAll(inner.Body)
	if inner.StatusCode != http.StatusOK || string(reply) != "relayed POST /provider/anthropic/v1/messages" {
		t.Fatalf("tunneled = %d %q", inner.StatusCode, reply)
	}
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := http.ReadResponse(br, nil); err == nil {
		t.Fatal("a second request on the tunnel was answered")
	}
	if seen, _ := gw.requests(); len(seen) != 1 {
		t.Fatalf("the gateway saw %d requests, want 1", len(seen))
	}
}

func TestModelRelayRouteStreamsTheAnswer(t *testing.T) {
	h, gw := relayRouteHarness(t, defaultRelayRoute())
	gw.stream = true
	conn, err := net.Dial("tcp", h.addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	io.WriteString(conn, relayPost("/v1/messages", "Authorization: Bearer "+relayRouteSecret+"\r\n", "Accept: text/event-stream\r\n", "{}"))
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, nil)
	if err != nil {
		t.Fatal(err)
	}
	first := make([]byte, len("event: one\ndata: {}\n\n"))
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err := io.ReadFull(resp.Body, first); err != nil || string(first) != "event: one\ndata: {}\n\n" {
		t.Fatalf("the first event did not arrive before the stream ended: %q %v", first, err)
	}
	close(gw.release)
	rest, _ := io.ReadAll(resp.Body)
	if string(rest) != "event: two\ndata: {}\n\n" {
		t.Fatalf("rest = %q", rest)
	}
}
