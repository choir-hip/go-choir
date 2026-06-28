package main

import (
	"log"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/yusefmosiah/go-choir/internal/auth"
	"github.com/yusefmosiah/go-choir/internal/server"
)

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

	s := server.NewServer("auth", cfg.Port)

	// Register /auth/* routes.
	s.HandleFunc("/auth/register/begin", handler.HandleRegisterBegin)
	s.HandleFunc("/auth/register/finish", handler.HandleRegisterFinish)
	s.HandleFunc("/auth/login/begin", handler.HandleLoginBegin)
	s.HandleFunc("/auth/login/finish", handler.HandleLoginFinish)
	s.HandleFunc("/auth/session", handler.HandleSession)
	s.HandleFunc("/auth/logout", handler.HandleLogout)
	s.HandleFunc("/auth/desktop/exchange", handler.HandleDesktopExchange)
	s.HandleFunc("/auth/desktop/exchange-redirect", handler.HandleDesktopExchangeRedirect)
	s.HandleFunc("/auth/desktop/redeem", handler.HandleDesktopRedeem)

	// M1: API key management (headless auth).
	s.HandleFunc("POST /auth/api-keys", handler.HandleCreateAPIKey)
	s.HandleFunc("GET /auth/api-keys", handler.HandleListAPIKeys)
	s.HandleFunc("DELETE /auth/api-keys/{id}", handler.HandleRevokeAPIKey)

	// M7: Account recovery, multi-device passkey management, session management.
	s.HandleFunc("POST /auth/recovery/request", handler.HandleRecoveryRequest)
	s.HandleFunc("POST /auth/recovery/verify", handler.HandleRecoveryVerify)
	s.HandleFunc("GET /auth/credentials", handler.HandleListCredentials)
	s.HandleFunc("POST /auth/credentials/rename", handler.HandleRenameCredential)
	s.HandleFunc("DELETE /auth/credentials/{id}", handler.HandleDeleteCredential)
	s.HandleFunc("GET /auth/sessions", handler.HandleListSessions)
	s.HandleFunc("DELETE /auth/sessions/{id}", handler.HandleRevokeSession)

	s.Start()
}
