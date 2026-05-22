# MissionGradient: Live Computer Sync Driver Lease v0

**Status:** complete
**Date:** 2026-05-22
**Operator:** Codex-operated MissionGradient mission
**Related prior mission:** [mission-computer-live-sync-hard-cutover-v0.md](mission-computer-live-sync-hard-cutover-v0.md)
**State ledger:** [platform-os-app-state.md](platform-os-app-state.md)
**Computer ontology:** [computer-ontology.md](computer-ontology.md)

## One-Line Goal String

```text
/goal Run docs/mission-live-computer-sync-driver-lease-v0.md as a Codex-operated MissionGradient mission: make Choir's live multi-device computer sync correct under active use. Replace the current desktop-state blob semantics with session-aware durable state: shared app instances, shared semantic stack/order, per-session/window-placement records, and an explicit driver lease renewed by real user input. WebSockets/SSE are notification and catch-up channels; Dolt-backed product APIs remain canonical. Only the current driving session may change visible focus, foreground window, or local geometry; passive sessions receive live content, app roster, shelf/overview/order, badges, media progress, VText/Trace/files/compute updates, and catch-up without stealing focus. Preserve mobile as the same overlapping desktop, not phone mode. Remove stale full-desktop reload/last-writer-wins paths, manual refresh sync crutches, and localStorage-backed synced state. Land through git/CI/deploy, verify staging identity, and prove on staging with two authenticated browser contexts/devices plus 390x844 mobile Playwright that cross-device updates converge while focus/geometry stay stable in the active session, with screenshots/DOM metrics, websocket catch-up evidence, rollback refs, residual risks, and the next realism axis. If incomplete, report checkpoint_incomplete or blocked_incomplete with a resumable mission-doc checkpoint and continue/redirect/delegate any safe executable next probe inside current authority before stopping.
```

## Research Summary

Existing systems converge on one useful lesson: persistence and presence are
different ledgers.

- Figma's multiplayer model is server-authoritative at the property boundary:
  clients apply changes immediately for responsiveness, but the server orders,
  validates, and resolves conflicts. That maps well to Choir records such as
  app instances, stack order, media progress, document heads, and preferences.
- Figma's reliability work uses a websocket multiplayer service with
  authoritative ordering and checkpointing to durable storage. Choir already
  has the stronger durability anchor in Dolt/product APIs, so the live channel
  should not become the only copy of truth.
- Replicache's push/pull/poke model is a good mental model for Choir:
  product APIs perform authoritative writes; websocket messages are pokes that
  tell clients which object/revision/sequence to pull or merge.
- Electric's shape-log model reinforces the same split: define scoped subsets
  of durable data, deliver ordered changes, and support catch-up from offsets.
  Choir should implement this with owner/computer/desktop scoped product
  events before importing a general sync engine.
- Yjs and Liveblocks separate durable collaborative state from ephemeral
  awareness/presence. Cursor, focus, driver, and session activity are
  high-churn presence state, not durable computer contents.
- Browser primitives such as Page Visibility, BroadcastChannel, and Web Locks
  are useful for local same-origin tab coordination, but they are not enough
  for cross-device sync. They should support local leadership and resource
  discipline, not define the product truth.

Sources:

- Figma, "How Figma's multiplayer technology works":
  https://www.figma.com/blog/how-figmas-multiplayer-technology-works/
- Figma, "Making multiplayer more reliable":
  https://www.figma.com/blog/making-multiplayer-more-reliable/
- Replicache docs:
  https://doc.replicache.dev/
- Electric Postgres Sync:
  https://electric-sql.com/primitives/postgres-sync
- Yjs Awareness and Presence:
  https://beta.yjs.dev/docs/getting-started/adding-awareness/
- Liveblocks concepts and Storage/Presence:
  https://liveblocks.io/docs/concepts
- MDN Page Visibility:
  https://developer.mozilla.org/docs/Web/API/Page_Visibility_API
- MDN BroadcastChannel:
  https://developer.mozilla.org/en-US/docs/Web/API/Broadcast_Channel_API
- MDN Web Locks:
  https://developer.mozilla.org/en-US/docs/Web/API/Web_Locks_API

## Mission Frame

Choir is a persistent user computer that can be open on multiple devices at the
same time. Live sync must make those sessions converge without making one
session unusable while another session is active.

The previous live-sync hard cutover made product state more live and removed
some manual refresh affordances. It also exposed a deeper design problem:
desktop state is still too close to a single shared blob:

```text
windows_json + active_window + z_index + geometry
```

That model makes stale focus and stale z-order capable of crossing devices. The
short-term guard that ignores remote desktop saves in a visible tab is safe,
but it is not the final model. The final model must separate:

```text
shared computer state
+ shared app/window identity
+ shared semantic stack/order
+ session-local placement/focus
+ ephemeral driver/presence state
```

The mission is not "add another websocket." It is to make the computer's live
state model respect multi-device authority.

## Real Artifact

The real artifact is deployed, session-aware live computer sync:

```text
owner/computer/desktop scoped durable state in Dolt
+ typed product APIs for app instances, stack/order, session placement, media,
   VText, Trace, Files, Compute Monitor, preferences, and app state updates
+ owner/computer scoped event stream with durable sequence/catch-up
+ ephemeral session presence and driver lease
+ frontend merge layer that applies shared state without stealing local focus
+ staging proof across two sessions/devices
```

The artifact is not:

- a bigger `windows_json` blob;
- a websocket that broadcasts "reload the whole desktop";
- last-writer-wins focus or geometry between devices;
- a CRDT framework imported before the domain merge laws are clear;
- localStorage/sessionStorage as compatibility storage for synced state;
- a manual Refresh/Reload product-state crutch;
- a phone-mode mobile simplification;
- local-only proof of multi-device behavior.

## Invariants

- Dolt-backed product APIs remain canonical. WebSocket/SSE messages are
  notification, presence, and catch-up transport.
- Events are owner/computer/desktop scoped and redacted for browser delivery.
  No host/global system telemetry, VM internals, secrets, private prompts, or
  raw provider credentials leak into browser events.
- One user computer can have multiple browser sessions active. Sync must not
  assume a single tab, device, viewport, or screen size.
- Only the current driving session may change visible focus, foreground
  window, or local geometry.
- Driver authority is renewed by real local user input, not merely by receiving
  a remote event, visibility change, timer tick, or app hydration side effect.
- Passive sessions may update app content, document revisions, trajectory
  moments, media progress, files, compute status, badges, recents, shelf order,
  overview order, and shared app roster. They must not pop windows over the
  user's current local work.
- Global z-order is modeled as semantic stack/recency/order, not raw CSS
  `z-index`. Each session maps semantic order to local presentation.
- `x`, `y`, width, height, maximized/restored geometry, and active focus are
  session/viewport local unless a deliberate "follow/mirror this session" mode
  is added later.
- Mobile remains the same real overlapping desktop with movable windows.
- Active foreground edits, media seeking, text selection, and app interaction
  must not be overwritten by remote catch-up.
- Platform behavior changes land through git, CI, deploy, staging identity, and
  deployed product-path proof.

## Recommended State Model

Use a hard-cut schema split instead of compatibility layering around
`desktop_workspaces.windows_json`.

```text
desktop_sessions
  owner_id
  computer_id / desktop_id
  session_id
  device_id
  viewport_profile
  visibility_state
  last_input_at
  driver_until
  created_at
  updated_at

desktop_app_instances
  owner_id
  computer_id / desktop_id
  app_instance_id
  app_id
  title
  app_context_json
  lifecycle: open | minimized | suspended | closed
  shared_stack_rank
  last_used_at
  created_by_session_id
  created_at
  updated_at

desktop_window_placements
  owner_id
  computer_id / desktop_id
  session_id or viewport_profile
  app_instance_id
  x
  y
  width
  height
  mode
  local_z_index
  local_focused
  restored_geometry_json
  updated_at
```

The names can change during implementation if codebase fit demands it, but the
merge laws should not:

- app instance identity and lifecycle are shared computer state;
- semantic stack/order is shared computer state;
- geometry and visible focus are session-local presentation state;
- driver lease and presence are ephemeral, observable, and bounded.

## Driver Lease Semantics

Default behavior:

1. Each page load creates or resumes a `session_id`.
2. Pointer, keyboard, touch, wheel, drag, resize, media control, or focused app
   interaction renews that session's driver lease for a short interval.
3. The driving session may write focus/raise/minimize/placement events for its
   local session and shared semantic recency/order.
4. Other sessions become passive for presentation. They continue live content
   sync and may update shelf/overview recency without changing foreground
   focus or geometry.
5. If the user interacts on another device, that device becomes the driver.
6. A hidden/background session may catch up and persist a clean snapshot, but
   must not reassert stale focus when visible sessions exist.

Future optional mode:

- "Follow session" or "mirror session" can deliberately let one session adopt
  another session's focus/geometry. This is out of scope for v0 and must be
  explicit.

## Event Vocabulary

Avoid one generic `desktop.state.updated` reload event. Prefer typed,
object-scoped events:

```text
desktop.session.presence_updated
desktop.driver_lease.updated
desktop.app_instance.opened
desktop.app_instance.context_updated
desktop.app_instance.lifecycle_updated
desktop.stack_order.updated
desktop.window_placement.updated
desktop.window_focus.updated
media.progress.updated
media.recent.opened
vtext.document_revision.created
trace.trajectory.updated
files.content_item.created
files.content_item.updated
compute.status.updated
preferences.theme.updated
```

Each event should include:

```text
event_id
stream_seq
owner/computer/desktop scope
object identifiers
source_session_id
source_device_id
updated_at
revision/ref/cursor needed for catch-up
redacted payload or product ref
```

Clients should merge or refetch by object. They should not reload the full
desktop unless recovery logic explicitly decides a full snapshot is safer.

## Homotopy Axes

Increase realism without changing topology:

- single browser tab -> two tabs same device -> desktop and mobile browser
  contexts -> actual human multi-device use;
- app roster only -> stack/order -> placement/focus -> media/VText/Trace/Files
  content convergence;
- live connected happy path -> reconnect/catch-up -> hidden/background resume
  -> VM wake/recovery;
- no simultaneous editing -> simultaneous app use on two sessions -> explicit
  conflict reporting where merge law is not yet safe;
- one user/computer -> candidate computer and primary computer separation.

Do not switch to a fake ladder such as local-only tab sync, test-only events,
or a generic CRDT sandbox that does not preserve Choir product authority.

## Dense Feedback

Required feedback during the run:

- unit/runtime tests for new schema and API merge laws;
- frontend static tests preventing full-desktop remote reload from stealing
  focus in visible sessions;
- Playwright two-context staging proof:
  - desktop context opens and focuses VText;
  - mobile 390x844 context opens Trace or another app;
  - desktop focus remains stable while shared shelf/overview/order updates;
  - mobile focus remains stable while desktop mutates content;
  - app roster and semantic order converge after reload/catch-up;
  - `x/y/width/height` do not cross viewport profiles incorrectly;
  - media progress or recent file updates propagate without localStorage;
  - VText or Trace updates appear without manual refresh;
  - reconnect from `after_seq` or equivalent catches up missed events.
- screenshots and DOM metrics for active/passive session behavior;
- websocket/SSE event evidence with source session/device ids;
- staging `/health` identity proving deployed commit.

## Forbidden Shortcuts

- Keep `desktop.state.updated` as a reload-the-whole-desktop acceptance path.
- Let any remote event update a visible session's `activeWindowId` or foreground
  raw `z-index` by default.
- Treat CSS `z-index` as cross-device durable state.
- Store synced state in localStorage/sessionStorage as compatibility.
- Use manual Refresh, browser reload, or logout/login as the product sync proof.
- Use browser-public internal/test mutation routes.
- Import a generic CRDT/sync framework before proving the Choir-specific state
  split and merge laws.
- Simplify mobile into full-screen phone mode to avoid placement complexity.
- Hide race/focus failures behind debounce delays without explaining the
  authority model.
- Claim completion from one browser context.

## Rollback Policy

- Keep the previous desktop state persistence behavior recoverable by git
  revert of the mission commits.
- If schema cutover lands, include migration rollback notes and preserve enough
  legacy `desktop_workspaces` data to reconstruct current open windows.
- If live events regress product use, the safe rollback is to disable the new
  typed desktop merge path and return to server fetch on load/background only,
  not to restore cross-device focus stealing.
- Record rollback commit refs and schema/data migration caveats in the final
  report.

## Learning Side-Channel

Update this mission document with a concise `Run Checkpoint & Resumption State`
before any incomplete stop.

Capture surprising findings in:

- this mission doc for live-sync-specific state;
- [platform-os-app-state.md](platform-os-app-state.md) if current OS/app
  behavior changes;
- Trace/run-acceptance records for deployed proof;
- VText mission dashboard if the run is executed through Choir-in-Choir.

## Stopping Condition

Status `complete` requires deployed staging proof that:

- two authenticated sessions for the same user computer live-sync product
  content and shared app state;
- the session with last real user input is the only visible focus/geometry
  driver;
- passive sessions receive updates without stealing focus or moving windows;
- shared semantic stack/shelf/overview order converges across devices;
- session-local geometry remains viewport appropriate;
- synced media progress/recents and at least one VText/Trace/Files update
  converge without localStorage/manual refresh;
- reconnect/catch-up works after one session misses events;
- rollback refs, residual risks, and next realism axis are recorded.

If the stopping condition is not reached, report:

- `checkpoint_incomplete` when useful durable progress shipped but proof is
  partial;
- `blocked_incomplete` only after root-cause probes and route-changing
  cognitive transforms identify an invariant-level or external blocker;
- never call a checkpoint complete.

## Run Completion & Evidence

```text
status: complete
completed: 2026-05-22
last checkpoint: deployed 8c0b941c36ce620d3f6cc5ed0b5fbcdb471cac65 and full two-context staging proof passed
current artifact state:
- desktop live state is split across desktop_sessions, desktop_app_instances, and desktop_window_placements
- /api/desktop/state accepts X-Choir-Session, X-Choir-Device, and X-Choir-Viewport and returns shared app identity/order plus session-local placements
- driver lease is renewed from real local input; only the driving session writes visible focus, foreground, and local geometry
- passive sessions merge shared app roster/order without taking active focus or local geometry
- /api/ws emits typed desktop events including desktop.driver_lease.updated, desktop.app_instances.updated, and desktop.window_placement.updated
- the Shelf reads the shared liveStatus store directly and exposes reactive Connected/Disconnected status
- Desktop Overview preserves shared semantic window order for both card and map identity, rather than sorting or reversing by session-local z-index
- media progress/recents, Files changes, and VText recent/document revision updates are delivered through product APIs plus /api/ws notifications/catch-up
what shipped:
- 484d2d6 Add session-aware desktop live sync
- 664df41 Mark live channel connected from server frame
- 93b530c Stabilize live channel status detection
- 68eb684 Read live status directly from shelf store
- 615fbd7 Make Shelf live status reactive
- 1b1fabd Keep overview order session neutral
- 8c0b941 Align overview card and map order
what was proven:
- GitHub Actions run 26304651141 passed and deployed
- staging /health reports proxy and sandbox commit 8c0b941c36ce620d3f6cc5ed0b5fbcdb471cac65
- local frontend build passed
- local focused Playwright passed: tests/mobile-real-desktop-overview.spec.js and tests/computer-live-sync-hard-cutover.spec.js, 8 passed
- deployed Playwright proof passed against https://draft.choir-ip.com with one desktop context at 1440x920 and one mobile context at 390x844
- both sessions showed exact live status Connected
- desktop opened Files; mobile received Files; mobile opened Audio; desktop received Audio; mobile opened VText; desktop received VText
- desktop active app remained files while mobile active app became vtext
- desktop Files geometry stayed stable while the mobile session drove Audio and VText
- media recents propagated: desktop PUT /api/media/recents made the proof audio visible in mobile Audio
- media progress propagated: desktop PUT /api/media/progress displayed 0:42 / 6:00 in mobile Audio, and mobile GET /api/media/progress returned current_time 42
- Files convergence propagated: desktop wrote live-sync-proof-1779474356092.txt and mobile Files showed it without manual refresh
- VText convergence propagated: desktop created doc 0868c421-3817-4dbf-aaad-f4a45dda1763 and revision fbc0b3ff-940c-4048-b9f6-64f897a7850a; mobile VText recent showed "Live sync VText proof 1779474356092"
- raw websocket catch-up proved /api/ws?after_seq=6 returned missed media.recent.updated, media.progress.updated, desktop.driver_lease.updated, desktop.app_instances.updated, desktop.window_placement.updated, file.changed, and vtext.document_revision.created events
- shared Overview identity converged on both sessions: cardAppIds and mapAppIds were files, audio, vtext on desktop and mobile
- session-local focus/geometry stayed distinct: desktop Files had zIndex 3 and active=true while mobile VText had zIndex 3 and active=true; Files rectangle was desktop-sized on desktop and compact/mobile-sized on mobile
- websocket evidence recorded desktop count 1 and mobile count 2, including wss://draft.choir-ip.com/api/ws?after_seq=6 catch-up
remaining unproven or partial claims:
- Trace-specific trajectory live updates were not separately exercised in the final proof; VText and Files covered the "at least one VText/Trace/Files update" stopping condition
- long-lived real-user heavy sessions and simultaneous human edits remain a realism axis
- proof metrics still report sessionId as null from DOM extraction, but browser-visible websocket catch-up payloads include source_session_id/source_device_id for desktop events
belief-state changes:
- z-order should be semantic shared order, not raw cross-device CSS z-index
- focus and geometry must be session-local unless explicit follow/mirror mode exists
- WebSocket/SSE should wake/refetch/merge durable state, not become volatile truth
- live-status assertions must check exact visible/data status; a loose /connected/i assertion matched Disconnected during proof hardening
- Svelte store updates that cross component boundaries should be wired directly enough for the Shelf to prove status changes, not hidden behind stale prop/helper paths
- Desktop Overview must not sort or reverse by session-local z-index; the first deployed proof after 1b1fabd exposed that cards and map could still disagree, so 8c0b941 made both use the same shared semantic order
- launcher proof should use the Desk menu for non-desktop-icon apps such as Audio
- websocket catch-up should be asserted as "contains required event kinds" because the correct behavior is to replay all missed scoped events, not only the few events a proof cares about
remaining error field:
- Trace-specific live content needs the same product-path treatment as VText and Files
- session id observability for product proof needs a safe browser-visible DOM data point if future proofs should assert it without parsing websocket payloads
next realism axis:
- exercise the same driver-lease merge law in a long-lived real-user session with Trace live trajectory updates, media playback, VText editing, hidden/offline reconnect, and heavier restored window state
evidence artifact refs:
- GitHub Actions run https://github.com/yusefmosiah/go-choir/actions/runs/26304651141
- staging /health identity:
  proxy and sandbox commit 8c0b941c36ce620d3f6cc5ed0b5fbcdb471cac65, built_at 20260522181828, deployed_at 2026-05-22T18:20:17Z
- staging proof command:
  LIVE_SYNC_EVIDENCE_DIR=/Users/wiz/go-choir/test-results/live-sync-driver-lease-staging-20260522T182540Z CHOIR_AUTH_STATE=/Users/wiz/go-choir/test-results/live-sync-driver-lease-auth-20260522T182540Z/storage.json PLAYWRIGHT_BASE_URL=https://draft.choir-ip.com BASE_URL=https://draft.choir-ip.com npx playwright test tests/live-sync-driver-lease-deployed.tmp.spec.js --project=chromium --workers=1 --timeout=420000 --reporter=list
- proof screenshots and metrics:
  /Users/wiz/go-choir/test-results/live-sync-driver-lease-staging-20260522T182540Z/
  desktop-driver-files.png
  mobile-passive-files-synced.png
  desktop-overview-order.png
  mobile-overview-order.png
  desktop-after-app-content-sync.png
  mobile-driver-vtext-content-sync.png
  metrics.json
- commit 0de237f9fb5dd7e972bd12e6a79e3a576527bc6f stopped remote desktop state stealing visible focus as a temporary guard
rollback refs:
- revert 8c0b941 if only Overview card/map order alignment regresses
- revert 1b1fabd if only Overview shared semantic order regresses
- revert 615fbd7 if only Shelf live-status rendering regresses
- revert 68eb684, 93b530c, and 664df41 if the live status connection path regresses
- revert 484d2d6 to return to the previous desktop persistence path, with the caveat that the new Dolt tables may contain checkpoint data
- rollback to 0de237f for the last safe pre-mission focus-stealing guard if the session-aware path regresses broadly
```
