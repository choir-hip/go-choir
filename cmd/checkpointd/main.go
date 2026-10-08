package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/yusefmosiah/go-choir/internal/keyescrow"
	"github.com/yusefmosiah/go-choir/internal/platform"
)

func main() {
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
	escrow, err := keyescrow.LoadOrGeneratePrivateKey(cfg.KeyEscrowKeyPath)
	if err != nil {
		log.Fatal(err)
	}
	defer clear(escrow[:])
	store, err := platform.OpenStore(cfg.DoltDSN, cfg.CorpusDoltDSN)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()
	svc := platform.NewService(store, cfg.ArtifactsRoot, cfg.SigningKeyPath)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
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
