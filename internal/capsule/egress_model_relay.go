package capsule

import (
	"bufio"
	"bytes"
	"context"
	"crypto/subtle"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"regexp"
	"strings"
	"time"
)

// The model relay route (docs/problems/claude-delegate-token-on-three-
// computers-2026-11-03.md, design section, "The guest half"). A claude-mode
// capsule's claude process calls ModelRelayRouteHost as its Anthropic base
// URL. The capsule's egress proxy, outside the capsule, serves that name
// itself: only the gateway relay's three paths, with the capsule's relay
// secret, a bounded body, and nothing the caller sent that authenticates.
// It adds this computer's gateway bearer and forwards to the gateway's
// /provider/anthropic/, where the credential is. The answer is spliced back
// as it arrives, so a server-sent-event stream is never held.

// ModelRelayRouteHost is the name; it is an egress route, not a DNS name.
const ModelRelayRouteHost = "anthropic.choir.internal"

const (
	modelRelayGatewayPrefix = "/provider/anthropic"
	modelRelayMaxBody       = 32 << 20 // the Messages API's own request limit
	modelRelayReadTimeout   = 2 * time.Minute
)

// ModelRelayRoute is the route's configuration, from the executor only.
type ModelRelayRoute struct {
	// Addr is the gateway (the guest's tap peer).
	Addr netip.AddrPort
	// Secret is the capsule's relay secret (NewClaudeRelaySecret).
	Secret string
	// Bearer is this computer's gateway bearer, read when each call is made.
	Bearer func() string
}

// modelRelayPaths are the relay paths and their methods: what `claude -p`
// needs, as the gateway serves them (internal/gateway relayRoutes).
var modelRelayPaths = map[string]string{
	"/v1/messages":              http.MethodPost,
	"/v1/messages/count_tokens": http.MethodPost,
	"/v1/models":                http.MethodGet,
}

// modelRelayHeaders are the only request headers forwarded.
var modelRelayHeaders = []string{"Content-Type", "Accept", "Anthropic-Version", "Anthropic-Beta", "User-Agent", "X-App"}

var modelRelayHeaderText = regexp.MustCompile(`^[A-Za-z0-9._,:;/()+=* -]*$`)

// requestTargetHost is the host a proxy request names: CONNECT's authority,
// or an absolute URL's host.
func requestTargetHost(req *http.Request) string {
	var host string
	switch {
	case req.Method == http.MethodConnect:
		h, _, err := net.SplitHostPort(req.Host)
		if err != nil {
			h = req.Host
		}
		host = h
	case req.URL != nil:
		host = req.URL.Hostname()
	}
	return strings.TrimSuffix(strings.ToLower(host), ".")
}

// relaySecretFrom reads the secret a client presents: Authorization
// "Bearer <secret>" (ANTHROPIC_AUTH_TOKEN) or x-api-key.
func relaySecretFrom(h http.Header) string {
	if auth := strings.TrimSpace(h.Get("Authorization")); len(auth) > 7 && strings.EqualFold(auth[:7], "bearer ") {
		return strings.TrimSpace(auth[7:])
	}
	return strings.TrimSpace(h.Get("X-Api-Key"))
}

func (p *EgressProxy) serveModelRelay(client net.Conn, br *bufio.Reader, req *http.Request, rec *EgressRecord, finish func(string, string), refuse func(int, string)) {
	rec.Host, rec.Port = ModelRelayRouteHost, 80
	route := p.policy.ModelRelay
	if route == nil || !route.Addr.IsValid() || route.Secret == "" || route.Bearer == nil {
		refuse(http.StatusForbidden, "this capsule has no model relay route")
		return
	}
	inner := req
	if req.Method == http.MethodConnect {
		// A client that tunnels even plain HTTP: a tunnel to port 80 of this
		// one name, then one origin-form request inside it.
		if _, port, err := net.SplitHostPort(req.Host); err != nil || port != "80" {
			refuse(http.StatusForbidden, ModelRelayRouteHost+" serves plain HTTP on port 80 only")
			return
		}
		if _, err := io.WriteString(client, "HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
			finish(EgressOutcomeFailed, "client went away")
			return
		}
		_ = client.SetReadDeadline(time.Now().Add(30 * time.Second))
		var err error
		inner, err = http.ReadRequest(br)
		_ = client.SetReadDeadline(time.Time{})
		if err != nil {
			refuse(http.StatusBadRequest, "malformed request in the model relay tunnel")
			return
		}
		rec.Method = "CONNECT+" + inner.Method
	} else {
		if req.URL == nil || req.URL.Scheme != "http" || (req.URL.Port() != "" && req.URL.Port() != "80") {
			refuse(http.StatusForbidden, ModelRelayRouteHost+" serves plain HTTP on port 80 only")
			return
		}
	}
	path := ""
	if inner.URL != nil {
		path = inner.URL.Path
	}
	method, ok := modelRelayPaths[path]
	if !ok || inner.URL.RawPath != "" {
		refuse(http.StatusNotFound, "the model relay serves only the Messages API paths")
		return
	}
	if inner.Method != method {
		refuse(http.StatusMethodNotAllowed, "the model relay serves "+path+" by "+method+" only")
		return
	}
	// The secret before any byte goes to the gateway: a cell in the capsule
	// that lacks it is refused here.
	if got := relaySecretFrom(inner.Header); subtle.ConstantTimeCompare([]byte(got), []byte(route.Secret)) != 1 {
		refuse(http.StatusUnauthorized, "the model relay needs this capsule's relay secret")
		return
	}
	bearer := strings.TrimSpace(route.Bearer())
	if bearer == "" {
		refuse(http.StatusServiceUnavailable, "this computer has no gateway credential")
		return
	}
	if reason := p.admit(); reason != "" {
		fmt.Fprintf(client, "HTTP/1.1 429 Too Many Requests\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s\n", len(reason)+1, reason)
		finish(EgressOutcomeBudget, reason)
		return
	}
	var body []byte
	if method == http.MethodPost {
		if inner.ContentLength > modelRelayMaxBody {
			refuse(http.StatusRequestEntityTooLarge, "request body is too large")
			return
		}
		_ = client.SetReadDeadline(time.Now().Add(modelRelayReadTimeout))
		var err error
		body, err = io.ReadAll(io.LimitReader(inner.Body, modelRelayMaxBody+1))
		_ = client.SetReadDeadline(time.Time{})
		if err != nil {
			refuse(http.StatusBadRequest, "could not read the request body")
			return
		}
		if len(body) > modelRelayMaxBody {
			refuse(http.StatusRequestEntityTooLarge, "request body is too large")
			return
		}
	}
	query := ""
	if inner.URL.Query().Get("beta") == "true" {
		query = "?beta=true"
	}
	target := "http://" + route.Addr.String() + modelRelayGatewayPrefix + path + query
	up, err := http.NewRequest(method, target, bytes.NewReader(body))
	if err != nil {
		finish(EgressOutcomeFailed, "request rewrite failed")
		return
	}
	if method != http.MethodPost {
		up.Body, up.ContentLength = nil, 0
	}
	for _, name := range modelRelayHeaders {
		if v := strings.TrimSpace(inner.Header.Get(name)); v != "" && len(v) <= 1024 && modelRelayHeaderText.MatchString(v) {
			up.Header.Set(name, v)
		}
	}
	up.Header.Set("Authorization", "Bearer "+bearer)
	up.Header.Set(CapsuleHeader, p.capsuleID)
	up.Header.Set("Accept-Encoding", "identity")
	up.Close = true

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	rec.IP = route.Addr.Addr().String()
	upstream, err := p.dial(ctx, "tcp", route.Addr.String())
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
	var head bytes.Buffer
	if err := up.Write(&head); err != nil {
		finish(EgressOutcomeFailed, "request rewrite failed")
		return
	}
	if !p.charge(true, int64(head.Len())) {
		finish(EgressOutcomeBudget, "upload budget exceeded")
		return
	}
	rec.BytesUp += int64(head.Len())
	if _, err := upstream.Write(head.Bytes()); err != nil {
		finish(EgressOutcomeFailed, "upstream write failed")
		return
	}
	// The gateway closes after its answer (Connection: close); the answer
	// goes back byte for byte as it arrives.
	outcome := p.splice(client, bytes.NewReader(nil), upstream, upstream, rec, false)
	finish(outcome, map[string]string{EgressOutcomeBudget: "byte budget exceeded"}[outcome])
}
