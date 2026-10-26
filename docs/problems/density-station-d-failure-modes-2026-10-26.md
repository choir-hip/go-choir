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

### Review 312 (tier 2, both seats): fixed, and named residuals

Fixed on the lane: the legacy 30 minute hold in `hibernationHoldReason` now
folds `guestLastBusyAt` into the idle time it compares, as the plan, the
sweep and reclaim do (Medium); make-room re-checks the start's context
between `pick` and the hibernate (Low 2); a computer that cannot count its
capsules (no computer id or store) omits `live_capsules` from `/health`
instead of publishing 0, so vmctl applies the legacy rule (Low 3);
`VMCTL_IDLE_TIMEOUT_LEGACY_GUEST` is floored at `VMCTL_IDLE_TIMEOUT` (nit).

Residuals, named and not fixed here:

- **R312-1** the 3 s `/health` capsule count cache can report a count that
  predates a capsule opened and passivated inside the TTL (DeepSeek Low 2).
  Narrow (at most 3 s, and a sweep must probe in that window). Fix when
  seen: invalidate on capsule open/close, or do not reuse a cached zero.
- **R312-2** the deferred D4 capture drains only at the sampler pass; with
  reclaim `off` the sampler does not run, so a much later capture records
  the capture-time RSS, and a same-VMID restart with no open reservation can
  be captured as the old window's value (Devin Low 3). Accuracy only.
- **R312-3** the M5 guard sees only open reservations: a newer start that
  ended without capture (`finish(false)`, TTL) while the VM stayed up leaves
  no reservation (DeepSeek Low 4, 305 F3). Overcharge direction, bounded by
  the guest clamp.
- **R312-4** a settled capture within `memoryPeakSaveDelta` of the old value
  is charged in memory but not written, so "the persisted value is the last
  one captured" is not exact (M9 tolerates).
- **R312-5** the recorded make-room decision keeps the first sample's
  numbers beside a refusal reason worded on the latest sample (by design:
  the record is the start's first refused sample).
- **R312-6** a legacy-hold computer is reported under `guest_busy` in
  pressure reclaim, hiding `guest_does_not_report_live_capsules` (Devin
  nit 4). Protective, cosmetic.
- **R312-7** `attemptMeasuredAdmission` takes no context, so a cancel during
  the final sample can still admit (microseconds), and hibernate's own
  re-check is lifecycle and state only: the probe-to-claim window is one
  probe, the same as the sweep (Devin Low 6).
- **R312-8** the paper's `/health` capsule count walks all assignments under
  the engine mutex every 3 s against a 1 s budget; measure on the paper
  before D3 reaches it. A 503 reads as busy (safe) but looks `store_stalled`.
- **R312-9** D4 `wake` charges the previous start's settled RSS; a wake that
  settled low followed by a deploy-voided snapshot and a cold boot
  undercharges. The staging proof must exercise that transition.

## D2. A platform deploy voids every hibernation snapshot

### Problem

Every deploy that runs a NixOS switch builds a new guest image, because the
commit is stamped into it (`buildCommit`, `build.json`, the guest manifest,
the Go `-X` ldflags). The guest is launched from stable paths under
`/var/lib/go-choir/guest`, a symlink the switch repoints to the new image.
A hibernated computer's snapshot therefore fails its launch-config hash and
its bound-file identity at the next wake, and the wake cold boots. In the
week to 2026-10-26, 13 snapshot hibernates produced 2 resumes and 5
`launch_config_mismatch` fallbacks; the real incompatibility is only the
store disk (the guest's page cache and file-backed text describe the old
erofs blocks), and most of the other checks are conservative. Two defects of
today's code ride along (H1, H6 in the design; their own record is
`docs/problems/guest-image-switch-window-kernel-params-and-bound-identity-2026-10-26.md`).
Evidence and the mechanism with citations: design §1.

### Design

[docs/design/density-d2-snapshots-survive-deploys-2026-10-26.md](../design/density-d2-snapshots-survive-deploys-2026-10-26.md)
(tier 2 reviews 306 and 313, answered in its §12). Release directories keep
the old bytes; a snapshot resumes onto its own release when the floor
digest, security epoch and guest ABI are unchanged and the release is
inside the support window, then the computer is updated at its next idle
moment through the existing planned, marker-backed refresh. The switch
(`snapshotReleaseRetain`) stays off until S5 has landed and the deploy-level
floor share (R9) is at least 50%. Slices S1, S1b, S1c are built first; S1b
and S5 repair defects of today's code and land whether or not S6 ever does.

### Failure modes (each pinned by a test or a reading in the design)

| # | Failure | Answer |
|---|---|---|
| M1 | Resume onto a deleted or overwritten release | R1 `release_missing`; bound-file identity (S3 test 5). Retention never deletes a running or young release by any rule, the support-window rule included, and deleting a snapshot's release only cold boots it. |
| M2 | Resume below the floor or the security epoch | R3 and R4, compared at every wake with the current release. S3 test 4. |
| M3 | A behind computer is never refreshed | Every release-bound resume enrolls; the sweep resolves the current release each pass, with a 1-minute sweep and a 2-minute user-idle gate. The entry survives a vmctl restart, not a hibernate; the next wake re-validates and re-enrolls. A computer that never goes idle is the residual in §6.7. The CI refresh at the next deploy also reaches it. Health lists `behind`. |
| M4 | Disk growth from retained releases | Kept: current, previous, young, in use; the cap (8); deletion past the support window. R7 sizes a release before S6. Logged per run. The snapshot budget (`VM_SNAPSHOT_BUDGET_GIB=48`) is separate and unchanged. |
| M5 | Jailer paths | The source is the release path, so the shared copy is stable per release (§4.2). `confinement_mismatch` and the uid checks are kept. S3 test 8. |
| M6 | Snapshot vault | No vault under the jailer (snapshot_lifecycle.go:131); vault metas are counted in `in_use` (S3 test 10). |
| M7 | Credential delivery on resume | Unchanged. An old guest that cannot install the new envelope fails the handshake and cold boots. |
| M8 | A vmctl restart resumes something | Nothing new runs at boot. Reattach reads the launch record. The pending set only asks guests for planned refreshes. |
| M9 | The switch races a launch | Launch resolves the link once and binds the resolved release paths. A release under 2 h old and the previous one are never pruned. |
| M10 | A reattached VM records the wrong release (today's `m.cfg` paths) | §4.3. With no launch record, the next snapshot is legacy, which is safe. S3 test 7. |
| M11 | `kernel-params` goes stale between the NixOS switch and the vmctl restart (H1) | With the switch on, every launch reads `kernel-params` from the resolved release. S3 test 2. |
| M12 | A planned-restart marker is captured in a snapshot, mark-then-fence or fence-then-mark | S5 item 1, both orderings; S6 waits for S5. |
| M13 | Silent host-guest protocol skew | R5 `guest_abi`, plus the fail-closed handshake. The same skew already exists for deferred computers. |
| M14 | Floor list drift: a floor dependency missing from the digest, including the build recipe | `scripts/check-guest-floor-paths` in CI. `flake.nix` and `flake.lock` are listed whole. The `guest-floor-commit-free` flake check. Over-approximation is safe. |
| M15 | CPU features change under a resumed guest | S5 item 2: every core, and an unknown fingerprint refuses a release-bound meta. |
| M16 | Rollback loads wrong bytes | Off: R0 refuses. Revert: the launch hash mismatches (§7 S3 rollback). Neither path loads. |
| M17 | A behind computer is hibernated again before its refresh (idle sweep, make-room) | Allowed: the snapshot names the old release, which stays in use. Each cycle stays bounded by R6 and the max age. |
| M18 | The update restarts a computer in front of its user | The user-idle gate (S4). The CI refresh at deploy time keeps today's semantics for every active computer, unchanged by D2. |
| M19 | A security fix outside the floor ships without an epoch bump | Refused by the CI check (§6.6, S1c) unless a reviewed "no security effect" marker is wrong; then M25. |
| M20 | A hibernate and wake inside the switch-to-restart window load a legacy guest against the new erofs (H6) | S1b: the launch record's `image_target` and `image_moved_since_launch`; kernel parameters read at each launch. Release-bound launches are immune. S1b tests 2 and 4. |
| M21 | A wake validates just before the link moves, loads the old release with `Behind=false`, and is never refreshed | Every release-bound resume enrolls; the sweep resolves the current release itself (§5.1). S4 test 2. |
| M22 | Materializing a release fails the NixOS switch | A separate activation snippet in a subshell that checks every step and always exits 0 (§4.1). S2 test. |
| M23 | R6 refuses snapshots of the current release after a quiet week, or after a rollback makes an old release current again | R6 exempts the current release. S3 test 4. |
| M24 | Retention deletes a vault snapshot's release while the vault is unreadable | The endpoint reports `complete: false`; the script then deletes only past-window releases not running. S2 and S3 tests. |
| M25 | A "no security effect" marker is wrong | A reviewed claim the check cannot verify (§6.6). Bounded as in §6.7: the computer refreshes at its first idle moment (user away 2 minutes, guest not busy); any later deploy's refresh asks again at its first not-busy minute; and a later raise or floor move refuses its snapshot at the next wake. No hard time bound for a computer that is never idle. |

### Protected surfaces, evidence, rollback

- **Protected surfaces:** VM launch and hibernate (vmmanager), snapshot
  validation, the boot path, deploy activation (`nix/node-b.nix`), the CI
  deploy gate (S1c). S1 is orange, S1b and S2 to S5 are red, S1c is yellow.
- **Evidence:** the staging proof is design §9 (P0 baseline, P1 acceptance),
  run by FP0 on d@e or a test account, never b@c, the paper, public_platform
  or the owner's computers.
- **Rollback:** S1, S1b and S1c are `git revert`; S1c alone is "remove the CI
  step". The switch is one let-binding (`snapshotReleaseRetain = false`).

## D6. A guest's page cache never goes back to the host

### Problem

A guest frees clean page cache only under its own memory pressure, and an
8 GiB guest holding 2 GiB of anonymous memory never feels any. Free page
reporting (on for every computer, `VM_BALLOON_FREE_PAGE_REPORTING=1`) and
Firecracker's discard carry freed pages to the host and work (host RSS fell
1,614.7 MiB for 1,288.2 MiB freed, ledger 10-22 21:26, a mixed anonymous and
cache free). What is missing is step 1, the guest freeing its cold cache.
The paper's guest held 4,747 MiB of page cache (4,210 MiB the Dolt store,
4.75 GB inactive against 0.17 GB active) with its VMM at 8,093 MiB; the
owner's computers (premium_always_on) never idle-hibernate, so their cache
never returns either. A hibernate then writes that cache into the snapshot.
No code asks a running guest to free cache outside a capsule's teardown
(`internal/capsule/teardown_reclaim.go`); the balloon is configured
(`amount_mib: 0`, `deflate_on_oom`, free page reporting) and never inflated.

### Design

[density-d6-page-cache-return-2026-10-26.md](../design/density-d6-page-cache-return-2026-10-26.md)
(revision 3, after review 320 and re-review 326). A guest-side, bounded, cold-first
`memory.reclaim` act on the daemon's own cgroup (route
`POST /internal/runtime/memory/cache-return`, host-peer only), triggered by
vmctl: idle (S2), before hibernate (S3), before make-room (S4). One switch,
`VMCTL_GUEST_CACHE_RETURN=off|observe|active`, code default `off`. This
build covers S0 to S2; S3 and S4 wait for the tier 2 re-review.

### Failure modes (each pinned by a test before the code)

- **D6-M1** an act runs while a run, a pending desk mutation, a
  self-development operation or a live capsule is in flight.
- **D6-M2** an act evicts the hot set and runs are slower afterwards.
- **D6-M3** an act reclaims anonymous memory or anything swaps.
- **D6-M4** the write blocks the guest daemon (EAGAIN parking; a write that
  outlasts the budget).
- **D6-M5** memory freed but not returned (reporting not negotiated;
  fragmentation).
- **D6-M6** D4's settled RSS is captured after an act.
- **D6-M7** a Diff becomes incorrect (a new non-resident class, frozen-time
  work, a changed Diff input, `init_on_free` or page poisoning on).
- **D6-M8** the hibernate act fails or delays the hibernate, or a stuck act
  holds the fence (the act is a separate bounded call; failure skips).
- **D6-M9** an old guest without the route is retried every sweep.
- **D6-M10** the idle trigger acts on the same computer every pass.
- **D6-M11** an act races a hibernate, stop or start of the same VM.
- **D6-M12** the paper, a public_platform computer or a held computer is
  acted on before S5.
- **D6-M13** the pressure step counts pending return as room, overruns its
  deadline, or two refused starts squeeze the same computer.
- **D6-M14** a vmctl restart or a guest crash resumes or replays an act.
- **D6-M15** rollback is not exact (with `off`, no guest call; prepare
  unchanged).
- **D6-M16** a typo turns it on.
- **D6-M17** a guest's self-report drives an admission decision.
- **D6-M18** the launch config changes and voids snapshots.
- **D6-M19** the act takes the host credential to a guest.
- **D6-M20** a DAMON-selected computer is acted on and its reading loses
  attribution.
- **D6-M21** the target counts shmem, dirty or writeback pages, so the act
  spins into EAGAIN or does writeback I/O (review 320 F3, DeepSeek F7).
- **D6-M22** the settle exits before the return happens (review 320 F1).
- **D6-M23** the hibernate mode runs on an unfenced guest (bypassing the
  busy check).
- **D6-M24** a candidate with no `ComputerURL` reads as idle (review 320
  DeepSeek F4). The shared probe already fails closed on this (density D3a,
  `dfdef0855`, pinned by `idle_hibernation_capsules_test.go`), so no problem
  record is written; D6's candidate rule keeps its own non-empty-URL check
  as defense in depth.
- **D6-M25** the act writes to a cgroup other than the configured scope, or
  falls back silently when the scope's file is absent (the report's `method`
  and `path` make every fallback visible).
- **D6-M26** the hibernate act and settle push the time from fence close to
  the end of the pause past the 2-minute fence TTL, so the fence lifts
  mid-snapshot (re-review 326; S3 skips the act past a 60 s fence budget).

Slice map: S1 pins M1, M3, M4, M21, M23, M25; S2 pins M1, M6, M9, M10, M11,
M12, M14, M15, M16, M17, M19, M24 (M20 in the nix comment); S3 pins M7, M8,
M15, M18, M22, M23, M26; S4 pins M12, M13.

### Protected surfaces, evidence, rollback

Mutation class: S0 green, S1 orange, S2 red (vmctl lifecycle neighbourhood;
additive, behind one switch). Protected surfaces touched from S2: vmctl's
idle sweep tick and health; never admission, the launch config, the snapshot
meta or the prepare body. Admissible evidence: the unit tests above; on
staging (FP0) the design's section 10 steps 1 and 2, with host RSS and
MemAvailable read from the host, never from the guest's report. Not b@c
until X1, never the owner's computers. Rollback, one act:
`VMCTL_GUEST_CACHE_RETURN=off` in `nix/node-b.nix`; no launch-config byte and
no prepare-body byte changes, so no snapshot is voided and no computer cold
boots. Second rollback: revert the code commits; the guest route left behind
is inert when uncalled.

Heresy delta: discovered, "guest page cache never returns to the host" is
step 1 of the return chain, not steps 2 and 3, which work; the design's
claim that the shared busy probe answered not-busy for an empty
`ComputerURL` did not hold on the build base (D3a already fails closed).
Introduced: a
host-to-guest memory act, bounded and stateless. Repaired: none until the
staging proof.
