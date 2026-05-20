# MissionGradient: Computer Live Sync Hard Cutover v0

**Status:** ready for execution
**Date:** 2026-05-20
**Operator:** Codex-operated MissionGradient mission with staging Playwright,
git, CI, deploy, product APIs, Trace/VText/run-acceptance evidence, and
root-cause investigation discipline
**State ledger:** [platform-os-app-state.md](platform-os-app-state.md)
**Computer ontology:** [computer-ontology.md](computer-ontology.md)
**Starting platform baseline:** `3b5890a3660611b98821e8c8214cd04f6bc37bc5`

## One-Line Goal String

```text
/goal Run docs/mission-computer-live-sync-hard-cutover-v0.md as a Codex-operated MissionGradient mission: make Choir's user computer state live by default. Implement all three passes of the Computer Live Bus hard cutover: turn /api/ws into the authenticated user-computer notification fabric, move synced media progress/recents and desktop/app state change notifications onto durable Dolt-backed product APIs, and remove manual Refresh/Reload synchronization UI entirely from product apps that should update live. Preserve Dolt/product APIs as canonical state; websockets/SSE are notification and catch-up channels, not volatile truth. Live-sync Compute Monitor, candidate/promotion queues, VText recent/head updates, Podcast/content library, Files/content changes, desktop/window state, media progress/recents, Trace trajectories, and theme changes where applicable. No backwards compatibility shims, no localStorage compatibility for synced state, no hidden debug refresh controls, no test/internal route shortcuts, no browser-public host/global telemetry, and no summaries that call polling or stale manual reload success. Land platform changes through git/CI/deploy, verify staging identity, and prove on staging with desktop and 390x844 Playwright sessions across two browser contexts/devices where possible, screenshots/DOM metrics, websocket/SSE reconnect/catch-up evidence, rollback refs, residual risks, and the next realism axis. If the stopping condition is not reached, report checkpoint_incomplete or blocked_incomplete, update this mission doc with a resumable checkpoint, and continue/redirect/delegate any safe executable next probe inside current authority before stopping.
```

## Mission Frame

Choir is becoming a persistent multi-device user computer. That means ordinary
app state cannot depend on reload buttons, browser-local storage, or manual
operator refresh habits. If a document changes, a file appears, a candidate
exports, media playback advances, or a second device opens the same computer,
the visible product should converge automatically from canonical computer
state.

This mission is a hard cutover from "apps fetch when opened and expose Refresh
when stale" to "the computer publishes durable state changes and every relevant
surface updates live." It is not merely a websocket feature. The artifact is a
durable live-state substrate:

```text
mutation through product API
-> Dolt/store commit or typed durable record
-> owner/computer-scoped event envelope
-> websocket or existing SSE notification
-> client store updates or refetches product projection
-> visible app state converges without manual Refresh UI
-> reconnect/catch-up repairs missed notifications
```

The hard cutover matters. Keeping the old manual refresh path as a visible
compatibility feature will hide stale-state bugs and teach the wrong product
model. Manual data refresh controls are defects after this mission. Browser
navigation reload remains a browser-app control only; it must not be used as a
Choir state-sync acceptance path.

## Real Artifact

The artifact is the deployed Computer Live Bus and app integration:

```text
authenticated /api/ws or equivalent user-computer live transport
+ existing document/trajectory SSE where it is already the right narrow stream
+ durable Dolt-backed state APIs
+ product event vocabulary and catch-up semantics
+ frontend live stores
+ app-specific hard cutover away from manual Refresh UI
+ staging proof across desktop and 390x844 mobile contexts
```

The artifact is not:

- an echo websocket that only reports connected status;
- a polling loop with a "live" label;
- a manual Refresh button renamed to Sync, Retry, Update, or Check;
- localStorage/sessionStorage state with a later server sync promise;
- an internal/test route that seeds success records;
- a fake event stream disconnected from durable mutations;
- a broad host telemetry dashboard;
- a compatibility mode preserving the stale product model.

## Invariants

- The product object is a persistent user computer. Live sync is scoped to the
  authenticated owner and selected `desktop_id`.
- Dolt-backed product state remains canonical. Websocket/SSE notifications
  wake clients and carry sequence/revision/object references; they do not
  become the only copy of state.
- Every live state mutation must flow through the same product authority path
  that ordinary app actions use. No browser-public internal/test mutation path
  may create proof.
- Events are owner-scoped and, where relevant, computer/desktop scoped. No
  emails, raw user ids, VM ids, host paths, secrets, provider credentials,
  private prompts, or global system stats are leaked into browser events.
- Reconnect and missed-event recovery are first-class. A client that sleeps,
  changes network, or resumes on mobile must catch up from durable state.
- Existing VText and Trace SSE streams may remain if they preserve the same
  event/revision semantics. The mission does not require replacing good
  document/trajectory streams with a single monolithic websocket.
- Manual refresh/reload controls for product data synchronization are removed,
  not hidden behind compatibility flags. Tests should catch their reintroduction
  in covered apps.
- localStorage compatibility for synced media progress, synced media recents,
  desktop state, and theme state is removed. Short-lived UI-only local state is
  allowed only when it is explicitly not cross-device computer state.
- Active user work is preserved. Live updates must not stomp local editing,
  local media seeking, or unsaved foreground actions without conflict handling.
- Platform behavior changes land through git, CI, deploy, staging identity, and
  deployed product-path proof.

## Value Criterion

Minimize:

```text
visible stale product state
+ manual refresh/reload affordances used as sync crutches
+ browser-local divergence between devices
+ hidden polling loops
+ event drops with no catch-up
+ duplicated state stores
+ cross-owner or host telemetry exposure
+ product proof that depends on internal routes
+ fake "live" labels over stale snapshots
+ regressions to Trace/VText/media/desktop usability
```

subject to the invariants above.

The mission moves uphill when two browser contexts for the same user computer
can observe and mutate desktop/app/media/content state, then converge without
manual refresh UI, without localStorage compatibility, and without losing active
foreground work.

## Quality Gradient

Target quality: **solid** substrate and **excellent** product honesty.

Solid means:

- the event envelope is typed, versioned, owner/computer scoped, and tested;
- `/api/ws` is no longer an echo-only shell channel;
- product mutations publish events after durable state is written;
- clients reconnect with backoff and recover by sequence/revision or refetch;
- apps subscribe through a small shared frontend live-store layer, not
  bespoke socket code in every component;
- manual sync buttons are removed from covered apps, with tests preventing
  their return;
- localStorage is removed for synced state, with Dolt-backed replacements;
- staging proof uses real product APIs and visible app surfaces.

Substandard work:

- adding a websocket while leaving polling/Refresh as the real behavior;
- keeping localStorage reads as fallback for synced state;
- publishing events before durable writes;
- letting websocket messages carry private host/system details;
- using broad "reload all" commands instead of object-scoped updates;
- deleting buttons cosmetically while stale data still requires page reload;
- claiming success from one browser context only;
- claiming completion without deployed staging proof.

## Current Belief State

Starting observations from code audit:

- `/api/ws` exists through proxy and sandbox routing, but sandbox `HandleWS`
  sends a connected message and echoes input. The Svelte desktop connects, but
  ignores `onmessage`.
- Trace uses trajectory-scoped SSE at
  `/api/trace/trajectories/{id}/events` with `after_seq` catch-up.
- VText uses document-scoped SSE at `/api/vtext/documents/{id}/stream` for
  revision/head changes.
- Compute Monitor polls `/api/compute/status` every 15 seconds and has a
  visible refresh icon.
- Candidate Desktop and Settings promotion queue expose manual Refresh buttons.
- VText recent documents exposes a manual Refresh button.
- Podcast/content library refetches on load/back/import; media progress is
  stored in localStorage.
- Image/Audio/Video/PDF/EPUB recents and generic media playback progress are
  localStorage-backed.
- Desktop state is already server-side through `/api/desktop/state`, backed by
  embedded Dolt `desktop_workspaces`, but cross-device live notification is
  absent.

Main uncertainties:

- Whether the right event transport should be one `/api/ws` bus plus narrow
  existing SSE streams, or whether some current SSE should be folded into the
  bus after the first proof.
- Which durable schema should own media progress, media recents, theme state,
  and per-device layout profile metadata.
- How much frontend live-store abstraction is enough without creating a
  generic state framework that slows app-specific improvements.
- Which existing polling loops are intentional recovery probes versus stale
  product-state crutches.

Highest-impact observation:

- A staging proof where one authenticated user opens two browser contexts,
  mutates desktop/media/content/promotion-observable state in one context, and
  the other context updates without visible Refresh controls or browser reload.

## Three-Pass Hard Cutover

The passes are described separately for orientation, but they are one artifact.
Do not stop after pass 1 if pass 2 or 3 has an executable next probe inside the
mission authority.

### Pass 1: Computer Live Bus

Turn `/api/ws` into the authenticated user-computer notification fabric.

Requirements:

- define a stable event envelope with at least:
  - `type`;
  - `event_id`;
  - `owner_scope` or equivalent caller-derived scope, not raw public user ids;
  - `desktop_id` where relevant;
  - object identity fields such as `doc_id`, `content_id`, `window_id`,
    `trajectory_id`, `candidate_id`, or `media_identity`;
  - durable `revision`, `updated_at`, `stream_seq`, or store commit reference
    sufficient for catch-up/refetch;
  - redacted payload suitable for browser delivery;
  - `source_device_id` or equivalent to suppress local echo where useful.
- connect the websocket handler to runtime event publishing rather than echo;
- keep auth and desktop selector semantics aligned with proxy/runtime routes;
- add reconnect/backoff/catch-up behavior on the frontend;
- create a small frontend live-store module for subscribing to bus events;
- preserve existing VText/Trace SSE if they remain the best scoped transport,
  but make the bus architecture compatible with them rather than parallel.

### Pass 2: Durable Synced State APIs

Move cross-device app state off browser localStorage and onto product-owned
Dolt-backed APIs.

Required durable state:

- media progress for Podcast, Audio, and Video:
  - media kind;
  - stable media identity;
  - current time;
  - duration;
  - playback rate where relevant;
  - updated timestamp;
  - source device metadata if useful for conflict handling.
- media recents for Image, Audio, Video, PDF, EPUB, and Podcast where
  applicable:
  - media kind;
  - stable identity;
  - title/file/source/content reference;
  - opened timestamp.
- theme/settings state currently stored in localStorage.
- desktop state change notifications for the existing Dolt-backed desktop
  state API.
- content/file change events for Files and content-backed apps.

Conflict rules:

- Media progress from a remote device may update idle/paused players. If the
  local device is actively playing or seeking the same media, do not jerk the
  playhead; record the remote position and converge when playback pauses or
  when the user explicitly switches continuation point.
- VText remote head updates should not overwrite dirty local edits. Preserve
  existing revision conflict behavior and make the live event a prompt to load
  or show a new-version state where needed.
- Desktop geometry is viewport-sensitive. Sync semantic window state and app
  context by default; geometry may require per-device layout profiles instead
  of blind cross-device overwrite.
- Theme changes may apply live across contexts unless doing so would disrupt an
  active accessibility preference edit in progress.

### Pass 3: Remove Manual Refresh Sync UI

Hard cutover product surfaces that should update live.

Remove visible manual data-refresh/reload controls from:

- Compute Monitor status refresh;
- Candidate Desktop queue refresh;
- Settings promotion queue refresh;
- VText recent documents refresh;
- Podcast library/content refresh behavior exposed through UI;
- Files/content list refresh if one is present or added during the run;
- any app-specific "Reload/Refresh" control whose only purpose is to repair
  stale Choir product state.

Replace with:

- live status indicators only where useful: `Live`, `Reconnecting`,
  `Catching up`, `Offline`, or `Stale: reconnecting`;
- automatic catch-up on reconnect, visibility return, and computer wake;
- precise error states with recovery action only when the transport or auth
  actually failed.

Do not keep a hidden debug refresh button in product UI. If an operator-only
diagnostic endpoint is needed, keep it out of browser-public product routes and
out of acceptance.

The Browser app page reload button is not a Choir state sync control and may
remain as browser navigation. It must not be used to prove live sync.

## Live Event Vocabulary

Start with the smallest durable vocabulary that covers the product surfaces.
Names may change during implementation if tests and docs are updated, but the
semantics should remain:

```text
desktop.state.updated
content.item.created
content.item.updated
content.file.changed
media.progress.updated
media.recent.updated
theme.updated
promotion.queue.updated
computer.status.updated
vtext.document.updated
trace.trajectory.updated
```

Event payloads should be object references plus revision/catch-up metadata. If
an app needs full data, it should refetch the product API projection for that
object.

## Homotopy Parameters

Increase realism continuously while preserving the artifact:

- **Transport:** connected echo channel -> owner-scoped event stream ->
  sequence/revision catch-up -> multi-context convergence under reconnect.
- **State coverage:** desktop state -> media progress/recents -> content/files
  -> promotion/candidate queues -> compute status -> theme/settings.
- **Device count:** one tab -> two tabs same browser -> two browser contexts ->
  desktop plus 390x844 mobile emulation.
- **Failure realism:** clean websocket -> reconnect -> missed events ->
  auth renewal -> computer wake/resume.
- **Mutation realism:** passive observation -> app-open recents -> media
  progress -> file/content import -> candidate/promotion queue transition.
- **UI cutover:** refresh buttons still present but unused in local probe ->
  buttons removed in code -> tests fail if buttons/text return -> staging proof
  has no visible manual sync controls.

## Investigation And Cognitive Reframing

Do not stop at "websocket unreliable" or "state is stale." Classify the
failure layer:

- durable mutation did not write the expected state;
- mutation wrote but did not publish an event;
- event published but wrong owner/desktop/object scope;
- proxy websocket failed auth or routing;
- client received event but ignored it;
- client refetch failed or used stale cache;
- local active foreground state correctly rejected remote update;
- UI still exposes or depends on manual refresh;
- staging/deploy identity is stale.

Before accepting a hard blocker, run root-cause probes at the implicated layer
and apply route-changing transforms:

- **Truth-before-transport:** verify the Dolt/product record first, then event
  delivery. Do not debug sockets for a missing write.
- **Notification-not-state:** shrink websocket payloads if payload ownership
  becomes confusing; refetch canonical projections.
- **Two-context witness:** if one browser looks correct, add a second context
  before calling it live.
- **Cutover honesty:** if removing Refresh exposes a stale-state bug, fix the
  event/refetch path rather than restoring the button.
- **Scope minimization:** if a broad bus leaks or over-updates, narrow by
  owner, desktop, object type, or app subscription rather than disabling live
  sync.

If a blocker defines an executable next probe inside current authority, run
that probe instead of ending.

## Receding-Horizon Control

Operate in short control intervals:

1. Choose the highest-error refresh/localStorage/polling surface.
2. Identify its canonical durable state and mutation path.
3. Add or connect the event emission after durable write.
4. Wire the frontend subscriber/refetch path.
5. Remove the manual refresh affordance for that surface.
6. Verify with targeted unit/component tests and at least one browser-context
   proof.
7. Update belief state, then move to the next surface.

Mutation radius should stay bounded by surface or layer. Avoid a giant
frontend rewrite. The shared live bus can be central; app state handling should
remain app-specific where semantics differ.

## Dense Feedback Channels

Use feedback that reveals local error:

- Go tests for websocket auth/scope, event envelope, catch-up metadata, and
  product API emission after durable writes.
- Frontend/unit tests for live store reconnect handling and removal of manual
  sync buttons.
- Playwright with two browser contexts:
  - same user, same desktop selector;
  - desktop and 390x844 mobile viewport where feasible;
  - mutate in one context and observe live convergence in the other;
  - assert no visible Refresh/Reload synchronization UI in covered apps.
- Browser console/network logs for websocket connect, reconnect, and event
  receipt without repeated polling.
- Product API reads proving Dolt-backed state exists after mutation.
- Trace/VText/run-acceptance records where long-running or agentic changes are
  involved.
- Staging identity proof after deploy.

## Evidence Ledger

For each nontrivial claim, record:

```text
claim:
evidence source:
command or observation:
artifact path:
result:
uncertainty/caveat:
promotion relevance:
```

Required claims:

- `/api/ws` is an authenticated owner/computer-scoped product event channel,
  not echo-only.
- durable state writes publish live events after persistence.
- reconnect/catch-up repairs missed events.
- media progress and recents sync through server state, not localStorage.
- covered apps no longer expose manual data-refresh controls.
- two visible contexts converge without reload.
- staging is serving the pushed commit.

## Run Checkpoint & Resumption State

Update this section during or after execution. A checkpoint is not completion.

```text
status: implementation_checkpoint_pending_staging
last checkpoint: local hard-cutover implementation and tests prepared from baseline 3b5890a3660611b98821e8c8214cd04f6bc37bc5
current artifact state: /api/ws is runtime-owned and event-backed; old sandbox echo WS handler removed; media progress/recents and theme preferences are Dolt-backed product APIs; desktop/content/file/media/theme mutations emit owner-scoped product events; covered manual refresh UI and synced-state browser-storage calls are removed
what shipped: not yet deployed at this checkpoint
what was proven: frontend build; source-level no localStorage/manual-refresh regression; focused Go tests for media/theme storage and event emission; focused Go tests for /api/ws live delivery and after_seq catch-up; sandbox/cmd compile
unproven or partial claims: deployed staging identity; two-browser-context convergence on staging; 390x844 mobile screenshots/DOM metrics; true push events for host/proxy-originated computer warmness changes
belief-state changes: Files mutations live outside runtime canonical tables, so file API writes now emit product events through the sandbox file handler after filesystem mutation; Compute Monitor can refresh on product events but still lacks a first-class vmctl status push source
remaining error field: staging deployment and cross-device proof; computer-status live source; broader Trace/VText live convergence proof beyond existing scoped SSE
highest-impact remaining uncertainty: whether staging proxy/runtime route order and VM boot state expose the new runtime /api/ws semantics cleanly under authenticated two-context use
next executable probe: push the implementation, monitor CI/deploy, verify staging commit identity, then run two-context desktop/mobile Playwright proof for theme/media/desktop/file convergence without manual refresh
suggested resume goal string: see One-Line Goal String
evidence artifact refs: local commands named in final report
rollback refs: git baseline 3b5890a3660611b98821e8c8214cd04f6bc37bc5
```

## Forbidden Shortcuts

- Do not preserve manual Refresh/Reload synchronization UI for compatibility.
- Do not add a hidden product refresh button or debug-only browser control.
- Do not keep localStorage as fallback for synced media progress, recents,
  theme, or desktop/app state.
- Do not call polling with a short interval "live sync."
- Do not publish websocket events before durable state has been written.
- Do not accept same-tab-only proof for cross-device state.
- Do not expose host/global telemetry, raw VM ids, raw user ids, emails,
  secrets, private paths, or provider credentials over websocket.
- Do not use `/internal/*`, `/api/test/*`, `/api/agent/*`, or raw event
  mutation endpoints for browser-public proof.
- Do not use browser reload, route reload, or clearing storage as acceptance.
- Do not merge VText/Trace streams into a generic bus if doing so weakens their
  current revision/trajectory catch-up semantics.
- Do not summarize stale or partial sync as success.

## Rollback Policy

- Git rollback target is the starting platform baseline
  `3b5890a3660611b98821e8c8214cd04f6bc37bc5` plus any later deployed SHA named
  in the run.
- Schema changes must be additive or come with explicit migration/rollback
  notes. If synced-state tables are added, document how stale rows can be
  ignored or migrated.
- Product event bus changes must fail closed: if websocket fails, state remains
  durable and clients show reconnecting/stale state rather than corrupting data.
- If a live update regression risks data loss, disable the affected emission or
  subscriber path through code rollback, not by restoring manual refresh UI as a
  product behavior.
- Preserve active computer state. Do not clear desktop/app state or media
  progress as a rollback substitute unless the user explicitly authorizes it.

## Learning Side-Channel

Record tactical discoveries in this mission doc checkpoint section and tests.
Update canonical docs only when the run changes durable operating rules:

- [platform-os-app-state.md](platform-os-app-state.md) if app state ownership or
  live-sync coverage changes;
- [computer-ontology.md](computer-ontology.md) if computer state ledgers or
  promotion semantics change;
- [architecture.md](architecture.md) if the real-time architecture changes from
  "SSE/WebSocket possible" to a concrete Computer Live Bus contract.

Do not hide important synchronization design decisions only in final chat.

## Stopping Condition

Report `complete` only when all are true:

- `/api/ws` or the chosen live transport path is a real authenticated
  user-computer event fabric with tests.
- Existing VText/Trace SSE either remain justified and integrated into the live
  architecture or are safely replaced without weakening catch-up semantics.
- Media progress and media recents are Dolt/product-backed and cross-context
  synced; localStorage compatibility for synced state is removed.
- Desktop/app state changes publish notifications and converge across contexts.
- Compute Monitor, candidate/promotion queues, VText recent documents,
  Podcast/content library, Files/content changes, theme changes, and covered
  media apps no longer rely on visible manual data-refresh UI.
- Tests fail if covered manual Refresh/Reload synchronization affordances
  return.
- Staging proof shows at least two authenticated browser contexts observing
  live convergence without browser reload or manual refresh.
- Desktop and 390x844 mobile Playwright evidence exists for the covered UI.
- Staging identity matches the pushed commit.
- Rollback refs and residual risks are named.

If useful progress lands but any required proof is missing, report
`checkpoint_incomplete`, update this mission doc with the exact frontier, and
continue or delegate the next safe probe unless an authority/time/safety
boundary prevents it.

If a blocker remains after root-cause probes and cognitive transforms, report
`blocked_incomplete` with exact evidence, rollback state, and the smallest safe
next probe.
