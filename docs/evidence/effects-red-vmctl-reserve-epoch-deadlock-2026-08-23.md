# Problem Documentation: vmctl reserveFreshVMConfig Lock Inversion Deadlock Blocking UI Boot

**Date:** 2026-08-23
**Mutation Class:** `red` (vmctl / routing / protected lifecycle boundary)
**Affected Users:** `a@b.com` (`computer-bb0f4fa583c0cde14334818d946e6378`), all interactive users on staging
**Status:** `diagnosed`

---

## 1. Observed Failure

When attempting to boot `a@b.com` from the Choir UI (`https://choir.news`), the browser enters Choir BIOS and loops indefinitely on pending probes:

```text
> CHOIR BIOS
COMPUTER BOOT IS STILL PENDING
00s
Powering user computer
00s
Resolving active computer
15s
Bootstrap probe 1 is still waiting; retrying
31s
Bootstrap probe 2 is still waiting; retrying
47s
Bootstrap probe 3 is still waiting; retrying
```

Simultaneously on Node B (`staging`), all HTTP endpoints on `vmctl` (`http://127.0.0.1:8083`), including `GET /health`, `GET /internal/vmctl/lookup`, and `POST /internal/vmctl/resolve`, hang and time out after 5–30s.

---

## 2. Evidence: Goroutine Dump on Node B

A SIGQUIT goroutine dump from `vmctl` (PID 3048197) revealed the deadlock:

```text
goroutine 41 [select]:
net/http.(*Transport).roundTrip(...)
github.com/yusefmosiah/go-choir/internal/vmmanager.(*Manager).bootVM(...)
github.com/yusefmosiah/go-choir/internal/vmmanager.(*Manager).BootVM(...)
main.(*vmManagerAdapter).BootVM(...)
github.com/yusefmosiah/go-choir/internal/vmctl.(*OwnershipRegistry).startExistingVM(...)
github.com/yusefmosiah/go-choir/internal/vmctl.(*OwnershipRegistry).ensureUniversalWirePlatformOwnership(...)
main.startUniversalWirePlatformComputer.func1(...)

goroutine 700 [sync.Mutex.Lock]:
github.com/yusefmosiah/go-choir/internal/vmmanager.(*Manager).lockVMOperation(...)
github.com/yusefmosiah/go-choir/internal/vmmanager.(*Manager).ReserveBootEpoch(...)
main.(*vmManagerAdapter).ReserveBootEpoch(...)
github.com/yusefmosiah/go-choir/internal/vmctl.(*OwnershipRegistry).reserveFreshVMConfigLocked(...)
github.com/yusefmosiah/go-choir/internal/vmctl.(*OwnershipRegistry).reserveFreshVMConfig(...)
github.com/yusefmosiah/go-choir/internal/vmctl.(*OwnershipRegistry).startExistingVM(...)
github.com/yusefmosiah/go-choir/internal/vmctl.(*OwnershipRegistry).ensureUniversalWirePlatformOwnership(...)
github.com/yusefmosiah/go-choir/internal/vmctl.(*Handler).HandleAutoputerProxy(...)

goroutines (all other requests, e.g. a@b.com resolve, health, lookup):
sync.(*RWMutex).RLock / sync.(*RWMutex).Lock
github.com/yusefmosiah/go-choir/internal/vmctl.(*OwnershipRegistry).GetOwnershipForDesktop(...)
github.com/yusefmosiah/go-choir/internal/vmctl.(*Handler).HandleResolve(...)
```

---

## 3. Root Cause Analysis

This is a classic **Lock Inversion Deadlock** between `OwnershipRegistry.mu` (`r.mu`) in `internal/vmctl` and `Manager.vmOpLocks` (`m.lockVMOperation(vmID)`) in `internal/vmmanager`:

1. **Goroutine A (`startUniversalWirePlatformComputer` $\rightarrow$ `BootVM`):**
   - Calls `mgr.BootVM(cfg)`.
   - `BootVM` acquires `m.lockVMOperation(platformVMID)` and holds it for the entire duration of the Firecracker boot and health polling loop (`waitForGuestReady`, up to 10–30 minutes).

2. **Goroutine B (`HandleAutoputerProxy` $\rightarrow$ `reserveFreshVMConfig`):**
   - Invokes `r.reserveFreshVMConfig(own, ...)`.
   - `reserveFreshVMConfig` acquires `r.mu.Lock()`.
   - Under `r.mu.Lock()`, `reserveFreshVMConfigLocked` calls `mgr.ReserveBootEpoch(platformVMID, ...)`.
   - `ReserveBootEpoch` attempts to acquire `m.lockVMOperation(platformVMID)`.
   - Because Goroutine A already holds `m.lockVMOperation(platformVMID)`, Goroutine B blocks on `m.lockVMOperation` **while continuing to hold `r.mu.Lock()`**.

3. **Global Process Freeze:**
   - Because `r.mu.Lock()` is held by Goroutine B, every other incoming request in `vmctl` (e.g. `a@b.com`'s `POST /internal/vmctl/resolve`, `GET /health`, `GET /internal/vmctl/lookup`, `WarmAlwaysOnDesktops`) blocks indefinitely on `r.mu.Lock()` or `r.mu.RLock()`.
   - The proxy's resolve call to `vmctl` times out after 15–60s, causing the frontend BIOS to report repeated `Bootstrap probe N is still waiting; retrying`.

4. **Coalescing Gap in `ensureUniversalWirePlatformOwnership`:**
   - `ensureUniversalWirePlatformOwnership` in `internal/vmctl/platform_computer.go` did not mark `own.State = VMStateBooting` and did not register `pendingWaiters` before dropping `r.mu`, allowing multiple concurrent callers to trigger `startExistingVM` on the platform VM.

---

## 4. Required Fix

1. **Break Lock Inversion:** `mgr.ReserveBootEpoch` must be called *outside* `r.mu.Lock()`. `reserveFreshVMConfig` should call `mgr.ReserveBootEpoch` first (without holding `r.mu`), and then acquire `r.mu.Lock()` only to update `r.epochCounter` and write persistence.
2. **Platform Boot Coalescing:** `ensureUniversalWirePlatformOwnership` in `internal/vmctl/platform_computer.go` must transition `own.State = VMStateBooting` and register `pendingWaiters` before releasing `r.mu`, preventing redundant concurrent boots of the platform computer.
3. **Regression Tests:** Add concurrency tests in `internal/vmctl` asserting that `ReserveBootEpoch` called during a slow/blocking `BootVM` does not block `r.mu` or stall concurrent user resolves.
