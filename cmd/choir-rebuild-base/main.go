package main

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
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
	advertise := fs.Bool("advertise", false, "POST the published blob as this computer's advertised watermark")
	platformURL := fs.String("platform-url", os.Getenv("CHOIR_PLATFORM_URL"), "Platform URL for --advertise (or CHOIR_PLATFORM_URL)")
	memoryLimitMB := fs.Int64("memory-limit-mb", 2048, "Memory limit in MB for replay process")
	sourceKind := fs.String("source", "disk", "Event source: disk (artifact store) or http (platform replay endpoint)")
	ownerID := fs.String("owner", os.Getenv("CHOIR_OWNER_ID"), "Owner user ID for --source http internal-caller auth (or CHOIR_OWNER_ID)")
	capability := fs.String("capability", os.Getenv("CHOIR_PLATFORM_CAPABILITY"), "Bearer capability for --advertise (or CHOIR_PLATFORM_CAPABILITY)")
	batchSize := fs.Int("batch-size", projectionbase.DefaultBatchSize, "Number of events per database transaction")

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
				if dec, err := base64.StdEncoding.DecodeString(kf.Key); err == nil && len(dec) == 32 {
					keyMaterial = dec
				} else if dec, err := base64.RawStdEncoding.DecodeString(kf.Key); err == nil && len(dec) == 32 {
					keyMaterial = dec
				} else {
					keyMaterial = []byte(kf.Key)
				}
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

	var replaySource projectionbase.CASReplaySource
	switch strings.TrimSpace(*sourceKind) {
	case "http":
		httpClient, err := computerevent.NewHTTPClient(
			*platformURL,
			&http.Client{Transport: &internalCallerTransport{ownerID: strings.TrimSpace(*ownerID)}},
			func(context.Context) (string, error) { return "internal", nil },
			true,
		)
		if err != nil {
			fmt.Fprintf(os.Stderr, "choir-rebuild-base: http source: %v\n", err)
			os.Exit(2)
		}
		replaySource = &httpReplaySource{HTTPClient: httpClient}
	case "disk":
		replaySource = projectionbase.NewDiskEventSource(cfg.ArtifactsRoot, cfg.ComputerID, cfg.TargetHead)
	default:
		fmt.Fprintf(os.Stderr, "choir-rebuild-base: unknown --source %q (expected disk or http)\n", *sourceKind)
		os.Exit(2)
	}

	fmt.Printf("Starting projection base rebuild for %s through head %s (source=%s)...\n", cfg.ComputerID, cfg.TargetHead, *sourceKind)
	result, err := rebuilder.Run(ctx, replaySource)
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

	if *advertise {
		if err := projectionbase.AdvertiseWatermark(ctx, *platformURL, *capability, cfg.ComputerID, result.Descriptor.Sequence, result.Descriptor.BlobSHA256); err != nil {
			fmt.Fprintf(os.Stderr, "choir-rebuild-base: advertise watermark failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Advertised watermark sequence %d (%s)\n", result.Descriptor.Sequence, result.Descriptor.BlobSHA256)
	}
}

// internalCallerTransport injects X-Internal-Caller and X-Authenticated-User
// headers so the platform event:read endpoints accept this tool as a host-side
// internal caller (authorizeComputerEvent allows event:read without a per-
// computer capability when both headers are present and the transport is
// loopback).
type internalCallerTransport struct {
	ownerID string
}

func (t *internalCallerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("X-Internal-Caller", "true")
	if t.ownerID != "" {
		req.Header.Set("X-Authenticated-User", t.ownerID)
	}
	return http.DefaultTransport.RoundTrip(req)
}

// httpReplaySource wraps computerevent.HTTPClient to satisfy
// projectionbase.CASReplaySource by adding a trivial VerifyEventHeadReceipt
// (mirroring DiskEventSource: kind-check only for offline rebuild).
type httpReplaySource struct {
	*computerevent.HTTPClient
}

func (s *httpReplaySource) VerifyEventHeadReceipt(_ context.Context, receipt computerevent.Receipt, _ computerevent.CASRequest) error {
	if receipt.ReceiptKind != "EventHeadReceipt" {
		return fmt.Errorf("http replay source: receipt kind mismatch: %s", receipt.ReceiptKind)
	}
	return nil
}

var _ projectionbase.CASReplaySource = (*httpReplaySource)(nil)
