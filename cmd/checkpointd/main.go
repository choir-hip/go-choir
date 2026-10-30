package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yusefmosiah/go-choir/internal/platform"
)

// storeOpenTimeout bounds connecting to the platform PostgreSQL databases. Observed
// maintenance runs take under one second, so a minute is two orders of
// magnitude of slack and still short of any unit timeout.
const storeOpenTimeout = 60 * time.Second

func main() {
	// checkpointd holds no escrow key and no signing key: the host no longer
	// rebuilds checkpoints, so it never opens a computer's tape payloads.
	cfg, err := platform.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	// The open is bounded; the worker keeps the signal context. checkpointd
	// does not own the schema (corpusd bootstraps), so it applies no DDL.
	openCtx, cancelOpen := context.WithTimeout(ctx, storeOpenTimeout)
	store, err := platform.OpenStoreWithoutBootstrap(openCtx, cfg.ControlDSN, cfg.GraphDSN)
	cancelOpen()
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()
	if err := runCheckpointWorker(ctx, store); err != nil {
		log.Printf("checkpoint worker: %v", err)
		os.Exit(1)
	}
}
