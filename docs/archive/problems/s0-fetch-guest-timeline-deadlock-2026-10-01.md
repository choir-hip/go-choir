# S0 — vmctl boot-timeline fetch deadlocks the manager lock

**Status:** confirmed on staging, fix in flight
**Found:** 2026-10-01, during S0a post-fix receipt capture; surfaced as a
user-visible outage ("cannot boot, regardless of account")
**Introduced:** `21bbabff` (fetch polling for `runtime_started`)
**Mutation class:** red — vmctl concurrency on the boot path

## Symptom

All VM boots hang in `booting`; `/internal/vmctl/boot-timeline` times out for
every computer while `/health` and `/list` respond normally. Fresh
registrations produce a VMID + epoch but never write `fc-config` or spawn
Firecracker. User report: login OK, computer never boots, any account.

## Root cause (two layers, same code path)

**Layer 1 — self-deadlock.** `fetchGuestBootTimeline`
(`internal/vmmanager/boot_timeline.go`) takes `t.mu` in the tail-out branch
and calls `t.mark("guest_receipt_fetch_done_partial")` inside the critical
section. `mark` → `markDetail` re-acquires `t.mu` — a non-reentrant
`sync.Mutex`. The goroutine parks on itself forever.

**Layer 2 — fetch runs under `m.mu`.** `bootVM` (manager.go ~867) and the
resume path (~1031) call `fetchGuestBootTimeline` while holding the manager
write lock. The tail-out is hit whenever the guest's `runtime_started` mark
lands outside the ~25-attempt fetch window — guaranteed on owner-sized tapes
where post-replay reconcile phases take minutes (observed: `runtime_started`
at 407s into an owner boot; fetch window ~30s). One deadlocked fetch holds
`m.mu` forever: every `bootVM` queues at `m.mu.Lock()`, every
`BootTimelineForVM`/`GetVM`/`checkAllHealth` queues at `m.mu.RLock()`, and
resolves pile up in `resolveDesktopContext`'s `r.mu.Lock()`.

## Evidence

- SIGQUIT dump of vmctl pid 434015 (journal 23:19:03): goroutine 351
  `[sync.Mutex.Lock, 18 minutes]` in `fetchGuestBootTimeline` ← `bootVM`
  ← `RefreshVMWithConfig` ← `HandleRefresh`; ~15 goroutines parked on
  `sync.RWMutex.RLock` (`HandleResolve`, `HandleBootTimeline`,
  `checkAllHealth`, `WarmAlwaysOnDesktops`); goroutine 562 in
  `resolveDesktopContext` `Mutex.Lock`.
- Restarted vmctl (pid 435793) re-booted the owner (FC child 435926); its
  guest reached `runtime_started` at 407544ms — the deadlock recurred, and
  the fresh-registration boot queued behind it with only `epoch` written to
  its state dir.
- Owner guest is healthy on its real tap (`10.200.149.2`): `vocab_fenced`
  10263ms, `applied_rows: 0` — the replayed-predicate fix is verified
  working; the wedge is purely the host-side fetch.

## Fix direction

1. `fetchGuestBootTimeline` tail: read `t.GuestReceipt` under `t.mu`,
   release, then `t.mark(...)` / set `GuestFetchError` — never call `mark`
   while holding `t.mu`.
2. Move `fetchGuestBootTimeline` + `finish` + `persist` outside `m.mu` in
   `bootVM` and the resume path: set `inst.State` under the lock, unlock,
   then do the network fetch. The fetch is observation, not shared-state
   mutation — the same reason `waitForGuestReady` already runs unlocked.

## Panel-adjudicated sibling defects (divergent panel 2026-10-01, 6/9 completed)

The boundary panel was asked to hunt the same risk class across vmctl +
vmmanager. Six panelists (codex, claude, gpt6-sol, gemini38, devin, opencode)
converged on the substrate: **every blocking resource class still runs under
`m.mu` or `r.mu`** — this fix only removed the fetch. Confirmed siblings:

1. `manager.go:646-841` — `bootVM` pre-launch holds `m.mu` across
   `allocateHostPortLocked` (`ip` execs), `createDataImage` (`git clone` +
   `mkfs.ext4 -d` of a repo checkout), `ensureDataImageMinSize` (`resize2fs`),
   `createCredentialDisk` (mkfs), `launchFirecracker` (~20 iptables execs).
   None of these execs carries a context/deadline. One hung exec wedges every
   boot and every m.mu reader — same outage shape as tonight. blocks-all.
   Probable explanation for the stuck fresh VM (23:20 `booting`, epoch
   written, never reached fc-config).
2. `ownership.go:1398,1927,1982,2048` — registry holds `r.mu` while calling
   `mgr.GetVM`/`StopVM`/`HibernateVM`; a manager stall becomes a registry
   stall; every resolve queues. blocks-all.
3. `ownership.go:1456-1481` — a held-computer resolve registers
   `pendingWaiters[key]=nil` and returns; every later resolve waits on a
   channel nobody signals, forever (no reaper). correctness → per-key wedge.
4. `manager.go:938` — `ResumeVM` never takes `lockVMOperation`; concurrent
   resume/resolve sees `Stopped`, `forceCleanup` kills the just-spawned FC.
   correctness (double-boot kill race).
5. `manager.go:1759` — monitor goroutine closes `inst.done` captured by
   reference; resume replaces `done` on the same instance → late monitor
   closes the successor's channel → panic ("close of closed channel").
   crash vector.
6. `manager.go:1260` + `cmd/vmctl/main.go:247` — `CheckHealth`/`GetVM` read
   `inst.State`/`HostURL` after releasing `m.mu`; resume writes them.
   correctness (race; stale HostURL probing).
7. `waitForGuestReady` replay-progress bypasses `BootReadyTimeout` — a guest
   advancing one row per <120s pins a boot forever; no absolute boot ceiling.
   correctness (stuck-boot forever).
8. `ownership.go:1447` — successful resolve writes the full registry to disk
   under `r.mu` on the hot path (`saveLocked` on LastActiveAt). IO stall →
   registry freeze. blocks-all under IO pressure.
9. `vmOps` map grows one mutex per vmID forever; `pendingWaiters` has no
   lease/expiry. leak.
10. Fetch fetch-window: 25 × (15s+0.8s) ≈ 6.5min held under `vmOps` even
    unlocked — same-VM ops still head-of-line blocked for that long after
    the guest is routable. correctness/latency.

Direction the panel converged on: treat `m.mu`/`r.mu` as in-memory CAS only;
all exec/network/IO runs under a per-VM operation record with an epoch fence;
`pendingWaiters` gets an identity + deadline + reaper. That is the S0b/S1
substrate work, not this hotfix.

## Notes for S1/S3

- `runtime_started` arrives late on owner-sized tapes because post-replay
  phases (`passivate_interrupted_activations` 2m8s, desk reconciles, agent
  resumes) run between `lifecycle_reconciled` and `afterReplay`. A fresh
  computer marks it at ~9s; an owner computer at ~400s+. Boot *health* opens
  much earlier — the gate vs the mark are different clocks. S3 should treat
  "healthy" (probe-visible) and "runtime_started" (instrument-visible) as
  separate readiness facts.
- `fetchGuestBootTimeline`'s outer loop also had `maxAttempts 25 × (15s
  client timeout + 800ms sleep)` ≈ up to 6.5min worst case when the guest is
  unreachable, all under `m.mu`. Layer 2 makes that window harmless to other
  boots; a wall-clock deadline would still be tighter (follow-up candidate,
  not in this fix).
