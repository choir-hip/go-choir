package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/yusefmosiah/go-choir/internal/accountpurge"
	"github.com/yusefmosiah/go-choir/internal/auth"
	"github.com/yusefmosiah/go-choir/internal/server"
)

type authRouteRegistrar interface {
	HandleFunc(pattern string, handler http.HandlerFunc)
}

func main() {
	// `auth deletion-restore-check`: after a restore from backup, find the
	// accounts deleted since (restore_check.go). It does not start the service.
	if len(os.Args) > 1 && os.Args[1] == "deletion-restore-check" {
		os.Exit(runDeletionRestoreCheck(os.Args[2:], os.Stdout))
	}
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

	// An account whose address was never proved is inert (it signs in to
	// nothing and mints nothing); what it still holds from before that rule is
	// revoked now, once per start, and the mail recovery that proves the
	// address replaces its passkeys (docs/problems/passkey-claims-an-unproved-address-2026-11-24.md).
	if n, err := store.RevokeUnprovedAccess(auth.BootstrapAdminAPIKeyUserID); err != nil {
		log.Fatalf("auth revoke unproved access: %v", err)
	} else if n > 0 {
		log.Printf("auth: revoked %d key, session or grant rows of accounts with an unproved address", n)
	}

	// Email sign-in: with the switch off (AUTH_EMAIL_SIGN_IN), delete the
	// attempts and email sessions a previous run left; a sweeper expires rows.
	if err := handler.ApplyEmailSignInSwitch(); err != nil {
		log.Fatalf("auth email sign-in switch: %v", err)
	}
	handler.StartEmailSweeper(context.Background(), 0)
	handler.StartNumberReconciler(context.Background(), 0)

	// Account deletion (docs/design/account-create-delete-2026-11-23.md):
	// the loop that holds a pending account's computer stopped and takes
	// suspended, and purges after 72 hours. It re-reads the deletion table
	// every minute; a boot only closes an attempt a crash left open.
	purge := accountpurge.New(store, accountpurge.NewHost(cfg.VMCTLURL, cfg.CorpusdURL, cfg.MaildURL, cfg.GatewayURL))
	// The ledger of finished deletions sits beside auth.db, not in it and not in
	// what the backup copies: after a restore, cmd/deletion-restore-check finds
	// the accounts that must be deleted again.
	purge.LedgerPath = filepath.Join(filepath.Dir(cfg.DBPath), "account-deletion-ledger.jsonl")
	handler.SetDeletionHost(purge)
	// The reviewer account (lane demo-account) may not be deleted by anyone:
	// when that lane lands, its Handler.IsDemoAccount goes here, as
	//
	//	handler.SetDeletionRefuser(handler.IsDemoAccount)
	//
	// (TestTheReviewerAccountCannotBeDeleted proves the hook; nothing refuses
	// until it is set.)
	purge.Start(context.Background())
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
// begin = login/register begin (account-existence oracles, 10 a minute),
// read = own-session reads, act = authenticated session acts, ceremony =
// unauthenticated finish and device routes, sensitive = recovery and email routes (the
// original 10 a minute), device_token = token polls.
func authRoutes(handler *auth.Handler) []authRoute {
	const (
		read      = auth.RateClassRead
		act       = auth.RateClassAct
		begin     = auth.RateClassBegin
		ceremony  = auth.RateClassCeremony
		sensitive = auth.RateClassSensitive
	)
	return []authRoute{
		{"POST /auth/register/mail", begin, handler.HandleRegisterMail},
		{"POST /auth/register/begin", begin, handler.HandleRegisterBegin},
		{"POST /auth/register/finish", ceremony, handler.HandleRegisterFinish},
		{"POST /auth/login/begin", begin, handler.HandleLoginBegin},
		{"POST /auth/login/finish", ceremony, handler.HandleLoginFinish},
		{"GET /auth/session", read, handler.HandleSession},
		{"POST /auth/logout", act, handler.HandleLogout},
		// Account switcher (docs/design/account-switcher-2026-11-09.md).
		{"GET /auth/accounts", read, handler.HandleAccountsList},
		{"POST /auth/accounts/switch", act, handler.HandleAccountSwitch},
		{"POST /auth/accounts/signout", act, handler.HandleAccountSignout},
		{"POST /auth/accounts/logout-all", act, handler.HandleAccountsLogoutAll},
		{"GET /auth/desktop/exchange-redirect", ceremony, handler.HandleDesktopExchangeRedirect},
		{"POST /auth/desktop/redeem", ceremony, handler.HandleDesktopRedeem},
		{"GET /api/auth/root/status", read, handler.HandleRootStatus},

		// M1: API key management (headless auth).
		{"POST /auth/api-keys", act, handler.HandleCreateAPIKey},
		{"GET /auth/api-keys", act, handler.HandleListAPIKeys},
		{"DELETE /auth/api-keys/{id}", act, handler.HandleRevokeAPIKey},
		{"GET /auth/whoami", read, handler.HandleWhoami},

		// The phone app's passkey ceremony, ending in a key (internal/auth/app_ceremony.go).
		{"POST /auth/app/login/begin", begin, handler.HandleAppLoginBegin},
		{"POST /auth/app/register/finish", ceremony, handler.HandleAppRegisterFinish},
		{"POST /auth/app/login/finish", ceremony, handler.HandleAppLoginFinish},

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

		// Account deletion (docs/design/account-create-delete-2026-11-23.md):
		// a fresh passkey assertion starts it; a passkey sign-in during the
		// 72 hours offers the cancel.
		{"POST /auth/account/delete/begin", sensitive, handler.HandleDeleteBegin},
		{"POST /auth/account/delete/finish", sensitive, handler.HandleDeleteFinish},
		{"POST /auth/account/delete/cancel", ceremony, handler.HandleDeleteCancel},

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
