package main

import (
	"context"
	"embed"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

// stagingURL is the backend this desktop app connects to.
// Override with CHOIR_BACKEND env var (e.g. for local dev).
func stagingURL() string {
	if u := os.Getenv("CHOIR_BACKEND"); u != "" {
		return u
	}
	return "https://choir.news"
}

// appInfo holds build-time metadata injected via ldflags.
var (
	appVersion = "dev"
	appCommit  = "unknown"
	appBuiltAt = "unknown"
)

// DesktopService exposes app metadata to the frontend via Wails bindings.
type DesktopService struct{}

func (d *DesktopService) GetAppInfo() map[string]string {
	return map[string]string{
		"version":  appVersion,
		"commit":   appCommit,
		"builtAt":  appBuiltAt,
		"backend":  stagingURL(),
		"platform": "wails-v3-desktop",
	}
}

func (d *DesktopService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	return nil
}

func (d *DesktopService) ServiceShutdown() error {
	return nil
}

// assetHandler serves embedded frontend assets and proxies /auth/* and /api/*
// to the staging backend. This lets the Svelte frontend use relative URLs
// unchanged — the Wails asset server intercepts and forwards them.
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

	// WebSocket support: upgrade the proxy for WS connections.
	proxy.ModifyResponse = func(resp *http.Response) error {
		// Strip any compression that might interfere with WS upgrade.
		return nil
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Proxy auth and API calls to the backend (includes WebSocket
		// upgrade requests under /api/*).
		if strings.HasPrefix(r.URL.Path, "/auth/") || strings.HasPrefix(r.URL.Path, "/api/") {
			// Check if this is a WebSocket upgrade request.
			if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
				// httputil.ReverseProxy supports WebSocket in Go 1.12+.
				proxy.ServeHTTP(w, r)
				return
			}
			proxy.ServeHTTP(w, r)
			return
		}

		// Serve embedded frontend assets for everything else.
		// Fallback to index.html for client-side routing (SPA).
		path := r.URL.Path
		if path == "/" {
			path = "/index.html"
		}
		// Check if the path exists in the embedded FS; if not, serve
		// index.html for SPA routing.
		stripped := strings.TrimPrefix(path, "/")
		if _, err := assets.ReadFile("frontend/dist/" + stripped); err != nil {
			r.URL.Path = "/"
		}
		embedded.ServeHTTP(w, r)
	})
}

func main() {
	backend := stagingURL()
	log.Printf("Choir Desktop starting — backend: %s, version: %s", backend, appVersion)

	app := application.New(application.Options{
		Name:        "Choir",
		Description: "Choir — your automatic computer, native on macOS",
		Services: []application.Service{
			application.NewService(&DesktopService{}),
		},
		Assets: application.AssetOptions{
			Handler: assetHandler(backend),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	// Create the main window.
	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "Choir",
		Name:      "main",
		Width:     1280,
		Height:    800,
		MinWidth:  900,
		MinHeight: 600,
		URL:       "/",
	})

	_ = window

	if err := app.Run(); err != nil {
		log.Fatal(fmt.Errorf("Choir Desktop exited with error: %w", err))
	}
}

// init sets a default timeout for the reverse proxy transport so that
// backend requests don't hang indefinitely.
func init() {
	http.DefaultTransport = &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}
}
