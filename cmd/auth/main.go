package main

import (
	"log"
	"net/http"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/yusefmosiah/go-choir/internal/auth"
	"github.com/yusefmosiah/go-choir/internal/server"
)

type authRouteRegistrar interface {
	HandleFunc(pattern string, handler http.HandlerFunc)
}

func main() {
	cfg, err := auth.LoadConfig()
	if err != nil {
		log.Fatalf("auth config: %v", err)
	}

	if err := cfg.EnsureDirs(); err != nil {
		log.Fatalf("auth dirs: %v", err)
	}

	store, err := auth.OpenStore(cfg.DBPath)
	if err != nil {
		log.Fatalf("auth store: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Create the WebAuthn Relying Party instance bound to the configured RP ID.
	wa, err := webauthn.New(&webauthn.Config{
		RPID:          cfg.RPID,
		RPDisplayName: "go-choir",
		RPOrigins:     cfg.RPOrigins,
	})
	if err != nil {
		log.Fatalf("auth webauthn: %v", err)
	}

	// Load the Ed25519 private key for JWT signing.
	signer, err := auth.LoadPrivateKey(cfg.JWTPrivateKeyPath)
	if err != nil {
		log.Fatalf("auth signing key: %v", err)
	}

	// Create the auth handler with store, WebAuthn, config, and signer.
	handler := auth.NewHandler(store, wa, cfg, signer)
	rateLimiter := auth.NewIPRateLimiter(auth.AuthEndpointRateLimit, auth.AuthEndpointRateWindow)

	s := server.NewServer("auth", cfg.Port)
	registerRoutes(s, handler, rateLimiter)

	s.Start()
}

func registerRoutes(s authRouteRegistrar, handler *auth.Handler, rateLimiter *auth.IPRateLimiter) {
	limit := rateLimiter.Wrap
	// Register /auth/* routes.
	s.HandleFunc("/auth/register/begin", limit(handler.HandleRegisterBegin))
	s.HandleFunc("/auth/register/finish", limit(handler.HandleRegisterFinish))
	s.HandleFunc("/auth/login/begin", limit(handler.HandleLoginBegin))
	s.HandleFunc("/auth/login/finish", limit(handler.HandleLoginFinish))
	s.HandleFunc("/auth/session", limit(handler.HandleSession))
	s.HandleFunc("/auth/logout", limit(handler.HandleLogout))
	s.HandleFunc("/auth/desktop/exchange", limit(handler.HandleDesktopExchange))
	s.HandleFunc("/auth/desktop/exchange-redirect", limit(handler.HandleDesktopExchangeRedirect))
	s.HandleFunc("/auth/desktop/redeem", limit(handler.HandleDesktopRedeem))

	// M1: API key management (headless auth).
	s.HandleFunc("POST /auth/api-keys", limit(handler.HandleCreateAPIKey))
	s.HandleFunc("GET /auth/api-keys", limit(handler.HandleListAPIKeys))
	s.HandleFunc("DELETE /auth/api-keys/{id}", limit(handler.HandleRevokeAPIKey))

	// M7: Account recovery, multi-device passkey management, session management.
	s.HandleFunc("POST /auth/recovery/request", limit(handler.HandleRecoveryRequest))
	s.HandleFunc("POST /auth/recovery/verify", limit(handler.HandleRecoveryVerify))
	s.HandleFunc("GET /auth/credentials", limit(handler.HandleListCredentials))
	s.HandleFunc("POST /auth/credentials/rename", limit(handler.HandleRenameCredential))
	s.HandleFunc("DELETE /auth/credentials/{id}", limit(handler.HandleDeleteCredential))
	s.HandleFunc("GET /auth/sessions", limit(handler.HandleListSessions))
	s.HandleFunc("DELETE /auth/sessions/{id}", limit(handler.HandleRevokeSession))
}
