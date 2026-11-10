package main

import (
	"crypto/ed25519"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/yusefmosiah/go-choir/internal/auth"
	"github.com/yusefmosiah/go-choir/internal/server"
)

func newRateTestHandler(t *testing.T) *auth.Handler {
	t.Helper()
	store, err := auth.OpenStore(filepath.Join(t.TempDir(), "auth.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	cfg := &auth.Config{
		Port:              "0",
		DBPath:            filepath.Join(t.TempDir(), "auth.db"),
		RPID:              "localhost",
		RPOrigins:         []string{"http://localhost:4173"},
		JWTPrivateKeyPath: filepath.Join(t.TempDir(), "jwt-key"),
		AccessTokenTTL:    5 * time.Minute,
		RefreshTokenTTL:   720 * time.Hour,
	}
	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate signer: %v", err)
	}
	wa, err := webauthn.New(&webauthn.Config{RPID: cfg.RPID, RPDisplayName: "go-choir test", RPOrigins: cfg.RPOrigins})
	if err != nil {
		t.Fatalf("create webauthn: %v", err)
	}
	return auth.NewHandler(store, wa, cfg, priv)
}

func serve(s *server.Server, method, path, addr string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	req.RemoteAddr = addr + ":5000"
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	return rec
}

func smallLimits(n int) map[auth.RateClass]int {
	return map[auth.RateClass]int{
		auth.RateClassRead: n, auth.RateClassAct: n, auth.RateClassCeremony: n,
		auth.RateClassSensitive: n, auth.RateClassDeviceToken: n,
	}
}

func TestRateClassesAreIsolated(t *testing.T) {
	const addr = "203.0.113.21"
	s := server.NewServer("auth", "0")
	registerRoutes(s, newRateTestHandler(t), auth.NewRateLimitersWithLimits(smallLimits(3)))

	// Exhausting reads never blocks the login ceremony or session acts.
	for i := 0; i < 3; i++ {
		if rec := serve(s, http.MethodGet, "/auth/session", addr); rec.Code == http.StatusTooManyRequests {
			t.Fatalf("read %d refused", i+1)
		}
	}
	rec := serve(s, http.MethodGet, "/auth/session", addr)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("read 4: got %d, want 429", rec.Code)
	}
	ra, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	if err != nil || ra < 1 || ra > 60 {
		t.Fatalf("Retry-After %q, want 1..60", rec.Header().Get("Retry-After"))
	}
	for _, p := range []string{"/auth/login/begin", "/auth/accounts/switch", "/auth/recovery/request"} {
		if rec := serve(s, http.MethodPost, p, addr); rec.Code == http.StatusTooManyRequests {
			t.Fatalf("%s refused after only reads were exhausted", p)
		}
	}

	// Exhausting ceremonies never blocks reads.
	const addr2 = "203.0.113.22"
	for i := 0; i < 4; i++ {
		serve(s, http.MethodPost, "/auth/login/begin", addr2)
	}
	if rec := serve(s, http.MethodPost, "/auth/login/finish", addr2); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("ceremony not limited: %d", rec.Code)
	}
	for _, p := range []string{"/auth/session", "/auth/accounts"} {
		if rec := serve(s, http.MethodGet, p, addr2); rec.Code == http.StatusTooManyRequests {
			t.Fatalf("%s refused after only ceremonies were exhausted", p)
		}
	}
}

func TestProductionLimitsReadsGenerousCeremonyBounded(t *testing.T) {
	const addr = "203.0.113.23"
	s := server.NewServer("auth", "0")
	registerRoutes(s, newRateTestHandler(t), auth.NewRateLimiters())
	for i := 0; i < auth.AuthReadRateLimit; i++ {
		if rec := serve(s, http.MethodGet, "/auth/session", addr); rec.Code == http.StatusTooManyRequests {
			t.Fatalf("read %d refused", i+1)
		}
	}
	if rec := serve(s, http.MethodGet, "/auth/session", addr); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("read over limit: %d", rec.Code)
	}
	// Several sign-ins' worth of ceremony fits comfortably.
	for i := 0; i < auth.AuthCeremonyRateLimit; i++ {
		if rec := serve(s, http.MethodPost, "/auth/login/begin", addr); rec.Code == http.StatusTooManyRequests {
			t.Fatalf("ceremony %d refused", i+1)
		}
	}
	if rec := serve(s, http.MethodPost, "/auth/login/begin", addr); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("ceremony over limit: %d", rec.Code)
	}
}

func TestRecoveryAndEmailRoutesKeepTheOriginalLimit(t *testing.T) {
	if auth.AuthEndpointRateLimit != 10 || auth.AuthEndpointRateWindow != time.Minute {
		t.Fatalf("sensitive limit changed: %d per %s", auth.AuthEndpointRateLimit, auth.AuthEndpointRateWindow)
	}
	s := server.NewServer("auth", "0")
	registerRoutes(s, newRateTestHandler(t), auth.NewRateLimiters())
	for i, p := range []string{"/auth/recovery/request", "/auth/email/begin"} {
		addr := "203.0.113." + strconv.Itoa(40+i)
		for n := 0; n < 10; n++ {
			if rec := serve(s, http.MethodPost, p, addr); rec.Code == http.StatusTooManyRequests {
				t.Fatalf("%s request %d refused", p, n+1)
			}
		}
		rec := serve(s, http.MethodPost, p, addr)
		if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" {
			t.Fatalf("%s request 11: got %d, Retry-After %q", p, rec.Code, rec.Header().Get("Retry-After"))
		}
	}
}

type recordingRegistrar struct{ patterns []string }

func (r *recordingRegistrar) HandleFunc(p string, _ http.HandlerFunc) {
	r.patterns = append(r.patterns, p)
}

// A new route must be added to authRoutes with a class; registerRoutes is the
// only place that registers, so an unclassified route cannot exist.
func TestEveryAuthRouteHasARateClass(t *testing.T) {
	h := newRateTestHandler(t)
	rec := &recordingRegistrar{}
	registerRoutes(rec, h, auth.NewRateLimiters())

	table := authRoutes(h)
	if len(rec.patterns) != len(table) || len(table) == 0 {
		t.Fatalf("registered %d routes, table has %d", len(rec.patterns), len(table))
	}
	seen := map[string]bool{}
	for _, rt := range table {
		if rt.class.String() == "unset" {
			t.Errorf("route %q has no rate class", rt.pattern)
		}
		if seen[rt.pattern] {
			t.Errorf("route %q listed twice", rt.pattern)
		}
		seen[rt.pattern] = true
	}
	for _, p := range rec.patterns {
		if !seen[p] {
			t.Errorf("registered route %q is not in the table", p)
		}
	}
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(src), ".HandleFunc("); n != 1 {
		t.Errorf("main.go registers routes with HandleFunc %d times; add routes to authRoutes with a class", n)
	}
}

func TestUnclassifiedRoutePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected a panic for an unset class")
		}
	}()
	auth.NewRateLimiters().Wrap(auth.RateClassUnset, func(http.ResponseWriter, *http.Request) {})
}
