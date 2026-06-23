package main

import (
	"log"

	"github.com/yusefmosiah/go-choir/internal/maild"
	"github.com/yusefmosiah/go-choir/internal/server"
)

func main() {
	cfg, err := maild.LoadConfig()
	if err != nil {
		log.Fatalf("maild config: %v", err)
	}
	if err := cfg.EnsureDirs(); err != nil {
		log.Fatalf("maild dirs: %v", err)
	}

	store, err := maild.OpenStore(cfg.DBPath, cfg.StorageRoot)
	if err != nil {
		log.Fatalf("maild store: %v", err)
	}
	defer func() {
		_ = store.Close()
	}()

	if err := store.EnsureSchema(cfg); err != nil {
		log.Fatalf("maild schema: %v", err)
	}

	handler := maild.NewHandler(cfg, store)
	s := server.NewServer("maild", cfg.Port)
	maild.RegisterRoutes(s, handler)
	s.Start()
}
