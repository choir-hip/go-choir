// Command desktop is Choir's native macOS client: a Wails v3 window onto a
// Choir server (default https://choir.news).
//
// The window renders whatever the server serves: the platform shell to a
// signed-out caller and the vmctl-resolved computer surface to a signed-in
// one, exactly as a browser sees it. Nothing of the UI is compiled into the
// app, so the desktop cannot drift from the computer's own surface (see
// docs/memo-per-computer-frontend-2026-08-13.md).
//
// The app's own job is the native session: passkey ceremonies run in
// ASWebAuthenticationSession (WKWebView has no platform authenticator), and
// the resulting cookies live in a process-local Go jar that the renderer can
// use through the proxy but never read (desktop_auth.go).
//
// A server-less mode that runs computers on this Mac is designed in
// docs/reports/desktop-sync-and-local-vms-2026-10-14.md; the earlier
// CHOIR_MODE=local, which ran services as host processes, was removed.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Build-time metadata injected via ldflags.
var (
	appVersion = "dev"
	appCommit  = "unknown"
	appBuiltAt = "unknown"
)

const defaultBackend = "https://choir.news"

// DesktopService exposes app metadata through the Wails bridge.
type DesktopService struct {
	backend string
}

func (d *DesktopService) GetAppInfo() map[string]string {
	return map[string]string{
		"version":  appVersion,
		"commit":   appCommit,
		"builtAt":  appBuiltAt,
		"backend":  d.backend,
		"platform": "wails-v3-desktop",
	}
}

func (d *DesktopService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	return nil
}

func (d *DesktopService) ServiceShutdown() error {
	return nil
}

func main() {
	if mode := os.Getenv("CHOIR_MODE"); mode != "" && mode != "cloud" {
		log.Fatalf("CHOIR_MODE=%q is not supported: the desktop is a client of a Choir server (set CHOIR_BACKEND to choose one)", mode)
	}
	backend := os.Getenv("CHOIR_BACKEND")
	if backend == "" {
		backend = defaultBackend
	}
	desktopSession, err := newDesktopSession(backend)
	if err != nil {
		log.Fatalf("Invalid Choir backend configuration: %v", err)
	}
	log.Printf("Choir desktop %s (%s) — backend: %s", appVersion, appCommit, desktopSession.backend)

	app := application.New(application.Options{
		Name:        "Choir",
		Description: "Choir — your automatic computer",
		Services: []application.Service{
			application.NewService(&DesktopService{backend: desktopSession.backend.String()}),
		},
		Assets: application.AssetOptions{
			Handler: assetHandler(desktopSession),
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
		URL:       "/",
		Mac: application.MacWindow{
			TitleBar: application.MacTitleBar{
				AppearsTransparent: true,
				HideTitle:          true,
				FullSizeContent:    true,
			},
			InvisibleTitleBarHeight: 28,
		},
	})

	// Development aid: CHOIR_DESKTOP_SNAPSHOT=/path.png writes the rendered
	// page to a PNG once, CHOIR_DESKTOP_SNAPSHOT_DELAY (default 10s) after
	// launch, as a repeatable launch artifact.
	if path := os.Getenv("CHOIR_DESKTOP_SNAPSHOT"); path != "" {
		delay := 10 * time.Second
		if parsed, err := time.ParseDuration(os.Getenv("CHOIR_DESKTOP_SNAPSHOT_DELAY")); err == nil {
			delay = parsed
		}
		time.AfterFunc(delay, func() { snapshotWindow(window, path) })
	}

	if err := app.Run(); err != nil {
		log.Fatal(fmt.Errorf("Choir exited with error: %w", err))
	}
}
