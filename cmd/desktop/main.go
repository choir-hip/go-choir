package main

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
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
	localGatewayPort  = "8084"
	localSandboxPort  = "8085"
	localFrontendPort = "3000"
)

// dataDir is the base directory for local service state.
// During dev we use ~/.choir so state persists across restarts.
// For distribution: ~/Library/Application Support/Choir (macOS standard).
var dataDir string

func init() {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.TempDir()
	}
	dataDir = filepath.Join(home, ".choir")
}

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

// startLocalServices launches gateway, sandbox, and proxy as child
// processes with environment configured for localhost operation.
func startLocalServices(binDir string) ([]*serviceProcess, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	// Common environment for all services.
	baseEnv := []string{
		"SERVER_HOST=127.0.0.1",
	}

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
	)

	services := []struct {
		name string
		bin  string
		env  []string
	}{
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
// proxies /api/* to the local proxy service. In local mode there is no auth
// service — device access is ownership. The /desktop-auth/start-session
// endpoint opens the cloud auth bridge via ASWebAuthenticationSession.
func startLocalFrontendServer(backend string) (*http.Server, error) {
	mux := http.NewServeMux()

	// API proxy — routes /api/* to the local proxy service.
	apiProxy := httputil.NewSingleHostReverseProxy(&url.URL{
		Scheme: "http",
		Host:   "127.0.0.1:" + localProxyPort,
	})
	mux.HandleFunc("/api/", apiProxy.ServeHTTP)

	// Desktop auth session — opens the cloud auth bridge via
	// ASWebAuthenticationSession, redeems the exchange code for tokens.
	mux.HandleFunc("/desktop-auth/start-session", func(w http.ResponseWriter, r *http.Request) {
		handleDesktopAuthSession(w, r, backend)
	})

	// Frontend assets with bridge script injection.
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

		// Check if the file exists in the embedded FS.
		if _, err := fs.Stat(frontendFS, strings.TrimPrefix(path, "/")); err != nil {
			// SPA fallback to index.html.
			r.URL.Path = "/"
			path = "/index.html"
		}

		// Inject bridge script into index.html.
		if path == "/index.html" {
			data, err := fs.ReadFile(frontendFS, "index.html")
			if err != nil {
				http.NotFound(w, r)
				return
			}
			injected := injectBridgeScript(data)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(injected)
			return
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

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// injectBridgeScript injects a <script> tag into the HTML that sets a flag
// so auth.js knows to route WebAuthn ceremonies through the desktop bridge
// (ASWebAuthenticationSession) instead of calling navigator.credentials
// (which doesn't support platform authenticators in WKWebView).
func injectBridgeScript(html []byte) []byte {
	script := `<script>` + bridgeScript + `</script>`
	// Insert before </head> if present, otherwise before </body>.
	idx := bytes.Index(html, []byte("</head>"))
	if idx == -1 {
		idx = bytes.Index(html, []byte("</body>"))
	}
	if idx == -1 {
		return html
	}
	result := make([]byte, 0, len(html)+len(script))
	result = append(result, html[:idx]...)
	result = append(result, []byte(script)...)
	result = append(result, html[idx:]...)
	return result
}

// bridgeScript is the JavaScript injected into index.html when served by the
// local frontend server. It sets a flag that auth.js checks to route
// WebAuthn ceremonies through the ASWebAuthenticationSession bridge.
const bridgeScript = `
(function() {
  window.__CHOIR_BRIDGE = true;
  console.log('[choir] Desktop bridge flag set');
})();
`

// ─── Main ───────────────────────────────────────────────────────────────

func main() {
	localMode := os.Getenv("CHOIR_MODE") == "local"
	backend := os.Getenv("CHOIR_BACKEND")
	if backend == "" {
		backend = "https://choir.news"
	}

	var procs []*serviceProcess
	var frontendSrv *http.Server
	var windowURL string

	if localMode {
		log.Printf("Choir starting in LOCAL mode — version: %s", appVersion)

		binDir := os.Getenv("CHOIR_BIN_DIR")
		if binDir == "" {
			candidates := []string{
				filepath.Join(filepath.Dir(os.Args[0]), "..", "..", "..", "bin"),
				filepath.Join(filepath.Dir(os.Args[0]), "..", "..", "..", "..", "..", "bin"),
				filepath.Join(dataDir, "bin"),
			}
			for _, c := range candidates {
				if abs, err := filepath.Abs(c); err == nil {
					if _, err := os.Stat(filepath.Join(abs, "gateway")); err == nil {
						binDir = abs
						break
					}
				}
			}
		}
		if binDir == "" {
			log.Fatalf("Failed to find service binaries. Set CHOIR_BIN_DIR or build with: task build:services")
		}
		log.Printf("Using service binaries from: %s", binDir)

		var err error
		procs, err = startLocalServices(binDir)
		if err != nil {
			log.Fatalf("Failed to start local services: %v\nBuild them with: task build:services", err)
		}
		defer stopLocalServices(procs)

		frontendSrv, err = startLocalFrontendServer(backend)
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
		log.Printf("Choir starting in CLOUD mode — backend: %s, version: %s", backend, appVersion)
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
		Mac: application.MacWindow{
			TitleBar: application.MacTitleBar{
				AppearsTransparent: true,
				HideTitle:          true,
				FullSizeContent:    true,
			},
			InvisibleTitleBarHeight: 28,
		},
	})

	_ = window

	if err := app.Run(); err != nil {
		log.Fatal(fmt.Errorf("Choir exited with error: %w", err))
	}
}

// assetHandler serves embedded frontend assets and proxies /auth/* and
// /api/* to the staging backend. Used only in cloud mode — in local mode,
// the window loads from the local frontend server which handles all routing.
// It also handles /desktop-auth/start-session for the ASWebAuthenticationSession
// flow and injects the bridge script into index.html.
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
		// Desktop auth session — same handler as local mode.
		if r.URL.Path == "/desktop-auth/start-session" {
			handleDesktopAuthSession(w, r, backend)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/auth/") || strings.HasPrefix(r.URL.Path, "/api/") {
			proxy.ServeHTTP(w, r)
			return
		}
		serveEmbedded(w, r, embedded)
	})
}

// serveEmbedded serves embedded frontend assets with SPA fallback.
// Injects the bridge script into index.html so the frontend knows it's
// running in desktop bridge mode.
func serveEmbedded(w http.ResponseWriter, r *http.Request, embedded http.Handler) {
	path := r.URL.Path
	if path == "/" {
		path = "/index.html"
	}
	stripped := strings.TrimPrefix(path, "/")
	if _, err := assets.ReadFile("frontend/dist/" + stripped); err != nil {
		r.URL.Path = "/"
		path = "/index.html"
	}
	if path == "/index.html" {
		data, err := assets.ReadFile("frontend/dist/index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		injected := injectBridgeScript(data)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(injected)
		return
	}
	embedded.ServeHTTP(w, r)
}

// handleDesktopAuthSession opens choir.news via ASWebAuthenticationSession.
// It first tries /auth/desktop/exchange-redirect — if the user is already
// signed in on Safari, the server immediately 302-redirects to choir://auth-complete?code=...,
// which ASWebAuthenticationSession reliably intercepts (unlike JS window.location.href).
// If that fails (user not signed in), it falls back to the bridge page for WebAuthn.
func handleDesktopAuthSession(w http.ResponseWriter, r *http.Request, backend string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Email    string `json:"email"`
		AuthType string `json:"authType"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	// Phase 1: Try exchange-redirect directly (works if user is already signed in on Safari).
	redirectURL := fmt.Sprintf("%s/auth/desktop/exchange-redirect", backend)
	log.Printf("Starting ASWebAuthenticationSession (exchange-redirect): %s", redirectURL)

	callbackURL, err := startWebAuthSession(redirectURL, "choir")
	if err == nil {
		log.Printf("ASWebAuthenticationSession completed via exchange-redirect: %s", callbackURL)
	} else {
		// Phase 2: Fall back to bridge page for WebAuthn ceremony.
		log.Printf("Exchange-redirect failed (%v), falling back to bridge page", err)
		bridgeURL := fmt.Sprintf("%s/desktop-bridge.html?email=%s",
			backend, url.QueryEscape(req.Email))
		log.Printf("Starting ASWebAuthenticationSession (bridge): %s", bridgeURL)

		callbackURL, err = startWebAuthSession(bridgeURL, "choir")
		if err != nil {
			log.Printf("ASWebAuthenticationSession error: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		log.Printf("ASWebAuthenticationSession completed via bridge: %s", callbackURL)
	}

	// Parse the callback URL to extract the exchange code.
	cbURL, err := url.Parse(callbackURL)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "invalid callback URL"})
		return
	}
	code := cbURL.Query().Get("code")
	if code == "" {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "no code in callback"})
		return
	}

	// Redeem the code for tokens.
	redeemBody, _ := json.Marshal(map[string]string{"code": code})
	redeemRes, err := http.Post(
		backend+"/auth/desktop/redeem",
		"application/json",
		bytes.NewReader(redeemBody),
	)
	if err != nil {
		log.Printf("Token redeem error: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to redeem token"})
		return
	}
	defer redeemRes.Body.Close()

	if redeemRes.StatusCode != http.StatusOK {
		var errResp map[string]string
		_ = json.NewDecoder(redeemRes.Body).Decode(&errResp)
		log.Printf("Token redeem failed: %s", errResp["error"])
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": errResp["error"]})
		return
	}

	var tokens struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(redeemRes.Body).Decode(&tokens); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to parse tokens"})
		return
	}

	log.Printf("Token redeem succeeded, returning tokens to frontend")
	writeJSON(w, http.StatusOK, tokens)
}

func init() {
	http.DefaultTransport = &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}
}
