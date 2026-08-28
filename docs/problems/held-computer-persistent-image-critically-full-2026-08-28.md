# Persistent Data Image Critically Full After Product-Path Recover

<problem_id: held-computer-persistent-image-critically-full-2026-08-28>
<first_observed: 2026-08-28T06:00Z>
<mutation_class: red>
<deployed_commit: 445c8fc2f4c42060478eb62fca3d8b9caaa1677b>
<affected_surfaces: [internal/persistentdisk/usage.go, internal/vmctl/ownership.go, internal/proxy/compute_status.go, guest data.img]>

## 1. Problem Description

`computer-03335285269bdba4f94377e56879f9e6` recovered on the product path
(`POST /api/computers/{id}/lifecycle/recover`, LifecycleReceipt `01a046ee`,
epoch 804 `stopped` → 805 `active` at 2026-08-28T05:54:09Z) and served three
consecutive matching `/api/shell/bootstrap` 200s plus `/api/compute/status`
`runtime_health=ready` / `researcher_count=3`.

Within ~6 minutes the same computer is `state: degraded`, bootstrap returns
`failed to resolve user autoputer`, and `/api/compute/status` reports
`runtime: null` with warning **`persistent data image is critically full`**.

A restart/recover retry without shrinking or replacing the image is expected
to fail the same way: the guest I/O surface has no host-side headroom.

## 2. Evidence (live, 2026-08-28 UTC, choir.news)

### A. Recover succeeded (host hold + guest fence cleared)
- Deployed commit `445c8fc2` at `2026-08-28T05:52:41Z` (force-deploy CI `33144950319`).
- Recover POST 201 in 12.657s; resulting epoch **805** `active`.
- Bootstrap probes 154ms / 138ms / 416ms, matching `computer_id`.
- At 05:55:17Z compute/status: `runtime.reachable=true`, `runtime_health=ready`,
  `researcher_count=3`, guest `persistent_disk` **source=guest**
  `used_bytes=11759788032` (~11.0 GiB) / `total_bytes=33501757440` (~31.2 GiB)
  `avail_bytes=21741969408`, `used_percent≈35`, `warning=true`, `critical=false`.

### B. Guest became unreachable; host image exceeds cap
- 06:00Z lifecycle/status: `state=degraded`, epoch still **805**.
- compute/status `runtime` omitted (guest health GET failed).
- `persistent_disk` now **source=host**:
  - `used_bytes=105273999360` (~98.1 GiB)
  - `total_bytes`/`cap_bytes=34359738368` (32.0 GiB)
  - `avail_bytes=0`
  - `used_percent≈306.4`
  - `critical=true`
- vmctl marks `degraded` when `CheckHealth` fails on an active VM
  (`ownership.go` lookup health path), not from the disk warning itself.
- **Measurement:** `LookupDataImageStats` sets `FileBytes`/`used_bytes` to
  **allocated blocks of the entire VM state directory** (`vmStateDirUsageBytes`,
  `stat.Blocks*512`), and `CapBytes` to `data.img` virtual size (`os.Stat().Size()`,
  32 GiB). The 98 GiB figure is host VM-dir occupancy (tape, Dolt, overlays,
  `data.img` allocation, logs), not guest-statfs fullness. Guest-visible usage
  at 05:55Z was ~11 GiB of ~31 GiB (`critical=false`). The compute/status
  warning compares those two different quantities, so "data image critically
  full" is not proof the guest ext4 is full — it is proof the host VM dir is
  3× the 32 GiB virtual cap.

### C. Product-path implication
`choir computer recover` / `start` will call `ResolveDesktopContext` and try
to boot this image again. That does not compact the image. File-CAS / Track F
restore is the intended O(delta) recovery for files; it is not yet proven on
this computer. Track M mail drain still terminates at guest `/api/mail/inbound`
and cannot run while the guest is unreachable.

## 3. Required Repair Invariants

1. **Do not treat `degraded` as a hold regression.** Host hold was cleared;
   `RUNTIME_MAINTENANCE_HOLD` was not injected on epoch 805. The new failure
   is guest health after boot, with a critically oversized host image.
2. **Do not SSH-mutate or SQL-empty the computer.** Product-path only.
3. **Do not raise the 32 GiB cap as the first move.** That hides allocation
   growth; the guest FS still reported 11 GiB used.
4. **Do not restart blindly.** First separate guest-ext4 fullness from host
   VM-dir occupancy. If the guest died of tape-open/OOM rather than ENOSPC,
   a start may bring it back (epoch 805 is unheld). If host ENOSPC is real,
   compact/reclaim VM-dir (tape compaction, overlay GC) or File-CAS
   rematerialize; do not raise the 32 GiB cap as the first move.
5. **Acceptance:** computer `active`, guest `runtime_health=ready`, host
   image `critical=false`, two consecutive matching bootstraps, no return to
   `degraded` within a 20s health window.

## 4. Classification & Ceremony

- **Mutation class:** `red` (vmctl health, guest persistent image, restore).
- **Protected surfaces:** guest `data.img`, File-CAS restore, vmctl ownership
  state, compute/status disk authority.
- **Rollback:** leave the computer `degraded` at epoch 805; do not destroy
  the image; File-CAS / snapshots remain the recovery substrate.
- **Heresy:** `discovered` this host-image vs guest-statfs mismatch after
  recover. Not a regression of `eb27cac8` / `445c8fc2`.

## 5. Next Safe Probe

Document this receipt first. Inspect how `fileBytes` is measured
(`persistentDiskFromHostImage` / vmctl data.img stat) and whether Track F
File-CAS restore can rebuild a cap-sized image without copying the 98 GiB
allocation. Do not POST recover/start until that plan exists.

## 6. Start Re-Probe (2026-08-28T06:03Z) — guest still dies ~20s after boot

Product-path `POST .../lifecycle/start` (idempotency
`effects-start-after-degraded-2026-08-28T0605Z`) returned 201 in 12.065s.
LifecycleReceipt `01a046f7-bec5-79e4-b2da-a3a15e8f70f5`: `degraded`/epoch 805
→ `active`/epoch **806**.

Immediate proof the guest is not ENOSPC-dead:
- bootstrap 200 in 128ms, matching `computer_id`
- compute/status `runtime_health=ready`, `researcher_count=3`
- guest `persistent_disk` source=guest used=11.76 GiB / 31.2 GiB (35.1%),
  `critical=false`, warning only (`nearing capacity`)

~20s later: bootstrap 502 `failed to resolve user autoputer` (10.1s),
lifecycle/status `degraded` epoch still 806. Same period as the old ~18s
hold-fatal crash-loop, but **without** `RUNTIME_MAINTENANCE_HOLD` (researchers
were admitted). Next cause is guest process death after store-open / GC /
another `log.Fatalf` — not the host-hold fence and not guest-ext4 fullness.

Host `used_bytes≈98 GiB` vs 32 GiB cap remains a VM-state-dir occupancy
warning and did not block start.

