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
