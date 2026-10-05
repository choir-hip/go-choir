package main

import (
	"context"
	"github.com/yusefmosiah/go-choir/internal/keyescrow"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/server"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
	"log"
	"strconv"
	"time"
)

func main() {
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
	// Periodic platform-artifacts reachability GC (dry-run unless explicitly
	// activated). The on-demand endpoint /internal/platform/artifact-gc admits
	// the same sweep on demand for the deployed-proof report.
	gcGrace, _ := time.ParseDuration(cfg.ArtifactGCGrace)
	gcInterval, _ := time.ParseDuration(cfg.ArtifactGCInterval)
	gcMax, _ := strconv.Atoi(cfg.ArtifactGCMaxDeletes)
	platform.NewGCRunner(svc, platform.ArtifactGCConfig{
		Mode:       cfg.ArtifactGCMode,
		Grace:      gcGrace,
		MaxDeletes: gcMax,
	}, gcInterval).Start(context.Background())
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

	// Object graph API: allows sourcecycled and VMs to project and query
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
	platform.RegisterObjectGraphRoutes(s, ogHandler)

	s.Start()
}
