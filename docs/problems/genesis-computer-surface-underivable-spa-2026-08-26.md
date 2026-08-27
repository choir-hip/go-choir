# Genesis Computer Surface: Underivable SPA and Missing Dynamic Chunk Preload

<problem_id: genesis-computer-surface-underivable-spa-2026-08-26>
<first_observed: 2026-08-26>
<mutation_class: orange>
<deployed_commit: 05cc87b6>
<affected_surfaces: [internal/autoputer/computer_surface.go, internal/actorruntime/adapter.go, internal/agentcore/rematerialize.go]>

## 1. Problem Description

When a new user signs up or an existing authenticated user navigates to an interactive computer surface (e.g. `new@new.com` on mobile Safari against `https://choir.news/`), the desktop shell initializes, but launching dynamically split Svelte sub-applications (such as the Settings App) fails with a client-side stylesheet preload error:

```text
Could not open Settings
Unable to preload CSS for /assets/SettingsApp-DtEB7MbW.css
[Reload app]
```

When the user taps `[Reload app]`, the browser performs a full document reload of the authenticated root URL (`/`). The page goes blank and displays a raw HTTP 503 error text:

```text
self-development checkpoint: served SPA is underivable
```

This error affects all newly provisioned computers and prevents users from accessing settings, desktop apps, or verifying clean computer initialization.

---

## 2. Evidence & Root Cause Analysis

### A. In-SPA Auth Transition & Reverse Proxy Routing
1. The unauthenticated initial page load on `https://choir.news/` receives the host platform shell (`PROXY_PLATFORM_SHELL_ROOT` at `/var/www/go-choir/frontend-current`).
2. After user authentication, `HandleComputerSurface` (`internal/proxy/computer_surface.go:30-75`) reverse-proxies authenticated root and asset requests (`/`, `/assets/...`, `/desktop`) to the user's dedicated microVM `ComputerSurface` service (`http://10.x.x.x:8085`).

### B. Fallback Defect on Hashed Assets (`internal/autoputer/computer_surface.go`)
`ComputerSurface.ServeHTTP` implements an SPA history fallback intended for client-side routing. However, it indiscriminately falls back to `index.html` for **all** missing files, including immutable hashed static assets under `/assets/*`:
```go
info, err := os.Stat(target)
if err != nil || info.IsDir() {
    target = filepath.Join(root, "index.html")
    info, err = os.Stat(target)
    if err != nil || info.IsDir() {
        http.Error(w, "self-development checkpoint: served SPA is underivable", http.StatusServiceUnavailable)
        return
    }
    w.Header().Set("Cache-Control", "no-store")
}
http.ServeFile(w, r, target)
```
When `assets/SettingsApp-DtEB7MbW.css` is missing from the microVM's `/mnt/persistent/choir-updater/current/frontend/` directory, `ComputerSurface` returns `index.html` with HTTP 200 and `Content-Type: text/html`.
Mobile Safari's Vite module loader attempts to parse this HTML as CSS, fails, and surfaces the error banner: `Unable to preload CSS for /assets/SettingsApp-DtEB7MbW.css`.

### C. Deferred Bootstrap & Incomplete Baseline Staging (`internal/actorruntime/adapter.go`)
In `internal/actorruntime/adapter.go:571-574`:
```go
if err := a.Runtime.EnsureComputerSurface(ctx); err != nil {
    log.Printf("actorruntime: computer surface baseline bootstrap deferred: %v", err)
}
a.Runtime.Start(ctx)
```
If `EnsureComputerSurface` fails during microVM startup (due to route resolution timing, updater socket readiness, or baseline path configuration), the failure is merely logged as `"deferred"`, and the microVM starts anyway without a valid `current/frontend` directory.

### D. 503 Fail-Closed Body on Reload
When the user clicks `[Reload app]`, the browser requests `GET /`. `ComputerSurface.resolvedRoot()` checks for `/mnt/persistent/choir-updater/current/frontend/index.html`. Because the baseline was not staged, `resolvedRoot()` returns an error, triggering HTTP 503 with the fail-closed body `self-development checkpoint: served SPA is underivable`.

---

## 3. Required Repair Invariants

1. **Immutable Asset 404/503 (No HTML Fallback for `/assets/*`):**
   Requests under `/assets/*` must return `http.NotFound` (404) or `StatusServiceUnavailable` (503) when the specific hashed asset is missing on disk. They must **never** return `index.html` or `text/html`.
2. **Blocking Genesis Serving Join:**
   `EnsureComputerSurface` must be a blocking startup invariant for genesis computers. A microVM must not declare itself ready or accept routed surface traffic before the full immutable frontend baseline is staged and verified.
3. **Full Asset Graph Verification:**
   Baseline verification at `EnsureComputerSurface` must check that the staged release directory contains a valid manifest and matching static files (`pinnedFrontendIdentity`), not merely that `index.html` exists.
4. **Preserve Isolation Boundaries:**
   Do not fall back to host platform shell assets from the guest VM. The per-computer frontend isolation doctrine must be preserved.

---

## 4. Problem Classification & Ceremony

- **Mutation Class:** `orange` (Runtime behavior, app state, asset routing, genesis startup).
- **Protected Surfaces:** `internal/autoputer/computer_surface.go`, `internal/actorruntime/adapter.go`, `internal/agentcore/rematerialize.go`.
- **Pre-Fix Documentation Rule:** This document establishes the problem receipt. No repair code will be committed prior to the approval of the accompanying Computer Recovery & Genesis Surface Definition.
