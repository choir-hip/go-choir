package hostfetch

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/sourcefetch"
)

// Ways the guest side could fail, written before the code:
//  1. a granted computer falls back to dialing directly when the gateway is
//     down, unconfigured or refuses (the route must fail closed);
//  2. redirects stop being re-checked: a hop to a private literal must be
//     refused in the guest before any gateway call, and every public hop
//     must cross the gateway again (one log line per hop);
//  3. the final URL (resp.Request) is lost, so Read reports the wrong page;
//  4. a gateway refusal or budget answer reaches the caller as an upstream
//     response instead of an error;
//  5. a body cut on the host reads as complete;
//  6. the grant turns on for an ungranted, withheld or other computer.

type testGateway struct {
	url   string
	calls atomic.Int32
}

func newTestGateway(t *testing.T, e *Executor) *testGateway {
	t.Helper()
	g := &testGateway{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.calls.Add(1)
		if r.URL.Path != Path || r.Header.Get("Authorization") != "Bearer tok" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, `{"error":"authentication required"}`)
			return
		}
		e.Serve(w, r)
	}))
	t.Cleanup(srv.Close)
	g.url = srv.URL
	return g
}

func withRoute(t *testing.T, rt http.RoundTripper) {
	t.Helper()
	prev := sourcefetch.SetRoute(rt)
	t.Cleanup(func() { sourcefetch.SetRoute(prev) })
}

func TestGuestClientRoutesEachHopThroughTheHost(t *testing.T) {
	var upstreamHits atomic.Int32
	e, _ := upstreamExecutor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHits.Add(1)
		switch r.URL.Path {
		case "/start":
			http.Redirect(w, r, "/final", http.StatusFound)
		case "/final":
			_, _ = io.WriteString(w, "landed")
		}
	}), publicIP)
	gw := newTestGateway(t, e)
	withRoute(t, &Transport{BaseURL: gw.url, Token: func() string { return "tok" }})

	resp, err := sourcefetch.Client(5 * time.Second).Get("http://example.com/start")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "landed" || resp.StatusCode != 200 {
		t.Fatalf("status %d body %q", resp.StatusCode, body)
	}
	if gw.calls.Load() != 2 || upstreamHits.Load() != 2 {
		t.Fatalf("gateway calls %d, upstream hits %d; want one each per hop", gw.calls.Load(), upstreamHits.Load())
	}
	if resp.Request == nil || resp.Request.URL.String() != "http://example.com/final" {
		t.Fatalf("final URL lost: %v", resp.Request)
	}
}

func TestGuestClientRechecksRedirectsBeforeTheGateway(t *testing.T) {
	e, _ := upstreamExecutor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://10.0.0.1/admin", http.StatusFound)
	}), publicIP)
	gw := newTestGateway(t, e)
	withRoute(t, &Transport{BaseURL: gw.url, Token: func() string { return "tok" }})
	if _, err := sourcefetch.Client(5 * time.Second).Get("http://example.com/"); err == nil {
		t.Fatal("redirect to a private literal was followed")
	}
	if gw.calls.Load() != 1 {
		t.Fatalf("gateway calls %d, want 1 (the private hop never leaves the guest)", gw.calls.Load())
	}
}

func TestGuestClientReturnsHostRefusalsAsErrors(t *testing.T) {
	e, _ := upstreamExecutor(t, http.NotFoundHandler(), "10.0.0.5")
	gw := newTestGateway(t, e)
	withRoute(t, &Transport{BaseURL: gw.url, Token: func() string { return "tok" }})
	_, err := sourcefetch.Client(5 * time.Second).Get("https://rebind.example/")
	if err == nil || !strings.Contains(err.Error(), "host fetch") || !strings.Contains(err.Error(), "forbidden") {
		t.Fatalf("err = %v", err)
	}

	withRoute(t, &Transport{BaseURL: gw.url, Token: func() string { return "wrong" }})
	if _, err := sourcefetch.Client(5 * time.Second).Get("https://example.com/"); err == nil || !strings.Contains(err.Error(), "authentication required") {
		t.Fatalf("auth refusal err = %v", err)
	}
}

func TestGuestClientNeverFallsBackToDirectEgress(t *testing.T) {
	prev := sourcefetch.SetAllowPrivateNetworkForTests(true)
	t.Cleanup(func() { sourcefetch.SetAllowPrivateNetworkForTests(prev) })
	var direct atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { direct.Add(1) }))
	t.Cleanup(upstream.Close)

	dead := httptest.NewServer(http.NotFoundHandler())
	deadURL := dead.URL
	dead.Close()
	for name, rt := range map[string]http.RoundTripper{
		"gateway down":        &Transport{BaseURL: deadURL, Token: func() string { return "tok" }, Base: http.DefaultTransport},
		"no gateway":          Refusing("granted but no gateway"),
		"empty token":         &Transport{BaseURL: deadURL, Token: func() string { return "" }},
		"transport no fields": &Transport{},
	} {
		withRoute(t, rt)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, upstream.URL, nil)
		_, err := sourcefetch.Client(5 * time.Second).Do(req)
		cancel()
		if err == nil {
			t.Fatalf("%s: fetch succeeded", name)
		}
	}
	if direct.Load() != 0 {
		t.Fatalf("a granted guest dialed the upstream directly %d times", direct.Load())
	}
}

func TestGuestClientSeesAHostAbortAsAnError(t *testing.T) {
	e, _ := upstreamExecutor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for i := 0; i < 10; i++ {
			_, _ = w.Write([]byte(strings.Repeat("c", 50)))
			w.(http.Flusher).Flush()
		}
	}), publicIP)
	e.MaxBody = 100
	gw := newTestGateway(t, e)
	withRoute(t, &Transport{BaseURL: gw.url, Token: func() string { return "tok" }})
	resp, err := sourcefetch.Client(5 * time.Second).Get("http://example.com/")
	if err == nil {
		_, err = io.ReadAll(resp.Body)
		resp.Body.Close()
	}
	if err == nil {
		t.Fatal("a body cut on the host read as complete")
	}
}

func writeGrant(t *testing.T, dir string, g Grant) {
	t.Helper()
	raw, _ := json.Marshal(g)
	if err := os.WriteFile(filepath.Join(dir, GrantFile), raw, 0o400); err != nil {
		t.Fatal(err)
	}
}

func TestGuestRouteIsOffUnlessThisComputerIsGranted(t *testing.T) {
	tok := func() string { return "tok" }
	empty := t.TempDir()
	if rt, _ := GuestRoute(empty, "computer-a", "http://10.200.0.1:8084", tok); rt != nil {
		t.Fatal("no grant turned the route on")
	}
	if rt, _ := GuestRoute("", "computer-a", "http://10.200.0.1:8084", tok); rt != nil {
		t.Fatal("no credential directory turned the route on")
	}

	withheld := t.TempDir()
	writeGrant(t, withheld, Grant{ComputerID: "computer-a", Status: "withheld"})
	if rt, _ := GuestRoute(withheld, "computer-a", "http://10.200.0.1:8084", tok); rt != nil {
		t.Fatal("a withheld grant turned the route on")
	}

	other := t.TempDir()
	writeGrant(t, other, Grant{ComputerID: "computer-b", Status: GrantGranted, Basis: BasisVM})
	if rt, _ := GuestRoute(other, "computer-a", "http://10.200.0.1:8084", tok); rt != nil {
		t.Fatal("another computer's grant turned the route on")
	}

	garbage := t.TempDir()
	if err := os.WriteFile(filepath.Join(garbage, GrantFile), []byte("{"), 0o400); err != nil {
		t.Fatal(err)
	}
	if rt, _ := GuestRoute(garbage, "computer-a", "http://10.200.0.1:8084", tok); rt != nil {
		t.Fatal("a malformed grant turned the route on")
	}

	granted := t.TempDir()
	writeGrant(t, granted, Grant{ComputerID: "computer-a", Status: GrantGranted, Basis: BasisVM})
	rt, status := GuestRoute(granted, "computer-a", "http://10.200.0.1:8084", tok)
	tr, ok := rt.(*Transport)
	if !ok || tr.BaseURL != "http://10.200.0.1:8084" || !strings.Contains(status, "on") {
		t.Fatalf("granted: %T %q", rt, status)
	}

	// Granted with no gateway: on, and refusing (never direct).
	rt, status = GuestRoute(granted, "computer-a", "", tok)
	if rt == nil {
		t.Fatalf("granted without a gateway fell back to direct egress (%q)", status)
	}
	if _, ok := rt.(*Transport); ok {
		t.Fatal("granted without a gateway built a forwarding transport")
	}
	req, _ := http.NewRequest(http.MethodGet, "https://example.com/", nil)
	if _, err := rt.RoundTrip(req); err == nil {
		t.Fatal("refusing route answered")
	}
}
