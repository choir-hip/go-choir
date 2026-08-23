# Staging Evidence: vmctl HandleResolve Global Mutex Lock Contention

**Date:** 2026-08-23  
**Mutation Class:** `red` (vmctl / routing / protected lifecycle boundary)  
**Affected Users:** `yusefnathanson@me.com` (`computer-03335285269bdba4f94377e56879f9e6`), `a@b.com` (`computer-bb0f4fa583c0cde14334818d946e6378`), all staging users  

---

## 1. Observed Symptom & Ground Truth

When users attempt to sign in or connect to `choir.news` in the browser, the frontend BIOS enters an infinite retry loop:

```text
>CHOIR BIOS
Computer boot is still pending
00s  Powering user computer
00s  Resolving active computer
15s  Bootstrap probe 1 is still waiting; retrying
31s  Bootstrap probe 2 is still waiting; retrying
47s  Bootstrap probe 3 is still waiting; retrying
```

On Node B (`staging`), the proxy logs repeated context cancellations during internal resolve:
```text
proxy: failed to resolve autoputer for user 0e5c45ab-44de-49cd-b07d-e58973b21ad5 desktop primary: vmctl client: resolve call failed: Post "http://127.0.0.1:8083/internal/vmctl/resolve": context canceled
proxy: failed to resolve autoputer for user 5bd6de97-3b58-408c-bf89-c42c81b083de desktop primary: vmctl client: resolve call failed: Post "http://127.0.0.1:8083/internal/vmctl/resolve": context canceled
```

Direct HTTP probes to `http://127.0.0.1:8083/internal/vmctl/lookup` and `/internal/vmctl/resolve` on Node B hung indefinitely (exceeding 30s timeouts).

---

## 2. Root Cause Analysis

Commit `4a33fff` (`vmctl: admit canonical ordinary route absence`) introduced a global lock acquisition with deferred unlock in `internal/vmctl/handlers.go`:

```go
func (h *Handler) HandleResolve(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost { ... }
    var req resolveRequest
    ...
    if h.routeAuthority != nil {
        h.routeAuthority.mutationMu.Lock()
        defer h.routeAuthority.mutationMu.Unlock()
    }
    route, routeKnown, err := h.resolveComputerVersionRoute(r.Context(), req.UserID, req.DesktopID)
    ...
    var own *VMOwnership
    if routeKnown && !route.RouteAbsent {
        own, err = h.registry.resolveExistingDesktopContext(r.Context(), req.UserID, req.DesktopID, expectedVMID)
    } else {
        own, err = h.registry.ResolveOrAssignDesktopContext(r.Context(), req.UserID, req.DesktopID)
    }
    ...
}
```

### Mechanisms of Failure:

1. **Global Contention:** `h.routeAuthority.mutationMu` is a process-wide mutex. Deferring its unlock in `HandleResolve` holds the mutex across `resolveExistingDesktopContext` / `ResolveOrAssignDesktopContext`.
2. **Head-of-Line Blocking during Cold Replays:** When a large computer (e.g. `computer-03335285269bdba4f94377e56879f9e6` with 132,436 events) boots, `mgr.BootVM` $\rightarrow$ `waitForGuestReady` waits for the single-vCPU guest to sequentially replay all events, taking up to `VM_BOOT_READY_TIMEOUT` (30 minutes).
3. **Cascading Resolve Drop:** While one VM is booting, every other user's resolve request (such as `a@b.com`) blocks on `mutationMu.Lock()`. When the proxy's 60s timeout elapses, the proxy cancels the context and returns HTTP 504 / 502, creating an infinite BIOS probe loop.
4. **Coalescing Gap in `ensureActiveVMReady`:** In `internal/vmctl/ownership.go:1221-1231`, `activeOwnershipNeedsReadinessCheck` drops `r.mu` and executes `ensureActiveVMReady` without registering `pendingWaiters`, which was masked only by the accidental global `mutationMu`.

---

## 3. Consensus Panel Findings & Decisions

An independent agentic consensus review confirmed:
* **Removal is Safe:** Route CAS transactions are already guarded by SQL generation CAS in `internal/routeledger/sql_ledger.go`. `mutationMu` is only for write CAS within `RouteAuthority`, not for read queries or VM lifecycle boots.
* **Per-Computer Waiter Coalescing:** `OwnershipRegistry`'s in-memory `pendingWaiters[key]` already provides correct per-computer isolation.
* **Readiness Check Coalescing:** `resolveDesktopContext` must register `pendingWaiters` before calling `ensureActiveVMReady` so concurrent resolve calls for the same computer coalesce.

---

## 4. Planned Repair

1. Remove `h.routeAuthority.mutationMu.Lock()` and deferred unlock from `internal/vmctl/handlers.go`.
2. Add waiter coalescing in `internal/vmctl/ownership.go` for the `ensureActiveVMReady` branch.
3. Add regression tests in `internal/vmctl/vmctl_test.go` verifying concurrent resolution of a ready VM while another VM executes a slow/blocking boot.
