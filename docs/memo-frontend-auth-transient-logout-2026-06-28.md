# Memo: Frontend Auth Transient Logout Bug

**Date:** 2026-06-28
**Mutation class:** red (auth/session renewal is a protected surface)
**Heresy delta:** discovered + repaired

## Problem

Users get logged out during staging deploys. The root cause is **not** session
loss — sessions persist in SQLite at `/var/lib/go-choir/auth/auth.db`, the JWT
signing key persists at `/var/lib/go-choir/auth-signing/ed25519-key`, and the
deploy uses `nixos-rebuild switch` (live, no reboot).

The actual cause is in the frontend: `requestSessionState()` in
`frontend/src/lib/auth.js` throws on ANY non-2xx response with no retry logic.
`checkSession()` in `frontend/src/App.svelte` catches ALL errors and
immediately transitions to `signed_out` state.

During a deploy, `systemctl restart go-choir-auth` causes a 2-14 second auth
service downtime window (confirmed in journal logs). Any `/auth/session` call
during that window gets a connection refusal or 5xx error, which the frontend
treats as a permanent auth failure and logs the user out.

## Evidence

1. **Journal logs** (node-b, 2026-06-28):
   ```
   03:44:33 auth: received terminated, shutting down gracefully
   03:44:33 auth: server stopped
   03:44:47 systemd: Starting go-choir Auth Service...
   03:44:48 auth: starting server on 127.0.0.1:8081
   ```
   14-second gap between stop and start.

2. **Code analysis** (`frontend/src/lib/auth.js:287-296`):
   ```javascript
   async function requestSessionState() {
     const res = await fetch('/auth/session', { credentials: 'include' });
     if (!res.ok) {
       throw new Error(`/auth/session failed: ${res.status}`);
     }
     return res.json();
   }
   ```
   No retry, no backoff, no distinction between 401 and 5xx.

3. **Code analysis** (`frontend/src/App.svelte:95-101`):
   ```javascript
   } catch (_err) {
     // Network error or unreachable — stay signed out.
     authState = 'signed_out';
     currentUser = null;
     return { authenticated: false };
   }
   ```
   The comment says "stay signed out" but the code actively transitions to
   `signed_out`, displaying the guest auth UI and requiring re-authentication.

4. **User report**: Users have been observed getting logged out during staging
   deploys.

## Fix

Distinguish transient failures (5xx, network errors) from permanent auth
failures (401/403). Add retry with exponential backoff for transient errors.
Only transition to `signed_out` when the server explicitly returns
`{authenticated: false}`.

### Files changed
- `frontend/src/lib/auth.js` — add retry logic to `requestSessionState()`
- `frontend/src/App.svelte` — don't logout on transient errors, retry instead

## Rollback

Revert the frontend changes. The old behavior (immediate logout on any error)
is the current production behavior and is safe to restore.
