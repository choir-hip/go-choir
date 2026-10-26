# Density station D: the problem per item and its failure modes, written before each item's code (2026-10-26)

Companion to `docs/problems/start-refused-for-memory-no-reclaim-2026-10-26.md`
(FP0's investigation of the 11:49 refusal; landing with
`docs/definitions/choir-consolidate-and-scale-metamission-2026-10-26.md`).
Goal file: `docs/definitions/choir-density-2026-10-26.md`. Each section is
committed before the tests and code of its item (problem documentation
first). Mutation class of this document: green. Every item it describes is
red.

## D1. A refused start never makes room

### Problem

`admitHostMemoryWith` (`internal/vmctl/host_admission.go`) compares one
MemAvailable sample against `floor + headroom + reserved + need` and returns
a `host_memory_pressure` refusal (503, Retry-After 60). Nothing on that path
hibernates anything. The two paths that hibernate run on their own timers:
the idle sweep (`StopIdleVMs`, 30 minutes, and never a primary computer
while the host is under capacity) and sustained pressure reclaim (only
under the 4779 MiB floor with PSI avg60 over 5.0 for four samples). Between
the floor and the admission line, starts are refused and nothing is
reclaimed, so a person stops a computer by hand. Today 11:49 b@c was 448 MiB
short and FP0 stopped d@e. Fourth recorded recurrence (10-13, 10-22 03:16
and 05:25, 10-26 03:1x).

### Callers

- `startExistingVM` (`ownership.go`): resolve of a stopped or hibernated
  computer, the explicit resume, deploy rescue retries, platform warms. No
  registry lock held; the starter holds its own lifecycle claim.
- `rescueBoot` (`ownership.go`): deploy rescue. No registry lock held.
- the new-computer branch of `resolveDesktopContext` (`ownership.go`):
  calls `admitHostMemoryLocked` **while holding `r.mu`**, before any
  ownership or waiter is registered.

### Design (proposal; the decision recorded in the goal file's `now`)

On a `host_memory_pressure` refusal with `VMCTL_ADMISSION_MAKE_ROOM=active`:

1. Take the single make-room slot (one decider; a second refused start
   waits for the first, then re-admits on a fresh sample before deciding
   anything itself).
2. Rank victims: active, not held, no lifecycle claim, not the starter, not
   a protected class (premium_always_on, critical_protected,
   public_platform), warmness priority at or below the starter's (a test
   start never evicts a primary computer), the pressure-reclaim idle rule
   (a test computer yields to a real one with no idle time; otherwise idle
   at least `VMCTL_PRESSURE_RECLAIM_MIN_IDLE`; a test victim of a test
   starter must be idle past the admission settle window), lowest priority
   first, longest idle first. The guest busy probe (fail closed: only a 200
   `/health` with no runs, no pending desk mutations and no active
   self-development operation is idle) runs last, only on the candidates in
   rank order.
3. If the eligible victims' resident memory (latest RSS observation, else
   the recorded peak) cannot cover the shortfall, refuse at once without
   hibernating anything, naming the shortfall and why each skipped computer
   was skipped.
4. Hibernate victims one at a time through the ordinary hibernate path
   (`hibernateVMForDesktopWithReason`, reason `make_room`), each followed
   by a re-admission on a fresh sample, polled for a bounded settle wait;
   stop at the first admit. At most `VMCTL_ADMISSION_MAKE_ROOM_MAX_VICTIMS`
   (default 3) per start.
5. Record the decision in `admission.recent` (vmctl health):
   `admit_after_make_room` with the hibernated VM ids, or
   `refuse_no_room` with the reason.

The new-computer branch releases `r.mu` before making room, then re-takes
it and re-checks that no other request registered the same computer
meanwhile; if one did, the reservation is released and the resolve starts
over.

### Failure modes (each pinned by a test before the code)

- **M1** a start hibernates a computer that is working (a run, a pending
  desk mutation, a self-development operation): the busy probe must answer
  idle, and an unanswered probe counts as busy.
- **M2** a start hibernates the paper, a public_platform computer, an
  always-on (premium) computer, or a held computer.
- **M3** a start hibernates a computer of a higher class than its own (a
  test start evicting a primary computer).
- **M4** two refused starts both pick the same victim (a double hibernate)
  or both evict for the same shortfall (over-eviction).
- **M5** the victim is the starter itself, or a computer whose own start is
  in flight (booting, or holding a lifecycle claim).
- **M6** make-room evicts when no eligible set could cover the shortfall.
- **M7** make-room hibernates without bound, or a start waits without
  bound for memory to show up.
- **M8** a refusal does not say why nothing was eligible.
- **M9** a race with the idle sweep or pressure reclaim hibernates one
  computer twice or fails the start; a victim the sweep already took is
  skipped and the re-admission re-samples.
- **M10** the new-computer branch holds `r.mu` across a hibernate (every
  lookup stalls for the snapshot write), or mints a second computer for
  the same user after re-taking the lock.
- **M11** the admitted start's reservation is lost or double-counted after
  making room.
- **M12** protected starts change (they are admitted without making room,
  as today), or guest-size mode changes.
- **M13** rollback: `VMCTL_ADMISSION_MAKE_ROOM=off` restores today's
  refusal exactly.
- **M14** restart semantics: make-room runs only inside a start; a vmctl
  restart resumes and hibernates nothing.

Added by review 302 (Devin and DeepSeek on abcf4f577), each pinned by a test
before its code:

- **M15** a victim that became working, held, recently used or unroutable
  between plan time and its own hibernate is still hibernated: the plan is
  re-read from the live registry and the victim probed busy immediately
  before each hibernate.
- **M16** a guest that answers busy at the hibernate fence is killed (the
  kill path) instead of left running; it is now a skip for every caller
  (`docs/problems/busy-guest-hibernate-takes-kill-path-2026-10-26.md`).
- **M17** the victim set is chosen once for the first shortfall, and another
  path's in-flight hibernate counts as this start's room, so a start refuses
  after evicting while eligible victims remain, with false refusal text. Now
  one victim at a time, the shortfall recomputed from a fresh sample after
  each, pending memory only waited for, never counted; a refusal is on a
  fresh sample.
- **M18** a typo or an observation word turns make-room on: the code default
  is off and only `active`, `on`, `enabled`, `true`, `1` enable it.
- **M19** a start superseded after it hibernated victims leaves no record
  (`abandon_make_room` names them).
- **M20** unbounded env overrides (capped at 10 victims and 60 s) and a
  cancelled start that keeps evicting (the start's context stops it and
  reaches the route guard).

### Protected surfaces, evidence, rollback

Protected: vmctl admission, hibernate (VM lifecycle), the idle sweep and
pressure reclaim's shared ranking. Admissible evidence: the unit tests
above for the decision and concurrency; staging (five test computers
started in a row on a host that cannot hold them, no hand stop,
`admission.recent` read from vmctl health) for the claim. Rollback:
`VMCTL_ADMISSION_MAKE_ROOM=off` in `nix/node-b.nix`, or a revert of the
code commit.

### Residuals noticed (not this item)

- Sustained pressure reclaim does not exclude public_platform computers
  (`protectedReclaimReasons` names premium_always_on and critical_protected
  only); the idle sweep does. Make-room excludes all three.
- Protected starts (`admit_protected`) are admitted under the floor without
  making room; the backstop is pressure reclaim.

## D3. Idle hibernation reaching primary computers: stopped at the busy-probe check (2026-10-26)

### Problem

D3 drops `VMCTL_IDLE_TIMEOUT` to 10 minutes and turns off the primary
keepalive, so a primary computer idle for 10 minutes is hibernated. The
owner rule is that a computer with a live engineering capsule is never
hibernated. The build spec asks first whether a live capsule shows in the
three fields vmctl's busy probe reads (`httpGuestBusy`, `internal/vmctl/ownership.go`:
`running_runs`, `desk_pending_mutations`, `selfdev_active_operations` on the
guest's `/health`).

### Finding: it does not always show

- `running_runs` is `Runtime.RunningCount()`, the length of `rt.running`
  (`internal/agentcore/runtime.go`), which holds only runs executing an
  activation. A passivated engineering run keeps its capsule: "a passivated
  run is parked, not dead. It keeps its capsule and round on a live runtime"
  (`assignedEngineeringCapsuleUsable`,
  `internal/agentcore/engineering_assignment_fate.go`, accepting
  `RunPassivated` beside `RunPending` and `RunRunning`). Parked, it is not in
  `rt.running`.
- `selfdev_active_operations` counts executing, accepted, materializing and
  rollback_pending operations only; frozen, verified and awaiting_approval
  are "deliberately excluded" (`activeSelfdevOperations`,
  `internal/agentcore/runtime.go`). An engineering assignment opens its
  operation at its first Freeze (`freezeCapsuleEffectBundle`,
  `internal/agentcore/tools_capsule.go`), so before the first Freeze there
  is no operation, and a frozen candidate awaiting the owner's decision has
  one that does not count.
- The guest already knows the difference: `plannedRestartLiveCapsule`
  (`internal/agentcore/planned_restart.go`) calls a capsule live when the
  assignment is not terminal and its disposition is active, freeze_requested,
  frozen or revoke_requested, and a planned restart refuses on it. It is a
  count (`live_capsules`) in the planned-restart refusal only. `/health`
  does not carry it, and the hibernate fence deliberately does not count
  capsules ("a memory snapshot keeps capsules",
  `prepareHibernate`, `internal/agentcore/hibernate.go`).
- So a computer whose engineer is parked, or whose frozen candidate waits for
  the owner, reads idle to vmctl's probe with a live capsule. The idle sweep
  already exposes this at 30 minutes; D3 would triple its frequency. The
  snapshot does keep the capsule, but every guest-artifact deploy voids the
  snapshot (R1) and the wake is then a crash-style cold boot, which revokes
  the capsule and loses its work
  (`docs/problems/always-busy-computer-never-updates-2026-10-15.md`).

### Decision for the orchestrator

The spec says a new problem doc and a code change to the busy probe before
the timer drops. The worker brief says to stop rather than invent a signal.
D3 is therefore not built. The signal that would be needed: `/health`
reports `live_capsules` (the count `plannedRestartBusy(ctx, true)` already
computes) and `httpGuestBusy` adds it. That is a guest change (agentcore,
`/health`, a guest-artifact deploy) plus a vmctl change, and old guests that
do not report it would read as zero, so the rule must be "a guest that does
not report the field is busy" or the timer drop must follow the guest deploy.
Not built here.

### D3 resumed after the capsule fix (FP0, 2026-10-26)

FP0 took the stop above to `docs/problems/idle-hibernate-blind-to-live-capsules-2026-10-26.md`
and asked for the fix first (D3a): `/health` reports `live_capsules` from the
one planned-restart counter (`liveCapsuleCount`, shared, read through one
flight and a 3 s cache because the count walks the assignments under the
non-cancellable engine mutex), vmctl's busy probe reads it, and a guest whose
`/health` lacks the field keeps the legacy 30 minute idle rule
(`VMCTL_IDLE_TIMEOUT_LEGACY_GUEST`, default 30m) in the idle sweep, pressure
reclaim and make-room; deploy refresh keeps the raw busy probe.

D3 itself (the timer drop in `nix/node-b.nix`) then has these failure modes,
each a test before the config change:

- **D3-M1** a primary computer with a live run, a pending desk mutation or an
  active self-development operation is hibernated (the busy fields).
- **D3-M2** a computer with a live engineering capsule is hibernated (D3a:
  `live_capsules`; the three blind states).
- **D3-M3** a guest that cannot report `live_capsules` gets the short timer
  (the legacy rule), or is held forever (it is not: past the legacy timeout
  it goes).
- **D3-M4** the paper (critical_protected), a public_platform computer or an
  owner's paid (premium_always_on) computer is hibernated by the idle sweep,
  pressure reclaim or make-room. The idle sweep and make-room exclude all
  three; pressure reclaim excluded only premium_always_on and
  critical_protected (`protectedReclaimReasons`). D3 lowers
  `VMCTL_PRESSURE_RECLAIM_MIN_IDLE` to 10m, which would widen that gap, so
  pressure reclaim now excludes public_platform too (reason
  `public_platform`).
- **D3-M5** a held computer is hibernated.
- **D3-M6** a computer is hibernated just after its apply restart (PR-09).
  The API-key touch (gap K) covers key traffic, but a host build of about 14
  minutes with no traffic leaves `LastActiveAt` older than 10 minutes, and the
  freshly booted guest reads idle: at 30 minutes this was rare, at 10 it is
  every long apply. The idle clock must count work: vmctl records the time
  it last saw a guest busy (every probe, so every sweep during the build) and
  idle is measured from the later of that and `LastActiveAt`. Not persisted:
  a vmctl restart forgets it and the guest's own busy fields still fence.
- **D3-M7** a computer driven only by API keys is hibernated while busy (the
  busy fields are the fence, not `LastActiveAt`).
- **D3-M8** the keepalive env default changes silently: the code default of
  `VMCTL_PRIMARY_KEEPALIVE_MODE` stays `under-capacity` (`envOr`); both values
  parse; nix sets `off` explicitly.

Rollback: `VMCTL_PRIMARY_KEEPALIVE_MODE=under-capacity`,
`VMCTL_IDLE_TIMEOUT=30m`, `VMCTL_PRESSURE_RECLAIM_MIN_IDLE=30m` in
`nix/node-b.nix`. The cost until D2 lands: a primary computer woken after a
deploy cold boots (about 10 s, closing open obligations; an idle computer has
none) instead of staying resident.

## D4. Admission charges the last peak, not what a wake settles at

### Problem

`estimateNeedWith` (`internal/vmctl/host_admission.go`) charges a start the
computer's `MemoryPeakRSSBytes`, the highest resident memory of its last
start, clamped to the guest size. For b@c that is 7847 MiB, from one long
run; a snapshot wake lands at about 0.3 to 1.4 GiB and settles near 0.3 to
0.4 GiB (`docs/evidence/s3-snapshot-hibernate-staging-2026-10-14.md`), a cold
boot at 1.15 to 1.37 GiB. At 11:49 the peak made a 448 MiB shortfall
(13650 MiB required against 13202 MiB) that a charge near the settled size
would not have had. An over-charge means refused starts and, with D1, extra
victims hibernated for memory the start never takes.

### Design (the D4 build spec in the goal file)

- Record each start's settled RSS: when a started reservation's settle
  window (5 minutes) ends, the ledger keeps the VM's latest resident reading
  (`outstandingLocked`); `sampleVMMemory` persists it on the ownership as
  `MemorySettledRSSBytes` (`memory_settled_rss_bytes`, with
  `MemorySettledAt`) beside the peak. The capture runs in every mode; only
  the charge is switched.
- `VMCTL_ADMISSION_NEED=wake` charges the settled RSS (at least
  `MinNeedBytes`, at most the guest), `DefaultNeedBytes` when unmeasured,
  never the peak. `peak` (the code default until the staging proof) is
  today's estimate exactly. The peak stays recorded and shown. Health
  `admission.recent` shows `need_source` `settled`.

### Failure modes (each pinned by a test before the code)

- **M1** overcommit: the admitted computer later grows to its peak and the
  host falls under its floor (RES-15). Not closed by this item. The
  backstops are sustained pressure reclaim and D1 make-room on the next
  start; the OOM order is unchanged (ordinary VMs at oom_score_adj 500). The
  proof must show the floor held with `wake` on.
- **M2** a settled RSS taken from a start that failed or was killed inside
  the window: `finish(false)` deletes the reservation, and a VM that stopped
  has no reading, so neither captures.
- **M3** the field missing on an old ownership file reads as unmeasured
  (default need), never as zero need; a JSON round trip of an ownership
  without it.
- **M4** the capture happens only when a started reservation's window ends,
  not before, not for an in-flight reservation, and not twice.
- **M5** a second start of the same VM inside the first's window must not
  have the first's window capture the second's early reading.
- **M6** protected starts are unchanged (admitted, reserving the same need
  as the mode charges).
- **M7** guest-size mode is unchanged (no estimate, no ledger).
- **M8** `VMCTL_ADMISSION_NEED=peak`, an unset or an unknown value restores
  today's estimate exactly.
- **M9** a vmctl restart does not invent a settled value: the ledger is not
  persisted, so a window open at a restart captures nothing; the persisted
  value is the last one captured.
- **M10** a settled value under the 1 GiB minimum charges the minimum; over
  the guest charges the guest.
- **M11** (review 305 F1) a VM killed in the last sampler interval of its
  window keeps its last reading in the ledger until the sampler prunes it, so
  an admission or health read that ends the window first would capture a dead
  VM's reading. An ended window now waits (one entry per VM, memory only) for
  the sampler's pass, which captures only after dropping the readings of VMs
  that are gone. The capture can lag the window end by one sampler interval
  (15 s); a vmctl restart in that gap loses it, as M9 allows.

### Protected surfaces, evidence, rollback

Protected: vmctl admission (`host_admission.go`), the ownership file's
schema (additive, omitempty). Admissible evidence: the unit tests above (the
11:49 numbers); staging, d@e or a test computer woken, `admission.recent`
`need_source` `settled` with the need near its settled RSS, and host
MemAvailable before and after. Not b@c (X1). Rollback:
`VMCTL_ADMISSION_NEED=peak` in `nix/node-b.nix`, or a revert of the code
commit; the extra ownership fields are ignored by older code.
