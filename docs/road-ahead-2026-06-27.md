# The Road Ahead

**Date:** 2026-06-27
**Status:** living document — sequencing and open loops
**Purpose:** provide a single view of all open work, the critical path, and
the sequencing logic that determines what to do next

## The Critical Insight: Sequencing Is Everything

Choir-in-choir working is a force multiplier. Once the system can create
and edit apps through candidate → verify → promote, dev throughput
increases dramatically:

- **Concurrency:** Multiple agents work on different apps/features in
  parallel candidate VMs. No more serial mission execution.
- **Rapid UX feedback:** An agent edits a Svelte component, builds the
  candidate, opens it in a browser, screenshots it, iterates. Minutes,
  not hours.
- **Supervisable long-running work:** Long missions run in candidate VMs
  with trace evidence and verification. The supervisor checks progress,
  not step-by-step execution.

This means: **the runtime refactor is not just technical debt cleanup. It
is the prerequisite for the force multiplier.** Every day the god object
persists is a day of serial execution. Every day the app registration API
doesn't exist is a day where adding a new app requires mutating 6 files.

The sequencing logic is simple:
1. Get the runtime refactored (3c_2 + extraction)
2. Get choir-in-choir working (app creation/editing via candidate promotion)
3. Use choir-in-choir to parallelize everything else

Steps 1 and 2 are serial. Step 3 is where the force multiplier kicks in.

## The Critical Path

```
3c_2 (current) — actor runtime migration, Phase 2
    ↓
Runtime cleanup — delete dead code, legacy compat, trace backend (~3-4K lines)
    ↓
agentcore extraction — tool loop into internal/agentcore/
    ↓
App registration API — appagent.App, init() registration, /api/apps endpoint
    ↓
Texture extraction — first app, hardest, most entangled (Wire depends on it)
    ↓
Wire extraction — Universal Wire as first verifier of the refactor
    ↓                   (proves the architecture end-to-end with a real production feature)
Remaining app extraction — browser, apppromotion, vmctl, etc.
    ↓
Runtime dissolution — delete internal/runtime/
    ↓
Mutation transaction hardening — fix appchange/promotion bugs
    ↓                   (complete capture, rollback refs, verifier evidence)
Choir-in-choir — candidate VM app creation/editing/promotion
    ↓
FORCE MULTIPLIER ACTIVATED
    ↓
Everything else in parallel
```

## Open Loops

### 1. Runtime Refactor (critical path)

**Status:** 3c_2 Phase 2 pending review. Extraction plan documented in
[docs/runtime-deletion-and-extraction-plan-2026-06-27.md](runtime-deletion-and-extraction-plan-2026-06-27.md).

**Scope:**
- 3c_2: Delete old concurrency, actor runtime as sole execution substrate
- Runtime cleanup: Delete ~3-4K lines of dead code (continuation, trace backend, legacy compat)
- agentcore extraction: Tool loop as library
- App registration API: `appagent.App` interface, init() registration, zero-edit discovery
- Texture extraction: First app extracted. Hardest, most entangled. Wire
  depends on Texture (publishes to Texture documents), so Texture must be
  its own package before Wire can be extracted.
- Wire extraction: **First verifier of the refactor.** Universal Wire is a
  real production feature with real complexity (synthesis, publication,
  trajectory bookkeeping, debouncer). If Wire works end-to-end on the new
  architecture, the refactor is proven. Browser is too simple to prove
  anything.
- Remaining app extraction: browser, apppromotion, vmctl, etc.
- Runtime dissolution: Delete `internal/runtime/`

**Why Texture first, Wire second:** Texture is the dependency root — Wire
publishes to Texture, and other apps (Conductor, Super, Researcher) interact
with Texture. Texture must exist as its own package before anything that
depends on it can be extracted. Wire is the verifier because it exercises
the full pipeline — source intake, synthesis, LLM calls, publication,
platform APIs, trajectory bookkeeping, inter-agent messaging. If Wire runs
correctly on `agentcore` + `appagent.App` + actor runtime + extracted
Texture, the architecture is sound.

**Blocking:** Everything in the critical path. This is the prerequisite for
choir-in-choir.

### 2. Production Readiness

**Status:** Comprehensive checklist at
[docs/production-readiness-checklist.md](production-readiness-checklist.md).
1085 lines, P0-P3 prioritized.

**P0 items (must do before real users):**
- Privacy policy + ToS — legal requirement
- LLM cost tracking — primary variable cost
- Snapshot save race window — correctness bug
- Old runtime removal — same as runtime refactor
- Race detector in CI — defense against the bug class that borked the port

**P1 items:**
- PII retraction pipeline
- Health checks + circuit breakers
- PR-based workflow
- Rate limiting
- Data retention policy
- Bounded inbox with backpressure
- Backpressure on Send
- Actor failure observability
- Graceful shutdown drain

**Sequencing:** P0 items that overlap with the runtime refactor (old runtime
removal, race detector, snapshot race window) should be done as part of the
refactor. Legal items (privacy policy, ToS) can be done in parallel. P1
items mostly depend on the actor runtime being in place.

### 3. Auth System

**Status:** WebAuthn/passkey only. 8,608 lines across `internal/auth/` and
`cmd/auth/`. No password reset, no account recovery, no alternative auth
methods.

**Current routes:**
- `/auth/register/begin`, `/auth/register/finish` — passkey registration
- `/auth/login/begin`, `/auth/login/finish` — passkey login
- `/auth/session` — session check
- `/auth/logout` — logout
- `/auth/desktop/exchange`, `/auth/desktop/exchange-redirect`, `/auth/desktop/redeem` — desktop auth bridge

**Problems:**
- **No reset/recovery:** If a user loses their passkey device, they lose
  their account. No email-based recovery, no backup codes, no admin reset.
- **Passkey-only:** Some users don't have passkey-capable devices. Need at
  least one alternative (email magic link, TOTP, or password as fallback).
- **No multi-device management:** Can't add a second passkey, can't remove
  a lost passkey, can't view registered devices.

**What's needed:**
- Account recovery flow (email-based magic link as fallback)
- Multi-device passkey management (add/remove/list credentials)
- Session management (view active sessions, revoke)
- Rate limiting on auth endpoints (overlaps with production readiness P1)

**Sequencing:** Not on the critical path. Can be done after choir-in-choir
works. But must be done before real users (overlaps with production
readiness P0 for legal/privacy).

### 4. Wails Desktop

**Status:** Phase 1 implemented (Wails v3 desktop shell). Phase 2 and 7
partially implemented (local service stack). Phases 3-6 spec'd.

**Spec:** [docs/archive/spec-choir-desktop-wails-v3-2026-06-22.md](archive/spec-choir-desktop-wails-v3-2026-06-22.md)

**Phases:**
- Phase 1: Wails v3 desktop shell — done
- Phase 2: Local Go services (native menus, tray, file access, notifications) — partial
- Phase 3: Base reconciliation kernel (pure Go) — spec'd
- Phase 4: Base sync wired into desktop — spec'd
- Phase 5: macOS File Provider extension — spec'd
- Phase 6: Host-to-VM folder sync — spec'd
- Phase 7: Workstation-native Choir (local VMs) — partial

**Sequencing:** Phase 2 can proceed in parallel with the runtime refactor.
Phases 3-7 depend on the runtime refactor and choir-in-choir (local VMs
need the app registration API to work).

### 5. UI/UX Improvements

**Status:** Multiple apps need work. No formal tracking.

**Known issues:**
- **Slides app:** 810 lines, basic PPTX/PDF/HTML player. Needs: proper
  slide rendering, transitions, presenter mode, editing. PPTX prototype
  exists in a worktree (see below).
- **Email app:** Had freeze issues (diagnosed in worktree). Bootstrap
  hardening done but app is still a silo state machine, not a view over
  mail objects.
- **Desktop/window manager:** Basic. Needs: better window tiling, mobile
  gestures, keyboard shortcuts, accessibility.
- **Error pages:** No user-facing error pages for 500s, 404s, maintenance.
- **Article quality feedback:** No way for users to report bad articles.

**Sequencing:** After choir-in-choir. The force multiplier makes UX work
fast — an agent can edit a Svelte component, build, screenshot, iterate in
minutes. Before choir-in-choir, UX work is serial and slow.

### 5b. Browser / Web Lens — needs rethinking

**Status:** Languishing. 1,508 lines backend (`browser.go`), 1,303 lines
frontend (`BrowserApp.svelte`). Neither the iframe approach nor the Obscura
approach worked well. The doctrine already flags this as heresy H029.

**History:**
1. **iframe approach** — first attempt. Most sites block iframe embedding
   (X-Frame-Options, CSP). Cross-origin policy means you can't read iframe
   content. The frontend literally has error text: "This site may block
   embedding. Try a Web Lens snapshot for readable text, links, source,
   and import."
2. **Obscura approach** — second attempt. CLI tool that fetches a URL and
   dumps text/HTML. Optional CDP screenshots. Works for snapshots but
   it's not a browser — no interactive navigation, no JavaScript rendering
   for SPAs, no login flows. The backend capabilities response defaults to
   `Available: false, Status: "not_configured"` because Obscura isn't
   configured in most environments.
3. **Current state** — the app exists in the registry (`id: 'browser'`,
   name: 'Web Lens'), routes are registered (`/api/browser/capabilities`,
   `/api/browser/sessions/`), but it's a Frankenstein of iframe fallback +
   Obscura snapshots + source entity display. Nobody knows what it's for.

**Doctrine (I13, H029):**
- I13: "Manual Browser is replaced in the source path by Source
  Viewer/reader artifacts plus explicit Web Lens live/original inspection."
- H029: "Browser is presented as the source-gathering app or default
  source reader for web material." This is a heresy. The successor pattern
  is: "Texture source marker → inline/transcluded expansion → Source
  Viewer/reader window → explicit Web Lens live/original inspection when
  needed."

**What it should be:**
The Browser/Web Lens app has one clear job today: **when a user hits "Open
original" in Source Viewer, it should open in the Browser app, not in a
new browser tab.** Currently, `ContentViewer.svelte:136` renders `<a
href={browserSourceUrl} target="_blank">` — it opens the URL in a new
browser tab, leaving Choir entirely. The link should dispatch a
`launchapp` event that opens BrowserApp with the URL as context, keeping
the user inside Choir.

Beyond that, Web Lens should be the **explicit live/original inspection**
surface — not the default source reader (that's Source Viewer/reader
artifacts), not a source-gathering tool (that's the agent pipeline), but
the place you go when you need to see the actual live web page. This is a
narrow, intentional use case, not a general browser.

**What needs to happen:**
1. **Fix "Open original"** — change ContentViewer to dispatch `launchapp`
   with `appId: 'browser'` and `appContext: { initialUrl: sourceUrl }`
   instead of `<a target="_blank">`. This is a small, concrete fix.
2. **Decide on the substrate** — iframe (limited but simple), Obscura
   (works for snapshots, not interactive), CDP/Playwright (full browser,
   heavy), or a new approach. The substrate choice determines what Web
   Lens can do.
3. **Rename and reframe** — stop calling it "Browser" or "Web Lens" in
   ways that suggest it's a general browser or source gatherer. It's the
   live/original inspection surface. The doctrine says implementation
   names may remain as transitional, but the product framing must change.
4. **Extract from runtime** — `browser.go` (1,508 lines) moves to
   `internal/browser/` or `internal/weblens/` as part of the app
   extraction. The backend browser session machinery (CDP, snapshots,
   Obscura integration) becomes the app's internal implementation.

**Sequencing:** The "Open original" fix can happen now (small frontend
change). The substrate decision and extraction happen after the runtime
refactor. Web Lens is not on the critical path — it's a UX improvement
that benefits from choir-in-choir.

### 6. Mission Graph Open Loops

**Status:** 87 nodes in mission graph. 40 settled, 27 open_handoff, 9
planned, 4 working, 2 superseded.

**27 open_handoff missions** — these are the accumulated open loops. Many
are historical (MissionGradient documents from before Parallax). Some are
actively relevant:

Key open_handoff missions to triage:
- Texture Hard Cutover v0 — texture cutover, may overlap with extraction
- M5 Wire On Settlement — wire pipeline, may overlap with extraction
- Doc Truth, Drift CI, And Context Packet v0 — docs checker, overlaps with worktree
- Surface Ontology Cleanup H027-H029 — trace/terminal cleanup, overlaps with deletion plan
- Super Console Real Zot Cutover — zot, the debugging surface
- Source System missions (multiple) — sourcecycled, source intake
- Wire Autonomous Ingestion — wire pipeline
- Texture source entities/transclusion/proposal cleanup — texture features

**Sequencing:** Triage after the runtime refactor. Many of these missions
will be absorbed into the extraction work or become trivial once choir-in-choir
works. The mission graph needs a cleanup pass to mark superseded missions
and consolidate overlapping ones.

### 7. Conceptual Refactor: Object Graph

**Status:** Design document at
[docs/report-conceptual-refactor-2026-06-23.md](archive/report-conceptual-refactor-2026-06-23.md).
Object graph prototype in worktree.

**Vision:** Choir is a persistent object graph with a frame-based
computation model. Every surface (mail, calendar, news, slides, desktop) is
a view over the graph. The graph is the product. Everything else is
rendering.

**Relationship to runtime refactor:** The object graph is the data model;
the actor runtime is the execution model. They're orthogonal. The object
graph can be built after the runtime refactor, as the shared data layer
that replaces per-app silo state. The app extraction naturally leads to
this — each app owns its state, and the object graph is the shared
persistence layer.

**Sequencing:** After the runtime refactor and app extraction. The object
graph is the natural successor to the god object's shared state.

## Worktrees From 3-5 Days Ago

Four Windsurf/cascade worktrees from ~2026-06-23, preserved with "preserve
O0" commits. These are pre-codex-universal-wire-flurry work that may still
be valuable.

### Docs Checker Cleanup (go-choir-6b7967c1)

**Changes:** 15 files, 1565 insertions, 115 deletions
- `doc-authority-manifest.yaml` — 1444 lines added (massive expansion)
- `choir-doctrine.md` — 92 lines changed
- Multiple docs updated for retired vocabulary
- Two test files fixed (`platform_publish_test.go`, `model_policy_test.go`)

**Assessment:** This is the docs truth checker cleanup. It clears the 975
warnings from retired vocabulary. The doc-authority-manifest expansion is
the machine-readable doc authority system.

**Disposition:** **Merge candidate.** This is green/yellow mutation class
(docs + test fixes). Should merge cleanly. Check for conflicts with recent
doctrine changes (3c_2 added circuit breaker to mission doc, but that's a
different file).

### ObjectGraph Prototype (go-choir-29131320)

**Changes:** 10 files, 1947 insertions
- `internal/objectgraph/` — full service with dolt/sqlite/memory stores
- Service, registry, object, blob_store, dolt_store, sqlite_store, memory_store
- 633 lines of tests

**Assessment:** Prototype of the object graph from the conceptual refactor.
Not wired into the runtime. Standalone package with tests.

**Disposition:** **Hold.** This is the data model for after the runtime
refactor. Don't merge yet — it will need to be integrated with the
extracted app packages, not the god object. But preserve the worktree.

### Qdrant Indexing Pipeline (go-choir-87c664e7)

**Changes:** 11 files, 1114 insertions
- `internal/qdrant/` — client, pipeline, schema, embed, samples
- `cmd/qdrantctl/main.go` — CLI tool
- `docker-compose.qdrant.yml` — local Qdrant
- Design doc and paradoc

**Assessment:** Prototype of the Qdrant semantic dedup pipeline. Standalone
package with tests.

**Disposition:** **Hold.** Same as object graph — this is infrastructure
for after the runtime refactor. The runtime already has qdrant_dedup.go;
this prototype is the standalone version. Preserve the worktree.

### PPTX Renderer Prototype (go-choir-f4fdeb09)

**Changes:** 14 files, 751 insertions
- `pptx-prototype/` — standalone Vite + Svelte app
- `SlidesApp.svelte` — 159 lines, PPTX rendering component
- `pptxGenerator.js` — 91 lines, PPTX generation
- `mockDeck.js` — 173 lines, mock data
- Design doc and paradoc

**Assessment:** Prototype PPTX renderer for the Slides app. Standalone
prototype, not integrated into the frontend.

**Disposition:** **Evaluate for integration.** The Slides app (810 lines)
needs work. This prototype has a PPTX rendering approach. After choir-in-choir
works, an agent can integrate this into the real Slides app quickly.

### Email Freeze Diagnosis (codex worktree, may be gone)

**Status:** Was reviewed in worktree-review-2026-06-23.md. 4 commits on
`diagnose/email-freeze` branch. Diagnosis doc + EmailApp.svelte hardening
+ Playwright spec.

**Disposition:** **Check if branch still exists.** If so, evaluate whether
the EmailApp hardening is still needed or if it was superseded by later
work.

## Sequencing

### Phase 1: Runtime Refactor (serial, critical path)

1. **3c_2 Phase 2** — finish old concurrency deletion (pending review)
2. **Runtime cleanup** — delete ~3-4K lines of dead code
3. **agentcore extraction** — tool loop as library
4. **App extraction** — browser → wire → texture → remaining
5. **App registration API** — `appagent.App`, init(), `/api/apps`
6. **Runtime dissolution** — delete `internal/runtime/`

**Estimated effort:** 3-5 missions. Each produces a working system.

### Phase 2: Mutation Transaction Hardening (serial, depends on Phase 1)

Before choir-in-choir can create and edit apps, the mutation transaction
system (appchange/promotion) must be trustworthy. The existing system has
known bugs from the conceptual refactor report:

- Promotions happened without complete capture
- Rollback refs were missing
- Verifier evidence was not always attached

**Scope:**
- **Complete capture:** Every promotion must capture the full delta —
  binary, frontend assets, schema changes, config changes. No partial
  captures.
- **Rollback refs:** Every promotion must have a valid rollback ref to the
  previous state. Missing rollback refs = no rollback path = unsafe
  promotion.
- **Verifier evidence:** Every promotion must carry verifier evidence
  (tests passed, acceptance records, trace evidence). No evidence =
  no promotion.
- **Transaction semantics:** Promotion is atomic — either the full delta
  is applied or none of it is. No half-promoted state.
- **Freshness checks:** Already partially implemented
  (`app_promotion_freshness_test.go`). Ensure the candidate is fresh
  relative to the active computer before promotion.

**Current system:** ~3,790 lines across `app_promotion.go`,
`app_promotion_build.go`, `api_app_promotion.go`,
`internal/proxy/app_change_packages.go`, `internal/types/app_promotion.go`.
Plus 1,077 lines of tests.

**Estimated effort:** 1 mission. The system exists; it needs hardening,
not rebuilding. The bugs are known and documented.

### Phase 3: Choir-in-Choir (serial, depends on Phase 2)

1. **Candidate VM app building** — agent writes new app files, candidate
   builds the binary (`go generate` + `go build` + frontend build)
2. **App verification in candidate** — verification agents test the app
   in the candidate environment, record evidence
3. **App promotion** — candidate → active, using the hardened mutation
   transaction. Rollback available if needed.
4. **Frontend auto-discovery** — `import.meta.glob` picks up new components
   when the new binary serves the frontend

**Estimated effort:** 1-2 missions. The promotion machinery exists (after
hardening); this is wiring app creation into it. The app registration API
from Phase 1 makes new apps automatically discoverable.

### Phase 4: Force Multiplier (parallel, depends on Phase 3)

Once choir-in-choir works, everything else can be parallelized:

- **Auth system redo** — recovery flow, multi-device, session management
- **UI/UX improvements** — slides, email, desktop, error pages
- **Production readiness P0-P1** — privacy policy, ToS, LLM cost tracking, health checks, circuit breakers, rate limiting
- **Wails desktop Phase 2-7** — local services, file provider, sync
- **Object graph** — shared data model for apps
- **Qdrant pipeline** — semantic dedup
- **Mission graph cleanup** — triage 27 open_handoff missions
- **Surface ontology cleanup** — H027-H029 (trace, terminal deletion)

Each of these can be a parallel candidate VM mission. The supervisor
checks progress; the agents do the work.

### What Can Start Now (parallel with Phase 1)

- **Docs checker cleanup merge** — green/yellow, no runtime dependency
- **Privacy policy + ToS** — legal, no code dependency
- **LLM cost tracking** — if it's a store/provider change, not a runtime change
- **Race detector in CI** — CI config, no runtime dependency
- **Mission graph triage** — docs work, no code dependency

## The One Thing That Matters Most

The runtime refactor is the bottleneck. Everything else is either
parallelizable (legal, CI, docs) or blocked (choir-in-choir, UX, auth,
object graph). The faster the runtime refactor completes, the faster the
force multiplier activates, and the faster everything else gets done.

Do not start new feature work in `internal/runtime/`. Every new feature
added to the god object is more code that has to be extracted later. If a
feature is needed, build it as a standalone package that will become an
extracted app. The app registration API design tells you how: implement
`appagent.App`, register in `init()`, own your tools and state.
