package main

import (
	"log"
	"time"

	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/server"
)

func main() {
	cfg, err := platform.LoadConfig()
	if err != nil {
		log.Fatalf("platformd config: %v", err)
	}
	if err := cfg.EnsureDirs(); err != nil {
		log.Fatalf("platformd dirs: %v", err)
	}

	var store *platform.Store
	var storeErr error
	for attempt := 1; attempt <= 20; attempt++ {
		store, storeErr = platform.OpenStore(cfg.DoltDSN)
		if storeErr == nil {
			break
		}
		log.Printf("platformd store: attempt %d/20: %v", attempt, storeErr)
		time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
	}
	if storeErr != nil {
		log.Fatalf("platformd store: %v", storeErr)
	}
	defer func() {
		_ = store.Close()
	}()

	svc := platform.NewService(store, cfg.ArtifactsRoot, cfg.SigningKeyPath)
	handler := platform.NewHandler(svc)
	s := server.NewServer("platformd", cfg.Port)
	platform.RegisterRoutes(s, handler)
	s.Start()
}
