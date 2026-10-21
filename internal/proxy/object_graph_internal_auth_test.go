package proxy

import (
	"crypto/ed25519"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/hostauth"
)

// The proxy's object-graph route and loopback (SEC-11;
// docs/problems/loopback-is-a-credential-2026-10-14.md, residual "The
// proxy's object-graph route still trusts loopback").
//
// Ways this could fail, written before the code:
//
//	P1 a loopback caller with X-Internal-Caller and no host credential
//	   reaches corpusd's internal authority in enforce;
//	P2 a wrong or doubled credential passes;
//	P3 the valid credential is refused (host callers break);
//	P4 the route's observe mode refuses (no observe window), or its enforce
//	   mode is weakened by observe elsewhere being read as "allow";
//	P5 local development (hostauth off) loses the loopback route;
//	P6 a production binary admits a TEST-NET-1 (192.0.2.*) peer as internal;
//	P7 an unknown route mode is read as observe (fail open);
//	P8 a loopback request without the marker header passes.

const objectGraphTestToken = "proxy-object-graph-test-token-0123456789abcdef"

func objectGraphGate(t *testing.T, mode hostauth.Mode, remote string, headers map[string][]string) bool {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	h, err := NewHandler(&Config{AllowDirectAutoputerForTests: true, Port: "0", ComputerURL: "http://127.0.0.1:1",
		AuthPublicKeyPath: "/unused/in/test", CorpusdURL: "http://127.0.0.1:1", ObjectGraphInternalAuth: mode}, pub)
	if err != nil {
		t.Fatal(err)
	}
	var passed bool
	req := httptest.NewRequest(http.MethodGet, "/internal/platform/objects/x", nil)
	req.RemoteAddr = remote
	for k, vs := range headers {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	hostauth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		passed = h.requirePlatformObjectGraphCaller(r) == nil
	})).ServeHTTP(httptest.NewRecorder(), req)
	return passed
}

func marker(token ...string) map[string][]string {
	h := map[string][]string{hostauth.MarkerHeader: {"true"}}
	if len(token) > 0 {
		h[hostauth.Header] = token
	}
	return h
}

func TestObjectGraphLoopbackNeedsHostCredential(t *testing.T) {
	defer hostauth.SetForTesting(hostauth.ModeEnforce, objectGraphTestToken)()
	if objectGraphGate(t, hostauth.ModeEnforce, "127.0.0.1:40000", marker()) {
		t.Fatal("P1: loopback + marker without the credential passed in enforce")
	}
	if objectGraphGate(t, hostauth.ModeEnforce, "127.0.0.1:40000", marker("wrong-wrong-wrong-wrong-wrong-wrong-wrong")) {
		t.Fatal("P2: a wrong credential passed")
	}
	if objectGraphGate(t, hostauth.ModeEnforce, "127.0.0.1:40000", marker(objectGraphTestToken, objectGraphTestToken)) {
		t.Fatal("P2: a doubled credential passed")
	}
	if !objectGraphGate(t, hostauth.ModeEnforce, "127.0.0.1:40000", marker(objectGraphTestToken)) {
		t.Fatal("P3: loopback + marker + credential was refused")
	}
	if !objectGraphGate(t, hostauth.ModeEnforce, "[::1]:40000", marker(objectGraphTestToken)) {
		t.Fatal("P3: IPv6 loopback + credential was refused")
	}
	if objectGraphGate(t, hostauth.ModeEnforce, "127.0.0.1:40000", map[string][]string{hostauth.Header: {objectGraphTestToken}}) {
		t.Fatal("P8: loopback without the marker passed")
	}
	// The zero value is enforce (fail closed).
	if objectGraphGate(t, "", "127.0.0.1:40000", marker()) {
		t.Fatal("P7: an unset route mode did not enforce")
	}
}

func TestObjectGraphRouteObserveAllowsAndLogs(t *testing.T) {
	defer hostauth.SetForTesting(hostauth.ModeEnforce, objectGraphTestToken)()
	if !objectGraphGate(t, hostauth.ModeObserve, "127.0.0.1:40000", marker()) {
		t.Fatal("P4: the route's observe mode refused")
	}
	// Observe on the route never widens beyond the marker check.
	if objectGraphGate(t, hostauth.ModeObserve, "127.0.0.1:40000", nil) {
		t.Fatal("P8: observe passed a loopback request without the marker")
	}
}

func TestObjectGraphLoopbackInLocalDevelopment(t *testing.T) {
	defer hostauth.SetForTesting(hostauth.ModeOff, "")()
	if !objectGraphGate(t, hostauth.ModeEnforce, "127.0.0.1:40000", marker()) {
		t.Fatal("P5: hostauth off (local development) lost the loopback route")
	}
}

func TestObjectGraphTestNetOnlyInTestBinaries(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/internal/platform/objects/x", nil)
	req.RemoteAddr = "192.0.2.10:1234"
	if trustedInternalProxyTransportFor(req, false) {
		t.Fatal("P6: a production binary trusts a TEST-NET-1 peer as internal")
	}
	if !trustedInternalProxyTransportFor(req, true) {
		t.Fatal("a test binary lost its TEST-NET-1 convenience")
	}
	req.RemoteAddr = "198.51.100.10:1234"
	if trustedInternalProxyTransportFor(req, true) {
		t.Fatal("TEST-NET-2 is an outside caller, in tests too")
	}
}

func TestObjectGraphInternalAuthModeFromEnv(t *testing.T) {
	for raw, want := range map[string]hostauth.Mode{
		"":         hostauth.ModeEnforce,
		"enforce":  hostauth.ModeEnforce,
		"observe":  hostauth.ModeObserve,
		" Observe": hostauth.ModeObserve,
		"off":      hostauth.ModeEnforce, // P7: the route has no legacy mode
		"obsrve":   hostauth.ModeEnforce, // P7: a typo fails closed
	} {
		t.Setenv("PROXY_OBJECT_GRAPH_INTERNAL_AUTH", raw)
		cfg, err := LoadConfig()
		if err != nil {
			t.Fatal(err)
		}
		if cfg.ObjectGraphInternalAuth != want {
			t.Errorf("PROXY_OBJECT_GRAPH_INTERNAL_AUTH=%q -> %q, want %q", raw, cfg.ObjectGraphInternalAuth, want)
		}
	}
}
