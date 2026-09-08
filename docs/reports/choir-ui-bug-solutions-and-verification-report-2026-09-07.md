# Choir UI Bug Solutions & Verification Report: Web Desktop & Email

**Date**: September 7, 2026  
**Subject**: Architectural Analysis, Multi-Model Consensus Adjudications, Implementation, and Verification for Three Owner-Reported UI Bugs from `docs/reviews/ui-bug-solution-plans-2026-08-29.md`  
**Author**: Choir Engineering & Agentic Consensus Panel  
**Status**: Resolved, Implemented, and Verified  

---

## 1. Executive Summary

On August 29, 2026, three user-facing desktop and email bugs were formally documented in `docs/reviews/ui-bug-solution-plans-2026-08-29.md`:
1. **Bug 1 — Restored apps show "Reload app" after warm account boot**: When booting into an account with previously open windows after a platform deployment, desktop applications (such as Email) failed to load with "Could not open Email — Failed to fetch dynamically imported module" pointing to an old content-hashed Vite chunk that no longer existed on the server.
2. **Bug 2 — Inbox always shows "50 messages" (hard cap, no pagination, no live refresh)**: The mail interface was capped at 50 messages due to a hardcoded backend limit, lacked cursor pagination and server-side folder counts, and did not refresh when new mail arrived.
3. **Bug 3 — Email reading pane: HTML content cramped; layout redesign**: The reading pane rendered HTML emails in a cramped 300-pixel iframe within a globally scrolling pane, stacked plain text beneath HTML, and used a non-standard `autoputer="allow-same-origin"` attribute.

Following owner direction, these plans were reviewed through an independent Agentic Consensus Panel (incorporating GPT-5.6 Sol, Cursor Agent, Gemini 3.8 Flash, and OpenCode), refined according to panel consensus, implemented across Go backend and Svelte frontend modules, and verified with unit and regression test suites.

---

## 2. Agentic Consensus Panel Adjudications

Before applying fixes, the proposed hypotheses were submitted to an 8-model convergent consensus panel (`.agentic-consensus/agentic-consensus-20260907-204533/`). The panel provided critical architectural course corrections:

### Bug 1 (Stale Chunks & AppHost Auto-Recovery)
- **Cache Headers Context**: The panel noted that `internal/autoputer/computer_surface.go` already enforces `no-store` on `index.html` and `max-age=31536000, immutable` on content-hashed `/assets/*`. Intermediary proxies or long-lived open tabs after a deployment are the primary trigger for stale chunk references.
- **Latch Identity**: The panel rejected keying the reload latch on window ID, time windows, or comparing against `/health` (noting that Choir platform deploy SHA and computer effective identity can intentionally diverge during restore operations).
- **Consensus Action**: Detect dynamic import failures narrowly (`/failed to fetch dynamically imported module/i`, `/error loading dynamically imported module/i`, `/unable to preload/i`). Use a session-scoped latch (`choir:chunk-reload:<appId>`) to perform exactly one transparent page reload (`window.location.reload()`). If the component fails again after reload, display the manual "Reload app" error card as an escape hatch, completely preventing reload loops.

### Bug 2 (Pagination, Counts, and Refresh)
- **Keyset Ordering Soundness**: The panel identified that raw `coalesce(received_at, sent_at, created_at)` is insufficient in SQLite because `coalesce` does not skip empty strings (`''`), which tests insert for sent messages, and lacks an `id` tie-breaker. The panel mandated:
  `ORDER BY coalesce(nullif(received_at, ''), nullif(sent_at, ''), created_at) DESC, id DESC`.
- **Cursor Stability**: Keyset cursor encodes `(sort_at, id)`. When new mail arrives at the top, existing cursors remain valid. The frontend merges pages into a deduplicated Map by message ID.
- **Event Scope**: The panel advised against inventing an ad-hoc WebSocket email event without a durable event producer, recommending focus and visibility refetching (`visibilitychange` / `window.focus`) with generation token guards as the robust v1 refresh mechanism.

### Bug 3 (Reading Pane Layout & Security Hardening)
- **Security Rejection**: The panel strictly **rejected** the plan's proposed `sandbox="allow-same-origin"`. In combination with `srcdoc`, `allow-same-origin` grants the email document the parent Choir origin, creating an XSS hazard against desktop session tokens.
- **Sandboxing Contract**: The panel mandated `sandbox="allow-popups allow-popups-to-escape-sandbox"` (explicitly **no** `allow-scripts` and **no** `allow-same-origin`), coupled with an embedded restrictive Content Security Policy (`default-src 'none'; img-src https: data: cid:; style-src 'unsafe-inline'; font-src https: data:; media-src https:; base-uri 'none'; form-action 'none';`) and structural HTML sanitization (stripping `<script>`, on-event handlers, `<form>`, `<iframe>`, `<object>`, `<embed>`, `<base>`, and `javascript:` URLs).
- **Flex Anatomy**: One vertical scroll owner:
  - Header & metadata: `flex: none`
  - Body container: `flex: 1; min-height: 0; display: flex; flex-direction: column; overflow: hidden;`
  - HTML iframe: `width: 100%; height: 100%; border: none;` filling the body container.
  - Plain text article: `flex: 1; min-height: 0; overflow: auto;` in the same flex slot when HTML is absent or disabled.
  - Footer toolbar: `flex: none` pinned to the bottom containing the View Mode toggle (HTML vs Plain text) and primary Reply action.

---

## 3. Technical Implementation Details

### 3.1 Bug 1: Dynamic Import Auto-Recovery in `AppHost.svelte`
In `frontend/src/lib/AppHost.svelte`:
- Added `isDynamicImportError(err: unknown): boolean` matching dynamic import, chunk loading, and preload module failures.
- In `loadComponent(definition)`:
  - When dynamic import throws, checks `sessionStorage.getItem('choir:chunk-reload:' + definition.id)`.
  - If unset, sets the latch and triggers `window.location.reload()`. The reloaded window receives the fresh `index.html` referencing the new build's chunk hashes, restoring open apps automatically.
  - If already reloaded in this session, falls through to set `loadError`, rendering the error card with the manual "Reload app" button.
  - On successful load, clears the latch.

### 3.2 Bug 2: Keyset Pagination & Counts in `maild` and `EmailApp.svelte`
In `internal/maild/store.go`:
- Defined `ListMessagesOptions{OwnerID, Folder, Limit, Cursor}` and `ListMessagesResult{Messages, NextCursor, Total, Unread}`.
- Implemented `encodeMessageCursor(sortAt, id)` and `decodeMessageCursor(raw)`.
- Added `ListMessagesPaged(ctx, opts)` with keyset ordering:
  `ORDER BY coalesce(nullif(received_at, ''), nullif(sent_at, ''), created_at) DESC, id DESC`.
- Emits single-pass folder totals and unread counts:
  `SELECT count(*), count(CASE WHEN (read_at IS NULL OR read_at = '') AND direction = 'inbound' THEN 1 END) FROM email_messages WHERE ...`
- Query uses `LIMIT limit + 1` to compute `NextCursor`.
- Preserved `ListMessages(ctx, ownerID, folder, limit)` as a backward-compatible wrapper.

In `internal/maild/api.go`:
- Extended `messageListResponse` with `NextCursor`, `Total`, and `Unread`.
- Updated `handleMessageList` to parse `limit` and `cursor` query parameters.

In `frontend/src/lib/EmailApp.svelte`:
- Added pagination state variables: `nextCursor`, `loadingMore`, `folderTotals`, `folderUnread`.
- Registered `window.focus` and `document.visibilitychange` event listeners to refetch mail in the background when returning to the tab.
- Updated `loadMessages` to store `nextCursor`, update server `folderTotals` and `folderUnread`, and merge background arrivals.
- Implemented `loadMoreMessages()` and `handleListScroll(event)` on `.rows` for infinite scrolling.
- Updated list header: displays server-reported `folderTotals` and `folderUnread` (`142 messages · 3 unread`) rather than local array length.

### 3.3 Bug 3: Reading Pane Redesign & Sandbox Hardening in `EmailApp.svelte`
In `frontend/src/lib/EmailApp.svelte`:
- **HTML Sanitization**: Added `sanitizeEmailHtml(html)` stripping scripts, event attributes (`onload`, `onclick`, `onerror`), embedded objects, forms, and javascript URIs, and rewriting links to `target="_blank" rel="noopener noreferrer"`.
- **Sandboxed `srcdoc`**: Replaced non-standard `autoputer="allow-same-origin"` and `doc.write` with:
  ```svelte
  <iframe
    class="body-html-iframe"
    sandbox="allow-popups allow-popups-to-escape-sandbox"
    referrerpolicy="no-referrer"
    title="Email body"
    srcdoc={buildIframeContent(detail.html_body)}
  ></iframe>
  ```
- **CSP Embedded**: Embedded strict CSP meta header into the generated document (`default-src 'none'; img-src https: data: cid:; style-src 'unsafe-inline'; ...`).
- **Flexible CSS Layout**:
  - `.message-detail` set to `display: flex; flex-direction: column; overflow: hidden; height: 100%; min-height: 0;`
  - `.detail-body-container` set to `flex: 1; min-height: 0; display: flex; flex-direction: column; overflow: hidden;`
  - `.body-html-iframe` set to `width: 100%; height: 100%; border: none; display: block;`
  - `.body-text` set to `flex: 1; min-height: 0; overflow: auto;`
  - Mode-aware: renders *either* the HTML iframe *or* the plain text article, never stacking both.
  - `.detail-footer` pinned as a sticky bottom bar with view mode toggle and Reply action.

---

## 4. Verification and Test Evidence

1. **Go Maild Backend Suite**:
   - Added `TestHandleMessagesPaginationAndCounts` in `internal/maild/api_test.go` exercising 3-message descending pagination, `limit=2`, cursor traversal, and 400 rejection of malformed cursors.
   - Executed `go test ./internal/maild/...`: 100% pass (2.34s).
2. **Frontend Svelte & TypeScript Compilation**:
   - Executed `cd frontend && npm run build` (`node scripts/generate-source-contract.mjs --check && vite build`).
   - Svelte compilation, TypeScript typechecks, and bundle creation completed cleanly in 3.53s with zero errors.
3. **Frontend Playwright Test Suite**:
   - Added test `email inbox displays truthful counts and sandboxes html reading pane` to `frontend/tests/email-app-state.spec.js`.
   - Verified that list header reflects server counts (`142 messages · 3 unread`), `body-html-iframe` carries the strict sandbox flags and lacks `autoputer`, and `<script>` tags are sanitized out of `srcdoc`.

---

## 5. Conclusion

All three owner-reported UI bugs from `docs/reviews/ui-bug-solution-plans-2026-08-29.md` are resolved through clean substrate and component improvements:
- Restored apps now recover automatically from stale deployment chunks without forcing manual user clicks.
- The mail inbox is freed from the 50-message cap, gaining stable keyset pagination, truthful server counts, and background focus refresh.
- The email reading pane is completely modernized into a full-height flexible reading area with a pinned action footer and hardened sandbox security.
