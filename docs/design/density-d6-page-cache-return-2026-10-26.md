# D6 design: guest page cache goes back to the host (2026-10-26)

Station D (density), item D6 of
[choir-density-2026-10-26.md](../definitions/choir-density-2026-10-26.md)
("balloon inflation returns guest page cache", RES-10 step 3, unbuilt,
red). Evidence:
[start-refused-for-memory-no-reclaim-2026-10-26.md](../problems/start-refused-for-memory-no-reclaim-2026-10-26.md) §1, §3, gap 4;
[balloon-stats-unread-no-emergency-inflation-2026-10-21.md](../problems/balloon-stats-unread-no-emergency-inflation-2026-10-21.md);
[guest-damon-reclaim-state-unreported-2026-10-22.md](../problems/guest-damon-reclaim-state-unreported-2026-10-22.md);
[guest-memory-return-outcomes-2026-10-22.md](../reports/guest-memory-return-outcomes-2026-10-22.md);
[lr-guest-memory-return-2026-10-14.md](../reports/lr-guest-memory-return-2026-10-14.md).

- **Revision 2** (2026-10-26), after review 320 (tier 2: Devin, approve;
  DeepSeek v4.1 flash, "direction accepted; do not build S3/S4 as
  written"). Every finding is adjudicated in §13. Rebased on origin/main
  d385633b6, where D3 (idle 10 minutes, keepalive off), D3a (`/health`
  `live_capsules` through a one-flight 3 s cache) and D4 (settled RSS,
  `VMCTL_ADMISSION_NEED=peak` on staging) have landed. The main changes:
  the hibernate act is its own call after the fence, not part of the
  prepare body (F2); the settle waits for a fall, not for stability (F1);
  the act is scoped to the guest daemon's own cgroup by default and sized
  from that cgroup's `memory.stat`; compaction is out; S5 waits for a
  recorded owner decision.
- **Revision 3** (2026-10-26), after re-review 326 (Devin, one seat,
  nothing blocking): the empty-URL premise corrected (fixed by D3a);
  Firecracker pin v1.17.0 and the no-PAGE_POISON fact re-read there; S3
  skips the act past a 60 s fence budget (D6-M26); one status code for
  unanswered store reads; S4 names mode `idle`; the report's `method` and
  `path`. Adjudicated in §13.1.
- **Mutation class of this document:** green. Build slices: S0 green, S1
  orange, S2 red, S3 and S4 red, S5 red.
- **Review tier:** this design was reviewed at tier 2 (320). S1 tier 1
  (guest only, inert until called). S2 tier 1 (additive, one switch,
  one-act rollback, no launch-config byte). **S3 and S4 tier 2**: S3 adds
  a step inside the hibernate path before the pause; S4 adds a step and new
  waits inside D1's make-room slot. S5 tier 2.
- **Owner inputs that bind this design:** guests do not swap ("swapping is
  service degradation. we cant tolerate it", 2026-10-09). "We shouldn't
  need to be doing that manual memory management ... we can virtualize
  their memory" (2026-10-26). Resource use ahead of short-term uptime, with
  a proof and a rollback (CLAUDE.md, 2026-10-22). A computer stays on while
  it is working; crash restarts close work (CLAUDE.md). Always-busy
  computers must still be served (2026-10-15).

## 0. The answer in brief

**What returns page cache.** A page reaches the host again only through
three steps: the guest frees it, the guest kernel reports the free block,
Firecracker discards it. Steps 2 and 3 are built and on for every computer
(free page reporting, `VM_BALLOON_FREE_PAGE_REPORTING=1`), and they return
freed memory: after a capsule burst the host RSS fell 1,614.7 MiB for
1,288.2 MiB the guest freed (ledger 10-22 21:26, outcome C). That burst end
freed a mix of anonymous memory (the job's exit) and the capsule cgroup's
cache (teardown `memory.reclaim`), so it proves the return path, not
reclaimed cache in isolation; S2's proof isolates it (§10). Step 1 is the
gap: a guest frees clean page cache only under its own memory pressure,
and an 8 GiB guest with 2 GiB of anonymous memory never feels any.
DAMON_RECLAIM, the built-in proactive path, freed 31 MiB in 16 idle
minutes on its test computer (ledger 10-22 13:06; its reading is still
owed).

**Recommended mechanism: a guest-side, bounded, cold-first cache return
act.** The guest writes to `memory.reclaim` of its daemon's own cgroup
(where the Dolt store cache is charged), sized from that cgroup's
`inactive_file` less dirty pages and a keep, in 64 MiB steps. vmctl
triggers it over a host-peer guest route. The free page reporting every
guest already has carries the freed pages to the host. The root cgroup is
a later scope option once a reading shows material cache outside the
daemon. Not balloon inflation (§3): it frees the same pages by the same
kernel path but adds a target that persists in Firecracker and in every
snapshot (a second guest-memory state authority beside RES-09's virtio-mem
leases), overshoot lands on the guest's OOM path, and a computer woken
from a snapshot wakes into its inflated balloon. Balloon inflation stays
the conditional S6.

**When it runs.** Three triggers, each its own slice, behind one switch:
1. **Idle** (S2): a running computer idle 3 minutes, its D4 settled RSS
   already captured, guest not busy by its own count. Once per idle
   period; computers the sweep never hibernates are re-armed every 30
   minutes. This is where memory sits today: the owner's computers
   (premium_always_on) never idle-hibernate, and their cache never returns.
2. **Before hibernate** (S3): a separate call after the fence and the
   hotplug unplug, before the Diff decision and the pause; the host waits
   for the VM's RSS to *fall* (or 5 s), and ignores any failure. D5's Diff
   and the Full file's real bytes shrink by the returned cache.
3. **Under host pressure, before make-room** (S4): D1's refused start first
   asks idle running computers to return cache, within a 12 s step-0
   deadline, re-admits on fresh samples, and hibernates a victim only if
   that did not cover the shortfall.

**Always-busy computers** (the paper, public_platform) are excluded from S2
to S4. S5 is gated twice: on the owed DAMON reading (which picks DAMON
boot arguments or a busy-safe mode of this act) and on a recorded owner
decision, because it reclaims on the paper while it works.

**Expected return** (estimates from recorded readings, §6): the paper about
4.5 GiB of its ~8 GiB host RSS (S5 only); each idle always-on computer its
inactive cache, about 1 to 3 GiB (UNVERIFIED split; reading R2); a
hibernate's real snapshot bytes about halved on a typical test computer.
Host-wide, on today's mix, about 2 to 4 GiB without the paper and 6 to 8
GiB with it, of Node B's 31 GiB.

**Switch:** `VMCTL_GUEST_CACHE_RETURN=off|observe|active` (fails closed),
with a selector, a trigger list and a scope. **Rollback, one act:**
`VMCTL_GUEST_CACHE_RETURN=off` and a vmctl restart. No launch-config byte
and no prepare-body byte changes, so no snapshot is voided and no computer
cold boots, either way.

**Owner decision needed:** none for S0 to S4. One for S5: a reclaim on the
paper (critical_protected) while it works. The conservative default is
that S5 does not ship without it (§12).

## 1. What exists today

| Lever | Code | On where | Proven |
|---|---|---|---|
| Balloon device, `amount_mib: 0`, `deflate_on_oom: true`, `free_page_reporting: true` | `internal/vmmanager/balloon.go` `firecrackerBalloonConfig`; in every cold boot's `--config-file` | every computer (`VM_BALLOON_FREE_PAGE_REPORTING=1`, node-b.nix) | outcome C, 10-22 21:26 |
| Balloon statistics | `balloon_stats.go` (`VM_BALLOON_STATS_ONLY`); reader `GuestMemoryStats`; route `GET /internal/vmctl/memory/balloon` | b@c only | inert case and b@c probe PASS (10-24) |
| Free page hinting | never sent (developer preview, documented corruption race) | nowhere | n/a |
| Balloon inflation (`PATCH /balloon`) | **none**: `fcapi.go` has `Balloon` and `BalloonStatistics` (GET) only | nowhere | n/a |
| virtio-mem hotplug and unplug | `memory_hotplug.go`, `unplugBeforeSnapshot` | `VM_MEMORY_HOTPLUG=off` | deleted 2026-10-26 (`6013d77d3`); moved only the hotplug region |
| DAMON_RECLAIM boot arguments | `guest_reclaim.go` (`VM_GUEST_RECLAIM_ONLY`) | two test computers | negative or unread (§2) |
| `memory.reclaim` on a capsule cgroup at teardown | `internal/capsule/teardown_reclaim.go`; raw write `job_memory.go` `writeCgroupFile` (unexported) | every guest (`CHOIR_CAPSULE_TEARDOWN_RECLAIM`, default on) | D3 drill; part of outcome C's burst end |
| Guest memory reader | `internal/guestmem` (meminfo incl. `Active(file)`, `Inactive(file)`, `Shmem`, `Dirty`; memory by cgroup; page cache by file tree) | every guest, read only | SDC RES-07 |
| Guest memory-return report | `internal/server/debug_memreturn.go` on `procmem`: DAMON, reporting order, balloon features, buddyinfo, vmstat | every guest | bunch 87, 102 |
| Hibernate fence | `agentcore/hibernate.go` `prepareHibernate` (closes admission, refuses busy, renews the capability, `sync`); host side `guestHibernatePrepare`, 90 s | every snapshot hibernate | S3 |
| Guest busy reads | `agentcore/api.go` `readHealthStore` (desk pending, selfdev active, `live_capsules` through `health_live_capsules.go`'s one-flight 3 s cache; unanswered is fail closed) and `RunningCount()` | every guest | D3a |

So: **the balloon exists in our launch config, with `deflate_on_oom` and
free page reporting on, never inflated; statistics on for one computer;
hinting never.** No code inflates it and no code asks a running guest to
free cache outside a capsule's teardown. There is no unwired balloon
implementation; `MemoryController.SetBalloonTargetMiB` in
`docs/reports/vm-backend-interface-2026-10-14.md` is documentation only.

## 2. Why guest page cache never returns

- **Step 1 fails.** Clean page cache is not free memory; the kernel
  reclaims it only below its watermarks. The paper's guest reported
  MemAvailable 5.2 to 6.0 GiB the whole time its VMM held 8,093 MiB (10-22
  07:41 to 07:50); its page cache was 4,747 MiB, 4,210 MiB of it the Dolt
  store, 4.75 GB inactive against 0.17 GB active (ledger 10-22 06:25 and
  11:44). Cold cache the guest would give back if asked; nothing asks.
- **Steps 2 and 3 work** for freed memory (outcome C, a mixed anonymous
  and cache free). Whether reclaimed cache alone forms reportable 2 MiB
  blocks at the same ratio is S2's first measured question (§10 step 2);
  a large `freed_not_returned` share is S6's trigger.
- **DAMON** is built and selected on two test computers; its one reading
  showed cache 1,055 to 1,024 MiB in 16 idle minutes and host RSS flat.
  Whether it ran is unknown; its DAMON-on reading is owed.

Consequences: a computer's host RSS rises to its guest size after one big
read or build and stays (b@c's peak 7,847 MiB after capsule builds); a Full
snapshot writes that cache and a Diff (D5) will too; the paper holds about
5 GiB of cold cache forever.

**The guest copy is the expensive copy.** The guest's data disk is a host
file that Firecracker reads with buffered I/O (no `cache_type` in the
drive config, so Firecracker's default; UNVERIFIED that the host page
cache holds the hot part of each `data.img`, reading R3). The host's copy
of a page is clean, shared and counted in MemAvailable; the guest's copy
is anonymous memory to the host and cannot be reclaimed or swapped.
Returning guest cache turns a guest cache hit into a virtio-blk read that
is often a host cache hit. The refault counters measure that price.

## 3. Mechanisms compared

| Mechanism | Frees what | Bounded? | Cold-first? | Needs | State left behind | Verdict |
|---|---|---|---|---|---|---|
| `drop_caches` (guest) | all clean cache, hot included | no | no | guest code | none | fallback only, hibernate mode only, when the scope's `memory.reclaim` is absent |
| **`memory.reclaim` on a guest cgroup** | file pages in LRU order (MGLRU oldest first); no swap, so never anonymous | yes (bytes per write; EAGAIN on partial) | yes | guest code (route), host trigger | none: an act, not a target | **recommended** |
| Balloon inflate (`PATCH /balloon`) | whatever the guest allocator reclaims to satisfy it: cache in LRU order, then anonymous pressure | yes (a target) | until cache runs out | a sizing input, a deflate controller, vmctl-restart reconciliation, one owner of guest memory targets with RES-09 | a target in the VMM and in every snapshot | conditional S6 only |
| Free page hinting | free pages, host-initiated | | | developer preview, corruption race | | rejected |
| virtio-mem unplug | hotplug region only; base memory never | yes | n/a | hotplug on, small-base guests | lease state (RES-09) | not this item; the longer substrate (§12) |
| DAMON_RECLAIM (boot args) | cold regions after `min_age`, quota-bounded, continuous | yes | yes, by access age | boot args (launch-config hash: a cold boot per selected computer) | none | unproven; the S5 candidate for always-busy computers |

Why not the balloon (the item's title names it, so the choice is argued):

1. **Same freeing path, more state.** Inflation asks the guest allocator
   for N pages; it reclaims cache to supply them, in the same LRU order.
   What the balloon adds is a target that lives in the VMM, is saved in
   every snapshot (`target_pages`), must be reconciled after a vmctl crash
   and deflated before a woken computer needs the memory: a second
   guest-memory state authority beside RES-09's leases (standing question
   "single state authority").
2. **Sizing needs an input.** A safe target needs the guest's own
   available memory. Firecracker statistics would provide it but are in
   the launch-config hash (a cold boot per selected computer); a guest
   report could provide it instead (review 320 DeepSeek: this reason is
   weaker than 1, 3 and 4, and is kept only as a cost of the stats route).
3. **Overshoot lands on the OOM path.** A target larger than the cold cache
   squeezes hot cache, then allocations. `deflate_on_oom` returns pages
   only from the OOM path, in small batches.
4. **Snapshots carry it.** A computer hibernated inflated wakes inflated,
   and its first run is squeezed until vmctl deflates.
5. **Its extra power is not yet needed.** Inflation takes scattered 4 KiB
   pages; reporting needs whole 2 MiB blocks. If S2 shows a large
   `freed_not_returned` share, S6 builds the balloon under RES-09's
   single-controller rule.

From source (not observed): with `VIRTIO_BALLOON_F_DEFLATE_ON_OOM`, Linux's
`virtio_balloon` does not call `adjust_managed_page_count` on inflate, so
`MemTotal` would not move (RES-10's open question, for S6). `memory.reclaim`
does not touch `MemTotal`.

## 4. Design: the cache return act

### 4.1 Guest side (S1): the reclaimer and its route

**Where.** Extend the existing read-only package `internal/guestmem` with
a new file (`reclaim.go`); do not create or rename a package. The raw
write is a copy of the idiom in `internal/capsule/job_memory.go`
`writeCgroupFile` (unexported there; copy it rather than export it, so the
capsule package is not touched): `unix.Open(O_WRONLY|O_CLOEXEC|O_NONBLOCK)`,
`unix.Write` retried only on EINTR. `os.WriteFile` must never be used:
it parks forever on EAGAIN (the 10-20 failure 3).

**Scope.** The cgroup the act writes to:
- `daemon` (default): the guest daemon's own cgroup, read from
  `/proc/self/cgroup` (the `0::` line) under `/sys/fs/cgroup`. The Dolt
  store cache, the bulk of every guest's cache, is charged there, because
  the daemon reads the store. Non-root `memory.reclaim` exists since 5.19.
- `root`: `/sys/fs/cgroup`. Root `memory.reclaim` exists on 6.18 (review
  320 DeepSeek, from source: the `reclaim` cftype is not
  `CFTYPE_NOT_ON_ROOT`, `user_proactive_reclaim` has no root check, and a
  root target walks the whole memcg subtree). Used only after reading R2
  shows material cache outside the daemon's cgroup.
Either way the file must exist; if not, the outcome is `unsupported` with
the path (no silent fallback), except in `hibernate` mode (below).

**Size**, from the scope cgroup's `memory.stat` (bytes): `inactive_file`,
`active_file`, `file`, `shmem`, `file_dirty`, `file_writeback`.

```
cold   = min(inactive_file, file - shmem) - file_dirty - file_writeback
idle:       target = cold - 256 MiB
hibernate:  target = cold
busy_cold:  target = min(1 GiB, cold - max(512 MiB, active_file))   (S5 only)
target < 128 MiB  ->  outcome nothing_to_return, no write
```

Linux keeps tmpfs and shmem pages on the anonymous LRU (they are
swap-backed; `folio_is_file_lru` is `!swapbacked`), so `inactive_file`
should already exclude them; the `file - shmem` clamp makes the target
correct even if that is wrong (review 320 F3, §13). Dirty and writeback
pages are subtracted because reclaiming them means writeback I/O in the
act (review 320 DeepSeek F7).

**Swap guard.** Before any write, `/proc/swaps` must list no device; if it
lists one, outcome `swap_present`, no write (pins the owner's no-swap
rule). Each write appends ` swappiness=0` (supported on 6.18, review 320
DeepSeek, source); a kernel that rejects the key (EINVAL) gets the bare
byte count on the next step and the report says so.

**Steps.** 64 MiB per write. Before each step: the time budget, and the
cheap busy check (below). EAGAIN ends the act as `partial`. A single write
cannot be interrupted (the kernel may retry up to 16 passes with LRU
drains), so the budget is checked between steps, and the host's HTTP bound
(10 s) is well above the guest's budget (2 s idle, 3 s hibernate, 1 s
busy_cold) plus one step.

**Run quietly.** The writes run on a locked OS thread at nice 19
(`runtime.LockOSThread`, `unix.Setpriority(unix.PRIO_PROCESS, tid, 19)`),
unlocked and restored at the end. Nice affects CPU only; the memory-side
cost is bounded by the target and the step size.

**No compaction.** The act never writes `/proc/sys/vm/compact_memory` (one
uninterruptible write that can outlast any budget, review 320 Devin F9).
If S2 shows freed pages not forming 2 MiB blocks, compaction or the
balloon is decided then (S6's gate).

**Busy predicate.**
- `idle` mode, before the first step: `RunningCount() > 0`, or
  `readHealthStore(ctx, 1 s)` with any read unanswered, desk pending > 0,
  selfdev active > 0, or `live_capsules` nil or > 0. Any of these is busy:
  409 with outcome `busy` and a `reason` (`running`, `store_unanswered`,
  `desk_pending`, `selfdev_active`, `live_capsules`), no write. An
  unanswered read is the same 409 as any other busy answer; the route has
  no 503. This is exactly `/health`'s view, so the guest and vmctl's probe
  agree.
- `idle` mode, between steps: `RunningCount() > 0` or the hibernate fence
  closed (a hibernate started) stops the act as `stopped_busy`. A new run
  shows in `RunningCount` at once; the store-backed reads are not repeated
  per step.
- `hibernate` mode: accepted **only while the hibernate fence is closed**
  (`rt.hibernate` holds it); otherwise 409 `not_fenced`. The fence already
  proved idle.
- `busy_cold` (S5): no live capsule and no self-development operation in
  `executing`, `accepted`, `materializing` or `rollback_pending` (the
  CLAUDE.md always-busy exclusion), plus the refault guard.

**Refault guard** (recorded in every mode; enforced in `busy_cold`):
`workingset_refault_file` per second over the act and the 60 s before it.

**One act at a time:** a guest mutex; a second call answers 409
`in_progress`.

**Hibernate-mode fallback:** if the scope's `memory.reclaim` is absent, one
`echo 1 > /proc/sys/vm/drop_caches` (the guest is fenced, so the hot set
costs only post-wake refaults). Never in other modes.

**Route.** `POST /internal/runtime/memory/cache-return`, host-peer only
(`requireInternalRuntimeCaller`, as the hibernate routes). Body, decoded
strictly: `{"mode":"idle"|"hibernate"|"busy_cold","scope":"daemon"|"root","dry_run":bool}`.
Answers: 200 with a report; 409 `busy` (with its `reason`, unanswered
store reads included), `in_progress`, `not_fenced`; 400 for a bad body;
403 for a non-host caller. No 503. Report: `outcome` (`returned`,
`partial`, `stopped_busy`, `nothing_to_return`, `unsupported`,
`swap_present`, `refault_stop`), `method` (`memory_reclaim`,
`drop_caches`, `none`) and `path` (the file written, or the
`memory.reclaim` path found absent), so the hibernate-mode fallback is
visible in every report, `mode`, `scope`, `cgroup`, `target_bytes`,
`reclaimed_bytes` (the scope's `memory.stat` `file` before minus after),
`steps`, `ms`, `swappiness_key`, `reporting_negotiated` (existing
memreturn reader), and `before` and `after` blocks: the scope's
`memory.stat` fields above, `/proc/meminfo` `MemFree`, `MemAvailable`,
`AnonPages`, `Shmem`, and `/proc/vmstat` `pgsteal_proactive`,
`workingset_refault_file`, `pgmajfault`, `pgpgout`, and buddyinfo
`reportable_kib`. `dry_run` sizes and reports, writes nothing.

The prepare route and its body are **not** changed.

The guest's numbers are the guest's claim. vmctl never uses them for an
admission decision; only host measurements (RSS, MemAvailable) count.

### 4.2 Host side, idle trigger (S2)

In vmctl, a pass in the idle sweep tick (`VMCTL_IDLE_SWEEP_INTERVAL`:
code default 1 minute, staging 2 minutes), after the sweep's hibernate
decisions for that tick. A computer is a candidate when every rule holds:

- selected by `VMCTL_GUEST_CACHE_RETURN_ONLY`, matched against the
  ownership (computer ids, `class:test`, `class:primary`,
  `class:premium`, `*`; using the ownership's own `WarmnessClass`);
- not `critical_protected` and not `public_platform`;
- has a non-empty `ComputerURL` (defense in depth: the shared probe,
  `httpGuestHealth` in `ownership.go`, already answers busy for an empty
  URL since D3a, dfdef0855; D6 does not call a guest it has no address
  for);
- running and active, not held, no lifecycle claim, not hibernating or
  starting;
- idle at least `VMCTL_GUEST_CACHE_RETURN_IDLE` (3 minutes) by the
  registry clock;
- **D4 has captured its settled RSS for the current boot**: the VM has no
  reservation in the admission ledger's open reservations and none in its
  expired-awaiting-capture set (`outstandingLocked`, `captureExpiredLocked`).
  D4 takes the settled RSS as the VM's latest sampler reading at the first
  sampler pass after `Settle` (5 minutes) has elapsed since the start
  finished; an act before that capture would make D4 under-charge every
  later wake (review 320 Devin F3);
- not acted on since its last activity, or, for a computer the sweep never
  hibernates (premium_always_on), not acted on in the last 30 minutes;
- the guest busy probe answers idle, run last, only for candidates that
  passed the rest.

At most two acts in flight host-wide, one per VM (an in-flight map), each
with a 10 s HTTP bound. A 404 marks the VM `unsupported` until its next
boot. A host timeout clears the in-flight entry and is recorded
`host_timeout`; a still-running guest act answers the next call
`in_progress`.

`observe` sends `dry_run: true`.

### 4.3 Before hibernate (S3)

The hibernate path today: budget check, prepare (fence, 90 s bound), hotplug
unplug, L3a Diff decision, pause, create. S3 inserts, for a VM vmctl marked
eligible (same switch, selector and trigger list; passed as a hibernate
option, never a `VMConfig` or launch-config field):

```
prepare (unchanged) -> unplug (unchanged)
  -> rss0 := VM RSS
  -> POST cache-return {"mode":"hibernate","scope":...}   bound 5 s; any failure: skip
  -> settle (below)
  -> L3a Diff decision -> pause -> create
```

**Settle (review 320 F1).** A fenced guest's RSS is already flat, so
"two close readings" exits before reporting has run (`PAGE_REPORTING_DELAY`
is 2 s). The settle instead waits for a **fall**:

```
R := report.reclaimed_bytes ; if the call failed or R < 128 MiB: no wait
poll RSS every 250 ms
exit "fell"  when rss0 - rss >= R/2 and the last two readings differ by < 16 MiB
exit "cap"   at 5 s
```

Typical exit is 2.5 to 4 s; S3 adds at most 10 s (5 s call, 5 s settle).

**Fence margin (re-review 326).** The guest's fence lifts 2 minutes
(`hibernateFenceTTL`) after it closes, and the host's worst case before
D6 was already 110 s (prepare 90 s, unplug 10 s, pause 10 s). If the fence
lifted before the pause, a run could be admitted mid-snapshot. So the host
records `fenceStart` (the moment it sends prepare, which is no later than
the guest's close) and **skips the act when `now − fenceStart` exceeds
`snapshotCacheReturnFenceBudget` = 60 s** after the unplug, recording
`cache_return skipped=fence_budget`. With the act taken, the worst case is
60 + 5 + 5 + 10 = 80 s from fence start to the end of the pause, 40 s
inside the TTL. The guest is fenced throughout. The timeline gains
`cache_return` (outcome, reclaimed MiB, rss0, rss at exit, settle ms, exit
reason); the hibernate log line gains `cache_return_mib=`, `rss_fell_mib=`
and `settle_ms=`.

**A failed act never fails the hibernate** (review 320 F2): the call is
separate from prepare, so a stuck guest write costs at most its 5 s bound
and the snapshot proceeds as today. A guest act still running when the host
gives up is paused with the VM; that is kernel state the snapshot saves and
the wake continues, which is safe.

### 4.4 Under host pressure, before make-room (S4)

In `makeRoomAndAdmit` (D1), when a start is refused for
`host_memory_pressure` and the `pressure` trigger is on, before planning a
victim, inside D1's `makeRoomMu` slot:

1. Candidates as in 4.2, except the idle floor is
   `VMCTL_GUEST_CACHE_RETURN_PRESSURE_IDLE` (default 1 minute) and the
   30-minute re-arm does not apply. Rank by the host-measured excess,
   `latest RSS − MemorySettledRSSBytes` (D4's field; unmeasured ranks
   last); skip any whose excess is under 256 MiB.
2. **One step-0 deadline, `VMCTL_GUEST_CACHE_RETURN_PRESSURE_BUDGET`
   (default 12 s), covers everything**: the calls, the settles and the
   re-samples (review 320 Devin F6). The start's context also ends it.
3. For each candidate in order, at most 3: call the act with
   `{"mode":"idle"}` (`hibernate` needs a fence; `busy_cold` is S5) (HTTP bound
   `min(10 s, remaining)`), settle as in 4.3 (cap `min(5 s, remaining)`),
   then one admission attempt on a **fresh** MemAvailable sample. Admit
   (`admit_after_cache_return`, with each VM id and its RSS fall in
   `admission.recent`) or go on.
4. **Pending return is waited for, never counted** (D1's M17): the only
   input to admit is a fresh sample. A guest's `reclaimed_bytes` is used
   only to size the settle wait.
5. On deadline or no candidates left, D1's victim plan runs unchanged, on
   its own fresh sample.

Sustained pressure reclaim gets the same step 0 only in a later slice, if
S4's evidence calls for it.

### 4.5 The switch

| Variable | Values | Code default | First staging value |
|---|---|---|---|
| `VMCTL_GUEST_CACHE_RETURN` | `off`, `observe`, `active` (exact words; anything else `off`) | `off` | `observe` |
| `VMCTL_GUEST_CACHE_RETURN_ONLY` | selector (empty: nobody) | empty | d@e's computer and one test computer |
| `VMCTL_GUEST_CACHE_RETURN_TRIGGERS` | subset of `idle,hibernate,pressure` | `idle` | `idle`, then `idle,hibernate`, then all |
| `VMCTL_GUEST_CACHE_RETURN_SCOPE` | `daemon`, `root` | `daemon` | `daemon` |
| `VMCTL_GUEST_CACHE_RETURN_IDLE` | 1m to 9m | 3m | 3m |
| `VMCTL_GUEST_CACHE_RETURN_PRESSURE_IDLE` | 0 to 9m | 1m | 1m |
| `VMCTL_GUEST_CACHE_RETURN_PRESSURE_BUDGET` | 2s to 30s | 12s | 12s |

Rollback, one act: `VMCTL_GUEST_CACHE_RETURN=off` in `nix/node-b.nix`;
vmctl restarts with the deploy. No guest restart, no cold boot, no
snapshot voided: nothing D6 does is in the launch config, the snapshot
meta or the prepare body. Second rollback: revert the code commits; the
guest route left behind is inert when uncalled.

### 4.6 The record

- vmctl journal, one line per act: `guest cache return VM <id>
  trigger=<idle|hibernate|pressure> mode= scope= outcome=
  guest_target_mib= guest_reclaimed_mib= rss_before_mib= rss_after_mib=
  rss_anon_before/after_mib= rss_file_before/after_mib=
  host_avail_before/after_mib= settle_ms= settle_exit= ms=`.
- vmctl health: `memory.cache_return.recent` (ring of 20, the size of
  `admission.recent`) and per-VM `last_cache_return_at`.
- Only host measurements feed decisions.

## 5. Interactions

### 5.1 D5 diff snapshots (correctness)

**L3a.** In a cold-booted Firecracker process guest memory is anonymous and
cannot swap, so a non-resident page was never touched or was discarded,
and reads as zero, as a hole does. D6 adds no new way for a page to become
non-resident: the act frees pages in the guest and the existing reporting
path discards them. A discarded page is free in the guest, and the guest
assumes nothing about a free page's bytes. The modes that do assume free
pages are zero, `init_on_free` and page poisoning, make `virtio_balloon`
refuse reporting unless the device offers `VIRTIO_BALLOON_F_PAGE_POISON`
(`virtballoon_validate`). Pins (review 320 Devin F8, DeepSeek):
1. a vmmanager test: the guest kernel command line never carries
   `init_on_free=1` or `page_poison=1`;
2. reading R1: every running guest's negotiated balloon features show
   reporting on (the memreturn reader), and the guest image manifest's
   `kernel_config` shows `CONFIG_INIT_ON_FREE_DEFAULT_ON` and
   `CONFIG_PAGE_POISONING` not set (read from the manifest, no guest
   shell);
3. Firecracker v1.17.0 (the pin, `flake.nix`) does not offer
   `VIRTIO_BALLOON_F_PAGE_POISON`: its balloon's `avail_features` are
   `VIRTIO_F_VERSION_1` plus, as configured, `DEFLATE_ON_OOM` (2),
   `STATS_VQ` (1), `FREE_PAGE_HINTING` (3) and `FREE_PAGE_REPORTING` (5),
   never bit 4 (`src/vmm/src/devices/virtio/balloon/device.rs` and
   `mod.rs` at tag v1.17.0, read 2026-10-26). So the driver's refusal above
   is the guard. R6 re-reads it at each Firecracker bump.

**Ordering.** The act and its settle run after the unplug and before
`diffSnapshotRefusal` and the pause (§4.3), so nothing runs in frozen time
and no input of the Diff decision changes. Pages being reported at the
pause sit in the balloon's virtqueue, which the snapshot saves: today's
behaviour with reporting on.

**L3b** (Diff of a restored guest, rebased onto the old memory file). A
page discarded after a restore is not resident, the Diff misses it, and the
rebase gives it the base's old bytes. That is correct only because the page
is free in the guest. Reporting already does this; D6 does it more often.
L3b's design must state this argument and carry the same pins. Whether a
discard of file-backed private memory lowers RSS is reading R4; it does not
change correctness.

### 5.2 D4 wake cost

- No act before D4 has captured the VM's settled RSS (§4.2).
- The peak stays recorded; acts lower later samples, not the recorded peak.
- A restored guest's RSS is part file-backed (364 MiB after a wake: 52
  anon, 312 file; s3 evidence 10-14). The file part is host page cache of
  the snapshot memory file, reclaimable and already in MemAvailable; D4's
  `VmRSS` charge slightly over-charges snapshot wakes (the safe direction).
  D6 records `RssAnon` and `RssFile` so a later D4 revision can decide.
- Overcommit: returned memory admits more; regrowing cache takes it back.
  D4's recorded overcommit failure mode, same backstops (sustained pressure
  reclaim, make-room, `oom_score_adj` 500), and S4 adds return before
  evict.

### 5.3 D1 and D3

S4 is step 0 inside D1's slot; D1's rules (one decider, fresh samples,
pending never counted, a cancelled start stops) are unchanged. D3 (landed,
staging `VMCTL_IDLE_TIMEOUT=10m`, `VMCTL_IDLE_TIMEOUT_LEGACY_GUEST=30m`,
keepalive `off`) hibernates idle unprotected computers at 10 minutes. The
idle act at 3 minutes returns cache 7 minutes early for those (minor) and
for the computers D3 never hibernates (premium_always_on: the main
target). The idle pass runs after the sweep's hibernate decisions in the
same tick, never on a computer being hibernated.

### 5.4 A working computer

The guest decides busy itself with `/health`'s own reads plus
`RunningCount`, failing closed (§4.1). A run that arrives mid-act meets at
most one 64 MiB write on a nice-19 thread, then the act stops. A single
write can take longer than its share of the budget (kernel retries, LRU
drains, writeback of pages that became dirty), which is why idle mode
targets only cold, clean pages and acts only after 3 idle minutes. The
refault counters are recorded on every act; S2's proof compares refaults
before and after.

### 5.5 The paper and always-busy computers

Excluded from S2 to S4 by class. S5 is gated:
1. **The DAMON reading** (owed since 10-22) picks the shape. DAMON frees
   cold cache with refaults flat: the paper joins `VM_GUEST_RECLAIM_ONLY`,
   taking effect at its next planned, marker-backed refresh (boot args are
   in the launch-config hash). DAMON does not: the paper gets `busy_cold`
   through the route, pressure trigger only, refault guard on, never
   during an apply or a live capsule. "Planned refresh" is operator
   sequencing (FP0 schedules it), not code.
2. **A recorded owner decision**, because either shape reclaims on the
   paper while it works.
Proof order: a test computer kept busy by a loop, then d@e, then the paper,
with the paper's poll latency and refaults read before and after.

### 5.6 Restart rules

The act is not work in the CLAUDE.md sense: it opens no obligation, writes
nothing durable in the guest, and is idempotent. A vmctl restart resumes
nothing (in-flight map and `last_cache_return_at` are memory only; a
restarted vmctl may act once more on a computer, harmless). A guest crash
mid-act leaves nothing to close. The planned-restart marker is untouched.

### 5.7 Other levers

RES-09 virtio-mem: untouched, and the unplug runs first in the hibernate
path. Balloon statistics: untouched; b@c's reading may be read as a
cross-check (b@c is not a proof computer until X1). DAMON: the two
DAMON-selected test computers stay out of D6's selector.

### 5.8 Security

Host-peer only route. vmctl's call carries no host credential to the guest
(`hostauth.MarkInternal` sends none to a guest address, review 320 Devin).
A guest can refuse or lie; vmctl acts only on host measurements. A density
lever, not an isolation control.

## 6. Metric, baseline and expected return

**Primary metric:** host `MemAvailable`, and per VM the Firecracker
process's `VmRSS`, `RssAnon`, `RssFile`, before each act and at settle
exit. **Proof the act did the freeing:** the guest's `pgsteal_proactive`
delta against the host RSS fall, with no process exit in the window.
**Price:** guest `workingset_refault_file`, `pgmajfault`, `pgpgout` per
minute in the 10 minutes after an act against the 10 before; the first desk
turn latency after an idle act. **Snapshot:** `stat -c '%s %b %B'` of
`snapshot/mem`, the hibernate line's `disk_bytes`, `snapshot=` ms, the
wake's settled RSS.

Baseline (recorded):

| Reading | Value | Source |
|---|---|---|
| Paper host RSS / guest cache / inactive vs active file | 8,093 MiB / 4,747 MiB (4,210 MiB Dolt store) / 4.75 GB vs 0.17 GB | balloon-stats doc 10-22 07:50; ledger 10-22 06:25, 11:44 |
| Paper guest MemAvailable while its VMM held 8 GiB | 5.2 to 6.0 GiB | balloon-stats doc |
| Owner's computer, another user's, candidate VM, test computer | 3,930 / 1,330 / 2,177 / 1,618 MiB host RSS | balloon-stats doc 10-22 07:50 |
| Test computer 71afbf14: guest in use / cache / host RSS | 820 / 631 / 1,347 MiB | ledger 10-22 12:25 |
| Return ratio, mixed anonymous and cache free | host −1,614.7 MiB for guest +1,288.2 MiB | ledger 10-22 21:26 |
| b@c last peak vs wake cost | 7,847 MiB vs 0.3 to 1.4 GiB | start-refused doc §3; s3 evidence |
| Full snapshot real bytes and time | 0.74 to 1.5 GB (outliers 2.4, 4.0 GB); 5.6 to 10.4 s | start-refused doc §6.4 |
| Wake RSS (snapshot) | 364 MiB (52 anon, 312 file) | s3 evidence 10-14 |

Expected return (estimates; S2's observe readings confirm them before any
`active`):

- **Per act:** host RSS falls by about 0.8 to 1.25 times the guest's
  reclaimed bytes within 2 to 4 s, if reclaimed cache reports like the
  mixed free did (S2 step 2 measures it).
- **Idle always-on computers:** their cold cache less 256 MiB; the owner's
  3,930 MiB computer is the type case, split unknown (R2). Estimate 1 to 3
  GiB each.
- **The paper (S5):** about 4.5 GiB, host RSS from about 8 GiB to about
  3.3 GiB.
- **Hibernate (S3):** on a 71afbf14-like computer, a Diff of about the
  in-use 0.2 to 0.4 GiB instead of about 1 GiB; a Full file's real bytes
  fall by about the same.
- **Admission:** D6 lowers no charge; it raises MemAvailable. The 11:49
  case (448 MiB short) would admit after one idle always-on computer's
  return.
- **Host-wide on today's mix:** about 2 to 4 GiB without the paper, 6 to 8
  GiB with it.

## 7. Failure modes (catalogue; each slice in §9 names the ones it pins)

- **D6-M1** an act runs while a run, a pending desk mutation, a
  self-development operation or a live capsule is in flight.
- **D6-M2** an act evicts the hot set and runs are slower afterwards.
- **D6-M3** an act reclaims anonymous memory or anything swaps.
- **D6-M4** the write blocks the guest daemon (EAGAIN parking; a write
  that outlasts the budget).
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
- **D6-M24** a candidate with no `ComputerURL` is acted on (fixed in the
  shared probe by D3a, which answers busy for an empty URL; D6 keeps its
  own non-empty-URL rule as defense in depth).
- **D6-M25** the act writes to a cgroup other than the configured scope, or
  falls back silently when the scope's file is absent (the report's
  `method` and `path` make every fallback visible).
- **D6-M26** the hibernate act and settle push the time from fence close to
  the end of the pause past the 2-minute fence TTL, so the fence lifts
  mid-snapshot (re-review 326).

## 8. Tests-first rules for the builder

Every slice: commit its failure modes' tests first (yellow, "red until the
code"), each test failing on the slice's base for the reason named; then
the code. Focused runs only: `go test ./internal/guestmem -run <Names>`,
`go test ./internal/agentcore -run <Names>`, `go test ./internal/vmctl -run
<Names>`, `go test ./internal/vmmanager -run <Names>`, `go test ./cmd/vmctl
-run Env`; `-race` for anything with goroutines. vmctl traps from the
density goal file apply: the idle sweep's `checkIdleOwnerships` measures
idle against `time.Now()`, not the registry clock; `NewOwnershipRegistry`
normalizes an empty keepalive to `off`; filter candidates before any HTTP
probe; make-room needs `SetMakeRoomRouteGuard` in tests; nothing that
calls a guest or hibernates may hold `r.mu`.

## 9. Slices (each sized for one Sonnet builder)

### S0: problem record and readings (green)

- Add the D6 section to
  `docs/problems/density-station-d-failure-modes-2026-10-26.md` (Problem,
  Design pointer to this file, D6-M1..M26, protected surfaces, evidence,
  rollback). Commit alone.
- FP0 takes readings R1 to R6 (§11) when convenient; S1 and S2 do not wait
  for them, S2's `active` does.

### S1: the guest reclaimer and route (orange, tier 1)

Files: `internal/guestmem/reclaim.go` (+ `_linux.go`/`_other.go` split if
needed for `unix` calls), `internal/guestmem/reclaim_test.go`;
`internal/agentcore/memory_cache_return.go` (handler), its test, and one
line in `internal/apihandler/routes.go`.

Failure list and tests (all with fake `/proc`, fake cgroup roots and an
injectable writer; no real cgroup writes in tests):
- **M21** sizing: a fixture `memory.stat` where `shmem` > 0, `file_dirty`
  > 0, `file_writeback` > 0: the target equals the formula in §4.1 for
  each mode; `inactive_file` larger than `file − shmem` takes the clamp;
  under 128 MiB is `nothing_to_return` with zero writes.
- **M1** `idle`: busy when `RunningCount` > 0, when any store read is
  unanswered, when desk pending, selfdev active or live capsules > 0, and
  when live capsules is nil: zero writes, outcome `busy`. A run that
  starts between step 2 and step 3 stops the act after step 2
  (`stopped_busy`); a fence closed mid-act stops it the same way.
- **M23** `hibernate` with the fence open: 409 `not_fenced`, zero writes;
  with it closed: the act runs.
- **M4** a fake writer returning EAGAIN on step 2: outcome `partial`, the
  call returns (with a test deadline: the test-hang-guard pattern); a fake
  writer that sleeps past the budget: no further step is started.
- **M3** `/proc/swaps` with a device: outcome `swap_present`, zero writes.
  Every write carries ` swappiness=0`; an EINVAL on the first write makes
  later writes bare and sets `swappiness_key=false`.
- **M25** scope `daemon` writes only to the cgroup named by a fixture
  `/proc/self/cgroup`; scope `root` only to the root; a missing
  `memory.reclaim` gives `unsupported` with `method: none` and `path`
  naming the absent file in `idle` mode, and one `drop_caches` write with
  `method: drop_caches` in `hibernate` mode; a normal act reports
  `method: memory_reclaim` and the scope's file as `path`.
- One act at a time: a second concurrent call answers `in_progress`
  (`-race`).
- `dry_run` writes nothing and reports the target.
- Route: a non-host caller gets 403; an unknown body field gets 400;
  unanswered store reads get 409 `busy` with reason `store_unanswered`
  (never 503).
- The prepare route is unchanged (an existing prepare test still passes;
  no new test needed).

Ships in the guest image; inert until vmctl calls it. Until D2 lands, each
guest-artifact deploy already voids snapshots, so S1 adds no extra cold
boot. Rollback: revert.

### S2: the vmctl idle trigger and record (red, tier 1)

Files: `internal/vmctl/guest_cache_return.go` (config, candidate rule,
client, ring), its test; the sweep hook in `ownership.go` (after the
hibernate decisions of a tick); health in `handlers.go`;
`cmd/vmctl/main.go` env and `cmd/vmctl/env_config_test.go`;
`nix/node-b.nix` (`observe`, selector d@e and one test computer).

Failure list and tests:
- **M16** parsing: `active`, `observe`, `off`, `Active`, `on`, `true`,
  `actve`: only the exact words enable; selector, triggers, scope and the
  three durations parse and clamp (`env_config_test.go`).
- **M1, M6, M10, M12, M24** `cacheReturnCandidates` as a pure function
  (pass `now`): ownerships of each class, held and not, busy and not, idle
  2 and 4 minutes, with an open reservation, with an expired reservation
  awaiting capture, with the settled RSS captured, acted on since last
  activity or not, premium at 29 and 31 minutes since its last act, empty
  `ComputerURL`: exactly the right ones are candidates, and the probe is
  called only for candidates that passed every other rule.
- **M15** `off`: an httptest guest that fails the test if called is never
  called across a full sweep tick.
- `observe`: the request body has `dry_run: true`.
- **M9** a 404 marks `unsupported`; the next tick does not call that VM
  again; a new boot clears it.
- **M19** the request carries no credential header.
- **M11** a VM with a lifecycle claim or a hibernate in flight is skipped;
  at most two acts in flight host-wide and one per VM (`-race`).
- **M17** the decision code never reads the guest's report fields (a
  guest that reports 4 GiB reclaimed while the fake RSS does not move
  changes nothing but the record).
- **M14** a new registry (a vmctl restart) has no in-flight or
  last-act state and performs no act at construction.
- **M20** documented in the nix comment beside the selector: the two
  DAMON-selected computers are not named.

Evidence (FP0): §10 steps 1 and 2. Rollback: `VMCTL_GUEST_CACHE_RETURN=off`.

### S3: before hibernate (red, tier 2)

Files: `internal/vmmanager/snapshot_lifecycle.go` (`captureSnapshotInto`:
the call and settle between the unplug and the L3a decision), a hibernate
option carried from vmctl (`HibernateVM` gains an options variant; the
existing signature stays for other callers), `internal/vmmanager`
tests; the vmctl side passes the option from the same switch and
selector.

Failure list and tests:
- **M22** settle: a fake RSS reader flat for 3 s then falling by R/2: the
  settle does not exit before the fall and exits `fell` after it; flat for
  5 s: exits `cap` at 5 s; R < 128 MiB: no wait.
- **M8** a guest that answers 409 or 500, times out (bound 5 s with a fake slow
  server) or returns garbage: the snapshot is taken exactly as without the
  option, and the timeline records the failure.
- **M7** order: the L3a decision runs after the settle (a fake decision
  recorder sees it after the act); the act runs after the unplug; nothing
  of the act runs after `Pause`. The guest kernel command line built by
  vmmanager never contains `init_on_free=1` or `page_poison=1`.
- **M18** `fc-config.json` is byte-identical with the option on and off;
  the prepare body is byte-identical (`{}`) with the option on and off.
- **M15** option off: no cache-return call is made.
- **M23** the call is made only after a successful prepare (a refused
  prepare makes no call).
- **M26** fence margin: with a fake clock, a prepare plus unplug that took
  61 s makes no call and records `skipped=fence_budget`; one that took
  59 s makes the call; in both cases the snapshot proceeds.

Evidence (FP0): §10 step 3. Rollback: remove `hibernate` from the trigger
list, or `off`.

### S4: the pressure step in make-room (red, tier 2)

Files: `internal/vmctl/host_admission_make_room.go` (step 0 before
`planMakeRoom`), `internal/vmctl/host_admission_make_room_test.go` with
`newMakeRoomFixture`.

Failure list and tests:
- A shortfall a return covers: admitted with no hibernate, decision
  `admit_after_cache_return` naming the VM and its RSS fall.
- **M13** pending not counted: the fake guest reports 2 GiB reclaimed, the
  fake RSS and MemAvailable do not move: no admit from step 0; D1's plan
  runs. Deadline: fake acts that each take 5 s with a 12 s budget: at most
  two acts start, and step 0 ends by the deadline. Cancelled start: step 0
  stops at once and hibernates nothing. Two refused starts at once: their
  step 0s serialize (`-race`) and no VM receives two acts.
- **M12** paper, public_platform and held computers are never step-0
  candidates; premium_always_on is.
- Trigger list without `pressure`: D1's existing tests pass unchanged and
  no guest is called.
- Ranking: excess under 256 MiB is skipped; unmeasured settled RSS ranks
  last.

Evidence (FP0): §10 step 4. Rollback: remove `pressure` from the trigger
list, or `off`.

### S5: always-busy computers (red, tier 2; gated)

Gates: the DAMON reading (R5) and a recorded owner decision (§12). Then
either a nix change (the paper into `VM_GUEST_RECLAIM_ONLY`, at a planned
refresh) or `busy_cold` with the refault guard (threshold set from S2's
readings), with its own failure list written then. The paper last.

### S6: balloon inflation (red, tier 2; conditional, not planned)

Only if S2 shows `freed_not_returned` above about a quarter of freed bytes
(guest MemFree up, `reportable_kib` small, host RSS flat). Built under
RES-09's single-controller rule. Compaction is weighed at the same gate.

D7 (the density bench) records D6's numbers with the rest.

## 10. Proof on staging (FP0; never b@c until X1; never probe the owner's computers)

1. Deploy S1 and S2 with `observe` on d@e and one test computer. Read
   `memory.cache_return.recent`: targets, scope cgroup, `swappiness_key`,
   `reporting_negotiated`.
2. Flip to `active` for the same two. For each act: the journal line,
   `RssAnon`/`RssFile` before and after, host MemAvailable before and
   after, the guest's `pgsteal_proactive` delta (must be at least the
   reported reclaimed bytes, or the act did not free what it claims), and
   refaults over 10 minutes (`scripts/fp8_memory_return_probe.py --burst
   none`). Pass: host RSS falls by at least 70% of `guest_reclaimed_mib`
   within 10 s with no process exit in the window; refaults per minute
   after within twice the before figure; the next desk turn answers
   normally. A fall under 30% is outcome B: record it, stop, go to S6's
   gate.
3. S3 on d@e (once D5 step 1 gives it Diff): hibernate after an idle act;
   record `snapshot/mem` real bytes, `snapshot=` ms, the settle exit and
   ms, wake time and settled RSS; compare with d@e's last hibernate without
   S3. Pass: settle exits `fell`, wake under 1 s, real bytes down, the L3a
   tmpfs sha256 check unchanged.
4. S4: the station's five-test-computer start with an idle always-on test
   computer holding cold cache; read `admission.recent` for
   `admit_after_cache_return`.
5. Rollback check: `off`, deploy, no call in the journal.

## 11. Readings (FP0, Node B, read only)

- **R1. Reporting and free-page assumptions:** the memreturn report's
  `balloon.reporting` and `page_reporting.registered` for each running
  computer; the guest image manifest's `kernel_config` for
  `CONFIG_INIT_ON_FREE_DEFAULT_ON` and `CONFIG_PAGE_POISONING`.
- **R2. The cold share:** after S1, a `dry_run` on d@e and one test
  computer, scope `daemon` and `root`, idle and busy: how much cache is
  outside the daemon's cgroup (decides whether `root` is ever needed).
- **R3. Host page cache of a guest's disk:** `fincore
  /var/lib/go-choir/vm-state/<vm>/data.img` for d@e.
- **R4. Restored-guest discard:** on a computer woken from a snapshot,
  `RssAnon` and `RssFile` before and after an act.
- **R5. The DAMON-on reading** owed since 10-22 (S5's gate).
- **R6. Firecracker balloon features, at each pinned tag:** the feature
  bits the device offers (source read): no `VIRTIO_BALLOON_F_PAGE_POISON`.
  Done for v1.17.0 on 2026-10-26 (§5.1 pin 3); repeat at every bump.

## 12. Heresy delta, conjecture, owner questions

**Discovered:**
- The item's name points at steps 2 and 3 of the return chain, which work;
  the gap is step 1.
- RES-10's `MemTotal` question is answered from source (no change with
  `deflate_on_oom`).
- A snapshot wake's RSS is mostly host-reclaimable file pages; D4's
  `VmRSS` charge over-counts restored guests (recorded, no change).
- L3b's correctness argument must cover pages discarded after a restore.
- Guest tmpfs never returns without swap (X1 fills guest `/tmp`).

**Introduced:** a host-to-guest memory act (a host-peer route), bounded and
stateless by design.

**Repaired (on proof):** "guest page cache never returns to the host" for
selected computers; the paper only at S5.

**Conjecture C-D6.** If an idle, unprotected computer returns its cold
guest file pages through `memory.reclaim` on the daemon's cgroup and free
page reporting, then its host RSS falls by at least 70% of the bytes the
guest reclaimed within 10 s, with refaults over the next 10 minutes no more
than twice the idle baseline, and a following Diff shrinks by about the
same bytes. Test: §10 steps 2 and 3. Edge: missing oracle (refault price
under real use); delta_o: the per-act record carries refaults and
`pgsteal_proactive` before and after.

**The longer substrate.** "Virtualize their memory" points past D6 to
guests that boot small and grow by plugging memory (virtio-mem, RES-09), so
the host decides residency and unplug forces whole blocks back. D6 is the
cheap step that works with today's memory shape and does not block that.

**Owner question (S5 only; does not block S0 to S4).** May the paper, a
critical_protected computer that is always working, have its cold page
cache reclaimed while it works, under host pressure, with a refault guard
and a one-act rollback? Default until answered: no; S5 does not ship. The
balloon-versus-reclaim choice is a design branch, taken by the
conservative option (no new device, no new state) and stated, per "No
Blocking Asks".

## 13. Review 320 adjudication

Seats: Devin (approve; F1 to F9) and DeepSeek v4.1 flash (direction
accepted, S3/S4 not as written; F1 to F8, answers, dissent). FP0's summary
labels in brackets.

| Finding | Seat | Disposition | Where |
|---|---|---|---|
| Settle exits before the return: a fenced guest's RSS is already stable [F1 HIGH] | DeepSeek F1 | **Adopted.** Settle waits for a fall of R/2 then stability, cap 5 s; S4 admits only on fresh samples after each settle | §4.3, §4.4, S3 test M22 |
| Hibernate act rides the 90 s prepare body and can fail the hibernate; act ran before the unplug [F2 HIGH] | DeepSeek F2 | **Adopted.** Separate call after prepare and unplug, bound 5 s, failure skips; prepare body unchanged; hibernate mode only while fenced | §4.1, §4.3, S3 tests M8, M18, M23 |
| `Inactive(file)` includes shmem [F3 MED-HIGH] | DeepSeek F3, Devin F1 | **Adopted as a clamp; premise disputed.** Linux keeps shmem on the anonymous LRU (swap-backed), so `inactive_file` should exclude it; the target nonetheless takes `min(inactive_file, file − shmem)`, correct either way. Sizing moved to the scope cgroup's `memory.stat` | §4.1, S1 test M21 |
| Busy probe not fail-closed for an empty `ComputerURL` | DeepSeek F4 | **Premise false; already fixed by D3a** (dfdef0855: `httpGuestHealth` answers busy for an empty URL; re-review 326). D6 keeps the non-empty-URL candidate rule as defense in depth; no problem record | §4.2, M24 |
| The "one-flight 3 s cache" on `plannedRestartBusy` does not exist [nit] | Devin F5, DeepSeek F5 | **Adopted.** The cache that exists (D3a, `health_live_capsules.go`) serves `readHealthStore`; the act's predicate is `readHealthStore` plus `RunningCount`, not `plannedRestartBusy` | §1, §4.1 |
| `internal/guestmem` exists; `writeCgroupFile` is unexported | DeepSeek F6 | **Adopted.** Extend `internal/guestmem`; copy the raw-write idiom | §4.1, S1 |
| Budget is advisory; idle reclaim can write back dirty pages; host bound vs guest worst case | DeepSeek F7 | **Adopted.** Dirty and writeback subtracted; 64 MiB steps; `pgpgout` recorded; host bound 10 s over a 2 to 3 s guest budget; host timeout recorded | §4.1, §4.2, M21 |
| Facts: idle 30 m not 10 m; sweep default 1 m; tier of S2 to S4; swappiness=0 supported | DeepSeek F8, Devin F4 | **Adopted.** Rebased: D3 landed with `VMCTL_IDLE_TIMEOUT=10m` (legacy guests 30m); sweep default 1 m, staging 2 m; S3 and S4 tier 2; `swappiness=0` sent with an EINVAL fallback; empty `/proc/swaps` guard added | header, §4.1, §4.2, §5.3 |
| D4's mechanism misstated [nit] | Devin F3 | **Adopted.** D4 (now landed) captures settled RSS at the first sampler pass after `Settle` from the start's finish; D6 waits until the VM has no open or awaiting-capture reservation | §4.2, §5.2, S2 test M6 |
| S4's 6 s budget does not reconcile with the bounds [nit] | Devin F6 | **Adopted.** One 12 s step-0 deadline covers calls, settles and samples; per-call and per-settle bounds are clipped to what remains | §4.4, S4 tests |
| Outcome C attribution [nit] | Devin F7 | **Adopted.** Restated as a mixed anonymous and cache free; S2's proof isolates reclaimed cache with `pgsteal_proactive` and no process exit | §0, §2, §10 step 2 |
| M7 pins miss config defaults [nit] | Devin F8, DeepSeek answers | **Adopted.** Three pins: command-line test, manifest `kernel_config` and negotiated features (R1), Firecracker offers no PAGE_POISON (R6) | §5.1 |
| `compact_memory` is not interruptible [nit] | Devin F9 | **Adopted by removal.** No compaction in S1 to S4; weighed at S6's gate | §4.1, S6 |
| Root `memory.reclaim` on 6.18 unverified | Devin F2 | **Adopted.** Closed from source (DeepSeek); default scope is the daemon's cgroup anyway; `dry_run` reports the file | §4.1, R2 |
| Balloon reason 2 overstated | DeepSeek answers | **Adopted.** Softened; reasons 1, 3, 4 carry the rejection | §3 |
| Paper ratification should be the owner's explicit call | DeepSeek answers | **Adopted.** S5 ships only after a recorded owner decision; default no | §5.5, §12 |
| Add a proof that `pgsteal_proactive` moves after a write | DeepSeek confidence note | **Adopted.** | §10 step 2 |
| Target the daemon cgroup rather than root first | DeepSeek dissent | **Adopted.** `VMCTL_GUEST_CACHE_RETURN_SCOPE=daemon` default; root after R2 | §4.1, §4.5 |
| Kill S3 outright | DeepSeek dissent | **Rejected.** With F1 and F2 fixed it adds at most 10 s before a pause (5 s call, 5 s settle) and only when the guest reclaimed at least 128 MiB, and it shrinks D5's Diff and the Full file's real bytes; S3 is its own trigger and can stay off | §4.3, §4.5 |
| Prefer the balloon (guest-independent, beats fragmentation) | DeepSeek dissent | **Rejected for now; S6 is its successor** if outcome B appears; a hostile guest can refuse either mechanism | §3, S6 |
| "Planned refresh" for the paper is operator sequencing, not code | DeepSeek answers | **Adopted as stated** | §5.5 |

### 13.1 Re-review 326 adjudication (Devin, on 7910eb8af)

| Finding | Severity | Disposition | Where |
|---|---|---|---|
| F4's premise is false: `httpGuestHealth` already answers busy for an empty `ComputerURL` (D3a, dfdef0855) | MED (doc) | **Adopted.** S0's problem-record bullet dropped; §4.2, M24, §12 and §13 reworded to "fixed by D3a; D6 keeps the URL rule as defense in depth" | §4.2, §7 M24, §9 S0, §12, §13 |
| R6 cites Firecracker v1.15.1; the pin is v1.17.0 | LOW | **Adopted.** Re-read at v1.17.0: `avail_features` are VERSION_1 plus DEFLATE_ON_OOM, STATS_VQ, FREE_PAGE_HINTING, FREE_PAGE_REPORTING; no PAGE_POISON (bit 4). R6 repeats at each bump | §5.1 pin 3, §11 R6 |
| S3 can consume the last fence margin (worst case reaches the 120 s TTL) | LOW | **Adopted.** Skip the act when more than 60 s have passed since the host sent prepare; worst case 80 s; new D6-M26 and its S3 test | §4.3, §7 M26, §9 S3 |
| 409 vs 503 for unanswered store reads | LOW | **Adopted.** One code: 409 `busy`, reason `store_unanswered`; the route has no 503 | §4.1, §9 S1 |
| S4 does not name the act's mode | LOW | **Adopted.** S4 calls `{"mode":"idle"}` | §4.4 |
| `admission.recent` holds 20, not a ring of 32 | nit | **Adopted.** `memory.cache_return.recent` is a ring of 20 | §4.6 |
| S3 adds up to 10 s, not 5 s | nit | **Adopted** | §4.3, §13 |
| The hibernate-mode `drop_caches` fallback is invisible in the report (M25 half-closed) | nit | **Adopted.** The report carries `method` (`memory_reclaim`, `drop_caches`, `none`) and `path`; S1's M25 test asserts both | §4.1, §7 M25, §9 S1 |
