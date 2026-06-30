package main

import (
	"crypto/ed25519"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/yusefmosiah/go-choir/internal/auth"
	"github.com/yusefmosiah/go-choir/internal/server"
)

func TestRegisterRoutesAppliesGlobalAuthRateLimit(t *testing.T) {
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
		CookieSecure:      false,
	}

	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate signer: %v", err)
	}
	wa, err := webauthn.New(&webauthn.Config{
		RPID:          cfg.RPID,
		RPDisplayName: "go-choir test",
		RPOrigins:     cfg.RPOrigins,
	})
	if err != nil {
		t.Fatalf("create webauthn: %v", err)
	}

	handler := auth.NewHandler(store, wa, cfg, priv)
	s := server.NewServer("auth", "0")
	registerRoutes(s, handler, auth.NewIPRateLimiter(2, time.Minute))

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/auth/session", nil)
		req.RemoteAddr = "203.0.113.10:5000"
		rec := httptest.NewRecorder()
		s.ServeHTTP(rec, req)
		if rec.Code == http.StatusTooManyRequests {
			t.Fatalf("request %d should not be rate limited", i+1)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/auth/session", nil)
	req.RemoteAddr = "203.0.113.10:5000"
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
}
