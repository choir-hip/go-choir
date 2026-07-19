package main

import (
	"log"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/server"
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
		store, storeErr = platform.OpenStore(cfg.DoltDSN)
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
	handler := platform.NewHandler(svc)
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
	s := server.NewServer("corpusd", cfg.Port)
	platform.RegisterRoutes(s, handler)

	// Object graph API: allows sourcecycled and VMs to project and query
	// object graph data stored in the platform Dolt SQL server (corpusd).
	ogStore := platform.NewObjectGraphStore(store)
	ogService := objectgraph.NewService(objectgraph.Config{
		Durable: ogStore,
	})
	ogHandler := platform.NewObjectGraphHandler(ogService, ogStore)
	platform.RegisterObjectGraphRoutes(s, ogHandler)

	s.Start()
}
