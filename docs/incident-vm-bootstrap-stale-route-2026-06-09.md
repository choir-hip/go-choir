# Incident: Stale active VM route returned bootstrap 502

Date: 2026-06-09

## Summary

An authenticated owner session on `https://choir.news` hung in the VM bootstrap
screen. The UI reached "Resolving active computer" and then retried with
`VM route returned 502; retrying`. A newly registered account could boot, but
its first bootstrap was several seconds slower than the warm path.

## Evidence

- Owner screenshot at 2026-06-09 16:33 showed:
  - `BOOTSTRAP FAILED (502)`
  - `Bootstrap probe 1 is still waiting; retrying`
  - `VM route returned 502; retrying`
- Staging `/health` before remediation reported:
  - `vmctl_routing: enabled`
  - `vmctl_status: ok`
  - `bootstrap.total` mostly failing with `http_502`
  - `bootstrap.upstream` mostly failing with `http_502`
  - websocket totals dominated by `dial_error`
- A fresh Playwright/WebAuthn account booted successfully:
  - first `/api/shell/bootstrap`: HTTP 200 in about 6.9s
  - repeated `/api/shell/bootstrap`: HTTP 200 in about 45-55ms
- Forced staging workflow run `27234167076` refreshed active interactive
  computers:
  - `vm-d067e51c904a6fc6b7810ec7dee75ad1`
  - `vm-cd22ad1759a2c91a1f9254fd9bf0edd5`
- Staging `/health` after the forced deploy reported:
  - proxy and sandbox deployed at `82c3db878b3cccda672d04cb1294b81ff6b08082`
  - `bootstrap.total`: `3` requests, all `http_200`
  - `bootstrap.upstream`: `3` requests, all `http_200`
  - `ws.total`: `1` connection, all `connected`

## Belief State

Auth was not the root cause. vmctl route resolution was working, but at least
one already-owned primary computer resolved to a sandbox endpoint that was not
serving bootstrap or websocket traffic. New accounts minted fresh VMs and
therefore avoided the stale route. The forced deploy's active-VM refresh repaired
the symptom by rebooting active interactive computers while preserving data
images.

## Remaining Error

The product recovery path exposes "Wake current computer", but the proxy
implementation only resolves and probes the current computer. It does not invoke
the vmctl refresh operation that repaired this incident when the resolved
runtime is unreachable. Deploy refresh can repair the condition, but an owner
cannot self-heal the same stale route from the boot/recovery surface.

## Prevention Direction

- Wire authenticated compute recovery to refresh the current computer when the
  resolved runtime health probe is unreachable.
- Keep the operation owner-scoped: only the authenticated user's selected
  desktop should be refreshed.
- Preserve data images; use vmctl refresh rather than ownership deletion.
- Add regression coverage for a dead resolved runtime that becomes reachable
  after `wake_current_computer`.
- Add a UI escape hatch after repeated bootstrap `502` retries so owners can
  trigger this repair without waiting for an operator-forced deploy.

## Second Finding: Resolve-timeout boot loop after the route refresh fix

After deploying `678d2df114d636a2c42e24f14f5a28d9e0f4e08b`, the same owner
session produced a different boot failure. The BIOS panel no longer showed
`BOOTSTRAP FAILED (502)`. It showed:

- `COMPUTER BOOT IS STILL PENDING`
- repeated `Bootstrap probe N is still waiting; retrying`
- probe intervals of about 15 seconds

Staging health at the same deployed commit showed:

- `bootstrap.auth`: all authenticated `ok`
- `bootstrap.resolve`: `23` requests, `17` errors, maximum duration about
  `15011ms`
- `bootstrap.total`: `17` `resolve_error` results
- `bootstrap.upstream`: `6` requests, all `http_200`, maximum duration about
  `1ms`
- `api.resolve`: maximum duration about `180042ms`

This moves the failing edge earlier than the first incident. The proxy is not
reaching an unhealthy sandbox and receiving `502`; it is waiting inside vmctl
desktop resolution until the browser's 15 second bootstrap probe aborts. The
longer `api.resolve` maximum indicates at least one authenticated API request
waited for the vmctl client's full 180 second timeout.

Code inspection narrowed the vulnerable path:

- `/api/shell/bootstrap` resolves the primary desktop with
  `ResolveDesktopContext`.
- `ResolveDesktopContext` is allowed to boot, resume, readiness-check, recover,
  or join an in-flight pending assignment before it returns a route.
- the boot UI aborts each bootstrap probe after 15 seconds.
- `/api/compute/recovery` also starts `wake_current_computer` by calling
  `ResolveDesktopContext`, so the recovery action can be trapped behind the
  same pending readiness wait before it gets a chance to refresh an unreachable
  current computer.

The current belief is therefore:

1. The previous fix repaired one recovery branch after a resolved route is
   returned.
2. It did not create a true escape hatch for a current computer whose primary
   failure is that vmctl resolution itself is blocked or slow.
3. Existing active ownership needs a fast lookup-first path for status/recovery,
   and the boot UI needs to request that recovery path after repeated pending
   bootstrap probes instead of only looping on `/api/shell/bootstrap`.
