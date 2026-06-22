# Choir Desktop with Wails v3 — Build Spec

**Date:** 2026-06-22  
**Status:** spec v1 — plan agreed; Phase 1 next  
**Research basis:** [choir-base-research-report-2026-06-06.md](choir-base-research-report-2026-06-06.md)  
**Product spec:** [choir-base-product-spec-2026-06-06.md](choir-base-product-spec-2026-06-06.md)  
**Architecture:** [intended-architecture-next-2026-06-06.md](intended-architecture-next-2026-06-06.md)

## Document Role

This spec defines the build order, architecture, and acceptance gates for
Choir Desktop with Wails v3. It supersedes the landmark ordering in the Base
product spec for the desktop/File Provider/sync slice, with justification.

It does not activate a mission. When ready, author a Parallax paradoc or
mission doc from the then-current code and staging state.

## Agreed Plan

```text
Now:     Phase 1 — Wails v3 desktop shell (functional, not just a spike)
Next:    Phase 2 — Local Go services (native menus, tray, file access, notifications)
Later:   Phase 3 — Base reconciliation kernel (pure Go, can run in parallel with Phase 2)
         Phase 4 — Base sync wired into desktop
         Phase 5 — macOS File Provider extension
         Phase 6 — Host-to-VM folder sync
         Phase 7 — Workstation-native Choir (local VMs instead of cloud)
```

Phase 1 and Phase 2 are the immediate execution sequence. Phases 3-7 are
spec'd but not yet activated. Phase 3 can run in parallel with Phase 2 once
Phase 1 proves the Wails v3 shell works.

## Decision: Development Order

### The Question

Should we build the desktop first, then macOS File Provider, then the cloud
storage backend (Base sync) to sync a shared folder inside a user's VM with
Choir Desktop? Or another order?

### The Answer: Desktop First, Then Base Sync, Then File Provider

```text
Phase 1: Wails v3 desktop shell wrapping the existing Svelte frontend
Phase 2: Local Go services (native menus, file access, notifications, tray)
Phase 3: Base reconciliation kernel + blob store (pure Go, no UI)
Phase 4: Base sync wired into the desktop app (status, repair, conflict UI)
Phase 5: macOS File Provider extension on the signed app bundle
Phase 6: Host-to-VM folder sync through Base contracts
Phase 7: Workstation-native Choir (local VMs, local runtime, no cloud dependency)
```

### Why This Order

**Desktop first** because:

- The Svelte desktop already works on staging at `choir.news`. Wrapping it in
  Wails v3 is the fastest path to a native macOS app — the frontend exists,
  the backend services exist, the WebSocket/EventSource streams exist.
- The Wails v3 spike gate (auth, WebSocket, PDF/media, bridge call, packaging)
  must be passed before anything else depends on the shell. File Provider
  requires a signed app bundle; Base sync needs a UI surface. Both depend on
  the desktop shell existing first.
- Wails v3 is alpha software. Proving it works with Choir's specific surfaces
  (WebAuthn, WebSocket, VText streams, PDF.js, ghostty-web) is the
  highest-information first step. If v3 fails, we learn before investing in
  Base or File Provider.
- The desktop app is useful immediately without Base or File Provider: it
  connects to staging, runs the full web desktop in a native window, and adds
  native menus/tray/notifications.

**Base sync second** (not File Provider) because:

- File Provider is a system contract that needs a sync backend to talk to.
  Building File Provider without Base would produce a shell extension with
  nothing to serve — no items, no blobs, no conflict resolution.
- Base reconciliation kernel is pure Go with deterministic tests. It has no
  UI dependency, no signing dependency, and no Apple API dependency. It can
  be built and tested in parallel with the desktop shell if desired.
- Once Base exists, the desktop app becomes its observer/control pane
  (Landmark 5 in the product spec). This is the natural integration point.

**File Provider third** because:

- File Provider requires: (1) a signed app bundle, (2) a sync backend, (3)
  Apple's `NSFileProviderReplicatedExtension` protocol, (4) entitlements and
  hardened runtime. All four dependencies must exist first.
- The signed app bundle comes from Phase 1. The sync backend comes from
  Phase 3. File Provider is the capstone, not the foundation.
- The product spec says: "The macOS File Provider utility must be the smallest
  signed app bundle that can become the full Choir app. It should not be
  disposable." This means File Provider is built on top of the real app, not
  as a throwaway prototype.

**Host-to-VM folder sync last** because:

- This is sync plane 4 in the product spec: "Mac host folder or File Provider
  domain <-> local Choir VM/candidate computer." It requires Base, File
  Provider, and vmctl's Apple Virtualization backend to all work together.
- It is the most integration-heavy piece and has the most external
  dependencies (Apple Virtualization entitlements, VM networking, file
  sharing semantics).

**Workstation-native as Phase 7** because:

- This is the far end of the platform continuum from the architecture doc:
  "desktop app running local VMs and optionally local models." It means the
  user's computer runs locally as an Apple Virtualization VM instead of on
  cloud infrastructure.
- It depends on the desktop shell (Phase 1-2) and vmctl's Apple
  Virtualization backend, but does NOT strictly depend on Base/File Provider
  (Phase 3-6). A user could run a local computer without file sync. The full
  product wants both, but they are separable.
- It is the most ambitious phase: local auth, local Dolt, local gateway,
  local vmctl, local runtime — essentially running the entire Choir platform
  stack on a Mac. It should be spec'd and deferred, not front-loaded.

### Why Not The Product Spec's Original Order

The product spec orders: Base kernel → Wails observer → Base platform →
Node A/B → File Provider → full desktop. That order front-loads the pure
algorithmic core, which is correct if the goal is to prove reconciliation
correctness before any UI investment.

The revised order front-loads the Wails v3 risk gate because:

1. Wails v3 is alpha. If it fails the spike gate, the entire desktop/File
   Provider/sync plan changes. Learning this early is higher-information than
   proving reconciliation correctness that has no UI to observe it.
2. The existing Svelte desktop already works. Wrapping it is low-risk product
   value, not speculative investment.
3. Base can be built in parallel with the desktop shell once the Wails spike
   passes. The two phases are not strictly serial.

## Phase 1: Wails v3 Desktop Shell

### Goal

A **functional** macOS desktop app, not just a spike. The user launches
Choir.app, sees the full Svelte desktop, can authenticate, can use all apps,
can edit Textures, can view sources — everything that works on
`choir.news` in a browser works in the native window.

The app connects to staging (`choir.news`) for all backend services. No local
backend services in Phase 1 — it is a native shell around the existing web
desktop.

### Architecture

```text
Wails v3 app (Go)
  -> embeds frontend/dist (Svelte build output)
  -> window loads embedded assets in production, dev server in dev mode
  -> all /auth/* and /api/* calls route to staging.choir.news (or localhost proxy)
  -> no local backend services in Phase 1 — pure shell
```

### Project Structure

```text
cmd/desktop/
  main.go              Wails app entry point
  go.mod               separate module (or tagged submodule) for Wails dependency
  build/
    darwin/
      Info.plist
      Info.dev.plist
      Taskfile.yml
      icons.icns
    config.yml
    Taskfile.yml
    appicon.png
  frontend/            symlink or build-copy from ../../frontend/dist
```

The Wails v3 dependency (`github.com/wailsapp/wails/v3`) lives in the desktop
module's `go.mod`, not in the main Choir `go.mod`. This follows the product
spec's dependency policy: "Wails v3 dependency to the desktop app module."

### Key Decisions

**Frontend embedding:** The Wails app embeds the Svelte build output
(`frontend/dist/`). In dev mode, the Wails window loads from the Vite dev
server (`http://localhost:5173`). In production, it loads from embedded
assets via Wails' asset server.

**API routing:** In Phase 1, the desktop app connects to staging. The
Svelte frontend's existing fetch/WebSocket calls work unchanged because
they use relative URLs (`/auth/*`, `/api/*`). The Wails app must either:

- **Option A:** Set a base URL (`https://choir.news`) and intercept/proxy
  relative requests through the Wails asset server. This requires a custom
  asset handler that forwards `/auth/*` and `/api/*` to the remote server.
- **Option B:** Patch the frontend to use absolute URLs when running inside
  Wails, configured via an environment variable or build-time define.

Option A is preferred because it requires zero frontend changes. Wails v3
supports custom asset handlers via `application.AssetOptions`.

**WebAuthn/passkeys:** This is the highest-risk item. macOS `WKWebView`
may or may not support WebAuthn correctly. The research report identifies
this as the top spike-gate risk. If WKWebView passkey support is broken,
the fallback is:

1. Route auth through the system browser (open `https://choir.news/auth/*`
   in Safari, get a session cookie, share it back to the Wails app via
   cookie jar or app-group keychain).
2. Or implement a native passkey flow in Go using the platform Keychain and
   WebAuthn APIs, exposed as a Wails service.

**Multi-window:** Wails v3 has first-class multi-window support. Phase 1
uses a single main window. The architecture should allow adding windows
later (settings, logs, node-admin) without restructuring.

### Spike Gate (from Research Report)

The Phase 1 build must pass or precisely fail this gate:

```text
Wails packaged app
  + clean-machine auth/passkey proof
  + WebSocket/EventSource proof
  + PDF/media/VText proof
  + one Go bridge call
  + one local status/Base mock
  + macOS signing/notarization dry run
  + File Provider extension feasibility note
```

If any item fails, record whether the blocker is Wails-specific,
system-WebView-specific, Apple-packaging-specific, or Choir-app-specific
before choosing fallback (Wails v2, Tauri, Electron, native Swift).

### Acceptance Criteria

- [ ] `wails3 dev` opens a window showing the Choir Svelte desktop
- [ ] User can authenticate via WebAuthn/passkey (or documented fallback)
- [ ] WebSocket desktop sync works (live state updates)
- [ ] EventSource VText streams work (document live editing)
- [ ] PDF.js renders a PDF (source viewer)
- [ ] One typed Go bridge call works (e.g., `DesktopService.GetAppInfo()`)
- [ ] `wails3 package` produces a `.app` bundle
- [ ] Ad-hoc signed app launches on a clean macOS user account
- [ ] Notarization dry run completes (submit to Apple, get ticket back)
- [ ] File Provider feasibility note written (what extension type, what
      entitlements, what Apple API friction if any)

## Phase 2: Local Go Services

### Goal

Add typed Wails v3 services for native host capabilities the web app cannot
access.

### Services

```text
DesktopService
  GetAppInfo() -> {version, commit, builtAt}
  GetSystemInfo() -> {os, arch, memory, disk}
  OpenInBrowser(url string) -> error
  ShowNotification(title, body string) -> error

FileAccessService
  ReadFile(path string) -> []byte
  WriteFile(path string, data []byte) -> error
  ListDir(path string) -> []FileInfo
  ChooseFolder() -> string  (native folder picker)

TrayService
  SetTrayIcon(icon []byte) -> error
  SetTrayMenu(items []MenuItem) -> error

BaseStatusService (mock in Phase 2, real in Phase 4)
  GetSyncStatus() -> SyncStatus
  GetConflicts() -> []Conflict
  RepairPreview(itemId string) -> RepairPlan
```

### Design Principles

- Services are plain Go structs with exported methods. Wails v3 auto-generates
  TypeScript bindings.
- Business logic stays framework-neutral. Each service delegates to an
  interface implementation that can be tested outside Wails.
- No service directly mutates server-side state. Services are local-only or
  read-through proxies to the staging API.
- The frontend calls services via generated bindings (`import { DesktopService }
  from '../bindings/...'`). No hand-written RPC.

### Acceptance Criteria

- [ ] Native macOS menu bar with File/Edit/Window/Help menus
- [ ] System tray icon with status indicator
- [ ] Native folder picker works via `FileAccessService.ChooseFolder()`
- [ ] macOS notification works via `DesktopService.ShowNotification()`
- [ ] `BaseStatusService` mock returns hardcoded sync status and the frontend
      renders it in a status panel
- [ ] All services have unit tests outside the Wails runtime

## Phase 3: Base Reconciliation Kernel

### Goal

Build the pure Go reconciliation engine that compares `remote`, `local`, and
`synced` trees and produces operations, conflicts, and stuck states.

This is the product spec's Landmark 1-3 (model nucleus, reconciliation
kernel, delta/repair API). It is pure Go with no UI, no Wails, no Apple
APIs, and no staging dependency.

### Packages

```text
internal/base/model      item/version/blob/event/status types
internal/base/blob       immutable byte storage and hash verification
internal/base/journal    append-only metadata events
internal/base/tree       derive consistent trees from journal events
internal/base/planner    pure three-tree reconciliation
internal/base/status     per-item state and repair handles
internal/base/api        HTTP endpoints over journal/blob/delta/status
internal/base/local      adapters for filesystem scan, Wails bridge
internal/base/testkit    deterministic scenarios, fault injection
```

### Acceptance Criteria

Per the product spec's non-negotiable tests:

- [ ] local add vs remote add same path
- [ ] local edit vs remote edit same file
- [ ] local delete vs remote edit
- [ ] local move vs remote edit
- [ ] concurrent folder moves
- [ ] case-only rename on case-insensitive FS
- [ ] duplicate remote event is idempotent
- [ ] missed local event recovered by scan
- [ ] crash after blob write before event append
- [ ] crash after event append before status update
- [ ] corrupt local blob detected by hash
- [ ] locked file becomes actionable stuck status
- [ ] projection export does not overwrite original

## Phase 4: Base Sync Wired Into Desktop

### Goal

Replace the `BaseStatusService` mock with real Base sync. The desktop app
becomes the observer/control pane for sync status, conflicts, and repair.

### Architecture

```text
Wails v3 app
  -> BaseStatusService (real)
    -> internal/base/local (filesystem adapter)
    -> internal/base/planner (reconciliation)
    -> internal/base/blob (local blob cache)
    -> staging API (remote tree, delta, blob fetch)
  -> frontend renders sync status, conflicts, repair UI
```

### Acceptance Criteria

- [ ] Desktop app shows real sync status per item (synced, syncing, conflict,
      stuck, error)
- [ ] User can preview repair actions before applying
- [ ] User can resolve conflicts (keep local, keep remote, keep both)
- [ ] Filesystem changes are observed and synced (watch + scan)
- [ ] Staging delta API is consumed via cursor
- [ ] Offline edits are queued and synced when connection returns

## Phase 5: macOS File Provider Extension

### Goal

Add a `NSFileProviderReplicatedExtension` to the signed Wails app bundle that
exposes a Base-backed subtree in Finder.

### Architecture

```text
Choir.app
  Contents/
    MacOS/
      choir-desktop        Wails v3 main binary
    PlugIns/
      FileProvider.appex   File Provider extension
```

The File Provider extension:

- Registers one domain (not Desktop/Documents takeover — a Choir-specific
  domain)
- Enumerates items from Base sync status
- Materializes/dehydrates files on demand
- Reports conflicts back to Base
- Uses the same Base contracts as the desktop app (no separate sync engine)

### Key Risks (from Research Report)

- Apple requires `NSFileProviderReplicatedExtension` and
  `NSFileProviderEnumerating` adoption
- Working-set enumeration, materialized/dataless state, signaling enumerators
- Version/conflict semantics must match Base planner output
- Entitlements: app-group, keychain sharing, File Provider extension
  entitlement
- Hardened runtime for both the main app and the extension
- 0-byte file conflict copies (Box has documented this issue)
- Eviction/free-space behavior confusion

### Implementation Path

The File Provider extension is Swift/Objective-C code. It cannot be pure Go.
Options:

1. **Swift extension with Go shared library:** The extension is a Swift
   appex that calls into a Go shared library (built with `go build
   -buildmode=c-shared`) containing the Base sync logic. The extension
   handles Apple protocol conformance; Go handles reconciliation.
2. **Pure Swift extension:** Reimplement Base sync logic in Swift. This
   duplicates the planner and is forbidden by the product spec's "one
   authoritative sync engine owns a subtree" invariant.
3. **Go-based extension via Wails:** Wails v3 does not currently support
   building app extensions. This would require custom build tooling.

Option 1 is the recommended path. The Go shared library wraps
`internal/base/planner` and `internal/base/local` behind a C ABI. The Swift
extension handles `NSFileProviderReplicatedExtension` protocol conformance
and calls the Go library for sync logic.

### Acceptance Criteria

- [ ] Signed app bundle with File Provider extension launches
- [ ] Finder shows Choir domain with file/folder listing
- [ ] User can open a file from Finder (materialization)
- [ ] User can edit and save (upload to Base)
- [ ] User can create a new file in Finder (upload to Base)
- [ ] Conflict produces a visible conflict copy, not silent overwrite
- [ ] Online-only file shows as placeholder (dehydrated)
- [ ] Eviction frees local space correctly
- [ ] Pause/resume sync works
- [ ] Sync status is visible in the desktop app, not only Finder

## Phase 6: Host-to-VM Folder Sync

### Goal

Sync a shared folder inside a user's local Choir VM with the Mac host through
Base contracts.

### Architecture

```text
Mac host
  -> Choir.app (Wails v3)
    -> Base sync service
      -> File Provider domain (host folder)
      -> Apple Virtualization VM (vmctl)
        -> shared folder inside VM
        -> Base sync agent inside VM
```

The VM runs a Base sync agent that talks to the same Base API as the host.
The host's File Provider domain and the VM's shared folder are two clients
of the same Base subtree. The reconciliation kernel handles conflicts
between them.

### Dependencies

- Phase 1-5 must be complete (desktop, Base, File Provider)
- `vmctl` Apple Virtualization backend must be functional
- VM networking and file sharing must be configured
- Base sync agent must run inside the VM (lightweight Go binary)

### Acceptance Criteria

- [ ] File created on host appears in VM shared folder
- [ ] File created in VM shared folder appears on host
- [ ] Concurrent edits produce conflicts, not silent overwrites
- [ ] VM hibernate/resume preserves sync state
- [ ] Large file sync does not block small file sync
- [ ] Sync status visible in both desktop app and VM

## Phase 7: Workstation-Native Choir

### Goal

Run a complete Choir computer locally on the user's Mac using Apple
Virtualization framework VMs, without depending on cloud infrastructure.

The desktop app becomes the control surface for a local Choir instance:
local auth, local Dolt, local gateway, local vmctl, local runtime. The
user's computer is a local VM, not a cloud VM.

### Architecture

```text
Mac host
  -> Choir.app (Wails v3)
    -> local service stack
      -> local auth service (WebAuthn, session)
      -> local proxy service (API routing)
      -> local gateway (model provider credentials, local LLM broker)
      -> local vmctl (Apple Virtualization backend)
      -> local Dolt (embedded, per-computer)
    -> user's active computer (Apple Virtualization VM)
      -> NixOS or Linux guest
      -> conductor, Texture, appagents, researchers, supers
      -> local Base sync agent (if Phase 3-6 are complete)
    -> optional: local LLM via Ollama or llama.cpp
```

### What Changes vs. Cloud Mode

| Component | Cloud mode (Phase 1) | Workstation-native (Phase 7) |
|---|---|---|
| Auth | staging auth service | local auth service (same code, localhost) |
| API routing | staging proxy | local proxy (same code, localhost) |
| Model providers | staging gateway | local gateway with user's API keys or local LLM |
| Computer VM | Firecracker on Node B | Apple Virtualization on Mac |
| Dolt | embedded in cloud VM | embedded in local VM |
| File sync | staging Base API | local Base (if Phase 3-6 complete) |
| Publication | platformd on staging | optional local publication or staging relay |

### Dependencies

- Phase 1-2 must be complete (desktop shell + services)
- `vmctl` Apple Virtualization backend must be functional (separate mission,
  already scoped in the product spec)
- Local service stack must be runnable as native processes or launch agents
  (auth, proxy, gateway can run as localhost Go binaries)
- Apple Virtualization entitlement (`com.apple.security.virtualization`)
- Sufficient Mac resources (RAM, disk for VM images)

### What Does NOT Change

- The Svelte frontend is the same — it talks to `/auth/*` and `/api/*`
  whether those routes are served by staging or localhost
- The Go runtime code is the same — conductor, Texture, appagents, vmctl
  lifecycle, Dolt workspace all run inside the VM
- The product ontology is the same — the user has a persistent computer,
  not a disposable sandbox

### Key Risks

- **Resource pressure:** Running a full Choir VM plus local services on a
  Mac requires significant RAM and disk. Weak machines may not support it.
  The product spec's research report covers this in "Tier C: Apple
  Virtualization Framework VM Per Candidate."
- **Apple Virtualization maturity:** The framework has API limits around GPU
  access, nested virtualization, and device support. VM startup time and
  disk usage may be too heavy for casual use.
- **Local model quality:** If the user routes to a local LLM (Ollama,
  llama.cpp), model quality may be lower than cloud providers for complex
  tasks. Cloud fallback should remain available.
- **Auth complexity:** Local WebAuthn works differently when the auth service
  is localhost. RP-ID and origin assumptions need verification.
- **Networking:** The local VM needs network access for web fetching,
  source ingestion, and optional cloud relay. Apple Virtualization
  networking (bridged vs. shared vs. host-only) needs product-grade
  diagnostics.

### Acceptance Criteria

- [ ] User can boot a local Choir computer from the desktop app
- [ ] Local auth works (WebAuthn registration and login on localhost)
- [ ] User can create and edit Textures in the local computer
- [ ] User can run researcher/super workers in the local VM
- [ ] VM hibernate/resume works from the desktop app
- [ ] Local model provider can be configured (Ollama or API key)
- [ ] Cloud fallback works when local capacity is insufficient
- [ ] Desktop app shows local resource status (CPU, RAM, disk, VM state)
- [ ] Clean shutdown of local services when app quits

### Relationship to Cloud Mode

Workstation-native is not a separate product. It is a deployment mode of the
same product. The desktop app should support both:

- **Cloud mode (Phase 1):** Connect to `choir.news`. User's computer runs on
  cloud infrastructure. Desktop app is a native viewer/controller.
- **Workstation mode (Phase 7):** Connect to localhost. User's computer runs
  as a local Apple Virtualization VM. Desktop app is the native control
  surface for the local stack.

The mode is a config setting, not a fork. The same binary, the same frontend,
the same Go services — just different endpoints.

## Wails v3 Version Pinning

Pin a specific Wails v3 alpha tag. Do not float `latest`. The research report
notes 99 alpha releases over 3+ years with real API churn. Pinning means:

```text
go.mod:
  github.com/wailsapp/wails/v3 v3.0.0-alpha.XXX
```

Record the pinned version and date in this spec when the build starts. If
upgrading, record what changed and re-run the spike gate.

## Fallback Decision Tree

```text
Wails v3 spike passes
  -> proceed with Wails v3

Wails v3 fails on WebAuthn
  -> try system-browser auth fallback
  -> if fallback works, proceed with Wails v3
  -> if fallback fails, evaluate Tauri (native auth APIs)

Wails v3 fails on WebSocket/EventSource
  -> this is a Wails/WebView bug, not Choir-specific
  -> evaluate Wails v2 (stable, single-window)
  -> or Tauri (different WebView bindings)

Wails v3 fails on packaging/signing
  -> evaluate Tauri (different packaging story)
  -> or Electron (heavier but mature macOS packaging)

Wails v3 fails on File Provider extension support
  -> File Provider extension can be built as a separate Swift Xcode target
  -> Wails app and extension share app-group/entitlements
  -> this does not require Wails to support extensions natively
```

## Open Questions

1. **Separate Go module or submodule?** The desktop app needs Wails v3 as a
   dependency. The main Choir `go.mod` should not import Wails. Options: (a)
   separate `cmd/desktop/go.mod` with its own module path, (b) Go workspace
   with multiple modules, (c) build tags that exclude Wails from non-desktop
   builds. Option (a) is simplest.

2. **Frontend sharing:** The desktop app embeds `frontend/dist/`. Should the
   desktop app's `frontend/` be a symlink to the main `frontend/` directory,
   or a build-time copy? Symlink is cleaner for development; copy is safer
   for packaging.

3. **Auth strategy:** Can WKWebView do WebAuthn, or do we need system-browser
   fallback? This is the first thing to test in Phase 1.

4. **Base metadata storage:** Should Base metadata live in the existing VText
   Dolt workspace or a separate Base workspace? (Carried from product spec
   open questions.)

5. **Go shared library for File Provider:** Can `go build -buildmode=c-shared`
   produce a library that Apple's extension loader accepts? Need to verify
   with a minimal proof before committing to Option 1 in Phase 5.

6. **Staging vs. local server:** In Phase 1, the desktop app connects to
   staging. Should it also support connecting to a local Choir server for
   development? Probably yes, via a config setting. This same config setting
   is the foundation for Phase 7 workstation-native mode.

7. **Phase 7 vmctl dependency:** The Apple Virtualization backend for vmctl
   is a separate mission already scoped in the product spec. Should it be
   built before Phase 7 starts, or is it the first task within Phase 7?
   Likely the former — vmctl Apple Virtualization is useful independently of
   the desktop app.
