package main

import (
	"context"
	"crypto/ed25519"
	"embed"
	"encoding/pem"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"golang.org/x/crypto/ssh"
)

//go:embed all:frontend/dist
var assets embed.FS

// appInfo holds build-time metadata injected via ldflags.
var (
	appVersion = "dev"
	appCommit  = "unknown"
	appBuiltAt = "unknown"
)

// Service ports for local mode.
const (
	localProxyPort    = "8082"
	localAuthPort     = "8081"
	localGatewayPort  = "8084"
	localSandboxPort  = "8085"
	localFrontendPort = "3000"
)

// dataDir is the base directory for local service state.
const dataDir = "/tmp/choir-desktop"

// DesktopService exposes app metadata and local status to the frontend.
type DesktopService struct {
	localMode bool
	backend   string
}

func (d *DesktopService) GetAppInfo() map[string]string {
	mode := "cloud"
	if d.localMode {
		mode = "local"
	}
	return map[string]string{
		"version":  appVersion,
		"commit":   appCommit,
		"builtAt":  appBuiltAt,
		"backend":  d.backend,
		"platform": "wails-v3-desktop",
		"mode":     mode,
	}
}

func (d *DesktopService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	return nil
}

func (d *DesktopService) ServiceShutdown() error {
	return nil
}

// ─── Local service manager ──────────────────────────────────────────────

// serviceProcess wraps a child process with metadata for cleanup.
type serviceProcess struct {
	name string
	cmd  *exec.Cmd
}

// ensureSigningKey generates an Ed25519 key pair in OpenSSH format if the
// private key file does not exist. The public key is written alongside it
// with a .pub suffix so the proxy can load it.
func ensureSigningKey(keyPath string) error {
	if _, err := os.Stat(keyPath); err == nil {
		return nil
	}

	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		return fmt.Errorf("generate ed25519 key: %w", err)
	}

	block, err := ssh.MarshalPrivateKey(priv, "choir-desktop")
	if err != nil {
		return fmt.Errorf("marshal private key: %w", err)
	}
	privData := pem.EncodeToMemory(block)
	if err := os.WriteFile(keyPath, privData, 0o600); err != nil {
		return fmt.Errorf("write private key: %w", err)
	}

	pubKey, err := ssh.NewPublicKey(priv.Public())
	if err != nil {
		return fmt.Errorf("derive public key: %w", err)
	}
	pubData := ssh.MarshalAuthorizedKey(pubKey)
	if err := os.WriteFile(keyPath+".pub", pubData, 0o600); err != nil {
		return fmt.Errorf("write public key: %w", err)
	}

	log.Printf("Generated Ed25519 signing key at %s", keyPath)
	return nil
}

// startLocalServices launches auth, gateway, sandbox, and proxy as child
// processes with environment configured for localhost operation.
func startLocalServices(binDir string) ([]*serviceProcess, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	// Generate Ed25519 signing key if not present.
	keyPath := filepath.Join(dataDir, "auth-signing-key")
	if err := ensureSigningKey(keyPath); err != nil {
		return nil, fmt.Errorf("generate signing key: %w", err)
	}

	// Common environment for all services.
	baseEnv := []string{
		"SERVER_HOST=127.0.0.1",
	}

	// Auth service env.
	authEnv := append(baseEnv,
		fmt.Sprintf("AUTH_PORT=%s", localAuthPort),
		fmt.Sprintf("AUTH_DB_PATH=%s/auth.db", dataDir),
		"AUTH_RP_ID=localhost",
		fmt.Sprintf("AUTH_RP_ORIGINS=http://localhost:%s", localFrontendPort),
		fmt.Sprintf("AUTH_JWT_PRIVATE_KEY_PATH=%s/auth-signing-key", dataDir),
		"AUTH_COOKIE_SECURE=false",
	)

	// Gateway service env.
	gatewayEnv := append(baseEnv,
		fmt.Sprintf("GATEWAY_PORT=%s", localGatewayPort),
		fmt.Sprintf("GATEWAY_IDENTITY_STORE_PATH=%s/gateway-identity.json", dataDir),
	)

	// Sandbox service env.
	sandboxEnv := append(baseEnv,
		fmt.Sprintf("SANDBOX_PORT=%s", localSandboxPort),
		fmt.Sprintf("SANDBOX_ID=desktop-local"),
		fmt.Sprintf("RUNTIME_STORE_PATH=%s/runtime-store", dataDir),
		fmt.Sprintf("SANDBOX_FILES_ROOT=%s/files", dataDir),
		fmt.Sprintf("RUNTIME_GATEWAY_URL=http://127.0.0.1:%s", localGatewayPort),
		"RUNTIME_GATEWAY_TOKEN=desktop-local-token",
	)

	// Proxy service env.
	proxyEnv := append(baseEnv,
		fmt.Sprintf("PROXY_PORT=%s", localProxyPort),
		fmt.Sprintf("PROXY_SANDBOX_URL=http://127.0.0.1:%s", localSandboxPort),
		fmt.Sprintf("PROXY_AUTH_PUBLIC_KEY_PATH=%s/auth-signing-key.pub", dataDir),
	)

	services := []struct {
		name string
		bin  string
		env  []string
	}{
		{"auth", "auth", authEnv},
		{"gateway", "gateway", gatewayEnv},
		{"sandbox", "sandbox", sandboxEnv},
		{"proxy", "proxy", proxyEnv},
	}

	var procs []*serviceProcess

	for _, svc := range services {
		binPath := filepath.Join(binDir, svc.bin)
		if _, err := os.Stat(binPath); err != nil {
			return nil, fmt.Errorf("service binary not found: %s (build with 'task build:services')", binPath)
		}

		cmd := exec.Command(binPath)
		cmd.Env = append(os.Environ(), svc.env...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		log.Printf("Starting %s service...", svc.name)
		if err := cmd.Start(); err != nil {
			for _, p := range procs {
				_ = p.cmd.Process.Kill()
			}
			return nil, fmt.Errorf("start %s: %w", svc.name, err)
		}

		procs = append(procs, &serviceProcess{name: svc.name, cmd: cmd})
	}

	if err := waitForServices(procs); err != nil {
		for _, p := range procs {
			_ = p.cmd.Process.Kill()
		}
		return nil, err
	}

	return procs, nil
}

// waitForServices polls each service's /health endpoint until ready.
func waitForServices(procs []*serviceProcess) error {
	client := &http.Client{Timeout: 2 * time.Second}
	ports := map[string]string{
		"auth":    localAuthPort,
		"gateway": localGatewayPort,
		"sandbox": localSandboxPort,
		"proxy":   localProxyPort,
	}

	for _, proc := range procs {
		port := ports[proc.name]
		healthURL := fmt.Sprintf("http://127.0.0.1:%s/health", port)

		var lastErr error
		for i := 0; i < 30; i++ {
			resp, err := client.Get(healthURL)
			if err == nil && resp.StatusCode == 200 {
				resp.Body.Close()
				log.Printf("  %s: ready (port %s)", proc.name, port)
				break
			}
			if err == nil {
				resp.Body.Close()
			}
			lastErr = err
			time.Sleep(500 * time.Millisecond)
		}

		if proc.cmd.ProcessState != nil && proc.cmd.ProcessState.Exited() {
			return fmt.Errorf("service %s exited unexpectedly", proc.name)
		}

		if lastErr != nil {
			log.Printf("  %s: health check warning (port %s): %v", proc.name, port, lastErr)
		}
	}

	return nil
}

// stopLocalServices terminates all child processes gracefully.
func stopLocalServices(procs []*serviceProcess) {
	for _, proc := range procs {
		if proc.cmd.Process != nil {
			log.Printf("Stopping %s service...", proc.name)
			_ = proc.cmd.Process.Signal(os.Interrupt)
		}
	}
	time.Sleep(2 * time.Second)
	for _, proc := range procs {
		if proc.cmd.Process != nil {
			_ = proc.cmd.Process.Kill()
		}
	}
}

// ─── Local frontend server ──────────────────────────────────────────────

// startLocalFrontendServer serves embedded frontend assets on localhost and
// proxies /auth/* and /api/* to the local services. This gives the Wails
// WebView a real http://localhost origin so WebAuthn passkeys work.
func startLocalFrontendServer() (*http.Server, error) {
	mux := http.NewServeMux()

	authProxy := newReverseProxy("http://127.0.0.1:" + localAuthPort)
	mux.HandleFunc("/auth/", authProxy.ServeHTTP)

	apiProxy := newReverseProxy("http://127.0.0.1:" + localProxyPort)
	mux.HandleFunc("/api/", apiProxy.ServeHTTP)

	frontendFS, err := fs.Sub(assets, "frontend/dist")
	if err != nil {
		return nil, fmt.Errorf("embed sub: %w", err)
	}
	fileServer := http.FileServer(http.FS(frontendFS))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/" {
			path = "/index.html"
		}
		if _, err := fs.Stat(frontendFS, strings.TrimPrefix(path, "/")); err != nil {
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	})

	ln, err := net.Listen("tcp", "127.0.0.1:"+localFrontendPort)
	if err != nil {
		return nil, fmt.Errorf("listen on port %s: %w", localFrontendPort, err)
	}

	srv := &http.Server{Handler: mux}
	go func() {
		log.Printf("Frontend server on http://localhost:%s", localFrontendPort)
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("Frontend server error: %v", err)
		}
	}()

	return srv, nil
}

func newReverseProxy(target string) *httputil.ReverseProxy {
	u, err := url.Parse(target)
	if err != nil {
		log.Fatalf("invalid proxy target %q: %v", target, err)
	}
	proxy := httputil.NewSingleHostReverseProxy(u)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = u.Host
	}
	return proxy
}

// ─── Main ───────────────────────────────────────────────────────────────

func main() {
	localMode := os.Getenv("CHOIR_MODE") != "cloud"
	backend := os.Getenv("CHOIR_BACKEND")
	if backend == "" {
		backend = "https://choir.news"
	}

	var procs []*serviceProcess
	var frontendSrv *http.Server
	var windowURL string

	if localMode {
		log.Printf("Choir Desktop starting in LOCAL mode — version: %s", appVersion)

		binDir := filepath.Join(filepath.Dir(os.Args[0]), "..", "..", "..", "bin")
		if abs, err := filepath.Abs(binDir); err == nil {
			binDir = abs
		}

		var err error
		procs, err = startLocalServices(binDir)
		if err != nil {
			log.Fatalf("Failed to start local services: %v\nBuild them with: task build:services", err)
		}
		defer stopLocalServices(procs)

		frontendSrv, err = startLocalFrontendServer()
		if err != nil {
			log.Fatalf("Failed to start frontend server: %v", err)
		}
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = frontendSrv.Shutdown(ctx)
		}()

		windowURL = fmt.Sprintf("http://localhost:%s", localFrontendPort)
	} else {
		log.Printf("Choir Desktop starting in CLOUD mode — backend: %s, version: %s", backend, appVersion)
		windowURL = "/"
	}

	app := application.New(application.Options{
		Name:        "Choir",
		Description: "Choir — your automatic computer, native on macOS",
		Services: []application.Service{
			application.NewService(&DesktopService{
				localMode: localMode,
				backend:   backend,
			}),
		},
		Assets: application.AssetOptions{
			Handler: assetHandler(backend),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "Choir",
		Name:      "main",
		Width:     1280,
		Height:    800,
		MinWidth:  900,
		MinHeight: 600,
		URL:       windowURL,
	})

	_ = window

	if err := app.Run(); err != nil {
		log.Fatal(fmt.Errorf("Choir Desktop exited with error: %w", err))
	}
}

// assetHandler serves embedded frontend assets and proxies /auth/* and
// /api/* to the staging backend. Used only in cloud mode — in local mode,
// the window loads from the local frontend server which handles all routing.
func assetHandler(backend string) http.Handler {
	embedded := application.AssetFileServerFS(assets)

	proxyTarget, err := url.Parse(backend)
	if err != nil {
		log.Fatalf("invalid backend URL %q: %v", backend, err)
	}

	proxy := httputil.NewSingleHostReverseProxy(proxyTarget)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = proxyTarget.Host
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/auth/") || strings.HasPrefix(r.URL.Path, "/api/") {
			proxy.ServeHTTP(w, r)
			return
		}
		serveEmbedded(w, r, embedded)
	})
}

// serveEmbedded serves embedded frontend assets with SPA fallback.
func serveEmbedded(w http.ResponseWriter, r *http.Request, embedded http.Handler) {
	path := r.URL.Path
	if path == "/" {
		path = "/index.html"
	}
	stripped := strings.TrimPrefix(path, "/")
	if _, err := assets.ReadFile("frontend/dist/" + stripped); err != nil {
		r.URL.Path = "/"
	}
	embedded.ServeHTTP(w, r)
}

func init() {
	http.DefaultTransport = &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}
}
