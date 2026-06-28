# Runtime Deletion and Extraction Plan

**Date:** 2026-06-27
**Status:** planning — catalog of skeletons, remnants, and extraction targets
**Depends on:** `mission-3c_2-actor-runtime-migration-real-v0.md` (actor runtime must be the execution substrate before extraction begins)
**Successor missions:** multiple (one per extraction target)

## Context

`internal/runtime/` is 48,777 lines of non-test Go across ~56 files. It is
the entire business logic of the system — not a concurrency substrate. The
concurrency code is ~2,000 lines. The rest is app logic that accumulated
through multiple refactors (Rust ractor port, interface extraction, actor
runtime migration, heresy deletion, wire rebuild).

This document catalogs everything found in a comprehensive audit on
2026-06-27 and defines the extraction architecture. Each item is a deletion
candidate, a normalization target, or an extraction into its own package.

The actor runtime migration (3c_2) is the prerequisite. You cannot extract
appagents until the execution substrate is the actor runtime, not the god
object. This document assumes 3c_2 Phase 2 is complete (old concurrency
deleted, actor runtime is the execution substrate).

## Deletion Policy

**We are pre-release. There are no users to break. There is no production
data to migrate. Every legacy compatibility shim, every backward-compat
type alias, every "kept for rollback reference" disabled tool, every
"legacy soft path" is dead weight from previous levels of understanding.
Delete it all.**

Rules:
- **Hard cutover, no legacy.** If old data formats or old field names are
  no longer produced, delete the code that reads them. Do not keep
  compatibility shims.
- **No "retained for rollback reference."** Disabled tools, retired
  endpoints, and dead functions are deleted, not commented out or kept as
  reference. Git history is the reference.
- **No "legacy soft path."** If a code path exists only to handle runs
  without some metadata field that is now always set, delete the path.
- **No "temporary" without a timeline.** If something is marked temporary,
  either delete it now or document the specific condition under which it
  will be deleted. "Until X matures" is not a condition — name X and the
  maturity criteria.
- **Delete first, refactor second.** When extracting app packages, delete
  the dead code and legacy compatibility BEFORE moving the surviving code
  to the new package. The extraction is cleaner when you're moving only
  live code.
- **When in doubt, delete.** If you cannot find a live caller, delete. If
  the only caller is a test, delete the test too (or rewrite it to test
  the live path). If the only caller is an eval harness, delete the harness.
  Tests and evals exist to verify live code, not to justify dead code.

## The Extraction Architecture

### Current state (after 3c_2)

```
cmd/sandbox/main.go
  → actorruntime.New()
    → runtime.New()  ← god object (48K lines, 243 methods, 4 remaining mutexes)
      → handler calls rt.ExecuteActivationSync()
        → rt.executeWithToolLoop()
          → tools call back into rt.* methods (texture, wire, browser, etc.)
```

### Target state

```
cmd/sandbox/main.go
  → shared deps: store, provider, eventBus, actorRT
  → register app handlers:
      texture.NewHandler(store, provider, bus, actorRT)
      wire.NewHandler(store, provider, bus, actorRT)
      browser.NewHandler(store, bus, actorRT)
      apppromotion.NewHandler(store, bus, actorRT)
      conductor.NewHandler(store, provider, bus, actorRT)
      super.NewHandler(store, provider, bus, actorRT)
      researcher.NewHandler(store, provider, bus, actorRT)
      vmctl.NewHandler(store, bus, actorRT)
      podcast.NewHandler(store, bus, actorRT)
      email.NewHandler(store, bus, actorRT)
      desktop.NewHandler(store, bus, actorRT)
  → actorRT dispatches: agentID → profile → handler.HandleUpdate()
```

Each app handler:
- Implements `actor.Handler` (HandleUpdate)
- Registers its own tools with the tool registry
- Owns its state (in the store or in actor memory — no shared mutexes)
- Receives shared deps through constructor injection
- Sends messages to other apps via `actorRT.Send()` — the only inter-app coupling

### Core (shared infrastructure, stays)

| Package | Lines (approx) | What |
|---|---|---|
| `internal/actor/` | 326 | Actor runtime (exists, do not modify) |
| `internal/agentcore/` (new) | ~6,000-8,000 | Tool loop, run lifecycle, run memory, coagent update injection, continuation (if kept) |
| `internal/store/` | exists | Persistence |
| `internal/events/` | exists | Event bus |
| `internal/provideriface/` | exists | Provider interface |
| `internal/toolregistry/` | exists | Tool registry |

### App packages (extracted from runtime)

| Package | Source files | Lines (approx) | Priority |
|---|---|---|---|
| `internal/texture/` | texture.go + 15 texture_*.go + tools_texture.go | ~12,000 | Hard (most entangled) |
| `internal/wire/` | wire_*.go + tools_wire_processor.go | ~5,000 | Medium |
| `internal/browser/` | browser.go | 1,508 | Easy (most self-contained) |
| `internal/apppromotion/` | app_promotion*.go | ~2,000 | Medium |
| `internal/vmctl/` | tools_vmctl.go | 2,842 | Medium |
| `internal/conductor/` | conductor routing in runtime.go | ~500 | Medium |
| `internal/super/` | super_controller.go, tools_coagent.go | ~1,500 | Medium |
| `internal/researcher/` | tools_research.go, tools_researcher.go | ~2,000 | Medium |
| `internal/podcast/` | podcast.go | ~500 | Easy |
| `internal/email/` | tools_email.go | ~800 | Easy |
| `internal/desktop/` | desktop.go | ~300 | Easy |
| `internal/content/` | content.go, content_extract.go | ~2,000 | Medium |

After extraction: `internal/runtime/` is deleted. ~40,000 lines move out,
~8,000 lines of core stay in `internal/agentcore/`.

### The tool loop as library

`toolloop.go` (1,521 lines) becomes a library in `internal/agentcore/`:

```go
type Deps struct {
    Store    StoreReader
    Provider provideriface.Provider
    Bus      *events.EventBus
    ActorRT  *actor.Runtime
}

type ToolLoop struct {
    deps   Deps
    tools  *toolregistry.ToolRegistry
    rec    *types.RunRecord
    memory []byte
}

func New(deps Deps, tools *toolregistry.ToolRegistry, rec *types.RunRecord) *ToolLoop
func (l *ToolLoop) Run(ctx context.Context) (state RunState, memory []byte, err error)
func (l *ToolLoop) Resume(ctx context.Context, update actor.Update, memory []byte) (RunState, []byte, error)
```

Appagents use it without referencing `*Runtime`:
```go
func (h *Handler) HandleUpdate(ctx context.Context, agentID string, u actor.Update, memory []byte) ([]byte, error) {
    rec := loadRun(h.deps.Store, u)
    loop := agentcore.New(h.deps, h.tools, rec)
    if u.Kind == "coagent_result" {
        return loop.Resume(ctx, u, memory)
    }
    return loop.Run(ctx)
}
```

### User-created appagents

A user app imports the public API surface and implements `actor.Handler`:

```go
// myapp/handler.go (user's code)
type MyAppHandler struct {
    deps  agentcore.Deps
    tools *toolregistry.ToolRegistry
}

func New(deps agentcore.Deps) *MyAppHandler {
    h := &MyAppHandler{deps: deps, tools: toolregistry.New()}
    h.tools.Register("my_tool", h.myTool)
    return h
}

func (h *MyAppHandler) HandleUpdate(ctx context.Context, agentID string, u actor.Update, memory []byte) ([]byte, error) {
    rec := loadRun(h.deps.Store, u)
    loop := agentcore.New(h.deps, h.tools, rec)
    if u.Kind == "coagent_result" {
        return loop.Resume(ctx, u, memory)
    }
    return loop.Run(ctx)
}
```

Public API surface for user apps:
- `pkg/agentcore` — tool loop, run lifecycle, deps
- `pkg/actor` — actor runtime, Handler interface, Update type
- `pkg/toolregistry` — tool registration
- `pkg/store` — persistence (or a narrower interface)
- `pkg/provideriface` — provider interface
- `pkg/events` — event bus

User apps never touch `internal/runtime/` because it no longer exists.

## App Registration API — Write-Only Extension

### Design rationale: Go + build-time registration + candidate promotion

Three approaches were considered for app extensibility:

1. **Embedded scripting (Lua/JS/WASM):** Hot-reload, but slower execution,
   sandboxing complexity, no compile-time safety, worse tooling, and a
   second runtime to maintain (interpreter, host bindings, memory
   management). Rejected.

2. **Go plugins (`plugin.Open`):** Dynamic loading without rebuild, but
   Linux-only, fragile ABI requirements, no cross-compilation, debugging
   nightmares. Worst of both worlds. Rejected.

3. **Go + build-time registration + candidate promotion (chosen):** Fast
   builds (Go compiles in seconds), full compile-time safety, full
   performance, existing promotion/rollback as the transaction, no
   interpreter, no ABI fragility. The "cost" is a build step, but Go
   builds are fast enough that the candidate → promote cycle is measured
   in seconds.

The transaction boundary is the binary. State lives in the store. Code
lives in the binary. Promotions swap the binary. Rollbacks swap it back.
The store persists across both. This gives transactional semantics without
the overhead of an embedded scripting language.

### The problem today

To add a new appagent today, you must edit **6 files**, **4 of which are
existing files in the god object**:

1. **`internal/agentprofile/agentprofile.go`** — add profile constant
2. **`internal/runtime/tool_profiles.go`** — add constant re-export, add
   `case` in `canonicalAgentProfile`, add `case` in `roleSpec`, add `case`
   in `agentProfileForRun`, add registry building + tool registration in
   `InstallDefaultAgentTools`, add to `toolProfiles` map (5 separate edits)
3. **`internal/runtime/prompt_store.go`** — add profile to prompt store list
4. **`cmd/sandbox/main.go`** — add to startup logging / health check
5. **New file: `internal/runtime/tools_podcast.go`** — tool implementations
6. **New file: `RegisterPodcastTools` function** — called from
   `InstallDefaultAgentTools`

This is exactly wrong. Adding a new app requires mutating the god object in
5 places. Every new app increases the surface area of `tool_profiles.go`,
which is already 700+ lines of switch statements and registry wiring. The
god object grows with every app.

### The design principle

**Adding a new app should require only writing new files. No existing file
should be mutated.** This is the open-closed principle applied to app
registration: open for extension (new files), closed for modification
(existing files don't change).

### The registration interface

```go
// pkg/appagent/appagent.go (public API)

package appagent

import (
    "context"
    "github.com/choir/agentcore"
    "github.com/choir/actor"
    "github.com/choir/toolregistry"
)

// App is the registration interface for an appagent. Each app package
// implements this and registers it in its package init() or via a
// Register function.
type App interface {
    // Profile returns the agent profile name (e.g., "texture", "wire",
    // "podcast"). This is the agentID prefix and the dispatch key.
    Profile() string

    // NewHandler creates a new actor.Handler for this app. The handler
    // receives shared deps and creates its own tool registry.
    NewHandler(deps agentcore.Deps) actor.Handler

    // RoleSpec returns the capabilities and delegation permissions for
    // this app's agents.
    RoleSpec() RoleSpec

    // PromptSpec returns the system prompt configuration for this app.
    // Optional — return nil to use a default prompt.
    PromptSpec() *PromptSpec
}

type RoleSpec struct {
    AllowReadOnlyFiles        bool
    AllowWritableFiles        bool
    AllowResearchTools        bool
    AllowEvidenceTools        bool
    AllowMemoryTools          bool
    AllowModelDiagnosticTools bool
    AllowCodingTools          bool
    AllowCoAgentTools         bool
    AllowedDelegateTargets    []string
}

type PromptSpec struct {
    SystemPrompt string
    Overlays     []string
}

// Registry is the global app registry. Apps register themselves at init
// time. The sandbox binary imports the app package, which triggers the
// init() registration. No central file needs editing.
type Registry struct {
    apps map[string]App
}

var defaultRegistry = &Registry{apps: make(map[string]App)}

func Register(app App) {
    defaultRegistry.apps[app.Profile()] = app
}

func Apps() map[string]App {
    return defaultRegistry.apps
}

func Get(profile string) (App, bool) {
    app, ok := defaultRegistry.apps[profile]
    return app, ok
}
```

### How an app registers itself

Each app package has an `init()` that registers:

```go
// internal/texture/app.go
package texture

import (
    "github.com/choir/appagent"
    "github.com/choir/agentcore"
    "github.com/choir/actor"
)

type app struct{}

func (app) Profile() string { return "texture" }

func (app) NewHandler(deps agentcore.Deps) actor.Handler {
    return NewHandler(deps)
}

func (app) RoleSpec() appagent.RoleSpec {
    return appagent.RoleSpec{
        AllowMemoryTools:       true,
        AllowCoAgentTools:      true,
        AllowedDelegateTargets: []string{"researcher"},
    }
}

func (app) PromptSpec() *appagent.PromptSpec {
    return &appagent.PromptSpec{
        SystemPrompt: textureSystemPrompt,
    }
}

func init() {
    appagent.Register(app{})
}
```

### How the sandbox wires it

```go
// cmd/sandbox/main.go

import (
    _ "github.com/choir/internal/texture"    // register via init()
    _ "github.com/choir/internal/wire"       // register via init()
    _ "github.com/choir/internal/browser"    // register via init()
    _ "github.com/choir/internal/podcast"    // register via init()
    _ "github.com/choir/internal/email"      // register via init()
    // ... etc
)

func main() {
    deps := agentcore.Deps{Store: store, Provider: provider, Bus: bus, ActorRT: actorRT}

    for profile, app := range appagent.Apps() {
        handler := app.NewHandler(deps)
        actorRT.RegisterHandler(profile, handler)

        roleSpec := app.RoleSpec()
        // ... configure tool permissions, delegation rules

        if promptSpec := app.PromptSpec(); promptSpec != nil {
            promptStore.Set(profile, promptSpec.SystemPrompt)
        }
    }

    actorRT.Start()
}
```

### What changed: adding a new app

**Before (6 files, 4 mutated):**
1. Edit `agentprofile/agentprofile.go` — add constant
2. Edit `tool_profiles.go` — 5 edits (canonical, roleSpec, agentProfileForRun, InstallDefaultAgentTools, toolProfiles map)
3. Edit `prompt_store.go` — add to list
4. Edit `cmd/sandbox/main.go` — add to logging
5. New: `tools_podcast.go`
6. New: `RegisterPodcastTools`

**After (new files only, 0 existing files mutated):**
1. New: `internal/apps/podcast/app.go` — implements `appagent.App`, registers in `init()`
2. New: `internal/apps/podcast/handler.go` — implements `actor.Handler`, registers tools

That's it. Two new files. Zero edits to existing files. The build system
auto-discovers the new package (see below). The backend app list endpoint
automatically includes the new app. The frontend auto-discovers the
component (see Frontend section below).

### Zero-edit discovery (backend)

Go requires a package to be imported for its `init()` to run. We solve this
with a build-time code generation step that scans for app packages and
generates the import file. This file is auto-generated, never edited by hand.

```go
// cmd/sandbox/app_imports.go — AUTO-GENERATED, DO NOT EDIT
// Generated by: go generate ./cmd/sandbox
//go:generate go run ./gen_app_imports.go

package main

import (
    _ "github.com/choir/internal/apps/browser"
    _ "github.com/choir/internal/apps/email"
    _ "github.com/choir/internal/apps/podcast"
    _ "github.com/choir/internal/apps/texture"
    _ "github.com/choir/internal/apps/wire"
    // ... etc
)
```

The generator (`cmd/sandbox/gen_app_imports.go`) scans `internal/apps/*/`
for directories containing `app.go` (or any file importing `appagent`) and
regenerates `app_imports.go`. Run via `go generate` before `go build`. Can
be wired into the Makefile / build script so it's automatic.

Adding a new app: create `internal/apps/myapp/app.go` + `handler.go`. Run
`go generate` (or just `go build` if the Makefile runs it). The import is
auto-added. No existing file is touched.

Alternative: if we don't want code generation, we can use a single
`internal/apps/apps.go` that blank-imports all app subpackages. Adding a
new app means creating a new directory under `internal/apps/` and adding
one line to `apps.go`. This is one edit to one file, but it's a trivial
import line, not logic. The code generation approach is cleaner (zero
edits) but adds a build step. The choice is a tradeoff between build
complexity and edit count.

### Choir-in-choir: apps creating and editing apps

For choir-in-choir (the system creating and editing apps), app changes go
through the **candidate VM promotion path**, not hot-reload. This is the
existing promotion/rollback machinery — the same path used for any
computer state change. The product is a persistent user computer;
candidate computers mutate; canonical state changes only by promotion.

The flow:

1. **Build a candidate:** A conductor or super agent writes a new app (or
   edits an existing one) by creating new files in `internal/apps/myapp/`
   and/or `frontend/src/lib/apps/MyApp.svelte`. This happens in a candidate
   VM — a fork of the user's computer. The candidate builds the new binary
   (`go generate` + `go build` + frontend build).

2. **Verify the candidate:** The candidate binary runs with the new app
   registered via `init()`. Verification agents test the app in the
   candidate environment — tools work, handler responds, frontend renders,
   state persists. Evidence is recorded.

3. **Promote the candidate:** Once verified, the candidate is promoted to
   the active computer. The new binary replaces the old one. The app's
   state in the store persists (same profile, same agentID prefix, same
   stored artifacts). The app's identity is unchanged; only the code changed.

4. **Rollback if needed:** If the promoted app has problems, the promotion
   is rolled back to the previous binary. The store state is preserved or
   rolled back per the existing rollback machinery.

There is no `ReplaceAtRuntime` hot-swap. There is no in-process handler
replacement. The app registration interface (`appagent.App` + `init()`) is
the build-time contract. Choir-in-choir app editing is a build-and-promote
cycle, not a live code swap. The candidate VM is the scratch space; the
active computer is the canonical state.

This means:
- **New apps:** Built in a candidate VM, verified, promoted. The `init()`
  registration makes them available as soon as the new binary runs.
- **Edited apps:** Same path. New handler code, new binary in candidate,
  verify, promote. State persists across the promotion because it's in the
  store, not in the binary.
- **Edited frontends:** New `.svelte` file in the candidate, frontend
  rebuild, verify in candidate, promote. The auto-discovery glob picks up
  the new component when the new binary serves the frontend.

The app registration interface is the boundary between the core system and
the apps that run on it. New apps extend the system without modifying it.
Existing apps evolve without modifying the core. The core is a fixed
substrate; apps evolve through candidate → verify → promote cycles.

### What the core does NOT know about apps

The core (`agentcore`, `actor`, `toolregistry`, `store`, `events`) knows
nothing about:
- Which apps exist
- What tools each app registers
- What system prompt each app uses
- What capabilities each app has
- How apps delegate to each other

The core provides:
- The tool loop (execution engine)
- The actor runtime (concurrency, messaging, park-resume)
- The tool registry (tool dispatch)
- The store (persistence)
- The event bus (events)

Apps provide everything else. The core is a fixed substrate; apps are
plugins. The substrate does not grow when apps are added.

## Frontend App Registration

### The problem today

The frontend has its own hardcoded app registry — `APP_REGISTRY` in
`frontend/src/lib/apps/registry.ts` is a static array of 18 apps. Each entry
specifies the app's id, name, icon, description, Svelte component (lazy-loaded),
launcher config, window geometry, auth policy, and theme.

The backend has no app list endpoint. The frontend does not know what apps
the backend supports. The two registries are completely independent, kept in
sync by nothing.

To add a new app today, the full file count is **8 files, 5 mutated**:

**Backend (6 files, 4 mutated):**
1. Edit `internal/agentprofile/agentprofile.go` — add constant
2. Edit `internal/runtime/tool_profiles.go` — 5 edits
3. Edit `internal/runtime/prompt_store.go` — add to list
4. Edit `cmd/sandbox/main.go` — add to logging
5. New: `tools_podcast.go`
6. New: `RegisterPodcastTools`

**Frontend (2 files, 1 mutated):**
7. Edit `frontend/src/lib/apps/registry.ts` — add entry to `APP_REGISTRY`
8. New: `frontend/src/lib/PodcastApp.svelte`

The two registries can drift — nothing prevents the frontend from listing an
app the backend doesn't support, or vice versa.

### The design principle

**The backend is the source of truth for which apps exist. The frontend
discovers apps from the backend.** Each app provides its frontend metadata
(icon, window geometry, surface kind, auth policy) through the `App`
interface. The backend exposes this via an API endpoint. The frontend
fetches it and dynamically loads the corresponding Svelte component.

### The app interface with frontend metadata

```go
// pkg/appagent/appagent.go

type App interface {
    Profile() string
    NewHandler(deps agentcore.Deps) actor.Handler
    RoleSpec() RoleSpec
    PromptSpec() *PromptSpec

    // FrontendMetadata returns the frontend app definition. Apps that
    // have no frontend component (internal-only agents) return nil.
    FrontendMetadata() *FrontendMetadata
}

type FrontendMetadata struct {
    Name        string            // "Podcast"
    Icon        string            // "📡"
    Description string            // "Podcast feed player"
    Surface     string            // "standard" | "document" | "media" | "terminal"
    Window      WindowGeometry    // width, height, minWidth, minHeight
    Launcher    LauncherConfig    // desk, desktopIcon, mobileSwitcher, order
    Auth        AuthPolicy        // preview policy, requiresAuthFor
    // Component is NOT here — the frontend maps app ID to a lazy-loaded
    // Svelte component. The backend doesn't know about Svelte.
}

type WindowGeometry struct {
    Width    int `json:"width,omitempty"`
    Height   int `json:"height,omitempty"`
    MinWidth int `json:"minWidth,omitempty"`
    MinHeight int `json:"minHeight,omitempty"`
}

type LauncherConfig struct {
    Desk         bool `json:"desk"`
    DesktopIcon  bool `json:"desktopIcon"`
    MobileSwitcher bool `json:"mobileSwitcher"`
    Order        int  `json:"order"`
}

type AuthPolicy struct {
    Preview       string   `json:"preview"`       // "public-preview" | "public-readonly" | "private"
    RequiresAuthFor []string `json:"requiresAuthFor"`
}
```

### The backend app list endpoint

```go
// GET /api/apps → list of app definitions with frontend metadata

func HandleAppList(w http.ResponseWriter, r *http.Request) {
    apps := appagent.Apps()
    var list []AppDefinitionResponse
    for _, app := range apps {
        fm := app.FrontendMetadata()
        if fm == nil {
            continue // internal-only agent, no frontend
        }
        list = append(list, AppDefinitionResponse{
            ID:          app.Profile(),
            Name:        fm.Name,
            Icon:        fm.Icon,
            Description: fm.Description,
            Surface:     fm.Surface,
            Window:      fm.Window,
            Launcher:    fm.Launcher,
            Auth:        fm.Auth,
        })
    }
    writeJSON(w, list)
}
```

### The frontend dynamic registry

The frontend uses Vite's `import.meta.glob` to auto-discover Svelte
components. No component map to edit. No registry to update. Drop a
`.svelte` file in `frontend/src/lib/apps/` and it's available.

```typescript
// frontend/src/lib/apps/registry.ts

import type { ComponentType } from 'svelte';

export type ChoirAppDefinition = {
  id: string;
  name: string;
  icon: string;
  description: string;
  component: () => Promise<{ default: ComponentType }>;
  launcher: { desk: boolean; desktopIcon: boolean; mobileSwitcher: boolean; order: number };
  window: { singleton: boolean; heavy: boolean; desktop?: WindowGeometry; compact?: WindowGeometry };
  auth: { preview: AppPreviewPolicy; requiresAuthFor: string[] };
  theme: { surface: AppSurfaceKind; shellDataAttr: string; contentClass: string };
};

// Auto-discover all .svelte files in this directory. Vite resolves this
// at build time. Each file is named by its path: apps/PodcastApp.svelte
// → id "podcast" (lowercase, strip "App.svelte" suffix).
const modules = import.meta.glob('./*App.svelte');

function componentIdFromPath(path: string): string {
  // './PodcastApp.svelte' → 'podcast'
  const name = path.replace(/^\.\//, '').replace(/App\.svelte$/, '');
  return name.toLowerCase();
}

const componentMap: Record<string, () => Promise<{ default: ComponentType }>> = {};
for (const [path, loader] of Object.entries(modules)) {
  componentMap[componentIdFromPath(path)] = loader as () => Promise<{ default: ComponentType }>;
}

// Fetch app definitions from the backend. The backend is the source of
// truth for which apps exist. The frontend maps app IDs to auto-discovered
// components. Apps without a component file are agent-only (no window).
let appRegistry: ChoirAppDefinition[] = [];

export async function fetchAppRegistry(): Promise<void> {
  const res = await fetch('/api/apps');
  const defs = await res.json();
  appRegistry = defs
    .filter((def: any) => componentMap[def.id]) // only apps with a component
    .map((def: any) => ({
      ...def,
      component: componentMap[def.id],
      theme: { surface: def.surface, shellDataAttr: `data-${def.id}-app`, contentClass: `${def.id}-content` },
    }));
}

export function getAppDefinition(appId: string): ChoirAppDefinition | null {
  return appRegistry.find((app) => app.id === appId) || null;
}

export function getRegisteredApps(): ChoirAppDefinition[] {
  return appRegistry;
}
```

### What changed: adding a new app (full stack)

**Before (8 files, 5 mutated):**
1. Edit `agentprofile/agentprofile.go`
2. Edit `tool_profiles.go` (5 edits)
3. Edit `prompt_store.go`
4. Edit `cmd/sandbox/main.go`
5. New: `tools_podcast.go`
6. New: `RegisterPodcastTools`
7. Edit `frontend/src/lib/apps/registry.ts`
8. New: `PodcastApp.svelte`

**After (new files only, 0 existing files mutated):**
1. New: `internal/apps/podcast/app.go` — implements `appagent.App` (including
   `FrontendMetadata()`), registers in `init()`
2. New: `internal/apps/podcast/handler.go` — implements `actor.Handler`,
   registers tools
3. New: `frontend/src/lib/apps/PodcastApp.svelte` — the Svelte component

**3 new files. 0 edits to existing files.** The backend auto-discovers the
app package via code generation. The frontend auto-discovers the component
via `import.meta.glob`. The backend app list endpoint automatically includes
the new app. The frontend fetches the list and maps the app ID to the
auto-discovered component.

### No generic fallback component

There is no `GenericApp.svelte`. Apps either have their own frontend
component or they don't have a frontend. An app without a component is still
a fully functional agent — it runs in the actor runtime, has tools, can
send and receive messages, can be invoked via the prompt bar or API. It
just doesn't get a desktop window.

This is intentional. New apps shouldn't be templated. "Anything goes" —
each app brings whatever frontend it wants, in whatever shape it wants. A
podcast app looks like a podcast player. A code editor looks like a code
editor. A mission dashboard looks like a mission dashboard. There is no
one-size-fits-all shell for app frontends.

Runtime-created apps (choir-in-choir) that don't have a frontend component
are agent-only. They're accessible via the prompt bar, the API, and other
apps' tools. If a runtime app wants a frontend, it can:

1. **Write its own component:** The app generates a `.svelte` file, drops it
   in the apps directory, and it's auto-discovered on next build. This
   requires a build step but gives full control.

2. **Inject HTML directly:** The app provides raw HTML/SVG/canvas through an
   API endpoint, and a thin host component renders it in an iframe or
   shadow DOM. No build step, full visual control, but no Svelte reactivity.

3. **Use an existing component as a canvas:** The app uses an existing
   artifact surface (Texture, or a custom canvas) and writes to it through
   the existing artifact APIs. The "frontend" is the artifact, not a custom
   component.

Option 3 is the most choir-native: the app produces artifacts through its
tools, and the existing Texture/artifact surfaces display them. No custom
frontend needed. The app is a producer; the existing surfaces are the
consumers.

### What the frontend does NOT hardcode

After this change, the frontend no longer hardcodes:
- Which apps exist (fetched from `/api/apps`)
- App names, icons, descriptions (from backend metadata)
- Window geometry (from backend metadata)
- Auth policies (from backend metadata)
- Launcher ordering (from backend metadata)
- The component map (auto-discovered via `import.meta.glob`)

The frontend only owns:
- The app host/surface shell (how apps are mounted, windowed, themed)
- The auto-discovery glob pattern (convention: `*App.svelte` in `apps/`)
- The prompt bar, desktop, and window manager (the shell, not the apps)

## Deletion Candidates

### Pattern: premature ambition that was never wired up

The continuation system is the exemplar. It's a deterministic attempt at
long-running choir-in-choir (auto-chaining runs via ContinuationProposal
with Objective, Reason, AuthorityProfile, LeaseSeconds) that was built
before the substrate existed to support it. In the actor model, continuation
is natural: an agent sends itself a message, passivates, re-activates on the
message. No ContinuationProposal needed.

**How to identify this pattern:**
1. A feature with elaborate structure (types, metadata fields, API endpoints)
2. No code path that populates the inputs the feature checks for
3. The feature is called but always returns early / no-ops
4. The feature tries to solve a problem the substrate now handles natively

#### DEAD: Continuation system (~800 lines)

| Item | File | Lines | Evidence |
|---|---|---|---|
| `continuation.go` | internal/runtime/continuation.go | 293 | `maybeStartConfiguredContinuation` checks for `continuation_objective` metadata — no code ever sets it. Called from runtime.go:1754, 1944 but always returns immediately. |
| `api_compaction_eval.go` | internal/runtime/api_compaction_eval.go | ~400 | Compaction recall eval harness — uses continuation to chain eval runs. Eval-only, not production. |
| Continuation API handlers | internal/runtime/api.go:651-760 | ~110 | `HandleRunContinuationsRoot`, `HandleRunContinuationDetail` — manual API for continuation. Only entry point that actually works. |
| Continuation metadata constants | internal/runtime/tool_profiles.go:39-43 | 5 | `runMetadataContObjective`, `runMetadataContReason`, etc. — never set by any tool or code path. |
| Continuation routes | internal/runtime/api.go:1742-1743 | 2 | `/api/continuations`, `/api/continuations/` |
| Compaction eval routes | internal/runtime/api.go:1748-1749 | 2 | `/api/evals/compaction-recall`, `/api/evals/compaction-recall/runs/` |

**Why delete:** The actor model's park-resume mechanism IS the continuation
mechanism. An agent that has more work to do sends itself a message via
`actor.Send()`, passivates, and re-activates on the message. No
ContinuationProposal, no LeaseSeconds, no AuthorityProfile. The deterministic
proposal-selection was premature — the substrate now handles it natively.

**Delete entirely.** The manual API endpoint can be rebuilt later if needed,
but the auto-continuation path and the compaction eval harness are dead.

### DEAD: Retired endpoint tombstones

| Item | File | Lines | Evidence |
|---|---|---|---|
| `texture_source_repairs.go` | internal/runtime/texture_source_repairs.go | 19 | Two HTTP handlers returning 410 Gone. Still wired in api.go:1903-1909. |
| Retired request types | internal/runtime/texture.go:125-142 | ~18 | `textureSourceGapRepairRequest`, `textureSourceArtifactAttachmentRequest`, `textureSourceArtifactAttachment` — types for the retired endpoints. |
| Retired route registrations | internal/runtime/api.go:1903-1909 | 7 | Routes for retired endpoints. |

**Delete entirely.** Remove the file, the types, the routes, and the tests
that reference the retired endpoints.

### DEAD: Disabled legacy tool

| Item | File | Lines | Evidence |
|---|---|---|---|
| `newLegacySynchronousDelegateWorkerVMTool` | internal/runtime/tools_vmctl.go:1464 | ~10 | Returns a tool with name `delegate_worker_vm_sync_legacy_disabled`. Description: "Disabled legacy synchronous worker delegation implementation retained only for rollback reference; do not register." |

**Delete.** Rollback reference is no longer needed.

### DEAD: Stub function

| Item | File | Lines | Evidence |
|---|---|---|---|
| `normalizeWireArticleRevisionForRead` | internal/runtime/universal_wire.go:1177 | 2 | Returns revision unchanged. Comment: "Source refs are no longer repaired from markdown/source-token syntax here." Called from texture.go but does nothing. |

**Delete.** Remove the function and its call site.

### DEAD: Test-only hook in production

| Item | File | Lines | Evidence |
|---|---|---|---|
| `wirePlatformPublisher` field | internal/runtime/runtime.go:56 | 1 | Function field on Runtime. Only set in tests (wire_publication_test.go:62, 179). No setter function. Never set in production. |

**Delete from production struct.** Move to a test-only injection mechanism
(interface or test helper).

### DEAD: Legacy reserved config setting

| Item | File | Lines | Evidence |
|---|---|---|---|
| `DefaultSupervisionInterval` | internal/runtime/config.go:37 | 2 | Comment: "legacy reserved setting kept only to avoid churning tests and config plumbing during the runtime cleanup." |

**Delete.** Update tests and config plumbing.

### DEAD: Stale comment about legacy goroutine fallback

| Item | File | Lines | Evidence |
|---|---|---|---|
| Comment at runtime.go:761 | internal/runtime/runtime.go:761 | 1 | Says "Dispatch via actor runtime (or legacy goroutine if no bridge)" but `activate()` panics if `dispatchActor` is nil. No fallback exists. |

**Delete the comment.** The code is correct; the comment is stale.

### DEAD: Trace app backend (H027 — Trace App Residue)

The Trace app was deleted from the frontend months ago. The frontend has
toast messages saying "Trace UI is unshipped." But the backend is still
serving 1,589 lines of trace API that nobody calls. This is heresy H027
from choir-doctrine.md.

| Item | File | Lines | Evidence |
|---|---|---|---|
| `api_trace.go` | internal/runtime/api_trace.go | 1,589 | 30+ functions for trace trajectory snapshots, moment details, agent nodes/edges, provenance summaries, log formatting. Zero frontend callers. |
| Trace routes | internal/runtime/api.go:1719-1720 | 2 | `/api/trace/trajectories`, `/api/trace/trajectories/` — still registered, nobody calls them. |
| `on:opentrace` handler | frontend/src/lib/AppHost.svelte:60 | 1 | Forwards opentrace event — dead, no trace app to open. |
| `handleOpenTraceFromContent` | frontend/src/lib/Desktop.svelte:1408 | ~3 | Shows toast: "Trace UI is unshipped; machine-readable evidence remains in logs." |
| `openTrace` in FeaturesApp | frontend/src/lib/FeaturesApp.svelte:427 | ~7 | Shows "Trace UI is unshipped. Evidence id: ..." |
| "Open Trace" button | frontend/src/lib/FeaturesApp.svelte:622 | 1 | Button that calls the dead `openTrace` function. |
| Legacy event handlers | internal/runtime/api.go:1407, 1577 | ~10 | `HandleEventList`, `HandleEvents` — comments say "Trace uses" these. Trace is gone. Evaluate if anything else uses them; if not, delete. |

**Delete entirely.** The trace evidence substrate (structured event records,
run bundles, acceptance records) stays in the store — it's machine-readable
evidence for zot, run acceptance, and operator diagnosis. The trace *API*
and *frontend handlers* are dead. Trace is not a surface. It's an evidence
substrate.

Per choir-doctrine.md H027: "keep trace evidence APIs, run bundles,
acceptance records, diagnosis artifacts, and machine-readable causal
ledgers; do not expose Trace as a normal desktop app."

**Nuance:** The doctrine says "keep trace evidence APIs." This needs
clarification. The evidence *records* stay (they're in the store). The
trace *API endpoints* that serve a deleted frontend are dead. If zot or
run acceptance needs to read trace evidence, they read it from the store
directly, not through the trace API endpoints. The API endpoints existed
to serve the trace frontend, which is gone.

**Action:** Delete `api_trace.go`, delete trace routes, delete frontend
`on:opentrace` handlers and "Open Trace" buttons. Verify that run
acceptance and zot read trace evidence from the store, not from the trace
API. If anything still calls the trace API, route it to the store directly.

### DEAD: Helper functions only used by dead paths

| Item | File | Lines | Evidence |
|---|---|---|---|
| `wireArticleStructuredNodeText` | internal/runtime/universal_wire.go:875 | ~10 | Only used by `wireArticleCollectVisibleStructuredSourceRefs` |
| `wireArticleCollectVisibleStructuredSourceRefs` | internal/runtime/universal_wire.go:858 | ~16 | Only used by `wireArticleVisibleStructuredSourceEntities` |
| `wireArticleVisibleStructuredSourceEntities` | internal/runtime/universal_wire.go:825 | ~32 | May not be actively used — trace call sites before deleting |

**Trace and delete if dead.**

## Legacy Compatibility Code — DELETE ALL

Per the deletion policy: we are pre-release. There are no users to break.
Every item below is a compatibility shim for a previous level of
understanding. Delete it all. No "evaluate for removal." No "document the
cutover criteria." Delete.

| Item | File | Lines | What it does | Action |
|---|---|---|---|---|
| Legacy `work_id` metadata | runtime.go:261-262 | 2 | Reads old `work_id` field as fallback for trajectory ID | Delete the fallback. Trajectory ID is always set now. |
| Legacy source entity IDs | texture.go:245, 260 | 2 | `LegacySourceEntityID` field in source entity types | Delete the field. Delete tests that reference it. |
| Legacy expanded block modes | tools_texture.go:1668-1669 | 2 | Maps old block modes to `expanded_ref` | Delete the mapping. Only `expanded_ref` exists now. |
| Legacy update coagent fields | tools_worker_update.go:786-824 | ~40 | `rejectLegacyUpdateCoagentFields` — rejects 9 old field names | Delete the function. The old fields are gone. If something sends them, it breaks — that's correct. |
| Legacy wire publication path | wire_publication.go:27-39 | ~13 | Runs without trajectory metadata get "legacy soft path" | Delete the soft path. All runs have trajectory metadata now. If a run doesn't, fail loudly. |
| Legacy terminal handoff | runtime.go:1684-1694 | ~10 | Fallback to old behavior when `actor_park_on_idle` not set | Delete the fallback. Actor parking is the only path. |
| Legacy synthesis function name | wire_synthesis.go:66 | — | `synthesizeUniversalWireSourceClusterTextureArticle` — name retained, body rewritten | Rename to current function. The name is a fossil. |
| Legacy API handlers | api.go:1407, 1576 | — | `HandleEventList`, `HandleEvents` — not browser-public, comments say "Trace uses" them | Trace is deleted. If nothing else uses these handlers, delete them. |
| Type alias re-exports | runtime/provider.go, config.go, tool_profiles.go | ~20 | `type Provider = provideriface.Provider` etc. | Delete after all callers import from `provideriface`/`agentprofile`/`toolregistry` directly. |
| `DefaultSupervisionInterval` | config.go:37 | 2 | "Legacy reserved setting kept only to avoid churning tests" | Delete. Update tests. |
| `wirePlatformPublisher` field | runtime.go:56 | 1 | Test-only hook in production struct | Delete from struct. Move to test injection. |

### Test-only exports (unexport)

| Item | File | Lines | Evidence |
|---|---|---|---|
| `SetHealth()` | runtime.go:1183 | ~10 | Only called from test files |
| `ToolRegistry()` | runtime.go:1289 | ~5 | Only called from channels_test.go. Production uses `ToolRegistryForProfile()` |
| `BrowserCapabilities()` | browser.go:513 | ~10 | Only called within browser.go itself |
| `notifyAgentSignal()` | runtime.go:3659 | ~15 | Only called from toolloop_test.go (Phase 2 deletes this anyway) |

**Unexport or move to test helpers.**

### Structural debt

| Item | File | Lines | Issue |
|---|---|---|---|
| Three layers of title canonicalization | texture.go:387-398 | ~12 | `APIHandler.canonicalizeAliasedTextureDocumentTitle`, `Runtime.canonicalizeAliasedTextureDocumentTitle`, standalone function. Consolidate to one. |
| Dual mutex in wire debouncer | wire_reconciler_debounce.go | — | `wirePublishDebouncer` has internal `mu` AND Runtime has `wirePublishDebounceMu`. Consolidate. |
| `runtimeRestartCommand` | runtime_refresh.go:11 | — | Package-level function variable executing `nix develop -c go run ./cmd/sandbox`. Executable logic at package scope. |
| Goroutine leak in app promotion | app_promotion.go:305-309 | ~5 | Untracked async goroutine with `context.Background()`. No WaitGroup, no cancellation. |
| WebSocket goroutine | live_ws.go:95-108 | ~14 | `done` channel created but never selected on. Potential leak if connection hangs. |
| Inconsistent error handling | throughout | 100+ instances | Mix of log-and-continue, return-error, panic. No consistent error boundary. |

### Temporary stability ceiling — DELETE or DEFINE

| Item | File | Lines | Issue |
|---|---|---|---|
| `maxToolLoopIterations` | toolloop.go:220-221 | — | "Temporary stability ceiling while worker leases, cancellation, compaction, and budget backpressure mature." No timeline for removal. |

**Action:** Per deletion policy: no "temporary" without a timeline. Either
delete the ceiling and rely on budget-governed limits, or document the
specific condition: "Delete when worker lease cancellation is wired through
actor.Evict and compaction budget is enforced by the tool loop's
`maxToolLoopBudget` parameter." If neither exists yet, keep the ceiling but
replace "temporary" with the specific condition. Do not leave "temporary"
in the codebase.

## Extraction Sequence

Each extraction produces a working system. Do not attempt parallel
extractions — each depends on the tool loop being decoupled first.

### Step 0: Delete dead code and legacy compatibility

**Do this before any extraction.** Moving dead code to a new package is
waste. Delete first, then extract only live code.

Delete:
- Continuation system (~800 lines) — dead, actor park-resume replaces it
- Trace app backend (~1,600 lines) — H027 heresy, frontend deleted months ago, backend serves nobody
- Trace frontend handlers (`on:opentrace`, "Open Trace" buttons, "Trace UI is unshipped" toasts)
- Retired endpoint tombstones + routes + types
- Disabled legacy tool (`newLegacySynchronousDelegateWorkerVMTool`)
- Stub function (`normalizeWireArticleRevisionForRead`)
- Test-only hook (`wirePlatformPublisher` field)
- Legacy reserved config (`DefaultSupervisionInterval`)
- Stale legacy goroutine comment
- All legacy compatibility code (see table above — 11 items)
- Type alias re-exports (after updating callers to import directly)
- Test-only exports (unexport `SetHealth`, `ToolRegistry`, `BrowserCapabilities`)
- Dead helper functions (trace and delete)
- Any other dead patterns found using the methods in "How to Find More"

Also fix:
- Goroutine leak in `app_promotion.go:305-309`
- WebSocket goroutine in `live_ws.go:95-108`
- Three layers of title canonicalization → consolidate to one
- Dual mutex in wire debouncer → consolidate
- `runtimeRestartCommand` package-level variable → inject or delete
- `maxToolLoopIterations` "temporary" → define specific condition or delete

**Postcondition:** `go build ./...` passes. `go test -race ./...` passes.
Line count of `internal/runtime/` non-test files is reduced by ~3,000-4,000
lines (continuation ~800, trace ~1,600, legacy compat ~500, other dead
code ~500-1,000). No "legacy", "temporary", "deprecated", "retained",
"fallback", or "stub" comments remain in non-test code. No "Trace UI" or
"Open Trace" references remain in frontend code.

### Step 1: Extract tool loop into `internal/agentcore/`

**Prerequisite for all other extractions.**

Move: `toolloop.go`, `run_memory.go`, `run_acceptance.go`,
`coagent_update_packet.go`, `toolloopvalidation_test.go` into
`internal/agentcore/`. (continuation.go is deleted in Step 0.)

Parameterize over `Deps` struct (Store, Provider, Bus, ActorRT). Remove all
references to `*Runtime`. The tool loop calls tools through the registry,
not through Runtime methods.

**Postcondition:** `internal/agentcore/` compiles. No references to
`internal/runtime/`. `go build ./...` passes. `go test -race ./...` passes.

### Step 2: Extract Browser (easiest)

Browser is the most self-contained: own state (sessions, CDP), own mutexes
(`browserOpMu`, `browserCDPMu`), minimal interaction with other apps.

Move: `browser.go` → `internal/browser/`. Browser tools become methods on
`browser.Handler`. Mutexes become handler state (or unnecessary if operations
are serialized through the actor mailbox).

**Postcondition:** `internal/browser/` compiles. Browser tools registered by
browser handler. `go build ./...` passes.

### Step 3: Extract Wire

Wire has own state (debouncer), own mutex (`wirePublishDebounceMu`),
interacts with Texture and store.

Move: `wire_*.go`, `tools_wire_processor.go` → `internal/wire/`. Wire tools
become methods on `wire.Handler`. Debouncer becomes handler state.

**Postcondition:** `internal/wire/` compiles. Wire tools registered by wire
handler. `go build ./...` passes.

### Step 4: Extract Texture (hardest, most entangled)

15+ files, interacts with Wire, Conductor, Super, Researcher.

Move: `texture*.go`, `tools_texture.go` → `internal/texture/`. `textureEditMu`
becomes unnecessary (Texture actor serializes through mailbox). `textureWakeMu`
becomes unnecessary (wakes via `actor.Send`).

**Postcondition:** `internal/texture/` compiles. Texture tools registered by
texture handler. `go build ./...` passes.

### Step 5: Extract remaining apps

Each moves to its own package, registers its tools, owns its state:
- `internal/apppromotion/` — app_promotion*.go
- `internal/vmctl/` — tools_vmctl.go
- `internal/conductor/` — conductor routing from runtime.go
- `internal/super/` — super_controller.go, tools_coagent.go
- `internal/researcher/` — tools_research.go, tools_researcher.go
- `internal/podcast/` — podcast.go
- `internal/email/` — tools_email.go
- `internal/desktop/` — desktop.go
- `internal/content/` — content.go, content_extract.go

### Step 6: Dissolve `*Runtime`

After all apps extracted, `*Runtime` has no methods. Delete it. Shared
dependencies passed directly to each handler constructor in
`cmd/sandbox/main.go`.

**Postcondition:** `internal/runtime/` does not exist. `go build ./...`
passes. `go test -race ./...` passes.

## How to Find More Dead Patterns

The continuation system was found by tracing metadata fields that nothing
sets. The legacy compatibility code was found by searching for "legacy" in
comments. Apply these methods to find more skeletons. **When you find them,
delete them.** Do not document them, do not add them to a tracking list,
do not leave them "for later." Delete.

1. **Trace metadata fields:** For each `metadataStringValue(rec.Metadata, X)`,
   search for who sets X. If nobody sets it, the code path that checks it is
   dead. Delete the check and the code path.

2. **Trace tool registrations:** For each tool in `InstallDefaultAgentTools`,
   trace whether the tool is ever called by any agent profile. Tools that
   are registered but never invoked are dead. Delete the registration and
   the tool function.

3. **Trace API endpoints:** For each route in api.go, trace whether anything
   calls it — frontend, external client, test. Routes that nothing exercises
   are dead. Delete the route and the handler.

4. **Trace run metadata keys:** The `runMetadata*` constants in
   `tool_profiles.go` define the metadata schema. For each key, find who
   writes it and who reads it. Keys that are only read (never written) are
   dead paths. Delete the key and the read path.

5. **Trace store methods:** For each `rt.store.*` call, check if the store
   method is used outside the runtime. Store methods that are only called
   from dead runtime paths are dead store surface. Delete the method.

6. **Search for markers:** Search for "temporary", "for now", "until", "TODO
   remove", "legacy", "deprecated", "old", "stub", "placeholder", "disabled",
   "retained", "fallback", "compat". Each is a candidate for deletion. Per
   deletion policy: delete unless there is a specific, documented reason to
   keep it.

7. **Trace goroutine spawns:** For each `go func` or `go rt.method`, check
   if the goroutine is tracked (WaitGroup, context cancellation, channel
   stop). Untracked goroutines are leaks. Fix or delete.

8. **Trace type aliases:** For each `type X = Y.Z` re-export, check if
   callers still import from the re-export or from the original package.
   Re-exports with no external callers are dead. Delete.

## Mission Sequencing

This is not one mission. It's a sequence:

1. **3c_2** (current) — actor runtime migration, Phase 2 deletion of old
   concurrency. Prerequisite for all extraction.
2. **Runtime cleanup** (Step 0) — delete all dead code and legacy
   compatibility. Hard cutover, no legacy. ~1,500-2,000 lines deleted. One
   mission, mostly mechanical deletion.
3. **agentcore extraction** (Step 1) — extract tool loop into
   `internal/agentcore/`. Prerequisite for app extraction.
4. **Browser extraction** (Step 2) — easiest app, proves the pattern.
5. **Wire extraction** (Step 3) — second app, validates the pattern.
6. **Texture extraction** (Step 4) — hardest app, most entangled.
7. **Remaining app extractions** (Step 5) — apppromotion, vmctl, conductor,
   super, researcher, podcast, email, desktop, content.
8. **Runtime dissolution** (Step 6) — delete `internal/runtime/`, update
   `cmd/sandbox/main.go` to wire handlers directly.

Steps 4-7 can overlap once the pattern is proven. Step 8 is the final
cleanup after all apps are out.

Each step is a separate Parallax mission with its own paradoc. The
extraction pattern (delete dead code → extract live code → register tools →
own state → verify) is the same for each app; the difficulty varies with
entanglement.
