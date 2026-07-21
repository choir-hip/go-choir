//go:build linux

package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/yusefmosiah/go-choir/internal/receiptsigner"
	"github.com/yusefmosiah/go-choir/internal/updater"
)

func main() {
	if handled, code := updater.RunKernelCapabilityProbeHelper(); handled {
		os.Exit(code)
	}
	if handled, code := updater.RunKernelCapabilityProbeWriter(context.Background()); handled {
		os.Exit(code)
	}
	var root, socketPath, computerID, realizationID, restartRequestPath, recoveryRequestPath, cleanupRequestPath, restartPrepareURL, healthURL, signerSocketPath, guestImageManifestPath, kernelConfigPath, kernelProbePath string
	flag.StringVar(&root, "root", "/var/lib/choir-updater", "Root-owned updater state directory")
	flag.StringVar(&socketPath, "socket", "/run/choir/updater.sock", "Permissioned updater Unix socket")
	flag.StringVar(&computerID, "computer-id", os.Getenv("CHOIR_COMPUTER_ID"), "Stable ComputerID")
	flag.StringVar(&realizationID, "realization-id", os.Getenv("CHOIR_REALIZATION_ID"), "Current realization identity")
	flag.StringVar(&restartRequestPath, "restart-request", "/run/choir-updater-control/restart", "Fixed systemd path-unit restart request")
	flag.StringVar(&recoveryRequestPath, "recovery-restart-request", "/run/choir-updater-control/recover", "Fixed root-owned recovery restart request")
	flag.StringVar(&cleanupRequestPath, "recovery-cleanup-request", "/run/choir-updater-control/cleanup", "Fixed root-owned recovery credential cleanup request")
	flag.StringVar(&restartPrepareURL, "restart-prepare-url", "http://127.0.0.1:8085/internal/self-development/restart-handoff", "Fixed guest restart credential preparation endpoint")
	flag.StringVar(&healthURL, "health-url", "http://127.0.0.1:8085/health", "Guest Choir health endpoint")
	flag.StringVar(&signerSocketPath, "signer-socket", "/run/choir-signers/guest-core.sock", "Isolated guest-core signer Unix socket")
	flag.StringVar(&guestImageManifestPath, "guest-image-manifest", os.Getenv("CHOIR_GUEST_IMAGE_MANIFEST"), "Immutable guest image manifest")
	flag.StringVar(&kernelConfigPath, "kernel-config", os.Getenv("CHOIR_KERNEL_CONFIG"), "Realized guest kernel config")
	flag.StringVar(&kernelProbePath, "kernel-probe", "/run/choir/kernel-capabilities.json", "Boot-time kernel capability probe artifact")
	flag.Parse()
	if strings.TrimSpace(computerID) == "" || strings.TrimSpace(realizationID) == "" {
		fatal("computer and realization identities are required")
	}
	guestSigner, err := receiptsigner.NewClient(signerSocketPath, receiptsigner.ModeGuestCore)
	if err != nil {
		fatal("configure guest signer: %v", err)
	}
	engine, err := updater.New(filepath.Clean(root), computerID, realizationID, updater.RestartRequestManager{Path: restartRequestPath, RecoveryPath: recoveryRequestPath, CleanupPath: cleanupRequestPath, PrepareURL: restartPrepareURL}, updater.HTTPHealthProber{URL: healthURL}, guestSigner)
	if err != nil {
		fatal("initialize: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(socketPath), 0o700); err != nil {
		fatal("create socket directory: %v", err)
	}
	if err := removeStaleSocket(socketPath); err != nil {
		fatal("prepare socket: %v", err)
	}
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		fatal("listen: %v", err)
	}
	defer listener.Close()
	defer os.Remove(socketPath)
	if err := os.Chmod(socketPath, 0o600); err != nil {
		fatal("protect socket: %v", err)
	}
	server := &http.Server{ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 30 * time.Second}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/public-key", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		ref, publicKey, err := guestSigner.PublicKey(r.Context())
		if err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "signing key unavailable"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{
			"signer_domain": ref.SignerDomain, "key_id": ref.KeyID,
			"public_key": base64.RawStdEncoding.EncodeToString(publicKey),
		})
	})
	mux.HandleFunc("/v1/kernel-capabilities", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		var request updater.KernelCapabilityRequest
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid kernel capability request"})
			return
		}
		guestImageDigest, imageErr := updater.DigestFile(guestImageManifestPath)
		kernelConfigDigest, configErr := updater.DigestFile(kernelConfigPath)
		probe, probeErr := updater.ReadKernelCapabilityProbe(kernelProbePath)
		generation, generationErr := strconv.ParseUint(strings.TrimSpace(os.Getenv("VM_EPOCH")), 10, 64)
		if imageErr != nil || configErr != nil || probeErr != nil || generationErr != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "mandatory kernel capability probe unavailable"})
			return
		}
		report, err := updater.NewKernelCapabilityReport(r.Context(), updater.KernelCapabilityIdentity{
			ComputerID: computerID, RealizationID: realizationID,
			GuestImageDigest: guestImageDigest, KernelConfigDigest: kernelConfigDigest,
			LifecycleGeneration: generation,
		}, request, probe, guestSigner, time.Now().UTC())
		if err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "mandatory kernel capability receipt refused"})
			return
		}
		writeJSON(w, http.StatusOK, report)
	})
	mux.HandleFunc("/v1/import-baseline", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		var request updater.BaselineImportRequest
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid baseline import request"})
			return
		}
		manifest, err := engine.ImportBaseline(r.Context(), request)
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, updater.ErrIdempotencyConflict) {
				status = http.StatusConflict
			}
			writeJSON(w, status, map[string]string{"error": "updater refused baseline import"})
			return
		}
		writeJSON(w, http.StatusOK, manifest)
	})
	mux.HandleFunc("/v1/admit-current", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		admitted, err := engine.AdmitCurrent(r.Context())
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "no authorized dynamic release"})
			return
		}
		writeJSON(w, http.StatusOK, admitted)
	})
	mux.HandleFunc("/v1/apply", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		var request updater.ApplyRequest
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid apply request"})
			return
		}
		result, err := engine.Apply(r.Context(), request)
		if err != nil {
			if result.RecoveryReceipt != nil {
				writeJSON(w, http.StatusConflict, struct {
					Result updater.ApplyResult `json:"result"`
					Error  string              `json:"error"`
				}{result, "materialization failed and prior release was restored"})
				return
			}
			status := http.StatusBadRequest
			if errors.Is(err, updater.ErrIdempotencyConflict) {
				status = http.StatusConflict
			}
			writeJSON(w, status, map[string]string{"error": "updater refused request"})
			return
		}
		writeJSON(w, http.StatusOK, result)
	})
	server.Handler = mux
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	if err := server.Serve(peerListener{Listener: listener, uid: uint32(os.Geteuid())}); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fatal("serve: %v", err)
	}
}

type peerListener struct {
	net.Listener
	uid uint32
}

func (l peerListener) Accept() (net.Conn, error) {
	for {
		connection, err := l.Listener.Accept()
		if err != nil {
			return nil, err
		}
		unixConnection, ok := connection.(*net.UnixConn)
		if !ok {
			_ = connection.Close()
			continue
		}
		raw, err := unixConnection.SyscallConn()
		if err != nil {
			_ = connection.Close()
			continue
		}
		var credential *unix.Ucred
		var credentialErr error
		if err := raw.Control(func(fd uintptr) {
			credential, credentialErr = unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
		}); err != nil || credentialErr != nil || credential == nil || credential.Uid != l.uid {
			_ = connection.Close()
			continue
		}
		return connection, nil
	}
}

func removeStaleSocket(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("refuse to remove non-socket path")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Geteuid()) {
		return fmt.Errorf("refuse to remove socket owned by another user")
	}
	return os.Remove(path)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func fatal(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, "choir-updater: "+format+"\n", args...)
	os.Exit(1)
}
