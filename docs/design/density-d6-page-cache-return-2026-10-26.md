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

- **Mutation class of this document:** green. Build slices: S1 orange
  (guest runtime, inert until called), S2 to S4 red (vmctl, the hibernate
  path, admission), S5 red tier 2 (the paper under load).
- **Review tier:** this is an architectural design (a new mechanism), so it
  is reviewed like the change it designs: tier 1 (Devin). Every red slice
  is additive behind one env switch with a one-act rollback that shows on
  staging within minutes, and none changes the launch config, snapshot
  validation or a protected class's lifecycle, so S2 to S4 are tier 1. S5
  touches the paper under load: tier 2.
- **Owner inputs that bind this design:** guests do not swap ("swapping is
  service degradation. we cant tolerate it", 2026-10-09). "We shouldn't
  need to be doing that manual memory management ... we can virtualize
  their memory" (2026-10-26). Resource use ahead of short-term uptime, with
  a proof and a rollback (CLAUDE.md, 2026-10-22). A computer stays on while
  it is working; crash restarts close work (CLAUDE.md). Always-busy
  computers are the exception that must still be served (2026-10-15).

## 0. The answer in brief

**What returns page cache.** A page reaches the host again only through
three steps: the guest frees it, the guest kernel reports the free block,
Firecracker discards it. Steps 2 and 3 are built and on for every computer
(free page reporting, `VM_BALLOON_FREE_PAGE_REPORTING=1`) and proved:
after a capsule burst the host RSS fell 1,614.7 MiB for 1,288.2 MiB the
guest freed (ledger 10-22 21:26, outcome C). Step 1 is the gap. A guest
frees clean page cache only under its own memory pressure, and an 8 GiB
guest with 2 GiB of anonymous memory never feels any. DAMON_RECLAIM, the
built-in proactive path, freed 31 MiB in 16 idle minutes on its test
computer (ledger 10-22 13:06; its reading is still owed).

**Recommended mechanism: a guest-side, bounded, cold-first cache return
act** (`memory.reclaim` on the guest's root cgroup, sized to the guest's
inactive file pages), triggered by vmctl over the existing host-peer guest
surface, carried to the host by the free page reporting every guest already
has. Not balloon inflation, for five reasons (§3): it frees the same pages
by the same kernel path (reclaim through the page allocator) but adds a
target that persists in Firecracker and in every snapshot (a second memory
state authority beside RES-09's virtio-mem leases); it needs balloon
statistics to size safely, which changes the launch config and cold boots
each computer once; it can push past cache into anonymous memory, where
`deflate_on_oom` gives pages back only at the guest's OOM point; a computer
woken from a snapshot wakes into its inflated balloon until vmctl deflates
it; and the evidence says the return path already works, so the balloon's
one extra power (taking scattered 4 KiB pages) is not needed. `memory.reclaim`
is already in production for capsule teardown
(`internal/capsule/teardown_reclaim.go`, V27/V28, the substrate this
design reuses, EAGAIN trap included). Balloon inflation stays the
conditional S6, built only if a reading shows freed-but-not-returned at
scale (outcome B).

**When it runs.** Three triggers, each its own slice, all behind one
switch:
1. **Idle** (S2): a running computer idle 3 minutes, outside its admission
   settle window, guest not busy by its own count. Once per idle period;
   always-on computers re-armed every 30 minutes. This is where the
   memory sits today: the owner's computers (premium_always_on) never
   idle-hibernate, and their cache never returns.
2. **Before hibernate** (S3): inside the hibernate fence, after the guest's
   `sync`, before the pause. The Diff (D5) and the Full file's real bytes
   shrink by the returned cache.
3. **Under host pressure, before make-room** (S4): D1's refused start first
   asks idle running computers to return cache, waits for the host to see
   it, re-samples, and hibernates a victim only if that was not enough.

**Always-busy computers** (the paper) are excluded from all three until S5,
a decision gate: run the owed DAMON reading; if DAMON frees cold cache, the
paper gets DAMON (boot arguments, one cold boot at a planned refresh); if
not, the paper gets a busy-safe mode of the same act (pressure only,
inactive file pages above a keep, 64 MiB steps, stop on refaults).

**Expected return** (estimates from recorded readings, §6): the paper about
4.5 GiB of its ~8 GiB host RSS (S5 only); each idle always-on computer its
inactive cache, about 1 to 3 GiB (UNVERIFIED split; reading R2); a
hibernate's real snapshot bytes about halved on a typical test computer.
Host-wide, on today's mix, about 2 to 4 GiB without the paper and 6 to 8
GiB with it, of Node B's 31 GiB.

**Switch:** `VMCTL_GUEST_CACHE_RETURN=off|observe|active` (fails closed),
with a selector and a trigger list. **Rollback, one act:**
`VMCTL_GUEST_CACHE_RETURN=off` and a vmctl restart. No launch-config byte
changes, so no snapshot is voided and no computer cold boots, either way.

**Owner decision needed:** none to build S1 to S4. S5 puts a reclaim on the
paper while it works; CLAUDE.md's resource-use rule permits that with a
proof and a rollback, and the conservative default (pressure only, refault
guard) is FP0's to rule. Recorded as a ratification point, not a blocking
ask (§12).

## 1. What exists today

| Lever | Code | On where | Proven |
|---|---|---|---|
| Balloon device, `amount_mib: 0`, `deflate_on_oom: true`, `free_page_reporting: true` | `internal/vmmanager/balloon.go` `firecrackerBalloonConfig`; in every cold boot's `--config-file` | every computer (`VM_BALLOON_FREE_PAGE_REPORTING=1`, node-b.nix:1668) | outcome C, 10-22 21:26 |
| Balloon statistics, `stats_polling_interval_s` | `balloon_stats.go` (`VM_BALLOON_STATS_ONLY`); reader `GuestMemoryStats`; route `GET /internal/vmctl/memory/balloon` | b@c only (node-b.nix:1674) | inert case and b@c probe PASS (10-24) |
| Free page hinting | never sent (developer preview, documented corruption race) | nowhere | n/a |
| Balloon inflation (`PATCH /balloon`) | **none**: `fcapi.go` has `Balloon` and `BalloonStatistics` (GET) only; `amount_mib` is always 0 | nowhere | n/a |
| virtio-mem hotplug and unplug | `memory_hotplug.go`, `unplugBeforeSnapshot` | `VM_MEMORY_HOTPLUG=off` | built; only moves the hotplug region, never base memory |
| DAMON_RECLAIM boot arguments | `guest_reclaim.go` (`VM_GUEST_RECLAIM_ONLY`) | two test computers (node-b.nix:1691) | negative or unread (§2) |
| `memory.reclaim` on a capsule cgroup at teardown | `internal/capsule/teardown_reclaim.go`, `job_memory.go` `writeCgroupFile` | every guest (`CHOIR_CAPSULE_TEARDOWN_RECLAIM`, code default on) | D3 drill; outcome C's burst end |
| Guest memory-return report | `internal/server/debug_memreturn.go` on `procmem`: DAMON, page reporting order, balloon features, buddyinfo, vmstat (`pgsteal_proactive`, `workingset_refault_file`, `pgmajfault`) | every guest | reads correctly (bunch 87, 102) |
| Hibernate fence | `agentcore/hibernate.go` `prepareHibernate`: closes admission, refuses busy, renews the capability, `sync` | every snapshot hibernate | S3 |

So, to the question asked: **the balloon exists in our launch config, with
`deflate_on_oom` and free page reporting on, never inflated; statistics on
for one computer; hinting never.** No code anywhere inflates it, and no code
anywhere asks a running guest to free cache outside a capsule's teardown.
There is no unwired balloon implementation to connect; the
`MemoryController.SetBalloonTargetMiB` seam in
`docs/reports/vm-backend-interface-2026-10-14.md` is documentation only.

## 2. Why guest page cache never returns

The chain (outcomes report): freed in the guest, gathered into a reportable
block (order 9, 2 MiB), discarded by the VMM.

- **Step 1 fails.** Clean page cache is not free memory. The kernel
  reclaims it only below its watermarks. The paper's guest reported
  MemAvailable 5.2 to 6.0 GiB the whole time its VMM held 8,093 MiB (10-22
  07:41 to 07:50); its page cache was 4,747 MiB, 4,210 MiB of it the Dolt
  store, 4.75 GB inactive against 0.17 GB active (ledger 10-22 06:25 and
  11:44). That is cold cache the guest would give back if asked, and
  nothing asks.
- **Steps 2 and 3 work** when step 1 happens: outcome C (host RSS fell
  1,614.7 MiB for 1,288.2 MiB freed after capsule teardown's
  `memory.reclaim`). So the freed pages did form reportable blocks, at
  least for a reclaim of a whole cgroup's cache.
- **DAMON** (the kernel's own proactive cold-page reclaim) is built and
  selected on two test computers. The one reading (10-22 13:06) showed the
  selected guest's cache fall 1,055 to 1,024 MiB in 16 idle minutes and host
  RSS flat. Whether DAMON was running is unknown (outcome A or A'); its
  DAMON-on reading is still owed.

Consequences, each in the evidence: a computer's host RSS rises to its
guest size after one big read or build and stays (b@c's peak 7,847 MiB
after engineering capsule builds); a Full snapshot writes that cache and a
Diff (D5) will too; admission charged b@c its last peak (D4 fixes the
charge, not the holding); the paper, which never idles into hibernation,
holds about 5 GiB of cold cache forever.

**The guest copy is the expensive copy.** The guest's data disk is a host
file that Firecracker reads with buffered I/O (`manager.go` drives: no
`cache_type`, so Firecracker's default; UNVERIFIED that the host page cache
holds the hot part of each `data.img`, reading R3). The host's copy of a
page is clean, shared and counted in MemAvailable; the guest's copy of the
same page is anonymous memory to the host, cannot be reclaimed and cannot
swap. Returning guest cache moves a re-read from a guest cache hit to a
virtio-blk read that is often a host cache hit. That is the trade every
mechanism below makes; the refault counters measure its price.

## 3. Mechanisms compared

| Mechanism | Frees what | Bounded? | Cold-first? | Needs | State left behind | Verdict |
|---|---|---|---|---|---|---|
| `drop_caches` (guest, `echo 1`) | all clean cache, hot included | no (all or nothing) | no | guest code | none | fallback only, hibernate mode only, when the root `memory.reclaim` is absent |
| **`memory.reclaim` on the guest root cgroup** | file pages in LRU order (MGLRU oldest generation first); no swap, so never anon | yes (bytes per write; EAGAIN on partial) | yes | guest code (route), host trigger | none: an act, not a target | **recommended** |
| Balloon inflate (`PATCH /balloon {amount_mib}`) | whatever the guest's allocator reclaims to satisfy it: cache in LRU order, then anon pressure | yes (a target) | by the guest's reclaim, yes; beyond cache, no | stats on (launch-config change, one cold boot per computer), a deflate controller, vmctl-restart reconciliation, one owner of guest memory targets with RES-09 | a target in the VMM and in every snapshot | conditional S6 only |
| Free page reporting + a cache-drop trigger | this *is* the recommendation: reporting carries what the trigger frees | | | | | (same row as `memory.reclaim`) |
| Free page hinting | free pages, host-initiated | | | API runs, developer preview, corruption race | | rejected |
| virtio-mem unplug | the hotplug region only; base memory never | yes | n/a | `VM_MEMORY_HOTPLUG` on, a guest booted small-base | lease state (RES-09) | not this item: it would need every guest to boot with a small base and grow by plugging, a redesign of memory shape with Diff-length consequences; recorded as the longer substrate (§12) |
| DAMON_RECLAIM (boot args) | cold regions after `min_age`, quota-bounded, continuous | yes (quota) | yes, by access age | boot args (launch-config hash: one cold boot per selected computer) | none | unproven; the right shape for always-busy computers if its reading passes (S5 gate) |

Why not the balloon, in detail (the item's own title names it, so the
choice is argued, not assumed):

1. **Same freeing path, more state.** Inflation asks the guest's page
   allocator for N pages; the allocator reclaims cache to supply them, in
   the same LRU order `memory.reclaim` uses. The balloon then holds the
   pages, and Firecracker discards them. The memory returned is the same.
   What the balloon adds is a *target*: it lives in the VMM, is saved in
   every snapshot (the device state, `target_pages`), and must be
   reconciled after a vmctl crash and deflated before a woken computer
   needs the memory. That is a second guest-memory state authority beside
   RES-09's virtio-mem leases (standing question "single state
   authority"; the RES-10 design already said step 3 waits for one
   controller).
2. **Sizing needs statistics.** A safe target needs the guest's own
   available memory, so every inflated computer needs
   `stats_polling_interval_s` at boot, which is in the launch-config hash:
   each computer cold boots once when selected and once when deselected
   (the rollback). The reclaim act reads `/proc/meminfo` inside the guest
   and needs no device change.
3. **Overshoot lands on anonymous memory.** A target larger than the cold
   cache makes the guest reclaim hot cache, then fail allocations.
   `deflate_on_oom` returns balloon pages only from the OOM path, in small
   batches: a run that starts while the balloon is inflated meets memory
   pressure first. The reclaim act asks only for inactive file pages and
   cannot touch anonymous memory on a guest with no swap.
4. **Snapshots carry it.** A computer hibernated inflated wakes inflated.
   Its wake is cheap, but its first run is squeezed until vmctl deflates.
   The reclaim act leaves nothing to undo.
5. **The balloon's real extra power is not needed yet.** Inflation takes
   any free 4 KiB page, so it beats fragmentation, where reporting needs
   whole 2 MiB blocks. Outcome C says reclaimed cache does form reportable
   blocks. If S2's readings show a large `freed_not_returned` share
   (guest MemFree up, `reportable_kib` small, host RSS flat), S6 builds the
   balloon as the fragmentation answer, under the RES-09 single-controller
   rule.

A settled fact, from source (not observed): with
`VIRTIO_BALLOON_F_DEFLATE_ON_OOM` negotiated, Linux's `virtio_balloon`
does not call `adjust_managed_page_count` on inflate
(`drivers/virtio/virtio_balloon.c`, `fill_balloon`), so the guest's
`MemTotal` would not move under inflation. That closes RES-10's open
question for S6 if it is ever built; GOMEMLIMIT and capsule admission,
which read `MemTotal` once, would be unaffected. `memory.reclaim` does not
touch `MemTotal` either.

## 4. Design: the cache return act

### 4.1 Guest side (S1): the reclaimer and its route

A small package, `internal/guestmem` (name is the builder's), with no
dependency on agentcore:

- **Read.** `/proc/meminfo`: `MemTotal`, `MemFree`, `MemAvailable`,
  `Active(file)`, `Inactive(file)`, `Dirty`, `Shmem`, `AnonPages`;
  `/proc/vmstat`: `pgsteal_proactive`, `workingset_refault_file`,
  `pgmajfault`; `/proc/buddyinfo` (the existing `readBuddyinfo` gives
  `reportable_kib`); the balloon's negotiated reporting bit (the existing
  `debug_memreturn.go` reader).
- **Size.** Mode `idle`: `target = Inactive(file) − keep`, keep 256 MiB.
  Mode `hibernate`: `target = Inactive(file)` (the active set stays, so the
  woken computer's hot pages are still in its snapshot). Mode `busy_cold`
  (S5 only): `target = min(1 GiB, Inactive(file) − max(512 MiB,
  Active(file)))`. Below 128 MiB: outcome `nothing_to_return`, no write.
  `Shmem` (tmpfs) and anonymous memory are never targets: without swap they
  cannot be reclaimed, and a target that counts them would make the act
  spin.
- **Reclaim.** Write the step size to `/sys/fs/cgroup/memory.reclaim` (the
  root; the memory controller's `reclaim` file is not root-excluded in
  `mm/memcontrol.c`, UNVERIFIED on our 6.18 guest, S1 dry run reports it)
  with `writeCgroupFile` from `internal/capsule/job_memory.go` (raw
  non-blocking descriptor: `os.WriteFile` parks forever on EAGAIN, the
  failure-3 trap of 10-20). Steps of 128 MiB (`busy_cold`: 64 MiB). Before
  each step, the busy predicate (below) and the time budget. EAGAIN is a
  partial reclaim: record it and stop. Where the kernel accepts it, pass
  `swappiness=0` in the write (the key exists on recent kernels;
  UNVERIFIED on 6.18; without swap it changes nothing, it is belt and
  braces for the owner's no-swap rule).
- **Fallback when the root file is absent:** in `hibernate` mode only, one
  `echo 1 > /proc/sys/vm/drop_caches` (the guest is fenced, so the hot set
  costs only post-wake refaults); in `idle` mode, `unsupported`, no write.
- **Compact.** After an `idle` or `hibernate` act that freed at least 256
  MiB, if `reportable_kib` is under half of `MemFree`, one write of `1` to
  `/proc/sys/vm/compact_memory`, inside the time budget. Never in
  `busy_cold`.
- **Run quietly.** The writes run on a locked OS thread at nice 19
  (`runtime.LockOSThread`, `unix.Setpriority(PRIO_PROCESS, tid, 19)`), so a
  run that starts mid-act wins the vCPU. Reclaim cost lands on that thread.
- **Busy predicate** (fail closed: any count it cannot read is busy):
  `RunningCount() > 0`, active self-development operations > 0, live
  engineering capsules > 0 (the D3a count, `plannedRestartBusy(ctx,
  true)`, through its one-flight 3 s cache), desk pending mutations > 0.
  Mode `hibernate` skips it (the fence already proved idle). Mode
  `busy_cold` replaces it with "no self-development apply in flight and no
  live capsule" (the CLAUDE.md always-busy exclusion) plus the refault
  guard.
- **Refault guard** (`busy_cold`, and recorded in every mode):
  `workingset_refault_file` per second over the act and the 60 s before it;
  a rise above a threshold (S5 sets it from S2's readings) stops the act
  and backs off 30 minutes.
- **One act at a time:** a guest mutex; a second call answers
  `in_progress`.
- **Budget:** `idle` 2 s, `hibernate` 3 s, `busy_cold` 1 s.

Route: `POST /internal/runtime/memory/cache-return`, host-peer only
(`requireInternalRuntimeCaller`, as the hibernate routes). Body
`{"mode":"idle"|"busy_cold","dry_run":bool}` (`hibernate` mode is not on
this route; see 4.3). Answers: 200 with a report; 409 `busy` or
`in_progress`; 503 when a count cannot be read. Report: `outcome`
(`returned`, `partial`, `nothing_to_return`, `busy`, `unsupported`,
`refault_stop`), `mode`, `target_bytes`, `reclaimed_bytes` (guest
`pgsteal_proactive` delta × page size), `steps`, `ms`, `compacted`,
`root_reclaim_supported`, `reporting_negotiated`, and `before` and `after`
blocks (the meminfo and vmstat fields above, `reportable_kib`).

S1 also adds `Active(file)`, `Inactive(file)` and `Dirty` to
`procMeminfoKeys` (`internal/server/debug_procmem.go`), so FP0's existing
probes see the cold share without the route.

The guest's numbers are the guest's claim. vmctl never uses them for
admission; it uses only what the host measures (§4.4). A guest that lies
or ignores the call returns nothing and costs one HTTP call.

### 4.2 Host side, idle trigger (S2)

In vmctl, a pass of the existing idle sweep (every `VMCTL_IDLE_SWEEP_INTERVAL`,
2 minutes) after the hibernate decisions, so a computer about to be
hibernated is not first reclaimed. A computer is a candidate when every
rule holds:

- selected (`VMCTL_GUEST_CACHE_RETURN_ONLY`, read against the ownership:
  computer ids, `class:test`, `class:primary`, `class:premium`, `*`; the
  ownership's own `WarmnessClass`, not `VMConfig`, so the FP8 selector
  residual where `class:` misses some boots does not apply);
- not `critical_protected` and not `public_platform` (until S5);
- running and active, not held, no lifecycle claim, no hibernate or start
  in flight;
- idle at least `VMCTL_GUEST_CACHE_RETURN_IDLE` (3 minutes) by the
  registry clock;
- **outside its admission settle window** (no unsettled reservation in
  `outstandingLocked`; D4 captures settled RSS at window end, and a
  reclaim before then would make D4 under-charge every later wake);
- not returned since its last activity, or, for a computer the idle sweep
  never hibernates (premium_always_on, and any primary while keepalive
  holds it), not returned in the last 30 minutes;
- the guest busy probe answers idle (the same fail-closed probe make-room
  uses), run last, only for candidates that passed the rest.

At most two acts in flight host-wide, one per VM (a per-VM in-flight map),
each with a 5 s HTTP bound. A 404 (an old guest) marks the VM
`unsupported` until its next boot; no retry storm.

Mode `observe` sends `dry_run: true`: the guest sizes and reports, writes
nothing; vmctl records what it would have returned.

### 4.3 Before hibernate (S3)

The hibernate fence already runs `sync` in the guest. S3 adds, behind the
same switch and selector:

- vmctl decides eligibility and passes it into the hibernate call as a
  hibernate option, never as a `VMConfig` or launch-config field (the
  launch-config hash must not move).
- `guestHibernatePrepare` posts `{"cache_return":"hibernate"}` instead of
  `{}`. The guest handler decodes the body leniently (an old vmctl's `{}`
  or an empty body means no reclaim; an old guest ignores the body, as its
  handler never reads it). After `syncFilesystems`, the guest runs the
  reclaimer in `hibernate` mode and adds `cache_return` to the prepare
  response.
- vmmanager then waits, before the pause and before the L3a decision, for
  the host to see the return: it polls the VM's `VmRSS` every 250 ms and
  goes on when two readings differ by under 16 MiB or after 4 s
  (`snapshotCacheReturnSettle`). The guest is fenced the whole time; the
  fence TTL is 2 minutes. Reporting runs about 2 s after a batch of free
  blocks appears (`PAGE_REPORTING_DELAY`), so most of the return lands in
  the window. Pages not yet reported are still resident and are written,
  which is correct, only larger.
- The timeline gains `cache_return` (guest report summary, RSS before and
  after the settle, settle ms). The hibernate log line gains
  `cache_return_mib=` and `rss_settle_ms=`.

A failed or refused act never fails the hibernate: the snapshot proceeds
exactly as today.

### 4.4 Under host pressure, before make-room (S4)

In `makeRoomAndAdmit` (D1), when a start is refused for
`host_memory_pressure` and the `pressure` trigger is on, before planning
any victim:

1. Candidates as in 4.2, except the idle floor is
   `VMCTL_GUEST_CACHE_RETURN_PRESSURE_IDLE` (default 1 minute) and the
   30-minute re-arm does not apply. Rank by the host-measured excess,
   `RSS − settled RSS` (D4's field; unmeasured ranks last).
2. Call the act on candidates one at a time until the summed host-measured
   RSS drop covers the shortfall, at most 3 computers and 6 s in all.
3. Re-sample MemAvailable after each act's RSS settles (as 4.3) and
   re-admit on the fresh sample. **Pending return is waited for, never
   counted** (D1's M17 rule).
4. Admit (`admit_after_cache_return`, with the VM ids and MiB each
   returned, in `admission.recent`) or fall through to D1's victim plan
   unchanged.

This runs inside the make-room slot (`makeRoomMu`, one decider), so two
refused starts never both squeeze the same computers. Sustained pressure
reclaim (`reclaimSustained`) gets the same step 0 in a later slice only if
S4's evidence calls for it.

### 4.5 The switch

| Variable | Values | Default (code) | First staging value |
|---|---|---|---|
| `VMCTL_GUEST_CACHE_RETURN` | `off`, `observe`, `active` (only these exact words; anything else is `off`) | `off` | `observe` |
| `VMCTL_GUEST_CACHE_RETURN_ONLY` | selector (empty: nobody) | empty | d@e's computer and one test computer |
| `VMCTL_GUEST_CACHE_RETURN_TRIGGERS` | subset of `idle,hibernate,pressure` | `idle` | `idle`, then `idle,hibernate`, then all |
| `VMCTL_GUEST_CACHE_RETURN_IDLE` | duration, 1m to 9m | 3m | 3m |
| `VMCTL_GUEST_CACHE_RETURN_PRESSURE_IDLE` | duration, 0 to 9m | 1m | 1m |

Rollback, one act: `VMCTL_GUEST_CACHE_RETURN=off` in `nix/node-b.nix`,
vmctl restarts with the deploy. No guest restart, no cold boot, no snapshot
voided: nothing D6 does is in the launch config or the snapshot meta. A
revert of the code commits is the second rollback; the guest route left
behind is inert when nobody calls it.

### 4.6 The record

- vmctl journal, one line per act: `guest cache return VM <id>
  trigger=<idle|hibernate|pressure> mode=<…> outcome=<…>
  guest_target_mib= guest_reclaimed_mib= rss_before_mib= rss_after_mib=
  rss_anon_before/after= rss_file_before/after= host_avail_before_mib=
  host_avail_after_mib= settle_ms= ms=`.
- vmctl health: `memory.cache_return.recent` (a ring of the last 32 acts,
  as `admission.recent`) and per-VM `last_cache_return_at`.
- The returned memory counted for any decision is only the host's own
  measurement: the VM's RSS drop and MemAvailable.

## 5. Interactions

### 5.1 D5 diff snapshots (correctness)

**L3a (cold-origin Diff).** L3a's argument: in a cold-booted Firecracker
process, guest memory is anonymous and cannot swap, so a page that is not
resident was never touched or was discarded (free page reporting,
virtio-mem unplug) and reads as zero, which is what a hole reads as. D6
adds no new way for a page to become non-resident: the act frees pages in
the guest, and the existing reporting path discards them. A discarded page
is free in the guest. The guest assumes nothing about a free page's bytes:
it zeroes on allocation when it needs zeroes. The one kernel mode that
assumes free pages are already zero, `init_on_free=1` (and page
poisoning), makes `virtio_balloon` refuse to negotiate reporting unless the
device offers `VIRTIO_BALLOON_F_PAGE_POISON`; outcome C proves our guests
negotiate reporting, so neither mode is on. Invariant to pin (S3 test): the
guest kernel command line built by vmmanager never carries
`init_on_free=1` or `page_poison=1`.

Ordering: the act and the RSS settle run before `diffSnapshotRefusal` and
before the pause, so nothing D6 does runs in frozen time or changes the
Diff decision's inputs. Pages being reported at the pause are in the
balloon's virtqueue, which the snapshot saves; that is today's behaviour
with reporting on, unchanged.

**L3b (Diff of a restored guest, rebased onto the old memory file).** A
page discarded after a restore is not resident, the Diff misses it, and
the rebase gives it the base's old bytes. That is correct only because the
page is free in the guest (same argument as above). Today reporting
already does this; D6 does it more often. L3b's design must state this
argument; D6 hands it the same invariant (no `init_on_free`, no page
poisoning). Whether Firecracker's discard on a restored, file-backed
private mapping maps fresh anonymous memory or reverts to the file page
does not change correctness, only whether RSS falls: reading R4.

**Smaller Diffs.** Every page the act frees and reporting discards before
the pause is a hole in the Diff and zeros in a Full file (which btrfs zstd
then shrinks). That is S3's benefit.

### 5.2 D4 wake cost

- The act never runs inside a VM's admission settle window (4.2), so D4's
  settled RSS is a wake's own cost, never a post-reclaim figure.
- The peak stays recorded and shown; the act lowers later samples, not the
  start's recorded peak.
- A restored guest's RSS is part file-backed: 364 MiB just after a wake was
  52 MiB anonymous and 312 MiB file (s3 evidence, 10-14). The file part is
  the snapshot memory file's host page cache, which the host can reclaim
  and already counts in MemAvailable. D4 charges `VmRSS`, so it slightly
  over-charges snapshot wakes, the safe direction. D6 records `RssAnon` and
  `RssFile` separately so a later D4 revision can decide; no change here.
- Overcommit: returned memory lets admission admit more, and a computer
  whose cache regrows takes it back. That is D4's recorded overcommit
  failure mode; the backstops are the same (sustained pressure reclaim,
  make-room on the next start, ordinary VMs at `oom_score_adj` 500), and S4
  adds one more (return before evict).

### 5.3 D1 make-room and D3 idle hibernation

S4 is a step 0 inside D1's slot, before its victim plan; D1's rules (one
decider, fresh sample, pending never counted, cancelled start stops) apply
unchanged. D3 hibernates idle computers at 10 minutes; the idle act at 3
minutes returns the cache of a computer that will stay up (always-on, or a
primary held by keepalive) and is redundant for one that will hibernate at
10 anyway, except that S3 then shrinks its snapshot. The idle pass runs
after the sweep's hibernate decisions in the same tick, so it never acts on
a computer the sweep is hibernating.

### 5.4 A working computer

The rule is "never slow a live run". The guest decides busy itself, with
the same counts the hibernate fence and D3a's `/health` use, and fails
closed. A run that arrives mid-act meets at most one 128 MiB step on a
nice-19 thread, then the act stops. The act takes only inactive file pages
above a keep, the pages the kernel itself would take first under pressure.
The refault counters are recorded on every act; a refault rise after idle
acts on test computers is the signal the keep is too small (S2 proof).

### 5.5 The paper and always-busy computers

The paper (critical_protected) and public_platform computers are excluded
from S2 to S4. They are always busy, so the idle trigger would never fire,
and a pressure act would run while they work. S5 is their slice, behind a
decision gate:

1. FP0 runs the owed DAMON-on reading (`scripts/fp8_memory_return_probe.py`
   on a reclaim-selected test computer, 20 idle minutes and a busy
   window). It decides between two shapes.
2. **DAMON frees cold cache (outcome C, refaults flat):** the paper joins
   `VM_GUEST_RECLAIM_ONLY`. Boot arguments are in the launch-config hash,
   so this takes effect at the paper's next planned, marker-backed
   refresh (the always-busy path of 10-15), never by a crash-style boot.
   No D6 code runs on the paper.
3. **DAMON does not:** the paper gets `busy_cold` through the route,
   pressure trigger only, with the refault guard, and never during a
   self-development apply or a live capsule.

Either way the proof order is a test computer kept busy by a loop, then
d@e, then the paper last (the realism axis), with the paper's poll latency
and refaults read before and after.

### 5.6 Restart rules

The act is not work in the CLAUDE.md sense: it opens no obligation, writes
nothing durable in the guest, and is idempotent. A vmctl restart resumes
nothing (the in-flight map and `last_cache_return_at` are memory only; a
restarted vmctl may act once more on a computer, harmless). A guest crash
mid-act leaves nothing to close. The planned-restart marker is untouched.

### 5.7 Other levers

- **RES-09 virtio-mem leases:** untouched; the act moves no target.
  `unplugBeforeSnapshot` still runs first in the hibernate path; S3's act
  runs after it.
- **Balloon statistics:** untouched; the act needs no device. b@c's
  statistics reading is a cross-check for the guest's own numbers, read
  only (b@c is not a proof computer until X1).
- **DAMON:** the two DAMON-selected test computers stay out of D6's
  selector, so their reading stays attributable.

### 5.8 Security

The route is host-peer only. vmctl's call carries no host credential to
the guest (`hostauth.MarkInternal`, as the busy probe; the guest gate is
the source address). A guest can refuse or lie; vmctl acts only on host
measurements, so a hostile guest can at most keep its own memory, which it
can today. This is a density lever, not an isolation control.

## 6. Metric, baseline and expected return

**Primary metric:** host `MemAvailable` (the admission input), and per VM
the Firecracker process's `VmRSS`, `RssAnon`, `RssFile`, read before each
act and after its settle. **Price:** guest `workingset_refault_file` and
`pgmajfault` per minute in the 10 minutes after an act, against the 10
minutes before; first desk turn latency after an idle act. **Snapshot:**
`stat -c '%s %b %B'` of `snapshot/mem` (real bytes for Diff), the hibernate
line's `disk_bytes`, `snapshot=` ms, and the wake's settled RSS.

Baseline (recorded):

| Reading | Value | Source |
|---|---|---|
| Paper host RSS / guest cache / inactive vs active file | 8,093 MiB / 4,747 MiB (4,210 MiB Dolt store) / 4.75 GB vs 0.17 GB | balloon-stats doc 10-22 07:50; ledger 10-22 06:25, 11:44 |
| Paper guest MemAvailable while its VMM held 8 GiB | 5.2 to 6.0 GiB | balloon-stats doc, 07:41 to 07:46 |
| Owner's computer, another user's, candidate VM, test computer | 3,930 / 1,330 / 2,177 / 1,618 MiB host RSS | balloon-stats doc 10-22 07:50 |
| Test computer 71afbf14: guest in use / cache / host RSS | 820 / 631 / 1,347 MiB | ledger 10-22 12:25 |
| Reporting return ratio | host −1,614.7 MiB for guest +1,288.2 MiB free | ledger 10-22 21:26 |
| b@c last peak vs wake cost | 7,847 MiB vs 0.3 to 1.4 GiB | start-refused doc §3; s3 evidence |
| Full snapshot real bytes and time | 0.74 to 1.5 GB (outliers 2.4, 4.0 GB); 5.6 to 10.4 s | start-refused doc §6.4; hibernate problem R1 |
| Wake RSS (snapshot) | 364 MiB (52 anon, 312 file) | s3 evidence 10-14 |

Expected return (estimates; each is confirmed or corrected by S2's
observe readings before any `active`):

- **Per act:** host RSS falls by about 0.8 to 1.25 times the guest's
  reclaimed bytes, within 2 to 4 s (outcome C's ratio; reporting delay).
- **Idle always-on computers:** their inactive file pages less 256 MiB.
  The owner's 3,930 MiB computer is the type case; its split is unknown
  (reading R2). Estimate 1 to 3 GiB each.
- **The paper (S5):** about 4.5 GiB (4.75 GB inactive), host RSS from about
  8 GiB to about 3.3 GiB. This is the largest single return on the host.
- **Hibernate (S3):** on a 71afbf14-like computer, a Diff of roughly the
  in-use 0.2 to 0.4 GiB instead of about 1 GiB with its cache; a Full file's
  real bytes fall by about the same.
- **Admission:** D6 does not lower any charge; it raises MemAvailable. The
  11:49 case (448 MiB short) would admit after one idle always-on
  computer's return, with no hibernate.
- **Host-wide on today's mix:** about 2 to 4 GiB without the paper, 6 to 8
  GiB with it, of 31 GiB.

## 7. Failure modes (written before the tests)

- **D6-M1** an act runs while a run, a pending desk mutation, a
  self-development operation or a live capsule is in flight (guest busy
  predicate, fail closed; re-checked before each step; vmctl's probe
  first).
- **D6-M2** an act evicts the hot set and runs slow afterwards (only
  `Inactive(file)` above a keep; refaults recorded; S2 proof compares
  refaults before and after; Goodharting is a host RSS win paid in
  refaults).
- **D6-M3** an act reclaims anonymous memory or anything swaps (no swap in
  the guest; targets never include `AnonPages` or `Shmem`; `swappiness=0`
  where the kernel takes it; test: `AnonPages` unchanged by a fake act).
- **D6-M4** the write blocks the guest daemon (EAGAIN parks `os.WriteFile`
  forever: raw non-blocking write; per-act time budget; one act at a time).
- **D6-M5** memory freed but not returned (reporting not negotiated on an
  old or restored-from-old guest; fragmentation): reported as an outcome
  with `reporting_negotiated` and `reportable_kib`, counted in health; no
  retry loop. A large share at scale is S6's trigger.
- **D6-M6** D4's settled RSS is taken after an act (never act inside the
  settle window).
- **D6-M7** a Diff becomes incorrect: D6 runs before the L3a decision and
  the pause, changes no input of `diffSnapshotRefusal`, adds no
  launch-config or meta field; the guest command line never carries
  `init_on_free=1` or `page_poison=1`.
- **D6-M8** the pre-hibernate act or settle stretches the hibernate past
  its fence TTL or holds make-room's start too long (3 s act, 4 s settle,
  fence TTL 2 minutes; a failed act never fails the hibernate).
- **D6-M9** an old guest (no route, no prepare field) breaks the hibernate
  or is retried every sweep (404 is `unsupported` until next boot; old
  guests ignore the prepare body; new guests take an empty body as no act).
- **D6-M10** the idle trigger acts on the same computer every pass (once
  per idle period; 30-minute re-arm only for computers never hibernated).
- **D6-M11** an act races a hibernate, stop or start of the same VM (no
  act on a VM with a lifecycle claim or an operation in flight; one act per
  VM; an act that loses the race fails its HTTP call and records it).
- **D6-M12** the paper, a public_platform computer or a held computer is
  acted on before S5 (class exclusions pinned by test; held excluded).
- **D6-M13** the pressure step counts pending return as room, or two
  refused starts squeeze the same computer (inside `makeRoomMu`; fresh
  sample after each settle; pending never counted).
- **D6-M14** a vmctl restart or a guest crash resumes or replays an act
  (nothing persisted; restart acts at most once more).
- **D6-M15** rollback is not exact: with `off`, no guest call is made and
  the prepare body is `{}` byte for byte.
- **D6-M16** a typo turns it on (only the exact words `observe` and
  `active`; anything else is `off`).
- **D6-M17** a guest's self-report drives an admission decision (only host
  RSS and MemAvailable are used).
- **D6-M18** the launch config changes and voids snapshots (test:
  `fc-config.json` byte-identical with the switch on and off).
- **D6-M19** the act takes the host credential to a guest
  (`hostauth.MarkInternal` only).
- **D6-M20** a DAMON-selected computer is acted on and its reading loses
  attribution (selector excludes them in proofs; documented, not coded).

## 8. Tests first (each failing before its code)

S1, `internal/guestmem` with fake `/proc` and cgroup roots:
- target sizing per mode from fixture meminfo (idle keeps 256 MiB,
  hibernate keeps the active set, under 128 MiB is `nothing_to_return`,
  `Shmem` and `AnonPages` never counted);
- steps stop when the busy predicate flips mid-act (M1) and when the
  budget expires (M4); a busy predicate that errors is busy;
- a fake `memory.reclaim` that returns EAGAIN ends the act as `partial`
  without hanging (M4; mirror the EAGAIN tests in `cgroup_write_linux_test.go` and `teardown_reclaim_test.go`);
- the root file absent: `idle` answers `unsupported` and writes nothing,
  `hibernate` writes `drop_caches` once;
- a second concurrent act answers `in_progress`;
- `dry_run` writes nothing and reports the target.
S1, agentcore route: host-peer gate (403 for a guest caller), 409 on busy,
503 when a count cannot be read; prepare with `{}`, an empty body and
`{"cache_return":"hibernate"}` (only the last acts). `debug_procmem` keys
include `Active(file)`, `Inactive(file)`, `Dirty`.

S2, `internal/vmctl` (fixture clock; the idle sweep trap: `checkIdleOwnerships`
measures against `time.Now()`, so set `LastActiveAt` from real time):
- `cacheReturnCandidates` (pure; pass `now`): each class, held, busy,
  idle 2 and 4 minutes, inside and outside the settle window, returned
  since last activity or not, always-on at 29 and 31 minutes since its last
  act; only the right ones are candidates (M6, M10, M12);
- switch parsing: `active`, `observe`, `off`, a typo (M16); env parsing in
  `cmd/vmctl/env_config_test.go`;
- `off` makes no guest call (an httptest guest that fails the test if
  called) (M15); `observe` sends `dry_run: true`;
- a 404 marks `unsupported` and is not retried until boot (M9);
- the request carries no credential header (M19);
- a VM with a lifecycle claim is skipped (M11).

S3, `internal/vmmanager`: with the option off, the prepare body is `{}`
(M15); with it on, the body names the mode, the settle loop stops on two
close readings or 4 s (fake RSS reader), a failed act still snapshots
(M8); the L3a decision is computed after the settle; `fc-config.json` is
byte-identical whatever the option (M18); the kernel command line never
contains `init_on_free=1` or `page_poison=1` (M7).

S4, `internal/vmctl` with `newMakeRoomFixture`: a shortfall covered by
returns admits with no hibernate (`admit_after_cache_return`); a return
that does not cover it falls through to D1's plan; returns not yet visible
in MemAvailable are not counted (M13); two refused starts serialize (M13,
`-race`); trigger list without `pressure` leaves D1 exactly as before.

## 9. Slices (each sized for one Sonnet builder)

- **S0, readings and problem record (green; FP0 and any worker).** Add the
  D6 section to
  `docs/problems/density-station-d-failure-modes-2026-10-26.md` pointing
  here (Problem, Design, D6-M1..M20). FP0 takes readings R1 to R5 (§11).
- **S1, the guest reclaimer and route (orange, tier 1).** `internal/guestmem`,
  the route in agentcore, the lenient prepare body, the procmem keys.
  Tests, then code. Ships in the guest image; inert until vmctl calls it.
  Rollout is by guest refresh at idle; until D2 lands, each guest-artifact
  deploy already voids snapshots, so S1 adds no extra cold boot.
- **S2, the vmctl idle trigger and record (red, tier 1).** Client, switch,
  selector, candidate rule, health ring, journal line. Lands with
  `VMCTL_GUEST_CACHE_RETURN=observe` on d@e and one test computer.
- **S3, before hibernate (red, tier 1).** The hibernate option, the
  prepare body, the RSS settle, the timeline fields. After S2 has run
  `active` on its selector for a day with refaults flat.
- **S4, the pressure step in make-room (red, tier 1; FP0 may take it to
  tier 2 as D1 was, for its concurrency).** After S2 and S3.
- **S5, always-busy computers (red, tier 2).** The DAMON gate, then either
  a config change (paper into `VM_GUEST_RECLAIM_ONLY` at its planned
  refresh) or `busy_cold` with the refault guard. The paper last.
- **S6, balloon inflation (red, tier 2; conditional, not planned).** Only
  if S2's readings show `freed_not_returned` above about a quarter of
  freed bytes. Built under RES-09's single-controller rule.

D7 (the density bench) records D6's numbers with the rest.

## 10. Proof on staging (FP0; never b@c until X1; never the owner's computers for probing)

1. Deploy S1 and S2 with `observe` on d@e and one test computer. Read
   `memory.cache_return.recent`: targets, `root_reclaim_supported`,
   `reporting_negotiated`.
2. Flip to `active` for the same two. For each act: the journal line, the
   VM's `RssAnon`/`RssFile` before and after, host MemAvailable before and
   after, guest refaults over the next 10 minutes
   (`scripts/fp8_memory_return_probe.py --burst none`). Pass: host RSS
   falls by at least 70% of `guest_reclaimed_mib` within 10 s, refaults per
   minute after are within twice the before figure, and the next desk turn
   answers normally.
3. S3 on d@e (D5 step 1 already gives it Diff): hibernate after an idle
   act and record `snapshot/mem` real bytes, `snapshot=` ms, the settle
   ms, wake time and settled RSS; compare with d@e's last hibernate without
   S3. Pass: wake under 1 s, real bytes down, `sha256` check of the L3a
   tmpfs proof unchanged.
4. S4: the five-test-computer start of the station acceptance, with an
   idle always-on test computer holding cold cache; read
   `admission.recent` for `admit_after_cache_return`.
5. Rollback check: `off`, deploy, confirm no call in the journal and the
   prepare body unchanged.

## 11. Readings before S2 goes active (FP0, Node B, read only)

- **R1. Reporting on every guest:** the memreturn report's
  `balloon.reporting` and `page_reporting.registered` for each running
  computer (`GET <guest>/internal/debug/pprof/procmem`, host-sourced, no
  credential). Any false: that computer booted without reporting and the
  act cannot return its memory until a cold boot.
- **R2. The cold share:** after S1, `Active(file)` and `Inactive(file)` on
  d@e, one test computer and (read only) the owner's computer, idle and
  busy. Sets the expected return per class.
- **R3. Host page cache of a guest's disk:** `fincore
  /var/lib/go-choir/vm-state/<vm>/data.img` for d@e (util-linux, read
  only). Confirms the double-caching argument of §2.
- **R4. Restored-guest discard:** on a computer woken from a snapshot,
  `RssAnon` and `RssFile` before and after an act. Says whether a discard
  of file-backed private memory lowers RSS (correctness does not depend on
  it, §5.1).
- **R5. The DAMON-on reading** owed since 10-22 (S5's gate).

## 12. Heresy delta, conjecture, owner questions

**Discovered:**
- The item's name (balloon inflation) points at steps 2 and 3 of the
  return chain, which work; the gap is step 1. This design builds step 1
  and keeps the balloon as a conditional.
- RES-10's open question on `MemTotal` under inflation is answered from the
  Linux source (no change with `deflate_on_oom`), pending observation if
  S6 is ever built.
- A snapshot wake's RSS is mostly host-reclaimable file pages (312 of 364
  MiB), so `VmRSS`-based charges over-count restored guests; recorded for
  D4, no change.
- L3b's correctness argument must cover pages discarded after a restore
  (they rebase to stale bytes, correct only because they are free); handed
  to D5 with the `init_on_free` invariant.
- Guest tmpfs (`Shmem`) never returns without swap; out of scope, noted
  because X1 fills guest `/tmp`.

**Introduced:** a host-to-guest memory act (a new host-peer route); bounded
and stateless by design.

**Repaired (on proof):** "guest page cache never returns to the host" for
selected computers; the paper only at S5.

**Conjecture C-D6.** If an idle, unprotected computer returns its inactive
guest file pages through `memory.reclaim` and free page reporting, then its
host RSS falls by at least 70% of the bytes the guest reclaimed within 10
s, with refaults over the next 10 minutes no more than twice the idle
baseline, and a following Diff snapshot shrinks by about the same bytes.
Test: §10 steps 2 and 3. Edge: missing oracle (refault price under real
use); delta_o: the per-act record carries refaults before and after.

**The longer substrate.** The owner's "virtualize their memory" points past
D6 to guests that boot small and grow by plugging memory (virtio-mem,
RES-09), so the host decides residency and unplug forces the guest to give
back whole blocks. D6 is the cheap step that works with today's memory
shape; it does not block that redesign.

**Owner questions.** None blocks S1 to S4. One ratification point at S5:
a reclaim on the paper while it works, under host pressure only, with the
refault guard (default: FP0 rules, by CLAUDE.md's resource-use rule; the
owner may reverse it). The balloon-versus-reclaim choice is a design
branch, taken here by the conservative option (no new device, no new
state) and stated, per "No Blocking Asks".
