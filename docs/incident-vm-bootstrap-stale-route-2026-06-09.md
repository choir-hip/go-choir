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

## Third Finding: stopped ownership still routes recovery through stale resume

After deploying `7ebd187e0d05b354c4d7ac3c7808e007e43bc7e4`, the boot UI did
request recovery after the second pending bootstrap probe, but the owner session
still did not recover. Node B inspection showed:

- proxy accepted the recovery request and later logged
  `proxy compute recovery: wake current computer desktop=primary: ... context
  deadline exceeded (Client.Timeout exceeded while awaiting headers)`.
- vmctl repeatedly tried to start the same primary VM
  `vm-5b0c1bef1e2b6d7f8dad7d0e8473ed19`.
- each attempt failed with `guest did not become healthy ... within 2m30s`.
- the primary ownership for the owner was persisted as:
  - `state: stopped`
  - `stopped_by: vmctl-restart`
  - stale `sandbox_url: http://10.203.109.2:8085`
  - `epoch: 159`

This explains why lookup-first recovery still failed. The code treated
`stopped` and `hibernated` ownership as normal wake/resume cases, so it still
called `ResolveDesktopContext`. In this incident, that means recovery kept
replaying the stale stopped/resume path instead of forcing the same current-image
refresh operation that deploy-time active VM refresh uses.

The next prevention step is to let owner-scoped recovery fall back to vmctl
refresh for stopped or hibernated current computers when wake/resume fails, and
to allow vmctl refresh to target those states while preserving the persistent
data image.

## Fourth Finding: stopped ownership can outlive the vmmanager instance

After deploying `6ce8526e58d47403ac8b8764ac2ab97f0a955259`, direct Node B
diagnostics against vmctl showed the stopped primary ownership still existed:

- `vm_id: vm-5b0c1bef1e2b6d7f8dad7d0e8473ed19`
- `state: stopped`
- `stopped_by: vmctl-restart`
- stale `sandbox_url: http://10.203.109.2:8085`

But `POST /internal/vmctl/refresh` returned:

```text
failed to refresh VM vm-5b0c1bef1e2b6d7f8dad7d0e8473ed19:
vm vm-5b0c1bef1e2b6d7f8dad7d0e8473ed19 not found
```

The ownership registry and vmmanager had diverged after restart: durable
ownership still pointed at the stopped VM, while the in-memory vmmanager no
longer had an instance to refresh. Accepting `stopped` in `RefreshVMForDesktop`
is therefore not sufficient. Refresh must also handle a missing manager
instance by booting from ownership-derived current deploy config, preserving the
data image and issuing a gateway credential as `startExistingVM` already does.

## Fifth Finding: operator primary VM crash-loop saturates vmctl (2026-06-09 21:45Z)

### Symptom timeline

- Deploy `41aee833` landed at `2026-06-09T21:34:39Z`. Deploy-time active-VM
  refresh rebooted four interactive computers (`vm-301125…`, `vm-61ab545…`,
  `vm-cd22ad17…`, `vm-d067e51c…`) but not the operator primary, which was
  already `state: stopped` / `stopped_by: vmctl-restart`.
- Staging `/health` at `21:45Z` reports `status: degraded`,
  `vmctl_status: unavailable`, and exclusively `bootstrap.resolve` /
  `api.resolve` `resolve_error` (~13–15s bootstrap, ~180s API).
- Operator BIOS shows `COMPUTER BOOT IS STILL PENDING`, bootstrap probes 1–3
  retrying, and `Requesting computer recovery` stalling.
- Node B `curl http://127.0.0.1:8083/health` times out; lookup/refresh also
  time out while resolve retries continue.

### Class test (rule 5)

Journal since `21:35Z` shows **only** user `5bd6de97-3b58-408c-bf89-c42c81b083de`
(operator) failing resolve (14 failures, no other users). Deploy refresh
succeeded for other active primaries. **Fault is single-computer, not platform
class**, though operator browser retries saturate vmctl globally.

### Bottom-up diagnosis

| Layer | Status | Evidence |
|-------|--------|----------|
| Host resources | Healthy | `/` 75% used (120G free), 15Gi RAM available, 1341 open FDs |
| vmctl daemon | Process up, API saturated | PID 1121315 listening `:8083`; `/health` hangs; threads in `do_wait` |
| Operator VM lifecycle | Crash-loop | `vm-5b0c1bef…` ownership `stopped` epoch 159; fc-config epoch 298; no stable firecracker; tap `vm-vm-5b0c1-tap` DOWN |
| Bootstrap probe target | Never reached | Resolve never returns route; probes abort at 15s |
| Proxy route | Symptom only | `resolve_error` / canceled / 180s timeout |

vmctl logs (operator user) repeat:

```text
start existing VM vm-5b0c1bef… failed: cannot be resumed (state=failed);
recovery also failed: guest did not become healthy at http://10.203.*.2:8085 within 2m30s
vmmanager: killing duplicate Firecracker process for VM vm-5b0c1bef… (pid=…)
vmmanager: firecracker process … exited with error: signal: killed
```

### What the four code fix attempts changed (and why they failed here)

1. `678d2df` — proxy refresh after wake when runtime probe fails. **Failed**
   because failure is inside vmctl resolve (no route returned).
2. `7ebd187` — lookup-first recovery + boot UI recovery after pending probes.
   **Failed** because recovery still calls resolve/refresh behind the same
   blocked vmctl mutex and the same `startExistingVM` resume/recover path.
3. `6ce8526` — refresh stopped ownership on recovery. **Failed** because
   refresh never completes: concurrent resolve attempts hold the registry lock
   for 2m30s health waits and stack firecracker start/kill races.
4. `41aee833` — `RefreshVMForDesktop` boots stopped VM when vmmanager instance
   missing. **Not reached** on the product path: `/api/shell/bootstrap` resolve
   still uses `startExistingVM` (resume → recover) for `stopped` ownership, not
   refresh.

### Falsifiable hypothesis

> Operator browser bootstrap retries issue concurrent `resolve` calls for a
> `stopped` primary whose vmmanager instance is `failed`. Each resolve runs
> `startExistingVM` → `RecoverVM`, starts firecracker, then kills it as
> duplicate when another concurrent resolve cleans up. Guest health never
> succeeds; vmctl registry mutex saturates; `/health` reports unavailable and
> all operator bootstrap probes show `resolve_error`.

**Expected observation if true:** after stopping retry traffic, a single
`POST /internal/vmctl/refresh` for user `5bd6de97…` desktop `primary` returns
200 within one guest-ready window, sets ownership `active` with new
`sandbox_url`, and subsequent bootstrap resolves succeed without probe retries.

**Expected observation if false:** refresh returns error or guest health still
fails on a single serialized attempt → inspect guest kernel log / sandbox boot
inside VM (data image corruption or guest image regression).

### Remediation attempt (operator recovery, data-preserving)

vmctl was restarted at `22:00Z`; `/health` returned ok again. A single
serialized `POST /internal/vmctl/refresh` for user `5bd6de97…` desktop
`primary` still failed: guest ping succeeded but `:8085/health` never returned
`ready` within 2m30s.

## Sixth Finding: operator data disk full — sandbox cannot start (2026-06-09 22:10Z)

### Root cause (guest serial console)

Firecracker serial log for `vm-5b0c1bef…` during refresh at `22:02:58Z`:

```text
Starting go-choir Sandbox Runtime (VM guest)...
mkdir: cannot create directory '/mnt/persistent/runtime/.sandbox-next': No space left on device
[FAILED] Failed to start go-choir Sandbox Runtime (VM guest).
```

Guest network comes up; sandbox install/update fails because the mutable data
disk is full. This explains the `:8085` health timeout without data corruption
or routing failure.

### Disk evidence (host read-only mount of `data.img`)

| Path inside guest | Size | Notes |
|-------------------|------|-------|
| Total `data.img` | 7.8G used / 7.8G (100%) | `dataImageSizeMB = 8192` sparse cap |
| `files/` | 3.0G | includes `.choir-tool-cache` (go mod/build caches) |
| `state.vtext/` | 2.5G | Dolt `noms/` is the bulk; possible compaction debt |
| `go/pkg/mod/` | 1.7G | module cache inside persistent disk |
| `go-build-cache/` | 512M | |
| `runtime/` | 231M | current + previous sandbox binaries |

User data (VTexts, mission files) is present and mountable; the failure is
capacity, not missing `data.img`.

### Class comparison: blank account `a@b.com` vs operator

| Field | `a@b.com` (`0e5c45ab…`) | `yusefnathanson@me.com` (`5bd6de97…`) |
|-------|-------------------------|---------------------------------------|
| Created | `2026-06-09T20:33:54Z` | `2026-05-26T08:58:23Z` |
| Published primary VM | `vm-d067e51c…` | `vm-5b0c1bef…` |
| `data.img` size | 327M | 7.9G (100% full) |
| Ownership state | `active` epoch 5 | `stopped` epoch 159 |
| Guest `:8085/health` | `ready` | unreachable (sandbox service failed) |

Blank account boots on the same staging deploy (`41aee833`). Operator failure
is **single-computer disk exhaustion**, not a platform-class regression.

### Belief-state update

The crash-loop / vmctl-saturation hypothesis (Fifth Finding) remains partially
true for concurrent retries, but the **terminal blocker** for operator recovery
is guest disk full. Expanding `data.img`, pruning guest caches (tool-cache,
`go/pkg/mod`, build caches), and/or Dolt maintenance are the realistic recovery
axes. Snapshot `data.img` before any mutation.

### Remaining error field

- ~~Operator computer still not booting; acceptance criteria not met.~~
- ~~No `data.img` snapshot taken yet.~~
- Dolt 2.5G bloat may be a separate compaction bug; deprioritized until disk
  headroom exists — headroom now restored (see Seventh Finding).

## Seventh Finding: operator disk pruned — boot recovered (2026-06-09 22:47Z)

### Actions taken (data-preserving)

1. Snapshot copy:
   `data.img.pre-prune-20260609T224644Z` (8.0G) beside live image.
2. Guest ext4 recovered with `e2fsck -fy` on loop mount.
3. Pruned **rebuildable caches only** (VText/Dolt untouched):
   - `files/.choir-tool-cache` (~2.9G)
   - `go/pkg/mod` (~1.7G)
   - `go-build-cache` (~512M)
4. Guest disk after prune: **2.8G used / 4.7G free** (37% utilization).
   `state.vtext/` remained ~2.5G.

### Recovery proof

Serialized refresh:

```bash
POST http://127.0.0.1:8083/internal/vmctl/refresh
{"user_id":"5bd6de97-3b58-408c-bf89-c42c81b083de","desktop_id":"primary"}
```

Response (22:47Z):

```json
{"vm_id":"vm-5b0c1bef1e2b6d7f8dad7d0e8473ed19","state":"active","sandbox_url":"http://10.203.139.2:8085"}
```

Guest health:

```json
{"status":"ready","service":"sandbox","sandbox_id":"vm-5b0c1bef1e2b6d7f8dad7d0e8473ed19","runtime_health":"ready"}
```

Deploy identity on guest: commit `41aee833`.

### Belief-state update

PROBLEM 0 **resolved** for boot/sandbox readiness. Operator primary computer
runs again with VText store intact. Follow-on missions: Dolt compaction,
proactive `data.img` growth alerts, platform VM disk policy (see community-news
v1 Platform Computer Requirements).
