// Package hostauth is the host-internal service credential: a secret that
// a host service must present, in addition to arriving over loopback or a
// unix socket, before an internal route (vmctl, the gateway's credential
// and operator routes, corpusd, maild, pprof) treats it as trusted.
//
// Before it, "came from 127.0.0.1" was the credential, and every process in
// the host network namespace held it: the Firecracker VMMs, SearXNG, the Go
// module proxy, Qdrant, Caddy
// (docs/problems/loopback-is-a-credential-2026-10-14.md).
//
// The secret is one root-only 0600 file delivered to each unit with
// systemd LoadCredential= under the name CredentialName, so it appears only
// in that unit's private $CREDENTIALS_DIRECTORY; it is never put in the
// environment (VMMs inherit vmctl's environment) and never logged.
//
// Rollout is a mode (CHOIR_INTERNAL_AUTH_MODE):
//
//	off      legacy: transport alone decides (the default when unset, and the
//	         rollback)
//	observe  a missing or wrong credential is logged ("would refuse") and
//	         allowed, so callers can be found before anything breaks
//	enforce  a missing or wrong credential is refused; with no credential
//	         loaded every internal call is refused (fail closed)
//
// Servers strip the token header from every request at ingress
// (Middleware), so no reverse proxy can forward it to a guest; clients
// attach it only to loopback destinations (MarkInternal).
package hostauth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	// Header carries the credential on host-internal requests.
	Header = "X-Choir-Internal-Token"
	// MarkerHeader is the older host-caller marker. It is a hint, never
	// authority: any caller can send it.
	MarkerHeader = "X-Internal-Caller"
	// CredentialName is the systemd credential name (LoadCredential=).
	CredentialName = "choir-internal-token"
	// EnvMode selects off, observe or enforce.
	EnvMode = "CHOIR_INTERNAL_AUTH_MODE"
	// EnvTokenFile overrides the credential path (CI shells, tests);
	// otherwise $CREDENTIALS_DIRECTORY/CredentialName is read.
	EnvTokenFile = "CHOIR_INTERNAL_TOKEN_FILE"
	// EnvPorts overrides the loopback ports MarkInternal sends the
	// credential to (comma-separated). The default is the host services
	// that check it: vmctl 8083, gateway 8084, corpusd 8086, maild 8087.
	// Node B also DNATs 127.0.0.1:9xxx to guests, so "any loopback port"
	// would hand the secret to a guest.
	EnvPorts = "CHOIR_INTERNAL_AUTH_PORTS"

	// minTokenLen refuses a weak secret: 32 bytes of hex or base64 at least.
	minTokenLen = 32
	// logInterval bounds refusal logging to one line per route and reason.
	logInterval  = 5 * time.Minute
	maxLogRoutes = 1024
)

// Mode is the rollout mode.
type Mode string

const (
	ModeOff     Mode = "off"
	ModeObserve Mode = "observe"
	ModeEnforce Mode = "enforce"
)

type config struct {
	mode     Mode
	raw      string
	digest   [sha256.Size]byte
	hasToken bool
	loadErr  error
	source   string
	modeNote string
	ports    map[string]bool
}

// Status describes the configuration without the secret.
func (c *config) Status() string {
	cred := "credential loaded from " + c.source
	if !c.hasToken {
		cred = "no credential"
		if c.loadErr != nil {
			cred += " (" + c.loadErr.Error() + ")"
		}
	}
	s := fmt.Sprintf("mode=%s, %s", c.mode, cred)
	if c.modeNote != "" {
		s += "; " + c.modeNote
	}
	return s
}

var (
	loadOnce sync.Once
	cfgMu    sync.RWMutex
	cfg      *config
)

func current() *config {
	loadOnce.Do(func() {
		c := loadFromEnv()
		cfgMu.Lock()
		if cfg == nil {
			cfg = c
		}
		cfgMu.Unlock()
	})
	cfgMu.RLock()
	defer cfgMu.RUnlock()
	return cfg
}

func loadFromEnv() *config {
	c := loadConfig(EnvMode, EnvTokenFile, CredentialName)
	c.ports = portsFromEnv()
	return c
}

// loadConfig reads one credential's mode and secret: the internal
// credential, or the narrower issuer credential (issuer.go).
func loadConfig(envMode, envTokenFile, credentialName string) *config {
	c := &config{}
	rawMode := strings.ToLower(strings.TrimSpace(os.Getenv(envMode)))
	switch Mode(rawMode) {
	case "", ModeOff:
		c.mode = ModeOff
	case ModeObserve:
		c.mode = ModeObserve
	case ModeEnforce:
		c.mode = ModeEnforce
	default:
		// Fail closed on a typo rather than open.
		c.mode = ModeEnforce
		c.modeNote = fmt.Sprintf("unknown %s %q treated as enforce", envMode, rawMode)
	}
	path := strings.TrimSpace(os.Getenv(envTokenFile))
	if path == "" {
		if dir := strings.TrimSpace(os.Getenv("CREDENTIALS_DIRECTORY")); dir != "" {
			path = filepath.Join(dir, credentialName)
		}
	}
	if path == "" {
		c.loadErr = fmt.Errorf("neither %s nor $CREDENTIALS_DIRECTORY/%s is set", envTokenFile, credentialName)
		return c
	}
	c.source = path
	data, err := os.ReadFile(path)
	if err != nil {
		c.loadErr = fmt.Errorf("read %s: %w", path, err)
		return c
	}
	token := strings.TrimSpace(string(data))
	if len(token) < minTokenLen {
		c.loadErr = fmt.Errorf("credential in %s is shorter than %d bytes", path, minTokenLen)
		return c
	}
	c.setToken(token)
	return c
}

func (c *config) setToken(token string) {
	c.raw = token
	c.digest = sha256.Sum256([]byte(token))
	c.hasToken = true
}

// SetForTesting installs a mode and token for the duration of a test and
// returns the restore function. An empty token means no credential loaded.
func SetForTesting(mode Mode, token string) (restore func()) {
	current() // settle the lazy load so it cannot overwrite the override
	c := &config{mode: mode, source: "test", ports: portsFromEnv()}
	if token != "" {
		c.setToken(token)
	} else {
		c.loadErr = fmt.Errorf("no test credential")
	}
	cfgMu.Lock()
	prev := cfg
	cfg = c
	cfgMu.Unlock()
	resetLogLimiter()
	return func() {
		cfgMu.Lock()
		cfg = prev
		cfgMu.Unlock()
		resetLogLimiter()
	}
}

// LogStartup records the configuration once per process. It is silent for
// the legacy configuration (off, no credential), which is every guest
// service and every local development process.
func LogStartup(service string) {
	c := current()
	if c.mode == ModeOff && !c.hasToken && c.modeNote == "" {
		return
	}
	log.Printf("%s: hostauth: internal routes %s", service, c.Status())
	if ic := currentIssuer(); ic.mode != ModeOff || ic.hasToken {
		log.Printf("%s: hostauth: issuer credential %s", service, ic.Status())
	}
	if c.mode == ModeEnforce && !c.hasToken {
		log.Printf("%s: hostauth: enforce without a credential: every internal call will be refused (fail closed)", service)
	}
}

type verdict int

const (
	verdictNone verdict = iota // never passed Middleware
	verdictValid
	verdictMissing
	verdictWrong
	verdictAmbiguous
)

func (v verdict) reason() string {
	switch v {
	case verdictValid:
		return "valid credential"
	case verdictMissing:
		return "missing credential"
	case verdictWrong:
		return "wrong credential"
	case verdictAmbiguous:
		return "more than one credential header"
	default:
		return "request did not pass the credential middleware"
	}
}

type ctxKey struct{}

// Middleware checks the credential once per request, records the verdict
// in the request context, and always removes the header so nothing
// downstream (a reverse proxy to a guest, a log of headers) can see it.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		values := r.Header.Values(Header)
		v := verdictMissing
		switch {
		case len(values) > 1:
			v = verdictAmbiguous
		case len(values) == 1:
			v = verdictWrong
			if c := current(); c.hasToken {
				presented := sha256.Sum256([]byte(strings.TrimSpace(values[0])))
				if subtle.ConstantTimeCompare(presented[:], c.digest[:]) == 1 {
					v = verdictValid
				}
			}
		}
		if len(values) > 0 {
			r.Header.Del(Header)
		}
		ctx := context.WithValue(r.Context(), ctxKey{}, v)
		ctx = withIssuerVerdict(ctx, issuerVerdict(r))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Authorize is the credential half of an internal-route gate. Call it only
// after the transport check (loopback or unix socket) has passed, so guest
// requests never produce "would refuse" noise. It returns false only in
// enforce mode.
func Authorize(r *http.Request) bool {
	return AuthorizeAtMost(r, ModeEnforce)
}

// AuthorizeAtMost is Authorize with the configured mode capped for one
// route: a route newly put behind the credential can run observe while the
// service enforces everywhere else (SEC-11). It never raises the mode: a
// service in observe or off stays there.
func AuthorizeAtMost(r *http.Request, routeCap Mode) bool {
	if r == nil {
		return false
	}
	c := current()
	mode := c.mode
	if mode == ModeEnforce && routeCap == ModeObserve {
		mode = ModeObserve
	}
	if mode == ModeOff || routeCap == ModeOff {
		return true
	}
	v, _ := r.Context().Value(ctxKey{}).(verdict)
	if v == verdictValid && c.hasToken {
		return true
	}
	reason := v.reason()
	if !c.hasToken {
		reason = "no credential loaded on this service"
		if c.loadErr != nil {
			reason += " (" + c.loadErr.Error() + ")"
		}
	}
	route := r.Pattern
	if route == "" && r.URL != nil {
		route = r.URL.Path
	}
	if mode == ModeObserve {
		logLimited("observe: would refuse", r, route, reason)
		return true
	}
	logLimited("enforce: refused", r, route, reason)
	return false
}

// MarkInternal marks an outbound request as a host-internal call: the
// marker header always, and the credential only when the destination is a
// loopback host on an internal service port, so the secret is never sent to
// a guest (directly, or through a 127.0.0.1 DNAT port) or the internet.
func MarkInternal(req *http.Request) {
	if req == nil {
		return
	}
	req.Header.Set(MarkerHeader, "true")
	req.Header.Del(Header)
	if req.URL == nil || !loopbackHost(req.URL.Hostname()) {
		return
	}
	if c := current(); c.hasToken && c.ports[req.URL.Port()] {
		req.Header.Set(Header, c.raw)
	}
}

// 8082 is the proxy: its object-graph route needs the credential from host
// callers (SEC-11), and its Middleware strips the header at ingress like
// every other service.
// Every port here must stay below vmctl's guest host-port base
// (VM_HOST_BASE_PORT, 9000): vmctl DNATs 127.0.0.1:<host port> to a guest,
// so a credential port in that range would hand the secret to a guest.
// TestCredentialPortsNeverInGuestHostPortRange holds it.
var defaultPorts = []string{"8082", "8083", "8084", "8086", "8087"}

func portsFromEnv() map[string]bool {
	list := defaultPorts
	if raw := strings.TrimSpace(os.Getenv(EnvPorts)); raw != "" {
		list = strings.Split(raw, ",")
	}
	ports := map[string]bool{}
	for _, p := range list {
		if p = strings.TrimSpace(p); p != "" {
			ports[p] = true
		}
	}
	return ports
}

// loopbackHost reports whether a URL host names this machine's loopback.
func loopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

type logEntry struct {
	last       time.Time
	suppressed int
}

var (
	logMu     sync.Mutex
	logRoutes = map[string]*logEntry{}
)

func resetLogLimiter() {
	logMu.Lock()
	logRoutes = map[string]*logEntry{}
	logMu.Unlock()
}

func logLimited(prefix string, r *http.Request, route, reason string) {
	key := prefix + "|" + route + "|" + reason
	now := time.Now()
	logMu.Lock()
	e := logRoutes[key]
	if e != nil && now.Sub(e.last) < logInterval {
		e.suppressed++
		logMu.Unlock()
		return
	}
	suppressed := 0
	if e != nil {
		suppressed = e.suppressed
	}
	if len(logRoutes) >= maxLogRoutes {
		logRoutes = map[string]*logEntry{}
	}
	logRoutes[key] = &logEntry{last: now}
	logMu.Unlock()
	extra := ""
	if suppressed > 0 {
		extra = fmt.Sprintf(" (%d more since the last line)", suppressed)
	}
	header, credential := Header, CredentialName
	if strings.HasPrefix(reason, "issuer:") {
		header, credential = IssuerHeader, IssuerCredentialName
	}
	log.Printf("hostauth: %s internal call %s %s from %s: %s; send %s from %s (docs/problems/loopback-is-a-credential-2026-10-14.md)%s",
		prefix, r.Method, route, r.RemoteAddr, reason, header, credential, extra)
}

// IsDocumentationNetwork reports whether ip is in one of the RFC 5737
// documentation ranges (192.0.2.0/24, 198.51.100.0/24, 203.0.113.0/24).
// httptest.NewRequest uses 192.0.2.1 as RemoteAddr.
func IsDocumentationNetwork(ip net.IP) bool {
	ip4 := ip.To4()
	if ip4 == nil {
		return false
	}
	return (ip4[0] == 192 && ip4[1] == 0 && ip4[2] == 2) ||
		(ip4[0] == 198 && ip4[1] == 51 && ip4[2] == 100) ||
		(ip4[0] == 203 && ip4[1] == 0 && ip4[2] == 113)
}

func documentationNetworkAllowed(ip net.IP, inTestBinary bool) bool {
	return inTestBinary && IsDocumentationNetwork(ip)
}

// TestNetworkAllowed reports whether a request source (an address or
// host:port) is in TEST-NET-1 (192.0.2.0/24, httptest.NewRequest's
// RemoteAddr) AND this process is a Go test binary. The internal-route
// gates (vmctl, corpusd, maild) used to admit TEST-NET-1 as a test
// convenience in production; now a production binary never does
// (docs/problems/loopback-is-a-credential-2026-10-14.md). The other two
// documentation ranges stay "outside callers" in tests, as before.
func TestNetworkAllowed(remote string) bool {
	host := strings.TrimSpace(remote)
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return testNet1Allowed(net.ParseIP(host), testing.Testing())
}

func testNet1Allowed(ip net.IP, inTestBinary bool) bool {
	ip4 := ip.To4()
	return inTestBinary && ip4 != nil && ip4[0] == 192 && ip4[1] == 0 && ip4[2] == 2
}
