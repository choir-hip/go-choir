package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/yusefmosiah/go-choir/internal/projectionbase"
)

func main() {
	fs := flag.NewFlagSet("choir-rebuild-base", flag.ExitOnError)
	computerID := fs.String("computer", "", "Computer ID (e.g. computer-03335285269bdba4f94377e56879f9e6)")
	targetHead := fs.String("target-head", "", "Target canonical event head SHA-256")
	artifactsRoot := fs.String("artifacts-root", "/var/lib/go-choir/platform-artifacts", "Path to platform-artifacts directory")
	scratchDir := fs.String("scratch-dir", "", "Scratch directory for projection reconstruction (defaults to temp dir)")
	keyFile := fs.String("key-file", "", "Path to privacy key file or mode-0400 guest key JSON")
	keyHex := fs.String("key-hex", "", "Hex-encoded 32-byte privacy key")
	batchSize := fs.Int("batch-size", projectionbase.DefaultBatchSize, "Number of events per database transaction")
	memoryLimitMB := fs.Int64("memory-limit-mb", 2048, "Memory limit in MB for replay process")

	if err := fs.Parse(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "choir-rebuild-base: %v\n", err)
		os.Exit(2)
	}

	if strings.TrimSpace(*computerID) == "" || strings.TrimSpace(*targetHead) == "" {
		fmt.Fprintln(os.Stderr, "choir-rebuild-base: --computer and --target-head are required")
		fs.Usage()
		os.Exit(2)
	}

	var keyMaterial []byte
	if strings.TrimSpace(*keyHex) != "" {
		raw, err := hex.DecodeString(strings.TrimSpace(*keyHex))
		if err != nil || len(raw) != 32 {
			fmt.Fprintf(os.Stderr, "choir-rebuild-base: invalid --key-hex: must be 64 hex characters (32 bytes)\n")
			os.Exit(2)
		}
		keyMaterial = raw
	} else if strings.TrimSpace(*keyFile) != "" {
		raw, err := os.ReadFile(filepath.Clean(*keyFile))
		if err != nil {
			fmt.Fprintf(os.Stderr, "choir-rebuild-base: read --key-file: %v\n", err)
			os.Exit(2)
		}
		// Check if it's raw 32-byte key or JSON key file.
		if len(raw) == 32 {
			keyMaterial = raw
		} else {
			var kf struct {
				Key string `json:"key"`
			}
			if err := json.Unmarshal(raw, &kf); err == nil && kf.Key != "" {
				// Base64-decoded in computerevent.
				keyMaterial = []byte(kf.Key)
			} else {
				keyMaterial = raw
			}
		}
	} else {
		fmt.Fprintln(os.Stderr, "choir-rebuild-base: either --key-hex or --key-file is required")
		os.Exit(2)
	}

	cfg := projectionbase.Config{
		ComputerID:     strings.TrimSpace(*computerID),
		TargetHead:     strings.TrimSpace(*targetHead),
		ArtifactsRoot:  filepath.Clean(*artifactsRoot),
		ScratchDir:     *scratchDir,
		KeyMaterial:    keyMaterial,
		BatchSize:      *batchSize,
		MemoryLimitRSS: *memoryLimitMB * 1024 * 1024,
	}

	rebuilder, err := projectionbase.NewRebuilder(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "choir-rebuild-base: config error: %v\n", err)
		os.Exit(2)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	diskSource := projectionbase.NewDiskEventSource(cfg.ArtifactsRoot, cfg.ComputerID)

	fmt.Printf("Starting offline projection base rebuild for %s through head %s...\n", cfg.ComputerID, cfg.TargetHead)
	result, err := rebuilder.Run(ctx, diskSource)
	if err != nil {
		fmt.Fprintf(os.Stderr, "choir-rebuild-base: rebuild failed: %v\n", err)
		os.Exit(1)
	}

	out, err := json.MarshalIndent(result.Descriptor, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "choir-rebuild-base: marshal result: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("ProjectionBase successfully published:")
	fmt.Println(string(out))
	fmt.Printf("Blob artifact path: %s\n", result.BlobPath)
}
