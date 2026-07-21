package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/receiptsigner"
)

func main() {
	mode := flag.String("mode", "", "Signer mode: guest-core or verifier-control")
	socket := flag.String("socket", "", "Absolute Unix socket path")
	keyPath := flag.String("key", "", "Absolute private key path")
	stateRoot := flag.String("state-root", "", "Absolute durable receipt state root")
	startupStatus := flag.String("startup-status", "", "Optional absolute boot-local startup stage path")
	computerID := flag.String("computer-id", os.Getenv("CHOIR_COMPUTER_ID"), "Stable ComputerID")
	flag.Parse()
	if *startupStatus != "" && !filepath.IsAbs(*startupStatus) {
		fmt.Fprintln(os.Stderr, "choir-receipt-signer: absolute startup status path is required")
		os.Exit(2)
	}
	recordStartupStage := func(stage receiptsigner.StartupStage) {
		if *startupStatus == "" {
			return
		}
		if err := receiptsigner.WriteStartupStage(*startupStatus, stage); err != nil {
			fmt.Fprintln(os.Stderr, "choir-receipt-signer: write startup status")
			os.Exit(1)
		}
	}
	recordStartupStage(receiptsigner.StartupStageStarted)
	if flag.NArg() != 0 || !filepath.IsAbs(*socket) || !filepath.IsAbs(*keyPath) || !filepath.IsAbs(*stateRoot) || strings.TrimSpace(*computerID) == "" {
		fmt.Fprintln(os.Stderr, "choir-receipt-signer: complete absolute configuration is required")
		os.Exit(2)
	}
	recordStartupStage(receiptsigner.StartupStageConfigured)
	key, err := receiptsigner.LoadOrCreateSigningKey(*keyPath, strings.TrimSpace(*mode))
	if err != nil {
		fmt.Fprintf(os.Stderr, "choir-receipt-signer: load key: %v\n", err)
		os.Exit(1)
	}
	recordStartupStage(receiptsigner.StartupStageKeyLoaded)
	handler, err := receiptsigner.NewHandler(*mode, *computerID, *stateRoot, key)
	if err != nil {
		fmt.Fprintf(os.Stderr, "choir-receipt-signer: configure: %v\n", err)
		os.Exit(1)
	}
	recordStartupStage(receiptsigner.StartupStageHandlerConfigured)
	if err := os.MkdirAll(filepath.Dir(*socket), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "choir-receipt-signer: socket directory: %v\n", err)
		os.Exit(1)
	}
	_ = os.Remove(*socket)
	listener, err := net.Listen("unix", *socket)
	if err != nil {
		fmt.Fprintf(os.Stderr, "choir-receipt-signer: listen: %v\n", err)
		os.Exit(1)
	}
	defer listener.Close()
	if err := os.Chmod(*socket, 0o660); err != nil {
		fmt.Fprintf(os.Stderr, "choir-receipt-signer: socket permissions: %v\n", err)
		os.Exit(1)
	}
	recordStartupStage(receiptsigner.StartupStageSocketListening)
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	serveErr := server.Serve(listener)
	recordStartupStage(receiptsigner.ClassifyServeExit(serveErr))
	if serveErr != nil && serveErr != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "choir-receipt-signer: serve: %v\n", serveErr)
		os.Exit(1)
	}
}
