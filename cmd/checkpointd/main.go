package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/yusefmosiah/go-choir/internal/keyescrow"
	"github.com/yusefmosiah/go-choir/internal/platform"
)

// storeOpenTimeout bounds connecting to the platform PostgreSQL databases. Observed
// maintenance runs take under one second, so a minute is two orders of
// magnitude of slack and still short of any unit timeout.
const storeOpenTimeout = 60 * time.Second

func main() {
	// The operator capture job (capture.go) holds no escrow key and runs
	// outside the timer's unit.
	if len(os.Args) > 1 && os.Args[1] == "capture-base" {
		os.Exit(captureBaseMain(os.Args[2:]))
	}
	// This is intentionally NOT a corpusd environment flag: the plaintext key
	// privilege belongs only to this isolated maintenance process.
	if os.Getenv("CHOIR_CHECKPOINT_MAINTENANCE_AUTHORIZED") != "true" {
		log.Fatal("checkpoint maintenance key authority not enabled")
	}
	cfg, err := platform.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}
	if _, err := os.Stat(cfg.KeyEscrowKeyPath); err != nil {
		log.Fatal("existing checkpoint escrow key unavailable")
	}
	if _, err := os.Stat(cfg.SigningKeyPath); err != nil {
		log.Fatal("existing checkpoint signing trust root unavailable")
	}
	if err := ensureDoltHome(os.Getenv); err != nil {
		log.Fatal(err)
	}
	escrow, escrowKeyState, escrowSealMode, err := keyescrow.LoadConfiguredPrivateKey(cfg.KeyEscrowKeyPath, os.Getenv)
	if err != nil {
		log.Fatal(err)
	}
	if escrowSealMode != keyescrow.SealOff {
		log.Printf("checkpoint escrow key: %s (seal mode %s)", escrowKeyState, escrowSealMode)
	}
	defer clear(escrow[:])
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
	svc := platform.NewService(store, cfg.ArtifactsRoot, cfg.SigningKeyPath)
	job, err := runCheckpointWorker(ctx, svc, store, escrow, filepath.Join(cfg.ArtifactsRoot, "checkpoint-scratch"), 15<<30)
	if job != nil {
		if encErr := json.NewEncoder(os.Stdout).Encode(job); encErr != nil {
			log.Printf("checkpoint receipt: %v", encErr)
		}
	}
	if err != nil {
		log.Printf("checkpoint worker: %v", err)
		os.Exit(1)
	}
}
