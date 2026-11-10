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
	rateLimiters := auth.NewRateLimiters()

	// Email sign-in: with the switch off (AUTH_EMAIL_SIGN_IN), delete the
	// attempts and email sessions a previous run left; a sweeper expires rows.
	if err := handler.ApplyEmailSignInSwitch(); err != nil {
		log.Fatalf("auth email sign-in switch: %v", err)
	}
	handler.StartEmailSweeper(context.Background(), 0)
	handler.StartNumberReconciler(context.Background(), 0)
	handler.StartDeviceSweeper(context.Background(), 0)

	s := server.NewServer("auth", cfg.Port)
	registerRoutes(s, handler, rateLimiters)

	s.Start()
}

// authRoute is one /auth route and its rate cost class. A route with no
// class panics at registration and fails TestEveryAuthRouteHasARateClass.
type authRoute struct {
	pattern string
	class   auth.RateClass
	handler http.HandlerFunc
}

// authRoutes is the route table. Classes (internal/auth/rate_limit.go):
// read = own-session reads, act = authenticated session acts, ceremony =
// unauthenticated begin/finish, sensitive = recovery and email routes (the
// original 10 a minute), device_token = token polls.
func authRoutes(handler *auth.Handler) []authRoute {
	const (
		read      = auth.RateClassRead
		act       = auth.RateClassAct
		ceremony  = auth.RateClassCeremony
		sensitive = auth.RateClassSensitive
	)
	return []authRoute{
		{"/auth/register/begin", ceremony, handler.HandleRegisterBegin},
		{"/auth/register/finish", ceremony, handler.HandleRegisterFinish},
		{"/auth/login/begin", ceremony, handler.HandleLoginBegin},
		{"/auth/login/finish", ceremony, handler.HandleLoginFinish},
		{"/auth/session", read, handler.HandleSession},
		{"/auth/logout", act, handler.HandleLogout},
		// Account switcher (docs/design/account-switcher-2026-11-09.md).
		{"/auth/accounts", read, handler.HandleAccountsList},
		{"/auth/accounts/switch", act, handler.HandleAccountSwitch},
		{"/auth/accounts/signout", act, handler.HandleAccountSignout},
		{"/auth/accounts/logout-all", act, handler.HandleAccountsLogoutAll},
		{"/auth/desktop/exchange-redirect", ceremony, handler.HandleDesktopExchangeRedirect},
		{"/auth/desktop/redeem", ceremony, handler.HandleDesktopRedeem},
		{"GET /api/auth/root/status", read, handler.HandleRootStatus},

		// M1: API key management (headless auth).
		{"POST /auth/api-keys", act, handler.HandleCreateAPIKey},
		{"GET /auth/api-keys", act, handler.HandleListAPIKeys},
		{"DELETE /auth/api-keys/{id}", act, handler.HandleRevokeAPIKey},

		// choir login: device authorization (docs/design/cli-agent-onboarding-2026-11-03.md).
		// Token polls get their own class: slow_down paces each request.
		{"POST /auth/device/begin", ceremony, handler.HandleDeviceBegin},
		{"GET /auth/device/lookup", ceremony, handler.HandleDeviceLookup},
		{"POST /auth/device/approve", act, handler.HandleDeviceApprove},
		{"POST /auth/device/token", auth.RateClassDeviceToken, handler.HandleDeviceToken},

		// M7: Account recovery, multi-device passkey management, session management.
		{"POST /auth/recovery/request", sensitive, handler.HandleRecoveryRequest},
		{"POST /auth/recovery/verify", sensitive, handler.HandleRecoveryVerify},
		{"GET /auth/credentials", read, handler.HandleListCredentials},
		{"POST /auth/credentials/rename", act, handler.HandleRenameCredential},
		{"DELETE /auth/credentials/{id}", act, handler.HandleDeleteCredential},
		{"GET /auth/sessions", read, handler.HandleListSessions},
		{"DELETE /auth/sessions/{id}", act, handler.HandleRevokeSession},

		// Email sign-in and address verification (switch auth.email_sign_in).
		{"POST /auth/email/begin", sensitive, handler.HandleEmailBegin},
		{"POST /auth/email/verify", sensitive, handler.HandleEmailVerify},
		{"POST /auth/email/verify-address/begin", sensitive, handler.HandleVerifyAddressBegin},
		{"POST /auth/email/verify-address/finish", sensitive, handler.HandleVerifyAddressFinish},
	}
}

func registerRoutes(s authRouteRegistrar, handler *auth.Handler, limiters *auth.RateLimiters) {
	for _, rt := range authRoutes(handler) {
		s.HandleFunc(rt.pattern, handler.Instrument(limiters, rt.pattern, rt.class, rt.handler))
	}
}
