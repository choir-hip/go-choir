# UI Bug Solution Plans — Web Desktop & Email (2026-08-29)

*Solution plans only — no fixes applied (owner direction). Three owner-reported
bugs, each root-caused in the actual Svelte/Go code so the plans name real
files and seams. Landing belongs to the UX milestone (after the candidate-A
gate), except where noted as safe-anytime.*

---

## Bug 1 — Restored apps show "Reload app" after boot (stale dynamic-import chunk)

**Reported.** Booting into the warm account, previously-open desktop apps
(e.g. Email) render "Could not open Email — Failed to fetch dynamically
imported module: `https://choir.news/assets/EmailApp-<hash>.js`" with a
"Reload app" button. A manual reload fixes it. No reload should be needed.

**Root cause (verified in code).** The desktop persists only app IDs and
geometry — server-side via `/api/desktop/state` (`desktop.js:28-105`; save
serializes `window_id, app_id, title, geometry, mode, z_index,
app_context` in `Desktop.svelte:720-745`) — so restored identity is correct.
The failure is in the asset chain: the app registry hardcodes static dynamic
imports (`registry.ts:53-67`, `component: () => import('../EmailApp.svelte')`),
whose URLs Vite rewrites to content-hashed chunks at build time. When a
deploy lands, the browser may still hold the **old `index.html` + old bundle**
from cache; restoring an app re-invokes the old bundle's import map, which
points at chunk hashes that no longer exist on the server. The error handler
(`AppHost.svelte:28-41`) offers only `reloadApp(){window.location.reload()}` —
it never retries the import, busts the URL, or checks for a newer build. The
July UX review's FINDING-001 documented the same mechanism (Retry repeats the
cached failed import). There is no build-version check anywhere: build
metadata is compile-time only (`build-info.js`), Settings merely displays
frontend vs `/health` commits (`SettingsApp.svelte:647-675`), and the live WS
listener handles only theme events (`App.svelte:454-489`).

**Plan.**
1. **Server: make `index.html` uncacheable, hashed assets immutable.** The
   proxy/CDN should send `Cache-Control: no-cache` for the HTML entry and
   `max-age=31536000, immutable` for `/assets/*` hashed chunks. This alone
   removes most recurrences: a booting browser always revalidates the entry
   document, which references only current hashes.
2. **Client: auto-recover in `AppHost` before showing the error.** On
   dynamic-import failure: (a) re-fetch `/` (or a tiny `/version.json`) with
   `cache: 'reload'`; (b) if the served frontend commit differs from the
   running one, transparently reload the window **once** with a toast
   ("App updated — restoring…"); (c) only if the reloaded build still fails
   to import, show the error UI. Fix `Retry` to actually retry: re-invoke the
   import with a cache-busted query (or re-`import()` after
   `location.reload()`), not the cached failure.
3. **Client: non-blocking update notice while apps are open** (prevention).
   On a periodic or WS-driven version check, if a newer frontend commit
   appears, show the existing toast (`Desktop.svelte:1568-1580`): "A new
   version is available — reload when convenient." Open windows keep running;
   the user reloads at a natural break instead of hitting broken imports
   later.

Class: yellow (client) + orange-lite (proxy headers). No desktop-state
change needed — persisted identity is already correct.

## Bug 2 — Inbox always shows "50 messages" (hard cap, no pagination, no live refresh)

**Reported.** Inbox always shows 50 messages (may be a max) while new
messages do arrive.

**Root cause (verified).** The backend caps hard: the maild handler passes a
literal 50 (`internal/maild/api.go:176-186`) and the store clamps limit to 50
with `ORDER BY … DESC LIMIT ?` (`internal/maild/store.go:1149-1210`). The API
response contains only `messages` — no cursor, page token, or total — so
older mail is unreachable from both UI and API (detail-by-ID works if an ID
is already known, `api.go:189-225`). The frontend fetches once per folder
select with no pagination params (`EmailApp.svelte:216-270`), and the header
count is just `messages.length` (`:726-734`). New mail does not trigger a
refresh: EmailApp has no live-event listener at all (the generic WS plumbing
in `live-events.js` exists but no email event kind is handled); the test
suite even asserts exactly one fetch with no pagination params
(`email-app-state.spec.js:56-82`).

**Plan.**
1. **API: additive pagination + counts.** Extend `GET /api/email/messages`
   with `limit` (default 50) and `cursor` (opaque, keyed on the same
   `ORDER BY` columns the store already sorts by — received date + ID); the
   response gains `next_cursor` and server-side `total` + `unread` per
   folder. Store query unchanged except `LIMIT ?+1` lookahead for
   `next_cursor`. Backward-compatible: old clients ignore new fields.
2. **UI: infinite scroll on the existing locally-scrolled list**
   (`EmailApp.svelte:1060-1064` already scrolls): when the list nears the
   bottom, fetch the next cursor page and append; keep newest-first order.
3. **UI: truthful counts.** Header shows server `total` (and unread badge)
   instead of `messages.length`.
4. **UI: live refresh** — reuse the existing `live-events` WS: add an email
   receipt event kind (maild already writes on delivery) that triggers a
   first-page refetch; also refetch on window focus. Keep the generation
   token that already guards stale responses.

Class: orange-lite (additive API extension behind existing auth), green
frontend. Safe-anytime relative to candidate A — independent surface.

## Bug 3 — Email reading pane: HTML content cramped; layout redesign

**Reported.** Email HTML content renders in a small area; it should expand to
the bottom of the window, where the "HTML | Plain text", "Details" and Reply
controls are. Owner gave latitude: redesign the email app intelligently —
the interface is months old.

**Root cause (verified).** The height chain is actually fine (window →
`.window-content flex:1; min-height:0` → `.email-app grid 3 cols`), but
inside `.message-detail` (flex column, `EmailApp.svelte:1123-1130`) nothing
gives the body flexible height: `.body-html-container` is a normal block
with `margin:18px 20px 0`, and the iframe only sets `min-height:300px` with
no `height`/`flex` (`:1170-1198`), so it renders at intrinsic minimum while
the whole pane scrolls past the footer. The toggle/Details/attachments/Reply
blocks are later siblings in the same scrolling container — the footer
"islands" the owner sees are just where content happens to end. Also noted
while grounding: the iframe writes raw `detail.html_body` via
`doc.write(buildIframeContent(...))` (`:596-638`) with a nonstandard
`autoputer="allow-same-origin"` attribute instead of a real `sandbox` — worth
fixing in the same pass.

**Plan (intelligent redesign of the reader, not just a CSS patch).**
1. **Proper flex anatomy for the detail pane**: header/metadata block fixed;
   body wrapper `flex:1; min-height:0; overflow:auto` — the ONLY scrolling
   region; body iframe `height:100%` inside it; toggle/Details/attachments/
   Reply become a **sticky footer** (`flex: none`, pinned to the pane
   bottom). Result: content fills from header to footer at every window
   size, matching the owner's sketch.
2. **Mode-aware body sizing**: HTML mode renders the sandboxed iframe at
   full pane height; Plain-text mode renders preformatted text in the same
   flex slot. The iframe auto-resizes content via a `postMessage` height
   report from inside the frame (or `contentDocument.body.scrollHeight` on
   load), so long newsletters scroll inside the pane rather than in a
   cramped box.
3. **Security hardening in the same pass**: replace `doc.write` +
   `autoputer` attr with `srcdoc` + real `sandbox="allow-same-origin"`
   (or `allow-popups` only if needed), and strip/mitigate script content
   when building the document — the current code relies solely on frame
   isolation.
4. **Visual refresh** (owner gave latitude): tighten the three-column
   rhythm — sender/subject header with inline actions, date + recipient
   meta row, full-bleed body region, footer action bar with Reply as the
   primary button; collapse Detail behind a disclosure that pushes content,
   not overlays; mobile keeps the existing stacked grid but inherits the
   same flex anatomy (`EmailApp.svelte:1384-1489` shares the missing-body-
   flex bug today).
5. **Reuse, don't invent**: shared `--choir` tokens, the existing toast and
   empty-state components; no new list virtualization needed (Bug 2's
   cursor append covers growth; the list already scrolls locally).

Class: green (frontend-only). Safe-anytime relative to candidate A.

## Sequencing note

All three are frontend/maild-surface work with zero overlap with the
candidate-A execution path, the RLM build, or the deletion wave. Natural
grouping: Bug 1's server header change + Bug 2's API extension can land as
one small orange-lite deploy; Bugs 1-3 client work as one UX pass; Bug 3's
security hardening (iframe sandbox) is the only item with security weight and
should not ride casually with visual changes.
