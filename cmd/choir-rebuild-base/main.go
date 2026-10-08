package main

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/projectionbase"
)

// cliResult is the machine-readable run result. Parent workers read
// --result-file (or --json stdout) instead of parsing human logs.
type cliResult struct {
	OK                bool                       `json:"ok"`
	Error             string                     `json:"error,omitempty"`
	ComputerID        string                     `json:"computer_id"`
	TargetHead        string                     `json:"target_head"`
	TargetSequence    uint64                     `json:"target_sequence"`
	WatermarkSequence uint64                     `json:"watermark_sequence"`
	BaseRef           string                     `json:"base_ref"`
	BlobPath          string                     `json:"blob_path"`
	Seeded            bool                       `json:"seeded"`
	SeedSequence      uint64                     `json:"seed_sequence,omitempty"`
	SeedBaseRef       string                     `json:"seed_base_ref,omitempty"`
	Descriptor        *projectionbase.Descriptor `json:"descriptor,omitempty"`
}

func main() {
	fs := flag.NewFlagSet("choir-rebuild-base", flag.ExitOnError)
	computerID := fs.String("computer", "", "Computer ID (e.g. computer-03335285269bdba4f94377e56879f9e6)")
	targetHead := fs.String("target-head", "", "Frozen target canonical event head SHA-256")
	targetSequence := fs.Uint64("target-sequence", 0, "Frozen target event sequence (0 derives it from the chain head)")
	artifactsRoot := fs.String("artifacts-root", "/var/lib/go-choir/platform-artifacts", "Path to platform-artifacts directory")
	scratchDir := fs.String("scratch-dir", "", "Durable scratch directory for incremental reconstruction (defaults to a temp dir)")
	keyFile := fs.String("key-file", "", "Path to privacy key file or guest key JSON; '-' reads the key JSON from stdin")
	keyHex := fs.String("key-hex", "", "Hex-encoded 32-byte privacy key")
	advertise := fs.Bool("advertise", false, "POST the published blob as this computer's advertised watermark")
	platformURL := fs.String("platform-url", os.Getenv("CHOIR_PLATFORM_URL"), "Platform URL for --source http and --advertise (or CHOIR_PLATFORM_URL)")
	memoryLimitMB := fs.Int64("memory-limit-mb", 2048, "Maximum replay RSS growth in MB above the run baseline")
	sourceKind := fs.String("source", "http", "Event source: http (platform replay endpoint, receipt-verified)")
	ownerID := fs.String("owner", os.Getenv("CHOIR_OWNER_ID"), "Owner user ID for --source http internal-caller auth (or CHOIR_OWNER_ID)")
	capability := fs.String("capability", os.Getenv("CHOIR_PLATFORM_CAPABILITY"), "Bearer capability for --advertise (or CHOIR_PLATFORM_CAPABILITY)")
	batchSize := fs.Int("batch-size", projectionbase.DefaultBatchSize, "Number of events per database transaction")
	seedKind := fs.String("seed", "auto", "Seed policy: auto (latest compatible verified base) or none (explicit genesis repair)")
	seedBaseRef := fs.String("seed-base-ref", "", "Pin an exact published base blob digest (pinned seed)")
	incremental := fs.Bool("incremental", false, "Require a compatible verified seed; refuse genesis fallback")
	maxTail := fs.Uint64("max-tail", 0, "Refuse when the replay span (frozen target minus seed watermark) exceeds this many events")
	receiptPublicKey := fs.String("receipt-public-key", "", "Pinned corpusd EventHeadReceipt ed25519 public key (hex or base64)")
	receiptKeyID := fs.String("receipt-key-id", "", "Optional pinned signer key id (default pins key material only)")
	receiptDomain := fs.String("receipt-domain", "platform-control", "Pinned receipt signer domain")
	resultFile := fs.String("result-file", "", "Write the JSON run result to this path (atomically, mode 0600)")
	jsonOnly := fs.Bool("json", false, "Print only the JSON run result to stdout")

	if err := fs.Parse(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "choir-rebuild-base: %v\n", err)
		os.Exit(2)
	}

	if strings.TrimSpace(*computerID) == "" || strings.TrimSpace(*targetHead) == "" {
		fmt.Fprintln(os.Stderr, "choir-rebuild-base: --computer and --target-head are required")
		fs.Usage()
		os.Exit(2)
	}

	policy := projectionbase.SeedAuto
	switch strings.ToLower(strings.TrimSpace(*seedKind)) {
	case "", "auto":
		policy = projectionbase.SeedAuto
	case "none":
		policy = projectionbase.SeedNone
	default:
		fmt.Fprintf(os.Stderr, "choir-rebuild-base: --seed must be auto or none (got %q)\n", *seedKind)
		os.Exit(2)
	}
	if strings.TrimSpace(*seedBaseRef) != "" {
		if policy == projectionbase.SeedNone {
			fmt.Fprintln(os.Stderr, "choir-rebuild-base: --seed-base-ref conflicts with --seed none")
			os.Exit(2)
		}
		policy = projectionbase.SeedPinned
	}

	switch strings.ToLower(strings.TrimSpace(*sourceKind)) {
	case "http":
		// The only admitted replay path: the platform replay endpoint returns
		// the original signed event head receipts, which the pinned key below
		// verifies event by event.
	case "disk":
		fmt.Fprintln(os.Stderr, "choir-rebuild-base: --source disk is refused: the artifact store carries no signed event head receipts, so replay cannot verify the original receipts; use --source http with --receipt-public-key")
		os.Exit(2)
	default:
		fmt.Fprintf(os.Stderr, "choir-rebuild-base: unknown --source %q (expected http)\n", *sourceKind)
		os.Exit(2)
	}

	publicKey, err := parseReceiptPublicKey(*receiptPublicKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "choir-rebuild-base: %v\n", err)
		os.Exit(2)
	}

	keyMaterial, err := readKeyMaterial(strings.TrimSpace(*keyHex), strings.TrimSpace(*keyFile))
	if err != nil {
		fmt.Fprintf(os.Stderr, "choir-rebuild-base: %v\n", err)
		os.Exit(2)
	}
	zeroKey := func() {
		for i := range keyMaterial {
			keyMaterial[i] = 0
		}
	}

	cfg := projectionbase.Config{
		ComputerID:     strings.TrimSpace(*computerID),
		TargetHead:     strings.TrimSpace(*targetHead),
		TargetSequence: *targetSequence,
		ArtifactsRoot:  filepath.Clean(*artifactsRoot),
		ScratchDir:     *scratchDir,
		KeyMaterial:    keyMaterial,
		BatchSize:      *batchSize,
		MemoryLimitRSS: *memoryLimitMB * 1024 * 1024,
		SeedPolicy:     policy,
		SeedBaseRef:    strings.TrimSpace(*seedBaseRef),
		SeedRequired:   *incremental,
		MaxTailEvents:  *maxTail,
	}
	rebuilder, err := projectionbase.NewRebuilder(cfg)
	if err != nil {
		zeroKey()
		fmt.Fprintf(os.Stderr, "choir-rebuild-base: config error: %v\n", err)
		os.Exit(2)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	httpClient, err := computerevent.NewHTTPClient(
		*platformURL,
		&http.Client{Transport: &internalCallerTransport{ownerID: strings.TrimSpace(*ownerID)}},
		func(context.Context) (string, error) { return "internal", nil },
		true,
	)
	if err != nil {
		zeroKey()
		fmt.Fprintf(os.Stderr, "choir-rebuild-base: http source: %v\n", err)
		os.Exit(2)
	}
	replaySource := &httpReplaySource{
		HTTPClient: httpClient,
		verifier: computerevent.EventHeadReceiptVerifier{Keys: projectionbase.PinnedReceiptKeyResolver{
			SignerDomain: strings.TrimSpace(*receiptDomain),
			KeyID:        strings.TrimSpace(*receiptKeyID),
			PublicKey:    publicKey,
		}},
	}

	if !*jsonOnly {
		fmt.Printf("Starting projection base rebuild for %s through head %s (source=http, seed=%s)...\n", cfg.ComputerID, cfg.TargetHead, policy)
	}
	result, err := rebuilder.Run(ctx, replaySource)
	zeroKey()
	if err != nil {
		res := cliResult{OK: false, Error: err.Error(), ComputerID: cfg.ComputerID, TargetHead: cfg.TargetHead, TargetSequence: cfg.TargetSequence}
		writeResultFile(*resultFile, res)
		if *jsonOnly {
			printResultJSON(res)
		} else {
			fmt.Fprintf(os.Stderr, "choir-rebuild-base: rebuild failed: %v\n", err)
		}
		os.Exit(1)
	}

	res := cliResult{
		OK:                true,
		ComputerID:        cfg.ComputerID,
		TargetHead:        result.Descriptor.CanonicalHead,
		TargetSequence:    result.TargetSequence,
		WatermarkSequence: result.Descriptor.Sequence,
		BaseRef:           result.Descriptor.BlobSHA256,
		BlobPath:          result.BlobPath,
		Descriptor:        &result.Descriptor,
	}
	if result.Seed != nil {
		res.Seeded = true
		res.SeedSequence = result.Seed.Sequence
		res.SeedBaseRef = result.Seed.BlobSHA256
	}
	writeResultFile(*resultFile, res)
	if *jsonOnly {
		printResultJSON(res)
	} else {
		out, err := json.MarshalIndent(result.Descriptor, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "choir-rebuild-base: marshal result: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("ProjectionBase successfully published:")
		fmt.Println(string(out))
		fmt.Printf("Blob artifact path: %s\n", result.BlobPath)
		if result.Seed != nil {
			fmt.Printf("Seeded from base %s at sequence %d; replayed tail (%d,%d]\n", result.Seed.BlobSHA256, result.Seed.Sequence, result.Seed.Sequence, result.Descriptor.Sequence)
		} else {
			fmt.Printf("Genesis replay (no seed) reached sequence %d\n", result.Descriptor.Sequence)
		}
	}

	if *advertise {
		if err := projectionbase.AdvertiseWatermark(ctx, *platformURL, *capability, cfg.ComputerID, result.Descriptor.Sequence, result.Descriptor.BlobSHA256); err != nil {
			fmt.Fprintf(os.Stderr, "choir-rebuild-base: advertise watermark failed: %v\n", err)
			os.Exit(1)
		}
		if !*jsonOnly {
			fmt.Printf("Advertised watermark sequence %d (%s)\n", result.Descriptor.Sequence, result.Descriptor.BlobSHA256)
		}
	}
}

func printResultJSON(res cliResult) {
	raw, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "choir-rebuild-base: encode result: %v\n", err)
		return
	}
	fmt.Println(string(raw))
}

func writeResultFile(path string, res cliResult) {
	path = strings.TrimSpace(path)
	if path == "" {
		return
	}
	raw, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "choir-rebuild-base: encode result file: %v\n", err)
		os.Exit(1)
	}
	path = filepath.Clean(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "choir-rebuild-base: create result directory: %v\n", err)
		os.Exit(1)
	}
	tmpFile, err := os.CreateTemp(filepath.Dir(path), ".result-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "choir-rebuild-base: create result file: %v\n", err)
		os.Exit(1)
	}
	tmpPath := tmpFile.Name()
	if _, err := tmpFile.Write(append(raw, '\n')); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		fmt.Fprintf(os.Stderr, "choir-rebuild-base: write result file: %v\n", err)
		os.Exit(1)
	}
	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		fmt.Fprintf(os.Stderr, "choir-rebuild-base: fsync result file: %v\n", err)
		os.Exit(1)
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		fmt.Fprintf(os.Stderr, "choir-rebuild-base: close result file: %v\n", err)
		os.Exit(1)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		fmt.Fprintf(os.Stderr, "choir-rebuild-base: publish result file: %v\n", err)
		os.Exit(1)
	}
	if dir, err := os.Open(filepath.Dir(path)); err == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}
}

// readKeyMaterial reads the guest privacy key without ever echoing it: either
// a 64-hex flag, a raw 32-byte file, or guest key JSON ({"key": base64}) from a
// file or stdin ("-"). Every failure message is key-free.
func readKeyMaterial(keyHex, keyFile string) ([]byte, error) {
	if keyHex != "" {
		raw, err := hex.DecodeString(keyHex)
		if err != nil || len(raw) != 32 {
			return nil, fmt.Errorf("invalid --key-hex: must be 64 hex characters (32 bytes)")
		}
		return raw, nil
	}
	if keyFile == "" {
		return nil, fmt.Errorf("either --key-hex or --key-file is required")
	}
	var raw []byte
	var err error
	if keyFile == "-" {
		raw, err = io.ReadAll(io.LimitReader(os.Stdin, 1<<20))
	} else {
		raw, err = os.ReadFile(filepath.Clean(keyFile))
	}
	if err != nil {
		return nil, fmt.Errorf("read key material: %w", err)
	}
	if len(raw) == 32 {
		return raw, nil
	}
	var keyJSON struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(raw, &keyJSON); err == nil && keyJSON.Key != "" {
		if decoded, err := base64.StdEncoding.DecodeString(keyJSON.Key); err == nil && len(decoded) == 32 {
			return decoded, nil
		}
		if decoded, err := base64.RawStdEncoding.DecodeString(keyJSON.Key); err == nil && len(decoded) == 32 {
			return decoded, nil
		}
		if len(keyJSON.Key) == 32 {
			return []byte(keyJSON.Key), nil
		}
		return nil, fmt.Errorf("key material JSON does not decode to 32 bytes")
	}
	return nil, fmt.Errorf("key material must be 32 raw bytes or JSON with a 32-byte key")
}

// parseReceiptPublicKey pins the corpusd EventHeadReceipt signing key from hex
// or base64. A missing or malformed key refuses: there is no unverified path.
func parseReceiptPublicKey(raw string) (ed25519.PublicKey, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("--receipt-public-key is required for --source http: replay must verify original signed receipts")
	}
	if decoded, err := hex.DecodeString(raw); err == nil && len(decoded) == ed25519.PublicKeySize {
		return ed25519.PublicKey(decoded), nil
	}
	for _, encoding := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		if decoded, err := encoding.DecodeString(raw); err == nil && len(decoded) == ed25519.PublicKeySize {
			return ed25519.PublicKey(decoded), nil
		}
	}
	return nil, fmt.Errorf("--receipt-public-key must be a 32-byte ed25519 public key in hex or base64")
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
// projectionbase.CASReplaySource. Receipt verification is real: every replayed
// event head receipt must verify against the pinned corpusd key.
type httpReplaySource struct {
	*computerevent.HTTPClient
	verifier computerevent.ReceiptVerifier
}

func (s *httpReplaySource) VerifyEventHeadReceipt(ctx context.Context, receipt computerevent.Receipt, request computerevent.CASRequest) error {
	if s.verifier == nil {
		return fmt.Errorf("%w: receipt verifier is not configured", projectionbase.ErrReceiptUnverifiable)
	}
	return s.verifier.VerifyEventHeadReceipt(ctx, receipt, request)
}

var _ projectionbase.CASReplaySource = (*httpReplaySource)(nil)
