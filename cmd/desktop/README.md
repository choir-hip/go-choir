# Choir for macOS (Wails v3)

A native macOS window onto a Choir server, built with
[Wails v3](https://v3.wails.io/). By default it talks to `https://choir.news`.

## What it is

The app compiles none of the UI in. Every request the window makes goes to
the Wails asset handler, which proxies it to the server through a native
session. So the window shows exactly what a browser shows: the platform shell
when signed out, and the computer's own surface (Desktop, Texture, apps) once
signed in. A computer that restores to an earlier head, or changes its own UI,
is shown as it is, never as a frozen copy shipped with the app
([memo-per-computer-frontend-2026-08-13.md](../../docs/memo-per-computer-frontend-2026-08-13.md)).

What the app adds natively:

- **Passkey sign-in.** WKWebView has no platform authenticator, so sign-in runs
  in `ASWebAuthenticationSession` (Safari, Touch ID). See "Authentication
  bridge" below.
- **A credential boundary.** Session cookies live in a process-local Go jar.
  The renderer can use them through the proxy but cannot read or set them.

It runs nothing locally. The server-less desktop (computers running as VMs on
this Mac), the synced folder, and Windows/Linux builds are designed in
[desktop-sync-and-local-vms-2026-10-14.md](../../docs/reports/desktop-sync-and-local-vms-2026-10-14.md).
The old `CHOIR_MODE=local` ran services as host processes; it could not work
after the persistent-computer cutover and was removed.

## Build and run

Requirements: macOS 12+ on Apple Silicon or Intel, Go 1.25+, and Xcode or the
Command Line Tools (`xcode-select --install`). No Node, no frontend build, and
no Apple Developer account.

```bash
cmd/desktop/build.sh          # builds and ad-hoc signs cmd/desktop/bin/Choir.app
cmd/desktop/build.sh run      # builds, then runs it with logs in the terminal
cmd/desktop/build.sh test     # go vet + go test
open cmd/desktop/bin/Choir.app
```

`build.sh` deliberately runs outside the repo's Nix dev environment. That
environment exports ICU link flags (for Dolt), a Nix clang and a Nix SDK, which
would link `/nix/store` libraries into the app. The script uses Xcode's clang
and SDK, and refuses to package a binary that references `/nix/store`.

The signature is ad hoc: the app runs on the Mac that built it. Distributing it
needs a Developer ID certificate, the hardened runtime, and notarization.

## Configuration

| Variable | Default | Effect |
|---|---|---|
| `CHOIR_BACKEND` | `https://choir.news` | Server to connect to. HTTPS required, except `http://localhost`/loopback for development |
| `CHOIR_DESKTOP_LOG_REQUESTS` | unset | Log each request the window makes, with its status |
| `CHOIR_DESKTOP_SNAPSHOT` | unset | Write a PNG of the rendered page to this path once, as a launch artifact (WKWebView snapshot; needs no Screen Recording permission) |
| `CHOIR_DESKTOP_SNAPSHOT_DELAY` | `10s` | When to take that snapshot |

Environment variables reach an app opened from Finder only through
`open --env NAME=value cmd/desktop/bin/Choir.app`.

## Architecture

```
Choir.app (Go, Wails v3)
  window (WKWebView, origin wails://localhost)
    -> Wails asset handler (surface.go)
         /desktop-auth/start-session   native passkey bridge (desktop_auth.go)
         /__choir_desktop/bridge.js    sets window.__CHOIR_BRIDGE = true
         /auth/desktop/{exchange*,redeem}  blocked for the renderer
         everything else               reverse proxy -> CHOIR_BACKEND
                                       (native cookie jar; HTML documents get
                                        the bridge <script> injected)
```

The bridge script is what tells `frontend/src/lib/auth.js` to route passkey
ceremonies through the native app. If the server is unreachable, page loads
get a small offline page and API calls get JSON 502.

### Known gaps

- **WebSockets.** The frontend opens `ws://${location.host}/api/ws` (live
  events). From `wails://localhost` that targets the user's own machine, and
  the Wails asset server answers WebSocket upgrades with 501. Live updates do
  not work in the app yet; see the design report for the fix. (The terminal's
  `/api/super-console/ws` was removed with zot on 2026-10-22.)
- **Session lifetime.** The cookie jar lasts only for the process: relaunching
  requires signing in again.

## Authentication bridge

WKWebView does not support WebAuthn platform authenticators (Touch ID).
The desktop app uses `ASWebAuthenticationSession` to delegate auth to
Safari, which shares cookies with the system browser.

1. The frontend calls `POST /desktop-auth/start-session` with the email.
2. The desktop app opens `desktop-bridge.html` exactly once in
   `ASWebAuthenticationSession`, listening for the `choir://` scheme.
3. The bridge checks the Safari session. An already-signed-in user continues
   immediately; otherwise Safari performs the WebAuthn ceremony with Touch ID.
4. The server redirects to `choir://auth-complete?code=...` and the native app
   validates the callback authority.
5. Go redeems the one-time, user-bound code through `/auth/desktop/redeem` with
   bounded network and response limits. The auth service mints a distinct
   native session through its canonical cookie issuer; Safari and the desktop
   never share a refresh credential.
6. Go's process-local cookie jar absorbs the auth service's HttpOnly
   `Set-Cookie` response and returns only `{"authenticated":true}` to the
   renderer. Go does not decode tokens or recreate cookie names, paths, or
   attributes.
7. The native reverse proxy deletes renderer-supplied cookies, attaches the jar
   cookies to backend requests, absorbs backend rotation/logout `Set-Cookie`
   headers, and strips those headers before the renderer sees the response.
8. The frontend reloads, and the proxy now serves the signed-in computer's
   surface.

Access tokens, refresh tokens, exchange codes, and cookie values are not
returned to JavaScript or written to logs.

| Endpoint | Method | Description |
|---|---|---|
| `/auth/desktop/exchange-redirect` | GET | Safari bridge receives a one-time code through the `choir://` callback |
| `/auth/desktop/redeem` | POST | Native Go broker redeems the one-time code into a distinct server-minted cookie session; renderer access is blocked |
| `/auth/session` | GET | Check if user is authenticated (used by bridge page) |

Safari's `ASWebAuthenticationSession` web view does not reliably intercept
`window.location.href` navigations to custom URL schemes, so the server
answers with a 302 to `choir://`, which the session does intercept.

## Signing and entitlements

`build/darwin/entitlements.plist` holds only `com.apple.security.*` keys.
Usage strings (`NSMicrophoneUsageDescription`, `NSCameraUsageDescription`)
belong in `Info.plist`; in the entitlements file they get the ad-hoc-signed
bundle SIGKILLed at launch on macOS 27
([problem doc](../../docs/problems/desktop-app-does-not-launch-when-signed-2026-10-14.md)).

## App icon

`build/darwin/icons.icns` is generated from `build/darwin/appicon.png` with
`build.sh icon` (`sips` + `iconutil`). macOS caches icons by path; if the icon
does not update, move the app or run
`rm -rf ~/Library/Caches/com.apple.iconservices.store && killall Dock`.

## Wails version

`github.com/wailsapp/wails/v3 v3.0.0-beta.26` (2026-09-25). v3 is still a
pre-release line with weekly betas; upgrade deliberately, then rebuild and
launch (`build.sh run`) before committing.
