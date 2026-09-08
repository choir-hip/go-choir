# Choir Engineering Report: App Window Boot Loading, Email Pagination, Multi-Browser Live Sync, and Session Concurrency

**Date**: September 8, 2026  
**Author**: Choir Platform Engineering  
**Status**: Settled, Implemented, Deployed, and Verified on Staging (`https://choir.news`)  
**Staging Target**: Retained Computer `computer-03335285269bdba4f94377e56879f9e6` (Node B)  

---

## 1. Executive Summary

This report documents the architectural investigations, agentic consensus adjudications, implementations, and deployed staging verifications for four interrelated user-facing stability issues resolved on September 8, 2026:

1. **Guest MicroVM Boot App Window Loading ("Reload the App" Defect)**:
   - **Symptom**: Opening a Choir account frequently loaded desktop windows (e.g. Email, Settings, Files) displaying `Could not open <App> — Failed to fetch dynamically imported module ... [Reload app]`. An automatic `window.location.reload()` workaround previously masked this by forcing page reloads.
   - **Root Cause**: An asset authority boundary crossing without document handoff. Unauthenticated visitors load `index.html` from the Host Platform Shell with host chunk hashes. Upon passkey login, the SPA transitioned in-memory without a document reload. The proxy (`computer_surface.go`) routed authenticated `/assets/*` requests to the guest microVM, which served a different build (commit `796dcb64` vs `2a26726f`) lacking the host chunks and returning HTTP 404.
   - **Resolution**: Implemented post-login document handoff in `frontend/src/App.svelte` that awaits microVM boot readiness (`prewarmAuthenticatedComputer()`) and cleanly navigates the document to the computer's surface (`window.location.replace()`) under doctrine C15/I25. Removed the AppHost auto-reload workaround.

2. **Email Inbox 50-Message Cap**:
   - **Symptom**: Inboxes with more than 50 messages displayed only 50 messages.
   - **Root Cause**: Backend `ListMessagesPaged` clamped limits to 50 when greater than 100 or unspecified. Frontend `.rows` was an unconstrained flex child missing `flex: 1; min-height: 0;`, preventing internal scroll events from triggering keyset cursor pagination.
   - **Resolution**: Raised backend default and query limit to 100 (clamped up to 500), configured `.rows` with `flex: 1; min-height: 0; overflow-y: auto;`, and added a fallback "Load older messages" button.

3. **Multi-Browser Window Live Sync ("Minimize then Reappear" Defect)**:
   - **Symptom**: When logged into multiple active browsers, minimizing a window in Browser B caused it to minimize locally, then reappear moments later as if re-syncing from Browser A.
   - **Root Cause**: A distributed race condition between driver leases, background visibility flushes, and remote WebSocket updates:
     - When Browser A was backgrounded, `visibilitychange` called `flushDesktopState({ keepalive: true })`, resubmitting Browser A's un-minimized snapshot to the backend.
     - The backend broadcasted WebSocket event `desktop.window_placement.updated`.
     - When Browser B received the event, `observeRemoteDriverSession` unilaterally wiped Browser B's local driver lease to zero (`driverLeaseUntil = 0`).
     - Because `isDrivingSession()` was now false, `shouldApplyRemoteDesktopStateUpdate()` returned true, triggering `loadDesktopState()`. Browser B fetched the backend state Browser A had just saved and called `setWindows()`, resurrecting the un-minimized window.
     - When Browser B's 500ms debounced save timer fired, `persistDesktopState` saw `isDrivingSession()` was false and aborted Browser B's save.
   - **Resolution**: Removed `driverLeaseUntil = 0` from `observeRemoteDriverSession` in `frontend/src/lib/live-events.js` so local interaction authority is never wiped by remote events. In `frontend/src/lib/Desktop.svelte`, protected active driving and pending saves by routing remote updates to `mergeRemoteDesktopSharedState()`, and restricted `handleVisibilityChange` and `handlePageHide` flushes to only trigger when `saveTimer !== null` (i.e. only when dirty un-persisted local changes exist).

4. **Multi-Browser & Concurrent Refresh Token Rotation Race**:
   - **Symptom**: Users logged in on multiple browsers or tabs occasionally experienced unprompted sign-outs.
   - **Root Cause**: Single-token refresh rotation in `internal/auth/handlers.go` immediately deleted the predecessor refresh session from SQLite without a grace period. When concurrent requests from multiple tabs in the same browser (or parallel API requests upon 5-minute access JWT expiry) presented the old refresh cookie, the second request hit `refresh session not found`, returning `{ "authenticated": false }`. The client treated this as session revocation and transitioned to `signed_out`.
   - **Resolution**: Added `RotationGraceTTL` (default 15s) and an in-memory `recentRotations` cache in `Handler`. When a refresh token is rotated, its predecessor hash is cached with the issued successor for 15–30 seconds. Concurrent requests presenting the old token within the grace window receive the valid session rather than a 401 unauthenticated response.

---

## 2. Deep Dive: Multi-Browser Window Live Sync & Driver Lease Flaw

### 2.1 The Race Condition Breakdown

```text
[Browser A: Backgrounding]                   [Browser B: Active]
          │                                           │
          │ User switches to Browser B                │ User clicks "Minimize"
          │                                           ▼
          ▼ document.visibilityState === 'hidden'     Local window store minimizes window
          Runs flushDesktopState()                    Arms 500ms debounced save timer
          Sends PUT /api/desktop/state (un-minimized)  │
          │                                           │
          ▼ Backend commits Browser A's state         │
          Backend emits WebSocket event               │
          │                                           │
          └────────── WebSocket frame ───────────────>▼
                                                      Receives desktop.window_placement.updated
                                                      observeRemoteDriverSession() zeroes lease!
                                                      isDrivingSession() becomes FALSE
                                                      Calls loadDesktopState()
                                                      Fetches GET /api/desktop/state (Browser A's state)
                                                      Overwrites local store with setWindows()
                                                      ===> WINDOW REAPPEARS!
                                                      Debounced save fires -> lease is 0 -> aborts!
```

### 2.2 Agentic Consensus Adjudication

An Agentic Consensus panel was convened across Codex (`gpt-6-astra`), OpenCode (`omen-alpha`), and panel models on prompt `.agentic-consensus/multi-browser-sync-prompt.md`.

- **Codex Verdict**:
  - A remote save event must **never revoke local interaction authority**. Its arrival indicates another write occurred, not that the other browser's intent is newer.
  - Remove `driverLeaseUntil = 0` from `observeRemoteDriverSession`.
  - Active sessions must use `mergeRemoteDesktopSharedState()` rather than full `loadDesktopState()` replacement while local intent is pending or driving.
  - Background visibility/pagehide events must never resubmit an unchanged snapshot; only flush if there are pending un-persisted mutations.
- **OpenCode Verdict**:
  - Concurred with preserving local window modes (`minimized`, `normal`) and active focus across remote merges.

### 2.3 Implementation Details

1. **`frontend/src/lib/live-events.js`**:
   Removed `driverLeaseUntil = 0` from `observeRemoteDriverSession`:
   ```javascript
   export function observeRemoteDriverSession(remoteSessionId = '') {
     // A remote event indicates another session saved state.
     // We do NOT unilaterally zero out local interaction authority (driverLeaseUntil = 0);
     // local user interactions remain authoritative for local window operations.
   }
   ```
2. **`frontend/src/lib/Desktop.svelte`**:
   - In `handleRemoteDesktopStateUpdate`:
     ```javascript
     if (isDrivingSession() || saveTimer !== null) {
       void mergeRemoteDesktopSharedState();
     } else if (shouldApplyRemoteDesktopStateUpdate()) {
       void loadDesktopState();
     } else {
       void mergeRemoteDesktopSharedState();
     }
     ```
   - In `handlePageHide` and `handleVisibilityChange`:
     ```javascript
     function handlePageHide() {
       if (saveTimer !== null) {
         void flushDesktopState({ keepalive: true });
       }
     }

     function handleVisibilityChange() {
       if (document.visibilityState === 'hidden' && saveTimer !== null) {
         void flushDesktopState({ keepalive: true });
       }
     }
     ```

---

## 3. Deep Dive: Refresh Token Rotation Concurrency & Grace Window

### 3.1 The Failure Mode

In single-token refresh rotation without a grace window:
1. Browser tabs share the same `choir_refresh` cookie.
2. When the 5-minute access JWT expires, Tab 1 and Tab 2 make requests near-simultaneously.
3. Tab 1's request hits `/auth/session`. The server deletes the old refresh session and sets a new cookie.
4. Tab 2's request hits `/auth/session` with the old cookie. The database lookup fails (`refresh session not found`).
5. The server responds `{ "authenticated": false }`.
6. Tab 2's client throws `AuthRequiredError`, dispatches `'authexpired'`, and transitions to `signed_out`, logging the user out.

### 3.2 Implementation Details

1. **`internal/auth/config.go`**:
   Added `RotationGraceTTL time.Duration` to `Config`, defaulted to `15 * time.Second` via `AUTH_ROTATION_GRACE_TTL`.
2. **`internal/auth/handlers.go`**:
   - Added `recentRotations map[string]rotatedGraceEntry` protected by `sync.Mutex` to `Handler`.
   - In `rotateRefreshSession`: cached the predecessor token hash with the issued successor before database deletion.
   - In `tryRefreshRotation`: checked the rotation grace cache before declaring a session invalid. Concurrent requests presenting the old token within the grace window receive the successor cookies and authenticated state.
   - In `HandleLogout`: explicitly evicted grace cache entries on logout to ensure instantaneous invalidation.
3. **`internal/auth/handlers_test.go`**:
   Added `TestConcurrentRefreshRotationGraceWindow` asserting that concurrent requests with rotated tokens succeed within the grace window while `TestReplayedOldRefreshTokenFailsAfterRotation` verifies replay rejection outside the grace window.

---

## 4. Deep Dive: App Window Boot Loading & Email Pagination

### 4.1 App Window Boot Loading (C15/I25 Document Handoff)

- **Problem**: Host SPA loaded with host chunk hashes; authenticated requests routed to guest microVM serving different chunk hashes, resulting in HTTP 404 on dynamic imports.
- **Fix**: Replaced in-memory auth state transition with clean post-login document handoff (`prewarmAuthenticatedComputer()` + `window.location.replace()`) in `frontend/src/App.svelte`. Deleted `AppHost.svelte` auto-reload workaround.

### 4.2 Email Inbox 100-Limit & Layout

- **Problem**: Inboxes capped at 50 messages; unconstrained `.rows` flex child prevented scroll events.
- **Fix**: Defaulted `maild/api.go` and `store.go` to 100 messages (clamped up to 500), styled `.rows` with `flex: 1; min-height: 0; overflow-y: auto;`, and added a fallback "Load older messages" button.

---

## 5. Verification & Staging Proof

### 5.1 Deployed State on Node B (`https://choir.news`)

- `/var/lib/go-choir/deploy-receipt.json`:
  - `target_commit`: `afaf99d1af390220af3f9ca405c3dc702ac01cf0`
  - `activated_at`: `2026-09-08T16:00:59Z`
  - `auth`: Active on `afaf99d1` (rotation grace window + device-scoped logout).
  - `frontend`: Active on asset `index-Dbeg1Gr9.js` (clean document handoff + protected desktop sync).
  - `maild`: Active on `3ef4405c` (100-limit default).
- **Service Verification**:
  - `go-choir-auth.service` active and running on Node B.
  - `curl -s https://choir.news` confirms active frontend asset `index-Dbeg1Gr9.js`.
- **Unit and Integration Test Suites**:
  - `go test ./internal/auth/...`: 100% pass (including `TestConcurrentRefreshRotationGraceWindow`).
  - `go test ./internal/maild/...`: 100% pass.
  - Frontend Vite build: 100% pass.
  - GitHub Actions CI Run `34247739505`: 100% green across all 20 test and build lanes.
