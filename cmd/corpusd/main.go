package main

import (
	"context"
	"github.com/yusefmosiah/go-choir/internal/frontimportance"
	"github.com/yusefmosiah/go-choir/internal/keyescrow"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/server"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
	"log"
	"os"
	"strconv"
	"time"
)

func main() {
	// `corpusd front-importance` is the host timer's scoring pass
	// (internal/frontimportance); it never starts the platform service.
	if len(os.Args) > 1 && os.Args[1] == "front-importance" {
		os.Exit(frontimportance.Main(os.Args[2:]))
	}
	cfg, err := platform.LoadConfig()
	if err != nil {
		log.Fatalf("corpusd config: %v", err)
	}
	if err := cfg.EnsureDirs(); err != nil {
		log.Fatalf("corpusd dirs: %v", err)
	}

	var store *platform.Store
	var storeErr error
	for attempt := 1; attempt <= 20; attempt++ {
		store, storeErr = platform.OpenStore(cfg.DoltDSN, cfg.CorpusDoltDSN)
		if storeErr == nil {
			break
		}
		log.Printf("corpusd store: attempt %d/20: %v", attempt, storeErr)
		time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
	}
	if storeErr != nil {
		log.Fatalf("corpusd store: %v", storeErr)
	}
	defer func() {
		_ = store.Close()
	}()

	svc := platform.NewService(store, cfg.ArtifactsRoot, cfg.SigningKeyPath)
	// Published versions' check overlay reads quote checks and support
	// verdicts from Store B, never from a paper computer (Cut 2 2b).
	svc.SetCheckOverlayProvider(platform.NewStoreCheckOverlay(svc))
	// The Autopaper front lists only these owners' auto stories when
	// CORPUSD_AUTOPAPER_OWNERS is set; every owner's when unset.
	svc.SetAutopaperOwners(platform.ParseAutopaperOwners(cfg.AutopaperOwnersRaw))
	// Front rows carry importance scores when the host scorer's file is
	// present; absent or unreadable, the front list is unchanged.
	svc.SetFrontImportancePath(cfg.FrontImportancePath)
	// Periodic platform-artifacts reachability GC (dry-run unless explicitly
	// activated). The on-demand endpoint /internal/platform/artifact-gc runs
	// the same sweep, clamped to this policy.
	gcGrace, _ := time.ParseDuration(cfg.ArtifactGCGrace)
	gcInterval, _ := time.ParseDuration(cfg.ArtifactGCInterval)
	gcMax, _ := strconv.Atoi(cfg.ArtifactGCMaxDeletes)
	gcMaxBytes, _ := strconv.ParseInt(cfg.ArtifactGCMaxBytes, 10, 64)
	gcPolicy := platform.ArtifactGCConfig{
		Mode:             cfg.ArtifactGCMode,
		Grace:            gcGrace,
		MaxDeletes:       gcMax,
		MaxBytes:         gcMaxBytes,
		ActiveNamespaces: platform.ParseArtifactGCNamespaces(cfg.ArtifactGCNamespaces),
	}
	svc.SetArtifactGCPolicy(gcPolicy)
	platform.NewGCRunner(svc, gcPolicy, gcInterval).Start(context.Background())
	handler := platform.NewHandler(svc)
	if cfg.VmctlURL != "" {
		if err := handler.ConfigureGuestBinding(vmctl.NewClient(cfg.VmctlURL)); err != nil {
			log.Fatalf("corpusd guest binding: %v", err)
		}
	}
	eventCAS, eventArtifacts, eventAuth, err := svc.ComputerEventRuntime()
	if err != nil {
		log.Fatalf("corpusd computer event runtime: %v", err)
	}
	if err := handler.ConfigureComputerEvents(eventCAS, eventArtifacts, eventAuth); err != nil {
		log.Fatalf("corpusd computer event routes: %v", err)
	}
	modeCAS, err := svc.SelfDevelopmentModeRuntime()
	if err != nil {
		log.Fatalf("corpusd self-development mode runtime: %v", err)
	}
	if err := handler.ConfigureSelfDevelopmentModes(modeCAS); err != nil {
		log.Fatalf("corpusd self-development mode routes: %v", err)
	}
	escrowPrivateKey, err := keyescrow.LoadOrGeneratePrivateKey(cfg.KeyEscrowKeyPath)
	if err != nil {
		log.Fatalf("corpusd key escrow key: %v", err)
	}
	if err := handler.ConfigureKeyEscrow(escrowPrivateKey, cfg.KeyEscrowOperatorsRaw); err != nil {
		log.Fatalf("corpusd key escrow routes: %v", err)
	}
	s := server.NewServer("corpusd", cfg.Port)
	platform.RegisterRoutes(s, handler)

	// Object graph API: allows VMs to project and query
	// object graph data stored in the platform Dolt SQL server (corpusd).
	ogStore := platform.NewObjectGraphStore(store)
	ogService := objectgraph.NewService(objectgraph.Config{
		Durable: ogStore,
	})
	ogHandler := platform.NewObjectGraphHandler(ogService, ogStore)
	if cfg.VmctlURL != "" {
		if err := ogHandler.ConfigureGuestBinding(vmctl.NewClient(cfg.VmctlURL)); err != nil {
			log.Fatalf("corpusd object graph guest binding: %v", err)
		}
	}
	ogHandler.ConfigureCaptureBodies(platform.CaptureBodiesRoot(cfg.ArtifactsRoot))
	platform.RegisterObjectGraphRoutes(s, ogHandler)

	s.Start()
}
