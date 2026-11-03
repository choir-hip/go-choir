package main

import (
	"context"
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

	// Email sign-in: with the switch off (AUTH_EMAIL_SIGN_IN), delete the
	// attempts and email sessions a previous run left; a sweeper expires rows.
	if err := handler.ApplyEmailSignInSwitch(); err != nil {
		log.Fatalf("auth email sign-in switch: %v", err)
	}
	handler.StartEmailSweeper(context.Background(), 0)
	handler.StartNumberReconciler(context.Background(), 0)
	handler.StartDeviceSweeper(context.Background(), 0)

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
	s.HandleFunc("/auth/desktop/exchange-redirect", limit(handler.HandleDesktopExchangeRedirect))
	s.HandleFunc("/auth/desktop/redeem", limit(handler.HandleDesktopRedeem))
	s.HandleFunc("GET /api/auth/root/status", limit(handler.HandleRootStatus))

	// M1: API key management (headless auth).
	s.HandleFunc("POST /auth/api-keys", limit(handler.HandleCreateAPIKey))
	s.HandleFunc("GET /auth/api-keys", limit(handler.HandleListAPIKeys))
	s.HandleFunc("DELETE /auth/api-keys/{id}", limit(handler.HandleRevokeAPIKey))

	// choir login: device authorization (docs/design/cli-agent-onboarding-2026-11-03.md).
	// Token polls get their own limiter: the global one would refuse a
	// 5-second poll; slow_down paces each request.
	tokenLimit := auth.NewIPRateLimiter(auth.DeviceTokenRateLimit, auth.AuthEndpointRateWindow).Wrap
	s.HandleFunc("POST /auth/device/begin", limit(handler.HandleDeviceBegin))
	s.HandleFunc("GET /auth/device/lookup", limit(handler.HandleDeviceLookup))
	s.HandleFunc("POST /auth/device/approve", limit(handler.HandleDeviceApprove))
	s.HandleFunc("POST /auth/device/token", tokenLimit(handler.HandleDeviceToken))

	// M7: Account recovery, multi-device passkey management, session management.
	s.HandleFunc("POST /auth/recovery/request", limit(handler.HandleRecoveryRequest))
	s.HandleFunc("POST /auth/recovery/verify", limit(handler.HandleRecoveryVerify))
	s.HandleFunc("GET /auth/credentials", limit(handler.HandleListCredentials))
	s.HandleFunc("POST /auth/credentials/rename", limit(handler.HandleRenameCredential))
	s.HandleFunc("DELETE /auth/credentials/{id}", limit(handler.HandleDeleteCredential))
	s.HandleFunc("GET /auth/sessions", limit(handler.HandleListSessions))
	s.HandleFunc("DELETE /auth/sessions/{id}", limit(handler.HandleRevokeSession))

	// Email sign-in and address verification (switch auth.email_sign_in).
	s.HandleFunc("POST /auth/email/begin", limit(handler.HandleEmailBegin))
	s.HandleFunc("POST /auth/email/verify", limit(handler.HandleEmailVerify))
	s.HandleFunc("POST /auth/email/verify-address/begin", limit(handler.HandleVerifyAddressBegin))
	s.HandleFunc("POST /auth/email/verify-address/finish", limit(handler.HandleVerifyAddressFinish))
}
