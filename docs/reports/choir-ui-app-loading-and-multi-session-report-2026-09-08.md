# Choir Engineering Report: App Window Boot Loading, Email Pagination, and Multi-Session Concurrency

**Date**: September 8, 2026  
**Author**: Choir Platform Engineering  
**Status**: Settled, Implemented, Deployed, and Verified on Staging (`https://choir.news`)  
**Staging Target**: Retained Computer `computer-03335285269bdba4f94377e56879f9e6` (Node B)  

---

## 1. Executive Summary

This report documents the architectural investigation, agentic consensus adjudication, implementation, and deployed staging verification for two critical platform issues resolved on September 8, 2026:

1. **Guest MicroVM Boot App Window Loading ("Reload the App" Defect)**:
   - **Symptom**: Opening a Choir account frequently loaded desktop windows (e.g. Email, Settings, Files) displaying the error: `Could not open <App> — Failed to fetch dynamically imported module ... [Reload app]`. A recent commit added an automatic `window.location.reload()` workaround in `AppHost.svelte`, which masked the failure with an unprompted page refresh rather than addressing the root cause.
   - **Root Cause**: An **asset authority boundary crossing without document handoff**. Unauthenticated visitors load `index.html` from the **Host Platform Shell**, which bakes the host's chunk hashes into browser memory. Upon passkey authentication, the SPA transitioned in-memory to `signed_in` without reloading the document. When the SPA attempted to dynamically import app components, the proxy (`computer_surface.go`) routed authenticated `/assets/*` requests to the **Guest MicroVM**. Because the microVM serves its own distinct frontend build (or older commit), it lacked the host's chunk hashes and returned HTTP 404 Not Found, causing Vite's dynamic import to throw.
   - **Resolution**: Implemented an explicit post-login document handoff in `frontend/src/App.svelte` that awaits microVM boot readiness (`prewarmAuthenticatedComputer()`) and cleanly navigates the document to the authenticated computer's surface (`window.location.replace()`). This guarantees the browser executes the computer's exact asset graph under Choir doctrine C15/I25, eliminating the `AppHost.svelte` auto-reload workaround.

2. **Email Inbox 50-Limit & Multi-Browser Session Invalidation**:
   - **Symptom A (Email)**: Inboxes with more than 50 messages (e.g. 93 messages on `yusefnathanson@choir.news`) displayed only 50 messages in the message list.
   - **Root Cause A**: Backend `ListMessagesPaged` clamped limits to 50 when greater than 100 or unspecified. In the frontend, `.rows` was an unconstrained flex child missing `flex: 1; min-height: 0;`, preventing internal scroll events from firing to trigger keyset cursor pagination.
   - **Resolution A**: Raised backend default and query limit to 100 (clamped up to 500), configured `.rows` with `flex: 1; min-height: 0; overflow-y: auto;`, and added a fallback "Load older messages" button.
   - **Symptom B (Auth)**: Logging in or signing out on one browser terminated active sessions on other browsers.
   - **Root Cause B**: `HandleLogout` in `internal/auth/handlers.go` unconditionally deleted all refresh sessions for the user (`DeleteRefreshSessionsByUserID`), invalidating concurrent sessions on other devices when access JWTs expired (5 minutes).
   - **Resolution B**: Scoped default logout to the presenting device's session (`DeleteRefreshSessionByID`), reserving global invalidation strictly for explicit `?all=true` queries.

---

## 2. Deep Dive: App Window Content Loading on Guest Boot

### 2.1 Direct Forensic Evidence on Live Staging (Node B)

Direct inspection of Node B revealed the exact version and network divergence:

1. **Host Platform Shell (`/var/www/go-choir/frontend-current/assets/`)**:
   - Built on commit `2a26726f`.
   - Entry asset: `index-CK-f-U79.js`.
   - Email component chunk: `EmailApp-Bax5fQfh.js`.
2. **Guest MicroVM (`ComputerSurface` on `http://10.200.21.2:8085`)**:
   - Built on commit `796dcb64`.
   - Entry asset: `index-DyBsa8zY.js`.
3. **Live Probe of the MicroVM for the Host's Asset**:
   ```http
   GET http://10.200.21.2:8085/assets/EmailApp-Bax5fQfh.js
   HTTP/1.1 404 Not Found
   Cache-Control: no-store
   Content-Type: text/plain; charset=utf-8
   X-Choir-Build-Commit: 796dcb64e4e952c667962e7cc99ea277f4bed92a
   X-Choir-Build-Service: autoputer
   ```

### 2.2 The Sequence of Failure

```text
[Browser: Unauthenticated]
       │
       ▼ Fetches GET / from Host Platform Shell
Loads index.html referencing host chunk hashes (index-CK-f-U79.js, EmailApp-Bax5fQfh.js)
       │
       ▼ User signs in via WebAuthn passkey (POST /auth/login/finish)
Browser receives choir_access cookie; SPA updates in-memory authState = 'signed_in'
       │
       ▼ In-memory SPA restores open windows (e.g. EmailApp)
AppHost.svelte executes: import('/assets/EmailApp-Bax5fQfh.js')
       │
       ▼ Browser sends authenticated GET /assets/EmailApp-Bax5fQfh.js
Proxy sees authenticated caller -> routes request to Guest MicroVM (10.200.21.2:8085)
       │
       ▼ MicroVM is running commit 796dcb64 (has index-DyBsa8zY.js, NOT EmailApp-Bax5fQfh.js)
MicroVM responds with HTTP 404 Not Found!
       │
       ▼ Browser dynamic import throws:
"Failed to fetch dynamically imported module: https://choir.news/assets/EmailApp-Bax5fQfh.js"
       │
       ▼ Old workaround: AppHost catches error, sets sessionStorage flag, forces window.location.reload()
Full page reload while authenticated fetches GET / from MicroVM -> loads index-DyBsa8zY.js -> succeeds
```

### 2.3 Agentic Consensus Adjudication

An Agentic Consensus panel was executed across the model panel (Codex `gpt-6-astra`, OpenCode `omen-alpha`, Devin, and Claude). The panel evaluated four architectural options:

- **Option A (Proxy Host Fallback on 404)**: *Rejected*. Violates Choir Doctrine C15/I25. A user computer may have diverged through self-development; silently falling back to the host platform shell would corrupt the computer's custom UI graph.
- **Option B (Force Sync Guest Baseline on Boot)**: *Rejected*. Overwriting the microVM's staged release destroys intentional self-development state.
- **Option C (Post-Login Document Handoff to Computer Surface)**: **Unanimously Accepted**. When authentication succeeds, the browser must transition from the host platform shell document to the authenticated computer's surface before mounting desktop windows.

### 2.4 Implementation

1. **`frontend/src/App.svelte`**:
   In `handleAuthBegin`, upon successful passkey authentication and session verification:
   ```javascript
   // Await prewarm so the user's computer microVM is awake and serving
   await prewarmAuthenticatedComputer().catch(() => {});

   // Under C15/I25, the UI that renders a user computer is that computer's surface.
   // Transition cleanly to the authenticated computer's surface so the browser
   // executes the computer's exact asset graph instead of mixing host and guest chunk hashes.
   window.location.replace(window.location.href);
   return;
   ```
2. **`frontend/src/lib/AppHost.svelte`**:
   Removed `isDynamicImportError` and the `sessionStorage` auto-reload workaround. Component loading errors now render clean diagnostics with an explicit manual retry button without uncontrolled page refreshes.

---

## 3. Deep Dive: Email Full Inbox & Multi-Browser Concurrency

### 3.1 Email 50-Message Cap

- **Problem**: Inboxes with more than 50 messages were cut off at 50 items.
- **Root Cause**:
  - `internal/maild/store.go` clamped `limit` to 50 if `limit <= 0 || limit > 100`.
  - `internal/maild/api.go` defaulted `limit = 50`.
  - In `EmailApp.svelte`, `.rows` lacked `flex: 1; min-height: 0;`, preventing scroll events from firing.
- **Fix**:
  - `internal/maild/store.go`: Raised upper limit clamp to 500 (`if limit <= 0 || limit > 500 { limit = 100 }`).
  - `internal/maild/api.go`: Default limit set to 100.
  - `frontend/src/lib/EmailApp.svelte`: `loadMessages` requests `limit=100`, `.rows` styled with `flex: 1; min-height: 0; overflow-y: auto;`, and a manual "Load older messages" button was added for keyset pagination fallback.

### 3.2 Multi-Browser Session Concurrency

- **Problem**: Signing out on one browser logged out all other browsers.
- **Root Cause**:
  - `internal/auth/handlers.go` (`HandleLogout`) called `h.store.DeleteRefreshSessionsByUserID(userID)`.
  - This deleted all session records in the `refresh_sessions` SQLite table for that user, causing subsequent 5-minute access token refreshes on other browsers to fail with `authenticated: false`.
- **Fix**:
  - Scoped default logout to the specific session presented in the request's `choir_refresh` cookie:
    ```go
    if rs, _, err := h.validateRefreshCookie(r); err == nil && rs != nil {
        _ = h.store.DeleteRefreshSessionByID(rs.ID)
    }
    ```
  - `DeleteRefreshSessionsByUserID` is now reserved strictly for explicit global logout queries (`?all=true` or `?all_devices=true`).
  - Updated `internal/auth/handlers_test.go` (`TestConcurrentSessionsOnReLogin`) to prove single-device logout preserves other active sessions across refresh rotation.

---

## 4. Verification & Staging Proof

### 4.1 Commit Sequence & CI Status

- **Commit `2a26726f`**: Activated maild 100-limit default and multi-browser auth fixes on Node B.
- **Commit `53856b78`**: Activated post-login document handoff and removed AppHost reload workaround.
- **CI Pipelines**: 100% green across all test lanes in CI runs `34239375251` and `34242654853`.

### 4.2 Deployed State on Node B (`https://choir.news`)

- `/var/lib/go-choir/deploy-receipt.json`:
  - `target_commit`: `53856b78174121ff381756430c6a7447aaaaa65b`
  - `activated_at`: `2026-09-08T15:08:12Z`
  - `frontend`: Active with asset `index-CJWU9fVc.js`.
  - `auth`: Active on `d380e4cb` (device-scoped logout).
  - `maild`: Active on `2a26726f` (100-limit default).
- **Live Inbox Query on Node B (`yusefnathanson@choir.news`)**:
  ```bash
  $ curl -s -H 'X-Internal-Caller: true' -H 'X-Authenticated-User: 5bd6de97-3b58-408c-bf89-c42c81b083de' \
    'http://127.0.0.1:8087/api/email/messages?folder=inbox'
  => total: 93, len(messages): 93
  ```
  **All 93 messages load directly on the initial page.**
- **Post-Login Verification**:
  - Logging in cleanly executes `prewarmAuthenticatedComputer()` followed by `window.location.replace()`.
  - Browser executes the authenticated microVM's exact asset graph.
  - Zero dynamic import errors; no "reload the app" banners; no reload loops.
