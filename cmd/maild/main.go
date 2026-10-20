package main

import (
	"context"
	"log"
	"time"

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

	// A restart never resumes a digest run: a run still "started" when the
	// process died is closed as interrupted, and nothing is sent at boot
	// (CLAUDE.md "Restarts End Work"). The next tick is new work.
	if n, err := store.CloseOpenDigestRuns(context.Background(), time.Now()); err != nil {
		log.Printf("maild: close open digest runs: %v", err)
	} else if n > 0 {
		log.Printf("maild: closed %d digest run(s) interrupted by a restart", n)
	}

	log.Printf("maild: %s", cfg.SendGateStatus())
	if cfg.DigestOff {
		log.Printf("maild: autopaper digest is switched off (MAILD_DIGEST=off)")
	}
	handler := maild.NewHandler(cfg, store)
	s := server.NewServer("maild", cfg.Port)
	maild.RegisterRoutes(s, handler)
	s.Start()
}
