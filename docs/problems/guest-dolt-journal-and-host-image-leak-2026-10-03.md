# Problem: Guest Dolt Journal Leak + Host Dead-Image Accumulation (Owner Computer Wedged)

**Date:** 2026-10-03
**Found while:** S0m stranded-bound deployed acceptance — owner computer
`computer-03335285269bdba4f94377e56879f9e6` (vm `candidate-fleet-e15cb89f25d963c220319b7b`)
stuck `degraded`, prompt-bar returning `502 failed to resolve user autoputer`,
desk texture activations pinned `pending` (`desk_pending_mutations` climbing
1→64, `running_runs: 0`).

**Mutation class:** `red` — persistent data image surgery + GC policy on a
protected surface (data persistence, VM lifecycle). Break-glass path is the
documented runbook `docs/runbooks/offline-guest-gc.md`.

## Evidence

### Leak A — guest-internal: unbounded noms journal (the active leak)

Read-only host mount of the live `data.img`:

| Path | Size |
|---|---|
| `state.texture/` | **27 GiB** |
| `choir-updater/` | 456 MiB |
| `files/` | 95 MiB |
| everything else | < 1 MiB |

Inside `state.texture`, the `.dolt/noms` **journal alone is ~22 GiB** —
uncommitted chunk garbage. Live store ≈ 5 GiB.

GC disposition written at last boot (`.choir-dolt-gc-disposition.json`):

```json
{"at":"2026-10-03T19:50:43Z","outcome":"skipped_size","used_gib":27,
 "journal_gib":22,"threshold_gib":5,
 "detail":"host-side offline GC required"}
```

Disk: 29.9 GiB used / 33.5 GiB image, 3.5 GiB avail, shrinking ~300 MiB/25min.
Trajectory: ENOSPC inside ~2-4 days at this rate — and the guest already wedges
under write pressure (the desk-mutation backlog observed now).

**Mechanism (ordering defect in `store.MaybeRunDoltGC`,
`internal/store/dolt_maintenance.go`):**

1. Live-store guard fires first — `liveBytes > 5 GiB` → `skipped_size`,
   `return nil` (line ~290).
2. The journal ≥1 GiB trigger (`doltGCJournalGiB`) that would run routine GC is
   evaluated **after** that guard, at line ~310 — unreachable once the live
   store is >5 GiB.
3. The emergency path only fires at `avail ≤ 512 MiB` — with a 22 GiB journal
   and a 33.5 GiB image, ENOSPC is reached while ~3.5 GiB still shows free to
   the OS in practice because the journal is what fills first and Dolt writes
   stall well before the 512 MiB floor.

So once `live > 5 GiB`, **no code path in the guest ever runs GC again** —
journal grows monotonically until ENOSPC. The 2026-09-11 fix made GC
journal-aware but left the journal trigger below the live-guard, so the
guard now permanently suppresses exactly the GC that would shrink the journal.

### Leak B — host-side: orphaned data images, no prune path

`/var/lib/go-choir/vm-state/` on node-b (host at 81%):

| File | Real bytes | Age | Cleanup path |
|---|---|---|---|
| `…e15cb8/data.img.corrupt` | **21 GiB** | Sep 30 | **none** — name not matched |
| `…e15cb8/data.img.quarantine-1-40e7813a` | **15 GiB** | Sep 11 | retained (maxRetained=2; journal `swapped` but it's the only gen-1 quarantine → kept as last rollback) |
| `…d03daca*/data.img.pre-upgrade-…` | 32 GiB apparent | Aug 24 | **none** |
| `…49ee3bd*/data.img.pre-upgrade-…` | 32 GiB apparent | Aug 24 | **none** |
| `…e15cb8/console-b14-serve3.log` | 104 MiB | Aug 27 | **none** |
| `…e15cb8/rec-*.journal` × 9 | ~1 KiB ea | mixed | stale, phases `swapped`/`fenced` |

`node-b-storage-report` classifies **45 GiB** as `manual_recovery_snapshots`
allocated bytes.

`vmmanager.pruneCompletedQuarantines` only matches `data.img.quarantine-N-*`;
`.corrupt` and `.pre-upgrade` images are orphaned by construction — no code
path ever deletes them, and `PruneRecoveryQuarantines` runs only inside the
manual `runReclaimSweep` (called by `node-b-deploy-disk-preflight` and the
`HandleReclaim` endpoint — not periodic).

## Clustering note

This is the **third storage/capacity substrate defect on this host inside a
week** (after `platform-dolt-oom-realization-cluster-2026-10-01.md` —
host platform-dolt RSS — and `texture-revision-read-flap-during-oom`).
Per AGENTS.md Root Cause Clustering: the shared cause is **unbounded
state growth with GC/prune paths that are either unreachable by ordering or
never scheduled**. The substrate fix is not another prune — it's one capacity
contract per store (guest Dolt, host vm-state, platform-dolt) where every
growth path has a *reachable, scheduled* bound.

## Immediate repair (this finding's own boundary)

1. Un-wedge the owner guest now via `docs/runbooks/offline-guest-gc.md`
   (hold → stop → mount rw → `dolt gc` → verify heads → maintenance-serve →
   unhold). Proven twice; expected reclaim ~20+ GiB journal.
2. Reclaim host dead images after owner review (the `.corrupt` is forensic —
   preserve or delete per owner; `pre-upgrade` images are from the Aug 24
   upgrade, stale by 6 weeks).
3. Durable code fix (separate commit, red — protected persistent-state /
   VM-lifecycle surface): surface guest GC disposition to the host and
   schedule offline GC; extend reclaim sweep to `.corrupt`/`.pre-upgrade`;
   schedule `runReclaimSweep` periodically rather than deploy-only.

## Repair executed 2026-10-03 (Leak A, owner guest)

Ran `docs/runbooks/offline-gc.md` break-glass on the held owner VM:

- hold + stop → mounted `data.img` RW → backed up `state.texture` to
  `/var/tmp/texture-backup-pre-gc-2026-10-03` → `dolt gc` → verified heads:
  `computer_event_projection_heads` sequence `404046` and
  `canonical_event_head` `50664b1a…` identical, `computer_event_index` count
  `404046` identical, `dolt log` 2924 commits preserved.
- Result: `state.texture` 27 GiB → 6.7 GiB (noms 23 GiB → ~6 GiB), guest fs
  **89% → 26%**. maintenance-serve → `ready`, `running_runs` recovered
  0→5, routing restored. Unheld; computer `active` epoch 1040.
  Backup retained at `/var/tmp/texture-backup-pre-gc-2026-10-03`.

**Residual:** ~60 `desk_pending_mutations` accumulated during the wedged
window drain slowly; submits return `failed to submit prompt` (HTTP 500)
while the queue clears — the computer is alive, the desk is saturated.

## Open durable work (substrate — belongs to capacity mission)

1. **Guest → host GC signal.** `skipped_size` is written inside the guest
   filesystem only; `probeGuestHealthDetailed` reads `/health` and never sees
   it. A guest whose live set crosses 5 GiB needs an automatic host-side
   offline-GC trigger (surface `gc_disposition` in `/health`, schedule the
   runbook from vmctl) — not an in-guest reorder, which would OOM the guest
   (embedded GC working set scales with the live chunk set).
2. **Host dead-image prune.** `pruneCompletedQuarantines` only matches
   `quarantine-N-*`; extend the reclaim sweep to `.corrupt`/`.pre-upgrade`
   (forensic retention gate) and schedule `runReclaimSweep` periodically.
3. **Stale RO loop devices** pin dead images (found `/dev/loop0`, `/dev/loop3`
4. **Stale MaintenanceHold re-fenced all admission** (found during this
   repair): after hold→stop→GC→maintenance-serve→unhold, the guest kept
   refusing every run admission — `texture prompt bar: submit: computer is
   under maintenance hold: run admission refused`. Cause: I unheld but skipped
   the runbook's final "boot normal" step, so the VM stayed on the
   maintenance-serve boot carrying `choir.runtime_maintenance_hold=1` in
   `fc-config.json`. A `vmctl refresh` (normal boot) cleared it; submits
   returned to HTTP 202. The desk-backlog/pending-run pile I initially read
   as a second defect was hold-refused admission + owner-disabled auto-resume,
   not a dispatch wedge. Runbook already warns about stale
   `MaintenanceHold`; this repair confirms the footgun — worth a vmctl-level
   "unhold implies next boot is normal" assertion or health-surface flag.

## Rollback

Runbook §Rollback: restore the pre-GC `state.texture` backup over the live
texture dir (VM stopped), reboot. Host image deletions are destructive — gate
on owner approval, keep `.corrupt` if the Sep 30 corruption is still under
forensic review.
