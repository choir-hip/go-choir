# The Desktop App: Revival, a Synced Folder, Local VMs, and Other Platforms

Date: 2026-10-14. Base: `origin/main` 6415505e3. Branch
`desktop-revive-and-sync-plan`.

Mutation class of this note: **green** (docs). Part A (the desktop build)
landed as orange on the same branch. Everything proposed in Parts B to E is
orange or red and is classified per slice. Nothing was pushed or deployed.

Owner, 2026-10-14 (about 21:00 UTC): "updating and bringing up the Wails Mac
desktop app, as well as R&D for the full server-less version where we're
running VMs on the Mac. Also thinking about Windows and Linux versions. And
the thing that I'm most interested in is the file provider service, as well
as syncing with a server instance, so that you could share a folder with your
Choir server. You could add files directly to that folder. You could copy
files from the folder, like a Google Drive or Dropbox. And we're going to
want to incorporate this into the mobile app."

Inputs:
- [self-hosting-and-desktop-2026-10-14.md](desktop-sync-and-local-vms-2026-10-14.md)
  (the desktop survey, §2 and §3)
- [vm-backend-interface-2026-10-14.md](vm-hypervisor-seam-design-2026-10-21.md)
  (the `vmbackend` seam; on commit b6f27ded4, not yet on `main`)
- [isolation-runtimes-2026-10-14.md](../research/isolation-runtimes-2026-10-14.md)
  (§5, §6: libkrun and Virtualization.framework for the desktop)
- [choir-state-and-vision-2026-10-14.md](../archive/reports/choir-state-and-vision-2026-10-14.md)
  (mobile and app-store constraints)

External facts come from primary sources where we could find them (Apple,
Microsoft, Wails, project READMEs). They are linked in §8. Anything not
confirmed is marked *(unconfirmed)*, and our own reasoning is marked
*(inference)*.

## Summary

1. **The Mac app builds and launches again** (Part A, §1). The signed bundle
   had been killed at launch. The cause was usage strings in the entitlements
   file, found by bisecting. The window now shows whatever the server serves
   instead of a frozen copy of the UI compiled into the app. Local mode is
   gone. `cmd/desktop/build.sh` builds the app without Nix dylibs leaking in.
   Wails is now `v3.0.0-beta.26`. The live Autopaper front from choir.news
   renders in the window. Two gaps remain: the WebSocket live channel, and
   sessions that do not survive a relaunch.
2. **The server half of sync already exists in part** (§2.2):
   - `/api/files` handles list, get (with Range), put, mkdir and delete.
   - `file.changed` events stream over `/api/ws` with `after_seq` catch-up.
   - filecas is a chunked, encrypted, content-addressed backup of the files
     root.
3. **What sync is missing** (§2.3):
   - a change cursor over the whole tree, with content hashes;
   - conditional writes (ETag / If-Match);
   - rename;
   - a size limit and resumable uploads;
   - change events for files that agents write inside the guest;
   - a credential for a long-running sync client.
4. **A File Provider extension needs a paid Apple Developer account in
   practice** (§2.4). Free accounts cannot distribute apps. Ad-hoc signing
   has no Team ID, so App Group containers fail validation on macOS 15+. We
   found no report of a File Provider extension loading from a free
   Personal Team build *(unconfirmed either way)*. The same holds on iOS.
5. **Recommended first slice: a plain two-way synced folder** (§2.6), with no
   placeholders. It is a Go sync engine (`internal/filesync`) that talks to
   the existing `/api/files` plus three small server additions. The CLI is
   its first client (`choir files sync ~/Choir`), so it can be proved E2E on
   staging. The Wails app is the second client. It works on macOS, Linux and
   Windows without any developer account.
   File Provider (macOS, then iOS) and Cloud Files (Windows) come later, as
   placeholder front-ends over the same engine and API.
6. **Server-less** (§3):
   - Recommendation: a native backend through **vfkit**
     (Virtualization.framework). It plugs into the `vmbackend` seam as a
     third backend with no snapshots, so hibernate is a stop.
   - The Linux appliance with nested Firecracker is kept only as a fallback
     prototype, because it runs only on M3 or later with macOS 15.
   - The `com.apple.security.virtualization` entitlement works with ad-hoc
     signing, so this R&D needs no Apple account.
   - It needs an aarch64 guest image first.
7. **Windows and Linux** (§4):
   - Wails v3 supports both (WebView2; GTK4 + WebKitGTK 6.0).
   - Neither embedded engine does passkeys reliably. Sign-in should go
     through the system browser with a loopback or custom-scheme redirect,
     which is the macOS bridge's pattern.
   - Windows signing costs about $10 a month (Azure Artifact Signing).
     SmartScreen reputation still builds slowly.
8. **Mobile** (§5) reuses the sync API (a Swift client plus an iOS File
   Provider extension) and the desktop's approach to its window: a native
   shell around the server's own surface, with native auth.

---

## 1. Part A: the Mac app, revived

### 1.1 What was wrong

The problems were documented first, before any fix, in
[desktop-app-does-not-launch-when-signed-2026-10-14.md](../problems/desktop-app-does-not-launch-when-signed-2026-10-14.md)
(commit 8e8fc9f95). In short:

| # | Problem | Evidence |
|---|---|---|
| 1 | The signed `.app` was SIGKILLed at launch (exit 137) | Bisected by re-signing. `NSMicrophoneUsageDescription` and `NSCameraUsageDescription` in the **entitlements** file reproduce the kill on their own. Every `com.apple.security.*` key, including the invalid `app-autoputer`, runs |
| 2 | The window rendered a frozen embedded UI | The run log showed `[AssetFileServerFS]` serving `/assets/index-*.js` locally. Only `/auth` and `/api` reached the server. A fresh checkout did not build (`//go:embed` of a gitignored dir) |
| 3 | The binary linked `/nix/store` ICU and libresolv dylibs | `otool -L`. The repo env exports `CGO_LDFLAGS=-licui18n -licuuc` for Dolt |
| 4 | Local mode and the Taskfile were stale | Code reading. `task` is not installed |
| 5 | The WebSocket live channel cannot connect | Code reading: the frontend builds `ws://${location.host}/api/ws`, which is `ws://localhost` in the app, and the Wails asset server returns 501 on upgrades |
| 6 | The Wails pin was four months old | `v3.0.0-alpha2.104` (2026-06-18); the v3 line is now in beta |

### 1.2 What changed (commit 4df9fd321)

- **Entitlements.** Only `com.apple.security.device.audio-input` and
  `com.apple.security.device.camera` remain. These matter once the app is
  signed with the hardened runtime. The usage strings were already in
  `Info.plist`.
- **No embedded UI.** `surface.go` handles every request the Wails window
  makes:
  - `/desktop-auth/start-session` is the native passkey bridge (unchanged).
  - `/__choir_desktop/bridge.js` is served natively and sets
    `window.__CHOIR_BRIDGE`.
  - The renderer is blocked from the desktop exchange routes (unchanged).
  - **Everything else is reverse-proxied to `CHOIR_BACKEND`** through the
    native cookie jar.

  HTML documents get one `<script src>` injected before `</head>`. The Go
  transport decodes compression first, and Content-Length is fixed up. So the
  desktop shows the platform shell when signed out, and the vmctl-resolved
  computer surface when signed in, exactly like a browser. This removes break
  case 4 of [memo-per-computer-frontend-2026-08-13.md](../memo-per-computer-frontend-2026-08-13.md).
  An unreachable server gives page loads an offline page and API calls JSON
  502. Tests were written before the code (`surface_proxy_test.go`; the
  failure modes are listed at its top). The existing proxy and auth tests
  still pass.
- **Removed:**
  - `CHOIR_MODE=local` (setting it now fails with a message);
  - the Taskfile;
  - `Info.dev.plist`;
  - the module's dependency on the root `go-choir` module.
- **`cmd/desktop/build.sh`** (`build|run|test|icon|clean`):
  - builds with Xcode's clang and SDK, outside the Nix environment;
  - passes `-mmacosx-version-min=12.0` to both cgo and the external linker;
  - **refuses to package a binary that references `/nix/store`**;
  - assembles `bin/Choir.app` and ad-hoc signs it with the entitlements;
  - runs `codesign --verify --strict`.
- **Wails:** `v3.0.0-alpha2.104` → `v3.0.0-beta.26` (2026-09-25; picked
  because it is more than two weeks old). No source changes were needed.
  `beta.29` (2026-10-13) exists.
- **Development aids:**
  - `CHOIR_DESKTOP_LOG_REQUESTS=1` logs each request with its status.
  - `CHOIR_DESKTOP_SNAPSHOT=/path.png` writes a WKWebView snapshot of the
    page as a launch artifact. It orders the window to the front first,
    because an occluded page is `hidden` and WebKit pauses its CSS
    animations.
- **Docs:** the README is rewritten, the `flake.nix` desktop shell is now Go
  only, and `current-architecture.md` and `platform-os-app-state.md` are
  aligned.

### 1.3 Verification (darwin/arm64, macOS 27.2, go1.26.4)

- `cmd/desktop/build.sh test`: `go vet` passes; `go test` passes (10 existing proxy
  and auth tests plus 4 new surface tests).
- `cmd/desktop/build.sh` produces `bin/Choir.app`. `otool -L` shows only
  `/usr/lib` and `/System` libraries, with no linker warnings.
  `codesign --verify --strict` passes.
- **Launch** (`open -n --env CHOIR_DESKTOP_LOG_REQUESTS=1 --env CHOIR_DESKTOP_SNAPSHOT=...`):
  - `surface: / from backend (641 bytes, bridge injected)`;
  - then `bridge.js`, CSS/JS assets, `/auth/session`, the AutopaperApp
    chunks and `/api/platform/publications/front?limit=60`, all returning
    200.
- **Screenshot:**
  `/private/tmp/claude-501/-Users-wiz-go-choir/3631f12d-c64d-40d6-a462-dc5bc4bd8b0b/scratchpad/shots/desktop-final-clean-build.png`.
  It shows the signed-out platform shell with the live Autopaper front open
  (today's stories).
  - `screencapture` itself could not be used: this shell has no Screen
    Recording permission (`CGPreflightScreenCaptureAccess() == false`, and
    both `-l <window>` and `-R` capture fail). The image is the app's own
    WKWebView snapshot instead.
  - An earlier snapshot showed the Autopaper window invisible. A probe in
    the renderer traced this to `document.visibilityState == "hidden"` (the
    window was occluded) with the open animation paused at opacity 0. It
    was not an app defect. Once the window was ordered to the front, both
    animations reported `finished`.
- **Not verified:** the passkey ceremony itself. It needs a human with Touch
  ID. The bridge code is unchanged, the flag script is served and injected
  (logged), and the post-sign-in reload makes the proxy serve the computer
  surface.

**Deploy impact.**
- `cmd/desktop` is a nested Go module. No Nix package builds it, so
  deployed closures are unchanged.
- On a push, CI's path classifier would still mark `cmd/desktop/*` for Go
  verification and SBOM fingerprinting (`cmd/*` glob, checked with
  `.github/scripts/ci-impact-classify`).
- The `flake.nix` change touches only `devShells.desktop`.

**Rollback:** `git revert 4df9fd321`.

### 1.4 What remains for the Mac app

1. **WebSocket live channel** (orange: frontend plus desktop). Options:
   - (a) The desktop runs a loopback WebSocket relay on a random port, behind
     a per-launch capability token. The frontend takes its WebSocket origin
     from `window.__CHOIR_WS_ORIGIN` when the bridge sets it. The relay
     attaches jar cookies. Without the unguessable token, other local
     processes cannot use the session.
   - (b) Test whether SSE streams through the Wails asset handler. The
     darwin response writer delivers each write *(inference from code)*. If
     it does, give the live channel an SSE fallback.

   **(a)** is the general fix, because the terminal needs a duplex socket.
2. **Session that survives relaunch** (red: auth/session). This is PC-7 in
   the README's history. Hold a refresh credential in the Keychain, scoped
   to this device and revocable from Settings. The same device credential
   serves sync (§2.5).
3. **Distribution** (Developer ID, hardened runtime, notarization, Wails
   `app.Updater` with ed25519-signed Sparkle or manifest feeds). This
   waits on a paid account.

---

## 2. The synced folder ("share a folder with your Choir server")

### 2.1 The product

A folder on the Mac, and later on Windows, Linux and iOS, mirrors the
computer's Files root (what the Files app shows, `/mnt/persistent/files` in
the guest):
- Drop a file in and it appears on the computer, visible to the Files app
  and to agents.
- An agent writes a file and it appears in the folder.
- Deletes and renames propagate.
- Conflicts are kept, never silently resolved.

**Authority:** the computer's files root is the single state authority
(standing question 4). The folder is a replica. filecas is the encrypted
off-guest backup of that root, not a second authority.

### 2.2 What Choir already has (code facts)

**The guest Files API** (`internal/autoputer/files.go`)
- `GET /api/files[/{path}]` lists a directory as `[{name,type,size,modified}]`
  or streams a file through `http.ServeContent`, with Range and
  If-Modified-Since.
- `POST` makes a directory. `PUT` writes a whole file (the parent must
  exist). `DELETE` removes a file or an empty directory (409 if not empty).
- Paths stay under the root, and `..` is rejected.
- The proxy forwards `/api/files*` to the guest (`internal/proxy/handlers.go`
  `HandleProtectedAPI`).
- The CLI already does `choir files mkdir|put|get` (`cmd/choir/files.go`).

**Change events**
- Each mutation made through the API emits a `file.changed` product event
  with operation, path, size and `source_device_id` (from `X-Choir-Device`)
  (`internal/autoputer/run.go:531-552`).
- `/api/ws` delivers these events, with `?after_seq=` catch-up capped at 500
  (`internal/agentcore/live_ws.go`).
- `FileBrowser.svelte` refreshes on them.

**filecas** (`internal/filecas`, `internal/autoputer/file_sync.go`, served
by corpusd)
- 4 MiB fixed chunks, XChaCha20-Poly1305 under a per-computer key, with
  deterministic nonces so unchanged chunks deduplicate.
- A plaintext manifest per root, whose root hash is SHA-256 of the canonical
  JSON.
- The guest syncs every 15 minutes, or on `POST /api/files/sync`. It then
  records `file_root_committed` on the tape.
- After a boot on a fresh volume, it hydrates from the latest root;
  otherwise the local copy wins.
- Old roots keep their manifests, but their chunks are collected after a
  30-minute grace, so there is **no usable file history** today.

**Auth for non-browser clients**
- `choir_sk_…` API keys sent as bearer tokens, optionally bound to a
  computer (`internal/proxy/handlers.go:526-584`,
  `api_key_computer_authority.go`).
- `/api/files` currently needs `read:runtime` / `write:runtime`. A `files`
  scope is a two-line addition: `validScopes` plus a case in
  `requiredAPIKeyScope`.
- The desktop's native session is cookie-only, and refresh is cookie-only.

**History: Base sync and File Provider were built once** (M5 and M6,
2026-06-28)
- What existed:
  - Go sync engine: scan, delta, plan, execute, conflicts (keep_local,
    keep_remote, keep_both, never silent), cursor.
  - API key in the Keychain; root folder `~/Choir`.
  - Swift `NSFileProviderReplicatedExtension` talking to the Go app over a
    Unix-socket HTTP bridge.
- Unregistered 2026-07-10, deleted 2026-10-12 (1edc2b1bf).
- The reason: it synced against **Base** (`/api/base/*`), a separate store
  that must not compete with the canonical one (SEM-09). Its last host went
  away 2026-07-19. It was never signed or run in Finder.
- **The lesson** *(inference)*: the architecture (Go engine, thin Swift
  extension, socket bridge, explicit conflicts) was sound; the backing store
  was wrong. The new design targets the computer's own files, so there is no
  second store.

### 2.3 Gaps between today's API and a sync client

| Gap | Why sync needs it | Fix (all in the guest Files API unless noted) | Class |
|---|---|---|---|
| No tree listing with content identity | Detect changes without re-downloading | `GET /api/files-sync/changes?cursor=` returns `{entries:[{path, kind, size, mtime, sha256, version}], tombstones:[...], cursor}`. An empty cursor returns the full tree | orange |
| No conditional writes | Detect "both sides changed" | Return an `ETag` on GET and PUT; `PUT` and `DELETE` honour `If-Match` and `If-None-Match: *`, with 412 on mismatch | orange |
| No rename | A move should not become a delete plus a re-upload | `POST /api/files-sync/move {from,to,if_match}` | orange |
| Unbounded `io.ReadAll` on PUT | Memory safety on a 4–8 GiB guest | A size cap. Above it, resumable uploads: a session with `Content-Range` chunks streamed to a temp file, then renamed | orange |
| No events for writes made inside the guest (agents, tools) | Agent output must reach the folder | A guest file watcher (inotify through fsnotify) over the files root feeds the same change log. The cursor is a per-computer file sequence, not the product-event seq. *Hypothesis:* `/api/ws` catch-up capped at 500 is the wrong substrate for a sync cursor | orange |
| No device credential | A sync daemon runs for weeks, unattended | "Link this device": from the native session, mint a computer-bound API key with scopes `read:files`/`write:files` and a device name. It is stored in the Keychain (Windows Credential Manager, libsecret) and listed and revocable in Settings | **red** (auth) |
| A sync poll wakes the computer | A hibernated computer should stay asleep when nothing changed | A host endpoint returns the latest filecas root and `head_sequence` for the caller's computer without resuming it (corpusd already holds `computer_file_roots`). Wake only when it differs, or for an upload | orange (proxy/corpusd read path) |
| No file history | "Previous versions", undo after a bad sync | Retain filecas roots on a schedule (for example hourly for 2 days, daily for 30) instead of latest plus 30 minutes, and serve `GET …/versions?path=` from the manifests. File bytes do **not** go in Dolt or the Autopaper object graph: that graph is for *published* objects. Sharing a file publicly is a later "publish" act | red (artifact GC, retention) |

**Conflict rule (carried over from M5):**
- Every upload carries the base version it was edited from.
- On a 412 the engine keeps both: the server copy stays at the path, and
  the local copy is uploaded as `name (conflict, <device>, <date>).ext`.
- A delete against a changed file becomes a conflict, not a delete.
- Nothing is resolved silently.

### 2.4 Placeholders and on-demand download, per platform

**macOS: File Provider (`NSFileProviderReplicatedExtension`, macOS 11+)**
- **How it works.** The system owns the local replica under
  `~/Library/CloudStorage`. Files can be "dataless" (metadata only):
  - Opening one makes the system call the extension's `fetchContents`
    (`fetchPartialContents` covers partial reads).
  - Enumerators serve the root, the working set and the trash. The working
    set answers `enumerateChanges(from: syncAnchor)`.
  - Remote changes arrive either through a PushKit File Provider push or
    through the host app calling `signalEnumerator`.
  - Dropbox and Google Drive moved to this model around 2023; macOS now
    controls their folder location and sync icons.
- **Requirements:**
  - a Swift or Objective-C `.appex` inside the host app;
  - a sandboxed extension (Apple DTS: macOS app extensions must be
    sandboxed; the host app need not be);
  - an App Group shared by app and extension;
  - an `NSFileProviderDomain` added by the host app.
  - There are no Go bindings. The extension talks to the Go app over
    `NSFileProviderServiceSource` XPC, the App Group container, or local
    HTTP.
- **Without a paid account:**
  - Apple's capability table lists App Groups, App Sandbox and keychain
    sharing for free accounts. It does **not** list FileProvider Testing
    Mode, push notifications or associated domains. Free accounts "can't
    distribute apps".
  - On macOS 15+, an App Group claim is valid only with an App Store or
    TestFlight install, a Team-ID-prefixed group, or a provisioning profile.
    An ad-hoc build has none of these *(inference)*.
  - Developers report the File Provider daemon refusing extensions with the
    wrong profile (-2001, sandbox deny).
  - **Verdict:** treat File Provider as needing the $99/year program.
    Loading from a free Personal Team build is *(unconfirmed)*. It costs
    about an hour to try once Xcode is signed in, but do not plan around it.
- **Remote change signal:** pushes need the paid program. Until then, the
  host app polls the cheap root endpoint (§2.3) and calls
  `signalEnumerator`.

**iOS: File Provider for the Files app**
- `NSFileProviderReplicatedExtension` on iOS 16+.
- Free provisioning expires in 7 days (3 devices, 3 apps). Shipping needs
  the paid program and App Store review.

**Windows: Cloud Files API (cfapi; Windows 10 1709+)**
- `CfRegisterSyncRoot` and `CfConnectSyncRoot` in `cldapi.dll`, plus
  `StorageProviderSyncRootManager` (WinRT) for Explorer registration.
- A kernel filter (`cldflt.sys`) calls the sync engine to hydrate
  placeholders: full, progressive or partial. NTFS only.
- Microsoft documents cleanup for unpackaged (non-MSIX) sync engines, so
  MSIX looks optional.
- We found no Go library *(unconfirmed)*. Use `x/sys/windows` syscalls,
  plus a small WinRT shim for registration.
- ProjFS is for fast local backing stores (VFS for Git) and is not
  recommended for cloud sync.

**Linux: no standard placeholder API**
- Nextcloud's Linux virtual files are experimental 0-byte placeholders.
  A cross-desktop "Open VFS" was only proposed (2023).
- FUSE (go-fuse) works on Linux. It is weak on macOS.
- **Linux stays a plain two-way sync folder**, the rclone bisync and
  Syncthing model.

### 2.5 Architecture

```text
Choir server (per computer)
  guest: /api/files (+ files-sync changes, If-Match, move, resumable upload)
         fsnotify watcher -> file change log (cursor)
         filecas sync -> corpusd (encrypted roots; history by retention)
  host:  proxy (device API key, scope files) ; corpusd latest-root read (no wake)

internal/filesync (Go, shared)            Swift (later)
  state DB (path, version, sha256, mtime)   macOS File Provider .appex  --XPC/socket--> Go engine
  scan local / pull changes / plan /        iOS File Provider .appex    --HTTP--> /api/files-sync
  apply / conflicts / cursor                (iOS has no Go engine; the API is
  clients: choir CLI, Wails app,             simple enough for a second client)
           Windows cfapi shim, Linux daemon
```

**Why the engine is a Go package with the CLI as its first client:**
- It can be driven on staging with no GUI and no Apple account. That is
  standing question 9 (no SSH needed) and gives an E2E artifact.
- The Wails app then runs the same engine as a background service. That is
  the "fate-sharing" question: a sync failure must not take the window
  down. Run it in its own goroutine with its own state.

### 2.6 Order and the minimal first slice

| Step | What | Needs | Class |
|---|---|---|---|
| **S1 (first slice)** | Server: `files-sync/changes` (full listing with sha256 plus cursor, from a guest watcher), ETag/If-Match on PUT/DELETE, move, a PUT size cap. Client: `internal/filesync` plus `choir files sync <dir>` (full download, no placeholders, conflict copies). Auth: a computer-bound API key with new `read:files`/`write:files` scopes | Nothing from Apple | orange (guest API, CLI) + **red** for the scope addition (auth) |
| S2 | Wails app: "Sync folder" setting (`~/Choir`), engine as a background service, device link (key in Keychain), status in the menu bar. Same on Linux | Wails beta | orange; red for device-link minting |
| S3 | No-wake root check (host), file history via filecas retention, "Previous versions" in the Files app | — | red (artifact GC) |
| S4 | macOS File Provider extension (dataless files, `fetchContents`, working-set changes, `signalEnumerator` on poll) | **Paid Apple account** | orange (client) |
| S5 | iOS File Provider extension inside the mobile app (Swift client of the same API) | Paid account, App Review | orange |
| S6 | Windows cfapi placeholders | Windows signing | orange |

**S1 acceptance proof** (staging; the deployed-acceptance level):
- **Script:** create a temp dir, run `choir files sync`, then:
  - write locally and see it via `GET /api/files`;
  - write via the API (or an agent run) and see it locally;
  - edit both sides from the same base and get one conflict copy with
    nothing lost;
  - rename and delete both propagate;
  - interrupt mid-upload and resume.
- **Artifact:** a JSON transcript with server ETags and local hashes.

---

## 3. Server-less desktop: computers running on the Mac

### 3.1 Options (from the survey and the isolation note, updated)

| Option | Works on | For | Against |
|---|---|---|---|
| **A. Linux appliance VM** running the Node B stack, with Firecracker nested inside | M3+ on macOS 15+ (`VZGenericPlatformConfiguration.isNestedVirtualizationSupported`); WSL2 | Almost no code divergence from the server | Excludes M1/M2 Macs; nested overhead; two layers of memory budget; large image |
| **B1. vfkit backend** (CLI over Virtualization.framework; used by Podman, CRC and minikube; v0.6.4; has a Go config package) | Every Apple Silicon Mac, macOS 12+ (save/restore needs 14+) | A subprocess per guest, the same shape as Firecracker. microvm.nix already builds vfkit guests. Fastest to prototype | No snapshot through vfkit *(unconfirmed for current vfkit)*, so hibernate is a stop |
| B2. In-process VZ via `Code-Hex/vz` (v3.8.0, 2026-09-28; macOS 12+, Go 1.25+) | Same | `SaveMachineStateToPath` (arm64, macOS 14+), balloon, virtio-fs, vsock, nested | VMs live inside the app process: an app crash kills every computer (fate-sharing). Restore fails on any config mismatch |
| C. libkrun/krunkit (HVF; libkrun 1.19.6) | macOS 14+ arm64; Linux | Library VMM, small device set, TSI networking | No snapshots; a second VMM to reason about on the desktop |
| D. Apple `container`/Containerization | macOS 26+, Apple Silicon | One VM per container; its `VirtualMachineManager` protocol already abstracts VZ and Cloud Hypervisor | Swift; container-shaped; macOS 26 floor |

**Recommendation: B1, then B2.**
- B1 (vfkit) implements the `vmbackend.Backend` interface (b6f27ded4) as a
  third backend, with `Caps{Snapshot:false}`. `PlanLaunch` emits vfkit
  arguments, and the process registry identifies it by cmdline.
- The interface was designed for this: capabilities, not type switches; a
  backend without snapshots makes hibernate a stop.
- Move to B2-style save/restore only when warm resume on the desktop
  matters. Do it in a helper process (a tiny Swift or Go VZ helper per
  computer), not inside the Wails process.
- Keep A as a one-off prototype if an M3+ machine is at hand. It is not the
  product path.

**Entitlement:** `com.apple.security.virtualization` works with **ad-hoc**
signing (the `Code-Hex/vz` README signs with `-s -`). So this R&D needs no
Apple account. Bridged networking (`com.apple.vm.networking`) is restricted;
use NAT plus vsock.

### 3.2 Prerequisites (in order)

1. **An aarch64-linux guest image.** The flake builds x86_64 only. The work:
   - add `aarch64-linux` outputs;
   - use the right `rusty_v8` archive per architecture;
   - drop the `i8042.*` boot arguments on arm64;
   - build with an arm64 kernel `Image`.

   Build it on the nix-darwin `linux-builder` or a remote aarch64 builder.
   Red: the guest image.
2. **The `vmbackend` seam lands and vmmanager migrates onto it** (the
   vm-backend report §9). Red: VM lifecycle.
3. **Guest identity off source IPs.** vmctl, maild and the gateway identify
   a guest by its tap address. VZ NAT and vsock have no per-guest
   host-visible tap. Use a per-guest credential (the gateway token already
   exists) or a vsock channel per guest. Red: auth.
4. **One state root** (`CHOIR_STATE_DIR`, defaulting to
   `~/Library/Application Support/Choir`) instead of `/var/lib/go-choir`
   literals. Orange.

### 3.3 What runs where

- **Host services as sidecars**, supervised by the Wails app: proxy, vmctl
  (vfkit backend), gateway, corpusd plus `dolt sql-server` (platform store),
  checkpointd, and goproxy for self-development.
  - Ship them as one multi-call binary (`choir-host <service>`).
  - Sidecars keep crash isolation and the server's code paths.
  - maild, Qdrant, SearXNG and front-importance are optional or off.
- **Single-owner auth.** The auth service still mints the JWTs the proxy
  verifies; possession of the device is the owner credential. The window
  loads the local proxy, the same proxy shape as cloud mode, so §1's
  surface handler is reused with `CHOIR_BACKEND=http://127.0.0.1:<port>`
  (loopback HTTP is already allowed).
- **Restart rules** (CLAUDE.md "Restarts End Work"):
  - App quit means a planned hibernate, with the marker written.
  - Laptop sleep means VM pause.
  - An app crash or power loss is a crash, and closes open work.

### 3.4 Models, search, Autopaper

- **Bring your own keys.** A settings pane writes provider keys to the
  Keychain. The supervisor injects them into the gateway's environment, so
  the guest still never sees them (VAL-VM-011).
- **Local models:**
  - Ollama (`localhost:11434/v1`), `llama-server` and `mlx_lm.server` all
    expose OpenAI-compatible endpoints.
  - The gateway lacks a **generic OpenAI-compatible provider**. Add one
    (orange: provider routing, full ceremony).
  - *Hypothesis* (from the cross-model cell-error report): small local
    models fail desk work (Go cells) long before they fail chat. Use local
    models for embeddings, redaction and summaries first; keep frontier
    APIs for the desks.
  - Rough fit at Q4: about 7–8B parameters on 16 GB, 8–14B on 24 GB, 14B
    comfortably on 32 GB *(estimate)*.
- **Search:** BYO search API keys, or a SearXNG the user runs.
- **Autopaper:**
  - Reading the public front works today without an account
    (`/api/platform/publications/front`, which the desktop just rendered).
  - Adopting objects into a local computer's tape needs signed cross-host
    export plus verification of the publishing platform key (the federation
    work in the self-hosting survey §3.7–3.8).
  - A personal local paper is expensive (about $109 a day for 14 sources at
    list prices, per the pricing notes), so consume the network's paper.

### 3.5 Resource budget (estimates)

| Item | Memory | Disk |
|---|---|---|
| macOS plus the user's apps | 6–8 GB | — |
| Host sidecars (Dolt about 0.3–0.5 GB; Go services about 50–150 MB each) | about 1 GB | platform Dolt and artifacts: grows with use |
| One computer guest | 4 GiB is a platform computer's size and thrashed on a compacted owner store, so 6 GiB is safer (Node B's interactive guests are 8 GiB; the code default is 2 GiB) | data.img 5–8 GiB; shared erofs store; a snapshot equal to memory if save/restore is used |
| A local 8B model | about 5–6 GB | about 5 GB |

- **A 16 GB Mac** (the base MacBook Air since late 2024) runs one computer
  with cloud models. A local model as well is tight.
- **24–32 GB** runs one computer plus a local 8B model.
- Two computers (the owner's plus a paper computer) wants 32 GB.

### 3.6 Staged plan

| Stage | Outcome | Proof |
|---|---|---|
| L0 | aarch64 guest image builds; boots under `vfkit` by hand on this Mac (serial console, health) | A boot log artifact |
| L1 | `vmbackend` vfkit backend: cold boot, stop, metrics, process identification; conformance suite runs on macOS | `go test` against real vfkit |
| L2 | `choir-host` multi-call binary plus supervisor in the Wails app; local proxy, auth (single owner), vmctl (vfkit), gateway (BYO key), corpusd plus Dolt. One computer boots and the desktop surface loads in the window | Desktop E2E artifact: snapshot plus run transcript (a new admissible evidence class, because staging cannot prove a Mac) |
| L3 | Quit maps to planned hibernate (marker), sleep to pause; crash semantics verified | Restart transcript |
| L4 | VZ save/restore in a helper process; warm resume | Resume timing artifact |
| L5 | Local models through the generic OpenAI-compatible provider; Autopaper adoption once federation exists | — |

All stages are red except L0 (yellow: an image build with no product
behaviour) and L5's provider (orange, with full ceremony because it is
provider routing).

---

## 4. Windows and Linux versions

**Wails v3 status**
- The beta line runs weekly: beta.0 on 2026-08-02, beta.29 on 2026-10-13.
  v2 remains the stable line, and no date is set for v3.0.
- **Windows:** WebView2 (Evergreen runtime), amd64 and arm64.
- **Linux:** GTK4 plus WebKitGTK 6.0 by default. GTK3 plus WebKit2GTK 4.1
  is available behind `-tags gtk3` and is removed in v3.1.
- **Packaging:** `wails3 package` builds NSIS (default) or MSIX on Windows,
  and AppImage, deb, rpm or Arch packages on Linux.
- **Updates:** a built-in `app.Updater` with ed25519 signatures (GitHub
  Releases, Sparkle appcast, keygen.sh, or the Wails manifest).
- **The custom-scheme asset server does not do WebSockets** on any
  platform. The §1.4 relay applies everywhere.

**What ports as-is**
- The surface proxy (`surface.go`) and the native cookie jar are pure Go.
- The asset handler is the same on all three platforms. The origin differs
  (`wails://localhost` on macOS, `http://wails.localhost` on Windows and
  Linux *(unconfirmed)*). Nothing depends on it.
- `snapshot_other.go` and `webauthn_session_other.go` are stubs.

**Passkeys per platform**

| Platform | Embedded engine | Plan |
|---|---|---|
| macOS | WKWebView: passkeys only for associated domains, which needs a paid account, and not for a `wails://` origin | Keep `ASWebAuthenticationSession` (works today, no account needed) |
| Windows | WebView2: Windows Hello reported missing in WebView2 while it works in Edge (WebView2Feedback #5663, 2026-08); mixed reports | System browser plus redirect to `choir://` (registered protocol handler) or a loopback port, then the same one-time-code redeem |
| Linux | WebKitGTK: no WebAuthn (WebKit bug 205350, still NEW) | System browser plus loopback redirect (`http://127.0.0.1:<port>/callback`) |

The server side (`/auth/desktop/exchange-redirect` and `redeem`) is reused.
Only the callback receiver differs per platform. The callback authority
check (`desktopExchangeCode`) needs a loopback variant: an exact
port-and-path match with the per-session `state`. Red (auth).

**Signing and packaging costs**
- **macOS:** Apple Developer Program, $99 a year. Needed for Developer ID,
  notarization, File Provider and associated domains.
- **Windows:**
  - Azure Artifact Signing (formerly Trusted Signing) from $9.99 a month.
    Individual onboarding requires US or Canada residence, and may be paused
    *(unconfirmed)*.
  - An OV certificate costs about $150–300 a year.
  - EV no longer bypasses SmartScreen; reputation builds over weeks either
    way. The Microsoft Store avoids the warnings.
- **Linux:** free. AppImage, plus deb and rpm, optionally signed repos.

**Order**
1. Linux first. It is the cheapest, and the same Firecracker backend makes
   it a server-less target soon after the desktop supervisor exists
   (survey §4 row 3).
2. Windows after the auth redirect variant. Its server-less path is WSL2
   with nested KVM (the appliance shape).

---

## 5. How this connects to the mobile app

There is no mobile code in the repo today. The Autoradio app is vision
([choir-state-and-vision-2026-10-14.md](../archive/reports/choir-state-and-vision-2026-10-14.md)).
What carries over:

- **The sync API is the contract.** `files-sync/changes`, If-Match writes,
  move, resumable upload and device keys are plain HTTP. The iOS app is a
  Swift client of the same API. Its File Provider extension puts the
  computer's files in the Files app, with on-demand download (iOS 16+
  replicated extension). Keep the API small enough to implement twice: Go
  for the desktops and CLI, Swift for iOS.
- **The same window pattern.** The Mac app now renders the server's own
  surface inside a native shell, with native auth. A mobile app can do the
  same for the parts that are web (Autopaper, Texture reading). Native code
  is kept for what must be native: audio and Autoradio, File Provider, and
  push. No second copy of the UI ships in the app, so it cannot drift from
  the computer's surface.
  - Caveat: App Review rules on downloaded code and mini-app catalogs
    (guidelines 2.5.2 and 4.7, to be re-checked) mean the mobile app should
    be a focused Autoradio/Autopaper/Files client, not the full desktop.
- **Auth.** On iOS, `ASWebAuthenticationSession` with the same
  `desktop-bridge.html` one-time-code flow works as on the Mac. Native
  passkeys through `ASAuthorizationPlatformPublicKeyCredentialProvider`
  need associated domains, so a paid account and an
  `apple-app-site-association` file on choir.news.
- **Device keys.** The "Link this device" credential from §2.3 is the same
  object on every platform, listed and revocable in Settings.

---

## 6. Recommended first slices

1. **Sync S1** (§2.6): guest `files-sync/changes` with a watcher, If-Match,
   move and a size cap; `internal/filesync`; `choir files sync`; and the
   `read:files`/`write:files` scopes. Classes: orange plus red (auth
   scope). It needs no Apple account, and its proof runs on staging.
2. **Desktop WebSocket relay** (§1.4a), so the Mac app has a live channel
   and the terminal.
3. **The aarch64 guest image** (§3.2.1). Every server-less path is blocked
   on it, and so is the owner's interest in the Mac running computers.
4. **Owner decision: enrol in the Apple Developer Program** when File
   Provider (S4), notarized distribution and iOS are wanted. It costs $99 a
   year and needs no design work, but it gates three tracks.

## 7. Standing questions (for the S1 mission definition)

| # | Question | S1 answer |
|---|---|---|
| Q1 | Decision provenance | Owner, 2026-10-14 (quoted above) |
| Q2 | Settled-decision conformance | No Base revival (SEM-09). The computer's files root stays the authority |
| Q4 | Single state authority | Guest files root. The local folder is a replica; filecas is a backup |
| Q5 | Artifact-verified success | The E2E JSON transcript (§2.6) |
| Q6 | Fate-sharing | The engine runs in its own goroutine and state; a sync crash does not take the window down |
| Q7 | Restart durability | The engine's cursor and state DB survive restarts. The watcher log is rebuilt from a full listing on a cursor mismatch |
| Q9 | No-SSH operability | CLI plus API key; no host access needed |

## 8. Sources

Apple:
- File Provider: https://developer.apple.com/documentation/fileprovider/synchronizing-files-using-file-provider-extensions
- Replicated extension: https://developer.apple.com/documentation/fileprovider/replicated-file-provider-extension
- Capability tables:
  - macOS: https://developer.apple.com/help/account/reference/supported-capabilities-macos
  - iOS: https://developer.apple.com/help/account/reference/supported-capabilities-ios
- App Groups on macOS 15: https://developer.apple.com/forums/thread/721701
- Extension sandboxing: https://developer.apple.com/forums/thread/798886
- File Provider profile errors: https://developer.apple.com/forums/thread/729740
- Passkeys: https://developer.apple.com/documentation/authenticationservices/supporting-passkeys
- Virtualization entitlement: https://developer.apple.com/documentation/bundleresources/entitlements/com.apple.security.virtualization
- Save state: https://developer.apple.com/documentation/virtualization/vzvirtualmachine/savemachinestateto(url:completionhandler:)
- Nested virtualization: https://developer.apple.com/documentation/virtualization/vzgenericplatformconfiguration/isnestedvirtualizationsupported

Microsoft:
- cfapi: https://learn.microsoft.com/en-us/windows/win32/cfapi/build-a-cloud-file-sync-engine
- ProjFS: https://learn.microsoft.com/en-us/windows/win32/projfs/projected-file-system
- SmartScreen: https://learn.microsoft.com/en-us/windows/apps/package-and-deploy/smartscreen-reputation
- Artifact Signing pricing: https://azure.microsoft.com/en-us/pricing/details/artifact-signing/
- WebView2 passkeys: https://github.com/MicrosoftEdge/WebView2Feedback/issues/5663

Wails:
- Releases: https://github.com/wailsapp/wails/releases
- Status: https://docs-preview.wails.io/status/
- Updater: https://docs-preview.wails.io/guides/updater/
- Builds: https://v3.wails.io/guides/build/windows and https://v3.wails.io/guides/build/linux

VMMs:
- https://github.com/Code-Hex/vz
- https://github.com/crc-org/vfkit
- https://github.com/containers/krunkit
- https://github.com/apple/container

Linux:
- WebKitGTK WebAuthn: https://bugs.webkit.org/show_bug.cgi?id=205350
- rclone bisync: https://rclone.org/bisync/
- go-fuse: https://github.com/hanwen/go-fuse

Local models:
- https://docs.ollama.com/api/openai-compatibility


## Merged from docs/reports/self-hosting-and-desktop-2026-10-14.md (2026-10-14)

**Self-Hosting Choir, and the Desktop App as a Server-less Deployment.**
Date: 2026-10-14. Research and design survey, read at `main` f54f05c42. Mutation class of this note: **green** (docs only). Most of the work it proposes is orange or red (§6). Nothing here was run against Node B or staging. One public, read-only probe was made: `GET https://choir.news/api/platform/publications/front` returned 200.
- Verdict.** A determined Linux and Nix operator with an Intel bare-metal

(Full text in git history at the path above.)
