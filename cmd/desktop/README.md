# Choir — Wails v3 macOS App

A native macOS desktop wrapper for the Choir web desktop, built with
[Wails v3](https://v3.wails.io/).

## What This Is

Choir wraps the existing Svelte frontend in a native macOS window
using Wails v3. By default it connects to the staging backend
(`https://choir.news`) for all API calls — no local backend services.
Local mode (`CHOIR_MODE=local`) runs the full Choir stack as child
processes for development.

## Prerequisites

- Go 1.25+
- Node.js and npm (for building the Svelte frontend)
- macOS 12.0+ (Monterey or later)
- [Task](https://taskfile.dev) (for build orchestration)

## Setup

### Option A: Nix dev shell (recommended)

```bash
# From the repo root — uses the desktop dev shell, not the default one
nix develop .#desktop -c bash

# Then from cmd/desktop:
cd cmd/desktop
task deps
task dev
```

This gives you Go, Node, and Task without pulling in ICU/Dolt or
interfering with the main `nix develop` shell.

### Option B: Manual setup

```bash
# From the desktop module directory
cd cmd/desktop

# Download Go dependencies
task deps

# Build the frontend and run in dev mode
task dev
```

## Building

```bash
# Build the binary
task build

# Package as .app bundle
task package

# Ad-hoc sign for local testing
task sign
```

## Configuration

The desktop app supports two modes:

### Cloud mode (default)

Connects to the staging backend at `choir.news`. No local services.
Double-clicking `Choir.app` launches in cloud mode.

```bash
# Run in cloud mode (default)
task cloud

# Or override the backend
CHOIR_BACKEND=https://choir.news task cloud
```

### Local mode

Runs the full Choir backend stack locally as child processes. The Wails
window loads `http://localhost:3000`, giving a real localhost origin so
WebAuthn passkeys work in WKWebView.

```bash
# Build everything and run in local mode
CHOIR_MODE=local task dev
```

Service binaries are built to `../../bin/` and launched with environment
variables configured for localhost. State is stored in `~/.choir/`.

### Environment variables

| Variable | Default | Description |
|---|---|---|
| `CHOIR_MODE` | (unset = cloud) | Set to `local` for local mode |
| `CHOIR_BACKEND` | `https://choir.news` | Backend URL |
| `CHOIR_BIN_DIR` | (auto-detected) | Path to service binaries for local mode |

## Architecture

```
Wails v3 app (Go)
  -> embeds frontend/dist (Svelte build output)
  -> window loads embedded assets in production
  -> /auth/* and /api/* requests are reverse-proxied to the backend
  -> DesktopService exposes app metadata via typed Go bridge
  -> transparent macOS title bar (FullSizeContent + AppearsTransparent)
```

The Svelte frontend works unchanged because it uses relative URLs
(`/auth/*`, `/api/*`). The Wails asset handler intercepts these and
forwards them to the backend.

## Authentication Bridge

WKWebView does not support WebAuthn platform authenticators (Touch ID).
The desktop app uses `ASWebAuthenticationSession` to delegate auth to
Safari, which shares cookies with the system browser.

### Flow (already-signed-in user)

1. Frontend calls `POST /desktop-auth/start-session`
2. Desktop app opens `choir.news/auth/desktop/exchange-redirect` in
   `ASWebAuthenticationSession` (listening for `choir://` scheme)
3. Server sees existing Safari cookies, creates exchange code, responds
   with 302 redirect to `choir://auth-complete?code=...`
4. `ASWebAuthenticationSession` intercepts the redirect, returns callback URL
5. Desktop app redeems the code for tokens via `POST /auth/desktop/redeem`
6. Tokens are injected as cookies into the WKWebView

### Flow (not signed in)

1. Phase 1 (exchange-redirect) fails — user has no Safari session
2. Desktop app falls back to opening `choir.news/desktop-bridge.html`
3. Bridge page performs WebAuthn ceremony (Touch ID via Safari)
4. Bridge page navigates to `/auth/desktop/exchange-redirect`
5. Server 302-redirects to `choir://auth-complete?code=...`
6. Same redemption flow as above

### Key endpoints

| Endpoint | Method | Description |
|---|---|---|
| `/auth/desktop/exchange` | POST | JSON exchange — returns code in response body |
| `/auth/desktop/exchange-redirect` | GET | 302 redirect — returns code via `choir://` scheme |
| `/auth/desktop/redeem` | POST | Redeem exchange code for access/refresh tokens |
| `/auth/session` | GET | Check if user is authenticated (used by bridge page) |

### Why server-side 302 instead of JS redirect

Safari's `ASWebAuthenticationSession` web view does not reliably
intercept `window.location.href` navigations to custom URL schemes.
A server-side 302 redirect is followed natively by the web view and
reliably triggers the `ASWebAuthenticationSession` callback.

## App Icon

The app icon is generated from `build/darwin/appicon.png` (a 1024×1024
PNG of the tetramark). The `task icon` command uses `sips` to generate
all required sizes and `iconutil` to produce `icons.icns`.

macOS aggressively caches app icons by path. If the icon doesn't update
after a rebuild, move the `.app` to a different path or clear the icon
cache:

```bash
rm -rf ~/Library/Caches/com.apple.iconservices.store
killall Dock
```

## Pinned Wails Version

```
github.com/wailsapp/wails/v3 v3.0.0-alpha2.104
```

Pinned 2026-06-22. See the spec for version upgrade policy:
[spec-choir-desktop-wails-v3-2026-06-22.md](../../docs/archive/spec-choir-desktop-wails-v3-2026-06-22.md)
