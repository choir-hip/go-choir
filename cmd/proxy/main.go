package main

import (
	"log"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/proxy"
	"github.com/yusefmosiah/go-choir/internal/server"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

func main() {
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

	// Conditionally wire the route-over-ComputerVersion resolver.
	// When RuntimeDBPath is set, the proxy opens a read-only lineage
	// reader and resolves the platform route through
	// ComputerSourceLineage instead of hard-coded VM identity constants.
	//
	// When RuntimeDBPath is empty (the default in embedded mode), the
	// proxy uses the hard-coded platform constants (H031 fallback).
	// This is the safety net: existing deployments continue to work
	// unchanged.
	//
	// In embedded mode, two processes cannot share the same Dolt
	// workspace. The runtime process owns the workspace, so the proxy
	// cannot open it. When the platform Dolt moves to sql-server mode,
	// the proxy can open a client connection and this wiring will be
	// enabled in production.
	wireRouteResolver(handler, cfg)

	s := server.NewServer("proxy", cfg.Port)

	// Register proxy routes.
	proxy.RegisterRoutes(s, handler)

	s.Start()
}

// wireRouteResolver conditionally opens a store and wires a
// LineageBasedRouteResolver to the handler. If RuntimeDBPath is empty
// or the store cannot be opened, the handler keeps the default
// hard-coded platform constants.
func wireRouteResolver(handler *proxy.Handler, cfg *proxy.Config) {
	runtimeDBPath := strings.TrimSpace(cfg.RuntimeDBPath)
	if runtimeDBPath == "" {
		return
	}

	st, err := store.Open(runtimeDBPath)
	if err != nil {
		log.Printf("proxy: route resolver: open runtime store %q: %v (falling back to hard-coded platform constants)", runtimeDBPath, err)
		return
	}

	reader := &proxy.StoreLineageReader{Store: st}
	resolver := &proxy.LineageBasedRouteResolver{
		Reader:     reader,
		OwnerID:    vmctl.UniversalWirePlatformOwnerID,
		ComputerID: vmctl.UniversalWirePlatformComputerID,
	}
	handler.SetRouteResolver(resolver)
	log.Printf("proxy: route resolver: wired lineage-based resolver (owner=%s computer=%s)", vmctl.UniversalWirePlatformOwnerID, vmctl.UniversalWirePlatformComputerID)
}
