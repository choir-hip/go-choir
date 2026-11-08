package main

import (
	"log"

	"github.com/yusefmosiah/go-choir/internal/apppackage"
	"github.com/yusefmosiah/go-choir/internal/proxy"
	"github.com/yusefmosiah/go-choir/internal/server"
)

func main() {
	// Host side: a release is admitted by the shipped isolation; the installing
	// guest checks its own (apppackage.CurrentPlatform).
	apppackage.SetPlatformSource(apppackage.ShippedPlatform)
	cfg, err := proxy.LoadConfig()
	if err != nil {
		log.Fatalf("proxy config: %v", err)
	}

	if err := cfg.EnsureDirs(); err != nil {
		log.Fatalf("proxy dirs: %v", err)
	}

	// Load the auth public key for JWT verification.
	pubKey, err := cfg.LoadAuthPublicKey()
	if err != nil {
		log.Fatalf("proxy auth public key: %v", err)
	}

	// Create the proxy handler with auth gating and reverse proxy.
	handler, err := proxy.NewHandler(cfg, pubKey)
	if err != nil {
		log.Fatalf("proxy handler: %v", err)
	}

	s := server.NewServer("proxy", cfg.Port)

	// Register proxy routes.
	proxy.RegisterRoutes(s, handler)

	s.Start()
}
