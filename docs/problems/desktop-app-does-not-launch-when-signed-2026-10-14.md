# The desktop app is killed at launch once signed, and ships a frozen UI (2026-10-14)

Status: findings 1-4 and 6 fixed in 4df9fd321 (branch
`desktop-revive-and-sync-plan`, not pushed); finding 5 (WebSocket) open, see
[reports/desktop-sync-and-local-vms-2026-10-14.md](../reports/desktop-sync-and-local-vms-2026-10-14.md) §1.4.
Documented before any fix. Evidence from building and launching
`cmd/desktop` on this Mac (darwin/arm64, macOS 27.2, go1.26.4, Wails
`v3.0.0-alpha2.104`) at `main` 6415505e3. Nothing here touches Node B or
staging; `cmd/desktop` is not deployed. Follows the survey in
[reports/self-hosting-and-desktop-2026-10-14.md](../reports/desktop-sync-and-local-vms-2026-10-14.md) §2.2.
Mutation class of the fix: orange for the desktop client (no server change).

## Findings

**1. The signed app bundle is SIGKILLed at launch (observed).**
- The `sign` task runs `codesign --entitlements build/darwin/entitlements.plist --sign -`.
- The bundle signed that way exits with code 137 (SIGKILL) before printing
  anything. The same bundle signed with no entitlements runs.
- Bisected by re-signing with subsets (scratch script, not in the repo):
  every `com.apple.security.*` key alone runs, including the invalid
  `app-autoputer`; the two string keys `NSMicrophoneUsageDescription` and
  `NSCameraUsageDescription` alone reproduce the kill; the original file
  minus those two strings runs.
- Those are Info.plist usage strings, not entitlements. They are already in
  `Info.plist`.
- The file also carries two keys that are not Apple entitlements:
  `com.apple.security.app-autoputer` (the 2026-08-11 global rename of
  `app-sandbox`) and `com.apple.security.cs.allow-javascript`.

**2. The desktop serves its own frozen copy of the UI (observed).**
- The run log shows `[AssetFileServerFS] Handling request url=/assets/index-*.js`:
  the window renders the `frontend/dist` compiled into the binary, and only
  `/auth/*` and `/api/*` reach `choir.news`.
- On the server, everything else goes to the proxy, which serves the
  platform shell to signed-out callers and the vmctl-resolved computer
  surface to signed-in ones (`internal/proxy/computer_surface.go`,
  `nix/node-b.nix` Caddy block).
- So the desktop is the "third copy" named as a break case in
  [memo-per-computer-frontend-2026-08-13.md](../memo-per-computer-frontend-2026-08-13.md):
  outside restore, and stale the moment staging deploys a new frontend.
- It also makes a fresh checkout fail to build: `//go:embed all:frontend/dist`
  matches nothing because the dist copy is gitignored.

**3. The binary links Nix store dylibs (observed).**
- `otool -L` on a build from the repo dev shell lists
  `/nix/store/...-icu4c-76.1/lib/libicui18n.76.1.dylib`, `libicuuc`, and
  `/nix/store/...-libresolv-93/lib/libresolv.9.dylib`.
- Cause: the direnv/zshenv exports `CGO_LDFLAGS=-licui18n -licuuc` (for
  Dolt) and the Nix clang wrapper; the desktop module does not use ICU.
- The app would not start on a Mac without those store paths. The linker
  also warns that objects built for macOS 14 are linked for macOS 11.

**4. Local mode and the Taskfile are stale (code reading, confirms the survey).**
- `CHOIR_MODE=local` starts gateway, autoputer and proxy as host processes
  with no auth, corpusd, maild, vmctl or Dolt. The proxy exits without an
  auth public key; the computer runtime as a host process is forbidden.
- `task` is not installed on this Mac and the `dev` target builds the
  local-mode service binaries.

**5. The live WebSocket channel cannot connect inside the Wails window (code reading; to confirm at runtime).**
- The frontend builds `ws://${location.host}/api/ws` (`Desktop.svelte`)
  and `/api/super-console/ws`.
- In the Wails window `location` is `wails://localhost/`, so the socket
  goes to `ws://localhost/api/ws` on the user's own machine, not to the
  backend. Wails' asset server also answers WebSocket upgrades with 501
  (`internal/assetserver/assetserver.go`).
- *Hypothesis:* live updates degrade to whatever polling the frontend has;
  the terminal app cannot work in the desktop.

**6. The Wails pin is four months old.** `v3.0.0-alpha2.104` (2026-06-18)
still builds with go1.26.4. The v3 line has since moved to beta
(`v3.0.0-beta.29`, 2026-10-13).

## Belief state

- The desktop has not been launchable as a signed bundle since the usage
  strings were put in the entitlements file. This is a claim about this
  macOS (27.2); older releases may have been more lenient.
- The passkey bridge code (`desktop_auth.go`, `webauthn_session_darwin.go`)
  is not implicated by any of the above.

## Remaining error field

- Whether ASWebAuthenticationSession completes the passkey ceremony from
  an ad-hoc-signed bundle needs a human with Touch ID; an agent cannot
  complete it.
- Whether the frontend falls back cleanly without the WebSocket.
